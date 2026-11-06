package woxview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	component "wox/ui/launcher/component"
	"wox/ui/widget"
)

type PresencePreset string

const (
	PresenceFade    PresencePreset = "fade"
	PresenceSlideUp PresencePreset = "slideUp"
	PresenceScale   PresencePreset = "scale"
)

func (PresencePreset) enumValues() []string { return []string{"fade", "slideUp", "scale"} }

// PresenceAnimation declares a mount/unmount preset in logical coordinates.
// A nil duration uses 200ms; explicit zero disables the animation.
type PresenceAnimation struct {
	Type     PresencePreset `json:"type"`
	Duration *int           `json:"duration,omitempty"`
}

// UnmarshalJSON accepts a preset shorthand or a strict object with timing.
func (a *PresenceAnimation) UnmarshalJSON(data []byte) error {
	var next PresenceAnimation
	if data = bytes.TrimSpace(data); len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &next.Type); err != nil {
			return err
		}
	} else {
		type plain PresenceAnimation
		if err := decodeStrict(data, (*plain)(&next)); err != nil {
			return err
		}
	}
	if err := next.validate(); err != nil {
		return err
	}
	*a = next
	return nil
}

// validate also checks programmatically assembled trees before they reach Build.
func (a *PresenceAnimation) validate() error {
	if a == nil {
		return nil
	}
	if !slices.Contains(PresencePreset("").enumValues(), string(a.Type)) {
		return fmt.Errorf("unknown presence preset %q", a.Type)
	}
	if a.Duration != nil && (*a.Duration < 0 || *a.Duration > 60000) {
		return fmt.Errorf("presence duration must be between 0 and 60000 milliseconds")
	}
	return nil
}

func (a PresenceAnimation) duration() time.Duration {
	if a.Duration != nil {
		return time.Duration(*a.Duration) * time.Millisecond
	}
	return 200 * time.Millisecond
}

// receiveEnter captures the first mounted declaration. Prop echoes and changes
// cannot restart a mount; removing the declaration cancels it immediately.
func (s *nodeVisualState) receiveEnter(owner *View, common CommonProps) {
	if s.owner != owner {
		s.owner, s.enter = owner, common.Enter
	} else if common.Enter == nil {
		s.enter = nil
	}
}

// enterStyle composes after transitions, so target interpolation never chases
// the temporary pose. State identity distinguishes a removed/reinserted key.
func (s *nodeVisualState) enterStyle(ctx widget.StateContext, b *builder, id string, style Style, build func(Style) widget.Widget) widget.Widget {
	a := s.enter
	if a == nil {
		return build(style)
	}
	initial := float32(0)
	return widget.AnimatedFloat{Key: widget.Key(fmt.Sprintf("%p/%p/%s/enter", b.view, s, id)), Initial: &initial, Target: 1,
		Duration: a.duration(), Curve: widget.AnimationEaseInOutCubic, Builder: func(progress float32) widget.Widget {
			if progress >= 1 {
				ctx.PostFrame(func() {
					if ctx.Mounted() && s.enter == a {
						ctx.SetState(func() { s.enter = nil })
					}
				})
			}
			return build(presenceStyle(style, a.Type, progress, b.theme()))
		}}
}

// presenceStyle multiplies authored opacity/scale and offsets resolved logical
// translation. Physical display scaling is still performed by the renderer.
func presenceStyle(style Style, preset PresencePreset, progress float32, theme component.ControlTheme) Style {
	opacity := float32(1)
	if style.Opacity != nil {
		opacity = *style.Opacity
	}
	opacity *= progress
	style.Opacity = &opacity
	switch preset {
	case PresenceSlideUp:
		style.TranslateY = Offset{Value: style.TranslateY.resolve(theme) + theme.Scaled(12)*(1-progress)}
	case PresenceScale:
		scale := float32(1)
		if style.Scale != nil {
			scale = *style.Scale
		}
		scale *= .95 + .05*progress
		style.Scale = &scale
	}
	return style
}
