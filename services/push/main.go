// Ðœ8:Ñ€>A5Ñ€28A ?ÑƒÑˆ-Ñƒ254><;5=89 (Web Push): ?>4?8A:8 2 Redis, >Ñ‚?Ñ€02:0 Ñ‡5Ñ€57 VAPID.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/pulse/internal/broker"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/push"
)

const (
	redisKeyPrefix   = "push:subs:"
	maxSubsPerUser   = 10
	subscriptionTTL  = 30 * 24 * time.Hour
	processedKeyPref = "push:processed:event:"
	processedTTL     = 7 * 24 * time.Hour
	brokerReadBatch  = 50
	brokerClaimIdle  = 30 * time.Second
	brokerClaimEvery = 10 * time.Second
)

type Config struct {
	ServerAddr      string
	RedisURL        string
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	ConsumerName    string
}

func loadConfig() *Config {
	c := &Config{
		ServerAddr:      getEnv("SERVER_ADDR", ":8082"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		ConsumerName:    getEnv("PUSH_CONSUMER_NAME", ""),
	}
	if c.ConsumerName == "" {
		host, err := os.Hostname()
		if err == nil && host != "" {
			c.ConsumerName = "push-" + host
		} else {
			c.ConsumerName = fmt.Sprintf("push-%d", os.Getpid())
		}
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

type NotifyRequest = broker.PushNotifyPayload

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
			logger.Infof("VAPID: =5 Ñƒ40;>AÑŒ 703Ñ€Ñƒ78Ñ‚ÑŒ/A35=5Ñ€8Ñ€>20Ñ‚ÑŒ :;ÑŽÑ‡8: %v â€” push >Ñ‚:;ÑŽÑ‡5=Ñ‹", err)
		}
	}
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		logger.Info("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY not set â€” push-Ñƒ254><;5=8O >Ñ‚:;ÑŽÑ‡5=Ñ‹ (?>4?8A:8 A>Ñ…Ñ€0=OÑŽÑ‚AO, >Ñ‚?Ñ€02:0 =5 2Ñ‹?>;=O5Ñ‚AO)")
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
			Subscriber:      "pulse-push",
			VAPIDPublicKey:  cfg.VAPIDPublicKey,
			VAPIDPrivateKey: cfg.VAPIDPrivateKey,
			TTL:             30,
		}
	}
	s := &Server{cfg: cfg, redis: rdb, vapid: vapidOpts}
	runCtx, runCancel := context.WithCancel(context.Background())

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

	var bgWG sync.WaitGroup
	bgWG.Add(1)
	go func() {
		defer bgWG.Done()
		s.runBrokerConsumer(runCtx)
	}()

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
	runCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("shutdown: %v", err)
	}
	bgWG.Wait()
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
	if err := s.saveSubscription(r.Context(), req.UserID, req.Subscription); err != nil {
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
	if err := s.deleteSubscription(r.Context(), req.UserID, req.Endpoint); err != nil {
		logger.Errorf("unsubscribe redis: %v", err)
		http.Error(w, "failed to unsubscribe", http.StatusInternalServerError)
		return
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
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if err := s.sendNotify(ctx, req); err != nil {
		logger.Errorf("notify: %v", err)
		http.Error(w, "failed to notify", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sendNotify(ctx context.Context, req NotifyRequest) error {
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		return fmt.Errorf("notify user_id required")
	}
	key := redisKeyPrefix + req.UserID
	list, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("notify redis lrange: %w", err)
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
		return nil
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
	return nil
}

func (s *Server) runBrokerConsumer(ctx context.Context) {
	if err := s.ensureBrokerGroup(ctx); err != nil {
		logger.Errorf("broker group init: %v", err)
		return
	}
	logger.Infof(
		"broker consumer started stream=%s group=%s consumer=%s",
		broker.PushStreamName,
		broker.PushConsumerGroup,
		s.cfg.ConsumerName,
	)

	nextClaimAt := time.Now()
	for {
		if ctx.Err() != nil {
			return
		}
		if time.Now().After(nextClaimAt) {
			s.claimPending(ctx)
			nextClaimAt = time.Now().Add(brokerClaimEvery)
		}

		streams, err := s.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    broker.PushConsumerGroup,
			Consumer: s.cfg.ConsumerName,
			Streams:  []string{broker.PushStreamName, ">"},
			Count:    brokerReadBatch,
			Block:    5 * time.Second,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if strings.Contains(err.Error(), "NOGROUP") {
				if initErr := s.ensureBrokerGroup(ctx); initErr != nil {
					logger.Errorf("broker group re-init: %v", initErr)
				}
				continue
			}
			logger.Errorf("broker read group: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		s.processStreamMessages(ctx, streams)
	}
}

func (s *Server) ensureBrokerGroup(ctx context.Context) error {
	err := s.redis.XGroupCreateMkStream(ctx, broker.PushStreamName, broker.PushConsumerGroup, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (s *Server) claimPending(ctx context.Context) {
	start := "0-0"
	for {
		messages, nextStart, err := s.redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   broker.PushStreamName,
			Group:    broker.PushConsumerGroup,
			Consumer: s.cfg.ConsumerName,
			MinIdle:  brokerClaimIdle,
			Start:    start,
			Count:    brokerReadBatch,
		}).Result()
		if err == redis.Nil {
			return
		}
		if err != nil {
			if strings.Contains(err.Error(), "NOGROUP") {
				if initErr := s.ensureBrokerGroup(ctx); initErr != nil {
					logger.Errorf("broker claim group init: %v", initErr)
				}
			} else {
				logger.Errorf("broker autoclaim: %v", err)
			}
			return
		}
		if len(messages) == 0 {
			return
		}
		s.processClaimedMessages(ctx, messages)
		if nextStart == "0-0" {
			return
		}
		start = nextStart
	}
}

func (s *Server) processStreamMessages(ctx context.Context, streams []redis.XStream) {
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			s.processOneMessage(ctx, msg)
		}
	}
}

func (s *Server) processClaimedMessages(ctx context.Context, messages []redis.XMessage) {
	for _, msg := range messages {
		s.processOneMessage(ctx, msg)
	}
}

func (s *Server) processOneMessage(ctx context.Context, msg redis.XMessage) {
	ack, err := s.consumeMessage(ctx, msg)
	if err != nil {
		logger.Errorf("broker consume id=%s: %v", msg.ID, err)
	}
	if !ack {
		return
	}
	if err := s.redis.XAck(ctx, broker.PushStreamName, broker.PushConsumerGroup, msg.ID).Err(); err != nil {
		logger.Errorf("broker ack id=%s: %v", msg.ID, err)
	}
}

func (s *Server) consumeMessage(ctx context.Context, msg redis.XMessage) (bool, error) {
	topic := asString(msg.Values["topic"])
	if topic == "" {
		return true, nil
	}
	eventID := asString(msg.Values["event_id"])
	if eventID != "" {
		done, err := s.redis.Exists(ctx, processedKeyPref+eventID).Result()
		if err != nil {
			return false, fmt.Errorf("broker check processed id=%s: %w", eventID, err)
		}
		if done > 0 {
			return true, nil
		}
	}
	payloadRaw := asString(msg.Values["payload"])
	switch topic {
	case broker.TopicPushNotify:
		if payloadRaw == "" {
			return true, nil
		}
		var req NotifyRequest
		if err := json.Unmarshal([]byte(payloadRaw), &req); err != nil {
			logger.Errorf("broker invalid payload id=%s topic=%s: %v", msg.ID, topic, err)
			return true, nil
		}
		req.UserID = strings.TrimSpace(req.UserID)
		if req.UserID == "" {
			logger.Errorf("broker invalid notify payload id=%s: empty user_id", msg.ID)
			return true, nil
		}
		notifyCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		err := s.sendNotify(notifyCtx, req)
		cancel()
		if err != nil {
			return false, err
		}
	case broker.TopicPushSubscribe:
		if payloadRaw == "" {
			return true, nil
		}
		var req broker.PushSubscribePayload
		if err := json.Unmarshal([]byte(payloadRaw), &req); err != nil {
			logger.Errorf("broker invalid payload id=%s topic=%s: %v", msg.ID, topic, err)
			return true, nil
		}
		req.UserID = strings.TrimSpace(req.UserID)
		req.Subscription.Endpoint = strings.TrimSpace(req.Subscription.Endpoint)
		if req.UserID == "" || req.Subscription.Endpoint == "" || req.Subscription.Keys.P256dh == "" || req.Subscription.Keys.Auth == "" {
			logger.Errorf("broker invalid subscribe payload id=%s", msg.ID)
			return true, nil
		}
		sub := PushSubscription{
			Endpoint: req.Subscription.Endpoint,
			Keys:     req.Subscription.Keys,
		}
		if err := s.saveSubscription(ctx, req.UserID, sub); err != nil {
			return false, err
		}
	case broker.TopicPushUnsubscribe:
		if payloadRaw == "" {
			return true, nil
		}
		var req broker.PushUnsubscribePayload
		if err := json.Unmarshal([]byte(payloadRaw), &req); err != nil {
			logger.Errorf("broker invalid payload id=%s topic=%s: %v", msg.ID, topic, err)
			return true, nil
		}
		req.UserID = strings.TrimSpace(req.UserID)
		req.Endpoint = strings.TrimSpace(req.Endpoint)
		if req.UserID == "" || req.Endpoint == "" {
			logger.Errorf("broker invalid unsubscribe payload id=%s", msg.ID)
			return true, nil
		}
		if err := s.deleteSubscription(ctx, req.UserID, req.Endpoint); err != nil {
			return false, err
		}
	default:
		return true, nil
	}

	s.markEventProcessed(ctx, eventID)
	return true, nil
}

func asString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprint(val)
	}
}

func (s *Server) removeSubscription(ctx context.Context, userID, endpoint string) {
	if err := s.deleteSubscription(ctx, userID, endpoint); err != nil {
		logger.Errorf("remove subscription user=%s: %v", userID, err)
	}
}

func (s *Server) saveSubscription(ctx context.Context, userID string, sub PushSubscription) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || sub.Endpoint == "" || sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		return fmt.Errorf("invalid subscription payload")
	}
	raw, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("subscription encode: %w", err)
	}

	key := redisKeyPrefix + userID
	list, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("subscription load: %w", err)
	}

	var kept []string
	for _, item := range list {
		var existing PushSubscription
		if json.Unmarshal([]byte(item), &existing) == nil && existing.Endpoint != "" && existing.Endpoint != sub.Endpoint {
			kept = append(kept, item)
		}
	}
	kept = append(kept, string(raw))
	if len(kept) > maxSubsPerUser {
		kept = kept[len(kept)-maxSubsPerUser:]
	}

	pipe := s.redis.TxPipeline()
	pipe.Del(ctx, key)
	for _, item := range kept {
		pipe.RPush(ctx, key, item)
	}
	pipe.Expire(ctx, key, subscriptionTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("subscription save: %w", err)
	}
	return nil
}

func (s *Server) deleteSubscription(ctx context.Context, userID, endpoint string) error {
	userID = strings.TrimSpace(userID)
	endpoint = strings.TrimSpace(endpoint)
	if userID == "" || endpoint == "" {
		return fmt.Errorf("invalid unsubscribe payload")
	}

	key := redisKeyPrefix + userID
	list, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("subscription load: %w", err)
	}
	var kept []string
	for _, item := range list {
		var sub PushSubscription
		if json.Unmarshal([]byte(item), &sub) == nil && sub.Endpoint != endpoint {
			kept = append(kept, item)
		}
	}

	pipe := s.redis.TxPipeline()
	pipe.Del(ctx, key)
	for _, item := range kept {
		pipe.RPush(ctx, key, item)
	}
	if len(kept) > 0 {
		pipe.Expire(ctx, key, subscriptionTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("subscription delete: %w", err)
	}
	return nil
}

func (s *Server) markEventProcessed(ctx context.Context, eventID string) {
	if eventID == "" {
		return
	}
	if err := s.redis.Set(ctx, processedKeyPref+eventID, "1", processedTTL).Err(); err != nil {
		logger.Errorf("broker set processed id=%s: %v", eventID, err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
