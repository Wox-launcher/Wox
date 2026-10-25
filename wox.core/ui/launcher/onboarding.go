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

	"wox/plugin/thirdparty/migrate"
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

// beginSessionLocked starts a fresh guide session on the first page.
func (c *onboardingController) beginSessionLocked(installations []migrate.Installation) {
	c.open = true
	c.step = 0
	c.choice = ""
	c.choiceAnchor = woxui.Rect{}
	c.pageError = ""
	c.queryHotkey = nil
	c.plugins = onboardingPluginState{}
	c.themes = onboardingThemeState{}
	c.migration = newOnboardingMigrationState(installations)
}

// leaveLocked ends the guide session and reports whether it was open. Callers hide the
// window and notify Core outside the UI lock.
func (c *onboardingController) leaveLocked() bool {
	wasOpen := c.open
	if wasOpen {
		c.releaseOnboardingMigrationSession()
	}
	c.open = false
	c.choice = ""
	c.choiceAnchor = woxui.Rect{}
	return wasOpen
}

// present shows the guide window and starts the loads its first pages depend on.
func (c *onboardingController) present() error {
	if err := c.notifyOnboardingViewChanged(true); err != nil {
		return err
	}
	onboardingView, err := c.ensureOnboardingWindow()
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
	if err := c.runOnUI("load onboarding themes", c.loadOnboardingThemes); err != nil {
		return err
	}
	util.Go(c.lifecycleCtx, "load onboarding glance catalog", c.loadGlanceCatalog)
	if runtime.GOOS == "darwin" {
		util.Go(c.lifecycleCtx, "load onboarding permission status", c.loadOnboardingPermissionStatus)
	}
	return window.Invalidate()
}

// finishOnboarding durably dismisses the guide and restores the launcher.
func (c *onboardingController) finishOnboarding() {
	util.Go(c.lifecycleCtx, "finish onboarding", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := c.services.UpdateGeneralSetting(ctx, c.sessionID, "OnboardingFinished", "true")
		if err != nil {
			cancel()
			_ = c.runOnUI("show onboarding finish error", func() {
				c.pageError = err.Error()
				c.invalidateOnboardingWindow()
			})
			return
		}
		show := fromCoreShowOptions(c.services.LauncherShowOptions(ctx, c.sessionID))
		cancel()
		var onboardingView *woxui.ManagedWindow
		_ = c.runOnUI("leave onboarding", func() {
			c.leaveLocked()
			c.stopHotkeyRecording()
			c.releaseDemoWallpaperLocked()
			onboardingView = c.view
		})
		if err := c.notifyOnboardingViewChanged(false); err != nil {
			log.Printf("notify Wox core after onboarding: %v", err)
		}
		if onboardingView != nil {
			_ = onboardingView.Hide()
		}
		if err := c.showLauncher(show); err != nil {
			log.Printf("show launcher after onboarding: %v", err)
		}
		if err := confettioverlay.Show(); err != nil {
			util.GetLogger().Warn(context.Background(), "show onboarding confetti: "+err.Error())
		}
	})
}

func (c *onboardingController) buildOnboarding(frame woxui.FrameInfo) woxwidget.Widget {
	snapshot := c.settingsSnapshot()
	snapshot.palette.DensityScale = c.appearance.densityMetrics.normalized().scale
	steps := c.onboardingSteps()
	for index := range steps {
		steps[index].Accent = onboardingAccentColor
	}
	active := min(max(0, c.step), len(steps)-1)
	step := steps[active]
	labels := c.onboardingLabels()
	labels["title"] = c.language.translate("i18n:onboarding_title")
	labels["subtitle"] = c.language.translate("i18n:onboarding_subtitle")
	labels["back"] = c.language.translate("i18n:onboarding_back")
	labels["next"] = c.language.translate("i18n:onboarding_next")
	labels["finish"] = c.language.translate("i18n:onboarding_finish")
	labels["language"] = c.language.translate("i18n:ui_lang")
	labels["permission.authorize"] = c.language.translate("i18n:onboarding_permission_authorize")
	labels["permission.checking"] = c.language.translate("i18n:onboarding_permission_checking")
	labels["glance.enable"] = c.language.translate("i18n:onboarding_glance_enable")
	labels["glance.enable.body"] = c.language.translate("i18n:onboarding_glance_enable_tips")
	labels["glance.primary"] = c.language.translate("i18n:ui_glance_primary")
	labels["hotkey.available"] = c.language.translate("i18n:onboarding_hotkey_available")
	labels["hotkey.change"] = c.language.translate("i18n:onboarding_hotkey_change")
	labels["hotkey.preview"] = c.language.translate("i18n:onboarding_hotkey_preview")
	if c.pageError != "" {
		labels[step.ID+".body"] = c.pageError
	}
	if step.ID == "migrate" && c.migration.choosing && len(c.migration.installations) > 1 {
		labels["migrate.body"] = labels["migrate.choose"]
	}

	choices := c.onboardingChoices(snapshot, c.choice, frame.Scale)
	choiceValue := ""
	if c.choice == "language" {
		choiceValue = snapshot.general.Data.LangCode
	}
	language := snapshot.general.Data.LangCode
	for _, choice := range snapshot.general.Languages {
		if choice.value == language {
			language = choice.label
			break
		}
	}
	glanceLabel := c.language.translate("i18n:onboarding_glance_sample_time")
	glanceValue := c.language.translate("i18n:onboarding_glance_sample_value")
	var glanceIcon *woxui.Image
	currentGlance := snapshot.general.Data.PrimaryGlance
	if c.choice == "glance" {
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
			glanceIcon = c.imageForTint(source, &snapshot.palette.Text, physicalImageSize(18, frame.Scale))
			break
		}
	}
	permissions := []launcherview.OnboardingPermission{
		{
			ID: "accessibility", Title: c.language.translate("i18n:onboarding_permission_accessibility_title"),
			Description: c.language.translate("i18n:onboarding_permission_accessibility_body"), Ready: c.permissionStatus.Accessibility == "granted",
		},
		{
			ID: "fullDiskAccess", Title: c.language.translate("i18n:onboarding_permission_disk_title"),
			Description: c.language.translate("i18n:onboarding_permission_disk_body"), Ready: c.permissionStatus.FullDiskAccess == "granted",
		},
	}
	var mainHotkeyLabels, selectionHotkeyLabels []string
	hotkeyStatus := labels["hotkey.available"]
	hotkeyError := snapshot.general.Data.MainHotkeyRegistrationFailed
	hotkeyBlocked := hotkeyError
	hotkeyRecording := false
	if snapshot.hotkey.Form != nil {
		mainHotkey := snapshot.hotkey.Form.values["MainHotkey"]
		presentation := c.hotkeyRecordingFieldStatus("hotkey-settings", 0)
		if presentation.Active {
			mainHotkey = presentation.Value
			hotkeyStatus = presentation.Status
			hotkeyError = presentation.Error
			hotkeyBlocked = presentation.Error || (snapshot.general.Data.MainHotkeyRegistrationFailed && presentation.Value == snapshot.general.Data.MainHotkey)
			hotkeyRecording = true
		} else if hotkeyError {
			hotkeyStatus = c.language.translate(firstNonEmpty(snapshot.general.Data.MainHotkeyRegistrationError, "i18n:ui_hotkey_registration_failed"))
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
	if state := c.queryHotkey; state != nil {
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
		presentation := c.hotkeyRecordingFieldStatus("onboarding-query-hotkey", 0)
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
	plugins := make([]launcherview.OnboardingPlugin, len(c.plugins.plugins))
	for index, plugin := range c.plugins.plugins {
		plugins[index] = launcherview.OnboardingPlugin{
			ID: plugin.ID, Name: plugin.Name, Description: plugin.Description,
			Icon: c.imageForSurface(plugin.Icon, physicalImageSize(28, frame.Scale), snapshot.palette.Background), Installed: plugin.IsInstalled,
			Installing: c.plugins.operationID == plugin.ID,
			Disabled:   c.plugins.operationID != "",
		}
	}
	themes := onboardingThemeItems(snapshot.theme.Themes, snapshot.general.Data.ThemeID)
	previewTexts := []string{c.language.translate("i18n:ui_theme_preview_text_1"), c.language.translate("i18n:ui_theme_preview_text_2")}
	previewSubtitles := []string{
		strings.ReplaceAll(c.language.translate("i18n:ui_theme_preview_subtitle"), "{index}", "1"),
		strings.ReplaceAll(c.language.translate("i18n:ui_theme_preview_subtitle"), "{index}", "2"),
	}
	if step.ID == "themeInstall" {
		nextDisabled = c.themes.loading || len(themes) == 0
	}
	migrationSources, migrationCategories := c.onboardingMigrationView(frame.Scale, snapshot.palette.Background, labels)
	nextLabel := ""
	skipLabel := ""
	var onSkip func()
	if step.ID == "migrate" {
		if c.migration.importing || (c.migration.loading && !c.migration.loaded && !c.migration.choosing) {
			nextDisabled = true
		}
		if !c.migration.done && (c.onboardingMigrationCanImport() || c.migration.importing) {
			nextLabel = labels["migrate.import"]
			if c.migration.importing {
				nextLabel = labels["migrate.importing"]
			}
			if !c.migration.importing {
				skipLabel = labels["migrate.skip"]
				onSkip = func() { c.selectOnboardingStep(active + 1) }
			}
		} else if !c.migration.done {
			nextLabel = labels["migrate.skip"]
		}
	}
	if step.ID == "mainHotkey" && c.migration.mainHotkeyPending {
		nextDisabled = true
	}
	return launcherview.OnboardingView(launcherview.OnboardingProps{
		Width: frame.Size.Width, Height: frame.Size.Height, AppIcon: c.imageFor(appIconImageSource),
		Wallpaper: snapshot.theme.ThemeWallpaperImage, WallpaperBlurred: snapshot.theme.ThemeWallpaperBlurred,
		Steps: steps, ActiveStep: active, Labels: labels, Language: language,
		GlanceEnabled: snapshot.general.Data.EnableGlance, GlanceLabel: glanceLabel, GlanceValue: glanceValue, GlanceIcon: glanceIcon,
		MainHotkeyLabels: mainHotkeyLabels, SelectHotkeyLabels: selectionHotkeyLabels,
		HotkeyStatus: hotkeyStatus, HotkeyError: hotkeyError, HotkeyRecording: hotkeyRecording,
		QueryHotkeyLabels: queryHotkeyLabels, QueryHotkeyStatus: queryHotkeyStatus, QueryHotkeyError: queryHotkeyError,
		QueryHotkeyRecording: queryHotkeyRecording, QueryHotkeyReady: queryHotkeyReady, QueryHotkeySelected: queryHotkeySelected, QueryHotkeyBusy: queryHotkeyBusy,
		Plugins: plugins, PluginsLoading: c.plugins.loading, PluginsError: c.plugins.error,
		Themes: themes, ThemesLoading: c.themes.loading, ThemesApplying: c.themes.applying, ThemesError: c.themes.error,
		MigrationChoosing: c.migration.choosing, MigrationSources: migrationSources, MigrationCategories: migrationCategories,
		MigrationLoading: c.migration.loading, MigrationImporting: c.migration.importing, MigrationDone: c.migration.done,
		MigrationActiveID: c.migration.activeID, MigrationError: c.migrationExportMessage(c.migration.error),
		MigrationUnlock:     c.migrationNeedsUnlock() && !c.migration.choosing,
		MigrationExportPath: c.migration.exportPath, MigrationPassphrase: c.migrationPassphraseController(),
		ThemePreviewTitle: c.language.translate("i18n:ui_theme_preview_title"), ThemePreviewTexts: previewTexts,
		ThemePreviewSubs: previewSubtitles, ThemePreviewOpen: c.language.translate("i18n:ui_theme_preview_open"),
		Permissions: permissions, PermissionLoading: c.permissionLoading,
		ChoiceKind: c.choice, ChoiceValue: choiceValue, ChoiceAnchor: c.choiceAnchor, Choices: choices,
		Window: c.onboardingNativeWindow(), Theme: snapshot.palette, PreviewTheme: onboardingPreviewTheme,
		OnDrag: func() {
			if window := c.onboardingNativeWindow(); window != nil {
				_ = window.StartDragging()
			}
		},
		NextDisabled: nextDisabled, NextLabel: nextLabel, SkipLabel: skipLabel, OnSkip: onSkip,
		OnStep: c.selectOnboardingStep, OnBack: func() { c.selectOnboardingStep(active - 1) }, OnNext: func() {
			if step.ID == "migrate" && !c.migration.done && c.onboardingMigrationCanImport() {
				c.importOnboardingMigration()
				return
			}
			c.selectOnboardingStep(active + 1)
		},
		OnFinish: c.finishOnboarding, OnToggleGlance: c.setOnboardingGlanceEnabled,
		OnRecordHotkey:          func() { c.focusOnboardingHotkey(0); c.recordHotkeySettingsField(0) },
		OnRecordQueryHotkey:     c.recordOnboardingQueryHotkey,
		OnToggleQueryHotkey:     c.toggleOnboardingQueryHotkey,
		OnInstallPlugin:         c.installOnboardingPlugin,
		OnSelectTheme:           c.selectOnboardingTheme,
		OnChooseMigration:       c.chooseOnboardingMigration,
		OnToggleMigrationItem:   c.toggleOnboardingMigrationItem,
		OnPickMigrationFile:     c.pickOnboardingMigrationFile,
		OnUnlockMigration:       c.unlockOnboardingMigration,
		OnToggleMigrationOption: c.toggleOnboardingMigrationOption,
		OnOpenChoice:            c.openOnboardingChoice, OnSelectChoice: c.selectOnboardingChoice, OnPermission: c.openOnboardingPermission,
	})
}

func (c *onboardingController) onboardingSteps() []launcherview.OnboardingStep {
	// Recording the main hotkey needs Accessibility. On macOS that permission
	// step comes first, otherwise a denied recording disables Continue.
	specs := []onboardingStepSpec{
		{"welcome", "onboarding_welcome_title", woxui.Color{R: 45, G: 212, B: 191, A: 255}},
	}
	if runtime.GOOS == "darwin" {
		specs = append(specs, onboardingStepSpec{"permissions", "onboarding_permissions_title", woxui.Color{R: 249, G: 115, B: 22, A: 255}})
	}
	// Migration is the next page, after permissions on macOS, so the hotkey pages can still adjust what was imported.
	if len(c.migration.installations) > 0 {
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
	if !c.migration.themeImported {
		specs = append(specs, onboardingStepSpec{"themeInstall", "onboarding_theme_install_title", woxui.Color{R: 232, G: 121, B: 249, A: 255}})
	}
	specs = append(specs, onboardingStepSpec{"finish", "onboarding_finish_title", woxui.Color{R: 45, G: 212, B: 191, A: 255}})
	steps := make([]launcherview.OnboardingStep, len(specs))
	for index, spec := range specs {
		steps[index] = launcherview.OnboardingStep{ID: spec.id, Title: c.language.translate("i18n:" + spec.key), Accent: spec.accent}
	}
	return steps
}

func (c *onboardingController) onboardingLabels() map[string]string {
	labels := map[string]string{
		"welcome.body":                  c.language.translate("i18n:onboarding_welcome_description"),
		"welcome.apps":                  c.language.translate("i18n:onboarding_welcome_apps"),
		"welcome.files":                 c.language.translate("i18n:onboarding_welcome_files"),
		"welcome.plugins":               c.language.translate("i18n:onboarding_welcome_plugins"),
		"welcome.ai":                    c.language.translate("i18n:onboarding_welcome_ai"),
		"welcome.hint":                  c.language.translate("i18n:onboarding_welcome_hint"),
		"permissions.body":              c.language.translate("i18n:onboarding_permissions_description"),
		"mainHotkey.body":               c.language.translate("i18n:onboarding_main_hotkey_description"),
		"selectionHotkey.body":          c.language.translate("i18n:onboarding_selection_hotkey_description"),
		"glance.body":                   c.language.translate("i18n:onboarding_glance_description"),
		"queryHotkeys.body":             c.language.translate("i18n:onboarding_query_hotkeys_body"),
		"queryHotkeys.checking":         c.language.translate("i18n:onboarding_query_hotkeys_checking"),
		"queryHotkeys.configured":       c.language.translate("i18n:onboarding_query_hotkeys_configured"),
		"queryHotkeys.notConfigured":    c.language.translate("i18n:onboarding_query_hotkeys_not_configured"),
		"queryHotkeys.status.title":     c.language.translate("i18n:onboarding_query_hotkeys_status_title"),
		"queryHotkeys.status.body":      c.language.translate("i18n:onboarding_query_hotkeys_status_body"),
		"queryHotkeys.clipboard":        c.language.translate("i18n:onboarding_query_hotkeys_clipboard"),
		"queryHotkeys.shortcut":         c.language.translate("i18n:onboarding_query_hotkeys_shortcut"),
		"plugins.loading":               c.language.translate("i18n:onboarding_plugins_loading"),
		"plugins.install":               c.language.translate("i18n:ui_plugin_install"),
		"plugins.installing":            c.language.translate("i18n:ui_plugin_installing"),
		"plugins.installed":             c.language.translate("i18n:onboarding_plugin_installed"),
		"plugins.more":                  c.language.translate("i18n:onboarding_plugins_more"),
		"theme.loading":                 c.language.translate("i18n:onboarding_theme_loading"),
		"queryAliases.body":             c.language.translate("i18n:onboarding_query_shortcuts_body"),
		"queryAliases.title":            c.language.translate("i18n:ui_query_shortcuts"),
		"trayQueries.body":              c.language.translate("i18n:onboarding_tray_queries_body"),
		"wpmInstall.body":               c.language.translate("i18n:onboarding_wpm_install_body"),
		"themeInstall.body":             c.language.translate("i18n:onboarding_theme_install_body"),
		"migrate.body":                  c.language.translate("i18n:onboarding_migrate_body"),
		"migrate.choose":                c.language.translate("i18n:onboarding_migrate_choose_body"),
		"migrate.import":                c.language.translate("i18n:onboarding_migrate_import"),
		"migrate.importing":             c.language.translate("i18n:onboarding_migrate_importing"),
		"migrate.skip":                  c.language.translate("i18n:onboarding_migrate_skip"),
		"migrate.empty":                 c.language.translate("i18n:onboarding_migrate_empty"),
		"migrate.loading":               c.language.translate("i18n:onboarding_migrate_loading"),
		"migrate.imported":              c.language.translate("i18n:onboarding_migrate_status_imported"),
		"migrate.unsupported":           c.language.translate("i18n:onboarding_migrate_status_unsupported"),
		"migrate.copying":               c.language.translate("i18n:onboarding_migrate_status_importing"),
		"migrate.copied":                c.language.translate("i18n:onboarding_migrate_status_done"),
		"migrate.failed":                c.language.translate("i18n:onboarding_migrate_status_failed"),
		"migrate.conflict":              c.language.translate("i18n:onboarding_migrate_status_conflict"),
		"migrate.cancelled":             c.language.translate("i18n:onboarding_migrate_status_cancelled"),
		"migrate.help":                  c.language.translate("i18n:onboarding_migrate_raycast_help"),
		"migrate.file":                  c.language.translate("i18n:onboarding_migrate_raycast_file"),
		"migrate.passphrase":            c.language.translate("i18n:onboarding_migrate_raycast_passphrase"),
		"migrate.continue":              c.language.translate("i18n:onboarding_migrate_raycast_continue"),
		"migrate.unlocking":             c.language.translate("i18n:onboarding_migrate_raycast_unlocking"),
		"migrate.python":                c.language.translate("i18n:onboarding_migrate_python"),
		"finish.body":                   c.language.translate("i18n:onboarding_finish_card_body"),
		"finish.hotkey":                 c.language.translate("i18n:onboarding_finish_summary_hotkey"),
		"finish.glance":                 c.language.translate("i18n:onboarding_finish_summary_glance"),
		"finish.plugins":                c.language.translate("i18n:onboarding_finish_summary_plugins"),
		"finish.hint":                   c.language.translate("i18n:onboarding_finish_summary_hint"),
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
		"preview.more":                  c.language.translate("i18n:onboarding_action_panel_title"),
		"demo.concept.title":            c.language.translate("i18n:onboarding_query_concept_title"),
		"demo.concept.trigger":          c.language.translate("i18n:onboarding_query_concept_trigger_keyword"),
		"demo.concept.command":          c.language.translate("i18n:onboarding_query_concept_command"),
		"demo.concept.search":           c.language.translate("i18n:onboarding_query_concept_search_term"),
		"demo.concept.optional":         c.language.translate("i18n:onboarding_query_concept_optional"),
		"demo.concept.result1.subtitle": c.language.translate("i18n:onboarding_query_concept_result1_subtitle"),
		"demo.concept.result2.subtitle": c.language.translate("i18n:onboarding_query_concept_result2_subtitle"),
		"demo.concept.result3.subtitle": c.language.translate("i18n:onboarding_query_concept_result3_subtitle"),
		"demo.install":                  c.language.translate("i18n:plugin_wpm_install"),
		"demo.installing":               c.language.translate("i18n:plugin_wpm_installing"),
		"demo.installed":                c.language.translate("i18n:plugin_wpm_start_using"),
		"demo.apply":                    c.language.translate("i18n:ui_setting_theme_apply"),
		"demo.actions":                  c.language.translate("i18n:onboarding_action_panel_title"),
		"demo.action.copy":              c.language.translate("i18n:onboarding_action_panel_copy"),
		"demo.action.more":              c.language.translate("i18n:onboarding_action_panel_more"),
		"demo.selection.preview":        c.language.translate("i18n:selection_preview"),
		"demo.selection.copy_path":      c.language.translate("i18n:selection_copy_path"),
		"demo.selection.open_folder":    c.language.translate("i18n:selection_open_containing_folder"),
		"demo.glance.provider":          c.language.translate("i18n:onboarding_glance_sample_provider"),
		"demo.glance.value":             c.language.translate("i18n:onboarding_glance_sample_time"),
		"demo.permission.ready":         c.language.translate("i18n:onboarding_permission_ready"),
		"demo.finish.title":             c.language.translate("i18n:onboarding_finish_card_title"),
		"demo.finish.badge":             c.language.translate("i18n:onboarding_finish_badge"),
		"demo.finish.settings":          c.language.translate("i18n:plugin_sys_open_wox_settings"),
		"demo.finish.system_settings":   c.language.translate("i18n:plugin_sys_open_system_settings"),
	}
	for _, step := range []string{"welcome", "permissions", "mainHotkey", "selectionHotkey", "glance", "queryHotkeys", "trayQueries", "wpmInstall", "themeInstall", "migrate", "finish"} {
		labels[step+".preview"] = labels[step+".body"]
	}
	return labels
}

func (c *onboardingController) selectOnboardingStep(index int) {
	steps := c.onboardingSteps()
	if index < 0 || index >= len(steps) {
		return
	}
	if c.step >= 0 && c.step < len(steps) && steps[c.step].ID == "migrate" && index != c.step {
		if c.migration.importing {
			return
		}
		if index == c.step-1 && c.retreatOnboardingMigration() {
			return
		}
		c.releaseOnboardingMigrationSession()
	}
	if index > c.step && steps[c.step].ID == "mainHotkey" {
		data := c.generalSettings().Data()
		presentation := c.hotkeyRecordingFieldStatus("hotkey-settings", 0)
		if (presentation.Active && presentation.Error) || (data.MainHotkeyRegistrationFailed && (!presentation.Active || presentation.Value == data.MainHotkey)) {
			return
		}
	}
	if index > c.step && steps[c.step].ID == "queryHotkeys" && (c.queryHotkey == nil || (c.queryHotkey.selected && !c.queryHotkey.ready)) {
		return
	}
	if index != c.step {
		c.stopHotkeyRecording()
	}
	c.step = index
	c.choice = ""
	c.choiceAnchor = woxui.Rect{}
	if form := c.hotkeySettings().Form(); form != nil {
		form.active = true
		if steps[index].ID == "mainHotkey" {
			setFormFieldsFocusLocked(form, 0)
		} else if steps[index].ID == "selectionHotkey" && len(form.definitions) > 1 {
			setFormFieldsFocusLocked(form, 1)
		}
	}
	if steps[index].ID == "mainHotkey" {
		c.applyStagedMigrationHotkey()
	}
	if steps[index].ID == "permissions" && runtime.GOOS == "darwin" {
		util.Go(c.lifecycleCtx, "refresh onboarding permission status", c.loadOnboardingPermissionStatus)
	}
	if steps[index].ID == "queryHotkeys" {
		c.prepareOnboardingQueryHotkey()
	}
	if steps[index].ID == "wpmInstall" {
		c.loadOnboardingPlugins()
	}
	if steps[index].ID == "themeInstall" {
		c.loadOnboardingThemes()
	}
	if steps[index].ID == "migrate" {
		c.prepareOnboardingMigration()
	}
	c.invalidateOnboardingWindow()
}

func defaultOnboardingQueryHotkey() string {
	if runtime.GOOS == "darwin" {
		return "Cmd+Shift+V"
	}
	return "Ctrl+Shift+V"
}

func (c *onboardingController) prepareOnboardingQueryHotkey() {
	if c.queryHotkey != nil {
		return
	}
	hotkey := defaultOnboardingQueryHotkey()
	form := newFormFieldsState([]formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "Hotkey"}}}, map[string]string{"Hotkey": hotkey}, true)
	state := &onboardingQueryHotkeyState{form: form, selected: true}
	c.queryHotkey = state
	for _, item := range c.generalSettings().Data().QueryHotkeys {
		if !item.Disabled && strings.EqualFold(strings.TrimSpace(item.Query), "cb") {
			state.form.values["Hotkey"] = item.Hotkey
			state.ready = true
			state.saved = true
			return
		}
	}
	util.Go(c.lifecycleCtx, "check onboarding query hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		availability, err := c.services.CheckHotkeyAvailability(ctx, c.sessionID, hotkey)
		cancel()
		_ = c.runOnUI("apply onboarding query hotkey availability", func() {
			if c.queryHotkey != state {
				return
			}
			if err != nil {
				state.error = err.Error()
			} else if !availability.Available {
				state.error = c.hotkeyConflictMessage(availability.ConflictType, availability.ConflictValue)
			} else {
				c.saveOnboardingQueryHotkey(hotkey)
			}
			c.invalidateOnboardingWindow()
		})
	})
}

func (c *onboardingController) recordOnboardingQueryHotkey() {
	state := c.queryHotkey
	if state == nil || state.saving {
		return
	}
	if !state.selected {
		c.toggleOnboardingQueryHotkey(true)
		return
	}
	state.error = ""
	c.startHotkeyRecording("onboarding-query-hotkey", &state.form, 0, "", defaultHotkeyRecordingKinds, nil)
}

func (c *onboardingController) saveOnboardingQueryHotkey(hotkey string) {
	state := c.queryHotkey
	if state == nil || state.saving {
		return
	}
	state.saving = true
	state.error = ""
	items := upsertOnboardingQueryHotkey(c.generalSettings().Data().QueryHotkeys, hotkey)
	raw, err := json.Marshal(items)
	if err != nil {
		state.saving = false
		state.error = err.Error()
		return
	}
	util.Go(c.lifecycleCtx, "save onboarding query hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := c.services.UpdateGeneralSetting(ctx, c.sessionID, "QueryHotkeys", string(raw))
		cancel()
		if err == nil {
			err = c.reloadSettings()
		}
		_ = c.runOnUI("apply onboarding query hotkey", func() {
			if c.queryHotkey != state {
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
			c.invalidateOnboardingWindow()
		})
	})
}

func (c *onboardingController) toggleOnboardingQueryHotkey(enabled bool) {
	state := c.queryHotkey
	if state == nil || state.saving || state.selected == enabled {
		return
	}
	if enabled {
		c.queryHotkey = nil
		c.prepareOnboardingQueryHotkey()
		c.invalidateOnboardingWindow()
		return
	}
	c.stopHotkeyRecording()
	state.selected = false
	state.ready = false
	state.error = ""
	if !state.saved {
		c.invalidateOnboardingWindow()
		return
	}
	state.saving = true
	raw, err := json.Marshal(removeOnboardingQueryHotkey(c.generalSettings().Data().QueryHotkeys))
	if err != nil {
		state.saving = false
		state.selected = true
		state.ready = true
		state.error = err.Error()
		return
	}
	util.Go(c.lifecycleCtx, "remove onboarding query hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := c.services.UpdateGeneralSetting(ctx, c.sessionID, "QueryHotkeys", string(raw))
		cancel()
		if err == nil {
			err = c.reloadSettings()
		}
		_ = c.runOnUI("apply removed onboarding query hotkey", func() {
			if c.queryHotkey != state {
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
			c.invalidateOnboardingWindow()
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
func (c *onboardingController) loadOnboardingPlugins() {
	if c.plugins.loading || len(c.plugins.plugins) > 0 {
		return
	}
	c.plugins.loading = true
	c.plugins.error = ""
	c.invalidateOnboardingWindow()
	util.Go(c.lifecycleCtx, "load onboarding plugins", func() {
		plugins, err := loadPluginSettingsPlugins(context.Background(), c.services, c.sessionID, true)
		_ = c.runOnUI("apply onboarding plugins", func() {
			c.plugins.loading = false
			if err != nil {
				c.plugins.error = err.Error()
			} else {
				c.plugins.plugins = onboardingRecommendedPlugins(plugins, runtime.GOOS)
				c.plugins.error = ""
			}
			c.invalidateOnboardingWindow()
		})
	})
}

// installOnboardingPlugin installs one recommendation through the shared plugin operation service.
func (c *onboardingController) installOnboardingPlugin(pluginID string) {
	if c.plugins.operationID != "" {
		return
	}
	found := false
	for _, plugin := range c.plugins.plugins {
		if plugin.ID == pluginID && !plugin.IsInstalled {
			found = true
			break
		}
	}
	if !found {
		return
	}
	c.plugins.operationID = pluginID
	c.plugins.error = ""
	c.invalidateOnboardingWindow()
	util.Go(c.lifecycleCtx, "install onboarding plugin", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		operationErr := c.services.OperatePlugin(ctx, c.sessionID, pluginID, contract.PluginOperationInstall)
		cancel()
		var plugins []pluginSettingsPlugin
		var refreshErr error
		if operationErr == nil {
			plugins, refreshErr = loadPluginSettingsPlugins(context.Background(), c.services, c.sessionID, true)
		}
		_ = c.runOnUI("apply onboarding plugin install", func() {
			c.plugins.operationID = ""
			if operationErr != nil {
				c.plugins.error = operationErr.Error()
			} else {
				if refreshErr == nil {
					c.plugins.plugins = onboardingRecommendedPlugins(plugins, runtime.GOOS)
					c.plugins.error = ""
				} else {
					for index := range c.plugins.plugins {
						if c.plugins.plugins[index].ID == pluginID {
							c.plugins.plugins[index].IsInstalled = true
						}
					}
					c.plugins.error = refreshErr.Error()
				}
				c.pluginSettings().invalidateCachedPlugins(true)
				c.pluginSettings().invalidateCachedPlugins(false)
				c.settingsSearch().SetLoaded(false)
			}
			c.invalidateOnboardingWindow()
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
func (c *onboardingController) loadOnboardingThemes() {
	if c.themes.loading || len(onboardingSystemThemes(c.themeSettings().Themes())) == 4 {
		return
	}
	c.themes.loading = true
	c.themes.error = ""
	themeID := c.currentThemeID()
	c.invalidateOnboardingWindow()
	util.Go(c.lifecycleCtx, "load onboarding themes", func() {
		util.GetLogger().Info(c.lifecycleCtx, "loading onboarding system themes")
		err := c.reloadThemes("installed", themeID)
		if applyErr := c.runOnUI("apply onboarding themes", func() {
			c.themes.loading = false
			if err != nil {
				c.themes.error = err.Error()
			} else {
				c.themes.error = ""
			}
			util.GetLogger().Info(c.lifecycleCtx, fmt.Sprintf("onboarding system themes loaded: count=%d error=%v", len(onboardingSystemThemes(c.themeSettings().Themes())), err))
			c.invalidateOnboardingWindow()
		}); applyErr != nil {
			util.GetLogger().Error(c.lifecycleCtx, applyErr.Error())
		}
	})
}

// selectOnboardingTheme applies a bundled theme without changing the onboarding step.
func (c *onboardingController) selectOnboardingTheme(themeID string) {
	if c.themes.loading || c.themes.applying {
		return
	}
	found := false
	for _, theme := range onboardingSystemThemes(c.themeSettings().Themes()) {
		if theme.ID == themeID {
			found = true
			break
		}
	}
	if !found {
		return
	}
	c.themes.error = ""
	if c.currentThemeID() == themeID {
		c.invalidateOnboardingWindow()
		return
	}
	c.themes.applying = true
	c.themes.error = ""
	c.invalidateOnboardingWindow()
	util.Go(c.lifecycleCtx, "apply onboarding theme", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := c.services.OperateTheme(ctx, c.sessionID, themeID, contract.ThemeOperationApply)
		cancel()
		if err == nil {
			err = c.appearance.reloadTheme()
		}
		_ = c.runOnUI("apply onboarding theme result", func() {
			c.themes.applying = false
			if err != nil {
				c.themes.error = err.Error()
			} else {
				c.generalSettings().Update(func(data *settingsData) { data.ThemeID = themeID })
				c.themes.error = ""
			}
			c.invalidateOnboardingWindow()
		})
	})
}

func (c *onboardingController) focusOnboardingHotkey(index int) {
	form := c.hotkeySettings().Form()
	if form == nil || index < 0 || index >= len(form.definitions) {
		return
	}
	form.active = true
	setFormFieldsFocusLocked(form, index)
	c.hotkeySettings().SetFocused(true)
	c.invalidateOnboardingWindow()
}

func (c *onboardingController) openOnboardingChoice(kind string) {
	if kind != "language" && kind != "glance" {
		return
	}
	if c.choice == kind {
		c.choice = ""
		c.choiceAnchor = woxui.Rect{}
	} else {
		c.choice = kind
		id := "onboarding-language"
		if kind == "glance" {
			id = "onboarding-glance-choice"
		}
		c.choiceAnchor = woxui.Rect{}
		if c.host != nil {
			c.choiceAnchor, _ = c.host.BoundsForKey(woxwidget.Key(id))
		}
	}
	c.invalidateOnboardingWindow()
}

func (c *onboardingController) onboardingChoices(snapshot settingsSnapshot, kind string, imageScale float32) []launcherview.OnboardingChoice {
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
				Value: glanceRefJSON(item.Ref), Label: label, Leading: c.imageForTint(source, &snapshot.palette.Text, physicalImageSize(18, imageScale)), Trailing: trailing,
			})
		}
		return choices
	}
	return nil
}

func (c *onboardingController) selectOnboardingChoice(value string) {
	kind := c.choice
	c.choice = ""
	c.choiceAnchor = woxui.Rect{}
	key := ""
	switch kind {
	case "language":
		key = "LangCode"
	case "glance":
		key = "PrimaryGlance"
	default:
		return
	}
	c.saveOnboardingSetting(key, value)
}

func (c *onboardingController) setOnboardingGlanceEnabled(enabled bool) {
	c.saveOnboardingSetting("EnableGlance", strconv.FormatBool(enabled))
}

func (c *onboardingController) saveOnboardingSetting(key, value string) {
	util.Go(c.lifecycleCtx, "save onboarding setting", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := c.services.UpdateGeneralSetting(ctx, c.sessionID, key, value)
		cancel()
		if err == nil {
			err = c.reloadSettings()
		}
		if err == nil && key == "LangCode" {
			err = c.reloadTranslations()
		}
		_ = c.runOnUI("apply onboarding setting", func() {
			if err != nil {
				c.pageError = err.Error()
			} else {
				c.pageError = ""
			}
			if form := c.hotkeySettings().Form(); form != nil {
				form.active = true
			}
			c.invalidateOnboardingWindow()
		})
	})
}

func (c *onboardingController) loadOnboardingPermissionStatus() {
	c.loadOnboardingPermissionStatusWithLoading(true)
}

// loadOnboardingPermissionStatusWithLoading probes TCC without flashing the authorize button on background refreshes.
func (c *onboardingController) loadOnboardingPermissionStatusWithLoading(showLoading bool) {
	if showLoading {
		_ = c.runOnUI("start onboarding permission check", func() {
			c.permissionLoading = true
			c.invalidateOnboardingWindow()
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	status, err := c.services.MacOSPermissionStatus(ctx, c.sessionID)
	cancel()
	if err == nil {
		if reconcileErr := keyboard.ReconcileRawKeyListenerAccessWithPermissionStatus(status.Accessibility == "granted"); reconcileErr != nil {
			util.GetLogger().Warn(context.Background(), "failed to reconcile macOS raw keyboard access: "+reconcileErr.Error())
		}
	}
	var completedPermissionFlow permission.MacOSPermissionType
	_ = c.runOnUI("apply onboarding permission status", func() {
		c.permissionLoading = false
		if err != nil {
			c.pageError = err.Error()
		} else {
			c.permissionStatus = status
			c.pageError = ""
			if permissionType := c.permissionFlowTarget(); permissionType != "" && macOSPermissionGranted(permissionType, status) {
				completedPermissionFlow = permissionType
			}
		}
		c.invalidateOnboardingWindow()
	})
	if completedPermissionFlow != "" {
		macospermission.Complete(completedPermissionFlow)
	}
}

func (c *onboardingController) openOnboardingPermission(permissionType string) {
	util.Go(c.lifecycleCtx, "open onboarding permission", func() {
		if err := c.openMacOSPermissionFlow(permissionType); err != nil {
			_ = c.runOnUI("show onboarding permission error", func() {
				c.pageError = err.Error()
				c.invalidateOnboardingWindow()
			})
		}
	})
}

func (c *onboardingController) notifyOnboardingViewChanged(inOnboardingView bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.services.OnboardingViewChanged(ctx, c.sessionID, inOnboardingView)
}

func (c *onboardingController) onOnboardingKey(event woxui.KeyEvent) bool {
	if !c.open {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		c.choice = ""
		c.choiceAnchor = woxui.Rect{}
		c.invalidateOnboardingWindow()
	case woxui.KeyArrowLeft:
		c.selectOnboardingStep(c.step - 1)
	case woxui.KeyArrowRight:
		c.selectOnboardingStep(c.step + 1)
	default:
		return false
	}
	return true
}

func (c *onboardingController) onOnboardingWindowFocus(event woxui.FocusEvent) {
	if event.Active && c.open && runtime.GOOS == "darwin" {
		util.Go(c.lifecycleCtx, "refresh focused onboarding permission status", c.loadOnboardingPermissionStatus)
	}
}
