package component

import (
	"math"

	woxui "wox/ui/runtime"
)

// ControlTheme defines appearance for reusable controls, independent of launcher layout and result styling.
type ControlTheme struct {
	ControlText               woxui.Color
	BodyText                  woxui.Color
	ChromeText                woxui.Color
	Background                woxui.Color
	Surface                   woxui.Color
	Text                      woxui.Color
	TextSecondary             woxui.Color
	InputBackground           woxui.Color
	InputText                 woxui.Color
	Focus                     woxui.Color
	TextSelectionBackground   woxui.Color
	TextSelectionText         woxui.Color
	SelectionBackground       woxui.Color
	SelectionText             woxui.Color
	Accent                    woxui.Color
	AccentText                woxui.Color
	Border                    woxui.Color
	Info                      woxui.Color
	Success                   woxui.Color
	Warning                   woxui.Color
	Error                     woxui.Color
	ScrollbarThumbColor       *woxui.Color
	ScrollbarThumbHoverColor  *woxui.Color
	ScrollbarThumbActiveColor *woxui.Color
	ScrollbarWidth            *int
	ScrollbarHoverWidth       *int
	ScrollbarBorderRadius     *int
	// DensityScale is the active Interface size. Zero and one keep normal-density bases.
	DensityScale float32
}

// Scaled rounds a normal-density size into the theme's Interface size.
func (theme ControlTheme) Scaled(value float32) float32 {
	scale := theme.DensityScale
	if scale <= 0 || scale == 1 || value == 0 {
		return value
	}
	return float32(math.Round(float64(value * scale)))
}
