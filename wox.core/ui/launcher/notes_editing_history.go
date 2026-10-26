package launcher

import (
	"time"
	"unicode/utf8"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	woxui "wox/ui/runtime"
)

// rememberDocumentUndo keeps bounded document snapshots and coalesces adjacent typing.
func (c *notesEditingSession) rememberDocumentUndo(document common.NoteDocument, coalesce bool) {
	if coalesce && len(c.undoDocuments) > 0 {
		return
	}
	if !coalesce {
		c.lastTextEdit = time.Time{}
	}
	if len(c.undoDocuments) >= 100 {
		copy(c.undoDocuments, c.undoDocuments[1:])
		c.undoDocuments = c.undoDocuments[:99]
	}
	c.undoDocuments = append(c.undoDocuments, noteUndoEntry{
		document:         cloneNoteDocument(document),
		selection:        c.selection,
		segment:          c.activeTextSegment,
		tableBlock:       c.focusedTableBlock,
		imageBlock:       c.focusedImageBlock,
		documentSelected: c.documentSelected,
	})
	c.redoDocuments = nil
}

// liveUndoEntry captures the current document and caret for the opposite history stack.
func (c *notesEditingSession) liveUndoEntry() noteUndoEntry {
	return noteUndoEntry{
		document:         cloneNoteDocument(c.document),
		selection:        c.editor.State().Selection,
		segment:          c.activeTextSegment,
		tableBlock:       c.focusedTableBlock,
		imageBlock:       c.focusedImageBlock,
		documentSelected: c.documentSelected,
	}
}

func (c *notesEditingSession) undoDocument() bool {
	if len(c.undoDocuments) == 0 || c.record.DeletedAt > 0 {
		return true
	}
	previous := c.undoDocuments[len(c.undoDocuments)-1]
	c.undoDocuments = c.undoDocuments[:len(c.undoDocuments)-1]
	c.redoDocuments = append(c.redoDocuments, c.liveUndoEntry())
	c.applyUndoEntry(previous)
	return true
}

func (c *notesEditingSession) redoDocument() bool {
	if len(c.redoDocuments) == 0 || c.record.DeletedAt > 0 {
		return true
	}
	next := c.redoDocuments[len(c.redoDocuments)-1]
	c.redoDocuments = c.redoDocuments[:len(c.redoDocuments)-1]
	c.undoDocuments = append(c.undoDocuments, c.liveUndoEntry())
	c.applyUndoEntry(next)
	return true
}

// applyUndoEntry restores a snapshot and the caret that belonged to that version.
func (c *notesEditingSession) applyUndoEntry(entry noteUndoEntry) {
	c.document = entry.document
	c.activeTextSegment = entry.segment
	c.focusedTableBlock = entry.tableBlock
	c.focusedImageBlock = entry.imageBlock
	c.documentSelected = entry.documentSelected
	c.lastTextEdit = time.Time{}
	c.syncEditorAfterDocumentChange(entry.selection)
	if c.documentSelected {
		c.focusedImageBlock = -1
		c.focusedTableBlock = -1
		c.editor.SelectAll()
		c.selection = c.editor.State().Selection
	}
}

// syncEditorAfterDocumentChange refreshes the visible editor after undo or redo.
func (c *notesEditingSession) syncEditorAfterDocumentChange(selection woxui.TextSelection) {
	if c.markdownView {
		value := notesplugin.ToMarkdown(c.document)
		c.editor.SetText(value, false)
		c.richRuns, c.blockRanges = nil, nil
		c.setEditorSelection(selection, utf8.RuneCountInString(value))
		c.edited()
		return
	}
	value, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	c.editor.SetText(value, false)
	c.setEditorSelection(selection, utf8.RuneCountInString(value))
	c.selection = c.editor.State().Selection
	c.edited()
}

// setEditorSelection clamps a restored caret onto the current editor text.
func (c *notesEditingSession) setEditorSelection(selection woxui.TextSelection, limit int) {
	c.editor.SetSelection(min(max(0, selection.Anchor), limit), min(max(0, selection.Focus), limit))
	c.selection = c.editor.State().Selection
}
