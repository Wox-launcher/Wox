package screenshot

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"testing"
	"time"

	"wox/util/screenshotedit"
)

// TestScreenshotRegionBackground covers freeform gestures, proportional bounds, export and editable history.
func TestScreenshotRegionBackground(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			frame := Size{Width: 800, Height: 600}
			// A non-RGBA capture with a negative origin exercises borrowed-image conversion and independent axes.
			source := image.NewNRGBA(image.Rect(-100, -40, int(frame.Width*scale)-100, int(frame.Height*scale)-39))
			originalColor := color.RGBA{R: 180, G: 100, B: 70, A: 255}
			draw.Draw(source, source.Bounds(), image.NewUniform(originalColor), image.Point{}, draw.Src)
			preview, err := newScreenshotEditorImage(source)
			if err != nil {
				t.Fatal(err)
			}
			state := newScreenshotEditorOverlayState(ScreenshotOptions{}, preview, screenshotEditorPlatform{
				frameSize: frame, chromeScale: func(Rect) float32 { return scale },
				captureWindow: func(Rect) (*image.RGBA, error) { panic("freeform drag requested native window pixels") },
			})
			state.originalSource = source
			wallpaper := image.NewRGBA(image.Rect(0, 0, 100, 80))
			draw.Draw(wallpaper, wallpaper.Bounds(), image.NewUniform(color.RGBA{B: 255, A: 255}), image.Point{}, draw.Src)
			state.backgroundWallpaper = preloadScreenshotWallpaper(func(context.Context) (image.Image, error) { return wallpaper, nil })
			t.Cleanup(state.releaseWindowBackground)
			state.toggleWindowBackground()
			if state.showBackground {
				t.Fatal("background enabled before a selection existed")
			}
			start, end := Point{X: 200, Y: 160}, Point{X: 400, Y: 260}
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: start})
			state.pointer(PointerEvent{Kind: PointerMove, Position: end})
			state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: end})
			selection := Rect{X: start.X, Y: start.Y, Width: end.X - start.X, Height: end.Y - start.Y}
			state.mu.Lock()
			done := state.backgroundDone
			state.mu.Unlock()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("freeform selection did not prepare the preloaded background")
			}
			if state.selection != selection || state.windowSelection != nil || state.windowSource != nil || state.showBackground || state.backgroundSource == nil {
				t.Fatal("freeform preparation changed selection, enabled the effect, or installed native alpha")
			}
			state.draw(&DisplayList{}, FrameInfo{Size: frame})
			button := state.backgroundRect
			if button.Width != 40*scale || button.X < 0 || button.X+button.Width > frame.Width {
				t.Fatalf("freeform background button has invalid bounds: %+v", button)
			}
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: button.X + button.Width/2, Y: button.Y + button.Height/2}})
			if !state.showBackground || state.image != preview || state.selection != selection {
				t.Fatal("background toggle changed frozen capture or underlying crop")
			}
			outer := state.selectionBoundsLocked()
			if outer.X >= selection.X || outer.Y >= selection.Y || outer.Width <= selection.Width || outer.Height <= selection.Height {
				t.Fatal("selection chrome did not expand to include wallpaper padding")
			}
			if !state.openSizeDialog() {
				t.Fatal("background dimensions cannot be edited")
			}
			state.activeSizeDialog().apply()
			if !state.showBackground || state.selection != selection {
				t.Fatal("unchanged outer dimensions cancelled the background")
			}
			clip, _ := screenshotEditorPixelSelection(source.Bounds(), selection, frame)
			cropped, err := exportScreenshotSelection(source, nil, selection, frame, scale, nil, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			output := composeScreenshotWindowBackground(cropped, state.backgroundSource)
			padding := output.Bounds().Size().Sub(clip.Size()).Div(2)
			if output.RGBAAt(0, 0).A != 0 || output.RGBAAt(padding.X, padding.Y) != originalColor {
				t.Fatal("export lost outer rounding or modified the rectangular capture's corners")
			}
			path := filepath.Join(t.TempDir(), "region.png")
			if err := writeScreenshotImage(path, output); err != nil {
				t.Fatal(err)
			}
			save, err := prepareScreenshotDocumentSave(path, source, output, state)
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
			if !document.ShowBackground || document.backgroundPixels == nil || document.windowPixels != nil || document.WindowSelection != nil {
				t.Fatal("history lost freeform background or misclassified it as a native window")
			}
			restoredCrop, err := exportScreenshotSelection(restored, marks, document.Selection, document.Frame, document.AnnotationScale, nil, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := composeScreenshotWindowBackground(restoredCrop, document.backgroundPixels); got.Bounds() != output.Bounds() || got.RGBAAt(padding.X, padding.Y) != originalColor || got.RGBAAt(0, 0).A != 0 {
				t.Fatal("editable history changed the freeform background composition")
			}
			state.key(KeyEvent{Key: KeySpace, Down: true})
			if state.showBackground || state.selectionBoundsLocked() != selection {
				t.Fatal("keyboard toggle did not restore the crop bounds")
			}
			state.key(KeyEvent{Key: KeySpace, Down: true})
			state.chooseSavePath = func() (string, error) { return filepath.Join(t.TempDir(), "download"), nil }
			state.requestSave()
			if outcome := <-state.result; filepath.Ext(outcome.saveAsPath) != ".png" {
				t.Fatal("background download did not default to PNG")
			}
			state.mu.Lock()
			state.setSelectionLocked(Rect{X: 210, Y: 160, Width: 200, Height: 100})
			state.backgroundWallpaper = preloadScreenshotWallpaper(func(context.Context) (image.Image, error) { return wallpaper, nil })
			state.mu.Unlock()
			if state.showBackground || state.backgroundSource != nil || state.backgroundPreview != nil {
				t.Fatal("selection change retained the old background")
			}
			state.toggleWindowBackground()
			state.mu.Lock()
			done = state.backgroundDone
			state.mu.Unlock()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("changed freeform selection cannot enable a new background")
			}
			if !state.showBackground || state.backgroundSource == nil || state.windowSelection != nil {
				t.Fatal("changed freeform selection lost background support")
			}
		})
	}
}

// TestScreenshotRegionBackgroundCancellation rejects a decoded result after selection changes or editor shutdown.
func TestScreenshotRegionBackgroundCancellation(t *testing.T) {
	for _, exit := range []bool{false, true} {
		t.Run(fmt.Sprint(exit), func(t *testing.T) {
			selection := Rect{X: 20, Y: 20, Width: 40, Height: 30}
			source := image.NewNRGBA(image.Rect(0, 0, 100, 100))
			state := newScreenshotEditorOverlayState(ScreenshotOptions{}, nil, screenshotEditorPlatform{frameSize: Size{Width: 100, Height: 100}, initialSelection: &selection})
			state.originalSource = source
			resolver := make(chan struct{})
			load := preloadScreenshotWallpaper(func(context.Context) (image.Image, error) {
				<-resolver
				return image.NewRGBA(image.Rect(0, 0, 50, 50)), nil
			})
			state.backgroundWallpaper = load
			t.Cleanup(state.releaseWindowBackground)
			state.toggleWindowBackground()
			done := state.backgroundDone
			if exit {
				state.releaseWindowBackground()
			} else {
				state.mu.Lock()
				state.setSelectionLocked(Rect{X: 21, Y: 20, Width: 40, Height: 30})
				state.mu.Unlock()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("abandoned freeform preparation still owns capture pixels")
			}
			state.releaseWindowBackground()
			close(resolver)
			<-load.done
			if state.showBackground || state.backgroundSource != nil || state.backgroundPreview != nil || state.backgroundWallpaper != nil || load.pixels != nil {
				t.Fatal("cancelled region retained or republished background resources")
			}
		})
	}
}
