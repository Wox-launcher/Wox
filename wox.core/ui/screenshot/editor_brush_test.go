package screenshot

import (
	"fmt"
	"image"
	"image/color"
	"reflect"
	"testing"

	woxui "wox/ui/runtime"
)

// TestScreenshotEditorFreehandInput covers crossing existing marks, release-only movement, dot strokes, and undo.
func TestScreenshotEditorFreehandInput(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize: Size{Width: 800, Height: 600}, selection: Rect{X: 20, Y: 20, Width: 700, Height: 500}, hasSelection: true,
		annotations: []screenshotEditorAnnotation{{tool: screenshotEditorToolRect, rect: Rect{X: 60, Y: 60, Width: 300, Height: 300}}},
	}
	for _, tool := range []struct {
		key  Key
		kind screenshotEditorTool
	}{{Key("b"), screenshotEditorToolBrush}, {Key("x"), screenshotEditorToolEraser}} {
		if !state.key(KeyEvent{Key: tool.key, Down: true}) || state.activeTool != tool.kind {
			t.Fatal("freehand shortcut did not select its tool")
		}
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 120, Y: 120}})
		if state.hasHoveredMark || state.pointerCursor != PointerCursorHidden {
			t.Fatal("existing shapes must not intercept the freehand cursor")
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 120, Y: 120}})
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 220, Y: 180}})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 300, Y: 200}})
		mark := state.annotations[len(state.annotations)-1]
		if mark.tool != tool.kind || len(mark.points) != 3 || mark.points[2] != (Point{X: 300, Y: 200}) || state.hasSelectedMark || state.editMode != screenshotEditorEditNone {
			t.Fatalf("freehand stroke was selected, truncated, or lost: %+v", mark)
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 120, Y: 120}})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 120, Y: 120}})
		if len(state.annotations[len(state.annotations)-1].points) != 1 {
			t.Fatal("a click must create one round dot")
		}
		state.key(KeyEvent{Key: Key("u"), Down: true})
		state.key(KeyEvent{Key: tool.key, Down: true})
		if state.activeTool != screenshotEditorToolSelect {
			t.Fatal("toggling freehand off must allow intact marks to be selected")
		}
	}
	if len(state.annotations) != 3 || state.annotations[0].rect != (Rect{X: 60, Y: 60, Width: 300, Height: 300}) {
		t.Fatal("freehand input mutated an existing shape")
	}
	state.key(KeyEvent{Key: Key("u"), Down: true})
	if len(state.annotations) != 2 || state.annotations[1].tool != screenshotEditorToolBrush {
		t.Fatal("undoing an eraser must retain the underlying brush")
	}
}

// TestScreenshotEditorEraserAllTools checks both compositors and restores the complete original mark even while it still crosses the eraser.
func TestScreenshotEditorEraserAllTools(t *testing.T) {
	marks := []screenshotEditorAnnotation{
		{tool: screenshotEditorToolRect, rect: Rect{X: 100, Y: 60, Width: 120, Height: 100}, cornerRadius: 12},
		{tool: screenshotEditorToolEllipse, rect: Rect{X: 100, Y: 60, Width: 120, Height: 100}},
		{tool: screenshotEditorToolArrow, start: Point{X: 100, Y: 110}, end: Point{X: 220, Y: 110}, arrowBend: Point{Y: 20}},
		{tool: screenshotEditorToolText, start: Point{X: 120, Y: 100}, text: "HELLO", fontSize: 24},
		{tool: screenshotEditorToolNumber, start: Point{X: 160, Y: 110}, number: 5},
		{tool: screenshotEditorToolMosaic, points: []Point{{X: 135, Y: 110}, {X: 160, Y: 110}, {X: 190, Y: 110}}},
		{tool: screenshotEditorToolBrush, points: []Point{{X: 100, Y: 110}, {X: 220, Y: 110}}, strokeRadius: 4},
	}
	frame := Size{Width: 320, Height: 240}
	selection := Rect{X: 10, Y: 10, Width: 300, Height: 220}
	for _, mark := range marks {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			t.Run(fmt.Sprintf("tool_%d_scale_%g", mark.tool, scale), func(t *testing.T) {
				source := screenshotStrokeTestSource(400, 360)
				eraser := screenshotEditorAnnotation{tool: screenshotEditorToolEraser, points: []Point{{X: 160, Y: 20}, {X: 160, Y: 200}}, strokeRadius: 10, paintOrder: 1}
				state := &screenshotEditorOverlayState{frameSize: frame, selection: selection, uiScale: scale,
					annotations: []screenshotEditorAnnotation{mark, eraser}, selectedAnnotation: 0, hasSelectedMark: true,
					editMode: screenshotEditorEditMoveAnnotation, editOriginalMark: mark, start: Point{X: 130, Y: 110}}
				before, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{mark}, selection, frame, scale)
				if err != nil {
					t.Fatal(err)
				}
				after, err := renderScreenshotEditorAnnotations(source, state.annotations, selection, frame, scale)
				if err != nil {
					t.Fatal(err)
				}
				checkScreenshotStrokeErasure(t, source, before, after, eraser, scale, 1.25, 1.5)
				previewSource := screenshotStrokeTestSource(320, 240)
				previewBefore := screenshotStrokeTestPreview(t, previewSource, []screenshotEditorAnnotation{mark}, frame, scale)
				previewAfter := screenshotStrokeTestPreview(t, previewSource, state.annotations, frame, scale)
				checkScreenshotStrokeErasure(t, previewSource, previewBefore, previewAfter, eraser, scale, 1, 1)
				state.updateSelectEditLocked(Point{X: 138, Y: 116}, 0)
				moved := state.annotations[0]
				if moved.paintOrder <= eraser.paintOrder || !reflect.DeepEqual(state.annotations[1], eraser) || !reflect.DeepEqual(state.editOriginalMark, mark) {
					t.Fatal("moving must restore only the intact mark without changing the eraser or original geometry")
				}
				want, _ := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{moved}, selection, frame, scale)
				got, _ := renderScreenshotEditorAnnotations(source, state.annotations, selection, frame, scale)
				if !reflect.DeepEqual(want.Pix, got.Pix) {
					t.Fatal("moved annotation was still erased in export")
				}
				wantPreview := screenshotStrokeTestPreview(t, previewSource, []screenshotEditorAnnotation{moved}, frame, scale)
				gotPreview := screenshotStrokeTestPreview(t, previewSource, state.annotations, frame, scale)
				if !reflect.DeepEqual(wantPreview.Pix, gotPreview.Pix) {
					t.Fatal("moved annotation was still erased in preview")
				}
				// A later eraser must cover the restored mark again; undo still removes that last operation.
				eraser.paintOrder = state.nextAnnotationPaintOrderLocked()
				state.annotations = append(state.annotations, eraser)
				again, _ := renderScreenshotEditorAnnotations(source, state.annotations, selection, frame, scale)
				checkScreenshotStrokeErasure(t, source, want, again, eraser, scale, 1.25, 1.5)
				state.hasSelection = true
				state.key(KeyEvent{Key: Key("u"), Down: true})
				undone, _ := renderScreenshotEditorAnnotations(source, state.annotations, selection, frame, scale)
				if !reflect.DeepEqual(want.Pix, undone.Pix) {
					t.Fatal("undo did not recover the restored mark")
				}
			})
		}
	}
}

// checkScreenshotStrokeErasure compares every pixel so erased coverage and untouched content are both verified.
func checkScreenshotStrokeErasure(t *testing.T, source, before, after *image.RGBA, eraser screenshotEditorAnnotation, scale, scaleX, scaleY float32) {
	t.Helper()
	changed := 0
	for y := after.Bounds().Min.Y; y < after.Bounds().Max.Y; y++ {
		for x := after.Bounds().Min.X; x < after.Bounds().Max.X; x++ {
			point := Point{X: (float32(x) + 0.5) / scaleX, Y: (float32(y) + 0.5) / scaleY}
			want := before.RGBAAt(x, y)
			if screenshotEditorStrokeContains(eraser.points, point, screenshotEditorStrokeRadius(eraser, scale)) {
				want = source.RGBAAt(x, y)
			}
			if after.RGBAAt(x, y) != want {
				t.Fatalf("unexpected erasure at %d,%d: got %v want %v", x, y, after.RGBAAt(x, y), want)
			}
			if before.RGBAAt(x, y) != after.RGBAAt(x, y) {
				changed++
			}
		}
	}
	if changed == 0 {
		t.Fatal("eraser did not remove any visible pixels")
	}
}

// screenshotStrokeTestSource creates an opaque patterned capture that exposes wrong source offsets and mosaic erasure.
func screenshotStrokeTestSource(width, height int) *image.RGBA {
	source := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x % 97), G: uint8(y % 113), B: uint8((x + y) % 127), A: 255})
		}
	}
	return source
}

// screenshotStrokeTestPreview exercises the same display list as the native overlay without launching Wox.
func screenshotStrokeTestPreview(t *testing.T, source *image.RGBA, annotations []screenshotEditorAnnotation, frame Size, scale float32) *image.RGBA {
	t.Helper()
	uiImage, err := NewImage(source)
	if err != nil {
		t.Fatal(err)
	}
	list := &DisplayList{}
	list.DrawImage(uiImage, Rect{Width: frame.Width, Height: frame.Height})
	drawScreenshotEditorAnnotations(list, nil, annotations, uiImage, frame, scale)
	renderer, err := woxui.NewSoftwareRenderer(int(frame.Width), int(frame.Height))
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Render(list); err != nil {
		t.Fatal(err)
	}
	return renderer.RGBA()
}

// TestScreenshotEditorStrokeSizeAndToolbar checks independent size controls and reachable buttons across monitor scale changes.
func TestScreenshotEditorStrokeSizeAndToolbar(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 1200, 900), frameSize: Size{Width: 800, Height: 600},
		selection: Rect{X: 100, Y: 100, Width: 500, Height: 250}, hasSelection: true,
		activeTool: screenshotEditorToolBrush, desktopPixelOrigin: Point{X: -1200, Y: -300},
	}
	for _, scale := range []float32{1, 2, 1.25, 1.5, 1} {
		state.chromeScale = func(Rect) float32 { return scale }
		for _, tool := range []screenshotEditorTool{screenshotEditorToolBrush, screenshotEditorToolEraser} {
			state.activeTool = tool
			state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
			buttons := append([]Rect(nil), state.toolRects[1:]...)
			buttons = append(buttons, state.undoRect, state.scrollRect, state.cursorRect, state.pinRect, state.cancelRect, state.saveRect, state.confirmRect)
			for _, button := range buttons {
				if button.X < state.toolbarRect.X || button.Y < state.toolbarRect.Y || button.X+button.Width > state.toolbarRect.X+state.toolbarRect.Width || button.Y+button.Height > state.toolbarRect.Y+state.toolbarRect.Height || button.X+button.Width > state.frameSize.Width {
					t.Fatalf("scale %g: toolbar button is outside its surface: %+v toolbar %+v", scale, button, state.toolbarRect)
				}
			}
			for index, button := range state.editSizeRects {
				if button.X+button.Width > state.editBarRect.X+state.editBarRect.Width || button.Y+button.Height > state.editBarRect.Y+state.editBarRect.Height {
					t.Fatal("stroke size control escaped the property bar")
				}
				state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: button.X + button.Width/2, Y: button.Y + button.Height/2}})
				want := screenshotEditorMosaicRadii[index]
				if tool == screenshotEditorToolBrush {
					want = screenshotEditorBrushRadii[index]
				}
				if state.strokeRadiusLocked(tool) != want {
					t.Fatalf("stroke size = %g, want %g", state.strokeRadiusLocked(tool), want)
				}
			}
		}
		if state.brushRadius != 8 || state.eraserRadius != 28 {
			t.Fatal("brush and eraser size preferences are not independent")
		}
	}
}

// TestScreenshotEditorEraserCache keeps published patches immutable during fast drags and rebuilds them after display scale changes.
func TestScreenshotEditorEraserCache(t *testing.T) {
	source := screenshotStrokeTestSource(600, 450)
	uiImage, err := NewImage(source)
	if err != nil {
		t.Fatal(err)
	}
	frame := Size{Width: 400, Height: 300}
	eraser := screenshotEditorAnnotation{tool: screenshotEditorToolEraser, strokeRadius: 12,
		points: []Point{{X: 160, Y: 120}}, eraserPreview: &screenshotEditorEraserPreview{}}
	for _, scale := range []float32{1, 1, 1.25, 2, 1.5, 1} {
		drawScreenshotEditorEraser(&DisplayList{}, eraser, uiImage, frame, scale)
		cache := eraser.eraserPreview
		previous, previousImage := cache.raster, cache.image
		snapshot := append([]byte(nil), previous.Pix...)
		drawScreenshotEditorEraser(&DisplayList{}, eraser, uiImage, frame, scale)
		if cache.image != previousImage {
			t.Fatal("unchanged eraser repaints should reuse the immutable patch")
		}
		eraser.points = append(eraser.points, Point{X: eraser.points[len(eraser.points)-1].X - 30, Y: eraser.points[len(eraser.points)-1].Y - 20})
		drawScreenshotEditorEraser(&DisplayList{}, eraser, uiImage, frame, scale)
		if !reflect.DeepEqual(previous.Pix, snapshot) {
			t.Fatal("extending a stroke mutated an already published renderer image")
		}
		want := image.NewRGBA(cache.raster.Bounds())
		paintScreenshotEditorStroke(want, want.Bounds(), eraser.points, screenshotEditorStrokeRadius(eraser, scale), 1.5, 1.5, source.RGBAAt)
		if !reflect.DeepEqual(cache.raster.Pix, want.Pix) {
			t.Fatal("incrementally extended eraser differs from a complete rasterization")
		}
	}
}

// TestScreenshotEditorEraserOverlaps keeps untouched pixels, post-erasure drawing, and original hit geometry available.
func TestScreenshotEditorEraserOverlaps(t *testing.T) {
	source := screenshotStrokeTestSource(320, 240)
	frame := Size{Width: 320, Height: 240}
	selection := Rect{X: 80, Y: 70, Width: 160, Height: 90}
	red, blue := Color{R: 255, A: 255}, Color{B: 255, A: 255}
	state := &screenshotEditorOverlayState{frameSize: frame, selection: selection, hasSelection: true, annotations: []screenshotEditorAnnotation{
		{tool: screenshotEditorToolBrush, points: []Point{{X: 40, Y: 100}, {X: 280, Y: 100}}, strokeRadius: 5, color: red, paintOrder: 1},
		{tool: screenshotEditorToolBrush, points: []Point{{X: 160, Y: 40}, {X: 160, Y: 200}}, strokeRadius: 5, color: blue, paintOrder: 2},
		{tool: screenshotEditorToolEraser, points: []Point{{X: 120, Y: 100}, {X: 180, Y: 100}}, strokeRadius: 10, paintOrder: 3},
	}}
	output, err := renderScreenshotEditorAnnotations(source, state.annotations, selection, frame, 1)
	if err != nil {
		t.Fatal(err)
	}
	if output.Bounds() != image.Rect(80, 70, 240, 160) || output.RGBAAt(160, 100) != source.RGBAAt(160, 100) || output.RGBAAt(100, 100).R != 255 || output.RGBAAt(160, 80).B != 255 {
		t.Fatal("overlapping marks or selection clipping were not preserved")
	}
	if index, found := screenshotEditorAnnotationAt(state.annotations, Point{X: 140, Y: 100}, 1); !found || index != 0 {
		t.Fatal("erased geometry must remain selectable and erasers must not intercept selection")
	}
	state.activeTool = screenshotEditorToolBrush
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 150, Y: 100}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 170, Y: 100}})
	output, _ = renderScreenshotEditorAnnotations(source, state.annotations, selection, frame, 1)
	if output.RGBAAt(160, 100) == source.RGBAAt(160, 100) {
		t.Fatal("new brush strokes must be visible over older erasers")
	}
	// Toggle painting off, then move the intact horizontal stroke using its erased segment.
	state.key(KeyEvent{Key: Key("b"), Down: true})
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 140, Y: 100}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 142, Y: 108}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 142, Y: 108}})
	if state.selectedAnnotation != 0 || state.annotations[0].paintOrder <= state.annotations[3].paintOrder {
		t.Fatal("dragging an erased segment did not restore the original brush above the eraser")
	}
}
