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
	alignment := body.Child.(woxwidget.Align)
	row := alignment.Child.(woxwidget.Flex)
	if body.Padding.Top != 0 || body.Padding.Bottom != 0 || alignment.Height != 40 || alignment.Vertical != 0.5 {
		t.Fatalf("toolbar alignment = padding %#v child %#v, want full-height vertical center", body.Padding, alignment)
	}
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
	row := body.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	dragArea := row.Children[1].(woxwidget.Gesture)
	dragArea.OnDragStart()

	if !dragged {
		t.Fatal("blank toolbar area did not start window dragging")
	}
	if dragArea.ID != "launcher-toolbar-drag-area" {
		t.Fatalf("toolbar drag area id = %q, want launcher-toolbar-drag-area", dragArea.ID)
	}
}

func TestLauncherToolbarExposesStatusAndActionSemantics(t *testing.T) {
	activated := false
	built := LauncherToolbarView(LauncherToolbarProps{
		Width: 800, Height: 40, Window: &woxui.Window{}, DensityScale: 1, Label: "Toolbar fixture ready",
		Actions: []LauncherToolbarAction{{ID: "toolbar-action-keep-open", Label: "Keep open", HotkeyLabels: []string{"Ctrl", "K"}, OnTap: func() { activated = true }}},
	}).(woxwidget.Stack)
	body := built.Children[0].Child.(woxwidget.Container)
	row := body.Child.(woxwidget.Align).Child.(woxwidget.Flex)

	leftFlex := row.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	left := leftFlex.Children[0].(woxwidget.Semantics)
	if left.AutomationID != "launcher.toolbar.status" || left.Role != woxui.AccessibilityRoleGroup || left.Value != "Toolbar fixture ready" {
		t.Fatalf("toolbar status semantics = %#v", left)
	}

	right := row.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	action := right.Children[0].(woxwidget.Semantics)
	if action.AutomationID != "toolbar-action-keep-open" || action.Role != woxui.AccessibilityRoleButton {
		t.Fatalf("toolbar action semantics = %#v", action)
	}
	if err := action.OnAction(woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("activate toolbar action semantics: %v", err)
	}
	if !activated {
		t.Fatal("toolbar action semantics did not invoke the action")
	}
}
