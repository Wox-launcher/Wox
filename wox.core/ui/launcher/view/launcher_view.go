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
	TitleBar      woxwidget.Widget
	Header        woxwidget.Widget
	Refinements   woxwidget.Widget
	Content       woxwidget.Widget
	Footer        woxwidget.Widget
	FooterOverlay bool
	QueryAtBottom bool
	Floating      *LauncherFloatingView
	Overlay       woxwidget.Widget
	Theme         woxcomponent.Theme
	PreviewOnly   bool
	BorderWidth   float32
	OnDragStart   func()
}

// BorderDragMoveArea exposes only the reserved outer chrome as logical drag regions.
func BorderDragMoveArea(width, height float32, insets woxwidget.Insets, child woxwidget.Widget, onDragStart func()) woxwidget.Widget {
	if width <= 0 || height <= 0 || onDragStart == nil || insets == (woxwidget.Insets{}) {
		return child
	}
	insets.Top = min(max(0, insets.Top), height)
	insets.Bottom = min(max(0, insets.Bottom), height-insets.Top)
	insets.Left = min(max(0, insets.Left), width)
	insets.Right = min(max(0, insets.Right), width-insets.Left)
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: child},
		{StretchWidth: true, Child: woxwidget.Gesture{ID: "launcher-border-drag-top", OnDragStart: onDragStart, Child: woxwidget.Container{Height: insets.Top}}},
		{AnchorBottom: true, StretchWidth: true, Child: woxwidget.Gesture{ID: "launcher-border-drag-bottom", OnDragStart: onDragStart, Child: woxwidget.Container{Height: insets.Bottom}}},
		{Top: insets.Top, Bottom: insets.Bottom, StretchHeight: true, Child: woxwidget.Gesture{ID: "launcher-border-drag-left", OnDragStart: onDragStart, Child: woxwidget.Container{Width: insets.Left}}},
		{Top: insets.Top, Bottom: insets.Bottom, AnchorRight: true, StretchHeight: true, Child: woxwidget.Gesture{ID: "launcher-border-drag-right", OnDragStart: onDragStart, Child: woxwidget.Container{Width: insets.Right}}},
	}}
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
		layers = append(layers, woxwidget.StackChild{Top: 20, Right: 20, AnchorRight: true, Child: button})
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
	windowWidth, windowHeight := props.Width, props.Height
	contentBounds := woxcomponent.LauncherContentBounds(windowWidth, windowHeight, props.Theme.AppContentInset, props.Theme.Surfaces)
	props.Width, props.Height = contentBounds.Width, contentBounds.Height
	sections := make([]woxwidget.Widget, 0, 5)
	if props.TitleBar != nil {
		sections = append(sections, props.TitleBar)
	}
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
	if props.Footer != nil && !props.FooterOverlay {
		sections = append(sections, props.Footer)
	}
	body := woxwidget.Widget(woxwidget.Flex{Axis: woxwidget.Vertical, Children: sections})
	if props.Footer != nil && props.FooterOverlay {
		body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Child: body},
			{AnchorBottom: true, Child: props.Footer},
		}}
	}
	// Keep the footer's backdrop on the main surface; only floating UI belongs
	// above an embedded WebView. The boundary is a no-op when no page was painted.
	body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: body},
		{Child: woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(list *woxui.DisplayList, _ woxui.Rect) {
			list.FlushEmbeddedSurfaceOverlay()
		}}},
	}}
	if props.Floating != nil && props.Floating.Child != nil {
		body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Child: body},
			{Left: props.Floating.Left, Top: props.Floating.Top, Bottom: props.Floating.Bottom, AnchorBottom: props.Floating.AnchorBottom, Child: props.Floating.Child},
		}}
	}
	if props.Overlay != nil {
		body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{{Child: body}, {Child: props.Overlay}}}
	}
	body = woxcomponent.WoxLauncherContent(windowWidth, windowHeight, props.Theme, body)
	props.Width, props.Height = windowWidth, windowHeight
	radius := props.Radius
	if props.Theme.AppWindowChrome && props.Theme.AppBorderRadius == nil && radius == 0 {
		radius = woxui.DefaultWindowCornerRadius
	}
	if props.Theme.AppBorderRadius != nil {
		requested := float32(*props.Theme.AppBorderRadius)
		if props.Theme.AppWindowChrome {
			radius = requested
		} else {
			radius = woxui.NativeWindowCornerRadius(requested)
		}
	}
	radius = min(max(float32(0), radius), min(props.Width, props.Height)/2)
	borderColor, borderWidth := woxui.Color{}, float32(0)
	if props.Theme.AppBorderWidth != nil {
		borderWidth = min(float32(*props.Theme.AppBorderWidth), min(props.Width, props.Height)/2)
	}
	if props.Theme.AppBorderColor != nil {
		borderColor = *props.Theme.AppBorderColor
	}
	// Draw the outline after content so a full-width footer cannot cover it.
	if borderWidth > 0 {
		body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Child: body}, {Child: woxwidget.Container{Width: props.Width, Height: props.Height, Radius: radius, BorderColor: borderColor, BorderWidth: borderWidth}},
		}}
	}
	window := woxwidget.Widget(woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.Background, Surface: props.Theme.Surfaces.Get("App"), Radius: radius, Child: body})
	dragInsets := woxwidget.Insets{Top: contentBounds.Y, Left: contentBounds.X, Right: max(0, windowWidth-contentBounds.X-contentBounds.Width), Bottom: max(0, windowHeight-contentBounds.Y-contentBounds.Height)}
	if props.PreviewOnly {
		border := min(max(0, props.BorderWidth), min(windowWidth, windowHeight)/2)
		dragInsets.Top = max(dragInsets.Top, border)
		dragInsets.Bottom = max(dragInsets.Bottom, border)
		dragInsets.Left = max(dragInsets.Left, border)
		dragInsets.Right = max(dragInsets.Right, border)
	}
	window = BorderDragMoveArea(props.Width, props.Height, dragInsets, window, props.OnDragStart)
	return woxwidget.Semantics{
		Key: "launcher-window-key", AutomationID: "launcher.window", Role: woxui.AccessibilityRoleWindow, Label: "Wox",
		Child: window,
	}
}
