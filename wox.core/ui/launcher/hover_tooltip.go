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
	"wox/util/mouse"
)

// nativeHoverTooltipDelay is the shared dwell before hover help or a demo overlay appears.
// Passing through a control should not flash help.
const nativeHoverTooltipDelay = 300 * time.Millisecond

// nativeHoverTooltipExplicitDismiss reports a caller-owned hide, such as window
// close. Ordinary pointer-leave is not a dismiss: showing the topmost tooltip
// HWND generates WM_MOUSELEAVE on the owner window, which would flash-dismiss
// the tooltip while the cursor is still on the trigger. util/tooltip tracking
// closes the overlay once the cursor leaves both the anchor and the panel.
func nativeHoverTooltipExplicitDismiss(inside bool, text string, anchor woxui.Rect) bool {
	return !inside && (strings.TrimSpace(text) == "" || anchor.Width <= 0 || anchor.Height <= 0)
}

func nativeHoverTooltipArmed(inside bool, text string, anchor woxui.Rect) bool {
	return inside && strings.TrimSpace(text) != "" && anchor.Width > 0 && anchor.Height > 0
}

// nativeHoverTooltipOSCursorOnAnchor reports whether the real OS pointer is on
// the trigger. Sample at hover-arm time: leftover user hovers have already
// left the anchor by show time, and must still use leftover-owner dismiss.
func nativeHoverTooltipOSCursorOnAnchor(windowFn func() *woxui.Window, anchor woxui.Rect) bool {
	if windowFn == nil {
		return false
	}
	window := windowFn()
	if window == nil {
		return false
	}
	windowBounds, err := window.Bounds()
	if err != nil {
		return false
	}
	point, ok := mouse.CurrentPosition()
	if !ok {
		return false
	}
	return nativeHoverTooltipPointOnAnchor(point.X, point.Y, windowBounds, anchor)
}

func nativeHoverTooltipPointOnAnchor(x, y float64, windowBounds woxui.Rect, anchor woxui.Rect) bool {
	left := float64(windowBounds.X + anchor.X)
	top := float64(windowBounds.Y + anchor.Y)
	return x >= left && x < left+float64(anchor.Width) && y >= top && y < top+float64(anchor.Height)
}

// nativeHoverTooltipIdentity remembers the last shown trigger for one overlay name.
type nativeHoverTooltipIdentity struct {
	text   string
	anchor woxui.Rect
}

// nativeHoverTooltipShouldReplace reports whether a new hover target should
// dismiss an already visible tooltip for the same overlay name. The same
// trigger can re-enter after the show-induced owner leave; hiding that would
// flash the hint while the pointer is still on it.
func nativeHoverTooltipShouldReplace(shown bool, shownText string, shownAnchor woxui.Rect, nextText string, nextAnchor woxui.Rect) bool {
	if !shown {
		return false
	}
	return shownText != nextText || shownAnchor != nextAnchor
}

// waitHoverTooltipDelay waits for the shared dwell unless a newer hover revision cancelled it.
func (a *App) waitHoverTooltipDelay(revision *atomic.Uint64, revisionID uint64) bool {
	timer := time.NewTimer(nativeHoverTooltipDelay)
	defer timer.Stop()
	select {
	case <-a.lifecycleCtx.Done():
		return false
	case <-timer.C:
	}
	return revisionID == revision.Load()
}

// setNativeHoverTooltip shows one named overlay tooltip after a hover dwell, or
// hides it only on an explicit dismiss such as window close. Ordinary leave
// cancels a pending dwell without closing an already visible overlay.
// Hovering a different control hides the previous hint immediately so the
// delayed show cannot leave the old tooltip stuck on the prior trigger.
func (a *App) setNativeHoverTooltip(revision *atomic.Uint64, name, job string, inside bool, text string, anchor woxui.Rect, side string, windowFn func() *woxui.Window) {
	if nativeHoverTooltipArmed(inside, text, anchor) {
		revisionID := revision.Add(1)
		if a.nativeHoverTooltipNeedsReplace(name, text, anchor) {
			a.hideNativeHoverTooltip(name, job)
		}
		ignoreOwnerLeave := !nativeHoverTooltipOSCursorOnAnchor(windowFn, anchor)
		util.Go(a.lifecycleCtx, job, func() {
			if !a.waitHoverTooltipDelay(revision, revisionID) {
				return
			}
			a.showNativeHoverTooltip(revision, revisionID, name, text, anchor, side, windowFn, ignoreOwnerLeave)
		})
		return
	}

	revision.Add(1)
	if !nativeHoverTooltipExplicitDismiss(false, text, anchor) {
		return
	}
	a.hideNativeHoverTooltip(name, job)
}

// nativeHoverTooltipNeedsReplace reports whether this name already shows a different trigger.
func (a *App) nativeHoverTooltipNeedsReplace(name, text string, anchor woxui.Rect) bool {
	a.tooltipMu.Lock()
	defer a.tooltipMu.Unlock()
	previous, shown := a.nativeHoverTooltipShown[name]
	return nativeHoverTooltipShouldReplace(shown, previous.text, previous.anchor, text, anchor)
}

// hideNativeHoverTooltip closes one named overlay and forgets its last shown trigger.
func (a *App) hideNativeHoverTooltip(name, job string) {
	util.Go(a.lifecycleCtx, job, func() {
		a.tooltipMu.Lock()
		defer a.tooltipMu.Unlock()
		delete(a.nativeHoverTooltipShown, name)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := a.services.HideTooltip(ctx, a.sessionID, name); err != nil {
			log.Printf("hide %s tooltip: %v", name, err)
		}
	})
}

func (a *App) showNativeHoverTooltip(revision *atomic.Uint64, revisionID uint64, name, text string, anchor woxui.Rect, side string, windowFn func() *woxui.Window, ignoreOwnerLeave bool) {
	a.tooltipMu.Lock()
	defer a.tooltipMu.Unlock()
	if revisionID != revision.Load() {
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.services.ShowTooltip(ctx, a.sessionID, contract.TooltipOptions{
		Name: name, Text: text, Side: side,
		AnchorX: float64(windowBounds.X + anchor.X), AnchorY: float64(windowBounds.Y + anchor.Y),
		AnchorWidth: float64(anchor.Width), AnchorHeight: float64(anchor.Height),
		OwnerX: float64(windowBounds.X), OwnerY: float64(windowBounds.Y),
		OwnerWidth: float64(windowBounds.Width), OwnerHeight: float64(windowBounds.Height),
		IgnoreOwnerLeave: ignoreOwnerLeave,
	}); err != nil {
		log.Printf("show %s tooltip: %v", name, err)
		return
	}
	if a.nativeHoverTooltipShown == nil {
		a.nativeHoverTooltipShown = map[string]nativeHoverTooltipIdentity{}
	}
	a.nativeHoverTooltipShown[name] = nativeHoverTooltipIdentity{text: text, anchor: anchor}
}
