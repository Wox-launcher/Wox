package component

import (
	"math"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SliderTrackProps lets retained and immediate-mode surfaces share range geometry and pointer states.
// The owning surface routes input and accessibility so dragging need not steal text-input focus.
type SliderTrackProps struct {
	Width, Height   float32
	Min, Max, Step  float32
	Value, Scale    float32
	Hovered, Active bool
	Focused         bool
	Theme           ControlTheme
}

// ValueAt maps local logical coordinates to a clamped step on the thumb's travel range.
func (props SliderTrackProps) ValueAt(x float32) float32 {
	scale := props.Scale
	if scale <= 0 {
		scale = 1
	}
	inset := 8 * scale
	travel := props.Width - 2*inset
	if travel <= 0 || props.Max <= props.Min {
		return props.Min
	}
	ratio := min(max(float32(0), (x-inset)/travel), 1)
	value := props.Min + ratio*(props.Max-props.Min)
	if props.Step > 0 {
		value = props.Min + float32(math.Round(float64((value-props.Min)/props.Step)))*props.Step
	}
	return min(max(props.Min, value), props.Max)
}

// WoxSliderTrack paints a fixed-size rail with distinct hover, drag, and keyboard-focus affordances.
func WoxSliderTrack(props SliderTrackProps) woxwidget.Widget {
	return woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(list *woxui.DisplayList, bounds woxui.Rect) {
		scale := props.Scale
		if scale <= 0 {
			scale = 1
		}
		inset := 8 * scale
		travel := max(float32(0), bounds.Width-2*inset)
		ratio := float32(0)
		if props.Max > props.Min {
			ratio = min(max(float32(0), (props.Value-props.Min)/(props.Max-props.Min)), 1)
		}
		center := woxui.Point{X: bounds.X + inset + ratio*travel, Y: bounds.Y + bounds.Height/2}
		track := woxui.Rect{X: bounds.X + inset, Y: center.Y - 2*scale, Width: travel, Height: 4 * scale}
		list.FillRoundedRect(track, 2*scale, props.Theme.Border)
		track.Width *= ratio
		list.FillRoundedRect(track, 2*scale, props.Theme.Accent)
		radius := 6 * scale
		if props.Hovered || props.Active {
			radius = 8 * scale
		}
		list.FillRoundedRect(woxui.Rect{X: center.X - radius, Y: center.Y - radius, Width: 2 * radius, Height: 2 * radius}, radius, props.Theme.Text)
		if props.Active {
			list.FillRoundedRect(woxui.Rect{X: center.X - 3*scale, Y: center.Y - 3*scale, Width: 6 * scale, Height: 6 * scale}, 3*scale, props.Theme.Accent)
		}
		if props.Focused {
			list.StrokeRoundedRect(bounds, 6*scale, scale, props.Theme.Focus)
		}
	}}
}
