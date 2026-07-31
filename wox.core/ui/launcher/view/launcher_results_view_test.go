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
