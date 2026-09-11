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
	woxwidget.AssertEqualCoversAllFields(t, actionSearchProps{})
}

type actionSearchHostServices struct{}

func (actionSearchHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}
func (actionSearchHostServices) Invalidate() error { return nil }
func (actionSearchHostServices) InvalidateRect(woxui.Rect) error {
	return nil
}
func (actionSearchHostServices) SetTextInputState(woxui.TextInputState) error { return nil }
func (actionSearchHostServices) SetPointerCursor(woxui.PointerCursor) error   { return nil }
func (actionSearchHostServices) UpdateAccessibility(woxui.AccessibilityTree, woxui.AccessibilityActionHandler) error {
	return nil
}

func TestActionSearchBoundaryVerifyPasses(t *testing.T) {
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return actionSearchBoundary(actionSearchProps{
			Width: 300, Height: 40, Window: &woxui.Window{},
			Style: woxui.TextStyle{Size: 14}, Theme: woxcomponent.Theme{},
		})
	})
	host.AttachServices(actionSearchHostServices{})
	if err := host.SetRepaintDebugMode(woxwidget.RepaintDebugVerify); err != nil {
		t.Fatal(err)
	}
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 40}, PixelSize: woxui.PixelSize{Width: 300, Height: 40}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	host.Frame(&woxui.DisplayList{}, frame)
	if diagnostics := host.Snapshot().Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("action search verify diagnostics = %v", diagnostics)
	}
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

func TestActionHeaderCentersLabel(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, ActionHeader: woxui.Color{A: 255}, HeaderLabel: "操作",
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	panel := view.Child.(woxwidget.Container)
	header := panel.Child.(woxwidget.Flex).Children[0].(woxwidget.Align)
	if header.Width != ActionPanelContentWidth || header.Height != ActionHeaderHeight || header.Vertical != 0.5 {
		t.Fatalf("action header slot = %#v, want a full-width centered %v-high slot", header, ActionHeaderHeight)
	}
	label := header.Child.(woxwidget.TextBlock)
	if label.Height != ActionHeaderHeight || label.LineHeight != ActionHeaderHeight || label.AlignmentY != 0.5 || label.Value != "操作" {
		t.Fatalf("action header label = %#v, want an 18px optically centered line", label)
	}
}

func TestActionFilterUsesReadableTypeWithoutChangingGeometry(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	panel := view.Child.(woxwidget.Container)
	searchSlot := panel.Child.(woxwidget.Flex).Children[3].(woxwidget.Container)
	searchBoundary := searchSlot.Child.(woxwidget.Boundary[actionSearchProps])
	search := searchBoundary.Props
	if searchSlot.Height != ActionSearchHeight || search.Height != 40 {
		t.Fatalf("action filter geometry = slot %v input %v, want %v and 40", searchSlot.Height, search.Height, ActionSearchHeight)
	}
	if search.Style.Size != woxcomponent.ActionFilterFontSize {
		t.Fatalf("action filter font size = %v, want shared size %v", search.Style.Size, woxcomponent.ActionFilterFontSize)
	}
	if search.Style.Size != 13 {
		t.Fatalf("action filter font size = %v, want readable 13px type", search.Style.Size)
	}
}

func TestActionPanelListHeightIncludesVisibleGroupDivider(t *testing.T) {
	items := []ActionItem{
		{ID: "copy"}, {ID: "keyword"}, {Kind: ActionItemKindSeparator}, {ID: "pin"}, {ID: "reset"},
	}
	got := ActionPanelListHeight(items)
	want := float32(4*ActionRowHeight + ActionGroupDividerHeight)
	if got != want {
		t.Fatalf("grouped list height = %v, want %v", got, want)
	}
}

func TestActionPanelListHeightOmitsDividerBelowFold(t *testing.T) {
	items := make([]ActionItem, 0, MaxVisibleActions+2)
	for index := 0; index < MaxVisibleActions; index++ {
		items = append(items, ActionItem{ID: "plugin"})
	}
	items = append(items, ActionItem{Kind: ActionItemKindSeparator}, ActionItem{ID: "pin"})
	if got := ActionPanelListHeight(items); got != float32(MaxVisibleActions*ActionRowHeight) {
		t.Fatalf("below-fold divider height = %v, want %v action rows", got, MaxVisibleActions)
	}
}

func TestActionGroupDividerGeometry(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{PreviewSplit: woxui.Color{A: 255}},
		Items: []ActionItem{{ID: "copy", Label: "Copy"}, {Kind: ActionItemKindSeparator}, {ID: "pin", Label: "Pin"}},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	panel := view.Child.(woxwidget.Container)
	actionList := panel.Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if actionList.Height != float32(2*ActionRowHeight+ActionGroupDividerHeight) {
		t.Fatalf("grouped list slot = %v, want two rows plus a %v divider", actionList.Height, ActionGroupDividerHeight)
	}
	rows := actionList.Content.(woxwidget.Flex).Children
	if len(rows) != 3 {
		t.Fatalf("grouped row count = %d, want action, divider, action", len(rows))
	}
	divider, ok := rows[1].(woxwidget.Align)
	if !ok || divider.Height != ActionGroupDividerHeight || divider.Vertical != 0.5 {
		t.Fatalf("group divider = %#v, want a %v-high title-divider slot", rows[1], ActionGroupDividerHeight)
	}
	line, ok := divider.Child.(woxwidget.Container)
	if !ok || line.Height != 1 || line.Color.A != 255 {
		t.Fatalf("group divider line = %#v, want a 1px PreviewSplit hairline", divider.Child)
	}
	gap, ok := panel.Child.(woxwidget.Flex).Children[1].(woxwidget.Container)
	if !ok || gap.Height != ActionHeaderGap || gap.Color.A != 0 || gap.Child != nil {
		t.Fatalf("title separator = %#v, want an empty %v-high gap without a hairline", panel.Child.(woxwidget.Flex).Children[1], ActionHeaderGap)
	}
}

func TestActionGroupDividerIsNotInteractive(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{},
		Items: []ActionItem{{ID: "copy", Label: "Copy"}, {Kind: ActionItemKindSeparator}, {ID: "pin", Label: "Pin"}},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	rows := view.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Flex).Children
	if _, ok := rows[1].(woxwidget.Gesture); ok {
		t.Fatal("group divider used a gesture")
	}
	if semantics, ok := rows[1].(woxwidget.Semantics); ok {
		t.Fatalf("group divider semantics = %#v, want no action automation node", semantics)
	}
	if _, ok := rows[0].(woxwidget.Semantics); !ok {
		t.Fatalf("plugin action = %T, want a menu item", rows[0])
	}
}

func TestActionKeepVisibleAccountsForGroupDivider(t *testing.T) {
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, Selected: 1,
		Items: []ActionItem{{Index: 0, ID: "copy", Label: "Copy"}, {Kind: ActionItemKindSeparator}, {Index: 1, ID: "pin", Label: "Pin"}},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	actionList := view.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if actionList.KeepVisible == nil {
		t.Fatal("grouped selection did not request KeepVisible")
	}
	start := float32(ActionRowHeight + ActionGroupDividerHeight)
	if actionList.KeepVisible.Start != start || actionList.KeepVisible.End != start+ActionRowHeight {
		t.Fatalf("KeepVisible = %+v, want [%v, %v] after the group divider", actionList.KeepVisible, start, start+ActionRowHeight)
	}
}

func TestActionStillPointerDoesNotStealDefaultSelection(t *testing.T) {
	phase := 0
	selected := 0
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		if phase == 0 {
			return woxwidget.Gesture{ID: "placeholder", Child: woxwidget.Container{Width: 400, Height: 220}}
		}
		view, _, _ := ActionsView(ActionsProps{
			Window: &woxui.Window{}, WindowWidth: 600, WindowHeight: 600, DensityScale: 1,
			ActionPadding: woxwidget.UniformInsets(10), Theme: woxcomponent.Theme{}, Selected: selected,
			Items: []ActionItem{
				{Index: 0, ID: "result-execute-0", Label: "Execute"},
				{Index: 1, ID: "result-execute_background-1", Label: "Background"},
			},
			OnSelect: func(index int) { selected = index },
		})
		return view
	})
	host.AttachServices(actionSearchHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 400, Height: 220}, PixelSize: woxui.PixelSize{Width: 400, Height: 220}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	// Park the pointer where the second action row will appear after the panel opens.
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 170, Y: 104}})
	phase = 1
	host.Frame(&woxui.DisplayList{}, frame)
	if selected != 0 {
		t.Fatalf("opening under a still pointer selected %d, want the default action", selected)
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 172, Y: 106}})
	if selected != 1 {
		t.Fatalf("pointer move selected %d, want the hovered action", selected)
	}
}

func TestActionRowUsesSelectedIconWithLabelColor(t *testing.T) {
	normal := &woxui.Image{Width: 1, Height: 1}
	selected := &woxui.Image{Width: 2, Height: 2}
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, Selected: 0,
		Items: []ActionItem{
			{Index: 0, ID: "copy", Label: "Copy Path", Icon: normal, SelectedIcon: selected},
			{Index: 1, ID: "open", Label: "Open", Icon: normal, SelectedIcon: selected},
		},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	rows := view.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Flex).Children
	selectedIcon := rows[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Container).Child.(woxwidget.Image)
	if selectedIcon.Source != selected {
		t.Fatalf("selected action icon = %#v, want the selected-text tint", selectedIcon.Source)
	}
	idleIcon := rows[1].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Container).Child.(woxwidget.Image)
	if idleIcon.Source != normal {
		t.Fatalf("unselected action icon = %#v, want the default text tint", idleIcon.Source)
	}
}

func TestActionRowShowsScoreTextAsTail(t *testing.T) {
	tailColor := woxui.Color{R: 180, G: 180, B: 190, A: 255}
	selectedTail := woxui.Color{R: 255, G: 255, B: 255, A: 255}
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, ResultTail: tailColor, SelectedTail: selectedTail, Selected: 0,
		Items: []ActionItem{
			{Index: 0, ID: "reset", Label: "Reset ranking", Icon: &woxui.Image{}, Tail: "+55"},
			{Index: 1, ID: "pin", Label: "Pin", Icon: &woxui.Image{}, Tail: "+8"},
		},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	rows := view.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Flex).Children
	selectedRow := rows[0].(woxwidget.Semantics)
	if selectedRow.Label != "Reset ranking +55" {
		t.Fatalf("selected score semantics = %q, want the boost in the accessible label", selectedRow.Label)
	}
	selectedTailSlot := selectedRow.Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Align)
	selectedTextWidth := actionPanelTailTextWidth(nil, "+55", woxcomponent.TailFontSize)
	if selectedTailSlot.Width != selectedTextWidth+15 || selectedTailSlot.Height != ActionRowHeight || selectedTailSlot.Vertical != 0.5 {
		t.Fatalf("selected score tail slot = %#v, want measured text in the trailing hotkey gutter", selectedTailSlot)
	}
	selectedContainer := selectedTailSlot.Child.(woxwidget.Container)
	if selectedContainer.Padding.Left != 10 || selectedContainer.Padding.Right != 5 {
		t.Fatalf("selected score tail inset = %#v, want the same 10/5 gutter as hotkeys", selectedContainer.Padding)
	}
	selectedText, ok := selectedContainer.Child.(woxwidget.Text)
	if !ok || selectedText.Value != "+55" || selectedText.Style.Size != woxcomponent.TailFontSize || selectedText.Color != selectedTail {
		t.Fatalf("selected score tail = %#v, want +55 in SelectedTail at TailFontSize", selectedContainer.Child)
	}
	idleText := rows[1].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Align).Child.(woxwidget.Container).Child.(woxwidget.Text)
	if idleText.Value != "+8" || idleText.Color != tailColor {
		t.Fatalf("unselected score tail = %#v, want +8 in ResultTail", idleText)
	}
}

func TestActionRowShowsPluginIconAsTail(t *testing.T) {
	tail := &woxui.Image{Width: 18, Height: 18}
	view := buildActionsView(woxwidget.StateContext{}, ActionsProps{
		WindowWidth: 600, WindowHeight: 600, DensityScale: 1, ActionPadding: woxwidget.UniformInsets(10),
		Theme: woxcomponent.Theme{}, Items: []ActionItem{{ID: "settings", Label: "Open App settings", Icon: &woxui.Image{}, TailIcon: tail}},
	}, woxwidget.NewScrollController(0)).(woxwidget.Gesture)
	content := view.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(content.Children) != 3 {
		t.Fatalf("action row children = %d, want leading icon, label, and plugin tail", len(content.Children))
	}
	tailSlot := content.Children[2].(woxwidget.Align)
	if tailSlot.Width != ActionTailIconSize+15 || tailSlot.Height != ActionRowHeight || tailSlot.Vertical != 0.5 {
		t.Fatalf("plugin tail slot = %#v, want an 18px icon in the trailing hotkey gutter", tailSlot)
	}
	if image, ok := tailSlot.Child.(woxwidget.Container).Child.(woxwidget.Image); !ok || image.Source != tail || image.Width != ActionTailIconSize {
		t.Fatalf("plugin tail icon = %#v, want the authored plugin image", tailSlot.Child)
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
