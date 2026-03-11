package push

import (
	"context"
	"time"

	"github.com/pulse/internal/broker"
	"github.com/pulse/internal/logger"
)

type brokerOutboxWriter interface {
	Enqueue(ctx context.Context, topic, eventKey string, payload any) error
}

// BrokerClient publishes push operations to durable outbox.
// It is used by API handlers and WS hub to avoid direct sync service calls.
type BrokerClient struct {
	outbox brokerOutboxWriter
}

func NewBrokerClient(outbox brokerOutboxWriter) *BrokerClient {
	return &BrokerClient{outbox: outbox}
}

func (c *BrokerClient) Subscribe(ctx context.Context, userID string, sub PushSubscription) error {
	if c == nil || c.outbox == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil {
		writeCtx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(writeCtx, 5*time.Second)
	defer cancel()
	payload := broker.PushSubscribePayload{
		UserID: userID,
		Subscription: broker.PushSubscription{
			Endpoint: sub.Endpoint,
		},
	}
	payload.Subscription.Keys.P256dh = sub.Keys.P256dh
	payload.Subscription.Keys.Auth = sub.Keys.Auth
	return c.outbox.Enqueue(writeCtx, broker.TopicPushSubscribe, userID, payload)
}

func (c *BrokerClient) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	if c == nil || c.outbox == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil {
		writeCtx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(writeCtx, 5*time.Second)
	defer cancel()
	return c.outbox.Enqueue(writeCtx, broker.TopicPushUnsubscribe, userID, broker.PushUnsubscribePayload{
		UserID:   userID,
		Endpoint: endpoint,
	})
}

func (c *BrokerClient) Notify(ctx context.Context, userID, title, body string, data map[string]string) {
	if c == nil || c.outbox == nil || userID == "" {
		return
	}
	writeCtx := ctx
	if writeCtx == nil {
		writeCtx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(writeCtx, 5*time.Second)
	defer cancel()
	if err := c.outbox.Enqueue(writeCtx, broker.TopicPushNotify, userID, broker.PushNotifyPayload{
		UserID: userID,
		Title:  title,
		Body:   body,
		Data:   data,
	}); err != nil {
		logger.Errorf("push broker enqueue user=%s: %v", userID, err)
	}
}
