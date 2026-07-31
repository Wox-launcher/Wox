package view

import (
	"fmt"

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
	return min(scaledLauncherSize(150, props.DensityScale), max(scaledLauncherSize(72, props.DensityScale), metrics.Size.Width+scaledLauncherSize(37, props.DensityScale)))
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
	contentHeight := scaledLauncherSize(24, props.DensityScale)
	return woxwidget.Gesture{ID: "query-refinements-toggle", OnTap: props.OnToggle, Child: woxwidget.Container{
		Width: width, Height: toggleHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(4, props.DensityScale)}, Child: woxwidget.Container{
			Width: width, Height: controlHeight, Radius: scaledLauncherSize(7, props.DensityScale), Color: refinementColorWithOpacity(tint, borderOpacity), Padding: woxwidget.UniformInsets(1),
			Child: woxwidget.Container{
				Width: width - 2, Height: contentHeight, Radius: scaledLauncherSize(6, props.DensityScale), Color: refinementOpaqueOverlay(props.Theme.Background, tint, backgroundOpacity),
				Padding: woxwidget.Insets{Left: scaledLauncherSize(7, props.DensityScale), Top: scaledLauncherSize(4, props.DensityScale), Right: scaledLauncherSize(8, props.DensityScale), Bottom: scaledLauncherSize(3, props.DensityScale)}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(5, props.DensityScale), Children: []woxwidget.Widget{
					refinementFilterIcon(refinementColorWithOpacity(tint, 0.92), props.DensityScale),
					woxwidget.Text{Value: props.Summary, Style: woxui.TextStyle{Size: scaledLauncherSize(11, props.DensityScale), Weight: woxui.FontWeightSemibold}, Color: refinementColorWithOpacity(props.Theme.QueryText, textOpacity)},
				}},
			},
		},
	}}
}

// RefinementsView builds the expanded horizontal controls.
func RefinementsView(props RefinementsProps) woxwidget.Widget {
	groupHeight := scaledLauncherSize(22, props.DensityScale)
	controlHeight := scaledLauncherSize(26, props.DensityScale)
	contentHeight := scaledLauncherSize(24, props.DensityScale)
	controls := make([]woxwidget.Widget, 0, len(props.Groups))
	for _, refinement := range props.Groups {
		group := make([]woxwidget.Widget, 0, len(refinement.Options)+2)
		if refinement.Title != "" {
			group = append(group, woxwidget.Container{Height: groupHeight, Padding: woxwidget.Insets{Left: scaledLauncherSize(7, props.DensityScale), Top: scaledLauncherSize(5, props.DensityScale), Right: scaledLauncherSize(7, props.DensityScale), Bottom: scaledLauncherSize(3, props.DensityScale)}, Child: woxwidget.Text{
				Value: refinement.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(11, props.DensityScale), Weight: woxui.FontWeightSemibold}, Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.68),
			}})
			group = append(group, woxwidget.Container{Width: 1, Height: groupHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(4, props.DensityScale), Bottom: scaledLauncherSize(4, props.DensityScale)}, Child: woxwidget.Container{
				Width: 1, Height: scaledLauncherSize(14, props.DensityScale), Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.13),
			}})
		}
		for _, option := range refinement.Options {
			group = append(group, refinementOption(option, props.Theme, props.DensityScale))
		}
		controls = append(controls, woxwidget.Container{
			Height: controlHeight, Radius: scaledLauncherSize(7, props.DensityScale), Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.12), Padding: woxwidget.UniformInsets(1),
			Child: woxwidget.Container{
				Height: contentHeight, Radius: scaledLauncherSize(6, props.DensityScale), Color: refinementOpaqueOverlay(props.Theme.Background, props.Theme.QueryText, 0.035), Padding: woxwidget.Insets{Left: scaledLauncherSize(2, props.DensityScale), Top: 1, Right: scaledLauncherSize(2, props.DensityScale), Bottom: 1},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 1, Children: group},
			},
		})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: scaledLauncherSize(8, props.DensityScale), Top: scaledLauncherSize(10, props.DensityScale), Right: scaledLauncherSize(8, props.DensityScale), Bottom: scaledLauncherSize(8, props.DensityScale)}, Child: woxwidget.Clip{
		Width: max(float32(0), props.Width-scaledLauncherSize(16, props.DensityScale)), Height: controlHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(10, props.DensityScale), Children: controls},
	}}
}

func refinementOption(option RefinementOption, theme woxcomponent.Theme, densityScale float32) woxwidget.Widget {
	background := woxui.Color{}
	foreground := refinementColorWithOpacity(theme.QueryText, 0.82)
	if option.Selected {
		background = refinementColorWithOpacity(theme.SelectedBackground, 0.22)
		foreground = theme.QueryText
	}
	label := option.Label
	if label == "" {
		label = option.Value
	}
	if option.Count != nil {
		label = fmt.Sprintf("%s (%d)", label, *option.Count)
	}
	children := make([]woxwidget.Widget, 0, 2)
	if option.Icon != nil {
		iconSize := scaledLauncherSize(16, densityScale)
		children = append(children, woxwidget.Image{Source: option.Icon, Width: iconSize, Height: iconSize})
	}
	children = append(children, woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: scaledLauncherSize(12, densityScale), Weight: woxui.FontWeightSemibold}, Color: foreground})
	return woxwidget.Gesture{ID: "refinement-" + option.Value, OnTap: option.OnTap, Child: woxwidget.Container{
		Height: scaledLauncherSize(22, densityScale), Radius: scaledLauncherSize(5, densityScale), Color: background, Padding: woxwidget.Insets{Left: scaledLauncherSize(10, densityScale), Top: scaledLauncherSize(4, densityScale), Right: scaledLauncherSize(10, densityScale), Bottom: scaledLauncherSize(3, densityScale)},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(6, densityScale), Children: children},
	}}
}

func refinementFilterIcon(color woxui.Color, densityScale float32) woxwidget.Widget {
	return woxwidget.Painter{Width: scaledLauncherSize(15, densityScale), Height: scaledLauncherSize(15, densityScale), Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		for index, lineWidth := range []float32{13, 9, 5} {
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + scaledLauncherSize(1, densityScale), Y: bounds.Y + scaledLauncherSize(3+float32(index)*4, densityScale), Width: scaledLauncherSize(lineWidth, densityScale), Height: scaledLauncherSize(1.5, densityScale)}, scaledLauncherSize(0.75, densityScale), color)
		}
	}}
}

func refinementColorWithOpacity(color woxui.Color, opacity float32) woxui.Color {
	color.A = uint8(float32(color.A)*min(max(opacity, float32(0)), float32(1)) + 0.5)
	return color
}

func refinementOpaqueOverlay(background, foreground woxui.Color, opacity float32) woxui.Color {
	opacity = min(max(opacity, float32(0)), float32(1))
	return woxui.Color{
		R: uint8(float32(background.R)*(1-opacity) + float32(foreground.R)*opacity + 0.5),
		G: uint8(float32(background.G)*(1-opacity) + float32(foreground.G)*opacity + 0.5),
		B: uint8(float32(background.B)*(1-opacity) + float32(foreground.B)*opacity + 0.5),
		A: 255,
	}
}
