package launcher

import (
	"context"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/permission"
)

// onboardingServices is the Core surface the first-run guide and launcher migration use.
type onboardingServices interface {
	contract.LifecycleServices
	contract.GeneralSettingsServices
	contract.OnboardingSettingsServices
	contract.HotkeyInteractionSettingsServices
	contract.PluginCatalogSettingsServices
	contract.PluginOperationSettingsServices
	contract.ThemeOperationSettingsServices
	contract.MigrationImportServices
}

// onboardingController owns the first-run guide: its window, page state, setup actions
// and launcher migration. Settings-domain controllers are read through getters because
// they are created after controller binding. Cross-window transitions (switching to
// Settings, hotkey recording routing, the macOS permission flow) stay with App.
type onboardingController struct {
	services     onboardingServices
	sessionID    string
	lifecycleCtx context.Context
	windows      *woxui.WindowManager
	runOnUI      func(string, func()) error
	fontFamily   func() string
	primaryOwner func() *onboardingController
	window       func() *woxui.Window
	language     *languageCatalog
	appearance   *appearanceController

	generalSettings func() *generalSettingsController
	hotkeySettings  func() *hotkeySettingsController
	themeSettings   func() *themeSettingsController
	pluginSettings  func() *pluginSettingsController
	settingsSearch  func() *settingsSearchController

	settingsSnapshot   func() settingsSnapshot
	imageFor           func(woxImage) *woxui.Image
	imageForTint       func(woxImage, *woxui.Color, int) *woxui.Image
	imageForSurface    func(woxImage, int, woxui.Color) *woxui.Image
	reloadSettings     func() error
	reloadTranslations func() error
	reloadThemes       func(mode, preferredID string) error
	currentThemeID     func() string
	loadGlanceCatalog  func()
	showLauncher       func(showAppParams) error

	startHotkeyRecording       func(idPrefix string, target *formFieldsState, index int, persistKey string, allowedKinds []string, onKey func(woxui.KeyEvent) bool)
	stopHotkeyRecording        func()
	hotkeyRecordingFieldStatus func(idPrefix string, index int) hotkeyRecordingPresentation
	recordHotkeySettingsField  func(index int)
	hotkeyConflictMessage      func(kind, value string) string
	releaseDemoWallpaperLocked func()
	openMacOSPermissionFlow    func(permissionType string) error
	// permissionFlowTarget reports the permission the open macOS guide is waiting for, if any.
	permissionFlowTarget func() permission.MacOSPermissionType

	hotkeyRecordingUsesOnboardingWindow func() bool
	onOnboardingWindowKey               func(woxui.KeyEvent) bool
	onOnboardingWindowClosed            func()

	view              *woxui.ManagedWindow
	host              *woxwidget.Host
	open              bool
	step              int
	choice            string
	choiceAnchor      woxui.Rect
	permissionStatus  contract.MacOSPermissionStatus
	permissionLoading bool
	pageError         string
	queryHotkey       *onboardingQueryHotkeyState
	plugins           onboardingPluginState
	themes            onboardingThemeState
	migration         onboardingMigrationState
}

// bindOnboarding supplies the guide's services, peer settings controllers and the
// App-owned actions it triggers.
func (a *App) bindOnboarding() {
	c := &a.onboarding
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.windows = a.windows
	c.runOnUI = a.runOnUI
	c.fontFamily = func() string { return a.generalSettings.Data().AppFontFamily }
	c.primaryOwner = func() *onboardingController {
		if !a.isPrimary && a.primary != nil {
			return &a.primary.onboarding
		}
		return nil
	}
	c.window = func() *woxui.Window { return a.window }
	c.language = &a.language
	c.appearance = &a.appearance

	c.generalSettings = func() *generalSettingsController { return a.generalSettings }
	c.hotkeySettings = func() *hotkeySettingsController { return a.hotkeySettings }
	c.themeSettings = func() *themeSettingsController { return a.themeSettings }
	c.pluginSettings = func() *pluginSettingsController { return a.pluginSettings }
	c.settingsSearch = func() *settingsSearchController { return a.settingsSearch }

	c.settingsSnapshot = a.settingsSnapshot
	c.imageFor = a.imageFor
	c.imageForTint = a.imageForTint
	c.imageForSurface = a.imageForSurface
	c.reloadSettings = a.reloadSettings
	c.reloadTranslations = a.reloadTranslations
	c.reloadThemes = func(mode, preferredID string) error { return a.themeSettings.reloadThemes(mode, preferredID) }
	c.currentThemeID = func() string { return a.themeSettings.currentThemeID() }
	c.loadGlanceCatalog = a.loadGlanceCatalog
	c.showLauncher = a.showWindow

	c.startHotkeyRecording = a.startHotkeyRecording
	c.stopHotkeyRecording = a.stopHotkeyRecording
	c.hotkeyRecordingFieldStatus = a.hotkeyRecordingFieldStatus
	c.recordHotkeySettingsField = a.recordHotkeySettingsField
	c.hotkeyConflictMessage = a.hotkeyConflictMessage
	c.releaseDemoWallpaperLocked = func() { a.themeSettings.releaseDemoWallpaperLocked() }
	c.openMacOSPermissionFlow = a.openMacOSPermissionFlow
	c.permissionFlowTarget = func() permission.MacOSPermissionType {
		if a.permissionFlow == nil {
			return ""
		}
		return a.permissionFlow.permissionType
	}

	c.hotkeyRecordingUsesOnboardingWindow = a.hotkeyRecordingUsesOnboardingWindow
	c.onOnboardingWindowKey = a.onOnboardingWindowKey
	c.onOnboardingWindowClosed = a.onOnboardingWindowClosed
}

func (c *onboardingController) isOpen() bool {
	return c.open
}

// ownsHotkeyTarget reports whether a hotkey recorder target is the guide's query hotkey field.
func (c *onboardingController) ownsHotkeyTarget(target *formFieldsState) bool {
	return c.open && c.queryHotkey != nil && target == &c.queryHotkey.form
}

// showPageError surfaces a result that arrives after the guide closed its hotkey recorder,
// such as an asynchronous main-hotkey save.
func (c *onboardingController) showPageError(message string) {
	if c.open {
		c.pageError = message
	}
}

// clearStagedMainHotkey drops the migrated shortcut once the user saved one explicitly.
func (c *onboardingController) clearStagedMainHotkey() {
	c.migration.stagedMainHotkey = ""
	c.migration.stagedMainHotkeyError = ""
}

// visibleWindow returns the guide window while the guide is open and on screen.
func (c *onboardingController) visibleWindow() *woxui.ManagedWindow {
	if c.open && c.view != nil && c.view.Lifecycle() == woxui.WindowLifecycleVisible {
		return c.view
	}
	return nil
}

func (c *onboardingController) setFontFamily(fontFamily string) error {
	if c.view == nil {
		return nil
	}
	return c.view.Window().SetFontFamily(fontFamily)
}
