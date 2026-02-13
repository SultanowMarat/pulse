package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/messenger/internal/middleware"
	"github.com/messenger/internal/push"
)

// PushHandler обрабатывает подписку на пуш-уведомления (сессия обязательна).
type PushHandler struct {
	client PushSubscriptionClient
}

type PushSubscriptionClient interface {
	Subscribe(ctx context.Context, userID string, sub push.PushSubscription) error
	Unsubscribe(ctx context.Context, userID, endpoint string) error
}

// NewPushHandler создаёт обработчик push.
func NewPushHandler(client PushSubscriptionClient) *PushHandler {
	return &PushHandler{client: client}
}

// SubscribeRequest — тело от фронта (subscription из PushManager.getSubscription()).
type SubscribeRequest struct {
	Subscription push.PushSubscription `json:"subscription"`
}

// Subscribe сохраняет подписку на push-сервисе для текущего пользователя.
func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusServiceUnavailable, "push service disabled")
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Subscription.Endpoint == "" || req.Subscription.Keys.P256dh == "" || req.Subscription.Keys.Auth == "" {
		writeError(w, http.StatusBadRequest, "subscription.endpoint and subscription.keys required")
		return
	}
	if err := h.client.Subscribe(r.Context(), userID, req.Subscription); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to subscribe")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnsubscribeRequest — тело для отписки по endpoint.
type UnsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// Unsubscribe удаляет подписку.
func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusServiceUnavailable, "push service disabled")
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req UnsubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "endpoint required")
		return
	}
	if err := h.client.Unsubscribe(r.Context(), userID, req.Endpoint); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unsubscribe")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
