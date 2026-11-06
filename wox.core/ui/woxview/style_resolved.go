package woxview

import (
	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// resolvedStyle is a per-build layout snapshot. Geometry is logical and tokens
// have already followed Interface size; font size and rich-text line height stay
// normal-density bases because their typography owners apply scaling. Plain and
// selectable text resolve line height at their call sites. The source is immutable.
type resolvedStyle struct {
	Style
	Width             float32
	Height            float32
	MinWidth          float32
	MaxWidth          float32
	MinHeight         float32
	MaxHeight         float32
	Gap               float32
	BorderWidth       float32
	BorderRadius      float32
	FontSize          float32
	LineHeight        float32
	Top               *float32
	Left              *float32
	Right             *float32
	Bottom            *float32
	BorderTopWidth    *float32
	BorderRightWidth  *float32
	BorderBottomWidth *float32
	BorderLeftWidth   *float32
	Padding, Margin   widget.Insets
}

// resolve converts declarative sizes without changing the document.
func (s Style) resolve(theme component.ControlTheme) resolvedStyle {
	pointer := func(value *Size) *float32 {
		if value == nil {
			return nil
		}
		resolved := value.resolve(theme)
		return &resolved
	}
	return resolvedStyle{Style: s,
		Width:             s.Width.resolve(theme),
		Height:            s.Height.resolve(theme),
		MinWidth:          s.MinWidth.resolve(theme),
		MaxWidth:          s.MaxWidth.resolve(theme),
		MinHeight:         s.MinHeight.resolve(theme),
		MaxHeight:         s.MaxHeight.resolve(theme),
		Gap:               s.Gap.resolve(theme),
		BorderWidth:       s.BorderWidth.resolve(theme),
		BorderRadius:      s.BorderRadius.resolve(theme),
		FontSize:          s.FontSize.base(),
		LineHeight:        s.LineHeight.base(),
		Top:               pointer(s.Top),
		Left:              pointer(s.Left),
		Right:             pointer(s.Right),
		Bottom:            pointer(s.Bottom),
		BorderTopWidth:    pointer(s.BorderTopWidth),
		BorderRightWidth:  pointer(s.BorderRightWidth),
		BorderBottomWidth: pointer(s.BorderBottomWidth),
		BorderLeftWidth:   pointer(s.BorderLeftWidth),
		Padding:           s.Padding.resolve(theme), Margin: s.Margin.resolve(theme),
	}
}

func (s Style) decorate(theme component.ControlTheme, child widget.Widget) widget.Widget {
	return s.resolve(theme).decorate(theme, child)
}

func (s Style) wrapOuter(theme component.ControlTheme, child widget.Widget) widget.Widget {
	return s.resolve(theme).wrapOuter(theme, child)
}

func (s Style) finishBox(theme component.ControlTheme, child widget.Widget) widget.Widget {
	return s.resolve(theme).finishBox(theme, child)
}

func (s Style) textStyle(theme component.ControlTheme, size float32) ui.TextStyle {
	return s.resolve(theme).textStyle(theme, size)
}
