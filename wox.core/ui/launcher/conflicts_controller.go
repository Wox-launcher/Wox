package launcher

import (
	"context"

	woxui "wox/ui/runtime"
)

// triggerConflictController owns its preview form, validation, and asynchronous save generation.
type triggerConflictController struct {
	services              pluginSettingsWriter
	sessionID             string
	lifecycleCtx          context.Context
	querySession          *queryController
	runOnUI               func(string, func()) error
	window                func() *woxui.Window
	updateFormTextInput   func(bool)
	restoreQueryTextInput func()
	applyWindowBounds     func() error
	setQuery              func(plainQuery)
	sendCurrentQuery      func() error
	triggerConflict       *triggerConflictPreviewState
}

// bindConflicts connects query restart and form input capabilities.
func (a *App) bindConflicts() {
	c := &a.conflicts
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.querySession = &a.querySession
	c.runOnUI = a.runOnUI
	c.window = func() *woxui.Window { return a.window }
	c.updateFormTextInput = a.updateFormTextInput
	c.restoreQueryTextInput = a.restoreQueryTextInput
	c.applyWindowBounds = a.applyWindowBounds
	c.setQuery = a.setQuery
	c.sendCurrentQuery = a.sendCurrentQuery
}
