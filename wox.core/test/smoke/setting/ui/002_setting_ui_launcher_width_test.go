//go:build wox_ui_smoke

package ui

import (
	"context"
	"math"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test002SettingUILauncherWidth verifies that the UI launcher-width choice controls the native launcher window.
// Flow: select 600 and show the launcher -> select 800 and show the launcher again.
// Evidence: each real native launcher bounds width matches the selected logical width.
func Test002SettingUILauncherWidth(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previousValue := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "AppWidth")
		t.Cleanup(func() {
			if previousValue != "800" {
				smoke.RestoreSettingChoice(t, client, "/appearance", "AppWidth", previousValue)
			}
		})

		setLauncherWidth(t, ctx, client, "600")
		smoke.ShowLauncher(t, ctx, client)
		waitForLauncherWidth(t, ctx, client, 600)
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher after verifying 600px width: %v", err)
		}

		setLauncherWidth(t, ctx, client, "800")
		smoke.ShowLauncher(t, ctx, client)
		waitForLauncherWidth(t, ctx, client, 800)
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read launcher semantics after verifying 800px width: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// setLauncherWidth applies one UI width option through the native Settings section.
func setLauncherWidth(t *testing.T, ctx context.Context, client *automationdriver.Client, width string) {
	t.Helper()
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "AppWidth")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-AppWidth", width)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close UI settings after selecting %spx launcher width: %v", width, err)
	}
}

// waitForLauncherWidth waits for native window resize to consume the selected launcher width.
func waitForLauncherWidth(t *testing.T, ctx context.Context, client *automationdriver.Client, expected float32) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read launcher bounds while waiting for %.0fpx width: %v", expected, err)
		}
		if math.Abs(float64(bounds.Width-expected)) <= 1 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for launcher width %.0fpx: last width %.1f; %v", expected, bounds.Width, ctx.Err())
		case <-ticker.C:
		}
	}
}
