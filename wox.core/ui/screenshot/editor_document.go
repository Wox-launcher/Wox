package screenshot

import (
	"encoding/json"
	"errors"
	"image"
	"image/draw"
	"math"
	"sync"

	"wox/util/screen"
	"wox/util/screenshotedit"
)

// screenshotDocument keeps capture coordinates stable across display and DPI changes.
// Native windows, text measurements, and drag state are deliberately not serialized.
type screenshotDocument struct {
	Displays                []screenshotDisplay
	Frame                   Size
	PixelWidth, PixelHeight int
	Selection               Rect
	WindowSelection         *Rect `json:",omitempty"`
	AnnotationScale         float32
	Annotations             []screenshotSavedAnnotation
	CursorPixel             *Point
	CursorHotspot           Point
	ShowCursor              bool
	DesktopPixelOrigin      Point
	ShowBackground          bool `json:",omitempty"`
	windowPixels            *image.RGBA
	backgroundPixels        *image.RGBA
}

type screenshotSavedAnnotation struct {
	Tool                                 string
	Rect                                 Rect
	Start, End, ArrowBend                Point
	CornerRadius                         float32
	Text                                 string
	Points                               []Point
	Color                                Color
	FontSize, MosaicRadius, StrokeRadius float32
	Number                               int
	PaintOrder                           uint64
}

// screenshotDisplay excludes work areas and labels, which can change without rearranging screens.
type screenshotDisplay struct {
	ID                  string
	Bounds, PixelBounds screen.Rect
	Scale               float64
	Primary             bool
}

// screenshotDisplayLayout records identity and both coordinate spaces used by capture and native windows.
func screenshotDisplayLayout(displays []screen.Display) []screenshotDisplay {
	layout := make([]screenshotDisplay, len(displays))
	for i, display := range displays {
		layout[i] = screenshotDisplay{ID: display.ID, Bounds: display.Bounds, PixelBounds: display.PixelBounds, Scale: display.Scale, Primary: display.Primary}
	}
	return layout
}

// screenshotDisplayLayoutMatches ignores enumeration order, but never infers a missing capture layout.
func screenshotDisplayLayoutMatches(saved []screenshotDisplay, current []screen.Display) bool {
	if len(saved) == 0 || len(saved) != len(current) {
		return false
	}
	counts := make(map[screenshotDisplay]int, len(saved))
	for _, display := range saved {
		if display.ID == "" || display.Bounds.IsEmpty() || display.PixelBounds.IsEmpty() || display.Scale <= 0 || math.IsNaN(display.Scale) || math.IsInf(display.Scale, 0) {
			return false
		}
		counts[display]++
	}
	for _, display := range screenshotDisplayLayout(current) {
		if counts[display] == 0 {
			return false
		}
		counts[display]--
	}
	return true
}

// prepareScreenshotDocumentSave detaches the scene from native pixels and mutable editor state.
// Compression, fingerprints, and disk writes belong to the returned background job.
func prepareScreenshotDocumentSave(path string, source, composited image.Image, state *screenshotEditorOverlayState) (func() error, error) {
	document := screenshotDocument{
		Displays: append([]screenshotDisplay(nil), state.capturedDisplays...),
		Frame:    state.frameSize, PixelWidth: source.Bounds().Dx(), PixelHeight: source.Bounds().Dy(),
		Selection: state.selection, AnnotationScale: state.annotationScale(), CursorPixel: state.cursorPixel,
		ShowCursor: state.showCursor, DesktopPixelOrigin: state.desktopPixelOrigin,
	}
	var window, background image.Image
	if state.windowSelection != nil && state.windowSource != nil && *state.windowSelection == state.selection {
		selection := *state.windowSelection
		document.WindowSelection = &selection
		clip, err := screenshotEditorPixelSelection(state.windowSource.Bounds(), selection, state.frameSize)
		if err != nil {
			return nil, err
		}
		// Keep original desktop pixels for re-editing and native window pixels as a separate export resource.
		// Sixteen-bit straight alpha avoids rounding the compositor's premultiplied edge colors on reload.
		pixels := image.NewNRGBA64(image.Rectangle{Max: clip.Size()})
		copyScreenshotSceneStraightAlpha(pixels, state.windowSource, clip.Min)
		window = pixels
	}
	if state.backgroundSource != nil {
		background = state.backgroundSource
	}
	document.ShowBackground = state.showBackground && background != nil
	if state.cursorPixel != nil {
		point := *state.cursorPixel
		document.CursorPixel = &point
	}
	for _, mark := range state.annotations {
		document.Annotations = append(document.Annotations, screenshotSavedAnnotation{
			Tool: screenshotEditorToolIconNames[mark.tool], Rect: mark.rect, Start: mark.start, End: mark.end,
			ArrowBend: mark.arrowBend, CornerRadius: mark.cornerRadius, Text: mark.text, Points: append([]Point(nil), mark.points...),
			Color: mark.color, FontSize: mark.fontSize, MosaicRadius: mark.mosaicRadius, StrokeRadius: mark.strokeRadius,
			Number: mark.number, PaintOrder: mark.paintOrder,
		})
	}
	var cursor image.Image
	if state.capturedCursor != nil && state.capturedCursor.raster != nil {
		cursor = state.capturedCursor.raster
		document.CursorHotspot = state.capturedCursor.hotspot
	} else if state.cursorPixel != nil {
		// Persist the fallback pointer too; its size must not depend on the next display's scale.
		x := float32(source.Bounds().Dx()) / document.Frame.Width
		y := float32(source.Bounds().Dy()) / document.Frame.Height
		var err error
		cursor, err = screenshotEditorCursorRaster(max(1, int(math.Round(float64(screenshotEditorCursorWidth*x)))), max(1, int(math.Round(float64(screenshotEditorCursorHeight*y)))))
		if err != nil {
			return nil, err
		}
		document.CursorHotspot = Point{X: screenshotEditorCursorHotspotX * x, Y: screenshotEditorCursorHotspotY * y}
	}
	if cursor != nil {
		// Eight-bit straight-alpha PNG rounds premultiplied edge pixels on reload.
		// Sixteen-bit storage preserves the original cursor's compositing values.
		lossless := image.NewNRGBA64(cursor.Bounds())
		copyScreenshotSceneStraightAlpha(lossless, cursor, cursor.Bounds().Min)
		cursor = lossless
	}
	// Windows captures are backed by a DIB that is freed when CaptureScreenshot returns.
	// The existing packed-pixel copy avoids a slow per-pixel image.Image conversion here.
	detachedSource := image.NewRGBA(image.Rect(0, 0, source.Bounds().Dx(), source.Bounds().Dy()))
	copyScreenshotCapture(detachedSource, source, source.Bounds().Min)
	var saveMu sync.Mutex
	var saved bool
	return func() error {
		saveMu.Lock()
		defer saveMu.Unlock()
		if saved {
			return nil
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			return err
		}
		if err := screenshotedit.Save(path, encoded, detachedSource, cursor, composited, window, background); err != nil {
			return err
		}
		// A completed job may remain with its result; release all rasters instead of retaining the desktop and wallpaper.
		saved = true
		detachedSource, cursor, composited, window, background = nil, nil, nil, nil, nil
		return nil
	}, nil
}

// copyScreenshotSceneStraightAlpha preserves edge precision without allocating a color object per pixel.
// Large native windows must detach before the editor closes; generic 16-bit draw conversion delays history publication.
func copyScreenshotSceneStraightAlpha(destination *image.NRGBA64, source image.Image, sourcePoint image.Point) {
	raster, ok := source.(*image.RGBA)
	if !ok {
		draw.Draw(destination, destination.Bounds(), source, sourcePoint, draw.Src)
		return
	}
	for y := 0; y < destination.Rect.Dy(); y++ {
		offset := raster.PixOffset(sourcePoint.X, sourcePoint.Y+y)
		src := raster.Pix[offset : offset+destination.Rect.Dx()*4]
		dst := destination.Pix[y*destination.Stride : y*destination.Stride+destination.Rect.Dx()*8]
		for x := 0; x < len(src); x += 4 {
			alpha := uint32(src[x+3])
			for channel := 0; channel < 4; channel++ {
				value := uint32(src[x+channel]) * 257
				if channel < 3 && alpha != 255 {
					if alpha == 0 {
						value = 0
					} else {
						value = uint32(src[x+channel]) * 65535 / alpha
					}
				}
				dst[x*2+channel*2], dst[x*2+channel*2+1] = byte(value>>8), byte(value)
			}
		}
	}
}

// loadScreenshotDocument validates persisted values before they reach geometry and drawing loops.
func loadScreenshotDocument(path string) (*screenshotDocument, image.Image, *screenshotEditorCapturedCursor, []screenshotEditorAnnotation, error) {
	metadata, source, cursor, window, background, err := screenshotedit.Load(path)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var document screenshotDocument
	if err := json.Unmarshal(metadata.State, &document); err != nil {
		return nil, nil, nil, nil, err
	}
	marks, err := document.annotations(source.Bounds())
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if window != nil {
		if document.WindowSelection == nil || *document.WindowSelection != document.Selection {
			return nil, nil, nil, nil, errors.New("screenshot window does not match its selection")
		}
		clip, err := screenshotEditorPixelSelection(source.Bounds(), document.Selection, document.Frame)
		if err != nil || clip.Size() != window.Bounds().Size() {
			return nil, nil, nil, nil, errors.New("invalid screenshot window image dimensions")
		}
		document.windowPixels = image.NewRGBA(image.Rectangle{Max: window.Bounds().Size()})
		draw.Draw(document.windowPixels, document.windowPixels.Bounds(), window, window.Bounds().Min, draw.Src)
	}
	if background != nil {
		clip, err := screenshotEditorPixelSelection(source.Bounds(), document.Selection, document.Frame)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		padding := background.Bounds().Size().Sub(clip.Size())
		if padding.X <= 0 || padding.Y <= 0 || padding.X%2 != 0 || padding.Y%2 != 0 {
			return nil, nil, nil, nil, errors.New("invalid screenshot background dimensions")
		}
		document.backgroundPixels = image.NewRGBA(image.Rectangle{Max: background.Bounds().Size()})
		draw.Draw(document.backgroundPixels, document.backgroundPixels.Bounds(), background, background.Bounds().Min, draw.Src)
		if !document.backgroundPixels.Opaque() {
			return nil, nil, nil, nil, errors.New("screenshot background is not opaque")
		}
	} else if document.ShowBackground {
		return nil, nil, nil, nil, errors.New("screenshot background image is missing")
	}
	if document.CursorPixel != nil && cursor == nil {
		return nil, nil, nil, nil, errors.New("screenshot cursor image is missing")
	}
	var captured *screenshotEditorCapturedCursor
	if cursor != nil {
		raster := image.NewRGBA(cursor.Bounds())
		draw.Draw(raster, raster.Bounds(), cursor, cursor.Bounds().Min, draw.Src)
		preview, err := newScreenshotEditorImage(raster)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		captured = &screenshotEditorCapturedCursor{raster: raster, preview: preview, hotspot: document.CursorHotspot}
	}
	return &document, source, captured, marks, nil
}

// annotations decodes stable tool names rather than persisting the editor's enum ordinals.
func (document *screenshotDocument) annotations(bounds image.Rectangle) ([]screenshotEditorAnnotation, error) {
	valid := func(values ...float32) bool {
		for _, value := range values {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || math.Abs(float64(value)) > 1e6 {
				return false
			}
		}
		return true
	}
	frame, selection := document.Frame, document.Selection
	if document.PixelWidth != bounds.Dx() || document.PixelHeight != bounds.Dy() ||
		!valid(frame.Width, frame.Height, selection.X, selection.Y, selection.Width, selection.Height, document.AnnotationScale,
			document.CursorHotspot.X, document.CursorHotspot.Y, document.DesktopPixelOrigin.X, document.DesktopPixelOrigin.Y) ||
		frame.Width <= 0 || frame.Height <= 0 || document.AnnotationScale < 1 || document.AnnotationScale > 16 ||
		selection.X < 0 || selection.Y < 0 || selection.Width < 2 || selection.Height < 2 ||
		selection.X+selection.Width > frame.Width+0.01 || selection.Y+selection.Height > frame.Height+0.01 || len(document.Annotations) > 100000 {
		return nil, errors.New("invalid screenshot scene geometry")
	}
	if document.CursorPixel != nil && !valid(document.CursorPixel.X, document.CursorPixel.Y) {
		return nil, errors.New("invalid screenshot cursor")
	}
	if window := document.WindowSelection; window != nil && (!valid(window.X, window.Y, window.Width, window.Height) ||
		window.X < 0 || window.Y < 0 || window.Width < 2 || window.Height < 2 || window.X+window.Width > frame.Width+0.01 || window.Y+window.Height > frame.Height+0.01) {
		return nil, errors.New("invalid screenshot window geometry")
	}
	marks := make([]screenshotEditorAnnotation, 0, len(document.Annotations))
	for _, saved := range document.Annotations {
		tool := screenshotEditorToolSelect
		for index, name := range screenshotEditorToolIconNames {
			if name == saved.Tool {
				tool = screenshotEditorTool(index)
				break
			}
		}
		if tool == screenshotEditorToolSelect || !valid(saved.Rect.X, saved.Rect.Y, saved.Rect.Width, saved.Rect.Height,
			saved.Start.X, saved.Start.Y, saved.End.X, saved.End.Y, saved.ArrowBend.X, saved.ArrowBend.Y,
			saved.CornerRadius, saved.FontSize, saved.MosaicRadius, saved.StrokeRadius) ||
			saved.FontSize < 0 || saved.FontSize > 1000 || saved.MosaicRadius < 0 || saved.MosaicRadius > 1000 ||
			saved.StrokeRadius < 0 || saved.StrokeRadius > 1000 || saved.CornerRadius < 0 || saved.Number < 0 || saved.Number > 1000000 {
			return nil, errors.New("invalid screenshot annotation")
		}
		for _, point := range saved.Points {
			if !valid(point.X, point.Y) {
				return nil, errors.New("invalid screenshot stroke")
			}
		}
		mark := screenshotEditorAnnotation{tool: tool, rect: saved.Rect, start: saved.Start, end: saved.End,
			arrowBend: saved.ArrowBend, cornerRadius: saved.CornerRadius, text: saved.Text, points: saved.Points,
			color: saved.Color, fontSize: saved.FontSize, mosaicRadius: saved.MosaicRadius, strokeRadius: saved.StrokeRadius,
			number: saved.Number, paintOrder: saved.PaintOrder}
		if !screenshotEditorAnnotationIsVisible(mark) {
			return nil, errors.New("empty screenshot annotation")
		}
		if tool == screenshotEditorToolEraser {
			mark.eraserPreview = &screenshotEditorEraserPreview{}
		}
		marks = append(marks, mark)
	}
	return marks, nil
}

// editSavedScreenshot enters the portable editor without taking a new desktop capture.
func editSavedScreenshot(options ScreenshotOptions) (ScreenshotResult, error) {
	document, source, cursor, marks, err := loadScreenshotDocument(options.EditScreenshotPath)
	if err != nil {
		return ScreenshotResult{}, err
	}
	displays, err := screen.ListDisplays()
	if err != nil || !screenshotDisplayLayoutMatches(document.Displays, displays) {
		return ScreenshotResult{}, ErrScreenshotDisplayLayoutChanged
	}
	platform := screenshotEditorPlatform{
		frameSize: document.Frame, initialSelection: &document.Selection,
		initialWindowSource: document.windowPixels, initialBackgroundSource: document.backgroundPixels,
		cursorPixel: document.CursorPixel, capturedCursor: cursor, desktopPixelOrigin: document.DesktopPixelOrigin,
		document: document, restoredAnnotations: marks,
	}
	if err := restoreScreenshotDocumentDesktop(&platform, document, displays); err != nil {
		return ScreenshotResult{}, err
	}
	options.capturedDisplays = document.Displays
	options.AllowVideoRecording = false
	options.AutoConfirm = false
	options.HideAnnotationToolbar = false
	options.ExportFilePath = ""
	options.SaveEditableScene = true
	return runScreenshotEditor(options, source, platform)
}

// screenshotDocumentDesktopBounds permits the original footprint only on an identical display layout.
func screenshotDocumentDesktopBounds(document *screenshotDocument, displays []screen.Display, physical bool) (Rect, bool) {
	xScale := float32(document.PixelWidth) / document.Frame.Width
	yScale := float32(document.PixelHeight) / document.Frame.Height
	bounds := Rect{X: document.DesktopPixelOrigin.X / xScale, Y: document.DesktopPixelOrigin.Y / yScale,
		Width: document.Frame.Width, Height: document.Frame.Height}
	available := screen.GetVirtualBounds(displays)
	if physical {
		bounds = Rect{X: document.DesktopPixelOrigin.X, Y: document.DesktopPixelOrigin.Y,
			Width: float32(document.PixelWidth), Height: float32(document.PixelHeight)}
		available = screen.GetVirtualPixelBounds(displays)
	}
	return bounds, screenshotDisplayLayoutMatches(document.Displays, displays) && bounds.X >= float32(available.X) && bounds.Y >= float32(available.Y) &&
		bounds.X+bounds.Width <= float32(available.Right()) && bounds.Y+bounds.Height <= float32(available.Bottom())
}

// screenshotDocumentViewport fits without modifying any persisted selection or annotation coordinates.
func screenshotDocumentViewport(canvas, surface Size) (float32, Point) {
	scale := min(float32(1), surface.Width/canvas.Width, surface.Height/canvas.Height)
	return scale, Point{X: (surface.Width - canvas.Width*scale) / 2, Y: (surface.Height - canvas.Height*scale) / 2}
}

func (state *screenshotEditorOverlayState) annotationScale() float32 {
	if state.document != nil {
		return state.document.AnnotationScale
	}
	return max(float32(1), state.uiScale)
}

// surfaceRect maps document coordinates to the native surface for IME, accessibility, and damage.
func (state *screenshotEditorOverlayState) surfaceRect(rect Rect) Rect {
	if state.document == nil {
		return rect
	}
	return Rect{X: state.viewportOffset.X + rect.X*state.viewportScale, Y: state.viewportOffset.Y + rect.Y*state.viewportScale,
		Width: rect.Width * state.viewportScale, Height: rect.Height * state.viewportScale}
}
