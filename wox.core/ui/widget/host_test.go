package widget

import (
	stdcontext "context"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

type fakeHostServices struct {
	tree          woxui.AccessibilityTree
	handler       woxui.AccessibilityActionHandler
	textInput     woxui.TextInputState
	invalidations int
}

func (f *fakeHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}

func (f *fakeHostServices) Invalidate() error {
	f.invalidations++
	return nil
}

func (f *fakeHostServices) SetTextInputState(state woxui.TextInputState) error {
	f.textInput = state
	return nil
}

func (f *fakeHostServices) UpdateAccessibility(tree woxui.AccessibilityTree, handler woxui.AccessibilityActionHandler) error {
	f.tree = tree
	f.handler = handler
	return nil
}

// testButton builds one keyed control whose visual, interaction, focus, and semantics identities coincide.
func testButton(id string, onTap func()) Widget {
	return Semantics{
		Key:          Key(id),
		AutomationID: id,
		Role:         woxui.AccessibilityRoleButton,
		Label:        id,
		Actions:      []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		Child: Focusable{
			Key: Key(id),
			Child: Gesture{
				ID:    id,
				OnTap: onTap,
				Child: Container{Width: 10, Height: 10},
			},
		},
	}
}

func renderTestFrame(host *Host) {
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 100}, PixelSize: woxui.PixelSize{Width: 100, Height: 100}, Scale: 1})
}

func findAutomationNode(t *testing.T, tree woxui.AccessibilityTree, automationID string) woxui.AccessibilityNode {
	t.Helper()
	for _, current := range tree.Nodes {
		if current.AutomationID == automationID {
			return current
		}
	}
	t.Fatalf("automation node %q was not found", automationID)
	return woxui.AccessibilityNode{}
}

func TestLoopAnimationAdvancesAndWraps(t *testing.T) {
	host := &animationHost{}
	start := time.Now()
	frame := animationFrame{host: host, generation: 1, now: start}
	if progress := host.loopValue(frame, "demo", time.Second); progress != 0 {
		t.Fatalf("initial loop progress = %v, want 0", progress)
	}
	if !host.active {
		t.Fatal("loop animation did not request another frame")
	}
	frame.generation++
	frame.now = start.Add(1250 * time.Millisecond)
	if progress := host.loopValue(frame, "demo", time.Second); progress != .25 {
		t.Fatalf("wrapped loop progress = %v, want 0.25", progress)
	}
}

func TestHostHidesCaretWhileWindowIsUnfocused(t *testing.T) {
	caretVisible := false
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return CaretPainter{Width: 20, Height: 20, Active: true, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect, visible bool) {
			caretVisible = visible
		}}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)
	defer host.Dispose()

	renderTestFrame(host)
	if !caretVisible {
		t.Fatal("caret is hidden while the window is focused")
	}

	host.SetWindowFocused(false)
	renderTestFrame(host)
	if caretVisible {
		t.Fatal("caret is visible while the window is unfocused")
	}
	host.caretBlinkMu.Lock()
	blinkActive := host.caretBlinkActive
	host.caretBlinkMu.Unlock()
	if blinkActive {
		t.Fatal("caret blink remains active while the window is unfocused")
	}

	host.SetWindowFocused(true)
	renderTestFrame(host)
	if !caretVisible {
		t.Fatal("caret did not return when the window regained focus")
	}
}

func TestHostKeepsPressedIdentityAcrossKeyedReorder(t *testing.T) {
	order := []string{"a", "b"}
	taps := map[string]int{}
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		children := make([]Widget, 0, len(order))
		for _, id := range order {
			currentID := id
			children = append(children, testButton(currentID, func() { taps[currentID]++ }))
		}
		return Flex{Axis: Horizontal, Children: children}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)
	renderTestFrame(host)
	before := findAutomationNode(t, host.Snapshot().Tree, "b")

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 15, Y: 5}})
	order = []string{"b", "a"}
	renderTestFrame(host)
	after := findAutomationNode(t, host.Snapshot().Tree, "b")
	if before.ID != after.ID {
		t.Fatalf("keyed node ID changed across reorder: before=%d after=%d", before.ID, after.ID)
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	if taps["b"] != 1 {
		t.Fatalf("expected reordered pressed button to activate once, got %d", taps["b"])
	}
}

func TestHostAutomationActivatePassesGestureBounds(t *testing.T) {
	var activatedBounds woxui.Rect
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Semantics{
			Key: "anchored-control", AutomationID: "anchored-control", Role: woxui.AccessibilityRoleButton, Label: "Anchored control",
			Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
			Child: Gesture{ID: "anchored-control", OnTapBounds: func(bounds woxui.Rect) {
				activatedBounds = bounds
			}, Child: Container{Width: 80, Height: 24}},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	control := findAutomationNode(t, host.Snapshot().Tree, "anchored-control")
	if err := host.performAccessibilityAction(control.ID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("activate anchored control: %v", err)
	}
	if activatedBounds.Width != 80 || activatedBounds.Height != 24 {
		t.Fatalf("activated bounds = %#v, want 80x24 control bounds", activatedBounds)
	}
}

func TestHostTrapsAndRestoresModalFocusOrder(t *testing.T) {
	modal := false
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		children := []StackChild{{Child: testButton("base", nil)}}
		if modal {
			children = append(children, StackChild{Top: 20, Child: FocusScope{Key: "dialog", Modal: true, Child: Flex{Axis: Horizontal, Children: []Widget{testButton("first", nil), testButton("second", nil)}}}})
		}
		return Stack{Width: 100, Height: 100, Children: children}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)
	renderTestFrame(host)
	if !host.FocusAutomationID("base") {
		t.Fatal("failed to focus the base control")
	}
	modal = true
	renderTestFrame(host)
	if !findAutomationNode(t, host.Snapshot().Tree, "first").Focused {
		t.Fatal("modal scope did not focus its first control")
	}

	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) {
		t.Fatal("Tab was not handled by the focus manager")
	}
	renderTestFrame(host)
	if !findAutomationNode(t, host.Snapshot().Tree, "second").Focused {
		t.Fatal("Tab did not advance focus")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierShift, Down: true}) {
		t.Fatal("Shift+Tab was not handled by the focus manager")
	}
	renderTestFrame(host)
	if !findAutomationNode(t, host.Snapshot().Tree, "first").Focused {
		t.Fatal("Shift+Tab did not move focus backward")
	}

	modal = false
	renderTestFrame(host)
	if !findAutomationNode(t, host.Snapshot().Tree, "base").Focused {
		t.Fatal("closing the modal scope did not restore the previous focus")
	}
}

func TestHostSemanticsProtectsValuesAndReportsDuplicateAutomationIDs(t *testing.T) {
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Flex{Children: []Widget{
			Semantics{Key: "password", AutomationID: "field", Role: woxui.AccessibilityRoleTextField, Label: "Password", Value: "secret", Protected: true, Child: Container{Width: 10, Height: 10}},
			Semantics{Key: "duplicate", AutomationID: "field", Role: woxui.AccessibilityRoleText, Label: "Duplicate", Child: Container{Width: 10, Height: 10}},
		}}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	snapshot := host.Snapshot()
	if value := findAutomationNode(t, snapshot.Tree, "field").Value; value != "" {
		t.Fatalf("protected semantics exposed value %q", value)
	}
	if len(snapshot.Diagnostics) == 0 {
		t.Fatal("duplicate automation ID did not produce a diagnostic")
	}
}

func TestHostWaitForChangeUsesFrameGeneration(t *testing.T) {
	host := NewHost(func(frame woxui.FrameInfo) Widget { return Container{Width: 10, Height: 10} })
	host.AttachServices(&fakeHostServices{})
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), time.Second)
	defer cancel()
	done := make(chan AutomationSnapshot, 1)
	go func() {
		snapshot, _ := host.WaitForChange(ctx, 0)
		done <- snapshot
	}()
	renderTestFrame(host)
	select {
	case snapshot := <-done:
		if snapshot.Tree.Generation != 1 {
			t.Fatalf("expected generation 1, got %d", snapshot.Tree.Generation)
		}
	case <-ctx.Done():
		t.Fatal("WaitForChange did not observe the rendered frame")
	}
}

func TestHorizontalScrollViewUsesHorizontalExtentAndPointerDelta(t *testing.T) {
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return ScrollView{
			Key: "horizontal-scroll", Width: 100, Height: 20, ContentWidth: 200, Horizontal: true,
			Child: Stack{Width: 200, Height: 20, Children: []StackChild{{
				Left: 150,
				Child: Semantics{
					Key: "target", AutomationID: "target", Role: woxui.AccessibilityRoleText,
					Child: Container{Width: 10, Height: 10},
				},
			}}},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	before := findAutomationNode(t, host.Snapshot().Tree, "target")

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 5, Y: 5}, Scroll: woxui.Point{X: -40}})
	renderTestFrame(host)
	after := findAutomationNode(t, host.Snapshot().Tree, "target")
	if after.Bounds.X != before.Bounds.X-40 {
		t.Fatalf("target x after horizontal scroll = %v, want %v", after.Bounds.X, before.Bounds.X-40)
	}
}

func TestVerticalWheelOverHorizontalScrollBubblesToOuterScrollView(t *testing.T) {
	outer := NewScrollController(0)
	inner := NewScrollController(0)
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return ScrollView{
			Key: "outer", Width: 100, Height: 100, ContentHeight: 200, Controller: outer,
			Child: Stack{Width: 100, Height: 200, Children: []StackChild{{
				Child: ScrollView{
					Key: "inner", Width: 100, Height: 20, ContentWidth: 200, Horizontal: true, Controller: inner,
					Child: Container{Width: 200, Height: 20},
				},
			}}},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 5, Y: 5}, Scroll: woxui.Point{Y: -30}})
	if inner.Offset() != 0 {
		t.Fatalf("horizontal inner offset = %v, want 0 for a vertical wheel event", inner.Offset())
	}
	if outer.Offset() != 30 {
		t.Fatalf("outer offset = %v, want 30 after bubbled vertical wheel event", outer.Offset())
	}
}

func TestNestedVerticalScrollBubblesAtInnerBoundary(t *testing.T) {
	outer := NewScrollController(0)
	inner := NewScrollController(0)
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return ScrollView{
			Key: "outer", Width: 100, Height: 100, ContentHeight: 200, Controller: outer,
			Child: Stack{Width: 100, Height: 200, Children: []StackChild{{
				Child: ScrollView{
					Key: "inner", Width: 100, Height: 20, ContentHeight: 40, Controller: inner,
					Child: Container{Width: 100, Height: 40},
				},
			}}},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	if !inner.JumpTo(20) {
		t.Fatal("failed to move inner scroll view to its lower boundary")
	}

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 5, Y: 5}, Scroll: woxui.Point{Y: -10}})
	if inner.Offset() != 20 {
		t.Fatalf("inner offset = %v, want to remain at boundary 20", inner.Offset())
	}
	if outer.Offset() != 10 {
		t.Fatalf("outer offset = %v, want 10 after inner boundary propagation", outer.Offset())
	}
}

// TestHostDragSelectionExtendsAndClickCollapses verifies that a press+drag on a selection
// gesture extends the selection, while a press+release without movement falls through to tap.
func TestHostDragSelectionExtendsAndClickCollapses(t *testing.T) {
	var startCalls, extendCalls int
	var lastExtend woxui.Point
	var tapCalls int
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Gesture{
			ID:                "select",
			OnTap:             func() { tapCalls++ },
			OnSelectionStart:  func(p woxui.Point) { startCalls++ },
			OnSelectionExtend: func(p woxui.Point) { extendCalls++; lastExtend = p },
			Child:             Container{Width: 100, Height: 20},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	// Press inside the gesture, drag to the right, release: selection should extend, tap should be skipped.
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	if startCalls != 1 {
		t.Fatalf("expected OnSelectionStart once, got %d", startCalls)
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 30, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 60, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 60, Y: 5}})
	if extendCalls == 0 {
		t.Fatal("drag did not extend the selection")
	}
	if tapCalls != 0 {
		t.Fatalf("tap should be skipped after drag selection, got %d taps", tapCalls)
	}
	if lastExtend.X != 60 {
		t.Fatalf("last extend local X = %v, want 60", lastExtend.X)
	}

	// Press and release without movement: OnSelectionStart fires on press, but tap still dispatches on release.
	startCalls = 0
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	if startCalls != 1 {
		t.Fatalf("expected OnSelectionStart on click press, got %d", startCalls)
	}
	if tapCalls != 1 {
		t.Fatalf("click without drag should dispatch tap, got %d taps", tapCalls)
	}
}

func TestHostRequiresPointerMoveBeforeHover(t *testing.T) {
	var hoverStates []bool
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Gesture{
			ID:      "hover",
			OnHover: func(inside bool) { hoverStates = append(hoverStates, inside) },
			Child:   Container{Width: 100, Height: 20},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	position := woxui.Point{X: 5, Y: 5}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerEnter, Position: position})
	if len(hoverStates) != 0 {
		t.Fatalf("pointer enter hover states = %v, want none", hoverStates)
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: position})
	if len(hoverStates) != 1 || !hoverStates[0] {
		t.Fatalf("pointer move hover states = %v, want [true]", hoverStates)
	}
}

func TestHostPanTracksPointerOutsideBounds(t *testing.T) {
	var points []woxui.Point
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Gesture{
			ID:          "pan",
			OnPanStart:  func(point woxui.Point) { points = append(points, point) },
			OnPanUpdate: func(point woxui.Point) { points = append(points, point) },
			Child:       Container{Width: 100, Height: 20},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 120, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 120, Y: 5}})

	if len(points) != 2 || points[0].X != 5 || points[1].X != 120 {
		t.Fatalf("pan points = %#v, want local X positions 5 and 120", points)
	}
}
