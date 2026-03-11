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
	"github.com/pulse/internal/config"
	"github.com/pulse/internal/logger"
)

// AudioHandler ?Ñ€>:A8Ñ€Ñƒ5Ñ‚ 703Ñ€Ñƒ7:Ñƒ 8 Ñ€0740Ñ‡Ñƒ 3>;>A>2Ñ‹Ñ… A>>1Ñ‰5=89 2 <8:Ñ€>A5Ñ€28A audio.
type AudioHandler struct {
	audioClient *http.Client
	audioBase   string
}

// NewAudioHandler A>740Ñ‘Ñ‚ handler, ?Ñ€>:A8Ñ€ÑƒÑŽÑ‰89 2 0Ñƒ48>-A5Ñ€28A. Ð•A;8 audioServiceURL ?ÑƒAÑ‚>9 â€” 2>72Ñ€0Ñ‰05Ñ‚AO nil.
func NewAudioHandler(cfg *config.Config) *AudioHandler {
	if cfg.AudioServiceURL == "" {
		return nil
	}
	return &AudioHandler{
		audioClient: &http.Client{Timeout: 60 * time.Second},
		audioBase:   strings.TrimSuffix(cfg.AudioServiceURL, "/"),
	}
}

// Upload ?Ñ€>:A8Ñ€Ñƒ5Ñ‚ POST =0 <8:Ñ€>A5Ñ€28A audio (multipart "file").
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

// Serve ?Ñ€>:A8Ñ€Ñƒ5Ñ‚ GET =0 <8:Ñ€>A5Ñ€28A audio (Ñ€0740Ñ‡0 Ñ„09;0 ?> 8<5=8).
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
