package screenshot

import (
	"context"
	"image"
	"slices"
	"testing"
	"time"
)

// TestScreenshotSelectionTransitionRetarget checks first presentation, interrupted movement, and an exact final frame without sleep.
func TestScreenshotSelectionTransitionRetarget(t *testing.T) {
	start := time.Unix(1, 0)
	first := Rect{X: -200, Y: 40, Width: 300, Height: 200}
	second := Rect{X: 50, Y: 80, Width: 40, Height: 30}
	third := Rect{X: 400, Y: 100, Width: 120, Height: 80}
	var transition screenshotSelectionTransition
	transition.present(first, start)
	if rect, running := transition.sample(start); rect != first || running {
		t.Fatal("first preview must appear immediately")
	}
	transition.presented = true
	transition.present(second, start)
	middle, running := transition.sample(start.Add(screenshotSelectionTransitionDuration / 2))
	if !running || middle.X <= first.X || middle.X >= second.X {
		t.Fatalf("invalid intermediate frame: %+v", middle)
	}
	transition.present(third, start.Add(screenshotSelectionTransitionDuration/2))
	if rect, _ := transition.sample(start.Add(screenshotSelectionTransitionDuration / 2)); rect != middle {
		t.Fatal("interruption jumped to an obsolete target")
	}
	if rect, running := transition.sample(start.Add(screenshotSelectionTransitionDuration * 2)); rect != third || running {
		t.Fatal("animation did not settle exactly")
	}
	transition.present(Rect{}, start.Add(time.Second))
	if rect, running := transition.sample(start.Add(time.Second)); rect != (Rect{}) || running {
		t.Fatal("leaving objects should clear immediately")
	}
}

// TestScreenshotSelectionBeforeFirstFrame avoids animating from a fallback that the renderer has never presented.
func TestScreenshotSelectionBeforeFirstFrame(t *testing.T) {
	start := time.Unix(1, 0)
	var transition screenshotSelectionTransition
	transition.present(Rect{Width: 800, Height: 600}, start)
	leaf := Rect{X: 40, Y: 60, Width: 100, Height: 20}
	transition.present(leaf, start.Add(time.Millisecond))
	if rect, animating := transition.sample(start.Add(time.Millisecond)); rect != leaf || animating {
		t.Fatal("first rendered native hit animated from an unseen window fallback")
	}
}

// TestScreenshotObjectSelectionHierarchy checks clipped nonzero origins, duplicate containers, explicit ancestor preservation, and committed geometry.
func TestScreenshotObjectSelectionHierarchy(t *testing.T) {
	window := Rect{X: 10, Y: 20, Width: 200, Height: 160}
	container := Rect{X: 20, Y: 30, Width: 100, Height: 80}
	leaf := Rect{X: 30, Y: 40, Width: 20, Height: 15}
	point := Point{X: 35, Y: 45}
	state := &screenshotEditorOverlayState{windowCandidates: []Rect{window}, pointerInside: true, pointerPosition: point, frameSize: Size{Width: 500, Height: 400}, result: make(chan screenshotEditorOverlayOutcome, 1)}
	state.updateObjectSelectionLocked(point)
	request := screenshotObjectRequest{point: point, generation: state.objectSelection.generation, ctx: context.Background()}
	if !state.applyObjectPathLocked(request, []Rect{leaf, leaf, container, {X: -10, Y: 0, Width: 300, Height: 200}}) {
		t.Fatal("native hierarchy was not applied")
	}
	if !slices.Equal(state.objectSelection.path, []Rect{leaf, container, window}) {
		t.Fatalf("path = %+v", state.objectSelection.path)
	}
	if state.applyObjectPathLocked(request, nil) || state.objectAtPointLocked(point) != leaf {
		t.Fatal("a timed-out refinement discarded the previous object")
	}
	state.pointer(PointerEvent{Kind: PointerScroll, Position: point, Scroll: Point{Y: 1}})
	if state.objectAtPointLocked(point) != container {
		t.Fatal("wheel did not select parent")
	}
	inner := Rect{X: 32, Y: 42, Width: 8, Height: 8}
	state.applyObjectPathLocked(request, []Rect{inner, leaf, container, window})
	if state.objectAtPointLocked(point) != container {
		t.Fatal("refinement changed explicit selection")
	}
	// The transition has not finished, but click must commit the actual container, not the in-between visual frame.
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
	if state.applyObjectPathLocked(request, []Rect{inner, window}) {
		t.Fatal("query changed a pressed selection")
	}
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: point})
	if state.selection != container || !state.hasSelection {
		t.Fatalf("committed crop = %+v", state.selection)
	}
}

// TestScreenshotObjectSelectionRejectsStaleResults covers pointer changes, cancelled providers, leaving, and disposal.
func TestScreenshotObjectSelectionRejectsStaleResults(t *testing.T) {
	window := Rect{Width: 400, Height: 300}
	state := &screenshotEditorOverlayState{windowCandidates: []Rect{window}, pointerInside: true}
	point := Point{X: 20, Y: 20}
	state.updateObjectSelectionLocked(point)
	request := screenshotObjectRequest{point: point, generation: state.objectSelection.generation, ctx: context.Background()}
	state.updateObjectSelectionLocked(Point{X: 100, Y: 100})
	if state.applyObjectPathLocked(request, []Rect{window}) {
		t.Fatal("obsolete pointer result was applied")
	}
	request.point, request.generation = state.objectSelection.point, state.objectSelection.generation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request.ctx = ctx
	if state.applyObjectPathLocked(request, []Rect{window}) {
		t.Fatal("cancelled result was applied")
	}
	request.ctx = context.Background()
	state.pointerInside = false
	if state.applyObjectPathLocked(request, []Rect{window}) {
		t.Fatal("result was applied after pointer left")
	}
	state.pointerInside = true
	state.stopObjectSelection()
	if state.applyObjectPathLocked(request, []Rect{window}) {
		t.Fatal("result was applied after session close")
	}
}

// TestScreenshotObjectSelectionDesktopFallback cancels stale window work and commits a display without requesting window pixels.
func TestScreenshotObjectSelectionDesktopFallback(t *testing.T) {
	display := Rect{X: 400, Y: 100, Width: 500, Height: 400}
	window := Rect{X: 420, Y: 120, Width: 80, Height: 60}
	state := &screenshotEditorOverlayState{
		frameSize: Size{Width: 900, Height: 500}, windowCandidates: []Rect{window}, displayCandidates: []Rect{display},
		pointerInside: true, objectRequests: make(chan screenshotObjectRequest, 1),
		objectQuery:   func(context.Context, Point) []Rect { t.Fatal("desktop fallback queried accessibility"); return nil },
		captureWindow: func(Rect) (*image.RGBA, error) { t.Error("desktop fallback requested window pixels"); return nil, nil },
		result:        make(chan screenshotEditorOverlayOutcome, 1),
	}
	state.updateObjectSelectionLocked(Point{X: 430, Y: 130})
	previous := <-state.objectRequests
	point := Point{X: 600, Y: 300}
	state.updateObjectSelectionLocked(point)
	if previous.ctx.Err() == nil || len(state.objectRequests) != 0 {
		t.Fatal("desktop fallback did not cancel window work or unnecessarily submitted a query")
	}
	if state.applyObjectPathLocked(previous, []Rect{window}) || state.objectAtPointLocked(point) != display {
		t.Fatal("stale window result replaced the display fallback")
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: point})
	if !state.hasSelection || state.selection != display {
		t.Fatalf("display click committed %+v", state.selection)
	}
}

// TestScreenshotObjectQueryCoalesces checks immediate foreground hits, latest-point coalescing during IPC, and cancellation on close.
func TestScreenshotObjectQueryCoalesces(t *testing.T) {
	calls := make(chan Point, 8)
	state := &screenshotEditorOverlayState{
		windowCandidates: []Rect{{Width: 400, Height: 300}}, pointerInside: true,
		objectQuery: func(ctx context.Context, point Point) []Rect {
			calls <- point
			<-ctx.Done()
			return nil
		},
	}
	state.startObjectSelection()
	defer state.stopObjectSelection()
	state.mu.Lock()
	state.updateObjectSelectionLocked(Point{X: 20, Y: 20})
	state.mu.Unlock()
	select {
	case point := <-calls:
		if point != (Point{X: 20, Y: 20}) {
			t.Fatalf("initial query point = %+v", point)
		}
	case <-time.After(60 * time.Millisecond):
		t.Fatal("foreground lookup incorrectly waited for pointer dwell")
	}
	state.mu.Lock()
	for _, point := range []Point{{X: 40, Y: 40}, {X: 60, Y: 60}} {
		state.updateObjectSelectionLocked(point)
	}
	state.mu.Unlock()
	select {
	case point := <-calls:
		// A request already being dispatched can observe the intermediate point; it is cancelled before publication.
		if point == (Point{X: 40, Y: 40}) {
			select {
			case point = <-calls:
			case <-time.After(time.Second):
				t.Fatal("latest query was lost after an intermediate dispatch")
			}
		}
		if point != (Point{X: 60, Y: 60}) {
			t.Fatalf("latest query point = %+v", point)
		}
	case <-time.After(time.Second):
		t.Fatal("latest query never started")
	}
	state.stopObjectSelection()
	select {
	case point := <-calls:
		t.Fatalf("closing submitted another query: %+v", point)
	case <-time.After(30 * time.Millisecond):
	}
}

// TestScreenshotObjectSelectionCrossing does not animate through the window fallback between adjacent controls.
func TestScreenshotObjectSelectionCrossing(t *testing.T) {
	window := Rect{Width: 400, Height: 300}
	first, second := Rect{X: 20, Y: 20, Width: 100, Height: 20}, Rect{X: 20, Y: 40, Width: 100, Height: 20}
	state := &screenshotEditorOverlayState{windowCandidates: []Rect{window}, pointerInside: true, objectQuery: func(context.Context, Point) []Rect { return nil }}
	state.updateObjectSelectionLocked(Point{X: 30, Y: 30})
	request := screenshotObjectRequest{point: state.objectSelection.point, generation: state.objectSelection.generation, ctx: context.Background()}
	state.applyObjectPathLocked(request, []Rect{first, window})
	state.updateObjectSelectionLocked(Point{X: 30, Y: 50})
	if state.objectSelection.transition.target != first || state.objectAtPointLocked(state.objectSelection.point) != window {
		t.Fatal("crossing either expanded the visual preview or kept an invalid logical target")
	}
	request.point, request.generation = state.objectSelection.point, state.objectSelection.generation
	state.applyObjectPathLocked(request, []Rect{second, window})
	if state.objectSelection.transition.target != second {
		t.Fatal("foreground hit did not transition directly to the new control")
	}
	request.refinement = true
	if state.applyObjectPathLocked(request, []Rect{window}) || state.objectAtPointLocked(request.point) != second {
		t.Fatal("refinement regressed to the window")
	}
}

// TestScreenshotObjectRefinementDwell separates immediate foreground publication from delayed background work.
func TestScreenshotObjectRefinementDwell(t *testing.T) {
	calls := make(chan time.Time, 4)
	window := Rect{Width: 400, Height: 300}
	state := &screenshotEditorOverlayState{windowCandidates: []Rect{window}, pointerInside: true,
		objectQuery: func(context.Context, Point) []Rect { calls <- time.Now(); return []Rect{window} }}
	state.startObjectSelection()
	defer state.stopObjectSelection()
	pointerAt := time.Now()
	state.mu.Lock()
	state.updateObjectSelectionLocked(Point{X: 20, Y: 20})
	state.mu.Unlock()
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("foreground did not run")
	}
	select {
	case refinement := <-calls:
		if refinement.Sub(pointerAt) < 80*time.Millisecond {
			t.Fatal("refinement did not wait for pointer dwell")
		}
	case <-time.After(time.Second):
		t.Fatal("refinement did not run")
	}
}

// TestScreenshotObjectQueryCleanup waits for foreign IPC on the worker without blocking editor teardown.
func TestScreenshotObjectQueryCleanup(t *testing.T) {
	entered, release, closed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	state := &screenshotEditorOverlayState{
		windowCandidates: []Rect{{Width: 400, Height: 300}}, pointerInside: true,
		objectQuery: func(context.Context, Point) []Rect {
			close(entered)
			<-release
			return nil
		},
		objectQueryClose: func() { close(closed) },
	}
	state.startObjectSelection()
	defer state.stopObjectSelection()
	defer close(release)
	state.mu.Lock()
	state.updateObjectSelectionLocked(Point{X: 20, Y: 20})
	state.mu.Unlock()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("query worker did not start")
	}
	state.stopObjectSelection()
	select {
	case <-closed:
		t.Fatal("provider references were released while IPC was outstanding")
	default:
	}
	// Release the simulated provider before checking asynchronous cleanup.
	release <- struct{}{}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("query worker did not release its session cache")
	}
}
