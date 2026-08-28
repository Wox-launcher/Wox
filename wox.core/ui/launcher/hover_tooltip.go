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

// linuxInlineTooltipTarget is the owner-window paint path used when native
// overlay windows cannot be positioned. Linux cannot control tooltip window
// placement, so the same hover dwell paints SettingsInlineTooltipOverlay inside the owner.
type linuxInlineTooltipTarget struct {
	revision   *atomic.Uint64
	state      **settingsInlineTooltipState
	open       bool
	invalidate func()
	job        string
}

// scheduleLinuxInlineTooltip paints one in-window tooltip after the shared hover dwell.
func (a *App) scheduleLinuxInlineTooltip(target linuxInlineTooltipTarget, inside bool, text string, anchor woxui.Rect, side string) {
	if target.revision == nil || target.state == nil {
		return
	}
	job := strings.TrimSpace(target.job)
	if job == "" {
		job = "show linux inline tooltip"
	}
	if !target.open {
		target.revision.Add(1)
		a.clearLinuxInlineTooltipState(target.state, target.invalidate)
		return
	}
	message := strings.TrimSpace(text)
	if !nativeHoverTooltipArmed(inside, message, anchor) {
		target.revision.Add(1)
		a.clearLinuxInlineTooltipState(target.state, target.invalidate)
		return
	}
	next := settingsInlineTooltipState{Text: message, Side: side, Anchor: anchor}
	if current := *target.state; current != nil && current.Text == next.Text && current.Side == next.Side && current.Anchor == next.Anchor {
		return
	}
	revisionID := target.revision.Add(1)
	util.Go(a.lifecycleCtx, job+" after dwell", func() {
		if !a.waitHoverTooltipDelay(target.revision, revisionID) {
			return
		}
		apply := func() {
			if revisionID != target.revision.Load() {
				return
			}
			if current := *target.state; current != nil && current.Text == next.Text && current.Side == next.Side && current.Anchor == next.Anchor {
				return
			}
			*target.state = &next
			if target.invalidate != nil {
				target.invalidate()
			}
		}
		if err := a.runOnUI(job, apply); err != nil {
			apply()
		}
	})
}

func (a *App) clearLinuxInlineTooltipState(state **settingsInlineTooltipState, invalidate func()) {
	if state == nil || *state == nil {
		return
	}
	*state = nil
	if invalidate != nil {
		invalidate()
	}
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

// launcherHoverTooltipNames are the overlay IDs owned by the launcher query window.
var launcherHoverTooltipNames = []string{
	"go-ui-preview-tag",
	"go-ui-glance",
	"go-ui-refinement",
	"go-ui-titlebar-action",
	"go-ui-result-tail",
}

// dismissLauncherHoverTooltipsOnUI closes launcher overlay tooltips on the current
// UI turn. Async HideTooltip from hideWindow raced gtk_widget_hide and SIGSEGV'd
// GTK on KDE Wayland when Notes was also open.
func (a *App) dismissLauncherHoverTooltipsOnUI() {
	a.previewTooltipRevision.Add(1)
	a.resultTailTooltipRevision.Add(1)
	a.glanceTooltipRevision.Add(1)
	a.refinementTooltipRevision.Add(1)
	a.tooltipMu.Lock()
	for _, name := range launcherHoverTooltipNames {
		delete(a.nativeHoverTooltipShown, name)
	}
	a.tooltipMu.Unlock()
	if a.services == nil {
		return
	}
	for _, name := range launcherHoverTooltipNames {
		if err := a.services.HideTooltip(context.Background(), a.sessionID, name); err != nil {
			log.Printf("hide %s tooltip: %v", name, err)
		}
	}
}

// hideNativeHoverTooltip closes one named overlay and forgets its last shown trigger.
func (a *App) hideNativeHoverTooltip(name, job string) {
	util.Go(a.lifecycleCtx, job, func() {
		// Service calls can synchronously enter the native UI thread and emit
		// another hover callback. Serialize them separately from tooltip state so
		// that callback never waits on a lock owned by the calling goroutine.
		a.tooltipCallMu.Lock()
		defer a.tooltipCallMu.Unlock()
		a.tooltipMu.Lock()
		delete(a.nativeHoverTooltipShown, name)
		a.tooltipMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := a.services.HideTooltip(ctx, a.sessionID, name); err != nil {
			log.Printf("hide %s tooltip: %v", name, err)
		}
	})
}

func (a *App) showNativeHoverTooltip(revision *atomic.Uint64, revisionID uint64, name, text string, anchor woxui.Rect, side string, windowFn func() *woxui.Window, ignoreOwnerLeave bool) {
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
	a.showNativeHoverTooltipAtBounds(revision, revisionID, name, text, anchor, side, windowBounds, ignoreOwnerLeave)
}

// showNativeHoverTooltipAtBounds presents one tooltip without holding the state lock across the synchronous UI boundary.
func (a *App) showNativeHoverTooltipAtBounds(revision *atomic.Uint64, revisionID uint64, name, text string, anchor woxui.Rect, side string, windowBounds woxui.Rect, ignoreOwnerLeave bool) {
	a.tooltipCallMu.Lock()
	defer a.tooltipCallMu.Unlock()
	if revisionID != revision.Load() {
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
	a.tooltipMu.Lock()
	defer a.tooltipMu.Unlock()
	if a.nativeHoverTooltipShown == nil {
		a.nativeHoverTooltipShown = map[string]nativeHoverTooltipIdentity{}
	}
	a.nativeHoverTooltipShown[name] = nativeHoverTooltipIdentity{text: text, anchor: anchor}
}
