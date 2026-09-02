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
}

// LauncherDemoHighlightTarget identifies one semantic surface inside the simulated launcher.
type LauncherDemoHighlightTarget uint8

const (
	LauncherDemoHighlightNone LauncherDemoHighlightTarget = iota
	LauncherDemoHighlightSurface
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
	LauncherDemoHighlightActionBackground
	LauncherDemoHighlightActionHeader
	LauncherDemoHighlightActionText
	LauncherDemoHighlightActionSelectedBackground
	LauncherDemoHighlightActionSelectedText
	LauncherDemoHighlightActionQueryBackground
)

// LauncherDemoProps contains the complete simulated launcher state shared by previews.
type LauncherDemoProps struct {
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
	QueryAccessory         woxwidget.Widget
	HighlightColor         woxui.Color
	HighlightTarget        LauncherDemoHighlightTarget
}

// WoxLauncherDemo builds the complete query, results, preview, action panel, and toolbar demo.
func WoxLauncherDemo(props LauncherDemoProps) woxwidget.Widget {
	opacity := min(max(float32(0), props.Opacity), float32(1))
	alpha := demoAlpha(opacity)
	background := props.Background
	if background.A == 0 {
		background = props.Theme.Background
	}
	const appPadding, queryHeight, windowRadius = float32(10), float32(55), float32(12)
	const resultContainerTop, rowHeight, toolbarHeight = float32(8), float32(56), float32(40)
	resultTop := resultContainerTop
	if props.ShowQuery {
		resultTop += appPadding + queryHeight
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
	listHeight := min(visibleResults*rowHeight, max(float32(0), props.Height-resultTop-footerHeight))
	renderHeight := props.Height
	compactHeight := resultTop + listHeight + footerHeight
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
	mica := woxwidget.Container{Width: props.Width, Height: renderHeight, Radius: windowRadius, Color: demoMicaColor(background)}
	underlay := woxwidget.Widget(woxwidget.Container{Width: props.Width, Height: renderHeight, Radius: windowRadius, Color: woxui.Color{A: 255}})
	if props.Backdrop != nil {
		underlay = woxwidget.Image{Source: props.Backdrop, Width: props.Width, Height: renderHeight, Radius: windowRadius, Fit: woxwidget.ImageFitCover}
	}
	// Keep the glass chrome at rest opacity while content fades. Fading the mica
	// tint or dropping the underlay punches through to the scene behind the window.
	children := []woxwidget.StackChild{{Child: underlay}, {Child: mica}}
	if props.ShowQuery {
		query := demoQuery(props, queryHeight, alpha)
		children = append(children, woxwidget.StackChild{Left: appPadding, Top: appPadding, Right: appPadding, StretchWidth: true, Child: demoHighlight(query, props.Width-appPadding*2, queryHeight, 8, props.HighlightTarget == LauncherDemoHighlightQueryBackground, props.HighlightColor)})
	}
	for index, result := range props.Results {
		resultAlpha := alpha
		if props.FadeResults {
			resultAlpha = demoScaledAlpha(opacity*props.ResultsOpacity, 255)
		}
		rowWidth := max(float32(0), resultWidth-appPadding*2)
		children = append(children, woxwidget.StackChild{
			Left: appPadding, Top: resultTop + float32(index)*rowHeight, Right: max(appPadding, props.Width-resultWidth+appPadding), StretchWidth: true,
			Child: demoResultRow(props, result, rowWidth, rowHeight, resultAlpha),
		})
	}
	if props.Preview != nil && (!props.FadeResults || props.ResultsOpacity > .01) {
		previewTop := resultTop + 4
		// Live results keep AppPaddingBottom above the toolbar; the preview pane should too.
		previewBottom := footerHeight + appPadding
		children = append(children, woxwidget.StackChild{
			Left: resultWidth + 2, Top: previewTop,
			Child: woxwidget.Clip{
				Width: max(float32(0), props.Width-resultWidth-16), Height: max(float32(0), renderHeight-previewBottom-previewTop),
				Child: props.Preview,
			},
		})
	}
	if props.ShowToolbar {
		children = append(children, woxwidget.StackChild{Top: renderHeight - footerHeight, Child: demoToolbar(props, footerHeight, renderHeight, windowRadius, alpha)})
	}
	if props.ActionProgress > .01 {
		panelWidth := min(float32(250), props.Width*.42)
		panelHeight := demoActionPanelHeight()
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
	borderColor, borderWidth := demoWindowBorderColor(props.Theme.PreviewSplit, opacity), float32(1)
	if props.HighlightTarget == LauncherDemoHighlightSurface {
		borderColor, borderWidth = props.HighlightColor, 2
	}
	children = append(children, woxwidget.StackChild{Child: woxwidget.Container{
		Width: props.Width, Height: renderHeight, Radius: windowRadius, BorderColor: borderColor, BorderWidth: borderWidth,
	}})
	return woxwidget.Clip{Width: props.Width, Height: renderHeight, Child: woxwidget.Stack{Width: props.Width, Height: renderHeight, Children: children}}
}

func demoQuery(props LauncherDemoProps, height float32, alpha uint8) woxwidget.Widget {
	const lineHeight = float32(34)
	style := woxui.TextStyle{Size: QueryFontSize}
	var query woxwidget.Widget
	if len(props.QueryParts) > 0 {
		parts := make([]woxwidget.Widget, 0, len(props.QueryParts))
		for _, part := range props.QueryParts {
			if part.Caret {
				caret := woxwidget.Widget(woxwidget.Container{Width: 2, Height: lineHeight, Color: withAlpha(part.Color, demoScaledAlpha(props.Opacity, part.Color.A))})
				if props.HighlightTarget == LauncherDemoHighlightQueryCaret {
					caret = demoHighlight(caret, 6, lineHeight, 2, true, props.HighlightColor)
				}
				parts = append(parts, caret)
				continue
			}
			text := woxwidget.Text{Value: part.Text, Style: style, Color: withAlpha(part.Color, demoScaledAlpha(props.Opacity, part.Color.A))}
			if part.Background.A == 0 {
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
	return woxwidget.Container{Height: height, Radius: 8, Color: demoColorOpacity(props.Theme.QueryBackground, props.Opacity), Padding: woxwidget.Insets{Left: 8, Right: 6}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children,
	}}
}

func demoResultRow(props LauncherDemoProps, result LauncherDemoResult, width, height float32, alpha uint8) woxwidget.Widget {
	const (
		baseHeight, iconSize, iconGap = float32(50), float32(28), float32(10)
		tailHeight, tailPadding       = float32(22), float32(8)
	)
	background := woxui.Color{}
	if result.Selected {
		background = demoColorOpacity(props.Theme.SelectedBackground, float32(alpha)/255)
	}
	tailWidth := float32(0)
	if result.Tail != "" {
		tailWidth = min(float32(140), demoResultTailTextWidth(result.Tail)+tailPadding*2)
	}
	textWidth := max(float32(0), width-26-iconSize-iconGap)
	if tailWidth > 0 {
		textWidth = max(float32(0), textWidth-iconGap-tailWidth)
	}
	highlightTitle := props.HighlightTarget == LauncherDemoHighlightResultTitle && !result.Selected || props.HighlightTarget == LauncherDemoHighlightSelectedTitle && result.Selected
	title := woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: ResultTitleFontSize}, Color: withAlpha(demoResultColor(result.Selected, props.Theme.SelectedTitle, props.Theme.ResultTitle), alpha)}
	labels := []woxwidget.Widget{demoInlineHighlight(title, 20, 3, highlightTitle, props.HighlightColor)}
	if result.Subtitle != "" {
		subtitle := woxwidget.Text{Value: result.Subtitle, Style: woxui.TextStyle{Size: ResultSubtitleFontSize}, Color: withAlpha(demoResultColor(result.Selected, props.Theme.SelectedSubtitle, props.Theme.ResultSubtitle), alpha)}
		labels = append(labels, demoInlineHighlight(subtitle, 16, 3, props.HighlightTarget == LauncherDemoHighlightResultSubtitle && !result.Selected, props.HighlightColor))
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
		children = append(children, woxwidget.Align{Width: tailWidth, Height: baseHeight, Vertical: .5, Child: demoHighlight(tail, tailWidth, tailHeight, tailHeight/2, highlightTail, props.HighlightColor)})
	}
	row := woxwidget.Container{Width: width, Height: height, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 13, Top: 3, Right: 13, Bottom: 3}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: iconGap, Children: children,
	}}
	return demoHighlight(row, width, height, 8, props.HighlightTarget == LauncherDemoHighlightSelectedBackground && result.Selected, props.HighlightColor)
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

func demoToolbar(props LauncherDemoProps, height, windowHeight, windowRadius float32, alpha uint8) woxwidget.Widget {
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
	keycap := func(label string, width float32, active bool) woxwidget.Widget {
		border := withAlpha(props.Theme.ToolbarText, demoScaledAlpha(float32(alpha)/255, 150))
		fill := withAlpha(props.Theme.ToolbarText, demoScaledAlpha(float32(alpha)/255, 9))
		if active {
			border = withAlpha(props.Accent, demoScaledAlpha(float32(alpha)/255, 200))
			fill = withAlpha(props.Accent, demoScaledAlpha(float32(alpha)/255, 28))
		}
		return woxwidget.Container{Width: width, Height: 24, Radius: 4, Color: fill, BorderColor: border, BorderWidth: 1, Child: woxwidget.Align{
			Width: width, Height: 24, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: withAlpha(props.Theme.ToolbarText, alpha)},
		}}
	}
	content := woxwidget.Widget(woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Text{Value: primary, Style: woxui.TextStyle{Size: 11}, Color: withAlpha(props.Theme.ToolbarText, alpha)}, keycap("Enter", 42, false),
		woxwidget.Container{Width: 8}, woxwidget.Text{Value: more, Style: woxui.TextStyle{Size: 11}, Color: withAlpha(props.Theme.ToolbarText, alpha)},
		keycap(modifier, 42, props.ToolbarPressed), keycap("J", 26, props.ToolbarPressed),
	}})
	content = demoInlineHighlight(content, 28, 4, props.HighlightTarget == LauncherDemoHighlightToolbarText, props.HighlightColor)
	// Clip a window-sized rounded fill to the footer. A square toolbar fill would
	// paint into the window's bottom corner cutouts; the query box is inset, so
	// the top corners never showed this.
	fill := woxwidget.Container{
		Width: props.Width, Height: windowHeight, Radius: windowRadius,
		Color: demoColorOpacity(props.Theme.ToolbarBackground, props.Opacity),
	}
	toolbar := woxwidget.Container{Width: props.Width, Height: height, Child: woxwidget.Clip{
		Width: props.Width, Height: height,
		Child: woxwidget.Stack{Width: props.Width, Height: height, Children: []woxwidget.StackChild{
			{Top: height - windowHeight, Child: fill},
			{Child: woxwidget.Container{Width: props.Width, Height: 1, Color: withAlpha(props.Theme.ToolbarText, demoScaledAlpha(props.Opacity, 26))}},
			{Child: woxwidget.Container{Width: props.Width, Height: height, Padding: woxwidget.Insets{Left: 12, Right: 12}, Child: woxwidget.Align{
				Width: props.Width - 24, Height: height, Horizontal: 1, Vertical: .5, Child: content,
			}}},
		}},
	}}
	return demoHighlight(toolbar, props.Width, height, windowRadius, props.HighlightTarget == LauncherDemoHighlightToolbarBackground, props.HighlightColor)
}

func demoActionPanel(props LauncherDemoProps, width, height float32, alpha uint8) woxwidget.Widget {
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
		woxwidget.Container{Width: width - 20, Height: demoActionDividerHeight, Padding: woxwidget.Insets{Top: 7, Bottom: 8}, Child: woxwidget.Container{Height: 1, Color: withAlpha(props.Theme.PreviewSplit, alpha)}},
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
		row := woxwidget.Container{Width: width - 20, Height: demoActionRowHeight, Radius: 5, Color: withAlpha(background, demoScaledAlpha(float32(alpha)/255, background.A)), Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Align{Width: iconSlotWidth, Height: demoActionRowHeight, Vertical: .5, Child: woxwidget.Container{
					Width: iconSlotWidth, Padding: woxwidget.Insets{Left: 5, Right: 10}, Child: action.icon(iconSize, iconColor),
				}},
				woxwidget.Align{Height: demoActionRowHeight, Vertical: .5, Child: demoInlineHighlight(woxwidget.Text{Value: action.label, Style: woxui.TextStyle{Size: 10}, Color: withAlpha(foreground, alpha)}, 16, 3, textHighlight, props.HighlightColor)},
			},
		}}
		children = append(children, demoHighlight(row, width-20, demoActionRowHeight, 5, props.HighlightTarget == LauncherDemoHighlightActionSelectedBackground && index == 0, props.HighlightColor))
	}
	query := woxwidget.Container{
		Width: width - 20, Height: 28, Radius: 5, Color: demoColorOpacity(props.Theme.QueryBackground, float32(alpha)/255), Padding: woxwidget.Insets{Left: 9}, Child: woxwidget.Align{Height: 28, Vertical: .5, Child: woxwidget.Text{Value: props.Query, Style: woxui.TextStyle{Size: 9}, Color: withAlpha(props.Theme.ActionText, demoScaledAlpha(float32(alpha)/255, 170))}},
	}
	children = append(children, woxwidget.Container{Width: width - 20, Height: demoActionSearchHeight, Padding: woxwidget.Insets{Top: 8}, Child: demoHighlight(query, width-20, 28, 5, props.HighlightTarget == LauncherDemoHighlightActionQueryBackground, props.HighlightColor)})
	panel := woxwidget.Container{Width: width, Height: height, Radius: 8, Color: demoColorOpacity(props.Theme.ActionBackground, float32(alpha)/255), Padding: woxwidget.Insets{Left: 10, Top: demoActionPanelPaddingTop, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
	return demoHighlight(panel, width, height, 8, props.HighlightTarget == LauncherDemoHighlightActionBackground, props.HighlightColor)
}

const (
	demoActionPanelPaddingTop = float32(10)
	demoActionHeaderHeight    = float32(18)
	demoActionDividerHeight   = float32(16)
	demoActionRowHeight       = float32(38)
	demoActionSearchHeight    = float32(36)
	demoActionCount           = 2
)

// demoActionPanelHeight sizes the overlay to the two demo actions, matching the live panel's row-driven height.
func demoActionPanelHeight() float32 {
	return demoActionPanelPaddingTop + demoActionHeaderHeight + demoActionDividerHeight + float32(demoActionCount)*demoActionRowHeight + demoActionSearchHeight
}

func demoAlpha(opacity float32) uint8 { return demoScaledAlpha(opacity, 255) }

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

func demoHighlight(child woxwidget.Widget, width, height, radius float32, visible bool, color woxui.Color) woxwidget.Widget {
	if !visible {
		return child
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
