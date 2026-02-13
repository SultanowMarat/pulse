package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/messenger/internal/logger"
)

const outboxLockTTL = 30 * time.Second

type OutboxRepository struct {
	pool *pgxpool.Pool
}

type OutboxEvent struct {
	ID       string
	Topic    string
	EventKey string
	Payload  []byte
	Attempts int
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) Enqueue(ctx context.Context, topic, eventKey string, payload any) error {
	defer logger.DeferLogDuration("outbox.Enqueue", time.Now())()
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("outbox marshal payload: %w", err)
	}
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("outbox topic is empty")
	}
	_, err = r.pool.Exec(
		ctx,
		`INSERT INTO outbox_events (id, topic, event_key, payload, available_at, created_at)
		 VALUES ($1, $2, $3, $4::jsonb, NOW(), NOW())`,
		uuid.NewString(), topic, eventKey, raw,
	)
	if err != nil {
		return fmt.Errorf("outbox enqueue: %w", err)
	}
	return nil
}

func (r *OutboxRepository) ClaimPending(ctx context.Context, owner string, limit int) ([]OutboxEvent, error) {
	defer logger.DeferLogDuration("outbox.ClaimPending", time.Now())()
	if limit <= 0 {
		limit = 50
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("outbox begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rows, err := tx.Query(ctx,
		`WITH picked AS (
			SELECT id
			FROM outbox_events
			WHERE published_at IS NULL
			  AND available_at <= NOW()
			  AND (locked_at IS NULL OR locked_at < NOW() - ($3 * INTERVAL '1 second'))
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events o
		SET locked_at = NOW(),
		    lock_owner = $2
		FROM picked
		WHERE o.id = picked.id
		RETURNING o.id, o.topic, o.event_key, o.payload, o.attempts`,
		limit, owner, int(outboxLockTTL.Seconds()),
	)
	if err != nil {
		return nil, fmt.Errorf("outbox claim query: %w", err)
	}
	defer rows.Close()

	events := make([]OutboxEvent, 0, limit)
	for rows.Next() {
		var evt OutboxEvent
		if err := rows.Scan(&evt.ID, &evt.Topic, &evt.EventKey, &evt.Payload, &evt.Attempts); err != nil {
			return nil, fmt.Errorf("outbox claim scan: %w", err)
		}
		events = append(events, evt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox claim rows: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("outbox claim commit: %w", err)
	}
	return events, nil
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	defer logger.DeferLogDuration("outbox.MarkPublished", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE outbox_events
		 SET published_at = NOW(),
		     locked_at = NULL,
		     lock_owner = '',
		     last_error = ''
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("outbox mark published: %w", err)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id, lastError string, nextAttemptAt time.Time) error {
	defer logger.DeferLogDuration("outbox.MarkFailed", time.Now())()
	lastError = strings.TrimSpace(lastError)
	if len(lastError) > 800 {
		lastError = lastError[:800]
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE outbox_events
		 SET attempts = attempts + 1,
		     last_error = $2,
		     available_at = $3,
		     locked_at = NULL,
		     lock_owner = ''
		 WHERE id = $1`,
		id, lastError, nextAttemptAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("outbox mark failed: %w", err)
	}
	return nil
}
