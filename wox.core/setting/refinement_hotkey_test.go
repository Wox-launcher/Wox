package setting

import (
	"runtime"
	"testing"

	"wox/database"
)

func TestRefinementHotkeyDefaultAndPlatformSync(t *testing.T) {
	db := openSettingSyncDB(t)
	settings := NewWoxSetting(NewWoxSettingStore(db))
	want := "ctrl+f"
	if runtime.GOOS == "darwin" {
		want = "command+f"
	}
	if got := settings.RefinementHotkey.Get(); got != want {
		t.Fatalf("default = %q, want %q", got, want)
	}
	if err := settings.RefinementHotkey.Set("alt+g"); err != nil {
		t.Fatal(err)
	}
	if got := NewWoxSetting(NewWoxSettingStore(db)).RefinementHotkey.Get(); got != "alt+g" {
		t.Fatalf("reloaded = %q", got)
	}
	var count int64
	if err := db.Model(&database.Oplog{}).Where("key = ?", settings.RefinementHotkey.Key()).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("sync oplogs = %d, error = %v", count, err)
	}
}
