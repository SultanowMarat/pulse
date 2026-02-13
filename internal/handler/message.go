package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/messenger/internal/cache"
	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/middleware"
	"github.com/messenger/internal/model"
	"github.com/messenger/internal/repository"
)

type MessageHandler struct {
	msgRepo    *repository.MessageRepository
	chatRepo   *repository.ChatRepository
	reactRepo  *repository.ReactionRepository
	pinnedRepo *repository.PinnedRepository
	cache      *cache.ChatCache
}

func NewMessageHandler(
	msgRepo *repository.MessageRepository,
	chatRepo *repository.ChatRepository,
	reactRepo *repository.ReactionRepository,
	pinnedRepo *repository.PinnedRepository,
	cache *cache.ChatCache,
) *MessageHandler {
	return &MessageHandler{msgRepo: msgRepo, chatRepo: chatRepo, reactRepo: reactRepo, pinnedRepo: pinnedRepo, cache: cache}
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

	if h.cache != nil {
		var cached []model.Message
		hit, err := h.cache.MessageList(r.Context(), chatID, userID, limit, offset, &cached)
		if err != nil {
			logger.Errorf("messages cache read chat=%s user=%s: %v", chatID, userID, err)
		}
		if hit {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}

	messages, err := h.msgRepo.GetChatMessages(r.Context(), chatID, userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}

	h.enrichMessages(r.Context(), messages)

	if h.cache != nil {
		if err := h.cache.SetMessageList(r.Context(), chatID, userID, limit, offset, messages); err != nil {
			logger.Errorf("messages cache write chat=%s user=%s: %v", chatID, userID, err)
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
	if h.cache != nil {
		if err := h.cache.InvalidateMessageLists(r.Context(), chatID); err != nil {
			logger.Errorf("mark read invalidate messages cache chat=%s: %v", chatID, err)
		}
		if err := h.cache.InvalidateUserChats(r.Context(), userID); err != nil {
			logger.Errorf("mark read invalidate user chats cache user=%s: %v", userID, err)
		}
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

func (h *MessageHandler) enrichMessages(ctx context.Context, messages []model.Message) {
	if len(messages) == 0 {
		return
	}

	messageIDs := make([]string, 0, len(messages))
	replyIDs := make([]string, 0, len(messages))
	seenReply := make(map[string]struct{}, len(messages))
	for i := range messages {
		messageIDs = append(messageIDs, messages[i].ID)
		if messages[i].ReplyToID == nil {
			continue
		}
		rid := *messages[i].ReplyToID
		if _, ok := seenReply[rid]; ok {
			continue
		}
		seenReply[rid] = struct{}{}
		replyIDs = append(replyIDs, rid)
	}

	if len(messageIDs) > 0 {
		reactionsByMessage, err := h.reactRepo.GetByMessageIDs(ctx, messageIDs)
		if err != nil {
			logger.Errorf("enrichMessages reactions: %v", err)
		} else {
			for i := range messages {
				if reactions := reactionsByMessage[messages[i].ID]; len(reactions) > 0 {
					messages[i].Reactions = reactions
				}
			}
		}
	}

	if len(replyIDs) > 0 {
		repliesByID, err := h.msgRepo.GetByIDs(ctx, replyIDs)
		if err != nil {
			logger.Errorf("enrichMessages replies: %v", err)
			return
		}
		for i := range messages {
			if messages[i].ReplyToID == nil {
				continue
			}
			if reply, ok := repliesByID[*messages[i].ReplyToID]; ok {
				messages[i].ReplyTo = reply
			}
		}
	}
}
