package sys

import (
	"encoding/json"
	"strings"
	"testing"

	ui "wox/ui/runtime"
	"wox/ui/widget"
	"wox/ui/woxview"
)

type noteTestServices struct{}

func (noteTestServices) MeasureText(text string, style ui.TextStyle) (ui.TextMetrics, error) {
	return ui.TextMetrics{Size: ui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}
func (noteTestServices) Invalidate() error                         { return nil }
func (noteTestServices) InvalidateRect(ui.Rect) error              { return nil }
func (noteTestServices) SetTextInputState(ui.TextInputState) error { return nil }
func (noteTestServices) SetPointerCursor(ui.PointerCursor) error   { return nil }
func (noteTestServices) UpdateAccessibility(ui.AccessibilityTree, ui.AccessibilityActionHandler) error {
	return nil
}

// TestShowcaseNotesState checks editing, filtering, selection and delayed events
// against their own note identity without touching the real Notes repository.
func TestShowcaseNotesState(t *testing.T) {
	s := &showcaseNotes{newTitle: "New", notes: []showcaseNote{{title: "First", body: "One"}, {title: "Second", body: "Two"}}}
	arg := func(value any) []json.RawMessage { b, _ := json.Marshal(value); return []json.RawMessage{b} }
	if !s.apply("select", "note-1", nil) || !s.apply("body", "body-0", arg("Edited first")) || s.notes[0].body != "Edited first" || s.notes[1].body != "Two" {
		t.Fatal("selection or delayed edit changed the wrong note")
	}
	if !s.apply("task", "task-1-2", arg(true)) || !s.notes[1].tasks[2] || s.apply("task", "task-9-2", arg(true)) {
		t.Fatal("checklist event was not validated")
	}
	if !s.apply("search", "search", arg("second")) || strings.Contains(s.document(), "First") {
		t.Fatal("search did not filter the sidebar")
	}
	if !s.apply("add", "", nil) || s.selected != 2 || s.search != "" || s.notes[2].title != "New" {
		t.Fatal("new note did not become visible and selected")
	}
	if _, err := woxview.Parse([]byte(s.document())); err != nil {
		t.Fatalf("new note document: %v", err)
	}
}

// TestShowcaseNoteLayout checks real WoxView layout and interaction at multiple
// logical sizes and display scales without launching a native Wox process.
func TestShowcaseNoteLayout(t *testing.T) {
	for _, size := range []struct{ width, height, scale float32 }{{960, 680, 1}, {560, 520, 1.5}, {960, 680, 2}} {
		s := &showcaseNotes{notes: []showcaseNote{{title: "First", body: "One"}, {title: "Second", body: "Two"}}, newTitle: "New"}
		view := woxview.NewView()
		setTree := func() {
			tree, err := woxview.Parse([]byte(s.document()))
			if err != nil {
				t.Fatal(err)
			}
			view.SetTree(tree)
		}
		setTree()
		host := widget.NewHost(func(frame ui.FrameInfo) widget.Widget {
			return view.Build(woxview.BuildProps{Width: frame.Size.Width, Height: frame.Size.Height, OnNodeEvent: func(event woxview.Event) {
				var args []json.RawMessage
				_ = json.Unmarshal(event.Args, &args)
				if s.apply(string(event.Handler), event.Key, args) {
					setTree()
				}
			}})
		})
		host.AttachServices(noteTestServices{})
		frame := func() {
			host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: size.width, Height: size.height}, PixelSize: ui.PixelSize{Width: int(size.width * size.scale), Height: int(size.height * size.scale)}, Scale: size.scale})
		}
		frame()
		find := func(fragment string) ui.AccessibilityNode {
			for _, node := range host.Snapshot().Tree.Nodes {
				if strings.Contains(node.AutomationID, fragment) {
					return node
				}
			}
			t.Fatalf("missing %s at %+v", fragment, size)
			return ui.AccessibilityNode{}
		}
		search := find("k:search/text-field/field")
		title := find("k:title-0/text-field/field")
		if search.Bounds.Width <= 0 || title.Bounds.Width <= 0 || title.Bounds.X < search.Bounds.X+search.Bounds.Width || title.Bounds.X+title.Bounds.Width > size.width {
			t.Fatalf("invalid sidebar/editor bounds at %+v: search=%+v title=%+v", size, search.Bounds, title.Bounds)
		}
		second := find("k:note-1/view#press")
		position := ui.Point{X: second.Bounds.X + second.Bounds.Width/2, Y: second.Bounds.Y + second.Bounds.Height/2}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: position})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: position})
		frame()
		activeTitle := find("k:title-1/text-field/field")
		if s.selected != 1 || activeTitle.Value != "Second" {
			t.Fatal("clicking the sidebar did not switch the editor")
		}
		if !host.FocusAutomationID(activeTitle.AutomationID) {
			t.Fatal("cannot focus the selected note title")
		}
		if err := host.SelectAllWithin(widget.Key(activeTitle.AutomationID)); err != nil {
			t.Fatal(err)
		}
		host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "Edited"})
		frame()
		if s.notes[1].title != "Edited" || find("k:title-1/text-field/field").Value != "Edited" {
			t.Fatal("editing did not update the selected note")
		}
		check := find("k:task-1-0/checkbox")
		position = ui.Point{X: check.Bounds.X + check.Bounds.Width/2, Y: check.Bounds.Y + check.Bounds.Height/2}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: position})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: position})
		frame()
		if !s.notes[1].tasks[0] || s.notes[0].tasks[0] {
			t.Fatal("checkbox did not update the selected note")
		}
		host.Dispose()
	}
}
