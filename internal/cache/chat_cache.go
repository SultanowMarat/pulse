package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultMessagesTTL  = 45 * time.Second
	defaultUserChatsTTL = 30 * time.Second
	defaultMembersTTL   = 2 * time.Minute
	defaultIndexTTL     = 30 * time.Minute
)

// ChatCache stores hot chat data in Redis.
// All methods are no-op when Redis client is nil.
type ChatCache struct {
	cli *redis.Client

	messagesTTL  time.Duration
	userChatsTTL time.Duration
	membersTTL   time.Duration
	indexTTL     time.Duration
}

func NewChatCache(cli *redis.Client) *ChatCache {
	return &ChatCache{
		cli:          cli,
		messagesTTL:  defaultMessagesTTL,
		userChatsTTL: defaultUserChatsTTL,
		membersTTL:   defaultMembersTTL,
		indexTTL:     defaultIndexTTL,
	}
}

func (c *ChatCache) Enabled() bool {
	return c != nil && c.cli != nil
}

func (c *ChatCache) MessageList(ctx context.Context, chatID, userID string, limit, offset int, out any) (bool, error) {
	key := c.messagesKey(chatID, userID, limit, offset)
	return c.getJSON(ctx, key, out)
}

func (c *ChatCache) SetMessageList(ctx context.Context, chatID, userID string, limit, offset int, value any) error {
	if !c.Enabled() {
		return nil
	}
	key := c.messagesKey(chatID, userID, limit, offset)
	idx := c.messagesIndexKey(chatID)
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("chatCache.SetMessageList marshal: %w", err)
	}

	pipe := c.cli.Pipeline()
	pipe.Set(ctx, key, raw, c.messagesTTL)
	pipe.SAdd(ctx, idx, key)
	pipe.Expire(ctx, idx, c.indexTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("chatCache.SetMessageList exec: %w", err)
	}
	return nil
}

func (c *ChatCache) InvalidateMessageLists(ctx context.Context, chatID string) error {
	if !c.Enabled() {
		return nil
	}
	idx := c.messagesIndexKey(chatID)
	keys, err := c.cli.SMembers(ctx, idx).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("chatCache.InvalidateMessageLists members: %w", err)
	}
	keys = append(keys, idx)
	if len(keys) == 0 {
		return nil
	}
	if err := c.cli.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("chatCache.InvalidateMessageLists del: %w", err)
	}
	return nil
}

func (c *ChatCache) UserChats(ctx context.Context, userID string, out any) (bool, error) {
	key := c.userChatsKey(userID)
	return c.getJSON(ctx, key, out)
}

func (c *ChatCache) SetUserChats(ctx context.Context, userID string, value any) error {
	key := c.userChatsKey(userID)
	return c.setJSON(ctx, key, value, c.userChatsTTL)
}

func (c *ChatCache) InvalidateUserChats(ctx context.Context, userIDs ...string) error {
	if !c.Enabled() || len(userIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		keys = append(keys, c.userChatsKey(userID))
	}
	if len(keys) == 0 {
		return nil
	}
	if err := c.cli.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("chatCache.InvalidateUserChats del: %w", err)
	}
	return nil
}

func (c *ChatCache) ChatMembers(ctx context.Context, chatID string, out any) (bool, error) {
	key := c.chatMembersKey(chatID)
	return c.getJSON(ctx, key, out)
}

func (c *ChatCache) SetChatMembers(ctx context.Context, chatID string, value any) error {
	key := c.chatMembersKey(chatID)
	return c.setJSON(ctx, key, value, c.membersTTL)
}

func (c *ChatCache) InvalidateChatMembers(ctx context.Context, chatID string) error {
	if !c.Enabled() || chatID == "" {
		return nil
	}
	if err := c.cli.Del(ctx, c.chatMembersKey(chatID)).Err(); err != nil {
		return fmt.Errorf("chatCache.InvalidateChatMembers del: %w", err)
	}
	return nil
}

func (c *ChatCache) messagesKey(chatID, userID string, limit, offset int) string {
	return fmt.Sprintf("chat:messages:%s:%s:%d:%d", chatID, userID, limit, offset)
}

func (c *ChatCache) messagesIndexKey(chatID string) string {
	return "chat:messages:index:" + chatID
}

func (c *ChatCache) userChatsKey(userID string) string {
	return "user:chats:" + userID
}

func (c *ChatCache) chatMembersKey(chatID string) string {
	return "chat:members:" + chatID
}

func (c *ChatCache) getJSON(ctx context.Context, key string, out any) (bool, error) {
	if !c.Enabled() {
		return false, nil
	}
	raw, err := c.cli.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("chatCache.getJSON get key=%s: %w", key, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		_ = c.cli.Del(ctx, key).Err()
		return false, fmt.Errorf("chatCache.getJSON unmarshal key=%s: %w", key, err)
	}
	return true, nil
}

func (c *ChatCache) setJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("chatCache.setJSON marshal key=%s: %w", key, err)
	}
	if err := c.cli.Set(ctx, key, raw, ttl).Err(); err != nil {
		return fmt.Errorf("chatCache.setJSON set key=%s: %w", key, err)
	}
	return nil
}
