package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse/internal/logger"
)

type FileSettings struct {
	MaxFileSizeMB int       `json:"max_file_size_mb"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type FileSettingsRepository struct {
	pool *pgxpool.Pool
}

func NewFileSettingsRepository(pool *pgxpool.Pool) *FileSettingsRepository {
	return &FileSettingsRepository{pool: pool}
}

func (r *FileSettingsRepository) Get(ctx context.Context, defaultMB int) (*FileSettings, error) {
	defer logger.DeferLogDuration("fileSettings.Get", time.Now())()
	if defaultMB <= 0 {
		defaultMB = 20
	}
	fs := &FileSettings{MaxFileSizeMB: defaultMB}
	err := r.pool.QueryRow(ctx,
		`SELECT max_file_size_mb, updated_at FROM app_file_settings WHERE id = 1`,
	).Scan(&fs.MaxFileSizeMB, &fs.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return fs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fileSettingsRepo.Get: %w", err)
	}
	if fs.MaxFileSizeMB <= 0 {
		fs.MaxFileSizeMB = defaultMB
	}
	return fs, nil
}

func (r *FileSettingsRepository) Upsert(ctx context.Context, maxFileSizeMB int) (*FileSettings, error) {
	defer logger.DeferLogDuration("fileSettings.Upsert", time.Now())()
	if maxFileSizeMB <= 0 {
		maxFileSizeMB = 20
	}
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO app_file_settings (id, max_file_size_mb, updated_at)
		 VALUES (1, $1, $2)
		 ON CONFLICT (id) DO UPDATE SET
		   max_file_size_mb = EXCLUDED.max_file_size_mb,
		   updated_at = EXCLUDED.updated_at`,
		maxFileSizeMB, now,
	)
	if err != nil {
		return nil, fmt.Errorf("fileSettingsRepo.Upsert: %w", err)
	}
	return &FileSettings{MaxFileSizeMB: maxFileSizeMB, UpdatedAt: now}, nil
}
