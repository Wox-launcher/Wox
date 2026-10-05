package screenshot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"wox/util/screenshotedit"
)

// TestScreenshotWindowBackgroundComposition checks centered native pixels and single alpha blending across capture scales.
func TestScreenshotWindowBackgroundComposition(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			frame := Size{Width: 800, Height: 600}
			bounds := image.Rect(-100, -40, int(800*scale)-100, int(600*scale)-40)
			selection := Rect{X: 0, Y: 0, Width: 400, Height: 200}
			clip, err := screenshotEditorPixelSelection(bounds, selection, frame)
			if err != nil {
				t.Fatal(err)
			}
			window := image.NewRGBA(clip)
			draw.Draw(window, window.Bounds(), image.NewUniform(color.RGBA{R: 200, G: 100, B: 60, A: 255}), image.Point{}, draw.Src)
			window.SetRGBA(clip.Min.X, clip.Min.Y, color.RGBA{})
			window.SetRGBA(clip.Min.X+1, clip.Min.Y, color.RGBA{R: 80, A: 128})
			padding := screenshotWindowBackgroundPadding(bounds, selection, frame, 1)
			if padding != image.Pt(int(64*scale), int(32*scale)) {
				t.Fatalf("physical padding=%v", padding)
			}
			wallpaper := image.NewUniform(color.RGBA{R: 20, G: 40, B: 80, A: 255})
			// A finite wallpaper avoids sampling an unbounded uniform in the aspect-fill resizer.
			finite := image.NewRGBA(image.Rect(0, 0, 400, 300))
			draw.Draw(finite, finite.Bounds(), wallpaper, image.Point{}, draw.Src)
			background := fitScreenshotWallpaper(finite, clip.Size().Add(padding.Mul(2)))
			output := composeScreenshotWindowBackground(window, background)
			if output.Opaque() || output.Bounds().Size() != clip.Size().Add(padding.Mul(2)) {
				t.Fatal("background output dimensions or alpha are wrong")
			}
			for _, corner := range []image.Point{output.Bounds().Min, {X: output.Rect.Max.X - 1}, {Y: output.Rect.Max.Y - 1}, output.Rect.Max.Sub(image.Pt(1, 1))} {
				if got := output.RGBAAt(corner.X, corner.Y); got != (color.RGBA{}) {
					t.Fatalf("background corner is not transparent: %v=%+v", corner, got)
				}
			}
			if !background.Opaque() {
				t.Fatal("rounded composition mutated the stored wallpaper")
			}
			if output.RGBAAt(padding.X, padding.Y) != (color.RGBA{R: 20, G: 40, B: 80, A: 255}) {
				t.Fatal("desktop pixels leaked into the native transparent corner")
			}
			if edge := output.RGBAAt(padding.X+1, padding.Y); edge != (color.RGBA{R: 90, G: 19, B: 39, A: 255}) {
				t.Fatalf("native edge was blended more than once: %+v", edge)
			}
			if output.RGBAAt(padding.X+2, padding.Y) != window.RGBAAt(clip.Min.X+2, clip.Min.Y) {
				t.Fatal("background composition rescaled window pixels")
			}
			source := image.NewRGBA(bounds)
			draw.Draw(source, bounds, image.NewUniform(color.RGBA{R: 200, G: 100, B: 60, A: 255}), image.Point{}, draw.Src)
			draw.Draw(source, clip, window, clip.Min, draw.Src)
			preview := composeScreenshotWindowBackgroundPreview(source, clip, background)
			if preview.Bounds() != bounds || !preview.Opaque() || preview.RGBAAt(clip.Min.X+1, clip.Min.Y) != output.RGBAAt(padding.X+1, padding.Y) {
				t.Fatal("combined preview shifted the window or changed native alpha blending")
			}
			if corner := preview.RGBAAt(bounds.Max.X-1, bounds.Max.Y-1); corner != (color.RGBA{R: 107, G: 53, B: 32, A: 255}) {
				t.Fatalf("combined preview did not dim the surrounding desktop once: %+v", corner)
			}
			if got := screenshotWindowBackgroundBounds(selection, background, bounds, frame); got != (Rect{X: -64, Y: -32, Width: 528, Height: 264}) {
				t.Fatalf("preview padded bounds=%+v", got)
			}
		})
	}
	// Windows uses physical editor coordinates while macOS can use points. Both represent the same inset.
	if got := screenshotWindowBackgroundPadding(image.Rect(0, 0, 1600, 900), Rect{Width: 800, Height: 400}, Size{Width: 1600, Height: 900}, 2); got != image.Pt(128, 64) {
		t.Fatalf("Windows DPI padding=%v", got)
	}
}

// TestScreenshotWindowBackgroundPaddingAspectRatio covers both orientations, square windows and independent pixel scales.
func TestScreenshotWindowBackgroundPaddingAspectRatio(t *testing.T) {
	for _, dimensions := range []image.Point{{X: 400, Y: 200}, {X: 200, Y: 400}, {X: 300, Y: 300}, {X: 2000, Y: 1468}} {
		selection := Rect{Width: float32(dimensions.X), Height: float32(dimensions.Y)}
		padding := screenshotWindowBackgroundPadding(image.Rect(0, 0, 2400, 1600), selection, Size{Width: 2400, Height: 1600}, 1)
		// Rounding either physical edge outward can introduce at most one pixel of aspect error.
		if difference := padding.X*dimensions.Y - padding.Y*dimensions.X; difference > dimensions.X+dimensions.Y || difference < -dimensions.X-dimensions.Y {
			t.Fatalf("padding lost window aspect ratio: window=%v padding=%v", dimensions, padding)
		}
		if dimensions.X > dimensions.Y && padding.X <= padding.Y || dimensions.Y > dimensions.X && padding.Y <= padding.X {
			t.Fatalf("longer window edge did not receive a wider inset: window=%v padding=%v", dimensions, padding)
		}
	}
	if padding := screenshotWindowBackgroundPadding(image.Rect(0, 0, 2400, 1600), Rect{Width: 400, Height: 200}, Size{Width: 1200, Height: 1000}, 1); padding != image.Pt(128, 52) {
		t.Fatalf("nonuniform capture axes used the same pixel conversion: %v", padding)
	}
}

// TestScreenshotWindowBackgroundOuterSelection keeps chrome, hit testing and size input on the full output bounds.
func TestScreenshotWindowBackgroundOuterSelection(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for _, scenario := range []string{"click", "move", "resize", "same size", "change size", "off-screen click", "toggle off"} {
			t.Run(fmt.Sprintf("scale=%v/%s", scale, scenario), func(t *testing.T) {
				frame := Size{Width: 1200, Height: 800}
				desktop := image.NewRGBA(image.Rect(0, 0, int(frame.Width*scale), int(frame.Height*scale)))
				preview, err := newScreenshotEditorImage(desktop)
				if err != nil {
					t.Fatal(err)
				}
				selection := Rect{X: 200, Y: 200, Width: 400, Height: 200}
				if scenario == "off-screen click" {
					selection.X, selection.Y = 0, 0
				}
				state := newScreenshotEditorOverlayState(ScreenshotOptions{}, preview, screenshotEditorPlatform{frameSize: frame, initialSelection: &selection})
				state.originalSource = desktop
				clip, _ := screenshotEditorPixelSelection(desktop.Bounds(), selection, frame)
				window := image.NewRGBA(image.Rectangle{Max: clip.Size()})
				if err := state.installWindowCapture(selection, window); err != nil {
					t.Fatal(err)
				}
				padding := screenshotWindowBackgroundPadding(desktop.Bounds(), selection, frame, 1)
				state.backgroundSource = fitScreenshotWallpaper(desktop, clip.Size().Add(padding.Mul(2)))
				state.backgroundPreview, err = newScreenshotEditorImage(composeScreenshotWindowBackgroundPreview(state.windowSource, clip, state.backgroundSource))
				if err != nil {
					t.Fatal(err)
				}
				state.toggleWindowBackground()
				state.draw(&DisplayList{}, FrameInfo{Size: frame})
				outer := state.selectionBoundsLocked()
				want := Rect{X: selection.X - 64, Y: selection.Y - 32, Width: 528, Height: 264}
				if outer != want {
					t.Fatalf("outer selection=%+v want=%+v", outer, want)
				}
				if !state.openSizeDialog() {
					t.Fatal("outer selection size dialog did not open")
				}
				dialog := state.activeSizeDialog()
				if dialog.width.Text() != strconv.Itoa(state.backgroundSource.Rect.Dx()) || dialog.height.Text() != strconv.Itoa(state.backgroundSource.Rect.Dy()) {
					t.Fatal("size dialog reports the inner window instead of the outer canvas")
				}
				dialog.setRatioLocked(true)
				if dialog.aspectRatio != 2 {
					t.Fatal("size ratio ignored the background canvas")
				}
				if scenario == "change size" {
					dialog.width.SetText(strconv.Itoa(state.backgroundSource.Rect.Dx()+20), false)
				}
				dialog.apply()
				if scenario != "change size" {
					if !state.showBackground || state.selection != selection {
						t.Fatal("applying unchanged outer dimensions cancelled window background")
					}
				}
				switch scenario {
				case "click", "resize", "off-screen click", "move":
					start := Point{X: outer.X + outer.Width, Y: outer.Y + outer.Height}
					if scenario == "move" {
						start = Point{X: outer.X + 20, Y: outer.Y + outer.Height/2}
					}
					state.pointer(PointerEvent{Kind: PointerMove, Position: start})
					if scenario != "move" && state.pointerCursor != PointerCursorResizeNWSE {
						t.Fatal("outer corner did not expose its resize cursor")
					}
					state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: start})
					if state.editMode != screenshotEditorEditResizeSelection && state.editMode != screenshotEditorEditMoveSelection {
						t.Fatal("outer frame started a new region rather than editing its selection")
					}
					end := start
					if scenario == "move" || scenario == "resize" {
						end.X += 10
						end.Y += 5
					}
					state.pointer(PointerEvent{Kind: PointerMove, Position: end})
					state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: end})
				case "toggle off":
					state.toggleWindowBackground()
					if state.selectionBoundsLocked() != selection || state.windowSource == nil {
						t.Fatal("background toggle failed to restore the inner window frame")
					}
				}
				changed := scenario == "move" || scenario == "resize" || scenario == "change size"
				if changed && (state.showBackground || state.windowSelection != nil || state.backgroundSource != nil) {
					t.Fatal("changing the outer selection retained background or native alpha")
				}
				if !changed && scenario != "toggle off" && (!state.showBackground || state.selection != selection) {
					t.Fatal("clicking the outer selection changed the window capture")
				}
			})
		}
	}
}

// TestScreenshotWindowBackgroundToggle exercises toolbar wrapping, pointer activation, keyboard toggling and cancellation.
func TestScreenshotWindowBackgroundToggle(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			desktop := image.NewRGBA(image.Rect(0, 0, 800, 600))
			preview, err := newScreenshotEditorImage(desktop)
			if err != nil {
				t.Fatal(err)
			}
			selection := Rect{X: 100, Y: 100, Width: 200, Height: 100}
			state := newScreenshotEditorOverlayState(ScreenshotOptions{}, preview, screenshotEditorPlatform{frameSize: Size{Width: 800, Height: 600}, initialSelection: &selection, chromeScale: func(Rect) float32 { return scale }})
			state.originalSource = desktop
			if err := state.installWindowCapture(selection, image.NewRGBA(image.Rect(0, 0, 200, 100))); err != nil {
				t.Fatal(err)
			}
			state.backgroundSource = image.NewRGBA(image.Rect(0, 0, 264, 164))
			state.backgroundPreview, err = newScreenshotEditorImage(composeScreenshotWindowBackgroundPreview(state.windowSource, image.Rect(100, 100, 300, 200), state.backgroundSource))
			if err != nil {
				t.Fatal(err)
			}
			state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
			button := state.backgroundRect
			if button.Width != 40*scale || button.X < 0 || button.X+button.Width > 800 || button.Y+button.Height > 600 {
				t.Fatalf("background button lost scaled hit target or wrapped off-screen: %+v", button)
			}
			position := Point{X: button.X + button.Width/2, Y: button.Y + button.Height/2}
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: position})
			if !state.showBackground || state.selection != selection || state.image != preview {
				t.Fatal("toolbar toggle altered selection or frozen desktop")
			}
			state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
			state.key(KeyEvent{Key: Key("d"), Down: true})
			if state.showBackground || state.backgroundSource == nil {
				t.Fatal("keyboard toggle did not retain reusable wallpaper")
			}
			state.backgroundLoading = true
			state.showBackground = true
			if state.openSizeDialog() {
				t.Fatal("size draft opened before the outer canvas dimensions were ready")
			}
			state.toggleWindowBackground()
			if state.showBackground {
				t.Fatal("pending background request could not be cancelled")
			}
			state.backgroundLoading = false
			state.toggleWindowBackground()
			state.setSelectionLocked(Rect{X: 110, Y: 100, Width: 200, Height: 100})
			state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
			if state.showBackground || state.backgroundSource != nil || state.backgroundPreview != nil || state.backgroundRect != (Rect{}) {
				t.Fatal("selection change retained window background")
			}
		})
	}
}

// TestScreenshotWindowBackgroundPreload verifies early decoding, eager preparation and stale-result rejection.
func TestScreenshotWindowBackgroundPreload(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for _, scenario := range []string{"idle", "requested", "toggle off", "selection changed", "closed", "failure"} {
			t.Run(fmt.Sprintf("%v/%s", scale, scenario), func(t *testing.T) {
				desktop := image.NewRGBA(image.Rect(0, 0, 250, 188))
				preview, err := newScreenshotEditorImage(desktop)
				if err != nil {
					t.Fatal(err)
				}
				selection := Rect{X: 20, Y: 20, Width: 60, Height: 40}
				frame := Size{Width: 200, Height: 150}
				state := newScreenshotEditorOverlayState(ScreenshotOptions{}, preview, screenshotEditorPlatform{
					frameSize: frame, initialSelection: &selection, chromeScale: func(Rect) float32 { return scale },
					captureWindow: func(Rect) (*image.RGBA, error) {
						return image.NewRGBA(image.Rect(0, 0, 75, 51)), nil
					},
				})
				state.originalSource = desktop
				started, release := make(chan struct{}), make(chan struct{})
				var loads atomic.Int32
				state.backgroundWallpaper = preloadScreenshotWallpaper(func(ctx context.Context) (image.Image, error) {
					loads.Add(1)
					close(started)
					select {
					case <-release:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
					if scenario == "failure" {
						return nil, errors.New("wallpaper unavailable")
					}
					wallpaper := image.NewRGBA(image.Rect(0, 0, 320, 200))
					draw.Draw(wallpaper, wallpaper.Bounds(), image.NewUniform(color.RGBA{R: 40, G: 50, B: 60, A: 255}), image.Point{}, draw.Src)
					return wallpaper, nil
				})
				t.Cleanup(state.releaseWindowBackground)
				select {
				case <-started:
				case <-time.After(time.Second):
					t.Fatal("wallpaper decode did not start before selecting a window")
				}
				state.captureSelectedWindow(selection, 0, make(chan struct{}), false)
				state.mu.Lock()
				done := state.backgroundDone
				loading, enabled := state.backgroundLoading, state.showBackground
				state.mu.Unlock()
				if done == nil || !loading || enabled {
					t.Fatal("native capture did not prepare the background without enabling it")
				}
				if scenario != "idle" {
					state.toggleWindowBackground()
				}
				switch scenario {
				case "toggle off":
					state.toggleWindowBackground()
				case "selection changed":
					state.mu.Lock()
					state.setSelectionLocked(Rect{X: 21, Y: 20, Width: 60, Height: 40})
					state.mu.Unlock()
				case "closed":
					state.releaseWindowBackground()
				}
				close(release)
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("background preparation did not finish")
				}
				if scenario == "selection changed" || scenario == "closed" || scenario == "failure" {
					if state.backgroundSource != nil || state.backgroundPreview != nil {
						t.Fatal("failed or abandoned preparation installed a background")
					}
					if scenario == "failure" && (!state.backgroundFailed || state.showBackground || state.backgroundWallpaper != nil) {
						t.Fatal("failed load could not be retried")
					}
					return
				}
				clip, _ := screenshotEditorPixelSelection(desktop.Bounds(), selection, frame)
				padding := screenshotWindowBackgroundPadding(desktop.Bounds(), selection, frame, scale)
				if state.backgroundSource == nil || state.backgroundSource.Bounds().Size() != clip.Size().Add(padding.Mul(2)) || state.image != preview || state.selection != selection {
					t.Fatal("preparation changed editing pixels or used the wrong display scale")
				}
				if state.showBackground != (scenario == "requested") || state.backgroundWallpaper != nil {
					t.Fatal("preparation enabled an unrequested effect or retained the full wallpaper")
				}
				background := state.backgroundSource
				for i := 0; i < 10; i++ {
					state.toggleWindowBackground()
				}
				if loads.Load() != 1 || state.backgroundSource != background || state.backgroundDone != done {
					t.Fatal("repeated toggles reloaded or recomposed a prepared background")
				}
				if state.backgroundPreview.Width != desktop.Bounds().Dx() || state.backgroundPreview.Height != desktop.Bounds().Dy() {
					t.Fatal("prepared preview did not include the full editor canvas")
				}
				state.showBackground, state.autoConfirm = true, true
				list := &DisplayList{}
				state.draw(list, FrameInfo{Size: frame})
				if list.ImageDrawCount() != 1 {
					t.Fatalf("background frame uploaded competing large images: %d", list.ImageDrawCount())
				}
			})
		}
	}
}

// TestScreenshotWallpaperCancellation covers decoded results and a resolver that completes after the editor exits.
func TestScreenshotWallpaperCancellation(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprint(pending), func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			load := preloadScreenshotWallpaper(func(context.Context) (image.Image, error) {
				close(started)
				<-release
				return image.NewRGBA(image.Rect(0, 0, 32, 32)), nil
			})
			<-started
			if !pending {
				close(release)
				<-load.done
				if pixels, err := load.result(); pixels == nil || err != nil {
					t.Fatal("completed preload has no pixels")
				}
			}
			load.close()
			if pending {
				close(release)
				<-load.done
			}
			if pixels, err := load.result(); pixels != nil || !errors.Is(err, context.Canceled) || load.pixels != nil {
				t.Fatal("cancelled preload retained or republished wallpaper pixels")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := screenshotWallpaperReader{ctx: ctx, reader: bytes.NewReader([]byte{1, 2, 3})}
	if n, err := reader.Read(make([]byte, 4)); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled decoder continued reading")
	}
	if pixels, err := loadScreenshotWallpaper(ctx); pixels != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled loader resolved or decoded a wallpaper")
	}
}

// TestScreenshotBackgroundExitReleasesWorkers rejects late results without waiting on a blocked system resolver.
func TestScreenshotBackgroundExitReleasesWorkers(t *testing.T) {
	for cycle := 0; cycle < 20; cycle++ {
		selection := Rect{X: 20, Y: 20, Width: 40, Height: 40}
		source := image.NewRGBA(image.Rect(0, 0, 100, 100))
		state := newScreenshotEditorOverlayState(ScreenshotOptions{}, nil, screenshotEditorPlatform{frameSize: Size{Width: 100, Height: 100}, initialSelection: &selection})
		state.originalSource = source
		if err := state.installWindowCapture(selection, image.NewRGBA(image.Rect(0, 0, 40, 40))); err != nil {
			t.Fatal(err)
		}
		resolver := make(chan struct{})
		load := preloadScreenshotWallpaper(func(context.Context) (image.Image, error) {
			<-resolver
			return image.NewRGBA(image.Rect(0, 0, 50, 50)), nil
		})
		state.backgroundWallpaper = load
		state.prepareWindowBackground()
		done := state.backgroundDone
		state.releaseWindowBackground()
		select {
		case <-done:
		default:
			t.Fatal("exited editor still owns a preparation worker")
		}
		if state.backgroundWallpaper != nil || state.backgroundCancel != nil || state.backgroundSource != nil || state.backgroundPreview != nil || state.windowSource != nil {
			t.Fatal("exited editor retained loaded capture or background resources")
		}
		close(resolver)
		<-load.done
		if pixels, err := load.result(); pixels != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("late resolver completion revived a closed editor's background")
		}
	}
	// The wrapper also preserves EOF during normal decoder reads.
	reader := screenshotWallpaperReader{ctx: context.Background(), reader: bytes.NewReader(nil)}
	if _, err := reader.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

// TestScreenshotWindowBackgroundScene keeps wallpaper separate from original desktop and native window pixels.
func TestScreenshotWindowBackgroundScene(t *testing.T) {
	desktop := image.NewRGBA(image.Rect(0, 0, 800, 600))
	draw.Draw(desktop, desktop.Bounds(), image.NewUniform(color.RGBA{R: 255, A: 255}), image.Point{}, draw.Src)
	selection := Rect{X: 100, Y: 100, Width: 200, Height: 100}
	window := image.NewRGBA(image.Rect(0, 0, 200, 100))
	draw.Draw(window, window.Bounds(), image.NewUniform(color.RGBA{G: 255, A: 255}), image.Point{}, draw.Src)
	window.SetRGBA(0, 0, color.RGBA{})
	state := newScreenshotEditorOverlayState(ScreenshotOptions{}, nil, screenshotEditorPlatform{frameSize: Size{Width: 800, Height: 600}, initialSelection: &selection})
	state.originalSource = desktop
	if err := state.installWindowCapture(selection, window); err != nil {
		t.Fatal(err)
	}
	state.backgroundSource = fitScreenshotWallpaper(desktop, image.Pt(264, 164))
	for _, enabled := range []bool{false, true} {
		state.showBackground = enabled
		output := composeScreenshotWindowBackground(window, state.backgroundSource)
		path := filepath.Join(t.TempDir(), "window.png")
		if err := writeScreenshotImage(path, output); err != nil {
			t.Fatal(err)
		}
		save, err := prepareScreenshotDocumentSave(path, desktop, output, state)
		if err != nil {
			t.Fatal(err)
		}
		if err := save(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = screenshotedit.Remove(path) })
		document, source, _, _, err := loadScreenshotDocument(path)
		if err != nil {
			t.Fatal(err)
		}
		if document.ShowBackground != enabled || document.backgroundPixels == nil || document.backgroundPixels.Bounds().Size() != image.Pt(264, 164) {
			t.Fatal("scene lost wallpaper or toggle state")
		}
		if _, _, _, alpha := document.windowPixels.At(0, 0).RGBA(); alpha != 0 {
			t.Fatal("scene flattened native window alpha")
		}
		if !document.backgroundPixels.Opaque() || screenshotImageHasTransparency(source) {
			t.Fatal("scene lost opaque background or original desktop")
		}
		if got := composeScreenshotWindowBackground(document.windowPixels, document.backgroundPixels); got.RGBAAt(32, 32) != output.RGBAAt(32, 32) {
			t.Fatal("scene changed rounded-corner composition")
		}
	}
}
