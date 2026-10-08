package migration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"wox/cloudsync"
	"wox/database"
	"wox/setting"
	"wox/util"

	"gorm.io/gorm"
)

const pinedResultsSettingKey = "PinedResults"

func init() { Register(&clearPinedResultsMigration{}) }

type clearPinedResultsMigration struct{}

func (m *clearPinedResultsMigration) ID() string { return "20261008_clear_pined_results" }

func (m *clearPinedResultsMigration) Description() string {
	return "Drop query pins saved before they were tied to a query text."
}

// Up replaces the old boolean pin map with an empty query-scoped map.
// Those rows only recorded that a result was pinned, not which query it belonged to.
func (m *clearPinedResultsMigration) Up(ctx context.Context, tx *gorm.DB) error {
	var row database.WoxSetting
	err := tx.Where("key = ?", pinedResultsSettingKey).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if !legacyPinedResultsValue(row.Value) {
		return nil
	}

	// A queued upload of the boolean map would restore pins that have no query.
	if err := tx.Model(&database.Oplog{}).
		Where("entity_type = ? AND entity_id = ? AND key = ? AND synced_to_cloud = ? AND cloud_sync_discarded = ?",
			cloudsync.EntityWoxSetting, pinedResultsSettingKey, pinedResultsSettingKey, false, false).
		Update("cloud_sync_discarded", true).Error; err != nil {
		return err
	}

	store := setting.NewWoxSettingStore(tx)
	if err := store.SetWithSync(pinedResultsSettingKey, util.NewHashMap[setting.ResultHash, setting.PinedQueryResult](), true); err != nil {
		return err
	}
	util.GetLogger().Info(ctx, "cleared query pins that were not stored with a query")
	return nil
}

// legacyPinedResultsValue reports the pre-query pin document, whose values are booleans.
func legacyPinedResultsValue(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return false
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil || len(entries) == 0 {
		return false
	}
	for _, value := range entries {
		var pinned bool
		if json.Unmarshal(value, &pinned) == nil {
			return true
		}
	}
	return false
}
