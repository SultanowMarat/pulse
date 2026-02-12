package model

import "time"

type ContentType string

const (
	ContentTypeText   ContentType = "text"
	ContentTypeImage  ContentType = "image"
	ContentTypeFile   ContentType = "file"
	ContentTypeVoice  ContentType = "voice"
	ContentTypeSystem ContentType = "system"
)

type MessageStatus string

const (
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
)

type Message struct {
	ID          string        `json:"id"`
	ChatID      string        `json:"chat_id"`
	SenderID    string        `json:"sender_id"`
	Content     string        `json:"content"`
	ContentType ContentType   `json:"content_type"`
	FileURL     string        `json:"file_url,omitempty"`
	FileName    string        `json:"file_name,omitempty"`
	FileSize    int64         `json:"file_size,omitempty"`
	Status      MessageStatus `json:"status"`
	ReplyToID   *string       `json:"reply_to_id,omitempty"`
	EditedAt    *time.Time    `json:"edited_at,omitempty"`
	IsDeleted   bool          `json:"is_deleted"`
	CreatedAt   time.Time     `json:"created_at"`
	Sender      *UserPublic   `json:"sender,omitempty"`
	ReplyTo     *Message      `json:"reply_to,omitempty"`
	Reactions   []Reaction    `json:"reactions,omitempty"`
}

type Reaction struct {
	MessageID string    `json:"message_id"`
	UserID    string    `json:"user_id"`
	Emoji     string    `json:"emoji"`
	Username  string    `json:"username,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ReactionGroup is aggregated reaction info for display.
type ReactionGroup struct {
	Emoji string   `json:"emoji"`
	Count int      `json:"count"`
	Users []string `json:"users"` // user IDs
}

type PinnedMessage struct {
	ChatID    string    `json:"chat_id"`
	MessageID string    `json:"message_id"`
	PinnedBy  string    `json:"pinned_by"`
	PinnedAt  time.Time `json:"pinned_at"`
	Message   *Message  `json:"message,omitempty"`
}
