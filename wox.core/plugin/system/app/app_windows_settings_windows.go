package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/util"
)

type windowsSettingItem struct {
	NameKey   string
	URI       string
	IconSource string
}

func (a *WindowsRetriever) getWindowsSettingsApps(ctx context.Context) []appInfo {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}

	defaultIcon := a.iconFromFile(ctx, filepath.Join(systemRoot, "ImmersiveControlPanel", "SystemSettings.exe"))
	if defaultIcon.IsEmpty() {
		defaultIcon = appIcon
	}

	items := []windowsSettingItem{
		{
			NameKey:   "i18n:plugin_app_windows_settings_system",
			URI:       "ms-settings:system",
			IconSource: filepath.Join(systemRoot, "System32", "SystemPropertiesAdvanced.exe"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_bluetooth",
			URI:       "ms-settings:bluetooth",
			IconSource: filepath.Join(systemRoot, "System32", "DevicePairingWizard.exe"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_network",
			URI:       "ms-settings:network",
			IconSource: filepath.Join(systemRoot, "System32", "ncpa.cpl"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_personalization",
			URI:       "ms-settings:personalization",
			IconSource: filepath.Join(systemRoot, "System32", "themecpl.dll"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_apps",
			URI:       "ms-settings:appsfeatures",
			IconSource: filepath.Join(systemRoot, "System32", "appwiz.cpl"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_accounts",
			URI:       "ms-settings:yourinfo",
			IconSource: filepath.Join(systemRoot, "System32", "netplwiz.exe"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_time_language",
			URI:       "ms-settings:time-language",
			IconSource: filepath.Join(systemRoot, "System32", "timedate.cpl"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_gaming",
			URI:       "ms-settings:gaming-gamebar",
			IconSource: filepath.Join(systemRoot, "System32", "GameBar.exe"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_accessibility",
			URI:       "ms-settings:easeofaccess",
			IconSource: filepath.Join(systemRoot, "System32", "access.cpl"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_privacy_security",
			URI:       "ms-settings:privacy",
			IconSource: filepath.Join(systemRoot, "System32", "wscui.cpl"),
		},
		{
			NameKey:   "i18n:plugin_app_windows_settings_windows_update",
			URI:       "ms-settings:windowsupdate",
			IconSource: filepath.Join(systemRoot, "System32", "wuauclt.exe"),
		},
	}

	apps := make([]appInfo, 0, len(items))
	for _, item := range items {
		icon := defaultIcon
		if item.IconSource != "" {
			if candidate := a.iconFromFile(ctx, item.IconSource); !candidate.IsEmpty() {
				icon = candidate
			}
		}

		apps = append(apps, appInfo{
			Name: item.NameKey,
			Path: item.URI,
			Icon: icon,
			Type: AppTypeWindowsSetting,
		})
	}

	return apps
}

func (a *WindowsRetriever) iconFromFile(ctx context.Context, iconSourcePath string) common.WoxImage {
	if strings.TrimSpace(iconSourcePath) == "" {
		return common.WoxImage{}
	}
	if _, err := os.Stat(iconSourcePath); err != nil {
		util.GetLogger().Debug(ctx, "Windows settings icon source missing: "+iconSourcePath)
		return common.WoxImage{}
	}

	img, err := a.GetAppIcon(ctx, iconSourcePath)
	if err != nil {
		a.api.Log(ctx, plugin.LogLevelDebug, "failed to extract icon from "+iconSourcePath+": "+err.Error())
		return common.WoxImage{}
	}
	woxIcon, err := common.NewWoxImage(img)
	if err != nil {
		a.api.Log(ctx, plugin.LogLevelDebug, "failed to convert icon image for "+iconSourcePath+": "+err.Error())
		return common.WoxImage{}
	}
	return woxIcon
}
