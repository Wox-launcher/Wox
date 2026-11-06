package widget

import (
	"testing"
	"time"

	ui "wox/ui/runtime"
)

// TestAnimatedFloatInitial starts fresh mounts at Initial while retaining normal
// target continuity and zero-duration behavior across subsequent updates.
func TestAnimatedFloatInitial(t *testing.T) {
	now := time.Unix(1, 0)
	initial, target, sample := float32(0), float32(1), float32(0)
	key, duration, show := Key("entry"), 200*time.Millisecond, true
	host := NewHost(func(ui.FrameInfo) Widget {
		if !show {
			return Container{}
		}
		return AnimatedFloat{Key: key, Target: target, Initial: &initial, Duration: duration, Builder: func(v float32) Widget { sample = v; return Container{Width: 20, Height: 20} }}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	host.SetAnimationClock(func() time.Time { return now })
	frame := func(want float32) {
		t.Helper()
		renderTestFrame(host)
		if sample != want {
			t.Fatalf("sample %g want %g", sample, want)
		}
	}
	frame(0)
	now = now.Add(100 * time.Millisecond)
	frame(.5)
	initial = .25
	frame(.5)
	target = 2
	frame(.5)
	now = now.Add(100 * time.Millisecond)
	frame(1.25)
	key = "fresh"
	frame(.25)
	duration = 0
	frame(2)
	show = false
	renderTestFrame(host)
	show, duration, initial = true, 200*time.Millisecond, 0
	frame(0)
	now = now.Add(200 * time.Millisecond)
	frame(2)
}
