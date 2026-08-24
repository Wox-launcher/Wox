package launcher

import (
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

func TestSetSettingsDemoHoverWaitsForDwell(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.themeSettings.SetThemeWallpaperImage(&woxui.Image{})
	app.themeSettings.SetThemeWallpaperBlurred(&woxui.Image{})

	anchor := woxui.Rect{X: 240, Y: 120, Width: 18, Height: 18}
	app.setSettingsDemoHover("query-hotkeys", true, anchor)
	if app.settingsDemo != nil {
		t.Fatal("settings demo must wait for the shared hover dwell")
	}

	deadline := time.Now().Add(nativeHoverTooltipDelay + 300*time.Millisecond)
	for app.settingsDemo == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
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

	anchor := woxui.Rect{X: 80, Y: 40, Width: 16, Height: 16}
	app.setCloudPlanTooltip(true, anchor)
	if app.cloudPlanTooltip != nil {
		t.Fatal("cloud plan tooltip must wait for the shared hover dwell")
	}

	deadline := time.Now().Add(nativeHoverTooltipDelay + 300*time.Millisecond)
	for app.cloudPlanTooltip == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
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
