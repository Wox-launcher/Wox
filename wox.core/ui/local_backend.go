package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"wox/util"

	"github.com/Wox-launcher/wox.ui.go/coreclient"
)

type localUISink interface {
	deliverRequest(message UIMessage) error
	deliverResponse(message UIMessage) error
}

var localUISinkState struct {
	sync.RWMutex
	sink localUISink
}

// LocalBackendFactory creates the launcher-facing backend for the embedded Go UI.
func LocalBackendFactory(sessionID string, onRequest coreclient.RequestHandler, onResponse coreclient.ResponseHandler) coreclient.Backend {
	return &localBackend{
		sessionID:  sessionID,
		onRequest:  onRequest,
		onResponse: onResponse,
		handler:    newRouterMux(context.Background()),
		messages:   make(chan coreclient.Message, 64),
		done:       make(chan struct{}),
	}
}

type localBackend struct {
	sessionID  string
	onRequest  coreclient.RequestHandler
	onResponse coreclient.ResponseHandler
	handler    http.Handler
	messages   chan coreclient.Message
	done       chan struct{}

	mu        sync.Mutex
	connected bool
	closed    bool
	closeOnce sync.Once
}

// Connect registers this launcher as core's active in-process UI sink.
func (b *localBackend) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("local core backend is closed")
	}
	if b.connected {
		return nil
	}
	localUISinkState.Lock()
	defer localUISinkState.Unlock()
	if localUISinkState.sink != nil {
		return errors.New("an in-process UI is already connected")
	}
	localUISinkState.sink = b
	b.connected = true
	go b.readLoop()
	return nil
}

func (b *localBackend) SendRequest(method string, data any) (string, error) {
	requestID := coreclient.NewID()
	return requestID, b.SendRequestWithID(requestID, method, data)
}

func (b *localBackend) SendRequestWithID(requestID string, method string, data any) error {
	if err := b.ensureConnected(); err != nil {
		return err
	}
	ctx := util.WithSessionContext(util.NewTraceContext(), b.sessionID)
	request := UIMessage{
		RequestId:     requestID,
		TraceId:       util.GetContextTraceId(ctx),
		SessionId:     b.sessionID,
		Method:        method,
		Success:       true,
		Data:          data,
		SendTimestamp: util.GetSystemTimestamp(),
	}
	util.Go(ctx, "handle in-process UI request", func() {
		onUIRequest(ctx, request)
	})
	return nil
}

func (b *localBackend) Post(ctx context.Context, path string, data any, target any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return b.doJSON(ctx, http.MethodPost, path, bytes.NewReader(body), target)
}

func (b *localBackend) Get(ctx context.Context, path string, target any) error {
	return b.doJSON(ctx, http.MethodGet, path, nil, target)
}

func (b *localBackend) doJSON(ctx context.Context, method string, path string, body *bytes.Reader, target any) error {
	if err := b.ensureConnected(); err != nil {
		return err
	}
	var request *http.Request
	var err error
	if body == nil {
		request, err = http.NewRequestWithContext(ctx, method, "http://wox.local"+path, nil)
	} else {
		request, err = http.NewRequestWithContext(ctx, method, "http://wox.local"+path, body)
	}
	if err != nil {
		return err
	}
	request.Header.Set("TraceId", coreclient.NewID())
	request.Header.Set("SessionId", b.sessionID)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	b.handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("core API %s returned %s", path, response.Status)
	}
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

func (b *localBackend) Close() error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		b.connected = false
		b.mu.Unlock()
		localUISinkState.Lock()
		if localUISinkState.sink == b {
			localUISinkState.sink = nil
		}
		localUISinkState.Unlock()
		close(b.done)
	})
	return nil
}

func (b *localBackend) deliverRequest(message UIMessage) error {
	return b.deliver(message, coreclient.MessageRequest)
}

func (b *localBackend) deliverResponse(message UIMessage) error {
	return b.deliver(message, coreclient.MessageResponse)
}

func (b *localBackend) deliver(message UIMessage, messageType string) error {
	data, err := json.Marshal(message.Data)
	if err != nil {
		return err
	}
	if message.SessionId != "" && message.SessionId != b.sessionID && !strings.HasPrefix(message.SessionId, "core-") {
		return nil
	}
	incoming := coreclient.Message{
		RequestID:     message.RequestId,
		TraceID:       message.TraceId,
		SessionID:     message.SessionId,
		Type:          messageType,
		Method:        message.Method,
		Success:       message.Success,
		Data:          data,
		SendTimestamp: message.SendTimestamp,
	}
	select {
	case <-b.done:
		return errors.New("local core backend is closed")
	case b.messages <- incoming:
		return nil
	}
}

func (b *localBackend) readLoop() {
	for {
		select {
		case <-b.done:
			return
		case message := <-b.messages:
			switch message.Type {
			case coreclient.MessageRequest:
				b.handleRequest(message)
			case coreclient.MessageResponse:
				if b.onResponse != nil {
					b.onResponse(message)
				}
			}
		}
	}
}

func (b *localBackend) handleRequest(message coreclient.Message) {
	var data any
	var err error
	if b.onRequest == nil {
		err = fmt.Errorf("unsupported UI method: %s", message.Method)
	} else {
		data, err = b.onRequest(message)
	}
	response := UIMessage{
		RequestId: message.RequestID,
		TraceId:   message.TraceID,
		SessionId: message.SessionID,
		Method:    message.Method,
		Success:   err == nil,
		Data:      data,
	}
	if err != nil {
		response.Data = err.Error()
	}
	ctx := util.WithSessionContext(util.NewTraceContextWith(message.TraceID), message.SessionID)
	util.Go(ctx, "handle in-process UI response", func() {
		onUIResponse(ctx, response)
	})
}

func (b *localBackend) ensureConnected() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("local core backend is closed")
	}
	if !b.connected {
		return errors.New("local core backend is not connected")
	}
	return nil
}

func getLocalUISink() localUISink {
	localUISinkState.RLock()
	defer localUISinkState.RUnlock()
	return localUISinkState.sink
}
