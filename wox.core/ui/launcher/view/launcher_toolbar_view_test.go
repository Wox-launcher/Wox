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

func TestLauncherToolbarUsesBlankCenterForWindowDragging(t *testing.T) {
	dragged := false
	built := LauncherToolbarView(LauncherToolbarProps{
		Width: 800, Height: 40, Window: &woxui.Window{}, DensityScale: 1,
		OnDragStart: func() { dragged = true },
		Actions:     []LauncherToolbarAction{{ID: "execute", Label: "Execute", HotkeyLabels: []string{"Enter"}}},
	}).(woxwidget.Stack)
	body := built.Children[0].Child.(woxwidget.Container)
	row := body.Child.(woxwidget.Flex)
	dragArea := row.Children[1].(woxwidget.Gesture)
	dragArea.OnDragStart()

	if !dragged {
		t.Fatal("blank toolbar area did not start window dragging")
	}
	if dragArea.ID != "launcher-toolbar-drag-area" {
		t.Fatalf("toolbar drag area id = %q, want launcher-toolbar-drag-area", dragArea.ID)
	}
}
