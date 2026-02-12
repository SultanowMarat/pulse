package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/messenger/internal/config"
	"github.com/messenger/internal/fileserver"
)

type FileHandler struct {
	cfg        *config.Config
	fileSvc    *fileserver.Service
	fileClient *http.Client
	fileBase   string
}

func NewFileHandler(cfg *config.Config) *FileHandler {
	h := &FileHandler{cfg: cfg}
	if cfg.FileServiceURL == "" {
		h.fileSvc = fileserver.New(cfg.UploadDir, cfg.MaxUploadSize)
	} else {
		h.fileClient = &http.Client{Timeout: 60 * time.Second}
		h.fileBase = strings.TrimSuffix(cfg.FileServiceURL, "/")
	}
	return h
}

type FileUploadResponse struct {
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if h.fileSvc != nil {
		r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadSize)
		h.fileSvc.Upload(w, r)
		return
	}
	// Прокси на микросервис файлов (Content-Length обязателен для корректного парсинга multipart)
	proxyURL := h.fileBase + "/upload"
	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, proxyURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	proxyReq.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadSize)
	if r.ContentLength > 0 {
		proxyReq.ContentLength = r.ContentLength
	}
	resp, err := h.fileClient.Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "file service unavailable")
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

func (h *FileHandler) Serve(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(chi.URLParam(r, "filename"))
	if h.fileSvc != nil {
		h.fileSvc.Serve(w, r, filename)
		return
	}
	// Прокси GET на микросервис файлов
	rawQuery := ""
	if name := r.URL.Query().Get("name"); name != "" {
		rawQuery = "name=" + url.QueryEscape(name)
	}
	proxyURL := h.fileBase + "/files/" + url.PathEscape(filename)
	if rawQuery != "" {
		proxyURL += "?" + rawQuery
	}
	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, proxyURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if v := r.Header.Get("If-None-Match"); v != "" {
		proxyReq.Header.Set("If-None-Match", v)
	}
	if v := r.Header.Get("If-Modified-Since"); v != "" {
		proxyReq.Header.Set("If-Modified-Since", v)
	}
	resp, err := h.fileClient.Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "file service unavailable")
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Content-Type") ||
			strings.EqualFold(k, "Content-Disposition") || strings.EqualFold(k, "ETag") ||
			strings.EqualFold(k, "Last-Modified") || strings.EqualFold(k, "Cache-Control") {
			w.Header()[k] = v
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// DeleteByMessageFileURL removes physical file by message file URL (e.g. /api/files/<filename>).
func (h *FileHandler) DeleteByMessageFileURL(ctx context.Context, fileURL string) error {
	filename := extractFilenameFromFileURL(fileURL)
	if filename == "" {
		return nil
	}

	if h.fileSvc != nil {
		return h.fileSvc.Delete(ctx, filename)
	}

	proxyURL := h.fileBase + "/files/" + url.PathEscape(filename)
	proxyReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, proxyURL, nil)
	if err != nil {
		return fmt.Errorf("build delete request: %w", err)
	}
	resp, err := h.fileClient.Do(proxyReq)
	if err != nil {
		return fmt.Errorf("delete file request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("delete file failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func extractFilenameFromFileURL(fileURL string) string {
	u := strings.TrimSpace(fileURL)
	if u == "" {
		return ""
	}
	if parsed, err := url.Parse(u); err == nil {
		u = parsed.Path
	}
	u = strings.TrimPrefix(u, "/api/files/")
	return filepath.Base(strings.TrimSpace(u))
}
