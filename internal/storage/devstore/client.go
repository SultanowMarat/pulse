package devstore

import (
	"context"
	"time"

	"github.com/messenger/internal/repository"
	"github.com/messenger/internal/storage/memory"
)

// Client реализует SessionOTPStore для режима -dev: OTP и rate limit в памяти,
// session_secret в БД — сессии переживают перезапуск Auth.
type Client struct {
	mem  *memory.Client
	repo *repository.SessionRepository
}

func New(repo *repository.SessionRepository) *Client {
	return &Client{mem: memory.New(), repo: repo}
}

func (c *Client) Close() error { return c.mem.Close() }

func (c *Client) SetOTP(ctx context.Context, email, code string) error {
	return c.mem.SetOTP(ctx, email, code)
}
func (c *Client) GetOTP(ctx context.Context, email string) (string, error) {
	return c.mem.GetOTP(ctx, email)
}
func (c *Client) GetOTPTTL(ctx context.Context, email string) (time.Duration, error) {
	return c.mem.GetOTPTTL(ctx, email)
}
func (c *Client) DeleteOTP(ctx context.Context, email string) error {
	return c.mem.DeleteOTP(ctx, email)
}
func (c *Client) CheckRateLimit(ctx context.Context, email string) (bool, error) {
	return c.mem.CheckRateLimit(ctx, email)
}

func (c *Client) SetSessionSecret(ctx context.Context, sessionID, secret string) error {
	return c.repo.SetSessionSecret(ctx, sessionID, secret)
}
func (c *Client) GetSessionSecret(ctx context.Context, sessionID string) (string, error) {
	return c.repo.GetSessionSecret(ctx, sessionID)
}
func (c *Client) DeleteSessionSecret(ctx context.Context, sessionID string) error {
	return c.repo.ClearSessionSecret(ctx, sessionID)
}
