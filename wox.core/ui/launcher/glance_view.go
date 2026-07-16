package launcher

import (
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func (a *App) buildGlance(item glanceItem, hovered, hideIcon bool, palette uiPalette, width float32) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, 2)
	textWidth := width - 16
	foreground := palette.queryText
	foreground.A = uint8(float32(foreground.A) * 0.8)
	if !hideIcon && item.Icon.ImageData != "" {
		iconTint := foreground
		iconTint.A = uint8(float32(iconTint.A) * 0.72)
		if image := a.imageForTint(item.Icon, &iconTint, 16); image != nil {
			children = append(children, woxwidget.Container{
				Width: 16, Height: 28, Padding: woxwidget.Insets{Top: 6, Bottom: 6},
				Child: woxwidget.Image{Source: image, Width: 16, Height: 16},
			})
			textWidth -= 21
		}
	}
	text := strings.TrimSpace(item.Text)
	children = append(children, woxwidget.Container{Width: max(float32(20), textWidth), Height: 28, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Text{
		Value: compactFormTableText(text, 22), Style: woxui.TextStyle{Size: 15}, Color: foreground,
	}})
	onTap := func() {}
	if item.Action != nil {
		onTap = a.executeGlanceAction
	}
	background := woxui.Color{}
	if hovered {
		background = palette.queryText
		background.A = uint8(float32(background.A) * 0.1)
	}
	tooltip := strings.TrimSpace(item.Tooltip)
	if tooltip == "" {
		tooltip = text
	}
	return woxwidget.Gesture{ID: "query-glance", OnTap: onTap, OnHoverAt: func(inside bool, bounds woxui.Rect) {
		a.setGlanceHover(inside, tooltip, bounds)
	}, Child: woxwidget.Container{
		Width: width, Height: 30, Radius: 5, Color: background, Padding: woxwidget.Insets{Left: 8, Top: 1, Right: 8, Bottom: 1},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, Children: children},
	}}
}
