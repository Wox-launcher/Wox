package view

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLauncherToolbarBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, LauncherToolbarAction{})
	woxwidget.AssertEqualCoversAllFields(t, LauncherToolbarProps{})
}

func TestLauncherToolbarOmitsEmptyLeftContent(t *testing.T) {
	built := LauncherToolbarView(LauncherToolbarProps{
		Width: 800, Height: 40, Window: &woxui.Window{}, DensityScale: 1,
		Actions: []LauncherToolbarAction{{ID: "execute", Label: "Execute", HotkeyLabels: []string{"Enter"}}},
	}).(woxwidget.Stack)
	body := built.Children[0].Child.(woxwidget.Container)
	row := body.Child.(woxwidget.Flex)
	left := row.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)

	if len(left.Children) != 0 {
		t.Fatal("empty toolbar status must not consume the action row")
	}
}
