package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pulse/internal/authkey"
	"github.com/pulse/internal/email"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/model"
	"github.com/pulse/internal/repository"
	"github.com/pulse/internal/storage"
)

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrInvalidOTP        = errors.New("invalid or expired OTP")
	ErrInvalidEmail      = errors.New("invalid email format")
	ErrInvalidLoginKey   = errors.New("invalid or expired login key")
	ErrUserDisabled      = errors.New("user disabled")
	ErrUserNotInvited    = errors.New("user is not invited")
)

const requestTimestampSkew = 10 * time.Minute

func maskSessionID(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "***"
}

type OTPAuthService struct {
	userRepo         *repository.UserRepository
	permRepo         *repository.PermissionRepository
	sessionRepo      *repository.SessionRepository
	mailSettingsRepo *repository.MailSettingsRepository
	store            storage.SessionOTPStore
}

func NewOTPAuthService(
	userRepo *repository.UserRepository,
	permRepo *repository.PermissionRepository,
	sessionRepo *repository.SessionRepository,
	mailSettingsRepo *repository.MailSettingsRepository,
	store storage.SessionOTPStore,
) *OTPAuthService {
	return &OTPAuthService{
		userRepo: userRepo, permRepo: permRepo, sessionRepo: sessionRepo, mailSettingsRepo: mailSettingsRepo, store: store,
	}
}

type RequestCodeRequest struct {
	Email      string `json:"email"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// Ð’0;840Ñ†8O email: 4>?ÑƒAÑ‚8<Ñ‹9 Ñ„>Ñ€<0Ñ‚ (Ñƒ?Ñ€>Ñ‰Ñ‘==Ñ‹9, 157 ?>;=>3> RFC).
var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// onlyDigits >AÑ‚02;O5Ñ‚ 2 AÑ‚Ñ€>:5 Ñ‚>;ÑŒ:> Ñ†8Ñ„Ñ€Ñ‹ (4;O :>40 87 ?8AÑŒ<0 â€” Ñƒ18Ñ€05Ñ‚ ?Ñ€>15;Ñ‹ 8 =52848<Ñ‹5 A8<2>;Ñ‹ ?Ñ€8 2AÑ‚02:5).
func onlyDigits(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			b = append(b, s[i])
		}
	}
	return string(b)
}

// normalizeEmailForKey ?Ñ€82>48Ñ‚ email : >4=><Ñƒ 284Ñƒ 4;O :;ÑŽÑ‡0 Redis (;0Ñ‚8=8Ñ†0, =86=89 Ñ€538AÑ‚Ñ€).
// Ð—0<5=O5Ñ‚ :8Ñ€8;;8Ñ‡5A:85 1Ñƒ:2Ñ‹-42>9=8:8 =0 ;0Ñ‚8=A:85, Ñ‡Ñ‚>1Ñ‹ 2AÑ‚02:0 87 1ÑƒÑ„5Ñ€0 =5 ;><0;0 :;ÑŽÑ‡.
func normalizeEmailForKey(s string) string {
	const (
		cyrO = '\u043e' // >
		cyrA = '\u0430' // 0
		cyrE = '\u0435' // 5
		cyrP = '\u0440' // Ñ€
		cyrC = '\u0441' // A
		cyrX = '\u0445' // Ñ…
		cyrY = '\u0443' // Ñƒ
	)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.TrimSpace(strings.ToLower(s)) {
		switch r {
		case cyrO:
			b.WriteByte('o')
		case cyrA:
			b.WriteByte('a')
		case cyrE:
			b.WriteByte('e')
		case cyrP:
			b.WriteByte('p')
		case cyrC:
			b.WriteByte('c')
		case cyrX:
			b.WriteByte('x')
		case cyrY:
			b.WriteByte('y')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *OTPAuthService) RequestCode(ctx context.Context, req RequestCodeRequest) (*VerifyCodeResponse, error) {
	identifier := strings.TrimSpace(req.Email)
	if identifier == "" {
		return nil, fmt.Errorf("email обязателен")
	}
	if emailRegexp.MatchString(strings.ToLower(identifier)) {
		return s.requestCodeByEmail(ctx, req, identifier)
	}
	return s.requestCodeByLoginKey(ctx, req, identifier)
}

func (s *OTPAuthService) requestCodeByLoginKey(ctx context.Context, req RequestCodeRequest, key string) (*VerifyCodeResponse, error) {
	if req.DeviceID == "" {
		req.DeviceID = "login-key-" + uuid.New().String()
	}
	if strings.TrimSpace(req.DeviceName) == "" {
		req.DeviceName = "Web"
	}

	keyHash := authkey.Hash(strings.TrimSpace(key))
	user, _, err := s.userRepo.ConsumeLoginKeyAttempt(ctx, keyHash, authkey.MaxAttempts)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidLoginKey
	}
	if user.DisabledAt != nil {
		return nil, ErrUserDisabled
	}
	return s.issueSession(ctx, user, req.DeviceID, req.DeviceName, false)
}

func (s *OTPAuthService) requestCodeByEmail(ctx context.Context, req RequestCodeRequest, identifier string) (*VerifyCodeResponse, error) {
	emailNorm := strings.TrimSpace(strings.ToLower(identifier))
	if !emailRegexp.MatchString(emailNorm) {
		return nil, ErrInvalidEmail
	}
	smtpCfg, err := s.mailSettingsRepo.GetSMTPConfig(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByEmail(ctx, emailNorm)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		// Bootstrap: very first user enters without OTP and becomes administrator.
		newUser, createErr := s.buildUserByEmail(ctx, emailNorm)
		if createErr != nil {
			return nil, createErr
		}
		created, createIfEmptyErr := s.userRepo.CreateIfNoUsers(ctx, newUser)
		if createIfEmptyErr != nil {
			return nil, createIfEmptyErr
		}
		if created {
			if err := s.permRepo.Upsert(ctx, &model.UserPermissions{
				UserID:        newUser.ID,
				Administrator: true,
				Member:        true,
			}); err != nil {
				return nil, err
			}
			// Ð”;O ?5Ñ€2>3> ?>;ÑŒ7>20Ñ‚5;O:
			// - 5A;8 SMTP =5 =0AÑ‚Ñ€>5=, Ñ€07Ñ€5Ñˆ05< 2Ñ…>4 157 OTP;
			// - 5A;8 SMTP =0AÑ‚Ñ€>5=, >Ñ‚?Ñ€02;O5< :>4 :0: >1Ñ‹Ñ‡=><Ñƒ ?>;ÑŒ7>20Ñ‚5;ÑŽ.
			if smtpCfg == nil {
				if req.DeviceID == "" {
					req.DeviceID = "bootstrap-" + uuid.New().String()
				}
				if strings.TrimSpace(req.DeviceName) == "" {
					req.DeviceName = "Web"
				}
				sessionResp, issueErr := s.issueSession(ctx, newUser, req.DeviceID, req.DeviceName, true)
				if issueErr != nil {
					return nil, issueErr
				}
				return sessionResp, nil
			}
			user = newUser
		} else {
			return nil, ErrUserNotInvited
		}
	}
	if user.DisabledAt != nil {
		return nil, ErrUserDisabled
	}
	// Ð•A;8 SMTP =5 =0AÑ‚Ñ€>5= â€” 2Ñ…>4 157 :>40 (?> Ñ‚Ñ€51>20=8ÑŽ 04<8=8AÑ‚Ñ€0Ñ‚>Ñ€0).
	if smtpCfg == nil {
		if req.DeviceID == "" {
			req.DeviceID = "passwordless-" + uuid.New().String()
		}
		if strings.TrimSpace(req.DeviceName) == "" {
			req.DeviceName = "Web"
		}
		return s.issueSession(ctx, user, req.DeviceID, req.DeviceName, false)
	}
	keyEmail := normalizeEmailForKey(emailNorm)
	allowed, err := s.store.CheckRateLimit(ctx, keyEmail)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrRateLimitExceeded
	}
	// Ð•A;8 :>4 Ñƒ65 1Ñ‹; 70?Ñ€>Ñˆ5= =5402=> (>AÑ‚0;>AÑŒ > 4 <8= TTL), ?5Ñ€5>Ñ‚?Ñ€02;O5< Ñ‚>Ñ‚ 65 :>4 â€” =5 ?5Ñ€570?8AÑ‹205< 2 Redis.
	const minTTLToReuse = 240 * time.Second
	if existing, _ := s.store.GetOTP(ctx, keyEmail); existing != "" && len(existing) == 6 {
		if ttl, _ := s.store.GetOTPTTL(ctx, keyEmail); ttl >= minTTLToReuse {
			logger.Infof("request-code: ?5Ñ€5>Ñ‚?Ñ€02:0 Ñ‚>3> 65 :>40 4;O key=otp:%s (TTL %.0fs)", keyEmail, ttl.Seconds())
			return nil, email.NewSender(smtpCfg).SendOTP(ctx, emailNorm, existing)
		}
	}
	code := generateOTP(6)
	if err := s.store.SetOTP(ctx, keyEmail, code); err != nil {
		return nil, err
	}
	logger.Infof("request-code: :>4 A>Ñ…Ñ€0=Ñ‘= 4;O key=otp:%s", keyEmail)
	return nil, email.NewSender(smtpCfg).SendOTP(ctx, emailNorm, code)
}

type VerifyCodeRequest struct {
	Email      string `json:"email"`
	Code       string `json:"code"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"` // >?Ñ†8>=0;ÑŒ=>
}

type VerifyCodeResponse struct {
	SessionID     string `json:"session_id"`
	SessionSecret string `json:"session_secret"`
	IsNewUser     bool   `json:"is_new_user"`
}

func (s *OTPAuthService) VerifyCode(ctx context.Context, req VerifyCodeRequest) (*VerifyCodeResponse, error) {
	emailNorm := strings.TrimSpace(strings.ToLower(req.Email))
	keyEmail := normalizeEmailForKey(emailNorm)
	codeNorm := onlyDigits(strings.TrimSpace(req.Code))
	if emailNorm == "" || codeNorm == "" || req.DeviceID == "" {
		return nil, fmt.Errorf("email, code и device_id обязательны")
	}
	if len(codeNorm) != 6 {
		return nil, ErrInvalidOTP
	}
	storedCode, err := s.store.GetOTP(ctx, keyEmail)
	if err != nil {
		logger.Errorf("verify-code: Redis GetOTP error key=%q err=%v", keyEmail, err)
		return nil, ErrInvalidOTP
	}
	if storedCode == "" {
		logger.Infof("verify-code: :;ÑŽÑ‡ otp:%s ?ÑƒAÑ‚ 8;8 8AÑ‚Ñ‘: (70?Ñ€>A8Ñ‚5 :>4 70=>2>)", keyEmail)
		return nil, ErrInvalidOTP
	}
	// !Ñ€02=5=85 constant-time. Ðš>4 2 Redis Ñ…Ñ€0=8Ñ‚AO :0: 6 Ñ†8Ñ„Ñ€, 22>4 =>Ñ€<0;87>20= Ñ‡5Ñ€57 onlyDigits.
	if len(storedCode) != 6 || subtle.ConstantTimeCompare([]byte(storedCode), []byte(codeNorm)) != 1 {
		logger.Infof("verify-code: =5A>2?045=85 key=%s len(stored)=%d len(entered)=%d", keyEmail, len(storedCode), len(codeNorm))
		return nil, ErrInvalidOTP
	}
	// Ðš>4 25Ñ€=Ñ‹9 â€” Ñƒ40;O5< OTP (>4=>Ñ€07>2>5 8A?>;ÑŒ7>20=85).
	if err := s.store.DeleteOTP(ctx, keyEmail); err != nil {
		logger.Errorf("verify-code: DeleteOTP key=%s: %v", keyEmail, err)
	}

	user, err := s.userRepo.GetByEmail(ctx, emailNorm)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotInvited
		}
		return nil, err
	}
	if user.DisabledAt != nil {
		return nil, ErrUserDisabled
	}
	return s.issueSession(ctx, user, req.DeviceID, req.DeviceName, false)
}

func (s *OTPAuthService) buildUserByEmail(ctx context.Context, emailAddr string) (*model.User, error) {
	username := deriveUsername(emailAddr)
	for i := 0; i < 10; i++ {
		try := username
		if i > 0 {
			try = username + "_" + uuid.New().String()[:8]
		}
		if len(try) > 50 {
			try = try[:50]
		}
		_, err := s.userRepo.GetByUsername(ctx, try)
		if errors.Is(err, repository.ErrNotFound) {
			now := time.Now().UTC()
			return &model.User{
				ID: uuid.New().String(), Username: try, Email: emailAddr, Phone: "",
				PasswordHash: "", LastSeenAt: now, IsOnline: false, CreatedAt: now,
			}, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("не удалось сгенерировать username")
}

func (s *OTPAuthService) issueSession(ctx context.Context, user *model.User, deviceID, deviceName string, isNewUser bool) (*VerifyCodeResponse, error) {
	sessionID := uuid.New().String()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	secretB64 := base64.StdEncoding.EncodeToString(secret)
	h := sha256.Sum256(secret)
	secretHash := hex.EncodeToString(h[:])
	now := time.Now().UTC()
	session := &model.Session{
		ID:         sessionID,
		UserID:     user.ID,
		DeviceID:   deviceID,
		DeviceName: strings.TrimSpace(deviceName),
		SecretHash: secretHash,
		LastSeenAt: now,
		CreatedAt:  now,
	}
	// !=0Ñ‡0;0 upsert (>4=0 >?5Ñ€0Ñ†8O, =5Ñ‚ duplicate key). ÐŸÑ€8 >Ñˆ81:5 (=0?Ñ€8<5Ñ€ AÑ‚0Ñ€0O Ð‘Ð”) â€” fallback: delete + insert.
	if err := s.sessionRepo.UpsertByUserIDAndDeviceID(ctx, session); err != nil {
		logger.Errorf("issue-session: Upsert session failed, fallback to delete+insert: %v", err)
		if delErr := s.sessionRepo.DeleteByUserIDAndDeviceID(ctx, user.ID, deviceID); delErr != nil {
			logger.Errorf("issue-session: DeleteByUserIDAndDeviceID failed: %v", delErr)
			return nil, fmt.Errorf("create session: %w", err)
		}
		if createErr := s.sessionRepo.Create(ctx, session); createErr != nil {
			logger.Errorf("issue-session: Create session failed: %v", createErr)
			return nil, fmt.Errorf("create session: %w", createErr)
		}
	}
	if err := s.store.SetSessionSecret(ctx, sessionID, secretB64); err != nil {
		logger.Errorf("issue-session: SetSessionSecret failed: %v", err)
		if delErr := s.sessionRepo.Delete(ctx, sessionID); delErr != nil {
			logger.Errorf("issue-session: rollback Delete session: %v", delErr)
		}
		return nil, fmt.Errorf("save session secret: %w", err)
	}
	return &VerifyCodeResponse{SessionID: sessionID, SessionSecret: secretB64, IsNewUser: isNewUser}, nil
}

func deriveUsername(emailAddr string) string {
	at := strings.Index(emailAddr, "@")
	if at <= 0 {
		return "user_" + uuid.New().String()[:8]
	}
	local := strings.ReplaceAll(emailAddr[:at], ".", "_")
	if len(local) > 50 {
		local = local[:50]
	}
	if local == "" {
		return "user_" + uuid.New().String()[:8]
	}
	return local
}

func generateOTP(length int) string {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		b[i] = digits[n.Int64()]
	}
	return string(b)
}

func hashOTP(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}

func (s *OTPAuthService) ListSessions(ctx context.Context, userID string) ([]model.Session, error) {
	return s.sessionRepo.ListByUserID(ctx, userID)
}

func (s *OTPAuthService) LogoutSession(ctx context.Context, userID, sessionID string) (bool, error) {
	ok, err := s.sessionRepo.DeleteByUserIDAndSessionID(ctx, userID, sessionID)
	if err != nil {
		return false, err
	}
	if ok {
		if err := s.store.DeleteSessionSecret(ctx, sessionID); err != nil {
			logger.Errorf("LogoutSession: DeleteSessionSecret session_id=%s: %v", maskSessionID(sessionID), err)
		}
	}
	return ok, nil
}

func (s *OTPAuthService) LogoutAllSessions(ctx context.Context, userID string) (int64, error) {
	ids, err := s.sessionRepo.RevokeByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.store.DeleteSessionSecret(ctx, id); err != nil {
			logger.Errorf("LogoutAllSessions: DeleteSessionSecret session_id=%s: %v", maskSessionID(id), err)
		}
	}
	return int64(len(ids)), nil
}

// ValidateRequest ?Ñ€>25Ñ€O5Ñ‚ ?>4?8AÑŒ 70?Ñ€>A0 8 2>72Ñ€0Ñ‰05Ñ‚ user_id. Ð˜A?>;ÑŒ7Ñƒ5Ñ‚AO API Ñ‡5Ñ€57 POST /internal/validate.
// timestamp â€” Unix A5:Ñƒ=4Ñ‹; 4>?ÑƒAÑ‚8<>5 >Ñ‚:;>=5=85 �10 <8=ÑƒÑ‚.
func (s *OTPAuthService) ValidateRequest(ctx context.Context, sessionID, timestamp, signature, method, path, body string) (userID string, err error) {
	if sessionID == "" || timestamp == "" || signature == "" {
		logger.Errorf("validate: missing session_id/timestamp/signature")
		return "", ErrInvalidOTP
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return "", ErrInvalidOTP
	}
	t := time.Unix(ts, 0)
	if time.Since(t) > requestTimestampSkew || time.Until(t) > requestTimestampSkew {
		logger.Errorf("validate: timestamp out of window session_id=%s", maskSessionID(sessionID))
		return "", ErrInvalidOTP
	}
	secretB64, err := s.store.GetSessionSecret(ctx, sessionID)
	if err != nil || secretB64 == "" {
		logger.Errorf("validate: no session_secret in Redis session_id=%s", maskSessionID(sessionID))
		return "", ErrInvalidOTP
	}
	secret, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil || len(secret) != 32 {
		return "", ErrInvalidOTP
	}
	tryPath := func(p string) bool {
		pl := method + p + body + timestamp
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(pl))
		expected := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(signature), []byte(expected))
	}
	if tryPath(path) {
		// ?>4?8AÑŒ A>2?0;0
	} else if strings.HasPrefix(path, "/api/") && tryPath(path[4:]) {
		// :;85=Ñ‚ ?>4?8A0; path 157 ?Ñ€5Ñ„8:A0 /api (AÑ‚0Ñ€Ñ‹9 Ñ„Ñ€>=Ñ‚ 8;8 ?Ñ€>:A8)
	} else {
		logger.Errorf("validate: signature mismatch path=%q", path)
		return "", ErrInvalidOTP
	}
	sess, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil || sess == nil {
		logger.Errorf("validate: session not found in DB session_id=%s err=%v", maskSessionID(sessionID), err)
		return "", ErrInvalidOTP
	}
	user, err := s.userRepo.GetByID(ctx, sess.UserID)
	if err != nil || user == nil || user.DisabledAt != nil {
		if user != nil && user.DisabledAt != nil {
			logger.Infof("validate: user %s disabled", sess.UserID)
		}
		return "", ErrInvalidOTP
	}
	if err := s.sessionRepo.UpdateLastSeen(ctx, sessionID, time.Now().UTC()); err != nil {
		logger.Errorf("validate: UpdateLastSeen session_id=%s: %v", maskSessionID(sessionID), err)
	}
	return sess.UserID, nil
}
