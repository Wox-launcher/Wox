package automationdriver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"wox/ui/automation"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Client drives one authenticated wox_automation process.
type Client struct {
	address string
	token   string
	http    *http.Client
	nextID  atomic.Uint64
}

type request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type response struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// NewClient creates a driver for automation endpoint metadata emitted by Wox.
func NewClient(info automation.Info) (*Client, error) {
	if strings.TrimSpace(info.Address) == "" || strings.TrimSpace(info.Token) == "" {
		return nil, errors.New("automation address and token are required")
	}
	return &Client{
		address: strings.TrimRight(info.Address, "/"),
		token:   info.Token,
		http:    &http.Client{Timeout: 35 * time.Second},
	}, nil
}

// ReadInfo waits for Wox to atomically publish its automation endpoint metadata.
func ReadInfo(ctx context.Context, path string) (automation.Info, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var info automation.Info
			if decodeErr := json.Unmarshal(data, &info); decodeErr == nil && info.Address != "" && info.Token != "" {
				return info, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return automation.Info{}, err
		}
		select {
		case <-ctx.Done():
			return automation.Info{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

// Snapshot returns the latest retained semantics tree.
func (c *Client) Snapshot(ctx context.Context) (woxwidget.AutomationSnapshot, error) {
	return call[woxwidget.AutomationSnapshot](ctx, c, "semantics.snapshot", nil)
}

// WaitForChange waits for a generation newer than afterGeneration.
func (c *Client) WaitForChange(ctx context.Context, afterGeneration uint64) (woxwidget.AutomationSnapshot, error) {
	deadline, hasDeadline := ctx.Deadline()
	timeoutMS := 5000
	if hasDeadline {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return woxwidget.AutomationSnapshot{}, context.DeadlineExceeded
		}
		timeoutMS = min(30000, max(1, int(remaining.Milliseconds())))
	}
	return call[woxwidget.AutomationSnapshot](ctx, c, "semantics.wait", map[string]any{
		"afterGeneration": afterGeneration,
		"timeoutMs":       timeoutMS,
	})
}

// WaitFor polls only after a published generation change until predicate succeeds.
func (c *Client) WaitFor(ctx context.Context, predicate func(woxwidget.AutomationSnapshot) bool) (woxwidget.AutomationSnapshot, error) {
	snapshot, err := c.Snapshot(ctx)
	if err != nil {
		return woxwidget.AutomationSnapshot{}, err
	}
	for {
		if predicate(snapshot) {
			return snapshot, nil
		}
		snapshot, err = c.WaitForChange(ctx, snapshot.Tree.Generation)
		if err != nil {
			return woxwidget.AutomationSnapshot{}, err
		}
	}
}

// Find returns the semantics node with the requested stable automation ID.
func Find(snapshot woxwidget.AutomationSnapshot, automationID string) (woxui.AccessibilityNode, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if node.AutomationID == automationID {
			return node, true
		}
	}
	return woxui.AccessibilityNode{}, false
}

// Perform invokes one action on a semantics node.
func (c *Client) Perform(ctx context.Context, automationID string, action woxui.AccessibilityAction, value string) error {
	_, err := call[bool](ctx, c, "semantics.perform", map[string]any{
		"automationId": automationID,
		"action":       action,
		"value":        value,
	})
	return err
}

// PressKey sends one complete semantic key press.
func (c *Client) PressKey(ctx context.Context, key woxui.Key, modifiers woxui.KeyModifiers) error {
	_, err := call[bool](ctx, c, "input.key", map[string]any{"key": key, "modifiers": modifiers})
	return err
}

// EnterText commits UTF-8 text through the focused editor.
func (c *Client) EnterText(ctx context.Context, text string) error {
	_, err := call[bool](ctx, c, "input.text", map[string]string{"text": text})
	return err
}

// Show opens the launcher through its product lifecycle.
func (c *Client) Show(ctx context.Context) error {
	_, err := call[bool](ctx, c, "window.show", nil)
	return err
}

// Hide closes the launcher through its product lifecycle.
func (c *Client) Hide(ctx context.Context) error {
	_, err := call[bool](ctx, c, "window.hide", nil)
	return err
}

// Bounds returns logical native window geometry.
func (c *Client) Bounds(ctx context.Context) (woxui.Rect, error) {
	return call[woxui.Rect](ctx, c, "window.bounds", nil)
}

// SetBounds updates logical native window geometry.
func (c *Client) SetBounds(ctx context.Context, bounds woxui.Rect) error {
	_, err := call[bool](ctx, c, "window.set_bounds", bounds)
	return err
}

// Capture writes the current native window pixels to an absolute PNG path in the Wox process.
func (c *Client) Capture(ctx context.Context, path string) error {
	_, err := call[bool](ctx, c, "window.capture", map[string]string{"path": path})
	return err
}

func call[T any](ctx context.Context, client *Client, method string, params any) (T, error) {
	var result T
	payload, err := json.Marshal(request{JSONRPC: "2.0", ID: client.nextID.Add(1), Method: method, Params: params})
	if err != nil {
		return result, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.address, bytes.NewReader(payload))
	if err != nil {
		return result, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+client.token)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, err := client.http.Do(httpRequest)
	if err != nil {
		return result, err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return result, fmt.Errorf("automation server returned %s", httpResponse.Status)
	}
	var envelope response
	if err := json.NewDecoder(httpResponse.Body).Decode(&envelope); err != nil {
		return result, err
	}
	if envelope.Error != nil {
		return result, fmt.Errorf("automation RPC %s failed (%d): %s", method, envelope.Error.Code, envelope.Error.Message)
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return result, nil
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return result, err
	}
	return result, nil
}
