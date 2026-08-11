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

func TestTextEditorPrimaryBackspaceDeletesPreviousWord(t *testing.T) {
	primary := KeyModifierControl | KeyModifierMeta
	tests := []struct {
		name   string
		text   string
		caret  int
		want   string
		cursor int
	}{
		{name: "word at end", text: "color asdf", caret: 10, want: "color ", cursor: 6},
		{name: "word before suffix", text: "color asdf", caret: 5, want: " asdf", cursor: 0},
		{name: "unicode word", text: "颜色 测试", caret: 5, want: "颜色 ", cursor: 3},
		{name: "trailing whitespace", text: "color   ", caret: 8, want: "", cursor: 0},
		{name: "punctuation boundary", text: "alpha-beta", caret: 10, want: "alpha-", cursor: 6},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			editor := NewTextEditor(test.text)
			editor.SetCaret(test.caret)
			handled, changed := editor.HandleKey(KeyEvent{Key: KeyBackspace, Modifiers: primary, Down: true})
			if !handled || !changed {
				t.Fatalf("primary+backspace = handled %v changed %v", handled, changed)
			}
			state := editor.State()
			if state.Text != test.want || state.Selection != (TextSelection{Anchor: test.cursor, Focus: test.cursor}) {
				t.Fatalf("state = %#v, want text %q caret %d", state, test.want, test.cursor)
			}
		})
	}
}

func TestTextEditorPrimaryBackspaceDeletesSelectionAndSupportsUndo(t *testing.T) {
	editor := NewTextEditor("color asdf")
	editor.SetSelection(2, 8)
	primary := KeyModifierControl | KeyModifierMeta

	handled, changed := editor.HandleKey(KeyEvent{Key: KeyBackspace, Modifiers: primary, Down: true})
	if !handled || !changed || editor.State().Text != "codf" {
		t.Fatalf("primary+backspace = handled %v changed %v state %#v", handled, changed, editor.State())
	}
	handled, changed = editor.HandleKey(KeyEvent{Key: Key("z"), Modifiers: primary, Down: true})
	if !handled || !changed || editor.State().Text != "color asdf" {
		t.Fatalf("undo = handled %v changed %v state %#v", handled, changed, editor.State())
	}
}

func TestTextEditorPrimaryBackspaceAtStartIsHandledWithoutChange(t *testing.T) {
	editor := NewTextEditor("color")
	editor.SetCaret(0)
	handled, changed := editor.HandleKey(KeyEvent{Key: KeyBackspace, Modifiers: KeyModifierControl | KeyModifierMeta, Down: true})
	if !handled || changed || editor.State().Text != "color" {
		t.Fatalf("primary+backspace = handled %v changed %v state %#v", handled, changed, editor.State())
	}
}
