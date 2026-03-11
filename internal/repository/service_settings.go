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

// ServiceSettingsRepository stores admin-editable service settings (id=1).
type ServiceSettingsRepository struct {
	pool *pgxpool.Pool
	cfg  *config.Config
}

func NewServiceSettingsRepository(pool *pgxpool.Pool, cfg *config.Config) *ServiceSettingsRepository {
	return &ServiceSettingsRepository{pool: pool, cfg: cfg}
}

func (r *ServiceSettingsRepository) defaults() *model.ServiceSettings {
	// These defaults are chosen to be safe for 2000+ concurrent users on typical setups.
	return &model.ServiceSettings{
		Maintenance:        r.cfg.AppMaintenance,
		ReadOnly:           r.cfg.AppReadOnly,
		Degradation:        r.cfg.AppDegradation,
		StatusMessage:      strings.TrimSpace(r.cfg.AppStatusMessage),
		CORSAllowedOrigins: strings.TrimSpace(r.cfg.CORSAllowedOrigins),
		InstallWindowsURL:  "",
		InstallAndroidURL:  "",
		InstallMacOSURL:    "",
		InstallIOSURL:      "",
		MaxWSConnections:   r.cfg.MaxWSConnections,
		WSSendBufferSize:   r.cfg.WSSendBufferSize,
		WSWriteTimeout:     r.cfg.WSWriteTimeout,
		WSPongTimeout:      r.cfg.WSPongTimeout,
		WSMaxMessageSize:   r.cfg.WSMaxMessageSize,
	}
}

func (r *ServiceSettingsRepository) Get(ctx context.Context) (*model.ServiceSettings, error) {
	defer logger.DeferLogDuration("serviceSettings.Get", time.Now())()
	def := r.defaults()
	s := *def
	err := r.pool.QueryRow(ctx,
		`SELECT maintenance, read_only, degradation, status_message,
		        cors_allowed_origins,
		        install_windows_url, install_android_url, install_macos_url, install_ios_url,
		        max_ws_connections, ws_send_buffer_size, ws_write_timeout, ws_pong_timeout, ws_max_message_size,
		        updated_at
		   FROM app_service_settings WHERE id = 1`,
	).Scan(
		&s.Maintenance, &s.ReadOnly, &s.Degradation, &s.StatusMessage,
		&s.CORSAllowedOrigins,
		&s.InstallWindowsURL, &s.InstallAndroidURL, &s.InstallMacOSURL, &s.InstallIOSURL,
		&s.MaxWSConnections, &s.WSSendBufferSize, &s.WSWriteTimeout, &s.WSPongTimeout, &s.WSMaxMessageSize,
		&s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// Not created yet; return defaults (caller may choose to Upsert).
		return def, nil
	}
	if err != nil {
		return nil, fmt.Errorf("serviceSettingsRepo.Get: %w", err)
	}

	// Basic sanity.
	if s.MaxWSConnections <= 0 {
		s.MaxWSConnections = def.MaxWSConnections
	}
	if s.WSSendBufferSize <= 0 {
		s.WSSendBufferSize = def.WSSendBufferSize
	}
	if s.WSWriteTimeout <= 0 {
		s.WSWriteTimeout = def.WSWriteTimeout
	}
	if s.WSPongTimeout <= 0 {
		s.WSPongTimeout = def.WSPongTimeout
	}
	if s.WSMaxMessageSize <= 0 {
		s.WSMaxMessageSize = def.WSMaxMessageSize
	}
	if strings.TrimSpace(s.CORSAllowedOrigins) == "" {
		s.CORSAllowedOrigins = def.CORSAllowedOrigins
	}
	s.InstallWindowsURL = strings.TrimSpace(s.InstallWindowsURL)
	s.InstallAndroidURL = strings.TrimSpace(s.InstallAndroidURL)
	s.InstallMacOSURL = strings.TrimSpace(s.InstallMacOSURL)
	s.InstallIOSURL = strings.TrimSpace(s.InstallIOSURL)
	s.StatusMessage = strings.TrimSpace(s.StatusMessage)
	return &s, nil
}

func (r *ServiceSettingsRepository) Upsert(ctx context.Context, s *model.ServiceSettings) (*model.ServiceSettings, error) {
	defer logger.DeferLogDuration("serviceSettings.Upsert", time.Now())()
	if s == nil {
		s = r.defaults()
	}
	def := r.defaults()

	cors := strings.TrimSpace(s.CORSAllowedOrigins)
	if cors == "" {
		cors = def.CORSAllowedOrigins
	}

	maxWS := s.MaxWSConnections
	if maxWS <= 0 {
		maxWS = def.MaxWSConnections
	}
	sendBuf := s.WSSendBufferSize
	if sendBuf <= 0 {
		sendBuf = def.WSSendBufferSize
	}
	writeTO := s.WSWriteTimeout
	if writeTO <= 0 {
		writeTO = def.WSWriteTimeout
	}
	pongTO := s.WSPongTimeout
	if pongTO <= 0 {
		pongTO = def.WSPongTimeout
	}
	maxMsg := s.WSMaxMessageSize
	if maxMsg <= 0 {
		maxMsg = def.WSMaxMessageSize
	}

	winURL := strings.TrimSpace(s.InstallWindowsURL)
	andURL := strings.TrimSpace(s.InstallAndroidURL)
	macURL := strings.TrimSpace(s.InstallMacOSURL)
	iosURL := strings.TrimSpace(s.InstallIOSURL)

	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO app_service_settings (
			 id, maintenance, read_only, degradation, status_message,
			 cors_allowed_origins,
			 install_windows_url, install_android_url, install_macos_url, install_ios_url,
			 max_ws_connections, ws_send_buffer_size, ws_write_timeout, ws_pong_timeout, ws_max_message_size,
			 updated_at
		 )
		 VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		 ON CONFLICT (id) DO UPDATE SET
		   maintenance = EXCLUDED.maintenance,
		   read_only = EXCLUDED.read_only,
		   degradation = EXCLUDED.degradation,
		   status_message = EXCLUDED.status_message,
		   cors_allowed_origins = EXCLUDED.cors_allowed_origins,
		   install_windows_url = EXCLUDED.install_windows_url,
		   install_android_url = EXCLUDED.install_android_url,
		   install_macos_url = EXCLUDED.install_macos_url,
		   install_ios_url = EXCLUDED.install_ios_url,
		   max_ws_connections = EXCLUDED.max_ws_connections,
		   ws_send_buffer_size = EXCLUDED.ws_send_buffer_size,
		   ws_write_timeout = EXCLUDED.ws_write_timeout,
		   ws_pong_timeout = EXCLUDED.ws_pong_timeout,
		   ws_max_message_size = EXCLUDED.ws_max_message_size,
		   updated_at = EXCLUDED.updated_at`,
		s.Maintenance, s.ReadOnly, s.Degradation, strings.TrimSpace(s.StatusMessage),
		cors,
		winURL, andURL, macURL, iosURL,
		maxWS, sendBuf, writeTO, pongTO, maxMsg,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("serviceSettingsRepo.Upsert: %w", err)
	}

	return &model.ServiceSettings{
		Maintenance:        s.Maintenance,
		ReadOnly:           s.ReadOnly,
		Degradation:        s.Degradation,
		StatusMessage:      strings.TrimSpace(s.StatusMessage),
		CORSAllowedOrigins: cors,
		InstallWindowsURL:  winURL,
		InstallAndroidURL:  andURL,
		InstallMacOSURL:    macURL,
		InstallIOSURL:      iosURL,
		MaxWSConnections:   maxWS,
		WSSendBufferSize:   sendBuf,
		WSWriteTimeout:     writeTO,
		WSPongTimeout:      pongTO,
		WSMaxMessageSize:   maxMsg,
		UpdatedAt:          now,
	}, nil
}
