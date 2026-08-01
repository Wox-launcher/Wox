package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// IconButtonProps describes a compact icon-only button with retained hover state.
type IconButtonProps struct {
	ID              string
	Label           string
	Icon            woxwidget.Widget
	Width           float32
	Height          float32
	Radius          float32
	Background      woxui.Color
	HoverBackground woxui.Color
	FocusRingColor  woxui.Color
	Disabled        bool
	OnTap           func()
	OnHoverAt       func(bool, woxui.Rect)
}

type iconButtonState struct {
	hovered bool
}

// WoxIconButton builds an icon-only button with centered content and hover feedback.
func WoxIconButton(props IconButtonProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.ID), Type: (*iconButtonState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &iconButtonState{} },
	}
}

func (s *iconButtonState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *iconButtonState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *iconButtonState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(IconButtonProps)
	background := props.Background
	if s.hovered {
		background = props.HoverBackground
	}
	onTap := props.OnTap
	actions := []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	if props.Disabled {
		onTap = nil
		actions = nil
	}
	key := woxwidget.Key(props.ID)
	content := woxwidget.Gesture{ID: props.ID, OnTap: onTap, OnHoverAt: func(inside bool, bounds woxui.Rect) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
		if props.OnHoverAt != nil {
			props.OnHoverAt(inside, bounds)
		}
	}, Child: woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: props.Radius, Color: background,
		Child: woxwidget.Align{Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.5, Child: props.Icon},
	}}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleButton, Label: props.Label, Actions: actions, Disabled: props.Disabled,
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: props.FocusRingColor, FocusRingRadius: props.Radius, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down && onTap != nil {
				onTap()
			}
			return true
		}, Child: content},
	}
}

func (s *iconButtonState) Dispose() {}

// CloseGlyph draws a centered close icon without depending on font baseline metrics.
func CloseGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		dotSize := max(float32(1.5), size/8)
		span := size * 0.5
		start := (size - span) * 0.5
		stepSize := (span - dotSize) / 4
		for step := 0; step < 5; step++ {
			offset := float32(step) * stepSize
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + start + offset, Y: bounds.Y + start + offset, Width: dotSize, Height: dotSize}, dotSize/2, color)
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + start + span - dotSize - offset, Y: bounds.Y + start + offset, Width: dotSize, Height: dotSize}, dotSize/2, color)
		}
	}}
}

// MenuGlyph draws a centered menu icon without depending on font baseline metrics.
func MenuGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 18
	}
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		lineWidth := size * 0.82
		lineHeight := max(float32(1.5), size/11)
		left := bounds.X + (size-lineWidth)*0.5
		for index := 0; index < 3; index++ {
			top := bounds.Y + size*0.25 + float32(index)*size*0.25 - lineHeight*0.5
			displayList.FillRoundedRect(woxui.Rect{X: left, Y: top, Width: lineWidth, Height: lineHeight}, lineHeight/2, color)
		}
	}}
}

// ChevronGlyph draws a font-independent disclosure chevron centered in its box.
func ChevronGlyph(size float32, color woxui.Color, expanded bool) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		x := bounds.X + (bounds.Width-size)*0.5
		y := bounds.Y + (bounds.Height-size)*0.5
		if expanded {
			displayList.FillConvexPolygon([]woxui.Point{{X: x + 3, Y: y + 5}, {X: x + 5, Y: y + 4}, {X: x + 9, Y: y + 8}, {X: x + 8, Y: y + 10}}, color)
			displayList.FillConvexPolygon([]woxui.Point{{X: x + 7, Y: y + 8}, {X: x + 11, Y: y + 4}, {X: x + 13, Y: y + 5}, {X: x + 8, Y: y + 10}}, color)
			return
		}
		displayList.FillConvexPolygon([]woxui.Point{{X: x + 5, Y: y + 3}, {X: x + 7, Y: y + 3}, {X: x + 11, Y: y + 8}, {X: x + 9, Y: y + 9}}, color)
		displayList.FillConvexPolygon([]woxui.Point{{X: x + 9, Y: y + 7}, {X: x + 11, Y: y + 8}, {X: x + 7, Y: y + 13}, {X: x + 5, Y: y + 13}}, color)
	}}
}

// CopyGlyph draws the overlapping document outline used by chat copy actions.
func CopyGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 14
	}
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		x := bounds.X + (bounds.Width-size)*0.5
		y := bounds.Y + (bounds.Height-size)*0.5
		displayList.StrokeRoundedRect(woxui.Rect{X: x + 2, Y: y + 2, Width: size - 5, Height: size - 5}, 1, 1.25, color)
		displayList.StrokeRoundedRect(woxui.Rect{X: x + 5, Y: y + 5, Width: size - 5, Height: size - 5}, 1, 1.25, color)
	}}
}

// EditGlyph draws the diagonal pencil used by chat edit actions.
func EditGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 14
	}
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		x := bounds.X + (bounds.Width-size)*0.5
		y := bounds.Y + (bounds.Height-size)*0.5
		displayList.FillConvexPolygon([]woxui.Point{{X: x + 4, Y: y + 10.5}, {X: x + 9.5, Y: y + 5}, {X: x + 11.5, Y: y + 7}, {X: x + 6, Y: y + 12.5}}, color)
		displayList.FillConvexPolygon([]woxui.Point{{X: x + 2.5, Y: y + 13}, {X: x + 4, Y: y + 10.5}, {X: x + 6, Y: y + 12.5}}, color)
	}}
}

// RefreshGlyph draws the circular arrow used by chat retry actions.
func RefreshGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 14
	}
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		x := bounds.X + (bounds.Width-size)*0.5
		y := bounds.Y + (bounds.Height-size)*0.5
		displayList.StrokeRoundedRect(woxui.Rect{X: x + 2.5, Y: y + 2.5, Width: size - 5, Height: size - 5}, (size-5)/2, 1.25, color)
		displayList.FillConvexPolygon([]woxui.Point{{X: x + 9, Y: y + 1.5}, {X: x + 13, Y: y + 2.5}, {X: x + 11, Y: y + 6}}, color)
	}}
}

// DebugGlyph draws a compact bug outline without relying on an icon font.
func DebugGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		x := bounds.X + (bounds.Width-size)*0.5
		y := bounds.Y + (bounds.Height-size)*0.5
		displayList.StrokeRoundedRect(woxui.Rect{X: x + 4, Y: y + 4, Width: size - 8, Height: size - 7}, 2, 1.25, color)
		for _, offset := range []float32{5, 9} {
			displayList.FillRoundedRect(woxui.Rect{X: x + 1.5, Y: y + offset, Width: 3, Height: 1.25}, 0.6, color)
			displayList.FillRoundedRect(woxui.Rect{X: x + size - 4.5, Y: y + offset, Width: 3, Height: 1.25}, 0.6, color)
		}
		displayList.FillRoundedRect(woxui.Rect{X: x + 6, Y: y + 1.5, Width: 1.25, Height: 3}, 0.6, color)
		displayList.FillRoundedRect(woxui.Rect{X: x + size - 7.25, Y: y + 1.5, Width: 1.25, Height: 3}, 0.6, color)
	}}
}
