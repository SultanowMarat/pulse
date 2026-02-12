package middleware

import (
	"net/http"
	"time"

	"github.com/messenger/internal/logger"
)

// RequestLog логирует каждый HTTP-запрос: method, path и время выполнения (асинхронно, не блокирует).
func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer logger.DeferLogDuration("http "+r.Method+" "+r.URL.Path, start)()
		next.ServeHTTP(w, r)
	})
}
