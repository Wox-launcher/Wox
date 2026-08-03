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
