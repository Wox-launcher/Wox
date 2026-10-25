package launcher

import (
	"context"
	"sync/atomic"
	"time"

	"wox/plugin"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type glanceServices interface {
	GlanceItems(context.Context, string, []plugin.GlanceKey, plugin.GlanceRefreshReason) ([]plugin.GlanceItemUI, error)
	ExecuteGlanceAction(context.Context, string, string, string, string) error
}

// glanceController owns refresh generations, provider scheduling, and the current accessory.
type glanceController struct {
	services                   glanceServices
	sessionID                  string
	lifecycleCtx               context.Context
	runOnUI                    func(string, func()) error
	window                     func() *woxui.Window
	tooltips                   *tooltipController
	primaryGlance              func() glanceRef
	queryID                    func() string
	glanceEligibleLocked       func() bool
	catalog                    func() []glanceCatalogItem
	loadGlanceCatalog          func()
	invalidateLauncherBoundary func(woxwidget.Key)
	hideWindow                 func(bool) error
	glanceItem                 *glanceItem
	glanceLoading              bool
	glanceRevision             uint64
	glanceTooltipRevision      atomic.Uint64
	glanceTimer                *time.Timer
}

// bindGlance provides eligibility and settings inputs without sharing query state.
func (a *App) bindGlance() {
	c := &a.glance
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.window = func() *woxui.Window { return a.window }
	c.tooltips = &a.tooltips
	c.primaryGlance = func() glanceRef { return a.generalSettings.Data().PrimaryGlance }
	c.queryID = func() string { return a.querySession.query.QueryID }
	c.glanceEligibleLocked = a.glanceEligibleLocked
	c.catalog = func() []glanceCatalogItem { return a.appearanceSettings.Snapshot().GlanceCatalog }
	c.loadGlanceCatalog = a.loadGlanceCatalog
	c.invalidateLauncherBoundary = a.invalidateLauncherBoundary
	c.hideWindow = a.hideWindow
}
