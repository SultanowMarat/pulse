package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse/internal/config"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/model"
)

// MailSettingsRepository Ñ…Ñ€0=8Ñ‚ 548=Ñ‹9 =01>Ñ€ SMTP-=0AÑ‚Ñ€>5: (id=1).
type MailSettingsRepository struct {
	pool *pgxpool.Pool
}

func NewMailSettingsRepository(pool *pgxpool.Pool) *MailSettingsRepository {
	return &MailSettingsRepository{pool: pool}
}

func (r *MailSettingsRepository) Get(ctx context.Context) (*model.MailSettings, error) {
	defer logger.DeferLogDuration("mailSettings.Get", time.Now())()
	ms := &model.MailSettings{}
	err := r.pool.QueryRow(ctx,
		`SELECT host, port, username, password, from_email, from_name, updated_at
		 FROM app_mail_settings WHERE id = 1`,
	).Scan(&ms.Host, &ms.Port, &ms.Username, &ms.Password, &ms.FromEmail, &ms.FromName, &ms.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &model.MailSettings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mailSettingsRepo.Get: %w", err)
	}
	return ms, nil
}

func (r *MailSettingsRepository) Upsert(ctx context.Context, ms *model.MailSettings) error {
	defer logger.DeferLogDuration("mailSettings.Upsert", time.Now())()
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO app_mail_settings (id, host, port, username, password, from_email, from_name, updated_at)
		 VALUES (1, $1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (id) DO UPDATE SET
		   host = EXCLUDED.host,
		   port = EXCLUDED.port,
		   username = EXCLUDED.username,
		   password = EXCLUDED.password,
		   from_email = EXCLUDED.from_email,
		   from_name = EXCLUDED.from_name,
		   updated_at = EXCLUDED.updated_at`,
		strings.TrimSpace(ms.Host), ms.Port, strings.TrimSpace(ms.Username), ms.Password,
		strings.TrimSpace(ms.FromEmail), strings.TrimSpace(ms.FromName), now,
	)
	if err != nil {
		return fmt.Errorf("mailSettingsRepo.Upsert: %w", err)
	}
	ms.UpdatedAt = now
	return nil
}

func (r *MailSettingsRepository) GetSMTPConfig(ctx context.Context) (*config.SMTPConfig, error) {
	ms, err := r.Get(ctx)
	if err != nil {
		return nil, err
	}
	if ms == nil || !ms.IsConfigured() {
		return nil, nil
	}
	cfg := &config.SMTPConfig{
		Host:      ms.Host,
		Port:      ms.Port,
		Username:  ms.Username,
		Password:  ms.Password,
		FromEmail: ms.FromEmail,
		FromName:  ms.FromName,
	}
	return cfg, nil
}
