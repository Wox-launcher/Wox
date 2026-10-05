package screenshot

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"wox/util"
	"wox/util/wallpaper"
)

// screenshotWallpaperLoad owns decoded pixels until preparation consumes them or the editor cancels the load.
type screenshotWallpaperLoad struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	pixels image.Image
	err    error
}

// preloadScreenshotWallpaper starts disk decoding without delaying the screenshot overlay.
func preloadScreenshotWallpaper(load func(context.Context) (image.Image, error)) *screenshotWallpaperLoad {
	ctx, cancel := context.WithCancel(context.Background())
	wallpaper := &screenshotWallpaperLoad{ctx: ctx, cancel: cancel, done: make(chan struct{})}
	go func() {
		pixels, err := load(ctx)
		wallpaper.mu.Lock()
		if ctx.Err() == nil {
			wallpaper.pixels, wallpaper.err = pixels, err
		}
		wallpaper.mu.Unlock()
		close(wallpaper.done)
	}()
	return wallpaper
}

// result transfers a reference under the same lock used to discard late decoder results on cancellation.
func (wallpaper *screenshotWallpaperLoad) result() (image.Image, error) {
	wallpaper.mu.Lock()
	defer wallpaper.mu.Unlock()
	if err := wallpaper.ctx.Err(); err != nil {
		return nil, err
	}
	return wallpaper.pixels, wallpaper.err
}

func (wallpaper *screenshotWallpaperLoad) close() {
	wallpaper.cancel()
	wallpaper.mu.Lock()
	wallpaper.pixels = nil
	wallpaper.mu.Unlock()
}

// stopWindowBackgroundPreparation joins every generation without waiting for a native wallpaper resolver.
// The resolver owns no editor pixels; its cancelled loader discards results before decoding or publishing them.
func (state *screenshotEditorOverlayState) stopWindowBackgroundPreparation() {
	state.mu.Lock()
	state.backgroundClosed = true
	state.cancelWindowBackgroundPreparationLocked()
	wallpaper := state.backgroundWallpaper
	state.backgroundWallpaper = nil
	state.mu.Unlock()
	if wallpaper != nil {
		wallpaper.close()
	}
	state.backgroundWorkers.Wait()
}

func (state *screenshotEditorOverlayState) cancelWindowBackgroundPreparationLocked() {
	if state.backgroundCancel != nil {
		state.backgroundCancel()
		state.backgroundCancel = nil
	}
}

// releaseWindowBackground runs after export and scene handoff, when no editor-owned background pixels are needed.
func (state *screenshotEditorOverlayState) releaseWindowBackground() {
	state.stopWindowBackgroundPreparation()
	state.mu.Lock()
	state.backgroundSource, state.backgroundPreview, state.backgroundDone = nil, nil, nil
	state.windowSource, state.windowSelection = nil, nil
	state.showBackground, state.backgroundLoading, state.backgroundFailed = false, false, false
	if state.document != nil {
		state.document.backgroundPixels, state.document.windowPixels = nil, nil
	}
	state.mu.Unlock()
}

// toggleWindowBackground reuses the prepared canvas or requests it while preloading finishes.
func (state *screenshotEditorOverlayState) toggleWindowBackground() {
	state.mu.Lock()
	if state.windowSource == nil || state.windowSelection == nil || state.backgroundClosed || state.scrolling || state.scrollingStarting {
		state.mu.Unlock()
		return
	}
	state.showBackground = !state.showBackground
	state.backgroundFailed = false
	prepare := state.showBackground && state.backgroundSource == nil && !state.backgroundLoading
	state.mu.Unlock()
	if prepare {
		state.prepareWindowBackground()
	}
	state.invalidate()
}

// prepareWindowBackground fits and composes the wallpaper as soon as native window pixels are ready.
// Preparing never enables the effect: editing still shows the frozen desktop until the user requests it.
func (state *screenshotEditorOverlayState) prepareWindowBackground() {
	state.mu.Lock()
	if state.windowSource == nil || state.windowSelection == nil || state.backgroundSource != nil || state.backgroundLoading || state.backgroundClosed {
		state.mu.Unlock()
		return
	}
	selection, generation := state.selection, state.windowCaptureGeneration
	source, frame, scale := state.windowSource, state.frameSize, state.annotationScale()
	if state.document == nil && state.chromeScale != nil {
		// Initial macOS captures can be ready before Draw publishes the active display's scale.
		scale = max(float32(1), state.chromeScale(selection))
	}
	wallpaper := state.backgroundWallpaper
	if wallpaper == nil {
		wallpaper = preloadScreenshotWallpaper(loadScreenshotWallpaper)
		state.backgroundWallpaper = wallpaper
	}
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	state.backgroundCancel = cancel
	state.backgroundWorkers.Add(1)
	state.backgroundLoading, state.backgroundDone = true, done
	state.mu.Unlock()
	state.invalidate()
	go func() {
		defer state.backgroundWorkers.Done()
		defer close(done)
		defer cancel()
		select {
		case <-ctx.Done():
			return
		case <-wallpaper.done:
		}
		pixels, err := wallpaper.result()
		state.mu.Lock()
		current := !state.backgroundClosed && generation == state.windowCaptureGeneration && state.windowSelection != nil && state.selection == selection
		state.mu.Unlock()
		if !current || ctx.Err() != nil {
			return
		}
		var background *image.RGBA
		var preview *Image
		if err == nil {
			clip, clipErr := screenshotEditorPixelSelection(source.Bounds(), selection, frame)
			err = clipErr
			if err == nil {
				padding := screenshotWindowBackgroundPadding(source.Bounds(), selection, frame, scale)
				background = fitScreenshotWallpaper(pixels, clip.Size().Add(padding.Mul(2)))
				if ctx.Err() != nil {
					return
				}
				preview, err = newScreenshotEditorImage(composeScreenshotWindowBackgroundPreview(source, clip, background))
			}
		}
		state.mu.Lock()
		current = ctx.Err() == nil && !state.backgroundClosed && generation == state.windowCaptureGeneration && state.windowSelection != nil && state.selection == selection
		if current {
			state.backgroundLoading = false
			state.backgroundFailed = err != nil && state.showBackground
			if err != nil {
				state.showBackground = false
			}
			if err == nil {
				state.backgroundSource, state.backgroundPreview = background, preview
			}
			// Keep only the fitted canvas, and allow a failed decode to be retried on the next click.
			state.backgroundWallpaper = nil
		}
		state.mu.Unlock()
		if current {
			wallpaper.close()
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			util.GetLogger().Warn(context.Background(), "screenshot wallpaper: "+err.Error())
		}
		if current {
			state.invalidate()
		}
	}()
}

// screenshotWallpaperReader lets image decoders stop at their next input read when the editor exits.
type screenshotWallpaperReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader screenshotWallpaperReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

// loadScreenshotWallpaper uses the existing platform resolver and cancellable macOS conversion for HEIC wallpapers.
func loadScreenshotWallpaper(ctx context.Context) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := wallpaper.GetSystemWallpaperPath()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	pixels, _, decodeErr := image.Decode(screenshotWallpaperReader{ctx: ctx, reader: file})
	_ = file.Close()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if decodeErr == nil || runtime.GOOS != "darwin" {
		return pixels, decodeErr
	}
	directory, err := os.MkdirTemp("", "wox-screenshot-wallpaper-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	output := filepath.Join(directory, "wallpaper.png")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "/usr/bin/sips", "-s", "format", "png", path, "--out", output).Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("cannot decode system wallpaper: " + decodeErr.Error())
	}
	file, err = os.Open(output)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	pixels, _, err = image.Decode(screenshotWallpaperReader{ctx: ctx, reader: file})
	return pixels, err
}

// screenshotWindowBackgroundPadding uses one fraction of each window dimension, preserving its aspect ratio.
// Bound the shorter inset in logical units, then convert both axes to actual capture pixels independently.
func screenshotWindowBackgroundPadding(bounds image.Rectangle, selection Rect, frame Size, scale float32) image.Point {
	scale = max(float32(1), scale)
	shortEdge := min(selection.Width, selection.Height)
	ratio := min(96*scale, max(32*scale, shortEdge*0.12)) / shortEdge
	return image.Pt(int(math.Ceil(float64(selection.Width*ratio*float32(bounds.Dx())/frame.Width))), int(math.Ceil(float64(selection.Height*ratio*float32(bounds.Dy())/frame.Height))))
}

// fitScreenshotWallpaper fills the output canvas without distorting the desktop image's aspect ratio.
func fitScreenshotWallpaper(pixels image.Image, size image.Point) *image.RGBA {
	result := image.NewRGBA(image.Rectangle{Max: size})
	draw.Draw(result, result.Bounds(), image.NewUniform(color.RGBA{R: 24, G: 29, B: 38, A: 255}), image.Point{}, draw.Src)
	stage := imaging.Fill(pixels, size.X, size.Y, imaging.Center, imaging.CatmullRom)
	draw.Draw(result, result.Bounds(), stage, stage.Bounds().Min, draw.Over)
	return result
}

// composeScreenshotWindowBackground centers native window pixels and rounds only the outer composited canvas.
func composeScreenshotWindowBackground(window image.Image, background *image.RGBA) *image.RGBA {
	result := image.NewRGBA(background.Bounds())
	draw.Draw(result, result.Bounds(), background, background.Bounds().Min, draw.Src)
	padding := background.Bounds().Size().Sub(window.Bounds().Size()).Div(2)
	origin := background.Bounds().Min.Add(padding)
	draw.Draw(result, image.Rectangle{Min: origin, Max: origin.Add(window.Bounds().Size())}, window, window.Bounds().Min, draw.Over)
	// The raw wallpaper stays opaque for scene storage; preview and export share this final alpha mask.
	radius := min(float64(min(result.Bounds().Dx(), result.Bounds().Dy()))/2, max(float64(4), float64(min(result.Bounds().Dx(), result.Bounds().Dy()))*0.02))
	cornerSize := int(math.Ceil(radius))
	for y := 0; y < cornerSize; y++ {
		for x := 0; x < cornerSize; x++ {
			coverage := min(float64(1), max(float64(0), radius+0.5-math.Hypot(float64(x)+0.5-radius, float64(y)+0.5-radius)))
			if coverage >= 1 {
				continue
			}
			for _, point := range [...]image.Point{{X: x, Y: y}, {X: result.Rect.Dx() - 1 - x, Y: y}, {X: x, Y: result.Rect.Dy() - 1 - y}, {X: result.Rect.Dx() - 1 - x, Y: result.Rect.Dy() - 1 - y}} {
				offset := result.PixOffset(result.Rect.Min.X+point.X, result.Rect.Min.Y+point.Y)
				// RGBA stores premultiplied channels, so coverage must scale color and alpha together.
				for channel := 0; channel < 4; channel++ {
					result.Pix[offset+channel] = uint8(float64(result.Pix[offset+channel])*coverage + 0.5)
				}
			}
		}
	}
	return result
}

// composeScreenshotWindowBackgroundPreview freezes the dimmed desktop and window canvas into one renderer image.
// Drawing them separately alternates two oversized images through the renderer's single large-image cache slot.
func composeScreenshotWindowBackgroundPreview(source *image.RGBA, clip image.Rectangle, background *image.RGBA) *image.RGBA {
	preview := image.NewRGBA(source.Bounds())
	draw.Draw(preview, preview.Bounds(), source, source.Bounds().Min, draw.Src)
	draw.Draw(preview, preview.Bounds(), image.NewUniform(color.RGBA{A: screenshotEditorDimAlpha}), image.Point{}, draw.Over)
	canvas := composeScreenshotWindowBackground(source.SubImage(clip), background)
	padding := background.Bounds().Size().Sub(clip.Size()).Div(2)
	origin := clip.Min.Sub(padding)
	draw.Draw(preview, image.Rectangle{Min: origin, Max: origin.Add(canvas.Bounds().Size())}, canvas, canvas.Bounds().Min, draw.Over)
	return preview
}

// selectionBoundsLocked exposes the outer background frame while keeping native window and annotation coordinates stable.
func (state *screenshotEditorOverlayState) selectionBoundsLocked() Rect {
	if state.showBackground && state.backgroundSource != nil && state.windowSource != nil {
		return screenshotWindowBackgroundBounds(state.selection, state.backgroundSource, state.windowSource.Bounds(), state.frameSize)
	}
	return state.selection
}

// selectionPixelBoundsLocked includes the full export canvas, even when its padding extends beyond the captured desktop.
func (state *screenshotEditorOverlayState) selectionPixelBoundsLocked() (image.Rectangle, error) {
	bounds := image.Rect(0, 0, state.image.Width, state.image.Height)
	clip, err := screenshotEditorPixelSelection(bounds, state.selection, state.frameSize)
	if err != nil {
		return image.Rectangle{}, err
	}
	if state.showBackground && state.backgroundSource != nil {
		padding := state.backgroundSource.Bounds().Size().Sub(clip.Size()).Div(2)
		clip.Min = clip.Min.Sub(padding)
		clip.Max = clip.Max.Add(padding)
	}
	return clip, nil
}

// screenshotWindowBackgroundBounds maps the padded image into editor coordinates without altering its window selection.
func screenshotWindowBackgroundBounds(selection Rect, background *image.RGBA, source image.Rectangle, frame Size) Rect {
	clip, err := screenshotEditorPixelSelection(source, selection, frame)
	if err != nil || background == nil {
		return selection
	}
	padding := background.Bounds().Size().Sub(clip.Size()).Div(2)
	x, y := frame.Width/float32(source.Dx()), frame.Height/float32(source.Dy())
	return Rect{X: float32(clip.Min.X-source.Min.X-padding.X) * x, Y: float32(clip.Min.Y-source.Min.Y-padding.Y) * y,
		Width: float32(background.Bounds().Dx()) * x, Height: float32(background.Bounds().Dy()) * y}
}
