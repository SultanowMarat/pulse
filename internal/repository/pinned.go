package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/model"
)

type PinnedRepository struct {
	pool *pgxpool.Pool
}

func NewPinnedRepository(pool *pgxpool.Pool) *PinnedRepository {
	return &PinnedRepository{pool: pool}
}

func (r *PinnedRepository) Pin(ctx context.Context, chatID, messageID, pinnedBy string) error {
	defer logger.DeferLogDuration("pinned.Pin", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO pinned_messages (chat_id, message_id, pinned_by, pinned_at)
		 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		chatID, messageID, pinnedBy, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("pinnedRepo.Pin: %w", err)
	}
	return nil
}

func (r *PinnedRepository) Unpin(ctx context.Context, chatID, messageID string) error {
	defer logger.DeferLogDuration("pinned.Unpin", time.Now())()
	_, err := r.pool.Exec(ctx,
		`DELETE FROM pinned_messages WHERE chat_id = $1 AND message_id = $2`,
		chatID, messageID,
	)
	if err != nil {
		return fmt.Errorf("pinnedRepo.Unpin: %w", err)
	}
	return nil
}

// GetPinnedForUser returns pinned messages visible to a specific user.
// Visibility follows the same rule as message listing: messages created at/before member cleared_at are hidden.
func (r *PinnedRepository) GetPinnedForUser(ctx context.Context, chatID, userID string) ([]model.PinnedMessage, error) {
	defer logger.DeferLogDuration("pinned.GetPinnedForUser", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT pm.chat_id, pm.message_id, pm.pinned_by, pm.pinned_at,
		        m.id, m.sender_id, m.content, m.content_type, m.created_at,
		        u.id, u.username, u.avatar_url
		 FROM pinned_messages pm
		 JOIN messages m ON m.id = pm.message_id
		 JOIN users u ON u.id = m.sender_id
		 JOIN chat_members cm ON cm.chat_id = pm.chat_id AND cm.user_id = $2
		 WHERE pm.chat_id = $1 AND m.created_at > cm.cleared_at
		 ORDER BY pm.pinned_at DESC`, chatID,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("pinnedRepo.GetPinnedForUser query: %w", err)
	}
	defer rows.Close()

	pins := make([]model.PinnedMessage, 0, 4)
	for rows.Next() {
		var p model.PinnedMessage
		msg := &model.Message{}
		sender := &model.UserPublic{}
		if err := rows.Scan(&p.ChatID, &p.MessageID, &p.PinnedBy, &p.PinnedAt,
			&msg.ID, &msg.SenderID, &msg.Content, &msg.ContentType, &msg.CreatedAt,
			&sender.ID, &sender.Username, &sender.AvatarURL); err != nil {
			return nil, fmt.Errorf("pinnedRepo.GetPinnedForUser scan: %w", err)
		}
		msg.Sender = sender
		p.Message = msg
		pins = append(pins, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pinnedRepo.GetPinnedForUser rows: %w", err)
	}
	return pins, nil
}

// NOTE: keep repository surface minimal; "is pinned" is currently unused by the server.
