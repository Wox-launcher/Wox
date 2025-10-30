package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"wox/ai"
	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/ui/dto"
	"wox/updater"
	"wox/util"
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
	"/hotkey/available": handleHotkeyAvailable,
	"/query/icon":       handleQueryIcon,
	"/query/ratio":      handleQueryRatio,
	"/query/mru":        handleQueryMRU,
	"/deeplink":         handleDeeplink,
	"/version":          handleVersion,

	// toolbar snooze/mute
	"/toolbar/snooze": handleToolbarSnooze,
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, "Wox")
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, "pong")
}

func handlePreview(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErrorResponse(w, "id is empty")
		return
	}

	preview, err := plugin.GetPluginManager().GetResultPreview(util.NewTraceContext(), id)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, preview)
}

func handleTheme(w http.ResponseWriter, r *http.Request) {
	theme := GetUIManager().GetCurrentTheme(util.NewTraceContext())
	writeSuccessResponse(w, theme)
}

func handlePluginStore(w http.ResponseWriter, r *http.Request) {
	getCtx := util.NewTraceContext()
	manifests := plugin.GetStoreManager().GetStorePluginManifests(util.NewTraceContext())
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
		plugins[i].Icon = common.NewWoxImageUrl(manifests[i].IconUrl)
		plugins[i].IsInstalled = isInstalled
		plugins[i] = convertPluginDto(getCtx, plugins[i], pluginInstance)
	}

	writeSuccessResponse(w, plugins)
}

func handlePluginInstalled(w http.ResponseWriter, r *http.Request) {
	defer util.GoRecover(util.NewTraceContext(), "get installed plugins")

	getCtx := util.NewTraceContext()
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
	ctx := util.NewTraceContext()

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

	logger.Info(ctx, fmt.Sprintf("Installing plugin '%s' (%s)", findPlugin.Name, pluginId))
	installErr := plugin.GetStoreManager().Install(ctx, findPlugin)
	if installErr != nil {
		errMsg := fmt.Sprintf("Failed to install plugin '%s': %s", findPlugin.Name, installErr.Error())
		logger.Error(ctx, errMsg)
		writeErrorResponse(w, errMsg)
		return
	}

	logger.Info(ctx, fmt.Sprintf("Successfully installed plugin '%s' (%s)", findPlugin.Name, pluginId))
	writeSuccessResponse(w, "")
}

func handlePluginUninstall(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()

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

	uninstallErr := plugin.GetStoreManager().Uninstall(ctx, findPlugin)
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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()
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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()

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
	woxSetting := setting.GetSettingManager().GetWoxSetting(util.NewTraceContext())

	var settingDto dto.WoxSettingDto
	settingDto.EnableAutostart = woxSetting.EnableAutostart.Get()
	settingDto.MainHotkey = woxSetting.MainHotkey.Get()
	settingDto.SelectionHotkey = woxSetting.SelectionHotkey.Get()
	settingDto.UsePinYin = woxSetting.UsePinYin.Get()
	settingDto.SwitchInputMethodABC = woxSetting.SwitchInputMethodABC.Get()
	settingDto.HideOnStart = woxSetting.HideOnStart.Get()
	settingDto.HideOnLostFocus = woxSetting.HideOnLostFocus.Get()
	settingDto.ShowTray = woxSetting.ShowTray.Get()
	settingDto.LangCode = woxSetting.LangCode.Get()
	settingDto.QueryHotkeys = woxSetting.QueryHotkeys.Get()
	settingDto.QueryShortcuts = woxSetting.QueryShortcuts.Get()
	settingDto.QueryMode = woxSetting.QueryMode.Get()
	settingDto.AIProviders = woxSetting.AIProviders.Get()
	settingDto.HttpProxyEnabled = woxSetting.HttpProxyEnabled.Get()
	settingDto.HttpProxyUrl = woxSetting.HttpProxyUrl.Get()
	settingDto.ShowPosition = woxSetting.ShowPosition.Get()
	settingDto.EnableAutoBackup = woxSetting.EnableAutoBackup.Get()
	settingDto.EnableAutoUpdate = woxSetting.EnableAutoUpdate.Get()
	settingDto.CustomPythonPath = woxSetting.CustomPythonPath.Get()
	settingDto.CustomNodejsPath = woxSetting.CustomNodejsPath.Get()

	settingDto.AppWidth = woxSetting.AppWidth.Get()
	settingDto.MaxResultCount = woxSetting.MaxResultCount.Get()
	settingDto.ThemeId = woxSetting.ThemeId.Get()

	writeSuccessResponse(w, settingDto)
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

	ctx := util.NewTraceContext()
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)

	var vb bool
	var vf float64
	var vs = kv.Value
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
	case "QueryMode":
		woxSetting.QueryMode.Set(vs)
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
	default:
		writeErrorResponse(w, "unknown setting key: "+kv.Key)
		return
	}

	GetUIManager().PostSettingUpdate(util.NewTraceContext(), kv.Key, kv.Value)

	writeSuccessResponse(w, "")
}

func handleRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()
	instances := plugin.GetPluginManager().GetPluginInstances()

	statuses := make([]dto.RuntimeStatusDto, 0, len(plugin.AllHosts))
	for _, host := range plugin.AllHosts {
		runtime := string(host.GetRuntime(ctx))

		if strings.EqualFold(runtime, string(plugin.PLUGIN_RUNTIME_SCRIPT)) {
			continue
		}

		var pluginNames []string
		for _, instance := range instances {
			if strings.EqualFold(instance.Metadata.Runtime, runtime) {
				pluginNames = append(pluginNames, instance.Metadata.Name)
			}
		}
		sort.Strings(pluginNames)

		statuses = append(statuses, dto.RuntimeStatusDto{
			Runtime:           runtime,
			IsStarted:         host.IsStarted(ctx),
			LoadedPluginCount: len(pluginNames),
			LoadedPluginNames: pluginNames,
		})
	}

	sort.SliceStable(statuses, func(i, j int) bool {
		return statuses[i].Runtime < statuses[j].Runtime
	})

	writeSuccessResponse(w, statuses)
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
		pluginInstance.API.SaveSetting(util.NewTraceContext(), kv.Key, kv.Value, isPlatformSpecific)
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
	ctx := util.NewTraceContext()

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
	backupErr := setting.GetSettingManager().Backup(util.NewTraceContext(), setting.BackupTypeManual)
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
	restoreErr := setting.GetSettingManager().Restore(util.NewTraceContext(), backupId)
	if restoreErr != nil {
		writeErrorResponse(w, restoreErr.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleBackupAll(w http.ResponseWriter, r *http.Request) {
	backups, err := setting.GetSettingManager().FindAllBackups(util.NewTraceContext())
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

func handleHotkeyAvailable(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()
	GetUIManager().GetUI(ctx).ShowApp(ctx, common.ShowContext{SelectAll: true})
	writeSuccessResponse(w, "")
}

func handleOnUIReady(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()
	GetUIManager().PostUIReady(ctx)
	writeSuccessResponse(w, "")
}

func handleOnFocusLost(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()
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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()
	GetUIManager().PostOnShow(ctx)
	writeSuccessResponse(w, "")
}

func handleOnQueryBoxFocus(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()
	GetUIManager().PostOnQueryBoxFocus(ctx)
	writeSuccessResponse(w, "")
}

func handleOnHide(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()

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

	GetUIManager().PostOnHide(ctx, plainQuery)
	writeSuccessResponse(w, "")
}

func handleQueryIcon(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()

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

	_, pluginInstance, err := plugin.GetPluginManager().NewQuery(ctx, plainQuery)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to new query: %s", err.Error()))
		writeSuccessResponse(w, common.WoxImage{})
		return
	}

	if pluginInstance == nil {
		// this query is not for any plugin (now a global query)
		writeSuccessResponse(w, common.WoxImage{})
		return
	}

	iconImg, parseErr := common.ParseWoxImage(pluginInstance.Metadata.Icon)
	if parseErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to parse icon: %s", parseErr.Error()))
		writeSuccessResponse(w, common.WoxImage{})
		return
	}

	iconImage := common.ConvertIcon(ctx, iconImg, pluginInstance.PluginDirectory)
	writeSuccessResponse(w, iconImage)
}

func handleQueryRatio(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()

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

	defaultRatio := 0.5

	_, pluginInstance, err := plugin.GetPluginManager().NewQuery(ctx, plainQuery)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to new query: %s", err.Error()))
		writeSuccessResponse(w, defaultRatio)
		return
	}

	if pluginInstance == nil {
		// this query is not for any plugin (now a global query)
		writeSuccessResponse(w, defaultRatio)
		return
	}

	metadata := pluginInstance.Metadata
	featureParams, err := metadata.GetFeatureParamsForResultPreviewWidthRatio()
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to get feature params for result preview width ratio: %s", err.Error()))
		writeSuccessResponse(w, defaultRatio)
		return
	}

	writeSuccessResponse(w, featureParams.WidthRatio)
}

func handleDeeplink(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	agents := chater.GetAllAgents(ctx)
	writeSuccessResponse(w, agents)
}

func handleAIDefaultModel(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	defaultModel := chater.GetDefaultModel(ctx)
	writeSuccessResponse(w, defaultModel)
}

func handleAIMCPServerTools(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()
	results := plugin.RunDoctorChecks(ctx)
	writeSuccessResponse(w, results)
}

func handleUserDataLocation(w http.ResponseWriter, r *http.Request) {
	location := util.GetLocation()
	writeSuccessResponse(w, location.GetUserDataDirectory())
}

func handleUserDataLocationUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()
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
	ctx := util.NewTraceContext()

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
	ctx := util.NewTraceContext()

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

func handleQueryMRU(w http.ResponseWriter, r *http.Request) {
	body, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		writeErrorResponse(w, fmt.Sprintf("failed to read request body: %s", readErr.Error()))
		return
	}

	var ctx context.Context
	traceIdResult := gjson.GetBytes(body, "traceId")
	if traceIdResult.Exists() && traceIdResult.String() != "" {
		ctx = util.NewTraceContextWith(traceIdResult.String())
	} else {
		ctx = util.NewTraceContext()
	}

	mruResults := plugin.GetPluginManager().QueryMRU(ctx)
	logger.Info(ctx, fmt.Sprintf("found %d MRU results", len(mruResults)))
	writeSuccessResponse(w, mruResults)
}
