package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/service"

	"github.com/go-chi/chi/v5"
)

type AuthHandler struct {
	otpSvc *service.OTPAuthService
}

func NewAuthHandler(otpSvc *service.OTPAuthService) *AuthHandler {
	return &AuthHandler{otpSvc: otpSvc}
}

func (h *AuthHandler) RequestCode(w http.ResponseWriter, r *http.Request) {
	if h.otpSvc == nil {
		writeError(w, http.StatusNotImplemented, "auth service unavailable")
		return
	}
	var req service.RequestCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}

	resp, err := h.otpSvc.RequestCode(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidLoginKey) {
			writeError(w, http.StatusUnauthorized, "invalid or expired login key")
			return
		}
		if errors.Is(err, service.ErrRateLimitExceeded) {
			writeError(w, http.StatusTooManyRequests, "too many requests, please retry later")
			return
		}
		if errors.Is(err, service.ErrInvalidEmail) {
			writeError(w, http.StatusBadRequest, "invalid email format")
			return
		}
		if errors.Is(err, service.ErrUserDisabled) {
			writeError(w, http.StatusForbidden, "user access is disabled")
			return
		}
		if errors.Is(err, service.ErrUserNotInvited) {
			writeError(w, http.StatusForbidden, "user is not invited")
			return
		}
		logger.Errorf("request-code send failed for %s: %v", req.Email, err)
		writeError(w, http.StatusInternalServerError, "failed to send code")
		return
	}
	if resp != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	if h.otpSvc == nil {
		writeError(w, http.StatusNotImplemented, "auth service unavailable")
		return
	}
	var req service.VerifyCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.otpSvc.VerifyCode(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidOTP) {
			writeError(w, http.StatusUnauthorized, "invalid or expired code")
			return
		}
		if errors.Is(err, service.ErrUserDisabled) {
			writeError(w, http.StatusForbidden, "user access is disabled")
			return
		}
		if errors.Is(err, service.ErrUserNotInvited) {
			writeError(w, http.StatusForbidden, "user is not invited")
			return
		}
		logger.Errorf("verify-code error email=%s device_id=%s: %v", req.Email, req.DeviceID, err)
		msg := "verification failed"
		if os.Getenv("APP_ENV") != "production" && os.Getenv("DEBUG") != "" {
			msg = msg + ": " + strings.ReplaceAll(err.Error(), "\n", " ")
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
	if h.otpSvc == nil {
		writeError(w, http.StatusNotImplemented, "auth service unavailable")
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	list, err := h.otpSvc.ListSessions(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": list})
}

func (h *AuthHandler) LogoutSession(w http.ResponseWriter, r *http.Request) {
	if h.otpSvc == nil {
		writeError(w, http.StatusNotImplemented, "auth service unavailable")
		return
	}
	userID := middleware.GetUserID(r.Context())
	sessionID := middleware.GetSessionID(r.Context())
	if userID == "" || sessionID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ok, err := h.otpSvc.LogoutSession(r.Context(), userID, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to logout session")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) LogoutAllSessions(w http.ResponseWriter, r *http.Request) {
	if h.otpSvc == nil {
		writeError(w, http.StatusNotImplemented, "auth service unavailable")
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_, err := h.otpSvc.LogoutAllSessions(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to logout sessions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// InternalLogoutUserSessions revokes all sessions for a specific user.
// Protected by middleware.InternalOnly; intended to be called by API service.
func (h *AuthHandler) InternalLogoutUserSessions(w http.ResponseWriter, r *http.Request) {
	if h.otpSvc == nil {
		writeError(w, http.StatusNotImplemented, "auth service unavailable")
		return
	}
	userID := chi.URLParam(r, "id")
	if strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "user id required")
		return
	}
	n, err := h.otpSvc.LogoutAllSessions(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to logout sessions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "revoked": n})
}

type ValidateRequest struct {
	SessionID string `json:"session_id"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Body      string `json:"body"`
}

type ValidateResponse struct {
	UserID string `json:"user_id"`
}

func ValidateSession(otpSvc *service.OTPAuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ValidateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		userID, err := otpSvc.ValidateRequest(r.Context(), req.SessionID, req.Timestamp, req.Signature, req.Method, req.Path, req.Body)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		writeJSON(w, http.StatusOK, ValidateResponse{UserID: userID})
	}
}
