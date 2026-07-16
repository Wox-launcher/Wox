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

// Controller exposes product behavior to the test-only automation transport.
type Controller interface {
	AutomationSnapshot() woxwidget.AutomationSnapshot
	WaitForAutomationChange(ctx context.Context, afterGeneration uint64) (woxwidget.AutomationSnapshot, error)
	PerformAutomationAction(automationID string, action woxui.AccessibilityAction, value string) error
	PressAutomationKey(key woxui.Key, modifiers woxui.KeyModifiers) error
	EnterAutomationText(text string) error
	ShowAutomationWindow() error
	HideAutomationWindow() error
	AutomationWindowBounds() (woxui.Rect, error)
	SetAutomationWindowBounds(bounds woxui.Rect) error
	CaptureAutomationWindow(path string) error
}
