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
	// SharedLifecyclePhaseEnvironment selects one runner-owned restart phase.
	SharedLifecyclePhaseEnvironment = "WOX_GO_UI_SMOKE_LIFECYCLE_PHASE"
	// SharedLifecycleStateEnvironment points lifecycle phases at state outside private-mode cleanup.
	SharedLifecycleStateEnvironment = "WOX_GO_UI_SMOKE_LIFECYCLE_STATE"
)

// ActionTimeout is the longest one smoke wait may block. A longer hang is a product bug.
const ActionTimeout = 10 * time.Second

// withActionTimeout caps one wait so a stuck predicate fails in 10s instead of the case budget.
func withActionTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && !deadline.After(time.Now().Add(ActionTimeout)) {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, ActionTimeout)
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
		http:      &http.Client{Timeout: ActionTimeout + time.Second},
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

// RequestFrame invalidates the active surface so performance tests can sample a settled UI.
func (c *Client) RequestFrame(ctx context.Context) error {
	_, err := call[bool](ctx, c, "render.invalidate", nil)
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

// InstallPerfFixture installs a deterministic wox_automation performance fixture.
func (c *Client) InstallPerfFixture(ctx context.Context, name string) error {
	_, err := call[bool](ctx, c, "perf.install_fixture", map[string]any{"name": name})
	return c.pauseAfterStep(ctx, err)
}

// WaitForChange waits for a generation newer than afterGeneration.
func (c *Client) WaitForChange(ctx context.Context, afterGeneration uint64) (woxwidget.AutomationSnapshot, error) {
	deadline, hasDeadline := ctx.Deadline()
	timeoutMS := int(ActionTimeout.Milliseconds())
	if hasDeadline {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return woxwidget.AutomationSnapshot{}, context.DeadlineExceeded
		}
		timeoutMS = min(timeoutMS, max(1, int(remaining.Milliseconds())))
	}
	return call[woxwidget.AutomationSnapshot](ctx, c, "semantics.wait", map[string]any{
		"afterGeneration": afterGeneration,
		"timeoutMs":       timeoutMS,
	})
}

// WaitFor polls the active surface until predicate succeeds.
func (c *Client) WaitFor(ctx context.Context, predicate func(woxwidget.AutomationSnapshot) bool) (woxwidget.AutomationSnapshot, error) {
	return c.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		return predicate(snapshot), ""
	})
}

// WaitForReason polls like WaitFor but lets the predicate describe the state it
// rejected. Timeouts then name the unmet condition, because a bare deadline
// message looks identical whether the wait was stuck on a label, a value, or
// semantics diagnostics.
func (c *Client) WaitForReason(ctx context.Context, predicate func(woxwidget.AutomationSnapshot) (bool, string)) (woxwidget.AutomationSnapshot, error) {
	ctx, cancel := withActionTimeout(ctx)
	defer cancel()
	snapshot, err := c.Snapshot(ctx)
	if err != nil {
		return woxwidget.AutomationSnapshot{}, err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		satisfied, reason := predicate(snapshot)
		if satisfied {
			return snapshot, nil
		}
		select {
		case <-ctx.Done():
			return snapshot, waitTimeoutError(snapshot, reason, ctx.Err())
		case <-ticker.C:
		}
		next, err := c.Snapshot(ctx)
		if err != nil {
			// The polling snapshot shares the wait deadline, so a request cut off
			// at that deadline is still an unmet predicate. Reporting it as a
			// transport error made every stuck wait look like a dead endpoint.
			if ctx.Err() != nil {
				return snapshot, waitTimeoutError(snapshot, reason, ctx.Err())
			}
			return snapshot, fmt.Errorf("refresh semantics after generation %d: %w", snapshot.Tree.Generation, err)
		}
		snapshot = next
	}
}

// waitSnapshotIDBudget caps the automation IDs one timeout prints. A settings page
// exposes far more nodes than a launcher surface, so the budget is generous enough
// to reach list and table rows, which are usually the nodes a wait is missing.
const waitSnapshotIDBudget = 120

// waitTimeoutError explains a stuck wait with the rejected state and any
// semantics diagnostics, which are a common reason a predicate never passes.
func waitTimeoutError(snapshot woxwidget.AutomationSnapshot, reason string, cause error) error {
	detail := fmt.Sprintf("wait for semantics after generation %d", snapshot.Tree.Generation)
	if reason != "" {
		detail += ": " + reason
	}
	if len(snapshot.Diagnostics) > 0 {
		detail += fmt.Sprintf(": diagnostics %q", snapshot.Diagnostics)
	}
	// Every timeout carries the observed surface, not just predicates that describe
	// what they rejected. A plain WaitFor cannot explain itself, and identifying the
	// missing node used to require downloading the whole smoke suite directory from CI.
	detail += ": observed " + DescribeSnapshot(snapshot)
	return fmt.Errorf("%s: %w", detail, cause)
}

// DescribeSnapshot summarizes the surface a wait observed. Most stuck waits are
// waiting for a node that never appeared, so the focused node and the list of
// present automation IDs are what identify the actual UI state.
func DescribeSnapshot(snapshot woxwidget.AutomationSnapshot) string {
	focused := "none"
	automationIDs := make([]string, 0, len(snapshot.Tree.Nodes))
	for _, node := range snapshot.Tree.Nodes {
		if node.Focused {
			focused = fmt.Sprintf("%q", node.AutomationID)
		}
		if node.AutomationID != "" {
			automationIDs = append(automationIDs, node.AutomationID)
		}
	}
	summary := fmt.Sprintf("focus=%s nodes=%d", focused, len(snapshot.Tree.Nodes))
	if len(automationIDs) > waitSnapshotIDBudget {
		return fmt.Sprintf("%s ids=%v (+%d more)", summary, automationIDs[:waitSnapshotIDBudget], len(automationIDs)-waitSnapshotIDBudget)
	}
	return fmt.Sprintf("%s ids=%v", summary, automationIDs)
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

// DescribeNodes renders the label and value of the requested nodes so a wait
// timeout can report the state it observed instead of only the deadline.
func DescribeNodes(snapshot woxwidget.AutomationSnapshot, automationIDs ...string) string {
	descriptions := make([]string, 0, len(automationIDs))
	for _, automationID := range automationIDs {
		node, found := Find(snapshot, automationID)
		if !found {
			descriptions = append(descriptions, fmt.Sprintf("%s missing", automationID))
			continue
		}
		descriptions = append(descriptions, fmt.Sprintf("%s label=%q value=%q", automationID, node.Label, node.Value))
	}
	return strings.Join(descriptions, ", ")
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
	_, err := c.PressKeyHandled(ctx, key, modifiers)
	return err
}

// PressKeyHandled sends one complete semantic key press and reports whether the product handled it.
func (c *Client) PressKeyHandled(ctx context.Context, key woxui.Key, modifiers woxui.KeyModifiers) (bool, error) {
	handled, err := call[bool](ctx, c, "input.key", map[string]any{"key": key, "modifiers": modifiers})
	if pauseErr := c.pauseAfterStep(ctx, err); pauseErr != nil {
		return false, pauseErr
	}
	return handled, nil
}

// SendKey sends one key-down or key-up through the normal product key path.
func (c *Client) SendKey(ctx context.Context, key woxui.Key, modifiers woxui.KeyModifiers, down bool) error {
	_, err := c.SendKeyHandled(ctx, key, modifiers, down)
	return err
}

// SendKeyHandled sends one key-down or key-up and reports whether the product handled it.
func (c *Client) SendKeyHandled(ctx context.Context, key woxui.Key, modifiers woxui.KeyModifiers, down bool) (bool, error) {
	handled, err := call[bool](ctx, c, "input.key_event", map[string]any{"key": key, "modifiers": modifiers, "down": down})
	if pauseErr := c.pauseAfterStep(ctx, err); pauseErr != nil {
		return false, pauseErr
	}
	return handled, nil
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
	ctx, cancel := withActionTimeout(ctx)
	defer cancel()
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
	ctx, cancel := withActionTimeout(ctx)
	defer cancel()
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
