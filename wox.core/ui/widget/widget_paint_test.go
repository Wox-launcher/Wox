package widget

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestRetainedPaintReusesBoundaryAndSkipsDescendants(t *testing.T) {
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Flex{Axis: Vertical, Children: []Widget{
			Text{Value: "stable", Style: woxui.TextStyle{Size: 12}},
			Boundary[boundaryTestProps]{Key: "paint-cache", Props: boundaryTestProps{Value: 1}, Build: func(boundaryTestProps) Widget {
				builds++
				return Flex{Axis: Vertical, Children: []Widget{
					Text{Value: "one", Style: woxui.TextStyle{Size: 12}},
					Text{Value: "two", Style: woxui.TextStyle{Size: 12}},
					Text{Value: "three", Style: woxui.TextStyle{Size: 12}},
				}}
			}},
		}}
	})
	services := &recordingHostServices{}
	host.AttachServices(services)

	first := &woxui.DisplayList{}
	first.AttachFrameMetricsID(1)
	host.Frame(first, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 100}})
	firstPaint := services.work.PaintVisits
	if builds != 1 || firstPaint < 4 || services.work.PaintSegmentReuses != 0 {
		t.Fatalf("first paint work = builds %d %+v", builds, services.work)
	}

	second := &woxui.DisplayList{}
	second.AttachFrameMetricsID(2)
	host.Frame(second, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 100}})
	if builds != 1 || services.work.PaintSegmentReuses == 0 {
		t.Fatalf("cached paint work = builds %d %+v", builds, services.work)
	}
	if services.work.PaintVisits >= firstPaint {
		t.Fatalf("cached paint visits = %d, want fewer than first-frame %d", services.work.PaintVisits, firstPaint)
	}
	if services.work.IdentityUpserts != 0 || services.work.A11yUpserts != 0 {
		t.Fatalf("unchanged frame upserts = identity %d a11y %d, want 0", services.work.IdentityUpserts, services.work.A11yUpserts)
	}
	if err := first.Compare(second); err != nil {
		t.Fatalf("retained paint changed the flattened command stream: %v", err)
	}
}

func TestBoundaryCanDisableRetainedPaintWithoutDisablingLayoutCache(t *testing.T) {
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{
			Key: "unretained-paint", Props: boundaryTestProps{}, DisableRetainedPaint: true,
			Build: func(boundaryTestProps) Widget {
				builds++
				return Container{Width: 20, Height: 20, Color: woxui.Color{A: 255}}
			},
		}
	})
	services := &recordingHostServices{}
	host.AttachServices(services)
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 20, Height: 20}}
	host.Frame(&woxui.DisplayList{}, frame)
	host.Frame(&woxui.DisplayList{}, frame)
	if builds != 1 {
		t.Fatalf("boundary builds = %d, want one cached layout build", builds)
	}
	if services.work.PaintSegmentReuses != 0 {
		t.Fatalf("paint segment reuses = %d, want immediate paint", services.work.PaintSegmentReuses)
	}
}

func TestRetainedPaintCaretRerecordsOnlyCaretBoundary(t *testing.T) {
	innerCache := &boundaryCache{}
	outerCache := &boundaryCache{}
	inner := &node{
		bounds: woxui.Rect{Width: 8, Height: 8},
		caretPaint: func(displayList *woxui.DisplayList, bounds woxui.Rect, _, caretVisible bool) {
			if caretVisible {
				displayList.FillRect(bounds, woxui.Color{A: 255})
			}
		},
		boundary: innerCache,
	}
	outer := &node{
		bounds: woxui.Rect{Width: 20, Height: 20},
		paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRect(bounds, woxui.Color{R: 255, A: 255})
		},
		children: []*node{inner},
		boundary: outerCache,
	}

	firstWork := &frameWorkCounters{}
	first := &woxui.DisplayList{}
	outer.draw(first, 0, 0, true, false, false, firstWork)
	if firstWork.paintSegmentReuses != 0 || innerCache.paint == nil || !innerCache.paint.Fingerprint.UsesCaret {
		t.Fatalf("first caret record = %+v paint %#v", firstWork, innerCache.paint)
	}
	visibleCount := first.CommandCount()
	publishedOuter := outerCache.paint

	secondWork := &frameWorkCounters{}
	second := &woxui.DisplayList{}
	outer.draw(second, 0, 0, false, false, false, secondWork)
	if secondWork.paintSegmentReuses != 1 {
		t.Fatalf("caret blink outer reuse = %d, want 1", secondWork.paintSegmentReuses)
	}
	if innerCache.paint == nil || innerCache.paint.Fingerprint.CaretVisible {
		t.Fatalf("caret boundary did not re-record hidden caret: %#v", innerCache.paint)
	}
	if visibleCount <= second.CommandCount() {
		t.Fatalf("hidden caret command count = %d, want fewer than visible %d", second.CommandCount(), visibleCount)
	}
	if outerCache.paint == nil || outerCache.paint == publishedOuter {
		t.Fatal("outer boundary should publish a new segment when a nested caret rerecords")
	}
	if first.CommandCount() != visibleCount {
		t.Fatalf("published first frame command count = %d, want unchanged %d", first.CommandCount(), visibleCount)
	}
}

func TestRetainedPaintRerecordsWhenFocusRingReappears(t *testing.T) {
	cache := &boundaryCache{}
	current := &node{
		id:     7,
		bounds: woxui.Rect{Width: 20, Height: 20},
		focus:  &focusBehavior{focusRingColor: woxui.Color{A: 255}},
		paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRect(bounds, woxui.Color{R: 255, A: 255})
		},
		boundary: cache,
	}

	hiddenWork := &frameWorkCounters{}
	hidden := &woxui.DisplayList{}
	current.draw(hidden, 7, 0, false, false, false, hiddenWork)
	if cache.paint == nil || !cache.paint.Fingerprint.UsesFocusRing || cache.paint.Fingerprint.FocusRing != 0 {
		t.Fatalf("pointer-focus fingerprint = %#v, want a hidden ring dependency", cache.paint)
	}
	hiddenCount := hidden.CommandCount()

	shownWork := &frameWorkCounters{}
	shown := &woxui.DisplayList{}
	current.draw(shown, 7, 7, false, false, false, shownWork)
	if shownWork.paintSegmentReuses != 0 {
		t.Fatalf("keyboard-focus ring reused the hidden-ring segment: %+v", shownWork)
	}
	if cache.paint == nil || cache.paint.Fingerprint.FocusRing != 7 {
		t.Fatalf("keyboard-focus fingerprint = %#v, want ring target 7", cache.paint)
	}
	if shown.CommandCount() <= hiddenCount {
		t.Fatalf("keyboard-focus command count = %d, want more than hidden %d", shown.CommandCount(), hiddenCount)
	}
}
