package system

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"wox/cloudsync"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
)

// doctorResilience exports only timestamps, sizes, statuses, counts, and public plugin IDs.
type doctorResilience struct {
	DatabaseChecked      bool                           `json:"databaseChecked"`
	DatabaseOK           bool                           `json:"databaseOk"`
	RecoveryMode         database.RecoveryMode          `json:"recoveryMode,omitempty"`
	RecoveryTime         int64                          `json:"recoveryTime,omitempty"`
	RecoveredRows        int                            `json:"recoveredRows,omitempty"`
	RecoveryBackupTime   int64                          `json:"recoveryBackupTime,omitempty"`
	LatestBackupTime     int64                          `json:"latestBackupTime,omitempty"`
	LatestBackupBytes    int64                          `json:"latestBackupBytes,omitempty"`
	LastCloudSyncTime    int64                          `json:"lastCloudSyncTime,omitempty"`
	PendingCloudChanges  int                            `json:"pendingCloudChanges"`
	UnsupportedPluginIDs []string                       `json:"unsupportedPluginIds,omitempty"`
	ArchivedConflicts    int                            `json:"archivedConflicts"`
	NotesConflictCopies  int                            `json:"notesConflictCopies"`
	PluginRecoveryEvents []plugin.ArtifactRecoveryEvent `json:"pluginRecoveryEvents,omitempty"`
}

func collectDoctorResilience(ctx context.Context) doctorResilience {
	root := util.GetLocation().GetBackupDirectory()
	integrity := database.GetIntegrityReport()
	event, _ := database.LatestRecovery(root)
	backupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	backup, _ := database.LatestHealthyBackup(backupCtx, root)
	cancel()
	total, notes := cloudsync.ConflictCounts(root)
	result := doctorResilience{
		DatabaseChecked: integrity.Ran, DatabaseOK: integrity.QuickCheckOK,
		RecoveryMode: event.Mode, RecoveryTime: event.Timestamp, RecoveredRows: event.SalvagedRows, RecoveryBackupTime: event.BackupTime,
		LatestBackupTime: backup.Timestamp, LatestBackupBytes: backup.Bytes,
		ArchivedConflicts: total, NotesConflictCopies: notes,
		PluginRecoveryEvents: plugin.ArtifactRecoveryEvents(),
	}
	if db := database.GetDB(); db != nil {
		var pending int64
		_ = db.WithContext(ctx).Model(&database.Oplog{}).Where("synced_to_cloud = ? AND cloud_sync_discarded = ?", false, false).Count(&pending).Error
		result.PendingCloudChanges = int(pending)
		if state, err := cloudsync.LoadCloudSyncState(ctx); err == nil && state != nil {
			result.LastCloudSyncTime = max(state.LastPushTs, state.LastPullTs)
		}
		if history, err := cloudsync.NewDefaultCloudSyncHistoryStore().ListRecent(ctx, 50); err == nil {
			ids := make(map[string]bool)
			for _, item := range history {
				for _, detail := range item.Details {
					if detail.Status == cloudsync.CloudSyncHistoryStatusSkippedUnsupportedOS && isPublicPluginID(detail.Key) {
						ids[detail.Key] = true
					}
				}
			}
			for id := range ids {
				result.UnsupportedPluginIDs = append(result.UnsupportedPluginIDs, id)
			}
			slices.Sort(result.UnsupportedPluginIDs)
		}
	}
	return result
}

// isPublicPluginID admits ordinary manifest IDs while excluding paths and arbitrary history text.
func isPublicPluginID(id string) bool {
	if id == "" || len(id) > 96 || strings.Contains(id, "..") {
		return false
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-') {
			return false
		}
	}
	return true
}

func (r doctorResilience) markdown(ctx context.Context, out *strings.Builder) {
	tr := func(key string) string { return i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_"+key) }
	date := func(timestamp int64) string {
		if timestamp == 0 {
			return "—"
		}
		return time.UnixMilli(timestamp).UTC().Format(time.RFC3339)
	}
	fmt.Fprintf(out, "\n**%s**\n\n", tr("resilience"))
	fmt.Fprintf(out, "- %s · quick_check: %t / %t\n", tr("database"), r.DatabaseChecked, r.DatabaseOK)
	if r.RecoveryTime > 0 {
		fmt.Fprintf(out, "- %s · %s · %s · %d\n", r.RecoveryMode, date(r.RecoveryTime), date(r.RecoveryBackupTime), r.RecoveredRows)
	}
	fmt.Fprintf(out, "- %s\n", fmt.Sprintf(tr("resilience_backup"), date(r.LatestBackupTime), r.LatestBackupBytes))
	fmt.Fprintf(out, "- %s\n", fmt.Sprintf(tr("resilience_sync"), date(r.LastCloudSyncTime), r.PendingCloudChanges, r.ArchivedConflicts, r.NotesConflictCopies))
	if len(r.UnsupportedPluginIDs) > 0 {
		fmt.Fprintf(out, "- %s · %s\n", tr("resilience_unsupported"), strings.Join(r.UnsupportedPluginIDs, ", "))
	}
	for _, event := range r.PluginRecoveryEvents {
		fmt.Fprintf(out, "- %s · %s\n", event.Kind, date(event.Timestamp))
	}
}
