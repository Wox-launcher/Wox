package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxScrollViewOpacityFollowsScrollActivity(t *testing.T) {
	props := ScrollViewProps{Key: "test-scroll", Width: 100, Height: 80, ContentHeight: 160, ThumbColor: woxui.Color{A: 255}}
	state := &scrollViewState{}
	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	animation := stack.Children[1].Child.(woxwidget.AnimatedFloat)
	if animation.Target != 0 {
		t.Fatalf("idle scrollbar opacity target = %.0f, want 0", animation.Target)
	}

	state.visible = true
	state.hovered = true
	stack = state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	animation = stack.Children[1].Child.(woxwidget.Gesture).Child.(woxwidget.AnimatedFloat)
	widthAnimation := animation.Builder(0.5).(woxwidget.AnimatedFloat)
	thumb := widthAnimation.Builder(3).(woxwidget.Align).Child.(woxwidget.Container)
	if animation.Target != 1 || widthAnimation.Target != 7 || thumb.Color.A != 75 {
		t.Fatalf("active scrollbar = opacity %.0f width %.0f alpha %d, want 1/7/75", animation.Target, widthAnimation.Target, thumb.Color.A)
	}
	state.Dispose()
}

func TestWoxScrollViewCanKeepOverflowIndicatorVisible(t *testing.T) {
	props := ScrollViewProps{Key: "persistent-scroll", Width: 100, Height: 80, ContentHeight: 160, ThumbColor: woxui.Color{A: 255}, AlwaysShowScrollbar: true, AutomationID: "persistent-scroll-state", Label: "Scroll position"}
	state := &scrollViewState{}
	semantics := state.Build(woxwidget.StateContext{}, props).(woxwidget.Semantics)
	view := semantics.Child.(woxwidget.Gesture)
	stack := view.Child.(woxwidget.Stack)
	thumb := stack.Children[1].Child.(woxwidget.Gesture)
	animation := thumb.Child.(woxwidget.AnimatedFloat)
	view.OnScroll(woxui.Point{Y: -20})
	semantics = state.Build(woxwidget.StateContext{}, props).(woxwidget.Semantics)
	if animation.Target != 1 || state.controller.Offset() != 20 || semantics.AutomationID != "persistent-scroll-state" || semantics.Value != "20/80" {
		t.Fatalf("persistent scrollbar = opacity %.0f offset %.0f automation %q value %q", animation.Target, state.controller.Offset(), semantics.AutomationID, semantics.Value)
	}
	state.Dispose()
}

func TestWoxScrollViewOwnsActivityHoverAndDrag(t *testing.T) {
	scrolled := float32(0)
	props := ScrollViewProps{Key: "test-scroll", Width: 100, Height: 80, ContentHeight: 160, OnScroll: func(delta float32) { scrolled += delta }}
	state := &scrollViewState{}
	view := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture)
	view.OnScroll(woxui.Point{Y: 20})
	if state.visible || scrolled != 0 {
		t.Fatalf("top-boundary scroll = visible %v delta %.0f, want false/0", state.visible, scrolled)
	}
	view.OnScroll(woxui.Point{Y: -20})
	if !state.visible || scrolled != 20 || state.hideTimer == nil {
		t.Fatalf("real scroll = visible %v delta %.0f timer %v, want true/20/non-nil", state.visible, scrolled, state.hideTimer)
	}

	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	thumb := stack.Children[1].Child.(woxwidget.Gesture)
	thumb.OnHover(true)
	thumb.OnPanStart(woxui.Point{Y: 10})
	thumb.OnPanUpdate(woxui.Point{Y: 20})
	if scrolled != 40 || !state.dragging || state.hideTimer != nil {
		t.Fatalf("thumb drag = delta %.0f dragging %v timer %v, want 40/true/nil", scrolled, state.dragging, state.hideTimer)
	}
	thumb.OnHover(false)
	thumb.OnPanEnd()
	if state.dragging || state.hideTimer == nil {
		t.Fatalf("ended drag = dragging %v timer %v, want false/non-nil", state.dragging, state.hideTimer)
	}
	state.Dispose()
}

func TestWoxScrollViewOwnsUncontrolledScrollController(t *testing.T) {
	keepVisible := &woxwidget.ScrollRange{Start: 80, End: 120}
	props := ScrollViewProps{Key: "catalog", Width: 100, Height: 80, ContentHeight: 160, KeepVisible: keepVisible}
	state := &scrollViewState{}
	state.InitState(woxwidget.StateContext{}, props)
	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	scroll := stack.Children[0].Child.(woxwidget.ScrollView)

	if scroll.Controller == nil || scroll.KeepVisible != keepVisible || scroll.OnOffsetChanged == nil || scroll.ID != "catalog-viewport" {
		t.Fatalf("uncontrolled scroll = controller %p keep visible %p callback %v id %q, want retained shared ownership", scroll.Controller, scroll.KeepVisible, scroll.OnOffsetChanged != nil, scroll.ID)
	}
	state.Dispose()
}

func TestWoxScrollViewAcceptsRetainedScrollController(t *testing.T) {
	controller := woxwidget.NewScrollController(0)
	props := ScrollViewProps{Key: "controlled", Width: 100, Height: 80, ContentHeight: 160, Controller: controller}
	state := &scrollViewState{}
	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	scroll := stack.Children[0].Child.(woxwidget.ScrollView)

	if scroll.Controller != controller || scroll.OnOffsetChanged == nil {
		t.Fatalf("retained controller = %p callback %v, want %p/non-nil", scroll.Controller, scroll.OnOffsetChanged != nil, controller)
	}
	scroll.OnOffsetChanged(20)
	if !state.visible || state.hideTimer == nil {
		t.Fatalf("controller activity = visible %v timer %v, want true/non-nil", state.visible, state.hideTimer)
	}
	state.Dispose()
}

func TestWoxScrollViewShowsForControlledOffsetMovement(t *testing.T) {
	state := &scrollViewState{}
	oldProps := ScrollViewProps{Height: 80, ContentHeight: 160}
	newProps := oldProps
	newProps.Offset = 20
	state.DidUpdateWidget(woxwidget.StateContext{}, oldProps, newProps)
	if !state.visible || state.hideTimer == nil {
		t.Fatalf("controlled offset movement = visible %v timer %v, want true/non-nil", state.visible, state.hideTimer)
	}
	state.Dispose()
}

func TestWoxScrollViewKeepsExternalOffsetDeclarative(t *testing.T) {
	props := ScrollViewProps{Key: "launcher-results", Width: 100, Height: 80, ContentHeight: 160, Offset: 20, OnScroll: func(float32) {}}
	state := &scrollViewState{}
	scroll := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	if scroll.Key != "" || scroll.Controller != nil || scroll.Offset != 20 {
		t.Fatalf("external viewport = key %q controller %p offset %.0f, want unretained nil/20", scroll.Key, scroll.Controller, scroll.Offset)
	}

	props.Offset = 60
	scroll = state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	if scroll.Offset != 60 {
		t.Fatalf("updated external viewport offset = %.0f, want 60", scroll.Offset)
	}
	state.Dispose()
}

func TestWoxScrollViewUsesCallerKeyForInteractionIDs(t *testing.T) {
	props := ScrollViewProps{Key: "settings-search", Width: 100, Height: 80, ContentHeight: 160}
	state := &scrollViewState{visible: true}
	view := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture)
	stack := view.Child.(woxwidget.Stack)
	thumb := stack.Children[1].Child.(woxwidget.Gesture)

	if view.ID != "settings-search" || thumb.ID != "settings-search-scrollbar" {
		t.Fatalf("scroll interaction ids = %q/%q, want caller-scoped ids", view.ID, thumb.ID)
	}
	state.Dispose()
}

func TestWoxScrollViewCanHideScrollbarWithoutDisablingScroll(t *testing.T) {
	props := ScrollViewProps{Key: "hidden-scrollbar", Width: 100, Height: 80, ContentHeight: 160, HideScrollbar: true}
	state := &scrollViewState{}
	state.InitState(woxwidget.StateContext{}, props)
	view := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture)
	view.OnScroll(woxui.Point{Y: -20})
	stack := view.Child.(woxwidget.Stack)

	if state.controller.Offset() != 20 || len(stack.Children) != 1 || state.visible || state.hideTimer != nil {
		t.Fatalf("hidden scrollbar = offset %.0f children %d visible %v timer %v, want 20/1/false/nil", state.controller.Offset(), len(stack.Children), state.visible, state.hideTimer)
	}
	state.Dispose()
}
