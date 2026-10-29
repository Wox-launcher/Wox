package setting

import (
	"testing"

	"wox/database"
)

func TestNavigationBindingsDefaultAndSync(t *testing.T) {
	db := openSettingSyncDB(t)
	settings := NewWoxSetting(NewWoxSettingStore(db))
	if got := settings.NavigationBindings.Get(); got != NavigationBindingModeOff {
		t.Fatalf("default = %q, want off", got)
	}
	if err := settings.NavigationBindings.Set(NavigationBindingModeVim); err != nil {
		t.Fatal(err)
	}
	reloaded := NewWoxSetting(NewWoxSettingStore(db))
	if got := reloaded.NavigationBindings.Get(); got != NavigationBindingModeVim {
		t.Fatalf("reloaded = %q, want vim", got)
	}
	var count int64
	if err := db.Model(&database.Oplog{}).Where("key = ?", "NavigationBindings").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("sync oplogs = %d, error = %v", count, err)
	}
	var stored database.WoxSetting
	if err := db.Where("key = ?", "NavigationBindings").First(&stored).Error; err != nil || stored.Value != "vim" {
		t.Fatalf("stored value = %q, error = %v", stored.Value, err)
	}
}
