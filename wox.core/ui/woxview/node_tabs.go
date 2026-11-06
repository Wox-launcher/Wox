package woxview

import widget "wox/ui/widget"

// TabsProps declares a tab strip; the producer renders the selected tab's content.
type TabsProps struct {
	Styled
	FieldProps
	Name     string         `json:"name,omitempty"`
	Label    string         `json:"label,omitempty"`
	Value    string         `json:"value,omitempty"`
	Items    []ChoiceOption `json:"items"`
	Disabled bool           `json:"disabled,omitempty"`
	OnChange *Handler       `json:"onChange,omitempty"`
}

func (p *TabsProps) fieldName() string { return p.Name }
func (p *TabsProps) initialValue() any { return p.Value }
func (p *TabsProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), namedHandler{"onChange", p.OnChange})
}

func (p *TabsProps) choice() ChoiceProps {
	return ChoiceProps{Styled: p.Styled, FieldProps: p.FieldProps, Name: p.Name, Label: p.Label, Value: p.Value, Options: p.Items, Disabled: p.Disabled, OnChange: p.OnChange}
}

func (p *TabsProps) prepare() error {
	choice := p.choice()
	return choice.prepare()
}

func (p *TabsProps) normalizeFieldValue(value any) any {
	choice := p.choice()
	return choice.normalizeFieldValue(value)
}

func buildTabs(b *builder, node *Node, id string) widget.Widget {
	return buildChoiceProps(b, node.Props.(*TabsProps).choice(), NodeTabs, id)
}

func (p *TabsProps) isDisabled() bool { return p.Disabled }
