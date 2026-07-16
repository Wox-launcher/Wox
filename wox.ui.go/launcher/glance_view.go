package launcher

import (
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

func (a *App) buildGlance(item glanceItem, hideIcon bool, palette uiPalette, width float32) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, 2)
	textWidth := width - 16
	if !hideIcon && item.Icon.ImageData != "" {
		if image := a.imageFor(item.Icon); image != nil {
			children = append(children, woxwidget.Image{Source: image, Width: 16, Height: 16})
			textWidth -= 21
		}
	}
	text := strings.TrimSpace(item.Text)
	foreground := palette.queryText
	foreground.A = uint8(float32(foreground.A) * 0.8)
	children = append(children, woxwidget.Container{Width: max(float32(20), textWidth), Height: 28, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Text{
		Value: compactFormTableText(text, 22), Style: woxui.TextStyle{Size: 15}, Color: foreground,
	}})
	onTap := func() {}
	if item.Action != nil {
		onTap = a.executeGlanceAction
	}
	return woxwidget.Gesture{ID: "query-glance", OnTap: onTap, Child: woxwidget.Container{
		Width: width, Height: 30, Radius: 5, Padding: woxwidget.Insets{Left: 8, Top: 1, Right: 8, Bottom: 1},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, Children: children},
	}}
}
