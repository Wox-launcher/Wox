package screenshot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"slices"
	"testing"

	woxui "wox/ui/runtime"
)

func TestScreenshotEditorHighlighterInput(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize: Size{Width: 900, Height: 650}, selection: Rect{X: 30, Y: 60, Width: 700, Height: 420}, hasSelection: true,
		annotationColor: screenshotEditorAnnotationColor, brushRadius: 4,
	}
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	center := func(rect Rect) Point { return Point{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height/2} }
	point := func(kind woxui.PointerEventKind, x, y float32, modifiers KeyModifiers) {
		state.pointer(PointerEvent{Kind: kind, Button: PointerButtonPrimary, Position: Point{X: x, Y: y}, Modifiers: modifiers})
	}
	button := center(state.toolRects[screenshotEditorToolHighlighter])
	point(PointerDown, button.X, button.Y, 0)
	yellow := Color{R: 255, G: 255, A: 255}
	palette := screenshotEditorToolPalette(screenshotEditorToolHighlighter)
	if state.activeTool != screenshotEditorToolHighlighter || state.strokeRadiusLocked(state.activeTool) != 10 || state.creationColorLocked() != yellow || palette[0] != yellow {
		t.Fatal("toolbar did not activate the default yellow, 20-pixel marker")
	}
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	swatch, width := center(state.editColorRects[3]), center(state.editSizeRects[2])
	point(PointerDown, swatch.X, swatch.Y, 0)
	point(PointerDown, width.X, width.Y, 0)
	point(PointerDown, 100, 160, KeyModifierShift)
	point(PointerMove, 130, 250, KeyModifierShift)
	point(PointerUp, 280, 160, KeyModifierShift)
	straight := state.annotations[0]
	if !straight.flatCap || !slices.Equal(straight.points, []Point{{X: 100, Y: 160}, {X: 280, Y: 160}}) ||
		straight.color != screenshotEditorPalette[2] || straight.strokeRadius != 14 || state.annotationColor != screenshotEditorAnnotationColor || state.brushRadius != 4 {
		t.Fatalf("unexpected Shift stroke or modified brush preferences: %+v", straight)
	}
	point(PointerDown, 100, 160, 0)
	point(PointerMove, 130, 200, 0)
	point(PointerMove, 180, 270, KeyModifierShift)
	point(PointerMove, 200, 200, KeyModifierShift)
	point(PointerUp, 230, 220, 0)
	want := []Point{{X: 100, Y: 160}, {X: 130, Y: 200}, {X: 200, Y: 200}, {X: 230, Y: 220}}
	if mixed := state.annotations[1]; mixed.flatCap || !slices.Equal(mixed.points, want) {
		t.Fatalf("mixed stroke = %+v, want points %v", mixed, want)
	}
	state.key(KeyEvent{Key: Key("u"), Down: true})
	state.key(KeyEvent{Key: Key("h"), Down: true})
	point(PointerDown, 180, 160, 0)
	point(PointerMove, 180, 190, 0)
	point(PointerUp, 180, 190, 0)
	if state.activeTool != screenshotEditorToolSelect || !state.hasSelectedMark || len(state.annotations) != 1 || state.annotations[0].points[0].Y != 190 {
		t.Fatal("marker could not be selected and moved after toggling the tool off")
	}
	state.key(KeyEvent{Key: KeyDelete, Down: true})
	if len(state.annotations) != 0 {
		t.Fatal("selected marker was not deleted")
	}
}

// TestScreenshotEditorHighlighterCompositesInPaintOrder checks export and display pixels, including alpha and capture scaling.
func TestScreenshotEditorHighlighterCompositesInPaintOrder(t *testing.T) {
	frame := Size{Width: 480, Height: 320}
	selection := Rect{X: 20, Y: 30, Width: 420, Height: 240}
	source := image.NewRGBA(image.Rect(0, 0, 720, 640))
	dark := color.RGBA{R: 33, G: 40, B: 48, A: 255}
	yellow := color.RGBA{R: 255, G: 255, A: 255}
	lightText := color.RGBA{R: 205, G: 217, B: 229, A: 255}
	draw.Draw(source, source.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(source, image.Rect(180, 160, 210, 250), image.Black, image.Point{}, draw.Src)
	draw.Draw(source, image.Rect(480, 0, 720, 640), image.NewUniform(dark), image.Point{}, draw.Src)
	draw.Draw(source, image.Rect(577, 180, 590, 220), image.NewUniform(lightText), image.Point{}, draw.Src)
	// Nearby glyph fragments and a single bright baseline must not flip the backdrop.
	draw.Draw(source, image.Rect(495, 144, 640, 147), image.White, image.Point{}, draw.Src)
	for _, x := range []int{524, 570} {
		for _, y := range []int{170, 230} {
			draw.Draw(source, image.Rect(x-2, y-2, x+3, y+3), image.White, image.Point{}, draw.Src)
		}
	}
	source.SetRGBA(225, 200, color.RGBA{R: 123, G: 123, B: 123, A: 128})
	mark := screenshotEditorAnnotation{tool: screenshotEditorToolHighlighter, flatCap: true, paintOrder: 1,
		points: []Point{{X: 80, Y: 100}, {X: 415, Y: 100}}}
	repeat := mark
	repeat.paintOrder = 2
	brush := screenshotEditorAnnotation{tool: screenshotEditorToolBrush, strokeRadius: 8, color: Color{B: 255, A: 255}, paintOrder: 3,
		points: []Point{{X: 220, Y: 100}}}
	eraser := screenshotEditorAnnotation{tool: screenshotEditorToolEraser, strokeRadius: 8, paintOrder: 4, points: []Point{{X: 275, Y: 100}}}
	marks := []screenshotEditorAnnotation{eraser, repeat, brush, mark}
	bitmap, err := newScreenshotEditorImage(source)
	if err != nil {
		t.Fatal(err)
	}
	state := &screenshotEditorOverlayState{
		image: bitmap, originalSource: source, frameSize: frame, selection: selection, hasSelection: true, autoConfirm: true,
		desktopPixelOrigin: Point{X: -1920, Y: -180}, chromeScale: func(Rect) float32 { return 1.25 }, annotations: marks,
	}
	list := &DisplayList{}
	state.draw(list, FrameInfo{Size: frame})
	renderer, err := woxui.NewSoftwareRenderer(int(frame.Width), int(frame.Height))
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Render(list); err != nil {
		t.Fatal(err)
	}
	export, err := exportScreenshotSelection(source, marks, selection, frame, state.annotationScale(), nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		x    int
		want color.RGBA
	}{
		{75, color.RGBA{R: 255, G: 255, B: 255, A: 255}}, // Before the flat cap.
		{100, color.RGBA{R: 255, G: 255, A: 255}},
		{130, color.RGBA{A: 255}},
		{220, color.RGBA{B: 255, A: 255}}, // Later annotations paint above the marker.
		{275, color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{300, color.RGBA{R: 255, G: 255, A: 255}},
		{321, yellow}, // The stroke crosses from light to dark.
		{365, yellow},
		{388, color.RGBA{A: 255}}, // Light glyph gets dark contrast on yellow.
		{425, dark},
	} {
		if got := export.RGBAAt(sample.x*3/2, 200); got != sample.want {
			t.Errorf("export at %d = %v, want %v", sample.x, got, sample.want)
		}
		if got := renderer.RGBA().RGBAAt(sample.x, 100); got != sample.want {
			t.Errorf("preview at %d = %v, want %v", sample.x, got, sample.want)
		}
	}
	if got := export.RGBAAt(225, 200); got != (color.RGBA{R: 128, G: 128, A: 128}) {
		t.Errorf("marker changed underlying alpha: %v", got)
	}
}

// TestScreenshotEditorHighlighterOverlappingColors verifies that the latest stroke
// determines the ink, even with an ordinary mark between the two strokes.
func TestScreenshotEditorHighlighterOverlappingColors(t *testing.T) {
	frame := Size{Width: 300, Height: 180}
	selection := Rect{Width: frame.Width, Height: frame.Height}
	source := image.NewRGBA(image.Rect(0, 0, 300, 180))
	draw.Draw(source, source.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(source, image.Rect(0, 90, 300, 180), image.Black, image.Point{}, draw.Src)
	draw.Draw(source, image.Rect(110, 30, 118, 60), image.Black, image.Point{}, draw.Src)
	draw.Draw(source, image.Rect(110, 120, 118, 150), image.White, image.Point{}, draw.Src)
	bitmap, err := newScreenshotEditorImage(source)
	if err != nil {
		t.Fatal(err)
	}
	palette := screenshotEditorToolPalette(screenshotEditorToolHighlighter)
	for index, ink := range palette {
		for _, y := range []float32{45, 135} {
			t.Run(fmt.Sprintf("ink%d/y%.0f", index, y), func(t *testing.T) {
				first := screenshotEditorAnnotation{tool: screenshotEditorToolHighlighter, paintOrder: 1, flatCap: true,
					color: palette[(index+1)%len(palette)], points: []Point{{X: 20, Y: y}, {X: 260, Y: y}}}
				brushColor := Color{A: 255}
				if y > 90 {
					brushColor.R, brushColor.G, brushColor.B = 255, 255, 255
				}
				brush := screenshotEditorAnnotation{tool: screenshotEditorToolBrush, paintOrder: 2, color: brushColor, points: []Point{{X: 160, Y: y}}}
				latest := first
				latest.color, latest.paintOrder = ink, 3
				base := []screenshotEditorAnnotation{first, brush}
				committed, err := renderScreenshotEditorAnnotations(source, append(base, latest), selection, frame, 1)
				if err != nil {
					t.Fatal(err)
				}
				onlyLatest, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{brush, latest}, selection, frame, 1)
				if err != nil || !bytes.Equal(committed.Pix, onlyLatest.Pix) {
					t.Fatalf("previous ink leaked into the overlap: %v", err)
				}
				if got := committed.RGBAAt(80, int(y)); got != (color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: 255}) {
					t.Fatalf("selected ink = %+v, rendered %+v", ink, got)
				}
				glyph := committed.RGBAAt(113, int(y))
				if glyph != (color.RGBA{A: 255}) && glyph != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
					t.Fatalf("foreground picked up the ink: %+v", glyph)
				}
				if got := committed.RGBAAt(160, int(y)); got != glyph {
					t.Fatalf("ordinary mark behind the marker lost its contrast: %+v", got)
				}
				list := &DisplayList{}
				list.DrawImage(bitmap, selection)
				preview := screenshotEditorHighlighterPreview{}
				preview.draw(list, source, nil, false, selection, frame, 1, base, &latest)
				renderer, err := woxui.NewSoftwareRenderer(300, 180)
				if err != nil {
					t.Fatal(err)
				}
				if err := renderer.Render(list); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(renderer.RGBA().Pix, committed.Pix) {
					t.Fatal("draft differs from the exported overlap")
				}
			})
		}
	}
}

// TestScreenshotEditorHighlighterDraftCache checks that a moving snapped endpoint only replaces its own preview.
func TestScreenshotEditorHighlighterDraftCache(t *testing.T) {
	frame := Size{Width: 640, Height: 320}
	selection := Rect{Width: frame.Width, Height: frame.Height}
	source := image.NewRGBA(image.Rect(0, 0, 640, 320))
	draw.Draw(source, source.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(source, image.Rect(0, 100, 640, 320), image.NewUniform(color.RGBA{R: 33, G: 38, B: 45, A: 255}), image.Point{}, draw.Src)
	bitmap, err := newScreenshotEditorImage(source)
	if err != nil {
		t.Fatal(err)
	}
	preview := screenshotEditorHighlighterPreview{}
	draft := screenshotEditorAnnotation{tool: screenshotEditorToolHighlighter,
		points: []Point{{X: 20, Y: 60}, {X: 240, Y: 60}, {X: 450, Y: 60}}}
	preview.draw(&DisplayList{}, source, nil, false, selection, frame, 1, nil, &draft)
	underlay, stable := preview.underlay, preview.stable.ordered[0].bitmap
	draft.points[2] = Point{X: 330, Y: 170}
	list := &DisplayList{}
	list.DrawImage(bitmap, selection)
	preview.draw(list, source, nil, false, selection, frame, 1, nil, &draft)
	if preview.underlay != underlay || preview.stable.ordered[0].bitmap != stable {
		t.Fatal("moving the endpoint rebuilt unchanged preview pixels")
	}
	renderer, err := woxui.NewSoftwareRenderer(640, 320)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Render(list); err != nil {
		t.Fatal(err)
	}
	if got := renderer.RGBA().RGBAAt(430, 60); got != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("old snapped endpoint left a ghost: %v", got)
	}
	if got := renderer.RGBA().RGBAAt(320, 160); got != (color.RGBA{R: 255, G: 255, A: 255}) {
		t.Fatalf("new snapped endpoint was not drawn: %v", got)
	}
}
