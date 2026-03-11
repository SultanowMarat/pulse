// Ðœ8:Ñ€>A5Ñ€28A 02Ñ‚>Ñ€870Ñ†88: OTP ?> email, A5AA88 ÑƒAÑ‚Ñ€>9AÑ‚2.
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pulse/internal/config"
	"github.com/pulse/internal/handler"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/repository"
	"github.com/pulse/internal/service"
	"github.com/pulse/internal/startup"
	"github.com/pulse/internal/storage"
	"github.com/pulse/internal/storage/devstore"
)

func main() {
	logger.SetPrefix("auth")
	dev := flag.Bool("dev", false, "use in-memory store instead of Redis (no Redis required)")
	flag.Parse()

	logger.Info("starting auth service")
	cfg := config.Load()
	logger.Info("SMTP 4;O OTP Ñƒ?Ñ€02;O5Ñ‚AO 87 04<8=-?0=5;8 ?Ñ€8;>65=8O")

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		logger.Errorf("parse db config: %v", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = int32(cfg.DBMaxConnections())
	pool := startup.ConnectDBWithRetry(poolCfg, 60*time.Second, "auth: ")
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	permRepo := repository.NewPermissionRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	mailSettingsRepo := repository.NewMailSettingsRepository(pool)

	var store storage.SessionOTPStore
	if *dev {
		logger.Info("auth -dev: using DB for session_secret (A5AA88 A>Ñ…Ñ€0=OÑŽÑ‚AO ?>A;5 ?5Ñ€570?ÑƒA:0)")
		store = devstore.New(sessionRepo)
	} else {
		redisClient := startup.ConnectRedisWithRetry(cfg.Redis.URL, 60*time.Second, "auth: ")
		defer redisClient.Close()
		store = redisClient
	}
	otpSvc := service.NewOTPAuthService(userRepo, permRepo, sessionRepo, mailSettingsRepo, store)
	authH := handler.NewAuthHandler(otpSvc)

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSAllowedOrigins},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Session-Id", "X-Timestamp", "X-Signature"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Post("/api/auth/request-code", authH.RequestCode)
	r.Post("/api/auth/verify-code", authH.VerifyCode)
	r.With(middleware.InternalOnly).Post("/internal/validate", handler.ValidateSession(otpSvc))
	r.With(middleware.InternalOnly).Post("/internal/users/{id}/logout-all", authH.InternalLogoutUserSessions)

	r.Group(func(r chi.Router) {
		r.Use(middleware.SessionAuth(sessionRepo, store))
		r.Get("/api/auth/sessions", authH.GetSessions)
		r.Delete("/api/auth/session", authH.LogoutSession)
		r.Delete("/api/auth/sessions", authH.LogoutAllSessions)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	srv := &http.Server{Addr: addr, Handler: r, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second}
	var srvWg sync.WaitGroup
	srvWg.Add(1)
	go func() {
		defer srvWg.Done()
		logger.Infof("auth server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("auth server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down auth server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("auth server shutdown: %v", err)
	}
	srvWg.Wait()
	logger.Info("auth server stopped")
}
