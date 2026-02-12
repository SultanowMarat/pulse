// Микросервис загрузки и раздачи файлов (upload + serve).
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/messenger/internal/fileserver"
	"github.com/messenger/internal/logger"
)

func main() {
	logger.SetPrefix("files")
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	maxMB := 20
	if v := os.Getenv("MAX_UPLOAD_SIZE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxMB = n
		}
	}
	maxSize := int64(maxMB) << 20
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8083"
	}
	logger.Infof("starting files service: upload_dir=%s max_upload_mb=%d", uploadDir, maxMB)

	svc := fileserver.New(uploadDir, maxSize)
	// Periodically pull current file size limit from API (admin setting) to apply immediately.
	// Default for docker-compose network: http://api:8080/api/config/file-settings
	cfgURL := strings.TrimSpace(os.Getenv("FILE_SETTINGS_URL"))
	if cfgURL == "" {
		cfgURL = "http://api:8080/api/config/file-settings"
	}
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		for {
			func() {
				resp, err := client.Get(cfgURL)
				if err != nil {
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return
				}
				body, _ := io.ReadAll(resp.Body)
				var data struct {
					MaxFileSizeMB int `json:"max_file_size_mb"`
				}
				if err := json.Unmarshal(body, &data); err != nil {
					return
				}
				if data.MaxFileSizeMB > 0 {
					svc.SetMaxUploadSizeBytes(int64(data.MaxFileSizeMB) << 20)
				}
			}()
			time.Sleep(3 * time.Second)
		}
	}()

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); w.Write([]byte("ok")) })
	r.Post("/upload", svc.Upload)
	r.Get("/files/{filename}", func(w http.ResponseWriter, r *http.Request) {
		svc.Serve(w, r, chi.URLParam(r, "filename"))
	})
	r.Delete("/files/{filename}", func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Delete(r.Context(), chi.URLParam(r, "filename")); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"failed to delete file"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{Addr: addr, Handler: r, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}
	go func() {
		logger.Infof("fileserver listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("fileserver: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("fileserver shutting down")
	srv.Close()
	logger.Info("fileserver stopped")
}
