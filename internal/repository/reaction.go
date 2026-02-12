package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/model"
)

type ReactionRepository struct {
	pool *pgxpool.Pool
}

func NewReactionRepository(pool *pgxpool.Pool) *ReactionRepository {
	return &ReactionRepository{pool: pool}
}

func (r *ReactionRepository) Add(ctx context.Context, messageID, userID, emoji string) error {
	defer logger.DeferLogDuration("reaction.Add", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO message_reactions (message_id, user_id, emoji)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		messageID, userID, emoji,
	)
	if err != nil {
		return fmt.Errorf("reactionRepo.Add: %w", err)
	}
	return nil
}

func (r *ReactionRepository) Remove(ctx context.Context, messageID, userID, emoji string) error {
	defer logger.DeferLogDuration("reaction.Remove", time.Now())()
	_, err := r.pool.Exec(ctx,
		`DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`,
		messageID, userID, emoji,
	)
	if err != nil {
		return fmt.Errorf("reactionRepo.Remove: %w", err)
	}
	return nil
}

func (r *ReactionRepository) GetByMessage(ctx context.Context, messageID string) ([]model.Reaction, error) {
	defer logger.DeferLogDuration("reaction.GetByMessage", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT mr.message_id, mr.user_id, mr.emoji, u.username, mr.created_at
		 FROM message_reactions mr
		 JOIN users u ON u.id = mr.user_id
		 WHERE mr.message_id = $1
		 ORDER BY mr.created_at`, messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("reactionRepo.GetByMessage query: %w", err)
	}
	defer rows.Close()

	reactions := make([]model.Reaction, 0, 8)
	for rows.Next() {
		var rc model.Reaction
		if err := rows.Scan(&rc.MessageID, &rc.UserID, &rc.Emoji, &rc.Username, &rc.CreatedAt); err != nil {
			return nil, fmt.Errorf("reactionRepo.GetByMessage scan: %w", err)
		}
		reactions = append(reactions, rc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reactionRepo.GetByMessage rows: %w", err)
	}
	return reactions, nil
}

// GetGroupedByMessage returns aggregated reaction groups for a message.
func (r *ReactionRepository) GetGroupedByMessage(ctx context.Context, messageID string) ([]model.ReactionGroup, error) {
	defer logger.DeferLogDuration("reaction.GetGroupedByMessage", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT emoji, COUNT(*), array_agg(user_id::text)
		 FROM message_reactions
		 WHERE message_id = $1
		 GROUP BY emoji
		 ORDER BY MIN(created_at)`, messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("reactionRepo.GetGroupedByMessage query: %w", err)
	}
	defer rows.Close()

	groups := make([]model.ReactionGroup, 0, 4)
	for rows.Next() {
		var g model.ReactionGroup
		if err := rows.Scan(&g.Emoji, &g.Count, &g.Users); err != nil {
			return nil, fmt.Errorf("reactionRepo.GetGroupedByMessage scan: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reactionRepo.GetGroupedByMessage rows: %w", err)
	}
	return groups, nil
}
