package component

import woxui "wox/ui/runtime"

// Theme contains the semantic colors shared by Wox launcher components.
type Theme struct {
	Background             woxui.Color
	QueryBackground        woxui.Color
	QueryText              woxui.Color
	Cursor                 woxui.Color
	SelectionBackground    woxui.Color
	SelectionText          woxui.Color
	ResultTitle            woxui.Color
	ResultSubtitle         woxui.Color
	ErrorText              woxui.Color
	SelectedBackground     woxui.Color
	SelectedTitle          woxui.Color
	SelectedSubtitle       woxui.Color
	ActionBackground       woxui.Color
	ActionHeader           woxui.Color
	ActionText             woxui.Color
	ActionSelected         woxui.Color
	ActionSelectedText     woxui.Color
	PreviewText            woxui.Color
	PreviewSplit           woxui.Color
	PreviewPropertyTitle   woxui.Color
	PreviewPropertyContent woxui.Color
	ToolbarBackground      woxui.Color
	ToolbarText            woxui.Color
}

func withAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
