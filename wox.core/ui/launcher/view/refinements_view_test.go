package view

import (
	"strconv"
	"strings"
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
	scroll, scrollContent := refinementScroll(t, controls.Child)
	content := scrollContent.(woxwidget.Flex).Children[0].(woxwidget.Container)

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

func TestRefinementStripUsesSharedHorizontalScrollbar(t *testing.T) {
	groups := []RefinementGroup{
		{Title: "Unread", Options: []RefinementOption{{Value: "unread", Label: "Unread only"}}},
		{Title: "Type", Options: []RefinementOption{{Value: "issue", Label: "Issue"}, {Value: "pr", Label: "Pull Request"}, {Value: "release", Label: "Release"}}},
		{Title: "Repository", Options: []RefinementOption{{Value: "wox", Label: "Wox-launcher/Wox"}, {Value: "other", Label: "other/repo"}}},
	}
	controls := RefinementsView(RefinementsProps{
		Width:  320,
		Height: 44,
		Theme:  woxcomponent.Theme{ResultTitle: woxui.Color{R: 210, G: 220, B: 230, A: 255}},
		Groups: groups,
	}).(woxwidget.Container)
	props, _ := refinementScroll(t, controls.Child)
	if !props.Horizontal || props.AlwaysShowScrollbar || props.Key != "launcher-refinements-scroll" {
		t.Fatalf("refinement scroll = %#v, want hover-revealed horizontal WoxScrollView", props)
	}
	if props.ContentWidth <= props.Width {
		t.Fatalf("refinement content width = %v, want greater than viewport %v", props.ContentWidth, props.Width)
	}
	if props.ThumbColor != (woxui.Color{R: 210, G: 220, B: 230, A: 255}) {
		t.Fatalf("refinement thumb = %#v, want result title color", props.ThumbColor)
	}
}

func TestRefinementDefaultStaysAtRightWhileFiltersScroll(t *testing.T) {
	const width, height, density = 380, 55, 1.25
	groups := make([]RefinementGroup, 8)
	for i := range groups {
		groups[i] = RefinementGroup{ID: strconv.Itoa(i), Title: "Filter", Current: "All"}
	}
	visibleGroups, clicked := groups[:1], false
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return RefinementsView(RefinementsProps{
			Width: width, Height: height, DensityScale: density, Groups: visibleGroups,
			DefaultButton: &RefinementDefaultButtonProps{Label: "Save default", Hotkey: "Ctrl+Shift+D", OnToggle: func() { clicked = true }},
		})
	})
	defer host.Dispose()
	host.AttachServices(&queryPointerHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: width, Height: height}, PixelSize: woxui.PixelSize{Width: 475, Height: 69}, Scale: density}
	bounds := func(key woxwidget.Key) woxui.Rect {
		t.Helper()
		rect, found := host.BoundsForKey(key)
		if !found {
			t.Fatalf("missing widget %q", key)
		}
		return rect
	}
	host.Frame(&woxui.DisplayList{}, frame)
	bookmark := bounds("refinement-default")
	if right := bookmark.X + bookmark.Width; right != width-scaledLauncherSize(8, density) {
		t.Fatalf("bookmark right edge = %v, want right edge of the padded bar", right)
	}

	visibleGroups = groups
	host.Frame(&woxui.DisplayList{}, frame)
	viewport := bounds("launcher-refinements-scroll-viewport")
	if viewport.X+viewport.Width >= bookmark.X {
		t.Fatalf("filter viewport %v overlaps bookmark %v", viewport, bookmark)
	}
	first := bounds(RefinementGroupKey(groups[0].ID))
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: viewport.X + 5, Y: viewport.Y + 5}, Scroll: woxui.Point{X: -40}})
	host.Frame(&woxui.DisplayList{}, frame)
	if after := bounds(RefinementGroupKey(groups[0].ID)); after.X >= first.X {
		t.Fatalf("filter did not scroll left, before %v, after %v", first, after)
	}
	if after := bounds("refinement-default"); after != bookmark {
		t.Fatalf("bookmark moved from %v to %v after adding and scrolling filters", bookmark, after)
	}
	position := woxui.Point{X: bookmark.X + bookmark.Width/2, Y: bookmark.Y + bookmark.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Position: position, Button: woxui.PointerButtonPrimary})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Position: position, Button: woxui.PointerButtonPrimary})
	if !clicked {
		t.Fatal("bookmark must remain clickable while filters overflow")
	}
}

func TestRefinementToggleOwnsSharedHoverState(t *testing.T) {
	queryText := woxui.Color{R: 230, G: 235, B: 240, A: 48}
	toggle := RefinementToggle(RefinementsProps{
		DefaultLabel: "Filters", Theme: woxcomponent.Theme{QueryText: queryText}, OnToggle: func() {},
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
		DefaultLabel: "Filters", Tooltip: "Filter search results (Ctrl+F)",
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

func TestRefinementToggleEllipsizesSummaryAndShowsFullTextOnHover(t *testing.T) {
	const shortcut = "Filter search results (Ctrl+F)"
	var tooltip string
	props := RefinementsProps{
		SummaryLabels: []string{"entretenimiento", "China", "中文广播"}, DefaultLabel: "Filters", DensityScale: 1.25, Tooltip: shortcut,
		OnTooltip: func(_ bool, text string, _ woxui.Rect) { tooltip = text },
	}
	toggle := RefinementToggle(props).(woxwidget.Stateful)
	gesture := toggle.CreateState().Build(woxwidget.StateContext{}, toggle.Widget).(woxwidget.Gesture)
	button := gesture.Child.(woxwidget.Container).Child.(woxwidget.Container)
	if button.Width != scaledLauncherSize(192, 1.25) {
		t.Fatalf("button width = %v, want the scaled maximum", button.Width)
	}
	content := button.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	padding, gap, iconSize := refinementToggleHorizontalMetrics(props.DensityScale)
	budget := button.Width - padding.Left - padding.Right - gap - iconSize
	layout := content.Children[1].(woxwidget.Flexible).Child.(woxwidget.LayoutBuilder)
	parts := layout.Build(woxui.Size{Width: budget, Height: button.Height}).(woxwidget.Flex).Children
	if len(parts) != 5 {
		t.Fatalf("got %d parts, want three values and two separators", len(parts))
	}
	total := float32(0)
	for i, part := range parts {
		label := part.(woxwidget.TextBlock)
		total += label.Width
		if i%2 != 0 {
			continue
		}
		fullWidth := refinementTextWidth(nil, label.Value, label.Style)
		if label.Value != props.SummaryLabels[i/2] || !label.HardEllipsis || label.MaxLines != 1 ||
			(i == 0 && label.Width >= fullWidth) || (i > 0 && label.Width != fullWidth) {
			t.Fatalf("button label %d = %#v, want only the long value truncated", i/2, label)
		}
	}
	if total > budget+0.01 {
		t.Fatalf("summary width = %v, exceeds text budget %v", total, budget)
	}
	gesture.OnHoverAt(true, woxui.Rect{Width: button.Width, Height: button.Height})
	if tooltip != strings.Join(props.SummaryLabels, ", ")+"\n"+shortcut {
		t.Fatalf("tooltip = %q, want the full summary and shortcut", tooltip)
	}
}

func TestRefinementBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, RefinementOption{})
	woxwidget.AssertEqualCoversAllFields(t, RefinementGroup{})
	woxwidget.AssertEqualCoversAllFields(t, RefinementsProps{})
}

// TestRefinementOptionTransparentStates verifies selected and hover fallbacks cannot replace authored transparency.
func TestRefinementOptionTransparentStates(t *testing.T) {
	transparent := woxui.Color{}
	theme := woxcomponent.Theme{
		ResultTitle: woxui.Color{R: 255, A: 255}, ActionSelected: woxui.Color{G: 255, A: 255},
		RefinementItemFontColor: &transparent, RefinementItemBackgroundColor: &transparent,
		RefinementItemHoverBackgroundColor: &transparent, RefinementItemActiveFontColor: &transparent,
		RefinementItemActiveBackgroundColor: &transparent, RefinementItemActiveHoverBackgroundColor: &transparent,
	}
	for _, selected := range []bool{false, true} {
		view, _ := refinementOption(RefinementOption{Value: "all", Selected: selected}, theme, nil, 1)
		stateful := view.(woxwidget.Stateful)
		state := stateful.CreateState()
		for _, hovered := range []bool{false, true} {
			gesture := state.Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Gesture)
			gesture.OnHoverAt(hovered, woxui.Rect{})
			gesture = state.Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Gesture)
			container := gesture.Child.(woxwidget.Container)
			label := container.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.Text)
			if container.Color.A != 0 || label.Color.A != 0 {
				t.Fatal("refinement option lost explicit transparency")
			}
		}
	}
}

func refinementScroll(t *testing.T, view woxwidget.Widget) (woxcomponent.ScrollViewProps, woxwidget.Widget) {
	t.Helper()
	switch typed := view.(type) {
	case woxwidget.Stateful:
		props := typed.Widget.(woxcomponent.ScrollViewProps)
		return props, props.Content
	case woxwidget.Gesture:
		scroll := typed.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
		return woxcomponent.ScrollViewProps{Width: scroll.Width, ContentWidth: scroll.ContentWidth, Horizontal: scroll.Horizontal}, scroll.Child
	default:
		t.Fatalf("refinement scroll = %T, want WoxScrollView", view)
		return woxcomponent.ScrollViewProps{}, nil
	}
}
