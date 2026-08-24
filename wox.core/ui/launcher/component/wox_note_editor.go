package component

import (
	"fmt"
	"strings"

	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const noteEditorSegmentGap = float32(8)

// NoteEditorProps describes the Notes document surface hosted by the utility window.
type NoteEditorProps struct {
	ID                   string
	Document             common.NoteDocument
	Width                float32
	Height               float32
	Padding              woxwidget.Insets
	Style                woxui.TextStyle
	LineHeight           float32
	Zoom                 float32
	TextColor            woxui.Color
	Theme                Theme
	Window               *woxui.Window
	ReadOnly             bool
	Autofocus            bool
	Controller           *woxwidget.TextEditingController
	FocusNode            *woxwidget.FocusNode
	Focused              bool
	Selection            woxui.TextSelection
	Label                string
	OnChanged            func(segmentStart int, value string)
	OnSelectionChanged   func(woxui.TextSelection)
	OnTapOffset          func(int) bool
	CursorAtOffset       func(int) woxui.PointerCursor
	OnKey                func(woxui.KeyEvent) bool
	OnUndo               func() bool
	OnRedo               func() bool
	OnPaste              func(string) bool
	TransformPaste       func(string) string
	OnTableChange        func(block int, table common.NoteTable)
	OnTableFocus         func(block, row, column int)
	OnTableKey           func(block, row, column int, event woxui.KeyEvent) bool
	OnTablePaste         func(block, row, column int, value string) bool
	OnTableInsertRow     func(block int)
	OnTableInsertColumn  func(block int)
	OnTableDeleteRow     func(block int)
	OnTableDeleteColumn  func(block int)
	OnTableDelete        func(block int)
	OnTableActionHover   func(inside bool, label string, bounds woxui.Rect)
	OnDeleteEmptySegment func(segmentStart int) bool
	TableActionLabels    NoteTableActionLabels
	FocusedTableBlock    int
	FocusedTableRow      int
	FocusedTableCol      int
}

// WoxNoteEditor renders linear Notes text segments and structural table blocks.
func WoxNoteEditor(props NoteEditorProps) woxwidget.Widget {
	if props.FocusedTableBlock < 0 {
		props.FocusedTableBlock = -1
	}
	segments := noteDocumentSegments(props.Document)
	if len(segments) == 1 && !segments[0].Table {
		return noteEditorTextField(props, segments[0], props.ID, props.Height, true, props.Padding)
	}
	children := make([]woxwidget.Widget, 0, len(segments))
	textIndex := 0
	contentHeight := float32(0)
	for _, segment := range segments {
		if len(children) > 0 {
			contentHeight += noteEditorSegmentGap
		}
		if segment.Table {
			block := props.Document.Blocks[segment.Start]
			table := common.NoteTable{}
			if block.Table != nil {
				table = *block.Table
			}
			height := noteTableBlockHeight(table, props.Zoom, !props.ReadOnly && props.OnTableInsertRow != nil)
			padding := noteEditorEdgePadding(props.Padding, len(children) == 0)
			children = append(children, woxwidget.Container{Width: props.Width, Padding: padding, Child: WoxNoteTable(NoteTableProps{
				ID: fmt.Sprintf("%s.table.%s", props.ID, block.ID), Block: segment.Start, Table: table,
				Width: max(float32(0), props.Width-props.Padding.Left-props.Padding.Right), ReadOnly: props.ReadOnly,
				Theme: props.Theme, Window: props.Window, Zoom: props.Zoom, Style: props.Style,
				Focused: props.FocusedTableBlock == segment.Start, FocusRow: props.FocusedTableRow, FocusCol: props.FocusedTableCol,
				OnChange: func(updated common.NoteTable) {
					if props.OnTableChange != nil {
						props.OnTableChange(segment.Start, updated)
					}
				},
				OnFocus: func(row, column int) {
					if props.OnTableFocus != nil {
						props.OnTableFocus(segment.Start, row, column)
					}
				},
				OnKey: func(row, column int, event woxui.KeyEvent) bool {
					if props.OnTableKey != nil {
						return props.OnTableKey(segment.Start, row, column, event)
					}
					return false
				},
				OnPaste: func(row, column int, value string) bool {
					if props.OnTablePaste != nil {
						return props.OnTablePaste(segment.Start, row, column, value)
					}
					return false
				},
				OnUndo: props.OnUndo, OnRedo: props.OnRedo,
				Actions: noteEditorTableActions(props, segment.Start),
			})})
			contentHeight += padding.Top + height
			continue
		}
		fieldID := props.ID
		if textIndex > 0 {
			fieldID = fmt.Sprintf("%s.%d", props.ID, textIndex)
		}
		projected, _, _ := ProjectNoteDocument(noteSegmentDocument(props.Document, segment), props.Style, props.Theme)
		lines := 1
		if projected != "" {
			lines = 1 + countNoteNewlines(projected)
		}
		padding := noteEditorEdgePadding(props.Padding, len(children) == 0)
		height := max(props.LineHeight, float32(lines)*props.LineHeight+padding.Top+padding.Bottom)
		if textIndex == 0 && segment.End == len(props.Document.Blocks) {
			height = max(height, props.Height-contentHeight)
		}
		children = append(children, noteEditorTextField(props, segment, fieldID, height, textIndex == 0, padding))
		contentHeight += height
		textIndex++
	}
	// Omit ContentHeight so the shared scroller measures the document and can show a vertical thumb.
	var content woxwidget.Widget = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: noteEditorSegmentGap, Children: children}
	if props.Padding.Bottom > 0 {
		// Tables sit outside the text field, so the document scroller must own the trailing inset.
		content = woxwidget.Container{Width: props.Width, Padding: woxwidget.Insets{Bottom: props.Padding.Bottom}, Child: content}
	}
	return WoxScrollView(ScrollViewProps{
		Key: "notes.editor.scroll", AutomationID: "notes.editor.scroll", Label: props.Label,
		Width: props.Width, Height: props.Height, Content: content,
		ThumbColor: props.Theme.ResultSubtitle,
	})
}

func noteEditorTextField(props NoteEditorProps, segment NoteDocumentSegment, id string, height float32, primary bool, padding woxwidget.Insets) woxwidget.Widget {
	value, runs, _ := ProjectNoteSegment(props.Document, segment, props.Style, props.Theme)
	controller := (*woxwidget.TextEditingController)(nil)
	focus := (*woxwidget.FocusNode)(nil)
	focused := false
	autofocus := false
	if primary {
		controller = props.Controller
		focus = props.FocusNode
		focused = props.Focused
		autofocus = props.Autofocus
		if controller != nil {
			value = controller.Text()
		}
	}
	start := segment.Start
	empty := strings.TrimSpace(value) == ""
	return WoxTextField(TextFieldProps{
		ID: id, Label: props.Label, Width: props.Width, Height: height, Padding: padding,
		Transparent: true, DisableHover: true, Style: props.Style, RichRuns: NoteFieldRuns(runs),
		LineHeight: props.LineHeight, TextAlignmentY: 0.5, TextColor: props.TextColor, Value: value,
		Controller: controller, FocusNode: focus, Focused: focused, Autofocus: autofocus,
		ReadOnly: props.ReadOnly, MaxLines: 10000, Window: props.Window, Theme: props.Theme,
		OnChanged: func(text string) {
			if props.OnChanged != nil {
				props.OnChanged(start, text)
			}
		},
		OnSelectionChanged: props.OnSelectionChanged,
		OnTapOffset:        props.OnTapOffset,
		CursorAtOffset:     props.CursorAtOffset,
		OnKey: func(event woxui.KeyEvent) bool {
			if noteEditorEmptyBoundaryKey(event) && empty && props.OnDeleteEmptySegment != nil && props.OnDeleteEmptySegment(start) {
				return true
			}
			if props.OnKey != nil {
				return props.OnKey(event)
			}
			return false
		},
		OnUndo:         props.OnUndo,
		OnRedo:         props.OnRedo,
		OnPaste:        props.OnPaste,
		TransformPaste: props.TransformPaste,
	})
}

func noteEditorTableActions(props NoteEditorProps, block int) NoteTableActions {
	if props.ReadOnly || props.OnTableInsertRow == nil || props.OnTableInsertColumn == nil ||
		props.OnTableDeleteRow == nil || props.OnTableDeleteColumn == nil || props.OnTableDelete == nil {
		return NoteTableActions{}
	}
	return NoteTableActions{
		Labels:         props.TableActionLabels,
		OnInsertRow:    func() { props.OnTableInsertRow(block) },
		OnInsertColumn: func() { props.OnTableInsertColumn(block) },
		OnDeleteRow:    func() { props.OnTableDeleteRow(block) },
		OnDeleteColumn: func() { props.OnTableDeleteColumn(block) },
		OnDeleteTable:  func() { props.OnTableDelete(block) },
		OnHover:        props.OnTableActionHover,
	}
}

// noteEditorEdgePadding keeps the document top inset on the first segment only.
// The scroller owns the trailing inset so text-above-table does not inherit Bottom: 24.
func noteEditorEdgePadding(padding woxwidget.Insets, first bool) woxwidget.Insets {
	edge := woxwidget.Insets{Left: padding.Left, Right: padding.Right}
	if first {
		edge.Top = padding.Top
	}
	return edge
}

func noteEditorEmptyBoundaryKey(event woxui.KeyEvent) bool {
	return event.Down && !event.Composing && event.Modifiers == 0 && (event.Key == woxui.KeyBackspace || event.Key == woxui.KeyDelete)
}

func countNoteNewlines(value string) int {
	count := 0
	for _, letter := range value {
		if letter == '\n' {
			count++
		}
	}
	return count
}
