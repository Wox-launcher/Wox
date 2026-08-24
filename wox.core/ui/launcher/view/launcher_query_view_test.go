package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLauncherQueryWiresClipboardAccessibilityActions(t *testing.T) {
	selectAll, copy, cut, paste := false, false, false, false
	query := launcherQueryEditable(LauncherQueryView(LauncherQueryProps{
		Height: 40, Enabled: true, State: woxui.TextEditingState{Text: "query"},
		OnSelectAll: func() error { selectAll = true; return nil },
		OnCopy:      func() error { copy = true; return nil },
		OnCut:       func() error { cut = true; return nil },
		OnPaste:     func() error { paste = true; return nil },
	}))
	if query.OnSelectAll == nil || query.OnCopy == nil || query.OnCut == nil || query.OnPaste == nil {
		t.Fatal("launcher query should expose clipboard handlers when wired")
	}
	_ = query.OnSelectAll()
	_ = query.OnCopy()
	_ = query.OnCut()
	_ = query.OnPaste()
	if !selectAll || !copy || !cut || !paste {
		t.Fatalf("clipboard handlers not invoked: selectAll=%v copy=%v cut=%v paste=%v", selectAll, copy, cut, paste)
	}
}

func TestLauncherQueryRemainsFocusableWithoutOwningFocus(t *testing.T) {
	query := launcherQueryEditable(LauncherQueryView(LauncherQueryProps{Height: 40, Focused: false, Enabled: true}))
	if query.Disabled {
		t.Fatal("unfocused query was disabled instead of remaining pointer-focusable")
	}
}

type queryPointerHostServices struct {
	pointerCursor woxui.PointerCursor
}

func (queryPointerHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}
func (queryPointerHostServices) Invalidate() error                            { return nil }
func (queryPointerHostServices) InvalidateRect(woxui.Rect) error              { return nil }
func (queryPointerHostServices) SetTextInputState(woxui.TextInputState) error { return nil }
func (s *queryPointerHostServices) SetPointerCursor(cursor woxui.PointerCursor) error {
	s.pointerCursor = cursor
	return nil
}
func (queryPointerHostServices) UpdateAccessibility(woxui.AccessibilityTree, woxui.AccessibilityActionHandler) error {
	return nil
}

func TestLauncherQueryDragAreaKeepsDefaultCursorWhileQueryChanges(t *testing.T) {
	textWidth := float32(40)
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return LauncherQueryBoundary(LauncherQueryProps{
			Width: 500, Height: 40, TextWidth: textWidth, Enabled: true,
			State: woxui.TextEditingState{Text: "query"},
		})
	})
	services := &queryPointerHostServices{}
	host.AttachServices(services)
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 500, Height: 40}, PixelSize: woxui.PixelSize{Width: 500, Height: 40}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 400, Y: 20}})
	if services.pointerCursor != woxui.PointerCursorDefault {
		t.Fatalf("drag hover cursor = %v, want default", services.pointerCursor)
	}

	textWidth = 70
	host.Frame(&woxui.DisplayList{}, frame)
	if services.pointerCursor != woxui.PointerCursorDefault {
		t.Fatalf("cursor after query change = %v, want default", services.pointerCursor)
	}

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerLeave, Position: woxui.Point{X: 400, Y: 20}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerEnter, Position: woxui.Point{X: 400, Y: 20}})
	if services.pointerCursor != woxui.PointerCursorDefault {
		t.Fatalf("cursor after leave/enter on drag area = %v, want default", services.pointerCursor)
	}
}

func TestLauncherQueryKeepsMinimumEditableAreaBeforeDragOverlay(t *testing.T) {
	tapped := false
	query := LauncherQueryView(LauncherQueryProps{
		Width: 500, Height: 40, TextWidth: 50, Enabled: true,
		OnTapEnd: func() { tapped = true },
	}).(woxwidget.Stack)
	dragArea := query.Children[1]
	if dragArea.Left != 350 {
		t.Fatalf("drag area left = %v, want 350", dragArea.Left)
	}
	dragGesture := dragArea.Child.(woxwidget.Gesture)
	if dragGesture.Cursor != woxui.PointerCursorDefault {
		t.Fatalf("drag area cursor = %v, want default", dragGesture.Cursor)
	}
	dragGesture.OnTap()
	if !tapped {
		t.Fatal("drag area tap did not request query focus")
	}
}

func TestLauncherQueryForwardsMultiClickSelection(t *testing.T) {
	doubleTaps, tripleTaps := 0, 0
	query := launcherQueryEditable(LauncherQueryView(LauncherQueryProps{
		Height: 40, Enabled: true, OnDoubleTapAt: func(woxui.Point) { doubleTaps++ }, OnTripleTapAt: func(woxui.Point) { tripleTaps++ },
	}))
	gesture := query.Child.(woxwidget.Gesture)
	gesture.OnDoubleTapAt(woxui.Point{X: 10})
	gesture.OnTripleTapAt(woxui.Point{X: 10})
	if doubleTaps != 1 || tripleTaps != 1 {
		t.Fatalf("query multi-click callbacks = double %d, triple %d, want 1 each", doubleTaps, tripleTaps)
	}
}

func TestLauncherQueryHidesCaretWhenTextIsSelected(t *testing.T) {
	theme := woxcomponent.Theme{
		QueryText:           woxui.Color{A: 255},
		SelectionBackground: woxui.Color{B: 255, A: 255},
		SelectionText:       woxui.Color{R: 255, G: 255, B: 255, A: 255},
		Cursor:              woxui.Color{A: 255},
	}
	style := woxui.TextStyle{Size: 20}
	bounds := woxui.Rect{Width: 200, Height: 40}
	selected := launcherQueryCaretPainter(LauncherQueryProps{
		Width: 200, Height: 40, LineHeight: 34, CaretHeight: 34, Focused: true, Style: style, Theme: theme,
		State: woxui.TextEditingState{Text: "note", Selection: woxui.TextSelection{Anchor: 0, Focus: 3}},
		Lines: []LauncherQueryLine{{Text: "note", Selected: "not", SelectedWidth: 30, TextWidth: 40}},
	})
	selectedVisible := &woxui.DisplayList{}
	selectedHidden := &woxui.DisplayList{}
	selected.Paint(selectedVisible, bounds, true, true)
	selected.Paint(selectedHidden, bounds, true, false)
	if selectedVisible.CommandCount() != selectedHidden.CommandCount() {
		t.Fatalf("selected query still paints a blinking caret: visible=%d hidden=%d", selectedVisible.CommandCount(), selectedHidden.CommandCount())
	}

	collapsed := launcherQueryCaretPainter(LauncherQueryProps{
		Width: 200, Height: 40, LineHeight: 34, CaretHeight: 34, Focused: true, Style: style, Theme: theme,
		State: woxui.TextEditingState{Text: "note", Selection: woxui.TextSelection{Anchor: 3, Focus: 3}},
		Lines: []LauncherQueryLine{{Text: "note", TextWidth: 40}},
	})
	collapsedVisible := &woxui.DisplayList{}
	collapsedHidden := &woxui.DisplayList{}
	collapsed.Paint(collapsedVisible, bounds, true, true)
	collapsed.Paint(collapsedHidden, bounds, true, false)
	if collapsedVisible.CommandCount() <= collapsedHidden.CommandCount() {
		t.Fatal("collapsed query should still paint a caret while focused")
	}
}

func TestLauncherQueryUsesSharedScrollViewForHiddenLines(t *testing.T) {
	query := LauncherQueryView(LauncherQueryProps{
		Width: 100, Height: 136, LineHeight: 34, CaretHeight: 34, CaretLine: 4, Lines: make([]LauncherQueryLine, 5),
	}).(woxwidget.Stateful)
	props := query.Widget.(woxcomponent.ScrollViewProps)
	if props.ContentHeight != 170 || props.KeepVisible == nil || props.KeepVisible.Start != 136 || props.KeepVisible.End != 170 || !props.AlwaysShowScrollbar || props.AutomationID != "launcher.query.scroll" {
		t.Fatalf("query scroll props = content %.0f keep %#v always visible %v automation %q", props.ContentHeight, props.KeepVisible, props.AlwaysShowScrollbar, props.AutomationID)
	}
}

func TestLauncherQueryLeavesSharedScrollbarGutterOutsideDragOverlay(t *testing.T) {
	query := LauncherQueryView(LauncherQueryProps{
		Width: 500, Height: 136, TextWidth: 50, LineHeight: 34, Lines: make([]LauncherQueryLine, 5),
	}).(woxwidget.Stack)
	dragArea := query.Children[1].Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if dragArea.Width != 136 {
		t.Fatalf("multiline drag area width = %.0f, want 136", dragArea.Width)
	}
}

func TestLauncherQueryExposesInlineCompletionSuffix(t *testing.T) {
	query := LauncherQueryView(LauncherQueryProps{Width: 500, Height: 40, CompletionSuffix: "pleted", Enabled: true}).(woxwidget.Stack)
	scrollSemantic := query.Children[0].Child.(woxwidget.Semantics)
	scrollStack := scrollSemantic.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	scroll := scrollStack.Children[0].Child.(woxwidget.ScrollView)
	content := scroll.Child.(woxwidget.Stack)
	completion := content.Children[1].Child.(woxwidget.Semantics)
	if completion.AutomationID != "launcher.query.completion" || completion.Role != woxui.AccessibilityRoleText || completion.Value != "pleted" || !completion.ReadOnly || completion.LiveRegion != woxui.AccessibilityLiveRegionPolite {
		t.Fatalf("query completion semantics = %#v", completion)
	}
}

func TestLauncherHeaderExposesQueryLoadingProgress(t *testing.T) {
	header := LauncherHeaderView(LauncherHeaderProps{
		Width: 500, Height: 50, QueryBoxHeight: 50, QueryEditorHeight: 34, QueryWidth: 400,
		Loading: true, LoadingWidth: 49, LoadingSize: 20,
	}).(woxwidget.Container)
	row := header.Child.(woxwidget.Constrained).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	loading := row.Children[1].(woxwidget.Semantics)
	if loading.AutomationID != "launcher.query.loading" || loading.Role != woxui.AccessibilityRoleProgressBar || loading.Value != "loading" || !loading.ReadOnly {
		t.Fatalf("query loading semantics = id %q role %q value %q readonly %v", loading.AutomationID, loading.Role, loading.Value, loading.ReadOnly)
	}
	boundary := loading.Child.(woxwidget.Boundary[launcherQueryLoadingProps])
	if boundary.Key != LauncherQueryLoadingBoundaryKey {
		t.Fatalf("query loading boundary key = %q, want %q", boundary.Key, LauncherQueryLoadingBoundaryKey)
	}
	loadingIndicator := boundary.Build(boundary.Props).(woxwidget.Align)
	if loadingIndicator.Horizontal != 0.5 || loadingIndicator.Vertical != 0.5 {
		t.Fatalf("query loading alignment = %.1f/%.1f, want centered", loadingIndicator.Horizontal, loadingIndicator.Vertical)
	}
}

func TestLauncherHeaderUsesAlignmentForVerticalAccessoryPlacement(t *testing.T) {
	header := LauncherHeaderView(LauncherHeaderProps{
		Width: 600, Height: 60, QueryBoxHeight: 50, QueryWidth: 400,
		Refinement: woxwidget.Container{Width: 40, Height: 34}, RefinementWidth: 40,
		Glance: woxwidget.Container{Width: 30, Height: 30}, GlanceWidth: 30,
		Icon: &woxui.Image{},
	}).(woxwidget.Container)
	row := header.Child.(woxwidget.Constrained).Child.(woxwidget.Container).Child.(woxwidget.Flex)

	querySlot, ok := row.Children[0].(woxwidget.Expanded)
	if !ok {
		t.Fatalf("query slot = %T, want Expanded", row.Children[0])
	}
	if alignment, ok := querySlot.Child.(woxwidget.Align); !ok || alignment.Vertical != 0.5 || alignment.Width != 400 {
		t.Fatalf("query alignment = %#v, want vertically centered Align 400 wide", querySlot.Child)
	}
	for index, child := range row.Children[1:4] {
		alignment, ok := child.(woxwidget.Align)
		if !ok || alignment.Vertical != 0.5 {
			t.Fatalf("header accessory %d = %T, want vertically centered Align", index+1, child)
		}
	}
}

func TestLauncherScopeIconsWidthIncludesRightPadding(t *testing.T) {
	if got := LauncherScopeIconsWidth(1, 1); got != 49 {
		t.Fatalf("single scope icon width = %.0f, want 49", got)
	}
	if got := LauncherScopeIconsWidth(3, 1); got != 77 {
		t.Fatalf("three scope icons width = %.0f, want 77", got)
	}
}

func TestLauncherHeaderExposesScopeIconGroup(t *testing.T) {
	header := LauncherHeaderView(LauncherHeaderProps{
		Width: 500, Height: 50, QueryBoxHeight: 50, QueryWidth: 400,
		Icons: []*woxui.Image{{}, {}},
	}).(woxwidget.Container)
	row := header.Child.(woxwidget.Constrained).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	scopeIcons := row.Children[1].(woxwidget.Semantics)
	if scopeIcons.AutomationID != "launcher.query.scope-icons" || scopeIcons.Role != woxui.AccessibilityRoleGroup || scopeIcons.Value != "2" || !scopeIcons.ReadOnly {
		t.Fatalf("scope icon semantics = id %q role %q value %q readonly %v", scopeIcons.AutomationID, scopeIcons.Role, scopeIcons.Value, scopeIcons.ReadOnly)
	}
	alignment := scopeIcons.Child.(woxwidget.Align)
	if alignment.Width != LauncherScopeIconsWidth(2, 1) || alignment.Vertical != 0.5 {
		t.Fatalf("scope icon alignment = width %.0f vertical %.1f", alignment.Width, alignment.Vertical)
	}
}

func TestLauncherHeaderAppliesBottomAppPaddingForBottomQueryChrome(t *testing.T) {
	header := LauncherHeaderView(LauncherHeaderProps{
		Width: 400, Height: 65, QueryBoxHeight: 55, QueryWidth: 300,
		AppPadding: woxwidget.Insets{Left: 8, Bottom: 10, Right: 8},
	}).(woxwidget.Container)
	if header.Padding.Top != 0 || header.Padding.Bottom != 10 || header.Padding.Left != 8 || header.Padding.Right != 8 {
		t.Fatalf("header padding = %+v, want bottom app padding under the query pill", header.Padding)
	}
	pill := header.Child.(woxwidget.Constrained).Child.(woxwidget.Container)
	if pill.Height != 55 {
		t.Fatalf("query pill height = %.0f, want 55", pill.Height)
	}
}

func TestLauncherViewPutsEmptyLeadingSpaceAboveBottomQuery(t *testing.T) {
	view := LauncherView(LauncherViewProps{
		Width: 400, Height: 75, QueryAtBottom: true,
		Content: woxwidget.Container{Width: 400, Height: 10},
		Header:  woxwidget.Container{Width: 400, Height: 65},
	}).(woxwidget.Semantics).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(view.Children) != 2 {
		t.Fatalf("section count = %d, want leading space then query chrome", len(view.Children))
	}
	leading, ok := view.Children[0].(woxwidget.Container)
	if !ok || leading.Height != 10 {
		t.Fatalf("leading section = %#v, want 10px top app padding", view.Children[0])
	}
	header, ok := view.Children[1].(woxwidget.Container)
	if !ok || header.Height != 65 {
		t.Fatalf("query section = %#v, want bottom-anchored query chrome", view.Children[1])
	}
}

func TestLauncherQueryBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, LauncherQueryProps{})
	woxwidget.AssertEqualCoversAllFields(t, launcherQueryLoadingProps{})
}

func TestGlanceBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, GlanceProps{})
}

func launcherQueryCaretPainter(props LauncherQueryProps) woxwidget.CaretPainter {
	return launcherQueryEditable(LauncherQueryView(props)).Child.(woxwidget.Gesture).Child.(woxwidget.CaretPainter)
}

func launcherQueryEditable(widget woxwidget.Widget) woxwidget.EditableText {
	if stack, ok := widget.(woxwidget.Stack); ok {
		widget = stack.Children[0].Child
	}
	if stateful, ok := widget.(woxwidget.Stateful); ok {
		return stateful.Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.EditableText)
	}
	if semantics, ok := widget.(woxwidget.Semantics); ok {
		widget = semantics.Child
	}
	stack := widget.(woxwidget.Gesture).Child.(woxwidget.Stack)
	scroll := stack.Children[0].Child.(woxwidget.ScrollView)
	return scroll.Child.(woxwidget.EditableText)
}
