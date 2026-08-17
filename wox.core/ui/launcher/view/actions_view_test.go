package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestActionsBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, ActionItem{})
	woxwidget.AssertEqualCoversAllFields(t, ActionsProps{})
}

func TestActionsEmptyStateCentersSearchIconAndMessage(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, ActionHeader: woxui.Color{A: 255}, NoMatchesLabel: "No matches",
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	panel := view.Child.(woxwidget.Container)
	content := panel.Child.(woxwidget.Flex)
	actionList := content.Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	empty := actionList.Content.(woxwidget.Flex).Children[0].(woxwidget.Align)
	if empty.Horizontal != 0.5 || empty.Vertical != 0.5 || empty.Width != actionList.Width || empty.Height != ActionRowHeight {
		t.Fatalf("empty state geometry = %+v, want centered %vx%v slot", empty, actionList.Width, ActionRowHeight)
	}
	row := empty.Child.(woxwidget.Flex)
	if row.Axis != woxwidget.Horizontal || row.Gap != 8 || row.CrossAxisAlignment != woxwidget.CrossAxisCenter || len(row.Children) != 2 {
		t.Fatalf("empty state content = %#v, want centered icon and message row", row)
	}
	if _, ok := row.Children[0].(woxwidget.Image); !ok {
		t.Fatalf("empty state icon = %T, want shared search image", row.Children[0])
	}
	if message, ok := row.Children[1].(woxwidget.Text); !ok || message.Value != "No matches" {
		t.Fatalf("empty state message = %#v, want No matches", row.Children[1])
	}
}

func TestActionRowCentersIconAndLabel(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, Items: []ActionItem{{ID: "open", Label: "打开 系统命令 设置", Icon: &woxui.Image{}}},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	panel := view.Child.(woxwidget.Container)
	actionList := panel.Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := actionList.Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	content := row.Child.(woxwidget.Flex)
	if content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("action row alignment = %v, want vertical center", content.CrossAxisAlignment)
	}
	icon := content.Children[0].(woxwidget.Align)
	if icon.Height != ActionRowHeight || icon.Vertical != 0.5 || icon.Child.(woxwidget.Container).Padding.Top != 0 {
		t.Fatalf("action icon slot = %#v, want a full-height centered slot", icon)
	}
	label := content.Children[1].(woxwidget.Align).Child.(woxwidget.TextBlock)
	if label.Height != 18 || label.LineHeight != 18 || label.AlignmentY != 0.5 || label.Value != "打开 系统命令 设置" {
		t.Fatalf("action label slot = %#v, want an 18px optically centered line", label)
	}
}
