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

type PermissionRepository struct {
	pool *pgxpool.Pool
}

func NewPermissionRepository(pool *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{pool: pool}
}

// GetByUserID возвращает права пользователя. Если записи нет — возвращает нулевые права без ошибки.
func (r *PermissionRepository) GetByUserID(ctx context.Context, userID string) (*model.UserPermissions, error) {
	defer logger.DeferLogDuration("permission.GetByUserID", time.Now())()
	p := &model.UserPermissions{UserID: userID}
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, COALESCE(administrator, false), COALESCE(member, true), COALESCE(admin_all_groups, false), updated_at
		 FROM user_permissions WHERE user_id = $1`,
		userID,
	).Scan(
		&p.UserID,
		&p.Administrator,
		&p.Member,
		&p.AssistantAdministrator,
		&p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return nil, fmt.Errorf("permissionRepo.GetByUserID: %w", err)
	}
	return p, nil
}

// GetByUserIDs возвращает права для набора пользователей (только существующие записи).
func (r *PermissionRepository) GetByUserIDs(ctx context.Context, userIDs []string) (map[string]model.UserPermissions, error) {
	defer logger.DeferLogDuration("permission.GetByUserIDs", time.Now())()
	out := make(map[string]model.UserPermissions, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT user_id, COALESCE(administrator, false), COALESCE(member, true), COALESCE(admin_all_groups, false), updated_at
		 FROM user_permissions
		 WHERE user_id = ANY($1::uuid[])`,
		userIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("permissionRepo.GetByUserIDs query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p model.UserPermissions
		if err := rows.Scan(&p.UserID, &p.Administrator, &p.Member, &p.AssistantAdministrator, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("permissionRepo.GetByUserIDs scan: %w", err)
		}
		out[p.UserID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("permissionRepo.GetByUserIDs rows: %w", err)
	}
	return out, nil
}

// Upsert создаёт или обновляет права пользователя.
func (r *PermissionRepository) Upsert(ctx context.Context, p *model.UserPermissions) error {
	defer logger.DeferLogDuration("permission.Upsert", time.Now())()
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_permissions (
			user_id, administrator, member, admin_all_groups, updated_at
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			administrator = EXCLUDED.administrator,
			member = EXCLUDED.member,
			admin_all_groups = EXCLUDED.admin_all_groups,
			updated_at = EXCLUDED.updated_at`,
		p.UserID,
		p.Administrator,
		p.Member,
		p.AssistantAdministrator,
		now,
	)
	if err != nil {
		return fmt.Errorf("permissionRepo.Upsert: %w", err)
	}
	p.UpdatedAt = now
	return nil
}
