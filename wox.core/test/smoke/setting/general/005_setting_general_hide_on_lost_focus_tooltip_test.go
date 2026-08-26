//go:build wox_ui_smoke

package general

import (
	"context"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

const previewTagTooltipOverlay = "overlay.go-ui-preview-tag"

// Test005SettingGeneralHideOnLostFocusTooltip verifies that Hide on focus loss does not dismiss the launcher when a native tooltip appears.
// Flow: enable Hide on focus loss -> query the tooltip fixture -> hover its preview tag.
// Evidence: the native tooltip overlay becomes visible while the primary launcher stays visible.
func Test005SettingGeneralHideOnLostFocusTooltip(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		previousValue := smoke.OpenGeneralSettingsAndReadSwitch(t, ctx, client, "HideOnLostFocus")
		smoke.SetSettingSwitch(t, ctx, client, "HideOnLostFocus", true)
		t.Cleanup(func() {
			if !previousValue {
				smoke.RestoreGeneralSettingSwitch(t, client, "HideOnLostFocus", previousValue)
			}
		})
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close General settings after enabling focus-loss hiding: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible && state.BlurReady && state.Lifecycle == "visible"
		}); err != nil {
			t.Fatalf("wait for visible primary launcher: %v", err)
		}

		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke tooltip ")
		if _, found := smoke.FindLauncherResult(snapshot, "Tooltip smoke fixture"); !found {
			t.Fatal("tooltip smoke fixture result was not found")
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "preview-tag-0")
			return found
		})
		if err != nil {
			t.Fatalf("wait for preview tag tooltip target: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		waitForPreviewTagTooltip(t, ctx, client)
		assertLauncherStaysVisibleAfterTooltip(t, ctx, client)
	})
}

// waitForPreviewTagTooltip re-arms the synthetic hover until the overlay stays visible.
// A query rebuild can cancel the 300ms dwell, so one MovePointerTo is not enough.
func waitForPreviewTagTooltip(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, automationdriver.ActionTimeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	rearmAt := time.Time{}
	var last automationdriver.WindowState
	for {
		if rearmAt.IsZero() || !time.Now().Before(rearmAt) {
			if _, err := client.MovePointerTo(ctx, "preview-tag-0"); err != nil {
				t.Fatalf("hover preview tag: %v", err)
			}
			rearmAt = time.Now().Add(400 * time.Millisecond)
		}
		state, err := client.WindowState(ctx, previewTagTooltipOverlay)
		if err != nil {
			if ctx.Err() != nil {
				t.Fatalf("wait for native preview tag tooltip: %v; last state: %+v", err, last)
			}
		} else {
			last = state
			if state.Exists && state.Visible && state.Lifecycle == "visible" {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for native preview tag tooltip: %v; last state: %+v", ctx.Err(), last)
		case <-ticker.C:
		}
	}
}

// assertLauncherStaysVisibleAfterTooltip watches the async hide-on-lost-focus path after a tooltip has already appeared.
func assertLauncherStaysVisibleAfterTooltip(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		launcher, err := client.WindowState(ctx, "primary")
		if err != nil {
			t.Fatalf("read primary launcher after tooltip hover: %v", err)
		}
		if !launcher.Exists || !launcher.Visible || launcher.Lifecycle != "visible" {
			t.Fatalf("launcher hid after hovering a tooltip: %+v", launcher)
		}
		if !time.Now().Before(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("watch launcher after tooltip hover: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
