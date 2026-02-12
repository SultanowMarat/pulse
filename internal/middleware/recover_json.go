package middleware

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"

	"github.com/messenger/internal/logger"
)

// responseWriter wraps http.ResponseWriter to detect if the response was already written.
// Реализует http.Hijacker для поддержки WebSocket upgrade.
type responseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *responseWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.status = code
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

// Hijack делегирует к нижележащему ResponseWriter, если он реализует http.Hijacker (нужно для WebSocket).
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// RecoverJSON при панике в handler логирует её и отдаёт клиенту JSON 500 (если ответ ещё не отправлен).
func RecoverJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrap := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("panic recovered: %v", err)
				if !wrap.wrote {
					wrap.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
					wrap.ResponseWriter.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(wrap.ResponseWriter).Encode(map[string]string{"error": "internal server error"})
				}
			}
		}()
		next.ServeHTTP(wrap, r)
	})
}
