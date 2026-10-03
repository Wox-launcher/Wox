package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestSliderTrackUsesLogicalTravel verifies endpoints and step snapping independently of display scale.
func TestSliderTrackUsesLogicalTravel(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.25, 1.5, 2} {
		props := SliderTrackProps{Width: 112 * scale, Height: 42 * scale, Min: 12, Max: 48, Step: 2, Scale: scale}
		for _, sample := range []struct{ x, want float32 }{{-50, 12}, {8, 12}, {35, 22}, {56, 30}, {104, 48}, {180, 48}} {
			if value := props.ValueAt(sample.x * scale); value != sample.want {
				t.Fatalf("scale=%v x=%v: got %v, want %v", scale, sample.x, value, sample.want)
			}
		}
	}
}

// TestSliderTrackStatesKeepTheRailStable checks visual feedback without moving the value or changing the hit target.
func TestSliderTrackStatesKeepTheRailStable(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for _, foreground := range []woxui.Color{{A: 255}, {R: 255, G: 255, B: 255, A: 255}} {
			props := SliderTrackProps{Width: 112 * scale, Height: 42 * scale, Min: 12, Max: 48, Value: 30, Scale: scale,
				Theme: ControlTheme{Text: foreground, Border: woxui.Color{A: 60}, Accent: woxui.Color{G: 255, A: 255}, Focus: woxui.Color{G: 255, A: 255}}}
			bounds := woxui.Rect{X: -120, Y: 30, Width: props.Width, Height: props.Height}
			idle, hover, drag, focus := &woxui.DisplayList{}, &woxui.DisplayList{}, &woxui.DisplayList{}, &woxui.DisplayList{}
			woxwidget.PaintStateless(nil, WoxSliderTrack(props), idle, bounds)
			props.Hovered = true
			woxwidget.PaintStateless(nil, WoxSliderTrack(props), hover, bounds)
			props.Active = true
			woxwidget.PaintStateless(nil, WoxSliderTrack(props), drag, bounds)
			props.Focused = true
			woxwidget.PaintStateless(nil, WoxSliderTrack(props), focus, bounds)
			if idle.Compare(hover) == nil || hover.Compare(drag) == nil || drag.Compare(focus) == nil {
				t.Fatalf("scale %v: slider states lack distinct feedback", scale)
			}
			if value := props.ValueAt(props.Width / 2); value != 30 {
				t.Fatalf("state changed the value's rail position: %v", value)
			}
		}
	}
}
