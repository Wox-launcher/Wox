package view

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	OnboardingHeaderHeight = float32(68)
	OnboardingFooterHeight = float32(72)
)

// OnboardingStep describes one first-run page and its demo accent.
type OnboardingStep struct {
	ID     string
	Title  string
	Accent woxui.Color
}

// OnboardingChoice describes one language or Glance option.
type OnboardingChoice struct {
	Value    string
	Label    string
	Leading  *woxui.Image
	Trailing string
}

// OnboardingPermission describes one passive permission check and action.
type OnboardingPermission struct {
	ID          string
	Title       string
	Description string
	Ready       bool
}

// OnboardingPlugin describes one recommended Store plugin and its install state.
type OnboardingPlugin struct {
	ID          string
	Name        string
	Description string
	Icon        *woxui.Image
	Installed   bool
	Installing  bool
	Disabled    bool
}

// OnboardingTheme describes one installed system theme and its resolved preview palettes.
type OnboardingTheme struct {
	ID                string
	Name              string
	Selected          bool
	IsAuto            bool
	PreviewTheme      woxcomponent.Theme
	LightPreviewTheme woxcomponent.Theme
	DarkPreviewTheme  woxcomponent.Theme
}

// OnboardingProps contains the prepared first-run state and actions.
type OnboardingProps struct {
	Width                float32
	Height               float32
	AppIcon              *woxui.Image
	Wallpaper            *woxui.Image
	WallpaperBlurred     *woxui.Image
	Steps                []OnboardingStep
	ActiveStep           int
	Labels               map[string]string
	Language             string
	GlanceEnabled        bool
	GlanceLabel          string
	GlanceValue          string
	GlanceIcon           *woxui.Image
	MainHotkeyLabels     []string
	SelectHotkeyLabels   []string
	HotkeyStatus         string
	HotkeyError          bool
	HotkeyRecording      bool
	QueryHotkeyLabels    []string
	QueryHotkeyStatus    string
	QueryHotkeyError     bool
	QueryHotkeyRecording bool
	QueryHotkeyReady     bool
	QueryHotkeySelected  bool
	QueryHotkeyBusy      bool
	Plugins              []OnboardingPlugin
	PluginsLoading       bool
	PluginsError         string
	Themes               []OnboardingTheme
	ThemesLoading        bool
	ThemesApplying       bool
	ThemesError          string
	ThemePreviewTitle    string
	ThemePreviewTexts    []string
	ThemePreviewSubs     []string
	ThemePreviewOpen     string
	Permissions          []OnboardingPermission
	PermissionLoading    bool
	ChoiceKind           string
	ChoiceValue          string
	ChoiceAnchor         woxui.Rect
	Choices              []OnboardingChoice
	Window               *woxui.Window
	Theme                woxcomponent.Theme
	NextDisabled         bool
	OnDrag               func()
	OnStep               func(int)
	OnBack               func()
	OnNext               func()
	OnFinish             func()
	OnRecordHotkey       func()
	OnRecordQueryHotkey  func()
	OnToggleQueryHotkey  func(bool)
	OnToggleGlance       func(bool)
	OnInstallPlugin      func(string)
	OnSelectTheme        func(string)
	OnOpenChoice         func(string)
	OnSelectChoice       func(string)
	OnPermission         func(string)
}

// OnboardingView builds the first-run setup surface.
func OnboardingView(props OnboardingProps) woxwidget.Widget {
	if len(props.Steps) == 0 {
		return woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.Background}
	}
	active := min(max(0, props.ActiveStep), len(props.Steps)-1)
	step := props.Steps[active]
	bodyHeight := max(float32(0), props.Height-OnboardingHeaderHeight-OnboardingFooterHeight)
	header := onboardingHeader(props)
	page := onboardingPage(props, step, bodyHeight)
	footer := onboardingFooter(props, active)
	layers := []woxwidget.StackChild{{Child: woxwidget.Container{
		Width: props.Width, Height: props.Height, Color: props.Theme.Background,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, page, footer}},
	}}}
	if props.ChoiceKind != "" {
		choices := make([]SettingsChoice, len(props.Choices))
		for index, choice := range props.Choices {
			choices[index] = SettingsChoice{
				Value: choice.Value, Label: choice.Label, Leading: choice.Leading, Trailing: choice.Trailing,
			}
		}
		title := props.Labels["language"]
		if props.ChoiceKind == "glance" {
			title = props.Labels["glance.primary"]
		}
		kind := props.ChoiceKind
		layers = append(layers, woxwidget.StackChild{Child: SettingsChoiceView(SettingsChoiceProps{
			ID: "onboarding-choice-picker", Width: props.Width, Height: props.Height, Anchor: props.ChoiceAnchor,
			Theme: props.Theme, Window: props.Window, Title: title, CurrentValue: props.ChoiceValue, Choices: choices,
			OnChoose: func(index int) {
				if props.OnSelectChoice != nil && index >= 0 && index < len(props.Choices) {
					props.OnSelectChoice(props.Choices[index].Value)
				}
			},
			OnCancel: func() {
				if props.OnOpenChoice != nil {
					props.OnOpenChoice(kind)
				}
			},
		})})
	}
	return woxwidget.Semantics{
		Key: "onboarding-window", AutomationID: "onboarding.window", Role: woxui.AccessibilityRoleWindow, Label: props.Labels["title"],
		Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers},
	}
}

func onboardingHeader(props OnboardingProps) woxwidget.Widget {
	var logo woxwidget.Widget = woxwidget.Container{
		Width: 28, Height: 28, Radius: 7, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255},
		Child: woxwidget.Align{Width: 28, Height: 28, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: "W", Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{A: 255},
		}},
	}
	if props.AppIcon != nil {
		logo = woxwidget.Image{Source: props.AppIcon, Width: 28, Height: 28}
	}
	openLanguage := func() {
		if props.OnOpenChoice != nil {
			props.OnOpenChoice("language")
		}
	}
	language := woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
		ID: "onboarding-language", Label: props.Labels["language"], Value: props.Language, Width: 164, Height: 34,
		Outline: settingsColorAlpha(props.Theme.PreviewSplit, 160), Foreground: props.Theme.ResultTitle,
		Secondary: props.Theme.ResultSubtitle, Theme: props.Theme, OnTap: openLanguage,
	})
	content := woxwidget.Container{
		Width: props.Width, Height: OnboardingHeaderHeight, Padding: woxwidget.Insets{Left: 28, Top: 17, Right: 28, Bottom: 17},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisSpaceBetween, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				logo,
				woxwidget.Text{Value: "Wox", Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
			}},
			language,
		}},
	}
	if drag := onboardingDragArea(props.Width, OnboardingHeaderHeight, "onboarding-header-drag", props.OnDrag); drag != nil {
		return woxwidget.Stack{Width: props.Width, Height: OnboardingHeaderHeight, Children: []woxwidget.StackChild{{Child: drag}, {Child: content}}}
	}
	return content
}

func onboardingPage(props OnboardingProps, step OnboardingStep, height float32) woxwidget.Widget {
	width := props.Width
	innerWidth := min(float32(880), max(float32(0), width-120))
	visualHeight := max(float32(180), height-136)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: 20},
		woxwidget.TextBlock{Value: step.Title, Width: innerWidth, Height: 44, LineHeight: 44, MaxLines: 1, Centered: true, Style: woxui.TextStyle{Size: 32, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
		woxwidget.Container{Width: innerWidth, Height: 12},
		woxwidget.TextBlock{Value: props.Labels[step.ID+".body"], Width: innerWidth, Height: 40, LineHeight: 20, MaxLines: 2, Centered: true, Style: woxui.TextStyle{Size: 14}, Color: props.Theme.ResultSubtitle},
		onboardingFeatureVisual(props, step, innerWidth, visualHeight),
	}
	page := woxwidget.Container{
		Width: width, Height: height,
		Child: woxwidget.Align{Width: width, Height: height, Horizontal: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}},
	}
	return page
}

// onboardingDragArea exposes the header chrome as a native window drag target.
func onboardingDragArea(width, height float32, id string, onDrag func()) woxwidget.Widget {
	if onDrag == nil || width <= 0 || height <= 0 {
		return nil
	}
	return woxwidget.Gesture{ID: id, OnDragStart: onDrag, Child: woxwidget.Container{Width: width, Height: height}}
}

func onboardingFeatureVisual(props OnboardingProps, step OnboardingStep, width, height float32) woxwidget.Widget {
	contentWidth := min(float32(640), width)
	var content woxwidget.Widget
	switch step.ID {
	case "permissions":
		content = onboardingPermissions(props, contentWidth, 172)
	case "mainHotkey", "selectionHotkey":
		content = onboardingMainHotkeyVisual(props, contentWidth, step.Accent)
	case "glance":
		content = onboardingGlanceVisual(props, contentWidth)
	case "queryHotkeys":
		content = onboardingQueryHotkeysVisual(props, min(float32(760), width), step.Accent)
	case "wpmInstall":
		content = onboardingPluginsVisual(props, contentWidth)
	case "themeInstall":
		content = onboardingThemesVisual(props, min(float32(680), width), step.Accent)
	case "finish":
		content = onboardingFinishVisual(props, contentWidth, step.Accent)
	default:
		content = onboardingWelcomeVisual(props, contentWidth, step.Accent)
	}
	return woxwidget.Align{Width: width, Height: height, Horizontal: 0.5, Vertical: 0.25, Child: content}
}

func onboardingMainHotkeyVisual(props OnboardingProps, width float32, accent woxui.Color) woxwidget.Widget {
	labels := props.MainHotkeyLabels
	if len(labels) == 0 {
		labels = []string{"Alt", "Space"}
	}
	if accent.A == 0 {
		accent = props.Theme.Cursor
	}
	keyBorder := settingsColorAlpha(props.Theme.PreviewSplit, 180)
	keyBorderWidth := float32(1)
	if props.HotkeyRecording {
		keyBorder = accent
		keyBorderWidth = 2
	}
	if props.HotkeyError {
		keyBorder = props.Theme.ErrorText
		keyBorderWidth = 2
	}
	keys := make([]woxwidget.Widget, 0, len(labels)*2-1)
	for index, label := range labels {
		if index > 0 {
			keys = append(keys, woxwidget.Align{Width: 28, Height: 44, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: "+", Style: woxui.TextStyle{Size: 14}, Color: props.Theme.ResultSubtitle}})
		}
		keyWidth := max(float32(52), float32(len([]rune(label)))*8+24)
		keys = append(keys, woxwidget.Container{
			Width: keyWidth, Height: 44, Radius: 7, Color: props.Theme.QueryBackground,
			BorderColor: keyBorder, BorderWidth: keyBorderWidth,
			Child: woxwidget.Align{Width: keyWidth, Height: 44, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}},
		})
	}
	statusColor := accent
	statusIcon := woxcomponent.CheckCircleGlyph(18, statusColor)
	if props.HotkeyRecording {
		statusColor = props.Theme.ResultSubtitle
		statusIcon = woxcomponent.KeyboardGlyph(18, statusColor)
	}
	if props.HotkeyError {
		statusColor = props.Theme.ErrorText
		statusIcon = woxcomponent.ErrorGlyph(18, statusColor)
	}
	keyControl := woxwidget.Semantics{
		Key: "onboarding-main-hotkey", AutomationID: "onboarding.main_hotkey", Role: woxui.AccessibilityRoleButton,
		Label: strings.Join(labels, " + "), Description: props.Labels["hotkey.change"], Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate && props.OnRecordHotkey != nil {
				props.OnRecordHotkey()
			}
			return nil
		},
		Child: woxwidget.Focusable{
			Key: "onboarding-main-hotkey", FocusRingColor: props.Theme.Cursor, FocusRingRadius: 7,
			OnKey: func(event woxui.KeyEvent) bool {
				if event.Down && (event.Key == woxui.KeyEnter || event.Key == woxui.KeySpace) && props.OnRecordHotkey != nil {
					props.OnRecordHotkey()
					return true
				}
				return false
			},
			Child: woxwidget.Gesture{ID: "onboarding-main-hotkey", OnTap: props.OnRecordHotkey, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: keys}},
		},
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Align{Width: width, Height: 44, Horizontal: 0.5, Child: keyControl},
		woxwidget.Align{Width: width, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			statusIcon,
			woxwidget.Text{Value: props.HotkeyStatus, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: statusColor},
		}}},
		woxwidget.Text{Value: props.Labels["hotkey.change"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		woxwidget.Container{Width: width, Height: 24},
		onboardingHotkeyPreview(props, labels, accent, width),
	}}
}

// onboardingHotkeyPreview mirrors the launch gesture above a real QueryBox silhouette.
func onboardingHotkeyPreview(props OnboardingProps, labels []string, accent woxui.Color, width float32) woxwidget.Widget {
	previewWidth := min(float32(480), max(float32(0), width-80))
	compactKeys := make([]woxwidget.Widget, 0, len(labels)*2-1)
	for index, label := range labels {
		if index > 0 {
			compactKeys = append(compactKeys, woxwidget.Text{Value: "+", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle})
		}
		keyWidth := max(float32(48), float32(len([]rune(label)))*6+16)
		compactKeys = append(compactKeys, woxwidget.Container{
			Width: keyWidth, Height: 32, Radius: 6, Color: settingsColorAlpha(props.Theme.ResultTitle, 18),
			BorderColor: settingsColorAlpha(props.Theme.PreviewSplit, 120), BorderWidth: 1,
			Child: woxwidget.Align{Width: keyWidth, Height: 32, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}},
		})
	}
	queryBox := onboardingQueryPreview(props, accent, previewWidth, props.Labels["hotkey.preview"], false)
	content := woxwidget.Align{Width: width, Height: 232, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: compactKeys},
		woxcomponent.KeyboardArrowDownGlyph(20, props.Theme.ResultSubtitle),
		queryBox,
	}}}
	return woxwidget.Stack{Width: width, Height: 232, Children: []woxwidget.StackChild{
		{Child: woxcomponent.FadingGrid(woxcomponent.FadingGridProps{Width: width, Height: 232, CenterY: 142, RadiusY: 108, Color: props.Theme.PreviewSplit})},
		{Child: content},
	}}
}

// onboardingQueryPreview gives the illustrative QueryBox its own elevated glass surface.
func onboardingQueryPreview(props OnboardingProps, accent woxui.Color, width float32, queryText string, typed bool) woxwidget.Widget {
	const chromeInset = float32(32)
	const chromeHeight = float32(104)
	surface := settingsColorAlpha(props.Theme.ActionBackground, 246)
	chrome := woxwidget.Painter{Width: width + chromeInset*2, Height: chromeHeight, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		for spread := float32(32); spread >= 8; spread -= 4 {
			displayList.FillRoundedRect(woxui.Rect{
				X: bounds.X + chromeInset - spread, Y: bounds.Y + 12 - spread/2 + 4,
				Width: width + spread*2, Height: 64 + spread,
			}, 10+spread/2, woxui.Color{A: 3})
		}
		displayList.FillRoundedRect(woxui.Rect{X: bounds.X + chromeInset - 2, Y: bounds.Y + 18, Width: width + 4, Height: 64}, 12, woxui.Color{A: 12})
		surfaceBounds := woxui.Rect{X: bounds.X + chromeInset, Y: bounds.Y + 12, Width: width, Height: 64}
		displayList.FillRoundedRect(surfaceBounds, 10, surface)
		displayList.StrokeRoundedRect(surfaceBounds, 10, 1, settingsColorAlpha(props.Theme.PreviewSplit, 32))
	}}
	queryColor := props.Theme.ResultSubtitle
	queryChildren := []woxwidget.Widget{woxwidget.Container{Width: 2, Height: 24, Color: accent}}
	if typed {
		queryColor = props.Theme.ResultTitle
		queryChildren = nil
	}
	queryChildren = append(queryChildren, woxwidget.Text{Value: queryText, Style: woxui.TextStyle{Size: 14}, Color: queryColor})
	if typed {
		queryChildren = append(queryChildren, woxwidget.Container{Width: 2, Height: 24, Color: accent})
	}
	trailing := woxwidget.Widget(woxcomponent.SearchGlyph(20, props.Theme.ResultSubtitle))
	if glance := onboardingConfiguredGlanceAccessory(props, props.Theme.ResultSubtitle); glance != nil {
		trailing = glance
	}
	queryChildren = append(queryChildren, woxwidget.Expanded{Child: woxwidget.Align{Height: 24, Horizontal: 1, Vertical: 0.5, Child: trailing}})
	content := woxwidget.Container{
		Width: width, Height: 64,
		Padding: woxwidget.Insets{Left: 18, Top: 20, Right: 18, Bottom: 20},
		Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: queryChildren},
	}
	return woxwidget.Stack{Width: width + chromeInset*2, Height: chromeHeight, Children: []woxwidget.StackChild{
		{Child: chrome},
		{Left: chromeInset, Top: 12, Child: content},
	}}
}

// onboardingConfiguredGlanceAccessory mirrors the currently selected Glance item in later QueryBox examples.
func onboardingConfiguredGlanceAccessory(props OnboardingProps, color woxui.Color) woxwidget.Widget {
	if !props.GlanceEnabled || strings.TrimSpace(props.GlanceValue) == "" {
		return nil
	}
	var icon woxwidget.Widget = woxcomponent.ClockGlyph(16, color)
	if props.GlanceIcon != nil {
		icon = woxwidget.Image{Source: props.GlanceIcon, Width: 16, Height: 16, Fit: woxwidget.ImageFitContain}
	}
	return onboardingGlanceQueryAccessory(props.GlanceValue, icon, color)
}

func onboardingFeatureCard(props OnboardingProps, width, height float32, child woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Container{
		Width: width, Height: height, Radius: 14, Padding: woxwidget.UniformInsets(24),
		Color: settingsColorAlpha(props.Theme.ResultTitle, 12), BorderColor: settingsColorAlpha(props.Theme.PreviewSplit, 110), BorderWidth: 1,
		Child: child,
	}
}

func onboardingWelcomeVisual(props OnboardingProps, width float32, accent woxui.Color) woxwidget.Widget {
	if accent.A == 0 {
		accent = props.Theme.Cursor
	}
	const stageHeight = float32(212)
	queryWidth := min(float32(480), max(float32(0), width-96))
	var logo woxwidget.Widget = woxwidget.Container{
		Width: 56, Height: 56, Radius: 14, Color: props.Theme.ResultTitle,
		Child: woxwidget.Align{Width: 56, Height: 56, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: "W", Style: woxui.TextStyle{Size: 32, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Background}},
	}
	if props.AppIcon != nil {
		logo = woxwidget.Image{Source: props.AppIcon, Width: 56, Height: 56}
	}
	capabilities := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Labels["welcome.apps"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		woxwidget.Text{Value: "·", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		woxwidget.Text{Value: props.Labels["welcome.files"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		woxwidget.Text{Value: "·", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		woxwidget.Text{Value: props.Labels["welcome.plugins"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		woxwidget.Text{Value: "·", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		woxwidget.Text{Value: props.Labels["welcome.ai"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
	}}
	stage := woxwidget.Stack{Width: width, Height: stageHeight, Children: []woxwidget.StackChild{
		{Top: -124, Child: woxcomponent.FadingGrid(woxcomponent.FadingGridProps{Width: width, Height: 336, CenterY: 236, RadiusY: 88, Color: props.Theme.PreviewSplit})},
		{Top: 4, Child: woxwidget.Align{Width: width, Height: 56, Horizontal: 0.5, Child: logo}},
		{Top: 68, Child: woxwidget.Align{Width: width, Height: 104, Horizontal: 0.5, Child: onboardingQueryPreview(props, accent, queryWidth, "wox", true)}},
		{Top: 184, Child: woxwidget.Align{Width: width, Height: 20, Horizontal: 0.5, Vertical: 0.5, Child: capabilities}},
	}}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 16, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		stage,
		woxwidget.Text{Value: props.Labels["welcome.hint"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
	}}
}

func onboardingQueryHotkeysVisual(props OnboardingProps, width float32, accent woxui.Color) woxwidget.Widget {
	const demoWidth = float32(420)
	if accent.A == 0 {
		accent = props.Theme.Cursor
	}
	labels := props.QueryHotkeyLabels
	if len(labels) == 0 {
		labels = []string{"Ctrl", "Shift", "V"}
	}
	keyBorder := settingsColorAlpha(props.Theme.PreviewSplit, 96)
	keyBorderWidth := float32(1)
	if props.QueryHotkeyRecording {
		keyBorder = accent
		keyBorderWidth = 2
	}
	if props.QueryHotkeyError {
		keyBorder = props.Theme.ErrorText
		keyBorderWidth = 2
	}
	keys := make([]woxwidget.Widget, 0, len(labels)*2-1)
	for index, label := range labels {
		if index > 0 {
			keys = append(keys, woxwidget.Text{Value: "+", Style: woxui.TextStyle{Size: 14}, Color: props.Theme.ResultSubtitle})
		}
		keyWidth := max(float32(52), float32(len([]rune(label)))*8+24)
		keys = append(keys, woxwidget.Container{
			Width: keyWidth, Height: 44, Radius: 7, Color: props.Theme.QueryBackground,
			BorderColor: keyBorder, BorderWidth: keyBorderWidth,
			Child: woxwidget.Align{Width: keyWidth, Height: 44, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}},
		})
	}
	keyControl := woxwidget.Semantics{
		Key: "onboarding-query-hotkey", AutomationID: "onboarding.query_hotkey", Role: woxui.AccessibilityRoleButton,
		Label: strings.Join(labels, " + "), Description: props.QueryHotkeyStatus, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate && props.OnRecordQueryHotkey != nil {
				props.OnRecordQueryHotkey()
			}
			return nil
		},
		Child: woxwidget.Gesture{ID: "onboarding-query-hotkey", OnTap: props.OnRecordQueryHotkey, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: keys}},
	}
	queryHeader := []woxwidget.Widget{
		woxwidget.Text{Value: "cb", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: accent},
		woxwidget.Container{Width: 2, Height: 22, Color: accent},
	}
	if glance := onboardingConfiguredGlanceAccessory(props, props.Theme.ResultSubtitle); glance != nil {
		queryHeader = append(queryHeader, woxwidget.Expanded{Child: woxwidget.Align{Height: 30, Horizontal: 1, Vertical: 0.5, Child: glance}})
	}
	queryWindow := woxwidget.Container{
		Width: demoWidth, Height: 188, Radius: 10, Color: props.Theme.ActionBackground,
		BorderColor: settingsColorAlpha(props.Theme.PreviewSplit, 48), BorderWidth: 1,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Container{Width: demoWidth, Height: 52, Padding: woxwidget.Insets{Left: 16, Top: 11, Right: 8, Bottom: 11}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: queryHeader}},
			onboardingQueryHotkeyResult(props, demoWidth, props.Labels["queryHotkeys.clipboard"], "Clipboard history", true),
			onboardingQueryHotkeyResult(props, demoWidth, "https://www.woxlauncher.com", "Copied link", false),
			onboardingQueryHotkeyResult(props, demoWidth, "Wox", "Copied text", false),
		}},
	}
	statusColor := props.Theme.ResultSubtitle
	statusLabel := props.Labels["queryHotkeys.notConfigured"]
	if props.QueryHotkeyReady {
		statusLabel = props.Labels["queryHotkeys.configured"]
	} else if props.QueryHotkeyError {
		statusColor = props.Theme.ErrorText
	}
	shortcutCaption := props.Labels["queryHotkeys.shortcut"]
	shortcutCaptionColor := props.Theme.ResultSubtitle
	if props.QueryHotkeyError {
		shortcutCaption = props.QueryHotkeyStatus
		shortcutCaptionColor = props.Theme.ErrorText
	}
	checkboxTheme := props.Theme
	checkboxTheme.ActionSelected = settingsColorAlpha(props.Theme.ResultTitle, 77)
	checkboxTheme.ActionSelectedText = props.Theme.ResultTitle
	return woxwidget.Flex{Axis: woxwidget.Vertical, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 24, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				keyControl,
				woxwidget.Text{Value: shortcutCaption, Style: woxui.TextStyle{Size: 12}, Color: shortcutCaptionColor},
				woxwidget.Text{Value: props.Labels["hotkey.change"], Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
			}},
			woxcomponent.ArrowRightGlyph(24, props.Theme.ResultSubtitle),
			queryWindow,
		}},
		woxwidget.Container{Width: width, Height: 100},
		woxwidget.Container{Width: width, Height: 1, Color: settingsColorAlpha(props.Theme.PreviewSplit, 48)},
		woxwidget.Container{Width: width, Height: 20},
		woxwidget.Container{Width: width, Height: 64, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Container{Width: 40, Height: 40, Radius: 8, Color: settingsColorAlpha(props.Theme.ResultTitle, 14), Child: woxwidget.Align{Width: 40, Height: 40, Horizontal: 0.5, Vertical: 0.5, Child: woxcomponent.KeyboardGlyph(20, props.Theme.ResultSubtitle)}},
			woxwidget.Container{Width: 12},
			woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.Labels["queryHotkeys.status.title"], Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
				woxwidget.Text{Value: props.Labels["queryHotkeys.status.body"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
			}}},
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxwidget.Text{Value: statusLabel, Style: woxui.TextStyle{Size: 12}, Color: statusColor},
				woxcomponent.WoxCheckbox(woxcomponent.CheckboxProps{ID: "onboarding-query-hotkey-enabled", Label: props.Labels["queryHotkeys.status.title"], Value: props.QueryHotkeySelected, Disabled: props.QueryHotkeyBusy, OnChange: props.OnToggleQueryHotkey, Theme: checkboxTheme}),
			}},
		}}},
	}}
}

func onboardingQueryHotkeyResult(props OnboardingProps, width float32, title, subtitle string, selected bool) woxwidget.Widget {
	background := woxui.Color{}
	titleColor := props.Theme.ResultTitle
	subtitleColor := props.Theme.ResultSubtitle
	if selected {
		background = props.Theme.SelectedBackground
		titleColor = props.Theme.SelectedTitle
		subtitleColor = props.Theme.SelectedSubtitle
	}
	return woxwidget.Align{Width: width, Height: 40, Horizontal: 0.5, Child: woxwidget.Container{Width: width - 20, Height: 40, Radius: 6, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 5, Right: 12, Bottom: 5}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxcomponent.CopyGlyph(18, subtitleColor),
		woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: titleColor},
			woxwidget.Text{Value: subtitle, Style: woxui.TextStyle{Size: 10}, Color: subtitleColor},
		}},
	}}}}
}

// onboardingPluginsVisual presents curated store plugins with their real metadata and install state.
func onboardingPluginsVisual(props OnboardingProps, width float32) woxwidget.Widget {
	if props.PluginsLoading && len(props.Plugins) == 0 {
		return woxwidget.Align{Width: width, Height: 220, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.HourglassGlyph(20, props.Theme.ResultSubtitle),
			woxwidget.Text{Value: props.Labels["plugins.loading"], Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle},
		}}}
	}
	if len(props.Plugins) == 0 {
		return woxwidget.Align{Width: width, Height: 220, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.TextBlock{
			Value: props.PluginsError, Width: width, MaxLines: 3, Centered: true, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ErrorText,
		}}
	}
	rows := make([]woxwidget.Widget, 0, len(props.Plugins))
	for index, plugin := range props.Plugins {
		plugin := plugin
		var icon woxwidget.Widget = woxcomponent.ExtensionGlyph(24, props.Theme.ResultSubtitle)
		if plugin.Icon != nil {
			icon = woxwidget.Image{Source: plugin.Icon, Width: 28, Height: 28}
		}
		var action woxwidget.Widget
		if plugin.Installed {
			action = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxcomponent.CheckCircleGlyph(18, props.Theme.ResultSubtitle),
				woxwidget.Text{Value: props.Labels["plugins.installed"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
			}}
		} else {
			label := props.Labels["plugins.install"]
			if plugin.Installing {
				label = props.Labels["plugins.installing"]
			}
			action = woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "onboarding-plugin-install-" + plugin.ID, Label: label, Width: 88, Variant: woxcomponent.ButtonOutline,
				Disabled: plugin.Disabled, Theme: props.Theme, OnTap: func() {
					if props.OnInstallPlugin != nil {
						props.OnInstallPlugin(plugin.ID)
					}
				},
			})
		}
		bottomBorder := float32(1)
		if index == len(props.Plugins)-1 {
			bottomBorder = 0
		}
		rows = append(rows, woxwidget.Container{
			Width: width, Height: 64, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 12},
			BottomBorderColor: settingsColorAlpha(props.Theme.PreviewSplit, 48), BottomBorderWidth: bottomBorder,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxwidget.Container{Width: 40, Height: 40, Radius: 8, Color: settingsColorAlpha(props.Theme.ResultTitle, 14), Child: woxwidget.Align{Width: 40, Height: 40, Horizontal: 0.5, Vertical: 0.5, Child: icon}},
				woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: plugin.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
					woxwidget.TextBlock{Value: plugin.Description, MaxLines: 1, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
				}}},
				action,
			}},
		})
	}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: float32(len(rows)) * 64, Radius: 10, Color: settingsColorAlpha(props.Theme.ResultTitle, 10), BorderColor: settingsColorAlpha(props.Theme.PreviewSplit, 48), BorderWidth: 1, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}},
		woxwidget.Text{Value: props.Labels["plugins.more"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
	}
	if props.PluginsError != "" {
		children = append(children, woxwidget.TextBlock{Value: props.PluginsError, Width: width, MaxLines: 2, Centered: true, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText})
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 20, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}
}

// onboardingThemesVisual reuses the production theme preview while keeping selection in onboarding.
func onboardingThemesVisual(props OnboardingProps, width float32, accent woxui.Color) woxwidget.Widget {
	if props.ThemesLoading && len(props.Themes) == 0 {
		return woxwidget.Align{Width: width, Height: 260, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.HourglassGlyph(20, props.Theme.ResultSubtitle),
			woxwidget.Text{Value: props.Labels["theme.loading"], Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle},
		}}}
	}
	if len(props.Themes) == 0 {
		return woxwidget.Align{Width: width, Height: 260, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.TextBlock{
			Value: props.ThemesError, Width: width, MaxLines: 3, Centered: true, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ErrorText,
		}}
	}
	if accent.A == 0 {
		accent = props.Theme.Cursor
	}
	rows := make([]woxwidget.Widget, 0, (len(props.Themes)+1)/2)
	for index := 0; index < len(props.Themes); index += 2 {
		items := make([]woxwidget.Widget, 0, 2)
		for itemIndex := index; itemIndex < min(index+2, len(props.Themes)); itemIndex++ {
			theme := props.Themes[itemIndex]
			items = append(items, woxwidget.Expanded{Child: woxwidget.LayoutBuilder{Build: func(size woxui.Size) woxwidget.Widget {
				return onboardingThemeCard(props, theme, size.Width, accent)
			}}})
		}
		if len(items) == 1 {
			items = append(items, woxwidget.Expanded{Child: woxwidget.Container{}})
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: items})
	}
	children := []woxwidget.Widget{woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 16, Children: rows}}
	if props.ThemesError != "" {
		children = append(children, woxwidget.TextBlock{Value: props.ThemesError, Width: width, MaxLines: 2, Centered: true, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText})
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 16, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}
}

// onboardingThemeCard provides shared hover, focus, keyboard, and selected behavior around a live preview.
func onboardingThemeCard(props OnboardingProps, theme OnboardingTheme, width float32, accent woxui.Color) woxwidget.Widget {
	const cardHeight = float32(232)
	const cardPadding = float32(8)
	contentWidth := max(float32(0), width-cardPadding*2)
	contentHeight := cardHeight - cardPadding*2
	previewProps := ThemeSettingsProps{
		Theme: props.Theme, PreviewTitle: props.ThemePreviewTitle, PreviewTexts: props.ThemePreviewTexts,
		PreviewSubtitles: props.ThemePreviewSubs, PreviewOpenLabel: props.ThemePreviewOpen, Window: props.Window,
	}
	preview := themeCatalogPreview(previewProps, theme.PreviewTheme, contentWidth-8, 184)
	if theme.IsAuto {
		preview = themeAutoCatalogPreview(previewProps, theme.LightPreviewTheme, theme.DarkPreviewTheme, contentWidth-8, 184)
	}
	previewChildren := []woxwidget.StackChild{{Left: 4, Top: 4, Child: preview}}
	if theme.Selected {
		previewChildren = append(previewChildren, woxwidget.StackChild{Right: 8, Top: 8, AnchorRight: true, Child: woxcomponent.CheckCircleGlyph(20, accent)})
	}
	border := settingsColorAlpha(props.Theme.PreviewSplit, 96)
	if theme.Selected {
		border = accent
	}
	selectTheme := func() {
		if props.OnSelectTheme != nil {
			props.OnSelectTheme(theme.ID)
		}
	}
	if props.ThemesApplying {
		selectTheme = nil
	}
	cardBackground := props.Theme.QueryBackground
	cardContent := woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Stack{Width: contentWidth, Height: 192, Children: previewChildren},
		woxwidget.Align{Width: contentWidth, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: theme.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}},
	}}
	cardChild := woxwidget.Stack{Width: contentWidth, Height: contentHeight, Children: []woxwidget.StackChild{{Child: cardContent}}}
	if selectTheme != nil {
		cardChild.Children = append(cardChild.Children, woxwidget.StackChild{Child: woxwidget.Gesture{
			ID: "onboarding-theme-hit-" + theme.ID, OnTap: selectTheme,
			Child: woxwidget.Container{Width: contentWidth, Height: contentHeight},
		}})
	}
	radius := float32(10)
	return woxcomponent.WoxListItem(woxcomponent.ListItemProps{
		ID: "onboarding-theme-" + theme.ID, Label: theme.Name, Width: width, Height: cardHeight, Radius: &radius,
		Padding: woxwidget.UniformInsets(cardPadding), Background: &cardBackground, HoverBackground: &cardBackground,
		BorderColor: border, BorderWidth: 1, Selected: theme.Selected, Disabled: props.ThemesApplying, Theme: props.Theme,
		OnTap: selectTheme, Child: cardChild,
	})
}

func onboardingFinishVisual(props OnboardingProps, width float32, accent woxui.Color) woxwidget.Widget {
	if accent.A == 0 {
		accent = props.Theme.Cursor
	}
	var logo woxwidget.Widget = woxwidget.Container{
		Width: 56, Height: 56, Radius: 14, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255},
		Child: woxwidget.Align{Width: 56, Height: 56, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: "W", Style: woxui.TextStyle{Size: 34, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{A: 255}}},
	}
	if props.AppIcon != nil {
		logo = woxwidget.Image{Source: props.AppIcon, Width: 56, Height: 56}
	}
	queryWidth := min(float32(480), max(float32(0), width-80))
	query := onboardingQueryPreview(props, accent, queryWidth, props.Labels["finish.query"], true)
	queryStage := woxwidget.Stack{Width: width, Height: 224, Children: []woxwidget.StackChild{
		{Child: woxcomponent.FadingGrid(woxcomponent.FadingGridProps{Width: width, Height: 224, CenterY: 132, RadiusY: 72, Color: props.Theme.PreviewSplit})},
		{Child: woxwidget.Align{Width: width, Height: 56, Horizontal: 0.5, Vertical: 0.5, Child: logo}},
		{Top: 88, Child: woxwidget.Align{Width: width, Height: 104, Horizontal: 0.5, Vertical: 0.5, Child: query}},
	}}
	summaries := []struct {
		label string
		value string
	}{{props.Labels["finish.hotkey"], strings.Join(props.MainHotkeyLabels, " + ")}}
	if props.GlanceEnabled {
		summaries = append(summaries, struct {
			label string
			value string
		}{props.Labels["finish.glance"], props.GlanceLabel})
	}
	rows := make([]woxwidget.Widget, 0, len(summaries)*2-1)
	for index, summary := range summaries {
		if index > 0 {
			rows = append(rows, woxwidget.Container{Width: queryWidth, Height: 1, Color: settingsColorAlpha(props.Theme.PreviewSplit, 48)})
		}
		rows = append(rows, woxwidget.Container{Width: queryWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.CheckCircleGlyph(16, accent),
			woxwidget.Text{Value: summary.label, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultTitle},
			woxwidget.Expanded{Child: woxwidget.Align{Height: 44, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{Value: summary.value, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}}},
		}}})
	}
	pluginIcons := make([]woxwidget.Widget, 0, len(props.Plugins))
	for _, plugin := range props.Plugins {
		if !plugin.Installed {
			continue
		}
		var icon woxwidget.Widget = woxcomponent.ExtensionGlyph(20, props.Theme.ResultSubtitle)
		if plugin.Icon != nil {
			icon = woxwidget.Image{Source: plugin.Icon, Width: 24, Height: 24}
		}
		pluginIcons = append(pluginIcons, woxwidget.Align{Width: 24, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: icon})
	}
	if len(pluginIcons) > 0 {
		if len(rows) > 0 {
			rows = append(rows, woxwidget.Container{Width: queryWidth, Height: 1, Color: settingsColorAlpha(props.Theme.PreviewSplit, 48)})
		}
		rows = append(rows, woxwidget.Container{Width: queryWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.CheckCircleGlyph(16, accent),
			woxwidget.Text{Value: props.Labels["finish.plugins"], Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultTitle},
			woxwidget.Expanded{Child: woxwidget.Align{Height: 44, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: pluginIcons}}},
		}}})
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		queryStage,
		woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		woxwidget.TextBlock{Value: props.Labels["finish.hint"], Width: queryWidth, Height: 20, MaxLines: 1, Centered: true, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
	}}
}

func onboardingPermissions(props OnboardingProps, width, height float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(props.Permissions))
	rowHeight := height / max(float32(1), float32(len(props.Permissions)))
	for _, permission := range props.Permissions {
		status := props.Labels["permission.authorize"]
		var action woxwidget.Widget
		if permission.Ready {
			status = "✓"
			action = woxwidget.Container{
				Width: 32, Height: 32, Radius: 16, Color: settingsColorAlpha(woxui.Color{R: 34, G: 197, B: 94, A: 255}, 36),
				Child: woxwidget.Align{Width: 32, Height: 32, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 18}, Color: woxui.Color{R: 34, G: 197, B: 94, A: 255}}},
			}
		} else {
			if props.PermissionLoading {
				status = props.Labels["permission.checking"]
			}
			action = woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "onboarding-permission-" + permission.ID, Label: status,
				Variant: woxcomponent.ButtonOutline, Disabled: props.PermissionLoading, Theme: props.Theme,
				OnTap: func() {
					if props.OnPermission != nil {
						props.OnPermission(permission.ID)
					}
				},
			})
		}
		iconColor := props.Theme.ResultTitle
		if iconColor.A == 0 {
			iconColor = woxui.Color{R: 255, G: 255, B: 255, A: 220}
		}
		rows = append(rows, woxwidget.Container{
			Width: width, Height: rowHeight, Padding: woxwidget.Insets{Left: 18, Top: 12, Right: 18, Bottom: 12},
			BorderColor: settingsColorAlpha(props.Theme.ResultSubtitle, 30), BorderWidth: 1,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxwidget.Container{
					Width: 38, Height: 38, Radius: 8, Color: settingsColorAlpha(props.Theme.ResultTitle, 18),
					Child: woxwidget.Align{Width: 38, Height: 38, Horizontal: 0.5, Vertical: 0.5, Child: permissionIcon(permission.ID, 20, iconColor)},
				},
				woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: permission.Title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
					woxwidget.TextBlock{Value: permission.Description, MaxLines: 2, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: props.Theme.ResultSubtitle},
				}}},
				woxwidget.Align{Width: 110, Horizontal: 1, Vertical: 0.5, Child: action},
			}},
		})
	}
	return woxwidget.Container{
		Width: width, Height: height, Radius: 8, Color: settingsColorAlpha(props.Theme.ResultTitle, 10),
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}
}

func onboardingGlance(props OnboardingProps, width, height float32) woxwidget.Widget {
	switchTheme := props.Theme
	switchTheme.ActionSelected = settingsColorAlpha(props.Theme.ResultTitle, 77)
	switchTheme.ActionSelectedText = props.Theme.ResultTitle
	switchControl := woxcomponent.WoxSwitch(woxcomponent.SwitchProps{
		ID: "onboarding-glance-enable", Label: props.Labels["glance.enable"], Value: props.GlanceEnabled, Theme: switchTheme, OnChange: props.OnToggleGlance,
	})
	openChoice := func() {
		if props.OnOpenChoice != nil {
			props.OnOpenChoice("glance")
		}
	}
	controls := []woxwidget.Widget{
		switchControl,
		woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
			ID: "onboarding-glance-choice", Label: props.Labels["glance.primary"], Value: props.GlanceLabel,
			Width: 220, Height: woxcomponent.SettingsControlHeight, Outline: settingsColorAlpha(props.Theme.ResultSubtitle, 96),
			Foreground: props.Theme.ResultTitle, Secondary: props.Theme.ResultSubtitle, Theme: props.Theme, OnTap: openChoice,
		}),
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 40, Height: 40, Radius: 8, Color: settingsColorAlpha(props.Theme.ResultTitle, 14), Child: woxwidget.Align{Width: 40, Height: 40, Horizontal: 0.5, Vertical: 0.5, Child: woxcomponent.ClockGlyph(20, props.Theme.ResultSubtitle)}},
		woxwidget.Container{Width: 12},
		woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Labels["glance.enable"], Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
			woxwidget.Text{Value: props.Labels["glance.enable.body"], Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		}}},
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: controls},
	}}}
}

func onboardingGlanceVisual(props OnboardingProps, width float32) woxwidget.Widget {
	queryWidth := min(float32(560), width)
	queryChildren := []woxwidget.Widget{
		woxwidget.Expanded{Child: woxwidget.Align{Height: 64, Vertical: 0.5, Child: woxwidget.Text{
			Value: props.Labels["glance.query"], Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.QueryText,
		}}},
	}
	if glance := onboardingConfiguredGlanceAccessory(props, props.Theme.ResultSubtitle); glance != nil {
		queryChildren = append(queryChildren, glance)
	}
	const chromeInset = float32(28)
	const chromeHeight = float32(96)
	chrome := woxwidget.Painter{Width: queryWidth + chromeInset*2, Height: chromeHeight, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		for spread := float32(28); spread >= 8; spread -= 4 {
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + chromeInset - spread, Y: bounds.Y + 12 - spread/2 + 4, Width: queryWidth + spread*2, Height: 64 + spread}, 10+spread/2, woxui.Color{A: 3})
		}
		displayList.FillRoundedRect(woxui.Rect{X: bounds.X + chromeInset - 2, Y: bounds.Y + 18, Width: queryWidth + 4, Height: 64}, 12, woxui.Color{A: 12})
		surface := woxui.Rect{X: bounds.X + chromeInset, Y: bounds.Y + 12, Width: queryWidth, Height: 64}
		displayList.FillRoundedRect(surface, 10, settingsColorAlpha(props.Theme.ActionBackground, 246))
		displayList.StrokeRoundedRect(surface, 10, 1, settingsColorAlpha(props.Theme.PreviewSplit, 32))
	}}
	queryContent := woxwidget.Container{
		Width: queryWidth, Height: 64,
		Padding: woxwidget.Insets{Left: 20, Top: 14, Right: 16, Bottom: 14},
		Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: queryChildren},
	}
	queryBox := woxwidget.Stack{Width: queryWidth + chromeInset*2, Height: chromeHeight, Children: []woxwidget.StackChild{
		{Child: chrome},
		{Left: chromeInset, Top: 12, Child: queryContent},
	}}
	queryStage := woxwidget.Stack{Width: width, Height: 156, Children: []woxwidget.StackChild{
		{Top: -176, Child: woxcomponent.FadingGrid(woxcomponent.FadingGridProps{Width: width, Height: 332, CenterY: 220, RadiusY: 88, Color: props.Theme.PreviewSplit})},
		{Child: woxwidget.Align{Width: width, Height: chromeHeight, Horizontal: 0.5, Child: queryBox}},
	}}
	return woxwidget.Flex{Axis: woxwidget.Vertical, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		queryStage,
		woxwidget.Container{Width: queryWidth, Height: 1, Color: settingsColorAlpha(props.Theme.PreviewSplit, 48)},
		woxwidget.Container{Width: queryWidth, Height: 20},
		onboardingGlance(props, queryWidth, 64),
	}}
}

func onboardingFooter(props OnboardingProps, active int) woxwidget.Widget {
	last := active == len(props.Steps)-1
	nextLabel := props.Labels["next"]
	nextAction := props.OnNext
	nextID := "onboarding-next"
	if last {
		nextLabel = props.Labels["finish"]
		nextAction = props.OnFinish
		nextID = "onboarding-finish"
	}
	accent := props.Steps[active].Accent
	if accent.A == 0 {
		accent = props.Theme.Cursor
	}
	dots := make([]woxwidget.Widget, 0, len(props.Steps))
	for index, step := range props.Steps {
		index := index
		disabled := props.NextDisabled && index > active
		color := settingsColorAlpha(props.Theme.ResultSubtitle, 84)
		size := float32(6)
		if index == active {
			color = accent
			size = 8
		}
		id := "onboarding-step-" + step.ID
		onTap := func() {
			if props.OnStep != nil {
				props.OnStep(index)
			}
		}
		actions := []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
		if disabled {
			onTap = nil
			actions = nil
		}
		dots = append(dots, woxwidget.Semantics{
			Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleButton, Label: step.Title,
			Actions: actions, Disabled: disabled,
			Child: woxwidget.Gesture{ID: id, OnTap: onTap, Child: woxwidget.Align{Width: 22, Height: 36, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{Width: size, Height: size, Radius: size / 2, Color: color}}},
		})
	}
	buttonTheme := props.Theme
	buttonTheme.ActionSelected = accent
	footerChildren := []woxwidget.StackChild{
		{Child: woxwidget.Align{Width: props.Width - 56, Height: 38, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 2, Children: dots}}},
		{AnchorRight: true, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: nextID, Label: nextLabel, Variant: woxcomponent.ButtonPrimary, Disabled: props.NextDisabled, Theme: buttonTheme, Padding: woxwidget.Insets{Left: 28, Right: 28}, OnTap: nextAction,
		})},
	}
	if active > 0 {
		footerChildren = append([]woxwidget.StackChild{{Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "onboarding-back", Label: props.Labels["back"], Padding: woxwidget.Insets{Left: 8, Right: 8},
			Variant: woxcomponent.ButtonText, Theme: props.Theme, OnTap: props.OnBack,
		})}}, footerChildren...)
	}
	content := woxwidget.Container{
		Width: props.Width, Height: OnboardingFooterHeight,
		Padding: woxwidget.Insets{Left: 28, Top: 17, Right: 28, Bottom: 17},
		Child:   woxwidget.Stack{Width: props.Width - 56, Height: 38, Children: footerChildren},
	}
	return content
}

// permissionIcon returns the monochrome SVG for one onboarding permission row.
func permissionIcon(id string, size float32, color woxui.Color) woxwidget.Widget {
	if id == "accessibility" {
		return woxcomponent.AccessibilityGlyph(size, color)
	}
	return woxcomponent.DiskAccessGlyph(size, color)
}

func permissionGlyph(id string) string {
	if id == "accessibility" {
		return "A"
	}
	return "F"
}
