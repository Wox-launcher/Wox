package ui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"wox/account"
	"wox/ai"
	"wox/cloudsync"
	"wox/common"
	"wox/diagnostic"
	"wox/i18n"
	"wox/plugin"
	pluginhost "wox/plugin/host"
	appplugin "wox/plugin/system/app"
	"wox/setting"
	"wox/telemetry"
	"wox/ui/dto"
	"wox/updater"
	"wox/util"
	"wox/util/font"
	"wox/util/keyboard"
	"wox/util/overlay"
	"wox/util/permission"
	"wox/util/processmemory"
	"wox/util/screen"
	utilselection "wox/util/selection"
	"wox/util/shell"
	"wox/util/tray"
	utilwindow "wox/util/window"

	"github.com/google/uuid"
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
	"/theme/save":      handleThemeSave,

	// settings
	"/setting/wox":                      handleSettingWox,
	"/setting/wox/update":               handleSettingWoxUpdate,
	"/setting/hotkey/apps":              handleHotkeyAppCandidates,
	"/setting/window-manager/displays":  handleWindowManagerDisplays,
	"/browser/extension/status":         handleBrowserExtensionStatus,
	"/setting/ui/fonts":                 handleSettingUIFontList,
	"/setting/plugin/update":            handleSettingPluginUpdate,
	"/setting/userdata/location":        handleUserDataLocation,
	"/setting/userdata/location/update": handleUserDataLocationUpdate,
	"/setting/position":                 handleSaveWindowPosition,
	"/runtime/status":                   handleRuntimeStatus,
	"/runtime/restart":                  handleRuntimeRestart,
	"/account/status":                   handleAccountStatus,
	"/account/refresh":                  handleAccountRefresh,
	"/account/register":                 handleAccountRegister,
	"/account/verify_email":             handleAccountVerifyEmail,
	"/account/login":                    handleAccountLogin,
	"/account/logout":                   handleAccountLogout,
	"/account/resend_verification":      handleAccountResendVerification,
	"/account/change_password":          handleAccountChangePassword,
	"/account/password_reset/request":   handleAccountPasswordResetRequest,
	"/account/password_reset/confirm":   handleAccountPasswordResetConfirm,
	"/account/billing/plan":             handleAccountBillingPlan,
	"/account/billing/checkout":         handleAccountBillingCheckout,
	"/account/billing/portal":           handleAccountBillingPortal,
	"/sync/status":                      handleSyncStatus,
	"/sync/bootstrap/status":            handleSyncBootstrapStatus,
	"/sync/bootstrap/start":             handleSyncBootstrapStart,
	"/sync/enable":                      handleSyncEnable,
	"/sync/disable":                     handleSyncDisable,
	"/sync/push":                        handleSyncPush,
	"/sync/pull":                        handleSyncPull,
	"/sync/key/init":                    handleSyncKeyInit,
	"/sync/key/fetch":                   handleSyncKeyFetch,
	"/sync/key/recovery_code":           handleSyncRecoveryCode,
	"/sync/key/reset/prepare":           handleSyncKeyResetPrepare,
	"/sync/key/reset":                   handleSyncKeyReset,
	"/sync/devices/list":                handleSyncDevicesList,
	"/sync/devices/revoke":              handleSyncDeviceRevoke,
	"/sync/devices/join":                handleSyncDeviceJoin,

	// events
	"/on/focus/lost":       handleOnFocusLost,
	"/on/ready":            handleOnUIReady,
	"/on/show":             handleOnShow,
	"/on/querybox/focus":   handleOnQueryBoxFocus,
	"/on/hide":             handleOnHide,
	"/on/setting":          handleOnSetting,
	"/on/hotkey/recording": handleOnHotkeyRecording,
	"/on/onboarding":       handleOnOnboarding,
	"/usage/stats":         handleUsageStats,

	// lang
	"/lang/available": handleLangAvailable,
	"/lang/json":      handleLangJson,

	// ai
	"/ai/providers":      handleAIProviders,
	"/ai/commands/store": handleAICommandStore,
	"/ai/models":         handleAIModels,
	"/ai/model/default":  handleAIDefaultModel,
	"/ai/ping":           handleAIPing,
	"/ai/chat":           handleAIChat,
	"/ai/mcp/tools":      handleAIMCPServerTools,
	"/ai/mcp/tools/all":  handleAIMCPServerToolsAll,
	"/ai/agents":         handleAIAgents,

	// doctor
	"/doctor/check":                  handleDoctorCheck,
	"/doctor/ignore":                 handleDoctorIgnore,
	"/doctor/unignore":               handleDoctorUnignore,
	"/permission/accessibility/open": handlePermissionAccessibilityOpen,
	"/permission/privacy/open":       handlePermissionPrivacyOpen,

	// others
	"/":                                   handleHome,
	"/show":                               handleShow,
	"/tooltip/show":                       handleTooltipOverlayShow,
	"/tooltip/hide":                       handleTooltipOverlayHide,
	"/ping":                               handlePing,
	"/preview":                            handlePreview,
	"/preview/image/overlay":              handlePreviewImageOverlay,
	"/preview/file/media":                 handlePreviewFileMedia,
	"/image/file/icon":                    handleFileIcon,
	"/image/lazy/load":                    handleLazyImageLoad,
	"/open":                               handleOpen,
	"/backup/now":                         handleBackupNow,
	"/backup/restore":                     handleBackupRestore,
	"/backup/all":                         handleBackupAll,
	"/backup/folder":                      handleBackupFolder,
	"/log/clear":                          handleLogClear,
	"/log/open":                           handleLogOpen,
	"/diagnostics/status":                 handleDiagnosticsStatus,
	"/diagnostics/monitor/enable":         handleDiagnosticsMonitorEnable,
	"/diagnostics/monitor/enable-restart": handleDiagnosticsMonitorEnableRestart,
	"/diagnostics/monitor/disable":        handleDiagnosticsMonitorDisable,
	"/diagnostics/export":                 handleDiagnosticsExport,
	"/hotkey/available":                   handleHotkeyAvailable,
	"/hotkey/availability":                handleHotkeyAvailability,
	"/glance":                             handleGlance,
	"/glance/action":                      handleGlanceAction,
	"/updater/channel/versions":           handleUpdateChannelVersions,
	"/deeplink":                           handleDeeplink,
	"/version":                            handleVersion,

	// test-only triggers
	"/test/plugin/install_local":     handleTestInstallLocalPlugin,
	"/test/trigger/open_setting":     handleTestTriggerOpenSetting,
	"/test/trigger/open_onboarding":  handleTestTriggerOpenOnboarding,
	"/test/trigger/query_hotkey":     handleTestTriggerQueryHotkey,
	"/test/trigger/screenshot":       handleTestTriggerScreenshot,
	"/test/trigger/selection_hotkey": handleTestTriggerSelectionHotkey,
	"/test/screen/mouse":             handleTestMouseScreen,
	"/test/trigger/tray_query":       handleTestTriggerTrayQuery,
}

var updateChannelVersionsProvider = updater.GetUpdateChannelVersions

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

type fileIconRequest struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

func handleFileIcon(w http.ResponseWriter, r *http.Request) {
	// File previews run in Flutter, but icon extraction already belongs to core's
	// platform-specific fileicon pipeline. Keep this endpoint small so previews
	// reuse the same cached icon artifacts as launcher results.
	ctx := getTraceContext(r)
	filePath := strings.TrimSpace(r.URL.Query().Get("path"))
	size := 0
	if rawSize := strings.TrimSpace(r.URL.Query().Get("size")); rawSize != "" {
		if parsedSize, err := strconv.Atoi(rawSize); err == nil {
			size = parsedSize
		}
	}

	if r.Body != nil {
		var request fileIconRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err == nil {
			if filePath == "" {
				filePath = strings.TrimSpace(request.Path)
			}
			if size <= 0 {
				size = request.Size
			}
		}
	}

	if filePath == "" {
		writeErrorResponse(w, "path is empty")
		return
	}
	if size <= 0 {
		size = common.ResultListIconSize
	}
	if size > common.ResultGridIconSize {
		size = common.ResultGridIconSize
	}

	icon := common.ConvertIconWithSize(ctx, common.NewWoxImageFileIcon(filePath), "", size)
	if icon.IsEmpty() || icon.ImageType == common.WoxImageTypeFileIcon {
		writeErrorResponse(w, "failed to resolve file icon")
		return
	}

	writeSuccessResponse(w, icon)
}

func handlePreviewFileMedia(w http.ResponseWriter, r *http.Request) {
	// Media previews need ordinary HTTP range requests so large video files can
	// stream into WebView without loading the whole file into Flutter memory.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	encodedPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if encodedPath == "" {
		http.Error(w, "path is empty", http.StatusBadRequest)
		return
	}

	decodedPath, err := base64.URLEncoding.DecodeString(encodedPath)
	if err != nil {
		http.Error(w, "path is invalid", http.StatusBadRequest)
		return
	}

	filePath := string(decodedPath)
	if filePath == "" {
		http.Error(w, "path is empty", http.StatusBadRequest)
		return
	}
	if !filepath.IsAbs(filePath) {
		http.Error(w, "path must be absolute", http.StatusBadRequest)
		return
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to stat file", http.StatusInternalServerError)
		return
	}
	if stat.IsDir() {
		http.Error(w, "path is a directory", http.StatusBadRequest)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if contentType := resolvePreviewFileMediaContentType(filePath); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, filepath.Base(filePath), stat.ModTime(), file)
}

func resolvePreviewFileMediaContentType(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".pdf":
		return "application/pdf"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".ogg", ".opus":
		return "audio/ogg"
	}

	return mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath)))
}

func handleLazyImageLoad(w http.ResponseWriter, r *http.Request) {
	// Result icon lazy loading is intentionally an internal UI/core endpoint.
	// Plugins still return ordinary WoxImage values, while Flutter exchanges the
	// manager-issued token for a resized cache image only after the widget exists.
	ctx := getTraceContext(r)
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" && r.Body != nil {
		var request struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err == nil {
			token = strings.TrimSpace(request.Token)
		}
	}
	if token == "" {
		writeErrorResponse(w, "token is empty")
		return
	}

	icon, err := plugin.GetPluginManager().LoadLazyResultIcon(ctx, token)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, icon)
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

type previewImageOverlayRequest struct {
	Image common.WoxImage
}

func handlePreviewImageOverlay(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, fmt.Sprintf("failed to read preview image overlay request: %s", err.Error()))
		return
	}

	var request previewImageOverlayRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writeErrorResponse(w, fmt.Sprintf("failed to parse preview image overlay request: %s", err.Error()))
		return
	}
	if request.Image.IsEmpty() {
		writeErrorResponse(w, "preview image is empty")
		return
	}

	// Refactor: image preview routing now calls the single shared overlay entry directly. The
	// overlay utility decides whether URL sources need loading/cache behavior, while non-URL sources
	// are displayed immediately through the same API.
	if err := overlay.ShowImageOverlay(ctx, overlay.ImageOverlayOptions{
		Title:         "Wox image preview",
		Image:         request.Image,
		FitToScreen:   true,
		Topmost:       true,
		Movable:       true,
		CloseOnEscape: true,
		Anchor:        overlay.AnchorCenter,
	}); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
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
		plugins[i].IsUpgradable = false
		if isInstalled {
			plugins[i].IsUpgradable = plugin.IsVersionUpgradable(pluginInstance.Metadata.Version, manifests[i].Version)
		}
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
	installedPlugin.Glances = translatePluginGlances(ctx, pluginInstance)

	//load screenshot urls from store if exist
	storePlugin, foundErr := plugin.GetStoreManager().GetStorePluginManifestById(ctx, pluginInstance.Metadata.Id)
	if foundErr == nil {
		installedPlugin.ScreenshotUrls = storePlugin.ScreenshotUrls
		installedPlugin.IsUpgradable = plugin.IsVersionUpgradable(pluginInstance.Metadata.Version, storePlugin.Version)
	} else {
		installedPlugin.ScreenshotUrls = []string{}
		installedPlugin.IsUpgradable = false
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

func translatePluginGlances(ctx context.Context, pluginInstance *plugin.Instance) []plugin.MetadataGlance {
	glances := make([]plugin.MetadataGlance, 0, len(pluginInstance.Metadata.Glances))
	for _, glance := range pluginInstance.Metadata.Glances {
		// Glance definitions are metadata used by settings. Translating them here
		// keeps Flutter dropdowns simple while preserving i18n keys in plugin.json.
		glance.Name = common.I18nString(pluginInstance.TranslateMetadataText(ctx, glance.Name))
		glance.Description = common.I18nString(pluginInstance.TranslateMetadataText(ctx, glance.Description))
		glances = append(glances, glance)
	}
	return glances
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
	effectiveStoreThemes := make([]common.Theme, 0, len(storeThemes))
	for _, storeTheme := range storeThemes {
		// New feature: store themes stay raw for install/persistence, but preview
		// responses should match the current OS so users see the style that will be
		// applied on this machine.
		effectiveStoreThemes = append(effectiveStoreThemes, GetUIManager().resolvePlatformTheme(ctx, storeTheme))
	}

	var themes = make([]dto.ThemeDto, len(effectiveStoreThemes))
	copyErr := copier.Copy(&themes, &effectiveStoreThemes)
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

type saveThemeRequest struct {
	Name      string       `json:"Name"`
	Theme     common.Theme `json:"Theme"`
	Overwrite bool         `json:"Overwrite"`
}

// handleThemeSave persists an edited draft as either a new user theme or an overwrite of the current user theme.
func handleThemeSave(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, "failed to read theme save request: "+err.Error())
		return
	}

	var request saveThemeRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writeErrorResponse(w, "failed to parse theme save request: "+err.Error())
		return
	}

	themeName := strings.TrimSpace(request.Name)
	if themeName == "" {
		writeErrorResponse(w, "theme name is empty")
		return
	}

	theme := request.Theme
	if theme.AppBackgroundColor == "" {
		writeErrorResponse(w, "theme data is empty")
		return
	}

	if request.Overwrite {
		if strings.TrimSpace(theme.ThemeId) == "" {
			writeErrorResponse(w, "theme id is empty")
			return
		}
		if GetUIManager().IsSystemTheme(theme.ThemeId) {
			writeErrorResponse(w, "can't overwrite system theme")
			return
		}
	} else {
		theme.ThemeId = uuid.NewString()
	}
	theme.ThemeName = themeName
	if strings.TrimSpace(theme.ThemeAuthor) == "" {
		theme.ThemeAuthor = "Wox Launcher"
	}
	if strings.TrimSpace(theme.ThemeUrl) == "" {
		theme.ThemeUrl = "https://github.com/Wox-launcher/Wox"
	}
	if strings.TrimSpace(theme.Version) == "" {
		theme.Version = "1.0.0"
	}
	theme.IsSystem = false
	theme.IsInstalled = true
	theme.IsAutoAppearance = false
	theme.DarkThemeId = ""
	theme.LightThemeId = ""
	theme.Windows = nil
	theme.MacOS = nil
	theme.Linux = nil

	if installErr := GetStoreManager().Install(ctx, theme); installErr != nil {
		writeErrorResponse(w, "can't save theme: "+installErr.Error())
		return
	}

	writeSuccessResponse(w, theme)
}

func handleSettingWox(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)

	var settingDto dto.WoxSettingDto
	settingDto.EnableAutostart = woxSetting.EnableAutostart.Get()
	settingDto.MainHotkey = woxSetting.MainHotkey.Get()
	settingDto.SelectionHotkey = woxSetting.SelectionHotkey.Get()
	settingDto.IgnoredHotkeyApps = woxSetting.IgnoredHotkeyApps.Get()
	settingDto.LogLevel = util.NormalizeLogLevel(woxSetting.LogLevel.Get())
	settingDto.UsePinYin = woxSetting.UsePinYin.Get()
	settingDto.SwitchInputMethodABC = woxSetting.SwitchInputMethodABC.Get()
	settingDto.HideOnStart = woxSetting.HideOnStart.Get()
	settingDto.OnboardingFinished = woxSetting.OnboardingFinished.Get()
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
	settingDto.IsLinuxWaylandSession = util.IsLinuxWaylandSession()
	settingDto.IsEvdevReadAvailable = keyboard.IsEvdevReadAvailable()
	settingDto.EnableAutoBackup = woxSetting.EnableAutoBackup.Get()
	settingDto.EnableAutoUpdate = woxSetting.EnableAutoUpdate.Get()
	settingDto.ReleaseChannel = woxSetting.ReleaseChannel.Get()
	settingDto.EnableAnonymousUsageStats = woxSetting.EnableAnonymousUsageStats.Get()
	settingDto.CustomPythonPath = woxSetting.CustomPythonPath.Get()
	settingDto.CustomNodejsPath = woxSetting.CustomNodejsPath.Get()
	settingDto.CloudSyncServerUrl = woxSetting.CloudSyncServerUrl.Get()
	settingDto.CloudSyncDisabledPlugins = woxSetting.CloudSyncDisabledPlugins.Get()

	settingDto.AppWidth = woxSetting.AppWidth.Get()
	settingDto.MaxResultCount = woxSetting.MaxResultCount.Get()
	settingDto.UiDensity = woxSetting.UiDensity.Get()
	settingDto.ThemeId = woxSetting.ThemeId.Get()
	settingDto.AppFontFamily = woxSetting.AppFontFamily.Get()
	settingDto.EnableQueryCompletionHint = woxSetting.EnableQueryCompletionHint.Get()
	settingDto.EnableGlance = woxSetting.EnableGlance.Get()
	settingDto.PrimaryGlance = woxSetting.PrimaryGlance.Get()
	settingDto.HideGlanceIcon = woxSetting.HideGlanceIcon.Get()
	settingDto.ShowScoreTail = woxSetting.ShowScoreTail.Get()
	settingDto.ShowPerformanceTail = woxSetting.ShowPerformanceTail.Get()
	settingDto.ShowPerformanceTailBatch = woxSetting.ShowPerformanceTailBatch.Get()
	settingDto.ShowPerformanceTailPluginQuery = woxSetting.ShowPerformanceTailPluginQuery.Get()
	settingDto.ShowPerformanceTailBackendPrepared = woxSetting.ShowPerformanceTailBackendPrepared.Get()
	settingDto.ShowPerformanceTailUiReceived = woxSetting.ShowPerformanceTailUiReceived.Get()

	writeSuccessResponse(w, settingDto)
}

func handleHotkeyAppCandidates(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, appplugin.GetHotkeyAppCandidates(getTraceContext(r)))
}

func handleWindowManagerDisplays(w http.ResponseWriter, r *http.Request) {
	displays, err := utilwindow.ListDisplays()
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, displays)
}

func handleBrowserExtensionStatus(w http.ResponseWriter, r *http.Request) {
	const browserPluginID = "8f68a760-86a0-46a9-b331-58dcaf091daa"
	sp := plugin.GetPluginManager().GetSystemPlugin(browserPluginID)
	type extensionStatus struct {
		Connected bool `json:"connected"`
	}
	connected := false
	if sp != nil {
		type connector interface {
			IsExtensionConnected() bool
		}
		if c, ok := sp.(connector); ok {
			connected = c.IsExtensionConnected()
		}
	}
	writeSuccessResponse(w, extensionStatus{Connected: connected})
}

func handleUpdateChannelVersions(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, updateChannelVersionsProvider(getTraceContext(r)))
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
	if kv.Key == "ReleaseChannel" {
		updatedValue, updateErr := updateWoxSettingValue(ctx, woxSetting, kv.Key, kv.Value)
		if updateErr != nil {
			writeErrorResponse(w, updateErr.Error())
			return
		}

		GetUIManager().PostSettingUpdate(ctx, kv.Key, updatedValue)
		writeSuccessResponse(w, "")
		return
	}

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

	// Hotkeys are registered before persisting settings so a denied or failed
	// system bind does not leave stored settings ahead of the actual OS
	// registration. These branches return early, so the normal PostSettingUpdate
	// path does not register the same change again.
	if kv.Key == "MainHotkey" {
		if vs != woxSetting.MainHotkey.Get() {
			if err := GetUIManager().RegisterMainHotkey(ctx, vs); err != nil {
				writeErrorResponse(w, err.Error())
				return
			}
		}
		woxSetting.MainHotkey.Set(vs)
		writeSuccessResponse(w, "")
		return
	}

	if kv.Key == "SelectionHotkey" {
		if vs != woxSetting.SelectionHotkey.Get() {
			if err := GetUIManager().RegisterSelectionHotkey(ctx, vs); err != nil {
				writeErrorResponse(w, err.Error())
				return
			}
		}
		woxSetting.SelectionHotkey.Set(vs)
		writeSuccessResponse(w, "")
		return
	}

	if kv.Key == "QueryHotkeys" {
		queryHotkeys, parseErr := parseQueryHotkeysSettingValue(vs)
		if parseErr != nil {
			writeErrorResponse(w, parseErr.Error())
			return
		}

		uiManager := GetUIManager()
		var registerErr error
		if shouldGroupWaylandPortalHotkeys() {
			uiManager.globalHotkeyMu.Lock()
			registerErr = uiManager.reregisterWaylandPortalGlobalHotkeys(ctx, woxSetting.MainHotkey.Get(), woxSetting.SelectionHotkey.Get(), queryHotkeys)
			uiManager.globalHotkeyMu.Unlock()
		} else {
			registerErr = uiManager.reregisterIndividualQueryHotkeys(ctx, queryHotkeys)
		}
		if registerErr != nil {
			writeErrorResponse(w, registerErr.Error())
			return
		}

		woxSetting.QueryHotkeys.Set(queryHotkeys)
		writeSuccessResponse(w, "")
		return
	}

	switch kv.Key {
	case "EnableAutostart":
		woxSetting.EnableAutostart.Set(vb)
	case "IgnoredHotkeyApps":
		var ignoredApps []setting.IgnoredHotkeyApp
		if err := json.Unmarshal([]byte(vs), &ignoredApps); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
		woxSetting.IgnoredHotkeyApps.Set(normalizeIgnoredHotkeyApps(ignoredApps))
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
	case "OnboardingFinished":
		// The guide writes completion through the existing settings endpoint so
		// skip and finish share one durable state transition with no extra API.
		woxSetting.OnboardingFinished.Set(vb)
	case "HideOnLostFocus":
		woxSetting.HideOnLostFocus.Set(vb)
	case "ShowTray":
		woxSetting.ShowTray.Set(vb)
	case "LangCode":
		woxSetting.LangCode.Set(i18n.LangCode(vs))
	case "QueryShortcuts":
		var queryShortcuts []setting.QueryShortcut
		if err := json.Unmarshal([]byte(vs), &queryShortcuts); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
		woxSetting.QueryShortcuts.Set(queryShortcuts)
	case "CloudSyncServerUrl":
		cloudSyncServerURL := strings.TrimSpace(vs)
		woxSetting.CloudSyncServerUrl.Set(cloudSyncServerURL)
		if err := applyCloudSyncServerURL(ctx, cloudSyncServerURL); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
	case "CloudSyncDisabledPlugins":
		var disabledPlugins []string
		if err := json.Unmarshal([]byte(vs), &disabledPlugins); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
		woxSetting.CloudSyncDisabledPlugins.Set(disabledPlugins)
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

			if rawHideQueryBox, ok := rawTrayQuery["HideQueryBox"]; ok {
				trayQuery.HideQueryBox = parseBool(rawHideQueryBox)
			}

			if rawHideToolbar, ok := rawTrayQuery["HideToolbar"]; ok {
				trayQuery.HideToolbar = parseBool(rawHideToolbar)
			}

			if rawDisabled, ok := rawTrayQuery["Disabled"]; ok {
				trayQuery.Disabled = parseBool(rawDisabled)
			}

			if rawWidth, ok := rawTrayQuery["Width"]; ok {
				trayQuery.Width = maxInt(parseInt(rawWidth), 0)
			}

			if rawMaxResultCount, ok := rawTrayQuery["MaxResultCount"]; ok {
				trayQuery.MaxResultCount = normalizeOptionalMaxResultCount(parseInt(rawMaxResultCount))
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
		if strings.TrimSpace(vs) != "" {
			// Bug fix: reject unsupported custom Python paths at save time. The
			// old flow persisted any path and only failed later when the host
			// tried to start, so this backend guard keeps API callers and the UI
			// on the same minimum-version contract.
			if _, validateErr := pluginhost.ValidatePythonExecutable(ctx, vs); validateErr != nil {
				writeErrorResponse(w, validateErr.Error())
				return
			}
		}
		woxSetting.CustomPythonPath.Set(vs)
	case "CustomNodejsPath":
		if strings.TrimSpace(vs) != "" {
			// Feature: Node.js custom paths use the same save-time validation as
			// Python. Checking the version here prevents non-UI API callers from
			// persisting a Node.js executable that the host will immediately reject.
			if _, validateErr := pluginhost.ValidateNodejsExecutable(ctx, vs); validateErr != nil {
				writeErrorResponse(w, validateErr.Error())
				return
			}
		}
		woxSetting.CustomNodejsPath.Set(vs)

	case "HttpProxyEnabled":
		woxSetting.HttpProxyEnabled.Set(vb)
	case "HttpProxyUrl":
		woxSetting.HttpProxyUrl.Set(vs)

	case "AppWidth":
		woxSetting.AppWidth.Set(int(vf))
	case "MaxResultCount":
		woxSetting.MaxResultCount.Set(int(vf))
	case "UiDensity":
		// New launcher presentation setting: store only the normalized density
		// enum. The old fixed-size behavior maps to normal, while unsupported
		// values fall back here before they can desync Go height estimates from
		// Flutter's rendered metrics.
		normalizedDensity := setting.NormalizeUiDensity(vs)
		updatedValue = string(normalizedDensity)
		if err := woxSetting.UiDensity.Set(normalizedDensity); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
	case "ThemeId":
		woxSetting.ThemeId.Set(vs)
	case "AppFontFamily":
		vs = font.NormalizeConfiguredFontFamily(vs, font.GetSystemFontFamilies(ctx))
		woxSetting.AppFontFamily.Set(vs)
	case "EnableQueryCompletionHint":
		woxSetting.EnableQueryCompletionHint.Set(vb)
	case "EnableGlance":
		woxSetting.EnableGlance.Set(vb)
	case "PrimaryGlance":
		var glance setting.GlanceRef
		if err := json.Unmarshal([]byte(vs), &glance); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
		woxSetting.PrimaryGlance.Set(glance)
	case "HideGlanceIcon":
		// This setting only changes the launcher presentation. Persisting it in
		// the shared settings API keeps the behavior consistent after reloads
		// without asking Glance providers to omit useful icon metadata.
		woxSetting.HideGlanceIcon.Set(vb)
	case "ShowScoreTail":
		// New dev setting: score tails used to be compiled into a helper but
		// effectively disabled by commented call sites. Persisting this switch
		// lets developers opt in without editing code for each debug session.
		woxSetting.ShowScoreTail.Set(vb)
	case "ShowPerformanceTail":
		// New dev setting: performance tags were previously always appended in
		// dev builds. Keeping the check in the backend prevents hidden UI tabs
		// from being the only guard for noisy query-result tags.
		woxSetting.ShowPerformanceTail.Set(vb)
	case "ShowPerformanceTailBatch":
		woxSetting.ShowPerformanceTailBatch.Set(vb)
	case "ShowPerformanceTailPluginQuery":
		woxSetting.ShowPerformanceTailPluginQuery.Set(vb)
	case "ShowPerformanceTailBackendPrepared":
		woxSetting.ShowPerformanceTailBackendPrepared.Set(vb)
	case "ShowPerformanceTailUiReceived":
		woxSetting.ShowPerformanceTailUiReceived.Set(vb)
	case "EnableAnonymousUsageStats":
		woxSetting.EnableAnonymousUsageStats.Set(vb)
		// When disabled, delete telemetry state to stop tracking
		if !vb {
			telemetry.DeleteTelemetryState(ctx)
		}
	default:
		writeErrorResponse(w, "unknown setting key: "+kv.Key)
		return
	}

	GetUIManager().PostSettingUpdate(getTraceContext(r), kv.Key, updatedValue)

	writeSuccessResponse(w, "")
}

// parseQueryHotkeysSettingValue normalizes query hotkey payloads before both
// pre-registration and persistence so portal errors do not leave two views
// of the same setting.
func parseQueryHotkeysSettingValue(value string) ([]setting.QueryHotkey, error) {
	var rawQueryHotkeys []map[string]any
	if err := json.Unmarshal([]byte(value), &rawQueryHotkeys); err != nil {
		return nil, err
	}

	var queryHotkeys []setting.QueryHotkey
	for _, rawQueryHotkey := range rawQueryHotkeys {
		queryHotkey := setting.QueryHotkey{
			Position: setting.QueryHotkeyPositionSystemDefault,
		}

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

// updateWoxSettingValue handles small shared setting writes that need normalization.
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

func handleGlance(w http.ResponseWriter, r *http.Request) {
	type glanceRequest struct {
		Glances []setting.GlanceRef
		Reason  plugin.GlanceRefreshReason
	}

	var request glanceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	keys := make([]plugin.GlanceKey, 0, len(request.Glances))
	for _, glance := range request.Glances {
		if glance.IsEmpty() {
			continue
		}
		keys = append(keys, plugin.GlanceKey{PluginId: glance.PluginId, GlanceId: glance.GlanceId})
	}

	// Glance data is requested by the UI only for user-selected slots. Keeping
	// this pull path in HTTP avoids giving plugins a persistent UI push channel.
	items := plugin.GetPluginManager().GetGlanceItems(getTraceContext(r), keys, request.Reason)
	writeSuccessResponse(w, items)
}

func handleGlanceAction(w http.ResponseWriter, r *http.Request) {
	type glanceActionRequest struct {
		PluginId string
		GlanceId string
		ActionId string
	}

	var request glanceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if request.PluginId == "" || request.GlanceId == "" || request.ActionId == "" {
		writeErrorResponse(w, "pluginId, glanceId and actionId are required")
		return
	}

	if err := plugin.GetPluginManager().ExecuteGlanceAction(getTraceContext(r), request.PluginId, request.GlanceId, request.ActionId); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleAccountStatus(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := account.GetService()
	if service == nil {
		writeSuccessResponse(w, account.Status{})
		return
	}
	writeSuccessResponse(w, service.Status(ctx))
}

// Refreshes account data from the server before returning the latest local status.
func handleAccountRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	if err := service.RefreshAccount(ctx); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, service.Status(ctx))
}

func handleAccountRegister(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Lang     string `json:"lang"`
	}
	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	result, err := service.Register(getTraceContext(r), payload.Email, payload.Password, accountRequestLang(payload.Lang))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, result)
}

func handleAccountVerifyEmail(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email string `json:"email"`
		Code  string `json:"code"`
		Lang  string `json:"lang"`
	}
	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	result, err := service.VerifyEmail(getTraceContext(r), payload.Email, payload.Code, accountRequestLang(payload.Lang))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, result)
}

func handleAccountLogin(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Lang     string `json:"lang"`
	}
	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	result, err := service.Login(getTraceContext(r), payload.Email, payload.Password, accountRequestLang(payload.Lang))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, result)
}

func handleAccountLogout(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := account.GetService()
	if service == nil {
		writeSuccessResponse(w, "")
		return
	}
	if err := service.Logout(ctx); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if cloudService := cloudsync.GetService(); cloudService != nil {
		if err := cloudService.ResetLocalState(ctx); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to reset cloud sync state during logout: %v", err))
		}
	}
	writeSuccessResponse(w, "")
}

func handleAccountResendVerification(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email string `json:"email"`
		Lang  string `json:"lang"`
	}
	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	if err := service.ResendVerification(getTraceContext(r), payload.Email, accountRequestLang(payload.Lang)); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleAccountPasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email string `json:"email"`
		Lang  string `json:"lang"`
	}
	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	if err := service.RequestPasswordReset(getTraceContext(r), payload.Email, accountRequestLang(payload.Lang)); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleAccountPasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Token    string `json:"token"`
		Password string `json:"password"`
		Lang     string `json:"lang"`
	}
	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	if err := service.ConfirmPasswordReset(getTraceContext(r), payload.Token, payload.Password, accountRequestLang(payload.Lang)); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleAccountChangePassword(w http.ResponseWriter, r *http.Request) {
	type request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		Lang            string `json:"lang"`
	}
	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	if err := service.ChangePassword(getTraceContext(r), payload.CurrentPassword, payload.NewPassword, accountRequestLang(payload.Lang)); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleAccountBillingCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	session, err := service.CreateCheckoutSession(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, session)
}

func handleAccountBillingPlan(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	plan, err := service.GetBillingPlan(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, plan)
}

func handleAccountBillingPortal(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := account.GetService()
	if service == nil {
		writeErrorResponse(w, "account service is not configured")
		return
	}
	session, err := service.CreatePortalSession(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, session)
}

// accountRequestLang maps Wox locale codes to the language set supported by the sync account API.
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

func handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	accountService := account.GetService()
	if accountService == nil || !accountService.Status(ctx).LoggedIn {
		writeSuccessResponse(w, cloudsync.ServiceStatus{Enabled: false})
		return
	}
	service := cloudsync.GetService()
	if service == nil {
		writeSuccessResponse(w, cloudsync.ServiceStatus{Enabled: false})
		return
	}

	writeSuccessResponse(w, service.Status(ctx))
}

type syncBootstrapStatusResponse struct {
	HasRemoteData bool `json:"has_remote_data"`
	HasRemoteKey  bool `json:"has_remote_key"`
}

func handleSyncBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	status, err := resolveSyncBootstrapStatus(getTraceContext(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, status)
}

func handleSyncBootstrapStart(w http.ResponseWriter, r *http.Request) {
	type request struct {
		RecoveryCode string `json:"recovery_code"`
	}

	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if strings.TrimSpace(payload.RecoveryCode) == "" {
		writeErrorResponse(w, "recovery_code is empty")
		return
	}

	if err := startSyncBootstrap(getTraceContext(r), payload.RecoveryCode); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func resolveSyncBootstrapStatus(ctx context.Context) (syncBootstrapStatusResponse, error) {
	if err := ensureSyncBootstrapAllowed(ctx); err != nil {
		return syncBootstrapStatusResponse{}, err
	}
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil || service.KeyManager == nil {
		return syncBootstrapStatusResponse{}, fmt.Errorf("cloud sync is not configured")
	}

	hasRemoteData, err := service.Manager.HasRemoteSnapshotData(ctx)
	if err != nil {
		return syncBootstrapStatusResponse{}, err
	}
	remoteKeyStatus, err := service.KeyManager.RemoteStatus(ctx)
	if err != nil {
		return syncBootstrapStatusResponse{}, err
	}
	return syncBootstrapStatusResponse{HasRemoteData: hasRemoteData, HasRemoteKey: remoteKeyStatus.Available}, nil
}

func startSyncBootstrap(ctx context.Context, recoveryCode string) error {
	status, err := resolveSyncBootstrapStatus(ctx)
	if err != nil {
		return err
	}
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil || service.KeyManager == nil {
		return fmt.Errorf("cloud sync is not configured")
	}

	if status.HasRemoteKey {
		if _, err := service.KeyManager.FetchWithRecoveryCode(ctx, recoveryCode); err != nil {
			return err
		}
	} else {
		if status.HasRemoteData {
			return fmt.Errorf("cloud sync key is missing")
		}
		if _, err := service.KeyManager.InitWithRecoveryCode(ctx, recoveryCode, ""); err != nil {
			return err
		}
	}
	cloudsync.MarkCloudSyncBootstrapPending(ctx)

	accountService := account.GetService()
	if accountService != nil {
		if err := accountService.SetSyncEnabled(ctx, true); err != nil {
			return err
		}
	}

	if status.HasRemoteData {
		scheduleCloudSyncBootstrapRestore(ctx, service)
		return nil
	}
	startCloudSyncManagerIfSyncEnabled(ctx, service)
	scheduleCloudSyncBootstrapInitialPush(ctx, service)
	return nil
}

// scheduleCloudSyncBootstrapRestore restores remote data before starting the regular sync manager.
func scheduleCloudSyncBootstrapRestore(ctx context.Context, service *cloudsync.Service) {
	util.Go(ctx, "cloud sync bootstrap restore", func() {
		if service == nil || service.Manager == nil {
			return
		}

		if err := service.Manager.RestoreSnapshot(ctx); err != nil {
			cloudsync.RecordCloudSyncBootstrapFailure(ctx, err)
			util.GetLogger().Error(ctx, fmt.Sprintf("cloud sync bootstrap restore failed: %v", err))
			return
		}
		startCloudSyncManagerIfSyncEnabled(ctx, service)
	})
}

// scheduleCloudSyncBootstrapInitialPush performs the first local-to-cloud push after the dialog can close.
func scheduleCloudSyncBootstrapInitialPush(ctx context.Context, service *cloudsync.Service) {
	util.Go(ctx, "cloud sync bootstrap initial push", func() {
		if service == nil || service.Manager == nil {
			return
		}

		service.Manager.PushLocalSnapshot(ctx, "bootstrap")
		state, err := cloudsync.LoadCloudSyncState(ctx)
		if err != nil {
			cloudsync.RecordCloudSyncBootstrapFailure(ctx, err)
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to load cloud sync bootstrap state: %v", err))
			return
		}
		if state.LastError != "" {
			return
		}
		cloudsync.MarkCloudSyncBootstrapComplete(ctx)
	})
}

func ensureSyncBootstrapAllowed(ctx context.Context) error {
	accountService := account.GetService()
	accountStatus := account.Status{}
	if accountService != nil {
		accountStatus = accountService.Status(ctx)
	}
	if accountService == nil || !accountStatus.LoggedIn {
		return fmt.Errorf("account is not logged in")
	}
	if !accountStatus.SyncEligible {
		return fmt.Errorf("subscription_required")
	}
	return nil
}

func handleSyncEnable(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	accountService := account.GetService()
	accountStatus := account.Status{}
	if accountService != nil {
		accountStatus = accountService.Status(ctx)
	}
	if accountService == nil || !accountStatus.LoggedIn {
		writeErrorResponse(w, "account is not logged in")
		return
	}
	if !accountStatus.SyncEligible {
		writeErrorResponse(w, "subscription_required")
		return
	}
	if err := accountService.SetSyncEnabled(ctx, true); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if service := cloudsync.GetService(); service != nil && service.KeyManager != nil && service.KeyManager.GetStatus(ctx).Available {
		startCloudSyncManagerIfSyncEnabled(ctx, service)
	}
	writeSuccessResponse(w, "")
}

func handleSyncDisable(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	if service := cloudsync.GetService(); service != nil && service.Manager != nil {
		service.Manager.Stop(ctx)
	}
	if accountService := account.GetService(); accountService != nil {
		if err := accountService.SetSyncEnabled(ctx, false); err != nil {
			writeErrorResponse(w, err.Error())
			return
		}
	}
	writeSuccessResponse(w, "")
}

func handleSyncDeviceJoin(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	accountService := account.GetService()
	accountStatus := account.Status{}
	if accountService != nil {
		accountStatus = accountService.Status(ctx)
	}
	if accountService == nil || !accountStatus.LoggedIn {
		writeErrorResponse(w, "account is not logged in")
		return
	}
	if !accountStatus.SyncEligible {
		writeErrorResponse(w, "subscription_required")
		return
	}

	service := cloudsync.GetService()
	if service == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}
	if err := service.JoinCurrentDevice(ctx); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	startCloudSyncManagerIfSyncEnabled(ctx, service)
	writeSuccessResponse(w, "")
}

func handleSyncPush(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}

	startCloudSyncManagerIfSyncEnabled(ctx, service)
	service.Manager.PushPending(ctx, "manual")
	writeSuccessResponse(w, "")
}

func handleSyncPull(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}

	startCloudSyncManagerIfSyncEnabled(ctx, service)
	service.Manager.Pull(ctx, "manual")
	writeSuccessResponse(w, "")
}

func handleSyncKeyInit(w http.ResponseWriter, r *http.Request) {
	type request struct {
		RecoveryCode string `json:"recovery_code"`
		DeviceName   string `json:"device_name"`
	}

	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if payload.RecoveryCode == "" {
		writeErrorResponse(w, "recovery_code is empty")
		return
	}

	ctx := getTraceContext(r)
	service := cloudsync.GetService()
	if service == nil || service.KeyManager == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}

	resp, err := service.KeyManager.InitWithRecoveryCode(ctx, payload.RecoveryCode, payload.DeviceName)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if accountService := account.GetService(); accountService != nil {
		_ = accountService.SetSyncEnabled(ctx, true)
	}
	startCloudSyncManagerIfSyncEnabled(ctx, service)

	writeSuccessResponse(w, resp)
}

func handleSyncKeyFetch(w http.ResponseWriter, r *http.Request) {
	type request struct {
		RecoveryCode string `json:"recovery_code"`
	}

	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if payload.RecoveryCode == "" {
		writeErrorResponse(w, "recovery_code is empty")
		return
	}

	ctx := getTraceContext(r)
	service := cloudsync.GetService()
	if service == nil || service.KeyManager == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}

	resp, err := service.KeyManager.FetchWithRecoveryCode(ctx, payload.RecoveryCode)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if accountService := account.GetService(); accountService != nil {
		_ = accountService.SetSyncEnabled(ctx, true)
	}
	startCloudSyncManagerIfSyncEnabled(ctx, service)

	writeSuccessResponse(w, resp)
}

func handleSyncRecoveryCode(w http.ResponseWriter, r *http.Request) {
	code, err := cloudsync.GenerateRecoveryCode()
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, code)
}

func handleSyncKeyResetPrepare(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := cloudsync.GetService()
	if service == nil || service.KeyManager == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}

	resp, err := service.KeyManager.PrepareReset(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, resp)
}

func handleSyncKeyReset(w http.ResponseWriter, r *http.Request) {
	type request struct {
		ResetToken string `json:"reset_token"`
		Confirm    bool   `json:"confirm"`
	}

	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if payload.ResetToken == "" {
		writeErrorResponse(w, "reset_token is empty")
		return
	}
	if !payload.Confirm {
		writeErrorResponse(w, "confirm is required")
		return
	}

	ctx := getTraceContext(r)
	service := cloudsync.GetService()
	if service == nil || service.KeyManager == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}

	resp, err := service.KeyManager.Reset(ctx, payload.ResetToken)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, resp)
}

func handleSyncDevicesList(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	service := cloudsync.GetService()
	if service == nil || service.DeviceProvider == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}
	deviceID, err := service.DeviceProvider.DeviceID(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	deviceClient := service.DeviceClient
	if deviceClient == nil {
		deviceClient = service.Client
	}
	if deviceClient == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}
	if err := service.UpdateCurrentDevice(ctx); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to update current cloud sync device before listing devices: %v", err))
	}
	resp, err := deviceClient.ListDevices(ctx, cloudsync.CloudSyncDeviceListRequest{DeviceID: deviceID})
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, resp)
}

func handleSyncDeviceRevoke(w http.ResponseWriter, r *http.Request) {
	type request struct {
		TargetDeviceID string `json:"target_device_id"`
	}

	var payload request
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	ctx := getTraceContext(r)
	service := cloudsync.GetService()
	if service == nil || service.DeviceProvider == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}
	deviceID, err := service.DeviceProvider.DeviceID(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	deviceClient := service.DeviceClient
	if deviceClient == nil {
		deviceClient = service.Client
	}
	if deviceClient == nil {
		writeErrorResponse(w, "cloud sync is not configured")
		return
	}
	resp, err := deviceClient.RevokeDevice(ctx, cloudsync.CloudSyncDeviceRevokeRequest{DeviceID: deviceID, TargetDeviceID: payload.TargetDeviceID})
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, resp)
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
		runtimeStatus := runtimeHost.RuntimeStatus(ctx)

		statuses = append(statuses, dto.RuntimeStatusDto{
			Runtime:           runtime,
			IsStarted:         runtimeHost.IsStarted(ctx),
			HostVersion:       getRuntimeHostVersion(ctx, runtime, runtimeStatus.ExecutablePath),
			StatusCode:        string(runtimeStatus.StatusCode),
			StatusMessage:     localizeRuntimeStatusMessage(ctx, runtime, runtimeStatus),
			ExecutablePath:    runtimeStatus.ExecutablePath,
			LastStartError:    runtimeStatus.LastStartError,
			CanRestart:        runtimeStatus.CanRestart,
			InstallUrl:        runtimeStatus.InstallUrl,
			LoadedPluginCount: len(pluginNames),
			LoadedPluginNames: pluginNames,
		})
	}

	sort.SliceStable(statuses, func(i, j int) bool {
		return statuses[i].Runtime < statuses[j].Runtime
	})

	writeSuccessResponse(w, statuses)
}

func localizeRuntimeStatusMessage(ctx context.Context, runtime string, status plugin.RuntimeHostStatus) string {
	runtimeName := runtime
	switch strings.ToUpper(runtime) {
	case string(plugin.PLUGIN_RUNTIME_NODEJS):
		runtimeName = "Node.js"
	case string(plugin.PLUGIN_RUNTIME_PYTHON):
		runtimeName = "Python"
	}

	// Feature: /runtime/status returns localized user-facing status text, while
	// LastStartError keeps raw technical details only for true host startup
	// failures. This prevents English executable resolver messages from leaking
	// into localized settings UI.
	switch status.StatusCode {
	case plugin.RuntimeHostStatusRunning:
		return i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_running")
	case plugin.RuntimeHostStatusExecutableMissing:
		return strings.ReplaceAll(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_executable_missing_detail"), "{runtime}", runtimeName)
	case plugin.RuntimeHostStatusUnsupportedVersion:
		return strings.ReplaceAll(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_unsupported_version_detail"), "{runtime}", runtimeName)
	case plugin.RuntimeHostStatusStartFailed:
		return i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_start_failed_detail")
	case plugin.RuntimeHostStatusStopped:
		return i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_stopped")
	default:
		return status.StatusMessage
	}
}

func getRuntimeHostVersion(ctx context.Context, runtime string, executablePath string) string {
	if executablePath == "" {
		return ""
	}

	runtimeUpper := strings.ToUpper(runtime)
	switch runtimeUpper {
	case string(plugin.PLUGIN_RUNTIME_NODEJS):
		return getNodejsHostVersion(ctx, executablePath)
	case string(plugin.PLUGIN_RUNTIME_PYTHON):
		return getPythonHostVersion(ctx, executablePath)
	default:
		return ""
	}
}

func getNodejsHostVersion(ctx context.Context, nodePath string) string {
	versionOutput, err := shell.RunOutput(nodePath, "-v")
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get nodejs host version: %s", err))
		return ""
	}

	return strings.TrimSpace(string(versionOutput))
}

func getPythonHostVersion(ctx context.Context, pythonPath string) string {
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

func handleRuntimeRestart(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	runtimeResult := gjson.GetBytes(body, "Runtime")
	if !runtimeResult.Exists() {
		writeErrorResponse(w, "Runtime is required")
		return
	}

	runtime := plugin.ConvertToRuntime(runtimeResult.String())
	if runtime != plugin.PLUGIN_RUNTIME_NODEJS && runtime != plugin.PLUGIN_RUNTIME_PYTHON {
		writeErrorResponse(w, fmt.Sprintf("runtime %s does not support restart from settings", runtime))
		return
	}

	// Feature: expose a small restart endpoint so users can recover after fixing
	// Node.js/Python paths without restarting Wox. Reusing the plugin manager
	// keeps loaded plugin restoration in one place.
	if err := plugin.GetPluginManager().RestartHostForRuntime(ctx, runtime, nil, nil); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
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
	ctx := getTraceContext(r)

	backups, err := setting.GetSettingManager().FindAllBackups(ctx)
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
	logFile := util.GetLogger().CurrentLogPath()
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

func handleDiagnosticsStatus(w http.ResponseWriter, r *http.Request) {
	state := diagnostic.GetManager().LoadState()
	writeSuccessResponse(w, map[string]any{
		"enabled":        state.Enabled,
		"lastCleanExit":  state.LastCleanExit,
		"lastExportPath": state.LastExportPath,
	})
}

func handleDiagnosticsMonitorEnable(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	state, err := enableDiagnosticsMonitor(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, state)
}

func handleDiagnosticsMonitorEnableRestart(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	state, err := enableDiagnosticsMonitor(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if err := diagnostic.GetManager().StartSupervisorDetached(ctx, true); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, state)
	util.Go(ctx, "restart wox for bug aware monitor", func() {
		time.Sleep(200 * time.Millisecond)
		GetUIManager().ExitApp(util.NewTraceContext())
	})
}

// enableDiagnosticsMonitor keeps all HTTP entry points aligned with the system plugin's enable behavior.
func enableDiagnosticsMonitor(ctx context.Context) (diagnostic.State, error) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	previousLogLevel := util.NormalizeLogLevel(woxSetting.LogLevel.Get())
	state, err := diagnostic.GetManager().Enable(ctx, previousLogLevel)
	if err != nil {
		return diagnostic.State{}, err
	}
	// New feature: API-based enabling mirrors the system plugin path so any
	// future settings surface gets the same clean-log DEBUG session behavior.
	woxSetting.LogLevel.Set(setting.LogLevelDebug)
	util.GetLogger().SetLevel(setting.LogLevelDebug)
	GetUIManager().GetUI(ctx).UpdateDiagnosticStatus(ctx, true)
	return state, nil
}

func handleDiagnosticsMonitorDisable(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	state, err := diagnostic.GetManager().Disable(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if state.PreviousLogLevel != "" {
		setting.GetSettingManager().GetWoxSetting(ctx).LogLevel.Set(state.PreviousLogLevel)
		util.GetLogger().SetLevel(state.PreviousLogLevel)
	}
	GetUIManager().GetUI(ctx).UpdateDiagnosticStatus(ctx, false)
	writeSuccessResponse(w, state)
}

func handleDiagnosticsExport(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	exportPath, err := diagnostic.GetManager().Export(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, exportPath)
}

func handleHotkeyAvailable(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	hotkeyResult := gjson.GetBytes(body, "hotkey")
	if !hotkeyResult.Exists() {
		writeErrorResponse(w, "hotkey is empty")
		return
	}

	isAvailable := GetUIManager().IsHotkeyAvailable(ctx, hotkeyResult.String())
	writeSuccessResponse(w, isAvailable)
}

func handleHotkeyAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	hotkeyResult := gjson.GetBytes(body, "hotkey")
	if !hotkeyResult.Exists() {
		writeErrorResponse(w, "hotkey is empty")
		return
	}

	availability := GetUIManager().CheckHotkeyAvailability(ctx, hotkeyResult.String())
	writeSuccessResponse(w, availability)
}

func handleShow(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	GetUIManager().GetUI(ctx).ShowApp(ctx, common.ShowContext{
		SelectAll: true,
	})
	writeSuccessResponse(w, "")
}

func ensureTestTriggerEnabled(w http.ResponseWriter) bool {
	if util.IsDev() || util.IsTestMode() {
		return true
	}

	writeErrorResponse(w, "test trigger endpoints are only available in dev/test mode")
	return false
}

func handleTestTriggerQueryHotkey(w http.ResponseWriter, r *http.Request) {
	if !ensureTestTriggerEnabled(w) {
		return
	}

	type request struct {
		Query             string
		IsSilentExecution bool
		HideQueryBox      bool
		HideToolbar       bool
		Width             int
		MaxResultCount    int
		Position          string
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeErrorResponse(w, "query is empty")
		return
	}

	ctx := getTraceContext(r)
	err := GetUIManager().triggerQueryHotkey(ctx, setting.QueryHotkey{
		Query:             req.Query,
		IsSilentExecution: req.IsSilentExecution,
		HideQueryBox:      req.HideQueryBox,
		HideToolbar:       req.HideToolbar,
		Width:             req.Width,
		MaxResultCount:    normalizeOptionalMaxResultCount(req.MaxResultCount),
		Position:          normalizeQueryHotkeyPosition(req.Position),
	})
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleTestInstallLocalPlugin(w http.ResponseWriter, r *http.Request) {
	if !ensureTestTriggerEnabled(w) {
		return
	}

	type request struct {
		FilePath string
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	filePath := filepath.Clean(strings.TrimSpace(req.FilePath))
	if filePath == "" {
		writeErrorResponse(w, "filePath is empty")
		return
	}
	if _, err := os.Stat(filePath); err != nil {
		writeErrorResponse(w, fmt.Sprintf("plugin package does not exist: %s", filePath))
		return
	}

	ctx := getTraceContext(r)
	if err := plugin.GetStoreManager().InstallFromLocal(ctx, filePath); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleTestTriggerOpenSetting(w http.ResponseWriter, r *http.Request) {
	if !ensureTestTriggerEnabled(w) {
		return
	}

	type request struct {
		Path   string
		Param  string
		Source string
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErrorResponse(w, err.Error())
		return
	}

	ctx := getTraceContext(r)
	GetUIManager().GetUI(ctx).OpenSettingWindow(ctx, common.SettingWindowContext{
		Path:   strings.TrimSpace(req.Path),
		Param:  strings.TrimSpace(req.Param),
		Source: common.SettingWindowSource(strings.TrimSpace(req.Source)),
	})
	writeSuccessResponse(w, "")
}

func handleTestTriggerOpenOnboarding(w http.ResponseWriter, r *http.Request) {
	if !ensureTestTriggerEnabled(w) {
		return
	}

	ctx := getTraceContext(r)
	GetUIManager().GetUI(ctx).OpenOnboardingWindow(ctx)
	writeSuccessResponse(w, "")
}

func handleTestTriggerSelectionHotkey(w http.ResponseWriter, r *http.Request) {
	if !ensureTestTriggerEnabled(w) {
		return
	}

	type request struct {
		Type      string
		Text      string
		FilePaths []string
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	selected := utilselection.Selection{
		Type:      utilselection.SelectionType(req.Type),
		Text:      req.Text,
		FilePaths: req.FilePaths,
	}
	switch selected.Type {
	case utilselection.SelectionTypeText, utilselection.SelectionTypeFile:
	default:
		writeErrorResponse(w, "selection type is invalid")
		return
	}
	if selected.IsEmpty() {
		writeErrorResponse(w, "selection is empty")
		return
	}

	ctx := getTraceContext(r)
	uiManager := GetUIManager()
	uiManager.RefreshActiveWindowSnapshot(ctx)
	uiManager.GetUI(ctx).ChangeQuery(ctx, common.PlainQuery{
		QueryType:      plugin.QueryTypeSelection,
		QuerySelection: selected,
	})
	time.Sleep(150 * time.Millisecond)
	uiManager.GetUI(ctx).ShowApp(ctx, common.ShowContext{
		ShowSource: common.ShowSourceSelection,
	})

	writeSuccessResponse(w, "")
}

func handleTestTriggerScreenshot(w http.ResponseWriter, r *http.Request) {
	if !ensureTestTriggerEnabled(w) {
		return
	}

	ctx := getTraceContext(r)
	// The screenshot smoke path needs a backend-triggered session so integration tests can verify
	// the same Go -> WebSocket -> Flutter round-trip used by the real system plugin action.
	result, err := GetUIManager().GetUI(ctx).CaptureScreenshot(ctx, common.DefaultCaptureScreenshotRequest())
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, result)
}

func handleTestMouseScreen(w http.ResponseWriter, r *http.Request) {
	if !ensureTestTriggerEnabled(w) {
		return
	}

	writeSuccessResponse(w, screen.GetMouseScreen())
}

func handleTestTriggerTrayQuery(w http.ResponseWriter, r *http.Request) {
	if !ensureTestTriggerEnabled(w) {
		return
	}

	type rectRequest struct {
		X      int
		Y      int
		Width  int
		Height int
	}

	type request struct {
		Query          string
		Width          int
		HideQueryBox   bool
		HideToolbar    bool
		Disabled       bool
		MaxResultCount int
		Rect           rectRequest
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeErrorResponse(w, "query is empty")
		return
	}

	clickRect := tray.ClickRect{
		X:      req.Rect.X,
		Y:      req.Rect.Y,
		Width:  req.Rect.Width,
		Height: req.Rect.Height,
	}
	if clickRect.Width <= 0 {
		clickRect.Width = 40
	}
	if clickRect.Height <= 0 {
		clickRect.Height = 40
	}

	ctx := getTraceContext(r)
	GetUIManager().executeTrayQuery(ctx, setting.TrayQuery{
		Query:          req.Query,
		Width:          req.Width,
		HideQueryBox:   req.HideQueryBox,
		HideToolbar:    req.HideToolbar,
		MaxResultCount: normalizeOptionalMaxResultCount(req.MaxResultCount),
		Disabled:       req.Disabled,
	}, clickRect)

	writeSuccessResponse(w, "")
}

func handleOnUIReady(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	type uiReadyRequest struct {
		Pid int
	}
	var request uiReadyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil && err != io.EOF {
		writeErrorResponse(w, err.Error())
		return
	}
	if request.Pid > 0 {
		// Dev mode usually starts Flutter outside the core process tree, so the
		// ready callback is the reliable boundary where core can learn the UI PID.
		processmemory.SetWoxUIProcessPid(request.Pid)
	}
	GetUIManager().PostUIReady(ctx)
	startCloudSyncManagerAfterUIReady(ctx)
	writeSuccessResponse(w, "")
}

// startCloudSyncManagerAfterUIReady starts the scheduler only after Flutter can
// acknowledge websocket requests if a scheduled pull applies settings.
func startCloudSyncManagerAfterUIReady(ctx context.Context) {
	startCloudSyncManagerIfSyncEnabled(ctx, cloudsync.GetService())
}

// startCloudSyncManagerIfSyncEnabled starts the scheduler once sync is configured; scheduled work checks plan rules before it runs.
func startCloudSyncManagerIfSyncEnabled(ctx context.Context, service *cloudsync.Service) {
	if service == nil || service.Manager == nil {
		return
	}
	accountService := account.GetService()
	if accountService == nil {
		return
	}
	accountStatus := accountService.Status(ctx)
	if !accountStatus.LoggedIn || !accountStatus.SyncEligible || !accountStatus.SyncEnabled {
		return
	}
	if service.KeyManager == nil || !service.KeyManager.GetStatus(ctx).Available {
		return
	}
	service.StartManager(ctx)
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

func handleOnHotkeyRecording(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	body, _ := io.ReadAll(r.Body)
	isRecordingResult := gjson.GetBytes(body, "isRecording")
	if !isRecordingResult.Exists() {
		writeErrorResponse(w, "isRecording is required")
		return
	}

	logger.Info(ctx, fmt.Sprintf("received hotkey recording state from UI: isRecording=%t", isRecordingResult.Bool()))
	GetUIManager().PostOnHotkeyRecording(ctx, isRecordingResult.Bool())
	writeSuccessResponse(w, "")
}

func handleOnOnboarding(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	body, _ := io.ReadAll(r.Body)
	inOnboardingViewResult := gjson.GetBytes(body, "inOnboardingView")
	if !inOnboardingViewResult.Exists() {
		writeErrorResponse(w, "inOnboardingView is required")
		return
	}

	GetUIManager().PostOnOnboarding(ctx, inOnboardingViewResult.Bool())
	writeSuccessResponse(w, "")
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

func handleAICommandStore(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	writeSuccessResponse(w, ai.GetStoreManager().GetCommands(ctx))
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

func handleDoctorIgnore(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	var req struct {
		CheckType string `json:"checkType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	current := woxSetting.IgnoredDoctorChecks.Get()
	for _, t := range current {
		if t == req.CheckType {
			writeSuccessResponse(w, nil)
			return
		}
	}
	_ = woxSetting.IgnoredDoctorChecks.Set(append(current, req.CheckType))
	writeSuccessResponse(w, nil)
}

func handleDoctorUnignore(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	var req struct {
		CheckType string `json:"checkType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	current := woxSetting.IgnoredDoctorChecks.Get()
	filtered := current[:0]
	for _, t := range current {
		if t != req.CheckType {
			filtered = append(filtered, t)
		}
	}
	_ = woxSetting.IgnoredDoctorChecks.Set(filtered)
	writeSuccessResponse(w, nil)
}

func handlePermissionAccessibilityOpen(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	// The onboarding permission page should be non-blocking: opening System
	// Settings is a best-effort side effect and the guide remains skippable if
	// the platform has no corresponding permission panel.
	permission.GrantAccessibilityPermission(ctx)
	writeSuccessResponse(w, "")
}

func handlePermissionPrivacyOpen(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	// Full Disk Access cannot be detected reliably here, so onboarding only
	// opens the privacy page and explains the File Search impact in UI text.
	permission.OpenPrivacySecuritySettings(ctx)
	writeSuccessResponse(w, "")
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
	switch setting.QueryHotkeyPosition(strings.TrimSpace(value)) {
	case setting.QueryHotkeyPositionTopLeft,
		setting.QueryHotkeyPositionTopCenter,
		setting.QueryHotkeyPositionTopRight,
		setting.QueryHotkeyPositionCenter,
		setting.QueryHotkeyPositionBottomLeft,
		setting.QueryHotkeyPositionBottomCenter,
		setting.QueryHotkeyPositionBottomRight:
		return setting.QueryHotkeyPosition(strings.TrimSpace(value))
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

func handleVersion(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, updater.CURRENT_VERSION)
}
