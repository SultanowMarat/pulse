package model

import "time"

// MailSettings хранит SMTP-настройки для отправки кодов и тестовых писем.
type MailSettings struct {
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	FromEmail string    `json:"from_email"`
	FromName  string    `json:"from_name"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// IsConfigured возвращает true, если заполнены обязательные SMTP-поля.
func (m MailSettings) IsConfigured() bool {
	return m.Host != "" && m.Port > 0 && m.Username != "" && m.Password != ""
}
