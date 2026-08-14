package overlay

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TitleBarCloseButton matches the WebView Windows close treatment and keeps compact chrome elsewhere.
func TitleBarCloseButton(windows bool, id string, foreground woxui.Color, onTap func()) woxwidget.Widget {
	if windows {
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: id, Label: "Close", Icon: woxcomponent.CloseGlyph(18, foreground),
			Width: 46, Height: 40, Radius: 0,
			HoverBackground: woxui.Color{R: 232, G: 17, B: 35, A: 255}, OnTap: onTap,
		})
	}
	return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: id, Label: "Close", Icon: woxcomponent.CloseGlyph(15, foreground),
		Width: 32, Height: 32, Radius: 5,
		HoverBackground: woxui.Color{R: 255, G: 255, B: 255, A: 28}, OnTap: onTap,
	})
}
