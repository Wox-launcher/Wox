package launcher

import (
	"context"
	"errors"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// AutomationSnapshot returns the latest immutable semantics tree.
func (a *App) AutomationSnapshot() woxwidget.AutomationSnapshot {
	if a.host == nil {
		return woxwidget.AutomationSnapshot{}
	}
	return a.host.Snapshot()
}

// WaitForAutomationChange waits for a newer reconciled frame.
func (a *App) WaitForAutomationChange(ctx context.Context, afterGeneration uint64) (woxwidget.AutomationSnapshot, error) {
	if a.host == nil {
		return woxwidget.AutomationSnapshot{}, errors.New("launcher widget host is not initialized")
	}
	return a.host.WaitForChange(ctx, afterGeneration)
}

// PerformAutomationAction invokes one semantics action by stable automation ID.
func (a *App) PerformAutomationAction(automationID string, action woxui.AccessibilityAction, value string) error {
	if a.host == nil {
		return errors.New("launcher widget host is not initialized")
	}
	return a.host.PerformAutomationAction(automationID, action, value)
}

// PressAutomationKey sends a complete key press through the normal widget and launcher handlers.
func (a *App) PressAutomationKey(key woxui.Key, modifiers woxui.KeyModifiers) error {
	if a.host == nil {
		return errors.New("launcher widget host is not initialized")
	}
	return woxui.Call(func() {
		down := woxui.KeyEvent{Key: key, Modifiers: modifiers, Down: true}
		if !a.host.Key(down) {
			a.onKey(down)
		}
		up := woxui.KeyEvent{Key: key, Modifiers: modifiers}
		if !a.host.Key(up) {
			a.onKey(up)
		}
	})
}

// EnterAutomationText commits UTF-8 text through the active text-input owner.
func (a *App) EnterAutomationText(text string) error {
	if a.host == nil {
		return errors.New("launcher widget host is not initialized")
	}
	return woxui.Call(func() {
		event := woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: text}
		if !a.host.TextInput(event) {
			a.onTextInput(event)
		}
	})
}

// ShowAutomationWindow opens the launcher through its normal product path.
func (a *App) ShowAutomationWindow() error {
	if a.window == nil {
		return errors.New("launcher window is not initialized")
	}
	var actionErr error
	err := woxui.Call(func() {
		a.mu.RLock()
		params := a.show
		a.mu.RUnlock()
		actionErr = a.showWindow(params)
	})
	if err != nil {
		return err
	}
	return actionErr
}

// HideAutomationWindow closes the launcher through its normal product path.
func (a *App) HideAutomationWindow() error {
	if a.window == nil {
		return errors.New("launcher window is not initialized")
	}
	var actionErr error
	err := woxui.Call(func() {
		actionErr = a.hideWindow(true)
	})
	if err != nil {
		return err
	}
	return actionErr
}

// AutomationWindowBounds reads native logical window bounds on the UI thread.
func (a *App) AutomationWindowBounds() (woxui.Rect, error) {
	if a.window == nil {
		return woxui.Rect{}, errors.New("launcher window is not initialized")
	}
	var bounds woxui.Rect
	var boundsErr error
	err := woxui.Call(func() {
		bounds, boundsErr = a.window.Bounds()
	})
	if err != nil {
		return woxui.Rect{}, err
	}
	return bounds, boundsErr
}

// SetAutomationWindowBounds changes native logical window bounds on the UI thread.
func (a *App) SetAutomationWindowBounds(bounds woxui.Rect) error {
	if a.window == nil {
		return errors.New("launcher window is not initialized")
	}
	var boundsErr error
	err := woxui.Call(func() {
		boundsErr = a.window.SetBounds(bounds)
	})
	if err != nil {
		return err
	}
	return boundsErr
}

// CaptureAutomationWindow writes current native window pixels on the UI thread.
func (a *App) CaptureAutomationWindow(path string) error {
	if a.window == nil {
		return errors.New("launcher window is not initialized")
	}
	var captureErr error
	err := woxui.Call(func() {
		captureErr = a.window.CapturePNG(path)
	})
	if err != nil {
		return err
	}
	return captureErr
}
