package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pulse/internal/authkey"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/model"
	"github.com/pulse/internal/repository"
)

// phoneRe â€” <564Ñƒ=0Ñ€>4=Ñ‹9 Ñ„>Ñ€<0Ñ‚: + 8 8â€“15 Ñ†8Ñ„Ñ€ (E.164).
var phoneRe = regexp.MustCompile(`^\+\d{8,15}$`)

type UserHandler struct {
	userRepo *repository.UserRepository
	msgRepo  *repository.MessageRepository
	permRepo *repository.PermissionRepository
	authURL  string
	httpC    *http.Client
	kicker   UserKicker
}

type UserKicker interface {
	KickUser(userID string)
}

func NewUserHandler(userRepo *repository.UserRepository, msgRepo *repository.MessageRepository, permRepo *repository.PermissionRepository, authServiceURL string, httpClient *http.Client, kicker UserKicker) *UserHandler {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &UserHandler{userRepo: userRepo, msgRepo: msgRepo, permRepo: permRepo, authURL: strings.TrimRight(authServiceURL, "/"), httpC: httpClient, kicker: kicker}
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSONCached(w, r, http.StatusOK, user.ToPublic(), user.CreatedAt)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user.ToPublic())
}

// UserStatsResponse combines user profile with activity stats.
type UserStatsResponse struct {
	User           model.UserPublic `json:"user"`
	MessagesToday  int              `json:"messages_today"`
	MessagesWeek   int              `json:"messages_week"`
	AvgResponseSec float64          `json:"avg_response_sec"`
}

func (h *UserHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	stats, err := h.msgRepo.GetUserStats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	writeJSON(w, http.StatusOK, UserStatsResponse{
		User:           user.ToPublic(),
		MessagesToday:  stats.MessagesToday,
		MessagesWeek:   stats.MessagesWeek,
		AvgResponseSec: stats.AvgResponseSec,
	})
}

func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.ListAll(r.Context(), 500)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list users failed")
		return
	}
	currentUserID := middleware.GetUserID(r.Context())
	result := make([]model.UserPublic, 0, len(users))
	for _, u := range users {
		if u.ID != currentUserID {
			result = append(result, u.ToPublic())
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// GetEmployees 2>72Ñ€0Ñ‰05Ñ‚ 2A5Ñ… ?>;ÑŒ7>20Ñ‚5;59 (A?8A>: A>Ñ‚Ñ€Ñƒ4=8:>2). ">;ÑŒ:> 4;O 04<8=8AÑ‚Ñ€0Ñ‚>Ñ€0.
func (h *UserHandler) GetEmployees(w http.ResponseWriter, r *http.Request) {
	currentUserID := middleware.GetUserID(r.Context())
	perm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
	if err != nil || !perm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can list employees")
		return
	}
	users, err := h.userRepo.ListAll(r.Context(), 2000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list employees failed")
		return
	}
	result := make([]model.UserPublic, 0, len(users))
	for _, u := range users {
		result = append(result, u.ToPublic())
	}
	writeJSON(w, http.StatusOK, result)
}

type EmployeesPageResponse struct {
	Users  []EmployeePublic `json:"users"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type EmployeePublic struct {
	model.UserPublic
	Role string `json:"role"`
}

func roleFromPermissions(p model.UserPermissions) string {
	if p.Administrator {
		return "administrator"
	}
	return "member"
}

// GetEmployeesPage 2>72Ñ€0Ñ‰05Ñ‚ ?>;ÑŒ7>20Ñ‚5;59 ?>AÑ‚Ñ€0=8Ñ‡=>. ">;ÑŒ:> 4;O 04<8=8AÑ‚Ñ€0Ñ‚>Ñ€0.
// Query params:
// - q: search by username/email/phone
// - limit, offset
// - sort_key: username|email|phone|status|last_seen_at|role
// - sort_dir: asc|desc
func (h *UserHandler) GetEmployeesPage(w http.ResponseWriter, r *http.Request) {
	currentUserID := middleware.GetUserID(r.Context())
	perm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
	if err != nil || !perm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can list employees")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 50
	offset := 0
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	sortKey := strings.TrimSpace(r.URL.Query().Get("sort_key"))
	sortDir := strings.TrimSpace(r.URL.Query().Get("sort_dir"))

	res, err := h.userRepo.ListPage(r.Context(), q, limit, offset, sortKey, sortDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list employees failed")
		return
	}

	userIDs := make([]string, 0, len(res.Users))
	for _, u := range res.Users {
		userIDs = append(userIDs, u.ID)
	}
	permMap, err := h.permRepo.GetByUserIDs(r.Context(), userIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list employees failed")
		return
	}

	out := make([]EmployeePublic, 0, len(res.Users))
	for _, u := range res.Users {
		p, ok := permMap[u.ID]
		role := "member"
		if ok {
			role = roleFromPermissions(p)
		}
		out = append(out, EmployeePublic{
			UserPublic: u.ToPublic(),
			Role:       role,
		})
	}
	writeJSON(w, http.StatusOK, EmployeesPageResponse{
		Users:  out,
		Total:  res.Total,
		Limit:  limit,
		Offset: offset,
	})
}

// CreateUserRequest â€” A>740=85 ?>;ÑŒ7>20Ñ‚5;O 04<8=8AÑ‚Ñ€0Ñ‚>Ñ€>< (A>Ñ‚Ñ€Ñƒ4=8: 157 2Ñ…>40; ?Ñ€8 ?5Ñ€2>< 2Ñ…>45 ?> email AÑ‚0=5Ñ‚ 53> ?Ñ€>Ñ„8;ÑŒ).
type CreateUserRequest struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	Phone       string `json:"phone"`
	Position    string `json:"position"`
	AvatarURL   string `json:"avatar_url"`
	Permissions *struct {
		Administrator *bool `json:"administrator"`
		Member        *bool `json:"member"`
	} `json:"permissions"`
}

// CreateUser A>740Ñ‘Ñ‚ ?>;ÑŒ7>20Ñ‚5;O (04<8=). Email 8 8<O >1O70Ñ‚5;ÑŒ=Ñ‹. ÐŸÑ€8 ?5Ñ€2>< 2Ñ…>45 ?> MÑ‚>9 ?>Ñ‡Ñ‚5 ?>;ÑŒ7>20Ñ‚5;ÑŒ ?>;ÑƒÑ‡8Ñ‚ MÑ‚>Ñ‚ ?Ñ€>Ñ„8;ÑŒ.
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	currentUserID := middleware.GetUserID(r.Context())
	perm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
	if err != nil || !perm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can create users")
		return
	}
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	emailNorm := strings.TrimSpace(strings.ToLower(req.Email))
	username := strings.TrimSpace(req.Username)
	if emailNorm == "" || username == "" {
		writeError(w, http.StatusBadRequest, "email and username required")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "invalid email format")
		return
	}
	phone := strings.TrimSpace(req.Phone)
	if phone != "" && !phoneRe.MatchString(phone) {
		writeError(w, http.StatusBadRequest, "invalid phone: use international format (+ and 8â€“15 digits)")
		return
	}
	if exists, err := h.userRepo.ExistsByEmail(r.Context(), emailNorm, ""); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check email")
		return
	} else if exists {
		writeError(w, http.StatusConflict, "Ð”0==Ñ‹9 email Ñƒ65 8A?>;ÑŒ7Ñƒ5Ñ‚AO")
		return
	}
	if phone != "" {
		if exists, err := h.userRepo.ExistsByPhone(r.Context(), phone, ""); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check phone")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "Ð”0==Ñ‹9 =><5Ñ€ Ñ‚5;5Ñ„>=0 Ñƒ65 8A?>;ÑŒ7Ñƒ5Ñ‚AO")
			return
		}
	}
	u := &model.User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        emailNorm,
		Phone:        phone,
		Position:     strings.TrimSpace(req.Position),
		PasswordHash: "",
		AvatarURL:    strings.TrimSpace(req.AvatarURL),
		LastSeenAt:   time.Now().UTC(),
		IsOnline:     false,
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.userRepo.Create(r.Context(), u); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	permNew := &model.UserPermissions{UserID: u.ID, Member: true}
	if req.Permissions != nil {
		if req.Permissions.Administrator != nil {
			permNew.Administrator = *req.Permissions.Administrator
		}
		if req.Permissions.Member != nil {
			permNew.Member = *req.Permissions.Member
		}
	}
	if err := h.permRepo.Upsert(r.Context(), permNew); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set permissions")
		return
	}
	writeJSON(w, http.StatusCreated, u.ToPublic())
}

func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, []model.UserPublic{})
		return
	}

	users, err := h.userRepo.SearchByUsername(r.Context(), query, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	currentUserID := middleware.GetUserID(r.Context())
	result := make([]model.UserPublic, 0, len(users))
	for _, u := range users {
		if u.ID != currentUserID {
			result = append(result, u.ToPublic())
		}
	}
	writeJSON(w, http.StatusOK, result)
}

type UpdateProfileRequest struct {
	Username  string  `json:"username"`
	AvatarURL string  `json:"avatar_url"`
	Email     string  `json:"email"`
	Phone     string  `json:"phone"`
	Position  *string `json:"position"`
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	// Ð’0;840Ñ†8O email (5A;8 ?5Ñ€540=)
	reqEmail := strings.TrimSpace(req.Email)
	if reqEmail != "" {
		if _, err := mail.ParseAddress(reqEmail); err != nil {
			writeError(w, http.StatusBadRequest, "invalid email format")
			return
		}
	}

	// Ð’0;840Ñ†8O Ñ‚5;5Ñ„>=0 (5A;8 ?5Ñ€540=): AÑ‚Ñ€>3> +993XXXXXXXX
	reqPhone := strings.TrimSpace(req.Phone)
	if reqPhone != "" {
		if !phoneRe.MatchString(reqPhone) {
			writeError(w, http.StatusBadRequest, "invalid phone: use international format (+ and 8â€“15 digits)")
			return
		}
	}

	userID := middleware.GetUserID(r.Context())
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	username := user.Username
	if req.Username != "" {
		username = req.Username
	}
	avatarURL := user.AvatarURL
	if req.AvatarURL != "" {
		avatarURL = req.AvatarURL
	}
	email := user.Email
	if reqEmail != "" {
		email = strings.ToLower(reqEmail)
	}
	phone := user.Phone
	if reqPhone != "" {
		phone = reqPhone
	}
	position := user.Position
	if req.Position != nil {
		position = strings.TrimSpace(*req.Position)
	}

	// Uniqueness checks (exclude current user)
	if reqEmail != "" {
		if exists, err := h.userRepo.ExistsByEmail(r.Context(), email, userID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check email")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "Ð”0==Ñ‹9 email Ñƒ65 8A?>;ÑŒ7Ñƒ5Ñ‚AO")
			return
		}
	}
	if reqPhone != "" {
		if exists, err := h.userRepo.ExistsByPhone(r.Context(), phone, userID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check phone")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "Ð”0==Ñ‹9 =><5Ñ€ Ñ‚5;5Ñ„>=0 Ñƒ65 8A?>;ÑŒ7Ñƒ5Ñ‚AO")
			return
		}
	}

	if err := h.userRepo.UpdateProfile(r.Context(), userID, username, avatarURL, email, phone, position); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	user.Username = username
	user.AvatarURL = avatarURL
	user.Email = email
	user.Phone = phone
	user.Position = position
	writeJSON(w, http.StatusOK, user.ToPublic())
}

// UpdateUserProfile >1=>2;O5Ñ‚ ?Ñ€>Ñ„8;ÑŒ ?>;ÑŒ7>20Ñ‚5;O ?> id. !2>Ñ‘ â€” 2A5340, Ñ‡Ñƒ6>5 â€” Ñ‚>;ÑŒ:> 04<8=8AÑ‚Ñ€0Ñ‚>Ñ€.
func (h *UserHandler) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	currentUserID := middleware.GetUserID(r.Context())
	if id == "" {
		writeError(w, http.StatusBadRequest, "user id required")
		return
	}
	if id != currentUserID {
		myPerm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
		if err != nil || !myPerm.Administrator {
			writeError(w, http.StatusForbidden, "only administrator can edit other user profile")
			return
		}
	}
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	reqEmail := strings.TrimSpace(req.Email)
	if reqEmail != "" {
		if _, err := mail.ParseAddress(reqEmail); err != nil {
			writeError(w, http.StatusBadRequest, "invalid email format")
			return
		}
	}
	reqPhone := strings.TrimSpace(req.Phone)
	if reqPhone != "" {
		if !phoneRe.MatchString(reqPhone) {
			writeError(w, http.StatusBadRequest, "invalid phone: use international format (+ and 8â€“15 digits)")
			return
		}
	}
	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	username := user.Username
	if req.Username != "" {
		username = req.Username
	}
	avatarURL := user.AvatarURL
	if req.AvatarURL != "" {
		avatarURL = req.AvatarURL
	}
	email := user.Email
	if reqEmail != "" {
		email = strings.ToLower(reqEmail)
	}
	phone := user.Phone
	if reqPhone != "" {
		phone = reqPhone
	}
	position := user.Position
	if req.Position != nil {
		position = strings.TrimSpace(*req.Position)
	}

	// Uniqueness checks (exclude the edited user)
	if reqEmail != "" {
		if exists, err := h.userRepo.ExistsByEmail(r.Context(), email, id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check email")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "Ð”0==Ñ‹9 email Ñƒ65 8A?>;ÑŒ7Ñƒ5Ñ‚AO")
			return
		}
	}
	if reqPhone != "" {
		if exists, err := h.userRepo.ExistsByPhone(r.Context(), phone, id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check phone")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "Ð”0==Ñ‹9 =><5Ñ€ Ñ‚5;5Ñ„>=0 Ñƒ65 8A?>;ÑŒ7Ñƒ5Ñ‚AO")
			return
		}
	}

	if err := h.userRepo.UpdateProfile(r.Context(), id, username, avatarURL, email, phone, position); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	user.Username = username
	user.AvatarURL = avatarURL
	user.Email = email
	user.Phone = phone
	user.Position = position
	writeJSON(w, http.StatusOK, user.ToPublic())
}

func (h *UserHandler) GetFavorites(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ids, err := h.userRepo.GetFavoriteChatIDs(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get favorites")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"chat_ids": ids})
}

func (h *UserHandler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req struct {
		ChatID string `json:"chat_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID == "" {
		writeError(w, http.StatusBadRequest, "chat_id required")
		return
	}
	if err := h.userRepo.AddFavorite(r.Context(), userID, req.ChatID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add favorite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *UserHandler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	chatID := chi.URLParam(r, "chatId")
	if chatID == "" {
		writeError(w, http.StatusBadRequest, "chat_id required")
		return
	}
	if err := h.userRepo.RemoveFavorite(r.Context(), userID, chatID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove favorite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *UserHandler) GetUserPermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "user id required")
		return
	}
	if _, err := h.userRepo.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	perm, err := h.permRepo.GetByUserID(r.Context(), id)
	if err != nil {
		logger.Errorf("GetUserPermissions fallback for user=%s: %v", id, err)
		// Do not fail profile/chat loading because of transient permissions read errors.
		writeJSON(w, http.StatusOK, &model.UserPermissions{
			UserID:        id,
			Administrator: false,
			Member:        true,
		})
		return
	}
	writeJSON(w, http.StatusOK, perm)
}

type UpdatePermissionsRequest struct {
	Administrator *bool `json:"administrator"`
	Member        *bool `json:"member"`
}

func (h *UserHandler) UpdateUserPermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	currentUserID := middleware.GetUserID(r.Context())
	if id == "" {
		writeError(w, http.StatusBadRequest, "user id required")
		return
	}
	if _, err := h.userRepo.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	// Ðœ5=OÑ‚ÑŒ ?Ñ€020 4Ñ€Ñƒ3>3> ?>;ÑŒ7>20Ñ‚5;O <>65Ñ‚ Ñ‚>;ÑŒ:> 04<8=8AÑ‚Ñ€0Ñ‚>Ñ€
	if id != currentUserID {
		myPerm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
		if err != nil || !myPerm.Administrator {
			writeError(w, http.StatusForbidden, "only administrator can edit other user permissions")
			return
		}
	}
	var req UpdatePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	perm, err := h.permRepo.GetByUserID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get permissions")
		return
	}
	if req.Administrator != nil {
		perm.Administrator = *req.Administrator
	}
	if req.Member != nil {
		perm.Member = *req.Member
	}
	if err := h.permRepo.Upsert(r.Context(), perm); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save permissions")
		return
	}
	writeJSON(w, http.StatusOK, perm)
}

func (h *UserHandler) GenerateUserLoginKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	currentUserID := middleware.GetUserID(r.Context())
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "user id required")
		return
	}
	myPerm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
	if err != nil || !myPerm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can generate login key")
		return
	}
	if _, err := h.userRepo.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	token, err := authkey.Generate()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate login key")
		return
	}
	if err := h.userRepo.SetLoginKey(r.Context(), id, authkey.Hash(token), time.Now().UTC()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save login key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"login_key":     token,
		"max_attempts":  authkey.MaxAttempts,
		"generated_now": true,
	})
}

// SetUserDisabledRequest Ñ‚5;> 70?Ñ€>A0 2:;ÑŽÑ‡8Ñ‚ÑŒ/>Ñ‚:;ÑŽÑ‡8Ñ‚ÑŒ ?>;ÑŒ7>20Ñ‚5;O.
type SetUserDisabledRequest struct {
	Disabled bool `json:"disabled"`
}

// SetUserDisabled >Ñ‚:;ÑŽÑ‡05Ñ‚ 8;8 2:;ÑŽÑ‡05Ñ‚ ?>;ÑŒ7>20Ñ‚5;O (Ñ‚>;ÑŒ:> 04<8=8AÑ‚Ñ€0Ñ‚>Ñ€). ÐžÑ‚:;ÑŽÑ‡Ñ‘==Ñ‹9 =5 <>65Ñ‚ 2>9Ñ‚8.
func (h *UserHandler) SetUserDisabled(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	currentUserID := middleware.GetUserID(r.Context())
	myPerm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
	if err != nil || !myPerm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can disable or enable users")
		return
	}
	if id == currentUserID {
		writeError(w, http.StatusBadRequest, "=5;ÑŒ7O >Ñ‚:;ÑŽÑ‡8Ñ‚ÑŒ A0<>3> A51O")
		return
	}
	_, err = h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	var req SetUserDisabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := h.userRepo.SetDisabled(r.Context(), id, req.Disabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"disabled": req.Disabled})
}

// LogoutAllDevices revokes all sessions for a user and disconnects their active WebSocket connections.
// Only administrator can perform this action.
func (h *UserHandler) LogoutAllDevices(w http.ResponseWriter, r *http.Request) {
	currentUserID := middleware.GetUserID(r.Context())
	perm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
	if err != nil || !perm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can revoke sessions")
		return
	}
	userID := chi.URLParam(r, "id")
	if strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "user id required")
		return
	}
	if _, err := h.userRepo.GetByID(r.Context(), userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if h.authURL == "" {
		writeError(w, http.StatusInternalServerError, "auth service not configured")
		return
	}
	url := fmt.Sprintf("%s/internal/users/%s/logout-all", h.authURL, userID)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build request")
		return
	}
	if secret := strings.TrimSpace(os.Getenv("INTERNAL_VALIDATE_SECRET")); secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
	resp, err := h.httpC.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "auth service unavailable")
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "failed to revoke sessions")
		return
	}
	var out struct {
		Status  string `json:"status"`
		Revoked int64  `json:"revoked"`
	}
	_ = json.Unmarshal(body, &out)
	if h.kicker != nil {
		h.kicker.KickUser(userID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "revoked": out.Revoked})
}
