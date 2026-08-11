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
	labelViewport := launcherResultRowContent(row).Children[1].(woxwidget.Clip)
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
	tailContainer := launcherResultRowContent(row).Children[2].(woxwidget.Container)
	tailBoundary := tailContainer.Child.(woxwidget.Boundary[launcherResultTailsProps])
	tails := tailBoundary.Build(tailBoundary.Props).(woxwidget.ScrollView)

	if !tails.Horizontal || tails.Key != "launcher-result-tails-many-tags" {
		t.Fatalf("tail scroll = horizontal %v key %q, want true/stable result key", tails.Horizontal, tails.Key)
	}
	if tails.Width != 80 || tails.ContentWidth != 120 {
		t.Fatalf("tail scroll geometry = viewport %.0f content %.0f, want 80/120", tails.Width, tails.ContentWidth)
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
	icon := content.Children[0].(woxwidget.Container).Child.(woxwidget.Boundary[launcherResultIconProps])
	labelViewport := content.Children[1].(woxwidget.Clip)
	labels := labelViewport.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	title := labels.Children[0].(woxwidget.Boundary[launcherResultTextProps])
	subtitle := labels.Children[1].(woxwidget.Boundary[launcherResultTextProps])
	tails := content.Children[2].(woxwidget.Container).Child.(woxwidget.Boundary[launcherResultTailsProps])

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

func launcherResultRowContent(row woxwidget.Semantics) woxwidget.Flex {
	stack := row.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	return stack.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Flex)
}

func TestLauncherResultImageTailOverlaysCenteredSVGText(t *testing.T) {
	tails := launcherResultTails([]LauncherResultTail{{
		Image: &woxui.Image{Width: 192, Height: 36}, Width: 96, Height: 18,
		ImageText: "周 --", ImageTextColor: woxui.Color{R: 31, G: 41, B: 55, A: 255}, ImageTextSize: 9.5,
	}}, 106, 18, woxui.Color{}, false).(woxwidget.Clip)
	row := tails.Child.(woxwidget.Flex)
	item := row.Children[0].(woxwidget.Container)
	stack := item.Child.(woxwidget.Align).Child.(woxwidget.Stack)
	label := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Text)

	if label.Value != "周 --" || label.Style.Size != 9.5 || label.Color != (woxui.Color{R: 31, G: 41, B: 55, A: 255}) {
		t.Fatalf("image tail label = %#v", label)
	}
}
