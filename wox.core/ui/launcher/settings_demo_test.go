package launcher

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

// Failed UI dispatch must leave delayed overlays untouched by the background worker.
func TestDelayedOverlaysDoNotApplyAfterDispatchFailure(t *testing.T) {
	for _, kind := range []string{"demo", "cloud", "inline"} {
		t.Run(kind, func(t *testing.T) {
			app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
			defer app.cancel()
			var dispatched atomic.Bool
			app.uiCall = func(func()) error {
				dispatched.Store(true)
				return errors.New("UI dispatcher unavailable")
			}
			var inline *settingsInlineTooltipState
			var revision atomic.Uint64
			anchor := woxui.Rect{Width: 18, Height: 18}
			switch kind {
			case "demo":
				app.setSettingsDemoHover("query-hotkeys", true, anchor)
			case "cloud":
				app.setCloudPlanTooltip(true, anchor)
			case "inline":
				app.scheduleLinuxInlineTooltip(linuxInlineTooltipTarget{revision: &revision, state: &inline, open: true}, true, "help", anchor, "bottom")
			}
			time.Sleep(nativeHoverTooltipDelay + 100*time.Millisecond)
			if !dispatched.Load() {
				t.Fatal("delayed overlay did not attempt UI dispatch")
			}
			if app.settingsDemo != nil || app.cloudPlanTooltip != nil || inline != nil {
				t.Fatal("failed UI dispatch applied overlay state on the worker")
			}
		})
	}
}

func TestSetSettingsDemoHoverWaitsForDwell(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	queued := make(chan func(), 1)
	app.uiCall = func(fn func()) error { queued <- fn; return nil }
	app.themeSettings.SetThemeWallpaperImage(&woxui.Image{})
	app.themeSettings.SetThemeWallpaperBlurred(&woxui.Image{})

	anchor := woxui.Rect{X: 240, Y: 120, Width: 18, Height: 18}
	app.setSettingsDemoHover("query-hotkeys", true, anchor)
	if app.settingsDemo != nil {
		t.Fatal("settings demo must wait for the shared hover dwell")
	}

	select {
	case apply := <-queued:
		app.uiCall = nil
		apply()
	case <-time.After(nativeHoverTooltipDelay + time.Second):
		t.Fatal("settings demo did not dispatch after dwell")
	}
	if app.settingsDemo == nil {
		t.Fatal("expected settings demo after the hover dwell")
	}
	if app.settingsDemo.kind != "query-hotkeys" || app.settingsDemo.anchor != anchor {
		t.Fatalf("settings demo = %+v, want query-hotkeys at %+v", app.settingsDemo, anchor)
	}
}

func TestSetSettingsDemoHoverCancelBeforeDwell(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()

	app.setSettingsDemoHover("query-hotkeys", true, woxui.Rect{X: 240, Y: 120, Width: 18, Height: 18})
	app.setSettingsDemoHover("query-hotkeys", false, woxui.Rect{})
	time.Sleep(nativeHoverTooltipDelay + 80*time.Millisecond)
	if app.settingsDemo != nil {
		t.Fatalf("settings demo = %+v, want nil after leaving before the dwell", app.settingsDemo)
	}
}

func TestSetCloudPlanTooltipWaitsForDwell(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	queued := make(chan func(), 1)
	app.uiCall = func(fn func()) error { queued <- fn; return nil }

	anchor := woxui.Rect{X: 80, Y: 40, Width: 16, Height: 16}
	app.setCloudPlanTooltip(true, anchor)
	if app.cloudPlanTooltip != nil {
		t.Fatal("cloud plan tooltip must wait for the shared hover dwell")
	}

	select {
	case apply := <-queued:
		app.uiCall = nil
		apply()
	case <-time.After(nativeHoverTooltipDelay + time.Second):
		t.Fatal("cloud tooltip did not dispatch after dwell")
	}
	if app.cloudPlanTooltip == nil {
		t.Fatal("expected cloud plan tooltip after the hover dwell")
	}
	if app.cloudPlanTooltip.anchor != anchor {
		t.Fatalf("cloud plan tooltip anchor = %+v, want %+v", app.cloudPlanTooltip.anchor, anchor)
	}

	app.setCloudPlanTooltip(false, woxui.Rect{})
	if app.cloudPlanTooltip != nil {
		t.Fatalf("cloud plan tooltip = %+v, want nil after leave", app.cloudPlanTooltip)
	}
}
