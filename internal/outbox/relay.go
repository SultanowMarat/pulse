package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/repository"
)

type EventPublisher interface {
	Publish(ctx context.Context, eventID, topic, eventKey string, payload []byte) error
}

type Relay struct {
	repo         *repository.OutboxRepository
	publisher    EventPublisher
	owner        string
	pollInterval time.Duration
	batchSize    int
}

func NewRelay(repo *repository.OutboxRepository, publisher EventPublisher, owner string) *Relay {
	if owner == "" {
		owner = "api-relay"
	}
	return &Relay{
		repo:         repo,
		publisher:    publisher,
		owner:        owner,
		pollInterval: 300 * time.Millisecond,
		batchSize:    50,
	}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return
		}
		processed, err := r.flushOnce(ctx)
		if err != nil {
			logger.Errorf("outbox relay flush: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
			continue
		}
		if processed == 0 {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}
}

func (r *Relay) flushOnce(ctx context.Context) (int, error) {
	events, err := r.repo.ClaimPending(ctx, r.owner, r.batchSize)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}

	for _, evt := range events {
		if err := r.publisher.Publish(ctx, evt.ID, evt.Topic, evt.EventKey, evt.Payload); err != nil {
			nextRetry := time.Now().UTC().Add(retryDelay(evt.Attempts))
			if markErr := r.repo.MarkFailed(ctx, evt.ID, fmt.Sprintf("publish: %v", err), nextRetry); markErr != nil {
				logger.Errorf("outbox relay mark failed id=%s: %v", evt.ID, markErr)
			}
			continue
		}
		if err := r.repo.MarkPublished(ctx, evt.ID); err != nil {
			// Message is already in stream; keep retrying mark to avoid event loss.
			nextRetry := time.Now().UTC().Add(retryDelay(evt.Attempts))
			if markErr := r.repo.MarkFailed(ctx, evt.ID, fmt.Sprintf("mark published: %v", err), nextRetry); markErr != nil {
				logger.Errorf("outbox relay mark-failed-after-publish id=%s: %v", evt.ID, markErr)
			}
		}
	}
	return len(events), nil
}

func retryDelay(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 7 {
		attempts = 7
	}
	d := time.Second << attempts
	if d > 2*time.Minute {
		d = 2 * time.Minute
	}
	return d
}
