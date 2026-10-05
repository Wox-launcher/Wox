package screenshot

import (
	"context"
	"errors"
	"image"
	"image/draw"

	xdraw "golang.org/x/image/draw"
	"wox/util"
)

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
	prepareBackground := current && err == nil && !autoConfirm && state.backgroundWallpaper != nil
	state.mu.Unlock()
	if err != nil {
		util.GetLogger().Debug(context.Background(), "screenshot native window capture: "+err.Error())
	}
	if current {
		if prepareBackground {
			state.prepareWindowBackground()
		}
		state.invalidate()
		if autoConfirm {
			state.complete(false)
		}
	}
}

// installWindowCapture prepares native pixels for export without changing the frozen desktop shown during editing.
// Src replaces background pixels at transparent edges instead of blending them into the window capture.
func (state *screenshotEditorOverlayState) installWindowCapture(selection Rect, pixels *image.RGBA) error {
	if pixels == nil || pixels.Bounds().Empty() || state.originalSource == nil {
		return errors.New("window capture has no pixels")
	}
	original := state.originalSource
	clip, err := screenshotEditorPixelSelection(original.Bounds(), selection, state.frameSize)
	if err != nil {
		return err
	}
	source := image.NewRGBA(original.Bounds())
	copyScreenshotCapture(source, original, original.Bounds().Min)
	if clip.Size() == pixels.Bounds().Size() {
		draw.Draw(source, clip, pixels, pixels.Bounds().Min, draw.Src)
	} else {
		// Native backing resolution can differ from the display owning a clipped cross-display window.
		xdraw.CatmullRom.Scale(source, clip, pixels, pixels.Bounds(), draw.Src, nil)
	}
	state.windowSource = source
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
	for y := 0; y < output.Bounds().Dy(); y++ {
		for x := 0; x < output.Bounds().Dx(); x++ {
			_, _, _, mask := source.At(clip.Min.X+x, clip.Min.Y+y).RGBA()
			alpha := uint8(mask >> 8)
			offset := output.PixOffset(output.Rect.Min.X+x, output.Rect.Min.Y+y)
			old := output.Pix[offset+3]
			if alpha >= old {
				continue
			}
			for channel := 0; channel < 3; channel++ {
				output.Pix[offset+channel] = uint8(uint16(output.Pix[offset+channel]) * uint16(alpha) / uint16(old))
			}
			output.Pix[offset+3] = alpha
		}
	}
}

// screenshotImageHasTransparency selects a lossless alpha-capable export only when the result needs it.
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
