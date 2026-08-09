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
		Labels:  map[string]string{"title": "Set up Wox", "subtitle": "Quick setup", "skip": "Skip", "back": "Back", "next": "Next"},
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

func TestOnboardingInfoPanelUsesIntrinsicDescriptionHeight(t *testing.T) {
	page := onboardingPage(OnboardingProps{
		Width: 1040, Height: 800,
		Labels: map[string]string{"trayQueries.body": "把常用查询钉到系统托盘，点一下就能直接触发，不用先唤起 Wox。"},
		Theme:  woxcomponent.Theme{},
	}, OnboardingStep{ID: "trayQueries", Title: "Tray Queries"}, 728).(woxwidget.Container)

	content := page.Child.(woxwidget.Flex)
	panel := content.Children[2].(woxwidget.Container)
	if panel.Height != 65 || panel.Color != settingsColorAlpha(woxui.Color{}, 14) {
		t.Fatalf("info panel = %#v, want one-line intrinsic height 65 with translucent surface", panel)
	}
	if description := panel.Child.(woxwidget.TextBlock); description.Height != 0 {
		t.Fatalf("description height = %v, want natural text height", description.Height)
	}
	preview := content.Children[4].(woxwidget.Container)
	if preview.Height != 535 {
		t.Fatalf("preview height = %v, want remaining height after intrinsic panel", preview.Height)
	}
}

func TestOnboardingContentCardsUseSurfaceColors(t *testing.T) {
	theme := woxcomponent.Theme{ResultTitle: woxui.Color{R: 240, G: 240, B: 240, A: 255}, ResultSubtitle: woxui.Color{R: 160, G: 160, B: 160, A: 255}}
	for _, test := range []struct {
		id     string
		height float32
		alpha  uint8
	}{
		{id: "welcome", height: 138, alpha: 14},
		{id: "permissions", height: 172, alpha: 10},
		{id: "mainHotkey", height: 90, alpha: 10},
		{id: "selectionHotkey", height: 90, alpha: 10},
		{id: "glance", height: 154, alpha: 14},
		{id: "queryHotkeys", height: 65, alpha: 14},
		{id: "trayQueries", height: 65, alpha: 14},
		{id: "wpmInstall", height: 65, alpha: 14},
		{id: "themeInstall", height: 65, alpha: 14},
		{id: "finish", height: 65, alpha: 14},
	} {
		props := OnboardingProps{Theme: theme}
		if test.id == "mainHotkey" || test.id == "selectionHotkey" {
			props.Hotkey = woxwidget.Container{Width: 400, Height: 62}
		}
		card := onboardingStepContent(props, OnboardingStep{ID: test.id}, 720, test.height).(woxwidget.Container)
		if card.Color != settingsColorAlpha(theme.ResultTitle, test.alpha) {
			t.Fatalf("%s card color = %#v, want translucent surface", test.id, card.Color)
		}
	}
}

func TestOnboardingChromeUsesOnlyInteriorDividers(t *testing.T) {
	props := OnboardingProps{
		Width: 1040, Height: 800, ActiveStep: 0,
		Steps:  []OnboardingStep{{ID: "welcome", Title: "Welcome"}},
		Labels: map[string]string{"title": "Set up Wox", "subtitle": "Quick setup", "skip": "跳过", "back": "Back", "next": "Next"},
		Theme:  woxcomponent.Theme{},
	}
	rail, ok := onboardingRail(props, 0, 728).(woxwidget.Stack)
	if !ok || len(rail.Children) != 2 || rail.Children[1].Left != OnboardingSidebarWidth-1 {
		t.Fatalf("rail = %#v, want content plus right divider", rail)
	}
	footer, ok := onboardingFooter(props, 0).(woxwidget.Stack)
	if !ok || len(footer.Children) != 2 {
		t.Fatalf("footer = %#v, want content plus top divider", footer)
	}
	content := footer.Children[0].Child.(woxwidget.Container)
	actions := content.Child.(woxwidget.Flex)
	skip := actions.Children[0].(woxwidget.Semantics)
	focusable := skip.Child.(woxwidget.Focusable)
	gesture := focusable.Child.(woxwidget.Gesture)
	button := gesture.Child.(woxwidget.Container)
	if button.Width != 0 {
		t.Fatalf("skip width = %v, want content-sized", button.Width)
	}
	if button.Padding.Left != 8 || button.Padding.Right != 8 {
		t.Fatalf("skip padding = %#v, want room for two CJK glyphs", button.Padding)
	}
	if actions.MainAxisAlignment != woxwidget.MainAxisSpaceBetween {
		t.Fatalf("footer alignment = %v, want space-between", actions.MainAxisAlignment)
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
	if got := demoEnterHoldExit(.94, .56, .74, .92, 1); got >= 1 || got <= 0 {
		t.Fatalf("selection window exit progress = %v, want in-flight exit", got)
	}
}

func TestOnboardingDemoQueriesUseSharedFastTypingSpeed(t *testing.T) {
	const start = float32(.2)
	for _, duration := range []time.Duration{4400 * time.Millisecond, 5600 * time.Millisecond, 9500 * time.Millisecond} {
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
	chrome := stack.Children[len(stack.Children)-1].Child.(woxwidget.Container)
	if chrome.Radius != 8 {
		t.Fatalf("desktop chrome radius = %v, want 8 to match wallpaper corners", chrome.Radius)
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
	if len(center.Children) != 7 || content.Children[0].Left <= 0 {
		t.Fatalf("centered taskbar apps = %d at left %v, want seven centered icons", len(center.Children), content.Children[0].Left)
	}
	tray := content.Children[1].Child.(woxwidget.Align)
	if tray.Width != 180 || tray.Horizontal != 1 {
		t.Fatalf("system tray = %#v, want right-aligned 180px tray", tray)
	}
}

func TestOnboardingDemoPreservesThemeTransparency(t *testing.T) {
	transparent := woxui.Color{R: 255, G: 255, B: 255, A: 0}
	if got := demoColorOpacity(transparent, 1); got.A != 0 {
		t.Fatalf("transparent query alpha = %d, want 0", got.A)
	}
	mica := onboardingDemoMicaColor(woxui.Color{R: 22, G: 22, B: 26, A: 133})
	if mica.A < 163 || mica.A > 219 {
		t.Fatalf("mica alpha = %d, want Flutter's 0.64-0.86 range", mica.A)
	}
	backdrop := &woxui.Image{Width: 702, Height: 344}
	window := onboardingDemoWindow(onboardingDemoWindowProps{
		Width: 400, Height: 260, Backdrop: backdrop, Opacity: 1, ShowQuery: true,
		Theme:   woxcomponent.Theme{Background: woxui.Color{R: 22, G: 22, B: 26, A: 133}, QueryBackground: transparent, SelectedBackground: woxui.Color{R: 255, G: 255, B: 255, A: 36}},
		Results: []onboardingDemoResult{{Title: "Everything", Selected: true}},
	})
	stack := window.(woxwidget.Clip).Child.(woxwidget.Stack)
	if image := stack.Children[0].Child.(woxwidget.Image); image.Source != backdrop {
		t.Fatal("demo window did not use the blurred wallpaper")
	}
	if query := stack.Children[2].Child.(woxwidget.Container); query.Color.A != 0 {
		t.Fatalf("rendered query alpha = %d, want theme alpha 0", query.Color.A)
	}
	if result := stack.Children[3].Child.(woxwidget.Container); result.Color.A != 36 {
		t.Fatalf("rendered selected result alpha = %d, want theme alpha 36", result.Color.A)
	}
}

func TestOnboardingDemoResultTextIsVerticallyCentered(t *testing.T) {
	row := onboardingDemoResultRow(onboardingDemoWindowProps{
		Width: 400,
		Theme: woxcomponent.Theme{},
	}, onboardingDemoResult{Title: "Everything", Subtitle: "Search Everything files", Tail: "Current time"}, 51, 255).(woxwidget.Container)
	content := row.Child.(woxwidget.Flex)
	text := content.Children[1].(woxwidget.Align)
	column := text.Child.(woxwidget.Flex)
	title := column.Children[0].(woxwidget.TextBlock)
	icon := content.Children[0].(woxwidget.Container)
	tail := content.Children[2].(woxwidget.Container)
	usedWidth := icon.Width + text.Width + tail.Width + content.Gap*2
	availableWidth := row.Width - row.Padding.Left - row.Padding.Right

	if text.Vertical != .5 || title.Height < title.LineHeight || usedWidth > availableWidth {
		t.Fatalf("result text = %#v, title = %#v, used width = %v, available width = %v", text, title, usedWidth, availableWidth)
	}
}

func TestOnboardingDemoHintCardTextIsVerticallyCentered(t *testing.T) {
	card := onboardingDemoHintCard(
		OnboardingProps{Theme: woxcomponent.Theme{}},
		OnboardingStep{},
		"Query Hotkeys",
		"Cmd+Shift+G",
		"github repo",
		580,
		255,
	).(woxwidget.Container)
	content := card.Child.(woxwidget.Stack)
	title := content.Children[0].Child.(woxwidget.Align)
	badge := content.Children[1].Child.(woxwidget.Container)
	expansion := badge.Child.(woxwidget.Align)

	if title.Vertical != .5 || expansion.Vertical != .5 || badge.Padding.Top != 0 {
		t.Fatalf("hint alignment: title=%#v expansion=%#v padding=%#v", title, expansion, badge.Padding)
	}
}

func TestOnboardingPluginStoreUsesSharedWindowMetrics(t *testing.T) {
	window := onboardingPluginStoreWindow(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{}, 700, 320, "wpm install", "Install", 1).(woxwidget.Clip)
	children := window.Child.(woxwidget.Stack).Children
	query := children[1].Child.(woxwidget.Container)
	result := children[2].Child.(woxwidget.Container)
	toolbar := children[len(children)-1].Child.(woxwidget.Container)

	if query.Height != 50 || result.Height != 51 || toolbar.Height != 36 {
		t.Fatalf("plugin store metrics = query %v, result %v, toolbar %v", query.Height, result.Height, toolbar.Height)
	}
}

func TestOnboardingMacDesktopUsesNativeMenuAndCursorGeometry(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS menu bar only")
	}
	desktop := onboardingDemoDesktop(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{}, 640, 360, false, nil).(woxwidget.Clip)
	menu := desktop.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Container).Child.(woxwidget.Stack)
	search := menu.Children[1].Child.(woxwidget.Image)
	timeSlot := menu.Children[2]
	cursor := onboardingDemoCursor(1).(woxwidget.Painter)

	if search.Source == nil || search.Width != 16 || timeSlot.Left-(menu.Children[1].Left+search.Width) != 12 || cursor.Width != 22 || cursor.Height != 30 {
		t.Fatalf("mac desktop geometry = search %#v, time left %v, cursor %#v", search, timeSlot.Left, cursor)
	}
}

func TestOnboardingGlanceRendersQueryAccessoryWhenEnabled(t *testing.T) {
	demo := onboardingGlanceDemo(OnboardingProps{
		GlanceEnabled: true,
		GlanceLabel:   "CPU",
		Labels:        map[string]string{"demo.glance.value": "当前时间", "glance.body": "Status", "demo.glance.provider": "Provider", "glance.enable.body": "Body", "glance.primary": "Primary"},
		Theme:         woxcomponent.Theme{},
	}, OnboardingStep{}, 640, 360).(woxwidget.Clip)
	desktop := demo.Child.(woxwidget.Stack)
	window := desktop.Children[len(desktop.Children)-1].Child.(woxwidget.Clip).Child.(woxwidget.Stack)
	query := window.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Stack)

	if len(query.Children) != 2 {
		t.Fatalf("glance query children = %d, want query text plus accessory", len(query.Children))
	}
	accessory := query.Children[1].Child.(woxwidget.Align)
	if accessory.Horizontal != .5 || accessory.Vertical != .5 {
		t.Fatalf("glance accessory alignment = (%v, %v), want centered", accessory.Horizontal, accessory.Vertical)
	}
}

func TestOnboardingGlanceUsesSharedRichDropdown(t *testing.T) {
	icon := &woxui.Image{Width: 18, Height: 18}
	panel := onboardingGlance(OnboardingProps{
		GlanceEnabled: true, GlanceLabel: "CPU", GlanceValue: "62%", GlanceIcon: icon,
		Labels: map[string]string{"glance.enable": "Glance", "glance.enable.body": "Status", "glance.primary": "Primary"},
		Theme:  woxcomponent.Theme{},
	}, 720, 150).(woxwidget.Container)
	rows := panel.Child.(woxwidget.Flex)
	selectorRow := rows.Children[1].(woxwidget.Stack)
	selectorSlot := selectorRow.Children[1]
	semantics := selectorSlot.Child.(woxwidget.Semantics)
	trigger := semantics.Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	content := trigger.Child.(woxwidget.Flex)

	if trigger.Width != 300 || selectorSlot.Left+trigger.Width > selectorRow.Width || len(content.Children) != 6 {
		t.Fatalf("glance dropdown = left %v, width %v, row width %v, children %d", selectorSlot.Left, trigger.Width, selectorRow.Width, len(content.Children))
	}
	leading := content.Children[0].(woxwidget.Align).Child.(woxwidget.Image)
	trailingSlot := content.Children[4].(woxwidget.Align)
	trailing := trailingSlot.Child.(woxwidget.Text)
	if leading.Source != icon || trailing.Value != "62%" || trailingSlot.Horizontal != 1 {
		t.Fatalf("glance dropdown content = icon %#v, trailing %q", leading.Source, trailing.Value)
	}
}
