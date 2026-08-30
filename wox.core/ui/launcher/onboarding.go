package launcher

import (
	"context"
	"encoding/json"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"

	"wox/resource"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/keyboard"
	"wox/util/overlay/confettioverlay"
	macospermission "wox/util/overlay/macos_permission"
	"wox/util/permission"
)

const (
	onboardingWindowWidth  = float32(1040)
	onboardingWindowHeight = float32(800)
	onboardingGlassDarkID  = "44a933d5-e6de-4c1f-8ee5-b2305c6abdf3"
)

// Onboarding keeps one restrained accent so neutral glass themes still expose progress and success states.
var onboardingAccentColor = woxui.Color{R: 20, G: 184, B: 166, A: 255}

var onboardingGlassDarkTheme = func() woxcomponent.Theme {
	data, err := resource.ThemeFS.ReadFile("themes/glass-dark.json")
	if err != nil {
		return defaultPalette().componentTheme()
	}
	var theme themeData
	if json.Unmarshal(data, &theme) != nil {
		return defaultPalette().componentTheme()
	}
	return paletteForTheme(theme).componentTheme()
}()

type onboardingStepSpec struct {
	id     string
	key    string
	accent woxui.Color
}

type onboardingQueryHotkeyState struct {
	form     formFieldsState
	ready    bool
	selected bool
	saved    bool
	saving   bool
	error    string
}

// onboardingPluginState tracks the catalog and the single install operation shown during onboarding.
type onboardingPluginState struct {
	plugins     []pluginSettingsPlugin
	loading     bool
	operationID string
	error       string
}

// onboardingThemeState tracks the theme being applied from the onboarding catalog.
type onboardingThemeState struct {
	selectedID string
	loading    bool
	applying   bool
	error      string
}

// openOnboarding presents the first-run guide in its dedicated window.
func (a *App) openOnboarding() error {
	if err := a.reloadSettings(); err != nil {
		return err
	}
	wasSettings := false
	var settingsView *woxui.ManagedWindow
	if err := a.runOnUI("prepare onboarding", func() {
		wasSettings = a.settingsOpen
		a.settingsOpen = false
		a.onboardingOpen = true
		a.onboardingStep = 0
		a.onboardingChoice = ""
		a.onboardingChoiceAnchor = woxui.Rect{}
		a.onboardingError = ""
		a.onboardingQueryHotkey = nil
		a.onboardingPlugins = onboardingPluginState{}
		a.onboardingTheme = onboardingThemeState{selectedID: onboardingGlassDarkID}
		a.stopHotkeyRecording()
		settingsView = a.settingsView
		if form := a.hotkeySettings.Form(); form != nil {
			form.active = true
			setFormFieldsFocusLocked(form, 0)
		}
	}); err != nil {
		return err
	}
	if wasSettings {
		if settingsView != nil {
			_ = settingsView.Hide()
		}
		if err := a.notifySettingViewChanged(false); err != nil {
			return err
		}
	}
	if err := a.notifyOnboardingViewChanged(true); err != nil {
		return err
	}
	onboardingView, err := a.ensureOnboardingWindow()
	if err != nil {
		return err
	}
	window := onboardingView.Window()
	if err := window.SetHideOnBlur(false); err != nil {
		return err
	}
	if err := window.SetTextInputState(woxui.TextInputState{}); err != nil {
		return err
	}
	if err := window.Center(woxui.Size{Width: onboardingWindowWidth, Height: onboardingWindowHeight}); err != nil {
		return err
	}
	if _, err := onboardingView.Show(); err != nil {
		return err
	}
	if err := a.runOnUI("load onboarding themes", a.loadOnboardingThemes); err != nil {
		return err
	}
	util.Go(a.lifecycleCtx, "load onboarding glance catalog", a.loadGlanceCatalog)
	if runtime.GOOS == "darwin" {
		util.Go(a.lifecycleCtx, "load onboarding permission status", a.loadOnboardingPermissionStatus)
	}
	return window.Invalidate()
}

// finishOnboarding durably dismisses the guide and restores the launcher.
func (a *App) finishOnboarding() {
	util.Go(a.lifecycleCtx, "finish onboarding", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.services.UpdateGeneralSetting(ctx, a.sessionID, "OnboardingFinished", "true")
		if err != nil {
			cancel()
			_ = a.runOnUI("show onboarding finish error", func() {
				a.onboardingError = err.Error()
				a.invalidateOnboardingWindow()
			})
			return
		}
		show := fromCoreShowOptions(a.services.LauncherShowOptions(ctx, a.sessionID))
		cancel()
		var onboardingView *woxui.ManagedWindow
		_ = a.runOnUI("leave onboarding", func() {
			a.onboardingOpen = false
			a.onboardingChoice = ""
			a.onboardingChoiceAnchor = woxui.Rect{}
			a.stopHotkeyRecording()
			a.releaseDemoWallpaperLocked()
			onboardingView = a.onboardingView
		})
		if err := a.notifyOnboardingViewChanged(false); err != nil {
			log.Printf("notify Wox core after onboarding: %v", err)
		}
		if onboardingView != nil {
			_ = onboardingView.Hide()
		}
		if err := a.showWindow(show); err != nil {
			log.Printf("show launcher after onboarding: %v", err)
		}
		if err := confettioverlay.Show(); err != nil {
			util.GetLogger().Warn(context.Background(), "show onboarding confetti: "+err.Error())
		}
	})
}

func (a *App) buildOnboarding(frame woxui.FrameInfo) woxwidget.Widget {
	snapshot := a.settingsSnapshot()
	steps := a.onboardingSteps()
	systemThemes := onboardingSystemThemes(snapshot.theme.Themes)
	theme := onboardingGlassDarkTheme
	for _, systemTheme := range systemThemes {
		if systemTheme.ID == onboardingGlassDarkID {
			theme = paletteForTheme(systemTheme.previewTheme).componentTheme()
			break
		}
	}
	for index := range steps {
		steps[index].Accent = onboardingAccentColor
	}
	active := min(max(0, a.onboardingStep), len(steps)-1)
	step := steps[active]
	labels := a.onboardingLabels()
	labels["title"] = a.translate("i18n:onboarding_title")
	labels["subtitle"] = a.translate("i18n:onboarding_subtitle")
	labels["back"] = a.translate("i18n:onboarding_back")
	labels["next"] = a.translate("i18n:onboarding_next")
	labels["finish"] = a.translate("i18n:onboarding_finish")
	labels["language"] = a.translate("i18n:ui_lang")
	labels["permission.authorize"] = a.translate("i18n:onboarding_permission_authorize")
	labels["permission.checking"] = a.translate("i18n:onboarding_permission_checking")
	labels["glance.enable"] = a.translate("i18n:onboarding_glance_enable")
	labels["glance.enable.body"] = a.translate("i18n:onboarding_glance_enable_tips")
	labels["glance.primary"] = a.translate("i18n:ui_glance_primary")
	labels["hotkey.available"] = a.translate("i18n:onboarding_hotkey_available")
	labels["hotkey.change"] = a.translate("i18n:onboarding_hotkey_change")
	labels["hotkey.preview"] = a.translate("i18n:onboarding_hotkey_preview")
	if a.onboardingError != "" {
		labels[step.ID+".body"] = a.onboardingError
	}

	choices := a.onboardingChoices(snapshot, a.onboardingChoice, frame.Scale)
	choiceValue := ""
	if a.onboardingChoice == "language" {
		choiceValue = snapshot.general.Data.LangCode
	}
	language := snapshot.general.Data.LangCode
	for _, choice := range snapshot.general.Languages {
		if choice.value == language {
			language = choice.label
			break
		}
	}
	glanceLabel := a.translate("i18n:onboarding_glance_sample_time")
	glanceValue := a.translate("i18n:onboarding_glance_sample_value")
	var glanceIcon *woxui.Image
	currentGlance := snapshot.general.Data.PrimaryGlance
	if a.onboardingChoice == "glance" {
		choiceValue = glanceRefJSON(currentGlance)
	}
	for _, item := range snapshot.appearance.GlanceCatalog {
		if item.Ref == currentGlance {
			glanceLabel = firstNonEmpty(item.Name, item.Ref.GlanceID)
			source := item.Icon
			if item.Preview != nil {
				glanceValue = firstNonEmpty(item.Preview.Text, glanceValue)
				if item.Preview.Icon.ImageData != "" {
					source = item.Preview.Icon
				}
			}
			glanceIcon = a.imageForTint(source, &snapshot.palette.resultTitle, physicalImageSize(18, frame.Scale))
			break
		}
	}
	permissions := []launcherview.OnboardingPermission{
		{
			ID: "accessibility", Title: a.translate("i18n:onboarding_permission_accessibility_title"),
			Description: a.translate("i18n:onboarding_permission_accessibility_body"), Ready: a.onboardingPermission.Accessibility == "granted",
		},
		{
			ID: "fullDiskAccess", Title: a.translate("i18n:onboarding_permission_disk_title"),
			Description: a.translate("i18n:onboarding_permission_disk_body"), Ready: a.onboardingPermission.FullDiskAccess == "granted",
		},
	}
	var mainHotkeyLabels, selectionHotkeyLabels []string
	hotkeyStatus := labels["hotkey.available"]
	hotkeyError := snapshot.general.Data.MainHotkeyRegistrationFailed
	hotkeyBlocked := hotkeyError
	hotkeyRecording := false
	if snapshot.hotkey.Form != nil {
		mainHotkey := snapshot.hotkey.Form.values["MainHotkey"]
		presentation := a.hotkeyRecordingFieldStatus("hotkey-settings", 0)
		if presentation.Active {
			mainHotkey = presentation.Value
			hotkeyStatus = presentation.Status
			hotkeyError = presentation.Error
			hotkeyBlocked = presentation.Error || (snapshot.general.Data.MainHotkeyRegistrationFailed && presentation.Value == snapshot.general.Data.MainHotkey)
			hotkeyRecording = true
		} else if hotkeyError {
			hotkeyStatus = a.translate("i18n:ui_hotkey_conflict_system")
		}
		mainHotkeyLabels = formatHotkeyLabels(mainHotkey)
		selectionHotkeyLabels = formatHotkeyLabels(snapshot.hotkey.Form.values["SelectionHotkey"])
	}
	nextDisabled := step.ID == "mainHotkey" && hotkeyBlocked
	var queryHotkeyLabels []string
	queryHotkeyStatus := labels["queryHotkeys.checking"]
	queryHotkeyError := false
	queryHotkeyRecording := false
	queryHotkeyReady := false
	queryHotkeySelected := false
	queryHotkeyBusy := false
	if state := a.onboardingQueryHotkey; state != nil {
		queryHotkeyLabels = formatHotkeyLabels(state.form.values["Hotkey"])
		queryHotkeyReady = state.ready
		queryHotkeySelected = state.selected
		queryHotkeyBusy = state.saving
		queryHotkeyError = state.error != ""
		if state.error != "" {
			queryHotkeyStatus = state.error
		} else if state.ready {
			queryHotkeyStatus = labels["queryHotkeys.configured"]
		}
		presentation := a.hotkeyRecordingFieldStatus("onboarding-query-hotkey", 0)
		if presentation.Active {
			queryHotkeyLabels = formatHotkeyLabels(presentation.Value)
			queryHotkeyStatus = presentation.Status
			queryHotkeyError = presentation.Error
			queryHotkeyRecording = true
		}
	}
	if step.ID == "queryHotkeys" {
		nextDisabled = queryHotkeyBusy || queryHotkeyRecording || (queryHotkeySelected && !queryHotkeyReady)
	}
	plugins := make([]launcherview.OnboardingPlugin, len(a.onboardingPlugins.plugins))
	for index, plugin := range a.onboardingPlugins.plugins {
		plugins[index] = launcherview.OnboardingPlugin{
			ID: plugin.ID, Name: plugin.Name, Description: plugin.Description,
			Icon: a.imageForSize(plugin.Icon, physicalImageSize(28, frame.Scale)), Installed: plugin.IsInstalled,
			Installing: a.onboardingPlugins.operationID == plugin.ID,
			Disabled:   a.onboardingPlugins.operationID != "",
		}
	}
	themes := make([]launcherview.OnboardingTheme, len(systemThemes))
	for index, theme := range systemThemes {
		preview := paletteForTheme(theme.previewTheme).componentTheme()
		lightPreview := preview
		darkPreview := preview
		if theme.IsAuto {
			lightPreview = themeVariantPreview(snapshot.theme.Themes, theme.LightThemeID, true)
			darkPreview = themeVariantPreview(snapshot.theme.Themes, theme.DarkThemeID, false)
		}
		themes[index] = launcherview.OnboardingTheme{
			ID: theme.ID, Name: theme.Name, Selected: theme.ID == a.onboardingTheme.selectedID, IsAuto: theme.IsAuto,
			PreviewTheme: preview, LightPreviewTheme: lightPreview, DarkPreviewTheme: darkPreview,
		}
	}
	previewTexts := []string{a.translate("i18n:ui_theme_preview_text_1"), a.translate("i18n:ui_theme_preview_text_2")}
	previewSubtitles := []string{
		strings.ReplaceAll(a.translate("i18n:ui_theme_preview_subtitle"), "{index}", "1"),
		strings.ReplaceAll(a.translate("i18n:ui_theme_preview_subtitle"), "{index}", "2"),
	}
	if step.ID == "themeInstall" {
		nextDisabled = a.onboardingTheme.loading || len(themes) == 0
	}
	return launcherview.OnboardingView(launcherview.OnboardingProps{
		Width: frame.Size.Width, Height: frame.Size.Height, AppIcon: a.imageFor(appIconImageSource),
		Wallpaper: snapshot.theme.ThemeWallpaperImage, WallpaperBlurred: snapshot.theme.ThemeWallpaperBlurred,
		Steps: steps, ActiveStep: active, Labels: labels, Language: language,
		GlanceEnabled: snapshot.general.Data.EnableGlance, GlanceLabel: glanceLabel, GlanceValue: glanceValue, GlanceIcon: glanceIcon,
		MainHotkeyLabels: mainHotkeyLabels, SelectHotkeyLabels: selectionHotkeyLabels,
		HotkeyStatus: hotkeyStatus, HotkeyError: hotkeyError, HotkeyRecording: hotkeyRecording,
		QueryHotkeyLabels: queryHotkeyLabels, QueryHotkeyStatus: queryHotkeyStatus, QueryHotkeyError: queryHotkeyError,
		QueryHotkeyRecording: queryHotkeyRecording, QueryHotkeyReady: queryHotkeyReady, QueryHotkeySelected: queryHotkeySelected, QueryHotkeyBusy: queryHotkeyBusy,
		Plugins: plugins, PluginsLoading: a.onboardingPlugins.loading, PluginsError: a.onboardingPlugins.error,
		Themes: themes, ThemesLoading: a.onboardingTheme.loading, ThemesApplying: a.onboardingTheme.applying, ThemesError: a.onboardingTheme.error,
		ThemePreviewTitle: a.translate("i18n:ui_theme_preview_title"), ThemePreviewTexts: previewTexts,
		ThemePreviewSubs: previewSubtitles, ThemePreviewOpen: a.translate("i18n:ui_theme_preview_open"),
		Permissions: permissions, PermissionLoading: a.onboardingLoading,
		ChoiceKind: a.onboardingChoice, ChoiceValue: choiceValue, ChoiceAnchor: a.onboardingChoiceAnchor, Choices: choices,
		Window: a.onboardingNativeWindow(), Theme: theme,
		OnDrag: func() {
			if window := a.onboardingNativeWindow(); window != nil {
				_ = window.StartDragging()
			}
		},
		NextDisabled: nextDisabled,
		OnStep:       a.selectOnboardingStep, OnBack: func() { a.selectOnboardingStep(active - 1) }, OnNext: func() { a.selectOnboardingStep(active + 1) },
		OnFinish: a.finishOnboarding, OnToggleGlance: a.setOnboardingGlanceEnabled,
		OnRecordHotkey:      func() { a.focusOnboardingHotkey(0); a.recordHotkeySettingsField(0) },
		OnRecordQueryHotkey: a.recordOnboardingQueryHotkey,
		OnToggleQueryHotkey: a.toggleOnboardingQueryHotkey,
		OnInstallPlugin:     a.installOnboardingPlugin,
		OnSelectTheme:       a.selectOnboardingTheme,
		OnOpenChoice:        a.openOnboardingChoice, OnSelectChoice: a.selectOnboardingChoice, OnPermission: a.openOnboardingPermission,
	})
}

func (a *App) onboardingSteps() []launcherview.OnboardingStep {
	specs := []onboardingStepSpec{
		{"welcome", "onboarding_welcome_title", woxui.Color{R: 45, G: 212, B: 191, A: 255}},
		{"mainHotkey", "onboarding_main_hotkey_title", woxui.Color{R: 249, G: 115, B: 22, A: 255}},
	}
	if runtime.GOOS == "darwin" {
		specs = append(specs, onboardingStepSpec{"permissions", "onboarding_permissions_title", woxui.Color{R: 249, G: 115, B: 22, A: 255}})
	}
	specs = append(specs,
		onboardingStepSpec{"glance", "onboarding_glance_title", woxui.Color{R: 250, G: 204, B: 21, A: 255}},
		onboardingStepSpec{"queryHotkeys", "onboarding_query_hotkeys_title", woxui.Color{R: 244, G: 63, B: 94, A: 255}},
	)
	specs = append(specs,
		onboardingStepSpec{"wpmInstall", "onboarding_wpm_install_title", woxui.Color{R: 56, G: 189, B: 248, A: 255}},
		onboardingStepSpec{"themeInstall", "onboarding_theme_install_title", woxui.Color{R: 232, G: 121, B: 249, A: 255}},
		onboardingStepSpec{"finish", "onboarding_finish_title", woxui.Color{R: 45, G: 212, B: 191, A: 255}},
	)
	steps := make([]launcherview.OnboardingStep, len(specs))
	for index, spec := range specs {
		steps[index] = launcherview.OnboardingStep{ID: spec.id, Title: a.translate("i18n:" + spec.key), Accent: spec.accent}
	}
	return steps
}

func (a *App) onboardingLabels() map[string]string {
	labels := map[string]string{
		"welcome.body":                  a.translate("i18n:onboarding_welcome_description"),
		"welcome.apps":                  a.translate("i18n:onboarding_welcome_apps"),
		"welcome.files":                 a.translate("i18n:onboarding_welcome_files"),
		"welcome.plugins":               a.translate("i18n:onboarding_welcome_plugins"),
		"welcome.ai":                    a.translate("i18n:onboarding_welcome_ai"),
		"welcome.hint":                  a.translate("i18n:onboarding_welcome_hint"),
		"permissions.body":              a.translate("i18n:onboarding_permissions_description"),
		"mainHotkey.body":               a.translate("i18n:onboarding_main_hotkey_description"),
		"selectionHotkey.body":          a.translate("i18n:onboarding_selection_hotkey_description"),
		"glance.body":                   a.translate("i18n:onboarding_glance_description"),
		"queryHotkeys.body":             a.translate("i18n:onboarding_query_hotkeys_body"),
		"queryHotkeys.checking":         a.translate("i18n:onboarding_query_hotkeys_checking"),
		"queryHotkeys.configured":       a.translate("i18n:onboarding_query_hotkeys_configured"),
		"queryHotkeys.notConfigured":    a.translate("i18n:onboarding_query_hotkeys_not_configured"),
		"queryHotkeys.status.title":     a.translate("i18n:onboarding_query_hotkeys_status_title"),
		"queryHotkeys.status.body":      a.translate("i18n:onboarding_query_hotkeys_status_body"),
		"queryHotkeys.clipboard":        a.translate("i18n:onboarding_query_hotkeys_clipboard"),
		"queryHotkeys.shortcut":         a.translate("i18n:onboarding_query_hotkeys_shortcut"),
		"plugins.loading":               a.translate("i18n:onboarding_plugins_loading"),
		"plugins.install":               a.translate("i18n:ui_plugin_install"),
		"plugins.installing":            a.translate("i18n:ui_plugin_installing"),
		"plugins.installed":             a.translate("i18n:onboarding_plugin_installed"),
		"plugins.more":                  a.translate("i18n:onboarding_plugins_more"),
		"theme.loading":                 a.translate("i18n:onboarding_theme_loading"),
		"queryShortcuts.body":           a.translate("i18n:onboarding_query_shortcuts_body"),
		"queryShortcuts.title":          a.translate("i18n:ui_query_shortcuts"),
		"trayQueries.body":              a.translate("i18n:onboarding_tray_queries_body"),
		"wpmInstall.body":               a.translate("i18n:onboarding_wpm_install_body"),
		"themeInstall.body":             a.translate("i18n:onboarding_theme_install_body"),
		"finish.body":                   a.translate("i18n:onboarding_finish_card_body"),
		"finish.hotkey":                 a.translate("i18n:onboarding_finish_summary_hotkey"),
		"finish.glance":                 a.translate("i18n:onboarding_finish_summary_glance"),
		"finish.plugins":                a.translate("i18n:onboarding_finish_summary_plugins"),
		"finish.hint":                   a.translate("i18n:onboarding_finish_summary_hint"),
		"welcome.query":                 "wpm install everything",
		"permissions.query":             "permissions",
		"mainHotkey.query":              "apps",
		"selectionHotkey.query":         "Roadmap.md",
		"glance.query":                  "wox",
		"queryHotkeys.query":            "github repo",
		"trayQueries.query":             "weather",
		"wpmInstall.query":              "wpm install",
		"themeInstall.query":            "theme",
		"finish.query":                  "setting",
		"preview.more":                  a.translate("i18n:onboarding_action_panel_title"),
		"demo.concept.title":            a.translate("i18n:onboarding_query_concept_title"),
		"demo.concept.trigger":          a.translate("i18n:onboarding_query_concept_trigger_keyword"),
		"demo.concept.command":          a.translate("i18n:onboarding_query_concept_command"),
		"demo.concept.search":           a.translate("i18n:onboarding_query_concept_search_term"),
		"demo.concept.optional":         a.translate("i18n:onboarding_query_concept_optional"),
		"demo.concept.result1.subtitle": a.translate("i18n:onboarding_query_concept_result1_subtitle"),
		"demo.concept.result2.subtitle": a.translate("i18n:onboarding_query_concept_result2_subtitle"),
		"demo.concept.result3.subtitle": a.translate("i18n:onboarding_query_concept_result3_subtitle"),
		"demo.install":                  a.translate("i18n:plugin_wpm_install"),
		"demo.installing":               a.translate("i18n:plugin_wpm_installing"),
		"demo.installed":                a.translate("i18n:plugin_wpm_start_using"),
		"demo.apply":                    a.translate("i18n:ui_setting_theme_apply"),
		"demo.actions":                  a.translate("i18n:onboarding_action_panel_title"),
		"demo.action.copy":              a.translate("i18n:onboarding_action_panel_copy"),
		"demo.action.more":              a.translate("i18n:onboarding_action_panel_more"),
		"demo.selection.preview":        a.translate("i18n:selection_preview"),
		"demo.selection.copy_path":      a.translate("i18n:selection_copy_path"),
		"demo.selection.open_folder":    a.translate("i18n:selection_open_containing_folder"),
		"demo.glance.provider":          a.translate("i18n:onboarding_glance_sample_provider"),
		"demo.glance.value":             a.translate("i18n:onboarding_glance_sample_time"),
		"demo.permission.ready":         a.translate("i18n:onboarding_permission_ready"),
		"demo.finish.title":             a.translate("i18n:onboarding_finish_card_title"),
		"demo.finish.badge":             a.translate("i18n:onboarding_finish_badge"),
		"demo.finish.settings":          a.translate("i18n:plugin_sys_open_wox_settings"),
		"demo.finish.system_settings":   a.translate("i18n:plugin_sys_open_system_settings"),
	}
	for _, step := range []string{"welcome", "permissions", "mainHotkey", "selectionHotkey", "glance", "queryHotkeys", "trayQueries", "wpmInstall", "themeInstall", "finish"} {
		labels[step+".preview"] = labels[step+".body"]
	}
	return labels
}

func (a *App) selectOnboardingStep(index int) {
	steps := a.onboardingSteps()
	if index < 0 || index >= len(steps) {
		return
	}
	if index > a.onboardingStep && steps[a.onboardingStep].ID == "mainHotkey" {
		data := a.generalSettings.Data()
		presentation := a.hotkeyRecordingFieldStatus("hotkey-settings", 0)
		if (presentation.Active && presentation.Error) || (data.MainHotkeyRegistrationFailed && (!presentation.Active || presentation.Value == data.MainHotkey)) {
			return
		}
	}
	if index > a.onboardingStep && steps[a.onboardingStep].ID == "queryHotkeys" && (a.onboardingQueryHotkey == nil || (a.onboardingQueryHotkey.selected && !a.onboardingQueryHotkey.ready)) {
		return
	}
	if index != a.onboardingStep {
		a.stopHotkeyRecording()
	}
	a.onboardingStep = index
	a.onboardingChoice = ""
	a.onboardingChoiceAnchor = woxui.Rect{}
	if form := a.hotkeySettings.Form(); form != nil {
		form.active = true
		if steps[index].ID == "mainHotkey" {
			setFormFieldsFocusLocked(form, 0)
		} else if steps[index].ID == "selectionHotkey" && len(form.definitions) > 1 {
			setFormFieldsFocusLocked(form, 1)
		}
	}
	if steps[index].ID == "permissions" && runtime.GOOS == "darwin" {
		util.Go(a.lifecycleCtx, "refresh onboarding permission status", a.loadOnboardingPermissionStatus)
	}
	if steps[index].ID == "queryHotkeys" {
		a.prepareOnboardingQueryHotkey()
	}
	if steps[index].ID == "wpmInstall" {
		a.loadOnboardingPlugins()
	}
	if steps[index].ID == "themeInstall" {
		a.loadOnboardingThemes()
	}
	a.invalidateOnboardingWindow()
}

func defaultOnboardingQueryHotkey() string {
	if runtime.GOOS == "darwin" {
		return "Cmd+Shift+V"
	}
	return "Ctrl+Shift+V"
}

func (a *App) prepareOnboardingQueryHotkey() {
	if a.onboardingQueryHotkey != nil {
		return
	}
	hotkey := defaultOnboardingQueryHotkey()
	form := newFormFieldsState([]formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "Hotkey"}}}, map[string]string{"Hotkey": hotkey}, true)
	state := &onboardingQueryHotkeyState{form: form, selected: true}
	a.onboardingQueryHotkey = state
	for _, item := range a.generalSettings.Data().QueryHotkeys {
		if !item.Disabled && strings.EqualFold(strings.TrimSpace(item.Query), "cb") {
			state.form.values["Hotkey"] = item.Hotkey
			state.ready = true
			state.saved = true
			return
		}
	}
	util.Go(a.lifecycleCtx, "check onboarding query hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		availability, err := a.services.CheckHotkeyAvailability(ctx, a.sessionID, hotkey)
		cancel()
		_ = a.runOnUI("apply onboarding query hotkey availability", func() {
			if a.onboardingQueryHotkey != state {
				return
			}
			if err != nil {
				state.error = err.Error()
			} else if !availability.Available {
				state.error = a.hotkeyConflictMessage(availability.ConflictType, availability.ConflictValue)
			} else {
				a.saveOnboardingQueryHotkey(hotkey)
			}
			a.invalidateOnboardingWindow()
		})
	})
}

func (a *App) recordOnboardingQueryHotkey() {
	state := a.onboardingQueryHotkey
	if state == nil || state.saving {
		return
	}
	if !state.selected {
		a.toggleOnboardingQueryHotkey(true)
		return
	}
	state.error = ""
	a.startHotkeyRecording("onboarding-query-hotkey", &state.form, 0, "", defaultHotkeyRecordingKinds)
}

func (a *App) saveOnboardingQueryHotkey(hotkey string) {
	state := a.onboardingQueryHotkey
	if state == nil || state.saving {
		return
	}
	state.saving = true
	state.error = ""
	items := upsertOnboardingQueryHotkey(a.generalSettings.Data().QueryHotkeys, hotkey)
	raw, err := json.Marshal(items)
	if err != nil {
		state.saving = false
		state.error = err.Error()
		return
	}
	util.Go(a.lifecycleCtx, "save onboarding query hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.services.UpdateGeneralSetting(ctx, a.sessionID, "QueryHotkeys", string(raw))
		cancel()
		if err == nil {
			err = a.reloadSettings()
		}
		_ = a.runOnUI("apply onboarding query hotkey", func() {
			if a.onboardingQueryHotkey != state {
				return
			}
			state.saving = false
			if err != nil {
				state.error = err.Error()
			} else {
				state.form.values["Hotkey"] = hotkey
				state.ready = true
				state.selected = true
				state.saved = true
				state.error = ""
			}
			a.invalidateOnboardingWindow()
		})
	})
}

func (a *App) toggleOnboardingQueryHotkey(enabled bool) {
	state := a.onboardingQueryHotkey
	if state == nil || state.saving || state.selected == enabled {
		return
	}
	if enabled {
		a.onboardingQueryHotkey = nil
		a.prepareOnboardingQueryHotkey()
		a.invalidateOnboardingWindow()
		return
	}
	a.stopHotkeyRecording()
	state.selected = false
	state.ready = false
	state.error = ""
	if !state.saved {
		a.invalidateOnboardingWindow()
		return
	}
	state.saving = true
	raw, err := json.Marshal(removeOnboardingQueryHotkey(a.generalSettings.Data().QueryHotkeys))
	if err != nil {
		state.saving = false
		state.selected = true
		state.ready = true
		state.error = err.Error()
		return
	}
	util.Go(a.lifecycleCtx, "remove onboarding query hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.services.UpdateGeneralSetting(ctx, a.sessionID, "QueryHotkeys", string(raw))
		cancel()
		if err == nil {
			err = a.reloadSettings()
		}
		_ = a.runOnUI("apply removed onboarding query hotkey", func() {
			if a.onboardingQueryHotkey != state {
				return
			}
			state.saving = false
			if err != nil {
				state.selected = true
				state.ready = true
				state.error = err.Error()
			} else {
				state.saved = false
			}
			a.invalidateOnboardingWindow()
		})
	})
}

// upsertOnboardingQueryHotkey preserves existing query hotkeys while keeping one enabled clipboard shortcut.
func upsertOnboardingQueryHotkey(current []queryHotkeySetting, hotkey string) []queryHotkeySetting {
	items := append([]queryHotkeySetting(nil), current...)
	updated := false
	for index, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Query), "cb") {
			items[index].Hotkey = hotkey
			items[index].Disabled = false
			updated = true
			break
		}
	}
	if !updated {
		items = append(items, queryHotkeySetting{Hotkey: hotkey, Query: "cb "})
	}
	return items
}

func removeOnboardingQueryHotkey(current []queryHotkeySetting) []queryHotkeySetting {
	items := make([]queryHotkeySetting, 0, len(current))
	for _, item := range current {
		if !strings.EqualFold(strings.TrimSpace(item.Query), "cb") {
			items = append(items, item)
		}
	}
	return items
}

// onboardingRecommendedPlugins selects the curated store entries in their intended display order.
func onboardingRecommendedPlugins(plugins []pluginSettingsPlugin, goos string) []pluginSettingsPlugin {
	ids := []string{
		"6dd42f91-009d-4d14-909c-97f25454eea7",
	}
	if goos == "windows" {
		ids = append(ids, "6987b7b1-89da-41ef-bab3-d1ba2e3daba0")
	}
	ids = append(ids,
		"0057ebd4-1a85-4653-8bfa-d51557c0c7a1",
		"8b8a1b35-3d9e-4d7d-9f2e-3b1d0b7f9e10",
	)
	byID := make(map[string]pluginSettingsPlugin, len(plugins))
	for _, plugin := range plugins {
		byID[plugin.ID] = plugin
	}
	recommended := make([]pluginSettingsPlugin, 0, len(ids))
	for _, id := range ids {
		if plugin, ok := byID[id]; ok {
			recommended = append(recommended, plugin)
		}
	}
	return recommended
}

// loadOnboardingPlugins loads real localized metadata from the plugin store once per onboarding session.
func (a *App) loadOnboardingPlugins() {
	if a.onboardingPlugins.loading || len(a.onboardingPlugins.plugins) > 0 {
		return
	}
	a.onboardingPlugins.loading = true
	a.onboardingPlugins.error = ""
	a.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "load onboarding plugins", func() {
		plugins, err := loadPluginSettingsPlugins(context.Background(), a.services, a.sessionID, true)
		_ = a.runOnUI("apply onboarding plugins", func() {
			a.onboardingPlugins.loading = false
			if err != nil {
				a.onboardingPlugins.error = err.Error()
			} else {
				a.onboardingPlugins.plugins = onboardingRecommendedPlugins(plugins, runtime.GOOS)
				a.onboardingPlugins.error = ""
			}
			a.invalidateOnboardingWindow()
		})
	})
}

// installOnboardingPlugin installs one recommendation through the shared plugin operation service.
func (a *App) installOnboardingPlugin(pluginID string) {
	if a.onboardingPlugins.operationID != "" {
		return
	}
	found := false
	for _, plugin := range a.onboardingPlugins.plugins {
		if plugin.ID == pluginID && !plugin.IsInstalled {
			found = true
			break
		}
	}
	if !found {
		return
	}
	a.onboardingPlugins.operationID = pluginID
	a.onboardingPlugins.error = ""
	a.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "install onboarding plugin", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		operationErr := a.services.OperatePlugin(ctx, a.sessionID, pluginID, contract.PluginOperationInstall)
		cancel()
		var plugins []pluginSettingsPlugin
		var refreshErr error
		if operationErr == nil {
			plugins, refreshErr = loadPluginSettingsPlugins(context.Background(), a.services, a.sessionID, true)
		}
		_ = a.runOnUI("apply onboarding plugin install", func() {
			a.onboardingPlugins.operationID = ""
			if operationErr != nil {
				a.onboardingPlugins.error = operationErr.Error()
			} else {
				if refreshErr == nil {
					a.onboardingPlugins.plugins = onboardingRecommendedPlugins(plugins, runtime.GOOS)
					a.onboardingPlugins.error = ""
				} else {
					for index := range a.onboardingPlugins.plugins {
						if a.onboardingPlugins.plugins[index].ID == pluginID {
							a.onboardingPlugins.plugins[index].IsInstalled = true
						}
					}
					a.onboardingPlugins.error = refreshErr.Error()
				}
				a.pluginSettings.invalidateCachedPlugins(true)
				a.pluginSettings.invalidateCachedPlugins(false)
				a.settingsSearch.SetLoaded(false)
			}
			a.invalidateOnboardingWindow()
		})
	})
}

// onboardingSystemThemes keeps the four bundled themes in the onboarding presentation order.
func onboardingSystemThemes(themes []themeSettingsTheme) []themeSettingsTheme {
	ids := []string{
		onboardingGlassDarkID,
		"53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03",
		"92dc0ea7-a52f-4b0a-9f0d-7cb36a634860",
		"532238bc-6eda-4011-a080-c365b67486fc",
	}
	byID := make(map[string]themeSettingsTheme, len(themes))
	for _, theme := range themes {
		byID[theme.ID] = theme
	}
	result := make([]themeSettingsTheme, 0, len(ids))
	for _, id := range ids {
		if theme, ok := byID[id]; ok {
			result = append(result, theme)
		}
	}
	return result
}

// loadOnboardingThemes loads installed system themes through the shared theme catalog.
func (a *App) loadOnboardingThemes() {
	if a.onboardingTheme.loading || len(onboardingSystemThemes(a.themeSettings.Themes())) == 4 {
		return
	}
	a.onboardingTheme.loading = true
	a.onboardingTheme.error = ""
	a.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "load onboarding themes", func() {
		err := a.reloadThemes("installed", onboardingGlassDarkID)
		_ = a.runOnUI("apply onboarding themes", func() {
			a.onboardingTheme.loading = false
			if err != nil {
				a.onboardingTheme.error = err.Error()
			} else {
				a.onboardingTheme.error = ""
				themes := onboardingSystemThemes(a.themeSettings.Themes())
				found := false
				for _, theme := range themes {
					found = found || theme.ID == a.onboardingTheme.selectedID
				}
				if !found && len(themes) > 0 {
					a.onboardingTheme.selectedID = themes[0].ID
				}
			}
			a.invalidateOnboardingWindow()
		})
	})
}

// selectOnboardingTheme applies a bundled theme without changing the onboarding step.
func (a *App) selectOnboardingTheme(themeID string) {
	if a.onboardingTheme.loading || a.onboardingTheme.applying {
		return
	}
	for _, theme := range onboardingSystemThemes(a.themeSettings.Themes()) {
		if theme.ID == themeID {
			a.onboardingTheme.selectedID = themeID
			a.onboardingTheme.error = ""
			a.applyOnboardingTheme()
			return
		}
	}
}

// applyOnboardingTheme applies the selected bundled theme immediately.
func (a *App) applyOnboardingTheme() {
	themeID := a.onboardingTheme.selectedID
	found := false
	for _, theme := range onboardingSystemThemes(a.themeSettings.Themes()) {
		if theme.ID == themeID {
			found = true
			break
		}
	}
	if !found {
		return
	}
	if a.currentThemeID() == themeID {
		a.invalidateOnboardingWindow()
		return
	}
	a.onboardingTheme.applying = true
	a.onboardingTheme.error = ""
	a.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "apply onboarding theme", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := a.services.OperateTheme(ctx, a.sessionID, themeID, contract.ThemeOperationApply)
		cancel()
		if err == nil {
			err = a.reloadTheme()
		}
		_ = a.runOnUI("apply onboarding theme result", func() {
			a.onboardingTheme.applying = false
			if err != nil {
				a.onboardingTheme.error = err.Error()
			} else {
				a.generalSettings.Update(func(data *settingsData) { data.ThemeID = themeID })
				a.onboardingTheme.error = ""
			}
			a.invalidateOnboardingWindow()
		})
	})
}

func (a *App) focusOnboardingHotkey(index int) {
	form := a.hotkeySettings.Form()
	if form == nil || index < 0 || index >= len(form.definitions) {
		return
	}
	form.active = true
	setFormFieldsFocusLocked(form, index)
	a.hotkeySettings.SetFocused(true)
	a.invalidateOnboardingWindow()
}

func (a *App) openOnboardingChoice(kind string) {
	if kind != "language" && kind != "glance" {
		return
	}
	if a.onboardingChoice == kind {
		a.onboardingChoice = ""
		a.onboardingChoiceAnchor = woxui.Rect{}
	} else {
		a.onboardingChoice = kind
		id := "onboarding-language"
		if kind == "glance" {
			id = "onboarding-glance-choice"
		}
		a.onboardingChoiceAnchor = woxui.Rect{}
		if a.onboardingHost != nil {
			a.onboardingChoiceAnchor, _ = a.onboardingHost.BoundsForKey(woxwidget.Key(id))
		}
	}
	a.invalidateOnboardingWindow()
}

func (a *App) onboardingChoices(snapshot settingsSnapshot, kind string, imageScale float32) []launcherview.OnboardingChoice {
	if kind == "language" {
		choices := make([]launcherview.OnboardingChoice, len(snapshot.general.Languages))
		for index, choice := range snapshot.general.Languages {
			choices[index] = launcherview.OnboardingChoice{Value: choice.value, Label: choice.label}
		}
		return choices
	}
	if kind == "glance" {
		choices := make([]launcherview.OnboardingChoice, 0, len(snapshot.appearance.GlanceCatalog))
		for _, item := range snapshot.appearance.GlanceCatalog {
			label := firstNonEmpty(item.Name, item.Ref.GlanceID)
			source := item.Icon
			trailing := ""
			if item.Preview != nil {
				trailing = item.Preview.Text
				if item.Preview.Icon.ImageData != "" {
					source = item.Preview.Icon
				}
			}
			choices = append(choices, launcherview.OnboardingChoice{
				Value: glanceRefJSON(item.Ref), Label: label, Leading: a.imageForTint(source, &snapshot.palette.resultTitle, physicalImageSize(18, imageScale)), Trailing: trailing,
			})
		}
		return choices
	}
	return nil
}

func (a *App) selectOnboardingChoice(value string) {
	kind := a.onboardingChoice
	a.onboardingChoice = ""
	a.onboardingChoiceAnchor = woxui.Rect{}
	key := ""
	switch kind {
	case "language":
		key = "LangCode"
	case "glance":
		key = "PrimaryGlance"
	default:
		return
	}
	a.saveOnboardingSetting(key, value)
}

func (a *App) setOnboardingGlanceEnabled(enabled bool) {
	a.saveOnboardingSetting("EnableGlance", strconv.FormatBool(enabled))
}

func (a *App) saveOnboardingSetting(key, value string) {
	util.Go(a.lifecycleCtx, "save onboarding setting", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.services.UpdateGeneralSetting(ctx, a.sessionID, key, value)
		cancel()
		if err == nil {
			err = a.reloadSettings()
		}
		if err == nil && key == "LangCode" {
			err = a.reloadTranslations()
		}
		_ = a.runOnUI("apply onboarding setting", func() {
			if err != nil {
				a.onboardingError = err.Error()
			} else {
				a.onboardingError = ""
			}
			if form := a.hotkeySettings.Form(); form != nil {
				form.active = true
			}
			a.invalidateOnboardingWindow()
		})
	})
}

func (a *App) loadOnboardingPermissionStatus() {
	a.loadOnboardingPermissionStatusWithLoading(true)
}

// loadOnboardingPermissionStatusWithLoading probes TCC without flashing the authorize button on background refreshes.
func (a *App) loadOnboardingPermissionStatusWithLoading(showLoading bool) {
	if showLoading {
		_ = a.runOnUI("start onboarding permission check", func() {
			a.onboardingLoading = true
			a.invalidateOnboardingWindow()
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	status, err := a.services.MacOSPermissionStatus(ctx, a.sessionID)
	cancel()
	if err == nil {
		if reconcileErr := keyboard.ReconcileRawKeyListenerAccessWithPermissionStatus(status.Accessibility == "granted"); reconcileErr != nil {
			util.GetLogger().Warn(context.Background(), "failed to reconcile macOS raw keyboard access: "+reconcileErr.Error())
		}
	}
	var completedPermissionFlow permission.MacOSPermissionType
	_ = a.runOnUI("apply onboarding permission status", func() {
		a.onboardingLoading = false
		if err != nil {
			a.onboardingError = err.Error()
		} else {
			a.onboardingPermission = status
			a.onboardingError = ""
			if a.permissionFlowHost != nil {
				permissionType := a.permissionFlowHost.permissionType
				if macOSPermissionGranted(permissionType, status) {
					completedPermissionFlow = permissionType
				}
			}
		}
		a.invalidateOnboardingWindow()
	})
	if completedPermissionFlow != "" {
		macospermission.Complete(completedPermissionFlow)
	}
}

func (a *App) openOnboardingPermission(permissionType string) {
	util.Go(a.lifecycleCtx, "open onboarding permission", func() {
		if err := a.openMacOSPermissionFlow(permissionType); err != nil {
			_ = a.runOnUI("show onboarding permission error", func() {
				a.onboardingError = err.Error()
				a.invalidateOnboardingWindow()
			})
		}
	})
}

func (a *App) notifyOnboardingViewChanged(inOnboardingView bool) error {
	ctx, cancel := a.lifecycleContext()
	defer cancel()
	return a.services.OnboardingViewChanged(ctx, a.sessionID, inOnboardingView)
}

func (a *App) onOnboardingKey(event woxui.KeyEvent) bool {
	if !a.onboardingOpen {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		a.onboardingChoice = ""
		a.onboardingChoiceAnchor = woxui.Rect{}
		a.invalidateOnboardingWindow()
	case woxui.KeyArrowLeft:
		a.selectOnboardingStep(a.onboardingStep - 1)
	case woxui.KeyArrowRight:
		a.selectOnboardingStep(a.onboardingStep + 1)
	default:
		return false
	}
	return true
}

func (a *App) onOnboardingWindowFocus(event woxui.FocusEvent) {
	if event.Active && a.onboardingOpen && runtime.GOOS == "darwin" {
		util.Go(a.lifecycleCtx, "refresh focused onboarding permission status", a.loadOnboardingPermissionStatus)
	}
}
