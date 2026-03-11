package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/runtime"
)

func wsWriteWait() time.Duration {
	s, _ := runtime.GetServiceSettings()
	if s.WSWriteTimeout <= 0 {
		return 10 * time.Second
	}
	return time.Duration(s.WSWriteTimeout) * time.Second
}

func wsPongWait() time.Duration {
	s, _ := runtime.GetServiceSettings()
	if s.WSPongTimeout <= 0 {
		return 60 * time.Second
	}
	return time.Duration(s.WSPongTimeout) * time.Second
}

func wsMaxMessageSize() int64 {
	s, _ := runtime.GetServiceSettings()
	if s.WSMaxMessageSize <= 0 {
		return 4096
	}
	return int64(s.WSMaxMessageSize)
}

func wsSendBufSize() int {
	s, _ := runtime.GetServiceSettings()
	if s.WSSendBufferSize <= 0 {
		return 256
	}
	return s.WSSendBufferSize
}

// bufPool pools bytes.Buffer for JSON encoding in the hot-path (writePump).
var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// Client represents a single WebSocket connection.
// Lifecycle: NewClient -> Start(ctx, cancel) -> [ReadPump, WritePump] -> Close -> Wait.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan OutgoingMessage
	userID string

	// done is used as a non-blocking guard in sendToClient.
	done chan struct{}
	// cancel cancels the context passed to Start, triggering pump shutdown.
	cancel context.CancelFunc
	once   sync.Once
	wg     sync.WaitGroup
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan OutgoingMessage, wsSendBufSize()),
		userID: userID,
		done:   make(chan struct{}),
	}
}

// Start launches ReadPump and WritePump goroutines with controlled lifecycle.
// ctx controls pump lifetime; cancel is stored for Close().
func (c *Client) Start(ctx context.Context, cancel context.CancelFunc) {
	c.cancel = cancel
	c.wg.Add(2)
	go c.writePump(ctx)
	go c.readPump(ctx)
}

// Wait blocks until both pump goroutines have exited.
func (c *Client) Wait() {
	c.wg.Wait()
}

// Close signals the client to stop. Safe to call multiple times from any goroutine.
func (c *Client) Close() {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		close(c.done)
		// Force both pumps to unblock (ReadMessage / WriteMessage will error).
		c.conn.Close()
	})
}

// readPump reads messages from the WebSocket connection.
// Exits on read error (triggered by conn.Close from Close() or WritePump exit).
func (c *Client) readPump(ctx context.Context) {
	defer c.wg.Done()
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(wsMaxMessageSize())
	if err := c.conn.SetReadDeadline(time.Now().Add(wsPongWait())); err != nil {
		logger.Errorf("ws set read deadline user=%s: %v", c.userID, err)
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(wsPongWait()))
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Apply limit changes without reconnect.
		c.conn.SetReadLimit(wsMaxMessageSize())

		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Errorf("ws read error user=%s: %v", c.userID, err)
			}
			return
		}

		var msg IncomingMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			logger.Errorf("ws unmarshal error user=%s: %v", c.userID, err)
			continue
		}

		c.hub.HandleMessage(ctx, c, msg)
	}
}

// writePump writes messages to the WebSocket connection.
// Exits on ctx cancellation, write error, or connection close.
func (c *Client) writePump(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	var lastPing time.Time
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			if err := c.conn.WriteMessage(websocket.CloseMessage, nil); err != nil {
				logger.Errorf("ws close message user=%s: %v", c.userID, err)
			}
			return
		case msg := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait())); err != nil {
				logger.Errorf("ws set write deadline user=%s: %v", c.userID, err)
				return
			}
			buf := bufPool.Get().(*bytes.Buffer)
			buf.Reset()
			enc := json.NewEncoder(buf)
			if err := enc.Encode(msg); err != nil {
				bufPool.Put(buf)
				logger.Errorf("ws marshal error user=%s: %v", c.userID, err)
				continue
			}
			data := buf.Bytes()
			// json.Encoder appends '\n'; trim it for WebSocket text messages.
			if len(data) > 0 && data[len(data)-1] == '\n' {
				data = data[:len(data)-1]
			}
			writeErr := c.conn.WriteMessage(websocket.TextMessage, data)
			bufPool.Put(buf)
			if writeErr != nil {
				return
			}
		case <-ticker.C:
			// Dynamic ping period derived from current pong timeout.
			pongWait := wsPongWait()
			pingPeriod := (pongWait * 9) / 10
			if !lastPing.IsZero() && time.Since(lastPing) < pingPeriod {
				continue
			}
			lastPing = time.Now()
			if err := c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait())); err != nil {
				logger.Errorf("ws set write deadline user=%s: %v", c.userID, err)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
