package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/model"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, s *model.Session) error {
	defer logger.DeferLogDuration("session.Create", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, device_id, device_name, secret_hash, last_seen_at, created_at, revoked_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULL)`,
		s.ID, s.UserID, s.DeviceID, s.DeviceName, s.SecretHash, s.LastSeenAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sessionRepo.Create: %w", err)
	}
	return nil
}

// UpsertByUserIDAndDeviceID вставляет сессию или обновляет существующую по (user_id, device_id). Устраняет duplicate key без отдельного DELETE.
func (r *SessionRepository) UpsertByUserIDAndDeviceID(ctx context.Context, s *model.Session) error {
	defer logger.DeferLogDuration("session.UpsertByUserIDAndDeviceID", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, device_id, device_name, secret_hash, last_seen_at, created_at, revoked_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULL)
		 ON CONFLICT (user_id, device_id) DO UPDATE SET
		   id = EXCLUDED.id,
		   device_name = EXCLUDED.device_name,
		   secret_hash = EXCLUDED.secret_hash,
		   last_seen_at = EXCLUDED.last_seen_at,
		   created_at = EXCLUDED.created_at,
		   revoked_at = NULL`,
		s.ID, s.UserID, s.DeviceID, s.DeviceName, s.SecretHash, s.LastSeenAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sessionRepo.UpsertByUserIDAndDeviceID: %w", err)
	}
	return nil
}

// GetByID возвращает сессию только если она не отозвана (revoked_at IS NULL).
func (r *SessionRepository) GetByID(ctx context.Context, id string) (*model.Session, error) {
	defer logger.DeferLogDuration("session.GetByID", time.Now())()
	s := &model.Session{}
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, device_id, device_name, secret_hash, last_seen_at, created_at, revoked_at
		 FROM sessions WHERE id = $1 AND revoked_at IS NULL`, id)
	err := row.Scan(&s.ID, &s.UserID, &s.DeviceID, &s.DeviceName, &s.SecretHash, &s.LastSeenAt, &s.CreatedAt, &s.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("sessionRepo.GetByID: %w", err)
	}
	return s, nil
}

// ListByUserID — только активные сессии (revoked_at IS NULL).
func (r *SessionRepository) ListByUserID(ctx context.Context, userID string) ([]model.Session, error) {
	defer logger.DeferLogDuration("session.ListByUserID", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, device_id, device_name, last_seen_at, created_at, revoked_at
		 FROM sessions WHERE user_id = $1 AND revoked_at IS NULL ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("sessionRepo.ListByUserID: %w", err)
	}
	defer rows.Close()
	var list []model.Session
	for rows.Next() {
		var s model.Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.DeviceID, &s.DeviceName, &s.LastSeenAt, &s.CreatedAt, &s.RevokedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *SessionRepository) UpdateLastSeen(ctx context.Context, sessionID string, t time.Time) error {
	defer logger.DeferLogDuration("session.UpdateLastSeen", time.Now())()
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = $1 WHERE id = $2 AND revoked_at IS NULL`, t, sessionID)
	return err
}

// RevokeByID помечает сессию отозванной (revoked_at = NOW()). Для удаления секрета из Redis вызывающий код делает отдельно.
func (r *SessionRepository) RevokeByID(ctx context.Context, sessionID string) (bool, error) {
	defer logger.DeferLogDuration("session.RevokeByID", time.Now())()
	tag, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// RevokeByUserID отзывает все сессии пользователя. Возвращает список id сессий для очистки Redis.
func (r *SessionRepository) RevokeByUserID(ctx context.Context, userID string) ([]string, error) {
	defer logger.DeferLogDuration("session.RevokeByUserID", time.Now())()
	rows, err := r.pool.Query(ctx, `SELECT id FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	_, err = r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *SessionRepository) Delete(ctx context.Context, sessionID string) error {
	defer logger.DeferLogDuration("session.Delete", time.Now())()
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

// RevokeByUserIDAndDeviceID отзывает сессию для пары (user_id, device_id).
func (r *SessionRepository) RevokeByUserIDAndDeviceID(ctx context.Context, userID, deviceID string) error {
	defer logger.DeferLogDuration("session.RevokeByUserIDAndDeviceID", time.Now())()
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND device_id = $2 AND revoked_at IS NULL`, userID, deviceID)
	return err
}

// DeleteByUserIDAndDeviceID удаляет строку сессии для (user_id, device_id). Нужно перед INSERT из-за UNIQUE(user_id, device_id).
func (r *SessionRepository) DeleteByUserIDAndDeviceID(ctx context.Context, userID, deviceID string) error {
	defer logger.DeferLogDuration("session.DeleteByUserIDAndDeviceID", time.Now())()
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND device_id = $2`, userID, deviceID)
	return err
}

// DeleteByUserIDAndSessionID отзывает одну сессию пользователя (revoked_at = NOW()).
func (r *SessionRepository) DeleteByUserIDAndSessionID(ctx context.Context, userID, sessionID string) (bool, error) {
	defer logger.DeferLogDuration("session.DeleteByUserIDAndSessionID", time.Now())()
	tag, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND id = $2 AND revoked_at IS NULL`, userID, sessionID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// SetSessionSecret сохраняет session_secret для сессии (используется в -dev для сохранения сессий после перезапуска).
func (r *SessionRepository) SetSessionSecret(ctx context.Context, sessionID, secret string) error {
	defer logger.DeferLogDuration("session.SetSessionSecret", time.Now())()
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET session_secret = $1 WHERE id = $2 AND revoked_at IS NULL`, secret, sessionID)
	return err
}

// GetSessionSecret возвращает session_secret для сессии (пусто если колонка NULL).
func (r *SessionRepository) GetSessionSecret(ctx context.Context, sessionID string) (string, error) {
	defer logger.DeferLogDuration("session.GetSessionSecret", time.Now())()
	var secret *string
	err := r.pool.QueryRow(ctx, `SELECT session_secret FROM sessions WHERE id = $1 AND revoked_at IS NULL`, sessionID).Scan(&secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if secret == nil {
		return "", nil
	}
	return *secret, nil
}

// ClearSessionSecret обнуляет session_secret при выходе/отзыве (для -dev).
func (r *SessionRepository) ClearSessionSecret(ctx context.Context, sessionID string) error {
	defer logger.DeferLogDuration("session.ClearSessionSecret", time.Now())()
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET session_secret = NULL WHERE id = $1`, sessionID)
	return err
}
