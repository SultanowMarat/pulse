package model

import "time"

// UserPermissions — права пользователя в команде (чаты и участники).
type UserPermissions struct {
	UserID                 string    `json:"user_id"`
	Administrator          bool      `json:"administrator"`
	Member                 bool      `json:"member"`
	UpdatedAt              time.Time `json:"updated_at"`
}
