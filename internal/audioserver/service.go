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
	"github.com/pulse/internal/logger"
)

//  07Ñ€5ÑˆÑ‘==Ñ‹5 Ñ€0AÑˆ8Ñ€5=8O 8 MIME 4;O 3>;>A>2Ñ‹Ñ… A>>1Ñ‰5=89 (:0: 2 Telegram: opus/webm, ogg, m4a).
var allowedExt = map[string]bool{
	".ogg": true, ".oga": true, ".webm": true, ".m4a": true, ".mp4": true,
}

var allowedMime = map[string]bool{
	"audio/ogg": true, "audio/webm": true, "audio/mp4": true, "audio/mpeg": true,
	"audio/x-m4a": true, "video/webm": true, "audio/opus": true,
	"audio/aac": true, "audio/x-aac": true,
}

const maxUploadSize = 25 << 20 // 25 MB

// UploadResponse â€” >Ñ‚25Ñ‚ ?>A;5 ÑƒA?5Ñˆ=>9 703Ñ€Ñƒ7:8.
type UploadResponse struct {
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

// Service >1Ñ€010Ñ‚Ñ‹205Ñ‚ 703Ñ€Ñƒ7:Ñƒ 8 Ñ€0740Ñ‡Ñƒ 3>;>A>2Ñ‹Ñ… A>>1Ñ‰5=89.
type Service struct {
	UploadDir     string
	MaxUploadSize int64
}

// New A>740Ñ‘Ñ‚ A5Ñ€28A A 7040==Ñ‹< :0Ñ‚0;>3>< 8 ;8<8Ñ‚>< Ñ€07<5Ñ€0 (2 109Ñ‚0Ñ…).
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

// Upload >1Ñ€010Ñ‚Ñ‹205Ñ‚ POST multipart/form-data A ?>;5< "file" (Ñ‚>;ÑŒ:> 0Ñƒ48>).
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

	// >Ñ€<0;870Ñ†8O 8<5=8: ?Ñ€>15; <>3 ?Ñ€89Ñ‚8 :0: "+"
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
	// Ð•A;8 Content-Type ?ÑƒAÑ‚>9 â€” ?>;0305<AO =0 Ñ€0AÑˆ8Ñ€5=85 (Ñ‡0AÑ‚ÑŒ 1Ñ€0Ñƒ75Ñ€>2 =5 2Ñ‹AÑ‚02;O5Ñ‚ Ñ‚8? Ñƒ Ñ‡0AÑ‚8)

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

// Serve >Ñ‚40Ñ‘Ñ‚ Ñ„09; ?> 8<5=8 (4;O 2>A?Ñ€>872545=8O).
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
