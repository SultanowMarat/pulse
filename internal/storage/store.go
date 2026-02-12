package storage

import (
	"context"
	"time"
)

// SessionOTPStore — хранилище OTP-кодов, rate limit и session_secret.
// Реализации: redis.Client, memory.Client (для -dev без Redis).
type SessionOTPStore interface {
	SetOTP(ctx context.Context, email, code string) error
	GetOTP(ctx context.Context, email string) (string, error)
	GetOTPTTL(ctx context.Context, email string) (time.Duration, error)
	DeleteOTP(ctx context.Context, email string) error
	CheckRateLimit(ctx context.Context, email string) (allowed bool, err error)
	SetSessionSecret(ctx context.Context, sessionID, secret string) error
	GetSessionSecret(ctx context.Context, sessionID string) (string, error)
	DeleteSessionSecret(ctx context.Context, sessionID string) error
	Close() error
}
