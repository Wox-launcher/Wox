package setting

import (
	"reflect"
	"strings"
	"testing"
	"wox/util"
)

func TestWoxSettingSyncClassification(t *testing.T) {
	expected := map[string]string{}
	for scope, fields := range map[string]string{
		"platform": `EnableAutostart MainHotkey SelectionHotkey RefinementHotkey ActionPanelHotkey IgnoreHotkeysOnFullscreen
			IgnoredHotkeyApps QueryHotkeys ResultBindings CustomPythonPath CustomNodejsPath HttpProxyEnabled HttpProxyUrl AppFontFamily`,
		"local": `UseSoftwareRendering ShowDisplay CloudSyncServerUrl LastWindowPosition LastWindowX LastWindowY EnableOfflineMode`,
		"global": `NavigationBindings LogLevel UsePinYin SwitchInputMethodABC HideOnStart OnboardingFinished HideOnLostFocus ShowTray
			LangCode QueryAliases TrayQueries LaunchMode StartPage ShowPosition AIProviders AIMCPServers AISkills AIDisabledBuiltinTools
			EnableAutoBackup EnableAutoUpdate ReleaseChannel CloudSyncDisabledPlugins CloudSyncCrossPlatformHotkeys HiddenPluginStores
			AppWidth MaxResultCount UiDensity ThemeId EnableQueryCompletionHint EnableGlance PrimaryGlance HideGlanceIcon
			ShowScoreTail ShowPerformanceTail ShowPerformanceTailBatch ShowPerformanceTailPluginQuery ShowPerformanceTailBackendPrepared
			ShowPerformanceTailUiReceived QueryHistories QueryCompletionFeedbacks PinedResults ActionedResults EnableAnonymousUsageStats
			IgnoredDoctorChecks`,
	} {
		for _, field := range strings.Fields(fields) {
			expected[field] = scope
		}
	}
	value := reflect.ValueOf(NewWoxSetting(nil)).Elem()
	for i := 0; i < value.NumField(); i++ {
		name := value.Type().Field(i).Name
		setting, ok := value.Field(i).Interface().(interface {
			Key() string
			IsSyncable() bool
		})
		if !ok {
			t.Errorf("%s has no sync classification", name)
			continue
		}
		scope := "global"
		if !setting.IsSyncable() {
			scope = "local"
		} else if _, platform, split := SplitPlatformSettingKey(setting.Key()); split {
			scope = "platform"
			if platform != util.GetCurrentPlatform() {
				t.Errorf("%s writes to %s rather than the current platform", name, platform)
			}
		}
		if expected[name] != scope {
			t.Errorf("%s sync scope = %s, expected %q", name, scope, expected[name])
		}
		delete(expected, name)
	}
	if len(expected) != 0 {
		t.Errorf("unknown settings in sync contract: %v", expected)
	}
}
