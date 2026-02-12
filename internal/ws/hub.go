package ws

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/model"
	"github.com/messenger/internal/repository"
	"github.com/messenger/internal/runtime"
)

// PushNotifier отправляет пуш-уведомления. Если nil — пуши не отправляются.
type PushNotifier interface {
	Notify(ctx context.Context, userID, title, body string, data map[string]string)
}

// MessageAssetCleaner removes message files from storage by message file URL.
type MessageAssetCleaner interface {
	DeleteByMessageFileURL(ctx context.Context, fileURL string) error
}

type Hub struct {
	mu           sync.RWMutex
	clients      map[string]map[*Client]struct{}
	total        int
	maxConns     int
	chatRepo     *repository.ChatRepository
	msgRepo      *repository.MessageRepository
	userRepo     *repository.UserRepository
	reactRepo    *repository.ReactionRepository
	pinnedRepo   *repository.PinnedRepository
	pushClient   PushNotifier
	assetCleaner MessageAssetCleaner
	register     chan *Client
	unregister   chan *Client
	done         chan struct{}
}

func NewHub(
	chatRepo *repository.ChatRepository,
	msgRepo *repository.MessageRepository,
	userRepo *repository.UserRepository,
	reactRepo *repository.ReactionRepository,
	pinnedRepo *repository.PinnedRepository,
	maxConns int,
	pushClient PushNotifier,
	assetCleaner MessageAssetCleaner,
) *Hub {
	if maxConns <= 0 {
		maxConns = 10000
	}
	return &Hub{
		clients:      make(map[string]map[*Client]struct{}),
		maxConns:     maxConns,
		chatRepo:     chatRepo,
		msgRepo:      msgRepo,
		userRepo:     userRepo,
		reactRepo:    reactRepo,
		pinnedRepo:   pinnedRepo,
		pushClient:   pushClient,
		assetCleaner: assetCleaner,
		register:     make(chan *Client, 64),
		unregister:   make(chan *Client, 64),
		done:         make(chan struct{}),
	}
}

func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return
		case client := <-h.register:
			h.addClient(client)
		case client := <-h.unregister:
			h.removeClient(client)
		}
	}
}

func (h *Hub) shutdown() {
	// Collect all clients under the lock, do NOT perform I/O under mutex.
	h.mu.Lock()
	allClients := make([]*Client, 0, h.total)
	for _, clients := range h.clients {
		for c := range clients {
			allClients = append(allClients, c)
		}
	}
	h.clients = make(map[string]map[*Client]struct{})
	h.total = 0
	h.mu.Unlock()

	// Close connections outside the lock (network I/O).
	for _, c := range allClients {
		c.Close()
	}
	for _, c := range allClients {
		c.Wait()
	}
}

func (h *Hub) addClient(c *Client) {
	h.mu.Lock()
	maxConns := h.maxConns
	if s, _ := runtime.GetServiceSettings(); s.MaxWSConnections > 0 {
		maxConns = s.MaxWSConnections
	}
	if h.total >= maxConns {
		h.mu.Unlock()
		logger.Errorf("ws connection limit reached (%d), rejecting user=%s", maxConns, c.userID)
		c.Close()
		return
	}
	if _, ok := h.clients[c.userID]; !ok {
		h.clients[c.userID] = make(map[*Client]struct{})
	}
	h.clients[c.userID][c] = struct{}{}
	h.total++
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.userRepo.SetOnline(ctx, c.userID, true); err != nil {
		logger.Errorf("ws set online user=%s: %v", c.userID, err)
	}
	h.broadcastUserStatus(c.userID, true)
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	clients, ok := h.clients[c.userID]
	if !ok {
		h.mu.Unlock()
		return
	}
	if _, exists := clients[c]; !exists {
		h.mu.Unlock()
		return
	}
	delete(clients, c)
	h.total--
	lastClient := len(clients) == 0
	if lastClient {
		delete(h.clients, c.userID)
	}
	h.mu.Unlock()

	// Network I/O outside the lock.
	c.Close()

	if lastClient {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.userRepo.SetOnline(ctx, c.userID, false); err != nil {
			logger.Errorf("ws set offline user=%s: %v", c.userID, err)
		}
		h.broadcastUserStatus(c.userID, false)
	}
}

// HandleMessage dispatches incoming WebSocket messages.
func (h *Hub) HandleMessage(ctx context.Context, c *Client, msg IncomingMessage) {
	switch msg.Type {
	case EventNewMessage:
		h.handleNewMessage(ctx, c, msg)
	case EventTyping:
		h.handleTyping(ctx, c, msg)
	case EventMessageRead:
		h.handleMessageRead(ctx, c, msg)
	case EventMessageEdited:
		h.handleEditMessage(ctx, c, msg)
	case EventMessageDeleted:
		h.handleDeleteMessage(ctx, c, msg)
	case EventReactionAdded:
		h.handleAddReaction(ctx, c, msg)
	case EventReactionRemoved:
		h.handleRemoveReaction(ctx, c, msg)
	case EventMessagePinned:
		h.handlePinMessage(ctx, c, msg)
	case EventMessageUnpinned:
		h.handleUnpinMessage(ctx, c, msg)
	default:
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "unknown event type"})
	}
}

// KickAll closes all active WS connections. Used to apply runtime settings that require reconnect (e.g. send buffer size).
func (h *Hub) KickAll() {
	h.mu.RLock()
	all := make([]*Client, 0, h.total)
	for _, clients := range h.clients {
		for c := range clients {
			all = append(all, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range all {
		c.Close()
	}
}

func (h *Hub) handleNewMessage(ctx context.Context, c *Client, msg IncomingMessage) {
	defer logger.DeferLogDuration("ws.handleNewMessage", time.Now())()
	if msg.ChatID == "" || (msg.Content == "" && msg.FileURL == "") {
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "chat_id and content required"})
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	isMember, err := h.chatRepo.IsMember(ctx, msg.ChatID, c.userID)
	if err != nil {
		logger.Errorf("ws check membership chat=%s user=%s: %v", msg.ChatID, c.userID, err)
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "internal error"})
		return
	}
	if !isMember {
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "not a member"})
		return
	}

	contentType := model.ContentTypeText
	if msg.ContentType != "" {
		contentType = msg.ContentType
	}

	var replyToID *string
	if msg.ReplyToID != "" {
		replyToID = &msg.ReplyToID
	}

	// Нормализация имени файла: "+" часто приходит вместо пробела (URL-кодирование), сохраняем в БД с пробелами (UTF-8).
	fileName := strings.TrimSpace(strings.ReplaceAll(msg.FileName, "+", " "))
	now := time.Now().UTC()
	m := &model.Message{
		ID:          uuid.New().String(),
		ChatID:      msg.ChatID,
		SenderID:    c.userID,
		Content:     msg.Content,
		ContentType: contentType,
		FileURL:     msg.FileURL,
		FileName:    fileName,
		FileSize:    msg.FileSize,
		Status:      model.MessageStatusSent,
		ReplyToID:   replyToID,
		CreatedAt:   now,
	}

	if err := h.msgRepo.Create(ctx, m); err != nil {
		logger.Errorf("ws save message chat=%s user=%s: %v", msg.ChatID, c.userID, err)
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "failed to save message"})
		return
	}

	sender, err := h.userRepo.GetByID(ctx, c.userID)
	if err != nil {
		logger.Errorf("ws get sender user=%s: %v", c.userID, err)
	} else {
		pub := sender.ToPublic()
		m.Sender = &pub
	}

	// Attach reply-to preview if present
	if replyToID != nil {
		replyMsg, err := h.msgRepo.GetByID(ctx, *replyToID)
		if err == nil {
			m.ReplyTo = &model.Message{
				ID:          replyMsg.ID,
				SenderID:    replyMsg.SenderID,
				Content:     replyMsg.Content,
				ContentType: replyMsg.ContentType,
				Sender:      replyMsg.Sender,
			}
		}
	}

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, msg.ChatID)
	if err != nil {
		logger.Errorf("ws get members chat=%s: %v", msg.ChatID, err)
		return
	}

	out := OutgoingMessage{Type: EventNewMessage, Payload: m}
	for _, uid := range memberIDs {
		h.sendToUser(uid, out)
	}

	// Пуш-уведомления получателям (кроме отправителя)
	if h.pushClient != nil {
		senderName := ""
		if m.Sender != nil {
			senderName = m.Sender.Username
		}
		if senderName == "" {
			senderName = "Сообщение"
		}
		body := m.Content
		if m.ContentType != "text" || body == "" {
			body = "Вложение"
		}
		if len(body) > 120 {
			body = body[:117] + "..."
		}
		data := map[string]string{"chat_id": msg.ChatID, "message_id": m.ID}
		muteMap, err := h.chatRepo.GetMemberMuteMap(ctx, msg.ChatID)
		if err != nil {
			logger.Errorf("ws get mute map chat=%s: %v", msg.ChatID, err)
		}
		for _, uid := range memberIDs {
			if uid == c.userID {
				continue
			}
			if muteMap != nil && muteMap[uid] {
				continue
			}
			uid := uid
			go h.pushClient.Notify(context.Background(), uid, senderName, body, data)
		}
	}
}

func (h *Hub) handleEditMessage(ctx context.Context, c *Client, msg IncomingMessage) {
	defer logger.DeferLogDuration("ws.handleEditMessage", time.Now())()
	if msg.MessageID == "" || msg.Content == "" {
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "message_id and content required"})
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	original, err := h.msgRepo.GetByID(ctx, msg.MessageID)
	if err != nil {
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "message not found"})
		return
	}
	if original.SenderID != c.userID {
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "can only edit own messages"})
		return
	}

	now := time.Now().UTC()
	if err := h.msgRepo.UpdateContent(ctx, msg.MessageID, msg.Content, now); err != nil {
		logger.Errorf("ws edit message %s: %v", msg.MessageID, err)
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "failed to edit"})
		return
	}

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, original.ChatID)
	if err != nil {
		return
	}

	out := OutgoingMessage{Type: EventMessageEdited, Payload: MessageEditedPayload{
		MessageID: msg.MessageID,
		ChatID:    original.ChatID,
		Content:   msg.Content,
		EditedAt:  now,
	}}
	for _, uid := range memberIDs {
		h.sendToUser(uid, out)
	}
}

func (h *Hub) handleDeleteMessage(ctx context.Context, c *Client, msg IncomingMessage) {
	defer logger.DeferLogDuration("ws.handleDeleteMessage", time.Now())()
	if msg.MessageID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	original, err := h.msgRepo.GetByID(ctx, msg.MessageID)
	if err != nil {
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "message not found"})
		return
	}
	if original.SenderID != c.userID {
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "can only delete own messages"})
		return
	}

	if strings.TrimSpace(original.FileURL) != "" {
		linkedCount, err := h.msgRepo.CountActiveByFileURLExcludingMessage(ctx, original.FileURL, msg.MessageID)
		if err != nil {
			logger.Errorf("ws count linked files message=%s file_url=%s: %v", msg.MessageID, original.FileURL, err)
			h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "failed to delete message"})
			return
		}
		if linkedCount == 0 && h.assetCleaner != nil {
			if err := h.assetCleaner.DeleteByMessageFileURL(ctx, original.FileURL); err != nil {
				logger.Errorf("ws delete file asset message=%s file_url=%s: %v", msg.MessageID, original.FileURL, err)
				h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "failed to delete file"})
				return
			}
		}
	}

	if err := h.msgRepo.SoftDelete(ctx, msg.MessageID); err != nil {
		logger.Errorf("ws soft delete message %s: %v", msg.MessageID, err)
		h.sendToClient(c, OutgoingMessage{Type: EventError, Payload: "failed to delete message"})
		return
	}

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, original.ChatID)
	if err != nil {
		return
	}

	out := OutgoingMessage{Type: EventMessageDeleted, Payload: MessageDeletedPayload{
		MessageID: msg.MessageID,
		ChatID:    original.ChatID,
	}}
	for _, uid := range memberIDs {
		h.sendToUser(uid, out)
	}
}

func (h *Hub) handleAddReaction(ctx context.Context, c *Client, msg IncomingMessage) {
	if msg.MessageID == "" || msg.Emoji == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	original, err := h.msgRepo.GetByID(ctx, msg.MessageID)
	if err != nil {
		return
	}

	if err := h.reactRepo.Add(ctx, msg.MessageID, c.userID, msg.Emoji); err != nil {
		logger.Errorf("ws add reaction %s: %v", msg.MessageID, err)
		return
	}

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, original.ChatID)
	if err != nil {
		return
	}

	out := OutgoingMessage{Type: EventReactionAdded, Payload: ReactionPayload{
		MessageID: msg.MessageID,
		ChatID:    original.ChatID,
		UserID:    c.userID,
		Emoji:     msg.Emoji,
	}}
	for _, uid := range memberIDs {
		h.sendToUser(uid, out)
	}
}

func (h *Hub) handleRemoveReaction(ctx context.Context, c *Client, msg IncomingMessage) {
	if msg.MessageID == "" || msg.Emoji == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	original, err := h.msgRepo.GetByID(ctx, msg.MessageID)
	if err != nil {
		return
	}

	if err := h.reactRepo.Remove(ctx, msg.MessageID, c.userID, msg.Emoji); err != nil {
		logger.Errorf("ws remove reaction %s: %v", msg.MessageID, err)
		return
	}

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, original.ChatID)
	if err != nil {
		return
	}

	out := OutgoingMessage{Type: EventReactionRemoved, Payload: ReactionPayload{
		MessageID: msg.MessageID,
		ChatID:    original.ChatID,
		UserID:    c.userID,
		Emoji:     msg.Emoji,
	}}
	for _, uid := range memberIDs {
		h.sendToUser(uid, out)
	}
}

func (h *Hub) handlePinMessage(ctx context.Context, c *Client, msg IncomingMessage) {
	if msg.MessageID == "" || msg.ChatID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.pinnedRepo.Pin(ctx, msg.ChatID, msg.MessageID, c.userID); err != nil {
		logger.Errorf("ws pin message %s: %v", msg.MessageID, err)
		return
	}

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, msg.ChatID)
	if err != nil {
		return
	}

	out := OutgoingMessage{Type: EventMessagePinned, Payload: PinPayload{
		MessageID: msg.MessageID,
		ChatID:    msg.ChatID,
		PinnedBy:  c.userID,
	}}
	for _, uid := range memberIDs {
		h.sendToUser(uid, out)
	}
}

func (h *Hub) handleUnpinMessage(ctx context.Context, c *Client, msg IncomingMessage) {
	if msg.MessageID == "" || msg.ChatID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.pinnedRepo.Unpin(ctx, msg.ChatID, msg.MessageID); err != nil {
		logger.Errorf("ws unpin message %s: %v", msg.MessageID, err)
		return
	}

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, msg.ChatID)
	if err != nil {
		return
	}

	out := OutgoingMessage{Type: EventMessageUnpinned, Payload: UnpinPayload{
		MessageID: msg.MessageID,
		ChatID:    msg.ChatID,
	}}
	for _, uid := range memberIDs {
		h.sendToUser(uid, out)
	}
}

func (h *Hub) handleTyping(ctx context.Context, c *Client, msg IncomingMessage) {
	if msg.ChatID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, msg.ChatID)
	if err != nil {
		logger.Errorf("ws get members for typing chat=%s: %v", msg.ChatID, err)
		return
	}

	out := OutgoingMessage{
		Type: EventTyping,
		Payload: TypingPayload{
			ChatID: msg.ChatID,
			UserID: c.userID,
		},
	}
	for _, uid := range memberIDs {
		if uid != c.userID {
			h.sendToUser(uid, out)
		}
	}
}

func (h *Hub) handleMessageRead(ctx context.Context, c *Client, msg IncomingMessage) {
	if msg.ChatID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.msgRepo.MarkAsRead(ctx, msg.ChatID, c.userID); err != nil {
		logger.Errorf("ws mark read chat=%s user=%s: %v", msg.ChatID, c.userID, err)
		return
	}

	// Update last_read_at for unread count tracking
	now := time.Now().UTC()
	if err := h.chatRepo.UpdateMemberLastRead(ctx, msg.ChatID, c.userID, now); err != nil {
		logger.Errorf("ws update last_read_at chat=%s user=%s: %v", msg.ChatID, c.userID, err)
	}

	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, msg.ChatID)
	if err != nil {
		logger.Errorf("ws get members for read chat=%s: %v", msg.ChatID, err)
		return
	}

	out := OutgoingMessage{
		Type: EventMessageRead,
		Payload: MessageReadPayload{
			ChatID: msg.ChatID,
			UserID: c.userID,
		},
	}
	for _, uid := range memberIDs {
		if uid != c.userID {
			h.sendToUser(uid, out)
		}
	}
}

func (h *Hub) broadcastUserStatus(userID string, online bool) {
	evType := EventUserOffline
	if online {
		evType = EventUserOnline
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chats, err := h.chatRepo.GetUserChats(ctx, userID)
	if err != nil {
		logger.Errorf("ws get chats for status broadcast user=%s: %v", userID, err)
		return
	}

	out := OutgoingMessage{
		Type: evType,
		Payload: UserStatusPayload{
			UserID: userID,
			Online: online,
		},
	}

	notified := make(map[string]struct{}, 16)
	for _, chat := range chats {
		memberIDs, err := h.chatRepo.GetMemberIDs(ctx, chat.ID)
		if err != nil {
			logger.Errorf("ws get members for status broadcast chat=%s: %v", chat.ID, err)
			continue
		}
		for _, uid := range memberIDs {
			if uid == userID {
				continue
			}
			if _, ok := notified[uid]; ok {
				continue
			}
			notified[uid] = struct{}{}
			h.sendToUser(uid, out)
		}
	}
}

// BroadcastToChat sends a message to all members of a chat.
func (h *Hub) BroadcastToChat(ctx context.Context, chatID string, msg OutgoingMessage) {
	defer logger.DeferLogDuration("ws.BroadcastToChat", time.Now())()
	memberIDs, err := h.chatRepo.GetMemberIDs(ctx, chatID)
	if err != nil {
		logger.Errorf("ws broadcast to chat %s: %v", chatID, err)
		return
	}
	for _, uid := range memberIDs {
		h.sendToUser(uid, msg)
	}
}

func (h *Hub) sendToUser(userID string, msg OutgoingMessage) {
	h.mu.RLock()
	clients, ok := h.clients[userID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	targets := make([]*Client, 0, len(clients))
	for c := range clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		h.sendToClient(c, msg)
	}
}

// KickUser sends a "session_revoked" event and closes all active WS connections for the user.
// This is used to force re-auth on all devices.
func (h *Hub) KickUser(userID string) {
	h.mu.RLock()
	clients, ok := h.clients[userID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	targets := make([]*Client, 0, len(clients))
	for c := range clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		h.sendToClient(c, OutgoingMessage{Type: EventSessionRevoked, Payload: map[string]string{"status": "revoked"}})
		c.Close()
	}
}

func (h *Hub) sendToClient(c *Client, msg OutgoingMessage) {
	select {
	case c.send <- msg:
	case <-c.done:
	default:
		// Backpressure: send buffer full, close slow client.
		logger.Errorf("ws send buffer full, closing slow client user=%s", c.userID)
		c.Close()
	}
}

func (h *Hub) Register(c *Client) {
	select {
	case h.register <- c:
	case <-h.done:
		c.Close()
	}
}

func (h *Hub) Unregister(c *Client) {
	select {
	case h.unregister <- c:
	case <-h.done:
	}
}
