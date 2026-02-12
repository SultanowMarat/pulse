// Микросервис загрузки и раздачи голосовых сообщений (как в Telegram).
package main

import (
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/messenger/internal/audioserver"
	"github.com/messenger/internal/logger"
)

func main() {
	logger.SetPrefix("audio")
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	maxMB := 25
	if v := os.Getenv("MAX_UPLOAD_SIZE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxMB = n
		}
	}
	maxSize := int64(maxMB) << 20
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8084"
	}
	logger.Infof("starting audio service: upload_dir=%s max_upload_mb=%d", uploadDir, maxMB)

	svc := audioserver.New(uploadDir, maxSize)

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); w.Write([]byte("ok")) })
	r.Post("/upload", svc.Upload)
	r.Get("/audio/{filename}", func(w http.ResponseWriter, r *http.Request) {
		svc.Serve(w, r, chi.URLParam(r, "filename"))
	})

	srv := &http.Server{Addr: addr, Handler: r, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}
	go func() {
		logger.Infof("audio service listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("audio: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("audio service shutting down")
	srv.Close()
	logger.Info("audio service stopped")
}
