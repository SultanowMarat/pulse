package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/pulse/internal/email"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/model"
	"github.com/pulse/internal/repository"
)

type AdminHandler struct {
	permRepo         *repository.PermissionRepository
	mailSettingsRepo *repository.MailSettingsRepository
	fileSettingsRepo *repository.FileSettingsRepository
	defaultUploadMB  int
}

func NewAdminHandler(
	permRepo *repository.PermissionRepository,
	mailSettingsRepo *repository.MailSettingsRepository,
	fileSettingsRepo *repository.FileSettingsRepository,
	defaultUploadMB int,
) *AdminHandler {
	return &AdminHandler{
		permRepo:         permRepo,
		mailSettingsRepo: mailSettingsRepo,
		fileSettingsRepo: fileSettingsRepo,
		defaultUploadMB:  defaultUploadMB,
	}
}

func (h *AdminHandler) isAdministrator(r *http.Request) (bool, error) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		return false, fmt.Errorf("unauthorized")
	}
	perm, err := h.permRepo.GetByUserID(r.Context(), userID)
	if err != nil {
		return false, err
	}
	return perm.Administrator, nil
}

func (h *AdminHandler) GetMailSettings(w http.ResponseWriter, r *http.Request) {
	isAdmin, err := h.isAdministrator(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "only administrator can manage mail settings")
		return
	}
	ms, err := h.mailSettingsRepo.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load mail settings")
		return
	}
	writeJSON(w, http.StatusOK, ms)
}

type UpdateMailSettingsRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromEmail string `json:"from_email"`
	FromName  string `json:"from_name"`
}

func (h *AdminHandler) UpdateMailSettings(w http.ResponseWriter, r *http.Request) {
	isAdmin, err := h.isAdministrator(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "only administrator can manage mail settings")
		return
	}

	var req UpdateMailSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	ms := &model.MailSettings{
		Host:      strings.TrimSpace(req.Host),
		Port:      req.Port,
		Username:  strings.TrimSpace(req.Username),
		Password:  req.Password,
		FromEmail: strings.TrimSpace(strings.ToLower(req.FromEmail)),
		FromName:  strings.TrimSpace(req.FromName),
	}

	allEmpty := ms.Host == "" && ms.Port == 0 && ms.Username == "" && strings.TrimSpace(ms.Password) == "" && ms.FromEmail == "" && ms.FromName == ""
	if !allEmpty {
		if !ms.IsConfigured() {
			writeError(w, http.StatusBadRequest, "for SMTP fill host, port, username and password")
			return
		}
		if ms.FromEmail != "" {
			if _, err := mail.ParseAddress(ms.FromEmail); err != nil {
				writeError(w, http.StatusBadRequest, "invalid from_email")
				return
			}
		}
	}

	if err := h.mailSettingsRepo.Upsert(r.Context(), ms); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save mail settings")
		return
	}
	writeJSON(w, http.StatusOK, ms)
}

type TestMailRequest struct {
	ToEmail string `json:"to_email"`
}

func (h *AdminHandler) SendTestMail(w http.ResponseWriter, r *http.Request) {
	isAdmin, err := h.isAdministrator(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "only administrator can manage mail settings")
		return
	}

	var req TestMailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	to := strings.TrimSpace(strings.ToLower(req.ToEmail))
	if to == "" {
		writeError(w, http.StatusBadRequest, "to_email required")
		return
	}
	if _, err := mail.ParseAddress(to); err != nil {
		writeError(w, http.StatusBadRequest, "invalid to_email")
		return
	}

	smtpCfg, err := h.mailSettingsRepo.GetSMTPConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load mail settings")
		return
	}
	if smtpCfg == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error_code": "SMTP_NOT_CONFIGURED",
			"error":      "ÐŸ>Ñ‡Ñ‚0 =5 =0AÑ‚Ñ€>5=0. Ð—0?>;=8Ñ‚5 SMTP-?>;O.",
		})
		return
	}

	sender := email.NewSender(smtpCfg)
	if err := sender.SendTest(r.Context(), to); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error_code": "SMTP_SEND_FAILED",
			"error":      "5 Ñƒ40;>AÑŒ >Ñ‚?Ñ€028Ñ‚ÑŒ Ñ‚5AÑ‚>2>5 ?8AÑŒ<>",
			"detail":     err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type UpdateFileSettingsRequest struct {
	MaxFileSizeMB int `json:"max_file_size_mb"`
}

func (h *AdminHandler) GetFileSettings(w http.ResponseWriter, r *http.Request) {
	isAdmin, err := h.isAdministrator(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "only administrator can manage file settings")
		return
	}
	fs, err := h.fileSettingsRepo.Get(r.Context(), h.defaultUploadMB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load file settings")
		return
	}
	writeJSON(w, http.StatusOK, fs)
}

func (h *AdminHandler) UpdateFileSettings(w http.ResponseWriter, r *http.Request) {
	isAdmin, err := h.isAdministrator(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "only administrator can manage file settings")
		return
	}
	var req UpdateFileSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.MaxFileSizeMB < 1 || req.MaxFileSizeMB > 200 {
		writeError(w, http.StatusBadRequest, "max_file_size_mb must be between 1 and 200")
		return
	}
	fs, err := h.fileSettingsRepo.Upsert(r.Context(), req.MaxFileSizeMB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file settings")
		return
	}
	writeJSON(w, http.StatusOK, fs)
}
