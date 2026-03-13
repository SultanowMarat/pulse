package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/model"
)

var ErrNotFound = errors.New("not found")

// userCols â€” A?8A>: :>;>=>: 4;O SELECT, 2:;ÑŽÑ‡0O phone/position 8 disabled_at.
const userCols = `id, username, COALESCE(email,''), COALESCE(phone,''), COALESCE(position,''), password_hash, avatar_url, last_seen_at, is_online, created_at, disabled_at`

// userColsAliased â€” Ñ‚5 65 :>;>=:8 A ?Ñ€5Ñ„8:A>< u. 4;O 70?Ñ€>A>2 A JOIN (ListPage). COALESCE 4>;65= ?>;ÑƒÑ‡0Ñ‚ÑŒ u.column.
const userColsAliased = `u.id, u.username, COALESCE(u.email,''), COALESCE(u.phone,''), COALESCE(u.position,''), u.password_hash, u.avatar_url, u.last_seen_at, u.is_online, u.created_at, u.disabled_at`

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

type UserPageResult struct {
	Users []model.User
	Total int
}

// scanUser A:0=8Ñ€Ñƒ5Ñ‚ AÑ‚Ñ€>:Ñƒ 2 model.User (?>Ñ€O4>: A>>Ñ‚25Ñ‚AÑ‚2Ñƒ5Ñ‚ userCols).
func scanUser(s interface{ Scan(dest ...any) error }, u *model.User) error {
	return s.Scan(&u.ID, &u.Username, &u.Email, &u.Phone, &u.Position, &u.PasswordHash, &u.AvatarURL, &u.LastSeenAt, &u.IsOnline, &u.CreatedAt, &u.DisabledAt)
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	defer logger.DeferLogDuration("user.Create", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, username, email, phone, position, password_hash, avatar_url, last_seen_at, is_online, created_at, disabled_at)
		 VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10, $11)`,
		u.ID, u.Username, u.Email, u.Phone, u.Position, u.PasswordHash, u.AvatarURL, u.LastSeenAt, u.IsOnline, u.CreatedAt, u.DisabledAt,
	)
	if err != nil {
		return fmt.Errorf("userRepo.Create: %w", err)
	}
	return nil
}

// CreateIfNoUsers creates user only when users table is empty.
// Returns true when user was created, false when at least one user already exists.
func (r *UserRepository) CreateIfNoUsers(ctx context.Context, u *model.User) (bool, error) {
	defer logger.DeferLogDuration("user.CreateIfNoUsers", time.Now())()
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, username, email, phone, position, password_hash, avatar_url, last_seen_at, is_online, created_at, disabled_at)
		 SELECT $1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10, $11
		 WHERE NOT EXISTS (SELECT 1 FROM users LIMIT 1)`,
		u.ID, u.Username, u.Email, u.Phone, u.Position, u.PasswordHash, u.AvatarURL, u.LastSeenAt, u.IsOnline, u.CreatedAt, u.DisabledAt,
	)
	if err != nil {
		return false, fmt.Errorf("userRepo.CreateIfNoUsers: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	defer logger.DeferLogDuration("user.GetByID", time.Now())()
	u := &model.User{}
	row := r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id)
	if err := scanUser(row, u); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("userRepo.GetByID: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	defer logger.DeferLogDuration("user.GetByUsername", time.Now())()
	u := &model.User{}
	row := r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE username = $1`, username)
	if err := scanUser(row, u); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("userRepo.GetByUsername: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	defer logger.DeferLogDuration("user.GetByEmail", time.Now())()
	u := &model.User{}
	row := r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE email = $1`, email)
	if err := scanUser(row, u); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("userRepo.GetByEmail: %w", err)
	}
	return u, nil
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string, excludeUserID string) (bool, error) {
	defer logger.DeferLogDuration("user.ExistsByEmail", time.Now())()
	email = strings.TrimSpace(email)
	if email == "" {
		return false, nil
	}
	var exists bool
	if excludeUserID != "" {
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND id <> $2)`, email, excludeUserID).Scan(&exists); err != nil {
			return false, fmt.Errorf("userRepo.ExistsByEmail: %w", err)
		}
		return exists, nil
	}
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return false, fmt.Errorf("userRepo.ExistsByEmail: %w", err)
	}
	return exists, nil
}

func (r *UserRepository) ExistsByPhone(ctx context.Context, phone string, excludeUserID string) (bool, error) {
	defer logger.DeferLogDuration("user.ExistsByPhone", time.Now())()
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false, nil
	}
	var exists bool
	if excludeUserID != "" {
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1 AND id <> $2)`, phone, excludeUserID).Scan(&exists); err != nil {
			return false, fmt.Errorf("userRepo.ExistsByPhone: %w", err)
		}
		return exists, nil
	}
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1)`, phone).Scan(&exists); err != nil {
		return false, fmt.Errorf("userRepo.ExistsByPhone: %w", err)
	}
	return exists, nil
}

func (r *UserRepository) ListAll(ctx context.Context, limit int) ([]model.User, error) {
	defer logger.DeferLogDuration("user.ListAll", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT `+userCols+` FROM users ORDER BY username LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListAll: %w", err)
	}
	defer rows.Close()
	users := make([]model.User, 0, limit)
	for rows.Next() {
		var u model.User
		if err := scanUser(rows, &u); err != nil {
			return nil, fmt.Errorf("userRepo.ListAll scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("userRepo.ListAll rows: %w", err)
	}
	return users, nil
}

func (r *UserRepository) ListPage(ctx context.Context, q string, limit, offset int, sortKey, sortDir string) (*UserPageResult, error) {
	defer logger.DeferLogDuration("user.ListPage", time.Now())()

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	q = strings.TrimSpace(q)

	dir := "ASC"
	if strings.EqualFold(sortDir, "desc") {
		dir = "DESC"
	}

	orderBy := func() string {
		switch strings.ToLower(strings.TrimSpace(sortKey)) {
		case "email":
			return fmt.Sprintf("u.email %s, u.username ASC, u.id ASC", dir)
		case "phone":
			return fmt.Sprintf("u.phone %s, u.username ASC, u.id ASC", dir)
		case "last_seen_at":
			return fmt.Sprintf("u.last_seen_at %s NULLS LAST, u.username ASC, u.id ASC", dir)
		case "status":
			return fmt.Sprintf("CASE WHEN u.disabled_at IS NULL THEN 0 ELSE 1 END %s, u.username ASC, u.id ASC", dir)
		case "role":
			return fmt.Sprintf("CASE WHEN COALESCE(up.administrator,false) THEN 0 ELSE 1 END %s, u.username ASC, u.id ASC", dir)
		case "username", "":
			fallthrough
		default:
			return fmt.Sprintf("u.username %s, u.id ASC", dir)
		}
	}()

	where := ""
	args := make([]any, 0, 3)
	if q != "" {
		where = "WHERE u.username ILIKE $1 OR u.email ILIKE $1 OR u.phone ILIKE $1"
		args = append(args, "%"+q+"%")
	}

	// Total count
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users u "+where, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("userRepo.ListPage count: %w", err)
	}

	// Page
	argsPage := append([]any{}, args...)
	argsPage = append(argsPage, limit, offset)
	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	qry := fmt.Sprintf(
		"SELECT %s FROM users u LEFT JOIN user_permissions up ON up.user_id = u.id %s ORDER BY %s LIMIT $%d OFFSET $%d",
		userColsAliased, where, orderBy, limitPos, offsetPos,
	)
	rows, err := r.pool.Query(ctx, qry, argsPage...)
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListPage query: %w", err)
	}
	defer rows.Close()

	users := make([]model.User, 0, limit)
	for rows.Next() {
		var u model.User
		if err := scanUser(rows, &u); err != nil {
			return nil, fmt.Errorf("userRepo.ListPage scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("userRepo.ListPage rows: %w", err)
	}

	return &UserPageResult{Users: users, Total: total}, nil
}

func (r *UserRepository) SearchByUsername(ctx context.Context, query string, limit int) ([]model.User, error) {
	defer logger.DeferLogDuration("user.SearchByUsername", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT `+userCols+` FROM users WHERE username ILIKE $1 ORDER BY username LIMIT $2`,
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("userRepo.SearchByUsername query: %w", err)
	}
	defer rows.Close()

	users := make([]model.User, 0, limit)
	for rows.Next() {
		var u model.User
		if err := scanUser(rows, &u); err != nil {
			return nil, fmt.Errorf("userRepo.SearchByUsername scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("userRepo.SearchByUsername rows: %w", err)
	}
	return users, nil
}

func (r *UserRepository) SetOnline(ctx context.Context, userID string, online bool) error {
	defer logger.DeferLogDuration("user.SetOnline", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET is_online = $1, last_seen_at = $2 WHERE id = $3`,
		online, time.Now().UTC(), userID,
	)
	if err != nil {
		return fmt.Errorf("userRepo.SetOnline: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, userID, username, avatarURL, email, phone, position string) error {
	defer logger.DeferLogDuration("user.UpdateProfile", time.Now())()
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET username = $1, avatar_url = $2, email = NULLIF($3, ''), phone = $4, position = $5 WHERE id = $6`,
		username, avatarURL, email, phone, position, userID,
	)
	if err != nil {
		return fmt.Errorf("userRepo.UpdateProfile: %w", err)
	}
	return nil
}

func (r *UserRepository) GetFavoriteChatIDs(ctx context.Context, userID string) ([]string, error) {
	defer logger.DeferLogDuration("user.GetFavoriteChatIDs", time.Now())()
	rows, err := r.pool.Query(ctx,
		`SELECT chat_id FROM user_favorite_chats WHERE user_id = $1 ORDER BY chat_id`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("userRepo.GetFavoriteChatIDs: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("userRepo.GetFavoriteChatIDs scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *UserRepository) AddFavorite(ctx context.Context, userID, chatID string) error {
	defer logger.DeferLogDuration("user.AddFavorite", time.Now())()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_favorite_chats (user_id, chat_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, chatID,
	)
	if err != nil {
		return fmt.Errorf("userRepo.AddFavorite: %w", err)
	}
	return nil
}

func (r *UserRepository) RemoveFavorite(ctx context.Context, userID, chatID string) error {
	defer logger.DeferLogDuration("user.RemoveFavorite", time.Now())()
	_, err := r.pool.Exec(ctx,
		`DELETE FROM user_favorite_chats WHERE user_id = $1 AND chat_id = $2`,
		userID, chatID,
	)
	if err != nil {
		return fmt.Errorf("userRepo.RemoveFavorite: %w", err)
	}
	return nil
}

// SetDisabled 2Ñ‹AÑ‚02;O5Ñ‚ 8;8 A=8<05Ñ‚ >Ñ‚:;ÑŽÑ‡5=85 ?>;ÑŒ7>20Ñ‚5;O (Ñ‚>;ÑŒ:> 4;O 04<8=8AÑ‚Ñ€0Ñ‚>Ñ€0 Ñ‡5Ñ€57 API).
func (r *UserRepository) SetDisabled(ctx context.Context, userID string, disabled bool) error {
	defer logger.DeferLogDuration("user.SetDisabled", time.Now())()
	if disabled {
		_, err := r.pool.Exec(ctx, `UPDATE users SET disabled_at = NOW() WHERE id = $1`, userID)
		if err != nil {
			return fmt.Errorf("userRepo.SetDisabled: %w", err)
		}
	} else {
		_, err := r.pool.Exec(ctx, `UPDATE users SET disabled_at = NULL WHERE id = $1`, userID)
		if err != nil {
			return fmt.Errorf("userRepo.SetDisabled: %w", err)
		}
	}
	return nil
}

// SetLoginKey replaces current login key metadata for the user.
func (r *UserRepository) SetLoginKey(ctx context.Context, userID, keyHash string, generatedAt time.Time) error {
	defer logger.DeferLogDuration("user.SetLoginKey", time.Now())()
	tag, err := r.pool.Exec(ctx,
		`UPDATE users
		 SET login_key_hash = $1,
		     login_key_attempts = 0,
		     login_key_active = TRUE,
		     login_key_generated_at = $2
		 WHERE id = $3`,
		keyHash, generatedAt.UTC(), userID,
	)
	if err != nil {
		return fmt.Errorf("userRepo.SetLoginKey: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsumeLoginKeyAttempt atomically consumes one login-key attempt.
// It guarantees race-safe increment and invalidation after maxAttempts.
func (r *UserRepository) ConsumeLoginKeyAttempt(ctx context.Context, keyHash string, maxAttempts int) (*model.User, bool, error) {
	defer logger.DeferLogDuration("user.ConsumeLoginKeyAttempt", time.Now())()
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("userRepo.ConsumeLoginKeyAttempt begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		u        model.User
		attempts int
	)
	row := tx.QueryRow(ctx,
		`SELECT `+userCols+`, COALESCE(login_key_attempts, 0)
		   FROM users
		  WHERE login_key_active = TRUE AND login_key_hash = $1
		  FOR UPDATE`,
		keyHash,
	)
	if err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.Phone, &u.Position, &u.PasswordHash, &u.AvatarURL, &u.LastSeenAt, &u.IsOnline, &u.CreatedAt, &u.DisabledAt,
		&attempts,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("userRepo.ConsumeLoginKeyAttempt scan: %w", err)
	}

	nextAttempts := attempts + 1
	exhausted := nextAttempts >= maxAttempts
	if exhausted {
		_, err = tx.Exec(ctx,
			`UPDATE users
			    SET login_key_hash = NULL,
			        login_key_attempts = 0,
			        login_key_active = FALSE,
			        login_key_generated_at = NULL
			  WHERE id = $1`,
			u.ID,
		)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE users
			    SET login_key_attempts = $1
			  WHERE id = $2`,
			nextAttempts, u.ID,
		)
	}
	if err != nil {
		return nil, false, fmt.Errorf("userRepo.ConsumeLoginKeyAttempt update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("userRepo.ConsumeLoginKeyAttempt commit: %w", err)
	}
	return &u, exhausted, nil
}
