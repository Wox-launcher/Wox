package view

import (
	"math"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func scaledLauncherSize(value, scale float32) float32 {
	if scale <= 0 {
		scale = 1
	}
	return float32(math.Round(float64(value * scale)))
}

// LauncherFloatingView contains one positioned launcher panel.
type LauncherFloatingView struct {
	Child        woxwidget.Widget
	Left         float32
	Top          float32
	Bottom       float32
	AnchorBottom bool
}

// LauncherViewProps contains the prepared launcher sections and overlays.
type LauncherViewProps struct {
	Width         float32
	Height        float32
	Radius        float32
	Header        woxwidget.Widget
	Refinements   woxwidget.Widget
	Content       woxwidget.Widget
	Footer        woxwidget.Widget
	QueryAtBottom bool
	Floating      *LauncherFloatingView
	Overlay       woxwidget.Widget
	Theme         woxcomponent.Theme
}

// PreviewHoverCloseProps describes the fallback close affordance for preview-only launcher layouts.
type PreviewHoverCloseProps struct {
	Width     float32
	Height    float32
	Child     woxwidget.Widget
	Label     string
	Theme     woxcomponent.Theme
	OnClose   func()
	OnTooltip func(bool, string, woxui.Rect)
}

type previewHoverCloseState struct {
	hovered bool
}

// PreviewHoverClose keeps the close affordance local to preview-only content.
func PreviewHoverClose(props PreviewHoverCloseProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "launcher-preview-hover-close-state", Type: (*previewHoverCloseState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &previewHoverCloseState{} },
	}
}

func (s *previewHoverCloseState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *previewHoverCloseState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build reveals the close button while the pointer remains anywhere over the preview.
func (s *previewHoverCloseState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(PreviewHoverCloseProps)
	layers := []woxwidget.StackChild{{Child: props.Child}}
	if s.hovered {
		hoverBackground := props.Theme.PreviewSplit
		hoverBackground.A = uint8(float32(hoverBackground.A) * 0.1)
		button := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "launcher-preview-close", Label: props.Label, Icon: woxcomponent.CloseGlyph(16, props.Theme.PreviewSplit), Width: 28, Height: 28, Radius: 6,
			HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnClose, OnHoverAt: func(inside bool, bounds woxui.Rect) {
				// Hover targets do not bubble, so keep the preview affordance visible while the button owns the pointer.
				if inside != s.hovered {
					context.SetState(func() { s.hovered = inside })
				}
				if props.OnTooltip != nil {
					props.OnTooltip(inside, props.Label, bounds)
				}
			},
		})
		layers = append(layers, woxwidget.StackChild{Left: max(float32(0), props.Width-48), Top: 20, Child: button})
	}
	return woxwidget.Gesture{ID: "launcher-preview-hover", OnHover: func(inside bool) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
	}, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers}}
}

func (s *previewHoverCloseState) Dispose() {}

// LauncherView builds the accessible launcher window and its overlay layers.
func LauncherView(props LauncherViewProps) woxwidget.Widget {
	sections := make([]woxwidget.Widget, 0, 4)
	if !props.QueryAtBottom {
		if props.Header != nil {
			sections = append(sections, props.Header)
		}
		if props.Refinements != nil {
			sections = append(sections, props.Refinements)
		}
	}
	if props.Content != nil {
		sections = append(sections, props.Content)
	}
	if props.QueryAtBottom {
		if props.Refinements != nil {
			sections = append(sections, props.Refinements)
		}
		if props.Header != nil {
			sections = append(sections, props.Header)
		}
	}
	if props.Footer != nil {
		sections = append(sections, props.Footer)
	}
	body := woxwidget.Widget(woxwidget.Flex{Axis: woxwidget.Vertical, Children: sections})
	if props.Floating != nil && props.Floating.Child != nil {
		body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Child: body},
			{Left: props.Floating.Left, Top: props.Floating.Top, Bottom: props.Floating.Bottom, AnchorBottom: props.Floating.AnchorBottom, Child: props.Floating.Child},
		}}
	}
	if props.Overlay != nil {
		body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{{Child: body}, {Child: props.Overlay}}}
	}
	return woxwidget.Semantics{
		Key: "launcher-window-key", AutomationID: "launcher.window", Role: woxui.AccessibilityRoleWindow, Label: "Wox",
		Child: woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.Background, Radius: props.Radius, Child: body},
	}
}
