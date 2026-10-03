package screenshot

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	woxui "wox/ui/runtime"
)

// TestScreenshotEditorArrowControlDrag covers creating, bending, and straightening with off-center grabs at different display scales.
func TestScreenshotEditorArrowControlDrag(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		state := &screenshotEditorOverlayState{
			frameSize: Size{Width: 600, Height: 400}, uiScale: scale,
			selection: Rect{X: 20, Y: 20, Width: 560, Height: 340}, hasSelection: true,
			activeTool: screenshotEditorToolArrow,
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 100, Y: 100}})
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 400, Y: 100}})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 400, Y: 100}})
		if len(state.annotations) != 1 {
			t.Fatalf("scale %v: failed to create arrow", scale)
		}
		original := state.annotations[0]
		grab := Point{X: 253, Y: 102}
		state.pointer(PointerEvent{Kind: PointerMove, Position: grab})
		if state.pointerCursor != PointerCursorMove {
			t.Fatalf("scale %v: middle cursor = %v", scale, state.pointerCursor)
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: grab})
		if state.editMode != screenshotEditorEditArrowMiddle {
			t.Fatalf("scale %v: middle started edit mode %v", scale, state.editMode)
		}
		state.pointer(PointerEvent{Kind: PointerMove, Position: grab})
		if state.annotations[0].arrowBend != (Point{}) {
			t.Fatal("off-center grab moved the curve before dragging")
		}
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 283, Y: 202}})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 283, Y: 202}})
		curved := state.annotations[0]
		if curved.start != original.start || curved.end != original.end || screenshotEditorArrowMiddle(curved) != (Point{X: 280, Y: 200}) {
			t.Fatalf("scale %v: bending changed endpoints or lost the grab offset: %+v", scale, curved)
		}
		for _, point := range []Point{curved.start, screenshotEditorArrowMiddle(curved), curved.end} {
			state.pointer(PointerEvent{Kind: PointerMove, Position: point})
			if state.pointerCursor != PointerCursorMove {
				t.Fatalf("scale %v: control %+v cursor = %v", scale, point, state.pointerCursor)
			}
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 280, Y: 200}})
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 250, Y: 100}})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 250, Y: 100}})
		if state.annotations[0].arrowBend != (Point{}) {
			t.Fatal("dragging the middle back to the chord did not restore a straight arrow")
		}
	}
}

// TestScreenshotEditorArrowEndpointGrab preserves the other controls and the grab offset, including when endpoints reach the selection bounds.
func TestScreenshotEditorArrowEndpointGrab(t *testing.T) {
	annotation := screenshotEditorAnnotation{tool: screenshotEditorToolArrow, start: Point{X: 100, Y: 100}, end: Point{X: 116, Y: 100}, arrowBend: Point{Y: 2}}
	for _, control := range []struct {
		point Point
		mode  screenshotEditorEditMode
	}{
		{annotation.start, screenshotEditorEditArrowStart},
		{screenshotEditorArrowMiddle(annotation), screenshotEditorEditArrowMiddle},
		{annotation.end, screenshotEditorEditArrowEnd},
	} {
		_, mode, found := screenshotEditorAnnotationHandleAt(annotation, control.point, 2)
		if !found || mode != control.mode {
			t.Fatalf("overlapping control %+v resolved to %v", control, mode)
		}
	}
	annotation.end = Point{X: 400, Y: 100}
	for _, bend := range []Point{{}, {X: 20, Y: 100}} {
		annotation.arrowBend = bend
		middle := screenshotEditorArrowMiddle(annotation)
		for _, endpoint := range []Point{annotation.start, annotation.end} {
			state := &screenshotEditorOverlayState{
				frameSize: Size{Width: 600, Height: 400}, uiScale: 1.5,
				selection: Rect{Width: 600, Height: 400}, hasSelection: true,
				annotations: []screenshotEditorAnnotation{annotation}, hasSelectedMark: true,
			}
			grab := Point{X: endpoint.X + 4, Y: endpoint.Y - 3}
			state.pointer(PointerEvent{Kind: PointerMove, Position: grab})
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: grab})
			for _, delta := range []Point{{}, {X: 20, Y: 30}, {X: 1000, Y: 1000}, {X: -1000, Y: -1000}, {}} {
				state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: grab.X + delta.X, Y: grab.Y + delta.Y}})
				got := state.annotations[0]
				moved, fixed, wantFixed := got.start, got.end, annotation.end
				if endpoint == annotation.end {
					moved, fixed, wantFixed = got.end, got.start, annotation.start
				}
				want := Point{X: min(max(endpoint.X+delta.X, 0), 600), Y: min(max(endpoint.Y+delta.Y, 0), 400)}
				if moved != want || fixed != wantFixed || screenshotEditorArrowMiddle(got) != middle {
					t.Fatalf("endpoint %+v delta %+v: moved=%+v fixed=%+v middle=%+v, want %+v, %+v, %+v", endpoint, delta, moved, fixed, screenshotEditorArrowMiddle(got), want, wantFixed, middle)
				}
			}
			state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: grab})
		}
	}
}

// TestScreenshotEditorArrowCurveHitAndMove checks that selection follows the curve after display changes and translation preserves its shape.
func TestScreenshotEditorArrowCurveHitAndMove(t *testing.T) {
	annotation := screenshotEditorAnnotation{tool: screenshotEditorToolArrow, start: Point{X: -300, Y: -200}, end: Point{X: -60, Y: -200}, arrowBend: Point{Y: 100}}
	middle := screenshotEditorArrowMiddle(annotation)
	for _, scale := range []float32{1, 2, 1.25, 1.5} {
		for _, point := range []Point{middle, {X: middle.X, Y: middle.Y + 5*scale}} {
			if _, found := screenshotEditorAnnotationAt([]screenshotEditorAnnotation{annotation}, point, scale); !found {
				t.Fatalf("scale %v: visible curve could not be selected at %+v", scale, point)
			}
		}
		if screenshotEditorAnnotationContains(annotation, Point{X: -180, Y: -200}, scale) {
			t.Fatal("empty endpoint chord was selectable")
		}
	}
	bounds := Rect{X: -400, Y: -300, Width: 400, Height: 300}
	moved := shiftScreenshotEditorAnnotationWithinBounds(annotation, Point{X: 1000, Y: 1000}, bounds, 1)
	if moved.arrowBend != annotation.arrowBend || moved.start != (Point{X: -240, Y: -100}) || moved.end != (Point{X: 0, Y: -100}) || screenshotEditorArrowMiddle(moved) != (Point{X: -120, Y: 0}) {
		t.Fatalf("clamped translation changed the curve or crossed the selection: %+v", moved)
	}
}

// TestScreenshotEditorArrowCurveDamage includes off-midpoint extrema and both old and new curves when bending across the chord.
func TestScreenshotEditorArrowCurveDamage(t *testing.T) {
	annotation := screenshotEditorAnnotation{tool: screenshotEditorToolArrow, start: Point{X: -300, Y: -200}, end: Point{X: -80, Y: -100}, arrowBend: Point{X: -70, Y: -90}}
	bounds := screenshotEditorAnnotationBounds(annotation, 1)
	state := &screenshotEditorOverlayState{
		selection:   Rect{X: -600, Y: -500, Width: 800, Height: 800},
		annotations: []screenshotEditorAnnotation{annotation}, hasSelectedMark: true,
		editMode: screenshotEditorEditArrowMiddle, editOriginalMark: annotation,
		start: screenshotEditorArrowMiddle(annotation), uiScale: 1.5,
	}
	damage := state.updateSelectEditLocked(Point{X: -140, Y: 40}, 0)
	for index := 0; index <= 1000; index++ {
		point := screenshotEditorArrowPoint(annotation, float32(index)/1000)
		if point.X < bounds.X-0.001 || point.X > bounds.X+bounds.Width+0.001 || point.Y < bounds.Y-0.001 || point.Y > bounds.Y+bounds.Height+0.001 {
			t.Fatalf("curve extrema escaped bounds %+v at %+v", bounds, point)
		}
		for _, curve := range []screenshotEditorAnnotation{annotation, state.annotations[0]} {
			point := screenshotEditorArrowPoint(curve, float32(index)/1000)
			if !screenshotEditorRectContains(damage, point) {
				t.Fatalf("old/new curve escaped drag damage %+v at %+v", damage, point)
			}
		}
	}
	for _, curve := range []screenshotEditorAnnotation{annotation, state.annotations[0]} {
		_, head := screenshotEditorArrowGeometry(curve, screenshotEditorAnnotationPreviewStroke(1.5), 0.25)
		for _, corner := range head {
			if !screenshotEditorRectContains(damage, corner) {
				t.Fatalf("arrowhead escaped drag damage: %+v", corner)
			}
		}
	}
}

// TestScreenshotEditorArrowCurveRender verifies the rendered shaft and tangent-aligned head independently of capture scale and chrome scale.
func TestScreenshotEditorArrowCurveRender(t *testing.T) {
	annotation := screenshotEditorAnnotation{tool: screenshotEditorToolArrow, start: Point{X: 100, Y: 100}, end: Point{X: 400, Y: 100}, arrowBend: Point{Y: 120}, color: screenshotEditorAnnotationColor}
	ink := color.RGBA{R: screenshotEditorAnnotationColor.R, G: screenshotEditorAnnotationColor.G, B: screenshotEditorAnnotationColor.B, A: 255}
	background := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	frame := Size{Width: 600, Height: 400}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		width := screenshotEditorAnnotationPreviewStroke(scale)
		_, head := screenshotEditorArrowGeometry(annotation, width, 0.25)
		// This curve reaches the tip in direction (300, -480), not along the horizontal chord.
		dx := head[0].X - (head[1].X+head[2].X)/2
		dy := head[0].Y - (head[1].Y+head[2].Y)/2
		if math.Abs(float64(dx*(-480)-dy*300)) > 0.02 || dx <= 0 || dy >= 0 {
			t.Fatalf("arrowhead did not follow the end tangent: %+v", head)
		}
		list := &DisplayList{RasterScale: scale}
		list.FillRect(Rect{Width: frame.Width, Height: frame.Height}, Color{R: 30, G: 30, B: 30, A: 255})
		drawScreenshotEditorArrow(list, annotation, width, screenshotEditorAnnotationColor)
		renderer, _ := woxui.NewSoftwareRenderer(600, 400)
		if err := renderer.Render(list); err != nil {
			t.Fatal(err)
		}
		if renderer.RGBA().RGBAAt(250, 220) != ink || renderer.RGBA().RGBAAt(250, 100) != background {
			t.Fatalf("scale %v: preview did not follow the curved shaft", scale)
		}
		for _, capture := range []Size{{Width: 600, Height: 400}, {Width: 750, Height: 500}, {Width: 1200, Height: 800}, {Width: 900, Height: 604}} {
			source := image.NewRGBA(image.Rect(0, 0, int(capture.Width), int(capture.Height)))
			draw.Draw(source, source.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
			output, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{annotation}, Rect{X: 50, Y: 50, Width: 450, Height: 300}, frame, scale)
			if err != nil {
				t.Fatal(err)
			}
			for _, sample := range []struct {
				point Point
				color color.RGBA
			}{
				{Point{X: 250, Y: 220}, ink},
				{Point{X: 175, Y: 190}, ink},
				{Point{X: (head[0].X + head[1].X + head[2].X) / 3, Y: (head[0].Y + head[1].Y + head[2].Y) / 3}, ink},
				{Point{X: 250, Y: 100}, background},
				{Point{X: 403, Y: 96}, background},
			} {
				pixel := screenshotEditorScalePoint(sample.point, capture.Width/frame.Width, capture.Height/frame.Height)
				if got := output.RGBAAt(pixel.X, pixel.Y); got != sample.color {
					t.Fatalf("chrome %v, capture %+v: pixel %v = %+v, want %+v", scale, capture, pixel, got, sample.color)
				}
			}
		}
	}
}
