package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// HotkeyRecorderProps describes the display state of a tappable hotkey recorder.
type HotkeyRecorderProps struct {
	Labels      []string
	Placeholder string
	Focused     bool
	Window      *woxui.Window
	Theme       Theme
}

// WoxHotkeyRecorder matches Flutter's outlined recorder with platform-labelled keycaps.
func WoxHotkeyRecorder(props HotkeyRecorderProps) (woxwidget.Widget, float32) {
	border := withAlpha(props.Theme.ResultSubtitle, 140)
	if props.Focused {
		border = props.Theme.Cursor
	}

	contentWidth := float32(80)
	var content woxwidget.Widget = woxwidget.Align{Width: contentWidth, Height: 22, Vertical: 0.5, Child: woxwidget.Text{
		Value: props.Placeholder, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
	}}
	if len(props.Labels) > 0 {
		content, contentWidth = WoxHotkey(HotkeyProps{
			// Flutter's recorder uses the app's default Material canvas rather than the launcher theme,
			// so key legends stay light and keyboard-like on both light and dark Wox surfaces.
			Labels: props.Labels, Foreground: woxui.Color{R: 33, G: 33, B: 33, A: 255}, Background: woxui.Color{R: 250, G: 250, B: 250, A: 255},
			Border: woxui.Color{R: 0, G: 0, B: 0, A: 31}, Compact: true, Window: props.Window,
		})
	}

	width := contentWidth + 16
	return woxwidget.Container{
		Width: width, Height: 30, Padding: woxwidget.Insets{Left: 8, Top: 4, Right: 8, Bottom: 4},
		BorderColor: border, BorderWidth: 1, Radius: 4, Child: content,
	}, width
}
