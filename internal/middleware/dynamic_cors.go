package middleware

import (
	"net/http"
	"strings"
)

// DynamicCORS applies CORS rules per-request based on a getter.
// This allows changing allowed origins at runtime from the admin UI.
func DynamicCORS(getAllowedOrigins func() string) func(http.Handler) http.Handler {
	allowedMethods := "GET, POST, PUT, DELETE, OPTIONS"
	allowedHeaders := "Accept, Authorization, Content-Type, X-Session-Id, X-Timestamp, X-Signature"

	originAllowed := func(origin, allowedRaw string) bool {
		allowedRaw = strings.TrimSpace(allowedRaw)
		if allowedRaw == "" || allowedRaw == "*" {
			return true
		}
		for _, o := range strings.Split(allowedRaw, ",") {
			if strings.TrimSpace(o) == origin {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			allowedRaw := ""
			if getAllowedOrigins != nil {
				allowedRaw = getAllowedOrigins()
			}

			if origin != "" && originAllowed(origin, allowedRaw) {
				// With credentials, we must echo the Origin (not "*").
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			} else if origin == "" && (strings.TrimSpace(allowedRaw) == "" || strings.TrimSpace(allowedRaw) == "*") {
				// Non-browser clients.
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

