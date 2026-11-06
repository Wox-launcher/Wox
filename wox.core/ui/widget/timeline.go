package widget

import (
	"time"

	ui "wox/ui/runtime"
)

// TimelineAnimation runs a finite, looping, or unbounded native clock. Elapsed
// initializes a new playback only; ordinary producer echoes never seek/restart it.
type TimelineAnimation struct {
	Key        Key
	PlaybackID string
	Duration   time.Duration
	Elapsed    time.Duration
	Step       time.Duration
	Loop       bool
	Paused     bool
	Builder    func(time.Duration) Widget
}

// layout participates in the shared animation clock and Boundary dependency tracking.
func (w TimelineAnimation) layout(ctx context, available constraints) *node {
	elapsed, active := w.Elapsed, false
	if w.Key != "" && ctx.animation.host != nil {
		elapsed, active = ctx.animation.host.timeline(ctx.animation, w)
		if ctx.dynamic != nil {
			ctx.dynamic.animations = append(ctx.dynamic.animations, animationDependency{key: w.Key, kind: animationDependencyTimeline, value: float32(elapsed.Seconds())})
		}
	}
	if w.Builder == nil {
		return &node{}
	}
	child := w.Builder(elapsed)
	if child == nil {
		return &node{}
	}
	result := child.layout(ctx, available)
	if active && ctx.damage != nil {
		ctx.damage.add(ui.Rect{}, result, true)
	}
	return result
}

type clockAnimation struct {
	playbackID          string
	startedAt, pausedAt time.Time
	duration, step      time.Duration
	loop, paused        bool
	lastSeenAt          uint64
}

// sample holds the final pose instead of scheduling frames after finite playback ends.
func (a *clockAnimation) sample(now time.Time) (time.Duration, bool) {
	if a.paused {
		now = a.pausedAt
	}
	elapsed := max(time.Duration(0), now.Sub(a.startedAt))
	active := !a.paused
	if a.duration > 0 {
		if a.loop {
			elapsed %= a.duration
		} else if elapsed >= a.duration {
			elapsed, active = a.duration, false
		}
	}
	// A finite endpoint may fall between ticks; do not truncate it or completion
	// observers could remain permanently below the duration after the clock stops.
	if a.step > 0 && (a.duration <= 0 || a.loop || elapsed < a.duration) {
		elapsed = elapsed / a.step * a.step
	}
	return elapsed, active
}

// timeline updates playback controls without changing the phase on unrelated builds.
func (h *animationHost) timeline(frame animationFrame, w TimelineAnimation) (time.Duration, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.timelines == nil {
		h.timelines = map[Key]*clockAnimation{}
	}
	a := h.timelines[w.Key]
	if a == nil || a.playbackID != w.PlaybackID || a.duration != w.Duration || a.loop != w.Loop {
		a = &clockAnimation{playbackID: w.PlaybackID, startedAt: frame.now.Add(-max(time.Duration(0), w.Elapsed)), duration: w.Duration, loop: w.Loop, paused: w.Paused, pausedAt: frame.now}
		h.timelines[w.Key] = a
	} else if a.paused != w.Paused {
		if w.Paused {
			a.pausedAt = frame.now
		} else {
			a.startedAt = a.startedAt.Add(frame.now.Sub(a.pausedAt))
		}
		a.paused = w.Paused
	}
	a.step, a.lastSeenAt = w.Step, frame.generation
	elapsed, active := a.sample(frame.now)
	h.active = h.active || active
	return elapsed, active
}
