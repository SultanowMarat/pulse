package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// AuthServiceValidate вызывает микросервис авторизации для проверки сессии (X-Session-Id, X-Timestamp, X-Signature).
func AuthServiceValidate(authServiceURL string, client *http.Client) func(http.Handler) http.Handler {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := r.Header.Get("X-Session-Id")
			if sessionID == "" {
				sessionID = r.URL.Query().Get("session_id")
			}
			timestamp := r.Header.Get("X-Timestamp")
			if timestamp == "" {
				timestamp = r.URL.Query().Get("timestamp")
			}
			signature := r.Header.Get("X-Signature")
			if signature == "" {
				signature = r.URL.Query().Get("signature")
			}
			if sessionID == "" || timestamp == "" || signature == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			// Путь для подписи: только pathname (r.URL.Path), без query. Должен совпадать с pathForSignature на фронте (API + pathname).
			path := r.URL.Path
			contentType := r.Header.Get("Content-Type")
			bodyForSignature := ""
			// Для multipart не читаем тело вообще: оно может быть большим (upload/backup restore).
			// Клиент подписывает такие запросы с пустым телом.
			if !strings.HasPrefix(contentType, "multipart/form-data") {
				var body []byte
				if r.Body != nil {
					var err error
					body, err = io.ReadAll(r.Body)
					if err != nil {
						http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
						return
					}
					r.Body = io.NopCloser(bytes.NewReader(body))
				}
				bodyForSignature = string(body)
			}
			reqBody := map[string]string{
				"session_id": sessionID,
				"timestamp":  timestamp,
				"signature":  signature,
				"method":     r.Method,
				"path":       path,
				"body":       bodyForSignature,
			}
			jsonBody, _ := json.Marshal(reqBody)
			req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, authServiceURL+"/internal/validate", bytes.NewReader(jsonBody))
			if err != nil {
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			var result struct {
				UserID string `json:"user_id"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.UserID == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, result.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
