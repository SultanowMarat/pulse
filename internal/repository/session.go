package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/model"
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

// UpsertByUserIDAndDeviceID 2AÑ‚02;O5Ñ‚ A5AA8ÑŽ 8;8 >1=>2;O5Ñ‚ AÑƒÑ‰5AÑ‚2ÑƒÑŽÑ‰ÑƒÑŽ ?> (user_id, device_id). #AÑ‚Ñ€0=O5Ñ‚ duplicate key 157 >Ñ‚45;ÑŒ=>3> DELETE.
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

// GetByID 2>72Ñ€0Ñ‰05Ñ‚ A5AA8ÑŽ Ñ‚>;ÑŒ:> 5A;8 >=0 =5 >Ñ‚>720=0 (revoked_at IS NULL).
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

// ListByUserID â€” Ñ‚>;ÑŒ:> 0:Ñ‚82=Ñ‹5 A5AA88 (revoked_at IS NULL).
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

// RevokeByID ?><5Ñ‡05Ñ‚ A5AA8ÑŽ >Ñ‚>720==>9 (revoked_at = NOW()). Ð”;O Ñƒ40;5=8O A5:Ñ€5Ñ‚0 87 Redis 2Ñ‹7Ñ‹20ÑŽÑ‰89 :>4 45;05Ñ‚ >Ñ‚45;ÑŒ=>.
func (r *SessionRepository) RevokeByID(ctx context.Context, sessionID string) (bool, error) {
	defer logger.DeferLogDuration("session.RevokeByID", time.Now())()
	tag, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// RevokeByUserID >Ñ‚7Ñ‹205Ñ‚ 2A5 A5AA88 ?>;ÑŒ7>20Ñ‚5;O. Ð’>72Ñ€0Ñ‰05Ñ‚ A?8A>: id A5AA89 4;O >Ñ‡8AÑ‚:8 Redis.
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

// RevokeByUserIDAndDeviceID >Ñ‚7Ñ‹205Ñ‚ A5AA8ÑŽ 4;O ?0Ñ€Ñ‹ (user_id, device_id).
func (r *SessionRepository) RevokeByUserIDAndDeviceID(ctx context.Context, userID, deviceID string) error {
	defer logger.DeferLogDuration("session.RevokeByUserIDAndDeviceID", time.Now())()
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND device_id = $2 AND revoked_at IS NULL`, userID, deviceID)
	return err
}

// DeleteByUserIDAndDeviceID Ñƒ40;O5Ñ‚ AÑ‚Ñ€>:Ñƒ A5AA88 4;O (user_id, device_id). Ñƒ6=> ?5Ñ€54 INSERT 87-70 UNIQUE(user_id, device_id).
func (r *SessionRepository) DeleteByUserIDAndDeviceID(ctx context.Context, userID, deviceID string) error {
	defer logger.DeferLogDuration("session.DeleteByUserIDAndDeviceID", time.Now())()
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND device_id = $2`, userID, deviceID)
	return err
}

// DeleteByUserIDAndSessionID >Ñ‚7Ñ‹205Ñ‚ >4=Ñƒ A5AA8ÑŽ ?>;ÑŒ7>20Ñ‚5;O (revoked_at = NOW()).
func (r *SessionRepository) DeleteByUserIDAndSessionID(ctx context.Context, userID, sessionID string) (bool, error) {
	defer logger.DeferLogDuration("session.DeleteByUserIDAndSessionID", time.Now())()
	tag, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND id = $2 AND revoked_at IS NULL`, userID, sessionID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// SetSessionSecret A>Ñ…Ñ€0=O5Ñ‚ session_secret 4;O A5AA88 (8A?>;ÑŒ7Ñƒ5Ñ‚AO 2 -dev 4;O A>Ñ…Ñ€0=5=8O A5AA89 ?>A;5 ?5Ñ€570?ÑƒA:0).
func (r *SessionRepository) SetSessionSecret(ctx context.Context, sessionID, secret string) error {
	defer logger.DeferLogDuration("session.SetSessionSecret", time.Now())()
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET session_secret = $1 WHERE id = $2 AND revoked_at IS NULL`, secret, sessionID)
	return err
}

// GetSessionSecret 2>72Ñ€0Ñ‰05Ñ‚ session_secret 4;O A5AA88 (?ÑƒAÑ‚> 5A;8 :>;>=:0 NULL).
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

// ClearSessionSecret >1=Ñƒ;O5Ñ‚ session_secret ?Ñ€8 2Ñ‹Ñ…>45/>Ñ‚7Ñ‹25 (4;O -dev).
func (r *SessionRepository) ClearSessionSecret(ctx context.Context, sessionID string) error {
	defer logger.DeferLogDuration("session.ClearSessionSecret", time.Now())()
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET session_secret = NULL WHERE id = $1`, sessionID)
	return err
}
