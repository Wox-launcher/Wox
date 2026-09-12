package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ResultIndicatorStyle carries immutable selection chrome through retained boundaries.
type ResultIndicatorStyle struct {
	Width, Left, Top, Bottom, Radius float32
	Color                            woxui.Color
}

// ResultIndicator resolves v2 geometry while preserving the v1 edge marker.
func (t Theme) ResultIndicator() ResultIndicatorStyle {
	s := ResultIndicatorStyle{Width: max(float32(0), t.SelectedBorderLeftWidth), Color: t.SelectedBorderLeftColor}
	for _, pair := range []struct {
		source *int
		target *float32
	}{
		{t.ResultItemActiveIndicatorWidth, &s.Width}, {t.ResultItemActiveIndicatorInsetLeft, &s.Left}, {t.ResultItemActiveIndicatorInsetTop, &s.Top}, {t.ResultItemActiveIndicatorInsetBottom, &s.Bottom}, {t.ResultItemActiveIndicatorBorderRadius, &s.Radius},
	} {
		if pair.source != nil {
			*pair.target = max(float32(0), float32(*pair.source))
		}
	}
	if t.ResultItemActiveIndicatorColor != nil {
		s.Color = *t.ResultItemActiveIndicatorColor
	}
	return s
}

// ResultIndicatorBackground shares the marker geometry between launcher rows and the demo.
func ResultIndicatorBackground(width, height, radius float32, color woxui.Color, s ResultIndicatorStyle) woxwidget.Widget {
	base := woxwidget.Container{Width: width, Height: height, Radius: radius, Color: color}
	if s.Left == 0 && s.Top == 0 && s.Bottom == 0 && s.Radius == 0 {
		base.LeftBorderWidth, base.LeftBorderColor = s.Width, s.Color
		return base
	}
	if s.Width <= 0 || s.Color.A == 0 {
		return base
	}
	left, top := min(s.Left, max(float32(0), width)), min(s.Top, max(float32(0), height))
	markerWidth, markerHeight := min(s.Width, max(float32(0), width-left)), max(float32(0), height-top-s.Bottom)
	if markerWidth <= 0 || markerHeight <= 0 {
		return base
	}
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: base}, {Left: left, Top: top, Child: woxwidget.Container{Width: markerWidth, Height: markerHeight, Radius: min(s.Radius, min(markerWidth, markerHeight)/2), Color: s.Color}},
	}}
}

// QueryBottomBorder resolves the optional inset query outline without affecting layout.
func (t Theme) QueryBottomBorder() (woxui.Color, float32) {
	color, width := woxui.Color{}, float32(0)
	if t.QueryBoxBorderBottomColor != nil {
		color = *t.QueryBoxBorderBottomColor
	}
	if t.QueryBoxBorderBottomWidth != nil {
		width = max(float32(0), float32(*t.QueryBoxBorderBottomWidth))
	}
	return color, width
}
