package main

import (
	"context"
	"wox/cloudsync"
	"wox/database"
	"wox/plugin"
	"wox/ui"
	"wox/util"
)

// prepareDatabaseRecovery protects a restored backup before any cloud pull starts.
func prepareDatabaseRecovery(ctx context.Context) (database.RecoveryEvent, bool) {
	event, pending := database.LatestRecovery(util.GetLocation().GetBackupDirectory())
	if !pending {
		return database.RecoveryEvent{}, true
	}
	var err error
	switch event.Mode {
	case database.RecoveryBackup:
		err = cloudsync.MarkPendingRestoreSnapshot()
	case database.RecoveryEmpty:
		// A marker left by a previous restore must not turn an empty fallback into an upload.
		err = cloudsync.ClearPendingRestoreSnapshot()
	}
	if err != nil {
		util.GetLogger().Error(ctx, "prepare cloud sync after database recovery: "+err.Error())
		return event, false
	}
	return event, true
}

func showDatabaseRecovery(ctx context.Context, event database.RecoveryEvent, syncReady bool) {
	if event.Timestamp == 0 {
		return
	}
	ui.GetUIManager().GetUI(ctx).ShowToast(ctx, plugin.DatabaseRecoveryToast(ctx, event))
	if syncReady {
		if err := event.Acknowledge(); err != nil {
			util.GetLogger().Warn(ctx, "acknowledge database recovery notice: "+err.Error())
		}
	}
}
