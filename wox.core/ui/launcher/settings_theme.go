package launcher

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

// settingsPalette owns fixed dark colors independently of launcher themes.
// Translucent window and popup tints reveal their existing blur materials.
func settingsPalette() woxcomponent.ControlTheme {
	text := woxui.Color{R: 245, G: 245, B: 247, A: 255}
	white := woxui.Color{R: 255, G: 255, B: 255, A: 255}
	return woxcomponent.ControlTheme{
		Background:  woxui.Color{R: 22, G: 22, B: 26, A: 191},
		Surface:     woxui.Color{R: 22, G: 22, B: 26, A: 88},
		ControlText: text, BodyText: text, ChromeText: woxui.Color{R: 168, G: 168, B: 179, A: 255},
		Text: text, TextSecondary: woxui.Color{R: 168, G: 168, B: 179, A: 255},
		InputBackground: woxui.Color{R: 255, G: 255, B: 255, A: 10}, InputText: text,
		Focus: text, TextSelectionBackground: woxui.Color{R: 255, G: 255, B: 255, A: 61}, TextSelectionText: white,
		SelectionBackground: woxui.Color{R: 255, G: 255, B: 255, A: 35}, SelectionText: white,
		// Active controls need an opaque fill distinct from the translucent row selection.
		Accent: text, AccentText: woxui.Color{R: 22, G: 22, B: 26, A: 255},
		Info:    woxui.Color{R: 64, G: 196, B: 255, A: 255},
		Success: woxui.Color{R: 74, G: 222, B: 128, A: 255},
		Warning: woxui.Color{R: 253, G: 186, B: 116, A: 255},
		Border:  woxui.Color{R: 255, G: 255, B: 255, A: 40}, Error: woxui.Color{R: 232, G: 95, B: 95, A: 255},
	}
}
