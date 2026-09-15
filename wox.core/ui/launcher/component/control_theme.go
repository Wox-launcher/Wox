package component

import woxui "wox/ui/runtime"

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
}
