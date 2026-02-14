package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"wox/ai"
	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/plugin/host"
	"wox/setting"
	"wox/ui/dto"
	"wox/updater"
	"wox/util"
	"wox/util/font"
	"wox/util/hotkey"
	"wox/util/shell"

	"github.com/jinzhu/copier"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

var routers = map[string]func(w http.ResponseWriter, r *http.Request){
	// plugins
	"/plugin/store":     handlePluginStore,
	"/plugin/installed": handlePluginInstalled,
	"/plugin/install":   handlePluginInstall,
	"/plugin/uninstall": handlePluginUninstall,
	"/plugin/disable":   handlePluginDisable,
	"/plugin/enable":    handlePluginEnable,
	"/plugin/detail":    handlePluginDetail,

	//	themes
	"/theme":           handleTheme,
	"/theme/store":     handleThemeStore,
	"/theme/installed": handleThemeInstalled,
	"/theme/install":   handleThemeInstall,
	"/theme/uninstall": handleThemeUninstall,
	"/theme/apply":     handleThemeApply,

	// settings
	"/setting/wox":                      handleSettingWox,
	"/setting/wox/update":               handleSettingWoxUpdate,
	"/setting/ui/fonts":                 handleSettingUIFontList,
	"/setting/plugin/update":            handleSettingPluginUpdate,
	"/setting/userdata/location":        handleUserDataLocation,
	"/setting/userdata/location/update": handleUserDataLocationUpdate,
	"/setting/position":                 handleSaveWindowPosition,
	"/runtime/status":                   handleRuntimeStatus,

	// events
	"/on/focus/lost":     handleOnFocusLost,
	"/on/ready":          handleOnUIReady,
	"/on/show":           handleOnShow,
	"/on/querybox/focus": handleOnQueryBoxFocus,
	"/on/hide":           handleOnHide,
	"/on/setting":        handleOnSetting,
	"/usage/stats":       handleUsageStats,

	// lang
	"/lang/available": handleLangAvailable,
	"/lang/json":      handleLangJson,

	// ai
	"/ai/providers":     handleAIProviders,
	"/ai/models":        handleAIModels,
	"/ai/model/default": handleAIDefaultModel,
	"/ai/ping":          handleAIPing,
	"/ai/chat":          handleAIChat,
	"/ai/mcp/tools":     handleAIMCPServerTools,
	"/ai/mcp/tools/all": handleAIMCPServerToolsAll,
	"/ai/agents":        handleAIAgents,

	// doctor
	"/doctor/check": handleDoctorCheck,

	// others
	"/":                 handleHome,
	"/show":             handleShow,
	"/ping":             handlePing,
	"/preview":          handlePreview,
	"/open":             handleOpen,
	"/backup/now":       handleBackupNow,
	"/backup/restore":   handleBackupRestore,
	"/backup/all":       handleBackupAll,
	"/backup/folder":    handleBackupFolder,
	"/log/clear":        handleLogClear,
	"/log/open":         handleLogOpen,
	"/hotkey/available": handleHotkeyAvailable,
	"/query/metadata":   handleQueryMetadata,
	"/deeplink":         handleDeeplink,
	"/version":          handleVersion,

	// toolbar snooze/mute
	"/toolbar/snooze": handleToolbarSnooze,
}

const traceIdHeader = "TraceId"
const sessionIdHeader = "SessionId"

func getTraceContext(r *http.Request) context.Context {
	traceId := strings.TrimSpace(r.Header.Get(traceIdHeader))
	sessionId := getSessionIdFromHeader(r)
	var ctx context.Context
	if traceId != "" {
		ctx = util.NewTraceContextWith(traceId)
	} else {
		ctx = util.NewTraceContext()
	}

	if sessionId != "" {
		ctx = util.WithSessionContext(ctx, sessionId)
	}

	return ctx
}

func getSessionIdFromHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(sessionIdHeader))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, "Wox")
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, "pong")
}

func handlePreview(w http.ResponseWriter, r *http.Request) {
	sessionId := r.URL.Query().Get("sessionId")
	queryId := r.URL.Query().Get("queryId")
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErrorResponse(w, "id is empty")
		return
	}
	if sessionId == "" {
		writeErrorResponse(w, "sessionId is empty")
		return
	}
	if queryId == "" {
		writeErrorResponse(w, "queryId is empty")
		return
	}

	preview, err := plugin.GetPluginManager().GetResultPreview(getTraceContext(r), sessionId, queryId, id)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, preview)
}

func handleTheme(w http.ResponseWriter, r *http.Request) {
	theme := GetUIManager().GetCurrentTheme(getTraceContext(r))
	writeSuccessResponse(w, theme)
}

func handlePluginStore(w http.ResponseWriter, r *http.Request) {
	getCtx := getTraceContext(r)
	manifests := plugin.GetStoreManager().GetStorePluginManifests(getTraceContext(r))
	var plugins = make([]dto.PluginDto, len(manifests))
	copyErr := copier.Copy(&plugins, &manifests)
	if copyErr != nil {
		writeErrorResponse(w, copyErr.Error())
		return
	}

	for i, storePlugin := range plugins {
		pluginInstance, isInstalled := lo.Find(plugin.GetPluginManager().GetPluginInstances(), func(item *plugin.Instance) bool {
			return item.Metadata.Id == storePlugin.Id
		})
		// Support both IconUrl and IconEmoji, prefer IconEmoji if both are present
		if manifests[i].IconEmoji != "" {
			plugins[i].Icon = common.NewWoxImageEmoji(manifests[i].IconEmoji)
		} else if manifests[i].IconUrl != "" {
			plugins[i].Icon = common.NewWoxImageUrl(manifests[i].IconUrl)
		}
		plugins[i].IsInstalled = isInstalled
		plugins[i].Name = manifests[i].GetName(getCtx)
		plugins[i].NameEn = manifests[i].GetNameEn(getCtx)
		plugins[i].Description = manifests[i].GetDescription(getCtx)
		plugins[i].DescriptionEn = manifests[i].GetDescriptionEn(getCtx)

		plugins[i] = convertPluginDto(getCtx, plugins[i], pluginInstance)
	}

	writeSuccessResponse(w, plugins)
}

func handlePluginInstalled(w http.ResponseWriter, r *http.Request) {
	defer util.GoRecover(getTraceContext(r), "get installed plugins")

	getCtx := getTraceContext(r)
	instances := plugin.GetPluginManager().GetPluginInstances()
	var plugins []dto.PluginDto
	for _, pluginInstance := range instances {
		installedPlugin, err := convertPluginInstanceToDto(getCtx, pluginInstance)
		if err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
		plugins = append(plugins, installedPlugin)
	}

	writeSuccessResponse(w, plugins)
}

func convertPluginInstanceToDto(ctx context.Context, pluginInstance *plugin.Instance) (installedPlugin dto.PluginDto, err error) {
	copyErr := copier.Copy(&installedPlugin, &pluginInstance.Metadata)
	if copyErr != nil {
		return dto.PluginDto{}, copyErr
	}
	installedPlugin.Name = pluginInstance.GetName(ctx)
	installedPlugin.NameEn = pluginInstance.Metadata.GetNameEn(ctx)
	installedPlugin.Description = pluginInstance.GetDescription(ctx)
	installedPlugin.DescriptionEn = pluginInstance.Metadata.GetDescriptionEn(ctx)

	installedPlugin.IsSystem = pluginInstance.IsSystemPlugin
	installedPlugin.IsDev = pluginInstance.IsDevPlugin
	installedPlugin.IsInstalled = true
	installedPlugin.IsDisable = pluginInstance.Setting.Disabled.Get()
	installedPlugin.TriggerKeywords = pluginInstance.GetTriggerKeywords()
	installedPlugin.Commands = pluginInstance.GetQueryCommands()

	//load screenshot urls from store if exist
	storePlugin, foundErr := plugin.GetStoreManager().GetStorePluginManifestById(ctx, pluginInstance.Metadata.Id)
	if foundErr == nil {
		installedPlugin.ScreenshotUrls = storePlugin.ScreenshotUrls
	} else {
		installedPlugin.ScreenshotUrls = []string{}
	}

	// load icon
	iconImg, parseErr := common.ParseWoxImage(pluginInstance.Metadata.Icon)
	if parseErr == nil {
		installedPlugin.Icon = iconImg
	} else {
		installedPlugin.Icon = common.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAAELUlEQVR4nO3ZW2xTdRwH8JPgkxE1XuKFQUe73rb1IriNyYOJvoiALRszvhqffHBLJjEx8Q0TlRiN0RiNrPd27boLY1wUFAQHyquJiTIYpefay7au2yiJG1/zb6Kx/ZfS055T1mS/5JtzXpr+Pufy//9P/gyzURulXIHp28Rp7H5eY/OSc6bRiiPNN9tBQs4bDsFrbN5/AQ2JANO3qRgx9dZ76I2vwingvsQhgHUK2NPQCKeAuOw7Mf72B1hPCEZu9bBrWE8IRm6RH60nBFMNQA3Eh6kVzCzzyOVu5I+HUyvqApREkOZxe5bKR+kVdQFKIcgVLwW4usyrD1ACcSsXKwm4lYvVB1Ar4r7fAWeNCPLClgIcruBFVhRQK4Jc8Vwulj/WZRQqh4i8+X5d5glGaYCDBzp/WYQ5KsJ98JDqCEZJgIO/g53nM9BHpXxMEQHuXnURjFIA0vyOHxfQMiIVxBgW4FIRwSgBcLB3YPt+DrqwWDKGEA9Xz7tlES/9nkPHuQyeP5/By3/crh9gf3wNlpMpaENC2egDHFwHSiBurqL78hLaJlNoPZaCeSIJ01gSu68sqw/YF1uDeTKF5qBQUXR+DkNFiOgbg7BOSBTAOJrIw1QD7J1dzf+Jxs/LitbL4qhjsAAROjAA67hEAQzRBLovLSkPePX6an6U2eblqorWE4en7xCFsIxJFEAfkcoiZANeufo3tMMitnq4qkIArZMp2E+k4H29COEcQPuoSAH0YQm7prPKAMhjsMXFVpUmN4f2qTRsp+dgPTUH21SyJKJtRKQALcNiSYRswLNH46gmTW4W7SfSsP8w/x/AcjIN6/EkvEWPU9AxgNaIQAF0IRFdF7O1AZ75Lg65aXKxsJxKw35mngIQlOVYoiTCHBYogDZQiJANePrbm5AT8uhYT8/hubPzdwW0HU/BMpGA5yCNMIV4CrDdL6DzQrY6wFPfxFBpSPOkabLEuBeAzAOWcYIonCcCr/XDFOQpQLOPIBblA578OoZKQprfcXYeO88tVAwg80D7mARPL40wBjgKoPHy8gFPfHUD90q++Z8W8usauQAyD7RFJbiLEfv7YfBztQMe/3IW5bLFzeYb7/g5UzXANJZE64gIdw+N0Hu52gCPfTGLu2Wrm0XHhQw6Ly7WDDCOJmCOiNQq1r+vHy0etnrAo59fR6mQcZ40Tr7GlAIYogmYhgVqFUsQOjdbHeCRz66hONt8HLqmF9E1nVUcoI9IMIUIYpBGuOLyAQ9/eg3/jybAo/tyFrsuZVUD6MMSjEGeQvj2vgMwLz4gC7D5yAy7+cgMSLYHBbzw2xK6f1Uf0DIswhDgMeQsRPAae0QW4sGP/9zz0Cd/seRTcfeVpboCdCEReh+PIUeNiHWx7dtsDxQibF6mkQpFCHLONOYGvM1Hmm+obd+NYhqg/gG2aOxED6eh5gAAAABJRU5ErkJggg==`)
	}
	installedPlugin.Icon = common.ConvertIcon(ctx, installedPlugin.Icon, pluginInstance.PluginDirectory)

	installedPlugin = convertPluginDto(ctx, installedPlugin, pluginInstance)

	return installedPlugin, nil
}

func handlePluginInstall(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "Plugin ID is required for installation")
		return
	}

	pluginId := idResult.String()

	plugins := plugin.GetStoreManager().GetStorePluginManifests(ctx)
	findPlugin, exist := lo.Find(plugins, func(item plugin.StorePluginManifest) bool {
		return item.Id == pluginId
	})
	if !exist {
		writeErrorResponse(w, fmt.Sprintf("Plugin '%s' not found in the store", pluginId))
		return
	}

	pluginName := findPlugin.GetName(ctx)
	logger.Info(ctx, fmt.Sprintf("Installing plugin '%s' (%s)", pluginName, pluginId))
	installErr := plugin.GetStoreManager().Install(ctx, findPlugin)
	if installErr != nil {
		errMsg := fmt.Sprintf("Failed to install plugin '%s': %s", pluginName, installErr.Error())
		logger.Error(ctx, errMsg)
		writeErrorResponse(w, errMsg)
		return
	}

	logger.Info(ctx, fmt.Sprintf("Successfully installed plugin '%s' (%s)", pluginName, pluginId))
	writeSuccessResponse(w, "")
}

func handlePluginUninstall(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	pluginId := idResult.String()

	plugins := plugin.GetPluginManager().GetPluginInstances()
	findPlugin, exist := lo.Find(plugins, func(item *plugin.Instance) bool {
		if item.Metadata.Id == pluginId {
			return true
		}
		return false
	})
	if !exist {
		writeErrorResponse(w, "can't find plugin")
		return
	}

	uninstallErr := plugin.GetStoreManager().Uninstall(ctx, findPlugin, false)
	if uninstallErr != nil {
		writeErrorResponse(w, "can't uninstall plugin: "+uninstallErr.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handlePluginDisable(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	pluginId := idResult.String()

	plugins := plugin.GetPluginManager().GetPluginInstances()
	findPlugin, exist := lo.Find(plugins, func(item *plugin.Instance) bool {
		if item.Metadata.Id == pluginId {
			return true
		}
		return false
	})
	if !exist {
		writeErrorResponse(w, "can't find plugin")
		return
	}

	findPlugin.Setting.Disabled.Set(true)
	writeSuccessResponse(w, "")
}

func handlePluginEnable(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	pluginId := idResult.String()

	plugins := plugin.GetPluginManager().GetPluginInstances()
	findPlugin, exist := lo.Find(plugins, func(item *plugin.Instance) bool {
		return item.Metadata.Id == pluginId
	})
	if !exist {
		writeErrorResponse(w, "can't find plugin")
		return
	}

	findPlugin.Setting.Disabled.Set(false)
	writeSuccessResponse(w, "")
}

func handleThemeStore(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	storeThemes := GetStoreManager().GetThemes()
	var themes = make([]dto.ThemeDto, len(storeThemes))
	copyErr := copier.Copy(&themes, &storeThemes)
	if copyErr != nil {
		writeErrorResponse(w, copyErr.Error())
		return
	}

	for i, storeTheme := range themes {
		isInstalled := lo.ContainsBy(GetUIManager().GetAllThemes(ctx), func(item common.Theme) bool {
			return item.ThemeId == storeTheme.ThemeId
		})
		themes[i].IsUpgradable = GetUIManager().IsThemeUpgradable(storeTheme.ThemeId, storeTheme.Version)
		themes[i].IsInstalled = isInstalled
		themes[i].IsSystem = GetUIManager().IsSystemTheme(storeTheme.ThemeId)
	}

	writeSuccessResponse(w, themes)
}

func handleThemeInstalled(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	installedThemes := GetUIManager().GetAllThemes(ctx)
	var themes = make([]dto.ThemeDto, len(installedThemes))
	copyErr := copier.Copy(&themes, &installedThemes)
	if copyErr != nil {
		writeErrorResponse(w, copyErr.Error())
		return
	}

	for i, storeTheme := range themes {
		themes[i].IsInstalled = true
		themes[i].IsUpgradable = GetUIManager().IsThemeUpgradable(storeTheme.ThemeId, storeTheme.Version)
		themes[i].IsSystem = GetUIManager().IsSystemTheme(storeTheme.ThemeId)
	}
	writeSuccessResponse(w, themes)
}

func handleThemeInstall(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	themeId := idResult.String()

	storeThemes := GetStoreManager().GetThemes()
	findTheme, exist := lo.Find(storeThemes, func(item common.Theme) bool {
		if item.ThemeId == themeId {
			return true
		}
		return false
	})
	if !exist {
		writeErrorResponse(w, "can't find theme in theme store")
		return
	}

	installErr := GetStoreManager().Install(ctx, findTheme)
	if installErr != nil {
		writeErrorResponse(w, "can't install theme: "+installErr.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleThemeUninstall(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	themeId := idResult.String()

	storeThemes := GetUIManager().GetAllThemes(ctx)
	findTheme, exist := lo.Find(storeThemes, func(item common.Theme) bool {
		if item.ThemeId == themeId {
			return true
		}
		return false
	})
	if !exist {
		writeErrorResponse(w, "can't find theme")
		return
	}

	uninstallErr := GetStoreManager().Uninstall(ctx, findTheme)
	if uninstallErr != nil {
		writeErrorResponse(w, "can't uninstall theme: "+uninstallErr.Error())
		return
	} else {
		GetUIManager().ChangeToDefaultTheme(ctx)
	}

	writeSuccessResponse(w, "")
}

func handleThemeApply(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	themeId := idResult.String()

	// Find theme in installed themes
	storeThemes := GetUIManager().GetAllThemes(ctx)
	findTheme, exist := lo.Find(storeThemes, func(item common.Theme) bool {
		return item.ThemeId == themeId
	})
	if !exist {
		writeErrorResponse(w, "can't find theme")
		return
	}

	GetUIManager().ChangeTheme(ctx, findTheme)
	writeSuccessResponse(w, "")
}

func handleSettingWox(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)

	var settingDto dto.WoxSettingDto
	settingDto.EnableAutostart = woxSetting.EnableAutostart.Get()
	settingDto.MainHotkey = woxSetting.MainHotkey.Get()
	settingDto.SelectionHotkey = woxSetting.SelectionHotkey.Get()
	settingDto.LogLevel = util.NormalizeLogLevel(woxSetting.LogLevel.Get())
	settingDto.UsePinYin = woxSetting.UsePinYin.Get()
	settingDto.SwitchInputMethodABC = woxSetting.SwitchInputMethodABC.Get()
	settingDto.HideOnStart = woxSetting.HideOnStart.Get()
	settingDto.HideOnLostFocus = woxSetting.HideOnLostFocus.Get()
	settingDto.ShowTray = woxSetting.ShowTray.Get()
	settingDto.LangCode = woxSetting.LangCode.Get()
	settingDto.QueryHotkeys = woxSetting.QueryHotkeys.Get()
	settingDto.QueryShortcuts = woxSetting.QueryShortcuts.Get()
	settingDto.TrayQueries = woxSetting.TrayQueries.Get()
	settingDto.LaunchMode = woxSetting.LaunchMode.Get()
	settingDto.StartPage = woxSetting.StartPage.Get()
	settingDto.AIProviders = woxSetting.AIProviders.Get()
	settingDto.HttpProxyEnabled = woxSetting.HttpProxyEnabled.Get()
	settingDto.HttpProxyUrl = woxSetting.HttpProxyUrl.Get()
	settingDto.ShowPosition = woxSetting.ShowPosition.Get()
	settingDto.EnableAutoBackup = woxSetting.EnableAutoBackup.Get()
	settingDto.EnableAutoUpdate = woxSetting.EnableAutoUpdate.Get()
	settingDto.CustomPythonPath = woxSetting.CustomPythonPath.Get()
	settingDto.CustomNodejsPath = woxSetting.CustomNodejsPath.Get()

	settingDto.EnableMCPServer = woxSetting.EnableMCPServer.Get()
	settingDto.MCPServerPort = woxSetting.MCPServerPort.Get()

	settingDto.AppWidth = woxSetting.AppWidth.Get()
	settingDto.MaxResultCount = woxSetting.MaxResultCount.Get()
	settingDto.ThemeId = woxSetting.ThemeId.Get()
	appFontFamily := woxSetting.AppFontFamily.Get()
	systemFontFamilies := font.GetSystemFontFamilies(ctx)
	normalizedAppFontFamily := font.NormalizeConfiguredFontFamily(appFontFamily, systemFontFamilies)
	if normalizedAppFontFamily != appFontFamily {
		woxSetting.AppFontFamily.Set(normalizedAppFontFamily)
	}
	settingDto.AppFontFamily = normalizedAppFontFamily

	writeSuccessResponse(w, settingDto)
}

func handleSettingUIFontList(w http.ResponseWriter, r *http.Request) {
	fontFamilies := font.GetSystemFontFamilies(getTraceContext(r))
	writeSuccessResponse(w, fontFamilies)
}

func handleSettingWoxUpdate(w http.ResponseWriter, r *http.Request) {
	type keyValuePair struct {
		Key   string
		Value string
	}

	decoder := json.NewDecoder(r.Body)
	var kv keyValuePair
	err := decoder.Decode(&kv)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	ctx := getTraceContext(r)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)

	var vb bool
	var vf float64
	var vs = kv.Value
	updatedValue := kv.Value
	if vb1, err := strconv.ParseBool(vs); err == nil {
		vb = vb1
	}
	if vf1, err := strconv.ParseFloat(vs, 64); err == nil {
		vf = vf1
	}

	switch kv.Key {
	case "EnableAutostart":
		woxSetting.EnableAutostart.Set(vb)
	case "MainHotkey":
		woxSetting.MainHotkey.Set(vs)
	case "SelectionHotkey":
		woxSetting.SelectionHotkey.Set(vs)
	case "LogLevel":
		updatedValue = util.NormalizeLogLevel(vs)
		if err := woxSetting.LogLevel.Set(updatedValue); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
	case "UsePinYin":
		woxSetting.UsePinYin.Set(vb)
	case "SwitchInputMethodABC":
		woxSetting.SwitchInputMethodABC.Set(vb)
	case "HideOnStart":
		woxSetting.HideOnStart.Set(vb)
	case "HideOnLostFocus":
		woxSetting.HideOnLostFocus.Set(vb)
	case "ShowTray":
		woxSetting.ShowTray.Set(vb)
	case "LangCode":
		woxSetting.LangCode.Set(i18n.LangCode(vs))
	case "QueryHotkeys":
		var queryHotkeys []setting.QueryHotkey
		if err := json.Unmarshal([]byte(vs), &queryHotkeys); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
		woxSetting.QueryHotkeys.Set(queryHotkeys)
	case "QueryShortcuts":
		var queryShortcuts []setting.QueryShortcut
		if err := json.Unmarshal([]byte(vs), &queryShortcuts); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
		woxSetting.QueryShortcuts.Set(queryShortcuts)
	case "TrayQueries":
		var rawTrayQueries []map[string]any
		if err := json.Unmarshal([]byte(vs), &rawTrayQueries); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}

		var trayQueries []setting.TrayQuery
		for _, rawTrayQuery := range rawTrayQueries {
			query, _ := rawTrayQuery["Query"].(string)
			trayQuery := setting.TrayQuery{
				Query: query,
			}

			if rawDisabled, ok := rawTrayQuery["Disabled"]; ok {
				switch disabled := rawDisabled.(type) {
				case bool:
					trayQuery.Disabled = disabled
				case string:
					if parsed, parseErr := strconv.ParseBool(disabled); parseErr == nil {
						trayQuery.Disabled = parsed
					}
				}
			}

			if rawWidth, ok := rawTrayQuery["Width"]; ok {
				switch width := rawWidth.(type) {
				case float64:
					trayQuery.Width = int(width)
				case int:
					trayQuery.Width = width
				case string:
					width = strings.TrimSpace(width)
					if width != "" {
						if parsed, parseErr := strconv.Atoi(width); parseErr == nil {
							trayQuery.Width = parsed
						}
					}
				}
				if trayQuery.Width < 0 {
					trayQuery.Width = 0
				}
			}

			if rawIcon, ok := rawTrayQuery["Icon"]; ok {
				switch icon := rawIcon.(type) {
				case map[string]any:
					iconData, marshalErr := json.Marshal(icon)
					if marshalErr == nil {
						_ = json.Unmarshal(iconData, &trayQuery.Icon)
					}
				case string:
					if parsed, parseErr := common.ParseWoxImage(icon); parseErr == nil {
						trayQuery.Icon = parsed
					}
				}
			}

			trayQueries = append(trayQueries, trayQuery)
		}
		woxSetting.TrayQueries.Set(trayQueries)
	case "LaunchMode":
		woxSetting.LaunchMode.Set(setting.LaunchMode(vs))
	case "StartPage":
		woxSetting.StartPage.Set(setting.StartPage(vs))
	case "ShowPosition":
		woxSetting.ShowPosition.Set(setting.PositionType(vs))
	case "AIProviders":
		var aiProviders []setting.AIProvider
		if err := json.Unmarshal([]byte(vs), &aiProviders); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
		woxSetting.AIProviders.Set(aiProviders)
	case "EnableAutoBackup":
		woxSetting.EnableAutoBackup.Set(vb)
	case "EnableAutoUpdate":
		woxSetting.EnableAutoUpdate.Set(vb)
	case "CustomPythonPath":
		woxSetting.CustomPythonPath.Set(vs)
	case "CustomNodejsPath":
		woxSetting.CustomNodejsPath.Set(vs)

	case "HttpProxyEnabled":
		woxSetting.HttpProxyEnabled.Set(vb)
	case "HttpProxyUrl":
		woxSetting.HttpProxyUrl.Set(vs)

	case "AppWidth":
		woxSetting.AppWidth.Set(int(vf))
	case "MaxResultCount":
		woxSetting.MaxResultCount.Set(int(vf))
	case "ThemeId":
		woxSetting.ThemeId.Set(vs)
	case "AppFontFamily":
		woxSetting.AppFontFamily.Set(vs)
	case "EnableMCPServer":
		woxSetting.EnableMCPServer.Set(vb)
	case "MCPServerPort":
		woxSetting.MCPServerPort.Set(int(vf))
	default:
		writeErrorResponse(w, "unknown setting key: "+kv.Key)
		return
	}

	GetUIManager().PostSettingUpdate(getTraceContext(r), kv.Key, updatedValue)

	writeSuccessResponse(w, "")
}

func handleRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	instances := plugin.GetPluginManager().GetPluginInstances()

	statuses := make([]dto.RuntimeStatusDto, 0, len(plugin.AllHosts))
	for _, runtimeHost := range plugin.AllHosts {
		runtime := string(runtimeHost.GetRuntime(ctx))

		var pluginNames []string
		for _, instance := range instances {
			if strings.EqualFold(instance.Metadata.Runtime, runtime) {
				pluginNames = append(pluginNames, instance.GetName(ctx))
			}
		}
		sort.Strings(pluginNames)

		statuses = append(statuses, dto.RuntimeStatusDto{
			Runtime:           runtime,
			IsStarted:         runtimeHost.IsStarted(ctx),
			HostVersion:       getRuntimeHostVersion(ctx, runtime),
			LoadedPluginCount: len(pluginNames),
			LoadedPluginNames: pluginNames,
		})
	}

	sort.SliceStable(statuses, func(i, j int) bool {
		return statuses[i].Runtime < statuses[j].Runtime
	})

	writeSuccessResponse(w, statuses)
}

func getRuntimeHostVersion(ctx context.Context, runtime string) string {
	runtimeUpper := strings.ToUpper(runtime)
	switch runtimeUpper {
	case string(plugin.PLUGIN_RUNTIME_NODEJS):
		return getNodejsHostVersion(ctx)
	case string(plugin.PLUGIN_RUNTIME_PYTHON):
		return getPythonHostVersion(ctx)
	default:
		return ""
	}
}

func getNodejsHostVersion(ctx context.Context) string {
	nodePath := host.FindNodejsPath(ctx)
	versionOutput, err := shell.RunOutput(nodePath, "-v")
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get nodejs host version: %s", err))
		return ""
	}

	return strings.TrimSpace(string(versionOutput))
}

func getPythonHostVersion(ctx context.Context) string {
	pythonPath := host.FindPythonPath(ctx)
	versionOutput, err := shell.RunOutput(pythonPath, "--version")
	version := strings.TrimSpace(string(versionOutput))
	if err != nil || version == "" {
		versionOutput, err = shell.RunOutput(pythonPath, "-c", "import sys;print(sys.version.split()[0])")
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get python host version: %s", err))
			return ""
		}
		version = strings.TrimSpace(string(versionOutput))
	}

	return strings.TrimPrefix(version, "Python ")
}

func handleSettingPluginUpdate(w http.ResponseWriter, r *http.Request) {
	type keyValuePair struct {
		PluginId string
		Key      string
		Value    string
	}

	decoder := json.NewDecoder(r.Body)
	var kv keyValuePair
	err := decoder.Decode(&kv)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	pluginInstance, exist := lo.Find(plugin.GetPluginManager().GetPluginInstances(), func(item *plugin.Instance) bool {
		if item.Metadata.Id == kv.PluginId {
			return true
		}
		return false
	})
	if !exist {
		writeErrorResponse(w, "can't find plugin")
		return
	}

	if kv.Key == "Disabled" {
		pluginInstance.Setting.Disabled.Set(kv.Value == "true")
	} else if kv.Key == "TriggerKeywords" {
		pluginInstance.Setting.TriggerKeywords.Set(strings.Split(kv.Value, ","))
	} else {
		var isPlatformSpecific = false
		for _, settingDefinition := range pluginInstance.Metadata.SettingDefinitions {
			if settingDefinition.Value != nil && settingDefinition.Value.GetKey() == kv.Key {
				isPlatformSpecific = settingDefinition.IsPlatformSpecific
				break
			}
		}
		pluginInstance.API.SaveSetting(getTraceContext(r), kv.Key, kv.Value, isPlatformSpecific)
	}

	writeSuccessResponse(w, "")
}

func handleOpen(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	pathResult := gjson.GetBytes(body, "path")
	if !pathResult.Exists() {
		writeErrorResponse(w, "path is empty")
		return
	}

	shell.Open(pathResult.String())

	writeSuccessResponse(w, "")
}

func handleSaveWindowPosition(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	type positionData struct {
		X int `json:"x"`
		Y int `json:"y"`
	}

	var pos positionData
	err := json.NewDecoder(r.Body).Decode(&pos)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	logger.Info(ctx, fmt.Sprintf("Received window position save request: x=%d, y=%d", pos.X, pos.Y))

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	woxSetting.LastWindowX.Set(pos.X)
	woxSetting.LastWindowY.Set(pos.Y)

	logger.Info(ctx, fmt.Sprintf("Window position saved successfully: x=%d, y=%d", pos.X, pos.Y))
	writeSuccessResponse(w, "")
}

func handleBackupNow(w http.ResponseWriter, r *http.Request) {
	backupErr := setting.GetSettingManager().Backup(getTraceContext(r), setting.BackupTypeManual)
	if backupErr != nil {
		writeErrorResponse(w, backupErr.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	backupId := idResult.String()
	restoreErr := setting.GetSettingManager().Restore(getTraceContext(r), backupId)
	if restoreErr != nil {
		writeErrorResponse(w, restoreErr.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleBackupAll(w http.ResponseWriter, r *http.Request) {
	backups, err := setting.GetSettingManager().FindAllBackups(getTraceContext(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, backups)
}

func handleBackupFolder(w http.ResponseWriter, r *http.Request) {
	backupDir := util.GetLocation().GetBackupDirectory()

	// Ensure backup directory exists
	if err := util.GetLocation().EnsureDirectoryExist(backupDir); err != nil {
		writeErrorResponse(w, fmt.Sprintf("Failed to create backup directory: %s", err.Error()))
		return
	}

	writeSuccessResponse(w, backupDir)
}

func handleLogClear(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	err := util.GetLogger().ClearHistory()
	if err != nil {
		GetUIManager().GetUI(ctx).Notify(ctx, common.NotifyMsg{
			Icon:           common.WoxIcon.String(),
			Text:           fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "ui_data_log_clear_notify_failed"), err.Error()),
			DisplaySeconds: 6,
		})
		writeErrorResponse(w, err.Error())
		return
	}

	GetUIManager().GetUI(ctx).Notify(ctx, common.NotifyMsg{
		Icon:           common.WoxIcon.String(),
		Text:           i18n.GetI18nManager().TranslateWox(ctx, "ui_data_log_clear_notify_success"),
		DisplaySeconds: 4,
	})
	writeSuccessResponse(w, "")
}

func handleLogOpen(w http.ResponseWriter, r *http.Request) {
	logFile := filepath.Join(util.GetLocation().GetLogDirectory(), "log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	_ = file.Close()

	if err := shell.OpenFileInFolder(logFile); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleHotkeyAvailable(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	hotkeyResult := gjson.GetBytes(body, "hotkey")
	if !hotkeyResult.Exists() {
		writeErrorResponse(w, "hotkey is empty")
		return
	}

	isAvailable := hotkey.IsHotkeyAvailable(ctx, hotkeyResult.String())
	writeSuccessResponse(w, isAvailable)
}

func handleShow(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	GetUIManager().GetUI(ctx).ShowApp(ctx, common.ShowContext{SelectAll: true})
	writeSuccessResponse(w, "")
}

func handleOnUIReady(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	GetUIManager().PostUIReady(ctx)
	writeSuccessResponse(w, "")
}

func handleOnFocusLost(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.HideOnLostFocus.Get() {
		GetUIManager().GetUI(ctx).HideApp(ctx)
	}
	writeSuccessResponse(w, "")
}

func handleLangAvailable(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, i18n.GetSupportedLanguages())
}

func handleLangJson(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	langCodeResult := gjson.GetBytes(body, "langCode")
	if !langCodeResult.Exists() {
		writeErrorResponse(w, "langCode is empty")
		return
	}
	langCode := langCodeResult.String()

	if !i18n.IsSupportedLangCode(langCode) {
		logger.Error(ctx, fmt.Sprintf("unsupported lang code: %s", langCode))
		writeErrorResponse(w, fmt.Sprintf("unsupported lang code: %s", langCode))
		return
	}

	langJson, err := i18n.GetI18nManager().GetLangJson(ctx, i18n.LangCode(langCode))
	if err != nil {
		logger.Error(ctx, err.Error())
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, langJson)
}

func handleOnShow(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	GetUIManager().PostOnShow(ctx)
	writeSuccessResponse(w, "")
}

func handleOnQueryBoxFocus(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	GetUIManager().PostOnQueryBoxFocus(ctx)
	writeSuccessResponse(w, "")
}

func handleOnHide(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	GetUIManager().PostOnHide(ctx)
	writeSuccessResponse(w, "")
}

func handleOnSetting(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	body, _ := io.ReadAll(r.Body)
	inSettingViewResult := gjson.GetBytes(body, "inSettingView")
	if !inSettingViewResult.Exists() {
		writeErrorResponse(w, "inSettingView is required")
		return
	}

	GetUIManager().PostOnSetting(ctx, inSettingViewResult.Bool())
	writeSuccessResponse(w, "")
}

func handleQueryMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	type metadataResponse struct {
		Icon             common.WoxImage
		WidthRatio       float64
		IsGridLayout     bool
		GridLayoutParams plugin.MetadataFeatureParamsGridLayout
	}
	var metadata metadataResponse
	metadata.WidthRatio = 0.5 // default width ratio

	body, _ := io.ReadAll(r.Body)
	queryResult := gjson.GetBytes(body, "query")
	if !queryResult.Exists() {
		writeErrorResponse(w, "query is empty")
		return
	}

	var plainQuery common.PlainQuery
	unmarshalErr := json.Unmarshal([]byte(queryResult.String()), &plainQuery)
	if unmarshalErr != nil {
		logger.Error(ctx, unmarshalErr.Error())
		writeErrorResponse(w, unmarshalErr.Error())
		return
	}
	query, pluginInstance, err := plugin.GetPluginManager().NewQuery(ctx, plainQuery)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to new query: %s", err.Error()))
		writeSuccessResponse(w, metadataResponse{})
		return
	}

	if pluginInstance == nil {
		// this query is not for any plugin (now a global query)
		writeSuccessResponse(w, metadataResponse{})
		return
	}

	iconImg, parseErr := common.ParseWoxImage(pluginInstance.Metadata.Icon)
	if parseErr == nil {
		metadata.Icon = common.ConvertIcon(ctx, iconImg, pluginInstance.PluginDirectory)
	} else {
		logger.Error(ctx, fmt.Sprintf("failed to parse icon: %s", parseErr.Error()))
	}

	featureParams, err := pluginInstance.Metadata.GetFeatureParamsForResultPreviewWidthRatio()
	if err == nil {
		metadata.WidthRatio = featureParams.WidthRatio
	} else {
		if !errors.Is(err, plugin.ErrFeatureNotSupported) {
			logger.Error(ctx, fmt.Sprintf("failed to get feature params for result preview width ratio: %s", err.Error()))
		}
	}

	featureParamsGridLayout, err := pluginInstance.Metadata.GetFeatureParamsForGridLayout()
	if err == nil {
		// Check if current command is in the allowed commands list
		currentCommand := query.Command

		shouldEnableGrid := true
		if len(featureParamsGridLayout.Commands) > 0 {
			// Check if first element starts with "!" to determine mode
			if strings.HasPrefix(featureParamsGridLayout.Commands[0], "!") {
				// Exclusion mode: grid enabled for all commands except those starting with "!"
				shouldEnableGrid = true
				for _, cmd := range featureParamsGridLayout.Commands {
					if strings.TrimPrefix(cmd, "!") == currentCommand {
						shouldEnableGrid = false
						break
					}
				}
			} else {
				// Inclusion mode: grid enabled only for commands in the list
				shouldEnableGrid = false
				for _, cmd := range featureParamsGridLayout.Commands {
					if cmd == currentCommand {
						shouldEnableGrid = true
						break
					}
				}
			}
		}

		if shouldEnableGrid {
			metadata.IsGridLayout = true
			metadata.GridLayoutParams = featureParamsGridLayout
		}
	} else {
		if !errors.Is(err, plugin.ErrFeatureNotSupported) {
			logger.Error(ctx, fmt.Sprintf("failed to get feature params for grid layout: %s", err.Error()))
		}
	}

	writeSuccessResponse(w, metadata)
}

func handleDeeplink(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	deeplinkResult := gjson.GetBytes(body, "deeplink")
	if !deeplinkResult.Exists() {
		writeErrorResponse(w, "deeplink is empty")
		return
	}

	GetUIManager().ProcessDeeplink(ctx, deeplinkResult.String())

	writeSuccessResponse(w, "")
}

func handleAIProviders(w http.ResponseWriter, r *http.Request) {
	providers := ai.GetAllProviders()
	writeSuccessResponse(w, providers)
}

func handleAIModels(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	var results = []common.Model{}
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	for _, providerSetting := range woxSetting.AIProviders.Get() {
		provider, err := ai.NewProvider(ctx, providerSetting)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to new ai provider: %s", err.Error()))
			continue
		}

		models, modelsErr := provider.Models(ctx)
		if modelsErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to get models for provider %s: %s", providerSetting.Name, modelsErr.Error()))
			continue
		}

		results = append(results, models...)
	}

	logger.Info(ctx, fmt.Sprintf("found %d ai models", len(results)))

	writeSuccessResponse(w, results)
}

func handleAIPing(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	providerResult := gjson.GetBytes(body, "name")
	if !providerResult.Exists() {
		writeErrorResponse(w, "provider name is empty")
		return
	}
	apiKeyResult := gjson.GetBytes(body, "apiKey")
	if !apiKeyResult.Exists() {
		writeErrorResponse(w, "apiKey is empty")
		return
	}
	hostResult := gjson.GetBytes(body, "host")
	if !hostResult.Exists() {
		writeErrorResponse(w, "host is empty")
		return
	}

	provider, err := ai.NewProvider(ctx, setting.AIProvider{
		Name:   common.ProviderName(providerResult.String()),
		ApiKey: apiKeyResult.String(),
		Host:   hostResult.String(),
	})
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to new ai provider: %s", err.Error()))
		writeErrorResponse(w, err.Error())
		return
	}

	err = provider.Ping(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleAIChat(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	chatDataResult := gjson.GetBytes(body, "chatData")
	if !chatDataResult.Exists() {
		writeErrorResponse(w, "chatData is empty")
		return
	}

	// Parse chat data
	chatData := common.AIChatData{}
	err := json.Unmarshal([]byte(chatDataResult.String()), &chatData)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	chater.Chat(ctx, chatData, 0)

	writeSuccessResponse(w, "")
}

func handleAIMCPServerToolsAll(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	tools := chater.GetAllTools(ctx)
	results := lo.Map(tools, func(tool common.MCPTool, _ int) map[string]any {
		return map[string]any{
			"Name":        tool.Name,
			"Description": tool.Description,
			"Parameters":  tool.Parameters,
		}
	})

	writeSuccessResponse(w, results)
}

func handleAIAgents(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	agents := chater.GetAllAgents(ctx)
	writeSuccessResponse(w, agents)
}

func handleAIDefaultModel(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	defaultModel := chater.GetDefaultModel(ctx)
	writeSuccessResponse(w, defaultModel)
}

func handleAIMCPServerTools(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	mcpConfigResult := gjson.ParseBytes(body)
	if !mcpConfigResult.Exists() {
		writeErrorResponse(w, "mcpConfig is empty")
		return
	}

	mcpConfig := common.AIChatMCPServerConfig{}
	err := json.Unmarshal([]byte(mcpConfigResult.String()), &mcpConfig)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	tools, err := ai.MCPListTools(ctx, mcpConfig)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("Found %d tools for mcp server %s", len(tools), mcpConfig.Name))

	results := lo.Map(tools, func(tool common.MCPTool, _ int) map[string]any {
		return map[string]any{
			"Name":        tool.Name,
			"Description": tool.Description,
			"Parameters":  tool.Parameters,
		}
	})

	writeSuccessResponse(w, results)
}

func handleDoctorCheck(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	results := plugin.RunDoctorChecks(ctx)
	writeSuccessResponse(w, results)
}

func handleUserDataLocation(w http.ResponseWriter, r *http.Request) {
	location := util.GetLocation()
	writeSuccessResponse(w, location.GetUserDataDirectory())
}

func handleUserDataLocationUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	logger.Info(ctx, "Updating user data directory location")

	body, _ := io.ReadAll(r.Body)
	locationResult := gjson.GetBytes(body, "location")
	if !locationResult.Exists() {
		writeErrorResponse(w, "location is empty")
		return
	}

	newLocation := locationResult.String()
	if newLocation == "" {
		writeErrorResponse(w, "location cannot be empty")
		return
	}

	// Use the manager method to handle directory change
	err := GetUIManager().ChangeUserDataDirectory(ctx, newLocation)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to change user data directory: %s", err.Error()))
		writeErrorResponse(w, fmt.Sprintf("Failed to change user data directory: %s", err.Error()))
		return
	}

	logger.Info(ctx, fmt.Sprintf("User data directory successfully changed to: %s", newLocation))
	writeSuccessResponse(w, "User data directory updated successfully")
}

func handlePluginDetail(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	plugins := plugin.GetPluginManager().GetPluginInstances()
	foundPlugin, exist := lo.Find(plugins, func(item *plugin.Instance) bool {
		return item.Metadata.Id == idResult.String()
	})
	if !exist {
		writeErrorResponse(w, fmt.Sprintf("Plugin with ID %s not found", idResult.String()))
		return
	}

	pluginDto, err := convertPluginInstanceToDto(ctx, foundPlugin)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, pluginDto)

}

func handleToolbarSnooze(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	textResult := gjson.GetBytes(body, "text")
	if !textResult.Exists() {
		writeErrorResponse(w, "text is empty")
		return
	}
	durationResult := gjson.GetBytes(body, "duration")
	if !durationResult.Exists() {
		writeErrorResponse(w, "duration is empty")
		return
	}

	text := textResult.String()
	dur := durationResult.String()

	var untilMillis int64
	switch dur {
	case "3d":
		untilMillis = time.Now().Add(3 * 24 * time.Hour).UnixMilli()
	case "7d":
		untilMillis = time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	case "1m":
		untilMillis = time.Now().Add(30 * 24 * time.Hour).UnixMilli()
	case "forever":
		untilMillis = 0
	default:
		writeErrorResponse(w, "unknown duration")
		return
	}

	if err := database.SnoozeToolbarText(ctx, text, untilMillis); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, updater.CURRENT_VERSION)
}
