package view

import (
	"reflect"
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

func TestLauncherQueryHorizontalOffsetFollowsFocusedCaret(t *testing.T) {
	props := LauncherQueryProps{Width: 100, CaretWidth: 240, Focused: true}
	if got := launcherQueryHorizontalOffset(props); got != 144 {
		t.Fatalf("focused overflow offset = %v, want 144 so the caret stays 4px from the right edge", got)
	}
	props.Focused = false
	if got := launcherQueryHorizontalOffset(props); got != 0 {
		t.Fatalf("unfocused overflow offset = %v, want 0 so the start stays visible", got)
	}
	props.Focused = true
	props.CaretWidth = 40
	if got := launcherQueryHorizontalOffset(props); got != 0 {
		t.Fatalf("short query offset = %v, want 0", got)
	}
}

func TestLauncherQueryOverflowKeepsCaretVisible(t *testing.T) {
	theme := woxcomponent.Theme{QueryText: woxui.Color{A: 255}, Cursor: woxui.Color{R: 255, A: 255}}
	props := LauncherQueryProps{
		Width: 100, Height: 34, LineHeight: 34, CaretHeight: 34, CaretWidth: 240, Focused: true,
		State: woxui.TextEditingState{Text: "long query text that overflows", Selection: woxui.TextSelection{Anchor: 30, Focus: 30}},
		Lines: []LauncherQueryLine{{Text: "long query text that overflows", TextWidth: 240}},
		Theme: theme,
	}
	bounds := woxui.Rect{X: 20, Width: 100, Height: 34}
	var actual, expected woxui.DisplayList
	expected.DrawText(props.Lines[0].Text, woxui.Rect{X: -124, Y: bounds.Y, Width: 244, Height: 34}, props.Style, theme.QueryText)
	expected.FillRect(woxui.Rect{X: 116, Y: bounds.Y, Width: 2, Height: 34}, theme.Cursor)
	launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&actual, bounds, true, true)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("overflow caret paint = %#v, want the caret at the visible right edge", actual)
	}

	var ime woxui.TextInputState
	props.OnTextInputState = func(state woxui.TextInputState) { ime = state }
	launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&woxui.DisplayList{}, bounds, true, true)
	if ime.CursorRect.X != 116 || ime.CursorRect.X < bounds.X || ime.CursorRect.X >= bounds.X+bounds.Width {
		t.Fatalf("IME caret = %#v, want a visible anchor at 116", ime.CursorRect)
	}
}

func TestLauncherQueryOverflowExposesVisibleCursorRect(t *testing.T) {
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return LauncherQueryView(LauncherQueryProps{
			Width: 100, Height: 34, LineHeight: 34, CaretHeight: 34, CaretWidth: 240, Focused: true, Enabled: true,
			State: woxui.TextEditingState{Text: "long query text that overflows", Selection: woxui.TextSelection{Anchor: 30, Focus: 30}},
			Lines: []LauncherQueryLine{{Text: "long query text that overflows", TextWidth: 240}},
		})
	})
	host.AttachServices(&queryPointerHostServices{})
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 34}, PixelSize: woxui.PixelSize{Width: 100, Height: 34}, Scale: 1})
	host.RequestFocus(LauncherQueryInputKey)
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 34}, PixelSize: woxui.PixelSize{Width: 100, Height: 34}, Scale: 1})
	var input woxui.AccessibilityNode
	for _, node := range host.Snapshot().Tree.Nodes {
		if node.AutomationID == "launcher.query.input" {
			input = node
			break
		}
	}
	if input.AutomationID == "" || input.CursorRect.Width <= 0 {
		t.Fatalf("query input cursor = %#v, want an exposed IME caret", input.CursorRect)
	}
	if input.CursorRect.X < input.Bounds.X || input.CursorRect.X+input.CursorRect.Width > input.Bounds.X+input.Bounds.Width {
		t.Fatalf("overflow caret %#v left the query bounds %#v", input.CursorRect, input.Bounds)
	}
}

func TestLauncherQueryOverflowRemapsPointerToContent(t *testing.T) {
	var tapped woxui.Point
	props := LauncherQueryProps{
		Width: 100, Height: 34, CaretWidth: 240, Focused: true, Enabled: true,
		OnTapAt: func(point woxui.Point) { tapped = point },
	}
	launcherQueryEditor(props).(woxwidget.EditableText).Child.(woxwidget.Gesture).OnTapAt(woxui.Point{X: 10, Y: 8})
	if tapped.X != 154 || tapped.Y != 8 {
		t.Fatalf("overflow tap = %#v, want content point 154,8", tapped)
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

// Multi-argument hints start before unused separators without moving the editor text.
func TestQueryHintCompletionOffset(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 2} {
		props := LauncherQueryProps{Width: 400 * scale, Height: 34 * scale, LineHeight: 34 * scale,
			State: woxui.TextEditingState{Text: "g  "}, Lines: []LauncherQueryLine{{Text: "g  ", TextWidth: 30 * scale}},
			CompletionSuffix: "query time", CompletionOffset: -8 * scale}
		bounds := woxui.Rect{X: -100 * scale, Width: props.Width, Height: props.Height}
		var actual, expected woxui.DisplayList
		launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&actual, bounds, true, false)
		color := props.Theme.QueryText
		color.A = 96
		expected.DrawText(props.CompletionSuffix, woxui.Rect{X: bounds.X + 22*scale, Width: props.Width - 22*scale, Height: props.LineHeight}, props.Style, color)
		expected.DrawText("g  ", bounds, props.Style, props.Theme.QueryText)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("hint did not start at the first empty argument at scale %v", scale)
		}
		original := props
		original.CompletionOffset = 0
		if props.Equal(original) {
			t.Fatal("hint offset missing from boundary dependencies")
		}
	}
}

func TestQueryHintSingleEmptyArgumentPaintsChip(t *testing.T) {
	props := LauncherQueryProps{Width: 400, Height: 34, LineHeight: 34, CaretHeight: 30,
		State: woxui.TextEditingState{Text: "set volume "}, Lines: []LauncherQueryLine{{Text: "set volume ", TextWidth: 90}},
		CompletionSuffix: "Volume (0–100)",
		CompletionChips:  []LauncherQueryCompletionChip{{Text: "Volume (0–100)", X: 0, Width: 120}},
		Theme:            woxcomponent.Theme{QueryText: woxui.Color{R: 255, G: 255, B: 255, A: 255}}}
	bounds := woxui.Rect{Width: props.Width, Height: props.Height}
	var actual, expected woxui.DisplayList
	chip := props.Theme.QueryText
	chip.A = 18
	hint := props.Theme.QueryText
	hint.A = 96
	expected.FillRoundedRect(woxui.Rect{X: 87, Width: 126, Height: props.CaretHeight}, 4, chip)
	expected.DrawText("Volume (0–100)", woxui.Rect{X: 90, Width: 310, Height: props.LineHeight}, props.Style, hint)
	expected.DrawText("set volume ", bounds, props.Style, props.Theme.QueryText)
	launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&actual, bounds, true, false)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("single empty argument chip = %#v", actual)
	}
}

func TestQueryHintCompletionChipsPaintSeparately(t *testing.T) {
	props := LauncherQueryProps{Width: 400, Height: 34, LineHeight: 34, CaretHeight: 30,
		State: woxui.TextEditingState{Text: "g "}, Lines: []LauncherQueryLine{{Text: "g ", TextWidth: 20}},
		CompletionSuffix: "search query time range",
		CompletionChips: []LauncherQueryCompletionChip{
			{Text: "search query", X: 0, Width: 80},
			{Text: "time range", X: 92, Width: 70},
		},
		Theme: woxcomponent.Theme{QueryText: woxui.Color{R: 255, G: 255, B: 255, A: 255}}}
	bounds := woxui.Rect{Width: props.Width, Height: props.Height}
	var actual, expected woxui.DisplayList
	chip := props.Theme.QueryText
	chip.A = 18
	hint := props.Theme.QueryText
	hint.A = 96
	expected.FillRoundedRect(woxui.Rect{X: 17, Width: 86, Height: props.CaretHeight}, 4, chip)
	expected.DrawText("search query", woxui.Rect{X: 20, Width: 380, Height: props.LineHeight}, props.Style, hint)
	expected.FillRoundedRect(woxui.Rect{X: 109, Width: 76, Height: props.CaretHeight}, 4, chip)
	expected.DrawText("time range", woxui.Rect{X: 112, Width: 288, Height: props.LineHeight}, props.Style, hint)
	expected.DrawText("g ", bounds, props.Style, props.Theme.QueryText)
	launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&actual, bounds, true, false)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("multi-hint chips = %#v", actual)
	}
}

func TestLauncherQueryTabHintCentersOnLetterInk(t *testing.T) {
	props := LauncherQueryProps{
		LineHeight: 38, Style: woxui.TextStyle{Size: 28}, TextBaseline: 30,
		Lines:   []LauncherQueryLine{{Text: "ing"}},
		TabHint: LauncherQueryTabHint{Height: 14},
	}
	lineBoxCenterTop := float32(2) + (38-14)/2
	top := launcherQueryTabHintTop(props, 38, 2)
	if top != 18 || top <= lineBoxCenterTop {
		t.Fatalf("tab hint top = %v, want 18 below the line-box center %v so it sits on the lowercase ink", top, lineBoxCenterTop)
	}
}

func TestLauncherQueryPlacesTabHintAfterTarget(t *testing.T) {
	theme := woxcomponent.Theme{QueryText: woxui.Color{R: 255, G: 255, B: 255, A: 255}}
	hint := LauncherQueryTabHint{Visible: true, Label: "Tab", X: 80, Width: 14, Height: 14}
	props := LauncherQueryProps{Width: 400, Height: 42, LineHeight: 38, CaretHeight: 34, Focused: true,
		Style: woxui.TextStyle{Size: 28}, TextBaseline: 30,
		State: woxui.TextEditingState{Text: "sett"}, Lines: []LauncherQueryLine{{Text: "sett", TextWidth: 40}},
		CompletionSuffix: "ing", TabHint: hint, Theme: theme}
	feedback := launcherQueryFeedback(props).(woxwidget.AnimatedFloat)
	stack := feedback.Builder(feedback.Target).(woxwidget.Stack)
	if stack.Children[1].Left != 80 || stack.Children[1].Top != 18 {
		t.Fatalf("tab hint origin = %v,%v, want 80,18 on the lowercase text center", stack.Children[1].Left, stack.Children[1].Top)
	}
	glyph := stack.Children[1].Child.(woxwidget.Image)
	if glyph.Width != 14 || glyph.Height != 14 || glyph.Source == nil {
		t.Fatalf("tab hint glyph = %#v, want a 14x14 keyboard-tab image", glyph)
	}

	hidden := props
	hidden.TabHint.Visible = false
	if props.Equal(hidden) {
		t.Fatal("tab hint missing from boundary dependencies")
	}
	idle := launcherQueryFeedback(hidden).(woxwidget.AnimatedFloat).Builder(0)
	if _, ok := idle.(woxwidget.CaretPainter); !ok {
		t.Fatalf("hidden tab hint = %T, want the bare caret painter", idle)
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

func TestLauncherQueryExposesTabHintSemantics(t *testing.T) {
	query := LauncherQueryView(LauncherQueryProps{
		Width: 500, Height: 40, Focused: true, Enabled: true,
		TabHint: LauncherQueryTabHint{Visible: true, Label: "Tab", Width: 14, Height: 14},
	}).(woxwidget.Stack)
	scrollSemantic := query.Children[0].Child.(woxwidget.Semantics)
	scrollStack := scrollSemantic.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	scroll := scrollStack.Children[0].Child.(woxwidget.ScrollView)
	content := scroll.Child.(woxwidget.Stack)
	hint := content.Children[1].Child.(woxwidget.Semantics)
	if hint.AutomationID != "launcher.query.tab-hint" || hint.Role != woxui.AccessibilityRoleText || hint.Value != "Tab" || !hint.ReadOnly {
		t.Fatalf("query tab hint semantics = %#v", hint)
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
	animation := launcherQueryEditable(LauncherQueryView(props)).Child.(woxwidget.Gesture).Child.(woxwidget.AnimatedFloat)
	return animation.Builder(animation.Target).(woxwidget.CaretPainter)
}

// Feedback must move only the painted caret and settle without moving the IME anchor.
func TestQueryTabFeedbackKeepsEditorGeometry(t *testing.T) {
	var anchor woxui.TextInputState
	props := LauncherQueryProps{Width: 200, Height: 34, LineHeight: 34, CaretHeight: 30,
		CaretWidth: 40, Focused: true, TabFeedback: 1,
		OnTextInputState: func(state woxui.TextInputState) { anchor = state }}
	bounds := woxui.Rect{X: -100, Width: 200, Height: 34}
	animation := launcherQueryFeedback(props).(woxwidget.AnimatedFloat)
	var moving, resting, expected woxui.DisplayList
	animation.Builder(0.125).(woxwidget.CaretPainter).Paint(&moving, bounds, true, false)
	if anchor.CursorRect.X != -60 {
		t.Fatal("feedback moved IME anchor")
	}
	animation.Builder(1).(woxwidget.CaretPainter).Paint(&resting, bounds, true, false)
	launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&expected, bounds, true, false)
	if reflect.DeepEqual(moving, resting) || !reflect.DeepEqual(resting, expected) {
		t.Fatal("feedback must reveal the shaking caret then restore normal blink rendering")
	}
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

func TestQueryTextBaselineSurvivesQueryHintFocusTransition(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 2} {
		props := LauncherQueryProps{Width: 240 * scale, Height: 42 * scale, LineHeight: 38 * scale, CaretHeight: 34 * scale,
			Style: woxui.TextStyle{Size: 28 * scale}, State: woxui.TextEditingState{Text: "set volume "},
			Lines: []LauncherQueryLine{{Text: "set volume "}}, Theme: woxcomponent.Theme{QueryText: woxui.Color{A: 255}}}
		editable := launcherQueryEditor(props).(woxwidget.EditableText)
		animation := editable.Child.(woxwidget.Gesture).Child.(woxwidget.AnimatedFloat)
		before := animation.Builder(animation.Target).(woxwidget.CaretPainter)
		after := LauncherQueryLabel(props).(woxwidget.CaretPainter)
		bounds := woxui.Rect{X: -120 * scale, Y: 18 * scale, Width: props.Width, Height: props.Height}
		var editing, label, expected woxui.DisplayList
		before.Paint(&editing, bounds, true, false)
		after.Paint(&label, bounds, false, false)
		expected.DrawText("set volume ", woxui.Rect{X: bounds.X, Y: bounds.Y + 2*scale, Width: bounds.Width, Height: props.LineHeight}, props.Style, props.Theme.QueryText)
		if !reflect.DeepEqual(editing, label) || !reflect.DeepEqual(label, expected) {
			t.Fatalf("query baseline changed at scale %v", scale)
		}
	}
}

// Decoration must not move text, selection or caret at fractional display scales.
func TestInlineQueryMarkPreservesTextGeometry(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 2} {
		props := LauncherQueryProps{Width: 400 * scale, Height: 42 * scale, LineHeight: 38 * scale, CaretHeight: 34 * scale,
			Style: woxui.TextStyle{Size: 28 * scale}, State: woxui.TextEditingState{Text: "set volume 30"},
			Lines: []LauncherQueryLine{{Text: "set volume 30"}}, Theme: woxcomponent.Theme{QueryText: woxui.Color{A: 255}}}
		bounds := woxui.Rect{X: -120 * scale, Y: 18 * scale, Width: props.Width, Height: props.Height}
		var expected, actual woxui.DisplayList
		color := props.Theme.QueryText
		color.A = 18
		expected.FillRoundedRect(woxui.Rect{X: bounds.X + 160*scale - 3, Y: bounds.Y + 2*scale, Width: 32*scale + 6, Height: props.CaretHeight}, 4, color)
		launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&expected, bounds, true, false)
		props.Marks = []LauncherQueryMark{{X: 160 * scale, Width: 32 * scale, Active: true}}
		launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&actual, bounds, true, false)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("decoration changed editor geometry at scale %v", scale)
		}
	}
}

// Tight parameter gaps and editor edges must not produce overlapping decoration.
func TestInlineQueryMarkPaddingStopsAtNeighborsAndEdges(t *testing.T) {
	props := LauncherQueryProps{Width: 40, Height: 30, LineHeight: 30, CaretHeight: 26,
		Theme: woxcomponent.Theme{QueryText: woxui.Color{R: 255, G: 255, B: 255, A: 255}}}
	bounds := woxui.Rect{X: -100, Y: 20, Width: 40, Height: 30}
	var expected, actual woxui.DisplayList
	color := props.Theme.QueryText
	color.A = 10
	expected.FillRoundedRect(woxui.Rect{X: -100, Y: 20, Width: 21, Height: 26}, 4, color)
	expected.FillRoundedRect(woxui.Rect{X: -79, Y: 20, Width: 19, Height: 26}, 4, color)
	launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&expected, bounds, true, false)
	props.Marks = []LauncherQueryMark{{X: 0, Width: 20}, {X: 22, Width: 18}}
	launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&actual, bounds, true, false)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("parameter decoration exceeded editor edges or overlapped its neighbor")
	}
}
