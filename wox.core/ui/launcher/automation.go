package launcher

import (
	"context"
	"errors"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func (a *App) automationSurface() (*woxwidget.Host, *woxui.Window, bool) {
	a.mu.RLock()
	if a.settingsOpen && a.settingsHost != nil && a.settingsView != nil {
		host := a.settingsHost
		window := a.settingsView.Window()
		a.mu.RUnlock()
		return host, window, true
	}
	host := a.host
	window := a.window
	a.mu.RUnlock()
	return host, window, false
}

// AutomationSnapshot returns the latest immutable semantics tree.
func (a *App) AutomationSnapshot() woxwidget.AutomationSnapshot {
	host, _, _ := a.automationSurface()
	if host == nil {
		return woxwidget.AutomationSnapshot{}
	}
	return host.Snapshot()
}

// WaitForAutomationChange waits for a newer reconciled frame.
func (a *App) WaitForAutomationChange(ctx context.Context, afterGeneration uint64) (woxwidget.AutomationSnapshot, error) {
	host, _, _ := a.automationSurface()
	if host == nil {
		return woxwidget.AutomationSnapshot{}, errors.New("active widget host is not initialized")
	}
	return host.WaitForChange(ctx, afterGeneration)
}

// PerformAutomationAction invokes one semantics action by stable automation ID.
func (a *App) PerformAutomationAction(automationID string, action woxui.AccessibilityAction, value string) error {
	host, _, _ := a.automationSurface()
	if host == nil {
		return errors.New("active widget host is not initialized")
	}
	return host.PerformAutomationAction(automationID, action, value)
}

// PressAutomationKey sends a complete key press through the normal widget and launcher handlers.
func (a *App) PressAutomationKey(key woxui.Key, modifiers woxui.KeyModifiers) error {
	host, _, settings := a.automationSurface()
	if host == nil {
		return errors.New("active widget host is not initialized")
	}
	return woxui.Call(func() {
		down := woxui.KeyEvent{Key: key, Modifiers: modifiers, Down: true}
		if !host.Key(down) {
			if settings {
				a.onSettingsWindowKey(down)
			} else {
				a.onKey(down)
			}
		}
		up := woxui.KeyEvent{Key: key, Modifiers: modifiers}
		if !host.Key(up) {
			if settings {
				a.onSettingsWindowKey(up)
			} else {
				a.onKey(up)
			}
		}
	})
}

// EnterAutomationText commits UTF-8 text through the active text-input owner.
func (a *App) EnterAutomationText(text string) error {
	host, _, settings := a.automationSurface()
	if host == nil {
		return errors.New("active widget host is not initialized")
	}
	return woxui.Call(func() {
		event := woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: text}
		if !host.TextInput(event) {
			if settings {
				a.onSettingsWindowTextInput(event)
			} else {
				a.onTextInput(event)
			}
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
	_, window, settings := a.automationSurface()
	if window == nil {
		return errors.New("active window is not initialized")
	}
	var actionErr error
	err := woxui.Call(func() {
		if settings {
			actionErr = a.closeSettings()
		} else {
			actionErr = a.hideWindow(true)
		}
	})
	if err != nil {
		return err
	}
	return actionErr
}

// AutomationWindowBounds reads native logical window bounds on the UI thread.
func (a *App) AutomationWindowBounds() (woxui.Rect, error) {
	_, window, _ := a.automationSurface()
	if window == nil {
		return woxui.Rect{}, errors.New("active window is not initialized")
	}
	var bounds woxui.Rect
	var boundsErr error
	err := woxui.Call(func() {
		bounds, boundsErr = window.Bounds()
	})
	if err != nil {
		return woxui.Rect{}, err
	}
	return bounds, boundsErr
}

// SetAutomationWindowBounds changes native logical window bounds on the UI thread.
func (a *App) SetAutomationWindowBounds(bounds woxui.Rect) error {
	_, window, _ := a.automationSurface()
	if window == nil {
		return errors.New("active window is not initialized")
	}
	var boundsErr error
	err := woxui.Call(func() {
		boundsErr = window.SetBounds(bounds)
	})
	if err != nil {
		return err
	}
	return boundsErr
}

// CaptureAutomationWindow writes current native window pixels on the UI thread.
func (a *App) CaptureAutomationWindow(path string) error {
	_, window, _ := a.automationSurface()
	if window == nil {
		return errors.New("active window is not initialized")
	}
	var captureErr error
	err := woxui.Call(func() {
		captureErr = window.CapturePNG(path)
	})
	if err != nil {
		return err
	}
	return captureErr
}
