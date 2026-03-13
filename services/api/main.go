package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/pulse/internal/broker"
	"github.com/pulse/internal/cache"
	"github.com/pulse/internal/config"
	"github.com/pulse/internal/handler"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/outbox"
	"github.com/pulse/internal/push"
	"github.com/pulse/internal/repository"
	"github.com/pulse/internal/runtime"
	"github.com/pulse/internal/startup"
	"github.com/pulse/internal/ws"
)

func main() {
	logger.SetPrefix("api")
	migrate := flag.Bool("migrate", false, "run database migrations")
	dev := flag.Bool("dev", false, "start with embedded PostgreSQL (no external DB required)")
	flag.Parse()

	logger.Info("starting API service")
	cfg := config.Load()

	var embeddedDB *embeddedpostgres.EmbeddedPostgres
	if *dev {
		var err error
		embeddedDB, err = startEmbeddedPostgres(cfg)
		if err != nil {
			logger.Errorf("embedded postgres: %v", err)
			os.Exit(1)
		}
		defer func() {
			logger.Info("stopping embedded postgres...")
			if err := embeddedDB.Stop(); err != nil {
				logger.Errorf("embedded postgres stop: %v", err)
			}
		}()
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		logger.Errorf("parse db config: %v", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = int32(cfg.DBMaxConnections())
	poolCfg.MinConns = 4

	pool := startup.ConnectDBWithRetry(poolCfg, 60*time.Second, "")
	defer pool.Close()

	runMigrations(pool)
	if *migrate && !*dev {
		return
	}

	resetCtx, resetCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, err := pool.Exec(resetCtx, "UPDATE users SET is_online = false"); err != nil {
		logger.Errorf("reset online status: %v", err)
	}
	resetCancel()
	logger.Info("database connected, migrations applied")

	var redisCache *redis.Client
	redisCtx, redisCancel := context.WithTimeout(context.Background(), 3*time.Second)
	redisOpts, redisErr := redis.ParseURL(cfg.Redis.URL)
	if redisErr != nil {
		logger.Errorf("redis cache disabled: parse url: %v", redisErr)
	} else {
		candidate := redis.NewClient(redisOpts)
		if err := candidate.Ping(redisCtx).Err(); err != nil {
			logger.Errorf("redis cache disabled: ping: %v", err)
			_ = candidate.Close()
		} else {
			redisCache = candidate
			logger.Info("redis cache enabled")
		}
	}
	redisCancel()
	if redisCache != nil {
		defer func() {
			if err := redisCache.Close(); err != nil {
				logger.Errorf("redis cache close: %v", err)
			}
		}()
	}
	chatCache := cache.NewChatCache(redisCache)
	userCache := cache.NewUserCache(redisCache)

	userRepo := repository.NewUserRepository(pool)
	permRepo := repository.NewPermissionRepository(pool)
	mailSettingsRepo := repository.NewMailSettingsRepository(pool)
	fileSettingsRepo := repository.NewFileSettingsRepository(pool)
	serviceSettingsRepo := repository.NewServiceSettingsRepository(pool, cfg)
	chatRepo := repository.NewChatRepository(pool)
	msgRepo := repository.NewMessageRepository(pool)
	reactRepo := repository.NewReactionRepository(pool)
	pinnedRepo := repository.NewPinnedRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	fileH := handler.NewFileHandler(cfg)
	pushBrokerClient := push.NewBrokerClient(outboxRepo)
	pushNotifier := ws.PushNotifier(nil)
	pushSubClient := handler.PushSubscriptionClient(nil)
	var relayCancel context.CancelFunc
	var relayWg sync.WaitGroup
	var streamPublisher *broker.StreamPublisher

	if cfg.PushServiceURL == "" {
		logger.Info("push notifier disabled: PUSH_SERVICE_URL is empty")
	} else {
		// Keep push alive even when broker is unavailable.
		pushHTTPClient := push.NewClient(cfg.PushServiceURL)
		pushNotifier = pushHTTPClient
		pushSubClient = pushHTTPClient

		redisCtx, redisCancel := context.WithTimeout(context.Background(), 5*time.Second)
		streamPublisher, err = broker.NewStreamPublisher(redisCtx, cfg.Redis.URL, broker.PushStreamName)
		redisCancel()
		if err != nil {
			logger.Errorf("outbox relay disabled (redis publisher init failed), using direct HTTP push client: %v", err)
		} else {
			pushNotifier = pushBrokerClient
			pushSubClient = pushBrokerClient
			host, hostErr := os.Hostname()
			if hostErr != nil || host == "" {
				host = "api"
			}
			relayCtx, cancel := context.WithCancel(context.Background())
			relayCancel = cancel
			relay := outbox.NewRelay(outboxRepo, streamPublisher, fmt.Sprintf("%s-%d", host, os.Getpid()))
			relayWg.Add(1)
			go func() {
				defer relayWg.Done()
				relay.Run(relayCtx)
			}()
			logger.Info("outbox relay started, push routed through broker")
		}
	}

	hubCtx, hubCancel := context.WithCancel(context.Background())
	hub := ws.NewHub(chatRepo, msgRepo, userRepo, reactRepo, pinnedRepo, cfg.MaxWSConnections, pushNotifier, fileH, chatCache)

	var hubWg sync.WaitGroup
	hubWg.Add(1)
	go func() {
		defer hubWg.Done()
		hub.Run(hubCtx)
	}()

	chatH := handler.NewChatHandler(chatRepo, userRepo, permRepo, msgRepo, hub, fileH, chatCache)
	msgH := handler.NewMessageHandler(msgRepo, chatRepo, reactRepo, pinnedRepo, chatCache)
	audioH := handler.NewAudioHandler(cfg)
	userH := handler.NewUserHandler(userRepo, msgRepo, permRepo, userCache, cfg.AuthServiceURL, nil, hub)
	adminH := handler.NewAdminHandler(permRepo, mailSettingsRepo, fileSettingsRepo, int(cfg.MaxUploadSize/(1024*1024)))
	backupH := handler.NewBackupHandler(permRepo, cfg.DatabaseURL(), cfg.UploadDir, os.Getenv("BACKUP_AUDIO_DIR"), os.Getenv("VAPID_KEYS_FILE"))
	wsH := handler.NewWSHandler(hub)
	configH := handler.NewConfigHandler(cfg, fileSettingsRepo, serviceSettingsRepo)
	// Initialize runtime settings from DB so changes apply without restarts.
	if s, err := serviceSettingsRepo.Get(context.Background()); err == nil && s != nil {
		runtime.SetServiceSettings(*s, s.UpdatedAt)
	}
	serviceSettingsH := handler.NewServiceSettingsHandler(permRepo, serviceSettingsRepo, hub)
	pushH := handler.NewPushHandler(pushSubClient)

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(middleware.RecoverJSON)
	// 5 A68<0Ñ‚ÑŒ WebSocket â€” 8=0Ñ‡5 ResponseWriter =5 Ñ€50;87Ñƒ5Ñ‚ http.Hijacker 8 upgrade 40Ñ‘Ñ‚ 500.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
				next.ServeHTTP(w, req)
				return
			}
			chimw.Compress(5)(next).ServeHTTP(w, req)
		})
	})
	r.Use(middleware.RequestLog)
	r.Use(middleware.SecureHeaders)
	r.Use(middleware.RateLimitAPI)
	r.Use(middleware.DynamicCORS(func() string { return runtime.AllowedOrigins() }))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); w.Write([]byte("ok")) })
	r.Get("/api/config/cache", configH.GetCacheConfig)
	r.Get("/api/config/push", configH.GetPushConfig)
	r.Get("/api/config/app", configH.GetAppConfig)
	r.Get("/api/config/links", configH.GetLinksConfig)
	r.Get("/api/config/file-settings", configH.GetFileSettings)
	r.Get("/api/files/{filename}", fileH.Serve)
	if audioH != nil {
		r.Get("/api/audio/{filename}", audioH.Serve)
	}

	if cfg.AuthServiceURL != "" {
		authProxy := authProxyHandler(cfg.AuthServiceURL)
		r.Post("/api/auth/request-code", authProxy)
		r.Post("/api/auth/verify-code", authProxy)
	}
	r.Post("/api/auth/register", authLegacyGone)
	r.Post("/api/auth/login", authLegacyGone)
	r.Post("/api/auth/refresh", authLegacyGone)

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthServiceValidate(cfg.AuthServiceURL, nil))
		r.Get("/api/users/me", userH.GetProfile)
		r.Put("/api/users/me", userH.UpdateProfile)
		r.Get("/api/users", userH.GetUsers)
		r.Get("/api/users/employees", userH.GetEmployees)
		r.Get("/api/users/employees/page", userH.GetEmployeesPage)
		r.Post("/api/users", userH.CreateUser)
		r.Get("/api/users/search", userH.SearchUsers)
		r.Get("/api/users/me/favorites", userH.GetFavorites)
		r.Post("/api/users/me/favorites", userH.AddFavorite)
		r.Delete("/api/users/me/favorites/{chatId}", userH.RemoveFavorite)
		r.Get("/api/users/{id}", userH.GetUser)
		r.Put("/api/users/{id}", userH.UpdateUserProfile)
		r.Get("/api/users/{id}/stats", userH.GetUserStats)
		r.Get("/api/users/{id}/permissions", userH.GetUserPermissions)
		r.Put("/api/users/{id}/permissions", userH.UpdateUserPermissions)
		r.Post("/api/users/{id}/login-key/generate", userH.GenerateUserLoginKey)
		r.Put("/api/users/{id}/disable", userH.SetUserDisabled)
		r.Post("/api/users/{id}/logout-all", userH.LogoutAllDevices)
		r.Get("/api/admin/mail-settings", adminH.GetMailSettings)
		r.Put("/api/admin/mail-settings", adminH.UpdateMailSettings)
		r.Post("/api/admin/mail-settings/test", adminH.SendTestMail)
		r.Get("/api/admin/file-settings", adminH.GetFileSettings)
		r.Put("/api/admin/file-settings", adminH.UpdateFileSettings)
		r.Get("/api/admin/service-settings", serviceSettingsH.Get)
		r.Put("/api/admin/service-settings", serviceSettingsH.Update)
		r.Get("/api/admin/backup", backupH.CreateBackup)
		r.Post("/api/admin/restore", backupH.RestoreBackup)
		r.Get("/api/chats", chatH.GetUserChats)
		r.Post("/api/chats/personal", chatH.CreatePersonalChat)
		r.Post("/api/chats/group", chatH.CreateGroupChat)
		r.Get("/api/chats/{id}", chatH.GetChat)
		r.Post("/api/chats/{id}/mute", chatH.SetMuted)
		r.Post("/api/chats/{id}/clear", chatH.ClearHistory)
		r.Put("/api/chats/{id}", chatH.UpdateChat)
		r.Post("/api/chats/{id}/members", chatH.AddMembers)
		r.Delete("/api/chats/{id}/members/{memberId}", chatH.RemoveMember)
		r.Post("/api/chats/{id}/leave", chatH.LeaveChat)
		r.Get("/api/chats/{chatId}/messages", msgH.GetMessages)
		r.Post("/api/chats/{chatId}/read", msgH.MarkAsRead)
		r.Get("/api/chats/{chatId}/pinned", msgH.GetPinnedMessages)
		r.Get("/api/messages/{messageId}/reactions", msgH.GetReactions)
		r.Get("/api/messages/search", msgH.SearchMessages)
		r.Post("/api/files/upload", fileH.Upload)
		if audioH != nil {
			r.Post("/api/audio/upload", audioH.Upload)
		} else {
			r.Post("/api/audio/upload", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "audio service not configured"})
			})
		}
		r.Post("/api/push/subscribe", pushH.Subscribe)
		r.Delete("/api/push/subscribe", pushH.Unsubscribe)
		r.Get("/ws", wsH.ServeWS)
	})

	webDist := "./web/dist"
	if info, err := os.Stat(webDist); err == nil && info.IsDir() {
		r.Get("/*", spaHandler(webDist))
	}

	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	var srvWg sync.WaitGroup
	errCh := make(chan error, 1)
	srvWg.Add(1)
	go func() {
		defer srvWg.Done()
		logger.Infof("server listening on %s", cfg.ServerAddr)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Errorf("server error: %v", err)
			os.Exit(1)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("server shutdown: %v", err)
	}
	logger.Info("server stopped accepting connections")
	hubCancel()
	hubWg.Wait()
	logger.Info("hub stopped")
	if relayCancel != nil {
		relayCancel()
		relayWg.Wait()
		logger.Info("outbox relay stopped")
	}
	if streamPublisher != nil {
		if err := streamPublisher.Close(); err != nil {
			logger.Errorf("outbox relay redis close: %v", err)
		}
	}
	srvWg.Wait()
	logger.Info("server goroutine exited")
}

func authLegacyGone(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusGone)
	const msg = `{"error":"Ð’Ñ…>4 ?> ?0Ñ€>;ÑŽ >Ñ‚:;ÑŽÑ‡Ñ‘=. Ð˜A?>;ÑŒ7Ñƒ5Ñ‚AO 2Ñ…>4 ?> :>4Ñƒ =0 email. Ðž1=>28Ñ‚5 AÑ‚Ñ€0=8Ñ†Ñƒ (Ctrl+F5) 8;8 ?5Ñ€5A>15Ñ€8Ñ‚5 Ñ„Ñ€>=Ñ‚: cd web && npm run build"}`
	w.Write([]byte(msg))
}

func authProxyHandler(authBaseURL string) http.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		targetURL := strings.TrimSuffix(authBaseURL, "/") + r.URL.Path
		proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, targetURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		if proxyReq.Header.Get("Content-Type") == "" {
			proxyReq.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(proxyReq)
		if err != nil {
			logger.Errorf("auth proxy: %v", err)
			http.Error(w, `{"error":"auth service unavailable"}`, http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func spaHandler(dir string) http.HandlerFunc {
	fs := http.Dir(dir)
	fileServer := http.FileServer(fs)
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if path == "" {
			path = "index.html"
		}
		if f, err := fs.Open(path); err != nil {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
		} else {
			f.Close()
			fileServer.ServeHTTP(w, r)
		}
	}
}

func runMigrations(pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	entries, err := os.ReadDir("migrations")
	if err != nil {
		logger.Errorf("read migrations dir: %v", err)
		os.Exit(1)
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		files = append(files, filepath.Join("migrations", name))
	}
	sort.Strings(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			logger.Errorf("read migration %s: %v", f, err)
			os.Exit(1)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			logger.Errorf("run migration %s: %v", f, err)
			os.Exit(1)
		}
	}
	logger.Info("migrations applied")
}

func startEmbeddedPostgres(cfg *config.Config) (*embeddedpostgres.EmbeddedPostgres, error) {
	const (
		port     = 5432
		user     = "pulse"
		password = "pulse_secret"
		database = "pulse"
	)

	dataDir := filepath.Join(".", ".pgdata")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create pgdata dir: %w", err)
	}

	db := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Port(port).
			Username(user).
			Password(password).
			Database(database).
			DataPath(dataDir).
			RuntimePath(filepath.Join(os.TempDir(), "embedded-pg-runtime")),
	)

	logger.Info("starting embedded PostgreSQL...")
	if err := db.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	cfg.Database.URL = fmt.Sprintf(
		"postgres://%s:%s@localhost:%d/%s?sslmode=disable",
		user, password, port, database,
	)
	logger.Infof("embedded PostgreSQL running on port %d", port)
	return db, nil
}
