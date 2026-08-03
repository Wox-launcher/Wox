package component

import (
	"fmt"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ScrollViewProps contains the geometry and optional controlled state for a vertical Wox scroll surface.
type ScrollViewProps struct {
	Key                 woxwidget.Key
	Content             woxwidget.Widget
	Width               float32
	Height              float32
	ContentHeight       float32
	Offset              float32
	Controller          *woxwidget.ScrollController
	KeepVisible         *woxwidget.ScrollRange
	ThumbColor          woxui.Color
	HideScrollbar       bool
	AlwaysShowScrollbar bool
	AutomationID        string
	Label               string
	OnScroll            func(float32)
}

type scrollViewState struct {
	visible    bool
	hovered    bool
	dragging   bool
	dragY      float32
	hideAt     time.Time
	hideTimer  *time.Timer
	controller *woxwidget.ScrollController
}

// WoxScrollView builds a vertical scroll surface with the shared fading draggable scrollbar.
func WoxScrollView(props ScrollViewProps) woxwidget.Widget {
	if props.Key == "" {
		props.Key = "wox-scroll"
	}
	if props.ContentHeight <= props.Height || props.Height <= 0 {
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
	offset := scrollCurrentOffset(props)
	viewportKey := props.Key + "-viewport"
	if viewportKey == "-viewport" {
		viewportKey = "wox-scroll-viewport"
	}
	scroll := woxwidget.ScrollView{
		Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight,
		Offset: offset, KeepVisible: props.KeepVisible, Child: props.Content,
	}
	if props.Controller != nil {
		scroll.Key = viewportKey
		scroll.ID = string(viewportKey)
		scroll.Controller = props.Controller
		scroll.OnOffsetChanged = func(float32) {
			if state != nil && !props.HideScrollbar {
				state.show(context)
			}
		}
	}
	children := []woxwidget.StackChild{{Child: scroll}}
	if !props.HideScrollbar && props.ContentHeight > props.Height && props.Height > 0 {
		thumbHeight := max(float32(24), props.Height*props.Height/props.ContentHeight)
		thumbTop := (props.Height - thumbHeight) * offset / (props.ContentHeight - props.Height)
		thumbColor := props.ThumbColor
		thumbColor.A = min(150, thumbColor.A)
		visible := state != nil && (state.visible || props.AlwaysShowScrollbar)
		targetOpacity := float32(0)
		if visible {
			targetOpacity = 1
		}
		targetWidth := float32(3)
		if state != nil && (state.hovered || state.dragging) {
			targetWidth = 7
		}
		opacityKey := props.Key + "-scrollbar-opacity"
		widthKey := props.Key + "-scrollbar-width"
		var thumb woxwidget.Widget = woxwidget.AnimatedFloat{Key: opacityKey, Target: targetOpacity, Duration: 200 * time.Millisecond, Builder: func(opacity float32) woxwidget.Widget {
			return woxwidget.AnimatedFloat{Key: widthKey, Target: targetWidth, Duration: 120 * time.Millisecond, Builder: func(width float32) woxwidget.Widget {
				color := thumbColor
				color.A = uint8(float32(color.A)*opacity + 0.5)
				return woxwidget.Align{Width: 12, Height: thumbHeight, Horizontal: 1, Child: woxwidget.Container{Width: width, Height: thumbHeight, Radius: width / 2, Color: color}}
			}}
		}}
		if visible {
			thumb = woxwidget.Gesture{ID: string(props.Key) + "-scrollbar", OnHover: func(hovered bool) { state.setHovered(context, hovered) }, OnPanStart: func(point woxui.Point) {
				state.dragY = thumbTop + point.Y
				state.dragging = true
				state.setHovered(context, true)
			}, OnPanUpdate: func(point woxui.Point) {
				pointerY := thumbTop + point.Y
				delta := (pointerY - state.dragY) * props.ContentHeight / props.Height
				state.dragY = pointerY
				if scrollOffset(props, delta) != scrollCurrentOffset(props) {
					applyScroll(props, delta)
				}
			}, OnPanEnd: func() {
				state.dragging = false
				state.scheduleHide(context)
				context.Invalidate()
			}, Child: thumb}
		}
		children = append(children, woxwidget.StackChild{Left: max(float32(0), props.Width-14), Top: thumbTop, Child: thumb})
	}
	var result woxwidget.Widget = woxwidget.Gesture{ID: string(props.Key), OnScrollHandled: func(delta woxui.Point) bool {
		scrollDelta := -delta.Y
		if scrollOffset(props, scrollDelta) == scrollCurrentOffset(props) {
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
			Value: fmt.Sprintf("%.0f/%.0f", offset, max(float32(0), props.ContentHeight-props.Height)), ReadOnly: true, Child: result,
		}
	}
	return result
}

func scrollOffset(props ScrollViewProps, delta float32) float32 {
	return min(max(float32(0), scrollCurrentOffset(props)+delta), max(float32(0), props.ContentHeight-props.Height))
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
