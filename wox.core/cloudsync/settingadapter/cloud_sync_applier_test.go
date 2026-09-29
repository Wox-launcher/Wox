package settingadapter

import (
	"context"
	"testing"
	"time"

	"wox/cloudsync"
)

func TestApplyPluginSettingNotifiesAfterSyncLockReleased(t *testing.T) {
	initSnapshotterTestDatabase(t)
	ctx := context.Background()
	applier := NewLocalSettingApplier()
	const pluginID = "py-plugin-notify"
	const key = "notes"

	original := dispatchPluginSettingChange
	t.Cleanup(func() { dispatchPluginSettingChange = original })
	dispatchPluginSettingChange = func(ctx context.Context, gotPluginID string, gotKey string, gotValue string) {
		t.Errorf("callback ran inside ApplyPluginSetting: %s %s %q", gotPluginID, gotKey, gotValue)
	}
	if err := cloudsync.WithLocalSyncMutation(func() error {
		return applier.ApplyPluginSetting(ctx, pluginID, key, cloudsync.OpUpsert, "hello")
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	calls := 0
	var gotValues []string
	dispatchPluginSettingChange = func(ctx context.Context, gotPluginID string, gotKey string, gotValue string) {
		calls++
		gotValues = append(gotValues, gotValue)
		if gotPluginID != pluginID || gotKey != key {
			t.Errorf("notice = %s %s %q", gotPluginID, gotKey, gotValue)
		}
		acquired := make(chan struct{})
		started := make(chan struct{})
		go func() {
			close(started)
			_ = cloudsync.WithLocalSyncMutation(func() error {
				close(acquired)
				return nil
			})
		}()
		<-started
		select {
		case <-acquired:
		case <-time.After(time.Second):
			t.Error("plugin callback write blocked on the sync lock")
		}
	}
	applier.DeliverDeferredPluginSettingNotification(ctx, pluginID, key, "hello")
	if calls != 1 || len(gotValues) != 1 || gotValues[0] != "hello" {
		t.Fatalf("notifications = %v, want [hello]", gotValues)
	}

	if err := applier.ApplyPluginSetting(ctx, pluginID, key, cloudsync.OpUpsert, "hello"); err != nil {
		t.Fatalf("unchanged apply: %v", err)
	}
	applier.DeliverDeferredPluginSettingNotification(ctx, pluginID, key, "hello")
	if calls != 1 {
		t.Fatalf("notifications after unchanged apply = %d, want 1", calls)
	}

	if err := applier.ApplyPluginSetting(ctx, pluginID, key, cloudsync.OpUpsert, "again"); err != nil {
		t.Fatalf("apply again: %v", err)
	}
	applier.DeliverDeferredPluginSettingNotification(ctx, pluginID, "other", "again")
	if calls != 1 {
		t.Fatalf("notifications for a different key = %d, want 1", calls)
	}
	if err := applier.ApplyPluginSetting(ctx, pluginID, key, cloudsync.OpUpsert, "again"); err != nil {
		t.Fatalf("retry unchanged apply: %v", err)
	}
	applier.DeliverDeferredPluginSettingNotification(ctx, pluginID, key, "again")
	if calls != 2 || gotValues[1] != "again" {
		t.Fatalf("notifications after bookkeeping retry = %v, want [hello again]", gotValues)
	}
}
