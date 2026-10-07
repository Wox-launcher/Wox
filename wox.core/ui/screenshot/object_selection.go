package screenshot

import (
	"context"
	"slices"
	"time"
)

const screenshotSelectionTransitionDuration = 101 * time.Millisecond
const screenshotObjectForegroundBudget = 168 * time.Millisecond

// screenshotSelectionTransition keeps visual interpolation separate from the rectangle committed by a click.
type screenshotSelectionTransition struct {
	from, target Rect
	started      time.Time
	presented    bool
}

// present retargets an interrupted transition from its current position, without delaying the first preview.
func (transition *screenshotSelectionTransition) present(target Rect, now time.Time) {
	if target == transition.target {
		return
	}
	current, _ := transition.sample(now)
	transition.from, transition.target = current, target
	transition.started = now
	if !transition.presented || current.Width < 2 || current.Height < 2 || target.Width < 2 || target.Height < 2 {
		transition.from = target
	}
}

// sample uses quadratic ease-out and stops requesting frames when the target is reached.
func (transition *screenshotSelectionTransition) sample(now time.Time) (Rect, bool) {
	if transition.from == transition.target {
		return transition.target, false
	}
	elapsed := float32(now.Sub(transition.started)) / float32(screenshotSelectionTransitionDuration)
	if elapsed >= 1 {
		return transition.target, false
	}
	elapsed = max(float32(0), elapsed)
	progress := 1 - (1-elapsed)*(1-elapsed)
	return Rect{
		X:      transition.from.X + (transition.target.X-transition.from.X)*progress,
		Y:      transition.from.Y + (transition.target.Y-transition.from.Y)*progress,
		Width:  transition.from.Width + (transition.target.Width-transition.from.Width)*progress,
		Height: transition.from.Height + (transition.target.Height-transition.from.Height)*progress,
	}, true
}

// screenshotObjectSelection stores only geometry, from the innermost control to its frozen window frame.
// Native provider objects stay on the query worker; generation rejects results from an obsolete pointer or session.
type screenshotObjectSelection struct {
	point      Point
	path       []Rect
	index      int
	explicit   bool
	generation uint64
	transition screenshotSelectionTransition
}

type screenshotObjectQuery func(context.Context, Point) []Rect

type screenshotObjectRequest struct {
	point      Point
	generation uint64
	ctx        context.Context
	started    time.Time
	refinement bool
}

// objectAtPointLocked returns the logical target even while its preview is still moving.
func (state *screenshotEditorOverlayState) objectAtPointLocked(point Point) Rect {
	selection := &state.objectSelection
	if selection.point == point && selection.index >= 0 && selection.index < len(selection.path) {
		return selection.path[selection.index]
	}
	return state.selectionFallbackAtPoint(point)
}

// updateObjectSelectionLocked keeps a valid logical fallback and coalesces native requests without flashing it between controls.
func (state *screenshotEditorOverlayState) updateObjectSelectionLocked(point Point) {
	selection := &state.objectSelection
	if selection.generation != 0 && selection.point == point {
		return
	}
	fallback := state.selectionFallbackAtPoint(point)
	sameTarget := len(selection.path) > 0 && selection.path[len(selection.path)-1] == fallback
	selection.point = point
	selection.generation++
	selection.explicit = false
	if !sameTarget ||
		!screenshotEditorRectContains(selection.path[0], point) {
		selection.path = nil
		selection.index = 0
		if fallback.Width >= 2 && fallback.Height >= 2 {
			selection.path = []Rect{fallback}
		}
	}
	// Keep the previous preview during a short native lookup in the same window.
	// Presenting the window fallback on every row crossing makes the border expand and collapse continuously.
	if !sameTarget || state.objectQuery == nil || len(selection.path) > 1 {
		selection.transition.present(state.objectAtPointLocked(point), time.Now())
	}
	if state.objectCancel != nil {
		state.objectCancel()
	}
	// Empty desktop needs no accessibility work, and must cancel any pending result from the previous window.
	if state.objectQuery == nil || state.objectRequests == nil || state.windowAtPoint(point).Width < 2 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.objectCancel = cancel
	request := screenshotObjectRequest{point: point, generation: selection.generation, ctx: ctx, started: time.Now()}
	select {
	case <-state.objectRequests:
	default:
	}
	state.objectRequests <- request
}

// applyObjectPathLocked clips native geometry and preserves a user-selected ancestor when refinement adds children.
func (state *screenshotEditorOverlayState) applyObjectPathLocked(request screenshotObjectRequest, path []Rect) bool {
	selection := &state.objectSelection
	if request.ctx.Err() != nil || request.generation != selection.generation || state.hasSelection ||
		state.dragging || !state.pointerInside || state.objectClosed || len(path) == 0 {
		return false
	}
	window := state.windowAtPoint(request.point)
	if window.Width < 2 || window.Height < 2 {
		return false
	}
	bounded := make([]Rect, 0, len(path)+1)
	for _, rect := range path {
		left, top := max(rect.X, window.X), max(rect.Y, window.Y)
		right, bottom := min(rect.X+rect.Width, window.X+window.Width), min(rect.Y+rect.Height, window.Y+window.Height)
		rect = Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
		if rect.Width < 2 || rect.Height < 2 || !screenshotEditorRectContains(rect, request.point) || slices.Contains(bounded, rect) {
			continue
		}
		if len(bounded) > 0 && !screenshotRectEncloses(rect, bounded[len(bounded)-1]) {
			continue
		}
		bounded = append(bounded, rect)
	}
	if !slices.Contains(bounded, window) {
		bounded = append(bounded, window)
	}
	if slices.Equal(bounded, selection.path) {
		return false
	}
	if request.refinement {
		// Refinement may add descendants or containers, but must not discard a previously established path.
		matched := 0
		for _, rect := range bounded {
			if matched < len(selection.path) && rect == selection.path[matched] {
				matched++
			}
		}
		if matched != len(selection.path) || len(bounded) <= len(selection.path) {
			return false
		}
	}
	previous := state.objectAtPointLocked(request.point)
	index := 0
	if selection.explicit {
		index = slices.Index(bounded, previous)
		if index < 0 {
			// A late provider result must not change an ancestor explicitly chosen with the wheel.
			return false
		}
	}
	selection.path, selection.index = bounded, index
	selection.transition.present(state.objectAtPointLocked(request.point), time.Now())
	return true
}

func screenshotRectEncloses(outer, inner Rect) bool {
	return outer.X <= inner.X && outer.Y <= inner.Y && outer.X+outer.Width >= inner.X+inner.Width && outer.Y+outer.Height >= inner.Y+inner.Height
}

// startObjectSelection submits foreground hits immediately; only expensive refinement waits for pointer dwell.
// One worker consumes the latest pointer, and closing never waits for a foreign application's IPC.
func (state *screenshotEditorOverlayState) startObjectSelection() {
	if state.objectQuery == nil || state.hasSelection {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.mu.Lock()
	state.objectStop = cancel
	state.objectRequests = make(chan screenshotObjectRequest, 1)
	requests := state.objectRequests
	state.mu.Unlock()
	go func() {
		// Native providers release session caches on their worker after outstanding IPC returns.
		if state.objectQueryClose != nil {
			defer state.objectQueryClose()
		}
		for {
			var request screenshotObjectRequest
			select {
			case <-ctx.Done():
				return
			case request = <-requests:
			}
			for _, budget := range []time.Duration{screenshotObjectForegroundBudget, 1500 * time.Millisecond} {
				if ctx.Err() != nil || request.ctx.Err() != nil {
					break
				}
				request.refinement = budget > screenshotObjectForegroundBudget
				if request.refinement {
					timer := time.NewTimer(max(time.Duration(0), 80*time.Millisecond-time.Since(request.started)))
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-request.ctx.Done():
						timer.Stop()
						continue
					case <-timer.C:
					}
				}
				queryContext, stop := context.WithTimeout(request.ctx, budget)
				path := state.objectQuery(queryContext, request.point)
				stop()
				state.mu.Lock()
				if len(path) == 0 && !request.refinement {
					path = []Rect{state.windowAtPoint(request.point)}
				}
				changed := state.applyObjectPathLocked(request, path)
				// A failed initial lookup still resolves the pending preview to the logical fallback.
				if !changed && !request.refinement && request.ctx.Err() == nil && request.generation == state.objectSelection.generation && state.pointerInside && !state.dragging && !state.hasSelection && !state.objectClosed {
					state.objectSelection.transition.present(state.objectAtPointLocked(request.point), time.Now())
					changed = true
				}
				window := state.window
				state.mu.Unlock()
				if changed && window != nil {
					_ = window.Invalidate()
				}
			}
		}
	}()
}

// stopObjectSelection invalidates all outstanding results before the editor window is released.
func (state *screenshotEditorOverlayState) stopObjectSelection() {
	state.mu.Lock()
	state.objectClosed = true
	state.objectSelection.generation++
	if state.objectCancel != nil {
		state.objectCancel()
	}
	if state.objectStop != nil {
		state.objectStop()
	}
	state.mu.Unlock()
}
