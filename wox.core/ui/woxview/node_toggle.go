package woxview

import (
	component "wox/ui/launcher/component"
	widget "wox/ui/widget"
)

// ToggleProps is a controlled boolean for switch and checkbox. Label is the accessible name;
// compose a row with a text node for a visible label.
type ToggleProps struct {
	Styled
	FieldProps
	Name     string   `json:"name,omitempty"`
	Value    bool     `json:"value,omitempty"`
	Label    string   `json:"label,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	OnChange *Handler `json:"onChange,omitempty"`
}

func (p *ToggleProps) fieldName() string { return p.Name }
func (p *ToggleProps) initialValue() any { return p.Value }
func (p *ToggleProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onChange", p.OnChange}}...)
}

func (p *ToggleProps) prepare() error {
	return p.Style.validate()
}

func buildSwitch(b *builder, node *Node, id string) widget.Widget {
	return buildToggle(b, node, id)
}

func buildCheckbox(b *builder, node *Node, id string) widget.Widget {
	return buildToggle(b, node, id)
}

type toggleConfig struct {
	props *ToggleProps
	id    string
	kind  NodeType
	theme component.ControlTheme
	view  *View
	emit  func(...any)
}

type toggleState struct{ echo *echoState[bool] }

func buildToggle(b *builder, node *Node, id string) widget.Widget {
	p := *node.Props.(*ToggleProps)
	p.Disabled = p.Disabled || b.fieldsDisabled()
	p.Label = b.translate(p.Label)
	p.Error = b.translate(p.Error)
	props := &p
	c := toggleConfig{props: props, id: id, kind: node.Type, theme: b.theme(), view: b.view, emit: b.emitChange(props.OnChange)}
	return props.Style.wrapControl(b.theme(), props.decorateField(props.Label, id, b.theme(), widget.Stateful{Key: widget.Key(id), Type: (*toggleState)(nil), Widget: c, CreateState: func() widget.State { return &toggleState{} }}))
}

func (s *toggleState) InitState(_ widget.StateContext, value any) {
	c := value.(toggleConfig)
	s.echo = retainEcho(c.view, c.id, c.props.Value, c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.value
}
func (s *toggleState) DidUpdateWidget(_ widget.StateContext, _, value any) {
	c := value.(toggleConfig)
	s.echo.receive(c.props.Value, c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.value
}
func (s *toggleState) Dispose() {}
func (s *toggleState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(toggleConfig)
	change := func(next bool) {
		ctx.SetState(func() {
			s.echo.local(next)
			c.view.fieldValues[c.id] = s.echo.value
		})
		if c.emit != nil {
			c.emit(next)
		}
	}
	if c.kind == NodeSwitch {
		return component.WoxSwitch(component.SwitchProps{ID: c.id, Label: c.props.fieldLabel(c.props.Label), Value: s.echo.value, Disabled: c.props.Disabled, OnChange: change, Theme: c.theme, Paint: c.props.Style.controlPaint(c.theme)})
	}
	return component.WoxCheckbox(component.CheckboxProps{ID: c.id, Label: c.props.fieldLabel(c.props.Label), Value: s.echo.value, Disabled: c.props.Disabled, OnChange: change, Theme: c.theme, Paint: c.props.Style.controlPaint(c.theme)})
}

func (p *ToggleProps) isDisabled() bool { return p.Disabled }
