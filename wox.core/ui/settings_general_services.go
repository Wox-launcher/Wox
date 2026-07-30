package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"wox/common"
	corehotkey "wox/hotkey"
	"wox/i18n"
	"wox/plugin"
	pluginhost "wox/plugin/host"
	"wox/privacy"
	"wox/setting"
	"wox/telemetry"
	"wox/ui/contract"
	"wox/util"
	"wox/util/font"
	"wox/util/keyboard"
)

// GeneralSettings returns the core-owned settings snapshot used across embedded settings pages.
func (s *CoreServices) GeneralSettings(ctx context.Context, sessionID string) (contract.GeneralSettings, error) {
	ctx = uiServiceContext(ctx, sessionID)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	skills := woxSetting.AISkills.Get()
	if chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx); chater != nil {
		skills = chater.GetAllSkills(ctx)
	}
	return contract.GeneralSettings{
		EnableAutostart:                    woxSetting.EnableAutostart.Get(),
		MainHotkey:                         woxSetting.MainHotkey.Get(),
		SelectionHotkey:                    woxSetting.SelectionHotkey.Get(),
		IgnoredHotkeyApps:                  append([]setting.IgnoredHotkeyApp(nil), woxSetting.IgnoredHotkeyApps.Get()...),
		LogLevel:                           util.NormalizeLogLevel(woxSetting.LogLevel.Get()),
		UsePinYin:                          woxSetting.UsePinYin.Get(),
		SwitchInputMethodABC:               woxSetting.SwitchInputMethodABC.Get(),
		HideOnStart:                        woxSetting.HideOnStart.Get(),
		OnboardingFinished:                 woxSetting.OnboardingFinished.Get(),
		HideOnLostFocus:                    woxSetting.HideOnLostFocus.Get(),
		ShowTray:                           woxSetting.ShowTray.Get(),
		LangCode:                           woxSetting.LangCode.Get(),
		QueryHotkeys:                       append([]setting.QueryHotkey(nil), woxSetting.QueryHotkeys.Get()...),
		QueryShortcuts:                     append([]setting.QueryShortcut(nil), woxSetting.QueryShortcuts.Get()...),
		TrayQueries:                        append([]setting.TrayQuery(nil), woxSetting.TrayQueries.Get()...),
		LaunchMode:                         woxSetting.LaunchMode.Get(),
		StartPage:                          woxSetting.StartPage.Get(),
		AIProviders:                        append([]setting.AIProvider(nil), woxSetting.AIProviders.Get()...),
		AIMCPServers:                       append([]common.AIChatMCPServerConfig(nil), woxSetting.AIMCPServers.Get()...),
		AISkills:                           append([]common.Skill(nil), skills...),
		HTTPProxyEnabled:                   woxSetting.HttpProxyEnabled.Get(),
		HTTPProxyURL:                       woxSetting.HttpProxyUrl.Get(),
		ShowPosition:                       woxSetting.ShowPosition.Get(),
		IsLinuxWaylandSession:              util.IsLinuxWaylandSession(),
		IsEvdevReadAvailable:               keyboard.IsEvdevReadAvailable(),
		EnableAutoBackup:                   woxSetting.EnableAutoBackup.Get(),
		EnableAutoUpdate:                   woxSetting.EnableAutoUpdate.Get(),
		ReleaseChannel:                     woxSetting.ReleaseChannel.Get(),
		EnableAnonymousUsageStats:          woxSetting.EnableAnonymousUsageStats.Get(),
		EnablePrivacyMode:                  privacy.IsEnabled(),
		CustomPythonPath:                   woxSetting.CustomPythonPath.Get(),
		CustomNodejsPath:                   woxSetting.CustomNodejsPath.Get(),
		CloudSyncServerURL:                 woxSetting.CloudSyncServerUrl.Get(),
		CloudSyncDisabledPlugins:           append([]string(nil), woxSetting.CloudSyncDisabledPlugins.Get()...),
		AppWidth:                           woxSetting.AppWidth.Get(),
		MaxResultCount:                     woxSetting.MaxResultCount.Get(),
		UIDensity:                          woxSetting.UiDensity.Get(),
		ThemeID:                            woxSetting.ThemeId.Get(),
		AppFontFamily:                      woxSetting.AppFontFamily.Get(),
		EnableQueryCompletionHint:          woxSetting.EnableQueryCompletionHint.Get(),
		EnableGlance:                       woxSetting.EnableGlance.Get(),
		PrimaryGlance:                      woxSetting.PrimaryGlance.Get(),
		HideGlanceIcon:                     woxSetting.HideGlanceIcon.Get(),
		ShowScoreTail:                      woxSetting.ShowScoreTail.Get(),
		ShowPerformanceTail:                woxSetting.ShowPerformanceTail.Get(),
		ShowPerformanceTailBatch:           woxSetting.ShowPerformanceTailBatch.Get(),
		ShowPerformanceTailPluginQuery:     woxSetting.ShowPerformanceTailPluginQuery.Get(),
		ShowPerformanceTailBackendPrepared: woxSetting.ShowPerformanceTailBackendPrepared.Get(),
		ShowPerformanceTailUIReceived:      woxSetting.ShowPerformanceTailUiReceived.Get(),
	}, nil
}

// AvailableLanguages returns the supported UI language catalog.
func (s *CoreServices) AvailableLanguages(ctx context.Context, sessionID string) ([]i18n.Lang, error) {
	_ = uiServiceContext(ctx, sessionID)
	return append([]i18n.Lang(nil), i18n.GetSupportedLanguages()...), nil
}

// LanguageJSON returns one validated translation bundle.
func (s *CoreServices) LanguageJSON(ctx context.Context, sessionID string, langCode i18n.LangCode) (string, error) {
	if !i18n.IsSupportedLangCode(string(langCode)) {
		return "", fmt.Errorf("unsupported lang code: %s", langCode)
	}
	return i18n.GetI18nManager().GetLangJson(uiServiceContext(ctx, sessionID), langCode)
}

// UpdateGeneralSetting applies one string-encoded setting while preserving runtime side effects.
func (s *CoreServices) UpdateGeneralSetting(ctx context.Context, sessionID string, key string, value string) error {
	ctx = uiServiceContext(ctx, sessionID)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if key == "ReleaseChannel" {
		updatedValue, err := updateWoxSettingValue(ctx, woxSetting, key, value)
		if err != nil {
			return err
		}
		GetUIManager().PostSettingUpdate(ctx, key, updatedValue)
		return privacy.RefreshPreservedSettings(woxSetting)
	}

	boolValue, _ := strconv.ParseBool(value)
	floatValue, _ := strconv.ParseFloat(value, 64)
	updatedValue := value

	// Register hotkeys before persistence so a failed system bind cannot leave settings ahead of the OS state.
	switch key {
	case "MainHotkey":
		if value != woxSetting.MainHotkey.Get() {
			if err := GetUIManager().RegisterMainHotkey(ctx, value); err != nil {
				return err
			}
		}
		woxSetting.MainHotkey.Set(value)
		return privacy.RefreshPreservedSettings(woxSetting)
	case "SelectionHotkey":
		if value != woxSetting.SelectionHotkey.Get() {
			if err := GetUIManager().RegisterSelectionHotkey(ctx, value); err != nil {
				return err
			}
		}
		woxSetting.SelectionHotkey.Set(value)
		return privacy.RefreshPreservedSettings(woxSetting)
	case "QueryHotkeys":
		queryHotkeys, err := parseQueryHotkeysSettingValue(value)
		if err != nil {
			return err
		}
		config := corehotkey.WoxConfigFromSetting(woxSetting)
		config.QueryHotkeys = queryHotkeys
		if err := GetUIManager().registerWoxHotkeys(ctx, config, true); err != nil {
			return err
		}
		woxSetting.QueryHotkeys.Set(queryHotkeys)
		return nil
	}

	switch key {
	case "EnableAutostart":
		woxSetting.EnableAutostart.Set(boolValue)
	case "IgnoredHotkeyApps":
		var ignoredApps []setting.IgnoredHotkeyApp
		if err := json.Unmarshal([]byte(value), &ignoredApps); err != nil {
			return err
		}
		woxSetting.IgnoredHotkeyApps.Set(normalizeIgnoredHotkeyApps(ignoredApps))
	case "LogLevel":
		updatedValue = util.NormalizeLogLevel(value)
		if err := woxSetting.LogLevel.Set(updatedValue); err != nil {
			return err
		}
	case "UsePinYin":
		woxSetting.UsePinYin.Set(boolValue)
	case "SwitchInputMethodABC":
		woxSetting.SwitchInputMethodABC.Set(boolValue)
	case "HideOnStart":
		woxSetting.HideOnStart.Set(boolValue)
	case "OnboardingFinished":
		woxSetting.OnboardingFinished.Set(boolValue)
	case "HideOnLostFocus":
		woxSetting.HideOnLostFocus.Set(boolValue)
	case "ShowTray":
		woxSetting.ShowTray.Set(boolValue)
	case "LangCode":
		woxSetting.LangCode.Set(i18n.LangCode(value))
	case "QueryShortcuts":
		var shortcuts []setting.QueryShortcut
		if err := json.Unmarshal([]byte(value), &shortcuts); err != nil {
			return err
		}
		woxSetting.QueryShortcuts.Set(shortcuts)
	case "CloudSyncServerUrl":
		serverURL := strings.TrimSpace(value)
		woxSetting.CloudSyncServerUrl.Set(serverURL)
		if err := applyCloudSyncServerURL(ctx, serverURL); err != nil {
			return err
		}
	case "CloudSyncDisabledPlugins":
		var disabledPlugins []string
		if err := json.Unmarshal([]byte(value), &disabledPlugins); err != nil {
			return err
		}
		woxSetting.CloudSyncDisabledPlugins.Set(disabledPlugins)
	case "TrayQueries":
		trayQueries, err := decodeTrayQueries(value)
		if err != nil {
			return err
		}
		woxSetting.TrayQueries.Set(trayQueries)
	case "LaunchMode":
		woxSetting.LaunchMode.Set(setting.LaunchMode(value))
	case "StartPage":
		woxSetting.StartPage.Set(setting.StartPage(value))
	case "ShowPosition":
		woxSetting.ShowPosition.Set(setting.PositionType(value))
	case "AIProviders":
		var providers []setting.AIProvider
		if err := json.Unmarshal([]byte(value), &providers); err != nil {
			return err
		}
		woxSetting.AIProviders.Set(providers)
	case "AIMCPServers":
		var servers []common.AIChatMCPServerConfig
		if err := json.Unmarshal([]byte(value), &servers); err != nil {
			return err
		}
		woxSetting.AIMCPServers.Set(servers)
	case "AISkills":
		var skills []common.Skill
		if err := json.Unmarshal([]byte(value), &skills); err != nil {
			return err
		}
		woxSetting.AISkills.Set(skills)
	case "EnableAutoBackup":
		woxSetting.EnableAutoBackup.Set(boolValue)
	case "EnableAutoUpdate":
		woxSetting.EnableAutoUpdate.Set(boolValue)
	case "CustomPythonPath":
		if strings.TrimSpace(value) != "" {
			if _, err := pluginhost.ValidatePythonExecutable(ctx, value); err != nil {
				return err
			}
		}
		woxSetting.CustomPythonPath.Set(value)
	case "CustomNodejsPath":
		if strings.TrimSpace(value) != "" {
			if _, err := pluginhost.ValidateNodejsExecutable(ctx, value); err != nil {
				return err
			}
		}
		woxSetting.CustomNodejsPath.Set(value)
	case "HttpProxyEnabled":
		woxSetting.HttpProxyEnabled.Set(boolValue)
	case "HttpProxyUrl":
		woxSetting.HttpProxyUrl.Set(value)
	case "AppWidth":
		woxSetting.AppWidth.Set(int(floatValue))
	case "MaxResultCount":
		woxSetting.MaxResultCount.Set(int(floatValue))
	case "UiDensity":
		density := setting.NormalizeUiDensity(value)
		updatedValue = string(density)
		if err := woxSetting.UiDensity.Set(density); err != nil {
			return err
		}
	case "ThemeId":
		woxSetting.ThemeId.Set(value)
	case "AppFontFamily":
		normalized := font.NormalizeConfiguredFontFamily(value, font.GetSystemFontFamilies(ctx))
		woxSetting.AppFontFamily.Set(normalized)
	case "EnableQueryCompletionHint":
		woxSetting.EnableQueryCompletionHint.Set(boolValue)
	case "EnableGlance":
		woxSetting.EnableGlance.Set(boolValue)
	case "PrimaryGlance":
		var glance setting.GlanceRef
		if err := json.Unmarshal([]byte(value), &glance); err != nil {
			return err
		}
		woxSetting.PrimaryGlance.Set(glance)
	case "HideGlanceIcon":
		woxSetting.HideGlanceIcon.Set(boolValue)
	case "ShowScoreTail":
		woxSetting.ShowScoreTail.Set(boolValue)
	case "ShowPerformanceTail":
		woxSetting.ShowPerformanceTail.Set(boolValue)
	case "ShowPerformanceTailBatch":
		woxSetting.ShowPerformanceTailBatch.Set(boolValue)
	case "ShowPerformanceTailPluginQuery":
		woxSetting.ShowPerformanceTailPluginQuery.Set(boolValue)
	case "ShowPerformanceTailBackendPrepared":
		woxSetting.ShowPerformanceTailBackendPrepared.Set(boolValue)
	case "ShowPerformanceTailUiReceived":
		woxSetting.ShowPerformanceTailUiReceived.Set(boolValue)
	case "EnableAnonymousUsageStats":
		woxSetting.EnableAnonymousUsageStats.Set(boolValue)
		if !boolValue {
			telemetry.DeleteTelemetryState(ctx)
		}
	case "EnablePrivacyMode":
		if err := privacy.SetEnabled(boolValue, woxSetting); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown setting key: %s", key)
	}

	if key != "EnablePrivacyMode" {
		if err := privacy.RefreshPreservedSettings(woxSetting); err != nil {
			return err
		}
	}
	GetUIManager().PostSettingUpdate(ctx, key, updatedValue)
	return nil
}

func decodeTrayQueries(value string) ([]setting.TrayQuery, error) {
	var rawQueries []map[string]any
	if err := json.Unmarshal([]byte(value), &rawQueries); err != nil {
		return nil, err
	}
	queries := make([]setting.TrayQuery, 0, len(rawQueries))
	for _, raw := range rawQueries {
		queryText, _ := raw["Query"].(string)
		query := setting.TrayQuery{Query: queryText}
		query.HideQueryBox = parseBool(raw["HideQueryBox"])
		query.HideToolbar = parseBool(raw["HideToolbar"])
		query.Disabled = parseBool(raw["Disabled"])
		query.Width = maxInt(parseInt(raw["Width"]), 0)
		query.MaxResultCount = normalizeOptionalMaxResultCount(parseInt(raw["MaxResultCount"]))
		if rawIcon, ok := raw["Icon"]; ok {
			switch icon := rawIcon.(type) {
			case map[string]any:
				iconData, err := json.Marshal(icon)
				if err == nil {
					_ = json.Unmarshal(iconData, &query.Icon)
				}
			case string:
				if parsed, err := common.ParseWoxImage(icon); err == nil {
					query.Icon = parsed
				}
			}
		}
		queries = append(queries, query)
	}
	return queries, nil
}
