package screenshot

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestNewScreenshotEditorImageSharesPackedRGBA(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 2))
	source.SetRGBA(1, 1, color.RGBA{R: 20, G: 40, B: 60, A: 255})

	prepared, err := newScreenshotEditorImage(source)
	if err != nil {
		t.Fatalf("prepare screenshot image: %v", err)
	}
	source.SetRGBA(1, 1, color.RGBA{R: 80, G: 100, B: 120, A: 255})
	if pixel := prepared.RGBAAt(1, 1); pixel.R != 80 || pixel.G != 100 || pixel.B != 120 {
		t.Fatalf("packed screenshot pixels were copied: got %+v", pixel)
	}
	if prepared.Width != 4 || prepared.Height != 2 {
		t.Fatalf("prepared size = %dx%d, want 4x2", prepared.Width, prepared.Height)
	}
}

func TestScreenshotEditorWindowHostSerializesCallbacks(t *testing.T) {
	host := &screenshotEditorWindowHost{}
	first := &screenshotEditorOverlayState{}
	second := &screenshotEditorOverlayState{}
	if err := host.begin(first); err != nil {
		t.Fatalf("begin first screenshot session: %v", err)
	}
	if err := host.begin(second); err == nil {
		t.Fatal("second screenshot session should be rejected while the first is active")
	}
	host.end(second)
	if host.current() != first {
		t.Fatal("ending a different session cleared the active screenshot")
	}
	host.end(first)
	if host.current() != nil {
		t.Fatal("active screenshot session was not cleared")
	}
}

func TestNewScreenshotEditorImageCopiesStridedRGBA(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 2))
	subImage := source.SubImage(image.Rect(1, 0, 3, 2)).(*image.RGBA)
	subImage.SetRGBA(1, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})

	prepared, err := newScreenshotEditorImage(subImage)
	if err != nil {
		t.Fatalf("prepare screenshot subimage: %v", err)
	}
	subImage.SetRGBA(1, 0, color.RGBA{R: 90, G: 100, B: 110, A: 255})
	if pixel := prepared.RGBAAt(0, 0); pixel.R != 10 || pixel.G != 20 || pixel.B != 30 {
		t.Fatalf("strided screenshot pixels did not use the normalized copy path: got %+v", pixel)
	}
	if prepared.Width != 2 || prepared.Height != 2 {
		t.Fatalf("prepared size = %dx%d, want 2x2", prepared.Width, prepared.Height)
	}
}

func TestScreenshotEditorSelectionMapsLogicalPointsToPixels(t *testing.T) {
	state := &screenshotEditorOverlayState{frameSize: Size{Width: 1000, Height: 500}}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 400, Y: 300}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 100, Y: 50}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 100, Y: 50}})

	wantSelection := Rect{X: 100, Y: 50, Width: 300, Height: 250}
	if state.selection != wantSelection {
		t.Fatalf("selection = %+v, want %+v", state.selection, wantSelection)
	}
	pixels, err := screenshotEditorPixelSelection(image.Rect(0, 0, 2000, 1000), state.selection, state.frameSize)
	if err != nil {
		t.Fatalf("map selection: %v", err)
	}
	wantPixels := image.Rect(200, 100, 800, 600)
	if pixels != wantPixels {
		t.Fatalf("pixels = %v, want %v", pixels, wantPixels)
	}
}

func TestNewScreenshotEditorOverlayStateAppliesNativeSelection(t *testing.T) {
	initial := Rect{X: -10, Y: 20, Width: 160, Height: 90}
	state := newScreenshotEditorOverlayState(
		ScreenshotOptions{AutoConfirm: true, HideAnnotationToolbar: true},
		&Image{},
		screenshotEditorPlatform{
			frameSize:        Size{Width: 120, Height: 80},
			initialSelection: &initial,
		},
	)

	if state.selection != (Rect{X: 0, Y: 20, Width: 120, Height: 60}) {
		t.Fatalf("selection = %+v, want clamped native selection", state.selection)
	}
	if !state.hasSelection {
		t.Fatal("native selection should be active")
	}
	if !state.autoConfirm || !state.hideTools {
		t.Fatalf("options were not preserved: autoConfirm=%t hideTools=%t", state.autoConfirm, state.hideTools)
	}
	if state.frameSize != (Size{Width: 120, Height: 80}) {
		t.Fatalf("frame size = %+v", state.frameSize)
	}
}

func TestScreenshotEditorToolbarMatchesFlutterGeometry(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 1, 1),
		selection:    Rect{X: 100, Y: 100, Width: 900, Height: 400},
		hasSelection: true,
		result:       make(chan screenshotEditorOverlayOutcome, 1),
	}
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 1200, Height: 700}})

	if state.toolbarRect.Width != 632 || state.toolbarRect.Height != 60 {
		t.Fatalf("toolbar bounds = %+v, want 632x60", state.toolbarRect)
	}
	if state.toolbarRect.X != state.selection.X+state.selection.Width-state.toolbarRect.Width {
		t.Fatalf("toolbar left = %v, want right-aligned to selection", state.toolbarRect.X)
	}
	if state.pinRect.Width != 40 || state.cancelRect.Width != 40 || state.confirmRect.Width != 40 {
		t.Fatalf("action bounds = pin %+v cancel %+v confirm %+v", state.pinRect, state.cancelRect, state.confirmRect)
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.pinRect.X + 20, Y: state.pinRect.Y + 20}})
	if outcome := <-state.result; !outcome.pinned || outcome.cancelled {
		t.Fatalf("pin outcome = %+v", outcome)
	}
}

func TestScreenshotEditorToolbarIconsRenderFromSharedSVGs(t *testing.T) {
	names := append(screenshotEditorToolIconNames[:],
		"control.undo",
		"screenshot.scrolling-capture",
		"screenshot.cursor",
		"screenshot.pin",
		"control.close",
		"control.check",
		"control.remove",
		"control.add",
		"control.delete",
	)
	for _, name := range names {
		displayList := &DisplayList{}
		drawScreenshotEditorToolbarIcon(displayList, name, Rect{Width: 40, Height: 40}, Color{R: 255, G: 255, B: 255, A: 255}, 1)
		if displayList.CommandCount() != 1 {
			t.Fatalf("toolbar icon %q did not render as an SVG image", name)
		}
	}
}

func TestScreenshotEditorChromeUsesSelectionMonitorScale(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 2000, 1200),
		selection:    Rect{X: 200, Y: 100, Width: 1400, Height: 800},
		hasSelection: true,
		chromeScale:  func(Rect) float32 { return 1.5 },
	}
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 2000, Height: 1200}})
	if state.uiScale != 1.5 {
		t.Fatalf("chrome scale = %.2f, want 1.5", state.uiScale)
	}
	if state.toolbarRect.Width != 948 || state.toolbarRect.Height != 90 {
		t.Fatalf("scaled toolbar = %+v, want 948x90", state.toolbarRect)
	}
	if state.confirmRect.Width != 60 || state.confirmRect.Height != 60 {
		t.Fatalf("scaled confirm action = %+v, want 60x60", state.confirmRect)
	}
}

func TestScreenshotEditorCursorToggleAndExport(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 400, 200))
	mapped := screenshotEditorCursorPixelFromDesktop(Point{X: -50, Y: 25}, Rect{X: -100, Width: 200, Height: 100}, source)
	if mapped == nil || *mapped != (Point{X: 100, Y: 50}) {
		t.Fatalf("mapped cursor = %+v, want source pixel 100,50", mapped)
	}
	if outside := screenshotEditorCursorPixelFromDesktop(Point{X: 100, Y: 25}, Rect{X: -100, Width: 200, Height: 100}, source); outside != nil {
		t.Fatalf("outside cursor should be unavailable, got %+v", outside)
	}

	cursorPixel := Point{X: 80, Y: 60}
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 200, 120),
		frameSize:    Size{Width: 200, Height: 120},
		selection:    Rect{X: 20, Y: 20, Width: 140, Height: 80},
		hasSelection: true,
		cursorPixel:  &cursorPixel,
	}
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.cursorRect.X + 20, Y: state.cursorRect.Y + 20}})
	if !state.showCursor {
		t.Fatal("cursor toolbar action did not enable the captured pointer")
	}

	target := image.NewRGBA(image.Rect(0, 0, 200, 120))
	draw.Draw(target, target.Bounds(), image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}), image.Point{}, draw.Src)
	if err := renderScreenshotEditorCursor(target, cursorPixel, state.selection, state.frameSize); err != nil {
		t.Fatalf("render cursor: %v", err)
	}
	foundDarkPixel := false
	for y := 55; y < 90 && !foundDarkPixel; y++ {
		for x := 75; x < 115; x++ {
			pixel := target.RGBAAt(x, y)
			if pixel.R < 80 && pixel.G < 80 && pixel.B < 80 && pixel.A > 0 {
				foundDarkPixel = true
				break
			}
		}
	}
	if !foundDarkPixel {
		t.Fatal("exported cursor marker did not contain its dark outline")
	}
}

func TestScreenshotEditorAnnotationDrawUndoAndExport(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 1, 1),
		frameSize:    Size{Width: 100, Height: 50},
		selection:    Rect{Width: 100, Height: 50},
		hasSelection: true,
		activeTool:   screenshotEditorToolRect,
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 20, Y: 10}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 50, Y: 30}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 50, Y: 30}})
	if len(state.annotations) != 1 || state.annotations[0].rect != (Rect{X: 20, Y: 10, Width: 30, Height: 20}) {
		t.Fatalf("annotations = %+v", state.annotations)
	}

	source := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			source.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	composited, err := renderScreenshotEditorAnnotations(source, state.annotations, state.selection, state.frameSize)
	if err != nil {
		t.Fatalf("render annotations: %v", err)
	}
	if pixel := composited.RGBAAt(40, 20); pixel.R != 255 || pixel.G != 91 || pixel.B != 54 {
		t.Fatalf("annotation pixel = %+v", pixel)
	}

	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.undoRect.X + 20, Y: state.undoRect.Y + 20}})
	if len(state.annotations) != 0 {
		t.Fatalf("undo left %d annotations", len(state.annotations))
	}
}

func TestScreenshotEditorCreatesEveryAnnotationTool(t *testing.T) {
	for _, tool := range []screenshotEditorTool{
		screenshotEditorToolRect,
		screenshotEditorToolEllipse,
		screenshotEditorToolArrow,
		screenshotEditorToolMosaic,
	} {
		state := &screenshotEditorOverlayState{
			frameSize:    Size{Width: 200, Height: 100},
			selection:    Rect{Width: 200, Height: 100},
			hasSelection: true,
			activeTool:   tool,
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 20, Y: 20}})
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 80, Y: 60}})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 60}})
		if len(state.annotations) != 1 || state.annotations[0].tool != tool {
			t.Fatalf("tool %d annotations = %+v", tool, state.annotations)
		}
	}

	textState := &screenshotEditorOverlayState{
		frameSize:    Size{Width: 200, Height: 100},
		selection:    Rect{Width: 200, Height: 100},
		hasSelection: true,
		activeTool:   screenshotEditorToolText,
	}
	textState.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 20, Y: 20}})
	textState.textInput(TextInputEvent{Kind: TextInputCommit, Text: "中文"})
	textState.key(KeyEvent{Key: KeyEnter, Down: true})
	if len(textState.annotations) != 1 || textState.annotations[0].text != "中文" {
		t.Fatalf("text annotations = %+v", textState.annotations)
	}
}

func TestScreenshotEditorSelectMovesResizesAndDeletesAnnotation(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize:    Size{Width: 240, Height: 140},
		selection:    Rect{X: 20, Y: 20, Width: 160, Height: 100},
		hasSelection: true,
		activeTool:   screenshotEditorToolSelect,
		annotations:  []screenshotEditorAnnotation{{tool: screenshotEditorToolRect, rect: Rect{X: 50, Y: 40, Width: 40, Height: 30}}},
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 65, Y: 55}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 85, Y: 65}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 85, Y: 65}})
	if got := state.annotations[0].rect; got != (Rect{X: 70, Y: 50, Width: 40, Height: 30}) {
		t.Fatalf("moved annotation = %+v", got)
	}

	bottomRight := Point{X: 110, Y: 80}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: bottomRight})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 130, Y: 95}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 130, Y: 95}})
	if got := state.annotations[0].rect; got != (Rect{X: 70, Y: 50, Width: 60, Height: 45}) {
		t.Fatalf("resized annotation = %+v", got)
	}

	if !state.key(KeyEvent{Key: KeyDelete, Down: true}) || len(state.annotations) != 0 {
		t.Fatalf("delete left annotations = %+v", state.annotations)
	}

	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 60, Y: 70}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 80, Y: 80}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 80}})
	if got := state.selection; got != (Rect{X: 40, Y: 30, Width: 160, Height: 100}) {
		t.Fatalf("moved selection = %+v", got)
	}
}

func TestScreenshotEditorEditBarUpdatesCreationAndSelectedAnnotation(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:           testScreenshotImage(t, 1, 1),
		frameSize:       Size{Width: 800, Height: 600},
		selection:       Rect{X: 100, Y: 100, Width: 400, Height: 300},
		hasSelection:    true,
		activeTool:      screenshotEditorToolRect,
		annotationColor: screenshotEditorAnnotationColor,
		mosaicRadius:    screenshotEditorMosaicRadius,
		textFontSize:    screenshotEditorTextFontSize,
	}
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	greenRect := state.editColorRects[2]
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: greenRect.X + 10, Y: greenRect.Y + 10}})
	if state.annotationColor != screenshotEditorPalette[2] {
		t.Fatalf("creation color = %+v, want %+v", state.annotationColor, screenshotEditorPalette[2])
	}

	state.activeTool = screenshotEditorToolMosaic
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	largeRect := state.editSizeRects[2]
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: largeRect.X + 16, Y: largeRect.Y + 16}})
	if state.mosaicRadius != 28 {
		t.Fatalf("mosaic radius = %v, want 28", state.mosaicRadius)
	}

	state.activeTool = screenshotEditorToolSelect
	state.annotations = []screenshotEditorAnnotation{{tool: screenshotEditorToolText, start: Point{X: 200, Y: 180}, text: "Wox", fontSize: 20}}
	state.selectedAnnotation = 0
	state.hasSelectedMark = true
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	increaseRect := state.editIncreaseRect
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: increaseRect.X + 21, Y: increaseRect.Y + 21}})
	if state.annotations[0].fontSize != 22 {
		t.Fatalf("text font size = %v, want 22", state.annotations[0].fontSize)
	}
	deleteRect := state.editDeleteRect
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: deleteRect.X + 21, Y: deleteRect.Y + 21}})
	if len(state.annotations) != 0 {
		t.Fatalf("delete left annotations = %+v", state.annotations)
	}
}

func testScreenshotImage(t *testing.T, width, height int) *Image {
	t.Helper()
	prepared, err := NewImage(image.NewRGBA(image.Rect(0, 0, width, height)))
	if err != nil {
		t.Fatalf("prepare test screenshot image: %v", err)
	}
	return prepared
}

func TestScreenshotScrollingCaptureMatchesAndStitchesBothDirections(t *testing.T) {
	document := image.NewRGBA(image.Rect(0, 0, 120, 900))
	for y := 0; y < document.Bounds().Dy(); y++ {
		for x := 0; x < document.Bounds().Dx(); x++ {
			document.SetRGBA(x, y, color.RGBA{
				R: uint8((x*7 + y*3) % 251),
				G: uint8((x*11 + y*5) % 253),
				B: uint8((x*13 + y*17) % 255),
				A: 255,
			})
		}
	}
	viewport := func(top int) *image.RGBA {
		frame := image.NewRGBA(image.Rect(0, 0, 120, 300))
		draw.Draw(frame, frame.Bounds(), document, image.Pt(0, top), draw.Src)
		return frame
	}

	frames, accepted := appendScreenshotScrollingFrame(nil, viewport(180))
	if !accepted {
		t.Fatal("first frame was rejected")
	}
	frames, accepted = appendScreenshotScrollingFrame(frames, viewport(0))
	if !accepted || len(frames) != 2 {
		t.Fatalf("prepend accepted=%v frames=%d", accepted, len(frames))
	}
	frames, accepted = appendScreenshotScrollingFrame(frames, viewport(360))
	if !accepted || len(frames) != 3 {
		t.Fatalf("append accepted=%v frames=%d", accepted, len(frames))
	}
	if _, accepted = appendScreenshotScrollingFrame(frames, viewport(360)); accepted {
		t.Fatal("duplicate frame was accepted")
	}

	stitched, err := stitchScreenshotScrollingFrames(frames)
	if err != nil {
		t.Fatalf("stitch frames: %v", err)
	}
	if stitched.Bounds() != image.Rect(0, 0, 120, 660) {
		t.Fatalf("stitched bounds = %v, want 120x660", stitched.Bounds())
	}
	for _, point := range []image.Point{{X: 17, Y: 0}, {X: 71, Y: 299}, {X: 23, Y: 479}, {X: 99, Y: 659}} {
		if got, want := stitched.RGBAAt(point.X, point.Y), document.RGBAAt(point.X, point.Y); got != want {
			t.Fatalf("pixel %v = %+v, want %+v", point, got, want)
		}
	}
}
