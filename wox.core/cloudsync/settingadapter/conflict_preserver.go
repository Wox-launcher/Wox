package settingadapter

import (
	"context"
	"wox/cloudsync"
	"wox/util"
)

type LocalConflictPreserver struct{}

// PreserveConflict archives the competing values and invokes any plugin-registered conflict handler.
func (LocalConflictPreserver) PreserveConflict(ctx context.Context, conflict cloudsync.Conflict) (func(), error) {
	var result cloudsync.PluginConflictResult
	if conflict.EntityType == cloudsync.EntityPluginSetting {
		if handler, ok := cloudsync.LookupPluginConflictHandler(conflict.PluginID); ok {
			var err error
			result, err = handler(ctx, conflict)
			if err != nil {
				return nil, err
			}
		}
	}
	if err := cloudsync.ArchiveConflict(util.GetLocation().GetBackupDirectory(), conflict, result.ArchiveKind == cloudsync.ConflictArchiveNote); err != nil {
		return nil, err
	}
	if result.SettingKey == "" || result.SettingValue == "" {
		return nil, nil
	}
	return func() { dispatchPluginSettingChange(ctx, conflict.PluginID, result.SettingKey, result.SettingValue) }, nil
}
