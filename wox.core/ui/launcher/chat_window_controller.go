package launcher

import (
	"context"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// chatWindowController owns dedicated Chat host, focus generation, and native placement.
type chatWindowController struct {
	windows                  *woxui.WindowManager
	runOnUI                  func(string, func()) error
	fontFamily               func() string
	lifecycleCtx             context.Context
	isPrimary                bool
	windowID                 woxui.WindowID
	language                 *languageCatalog
	chat                     *chatController
	palette                  func() uiPalette
	buildChatWindow          func(woxui.FrameInfo) woxwidget.Widget
	dispatchWindowKey        func(woxui.KeyEvent, *woxwidget.Host, string, func(), func(woxui.KeyEvent) bool) bool
	handleChatWindowFileDrop func([]string)
	onChatWindowClosed       func()
	chatView                 *woxui.ManagedWindow
	chatHost                 *woxwidget.Host
	chatWindowFocused        bool
	chatWindowMaximized      bool
	chatWindowRestoreFrame   woxui.Rect
	chatWindowGeneration     uint64
	sidebarMinimumWidth      float32
}

// bindChatWindow supplies rendering and feature handoffs to the native window owner.
func (a *App) bindChatWindow() {
	c := &a.chatWindow
	c.windows = a.windows
	c.runOnUI = a.runOnUI
	c.fontFamily = func() string { return a.generalSettings.Data().AppFontFamily }
	c.lifecycleCtx = a.lifecycleCtx
	c.isPrimary = a.isPrimary
	c.windowID = a.windowID
	c.language = &a.language
	c.chat = &a.chat
	c.palette = func() uiPalette { return a.appearance.palette }
	c.buildChatWindow = a.buildChatWindow
	c.dispatchWindowKey = a.dispatchWindowKey
	c.handleChatWindowFileDrop = a.handleChatWindowFileDrop
	c.onChatWindowClosed = a.onChatWindowClosed
}
