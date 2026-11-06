package woxview

import (
	"encoding/base64"
	"fmt"
	"slices"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// MultiSelectProps renders wrapping choice tags and reports an ordered string array.
type MultiSelectProps struct {
	Styled
	FieldProps
	Name     string         `json:"name,omitempty"`
	Label    string         `json:"label,omitempty"`
	Value    []string       `json:"value,omitempty"`
	Options  []ChoiceOption `json:"options"`
	Disabled bool           `json:"disabled,omitempty"`
	OnChange *Handler       `json:"onChange,omitempty"`
}

func (p *MultiSelectProps) fieldName() string { return p.Name }
func (p *MultiSelectProps) initialValue() any { return append([]string{}, p.Value...) }
func (p *MultiSelectProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), namedHandler{"onChange", p.OnChange})
}

// prepare rejects ambiguous selections while sharing single-choice option validation.
func (p *MultiSelectProps) prepare() error {
	choice := ChoiceProps{Styled: p.Styled, Options: p.Options}
	if err := choice.prepare(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, value := range p.Value {
		if seen[value] || !slices.ContainsFunc(p.Options, func(o ChoiceOption) bool { return o.Value == value }) {
			return fmt.Errorf("selected values must be unique options: %q", value)
		}
		seen[value] = true
	}
	return nil
}

// normalizeFieldValue removes unavailable choices without discarding the other local selections.
func (p *MultiSelectProps) normalizeFieldValue(value any) any {
	selected := []string{}
	for _, v := range value.([]string) {
		if slices.ContainsFunc(p.Options, func(o ChoiceOption) bool { return o.Value == v }) {
			selected = append(selected, v)
		}
	}
	return selected
}

type multiSelectConfig struct {
	props MultiSelectProps
	id    string
	theme component.ControlTheme
	view  *View
	emit  func(...any)
}

type multiSelectState struct{ echo *echoState[[]string] }

// buildMultiSelect copies presentation props so locale updates never mutate the immutable tree.
func buildMultiSelect(b *builder, node *Node, id string) widget.Widget {
	p := *node.Props.(*MultiSelectProps)
	p.Disabled = p.Disabled || b.fieldsDisabled()
	p.Label, p.Error = b.translate(p.Label), b.translate(p.Error)
	p.Options = slices.Clone(p.Options)
	for i := range p.Options {
		p.Options[i].Label = b.translate(p.Options[i].Label)
	}
	c := multiSelectConfig{props: p, id: id, theme: b.theme(), view: b.view, emit: b.emitChange(p.OnChange)}
	return p.Style.wrapOuter(b.theme(), p.decorateField(p.Label, id, b.theme(), widget.Stateful{
		Key: widget.Key(id), Type: (*multiSelectState)(nil), Widget: c, CreateState: func() widget.State { return &multiSelectState{} },
	}))
}

func (s *multiSelectState) InitState(_ widget.StateContext, value any) {
	c := value.(multiSelectConfig)
	s.echo = retainEcho(c.view, c.id, c.props.initialValue().([]string), c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.receiveField(&c.props)
}

func (s *multiSelectState) DidUpdateWidget(_ widget.StateContext, _, value any) {
	c := value.(multiSelectConfig)
	c.view.fieldValues[c.id] = s.echo.receiveField(&c.props)
}

func (*multiSelectState) Dispose() {}

// Build always creates a fresh selection snapshot; pending echoes must never share a mutable slice.
func (s *multiSelectState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(multiSelectConfig)
	items := make([]component.MenuItem, 0, len(c.props.Options))
	for _, option := range c.props.Options {
		items = append(items, component.MenuItem{
			ID: c.id + "/option:" + base64.RawURLEncoding.EncodeToString([]byte(option.Value)), Label: option.Label,
			Selected: slices.Contains(s.echo.value, option.Value), Disabled: c.props.Disabled || option.Disabled,
			OnSelect: func() {
				if c.props.Disabled || option.Disabled {
					return
				}
				next := append([]string{}, s.echo.value...)
				if index := slices.Index(next, option.Value); index >= 0 {
					next = slices.Delete(next, index, index+1)
				} else {
					next = append(next, option.Value)
				}
				ctx.SetState(func() { s.echo.local(next); c.view.fieldValues[c.id] = next })
				if c.emit != nil {
					c.emit(next)
				}
			},
		})
	}
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		return widget.Container{Width: widthOr(c.props.Style.resolve(c.theme), available), Child: component.WoxChoiceGroup(component.ChoiceGroupProps{
			ID: c.id + "/group", Label: c.props.fieldLabel(c.props.Label), Items: items, Chips: true, Theme: c.theme, Paint: c.props.Style.controlPaint(c.theme),
		})}
	}}
}

func (p *MultiSelectProps) isDisabled() bool { return p.Disabled }
