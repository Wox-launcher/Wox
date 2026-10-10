//go:build wox_ui_smoke

package data

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test002SettingDataRestoreBackup verifies restoring a backup returns the saved settings after Wox restarts.
// Flow: change the result limit -> create a backup -> change the limit again -> confirm Restore -> wait for restart.
// Evidence: the result limit matches the backed-up value, and supervisor.log records the completed restore task.
func Test002SettingDataRestoreBackup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := smoke.SharedClient(t, ctx)
	if err := client.Reset(ctx); err != nil {
		t.Fatalf("reset Wox before backup restore: %v", err)
	}
	t.Cleanup(func() {
		marker := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "pending-restore-snapshot")
		_ = os.Remove(marker)
	})

	original := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount")
	backupValue := otherResultLimit(original, "")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-MaxResultCount", backupValue)
	if got := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount"); got != backupValue {
		t.Fatalf("result limit before backup = %q, want %q", got, backupValue)
	}

	createBackup(t, ctx, client)
	changedValue := otherResultLimit(backupValue, original)
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-MaxResultCount", changedValue)

	previousPID := smoke.LatestStartupPID(t)
	info, err := automationdriver.ReadInfo(ctx, infoFile(t))
	if err != nil {
		t.Fatalf("read automation endpoint before restore: %v", err)
	}
	confirmRestore(t, ctx, client)

	restored, _ := smoke.WaitForSupervisorReplacement(t, ctx, info, previousPID, "task restore completed")
	if got := smoke.OpenSettingsAndReadChoice(t, ctx, restored, "/appearance", "MaxResultCount"); got != backupValue {
		t.Fatalf("result limit after restore = %q, want backed-up %q", got, backupValue)
	}
	smoke.SelectSettingChoiceByLabel(t, ctx, restored, "setting-choice-MaxResultCount", original)
	snapshot, err := restored.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read settings after restoring the original result limit: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

func createBackup(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	openDataSettings(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pageFound := automationdriver.Find(snapshot, "settings.page.data")
		button, buttonFound := automationdriver.Find(snapshot, "data-backups-secondary")
		return pageFound && buttonFound && button.Enabled
	}); err != nil {
		t.Fatalf("wait for backup action: %v", err)
	}
	if err := client.Perform(ctx, "data-backups-secondary", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "data-backup-restore-0")
		return found
	}); err != nil {
		t.Fatalf("wait for backup row: %v", err)
	}
}

func confirmRestore(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	openDataSettings(t, ctx, client)
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "data-backup-restore-0")
		return found
	})
	if err != nil {
		t.Fatalf("wait for restore action: %v", err)
	}
	button, _ := automationdriver.Find(snapshot, "data-backup-restore-0")
	idleLabel := button.Label
	if err := client.Perform(ctx, "data-backup-restore-0", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("arm restore confirmation: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		button, found := automationdriver.Find(snapshot, "data-backup-restore-0")
		return found && button.Label != "" && button.Label != idleLabel
	}); err != nil {
		t.Fatalf("wait for restore confirmation: %v", err)
	}
	_ = client.Perform(ctx, "data-backup-restore-0", woxui.AccessibilityActionActivate, "")
}

func otherResultLimit(current, avoid string) string {
	for _, candidate := range []string{"5", "15", "8"} {
		if candidate != current && candidate != avoid {
			return candidate
		}
	}
	return "6"
}

func infoFile(t *testing.T) string {
	t.Helper()
	path := os.Getenv(automationdriver.SharedInfoFileEnvironment)
	if path == "" {
		t.Fatal("automation info file is not configured")
	}
	return path
}
