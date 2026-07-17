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
	Summary      string
	DefaultLabel string
	Open         bool
	Groups       []RefinementGroup
	OnToggle     func()
}

// RefinementToggleWidth measures the shared query accessory.
func RefinementToggleWidth(props RefinementsProps) float32 {
	metrics, _ := props.Window.MeasureText(props.Summary, woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold})
	return min(float32(150), max(float32(72), metrics.Size.Width+37))
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
	return woxwidget.Gesture{ID: "query-refinements-toggle", OnTap: props.OnToggle, Child: woxwidget.Container{
		Width: width, Height: 34, Padding: woxwidget.Insets{Top: 4}, Child: woxwidget.Container{
			Width: width, Height: 26, Radius: 7, Color: refinementColorWithOpacity(tint, borderOpacity), Padding: woxwidget.UniformInsets(1),
			Child: woxwidget.Container{
				Width: width - 2, Height: 24, Radius: 6, Color: refinementOpaqueOverlay(props.Theme.Background, tint, backgroundOpacity),
				Padding: woxwidget.Insets{Left: 7, Top: 4, Right: 8, Bottom: 3}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, Children: []woxwidget.Widget{
					refinementFilterIcon(refinementColorWithOpacity(tint, 0.92)),
					woxwidget.Text{Value: props.Summary, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: refinementColorWithOpacity(props.Theme.QueryText, textOpacity)},
				}},
			},
		},
	}}
}

// RefinementsView builds the expanded horizontal controls.
func RefinementsView(props RefinementsProps) woxwidget.Widget {
	controls := make([]woxwidget.Widget, 0, len(props.Groups))
	for _, refinement := range props.Groups {
		group := make([]woxwidget.Widget, 0, len(refinement.Options)+2)
		if refinement.Title != "" {
			group = append(group, woxwidget.Container{Height: 22, Padding: woxwidget.Insets{Left: 7, Top: 5, Right: 7, Bottom: 3}, Child: woxwidget.Text{
				Value: refinement.Title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.68),
			}})
			group = append(group, woxwidget.Container{Width: 1, Height: 22, Padding: woxwidget.Insets{Top: 4, Bottom: 4}, Child: woxwidget.Container{
				Width: 1, Height: 14, Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.13),
			}})
		}
		for _, option := range refinement.Options {
			group = append(group, refinementOption(option, props.Theme))
		}
		controls = append(controls, woxwidget.Container{
			Height: 26, Radius: 7, Color: refinementColorWithOpacity(props.Theme.ResultSubtitle, 0.12), Padding: woxwidget.UniformInsets(1),
			Child: woxwidget.Container{
				Height: 24, Radius: 6, Color: refinementOpaqueOverlay(props.Theme.Background, props.Theme.QueryText, 0.035), Padding: woxwidget.Insets{Left: 2, Top: 1, Right: 2, Bottom: 1},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 1, Children: group},
			},
		})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 8, Bottom: 8}, Child: woxwidget.Clip{
		Width: max(float32(0), props.Width-16), Height: 26, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: controls},
	}}
}

func refinementOption(option RefinementOption, theme woxcomponent.Theme) woxwidget.Widget {
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
		children = append(children, woxwidget.Image{Source: option.Icon, Width: 16, Height: 16})
	}
	children = append(children, woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground})
	return woxwidget.Gesture{ID: "refinement-" + option.Value, OnTap: option.OnTap, Child: woxwidget.Container{
		Height: 22, Radius: 5, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 4, Right: 10, Bottom: 3},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: children},
	}}
}

func refinementFilterIcon(color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: 15, Height: 15, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		for index, lineWidth := range []float32{13, 9, 5} {
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 1, Y: bounds.Y + 3 + float32(index)*4, Width: lineWidth, Height: 1.5}, 0.75, color)
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
