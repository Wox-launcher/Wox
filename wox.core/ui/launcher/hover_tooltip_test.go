package launcher

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

type reentrantTooltipServices struct {
	contract.Services
	app        *App
	hideCalled chan struct{}
}

func (s *reentrantTooltipServices) ShowTooltip(_ context.Context, _ string, options contract.TooltipOptions) error {
	s.app.nativeHoverTooltipNeedsReplace(options.Name, options.Text, woxui.Rect{Width: 1, Height: 1})
	return nil
}

func (s *reentrantTooltipServices) HideTooltip(_ context.Context, _ string, name string) error {
	s.app.nativeHoverTooltipNeedsReplace(name, "next", woxui.Rect{Width: 1, Height: 1})
	close(s.hideCalled)
	return nil
}

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

type recordingTooltipServices struct {
	contract.Services
	hidden []string
}

func (s *recordingTooltipServices) HideTooltip(_ context.Context, _ string, name string) error {
	s.hidden = append(s.hidden, name)
	return nil
}

func TestDismissLauncherHoverTooltipsClosesNamedOverlaysOnUITurn(t *testing.T) {
	services := &recordingTooltipServices{}
	app := &App{
		services: services,
		nativeHoverTooltipShown: map[string]nativeHoverTooltipIdentity{
			"go-ui-preview-tag": {text: "Tag", anchor: woxui.Rect{Width: 10, Height: 10}},
			"go-ui-glance":      {text: "CPU", anchor: woxui.Rect{Width: 10, Height: 10}},
		},
	}
	app.previewTooltipRevision.Store(4)
	app.dismissLauncherHoverTooltipsOnUI()
	if app.previewTooltipRevision.Load() != 5 {
		t.Fatalf("preview tooltip revision = %d, want 5 so a pending dwell cannot show after hide", app.previewTooltipRevision.Load())
	}
	if _, ok := app.nativeHoverTooltipShown["go-ui-preview-tag"]; ok {
		t.Fatal("dismiss must forget the preview tooltip trigger")
	}
	if _, ok := app.nativeHoverTooltipShown["go-ui-glance"]; ok {
		t.Fatal("dismiss must forget the glance tooltip trigger")
	}
	if len(services.hidden) != len(launcherHoverTooltipNames) {
		t.Fatalf("hidden = %v, want %v", services.hidden, launcherHoverTooltipNames)
	}
	for i, name := range launcherHoverTooltipNames {
		if services.hidden[i] != name {
			t.Fatalf("hidden[%d] = %q, want %q", i, services.hidden[i], name)
		}
	}
}

func TestHideWindowDismissesLauncherTooltipsOnTheSameUITurn(t *testing.T) {
	services := &recordingTooltipServices{}
	app := &App{
		isPrimary: true,
		visible:   true,
		services:  services,
		nativeHoverTooltipShown: map[string]nativeHoverTooltipIdentity{
			"go-ui-preview-tag": {text: "Tag", anchor: woxui.Rect{Width: 8, Height: 8}},
		},
	}
	err := app.hideWindow(false)
	if err == nil || err.Error() != "launcher window is not initialized" {
		t.Fatalf("hideWindow error = %v, want missing native window", err)
	}
	if app.visible {
		t.Fatal("hide must clear visible before returning the missing-window error")
	}
	if _, ok := app.nativeHoverTooltipShown["go-ui-preview-tag"]; ok {
		t.Fatal("hide must close launcher tooltips before gtk_widget_hide")
	}
	if len(services.hidden) != len(launcherHoverTooltipNames) {
		t.Fatalf("hidden = %v, want launcher overlay names closed on the hide turn", services.hidden)
	}
}

func TestNativeHoverTooltipServiceCallsAllowReentrantStateAccess(t *testing.T) {
	services := &reentrantTooltipServices{hideCalled: make(chan struct{})}
	app := &App{lifecycleCtx: t.Context(), services: services}
	services.app = app

	var revision atomic.Uint64
	revisionID := revision.Add(1)
	showDone := make(chan struct{})
	go func() {
		app.showNativeHoverTooltipAtBounds(
			&revision, revisionID, "tooltip", "Help", woxui.Rect{Width: 20, Height: 20}, "top",
			woxui.Rect{X: 100, Y: 100, Width: 400, Height: 300}, false,
		)
		close(showDone)
	}()
	select {
	case <-showDone:
	case <-time.After(time.Second):
		t.Fatal("show tooltip deadlocked during reentrant state access")
	}

	app.hideNativeHoverTooltip("tooltip", "hide tooltip in test")
	select {
	case <-services.hideCalled:
	case <-time.After(time.Second):
		t.Fatal("hide tooltip deadlocked during reentrant state access")
	}
}
