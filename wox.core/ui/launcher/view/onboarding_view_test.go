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

func TestOnboardingSingleLineTextSlotsAlignLineHeight(t *testing.T) {
	page := onboardingPage(OnboardingProps{Width: 1040, Height: 800, Theme: woxcomponent.Theme{}}, OnboardingStep{ID: "welcome", Title: "Welcome"}, 728).(woxwidget.Container)
	title := page.Child.(woxwidget.Flex).Children[0].(woxwidget.TextBlock)
	if title.Height != 44 || title.LineHeight != 44 {
		t.Fatalf("page title slot = height %v line height %v, want 44/44", title.Height, title.LineHeight)
	}

	step := onboardingRailStep(OnboardingStep{ID: "welcome", Title: "Welcome"}, 0, 0, 214, nil, woxcomponent.Theme{})
	label := step.(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.TextBlock)
	if label.Height != 18 || label.LineHeight != 18 {
		t.Fatalf("rail step slot = height %v line height %v, want 18/18", label.Height, label.LineHeight)
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

func TestOnboardingChromeUsesOnlyInteriorDividers(t *testing.T) {
	props := OnboardingProps{
		Width: 1040, Height: 800, ActiveStep: 0,
		Steps:  []OnboardingStep{{ID: "welcome", Title: "Welcome"}},
		Labels: map[string]string{"title": "Set up Wox", "subtitle": "Quick setup", "skip": "跳过", "back": "Back", "next": "Next"},
		Theme:  woxcomponent.Theme{},
	}
	rail, ok := onboardingRail(props, 0, 728).(woxwidget.Stack)
	if !ok || len(rail.Children) != 2 || !rail.Children[1].AnchorRight || !rail.Children[1].StretchHeight {
		t.Fatalf("rail = %#v, want content plus right divider", rail)
	}
	footer, ok := onboardingFooter(props, 0).(woxwidget.Stack)
	if !ok || len(footer.Children) != 2 {
		t.Fatalf("footer = %#v, want content plus top divider", footer)
	}
	content := footer.Children[0].Child.(woxwidget.Container)
	if content.Color.A != 0 {
		t.Fatalf("footer fill alpha = %d, want the window canvas material", content.Color.A)
	}
	actions := content.Child.(woxwidget.Flex)
	skip := actions.Children[0].(woxwidget.Semantics)
	gesture := focusedControlGesture(skip)
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

func TestOnboardingHeaderAreasStartWindowDragging(t *testing.T) {
	dragged := false
	props := OnboardingProps{
		Width: 1040, Height: 800, ActiveStep: 0, OnDrag: func() { dragged = true },
		Steps:  []OnboardingStep{{ID: "welcome", Title: "Welcome"}},
		Labels: map[string]string{"title": "Set up Wox", "subtitle": "Quick setup"},
		Theme:  woxcomponent.Theme{},
	}
	rail, ok := onboardingRail(props, 0, 728).(woxwidget.Stack)
	if !ok || len(rail.Children) != 3 {
		t.Fatalf("rail = %#v, want drag strip, content, and divider", rail)
	}
	railDrag := rail.Children[0].Child.(woxwidget.Gesture)
	railDrag.OnDragStart()
	if !dragged {
		t.Fatal("rail header did not start window dragging")
	}
	if railDrag.ID != "onboarding-rail-drag" {
		t.Fatalf("rail drag id = %q, want onboarding-rail-drag", railDrag.ID)
	}
	if drag := railDrag.Child.(woxwidget.Container); drag.Width != OnboardingSidebarWidth || drag.Height != OnboardingRailDragHeight {
		t.Fatalf("rail drag size = %vx%v, want %vx%v", drag.Width, drag.Height, OnboardingSidebarWidth, OnboardingRailDragHeight)
	}

	dragged = false
	page, ok := onboardingPage(props, props.Steps[0], 728).(woxwidget.Stack)
	if !ok || len(page.Children) != 2 {
		t.Fatalf("page = %#v, want drag strip and content", page)
	}
	pageDrag := page.Children[0].Child.(woxwidget.Gesture)
	pageDrag.OnDragStart()
	if !dragged {
		t.Fatal("page header did not start window dragging")
	}
	if pageDrag.ID != "onboarding-page-drag" {
		t.Fatalf("page drag id = %q, want onboarding-page-drag", pageDrag.ID)
	}
	if drag := pageDrag.Child.(woxwidget.Container); drag.Width != 784 || drag.Height != OnboardingPageDragHeight {
		t.Fatalf("page drag size = %vx%v, want 784x%v", drag.Width, drag.Height, OnboardingPageDragHeight)
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
	if len(center.Children) != 7 || content.Children[0].Left <= 0 {
		t.Fatalf("centered taskbar apps = %d at left %v, want seven centered icons", len(center.Children), content.Children[0].Left)
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
		onboardingFinishDemo(OnboardingProps{Theme: woxcomponent.Theme{}, Labels: map[string]string{}}, OnboardingStep{}, 640, 360).(woxwidget.Clip),
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
	window := onboardingPluginStoreWindow(OnboardingProps{Theme: woxcomponent.Theme{}}, OnboardingStep{}, 700, 320, "wpm install", "Install", 1).(woxwidget.Clip)
	children := window.Child.(woxwidget.Stack).Children
	query := children[2].Child.(woxwidget.Container)
	result := children[3].Child.(woxwidget.Container)
	toolbar := children[len(children)-2].Child.(woxwidget.Container)

	if query.Height != 55 || result.Height != 56 || toolbar.Height != 40 {
		t.Fatalf("plugin store metrics = query %v, result %v, toolbar %v", query.Height, result.Height, toolbar.Height)
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
	trigger := focusedControlGesture(semantics).Child.(woxwidget.Container)
	content := trigger.Child.(woxwidget.Flex)

	if trigger.Width != 300 || !selectorSlot.AnchorRight || len(content.Children) != 6 {
		t.Fatalf("glance dropdown = anchor %v, width %v, children %d", selectorSlot.AnchorRight, trigger.Width, len(content.Children))
	}
	leading := content.Children[0].(woxwidget.Align).Child.(woxwidget.Image)
	trailingSlot := content.Children[4].(woxwidget.Align)
	trailing := trailingSlot.Child.(woxwidget.Text)
	if leading.Source != icon || trailing.Value != "62%" || trailingSlot.Horizontal != 1 {
		t.Fatalf("glance dropdown content = icon %#v, trailing %q", leading.Source, trailing.Value)
	}
}
