package view

import (
	"runtime"
	"testing"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestOnboardingViewExposesWindowAndChoiceOverlay(t *testing.T) {
	view := OnboardingView(OnboardingProps{
		Width: 1040, Height: 800, ActiveStep: 0, ChoiceKind: "language",
		Steps:   []OnboardingStep{{ID: "welcome", Title: "Welcome", Accent: woxui.Color{G: 200, A: 255}}},
		Labels:  map[string]string{"title": "Set up Wox", "subtitle": "Quick setup", "back": "Back", "next": "Next"},
		Choices: []OnboardingChoice{{Value: "en_US", Label: "English"}},
		Theme:   woxcomponent.Theme{},
	})
	root, ok := view.(woxwidget.Semantics)
	if !ok || root.AutomationID != "onboarding.window" {
		t.Fatalf("root = %#v, want onboarding window semantics", view)
	}
	stack, ok := root.Child.(woxwidget.Stack)
	if !ok || len(stack.Children) != 2 {
		t.Fatalf("root child = %#v, want body plus choice overlay", root.Child)
	}
	dropdown, ok := stack.Children[1].Child.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("choice overlay = %#v, want shared SettingsChoiceView", stack.Children[1].Child)
	}
	dropdownProps, ok := dropdown.Widget.(SettingsChoiceProps)
	if !ok || dropdownProps.ID != "onboarding-choice-picker" {
		t.Fatalf("choice props = %#v, want onboarding shared dropdown", dropdown.Widget)
	}
}

func TestOnboardingPageCentersTitleAndUsesStaticFeatureVisual(t *testing.T) {
	page := onboardingPage(OnboardingProps{
		Width: 1040, MainHotkeyLabels: []string{"Alt", "Space"}, HotkeyStatus: "Available",
		Labels: map[string]string{"mainHotkey.body": "Choose a hotkey.", "hotkey.change": "Click to record", "hotkey.preview": "Type to search"}, Theme: woxcomponent.Theme{},
	}, OnboardingStep{ID: "mainHotkey", Title: "Set hotkey"}, 660).(woxwidget.Container)
	content := page.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	title := content.Children[1].(woxwidget.TextBlock)
	if !title.Centered || title.Width != 880 {
		t.Fatalf("title = %#v, want centered 880-wide title", title)
	}
	gap := content.Children[2].(woxwidget.Container)
	if gap.Height != 12 {
		t.Fatalf("title gap = %v, want 12", gap.Height)
	}
	visual, ok := content.Children[len(content.Children)-1].(woxwidget.Align)
	if !ok {
		t.Fatalf("last page child = %#v, want centered feature visual", content.Children[len(content.Children)-1])
	}
	if visual.Vertical != 0.25 {
		t.Fatalf("feature visual vertical alignment = %v, want 0.25", visual.Vertical)
	}
	if _, ok := visual.Child.(woxwidget.Flex); !ok {
		t.Fatalf("feature visual = %#v, want onboarding-specific hotkey layout", visual.Child)
	}
	hotkeyVisual := visual.Child.(woxwidget.Flex)
	stageGap := hotkeyVisual.Children[len(hotkeyVisual.Children)-2].(woxwidget.Container)
	if stageGap.Height != 24 {
		t.Fatalf("hotkey demo stage gap = %v, want 24", stageGap.Height)
	}
	preview := hotkeyVisual.Children[len(hotkeyVisual.Children)-1].(woxwidget.Stack)
	if preview.Height != 232 {
		t.Fatalf("hotkey preview height = %v, want 232", preview.Height)
	}
	grid, ok := preview.Children[0].Child.(woxwidget.Painter)
	if !ok {
		t.Fatalf("hotkey preview backdrop = %T, want fading grid painter", preview.Children[0].Child)
	}
	if grid.Height != 232 || preview.Children[0].Top != 0 {
		t.Fatalf("hotkey grid = height %v top %v, want full-stage backdrop", grid.Height, preview.Children[0].Top)
	}
	queryPreview := onboardingQueryPreview(OnboardingProps{Theme: woxcomponent.Theme{
		ActionBackground: woxui.Color{R: 35, G: 35, B: 38, A: 255}, PreviewSplit: woxui.Color{R: 255, G: 255, B: 255, A: 40}, ResultTitle: woxui.Color{R: 245, G: 245, B: 247, A: 255},
	}}, woxui.Color{G: 184, A: 255}, 480, "Type to search", false).(woxwidget.Stack)
	if queryPreview.Width != 544 || queryPreview.Height != 104 {
		t.Fatalf("query preview = %vx%v, want 544x104 chrome", queryPreview.Width, queryPreview.Height)
	}
	chrome := queryPreview.Children[0].Child.(woxwidget.Painter)
	displayList := &woxui.DisplayList{}
	chrome.Paint(displayList, woxui.Rect{Width: chrome.Width, Height: chrome.Height})
	if displayList.CommandCount() != 10 {
		t.Fatalf("query preview chrome commands = %d, want ambient shadow, contact shadow, surface, and border", displayList.CommandCount())
	}
	if _, ok := DemoPreview(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "welcome"}, 640, 360).(woxwidget.LoopAnimation); !ok {
		t.Fatal("settings DemoPreview no longer exposes the preserved animated demo")
	}
}

func TestOnboardingWelcomeUsesSharedGridAndQueryPreview(t *testing.T) {
	visual := onboardingWelcomeVisual(OnboardingProps{
		Labels: map[string]string{
			"welcome.apps": "Apps", "welcome.files": "Files", "welcome.plugins": "Plugins", "welcome.ai": "AI", "welcome.hint": "A few steps remain",
		},
		Theme: woxcomponent.Theme{},
	}, 640, woxui.Color{G: 184, A: 255}).(woxwidget.Flex)
	stage := visual.Children[0].(woxwidget.Stack)
	if _, ok := stage.Children[0].Child.(woxwidget.Painter); !ok {
		t.Fatalf("welcome backdrop = %T, want shared fading grid painter", stage.Children[0].Child)
	}
	query := stage.Children[2].Child.(woxwidget.Align).Child.(woxwidget.Stack)
	if query.Width != 544 || query.Height != 104 {
		t.Fatalf("welcome query preview = %vx%v, want 544x104", query.Width, query.Height)
	}
	if hint := visual.Children[1].(woxwidget.Text); hint.Value != "A few steps remain" {
		t.Fatalf("welcome hint = %q", hint.Value)
	}
}

func TestOnboardingQueryPreviewUsesConfiguredGlance(t *testing.T) {
	preview := onboardingQueryPreview(OnboardingProps{
		GlanceEnabled: true, GlanceValue: "62%", Theme: woxcomponent.Theme{},
	}, woxui.Color{G: 184, A: 255}, 480, "setting", true).(woxwidget.Stack)
	query := preview.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	trailing := query.Children[len(query.Children)-1].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Container)
	text := trailing.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[1].(woxwidget.Text)
	if text.Value != "62%" {
		t.Fatalf("query glance value = %q, want configured value", text.Value)
	}
}

func TestOnboardingHotkeyConflictDisablesNext(t *testing.T) {
	footer := onboardingFooter(OnboardingProps{
		Width: 1040, NextDisabled: true, Steps: []OnboardingStep{{ID: "mainHotkey", Title: "Hotkey"}, {ID: "finish", Title: "Finish"}},
		Labels: map[string]string{"next": "Next", "finish": "Finish"}, Theme: woxcomponent.Theme{},
	}, 0).(woxwidget.Container)
	stack := footer.Child.(woxwidget.Stack)
	next := stack.Children[len(stack.Children)-1].Child.(woxwidget.Semantics)
	if !next.Disabled || len(next.Actions) != 0 {
		t.Fatalf("next semantics = %#v, want disabled while hotkey conflicts", next)
	}
	dots := stack.Children[0].Child.(woxwidget.Align).Child.(woxwidget.Flex)
	future := dots.Children[1].(woxwidget.Semantics)
	if !future.Disabled || len(future.Actions) != 0 {
		t.Fatalf("future progress dot = %#v, want disabled while hotkey conflicts", future)
	}
}

func TestOnboardingQueryHotkeyVisualShowsClipboardMapping(t *testing.T) {
	toggled := true
	selected := woxui.Color{R: 30, G: 60, B: 60, A: 255}
	title := woxui.Color{R: 240, G: 240, B: 240, A: 255}
	subtitle := woxui.Color{R: 160, G: 160, B: 160, A: 255}
	visual := onboardingQueryHotkeysVisual(OnboardingProps{
		QueryHotkeyLabels: []string{"Ctrl", "Shift", "V"}, QueryHotkeyStatus: "Available", QueryHotkeyReady: true, QueryHotkeySelected: true,
		GlanceEnabled: true, GlanceValue: "62%",
		Labels: map[string]string{
			"queryHotkeys.clipboard": "Clipboard", "queryHotkeys.status.title": "Query Hotkey",
			"queryHotkeys.status.body": "Add more in Settings", "queryHotkeys.configured": "1 configured", "queryHotkeys.notConfigured": "Not configured", "queryHotkeys.shortcut": "Clipboard search",
			"hotkey.change": "Click the hotkey to record another one",
		},
		Theme: woxcomponent.Theme{ResultTitle: title, ResultSubtitle: subtitle, SelectedBackground: selected}, OnToggleQueryHotkey: func(value bool) { toggled = value },
	}, 640, woxui.Color{G: 184, A: 255}).(woxwidget.Flex)
	showcase := visual.Children[0].(woxwidget.Flex)
	shortcutColumn := showcase.Children[0].(woxwidget.Flex)
	shortcut := shortcutColumn.Children[0].(woxwidget.Semantics)
	if shortcut.Label != "Ctrl + Shift + V" || shortcut.AutomationID != "onboarding.query_hotkey" {
		t.Fatalf("query shortcut = %#v", shortcut)
	}
	if caption := shortcutColumn.Children[1].(woxwidget.Text); caption.Value != "Clipboard search" {
		t.Fatalf("shortcut caption = %q", caption.Value)
	}
	if hint := shortcutColumn.Children[2].(woxwidget.Text); hint.Value != "Click the hotkey to record another one" {
		t.Fatalf("shortcut hint = %q", hint.Value)
	}
	window := showcase.Children[2].(woxwidget.Container)
	queryWindow := window.Child.(woxwidget.Flex)
	query := queryWindow.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Text)
	caret := queryWindow.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[1].(woxwidget.Container)
	accessory := queryWindow.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Container)
	accessoryText := accessory.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[1].(woxwidget.Text)
	firstResult := queryWindow.Children[1].(woxwidget.Align).Child.(woxwidget.Container)
	if query.Value != "cb" || caret.Width != 2 || caret.Height != 22 || accessoryText.Value != "62%" || firstResult.Color != selected || window.Width != 420 || window.Height != 188 {
		t.Fatalf("query demo = value %q caret %#v result %#v window %.0f", query.Value, caret, firstResult.Color, window.Height)
	}
	if spacer := visual.Children[1].(woxwidget.Container); spacer.Height != 100 {
		t.Fatalf("demo-to-divider spacing = %.0f, want 100", spacer.Height)
	}
	bottom := visual.Children[4].(woxwidget.Container).Child.(woxwidget.Flex)
	if icon := bottom.Children[0].(woxwidget.Container); icon.Color != settingsColorAlpha(title, 14) {
		t.Fatalf("keyboard background = %#v", icon.Color)
	}
	status := bottom.Children[3].(woxwidget.Flex)
	if label := status.Children[0].(woxwidget.Text); label.Value != "1 configured" || label.Color != subtitle {
		t.Fatalf("status order = %#v", status.Children)
	}
	checkbox := status.Children[1].(woxwidget.Semantics)
	if !checkbox.Checked {
		t.Fatal("query hotkey checkbox is not checked")
	}
	if err := checkbox.OnAction(woxui.AccessibilityActionToggle, ""); err != nil || toggled {
		t.Fatalf("toggle result = %v, value %v", err, toggled)
	}
}

func TestOnboardingHotkeyRecordingHighlightsKeyBorders(t *testing.T) {
	accent := woxui.Color{G: 184, A: 255}
	main := onboardingMainHotkeyVisual(OnboardingProps{
		MainHotkeyLabels: []string{"Alt", "Space"}, HotkeyRecording: true, Labels: map[string]string{}, Theme: woxcomponent.Theme{},
	}, 640, accent).(woxwidget.Flex)
	mainKeys := main.Children[0].(woxwidget.Align).Child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Flex)
	mainKey := mainKeys.Children[0].(woxwidget.Container)
	if mainKey.Width != 52 || mainKey.Height != 44 || mainKey.Radius != 7 || mainKey.BorderColor != accent || mainKey.BorderWidth != 2 {
		t.Fatalf("main recording key = %#v", mainKey)
	}

	query := onboardingQueryHotkeysVisual(OnboardingProps{
		QueryHotkeyLabels: []string{"Ctrl", "Shift", "V"}, QueryHotkeyRecording: true, Labels: map[string]string{}, Theme: woxcomponent.Theme{},
	}, 640, accent).(woxwidget.Flex)
	queryKeys := query.Children[0].(woxwidget.Flex).Children[0].(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Flex)
	queryKey := queryKeys.Children[0].(woxwidget.Container)
	if queryKey.BorderColor != accent || queryKey.BorderWidth != 2 {
		t.Fatalf("query recording border = %#v/%.0f", queryKey.BorderColor, queryKey.BorderWidth)
	}
}

func TestOnboardingPluginsVisualUsesStoreMetadataAndInstallActions(t *testing.T) {
	installedIcon := &woxui.Image{Width: 40, Height: 40}
	clickedID := ""
	visual := onboardingPluginsVisual(OnboardingProps{
		Plugins: []OnboardingPlugin{
			{ID: "awake", Name: "Awake", Description: "Keep your computer awake", Icon: installedIcon},
			{ID: "unsplash", Name: "Unsplash", Description: "Search images", Installed: true},
		},
		Labels: map[string]string{"plugins.install": "Install", "plugins.installing": "Installing", "plugins.installed": "Installed", "plugins.more": "More in Store"},
		Theme:  woxcomponent.Theme{}, OnInstallPlugin: func(id string) { clickedID = id },
	}, 640).(woxwidget.Flex)
	rows := visual.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	first := rows.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	icon := first.Children[0].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Image)
	button := first.Children[2].(woxwidget.Semantics)
	if icon.Source != installedIcon || icon.Width != 28 || icon.Height != 28 || button.Label != "Install" {
		t.Fatalf("plugin row = icon %#v button %#v", icon.Source, button)
	}
	focusedControlGesture(button).OnTap()
	if clickedID != "awake" {
		t.Fatalf("install action id = %q", clickedID)
	}
	second := rows.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	if status := second.Children[2].(woxwidget.Flex).Children[1].(woxwidget.Text); status.Value != "Installed" {
		t.Fatalf("installed status = %q", status.Value)
	}
}

func TestOnboardingQueryHotkeySelectedResultUsesSelectedForeground(t *testing.T) {
	theme := woxcomponent.Theme{
		ResultTitle: woxui.Color{R: 1, A: 255}, ResultSubtitle: woxui.Color{R: 2, A: 255},
		SelectedBackground: woxui.Color{R: 3, A: 255}, SelectedTitle: woxui.Color{R: 4, A: 255}, SelectedSubtitle: woxui.Color{R: 5, A: 255},
	}
	row := onboardingQueryHotkeyResult(OnboardingProps{Theme: theme}, 420, "Clipboard", "Clipboard history", true).(woxwidget.Align).Child.(woxwidget.Container)
	content := row.Child.(woxwidget.Flex)
	texts := content.Children[1].(woxwidget.Flex)
	if row.Color != theme.SelectedBackground || texts.Children[0].(woxwidget.Text).Color != theme.SelectedTitle || texts.Children[1].(woxwidget.Text).Color != theme.SelectedSubtitle {
		t.Fatalf("selected result colors = background %#v title %#v subtitle %#v", row.Color, texts.Children[0].(woxwidget.Text).Color, texts.Children[1].(woxwidget.Text).Color)
	}
	wantIcon := woxcomponent.CopyGlyph(18, theme.SelectedSubtitle).(woxwidget.Image)
	if icon := content.Children[0].(woxwidget.Image); icon.Source != wantIcon.Source {
		t.Fatal("selected result icon does not use selected subtitle color")
	}
}

func TestOnboardingThemeCardIsSelectableAndUsesThemePreview(t *testing.T) {
	selectedID := ""
	accent := woxui.Color{R: 20, G: 184, B: 166, A: 255}
	card := onboardingThemeCard(OnboardingProps{
		Theme: woxcomponent.Theme{}, ThemePreviewTitle: "wox",
		ThemePreviewTexts: []string{"Wox", "Wox Settings"}, ThemePreviewSubs: []string{"Launcher", "Settings"}, ThemePreviewOpen: "Open",
		OnSelectTheme: func(id string) { selectedID = id },
	}, OnboardingTheme{ID: "glass", Name: "Wox Glass Dark", Selected: true}, 180, accent).(woxwidget.Semantics)
	if !card.Selected {
		t.Fatal("selected theme card does not expose selected semantics")
	}
	container := focusedControlGesture(card).Child.(woxwidget.Container)
	if container.BorderColor != accent || container.BorderWidth != 1 {
		t.Fatalf("selected theme border = %#v width %.0f", container.BorderColor, container.BorderWidth)
	}
	if container.Height != 232 || container.Padding != woxwidget.UniformInsets(8) {
		t.Fatalf("theme card geometry = height %.0f padding %#v", container.Height, container.Padding)
	}
	cardContent := container.Child.(woxwidget.Stack)
	preview := cardContent.Children[0].Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	if len(preview.Children) != 2 {
		t.Fatalf("selected theme preview children = %d, want preview and check", len(preview.Children))
	}
	hitTarget := cardContent.Children[1].Child.(woxwidget.Gesture)
	if target := hitTarget.Child.(woxwidget.Container); target.Width != 164 || target.Height != 216 {
		t.Fatalf("theme hit target = %.0fx%.0f", target.Width, target.Height)
	}
	hitTarget.OnTap()
	if selectedID != "glass" {
		t.Fatalf("selected theme id = %q", selectedID)
	}
}

func TestOnboardingThemesUseTwoColumnGrid(t *testing.T) {
	visual := onboardingThemesVisual(OnboardingProps{
		Themes: []OnboardingTheme{{ID: "glass"}, {ID: "dark"}, {ID: "light"}, {ID: "auto"}},
		Theme:  woxcomponent.Theme{},
	}, 760, woxui.Color{R: 20, G: 184, B: 166, A: 255}).(woxwidget.Flex)
	grid := visual.Children[0].(woxwidget.Flex)
	if grid.Axis != woxwidget.Vertical || len(grid.Children) != 2 {
		t.Fatalf("theme grid axis = %v rows = %d", grid.Axis, len(grid.Children))
	}
	for index, child := range grid.Children {
		row := child.(woxwidget.Flex)
		if row.Axis != woxwidget.Horizontal || len(row.Children) != 2 {
			t.Fatalf("theme row %d axis = %v columns = %d", index, row.Axis, len(row.Children))
		}
	}
}

func TestOnboardingThemeStepUsesContinueLabel(t *testing.T) {
	footer := onboardingFooter(OnboardingProps{
		Width: 1040, Steps: []OnboardingStep{{ID: "themeInstall", Title: "Theme"}, {ID: "finish", Title: "Finish"}},
		Labels: map[string]string{"next": "Continue"}, Theme: woxcomponent.Theme{},
	}, 0).(woxwidget.Container)
	button := footer.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Semantics)
	if button.Label != "Continue" {
		t.Fatalf("theme step action = %q", button.Label)
	}
}

func TestOnboardingFinishVisualShowsSettingQueryAndConfiguredSummary(t *testing.T) {
	visual := onboardingFinishVisual(OnboardingProps{
		MainHotkeyLabels: []string{"Alt", "Space"}, GlanceEnabled: true, GlanceLabel: "Current time",
		Plugins: []OnboardingPlugin{{Name: "Awake", Installed: true}, {Name: "Everything", Installed: true}},
		Labels: map[string]string{
			"finish.query": "setting", "finish.hotkey": "Open hotkey", "finish.glance": "Glance",
			"finish.plugins": "Starter plugin", "finish.hint": "Change more in Settings",
		},
		Theme: woxcomponent.Theme{},
	}, 640, woxui.Color{G: 184, A: 255}).(woxwidget.Flex)
	queryStage := visual.Children[0].(woxwidget.Stack)
	if queryStage.Height != 224 {
		t.Fatalf("finish query stage height = %.0f, want 224", queryStage.Height)
	}
	if _, ok := queryStage.Children[0].Child.(woxwidget.Painter); !ok {
		t.Fatalf("finish query backdrop = %T, want fading grid painter", queryStage.Children[0].Child)
	}
	queryPreview := queryStage.Children[2].Child.(woxwidget.Align).Child.(woxwidget.Stack)
	query := queryPreview.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if text := query.Children[0].(woxwidget.Text); text.Value != "setting" {
		t.Fatalf("finish query = %q", text.Value)
	}
	if caret := query.Children[1].(woxwidget.Container); caret.Width != 2 || caret.Height != 24 {
		t.Fatalf("finish caret = %#v", caret)
	}
	rows := visual.Children[1].(woxwidget.Flex)
	if len(rows.Children) != 5 {
		t.Fatalf("finish summary children = %d, want three rows and two dividers", len(rows.Children))
	}
	pluginRow := rows.Children[4].(woxwidget.Container).Child.(woxwidget.Flex)
	pluginIcons := pluginRow.Children[2].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if len(pluginIcons.Children) != 2 {
		t.Fatalf("finish plugin icons = %d, want all two installed plugins", len(pluginIcons.Children))
	}
}

func TestOnboardingGlanceVisualIncludesLiveQueryBox(t *testing.T) {
	props := OnboardingProps{
		GlanceEnabled: true, GlanceValue: "09:41",
		Labels: map[string]string{
			"glance.query": "wox", "glance.enable": "Enable Glance", "glance.enable.body": "Show useful information", "glance.primary": "Primary Glance",
		},
		Theme: woxcomponent.Theme{
			QueryBackground: woxui.Color{R: 20, G: 21, B: 24, A: 255},
			ResultTitle:     woxui.Color{R: 240, G: 240, B: 240, A: 255},
			ResultSubtitle:  woxui.Color{R: 160, G: 160, B: 160, A: 255},
		},
	}
	visual := onboardingGlanceVisual(props, 640).(woxwidget.Flex)
	queryStage := visual.Children[0].(woxwidget.Stack)
	backdrop, ok := queryStage.Children[0].Child.(woxwidget.Painter)
	if !ok {
		t.Fatalf("glance query backdrop = %T, want fading grid painter", queryStage.Children[0].Child)
	}
	if queryStage.Children[0].Top != -176 || backdrop.Height != 332 {
		t.Fatalf("glance query backdrop = top %.0f height %.0f, want extended fade above QueryBox", queryStage.Children[0].Top, backdrop.Height)
	}
	queryBox := queryStage.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stack)
	if queryBox.Width != 616 || queryBox.Height != 96 {
		t.Fatalf("query box = %#v, want elevated 560-wide preview", queryBox)
	}
	query := queryBox.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(query.Children) != 2 {
		t.Fatalf("query children = %d, want query and Glance value without a divider", len(query.Children))
	}
	if divider := visual.Children[1].(woxwidget.Container); divider.Color.A != 48 {
		t.Fatalf("divider alpha = %d, want 48", divider.Color.A)
	}
	glanceRow := visual.Children[3].(woxwidget.Container).Child.(woxwidget.Flex)
	if icon := glanceRow.Children[0].(woxwidget.Container); icon.Width != 40 || icon.Height != 40 || icon.Color != settingsColorAlpha(props.Theme.ResultTitle, 14) {
		t.Fatalf("glance row icon = %#v", icon)
	}

	props.GlanceEnabled = false
	disabledStage := onboardingGlanceVisual(props, 640).(woxwidget.Flex).Children[0].(woxwidget.Stack)
	disabledBox := disabledStage.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stack)
	disabled := disabledBox.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(disabled.Children) != 1 {
		t.Fatalf("disabled query children = %d, want Glance value hidden", len(disabled.Children))
	}
}

func TestOnboardingPermissionsCenterCopyAndUseMonochromeIcons(t *testing.T) {
	card := onboardingPermissions(OnboardingProps{
		Theme: woxcomponent.Theme{ResultTitle: woxui.Color{R: 240, G: 240, B: 240, A: 255}, ResultSubtitle: woxui.Color{R: 160, G: 160, B: 160, A: 255}},
		Permissions: []OnboardingPermission{
			{ID: "accessibility", Title: "Accessibility", Description: "Read selected text."},
			{ID: "fullDiskAccess", Title: "Full Disk Access", Description: "Search protected folders."},
		},
		Labels: map[string]string{"permission.authorize": "Authorize"},
	}, 720, 172).(woxwidget.Container)
	row := card.Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	if row.Padding.Top != row.Padding.Bottom {
		t.Fatalf("permission row padding = %#v, want equal vertical inset so title and subtitle can center", row.Padding)
	}
	content := row.Child.(woxwidget.Flex)
	if content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatal("permission row does not center title and subtitle")
	}
	icon := content.Children[0].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Image)
	if icon.Source == nil {
		t.Fatal("permission icon is empty, want a monochrome SVG")
	}
	column := content.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	if column.Axis != woxwidget.Vertical {
		t.Fatalf("permission copy = %#v, want a naturally sized vertical stack", column)
	}
	if _, ok := column.Children[1].(woxwidget.TextBlock); !ok {
		t.Fatal("permission subtitle is missing from the centered copy stack")
	}
	color := woxui.Color{R: 240, G: 240, B: 240, A: 255}
	access := permissionIcon("accessibility", 20, color).(woxwidget.Image)
	disk := permissionIcon("fullDiskAccess", 20, color).(woxwidget.Image)
	folder := woxcomponent.FolderGlyph(20, color).(woxwidget.Image)
	if access.Source == nil || disk.Source == nil || access.Source == disk.Source {
		t.Fatal("permission icons should be distinct monochrome SVGs")
	}
	if disk.Source == folder.Source {
		t.Fatal("disk permission still uses the folder glyph")
	}
	status := content.Children[2].(woxwidget.Align)
	if status.Vertical != .5 {
		t.Fatalf("permission status align = %#v, want vertically centered in the row", status)
	}
}

func TestOnboardingHeaderAndFooterUseCompactChrome(t *testing.T) {
	accent := woxui.Color{R: 20, G: 184, B: 166, A: 255}
	props := OnboardingProps{
		Width: 1040, Height: 800, ActiveStep: 0,
		Steps:  []OnboardingStep{{ID: "welcome", Title: "Welcome", Accent: accent}, {ID: "finish", Title: "Finish", Accent: accent}},
		Labels: map[string]string{"title": "Set up Wox", "subtitle": "Quick setup", "back": "Back", "next": "Next"},
		Theme:  woxcomponent.Theme{Cursor: woxui.Color{R: 240, G: 240, B: 240, A: 255}},
	}
	header := onboardingHeader(props).(woxwidget.Container)
	if header.Height != OnboardingHeaderHeight {
		t.Fatalf("header height = %v, want %v", header.Height, OnboardingHeaderHeight)
	}
	footer := onboardingFooter(props, 0).(woxwidget.Container)
	stack := footer.Child.(woxwidget.Stack)
	if len(stack.Children) != 2 {
		t.Fatalf("first-step footer children = %d, want progress and continue only", len(stack.Children))
	}
	progress := stack.Children[0].Child.(woxwidget.Align).Child.(woxwidget.Flex)
	activeDot := progress.Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Align).Child.(woxwidget.Container)
	if activeDot.Color != accent {
		t.Fatalf("active progress color = %#v, want onboarding accent %#v", activeDot.Color, accent)
	}
}

func TestOnboardingHeaderStartsWindowDragging(t *testing.T) {
	dragged := false
	props := OnboardingProps{
		Width: 1040, Height: 800, ActiveStep: 0, OnDrag: func() { dragged = true },
		Steps:  []OnboardingStep{{ID: "welcome", Title: "Welcome"}},
		Labels: map[string]string{"title": "Set up Wox", "subtitle": "Quick setup"},
		Theme:  woxcomponent.Theme{},
	}
	header := onboardingHeader(props).(woxwidget.Stack)
	drag := header.Children[0].Child.(woxwidget.Gesture)
	drag.OnDragStart()
	if !dragged {
		t.Fatal("rail header did not start window dragging")
	}
	if drag.ID != "onboarding-header-drag" {
		t.Fatalf("header drag id = %q, want onboarding-header-drag", drag.ID)
	}
	if target := drag.Child.(woxwidget.Container); target.Width != 1040 || target.Height != OnboardingHeaderHeight {
		t.Fatalf("header drag size = %vx%v", target.Width, target.Height)
	}
}

func TestOnboardingDemoTimelinesMatchFlutterPhases(t *testing.T) {
	if got := onboardingDemoDuration("queryHotkeys"); got != 9200*time.Millisecond {
		t.Fatalf("query hotkey duration = %v, want 9.2s Flutter showcase", got)
	}
	for _, mode := range []string{"queryHotkeysNormal", "queryHotkeysWebPanel", "queryHotkeysSilent"} {
		if got := onboardingDemoDuration(mode); got != 4600*time.Millisecond {
			t.Fatalf("%s duration = %v, want 4.6s Flutter preset demo", mode, got)
		}
	}
	if got := onboardingSelectionWindowProgress(.96); got >= 1 || got <= 0 {
		t.Fatalf("selection window exit progress = %v, want in-flight exit", got)
	}
}

func TestOnboardingHotkeyDemosDoNotOverlapWindows(t *testing.T) {
	for progress := float32(0); progress <= 1; progress += .01 {
		if onboardingMainHotkeyProgress(progress) > .01 && onboardingMainWindowProgress(progress) > .01 {
			t.Fatalf("main hotkey and launcher both visible at %.2f", progress)
		}
		if onboardingSelectionHotkeyProgress(progress) > .01 && onboardingSelectionWindowProgress(progress) > .01 {
			t.Fatalf("selection hotkey and launcher both visible at %.2f", progress)
		}
		if onboardingSelectionCursorOpacity(progress) > .01 && onboardingSelectionWindowProgress(progress) > .01 {
			t.Fatalf("selection cursor and launcher both visible at %.2f", progress)
		}
		if onboardingQueryHotkeyExample1Shortcut(progress) > .01 && onboardingQueryHotkeyExample1Window(progress) > .01 {
			t.Fatalf("query hotkey example 1 shortcut and launcher both visible at %.2f", progress)
		}
		if onboardingQueryHotkeyExample2Shortcut(progress) > .01 && onboardingQueryHotkeyExample2Window(progress) > .01 {
			t.Fatalf("query hotkey example 2 shortcut and window both visible at %.2f", progress)
		}
		if onboardingQueryHotkeySilentShortcut(progress) > .01 && onboardingQueryHotkeySilentToast(progress) > .01 {
			t.Fatalf("silent query hotkey and toast both visible at %.2f", progress)
		}
	}
}

func TestOnboardingSelectionWindowMirrorsLiveSelectionQuery(t *testing.T) {
	window := onboardingSelectionWindow(OnboardingProps{
		Theme:  woxcomponent.Theme{QueryText: woxui.Color{A: 255}},
		Labels: map[string]string{"demo.selection.preview": "Preview"},
	}, OnboardingStep{}, 640, 330, 1).(woxwidget.Clip)
	children := window.Child.(woxwidget.Stack).Children
	query := children[2].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	querySlot := query.Children[0].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if len(querySlot.Children) != 1 {
		t.Fatalf("selection query parts = %d, want a caret only", len(querySlot.Children))
	}
	row := children[3].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(row.Children) != 2 {
		t.Fatalf("selected result children = %d, want icon and text without a hotkey tail", len(row.Children))
	}
	preview := children[len(children)-3]
	if preview.Left <= 10 {
		t.Fatalf("selection preview left = %v, want a right-hand preview pane", preview.Left)
	}
}

func TestOnboardingSelectionPreviewKeepsGapBelowTags(t *testing.T) {
	const width, height float32 = 280, 180
	preview := onboardingSelectionPreview(OnboardingProps{}, width, height, 1, "Quarterly plan.pdf", "/tmp/file.pdf").(woxwidget.Container)
	if preview.BorderWidth != 0 {
		t.Fatal("tags belong below the preview surface, not inside its border")
	}
	if preview.Padding.Bottom != 10 {
		t.Fatalf("padding below tags = %v, want 10", preview.Padding.Bottom)
	}
	flex := preview.Child.(woxwidget.Flex)
	if flex.Gap != 10 || len(flex.Children) != 2 {
		t.Fatalf("preview body/tags = gap %v children %d, want a 10px gap and two children", flex.Gap, len(flex.Children))
	}
	surface := flex.Children[0].(woxwidget.Container)
	if surface.BorderWidth != 1 {
		t.Fatalf("file surface border = %v, want a bordered preview body", surface.BorderWidth)
	}
	if got, want := surface.Height+flex.Gap+26+preview.Padding.Top+preview.Padding.Bottom, height; got != want {
		t.Fatalf("preview vertical layout = %v, want %v so tags keep 10px below them", got, want)
	}
}

func TestOnboardingTypedQueryDemosShowResultsAfterQuery(t *testing.T) {
	labels := map[string]string{
		"demo.finish.settings":        "Open Wox Settings",
		"demo.finish.system_settings": "Open System Settings",
	}
	cases := []struct {
		name     string
		query    string
		start    float32
		duration time.Duration
		build    func(progress float32) woxwidget.Clip
	}{
		{"welcome", "wpm install everything", .28, onboardingDemoDuration("welcome"), func(progress float32) woxwidget.Clip {
			return onboardingWelcomeDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "welcome"}, 640, 360, progress).(woxwidget.Clip)
		}},
		{"queryShortcuts", "gh repo", .18, onboardingDemoDuration("queryShortcuts"), func(progress float32) woxwidget.Clip {
			return onboardingQueryShortcutsDemo(OnboardingProps{Theme: woxcomponent.Theme{}, Labels: labels}, OnboardingStep{ID: "queryShortcuts"}, 640, 360, progress).(woxwidget.Clip)
		}},
		{"wpmInstall", "wpm install", .50, onboardingDemoDuration("wpmInstall"), func(progress float32) woxwidget.Clip {
			return onboardingPluginStoreDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "wpmInstall"}, 640, 360, progress).(woxwidget.Clip)
		}},
		{"themeInstall", "theme ocean dark", .08, onboardingDemoDuration("themeInstall"), func(progress float32) woxwidget.Clip {
			return onboardingThemeInstallDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "themeInstall"}, 640, 360, progress).(woxwidget.Clip)
		}},
		{"finish", "setting", .16, onboardingDemoDuration("finish"), func(progress float32) woxwidget.Clip {
			return onboardingFinishDemo(OnboardingProps{Theme: woxcomponent.Theme{}, Labels: labels}, OnboardingStep{ID: "finish"}, 640, 360, progress).(woxwidget.Clip)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			done := demoTypedQueryDoneProgress(tc.query, tc.start, tc.duration)
			mid := (tc.start + done) / 2
			typing := tc.build(mid)
			_, _, typingWindow := onboardingPlacedLauncherSlot(typing)
			if typingWindow.Height != 113 {
				t.Fatalf("height while typing = %.0f at %.2f, want query plus toolbar before results", typingWindow.Height, mid)
			}
			resultsProgress := min(float32(1), done+.08)
			doneDemo := tc.build(resultsProgress)
			_, _, doneWindow := onboardingPlacedLauncherSlot(doneDemo)
			if doneWindow.Height <= typingWindow.Height {
				t.Fatalf("height after query = %.0f at %.2f, want results to grow the window", doneWindow.Height, resultsProgress)
			}
		})
	}
}

func TestOnboardingMainHotkeyDemoShowsCompletedQueryWithResults(t *testing.T) {
	demo := onboardingMainHotkeyDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "mainHotkey"}, 640, 360, .72).(woxwidget.Clip)
	_, _, window := onboardingPlacedLauncherSlot(demo)
	query := window.Child.(woxwidget.Stack).Children[2].Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Text)
	if query.Value != "app" {
		t.Fatalf("main hotkey query = %q, want a completed app query with no typing", query.Value)
	}
	if window.Height <= 113 {
		t.Fatalf("main hotkey height = %.0f, want results already visible when the window opens", window.Height)
	}
}

func TestOnboardingDemoQueriesUseSharedFastTypingSpeed(t *testing.T) {
	const start = float32(.2)
	for _, duration := range []time.Duration{4400 * time.Millisecond, 5600 * time.Millisecond, 7000 * time.Millisecond} {
		progress := start + float32(175*time.Millisecond)/float32(duration)
		if got := demoTypedQuery("abcdef", progress, start, duration); got != "abc" {
			t.Fatalf("typed query after 175ms with %v timeline = %q, want three characters", duration, got)
		}
	}
	if demoQueryTypingInterval > 65*time.Millisecond {
		t.Fatalf("query typing interval = %v, want no slower than the Flutter fast reference", demoQueryTypingInterval)
	}
}

func TestOnboardingDemoDesktopUsesLoadedWallpaper(t *testing.T) {
	wallpaper := &woxui.Image{Width: 1800, Height: 840}
	desktop := onboardingDemoDesktop(OnboardingProps{Wallpaper: wallpaper}, OnboardingStep{}, 640, 360, false, nil)
	clip := desktop.(woxwidget.Clip)
	stack := clip.Child.(woxwidget.Stack)
	if clip.Width != 640 || clip.Height != 360 {
		t.Fatalf("desktop clip = %v x %v, want demo bounds", clip.Width, clip.Height)
	}
	background := stack.Children[0].Child.(woxwidget.Container)
	if background.Radius != 8 {
		t.Fatalf("desktop base radius = %v, want 8", background.Radius)
	}
	image, ok := stack.Children[1].Child.(woxwidget.Image)
	if !ok || image.Source != wallpaper || image.Radius != 8 {
		t.Fatalf("desktop wallpaper = %#v, want loaded image", stack.Children[1].Child)
	}
	overlay := stack.Children[2].Child.(woxwidget.Container)
	if overlay.Radius != 8 {
		t.Fatalf("desktop wallpaper overlay radius = %v, want 8", overlay.Radius)
	}
}

func TestOnboardingDemoDesktopUsesBlackBeforeWallpaperLoads(t *testing.T) {
	desktop := onboardingDemoDesktop(OnboardingProps{}, OnboardingStep{}, 640, 360, false, nil)
	clip := desktop.(woxwidget.Clip)
	stack := clip.Child.(woxwidget.Stack)
	background, ok := stack.Children[0].Child.(woxwidget.Container)
	if !ok || background.Color != (woxui.Color{A: 255}) || background.Radius != 8 {
		t.Fatalf("desktop background = %#v, want opaque black", stack.Children[0].Child)
	}
	chrome := stack.Children[len(stack.Children)-1].Child
	if runtime.GOOS == "darwin" {
		menuBar, ok := chrome.(woxwidget.Clip)
		if !ok || menuBar.Width != 640 || menuBar.Height != 28 {
			t.Fatalf("macOS menu bar = %#v, want a 28px clip of the rounded desktop", chrome)
		}
		return
	}
	container, ok := chrome.(woxwidget.Container)
	if !ok || container.Radius != 8 {
		t.Fatalf("desktop chrome = %#v, want radius 8 to match wallpaper corners", chrome)
	}
}

func TestOnboardingWindowsTaskbarUsesCenteredAppsAndSystemTray(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Windows taskbar only")
	}
	desktop := onboardingDemoDesktop(OnboardingProps{}, OnboardingStep{}, 640, 360, false, nil).(woxwidget.Clip)
	stack := desktop.Child.(woxwidget.Stack)
	taskbar := stack.Children[len(stack.Children)-1].Child.(woxwidget.Container)
	if taskbar.Height != 42 || taskbar.Radius != 8 || taskbar.Color.A != 198 {
		t.Fatalf("taskbar surface = %#v, want translucent 42px rounded bar", taskbar)
	}
	content := taskbar.Child.(woxwidget.Stack)
	if len(content.Children) != 2 || !content.Children[1].AnchorRight {
		t.Fatalf("taskbar layout = %#v, want centered apps plus right tray", content.Children)
	}
	center := content.Children[0].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(center.Children) != 6 || content.Children[0].Left <= 0 {
		t.Fatalf("centered taskbar apps = %d at left %v, want six centered icons without the Wox pin", len(center.Children), content.Children[0].Left)
	}
	tray := content.Children[1].Child.(woxwidget.Align)
	if tray.Width != 180 || tray.Horizontal != 1 {
		t.Fatalf("system tray = %#v, want right-aligned 180px tray", tray)
	}
}

func TestOnboardingTrayQueriesWindowSitsAgainstTrayChrome(t *testing.T) {
	demo := onboardingTrayQueriesDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{}, 640, 360, .80).(woxwidget.Clip)
	desktop := demo.Child.(woxwidget.Stack)
	slot := desktop.Children[len(desktop.Children)-1]
	window := slot.Child.(woxwidget.Clip)
	trayX := float32(640 - 88)
	if runtime.GOOS != "darwin" {
		trayX = 640 - 120
	}
	left, top := onboardingTrayWindowOrigin(640, 360, window.Width, window.Height, trayX)
	if slot.Left != left || slot.Top != top {
		t.Fatalf("tray window origin = %v/%v, want %v/%v against the tray chrome", slot.Left, slot.Top, left, top)
	}
	if runtime.GOOS == "darwin" && slot.Top != onboardingDemoDesktopChromeTop()+onboardingTrayWindowGap {
		t.Fatalf("tray window top = %v, want below the menu bar", slot.Top)
	}
	if runtime.GOOS != "darwin" && slot.Top+window.Height+onboardingTrayWindowGap != 360-onboardingDemoDesktopChromeBottom() {
		t.Fatalf("tray window bottom = %v, want above the taskbar", slot.Top+window.Height)
	}
}

func TestOnboardingWelcomeDemoGrowsDownIntoCenteredSlot(t *testing.T) {
	collapsed := onboardingWelcomeDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "welcome"}, 640, 360, .40).(woxwidget.Clip)
	expanded := onboardingWelcomeDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "welcome"}, 640, 360, 1).(woxwidget.Clip)
	collapsedSlot, collapsedFrame, collapsedWindow := onboardingPlacedLauncherSlot(collapsed)
	expandedSlot, expandedFrame, expandedWindow := onboardingPlacedLauncherSlot(expanded)
	contentTop := onboardingDemoDesktopChromeTop()
	contentHeight := float32(360) - contentTop - onboardingDemoDesktopChromeBottom()

	if collapsedSlot.Top != contentTop || expandedSlot.Top != contentTop {
		t.Fatalf("welcome launcher chrome top = %v/%v, want %v", collapsedSlot.Top, expandedSlot.Top, contentTop)
	}
	collapsedAlign := collapsedSlot.Child.(woxwidget.Align)
	if collapsedAlign.Width != 640 || collapsedAlign.Height != contentHeight || collapsedAlign.Horizontal != .5 || collapsedAlign.Vertical != .5 {
		t.Fatalf("welcome launcher desktop align = %#v, want centered in the desktop below chrome", collapsedAlign)
	}
	if collapsedFrame.Height != expandedFrame.Height || collapsedFrame.Vertical != 0 || expandedFrame.Vertical != 0 {
		t.Fatalf("welcome launcher frame = %#v / %#v, want a stable top-aligned expanded slot", collapsedFrame, expandedFrame)
	}
	if collapsedWindow.Height != 113 {
		t.Fatalf("welcome demo height before results = %.0f, want query plus toolbar only", collapsedWindow.Height)
	}
	if expandedWindow.Height != expandedFrame.Height || expandedWindow.Height <= collapsedWindow.Height {
		t.Fatalf("welcome demo height after results = %.0f in frame %.0f, want to fill the reserved slot by growing downward", expandedWindow.Height, expandedFrame.Height)
	}
}

func TestOnboardingWelcomeDemoFadesConceptCardAsLauncherAppears(t *testing.T) {
	demo := onboardingWelcomeDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "welcome"}, 640, 360, .24).(woxwidget.Clip)
	desktop := demo.Child.(woxwidget.Stack)
	card := desktop.Children[len(desktop.Children)-2]
	_, _, window := onboardingPlacedLauncherSlot(demo)
	if card.Top != 360*.24 {
		t.Fatalf("concept card top = %v, want a stationary fade at 24%% of desktop height", card.Top)
	}
	if window.Height != 113 {
		t.Fatalf("launcher during concept fade = %.0f, want query plus toolbar while the card fades out", window.Height)
	}
}

func TestOnboardingLauncherDemosShareCenteredDownwardSlot(t *testing.T) {
	demos := []woxwidget.Clip{
		onboardingPermissionsDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{}, 640, 360).(woxwidget.Clip),
		onboardingFinishDemo(OnboardingProps{Theme: woxcomponent.Theme{}, Labels: map[string]string{}}, OnboardingStep{}, 640, 360, 1).(woxwidget.Clip),
		onboardingQueryShortcutsDemo(OnboardingProps{Theme: woxcomponent.Theme{}, Labels: map[string]string{}}, OnboardingStep{}, 640, 360, 1).(woxwidget.Clip),
	}
	for _, demo := range demos {
		slot, frame, window := onboardingPlacedLauncherSlot(demo)
		align := slot.Child.(woxwidget.Align)
		if align.Horizontal != .5 || align.Vertical != .5 || frame.Vertical != 0 || window.Height != frame.Height {
			t.Fatalf("launcher placement = slot %#v frame %#v height %.0f, want a centered expanded slot with downward growth", align, frame, window.Height)
		}
	}
}

func onboardingPlacedLauncherSlot(demo woxwidget.Clip) (woxwidget.StackChild, woxwidget.Align, woxwidget.Clip) {
	desktop := demo.Child.(woxwidget.Stack)
	slot := desktop.Children[len(desktop.Children)-1]
	frame := slot.Child.(woxwidget.Align).Child.(woxwidget.Align)
	return slot, frame, frame.Child.(woxwidget.Clip)
}

func TestOnboardingDemoPreservesThemeTransparency(t *testing.T) {
	transparent := woxui.Color{R: 255, G: 255, B: 255, A: 0}
	backdrop := &woxui.Image{Width: 702, Height: 344}
	window := onboardingDemoWindow(onboardingDemoWindowProps{
		Width: 400, Height: 260, Backdrop: backdrop, Opacity: 1, ShowQuery: true,
		Theme:   woxcomponent.Theme{Background: woxui.Color{R: 22, G: 22, B: 26, A: 133}, QueryBackground: transparent, SelectedBackground: woxui.Color{R: 255, G: 255, B: 255, A: 36}},
		Results: []onboardingDemoResult{{Title: "Everything", Selected: true}},
	})
	stack := window.(woxwidget.Clip).Child.(woxwidget.Stack)
	if image := stack.Children[0].Child.(woxwidget.Image); image.Source != backdrop || image.Fit != woxwidget.ImageFitCover {
		t.Fatal("demo window did not cover the blurred wallpaper")
	}
	if query := stack.Children[2].Child.(woxwidget.Container); query.Color.A != 0 {
		t.Fatalf("rendered query alpha = %d, want theme alpha 0", query.Color.A)
	}
	if result := stack.Children[3].Child.(woxwidget.Container); result.Color.A != 36 {
		t.Fatalf("rendered selected result alpha = %d, want theme alpha 36", result.Color.A)
	}
}

func TestOnboardingDemoHintCardTextIsCentered(t *testing.T) {
	card := onboardingDemoHintCard(
		OnboardingProps{Theme: woxcomponent.Theme{}},
		OnboardingStep{},
		"Query Hotkeys",
		"Cmd+Shift+G",
		"github repo",
		580,
		255,
	).(woxwidget.Container)
	content := card.Child.(woxwidget.Flex)
	title := content.Children[0].(woxwidget.Expanded).Child.(woxwidget.Align)
	badge := content.Children[1].(woxwidget.Container)
	expansion := badge.Child.(woxwidget.Align)
	label := expansion.Child.(woxwidget.TextBlock)

	if title.Vertical != .5 || expansion.Vertical != .5 || badge.Padding.Top != 0 {
		t.Fatalf("hint alignment: title=%#v expansion=%#v padding=%#v", title, expansion, badge.Padding)
	}
	if expansion.Horizontal != .5 || expansion.Width != badge.Width-20 || !label.Centered || label.Width != expansion.Width {
		t.Fatalf("hint badge text = align %#v label %#v, want horizontally centered in the badge", expansion, label)
	}
}

func TestOnboardingPluginStoreUsesSharedWindowMetrics(t *testing.T) {
	window := onboardingPluginStoreWindow(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{}, 700, 320, "wpm install", "Install", 1, 1).(woxwidget.Clip)
	children := window.Child.(woxwidget.Stack).Children
	query := children[2].Child.(woxwidget.Container)
	result := children[3].Child.(woxwidget.Container)
	toolbar := children[len(children)-2].Child.(woxwidget.Container)

	if query.Height != 55 || result.Height != 56 || toolbar.Height != 40 {
		t.Fatalf("plugin store metrics = query %v, result %v, toolbar %v", query.Height, result.Height, toolbar.Height)
	}
}

func TestOnboardingPluginStorePreviewFillsToToolbar(t *testing.T) {
	window := onboardingPluginStoreWindow(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{}, 700, 320, "wpm install", "Install", 1, 1).(woxwidget.Clip)
	children := window.Child.(woxwidget.Stack).Children
	preview := children[len(children)-3]
	toolbar := children[len(children)-2]
	previewClip := preview.Child.(woxwidget.Clip)
	detail := previewClip.Child.(woxwidget.Container)

	if detail.Height != previewClip.Height {
		t.Fatalf("plugin store preview height = %v, want %v to fill the live preview pane", detail.Height, previewClip.Height)
	}
	if gap := toolbar.Top - (preview.Top + previewClip.Height); gap != 10 {
		t.Fatalf("plugin store preview/toolbar gap = %.0f, want 10px app padding above the toolbar", gap)
	}
}

func TestOnboardingPluginStoreWindowFitsAboveDesktopChrome(t *testing.T) {
	const width, height = float32(640), float32(360)
	demo := onboardingPluginStoreDemo(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "wpmInstall"}, width, height, 1).(woxwidget.Clip)
	slot, frame, window := onboardingPlacedLauncherSlot(demo)
	areaTop := onboardingDemoHintContentTop()
	areaHeight := onboardingDemoDesktopContentBottom(height) - areaTop

	if slot.Top != areaTop {
		t.Fatalf("plugin store slot top = %v, want %v below the hint card", slot.Top, areaTop)
	}
	align := slot.Child.(woxwidget.Align)
	if align.Height != areaHeight {
		t.Fatalf("plugin store desktop align height = %v, want %v above the desktop chrome", align.Height, areaHeight)
	}
	if window.Height > areaHeight || frame.Height > areaHeight {
		t.Fatalf("plugin store window %v overflows the %v desktop content area", window.Height, areaHeight)
	}
	if toolbarTop := window.Height - 40; toolbarTop+40 > areaHeight {
		t.Fatalf("plugin store toolbar bottom = %v, want at or above the desktop chrome", toolbarTop+40)
	}
}

func TestOnboardingFinishDemoTypesSettingToOpenSettings(t *testing.T) {
	labels := map[string]string{
		"demo.finish.settings":        "Open Wox Settings",
		"demo.finish.system_settings": "Open System Settings",
	}
	step := OnboardingStep{ID: "finish"}
	typing := onboardingFinishDemo(OnboardingProps{Theme: woxcomponent.Theme{}, Labels: labels}, step, 640, 360, .20).(woxwidget.Clip)
	_, _, typingWindow := onboardingPlacedLauncherSlot(typing)
	typingQuery := typingWindow.Child.(woxwidget.Stack).Children[2].Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Text)
	if typingQuery.Value == "" || typingQuery.Value == "setting" {
		t.Fatalf("finish query while typing = %q, want a partial setting", typingQuery.Value)
	}
	if typingWindow.Height != 113 {
		t.Fatalf("finish height while typing = %.0f, want query plus toolbar before results", typingWindow.Height)
	}

	demo := onboardingFinishDemo(OnboardingProps{Theme: woxcomponent.Theme{}, Labels: labels}, step, 640, 360, 1).(woxwidget.Clip)
	_, _, window := onboardingPlacedLauncherSlot(demo)
	children := window.Child.(woxwidget.Stack).Children
	query := children[2].Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Text)
	if query.Value != "setting" {
		t.Fatalf("finish query = %q, want setting", query.Value)
	}
	if window.Height <= typingWindow.Height {
		t.Fatalf("finish height after typing = %.0f, want results to grow the window", window.Height)
	}

	var titles []string
	for _, child := range children[3 : len(children)-2] {
		row := child.Child.(woxwidget.Container)
		rowLabels := row.Child.(woxwidget.Flex).Children[1].(woxwidget.Clip).Child.(woxwidget.Align).Child.(woxwidget.Flex)
		if len(rowLabels.Children) != 1 {
			t.Fatalf("finish result labels = %#v, want title only", rowLabels.Children)
		}
		titles = append(titles, rowLabels.Children[0].(woxwidget.Text).Value)
	}
	if len(titles) != 2 || titles[0] != "Open Wox Settings" || titles[1] != "Open System Settings" {
		t.Fatalf("finish titles = %v, want Wox and system settings", titles)
	}
}

func TestOnboardingMacDesktopUsesNativeMenuAndCursorGeometry(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS menu bar only")
	}
	desktop := onboardingDemoDesktop(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{}, 640, 360, false, nil).(woxwidget.Clip)
	menuBar := desktop.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Clip)
	menu := menuBar.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Container).Child.(woxwidget.Stack)
	search := menu.Children[1].Child.(woxwidget.Image)
	timeSlot := menu.Children[2]
	cursor := onboardingDemoCursor(1).(woxwidget.Painter)

	if menuBar.Width != 640 || menuBar.Height != 28 {
		t.Fatalf("macOS menu bar clip = %vx%v, want a 28px strip clipped to the desktop", menuBar.Width, menuBar.Height)
	}
	if search.Source == nil || search.Width != 16 || timeSlot.Left-(menu.Children[1].Left+search.Width) != 12 || cursor.Width != 22 || cursor.Height != 30 {
		t.Fatalf("mac desktop geometry = search %#v, time left %v, cursor %#v", search, timeSlot.Left, cursor)
	}
}

func TestOnboardingMacMenuBarFillStaysInsideDesktopCorners(t *testing.T) {
	const width, height = 64, 28
	color := woxui.Color{R: 220, G: 40, B: 40, A: 255}
	fill := onboardingDemoMacMenuBarFill(width, height, color).(woxwidget.Painter)
	displayList := &woxui.DisplayList{}
	displayList.Clear(woxui.Color{})
	displayList.PushClipRect(woxui.Rect{Width: width, Height: height})
	fill.Paint(displayList, woxui.Rect{Width: width, Height: height})
	displayList.PopClipRect()

	renderer, err := woxui.NewSoftwareRenderer(width, height)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Render(displayList); err != nil {
		t.Fatal(err)
	}
	img := renderer.RGBA()
	alphaAt := func(x, y int) uint8 { return img.Pix[y*img.Stride+x*4+3] }
	redAt := func(x, y int) uint8 { return img.Pix[y*img.Stride+x*4] }
	if got := alphaAt(0, 0); got != 0 {
		t.Fatalf("top-left desktop corner alpha = %d, want empty outside the rounded clip", got)
	}
	if got := redAt(width/2, height/2); got < 200 {
		t.Fatalf("menu bar interior red = %d, want the filled bar", got)
	}
	if got := redAt(0, height-1); got < 200 {
		t.Fatalf("menu bar bottom-left red = %d, want a square bottom edge", got)
	}
}

func TestOnboardingGlanceRendersQueryAccessoryWhenEnabled(t *testing.T) {
	demo := onboardingGlanceDemo(OnboardingProps{
		GlanceEnabled: true,
		GlanceLabel:   "CPU",
		GlanceValue:   "62%",
		Labels:        map[string]string{"demo.glance.value": "当前时间", "glance.body": "Status", "demo.glance.provider": "Provider", "glance.enable.body": "Body", "glance.primary": "Glance item"},
		Theme:         woxcomponent.Theme{},
	}, OnboardingStep{}, 640, 360).(woxwidget.Clip)
	_, frame, windowClip := onboardingPlacedLauncherSlot(demo)
	window := windowClip.Child.(woxwidget.Stack)
	query := window.Children[2].Child.(woxwidget.Container).Child.(woxwidget.Flex)

	if frame.Vertical != 0 {
		t.Fatalf("glance launcher frame vertical = %v, want top-aligned downward growth", frame.Vertical)
	}
	if len(query.Children) != 2 {
		t.Fatalf("glance query children = %d, want query text plus accessory", len(query.Children))
	}
	accessoryBox := query.Children[1].(woxwidget.Container)
	wantWidth := onboardingGlanceAccessoryWidth("62%", true)
	if accessoryBox.Width != wantWidth {
		t.Fatalf("glance accessory width = %v, want content-sized slot %v", accessoryBox.Width, wantWidth)
	}
	if wantWidth >= 100 {
		t.Fatalf("glance accessory width = %v, want tighter than the old 100px minimum", wantWidth)
	}
	if accessoryBox.Padding != (woxwidget.Insets{Left: 8, Right: 8}) {
		t.Fatalf("glance accessory padding = %+v, want 8px horizontal insets", accessoryBox.Padding)
	}
	accessoryAlign := accessoryBox.Child.(woxwidget.Align)
	if accessoryAlign.Horizontal != 1 || accessoryAlign.Vertical != .5 {
		t.Fatalf("glance accessory alignment = (%v, %v), want right-aligned", accessoryAlign.Horizontal, accessoryAlign.Vertical)
	}
	accessoryFlex := accessoryAlign.Child.(woxwidget.Flex)
	accessoryText := accessoryFlex.Children[1].(woxwidget.Text)
	if accessoryText.Value != "62%" {
		t.Fatalf("glance accessory text = %q, want live glance value", accessoryText.Value)
	}
	if accessoryText.Style.Size != woxcomponent.GlanceFontSize {
		t.Fatalf("glance accessory font = %v, want GlanceFontSize", accessoryText.Style.Size)
	}
}

func TestOnboardingGlanceAccessoryWidthStaysContentSized(t *testing.T) {
	width := onboardingGlanceAccessoryWidth("21:24", true)
	if width >= 100 || width > onboardingGlanceAccessoryWidth("62%", true)+30 {
		t.Fatalf("time glance width = %v, want a short content-sized slot", width)
	}
}

func TestOnboardingGlanceUsesCompactInlineSettings(t *testing.T) {
	settings := onboardingGlance(OnboardingProps{
		GlanceEnabled: true, GlanceLabel: "CPU", GlanceValue: "62%",
		Labels: map[string]string{"glance.enable": "Glance", "glance.enable.body": "Status", "glance.primary": "Primary"},
		Theme:  woxcomponent.Theme{},
	}, 720, 150).(woxwidget.Container)
	row := settings.Child.(woxwidget.Flex)
	controls := row.Children[3].(woxwidget.Flex)
	semantics := controls.Children[1].(woxwidget.Semantics)
	trigger := focusedControlGesture(semantics).Child.(woxwidget.Container)
	content := trigger.Child.(woxwidget.Flex)

	if settings.Color.A != 0 || settings.BorderWidth != 0 || trigger.Width != 220 {
		t.Fatalf("glance settings = surface %#v, border %v, dropdown width %v", settings.Color, settings.BorderWidth, trigger.Width)
	}
	if len(content.Children) == 0 {
		t.Fatal("glance dropdown has no content")
	}
	disabled := onboardingGlance(OnboardingProps{
		GlanceLabel: "CPU", Labels: map[string]string{"glance.enable": "Glance", "glance.enable.body": "Status", "glance.primary": "Primary"}, Theme: woxcomponent.Theme{},
	}, 720, 150).(woxwidget.Container).Child.(woxwidget.Flex).Children[3].(woxwidget.Flex)
	if len(disabled.Children) != 2 {
		t.Fatalf("disabled Glance controls = %d, want stable switch and dropdown", len(disabled.Children))
	}
}
