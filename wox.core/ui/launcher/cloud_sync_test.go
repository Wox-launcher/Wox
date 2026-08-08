package launcher

import (
	"testing"

	"wox/cloudsync"
	"wox/ui/contract"
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
	app := &App{
		translations: map[string]string{
			"ui_cloud_sync_syncing":              "Syncing...",
			"ui_cloud_sync_progress_starting":    "Preparing sync...",
			"ui_cloud_sync_progress_count":       " ({count} items processed)",
			"ui_cloud_sync_progress_restoring":   "Restoring {target}{count}",
			"ui_cloud_sync_progress_plugin":      "plugin {plugin}",
			"ui_cloud_sync_progress_data":        "sync data",
			"ui_cloud_sync_progress_uploading":   "Uploading {target}{count}",
			"ui_cloud_sync_progress_wox_setting": "Wox settings",
		},
		pluginSettings: newPluginSettingsController(CommonDeps{}),
	}
	_, detail, _ := app.cloudSyncPresentation(settingsSnapshot{
		cloud: cloudSettingsSnapshot{
			Sync: cloudSyncStatus{
				Progress: &cloudSyncProgress{
					Active:     true,
					Operation:  cloudsync.CloudSyncProgressOperationRestore,
					EntityType: cloudsync.EntityInstalledPlugin,
					PluginID:   "plugin-deepl",
					Current:    17,
				},
			},
		},
	})
	if detail != "Restoring plugin plugin-deepl (17 items processed)" {
		t.Fatalf("restore detail = %q, want %q", detail, "Restoring plugin plugin-deepl (17 items processed)")
	}

	label, detail, _ := app.cloudSyncPresentation(settingsSnapshot{cloud: cloudSettingsSnapshot{Busy: "sync"}})
	if label != "Syncing..." || detail != "Preparing sync..." {
		t.Fatalf("busy sync presentation = %q / %q, want Syncing... / Preparing sync...", label, detail)
	}
}

func TestNewCloudBootstrapFormUsesI18nKeys(t *testing.T) {
	restore := newCloudBootstrapForm(contract.CloudBootstrapStatus{HasRemoteData: true})
	if restore.title != "i18n:ui_cloud_sync_bootstrap_restore_title" {
		t.Fatalf("restore title = %q", restore.title)
	}
	if len(restore.definitions) != 1 || restore.definitions[0].Value.Label != "i18n:ui_cloud_sync_recovery_code" {
		t.Fatalf("restore fields = %#v", restore.definitions)
	}

	start := newCloudBootstrapForm(contract.CloudBootstrapStatus{HasRemoteData: false})
	if start.title != "i18n:ui_cloud_sync_bootstrap_start_title" {
		t.Fatalf("start title = %q", start.title)
	}
	if len(start.definitions) != 2 || start.definitions[1].Value.Label != "i18n:ui_cloud_sync_recovery_code_confirm" {
		t.Fatalf("start fields = %#v", start.definitions)
	}
	if got := cloudFormDescription(&cloudFormSnapshot{kind: "bootstrap", hasRemoteData: true}); got != "i18n:ui_cloud_sync_bootstrap_restore_description" {
		t.Fatalf("restore description = %q", got)
	}
	if got := validateCloudForm("bootstrap", map[string]string{"RecoveryCode": ""}, true); got != "i18n:ui_cloud_sync_recovery_code_required" {
		t.Fatalf("required validation = %q", got)
	}
}

func TestFormatCloudSyncProgressUsesPushTargetAndTotal(t *testing.T) {
	app := &App{
		translations: map[string]string{
			"ui_cloud_sync_progress_count":       " ({count} items processed)",
			"ui_cloud_sync_progress_uploading":   "Uploading {target}{count}",
			"ui_cloud_sync_progress_wox_setting": "Wox settings",
		},
		pluginSettings: newPluginSettingsController(CommonDeps{}),
	}
	detail := app.formatCloudSyncProgress(&cloudSyncProgress{
		Active:     true,
		Operation:  cloudsync.CloudSyncProgressOperationPush,
		EntityType: cloudsync.EntityWoxSetting,
		Current:    3,
		Total:      10,
	}, false)
	if detail != "Uploading Wox settings (3/10 items processed)" {
		t.Fatalf("push detail = %q", detail)
	}
}
