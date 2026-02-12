package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// InternalOnly разрешает запрос только с приватных IP или при заголовке X-Internal-Secret == INTERNAL_VALIDATE_SECRET.
// В prod auth не экспонируется наружу; вызовы только от api в той же сети (private IP).
func InternalOnly(next http.Handler) http.Handler {
	secret := strings.TrimSpace(os.Getenv("INTERNAL_VALIDATE_SECRET"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret != "" && r.Header.Get("X-Internal-Secret") == secret {
			next.ServeHTTP(w, r)
			return
		}
		ipStr := r.Header.Get("X-Real-Ip")
		if ipStr == "" {
			ipStr = r.Header.Get("X-Forwarded-For")
			if idx := strings.Index(ipStr, ","); idx > 0 {
				ipStr = strings.TrimSpace(ipStr[:idx])
			}
		}
		if ipStr == "" {
			ipStr, _, _ = net.SplitHostPort(r.RemoteAddr)
			if ipStr == "" {
				ipStr = r.RemoteAddr
			}
		}
		if ipStr != "" && isPrivateIP(ipStr) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

func isPrivateIP(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}
