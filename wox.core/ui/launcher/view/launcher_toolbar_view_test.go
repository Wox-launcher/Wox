package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLauncherToolbarSplitGivesStatusPriority(t *testing.T) {
	left, right := launcherToolbarSplit(740, 220, 150, 16)
	if left != 220 || right != 504 {
		t.Fatalf("toolbar split = left %v right %v, want status to keep 220 and leave 504 for shortcuts", left, right)
	}
}

func TestLauncherToolbarSplitEllipsizesStatusToKeepReservedShortcuts(t *testing.T) {
	left, right := launcherToolbarSplit(300, 220, 150, 16)
	if left != 134 || right != 150 {
		t.Fatalf("toolbar split = left %v right %v, want the long status to shrink to 134 so Enter and More stay", left, right)
	}
}

func TestFitLauncherToolbarActionsKeepsEnterAndMoreThenFills(t *testing.T) {
	actions := []measuredLauncherToolbarAction{
		{id: "toolbar-action-result-hide-launcher-0", width: 80, pinned: true},
		{id: "toolbar-action-result-open-folder-1", width: 80},
		{id: "toolbar-action-toolbar-keep-open-0", width: 70},
		{id: launcherToolbarMoreActionID, width: 70},
	}
	tight, used := fitLauncherToolbarActions(actions, 0, 150)
	if used != 150 || !toolbarActionIDsEqual(tight, "toolbar-action-result-hide-launcher-0", launcherToolbarMoreActionID) {
		t.Fatalf("tight toolbar actions = %v width %v, want Enter and More", toolbarActionIDs(tight), used)
	}
	wider, used := fitLauncherToolbarActions(actions, 0, 230)
	if used != 230 || !toolbarActionIDsEqual(wider, "toolbar-action-result-hide-launcher-0", "toolbar-action-result-open-folder-1", launcherToolbarMoreActionID) {
		t.Fatalf("wider toolbar actions = %v width %v, want Enter, the next hotkey, and More", toolbarActionIDs(wider), used)
	}
}

func toolbarActionIDs(actions []measuredLauncherToolbarAction) []string {
	ids := make([]string, len(actions))
	for index, action := range actions {
		ids[index] = action.id
	}
	return ids
}

func toolbarActionIDsEqual(actions []measuredLauncherToolbarAction, want ...string) bool {
	if len(actions) != len(want) {
		return false
	}
	for index, action := range actions {
		if action.id != want[index] {
			return false
		}
	}
	return true
}

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

func TestLauncherToolbarKeepsSixteenPixelActionContentSpacing(t *testing.T) {
	built := LauncherToolbarView(LauncherToolbarProps{
		Width: 800, Height: 40, Window: &woxui.Window{}, DensityScale: 1,
		Actions: []LauncherToolbarAction{
			{ID: "execute", Label: "Execute", HotkeyLabels: []string{"Enter"}},
			{ID: "background", Label: "Execute in Background", HotkeyLabels: []string{"Ctrl", "Enter"}},
		},
	}).(woxwidget.Stack)
	body := built.Children[0].Child.(woxwidget.Container)
	row := body.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	right := row.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	if len(right.Children) != 2 {
		t.Fatalf("toolbar action count = %d, want both actions visible", len(right.Children))
	}
	if right.Gap != 0 {
		t.Fatalf("toolbar action gap = %v, want 0 so 8px hover padding keeps 16px between contents", right.Gap)
	}
}

func TestLauncherToolbarOmitsEmptyActionLabels(t *testing.T) {
	theme := woxcomponent.Theme{
		ToolbarBackground: woxui.Color{R: 20, G: 24, B: 28, A: 255},
		ToolbarText:       woxui.Color{R: 220, G: 230, B: 240, A: 255},
	}
	empty, emptyWidth := launcherToolbarActionSurface(LauncherToolbarAction{ID: "blank", HotkeyLabels: []string{"Enter"}}, theme, &woxui.Window{}, 1, false)
	labeled, labeledWidth := launcherToolbarActionSurface(LauncherToolbarAction{ID: "more", Label: "More Actions", HotkeyLabels: []string{"Ctrl", "J"}}, theme, &woxui.Window{}, 1, false)

	emptyFlex := empty.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(emptyFlex.Children) != 1 {
		t.Fatalf("empty toolbar action children = %d, want only the Enter keycap", len(emptyFlex.Children))
	}
	labeledFlex := labeled.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(labeledFlex.Children) != 2 {
		t.Fatalf("labeled toolbar action children = %d, want label and keycaps", len(labeledFlex.Children))
	}
	if _, isAlign := emptyFlex.Children[0].(woxwidget.Align); isAlign {
		t.Fatal("empty toolbar action must not use Align with width 0, which fills the row and overlaps the next action")
	}
	if _, isText := emptyFlex.Children[0].(woxwidget.Text); isText {
		t.Fatal("empty toolbar action must not render a blank label before the Enter keycap")
	}
	if emptyWidth >= labeledWidth {
		t.Fatalf("empty toolbar action width = %v, want smaller than labeled width %v", emptyWidth, labeledWidth)
	}

	built := LauncherToolbarView(LauncherToolbarProps{
		Width: 800, Height: 40, Window: &woxui.Window{}, DensityScale: 1,
		Actions: []LauncherToolbarAction{
			{ID: "blank", HotkeyLabels: []string{"Enter"}},
			{ID: "more", Label: "More Actions", HotkeyLabels: []string{"Ctrl", "J"}},
		},
	}).(woxwidget.Stack)
	right := built.Children[0].Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[2].(woxwidget.Container)
	if right.Width != emptyWidth+labeledWidth {
		t.Fatalf("toolbar action row width = %v, want %v so empty Enter does not overlap More Actions", right.Width, emptyWidth+labeledWidth)
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
