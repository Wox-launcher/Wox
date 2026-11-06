package woxview

import (
	"errors"
	"fmt"

	component "wox/ui/launcher/component"
	widget "wox/ui/widget"
)

// FormProps groups named fields and reports their current values on submit.
type FormProps struct {
	Styled
	Disabled   bool     `json:"disabled,omitempty"`
	Submitting bool     `json:"submitting,omitempty"`
	OnSubmit   *Handler `json:"onSubmit,omitempty"`
}

func (p *FormProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onSubmit", p.OnSubmit}}...)
}

func (p *FormProps) prepare() error {
	return p.Style.validate()
}

type formContext struct {
	node       *Node
	id         string
	handler    *Handler
	disabled   bool
	submitting bool
}

// validateForm rejects ambiguous snapshots and nested form ownership.
func validateForm(form *Node) error {
	names := map[string]bool{}
	var walk func(*Node) error
	walk = func(n *Node) error {
		if n != form && n.Type == NodeForm {
			return errors.New("nested forms are not supported")
		}
		if field, ok := n.Props.(fieldOwner); ok && field.fieldName() != "" {
			name := field.fieldName()
			if names[name] {
				return fmt.Errorf("duplicate field name %q", name)
			}
			names[name] = true
		}
		if n.Type == NodeSubmit && n.Props.(*ButtonProps).OnPress != nil {
			return errors.New("submit uses form onSubmit, not onPress")
		}
		for _, child := range n.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(form)
}

func buildForm(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*FormProps)
	childBuilder := *b
	childBuilder.form = &formContext{node: node, id: id, handler: p.OnSubmit, disabled: p.Disabled, submitting: p.Submitting}
	return p.Style.decorate(b.theme(), widget.Flex{Axis: widget.Vertical, Gap: p.Style.Gap.resolve(b.theme()), MainAxisAlignment: p.Style.mainAxis(), CrossAxisAlignment: p.Style.crossAxis(), Children: childBuilder.flexChildren(node, id)})
}

// formValues walks the current tree so a removed field can never be submitted.
func (v *View) formValues(form *Node, id string) map[string]any {
	values := map[string]any{}
	var walk func(*Node, string)
	walk = func(n *Node, id string) {
		if field, ok := n.Props.(fieldOwner); ok && field.fieldName() != "" {
			value, found := v.fieldValues[id]
			if !found {
				value = field.initialValue()
			}
			values[field.fieldName()] = value
		}
		for i, child := range n.Children {
			walk(child, nodeID(id, child, i))
		}
	}
	walk(form, id)
	return values
}

// formSubmit captures this tree's form and prevents duplicate submission while busy.
func (b *builder) formSubmit() func() {
	if b.form == nil || b.form.disabled || b.form.submitting {
		return nil
	}
	local := *b
	local.eventOwner = b.view.nodes[b.form.id]
	emit := local.emit(b.form.handler)
	if emit == nil {
		return nil
	}
	form, view := b.form, b.view
	source, epoch := b.nodeOwner, view.eventEpoch
	return func() {
		owner := local.eventOwner
		if source == nil || !source.live || owner == nil || !owner.live || view.eventEpoch != epoch || !owner.handlers[form.handler.ID] {
			return
		}
		props := owner.node.Props.(*FormProps)
		if props.Disabled || props.Submitting {
			return
		}
		view.flushFormChanges(owner.node, form.id)
		emit(view.formValues(owner.node, form.id))
	}
}

// buildSubmit reuses button styling and exposes busy feedback without another producer handler.
func buildSubmit(b *builder, node *Node, id string) widget.Widget {
	p := *node.Props.(*ButtonProps)
	submit := b.formSubmit()
	p.Disabled = p.Disabled || submit == nil
	// Pass the action directly instead of intercepting only the legacy OnEvent callback.
	// Embedders using OnNodeEvent need the same form snapshot and triggering node key.
	built := buildButtonControl(b, &p, id, submit)
	if b.form != nil && b.form.submitting {
		return widget.Flex{Axis: widget.Horizontal, Gap: 8, CrossAxisAlignment: widget.CrossAxisCenter, Children: []widget.Widget{built, component.WoxLoadingIndicator(16, b.theme().Text)}}
	}
	return built
}

func (p *FormProps) isDisabled() bool { return p.Disabled }
