package screenshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"weak"

	"wox/util/clipboard"
	"wox/util/screen"
	"wox/util/screenshotedit"
)

func TestPreparedScreenshotSceneOwnsPixelsAndAnnotations(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 32, 32))
	source.SetRGBA(4, 5, color.RGBA{R: 255, A: 255})
	state := newScreenshotEditorOverlayState(ScreenshotOptions{}, nil, screenshotEditorPlatform{frameSize: Size{Width: 32, Height: 32}})
	state.selection = Rect{Width: 32, Height: 32}
	state.cursorPixel = &Point{X: 8, Y: 9}
	state.annotations = []screenshotEditorAnnotation{{tool: screenshotEditorToolBrush, points: []Point{{X: 3, Y: 3}, {X: 6, Y: 6}}, strokeRadius: 2, color: Color{A: 255}}}
	path := filepath.Join(t.TempDir(), "capture.jpg")
	composited, err := exportScreenshotSelection(source, state.annotations, state.selection, state.frameSize, 1, state.cursorPixel, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeScreenshotImage(path, composited); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = screenshotedit.Remove(path) })
	save, err := prepareScreenshotDocumentSave(path, source, composited, state)
	if err != nil {
		t.Fatal(err)
	}
	if screenshotedit.Available(path) {
		t.Fatal("preparation performed the scene write synchronously")
	}
	// The native source and editor are gone before the plugin starts the deferred write.
	clear(source.Pix)
	state.annotations[0].points[0] = Point{X: 30, Y: 30}
	state.cursorPixel.X = 31
	if err := save(); err != nil {
		t.Fatal(err)
	}
	doc, restored, _, marks, err := loadScreenshotDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if color.RGBAModel.Convert(restored.At(4, 5)) != (color.RGBA{R: 255, A: 255}) || marks[0].points[0] != (Point{X: 3, Y: 3}) || doc.CursorPixel.X != 8 {
		t.Fatal("background save retained mutable editor or native capture data")
	}
}

// TestPreparedScreenshotSceneReleasesBackground keeps the job alive to catch rasters retained after successful saving.
func TestPreparedScreenshotSceneReleasesBackground(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 32, 32))
	selection := Rect{X: 8, Y: 8, Width: 16, Height: 16}
	state := newScreenshotEditorOverlayState(ScreenshotOptions{}, nil, screenshotEditorPlatform{frameSize: Size{Width: 32, Height: 32}, initialSelection: &selection})
	state.originalSource = source
	if err := state.installWindowCapture(selection, image.NewRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	background := image.NewRGBA(image.Rect(0, 0, 24, 24))
	draw.Draw(background, background.Bounds(), image.NewUniform(color.RGBA{A: 255}), image.Point{}, draw.Src)
	retained := weak.Make(background)
	state.backgroundSource, state.showBackground = background, true
	background = nil
	path := filepath.Join(t.TempDir(), "capture.png")
	if err := writeScreenshotImage(path, source); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = screenshotedit.Remove(path) })
	save, err := prepareScreenshotDocumentSave(path, source, source, state)
	if err != nil {
		t.Fatal(err)
	}
	state.releaseWindowBackground()
	if retained.Value() == nil {
		t.Fatal("scene lost its wallpaper before saving")
	}
	for call := 0; call < 2; call++ {
		if err := save(); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	if retained.Value() != nil {
		t.Fatal("completed scene job retained its background raster")
	}
	runtime.KeepAlive(save)
	if !screenshotedit.Available(path) {
		t.Fatal("releasing scene pixels removed the saved editable history")
	}
}

func TestScreenshotDisplayLayoutRequiresExactMatch(t *testing.T) {
	original := []screen.Display{
		{ID: "left", Bounds: screen.Rect{X: -1920, Width: 1920, Height: 1080}, PixelBounds: screen.Rect{X: -1920, Width: 1920, Height: 1080}, Scale: 1},
		{ID: "right", Bounds: screen.Rect{Width: 1920, Height: 1080}, PixelBounds: screen.Rect{Width: 3840, Height: 2160}, Scale: 2, Primary: true},
	}
	saved := screenshotDisplayLayout(original)
	if !screenshotDisplayLayoutMatches(saved, []screen.Display{original[1], original[0]}) {
		t.Fatal("enumeration order must not affect layout matching")
	}
	for _, test := range []struct {
		name   string
		change func([]screen.Display) []screen.Display
	}{
		{"removed", func(d []screen.Display) []screen.Display { return d[:1] }},
		{"added", func(d []screen.Display) []screen.Display { return append(d, d[0]) }},
		{"replaced", func(d []screen.Display) []screen.Display { d[0].ID = "replacement"; return d }},
		{"rearranged with same virtual bounds", func(d []screen.Display) []screen.Display { d[0].ID, d[1].ID = d[1].ID, d[0].ID; return d }},
		{"moved", func(d []screen.Display) []screen.Display { d[0].PixelBounds.Y--; return d }},
		{"resolution", func(d []screen.Display) []screen.Display { d[0].PixelBounds.Width--; return d }},
		{"logical geometry", func(d []screen.Display) []screen.Display { d[0].Bounds.Width--; return d }},
		{"DPI", func(d []screen.Display) []screen.Display { d[0].Scale = 1.5; return d }},
		{"primary", func(d []screen.Display) []screen.Display { d[0].Primary = true; d[1].Primary = false; return d }},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := test.change(append([]screen.Display(nil), original...))
			if screenshotDisplayLayoutMatches(saved, current) {
				t.Fatal("changed display configuration accepted")
			}
			platform := screenshotEditorPlatform{}
			doc := screenshotDocument{Displays: saved, Frame: Size{Width: 100, Height: 100}, PixelWidth: 100, PixelHeight: 100}
			if err := restoreScreenshotDocumentDesktop(&platform, &doc, current); !errors.Is(err, ErrScreenshotDisplayLayoutChanged) || platform.setWindowBounds != nil {
				t.Fatal("changed layout configured an editor window instead of rejecting restoration")
			}
		})
	}
	if screenshotDisplayLayoutMatches(nil, original) || screenshotDisplayLayoutMatches(nil, nil) {
		t.Fatal("legacy or missing display metadata accepted")
	}
	current := append([]screen.Display(nil), original...)
	current[0].Name = "Renamed label"
	current[0].WorkArea.Height = 900
	if !screenshotDisplayLayoutMatches(saved, current) {
		t.Fatal("taskbar or display labels are not layout changes")
	}
}

func TestScreenshotDocumentRestoresOriginalDesktopAcrossMixedDPIDisplays(t *testing.T) {
	displays := []screen.Display{
		{ID: "left", Bounds: screen.Rect{X: -1920, Y: -120, Width: 1920, Height: 1080}, PixelBounds: screen.Rect{X: -1920, Y: -120, Width: 1920, Height: 1080}, Scale: 1},
		{ID: "middle", Bounds: screen.Rect{Width: 2560, Height: 1440}, PixelBounds: screen.Rect{Width: 3840, Height: 2160}, Scale: 1.5},
		{ID: "right", Bounds: screen.Rect{X: 1920, Width: 1920, Height: 1080}, PixelBounds: screen.Rect{X: 3840, Width: 3840, Height: 2160}, Scale: 2},
	}
	document := screenshotDocument{Frame: Size{Width: 9600, Height: 2280}, PixelWidth: 9600, PixelHeight: 2280,
		DesktopPixelOrigin: Point{X: -1920, Y: -120}, Displays: screenshotDisplayLayout(displays)}
	bounds, available := screenshotDocumentDesktopBounds(&document, displays, true)
	if !available || bounds != (Rect{X: -1920, Y: -120, Width: 9600, Height: 2280}) {
		t.Fatalf("original physical desktop lost: %+v, available=%v", bounds, available)
	}
	scale, offset := screenshotDocumentViewport(document.Frame, Size{Width: bounds.Width, Height: bounds.Height})
	if scale != 1 || offset != (Point{}) {
		t.Fatal("multi-monitor restore shrank or shifted the saved desktop")
	}
	for _, display := range displays {
		local := Point{X: float32(display.PixelBounds.X) - document.DesktopPixelOrigin.X,
			Y: float32(display.PixelBounds.Y) - document.DesktopPixelOrigin.Y}
		if bounds.X+offset.X+local.X*scale != float32(display.PixelBounds.X) || bounds.Y+offset.Y+local.Y*scale != float32(display.PixelBounds.Y) {
			t.Fatal("saved display pixels moved onto a different monitor")
		}
	}
	if _, available := screenshotDocumentDesktopBounds(&document, displays[:2], true); available {
		t.Fatal("removed monitor must reject editing")
	}
	// Logical platforms restore only their actual captured area, including a Retina display at a negative origin.
	document = screenshotDocument{Frame: Size{Width: 1920, Height: 1080}, PixelWidth: 3840, PixelHeight: 2160,
		DesktopPixelOrigin: Point{X: -3840, Y: -240}, Displays: screenshotDisplayLayout(displays)}
	if bounds, available := screenshotDocumentDesktopBounds(&document, displays, false); !available || bounds != (Rect{X: -1920, Y: -120, Width: 1920, Height: 1080}) {
		t.Fatalf("logical captured display moved: %+v, available=%v", bounds, available)
	}
}

func TestScreenshotDocumentRoundTripAllAnnotations(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 600, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 600; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	state := newScreenshotEditorOverlayState(ScreenshotOptions{}, nil, screenshotEditorPlatform{frameSize: Size{Width: 300, Height: 200}})
	state.selection = Rect{X: 10, Y: 10, Width: 250, Height: 170}
	state.uiScale = 1.5
	state.capturedDisplays = screenshotDisplayLayout([]screen.Display{{ID: "capture-display", Bounds: screen.Rect{Width: 300, Height: 200}, PixelBounds: screen.Rect{Width: 600, Height: 400}, Scale: 2}})
	state.desktopPixelOrigin = Point{X: -1920, Y: -1080}
	state.cursorPixel = &Point{X: 420, Y: 270}
	state.showCursor = true
	for tool := screenshotEditorToolRect; tool < screenshotEditorToolCount; tool++ {
		state.annotations = append(state.annotations, screenshotEditorAnnotation{
			tool: tool, rect: Rect{X: 30, Y: 30, Width: 80, Height: 40}, start: Point{X: 40, Y: 45}, end: Point{X: 100, Y: 100},
			arrowBend: Point{X: 15, Y: -8}, cornerRadius: 8, text: "Hello\n截图", number: 12, fontSize: 21,
			points: []Point{{X: 60, Y: 60}, {X: 90, Y: 70}}, mosaicRadius: 18, strokeRadius: 4,
			color: Color{R: 255, G: 91, B: 54, A: 255}, paintOrder: uint64(screenshotEditorToolCount - tool),
		})
	}
	before, err := exportScreenshotSelection(source, state.annotations, state.selection, state.frameSize, state.annotationScale(), state.cursorPixel, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "capture.jpg")
	if err := writeScreenshotImage(path, before); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = screenshotedit.Remove(path) })
	save, err := prepareScreenshotDocumentSave(path, source, before, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := save(); err != nil {
		t.Fatal(err)
	}
	doc, restored, cursor, marks, err := loadScreenshotDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(marks) != int(screenshotEditorToolCount)-1 || marks[0].arrowBend != state.annotations[0].arrowBend {
		t.Fatal("annotation properties lost")
	}
	if doc.DesktopPixelOrigin != state.desktopPixelOrigin {
		t.Fatal("negative desktop origin changed")
	}
	if len(doc.Displays) != 1 || doc.Displays[0] != state.capturedDisplays[0] {
		t.Fatal("captured display configuration was not persisted")
	}
	if clipboard.ImageHash(source) != clipboard.ImageHash(restored) {
		t.Fatal("source pixels changed")
	}
	after, err := exportScreenshotSelection(restored, marks, doc.Selection, doc.Frame, doc.AnnotationScale, doc.CursorPixel, doc.ShowCursor, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if before.Bounds() != after.Bounds() || !bytes.Equal(before.Pix, after.Pix) {
		t.Fatal("unchanged scene exported different pixels")
	}

	// A second save is independent of the first history entry and its archive.
	second := filepath.Join(t.TempDir(), "edited.jpg")
	state.document, state.annotations = doc, marks
	state.capturedCursor = cursor
	state.annotations[0].color = Color{G: 255, A: 255}
	if err := writeScreenshotImage(second, after); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = screenshotedit.Remove(second) })
	save, err = prepareScreenshotDocumentSave(second, restored, after, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := save(); err != nil {
		t.Fatal(err)
	}
	if err := screenshotedit.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	_, _, _, secondMarks, err := loadScreenshotDocument(second)
	if err != nil || secondMarks[0].color != state.annotations[0].color {
		t.Fatal("new history entry depends on the old scene")
	}
}

func TestScreenshotDocumentViewportAndRestoredTools(t *testing.T) {
	canvas := Size{Width: 3840, Height: 2160}
	for _, dpi := range []float32{1, 1.5, 2, 2.25} {
		surface := Size{Width: 1600 * dpi, Height: 900 * dpi}
		scale, offset := screenshotDocumentViewport(canvas, surface)
		state := newScreenshotEditorOverlayState(ScreenshotOptions{}, nil, screenshotEditorPlatform{
			document: &screenshotDocument{Frame: canvas, AnnotationScale: 1.5}, chromeScale: func(Rect) float32 { return dpi },
			frameSize: canvas, initialSelection: &Rect{X: 200, Y: 100, Width: 800, Height: 500},
		})
		state.viewportScale, state.viewportOffset = scale, offset
		state.uiScale = dpi / scale
		if state.annotationScale() != 1.5 {
			t.Fatal("display DPI changed document strokes")
		}
		mapped := state.surfaceRect(state.selection)
		if (mapped.X-offset.X)/scale != 200 || mapped.Width/scale != 800 {
			t.Fatal("viewport coordinate round trip failed")
		}
		if !state.key(KeyEvent{Key: Key("l"), Down: true}) || state.scrollingStarting {
			t.Fatal("restored scene started scrolling capture")
		}
		if state.key(KeyEvent{Key: Key("v"), Down: true}) || state.allowVideoRecording {
			t.Fatal("restored scene started recording")
		}
	}
}

func TestScreenshotDocumentRejectsInvalidGeometry(t *testing.T) {
	document := screenshotDocument{Frame: Size{Width: 100, Height: 80}, PixelWidth: 200, PixelHeight: 160,
		Selection: Rect{Width: 90, Height: 70}, AnnotationScale: 1}
	if _, err := document.annotations(image.Rect(0, 0, 200, 160)); err != nil {
		t.Fatal(err)
	}
	document.Selection.X = 80
	if _, err := document.annotations(image.Rect(0, 0, 200, 160)); err == nil {
		t.Fatal("out-of-bounds crop accepted")
	}
	document.Selection.X = 0
	document.Annotations = []screenshotSavedAnnotation{{Tool: "unknown"}}
	if _, err := document.annotations(image.Rect(0, 0, 200, 160)); err == nil {
		t.Fatal("unknown tool accepted")
	}
	var malformed screenshotDocument
	if err := json.Unmarshal([]byte(`{"AnnotationScale":1e100}`), &malformed); err == nil {
		t.Fatal("overflowing scalar accepted")
	}
}
