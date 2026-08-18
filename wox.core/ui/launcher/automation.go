package launcher

import (
	"context"
	"errors"
	"strings"
	"time"

	"wox/common"
	"wox/ui/automation"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxscreenshot "wox/ui/screenshot"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
)

type automationSessionResetter interface {
	ResetAutomationSession(ctx context.Context, sessionID string) error
}

type automationSurfaceKind uint8

const (
	automationSurfaceLauncher automationSurfaceKind = iota
	automationSurfaceSettings
	automationSurfaceOnboarding
	automationSurfaceOverlay
)

const automationOverlayInstancePrefix = "overlay."

func (a *App) automationSurface() (*woxwidget.Host, *woxui.Window, automationSurfaceKind) {
	if overlayID, ok := a.automationOverlayID(); ok {
		host, window, _, found := overlay.AutomationSurface(overlayID)
		if found {
			return host, window, automationSurfaceOverlay
		}
	}
	target := a.resolveAutomationTarget()
	var host *woxwidget.Host
	var window *woxui.Window
	kind := automationSurfaceLauncher
	_ = target.runOnUI("resolve automation surface", func() {
		if target == a {
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
		}
		host = target.host
		window = target.window
	})
	return host, window, kind
}

// automationOverlayID resolves the focused runtime overlay without changing its native lifecycle.
func (a *App) automationOverlayID() (string, bool) {
	var focus string
	_ = a.runOnUI("read automation overlay focus", func() {
		focus = strings.TrimSpace(a.automationFocusInstance)
	})
	if !strings.HasPrefix(focus, automationOverlayInstancePrefix) {
		return "", false
	}
	id := strings.TrimSpace(strings.TrimPrefix(focus, automationOverlayInstancePrefix))
	return id, id != ""
}

// resolveAutomationTarget returns the primary or focused secondary smoke target.
func (a *App) resolveAutomationTarget() *App {
	var focus string
	_ = a.runOnUI("read automation focus instance", func() {
		focus = strings.TrimSpace(a.automationFocusInstance)
	})
	if focus == "" || focus == "primary" || a.instances == nil {
		return a
	}
	a.instances.mu.Lock()
	sessionID := a.instances.sessionByName[focus]
	target := a.instances.bySessionID[sessionID]
	a.instances.mu.Unlock()
	if target == nil || !target.isLive() {
		return a
	}
	return target
}

// independentAutomationWindow routes black-box input and capture to a live raw multi-window surface.
func (a *App) independentAutomationWindow() *woxui.Window {
	managed, found := a.windows.Get(woxscreenshot.ScreenshotWindowID)
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

// AutomationFrameMetrics returns timings for the native window currently controlled by automation.
func (a *App) AutomationFrameMetrics() (woxui.FrameMetricsSnapshot, error) {
	_, window, _ := a.automationSurface()
	if window == nil {
		return woxui.FrameMetricsSnapshot{}, errors.New("active automation window is not initialized")
	}
	return window.FrameMetrics(), nil
}

// ResetAutomationFrameMetrics starts a fresh measurement interval for the active window.
func (a *App) ResetAutomationFrameMetrics() error {
	_, window, _ := a.automationSurface()
	if window == nil {
		return errors.New("active automation window is not initialized")
	}
	window.ResetFrameMetrics()
	return nil
}

// RequestAutomationFrame invalidates the active surface for deterministic performance sampling.
func (a *App) RequestAutomationFrame() error {
	_, window, _ := a.automationSurface()
	if window == nil {
		return errors.New("active automation window is not initialized")
	}
	return window.Invalidate()
}

func nextRepaintDebugMode(current woxwidget.RepaintDebugMode) woxwidget.RepaintDebugMode {
	if current == woxwidget.RepaintDebugOff {
		return woxwidget.RepaintDebugRainbow
	}
	return woxwidget.RepaintDebugOff
}

// ToggleRepaintDebugMode toggles repaint highlighting on the launcher widget host.
func (a *App) ToggleRepaintDebugMode(_ context.Context) (string, error) {
	if a.host == nil {
		return string(woxwidget.RepaintDebugOff), errors.New("launcher widget host is not initialized")
	}
	var mode woxwidget.RepaintDebugMode
	var modeErr error
	if err := a.runOnUI("toggle repaint debug mode", func() {
		mode = nextRepaintDebugMode(a.host.RepaintDebugMode())
		modeErr = a.host.SetRepaintDebugMode(mode)
	}); err != nil {
		return "", err
	}
	if modeErr != nil {
		return "", modeErr
	}
	return string(mode), nil
}

// SetAutomationRepaintDebugMode changes incremental-rendering diagnostics on the active widget host.
func (a *App) SetAutomationRepaintDebugMode(mode woxwidget.RepaintDebugMode) error {
	host, _, _ := a.automationSurface()
	if host == nil {
		return errors.New("active widget host is not initialized")
	}
	var modeErr error
	if err := woxui.Call(func() {
		modeErr = host.SetRepaintDebugMode(mode)
	}); err != nil {
		return err
	}
	return modeErr
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
	host, window, kind := a.automationSurface()
	if host == nil {
		return errors.New("active widget host is not initialized")
	}
	if kind == automationSurfaceOverlay {
		return window.DispatchPointer(event)
	}
	return woxui.Call(func() {
		host.Pointer(event)
	})
}

// PressAutomationKey sends a complete key press through the normal widget and launcher handlers.
func (a *App) PressAutomationKey(key woxui.Key, modifiers woxui.KeyModifiers) (bool, error) {
	if window := a.independentAutomationWindow(); window != nil {
		downHandled, err := window.DispatchKey(woxui.KeyEvent{Key: key, Modifiers: modifiers, Down: true})
		if err != nil {
			return false, err
		}
		upHandled, err := window.DispatchKey(woxui.KeyEvent{Key: key, Modifiers: modifiers})
		return downHandled || upHandled, err
	}
	target := a.resolveAutomationTarget()
	host, window, kind := a.automationSurface()
	if host == nil {
		return false, errors.New("active widget host is not initialized")
	}
	if kind == automationSurfaceOverlay {
		downHandled, err := window.DispatchKey(woxui.KeyEvent{Key: key, Modifiers: modifiers, Down: true})
		if err != nil {
			return false, err
		}
		upHandled, err := window.DispatchKey(woxui.KeyEvent{Key: key, Modifiers: modifiers})
		return downHandled || upHandled, err
	}
	handled := false
	err := woxui.Call(func() {
		down := woxui.KeyEvent{Key: key, Modifiers: modifiers, Down: true}
		downHandled := host.Key(down)
		if !downHandled {
			switch kind {
			case automationSurfaceOnboarding:
				downHandled = a.onOnboardingWindowKey(down)
			case automationSurfaceSettings:
				downHandled = a.onSettingsWindowKey(down)
			default:
				downHandled = target.onKey(down)
			}
		}
		up := woxui.KeyEvent{Key: key, Modifiers: modifiers}
		upHandled := host.Key(up)
		if !upHandled {
			switch kind {
			case automationSurfaceOnboarding:
				upHandled = a.onOnboardingWindowKey(up)
			case automationSurfaceSettings:
				upHandled = a.onSettingsWindowKey(up)
			default:
				upHandled = target.onKey(up)
			}
		}
		handled = downHandled || upHandled
	})
	return handled, err
}

// EnterAutomationText commits UTF-8 text through the active text-input owner.
func (a *App) EnterAutomationText(text string) error {
	target := a.resolveAutomationTarget()
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
				target.onTextInput(event)
			}
		}
	})
}

// ResetAutomationState destroys secondary surfaces and clears primary launcher state between smoke cases.
func (a *App) ResetAutomationState() error {
	if a.window == nil || a.windows == nil {
		return errors.New("launcher automation state is not initialized")
	}
	var resetErr error
	if err := woxui.Call(func() {
		a.automationFocusInstance = ""
		if err := a.windows.CloseAllExcept(a.windowID); err != nil {
			resetErr = err
			return
		}
		a.setQuery(plainQuery{})
		a.queryHistories = nil
		a.toolbarMsg = nil
		a.toolbarFallbackMsg = nil
		a.toolbarRevision++
		a.resetAutomationPerfCatalog()
		resetErr = a.hideWindow(true)
	}); err != nil {
		return err
	}
	if resetErr != nil {
		return resetErr
	}
	if resetter, ok := a.services.(automationSessionResetter); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := resetter.ResetAutomationSession(ctx, a.sessionID); err != nil {
			return err
		}
	}
	return nil
}

// ShowAutomationWindow opens the launcher through its normal product path.
func (a *App) ShowAutomationWindow() error {
	if a.window == nil {
		return errors.New("launcher window is not initialized")
	}
	var params showAppParams
	resolved := false
	if provider, ok := a.services.(interface {
		AutomationShowOptions(context.Context, string) contract.ShowOptions
	}); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		params = fromCoreShowOptions(provider.AutomationShowOptions(ctx, a.sessionID))
		resolved = true
	}
	var actionErr error
	err := woxui.Call(func() {
		if !resolved {
			params = a.show
		}
		actionErr = a.showWindow(params)
	})
	if err != nil {
		return err
	}
	return actionErr
}

// OpenAutomationSelectionQuery enters the production selection-query flow after deterministic selection capture.
func (a *App) OpenAutomationSelectionQuery(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("selection text is required")
	}
	provider, ok := a.services.(interface {
		AutomationOpenSelectionQuery(context.Context, string, string) error
	})
	if !ok {
		return errors.New("selection query automation is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return provider.AutomationOpenSelectionQuery(ctx, a.sessionID, text)
}

// OpenAutomationExplorerQuery opens the File Explorer Search secondary with bottom-anchored chrome.
func (a *App) OpenAutomationExplorerQuery(query string) error {
	provider, ok := a.services.(interface {
		AutomationOpenExplorerQuery(context.Context, string, string) error
	})
	if !ok {
		return errors.New("explorer query automation is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.AutomationOpenExplorerQuery(ctx, a.sessionID, query); err != nil {
		return err
	}
	return a.SetAutomationFocusInstance(string(common.ShowSourceExplorer))
}

// SetAutomationFocusInstance routes later smoke Snapshot/Bounds/Perform calls to one launcher instance.
func (a *App) SetAutomationFocusInstance(instanceName string) error {
	instanceName = strings.TrimSpace(instanceName)
	if instanceName == "" {
		instanceName = "primary"
	}
	if instanceName != "primary" {
		state, err := a.AutomationWindowState(instanceName)
		if err != nil {
			return err
		}
		if !state.Exists {
			return errors.New("automation focus instance does not exist")
		}
	}
	return a.runOnUI("set automation focus instance", func() {
		if instanceName == "primary" {
			a.automationFocusInstance = ""
			return
		}
		a.automationFocusInstance = instanceName
	})
}

// AutomationWindowState returns the real managed lifecycle for the primary or a named secondary launcher.
func (a *App) AutomationWindowState(instanceName string) (automation.WindowState, error) {
	instanceName = strings.TrimSpace(instanceName)
	if strings.HasPrefix(instanceName, automationOverlayInstancePrefix) {
		overlayID := strings.TrimSpace(strings.TrimPrefix(instanceName, automationOverlayInstancePrefix))
		_, _, managed, found := overlay.AutomationSurface(overlayID)
		if !found {
			return automation.WindowState{}, nil
		}
		lifecycle := managed.Lifecycle()
		return automation.WindowState{
			Exists:    true,
			Visible:   lifecycle == woxui.WindowLifecycleVisible,
			Lifecycle: automationWindowLifecycle(lifecycle),
		}, nil
	}
	var target *App
	if instanceName == "primary" {
		target = a
	} else if a.instances != nil {
		a.instances.mu.Lock()
		sessionID := a.instances.sessionByName[instanceName]
		target = a.instances.bySessionID[sessionID]
		a.instances.mu.Unlock()
	}
	if target == nil {
		return automation.WindowState{}, nil
	}

	state := automation.WindowState{Exists: true, Lifecycle: "closed"}
	if err := target.runOnUI("read automation window state", func() {
		state.Visible = target.visible
		if target.launcher != nil {
			state.Lifecycle = automationWindowLifecycle(target.launcher.Lifecycle())
			state.BlurReady = target.launcher.Window().FocusReadyForBlur()
		}
	}); err != nil {
		if target.destroyed.Load() {
			return automation.WindowState{}, nil
		}
		return automation.WindowState{}, err
	}
	return state, nil
}

func automationWindowLifecycle(lifecycle woxui.WindowLifecycle) string {
	switch lifecycle {
	case woxui.WindowLifecycleCreated:
		return "created"
	case woxui.WindowLifecyclePresenting:
		return "presenting"
	case woxui.WindowLifecycleVisible:
		return "visible"
	case woxui.WindowLifecycleHidden:
		return "hidden"
	case woxui.WindowLifecycleClosing:
		return "closing"
	default:
		return "closed"
	}
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
