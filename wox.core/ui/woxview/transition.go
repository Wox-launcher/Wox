package woxview

import (
	"fmt"
	"slices"
	"time"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

type TransitionProperty string

const (
	TransitionOpacity         TransitionProperty = "opacity"
	TransitionBackgroundColor TransitionProperty = "backgroundColor"
	TransitionWidth           TransitionProperty = "width"
	TransitionHeight          TransitionProperty = "height"
	TransitionTranslateX      TransitionProperty = "translateX"
	TransitionTranslateY      TransitionProperty = "translateY"
	TransitionScale           TransitionProperty = "scale"
)

func (TransitionProperty) enumValues() []string {
	return []string{"opacity", "backgroundColor", "width", "height", "translateX", "translateY", "scale"}
}

type TransitionCurve string

const (
	TransitionLinear      TransitionCurve = "linear"
	TransitionEaseOutBack TransitionCurve = "easeOutBack"
	TransitionEaseInOut   TransitionCurve = "easeInOut"
)

func (TransitionCurve) enumValues() []string {
	return []string{"", "linear", "easeOutBack", "easeInOut"}
}

// Transition declares target interpolation in Core; nil duration defaults to 200ms.
type Transition struct {
	Properties []TransitionProperty `json:"properties"`
	Duration   *int                 `json:"duration,omitempty"`
	Curve      TransitionCurve      `json:"curve,omitempty"`
}

// validate keeps channel identities unique and respects the inline style subset.
func (t *Transition) validate(kind NodeType) error {
	if t == nil {
		return nil
	}
	if len(t.Properties) == 0 || len(t.Properties) > 7 {
		return fmt.Errorf("transition.properties must contain 1 to 7 properties")
	}
	seen := map[TransitionProperty]bool{}
	for _, property := range t.Properties {
		if !slices.Contains(TransitionProperty("").enumValues(), string(property)) || seen[property] {
			return fmt.Errorf("invalid or duplicate transition property %q", property)
		}
		if kind == NodeSpan && property != TransitionOpacity && property != TransitionBackgroundColor {
			return fmt.Errorf("span transition only supports opacity and backgroundColor")
		}
		seen[property] = true
	}
	if t.Duration != nil && (*t.Duration < 0 || *t.Duration > 60000) {
		return fmt.Errorf("transition.duration must be between 0 and 60000 milliseconds")
	}
	if !slices.Contains(TransitionCurve("").enumValues(), string(t.Curve)) {
		return fmt.Errorf("unknown transition curve %q", t.Curve)
	}
	return nil
}

// timing maps wire curves to the shared widget clock rather than a second scheduler.
func (t Transition) timing() (time.Duration, widget.AnimationCurve) {
	duration := 200
	if t.Duration != nil {
		duration = *t.Duration
	}
	curve := widget.AnimationEaseInOutCubic
	if t.Curve == TransitionLinear {
		curve = widget.AnimationLinear
	} else if t.Curve == TransitionEaseOutBack {
		curve = widget.AnimationEaseOutBack
	}
	return time.Duration(duration) * time.Millisecond, curve
}

// transitionStyle nests keyed channels around an immutable style snapshot. Tokens
// resolve in logical units before interpolation; display scaling remains in runtime.
func (b *builder) transitionStyle(style Style, transition *Transition, id string, build func(Style) widget.Widget) widget.Widget {
	if transition == nil {
		return build(style)
	}
	duration, curve := transition.timing()
	if duration == 0 {
		return build(style)
	}
	var channel func(int, Style) widget.Widget
	channel = func(index int, sample Style) widget.Widget {
		if index == len(transition.Properties) {
			return build(sample)
		}
		property := transition.Properties[index]
		key := widget.Key(fmt.Sprintf("%p/%s/transition/%s", b.view, id, property))
		next := func(value Style) widget.Widget { return channel(index+1, value) }
		if property == TransitionBackgroundColor {
			color := style.BackgroundColor.resolve(b.theme(), ui.Color{})
			return transitionColor(key, color, duration, curve, func(value Color) widget.Widget {
				current := sample
				current.controlTransition = &component.PaintTransition{Key: widget.Key(string(key) + "/control"), Duration: duration, Curve: curve}
				if style.BackgroundColor != (Color{}) {
					current.controlBackground = &color
				}
				current.BackgroundColor = value
				if current.state != nil && current.state.BackgroundColor != nil {
					patch := *current.state
					patch.BackgroundColor = &value
					current.state = &patch
				}
				return next(current)
			})
		}
		target := float32(1)
		switch property {
		case TransitionOpacity:
			if style.Opacity != nil {
				target = *style.Opacity
			}
		case TransitionWidth:
			target = style.Width.resolve(b.theme())
		case TransitionHeight:
			target = style.Height.resolve(b.theme())
		case TransitionTranslateX:
			target = style.TranslateX.resolve(b.theme())
		case TransitionTranslateY:
			target = style.TranslateY.resolve(b.theme())
		case TransitionScale:
			if style.Scale != nil {
				target = *style.Scale
			}
		}
		// Zero dimensions mean natural layout, not zero pixels. A natural size
		// cannot be measured by laying out the same retained subtree twice.
		if (property == TransitionWidth || property == TransitionHeight) && target == 0 {
			return next(sample)
		}
		return widget.AnimatedFloat{Key: key, Target: target, Duration: duration, Curve: curve, Builder: func(value float32) widget.Widget {
			current := sample
			switch property {
			case TransitionOpacity:
				value = max(float32(0), min(float32(1), value))
				current.Opacity = &value
			case TransitionWidth:
				current.Width = Size{Value: max(float32(.001), value)}
			case TransitionHeight:
				current.Height = Size{Value: max(float32(.001), value)}
			case TransitionTranslateX:
				current.TranslateX = Offset{Value: value}
			case TransitionTranslateY:
				current.TranslateY = Offset{Value: value}
			case TransitionScale:
				current.Scale = &value
			}
			return next(current)
		}}
	}
	return channel(0, style)
}

// transitionColor interpolates straight RGBA channels independently, including alpha.
func transitionColor(key widget.Key, target ui.Color, duration time.Duration, curve widget.AnimationCurve, build func(Color) widget.Widget) widget.Widget {
	return widget.AnimatedColor{Key: key, Target: target, Duration: duration, Curve: curve, Builder: func(sample ui.Color) widget.Widget {
		return build(Color{Value: ColorValue(fmt.Sprintf("#%02x%02x%02x%02x", sample.R, sample.G, sample.B, sample.A))})
	}}
}
