package ws

import (
	"time"

	"github.com/pulse/internal/model"
)

type EventType string

const (
	EventNewMessage      EventType = "new_message"
	EventMessageRead     EventType = "message_read"
	EventMessageEdited   EventType = "message_edited"
	EventMessageDeleted  EventType = "message_deleted"
	EventTyping          EventType = "typing"
	EventUserOnline      EventType = "user_online"
	EventUserOffline     EventType = "user_offline"
	EventChatCreated     EventType = "chat_created"
	EventReactionAdded   EventType = "reaction_added"
	EventReactionRemoved EventType = "reaction_removed"
	EventMessagePinned   EventType = "message_pinned"
	EventMessageUnpinned EventType = "message_unpinned"
	EventMemberAdded     EventType = "member_added"
	EventMemberRemoved   EventType = "member_removed"
	EventChatUpdated     EventType = "chat_updated"
	EventSessionRevoked  EventType = "session_revoked"
	EventError           EventType = "error"
)

// IncomingMessage is what the client sends to the server.
type IncomingMessage struct {
	Type    EventType `json:"type"`
	ChatID  string    `json:"chat_id,omitempty"`
	Content string    `json:"content,omitempty"`
	// ClientMsgID is generated on client for reliable optimistic UI reconciliation.
	ClientMsgID string `json:"client_msg_id,omitempty"`

	// For file messages
	ContentType model.ContentType `json:"content_type,omitempty"`
	FileURL     string            `json:"file_url,omitempty"`
	FileName    string            `json:"file_name,omitempty"`
	FileSize    int64             `json:"file_size,omitempty"`

	// For reply
	ReplyToID string `json:"reply_to_id,omitempty"`

	// For edit/delete
	MessageID string `json:"message_id,omitempty"`

	// For reactions
	Emoji string `json:"emoji,omitempty"`

	// For forward
	ForwardChatID string `json:"forward_chat_id,omitempty"`
}

// OutgoingMessage is what the server sends to the client.
// Payload uses typed structs to avoid heap-heavy map[string]any.
type OutgoingMessage struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload"`
}

// --- Typed payloads for hot-path (avoid map[string]any allocations) ---

// MessageEditedPayload is broadcast when a message is edited.
type MessageEditedPayload struct {
	MessageID string    `json:"message_id"`
	ChatID    string    `json:"chat_id"`
	Content   string    `json:"content"`
	EditedAt  time.Time `json:"edited_at"`
}

// MessageDeletedPayload is broadcast when a message is deleted.
type MessageDeletedPayload struct {
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
}

// ReactionPayload is broadcast when a reaction is added or removed.
type ReactionPayload struct {
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
	UserID    string `json:"user_id"`
	Emoji     string `json:"emoji"`
}

// PinPayload is broadcast when a message is pinned.
type PinPayload struct {
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
	PinnedBy  string `json:"pinned_by,omitempty"`
}

// UnpinPayload is broadcast when a message is unpinned.
type UnpinPayload struct {
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
}

// TypingPayload is broadcast when a user is typing.
type TypingPayload struct {
	ChatID string `json:"chat_id"`
	UserID string `json:"user_id"`
}

// MessageReadPayload is broadcast when messages are read.
type MessageReadPayload struct {
	ChatID string `json:"chat_id"`
	UserID string `json:"user_id"`
}

// UserStatusPayload is broadcast for online/offline status.
type UserStatusPayload struct {
	UserID string `json:"user_id"`
	Online bool   `json:"online"`
}

// MemberAddedPayload is broadcast when a member is added to a group.
type MemberAddedPayload struct {
	ChatID    string `json:"chat_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	ActorID   string `json:"actor_id"`
	ActorName string `json:"actor_name"`
}

// MemberRemovedPayload is broadcast when a member is removed or leaves.
type MemberRemovedPayload struct {
	ChatID    string `json:"chat_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	IsLeave   bool   `json:"is_leave"`   // true if user left themselves
	ActorName string `json:"actor_name"` // who removed (empty if is_leave)
}
