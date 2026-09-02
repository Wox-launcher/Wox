package component

import (
	"fmt"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ScrollViewProps contains the geometry and optional controlled state for a Wox scroll surface.
type ScrollViewProps struct {
	Key     woxwidget.Key
	Content woxwidget.Widget
	Width   float32
	// FillWidth and FillHeight adopt dimensions resolved by the parent layout.
	FillWidth     bool
	FillHeight    bool
	Height        float32
	ContentWidth  float32
	ContentHeight float32
	// Horizontal scrolls along X and places the shared fading thumb along the bottom.
	Horizontal          bool
	Offset              float32
	Controller          *woxwidget.ScrollController
	KeepVisible         *woxwidget.ScrollRange
	ThumbColor          woxui.Color
	HideScrollbar       bool
	AlwaysShowScrollbar bool
	AutomationID        string
	Label               string
	OnScroll            func(float32)
	// OnOffsetChanged reports absolute offset changes made through a retained scroll controller.
	OnOffsetChanged func(float32)
	// OnGeometryChanged reports measured geometry when the scroll-axis content extent is omitted.
	OnGeometryChanged func(viewport, content float32)
}

type scrollViewState struct {
	visible        bool
	hovered        bool
	dragging       bool
	dragOrigin     float32
	hideAt         time.Time
	hideTimer      *time.Timer
	controller     *woxwidget.ScrollController
	hasGeometry    bool
	viewportExtent float32
	contentExtent  float32
}

// WoxScrollView builds a scroll surface with the shared fading draggable scrollbar.
func WoxScrollView(props ScrollViewProps) woxwidget.Widget {
	if props.Key == "" {
		props.Key = "wox-scroll"
	}
	if props.FillWidth || props.FillHeight {
		fillWidth := props.FillWidth
		fillHeight := props.FillHeight
		props.FillWidth = false
		props.FillHeight = false
		return woxwidget.LayoutBuilder{Build: func(size woxui.Size) woxwidget.Widget {
			if fillWidth {
				props.Width = size.Width
			}
			if fillHeight {
				props.Height = size.Height
			}
			return WoxScrollView(props)
		}}
	}
	if !woxScrollNeedsState(props) {
		return buildWoxScrollView(woxwidget.StateContext{}, props, nil)
	}
	return woxwidget.Stateful{
		Key: props.Key, Type: (*scrollViewState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &scrollViewState{} },
	}
}

// InitState creates a controller when the caller does not own scroll state.
func (s *scrollViewState) InitState(_ woxwidget.StateContext, widget any) {
	props := widget.(ScrollViewProps)
	if props.Controller == nil && props.OnScroll == nil {
		s.controller = woxwidget.NewScrollController(props.Offset)
	}
}

// DidUpdateWidget reveals the scrollbar when a controlled offset moves.
func (s *scrollViewState) DidUpdateWidget(context woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(ScrollViewProps)
	newProps := newWidget.(ScrollViewProps)
	if newProps.Height != oldProps.Height || newProps.ContentHeight != oldProps.ContentHeight || newProps.Width != oldProps.Width || newProps.ContentWidth != oldProps.ContentWidth {
		s.hasGeometry = false
	}
	if newProps.HideScrollbar {
		s.visible = false
		s.hideAt = time.Time{}
		if s.hideTimer != nil {
			s.hideTimer.Stop()
			s.hideTimer = nil
		}
		return
	}
	if newProps.Controller == nil && newProps.Offset != oldProps.Offset && newProps.Height == oldProps.Height && newProps.ContentHeight == oldProps.ContentHeight {
		s.show(context)
	}
}

// Build expires inactivity and composes the scroll surface from retained interaction state.
func (s *scrollViewState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(ScrollViewProps)
	if props.Controller == nil && props.OnScroll == nil {
		if s.controller == nil {
			s.controller = woxwidget.NewScrollController(props.Offset)
		}
		props.Controller = s.controller
	}
	if s.visible && !s.hovered && !s.dragging && !s.hideAt.IsZero() && !time.Now().Before(s.hideAt) {
		s.visible = false
		s.hideAt = time.Time{}
		s.hideTimer = nil
	}
	return buildWoxScrollView(context, props, s)
}

// Dispose cancels the pending inactivity frame when the scroll surface leaves the tree.
func (s *scrollViewState) Dispose() {
	if s.hideTimer != nil {
		s.hideTimer.Stop()
		s.hideTimer = nil
	}
}

func (s *scrollViewState) show(context woxwidget.StateContext) {
	s.visible = true
	s.scheduleHide(context)
	context.Invalidate()
}

func (s *scrollViewState) scheduleHide(context woxwidget.StateContext) {
	if s.hideTimer != nil {
		s.hideTimer.Stop()
		s.hideTimer = nil
	}
	s.hideAt = time.Time{}
	if s.hovered || s.dragging {
		return
	}
	s.hideAt = time.Now().Add(2 * time.Second)
	s.hideTimer = time.AfterFunc(2*time.Second, context.Invalidate)
}

func (s *scrollViewState) setHovered(context woxwidget.StateContext, hovered bool) {
	if s.hovered == hovered {
		return
	}
	s.hovered = hovered
	s.scheduleHide(context)
	context.Invalidate()
}

func buildWoxScrollView(context woxwidget.StateContext, props ScrollViewProps, state *scrollViewState) woxwidget.Widget {
	contentHint := scrollAxisContent(props)
	if state != nil && state.hasGeometry {
		if props.Horizontal {
			props.Width = state.viewportExtent
			props.ContentWidth = state.contentExtent
		} else {
			props.Height = state.viewportExtent
			props.ContentHeight = state.contentExtent
		}
	}
	if props.Horizontal {
		props.ContentWidth = max(props.Width, props.ContentWidth)
	} else {
		props.ContentHeight = max(props.Height, props.ContentHeight)
	}
	offset := scrollCurrentOffset(props)
	viewportKey := props.Key + "-viewport"
	if viewportKey == "-viewport" {
		viewportKey = "wox-scroll-viewport"
	}
	scroll := woxwidget.ScrollView{
		// A Wox strip is never nested inside another scroller, so a horizontal
		// surface always consumes the ordinary mouse wheel.
		Width: props.Width, Height: props.Height, Horizontal: props.Horizontal, MapVerticalWheel: props.Horizontal,
		Offset: offset, KeepVisible: props.KeepVisible, Child: props.Content,
	}
	if props.Horizontal {
		scroll.ContentWidth = contentHint
	} else {
		scroll.ContentHeight = contentHint
	}
	if contentHint <= 0 {
		scroll.OnGeometryChanged = func(viewport, content float32) {
			geometryChanged := state == nil || !state.hasGeometry || state.viewportExtent != viewport || state.contentExtent != content
			if !geometryChanged {
				return
			}
			if state != nil {
				state.hasGeometry = true
				state.viewportExtent = viewport
				state.contentExtent = content
				context.Invalidate()
			}
			if props.OnGeometryChanged != nil {
				props.OnGeometryChanged(viewport, content)
			}
		}
	}
	if props.Controller != nil {
		scroll.Key = viewportKey
		scroll.ID = string(viewportKey)
		scroll.Controller = props.Controller
		scroll.OnOffsetChanged = func(offset float32) {
			if state != nil && !props.HideScrollbar {
				state.show(context)
			}
			if props.OnOffsetChanged != nil {
				props.OnOffsetChanged(offset)
			}
		}
	}
	children := []woxwidget.StackChild{{Child: scroll}}
	viewport := scrollAxisViewport(props)
	content := scrollAxisContent(props)
	if !props.HideScrollbar && content > viewport && viewport > 0 {
		thumbLength := max(float32(24), viewport*viewport/content)
		thumbOffset := (viewport - thumbLength) * offset / (content - viewport)
		thumbColor := props.ThumbColor
		thumbColor.A = min(150, thumbColor.A)
		visible := state != nil && (state.visible || props.AlwaysShowScrollbar)
		targetOpacity := float32(0)
		if visible {
			targetOpacity = 1
		}
		targetThickness := float32(3)
		if state != nil && (state.hovered || state.dragging) {
			targetThickness = 7
		}
		opacityKey := props.Key + "-scrollbar-opacity"
		widthKey := props.Key + "-scrollbar-width"
		var thumb woxwidget.Widget = woxwidget.AnimatedFloat{Key: opacityKey, Target: targetOpacity, Duration: 200 * time.Millisecond, Builder: func(opacity float32) woxwidget.Widget {
			return woxwidget.AnimatedFloat{Key: widthKey, Target: targetThickness, Duration: 120 * time.Millisecond, Builder: func(thickness float32) woxwidget.Widget {
				color := thumbColor
				color.A = uint8(float32(color.A)*opacity + 0.5)
				if props.Horizontal {
					return woxwidget.Align{Width: thumbLength, Height: 12, Vertical: 1, Child: woxwidget.Container{Width: thumbLength, Height: thickness, Radius: thickness / 2, Color: color}}
				}
				return woxwidget.Align{Width: 12, Height: thumbLength, Horizontal: 1, Child: woxwidget.Container{Width: thickness, Height: thumbLength, Radius: thickness / 2, Color: color}}
			}}
		}}
		if visible {
			thumb = woxwidget.Gesture{ID: string(props.Key) + "-scrollbar", OnHover: func(hovered bool) { state.setHovered(context, hovered) }, OnPanStart: func(point woxui.Point) {
				state.dragOrigin = thumbOffset + scrollAxisPointer(props, point)
				state.dragging = true
				state.setHovered(context, true)
			}, OnPanUpdate: func(point woxui.Point) {
				pointer := thumbOffset + scrollAxisPointer(props, point)
				delta := (pointer - state.dragOrigin) * content / viewport
				state.dragOrigin = pointer
				if scrollOffset(props, delta) != scrollCurrentOffset(props) {
					applyScroll(props, delta)
				}
			}, OnPanEnd: func() {
				state.dragging = false
				state.scheduleHide(context)
				context.Invalidate()
			}, Child: thumb}
		}
		if props.Horizontal {
			children = append(children, woxwidget.StackChild{Left: thumbOffset, Bottom: 2, AnchorBottom: true, Child: thumb})
		} else {
			children = append(children, woxwidget.StackChild{Top: thumbOffset, Right: 2, AnchorRight: true, Child: thumb})
		}
	}
	var result woxwidget.Widget = woxwidget.Gesture{ID: string(props.Key), CoverHover: true, OnHover: func(inside bool) {
		if state == nil || props.HideScrollbar {
			return
		}
		if inside {
			state.show(context)
		}
		state.setHovered(context, inside)
	}, OnScrollHandled: func(delta woxui.Point) bool {
		scrollDelta := woxScrollWheelDelta(props, delta)
		if scrollDelta == 0 || scrollOffset(props, scrollDelta) == scrollCurrentOffset(props) {
			return false
		}
		if state != nil && !props.HideScrollbar {
			state.show(context)
		}
		applyScroll(props, scrollDelta)
		return true
	}, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: children}}
	if props.AutomationID != "" {
		result = woxwidget.Semantics{
			Key: props.Key + "-semantics", AutomationID: props.AutomationID, Role: woxui.AccessibilityRoleGroup, Label: props.Label,
			Value: fmt.Sprintf("%.0f/%.0f", offset, max(float32(0), content-viewport)), ReadOnly: true, Child: result,
		}
	}
	return result
}

func woxScrollNeedsState(props ScrollViewProps) bool {
	if scrollAxisViewport(props) <= 0 {
		return false
	}
	content := scrollAxisContent(props)
	return content <= 0 || content > scrollAxisViewport(props)
}

func scrollAxisViewport(props ScrollViewProps) float32 {
	if props.Horizontal {
		return props.Width
	}
	return props.Height
}

func scrollAxisContent(props ScrollViewProps) float32 {
	if props.Horizontal {
		return props.ContentWidth
	}
	return props.ContentHeight
}

func scrollAxisPointer(props ScrollViewProps, point woxui.Point) float32 {
	if props.Horizontal {
		return point.X
	}
	return point.Y
}

func woxScrollWheelDelta(props ScrollViewProps, delta woxui.Point) float32 {
	if !props.Horizontal {
		return -delta.Y
	}
	if delta.X != 0 {
		return -delta.X
	}
	return -delta.Y
}

func scrollOffset(props ScrollViewProps, delta float32) float32 {
	return min(max(float32(0), scrollCurrentOffset(props)+delta), max(float32(0), scrollAxisContent(props)-scrollAxisViewport(props)))
}

func scrollCurrentOffset(props ScrollViewProps) float32 {
	if props.Controller != nil {
		return props.Controller.Offset()
	}
	return props.Offset
}

func applyScroll(props ScrollViewProps, delta float32) {
	if props.Controller != nil {
		props.Controller.ScrollBy(delta)
	} else if props.OnScroll != nil {
		props.OnScroll(delta)
	}
}
