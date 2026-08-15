package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
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
	left := row.Children[0].(woxwidget.Container)
	right := row.Children[2].(woxwidget.Container)
	if left.Height != 32 || right.Height != 32 {
		t.Fatalf("toolbar row height = left %v right %v, want 32 so action hover insets fit", left.Height, right.Height)
	}
	if len(left.Child.(woxwidget.Flex).Children) != 0 {
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

	stateful, ok := action.Child.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("toolbar action child = %T, want shared hoverable state", action.Child)
	}
	gesture := stateful.CreateState().Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Gesture)
	if gesture.OnTap == nil || gesture.OnHoverAt == nil {
		t.Fatalf("toolbar action gesture = tap %v hover %v, want shared hover state", gesture.OnTap != nil, gesture.OnHoverAt != nil)
	}
}

func TestLauncherToolbarActionHoversLabelAndKeycapsTogether(t *testing.T) {
	theme := woxcomponent.Theme{
		ToolbarBackground: woxui.Color{R: 20, G: 24, B: 28, A: 255},
		ToolbarText:       woxui.Color{R: 220, G: 230, B: 240, A: 255},
	}
	action := LauncherToolbarAction{ID: "open", Label: "Open", HotkeyLabels: []string{"Enter"}}
	normal, normalWidth := launcherToolbarActionSurface(action, theme, &woxui.Window{}, 1, false)
	hovered, hoveredWidth := launcherToolbarActionSurface(action, theme, &woxui.Window{}, 1, true)
	if normalWidth != hoveredWidth {
		t.Fatalf("toolbar action hover width = %v, want unchanged %v", hoveredWidth, normalWidth)
	}

	normalContainer := normal.(woxwidget.Container)
	hoveredContainer := hovered.(woxwidget.Container)
	want := woxcomponent.ControlHoverColor(theme.ToolbarBackground, theme.ToolbarText)
	if normalContainer.Height != 32 || hoveredContainer.Height != 32 {
		t.Fatalf("toolbar action height = %v / %v, want 32 with 2px vertical inset", normalContainer.Height, hoveredContainer.Height)
	}
	if normalContainer.Padding != (woxwidget.Insets{Left: 8, Top: 2, Right: 8, Bottom: 2}) || hoveredContainer.Padding != normalContainer.Padding {
		t.Fatalf("toolbar action padding = %#v / %#v, want stable 8px horizontal and 2px vertical inset", normalContainer.Padding, hoveredContainer.Padding)
	}
	if normalContainer.Color != (woxui.Color{}) {
		t.Fatalf("toolbar action default background = %#v, want transparent", normalContainer.Color)
	}
	if hoveredContainer.Color != want {
		t.Fatalf("toolbar action hover background = %#v, want %#v", hoveredContainer.Color, want)
	}

	normalChip := toolbarActionKeycapFill(t, normalContainer)
	hoveredChip := toolbarActionKeycapFill(t, hoveredContainer)
	if normalChip != theme.ToolbarBackground {
		t.Fatalf("toolbar keycap default fill = %#v, want toolbar background", normalChip)
	}
	if hoveredChip != want {
		t.Fatalf("toolbar keycap hover fill = %#v, want shared action hover %#v", hoveredChip, want)
	}
}

func toolbarActionKeycapFill(t *testing.T, action woxwidget.Container) woxui.Color {
	t.Helper()
	chip := action.Child.(woxwidget.Flex).Children[1].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	return chip.Children[0].Child.(woxwidget.Container).Color
}
