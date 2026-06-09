package wecom

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wecomWSSURL = "wss://openws.work.weixin.qq.com"
	pingInterval = 30 * time.Second
)

// Message types from WeCom
const (
	MsgTypeText  = "text"
	MsgTypeImage = "image"
	MsgTypeEvent = "event"
)

// Event types
const (
	EventEnterChat = "enter_chat"
)

// WsFrame is the top-level WebSocket message frame.
type WsFrame struct {
	ID          string          `json:"id,omitempty"`
	Type        string          `json:"type"`
	MsgType     string          `json:"msgtype,omitempty"`
	Content     json.RawMessage `json:"content,omitempty"`
	Text        *TextContent    `json:"text,omitempty"`
	From        *FromInfo       `json:"from,omitempty"`
	ChatID      string          `json:"chatid,omitempty"`
	ChatType    string          `json:"chattype,omitempty"`
	EventType   string          `json:"event_type,omitempty"`
	StreamID    string          `json:"stream_id,omitempty"`
	Seq         int64           `json:"seq,omitempty"`
	Success     bool            `json:"success,omitempty"`
	Description string          `json:"description,omitempty"`
}

// TextContent represents the text content of a message.
type TextContent struct {
	Content string `json:"content"`
}

// FromInfo identifies the sender.
type FromInfo struct {
	UserID string `json:"userid"`
	Name   string `json:"name,omitempty"`
}

// TextMessage is the parsed text message body.
type TextMessage struct {
	Content string   `json:"content"`
	From    FromInfo `json:"from"`
	ChatID  string   `json:"chatid"`
	ChatType string `json:"chattype"`
}

// ReplyStreamBody is sent to reply to a message using streaming.
type ReplyStreamBody struct {
	StreamID string `json:"stream_id"`
	Content  string `json:"content"`
	IsEnd    bool   `json:"is_end"`
	MsgType  string `json:"msgtype"`
}

// aibotSubscribe is the authentication message sent on connect.
type aibotSubscribe struct {
	Type      string `json:"type"`
	BotID     string `json:"bot_id"`
	BotSecret string `json:"bot_secret"`
}

// Client handles the WebSocket connection to WeCom.
type Client struct {
	conn      *websocket.Conn
	botID     string
	botSecret string
	mu        sync.Mutex

	// Callbacks
	onTextMessage  func(*TextMessage)
	onEvent        func(string, json.RawMessage)
	onConnected    func()
	onDisconnected func(error)

	// Control
	stopCh     chan struct{}
	writeCh    chan []byte
	seq        int64
}

// NewClient creates a new WeCom WebSocket client.
func NewClient(botID, botSecret string) *Client {
	return &Client{
		botID:     botID,
		botSecret: botSecret,
		stopCh:    make(chan struct{}),
		writeCh:   make(chan []byte, 64),
	}
}

// OnTextMessage registers a handler for text messages.
func (c *Client) OnTextMessage(handler func(*TextMessage)) {
	c.onTextMessage = handler
}

// OnEvent registers a handler for events.
func (c *Client) OnEvent(handler func(string, json.RawMessage)) {
	c.onEvent = handler
}

// OnConnected registers a handler for successful connection.
func (c *Client) OnConnected(handler func()) {
	c.onConnected = handler
}

// OnDisconnected registers a handler for disconnection.
func (c *Client) OnDisconnected(handler func(error)) {
	c.onDisconnected = handler
}

// Connect establishes the WebSocket connection to WeCom and starts
// reading messages. It blocks until the connection is established.
func (c *Client) Connect() error {
	log.Printf("[wecom] connecting to %s ...", wecomWSSURL)

	conn, _, err := websocket.DefaultDialer.Dial(wecomWSSURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to wecom: %w", err)
	}
	c.conn = conn

	// Send authentication
	auth := aibotSubscribe{
		Type:      "aibot_subscribe",
		BotID:     c.botID,
		BotSecret: c.botSecret,
	}
	if err := c.writeJSON(auth); err != nil {
		conn.Close()
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	log.Println("[wecom] authentication sent, waiting for response...")

	// Wait for auth response
	var authResp WsFrame
	if err := conn.ReadJSON(&authResp); err != nil {
		conn.Close()
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	if authResp.Type == "error" {
		conn.Close()
		return fmt.Errorf("wecom auth error: %s (description: %s)",
			authResp.Type, authResp.Description)
	}

	log.Printf("[wecom] connected and authenticated successfully (type=%s)", authResp.Type)

	if c.onConnected != nil {
		c.onConnected()
	}

	// Start write pump (for heartbeat and outgoing messages)
	go c.writePump()

	// Start read pump (main message processing)
	go c.readPump()

	return nil
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.reconnect()
	}()

	for {
		var frame WsFrame
		err := c.conn.ReadJSON(&frame)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[wecom] websocket error: %v", err)
			}
			if c.onDisconnected != nil {
				c.onDisconnected(err)
			}
			return
		}

		c.handleFrame(&frame)
	}
}

// handleFrame routes incoming frames to the appropriate handler.
func (c *Client) handleFrame(frame *WsFrame) {
	switch frame.Type {
	case "aibot_msg_callback":
		c.handleMessage(frame)
	case "pong":
		// Heartbeat response — nothing to do
	case "ack":
		// Message delivery acknowledgment
	default:
		log.Printf("[wecom] unhandled frame type: %s", frame.Type)
	}
}

// handleMessage processes incoming message callbacks.
func (c *Client) handleMessage(frame *WsFrame) {
	switch frame.MsgType {
	case MsgTypeText:
		if c.onTextMessage == nil {
			return
		}
		msg := &TextMessage{
			Content:  frame.Text.Content,
			ChatID:   frame.ChatID,
			ChatType: frame.ChatType,
		}
		if frame.From != nil {
			msg.From = *frame.From
		}
		c.onTextMessage(msg)

	case MsgTypeEvent:
		if c.onEvent != nil {
			c.onEvent(frame.EventType, frame.Content)
		}

	default:
		log.Printf("[wecom] unhandled message type: %s", frame.MsgType)
	}
}

// ReplyText sends a plain text reply to a user.
func (c *Client) ReplyText(text string) error {
	return c.sendMsg("text", map[string]string{"content": text})
}

// ReplyStream sends a streaming reply (typing indicator then final answer).
func (c *Client) ReplyStream(streamID, content string, isEnd bool) error {
	body := ReplyStreamBody{
		StreamID: streamID,
		Content:  content,
		IsEnd:    isEnd,
		MsgType:  "stream",
	}
	return c.writeJSON(map[string]interface{}{
		"type":   "aibot_send_msg",
		"stream": body,
	})
}

// sendMsg sends a message to WeCom.
func (c *Client) sendMsg(msgType string, content interface{}) error {
	msg := map[string]interface{}{
		"type":    "aibot_send_msg",
		"msgtype": msgType,
		msgType:   content,
	}
	return c.writeJSON(msg)
}

// writeJSON sends a JSON message over the WebSocket.
func (c *Client) writeJSON(data interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}
	return c.conn.WriteJSON(data)
}

// writePump handles outgoing messages and heartbeat.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send ping
			c.mu.Lock()
			if c.conn != nil {
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("[wecom] ping error: %v", err)
				}
			}
			c.mu.Unlock()

		case msg := <-c.writeCh:
			c.mu.Lock()
			if c.conn != nil {
				c.conn.WriteMessage(websocket.TextMessage, msg)
			}
			c.mu.Unlock()

		case <-c.stopCh:
			return
		}
	}
}

// reconnect closes the current connection and attempts to reconnect.
func (c *Client) reconnect() {
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	if c.onDisconnected != nil {
		c.onDisconnected(fmt.Errorf("connection lost, reconnecting..."))
	}

	// Exponential backoff reconnection
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-c.stopCh:
			return
		case <-time.After(backoff):
			log.Printf("[wecom] reconnecting (backoff=%v)...", backoff)
			if err := c.Connect(); err != nil {
				log.Printf("[wecom] reconnect failed: %v", err)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			return
		}
	}
}

// Disconnect gracefully closes the WebSocket connection.
func (c *Client) Disconnect() {
	close(c.stopCh)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(
			websocket.CloseNormalClosure, "shutting down",
		))
		c.conn.Close()
		c.conn = nil
	}
}
