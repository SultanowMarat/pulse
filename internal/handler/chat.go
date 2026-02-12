package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/middleware"
	"github.com/messenger/internal/model"
	"github.com/messenger/internal/repository"
	"github.com/messenger/internal/ws"
)

type ChatHandler struct {
	chatRepo *repository.ChatRepository
	userRepo *repository.UserRepository
	permRepo *repository.PermissionRepository
	msgRepo  *repository.MessageRepository
	hub      *ws.Hub
	fileH    *FileHandler
}

func NewChatHandler(chatRepo *repository.ChatRepository, userRepo *repository.UserRepository, permRepo *repository.PermissionRepository, msgRepo *repository.MessageRepository, hub *ws.Hub, fileH *FileHandler) *ChatHandler {
	return &ChatHandler{chatRepo: chatRepo, userRepo: userRepo, permRepo: permRepo, msgRepo: msgRepo, hub: hub, fileH: fileH}
}

type CreatePersonalChatRequest struct {
	UserID string `json:"user_id"`
}

type CreateGroupChatRequest struct {
	Name      string   `json:"name"`
	MemberIDs []string `json:"member_ids"`
}

func (h *ChatHandler) CreatePersonalChat(w http.ResponseWriter, r *http.Request) {
	var req CreatePersonalChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	currentUserID := middleware.GetUserID(r.Context())
	if req.UserID == currentUserID {
		writeError(w, http.StatusBadRequest, "cannot create chat with yourself")
		return
	}

	existing, err := h.chatRepo.FindPersonalChat(r.Context(), currentUserID, req.UserID)
	if err == nil && existing != nil {
		enriched, err := h.enrichChat(r.Context(), existing, currentUserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to enrich chat")
			return
		}
		writeJSON(w, http.StatusOK, enriched)
		return
	}

	if _, err := h.userRepo.GetByID(r.Context(), req.UserID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	now := time.Now().UTC()
	chat := &model.Chat{
		ID:        uuid.New().String(),
		ChatType:  model.ChatTypePersonal,
		CreatedBy: currentUserID,
		CreatedAt: now,
	}

	if err := h.chatRepo.Create(r.Context(), chat); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chat")
		return
	}

	for _, uid := range []string{currentUserID, req.UserID} {
		member := &model.ChatMember{
			ChatID:   chat.ID,
			UserID:   uid,
			Role:     "member",
			JoinedAt: now,
		}
		if err := h.chatRepo.AddMember(r.Context(), member); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add member")
			return
		}
	}

	enriched, err := h.enrichChat(r.Context(), chat, currentUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enrich chat")
		return
	}

	// Broadcast chat_created to the other user
	h.hub.BroadcastToChat(r.Context(), chat.ID, ws.OutgoingMessage{
		Type:    ws.EventChatCreated,
		Payload: enriched,
	})

	writeJSON(w, http.StatusCreated, enriched)
}

func (h *ChatHandler) CreateGroupChat(w http.ResponseWriter, r *http.Request) {
	var req CreateGroupChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	currentUserID := middleware.GetUserID(r.Context())
	now := time.Now().UTC()
	chat := &model.Chat{
		ID:        uuid.New().String(),
		ChatType:  model.ChatTypeGroup,
		Name:      req.Name,
		CreatedBy: currentUserID,
		CreatedAt: now,
	}

	if err := h.chatRepo.Create(r.Context(), chat); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chat")
		return
	}

	adminMember := &model.ChatMember{
		ChatID:   chat.ID,
		UserID:   currentUserID,
		Role:     "admin",
		JoinedAt: now,
	}
	if err := h.chatRepo.AddMember(r.Context(), adminMember); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add admin member")
		return
	}

	for _, uid := range req.MemberIDs {
		if uid == currentUserID {
			continue
		}
		member := &model.ChatMember{
			ChatID:   chat.ID,
			UserID:   uid,
			Role:     "member",
			JoinedAt: now,
		}
		if err := h.chatRepo.AddMember(r.Context(), member); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add member")
			return
		}
	}

	enriched, err := h.enrichChat(r.Context(), chat, currentUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enrich chat")
		return
	}

	h.hub.BroadcastToChat(r.Context(), chat.ID, ws.OutgoingMessage{
		Type:    ws.EventChatCreated,
		Payload: enriched,
	})

	writeJSON(w, http.StatusCreated, enriched)
}

func (h *ChatHandler) GetUserChats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	// Service group "Общий чат": ensure every user is a member on first login.
	// This guarantees the chat appears in the "ВСЕ" tab for everyone.
	if err := h.ensureGeneralChatMember(ctx, userID); err != nil {
		logger.Errorf("GetUserChats ensure general chat: %v", err)
	}

	chats, err := h.chatRepo.GetUserChats(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chats")
		return
	}

	result := make([]model.ChatWithLastMessage, 0, len(chats)+1)
	for i := range chats {
		if chats[i].ChatType == model.ChatTypeNotes {
			continue
		}
		enriched, err := h.enrichChat(ctx, &chats[i], userID)
		if err != nil {
			continue
		}
		result = append(result, *enriched)
	}

	notesChat, err := h.chatRepo.GetOrCreateNotesChat(ctx, userID)
	if err != nil {
		logger.Errorf("GetUserChats get or create notes chat: %v", err)
	} else {
		if err := h.userRepo.AddFavorite(ctx, userID, notesChat.ID); err != nil {
			logger.Errorf("GetUserChats add notes to favorites: %v", err)
		}
		enrichedNotes, err := h.enrichChat(ctx, notesChat, userID)
		if err != nil {
			logger.Errorf("GetUserChats enrich notes chat: %v", err)
		} else {
			result = append(result, *enrichedNotes)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *ChatHandler) ensureGeneralChatMember(ctx context.Context, userID string) error {
	chat, err := h.chatRepo.GetOrCreateGeneralChat(ctx, userID)
	if err != nil {
		return err
	}

	isMember, err := h.chatRepo.IsMember(ctx, chat.ID, userID)
	if err != nil {
		return err
	}
	if isMember {
		return nil
	}

	now := time.Now().UTC()
	member := &model.ChatMember{ChatID: chat.ID, UserID: userID, Role: "member", JoinedAt: now}
	if err := h.chatRepo.AddMember(ctx, member); err != nil {
		return err
	}

	u, _ := h.userRepo.GetByID(ctx, userID)
	name := userID
	if u != nil && u.Username != "" {
		name = u.Username
	}

	sysMsg := &model.Message{
		ID:          uuid.New().String(),
		ChatID:      chat.ID,
		SenderID:    userID,
		Content:     "Добавлен пользователь " + name,
		ContentType: model.ContentTypeSystem,
		Status:      model.MessageStatusSent,
		CreatedAt:   now,
	}
	if err := h.msgRepo.Create(ctx, sysMsg); err != nil {
		logger.Errorf("general chat join system message: %v", err)
	} else {
		sysMsg.Sender = &model.UserPublic{ID: userID, Username: name}
		h.hub.BroadcastToChat(ctx, chat.ID, ws.OutgoingMessage{Type: ws.EventNewMessage, Payload: sysMsg})
	}

	h.hub.BroadcastToChat(ctx, chat.ID, ws.OutgoingMessage{
		Type: ws.EventMemberAdded,
		Payload: ws.MemberAddedPayload{
			ChatID: chat.ID, UserID: userID, Username: name,
			ActorID: "", ActorName: "",
		},
	})

	return nil
}

func (h *ChatHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "id")
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

	chat, err := h.chatRepo.GetByID(r.Context(), chatID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "chat not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get chat")
		return
	}

	enriched, err := h.enrichChat(r.Context(), chat, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enrich chat")
		return
	}
	writeJSON(w, http.StatusOK, enriched)
}

// SetMuted enables/disables notifications for the current user in a chat.
func (h *ChatHandler) SetMuted(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "id")
	if chatID == "" {
		writeError(w, http.StatusBadRequest, "chat_id required")
		return
	}
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
	var req struct {
		Muted bool `json:"muted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := h.chatRepo.SetMemberMuted(r.Context(), chatID, userID, req.Muted); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update mute")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"muted": req.Muted})
}

// ClearHistory hides messages for the current user; if all members cleared, deletes messages.
func (h *ChatHandler) ClearHistory(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "id")
	if chatID == "" {
		writeError(w, http.StatusBadRequest, "chat_id required")
		return
	}
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

	now := time.Now().UTC()
	if err := h.chatRepo.SetMemberClearedAt(r.Context(), chatID, userID, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear chat")
		return
	}
	_ = h.chatRepo.UpdateMemberLastRead(r.Context(), chatID, userID, now)

	chat, err := h.chatRepo.GetByID(r.Context(), chatID)
	if err == nil && chat.ChatType == model.ChatTypePersonal {
		allCleared, err := h.chatRepo.AllMembersCleared(r.Context(), chatID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check clear state")
			return
		}
		if allCleared {
			fileURLs, err := h.msgRepo.GetDistinctFileURLsByChat(r.Context(), chatID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to collect chat files")
				return
			}
			for _, fileURL := range fileURLs {
				refsOutside, err := h.msgRepo.CountByFileURLOutsideChat(r.Context(), fileURL, chatID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "failed to check file references")
					return
				}
				if refsOutside == 0 && h.fileH != nil {
					if err := h.fileH.DeleteByMessageFileURL(r.Context(), fileURL); err != nil {
						writeError(w, http.StatusInternalServerError, "failed to delete chat files")
						return
					}
				}
			}
			if err := h.msgRepo.DeleteChatMessages(r.Context(), chatID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to delete messages")
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateChat updates group name, description and avatar.
type UpdateChatRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

func (h *ChatHandler) UpdateChat(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	var req UpdateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	chat, err := h.chatRepo.GetByID(r.Context(), chatID)
	if err != nil {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}
	if chat.ChatType != model.ChatTypeGroup {
		writeError(w, http.StatusBadRequest, "only group chats can be updated")
		return
	}

	// System "Общий чат" can only be edited by global administrators.
	if chat.SystemKey == repository.GeneralChatSystemKey {
		perm, err := h.permRepo.GetByUserID(r.Context(), userID)
		if err != nil || perm == nil || !perm.Administrator {
			writeError(w, http.StatusForbidden, "only administrator can update this chat")
			return
		}
	}

	isMember, err := h.chatRepo.IsMember(r.Context(), chatID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}

	name := chat.Name
	if req.Name != "" {
		name = req.Name
	}
	desc := chat.Description
	if req.Description != "" {
		desc = req.Description
	}
	avatarURL := chat.AvatarURL
	if req.AvatarURL != nil {
		avatarURL = *req.AvatarURL
	}

	if err := h.chatRepo.UpdateChat(r.Context(), chatID, name, desc, avatarURL); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update chat")
		return
	}

	h.hub.BroadcastToChat(r.Context(), chatID, ws.OutgoingMessage{
		Type: ws.EventChatUpdated,
		Payload: map[string]string{
			"chat_id":     chatID,
			"name":        name,
			"description": desc,
			"avatar_url":  avatarURL,
		},
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AddMembers adds members to a group chat.
type AddMembersRequest struct {
	MemberIDs []string `json:"member_ids"`
}

func (h *ChatHandler) AddMembers(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	var req AddMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	chat, err := h.chatRepo.GetByID(r.Context(), chatID)
	if err != nil {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}
	if chat.ChatType != model.ChatTypeGroup {
		writeError(w, http.StatusBadRequest, "only group chats support adding members")
		return
	}

	// System general chat: only global administrators may add members manually.
	if chat.SystemKey == repository.GeneralChatSystemKey {
		perm, err := h.permRepo.GetByUserID(r.Context(), userID)
		if err != nil || perm == nil || !perm.Administrator {
			writeError(w, http.StatusForbidden, "only administrator can add members to this chat")
			return
		}
	}

	isMember, err := h.chatRepo.IsMember(r.Context(), chatID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}

	actor, _ := h.userRepo.GetByID(r.Context(), userID)
	actorName := ""
	if actor != nil {
		actorName = actor.Username
	}
	now := time.Now().UTC()
	for _, uid := range req.MemberIDs {
		member := &model.ChatMember{ChatID: chatID, UserID: uid, Role: "member", JoinedAt: now}
		if err := h.chatRepo.AddMember(r.Context(), member); err != nil {
			logger.Errorf("addMember chat=%s user=%s: %v", chatID, uid, err)
		} else {
			added, _ := h.userRepo.GetByID(r.Context(), uid)
			addedName := uid
			if added != nil {
				addedName = added.Username
			}
			// Системное сообщение в чат: «Иван добавил(а) Марию в группу»
			systemContent := actorName + " добавил(а) " + addedName + " в группу"
			sysMsg := &model.Message{
				ID:          uuid.New().String(),
				ChatID:      chatID,
				SenderID:    userID,
				Content:     systemContent,
				ContentType: model.ContentTypeSystem,
				Status:      model.MessageStatusSent,
				CreatedAt:   now,
			}
			if err := h.msgRepo.Create(r.Context(), sysMsg); err != nil {
				logger.Errorf("addMember system message chat=%s: %v", chatID, err)
			} else {
				sysMsg.Sender = &model.UserPublic{ID: userID, Username: actorName}
				h.hub.BroadcastToChat(r.Context(), chatID, ws.OutgoingMessage{
					Type:    ws.EventNewMessage,
					Payload: sysMsg,
				})
			}
			h.hub.BroadcastToChat(r.Context(), chatID, ws.OutgoingMessage{
				Type: ws.EventMemberAdded,
				Payload: ws.MemberAddedPayload{
					ChatID: chatID, UserID: uid, Username: addedName,
					ActorID: userID, ActorName: actorName,
				},
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RemoveMember removes a member from a group chat.
func (h *ChatHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "id")
	memberID := chi.URLParam(r, "memberId")
	userID := middleware.GetUserID(r.Context())

	chat, err := h.chatRepo.GetByID(r.Context(), chatID)
	if err != nil {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}
	if chat.ChatType != model.ChatTypeGroup {
		writeError(w, http.StatusBadRequest, "only group chats support removing members")
		return
	}
	if chat.SystemKey == repository.GeneralChatSystemKey {
		writeError(w, http.StatusForbidden, "cannot remove members from this chat")
		return
	}

	// Only admin or the member themselves can remove
	role, err := h.chatRepo.GetMemberRole(r.Context(), chatID, userID)
	if err != nil {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}
	if role != "admin" && userID != memberID {
		writeError(w, http.StatusForbidden, "only admin can remove members")
		return
	}

	if err := h.chatRepo.RemoveMember(r.Context(), chatID, memberID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	removed, _ := h.userRepo.GetByID(r.Context(), memberID)
	removedName := memberID
	if removed != nil {
		removedName = removed.Username
	}
	actor, _ := h.userRepo.GetByID(r.Context(), userID)
	actorName := ""
	if actor != nil {
		actorName = actor.Username
	}
	now := time.Now().UTC()
	sysContent := actorName + " исключил(а) " + removedName + " из группы"
	sysMsg := &model.Message{
		ID:          uuid.New().String(),
		ChatID:      chatID,
		SenderID:    userID,
		Content:     sysContent,
		ContentType: model.ContentTypeSystem,
		Status:      model.MessageStatusSent,
		CreatedAt:   now,
	}
	if err := h.msgRepo.Create(r.Context(), sysMsg); err != nil {
		logger.Errorf("removeMember system message chat=%s: %v", chatID, err)
	} else {
		sysMsg.Sender = &model.UserPublic{ID: userID, Username: actorName}
		h.hub.BroadcastToChat(r.Context(), chatID, ws.OutgoingMessage{Type: ws.EventNewMessage, Payload: sysMsg})
	}
	h.hub.BroadcastToChat(r.Context(), chatID, ws.OutgoingMessage{
		Type: ws.EventMemberRemoved,
		Payload: ws.MemberRemovedPayload{
			ChatID: chatID, UserID: memberID, Username: removedName,
			IsLeave: false, ActorName: actorName,
		},
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// LeaveChat lets a user leave a group chat.
func (h *ChatHandler) LeaveChat(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	chat, err := h.chatRepo.GetByID(r.Context(), chatID)
	if err != nil {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}
	if chat.SystemKey == repository.GeneralChatSystemKey {
		writeError(w, http.StatusForbidden, "cannot leave this chat")
		return
	}

	isMember, err := h.chatRepo.IsMember(r.Context(), chatID, userID)
	if err != nil || !isMember {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}

	if err := h.chatRepo.RemoveMember(r.Context(), chatID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to leave chat")
		return
	}

	leaver, _ := h.userRepo.GetByID(r.Context(), userID)
	leaverName := userID
	if leaver != nil {
		leaverName = leaver.Username
	}
	now := time.Now().UTC()
	sysContent := leaverName + " покинул(а) группу"
	sysMsg := &model.Message{
		ID:          uuid.New().String(),
		ChatID:      chatID,
		SenderID:    userID,
		Content:     sysContent,
		ContentType: model.ContentTypeSystem,
		Status:      model.MessageStatusSent,
		CreatedAt:   now,
	}
	if err := h.msgRepo.Create(r.Context(), sysMsg); err != nil {
		logger.Errorf("leaveChat system message chat=%s: %v", chatID, err)
	} else {
		sysMsg.Sender = &model.UserPublic{ID: userID, Username: leaverName}
		h.hub.BroadcastToChat(r.Context(), chatID, ws.OutgoingMessage{Type: ws.EventNewMessage, Payload: sysMsg})
	}
	h.hub.BroadcastToChat(r.Context(), chatID, ws.OutgoingMessage{
		Type: ws.EventMemberRemoved,
		Payload: ws.MemberRemovedPayload{
			ChatID: chatID, UserID: userID, Username: leaverName,
			IsLeave: true, ActorName: "",
		},
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ChatHandler) enrichChat(ctx context.Context, chat *model.Chat, userID string) (*model.ChatWithLastMessage, error) {
	members, err := h.chatRepo.GetMembers(ctx, chat.ID)
	if err != nil {
		return nil, err
	}

	pubMembers := make([]model.UserPublic, 0, len(members))
	for _, m := range members {
		pubMembers = append(pubMembers, m.ToPublic())
	}

	lastMsg, err := h.msgRepo.GetLastMessageForUser(ctx, chat.ID, userID)
	if err != nil {
		logger.Errorf("enrichChat get last message chat=%s: %v", chat.ID, err)
	}

	unread, err := h.chatRepo.GetUnreadCount(ctx, chat.ID, userID)
	if err != nil {
		logger.Errorf("enrichChat get unread count chat=%s: %v", chat.ID, err)
	}

	muted, _, err := h.chatRepo.GetMemberSettings(ctx, chat.ID, userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		logger.Errorf("enrichChat get member settings chat=%s user=%s: %v", chat.ID, userID, err)
	}

	return &model.ChatWithLastMessage{
		Chat:        *chat,
		LastMessage: lastMsg,
		Members:     pubMembers,
		UnreadCount: unread,
		Muted:       muted,
	}, nil
}
