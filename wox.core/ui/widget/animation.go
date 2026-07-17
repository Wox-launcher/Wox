package widget

import (
	"sync"
	"time"
)

const animationFrameInterval = time.Second / 60

// AnimationCurve selects how an AnimatedFloat moves between values.
type AnimationCurve uint8

const (
	AnimationLinear AnimationCurve = iota
	AnimationEaseOutBack
)

// AnimatedFloat retains a keyed numeric value and rebuilds its child while the value changes.
type AnimatedFloat struct {
	Key      Key
	Target   float32
	Duration time.Duration
	Curve    AnimationCurve
	Builder  func(float32) Widget
}

func (w AnimatedFloat) layout(ctx context, available constraints) *node {
	value := w.Target
	if w.Key != "" && w.Duration > 0 {
		value = ctx.animation.value(w.Key, w.Target, w.Duration, w.Curve)
	}
	if w.Builder == nil {
		return &node{}
	}
	child := w.Builder(value)
	if child == nil {
		return &node{}
	}
	return child.layout(ctx, available)
}

type animationFrame struct {
	host       *animationHost
	generation uint64
	now        time.Time
}

func (f animationFrame) value(key Key, target float32, duration time.Duration, curve AnimationCurve) float32 {
	if f.host == nil {
		return target
	}
	return f.host.value(f, key, target, duration, curve)
}

type floatAnimation struct {
	start      float32
	target     float32
	startedAt  time.Time
	duration   time.Duration
	curve      AnimationCurve
	lastSeenAt uint64
}

// valueAt resolves the current value without mutating the animation timeline.
func (a *floatAnimation) valueAt(now time.Time) float32 {
	if a.start == a.target || a.duration <= 0 {
		return a.target
	}
	progress := float32(now.Sub(a.startedAt)) / float32(a.duration)
	if progress <= 0 {
		return a.start
	}
	if progress >= 1 {
		return a.target
	}
	progress = transformAnimationProgress(progress, a.curve)
	return a.start + (a.target-a.start)*progress
}

// transformAnimationProgress applies the selected timing curve to normalized time.
func transformAnimationProgress(progress float32, curve AnimationCurve) float32 {
	if curve != AnimationEaseOutBack {
		return progress
	}
	const overshoot = float32(1.70158)
	shifted := progress - 1
	return 1 + (overshoot+1)*shifted*shifted*shifted + overshoot*shifted*shifted
}

type animationHost struct {
	mu         sync.Mutex
	values     map[Key]*floatAnimation
	generation uint64
	active     bool
	timer      *time.Timer
	window     HostServices
}

// beginFrame records one shared timestamp so every animation in the tree advances together.
func (h *animationHost) beginFrame(window HostServices) animationFrame {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.generation++
	h.active = false
	h.window = window
	return animationFrame{host: h, generation: h.generation, now: time.Now()}
}

// value preserves continuity when an in-flight animation receives a new target.
func (h *animationHost) value(frame animationFrame, key Key, target float32, duration time.Duration, curve AnimationCurve) float32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.values == nil {
		h.values = map[Key]*floatAnimation{}
	}
	animation := h.values[key]
	if animation == nil {
		animation = &floatAnimation{start: target, target: target, startedAt: frame.now, duration: duration, curve: curve}
		h.values[key] = animation
	}
	current := animation.valueAt(frame.now)
	if animation.target != target {
		animation.start = current
		animation.target = target
		animation.startedAt = frame.now
		animation.duration = duration
		animation.curve = curve
		current = animation.start
	}
	animation.lastSeenAt = frame.generation
	if current != animation.target {
		h.active = true
	}
	return animation.valueAt(frame.now)
}

// endFrame drops absent animations and requests the next frame only while a value is moving.
func (h *animationHost) endFrame(frame animationFrame) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, animation := range h.values {
		if animation.lastSeenAt != frame.generation {
			delete(h.values, key)
		}
	}
	if !h.active {
		if h.timer != nil {
			h.timer.Stop()
			h.timer = nil
		}
		return
	}
	if h.timer != nil {
		return
	}
	var timer *time.Timer
	timer = time.AfterFunc(animationFrameInterval, func() {
		h.mu.Lock()
		if h.timer != timer {
			h.mu.Unlock()
			return
		}
		h.timer = nil
		window := h.window
		h.mu.Unlock()
		if window != nil {
			_ = window.Invalidate()
		}
	})
	h.timer = timer
}

// reset cancels pending animation work when the host has no widget tree to render.
func (h *animationHost) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	h.values = nil
	h.active = false
}
