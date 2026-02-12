package memory

import (
	"context"
	"sync"
	"time"
)

const (
	otpTTL             = 300 * time.Second
	otpRateLimitWindow = 600 * time.Second
	otpRateLimitMax    = 10
	sessionSecretTTL   = 30 * 24 * time.Hour
)

type item struct {
	val string
	exp time.Time
}

type Client struct {
	mu      sync.RWMutex
	otp     map[string]item
	limit   map[string][]time.Time
	secrets map[string]item
}

func New() *Client {
	return &Client{
		otp:     make(map[string]item),
		limit:   make(map[string][]time.Time),
		secrets: make(map[string]item),
	}
}

func (c *Client) Close() error { return nil }

func (c *Client) SetOTP(ctx context.Context, email, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.otp[email] = item{val: code, exp: time.Now().Add(otpTTL)}
	return nil
}

func (c *Client) GetOTP(ctx context.Context, email string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.otp[email]
	if !ok || time.Now().After(v.exp) {
		return "", nil
	}
	return v.val, nil
}

func (c *Client) GetOTPTTL(ctx context.Context, email string) (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.otp[email]
	if !ok || time.Now().After(v.exp) {
		return 0, nil
	}
	d := time.Until(v.exp)
	if d < 0 {
		return 0, nil
	}
	return d, nil
}

func (c *Client) DeleteOTP(ctx context.Context, email string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.otp, email)
	return nil
}

func (c *Client) CheckRateLimit(ctx context.Context, email string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	cut := now.Add(-otpRateLimitWindow)
	slice := c.limit[email]
	var kept []time.Time
	for _, t := range slice {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= otpRateLimitMax {
		return false, nil
	}
	kept = append(kept, now)
	c.limit[email] = kept
	return true, nil
}

func (c *Client) SetSessionSecret(ctx context.Context, sessionID, secret string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.secrets[sessionID] = item{val: secret, exp: time.Now().Add(sessionSecretTTL)}
	return nil
}

func (c *Client) GetSessionSecret(ctx context.Context, sessionID string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.secrets[sessionID]
	if !ok || time.Now().After(v.exp) {
		return "", nil
	}
	return v.val, nil
}

func (c *Client) DeleteSessionSecret(ctx context.Context, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.secrets, sessionID)
	return nil
}
