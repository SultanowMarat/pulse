package handler

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/repository"
)

// BackupHandler creates and restores backups (zip) containing DB dump + mounted data directories.
// This is intended for Docker deployments where API has access to the same volumes.
type BackupHandler struct {
	permRepo    *repository.PermissionRepository
	dbURL       string
	uploadsDir  string
	audioDir    string
	vapidPath   string
	zipFileName string
}

func NewBackupHandler(permRepo *repository.PermissionRepository, dbURL, uploadsDir, audioDir, vapidPath string) *BackupHandler {
	return &BackupHandler{
		permRepo:    permRepo,
		dbURL:       strings.TrimSpace(dbURL),
		uploadsDir:  strings.TrimSpace(uploadsDir),
		audioDir:    strings.TrimSpace(audioDir),
		vapidPath:   strings.TrimSpace(vapidPath),
		zipFileName: "pulse-backup.zip",
	}
}

func (h *BackupHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	userID := middleware.GetUserID(r.Context())
	perm, err := h.permRepo.GetByUserID(r.Context(), userID)
	if err != nil || !perm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can access backups")
		return false
	}
	return true
}

type backupMeta struct {
	CreatedAt string `json:"created_at"`
	App       string `json:"app"`
	Version   string `json:"version"`
}

func (h *BackupHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	tmp, err := os.CreateTemp("", "pulse-backup-*.zip")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp file")
		return
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	if err := h.writeBackupZip(r.Context(), tmp); err != nil {
		logger.Errorf("backup: create zip: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create backup")
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finalize backup")
		return
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open backup")
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stat backup")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", h.zipFileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", st.Size()))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func (h *BackupHandler) writeBackupZip(ctx context.Context, out io.Writer) error {
	zw := zip.NewWriter(out)
	defer zw.Close()

	meta := backupMeta{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		App:       "pulse",
		Version:   "1",
	}
	if err := writeZipJSON(zw, "meta.json", meta); err != nil {
		return err
	}

	// DB dump (logical): db.sql
	if err := h.addDBDump(ctx, zw, "db.sql"); err != nil {
		return err
	}

	// Files (if mounted)
	if h.uploadsDir != "" {
		if err := addDirToZip(zw, h.uploadsDir, "uploads"); err != nil {
			return err
		}
	}
	if h.audioDir != "" {
		if err := addDirToZip(zw, h.audioDir, "audio"); err != nil {
			return err
		}
	}

	// VAPID keys file (optional)
	if h.vapidPath != "" {
		if err := addFileToZip(zw, h.vapidPath, filepath.ToSlash(filepath.Join("vapid", filepath.Base(h.vapidPath)))); err != nil {
			// Missing keys should not break backups.
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}

	return nil
}

func (h *BackupHandler) addDBDump(ctx context.Context, zw *zip.Writer, name string) error {
	if h.dbURL == "" {
		return fmt.Errorf("db url empty")
	}
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return fmt.Errorf("pg_dump not found in PATH")
	}
	w, err := zw.Create(name)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "pg_dump",
		"--dbname", h.dbURL,
		"--no-owner",
		"--no-privileges",
	)
	cmd.Stdout = w
	var stderr bytesLimited
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w: %s", err, stderr.String())
	}
	return nil
}

// RestoreBackup restores DB + volumes from an uploaded zip file.
// Expected entries:
// - db.sql
// - uploads/...
// - audio/...
// - vapid/...
func (h *BackupHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	// Stream multipart file to disk to avoid memory spikes.
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "expected multipart/form-data")
		return
	}
	tmp, err := os.CreateTemp("", "pulse-restore-*.zip")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp file")
		return
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	found := false
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid multipart")
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		found = true
		_, _ = io.Copy(tmp, part)
		_ = part.Close()
		break
	}
	if !found {
		writeError(w, http.StatusBadRequest, "file required")
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read upload")
		return
	}

	if err := h.restoreFromZip(r.Context(), tmpPath); err != nil {
		logger.Errorf("backup: restore: %v", err)
		writeError(w, http.StatusInternalServerError, "restore failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": " 5:><5=4Ñƒ5Ñ‚AO ?5Ñ€570?ÑƒAÑ‚8Ñ‚ÑŒ :>=Ñ‚59=5Ñ€ API (docker compose restart api), Ñ‡Ñ‚>1Ñ‹ ?Ñ€8 =5>1Ñ…>48<>AÑ‚8 ?Ñ€8<5=8;8AÑŒ <83Ñ€0Ñ†88 8 A5AA88 A8=Ñ…Ñ€>=878Ñ€>20;8AÑŒ A 2>AAÑ‚0=>2;5==>9 Ð‘Ð”.",
	})
}

func (h *BackupHandler) restoreFromZip(ctx context.Context, zipPath string) error {
	f, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		return err
	}

	// Stage files first (no changes to live dirs yet).
	stageRoot := func(target string) (string, func() error, error) {
		if strings.TrimSpace(target) == "" {
			return "", nil, nil
		}
		// For bind-mounted directories inside Docker (e.g. /app/uploads),
		// renaming the mountpoint itself will fail (EBUSY). To make restore work
		// with mounts, stage inside the target directory and later swap contents.
		if err := os.MkdirAll(target, 0o755); err != nil {
			return "", nil, err
		}
		tmpDir, err := os.MkdirTemp(target, ".restore-*")
		if err != nil {
			return "", nil, err
		}
		cleanup := func() error { return os.RemoveAll(tmpDir) }
		return tmpDir, cleanup, nil
	}

	stageUploads, cleanUploads, err := stageRoot(h.uploadsDir)
	if err != nil {
		return err
	}
	defer func() {
		if cleanUploads != nil {
			_ = cleanUploads()
		}
	}()
	stageAudio, cleanAudio, err := stageRoot(h.audioDir)
	if err != nil {
		return err
	}
	defer func() {
		if cleanAudio != nil {
			_ = cleanAudio()
		}
	}()
	var stageVapid string
	var cleanVapid func() error
	if strings.TrimSpace(h.vapidPath) != "" {
		stageVapid, cleanVapid, err = stageRoot(filepath.Dir(h.vapidPath))
		if err != nil {
			return err
		}
		defer func() {
			if cleanVapid != nil {
				_ = cleanVapid()
			}
		}()
	}

	var dbSQL *zip.File
	for _, zf := range zr.File {
		name := filepath.ToSlash(zf.Name)
		if name == "db.sql" {
			dbSQL = zf
			continue
		}
		if strings.HasPrefix(name, "uploads/") && stageUploads != "" {
			if err := extractZipEntry(zf, stageUploads, strings.TrimPrefix(name, "uploads/")); err != nil {
				return err
			}
		}
		if strings.HasPrefix(name, "audio/") && stageAudio != "" {
			if err := extractZipEntry(zf, stageAudio, strings.TrimPrefix(name, "audio/")); err != nil {
				return err
			}
		}
		if strings.HasPrefix(name, "vapid/") && stageVapid != "" {
			if err := extractZipEntry(zf, stageVapid, strings.TrimPrefix(name, "vapid/")); err != nil {
				return err
			}
		}
	}
	if dbSQL == nil {
		return fmt.Errorf("db.sql not found in backup")
	}

	// Restore DB first (most critical).
	if err := h.restoreDB(ctx, dbSQL); err != nil {
		return err
	}

	// Swap staged files into place.
	if stageUploads != "" {
		if err := swapDirContents(stageUploads, h.uploadsDir); err != nil {
			return err
		}
	}
	if stageAudio != "" {
		if err := swapDirContents(stageAudio, h.audioDir); err != nil {
			return err
		}
	}
	if stageVapid != "" && h.vapidPath != "" {
		// stageVapid is a directory; swap contents of the directory holding keys.
		if err := swapDirContents(stageVapid, filepath.Dir(h.vapidPath)); err != nil {
			return err
		}
	}

	return nil
}

func (h *BackupHandler) restoreDB(ctx context.Context, zf *zip.File) error {
	if h.dbURL == "" {
		return fmt.Errorf("db url empty")
	}
	if _, err := exec.LookPath("psql"); err != nil {
		return fmt.Errorf("psql not found in PATH")
	}

	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Write SQL to temp file for psql stdin.
	tmpSQL, err := os.CreateTemp("", "pulse-db-*.sql")
	if err != nil {
		return err
	}
	tmpPath := tmpSQL.Name()
	defer func() {
		_ = tmpSQL.Close()
		_ = os.Remove(tmpPath)
	}()
	if _, err := io.Copy(tmpSQL, rc); err != nil {
		return err
	}
	if err := tmpSQL.Close(); err != nil {
		return err
	}

	dropCmd := exec.CommandContext(ctx, "psql",
		"--dbname", h.dbURL,
		"-v", "ON_ERROR_STOP=1",
		"-c", "DROP SCHEMA public CASCADE; CREATE SCHEMA public;",
	)
	var dropErr bytesLimited
	dropCmd.Stderr = &dropErr
	if err := dropCmd.Run(); err != nil {
		return fmt.Errorf("psql drop schema failed: %w: %s", err, dropErr.String())
	}

	sqlFile, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	defer sqlFile.Close()

	restoreCmd := exec.CommandContext(ctx, "psql",
		"--dbname", h.dbURL,
		"-v", "ON_ERROR_STOP=1",
	)
	restoreCmd.Stdin = sqlFile
	var restoreErr bytesLimited
	restoreCmd.Stderr = &restoreErr
	if err := restoreCmd.Run(); err != nil {
		return fmt.Errorf("psql restore failed: %w: %s", err, restoreErr.String())
	}
	return nil
}

func addFileToZip(zw *zip.Writer, srcPath, zipPath string) error {
	st, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("expected file, got dir: %s", srcPath)
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := zw.Create(zipPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

func addDirToZip(zw *zip.Writer, dir, prefix string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		zipName := filepath.ToSlash(filepath.Join(prefix, rel))
		return addFileToZip(zw, path, zipName)
	})
}

func writeZipJSON(zw *zip.Writer, name string, v any) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func extractZipEntry(zf *zip.File, targetRoot, rel string) error {
	rel = filepath.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("invalid path in zip: %s", zf.Name)
	}
	dst := filepath.Join(targetRoot, rel)
	if zf.FileInfo().IsDir() {
		return os.MkdirAll(dst, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// swapDirContents replaces contents of target directory with contents of staged directory.
// This works even when target is a bind mount (where renaming the mountpoint would fail).
func swapDirContents(staged, target string) error {
	ts := time.Now().UTC().Format("20060102T150405Z")
	stageBase := filepath.Base(staged)
	backupBase := ".bak-" + ts
	backupDir := filepath.Join(target, backupBase)

	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	_ = os.RemoveAll(backupDir)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}

	// Move existing entries into backup (excluding staging dir and backup dir itself).
	for _, e := range entries {
		name := e.Name()
		if name == stageBase || name == backupBase {
			continue
		}
		src := filepath.Join(target, name)
		dst := filepath.Join(backupDir, name)
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}

	// Move staged entries into target root.
	stageEntries, err := os.ReadDir(staged)
	if err != nil {
		return err
	}
	for _, e := range stageEntries {
		name := e.Name()
		src := filepath.Join(staged, name)
		dst := filepath.Join(target, name)
		_ = os.RemoveAll(dst)
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}

	// Clean up staging and backups.
	_ = os.RemoveAll(staged)
	_ = os.RemoveAll(backupDir)
	return nil
}

// bytesLimited captures stderr with a size cap (to avoid huge logs).
type bytesLimited struct {
	buf []byte
}

func (b *bytesLimited) Write(p []byte) (int, error) {
	const capBytes = 32 * 1024
	if len(b.buf) >= capBytes {
		return len(p), nil
	}
	remain := capBytes - len(b.buf)
	if len(p) > remain {
		p = p[:remain]
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bytesLimited) String() string { return strings.TrimSpace(string(b.buf)) }

// Ensure upload uses correct content-type without manual parsing.
func init() {
	_ = mime.TypeByExtension(".zip")
}
