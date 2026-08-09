package view

import (
	"runtime"
	"strings"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const demoQueryTypingInterval = 50 * time.Millisecond

type onboardingDemoResult struct {
	Title      string
	Subtitle   string
	Tail       string
	Glyph      string
	GlyphColor woxui.Color
	Selected   bool
}

type onboardingDemoQueryPart struct {
	Text  string
	Color woxui.Color
}

type onboardingDemoWindowProps struct {
	Width          float32
	Height         float32
	Backdrop       *woxui.Image
	Query          string
	QueryParts     []onboardingDemoQueryPart
	Results        []onboardingDemoResult
	Accent         woxui.Color
	Theme          woxcomponent.Theme
	Opacity        float32
	ShowQuery      bool
	ShowToolbar    bool
	ToolbarPressed bool
	ActionProgress float32
	ActionCopy     string
	ActionMore     string
	FadeResults    bool
	ResultsOpacity float32
	Background     woxui.Color
	ResultWidth    float32
	Preview        woxwidget.Widget
	PrimaryAction  string
	QueryAccessory woxwidget.Widget
}

// onboardingPreview selects the Flutter-equivalent scene instead of reusing one generic launcher mock.
func onboardingPreview(props OnboardingProps, step OnboardingStep, width, height float32) woxwidget.Widget {
	duration := onboardingDemoDuration(step.ID)
	if duration == 0 {
		return onboardingDemoScene(props, step, width, height, 1)
	}
	return woxwidget.LoopAnimation{
		Key:      woxwidget.Key("onboarding-demo-" + step.ID),
		Duration: duration,
		Builder: func(progress float32) woxwidget.Widget {
			return onboardingDemoScene(props, step, width, height, progress)
		},
	}
}

func onboardingDemoDuration(stepID string) time.Duration {
	switch stepID {
	case "welcome":
		return 9500 * time.Millisecond
	case "mainHotkey":
		return 4200 * time.Millisecond
	case "selectionHotkey":
		return 5200 * time.Millisecond
	case "queryHotkeys":
		return 9200 * time.Millisecond
	case "queryHotkeysNormal", "queryHotkeysWebPanel", "queryHotkeysSilent":
		return 4600 * time.Millisecond
	case "queryShortcuts":
		return 4400 * time.Millisecond
	case "trayQueries":
		return 5000 * time.Millisecond
	case "wpmInstall":
		return 6200 * time.Millisecond
	case "themeInstall":
		return 5600 * time.Millisecond
	default:
		return 0
	}
}

func onboardingDemoScene(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	switch step.ID {
	case "welcome":
		return onboardingWelcomeDemo(props, step, width, height, progress)
	case "permissions":
		return onboardingPermissionsDemo(props, step, width, height)
	case "mainHotkey":
		return onboardingMainHotkeyDemo(props, step, width, height, progress)
	case "selectionHotkey":
		return onboardingSelectionHotkeyDemo(props, step, width, height, progress)
	case "glance":
		return onboardingGlanceDemo(props, step, width, height)
	case "queryHotkeys":
		return onboardingQueryHotkeysDemo(props, step, width, height, progress)
	case "queryHotkeysNormal":
		step.ID = "queryHotkeys"
		return onboardingQueryHotkeysDemo(props, step, width, height, progress*.42)
	case "queryHotkeysWebPanel":
		step.ID = "queryHotkeys"
		return onboardingQueryHotkeysDemo(props, step, width, height, .50+progress*.44)
	case "queryHotkeysSilent":
		return onboardingQueryHotkeySilentDemo(props, step, width, height, progress)
	case "queryShortcuts":
		return onboardingQueryShortcutsDemo(props, step, width, height, progress)
	case "trayQueries":
		return onboardingTrayQueriesDemo(props, step, width, height, progress)
	case "wpmInstall":
		return onboardingPluginStoreDemo(props, step, width, height, progress)
	case "themeInstall":
		return onboardingThemeInstallDemo(props, step, width, height, progress)
	default:
		return onboardingFinishDemo(props, step, width, height)
	}
}

// DemoPreview reuses the onboarding animation scenes in compact settings popovers.
func DemoPreview(props OnboardingProps, step OnboardingStep, width, height float32) woxwidget.Widget {
	return onboardingPreview(props, step, width, height)
}

// onboardingDemoDesktop reproduces the simulated desktop chrome shared by Flutter demos.
func onboardingDemoDesktop(props OnboardingProps, step OnboardingStep, width, height float32, showDefaultIcons bool, foreground []woxwidget.StackChild) woxwidget.Widget {
	children := []woxwidget.StackChild{{Child: woxwidget.Container{Width: width, Height: height, Radius: 8, Color: onboardingDemoDesktopBaseColor(props.Theme.Background)}}}
	if props.Wallpaper != nil {
		children = append(children,
			woxwidget.StackChild{Child: woxwidget.Image{Source: props.Wallpaper, Width: width, Height: height, Radius: 8}},
			woxwidget.StackChild{Child: woxwidget.Container{Width: width, Height: height, Radius: 8, Color: woxui.Color{A: 87}}},
		)
	}
	if showDefaultIcons {
		children = append(children,
			woxwidget.StackChild{Left: 28, Top: 44, Child: onboardingDemoDesktopIcon("Apps", "▦", step.Accent, props.Theme)},
			woxwidget.StackChild{Left: 28, Top: 120, Child: onboardingDemoDesktopIcon("Files", "◆", woxui.Color{R: 250, G: 204, B: 21, A: 255}, props.Theme)},
		)
	}
	if runtime.GOOS == "darwin" {
		menuWidth := width - 28
		children = append(children, woxwidget.StackChild{Child: woxwidget.Container{
			Width: width, Height: 28, Radius: 8, Color: settingsColorAlpha(props.Theme.Background, 220),
			Padding: woxwidget.Insets{Left: 14, Top: 7, Right: 14},
			Child: woxwidget.Stack{Width: menuWidth, Height: 16, Children: []woxwidget.StackChild{
				{Child: woxwidget.Text{Value: "   Finder     File", Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}},
				{Left: max(float32(0), menuWidth-60), Child: onboardingDemoSearchIcon(settingsColorAlpha(props.Theme.ResultTitle, 184))},
				{Left: max(float32(0), menuWidth-32), Child: woxwidget.Align{Width: 32, Height: 16, Horizontal: 1, Vertical: .5, Child: woxwidget.Text{Value: "09:41", Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(props.Theme.ResultTitle, 199)}}},
			}},
		}})
	} else {
		children = append(children, woxwidget.StackChild{AnchorBottom: true, Child: onboardingDemoWindowsTaskbar(props, width)})
	}
	children = append(children, foreground...)
	return woxwidget.Clip{Width: width, Height: height, Child: woxwidget.Stack{Width: width, Height: height, Children: children}}
}

// onboardingDemoDesktopBaseColor matches Flutter's darkened fallback surface under the wallpaper.
func onboardingDemoDesktopBaseColor(background woxui.Color) woxui.Color {
	if background.A == 0 {
		return woxui.Color{A: 255}
	}
	const backgroundDarkening = float32(.38)
	background.R = uint8(float32(background.R) * (1 - backgroundDarkening))
	background.G = uint8(float32(background.G) * (1 - backgroundDarkening))
	background.B = uint8(float32(background.B) * (1 - backgroundDarkening))
	background.A = demoScaledAlpha(.88, background.A)
	return background
}

// onboardingDemoSearchIcon reuses the shared SVG search icon.
func onboardingDemoSearchIcon(color woxui.Color) woxwidget.Widget {
	return woxcomponent.SearchGlyph(16, color)
}

// onboardingDemoClockIcon reuses the shared SVG clock icon.
func onboardingDemoClockIcon(color woxui.Color) woxwidget.Widget {
	return woxcomponent.ClockGlyph(16, color)
}

// onboardingDemoWindowsTaskbar mirrors the centered app strip and right system tray of Windows 11.
func onboardingDemoWindowsTaskbar(props OnboardingProps, width float32) woxwidget.Widget {
	const (
		taskbarHeight = float32(42)
		contentInset  = float32(12)
		iconSize      = float32(24)
		iconGap       = float32(6)
		trayWidth     = float32(180)
	)
	textColor := settingsColorAlpha(props.Theme.ResultTitle, 212)
	mutedColor := settingsColorAlpha(props.Theme.ResultTitle, 176)
	appIcon := func(glyph string, background, color woxui.Color) woxwidget.Widget {
		return onboardingDemoWindowsTaskbarIcon(woxwidget.Text{Value: glyph, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: color}, background)
	}
	apps := []woxwidget.Widget{
		onboardingDemoWindowsTaskbarIcon(woxcomponent.WindowsGlyph(16, woxui.Color{R: 77, G: 190, B: 245, A: 255}), woxui.Color{}),
		onboardingDemoWindowsTaskbarIcon(woxcomponent.FolderGlyph(16, woxui.Color{R: 247, G: 190, B: 46, A: 255}), woxui.Color{}),
		onboardingDemoWindowsTaskbarIcon(woxcomponent.BrowserGlyph(16, woxui.Color{R: 50, G: 145, B: 235, A: 255}), woxui.Color{}),
		onboardingDemoWindowsTaskbarIcon(woxcomponent.TerminalGlyph(15, woxui.Color{R: 232, G: 235, B: 238, A: 255}), woxui.Color{}),
		onboardingDemoWindowsTaskbarIcon(woxcomponent.SparklesGlyph(15, woxui.Color{R: 240, G: 240, B: 240, A: 255}), woxui.Color{R: 45, G: 45, B: 50, A: 255}),
		onboardingDemoWindowsTaskbarIcon(woxcomponent.CodeGlyph(16, woxui.Color{R: 55, G: 165, B: 240, A: 255}), woxui.Color{}),
	}
	if props.AppIcon != nil {
		apps = append(apps, onboardingDemoWindowsTaskbarIcon(woxwidget.Image{Source: props.AppIcon, Width: 18, Height: 18, Radius: 4}, woxui.Color{R: 245, G: 245, B: 245, A: 255}))
	} else {
		apps = append(apps, appIcon("W", woxui.Color{R: 45, G: 45, B: 50, A: 255}, woxui.Color{R: 245, G: 245, B: 245, A: 255}))
	}
	centerWidth := float32(len(apps))*iconSize + float32(len(apps)-1)*iconGap
	center := woxwidget.Container{
		Width: centerWidth, Height: iconSize,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: iconGap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: apps},
	}
	tray := woxwidget.Align{
		Width: trayWidth, Height: iconSize, Horizontal: 1, Vertical: .5,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.WifiGlyph(13, mutedColor),
			woxcomponent.VolumeGlyph(13, mutedColor),
			woxcomponent.BluetoothGlyph(13, mutedColor),
			woxwidget.Text{Value: "ENG", Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: textColor},
			woxwidget.Container{Width: 48, Height: iconSize, Child: woxwidget.Flex{Axis: woxwidget.Vertical, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "09:41", Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: textColor},
				woxwidget.Text{Value: "2026/8/8", Style: woxui.TextStyle{Size: 7}, Color: mutedColor},
			}}},
		}},
	}
	contentWidth := max(float32(0), width-contentInset*2)
	return woxwidget.Container{
		Width: width, Height: taskbarHeight, Radius: 8, Color: settingsColorAlpha(props.Theme.Background, 198),
		BorderColor: settingsColorAlpha(props.Theme.ResultTitle, 16), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: contentInset, Top: 9, Right: contentInset, Bottom: 9},
		Child: woxwidget.Stack{Width: contentWidth, Height: iconSize, Children: []woxwidget.StackChild{
			{Left: max(float32(0), (contentWidth-centerWidth)/2), Child: center},
			{Right: 0, AnchorRight: true, Child: tray},
		}},
	}
}

// onboardingDemoWindowsTaskbarIcon gives simulated pinned apps the compact square silhouette used by Windows.
func onboardingDemoWindowsTaskbarIcon(child woxwidget.Widget, background woxui.Color) woxwidget.Widget {
	return woxwidget.Container{Width: 24, Height: 24, Radius: 5, Color: background, Child: woxwidget.Align{Width: 24, Height: 24, Horizontal: .5, Vertical: .5, Child: child}}
}

// onboardingDemoCursor draws the outlined arrow used by the macOS pointer.
func onboardingDemoCursor(opacity float32) woxwidget.Widget {
	return woxwidget.Painter{Width: 22, Height: 30, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		draw := func(points []woxui.Point, color woxui.Color) {
			for _, indices := range [][3]int{{0, 1, 2}, {0, 2, 5}, {2, 3, 4}, {2, 4, 5}, {0, 5, 6}} {
				displayList.FillConvexPolygon([]woxui.Point{points[indices[0]], points[indices[1]], points[indices[2]]}, color)
			}
		}
		point := func(x, y float32) woxui.Point {
			return woxui.Point{X: bounds.X + x, Y: bounds.Y + y}
		}
		alpha := demoAlpha(opacity)
		draw([]woxui.Point{
			point(1, 1), point(1, 24), point(7, 18), point(13, 29), point(18, 26), point(12, 16), point(21, 16),
		}, woxui.Color{R: 255, G: 255, B: 255, A: alpha})
		draw([]woxui.Point{
			point(3, 4), point(3, 20), point(7.5, 15.5), point(13.5, 26.5), point(15.5, 25.3), point(9.7, 14), point(17.5, 14),
		}, woxui.Color{A: alpha})
	}}
}

func onboardingDemoDesktopIcon(label, glyph string, accent woxui.Color, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: 58, Height: 64, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 38, Height: 34, Radius: 8, Color: settingsColorAlpha(accent, 190), Child: woxwidget.Align{
			Width: 38, Height: 34, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: glyph, Style: woxui.TextStyle{Size: 19}, Color: woxui.Color{R: 255, G: 255, B: 255, A: 245}},
		}},
		woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(theme.ResultTitle, 210)},
	}}}
}

// onboardingDemoWindow mirrors the query bar, result density, toolbar, and optional action panel used by Flutter's WoxDemoWindow.
func onboardingDemoWindow(window onboardingDemoWindowProps) woxwidget.Widget {
	opacity := min(max(float32(0), window.Opacity), float32(1))
	alpha := demoAlpha(opacity)
	background := window.Background
	if background.A == 0 {
		background = window.Theme.Background
	}
	const queryTop = float32(12)
	const queryBoxHeight = float32(50)
	const queryGap = float32(10)
	const resultRowHeight = float32(51)
	const toolbarHeight = float32(36)
	resultTop := queryTop
	if window.ShowQuery {
		resultTop += queryBoxHeight + queryGap
	}
	footerHeight := float32(0)
	if window.ShowToolbar {
		footerHeight = toolbarHeight
	}
	resultListHeight := min(float32(len(window.Results))*resultRowHeight, max(float32(0), window.Height-resultTop-footerHeight))
	renderHeight := window.Height
	if window.ShowToolbar || !window.ShowQuery {
		renderHeight = min(renderHeight, resultTop+resultListHeight+footerHeight)
	}
	resultWidth := window.Width
	if window.Preview != nil && window.ResultWidth > 0 {
		resultWidth = min(window.ResultWidth, window.Width-120)
	}
	resultWindow := window
	resultWindow.Width = resultWidth
	children := []woxwidget.StackChild{}
	if window.Backdrop != nil {
		children = append(children, woxwidget.StackChild{Child: woxwidget.Image{Source: window.Backdrop, Width: window.Width, Height: renderHeight}})
	}
	windowSurface := woxwidget.Container{
		Width: window.Width, Height: renderHeight, Radius: 8,
		Color:       demoColorOpacity(onboardingDemoMicaColor(background), opacity),
		BorderColor: settingsColorAlpha(window.Theme.ResultTitle, demoScaledAlpha(opacity, 28)), BorderWidth: 1,
	}
	if window.Backdrop == nil {
		children = append(children, woxwidget.StackChild{Child: woxwidget.Stack{Width: window.Width, Height: renderHeight, Children: []woxwidget.StackChild{
			{Child: woxwidget.Container{Width: window.Width, Height: renderHeight, Color: woxui.Color{A: 255}}},
			{Child: windowSurface},
		}}})
	} else {
		children = append(children, woxwidget.StackChild{Child: windowSurface})
	}
	if window.ShowQuery {
		var query woxwidget.Widget
		if len(window.QueryParts) > 0 {
			parts := make([]woxwidget.Widget, 0, len(window.QueryParts))
			for _, part := range window.QueryParts {
				parts = append(parts, woxwidget.Text{Value: part.Text, Style: woxui.TextStyle{Size: 25, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(part.Color, alpha)})
			}
			query = woxwidget.Flex{Axis: woxwidget.Horizontal, Children: parts}
		} else {
			query = woxwidget.TextBlock{Value: window.Query, Height: 32, MaxLines: 1, LineHeight: 32, Style: woxui.TextStyle{Size: 25}, Color: settingsColorAlpha(window.Theme.QueryText, alpha)}
		}
		queryChildren := []woxwidget.Widget{woxwidget.Expanded{Child: woxwidget.Align{Height: 32, Vertical: .5, Child: query}}}
		if window.QueryAccessory != nil {
			queryChildren = append(queryChildren, window.QueryAccessory)
		}
		children = append(children, woxwidget.StackChild{Left: 12, Top: queryTop, Right: 12, StretchWidth: true, Child: woxwidget.Container{
			Height: queryBoxHeight, Radius: 8, Color: demoColorOpacity(window.Theme.QueryBackground, opacity),
			Padding: woxwidget.Insets{Left: 8, Top: 9, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: queryChildren},
		}})
	}
	for index, result := range window.Results {
		top := resultTop + float32(index)*resultRowHeight
		resultAlpha := alpha
		if window.FadeResults {
			resultAlpha = demoScaledAlpha(opacity*window.ResultsOpacity, 255)
		}
		children = append(children, woxwidget.StackChild{Left: 12, Top: top, Right: 12, StretchWidth: true, Child: onboardingDemoResultRow(resultWindow, result, resultRowHeight, resultAlpha)})
	}
	if window.Preview != nil {
		children = append(children, woxwidget.StackChild{Left: resultWidth + 2, Top: resultTop + 4, Child: window.Preview})
	}
	if window.ShowToolbar {
		children = append(children, woxwidget.StackChild{Top: renderHeight - footerHeight, Child: onboardingDemoToolbar(window, footerHeight, alpha)})
	}
	if window.ActionProgress > .01 {
		panelWidth := min(float32(250), window.Width*.42)
		panelHeight := min(float32(176), renderHeight-footerHeight-24)
		children = append(children, woxwidget.StackChild{
			Left:  window.Width - panelWidth - 16 + 18*(1-window.ActionProgress),
			Top:   renderHeight - footerHeight - panelHeight - 12 + 10*(1-window.ActionProgress),
			Child: onboardingDemoActionPanel(window, panelWidth, panelHeight, demoAlpha(window.ActionProgress)),
		})
	}
	return woxwidget.Clip{Width: window.Width, Height: renderHeight, Child: woxwidget.Stack{Width: window.Width, Height: renderHeight, Children: children}}
}

func onboardingDemoResultRow(window onboardingDemoWindowProps, result onboardingDemoResult, height float32, alpha uint8) woxwidget.Widget {
	background := woxui.Color{}
	if result.Selected {
		background = demoColorOpacity(window.Theme.SelectedBackground, float32(alpha)/255)
	}
	tailWidth := float32(0)
	if result.Tail != "" {
		tailWidth = min(float32(120), float32(len([]rune(result.Tail)))*7+18)
	}
	textHeight := max(float32(28), height-10)
	return woxwidget.Container{
		Height: height, Radius: 6, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 6, Right: 10, Bottom: 4},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 9, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Container{Width: 28, Height: 28, Radius: 7, Color: settingsColorAlpha(result.GlyphColor, demoScaledAlpha(float32(alpha)/255, 54)), Child: woxwidget.Align{
				Width: 28, Height: 28, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: result.Glyph, Style: woxui.TextStyle{Size: 15}, Color: settingsColorAlpha(result.GlyphColor, alpha)},
			}},
			woxwidget.Expanded{Child: woxwidget.Align{Height: textHeight, Vertical: .5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
				woxwidget.TextBlock{Value: result.Title, Height: 19, MaxLines: 1, LineHeight: 19, Style: woxui.TextStyle{Size: 14}, Color: settingsColorAlpha(boolColor(result.Selected, window.Theme.SelectedTitle, window.Theme.ResultTitle), alpha)},
				woxwidget.TextBlock{Value: result.Subtitle, Height: 15, MaxLines: 1, LineHeight: 15, Style: woxui.TextStyle{Size: 11}, Color: settingsColorAlpha(boolColor(result.Selected, window.Theme.SelectedSubtitle, window.Theme.ResultSubtitle), alpha)},
			}}}},
			woxwidget.Container{Width: tailWidth, Height: 24, Radius: 12, BorderColor: settingsColorAlpha(window.Theme.ResultSubtitle, demoScaledAlpha(float32(alpha)/255, 90)), BorderWidth: boolFloat(result.Tail != ""),
				Child: woxwidget.Align{Width: tailWidth, Height: 24, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: result.Tail, Style: woxui.TextStyle{Size: 9}, Color: settingsColorAlpha(window.Theme.ResultSubtitle, alpha)}}},
		}},
	}
}

func onboardingDemoToolbar(window onboardingDemoWindowProps, height float32, alpha uint8) woxwidget.Widget {
	moreColor := window.Theme.ToolbarText
	if window.ToolbarPressed {
		moreColor = window.Accent
	}
	primaryAction := window.PrimaryAction
	if primaryAction == "" {
		primaryAction = "Execute"
	}
	keycap := func(label string, width float32, active bool) woxwidget.Widget {
		border := settingsColorAlpha(window.Theme.ResultTitle, demoScaledAlpha(float32(alpha)/255, 120))
		fill := settingsColorAlpha(window.Theme.ResultTitle, demoScaledAlpha(float32(alpha)/255, 9))
		if active {
			border = settingsColorAlpha(window.Accent, demoScaledAlpha(float32(alpha)/255, 200))
			fill = settingsColorAlpha(window.Accent, demoScaledAlpha(float32(alpha)/255, 28))
		}
		return woxwidget.Container{
			Width: width, Height: 22, Radius: 4, Color: fill, BorderColor: border, BorderWidth: 1,
			Child: woxwidget.Align{Width: width, Height: 22, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{
				Value: label, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(window.Theme.ResultTitle, alpha),
			}},
		}
	}
	actionModifier := "Ctrl"
	if runtime.GOOS == "darwin" {
		actionModifier = "Cmd"
	}
	content := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Text{Value: primaryAction, Style: woxui.TextStyle{Size: 10}, Color: settingsColorAlpha(window.Theme.ResultTitle, alpha)},
		keycap("Enter", 40, false),
		woxwidget.Container{Width: 6},
		woxwidget.Text{Value: "More Actions", Style: woxui.TextStyle{Size: 10}, Color: settingsColorAlpha(moreColor, alpha)},
		keycap(actionModifier, 36, window.ToolbarPressed),
		keycap("J", 24, window.ToolbarPressed),
	}}
	return woxwidget.Container{
		Width: window.Width, Height: height, Color: settingsColorAlpha(window.Theme.ResultTitle, demoScaledAlpha(float32(alpha)/255, 9)),
		BorderColor: settingsColorAlpha(window.Theme.ResultTitle, demoScaledAlpha(float32(alpha)/255, 18)), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 12, Top: 7, Right: 12},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisEnd, Children: []woxwidget.Widget{
			content,
		}},
	}
}

func onboardingDemoActionPanel(window onboardingDemoWindowProps, width, height float32, alpha uint8) woxwidget.Widget {
	copyLabel := window.ActionCopy
	if copyLabel == "" {
		copyLabel = "Copy"
	}
	moreLabel := window.ActionMore
	if moreLabel == "" {
		moreLabel = "More"
	}
	rows := []struct {
		glyph string
		title string
	}{{"▶", "Execute"}, {"⌖", copyLabel}, {"•••", moreLabel}}
	children := []woxwidget.Widget{
		woxwidget.Text{Value: "Actions", Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(window.Theme.ResultTitle, alpha)},
		woxwidget.Constrained{FillWidth: true, Child: woxwidget.Container{Height: 1, Color: settingsColorAlpha(window.Theme.ResultTitle, demoScaledAlpha(float32(alpha)/255, 138))}},
	}
	for index, row := range rows {
		color := settingsColorAlpha(window.Theme.ResultTitle, demoScaledAlpha(float32(alpha)/255, 14))
		if index == 0 {
			color = settingsColorAlpha(window.Accent, demoScaledAlpha(float32(alpha)/255, 210))
		}
		children = append(children, woxwidget.Constrained{FillWidth: true, Child: woxwidget.Container{
			Height: 32, Radius: 7, Color: color, Padding: woxwidget.Insets{Left: 9, Top: 8},
			Child: woxwidget.Text{Value: row.glyph + "   " + row.title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(window.Theme.ResultTitle, alpha)},
		}})
	}
	return woxwidget.Container{
		Width: width, Height: height, Radius: 8, Color: settingsColorAlpha(window.Theme.Background, demoScaledAlpha(float32(alpha)/255, 240)),
		BorderColor: settingsColorAlpha(window.Theme.ResultTitle, demoScaledAlpha(float32(alpha)/255, 18)), BorderWidth: 1,
		Padding: woxwidget.UniformInsets(12), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: children},
	}
}

func onboardingDemoHintCard(props OnboardingProps, step OnboardingStep, title, from, to string, width float32, alpha uint8) woxwidget.Widget {
	badgeWidth := min(width*.48, max(float32(160), float32(len([]rune(from+to)))*7+48))
	return woxwidget.Container{
		Width: width, Height: 58, Radius: 8, Color: settingsColorAlpha(props.Theme.Background, demoScaledAlpha(float32(alpha)/255, 238)),
		BorderColor: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(float32(alpha)/255, 26)), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 14, Top: 11, Right: 12},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Expanded{Child: woxwidget.Align{Height: 36, Vertical: .5, Child: woxwidget.TextBlock{Value: "⌘  " + title, MaxLines: 1, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(props.Theme.ResultTitle, alpha)}}},
			woxwidget.Container{Width: badgeWidth, Height: 36, Radius: 8, BorderColor: settingsColorAlpha(step.Accent, demoScaledAlpha(float32(alpha)/255, 82)), BorderWidth: 1,
				Padding: woxwidget.Insets{Left: 10, Right: 10}, Child: woxwidget.Align{Height: 36, Vertical: .5, Child: woxwidget.TextBlock{Value: from + "   →   " + to, MaxLines: 1, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(step.Accent, alpha)}}},
		}},
	}
}

func onboardingDemoHotkey(labels []string, accent woxui.Color, theme woxcomponent.Theme, pressed bool, opacity float32) woxwidget.Widget {
	alpha := demoAlpha(opacity)
	if len(labels) == 0 {
		labels = []string{"Option", "Space"}
	}
	children := make([]woxwidget.Widget, 0, len(labels)*2-1)
	for index, label := range labels {
		if index > 0 {
			children = append(children, woxwidget.Text{Value: "+", Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(theme.ResultSubtitle, alpha)})
		}
		border := theme.ResultSubtitle
		color := settingsColorAlpha(theme.ResultTitle, demoScaledAlpha(opacity, 14))
		if pressed {
			border = accent
			color = settingsColorAlpha(accent, demoScaledAlpha(opacity, 52))
		}
		keyWidth := max(float32(48), float32(len([]rune(label)))*7+22)
		children = append(children, woxwidget.Container{
			Width: keyWidth, Height: 34, Radius: 7, Color: color, BorderColor: settingsColorAlpha(border, alpha), BorderWidth: 1,
			Child: woxwidget.Align{Width: keyWidth, Height: 34, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(theme.ResultTitle, alpha)}},
		})
	}
	totalWidth := float32(32)
	for _, child := range children {
		switch current := child.(type) {
		case woxwidget.Container:
			totalWidth += current.Width
		default:
			totalWidth += 20
		}
	}
	return woxwidget.Container{
		Width: totalWidth, Height: 62, Radius: 14, Color: settingsColorAlpha(theme.Background, demoScaledAlpha(opacity, 224)),
		BorderColor: settingsColorAlpha(accent, demoScaledAlpha(opacity, boolAlpha(pressed, 224, 44))), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 16, Top: 14, Right: 16, Bottom: 14}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children},
	}
}

// onboardingWelcomeDemo follows Flutter's concept-card, flying-token, query, results, and action-panel timeline.
func onboardingWelcomeDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	cardOpacity := float32(1)
	if progress >= .20 {
		cardOpacity = 1 - demoEaseInCubic(demoInterval(progress, .20, .40))
	}
	windowProgress := demoEaseOutCubic(demoInterval(progress, .28, .45))
	query := demoTypedQuery("wpm install everything", progress, .42, onboardingDemoDuration(step.ID))
	resultsOpacity := demoEaseOutCubic(demoInterval(progress, .57, .65))
	actionProgress := float32(0)
	if progress >= .72 && progress < .79 {
		actionProgress = demoEaseOutCubic(demoInterval(progress, .72, .79))
	} else if progress >= .79 && progress < .95 {
		actionProgress = 1
	} else if progress >= .95 && progress < .99 {
		actionProgress = 1 - demoEaseInCubic(demoInterval(progress, .95, .99))
	}
	children := []woxwidget.StackChild{}
	if cardOpacity > .01 {
		cardWidth := min(float32(500), width-80)
		children = append(children, woxwidget.StackChild{
			Left: (width - cardWidth) / 2, Top: height*.24 - 30*(1-cardOpacity),
			Child: onboardingQueryConceptCard(props, step, cardWidth, cardOpacity),
		})
	}
	if progress >= .20 && progress < .42 {
		fly := demoEaseInCubic(demoInterval(progress, .20, .42))
		children = append(children, onboardingFlyingConceptTokens(props, step, width, height, fly)...)
	}
	if windowProgress > .01 {
		windowWidth := max(float32(320), width-100)
		windowHeight := max(float32(190), height-90)
		parts := onboardingColoredQueryParts(query, step.Accent)
		results := []onboardingDemoResult{
			{Title: "Everything", Subtitle: props.Labels["demo.concept.result1.subtitle"], Glyph: "⌕", GlyphColor: step.Accent, Selected: true},
			{Title: "Everything (portable)", Subtitle: props.Labels["demo.concept.result2.subtitle"], Glyph: "⌕", GlyphColor: settingsColorAlpha(step.Accent, 170)},
			{Title: "Everything-cli", Subtitle: props.Labels["demo.concept.result3.subtitle"], Glyph: ">_", GlyphColor: woxui.Color{R: 148, G: 163, B: 184, A: 255}},
		}
		for index := range results {
			results[index].GlyphColor = settingsColorAlpha(results[index].GlyphColor, demoAlpha(resultsOpacity))
		}
		children = append(children, woxwidget.StackChild{
			Left: 48, Top: 34 + 22*(1-windowProgress),
			Child: onboardingDemoWindow(onboardingDemoWindowProps{
				Width: windowWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, QueryParts: parts, Results: results, Accent: step.Accent, Theme: props.Theme,
				Opacity: windowProgress, ShowQuery: true, ShowToolbar: true, ToolbarPressed: progress >= .70 && progress < .83, ActionProgress: actionProgress,
				ActionCopy: props.Labels["demo.action.copy"], ActionMore: props.Labels["demo.action.more"], FadeResults: true, ResultsOpacity: resultsOpacity,
			}),
		})
	}
	return onboardingDemoDesktop(props, step, width, height, false, children)
}

func onboardingQueryConceptCard(props OnboardingProps, step OnboardingStep, width, opacity float32) woxwidget.Widget {
	tokenWidth := (width - 60) / 3
	tokens := []struct {
		value string
		label string
		color woxui.Color
	}{
		{"wpm", props.Labels["demo.concept.trigger"], step.Accent},
		{"install", props.Labels["demo.concept.command"], woxui.Color{R: 250, G: 204, B: 21, A: 255}},
		{"everything", props.Labels["demo.concept.search"], woxui.Color{R: 74, G: 222, B: 128, A: 255}},
	}
	children := []woxwidget.Widget{woxwidget.Text{Value: props.Labels["demo.concept.title"], Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(props.Theme.ResultSubtitle, demoAlpha(opacity))}}
	row := make([]woxwidget.Widget, 0, len(tokens))
	for _, token := range tokens {
		row = append(row, woxwidget.Container{Width: tokenWidth, Height: 78, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Container{Width: tokenWidth, Height: 29, Radius: 6, Color: settingsColorAlpha(token.color, demoScaledAlpha(opacity, 36)), Child: woxwidget.Align{
				Width: tokenWidth, Height: 29, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: token.value, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(token.color, demoAlpha(opacity))},
			}},
			woxwidget.Container{Width: 1, Height: 10, Color: settingsColorAlpha(token.color, demoScaledAlpha(opacity, 90))},
			woxwidget.TextBlock{Value: token.label, Width: tokenWidth, Height: 15, MaxLines: 1, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(token.color, demoAlpha(opacity))},
			woxwidget.Text{Value: props.Labels["demo.concept.optional"], Style: woxui.TextStyle{Size: 8}, Color: settingsColorAlpha(token.color, demoScaledAlpha(opacity, 164))},
		}}})
	}
	children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: row})
	return woxwidget.Container{
		Width: width, Height: 132, Radius: 10, Color: settingsColorAlpha(props.Theme.Background, demoScaledAlpha(opacity, 234)),
		BorderColor: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 24)), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 20, Top: 16, Right: 20, Bottom: 14}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
	}
}

func onboardingFlyingConceptTokens(props OnboardingProps, step OnboardingStep, width, height, progress float32) []woxwidget.StackChild {
	values := []string{"wpm", "install", "everything"}
	colors := []woxui.Color{step.Accent, {R: 250, G: 204, B: 21, A: 255}, {R: 74, G: 222, B: 128, A: 255}}
	sourceX := []float32{width*.38 - 90, width*.50 - 25, width*.62 + 30}
	targetX := []float32{72, 116, 176}
	alpha := float32(1)
	if progress < .08 {
		alpha = progress / .08
	} else if progress > .70 {
		alpha = 1 - (progress-.70)/.30
	}
	children := make([]woxwidget.StackChild, 0, 3)
	for index, value := range values {
		x := demoLerp(sourceX[index], targetX[index], progress)
		y := demoLerp(height*.40, 72, progress)
		tileWidth := max(float32(42), float32(len(value))*7+18)
		children = append(children, woxwidget.StackChild{Left: x, Top: y, Child: woxwidget.Container{
			Width: tileWidth, Height: 29, Radius: 6, Color: settingsColorAlpha(colors[index], demoScaledAlpha(alpha, 42)),
			Child: woxwidget.Align{Width: tileWidth, Height: 29, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: value, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(colors[index], demoAlpha(alpha))}},
		}})
	}
	return children
}

func onboardingColoredQueryParts(query string, accent woxui.Color) []onboardingDemoQueryPart {
	runes := []rune(query)
	part := func(start, end int, color woxui.Color) onboardingDemoQueryPart {
		end = min(end, len(runes))
		start = min(start, end)
		return onboardingDemoQueryPart{Text: string(runes[start:end]), Color: color}
	}
	parts := []onboardingDemoQueryPart{part(0, 3, accent)}
	if len(runes) > 3 {
		parts = append(parts, part(3, 11, woxui.Color{R: 250, G: 204, B: 21, A: 255}))
	}
	if len(runes) > 11 {
		parts = append(parts, part(11, len(runes), woxui.Color{R: 74, G: 222, B: 128, A: 255}))
	}
	return parts
}

func onboardingMainHotkeyDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	hotkeyProgress := demoEnterHoldExit(progress, .10, .22, .54, .68)
	windowProgress := demoEnterHoldExit(progress, .28, .46, .88, 1)
	labels := props.MainHotkeyLabels
	if len(labels) == 0 {
		labels = demoDefaultHotkey(false)
	}
	children := []woxwidget.StackChild{}
	if hotkeyProgress > .01 {
		hotkey := onboardingDemoHotkey(labels, step.Accent, props.Theme, progress >= .20 && progress <= .34, hotkeyProgress)
		children = append(children, woxwidget.StackChild{Left: (width - demoHotkeyWidth(labels)) / 2, Top: (height-62)/2 + 8*(1-hotkeyProgress), Child: hotkey})
	}
	if windowProgress > .01 {
		windowWidth := width - 68
		windowHeight := height - 84
		children = append(children, woxwidget.StackChild{Left: 34, Top: 42 + 22*(1-windowProgress), Child: onboardingDemoWindow(onboardingDemoWindowProps{
			Width: windowWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, Query: demoTypedQuery("app", progress, .52, onboardingDemoDuration(step.ID)), Accent: step.Accent, Theme: props.Theme, Opacity: windowProgress, ShowQuery: true, ShowToolbar: true,
			Results: []onboardingDemoResult{
				{Title: step.Title, Subtitle: props.Labels[step.ID+".body"], Tail: strings.Join(labels, "+"), Glyph: "⌨", GlyphColor: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Selected: true},
				{Title: "Applications", Subtitle: "Open installed applications", Tail: "Apps", Glyph: "▦", GlyphColor: step.Accent},
				{Title: "Files", Subtitle: "Search files and folders", Tail: "Files", Glyph: "◆", GlyphColor: woxui.Color{R: 250, G: 204, B: 21, A: 255}},
				{Title: "Plugins", Subtitle: "Wox.Plugin.Template.Nodejs", Tail: "51 day ago", Glyph: "⬡", GlyphColor: woxui.Color{R: 96, G: 165, B: 250, A: 255}},
			},
		})})
	}
	return onboardingDemoDesktop(props, step, width, height, true, children)
}

func onboardingSelectionHotkeyDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	cursorProgress := demoEaseInOutCubic(demoInterval(progress, .08, .34))
	hotkeyProgress := demoEnterHoldExit(progress, .36, .46, .66, .78)
	windowProgress := demoEnterHoldExit(progress, .56, .74, .92, 1)
	selected := progress >= .30 && progress < .95
	labels := props.SelectHotkeyLabels
	if len(labels) == 0 {
		labels = demoDefaultHotkey(true)
	}
	children := []woxwidget.StackChild{
		{Left: 42, Top: 54, Child: onboardingDemoFileIcon("Roadmap.md", "▤", woxui.Color{R: 96, G: 165, B: 250, A: 255}, false, props.Theme)},
		{Left: 150, Top: 54, Child: onboardingDemoFileIcon("Quarterly plan.pdf", "PDF", step.Accent, selected, props.Theme)},
		{Left: 258, Top: 54, Child: onboardingDemoFileIcon("Screenshots", "◆", woxui.Color{R: 250, G: 204, B: 21, A: 255}, false, props.Theme)},
		{Left: 64, Top: 150, Child: onboardingDemoFileIcon("Release notes.txt", "≡", woxui.Color{R: 52, G: 211, B: 153, A: 255}, false, props.Theme)},
	}
	cursorOpacity := 1 - demoEaseInCubic(demoInterval(progress, .70, .86))
	if cursorOpacity > .01 {
		children = append(children, woxwidget.StackChild{
			Left: demoLerp(width-96, 186, cursorProgress), Top: demoLerp(height-86, 112, cursorProgress),
			Child: onboardingDemoCursor(cursorOpacity),
		})
	}
	if hotkeyProgress > .01 {
		children = append(children, woxwidget.StackChild{Left: (width - demoHotkeyWidth(labels)) / 2, Top: (height-62)/2 + 8*(1-hotkeyProgress), Child: onboardingDemoHotkey(labels, step.Accent, props.Theme, progress >= .46 && progress <= .58, hotkeyProgress)})
	}
	if windowProgress > .01 {
		windowWidth := min(float32(660), width-72)
		windowHeight := min(float32(330), height*.72)
		children = append(children, woxwidget.StackChild{
			Left: (width - windowWidth) / 2, Top: (height-windowHeight)/2 + 20*(1-windowProgress),
			Child: onboardingSelectionWindow(props, step, windowWidth, windowHeight, windowProgress, strings.Join(labels, "+")),
		})
	}
	return onboardingDemoDesktop(props, step, width, height, false, children)
}

func onboardingDemoFileIcon(label, glyph string, accent woxui.Color, selected bool, theme woxcomponent.Theme) woxwidget.Widget {
	background := woxui.Color{}
	border := woxui.Color{}
	if selected {
		background = settingsColorAlpha(theme.SelectedBackground, 42)
		border = settingsColorAlpha(accent, 160)
	}
	return woxwidget.Container{Width: 86, Height: 82, Radius: 8, Color: background, BorderColor: border, BorderWidth: boolFloat(selected), Padding: woxwidget.Insets{Top: 8},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Container{Width: 42, Height: 42, Radius: 10, Color: settingsColorAlpha(accent, boolAlpha(selected, 230, 184)), Child: woxwidget.Align{
				Width: 42, Height: 42, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: glyph, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 255, G: 255, B: 255, A: 245}},
			}},
			woxwidget.TextBlock{Value: label, Width: 78, Height: 22, MaxLines: 2, Style: woxui.TextStyle{Size: 8, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
		}}}
}

func onboardingSelectionWindow(props OnboardingProps, step OnboardingStep, width, height, opacity float32, hotkey string) woxwidget.Widget {
	leftWidth := width * .40
	footerHeight := float32(38)
	queryHeight := float32(54)
	results := []onboardingDemoResult{
		{Title: "Preview", Subtitle: "Quarterly plan.pdf", Tail: hotkey, Glyph: "◉", GlyphColor: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Selected: true},
		{Title: "Open containing folder", Subtitle: "Open containing folder", Tail: "Enter", Glyph: "◆", GlyphColor: step.Accent},
		{Title: "Copy path", Subtitle: `C:\Users\qianl\Desktop\Quarterly plan.pdf`, Tail: "Copy", Glyph: "□", GlyphColor: woxui.Color{R: 56, G: 189, B: 248, A: 255}},
		{Title: "Translate text", Subtitle: "Input tr hello, tr openai hello", Glyph: "文", GlyphColor: woxui.Color{R: 34, G: 211, B: 238, A: 255}},
	}
	window := onboardingDemoWindow(onboardingDemoWindowProps{
		Width: leftWidth, Height: height, Backdrop: props.WallpaperBlurred, Query: "", Results: results, Accent: step.Accent, Theme: props.Theme, Opacity: opacity, ShowQuery: true, ShowToolbar: true,
	})
	previewWidth := width - leftWidth - 26
	previewHeight := height - queryHeight - footerHeight - 18
	preview := woxwidget.Container{
		Width: previewWidth, Height: previewHeight, Radius: 8, BorderColor: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 76)), BorderWidth: 1,
		Padding: woxwidget.UniformInsets(14), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
			woxwidget.Constrained{FillWidth: true, Child: woxwidget.Container{Height: 6, Radius: 3, Color: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 24))}},
			woxwidget.Container{Width: previewWidth - 96, Height: 6, Radius: 3, Color: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 24))},
			woxwidget.Constrained{FillWidth: true, Child: woxwidget.Container{Height: max(float32(40), previewHeight-62), Radius: 6, Color: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 18)), Child: woxwidget.Align{
				Height: max(float32(40), previewHeight-62), Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: "▧", Style: woxui.TextStyle{Size: 28}, Color: settingsColorAlpha(props.Theme.ResultSubtitle, demoScaledAlpha(opacity, 120))},
			}}},
		}},
	}
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: width, Height: height, Radius: 8, Color: settingsColorAlpha(props.Theme.Background, demoScaledAlpha(opacity, 246)), BorderColor: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 28)), BorderWidth: 1}},
		{Child: window},
		{Left: leftWidth + 14, Top: queryHeight + 8, Child: preview},
	}}
}

func onboardingPermissionsDemo(props OnboardingProps, step OnboardingStep, width, height float32) woxwidget.Widget {
	results := make([]onboardingDemoResult, 0, len(props.Permissions))
	for index, permission := range props.Permissions {
		tail := props.Labels["permission.authorize"]
		color := woxui.Color{R: 245, G: 158, B: 11, A: 255}
		if permission.Ready {
			tail = props.Labels["demo.permission.ready"]
			color = woxui.Color{R: 34, G: 197, B: 94, A: 255}
		}
		results = append(results, onboardingDemoResult{Title: permission.Title, Subtitle: permission.Description, Tail: tail, Glyph: permissionGlyph(permission.ID), GlyphColor: color, Selected: index == 0})
	}
	windowWidth := min(float32(620), width-96)
	windowHeight := min(float32(220), height-88)
	window := onboardingDemoWindow(onboardingDemoWindowProps{Width: windowWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, Query: "permissions", Results: results, Accent: step.Accent, Theme: props.Theme, Opacity: 1, ShowQuery: true, ShowToolbar: true})
	return onboardingDemoDesktop(props, step, width, height, false, []woxwidget.StackChild{{Left: (width - windowWidth) / 2, Top: (height - windowHeight) / 2, Child: window}})
}

func onboardingGlanceDemo(props OnboardingProps, step OnboardingStep, width, height float32) woxwidget.Widget {
	title := props.Labels["glance.enable"]
	tail := ""
	var accessory woxwidget.Widget
	accessoryWidth := float32(0)
	if props.GlanceEnabled {
		title = props.GlanceLabel
		tail = props.Labels["demo.glance.value"]
		accessoryText := tail
		if accessoryText == "" {
			accessoryText = props.GlanceLabel
		}
		accessoryWidth = min(float32(200), max(float32(100), float32(len([]rune(accessoryText)))*12+42))
		accessoryColor := settingsColorAlpha(props.Theme.QueryText, 204)
		textWidth := min(accessoryWidth-37, float32(len([]rune(accessoryText)))*12)
		accessory = woxwidget.Align{
			Width: accessoryWidth, Height: 30, Horizontal: .5, Vertical: .5,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				onboardingDemoClockIcon(accessoryColor),
				woxwidget.TextBlock{Value: accessoryText, Width: textWidth, Height: 18, MaxLines: 1, LineHeight: 18, Style: woxui.TextStyle{Size: 11}, Color: accessoryColor},
			}},
		}
	}
	windowWidth := width - 100
	windowHeight := height - 88
	window := onboardingDemoWindow(onboardingDemoWindowProps{
		Width: windowWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, Query: "wox", QueryAccessory: accessory,
		Accent: step.Accent, Theme: props.Theme, Opacity: 1, ShowQuery: true, ShowToolbar: true,
		Results: []onboardingDemoResult{
			{Title: title, Subtitle: props.Labels["glance.body"], Tail: tail, Glyph: "◉", GlyphColor: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Selected: true},
			{Title: props.Labels["demo.glance.provider"], Subtitle: props.Labels["glance.enable.body"], Tail: "Glance", Glyph: "ϟ", GlyphColor: step.Accent},
			{Title: props.Labels["glance.primary"], Subtitle: props.Labels["glance.primary"], Tail: props.GlanceLabel, Glyph: "⌖", GlyphColor: woxui.Color{R: 96, G: 165, B: 250, A: 255}},
		},
	})
	return onboardingDemoDesktop(props, step, width, height, false, []woxwidget.StackChild{{Left: 48, Top: 44, Child: window}})
}

// onboardingQueryHotkeysDemo crossfades the normal launcher example into the narrow borderless webview example.
func onboardingQueryHotkeysDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	example1Opacity := float32(1)
	if progress >= .43 {
		example1Opacity = 1 - demoEaseInCubic(demoInterval(progress, .43, .52))
	}
	example2Opacity := float32(0)
	if progress >= .50 && progress < .60 {
		example2Opacity = demoEaseOutCubic(demoInterval(progress, .50, .60))
	} else if progress >= .60 && progress < .94 {
		example2Opacity = 1
	} else if progress >= .94 {
		example2Opacity = 1 - demoEaseInCubic(demoInterval(progress, .94, 1))
	}
	contentLeft := float32(48)
	contentWidth := width - 100
	contentTop := demoHintTop()
	contentHeight := height - contentTop - 36
	hotkey1 := demoQueryHotkey("G")
	hotkey2 := demoQueryHotkey("I")
	children := []woxwidget.StackChild{}
	if example1Opacity > .01 {
		children = append(children, woxwidget.StackChild{Left: contentLeft, Top: contentTop, Child: onboardingDemoHintCard(props, step, step.Title, strings.Join(hotkey1, "+"), "github repo", contentWidth, demoAlpha(example1Opacity))})
		shortcutProgress := demoEnterHoldExit(progress, .09, .15, .25, .31)
		if shortcutProgress > .01 {
			children = append(children, woxwidget.StackChild{Left: (width - demoHotkeyWidth(hotkey1)) / 2, Top: contentTop + 86, Child: onboardingDemoHotkey(hotkey1, step.Accent, props.Theme, progress >= .15 && progress <= .21, shortcutProgress*example1Opacity)})
		}
		windowProgress := demoEaseOutCubic(demoInterval(progress, .20, .29))
		if windowProgress > .01 {
			windowHeight := max(float32(180), contentHeight-76)
			children = append(children, woxwidget.StackChild{Left: contentLeft, Top: contentTop + 70 + 20*(1-windowProgress), Child: onboardingDemoWindow(onboardingDemoWindowProps{
				Width: contentWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, Query: "github repo", Accent: step.Accent, Theme: props.Theme, Opacity: windowProgress * example1Opacity, ShowQuery: true, ShowToolbar: true,
				Results: []onboardingDemoResult{
					{Title: "Wox repository", Subtitle: "Open Wox-launcher/Wox on GitHub", Tail: strings.Join(hotkey1, "+"), Glyph: "</>", GlyphColor: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Selected: true},
					{Title: step.Title, Subtitle: props.Labels[step.ID+".body"], Tail: "Query Hotkeys", Glyph: "ϟ", GlyphColor: step.Accent},
					{Title: "Issues", Subtitle: "github repo issues", Tail: "GitHub", Glyph: "!", GlyphColor: woxui.Color{R: 250, G: 204, B: 21, A: 255}},
				},
			})})
		}
	}
	if example2Opacity > .01 {
		children = append(children, woxwidget.StackChild{Left: contentLeft, Top: contentTop, Child: onboardingDemoHintCard(props, step, step.Title, strings.Join(hotkey2, "+"), "webview instagram", contentWidth, demoAlpha(example2Opacity))})
		shortcutProgress := demoEnterHoldExit(progress, .55, .63, .72, .79)
		if shortcutProgress > .01 {
			children = append(children, woxwidget.StackChild{Left: (width - demoHotkeyWidth(hotkey2)) / 2, Top: contentTop + 86, Child: onboardingDemoHotkey(hotkey2, step.Accent, props.Theme, progress >= .63 && progress <= .70, shortcutProgress*example2Opacity)})
		}
		windowProgress := demoEaseOutCubic(demoInterval(progress, .68, .80))
		if windowProgress > .01 {
			instagramWidth := min(float32(340), width-160)
			instagramHeight := max(float32(190), contentHeight-72)
			children = append(children, woxwidget.StackChild{Left: (width - instagramWidth) / 2, Top: contentTop + 70 + 20*(1-windowProgress), Child: onboardingInstagramWindow(props, instagramWidth, instagramHeight, windowProgress*example2Opacity)})
		}
	}
	return onboardingDemoDesktop(props, step, width, height, false, children)
}

func onboardingQueryHotkeySilentDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	sceneOpacity := float32(1)
	if progress < .04 {
		sceneOpacity = demoEaseOutCubic(demoInterval(progress, 0, .04))
	} else if progress > .94 {
		sceneOpacity = 1 - demoEaseInCubic(demoInterval(progress, .94, 1))
	}
	contentLeft := float32(48)
	contentWidth := width - 100
	contentTop := demoHintTop()
	hotkey := demoQueryHotkey("S")
	children := []woxwidget.StackChild{{Left: contentLeft, Top: contentTop, Child: onboardingDemoHintCard(props, step, step.Title, strings.Join(hotkey, "+"), "copy github repo", contentWidth, demoAlpha(sceneOpacity))}}
	shortcutProgress := demoEnterHoldExit(progress, .10, .16, .26, .32)
	if shortcutProgress > .01 {
		children = append(children, woxwidget.StackChild{Left: (width - demoHotkeyWidth(hotkey)) / 2, Top: contentTop + 86, Child: onboardingDemoHotkey(hotkey, step.Accent, props.Theme, progress >= .16 && progress <= .22, shortcutProgress*sceneOpacity)})
	}
	toastProgress := demoEaseOutCubic(demoInterval(progress, .22, .34)) * sceneOpacity
	if toastProgress > .01 {
		toastWidth := min(float32(312), width-120)
		toast := woxwidget.Container{Width: toastWidth, Height: 60, Radius: 12, Color: settingsColorAlpha(props.Theme.Background, demoScaledAlpha(toastProgress, 245)), BorderColor: settingsColorAlpha(step.Accent, demoScaledAlpha(toastProgress, 71)), BorderWidth: 1, Padding: woxwidget.Insets{Left: 14, Top: 13, Right: 14, Bottom: 13}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "✓", Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(step.Accent, demoAlpha(toastProgress))},
			woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
				woxwidget.Text{Value: step.Title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(props.Theme.ResultTitle, demoAlpha(toastProgress))},
				woxwidget.Text{Value: "copy github repo", Style: woxui.TextStyle{Size: 10.5}, Color: settingsColorAlpha(props.Theme.ResultSubtitle, demoAlpha(toastProgress))},
			}},
		}}}
		children = append(children, woxwidget.StackChild{Left: (width - toastWidth) / 2, Top: height - 96 + 18*(1-toastProgress), Child: toast})
	}
	return onboardingDemoDesktop(props, step, width, height, false, children)
}

func onboardingInstagramWindow(props OnboardingProps, width, height, opacity float32) woxwidget.Widget {
	alpha := demoAlpha(opacity)
	white := woxui.Color{R: 255, G: 255, B: 255, A: alpha}
	black := woxui.Color{A: alpha}
	imageHeight := max(float32(70), height-142)
	return woxwidget.Container{
		Width: width, Height: height, Radius: 8, Color: white, BorderColor: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 28)), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 12, Top: 9, Right: 12, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
			woxwidget.Stack{Width: width - 24, Height: 24, Children: []woxwidget.StackChild{
				{Child: woxwidget.Text{Value: "Instagram⌄", Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: black}},
				{Left: width - 92, Child: woxwidget.Text{Value: "▣    ♡", Style: woxui.TextStyle{Size: 15}, Color: black}},
			}},
			woxwidget.Container{Width: width - 24, Height: imageHeight, Color: woxui.Color{R: 135, G: 206, B: 235, A: alpha}, Child: woxwidget.Stack{Width: width - 24, Height: imageHeight, Children: []woxwidget.StackChild{
				{Left: 18, Top: 12, Child: onboardingColorFlag(woxui.Color{R: 233, G: 107, B: 107, A: alpha})},
				{Left: 42, Top: 6, Child: onboardingColorFlag(woxui.Color{R: 91, G: 200, B: 232, A: alpha})},
				{Left: 68, Top: 17, Child: onboardingColorFlag(woxui.Color{R: 245, G: 200, B: 66, A: alpha})},
				{Left: width - 82, Top: 10, Child: onboardingColorFlag(woxui.Color{R: 130, G: 212, B: 138, A: alpha})},
				{Left: (width - 64) / 2, Top: imageHeight - 52, Child: woxwidget.Container{Width: 40, Height: 40, Radius: 20, Color: woxui.Color{R: 204, G: 204, B: 204, A: alpha}, Child: woxwidget.Align{
					Width: 40, Height: 40, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: "●", Style: woxui.TextStyle{Size: 20}, Color: woxui.Color{R: 136, G: 136, B: 136, A: alpha}},
				}}},
			}}},
			woxwidget.Text{Value: "♡    ◯    ➤                                      ♧", Style: woxui.TextStyle{Size: 13}, Color: black},
			woxwidget.Text{Value: "eu_imozou和其他用户赞了", Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: black},
			woxwidget.Text{Value: "camel8326  こどもの日…   更多", Style: woxui.TextStyle{Size: 9}, Color: black},
			woxwidget.Text{Value: "⌂        ⌕        ▷        ➤        ●", Style: woxui.TextStyle{Size: 13}, Color: black},
		}},
	}
}

func onboardingColorFlag(color woxui.Color) woxwidget.Widget {
	return woxwidget.Container{Width: 8, Height: 28, Radius: 2, Color: color}
}

// onboardingQueryShortcutsDemo shows that the visible alias stays unchanged while its provider query expands.
func onboardingQueryShortcutsDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	query := demoTypedQuery("gh repo", progress, .18, onboardingDemoDuration(step.ID))
	expanded := progress >= .68 && progress < .94
	contentLeft := float32(48)
	contentTop := demoHintTop()
	contentWidth := width - 100
	windowTop := contentTop + 70
	windowHeight := max(float32(180), height-windowTop-36)
	subtitle := props.Labels["queryShortcuts.body"]
	tail := props.Labels["queryShortcuts.title"]
	if expanded {
		subtitle = "github repo"
		tail = "gh"
	}
	children := []woxwidget.StackChild{
		{Left: contentLeft, Top: contentTop, Child: onboardingDemoHintCard(props, step, step.Title, "gh repo", "github repo", contentWidth, 255)},
		{Left: contentLeft, Top: windowTop, Child: onboardingDemoWindow(onboardingDemoWindowProps{
			Width: contentWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, Query: query,
			Accent: step.Accent, Theme: props.Theme, Opacity: 1, ShowQuery: true, ShowToolbar: true,
			Results: []onboardingDemoResult{
				{Title: "Open repository", Subtitle: subtitle, Tail: tail, Glyph: "≡", GlyphColor: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Selected: true},
				{Title: "Repository search", Subtitle: "github repo", Tail: "Enter", Glyph: "↗", GlyphColor: step.Accent},
				{Title: "Search issues", Subtitle: "github issues", Glyph: "⌕", GlyphColor: woxui.Color{R: 96, G: 165, B: 250, A: 255}},
			},
		})},
	}
	return onboardingDemoDesktop(props, step, width, height, false, children)
}

func onboardingTrayQueriesDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	cursorProgress := demoEaseInOutCubic(demoInterval(progress, .10, .38))
	windowProgress := demoEnterHoldExit(progress, .48, .66, .92, 1)
	trayX := width - 88
	trayY := float32(14)
	if runtime.GOOS != "darwin" {
		trayX = width - 120
		trayY = height - 23
	}
	hintTop := demoHintTop()
	if runtime.GOOS == "darwin" {
		hintTop = height - 94
	}
	children := []woxwidget.StackChild{
		{Left: 48, Top: hintTop, Child: onboardingDemoHintCard(props, step, step.Title, "tray icon", "weather", width-100, 255)},
		{Left: trayX - 10, Top: trayY - 10, Child: woxwidget.Container{Width: 20, Height: 20, Radius: 5, Color: settingsColorAlpha(step.Accent, boolAlpha(progress >= .38 && progress <= .50, 180, 48)), BorderColor: step.Accent, BorderWidth: 1,
			Child: woxwidget.Align{Width: 20, Height: 20, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: "W", Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}}}},
		{Left: demoLerp(60, trayX-6, cursorProgress), Top: demoLerp(height-82, trayY-7, cursorProgress), Child: onboardingDemoCursor(1)},
	}
	if windowProgress > .01 {
		windowWidth := min(float32(420), width-96)
		windowHeight := min(float32(190), height-160)
		windowLeft := max(float32(48), trayX-windowWidth+10)
		windowTop := trayY + 14
		if runtime.GOOS != "darwin" {
			windowTop = trayY - windowHeight - 14
		}
		children = append(children, woxwidget.StackChild{Left: windowLeft, Top: windowTop + 18*(1-windowProgress), Child: onboardingDemoWindow(onboardingDemoWindowProps{
			Width: windowWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, Accent: step.Accent, Theme: props.Theme, Opacity: windowProgress, ShowQuery: false, ShowToolbar: false,
			Results: []onboardingDemoResult{
				{Title: "Weather", Subtitle: "Sunny, 24 C", Tail: "Tray Queries", Glyph: "☀", GlyphColor: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Selected: true},
				{Title: step.Title, Subtitle: props.Labels[step.ID+".body"], Tail: "Tray", Glyph: "◎", GlyphColor: step.Accent},
				{Title: "Calendar", Subtitle: "Next meeting in 25 minutes", Glyph: "▣", GlyphColor: woxui.Color{R: 96, G: 165, B: 250, A: 255}},
			},
		})})
	}
	return onboardingDemoDesktop(props, step, width, height, false, children)
}

// onboardingPluginStoreDemo starts with the same 4x4 icon convergence before revealing the split plugin store.
func onboardingPluginStoreDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	contentLeft := float32(48)
	contentTop := demoHintTop()
	contentWidth := width - 100
	children := []woxwidget.StackChild{{Left: contentLeft, Top: contentTop, Child: onboardingDemoHintCard(props, step, step.Title, "wpm install", "Plugin Store", contentWidth, 255)}}
	gridOpacity := float32(1)
	if progress < .08 {
		gridOpacity = demoEaseOutCubic(demoInterval(progress, 0, .08))
	} else if progress >= .42 {
		gridOpacity = 1 - demoEaseInCubic(demoInterval(progress, .42, .55))
	}
	if gridOpacity > .01 {
		gridHeight := height - contentTop - 106
		children = append(children, onboardingPluginIconGrid(props, step, contentLeft, contentTop+70, contentWidth, gridHeight, demoEaseInOutCubic(demoInterval(progress, .08, .40)), gridOpacity)...)
	}
	windowProgress := demoEaseOutCubic(demoInterval(progress, .34, .50))
	if windowProgress > .01 {
		windowHeight := height - contentTop - 82
		tail := props.Labels["demo.install"]
		if progress >= .82 && progress < .90 {
			tail = props.Labels["demo.installing"]
		} else if progress >= .90 && progress < .97 {
			tail = props.Labels["demo.installed"]
		}
		children = append(children, woxwidget.StackChild{Left: contentLeft, Top: contentTop + 70 + 18*(1-windowProgress), Child: onboardingPluginStoreWindow(
			props, step, contentWidth, windowHeight, demoTypedQuery("wpm install", progress, .50, onboardingDemoDuration(step.ID)), tail, windowProgress,
		)})
	}
	return onboardingDemoDesktop(props, step, width, height, false, children)
}

func onboardingPluginIconGrid(props OnboardingProps, step OnboardingStep, left, top, width, height, progress, opacity float32) []woxwidget.StackChild {
	glyphs := []string{"↗", "▧", "RSS", "♫", "△", "Σ", "文", "✣", "➤", "▤", "◉", "◆", "☕", "⌕", "▣", "译"}
	colors := []woxui.Color{
		{R: 79, G: 110, B: 247, A: 255}, {R: 59, G: 130, B: 246, A: 255}, {R: 249, G: 115, B: 22, A: 255}, {R: 34, G: 197, B: 94, A: 255},
		{R: 252, G: 76, B: 2, A: 255}, {R: 250, G: 204, B: 21, A: 255}, {R: 14, G: 165, B: 233, A: 255}, {R: 168, G: 85, B: 247, A: 255},
		{R: 6, G: 182, B: 212, A: 255}, {R: 245, G: 158, B: 11, A: 255}, {R: 244, G: 63, B: 94, A: 255}, {R: 59, G: 130, B: 246, A: 255},
		{R: 234, G: 179, B: 8, A: 255}, {R: 34, G: 197, B: 94, A: 255}, {R: 100, G: 116, B: 139, A: 255}, {R: 6, G: 182, B: 212, A: 255},
	}
	targetX := left + width/2
	targetY := top + height/2
	children := make([]woxwidget.StackChild, 0, 17)
	for index, glyph := range glyphs {
		row := index / 4
		column := index % 4
		startX := left + width*.20 + float32(column)*width*.20
		startY := top + height*.16 + float32(row)*height*.22
		local := demoInterval(progress, float32(row)*.025+float32(column)*.012, 1)
		x := demoLerp(startX, targetX, local)
		y := demoLerp(startY, targetY, local)
		size := demoLerp(boolFloatValue(index == 1, 58, 50), boolFloatValue(index == 1, 29, 14), local)
		children = append(children, woxwidget.StackChild{Left: x - size/2, Top: y - size/2, Child: woxwidget.Container{
			Width: size, Height: size, Radius: min(float32(12), size*.24), Color: settingsColorAlpha(props.Theme.Background, demoScaledAlpha(opacity*(1-.62*local), 236)),
			BorderColor: settingsColorAlpha(colors[index], demoScaledAlpha(opacity, 130)), BorderWidth: 1,
			Child: woxwidget.Align{Width: size, Height: size, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: glyph, Style: woxui.TextStyle{Size: max(float32(7), size*.25), Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(colors[index], demoAlpha(opacity))}},
		}})
	}
	if progress > .35 && progress < .95 {
		centerOpacity := min(demoInterval(progress, .35, .62), 1-demoInterval(progress, .62, 1))
		children = append(children, woxwidget.StackChild{Left: targetX - 24, Top: targetY - 24, Child: woxwidget.Container{
			Width: 48, Height: 48, Radius: 12, Color: settingsColorAlpha(step.Accent, demoScaledAlpha(centerOpacity*opacity, 42)), BorderColor: settingsColorAlpha(step.Accent, demoScaledAlpha(centerOpacity*opacity, 108)), BorderWidth: 1,
			Child: woxwidget.Align{Width: 48, Height: 48, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: "W", Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(props.Theme.ResultTitle, demoAlpha(centerOpacity*opacity))}},
		}})
	}
	return children
}

func onboardingPluginStoreWindow(props OnboardingProps, step OnboardingStep, width, height float32, query, installLabel string, opacity float32) woxwidget.Widget {
	alpha := demoAlpha(opacity)
	listWidth := min(float32(300), width*.38)
	items := []onboardingDemoResult{
		{Title: "Quick Links", Subtitle: "Quickly open named URLs", Glyph: "↗", GlyphColor: woxui.Color{R: 79, G: 110, B: 247, A: 255}},
		{Title: "RImage", Subtitle: "使用 rimage 压缩选中的图片", Glyph: "▧", GlyphColor: woxui.Color{R: 59, G: 130, B: 246, A: 255}, Selected: true},
		{Title: "RSS Reader", Subtitle: "Read RSS feeds", Tail: "✓", Glyph: "RSS", GlyphColor: woxui.Color{R: 249, G: 115, B: 22, A: 255}},
		{Title: "Recent Files", Subtitle: "List recently used files", Tail: "✓", Glyph: "◷", GlyphColor: woxui.Color{R: 148, G: 163, B: 184, A: 255}},
	}
	renderHeight := min(height, float32(72+len(items)*51+36))
	detailWidth := width - listWidth - 14
	detailHeight := max(float32(40), renderHeight-116)
	detail := woxwidget.Container{
		Width: detailWidth, Height: detailHeight, Radius: 8, Color: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 14)), BorderColor: settingsColorAlpha(props.Theme.ResultTitle, demoScaledAlpha(opacity, 26)), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 16, Top: 14, Right: 16, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "▧   RImage", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(props.Theme.ResultTitle, alpha)},
			woxwidget.TextBlock{Value: "使用 rimage 压缩选中的图片 · qianlifeng", Height: 18, MaxLines: 1, LineHeight: 18, Style: woxui.TextStyle{Size: 9}, Color: settingsColorAlpha(props.Theme.ResultSubtitle, alpha)},
			woxwidget.Text{Value: "v0.0.1     NodeJS     GitHub ↗", Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(props.Theme.ResultSubtitle, alpha)},
			woxwidget.Constrained{FillWidth: true, Child: woxwidget.Container{Height: max(float32(40), detailHeight-86), Radius: 7, Color: settingsColorAlpha(woxui.Color{A: 255}, demoScaledAlpha(opacity, 58)), Child: woxwidget.Align{
				Height: max(float32(40), detailHeight-86), Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: "RImage", Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: settingsColorAlpha(step.Accent, alpha)},
			}}},
		}},
	}
	return onboardingDemoWindow(onboardingDemoWindowProps{
		Width: width, Height: height, Backdrop: props.WallpaperBlurred, Query: query, Results: items, Accent: step.Accent, Theme: props.Theme, Opacity: opacity,
		ShowQuery: true, ShowToolbar: true, ResultWidth: listWidth, Preview: detail, PrimaryAction: installLabel,
	})
}

func onboardingThemeInstallDemo(props OnboardingProps, step OnboardingStep, width, height, progress float32) woxwidget.Widget {
	contentLeft := float32(48)
	contentTop := demoHintTop()
	contentWidth := width - 100
	applied := progress >= .64 && progress < .95
	theme := props.Theme
	background := woxui.Color{}
	if applied {
		background = woxui.Color{R: 15, G: 23, B: 42, A: 255}
		theme.Background = background
		theme.QueryBackground = woxui.Color{R: 30, G: 41, B: 59, A: 255}
		theme.QueryText = woxui.Color{R: 224, G: 242, B: 254, A: 255}
		theme.ResultTitle = woxui.Color{R: 203, G: 213, B: 225, A: 255}
		theme.ResultSubtitle = woxui.Color{R: 100, G: 116, B: 139, A: 255}
		theme.SelectedBackground = woxui.Color{R: 30, G: 58, B: 95, A: 255}
	}
	tail := props.Labels["demo.install"]
	if progress >= .53 && progress < .63 {
		tail = props.Labels["demo.installing"]
	} else if progress >= .63 && progress < .95 {
		tail = props.Labels["demo.apply"]
	}
	windowHeight := height - contentTop - 82
	children := []woxwidget.StackChild{
		{Left: contentLeft, Top: contentTop, Child: onboardingDemoHintCard(props, step, step.Title, "theme", "theme ocean dark", contentWidth, 255)},
		{Left: contentLeft, Top: contentTop + 70, Child: onboardingDemoWindow(onboardingDemoWindowProps{
			Width: contentWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, Query: demoTypedQuery("theme ocean dark", progress, .08, onboardingDemoDuration(step.ID)), Accent: boolColor(applied, woxui.Color{R: 56, G: 189, B: 248, A: 255}, step.Accent), Theme: theme, Background: background,
			Opacity: 1, ShowQuery: true, ShowToolbar: true,
			Results: []onboardingDemoResult{
				{Title: "Ocean Dark", Subtitle: "A calm blue dark theme", Tail: tail, Glyph: "◈", GlyphColor: woxui.Color{R: 56, G: 189, B: 248, A: 255}, Selected: true},
				{Title: "Aurora", Subtitle: "Theme Store", Tail: props.Labels["demo.install"], Glyph: "◈", GlyphColor: woxui.Color{R: 232, G: 121, B: 249, A: 255}},
				{Title: "Default Dark", Subtitle: "Current theme", Tail: "System", Glyph: "◈", GlyphColor: woxui.Color{R: 96, G: 165, B: 250, A: 255}},
			},
		})},
	}
	return onboardingDemoDesktop(props, step, width, height, false, children)
}

func onboardingFinishDemo(props OnboardingProps, step OnboardingStep, width, height float32) woxwidget.Widget {
	windowWidth := width - 100
	windowHeight := height - 88
	window := onboardingDemoWindow(onboardingDemoWindowProps{
		Width: windowWidth, Height: windowHeight, Backdrop: props.WallpaperBlurred, Query: "ready", Accent: step.Accent, Theme: props.Theme, Opacity: 1, ShowQuery: true, ShowToolbar: true,
		Results: []onboardingDemoResult{
			{Title: props.Labels["demo.finish.title"], Subtitle: props.Labels["finish.body"], Tail: props.Labels["demo.finish.badge"], Glyph: "✓", GlyphColor: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Selected: true},
			{Title: "Open Wox Settings", Subtitle: `C:\Users\qianl\AppData\Roaming\Wox`, Glyph: "W", GlyphColor: step.Accent},
			{Title: props.Labels["demo.actions"], Subtitle: props.Labels["welcome.body"], Tail: demoActionHotkey(), Glyph: "▶", GlyphColor: step.Accent},
			{Title: props.Labels["queryHotkeys.body"], Subtitle: props.Labels["trayQueries.body"], Tail: "Tray Queries", Glyph: "⌕", GlyphColor: woxui.Color{R: 167, G: 139, B: 250, A: 255}},
		},
	})
	return onboardingDemoDesktop(props, step, width, height, false, []woxwidget.StackChild{{Left: 48, Top: 44, Child: window}})
}

// demoTypedQuery reveals every animated query at one shared fast per-character cadence.
func demoTypedQuery(target string, progress, start float32, duration time.Duration) string {
	runes := []rune(target)
	elapsed := max(float32(0), progress-start) * float32(duration)
	count := int(elapsed / float32(demoQueryTypingInterval))
	return string(runes[:min(count, len(runes))])
}

func demoEnterHoldExit(progress, enterStart, enterEnd, exitStart, exitEnd float32) float32 {
	if progress < enterStart {
		return 0
	}
	if progress < enterEnd {
		return demoEaseOutCubic(demoInterval(progress, enterStart, enterEnd))
	}
	if progress < exitStart {
		return 1
	}
	return 1 - demoEaseInCubic(demoInterval(progress, exitStart, exitEnd))
}

func demoInterval(progress, start, end float32) float32 {
	return min(max(float32(0), (progress-start)/(end-start)), float32(1))
}

func demoEaseInCubic(value float32) float32 {
	return value * value * value
}

func demoEaseOutCubic(value float32) float32 {
	inverse := 1 - value
	return 1 - inverse*inverse*inverse
}

func demoEaseInOutCubic(value float32) float32 {
	if value < .5 {
		return 4 * value * value * value
	}
	inverse := -2*value + 2
	return 1 - inverse*inverse*inverse/2
}

func demoAlpha(progress float32) uint8 {
	return demoScaledAlpha(progress, 255)
}

func demoScaledAlpha(progress float32, alpha uint8) uint8 {
	return uint8(min(max(float32(0), progress), float32(1))*float32(alpha) + .5)
}

func demoColorOpacity(color woxui.Color, opacity float32) woxui.Color {
	color.A = demoScaledAlpha(opacity, color.A)
	return color
}

func onboardingDemoMicaColor(color woxui.Color) woxui.Color {
	if color.A >= 245 {
		color.A = 255
		return color
	}
	tint := woxui.Color{R: 32, G: 32, B: 32, A: 255}
	if !themeColorIsDarkForDemo(color) {
		tint = woxui.Color{R: 242, G: 242, B: 242, A: 255}
	}
	const mix = float32(.18)
	color.R = uint8(float32(color.R)*(1-mix) + float32(tint.R)*mix + .5)
	color.G = uint8(float32(color.G)*(1-mix) + float32(tint.G)*mix + .5)
	color.B = uint8(float32(color.B)*(1-mix) + float32(tint.B)*mix + .5)
	alpha := min(max(float32(.64)+float32(color.A)/255*.18, float32(.64)), float32(.86))
	color.A = uint8(alpha*255 + .5)
	return color
}

func themeColorIsDarkForDemo(color woxui.Color) bool {
	return .2126*float32(color.R)+.7152*float32(color.G)+.0722*float32(color.B) < 127.5
}

func demoLerp(from, to, progress float32) float32 {
	return from + (to-from)*min(max(float32(0), progress), float32(1))
}

func demoDefaultHotkey(selection bool) []string {
	if runtime.GOOS == "darwin" {
		if selection {
			return []string{"Cmd", "Option", "Space"}
		}
		return []string{"Option", "Space"}
	}
	if selection {
		return []string{"Ctrl", "Alt", "Space"}
	}
	return []string{"Alt", "Space"}
}

func demoQueryHotkey(key string) []string {
	if runtime.GOOS == "darwin" {
		return []string{"Cmd", "Shift", key}
	}
	return []string{"Ctrl", "Shift", key}
}

func demoActionHotkey() string {
	if runtime.GOOS == "darwin" {
		return "⌘ J"
	}
	return "Ctrl J"
}

func demoHintTop() float32 {
	if runtime.GOOS == "darwin" {
		return 42
	}
	return 18
}

func demoHotkeyWidth(labels []string) float32 {
	width := float32(32)
	for index, label := range labels {
		if index > 0 {
			width += 28
		}
		width += max(float32(48), float32(len([]rune(label)))*7+22)
	}
	return width
}

func boolFloat(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

func boolFloatValue(value bool, trueValue, falseValue float32) float32 {
	if value {
		return trueValue
	}
	return falseValue
}

func boolAlpha(value bool, trueValue, falseValue uint8) uint8 {
	if value {
		return trueValue
	}
	return falseValue
}

func boolColor(value bool, trueValue, falseValue woxui.Color) woxui.Color {
	if value {
		return trueValue
	}
	return falseValue
}
