package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"wox/ui/coreclient"
)

// LocalBackendFactory creates the transitional launcher-facing core API adapter.
func LocalBackendFactory(sessionID string) coreclient.Backend {
	return &localBackend{
		sessionID: sessionID,
		handler:   newRouterMux(context.Background()),
	}
}

type localBackend struct {
	sessionID string
	handler   http.Handler

	mu        sync.Mutex
	connected bool
	closed    bool
	closeOnce sync.Once
}

// Connect enables the in-process adapter after composition has completed.
func (b *localBackend) Connect(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("local core backend is closed")
	}
	if b.connected {
		return nil
	}
	b.connected = true
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
	})
	return nil
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
