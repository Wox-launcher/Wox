package privacy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/database"
	"wox/setting"
	"wox/util"
)

// TestOfflinePreferenceSurvivesCleanup verifies local-only storage and the actual cleanup/restore boundary.
func TestOfflinePreferenceSurvivesCleanup(t *testing.T) {
	root := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, root)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(root, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	open := func() *setting.WoxSetting {
		if err := util.GetLocation().Init(); err != nil {
			t.Fatal(err)
		}
		if err := database.Init(context.Background()); err != nil {
			t.Fatal(err)
		}
		return setting.NewWoxSetting(setting.NewWoxSettingStore(database.GetDB()))
	}
	closeDB := func() {
		if db := database.GetDB(); db != nil {
			sqlDB, err := db.DB()
			if err == nil {
				_ = sqlDB.Close()
			}
		}
	}
	t.Cleanup(closeDB)
	settings := open()
	if settings.EnableOfflineMode.Get() {
		t.Fatal("new installs must start online")
	}
	if err := settings.EnableOfflineMode.Set(true); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := database.GetDB().Model(&database.Oplog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("offline preference generated a cloud oplog")
	}
	if err := SetEnabled(true, settings); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(root, "private-data")
	if err := os.WriteFile(sentinel, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	closeDB()
	if err := PrepareAtStartup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("cleanup did not remove private data")
	}
	settings = open()
	if err := ApplyPreservedSettings(settings); err != nil {
		t.Fatal(err)
	}
	if !settings.EnableOfflineMode.Get() {
		t.Fatal("cleanup unexpectedly enabled networking")
	}
	if err := settings.EnableOfflineMode.SetLocal(false); err != nil {
		t.Fatal(err)
	}
	if err := RefreshPreservedSettings(settings); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfile(root)
	if err != nil || profile.PreservedSettings.EnableOfflineMode {
		t.Fatalf("updated offline preference was not preserved: %v", err)
	}
}
