package launcher

import (
	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

func (c *notesEditingSession) replaceTable(block int, table common.NoteTable) {
	c.rememberDocumentUndo(c.document, true)
	c.document = woxcomponent.ReplaceNoteTable(c.document, block, &table)
	c.edited()
}

func (c *notesEditingSession) focusTableCell(block, row, column int) {
	c.documentSelected = false
	c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol = block, row, column
	c.focusedImageBlock = -1
	c.invalidate()
}

func (c *notesEditingSession) onTableKey(block, row, column int, event woxui.KeyEvent) bool {
	if !event.Down || event.Composing || block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Table == nil {
		return false
	}
	table := *c.document.Blocks[block].Table
	switch event.Key {
	case woxui.KeyTab:
		delta := 1
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		nextRow, nextCol, ok := woxcomponent.NextNoteTableCell(table, row, column, 0, delta)
		if ok {
			c.focusTableCell(block, nextRow, nextCol)
			return true
		}
		c.focusedTableBlock = -1
		c.editorFocus.RequestFocus()
		c.invalidate()
		return true
	case woxui.KeyEnter:
		nextRow, nextCol, ok := woxcomponent.NextNoteTableCell(table, row, column, 1, 0)
		if !ok {
			c.rememberDocumentUndo(c.document, false)
			c.document = woxcomponent.InsertNoteTableRow(c.document, block, row)
			c.focusTableCell(block, row+1, column)
			c.reproject(false)
			return true
		}
		c.focusTableCell(block, nextRow, nextCol)
		return true
	case woxui.KeyArrowUp:
		if row == 0 {
			c.focusedTableBlock = -1
			c.editorFocus.RequestFocus()
			c.invalidate()
			return true
		}
		c.focusTableCell(block, row-1, column)
		return true
	case woxui.KeyArrowDown:
		if row+1 >= len(table.Rows) {
			c.focusedTableBlock = -1
			c.editorFocus.RequestFocus()
			c.invalidate()
			return true
		}
		c.focusTableCell(block, row+1, column)
		return true
	default:
		return false
	}
}

func (c *notesEditingSession) insertTableRow() {
	c.tableInsertRow(c.focusedTableBlock)
}

func (c *notesEditingSession) insertTableColumn() {
	c.tableInsertColumn(c.focusedTableBlock)
}

func (c *notesEditingSession) deleteTableRow() {
	c.tableDeleteRow(c.focusedTableBlock)
}

func (c *notesEditingSession) deleteTableColumn() {
	c.tableDeleteColumn(c.focusedTableBlock)
}

func (c *notesEditingSession) deleteFocusedTable() {
	c.tableDelete(c.focusedTableBlock)
}

func (c *notesEditingSession) tableInsertRow(block int) {
	row, column, ok := c.tableActionTarget(block)
	if !ok {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.InsertNoteTableRow(c.document, block, row)
	c.focusTableCell(block, row+1, column)
	c.reproject(false)
}

func (c *notesEditingSession) tableInsertColumn(block int) {
	row, column, ok := c.tableActionTarget(block)
	if !ok {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.InsertNoteTableColumn(c.document, block, column)
	c.focusTableCell(block, row, column+1)
	c.reproject(false)
}

func (c *notesEditingSession) tableDeleteRow(block int) {
	row, column, ok := c.tableActionTarget(block)
	if !ok {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.DeleteNoteTableRow(c.document, block, row)
	if table := c.document.Blocks[block].Table; table != nil {
		c.focusTableCell(block, min(row, len(table.Rows)-1), column)
	}
	c.reproject(false)
}

func (c *notesEditingSession) tableDeleteColumn(block int) {
	row, column, ok := c.tableActionTarget(block)
	if !ok {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.DeleteNoteTableColumn(c.document, block, column)
	if table := c.document.Blocks[block].Table; table != nil {
		c.focusTableCell(block, row, min(column, noteTableColumnCount(*table)-1))
	}
	c.reproject(false)
}

// noteSegmentTouchesImage reports empty caret gaps that must stay so typing can land beside an image.
func noteSegmentTouchesImage(document common.NoteDocument, segment woxcomponent.NoteDocumentSegment) bool {
	if segment.Start > 0 && segment.Start <= len(document.Blocks) && document.Blocks[segment.Start-1].Type == common.NoteBlockImage {
		return true
	}
	return segment.End < len(document.Blocks) && document.Blocks[segment.End].Type == common.NoteBlockImage
}

// deleteEmptyTextSegment removes a blank paragraph so Backspace can close the gap between tables.
func (c *notesEditingSession) deleteEmptyTextSegment(segmentStart int) bool {
	if c.record.DeletedAt > 0 {
		return false
	}
	segment := woxcomponent.NoteSegmentAtBlock(c.document, segmentStart)
	if noteSegmentTouchesImage(c.document, segment) {
		return false
	}
	updated, ok := woxcomponent.RemoveEmptyNoteSegment(c.document, segment)
	if !ok {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = updated
	if prev := segment.Start - 1; prev >= 0 && prev < len(c.document.Blocks) && c.document.Blocks[prev].Type == common.NoteBlockTable {
		row := 0
		if table := c.document.Blocks[prev].Table; table != nil && len(table.Rows) > 0 {
			row = len(table.Rows) - 1
		}
		c.focusTableCell(prev, row, 0)
	} else {
		c.focusedTableBlock = -1
		if c.editorFocus != nil {
			c.editorFocus.RequestFocus()
		}
	}
	c.reproject(true)
	return true
}

func (c *notesEditingSession) tableDelete(block int) {
	if block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Table == nil {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.DeleteNoteTable(c.document, block)
	c.focusedTableBlock = -1
	c.reproject(false)
}

func (c *notesEditingSession) tableActionTarget(block int) (int, int, bool) {
	if block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Table == nil {
		return 0, 0, false
	}
	table := *c.document.Blocks[block].Table
	row, column := len(table.Rows)-1, 0
	if columns := noteTableColumnCount(table); columns > 0 {
		column = columns - 1
	}
	if c.focusedTableBlock == block {
		row, column = c.focusedTableRow, c.focusedTableCol
	}
	return row, column, true
}

func noteTableColumnCount(table common.NoteTable) int {
	columns := 1
	for _, row := range table.Rows {
		columns = max(columns, len(row))
	}
	return columns
}
