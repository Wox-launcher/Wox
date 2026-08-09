package woxui

import "testing"

func TestTextEditorSelectedTextAndDeleteSelection(t *testing.T) {
	editor := NewTextEditor("hello")
	if got := editor.SelectedText(); got != "" {
		t.Fatalf("initial selection should be empty, got %q", got)
	}
	// Select "ll" (runes 2..4) and verify SelectedText reports it.
	editor.SetSelection(2, 4)
	if got := editor.SelectedText(); got != "ll" {
		t.Fatalf("expected selected %q, got %q", "ll", got)
	}
	// DeleteSelection should collapse to the start of the deleted range.
	if !editor.DeleteSelection() {
		t.Fatal("DeleteSelection should report true for a non-empty range")
	}
	if got := editor.State().Text; got != "heo" {
		t.Fatalf("after delete text = %q, want %q", got, "heo")
	}
	if anchor, focus := editor.State().Selection.Anchor, editor.State().Selection.Focus; anchor != 2 || focus != 2 {
		t.Fatalf("after delete caret = (%d,%d), want (2,2)", anchor, focus)
	}
	// DeleteSelection on a collapsed caret is a no-op.
	if editor.DeleteSelection() {
		t.Fatal("DeleteSelection on collapsed caret should return false")
	}
}

func TestTextEditorInsertTextReplacesSelection(t *testing.T) {
	editor := NewTextEditor("hello")
	editor.SetSelection(1, 3)
	if !editor.InsertText("XYZ") {
		t.Fatal("InsertText should report true when text is committed")
	}
	if got := editor.State().Text; got != "hXYZlo" {
		t.Fatalf("after insert text = %q, want %q", got, "hXYZlo")
	}
	if anchor, focus := editor.State().Selection.Anchor, editor.State().Selection.Focus; anchor != 4 || focus != 4 {
		t.Fatalf("after insert caret = (%d,%d), want (4,4)", anchor, focus)
	}
}

func TestTextEditorMultiClickSelections(t *testing.T) {
	editor := NewTextEditor("alpha_beta 世界\nsecond line")
	editor.SelectWordAt(3)
	if selected := editor.SelectedText(); selected != "alpha_beta" {
		t.Fatalf("word selection = %q, want %q", selected, "alpha_beta")
	}
	editor.SelectWordAt(11)
	if selected := editor.SelectedText(); selected != "世界" {
		t.Fatalf("Unicode word selection = %q, want %q", selected, "世界")
	}
	editor.SelectLineAt(17)
	if selected := editor.SelectedText(); selected != "second line" {
		t.Fatalf("line selection = %q, want %q", selected, "second line")
	}
}

func TestTextEditorSelectAllUndoAndRedoShortcuts(t *testing.T) {
	editor := NewTextEditor("first\nsecond")
	editor.SetCaret(5)
	if !editor.InsertText(" changed") {
		t.Fatal("insert should change text")
	}
	changedText := editor.State().Text
	primary := KeyModifierControl | KeyModifierMeta

	handled, changed := editor.HandleKey(KeyEvent{Key: Key("a"), Modifiers: primary, Down: true})
	if !handled || changed || editor.SelectedText() != changedText {
		t.Fatalf("select all = handled %v changed %v selected %q", handled, changed, editor.SelectedText())
	}

	handled, changed = editor.HandleKey(KeyEvent{Key: Key("z"), Modifiers: primary, Down: true})
	if !handled || !changed || editor.State().Text != "first\nsecond" {
		t.Fatalf("undo = handled %v changed %v text %q", handled, changed, editor.State().Text)
	}

	handled, changed = editor.HandleKey(KeyEvent{Key: Key("z"), Modifiers: primary | KeyModifierShift, Down: true})
	if !handled || !changed || editor.State().Text != changedText {
		t.Fatalf("shift redo = handled %v changed %v text %q", handled, changed, editor.State().Text)
	}

	editor.HandleKey(KeyEvent{Key: Key("z"), Modifiers: primary, Down: true})
	handled, changed = editor.HandleKey(KeyEvent{Key: Key("y"), Modifiers: primary, Down: true})
	if !handled || !changed || editor.State().Text != changedText {
		t.Fatalf("ctrl+y redo = handled %v changed %v text %q", handled, changed, editor.State().Text)
	}
}
