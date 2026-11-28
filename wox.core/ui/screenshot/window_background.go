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
// Preparation must finish before a freeform capture's borrowed desktop storage can be released.
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
	if !state.hasSelection || state.dragging || state.backgroundCaptureSourceLocked() == nil || (state.windowSelection != nil && state.windowSource == nil) || state.backgroundClosed || state.scrolling || state.scrollingStarting {
		state.mu.Unlock()
		return
	}
	state.showBackground = !state.showBackground
	state.backgroundFailed = false
	prepare := state.showBackground && (state.backgroundSource == nil || state.backgroundPreview == nil) && !state.backgroundLoading
	state.mu.Unlock()
	if prepare {
		state.prepareWindowBackground()
	}
	state.invalidate()
}

// prepareWindowBackground loads and composes the wallpaper lazily when requested by the user.
func (state *screenshotEditorOverlayState) prepareWindowBackground() {
	state.mu.Lock()
	source := state.backgroundCaptureSourceLocked()
	if !state.hasSelection || state.dragging || source == nil || (state.windowSelection != nil && state.windowSource == nil) || (state.backgroundSource != nil && state.backgroundPreview != nil) || state.backgroundLoading || state.backgroundClosed || state.scrolling || state.scrollingStarting {
		state.mu.Unlock()
		return
	}
	selection, generation := state.selection, state.windowCaptureGeneration
	frame, scale := state.frameSize, state.annotationScale()
	if state.document == nil && state.chromeScale != nil {
		// Initial macOS captures can be ready before Draw publishes the active display's scale.
		scale = max(float32(1), state.chromeScale(selection))
	}
	existingBackground := state.backgroundSource
	var wallpaper *screenshotWallpaperLoad
	if existingBackground == nil {
		wallpaper = state.backgroundWallpaper
		if wallpaper == nil {
			wallpaper = preloadScreenshotWallpaper(loadScreenshotWallpaper)
			state.backgroundWallpaper = wallpaper
		}
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
		var pixels image.Image
		var err error
		if existingBackground == nil {
			select {
			case <-ctx.Done():
				return
			case <-wallpaper.done:
			}
			pixels, err = wallpaper.result()
		}
		state.mu.Lock()
		current := !state.backgroundClosed && generation == state.windowCaptureGeneration && state.hasSelection && state.selection == selection
		state.mu.Unlock()
		if !current || ctx.Err() != nil {
			return
		}
		background := existingBackground
		var preview *Image
		if err == nil {
			clip, clipErr := screenshotEditorPixelSelection(source.Bounds(), selection, frame)
			err = clipErr
			if err == nil {
				if background == nil {
					padding := screenshotWindowBackgroundPadding(source.Bounds(), selection, frame, scale)
					background = fitScreenshotWallpaper(pixels, clip.Size().Add(padding.Mul(2)))
					if ctx.Err() != nil {
						return
					}
				}
				preview, err = newScreenshotEditorImage(composeScreenshotWindowBackgroundPreview(source, clip, background))
			}
		}
		state.mu.Lock()
		current = ctx.Err() == nil && !state.backgroundClosed && generation == state.windowCaptureGeneration && state.hasSelection && state.selection == selection
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
		if current && wallpaper != nil {
			wallpaper.close()
			pixels = nil
			scheduleScreenshotMemoryRelease()
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

// screenshotWindowBackgroundPadding uses one fraction of each selection dimension, preserving its aspect ratio.
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
	if pixels == nil || pixels.Bounds().Empty() || size.X <= 0 || size.Y <= 0 {
		return result
	}
	srcBounds := pixels.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
	var cropW, cropH int
	if int64(srcW)*int64(size.Y) > int64(size.X)*int64(srcH) {
		cropH = srcH
		cropW = max(1, min(srcW, int(math.Round(float64(srcH)*float64(size.X)/float64(size.Y)))))
	} else {
		cropW = srcW
		cropH = max(1, min(srcH, int(math.Round(float64(srcW)*float64(size.Y)/float64(size.X)))))
	}
	x0 := srcBounds.Min.X + (srcW-cropW)/2
	y0 := srcBounds.Min.Y + (srcH-cropH)/2
	cropRect := image.Rect(x0, y0, x0+cropW, y0+cropH)
	scaleImageCatmullRom(result, pixels, cropRect, true)
	return result
}

type scaleDistribEntry struct {
	start   int
	end     int
	weights []float32
}

func catmullRomKernel(t float64) float64 {
	if t < 0 {
		t = -t
	}
	if t < 1 {
		return (1.5*t-2.5)*t*t + 1
	}
	if t < 2 {
		return ((-0.5*t+2.5)*t-4)*t + 2
	}
	return 0
}

func buildCatmullRomDistrib(dstLen, srcLen int) []scaleDistribEntry {
	entries := make([]scaleDistribEntry, dstLen)
	scale := float64(srcLen) / float64(dstLen)
	halfWidth := 2.0
	argScale := 1.0
	if scale > 1 {
		halfWidth = 2.0 * scale
		argScale = 1.0 / scale
	}
	for d := 0; d < dstLen; d++ {
		center := (float64(d)+0.5)*scale - 0.5
		start := max(0, int(math.Floor(center-halfWidth)))
		end := min(srcLen, int(math.Ceil(center+halfWidth)))
		if start >= end {
			nearest := max(0, min(srcLen-1, int(math.Round(center))))
			entries[d] = scaleDistribEntry{start: nearest, end: nearest + 1, weights: []float32{1}}
			continue
		}
		weights := make([]float32, end-start)
		total := 0.0
		for s := start; s < end; s++ {
			weight := catmullRomKernel((float64(s) - center) * argScale)
			weights[s-start] = float32(weight)
			total += weight
		}
		if total == 0 {
			nearest := max(0, min(srcLen-1, int(math.Round(center))))
			entries[d] = scaleDistribEntry{start: nearest, end: nearest + 1, weights: []float32{1}}
			continue
		}
		inv := float32(1.0 / total)
		for i := range weights {
			weights[i] *= inv
		}
		entries[d] = scaleDistribEntry{start: start, end: end, weights: weights}
	}
	return entries
}

func readScaledSourceRow(dst []float32, src image.Image, minX, y, width int) {
	switch raster := src.(type) {
	case *image.RGBA:
		pix := raster.Pix[raster.PixOffset(minX, y):]
		for x := 0; x < width; x++ {
			off := x * 4
			dst[off], dst[off+1], dst[off+2], dst[off+3] = float32(pix[off]), float32(pix[off+1]), float32(pix[off+2]), float32(pix[off+3])
		}
	case *image.NRGBA:
		pix := raster.Pix[raster.PixOffset(minX, y):]
		for x := 0; x < width; x++ {
			off := x * 4
			alpha := pix[off+3]
			if alpha == 255 {
				dst[off], dst[off+1], dst[off+2], dst[off+3] = float32(pix[off]), float32(pix[off+1]), float32(pix[off+2]), 255
			} else if alpha == 0 {
				dst[off], dst[off+1], dst[off+2], dst[off+3] = 0, 0, 0, 0
			} else {
				factor := float32(alpha) / 255.0
				dst[off], dst[off+1], dst[off+2], dst[off+3] = float32(pix[off])*factor, float32(pix[off+1])*factor, float32(pix[off+2])*factor, float32(alpha)
			}
		}
	case *image.YCbCr:
		for x := 0; x < width; x++ {
			yi := raster.YOffset(minX+x, y)
			ci := raster.COffset(minX+x, y)
			r, g, b := color.YCbCrToRGB(raster.Y[yi], raster.Cb[ci], raster.Cr[ci])
			off := x * 4
			dst[off], dst[off+1], dst[off+2], dst[off+3] = float32(r), float32(g), float32(b), 255
		}
	default:
		const scale = float32(1.0 / 257.0)
		for x := 0; x < width; x++ {
			r, g, b, a := src.At(minX+x, y).RGBA()
			off := x * 4
			dst[off], dst[off+1], dst[off+2], dst[off+3] = float32(r)*scale, float32(g)*scale, float32(b)*scale, float32(a)*scale
		}
	}
}

func clampScaledByte(value float32, limit uint8) uint8 {
	if value <= 0 {
		return 0
	}
	maxVal := float32(limit)
	if value >= maxVal {
		return limit
	}
	return uint8(value + 0.5)
}

// scaleImageCatmullRom resamples srcRect into dst using a rolling-row ring buffer so large
// wallpapers never allocate a full dstW*srcH float64 intermediate slice.
func scaleImageCatmullRom(dst *image.RGBA, src image.Image, srcRect image.Rectangle, over bool) {
	if dst == nil || dst.Bounds().Empty() || src == nil {
		return
	}
	srcRect = srcRect.Intersect(src.Bounds())
	if srcRect.Empty() {
		return
	}
	dstBounds := dst.Bounds()
	dstW, dstH := dstBounds.Dx(), dstBounds.Dy()
	srcW, srcH := srcRect.Dx(), srcRect.Dy()
	xDistrib := buildCatmullRomDistrib(dstW, srcW)
	yDistrib := buildCatmullRomDistrib(dstH, srcH)

	maxWindow := 1
	for _, yr := range yDistrib {
		if span := yr.end - yr.start; span > maxWindow {
			maxWindow = span
		}
	}
	srcRow := make([]float32, srcW*4)
	ring := make([]float32, maxWindow*dstW*4)
	ringRow := make([]int, maxWindow)
	for i := range ringRow {
		ringRow[i] = -1
	}

	for dy := 0; dy < dstH; dy++ {
		yr := yDistrib[dy]
		for sy := yr.start; sy < yr.end; sy++ {
			slot := sy % maxWindow
			if ringRow[slot] == sy {
				continue
			}
			readScaledSourceRow(srcRow, src, srcRect.Min.X, srcRect.Min.Y+sy, srcW)
			out := ring[slot*dstW*4 : (slot+1)*dstW*4]
			for dx := 0; dx < dstW; dx++ {
				xr := xDistrib[dx]
				var r, g, b, a float32
				for i, w := range xr.weights {
					off := (xr.start + i) * 4
					r += srcRow[off] * w
					g += srcRow[off+1] * w
					b += srcRow[off+2] * w
					a += srcRow[off+3] * w
				}
				dOff := dx * 4
				out[dOff], out[dOff+1], out[dOff+2], out[dOff+3] = r, g, b, a
			}
			ringRow[slot] = sy
		}

		dstOff := dst.PixOffset(dstBounds.Min.X, dstBounds.Min.Y+dy)
		dstPix := dst.Pix[dstOff : dstOff+dstW*4]
		for dx := 0; dx < dstW; dx++ {
			var r, g, b, a float32
			for i, w := range yr.weights {
				slot := (yr.start + i) % maxWindow
				in := ring[(slot*dstW+dx)*4:]
				r += in[0] * w
				g += in[1] * w
				b += in[2] * w
				a += in[3] * w
			}
			sa := clampScaledByte(a, 255)
			sr := clampScaledByte(r, sa)
			sg := clampScaledByte(g, sa)
			sb := clampScaledByte(b, sa)
			dOff := dx * 4
			if !over || sa == 255 {
				dstPix[dOff], dstPix[dOff+1], dstPix[dOff+2], dstPix[dOff+3] = sr, sg, sb, sa
			} else if sa > 0 {
				invA := uint32(255 - sa)
				dstPix[dOff] = uint8(min(255, uint32(sr)+(uint32(dstPix[dOff])*invA+127)/255))
				dstPix[dOff+1] = uint8(min(255, uint32(sg)+(uint32(dstPix[dOff+1])*invA+127)/255))
				dstPix[dOff+2] = uint8(min(255, uint32(sb)+(uint32(dstPix[dOff+2])*invA+127)/255))
				dstPix[dOff+3] = uint8(min(255, uint32(sa)+(uint32(dstPix[dOff+3])*invA+127)/255))
			}
		}
	}
}

// composeScreenshotWindowBackground centers capture pixels and rounds only the outer composited canvas.
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

// composeScreenshotWindowBackgroundPreview builds only the padded, rounded capture card so the
// editor can draw it over the already-cached desktop image without a full-desktop copy.
func composeScreenshotWindowBackgroundPreview(source image.Image, clip image.Rectangle, background *image.RGBA) *image.RGBA {
	if capture, ok := source.(*screenshotWindowCaptureImage); ok && capture.window != nil && capture.window.Rect == clip {
		return composeScreenshotWindowBackground(capture.window, background)
	}
	if sub, ok := source.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return composeScreenshotWindowBackground(sub.SubImage(clip), background)
	}
	cropped := image.NewRGBA(image.Rectangle{Max: clip.Size()})
	copyScreenshotCapture(cropped, source, clip.Min)
	return composeScreenshotWindowBackground(cropped, background)
}

// backgroundCaptureSourceLocked preserves native alpha only while the original window selection is intact.
func (state *screenshotEditorOverlayState) backgroundCaptureSourceLocked() image.Image {
	if state.windowSource != nil && state.windowSelection != nil && *state.windowSelection == state.selection {
		return state.windowSource
	}
	return state.originalSource
}

// selectionBoundsLocked exposes the outer background frame while keeping capture and annotation coordinates stable.
func (state *screenshotEditorOverlayState) selectionBoundsLocked() Rect {
	if source := state.backgroundCaptureSourceLocked(); state.showBackground && state.backgroundSource != nil && source != nil {
		return screenshotWindowBackgroundBounds(state.selection, state.backgroundSource, source.Bounds(), state.frameSize)
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
