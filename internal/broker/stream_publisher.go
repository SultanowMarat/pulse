package broker

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const defaultMaxStreamLen = 100000

// StreamPublisher publishes events to Redis Stream.
type StreamPublisher struct {
	redis  *redis.Client
	stream string
	maxLen int64
}

// NewStreamPublisher creates a stream publisher.
// It does not require Redis to be available at startup: go-redis reconnects on publish.
func NewStreamPublisher(ctx context.Context, redisURL, stream string) (*StreamPublisher, error) {
	_ = ctx
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is empty")
	}
	if stream == "" {
		stream = PushStreamName
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis parse url: %w", err)
	}
	cli := redis.NewClient(opts)

	return &StreamPublisher{
		redis:  cli,
		stream: stream,
		maxLen: defaultMaxStreamLen,
	}, nil
}

// Publish pushes one event to stream.
func (p *StreamPublisher) Publish(ctx context.Context, eventID, topic, eventKey string, payload []byte) error {
	if p == nil || p.redis == nil {
		return fmt.Errorf("stream publisher is nil")
	}
	values := map[string]any{
		"event_id":  eventID,
		"topic":     topic,
		"event_key": eventKey,
		"payload":   string(payload),
	}
	if err := p.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		MaxLen: p.maxLen,
		Approx: true,
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("xadd: %w", err)
	}
	return nil
}

func (p *StreamPublisher) Close() error {
	if p == nil || p.redis == nil {
		return nil
	}
	return p.redis.Close()
}
