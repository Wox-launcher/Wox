package view

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// dropdownIndicator centers Flutter's 10x6 arrow_drop_down geometry in the available icon slot.
func dropdownIndicator(width, height float32, color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		triangleWidth := min(float32(10), bounds.Width)
		triangleHeight := min(float32(6), bounds.Height)
		left := bounds.X + (bounds.Width-triangleWidth)/2
		top := bounds.Y + (bounds.Height-triangleHeight)/2
		displayList.FillConvexPolygon([]woxui.Point{
			{X: left, Y: top},
			{X: left + triangleWidth, Y: top},
			{X: left + triangleWidth/2, Y: top + triangleHeight},
		}, color)
	}}
}

type dropdownTriggerProps struct {
	ID          string
	Value       string
	Trailing    string
	Leading     *woxui.Image
	Width       float32
	Height      float32
	Outline     woxui.Color
	Foreground  woxui.Color
	Secondary   woxui.Color
	OnTap       func()
	OnTapBounds func(woxui.Rect)
}

// woxDropdownTrigger keeps rich and plain selected values aligned in every outlined dropdown.
func woxDropdownTrigger(props dropdownTriggerProps) woxwidget.Widget {
	const horizontalPadding = float32(8)
	const indicatorWidth = float32(24)
	contentWidth := max(float32(0), props.Width-horizontalPadding*2-indicatorWidth)
	children := make([]woxwidget.Widget, 0, 5)
	if props.Leading != nil {
		children = append(children,
			woxwidget.Align{Width: 18, Height: props.Height, Vertical: 0.5, Child: woxwidget.Image{Source: props.Leading, Width: 18, Height: 18}},
			woxwidget.Container{Width: 8, Height: props.Height},
		)
		contentWidth = max(float32(0), contentWidth-26)
	}
	trailingWidth := float32(0)
	if props.Trailing != "" {
		trailingWidth = min(float32(80), max(float32(0), contentWidth-60))
		contentWidth = max(float32(0), contentWidth-trailingWidth-10)
	}
	children = append(children, woxwidget.Align{Width: contentWidth, Height: props.Height, Vertical: 0.5, Child: woxwidget.TextBlock{
		Value: props.Value, Width: contentWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 13}, Color: props.Foreground,
	}})
	if trailingWidth > 0 {
		secondary := props.Secondary
		if secondary.A == 0 {
			secondary = props.Foreground
		}
		children = append(children,
			woxwidget.Container{Width: 10, Height: props.Height},
			woxwidget.Align{Width: trailingWidth, Height: props.Height, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{
				Value: props.Trailing, Style: woxui.TextStyle{Size: 12}, Color: secondary,
			}},
		)
	}
	children = append(children, dropdownIndicator(indicatorWidth, props.Height, props.Foreground))
	return woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, OnTapBounds: props.OnTapBounds, Child: woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: 4, BorderColor: props.Outline, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: horizontalPadding, Right: horizontalPadding},
		Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children},
	}}
}
