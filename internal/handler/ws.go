package handler

import (
	"context"
	"net/url"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/runtime"
	"github.com/pulse/internal/ws"
)

type WSHandler struct {
	hub             *ws.Hub
}

// NewWSHandler A>740Ñ‘Ñ‚ >1Ñ€01>Ñ‚Ñ‡8: WebSocket. Origins 15Ñ€ÑƒÑ‚AO 87 runtime =0AÑ‚Ñ€>5:.
func NewWSHandler(hub *ws.Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

func (h *WSHandler) checkOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	// Always allow same-origin WS (current host/proto), even with strict allow-list.
	scheme := "http"
	if xfProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xfProto != "" {
		scheme = xfProto
	} else if r.TLS != nil {
		scheme = "https"
	}
	expectedOrigin := scheme + "://" + r.Host
	if strings.EqualFold(origin, expectedOrigin) {
		return true
	}

	allowed := runtime.AllowedOrigins()
	if allowed == "*" || allowed == "" {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil || originURL.Scheme == "" || originURL.Host == "" {
		return false
	}
	normalizedOrigin := strings.ToLower(originURL.Scheme + "://" + originURL.Host)
	for _, o := range strings.Split(allowed, ",") {
		candidate := strings.ToLower(strings.TrimSpace(o))
		if candidate == normalizedOrigin {
			return true
		}
	}
	return false
}

func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.checkOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return h.checkOrigin(r) },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Errorf("ws upgrade: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	client := ws.NewClient(h.hub, conn, userID)
	client.Start(ctx, cancel)
	h.hub.Register(client)
}
