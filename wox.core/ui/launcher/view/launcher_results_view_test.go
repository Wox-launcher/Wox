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
	row := buildLauncherResultBoundary(content.Child.(woxwidget.Flex).Children[0]).(woxwidget.Container)
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
	row := buildLauncherResultBoundary(listScroll.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0]).(woxwidget.Semantics)
	tailContainer := row.Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Container)
	tails := tailContainer.Child.(woxwidget.ScrollView)

	if !tails.Horizontal || tails.Key != "launcher-result-tails-many-tags" {
		t.Fatalf("tail scroll = horizontal %v key %q, want true/stable result key", tails.Horizontal, tails.Key)
	}
	if tails.Width != 80 || tails.ContentWidth != 120 {
		t.Fatalf("tail scroll geometry = viewport %.0f content %.0f, want 80/120", tails.Width, tails.ContentWidth)
	}
}

func TestLauncherResultBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, LauncherResultItem{})
	woxwidget.AssertEqualCoversAllFields(t, launcherResultRowProps{})
	woxwidget.AssertEqualCoversAllFields(t, LauncherResultsProps{})
}

func TestLauncherResultHoverRebuildsOnlyChangedRows(t *testing.T) {
	hovered := 0
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		items := []LauncherResultItem{
			{ID: "first", Revision: 1, Title: "First", Hovered: hovered == 0},
			{ID: "second", Revision: 2, Title: "Second", Hovered: hovered == 1},
			{ID: "third", Revision: 3, Title: "Third", Hovered: hovered == 2},
		}
		return LauncherResultsBoundary(LauncherResultsProps{
			Width: 320, Height: 150, ContentHeight: 150, RowHeight: 50, Items: items,
			Theme: woxcomponent.Theme{ResultTitle: woxui.Color{A: 255}, SelectedBackground: woxui.Color{R: 50, G: 100, B: 200, A: 255}},
		})
	})
	host.AttachServices(settingsWindowHostServices{})
	if err := host.SetRepaintDebugMode(woxwidget.RepaintDebugRainbow); err != nil {
		t.Fatal(err)
	}
	render := func() int {
		displayList := &woxui.DisplayList{}
		host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 320, Height: 150}, PixelSize: woxui.PixelSize{Width: 320, Height: 150}, Scale: 1})
		return displayList.CommandCount()
	}
	render()
	stableCommands := render()
	hovered = 1
	movedCommands := render()
	if movedCommands != stableCommands+3 {
		t.Fatalf("hover move commands = %d, stable = %d; want three repaint outlines for section and two changed rows", movedCommands, stableCommands)
	}
}

func buildLauncherResultBoundary(value woxwidget.Widget) woxwidget.Widget {
	boundary := value.(woxwidget.Boundary[launcherResultRowProps])
	return boundary.Build(boundary.Props)
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
