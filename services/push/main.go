// Микросервис пуш-уведомлений (Web Push): подписки в Redis, отправка через VAPID.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/push"
)

const (
	redisKeyPrefix   = "push:subs:"
	maxSubsPerUser   = 10
	subscriptionTTL  = 30 * 24 * time.Hour
)

type Config struct {
	ServerAddr      string
	RedisURL        string
	VAPIDPublicKey  string
	VAPIDPrivateKey string
}

func loadConfig() *Config {
	c := &Config{
		ServerAddr:      getEnv("SERVER_ADDR", ":8082"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
	}
	return c
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type PushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type SubscribeRequest struct {
	UserID       string           `json:"user_id"`
	Subscription PushSubscription `json:"subscription"`
}

type NotifyRequest struct {
	UserID string            `json:"user_id"`
	Title  string            `json:"title"`
	Body   string            `json:"body"`
	Data   map[string]string `json:"data,omitempty"`
}

type Server struct {
	cfg   *Config
	redis *redis.Client
	vapid *webpush.Options
}

func main() {
	logger.SetPrefix("push")
	if len(os.Args) > 1 && (os.Args[1] == "-gen-vapid" || os.Args[1] == "--gen-vapid") {
		priv, pub, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			logger.Errorf("generate VAPID: %v", err)
			os.Exit(1)
		}
		logger.Infof("VAPID_PUBLIC_KEY=%s", pub)
		logger.Infof("VAPID_PRIVATE_KEY=%s", priv)
		return
	}
	logger.Info("starting push service")
	cfg := loadConfig()
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		keys, err := push.EnsureVAPIDKeys("")
		if err == nil {
			cfg.VAPIDPublicKey = keys.PublicKey
			cfg.VAPIDPrivateKey = keys.PrivateKey
		} else {
			logger.Infof("VAPID: не удалось загрузить/сгенерировать ключи: %v — push отключены", err)
		}
	}
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		logger.Info("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY not set — push-уведомления отключены (подписки сохраняются, отправка не выполняется)")
	}

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Errorf("redis url: %v", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		cancel()
		logger.Errorf("redis ping: %v", err)
		os.Exit(1)
	}
	cancel()
	defer rdb.Close()
	logger.Info("redis connected")

	var vapidOpts *webpush.Options
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		vapidOpts = &webpush.Options{
			Subscriber:      "messenger-push",
			VAPIDPublicKey:  cfg.VAPIDPublicKey,
			VAPIDPrivateKey: cfg.VAPIDPrivateKey,
			TTL:             30,
		}
	}
	s := &Server{cfg: cfg, redis: rdb, vapid: vapidOpts}

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); w.Write([]byte("ok")) })
	r.Get("/api/vapid-public", s.handleVAPIDPublic)
	r.Route("/api", func(r chi.Router) {
		r.Post("/subscribe", s.handleSubscribe)
		r.Delete("/subscribe", s.handleUnsubscribe)
		r.Post("/notify", s.handleNotify)
	})

	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Infof("push server listening on %s", cfg.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("push server: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown signal received")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("shutdown: %v", err)
	}
	logger.Info("push server stopped")
}

func (s *Server) handleVAPIDPublic(w http.ResponseWriter, r *http.Request) {
	if s.cfg.VAPIDPublicKey == "" {
		http.Error(w, "push not configured", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(s.cfg.VAPIDPublicKey))
}

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" || req.Subscription.Endpoint == "" || req.Subscription.Keys.P256dh == "" || req.Subscription.Keys.Auth == "" {
		http.Error(w, "user_id and subscription (endpoint, keys.p256dh, keys.auth) required", http.StatusBadRequest)
		return
	}
	raw, err := json.Marshal(req.Subscription)
	if err != nil {
		http.Error(w, "subscription encode", http.StatusInternalServerError)
		return
	}
	key := redisKeyPrefix + req.UserID
	ctx := r.Context()
	pipe := s.redis.Pipeline()
	pipe.RPush(ctx, key, string(raw))
	pipe.LTrim(ctx, key, -maxSubsPerUser, -1)
	pipe.Expire(ctx, key, subscriptionTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		logger.Errorf("subscribe redis: %v", err)
		http.Error(w, "failed to save subscription", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" || req.Endpoint == "" {
		http.Error(w, "user_id and endpoint required", http.StatusBadRequest)
		return
	}
	key := redisKeyPrefix + req.UserID
	ctx := r.Context()
	list, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		http.Error(w, "failed to get subscriptions", http.StatusInternalServerError)
		return
	}
	var kept []string
	for _, item := range list {
		var sub PushSubscription
		if json.Unmarshal([]byte(item), &sub) == nil && sub.Endpoint != req.Endpoint {
			kept = append(kept, item)
		}
	}
	if len(kept) == 0 {
		s.redis.Del(ctx, key)
	} else {
		s.redis.Del(ctx, key)
		for _, v := range kept {
			s.redis.RPush(ctx, key, v)
		}
		s.redis.Expire(ctx, key, subscriptionTTL)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	var req NotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}
	key := redisKeyPrefix + req.UserID
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	list, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		logger.Errorf("notify redis: %v", err)
		http.Error(w, "failed to get subscriptions", http.StatusInternalServerError)
		return
	}
	payload := map[string]interface{}{"title": req.Title, "body": req.Body, "data": req.Data}
	payloadBytes, _ := json.Marshal(payload)
	var subs []PushSubscription
	for _, item := range list {
		var sub PushSubscription
		if json.Unmarshal([]byte(item), &sub) == nil && sub.Endpoint != "" {
			subs = append(subs, sub)
		}
	}
	if s.vapid == nil {
		return
	}
	for i := range subs {
		sub := &subs[i]
		wpSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys:     webpush.Keys{P256dh: sub.Keys.P256dh, Auth: sub.Keys.Auth},
		}
		resp, err := webpush.SendNotificationWithContext(ctx, payloadBytes, wpSub, s.vapid)
		if err != nil {
			logger.Errorf("send %s: %v", sub.Endpoint[:min(50, len(sub.Endpoint))], err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 410 || resp.StatusCode == 404 {
			s.removeSubscription(ctx, req.UserID, sub.Endpoint)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeSubscription(ctx context.Context, userID, endpoint string) {
	key := redisKeyPrefix + userID
	list, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return
	}
	var kept []string
	for _, item := range list {
		var sub PushSubscription
		if json.Unmarshal([]byte(item), &sub) == nil && sub.Endpoint != endpoint {
			kept = append(kept, item)
		}
	}
	s.redis.Del(ctx, key)
	for _, v := range kept {
		s.redis.RPush(ctx, key, v)
	}
	if len(kept) > 0 {
		s.redis.Expire(ctx, key, subscriptionTTL)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
