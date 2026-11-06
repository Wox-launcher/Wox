package woxview

import "fmt"

// StateStyle is a paint-only patch. Pointers distinguish omission from explicit
// transparent colors, zero opacity, square corners, and removed borders.
type StateStyle struct {
	Color             *Color   `json:"color,omitempty"`
	BackgroundColor   *Color   `json:"backgroundColor,omitempty"`
	BorderColor       *Color   `json:"borderColor,omitempty"`
	BorderWidth       *Size    `json:"borderWidth,omitempty"`
	BorderRadius      *Size    `json:"borderRadius,omitempty"`
	BorderTopWidth    *Size    `json:"borderTopWidth,omitempty"`
	BorderRightWidth  *Size    `json:"borderRightWidth,omitempty"`
	BorderBottomWidth *Size    `json:"borderBottomWidth,omitempty"`
	BorderLeftWidth   *Size    `json:"borderLeftWidth,omitempty"`
	Opacity           *float32 `json:"opacity,omitempty"`
}

// InteractionStyles declares local paint feedback without producer round trips.
type InteractionStyles struct {
	HoverStyle    *StateStyle `json:"hoverStyle,omitempty"`
	PressedStyle  *StateStyle `json:"pressedStyle,omitempty"`
	FocusStyle    *StateStyle `json:"focusStyle,omitempty"`
	DisabledStyle *StateStyle `json:"disabledStyle,omitempty"`
}

func (p InteractionStyles) interactionStyles() InteractionStyles { return p }

func (p InteractionStyles) empty() bool {
	return p.HoverStyle == nil && p.PressedStyle == nil && p.FocusStyle == nil && p.DisabledStyle == nil
}

// validate uses the same size/color limits as ordinary style without accepting layout fields.
func (p InteractionStyles) validate() error {
	for name, patch := range map[string]*StateStyle{"hoverStyle": p.HoverStyle, "pressedStyle": p.PressedStyle, "focusStyle": p.FocusStyle, "disabledStyle": p.DisabledStyle} {
		if patch == nil {
			continue
		}
		if err := patch.apply(Style{}).validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// merge overlays only supplied properties; disabled styles bypass the other states.
func (p StateStyle) merge(next *StateStyle) StateStyle {
	if next == nil {
		return p
	}
	if next.Color != nil {
		p.Color = next.Color
	}
	if next.BackgroundColor != nil {
		p.BackgroundColor = next.BackgroundColor
	}
	if next.BorderColor != nil {
		p.BorderColor = next.BorderColor
	}
	if next.BorderWidth != nil {
		p.BorderWidth = next.BorderWidth
	}
	if next.BorderRadius != nil {
		p.BorderRadius = next.BorderRadius
	}
	if next.BorderTopWidth != nil {
		p.BorderTopWidth = next.BorderTopWidth
	}
	if next.BorderRightWidth != nil {
		p.BorderRightWidth = next.BorderRightWidth
	}
	if next.BorderBottomWidth != nil {
		p.BorderBottomWidth = next.BorderBottomWidth
	}
	if next.BorderLeftWidth != nil {
		p.BorderLeftWidth = next.BorderLeftWidth
	}
	if next.Opacity != nil {
		p.Opacity = next.Opacity
	}
	return p
}

// apply creates a new style snapshot and preserves the producer's immutable props.
func (p StateStyle) apply(style Style) Style {
	if p.Color != nil {
		style.Color = *p.Color
	}
	if p.BackgroundColor != nil {
		style.BackgroundColor = *p.BackgroundColor
	}
	if p.BorderColor != nil {
		style.BorderColor = *p.BorderColor
	}
	if p.BorderWidth != nil {
		style.BorderWidth = *p.BorderWidth
	}
	if p.BorderRadius != nil {
		style.BorderRadius = *p.BorderRadius
	}
	if p.BorderTopWidth != nil {
		style.BorderTopWidth = p.BorderTopWidth
	}
	if p.BorderRightWidth != nil {
		style.BorderRightWidth = p.BorderRightWidth
	}
	if p.BorderBottomWidth != nil {
		style.BorderBottomWidth = p.BorderBottomWidth
	}
	if p.BorderLeftWidth != nil {
		style.BorderLeftWidth = p.BorderLeftWidth
	}
	if p.Opacity != nil {
		style.Opacity = p.Opacity
	}
	if p.BorderWidth != nil || p.BorderColor != nil {
		width := style.BorderWidth
		if p.BorderWidth == nil && width == (Size{}) {
			width = Size{Value: 1}
		}
		// Side overrides paint above shared control chrome and preserve explicit zero.
		if style.BorderTopWidth == nil {
			style.BorderTopWidth = &width
		}
		if style.BorderRightWidth == nil {
			style.BorderRightWidth = &width
		}
		if style.BorderBottomWidth == nil {
			style.BorderBottomWidth = &width
		}
		if style.BorderLeftWidth == nil {
			style.BorderLeftWidth = &width
		}
	}
	style.state = &p
	return style
}
