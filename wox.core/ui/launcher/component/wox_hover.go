package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const controlHoverAlpha = uint8(25)

type hoverableProps struct {
	disabled bool
	build    func(bool, func(bool, woxui.Rect)) woxwidget.Widget
}

type hoverableState struct {
	hovered bool
}

func hoverable(key woxwidget.Key, disabled bool, build func(bool, func(bool, woxui.Rect)) woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: key, Type: (*hoverableState)(nil), Widget: hoverableProps{disabled: disabled, build: build},
		CreateState: func() woxwidget.State { return &hoverableState{} },
	}
}

func (s *hoverableState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *hoverableState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *hoverableState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(hoverableProps)
	return props.build(s.hovered && !props.disabled, func(inside bool, _ woxui.Rect) {
		inside = inside && !props.disabled
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
	})
}

func (s *hoverableState) Dispose() {}

// controlHoverColor composites the standard hover overlay without replacing a variant's base color.
func controlHoverColor(base, foreground woxui.Color) woxui.Color {
	overlayAlpha := float32(controlHoverAlpha) / 255
	baseAlpha := float32(base.A) / 255
	outputAlpha := overlayAlpha + baseAlpha*(1-overlayAlpha)
	if outputAlpha == 0 {
		return woxui.Color{}
	}
	blend := func(overlay, background uint8) uint8 {
		return uint8((float32(overlay)*overlayAlpha+float32(background)*baseAlpha*(1-overlayAlpha))/outputAlpha + 0.5)
	}
	return woxui.Color{
		R: blend(foreground.R, base.R),
		G: blend(foreground.G, base.G),
		B: blend(foreground.B, base.B),
		A: uint8(outputAlpha*255 + 0.5),
	}
}
