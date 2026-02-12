package audioserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/messenger/internal/logger"
)

// Разрешённые расширения и MIME для голосовых сообщений (как в Telegram: opus/webm, ogg, m4a).
var allowedExt = map[string]bool{
	".ogg": true, ".oga": true, ".webm": true, ".m4a": true, ".mp4": true,
}

var allowedMime = map[string]bool{
	"audio/ogg": true, "audio/webm": true, "audio/mp4": true, "audio/mpeg": true,
	"audio/x-m4a": true, "video/webm": true, "audio/opus": true,
	"audio/aac": true, "audio/x-aac": true,
}

const maxUploadSize = 25 << 20 // 25 MB

// UploadResponse — ответ после успешной загрузки.
type UploadResponse struct {
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

// Service обрабатывает загрузку и раздачу голосовых сообщений.
type Service struct {
	UploadDir     string
	MaxUploadSize int64
}

// New создаёт сервис с заданным каталогом и лимитом размера (в байтах).
func New(uploadDir string, maxSize int64) *Service {
	if maxSize <= 0 || maxSize > maxUploadSize {
		maxSize = maxUploadSize
	}
	return &Service{UploadDir: uploadDir, MaxUploadSize: maxSize}
}

func (s *Service) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Errorf("audioserver writeJSON: %v", err)
	}
}

func (s *Service) writeError(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, struct{ Error string }{Error: msg})
}

func safeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '/' || r == '\\' || r == 0 {
			continue
		}
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// Upload обрабатывает POST multipart/form-data с полем "file" (только аудио).
func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxUploadSize)

	if err := r.ParseMultipartForm(s.MaxUploadSize); err != nil {
		logger.Errorf("audioserver upload: parse multipart: %v", err)
		s.writeError(w, http.StatusBadRequest, "file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		logger.Errorf("audioserver upload: form file: %v", err)
		s.writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Нормализация имени: пробел мог прийти как "+"
	rawFilename := strings.ReplaceAll(header.Filename, "+", " ")
	ext := strings.ToLower(filepath.Ext(rawFilename))
	if !allowedExt[ext] {
		logger.Errorf("audioserver upload: disallowed extension filename=%q ext=%q", header.Filename, ext)
		s.writeError(w, http.StatusBadRequest, "only audio files are allowed")
		return
	}

	ct := header.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	if ct != "" && !allowedMime[ct] {
		logger.Errorf("audioserver upload: disallowed content-type filename=%q content_type=%q", header.Filename, header.Header.Get("Content-Type"))
		s.writeError(w, http.StatusBadRequest, "only audio content type allowed")
		return
	}
	// Если Content-Type пустой — полагаемся на расширение (часть браузеров не выставляет тип у части)

	newName := uuid.New().String() + ext
	if err := os.MkdirAll(s.UploadDir, 0o755); err != nil {
		logger.Errorf("audioserver upload: mkdir %s: %v", s.UploadDir, err)
		s.writeError(w, http.StatusInternalServerError, "failed to create upload dir")
		return
	}

	dstPath := filepath.Join(s.UploadDir, newName)
	dst, err := os.Create(dstPath)
	if err != nil {
		logger.Errorf("audioserver upload: create %s: %v", dstPath, err)
		s.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	defer dst.Close()

	n, err := copyWithContext(ctx, dst, file)
	if err != nil {
		os.Remove(dstPath)
		if ctx.Err() != nil {
			return
		}
		logger.Errorf("audioserver upload: copy: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	displayName := strings.TrimSpace(filepath.Base(rawFilename))
	if displayName == "" || safeFilename(displayName) == "" {
		displayName = "voice" + ext
	} else {
		displayName = safeFilename(displayName)
	}

	logger.Infof("audioserver upload: ok filename=%s size=%d", newName, n)
	s.writeJSON(w, http.StatusOK, UploadResponse{
		URL:         "/api/audio/" + newName,
		FileName:    displayName,
		FileSize:    n,
		ContentType: "voice",
	})
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	var n int64
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return n, ctx.Err()
		default:
		}
		nr, err := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			n += int64(nw)
			if ew != nil {
				return n, ew
			}
		}
		if err != nil {
			if err == io.EOF {
				return n, nil
			}
			return n, err
		}
	}
}

// Serve отдаёт файл по имени (для воспроизведения).
func (s *Service) Serve(w http.ResponseWriter, r *http.Request, filename string) {
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	path := filepath.Join(s.UploadDir, filename)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ext := strings.ToLower(filepath.Ext(filename))
	ct := "audio/ogg"
	switch ext {
	case ".webm":
		ct = "audio/webm"
	case ".m4a", ".mp4":
		ct = "audio/mp4"
	case ".ogg", ".oga":
		ct = "audio/ogg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	http.ServeContent(w, r, filename, info.ModTime(), f)
}
