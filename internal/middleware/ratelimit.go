package middleware

import (
	"net/http"
	"sync"
	"time"
)

const (
	rateLimitWindow = time.Minute
	rateLimitMaxIP  = 200
	rateLimitMaxUser = 100
)

type rateLimiter struct {
	mu     sync.Mutex
	times  map[string][]time.Time
	max    int
	window time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{times: make(map[string][]time.Time), max: max, window: window}
}

func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-r.window)
	slice := r.times[key]
	i := 0
	for _, t := range slice {
		if t.After(cutoff) {
			slice[i] = t
			i++
		}
	}
	slice = slice[:i]
	if len(slice) >= r.max {
		return false
	}
	r.times[key] = append(slice, now)
	return true
}

var (
	apiRateByIP   = newRateLimiter(rateLimitMaxIP, rateLimitWindow)
	apiRateByUser = newRateLimiter(rateLimitMaxUser, rateLimitWindow)
)

// RateLimitAPI ограничивает запросы к /api/* по IP и по user_id (если есть в контексте). 429 при превышении.
func RateLimitAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if x := r.Header.Get("X-Real-Ip"); x != "" {
			ip = x
		} else if x := r.Header.Get("X-Forwarded-For"); x != "" {
			ip = x
		}
		if !apiRateByIP.allow(ip) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		if userID := GetUserID(r.Context()); userID != "" {
			if !apiRateByUser.allow("u:"+userID) {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
