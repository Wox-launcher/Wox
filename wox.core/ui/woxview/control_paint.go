package woxview

import (
	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// controlFill returns the authored target, not the outer box's animated sample.
// The boolean preserves native transparent/default behavior when no fill exists.
func (s Style) controlFill(theme component.ControlTheme) (ui.Color, bool) {
	if s.controlTransition != nil {
		if s.controlBackground == nil {
			return ui.Color{}, false
		}
		return *s.controlBackground, true
	}
	return s.BackgroundColor.resolve(theme, ui.Color{}), s.BackgroundColor != (Color{})
}

// controlPaint leaves shared defaults intact unless this build has an active
// state patch. Foreground borders are owned by the outer Style wrapper, so the
// control's original stroke must not remain underneath a transparent override.
func (s Style) controlPaint(theme component.ControlTheme) *component.ControlPaint {
	if s.state == nil && s.controlTransition == nil {
		return nil
	}
	p := &component.ControlPaint{BackgroundTransition: s.controlTransition}
	if s.controlTransition != nil {
		p.Background = s.controlBackground
	}
	if s.state == nil {
		return p
	}
	if s.state.BackgroundColor != nil {
		color := s.BackgroundColor.resolve(theme, ui.Color{})
		p.Background = &color
		if s.controlTransition != nil {
			p.Background = s.controlBackground
		}
	}
	if s.state.Color != nil {
		color := s.Color.resolve(theme, theme.Text)
		p.Foreground = &color
	}
	if s.state.BorderRadius != nil {
		radius := s.BorderRadius.resolve(theme)
		p.Radius = &radius
	}
	if s.resolve(theme).hasSideBorders() {
		color, width := ui.Color{}, float32(0)
		p.BorderColor, p.BorderWidth = &color, &width
	}
	return p
}

// wrapControl avoids painting an authored translucent fill twice when the
// shared control owns its background and the outer wrapper adds foreground borders.
func (s Style) wrapControl(theme component.ControlTheme, child widget.Widget) widget.Widget {
	outer := s.resolve(theme)
	if s.controlTransition != nil || s.state != nil && s.state.BackgroundColor != nil {
		outer.BackgroundColor = Color{}
	}
	return outer.wrapOuter(theme, child)
}

// decorateControl keeps authored layout insets around a control that owns its fill.
func (s Style) decorateControl(theme component.ControlTheme, child widget.Widget) widget.Widget {
	outer := s.resolve(theme)
	if s.controlTransition != nil || s.state != nil && s.state.BackgroundColor != nil {
		outer.BackgroundColor = Color{}
	}
	return outer.decorate(theme, child)
}
