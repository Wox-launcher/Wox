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
	"strconv"
	"strings"
	"time"

	"wox/account"
	"wox/ai"
	_ "wox/ai/builtintool"
	aitool "wox/ai/builtintool/wox"
	"wox/cloudsync"
	"wox/common"
	"wox/diagnostic"
	"wox/i18n"
	"wox/plugin"
	appplugin "wox/plugin/system/app"
	"wox/setting"
	"wox/ui/contract"
	"wox/ui/dto"
	"wox/updater"
	"wox/util"
	"wox/util/font"
	"wox/util/keyboard"
	"wox/util/permission"
	utilwindow "wox/util/window"

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
	"/on/querybox/focus":             handleOnQueryBoxFocus,
	"/on/hotkey/recording":           handleOnHotkeyRecording,
	"/on/hotkey/recording/candidate": handleOnHotkeyRecordingCandidate,
	"/on/onboarding":                 handleOnOnboarding,
	"/usage/stats":                   handleUsageStats,

	// lang
	"/lang/available": handleLangAvailable,
	"/lang/json":      handleLangJson,

	// ai
	"/ai/providers":       handleAIProviders,
	"/ai/commands/store":  handleAICommandStore,
	"/ai/models":          handleAIModels,
	"/ai/model/default":   handleAIDefaultModel,
	"/ai/ping":            handleAIPing,
	"/ai/chat":            handleAIChat,
	"/ai/chat/get":        handleAIChatGet,
	"/ai/chat/stop":       handleAIChatStop,
	"/ai/chat/delete":     handleAIChatDelete,
	"/ai/chat/summarize":  handleAIChatSummarize,
	"/ai/mcp/tools":       handleAIMCPServerTools,
	"/ai/mcp/tools/all":   handleAIMCPServerToolsAll,
	"/ai/skills":          handleAISkills,
	"/ai/skills/clone":    handleAISkillsClone,
	"/ai/question/answer": handleAIQuestionAnswer,

	// doctor
	"/doctor/check":               handleDoctorCheck,
	"/doctor/ignore":              handleDoctorIgnore,
	"/doctor/unignore":            handleDoctorUnignore,
	"/permission/macos/status":    handleMacOSPermissionStatus,
	"/permission/macos/reconcile": handleMacOSPermissionReconcile,
	"/permission/macos/open":      handleMacOSPermissionOpen,

	// dictation
	"/dictation/model/download":      handleDictationModelDownload,
	"/dictation/model/delete":        handleDictationModelDelete,
	"/dictation/model/status":        handleDictationModelStatus,
	"/dictation/native-lib/status":   handleDictationNativeLibStatus,
	"/dictation/native-lib/download": handleDictationNativeLibDownload,

	// OCR
	"/ocr/model/status":    handleOCRModelStatus,
	"/ocr/model/download":  handleOCRModelDownload,
	"/ocr/engine/status":   handleOCREngineStatus,
	"/ocr/engine/download": handleOCREngineDownload,

	// others
	"/":                            handleHome,
	"/tooltip/show":                handleTooltipOverlayShow,
	"/tooltip/hide":                handleTooltipOverlayHide,
	"/preview":                     handlePreview,
	"/preview/image/overlay":       handlePreviewImageOverlay,
	"/preview/file/media":          handlePreviewFileMedia,
	"/image/file/icon":             handleFileIcon,
	"/image/resolve":               handleResolveImage,
	"/image/lazy/load":             handleLazyImageLoad,
	"/open":                        handleOpen,
	"/backup/now":                  handleBackupNow,
	"/backup/restore":              handleBackupRestore,
	"/backup/all":                  handleBackupAll,
	"/backup/folder":               handleBackupFolder,
	"/log/clear":                   handleLogClear,
	"/log/open":                    handleLogOpen,
	"/diagnostics/status":          handleDiagnosticsStatus,
	"/diagnostics/monitor/enable":  handleDiagnosticsMonitorEnable,
	"/diagnostics/monitor/disable": handleDiagnosticsMonitorDisable,
	"/diagnostics/export":          handleDiagnosticsExport,
	"/hotkey/available":            handleHotkeyAvailable,
	"/hotkey/availability":         handleHotkeyAvailability,
	"/glance":                      handleGlance,
	"/glance/action":               handleGlanceAction,
	"/updater/channel/versions":    handleUpdateChannelVersions,
	"/version":                     handleVersion,
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

type fileIconRequest struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

func handleFileIcon(w http.ResponseWriter, r *http.Request) {
	// File previews run in UI, but icon extraction already belongs to core's
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

type resolveImageRequest struct {
	Image common.WoxImage
	Size  int
}

// handleResolveImage converts image types whose cache and network policy belong to core into a raster payload.
func handleResolveImage(w http.ResponseWriter, r *http.Request) {
	var request resolveImageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeErrorResponse(w, fmt.Sprintf("invalid image resolve request: %s", err.Error()))
		return
	}
	resolved, err := NewCoreServices().ResolveImage(getTraceContext(r), getSessionIdFromHeader(r), request.Image, request.Size)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, resolved)
}

func handlePreviewFileMedia(w http.ResponseWriter, r *http.Request) {
	// Media previews need ordinary HTTP range requests so large video files can
	// stream into WebView without loading the whole file into UI memory.
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
	// Plugins still return ordinary WoxImage values, while UI exchanges the
	// manager-issued token for a resized cache image only after the widget exists.
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

	icon, err := NewCoreServices().LoadLazyResultImage(getTraceContext(r), getSessionIdFromHeader(r), token)
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

	preview, err := NewCoreServices().ResultPreview(getTraceContext(r), getSessionIdFromHeader(r), sessionId, queryId, id)
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
	if err := NewCoreServices().ShowPreviewImage(getTraceContext(r), getSessionIdFromHeader(r), request.Image); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleTheme(w http.ResponseWriter, r *http.Request) {
	theme, err := NewCoreServices().CurrentTheme(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, theme)
}

func handlePluginStore(w http.ResponseWriter, r *http.Request) {
	plugins, err := getStorePluginDTOs(getTraceContext(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, plugins)
}

func handlePluginInstalled(w http.ResponseWriter, r *http.Request) {
	defer util.GoRecover(getTraceContext(r), "get installed plugins")

	plugins, err := getInstalledPluginDTOs(getTraceContext(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
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
		// keeps UI dropdowns simple while preserving i18n keys in plugin.json.
		glance.Name = common.I18nString(pluginInstance.TranslateMetadataText(ctx, glance.Name))
		glance.Description = common.I18nString(pluginInstance.TranslateMetadataText(ctx, glance.Description))
		glances = append(glances, glance)
	}
	return glances
}

func handlePluginInstall(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "Plugin ID is required for installation")
		return
	}

	if err := NewCoreServices().OperatePlugin(getTraceContext(r), getSessionIdFromHeader(r), idResult.String(), contract.PluginOperationInstall); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handlePluginUninstall(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	if err := NewCoreServices().OperatePlugin(getTraceContext(r), getSessionIdFromHeader(r), idResult.String(), contract.PluginOperationUninstall); err != nil {
		writeErrorResponse(w, err.Error())
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

	if err := NewCoreServices().OperatePlugin(getTraceContext(r), getSessionIdFromHeader(r), idResult.String(), contract.PluginOperationDisable); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handlePluginEnable(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	if err := NewCoreServices().OperatePlugin(getTraceContext(r), getSessionIdFromHeader(r), idResult.String(), contract.PluginOperationEnable); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

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
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	if err := NewCoreServices().OperateTheme(getTraceContext(r), getSessionIdFromHeader(r), idResult.String(), contract.ThemeOperationInstall); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleThemeUninstall(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	if err := NewCoreServices().OperateTheme(getTraceContext(r), getSessionIdFromHeader(r), idResult.String(), contract.ThemeOperationUninstall); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleThemeApply(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		writeErrorResponse(w, "id is empty")
		return
	}

	if err := NewCoreServices().OperateTheme(getTraceContext(r), getSessionIdFromHeader(r), idResult.String(), contract.ThemeOperationApply); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

type saveThemeRequest struct {
	Name      string       `json:"Name"`
	Theme     common.Theme `json:"Theme"`
	Overwrite bool         `json:"Overwrite"`
}

// handleThemeSave persists an edited draft as either a new user theme or an overwrite of the current user theme.
func handleThemeSave(w http.ResponseWriter, r *http.Request) {
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

	theme, err := NewCoreServices().SaveTheme(getTraceContext(r), getSessionIdFromHeader(r), request.Name, request.Theme, request.Overwrite)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, theme)
}

func handleSettingWox(w http.ResponseWriter, r *http.Request) {
	loaded, err := NewCoreServices().GeneralSettings(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, dto.WoxSettingDto{
		EnableAutostart: loaded.EnableAutostart, MainHotkey: loaded.MainHotkey, SelectionHotkey: loaded.SelectionHotkey,
		IgnoredHotkeyApps: loaded.IgnoredHotkeyApps, LogLevel: loaded.LogLevel, UsePinYin: loaded.UsePinYin,
		SwitchInputMethodABC: loaded.SwitchInputMethodABC, HideOnStart: loaded.HideOnStart, OnboardingFinished: loaded.OnboardingFinished,
		HideOnLostFocus: loaded.HideOnLostFocus, ShowTray: loaded.ShowTray, LangCode: loaded.LangCode,
		QueryHotkeys: loaded.QueryHotkeys, QueryShortcuts: loaded.QueryShortcuts, TrayQueries: loaded.TrayQueries,
		LaunchMode: loaded.LaunchMode, StartPage: loaded.StartPage, AIProviders: loaded.AIProviders,
		AIMCPServers: loaded.AIMCPServers, AISkills: loaded.AISkills, HttpProxyEnabled: loaded.HTTPProxyEnabled,
		HttpProxyUrl: loaded.HTTPProxyURL, ShowPosition: loaded.ShowPosition, IsLinuxWaylandSession: loaded.IsLinuxWaylandSession,
		IsEvdevReadAvailable: loaded.IsEvdevReadAvailable, EnableAutoBackup: loaded.EnableAutoBackup, EnableAutoUpdate: loaded.EnableAutoUpdate,
		ReleaseChannel: loaded.ReleaseChannel, EnableAnonymousUsageStats: loaded.EnableAnonymousUsageStats,
		CustomPythonPath: loaded.CustomPythonPath, CustomNodejsPath: loaded.CustomNodejsPath,
		CloudSyncServerUrl: loaded.CloudSyncServerURL, CloudSyncDisabledPlugins: loaded.CloudSyncDisabledPlugins,
		AppWidth: loaded.AppWidth, MaxResultCount: loaded.MaxResultCount, UiDensity: loaded.UIDensity, ThemeId: loaded.ThemeID,
		AppFontFamily: loaded.AppFontFamily, EnableQueryCompletionHint: loaded.EnableQueryCompletionHint,
		EnableGlance: loaded.EnableGlance, PrimaryGlance: loaded.PrimaryGlance, HideGlanceIcon: loaded.HideGlanceIcon,
		ShowScoreTail: loaded.ShowScoreTail, ShowPerformanceTail: loaded.ShowPerformanceTail,
		ShowPerformanceTailBatch: loaded.ShowPerformanceTailBatch, ShowPerformanceTailPluginQuery: loaded.ShowPerformanceTailPluginQuery,
		ShowPerformanceTailBackendPrepared: loaded.ShowPerformanceTailBackendPrepared, ShowPerformanceTailUiReceived: loaded.ShowPerformanceTailUIReceived,
	})
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

	var payload keyValuePair
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if err := NewCoreServices().UpdateGeneralSetting(getTraceContext(r), getSessionIdFromHeader(r), payload.Key, payload.Value); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
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

	items, err := NewCoreServices().GlanceItems(getTraceContext(r), getSessionIdFromHeader(r), keys, request.Reason)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
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

	if err := NewCoreServices().ExecuteGlanceAction(getTraceContext(r), getSessionIdFromHeader(r), request.PluginId, request.GlanceId, request.ActionId); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, "")
}

func handleAccountStatus(w http.ResponseWriter, r *http.Request) {
	status, err := NewCoreServices().AccountStatus(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, status)
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
	result, err := NewCoreServices().RegisterAccount(getTraceContext(r), getSessionIdFromHeader(r), payload.Email, payload.Password, payload.Lang)
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
	result, err := NewCoreServices().VerifyAccountEmail(getTraceContext(r), getSessionIdFromHeader(r), payload.Email, payload.Code, payload.Lang)
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
	result, err := NewCoreServices().LoginAccount(getTraceContext(r), getSessionIdFromHeader(r), payload.Email, payload.Password, payload.Lang)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, result)
}

func handleAccountLogout(w http.ResponseWriter, r *http.Request) {
	if err := NewCoreServices().LogoutAccount(getTraceContext(r), getSessionIdFromHeader(r)); err != nil {
		writeErrorResponse(w, err.Error())
		return
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
	if err := NewCoreServices().ResendAccountVerification(getTraceContext(r), getSessionIdFromHeader(r), payload.Email, payload.Lang); err != nil {
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
	if err := NewCoreServices().RequestAccountPasswordReset(getTraceContext(r), getSessionIdFromHeader(r), payload.Email, payload.Lang); err != nil {
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
	if err := NewCoreServices().ConfirmAccountPasswordReset(getTraceContext(r), getSessionIdFromHeader(r), payload.Token, payload.Password, payload.Lang); err != nil {
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
	if err := NewCoreServices().ChangeAccountPassword(getTraceContext(r), getSessionIdFromHeader(r), payload.CurrentPassword, payload.NewPassword, payload.Lang); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleAccountBillingCheckout(w http.ResponseWriter, r *http.Request) {
	session, err := NewCoreServices().BillingSession(getTraceContext(r), getSessionIdFromHeader(r), contract.BillingSessionCheckout)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, session)
}

func handleAccountBillingPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := NewCoreServices().BillingPlan(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, plan)
}

func handleAccountBillingPortal(w http.ResponseWriter, r *http.Request) {
	session, err := NewCoreServices().BillingSession(getTraceContext(r), getSessionIdFromHeader(r), contract.BillingSessionPortal)
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
	status, err := NewCoreServices().CloudSyncStatus(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, status)
}

type syncBootstrapStatusResponse struct {
	HasRemoteData bool `json:"has_remote_data"`
	HasRemoteKey  bool `json:"has_remote_key"`
}

func handleSyncBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	status, err := NewCoreServices().CloudBootstrapStatus(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, syncBootstrapStatusResponse{HasRemoteData: status.HasRemoteData, HasRemoteKey: status.HasRemoteKey})
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
	if err := NewCoreServices().StartCloudBootstrap(getTraceContext(r), getSessionIdFromHeader(r), payload.RecoveryCode); err != nil {
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
		// A failed snapshot apply does not invalidate the locally restored key, so retries must not request the recovery code again.
		if !service.KeyManager.GetStatus(ctx).Available {
			if strings.TrimSpace(recoveryCode) == "" {
				return fmt.Errorf("recovery_code is empty")
			}
			if _, err := service.KeyManager.FetchWithRecoveryCode(ctx, recoveryCode); err != nil {
				return err
			}
		}
	} else {
		if status.HasRemoteData {
			return fmt.Errorf("cloud sync key is missing")
		}
		if strings.TrimSpace(recoveryCode) == "" {
			return fmt.Errorf("recovery_code is empty")
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
		startCloudSyncManagerIfSyncEnabled(ctx, service)
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
	if err := NewCoreServices().JoinCloudDevice(getTraceContext(r), getSessionIdFromHeader(r)); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleSyncPush(w http.ResponseWriter, r *http.Request) {
	if err := NewCoreServices().PushCloudChanges(getTraceContext(r), getSessionIdFromHeader(r)); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleSyncPull(w http.ResponseWriter, r *http.Request) {
	if err := NewCoreServices().PullCloudChanges(getTraceContext(r), getSessionIdFromHeader(r)); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
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
	resp, err := NewCoreServices().CloudDevices(getTraceContext(r), getSessionIdFromHeader(r))
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

	response, err := NewCoreServices().RevokeCloudDevice(getTraceContext(r), getSessionIdFromHeader(r), payload.TargetDeviceID)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, response)
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
	statuses := getRuntimeStatuses(getTraceContext(r))
	converted := make([]dto.RuntimeStatusDto, len(statuses))
	for index, status := range statuses {
		converted[index] = dto.RuntimeStatusDto{
			Runtime:           status.Runtime,
			IsStarted:         status.IsStarted,
			HostVersion:       status.HostVersion,
			StatusCode:        status.StatusCode,
			StatusMessage:     status.StatusMessage,
			ExecutablePath:    status.ExecutablePath,
			LastStartError:    status.LastStartError,
			CanRestart:        status.CanRestart,
			InstallUrl:        status.InstallURL,
			LoadedPluginCount: status.LoadedPluginCount,
			LoadedPluginNames: append([]string(nil), status.LoadedPluginNames...),
		}
	}
	writeSuccessResponse(w, converted)
}

func handleRuntimeRestart(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	runtimeResult := gjson.GetBytes(body, "Runtime")
	if !runtimeResult.Exists() {
		writeErrorResponse(w, "Runtime is required")
		return
	}

	if err := NewCoreServices().RestartRuntime(getTraceContext(r), getSessionIdFromHeader(r), runtimeResult.String()); err != nil {
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

	if err := NewCoreServices().UpdatePluginSettings(getTraceContext(r), getSessionIdFromHeader(r), kv.PluginId, map[string]string{kv.Key: kv.Value}); err != nil {
		writeErrorResponse(w, err.Error())
		return
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

	if err := NewCoreServices().OpenPath(getTraceContext(r), getSessionIdFromHeader(r), pathResult.String()); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
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
	if err := NewCoreServices().CreateDataBackup(getTraceContext(r), getSessionIdFromHeader(r)); err != nil {
		writeErrorResponse(w, err.Error())
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

	if err := NewCoreServices().RestoreDataBackup(getTraceContext(r), getSessionIdFromHeader(r), idResult.String()); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleBackupAll(w http.ResponseWriter, r *http.Request) {
	backups, err := NewCoreServices().DataBackups(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	legacy := make([]setting.Backup, len(backups))
	for index, backup := range backups {
		legacy[index] = setting.Backup{Id: backup.ID, Name: backup.Name, Timestamp: backup.Timestamp, Type: setting.BackupType(backup.Type), Path: backup.Path}
	}
	writeSuccessResponse(w, legacy)
}

func handleBackupFolder(w http.ResponseWriter, r *http.Request) {
	backupDir, err := NewCoreServices().BackupFolder(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, backupDir)
}

func handleLogClear(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	if err := NewCoreServices().ClearLogs(ctx, getSessionIdFromHeader(r)); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

func handleLogOpen(w http.ResponseWriter, r *http.Request) {
	if err := NewCoreServices().OpenLog(getTraceContext(r), getSessionIdFromHeader(r)); err != nil {
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
	state, err := EnableDiagnosticsMonitorAndRestart(ctx)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, state)
}

// EnableDiagnosticsMonitorAndRestart enables bug-aware monitoring before restarting the primary instance.
func EnableDiagnosticsMonitorAndRestart(ctx context.Context) (diagnostic.State, error) {
	state, err := enableDiagnosticsMonitor(ctx)
	if err != nil {
		return diagnostic.State{}, err
	}
	if err := diagnostic.GetManager().StartSupervisorDetached(ctx, true); err != nil {
		return diagnostic.State{}, err
	}
	util.Go(ctx, "restart wox for bug aware monitor", func() {
		time.Sleep(200 * time.Millisecond)
		GetUIManager().ExitApp(util.NewTraceContext())
	})
	return state, nil
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
	body, _ := io.ReadAll(r.Body)
	hotkeyResult := gjson.GetBytes(body, "hotkey")
	if !hotkeyResult.Exists() {
		writeErrorResponse(w, "hotkey is empty")
		return
	}

	availability, err := NewCoreServices().CheckHotkeyAvailability(getTraceContext(r), getSessionIdFromHeader(r), hotkeyResult.String())
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, availability)
}

// startCloudSyncManagerAfterUIReady starts the scheduler only after the UI can apply settings from a scheduled pull.
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
	state, err := cloudsync.LoadCloudSyncState(ctx)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to load cloud sync state before starting scheduler: %v", err))
		return
	}
	if !state.Bootstrapped {
		return
	}
	service.StartManager(ctx)
}

func handleLangAvailable(w http.ResponseWriter, r *http.Request) {
	languages, err := NewCoreServices().AvailableLanguages(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, languages)
}

func handleLangJson(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	langCodeResult := gjson.GetBytes(body, "langCode")
	if !langCodeResult.Exists() {
		writeErrorResponse(w, "langCode is empty")
		return
	}
	langCode := langCodeResult.String()

	langJson, err := NewCoreServices().LanguageJSON(getTraceContext(r), getSessionIdFromHeader(r), i18n.LangCode(langCode))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	writeSuccessResponse(w, langJson)
}

func handleOnQueryBoxFocus(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	GetUIManager().PostOnQueryBoxFocus(ctx)
	writeSuccessResponse(w, "")
}

func handleOnHotkeyRecording(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	isRecordingResult := gjson.GetBytes(body, "isRecording")
	if !isRecordingResult.Exists() {
		writeErrorResponse(w, "isRecording is required")
		return
	}

	purpose := strings.TrimSpace(gjson.GetBytes(body, "purpose").String())
	allowedKinds := []string{}
	gjson.GetBytes(body, "allowedKinds").ForEach(func(_, value gjson.Result) bool {
		if kind := strings.TrimSpace(value.String()); kind != "" {
			allowedKinds = append(allowedKinds, kind)
		}
		return true
	})
	if isRecordingResult.Bool() {
		if purpose == "" {
			writeErrorResponse(w, "purpose is required when recording starts")
			return
		}
		if len(allowedKinds) == 0 {
			writeErrorResponse(w, "allowedKinds is required when recording starts")
			return
		}
	}

	var (
		capability contract.HotkeyRecordingCapability
		err        error
	)
	if isRecordingResult.Bool() {
		capability, err = NewCoreServices().StartHotkeyRecording(getTraceContext(r), getSessionIdFromHeader(r), purpose, allowedKinds)
	} else {
		err = NewCoreServices().StopHotkeyRecording(getTraceContext(r), getSessionIdFromHeader(r))
	}
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, capability)
}

func handleOnHotkeyRecordingCandidate(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	hotkeyResult := gjson.GetBytes(body, "hotkey")
	if !hotkeyResult.Exists() || strings.TrimSpace(hotkeyResult.String()) == "" {
		writeErrorResponse(w, "hotkey is required")
		return
	}

	if err := NewCoreServices().SubmitHotkeyRecordingCandidate(getTraceContext(r), getSessionIdFromHeader(r), hotkeyResult.String()); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
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
	results := getAIModels(ctx)
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

// handleAIChatGet returns the full chat data for a lightweight preview sidebar entry.
func handleAIChatGet(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	chatId := gjson.GetBytes(body, "chatId").String()
	if chatId == "" {
		writeErrorResponse(w, "chatId is empty")
		return
	}

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	chatData, ok := chater.GetChat(ctx, chatId)
	if !ok {
		writeErrorResponse(w, "chat not found")
		return
	}

	writeSuccessResponse(w, chatData)
}

// handleAIChatStop cancels the active streaming session for a chat.
func handleAIChatStop(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	chatId := gjson.GetBytes(body, "chatId").String()
	if chatId == "" {
		writeErrorResponse(w, "chatId is empty")
		return
	}

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	stopped := chater.StopChat(ctx, chatId)
	writeSuccessResponse(w, stopped)
}

// handleAIChatDelete lets the chat preview manage its own sidebar state.
func handleAIChatDelete(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	chatId := gjson.GetBytes(body, "chatId").String()
	if chatId == "" {
		writeErrorResponse(w, "chatId is empty")
		return
	}

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	if !chater.DeleteChat(ctx, chatId) {
		writeErrorResponse(w, "chat not found")
		return
	}

	writeSuccessResponse(w, true)
}

// handleAIChatSummarize starts a title refresh without going through result actions.
func handleAIChatSummarize(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	chatId := gjson.GetBytes(body, "chatId").String()
	if chatId == "" {
		writeErrorResponse(w, "chatId is empty")
		return
	}

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	if !chater.SummarizeChat(ctx, chatId) {
		writeErrorResponse(w, "chat not found")
		return
	}

	writeSuccessResponse(w, true)
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

func handleAISkills(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	chater := plugin.GetPluginManager().GetAIChatPluginChater(ctx)
	if chater == nil {
		writeErrorResponse(w, "ai chat plugin not found")
		return
	}

	skills := chater.GetAllSkills(ctx)
	writeSuccessResponse(w, skills)
}

func handleAISkillsClone(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	parsed := gjson.ParseBytes(body)
	url := parsed.Get("url").String()
	skills, err := NewCoreServices().CloneAISkills(getTraceContext(r), getSessionIdFromHeader(r), url)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}

	results := make([]map[string]interface{}, 0, len(skills))
	for _, skill := range skills {
		results = append(results, map[string]interface{}{
			"Path":         skill.Path,
			"ManifestPath": skill.ManifestPath,
			"Name":         skill.Name,
			"Description":  skill.Description,
			"Error":        skill.Error,
			"Source":       skill.Source,
			"SourceName":   skill.SourceName,
			"SourceUrl":    skill.SourceURL,
			"Enabled":      skill.Enabled,
		})
	}
	writeSuccessResponse(w, results)
}

func handleAIQuestionAnswer(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)

	body, _ := io.ReadAll(r.Body)
	parsed := gjson.ParseBytes(body)
	questionId := parsed.Get("questionId").String()
	answer := parsed.Get("answer").String()
	if questionId == "" {
		writeErrorResponse(w, "questionId is required")
		return
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("AI: resolving question answer for questionId=%s", questionId))
	aitool.ResolveAIQuestionAnswer(questionId, answer)
	writeSuccessResponse(w, "")
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

func handleMacOSPermissionStatus(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	writeSuccessResponse(w, currentMacOSPermissionStatus(ctx))
}

func handleMacOSPermissionReconcile(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	status := currentMacOSPermissionStatus(ctx)
	if err := keyboard.ReconcileRawKeyListenerAccessWithPermissionStatus(status.Accessibility == permission.MacOSPermissionGranted); err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to reconcile macOS raw keyboard access: %s", err.Error()))
	}
	writeSuccessResponse(w, status)
}

// currentMacOSPermissionStatus falls back to the in-process checks if the isolated probe cannot start.
func currentMacOSPermissionStatus(ctx context.Context) permission.MacOSPermissionStatus {
	status, err := permission.ProbeMacOSPermissionStatus(ctx)
	if err == nil {
		return status
	}
	logger.Warn(ctx, fmt.Sprintf("failed to run isolated macOS permission probe: %s", err.Error()))
	return permission.GetMacOSPermissionStatusDirect(ctx)
}

func handleMacOSPermissionOpen(w http.ResponseWriter, r *http.Request) {
	ctx := getTraceContext(r)
	var req struct {
		PermissionType permission.MacOSPermissionType `json:"permissionType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	if !permission.IsValidMacOSPermissionType(req.PermissionType) {
		writeErrorResponse(w, "invalid macOS permission type")
		return
	}
	GetUIManager().GetUI(ctx).OpenMacOSPermissionFlow(ctx, string(req.PermissionType))
	writeSuccessResponse(w, "")
}

func handleUserDataLocation(w http.ResponseWriter, r *http.Request) {
	location, err := NewCoreServices().DataLocation(getTraceContext(r), getSessionIdFromHeader(r))
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, location)
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

	err := NewCoreServices().ChangeDataLocation(ctx, getSessionIdFromHeader(r), newLocation)
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

// handleDictationModelDownload starts a model download for the dictation plugin.
// The request body should contain {"modelId": "..."} where modelId is one of the
// recommended model IDs. The download runs asynchronously and progress is
// reported via the model status endpoint.
func handleDictationModelDownload(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	modelID := gjson.GetBytes(body, "modelId").String()
	if err := NewCoreServices().OperateManagedModel(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelDictation, contract.ManagedModelOperationDownload, modelID); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

// handleDictationModelDelete deletes a downloaded model from disk.
func handleDictationModelDelete(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	modelID := gjson.GetBytes(body, "modelId").String()
	if err := NewCoreServices().OperateManagedModel(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelDictation, contract.ManagedModelOperationDelete, modelID); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

// handleDictationModelStatus returns the download status for all known models.
// The UI side polls this to update the model dropdown with live progress.
func handleDictationModelStatus(w http.ResponseWriter, r *http.Request) {
	status, err := NewCoreServices().ManagedModelStatuses(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelDictation)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, status)
}

// handleDictationNativeLibStatus returns the native library download status.
func handleDictationNativeLibStatus(w http.ResponseWriter, r *http.Request) {
	status, err := NewCoreServices().ManagedModelEngineStatus(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelDictation)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, status)
}

// handleDictationNativeLibDownload triggers a native library download.
func handleDictationNativeLibDownload(w http.ResponseWriter, r *http.Request) {
	if err := NewCoreServices().OperateManagedModel(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelDictation, contract.ManagedModelOperationDownloadEngine, ""); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

// handleOCRModelStatus returns the shared downloadable OCR model status.
func handleOCRModelStatus(w http.ResponseWriter, r *http.Request) {
	status, err := NewCoreServices().ManagedModelStatuses(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelOCR)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, status)
}

// handleOCRModelDownload starts a PP-OCRv6 small download without blocking
// the settings request. The picker polls the status endpoint for progress.
func handleOCRModelDownload(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	modelID := gjson.GetBytes(body, "modelId").String()
	if err := NewCoreServices().OperateManagedModel(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelOCR, contract.ManagedModelOperationDownload, modelID); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}

// handleOCREngineStatus returns the shared ONNX Runtime state for PaddleOCR.
func handleOCREngineStatus(w http.ResponseWriter, r *http.Request) {
	status, err := NewCoreServices().ManagedModelEngineStatus(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelOCR)
	if err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, status)
}

// handleOCREngineDownload starts the OCR runtime download without blocking settings.
func handleOCREngineDownload(w http.ResponseWriter, r *http.Request) {
	if err := NewCoreServices().OperateManagedModel(getTraceContext(r), getSessionIdFromHeader(r), contract.ManagedModelOCR, contract.ManagedModelOperationDownloadEngine, ""); err != nil {
		writeErrorResponse(w, err.Error())
		return
	}
	writeSuccessResponse(w, "")
}
