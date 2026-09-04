package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/util"
)

// ignoredApp is one exact-app IgnoreRules row. Paths differ across operating
// systems, so IgnoreRules stays a platform-specific setting.
type ignoredApp struct {
	Name     string          `json:"Name"`
	Identity string          `json:"Identity"`
	Path     string          `json:"Path"`
	Icon     common.WoxImage `json:"Icon"`
}

// ignoredAppMatchKey prefers a filesystem path so two shortcuts to the same
// executable can be hidden independently. Identity is only used when no path
// exists, which covers UWP and Settings entries.
func ignoredAppMatchKey(appPath string, identity string) string {
	if key := appPathMatchKey(appPath); key != "" {
		return "path:" + key
	}
	if identity = strings.ToLower(strings.TrimSpace(identity)); identity != "" {
		return "identity:" + identity
	}
	return ""
}

func appPathMatchKey(appPath string) string {
	appPath = strings.TrimSpace(appPath)
	if appPath == "" {
		return ""
	}

	cleaned := filepath.Clean(appPath)
	if util.IsWindows() {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

func isIgnoredApp(info appInfo, ignored []ignoredApp) bool {
	if len(ignored) == 0 {
		return false
	}

	infoKey := ignoredAppMatchKey(info.Path, info.Identity)
	if infoKey == "" {
		return false
	}

	for _, app := range ignored {
		if ignoredAppMatchKey(app.Path, app.Identity) == infoKey {
			return true
		}
	}
	return false
}

func (a *ApplicationPlugin) getIgnoredAppsSnapshot() []ignoredApp {
	a.queryEntriesMutex.RLock()
	apps := a.ignoredApps
	a.queryEntriesMutex.RUnlock()
	return apps
}

func (a *ApplicationPlugin) toIgnoredApp(info appInfo, displayName string) ignoredApp {
	name := strings.TrimSpace(displayName)
	if name == "" || strings.HasPrefix(name, "i18n:") {
		name = strings.TrimSpace(info.Name)
	}
	if name == "" {
		name = strings.TrimSpace(info.Path)
	}
	if name == "" {
		name = strings.TrimSpace(info.Identity)
	}

	icon := info.Icon
	if icon.IsEmpty() {
		icon = common.PluginAppIcon
	}

	return ignoredApp{
		Name:     name,
		Identity: strings.TrimSpace(info.Identity),
		Path:     strings.TrimSpace(info.Path),
		Icon:     icon,
	}
}

func ignoreRuleHidesApp(info appInfo, displayName string, rules []appIgnoreRule) bool {
	matchers, apps := splitIgnoreRules(rules)
	if isIgnoredApp(info, apps) {
		return true
	}
	candidates := buildIgnoreRuleCandidates(info, displayName)
	for _, candidate := range candidates {
		for _, matcher := range matchers {
			if matcher.regex.MatchString(candidate) {
				return true
			}
		}
	}
	return false
}

// hideAppFromSearch appends one exact-app IgnoreRules row for the current platform.
func (a *ApplicationPlugin) hideAppFromSearch(ctx context.Context, info appInfo, displayName string) {
	current, err := parseIgnoreRules(a.api.GetSetting(ctx, ignoreRulesSettingKey))
	if err != nil {
		a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to parse %s: %s", ignoreRulesSettingKey, err.Error()))
		a.api.Notify(ctx, "i18n:plugin_app_hide_failed")
		return
	}

	if ignoreRuleHidesApp(info, displayName, current) {
		a.api.Notify(ctx, "i18n:plugin_app_already_hidden")
		return
	}

	hidden := a.toIgnoredApp(info, displayName)
	current = append(current, appIgnoreRule{Pattern: hidden.Name, IncludeFuture: false, Apps: []ignoredApp{hidden}})
	payload, err := json.Marshal(current)
	if err != nil {
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to encode %s: %s", ignoreRulesSettingKey, err.Error()))
		a.api.Notify(ctx, "i18n:plugin_app_hide_failed")
		return
	}

	result := a.api.SetSetting(ctx, plugin.SetSettingOption{
		Key:              ignoreRulesSettingKey,
		Value:            string(payload),
		PlatformSpecific: true,
	})
	if !result.Success {
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to save %s: %s", ignoreRulesSettingKey, result.ErrMsg))
		a.api.Notify(ctx, "i18n:plugin_app_hide_failed")
		return
	}

	a.api.Notify(ctx, "i18n:plugin_app_hide_completed")
}
