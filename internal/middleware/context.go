package middleware

import "context"

type contextKey string

const UserIDKey contextKey = "user_id"

// GetUserID возвращает user_id из контекста (устанавливается AuthServiceValidate или SessionAuth).
func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}
