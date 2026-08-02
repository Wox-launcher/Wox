package view

import (
	"fmt"
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
	OnTap    func()
}

// RefinementGroup contains one titled set of query controls.
type RefinementGroup struct {
	Title   string
	Hotkey  string
	Options []RefinementOption
}

// RefinementsProps contains the query refinement presentation state.
type RefinementsProps struct {
	Width        float32
	Height       float32
	Theme        woxcomponent.Theme
	Window       *woxui.Window
	DensityScale float32
	Summary      string
	DefaultLabel string
	Open         bool
	Groups       []RefinementGroup
	OnToggle     func()
}

// RefinementToggleWidth measures the shared query accessory.
func RefinementToggleWidth(props RefinementsProps) float32 {
	metrics, _ := props.Window.MeasureText(props.Summary, woxui.TextStyle{Size: scaledLauncherSize(11, props.DensityScale), Weight: woxui.FontWeightSemibold})
	return min(scaledLauncherSize(150, props.DensityScale), metrics.Size.Width+scaledLauncherSize(37, props.DensityScale))
}

// RefinementToggle builds the compact query accessory.
func RefinementToggle(props RefinementsProps) woxwidget.Widget {
	active := props.Open || props.Summary != props.DefaultLabel
	tint := props.Theme.QueryText
	backgroundOpacity := float32(0.075)
	borderOpacity := float32(0.13)
	textOpacity := float32(0.72)
	if active {
		tint = props.Theme.Cursor
		backgroundOpacity = 0.15
		borderOpacity = 0.32
		textOpacity = 0.94
	}
	width := RefinementToggleWidth(props)
	toggleHeight := scaledLauncherSize(34, props.DensityScale)
	controlHeight := scaledLauncherSize(26, props.DensityScale)
	return woxwidget.Gesture{ID: "query-refinements-toggle", OnTap: props.OnToggle, Child: woxwidget.Container{
		Width: width, Height: toggleHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(4, props.DensityScale)}, Child: woxwidget.Container{
			Width: width, Height: controlHeight, Radius: scaledLauncherSize(7, props.DensityScale), Color: refinementColorWithOpacity(tint, backgroundOpacity),
			BorderColor: refinementColorWithOpacity(tint, borderOpacity), BorderWidth: 1,
			Padding: woxwidget.Insets{Left: scaledLauncherSize(8, props.DensityScale), Right: scaledLauncherSize(9, props.DensityScale)}, Child: woxwidget.Align{Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{
				Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(5, props.DensityScale), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
					refinementFilterIcon(refinementColorWithOpacity(tint, 0.92), props.DensityScale),
					woxwidget.Text{Value: props.Summary, Style: woxui.TextStyle{Size: scaledLauncherSize(11, props.DensityScale), Weight: woxui.FontWeightSemibold}, Color: refinementColorWithOpacity(props.Theme.QueryText, textOpacity)},
				},
			}},
		},
	}}
}

// RefinementsView builds the expanded horizontal controls.
func RefinementsView(props RefinementsProps) woxwidget.Widget {
	groupHeight := scaledLauncherSize(22, props.DensityScale)
	controlHeight := scaledLauncherSize(26, props.DensityScale)
	controls := make([]woxwidget.Widget, 0, len(props.Groups))
	contentWidth := float32(0)
	for _, refinement := range props.Groups {
		group := make([]woxwidget.Widget, 0, len(refinement.Options)+8)
		groupWidth := float32(0)
		if refinement.Title != "" {
			titleStyle := woxui.TextStyle{Size: scaledLauncherSize(11, props.DensityScale), Weight: woxui.FontWeightSemibold}
			titleWidth := refinementTextWidth(props.Window, refinement.Title, titleStyle) + scaledLauncherSize(14, props.DensityScale)
			group = append(group, woxwidget.Container{Width: titleWidth, Height: groupHeight, Padding: woxwidget.Insets{Left: scaledLauncherSize(7, props.DensityScale), Right: scaledLauncherSize(7, props.DensityScale)}, Child: woxwidget.Align{Vertical: 0.5, Child: woxwidget.Text{
				Value: refinement.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(11, props.DensityScale), Weight: woxui.FontWeightSemibold}, Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.68),
			}}})
			group = append(group,
				woxwidget.Container{Width: 1, Height: scaledLauncherSize(14, props.DensityScale), Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.13)},
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
			hotkeyStyle := woxui.TextStyle{Size: scaledLauncherSize(11, props.DensityScale), Weight: woxui.FontWeightSemibold}
			hotkeyWidth := refinementTextWidth(props.Window, refinement.Hotkey, hotkeyStyle)
			leadingGap := scaledLauncherSize(7, props.DensityScale)
			trailingGap := scaledLauncherSize(4, props.DensityScale)
			group = append(group,
				woxwidget.Painter{Width: leadingGap, Height: groupHeight},
				woxwidget.Container{Width: 1, Height: scaledLauncherSize(14, props.DensityScale), Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.11)},
				woxwidget.Painter{Width: leadingGap, Height: groupHeight},
				woxwidget.Text{Value: refinement.Hotkey, Style: hotkeyStyle, Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.58)},
				woxwidget.Painter{Width: trailingGap, Height: groupHeight},
			)
			groupWidth += leadingGap + 1 + leadingGap + hotkeyWidth + trailingGap
		}
		shellPadding := scaledLauncherSize(3, props.DensityScale)
		shellWidth := groupWidth + shellPadding*2
		controls = append(controls, woxwidget.Container{
			Width: shellWidth, Height: controlHeight, Radius: scaledLauncherSize(7, props.DensityScale), Color: refinementColorWithOpacity(props.Theme.ResultTitle, 0.035),
			BorderColor: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.12), BorderWidth: 1,
			Padding: woxwidget.Insets{Left: shellPadding, Top: scaledLauncherSize(2, props.DensityScale), Right: shellPadding, Bottom: scaledLauncherSize(2, props.DensityScale)},
			Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: group},
		})
		if len(controls) > 1 {
			contentWidth += scaledLauncherSize(10, props.DensityScale)
		}
		contentWidth += shellWidth
	}
	viewportWidth := max(float32(0), props.Width-scaledLauncherSize(16, props.DensityScale))
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: scaledLauncherSize(8, props.DensityScale), Top: scaledLauncherSize(10, props.DensityScale), Right: scaledLauncherSize(8, props.DensityScale), Bottom: scaledLauncherSize(8, props.DensityScale)}, Child: woxwidget.ScrollView{
		Width: viewportWidth, Height: controlHeight, ContentWidth: max(viewportWidth, contentWidth), Horizontal: true,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(10, props.DensityScale), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: controls},
	}}
}

func refinementOption(option RefinementOption, theme woxcomponent.Theme, window *woxui.Window, densityScale float32) (woxwidget.Widget, float32) {
	background := woxui.Color{}
	foreground := refinementColorWithOpacity(theme.ResultTitle, 0.82)
	if option.Selected {
		background = refinementColorWithOpacity(theme.ActionSelected, 0.22)
		foreground = theme.ResultTitle
	}
	label := option.Label
	if label == "" {
		label = option.Value
	}
	if option.Count != nil {
		label = fmt.Sprintf("%s (%d)", label, *option.Count)
	}
	style := woxui.TextStyle{Size: scaledLauncherSize(11, densityScale), Weight: woxui.FontWeightSemibold}
	contentWidth := refinementTextWidth(window, label, style)
	children := make([]woxwidget.Widget, 0, 2)
	if option.Icon != nil {
		iconSize := scaledLauncherSize(16, densityScale)
		children = append(children, woxwidget.Image{Source: option.Icon, Width: iconSize, Height: iconSize})
		contentWidth += iconSize + scaledLauncherSize(5, densityScale)
	}
	children = append(children, woxwidget.Text{Value: label, Style: style, Color: foreground})
	optionWidth := min(scaledLauncherSize(118, densityScale), contentWidth+scaledLauncherSize(20, densityScale))
	return woxwidget.Gesture{ID: "refinement-" + option.Value, OnTap: option.OnTap, Child: woxwidget.Container{
		Width: optionWidth, Height: scaledLauncherSize(22, densityScale), Radius: scaledLauncherSize(5, densityScale), Color: background, Padding: woxwidget.Insets{Left: scaledLauncherSize(10, densityScale), Right: scaledLauncherSize(10, densityScale)},
		Child: woxwidget.Align{Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(5, densityScale), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}},
	}}, optionWidth
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

func refinementFilterIcon(color woxui.Color, densityScale float32) woxwidget.Widget {
	return woxcomponent.FilterListGlyph(scaledLauncherSize(15, densityScale), color)
}

func refinementColorWithOpacity(color woxui.Color, opacity float32) woxui.Color {
	color.A = uint8(min(max(opacity, float32(0)), float32(1))*255 + 0.5)
	return color
}
