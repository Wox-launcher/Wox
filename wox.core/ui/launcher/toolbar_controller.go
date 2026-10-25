package launcher

import (
	"context"

	woxui "wox/ui/runtime"
)

type toolbarServices interface {
	ExecuteToolbarMessageAction(context.Context, string, string, string) error
}

// toolbarController owns persistent and transient messages and their expiry generation.
type toolbarController struct {
	services              toolbarServices
	sessionID             string
	lifecycleCtx          context.Context
	runOnUI               func(string, func()) error
	window                func() *woxui.Window
	actions               *actionPanelController
	applyWindowBounds     func() error
	restoreQueryTextInput func()
	hideWindow            func(bool) error
	toolbarMsg            *toolbarMessage
	toolbarFallbackMsg    *toolbarMessage
	toolbarRevision       uint64
}

// bindToolbar supplies action-panel reconciliation and launcher visibility commands.
func (a *App) bindToolbar() {
	c := &a.toolbar
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.window = func() *woxui.Window { return a.window }
	c.actions = &a.actions
	c.applyWindowBounds = a.applyWindowBounds
	c.restoreQueryTextInput = a.restoreQueryTextInput
	c.hideWindow = a.hideWindow
}
