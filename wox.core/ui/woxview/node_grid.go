package woxview

import (
	"errors"

	widget "wox/ui/widget"
)

// GridProps arranges children in equal-width columns with content-sized rows.
type GridProps struct {
	BoxProps
	Columns int  `json:"columns"`
	Gap     Size `json:"gap,omitempty"`
}

// prepare bounds the authored grid dimensions before any layout calculations.
func (p *GridProps) prepare() error {
	if p.Columns < 1 || p.Columns > MaxNodes {
		return errors.New("columns must be between 1 and 5000")
	}
	if p.Gap.validate() != nil {
		return errors.New("gap must be between 0 and 16384")
	}
	return p.Style.validate()
}

// buildGrid delegates measurement and cell alignment to the shared grid primitive.
func buildGrid(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*GridProps)
	content := widget.Grid{Columns: p.Columns, ColumnGap: p.Gap.resolve(b.theme()), RowGap: p.Gap.resolve(b.theme()), CrossAxisAlignment: p.Style.crossAxis(), Children: b.children(node, id)}
	return pressable(b, id, &p.BoxProps, p.Style.decorate(b.theme(), content))
}
