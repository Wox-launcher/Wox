//go:build wox_ui_smoke

package query

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test036LauncherToastAction verifies an independent notification invokes its action with Wox visible or hidden.
// Flow: show a fixture toast -> activate its button -> repeat after hiding Wox.
// Evidence: the toast closes and its callback opens the expected query with its context intact.
func Test036LauncherToastAction(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		for _, mode := range []string{"visible", "hidden", "long"} {
			if err := client.FocusInstance(ctx, "primary"); err != nil {
				t.Fatal(err)
			}
			smoke.ShowLauncher(t, ctx, client)
			snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "wox-smoke toast "+mode)
			id, found := smoke.FindLauncherResult(snapshot, "Show toast fixture")
			if !found {
				t.Fatalf("%s fixture not found", mode)
			}
			if err := client.Perform(ctx, id, woxui.AccessibilityActionActivate, ""); err != nil {
				t.Fatalf("%s show toast: %v", mode, err)
			}
			if _, err := client.WaitForWindowState(ctx, "overlay.wox_notifier", func(state automationdriver.WindowState) bool { return state.Exists && state.Visible }); err != nil {
				t.Fatal(err)
			}
			if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool { return state.Visible == (mode != "hidden") }); err != nil {
				t.Fatalf("%s launcher visibility: %v", mode, err)
			}
			if err := client.FocusInstance(ctx, "overlay.wox_notifier"); err != nil {
				t.Fatal(err)
			}
			actionLabel := "Open fixture"
			if mode == "long" {
				actionLabel = "View receive history"
			}
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				button, found := automationdriver.Find(snapshot, "toast-action")
				return found && button.Label == actionLabel && button.Role == woxui.AccessibilityRoleButton
			})
			if err != nil {
				t.Fatalf("%s toast button: %v", mode, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
			if directory := os.Getenv("WOX_SMOKE_ARTIFACT_DIR"); directory != "" {
				if err := client.Capture(ctx, filepath.Join(directory, "toast-"+mode+".png")); err != nil {
					t.Fatalf("capture %s toast: %v", mode, err)
				}
			}
			if err := client.Perform(ctx, "toast-action", woxui.AccessibilityActionActivate, ""); err != nil {
				t.Fatalf("%s activate toast: %v", mode, err)
			}
			if _, err := client.WaitForWindowState(ctx, "overlay.wox_notifier", func(state automationdriver.WindowState) bool { return !state.Exists }); err != nil {
				t.Fatal(err)
			}
			if err := client.FocusInstance(ctx, "primary"); err != nil {
				t.Fatal(err)
			}
			if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool { return state.Visible }); err != nil {
				t.Fatal(err)
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				_, found := smoke.FindLauncherResult(snapshot, "Toast action completed")
				return found
			}); err != nil {
				t.Fatalf("%s callback result: %v", mode, err)
			}
		}
	})
}
