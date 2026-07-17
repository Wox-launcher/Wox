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