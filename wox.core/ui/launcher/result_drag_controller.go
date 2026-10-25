package launcher

import (
	"context"

	"wox/plugin"
	woxui "wox/ui/runtime"
)

type resultDragServices interface {
	PrepareResultDrag(context.Context, string, string, string) func(plugin.DragOutEvent)
}

// resultDragController owns the native drag session and post-drag visibility hold.
type resultDragController struct {
	services              resultDragServices
	sessionID             string
	lifecycleCtx          context.Context
	resultList            *resultController
	window                func() *woxui.Window
	hideOnBlur            func() bool
	hideWindow            func(bool) error
	pendingResultFileDrag pendingResultFileDrag
	resultFileDragActive  bool
	// resultFileDragHeld keeps the launcher visible until the next user interaction.
	resultFileDragHeld bool
}

// bindResultDrag supplies the configured blur policy and result-drag core capability.
func (a *App) bindResultDrag() {
	c := &a.resultDrag
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	if c.lifecycleCtx == nil {
		c.lifecycleCtx = context.Background()
	}
	c.resultList = &a.resultList
	c.window = func() *woxui.Window { return a.window }
	c.hideOnBlur = func() bool { return a.show.HideOnBlur }
	c.hideWindow = a.hideWindow
}
