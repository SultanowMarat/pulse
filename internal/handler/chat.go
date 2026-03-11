package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pulse/internal/cache"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/model"
	"github.com/pulse/internal/repository"
	"github.com/pulse/internal/ws"
)

type ChatHandler struct {
	chatRepo *repository.ChatRepository
	userRepo *repository.UserRepository
	permRepo *repository.PermissionRepository
	msgRepo  *repository.MessageRepository
	hub      *ws.Hub
	fileH    *FileHandler
	cache    *cache.ChatCache
}

func NewChatHandler(chatRepo *repository.ChatRepository, userRepo *repository.UserRepository, permRepo *repository.PermissionRepository, msgRepo *repository.MessageRepository, hub *ws.Hub, fileH *FileHandler, cache *cache.ChatCache) *ChatHandler {
	return &ChatHandler{chatRepo: chatRepo, userRepo: userRepo, permRepo: permRepo, msgRepo: msgRepo, hub: hub, fileH: fileH, cache: cache}
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
	h.invalidateChatCaches(r.Context(), chat.ID, currentUserID, req.UserID)

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
	h.invalidateChatCaches(r.Context(), chat.ID, append([]string{currentUserID}, req.MemberIDs...)...)

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

	// Service group "Ðž1Ñ‰89 Ñ‡0Ñ‚": ensure every user is a member on first login.
	// This guarantees the chat appears in the "Ð’!Ð•" tab for everyone.
	if err := h.ensureGeneralChatMember(ctx, userID); err != nil {
		logger.Errorf("GetUserChats ensure general chat: %v", err)
	}

	if h.cache != nil {
		var cached []model.ChatWithLastMessage
		hit, err := h.cache.UserChats(ctx, userID, &cached)
		if err != nil {
			logger.Errorf("GetUserChats cache read user=%s: %v", userID, err)
		}
		if hit {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}

	result, err := h.buildUserChats(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chats")
		return
	}

	if h.cache != nil {
		if err := h.cache.SetUserChats(ctx, userID, result); err != nil {
			logger.Errorf("GetUserChats cache write user=%s: %v", userID, err)
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
	if h.cache != nil {
		if err := h.cache.InvalidateChatMembers(ctx, chat.ID); err != nil {
			logger.Errorf("general chat invalidate members cache chat=%s: %v", chat.ID, err)
		}
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
		Content:     "Ð”>102;5= ?>;ÑŒ7>20Ñ‚5;ÑŒ " + name,
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
	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, chat.ID)
	if err != nil {
		logger.Errorf("general chat join member ids chat=%s: %v", chat.ID, err)
		memberIDs = []string{userID}
	}
	h.invalidateChatCaches(ctx, chat.ID, memberIDs...)

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
	if h.cache != nil {
		if err := h.cache.InvalidateUserChats(r.Context(), userID); err != nil {
			logger.Errorf("SetMuted invalidate cache user=%s: %v", userID, err)
		}
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
	memberIDs, err := h.chatRepo.GetMemberIDs(r.Context(), chatID)
	if err != nil {
		logger.Errorf("ClearHistory member ids chat=%s: %v", chatID, err)
		memberIDs = []string{userID}
	}
	h.invalidateChatCaches(r.Context(), chatID, memberIDs...)

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

	// System "Ðž1Ñ‰89 Ñ‡0Ñ‚" can only be edited by global administrators.
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
	memberIDs, err := h.chatRepo.GetMemberIDs(r.Context(), chatID)
	if err != nil {
		logger.Errorf("UpdateChat member ids chat=%s: %v", chatID, err)
		memberIDs = []string{userID}
	}
	if h.cache != nil {
		if err := h.cache.InvalidateUserChats(r.Context(), memberIDs...); err != nil {
			logger.Errorf("UpdateChat invalidate user chats cache chat=%s: %v", chatID, err)
		}
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
	actorName := userID
	if actor != nil && actor.Username != "" {
		actorName = actor.Username
	}
	targetIDs := make([]string, 0, len(req.MemberIDs))
	seen := make(map[string]struct{}, len(req.MemberIDs))
	for _, rawID := range req.MemberIDs {
		uid := strings.TrimSpace(rawID)
		if uid == "" || uid == userID {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		targetIDs = append(targetIDs, uid)
	}
	if len(targetIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	existingIDs, err := h.chatRepo.GetMemberIDs(r.Context(), chatID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check current members")
		return
	}
	existing := make(map[string]struct{}, len(existingIDs))
	for _, mid := range existingIDs {
		existing[mid] = struct{}{}
	}

	now := time.Now().UTC()
	addedIDs := make([]string, 0, len(targetIDs))
	for _, uid := range targetIDs {
		if _, ok := existing[uid]; ok {
			continue
		}
		member := &model.ChatMember{ChatID: chatID, UserID: uid, Role: "member", JoinedAt: now}
		if err := h.chatRepo.AddMember(r.Context(), member); err != nil {
			logger.Errorf("addMember chat=%s user=%s: %v", chatID, uid, err)
			continue
		}
		addedIDs = append(addedIDs, uid)
		existing[uid] = struct{}{}
	}
	if len(addedIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if h.cache != nil {
		if err := h.cache.InvalidateChatMembers(r.Context(), chatID); err != nil {
			logger.Errorf("AddMembers invalidate members cache chat=%s: %v", chatID, err)
		}
	}

	addedNames := make(map[string]string, len(addedIDs))
	for _, uid := range addedIDs {
		addedNames[uid] = uid
		added, err := h.userRepo.GetByID(r.Context(), uid)
		if err == nil && added != nil && added.Username != "" {
			addedNames[uid] = added.Username
		}
	}

	for _, uid := range addedIDs {
		addedName := addedNames[uid]
		systemContent := actorName + " 4>1028;(0) " + addedName + " 2 3Ñ€Ñƒ??Ñƒ"
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
	memberIDs, err := h.chatRepo.GetMemberIDs(r.Context(), chatID)
	if err != nil {
		logger.Errorf("AddMembers member ids chat=%s: %v", chatID, err)
		memberIDs = append([]string{userID}, addedIDs...)
	}
	h.invalidateChatCaches(r.Context(), chatID, memberIDs...)

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
	if h.cache != nil {
		if err := h.cache.InvalidateChatMembers(r.Context(), chatID); err != nil {
			logger.Errorf("RemoveMember invalidate members cache chat=%s: %v", chatID, err)
		}
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
	sysContent := actorName + " 8A:;ÑŽÑ‡8;(0) " + removedName + " 87 3Ñ€Ñƒ??Ñ‹"
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
	memberIDs, err := h.chatRepo.GetMemberIDs(r.Context(), chatID)
	if err != nil {
		logger.Errorf("RemoveMember member ids chat=%s: %v", chatID, err)
		memberIDs = []string{userID, memberID}
	} else {
		memberIDs = append(memberIDs, memberID)
	}
	h.invalidateChatCaches(r.Context(), chatID, memberIDs...)

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
	if h.cache != nil {
		if err := h.cache.InvalidateChatMembers(r.Context(), chatID); err != nil {
			logger.Errorf("LeaveChat invalidate members cache chat=%s: %v", chatID, err)
		}
	}

	leaver, _ := h.userRepo.GetByID(r.Context(), userID)
	leaverName := userID
	if leaver != nil {
		leaverName = leaver.Username
	}
	now := time.Now().UTC()
	sysContent := leaverName + " ?>:8=Ñƒ;(0) 3Ñ€Ñƒ??Ñƒ"
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
	memberIDs, err := h.chatRepo.GetMemberIDs(r.Context(), chatID)
	if err != nil {
		logger.Errorf("LeaveChat member ids chat=%s: %v", chatID, err)
		memberIDs = []string{userID}
	} else {
		memberIDs = append(memberIDs, userID)
	}
	h.invalidateChatCaches(r.Context(), chatID, memberIDs...)

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

func (h *ChatHandler) buildUserChats(ctx context.Context, userID string) ([]model.ChatWithLastMessage, error) {
	chats, err := h.chatRepo.GetUserChats(ctx, userID)
	if err != nil {
		return nil, err
	}

	baseChats := make([]model.Chat, 0, len(chats)+1)
	for i := range chats {
		if chats[i].ChatType == model.ChatTypeNotes {
			continue
		}
		baseChats = append(baseChats, chats[i])
	}

	notesChat, err := h.chatRepo.GetOrCreateNotesChat(ctx, userID)
	if err != nil {
		logger.Errorf("GetUserChats get or create notes chat: %v", err)
	} else {
		if err := h.userRepo.AddFavorite(ctx, userID, notesChat.ID); err != nil {
			logger.Errorf("GetUserChats add notes to favorites: %v", err)
		}
		baseChats = append(baseChats, *notesChat)
	}

	return h.enrichChatsBatch(ctx, baseChats, userID), nil
}

func (h *ChatHandler) enrichChatsBatch(ctx context.Context, chats []model.Chat, userID string) []model.ChatWithLastMessage {
	result := make([]model.ChatWithLastMessage, 0, len(chats))
	if len(chats) == 0 {
		return result
	}

	chatIDs := make([]string, 0, len(chats))
	for i := range chats {
		chatIDs = append(chatIDs, chats[i].ID)
	}

	membersByChat, err := h.chatRepo.GetMembersByChatIDs(ctx, chatIDs)
	if err != nil {
		logger.Errorf("GetUserChats members batch: %v", err)
		membersByChat = make(map[string][]model.UserPublic)
	}

	lastByChat, err := h.msgRepo.GetLastMessagesForUserChats(ctx, userID, chatIDs)
	if err != nil {
		logger.Errorf("GetUserChats last messages batch: %v", err)
		lastByChat = make(map[string]*model.Message)
	}

	unreadByChat, err := h.chatRepo.GetUnreadCountsForUserChats(ctx, userID, chatIDs)
	if err != nil {
		logger.Errorf("GetUserChats unread batch: %v", err)
		unreadByChat = make(map[string]int)
	}

	mutedByChat, err := h.chatRepo.GetMutedMapForUserChats(ctx, userID, chatIDs)
	if err != nil {
		logger.Errorf("GetUserChats muted batch: %v", err)
		mutedByChat = make(map[string]bool)
	}

	for i := range chats {
		chat := chats[i]
		members := membersByChat[chat.ID]
		if members == nil {
			members = make([]model.UserPublic, 0)
		}
		result = append(result, model.ChatWithLastMessage{
			Chat:        chat,
			LastMessage: lastByChat[chat.ID],
			Members:     members,
			UnreadCount: unreadByChat[chat.ID],
			Muted:       mutedByChat[chat.ID],
		})
	}
	return result
}

func (h *ChatHandler) invalidateChatCaches(ctx context.Context, chatID string, userIDs ...string) {
	if h.cache == nil {
		return
	}
	if err := h.cache.InvalidateMessageLists(ctx, chatID); err != nil {
		logger.Errorf("invalidate message cache chat=%s: %v", chatID, err)
	}
	if err := h.cache.InvalidateChatMembers(ctx, chatID); err != nil {
		logger.Errorf("invalidate members cache chat=%s: %v", chatID, err)
	}
	if err := h.cache.InvalidateUserChats(ctx, userIDs...); err != nil {
		logger.Errorf("invalidate user chats cache chat=%s: %v", chatID, err)
	}
}
