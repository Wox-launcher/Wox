package automation

import (
	"context"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Info describes a running test-only automation endpoint.
type Info struct {
	Address string `json:"address"`
	Token   string `json:"token"`
}

// QueryState correlates current launcher results with the retained semantics frame.
// Visible and Complete never describe rows retained from an older query.
type QueryState struct {
	QueryID     string `json:"queryId"`
	QueryText   string `json:"queryText"`
	ResultCount int    `json:"resultCount"`
	Visible     bool   `json:"visible"`
	Complete    bool   `json:"complete"`
}

// RuntimeStats captures Go ownership without forcing GC or showing the launcher.
type RuntimeStats struct {
	PID            int    `json:"pid"`
	GOMAXPROCS     int    `json:"gomaxprocs"`
	HeapAlloc      uint64 `json:"heapAlloc"`
	Retained       uint64 `json:"retained"`
	StackInuse     uint64 `json:"stackInuse"`
	MemProfileRate int    `json:"memProfileRate"`
}

// WindowState describes one real managed window exposed to lifecycle smoke tests.
type WindowState struct {
	Exists    bool   `json:"exists"`
	Visible   bool   `json:"visible"`
	BlurReady bool   `json:"blurReady"`
	Lifecycle string `json:"lifecycle"`
}

// Controller exposes product behavior to the test-only automation transport.
type Controller interface {
	AutomationSnapshot() woxwidget.AutomationSnapshot
	AutomationFrameMetrics() (woxui.FrameMetricsSnapshot, error)
	ResetAutomationFrameMetrics() error
	RequestAutomationFrame() error
	SetAutomationRepaintDebugMode(mode woxwidget.RepaintDebugMode) error
	SetAutomationCaretBlinkInterval(interval time.Duration) error
	WaitForAutomationChange(ctx context.Context, afterGeneration uint64) (woxwidget.AutomationSnapshot, error)
	PerformAutomationAction(automationID string, action woxui.AccessibilityAction, value string) error
	DispatchAutomationPointer(event woxui.PointerEvent) error
	PressAutomationKey(key woxui.Key, modifiers woxui.KeyModifiers) (bool, error)
	DispatchAutomationKey(event woxui.KeyEvent) (bool, error)
	EnterAutomationText(text string) error
	ResetAutomationState() error
	ShowAutomationWindow() error
	OpenAutomationSelectionQuery(text string) error
	OpenAutomationQuickJumpQuery(query string) error
	SetAutomationFocusInstance(instanceName string) error
	OpenAutomationSettings(path string) error
	HideAutomationWindow() error
	AutomationWindowState(instanceName string) (WindowState, error)
	AutomationWindowBounds() (woxui.Rect, error)
	SetAutomationWindowBounds(bounds woxui.Rect) error
	CaptureAutomationWindow(path string) error
}
