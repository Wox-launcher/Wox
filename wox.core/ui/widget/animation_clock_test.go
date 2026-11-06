package widget

import (
	"testing"
	"time"

	ui "wox/ui/runtime"
)

type clockTestProps struct {
	Target float32
	Sample *float32
	Builds *int
}

func (p clockTestProps) Equal(other clockTestProps) bool { return p == other }

// TestAnimationClock drives cached, retargeted and removed channels without sleeps.
func TestAnimationClock(t *testing.T) {
	now := time.Unix(1, 0)
	target, sample := float32(10), float32(0)
	builds, clockReads := 0, 0
	show := true
	host := NewHost(func(ui.FrameInfo) Widget {
		if !show {
			return Container{}
		}
		return Boundary[clockTestProps]{Key: "clock-boundary", Props: clockTestProps{target, &sample, &builds}, Build: func(p clockTestProps) Widget {
			*p.Builds++
			return AnimatedFloat{Key: "value", Target: p.Target, Duration: time.Second, Builder: func(v float32) Widget {
				*p.Sample = v
				return Container{Width: v, Height: 20}
			}}
		}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	host.SetAnimationClock(func() time.Time { clockReads++; return now })
	frame := func(want float32) {
		t.Helper()
		reads := clockReads
		renderTestFrame(host)
		if sample != want || clockReads != reads+1 {
			t.Fatalf("sample=%g want=%g clock reads=%d", sample, want, clockReads-reads)
		}
		if host.animations.timer != nil || host.animations.vsyncPending {
			t.Fatal("manual clock scheduled a real frame")
		}
	}
	frame(10)
	target = 30
	frame(10)
	now = now.Add(500 * time.Millisecond)
	frame(20)
	target = 40
	frame(20)
	now = now.Add(500 * time.Millisecond)
	frame(30)
	now = now.Add(500 * time.Millisecond)
	frame(40)
	previous := builds
	frame(40)
	if builds != previous {
		t.Fatal("settled animation did not cache")
	}
	show = false
	renderTestFrame(host)
	if len(host.animations.values) != 0 {
		t.Fatal("removed animation retained")
	}
	show, target = true, 60
	frame(60)
	host.SetAnimationClock(nil)
	if host.animations.clock != nil || len(host.animations.values) != 0 {
		t.Fatal("clock reset retained timeline")
	}
}

// TestLoopClockLifecycle verifies phase retention, pause, fresh identity, period
// changes and removal using caller-driven frames instead of elapsed real time.
func TestLoopClockLifecycle(t *testing.T) {
	now := time.Unix(1, 0)
	duration := 1200 * time.Millisecond
	key, paused, show := Key("loop"), false, true
	var sample float32
	host := NewHost(func(ui.FrameInfo) Widget {
		if !show {
			return Container{}
		}
		return LoopAnimation{Key: key, Duration: duration, Paused: paused, Builder: func(progress float32) Widget {
			sample = progress
			return Container{Width: 20, Height: 30}
		}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	host.SetAnimationClock(func() time.Time { return now })
	frame := func(want float32) {
		t.Helper()
		renderTestFrame(host)
		if sample != want {
			t.Fatalf("phase %g want %g", sample, want)
		}
	}
	frame(0)
	now = now.Add(300 * time.Millisecond)
	frame(.25)
	paused = true
	frame(.25)
	now = now.Add(time.Second)
	frame(.25)
	paused = false
	frame(.25)
	now = now.Add(300 * time.Millisecond)
	frame(.5)
	duration = time.Second
	frame(0)
	now = now.Add(500 * time.Millisecond)
	frame(.5)
	key = "new-loop"
	frame(0)
	show = false
	renderTestFrame(host)
	if len(host.animations.loops) != 0 || host.animations.active || host.animations.timer != nil {
		t.Fatal("removed loop still scheduled")
	}
	show = true
	frame(0)
}

// TestUncachedLoopSchedulesLocalDamage covers looping widgets outside Boundary.
func TestUncachedLoopSchedulesLocalDamage(t *testing.T) {
	services := &fakeHostServices{invalidatedRects: make(chan ui.Rect, 1)}
	host := NewHost(func(ui.FrameInfo) Widget {
		return LoopAnimation{Key: "local", Duration: time.Second, Builder: func(float32) Widget { return Container{Width: 20, Height: 30} }}
	})
	host.AttachServices(services)
	defer host.Dispose()
	renderTestFrame(host)
	select {
	case rect := <-services.invalidatedRects:
		if rect != (ui.Rect{Width: 20, Height: 30}) {
			t.Fatalf("loop damage %+v", rect)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("loop did not request local damage")
	}
}
