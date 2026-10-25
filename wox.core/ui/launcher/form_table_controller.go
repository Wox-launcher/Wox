package launcher

import (
	"context"

	"wox/common"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// formTableServices is the Core surface used by table row helpers.
type formTableServices interface {
	FetchWebsiteIcon(ctx context.Context, sessionID string, websiteURL string) (common.WoxImage, error)
	IndexedApps(ctx context.Context, sessionID string, pattern string) ([]contract.HotkeyApp, error)
	CloneAISkills(ctx context.Context, sessionID string, sourceURL string) ([]contract.AISkill, error)
}

// formTableController owns the shared table editor overlay used by Settings pages,
// plugin settings, action forms and requirement forms. Editors opened from a Settings
// form live in settingsEditor and render in the Settings window; all others live in
// launcherEditor. The Settings editor wins when both exist because it stays modal while
// the launcher is used. Which form currently owns a table is decided by App through
// targetCurrent and targetUsesSettings.
//
// Some tables carry feature-specific row editors (window-manager groups, AI providers,
// MCP servers, skills, plugin trigger keywords). Those features stay with their owners and
// are reached through the callbacks below.
type formTableController struct {
	settingsEditor *formTableEditorState
	launcherEditor *formTableEditorState
	emoji          formTableEmojiCatalog

	services     formTableServices
	sessionID    string
	lifecycleCtx context.Context
	runOnUI      func(string, func()) error
	language     *languageCatalog
	launcher     func() *woxui.Window
	launcherHost func() *woxwidget.Host
	settings     *settingsWindowController

	aiSettings     func() *aiSettingsController
	pluginSettings func() *pluginSettingsController
	hotkeySettings func() *hotkeySettingsController

	imageForTint    func(woxImage, *woxui.Color, int) *woxui.Image
	imageForSurface func(woxImage, int, woxui.Color) *woxui.Image
	usePinYin       func() bool

	targetCurrent      func(*formFieldsState) bool
	targetUsesSettings func(*formFieldsState) bool
	settingsOwned      func(*formFieldsState) bool
	updateSettingsText func(bool)
	updateLauncherText func(bool)
	runQueryTest       func(plainQuery)

	// Settings page capabilities.
	saveSettingsTable       func(state *formTableEditorState, key, value, previousValue string)
	setSettingChoiceTooltip func(inside bool, text string, anchor woxui.Rect)
	setSettingsDemoHover    func(kind string, inside bool, anchor woxui.Rect)
	selectSettingTab        func(tab string)
	openAboutLink           func(target string)

	// Hotkey recording inside row editors.
	stopHotkeyRecording                  func()
	stopHotkeyRecordingForDifferentField func(target *formFieldsState, index int)
	commitHotkeyRecordingValue           func()
	recordFormTableRowHotkey             func(index int)
	hotkeyRecordingFieldStatus           func(idPrefix string, index int) hotkeyRecordingPresentation
	loadHotkeyAppCandidates              func()

	// Plugin settings tables.
	selectedPluginID                     func() string
	submitPluginSettings                 func()
	validatePluginTriggerKeywordTableRow func(state *formTableEditorState) string
	openAICommandTemplatePicker          func(fieldIndex int)

	// AI settings tables.
	openAISettingsTable func(index int)
	loadAIModels        func()

	// Window-manager group rows use their own editor inside the table overlay.
	beginWindowManagerGroupEdit       func(rowIndex int, rowEditorOnly bool) bool
	buildWindowManagerGroupEditor     func(snapshot *windowGroupEditorSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget
	cancelWindowManagerGroupEdit      func()
	cancelWindowManagerGroupUrlEditor func()
	closeWindowManagerGroupAppPicker  func()
}

// bindFormTables supplies the editor's host windows, App-owned target checks and the
// feature hooks used by specialised tables.
func (a *App) bindFormTables() {
	c := &a.formTables
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.emoji.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.language = &a.language
	c.launcher = func() *woxui.Window { return a.window }
	c.launcherHost = func() *woxwidget.Host { return a.host }
	c.settings = &a.settingsWindow

	c.aiSettings = func() *aiSettingsController { return a.aiSettings }
	c.pluginSettings = func() *pluginSettingsController { return a.pluginSettings }
	c.hotkeySettings = func() *hotkeySettingsController { return a.hotkeySettings }

	c.imageForTint = a.imageForTint
	c.imageForSurface = a.imageForSurface
	c.usePinYin = a.usePinYin

	c.targetCurrent = a.formTableTargetCurrentLocked
	c.targetUsesSettings = a.formTableTargetUsesSettingsLocked
	c.settingsOwned = a.settingsOwnedTableTarget
	c.updateSettingsText = a.updateSettingsTextInput
	c.updateLauncherText = a.updateFormTextInput
	c.runQueryTest = a.runLauncherQueryTest

	c.saveSettingsTable = a.saveSettingsTable
	c.setSettingChoiceTooltip = a.setSettingChoiceTooltip
	c.setSettingsDemoHover = a.setSettingsDemoHover
	c.selectSettingTab = a.selectSettingTab
	c.openAboutLink = a.openAboutLink

	c.stopHotkeyRecording = a.stopHotkeyRecording
	c.stopHotkeyRecordingForDifferentField = a.stopHotkeyRecordingForDifferentField
	c.commitHotkeyRecordingValue = a.commitHotkeyRecordingValue
	c.recordFormTableRowHotkey = a.recordFormTableRowHotkey
	c.hotkeyRecordingFieldStatus = a.hotkeyRecordingFieldStatus
	c.loadHotkeyAppCandidates = a.loadHotkeyAppCandidates

	c.selectedPluginID = a.selectedPluginID
	c.submitPluginSettings = func() { a.pluginSettings.submitPluginSettings() }
	c.validatePluginTriggerKeywordTableRow = func(state *formTableEditorState) string {
		return a.pluginSettings.validatePluginTriggerKeywordTableRow(state)
	}
	c.openAICommandTemplatePicker = a.openAICommandTemplatePicker

	c.openAISettingsTable = a.openAISettingsTable
	c.loadAIModels = a.loadAIModels

	c.beginWindowManagerGroupEdit = a.beginWindowManagerGroupEdit
	c.buildWindowManagerGroupEditor = a.buildWindowManagerGroupEditor
	c.cancelWindowManagerGroupEdit = a.cancelWindowManagerGroupEdit
	c.cancelWindowManagerGroupUrlEditor = a.cancelWindowManagerGroupUrlEditor
	c.closeWindowManagerGroupAppPicker = a.closeWindowManagerGroupAppPicker
}

// activeFormTableEditor returns the Settings editor first because its modal state
// remains independent when the launcher is activated.
func (c *formTableController) activeFormTableEditor() *formTableEditorState {
	if c.settingsEditor != nil {
		return c.settingsEditor
	}
	return c.launcherEditor
}

func (c *formTableController) formTableUsesSettingsWindow() bool {
	return c.settingsEditor != nil
}

func (c *formTableController) formTableNativeWindow() *woxui.Window {
	if c.formTableUsesSettingsWindow() {
		return c.settings.settingsNativeWindow()
	}
	return c.launcher()
}

// formTableHost is the widget host of the window presenting the active editor.
func (c *formTableController) formTableHost() *woxwidget.Host {
	if c.formTableUsesSettingsWindow() {
		return c.settings.host
	}
	return c.launcherHost()
}

func (c *formTableController) invalidateFormTableWindow() {
	if window := c.formTableNativeWindow(); window != nil {
		_ = window.Invalidate()
	}
}

func (c *formTableController) updateFormTableTextInput(enabled bool) {
	if c.formTableUsesSettingsWindow() {
		c.updateSettingsText(enabled)
		return
	}
	c.updateLauncherText(enabled)
}

// discardSettingsEditor drops a Settings-hosted editor when the Settings window closes.
func (c *formTableController) discardSettingsEditor() {
	c.settingsEditor = nil
}

// discardLauncherEditor drops a launcher-hosted editor when Settings takes over.
func (c *formTableController) discardLauncherEditor() {
	c.launcherEditor = nil
}
