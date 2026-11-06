package widget

import (
	"reflect"
	"testing"
	ui "wox/ui/runtime"
)

// TestFocusObserverTracksImmediateTransitionsAndRemoval guards focus lifetime independently of rendering cadence.
func TestFocusObserverTracksImmediateTransitionsAndRemoval(t *testing.T) {
	var changes []bool
	showFields := true
	host := NewHost(func(ui.FrameInfo) Widget {
		var children []Widget
		if showFields {
			for _, key := range []Key{"a", "b"} {
				children = append(children, Focusable{Key: key, Child: Container{Width: 40, Height: 30}})
			}
		}
		return FocusObserver{Key: "group", OnChange: func(v bool) { changes = append(changes, v) }, Child: Flex{Axis: Vertical, Children: children}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	frame := func() { host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 100, Height: 100}, Scale: 1}) }
	frame()
	host.RequestFocus("a")
	host.RequestFocus("b")
	if !reflect.DeepEqual(changes, []bool{true}) {
		t.Fatalf("sibling switch: %v", changes)
	}
	frame()
	if len(changes) != 1 {
		t.Fatal("rebuild produced a focus transition")
	}
	host.ClearFocus()
	host.RequestFocus("a")
	host.ClearFocus()
	if !reflect.DeepEqual(changes, []bool{true, false, true, false}) {
		t.Fatalf("same-frame transitions: %v", changes)
	}
	host.RequestFocus("a")
	showFields = false
	frame()
	if !reflect.DeepEqual(changes, []bool{true, false, true, false, true, false}) {
		t.Fatalf("removal: %v", changes)
	}
}
