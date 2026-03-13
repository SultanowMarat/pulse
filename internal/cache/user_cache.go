package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultUserProfileTTL = 60 * time.Second
	defaultUserPermTTL    = 45 * time.Second
	defaultUserListTTL    = 15 * time.Second
	defaultUserIndexTTL   = 30 * time.Minute
)

// UserCache stores hot user/admin payloads in Redis.
// All methods are no-op when Redis client is nil.
type UserCache struct {
	cli *redis.Client

	profileTTL time.Duration
	permTTL    time.Duration
	listTTL    time.Duration
	indexTTL   time.Duration
}

func NewUserCache(cli *redis.Client) *UserCache {
	return &UserCache{
		cli:        cli,
		profileTTL: defaultUserProfileTTL,
		permTTL:    defaultUserPermTTL,
		listTTL:    defaultUserListTTL,
		indexTTL:   defaultUserIndexTTL,
	}
}

func (c *UserCache) Enabled() bool {
	return c != nil && c.cli != nil
}

func (c *UserCache) Profile(ctx context.Context, userID string, out any) (bool, error) {
	return c.getJSON(ctx, c.profileKey(userID), out)
}

func (c *UserCache) SetProfile(ctx context.Context, userID string, value any) error {
	return c.setJSON(ctx, c.profileKey(userID), value, c.profileTTL)
}

func (c *UserCache) InvalidateProfiles(ctx context.Context, userIDs ...string) error {
	if !c.Enabled() || len(userIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		keys = append(keys, c.profileKey(userID))
	}
	if len(keys) == 0 {
		return nil
	}
	if err := c.cli.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("userCache.InvalidateProfiles del: %w", err)
	}
	return nil
}

func (c *UserCache) Permission(ctx context.Context, userID string, out any) (bool, error) {
	return c.getJSON(ctx, c.permissionKey(userID), out)
}

func (c *UserCache) SetPermission(ctx context.Context, userID string, value any) error {
	return c.setJSON(ctx, c.permissionKey(userID), value, c.permTTL)
}

func (c *UserCache) InvalidatePermissions(ctx context.Context, userIDs ...string) error {
	if !c.Enabled() || len(userIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		keys = append(keys, c.permissionKey(userID))
	}
	if len(keys) == 0 {
		return nil
	}
	if err := c.cli.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("userCache.InvalidatePermissions del: %w", err)
	}
	return nil
}

func (c *UserCache) UsersList(ctx context.Context, viewerID string, limit int, out any) (bool, error) {
	return c.getJSON(ctx, fmt.Sprintf("users:list:v:%s:l:%d", viewerID, limit), out)
}

func (c *UserCache) SetUsersList(ctx context.Context, viewerID string, limit int, value any) error {
	return c.setListJSON(ctx, fmt.Sprintf("users:list:v:%s:l:%d", viewerID, limit), value)
}

func (c *UserCache) UsersSearch(ctx context.Context, viewerID, query string, limit int, out any) (bool, error) {
	key := fmt.Sprintf("users:search:v:%s:%s", viewerID, c.digest(query, fmt.Sprintf("%d", limit)))
	return c.getJSON(ctx, key, out)
}

func (c *UserCache) SetUsersSearch(ctx context.Context, viewerID, query string, limit int, value any) error {
	key := fmt.Sprintf("users:search:v:%s:%s", viewerID, c.digest(query, fmt.Sprintf("%d", limit)))
	return c.setListJSON(ctx, key, value)
}

func (c *UserCache) EmployeesList(ctx context.Context, limit int, out any) (bool, error) {
	return c.getJSON(ctx, fmt.Sprintf("employees:list:l:%d", limit), out)
}

func (c *UserCache) SetEmployeesList(ctx context.Context, limit int, value any) error {
	return c.setListJSON(ctx, fmt.Sprintf("employees:list:l:%d", limit), value)
}

func (c *UserCache) EmployeesPage(ctx context.Context, q string, limit, offset int, sortKey, sortDir string, out any) (bool, error) {
	key := fmt.Sprintf("employees:page:%s", c.digest(q, fmt.Sprintf("%d", limit), fmt.Sprintf("%d", offset), sortKey, sortDir))
	return c.getJSON(ctx, key, out)
}

func (c *UserCache) SetEmployeesPage(ctx context.Context, q string, limit, offset int, sortKey, sortDir string, value any) error {
	key := fmt.Sprintf("employees:page:%s", c.digest(q, fmt.Sprintf("%d", limit), fmt.Sprintf("%d", offset), sortKey, sortDir))
	return c.setListJSON(ctx, key, value)
}

func (c *UserCache) InvalidateListCaches(ctx context.Context) error {
	if !c.Enabled() {
		return nil
	}
	idx := c.listIndexKey()
	keys, err := c.cli.SMembers(ctx, idx).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("userCache.InvalidateListCaches members: %w", err)
	}
	keys = append(keys, idx)
	if len(keys) == 0 {
		return nil
	}
	if err := c.cli.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("userCache.InvalidateListCaches del: %w", err)
	}
	return nil
}

func (c *UserCache) setListJSON(ctx context.Context, key string, value any) error {
	if !c.Enabled() {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("userCache.setListJSON marshal key=%s: %w", key, err)
	}
	idx := c.listIndexKey()
	pipe := c.cli.Pipeline()
	pipe.Set(ctx, key, raw, c.listTTL)
	pipe.SAdd(ctx, idx, key)
	pipe.Expire(ctx, idx, c.indexTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("userCache.setListJSON exec: %w", err)
	}
	return nil
}

func (c *UserCache) listIndexKey() string {
	return "users:list:index"
}

func (c *UserCache) profileKey(userID string) string {
	return "user:profile:" + userID
}

func (c *UserCache) permissionKey(userID string) string {
	return "user:perm:" + userID
}

func (c *UserCache) digest(parts ...string) string {
	h := sha1.New()
	for _, p := range parts {
		_, _ = io.WriteString(h, p)
		_, _ = io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *UserCache) getJSON(ctx context.Context, key string, out any) (bool, error) {
	if !c.Enabled() {
		return false, nil
	}
	raw, err := c.cli.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("userCache.getJSON get key=%s: %w", key, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		_ = c.cli.Del(ctx, key).Err()
		return false, fmt.Errorf("userCache.getJSON unmarshal key=%s: %w", key, err)
	}
	return true, nil
}

func (c *UserCache) setJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("userCache.setJSON marshal key=%s: %w", key, err)
	}
	if err := c.cli.Set(ctx, key, raw, ttl).Err(); err != nil {
		return fmt.Errorf("userCache.setJSON set key=%s: %w", key, err)
	}
	return nil
}
