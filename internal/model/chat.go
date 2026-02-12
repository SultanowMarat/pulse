package model

import "time"

type ChatType string

const (
	ChatTypePersonal ChatType = "personal"
	ChatTypeGroup    ChatType = "group"
	ChatTypeNotes    ChatType = "notes"
)

type Chat struct {
	ID          string    `json:"id"`
	ChatType    ChatType  `json:"chat_type"`
	SystemKey   string    `json:"system_key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type ChatMember struct {
	ChatID     string    `json:"chat_id"`
	UserID     string    `json:"user_id"`
	Role       string    `json:"role"`
	JoinedAt   time.Time `json:"joined_at"`
	LastReadAt time.Time `json:"last_read_at"`
	Muted      bool      `json:"muted"`
	ClearedAt  time.Time `json:"cleared_at"`
}

type ChatWithLastMessage struct {
	Chat        Chat         `json:"chat"`
	LastMessage *Message     `json:"last_message,omitempty"`
	Members     []UserPublic `json:"members"`
	UnreadCount int          `json:"unread_count"`
	Muted       bool         `json:"muted"`
}
