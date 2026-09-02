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
	AnimationEaseInOutCubic
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
		if ctx.dynamic != nil {
			ctx.dynamic.animations = append(ctx.dynamic.animations, animationDependency{key: w.Key, kind: animationDependencyFloat, value: value})
		}
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

// LoopAnimation rebuilds its child with a repeating normalized progress value.
type LoopAnimation struct {
	Key      Key
	Duration time.Duration
	Paused   bool
	Builder  func(float32) Widget
}

func (w LoopAnimation) layout(ctx context, available constraints) *node {
	progress := float32(0)
	if w.Key != "" && w.Duration > 0 {
		progress = ctx.animation.loopValue(w.Key, w.Duration, w.Paused)
		if ctx.dynamic != nil {
			ctx.dynamic.animations = append(ctx.dynamic.animations, animationDependency{key: w.Key, kind: animationDependencyLoop, value: progress})
		}
	}
	if w.Builder == nil {
		return &node{}
	}
	child := w.Builder(progress)
	if child == nil {
		return &node{}
	}
	return child.layout(ctx, available)
}

// FrameAnimation rebuilds its child only when the current GIF-style frame index changes.
type FrameAnimation struct {
	Key     Key
	Delays  []time.Duration
	Paused  bool
	Builder func(int) Widget
}

func (w FrameAnimation) layout(ctx context, available constraints) *node {
	index := 0
	if w.Key != "" && len(w.Delays) > 1 {
		index = ctx.animation.frameIndex(w.Key, w.Delays, w.Paused)
		if ctx.dynamic != nil {
			ctx.dynamic.animations = append(ctx.dynamic.animations, animationDependency{key: w.Key, kind: animationDependencyFrame, value: float32(index)})
		}
	}
	if w.Builder == nil {
		return &node{}
	}
	child := w.Builder(index)
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

func (f animationFrame) loopValue(key Key, duration time.Duration, paused bool) float32 {
	if f.host == nil {
		return 0
	}
	return f.host.loopValue(f, key, duration, paused)
}

func (f animationFrame) frameIndex(key Key, delays []time.Duration, paused bool) int {
	if f.host == nil {
		return 0
	}
	return f.host.frameIndex(f, key, delays, paused)
}

func (f animationFrame) observe(dependency animationDependency) (float32, bool) {
	if f.host == nil {
		return dependency.value, true
	}
	return f.host.observe(f, dependency)
}

type floatAnimation struct {
	start      float32
	target     float32
	startedAt  time.Time
	duration   time.Duration
	curve      AnimationCurve
	lastSeenAt uint64
}

type loopAnimation struct {
	startedAt  time.Time
	duration   time.Duration
	pausedAt   time.Time
	paused     bool
	lastSeenAt uint64
	delays     []time.Duration
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
	if curve == AnimationEaseInOutCubic {
		if progress < 0.5 {
			return 4 * progress * progress * progress
		}
		shifted := -2*progress + 2
		return 1 - shifted*shifted*shifted/2
	}
	if curve != AnimationEaseOutBack {
		return progress
	}
	const overshoot = float32(1.70158)
	shifted := progress - 1
	return 1 + (overshoot+1)*shifted*shifted*shifted + overshoot*shifted*shifted
}

type animationFrameScheduler interface {
	RequestAnimationFrame() error
	StopAnimationFrames() error
}

type animationHost struct {
	mu           sync.Mutex
	values       map[Key]*floatAnimation
	loops        map[Key]*loopAnimation
	generation   uint64
	active       bool
	timer        *time.Timer
	vsyncPending bool
}

// observe keeps an animation alive when a cached Boundary skips its widget layout.
func (h *animationHost) observe(frame animationFrame, dependency animationDependency) (float32, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch dependency.kind {
	case animationDependencyFloat:
		animation := h.values[dependency.key]
		if animation == nil {
			return 0, false
		}
		animation.lastSeenAt = frame.generation
		value := animation.valueAt(frame.now)
		if value != animation.target {
			h.active = true
		}
		return value, true
	case animationDependencyLoop:
		animation := h.loops[dependency.key]
		if animation == nil || animation.duration <= 0 {
			return 0, false
		}
		animation.lastSeenAt = frame.generation
		now := frame.now
		if animation.paused {
			now = animation.pausedAt
		} else {
			h.active = true
		}
		return float32(now.Sub(animation.startedAt)%animation.duration) / float32(animation.duration), true
	case animationDependencyFrame:
		animation := h.loops[dependency.key]
		if animation == nil || animation.duration <= 0 {
			return 0, false
		}
		animation.lastSeenAt = frame.generation
		now := frame.now
		if animation.paused {
			now = animation.pausedAt
		} else {
			h.active = true
		}
		elapsed := now.Sub(animation.startedAt) % animation.duration
		return float32(frameIndexAtElapsed(animation.delays, elapsed)), true
	default:
		return 0, false
	}
}

// beginFrame records one shared timestamp so every animation in the tree advances together.
func (h *animationHost) beginFrame() animationFrame {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.generation++
	h.active = false
	h.vsyncPending = false
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

func (h *animationHost) loopValue(frame animationFrame, key Key, duration time.Duration, paused bool) float32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	animation := h.ensureLoopLocked(frame, key, duration, paused, nil)
	now := frame.now
	if paused {
		now = animation.pausedAt
	}
	return float32(now.Sub(animation.startedAt)%duration) / float32(duration)
}

func (h *animationHost) frameIndex(frame animationFrame, key Key, delays []time.Duration, paused bool) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	duration := sumFrameDelays(delays)
	if duration <= 0 || len(delays) <= 1 {
		return 0
	}
	animation := h.ensureLoopLocked(frame, key, duration, paused, delays)
	now := frame.now
	if paused {
		now = animation.pausedAt
	}
	return frameIndexAtElapsed(delays, now.Sub(animation.startedAt)%duration)
}

// ensureLoopLocked creates or updates one repeating timeline used by both progress loops and GIF frames.
func (h *animationHost) ensureLoopLocked(frame animationFrame, key Key, duration time.Duration, paused bool, delays []time.Duration) *loopAnimation {
	if h.loops == nil {
		h.loops = map[Key]*loopAnimation{}
	}
	animation := h.loops[key]
	if animation == nil || animation.duration != duration {
		animation = &loopAnimation{startedAt: frame.now, duration: duration, paused: paused, delays: delays}
		if paused {
			animation.pausedAt = frame.now
		}
		h.loops[key] = animation
	} else if animation.paused != paused {
		if paused {
			animation.pausedAt = frame.now
		} else {
			animation.startedAt = animation.startedAt.Add(frame.now.Sub(animation.pausedAt))
		}
		animation.paused = paused
	}
	if delays != nil {
		animation.delays = delays
	}
	animation.lastSeenAt = frame.generation
	if !paused {
		h.active = true
	}
	return animation
}

func sumFrameDelays(delays []time.Duration) time.Duration {
	var total time.Duration
	for _, delay := range delays {
		total += delay
	}
	return total
}

func frameIndexAtElapsed(delays []time.Duration, elapsed time.Duration) int {
	if len(delays) == 0 {
		return 0
	}
	if elapsed < 0 {
		elapsed = 0
	}
	var cursor time.Duration
	for index, delay := range delays {
		cursor += delay
		if elapsed < cursor {
			return index
		}
	}
	return len(delays) - 1
}

// endFrame drops absent animations and requests the next frame only while a value is moving.
func (h *animationHost) endFrame(frame animationFrame, invalidate func(), vsync animationFrameScheduler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, animation := range h.values {
		if animation.lastSeenAt != frame.generation {
			delete(h.values, key)
		}
	}
	for key, animation := range h.loops {
		if animation.lastSeenAt != frame.generation {
			delete(h.loops, key)
		}
	}
	if !h.active {
		if h.timer != nil {
			h.timer.Stop()
			h.timer = nil
		}
		h.vsyncPending = false
		if vsync != nil {
			_ = vsync.StopAnimationFrames()
		}
		return
	}
	if h.timer != nil || h.vsyncPending {
		return
	}
	if vsync != nil {
		h.vsyncPending = true
		h.mu.Unlock()
		err := vsync.RequestAnimationFrame()
		h.mu.Lock()
		if err != nil {
			h.vsyncPending = false
		} else {
			return
		}
	}
	var timer *time.Timer
	timer = time.AfterFunc(animationFrameInterval, func() {
		h.mu.Lock()
		if h.timer != timer {
			h.mu.Unlock()
			return
		}
		h.timer = nil
		h.mu.Unlock()
		if invalidate != nil {
			invalidate()
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
	h.vsyncPending = false
	h.values = nil
	h.loops = nil
	h.active = false
}
