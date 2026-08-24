package cloudsync

import (
	"testing"
	"time"

	"wox/common"
)

func TestNotesPluginSettingsUseShortDeferredSync(t *testing.T) {
	policy := ResolveOplogSyncPolicy(EntityPluginSetting, common.NotesPluginID, "note:one", OpUpsert)
	if policy.Delay != 5*time.Second {
		t.Fatalf("notes sync delay = %v, want 5s", policy.Delay)
	}
	if policy := ResolveOplogSyncPolicy(EntityPluginSetting, common.NotesPluginID, "window", OpUpsert); policy.Delay != 0 {
		t.Fatalf("local notes metadata should not receive note delay: %v", policy.Delay)
	}
	if policy := ResolveOplogSyncPolicy(EntityPluginSetting, common.NotesPluginID, "note:one", OpDelete); policy.Delay != 0 {
		t.Fatalf("notes delete must remain immediate: %v", policy.Delay)
	}
}
