package launcher

import (
	"context"
	"log"
	"runtime"
	"strconv"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const (
	onboardingWindowWidth  = float32(1040)
	onboardingWindowHeight = float32(800)
)

type onboardingStepSpec struct {
	id     string
	key    string
	accent woxui.Color
}

// openOnboarding presents the first-run guide in its dedicated window.
func (a *App) openOnboarding() error {
	if err := a.reloadSettings(); err != nil {
		return err
	}
	if err := a.hideWindow(true); err != nil {
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
		a.stopHotkeyRecording()
		a.preloadDemoWallpaper(true)
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
		cancel()
		if err != nil {
			_ = a.runOnUI("show onboarding finish error", func() {
				a.onboardingError = err.Error()
				a.invalidateOnboardingWindow()
			})
			return
		}
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
		if err := a.showWindow(a.show); err != nil {
			log.Printf("show launcher after onboarding: %v", err)
		}
	})
}

func (a *App) buildOnboarding(frame woxui.FrameInfo) woxwidget.Widget {
	snapshot := a.settingsSnapshot()
	steps := a.onboardingSteps()
	active := min(max(0, a.onboardingStep), len(steps)-1)
	step := steps[active]
	labels := a.onboardingLabels()
	labels["title"] = a.translate("i18n:onboarding_title")
	labels["subtitle"] = a.translate("i18n:onboarding_subtitle")
	labels["skip"] = a.translate("i18n:onboarding_skip")
	labels["back"] = a.translate("i18n:onboarding_back")
	labels["next"] = a.translate("i18n:onboarding_next")
	labels["finish"] = a.translate("i18n:onboarding_finish")
	labels["language"] = a.translate("i18n:ui_lang")
	labels["permission.authorize"] = a.translate("i18n:onboarding_permission_authorize")
	labels["permission.checking"] = a.translate("i18n:onboarding_permission_checking")
	labels["glance.enable"] = a.translate("i18n:ui_glance_enable")
	labels["glance.enable.body"] = a.translate("i18n:ui_glance_enable_tips")
	labels["glance.primary"] = a.translate("i18n:onboarding_glance_picker_label")
	if a.onboardingError != "" {
		labels[step.ID+".body"] = a.onboardingError
	}

	var hotkey woxwidget.Widget = woxwidget.Container{}
	if snapshot.hotkey.Form != nil && (step.ID == "mainHotkey" || step.ID == "selectionHotkey") {
		index := 0
		if step.ID == "selectionHotkey" {
			index = 1
		}
		if index < len(snapshot.hotkey.Form.definitions) {
			fields := *snapshot.hotkey.Form
			fields.active = true
			fields.focused = index
			hotkey = a.buildFormHotkey(fields, formFieldCallbacks{
				idPrefix: "hotkey-settings", imageScale: frame.Scale, alignHotkeyRight: true, focus: a.focusOnboardingHotkey, recordKey: a.recordHotkeySettingsField,
			}, snapshot.palette, index, fields.definitions[index], max(float32(0), frame.Size.Width-launcherview.OnboardingSidebarWidth-112), 62)
		}
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
	if snapshot.hotkey.Form != nil {
		mainHotkeyLabels = formatHotkeyLabels(snapshot.hotkey.Form.values["MainHotkey"])
		selectionHotkeyLabels = formatHotkeyLabels(snapshot.hotkey.Form.values["SelectionHotkey"])
	}
	return launcherview.OnboardingView(launcherview.OnboardingProps{
		Width: frame.Size.Width, Height: frame.Size.Height, AppIcon: a.imageFor(appIconImageSource),
		Wallpaper: snapshot.theme.ThemeWallpaperImage, WallpaperBlurred: snapshot.theme.ThemeWallpaperBlurred,
		Steps: steps, ActiveStep: active, Labels: labels, Language: language,
		GlanceEnabled: snapshot.general.Data.EnableGlance, GlanceLabel: glanceLabel, GlanceValue: glanceValue, GlanceIcon: glanceIcon,
		MainHotkeyLabels: mainHotkeyLabels, SelectHotkeyLabels: selectionHotkeyLabels, Hotkey: hotkey,
		Permissions: permissions, PermissionLoading: a.onboardingLoading,
		ChoiceKind: a.onboardingChoice, ChoiceValue: choiceValue, ChoiceAnchor: a.onboardingChoiceAnchor, Choices: choices,
		Window: a.onboardingNativeWindow(), Theme: snapshot.palette.componentTheme(),
		OnStep: a.selectOnboardingStep, OnBack: func() { a.selectOnboardingStep(active - 1) }, OnNext: func() { a.selectOnboardingStep(active + 1) },
		OnSkip: a.finishOnboarding, OnFinish: a.finishOnboarding, OnToggleGlance: a.setOnboardingGlanceEnabled,
		OnOpenChoice: a.openOnboardingChoice, OnSelectChoice: a.selectOnboardingChoice, OnPermission: a.openOnboardingPermission,
	})
}

func (a *App) onboardingSteps() []launcherview.OnboardingStep {
	specs := []onboardingStepSpec{
		{"welcome", "onboarding_welcome_title", woxui.Color{R: 45, G: 212, B: 191, A: 255}},
	}
	if runtime.GOOS == "darwin" {
		specs = append(specs, onboardingStepSpec{"permissions", "onboarding_permissions_title", woxui.Color{R: 249, G: 115, B: 22, A: 255}})
	}
	specs = append(specs,
		onboardingStepSpec{"mainHotkey", "onboarding_main_hotkey_title", woxui.Color{R: 249, G: 115, B: 22, A: 255}},
		onboardingStepSpec{"selectionHotkey", "onboarding_selection_hotkey_title", woxui.Color{R: 96, G: 165, B: 250, A: 255}},
		onboardingStepSpec{"glance", "onboarding_glance_title", woxui.Color{R: 250, G: 204, B: 21, A: 255}},
		onboardingStepSpec{"queryHotkeys", "onboarding_query_hotkeys_title", woxui.Color{R: 244, G: 63, B: 94, A: 255}},
		onboardingStepSpec{"trayQueries", "onboarding_tray_queries_title", woxui.Color{R: 34, G: 197, B: 94, A: 255}},
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
		"welcome.body":                  a.translate("i18n:onboarding_welcome_card_body"),
		"permissions.body":              a.translate("i18n:onboarding_permissions_description"),
		"mainHotkey.body":               a.translate("i18n:onboarding_main_hotkey_description"),
		"selectionHotkey.body":          a.translate("i18n:onboarding_selection_hotkey_description"),
		"glance.body":                   a.translate("i18n:onboarding_glance_description"),
		"queryHotkeys.body":             a.translate("i18n:onboarding_query_hotkeys_body"),
		"queryShortcuts.body":           a.translate("i18n:onboarding_query_shortcuts_body"),
		"queryShortcuts.title":          a.translate("i18n:ui_query_shortcuts"),
		"trayQueries.body":              a.translate("i18n:onboarding_tray_queries_body"),
		"wpmInstall.body":               a.translate("i18n:onboarding_wpm_install_body"),
		"themeInstall.body":             a.translate("i18n:onboarding_theme_install_body"),
		"finish.body":                   a.translate("i18n:onboarding_finish_card_body"),
		"welcome.query":                 "wpm install everything",
		"permissions.query":             "permissions",
		"mainHotkey.query":              "apps",
		"selectionHotkey.query":         "Roadmap.md",
		"glance.query":                  "wox",
		"queryHotkeys.query":            "github repo",
		"trayQueries.query":             "weather",
		"wpmInstall.query":              "wpm install",
		"themeInstall.query":            "theme",
		"finish.query":                  "ready",
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
		"demo.glance.provider":          a.translate("i18n:onboarding_glance_sample_provider"),
		"demo.glance.value":             a.translate("i18n:onboarding_glance_sample_time"),
		"demo.permission.ready":         a.translate("i18n:onboarding_permission_ready"),
		"demo.finish.title":             a.translate("i18n:onboarding_finish_card_title"),
		"demo.finish.badge":             a.translate("i18n:onboarding_finish_badge"),
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
	a.invalidateOnboardingWindow()
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
	_ = a.runOnUI("start onboarding permission check", func() {
		a.onboardingLoading = true
		a.invalidateOnboardingWindow()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	status, err := a.services.MacOSPermissionStatus(ctx, a.sessionID)
	cancel()
	_ = a.runOnUI("apply onboarding permission status", func() {
		a.onboardingLoading = false
		if err != nil {
			a.onboardingError = err.Error()
		} else {
			a.onboardingPermission = status
			a.onboardingError = ""
		}
		a.invalidateOnboardingWindow()
	})
}

func (a *App) openOnboardingPermission(permissionType string) {
	util.Go(a.lifecycleCtx, "open onboarding permission", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := a.services.OpenMacOSPermission(ctx, a.sessionID, permissionType)
		cancel()
		if err != nil {
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
