package launcher

import (
	"testing"
	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestNotesFindAcrossSegmentsAndTables(t *testing.T) {
	c := &notesWindowController{notesEditingSession: newNotesEditingSession(), findOpen: true, findEditor: woxwidget.NewTextEditingController("hello")}
	c.document = common.NoteDocument{Blocks: []common.NoteBlock{
		{Type: common.NoteBlockParagraph, Text: "😀 hello"},
		{Type: common.NoteBlockTable, Table: &common.NoteTable{Rows: [][]common.NoteTableCell{{{Text: "HELLO"}, {Text: "other"}}}}},
		{Type: common.NoteBlockParagraph, Text: "hello again"},
	}}
	matches := c.findMatches()
	if len(matches) != 3 || matches[0].Offset != 2 || matches[1].Row != 0 || matches[2].Block != 2 {
		t.Fatalf("matches: %+v", matches)
	}
	c.moveFind(-1)
	if c.findIndex != 2 {
		t.Fatal("previous did not wrap")
	}
	c.moveFind(1)
	if c.findIndex != 0 {
		t.Fatal("next did not wrap")
	}
	if c.dirty {
		t.Fatal("find changed the document")
	}
	c.markdownView = true
	c.editor.SetText("**hello** hello", false)
	if len(c.findMatches()) != 2 {
		t.Fatal("source find failed")
	}
	c.findEditor.SetText("none", false)
	if _, target := c.currentFind(); target != nil {
		t.Fatal("stale result after query changed")
	}
}

func TestNotesFindShortcutPreservesDocument(t *testing.T) {
	c := &notesWindowController{notesEditingSession: newNotesEditingSession(), findEditor: woxwidget.NewTextEditingController(""), findFocus: woxwidget.NewFocusNode()}
	if !c.onFindKey(woxui.KeyEvent{Key: woxui.Key("f"), Down: true, Modifiers: woxui.KeyModifierControl | woxui.KeyModifierMeta}) || !c.findOpen {
		t.Fatal("find shortcut not handled")
	}
	if !c.onFindKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || c.findOpen {
		t.Fatal("escape did not dismiss find")
	}
	if c.dirty {
		t.Fatal("find marked note dirty")
	}
}
