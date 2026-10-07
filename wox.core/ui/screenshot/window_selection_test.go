package screenshot

import (
	"fmt"
	"testing"
	"wox/util/screen"
)

// TestScreenshotWindowSelection distinguishes a click from freeform selection and later editing.
func TestScreenshotWindowSelection(t *testing.T) {
	front := Rect{X: 40, Y: 40, Width: 80, Height: 80}
	back := Rect{X: 20, Y: 20, Width: 200, Height: 200}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, scenario := range []string{"click", "jitter", "drag", "drag back", "empty desktop", "auto confirm", "edit selection"} {
			t.Run(fmt.Sprintf("%s/scale=%v", scenario, scale), func(t *testing.T) {
				state := &screenshotEditorOverlayState{
					frameSize: Size{Width: 800, Height: 600}, uiScale: scale,
					windowCandidates:  []Rect{front, back},
					displayCandidates: []Rect{{Width: 800, Height: 600}},
					result:            make(chan screenshotEditorOverlayOutcome, 1),
					autoConfirm:       scenario == "auto confirm",
				}
				start, end := Point{X: 60, Y: 60}, Point{X: 60, Y: 60}
				want := front
				if scenario == "jitter" {
					end.X += 2 * scale
				} else if scenario == "drag" {
					end = Point{X: 180, Y: 140}
					want = Rect{X: 60, Y: 60, Width: 120, Height: 80}
				} else if scenario == "empty desktop" {
					start, end, want = Point{X: 300, Y: 300}, Point{X: 300, Y: 300}, Rect{Width: 800, Height: 600}
				}
				state.pointer(PointerEvent{Kind: PointerMove, Position: start})
				if state.hasSelection {
					t.Fatal("hover must not commit a selection")
				}
				state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: start})
				if scenario == "drag back" {
					state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 100, Y: 100}})
					want = Rect{X: 60, Y: 60}
				}
				state.pointer(PointerEvent{Kind: PointerMove, Position: end})
				state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: end})
				if state.selection != want || state.hasSelection != (want.Width >= 2 && want.Height >= 2) {
					t.Fatalf("selection = %+v, selected = %v, want %+v", state.selection, state.hasSelection, want)
				}
				if scenario == "auto confirm" {
					select {
					case outcome := <-state.result:
						if outcome.cancelled {
							t.Fatal("window click was cancelled")
						}
					default:
						t.Fatal("window click did not auto confirm")
					}
				}
				if scenario == "edit selection" {
					state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: start})
					state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 70, Y: 70}})
					state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 70, Y: 70}})
					if state.selection != (Rect{X: 50, Y: 50, Width: 80, Height: 80}) {
						t.Fatalf("selected window cannot be moved: %+v", state.selection)
					}
				}
			})
		}
	}
}

// TestScreenshotDisplayFallbackMapsMixedDPI checks both platform coordinate spaces, monitor transitions, gaps, and clipped captures.
func TestScreenshotDisplayFallbackMapsMixedDPI(t *testing.T) {
	displays := screenshotDisplayLayout([]screen.Display{
		{Bounds: screen.Rect{X: -1280, Width: 1280, Height: 800}, PixelBounds: screen.Rect{X: -1600, Width: 1600, Height: 1000}, Scale: 1.25},
		{Bounds: screen.Rect{Width: 1440, Height: 900}, PixelBounds: screen.Rect{Width: 2880, Height: 1800}, Scale: 2},
		{Bounds: screen.Rect{Y: -600, Width: 800, Height: 600}, PixelBounds: screen.Rect{Y: -900, Width: 1200, Height: 900}, Scale: 1.5},
	})
	for _, physical := range []bool{false, true} {
		capture := Rect{X: -1280, Y: -600, Width: 2720, Height: 1500}
		want := []Rect{{Y: 600, Width: 1280, Height: 800}, {X: 1280, Y: 600, Width: 1440, Height: 900}, {X: 1280, Width: 800, Height: 600}}
		if physical {
			capture = Rect{X: -1600, Y: -900, Width: 4480, Height: 2700}
			want = []Rect{{Y: 900, Width: 1600, Height: 1000}, {X: 1600, Y: 900, Width: 2880, Height: 1800}, {X: 1600, Width: 1200, Height: 900}}
		}
		state := &screenshotEditorOverlayState{displayCandidates: screenshotSelectionDisplayBounds(displays, capture, physical)}
		for _, display := range want {
			point := Point{X: display.X + 10, Y: display.Y + 10}
			state.updateObjectSelectionLocked(point)
			if got := state.objectAtPointLocked(point); got != display {
				t.Fatalf("physical=%v display at %+v = %+v, want %+v", physical, point, got, display)
			}
		}
		for _, point := range []Point{{X: 10, Y: 10}, {X: capture.Width, Y: capture.Height}, {X: -1, Y: 10}} {
			if state.selectionFallbackAtPoint(point) != (Rect{}) {
				t.Fatalf("physical=%v selected a monitor gap or outside point %+v", physical, point)
			}
		}
	}
	clipped := screenshotSelectionDisplayBounds(displays, Rect{X: -1200, Y: 20, Width: 800, Height: 600}, false)
	if len(clipped) != 1 || clipped[0] != (Rect{Width: 800, Height: 600}) {
		t.Fatalf("display fallback escaped the captured region: %+v", clipped)
	}
}

// TestScreenshotWindowHitTestUsesFrontToBackOrder covers overlap and exclusive frame edges.
func TestScreenshotWindowHitTestUsesFrontToBackOrder(t *testing.T) {
	front := Rect{X: 40, Y: 40, Width: 80, Height: 80}
	back := Rect{X: 20, Y: 20, Width: 200, Height: 200}
	state := &screenshotEditorOverlayState{windowCandidates: []Rect{front, back}}
	for _, tc := range []struct {
		point Point
		want  Rect
	}{
		{Point{X: 60, Y: 60}, front}, {Point{X: 120, Y: 60}, back},
		{Point{X: 30, Y: 30}, back}, {Point{X: 220, Y: 220}, Rect{}},
	} {
		if got := state.windowAtPoint(tc.point); got != tc.want {
			t.Fatalf("window at %+v = %+v, want %+v", tc.point, got, tc.want)
		}
	}
}

// TestScreenshotWindowPreviewUsesPointerDisplayScale keeps hover chrome on the active mixed-DPI display.
func TestScreenshotWindowPreviewUsesPointerDisplayScale(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 800, 600), frameSize: Size{Width: 800, Height: 600},
		windowCandidates: []Rect{{X: 0, Y: 0, Width: 800, Height: 600}},
		pointerPosition:  Point{X: 100, Y: 100}, pointerInside: true,
		chromeScale: func(bounds Rect) float32 {
			if bounds.X < 400 {
				return 1.5
			}
			return 2
		},
	}
	first := &DisplayList{}
	state.draw(first, FrameInfo{Size: state.frameSize})
	if state.uiScale != 1.5 || state.hasSelection || state.toolbarRect != (Rect{}) || state.sizeLabelRect != (Rect{}) {
		t.Fatalf("preview committed selection or used wrong display scale: scale=%v selected=%v toolbar=%+v", state.uiScale, state.hasSelection, state.toolbarRect)
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 500, Y: 100}})
	second := &DisplayList{}
	state.draw(second, FrameInfo{Size: state.frameSize})
	if state.uiScale != 2 || first.Compare(second) == nil {
		t.Fatal("crossing displays did not update preview chrome")
	}
	// Even at the desktop origin, an uncommitted preview must start a click gesture rather than resizing an empty selection.
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{}})
	if !state.hasSelection || state.selection != state.windowCandidates[0] {
		t.Fatalf("origin click failed to select the window: %+v", state.selection)
	}
}
