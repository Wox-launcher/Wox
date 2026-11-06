package woxview

import (
	"encoding/base64"
	"fmt"
	"strings"

	component "wox/ui/launcher/component"
	widget "wox/ui/widget"
)

type TableColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Width Size   `json:"width,omitempty"`
}

// TableAction identifies one row operation; the handler receives the row key.
type TableAction struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Icon     string   `json:"icon,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	OnPress  *Handler `json:"onPress"`
}

type TableRow struct {
	Key     string        `json:"key"`
	Cells   []string      `json:"cells"`
	Actions []TableAction `json:"actions,omitempty"`
}

// TableProps renders bounded text rows with stable row/column identities and local actions.
type TableProps struct {
	Styled
	Label    string        `json:"label,omitempty"`
	Columns  []TableColumn `json:"columns"`
	Rows     []TableRow    `json:"rows"`
	Disabled bool          `json:"disabled,omitempty"`
}

func (p *TableProps) handlers() []namedHandler {
	handlers := p.Styled.handlers()
	for i, row := range p.Rows {
		for j, action := range row.Actions {
			handlers = append(handlers, namedHandler{fmt.Sprintf("rows[%d].actions[%d].onPress", i, j), action.OnPress})
		}
	}
	return handlers
}

// prepare bounds expanded cells as well as rows, since table data is not counted as tree nodes.
func (p *TableProps) prepare() error {
	if len(p.Columns) == 0 || len(p.Columns) > 64 || len(p.Rows) > 1000 || len(p.Rows)*len(p.Columns) > 10000 {
		return fmt.Errorf("table requires 1-64 columns, at most 1000 rows and 10000 cells")
	}
	columns := map[string]bool{}
	for _, column := range p.Columns {
		if column.Key == "" || len(column.Key) > 256 || columns[column.Key] {
			return fmt.Errorf("column keys must be nonempty, unique and at most 256 bytes")
		}
		columns[column.Key] = true
		if err := (Style{Width: column.Width}).validate(); err != nil {
			return err
		}
	}
	rows := map[string]bool{}
	for i, row := range p.Rows {
		if row.Key == "" || len(row.Key) > 256 || rows[row.Key] || len(row.Cells) != len(p.Columns) || len(row.Actions) > 32 {
			return fmt.Errorf("rows[%d]: require a unique nonempty key (up to 256 bytes), one cell per column and at most 32 actions", i)
		}
		rows[row.Key] = true
		actions := map[string]bool{}
		for _, action := range row.Actions {
			if action.Key == "" || len(action.Key) > 256 || actions[action.Key] || strings.TrimSpace(action.Label) == "" || action.OnPress == nil {
				return fmt.Errorf("rows[%d]: actions require unique nonempty keys, labels and handlers", i)
			}
			actions[action.Key] = true
			if err := validateIcon(action.Icon); err != nil {
				return err
			}
		}
	}
	return p.Style.validate()
}

// buildTable translates display text and binds actions to row keys without retaining mutable data.
func buildTable(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*TableProps)
	props := component.DataTableProps{ID: id, Label: b.translate(p.Label), Height: p.Style.Height.resolve(b.theme()), Theme: b.theme(), Paint: p.Style.controlPaint(b.theme())}
	for _, c := range p.Columns {
		props.Columns = append(props.Columns, component.DataTableColumn{ID: base64.RawURLEncoding.EncodeToString([]byte(c.Key)), Label: b.translate(c.Label), Width: c.Width.resolve(b.theme())})
	}
	for _, row := range p.Rows {
		r := component.DataTableRow{ID: id + "/row:" + base64.RawURLEncoding.EncodeToString([]byte(row.Key))}
		for _, text := range row.Cells {
			r.Cells = append(r.Cells, b.translate(text))
		}
		for _, action := range row.Actions {
			emit := b.emit(action.OnPress)
			r.Actions = append(r.Actions, component.MenuItem{
				ID: r.ID + "/action:" + base64.RawURLEncoding.EncodeToString([]byte(action.Key)), Label: b.translate(action.Label), Icon: action.Icon, Disabled: p.Disabled || action.Disabled,
				OnSelect: func() {
					if emit != nil {
						emit(row.Key)
					}
				},
			})
		}
		props.Rows = append(props.Rows, r)
	}
	return p.Style.decorateControl(b.theme(), component.WoxDataTable(props))
}

func (p *TableProps) isDisabled() bool { return p.Disabled }
