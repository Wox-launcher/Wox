package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"wox/common"
	"wox/database"
	"wox/setting"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestFolderBrowseQueryUsesConfiguredTriggerKeyword(t *testing.T) {
	manager := GetPluginManager()
	instance := &Instance{Metadata: Metadata{Id: common.FolderPluginID, TriggerKeywords: []string{"*", "cd"}}}
	if !manager.lifecycle.appendPluginInstance(instance) {
		t.Fatal("folder plugin was already registered")
	}
	t.Cleanup(func() {
		manager.lifecycle.removePluginInstances(func(item *Instance) bool {
			return item != nil && item.Metadata.Id == common.FolderPluginID
		})
	})

	path := filepath.Join(t.TempDir(), "projects")
	separator := string(os.PathSeparator)
	assertFolderBrowseQuery(t, path, "cd "+path+separator)

	instance.Metadata.TriggerKeywords = []string{"*", "dir"}
	assertFolderBrowseQuery(t, path, "dir "+path+separator)

	instance.Metadata.TriggerKeywords = []string{"*"}
	assertFolderBrowseQuery(t, path, path+separator)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}); err != nil {
		t.Fatal(err)
	}
	instance.Metadata.TriggerKeywords = []string{"*", "cd"}
	instance.Setting = setting.NewPluginSetting(setting.NewPluginSettingStore(db, common.FolderPluginID), nil)
	if err := instance.Setting.TriggerKeywords.SetLocal([]string{"files"}); err != nil {
		t.Fatal(err)
	}
	assertFolderBrowseQuery(t, path, "files "+path+separator)
}

func assertFolderBrowseQuery(t *testing.T, path string, want string) {
	t.Helper()
	got := FolderBrowseQuery(path)
	if got.QueryType != QueryTypeInput || got.QueryText != want {
		t.Fatalf("browse query = %+v, want %q", got, want)
	}
}
