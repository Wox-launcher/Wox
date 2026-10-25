package launcher

import (
	"context"
	"sync"
	"sync/atomic"

	"wox/plugin/system/shell/terminal"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type terminalServices interface {
	SubscribeTerminal(context.Context, string, string, int64) (terminal.SessionState, error)
	UnsubscribeTerminal(context.Context, string, string) error
}

// terminalController owns the terminal subscription, output buffer, search, and selection.
// Subscription reconciliation remains serialized independently of UI updates.
type terminalController struct {
	services               terminalServices
	sessionID              string
	lifecycleCtx           context.Context
	runOnUI                func(string, func()) error
	window                 func() *woxui.Window
	host                   func() *woxwidget.Host
	palette                func() uiPalette
	restoreQueryTextInput  func()
	updateFormTextInput    func(bool)
	queryCanFocus          func() bool
	applyWindowBounds      func() error
	terminalSubscriptionMu sync.Mutex
	terminalSubscribed     string
	terminalDesired        atomic.Value
	terminalMenuAnchor     woxui.Point
	terminalMenuEnablement terminalMenuEnablement
	terminalFullscreen     bool
	terminalLayout         textLayoutCache
	terminalPreview        *terminalPreviewState
}

// bindTerminal connects terminal presentation to the shared window and input routing.
func (a *App) bindTerminal() {
	a.terminal.services = a.services
	a.terminal.sessionID = a.sessionID
	a.terminal.lifecycleCtx = a.lifecycleCtx
	a.terminal.runOnUI = a.runOnUI
	a.terminal.window = func() *woxui.Window { return a.window }
	a.terminal.host = func() *woxwidget.Host { return a.host }
	a.terminal.palette = func() uiPalette { return a.appearance.palette }
	a.terminal.restoreQueryTextInput = a.restoreQueryTextInput
	a.terminal.updateFormTextInput = a.updateFormTextInput
	a.terminal.queryCanFocus = a.queryCanFocus
	a.terminal.applyWindowBounds = a.applyWindowBounds
}
