package view

import (
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// formTableSearchState follows the mounted table, so unrelated plugin tables
// cannot inherit its query or editor focus.
type formTableSearchState struct {
	open    bool
	query   string
	focused bool
}

// FormTableField mounts local search state only for searchable tables.
func FormTableField(props FormTableFieldProps) woxwidget.Widget {
	if !props.EnableSearch {
		return formTableField(props)
	}
	key := props.StateKey
	if key == "" {
		key = props.ID
	}
	return woxwidget.Stateful{Key: woxwidget.Key(key), Type: (*formTableSearchState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &formTableSearchState{} },
	}
}

func (s *formTableSearchState) InitState(_ woxwidget.StateContext, _ any)          {}
func (s *formTableSearchState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}
func (s *formTableSearchState) Dispose()                                           {}

// Build filters copies of props; editor state and source rows remain independent.
func (s *formTableSearchState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(FormTableFieldProps)
	props.SearchOpen, props.SearchQuery, props.SearchFocused = s.open, s.query, s.focused
	props.OnToggleSearch = func() { context.SetState(func() { s.open = !s.open; s.focused = s.open }) }
	props.OnSearchChanged = func(value string) { context.SetState(func() { s.query = value }) }
	props.OnSearchSetValue = func(value string) error { props.OnSearchChanged(value); return nil }
	props.OnSearchClear = func() { props.OnSearchChanged("") }
	props.OnSearchFocusChange = func(focused bool) {
		if s.focused != focused {
			context.SetState(func() { s.focused = focused })
		}
	}
	props.OnSearchKey = func(event woxui.KeyEvent) bool {
		if event.Key != woxui.KeyEscape {
			return false
		}
		if event.Down {
			props.OnToggleSearch()
		}
		return true
	}
	if s.open {
		props = filterFormTableRows(props, s.query)
	}
	return formTableField(props)
}

// filterFormTableRows pins the requested column without mutating incoming props.
func filterFormTableRows(props FormTableFieldProps, query string) FormTableFieldProps {
	columnIndex := -1
	for index, column := range props.Columns {
		if column.Key == props.SearchColumnKey && props.SearchColumnKey != "" {
			columnIndex = index
			break
		}
	}
	rows := make([]FormTableRow, 0, len(props.Rows))
	for _, row := range props.Rows {
		texts := make([]string, 0, len(row.Cells))
		for index, cell := range row.Cells {
			if columnIndex >= 0 && index != columnIndex {
				continue
			}
			text := cell.SearchText
			if text == "" {
				text = cell.Text
			}
			texts = append(texts, text)
		}
		if !formTableRowMatchesSearch(texts, query, props.UsePinYin) {
			continue
		}
		if columnIndex > 0 && columnIndex < len(row.Cells) {
			cells := append([]FormTableCell(nil), row.Cells...)
			copy(cells[1:columnIndex+1], row.Cells[:columnIndex])
			cells[0] = row.Cells[columnIndex]
			row.Cells = cells
		}
		rows = append(rows, row)
	}
	if columnIndex > 0 {
		columns := append([]FormTableColumn(nil), props.Columns...)
		copy(columns[1:columnIndex+1], props.Columns[:columnIndex])
		columns[0] = props.Columns[columnIndex]
		props.Columns = columns
	}
	props.Rows = rows
	if strings.TrimSpace(query) != "" {
		props.EmptyLabel = props.NoMatchesLabel
	}
	return props
}

// formTableRowMatchesSearch shares fuzzy and optional pinyin matching with Settings.
func formTableRowMatchesSearch(texts []string, query string, usePinYin bool) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	for _, text := range texts {
		if strings.TrimSpace(text) != "" && util.IsStringMatch(text, query, usePinYin) {
			return true
		}
	}
	return false
}
