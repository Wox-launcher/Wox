package widget

import (
	"testing"

	ui "wox/ui/runtime"
)

// TestTargetFocusAndSelection preserves child identity and rejects unavailable controls.
func TestTargetFocusAndSelection(t *testing.T) {
	selected := 0
	host := NewHost(func(ui.FrameInfo) Widget {
		return Flex{Children: []Widget{
			Target{Key: "disabled", Child: EditableText{Key: "disabled-input", Disabled: true, OnSelectAll: func() error { t.Fatal("disabled selection"); return nil }, Child: Container{Width: 80, Height: 20}}},
			Target{Key: "group", Child: Flex{Children: []Widget{
				Focusable{Key: "skip", Disabled: true, Child: Container{Width: 80, Height: 20}},
				EditableText{Key: "editor", ReadOnly: true, OnSelectAll: func() error { selected++; return nil }, Child: Container{Width: 80, Height: 20}},
			}}},
			Target{Key: "label", Child: Text{Value: "No editor"}},
		}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	renderTestFrame(host)
	if err := host.FocusWithin("group"); err != nil || host.FocusedKey() != "editor" {
		t.Fatalf("focus replaced child identity: %s, %v", host.FocusedKey(), err)
	}
	if err := host.SelectAllWithin("group"); err != nil || selected != 1 {
		t.Fatalf("read-only selection: %d, %v", selected, err)
	}
	for _, key := range []Key{"missing", "disabled", "label"} {
		if host.FocusWithin(key) == nil || host.SelectAllWithin(key) == nil {
			t.Fatalf("accepted unavailable target %q", key)
		}
	}
	host.SetOverlay("", FocusScope{Key: "modal", Modal: true, Child: Focusable{Key: "modal-input", Child: Container{Width: 20, Height: 20}}})
	renderTestFrame(host)
	if !host.HasModalScope() || host.FocusWithin("group") == nil || host.SelectAllWithin("group") == nil {
		t.Fatal("command escaped modal scope")
	}
	if _, err := host.Reveal("group"); err == nil {
		t.Fatal("reveal escaped modal scope")
	}
}

// TestTargetRevealAcrossAxes checks logical geometry without introducing focus.
func TestTargetRevealAcrossAxes(t *testing.T) {
	outer, inner := NewScrollController(0), NewScrollController(0)
	host := NewHost(func(ui.FrameInfo) Widget {
		return ScrollView{Key: "horizontal", Width: 100, Height: 80, ContentWidth: 300, Horizontal: true, Controller: outer,
			Child: ScrollView{Key: "vertical", Width: 300, Height: 80, Controller: inner,
				Child: Flex{Axis: Vertical, Children: []Widget{Container{Height: 200}, Flex{Axis: Horizontal, Children: []Widget{
					Container{Width: 260}, Target{Key: "target", Child: Container{Width: 30, Height: 32}},
				}}}},
			},
		}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2, 1} {
		outer.JumpTo(0)
		inner.JumpTo(0)
		for i := 0; i < 4; i++ {
			host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 100, Height: 80}, PixelSize: ui.PixelSize{Width: int(100 * scale), Height: int(80 * scale)}, Scale: scale})
			if _, err := host.Reveal("target"); err != nil {
				t.Fatal(err)
			}
		}
		if outer.Offset() != 190 || inner.Offset() != 152 || host.FocusedKey() != "" {
			t.Fatalf("scale %v: x=%v y=%v focus=%s", scale, outer.Offset(), inner.Offset(), host.FocusedKey())
		}
	}
}
