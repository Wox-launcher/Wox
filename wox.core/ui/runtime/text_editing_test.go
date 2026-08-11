package woxui

import (
	"runtime"
	"testing"
	"time"
)

func TestTextEditorSelectedTextAndDeleteSelection(t *testing.T) {
	editor := NewTextEditor("hello")
	if got := editor.SelectedText(); got != "" {
		t.Fatalf("initial selection should be empty, got %q", got)
	}
	editor.SetSelection(2, 4)
	if got := editor.SelectedText(); got != "ll" {
		t.Fatalf("expected selected %q, got %q", "ll", got)
	}
	if !editor.DeleteSelection() {
		t.Fatal("DeleteSelection should report true for a non-empty range")
	}
	if got := editor.State().Text; got != "heo" {
		t.Fatalf("after delete text = %q, want %q", got, "heo")
	}
	if anchor, focus := editor.State().Selection.Anchor, editor.State().Selection.Focus; anchor != 2 || focus != 2 {
		t.Fatalf("after delete caret = (%d,%d), want (2,2)", anchor, focus)
	}
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

func wordModifierForTest() KeyModifiers {
	if runtime.GOOS == "darwin" {
		return KeyModifierAlt
	}
	return KeyModifierControl
}

func TestTextEditorWordModifierBackspaceDeletesPreviousWord(t *testing.T) {
	word := wordModifierForTest()
	tests := []struct {
		name   string
		text   string
		caret  int
		want   string
		cursor int
	}{
		{name: "word at end", text: "color asdf", caret: 10, want: "color ", cursor: 6},
		{name: "word before suffix", text: "color asdf", caret: 5, want: " asdf", cursor: 0},
		// UAX segments CJK per ideograph; platform delete takes one segment when no whitespace.
		{name: "unicode word", text: "颜色 测试", caret: 5, want: "颜色 测", cursor: 4},
		// Trailing whitespace is skipped so the previous word is deleted with it.
		{name: "trailing whitespace", text: "color   ", caret: 8, want: "", cursor: 0},
		{name: "punctuation boundary", text: "alpha-beta", caret: 10, want: "alpha-", cursor: 6},
		{name: "emoji grapheme", text: "hi ✈️", caret: len([]rune("hi ✈️")), want: "hi ", cursor: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			editor := NewTextEditor(test.text)
			editor.SetCaret(test.caret)
			handled, changed := editor.HandleKey(KeyEvent{Key: KeyBackspace, Modifiers: word, Down: true})
			if !handled || !changed {
				t.Fatalf("word+backspace = handled %v changed %v", handled, changed)
			}
			state := editor.State()
			if state.Text != test.want || state.Selection != (TextSelection{Anchor: test.cursor, Focus: test.cursor}) {
				t.Fatalf("state = %#v, want text %q caret %d", state, test.want, test.cursor)
			}
		})
	}
}

func TestTextEditorWordModifierBackspaceDeletesSelectionAndSupportsUndo(t *testing.T) {
	editor := NewTextEditor("color asdf")
	editor.SetSelection(2, 8)
	word := wordModifierForTest()
	primary := KeyModifierControl | KeyModifierMeta

	handled, changed := editor.HandleKey(KeyEvent{Key: KeyBackspace, Modifiers: word, Down: true})
	if !handled || !changed || editor.State().Text != "codf" {
		t.Fatalf("word+backspace = handled %v changed %v state %#v", handled, changed, editor.State())
	}
	handled, changed = editor.HandleKey(KeyEvent{Key: Key("z"), Modifiers: primary, Down: true})
	if !handled || !changed || editor.State().Text != "color asdf" {
		t.Fatalf("undo = handled %v changed %v state %#v", handled, changed, editor.State())
	}
}

func TestTextEditorWordModifierBackspaceAtStartIsHandledWithoutChange(t *testing.T) {
	editor := NewTextEditor("color")
	editor.SetCaret(0)
	handled, changed := editor.HandleKey(KeyEvent{Key: KeyBackspace, Modifiers: wordModifierForTest(), Down: true})
	if !handled || changed || editor.State().Text != "color" {
		t.Fatalf("word+backspace = handled %v changed %v state %#v", handled, changed, editor.State())
	}
}

func TestTextEditorGraphemeSafeArrowAndDelete(t *testing.T) {
	editor := NewTextEditor("a👨‍👩‍👧‍👦b")
	editor.SetCaret(len([]rune("a👨‍👩‍👧‍👦b")))
	handled, changed := editor.HandleKey(KeyEvent{Key: KeyArrowLeft, Down: true})
	if !handled || changed {
		t.Fatalf("arrow left = handled %v changed %v", handled, changed)
	}
	if focus := editor.State().Selection.Focus; focus != len([]rune("a👨‍👩‍👧‍👦")) {
		t.Fatalf("caret after left = %d, want after family emoji", focus)
	}
	handled, changed = editor.HandleKey(KeyEvent{Key: KeyBackspace, Down: true})
	if !handled || !changed || editor.State().Text != "ab" {
		t.Fatalf("backspace emoji = handled %v changed %v text %q", handled, changed, editor.State().Text)
	}
}

func TestTextEditorWordArrowAndDeleteForward(t *testing.T) {
	editor := NewTextEditor("one two three")
	editor.SetCaret(0)
	word := wordModifierForTest()
	handled, changed := editor.HandleKey(KeyEvent{Key: KeyArrowRight, Modifiers: word, Down: true})
	if !handled || changed || editor.State().Selection.Focus != 3 {
		t.Fatalf("word right = handled %v changed %v focus %d", handled, changed, editor.State().Selection.Focus)
	}
	// Platform forward delete covers intervening whitespace plus the next word.
	handled, changed = editor.HandleKey(KeyEvent{Key: KeyDelete, Modifiers: word, Down: true})
	if !handled || !changed || editor.State().Text != "one three" {
		t.Fatalf("word delete = handled %v changed %v text %q", handled, changed, editor.State().Text)
	}
}

func TestTextEditorWordMoveSkipsWhitespace(t *testing.T) {
	editor := NewTextEditor("one two")
	editor.SetCaret(4)
	word := wordModifierForTest()
	handled, changed := editor.HandleKey(KeyEvent{Key: KeyArrowLeft, Modifiers: word, Down: true})
	if !handled || changed || editor.State().Selection.Focus != 0 {
		t.Fatalf("word left over space = handled %v changed %v focus %d", handled, changed, editor.State().Selection.Focus)
	}
	editor.SetCaret(3)
	handled, changed = editor.HandleKey(KeyEvent{Key: KeyArrowRight, Modifiers: word, Down: true})
	if !handled || changed || editor.State().Selection.Focus != 7 {
		t.Fatalf("word right over space = handled %v changed %v focus %d", handled, changed, editor.State().Selection.Focus)
	}
}

func TestTextEditorTypingUndoDoesNotMergeAcrossCaretMoves(t *testing.T) {
	editor := NewTextEditor("")
	if !editor.InsertText("a") {
		t.Fatal("insert a failed")
	}
	editor.lastUndoAt = time.Now()
	editor.SetCaret(0)
	if !editor.InsertText("b") {
		t.Fatal("insert b failed")
	}
	primary := KeyModifierControl | KeyModifierMeta
	handled, changed := editor.HandleKey(KeyEvent{Key: Key("z"), Modifiers: primary, Down: true})
	if !handled || !changed || editor.State().Text != "a" {
		t.Fatalf("undo after caret move = handled %v changed %v text %q", handled, changed, editor.State().Text)
	}
}

func TestMapProtectedDisplayOffsetToRune(t *testing.T) {
	text := "a👨‍👩‍👧‍👦b"
	if got := MapProtectedDisplayOffsetToRune(text, 0); got != 0 {
		t.Fatalf("offset 0 = %d", got)
	}
	if got := MapProtectedDisplayOffsetToRune(text, 1); got != 1 {
		t.Fatalf("offset 1 = %d, want after first grapheme", got)
	}
	if got := MapProtectedDisplayOffsetToRune(text, 2); got != len([]rune("a👨‍👩‍👧‍👦")) {
		t.Fatalf("offset 2 = %d, want after family emoji", got)
	}
	if got := MapProtectedDisplayOffsetToRune(text, 3); got != len([]rune(text)) {
		t.Fatalf("offset 3 = %d, want end", got)
	}
}

func TestTextEditorTypingMergesUndoHistory(t *testing.T) {
	editor := NewTextEditor("")
	for _, letter := range []string{"h", "e", "l", "l", "o"} {
		if !editor.InsertText(letter) {
			t.Fatalf("insert %q failed", letter)
		}
		// Keep merges deterministic under fast CI clocks.
		editor.lastUndoAt = time.Now()
	}
	primary := KeyModifierControl | KeyModifierMeta
	handled, changed := editor.HandleKey(KeyEvent{Key: Key("z"), Modifiers: primary, Down: true})
	if !handled || !changed || editor.State().Text != "" {
		t.Fatalf("merged undo = handled %v changed %v text %q", handled, changed, editor.State().Text)
	}
}

func TestTextEditorPasteDoesNotMergeWithTypingUndo(t *testing.T) {
	editor := NewTextEditor("")
	if !editor.InsertText("hi") {
		t.Fatal("typing insert failed")
	}
	if !editor.InsertTextSeparate(" there") {
		t.Fatal("paste insert failed")
	}
	primary := KeyModifierControl | KeyModifierMeta
	handled, changed := editor.HandleKey(KeyEvent{Key: Key("z"), Modifiers: primary, Down: true})
	if !handled || !changed || editor.State().Text != "hi" {
		t.Fatalf("paste undo = handled %v changed %v text %q", handled, changed, editor.State().Text)
	}
}

func TestTextEditorDocumentHomeEnd(t *testing.T) {
	editor := NewTextEditor("alpha\nbeta")
	editor.SetCaret(8)
	handled, changed := editor.HandleKey(KeyEvent{Key: KeyHome, Down: true})
	if !handled || changed || editor.State().Selection.Focus != 0 {
		t.Fatalf("document home = handled %v changed %v focus %d", handled, changed, editor.State().Selection.Focus)
	}
	handled, changed = editor.HandleKey(KeyEvent{Key: KeyEnd, Down: true})
	if !handled || changed || editor.State().Selection.Focus != len([]rune("alpha\nbeta")) {
		t.Fatalf("document end = handled %v changed %v focus %d", handled, changed, editor.State().Selection.Focus)
	}
}

func TestFilterSingleLineNewlines(t *testing.T) {
	if got := FilterSingleLineNewlines("a\r\nb\nc"); got != "abc" {
		t.Fatalf("filter = %q, want abc", got)
	}
}

func TestMaskProtectedTextUsesGraphemes(t *testing.T) {
	masked := MaskProtectedText("a👨‍👩‍👧‍👦")
	if got := []rune(masked); len(got) != 2 || string(got) != "••" {
		t.Fatalf("masked = %q len %d, want two bullets", masked, len(got))
	}
	selection := MapSelectionToProtectedDisplay("a👨‍👩‍👧‍👦", TextSelection{Anchor: 1, Focus: len([]rune("a👨‍👩‍👧‍👦"))})
	if selection != (TextSelection{Anchor: 1, Focus: 2}) {
		t.Fatalf("mapped selection = %#v, want (1,2)", selection)
	}
}
