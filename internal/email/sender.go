package email

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"time"

	"github.com/messenger/internal/config"
)

type Sender struct {
	cfg *config.SMTPConfig
}

func NewSender(cfg *config.SMTPConfig) *Sender {
	return &Sender{cfg: cfg}
}

func (s *Sender) SendOTP(ctx context.Context, to, code string) error {
	if !isConfigured(s.cfg) {
		return fmt.Errorf("email: SMTP не настроен")
	}
	from := s.cfg.FromEmail
	if from == "" {
		from = s.cfg.Username
	}
	body := fmt.Sprintf("Ваш код: %s\n\nКод действителен 5 минут.", code)
	var buf bytes.Buffer
	buf.WriteString("From: " + s.cfg.FromName + " <" + from + ">\r\n")
	buf.WriteString("To: " + to + "\r\n")
	buf.WriteString("Subject: Код для входа\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	buf.WriteString(body)
	addr := s.cfg.Host + ":" + strconv.Itoa(s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(addr, auth, from, []string{to}, buf.Bytes()) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// SendTest отправляет тестовое письмо на to (код TEST-xxxx) для проверки SMTP.
func (s *Sender) SendTest(ctx context.Context, to string) error {
	code := fmt.Sprintf("TEST-%d", time.Now().Unix()%10000)
	return s.SendOTP(ctx, to, code)
}

func isConfigured(cfg *config.SMTPConfig) bool {
	if cfg == nil {
		return false
	}
	return cfg.Host != "" && cfg.Port > 0 && cfg.Username != "" && cfg.Password != ""
}
