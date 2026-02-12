package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/messenger/internal/logger"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Errorf("writeJSON encode: %v", err)
	}
}

// writeJSONCached writes JSON with ETag/Last-Modified and returns 304 for conditional GET/HEAD.
func writeJSONCached(w http.ResponseWriter, r *http.Request, status int, data any, lastModified time.Time) {
	payload, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("writeJSONCached marshal: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	sum := sha256.Sum256(payload)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("ETag", etag)
	if !lastModified.IsZero() {
		w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	}

	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		if ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match")); ifNoneMatch != "" && etagInList(ifNoneMatch, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match")); ifNoneMatch == "" && !lastModified.IsZero() {
			if ims := strings.TrimSpace(r.Header.Get("If-Modified-Since")); ims != "" {
				if t, parseErr := time.Parse(http.TimeFormat, ims); parseErr == nil {
					if !lastModified.After(t.Add(time.Second)) {
						w.WriteHeader(http.StatusNotModified)
						return
					}
				}
			}
		}
	}

	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := w.Write(payload); err != nil {
		logger.Errorf("writeJSONCached write: %v", err)
		return
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		logger.Errorf("writeJSONCached newline: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func etagInList(raw, target string) bool {
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "*" || p == target || strings.TrimPrefix(p, "W/") == target {
			return true
		}
	}
	return false
}
