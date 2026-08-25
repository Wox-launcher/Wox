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
	titleBoundary := row.Child.(woxwidget.Boundary[launcherResultTextProps])
	label := titleBoundary.Build(titleBoundary.Props).(woxwidget.Text)

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

func TestLauncherResultWiresSecondaryTap(t *testing.T) {
	tapped := false
	result := LauncherResultsView(LauncherResultsProps{
		Width: 320, Height: 50, ContentHeight: 50, RowHeight: 50,
		Items: []LauncherResultItem{{ID: "result", Title: "Result", OnSecondaryTapDown: func() { tapped = true }}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	gesture := row.Child.(woxwidget.Gesture)

	gesture.OnSecondaryTapDown(woxui.Point{})
	if !tapped {
		t.Fatal("secondary tap callback was not wired to the result row")
	}
}

func TestLauncherResultWithoutSubtitleCentersTitleVertically(t *testing.T) {
	result := LauncherResultsView(LauncherResultsProps{
		Width: 320, Height: 50, ContentHeight: 50, RowHeight: 50,
		Items: []LauncherResultItem{{ID: "no-subtitle", Title: "Everything"}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	content := launcherResultRowContent(row)
	if len(content.Children) != 2 {
		t.Fatalf("row children = %d, want icon and label without a zero-width tail slot", len(content.Children))
	}
	labelViewport := content.Children[1].(woxwidget.Clip)
	label := labelViewport.Child.(woxwidget.Container).Child.(woxwidget.Align)

	if label.Vertical != 0.5 || label.Height != 50 {
		t.Fatalf("title alignment = vertical %.1f height %.0f, want 0.5/50", label.Vertical, label.Height)
	}
}

func TestLauncherResultMultilineSubtitleUsesSingleLineCenteredGroup(t *testing.T) {
	result := LauncherResultsView(LauncherResultsProps{
		Width: 320, Height: 50, ContentHeight: 50, RowHeight: 50,
		Items: []LauncherResultItem{{ID: "multiline-subtitle", Title: "Reinstall plugin", Subtitle: "Version: 1.0\nDescription: details"}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	labelContainer := launcherResultRowContent(row).Children[1].(woxwidget.Clip).Child.(woxwidget.Container)
	label := labelContainer.Child.(woxwidget.Align)
	labels := label.Child.(woxwidget.Flex)
	subtitle := labels.Children[1].(woxwidget.Boundary[launcherResultTextProps])

	if label.Vertical != 0.5 || subtitle.Props.Value != "Version: 1.0" {
		t.Fatalf("multiline subtitle layout = vertical %.1f value %q, want 0.5/first line", label.Vertical, subtitle.Props.Value)
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
	tailAlignment := launcherResultRowContent(row).Children[2].(woxwidget.Align)
	tailBoundary := tailAlignment.Child.(woxwidget.Boundary[launcherResultTailsProps])
	if tailAlignment.Vertical != 0.5 || tailAlignment.Height != 50 {
		t.Fatalf("tail alignment = %#v, want full-height vertical center", tailAlignment)
	}
	tails := tailBoundary.Build(tailBoundary.Props).(woxwidget.ScrollView)

	if !tails.Horizontal || tails.Key != "launcher-result-tails-many-tags" {
		t.Fatalf("tail scroll = horizontal %v key %q, want true/stable result key", tails.Horizontal, tails.Key)
	}
	if tails.Width != 80 || tails.ContentWidth != 120 || tails.InitialOffset != 40 {
		t.Fatalf("tail scroll geometry = viewport %.0f content %.0f offset %.0f, want 80/120 starting at the right edge", tails.Width, tails.ContentWidth, tails.InitialOffset)
	}
	if tails.KeepVisible == nil || tails.KeepVisible.Start != 40 || tails.KeepVisible.End != 120 {
		t.Fatalf("tail keep-visible = %#v, want the overflowing right edge", tails.KeepVisible)
	}
}

func TestLauncherResultTailsAlignToTheRightWhenTheyFit(t *testing.T) {
	result := LauncherResultsView(LauncherResultsProps{
		Width: 300, Height: 50, ContentHeight: 50, RowHeight: 50,
		Theme: woxcomponent.Theme{ResultTitle: woxui.Color{A: 255}},
		Items: []LauncherResultItem{{
			ID: "short-tags", Title: "Result", TailWidth: 120, TailHeight: 22,
			Tails: []LauncherResultTail{{Text: "cpu", Width: 40, Height: 22}},
		}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	tailBoundary := launcherResultRowContent(row).Children[2].(woxwidget.Align).Child.(woxwidget.Boundary[launcherResultTailsProps])
	aligned := tailBoundary.Build(tailBoundary.Props).(woxwidget.Align)
	pinned := aligned.Child.(woxwidget.Container)
	if aligned.Width != 120 || aligned.Horizontal != 1 || pinned.Width != 50 {
		t.Fatalf("fitting tails = slot %.0f horizontal %.1f content %.0f, want a right-aligned 50-wide tag in a 120 slot", aligned.Width, aligned.Horizontal, pinned.Width)
	}
}

func TestLauncherResultBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, launcherResultBackgroundProps{})
	woxwidget.AssertEqualCoversAllFields(t, launcherResultIconProps{})
	woxwidget.AssertEqualCoversAllFields(t, launcherResultTextProps{})
	woxwidget.AssertEqualCoversAllFields(t, launcherResultTailsProps{})
}

func TestLauncherResultUsesIndependentUpdateBoundaries(t *testing.T) {
	result := LauncherResultsView(LauncherResultsProps{
		Width: 300, Height: 50, ContentHeight: 50, RowHeight: 50,
		Theme: woxcomponent.Theme{ResultTitle: woxui.Color{A: 255}},
		Items: []LauncherResultItem{{
			ID: "live", Title: "Title", Subtitle: "Subtitle", Icon: &woxui.Image{Width: 28, Height: 28}, TailWidth: 60, TailHeight: 22,
			Tails: []LauncherResultTail{{Text: "1%", Width: 60, Height: 22}},
		}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	rowStack := row.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	background := rowStack.Children[0].Child.(woxwidget.Boundary[launcherResultBackgroundProps])
	content := launcherResultRowContent(row)
	icon := content.Children[0].(woxwidget.Align).Child.(woxwidget.Boundary[launcherResultIconProps])
	labelViewport := content.Children[1].(woxwidget.Clip)
	labels := labelViewport.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	title := labels.Children[0].(woxwidget.Boundary[launcherResultTextProps])
	subtitle := labels.Children[1].(woxwidget.Boundary[launcherResultTextProps])
	tails := content.Children[2].(woxwidget.Align).Child.(woxwidget.Boundary[launcherResultTailsProps])

	want := []woxwidget.Key{
		LauncherResultBackgroundBoundaryKey("live"), LauncherResultIconBoundaryKey("live"), LauncherResultTitleBoundaryKey("live"),
		LauncherResultSubtitleBoundaryKey("live"), LauncherResultTailsBoundaryKey("live"),
	}
	got := []woxwidget.Key{background.Key, icon.Key, title.Key, subtitle.Key, tails.Key}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("result boundary keys = %v, want %v", got, want)
		}
	}
}

func TestLauncherResultTailWidthDoesNotChangeLabelBoundaryConstraints(t *testing.T) {
	labelGeometry := func(tailWidth float32) (float32, float32) {
		result := LauncherResultsView(LauncherResultsProps{
			Width: 300, Height: 50, ContentHeight: 50, RowHeight: 50,
			Items: []LauncherResultItem{{ID: "live", Title: "Title", TailWidth: tailWidth}},
		}).(woxwidget.Semantics)
		listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
		row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
		viewport := launcherResultRowContent(row).Children[1].(woxwidget.Clip)
		content := viewport.Child.(woxwidget.Container)
		return viewport.Width, content.Width
	}

	firstViewport, firstContent := labelGeometry(40)
	secondViewport, secondContent := labelGeometry(80)
	if firstViewport == secondViewport || firstContent != secondContent {
		t.Fatalf("label widths = viewport %.0f/%.0f content %.0f/%.0f, want changing viewport and stable content", firstViewport, secondViewport, firstContent, secondContent)
	}
}

func TestLauncherResultShowsQuickSelectBadge(t *testing.T) {
	fill := woxui.Color{R: 180, G: 180, B: 190, A: 255}
	text := woxui.Color{R: 24, G: 29, B: 38, A: 255}
	result := LauncherResultsView(LauncherResultsProps{
		Width: 320, Height: 50, ContentHeight: 50, RowHeight: 50,
		TailColor: fill, Theme: woxcomponent.Theme{Background: text, ResultTitle: woxui.Color{A: 255}},
		Items: []LauncherResultItem{{ID: "one", Title: "One", QuickSelectNumber: "1"}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	content := launcherResultRowContent(row)
	if len(content.Children) != 3 {
		t.Fatalf("row children = %d, want icon, label, and quick select badge", len(content.Children))
	}
	badgeSlot := content.Children[2].(woxwidget.Align).Child.(woxwidget.Container)
	chip := badgeSlot.Child.(woxwidget.Container)
	label := chip.Child.(woxwidget.Align).Child.(woxwidget.Text)
	if badgeSlot.Width != 35 || chip.Width != 20 || chip.Height != 20 || chip.Radius != 4 {
		t.Fatalf("quick select badge geometry = slot %.0f chip %.0fx%.0f radius %.0f, want Flutter 35/20/4", badgeSlot.Width, chip.Width, chip.Height, chip.Radius)
	}
	if chip.Color != fill || label.Value != "1" || label.Color != text || label.Style.Size != 11 || label.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("quick select badge style = fill %#v text %q color %#v style %#v", chip.Color, label.Value, label.Color, label.Style)
	}
	if row.Value != "1" {
		t.Fatalf("quick select result value = %q, want the visible number", row.Value)
	}
}

func TestLauncherResultSelectedQuickSelectKeepsReadableDigit(t *testing.T) {
	background := woxui.Color{R: 24, G: 29, B: 38, A: 255}
	selectedFill := woxui.Color{R: 245, G: 245, B: 245, A: 255}
	result := LauncherResultsView(LauncherResultsProps{
		Width: 320, Height: 50, ContentHeight: 50, RowHeight: 50,
		SelectedTailColor: selectedFill,
		Theme:             woxcomponent.Theme{Background: background, SelectedBackground: selectedFill, ResultTitle: woxui.Color{A: 255}},
		Items:             []LauncherResultItem{{ID: "one", Title: "One", Selected: true, QuickSelectNumber: "1"}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	badgeSlot := launcherResultRowContent(row).Children[2].(woxwidget.Align).Child.(woxwidget.Container)
	label := badgeSlot.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Text)
	if label.Color != background {
		t.Fatalf("selected quick select digit = %#v, want the window background so it stays readable on a light chip", label.Color)
	}
}

func TestLauncherResultTrailingClusterKeepsBadgeOnTheRightEdge(t *testing.T) {
	result := LauncherResultsView(LauncherResultsProps{
		Width: 320, Height: 50, ContentHeight: 50, RowHeight: 50, ItemPadding: woxwidget.Insets{Left: 8, Right: 8},
		Theme: woxcomponent.Theme{ResultTitle: woxui.Color{A: 255}, Background: woxui.Color{A: 255}},
		Items: []LauncherResultItem{{
			ID: "tagged", Title: "QQ Music", TailWidth: 80, TailHeight: 22, QuickSelectNumber: "2",
			Tails: []LauncherResultTail{{Text: "CPU", Width: 40, Height: 22}, {Text: "MEM", Width: 40, Height: 22}},
		}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	content := launcherResultRowContent(row)
	if len(content.Children) != 3 {
		t.Fatalf("row children = %d, want icon, label, and one trailing cluster", len(content.Children))
	}
	cluster := content.Children[2].(woxwidget.Container)
	if cluster.Width != 115 {
		t.Fatalf("trailing cluster width = %.0f, want tails plus badge without an extra flex gap", cluster.Width)
	}
	children := cluster.Child.(woxwidget.Flex).Children
	if len(children) != 2 {
		t.Fatalf("trailing children = %d, want tails then badge", len(children))
	}
	badge := children[1].(woxwidget.Align)
	if badge.Width != 35 {
		t.Fatalf("trailing badge width = %.0f, want the 35-wide quick select slot", badge.Width)
	}
}

func launcherResultRowContent(row woxwidget.Semantics) woxwidget.Flex {
	stack := row.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	return stack.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Flex)
}

func TestLauncherResultTailHoverUsesTooltipAndKeepsRowActions(t *testing.T) {
	var hovered bool
	var tooltip string
	var anchor woxui.Rect
	tapped := false
	result := LauncherResultsView(LauncherResultsProps{
		Width: 320, Height: 50, ContentHeight: 50, RowHeight: 50,
		Theme: woxcomponent.Theme{ResultTitle: woxui.Color{A: 255}},
		Items: []LauncherResultItem{{
			ID: "perf", Title: "Result", TailWidth: 160, TailHeight: 22,
			Tails:     []LauncherResultTail{{Text: "B1", Width: 40, Height: 22, Tooltip: "First batch flush tick: 5.0ms"}, {Text: "1ms", Width: 40, Height: 22}},
			OnSelect:  func() { tapped = true },
			OnTooltip: func(inside bool, text string, bounds woxui.Rect) { hovered, tooltip, anchor = inside, text, bounds },
		}},
	}).(woxwidget.Semantics)
	listScroll := result.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	tailBoundary := launcherResultRowContent(row).Children[2].(woxwidget.Align).Child.(woxwidget.Boundary[launcherResultTailsProps])
	aligned := tailBoundary.Build(tailBoundary.Props).(woxwidget.Align)
	rowFlex := aligned.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	batch := rowFlex.Children[0].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Semantics)
	if batch.AutomationID != "result-tail-perf-0" || batch.Label != "B1" || batch.Description != "First batch flush tick: 5.0ms" {
		t.Fatalf("batch tail semantics = %+v, want a hoverable performance tooltip target", batch)
	}
	wantAnchor := woxui.Rect{X: 4, Y: 6, Width: 40, Height: 22}
	batch.Child.(woxwidget.Gesture).OnHoverAt(true, wantAnchor)
	if !hovered || tooltip != "First batch flush tick: 5.0ms" || anchor != wantAnchor {
		t.Fatalf("hover = %v, %q, %#v; want tooltip and anchor", hovered, tooltip, anchor)
	}
	batch.Child.(woxwidget.Gesture).OnTap()
	if !tapped {
		t.Fatal("tooltip tail should still activate the result row")
	}
	plain := rowFlex.Children[1].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Container)
	if plain.Child == nil {
		t.Fatal("tails without tooltip should stay unwrapped")
	}
}

func TestLauncherResultImageTailOverlaysCenteredSVGText(t *testing.T) {
	tails := launcherResultTails([]LauncherResultTail{{
		Image: &woxui.Image{Width: 192, Height: 36}, Width: 96, Height: 18,
		ImageText: "周 --", ImageTextColor: woxui.Color{R: 31, G: 41, B: 55, A: 255}, ImageTextSize: 9.5,
	}}, 106, 18, woxui.Color{}, false).(woxwidget.Align)
	row := tails.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	item := row.Children[0].(woxwidget.Container)
	stack := item.Child.(woxwidget.Align).Child.(woxwidget.Stack)
	label := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Text)

	if label.Value != "周 --" || label.Style.Size != 9.5 || label.Color != (woxui.Color{R: 31, G: 41, B: 55, A: 255}) {
		t.Fatalf("image tail label = %#v", label)
	}
}
