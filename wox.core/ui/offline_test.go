package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/database"
	"wox/network"
	"wox/setting"
	"wox/util"
)

// TestOfflinePersistenceFailureKeepsRuntime verifies that failed writes never disconnect a running session.
func TestOfflinePersistenceFailureKeepsRuntime(t *testing.T) {
	root := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, root)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(root, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	settings := setting.NewWoxSetting(setting.NewWoxSettingStore(db))
	previous := network.IsOffline()
	network.Default.SetOffline(false)
	t.Cleanup(func() { network.Default.SetOffline(previous) })
	// A malformed existing privacy profile makes its refresh fail after the setting write.
	if err := os.WriteFile(filepath.Join(root, "privacy.json"), []byte("invalid json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := setOfflineMode(context.Background(), settings, true); err == nil {
		t.Fatal("expected profile failure")
	}
	if network.IsOffline() || settings.EnableOfflineMode.Get() {
		t.Fatal("profile failure changed runtime or saved preference")
	}
	reloaded := setting.NewWoxSetting(setting.NewWoxSettingStore(db))
	if reloaded.EnableOfflineMode.Get() {
		t.Fatal("preference rollback was not persisted")
	}
	_ = sqlDB.Close()
	if err := setOfflineMode(context.Background(), settings, true); err == nil {
		t.Fatal("expected database failure")
	}
	if network.IsOffline() {
		t.Fatal("database failure changed runtime policy")
	}
}
