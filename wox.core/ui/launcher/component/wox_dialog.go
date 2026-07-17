package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// DialogProps describes one modal Wox surface.
type DialogProps struct {
	ID            string
	Label         string
	Width         float32
	Height        float32
	OverlayWidth  float32
	OverlayHeight float32
	BackdropID    string
	BackdropColor woxui.Color
	BackdropAlpha uint8
	Radius        float32
	Padding       woxwidget.Insets
	BorderColor   woxui.Color
	BorderWidth   float32
	OnDismiss     func()
	Child         woxwidget.Widget
	Theme         Theme
}

// WoxDialog builds shared modal chrome, focus trapping, and dialog semantics.
func WoxDialog(props DialogProps) woxwidget.Widget {
	radius := props.Radius
	if radius <= 0 {
		radius = 12
	}
	key := woxwidget.Key(props.ID)
	dialog := woxwidget.FocusScope{Key: key, Modal: true, Child: woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleDialog, Label: props.Label,
		Child: woxwidget.Container{
			Width: props.Width, Height: props.Height, Radius: radius, Color: props.Theme.ActionBackground, Padding: props.Padding,
			BorderColor: props.BorderColor, BorderWidth: props.BorderWidth, Child: props.Child,
		},
	}}
	if props.OverlayWidth <= 0 || props.OverlayHeight <= 0 {
		return dialog
	}
	backdropID := props.BackdropID
	if backdropID == "" {
		backdropID = props.ID + "-backdrop"
	}
	backdrop := props.BackdropColor
	if backdrop == (woxui.Color{}) {
		backdrop = props.Theme.Background
		backdrop.A = props.BackdropAlpha
	} else if props.BackdropAlpha > 0 {
		backdrop.A = props.BackdropAlpha
	}
	if backdrop.A == 0 {
		backdrop.A = 210
	}
	left := max(float32(0), (props.OverlayWidth-props.Width)/2)
	top := max(float32(0), (props.OverlayHeight-props.Height)/2)
	return woxwidget.Stack{Width: props.OverlayWidth, Height: props.OverlayHeight, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: backdropID, OnTap: props.OnDismiss, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: props.OverlayWidth, Height: props.OverlayHeight, Color: backdrop}}},
		{Left: left, Top: top, Child: dialog},
	}}
}
