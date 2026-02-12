package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/messenger/internal/middleware"
	"github.com/messenger/internal/repository"
)

type MessageHandler struct {
	msgRepo    *repository.MessageRepository
	chatRepo   *repository.ChatRepository
	reactRepo  *repository.ReactionRepository
	pinnedRepo *repository.PinnedRepository
}

func NewMessageHandler(
	msgRepo *repository.MessageRepository,
	chatRepo *repository.ChatRepository,
	reactRepo *repository.ReactionRepository,
	pinnedRepo *repository.PinnedRepository,
) *MessageHandler {
	return &MessageHandler{msgRepo: msgRepo, chatRepo: chatRepo, reactRepo: reactRepo, pinnedRepo: pinnedRepo}
}

func (h *MessageHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chatId")
	userID := middleware.GetUserID(r.Context())

	isMember, err := h.chatRepo.IsMember(r.Context(), chatID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}
	if !isMember {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	if limit > 100 {
		limit = 100
	}

	messages, err := h.msgRepo.GetChatMessages(r.Context(), chatID, userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}

	// Enrich with reactions and reply-to
	for i := range messages {
		reactions, err := h.reactRepo.GetByMessage(r.Context(), messages[i].ID)
		if err == nil && len(reactions) > 0 {
			messages[i].Reactions = reactions
		}

		if messages[i].ReplyToID != nil {
			replyMsg, err := h.msgRepo.GetByID(r.Context(), *messages[i].ReplyToID)
			if err == nil {
				messages[i].ReplyTo = replyMsg
			}
		}
	}

	writeJSON(w, http.StatusOK, messages)
}

func (h *MessageHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chatId")
	userID := middleware.GetUserID(r.Context())

	isMember, err := h.chatRepo.IsMember(r.Context(), chatID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}
	if !isMember {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}

	if err := h.msgRepo.MarkAsRead(r.Context(), chatID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark as read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SearchMessages searches messages across the user's chats.
func (h *MessageHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	limit := queryInt(r, "limit", 30)
	if limit > 50 {
		limit = 50
	}
	chatID := r.URL.Query().Get("chat_id")

	messages, err := h.msgRepo.SearchMessages(r.Context(), userID, query, limit, chatID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

// GetPinnedMessages returns pinned messages for a chat.
func (h *MessageHandler) GetPinnedMessages(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chatId")
	userID := middleware.GetUserID(r.Context())

	isMember, err := h.chatRepo.IsMember(r.Context(), chatID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}
	if !isMember {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}

	pinned, err := h.pinnedRepo.GetPinnedForUser(r.Context(), chatID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get pinned messages")
		return
	}
	writeJSON(w, http.StatusOK, pinned)
}

// GetReactions returns reactions for a message.
func (h *MessageHandler) GetReactions(w http.ResponseWriter, r *http.Request) {
	messageID := chi.URLParam(r, "messageId")
	reactions, err := h.reactRepo.GetByMessage(r.Context(), messageID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get reactions")
		return
	}
	writeJSON(w, http.StatusOK, reactions)
}
