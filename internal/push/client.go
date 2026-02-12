package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/messenger/internal/logger"
)

// Client вызывает микросервис пуш-уведомлений. Если URL пустой — методы no-op.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient создаёт клиент. baseURL пустой — пуши отключены.
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

// SubscribeRequest — тело запроса подписки.
type SubscribeRequest struct {
	UserID       string          `json:"user_id"`
	Subscription PushSubscription `json:"subscription"`
}

// PushSubscription — подписка из браузера.
type PushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Subscribe сохраняет подписку для user_id на push-сервисе.
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

// Unsubscribe удаляет подписку по endpoint.
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

// NotifyRequest — запрос на отправку уведомления.
type NotifyRequest struct {
	UserID string            `json:"user_id"`
	Title  string            `json:"title"`
	Body   string            `json:"body"`
	Data   map[string]string `json:"data,omitempty"`
}

// Notify отправляет пуш пользователю (вызывается из API при новом сообщении и т.п.).
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
