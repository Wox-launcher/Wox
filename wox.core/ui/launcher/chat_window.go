package launcher

import (
	"fmt"
	"runtime"
	"strings"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const (
	chatWindowRole          = woxui.WindowRoleApplication
	chatWindowDefaultWidth  = float32(900)
	chatWindowDefaultHeight = float32(680)
	chatWindowMinimumWidth  = float32(640)
	chatWindowMinimumHeight = float32(480)
)

var chatWindowIcon, _ = decodeWoxImageWithTint(fromCoreImage(common.PluginAIChatIcon), nil, 256)

func chatWindowNativeMinSize() woxui.Size {
	return woxui.Size{Width: chatWindowMinimumWidth, Height: chatWindowMinimumHeight}
}

// requestOpenDedicatedChatWindow pops the current conversation into the single chat window.
func (a *App) requestOpenDedicatedChatWindow() {
	util.Go(a.lifecycleCtx, "open dedicated chat window", func() {
		if err := a.openDedicatedChatWindow(); err != nil {
			util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("open dedicated chat window: %v", err))
		}
	})
}

// chatWindowOpen reports whether the dedicated chat window still has a live native instance.
func (a *App) chatWindowOpen() bool {
	return a != nil && a.chatView != nil && a.chatView.Lifecycle() != woxui.WindowLifecycleClosed
}

// chatSurfaceVisible is true while launcher chat or the dedicated window can present the conversation.
func (a *App) chatSurfaceVisible() bool {
	return a.visible || a.chatWindowOpen()
}

func (a *App) chatNativeWindow() *woxui.Window {
	if !a.chatWindowOpen() {
		return nil
	}
	return a.chatView.Window()
}

func (a *App) invalidateChatWindow() {
	if window := a.chatNativeWindow(); window != nil {
		_ = window.Invalidate()
	}
}

// invalidateChatSurfaces refreshes launcher chat and the dedicated window together.
func (a *App) invalidateChatSurfaces() {
	if a.window != nil {
		_ = a.window.Invalidate()
	}
	a.invalidateChatWindow()
}

// chatTextInputWindow routes imperative IME updates to the active input surface.
func (a *App) chatTextInputWindow() *woxui.Window {
	if a.chatWindowFocused || !a.visible {
		if window := a.chatNativeWindow(); window != nil {
			return window
		}
	}
	return a.window
}

// openDedicatedChatWindow creates or focuses the single resizable chat window.
func (a *App) openDedicatedChatWindow() error {
	var openErr error
	if err := a.runOnUI("open dedicated chat window", func() {
		openErr = a.openDedicatedChatWindowLocked()
	}); err != nil {
		return err
	}
	return openErr
}

// openDedicatedChatWindowLocked transfers presentation and input ownership on the UI thread.
func (a *App) openDedicatedChatWindowLocked() error {
	if a.chatPreview == nil {
		return fmt.Errorf("chat is not active")
	}
	managed, err := a.ensureChatWindow()
	if err != nil {
		return err
	}
	a.chatPreview.active = true
	a.chatFullscreen = false
	a.updateChatTextInput(true)
	if _, err := managed.Show(); err != nil {
		return err
	}
	a.hideLauncherAfterChatPopOut()
	return managed.Window().Invalidate()
}

// hideLauncherAfterChatPopOut leaves the conversation in the dedicated window.
func (a *App) hideLauncherAfterChatPopOut() {
	a.visible = false
	a.chatFullscreen = false
	if a.launcher == nil {
		return
	}
	if err := a.launcher.Hide(); err != nil {
		util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("hide launcher after chat pop-out: %v", err))
	}
	if a.isPrimary {
		util.Go(a.lifecycleCtx, "notify hidden after chat pop-out", func() {
			if err := a.notifyHidden(); err != nil {
				util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("notify hidden after chat pop-out: %v", err))
			}
		})
	}
}

// ensureChatWindow creates the dedicated chat host once per native lifetime.
func (a *App) ensureChatWindow() (*woxui.ManagedWindow, error) {
	if a.windows == nil {
		return nil, fmt.Errorf("window manager is not initialized")
	}
	var managed *woxui.ManagedWindow
	var openErr error
	var fontFamily string
	var isDark bool
	created := false
	if err := woxui.Call(func() {
		if existing := a.chatView; existing != nil && existing.Lifecycle() != woxui.WindowLifecycleClosed {
			managed = existing
			return
		}
		host := woxwidget.NewHost(a.buildChatWindow)
		id := chatWindowID
		if !a.isPrimary {
			// Named launcher sessions must not attach another host to the primary chat window.
			id = a.windowID + ".chat"
		}
		managed, _, openErr = a.windows.Open(id, woxui.WindowOptions{
			Title:   a.chatWindowTitle(),
			Size:    woxui.Size{Width: chatWindowDefaultWidth, Height: chatWindowDefaultHeight},
			MinSize: chatWindowNativeMinSize(),
			Role:    chatWindowRole, Icon: chatWindowIcon, Resizable: true, HideOnBlur: false,
			OnFrame: host.Frame, OnPointer: host.Pointer,
			OnFocus: func(event woxui.FocusEvent) {
				host.SetWindowFocused(event.Active)
				a.chatWindowFocused = event.Active
				if event.Active {
					a.updateChatTextInput(true)
				}
			},
			OnKey: func(event woxui.KeyEvent) bool {
				if host.Key(event) {
					return true
				}
				return a.onDedicatedChatKey(event)
			},
			OnTextInput:      func(event woxui.TextInputEvent) { host.TextInput(event) },
			OnCloseRequested: a.requestCloseChatWindow,
			OnClosed: func() {
				host.Dispose()
				a.onChatWindowClosed()
			},
		})
		if openErr == nil {
			host.Attach(managed.Window())
			a.chatView = managed
			a.chatHost = host
			fontFamily = a.generalSettings.Data().AppFontFamily
			isDark = themeColorIsDark(a.palette.background)
			created = true
		}
	}); err != nil {
		return nil, err
	}
	if openErr != nil {
		return nil, openErr
	}
	if !created {
		return managed, nil
	}
	window := managed.Window()
	if err := window.SetAppearance(isDark); err != nil {
		_ = managed.Close()
		return nil, err
	}
	if err := window.SetFontFamily(fontFamily); err != nil {
		_ = managed.Close()
		return nil, err
	}
	if a.chatWindowRestoreFrame.Width > 0 && a.chatWindowRestoreFrame.Height > 0 && !a.chatWindowMaximized {
		_ = window.SetBounds(clampChatWindowBounds(a.chatWindowRestoreFrame))
	} else if err := window.CenterOnMouseScreen(woxui.Size{Width: chatWindowDefaultWidth, Height: chatWindowDefaultHeight}); err != nil {
		util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("center dedicated chat window: %v", err))
	}
	if a.chatWindowMaximized {
		a.maximizeChatWindowOn(window)
	}
	return managed, nil
}

// chatWindowTitle resolves the shared native and accessibility window label.
func (a *App) chatWindowTitle() string {
	title := strings.TrimSpace(a.translate("i18n:ui_ai_chat_window_title"))
	if title == "" || title == "i18n:ui_ai_chat_window_title" {
		return "Wox Chat"
	}
	return title
}

// closeChatWindow saves placement before the native handle is destroyed.
func (a *App) closeChatWindow() error {
	var managed *woxui.ManagedWindow
	if err := a.runOnUI("close dedicated chat window", func() {
		managed = a.chatView
		if managed != nil && !a.chatWindowMaximized {
			if bounds, err := managed.Window().Bounds(); err == nil {
				a.chatWindowRestoreFrame = bounds
			}
		}
	}); err != nil {
		return err
	}
	if managed == nil {
		return nil
	}
	return managed.Close()
}

// onChatWindowClosed returns preview lifecycle and keyboard ownership to the launcher.
func (a *App) onChatWindowClosed() {
	a.chatView = nil
	a.chatHost = nil
	a.chatWindowFocused = false
	if a.destroyed.Load() {
		return
	}
	a.setPreviewTooltip(false, "", woxui.Rect{})
	if a.visible && a.chatFullscreen {
		// Closing one entrance must not discard the conversation still open in the other.
		a.invalidateChatSurfaces()
		return
	}
	a.deactivateChatPreview()
	a.reconcileSelectedPreviewOnUI()
	if !a.isPrimary && !a.visible {
		util.Go(a.lifecycleCtx, "close chat launcher session", func() {
			if err := a.Close(); err != nil {
				util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("close chat launcher session: %v", err))
			}
		})
	}
}

// buildChatWindow adapts the current conversation to the dedicated window view.
func (a *App) buildChatWindow(frame woxui.FrameInfo) woxwidget.Widget {
	width := frame.Size.Width
	height := frame.Size.Height
	a.syncChatWindowMaximizedFromFrame(frame.Size)
	theme := a.palette.componentTheme()
	titleBar := a.buildChatWindowTitleBar(width, frame.WindowFocused, theme)
	bodyHeight := max(float32(0), height-launcherview.ChatWindowTitleBarHeight)
	body := a.buildActiveChatPreview(width, bodyHeight, frame.Scale, false)
	return launcherview.ChatWindow(launcherview.ChatWindowProps{
		Width: width, Height: height, Title: a.chatWindowTitle(), TitleBar: titleBar, Body: body, Theme: theme,
	})
}

// buildActiveChatPreview shares the preview view with the dedicated conversation owner.
func (a *App) buildActiveChatPreview(width, height, imageScale float32, showHeader bool) woxwidget.Widget {
	snapshot := snapshotChatPreviewLocked(a.chatPreview)
	if snapshot == nil {
		return nil
	}
	a.attachChatPreviewCatalogs(snapshot)
	return a.buildChatPreviewFromSnapshot(snapshot, a.palette, width, height, imageScale, showHeader, false)
}

// buildChatWindowTitleBar supplies conversation data and stable native callbacks to the view.
func (a *App) buildChatWindowTitleBar(width float32, active bool, theme woxcomponent.Theme) woxwidget.Widget {
	snapshot := snapshotChatPreviewLocked(a.chatPreview)
	var header *previewview.ChatHeaderProps
	if snapshot != nil {
		props := a.chatHeaderProps(snapshot, a.palette, width, launcherview.ChatWindowTitleBarHeight, false, false)
		props.OnDrag = a.startChatWindowDrag
		props.OnDoubleTap = a.toggleChatWindowMaximize
		header = &props
	}
	return launcherview.ChatWindowTitleBar(launcherview.ChatWindowTitleBarProps{
		Width: width, Platform: runtime.GOOS, Active: active, Maximized: a.chatWindowMaximized, Theme: theme, Header: header,
		OnDrag: a.startChatWindowDrag, OnMinimize: a.minimizeChatWindow, OnMaximize: a.toggleChatWindowMaximize, OnClose: a.requestCloseChatWindow,
	})
}

// startChatWindowDrag hands the pointer to the dedicated chat window, not the launcher.
func (a *App) startChatWindowDrag() {
	if a.chatWindowMaximized {
		a.restoreChatWindowFromMaximize()
	}
	if window := a.chatNativeWindow(); window != nil {
		_ = window.StartDragging()
	}
}

// requestCloseChatWindow schedules native destruction outside the input callback.
func (a *App) requestCloseChatWindow() {
	util.Go(a.lifecycleCtx, "close dedicated chat window", func() {
		if err := a.closeChatWindow(); err != nil {
			util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("close dedicated chat window: %v", err))
		}
	})
}

func (a *App) minimizeChatWindow() {
	if window := a.chatNativeWindow(); window != nil {
		_ = window.Minimize()
	}
}

// toggleChatWindowMaximize switches between the saved frame and the active display work area.
func (a *App) toggleChatWindowMaximize() {
	if a.chatWindowMaximized {
		a.restoreChatWindowFromMaximize()
		return
	}
	a.maximizeChatWindow()
}

func (a *App) maximizeChatWindow() {
	a.maximizeChatWindowOn(a.chatNativeWindow())
}

// maximizeChatWindowOn saves logical placement before filling the window's display.
func (a *App) maximizeChatWindowOn(window *woxui.Window) {
	if window == nil {
		return
	}
	bounds, err := window.Bounds()
	if err != nil {
		return
	}
	if !a.chatWindowMaximized || a.chatWindowRestoreFrame.Width <= 0 || a.chatWindowRestoreFrame.Height <= 0 {
		a.chatWindowRestoreFrame = bounds
	}
	target := notesMaximizeBounds(bounds)
	if err := window.SetBounds(target); err != nil {
		return
	}
	a.chatWindowMaximized = true
	a.invalidateChatWindow()
}

// restoreChatWindowFromMaximize restores the saved logical frame with minimum-size constraints.
func (a *App) restoreChatWindowFromMaximize() {
	window := a.chatNativeWindow()
	if window == nil {
		return
	}
	target := a.chatWindowRestoreFrame
	if target.Width <= 0 || target.Height <= 0 {
		target = woxui.Rect{Width: chatWindowDefaultWidth, Height: chatWindowDefaultHeight}
	}
	target = clampChatWindowBounds(target)
	if err := window.SetBounds(target); err != nil {
		return
	}
	a.chatWindowMaximized = false
	a.invalidateChatWindow()
}

// syncChatWindowMaximizedFromFrame recognizes native resizing after a custom maximize.
func (a *App) syncChatWindowMaximizedFromFrame(size woxui.Size) {
	if !a.chatWindowMaximized {
		return
	}
	var current woxui.Rect
	if window := a.chatNativeWindow(); window != nil {
		if bounds, err := window.Bounds(); err == nil {
			current = bounds
		}
	}
	if current.Width <= 0 || current.Height <= 0 {
		current.Width, current.Height = size.Width, size.Height
	}
	target := notesMaximizeBounds(current)
	if abs32(size.Width-target.Width) <= 4 && abs32(size.Height-target.Height) <= 4 {
		return
	}
	a.chatWindowMaximized = false
	a.chatWindowRestoreFrame = current
}

// clampChatWindowBounds enforces logical minimum dimensions without changing the desktop origin.
func clampChatWindowBounds(bounds woxui.Rect) woxui.Rect {
	if bounds.Width < chatWindowMinimumWidth {
		bounds.Width = chatWindowMinimumWidth
	}
	if bounds.Height < chatWindowMinimumHeight {
		bounds.Height = chatWindowMinimumHeight
	}
	return bounds
}
