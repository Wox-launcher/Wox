package woxview

import (
	component "wox/ui/launcher/component"
	widget "wox/ui/widget"
)

// CodeProps displays and copies literal source; language is a presentation label, not executable code.
type CodeProps struct {
	Styled
	Code     string `json:"code"`
	Language string `json:"language,omitempty"`
}

func buildCode(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*CodeProps)
	return p.Style.decorateControl(b.theme(), component.WoxCodeBlock(component.CodeBlockProps{
		ID: id, Code: p.Code, Language: b.translate(p.Language), FontSize: p.Style.FontSize.base(), Theme: b.theme(), Window: b.props.Window, Paint: p.Style.controlPaint(b.theme()),
	}))
}
