package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/model"
)

type ChatRepository struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{pool: pool}
}

func (r *ChatRepository) Create(ctx context.Context, c *model.Chat) error {
	defer logger.DeferLogDuration("chat.Create", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO chats (id, chat_type, system_key, name, description, avatar_url, created_by, created_at, is_system)
		 VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, CASE WHEN NULLIF($3, '') IS NULL THEN FALSE ELSE TRUE END)`,
		c.ID, c.ChatType, c.SystemKey, c.Name, c.Description, c.AvatarURL, c.CreatedBy, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("chatRepo.Create: %w", err)
	}
	return nil
}

func (r *ChatRepository) GetByID(ctx context.Context, id string) (*model.Chat, error) {
	defer logger.DeferLogDuration("chat.GetByID", time.Now())()
	c := &model.Chat{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, chat_type, COALESCE(system_key,''), name, COALESCE(description,''), avatar_url, created_by, created_at
		 FROM chats WHERE id = $1`, id,
	).Scan(&c.ID, &c.ChatType, &c.SystemKey, &c.Name, &c.Description, &c.AvatarURL, &c.CreatedBy, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetByID: %w", err)
	}
	return c, nil
}

func (r *ChatRepository) UpdateChat(ctx context.Context, id, name, description, avatarURL string) error {
	defer logger.DeferLogDuration("chat.UpdateChat", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE chats SET name = $1, description = $2, avatar_url = $3 WHERE id = $4`,
		name, description, avatarURL, id,
	)
	if err != nil {
		return fmt.Errorf("chatRepo.UpdateChat: %w", err)
	}
	return nil
}

func (r *ChatRepository) AddMember(ctx context.Context, m *model.ChatMember) error {
	defer logger.DeferLogDuration("chat.AddMember", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO chat_members (chat_id, user_id, role, joined_at)
		 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		m.ChatID, m.UserID, m.Role, m.JoinedAt,
	)
	if err != nil {
		return fmt.Errorf("chatRepo.AddMember: %w", err)
	}
	return nil
}

func (r *ChatRepository) RemoveMember(ctx context.Context, chatID, userID string) error {
	defer logger.DeferLogDuration("chat.RemoveMember", time.Now())()
	_, err := r.pool.Exec(ctx,
		`DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID,
	)
	if err != nil {
		return fmt.Errorf("chatRepo.RemoveMember: %w", err)
	}
	return nil
}

func (r *ChatRepository) GetMembers(ctx context.Context, chatID string) ([]model.User, error) {
	defer logger.DeferLogDuration("chat.GetMembers", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT u.id, u.username, COALESCE(u.email,''), u.password_hash, u.avatar_url, u.last_seen_at, u.is_online, u.created_at
		 FROM users u
		 JOIN chat_members cm ON cm.user_id = u.id
		 WHERE cm.chat_id = $1
		 ORDER BY cm.joined_at`, chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetMembers query: %w", err)
	}
	defer rows.Close()

	users := make([]model.User, 0, 8)
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.AvatarURL, &u.LastSeenAt, &u.IsOnline, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("chatRepo.GetMembers scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatRepo.GetMembers rows: %w", err)
	}
	return users, nil
}

// GetMembersByChatIDs returns public members grouped by chat ID.
func (r *ChatRepository) GetMembersByChatIDs(ctx context.Context, chatIDs []string) (map[string][]model.UserPublic, error) {
	defer logger.DeferLogDuration("chat.GetMembersByChatIDs", time.Now())()
	out := make(map[string][]model.UserPublic, len(chatIDs))
	if len(chatIDs) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT cm.chat_id,
		        u.id, u.username, COALESCE(u.email,''), COALESCE(u.phone,''), COALESCE(u.position,''),
		        u.avatar_url, u.is_online, u.last_seen_at, u.disabled_at
		 FROM chat_members cm
		 JOIN users u ON u.id = cm.user_id
		 WHERE cm.chat_id = ANY($1::uuid[])
		 ORDER BY cm.chat_id, cm.joined_at`,
		chatIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetMembersByChatIDs query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var chatID string
		var u model.UserPublic
		if err := rows.Scan(&chatID, &u.ID, &u.Username, &u.Email, &u.Phone, &u.Position, &u.AvatarURL, &u.IsOnline, &u.LastSeenAt, &u.DisabledAt); err != nil {
			return nil, fmt.Errorf("chatRepo.GetMembersByChatIDs scan: %w", err)
		}
		out[chatID] = append(out[chatID], u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatRepo.GetMembersByChatIDs rows: %w", err)
	}
	return out, nil
}

func (r *ChatRepository) GetMemberIDs(ctx context.Context, chatID string) ([]string, error) {
	defer logger.DeferLogDuration("chat.GetMemberIDs", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT user_id FROM chat_members WHERE chat_id = $1`, chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetMemberIDs query: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0, 8)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("chatRepo.GetMemberIDs scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatRepo.GetMemberIDs rows: %w", err)
	}
	return ids, nil
}

func (r *ChatRepository) IsMember(ctx context.Context, chatID, userID string) (bool, error) {
	defer logger.DeferLogDuration("chat.IsMember", time.Now())()
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2)`,
		chatID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("chatRepo.IsMember: %w", err)
	}
	return exists, nil
}

func (r *ChatRepository) GetMemberRole(ctx context.Context, chatID, userID string) (string, error) {
	defer logger.DeferLogDuration("chat.GetMemberRole", time.Now())()
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM chat_members WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("chatRepo.GetMemberRole: %w", err)
	}
	return role, nil
}

// GetMemberSettings returns mute + clear settings for a member in a chat.
func (r *ChatRepository) GetMemberSettings(ctx context.Context, chatID, userID string) (bool, time.Time, error) {
	defer logger.DeferLogDuration("chat.GetMemberSettings", time.Now())()
	var muted bool
	var clearedAt time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT muted, cleared_at FROM chat_members WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID,
	).Scan(&muted, &clearedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, time.Time{}, ErrNotFound
	}
	if err != nil {
		return false, time.Time{}, fmt.Errorf("chatRepo.GetMemberSettings: %w", err)
	}
	return muted, clearedAt, nil
}

// SetMemberMuted enables/disables notifications for a member in a chat.
func (r *ChatRepository) SetMemberMuted(ctx context.Context, chatID, userID string, muted bool) error {
	defer logger.DeferLogDuration("chat.SetMemberMuted", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE chat_members SET muted = $1 WHERE chat_id = $2 AND user_id = $3`,
		muted, chatID, userID,
	)
	if err != nil {
		return fmt.Errorf("chatRepo.SetMemberMuted: %w", err)
	}
	return nil
}

// SetMemberClearedAt updates the cleared_at timestamp for a member in a chat.
func (r *ChatRepository) SetMemberClearedAt(ctx context.Context, chatID, userID string, t time.Time) error {
	defer logger.DeferLogDuration("chat.SetMemberClearedAt", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE chat_members SET cleared_at = $1 WHERE chat_id = $2 AND user_id = $3`,
		t, chatID, userID,
	)
	if err != nil {
		return fmt.Errorf("chatRepo.SetMemberClearedAt: %w", err)
	}
	return nil
}

// AllMembersCleared returns true if every member has cleared_at after epoch.
func (r *ChatRepository) AllMembersCleared(ctx context.Context, chatID string) (bool, error) {
	defer logger.DeferLogDuration("chat.AllMembersCleared", time.Now())()
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) = COUNT(*) FILTER (WHERE cleared_at > '1970-01-01T00:00:00Z')
		 FROM chat_members WHERE chat_id = $1`, chatID,
	).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("chatRepo.AllMembersCleared: %w", err)
	}
	return ok, nil
}

// GetMemberMuteMap returns muted status per member for push notifications.
func (r *ChatRepository) GetMemberMuteMap(ctx context.Context, chatID string) (map[string]bool, error) {
	defer logger.DeferLogDuration("chat.GetMemberMuteMap", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT user_id, muted FROM chat_members WHERE chat_id = $1`, chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetMemberMuteMap query: %w", err)
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var userID string
		var muted bool
		if err := rows.Scan(&userID, &muted); err != nil {
			return nil, fmt.Errorf("chatRepo.GetMemberMuteMap scan: %w", err)
		}
		out[userID] = muted
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatRepo.GetMemberMuteMap rows: %w", err)
	}
	return out, nil
}

func (r *ChatRepository) GetUserChats(ctx context.Context, userID string) ([]model.Chat, error) {
	defer logger.DeferLogDuration("chat.GetUserChats", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT c.id, c.chat_type, COALESCE(c.system_key,''), c.name, COALESCE(c.description,''), c.avatar_url, c.created_by, c.created_at
		 FROM chats c
		 JOIN chat_members cm ON cm.chat_id = c.id
		 WHERE cm.user_id = $1
		 ORDER BY c.created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetUserChats query: %w", err)
	}
	defer rows.Close()

	chats := make([]model.Chat, 0, 16)
	for rows.Next() {
		var c model.Chat
		if err := rows.Scan(&c.ID, &c.ChatType, &c.SystemKey, &c.Name, &c.Description, &c.AvatarURL, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("chatRepo.GetUserChats scan: %w", err)
		}
		chats = append(chats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatRepo.GetUserChats rows: %w", err)
	}
	return chats, nil
}

func (r *ChatRepository) FindPersonalChat(ctx context.Context, userID1, userID2 string) (*model.Chat, error) {
	defer logger.DeferLogDuration("chat.FindPersonalChat", time.Now())()
	c := &model.Chat{}
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.chat_type, COALESCE(c.system_key,''), c.name, COALESCE(c.description,''), c.avatar_url, c.created_by, c.created_at
		 FROM chats c
		 WHERE c.chat_type = 'personal'
		   AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = $1)
		   AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = $2)`,
		userID1, userID2,
	).Scan(&c.ID, &c.ChatType, &c.SystemKey, &c.Name, &c.Description, &c.AvatarURL, &c.CreatedBy, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("chatRepo.FindPersonalChat: %w", err)
	}
	return c, nil
}

// FindNotesChat returns the notes chat for the user (chat_type=notes, single member=userID).
func (r *ChatRepository) FindNotesChat(ctx context.Context, userID string) (*model.Chat, error) {
	defer logger.DeferLogDuration("chat.FindNotesChat", time.Now())()
	c := &model.Chat{}
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.chat_type, COALESCE(c.system_key,''), c.name, COALESCE(c.description,''), c.avatar_url, c.created_by, c.created_at
		 FROM chats c
		 WHERE c.chat_type = 'notes'
		   AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = $1)
		   AND (SELECT COUNT(*) FROM chat_members WHERE chat_id = c.id) = 1`,
		userID,
	).Scan(&c.ID, &c.ChatType, &c.SystemKey, &c.Name, &c.Description, &c.AvatarURL, &c.CreatedBy, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("chatRepo.FindNotesChat: %w", err)
	}
	return c, nil
}

// Service/system chat key for "Общий чат".
const GeneralChatSystemKey = "general"

// Default name for the general chat.
const GeneralChatName = "Общий чат"

// GetBySystemKey returns a chat by system_key.
func (r *ChatRepository) GetBySystemKey(ctx context.Context, systemKey string) (*model.Chat, error) {
	defer logger.DeferLogDuration("chat.GetBySystemKey", time.Now())()
	c := &model.Chat{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, chat_type, COALESCE(system_key,''), name, COALESCE(description,''), avatar_url, created_by, created_at
		 FROM chats WHERE system_key = $1`,
		systemKey,
	).Scan(&c.ID, &c.ChatType, &c.SystemKey, &c.Name, &c.Description, &c.AvatarURL, &c.CreatedBy, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetBySystemKey: %w", err)
	}
	return c, nil
}

// GetOrCreateGeneralChat returns the general chat, creating it if it doesn't exist.
// createdBy is used for created_by when creating the chat (the chat itself is admin-managed by permissions).
func (r *ChatRepository) GetOrCreateGeneralChat(ctx context.Context, createdBy string) (*model.Chat, error) {
	defer logger.DeferLogDuration("chat.GetOrCreateGeneralChat", time.Now())()
	c, err := r.GetBySystemKey(ctx, GeneralChatSystemKey)
	if err == nil {
		// Self-heal renamed/corrupted system chat title.
		if c.Name != GeneralChatName {
			if err := r.UpdateChat(ctx, c.ID, GeneralChatName, c.Description, c.AvatarURL); err != nil {
				return nil, err
			}
			c.Name = GeneralChatName
		}
		return c, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	c = &model.Chat{
		ID:        uuid.New().String(),
		ChatType:  model.ChatTypeGroup,
		SystemKey: GeneralChatSystemKey,
		Name:      GeneralChatName,
		CreatedBy: createdBy,
		CreatedAt: now,
	}
	if err := r.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// NotesChatName is the display name for the system notes chat.
const NotesChatName = "Заметки"

// NotesChatDescription is the text shown in the notes chat.
const NotesChatDescription = "Это чат для личных заметок. Здесь удобно хранить:\n— планы на день;\n— список покупок;\n— полезные ссылки;\n— мысли и идеи.\n\nЭто ваш личный чат, в него нельзя добавлять других людей. Но при желании вы можете пересылать сообщения из заметок в другие чаты."

// GetOrCreateNotesChat returns the user's notes chat, creating it if it does not exist.
func (r *ChatRepository) GetOrCreateNotesChat(ctx context.Context, userID string) (*model.Chat, error) {
	defer logger.DeferLogDuration("chat.GetOrCreateNotesChat", time.Now())()
	c, err := r.FindNotesChat(ctx, userID)
	if err == nil {
		// Self-heal notes system text if it was previously saved with broken encoding.
		if c.Name != NotesChatName || c.Description != NotesChatDescription {
			if err := r.UpdateChat(ctx, c.ID, NotesChatName, NotesChatDescription, c.AvatarURL); err != nil {
				return nil, err
			}
			c.Name = NotesChatName
			c.Description = NotesChatDescription
		}
		return c, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	c = &model.Chat{
		ID:          uuid.New().String(),
		ChatType:    model.ChatTypeNotes,
		Name:        NotesChatName,
		Description: NotesChatDescription,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
	}
	if err := r.Create(ctx, c); err != nil {
		return nil, err
	}
	member := &model.ChatMember{
		ChatID:   c.ID,
		UserID:   userID,
		Role:     "admin",
		JoinedAt: time.Now(),
	}
	if err := r.AddMember(ctx, member); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateMemberLastRead updates the last_read_at timestamp for a member.
func (r *ChatRepository) UpdateMemberLastRead(ctx context.Context, chatID, userID string, t time.Time) error {
	defer logger.DeferLogDuration("chat.UpdateMemberLastRead", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE chat_members SET last_read_at = $1 WHERE chat_id = $2 AND user_id = $3`,
		t, chatID, userID,
	)
	if err != nil {
		return fmt.Errorf("chatRepo.UpdateMemberLastRead: %w", err)
	}
	return nil
}

// GetUnreadCount counts messages in a chat created after the user's last_read_at.
func (r *ChatRepository) GetUnreadCount(ctx context.Context, chatID, userID string) (int, error) {
	defer logger.DeferLogDuration("chat.GetUnreadCount", time.Now())()
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages m
		 JOIN chat_members cm ON cm.chat_id = m.chat_id AND cm.user_id = $2
		 WHERE m.chat_id = $1
		   AND m.sender_id != $2
		   AND m.created_at > GREATEST(cm.last_read_at, cm.cleared_at)
		   AND m.is_deleted = false`,
		chatID, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("chatRepo.GetUnreadCount: %w", err)
	}
	return count, nil
}

// GetUnreadCountsForUserChats returns unread counts grouped by chat.
func (r *ChatRepository) GetUnreadCountsForUserChats(ctx context.Context, userID string, chatIDs []string) (map[string]int, error) {
	defer logger.DeferLogDuration("chat.GetUnreadCountsForUserChats", time.Now())()
	out := make(map[string]int, len(chatIDs))
	if len(chatIDs) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT m.chat_id, COUNT(*)
		 FROM messages m
		 JOIN chat_members cm ON cm.chat_id = m.chat_id AND cm.user_id = $1
		 WHERE m.chat_id = ANY($2::uuid[])
		   AND m.sender_id != $1
		   AND m.created_at > GREATEST(cm.last_read_at, cm.cleared_at)
		   AND m.is_deleted = false
		 GROUP BY m.chat_id`,
		userID,
		chatIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetUnreadCountsForUserChats query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var chatID string
		var count int
		if err := rows.Scan(&chatID, &count); err != nil {
			return nil, fmt.Errorf("chatRepo.GetUnreadCountsForUserChats scan: %w", err)
		}
		out[chatID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatRepo.GetUnreadCountsForUserChats rows: %w", err)
	}
	return out, nil
}

// GetMutedMapForUserChats returns muted state per chat for a user.
func (r *ChatRepository) GetMutedMapForUserChats(ctx context.Context, userID string, chatIDs []string) (map[string]bool, error) {
	defer logger.DeferLogDuration("chat.GetMutedMapForUserChats", time.Now())()
	out := make(map[string]bool, len(chatIDs))
	if len(chatIDs) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT chat_id, muted
		 FROM chat_members
		 WHERE user_id = $1 AND chat_id = ANY($2::uuid[])`,
		userID,
		chatIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("chatRepo.GetMutedMapForUserChats query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var chatID string
		var muted bool
		if err := rows.Scan(&chatID, &muted); err != nil {
			return nil, fmt.Errorf("chatRepo.GetMutedMapForUserChats scan: %w", err)
		}
		out[chatID] = muted
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatRepo.GetMutedMapForUserChats rows: %w", err)
	}
	return out, nil
}
