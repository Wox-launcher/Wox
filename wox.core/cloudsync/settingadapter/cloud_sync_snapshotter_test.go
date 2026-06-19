package settingadapter

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"wox/cloudsync"
	"wox/database"
	"wox/setting"
	"wox/util"
)

func TestCollectLocalSnapshotOplogsKeepsOnlyCommonAndCurrentPlatformSettings(t *testing.T) {
	ctx := context.Background()
	initSnapshotterTestDatabase(t)

	currentPlatform := util.GetCurrentPlatform()
	otherPlatform := snapshotterTestOtherPlatform()
	db := database.GetDB()
	rows := []database.WoxSetting{
		{Key: "ThemeId", Value: "dark"},
		{Key: setting.PlatformSettingKey("MainHotkey", currentPlatform), Value: "current-hotkey"},
		{Key: setting.PlatformSettingKey("MainHotkey", otherPlatform), Value: "other-hotkey"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create wox settings: %v", err)
	}

	pluginRows := []database.PluginSetting{
		{PluginID: "browser", Key: "defaultBrowser", Value: "system"},
		{PluginID: "browser", Key: setting.PlatformSettingKey("defaultBrowser", currentPlatform), Value: "current-browser"},
		{PluginID: "browser", Key: setting.PlatformSettingKey("defaultBrowser", otherPlatform), Value: "other-browser"},
	}
	if err := db.Create(&pluginRows).Error; err != nil {
		t.Fatalf("create plugin settings: %v", err)
	}

	oplogs, err := NewLocalSnapshotter().collectLocalSnapshotOplogs(ctx)
	if err != nil {
		t.Fatalf("collect snapshot oplogs: %v", err)
	}

	woxKeys := snapshotterTestOplogKeys(oplogs, cloudsync.EntityWoxSetting, "")
	if !slices.Contains(woxKeys, "ThemeId") {
		t.Fatalf("wox keys = %#v, want ThemeId", woxKeys)
	}
	if !slices.Contains(woxKeys, setting.PlatformSettingKey("MainHotkey", currentPlatform)) {
		t.Fatalf("wox keys = %#v, want current platform hotkey", woxKeys)
	}
	if slices.Contains(woxKeys, setting.PlatformSettingKey("MainHotkey", otherPlatform)) {
		t.Fatalf("wox keys = %#v, must not include other platform hotkey", woxKeys)
	}

	pluginKeys := snapshotterTestOplogKeys(oplogs, cloudsync.EntityPluginSetting, "browser")
	if !slices.Contains(pluginKeys, "defaultBrowser") {
		t.Fatalf("plugin keys = %#v, want defaultBrowser", pluginKeys)
	}
	if !slices.Contains(pluginKeys, setting.PlatformSettingKey("defaultBrowser", currentPlatform)) {
		t.Fatalf("plugin keys = %#v, want current platform browser", pluginKeys)
	}
	if slices.Contains(pluginKeys, setting.PlatformSettingKey("defaultBrowser", otherPlatform)) {
		t.Fatalf("plugin keys = %#v, must not include other platform browser", pluginKeys)
	}
}

func initSnapshotterTestDatabase(t *testing.T) {
	t.Helper()
	t.Setenv(util.TestWoxDataDirEnv, filepath.Join(t.TempDir(), "wox"))
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(t.TempDir(), "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init location: %v", err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatalf("init database: %v", err)
	}
}

func snapshotterTestOplogKeys(oplogs []database.Oplog, entityType string, entityID string) []string {
	keys := []string{}
	for _, oplog := range oplogs {
		if oplog.EntityType != entityType {
			continue
		}
		if entityID != "" && oplog.EntityID != entityID {
			continue
		}
		keys = append(keys, oplog.Key)
	}
	return keys
}

func snapshotterTestOtherPlatform() string {
	switch util.GetCurrentPlatform() {
	case util.PlatformWindows:
		return util.PlatformMacOS
	case util.PlatformMacOS:
		return util.PlatformWindows
	default:
		return util.PlatformMacOS
	}
}
