package handler

import (
	"context"
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
	"github.com/pulse/internal/cache"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/middleware"
	"github.com/pulse/internal/model"
	"github.com/pulse/internal/repository"
)

// phoneRe ��‚¬â€ <564Ã‘Æ’=0Ã‘â‚¬>4=Ã‘â€¹9 Ã‘â€ž>Ã‘â‚¬<0Ã‘â€š: + 8 8��‚¬â€œ15 Ã‘â€ 8Ã‘â€žÃ‘â‚¬ (E.164).
var phoneRe = regexp.MustCompile(`^\+\d{8,15}$`)

type UserHandler struct {
	userRepo *repository.UserRepository
	msgRepo  *repository.MessageRepository
	permRepo *repository.PermissionRepository
	cache    *cache.UserCache
	authURL  string
	httpC    *http.Client
	kicker   UserKicker
}

type UserKicker interface {
	KickUser(userID string)
}

func NewUserHandler(userRepo *repository.UserRepository, msgRepo *repository.MessageRepository, permRepo *repository.PermissionRepository, cache *cache.UserCache, authServiceURL string, httpClient *http.Client, kicker UserKicker) *UserHandler {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &UserHandler{userRepo: userRepo, msgRepo: msgRepo, permRepo: permRepo, cache: cache, authURL: strings.TrimRight(authServiceURL, "/"), httpC: httpClient, kicker: kicker}
}

func (h *UserHandler) invalidateUserCaches(ctx context.Context, userIDs ...string) {
	if h.cache == nil {
		return
	}
	if err := h.cache.InvalidateListCaches(ctx); err != nil {
		logger.Errorf("User cache invalidate list caches: %v", err)
	}
	if len(userIDs) > 0 {
		if err := h.cache.InvalidateProfiles(ctx, userIDs...); err != nil {
			logger.Errorf("User cache invalidate profiles: %v", err)
		}
		if err := h.cache.InvalidatePermissions(ctx, userIDs...); err != nil {
			logger.Errorf("User cache invalidate permissions: %v", err)
		}
	}
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if h.cache != nil {
		var cached model.UserPublic
		hit, err := h.cache.Profile(r.Context(), userID, &cached)
		if err != nil {
			logger.Errorf("GetProfile cache read user=%s: %v", userID, err)
		} else if hit {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	pub := user.ToPublic()
	if h.cache != nil {
		if err := h.cache.SetProfile(r.Context(), userID, pub); err != nil {
			logger.Errorf("GetProfile cache write user=%s: %v", userID, err)
		}
	}
	writeJSONCached(w, r, http.StatusOK, pub, user.CreatedAt)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.cache != nil {
		var cached model.UserPublic
		hit, err := h.cache.Profile(r.Context(), id, &cached)
		if err != nil {
			logger.Errorf("GetUser cache read user=%s: %v", id, err)
		} else if hit {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}
	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	pub := user.ToPublic()
	if h.cache != nil {
		if err := h.cache.SetProfile(r.Context(), id, pub); err != nil {
			logger.Errorf("GetUser cache write user=%s: %v", id, err)
		}
	}
	writeJSON(w, http.StatusOK, pub)
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
	currentUserID := middleware.GetUserID(r.Context())
	if h.cache != nil {
		var cached []model.UserPublic
		hit, err := h.cache.UsersList(r.Context(), currentUserID, 500, &cached)
		if err != nil {
			logger.Errorf("GetUsers cache read user=%s: %v", currentUserID, err)
		} else if hit {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}
	users, err := h.userRepo.ListAll(r.Context(), 500)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list users failed")
		return
	}
	result := make([]model.UserPublic, 0, len(users))
	for _, u := range users {
		if u.ID != currentUserID {
			result = append(result, u.ToPublic())
		}
	}
	if h.cache != nil {
		if err := h.cache.SetUsersList(r.Context(), currentUserID, 500, result); err != nil {
			logger.Errorf("GetUsers cache write user=%s: %v", currentUserID, err)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// GetEmployees 2>72Ã‘â‚¬0Ã‘â€°05Ã‘â€š 2A5Ã‘â€¦ ?>;Ã‘Å’7>20Ã‘â€š5;59 (A?8A>: A>Ã‘â€šÃ‘â‚¬Ã‘Æ’4=8:>2). ">;Ã‘Å’:> 4;O 04<8=8AÃ‘â€šÃ‘â‚¬0Ã‘â€š>Ã‘â‚¬0.
func (h *UserHandler) GetEmployees(w http.ResponseWriter, r *http.Request) {
	currentUserID := middleware.GetUserID(r.Context())
	perm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
	if err != nil || !perm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can list employees")
		return
	}
	if h.cache != nil {
		var cached []model.UserPublic
		hit, err := h.cache.EmployeesList(r.Context(), 2000, &cached)
		if err != nil {
			logger.Errorf("GetEmployees cache read admin=%s: %v", currentUserID, err)
		} else if hit {
			writeJSON(w, http.StatusOK, cached)
			return
		}
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
	if h.cache != nil {
		if err := h.cache.SetEmployeesList(r.Context(), 2000, result); err != nil {
			logger.Errorf("GetEmployees cache write admin=%s: %v", currentUserID, err)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

type EmployeesPageResponse struct {
	Users  []EmployeePublic `json:"users"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type UsersPageResponse struct {
	Users  []model.UserPublic `json:"users"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
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

// GetEmployeesPage 2>72Ã‘â‚¬0Ã‘â€°05Ã‘â€š ?>;Ã‘Å’7>20Ã‘â€š5;59 ?>AÃ‘â€šÃ‘â‚¬0=8Ã‘â€¡=>. ">;Ã‘Å’:> 4;O 04<8=8AÃ‘â€šÃ‘â‚¬0Ã‘â€š>Ã‘â‚¬0.
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
	if h.cache != nil {
		var cached EmployeesPageResponse
		hit, err := h.cache.EmployeesPage(r.Context(), q, limit, offset, sortKey, sortDir, &cached)
		if err != nil {
			logger.Errorf("GetEmployeesPage cache read admin=%s: %v", currentUserID, err)
		} else if hit {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}

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
	resp := EmployeesPageResponse{
		Users:  out,
		Total:  res.Total,
		Limit:  limit,
		Offset: offset,
	}
	if h.cache != nil {
		if err := h.cache.SetEmployeesPage(r.Context(), q, limit, offset, sortKey, sortDir, resp); err != nil {
			logger.Errorf("GetEmployeesPage cache write admin=%s: %v", currentUserID, err)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetUsersPage returns users for sidebar in pages (for "All" tab lazy loading).
// Query params:
// - q: optional search by username/email/phone
// - limit, offset
func (h *UserHandler) GetUsersPage(w http.ResponseWriter, r *http.Request) {
	currentUserID := middleware.GetUserID(r.Context())
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

	res, err := h.userRepo.ListPage(r.Context(), q, limit, offset, "username", "asc")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list users failed")
		return
	}

	users := make([]model.UserPublic, 0, len(res.Users))
	for _, u := range res.Users {
		if u.ID == currentUserID {
			continue
		}
		users = append(users, u.ToPublic())
	}
	total := res.Total
	if total > 0 {
		total--
	}

	writeJSON(w, http.StatusOK, UsersPageResponse{
		Users:  users,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// CreateUserRequest ��‚¬â€ A>740=85 ?>;Ã‘Å’7>20Ã‘â€š5;O 04<8=8AÃ‘â€šÃ‘â‚¬0Ã‘â€š>Ã‘â‚¬>< (A>Ã‘â€šÃ‘â‚¬Ã‘Æ’4=8: 157 2Ã‘â€¦>40; ?Ã‘â‚¬8 ?5Ã‘â‚¬2>< 2Ã‘â€¦>45 ?> email AÃ‘â€š0=5Ã‘â€š 53> ?Ã‘â‚¬>Ã‘â€ž8;Ã‘Å’).
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

// CreateUser A>740Ã‘â€˜Ã‘â€š ?>;Ã‘Å’7>20Ã‘â€š5;O (04<8=). Email 8 8<O >1O70Ã‘â€š5;Ã‘Å’=Ã‘â€¹. ���Ã‘â‚¬8 ?5Ã‘â‚¬2>< 2Ã‘â€¦>45 ?> MÃ‘â€š>9 ?>Ã‘â€¡Ã‘â€š5 ?>;Ã‘Å’7>20Ã‘â€š5;Ã‘Å’ ?>;Ã‘Æ’Ã‘â€¡8Ã‘â€š MÃ‘â€š>Ã‘â€š ?Ã‘â‚¬>Ã‘â€ž8;Ã‘Å’.
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
	if username == "" {
		writeError(w, http.StatusBadRequest, "username required")
		return
	}
	if emailNorm != "" {
		if _, err := mail.ParseAddress(req.Email); err != nil {
			writeError(w, http.StatusBadRequest, "invalid email format")
			return
		}
	}
	phone := strings.TrimSpace(req.Phone)
	if phone != "" && !phoneRe.MatchString(phone) {
		writeError(w, http.StatusBadRequest, "invalid phone: use international format (+ and 8-15 digits)")
		return
	}
	if emailNorm != "" {
		if exists, err := h.userRepo.ExistsByEmail(r.Context(), emailNorm, ""); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check email")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "email already exists")
			return
		}
	}
	if phone != "" {
		if exists, err := h.userRepo.ExistsByPhone(r.Context(), phone, ""); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check phone")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "phone already exists")
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
	h.invalidateUserCaches(r.Context(), u.ID)
	writeJSON(w, http.StatusCreated, u.ToPublic())
}

func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, []model.UserPublic{})
		return
	}

	currentUserID := middleware.GetUserID(r.Context())
	if h.cache != nil {
		var cached []model.UserPublic
		hit, err := h.cache.UsersSearch(r.Context(), currentUserID, query, 20, &cached)
		if err != nil {
			logger.Errorf("SearchUsers cache read user=%s: %v", currentUserID, err)
		} else if hit {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}

	users, err := h.userRepo.SearchByUsername(r.Context(), query, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	result := make([]model.UserPublic, 0, len(users))
	for _, u := range users {
		if u.ID != currentUserID {
			result = append(result, u.ToPublic())
		}
	}
	if h.cache != nil {
		if err := h.cache.SetUsersSearch(r.Context(), currentUserID, query, 20, result); err != nil {
			logger.Errorf("SearchUsers cache write user=%s: %v", currentUserID, err)
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

	// ��€™0;840Ã‘â€ 8O email (5A;8 ?5Ã‘â‚¬540=)
	reqEmail := strings.TrimSpace(req.Email)
	if reqEmail != "" {
		if _, err := mail.ParseAddress(reqEmail); err != nil {
			writeError(w, http.StatusBadRequest, "invalid email format")
			return
		}
	}

	// ��€™0;840Ã‘â€ 8O Ã‘â€š5;5Ã‘â€ž>=0 (5A;8 ?5Ã‘â‚¬540=): AÃ‘â€šÃ‘â‚¬>3> +993XXXXXXXX
	reqPhone := strings.TrimSpace(req.Phone)
	if reqPhone != "" {
		if !phoneRe.MatchString(reqPhone) {
			writeError(w, http.StatusBadRequest, "invalid phone: use international format (+ and 8-15 digits)")
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
			writeError(w, http.StatusConflict, "email already exists")
			return
		}
	}
	if reqPhone != "" {
		if exists, err := h.userRepo.ExistsByPhone(r.Context(), phone, userID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check phone")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "phone already exists")
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
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, http.StatusOK, user.ToPublic())
}

// UpdateUserProfile >1=>2;O5Ã‘â€š ?Ã‘â‚¬>Ã‘â€ž8;Ã‘Å’ ?>;Ã‘Å’7>20Ã‘â€š5;O ?> id. !2>Ã‘â€˜ ��‚¬â€ 2A5340, Ã‘â€¡Ã‘Æ’6>5 ��‚¬â€ Ã‘â€š>;Ã‘Å’:> 04<8=8AÃ‘â€šÃ‘â‚¬0Ã‘â€š>Ã‘â‚¬.
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
			writeError(w, http.StatusBadRequest, "invalid phone: use international format (+ and 8-15 digits)")
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
			writeError(w, http.StatusConflict, "email already exists")
			return
		}
	}
	if reqPhone != "" {
		if exists, err := h.userRepo.ExistsByPhone(r.Context(), phone, id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check phone")
			return
		} else if exists {
			writeError(w, http.StatusConflict, "phone already exists")
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
	h.invalidateUserCaches(r.Context(), id)
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
	if h.cache != nil {
		var cached model.UserPermissions
		hit, err := h.cache.Permission(r.Context(), id, &cached)
		if err != nil {
			logger.Errorf("GetUserPermissions cache read user=%s: %v", id, err)
		} else if hit {
			writeJSON(w, http.StatusOK, &cached)
			return
		}
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
	if h.cache != nil {
		if err := h.cache.SetPermission(r.Context(), id, perm); err != nil {
			logger.Errorf("GetUserPermissions cache write user=%s: %v", id, err)
		}
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
	// ��“5=OÃ‘â€šÃ‘Å’ ?Ã‘â‚¬020 4Ã‘â‚¬Ã‘Æ’3>3> ?>;Ã‘Å’7>20Ã‘â€š5;O <>65Ã‘â€š Ã‘â€š>;Ã‘Å’:> 04<8=8AÃ‘â€šÃ‘â‚¬0Ã‘â€š>Ã‘â‚¬
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
	h.invalidateUserCaches(r.Context(), id)
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

// SetUserDisabledRequest Ã‘â€š5;> 70?Ã‘â‚¬>A0 2:;Ã‘Å½Ã‘â€¡8Ã‘â€šÃ‘Å’/>Ã‘â€š:;Ã‘Å½Ã‘â€¡8Ã‘â€šÃ‘Å’ ?>;Ã‘Å’7>20Ã‘â€š5;O.
type SetUserDisabledRequest struct {
	Disabled bool `json:"disabled"`
}

// SetUserDisabled >Ã‘â€š:;Ã‘Å½Ã‘â€¡05Ã‘â€š 8;8 2:;Ã‘Å½Ã‘â€¡05Ã‘â€š ?>;Ã‘Å’7>20Ã‘â€š5;O (Ã‘â€š>;Ã‘Å’:> 04<8=8AÃ‘â€šÃ‘â‚¬0Ã‘â€š>Ã‘â‚¬). ���Ã‘â€š:;Ã‘Å½Ã‘â€¡Ã‘â€˜==Ã‘â€¹9 =5 <>65Ã‘â€š 2>9Ã‘â€š8.
func (h *UserHandler) SetUserDisabled(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	currentUserID := middleware.GetUserID(r.Context())
	myPerm, err := h.permRepo.GetByUserID(r.Context(), currentUserID)
	if err != nil || !myPerm.Administrator {
		writeError(w, http.StatusForbidden, "only administrator can disable or enable users")
		return
	}
	var req SetUserDisabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// Allow saving own profile when disabled=false; forbid only self-disable.
	if req.Disabled && id == currentUserID {
		writeError(w, http.StatusBadRequest, "cannot disable yourself")
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
	if err := h.userRepo.SetDisabled(r.Context(), id, req.Disabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	h.invalidateUserCaches(r.Context(), id)
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
