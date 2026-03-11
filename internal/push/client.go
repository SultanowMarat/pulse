package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pulse/internal/logger"
)

// Client 2Ñ‹7Ñ‹205Ñ‚ <8:Ñ€>A5Ñ€28A ?ÑƒÑˆ-Ñƒ254><;5=89. Ð•A;8 URL ?ÑƒAÑ‚>9 â€” <5Ñ‚>4Ñ‹ no-op.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient A>740Ñ‘Ñ‚ :;85=Ñ‚. baseURL ?ÑƒAÑ‚>9 â€” ?ÑƒÑˆ8 >Ñ‚:;ÑŽÑ‡5=Ñ‹.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		return &Client{}
	}
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SubscribeRequest â€” Ñ‚5;> 70?Ñ€>A0 ?>4?8A:8.
type SubscribeRequest struct {
	UserID       string          `json:"user_id"`
	Subscription PushSubscription `json:"subscription"`
}

// PushSubscription â€” ?>4?8A:0 87 1Ñ€0Ñƒ75Ñ€0.
type PushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Subscribe A>Ñ…Ñ€0=O5Ñ‚ ?>4?8A:Ñƒ 4;O user_id =0 push-A5Ñ€28A5.
func (c *Client) Subscribe(ctx context.Context, userID string, sub PushSubscription) error {
	if c.baseURL == "" {
		return nil
	}
	body, err := json.Marshal(SubscribeRequest{UserID: userID, Subscription: sub})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/subscribe", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("push subscribe: %d", resp.StatusCode)
	}
	return nil
}

// Unsubscribe Ñƒ40;O5Ñ‚ ?>4?8A:Ñƒ ?> endpoint.
func (c *Client) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	if c.baseURL == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"user_id": userID, "endpoint": endpoint})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/subscribe", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("push unsubscribe: %d", resp.StatusCode)
	}
	return nil
}

// NotifyRequest â€” 70?Ñ€>A =0 >Ñ‚?Ñ€02:Ñƒ Ñƒ254><;5=8O.
type NotifyRequest struct {
	UserID string            `json:"user_id"`
	Title  string            `json:"title"`
	Body   string            `json:"body"`
	Data   map[string]string `json:"data,omitempty"`
}

// Notify >Ñ‚?Ñ€02;O5Ñ‚ ?ÑƒÑˆ ?>;ÑŒ7>20Ñ‚5;ÑŽ (2Ñ‹7Ñ‹205Ñ‚AO 87 API ?Ñ€8 =>2>< A>>1Ñ‰5=88 8 Ñ‚.?.).
func (c *Client) Notify(ctx context.Context, userID, title, body string, data map[string]string) {
	if c.baseURL == "" {
		return
	}
	payload := NotifyRequest{UserID: userID, Title: title, Body: body, Data: data}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/notify", bytes.NewReader(bodyBytes))
	if err != nil {
		logger.Errorf("push notify request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Errorf("push notify: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		logger.Errorf("push notify: %d", resp.StatusCode)
	}
}
