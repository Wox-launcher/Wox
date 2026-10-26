package launcher

import (
	"testing"
	"time"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	woxui "wox/ui/runtime"
)

// TestNotesSessionHistory exercises document and caret transitions without App or a native host.
func TestNotesSessionHistory(t *testing.T) {
	for _, markdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "preview", true: "markdown"}[markdown], func(t *testing.T) {
			session := newNotesEditingSession()
			session.markdownView = markdown
			session.load(common.NoteRecord{ID: "note", Document: notesplugin.ParseMarkdown("hello world")})
			changes := 0
			session.onChanged = func() { changes++ }
			session.editor.SetSelection(5, 5)
			session.onEditorSelectionChanged(session.editor.State().Selection)
			session.editor.SetText("helloXY world", false)
			session.editor.SetSelection(7, 7)
			if markdown {
				session.onMarkdownChanged(session.editor.Text())
			} else {
				session.onEditorChanged(session.editor.Text())
			}
			session.undoDocument()
			if session.editor.Text() != "hello world" || session.selection != (woxui.TextSelection{Anchor: 5, Focus: 5}) {
				t.Fatalf("undo = %q, %+v", session.editor.Text(), session.selection)
			}
			session.redoDocument()
			if session.editor.Text() != "helloXY world" || session.selection != (woxui.TextSelection{Anchor: 7, Focus: 7}) {
				t.Fatalf("redo = %q, %+v", session.editor.Text(), session.selection)
			}
			if changes != 3 || !session.dirty {
				t.Fatalf("edits must notify persistence once each: changes=%d dirty=%v", changes, session.dirty)
			}
		})
	}
}

func TestNotesSessionStructuralSelectionAndHistory(t *testing.T) {
	session := newNotesEditingSession()
	session.load(common.NoteRecord{ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "before", Type: common.NoteBlockParagraph, Text: "before"},
		{ID: "image", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "missing.png", Width: 100, Height: 100}},
		{ID: "table", Type: common.NoteBlockTable, Table: &common.NoteTable{Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}}}},
		{ID: "after", Type: common.NoteBlockParagraph, Text: "after"},
	}}})
	if !session.selectEntireDocument() || !session.clearDocumentSelection() || !notesplugin.DocumentIsEmpty(session.document) {
		t.Fatal("whole-document selection must delete text, images, and tables")
	}
	session.undoDocument()
	if !session.documentSelected || !noteDocumentHasImage(session.document) || session.editor.SelectedText() != "before" {
		t.Fatal("undo must restore the full document and selection")
	}
	tableBlock := -1
	for i, block := range session.document.Blocks {
		if block.Table != nil {
			tableBlock = i
		}
	}
	if tableBlock < 0 {
		t.Fatal("undo lost the table")
	}
	session.focusTableCell(tableBlock, 0, 1)
	session.tableDeleteColumn(tableBlock)
	session.undoDocument()
	if table := session.document.Blocks[tableBlock].Table; len(table.Rows[0]) != 2 || table.Rows[0][1].Text != "B" {
		t.Fatalf("table history was aliased by column deletion: %+v", table)
	}
	session.redoDocument()
	if len(session.document.Blocks[tableBlock].Table.Rows[0]) != 1 {
		t.Fatal("redo must reapply column deletion")
	}
}

func TestNotesSessionSwitchResetsHistoryAndTypingGroup(t *testing.T) {
	session := newNotesEditingSession()
	session.load(common.NoteRecord{ID: "first", Document: notesplugin.ParseMarkdown("first")})
	session.onEditorChanged("edited")
	session.undoDocument()
	session.lastTextEdit = time.Now()
	session.load(common.NoteRecord{ID: "second", Document: notesplugin.ParseMarkdown("second")})
	if session.dirty || len(session.undoDocuments) != 0 || len(session.redoDocuments) != 0 || !session.lastTextEdit.IsZero() {
		t.Fatal("a new record inherited edit history or typing coalescence")
	}
	session.onEditorChanged("second edit")
	session.undoDocument()
	if session.editor.Text() != "second" {
		t.Fatalf("undo crossed the record boundary: %q", session.editor.Text())
	}
}

func TestNotesSessionMarkdownRoundTripWithoutWindow(t *testing.T) {
	session := newNotesEditingSession()
	session.load(common.NoteRecord{ID: "note", Document: notesplugin.ParseMarkdown("hello **world**")})
	session.setEditorSelection(woxui.TextSelection{Anchor: 3, Focus: 3}, len(session.editor.Text()))
	session.toggleMarkdownView()
	if !session.markdownView || session.editor.Text() != "hello **world**" {
		t.Fatalf("Markdown projection = %q", session.editor.Text())
	}
	session.toggleMarkdownView()
	if session.markdownView || session.selection.Focus != 3 || session.editor.Text() != "hello world" || session.dirty {
		t.Fatalf("preview round trip changed the edit: text=%q selection=%+v dirty=%v", session.editor.Text(), session.selection, session.dirty)
	}
}
