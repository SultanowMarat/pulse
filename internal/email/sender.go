package email

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"time"

	"github.com/pulse/internal/config"
)

type Sender struct {
	cfg *config.SMTPConfig
}

func NewSender(cfg *config.SMTPConfig) *Sender {
	return &Sender{cfg: cfg}
}

func (s *Sender) SendOTP(ctx context.Context, to, code string) error {
	if !isConfigured(s.cfg) {
		return fmt.Errorf("email: SMTP =5 =0AÑ‚Ñ€>5=")
	}
	from := s.cfg.FromEmail
	if from == "" {
		from = s.cfg.Username
	}
	body := fmt.Sprintf("Ð’0Ñˆ :>4: %s\n\nÐš>4 459AÑ‚28Ñ‚5;5= 5 <8=ÑƒÑ‚.", code)
	var buf bytes.Buffer
	buf.WriteString("From: " + s.cfg.FromName + " <" + from + ">\r\n")
	buf.WriteString("To: " + to + "\r\n")
	buf.WriteString("Subject: Ðš>4 4;O 2Ñ…>40\r\n")
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

// SendTest >Ñ‚?Ñ€02;O5Ñ‚ Ñ‚5AÑ‚>2>5 ?8AÑŒ<> =0 to (:>4 TEST-xxxx) 4;O ?Ñ€>25Ñ€:8 SMTP.
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
