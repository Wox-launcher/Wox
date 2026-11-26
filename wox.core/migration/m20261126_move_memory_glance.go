package migration

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"wox/common"
	"wox/database"
	"wox/setting"

	"gorm.io/gorm"
)

func init() { Register(&moveMemoryGlanceMigration{}) }

type moveMemoryGlanceMigration struct{}

func (*moveMemoryGlanceMigration) ID() string { return "20261126_move_memory_glance" }

func (*moveMemoryGlanceMigration) Description() string {
	return "Transfer the selected Wox Memory Glance to Wox Doctor."
}

// Up changes only the provider ID and records the new selection for Cloud Sync.
func (*moveMemoryGlanceMigration) Up(_ context.Context, tx *gorm.DB) error {
	const key = "PrimaryGlance"
	var row database.WoxSetting
	if err := tx.Where("key = ?", key).First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}

	var glance setting.GlanceRef
	if json.Unmarshal([]byte(row.Value), &glance) != nil || glance.GlanceId != "wox_memory" {
		return nil
	}
	if glance.PluginId != common.WoxMemoryPluginID && glance.PluginId != common.GlancePluginID {
		return nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(row.Value), &fields); err != nil {
		return err
	}
	for name := range fields {
		if strings.EqualFold(name, "PluginId") {
			delete(fields, name)
		}
	}
	fields["PluginId"] = json.RawMessage(strconv.Quote(common.DoctorPluginID))
	return setting.NewWoxSettingStore(tx).SetWithSync(key, fields, true)
}
