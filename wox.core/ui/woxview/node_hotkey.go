package woxview

import (
	"fmt"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// HotkeyProps records a normal local key combination without registering a global shortcut.
type HotkeyProps struct {
	Styled
	FieldProps
	Name        string   `json:"name,omitempty"`
	Label       string   `json:"label,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Value       string   `json:"value,omitempty"`
	Disabled    bool     `json:"disabled,omitempty"`
	OnChange    *Handler `json:"onChange,omitempty"`
}

func (p *HotkeyProps) fieldName() string { return p.Name }
func (p *HotkeyProps) initialValue() any { return p.Value }
func (p *HotkeyProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), namedHandler{"onChange", p.OnChange})
}

func (p *HotkeyProps) prepare() error {
	if _, ok := ui.ParseHotkey(p.Value); p.Value != "" && !ok {
		return fmt.Errorf("invalid normal hotkey %q", p.Value)
	}
	return p.Style.validate()
}

type hotkeyConfig struct {
	props  HotkeyProps
	id     string
	theme  component.ControlTheme
	window *ui.Window
	view   *View
	emit   func(...any)
}

type hotkeyState struct{ echo *echoState[string] }

// buildHotkey keeps the shared recorder's keyboard/focus lifecycle below the protocol adapter.
func buildHotkey(b *builder, node *Node, id string) widget.Widget {
	p := *node.Props.(*HotkeyProps)
	p.Disabled = p.Disabled || b.fieldsDisabled()
	p.Label, p.Error, p.Placeholder = b.translate(p.Label), b.translate(p.Error), b.translate(p.Placeholder)
	c := hotkeyConfig{props: p, id: id, theme: b.theme(), window: b.props.Window, view: b.view, emit: b.emitChange(p.OnChange)}
	return p.Style.wrapControl(b.theme(), p.decorateField(p.Label, id, b.theme(), widget.Stateful{Key: widget.Key(id), Type: (*hotkeyState)(nil), Widget: c, CreateState: func() widget.State { return &hotkeyState{} }}))
}

func (s *hotkeyState) InitState(_ widget.StateContext, value any) {
	c := value.(hotkeyConfig)
	s.echo = retainEcho(c.view, c.id, c.props.Value, c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.value
}

func (s *hotkeyState) DidUpdateWidget(_ widget.StateContext, _, value any) {
	c := value.(hotkeyConfig)
	c.view.fieldValues[c.id] = s.echo.receiveField(&c.props)
}

func (*hotkeyState) Dispose() {}

// Build gives recording priority over ancestor shortcuts while leaving Tab and Escape available.
func (s *hotkeyState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(hotkeyConfig)
	onKey := func(e ui.KeyEvent) bool {
		if c.props.Disabled || !e.Down || e.Composing || e.Key == ui.KeyEscape || (e.Key == ui.KeyTab && e.Modifiers & ^ui.KeyModifierShift == 0) || (e.Key == ui.KeyEnter && e.Modifiers == 0) {
			return false
		}
		candidate := e
		candidate.Repeat = false
		next := component.RecordedHotkey(candidate)
		clear := e.Key == ui.KeyBackspace && e.Modifiers == 0
		if clear {
			next = ""
		} else if next == "" {
			return false
		}
		// Repeats still belong to the recorder, so holding a key cannot invoke an ancestor shortcut.
		if e.Repeat {
			return true
		}
		if next != s.echo.value {
			ctx.SetState(func() { s.echo.local(next); c.view.fieldValues[c.id] = next })
			if c.emit != nil {
				c.emit(next)
			}
		}
		return true
	}
	recorder, _ := component.WoxHotkeyRecorder(component.HotkeyRecorderProps{
		ID: c.id + "/field", Labels: component.FormatHotkeyLabels(s.echo.value), Placeholder: c.props.Placeholder,
		Disabled: c.props.Disabled, Error: c.props.Error != "", Theme: c.theme, Window: c.window, OnKey: onKey, Paint: c.props.Style.controlPaint(c.theme),
	})
	return widget.ShortcutScope{Key: widget.Key(c.id + "/recording"), OnKey: onKey, Child: widget.Semantics{
		AutomationID: c.id + "/field", Role: ui.AccessibilityRoleTextField, Label: c.props.fieldLabel(c.props.Label), Value: s.echo.value, Disabled: c.props.Disabled, Child: recorder,
	}}
}

func (p *HotkeyProps) isDisabled() bool { return p.Disabled }
