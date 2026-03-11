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

type PermissionRepository struct {
	pool *pgxpool.Pool
}

func NewPermissionRepository(pool *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{pool: pool}
}

// GetByUserID 2>72Ñ€0Ñ‰05Ñ‚ ?Ñ€020 ?>;ÑŒ7>20Ñ‚5;O. Ð•A;8 70?8A8 =5Ñ‚ â€” 2>72Ñ€0Ñ‰05Ñ‚ =Ñƒ;52Ñ‹5 ?Ñ€020 157 >Ñˆ81:8.
func (r *PermissionRepository) GetByUserID(ctx context.Context, userID string) (*model.UserPermissions, error) {
	defer logger.DeferLogDuration("permission.GetByUserID", time.Now())()
	p := &model.UserPermissions{UserID: userID}
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, COALESCE(administrator, false), COALESCE(member, true), updated_at
		 FROM user_permissions WHERE user_id = $1`,
		userID,
	).Scan(
		&p.UserID,
		&p.Administrator,
		&p.Member,
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

// GetByUserIDs 2>72Ñ€0Ñ‰05Ñ‚ ?Ñ€020 4;O =01>Ñ€0 ?>;ÑŒ7>20Ñ‚5;59 (Ñ‚>;ÑŒ:> AÑƒÑ‰5AÑ‚2ÑƒÑŽÑ‰85 70?8A8).
func (r *PermissionRepository) GetByUserIDs(ctx context.Context, userIDs []string) (map[string]model.UserPermissions, error) {
	defer logger.DeferLogDuration("permission.GetByUserIDs", time.Now())()
	out := make(map[string]model.UserPermissions, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT user_id, COALESCE(administrator, false), COALESCE(member, true), updated_at
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
		if err := rows.Scan(&p.UserID, &p.Administrator, &p.Member, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("permissionRepo.GetByUserIDs scan: %w", err)
		}
		out[p.UserID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("permissionRepo.GetByUserIDs rows: %w", err)
	}
	return out, nil
}

// Upsert A>740Ñ‘Ñ‚ 8;8 >1=>2;O5Ñ‚ ?Ñ€020 ?>;ÑŒ7>20Ñ‚5;O.
func (r *PermissionRepository) Upsert(ctx context.Context, p *model.UserPermissions) error {
	defer logger.DeferLogDuration("permission.Upsert", time.Now())()
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_permissions (
			user_id, administrator, member, updated_at
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			administrator = EXCLUDED.administrator,
			member = EXCLUDED.member,
			updated_at = EXCLUDED.updated_at`,
		p.UserID,
		p.Administrator,
		p.Member,
		now,
	)
	if err != nil {
		return fmt.Errorf("permissionRepo.Upsert: %w", err)
	}
	p.UpdatedAt = now
	return nil
}
