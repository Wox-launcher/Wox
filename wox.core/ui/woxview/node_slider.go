package woxview

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// SliderProps is a horizontal range control. Max defaults to 1 when unset.
type SliderProps struct {
	Styled
	FieldProps
	Name     string   `json:"name,omitempty"`
	Label    string   `json:"label,omitempty"`
	Value    float32  `json:"value,omitempty"`
	Min      float32  `json:"min,omitempty"`
	Max      float32  `json:"max,omitempty"`
	Step     float32  `json:"step,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	OnChange *Handler `json:"onChange,omitempty"`
}

func (p *SliderProps) fieldName() string { return p.Name }
func (p *SliderProps) initialValue() any { return p.Value }
func (p *SliderProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onChange", p.OnChange}}...)
}

func (p *SliderProps) prepare() error {
	if p.Max == 0 && p.Min == 0 {
		p.Max = 1
	}
	if p.Max <= p.Min {
		return errors.New("max must be greater than min")
	}
	if p.Step < 0 {
		return errors.New("step must not be negative")
	}
	p.Value = min(max(p.Value, p.Min), p.Max)
	return p.Style.validate()
}

type sliderConfig struct {
	props    *SliderProps
	id       string
	theme    component.ControlTheme
	onChange func(args ...any)
	view     *View
}

// sliderState shows the dragged value immediately; producer echoes are reconciled like text fields.
type sliderState struct {
	echo    *echoState[float32]
	hovered bool
	active  bool
}

func buildSlider(b *builder, node *Node, id string) widget.Widget {
	p := *node.Props.(*SliderProps)
	p.Disabled = p.Disabled || b.fieldsDisabled()
	p.Label = b.translate(p.Label)
	p.Error = b.translate(p.Error)
	props := &p
	config := sliderConfig{props: props, id: id, theme: b.theme(), onChange: b.emitChange(props.OnChange), view: b.view}
	return props.Style.wrapControl(b.theme(), props.decorateField(props.Label, id, b.theme(), widget.Stateful{
		Key: widget.Key(id), Type: (*sliderState)(nil), Widget: config,
		CreateState: func() widget.State { return &sliderState{} },
	}))
}

func (s *sliderState) InitState(_ widget.StateContext, config any) {
	c := config.(sliderConfig)
	s.echo = retainEcho(c.view, c.id, c.props.Value, c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.value
}

func (s *sliderState) DidUpdateWidget(_ widget.StateContext, _, config any) {
	c := config.(sliderConfig)
	s.echo.receive(c.props.Value, c.props.Revision)
	c.view.fieldValues[c.id] = s.echo.value
}

func (s *sliderState) Dispose() {}

func (s *sliderState) Build(context widget.StateContext, value any) widget.Widget {
	config := value.(sliderConfig)
	props := config.props
	height := props.Style.Height.resolve(config.theme)
	if height <= 0 {
		height = 24
	}
	return widget.LayoutBuilder{Build: func(available woxui.Size) widget.Widget {
		track := component.SliderTrackProps{
			Width: widthOr(props.Style.resolve(config.theme), available), Height: height, Min: props.Min, Max: props.Max, Step: props.Step,
			Value: s.echo.value, Hovered: s.hovered && !props.Disabled, Active: s.active, Theme: config.theme, Paint: props.Style.controlPaint(config.theme),
		}
		setValue := func(next float32) {
			next = min(max(next, props.Min), props.Max)
			if props.Step > 0 {
				next = min(props.Max, props.Min+float32(math.Round(float64((next-props.Min)/props.Step)))*props.Step)
			}
			if next == s.echo.value {
				return
			}
			context.SetState(func() {
				s.echo.local(next)
				config.view.fieldValues[config.id] = s.echo.value
			})
			if config.onChange != nil {
				config.onChange(next)
			}
		}
		update := func(position woxui.Point) { setValue(track.ValueAt(position.X)) }
		step := props.Step
		if step == 0 {
			step = (props.Max - props.Min) / 100
		}
		actions := []woxui.AccessibilityAction(nil)
		if !props.Disabled {
			actions = []woxui.AccessibilityAction{woxui.AccessibilityActionIncrement, woxui.AccessibilityActionDecrement, woxui.AccessibilityActionSetValue}
		}
		key := widget.Key(config.id)
		gesture := widget.Gesture{
			ID: config.id + "/track", Cursor: woxui.PointerCursorHand,
			OnHover:     func(inside bool) { context.SetState(func() { s.hovered = inside }) },
			OnPanStart:  func(position woxui.Point) { context.SetState(func() { s.active = true }); update(position) },
			OnPanUpdate: update,
			OnPanEnd:    func() { context.SetState(func() { s.active = false }) },
			OnTapAt:     update,
			Child:       component.WoxSliderTrack(track),
		}
		if props.Disabled {
			gesture.OnHover = nil
			gesture.OnPanStart = nil
			gesture.OnPanUpdate = nil
			gesture.OnPanEnd = nil
			gesture.OnTapAt = nil
		}
		return widget.Semantics{Key: key, AutomationID: config.id, Role: woxui.AccessibilityRoleSlider, Label: props.fieldLabel(props.Label), Value: strconv.FormatFloat(float64(s.echo.value), 'f', -1, 32), Disabled: props.Disabled, Actions: actions,
			OnAction: func(action woxui.AccessibilityAction, value string) error {
				if props.Disabled {
					return errors.New("slider disabled")
				}
				switch action {
				case woxui.AccessibilityActionIncrement:
					setValue(s.echo.value + step)
				case woxui.AccessibilityActionDecrement:
					setValue(s.echo.value - step)
				case woxui.AccessibilityActionSetValue:
					n, err := strconv.ParseFloat(value, 32)
					if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
						return errors.New("invalid slider value")
					}
					setValue(float32(n))
				default:
					return fmt.Errorf("unsupported slider action %q", action)
				}
				return nil
			},
			Child: widget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: config.theme.Focus, OnKey: func(e woxui.KeyEvent) bool {
				if !e.Down || props.Disabled {
					return false
				}
				switch e.Key {
				case woxui.KeyArrowUp, woxui.KeyArrowRight:
					setValue(s.echo.value + step)
				case woxui.KeyArrowDown, woxui.KeyArrowLeft:
					setValue(s.echo.value - step)
				case woxui.KeyHome:
					setValue(props.Min)
				case woxui.KeyEnd:
					setValue(props.Max)
				default:
					return false
				}
				return true
			}, Child: gesture},
		}
	}}
}

func (p *SliderProps) isDisabled() bool { return p.Disabled }
