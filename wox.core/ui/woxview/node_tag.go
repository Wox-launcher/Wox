package woxview

import (
	component "wox/ui/launcher/component"
	widget "wox/ui/widget"
)

// TagProps supplies translated metadata text; badge uses the shared compact tag tier.
type TagProps struct {
	Styled
	Text string `json:"text"`
}

// buildTag keeps status appearance in the shared Wox tag controls.
func buildTag(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*TagProps)
	text, theme := b.translate(p.Text), b.theme()
	color := p.Style.Color.resolve(theme, theme.TextSecondary)
	content := component.WoxTagWithPaint(text, color, theme, node.Type == NodeBadge, p.Style.controlPaint(theme))
	return p.Style.decorateControl(theme, widget.Semantics{AutomationID: id, Label: text, Child: content})
}
