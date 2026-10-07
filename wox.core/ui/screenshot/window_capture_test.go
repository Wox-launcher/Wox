package screenshot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wox/util/screenshotedit"
)

// TestScreenshotWindowAlphaPackedRows covers cropped strides, negative origins, and the generic image fallback.
func TestScreenshotWindowAlphaPackedRows(t *testing.T) {
	parent := image.NewRGBA(image.Rect(-20, -10, 40, 30))
	for y := parent.Rect.Min.Y; y < parent.Rect.Max.Y; y++ {
		for x := parent.Rect.Min.X; x < parent.Rect.Max.X; x++ {
			alpha := uint8((x - parent.Rect.Min.X + y - parent.Rect.Min.Y) * 7)
			parent.SetRGBA(x, y, color.RGBA{R: alpha / 2, G: alpha / 3, B: alpha / 4, A: alpha})
		}
	}
	source := parent.SubImage(image.Rect(-11, -4, 25, 22)).(*image.RGBA)
	selection := Rect{X: 3, Y: 2, Width: 9, Height: 7}
	frame := Size{Width: 18, Height: 13}
	clip, err := screenshotEditorPixelSelection(source.Bounds(), selection, frame)
	if err != nil {
		t.Fatal(err)
	}
	want := image.NewRGBA(image.Rect(8, -12, 8+clip.Dx(), -12+clip.Dy()))
	for i := 0; i < len(want.Pix); i += 4 {
		want.Pix[i], want.Pix[i+1], want.Pix[i+2], want.Pix[i+3] = 80, 40, 20, 160
	}
	got := image.NewRGBA(want.Rect)
	copy(got.Pix, want.Pix)
	// Wrapping the raster hides its concrete type and exercises the original per-pixel semantics.
	clipScreenshotWindowAlpha(want, struct{ image.Image }{source}, selection, frame)
	clipScreenshotWindowAlpha(got, source, selection, frame)
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("packed alpha clipping differs from generic image semantics")
	}
}

// screenshotImageHasTransparency checks capture resources without assuming a decoded raster type.
func screenshotImageHasTransparency(pixels image.Image) bool {
	if opaque, ok := pixels.(interface{ Opaque() bool }); ok {
		return !opaque.Opaque()
	}
	for y := pixels.Bounds().Min.Y; y < pixels.Bounds().Max.Y; y++ {
		for x := pixels.Bounds().Min.X; x < pixels.Bounds().Max.X; x++ {
			if _, _, _, alpha := pixels.At(x, y).RGBA(); alpha != 65535 {
				return true
			}
		}
	}
	return false
}

// TestScreenshotWindowAlphaSurvivesExportAndScene verifies native alpha through annotation, PNG, and editable history.
func TestScreenshotWindowAlphaSurvivesExportAndScene(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprintf("scale=%v", scale), func(t *testing.T) {
			desktop := image.NewRGBA(image.Rect(0, 0, int(120*scale), int(100*scale)))
			draw.Draw(desktop, desktop.Bounds(), image.NewUniform(color.RGBA{R: 240, G: 180, B: 90, A: 255}), image.Point{}, draw.Src)
			selection := Rect{X: 20, Y: 20, Width: 40, Height: 40}
			pixels := image.NewRGBA(image.Rect(0, 0, int(40*scale), int(40*scale)))
			draw.Draw(pixels, pixels.Bounds(), image.NewUniform(color.RGBA{R: 60, G: 80, B: 100, A: 255}), image.Point{}, draw.Src)
			pixels.SetRGBA(0, 0, color.RGBA{})
			pixels.SetRGBA(1, 0, color.RGBA{R: 40, G: 20, B: 10, A: 128})
			preview, err := newScreenshotEditorImage(desktop)
			if err != nil {
				t.Fatal(err)
			}
			state := newScreenshotEditorOverlayState(ScreenshotOptions{}, preview, screenshotEditorPlatform{frameSize: Size{Width: 120, Height: 100}})
			state.originalSource = desktop
			state.selection, state.hasSelection = selection, true
			if err := state.installWindowCapture(selection, pixels); err != nil {
				t.Fatal(err)
			}
			if state.image != preview || !desktop.Opaque() {
				t.Fatal("window capture changed the editing preview or desktop pixels")
			}
			state.annotations = []screenshotEditorAnnotation{{tool: screenshotEditorToolBrush,
				points: []Point{{X: 20, Y: 20}, {X: 30, Y: 20}}, strokeRadius: 3, color: Color{R: 255, A: 255}}}
			output, err := exportScreenshotSelection(state.windowSource, state.annotations, selection, state.frameSize, 1, nil, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			clipScreenshotWindowAlpha(output, state.windowSource, selection, state.frameSize)
			corner := output.Rect.Min
			if output.RGBAAt(corner.X, corner.Y) != (color.RGBA{}) {
				t.Fatal("annotation filled a transparent corner")
			}
			if output.RGBAAt(corner.X+1, corner.Y).A != 128 {
				t.Fatal("native edge alpha was multiplied twice")
			}
			if !screenshotImageHasTransparency(output) {
				t.Fatal("transparent capture chose an opaque format")
			}
			path := filepath.Join(t.TempDir(), "window.png")
			if err := writeScreenshotImage(path, output); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			decoded, format, err := image.Decode(file)
			file.Close()
			if err != nil || format != "png" {
				t.Fatalf("PNG export: format=%q err=%v", format, err)
			}
			if _, _, _, alpha := decoded.At(0, 0).RGBA(); alpha != 0 {
				t.Fatal("PNG flattened transparent corner")
			}
			save, err := prepareScreenshotDocumentSave(path, desktop, output, state)
			if err != nil {
				t.Fatal(err)
			}
			if err := save(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = screenshotedit.Remove(path) })
			document, restored, _, marks, err := loadScreenshotDocument(path)
			if err != nil {
				t.Fatal(err)
			}
			if document.WindowSelection == nil || *document.WindowSelection != selection {
				t.Fatal("editable scene lost native window geometry")
			}
			if screenshotImageHasTransparency(restored) || document.windowPixels == nil {
				t.Fatal("editable scene lost original background or native window pixels")
			}
			restoredPreview, err := newScreenshotEditorImage(restored)
			if err != nil {
				t.Fatal(err)
			}
			restoredState := newScreenshotEditorOverlayState(ScreenshotOptions{}, restoredPreview,
				screenshotEditorPlatform{frameSize: document.Frame, initialSelection: &document.Selection, document: document, restoredAnnotations: marks})
			restoredState.originalSource = restored
			if err := restoredState.installWindowCapture(document.Selection, document.windowPixels); err != nil {
				t.Fatal(err)
			}
			if restoredState.image != restoredPreview {
				t.Fatal("re-edit changed the original desktop preview")
			}
			reedited, err := exportScreenshotSelection(restoredState.windowSource, marks, document.Selection, document.Frame, document.AnnotationScale, nil, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			clipScreenshotWindowAlpha(reedited, restoredState.windowSource, document.Selection, document.Frame)
			if _, _, _, alpha := reedited.At(reedited.Bounds().Min.X, reedited.Bounds().Min.Y).RGBA(); alpha != 0 {
				t.Fatal("re-edit filled transparent corner")
			}
			clip, _ := screenshotEditorPixelSelection(restoredState.windowSource.Bounds(), selection, document.Frame)
			if restoredState.windowSource.RGBAAt(clip.Min.X+1, clip.Min.Y) != pixels.RGBAAt(1, 0) {
				t.Fatal("scene rounded native premultiplied edge colors")
			}
			restoredState.setSelectionLocked(Rect{X: 10, Y: 10, Width: 60, Height: 60})
			freeform, err := exportScreenshotSelection(restored, marks, restoredState.selection, document.Frame, document.AnnotationScale, nil, false, nil)
			if err != nil || screenshotImageHasTransparency(freeform) || restoredState.windowSource != nil {
				t.Fatal("re-edit retained transparency after changing selection")
			}
		})
	}
}

// TestScreenshotWindowCaptureRejectsAbandonedGesture prevents a slow native frame from replacing a subsequent freeform selection.
func TestScreenshotWindowCaptureRejectsAbandonedGesture(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	selection := Rect{X: 20, Y: 20, Width: 40, Height: 40}
	state := &screenshotEditorOverlayState{frameSize: Size{Width: 120, Height: 100}, selection: selection, hasSelection: true,
		originalSource: image.NewRGBA(image.Rect(0, 0, 120, 100)),
		captureWindow: func(Rect) (*image.RGBA, error) {
			close(started)
			<-release
			return image.NewRGBA(image.Rect(0, 0, 40, 40)), nil
		},
	}
	done := make(chan struct{})
	go state.captureSelectedWindow(selection, 0, done, false)
	<-started
	state.mu.Lock()
	state.setSelectionLocked(Rect{X: 25, Y: 20, Width: 40, Height: 40})
	// Returning to the same geometry must not revive a frame from the abandoned capture.
	state.setSelectionLocked(selection)
	state.mu.Unlock()
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("native capture did not finish")
	}
	if state.windowSource != nil || state.windowSelection != nil {
		t.Fatal("abandoned native frame replaced current source")
	}
}

// TestScreenshotFreeformSelectionRestoresDesktop checks that native transparent pixels do not leak into another capture.
func TestScreenshotFreeformSelectionRestoresDesktop(t *testing.T) {
	desktop := image.NewRGBA(image.Rect(0, 0, 120, 100))
	draw.Draw(desktop, desktop.Bounds(), image.NewUniform(color.RGBA{A: 255}), image.Point{}, draw.Src)
	preview, err := newScreenshotEditorImage(desktop)
	if err != nil {
		t.Fatal(err)
	}
	state := newScreenshotEditorOverlayState(ScreenshotOptions{}, preview, screenshotEditorPlatform{frameSize: Size{Width: 120, Height: 100}})
	state.originalSource = desktop
	selection := Rect{X: 20, Y: 20, Width: 40, Height: 40}
	state.selection, state.hasSelection = selection, true
	if err := state.installWindowCapture(selection, image.NewRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 90, Y: 80}})
	if state.windowSource != nil || state.windowSelection != nil || state.image != preview {
		t.Fatal("freeform selection retained native window pixels")
	}
}

// TestScreenshotWindowSelectionEditsCancelAlpha covers the real pointer and dimension-dialog paths.
func TestScreenshotWindowSelectionEditsCancelAlpha(t *testing.T) {
	for _, scenario := range []string{"click", "move", "resize", "move back", "same size", "change size", "invalid size", "annotation"} {
		t.Run(scenario, func(t *testing.T) {
			desktop := image.NewRGBA(image.Rect(0, 0, 800, 600))
			draw.Draw(desktop, desktop.Bounds(), image.NewUniform(color.RGBA{A: 255}), image.Point{}, draw.Src)
			preview, err := newScreenshotEditorImage(desktop)
			if err != nil {
				t.Fatal(err)
			}
			selection := Rect{X: 200, Y: 200, Width: 200, Height: 200}
			state := newScreenshotEditorOverlayState(ScreenshotOptions{}, preview, screenshotEditorPlatform{frameSize: Size{Width: 800, Height: 600}, initialSelection: &selection})
			state.originalSource = desktop
			if err := state.installWindowCapture(selection, image.NewRGBA(image.Rect(0, 0, 200, 200))); err != nil {
				t.Fatal(err)
			}
			cancelled := scenario == "move" || scenario == "resize" || scenario == "move back" || scenario == "change size"
			switch scenario {
			case "same size", "change size", "invalid size":
				if !state.openSizeDialog() {
					t.Fatal("size dialog did not open")
				}
				dialog := state.activeSizeDialog()
				if scenario == "change size" {
					dialog.width.SetText("210", true)
				} else if scenario == "invalid size" {
					dialog.width.SetText("0", true)
				}
				dialog.apply()
			case "annotation":
				state.activeTool = screenshotEditorToolRect
				state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 250, Y: 250}})
				state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 300, Y: 300}})
				state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 300, Y: 300}})
				if len(state.annotations) != 1 {
					t.Fatal("annotation was not committed")
				}
			default:
				start, end := Point{X: 280, Y: 280}, Point{X: 280, Y: 280}
				if scenario == "resize" {
					start, end = Point{X: 400, Y: 400}, Point{X: 420, Y: 420}
				} else if scenario != "click" {
					end = Point{X: 300, Y: 300}
				}
				state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: start})
				state.pointer(PointerEvent{Kind: PointerMove, Position: end})
				if scenario == "move back" {
					end = start
					state.pointer(PointerEvent{Kind: PointerMove, Position: end})
				}
				state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: end})
			}
			if (state.windowSource == nil) != cancelled || (state.windowSelection == nil) != cancelled {
				t.Fatalf("selection edit retained/cancelled alpha incorrectly: selection=%+v cancelled=%v", state.selection, cancelled)
			}
			if state.image != preview || !desktop.Opaque() {
				t.Fatal("selection edit changed the frozen desktop preview")
			}
		})
	}
}
