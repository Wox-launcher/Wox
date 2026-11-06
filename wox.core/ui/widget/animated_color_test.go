package widget

import (
	"testing"
	"time"

	ui "wox/ui/runtime"
)

// TestAnimatedColor exercises independent RGBA channels, continuous retargeting,
// clamped overshoot and unkeyed/immediate behavior on the real Host clock.
func TestAnimatedColor(t *testing.T) {
	now := time.Unix(1, 0)
	target := ui.Color{R: 255, A: 255}
	var sample ui.Color
	key, duration, curve := Key("color"), 200*time.Millisecond, AnimationLinear
	host := NewHost(func(ui.FrameInfo) Widget {
		return AnimatedColor{Key: key, Target: target, Duration: duration, Curve: curve, Builder: func(c ui.Color) Widget {
			sample = c
			return Container{Width: 20, Height: 20, Color: c}
		}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	host.SetAnimationClock(func() time.Time { return now })
	renderTestFrame(host)
	target = ui.Color{B: 255, A: 127}
	renderTestFrame(host)
	now = now.Add(100 * time.Millisecond)
	renderTestFrame(host)
	if sample != (ui.Color{R: 128, B: 128, A: 191}) {
		t.Fatalf("midpoint %+v", sample)
	}
	target = ui.Color{G: 255, A: 255}
	renderTestFrame(host)
	if sample != (ui.Color{R: 128, B: 128, A: 191}) {
		t.Fatalf("retarget jumped %+v", sample)
	}
	now = now.Add(200 * time.Millisecond)
	renderTestFrame(host)
	if sample != target {
		t.Fatalf("final %+v", sample)
	}
	curve, target = AnimationEaseOutBack, ui.Color{R: 255, A: 255}
	renderTestFrame(host)
	now = now.Add(140 * time.Millisecond)
	renderTestFrame(host)
	if sample.R != 255 || sample.G != 0 {
		t.Fatalf("overshoot wrapped bytes %+v", sample)
	}
	duration, target = 0, ui.Color{B: 70, A: 130}
	renderTestFrame(host)
	if sample != target {
		t.Fatal("zero duration did not snap")
	}
	key, duration, target = "", time.Second, ui.Color{R: 90, A: 160}
	renderTestFrame(host)
	if sample != target {
		t.Fatal("empty key created a timeline")
	}
}
