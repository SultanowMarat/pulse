package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/push"
)

// PushHandler >1Ñ€010Ñ‚Ñ‹205Ñ‚ ?>4?8A:Ñƒ =0 ?ÑƒÑˆ-Ñƒ254><;5=8O (A5AA8O >1O70Ñ‚5;ÑŒ=0).
type PushHandler struct {
	client PushSubscriptionClient
}

type PushSubscriptionClient interface {
	Subscribe(ctx context.Context, userID string, sub push.PushSubscription) error
	Unsubscribe(ctx context.Context, userID, endpoint string) error
}

// NewPushHandler A>740Ñ‘Ñ‚ >1Ñ€01>Ñ‚Ñ‡8: push.
func NewPushHandler(client PushSubscriptionClient) *PushHandler {
	return &PushHandler{client: client}
}

// SubscribeRequest â€” Ñ‚5;> >Ñ‚ Ñ„Ñ€>=Ñ‚0 (subscription 87 PushManager.getSubscription()).
type SubscribeRequest struct {
	Subscription push.PushSubscription `json:"subscription"`
}

// Subscribe A>Ñ…Ñ€0=O5Ñ‚ ?>4?8A:Ñƒ =0 push-A5Ñ€28A5 4;O Ñ‚5:ÑƒÑ‰53> ?>;ÑŒ7>20Ñ‚5;O.
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

// UnsubscribeRequest â€” Ñ‚5;> 4;O >Ñ‚?8A:8 ?> endpoint.
type UnsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// Unsubscribe Ñƒ40;O5Ñ‚ ?>4?8A:Ñƒ.
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
