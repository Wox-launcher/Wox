package screenshot

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"

	"wox/util"
)

// screenshotWindowCaptureImage overlays a cropped native window capture onto the desktop
// coordinate space without copying the full desktop bitmap.
type screenshotWindowCaptureImage struct {
	bounds   image.Rectangle
	window   *image.RGBA
	fallback image.Image
}

func (img *screenshotWindowCaptureImage) ColorModel() color.Model {
	return color.RGBAModel
}

func (img *screenshotWindowCaptureImage) Bounds() image.Rectangle {
	return img.bounds
}

func (img *screenshotWindowCaptureImage) At(x, y int) color.Color {
	return img.RGBAAt(x, y)
}

func (img *screenshotWindowCaptureImage) RGBAAt(x, y int) color.RGBA {
	if img.window != nil && image.Pt(x, y).In(img.window.Rect) {
		return img.window.RGBAAt(x, y)
	}
	if img.fallback == nil {
		return color.RGBA{}
	}
	if rgba, ok := img.fallback.(interface{ RGBAAt(int, int) color.RGBA }); ok {
		return rgba.RGBAAt(x, y)
	}
	return color.RGBAModel.Convert(img.fallback.At(x, y)).(color.RGBA)
}

// WriteRGBA fills dst from origin while overlaying the cropped native window without a full-desktop copy.
func (img *screenshotWindowCaptureImage) WriteRGBA(dst *image.RGBA, origin image.Point) {
	if dst == nil {
		return
	}
	requested := image.Rectangle{Min: origin, Max: origin.Add(dst.Bounds().Size())}
	if img.window != nil && requested.In(img.window.Rect) {
		draw.Draw(dst, dst.Bounds(), img.window, origin, draw.Src)
		return
	}
	if img.fallback != nil {
		copyScreenshotCapture(dst, img.fallback, origin)
	}
	if img.window == nil {
		return
	}
	overlap := requested.Intersect(img.window.Rect)
	if overlap.Empty() {
		return
	}
	dstRect := image.Rectangle{
		Min: dst.Bounds().Min.Add(overlap.Min.Sub(origin)),
		Max: dst.Bounds().Min.Add(overlap.Max.Sub(origin)),
	}
	draw.Draw(dst, dstRect, img.window, overlap.Min, draw.Src)
}

// captureSelectedWindow keeps compositor waits off the UI thread and rejects captures from an abandoned gesture.
func (state *screenshotEditorOverlayState) captureSelectedWindow(selection Rect, generation uint64, done chan struct{}, autoConfirm bool) {
	defer close(done)
	pixels, err := state.captureWindow(selection)
	state.mu.Lock()
	current := !state.backgroundClosed && generation == state.windowCaptureGeneration && state.hasSelection && state.selection == selection
	if current && err == nil {
		err = state.installWindowCapture(selection, pixels)
	}
	if current && err != nil {
		state.windowSelection = nil
	}
	state.mu.Unlock()
	if err != nil {
		util.GetLogger().Debug(context.Background(), "screenshot native window capture: "+err.Error())
	}
	if current {
		state.invalidate()
		if autoConfirm {
			state.complete(false)
		}
	}
}

// installWindowCapture prepares native pixels for export without changing the frozen desktop shown during editing.
func (state *screenshotEditorOverlayState) installWindowCapture(selection Rect, pixels *image.RGBA) error {
	if pixels == nil || pixels.Bounds().Empty() || state.originalSource == nil {
		return errors.New("window capture has no pixels")
	}
	original := state.originalSource
	clip, err := screenshotEditorPixelSelection(original.Bounds(), selection, state.frameSize)
	if err != nil {
		return err
	}
	var window *image.RGBA
	if clip.Size() == pixels.Bounds().Size() {
		start := pixels.PixOffset(pixels.Rect.Min.X, pixels.Rect.Min.Y)
		window = &image.RGBA{Pix: pixels.Pix[start:], Stride: pixels.Stride, Rect: clip}
	} else {
		// Native backing resolution can differ from the display owning a clipped cross-display window.
		window = image.NewRGBA(clip)
		scaleImageCatmullRom(window, pixels, pixels.Bounds(), false)
	}
	state.windowSource = &screenshotWindowCaptureImage{
		bounds:   original.Bounds(),
		window:   window,
		fallback: original,
	}
	state.windowSelection = &selection
	return nil
}

// setSelectionLocked permanently abandons native window alpha after an actual selection change.
// Keep the capture completion channel so editor shutdown can wait for an in-flight native capture.
func (state *screenshotEditorOverlayState) setSelectionLocked(selection Rect) {
	// An unchanged outer frame is a click on background chrome, not a user change to the underlying window crop.
	if selection == state.selectionBoundsLocked() {
		return
	}
	if state.selection != selection {
		state.cancelWindowBackgroundPreparationLocked()
		state.windowCaptureGeneration++
		state.windowSource, state.windowSelection = nil, nil
		state.backgroundSource, state.backgroundPreview = nil, nil
		state.showBackground, state.backgroundLoading, state.backgroundFailed = false, false, false
	}
	state.selection = selection
}

// clipScreenshotWindowAlpha prevents annotations and cursor pixels from repainting transparent window edges.
// Taking the minimum preserves native translucent pixels without multiplying their alpha a second time.
func clipScreenshotWindowAlpha(output *image.RGBA, source image.Image, selection Rect, frame Size) {
	clip, err := screenshotEditorPixelSelection(source.Bounds(), selection, frame)
	if err != nil {
		return
	}
	raster, _ := source.(*image.RGBA)
	if raster == nil {
		if capture, ok := source.(*screenshotWindowCaptureImage); ok && capture.window != nil && clip.In(capture.window.Rect) {
			raster = capture.window
		}
	}
	for y := 0; y < output.Bounds().Dy(); y++ {
		offset := output.PixOffset(output.Rect.Min.X, output.Rect.Min.Y+y)
		row := output.Pix[offset : offset+output.Bounds().Dx()*4]
		var maskRow []byte
		if raster != nil {
			// Native window captures are RGBA. Reading packed alpha avoids boxing a color for every pixel.
			start := raster.PixOffset(clip.Min.X, clip.Min.Y+y)
			maskRow = raster.Pix[start : start+len(row)]
		}
		for x := 0; x < len(row); x += 4 {
			var alpha uint8
			if maskRow != nil {
				alpha = maskRow[x+3]
			} else {
				_, _, _, mask := source.At(clip.Min.X+x/4, clip.Min.Y+y).RGBA()
				alpha = uint8(mask >> 8)
			}
			old := row[x+3]
			if alpha >= old {
				continue
			}
			for channel := 0; channel < 3; channel++ {
				row[x+channel] = uint8(uint16(row[x+channel]) * uint16(alpha) / uint16(old))
			}
			row[x+3] = alpha
		}
	}
}
