package setting

import (
	"testing"
	"wox/database"
)

// TestSoftwareRenderingPersistsLocally verifies the preference survives a reload without cloud replication.
func TestSoftwareRenderingPersistsLocally(t *testing.T) {
	db := openSettingSyncDB(t)
	settings := NewWoxSetting(NewWoxSettingStore(db))
	if settings.UseSoftwareRendering.Get() {
		t.Fatal("software rendering must be opt-in")
	}
	for _, enabled := range []bool{true, false} {
		if err := settings.UseSoftwareRendering.Set(enabled); err != nil {
			t.Fatal(err)
		}
		reloaded := NewWoxSetting(NewWoxSettingStore(db))
		if reloaded.UseSoftwareRendering.Get() != enabled {
			t.Fatal("rendering preference did not survive reload")
		}
	}
	var count int64
	if err := db.Model(&database.Oplog{}).Where("key = ?", "UseSoftwareRendering").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("device-specific rendering preference generated a cloud oplog")
	}
}
