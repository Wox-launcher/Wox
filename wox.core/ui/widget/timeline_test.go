package widget

import (
	"testing"
	"time"
)

// TestTimelinePlaybackControls covers producer echoes, pause/resume, completion and replay.
func TestTimelinePlaybackControls(t *testing.T) {
	h := &animationHost{}
	now := time.Unix(100, 0)
	frame := animationFrame{host: h, generation: 1, now: now}
	w := TimelineAnimation{Key: "walk", PlaybackID: "one", Duration: time.Second}
	check := func(offset, want time.Duration, active bool) {
		t.Helper()
		frame.now = now.Add(offset)
		got, running := h.timeline(frame, w)
		if got != want || running != active {
			t.Fatalf("at %s: %s/%v want %s/%v", offset, got, running, want, active)
		}
	}
	check(0, 0, true)
	w.Elapsed = 900 * time.Millisecond
	check(250*time.Millisecond, 250*time.Millisecond, true)
	w.Paused = true
	check(300*time.Millisecond, 300*time.Millisecond, false)
	check(time.Second, 300*time.Millisecond, false)
	w.Paused = false
	check(2*time.Second, 300*time.Millisecond, true)
	check(3*time.Second, time.Second, false)
	w.PlaybackID, w.Elapsed = "two", 100*time.Millisecond
	check(4*time.Second, 100*time.Millisecond, true)
	w.Loop, w.Elapsed, w.Step = true, 0, 200*time.Millisecond
	check(5*time.Second, 0, true)
	check(6350*time.Millisecond, 200*time.Millisecond, true)
	dependency := animationDependency{key: w.Key, kind: animationDependencyTimeline}
	frame.generation++
	value, ok := h.observe(frame, dependency)
	if !ok || value != .2 {
		t.Fatal("cached boundary lost its timeline")
	}
	h.clock = func() time.Time { return frame.now }
	h.endFrame(animationFrame{generation: frame.generation + 1}, nil, nil)
	if len(h.timelines) != 0 {
		t.Fatal("unmounted clock retained playback state")
	}
}

// TestTimelineFractionalEndpoint keeps finite deadlines exact despite a coarser display step.
func TestTimelineFractionalEndpoint(t *testing.T) {
	start := time.Unix(100, 0)
	clock := clockAnimation{startedAt: start, duration: 5500 * time.Millisecond, step: time.Second}
	for _, tc := range []struct {
		after, want time.Duration
		active      bool
	}{
		{5499 * time.Millisecond, 5 * time.Second, true},
		{5500 * time.Millisecond, 5500 * time.Millisecond, false},
		{time.Hour, 5500 * time.Millisecond, false},
	} {
		got, active := clock.sample(start.Add(tc.after))
		if got != tc.want || active != tc.active {
			t.Fatalf("at %s: %s/%v, want %s/%v", tc.after, got, active, tc.want, tc.active)
		}
	}
}
