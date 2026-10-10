package cloudsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"wox/util"
)

const pendingRestoreSnapshotName = "pending-restore-snapshot"

// RestoreBackupPushReason is the sync history source for the snapshot uploaded after a backup restore.
const RestoreBackupPushReason = "restore-backup"

var errCloudSyncBackoff = errors.New("cloud sync backoff is active")

func pendingRestoreSnapshotPath() string {
	return filepath.Join(util.GetLocation().GetWoxDataDirectory(), pendingRestoreSnapshotName)
}

// MarkPendingRestoreSnapshot asks the next cloud sync to upload the restored data before pulling.
// The marker lives in the Wox data directory, outside the user data directory that restore replaces.
func MarkPendingRestoreSnapshot() error {
	dir := util.GetLocation().GetWoxDataDirectory()
	if dir == "" {
		return fmt.Errorf("wox data directory is not initialized")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(pendingRestoreSnapshotPath(), []byte(RestoreBackupPushReason+"\n"), 0644)
}

// HasPendingRestoreSnapshot reports whether a restored backup still needs to be pushed.
func HasPendingRestoreSnapshot() bool {
	_, err := os.Stat(pendingRestoreSnapshotPath())
	return err == nil
}

// ClearPendingRestoreSnapshot removes the marker after the restored backup has been pushed.
func ClearPendingRestoreSnapshot() error {
	err := os.Remove(pendingRestoreSnapshotPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ensureRestoredSnapshotPushed uploads a restored backup before a pull can replace it.
// A failed push keeps the marker so the next attempt still runs before any pull.
func (m *CloudSyncManager) ensureRestoredSnapshotPushed(ctx context.Context) bool {
	if !HasPendingRestoreSnapshot() {
		return true
	}
	if err := m.PushLocalSnapshot(ctx, RestoreBackupPushReason); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("cloud sync deferred pull until the restored backup can be pushed: %v", err))
		return false
	}
	if err := ClearPendingRestoreSnapshot(); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("cloud sync pushed the restored backup but could not clear the marker: %v", err))
		return false
	}
	util.GetLogger().Info(ctx, "cloud sync pushed the restored backup before pulling")
	return true
}
