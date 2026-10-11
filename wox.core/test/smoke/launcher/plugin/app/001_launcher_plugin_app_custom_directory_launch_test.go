//go:build wox_ui_smoke

package app

import (
	"context"
	"path/filepath"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test001LauncherPluginAppCustomDirectoryLaunch verifies an application added through App Directories can be found and launched.
// Flow: create a local launcher in a temporary directory -> add that directory in App settings -> query its name -> activate the result -> remove the directory.
// Evidence: the launcher writes its marker file, and the same query no longer returns the application after the directory is removed.
func Test001LauncherPluginAppCustomDirectoryLaunch(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		dir := smokeAppFixtureDirectory(t)
		marker := filepath.Join(dir, "launched.txt")
		state := &smokeAppLaunchState{}
		t.Cleanup(func() { cleanupSmokeAppLaunch(t, client, dir, state) })
		createSmokeAppFixture(t, dir, marker)

		logPath := smokeDataLogPath(t)
		openAppDirectorySettings(t, ctx, client)
		// Count completions before the delete. An update already in flight can log once
		// before it sees that the directory is gone.
		beforeLeak := appDirectoryUpdateCount(logPath)
		if removed := removeSmokeAppDirectoryRows(t, ctx, client, dir); removed > 0 {
			if err := client.Hide(ctx); err != nil {
				t.Fatalf("hide after clearing a leaked app directory: %v", err)
			}
			waitUntilSmokeApp(t, ctx, client, logPath, beforeLeak, dir, false)
			openAppDirectorySettings(t, ctx, client)
		}

		before := appDirectoryUpdateCount(logPath)
		state.directoryPending = true
		addSmokeAppDirectory(t, ctx, client, dir)
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide after adding app directory: %v", err)
		}
		resultID := waitUntilSmokeApp(t, ctx, client, logPath, before, dir, true)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("launch %s: %v", smokeAppName, err)
		}
		state.launched = true
		waitForLaunchMarker(t, ctx, marker)

		removeSmokeAppDirectoryAndWait(t, ctx, client, dir)
		state.directoryPending = false
	})
}
