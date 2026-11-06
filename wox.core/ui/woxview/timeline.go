package woxview

import (
	"fmt"
	"math"
	"time"

	"wox/ui/widget"
)

// TimelineKeyframe contains paint-only channels; omitted channels hold their previous value.
type TimelineKeyframe struct {
	At         int      `json:"at"`
	TranslateX *float32 `json:"translateX,omitempty"`
	TranslateY *float32 `json:"translateY,omitempty"`
	Scale      *float32 `json:"scale,omitempty"`
	Rotate     *float32 `json:"rotate,omitempty"`
	Opacity    *float32 `json:"opacity,omitempty"`
}

// Timeline plays native keyframes in milliseconds, independently of producer updates.
type Timeline struct {
	PlaybackID string             `json:"playbackId,omitempty"`
	Duration   int                `json:"duration"`
	Elapsed    int                `json:"elapsed,omitempty"`
	Loop       bool               `json:"loop,omitempty"`
	Paused     bool               `json:"paused,omitempty"`
	Keyframes  []TimelineKeyframe `json:"keyframes"`
}

// validate limits work and rejects ambiguous keyframe times or nonfinite paint values.
func (t *Timeline) validate(kind NodeType) error {
	if t == nil {
		return nil
	}
	if kind == NodeSpan {
		return fmt.Errorf("timeline is not supported on span")
	}
	if t.Duration < 1 || t.Duration > 600000 || t.Elapsed < 0 || t.Elapsed > 86400000 || len(t.PlaybackID) > 256 {
		return fmt.Errorf("invalid timeline duration, elapsed or playbackId")
	}
	if len(t.Keyframes) < 2 || len(t.Keyframes) > 256 || t.Keyframes[0].At != 0 || t.Keyframes[len(t.Keyframes)-1].At != t.Duration {
		return fmt.Errorf("timeline needs 2..256 keyframes from 0 to duration")
	}
	for i, k := range t.Keyframes {
		if i > 0 && k.At <= t.Keyframes[i-1].At {
			return fmt.Errorf("timeline keyframes must be strictly ordered")
		}
		for _, v := range []*float32{k.TranslateX, k.TranslateY, k.Scale, k.Rotate, k.Opacity} {
			if v != nil && (math.IsNaN(float64(*v)) || math.IsInf(float64(*v), 0) || *v < -16384 || *v > 16384) {
				return fmt.Errorf("invalid timeline channel")
			}
		}
		if k.Scale != nil && (*k.Scale < 0 || *k.Scale > 100) || k.Opacity != nil && (*k.Opacity < 0 || *k.Opacity > 1) {
			return fmt.Errorf("invalid timeline scale or opacity")
		}
	}
	return nil
}

// apply overlays only authored channels, preserving unrelated styles and layout.
func (k TimelineKeyframe) apply(s Style) Style {
	if k.TranslateX != nil {
		s.TranslateX = Offset{Value: *k.TranslateX}
	}
	if k.TranslateY != nil {
		s.TranslateY = Offset{Value: *k.TranslateY}
	}
	if k.Scale != nil {
		s.Scale = k.Scale
	}
	if k.Rotate != nil {
		s.Rotate = *k.Rotate
	}
	if k.Opacity != nil {
		s.Opacity = k.Opacity
	}
	return s
}

// sample interpolates a path without modifying the immutable document.
func (t *Timeline) sample(base Style, elapsed int) Style {
	previous := t.Keyframes[0].apply(base)
	for i := 1; i < len(t.Keyframes); i++ {
		a, z := t.Keyframes[i-1], t.Keyframes[i]
		next := z.apply(previous)
		if elapsed <= z.At {
			progress := min(float32(1), max(float32(0), float32(elapsed-a.At)/float32(z.At-a.At)))
			mix := func(a, z float32) float32 { return a + (z-a)*progress }
			previous.TranslateX = Offset{Value: mix(previous.TranslateX.Value, next.TranslateX.Value)}
			previous.TranslateY = Offset{Value: mix(previous.TranslateY.Value, next.TranslateY.Value)}
			previous.Rotate = mix(previous.Rotate, next.Rotate)
			if previous.Scale != nil || next.Scale != nil {
				from, to := float32(1), float32(1)
				if previous.Scale != nil {
					from = *previous.Scale
				}
				if next.Scale != nil {
					to = *next.Scale
				}
				value := mix(from, to)
				previous.Scale = &value
			}
			if previous.Opacity != nil || next.Opacity != nil {
				from, to := float32(1), float32(1)
				if previous.Opacity != nil {
					from = *previous.Opacity
				}
				if next.Opacity != nil {
					to = *next.Opacity
				}
				value := mix(from, to)
				previous.Opacity = &value
			}
			return previous
		}
		previous = next
	}
	return previous
}

// timelineStyle shares one host clock with sprite clips, transitions and presence.
func (b *builder) timelineStyle(style Style, t *Timeline, id string, build func(Style) widget.Widget) widget.Widget {
	if t == nil {
		return build(style)
	}
	style.TranslateX = Offset{Value: style.TranslateX.resolve(b.theme())}
	style.TranslateY = Offset{Value: style.TranslateY.resolve(b.theme())}
	return widget.TimelineAnimation{Key: widget.Key(fmt.Sprintf("%p/%s/timeline", b.view, id)), PlaybackID: t.PlaybackID, Duration: time.Duration(t.Duration) * time.Millisecond, Elapsed: time.Duration(t.Elapsed) * time.Millisecond, Loop: t.Loop, Paused: t.Paused,
		Builder: func(elapsed time.Duration) widget.Widget {
			return build(t.sample(style, int(elapsed/time.Millisecond)))
		},
	}
}
