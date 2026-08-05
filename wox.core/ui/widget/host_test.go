package widget

import (
	stdcontext "context"
	"slices"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

type fakeHostServices struct {
	tree             woxui.AccessibilityTree
	handler          woxui.AccessibilityActionHandler
	textInput        woxui.TextInputState
	pointerCursor    woxui.PointerCursor
	invalidations    int
	invalidatedRect  woxui.Rect
	invalidatedRects chan woxui.Rect
}

func (f *fakeHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}

func (f *fakeHostServices) Invalidate() error {
	f.invalidations++
	return nil
}

func (f *fakeHostServices) InvalidateRect(rect woxui.Rect) error {
	f.invalidations++
	f.invalidatedRect = rect
	if f.invalidatedRects != nil {
		select {
		case f.invalidatedRects <- rect:
		default:
		}
	}
	return nil
}

func (*fakeHostServices) DisplayListDamageCullingEnabled() bool { return true }

func TestHostExpandsPartialDamageForPaintOutsets(t *testing.T) {
	host := NewHost(func(woxui.FrameInfo) Widget { return Container{Width: 100, Height: 100} })
	services := &fakeHostServices{}
	host.AttachServices(services)
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 100}, Damage: woxui.Rect{X: 10, Y: 10, Width: 10, Height: 10}})
	if got, want := displayList.Damage(), (woxui.Rect{X: 6, Y: 6, Width: 18, Height: 18}); got != want {
		t.Fatalf("display-list damage = %+v, want conservative %+v", got, want)
	}
	if displayList.NativeDamage() != displayList.Damage() {
		t.Fatalf("native damage = %+v, want recorded damage %+v", displayList.NativeDamage(), displayList.Damage())
	}
}

func TestDisableIncrementalForcesFullFrameDamage(t *testing.T) {
	t.Setenv(DisableIncrementalEnvironment, "1")
	host := NewHost(func(woxui.FrameInfo) Widget { return Container{Width: 100, Height: 100} })
	host.AttachServices(&fakeHostServices{})
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 100}, Damage: woxui.Rect{X: 10, Y: 10, Width: 10, Height: 10}})
	if got := displayList.Damage(); got != (woxui.Rect{}) {
		t.Fatalf("disabled incremental damage = %+v, want full frame", got)
	}
	if got := displayList.NativeDamage(); got != (woxui.Rect{}) {
		t.Fatalf("disabled incremental native damage = %+v, want full frame", got)
	}
}

func (f *fakeHostServices) SetTextInputState(state woxui.TextInputState) error {
	f.textInput = state
	return nil
}

func (f *fakeHostServices) SetPointerCursor(cursor woxui.PointerCursor) error {
	f.pointerCursor = cursor
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
	if progress := host.loopValue(frame, "demo", time.Second, false); progress != 0 {
		t.Fatalf("initial loop progress = %v, want 0", progress)
	}
	if !host.active {
		t.Fatal("loop animation did not request another frame")
	}
	frame.generation++
	frame.now = start.Add(1250 * time.Millisecond)
	if progress := host.loopValue(frame, "demo", time.Second, false); progress != .25 {
		t.Fatalf("wrapped loop progress = %v, want 0.25", progress)
	}
}

func TestLoopAnimationPreservesPhaseWhilePaused(t *testing.T) {
	host := &animationHost{}
	start := time.Now()
	frame := animationFrame{host: host, generation: 1, now: start}
	host.loopValue(frame, "record", time.Second, false)
	frame.generation++
	frame.now = start.Add(400 * time.Millisecond)
	if progress := host.loopValue(frame, "record", time.Second, true); progress != .4 {
		t.Fatalf("paused loop progress = %v, want 0.4", progress)
	}
	frame.generation++
	frame.now = start.Add(900 * time.Millisecond)
	if progress := host.loopValue(frame, "record", time.Second, true); progress != .4 {
		t.Fatalf("held loop progress = %v, want 0.4", progress)
	}
	frame.generation++
	frame.now = start.Add(1100 * time.Millisecond)
	if progress := host.loopValue(frame, "record", time.Second, false); progress != .4 {
		t.Fatalf("resumed loop progress = %v, want continuity at 0.4", progress)
	}
}

func TestBoundaryAnimationSchedulesPartialInvalidation(t *testing.T) {
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "animated-region", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			return LoopAnimation{Key: "animated-region-loop", Duration: time.Second, Builder: func(float32) Widget {
				return Container{Width: 20, Height: 30}
			}}
		}}
	})
	services := &fakeHostServices{invalidatedRects: make(chan woxui.Rect, 1)}
	host.AttachServices(services)
	defer host.Dispose()
	renderTestFrame(host)

	select {
	case rect := <-services.invalidatedRects:
		if rect != (woxui.Rect{Width: 20, Height: 30}) {
			t.Fatalf("animation invalidation = %+v, want Boundary bounds", rect)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("animation did not schedule its next frame")
	}
}

func TestHostIncludesRemovedBoundaryInDamage(t *testing.T) {
	show := true
	host := NewHost(func(woxui.FrameInfo) Widget {
		children := []StackChild{}
		if show {
			children = append(children, StackChild{Left: 10, Top: 20, Child: Boundary[boundaryTestProps]{Key: "removed-region", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
				return Container{Width: 20, Height: 30}
			}}})
		}
		return Stack{Width: 100, Height: 100, Children: children}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	renderTestFrame(host)

	show = false
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 100}, PixelSize: woxui.PixelSize{Width: 100, Height: 100}, Scale: 1})
	if got, want := displayList.NativeDamage(), (woxui.Rect{X: 6, Y: 16, Width: 28, Height: 38}); got != want {
		t.Fatalf("removed Boundary damage = %+v, want old bounds with paint outset %+v", got, want)
	}
}

func TestEaseInOutCubicMatchesFlutterTonearmCurve(t *testing.T) {
	if got := transformAnimationProgress(.25, AnimationEaseInOutCubic); got != .0625 {
		t.Fatalf("ease-in progress = %v, want 0.0625", got)
	}
	if got := transformAnimationProgress(.75, AnimationEaseInOutCubic); got != .9375 {
		t.Fatalf("ease-out progress = %v, want 0.9375", got)
	}
}

func TestHostHidesCaretWhileWindowIsUnfocused(t *testing.T) {
	caretVisible := false
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return CaretPainter{Width: 20, Height: 20, Active: true, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect, focused, visible bool) {
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
			Child: Focusable{Key: "anchored-control", Child: Gesture{ID: "anchored-control", OnTapBounds: func(bounds woxui.Rect) {
				activatedBounds = bounds
			}, Child: Container{Width: 80, Height: 24}}},
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
	activatedBounds = woxui.Rect{}
	if !host.FocusAutomationID("anchored-control") || !host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
		t.Fatal("keyboard activation was not handled")
	}
	if activatedBounds.Width != 80 || activatedBounds.Height != 24 {
		t.Fatalf("keyboard activated bounds = %#v, want 80x24 control bounds", activatedBounds)
	}
}

func TestHostPaintsOnlyTheFocusedCaretWhenModalFocusChanges(t *testing.T) {
	modal := false
	backgroundFocused := false
	modalFocused := false
	paintedFocus := map[string]bool{}
	caret := func(id string, focused *bool, autofocus bool) Widget {
		return Focusable{
			Key: Key(id), Autofocus: autofocus, OnFocusChange: func(next bool) { *focused = next },
			Child: CaretPainter{Width: 20, Height: 20, Active: *focused, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect, focused, caretVisible bool) {
				paintedFocus[id] = focused
			}},
		}
	}
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		children := []StackChild{{Child: caret("background", &backgroundFocused, true)}}
		if modal {
			children = append(children, StackChild{Top: 20, Child: FocusScope{Key: "dialog", Modal: true, Child: caret("modal", &modalFocused, true)}})
		}
		return Stack{Width: 100, Height: 100, Children: children}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()

	renderTestFrame(host)
	if !paintedFocus["background"] {
		t.Fatal("background caret was not painted as focused")
	}

	modal = true
	paintedFocus = map[string]bool{}
	renderTestFrame(host)
	if paintedFocus["background"] || !paintedFocus["modal"] {
		t.Fatalf("painted focus = %v, want only the modal caret focused", paintedFocus)
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
	if host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierControl, Down: true}) {
		t.Fatal("Ctrl+Tab should remain available to the focused control")
	}
	renderTestFrame(host)
	if !findAutomationNode(t, host.Snapshot().Tree, "first").Focused {
		t.Fatal("Ctrl+Tab unexpectedly advanced focus")
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

func TestHostShowsFocusRingOnlyForKeyboardTraversal(t *testing.T) {
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Stack{Width: 100, Height: 100, Children: []StackChild{
			{Child: testButton("first", nil)},
			{Top: 20, Child: testButton("second", nil)},
		}}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	if !host.FocusAutomationID("first") || host.focusVisible {
		t.Fatal("programmatic focus should not show the focus ring")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || !host.focusVisible {
		t.Fatal("Tab traversal should show the focus ring")
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 25}})
	if host.focusVisible {
		t.Fatal("pointer focus should hide the focus ring")
	}
	if !host.isFocusedKey("second") {
		t.Fatal("pointer focus should keep the control logically focused")
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

func TestScrollViewKeepsMeasuredKeyVisible(t *testing.T) {
	controller := NewScrollController(0)
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return ScrollView{
			Key: "measured-scroll", Width: 100, Height: 50, Controller: controller, KeepVisibleKey: "target-row",
			Child: Flex{Axis: Vertical, Children: []Widget{
				Container{Width: 100, Height: 70},
				Keyed{Key: "target-row", Child: Container{Width: 100, Height: 30}},
			}},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	if controller.Offset() != 50 {
		t.Fatalf("measured keep-visible offset = %v, want 50", controller.Offset())
	}
}

func TestHostKeepsTabFocusedControlVisibleInScrollView(t *testing.T) {
	controller := NewScrollController(0)
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return ScrollView{
			Key: "focus-scroll", Width: 100, Height: 40, Controller: controller,
			Child: Flex{Axis: Vertical, Children: []Widget{
				testButton("first", nil),
				Container{Width: 100, Height: 60},
				testButton("second", nil),
				Container{Width: 100, Height: 100},
			}},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	if !host.FocusAutomationID("first") || !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) {
		t.Fatal("Tab should move focus to the second control")
	}
	if controller.Offset() != 40 {
		t.Fatalf("offset after Tab = %v, want 40", controller.Offset())
	}
	renderTestFrame(host)
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 5, Y: 5}, Scroll: woxui.Point{Y: -50}})
	renderTestFrame(host)
	if controller.Offset() != 90 {
		t.Fatalf("offset after pointer scroll = %v, want 90 without focus snap-back", controller.Offset())
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true, Modifiers: woxui.KeyModifierShift}) {
		t.Fatal("Shift+Tab should move focus to the first control")
	}
	if controller.Offset() != 0 {
		t.Fatalf("offset after Shift+Tab = %v, want 0", controller.Offset())
	}
}

func TestScrollViewShrinksToMeasuredContentBelowMaximumHeight(t *testing.T) {
	controller := NewScrollController(0)
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return ScrollView{
			Key: "shrink-scroll", Width: 100, MaxHeight: 80, Controller: controller,
			Child: Container{Width: 100, Height: 30},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	controller.mu.Lock()
	viewport, content := controller.viewport, controller.content
	controller.mu.Unlock()
	if viewport != 30 || content != 30 {
		t.Fatalf("shrink scroll geometry = viewport %v content %v, want 30/30", viewport, content)
	}
}

func TestStackCanAnchorMeasuredChildToBottom(t *testing.T) {
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Stack{Width: 100, Height: 100, Children: []StackChild{{
			Bottom: 10, AnchorBottom: true,
			Child: Semantics{Key: "bottom", AutomationID: "bottom", Role: woxui.AccessibilityRoleText, Child: Container{Width: 20, Height: 30}},
		}}}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)
	target := findAutomationNode(t, host.Snapshot().Tree, "bottom")
	if target.Bounds.Y != 60 {
		t.Fatalf("bottom-anchored y = %v, want 60", target.Bounds.Y)
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

func TestHostDispatchesPositionedDoubleAndTripleTap(t *testing.T) {
	var taps, doubleTaps, tripleTaps, selectionStarts int
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Gesture{
			ID: "multi-tap", OnTapAt: func(woxui.Point) { taps++ }, OnDoubleTapAt: func(woxui.Point) { doubleTaps++ }, OnTripleTapAt: func(woxui.Point) { tripleTaps++ },
			OnSelectionStart: func(woxui.Point) { selectionStarts++ },
			Child:            Container{Width: 100, Height: 20},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	for range 3 {
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	}
	if taps != 1 || doubleTaps != 1 || tripleTaps != 1 {
		t.Fatalf("tap counts = single %d, double %d, triple %d, want 1 each", taps, doubleTaps, tripleTaps)
	}
	if selectionStarts != 1 {
		t.Fatalf("selection starts = %d, want 1 without collapsing the double or triple selection", selectionStarts)
	}
}

func TestHostMultiTapRequiresNearbyClicks(t *testing.T) {
	var taps, doubleTaps int
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Gesture{
			ID: "multi-tap-distance", OnTapAt: func(woxui.Point) { taps++ }, OnDoubleTapAt: func(woxui.Point) { doubleTaps++ },
			Child: Container{Width: 100, Height: 20},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	for _, x := range []float32{5, 20} {
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: x, Y: 5}})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: x, Y: 5}})
	}
	if taps != 2 || doubleTaps != 0 {
		t.Fatalf("distant click counts = single %d, double %d, want 2 and 0", taps, doubleTaps)
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
	services := &fakeHostServices{}
	host.AttachServices(services)
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
	if services.invalidations != 1 || services.invalidatedRect != (woxui.Rect{Width: 100, Height: 20}) {
		t.Fatalf("Host hover damage = %d %#v, want one row-local invalidation", services.invalidations, services.invalidatedRect)
	}
}

func TestHostHoverMoveInvalidatesOldAndNewBounds(t *testing.T) {
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Flex{Axis: Vertical, Children: []Widget{
			Gesture{ID: "first", Child: Container{Width: 100, Height: 20}},
			Gesture{ID: "second", Child: Container{Width: 100, Height: 20}},
		}}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)
	renderTestFrame(host)

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 5, Y: 5}})
	services.invalidations = 0
	services.invalidatedRect = woxui.Rect{}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 5, Y: 25}})
	if services.invalidations != 1 || services.invalidatedRect != (woxui.Rect{Width: 100, Height: 40}) {
		t.Fatalf("hover move damage = %d %#v, want one combined two-row invalidation", services.invalidations, services.invalidatedRect)
	}
}

func TestHostReportsGesturePressWithoutChangingTap(t *testing.T) {
	var pressed []bool
	taps := 0
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Gesture{
			ID: "button", OnPressChange: func(value bool) { pressed = append(pressed, value) }, OnTap: func() { taps++ },
			Child: Container{Width: 40, Height: 20},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	if !slices.Equal(pressed, []bool{true, false}) || taps != 1 {
		t.Fatalf("press states = %v, taps = %d, want [true false] and one tap", pressed, taps)
	}

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 50, Y: 5}})
	if !slices.Equal(pressed, []bool{true, false, true, false}) || taps != 1 {
		t.Fatalf("outside release states = %v, taps = %d, want paired release without another tap", pressed, taps)
	}
}

func TestHostUpdatesPointerCursorForHoveredGesture(t *testing.T) {
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Gesture{ID: "input", Cursor: woxui.PointerCursorText, Child: Container{Width: 100, Height: 20}}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)
	renderTestFrame(host)

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 5, Y: 5}})
	if services.pointerCursor != woxui.PointerCursorText {
		t.Fatalf("hover cursor = %v, want text", services.pointerCursor)
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerLeave, Position: woxui.Point{X: 5, Y: 5}})
	if services.pointerCursor != woxui.PointerCursorDefault {
		t.Fatalf("leave cursor = %v, want default", services.pointerCursor)
	}
}

func TestHostUnfocusesOptedInControlOnOutsidePointerDown(t *testing.T) {
	var focusChanges []bool
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Flex{Axis: Horizontal, Children: []Widget{
			Focusable{
				Key: "recorder", UnfocusOnPointerOutside: true,
				OnFocusChange: func(focused bool) { focusChanges = append(focusChanges, focused) },
				Child:         Gesture{ID: "recorder", Child: Container{Width: 20, Height: 20}},
			},
			Gesture{ID: "outside", Child: Container{Width: 20, Height: 20}},
		}}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	if len(focusChanges) != 1 || !focusChanges[0] {
		t.Fatalf("focus changes after recorder press = %v, want [true]", focusChanges)
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 25, Y: 5}})
	if len(focusChanges) != 2 || focusChanges[1] {
		t.Fatalf("focus changes after outside press = %v, want [true false]", focusChanges)
	}
}

func TestHostPanTracksPointerOutsideBounds(t *testing.T) {
	var points []woxui.Point
	ended := false
	host := NewHost(func(frame woxui.FrameInfo) Widget {
		return Gesture{
			ID:          "pan",
			OnPanStart:  func(point woxui.Point) { points = append(points, point) },
			OnPanUpdate: func(point woxui.Point) { points = append(points, point) },
			OnPanEnd:    func() { ended = true },
			Child:       Container{Width: 100, Height: 20},
		}
	})
	host.AttachServices(&fakeHostServices{})
	renderTestFrame(host)

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 5, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 120, Y: 5}})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 120, Y: 5}})

	if len(points) != 2 || points[0].X != 5 || points[1].X != 120 || !ended {
		t.Fatalf("pan = points %#v ended %v, want local X positions 5/120 and ended", points, ended)
	}
}
