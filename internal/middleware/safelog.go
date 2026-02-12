package middleware

import "strings"

// MaskSessionID маскирует session_id в логах (в prod не светить полный id).
func MaskSessionID(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "***"
}
