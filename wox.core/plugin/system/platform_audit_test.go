package system

import (
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
)

func TestSystemPluginMachineSettingsStayOnTheirPlatform(t *testing.T) {
	for _, system := range plugin.AllSystemPlugin {
		metadata := system.GetMetadata()
		for _, item := range metadata.SettingDefinitions {
			if item.Value == nil || item.IsPlatformSpecific {
				continue
			}
			if metadata.Id == common.ScreenshotPluginID && item.Value.GetKey() == screenshotSaveDirectorySettingKey {
				// Screenshot storage is persisted in a device-local sidecar by its settings override.
				continue
			}
			machineSpecific := false
			switch value := item.Value.(type) {
			case *definition.PluginSettingValueDirPath, *definition.PluginSettingValueFilePath, *definition.PluginSettingValueHotkey:
				machineSpecific = true
			case *definition.PluginSettingValueTable:
				for _, column := range value.Columns {
					switch column.Type {
					case definition.PluginSettingValueTableColumnTypeDirPath, definition.PluginSettingValueTableColumnTypeFilePath,
						definition.PluginSettingValueTableColumnTypeHotkey, definition.PluginSettingValueTableColumnTypeApp,
						definition.PluginSettingValueTableColumnTypeIgnoredApps:
						machineSpecific = true
					}
				}
			}
			if machineSpecific {
				t.Errorf("%s/%s needs a platform or device-local setting contract", metadata.Id, item.Value.GetKey())
			}
		}
	}
}
