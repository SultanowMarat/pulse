package model

import "time"

type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	Phone        string     `json:"phone"`
	Position     string     `json:"position"`
	PasswordHash string     `json:"-"`
	AvatarURL    string     `json:"avatar_url"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	IsOnline     bool       `json:"is_online"`
	CreatedAt    time.Time  `json:"created_at"`
	DisabledAt   *time.Time `json:"-"` // не null = пользователь отключён, не может войти
}

type UserPublic struct {
	ID         string     `json:"id"`
	Username   string     `json:"username"`
	Email      string     `json:"email"`
	Phone      string     `json:"phone"`
	Position   string     `json:"position"`
	AvatarURL  string     `json:"avatar_url"`
	IsOnline   bool       `json:"is_online"`
	LastSeenAt time.Time  `json:"last_seen_at"`
	DisabledAt *time.Time `json:"disabled_at,omitempty"` // не null = отключён администратором
}

func (u *User) ToPublic() UserPublic {
	return UserPublic{
		ID:         u.ID,
		Username:   u.Username,
		Email:      u.Email,
		Phone:      u.Phone,
		Position:   u.Position,
		AvatarURL:  u.AvatarURL,
		IsOnline:   u.IsOnline,
		LastSeenAt: u.LastSeenAt,
		DisabledAt: u.DisabledAt,
	}
}
