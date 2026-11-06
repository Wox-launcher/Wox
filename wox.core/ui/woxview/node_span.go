package woxview

import (
	component "wox/ui/launcher/component"
	"wox/ui/widget"
)

// SpanStyle deliberately excludes layout properties. Optional booleans can turn
// off traits inherited from the parent text without changing its line geometry.
type SpanStyle struct {
	Color      Color      `json:"color,omitempty"`
	FontWeight FontWeight `json:"fontWeight,omitempty"`
	FontFamily FontFamily `json:"fontFamily,omitempty"`
	Italic     *bool      `json:"italic,omitempty"`
	Underline  *bool      `json:"underline,omitempty"`
}

type SpanProps struct {
	CommonProps
	InteractionStyles
	Text    string    `json:"text"`
	Style   SpanStyle `json:"style,omitempty"`
	OnPress *Handler  `json:"onPress,omitempty"`
}

func (p *SpanProps) handlers() []namedHandler {
	return append(p.CommonProps.handlers(), namedHandler{"onPress", p.OnPress})
}

func (p *SpanProps) prepare() error {
	return (Style{Color: p.Style.Color, FontWeight: p.Style.FontWeight, FontFamily: p.Style.FontFamily}).validate()
}

// richTextRuns concatenates translated display strings while keeping event keys
// and callback identities attached to each original span, including after reorder.
func (b *builder) richTextRuns(node *Node, id string) []component.RichTextRun {
	p := node.Props.(*TextProps)
	base := p.Style
	result := []component.RichTextRun{{ID: id + "/text", Text: b.translate(p.Text), Style: base.textStyle(component.ControlTheme{}, 14), Color: base.Color.resolve(b.theme(), b.theme().Text)}}
	for i, child := range node.Children {
		span := child.Props.(*SpanProps)
		style := base
		if span.Style.Color != (Color{}) {
			style.Color = span.Style.Color
		}
		if span.Style.FontWeight != "" {
			style.FontWeight = span.Style.FontWeight
		}
		if span.Style.FontFamily != "" {
			style.FontFamily = span.Style.FontFamily
		}
		if span.Style.Italic != nil {
			style.Italic = *span.Style.Italic
		}
		underline := span.OnPress != nil
		if span.Style.Underline != nil {
			underline = *span.Style.Underline
		}
		childID := nodeID(id, child, i)
		local := *b
		local.nodeKey = child.Key
		local.nodeOwner = b.view.nodes[childID]
		local.eventOwner = local.nodeOwner
		color := style.Color.resolve(b.theme(), b.theme().Text)
		if span.OnPress != nil && span.Style.Color == (Color{}) && base.Color == (Color{}) {
			color = b.theme().Accent
		}
		result = append(result, component.RichTextRun{
			ID: childID, Text: b.translate(span.Text), Style: style.textStyle(component.ControlTheme{}, 14), Color: color,
			Underline: underline, OnPress: local.tap(span.OnPress),
			Decorate: func(content widget.Widget) widget.Widget { return local.decorateNode(child, childID, content) },
		})
	}
	return result
}
