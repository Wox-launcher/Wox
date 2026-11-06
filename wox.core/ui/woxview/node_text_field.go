package woxview

import (
	"errors"
	"reflect"
	"runtime"
	"slices"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// TextFieldProps is a text input. It works uncontrolled (Value is only the initial text) and
// controlled (the producer echoes onChange values back, or replaces the text by sending a new Value).
type TextFieldProps struct {
	Styled
	FieldProps
	Name           string   `json:"name,omitempty"`
	Label          string   `json:"label,omitempty"`
	Value          string   `json:"value,omitempty"`
	Placeholder    string   `json:"placeholder,omitempty"`
	MaxLines       int      `json:"maxLines,omitempty"`
	ChangeDebounce int      `json:"changeDebounce,omitempty"`
	Disabled       bool     `json:"disabled,omitempty"`
	Autofocus      bool     `json:"autofocus,omitempty"`
	Password       bool     `json:"password,omitempty"`
	OnChange       *Handler `json:"onChange,omitempty"`
	OnSubmit       *Handler `json:"onSubmit,omitempty"`
}

func (p *TextFieldProps) fieldName() string { return p.Name }
func (p *TextFieldProps) initialValue() any { return p.Value }
func (p *TextFieldProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onChange", p.OnChange}, {"onSubmit", p.OnSubmit}}...)
}

func (p *TextFieldProps) prepare() error {
	if p.MaxLines < 0 || p.MaxLines > 100 {
		return errors.New("maxLines must be between 0 and 100")
	}
	if p.ChangeDebounce < 0 || p.ChangeDebounce > 60000 {
		return errors.New("changeDebounce must be between 0 and 60000 milliseconds")
	}
	return p.Style.validate()
}

// echoState keeps a locally edited value stable while the producer's echoes arrive late.
// Without it, a controlled field would jump back to an older value whenever the user types
// faster than the round trip, because each stale echo looks like a programmatic change.
// Values are immutable snapshots; structural equality also supports array-valued fields.
type echoState[T any] struct {
	value    T // what the control shows
	lastProp T // last value received from the producer
	revision uint64
	pending  []T // local values reported to the producer and not yet echoed back
}

func (s *echoState[T]) init(value T, revision uint64) {
	s.value, s.lastProp, s.revision, s.pending = value, value, revision, nil
}

// local records a user edit that will be reported to the producer.
func (s *echoState[T]) local(value T) {
	s.value = value
	s.pending = append(s.pending, value)
	if len(s.pending) > 64 {
		s.pending = s.pending[len(s.pending)-64:]
	}
}

// receive applies a producer value. Echoes of local edits only acknowledge them; an unchanged
// prop is ignored (uncontrolled use); any other change is a programmatic update and wins.
// A revision change explicitly resets even an unchanged prop and discards pending echoes.
func (s *echoState[T]) receive(value T, revision uint64) {
	if revision != s.revision {
		s.init(value, revision)
		return
	}
	if reflect.DeepEqual(value, s.lastProp) {
		return
	}
	s.lastProp = value
	if index := slices.IndexFunc(s.pending, func(pending T) bool { return reflect.DeepEqual(pending, value) }); index >= 0 {
		s.pending = s.pending[index+1:]
		return
	}
	s.value, s.pending = value, nil
}

type textFieldConfig struct {
	props      *TextFieldProps
	id         string
	theme      component.ControlTheme
	window     *woxui.Window
	onChange   func(args ...any)
	onSubmit   func(args ...any)
	submitForm func()
	view       *View
}

type textFieldState struct {
	echo       *echoState[string]
	controller *widget.TextEditingController
	revision   uint64
}

func buildTextField(b *builder, node *Node, id string) widget.Widget {
	p := *node.Props.(*TextFieldProps)
	p.Disabled = p.Disabled || b.fieldsDisabled()
	p.Label = b.translate(p.Label)
	p.Error = b.translate(p.Error)
	p.Placeholder = b.translate(p.Placeholder)
	props := &p
	config := textFieldConfig{props: props, id: id, theme: b.theme(), window: b.props.Window, onChange: b.emitChange(props.OnChange), onSubmit: b.emit(props.OnSubmit), view: b.view}
	config.submitForm = b.formSubmit()
	return props.Style.wrapControl(b.theme(), props.decorateField(props.Label, id, b.theme(), widget.Stateful{
		Key: widget.Key(id), Type: (*textFieldState)(nil), Widget: config,
		CreateState: func() widget.State { return &textFieldState{} },
	}))
}

func (s *textFieldState) InitState(_ widget.StateContext, config any) {
	c := config.(textFieldConfig)
	s.echo = retainEcho(c.view, c.id, c.props.Value, c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.value
	s.controller = widget.NewTextEditingController(s.echo.value)
	s.revision = c.props.Revision
}

func (s *textFieldState) DidUpdateWidget(_ widget.StateContext, _, config any) {
	c := config.(textFieldConfig)
	reset := s.revision != c.props.Revision
	s.revision = c.props.Revision
	s.echo.receive(c.props.Value, c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.value
	if reset || s.controller.Text() != s.echo.value {
		s.controller.SetText(s.echo.value, false)
	}
}

func (s *textFieldState) Dispose() {}

// Build uses the settings input look so plugin windows and future settings pages match.
func (s *textFieldState) Build(_ widget.StateContext, value any) widget.Widget {
	config := value.(textFieldConfig)
	props, theme := config.props, config.theme
	style := props.Style.resolve(theme)
	maxLines := max(1, props.MaxLines)
	height := style.Height
	if height <= 0 {
		height = component.SettingsControlHeight
		if maxLines > 1 {
			height = float32(maxLines)*20 + 12
		}
	}
	textColor := style.Color.resolve(theme, theme.Text)
	if props.Disabled {
		textColor = theme.TextSecondary
	}
	fontSize := style.FontSize
	if fontSize <= 0 {
		fontSize = component.SettingsControlFontSize
	}
	radius := style.BorderRadius
	if radius <= 0 {
		radius = 4
	}
	background, authoredFill := props.Style.controlFill(theme)
	return widget.LayoutBuilder{Build: func(available woxui.Size) widget.Widget {
		return component.WoxTextField(component.TextFieldProps{
			ID: config.id + "/field", Label: props.fieldLabel(props.Label), Controller: s.controller, Value: s.echo.value, Hint: props.Placeholder, Width: widthOr(style, available), Height: height,
			Radius: radius, Padding: widget.Insets{Left: 8, Top: 6, Right: 8, Bottom: 6}, Transparent: !authoredFill,
			Background: background, BorderColor: style.BorderColor.resolve(theme, theme.TextSecondary), BorderWidth: 1,
			FocusRingColor: theme.Focus, Style: woxui.TextStyle{Size: fontSize}, TextColor: textColor, MaxLines: maxLines,
			Disabled: props.Disabled, Autofocus: props.Autofocus, Protected: props.Password, Window: config.window, Theme: theme, Paint: props.Style.controlPaint(theme),
			OnChanged: func(text string) {
				s.echo.local(text)
				config.view.fieldValues[config.id] = s.echo.value
				config.view.textChanged(config, text)
			},
			OnFocusChange: func(focused bool) {
				if !focused {
					config.view.flushChange(config.id)
				}
			},
			OnKey: func(event woxui.KeyEvent) bool {
				// Multiline fields reserve plain Enter for a newline.
				modifier := woxui.KeyModifierControl
				if runtime.GOOS == "darwin" {
					modifier = woxui.KeyModifierMeta
				}
				if props.Disabled || !event.Down || event.Key != woxui.KeyEnter || (maxLines > 1 && event.Modifiers&modifier == 0) {
					return false
				}
				if config.onSubmit != nil {
					config.view.flushChange(config.id)
					config.onSubmit(s.echo.value)
					return true
				}
				if config.submitForm != nil {
					config.submitForm()
					return true
				}
				return false
			},
		})
	}}
}

func (p *TextFieldProps) isDisabled() bool { return p.Disabled }
