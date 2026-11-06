package widget

import (
	"math"
	"strconv"
	"time"

	ui "wox/ui/runtime"
)

// AnimatedColor shares the Host clock across four straight RGBA channels.
// Overshooting curves are clamped to byte range without changing the timeline.
type AnimatedColor struct {
	Key      Key
	Target   ui.Color
	Duration time.Duration
	Curve    AnimationCurve
	Builder  func(ui.Color) Widget
}

func (w AnimatedColor) layout(ctx context, available constraints) *node {
	if w.Builder == nil {
		return &node{}
	}
	values := [4]uint8{w.Target.R, w.Target.G, w.Target.B, w.Target.A}
	var channel func(int, [4]uint8) Widget
	channel = func(index int, sample [4]uint8) Widget {
		if index == 4 {
			return w.Builder(ui.Color{R: sample[0], G: sample[1], B: sample[2], A: sample[3]})
		}
		key := w.Key
		if key != "" {
			key = Key(string(key) + "/" + strconv.Itoa(index))
		}
		return AnimatedFloat{Key: key, Target: float32(values[index]), Duration: w.Duration, Curve: w.Curve, Builder: func(value float32) Widget {
			current := sample
			current[index] = uint8(math.Round(float64(max(float32(0), min(float32(255), value)))))
			return channel(index+1, current)
		}}
	}
	child := channel(0, [4]uint8{})
	if child == nil {
		return &node{}
	}
	return child.layout(ctx, available)
}
