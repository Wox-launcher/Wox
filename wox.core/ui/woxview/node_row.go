package woxview

import widget "wox/ui/widget"

// RowProps optionally wraps natural-size children into multiple horizontal runs.
type RowProps struct {
	BoxProps
	Wrap bool `json:"wrap,omitempty"`
}

// buildRow keeps ordinary flex behavior unless wrapping was explicitly requested.
func buildRow(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*RowProps)
	if !p.Wrap {
		return buildFlexBox(b, node, id, &p.BoxProps, widget.Horizontal)
	}
	content := widget.Wrap{Gap: p.Style.Gap.resolve(b.theme()), RunGap: p.Style.Gap.resolve(b.theme()), Children: b.children(node, id)}
	return pressable(b, id, &p.BoxProps, p.Style.decorate(b.theme(), content))
}
