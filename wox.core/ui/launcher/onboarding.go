package launcher

import (
	"context"
	"encoding/json"
	"fmt"
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
	onboardingGlassID      = "44a933d5-e6de-4c1f-8ee5-b2305c6abdf3"
)

// Progress and illustrations use a brighter accent; management controls use Settings colors.
var onboardingAccentColor = woxui.Color{R: 20, G: 184, B: 166, A: 255}

// Only illustrative launcher windows use Glass; management chrome never reads this theme.
var onboardingPreviewTheme = func() woxcomponent.Theme {
	data, err := resource.ThemeFS.ReadFile("themes/glass.json")
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
	loading  bool
	applying bool
	error    string
}

// openOnboarding presents the first-run guide in its dedicated window.
func (a *App) openOnboarding() error {
	if err := a.reloadSettingsForWindow(); err != nil {
		return err
	}
	wasSettings := false
	var settingsView *woxui.ManagedWindow
	installations := detectOnboardingMigrations(context.Background())
	if err := a.runOnUI("prepare onboarding", func() {
		wasSettings = a.settingsWindow.settingsOpen
		a.settingsWindow.settingsOpen = false
		a.onboarding.onboardingOpen = true
		a.onboarding.onboardingStep = 0
		a.onboarding.onboardingChoice = ""
		a.onboarding.onboardingChoiceAnchor = woxui.Rect{}
		a.onboarding.onboardingError = ""
		a.onboarding.onboardingQueryHotkey = nil
		a.onboarding.onboardingPlugins = onboardingPluginState{}
		a.onboarding.onboardingTheme = onboardingThemeState{}
		a.onboarding.onboardingMigration = newOnboardingMigrationState(installations)
		a.stopHotkeyRecording()
		settingsView = a.settingsWindow.settingsView
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
	onboardingView, err := a.onboarding.ensureOnboardingWindow()
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
				a.onboarding.onboardingError = err.Error()
				a.onboarding.invalidateOnboardingWindow()
			})
			return
		}
		show := fromCoreShowOptions(a.services.LauncherShowOptions(ctx, a.sessionID))
		cancel()
		var onboardingView *woxui.ManagedWindow
		_ = a.runOnUI("leave onboarding", func() {
			a.releaseOnboardingMigrationSession()
			a.onboarding.onboardingOpen = false
			a.onboarding.onboardingChoice = ""
			a.onboarding.onboardingChoiceAnchor = woxui.Rect{}
			a.stopHotkeyRecording()
			a.releaseDemoWallpaperLocked()
			onboardingView = a.onboarding.onboardingView
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
	snapshot.palette.DensityScale = a.appearance.densityMetrics.normalized().scale
	steps := a.onboardingSteps()
	for index := range steps {
		steps[index].Accent = onboardingAccentColor
	}
	active := min(max(0, a.onboarding.onboardingStep), len(steps)-1)
	step := steps[active]
	labels := a.onboardingLabels()
	labels["title"] = a.language.translate("i18n:onboarding_title")
	labels["subtitle"] = a.language.translate("i18n:onboarding_subtitle")
	labels["back"] = a.language.translate("i18n:onboarding_back")
	labels["next"] = a.language.translate("i18n:onboarding_next")
	labels["finish"] = a.language.translate("i18n:onboarding_finish")
	labels["language"] = a.language.translate("i18n:ui_lang")
	labels["permission.authorize"] = a.language.translate("i18n:onboarding_permission_authorize")
	labels["permission.checking"] = a.language.translate("i18n:onboarding_permission_checking")
	labels["glance.enable"] = a.language.translate("i18n:onboarding_glance_enable")
	labels["glance.enable.body"] = a.language.translate("i18n:onboarding_glance_enable_tips")
	labels["glance.primary"] = a.language.translate("i18n:ui_glance_primary")
	labels["hotkey.available"] = a.language.translate("i18n:onboarding_hotkey_available")
	labels["hotkey.change"] = a.language.translate("i18n:onboarding_hotkey_change")
	labels["hotkey.preview"] = a.language.translate("i18n:onboarding_hotkey_preview")
	if a.onboarding.onboardingError != "" {
		labels[step.ID+".body"] = a.onboarding.onboardingError
	}
	if step.ID == "migrate" && a.onboarding.onboardingMigration.choosing && len(a.onboarding.onboardingMigration.installations) > 1 {
		labels["migrate.body"] = labels["migrate.choose"]
	}

	choices := a.onboardingChoices(snapshot, a.onboarding.onboardingChoice, frame.Scale)
	choiceValue := ""
	if a.onboarding.onboardingChoice == "language" {
		choiceValue = snapshot.general.Data.LangCode
	}
	language := snapshot.general.Data.LangCode
	for _, choice := range snapshot.general.Languages {
		if choice.value == language {
			language = choice.label
			break
		}
	}
	glanceLabel := a.language.translate("i18n:onboarding_glance_sample_time")
	glanceValue := a.language.translate("i18n:onboarding_glance_sample_value")
	var glanceIcon *woxui.Image
	currentGlance := snapshot.general.Data.PrimaryGlance
	if a.onboarding.onboardingChoice == "glance" {
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
			glanceIcon = a.imageForTint(source, &snapshot.palette.Text, physicalImageSize(18, frame.Scale))
			break
		}
	}
	permissions := []launcherview.OnboardingPermission{
		{
			ID: "accessibility", Title: a.language.translate("i18n:onboarding_permission_accessibility_title"),
			Description: a.language.translate("i18n:onboarding_permission_accessibility_body"), Ready: a.onboarding.onboardingPermission.Accessibility == "granted",
		},
		{
			ID: "fullDiskAccess", Title: a.language.translate("i18n:onboarding_permission_disk_title"),
			Description: a.language.translate("i18n:onboarding_permission_disk_body"), Ready: a.onboarding.onboardingPermission.FullDiskAccess == "granted",
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
			hotkeyStatus = a.language.translate(firstNonEmpty(snapshot.general.Data.MainHotkeyRegistrationError, "i18n:ui_hotkey_registration_failed"))
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
	if state := a.onboarding.onboardingQueryHotkey; state != nil {
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
	plugins := make([]launcherview.OnboardingPlugin, len(a.onboarding.onboardingPlugins.plugins))
	for index, plugin := range a.onboarding.onboardingPlugins.plugins {
		plugins[index] = launcherview.OnboardingPlugin{
			ID: plugin.ID, Name: plugin.Name, Description: plugin.Description,
			Icon: a.imageForSurface(plugin.Icon, physicalImageSize(28, frame.Scale), snapshot.palette.Background), Installed: plugin.IsInstalled,
			Installing: a.onboarding.onboardingPlugins.operationID == plugin.ID,
			Disabled:   a.onboarding.onboardingPlugins.operationID != "",
		}
	}
	themes := onboardingThemeItems(snapshot.theme.Themes, snapshot.general.Data.ThemeID)
	previewTexts := []string{a.language.translate("i18n:ui_theme_preview_text_1"), a.language.translate("i18n:ui_theme_preview_text_2")}
	previewSubtitles := []string{
		strings.ReplaceAll(a.language.translate("i18n:ui_theme_preview_subtitle"), "{index}", "1"),
		strings.ReplaceAll(a.language.translate("i18n:ui_theme_preview_subtitle"), "{index}", "2"),
	}
	if step.ID == "themeInstall" {
		nextDisabled = a.onboarding.onboardingTheme.loading || len(themes) == 0
	}
	migrationSources, migrationCategories := a.onboardingMigrationView(frame.Scale, snapshot.palette.Background, labels)
	nextLabel := ""
	skipLabel := ""
	var onSkip func()
	if step.ID == "migrate" {
		if a.onboarding.onboardingMigration.importing || (a.onboarding.onboardingMigration.loading && !a.onboarding.onboardingMigration.loaded && !a.onboarding.onboardingMigration.choosing) {
			nextDisabled = true
		}
		if !a.onboarding.onboardingMigration.done && (a.onboardingMigrationCanImport() || a.onboarding.onboardingMigration.importing) {
			nextLabel = labels["migrate.import"]
			if a.onboarding.onboardingMigration.importing {
				nextLabel = labels["migrate.importing"]
			}
			if !a.onboarding.onboardingMigration.importing {
				skipLabel = labels["migrate.skip"]
				onSkip = func() { a.selectOnboardingStep(active + 1) }
			}
		} else if !a.onboarding.onboardingMigration.done {
			nextLabel = labels["migrate.skip"]
		}
	}
	if step.ID == "mainHotkey" && a.onboarding.onboardingMigration.mainHotkeyPending {
		nextDisabled = true
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
		Plugins: plugins, PluginsLoading: a.onboarding.onboardingPlugins.loading, PluginsError: a.onboarding.onboardingPlugins.error,
		Themes: themes, ThemesLoading: a.onboarding.onboardingTheme.loading, ThemesApplying: a.onboarding.onboardingTheme.applying, ThemesError: a.onboarding.onboardingTheme.error,
		MigrationChoosing: a.onboarding.onboardingMigration.choosing, MigrationSources: migrationSources, MigrationCategories: migrationCategories,
		MigrationLoading: a.onboarding.onboardingMigration.loading, MigrationImporting: a.onboarding.onboardingMigration.importing, MigrationDone: a.onboarding.onboardingMigration.done,
		MigrationActiveID: a.onboarding.onboardingMigration.activeID, MigrationError: a.migrationExportMessage(a.onboarding.onboardingMigration.error),
		MigrationUnlock:     a.migrationNeedsUnlock() && !a.onboarding.onboardingMigration.choosing,
		MigrationExportPath: a.onboarding.onboardingMigration.exportPath, MigrationPassphrase: a.migrationPassphraseController(),
		ThemePreviewTitle: a.language.translate("i18n:ui_theme_preview_title"), ThemePreviewTexts: previewTexts,
		ThemePreviewSubs: previewSubtitles, ThemePreviewOpen: a.language.translate("i18n:ui_theme_preview_open"),
		Permissions: permissions, PermissionLoading: a.onboarding.onboardingLoading,
		ChoiceKind: a.onboarding.onboardingChoice, ChoiceValue: choiceValue, ChoiceAnchor: a.onboarding.onboardingChoiceAnchor, Choices: choices,
		Window: a.onboarding.onboardingNativeWindow(), Theme: snapshot.palette, PreviewTheme: onboardingPreviewTheme,
		OnDrag: func() {
			if window := a.onboarding.onboardingNativeWindow(); window != nil {
				_ = window.StartDragging()
			}
		},
		NextDisabled: nextDisabled, NextLabel: nextLabel, SkipLabel: skipLabel, OnSkip: onSkip,
		OnStep: a.selectOnboardingStep, OnBack: func() { a.selectOnboardingStep(active - 1) }, OnNext: func() {
			if step.ID == "migrate" && !a.onboarding.onboardingMigration.done && a.onboardingMigrationCanImport() {
				a.importOnboardingMigration()
				return
			}
			a.selectOnboardingStep(active + 1)
		},
		OnFinish: a.finishOnboarding, OnToggleGlance: a.setOnboardingGlanceEnabled,
		OnRecordHotkey:          func() { a.focusOnboardingHotkey(0); a.recordHotkeySettingsField(0) },
		OnRecordQueryHotkey:     a.recordOnboardingQueryHotkey,
		OnToggleQueryHotkey:     a.toggleOnboardingQueryHotkey,
		OnInstallPlugin:         a.installOnboardingPlugin,
		OnSelectTheme:           a.selectOnboardingTheme,
		OnChooseMigration:       a.chooseOnboardingMigration,
		OnToggleMigrationItem:   a.toggleOnboardingMigrationItem,
		OnPickMigrationFile:     a.pickOnboardingMigrationFile,
		OnUnlockMigration:       a.unlockOnboardingMigration,
		OnToggleMigrationOption: a.toggleOnboardingMigrationOption,
		OnOpenChoice:            a.onboarding.openOnboardingChoice, OnSelectChoice: a.selectOnboardingChoice, OnPermission: a.openOnboardingPermission,
	})
}

func (a *App) onboardingSteps() []launcherview.OnboardingStep {
	// Recording the main hotkey needs Accessibility. On macOS that permission
	// step comes first, otherwise a denied recording disables Continue.
	specs := []onboardingStepSpec{
		{"welcome", "onboarding_welcome_title", woxui.Color{R: 45, G: 212, B: 191, A: 255}},
	}
	if runtime.GOOS == "darwin" {
		specs = append(specs, onboardingStepSpec{"permissions", "onboarding_permissions_title", woxui.Color{R: 249, G: 115, B: 22, A: 255}})
	}
	// Migration is the next page, after permissions on macOS, so the hotkey pages can still adjust what was imported.
	if len(a.onboarding.onboardingMigration.installations) > 0 {
		specs = append(specs, onboardingStepSpec{"migrate", "onboarding_migrate_title", woxui.Color{R: 20, G: 184, B: 166, A: 255}})
	}
	specs = append(specs,
		onboardingStepSpec{"mainHotkey", "onboarding_main_hotkey_title", woxui.Color{R: 249, G: 115, B: 22, A: 255}},
		onboardingStepSpec{"glance", "onboarding_glance_title", woxui.Color{R: 250, G: 204, B: 21, A: 255}},
		onboardingStepSpec{"queryHotkeys", "onboarding_query_hotkeys_title", woxui.Color{R: 244, G: 63, B: 94, A: 255}},
	)
	specs = append(specs,
		onboardingStepSpec{"wpmInstall", "onboarding_wpm_install_title", woxui.Color{R: 56, G: 189, B: 248, A: 255}},
	)
	// The migration page already installed and selected a theme. Showing the bundled picker would replace it.
	if !a.onboarding.onboardingMigration.themeImported {
		specs = append(specs, onboardingStepSpec{"themeInstall", "onboarding_theme_install_title", woxui.Color{R: 232, G: 121, B: 249, A: 255}})
	}
	specs = append(specs, onboardingStepSpec{"finish", "onboarding_finish_title", woxui.Color{R: 45, G: 212, B: 191, A: 255}})
	steps := make([]launcherview.OnboardingStep, len(specs))
	for index, spec := range specs {
		steps[index] = launcherview.OnboardingStep{ID: spec.id, Title: a.language.translate("i18n:" + spec.key), Accent: spec.accent}
	}
	return steps
}

func (a *App) onboardingLabels() map[string]string {
	labels := map[string]string{
		"welcome.body":                  a.language.translate("i18n:onboarding_welcome_description"),
		"welcome.apps":                  a.language.translate("i18n:onboarding_welcome_apps"),
		"welcome.files":                 a.language.translate("i18n:onboarding_welcome_files"),
		"welcome.plugins":               a.language.translate("i18n:onboarding_welcome_plugins"),
		"welcome.ai":                    a.language.translate("i18n:onboarding_welcome_ai"),
		"welcome.hint":                  a.language.translate("i18n:onboarding_welcome_hint"),
		"permissions.body":              a.language.translate("i18n:onboarding_permissions_description"),
		"mainHotkey.body":               a.language.translate("i18n:onboarding_main_hotkey_description"),
		"selectionHotkey.body":          a.language.translate("i18n:onboarding_selection_hotkey_description"),
		"glance.body":                   a.language.translate("i18n:onboarding_glance_description"),
		"queryHotkeys.body":             a.language.translate("i18n:onboarding_query_hotkeys_body"),
		"queryHotkeys.checking":         a.language.translate("i18n:onboarding_query_hotkeys_checking"),
		"queryHotkeys.configured":       a.language.translate("i18n:onboarding_query_hotkeys_configured"),
		"queryHotkeys.notConfigured":    a.language.translate("i18n:onboarding_query_hotkeys_not_configured"),
		"queryHotkeys.status.title":     a.language.translate("i18n:onboarding_query_hotkeys_status_title"),
		"queryHotkeys.status.body":      a.language.translate("i18n:onboarding_query_hotkeys_status_body"),
		"queryHotkeys.clipboard":        a.language.translate("i18n:onboarding_query_hotkeys_clipboard"),
		"queryHotkeys.shortcut":         a.language.translate("i18n:onboarding_query_hotkeys_shortcut"),
		"plugins.loading":               a.language.translate("i18n:onboarding_plugins_loading"),
		"plugins.install":               a.language.translate("i18n:ui_plugin_install"),
		"plugins.installing":            a.language.translate("i18n:ui_plugin_installing"),
		"plugins.installed":             a.language.translate("i18n:onboarding_plugin_installed"),
		"plugins.more":                  a.language.translate("i18n:onboarding_plugins_more"),
		"theme.loading":                 a.language.translate("i18n:onboarding_theme_loading"),
		"queryAliases.body":             a.language.translate("i18n:onboarding_query_shortcuts_body"),
		"queryAliases.title":            a.language.translate("i18n:ui_query_shortcuts"),
		"trayQueries.body":              a.language.translate("i18n:onboarding_tray_queries_body"),
		"wpmInstall.body":               a.language.translate("i18n:onboarding_wpm_install_body"),
		"themeInstall.body":             a.language.translate("i18n:onboarding_theme_install_body"),
		"migrate.body":                  a.language.translate("i18n:onboarding_migrate_body"),
		"migrate.choose":                a.language.translate("i18n:onboarding_migrate_choose_body"),
		"migrate.import":                a.language.translate("i18n:onboarding_migrate_import"),
		"migrate.importing":             a.language.translate("i18n:onboarding_migrate_importing"),
		"migrate.skip":                  a.language.translate("i18n:onboarding_migrate_skip"),
		"migrate.empty":                 a.language.translate("i18n:onboarding_migrate_empty"),
		"migrate.loading":               a.language.translate("i18n:onboarding_migrate_loading"),
		"migrate.imported":              a.language.translate("i18n:onboarding_migrate_status_imported"),
		"migrate.unsupported":           a.language.translate("i18n:onboarding_migrate_status_unsupported"),
		"migrate.copying":               a.language.translate("i18n:onboarding_migrate_status_importing"),
		"migrate.copied":                a.language.translate("i18n:onboarding_migrate_status_done"),
		"migrate.failed":                a.language.translate("i18n:onboarding_migrate_status_failed"),
		"migrate.conflict":              a.language.translate("i18n:onboarding_migrate_status_conflict"),
		"migrate.cancelled":             a.language.translate("i18n:onboarding_migrate_status_cancelled"),
		"migrate.help":                  a.language.translate("i18n:onboarding_migrate_raycast_help"),
		"migrate.file":                  a.language.translate("i18n:onboarding_migrate_raycast_file"),
		"migrate.passphrase":            a.language.translate("i18n:onboarding_migrate_raycast_passphrase"),
		"migrate.continue":              a.language.translate("i18n:onboarding_migrate_raycast_continue"),
		"migrate.unlocking":             a.language.translate("i18n:onboarding_migrate_raycast_unlocking"),
		"migrate.python":                a.language.translate("i18n:onboarding_migrate_python"),
		"finish.body":                   a.language.translate("i18n:onboarding_finish_card_body"),
		"finish.hotkey":                 a.language.translate("i18n:onboarding_finish_summary_hotkey"),
		"finish.glance":                 a.language.translate("i18n:onboarding_finish_summary_glance"),
		"finish.plugins":                a.language.translate("i18n:onboarding_finish_summary_plugins"),
		"finish.hint":                   a.language.translate("i18n:onboarding_finish_summary_hint"),
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
		"preview.more":                  a.language.translate("i18n:onboarding_action_panel_title"),
		"demo.concept.title":            a.language.translate("i18n:onboarding_query_concept_title"),
		"demo.concept.trigger":          a.language.translate("i18n:onboarding_query_concept_trigger_keyword"),
		"demo.concept.command":          a.language.translate("i18n:onboarding_query_concept_command"),
		"demo.concept.search":           a.language.translate("i18n:onboarding_query_concept_search_term"),
		"demo.concept.optional":         a.language.translate("i18n:onboarding_query_concept_optional"),
		"demo.concept.result1.subtitle": a.language.translate("i18n:onboarding_query_concept_result1_subtitle"),
		"demo.concept.result2.subtitle": a.language.translate("i18n:onboarding_query_concept_result2_subtitle"),
		"demo.concept.result3.subtitle": a.language.translate("i18n:onboarding_query_concept_result3_subtitle"),
		"demo.install":                  a.language.translate("i18n:plugin_wpm_install"),
		"demo.installing":               a.language.translate("i18n:plugin_wpm_installing"),
		"demo.installed":                a.language.translate("i18n:plugin_wpm_start_using"),
		"demo.apply":                    a.language.translate("i18n:ui_setting_theme_apply"),
		"demo.actions":                  a.language.translate("i18n:onboarding_action_panel_title"),
		"demo.action.copy":              a.language.translate("i18n:onboarding_action_panel_copy"),
		"demo.action.more":              a.language.translate("i18n:onboarding_action_panel_more"),
		"demo.selection.preview":        a.language.translate("i18n:selection_preview"),
		"demo.selection.copy_path":      a.language.translate("i18n:selection_copy_path"),
		"demo.selection.open_folder":    a.language.translate("i18n:selection_open_containing_folder"),
		"demo.glance.provider":          a.language.translate("i18n:onboarding_glance_sample_provider"),
		"demo.glance.value":             a.language.translate("i18n:onboarding_glance_sample_time"),
		"demo.permission.ready":         a.language.translate("i18n:onboarding_permission_ready"),
		"demo.finish.title":             a.language.translate("i18n:onboarding_finish_card_title"),
		"demo.finish.badge":             a.language.translate("i18n:onboarding_finish_badge"),
		"demo.finish.settings":          a.language.translate("i18n:plugin_sys_open_wox_settings"),
		"demo.finish.system_settings":   a.language.translate("i18n:plugin_sys_open_system_settings"),
	}
	for _, step := range []string{"welcome", "permissions", "mainHotkey", "selectionHotkey", "glance", "queryHotkeys", "trayQueries", "wpmInstall", "themeInstall", "migrate", "finish"} {
		labels[step+".preview"] = labels[step+".body"]
	}
	return labels
}

func (a *App) selectOnboardingStep(index int) {
	steps := a.onboardingSteps()
	if index < 0 || index >= len(steps) {
		return
	}
	if a.onboarding.onboardingStep >= 0 && a.onboarding.onboardingStep < len(steps) && steps[a.onboarding.onboardingStep].ID == "migrate" && index != a.onboarding.onboardingStep {
		if a.onboarding.onboardingMigration.importing {
			return
		}
		if index == a.onboarding.onboardingStep-1 && a.retreatOnboardingMigration() {
			return
		}
		a.releaseOnboardingMigrationSession()
	}
	if index > a.onboarding.onboardingStep && steps[a.onboarding.onboardingStep].ID == "mainHotkey" {
		data := a.generalSettings.Data()
		presentation := a.hotkeyRecordingFieldStatus("hotkey-settings", 0)
		if (presentation.Active && presentation.Error) || (data.MainHotkeyRegistrationFailed && (!presentation.Active || presentation.Value == data.MainHotkey)) {
			return
		}
	}
	if index > a.onboarding.onboardingStep && steps[a.onboarding.onboardingStep].ID == "queryHotkeys" && (a.onboarding.onboardingQueryHotkey == nil || (a.onboarding.onboardingQueryHotkey.selected && !a.onboarding.onboardingQueryHotkey.ready)) {
		return
	}
	if index != a.onboarding.onboardingStep {
		a.stopHotkeyRecording()
	}
	a.onboarding.onboardingStep = index
	a.onboarding.onboardingChoice = ""
	a.onboarding.onboardingChoiceAnchor = woxui.Rect{}
	if form := a.hotkeySettings.Form(); form != nil {
		form.active = true
		if steps[index].ID == "mainHotkey" {
			setFormFieldsFocusLocked(form, 0)
		} else if steps[index].ID == "selectionHotkey" && len(form.definitions) > 1 {
			setFormFieldsFocusLocked(form, 1)
		}
	}
	if steps[index].ID == "mainHotkey" {
		a.applyStagedMigrationHotkey()
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
	if steps[index].ID == "migrate" {
		a.prepareOnboardingMigration()
	}
	a.onboarding.invalidateOnboardingWindow()
}

func defaultOnboardingQueryHotkey() string {
	if runtime.GOOS == "darwin" {
		return "Cmd+Shift+V"
	}
	return "Ctrl+Shift+V"
}

func (a *App) prepareOnboardingQueryHotkey() {
	if a.onboarding.onboardingQueryHotkey != nil {
		return
	}
	hotkey := defaultOnboardingQueryHotkey()
	form := newFormFieldsState([]formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "Hotkey"}}}, map[string]string{"Hotkey": hotkey}, true)
	state := &onboardingQueryHotkeyState{form: form, selected: true}
	a.onboarding.onboardingQueryHotkey = state
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
			if a.onboarding.onboardingQueryHotkey != state {
				return
			}
			if err != nil {
				state.error = err.Error()
			} else if !availability.Available {
				state.error = a.hotkeyConflictMessage(availability.ConflictType, availability.ConflictValue)
			} else {
				a.saveOnboardingQueryHotkey(hotkey)
			}
			a.onboarding.invalidateOnboardingWindow()
		})
	})
}

func (a *App) recordOnboardingQueryHotkey() {
	state := a.onboarding.onboardingQueryHotkey
	if state == nil || state.saving {
		return
	}
	if !state.selected {
		a.toggleOnboardingQueryHotkey(true)
		return
	}
	state.error = ""
	a.startHotkeyRecording("onboarding-query-hotkey", &state.form, 0, "", defaultHotkeyRecordingKinds, nil)
}

func (a *App) saveOnboardingQueryHotkey(hotkey string) {
	state := a.onboarding.onboardingQueryHotkey
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
			if a.onboarding.onboardingQueryHotkey != state {
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
			a.onboarding.invalidateOnboardingWindow()
		})
	})
}

func (a *App) toggleOnboardingQueryHotkey(enabled bool) {
	state := a.onboarding.onboardingQueryHotkey
	if state == nil || state.saving || state.selected == enabled {
		return
	}
	if enabled {
		a.onboarding.onboardingQueryHotkey = nil
		a.prepareOnboardingQueryHotkey()
		a.onboarding.invalidateOnboardingWindow()
		return
	}
	a.stopHotkeyRecording()
	state.selected = false
	state.ready = false
	state.error = ""
	if !state.saved {
		a.onboarding.invalidateOnboardingWindow()
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
			if a.onboarding.onboardingQueryHotkey != state {
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
			a.onboarding.invalidateOnboardingWindow()
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
	if a.onboarding.onboardingPlugins.loading || len(a.onboarding.onboardingPlugins.plugins) > 0 {
		return
	}
	a.onboarding.onboardingPlugins.loading = true
	a.onboarding.onboardingPlugins.error = ""
	a.onboarding.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "load onboarding plugins", func() {
		plugins, err := loadPluginSettingsPlugins(context.Background(), a.services, a.sessionID, true)
		_ = a.runOnUI("apply onboarding plugins", func() {
			a.onboarding.onboardingPlugins.loading = false
			if err != nil {
				a.onboarding.onboardingPlugins.error = err.Error()
			} else {
				a.onboarding.onboardingPlugins.plugins = onboardingRecommendedPlugins(plugins, runtime.GOOS)
				a.onboarding.onboardingPlugins.error = ""
			}
			a.onboarding.invalidateOnboardingWindow()
		})
	})
}

// installOnboardingPlugin installs one recommendation through the shared plugin operation service.
func (a *App) installOnboardingPlugin(pluginID string) {
	if a.onboarding.onboardingPlugins.operationID != "" {
		return
	}
	found := false
	for _, plugin := range a.onboarding.onboardingPlugins.plugins {
		if plugin.ID == pluginID && !plugin.IsInstalled {
			found = true
			break
		}
	}
	if !found {
		return
	}
	a.onboarding.onboardingPlugins.operationID = pluginID
	a.onboarding.onboardingPlugins.error = ""
	a.onboarding.invalidateOnboardingWindow()
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
			a.onboarding.onboardingPlugins.operationID = ""
			if operationErr != nil {
				a.onboarding.onboardingPlugins.error = operationErr.Error()
			} else {
				if refreshErr == nil {
					a.onboarding.onboardingPlugins.plugins = onboardingRecommendedPlugins(plugins, runtime.GOOS)
					a.onboarding.onboardingPlugins.error = ""
				} else {
					for index := range a.onboarding.onboardingPlugins.plugins {
						if a.onboarding.onboardingPlugins.plugins[index].ID == pluginID {
							a.onboarding.onboardingPlugins.plugins[index].IsInstalled = true
						}
					}
					a.onboarding.onboardingPlugins.error = refreshErr.Error()
				}
				a.pluginSettings.invalidateCachedPlugins(true)
				a.pluginSettings.invalidateCachedPlugins(false)
				a.settingsSearch.SetLoaded(false)
			}
			a.onboarding.invalidateOnboardingWindow()
		})
	})
}

// onboardingSystemThemes keeps the four bundled themes in the onboarding presentation order.
func onboardingSystemThemes(themes []themeSettingsTheme) []themeSettingsTheme {
	ids := []string{
		onboardingGlassID,
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

// onboardingThemeItems derives the checked card from the active setting so imports
// and failed theme changes cannot leave a separate onboarding selection stale.
func onboardingThemeItems(catalog []themeSettingsTheme, currentID string) []launcherview.OnboardingTheme {
	systemThemes := onboardingSystemThemes(catalog)
	themes := make([]launcherview.OnboardingTheme, len(systemThemes))
	for index, theme := range systemThemes {
		preview := paletteForTheme(theme.previewTheme).componentTheme()
		lightPreview := preview
		darkPreview := preview
		if theme.IsAuto {
			lightPreview = themeVariantPreview(catalog, theme.LightThemeID, true)
			darkPreview = themeVariantPreview(catalog, theme.DarkThemeID, false)
		}
		themes[index] = launcherview.OnboardingTheme{
			ID: theme.ID, Name: theme.Name, Selected: theme.ID == currentID, IsAuto: theme.IsAuto,
			PreviewTheme: preview, LightPreviewTheme: lightPreview, DarkPreviewTheme: darkPreview,
		}
	}
	return themes
}

// loadOnboardingThemes loads installed system themes through the shared theme catalog.
func (a *App) loadOnboardingThemes() {
	if a.onboarding.onboardingTheme.loading || len(onboardingSystemThemes(a.themeSettings.Themes())) == 4 {
		return
	}
	a.onboarding.onboardingTheme.loading = true
	a.onboarding.onboardingTheme.error = ""
	themeID := a.currentThemeID()
	a.onboarding.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "load onboarding themes", func() {
		util.GetLogger().Info(a.lifecycleCtx, "loading onboarding system themes")
		err := a.reloadThemes("installed", themeID)
		if applyErr := a.runOnUI("apply onboarding themes", func() {
			a.onboarding.onboardingTheme.loading = false
			if err != nil {
				a.onboarding.onboardingTheme.error = err.Error()
			} else {
				a.onboarding.onboardingTheme.error = ""
			}
			util.GetLogger().Info(a.lifecycleCtx, fmt.Sprintf("onboarding system themes loaded: count=%d error=%v", len(onboardingSystemThemes(a.themeSettings.Themes())), err))
			a.onboarding.invalidateOnboardingWindow()
		}); applyErr != nil {
			util.GetLogger().Error(a.lifecycleCtx, applyErr.Error())
		}
	})
}

// selectOnboardingTheme applies a bundled theme without changing the onboarding step.
func (a *App) selectOnboardingTheme(themeID string) {
	if a.onboarding.onboardingTheme.loading || a.onboarding.onboardingTheme.applying {
		return
	}
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
	a.onboarding.onboardingTheme.error = ""
	if a.currentThemeID() == themeID {
		a.onboarding.invalidateOnboardingWindow()
		return
	}
	a.onboarding.onboardingTheme.applying = true
	a.onboarding.onboardingTheme.error = ""
	a.onboarding.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "apply onboarding theme", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := a.services.OperateTheme(ctx, a.sessionID, themeID, contract.ThemeOperationApply)
		cancel()
		if err == nil {
			err = a.appearance.reloadTheme()
		}
		_ = a.runOnUI("apply onboarding theme result", func() {
			a.onboarding.onboardingTheme.applying = false
			if err != nil {
				a.onboarding.onboardingTheme.error = err.Error()
			} else {
				a.generalSettings.Update(func(data *settingsData) { data.ThemeID = themeID })
				a.onboarding.onboardingTheme.error = ""
			}
			a.onboarding.invalidateOnboardingWindow()
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
	a.onboarding.invalidateOnboardingWindow()
}

func (a *onboardingController) openOnboardingChoice(kind string) {
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
				Value: glanceRefJSON(item.Ref), Label: label, Leading: a.imageForTint(source, &snapshot.palette.Text, physicalImageSize(18, imageScale)), Trailing: trailing,
			})
		}
		return choices
	}
	return nil
}

func (a *App) selectOnboardingChoice(value string) {
	kind := a.onboarding.onboardingChoice
	a.onboarding.onboardingChoice = ""
	a.onboarding.onboardingChoiceAnchor = woxui.Rect{}
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
				a.onboarding.onboardingError = err.Error()
			} else {
				a.onboarding.onboardingError = ""
			}
			if form := a.hotkeySettings.Form(); form != nil {
				form.active = true
			}
			a.onboarding.invalidateOnboardingWindow()
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
			a.onboarding.onboardingLoading = true
			a.onboarding.invalidateOnboardingWindow()
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
		a.onboarding.onboardingLoading = false
		if err != nil {
			a.onboarding.onboardingError = err.Error()
		} else {
			a.onboarding.onboardingPermission = status
			a.onboarding.onboardingError = ""
			if a.onboarding.permissionFlowHost != nil {
				permissionType := a.onboarding.permissionFlowHost.permissionType
				if macOSPermissionGranted(permissionType, status) {
					completedPermissionFlow = permissionType
				}
			}
		}
		a.onboarding.invalidateOnboardingWindow()
	})
	if completedPermissionFlow != "" {
		macospermission.Complete(completedPermissionFlow)
	}
}

func (a *App) openOnboardingPermission(permissionType string) {
	util.Go(a.lifecycleCtx, "open onboarding permission", func() {
		if err := a.openMacOSPermissionFlow(permissionType); err != nil {
			_ = a.runOnUI("show onboarding permission error", func() {
				a.onboarding.onboardingError = err.Error()
				a.onboarding.invalidateOnboardingWindow()
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
	if !a.onboarding.onboardingOpen {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		a.onboarding.onboardingChoice = ""
		a.onboarding.onboardingChoiceAnchor = woxui.Rect{}
		a.onboarding.invalidateOnboardingWindow()
	case woxui.KeyArrowLeft:
		a.selectOnboardingStep(a.onboarding.onboardingStep - 1)
	case woxui.KeyArrowRight:
		a.selectOnboardingStep(a.onboarding.onboardingStep + 1)
	default:
		return false
	}
	return true
}

func (a *App) onOnboardingWindowFocus(event woxui.FocusEvent) {
	if event.Active && a.onboarding.onboardingOpen && runtime.GOOS == "darwin" {
		util.Go(a.lifecycleCtx, "refresh focused onboarding permission status", a.loadOnboardingPermissionStatus)
	}
}
