package view

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// RefinementOption contains resolved presentation data for one query refinement value.
type RefinementOption struct {
	Value    string
	Label    string
	Count    *int
	Icon     *woxui.Image
	Selected bool
	OnTap    func() `boundary:"stable"`
}

// Equal compares every visual dependency for one refinement option.
func (o RefinementOption) Equal(other RefinementOption) bool {
	if o.Value != other.Value || o.Label != other.Label || o.Icon != other.Icon || o.Selected != other.Selected {
		return false
	}
	if o.Count == nil || other.Count == nil {
		return o.Count == nil && other.Count == nil
	}
	return *o.Count == *other.Count
}

// RefinementGroup contains one titled set of query controls.
type RefinementGroup struct {
	ID       string
	Title    string
	Hotkey   string
	Options  []RefinementOption
	Current  string
	Expanded bool
	OnTap    func()                    `boundary:"stable"`
	OnKey    func(woxui.KeyEvent) bool `boundary:"stable"`
}

// Equal compares every visual dependency for one refinement group.
func (g RefinementGroup) Equal(other RefinementGroup) bool {
	if g.ID != other.ID || g.Title != other.Title || g.Hotkey != other.Hotkey || g.Current != other.Current || g.Expanded != other.Expanded || len(g.Options) != len(other.Options) {
		return false
	}
	for index := range g.Options {
		if !g.Options[index].Equal(other.Options[index]) {
			return false
		}
	}
	return true
}

// RefinementsProps contains the query refinement presentation state.
type RefinementsProps struct {
	Revision      uint64
	Width         float32
	Height        float32
	Theme         woxcomponent.Theme
	Window        *woxui.Window
	DensityScale  float32
	SummaryLabels []string
	DefaultLabel  string
	Open          bool
	Tooltip       string
	Groups        []RefinementGroup
	PickerID      string
	DefaultButton *RefinementDefaultButtonProps
	OnToggle      func()                         `boundary:"stable"`
	OnTooltip     func(bool, string, woxui.Rect) `boundary:"stable"`
}

// Equal compares every render dependency for the expanded refinement section.
func (p RefinementsProps) Equal(other RefinementsProps) bool {
	if p.Revision != other.Revision || p.Width != other.Width || p.Height != other.Height || p.Theme != other.Theme || p.Window != other.Window || p.DensityScale != other.DensityScale || !slices.Equal(p.SummaryLabels, other.SummaryLabels) || p.DefaultLabel != other.DefaultLabel || p.Open != other.Open || p.Tooltip != other.Tooltip || p.PickerID != other.PickerID || !p.DefaultButton.Equal(other.DefaultButton) || len(p.Groups) != len(other.Groups) {
		return false
	}
	for groupIndex := range p.Groups {
		if !p.Groups[groupIndex].Equal(other.Groups[groupIndex]) {
			return false
		}
	}
	return true
}

// RefinementsBoundary retains the expanded refinement section while its prepared props are unchanged.
func RefinementsBoundary(props RefinementsProps) woxwidget.Widget {
	return woxwidget.Boundary[RefinementsProps]{
		Key: "launcher-refinements-boundary", Label: "refinements", Props: props,
		Build: func(props RefinementsProps) woxwidget.Widget { return RefinementsView(props) },
	}
}

// RefinementToggleBoundary retains the compact query accessory independently from the expanded bar.
func RefinementToggleBoundary(props RefinementsProps) woxwidget.Widget {
	return woxwidget.Boundary[RefinementsProps]{
		Key: "launcher-refinement-toggle-boundary", Label: "header:refinement", Props: props,
		Build: func(props RefinementsProps) woxwidget.Widget { return RefinementToggle(props) },
	}
}

// RefinementToggleWidth measures the shared query accessory.
func RefinementToggleWidth(props RefinementsProps) float32 {
	padding, gap, iconSize := refinementToggleHorizontalMetrics(props.DensityScale)
	style := refinementSummaryStyle(props.DensityScale)
	textWidth := refinementTextWidth(props.Window, props.DefaultLabel, style)
	if len(props.SummaryLabels) > 0 {
		textWidth = float32(len(props.SummaryLabels)-1) * refinementSummarySeparatorWidth(props, style)
		for _, label := range props.SummaryLabels {
			textWidth += refinementTextWidth(props.Window, label, style)
		}
	}
	return min(scaledLauncherSize(192, props.DensityScale), textWidth+padding.Left+padding.Right+gap+iconSize)
}

func refinementToggleHorizontalMetrics(densityScale float32) (padding woxwidget.Insets, gap, iconSize float32) {
	padding = woxwidget.Insets{Left: scaledLauncherSize(8, densityScale), Right: scaledLauncherSize(9, densityScale)}
	return padding, scaledLauncherSize(5, densityScale), scaledLauncherSize(15, densityScale)
}

// RefinementToggle builds the compact query accessory.
func RefinementToggle(props RefinementsProps) woxwidget.Widget {
	active := props.Open || len(props.SummaryLabels) > 0
	width := RefinementToggleWidth(props)
	toggleHeight := scaledLauncherSize(34, props.DensityScale)
	controlHeight := scaledLauncherSize(26, props.DensityScale)
	padding, gap, iconSize := refinementToggleHorizontalMetrics(props.DensityScale)
	tooltip := props.Tooltip
	if len(props.SummaryLabels) > 0 {
		tooltip = strings.Join(props.SummaryLabels, ", ") + "\n" + tooltip
	}
	return woxcomponent.Hoverable(woxwidget.Key("query-refinements-toggle"), false, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		foreground, icon, toggleBackground, border := props.Theme.RefinementButtonColors(active, hovered)
		return woxwidget.Gesture{ID: "query-refinements-toggle", OnTap: props.OnToggle, OnHoverAt: func(inside bool, bounds woxui.Rect) {
			onHoverAt(inside, bounds)
			if props.OnTooltip != nil {
				props.OnTooltip(inside, tooltip, bounds)
			}
		}, Child: woxwidget.Container{
			Width: width, Height: toggleHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(4, props.DensityScale)}, Child: woxwidget.Container{
				Width: width, Height: controlHeight, Radius: scaledLauncherSize(7, props.DensityScale), Color: toggleBackground,
				BorderColor: border, BorderWidth: 1,
				Padding: padding, Child: woxwidget.Align{Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{
					Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
						woxcomponent.FilterListGlyph(iconSize, icon),
						woxwidget.Flexible{Child: refinementSummaryView(props, foreground, controlHeight)},
					},
				}},
			},
		}}
	})
}

func refinementSummaryStyle(scale float32) woxui.TextStyle {
	return woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.TailFontSize, scale), Weight: woxui.FontWeightSemibold}
}

func refinementSummarySeparatorWidth(props RefinementsProps, style woxui.TextStyle) float32 {
	return refinementTextWidth(props.Window, ",", style) + scaledLauncherSize(3, props.DensityScale)
}

// refinementSummaryView truncates individual values within the space left by the filter icon.
func refinementSummaryView(props RefinementsProps, color woxui.Color, height float32) woxwidget.Widget {
	style := refinementSummaryStyle(props.DensityScale)
	text := woxwidget.TextBlock{Height: height, LineHeight: height, MaxLines: 1, HardEllipsis: true, AlignmentY: 0.5, Style: style, Color: color}
	switch len(props.SummaryLabels) {
	case 0:
		text.Value = props.DefaultLabel
		return text
	case 1:
		text.Value = props.SummaryLabels[0]
		return text
	}

	separatorWidth := refinementSummarySeparatorWidth(props, style)
	return woxwidget.LayoutBuilder{Build: func(size woxui.Size) woxwidget.Widget {
		labels := props.SummaryLabels
		widths := make([]float32, len(labels))
		total := float32(len(labels)-1) * separatorWidth
		for i, label := range labels {
			widths[i] = refinementTextWidth(props.Window, label, style)
			total += widths[i]
		}
		if total > size.Width {
			minimumLabelWidth := refinementTextWidth(props.Window, "W…", style)
			capacity := max(1, int((size.Width+separatorWidth)/(minimumLabelWidth+separatorWidth)))
			if len(labels) > capacity {
				visible := capacity - 1
				more := fmt.Sprintf("+%d", len(labels)-visible)
				labels = append(slices.Clone(labels[:visible]), more)
				widths = append(widths[:visible], refinementTextWidth(props.Window, more, style))
			}
		}
		labelBudget := max(0, size.Width-float32(len(labels)-1)*separatorWidth)
		limit := refinementSummaryLabelLimit(widths, labelBudget)
		children := make([]woxwidget.Widget, 0, 2*len(labels)-1)
		for i, label := range labels {
			if i > 0 {
				separator := text
				separator.Value, separator.Width = ",", separatorWidth
				children = append(children, separator)
			}
			if width := min(widths[i], limit); width > 0 {
				part := text
				part.Value, part.Width = label, width
				part.ShrinkWrap = true
				children = append(children, part)
			}
		}
		return woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}
	}}
}

// refinementSummaryLabelLimit shares width among long values after reserving space for short ones.
func refinementSummaryLabelLimit(widths []float32, budget float32) float32 {
	sorted := slices.Clone(widths)
	slices.Sort(sorted)
	for i, width := range sorted {
		share := budget / float32(len(sorted)-i)
		if width >= share {
			return share
		}
		budget -= width
	}
	return sorted[len(sorted)-1]
}

// RefinementsView builds the expanded horizontal controls.
func RefinementsView(props RefinementsProps) woxwidget.Widget {
	groupHeight := scaledLauncherSize(22, props.DensityScale)
	controlHeight := scaledLauncherSize(26, props.DensityScale)
	gap := scaledLauncherSize(10, props.DensityScale)
	controls := make([]woxwidget.Widget, 0, len(props.Groups))
	contentWidth := float32(0)
	for _, refinement := range props.Groups {
		if len(controls) > 0 {
			contentWidth += gap
		}
		if refinement.ID != "" {
			control, width := refinementDropdown(refinement, props, controlHeight)
			controls = append(controls, control)
			contentWidth += width
			continue
		}
		group := make([]woxwidget.Widget, 0, len(refinement.Options)+8)
		groupWidth := float32(0)
		if refinement.Title != "" {
			titleStyle := woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.TailFontSize, props.DensityScale), Weight: woxui.FontWeightSemibold}
			titleWidth := refinementTextWidth(props.Window, refinement.Title, titleStyle) + scaledLauncherSize(14, props.DensityScale)
			group = append(group, woxwidget.Container{Width: titleWidth, Height: groupHeight, Padding: woxwidget.Insets{Left: scaledLauncherSize(7, props.DensityScale), Right: scaledLauncherSize(7, props.DensityScale)}, Child: woxwidget.Align{Vertical: 0.5, Child: woxwidget.Text{
				Value: refinement.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.TailFontSize, props.DensityScale), Weight: woxui.FontWeightSemibold}, Color: refinementThemeColor(props.Theme.RefinementTitleColor, refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.68)),
			}}})
			group = append(group,
				woxwidget.Container{Width: 1, Height: scaledLauncherSize(14, props.DensityScale), Color: refinementThemeColor(props.Theme.RefinementDividerColor, refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.13))},
				woxwidget.Painter{Width: scaledLauncherSize(3, props.DensityScale), Height: groupHeight},
			)
			groupWidth += titleWidth + 1 + scaledLauncherSize(3, props.DensityScale)
		}
		for index, option := range refinement.Options {
			if index > 0 {
				gap := scaledLauncherSize(1, props.DensityScale)
				group = append(group, woxwidget.Painter{Width: gap, Height: groupHeight})
				groupWidth += gap
			}
			optionView, optionWidth := refinementOption(option, props.Theme, props.Window, props.DensityScale)
			group = append(group, optionView)
			groupWidth += optionWidth
		}
		if refinement.Hotkey != "" {
			hotkeyStyle := woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.TailFontSize, props.DensityScale), Weight: woxui.FontWeightSemibold}
			hotkeyWidth := refinementTextWidth(props.Window, refinement.Hotkey, hotkeyStyle)
			leadingGap := scaledLauncherSize(7, props.DensityScale)
			trailingGap := scaledLauncherSize(4, props.DensityScale)
			group = append(group,
				woxwidget.Painter{Width: leadingGap, Height: groupHeight},
				woxwidget.Container{Width: 1, Height: scaledLauncherSize(14, props.DensityScale), Color: refinementThemeColor(props.Theme.RefinementDividerColor, refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.11))},
				woxwidget.Painter{Width: leadingGap, Height: groupHeight},
				woxwidget.Text{Value: refinement.Hotkey, Style: hotkeyStyle, Color: refinementThemeColor(props.Theme.RefinementHotkeyColor, refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.58))},
				woxwidget.Painter{Width: trailingGap, Height: groupHeight},
			)
			groupWidth += leadingGap + 1 + leadingGap + hotkeyWidth + trailingGap
		}
		shellPadding := scaledLauncherSize(3, props.DensityScale)
		shellWidth := groupWidth + shellPadding*2
		controls = append(controls, woxwidget.Container{
			Width: shellWidth, Height: controlHeight, Radius: scaledLauncherSize(7, props.DensityScale), Color: refinementThemeColor(props.Theme.RefinementBackgroundColor, refinementColorWithOpacity(props.Theme.ResultTitle, 0.035)),
			BorderColor: refinementThemeColor(props.Theme.RefinementBorderColor, refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.12)), BorderWidth: 1,
			Padding: woxwidget.Insets{Left: shellPadding, Top: scaledLauncherSize(2, props.DensityScale), Right: shellPadding, Bottom: scaledLauncherSize(2, props.DensityScale)},
			Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: group},
		})
		contentWidth += shellWidth
	}
	viewportWidth := max(float32(0), props.Width-scaledLauncherSize(16, props.DensityScale))
	var keepVisible woxwidget.Key
	if props.PickerID != "" {
		keepVisible = RefinementGroupKey(props.PickerID)
	}
	scroll := woxcomponent.ScrollViewProps{
		Key: "launcher-refinements-scroll", Width: viewportWidth, FillWidth: props.DefaultButton != nil, Height: controlHeight, ContentWidth: contentWidth,
		Horizontal: true, Theme: props.Theme.Controls, ThumbColor: props.Theme.ResultTitle, KeepVisibleKey: keepVisible,
		Content: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: controls},
	}
	content := woxcomponent.WoxScrollView(scroll)
	if props.DefaultButton != nil {
		content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Expanded{Child: content}, refinementDefaultButton(*props.DefaultButton, props, controlHeight),
		}}
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: scaledLauncherSize(8, props.DensityScale), Top: gap, Right: scaledLauncherSize(8, props.DensityScale), Bottom: scaledLauncherSize(8, props.DensityScale)}, Child: content}
}

func refinementOption(option RefinementOption, theme woxcomponent.Theme, window *woxui.Window, densityScale float32) (woxwidget.Widget, float32) {
	background := woxui.Color{}
	foreground := refinementColorWithOpacity(theme.ResultTitle, 0.82)
	if option.Selected {
		background = refinementColorWithOpacity(theme.ActionSelected, 0.22)
		foreground = theme.ResultTitle
	}

	backgroundOverride, foregroundOverride, hoverOverride := theme.RefinementItemBackgroundColor, theme.RefinementItemFontColor, theme.RefinementItemHoverBackgroundColor
	if option.Selected {
		backgroundOverride, foregroundOverride, hoverOverride = theme.RefinementItemActiveBackgroundColor, theme.RefinementItemActiveFontColor, theme.RefinementItemActiveHoverBackgroundColor
	}
	background = refinementThemeColor(backgroundOverride, background)
	foreground = refinementThemeColor(foregroundOverride, foreground)
	label := option.Label
	if label == "" {
		label = option.Value
	}
	if option.Count != nil {
		label = fmt.Sprintf("%s (%d)", label, *option.Count)
	}
	style := woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.TailFontSize, densityScale), Weight: woxui.FontWeightSemibold}
	contentWidth := refinementTextWidth(window, label, style)
	children := make([]woxwidget.Widget, 0, 2)
	if option.Icon != nil {
		iconSize := scaledLauncherSize(16, densityScale)
		children = append(children, woxwidget.Image{Source: option.Icon, Width: iconSize, Height: iconSize})
		contentWidth += iconSize + scaledLauncherSize(5, densityScale)
	}
	children = append(children, woxwidget.Text{Value: label, Style: style, Color: foreground})
	optionWidth := min(scaledLauncherSize(118, densityScale), contentWidth+scaledLauncherSize(20, densityScale))
	content := woxcomponent.Hoverable(woxwidget.Key("refinement-"+option.Value), false, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		optionBackground := background
		if hovered {
			optionBackground = refinementThemeColor(hoverOverride, woxcomponent.ControlHoverColor(background, foreground))
		}
		return woxwidget.Gesture{ID: "refinement-" + option.Value, OnTap: option.OnTap, OnHoverAt: onHoverAt, Child: woxwidget.Container{
			Width: optionWidth, Height: scaledLauncherSize(22, densityScale), Radius: scaledLauncherSize(5, densityScale), Color: optionBackground, Padding: woxwidget.Insets{Left: scaledLauncherSize(10, densityScale), Right: scaledLauncherSize(10, densityScale)},
			Child: woxwidget.Align{Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(5, densityScale), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}},
		}}
	})
	return content, optionWidth
}

// refinementTextWidth measures shrink-wrapped filter labels and keeps pure view tests deterministic without a native window.
func refinementTextWidth(window *woxui.Window, value string, style woxui.TextStyle) float32 {
	if window != nil {
		if metrics, err := window.MeasureText(value, style); err == nil {
			return metrics.Size.Width
		}
	}
	return float32(utf8.RuneCountInString(value)) * style.Size * 0.62
}

func refinementColorWithOpacity(color woxui.Color, opacity float32) woxui.Color {
	color.A = uint8(min(max(opacity, float32(0)), float32(1))*255 + 0.5)
	return color
}

func refinementThemeColor(authored *woxui.Color, fallback woxui.Color) woxui.Color {
	if authored != nil {
		return *authored
	}
	return fallback
}
