package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/messenger/internal/config"
	"github.com/messenger/internal/logger"
)

// AudioHandler проксирует загрузку и раздачу голосовых сообщений в микросервис audio.
type AudioHandler struct {
	audioClient *http.Client
	audioBase   string
}

// NewAudioHandler создаёт handler, проксирующий в аудио-сервис. Если audioServiceURL пустой — возвращается nil.
func NewAudioHandler(cfg *config.Config) *AudioHandler {
	if cfg.AudioServiceURL == "" {
		return nil
	}
	return &AudioHandler{
		audioClient: &http.Client{Timeout: 60 * time.Second},
		audioBase:   strings.TrimSuffix(cfg.AudioServiceURL, "/"),
	}
}

// Upload проксирует POST на микросервис audio (multipart "file").
func (h *AudioHandler) Upload(w http.ResponseWriter, r *http.Request) {
	proxyURL := h.audioBase + "/upload"
	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, proxyURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	proxyReq.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	if r.ContentLength > 0 {
		proxyReq.ContentLength = r.ContentLength
	}
	resp, err := h.audioClient.Do(proxyReq)
	if err != nil {
		logger.Errorf("audio upload proxy: request failed: %v", err)
		writeError(w, http.StatusBadGateway, "audio service unavailable")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		logger.Errorf("audio upload proxy: upstream status=%d body=%s", resp.StatusCode, bytes.TrimSpace(body))
		w.WriteHeader(resp.StatusCode)
		for k, v := range resp.Header {
			if strings.EqualFold(k, "Content-Type") {
				w.Header()[k] = v
				break
			}
		}
		if len(body) > 0 {
			w.Write(body)
		}
		return
	}
	logger.Infof("audio upload proxy: success status=%d", resp.StatusCode)
	for k, v := range resp.Header {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Content-Type") {
			w.Header()[k] = v
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// Serve проксирует GET на микросервис audio (раздача файла по имени).
func (h *AudioHandler) Serve(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(chi.URLParam(r, "filename"))
	if filename == "" || strings.Contains(filename, "..") {
		writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}
	proxyURL := h.audioBase + "/audio/" + url.PathEscape(filename)
	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, proxyURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp, err := h.audioClient.Do(proxyReq)
	if err != nil {
		logger.Errorf("audio serve proxy: request failed: %v", err)
		writeError(w, http.StatusBadGateway, "audio service unavailable")
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Content-Type") ||
			strings.EqualFold(k, "Content-Disposition") {
			w.Header()[k] = v
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
