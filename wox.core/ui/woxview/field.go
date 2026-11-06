package woxview

import (
	"reflect"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// FieldProps supplies presentation and explicit reset semantics for every form field.
type FieldProps struct {
	Revision uint64 `json:"revision,omitempty"`
	Error    string `json:"error,omitempty"`
	Required bool   `json:"required,omitempty"`
}

func (p FieldProps) fieldRevision() uint64 { return p.Revision }

// retainedField reconciles producer updates even when virtualization unmounts its widget.
type retainedField interface {
	receiveField(fieldOwner) any
}

// fieldNormalizer keeps a retained edit valid when its available choices or range changes.
type fieldNormalizer interface {
	normalizeFieldValue(any) any
}

func (s *echoState[T]) receiveField(p fieldOwner) any {
	s.receive(p.initialValue().(T), p.fieldRevision())
	if normalizer, ok := p.(fieldNormalizer); ok {
		value := normalizer.normalizeFieldValue(s.value).(T)
		if !reflect.DeepEqual(value, s.value) {
			// A removed option changes the local value, not the producer's last prop.
			// Retaining lastProp prevents an unchanged uncontrolled prop from undoing normalization.
			s.value, s.pending = value, nil
		}
	}
	return s.value
}

// retainEcho separates the field's document lifetime from its visible widget lifetime.
func retainEcho[T any](v *View, id string, value T, revision uint64) *echoState[T] {
	if state, ok := v.fieldStates[id].(*echoState[T]); ok {
		state.receive(value, revision)
		return state
	}
	state := &echoState[T]{}
	state.init(value, revision)
	v.fieldStates[id] = state
	return state
}

func (p FieldProps) fieldLabel(label string) string {
	if p.Required {
		return label + " *"
	}
	return label
}

// decorateField keeps validation feedback outside the control's own fixed-height chrome.
// Required is descriptive only; the producer remains responsible for validation.
func (p FieldProps) decorateField(label, id string, theme component.ControlTheme, child widget.Widget) widget.Widget {
	if !p.Required && p.Error == "" {
		return child
	}
	children := []widget.Widget{}
	if p.Required {
		children = append(children, widget.Text{Value: p.fieldLabel(label), Style: woxui.TextStyle{Size: component.SettingsControlFontSize}, Color: theme.Text})
	}
	children = append(children, child)
	if p.Error != "" {
		children = append(children, widget.Semantics{AutomationID: id + "/error", Label: p.Error, Child: widget.Text{Value: p.Error, Style: woxui.TextStyle{Size: component.SettingsControlFontSize}, Color: theme.Error}})
	}
	return widget.Flex{Axis: widget.Vertical, Gap: 4, CrossAxisAlignment: widget.CrossAxisStart, Children: children}
}

func (b *builder) fieldsDisabled() bool { return b.form != nil && b.form.disabled }
