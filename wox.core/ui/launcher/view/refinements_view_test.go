package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestRefinementControlsUseFlutterTranslucentFill(t *testing.T) {
	queryText := woxui.Color{R: 230, G: 235, B: 240, A: 48}
	resultTitle := woxui.Color{R: 210, G: 220, B: 230, A: 255}
	actionSelected := woxui.Color{R: 80, G: 90, B: 100, A: 255}
	controls := RefinementsView(RefinementsProps{
		Width:  400,
		Height: 44,
		Theme: woxcomponent.Theme{
			QueryText:      queryText,
			ResultTitle:    resultTitle,
			ResultSubtitle: woxui.Color{R: 120, G: 130, B: 140, A: 96},
			ActionSelected: actionSelected,
		},
		Groups: []RefinementGroup{{Title: "Type", Hotkey: "Cmd+T", Options: []RefinementOption{{Value: "all", Label: "All", Selected: true}, {Value: "text", Label: "Text"}}}},
	}).(woxwidget.Container)
	scroll := controls.Child.(woxwidget.ScrollView)
	content := scroll.Child.(woxwidget.Flex).Children[0].(woxwidget.Container)

	if !scroll.Horizontal || content.Width >= scroll.Width {
		t.Fatalf("refinement strip = %#v with shell width %v, want shrink-wrapped horizontal scroller", scroll, content.Width)
	}
	if content.Color != (woxui.Color{R: 210, G: 220, B: 230, A: 9}) {
		t.Fatalf("refinement fill = %#v, want result title color at Flutter 0.035 alpha", content.Color)
	}
	if content.BorderWidth != 1 || content.BorderColor.A != 31 {
		t.Fatalf("refinement border = %#v at %v, want a translucent 1px stroke", content.BorderColor, content.BorderWidth)
	}
	group := content.Child.(woxwidget.Flex)
	if group.CrossAxisAlignment != woxwidget.CrossAxisCenter || len(group.Children) != 11 {
		t.Fatalf("refinement group = %#v, want title, two options, dividers, and hotkey", group)
	}
	optionStateful := group.Children[3].(woxwidget.Stateful)
	optionGesture := optionStateful.CreateState().Build(woxwidget.StateContext{}, optionStateful.Widget).(woxwidget.Gesture)
	option := optionGesture.Child.(woxwidget.Container)
	optionAlignment, ok := option.Child.(woxwidget.Align)
	if !ok || optionAlignment.Horizontal != 0.5 || optionAlignment.Vertical != 0.5 {
		t.Fatalf("refinement option alignment = %#v, want centered on both axes", option.Child)
	}
	if optionGesture.OnHoverAt == nil {
		t.Fatal("refinement option gesture has no hover handler, want shared hover state")
	}
	if option.Color != (woxui.Color{R: 80, G: 90, B: 100, A: 56}) {
		t.Fatalf("selected option fill = %#v, want Flutter action active color at 0.22 alpha", option.Color)
	}
	hotkey := group.Children[len(group.Children)-2].(woxwidget.Text)
	if hotkey.Value != "Cmd+T" || hotkey.Style.Size != 11 {
		t.Fatalf("refinement hotkey = %#v, want Flutter inline chord", hotkey)
	}
	if color := refinementColorWithOpacity(queryText, 0.075); color.A != 19 {
		t.Fatalf("refinement alpha = %d, want Flutter absolute 0.075 alpha", color.A)
	}
}

func TestRefinementToggleOwnsSharedHoverState(t *testing.T) {
	queryText := woxui.Color{R: 230, G: 235, B: 240, A: 48}
	toggle := RefinementToggle(RefinementsProps{
		Summary: "Filters", DefaultLabel: "Filters", Theme: woxcomponent.Theme{QueryText: queryText}, OnToggle: func() {},
	}).(woxwidget.Stateful)
	gesture := toggle.CreateState().Build(woxwidget.StateContext{}, toggle.Widget).(woxwidget.Gesture)
	if gesture.OnTap == nil || gesture.OnHoverAt == nil {
		t.Fatalf("refinement toggle gesture = tap %v hover %v, want shared hover state", gesture.OnTap != nil, gesture.OnHoverAt != nil)
	}
}

func TestRefinementToggleForwardsHotkeyTooltip(t *testing.T) {
	var gotInside bool
	var gotText string
	toggle := RefinementToggle(RefinementsProps{
		Summary: "Filters", DefaultLabel: "Filters", Tooltip: "Filter search results (Ctrl+F)",
		Theme:    woxcomponent.Theme{QueryText: woxui.Color{R: 230, G: 235, B: 240, A: 48}},
		OnToggle: func() {},
		OnTooltip: func(inside bool, text string, _ woxui.Rect) {
			gotInside = inside
			gotText = text
		},
	}).(woxwidget.Stateful)
	gesture := toggle.CreateState().Build(woxwidget.StateContext{}, toggle.Widget).(woxwidget.Gesture)
	gesture.OnHoverAt(true, woxui.Rect{Width: 80, Height: 26})
	if !gotInside || gotText != "Filter search results (Ctrl+F)" {
		t.Fatalf("refinement tooltip = inside %v text %q, want hover with hotkey hint", gotInside, gotText)
	}
}

func TestRefinementBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, RefinementOption{})
	woxwidget.AssertEqualCoversAllFields(t, RefinementGroup{})
	woxwidget.AssertEqualCoversAllFields(t, RefinementsProps{})
}
