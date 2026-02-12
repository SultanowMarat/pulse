package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/repository"
	"github.com/messenger/internal/storage"
)

const TimestampSkew = 30 * time.Second

func SessionAuth(sessionRepo *repository.SessionRepository, store storage.SessionOTPStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := r.Header.Get("X-Session-Id")
			if sessionID == "" {
				sessionID = r.URL.Query().Get("session_id")
			}
			timestampStr := r.Header.Get("X-Timestamp")
			if timestampStr == "" {
				timestampStr = r.URL.Query().Get("timestamp")
			}
			signature := r.Header.Get("X-Signature")
			if signature == "" {
				signature = r.URL.Query().Get("signature")
			}
			if sessionID == "" || timestampStr == "" || signature == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ts, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			reqTime := time.Unix(ts, 0)
			if time.Since(reqTime) > TimestampSkew || time.Until(reqTime) > TimestampSkew {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			var body []byte
			if r.Body != nil {
				body, err = io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
			// session_secret хранится в store (Redis или in-memory в -dev).
			secretB64, err := store.GetSessionSecret(r.Context(), sessionID)
			if err != nil || secretB64 == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			secret, err := base64.StdEncoding.DecodeString(secretB64)
			if err != nil || len(secret) != 32 {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			payload := r.Method + r.URL.Path + string(body) + timestampStr
			mac := hmac.New(sha256.New, secret)
			mac.Write([]byte(payload))
			expected := hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(signature), []byte(expected)) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			session, err := sessionRepo.GetByID(r.Context(), sessionID)
			if err != nil || session == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if err := sessionRepo.UpdateLastSeen(r.Context(), sessionID, time.Now().UTC()); err != nil {
				logger.Errorf("session middleware UpdateLastSeen session_id=%s: %v", MaskSessionID(sessionID), err)
			}
			ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)
			ctx = context.WithValue(ctx, SessionIDKey, sessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

var SessionIDKey contextKey = "session_id"

func GetSessionID(ctx context.Context) string {
	v, _ := ctx.Value(SessionIDKey).(string)
	return v
}
