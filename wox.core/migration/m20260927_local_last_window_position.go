package migration

import (
	"context"
	"errors"
	"strconv"
	"wox/database"
	"wox/setting"

	"gorm.io/gorm"
)

func init() { Register(&localLastWindowPositionMigration{}) }

type localLastWindowPositionMigration struct{}

func (m *localLastWindowPositionMigration) ID() string { return "20260927_local_last_window_position" }

func (m *localLastWindowPositionMigration) Description() string {
	return "Keep the last launcher origin together on this device instead of syncing its coordinates."
}

// Up copies a valid legacy pair before deleting the synced keys and their pending upserts.
func (m *localLastWindowPositionMigration) Up(_ context.Context, tx *gorm.DB) error {
	store := setting.NewWoxSettingStore(tx)
	var legacy []database.WoxSetting
	if err := tx.Where("key IN ?", []string{"LastWindowX", "LastWindowY"}).Find(&legacy).Error; err != nil {
		return err
	}
	var current database.WoxSetting
	err := tx.Where("key = ?", "LastWindowPosition").First(&current).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		values := map[string]string{}
		for _, row := range legacy {
			values[row.Key] = row.Value
		}
		x, xErr := strconv.Atoi(values["LastWindowX"])
		y, yErr := strconv.Atoi(values["LastWindowY"])
		if xErr == nil && yErr == nil && x != -1 && y != -1 {
			if err := store.SetWithSync("LastWindowPosition", setting.SavedWindowPosition{X: x, Y: y, Valid: true}, false); err != nil {
				return err
			}
		}
	}
	for _, row := range legacy {
		if err := store.DeleteWithSync(row.Key, true); err != nil {
			return err
		}
	}
	return nil
}
