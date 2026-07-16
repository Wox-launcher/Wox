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
			textWidth -= 22
		}
	}
	text := strings.TrimSpace(item.Text)
	children = append(children, woxwidget.Container{Width: max(float32(20), textWidth), Height: 28, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Text{
		Value: compactFormTableText(text, 22), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: palette.queryText,
	}})
	background := palette.queryBackground
	if item.Action != nil {
		background = palette.selectedBackground
	}
	onTap := func() {}
	if item.Action != nil {
		onTap = a.executeGlanceAction
	}
	return woxwidget.Gesture{ID: "query-glance", OnTap: onTap, Child: woxwidget.Container{
		Width: width, Height: 30, Radius: 6, Color: background, Padding: woxwidget.Insets{Left: 8, Top: 1, Right: 8, Bottom: 1},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: children},
	}}
}
