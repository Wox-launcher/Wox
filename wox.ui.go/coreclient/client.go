package coreclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MessageRequest  = "WebsocketMsgTypeRequest"
	MessageResponse = "WebsocketMsgTypeResponse"
)

// Message is the stable transport envelope shared by Wox core and its UI process.
type Message struct {
	RequestID     string          `json:"RequestId"`
	TraceID       string          `json:"TraceId"`
	SessionID     string          `json:"SessionId"`
	Type          string          `json:"Type"`
	Method        string          `json:"Method"`
	Success       bool            `json:"Success"`
	Data          json.RawMessage `json:"Data"`
	SendTimestamp int64           `json:"SendTimestamp"`
}

// RequestHandler applies a core-to-UI request and returns its acknowledgement payload.
type RequestHandler func(message Message) (any, error)

// ResponseHandler receives asynchronous UI-to-core responses, including query result batches.
type ResponseHandler func(message Message)

// Client connects the standalone Go UI process to Wox core's existing HTTP and WebSocket protocol.
type Client struct {
	port       int
	sessionID  string
	httpClient *http.Client

	mu         sync.Mutex
	conn       *websocket.Conn
	closed     bool
	writeMu    sync.Mutex
	onRequest  RequestHandler
	onResponse ResponseHandler
	onError    func(error)
}

// New creates a disconnected client for one UI process session.
func New(port int, sessionID string, onRequest RequestHandler, onResponse ResponseHandler, onError func(error)) *Client {
	return &Client{
		port:       port,
		sessionID:  sessionID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		onRequest:  onRequest,
		onResponse: onResponse,
		onError:    onError,
	}
}

// Connect opens the protocol stream and starts its single ordered read loop.
func (c *Client) Connect(ctx context.Context) error {
	url := fmt.Sprintf("ws://127.0.0.1:%d/ws", c.port)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("connect to Wox core: %w", err)
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = conn.Close()
		return errors.New("core client is closed")
	}
	c.conn = conn
	c.mu.Unlock()
	go c.readLoop()
	return nil
}

// SendRequest sends one UI-originated request without waiting for its response.
func (c *Client) SendRequest(method string, data any) (string, error) {
	requestID := NewID()
	return requestID, c.SendRequestWithID(requestID, method, data)
}

// SendRequestWithID sends a request whose identifier was reserved by the caller before the write.
func (c *Client) SendRequestWithID(requestID string, method string, data any) error {
	return c.write(outboundMessage{
		RequestID:     requestID,
		TraceID:       NewID(),
		SessionID:     c.sessionID,
		Type:          MessageRequest,
		Method:        method,
		Success:       true,
		Data:          data,
		SendTimestamp: time.Now().UnixMilli(),
	})
}

// Post calls one of Wox core's JSON HTTP endpoints and decodes its data field.
func (c *Client) Post(ctx context.Context, path string, data any, target any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, path, bytes.NewReader(body), target)
}

// Get calls one of Wox core's JSON HTTP endpoints and decodes its data field.
func (c *Client) Get(ctx context.Context, path string, target any) error {
	return c.doJSON(ctx, http.MethodGet, path, nil, target)
}

// doJSON applies the stable headers and response envelope shared by core HTTP endpoints.
func (c *Client) doJSON(ctx context.Context, method, path string, body io.Reader, target any) error {
	request, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d%s", c.port, path), body)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("TraceId", NewID())
	request.Header.Set("SessionId", c.sessionID)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope struct {
		Success bool            `json:"Success"`
		Message string          `json:"Message"`
		Data    json.RawMessage `json:"Data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if !envelope.Success {
		return errors.New(envelope.Message)
	}
	if target == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, target)
}

// Close stops the protocol stream. It is safe to call more than once.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

type outboundMessage struct {
	RequestID     string `json:"RequestId"`
	TraceID       string `json:"TraceId"`
	SessionID     string `json:"SessionId"`
	Type          string `json:"Type"`
	Method        string `json:"Method"`
	Success       bool   `json:"Success"`
	Data          any    `json:"Data"`
	SendTimestamp int64  `json:"SendTimestamp"`
}

func (c *Client) readLoop() {
	for {
		c.mu.Lock()
		conn := c.conn
		closed := c.closed
		c.mu.Unlock()
		if conn == nil || closed {
			return
		}
		var message Message
		if err := conn.ReadJSON(&message); err != nil {
			c.mu.Lock()
			closed = c.closed
			c.mu.Unlock()
			if !closed && c.onError != nil {
				// ponytail: Add reconnect state only after real process restarts need to preserve visible UI state.
				c.onError(fmt.Errorf("read Wox core message: %w", err))
			}
			return
		}
		if message.SessionID != "" && message.SessionID != c.sessionID && !hasCoreSessionPrefix(message.SessionID) {
			continue
		}
		switch message.Type {
		case MessageRequest:
			c.handleRequest(message)
		case MessageResponse:
			if c.onResponse != nil {
				c.onResponse(message)
			}
		}
	}
}

func (c *Client) handleRequest(message Message) {
	var data any
	var err error
	if c.onRequest == nil {
		err = fmt.Errorf("unsupported UI method: %s", message.Method)
	} else {
		data, err = c.onRequest(message)
	}
	response := outboundMessage{
		RequestID: message.RequestID,
		TraceID:   message.TraceID,
		SessionID: message.SessionID,
		Type:      MessageResponse,
		Method:    message.Method,
		Success:   err == nil,
		Data:      data,
	}
	if err != nil {
		response.Data = err.Error()
	}
	if writeErr := c.write(response); writeErr != nil && c.onError != nil {
		c.onError(writeErr)
	}
}

func (c *Client) write(message outboundMessage) error {
	c.mu.Lock()
	conn := c.conn
	closed := c.closed
	c.mu.Unlock()
	if conn == nil || closed {
		return errors.New("core client is not connected")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteJSON(message)
}

func hasCoreSessionPrefix(value string) bool {
	return len(value) >= 5 && value[:5] == "core-"
}

// NewID returns a UUID-shaped random protocol identifier without adding another dependency.
func NewID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("generate protocol id: %v", err))
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := make([]byte, 32)
	hex.Encode(encoded, value[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:])
}
