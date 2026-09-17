package component

import (
	"runtime"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherDemoQueryPart describes one styled segment in the simulated query.
type LauncherDemoQueryPart struct {
	Text       string
	Color      woxui.Color
	Background woxui.Color
	Selected   bool // A transparent selection still needs a discoverable locator target.
	Caret      bool
}

// LauncherDemoResult describes one simulated launcher result.
type LauncherDemoResult struct {
	Title      string
	Subtitle   string
	Tail       string
	Glyph      string
	GlyphColor woxui.Color
	Selected   bool
	Hovered    bool
}

// LauncherDemoHighlightTarget identifies one semantic surface inside the simulated launcher.
type LauncherDemoHighlightTarget uint8

const (
	LauncherDemoHighlightNone LauncherDemoHighlightTarget = iota
	LauncherDemoHighlightScrollbar
	LauncherDemoHighlightIndicator
	LauncherDemoHighlightSurface
	LauncherDemoHighlightContent
	LauncherDemoHighlightQueryBackground
	LauncherDemoHighlightQueryText
	LauncherDemoHighlightQueryCaret
	LauncherDemoHighlightQuerySelection
	LauncherDemoHighlightResultTitle
	LauncherDemoHighlightResultSubtitle
	LauncherDemoHighlightResultTail
	LauncherDemoHighlightSelectedBackground
	LauncherDemoHighlightSelectedTitle
	LauncherDemoHighlightSelectedTail
	LauncherDemoHighlightToolbarBackground
	LauncherDemoHighlightToolbarText
	LauncherDemoHighlightToolbarPrimaryText
	LauncherDemoHighlightToolbarPrimaryHotkey
	LauncherDemoHighlightActionBackground
	LauncherDemoHighlightActionHeader
	LauncherDemoHighlightActionText
	LauncherDemoHighlightActionSelectedBackground
	LauncherDemoHighlightActionSelectedText
	LauncherDemoHighlightActionQueryBackground
	LauncherDemoHighlightResultHover
	LauncherDemoHighlightActionDivider
	LauncherDemoHighlightHotkey
	LauncherDemoHighlightActionHotkey
	LauncherDemoHighlightActionActiveHotkey
)

// LauncherDemoProps contains the complete simulated launcher state shared by previews.
type LauncherDemoProps struct {
	// Geometry supplies resolved editor spacing; nil retains compact catalog/onboarding layout.
	Geometry               *LauncherDemoGeometry
	Width, Height          float32
	Backdrop               *woxui.Image
	Query                  string
	QueryParts             []LauncherDemoQueryPart
	Results                []LauncherDemoResult
	Accent                 woxui.Color
	Theme                  Theme
	Opacity                float32
	ShowQuery, ShowToolbar bool
	ToolbarPressed         bool
	ActionProgress         float32
	ActionCopy, ActionMore string
	FadeResults            bool
	ResultsOpacity         float32
	Background             woxui.Color
	ResultWidth            float32
	Preview                woxwidget.Widget
	PrimaryAction          string
	Window                 *woxui.Window
	QueryAccessory         woxwidget.Widget
	HighlightCorners       bool
	HighlightColor         woxui.Color
	HighlightTarget        LauncherDemoHighlightTarget
	QueryFontSize          float32
	ResultTitleFontSize    float32
	ResultSubtitleFontSize float32
	QueryHeight            float32
	RowHeight              float32
	RowGap                 float32
	ToolbarHeight          float32
}

// LauncherDemoGeometry carries logical theme spacing without changing demo typography or density.
type LauncherDemoGeometry struct {
	AppPadding        woxwidget.Insets
	ResultPadding     woxwidget.Insets
	ItemPadding       woxwidget.Insets
	ActionPadding     woxwidget.Insets
	ToolbarPadding    woxwidget.Insets
	ActionQueryRadius float32
}

// WoxLauncherDemo builds the complete query, results, preview, action panel, and toolbar demo.
func WoxLauncherDemo(props LauncherDemoProps) woxwidget.Widget {
	opacity := min(max(float32(0), props.Opacity), float32(1))
	alpha := demoAlpha(opacity)
	background := props.Background
	if background.A == 0 {
		background = props.Theme.Background
	}
	windowWidth := props.Width
	contentBounds := LauncherContentBounds(props.Width, props.Height, props.Theme.AppContentInset)
	props.Width, props.Height = contentBounds.Width, contentBounds.Height
	appPadding, queryHeight := demoAppPadding(props.Theme), demoQueryHeight(props)
	appInsets := woxwidget.UniformInsets(appPadding)
	resultInsets := woxwidget.Insets{Top: 8}
	if props.Geometry != nil {
		appInsets = props.Geometry.AppPadding
		resultInsets = props.Geometry.ResultPadding
	}
	windowRadius := float32(12)
	if props.Theme.AppBorderRadius != nil {
		windowRadius = float32(*props.Theme.AppBorderRadius)
	}
	rowHeight, rowGap, toolbarHeight := demoRowHeight(props), max(float32(0), props.RowGap), demoToolbarHeight(props)
	if props.Geometry != nil {
		rowHeight = max(float32(0), rowHeight-6+props.Geometry.ItemPadding.Top+props.Geometry.ItemPadding.Bottom)
	}
	resultTop := resultInsets.Top
	if props.ShowQuery {
		resultTop += appInsets.Top + queryHeight
	}
	footerHeight := float32(0)
	if props.ShowToolbar {
		footerHeight = toolbarHeight
	}
	visibleResults := float32(len(props.Results))
	if props.FadeResults {
		// Welcome onboarding fades results in after the query appears. Shrink the
		// window with that opacity, otherwise the empty slots keep a tall hole of
		// the light blurred wallpaper between the query box and toolbar.
		visibleResults *= min(max(float32(0), props.ResultsOpacity), 1)
	}
	listHeight := min(demoResultListHeight(visibleResults, rowHeight, rowGap), max(float32(0), props.Height-resultTop-footerHeight))
	renderHeight := props.Height
	compactHeight := resultTop + listHeight + footerHeight
	if props.Geometry != nil {
		compactHeight += resultInsets.Bottom + appInsets.Bottom
	}
	if props.ShowToolbar || !props.ShowQuery {
		if props.Preview == nil {
			renderHeight = min(renderHeight, compactHeight)
		} else if props.FadeResults {
			// Preview would otherwise pin the window to the full slot while the
			// query is still being typed. Grow from the compact query chrome to
			// the preview pane as results fade in.
			fade := min(max(float32(0), props.ResultsOpacity), 1)
			renderHeight = min(renderHeight, compactHeight+(props.Height-compactHeight)*fade)
		}
	}
	resultWidth := props.Width
	if props.Preview != nil && props.ResultWidth > 0 {
		resultWidth = min(props.ResultWidth, props.Width-120)
	}
	windowHeight := renderHeight + 2*contentBounds.Y
	windowRadius = min(max(float32(0), windowRadius), min(windowWidth, windowHeight)/2)
	tint := demoMicaColor(background)
	if props.Theme.AppWindowChrome {
		// Custom chrome disables system glass, but retains authored alpha.
		tint = background
	}
	mica := woxwidget.Container{Width: windowWidth, Height: windowHeight, Radius: windowRadius, Color: tint}
	underlay := woxwidget.Widget(woxwidget.Container{Width: windowWidth, Height: windowHeight, Radius: windowRadius, Color: woxui.Color{A: 255}})
	if props.Backdrop != nil {
		underlay = woxwidget.Image{Source: props.Backdrop, Width: windowWidth, Height: windowHeight, Radius: windowRadius, Fit: woxwidget.ImageFitCover}
	}
	// Keep the glass chrome at rest opacity while content fades. Fading the mica
	// tint or dropping the underlay punches through to the scene behind the window.
	children := []woxwidget.StackChild{}
	if props.ShowQuery {
		query := demoQuery(props, queryHeight, alpha)
		children = append(children, woxwidget.StackChild{Left: appInsets.Left, Top: appInsets.Top, Right: appInsets.Right, StretchWidth: true, Child: demoHighlight(query, max(float32(0), props.Width-appInsets.Left-appInsets.Right), queryHeight, demoQueryRadius(props.Theme), props.HighlightTarget == LauncherDemoHighlightQueryBackground, props.HighlightColor, props.HighlightCorners)})
	}
	for index, result := range props.Results {
		resultAlpha := alpha
		if props.FadeResults {
			resultAlpha = demoScaledAlpha(opacity*props.ResultsOpacity, 255)
		}
		rowWidth := max(float32(0), resultWidth-appInsets.Left-appInsets.Right-resultInsets.Left-resultInsets.Right)
		children = append(children, woxwidget.StackChild{
			Left: appInsets.Left + resultInsets.Left, Top: resultTop + float32(index)*(rowHeight+rowGap), Right: max(appInsets.Right, props.Width-resultWidth+appInsets.Right+resultInsets.Right), StretchWidth: true,
			Child: demoResultRow(props, result, rowWidth, rowHeight, resultAlpha),
		})
	}
	if props.Preview != nil && (!props.FadeResults || props.ResultsOpacity > .01) {
		previewTop := resultTop + 4
		// Live results keep AppPaddingBottom above the toolbar; the preview pane should too.
		previewBottom := footerHeight + appInsets.Bottom
		children = append(children, woxwidget.StackChild{
			Left: resultWidth + 2, Top: previewTop,
			Child: woxwidget.Clip{
				Width: max(float32(0), props.Width-resultWidth-16), Height: max(float32(0), renderHeight-previewBottom-previewTop),
				Child: props.Preview,
			},
		})
	}
	if props.ShowToolbar {
		footerRadius := windowRadius
		if props.Theme.AppContentInset > 0 || props.Theme.AppContentBorderRadius > 0 {
			footerRadius = props.Theme.AppContentBorderRadius
		}
		children = append(children, woxwidget.StackChild{Top: renderHeight - footerHeight, Child: demoToolbar(props, footerHeight, footerRadius, alpha)})
	}
	if props.ActionProgress > .01 {
		panelWidth := min(float32(320), max(float32(0), props.Width-32))
		panelHeight := demoActionPanelHeight()
		if props.Geometry != nil {
			panelHeight += props.Geometry.ActionPadding.Top + props.Geometry.ActionPadding.Bottom - demoActionPanelPaddingTop
		}
		queryLimit := float32(0)
		if props.ShowQuery {
			queryLimit = queryHeight
		}
		// Keep the overlay below the query box, the same constraint the live action panel uses.
		panelHeight = min(panelHeight, max(float32(100), renderHeight-queryLimit-footerHeight-20))
		children = append(children, woxwidget.StackChild{
			Left: props.Width - panelWidth - 16 + 18*(1-props.ActionProgress), Top: renderHeight - footerHeight - panelHeight - 12 + 10*(1-props.ActionProgress),
			Child: demoActionPanel(props, panelWidth, panelHeight, demoAlpha(props.ActionProgress)),
		})
	}
	if props.Theme.AppContentInset != 0 || props.Theme.AppContentBackground.A != 0 || props.Theme.AppContentBorderRadius != 0 || props.HighlightTarget == LauncherDemoHighlightContent {
		inner := demoHighlight(woxwidget.Stack{Width: props.Width, Height: renderHeight, Children: children}, props.Width, renderHeight, props.Theme.AppContentBorderRadius, props.HighlightTarget == LauncherDemoHighlightContent, props.HighlightColor, props.HighlightCorners)
		content := WoxLauncherContent(windowWidth, windowHeight, props.Theme, inner)
		children = []woxwidget.StackChild{{Child: content}}
	}
	children = append([]woxwidget.StackChild{{Child: underlay}, {Child: mica}}, children...)
	borderColor, borderWidth := demoWindowBorderColor(props.Theme.PreviewSplit, opacity), float32(1)
	if props.Theme.AppBorderColor != nil {
		borderColor = *props.Theme.AppBorderColor
		borderColor.A = uint8(float32(borderColor.A) * opacity)
	}
	if props.Theme.AppBorderWidth != nil {
		borderWidth = float32(*props.Theme.AppBorderWidth)
	}
	if props.HighlightTarget == LauncherDemoHighlightSurface && !props.HighlightCorners {
		borderColor, borderWidth = props.HighlightColor, 2
	}
	children = append(children, woxwidget.StackChild{Child: woxwidget.Container{
		Width: windowWidth, Height: windowHeight, Radius: windowRadius, BorderColor: borderColor, BorderWidth: borderWidth,
	}})
	if props.HighlightTarget == LauncherDemoHighlightScrollbar {
		thumbWidth, radius := float32(6), float32(3)
		if props.Theme.ScrollbarWidth != nil {
			thumbWidth = max(float32(1), float32(*props.Theme.ScrollbarWidth))
		}
		if props.Theme.ScrollbarBorderRadius != nil {
			radius = float32(*props.Theme.ScrollbarBorderRadius)
		}
		children = append(children, woxwidget.StackChild{Left: windowWidth - thumbWidth - 2, Top: queryHeight + 8, Child: demoHighlight(woxwidget.Container{Width: thumbWidth, Height: 48, Radius: radius, Color: props.Theme.ResultSubtitle}, thumbWidth, 48, radius, true, props.HighlightColor, true)})
	}
	if props.HighlightTarget == LauncherDemoHighlightSurface && props.HighlightCorners {
		children = append(children, woxwidget.StackChild{Child: CornerRadiusHighlight(windowWidth, windowHeight, windowRadius, props.HighlightColor)})
	}
	return woxwidget.Clip{Width: windowWidth, Height: windowHeight, Child: woxwidget.Stack{Width: windowWidth, Height: windowHeight, Children: children}}
}

func demoQuery(props LauncherDemoProps, height float32, alpha uint8) woxwidget.Widget {
	lineHeight := min(float32(34), max(float32(18), height-16))
	style := woxui.TextStyle{Size: demoQueryFontSize(props)}
	var query woxwidget.Widget
	if len(props.QueryParts) > 0 {
		parts := make([]woxwidget.Widget, 0, len(props.QueryParts))
		for _, part := range props.QueryParts {
			if part.Caret {
				caret := woxwidget.Widget(woxwidget.Container{Width: 2, Height: lineHeight, Color: withAlpha(part.Color, demoScaledAlpha(props.Opacity, part.Color.A))})
				if props.HighlightTarget == LauncherDemoHighlightQueryCaret {
					caret = demoHighlight(caret, 6, lineHeight, 2, true, props.HighlightColor, props.HighlightCorners)
				}
				parts = append(parts, caret)
				continue
			}
			text := woxwidget.Text{Value: part.Text, Style: style, Color: withAlpha(part.Color, demoScaledAlpha(props.Opacity, part.Color.A))}
			if !part.Selected && part.Background.A == 0 {
				parts = append(parts, demoInlineHighlight(text, lineHeight, 3, props.HighlightTarget == LauncherDemoHighlightQueryText, props.HighlightColor))
				continue
			}
			selection := woxwidget.Container{Height: lineHeight, Radius: 3, Color: demoColorOpacity(part.Background, props.Opacity), Child: text}
			parts = append(parts, demoInlineHighlight(selection, lineHeight, 3, props.HighlightTarget == LauncherDemoHighlightQuerySelection, props.HighlightColor))
		}
		query = woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: parts}
	} else {
		query = demoInlineHighlight(woxwidget.Text{Value: props.Query, Style: style, Color: withAlpha(props.Theme.QueryText, alpha)}, lineHeight, 3, props.HighlightTarget == LauncherDemoHighlightQueryText, props.HighlightColor)
	}
	children := []woxwidget.Widget{woxwidget.Expanded{Child: woxwidget.Align{Height: height, Vertical: .5, Child: query}}}
	if props.QueryAccessory != nil {
		children = append(children, props.QueryAccessory)
	}
	// Live query chrome uses 8px left / 6px right so glance sits on the same edge.
	borderColor, borderWidth := props.Theme.QueryBottomBorder()
	return woxwidget.Container{BottomBorderColor: demoColorOpacity(borderColor, props.Opacity), BottomBorderWidth: borderWidth, Height: height, Radius: demoQueryRadius(props.Theme), Color: demoColorOpacity(props.Theme.QueryBackground, props.Opacity), Padding: woxwidget.Insets{Left: 8, Right: 6}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children,
	}}
}

func demoResultRow(props LauncherDemoProps, result LauncherDemoResult, width, height float32, alpha uint8) woxwidget.Widget {
	const tailHeight, tailPadding = float32(22), float32(8)
	padding := woxwidget.Insets{Left: 13, Right: 13, Top: 3, Bottom: 3}
	if props.Geometry != nil {
		padding = props.Geometry.ItemPadding
	}
	baseHeight := max(float32(0), height-padding.Top-padding.Bottom)
	iconSize := min(float32(28), max(float32(18), baseHeight-8))
	iconGap := float32(10)
	if height < 50 {
		iconGap = 8
	}
	background := woxui.Color{}
	if result.Selected {
		background = demoColorOpacity(props.Theme.SelectedBackground, float32(alpha)/255)
	} else if result.Hovered {
		background = demoColorOpacity(props.Theme.ResultHoverColor(), float32(alpha)/255)
	}
	tailWidth := float32(0)
	if result.Tail != "" {
		tailWidth = min(float32(140), demoResultTailTextWidth(result.Tail)+tailPadding*2)
	}
	textWidth := max(float32(0), width-padding.Left-padding.Right-iconSize-iconGap)
	if tailWidth > 0 {
		textWidth = max(float32(0), textWidth-iconGap-tailWidth)
	}
	highlightTitle := props.HighlightTarget == LauncherDemoHighlightResultTitle && !result.Selected || props.HighlightTarget == LauncherDemoHighlightSelectedTitle && result.Selected
	titleLine, subtitleLine := float32(20), float32(16)
	if height < 50 {
		titleLine, subtitleLine = 18, 14
	}
	title := woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: demoResultTitleFontSize(props)}, Color: withAlpha(demoResultColor(result.Selected, props.Theme.SelectedTitle, props.Theme.ResultTitle), alpha)}
	labels := []woxwidget.Widget{demoInlineHighlight(title, titleLine, 3, highlightTitle, props.HighlightColor)}
	if result.Subtitle != "" {
		subtitle := woxwidget.Text{Value: result.Subtitle, Style: woxui.TextStyle{Size: demoResultSubtitleFontSize(props)}, Color: withAlpha(demoResultColor(result.Selected, props.Theme.SelectedSubtitle, props.Theme.ResultSubtitle), alpha)}
		labels = append(labels, demoInlineHighlight(subtitle, subtitleLine, 3, props.HighlightTarget == LauncherDemoHighlightResultSubtitle && !result.Selected, props.HighlightColor))
	}
	children := []woxwidget.Widget{
		woxwidget.Align{Width: iconSize, Height: baseHeight, Vertical: .5, Child: woxwidget.Container{Width: iconSize, Height: iconSize, Radius: 7, Color: withAlpha(result.GlyphColor, demoScaledAlpha(float32(alpha)/255, 54)), Child: woxwidget.Align{
			Width: iconSize, Height: iconSize, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: result.Glyph, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: withAlpha(result.GlyphColor, alpha)},
		}}},
		woxwidget.Clip{Width: textWidth, Height: baseHeight, Child: woxwidget.Align{Width: textWidth, Height: baseHeight, Vertical: .5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: labels}}},
	}
	if tailWidth > 0 {
		// Result tails have their own theme tokens. Reusing ResultSubtitle made the
		// theme editor look like subtitle edits also restyled the tail chips.
		textSlot := max(float32(0), tailWidth-tailPadding*2)
		tailColor := demoResultColor(result.Selected, props.Theme.SelectedTail, props.Theme.ResultTail)
		borderAlpha := uint8(51)
		if result.Selected {
			borderAlpha = 87
		}
		tail := woxwidget.Container{
			Width: tailWidth, Height: tailHeight, Radius: tailHeight / 2, BorderColor: withAlpha(tailColor, demoScaledAlpha(float32(alpha)/255, borderAlpha)), BorderWidth: 1,
			Padding: woxwidget.Insets{Left: tailPadding, Right: tailPadding},
			Child:   woxwidget.Align{Width: textSlot, Height: tailHeight, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: result.Tail, Style: woxui.TextStyle{Size: TailFontSize}, Color: withAlpha(tailColor, alpha)}},
		}
		highlightTail := props.HighlightTarget == LauncherDemoHighlightResultTail && !result.Selected || props.HighlightTarget == LauncherDemoHighlightSelectedTail && result.Selected
		children = append(children, woxwidget.Align{Width: tailWidth, Height: baseHeight, Vertical: .5, Child: demoHighlight(tail, tailWidth, tailHeight, tailHeight/2, highlightTail, props.HighlightColor, props.HighlightCorners)})
	}
	row := woxwidget.Container{Width: width, Height: height, Radius: demoResultRadius(props.Theme), Color: background, Padding: padding, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: iconGap, Children: children,
	}}
	if result.Selected {
		if props.Theme.ResultItemActiveIndicatorWidth != nil {
			indicator := props.Theme.ResultIndicator()
			indicator.Color = demoColorOpacity(indicator.Color, float32(alpha)/255)
			content := row
			content.Color = woxui.Color{}
			var contentWidget woxwidget.Widget = content
			if props.HighlightTarget == LauncherDemoHighlightIndicator {
				contentWidget = woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Child: content}, {Left: indicator.Left, Top: indicator.Top, Child: CornerRadiusHighlight(min(width, indicator.Width), max(float32(0), height-indicator.Top-indicator.Bottom), indicator.Radius, props.HighlightColor)}}}
			}
			return demoHighlight(woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Child: ResultIndicatorBackground(width, height, row.Radius, background, indicator)}, {Child: contentWidget}}}, width, height, row.Radius, props.HighlightTarget == LauncherDemoHighlightSelectedBackground, props.HighlightColor, props.HighlightCorners)
		}
		row.LeftBorderWidth = max(float32(0), props.Theme.SelectedBorderLeftWidth)
		row.LeftBorderColor = demoColorOpacity(props.Theme.SelectedBorderLeftColor, float32(alpha)/255)
	}
	return demoHighlight(row, width, height, demoResultRadius(props.Theme), props.HighlightTarget == LauncherDemoHighlightSelectedBackground && result.Selected || props.HighlightTarget == LauncherDemoHighlightResultHover && result.Hovered && !result.Selected, props.HighlightColor, props.HighlightCorners)
}

// demoResultTailTextWidth estimates tail text so CJK glyphs keep the same 8px inset as production tags.
func demoResultTailTextWidth(text string) float32 {
	width := float32(0)
	for _, r := range text {
		if r > 0xFF {
			width += TailFontSize
			continue
		}
		width += 7
	}
	return width
}

func demoToolbar(props LauncherDemoProps, height, windowRadius float32, alpha uint8) woxwidget.Widget {
	padding := woxwidget.Insets{Left: 12, Right: 12}
	if props.Geometry != nil {
		padding = props.Geometry.ToolbarPadding
	}
	primary := props.PrimaryAction
	if primary == "" {
		primary = "Execute"
	}
	more := props.ActionMore
	if more == "" {
		more = "More Actions"
	}
	modifier := "Ctrl"
	if runtime.GOOS == "darwin" {
		modifier = "Cmd"
	}
	keycap := func(label string, width float32, active, primary bool) woxwidget.Widget {
		border := withAlpha(props.Theme.ToolbarText, demoScaledAlpha(float32(alpha)/255, 150))
		fill := withAlpha(props.Theme.ToolbarText, demoScaledAlpha(float32(alpha)/255, 9))
		if active {
			border = withAlpha(props.Accent, demoScaledAlpha(float32(alpha)/255, 200))
			fill = withAlpha(props.Accent, demoScaledAlpha(float32(alpha)/255, 28))
		}
		foreground := withAlpha(props.Theme.ToolbarText, alpha)
		if c := props.Theme.ToolbarHotkeyFontColor; c != nil {
			foreground = demoColorOpacity(*c, float32(alpha)/255)
		}
		if c := props.Theme.ToolbarHotkeyBackgroundColor; c != nil {
			fill = demoColorOpacity(*c, float32(alpha)/255)
		}
		if c := props.Theme.ToolbarHotkeyBorderColor; c != nil {
			border = demoColorOpacity(*c, float32(alpha)/255)
		}
		if primary {
			if c := props.Theme.ToolbarPrimaryHotkeyFontColor; c != nil {
				foreground = demoColorOpacity(*c, float32(alpha)/255)
			}
			if c := props.Theme.ToolbarPrimaryHotkeyBackgroundColor; c != nil {
				fill = demoColorOpacity(*c, float32(alpha)/255)
			}
			if c := props.Theme.ToolbarPrimaryHotkeyBorderColor; c != nil {
				border = demoColorOpacity(*c, float32(alpha)/255)
			}
		}
		cap := woxwidget.Container{Width: width, Height: 20, Radius: 4, Color: fill, BorderColor: border, BorderWidth: 1, Child: woxwidget.Align{
			Width: width, Height: 20, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightRegular}, Color: foreground},
		}}
		return demoHighlight(cap, width, 20, 4, props.HighlightTarget == LauncherDemoHighlightHotkey || primary && props.HighlightTarget == LauncherDemoHighlightToolbarPrimaryHotkey, props.HighlightColor, props.HighlightCorners)
	}
	primaryColor := props.Theme.ToolbarText
	if props.Theme.ToolbarPrimaryFontColor != nil {
		primaryColor = *props.Theme.ToolbarPrimaryFontColor
	}
	content := woxwidget.Widget(woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		demoInlineHighlight(woxwidget.Text{Value: primary, Style: woxui.TextStyle{Size: 11}, Color: demoColorOpacity(primaryColor, float32(alpha)/255)}, 20, 3, props.HighlightTarget == LauncherDemoHighlightToolbarText || props.HighlightTarget == LauncherDemoHighlightToolbarPrimaryText, props.HighlightColor), keycap("Enter", 36, false, true),
		woxwidget.Container{Width: 8}, demoInlineHighlight(woxwidget.Text{Value: more, Style: woxui.TextStyle{Size: 11}, Color: demoColorOpacity(props.Theme.ToolbarText, float32(alpha)/255)}, 20, 3, props.HighlightTarget == LauncherDemoHighlightToolbarText, props.HighlightColor),
		keycap(modifier, 30, props.ToolbarPressed, false), keycap("J", 20, props.ToolbarPressed, false),
	}})
	// Live toolbar uses Floating material. A solid fill keeps Glass's near-clear
	// tint as a hard band. Extend the rounded rect above the clip so only the
	// window's bottom corners stay rounded — the same trick as LauncherToolbarView.
	fill := woxwidget.Container{
		Width: props.Width, Height: height + windowRadius, Radius: windowRadius,
		Color: demoColorOpacity(props.Theme.ToolbarBackground, props.Opacity), Floating: true,
	}
	toolbar := woxwidget.Container{Width: props.Width, Height: height, Child: woxwidget.Clip{
		Width: props.Width, Height: height,
		Child: woxwidget.Stack{Width: props.Width, Height: height, Children: []woxwidget.StackChild{
			{Top: -windowRadius, Child: fill},
			{Child: woxwidget.Painter{Width: props.Width, Height: min(height, max(float32(0), props.Theme.ToolbarBorderWidth)), Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				if props.Theme.ToolbarBorderWidth > 0 {
					displayList.FillRect(bounds, demoColorOpacity(props.Theme.ToolbarBorder, props.Opacity))
				}
			}}},
			{Child: woxwidget.Container{Width: props.Width, Height: height, Padding: padding, Child: woxwidget.Align{
				Width: max(float32(0), props.Width-padding.Left-padding.Right), Height: height, Horizontal: 1, Vertical: .5, Child: content,
			}}},
		}},
	}}
	return demoHighlight(toolbar, props.Width, height, windowRadius, props.HighlightTarget == LauncherDemoHighlightToolbarBackground, props.HighlightColor, props.HighlightCorners)
}

func demoActionPanel(props LauncherDemoProps, width, height float32, alpha uint8) woxwidget.Widget {
	padding := woxwidget.Insets{Left: 10, Right: 10, Top: demoActionPanelPaddingTop}
	queryRadius := float32(5)
	if props.Geometry != nil {
		padding = props.Geometry.ActionPadding
		queryRadius = props.Geometry.ActionQueryRadius
	}
	innerWidth := max(float32(0), width-padding.Left-padding.Right)
	copyLabel := props.ActionCopy
	if copyLabel == "" {
		copyLabel = "Copy"
	}
	moreLabel := props.ActionMore
	if moreLabel == "" {
		moreLabel = "More"
	}
	children := []woxwidget.Widget{
		demoInlineHighlight(woxwidget.Text{Value: "Actions", Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: withAlpha(props.Theme.ActionHeader, alpha)}, demoActionHeaderHeight, 3, props.HighlightTarget == LauncherDemoHighlightActionHeader, props.HighlightColor),
		woxwidget.Container{Width: innerWidth, Height: demoActionHeaderGap},
	}
	if props.HighlightTarget == LauncherDemoHighlightActionDivider {
		children[1] = woxwidget.Align{Width: innerWidth, Height: demoActionHeaderGap, Vertical: .5, Child: woxwidget.Container{Width: innerWidth, Height: 1, Color: demoColorOpacity(props.Theme.ActionDividerColor(), float32(alpha)/255)}}
		children[1] = demoHighlight(children[1], innerWidth, demoActionHeaderGap, 2, true, props.HighlightColor, props.HighlightCorners)
	}
	actions := []struct {
		label string
		icon  func(float32, woxui.Color) woxwidget.Widget
	}{
		{copyLabel, CopyGlyph},
		{moreLabel, MenuGlyph},
	}
	const iconSize, iconSlotWidth = float32(22), float32(37)
	for index, action := range actions {
		background, foreground := woxui.Color{}, props.Theme.ActionText
		if index == 0 {
			background, foreground = props.Theme.ActionSelected, props.Theme.ActionSelectedText
		}
		textHighlight := props.HighlightTarget == LauncherDemoHighlightActionText && index > 0 || props.HighlightTarget == LauncherDemoHighlightActionSelectedText && index == 0
		iconColor := withAlpha(foreground, alpha)
		row := woxwidget.Container{Width: innerWidth, Height: demoActionRowHeight, Radius: props.Theme.ActionItemRadius, Color: withAlpha(background, demoScaledAlpha(float32(alpha)/255, background.A)), Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Align{Width: iconSlotWidth, Height: demoActionRowHeight, Vertical: .5, Child: woxwidget.Container{
					Width: iconSlotWidth, Padding: woxwidget.Insets{Left: 5, Right: 10}, Child: action.icon(iconSize, iconColor),
				}},
				woxwidget.Align{Height: demoActionRowHeight, Vertical: .5, Child: demoInlineHighlight(woxwidget.Text{Value: action.label, Style: woxui.TextStyle{Size: 10}, Color: withAlpha(foreground, alpha)}, 16, 3, textHighlight, props.HighlightColor)},
			},
		}}
		keyTheme := props.Theme
		for _, slot := range []**woxui.Color{&keyTheme.ActionItemHotkeyFontColor, &keyTheme.ActionItemHotkeyBackgroundColor, &keyTheme.ActionItemHotkeyBorderColor, &keyTheme.ActionItemActiveHotkeyFontColor, &keyTheme.ActionItemActiveHotkeyBackgroundColor, &keyTheme.ActionItemActiveHotkeyBorderColor} {
			if *slot != nil {
				color := demoColorOpacity(**slot, float32(alpha)/255)
				*slot = &color
			}
		}
		label := "Enter"
		if index > 0 {
			label = "Ctrl"
		}
		keycap, keyWidth := WoxHotkey(HotkeyProps{Theme: &keyTheme, Selected: index == 0, Labels: []string{label}, Foreground: withAlpha(foreground, alpha), Background: withAlpha(background, alpha), Window: props.Window, Compact: true})
		flash := index == 0 && props.HighlightTarget == LauncherDemoHighlightActionActiveHotkey || index > 0 && props.HighlightTarget == LauncherDemoHighlightActionHotkey
		// Reserve a trailing slot so the preview label cannot overlap its keycap.
		content := row.Child
		row.Child = woxwidget.Stack{Width: innerWidth, Height: demoActionRowHeight, Children: []woxwidget.StackChild{{Child: woxwidget.Clip{Width: max(float32(0), innerWidth-10-keyWidth), Height: demoActionRowHeight, Child: content}}, {Left: innerWidth - 5 - keyWidth, Child: woxwidget.Align{Width: keyWidth, Height: demoActionRowHeight, Vertical: .5, Child: demoHighlight(keycap, keyWidth, 22, 4, flash, props.HighlightColor, props.HighlightCorners)}}}}
		children = append(children, demoHighlight(row, innerWidth, demoActionRowHeight, props.Theme.ActionItemRadius, props.HighlightTarget == LauncherDemoHighlightActionSelectedBackground && index == 0, props.HighlightColor, props.HighlightCorners))
	}
	query := woxwidget.Container{
		Width: innerWidth, Height: 28, Radius: queryRadius, Color: demoColorOpacity(props.Theme.QueryBackground, float32(alpha)/255), Padding: woxwidget.Insets{Left: 9}, Child: woxwidget.Align{Height: 28, Vertical: .5, Child: woxwidget.Text{Value: props.Query, Style: woxui.TextStyle{Size: 9}, Color: withAlpha(props.Theme.ActionText, demoScaledAlpha(float32(alpha)/255, 170))}},
	}

	children = append(children, woxwidget.Container{Width: innerWidth, Height: demoActionSearchHeight, Padding: woxwidget.Insets{Top: 8}, Child: demoHighlight(query, innerWidth, 28, queryRadius, props.HighlightTarget == LauncherDemoHighlightActionQueryBackground, props.HighlightColor, props.HighlightCorners)})
	panel := woxwidget.Container{Width: width, Height: height, Radius: 8, Floating: true, Color: demoColorOpacity(props.Theme.ActionBackground, float32(alpha)/255), Padding: padding, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
	panel.BorderColor = demoColorOpacity(props.Theme.ActionBorder, float32(alpha)/255)
	panel.BorderWidth = props.Theme.ActionBorderWidth
	panel.Radius = props.Theme.ActionContainerRadius
	return demoHighlight(panel, width, height, panel.Radius, props.HighlightTarget == LauncherDemoHighlightActionBackground, props.HighlightColor, props.HighlightCorners)
}

const (
	demoActionPanelPaddingTop = float32(10)
	demoActionHeaderHeight    = float32(18)
	// demoActionHeaderGap mirrors the live panel, which separates the title by spacing only.
	demoActionHeaderGap    = float32(8)
	demoActionRowHeight    = float32(38)
	demoActionSearchHeight = float32(36)
	demoActionCount        = 2
)

// demoActionPanelHeight sizes the overlay to the two demo actions, matching the live panel's row-driven height.
func demoActionPanelHeight() float32 {
	return demoActionPanelPaddingTop + demoActionHeaderHeight + demoActionHeaderGap + float32(demoActionCount)*demoActionRowHeight + demoActionSearchHeight
}

func demoAlpha(opacity float32) uint8 { return demoScaledAlpha(opacity, 255) }

// demoAppPadding uses the leading inset as the demo's uniform window padding.
func demoAppPadding(theme Theme) float32 {
	if theme.AppPadding.Left > 0 {
		return theme.AppPadding.Left
	}
	return 10
}

// demoQueryRadius keeps the previous 8-unit default when a Theme literal omits QueryRadius.
func demoQueryRadius(theme Theme) float32 {
	if theme.QueryRadius > 0 || theme.QueryBoxBorderBottomWidth != nil {
		return theme.QueryRadius
	}
	return 8
}

func demoResultRadius(theme Theme) float32 {
	if theme.ResultItemRadius > 0 || theme.ResultItemActiveIndicatorWidth != nil {
		return theme.ResultItemRadius
	}
	return 8
}

func demoQueryFontSize(props LauncherDemoProps) float32 {
	if props.QueryFontSize > 0 {
		return props.QueryFontSize
	}
	return QueryFontSize
}

func demoResultTitleFontSize(props LauncherDemoProps) float32 {
	if props.ResultTitleFontSize > 0 {
		return props.ResultTitleFontSize
	}
	return ResultTitleFontSize
}

func demoResultSubtitleFontSize(props LauncherDemoProps) float32 {
	if props.ResultSubtitleFontSize > 0 {
		return props.ResultSubtitleFontSize
	}
	return ResultSubtitleFontSize
}

func demoQueryHeight(props LauncherDemoProps) float32 {
	if props.QueryHeight > 0 {
		return props.QueryHeight
	}
	return 55
}

func demoRowHeight(props LauncherDemoProps) float32 {
	if props.RowHeight > 0 {
		return props.RowHeight
	}
	return 56
}

func demoToolbarHeight(props LauncherDemoProps) float32 {
	if props.ToolbarHeight > 0 {
		return props.ToolbarHeight
	}
	return 40
}

func demoResultListHeight(count, rowHeight, rowGap float32) float32 {
	if count <= 0 {
		return 0
	}
	return count*rowHeight + max(float32(0), count-1)*rowGap
}

// demoWindowBorderMaxAlpha keeps opaque split colors as a quiet window edge.
// Glass themes already author a hairline around 0.16; replacing that alpha
// with a heavier overlay turns white splits into a bright outline.
const demoWindowBorderMaxAlpha uint8 = 56

// demoWindowBorderColor preserves translucent theme hairlines and caps opaque splits.
func demoWindowBorderColor(split woxui.Color, opacity float32) woxui.Color {
	alpha := split.A
	if alpha == 0 || alpha > demoWindowBorderMaxAlpha {
		alpha = demoWindowBorderMaxAlpha
	}
	return withAlpha(split, demoScaledAlpha(opacity, alpha))
}

func demoScaledAlpha(opacity float32, alpha uint8) uint8 {
	return uint8(min(max(float32(0), opacity), float32(1))*float32(alpha) + .5)
}

func demoColorOpacity(color woxui.Color, opacity float32) woxui.Color {
	color.A = demoScaledAlpha(opacity, color.A)
	return color
}

func demoMicaColor(color woxui.Color) woxui.Color {
	if color.A >= 245 {
		color.A = 255
		return color
	}
	tint := float32(32)
	if .2126*float32(color.R)+.7152*float32(color.G)+.0722*float32(color.B) >= 127.5 {
		tint = 242
	}
	const mix = float32(.18)
	color.R = uint8(float32(color.R)*(1-mix) + tint*mix + .5)
	color.G = uint8(float32(color.G)*(1-mix) + tint*mix + .5)
	color.B = uint8(float32(color.B)*(1-mix) + tint*mix + .5)
	color.A = uint8(min(max(float32(.64)+float32(color.A)/255*.18, float32(.64)), float32(.86))*255 + .5)
	return color
}

func demoResultColor(selected bool, selectedColor, normalColor woxui.Color) woxui.Color {
	if selected {
		return selectedColor
	}
	return normalColor
}

func demoBoolFloat(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

func demoHighlight(child woxwidget.Widget, width, height, radius float32, visible bool, color woxui.Color, corners ...bool) woxwidget.Widget {
	if !visible {
		return child
	}
	if len(corners) > 0 && corners[0] {
		return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Child: child}, {Child: CornerRadiusHighlight(width, height, radius, color)}}}
	}
	fill := color
	fill.A = 42
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: child},
		{Child: woxwidget.Container{Width: width, Height: height, Radius: radius, Color: fill, BorderColor: color, BorderWidth: 2}},
	}}
}

func demoInlineHighlight(child woxwidget.Widget, height, radius float32, visible bool, color woxui.Color) woxwidget.Widget {
	if !visible {
		return child
	}
	fill := color
	fill.A = 42
	return woxwidget.Container{Height: height, Radius: radius, Color: fill, BorderColor: color, BorderWidth: 2, Child: child}
}

// CornerRadiusHighlight draws only the corner segments in logical units, following the effective radius.
func CornerRadiusHighlight(width, height, radius float32, color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: width, Height: height, Paint: func(dl *woxui.DisplayList, bounds woxui.Rect) {
		r := min(max(float32(0), radius), bounds.Width/2, bounds.Height/2)
		extent := min(max(float32(6), r+3), bounds.Width/2, bounds.Height/2)
		for _, x := range []float32{bounds.X, bounds.X + bounds.Width - extent} {
			for _, y := range []float32{bounds.Y, bounds.Y + bounds.Height - extent} {
				dl.PushClipRect(woxui.Rect{X: x, Y: y, Width: extent, Height: extent})
				dl.StrokeRoundedRect(bounds, r, 2, color)
				dl.PopClipRect()
			}
		}
	}}
}
