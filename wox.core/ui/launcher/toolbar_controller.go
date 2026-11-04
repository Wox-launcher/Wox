package launcher

import (
	"context"

	"wox/common"

	woxui "wox/ui/runtime"
	"wox/util/notifier"
)

type toolbarServices interface {
	ExecuteToolbarMessageAction(context.Context, string, string, string) error
}

// toolbarController owns persistent and transient messages and their expiry generation.
type toolbarController struct {
	services              toolbarServices
	writeClipboard        func(string) error
	showFullMessage       func(string) bool
	openSettings          func(context.Context, common.SettingWindowContext) error
	sessionID             string
	lifecycleCtx          context.Context
	runOnUI               func(string, func()) error
	window                func() *woxui.Window
	actions               *actionPanelController
	applyWindowBounds     func() error
	restoreQueryTextInput func()
	hideWindow            func(bool) error
	toolbarMsg            *toolbarMessage
	systemMessages        map[string]*systemToolbarEntry
	systemMessageRevision uint64
	toolbarRevision       uint64
	messageTruncated      bool
}

// bindToolbar supplies action-panel reconciliation and launcher visibility commands.
func (a *App) bindToolbar() {
	c := &a.toolbar
	c.services = a.services
	c.openSettings = a.OpenSetting
	c.writeClipboard = func(text string) error { return a.window.WriteClipboardText(text) }
	c.showFullMessage = func(text string) bool { return notifier.ShowToast(notifier.Toast{Message: text, Persistent: true}) }
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.window = func() *woxui.Window { return a.window }
	c.actions = &a.actions
	c.applyWindowBounds = a.applyWindowBounds
	c.restoreQueryTextInput = a.restoreQueryTextInput
	c.hideWindow = a.hideWindow
}
