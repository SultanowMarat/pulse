package fileserver

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pulse/internal/logger"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Ð‘;>:8Ñ€Ñƒ5< Ñ‚>;ÑŒ:> >?0A=Ñ‹5 Ñ€0AÑˆ8Ñ€5=8O (8A?>;=O5<Ñ‹5/A:Ñ€8?Ñ‚Ñ‹). ÐžAÑ‚0;ÑŒ=Ñ‹5 â€” Ñ€07Ñ€5Ñˆ5=Ñ‹.
var BlockedExt = map[string]bool{
	".exe": true, ".sh": true, ".js": true, ".bat": true, ".cmd": true,
	".php": true, ".py": true, ".rb": true,
}

// UploadResponse â€” >Ñ‚25Ñ‚ ?>A;5 ÑƒA?5Ñˆ=>9 703Ñ€Ñƒ7:8.
type UploadResponse struct {
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

// Service >1Ñ€010Ñ‚Ñ‹205Ñ‚ 703Ñ€Ñƒ7:Ñƒ 8 Ñ€0740Ñ‡Ñƒ Ñ„09;>2.
type Service struct {
	UploadDir string
	// maxUploadSize is dynamic (can be updated at runtime by admin settings).
	maxUploadSize atomic.Int64
}

// New A>740Ñ‘Ñ‚ A5Ñ€28A A 7040==Ñ‹< :0Ñ‚0;>3>< 8 ;8<8Ñ‚>< Ñ€07<5Ñ€0 (2 109Ñ‚0Ñ…).
func New(uploadDir string, maxUploadSize int64) *Service {
	s := &Service{UploadDir: uploadDir}
	if maxUploadSize <= 0 {
		maxUploadSize = 20 << 20
	}
	s.maxUploadSize.Store(maxUploadSize)
	return s
}

func (s *Service) SetMaxUploadSizeBytes(n int64) {
	if n <= 0 {
		return
	}
	s.maxUploadSize.Store(n)
}

func (s *Service) MaxUploadSizeBytes() int64 {
	n := s.maxUploadSize.Load()
	if n <= 0 {
		return 20 << 20
	}
	return n
}

func (s *Service) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Errorf("fileserver writeJSON: %v", err)
	}
}

func (s *Service) writeError(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, struct{ Error string }{Error: msg})
}

// Upload >1Ñ€010Ñ‚Ñ‹205Ñ‚ POST multipart/form-data A ?>;5< "file".
func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	maxSize := s.MaxUploadSizeBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	if err := r.ParseMultipartForm(maxSize); err != nil {
		s.writeError(w, http.StatusBadRequest, "file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Ð’ Ñ€O45 :;85=Ñ‚>2/?Ñ€>:A8 ?Ñ€>15; 2 8<5=8 :>48Ñ€Ñƒ5Ñ‚AO :0: "+"; =>Ñ€<0;87Ñƒ5< 4;O >Ñ‚>1Ñ€065=8O 8 Ñ€0AÑˆ8Ñ€5=8O.
	rawFilename := strings.ReplaceAll(header.Filename, "+", " ")
	ext := strings.ToLower(filepath.Ext(rawFilename))
	if BlockedExt[ext] {
		s.writeError(w, http.StatusBadRequest, "file type not allowed")
		return
	}

	head := make([]byte, 512)
	n, _ := io.ReadAtLeast(file, head, len(head))
	head = head[:n]
	if !matchMagic(ext, head) {
		s.writeError(w, http.StatusBadRequest, "file content does not match type")
		return
	}

	newName := uuid.New().String() + ext
	if err := os.MkdirAll(s.UploadDir, 0o755); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to create upload dir")
		return
	}

	// !>Ñ…Ñ€0=O5< 2 A60Ñ‚>< 2845 (.gz) 4;O M:>=><88 <5AÑ‚0
	dstPath := filepath.Join(s.UploadDir, newName+".gz")
	dst, err := os.Create(dstPath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	gz := gzip.NewWriter(dst)
	if _, err := gz.Write(head); err != nil {
		gz.Close()
		dst.Close()
		os.Remove(dstPath)
		s.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	if err := copyWithContext(ctx, gz, file); err != nil {
		gz.Close()
		dst.Close()
		os.Remove(dstPath)
		if ctx.Err() != nil {
			return
		}
		s.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	if err := gz.Close(); err != nil {
		dst.Close()
		os.Remove(dstPath)
		s.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	if err := dst.Close(); err != nil {
		os.Remove(dstPath)
		s.writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	contentType := "file"
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".heic" {
		contentType = "image"
	}

	// Ð˜<O 4;O >Ñ‚>1Ñ€065=8O: Ñ‚>;ÑŒ:> 107>20O Ñ‡0AÑ‚ÑŒ 157 ?ÑƒÑ‚8, 157>?0A=Ñ‹5 A8<2>;Ñ‹; 8=0Ñ‡5 â€” A35=5Ñ€8Ñ€>20==>5
	displayName := strings.TrimSpace(filepath.Base(rawFilename))
	if displayName == "" || safeFilename(displayName) == "" {
		displayName = newName
	} else {
		displayName = safeFilename(displayName)
	}

	s.writeJSON(w, http.StatusOK, UploadResponse{
		URL:         "/api/files/" + newName,
		FileName:    displayName,
		FileSize:    header.Size,
		ContentType: contentType,
	})
}

func matchMagic(ext string, head []byte) bool {
	switch ext {
	case ".jpg", ".jpeg":
		return len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF
	case ".png":
		return len(head) >= 8 && bytes.Equal(head[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	case ".gif":
		return len(head) >= 6 && (bytes.Equal(head[:6], []byte("GIF87a")) || bytes.Equal(head[:6], []byte("GIF89a")))
	case ".webp":
		return len(head) >= 12 && bytes.Equal(head[8:12], []byte("WEBP"))
	case ".heic":
		return len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp")) && (bytes.Equal(head[8:12], []byte("heic")) || bytes.Equal(head[8:12], []byte("heix")) || bytes.Equal(head[8:12], []byte("mif1")))
	case ".pdf":
		return len(head) >= 5 && bytes.Equal(head[:5], []byte("%PDF-"))
	case ".doc":
		return len(head) >= 8 && head[0] == 0xD0 && head[1] == 0xCF && head[2] == 0x11 && head[3] == 0xE0
	case ".docx":
		return len(head) >= 4 && head[0] == 0x50 && head[1] == 0x4B && (head[2] == 0x03 || head[2] == 0x05) && head[3] == 0x04
	case ".txt":
		return true
	}
	return true
}

// Serve >Ñ‚40Ñ‘Ñ‚ Ñ„09; ?> 8<5=8 (Ñ€070Ñ€Ñ…828Ñ€Ñƒ5Ñ‚ ?Ñ€8 >Ñ‚40Ñ‡5); query name= â€” >Ñ€838=0;ÑŒ=>5 8<O 4;O Content-Disposition.
func (s *Service) Serve(w http.ResponseWriter, r *http.Request, filename string) {
	filename = filepath.Base(filename)
	ext := filepath.Ext(filename)
	gzPath := filepath.Join(s.UploadDir, filename+".gz")
	plainPath := filepath.Join(s.UploadDir, filename)

	if ct := contentTypeByExt(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if origName := r.URL.Query().Get("name"); origName != "" {
		// Ð’ URL ?Ñ€>15; <>65Ñ‚ ?Ñ€8Ñ…>48Ñ‚ÑŒ :0: "+"; =>Ñ€<0;87Ñƒ5< 4;O A>Ñ…Ñ€0=5=8O 8<5=8 ?Ñ€8 A:0Ñ‡820=88 (UTF-8).
		origName = strings.TrimSpace(strings.ReplaceAll(origName, "+", " "))
		safe := safeFilename(origName)
		if safe != "" {
			disp := "attachment; filename*=UTF-8''" + url.QueryEscape(safe)
			// Legacy filename= A ASCII 8A:0605Ñ‚ :8Ñ€8;;8Ñ†Ñƒ (?>4Ñ‡Ñ‘Ñ€:820=8O) â€” =5 4>102;O5< 53>,
			// Ñ‡Ñ‚>1Ñ‹ ?0=5;ÑŒ 703Ñ€Ñƒ7:8 1Ñ€0Ñƒ75Ñ€0 ?>:07Ñ‹20;0 8<O 87 filename*=UTF-8''.
			if ascii := asciiFallbackFilename(safe); ascii != "" && ascii == safe {
				disp = "attachment; filename=\"" + ascii + "\"; " + disp
			}
			w.Header().Set("Content-Disposition", disp)
		}
	}

	// !=0Ñ‡0;0 A60Ñ‚Ñ‹9 .gz, 8=0Ñ‡5 â€” >1Ñ‹Ñ‡=Ñ‹9 Ñ„09; (>1Ñ€0Ñ‚=0O A>2<5AÑ‚8<>AÑ‚ÑŒ)
	if fi, err := os.Stat(gzPath); err == nil {
		etag := buildFileETag(filename, fi)
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))
		w.Header().Set("Cache-Control", "public, max-age=300")
		if isNotModified(r, etag, fi.ModTime()) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		f, err := os.Open(gzPath)
		if err != nil {
			s.writeError(w, http.StatusNotFound, "file not found")
			return
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "failed to read file")
			return
		}
		defer gz.Close()
		w.WriteHeader(http.StatusOK)
		io.Copy(w, gz)
		return
	}
	if fi, err := os.Stat(plainPath); err == nil {
		etag := buildFileETag(filename, fi)
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))
		w.Header().Set("Cache-Control", "public, max-age=300")
		if isNotModified(r, etag, fi.ModTime()) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		f, err := os.Open(plainPath)
		if err != nil {
			s.writeError(w, http.StatusNotFound, "file not found")
			return
		}
		defer f.Close()
		w.WriteHeader(http.StatusOK)
		io.Copy(w, f)
		return
	}
	s.writeError(w, http.StatusNotFound, "file not found")
}

// Delete physically removes file from storage by filename.
// It tries both compressed (.gz) and plain variants for backward compatibility.
func (s *Service) Delete(ctx context.Context, filename string) error {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return fmt.Errorf("invalid filename")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	gzPath := filepath.Join(s.UploadDir, filename+".gz")
	plainPath := filepath.Join(s.UploadDir, filename)

	errGz := os.Remove(gzPath)
	errPlain := os.Remove(plainPath)

	if err := ctx.Err(); err != nil {
		return err
	}
	if errGz != nil && !errors.Is(errGz, os.ErrNotExist) {
		return errGz
	}
	if errPlain != nil && !errors.Is(errPlain, os.ErrNotExist) {
		return errPlain
	}
	return nil
}

func contentTypeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".txt":
		return "text/plain"
	}
	return ""
}

// safeFilename >AÑ‚02;O5Ñ‚ 8<O Ñ„09;0 157>?0A=Ñ‹< 4;O Content-Disposition (157 Ñƒ?Ñ€02;OÑŽÑ‰8Ñ… A8<2>;>2 8 :02Ñ‹Ñ‡5:).
// ÐŸ>445Ñ€68205Ñ‚AO UTF-8, Ñ‡Ñ‚>1Ñ‹ A>Ñ…Ñ€0=OÑ‚ÑŒ :8Ñ€8;;8Ñ†Ñƒ 8 4Ñ€Ñƒ385 O7Ñ‹:8.
func safeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\r', '\n', '"', '\\', '/', '\x00':
			continue
		}
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// asciiFallbackFilename 2>72Ñ€0Ñ‰05Ñ‚ 8<O Ñ‚>;ÑŒ:> 87 ASCII 4;O legacy filename= 2 Content-Disposition.
// ÐŸÑ€>15;Ñ‹ 8 =5-ASCII 70<5=OÑŽÑ‚AO =0 ?>4Ñ‡Ñ‘Ñ€:820=85, Ñ‡Ñ‚>1Ñ‹ =5 ?>O2;O;>AÑŒ "+" 2 ?Ñ€54;>65==>< 8<5=8.
func asciiFallbackFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('_')
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("upload cancelled: %w", ctx.Err())
		default:
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return fmt.Errorf("write: %w", err)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read: %w", readErr)
		}
	}
}

func buildFileETag(name string, fi os.FileInfo) string {
	sum := sha256.Sum256([]byte(name + "|" + fi.ModTime().UTC().Format(time.RFC3339Nano) + "|" + fmt.Sprintf("%d", fi.Size())))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

func isNotModified(r *http.Request, etag string, lastModified time.Time) bool {
	ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match"))
	if ifNoneMatch != "" {
		for _, p := range strings.Split(ifNoneMatch, ",") {
			part := strings.TrimSpace(p)
			if part == "*" || part == etag || strings.TrimPrefix(part, "W/") == etag {
				return true
			}
		}
	}
	if ifNoneMatch == "" {
		if ims := strings.TrimSpace(r.Header.Get("If-Modified-Since")); ims != "" {
			if t, err := time.Parse(http.TimeFormat, ims); err == nil {
				return !lastModified.After(t.Add(time.Second))
			}
		}
	}
	return false
}
