package screenshot

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	woxui "wox/ui/runtime"
)

// TestScreenshotEditorRectRadiusDrag exercises all four controls through pointer events, including off-center grabs and both limits.
func TestScreenshotEditorRectRadiusDrag(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for corner, direction := range []Point{{X: 1, Y: 1}, {X: -1, Y: 1}, {X: -1, Y: -1}, {X: 1, Y: -1}} {
			state := &screenshotEditorOverlayState{
				frameSize: Size{Width: 600, Height: 500}, uiScale: scale,
				selection: Rect{X: 20, Y: 20, Width: 560, Height: 440}, hasSelection: true,
				activeTool: screenshotEditorToolRect,
			}
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 100, Y: 100}})
			state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 400, Y: 340}})
			state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 400, Y: 340}})
			if len(state.annotations) != 1 || state.annotations[0].cornerRadius != 0 {
				t.Fatal("new rectangle should start with square corners")
			}
			original := state.annotations[0]
			control := screenshotEditorRectRadiusHandlePoints(original, scale)[corner]
			grab := Point{X: control.X + 2, Y: control.Y - 1}
			state.pointer(PointerEvent{Kind: PointerMove, Position: grab})
			wantCursor := PointerCursorResizeNWSE
			if corner == 1 || corner == 3 {
				wantCursor = PointerCursorResizeNESW
			}
			if state.pointerCursor != wantCursor {
				t.Fatalf("scale %v corner %d: radius cursor = %v", scale, corner, state.pointerCursor)
			}
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: grab})
			if state.editMode != screenshotEditorEditRectRadius || state.editHandle != screenshotEditorHandle(corner*2) {
				t.Fatalf("scale %v corner %d: radius drag started mode %v handle %v", scale, corner, state.editMode, state.editHandle)
			}
			for _, distance := range []float32{0, 20, 1000, -1000, 20} {
				position := Point{X: grab.X + distance*direction.X, Y: grab.Y + distance*direction.Y}
				state.pointer(PointerEvent{Kind: PointerMove, Position: position})
				got := state.annotations[0]
				want := float32(68.28427)
				if distance <= 0 {
					want = 0
				} else if distance == 1000 {
					want = 120
				}
				if got.rect != original.rect || math.Abs(float64(got.cornerRadius-want)) > 0.001 {
					t.Fatalf("scale %v corner %d distance %v: rect=%+v radius=%v, want %v", scale, corner, distance, got.rect, got.cornerRadius, want)
				}
				if distance == 20 {
					moved := screenshotEditorRectRadiusHandlePoints(got, scale)[corner]
					if math.Abs(float64(moved.X-control.X-20*direction.X)) > 0.001 || math.Abs(float64(moved.Y-control.Y-20*direction.Y)) > 0.001 {
						t.Fatalf("control did not follow the pointer: %+v -> %+v", control, moved)
					}
				}
			}
			state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: grab})
			if state.editMode != screenshotEditorEditNone || len(state.annotations) != 1 {
				t.Fatal("radius drag did not finish as a single annotation edit")
			}
		}
	}
}

// TestScreenshotEditorRectRadiusHoverCursor distinguishes rounding from moving, including the first hover over an unselected mark.
func TestScreenshotEditorRectRadiusHoverCursor(t *testing.T) {
	annotation := screenshotEditorAnnotation{tool: screenshotEditorToolRect, rect: Rect{X: 100, Y: 100, Width: 300, Height: 240}, cornerRadius: 60}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, selected := range []bool{false, true} {
			for corner, cursor := range []PointerCursor{PointerCursorResizeNWSE, PointerCursorResizeNESW, PointerCursorResizeNWSE, PointerCursorResizeNESW} {
				state := &screenshotEditorOverlayState{
					frameSize: Size{Width: 600, Height: 500}, uiScale: scale,
					selection: Rect{X: 20, Y: 20, Width: 560, Height: 440}, hasSelection: true,
					annotations: []screenshotEditorAnnotation{annotation}, hasSelectedMark: selected,
				}
				control := screenshotEditorRectRadiusHandlePoints(annotation, scale)[corner]
				state.pointer(PointerEvent{Kind: PointerMove, Position: control})
				if state.pointerCursor != cursor || !state.hasHoveredMark {
					t.Fatalf("scale %v selected %t corner %d: first hover cursor = %v, want %v", scale, selected, corner, state.pointerCursor, cursor)
				}
				state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 250, Y: 220}})
				if state.pointerCursor != PointerCursorMove {
					t.Fatal("leaving the control did not restore the rectangle move cursor")
				}
				state.pointer(PointerEvent{Kind: PointerMove, Position: control})
				state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: control})
				if state.pointerCursor != cursor || state.editMode != screenshotEditorEditRectRadius {
					t.Fatal("pressing the radius control lost its cursor or edit mode")
				}
			}
		}
	}
}

// TestScreenshotEditorRectControlsRemainReachable covers overlapping hit targets, maximum rounding, negative origins, and display changes.
func TestScreenshotEditorRectControlsRemainReachable(t *testing.T) {
	for _, size := range []Size{{Width: 240, Height: 160}, {Width: 30, Height: 20}, {Width: 8, Height: 8}} {
		annotation := screenshotEditorAnnotation{tool: screenshotEditorToolRect, rect: Rect{X: -300, Y: -200, Width: size.Width, Height: size.Height}}
		for _, radius := range []float32{0, min(size.Width, size.Height) / 2} {
			annotation.cornerRadius = radius
			for _, scale := range []float32{1, 2, 1.25, 1.5} {
				for index, point := range screenshotEditorRectHandlePoints(annotation.rect) {
					handle, mode, found := screenshotEditorAnnotationHandleAt(annotation, point, scale)
					if !found || mode != screenshotEditorEditResizeAnnotation || handle != screenshotEditorHandle(index) {
						t.Fatalf("size %+v radius %v scale %v: resize handle %d inaccessible", size, radius, scale, index)
					}
				}
				for index, point := range screenshotEditorRectRadiusHandlePoints(annotation, scale) {
					handle, mode, found := screenshotEditorAnnotationHandleAt(annotation, point, scale)
					if !found || mode != screenshotEditorEditRectRadius || handle != screenshotEditorHandle(index*2) || !screenshotEditorAnnotationContains(annotation, point, scale) {
						t.Fatalf("size %+v radius %v scale %v: radius handle %d inaccessible", size, radius, scale, index)
					}
				}
				if screenshotEditorAnnotationContains(annotation, Point{X: -300, Y: -200}, scale) != (radius == 0) {
					t.Fatal("rounded rectangle hit testing did not exclude its empty corner")
				}
			}
		}
	}
}

// TestScreenshotEditorRoundedRectMoveResize preserves rounding on moves and bounds it when shrinking or resizing with Shift.
func TestScreenshotEditorRoundedRectMoveResize(t *testing.T) {
	annotation := screenshotEditorAnnotation{tool: screenshotEditorToolRect, rect: Rect{X: -300, Y: -200, Width: 240, Height: 160}, cornerRadius: 60}
	bounds := Rect{X: -400, Y: -300, Width: 800, Height: 600}
	moved := shiftScreenshotEditorAnnotationWithinBounds(annotation, Point{X: 500, Y: 400}, bounds, 1.5)
	if moved.cornerRadius != 60 || moved.rect != (Rect{X: 160, Y: 140, Width: 240, Height: 160}) {
		t.Fatalf("moving changed rounding or escaped selection: %+v", moved)
	}
	for _, modifiers := range []KeyModifiers{0, KeyModifierShift} {
		state := &screenshotEditorOverlayState{
			selection: bounds, annotations: []screenshotEditorAnnotation{annotation}, hasSelectedMark: true,
			editMode: screenshotEditorEditResizeAnnotation, editHandle: screenshotEditorHandleBottomRight,
			editOriginalMark: annotation, start: Point{X: -60, Y: -40}, uiScale: 1.5,
		}
		dirty := state.updateSelectEditLocked(Point{X: -260, Y: -180}, modifiers)
		resized := state.annotations[0]
		if resized.cornerRadius != min(resized.rect.Width, resized.rect.Height)/2 {
			t.Fatalf("resized radius escaped the shorter side: %+v", resized)
		}
		if modifiers != 0 && resized.rect.Width != resized.rect.Height {
			t.Fatalf("Shift resize lost square constraint: %+v", resized.rect)
		}
		if !screenshotEditorRectContains(dirty, Point{X: -60, Y: -40}) {
			t.Fatal("resize damage omitted the old corner")
		}
		state.updateSelectEditLocked(state.start, modifiers)
		if state.annotations[0].cornerRadius != 60 {
			t.Fatal("resizing back in the same drag did not restore the original radius")
		}
	}
}

// TestScreenshotEditorRoundedRectRender checks the visible arcs, hollow interior, capture clipping, and independent chrome/pixel scales.
func TestScreenshotEditorRoundedRectRender(t *testing.T) {
	annotation := screenshotEditorAnnotation{tool: screenshotEditorToolRect, rect: Rect{X: 100, Y: 80, Width: 240, Height: 160}, cornerRadius: 40, color: screenshotEditorAnnotationColor}
	ink := color.RGBA{R: 255, G: 91, B: 54, A: 255}
	background := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	frame := Size{Width: 600, Height: 400}
	samples := []struct {
		point Point
		ink   bool
	}{
		{Point{X: 200, Y: 81}, true}, {Point{X: 113, Y: 93}, true},
		{Point{X: 327, Y: 93}, true}, {Point{X: 327, Y: 227}, true}, {Point{X: 113, Y: 227}, true},
		{Point{X: 100, Y: 80}, false}, {Point{X: 102, Y: 82}, false}, {Point{X: 200, Y: 160}, false},
	}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		list := &DisplayList{}
		list.FillRect(Rect{Width: frame.Width, Height: frame.Height}, Color{R: 30, G: 30, B: 30, A: 255})
		drawScreenshotEditorAnnotations(list, nil, []screenshotEditorAnnotation{annotation}, nil, frame, scale)
		renderer, err := woxui.NewSoftwareRenderer(600, 400)
		if err != nil {
			t.Fatal(err)
		}
		if err := renderer.Render(list); err != nil {
			t.Fatal(err)
		}
		for _, sample := range samples {
			want := background
			if sample.ink {
				want = ink
			}
			if got := renderer.RGBA().RGBAAt(int(sample.point.X), int(sample.point.Y)); got != want {
				t.Fatalf("preview scale %v point %+v = %+v, want %+v", scale, sample.point, got, want)
			}
		}
		for _, capture := range []Size{{Width: 600, Height: 400}, {Width: 750, Height: 500}, {Width: 1200, Height: 800}, {Width: 900, Height: 604}} {
			source := image.NewRGBA(image.Rect(0, 0, int(capture.Width), int(capture.Height)))
			draw.Draw(source, source.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
			for _, selection := range []Rect{{X: 50, Y: 50, Width: 450, Height: 300}, {X: 110, Y: 90, Width: 225, Height: 145}} {
				output, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{annotation}, selection, frame, scale)
				if err != nil {
					t.Fatal(err)
				}
				for _, sample := range samples {
					pixel := screenshotEditorScalePoint(sample.point, capture.Width/frame.Width, capture.Height/frame.Height)
					if !pixel.In(output.Bounds()) {
						continue
					}
					got := output.RGBAAt(pixel.X, pixel.Y)
					if (sample.ink && got.R < 220) || (!sample.ink && got != background) {
						t.Fatalf("chrome %v capture %+v crop %+v point %+v = %+v, ink=%t", scale, capture, selection, sample.point, got, sample.ink)
					}
				}
			}
		}
	}
}
