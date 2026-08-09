package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"wox/account"
	"wox/cloudsync"
	"wox/i18n"
	"wox/setting"
	"wox/updater"
	"wox/util"
)

// parseQueryHotkeysSettingValue normalizes query hotkeys before registration and persistence.
func parseQueryHotkeysSettingValue(value string) ([]setting.QueryHotkey, error) {
	var rawQueryHotkeys []map[string]any
	if err := json.Unmarshal([]byte(value), &rawQueryHotkeys); err != nil {
		return nil, err
	}

	queryHotkeys := make([]setting.QueryHotkey, 0, len(rawQueryHotkeys))
	for _, rawQueryHotkey := range rawQueryHotkeys {
		queryHotkey := setting.QueryHotkey{Position: setting.QueryHotkeyPositionSystemDefault}
		if rawName, ok := rawQueryHotkey["Name"]; ok {
			queryHotkey.Name = strings.TrimSpace(parseString(rawName))
		}
		if rawHotkey, ok := rawQueryHotkey["Hotkey"]; ok {
			queryHotkey.Hotkey = strings.TrimSpace(parseString(rawHotkey))
		}
		if rawQuery, ok := rawQueryHotkey["Query"]; ok {
			queryHotkey.Query = parseString(rawQuery)
		}
		if rawSilentExecution, ok := rawQueryHotkey["IsSilentExecution"]; ok {
			queryHotkey.IsSilentExecution = parseBool(rawSilentExecution)
		}
		if rawHideQueryBox, ok := rawQueryHotkey["HideQueryBox"]; ok {
			queryHotkey.HideQueryBox = parseBool(rawHideQueryBox)
		}
		if rawHideToolbar, ok := rawQueryHotkey["HideToolbar"]; ok {
			queryHotkey.HideToolbar = parseBool(rawHideToolbar)
		}
		if rawDisabled, ok := rawQueryHotkey["Disabled"]; ok {
			queryHotkey.Disabled = parseBool(rawDisabled)
		}
		if rawWidth, ok := rawQueryHotkey["Width"]; ok {
			queryHotkey.Width = maxInt(parseInt(rawWidth), 0)
		}
		if rawMaxResultCount, ok := rawQueryHotkey["MaxResultCount"]; ok {
			queryHotkey.MaxResultCount = normalizeOptionalMaxResultCount(parseInt(rawMaxResultCount))
		}
		if rawPosition, ok := rawQueryHotkey["Position"]; ok {
			queryHotkey.Position = normalizeQueryHotkeyPosition(parseString(rawPosition))
		}
		queryHotkeys = append(queryHotkeys, queryHotkey)
	}
	return queryHotkeys, nil
}

// updateWoxSettingValue handles shared setting writes that require normalization.
func updateWoxSettingValue(_ context.Context, woxSetting *setting.WoxSetting, key string, value string) (string, error) {
	switch key {
	case "ReleaseChannel":
		normalizedChannel := setting.NormalizeReleaseChannel(value)
		if err := woxSetting.ReleaseChannel.Set(normalizedChannel); err != nil {
			return "", err
		}
		updater.ResetUpdateInfoForReleaseChannel(normalizedChannel)
		return string(normalizedChannel), nil
	default:
		return "", fmt.Errorf("unknown setting key: %s", key)
	}
}

// accountRequestLang maps Wox locales to the language set supported by the account API.
func accountRequestLang(lang string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(lang), "_", "-"))
	if normalized == "" {
		normalized = strings.ToLower(strings.ReplaceAll(string(i18n.GetI18nManager().GetCurrentLangCode()), "_", "-"))
	}
	if strings.HasPrefix(normalized, "zh") {
		return "zh"
	}
	return "en"
}

func applyCloudSyncServerURL(ctx context.Context, url string) error {
	baseURL := resolveCloudSyncServerURL(url)
	changed := false
	accountService := account.GetService()
	if accountService != nil && accountService.BaseURL() != baseURL {
		changed = true
	}
	if cloudService := cloudsync.GetService(); cloudService != nil && cloudService.Client != nil && cloudService.Client.BaseURL() != baseURL {
		changed = true
	}
	if !changed {
		return nil
	}

	if cloudService := cloudsync.GetService(); cloudService != nil {
		if err := cloudService.ResetLocalState(ctx); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to reset cloud sync state after server change: %v", err))
		}
		if cloudService.Client != nil {
			cloudService.Client.SetBaseURL(baseURL)
		}
	}
	if accountService == nil {
		return nil
	}
	accountService.SetBaseURL(baseURL)
	return accountService.ResetLocalSession(ctx)
}

func resolveCloudSyncServerURL(url string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(url), "/")
	if trimmed == "" {
		return "https://sync.woxlauncher.com"
	}
	return trimmed
}

func normalizeIgnoredHotkeyApps(apps []setting.IgnoredHotkeyApp) []setting.IgnoredHotkeyApp {
	normalized := make([]setting.IgnoredHotkeyApp, 0, len(apps))
	seen := make(map[string]bool)
	for _, app := range apps {
		app.Name = strings.TrimSpace(app.Name)
		app.Identity = strings.TrimSpace(app.Identity)
		app.Path = strings.TrimSpace(app.Path)
		if app.Identity == "" {
			continue
		}
		key := strings.ToLower(app.Identity)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, app)
	}
	return normalized
}

func parseString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func parseBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return err == nil && parsed
	default:
		return false
	}
}

func parseInt(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case float32:
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func normalizeOptionalMaxResultCount(value int) int {
	if value <= 0 {
		return 0
	}
	return clampInt(value, 5, 15)
}

func normalizeQueryHotkeyPosition(value string) setting.QueryHotkeyPosition {
	position := setting.QueryHotkeyPosition(strings.TrimSpace(value))
	switch position {
	case setting.QueryHotkeyPositionTopLeft,
		setting.QueryHotkeyPositionTopCenter,
		setting.QueryHotkeyPositionTopRight,
		setting.QueryHotkeyPositionMiddleLeft,
		setting.QueryHotkeyPositionCenter,
		setting.QueryHotkeyPositionMiddleRight,
		setting.QueryHotkeyPositionBottomLeft,
		setting.QueryHotkeyPositionBottomCenter,
		setting.QueryHotkeyPositionBottomRight:
		return position
	default:
		return setting.QueryHotkeyPositionSystemDefault
	}
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
