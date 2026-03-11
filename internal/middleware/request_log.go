package middleware

import (
	"net/http"
	"time"

	"github.com/pulse/internal/logger"
)

// RequestLog ;>38Ñ€Ñƒ5Ñ‚ :064Ñ‹9 HTTP-70?Ñ€>A: method, path 8 2Ñ€5<O 2Ñ‹?>;=5=8O (0A8=Ñ…Ñ€>==>, =5 1;>:8Ñ€Ñƒ5Ñ‚).
func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer logger.DeferLogDuration("http "+r.Method+" "+r.URL.Path, start)()
		next.ServeHTTP(w, r)
	})
}
