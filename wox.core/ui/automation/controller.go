package automation

import (
	"context"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Info describes a running test-only automation endpoint.
type Info struct {
	Address string `json:"address"`
	Token   string `json:"token"`
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
	SetAutomationRepaintDebugMode(mode woxwidget.RepaintDebugMode) error
	WaitForAutomationChange(ctx context.Context, afterGeneration uint64) (woxwidget.AutomationSnapshot, error)
	PerformAutomationAction(automationID string, action woxui.AccessibilityAction, value string) error
	DispatchAutomationPointer(event woxui.PointerEvent) error
	PressAutomationKey(key woxui.Key, modifiers woxui.KeyModifiers) error
	EnterAutomationText(text string) error
	ResetAutomationState() error
	ShowAutomationWindow() error
	OpenAutomationSelectionQuery(text string) error
	OpenAutomationExplorerQuery(query string) error
	SetAutomationFocusInstance(instanceName string) error
	OpenAutomationSettings(path string) error
	HideAutomationWindow() error
	AutomationWindowState(instanceName string) (WindowState, error)
	AutomationWindowBounds() (woxui.Rect, error)
	SetAutomationWindowBounds(bounds woxui.Rect) error
	CaptureAutomationWindow(path string) error
}
