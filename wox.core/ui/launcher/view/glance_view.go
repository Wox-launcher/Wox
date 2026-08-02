package view

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// GlanceProps contains the display state and actions for the query-box glance accessory.
type GlanceProps struct {
	Text         string
	Tooltip      string
	Width        float32
	Icon         *woxui.Image
	Theme        woxcomponent.Theme
	DensityScale float32
	OnTap        func()
	OnHover      func(bool, string, woxui.Rect)
}

type glanceViewState struct {
	hovered bool
}

// GlanceView builds the retained compact query-box glance accessory.
func GlanceView(props GlanceProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "query-glance-state", Type: (*glanceViewState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &glanceViewState{} },
	}
}

// InitState starts the accessory outside its transient hover state.
func (s *glanceViewState) InitState(_ woxwidget.StateContext, _ any) {
	s.hovered = false
}

// DidUpdateWidget preserves hover while immutable glance content is refreshed.
func (s *glanceViewState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build composes immutable glance content with locally owned hover state.
func (s *glanceViewState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(GlanceProps)
	iconSize := scaledLauncherSize(16, props.DensityScale)
	height := scaledLauncherSize(30, props.DensityScale)
	horizontalPadding := scaledLauncherSize(8, props.DensityScale)
	gap := scaledLauncherSize(5, props.DensityScale)
	children := make([]woxwidget.Widget, 0, 2)
	contentWidth := max(float32(0), props.Width-horizontalPadding*2)
	textWidth := contentWidth
	foreground := props.Theme.QueryText
	foreground.A = uint8(float32(foreground.A) * 0.8)
	if props.Icon != nil {
		children = append(children, woxwidget.Image{Source: props.Icon, Width: iconSize, Height: iconSize})
		textWidth -= iconSize + gap
	}
	text := strings.TrimSpace(props.Text)
	children = append(children, woxwidget.Container{Width: max(scaledLauncherSize(20, props.DensityScale), textWidth), Child: woxwidget.Text{
		Value: compactViewText(text, 22), Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.GlanceFontSize, props.DensityScale)}, Color: foreground,
	}})
	background := woxui.Color{}
	if s.hovered {
		background = props.Theme.QueryText
		background.A = uint8(float32(background.A) * 0.1)
	}
	tooltip := strings.TrimSpace(props.Tooltip)
	if tooltip == "" {
		tooltip = text
	}
	return woxwidget.Gesture{ID: "query-glance", OnTap: props.OnTap, OnHoverAt: func(inside bool, bounds woxui.Rect) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
		if props.OnHover != nil {
			props.OnHover(inside, tooltip, bounds)
		}
	}, Child: woxwidget.Container{
		Width: props.Width, Height: height, Radius: scaledLauncherSize(5, props.DensityScale), Color: background, Padding: woxwidget.Insets{Left: horizontalPadding, Right: horizontalPadding},
		Child: woxwidget.Align{Width: contentWidth, Height: height, Vertical: 0.5, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children,
		}},
	}}
}

// Dispose releases no resources because tooltip ownership remains with the launcher.
func (s *glanceViewState) Dispose() {}

func compactViewText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:max(0, maxRunes-1)]) + "…"
}
