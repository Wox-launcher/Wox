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
	address   string
	token     string
	http      *http.Client
	nextID    atomic.Uint64
	stepDelay time.Duration
}

// WindowState is the driver's public view of one managed Wox window.
type WindowState = automation.WindowState

const (
	// SharedInfoFileEnvironment points smoke clients at the suite-owned automation endpoint.
	SharedInfoFileEnvironment = "WOX_GO_UI_AUTOMATION_INFO_FILE"
	// SharedDataDirectoryEnvironment points smoke cases at the isolated Wox data directory.
	SharedDataDirectoryEnvironment = "WOX_GO_UI_SMOKE_DATA_DIR"
	// SharedUserDataDirectoryEnvironment points smoke cases at the isolated persisted user data directory.
	SharedUserDataDirectoryEnvironment = "WOX_GO_UI_SMOKE_USER_DATA_DIR"
	// SmokeStepDelayEnvironment slows visible automation steps for interactive observation.
	SmokeStepDelayEnvironment = "WOX_GO_UI_SMOKE_STEP_DELAY"
)

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
	var stepDelay time.Duration
	if value := strings.TrimSpace(os.Getenv(SmokeStepDelayEnvironment)); value != "" {
		var err error
		stepDelay, err = time.ParseDuration(value)
		if err != nil || stepDelay < 0 {
			return nil, fmt.Errorf("invalid %s %q", SmokeStepDelayEnvironment, value)
		}
	}
	return &Client{
		address:   strings.TrimRight(info.Address, "/"),
		token:     info.Token,
		http:      &http.Client{Timeout: 35 * time.Second},
		stepDelay: stepDelay,
	}, nil
}

// pauseStep delays only user-visible automation operations when slow mode is enabled.
func (c *Client) pauseStep(ctx context.Context) error {
	if c.stepDelay <= 0 {
		return nil
	}
	timer := time.NewTimer(c.stepDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) pauseAfterStep(ctx context.Context, err error) error {
	if err != nil {
		return err
	}
	return c.pauseStep(ctx)
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

// FrameMetrics returns correlated Host and native frame timings for the active window.
func (c *Client) FrameMetrics(ctx context.Context) (woxui.FrameMetricsSnapshot, error) {
	return call[woxui.FrameMetricsSnapshot](ctx, c, "render.metrics", nil)
}

// ResetFrameMetrics starts a fresh frame measurement interval for the active window.
func (c *Client) ResetFrameMetrics(ctx context.Context) error {
	_, err := call[bool](ctx, c, "render.metrics.reset", nil)
	return err
}

// SetRepaintDebugMode changes incremental-rendering diagnostics in the active surface.
func (c *Client) SetRepaintDebugMode(ctx context.Context, mode woxwidget.RepaintDebugMode) error {
	_, err := call[bool](ctx, c, "render.repaint_debug", map[string]any{"mode": mode})
	return err
}

// SimulateRendererDeviceRemoved makes the active Windows renderer report device loss on its next frame.
func (c *Client) SimulateRendererDeviceRemoved(ctx context.Context) error {
	_, err := call[bool](ctx, c, "render.simulate_device_removed", nil)
	return c.pauseAfterStep(ctx, err)
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

// FindByAutomationIDPrefix returns the first semantics node with the requested dynamic ID prefix.
func FindByAutomationIDPrefix(snapshot woxwidget.AutomationSnapshot, prefix string) (woxui.AccessibilityNode, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, prefix) {
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
	return c.pauseAfterStep(ctx, err)
}

// Pointer sends one logical pointer event to the active widget host.
func (c *Client) Pointer(ctx context.Context, event woxui.PointerEvent) error {
	_, err := call[bool](ctx, c, "input.pointer", event)
	return c.pauseAfterStep(ctx, err)
}

// MovePointer moves the logical pointer without changing button state.
func (c *Client) MovePointer(ctx context.Context, position woxui.Point) error {
	return c.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerMove, Position: position})
}

// MovePointerTo centers the logical pointer on a semantics node.
func (c *Client) MovePointerTo(ctx context.Context, automationID string) (woxui.AccessibilityNode, error) {
	snapshot, err := c.Snapshot(ctx)
	if err != nil {
		return woxui.AccessibilityNode{}, err
	}
	node, found := Find(snapshot, automationID)
	if !found {
		return woxui.AccessibilityNode{}, fmt.Errorf("automation node %q was not found", automationID)
	}
	position := woxui.Point{X: node.Bounds.X + node.Bounds.Width/2, Y: node.Bounds.Y + node.Bounds.Height/2}
	return node, c.MovePointer(ctx, position)
}

// LeavePointer clears hover through the same pointer-leave path as a native window.
func (c *Client) LeavePointer(ctx context.Context) error {
	return c.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerLeave})
}

// PressKey sends one complete semantic key press.
func (c *Client) PressKey(ctx context.Context, key woxui.Key, modifiers woxui.KeyModifiers) error {
	_, err := call[bool](ctx, c, "input.key", map[string]any{"key": key, "modifiers": modifiers})
	return c.pauseAfterStep(ctx, err)
}

// EnterText commits UTF-8 text through the focused editor.
func (c *Client) EnterText(ctx context.Context, text string) error {
	_, err := call[bool](ctx, c, "input.text", map[string]string{"text": text})
	return c.pauseAfterStep(ctx, err)
}

// Reset returns the shared smoke process to its hidden launcher baseline.
func (c *Client) Reset(ctx context.Context) error {
	_, err := call[bool](ctx, c, "suite.reset", nil)
	return err
}

// Show opens the launcher through its product lifecycle.
func (c *Client) Show(ctx context.Context) error {
	_, err := call[bool](ctx, c, "window.show", nil)
	return c.pauseAfterStep(ctx, err)
}

// OpenSelectionQuery opens a real selection-query secondary after the OS selection-capture boundary.
func (c *Client) OpenSelectionQuery(ctx context.Context, text string) error {
	_, err := call[bool](ctx, c, "window.open_selection_query", map[string]string{"text": text})
	return c.pauseAfterStep(ctx, err)
}

// OpenExplorerQuery opens the File Explorer Search secondary with bottom-anchored chrome.
func (c *Client) OpenExplorerQuery(ctx context.Context, query string) error {
	_, err := call[bool](ctx, c, "window.open_explorer_query", map[string]string{"query": query})
	return c.pauseAfterStep(ctx, err)
}

// FocusInstance routes later Snapshot/Bounds/Perform calls to one launcher instance.
func (c *Client) FocusInstance(ctx context.Context, instanceName string) error {
	_, err := call[bool](ctx, c, "window.focus_instance", map[string]string{"instanceName": instanceName})
	return err
}

// OpenSettings opens one settings route through the product window lifecycle.
func (c *Client) OpenSettings(ctx context.Context, path string) error {
	_, err := call[bool](ctx, c, "window.open_settings", map[string]string{"path": path})
	return c.pauseAfterStep(ctx, err)
}

// Hide closes the launcher through its product lifecycle.
func (c *Client) Hide(ctx context.Context) error {
	if err := c.pauseStep(ctx); err != nil {
		return err
	}
	_, err := call[bool](ctx, c, "window.hide", nil)
	return err
}

// WindowState returns the current managed lifecycle state for one launcher instance.
func (c *Client) WindowState(ctx context.Context, instanceName string) (automation.WindowState, error) {
	return call[automation.WindowState](ctx, c, "window.state", map[string]string{"instanceName": instanceName})
}

// WaitForWindowState polls real managed-window state until predicate accepts it.
func (c *Client) WaitForWindowState(ctx context.Context, instanceName string, predicate func(automation.WindowState) bool) (automation.WindowState, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last automation.WindowState
	for {
		state, err := c.WindowState(ctx, instanceName)
		if err != nil {
			return last, err
		}
		last = state
		if predicate(state) {
			return state, nil
		}
		select {
		case <-ctx.Done():
			return last, fmt.Errorf("wait for window %q state: %w; last state: %+v", instanceName, ctx.Err(), last)
		case <-ticker.C:
		}
	}
}

// Bounds returns logical native window geometry.
func (c *Client) Bounds(ctx context.Context) (woxui.Rect, error) {
	return call[woxui.Rect](ctx, c, "window.bounds", nil)
}

// SetBounds updates logical native window geometry.
func (c *Client) SetBounds(ctx context.Context, bounds woxui.Rect) error {
	_, err := call[bool](ctx, c, "window.set_bounds", bounds)
	return c.pauseAfterStep(ctx, err)
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
