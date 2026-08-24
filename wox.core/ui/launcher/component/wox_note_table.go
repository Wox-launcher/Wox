package component

import (
	"fmt"
	"slices"

	"github.com/google/uuid"

	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	noteTableCellHeight    = float32(32)
	noteTableMinCell       = float32(120)
	noteTableToolbarHeight = float32(28)
	noteTableToolbarGap    = float32(4)
)

// NoteTableActionLabels holds already-translated table structure commands.
type NoteTableActionLabels struct {
	InsertRow, InsertColumn, DeleteRow, DeleteColumn, DeleteTable string
}

// NoteTableActions exposes structure commands above the grid while the caret is inside the table.
type NoteTableActions struct {
	Labels         NoteTableActionLabels
	OnInsertRow    func()
	OnInsertColumn func()
	OnDeleteRow    func()
	OnDeleteColumn func()
	OnDeleteTable  func()
	OnHover        func(inside bool, label string, bounds woxui.Rect)
}

// noteTableCellPadding is a non-zero inset so WoxTextField does not apply form-field defaults.
var noteTableCellPadding = woxwidget.Insets{Top: 2, Bottom: 2}

// NoteTableProps describes one editable Notes table block.
type NoteTableProps struct {
	ID       string
	Block    int
	Table    common.NoteTable
	Width    float32
	ReadOnly bool
	Theme    Theme
	Window   *woxui.Window
	Zoom     float32
	Style    woxui.TextStyle
	Focused  bool
	FocusRow int
	FocusCol int
	OnChange func(common.NoteTable)
	OnFocus  func(row, column int)
	OnKey    func(row, column int, event woxui.KeyEvent) bool
	OnPaste  func(row, column int, value string) bool
	OnUndo   func() bool
	OnRedo   func() bool
	Actions  NoteTableActions
}

// WoxNoteTable builds an editable GFM table at Notes editor density.
func WoxNoteTable(props NoteTableProps) woxwidget.Widget {
	if len(props.Table.Rows) == 0 {
		return woxwidget.Container{Width: props.Width}
	}
	table := &props.Table
	columns := 1
	for _, row := range table.Rows {
		columns = max(columns, len(row))
	}
	cellWidth := max(noteTableMinCell, props.Width/float32(columns))
	contentWidth := cellWidth * float32(columns)
	rows := make([]woxwidget.Widget, 0, len(table.Rows))
	for rowIndex, row := range table.Rows {
		cells := make([]woxwidget.Widget, 0, columns)
		for column := 0; column < columns; column++ {
			cell := common.NoteTableCell{}
			if column < len(row) {
				cell = row[column]
			}
			cells = append(cells, noteTableCellField(props, *table, rowIndex, column, cell, cellWidth))
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells})
	}
	gridHeight := float32(len(rows)) * noteTableRowHeight(props.Zoom)
	grid := woxwidget.ScrollView{
		Key: woxwidget.Key(props.ID + "-hscroll"), ID: props.ID + "-hscroll",
		Width: props.Width, Height: gridHeight, ContentWidth: contentWidth, Horizontal: true,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}
	var child woxwidget.Widget = grid
	if toolbar := noteTableToolbar(props); toolbar != nil {
		child = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: noteTableToolbarGap, Children: []woxwidget.Widget{toolbar, grid}}
	}
	return woxwidget.Semantics{
		Key: woxwidget.Key(props.ID), AutomationID: props.ID, Role: woxui.AccessibilityRoleGroup, Label: "Table",
		Child: child,
	}
}

func noteTableHasActions(props NoteTableProps) bool {
	return !props.ReadOnly && props.Actions.OnInsertRow != nil && props.Actions.OnInsertColumn != nil &&
		props.Actions.OnDeleteRow != nil && props.Actions.OnDeleteColumn != nil && props.Actions.OnDeleteTable != nil
}

func noteTableBlockHeight(table common.NoteTable, zoom float32, actions bool) float32 {
	height := float32(max(1, len(table.Rows))) * noteTableRowHeight(zoom)
	if actions {
		height += noteTableToolbarGap + noteTableToolbarHeight
	}
	return height
}

func noteTableToolbar(props NoteTableProps) woxwidget.Widget {
	if !noteTableHasActions(props) {
		return nil
	}
	if !props.Focused {
		// Keep the same 28-unit slot while idle so revealing the bar does not push the grid down.
		return woxwidget.Container{Width: props.Width, Height: noteTableToolbarHeight}
	}
	color := props.Theme.ResultSubtitle
	if color.A == 0 {
		color = props.Theme.PreviewText
	}
	button := func(id, label string, action func()) woxwidget.Widget {
		return WoxIconButton(IconButtonProps{
			ID: props.ID + "." + id, Label: label, Icon: FormatGlyph(id, 16, color),
			Width: noteTableToolbarHeight, Height: noteTableToolbarHeight, Radius: 6,
			HoverBackground: TitleBarAlpha(color, 20), FocusRingColor: props.Theme.Cursor, OnTap: action,
			OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.Actions.OnHover != nil {
					props.Actions.OnHover(inside, label, bounds)
				}
			},
		})
	}
	labels := props.Actions.Labels
	// SpaceBetween keeps delete on the trailing edge. A Stack+Align pair cannot:
	// Align without Width fills the stack, so AnchorRight places a full-width child at x=0.
	return woxwidget.Container{Width: props.Width, Height: noteTableToolbarHeight, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisSpaceBetween, CrossAxisAlignment: woxwidget.CrossAxisCenter,
		Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				button("table-insert-row", labels.InsertRow, props.Actions.OnInsertRow),
				button("table-insert-column", labels.InsertColumn, props.Actions.OnInsertColumn),
				button("table-delete-row", labels.DeleteRow, props.Actions.OnDeleteRow),
				button("table-delete-column", labels.DeleteColumn, props.Actions.OnDeleteColumn),
			}},
			button("table-delete", labels.DeleteTable, props.Actions.OnDeleteTable),
		},
	}}
}

func noteTableCellField(props NoteTableProps, table common.NoteTable, row, column int, cell common.NoteTableCell, width float32) woxwidget.Widget {
	background := woxui.Color{}
	weight := props.Style.Weight
	if row < table.HeaderRows {
		weight = woxui.FontWeightSemibold
		background = withAlpha(props.Theme.PreviewText, 12)
	}
	focused := props.Focused && props.FocusRow == row && props.FocusCol == column
	runs := NoteFieldRuns(noteTableCellRuns(cell, woxui.TextStyle{Size: props.Style.Size, Weight: weight, Family: props.Style.Family}, props.Theme))
	rowIndex, colIndex := row, column
	cellHeight := noteTableRowHeight(props.Zoom)
	lineHeight := noteTableLineHeight(props.Zoom)
	return woxwidget.Container{
		Width: width, Height: cellHeight, Color: background,
		BorderColor: withAlpha(props.Theme.PreviewSplit, 100), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 8, Right: 8},
		Child: WoxTextField(TextFieldProps{
			ID: fmt.Sprintf("%s.%d.%d", props.ID, row, column), Label: "Table cell",
			Width: max(float32(0), width-16), Height: cellHeight - 2, Padding: noteTableCellPadding,
			Transparent: true, DisableHover: true,
			Style:    woxui.TextStyle{Size: props.Style.Size, Weight: weight, Family: props.Style.Family},
			RichRuns: runs, LineHeight: lineHeight, TextAlignmentY: 0.5,
			TextColor: props.Theme.PreviewText, Value: cell.Text, Focused: focused, ReadOnly: props.ReadOnly,
			MaxLines: 1, Window: props.Window, Theme: props.Theme,
			OnFocusChange: func(hasFocus bool) {
				if hasFocus && props.OnFocus != nil {
					props.OnFocus(rowIndex, colIndex)
				}
			},
			OnChanged: func(value string) {
				if props.OnChange == nil {
					return
				}
				updated := table
				updated.Rows = append([][]common.NoteTableCell(nil), table.Rows...)
				if rowIndex < len(updated.Rows) {
					updated.Rows[rowIndex] = append([]common.NoteTableCell(nil), updated.Rows[rowIndex]...)
					if colIndex < len(updated.Rows[rowIndex]) {
						updated.Rows[rowIndex][colIndex].Text = value
					}
				}
				props.OnChange(updated)
			},
			OnKey: func(event woxui.KeyEvent) bool {
				if props.OnKey != nil {
					return props.OnKey(rowIndex, colIndex, event)
				}
				return false
			},
			OnPaste: func(value string) bool {
				if props.OnPaste != nil {
					return props.OnPaste(rowIndex, colIndex, value)
				}
				return false
			},
			OnUndo: props.OnUndo,
			OnRedo: props.OnRedo,
		}),
	}
}

func noteTableCellRuns(cell common.NoteTableCell, base woxui.TextStyle, theme Theme) []NoteTextRun {
	styles := noteBlockStyles(common.NoteBlock{Text: cell.Text, Spans: cell.Spans}, cell.Text)
	runs := make([]NoteTextRun, 0)
	for offset := 0; offset < len(styles); {
		end := offset + 1
		for end < len(styles) && styles[end] == styles[offset] {
			end++
		}
		style := base
		inline := styles[offset]
		if inline.bold {
			style.Weight = woxui.FontWeightSemibold
		}
		style.Italic = inline.italic
		if inline.code {
			style.Family = woxui.FontFamilyMonospace
		}
		color := woxui.Color{}
		if NoteOpenableLink(inline.link) != "" {
			color = theme.Cursor
		}
		runs = append(runs, NoteTextRun{
			Start: offset, End: end, Style: style, Color: color,
			Underline: inline.underline || inline.link != "", Strike: inline.strike,
		})
		offset = end
	}
	return runs
}

// InsertNoteTable places a default 3x2 table at the caret block and keeps a trailing paragraph after the last table.
func InsertNoteTable(document common.NoteDocument, ranges []NoteBlockRange, selection woxui.TextSelection) (common.NoteDocument, int) {
	updated := CloneNoteDocument(document)
	if len(updated.Blocks) == 0 {
		updated.Blocks = []common.NoteBlock{{ID: uuid.NewString(), Type: common.NoteBlockParagraph}}
	}
	index := 0
	if len(ranges) > 0 {
		index = NoteBlockAt(ranges, selection.Focus)
	}
	table := defaultNoteTable()
	block := common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockTable, Table: &table, Text: noteTableMarkdown(table)}
	if updated.Blocks[index].Type == common.NoteBlockParagraph && updated.Blocks[index].Text == "" {
		updated.Blocks[index] = block
	} else {
		updated.Blocks = slices.Insert(updated.Blocks, index+1, block)
		index++
	}
	if index+1 >= len(updated.Blocks) {
		updated.Blocks = slices.Insert(updated.Blocks, index+1, common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph})
	}
	return ensureNoteDocumentHasTextBlock(updated), index
}

// InsertNoteTableRow inserts an empty row after the focused row.
func InsertNoteTableRow(document common.NoteDocument, block, row int) common.NoteDocument {
	table := noteTableCopy(document, block)
	if table == nil {
		return document
	}
	columns := 1
	for _, existing := range table.Rows {
		columns = max(columns, len(existing))
	}
	cells := make([]common.NoteTableCell, columns)
	table.Rows = slices.Insert(table.Rows, min(len(table.Rows), max(0, row+1)), cells)
	return replaceNoteTable(document, block, table)
}

// InsertNoteTableColumn inserts an empty column after the focused column.
func InsertNoteTableColumn(document common.NoteDocument, block, column int) common.NoteDocument {
	table := noteTableCopy(document, block)
	if table == nil {
		return document
	}
	insertAt := min(column+1, noteTableColumns(*table))
	for index := range table.Rows {
		table.Rows[index] = slices.Insert(table.Rows[index], min(insertAt, len(table.Rows[index])), common.NoteTableCell{})
	}
	return replaceNoteTable(document, block, table)
}

// DeleteNoteTableRow removes the focused row, keeping at least one row.
func DeleteNoteTableRow(document common.NoteDocument, block, row int) common.NoteDocument {
	table := noteTableCopy(document, block)
	if table == nil || len(table.Rows) <= 1 || row < 0 || row >= len(table.Rows) {
		return document
	}
	table.Rows = slices.Delete(table.Rows, row, row+1)
	if table.HeaderRows > len(table.Rows) {
		table.HeaderRows = len(table.Rows)
	}
	return replaceNoteTable(document, block, table)
}

// DeleteNoteTableColumn removes the focused column, keeping at least one column.
func DeleteNoteTableColumn(document common.NoteDocument, block, column int) common.NoteDocument {
	table := noteTableCopy(document, block)
	if table == nil || noteTableColumns(*table) <= 1 {
		return document
	}
	for index := range table.Rows {
		if column >= 0 && column < len(table.Rows[index]) {
			table.Rows[index] = slices.Delete(table.Rows[index], column, column+1)
		}
	}
	return replaceNoteTable(document, block, table)
}

// DeleteNoteTable removes the table block and leaves a paragraph in its place when needed.
func DeleteNoteTable(document common.NoteDocument, block int) common.NoteDocument {
	if block < 0 || block >= len(document.Blocks) {
		return document
	}
	updated := CloneNoteDocument(document)
	updated.Blocks = slices.Delete(updated.Blocks, block, block+1)
	if len(updated.Blocks) == 0 {
		updated.Blocks = []common.NoteBlock{{ID: uuid.NewString(), Type: common.NoteBlockParagraph}}
	}
	return updated
}

func noteTableCopy(document common.NoteDocument, block int) *common.NoteTable {
	if block < 0 || block >= len(document.Blocks) || document.Blocks[block].Table == nil {
		return nil
	}
	clone := CloneNoteDocument(common.NoteDocument{Blocks: []common.NoteBlock{document.Blocks[block]}})
	return clone.Blocks[0].Table
}

// ReplaceNoteTable writes one table payload back onto its document block.
func ReplaceNoteTable(document common.NoteDocument, block int, table *common.NoteTable) common.NoteDocument {
	return replaceNoteTable(document, block, table)
}

func replaceNoteTable(document common.NoteDocument, block int, table *common.NoteTable) common.NoteDocument {
	updated := CloneNoteDocument(document)
	if block < 0 || block >= len(updated.Blocks) || table == nil {
		return document
	}
	updated.Blocks[block].Type = common.NoteBlockTable
	updated.Blocks[block].Table = table
	updated.Blocks[block].Text = noteTableMarkdown(*table)
	return updated
}

func noteTableZoom(zoom float32) float32 {
	if zoom <= 0 {
		return 1
	}
	return zoom
}

func noteTableRowHeight(zoom float32) float32 {
	return noteTableCellHeight * noteTableZoom(zoom)
}

func noteTableLineHeight(zoom float32) float32 {
	return 24 * noteTableZoom(zoom)
}

func noteTableColumns(table common.NoteTable) int {
	columns := 1
	for _, row := range table.Rows {
		columns = max(columns, len(row))
	}
	return columns
}

// NextNoteTableCell moves within the table and reports when the move leaves the grid.
func NextNoteTableCell(table common.NoteTable, row, column, rowDelta, colDelta int) (int, int, bool) {
	return nextNoteTableCell(table, row, column, rowDelta, colDelta)
}

func nextNoteTableCell(table common.NoteTable, row, column, rowDelta, colDelta int) (int, int, bool) {
	columns := noteTableColumns(table)
	row += rowDelta
	column += colDelta
	if colDelta != 0 {
		if column >= columns {
			column = 0
			row++
		}
		if column < 0 {
			column = columns - 1
			row--
		}
	}
	if row < 0 || row >= len(table.Rows) {
		return row, column, false
	}
	return row, min(column, columns-1), true
}
