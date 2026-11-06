package woxview

import (
	"errors"
	"math"
	"strconv"
	"strings"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// NumberFieldProps keeps numeric form values separate from the editor's incomplete text.
type NumberFieldProps struct {
	Styled
	FieldProps
	Name        string   `json:"name,omitempty"`
	Label       string   `json:"label,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Value       float64  `json:"value,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Step        float64  `json:"step,omitempty"`
	Disabled    bool     `json:"disabled,omitempty"`
	Autofocus   bool     `json:"autofocus,omitempty"`
	OnChange    *Handler `json:"onChange,omitempty"`
	OnSubmit    *Handler `json:"onSubmit,omitempty"`
}

func (p *NumberFieldProps) fieldName() string { return p.Name }
func (p *NumberFieldProps) initialValue() any { return p.Value }
func (p *NumberFieldProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), namedHandler{"onChange", p.OnChange}, namedHandler{"onSubmit", p.OnSubmit})
}

// prepare validates finite arithmetic before accepting a range into retained UI state.
func (p *NumberFieldProps) prepare() error {
	for _, v := range []*float64{&p.Value, &p.Step, p.Min, p.Max} {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0)) {
			return errors.New("numeric values must be finite")
		}
	}
	if p.Min != nil && p.Max != nil && *p.Min > *p.Max {
		return errors.New("max must not be less than min")
	}
	if p.Step < 0 {
		return errors.New("step must not be negative")
	}
	if p.Step == 0 {
		p.Step = 1
	}
	p.Value = p.clamp(p.Value)
	return p.Style.validate()
}

func (p *NumberFieldProps) normalizeFieldValue(value any) any { return p.clamp(value.(float64)) }

// clamp also reconciles local values when a producer narrows an offscreen field's range.
func (p *NumberFieldProps) clamp(value float64) float64 {
	if p.Min != nil {
		value = max(value, *p.Min)
	}
	if p.Max != nil {
		value = min(value, *p.Max)
	}
	return value
}

// snap uses min (or zero) as the step origin, without allowing overflow to escape as an event.
func (p *NumberFieldProps) snap(value float64) float64 {
	value = p.clamp(value)
	origin := float64(0)
	if p.Min != nil {
		origin = *p.Min
	}
	steps := math.Round((value - origin) / p.Step)
	snapped := origin + steps*p.Step
	if !math.IsInf(snapped, 0) && !math.IsNaN(snapped) {
		// Step arithmetic such as 3 * 0.1 must not render an artificial 0.30000000000000004.
		precision := 0
		for _, n := range []float64{origin, p.Step} {
			if _, fraction, ok := strings.Cut(strconv.FormatFloat(n, 'f', -1, 64), "."); ok {
				precision = max(precision, len(fraction))
			}
		}
		value, _ = strconv.ParseFloat(strconv.FormatFloat(snapped, 'f', precision, 64), 64)
	}
	return p.clamp(value)
}

type numberFieldConfig struct {
	props      NumberFieldProps
	id         string
	theme      component.ControlTheme
	window     *ui.Window
	view       *View
	onChange   func(...any)
	onSubmit   func(...any)
	submitForm func()
}

type numberFieldState struct {
	echo       *echoState[float64]
	controller *widget.TextEditingController
	shown      float64
	revision   uint64
}

// buildNumberField uses the same native editor and field decoration as other form inputs.
func buildNumberField(b *builder, node *Node, id string) widget.Widget {
	p := *node.Props.(*NumberFieldProps)
	p.Disabled = p.Disabled || b.fieldsDisabled()
	p.Label, p.Error, p.Placeholder = b.translate(p.Label), b.translate(p.Error), b.translate(p.Placeholder)
	c := numberFieldConfig{props: p, id: id, theme: b.theme(), window: b.props.Window, view: b.view,
		onChange: b.emitChange(p.OnChange), onSubmit: b.emit(p.OnSubmit), submitForm: b.formSubmit()}
	return p.Style.wrapControl(b.theme(), p.decorateField(p.Label, id, b.theme(), widget.Stateful{Key: widget.Key(id), Type: (*numberFieldState)(nil), Widget: c, CreateState: func() widget.State { return &numberFieldState{} }}))
}

func (s *numberFieldState) InitState(_ widget.StateContext, value any) {
	c := value.(numberFieldConfig)
	s.echo = retainEcho(c.view, c.id, c.props.Value, c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.receiveField(&c.props)
	s.shown, s.revision = s.echo.value, c.props.Revision
	s.controller = widget.NewTextEditingController(strconv.FormatFloat(s.shown, 'f', -1, 64))
}

// DidUpdateWidget preserves incomplete drafts until a real value change or explicit reset arrives.
func (s *numberFieldState) DidUpdateWidget(_ widget.StateContext, _, value any) {
	c := value.(numberFieldConfig)
	c.view.fieldValues[c.id] = s.echo.receiveField(&c.props)
	if s.shown != s.echo.value || s.revision != c.props.Revision {
		s.controller.SetText(strconv.FormatFloat(s.echo.value, 'f', -1, 64), false)
	}
	s.shown, s.revision = s.echo.value, c.props.Revision
}

func (*numberFieldState) Dispose() {}

// setValue updates the form immediately, while the editor may retain a different textual spelling.
func (s *numberFieldState) setValue(ctx widget.StateContext, c numberFieldConfig, value float64) {
	if value == s.echo.value {
		return
	}
	ctx.SetState(func() {
		s.echo.local(value)
		s.shown = value
		c.view.fieldValues[c.id] = value
	})
	if c.onChange != nil {
		c.onChange(value)
	}
}

// commit resolves incomplete or out-of-range text before a blur or form submission.
func (s *numberFieldState) commit(ctx widget.StateContext, c numberFieldConfig) {
	value, err := strconv.ParseFloat(strings.TrimSpace(s.controller.Text()), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		value = s.echo.value
	}
	value = c.props.snap(value)
	s.setValue(ctx, c, value)
	s.controller.SetText(strconv.FormatFloat(value, 'f', -1, 64), false)
	ctx.Invalidate()
}

// Build keeps native editing/IME behavior while handling numeric commit and arrow stepping locally.
func (s *numberFieldState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(numberFieldConfig)
	p, theme := c.props, c.theme
	height := p.Style.Height.resolve(theme)
	if height == 0 {
		height = theme.Scaled(component.SettingsControlHeight)
	}
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		return component.WoxTextField(component.TextFieldProps{
			ID: c.id + "/field", Label: p.fieldLabel(p.Label), Controller: s.controller, Value: s.controller.Text(), Hint: p.Placeholder,
			Width: widthOr(p.Style.resolve(theme), available), Height: height, Radius: theme.Scaled(4),
			Padding: widget.Insets{Left: theme.Scaled(8), Top: theme.Scaled(6), Right: theme.Scaled(8), Bottom: theme.Scaled(6)}, Theme: theme, Window: c.window,
			Background: theme.InputBackground, BorderColor: theme.Border, BorderWidth: 1, FocusRingColor: theme.Focus,
			TextColor: theme.Text, Style: ui.TextStyle{Size: component.SettingsControlFontSize}, MaxLines: 1, Disabled: p.Disabled, Autofocus: p.Autofocus, Paint: p.Style.controlPaint(theme),
			OnChanged: func(text string) {
				if p.Disabled {
					return
				}
				if n, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && p.clamp(n) == n {
					s.setValue(ctx, c, n)
				}
			},
			OnFocusChange: func(focused bool) {
				if !focused && !p.Disabled && c.view.fieldStates[c.id] == s.echo {
					s.commit(ctx, c)
				}
			},
			OnKey: func(e ui.KeyEvent) bool {
				if p.Disabled || !e.Down || e.Composing || e.Modifiers != 0 {
					return false
				}
				switch e.Key {
				case ui.KeyEnter:
					s.commit(ctx, c)
					if c.onSubmit != nil {
						c.onSubmit(s.echo.value)
					} else if c.submitForm != nil {
						c.submitForm()
					}
					return true
				case ui.KeyArrowUp, ui.KeyArrowDown:
					delta := p.Step
					if e.Key == ui.KeyArrowDown {
						delta = -delta
					}
					next := s.echo.value + delta
					if !math.IsInf(next, 0) {
						s.setValue(ctx, c, p.snap(next))
					}
					s.controller.SetText(strconv.FormatFloat(s.echo.value, 'f', -1, 64), false)
					ctx.Invalidate()
					return true
				}
				return false
			},
		})
	}}
}

func (p *NumberFieldProps) isDisabled() bool { return p.Disabled }
