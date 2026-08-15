package launcher

import (
	"context"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	"wox/util"
)

// nativeHoverTooltipExplicitDismiss reports a caller-owned hide, such as window
// close. Ordinary pointer-leave is not a dismiss: showing the topmost tooltip
// HWND generates WM_MOUSELEAVE on the owner window, which would flash-dismiss
// the tooltip while the cursor is still on the trigger. util/tooltip tracking
// closes the overlay once the cursor leaves both the anchor and the panel.
func nativeHoverTooltipExplicitDismiss(inside bool, text string, anchor woxui.Rect) bool {
	return !inside && (strings.TrimSpace(text) == "" || anchor.Width <= 0 || anchor.Height <= 0)
}

// setNativeHoverTooltip shows one named overlay tooltip, or hides it only on an
// explicit dismiss such as window close.
func (a *App) setNativeHoverTooltip(revision *atomic.Uint64, name, job string, inside bool, text string, anchor woxui.Rect, side string, windowFn func() *woxui.Window) {
	if !inside && !nativeHoverTooltipExplicitDismiss(false, text, anchor) {
		return
	}
	revisionID := revision.Add(1)
	util.Go(a.lifecycleCtx, job, func() {
		a.tooltipMu.Lock()
		defer a.tooltipMu.Unlock()
		if revisionID != revision.Load() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if nativeHoverTooltipExplicitDismiss(inside, text, anchor) {
			if err := a.services.HideTooltip(ctx, a.sessionID, name); err != nil {
				log.Printf("hide %s tooltip: %v", name, err)
			}
			return
		}
		window := windowFn()
		if window == nil {
			return
		}
		windowBounds, err := window.Bounds()
		if err != nil {
			log.Printf("read bounds for %s tooltip: %v", name, err)
			return
		}
		if err := a.services.ShowTooltip(ctx, a.sessionID, contract.TooltipOptions{
			Name: name, Text: text, Side: side,
			AnchorX: float64(windowBounds.X + anchor.X), AnchorY: float64(windowBounds.Y + anchor.Y),
			AnchorWidth: float64(anchor.Width), AnchorHeight: float64(anchor.Height),
		}); err != nil {
			log.Printf("show %s tooltip: %v", name, err)
		}
	})
}
