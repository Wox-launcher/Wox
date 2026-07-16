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
