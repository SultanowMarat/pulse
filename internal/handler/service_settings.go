package handler

import (
	"encoding/json"
	"net/http"

	"github.com/messenger/internal/middleware"
	"github.com/messenger/internal/model"
	"github.com/messenger/internal/repository"
	"github.com/messenger/internal/runtime"
)

type ServiceSettingsHandler struct {
	permRepo *repository.PermissionRepository
	repo     *repository.ServiceSettingsRepository
	hub      interface{ KickAll() }
}

func NewServiceSettingsHandler(permRepo *repository.PermissionRepository, repo *repository.ServiceSettingsRepository, hub interface{ KickAll() }) *ServiceSettingsHandler {
	return &ServiceSettingsHandler{permRepo: permRepo, repo: repo, hub: hub}
}

func (h *ServiceSettingsHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	userID := middleware.GetUserID(r.Context())
	perm, err := h.permRepo.GetByUserID(r.Context(), userID)
	if err != nil || !perm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can access service settings")
		return false
	}
	return true
}

func (h *ServiceSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	s, err := h.repo.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load service settings")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *ServiceSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	prev, _ := runtime.GetServiceSettings()
	var req model.ServiceSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	updated, err := h.repo.Upsert(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save service settings")
		return
	}
	runtime.SetServiceSettings(*updated, updated.UpdatedAt)
	// Some WS settings are applied on new connections; to apply them immediately for everyone,
	// we reconnect clients on change.
	if h.hub != nil {
		if prev.WSSendBufferSize != updated.WSSendBufferSize ||
			prev.WSMaxMessageSize != updated.WSMaxMessageSize ||
			prev.WSWriteTimeout != updated.WSWriteTimeout ||
			prev.WSPongTimeout != updated.WSPongTimeout ||
			prev.MaxWSConnections != updated.MaxWSConnections {
			h.hub.KickAll()
		}
	}
	writeJSON(w, http.StatusOK, updated)
}
