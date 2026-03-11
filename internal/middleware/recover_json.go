package middleware

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"

	"github.com/pulse/internal/logger"
)

// responseWriter wraps http.ResponseWriter to detect if the response was already written.
//  50;87Ñƒ5Ñ‚ http.Hijacker 4;O ?>445Ñ€6:8 WebSocket upgrade.
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

// Hijack 45;538Ñ€Ñƒ5Ñ‚ : =865;560Ñ‰5<Ñƒ ResponseWriter, 5A;8 >= Ñ€50;87Ñƒ5Ñ‚ http.Hijacker (=Ñƒ6=> 4;O WebSocket).
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// RecoverJSON ?Ñ€8 ?0=8:5 2 handler ;>38Ñ€Ñƒ5Ñ‚ 5Ñ‘ 8 >Ñ‚40Ñ‘Ñ‚ :;85=Ñ‚Ñƒ JSON 500 (5A;8 >Ñ‚25Ñ‚ 5Ñ‰Ñ‘ =5 >Ñ‚?Ñ€02;5=).
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
