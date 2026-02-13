package push

import (
	"context"
	"time"

	"github.com/messenger/internal/broker"
	"github.com/messenger/internal/logger"
)

type outboxWriter interface {
	Enqueue(ctx context.Context, topic, eventKey string, payload any) error
}

// OutboxNotifier writes push notifications to durable outbox.
type OutboxNotifier struct {
	outbox outboxWriter
}

func NewOutboxNotifier(outbox outboxWriter) *OutboxNotifier {
	return &OutboxNotifier{outbox: outbox}
}

func (n *OutboxNotifier) Notify(ctx context.Context, userID, title, body string, data map[string]string) {
	if n == nil || n.outbox == nil || userID == "" {
		return
	}
	evt := broker.PushNotifyPayload{
		UserID: userID,
		Title:  title,
		Body:   body,
		Data:   data,
	}
	writeCtx := ctx
	if writeCtx == nil {
		writeCtx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(writeCtx, 5*time.Second)
	defer cancel()
	if err := n.outbox.Enqueue(writeCtx, broker.TopicPushNotify, userID, evt); err != nil {
		logger.Errorf("push outbox enqueue user=%s: %v", userID, err)
	}
}
