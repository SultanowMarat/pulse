package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/model"
)

type MessageRepository struct {
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

func (r *MessageRepository) Create(ctx context.Context, m *model.Message) error {
	defer logger.DeferLogDuration("msg.Create", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO messages (id, chat_id, sender_id, content, content_type, file_url, file_name, file_size, status, reply_to_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		m.ID, m.ChatID, m.SenderID, m.Content, m.ContentType, m.FileURL, m.FileName, m.FileSize, m.Status, m.ReplyToID, m.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("msgRepo.Create: %w", err)
	}
	return nil
}

func (r *MessageRepository) GetByID(ctx context.Context, id string) (*model.Message, error) {
	defer logger.DeferLogDuration("msg.GetByID", time.Now())()
	m := &model.Message{}
	sender := &model.UserPublic{}
	err := r.pool.QueryRow(ctx,
		`SELECT m.id, m.chat_id, m.sender_id, m.content, m.content_type, m.file_url, m.file_name, m.file_size, m.status,
		        m.reply_to_id, m.edited_at, m.is_deleted, m.created_at,
		        u.id, u.username, u.avatar_url, u.is_online, u.last_seen_at
		 FROM messages m
		 JOIN users u ON u.id = m.sender_id
		 WHERE m.id = $1`, id,
	).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.ContentType, &m.FileURL, &m.FileName, &m.FileSize, &m.Status,
		&m.ReplyToID, &m.EditedAt, &m.IsDeleted, &m.CreatedAt,
		&sender.ID, &sender.Username, &sender.AvatarURL, &sender.IsOnline, &sender.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("msgRepo.GetByID: %w", err)
	}
	m.Sender = sender
	return m, nil
}

func (r *MessageRepository) GetChatMessages(ctx context.Context, chatID, userID string, limit, offset int) ([]model.Message, error) {
	defer logger.DeferLogDuration("msg.GetChatMessages", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT m.id, m.chat_id, m.sender_id, m.content, m.content_type, m.file_url, m.file_name, m.file_size, m.status,
		        m.reply_to_id, m.edited_at, m.is_deleted, m.created_at,
		        u.id, u.username, u.avatar_url, u.is_online, u.last_seen_at
		 FROM messages m
		 JOIN users u ON u.id = m.sender_id
		 JOIN chat_members cm ON cm.chat_id = m.chat_id AND cm.user_id = $2
		 WHERE m.chat_id = $1 AND m.created_at > cm.cleared_at
		 ORDER BY m.created_at DESC
		 LIMIT $3 OFFSET $4`, chatID, userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("msgRepo.GetChatMessages query: %w", err)
	}
	defer rows.Close()

	messages := make([]model.Message, 0, limit)
	for rows.Next() {
		var m model.Message
		sender := &model.UserPublic{}
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.ContentType, &m.FileURL, &m.FileName, &m.FileSize, &m.Status,
			&m.ReplyToID, &m.EditedAt, &m.IsDeleted, &m.CreatedAt,
			&sender.ID, &sender.Username, &sender.AvatarURL, &sender.IsOnline, &sender.LastSeenAt); err != nil {
			return nil, fmt.Errorf("msgRepo.GetChatMessages scan: %w", err)
		}
		m.Sender = sender
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("msgRepo.GetChatMessages rows: %w", err)
	}
	return messages, nil
}

func (r *MessageRepository) GetLastMessageForUser(ctx context.Context, chatID, userID string) (*model.Message, error) {
	defer logger.DeferLogDuration("msg.GetLastMessageForUser", time.Now())()
	m := &model.Message{}
	sender := &model.UserPublic{}
	err := r.pool.QueryRow(ctx,
		`SELECT m.id, m.chat_id, m.sender_id, m.content, m.content_type, m.file_url, m.file_name, m.file_size, m.status,
		        m.reply_to_id, m.edited_at, m.is_deleted, m.created_at,
		        u.id, u.username, u.avatar_url, u.is_online, u.last_seen_at
		 FROM messages m
		 JOIN users u ON u.id = m.sender_id
		 JOIN chat_members cm ON cm.chat_id = m.chat_id AND cm.user_id = $2
		 WHERE m.chat_id = $1 AND m.created_at > cm.cleared_at
		 ORDER BY m.created_at DESC
		 LIMIT 1`, chatID, userID,
	).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.ContentType, &m.FileURL, &m.FileName, &m.FileSize, &m.Status,
		&m.ReplyToID, &m.EditedAt, &m.IsDeleted, &m.CreatedAt,
		&sender.ID, &sender.Username, &sender.AvatarURL, &sender.IsOnline, &sender.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("msgRepo.GetLastMessageForUser: %w", err)
	}
	m.Sender = sender
	return m, nil
}

func (r *MessageRepository) MarkAsRead(ctx context.Context, chatID, userID string) error {
	defer logger.DeferLogDuration("msg.MarkAsRead", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE messages SET status = 'read'
		 WHERE chat_id = $1 AND sender_id != $2 AND status != 'read'`,
		chatID, userID,
	)
	if err != nil {
		return fmt.Errorf("msgRepo.MarkAsRead: %w", err)
	}
	return nil
}

// DeleteChatMessages removes all messages for a chat (used when both sides cleared).
func (r *MessageRepository) DeleteChatMessages(ctx context.Context, chatID string) error {
	defer logger.DeferLogDuration("msg.DeleteChatMessages", time.Now())()
	_, err := r.pool.Exec(ctx,
		`DELETE FROM messages WHERE chat_id = $1`, chatID,
	)
	if err != nil {
		return fmt.Errorf("msgRepo.DeleteChatMessages: %w", err)
	}
	return nil
}

// GetDistinctFileURLsByChat returns unique non-empty file URLs for messages in chat.
func (r *MessageRepository) GetDistinctFileURLsByChat(ctx context.Context, chatID string) ([]string, error) {
	defer logger.DeferLogDuration("msg.GetDistinctFileURLsByChat", time.Now())()
	rows, err := r.pool.Query(
		ctx,
		`SELECT DISTINCT file_url
		 FROM messages
		 WHERE chat_id = $1 AND COALESCE(file_url, '') <> ''`,
		chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("msgRepo.GetDistinctFileURLsByChat query: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0, 16)
	for rows.Next() {
		var fileURL string
		if err := rows.Scan(&fileURL); err != nil {
			return nil, fmt.Errorf("msgRepo.GetDistinctFileURLsByChat scan: %w", err)
		}
		out = append(out, fileURL)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("msgRepo.GetDistinctFileURLsByChat rows: %w", err)
	}
	return out, nil
}

// CountByFileURLOutsideChat counts references to file URL outside the given chat.
func (r *MessageRepository) CountByFileURLOutsideChat(ctx context.Context, fileURL, excludeChatID string) (int, error) {
	defer logger.DeferLogDuration("msg.CountByFileURLOutsideChat", time.Now())()
	var cnt int
	err := r.pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM messages
		 WHERE file_url = $1 AND chat_id <> $2`,
		fileURL,
		excludeChatID,
	).Scan(&cnt)
	if err != nil {
		return 0, fmt.Errorf("msgRepo.CountByFileURLOutsideChat: %w", err)
	}
	return cnt, nil
}

// UpdateContent edits a message's content and sets edited_at.
func (r *MessageRepository) UpdateContent(ctx context.Context, id, content string, editedAt time.Time) error {
	defer logger.DeferLogDuration("msg.UpdateContent", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE messages SET content = $1, edited_at = $2 WHERE id = $3`,
		content, editedAt, id,
	)
	if err != nil {
		return fmt.Errorf("msgRepo.UpdateContent: %w", err)
	}
	return nil
}

// SoftDelete marks a message as deleted and clears content.
func (r *MessageRepository) SoftDelete(ctx context.Context, id string) error {
	defer logger.DeferLogDuration("msg.SoftDelete", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE messages
		 SET is_deleted = true,
		     content = '',
		     content_type = 'text',
		     file_url = '',
		     file_name = '',
		     file_size = 0
		 WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("msgRepo.SoftDelete: %w", err)
	}
	return nil
}

// DeleteByID permanently removes message record from DB.
func (r *MessageRepository) DeleteByID(ctx context.Context, id string) error {
	defer logger.DeferLogDuration("msg.DeleteByID", time.Now())()
	_, err := r.pool.Exec(ctx, `DELETE FROM messages WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("msgRepo.DeleteByID: %w", err)
	}
	return nil
}

// CountActiveByFileURLExcludingMessage counts active (not soft-deleted) messages
// referencing the same file URL, excluding the current message.
func (r *MessageRepository) CountActiveByFileURLExcludingMessage(ctx context.Context, fileURL, excludeMessageID string) (int, error) {
	defer logger.DeferLogDuration("msg.CountActiveByFileURLExcludingMessage", time.Now())()
	var cnt int
	err := r.pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM messages
		 WHERE file_url = $1 AND id <> $2 AND is_deleted = false`,
		fileURL,
		excludeMessageID,
	).Scan(&cnt)
	if err != nil {
		return 0, fmt.Errorf("msgRepo.CountActiveByFileURLExcludingMessage: %w", err)
	}
	return cnt, nil
}

// UserActivityStats contains aggregated user activity metrics.
type UserActivityStats struct {
	MessagesToday  int     `json:"messages_today"`
	MessagesWeek   int     `json:"messages_week"`
	AvgResponseSec float64 `json:"avg_response_sec"`
}

// GetUserStats calculates activity stats for a user.
func (r *MessageRepository) GetUserStats(ctx context.Context, userID string) (*UserActivityStats, error) {
	defer logger.DeferLogDuration("msg.GetUserStats", time.Now())()
	stats := &UserActivityStats{}

	// Messages today
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages
		 WHERE sender_id = $1 AND is_deleted = false
		 AND created_at >= CURRENT_DATE`, userID,
	).Scan(&stats.MessagesToday)
	if err != nil {
		return nil, fmt.Errorf("msgRepo.GetUserStats today: %w", err)
	}

	// Messages this week
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages
		 WHERE sender_id = $1 AND is_deleted = false
		 AND created_at >= CURRENT_DATE - INTERVAL '7 days'`, userID,
	).Scan(&stats.MessagesWeek)
	if err != nil {
		return nil, fmt.Errorf("msgRepo.GetUserStats week: %w", err)
	}

	// Average response time: for each message from this user,
	// find the time difference to the previous message in the same chat
	// (from a different user). This gives us how quickly the user responds.
	err = r.pool.QueryRow(ctx,
		`SELECT COALESCE(AVG(diff), 0) FROM (
			SELECT EXTRACT(EPOCH FROM (m.created_at - prev.created_at)) AS diff
			FROM messages m
			JOIN LATERAL (
				SELECT created_at FROM messages p
				WHERE p.chat_id = m.chat_id
				  AND p.sender_id != m.sender_id
				  AND p.created_at < m.created_at
				  AND p.is_deleted = false
				ORDER BY p.created_at DESC
				LIMIT 1
			) prev ON true
			WHERE m.sender_id = $1
			  AND m.is_deleted = false
			  AND m.created_at >= CURRENT_DATE - INTERVAL '7 days'
			LIMIT 100
		) sub
		WHERE diff > 0 AND diff < 86400`, userID,
	).Scan(&stats.AvgResponseSec)
	if err != nil {
		return nil, fmt.Errorf("msgRepo.GetUserStats avgResp: %w", err)
	}

	return stats, nil
}

// SearchMessages searches messages in a user's chats using ILIKE. If chatID is not empty, limits to that chat.
func (r *MessageRepository) SearchMessages(ctx context.Context, userID, query string, limit int, chatID string) ([]model.Message, error) {
	defer logger.DeferLogDuration("msg.SearchMessages", time.Now())()
	sql := `SELECT m.id, m.chat_id, m.sender_id, m.content, m.content_type, m.file_url, m.file_name, m.file_size, m.status,
		        m.reply_to_id, m.edited_at, m.is_deleted, m.created_at,
		        u.id, u.username, u.avatar_url, u.is_online, u.last_seen_at
		 FROM messages m
		 JOIN users u ON u.id = m.sender_id
		 JOIN chat_members cm ON cm.chat_id = m.chat_id AND cm.user_id = $1
		 WHERE m.is_deleted = false
		   AND (
		     m.content ILIKE '%' || $2 || '%'
		     OR COALESCE(m.file_name, '') ILIKE '%' || $2 || '%'
		   )
		   AND m.created_at > cm.cleared_at`
	args := []interface{}{userID, query}
	if chatID != "" {
		sql += ` AND m.chat_id = $3`
		args = append(args, chatID)
	}
	args = append(args, limit)
	sql += ` ORDER BY m.created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args))

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("msgRepo.SearchMessages query: %w", err)
	}
	defer rows.Close()

	msgs := make([]model.Message, 0, limit)
	for rows.Next() {
		var m model.Message
		sender := &model.UserPublic{}
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.ContentType, &m.FileURL, &m.FileName, &m.FileSize, &m.Status,
			&m.ReplyToID, &m.EditedAt, &m.IsDeleted, &m.CreatedAt,
			&sender.ID, &sender.Username, &sender.AvatarURL, &sender.IsOnline, &sender.LastSeenAt); err != nil {
			return nil, fmt.Errorf("msgRepo.SearchMessages scan: %w", err)
		}
		m.Sender = sender
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("msgRepo.SearchMessages rows: %w", err)
	}
	return msgs, nil
}
