package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// OTP TTL 5 минут (время на ввод кода); rate limit 10 запросов / 10 минут на email.
const (
	OTPTTL             = 300
	OTPRateLimitWindow = 600  // 10 минут
	OTPRateLimitMax    = 10   // запросов кода за окно
	SessionSecretTTL   = 30 * 24 * 3600
)

type Client struct {
	cli *redis.Client
}

func New(ctx context.Context, url string) (*Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis parse url: %w", err)
	}
	cli := redis.NewClient(opts)
	if err := cli.Ping(ctx).Err(); err != nil {
		if closeErr := cli.Close(); closeErr != nil {
			return nil, fmt.Errorf("redis ping: %w (close: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

// SetOTP сохраняет код (6 цифр) по ключу otp:{email}, TTL 5 мин. Храним код как есть для надёжной верификации.
func (c *Client) SetOTP(ctx context.Context, email, code string) error {
	return c.cli.Set(ctx, "otp:"+email, code, OTPTTL*time.Second).Err()
}

// GetOTP возвращает код по email (ключ не удаляется — удаляем только после успешной верификации).
func (c *Client) GetOTP(ctx context.Context, email string) (string, error) {
	val, err := c.cli.Get(ctx, "otp:"+email).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// GetOTPTTL возвращает оставшийся TTL ключа OTP. Если ключа нет, возвращает 0.
func (c *Client) GetOTPTTL(ctx context.Context, email string) (time.Duration, error) {
	d, err := c.cli.TTL(ctx, "otp:"+email).Result()
	if err != nil || d < 0 {
		return 0, err
	}
	return d, nil
}

// DeleteOTP удаляет OTP после успешной верификации (одноразовое использование кода).
func (c *Client) DeleteOTP(ctx context.Context, email string) error {
	return c.cli.Del(ctx, "otp:"+email).Err()
}

// CheckRateLimit проверяет otp_limit:{email}: макс. OTPRateLimitMax запросов за окно. При превышении — HTTP 429.
func (c *Client) CheckRateLimit(ctx context.Context, email string) (allowed bool, err error) {
	key := "otp_limit:" + email
	n, err := c.cli.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		c.cli.Expire(ctx, key, OTPRateLimitWindow*time.Second)
	}
	return n <= int64(OTPRateLimitMax), nil
}

func (c *Client) SetSessionSecret(ctx context.Context, sessionID, secret string) error {
	return c.cli.Set(ctx, "session_secret:"+sessionID, secret, SessionSecretTTL*time.Second).Err()
}

func (c *Client) GetSessionSecret(ctx context.Context, sessionID string) (string, error) {
	val, err := c.cli.Get(ctx, "session_secret:"+sessionID).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (c *Client) DeleteSessionSecret(ctx context.Context, sessionID string) error {
	return c.cli.Del(ctx, "session_secret:"+sessionID).Err()
}

// FlushDB очищает текущую БД Redis (для сброса OTP, rate limit, session_secret при тестах/перезапуске).
func (c *Client) FlushDB(ctx context.Context) error {
	return c.cli.FlushDB(ctx).Err()
}
