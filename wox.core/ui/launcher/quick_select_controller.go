package launcher

import (
	"context"
	"time"

	woxui "wox/ui/runtime"
)

// quickSelectController owns the modifier-hold timer and visible result numbering.
type quickSelectController struct {
	lifecycleCtx          context.Context
	resultList            *resultController
	window                func() *woxui.Window
	runOnUI               func(string, func()) error
	quickSelectBlocked    func() bool
	selectResult          func(int)
	activateResult        func(int)
	quickSelectMode       bool
	quickSelectKeyPressed bool
	quickSelectTimer      *time.Timer
	quickSelectViewport   quickSelectViewport
}

// bindQuickSelect supplies result activation without coupling to overlay implementations.
func (a *App) bindQuickSelect() {
	c := &a.quickSelect
	c.lifecycleCtx = a.lifecycleCtx
	c.resultList = &a.resultList
	c.window = func() *woxui.Window { return a.window }
	c.runOnUI = a.runOnUI
	c.quickSelectBlocked = a.quickSelectBlocked
	c.selectResult = a.selectResult
	c.activateResult = a.activateResult
}
