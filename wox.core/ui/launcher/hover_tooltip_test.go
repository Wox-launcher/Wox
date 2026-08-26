package launcher

import (
	"sync/atomic"
	"testing"

	woxui "wox/ui/runtime"
)

func TestNativeHoverTooltipIgnoresOwnerPointerLeave(t *testing.T) {
	anchor := woxui.Rect{X: 10, Y: 20, Width: 80, Height: 26}
	if nativeHoverTooltipExplicitDismiss(false, "Copied at", anchor) {
		t.Fatal("pointer-leave from the owner window must not dismiss the native tooltip")
	}
	if nativeHoverTooltipExplicitDismiss(true, "Copied at", anchor) {
		t.Fatal("hover enter must show rather than dismiss")
	}
	if !nativeHoverTooltipArmed(true, "Copied at", anchor) {
		t.Fatal("hover enter with text and an anchor must arm the dwell")
	}
	if nativeHoverTooltipArmed(false, "Copied at", anchor) {
		t.Fatal("ordinary leave must cancel the dwell instead of showing")
	}
}

func TestNativeHoverTooltipDismissesOnExplicitClose(t *testing.T) {
	if !nativeHoverTooltipExplicitDismiss(false, "", woxui.Rect{}) {
		t.Fatal("empty close from window hide must dismiss the native tooltip")
	}
	if !nativeHoverTooltipExplicitDismiss(false, "Copied at", woxui.Rect{}) {
		t.Fatal("leave without an anchor must dismiss the native tooltip")
	}
}

func TestNativeHoverTooltipReplacesADifferentVisibleTrigger(t *testing.T) {
	shown := woxui.Rect{X: 200, Y: 8, Width: 28, Height: 28}
	next := woxui.Rect{X: 10, Y: 8, Width: 28, Height: 28}
	if !nativeHoverTooltipShouldReplace(true, "更多", shown, "搜索", next) {
		t.Fatal("moving onto another control must hide the previous delayed tooltip")
	}
	if nativeHoverTooltipShouldReplace(true, "更多", shown, "更多", shown) {
		t.Fatal("re-entering the same trigger after owner leave must not flash-hide")
	}
	if nativeHoverTooltipShouldReplace(false, "", woxui.Rect{}, "搜索", next) {
		t.Fatal("a first hover has no previous tooltip to replace")
	}
}

func TestWaitHoverTooltipDelayRespectsCancellation(t *testing.T) {
	app := &App{lifecycleCtx: t.Context()}
	var revision atomic.Uint64
	revisionID := revision.Add(1)
	revision.Add(1)
	if app.waitHoverTooltipDelay(&revision, revisionID) {
		t.Fatal("a newer hover revision must cancel the pending tooltip dwell")
	}
}

func TestNativeHoverTooltipPointOnAnchor(t *testing.T) {
	window := woxui.Rect{X: 100, Y: 200, Width: 400, Height: 300}
	anchor := woxui.Rect{X: 10, Y: 20, Width: 80, Height: 26}
	if !nativeHoverTooltipPointOnAnchor(110, 220, window, anchor) {
		t.Fatal("cursor on the trigger must count as an OS hover")
	}
	if nativeHoverTooltipPointOnAnchor(150, 250, window, anchor) {
		t.Fatal("cursor idle elsewhere on the owner must not count as an OS hover")
	}
	if nativeHoverTooltipOSCursorOnAnchor(nil, anchor) {
		t.Fatal("a missing window must not count as an OS hover")
	}
	if nativeHoverTooltipOSCursorOnAnchor(func() *woxui.Window { return nil }, anchor) {
		t.Fatal("a nil window must not count as an OS hover")
	}
}

func TestWaitHoverTooltipDelayCompletesForCurrentRevision(t *testing.T) {
	app := &App{lifecycleCtx: t.Context()}
	var revision atomic.Uint64
	revisionID := revision.Add(1)
	if !app.waitHoverTooltipDelay(&revision, revisionID) {
		t.Fatal("an uncancelled dwell must show after the shared delay")
	}
}
