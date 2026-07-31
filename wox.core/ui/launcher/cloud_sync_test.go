package launcher

import (
	"testing"

	"wox/cloudsync"
)

func TestCloudSyncNeedsBootstrapAfterAuthentication(t *testing.T) {
	account := cloudAccountStatus{LoggedIn: true, SyncEligible: true}
	if !cloudSyncNeedsBootstrap(cloudSettingsSnapshot{Account: account}) {
		t.Fatal("eligible authenticated account without a key should continue bootstrap")
	}

	account.SyncEnabled = true
	ready := cloudSettingsSnapshot{
		Account: account,
		Sync: cloudSyncStatus{
			KeyStatus: cloudSyncKeyStatus{Available: true},
			State:     &cloudSyncState{Bootstrapped: true},
		},
	}
	if cloudSyncNeedsBootstrap(ready) {
		t.Fatal("fully bootstrapped account should not restart bootstrap")
	}
}

func TestCloudSyncPresentationShowsUnknownTotalProgress(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_cloud_sync_syncing":           "Syncing...",
		"ui_cloud_sync_progress_starting": "Preparing sync...",
	}}
	_, detail, _ := app.cloudSyncPresentation(settingsSnapshot{
		cloud: cloudSettingsSnapshot{
			Sync: cloudSyncStatus{
				Progress: &cloudSyncProgress{Active: true, Operation: cloudsync.CloudSyncProgressOperationRestore, Current: 17},
			},
		},
	})
	if detail != "Restore · 17" {
		t.Fatalf("restore detail = %q, want %q", detail, "Restore · 17")
	}

	label, detail, _ := app.cloudSyncPresentation(settingsSnapshot{cloud: cloudSettingsSnapshot{Busy: "sync"}})
	if label != "Syncing..." || detail != "Preparing sync..." {
		t.Fatalf("busy sync presentation = %q / %q, want Syncing... / Preparing sync...", label, detail)
	}
}
