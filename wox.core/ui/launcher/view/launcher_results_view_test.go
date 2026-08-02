package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLauncherResultGroupUsesFlutterTitleTypography(t *testing.T) {
	titleColor := woxui.Color{R: 240, G: 242, B: 244, A: 255}
	result := LauncherResultsView(LauncherResultsProps{
		Width:         320,
		Height:        50,
		ContentHeight: 50,
		RowHeight:     50,
		Theme: woxcomponent.Theme{
			ResultTitle:    titleColor,
			ResultSubtitle: woxui.Color{R: 120, G: 125, B: 130, A: 255},
		},
		Items: []LauncherResultItem{{Title: "Today", Group: true}},
	}).(woxwidget.Semantics)
	scrollGesture := result.Child.(woxwidget.Gesture)
	stack := scrollGesture.Child.(woxwidget.Stack)
	scroll := stack.Children[0].Child.(woxwidget.ScrollView)
	content := scroll.Child.(woxwidget.Container)
	row := content.Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	label := row.Child.(woxwidget.Text)

	if label.Color != titleColor {
		t.Fatalf("group title color = %#v, want Flutter result title color %#v", label.Color, titleColor)
	}
	if label.Style.Size != 15 || label.Style.Weight != woxui.FontWeightRegular {
		t.Fatalf("group title style = %#v, want Flutter 15px normal result title typography", label.Style)
	}
}

func TestLauncherResultsExposeCompletionState(t *testing.T) {
	result := LauncherResultsView(LauncherResultsProps{Width: 320, Height: 50, ContentHeight: 50, RowHeight: 50, Complete: true}).(woxwidget.Semantics)
	if result.Value != "complete" || !result.ReadOnly {
		t.Fatalf("result completion semantics = value %q readonly %v", result.Value, result.ReadOnly)
	}
}

func TestLauncherResultTailsScrollHorizontallyWhenClipped(t *testing.T) {
	result := LauncherResultsView(LauncherResultsProps{
		Width: 300, Height: 50, ContentHeight: 50, RowHeight: 50,
		Theme: woxcomponent.Theme{ResultTitle: woxui.Color{A: 255}},
		Items: []LauncherResultItem{{
			ID: "many-tags", Title: "Result", TailWidth: 80, TailHeight: 22,
			Tails: []LauncherResultTail{{Text: "first", Width: 50, Height: 22}, {Text: "second", Width: 50, Height: 22}},
		}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	tailContainer := row.Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Container)
	tails := tailContainer.Child.(woxwidget.ScrollView)

	if !tails.Horizontal || tails.Key != "launcher-result-tails-many-tags" {
		t.Fatalf("tail scroll = horizontal %v key %q, want true/stable result key", tails.Horizontal, tails.Key)
	}
	if tails.Width != 80 || tails.ContentWidth != 120 {
		t.Fatalf("tail scroll geometry = viewport %.0f content %.0f, want 80/120", tails.Width, tails.ContentWidth)
	}
}

func TestLauncherResultScrollbarOpacityFollowsScrollActivity(t *testing.T) {
	props := launcherResultScrollProps{Width: 100, Height: 80, ContentHeight: 160, ThumbColor: woxui.Color{A: 255}}
	state := &launcherResultScrollState{}
	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	animation := stack.Children[1].Child.(woxwidget.AnimatedFloat)
	if animation.Target != 0 {
		t.Fatalf("idle scrollbar opacity target = %.0f, want 0", animation.Target)
	}

	state.visible = true
	stack = state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	thumbGesture := stack.Children[1].Child.(woxwidget.Gesture)
	animation = thumbGesture.Child.(woxwidget.AnimatedFloat)
	if animation.Target != 1 {
		t.Fatalf("active scrollbar opacity target = %.0f, want 1", animation.Target)
	}
	widthAnimation := animation.Builder(0.5).(woxwidget.AnimatedFloat)
	thumb := widthAnimation.Builder(3).(woxwidget.Align).Child.(woxwidget.Container)
	if thumb.Color.A != 75 || thumb.Width != 3 {
		t.Fatalf("half-faded scrollbar = alpha %d width %.0f, want 75/3", thumb.Color.A, thumb.Width)
	}

	state.hovered = true
	stack = state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	animation = stack.Children[1].Child.(woxwidget.Gesture).Child.(woxwidget.AnimatedFloat)
	widthAnimation = animation.Builder(1).(woxwidget.AnimatedFloat)
	if widthAnimation.Target != 7 {
		t.Fatalf("hovered scrollbar width target = %.0f, want 7", widthAnimation.Target)
	}
	state.Dispose()
}

func TestLauncherResultScrollbarOwnsActivityHoverAndDrag(t *testing.T) {
	scrolled := float32(0)
	props := launcherResultScrollProps{Width: 100, Height: 80, ContentHeight: 160, Offset: 0, OnScroll: func(delta float32) { scrolled += delta }}
	state := &launcherResultScrollState{}
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
	if !state.hovered || state.hideTimer != nil {
		t.Fatalf("hovered scrollbar = hovered %v timer %v, want true/nil", state.hovered, state.hideTimer)
	}
	thumb.OnPanStart(woxui.Point{Y: 10})
	thumb.OnPanUpdate(woxui.Point{Y: 20})
	if scrolled != 40 {
		t.Fatalf("thumb drag delta = %.0f, want 40", scrolled)
	}
	thumb.OnHover(false)
	if state.hovered || !state.dragging || state.hideTimer != nil {
		t.Fatalf("drag outside = hovered %v dragging %v timer %v, want false/true/nil", state.hovered, state.dragging, state.hideTimer)
	}
	stack = state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	animation := stack.Children[1].Child.(woxwidget.Gesture).Child.(woxwidget.AnimatedFloat)
	if width := animation.Builder(1).(woxwidget.AnimatedFloat).Target; width != 7 {
		t.Fatalf("dragging scrollbar width target = %.0f, want 7", width)
	}
	thumb.OnPanEnd()
	if state.dragging || state.hideTimer == nil {
		t.Fatalf("ended drag = dragging %v timer %v, want false/non-nil", state.dragging, state.hideTimer)
	}
	state.Dispose()
}

func TestLauncherResultScrollbarShowsForKeyboardOffsetMovement(t *testing.T) {
	state := &launcherResultScrollState{}
	oldProps := launcherResultScrollProps{Height: 80, ContentHeight: 160}
	newProps := oldProps
	newProps.Offset = 20
	state.DidUpdateWidget(woxwidget.StateContext{}, oldProps, newProps)
	if !state.visible || state.hideTimer == nil {
		t.Fatalf("keyboard offset movement = visible %v timer %v, want true/non-nil", state.visible, state.hideTimer)
	}
	state.Dispose()
}
