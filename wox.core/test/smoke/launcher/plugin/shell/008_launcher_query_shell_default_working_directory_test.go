//go:build wox_ui_smoke

package shell

import (
	"context"
	"os"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test008LauncherQueryShellDefaultWorkingDirectory verifies each default-directory mode changes later Shell queries.
// Flow: home mode -> custom path A then B -> last-used mode after A was executed.
// Evidence: preview tags and command output/files follow home, then B, then A.
func Test008LauncherQueryShellDefaultWorkingDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatal("user home directory is required for the home-mode contract")
	}
	lastUsedDirectory := t.TempDir()
	customDirectory := t.TempDir()
	if sameShellWorkingDirectory(lastUsedDirectory, customDirectory) {
		t.Fatal("last-used and custom smoke directories must differ")
	}

	homeCommand := shellPrintWorkingDirectoryCommand()
	lastUsedSeedCommand := shellWriteCwdMarkerCommand("wox-cwd-last-used-seed.txt")
	lastUsedCommand := shellWriteCwdMarkerCommand("wox-cwd-last-used-run.txt")

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		defer restoreShellHomeWorkingDirectory(t, client)

		homeLabel := selectShellWorkingDirectoryMode(t, ctx, client, shellHomeWorkingDirectoryOption)
		confirmShellWorkingDirectoryModePersisted(t, ctx, client, homeLabel, false)
		queryShellWorkingDirectory(t, ctx, client, "> ", home)
		queryShellWorkingDirectory(t, ctx, client, "> "+homeCommand, home)
		if err := client.Perform(ctx, waitForShellResult(t, ctx, client, homeCommand), woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute home-directory Shell command: %v", err)
		}
		waitForShellTerminalOutputDirectory(t, ctx, client, home)

		configureShellCustomWorkingDirectory(t, ctx, client, lastUsedDirectory)
		confirmShellCustomWorkingDirectoryPersisted(t, ctx, client, lastUsedDirectory)
		queryShellWorkingDirectory(t, ctx, client, "> "+lastUsedSeedCommand, lastUsedDirectory)
		executeShellCwdMarker(t, ctx, client, lastUsedSeedCommand, lastUsedDirectory, "wox-cwd-last-used-seed.txt")

		configureShellCustomWorkingDirectory(t, ctx, client, customDirectory)
		confirmShellCustomWorkingDirectoryPersisted(t, ctx, client, customDirectory)
		queryShellWorkingDirectory(t, ctx, client, "> ", customDirectory)

		lastUsedLabel := selectShellWorkingDirectoryMode(t, ctx, client, shellLastUsedWorkingDirectoryOption)
		confirmShellWorkingDirectoryModePersisted(t, ctx, client, lastUsedLabel, false)
		queryShellWorkingDirectory(t, ctx, client, "> ", lastUsedDirectory)
		queryShellWorkingDirectory(t, ctx, client, "> "+lastUsedCommand, lastUsedDirectory)
		executeShellCwdMarker(t, ctx, client, lastUsedCommand, lastUsedDirectory, "wox-cwd-last-used-run.txt")
	})
}
