package launcher

import (
	"context"
	"sync"

	"wox/ui/contract"
	"wox/util"
)

type tooltipServices interface {
	ShowTooltip(context.Context, string, contract.TooltipOptions) error
	HideTooltip(context.Context, string, string) error
}

// tooltipController owns hover scheduling and native tooltip identity, independent of the owner window.
type tooltipController struct {
	isDestroyed  func() bool
	services     tooltipServices
	sessionID    string
	lifecycleCtx context.Context
	runOnUI      func(string, func()) error
	tooltipMu    sync.Mutex
	// Native calls can reenter the UI thread, so they must not hold tooltipMu.
	tooltipCallMu           sync.Mutex
	nativeHoverTooltipShown map[string]nativeHoverTooltipIdentity
}

// bindTooltips supplies the narrow overlay capability shared by this instance's surfaces.
func (a *App) bindTooltips() {
	a.tooltips.isDestroyed = a.isDestroyed
	a.tooltips.services = a.services
	a.tooltips.sessionID = a.sessionID
	a.tooltips.lifecycleCtx = a.lifecycleCtx
	if a.tooltips.lifecycleCtx == nil {
		a.tooltips.lifecycleCtx = context.Background()
	}
	a.tooltips.runOnUI = a.runOnUI
}

// disposeOnUI closes every surface's tracked overlay without waiting on a worker's native call lock.
func (c *tooltipController) disposeOnUI() {
	c.tooltipMu.Lock()
	names := make([]string, 0, len(c.nativeHoverTooltipShown))
	for name := range c.nativeHoverTooltipShown {
		names = append(names, name)
	}
	c.tooltipMu.Unlock()
	c.dismissOnUI(names)
}

// dismissOnUI closes overlays before their native owner is hidden or destroyed.
func (c *tooltipController) dismissOnUI(names []string) {
	c.tooltipMu.Lock()
	for _, name := range names {
		delete(c.nativeHoverTooltipShown, name)
	}
	c.tooltipMu.Unlock()
	if c.services == nil {
		return
	}
	for _, name := range names {
		if err := c.services.HideTooltip(context.Background(), c.sessionID, name); err != nil {
			util.GetLogger().Warn(c.lifecycleCtx, "hide "+name+" tooltip: "+err.Error())
		}
	}
}
