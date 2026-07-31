package launcher

import (
	"context"
	"errors"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type automationSurfaceKind uint8

const (
	automationSurfaceLauncher automationSurfaceKind = iota
	automationSurfaceSettings
	automationSurfaceOnboarding
)

func (a *App) automationSurface() (*woxwidget.Host, *woxui.Window, automationSurfaceKind) {
	var host *woxwidget.Host
	var window *woxui.Window
	kind := automationSurfaceLauncher
	_ = a.runOnUI("resolve automation surface", func() {
		if a.onboardingOpen && a.onboardingHost != nil && a.onboardingView != nil {
			host = a.onboardingHost
			window = a.onboardingView.Window()
			kind = automationSurfaceOnboarding
			return
		}
		if a.settingsOpen && a.settingsHost != nil && a.settingsView != nil {
			host = a.settingsHost
			window = a.settingsView.Window()
			kind = automationSurfaceSettings
			return
		}
		host = a.host
		window = a.window
	})
	return host, window, kind
}

// independentAutomationWindow routes black-box input and capture to a live raw multi-window surface.
func (a *App) independentAutomationWindow() *woxui.Window {
	managed, found := a.windows.Get(woxui.ScreenshotWindowID)
	if !found {
		return nil
	}
	switch managed.Lifecycle() {
	case woxui.WindowLifecycleClosing, woxui.WindowLifecycleClosed:
		return nil
	default:
		return managed.Window()
	}
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

// DispatchAutomationPointer sends a logical pointer event through the active retained host.
func (a *App) DispatchAutomationPointer(event woxui.PointerEvent) error {
	if window := a.independentAutomationWindow(); window != nil {
		return window.DispatchPointer(event)
	}
	host, _, _ := a.automationSurface()
	if host == nil {
		return errors.New("active widget host is not initialized")
	}
	return woxui.Call(func() {
		host.Pointer(event)
	})
}

// PressAutomationKey sends a complete key press through the normal widget and launcher handlers.
func (a *App) PressAutomationKey(key woxui.Key, modifiers woxui.KeyModifiers) error {
	if window := a.independentAutomationWindow(); window != nil {
		if _, err := window.DispatchKey(woxui.KeyEvent{Key: key, Modifiers: modifiers, Down: true}); err != nil {
			return err
		}
		_, err := window.DispatchKey(woxui.KeyEvent{Key: key, Modifiers: modifiers})
		return err
	}
	host, _, kind := a.automationSurface()
	if host == nil {
		return errors.New("active widget host is not initialized")
	}
	return woxui.Call(func() {
		down := woxui.KeyEvent{Key: key, Modifiers: modifiers, Down: true}
		if !host.Key(down) {
			switch kind {
			case automationSurfaceOnboarding:
				a.onOnboardingWindowKey(down)
			case automationSurfaceSettings:
				a.onSettingsWindowKey(down)
			default:
				a.onKey(down)
			}
		}
		up := woxui.KeyEvent{Key: key, Modifiers: modifiers}
		if !host.Key(up) {
			switch kind {
			case automationSurfaceOnboarding:
				a.onOnboardingWindowKey(up)
			case automationSurfaceSettings:
				a.onSettingsWindowKey(up)
			default:
				a.onKey(up)
			}
		}
	})
}

// EnterAutomationText commits UTF-8 text through the active text-input owner.
func (a *App) EnterAutomationText(text string) error {
	host, _, kind := a.automationSurface()
	if host == nil {
		return errors.New("active widget host is not initialized")
	}
	return woxui.Call(func() {
		event := woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: text}
		if !host.TextInput(event) {
			if kind == automationSurfaceSettings {
				a.onSettingsWindowTextInput(event)
			} else if kind == automationSurfaceLauncher {
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
		params := a.show
		actionErr = a.showWindow(params)
	})
	if err != nil {
		return err
	}
	return actionErr
}

// OpenAutomationSettings opens a settings route through the normal independent-window lifecycle.
func (a *App) OpenAutomationSettings(path string) error {
	if a.window == nil {
		return errors.New("launcher window is not initialized")
	}
	var actionErr error
	err := woxui.Call(func() {
		actionErr = a.openSettings(settingWindowContext{Path: path, Source: "automation"})
	})
	if err != nil {
		return err
	}
	return actionErr
}

// HideAutomationWindow closes the launcher through its normal product path.
func (a *App) HideAutomationWindow() error {
	_, window, kind := a.automationSurface()
	if window == nil {
		return errors.New("active window is not initialized")
	}
	var actionErr error
	err := woxui.Call(func() {
		switch kind {
		case automationSurfaceOnboarding:
			a.finishOnboarding()
		case automationSurfaceSettings:
			actionErr = a.closeSettings()
		default:
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
	if independent := a.independentAutomationWindow(); independent != nil {
		window = independent
	}
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
	if independent := a.independentAutomationWindow(); independent != nil {
		window = independent
	}
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
