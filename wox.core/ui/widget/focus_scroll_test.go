package widget

import (
	"testing"

	ui "wox/ui/runtime"
)

// TestHostRevealsNonEditorAcrossBothScrollAxes catches focus remaining horizontally clipped after a vertical reveal.
func TestHostRevealsNonEditorAcrossBothScrollAxes(t *testing.T) {
	outer, inner := NewScrollController(0), NewScrollController(0)
	var host *Host
	host = NewHost(func(ui.FrameInfo) Widget {
		return ScrollView{Key: "horizontal", Width: 100, Height: 80, ContentWidth: 300, Horizontal: true, Controller: outer,
			Child: ScrollView{Key: "vertical", Width: 300, Height: 80, Controller: inner,
				Child: Flex{Axis: Vertical, Children: []Widget{Container{Height: 200}, Flex{Axis: Horizontal, Children: []Widget{
					Container{Width: 260}, Semantics{AutomationID: "target", Role: ui.AccessibilityRoleButton, Label: "Target",
						Child: Focusable{Key: "target", Child: Gesture{OnTap: func() { host.RequestFocus("target") }, Child: Container{Width: 30, Height: 32}}}},
				}}}},
			},
		}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	render := func() { renderTestFrame(host); renderTestFrame(host); renderTestFrame(host) }
	render()
	if !host.FocusAutomationID("target") {
		t.Fatal("missing focus target")
	}
	render()
	if outer.Offset() != 190 || inner.Offset() != 152 {
		t.Fatalf("both ancestors must reveal the target: x=%v y=%v", outer.Offset(), inner.Offset())
	}
	outer.JumpTo(0)
	render()
	if outer.Offset() != 0 {
		t.Fatal("idle frames changed manual scrolling")
	}
	host.FocusAutomationID("target")
	render()
	if outer.Offset() != 190 {
		t.Fatal("explicit focus on the same target did not reveal it")
	}
	outer.JumpTo(180)
	render()
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 85, Y: 55}})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 85, Y: 55}})
	render()
	if outer.Offset() != 180 {
		t.Fatal("clicking an already focused target changed manual scrolling")
	}
}
