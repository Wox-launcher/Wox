package view

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// GlanceProps contains the display state and actions for the query-box glance accessory.
type GlanceProps struct {
	Text    string
	Tooltip string
	Width   float32
	Hovered bool
	Icon    *woxui.Image
	Theme   woxcomponent.Theme
	OnTap   func()
	OnHover func(bool, string, woxui.Rect)
}

// GlanceView builds the compact query-box glance accessory.
func GlanceView(props GlanceProps) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, 2)
	textWidth := props.Width - 16
	foreground := props.Theme.QueryText
	foreground.A = uint8(float32(foreground.A) * 0.8)
	if props.Icon != nil {
		children = append(children, woxwidget.Container{
			Width: 16, Height: 28, Padding: woxwidget.Insets{Top: 6, Bottom: 6},
			Child: woxwidget.Image{Source: props.Icon, Width: 16, Height: 16},
		})
		textWidth -= 21
	}
	text := strings.TrimSpace(props.Text)
	children = append(children, woxwidget.Container{Width: max(float32(20), textWidth), Height: 28, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Text{
		Value: compactViewText(text, 22), Style: woxui.TextStyle{Size: 15}, Color: foreground,
	}})
	background := woxui.Color{}
	if props.Hovered {
		background = props.Theme.QueryText
		background.A = uint8(float32(background.A) * 0.1)
	}
	tooltip := strings.TrimSpace(props.Tooltip)
	if tooltip == "" {
		tooltip = text
	}
	return woxwidget.Gesture{ID: "query-glance", OnTap: props.OnTap, OnHoverAt: func(inside bool, bounds woxui.Rect) {
		if props.OnHover != nil {
			props.OnHover(inside, tooltip, bounds)
		}
	}, Child: woxwidget.Container{
		Width: props.Width, Height: 30, Radius: 5, Color: background, Padding: woxwidget.Insets{Left: 8, Top: 1, Right: 8, Bottom: 1},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, Children: children},
	}}
}

func compactViewText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:max(0, maxRunes-1)]) + "…"
}
