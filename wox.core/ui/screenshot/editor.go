package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"wox/common"
	"wox/util"
	woxsvg "wox/util/svg"
)

const screenshotJPEGQuality = 90

type screenshotEditorOverlayOutcome struct {
	cancelled   bool
	pinned      bool
	record      bool
	copiedColor string
	// saveAsPath is set when the user asked to download the image to a chosen file.
	// Confirm still copies to the clipboard; save keeps that path as an extra export.
	saveAsPath string
}

type screenshotEditorTool uint8

const (
	screenshotEditorToolSelect screenshotEditorTool = iota
	screenshotEditorToolRect
	screenshotEditorToolEllipse
	screenshotEditorToolText
	screenshotEditorToolArrow
	screenshotEditorToolNumber
	screenshotEditorToolMosaic
	screenshotEditorToolCount
)

var screenshotEditorToolIconNames = [...]string{
	"screenshot.select",
	"screenshot.rectangle",
	"screenshot.ellipse",
	"screenshot.text",
	"screenshot.arrow",
	"screenshot.number",
	"screenshot.mosaic",
}

var screenshotEditorDefaultTooltips = [...]string{
	"",
	"Rectangle",
	"Ellipse",
	"Text",
	"Arrow",
	"Number",
	"Mosaic",
}

var screenshotEditorToolShortcuts = [...]string{"", "R", "E", "T", "A", "N", "M"}

type screenshotEditorAction uint8

const (
	screenshotEditorActionNone screenshotEditorAction = iota
	screenshotEditorActionUndo
	screenshotEditorActionScrollingCapture
	screenshotEditorActionCursor
	screenshotEditorActionPin
	screenshotEditorActionRecord
	screenshotEditorActionCancel
	screenshotEditorActionSave
	screenshotEditorActionConfirm
)

type screenshotEditorIconCacheKey struct {
	name  string
	color Color
}

type screenshotEditorCapturedCursor struct {
	raster  *image.RGBA
	preview *Image
	hotspot Point
}

var screenshotEditorIconCache sync.Map

const (
	screenshotEditorCursorWidth    = float32(28)
	screenshotEditorCursorHeight   = float32(36)
	screenshotEditorCursorHotspotX = float32(5.25)
	screenshotEditorCursorHotspotY = float32(3.75)
)

var (
	screenshotEditorCursorPreviewOnce  sync.Once
	screenshotEditorCursorPreviewImage *Image
)

type screenshotEditorEditMode uint8

const (
	screenshotEditorEditNone screenshotEditorEditMode = iota
	screenshotEditorEditMoveSelection
	screenshotEditorEditResizeSelection
	screenshotEditorEditMoveAnnotation
	screenshotEditorEditResizeAnnotation
	screenshotEditorEditArrowStart
	screenshotEditorEditArrowEnd
)

type screenshotEditorHandle uint8

const (
	screenshotEditorHandleTopLeft screenshotEditorHandle = iota
	screenshotEditorHandleTop
	screenshotEditorHandleTopRight
	screenshotEditorHandleRight
	screenshotEditorHandleBottomRight
	screenshotEditorHandleBottom
	screenshotEditorHandleBottomLeft
	screenshotEditorHandleLeft
)

type screenshotEditorOverlayState struct {
	mu                      sync.Mutex
	once                    sync.Once
	window                  *Window
	image                   *Image
	frameSize               Size
	workspaceSize           Size
	start                   Point
	selection               Rect
	confirmRect             Rect
	cancelRect              Rect
	saveRect                Rect
	pinRect                 Rect
	undoRect                Rect
	scrollRect              Rect
	cursorRect              Rect
	recordRect              Rect
	toolbarRect             Rect
	toolRects               [screenshotEditorToolCount]Rect
	tooltips                [screenshotEditorToolCount]string
	actionTooltips          ScreenshotActionTooltips
	editBarRect             Rect
	editColorRects          [6]Rect
	editSizeRects           [3]Rect
	editDecreaseRect        Rect
	editIncreaseRect        Rect
	editDeleteRect          Rect
	activeTool              screenshotEditorTool
	annotations             []screenshotEditorAnnotation
	draft                   *screenshotEditorAnnotation
	editMode                screenshotEditorEditMode
	editHandle              screenshotEditorHandle
	editOriginalRect        Rect
	editOriginalMark        screenshotEditorAnnotation
	selectedAnnotation      int
	hasSelectedMark         bool
	hoveredAnnotation       int
	hasHoveredMark          bool
	hoveredTool             int
	hasHoveredTool          bool
	hoveredAction           screenshotEditorAction
	hasHoveredAction        bool
	pointerCursor           PointerCursor
	pointerPosition         Point
	pointerInside           bool
	colorInspectorDismissed bool
	writeClipboardText      func(string) error
	setPointerPosition      func(Point) error
	textPosition            Point
	textDraft               string
	textMarked              string
	textCaret               int
	textEditing             bool
	editingTextIndex        int
	hasEditingText          bool
	caretVisible            bool
	caretBlinkAt            time.Time
	caretBlinkStop          chan struct{}
	caretBlinkDone          chan struct{}
	dragging                bool
	annotationDragging      bool
	hasSelection            bool
	autoConfirm             bool
	hideTools               bool
	allowVideoRecording     bool
	saving                  bool
	chooseSavePath          func() (string, error)
	scrolling               bool
	scrollingStarting       bool
	scrollingFrames         []screenshotScrollingFrame
	scrollingPreview        *Image
	scrollingStop           chan struct{}
	scrollingDone           chan struct{}
	scrollingStopOnce       sync.Once
	scrollingOverlaps       bool
	scrollBorderClose       func()
	startScrolling          func()
	annotationColor         Color
	mosaicRadius            float32
	textFontSize            float32
	nextNumber              int
	cursorPixel             *Point
	capturedCursor          *screenshotEditorCapturedCursor
	showCursor              bool
	uiScale                 float32
	chromeScale             func(selection Rect) float32
	desktopPixelOrigin      Point
	recordingUI             *recordingToolbarState
	result                  chan screenshotEditorOverlayOutcome
}

type screenshotEditorPlatform struct {
	setWindowBounds    func(window *Window) error
	logicalSelection   func(selection Rect, frameSize Size) Rect
	captureDesktop     func() (screenshotDesktopCapture, error)
	captureDesktopRect func(pixelBounds image.Rectangle) (*image.RGBA, error)
	// openRecordingCapture keeps a reusable OS capture surface for one recording session.
	openRecordingCapture func(pixelBounds image.Rectangle) (func() (*image.RGBA, error), func(), error)
	setScrollBounds      func(window *Window, controls Rect, frameSize Size) error
	showScrollBorder     func(selection Rect, frameSize Size) (func(), error)
	frameSize            Size
	initialSelection     *Rect
	cursorPixel          *Point
	capturedCursor       *screenshotEditorCapturedCursor
	afterShow            func()
	chromeScale          func(selection Rect) float32
	desktopPixelOrigin   Point
	setPointerPosition   func(Point) error
	cursorPosition       func() *Point
	setRecordingBounds   func(*Window, Rect, Size, float32) error
	// retainRecordingBorder keeps the Go-drawn selection stroke visible while
	// recording. Linux has no native hollow strip windows like Windows.
	retainRecordingBorder bool
	// recordingFrameIsRGBA is true when desktop capture stores packed RGBA.
	// Windows BitBlt frames stay BGR0; the H.264 encoder always consumes BGR0.
	recordingFrameIsRGBA bool
	preparedWindow       *ManagedWindow
	windowHost           *screenshotEditorWindowHost
}

type screenshotDesktopCapture struct {
	source  image.Image
	release func()
}

func (capture screenshotDesktopCapture) close() {
	if capture.release != nil {
		capture.release()
	}
}

// newScreenshotEditorOverlayState applies an optional native selection before the portable editor is shown.
func newScreenshotEditorOverlayState(options ScreenshotOptions, uiImage *Image, platform screenshotEditorPlatform) *screenshotEditorOverlayState {
	state := &screenshotEditorOverlayState{
		image:               uiImage,
		frameSize:           platform.frameSize,
		autoConfirm:         options.AutoConfirm,
		hideTools:           options.HideAnnotationToolbar,
		allowVideoRecording: options.AllowVideoRecording,
		annotationColor:     screenshotEditorAnnotationColor,
		mosaicRadius:        screenshotEditorMosaicRadius,
		textFontSize:        screenshotEditorTextFontSize,
		nextNumber:          1,
		cursorPixel:         platform.cursorPixel,
		capturedCursor:      platform.capturedCursor,
		uiScale:             1,
		chromeScale:         platform.chromeScale,
		desktopPixelOrigin:  platform.desktopPixelOrigin,
		setPointerPosition:  platform.setPointerPosition,
		result:              make(chan screenshotEditorOverlayOutcome, 1),
		scrollingStop:       make(chan struct{}),
		caretVisible:        true,
		caretBlinkAt:        time.Now(),
	}
	state.tooltips = [screenshotEditorToolCount]string{
		"",
		options.AnnotationTooltips.Rectangle,
		options.AnnotationTooltips.Ellipse,
		options.AnnotationTooltips.Text,
		options.AnnotationTooltips.Arrow,
		options.AnnotationTooltips.Number,
		options.AnnotationTooltips.Mosaic,
	}
	state.actionTooltips = options.ActionTooltips
	if platform.initialSelection != nil {
		state.selection = normalizeScreenshotEditorRect(*platform.initialSelection, platform.frameSize)
		state.hasSelection = state.selection.Width >= 2 && state.selection.Height >= 2
		state.colorInspectorDismissed = true
	}
	return state
}

func runScreenshotEditor(options ScreenshotOptions, source image.Image, platform screenshotEditorPlatform) (ScreenshotResult, error) {
	managed := platform.preparedWindow
	defer func() {
		if managed != nil {
			_ = managed.Close()
		}
	}()
	if source == nil || platform.setWindowBounds == nil || platform.logicalSelection == nil || platform.captureDesktop == nil {
		return ScreenshotResult{}, errors.New("screenshot editor platform is incomplete")
	}
	uiImage, err := newScreenshotEditorImage(source)
	if err != nil {
		return ScreenshotResult{}, fmt.Errorf("prepare screenshot overlay image: %w", err)
	}

	state := newScreenshotEditorOverlayState(options, uiImage, platform)
	state.startScrolling = func() {
		state.beginScrollingCapture(source, platform)
	}
	manager := options.WindowManager
	if manager == nil {
		manager = NewWindowManager()
	}
	host := platform.windowHost
	if host == nil {
		host = &screenshotEditorWindowHost{}
	}
	if err := host.begin(state); err != nil {
		return ScreenshotResult{}, err
	}
	defer host.end(state)
	var openErr error
	err = Call(func() {
		if managed == nil {
			managed, openErr = prepareScreenshotEditorWindow(manager, host)
			if openErr != nil {
				return
			}
		}
		overlay := managed.Window()
		state.mu.Lock()
		state.window = overlay
		state.writeClipboardText = overlay.WriteClipboardText
		state.mu.Unlock()
		openErr = platform.setWindowBounds(overlay)
		if openErr == nil {
			_, openErr = managed.Show()
		}
		if openErr == nil && platform.afterShow != nil {
			platform.afterShow()
		}
		if openErr == nil && state.autoConfirm && state.hasSelection {
			state.complete(false)
		}
	})
	if err != nil {
		return ScreenshotResult{}, err
	}
	if openErr != nil {
		return ScreenshotResult{}, openErr
	}
	state.startCaretBlink()
	defer state.stopCaretBlink()

	overlay := managed.Window()
	var outcome screenshotEditorOverlayOutcome
	select {
	case outcome = <-state.result:
	case <-time.After(175 * time.Second):
		outcome.cancelled = true
	}
	state.stopScrollingCapture()
	if outcome.cancelled {
		return ScreenshotResult{Cancelled: true}, nil
	}
	if outcome.copiedColor != "" {
		return ScreenshotResult{CopiedColor: outcome.copiedColor}, nil
	}

	state.mu.Lock()
	selection := state.selection
	frameSize := state.frameSize
	if state.workspaceSize.Width > 0 && state.workspaceSize.Height > 0 {
		frameSize = state.workspaceSize
	}
	annotations := append([]screenshotEditorAnnotation(nil), state.annotations...)
	scrolling := state.scrolling
	scrollingFrames := append([]screenshotScrollingFrame(nil), state.scrollingFrames...)
	cursorPixel := state.cursorPixel
	showCursor := state.showCursor
	annotationScale := max(float32(1), state.uiScale)
	state.mu.Unlock()
	if outcome.record {
		return runScreenshotRecording(options, state, selection, frameSize, platform)
	}
	exportPath := options.ExportFilePath
	reservedExport := false
	if exportPath == "" {
		exportPath, err = reserveScreenshotExportFilePath()
		if err != nil {
			return ScreenshotResult{}, err
		}
		reservedExport = true
	}
	exportSucceeded := false
	if reservedExport {
		defer func() {
			if !exportSucceeded {
				_ = os.Remove(exportPath)
			}
		}()
	}
	var exportedImage image.Image
	if scrolling {
		stitched, stitchErr := stitchScreenshotScrollingFrames(scrollingFrames)
		if stitchErr != nil {
			return ScreenshotResult{}, stitchErr
		}
		if err := writeScreenshotImage(exportPath, stitched); err != nil {
			return ScreenshotResult{}, err
		}
		exportedImage = stitched
	} else {
		pixelSelection, err := screenshotEditorPixelSelection(source.Bounds(), selection, frameSize)
		if err != nil {
			return ScreenshotResult{}, err
		}
		composited, err := renderScreenshotEditorAnnotations(source, annotations, selection, frameSize, annotationScale)
		if err != nil {
			return ScreenshotResult{}, err
		}
		if showCursor && cursorPixel != nil {
			if err := renderScreenshotEditorCursor(composited, *cursorPixel, selection, frameSize, state.capturedCursor); err != nil {
				return ScreenshotResult{}, err
			}
		}
		cropped := image.NewRGBA(image.Rect(0, 0, pixelSelection.Dx(), pixelSelection.Dy()))
		draw.Draw(cropped, cropped.Bounds(), composited, pixelSelection.Min, draw.Src)
		if err := writeScreenshotImage(exportPath, cropped); err != nil {
			return ScreenshotResult{}, err
		}
		exportedImage = cropped
	}
	if savePath := screenshotSaveAsExportPath(outcome.saveAsPath); savePath != "" && !screenshotExportPathsEqual(savePath, exportPath) {
		if err := writeScreenshotImage(savePath, exportedImage); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to write screenshot download: path=%s err=%s", savePath, err.Error()))
		}
	}

	result := ScreenshotResult{
		PinToScreen:             outcome.pinned,
		ArtifactKind:            "image",
		ArtifactPath:            exportPath,
		ScreenshotPath:          exportPath,
		ClipboardWriteSucceeded: outcome.pinned || outcome.saveAsPath != "" || !options.CopyToClipboard,
		LogicalSelection:        platform.logicalSelection(selection, frameSize),
	}
	if options.CopyToClipboard && !outcome.pinned && outcome.saveAsPath == "" {
		if err := overlay.WriteClipboardImage(exportedImage); err != nil {
			result.ClipboardWarningMessage = err.Error()
		} else {
			result.ClipboardWriteSucceeded = true
		}
	}
	exportSucceeded = true
	return result, nil
}

// newScreenshotEditorImage shares a tightly packed RGBA capture because the editor keeps the
// source immutable and both views have the same lifetime. Other image layouts use the normal copy.
func newScreenshotEditorImage(source image.Image) (*Image, error) {
	if rgba, ok := source.(*image.RGBA); ok && rgba.Stride == rgba.Rect.Dx()*4 {
		return NewImageFromPackedRGBA(rgba)
	}
	return NewImage(source)
}

func screenshotEditorPixelSelection(sourceBounds image.Rectangle, selection Rect, frameSize Size) (image.Rectangle, error) {
	if selection.Width <= 0 || selection.Height <= 0 || frameSize.Width <= 0 || frameSize.Height <= 0 {
		return image.Rectangle{}, errors.New("screenshot selection is empty")
	}
	scaleX := float32(sourceBounds.Dx()) / frameSize.Width
	scaleY := float32(sourceBounds.Dy()) / frameSize.Height
	pixelSelection := image.Rect(
		sourceBounds.Min.X+max(0, int(math.Floor(float64(selection.X*scaleX)))),
		sourceBounds.Min.Y+max(0, int(math.Floor(float64(selection.Y*scaleY)))),
		sourceBounds.Min.X+min(sourceBounds.Dx(), int(math.Ceil(float64((selection.X+selection.Width)*scaleX)))),
		sourceBounds.Min.Y+min(sourceBounds.Dy(), int(math.Ceil(float64((selection.Y+selection.Height)*scaleY)))),
	)
	if pixelSelection.Empty() {
		return image.Rectangle{}, errors.New("screenshot pixel selection is empty")
	}
	return pixelSelection, nil
}

// writeScreenshotImage uses JPEG for default exports while honoring explicit PNG paths from callers.
func writeScreenshotImage(path string, source image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create screenshot export directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create screenshot export file: %w", err)
	}
	var encodeErr error
	if extension := strings.ToLower(filepath.Ext(path)); extension == ".jpg" || extension == ".jpeg" {
		encodeErr = jpeg.Encode(file, source, &jpeg.Options{Quality: screenshotJPEGQuality})
	} else {
		encodeErr = png.Encode(file, source)
	}
	if encodeErr != nil {
		_ = file.Close()
		return fmt.Errorf("encode screenshot image: %w", encodeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close screenshot export file: %w", err)
	}
	return nil
}

func (state *screenshotEditorOverlayState) draw(displayList *DisplayList, frame FrameInfo) {
	state.mu.Lock()
	if state.scrolling {
		preview := state.scrollingPreview
		uiScale := max(float32(1), state.uiScale)
		toolbar, cancel, save, confirm := screenshotScrollingControlLayout(frame.Size, uiScale)
		state.confirmRect = confirm
		state.cancelRect = cancel
		state.saveRect = save
		state.toolbarRect = toolbar
		state.mu.Unlock()
		drawScreenshotScrollingControls(displayList, frame.Size, preview, uiScale)
		return
	}
	state.frameSize = frame.Size
	selection := normalizeScreenshotEditorRect(state.selection, frame.Size)
	hasSelection := state.hasSelection || state.dragging
	pointerPosition := state.pointerPosition
	pointerInside := state.pointerInside
	colorInspectorDismissed := state.colorInspectorDismissed
	uiScale := float32(1)
	if state.chromeScale != nil {
		scaleRect := selection
		if !hasSelection && pointerInside {
			scaleRect = Rect{X: pointerPosition.X, Y: pointerPosition.Y, Width: 1, Height: 1}
		}
		if scaleRect.Width > 0 && scaleRect.Height > 0 {
			uiScale = max(float32(1), state.chromeScale(scaleRect))
		}
	}
	inspectorScale := uiScale
	if pointerInside && state.chromeScale != nil {
		inspectorScale = max(float32(1), state.chromeScale(Rect{X: pointerPosition.X, Y: pointerPosition.Y, Width: 1, Height: 1}))
	}
	state.uiScale = uiScale
	scaled := func(value float32) float32 { return value * uiScale }
	for index := range state.annotations {
		state.measureTextAnnotation(&state.annotations[index], uiScale)
		state.measureNumberAnnotation(&state.annotations[index], uiScale)
	}
	dragging := state.dragging
	activeTool := state.activeTool
	hideTools := state.hideTools
	annotationColor := state.annotationColor
	mosaicRadius := state.mosaicRadius
	textFontSize := state.textFontSize
	textEditing := state.textEditing
	caretVisible := state.caretVisible
	editingTextIndex := state.editingTextIndex
	hasEditingText := state.hasEditingText && editingTextIndex >= 0 && editingTextIndex < len(state.annotations)
	textPosition := state.textPosition
	textPreview, textCaretPrefix := screenshotEditorTextEditingValue(state.textDraft, state.textMarked, state.textCaret)
	scrollingStarting := state.scrollingStarting
	cursorPixel := state.cursorPixel
	capturedCursor := state.capturedCursor
	showCursor := state.showCursor
	desktopPixelOrigin := state.desktopPixelOrigin
	annotations := append([]screenshotEditorAnnotation(nil), state.annotations...)
	if hasEditingText {
		if annotations[editingTextIndex].text != textPreview {
			annotations[editingTextIndex].textSize = Size{}
			annotations[editingTextIndex].measuredSize = 0
		}
		annotations[editingTextIndex].text = textPreview
		annotations[editingTextIndex].color = annotationColor
		annotations[editingTextIndex].fontSize = textFontSize
		state.measureTextAnnotation(&annotations[editingTextIndex], uiScale)
	}
	drawAnnotations := annotations
	selectedAnnotation := state.selectedAnnotation
	hasSelectedMark := state.hasSelectedMark && selectedAnnotation >= 0 && selectedAnnotation < len(state.annotations)
	hoveredAnnotation := state.hoveredAnnotation
	hasHoveredMark := state.hasHoveredMark && hoveredAnnotation >= 0 && hoveredAnnotation < len(state.annotations)
	hoveredTool := state.hoveredTool
	hasHoveredTool := state.hasHoveredTool && hoveredTool > int(screenshotEditorToolSelect) && hoveredTool < len(state.toolRects)
	tooltips := state.tooltips
	hoveredAction := state.hoveredAction
	hasHoveredAction := state.hasHoveredAction
	actionTooltips := state.actionTooltips
	if state.draft != nil {
		annotations = append(annotations, *state.draft)
		drawAnnotations = annotations
	}
	if hasEditingText {
		drawAnnotations = append([]screenshotEditorAnnotation(nil), annotations[:editingTextIndex]...)
		drawAnnotations = append(drawAnnotations, annotations[editingTextIndex+1:]...)
	}
	if textEditing && textPreview != "" {
		preview := screenshotEditorAnnotation{
			tool: screenshotEditorToolText, start: textPosition, text: textPreview,
			color: annotationColor, fontSize: textFontSize,
		}
		state.measureTextAnnotation(&preview, uiScale)
		drawAnnotations = append(drawAnnotations, preview)
	}
	state.confirmRect = Rect{}
	state.cancelRect = Rect{}
	state.saveRect = Rect{}
	state.pinRect = Rect{}
	state.undoRect = Rect{}
	state.scrollRect = Rect{}
	state.cursorRect = Rect{}
	state.recordRect = Rect{}
	state.toolbarRect = Rect{}
	state.toolRects = [screenshotEditorToolCount]Rect{}
	state.editBarRect = Rect{}
	state.editColorRects = [6]Rect{}
	state.editSizeRects = [3]Rect{}
	state.editDecreaseRect = Rect{}
	state.editIncreaseRect = Rect{}
	state.editDeleteRect = Rect{}
	state.mu.Unlock()

	displayList.Clear(Color{A: 255})
	displayList.DrawImage(state.image, Rect{Width: frame.Size.Width, Height: frame.Size.Height})
	dim := Color{A: 119}
	if !hasSelection || selection.Width <= 0 || selection.Height <= 0 {
		displayList.FillRect(Rect{Width: frame.Size.Width, Height: frame.Size.Height}, dim)
		if pointerInside && !colorInspectorDismissed {
			drawScreenshotEditorColorInspector(displayList, state.image, frame.Size, pointerPosition, desktopPixelOrigin, inspectorScale)
		}
		return
	}
	displayList.FillRect(Rect{Width: frame.Size.Width, Height: selection.Y}, dim)
	displayList.FillRect(Rect{Y: selection.Y + selection.Height, Width: frame.Size.Width, Height: max(float32(0), frame.Size.Height-selection.Y-selection.Height)}, dim)
	displayList.FillRect(Rect{Y: selection.Y, Width: selection.X, Height: selection.Height}, dim)
	displayList.FillRect(Rect{X: selection.X + selection.Width, Y: selection.Y, Width: max(float32(0), frame.Size.Width-selection.X-selection.Width), Height: selection.Height}, dim)

	displayList.PushClipRect(selection)
	drawScreenshotEditorAnnotations(displayList, drawAnnotations, state.image, frame.Size, uiScale)
	if textEditing && caretVisible {
		renderedTextSize := textFontSize * uiScale
		caretX := textPosition.X + screenshotEditorEstimatedTextWidth(textCaretPrefix, renderedTextSize)
		displayList.FillRect(Rect{X: caretX, Y: textPosition.Y, Width: max(float32(1), 1.5*uiScale), Height: renderedTextSize + 4*uiScale}, annotationColor)
	}
	if showCursor && cursorPixel != nil {
		drawScreenshotEditorCursor(displayList, screenshotEditorCursorLogicalPoint(*cursorPixel, state.image, frame.Size), capturedCursor, state.image, frame.Size)
	}
	if hasHoveredMark {
		drawScreenshotEditorAnnotationHandles(displayList, annotations[hoveredAnnotation], uiScale)
	} else if hasSelectedMark {
		drawScreenshotEditorAnnotationHandles(displayList, annotations[selectedAnnotation], uiScale)
	}
	displayList.PopClipRect()

	green := Color{R: 41, G: 255, B: 114, A: 255}
	displayList.StrokeRoundedRect(selection, 0, scaled(2), green)
	drawScreenshotEditorHandles(displayList, selection, green, uiScale)
	toolbarRect := Rect{}
	if !dragging && !state.autoConfirm {
		toolbarWidth := scaled(182)
		if !hideTools {
			toolbarWidth = scaled(686)
			if state.allowVideoRecording {
				toolbarWidth += scaled(54)
			}
		}
		toolbarHeight := scaled(60)
		toolbarStackHeight := toolbarHeight
		if !hideTools {
			toolbarStackHeight += scaled(64)
		}
		toolbarRect = screenshotEditorToolbarPlacement(selection, Rect{Width: frame.Size.Width, Height: frame.Size.Height}, toolbarWidth, toolbarHeight, toolbarStackHeight, uiScale)
	}
	drawScreenshotEditorSizeLabel(displayList, fmt.Sprintf("%.0f x %.0f", selection.Width, selection.Height), selection, toolbarRect, frame.Size, uiScale)
	if dragging || state.autoConfirm {
		return
	}
	toolbarLeft := toolbarRect.X
	toolbarTop := toolbarRect.Y
	slotLeft := toolbarLeft + scaled(16)
	var toolRects [screenshotEditorToolCount]Rect
	pinRect := Rect{}
	undoRect := Rect{}
	scrollRect := Rect{}
	cursorRect := Rect{}
	recordRect := Rect{}
	if !hideTools {
		for index := 1; index < len(toolRects); index++ {
			toolRects[index] = Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
			slotLeft += scaled(48)
		}
		slotLeft += scaled(6)
		undoRect = Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
		slotLeft += scaled(54)
		scrollRect = Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
		slotLeft += scaled(54)
		cursorRect = Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
		slotLeft += scaled(54)
		pinRect = Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
		slotLeft += scaled(54)
		if state.allowVideoRecording {
			recordRect = Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
			slotLeft += scaled(48)
		}
	}
	cancelRect := Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
	slotLeft += scaled(48)
	saveRect := Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
	slotLeft += scaled(48)
	confirmRect := Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
	state.mu.Lock()
	state.toolbarRect = toolbarRect
	state.toolRects = toolRects
	state.pinRect = pinRect
	state.undoRect = undoRect
	state.scrollRect = scrollRect
	state.cursorRect = cursorRect
	state.recordRect = recordRect
	state.cancelRect = cancelRect
	state.saveRect = saveRect
	state.confirmRect = confirmRect
	state.mu.Unlock()

	displayList.FillRoundedRect(toolbarRect, scaled(18), Color{R: 30, G: 26, B: 24, A: 255})
	if !hideTools {
		highlightedTool := activeTool
		if hasSelectedMark {
			highlightedTool = annotations[selectedAnnotation].tool
		}
		for index := 1; index < len(toolRects); index++ {
			rect := toolRects[index]
			selected := screenshotEditorTool(index) == highlightedTool
			foreground := Color{R: 255, G: 255, B: 255, A: 255}
			if selected {
				displayList.FillRoundedRect(rect, scaled(10), Color{R: 41, G: 255, B: 114, A: 51})
				foreground = green
			}
			drawScreenshotEditorToolbarIcon(displayList, screenshotEditorToolIconNames[index], rect, foreground, uiScale)
		}
		undoColor := Color{R: 255, G: 255, B: 255, A: 97}
		if len(annotations) > 0 {
			undoColor.A = 255
		}
		drawScreenshotEditorToolbarIcon(displayList, "control.undo", undoRect, undoColor, uiScale)
		scrollColor := Color{R: 255, G: 255, B: 255, A: 255}
		if scrollingStarting {
			scrollColor = green
		}
		drawScreenshotEditorToolbarIcon(displayList, "screenshot.scrolling-capture", scrollRect, scrollColor, uiScale)
		cursorColor := Color{R: 255, G: 255, B: 255, A: 255}
		if cursorPixel == nil {
			cursorColor.A = 97
		} else if showCursor {
			displayList.FillRoundedRect(cursorRect, scaled(10), Color{R: 41, G: 255, B: 114, A: 51})
			cursorColor = green
		}
		drawScreenshotEditorToolbarIconSized(displayList, "screenshot.cursor", cursorRect, cursorColor, uiScale, 20)
		drawScreenshotEditorToolbarIcon(displayList, "screenshot.pin", pinRect, Color{R: 255, G: 255, B: 255, A: 255}, uiScale)
		if recordRect.Width > 0 {
			drawScreenshotEditorToolbarIcon(displayList, "screenshot.video-camera", recordRect, Color{R: 255, G: 255, B: 255, A: 255}, uiScale)
		}
	}
	drawScreenshotEditorToolbarIcon(displayList, "control.close", cancelRect, Color{R: 255, G: 107, B: 107, A: 255}, uiScale)
	drawScreenshotEditorToolbarIcon(displayList, "control.download", saveRect, Color{R: 255, G: 255, B: 255, A: 255}, uiScale)
	drawScreenshotEditorToolbarIcon(displayList, "control.check", confirmRect, Color{R: 48, G: 227, B: 122, A: 255}, uiScale)
	if !hideTools {
		var selectedMark *screenshotEditorAnnotation
		if hasSelectedMark {
			selectedCopy := annotations[selectedAnnotation]
			if hasEditingText && selectedAnnotation == editingTextIndex {
				selectedCopy.color = annotationColor
				selectedCopy.fontSize = textFontSize
			}
			selectedMark = &selectedCopy
		}
		state.drawEditBar(displayList, frame.Size, toolbarRect, selection, activeTool, selectedMark, annotationColor, mosaicRadius, textFontSize, uiScale)
	}
	if hasHoveredTool {
		tooltip := screenshotEditorToolTooltip(hoveredTool, tooltips)
		drawScreenshotEditorToolTooltip(displayList, frame.Size, toolRects[hoveredTool], selection, tooltip, uiScale)
	} else if hasHoveredAction {
		anchor, tooltip := screenshotEditorActionTooltip(hoveredAction, actionTooltips, undoRect, scrollRect, cursorRect, pinRect, recordRect, cancelRect, saveRect, confirmRect)
		drawScreenshotEditorToolTooltip(displayList, frame.Size, anchor, selection, tooltip, uiScale)
	}
	if pointerInside && !colorInspectorDismissed {
		drawScreenshotEditorColorInspector(displayList, state.image, frame.Size, pointerPosition, desktopPixelOrigin, inspectorScale)
	}
}

const (
	screenshotEditorInspectorColumns = 17
	screenshotEditorInspectorRows    = 9
)

// drawScreenshotEditorColorInspector shows the exact captured pixel under the pointer and its surrounding pixels.
func drawScreenshotEditorColorInspector(displayList *DisplayList, source *Image, frame Size, pointer, desktopPixelOrigin Point, uiScale float32) {
	pixelX, pixelY, pixel, ok := screenshotEditorPixelAtPoint(source, frame, pointer)
	if !ok {
		return
	}
	uiScale = max(float32(1), uiScale)
	scaled := func(value float32) float32 { return value * uiScale }
	cellSize := scaled(12)
	previewWidth := cellSize * screenshotEditorInspectorColumns
	previewHeight := cellSize * screenshotEditorInspectorRows
	panelSize := Size{Width: previewWidth, Height: previewHeight + scaled(104)}
	panel := screenshotEditorInspectorRect(frame, pointer, panelSize, uiScale)

	displayList.FillRoundedRect(panel, scaled(10), Color{R: 20, G: 18, B: 17, A: 248})
	halfColumns := screenshotEditorInspectorColumns / 2
	halfRows := screenshotEditorInspectorRows / 2
	gridColor := Color{R: 0, G: 0, B: 0, A: 55}
	for row := 0; row < screenshotEditorInspectorRows; row++ {
		for column := 0; column < screenshotEditorInspectorColumns; column++ {
			sampleX := min(max(0, pixelX+column-halfColumns), source.Width-1)
			sampleY := min(max(0, pixelY+row-halfRows), source.Height-1)
			sample := source.RGBAAt(sampleX, sampleY)
			cell := Rect{
				X:      panel.X + float32(column)*cellSize,
				Y:      panel.Y + float32(row)*cellSize,
				Width:  cellSize,
				Height: cellSize,
			}
			displayList.FillRect(cell, Color{R: sample.R, G: sample.G, B: sample.B, A: 255})
			displayList.StrokeRoundedRect(cell, 0, max(float32(0.5), scaled(0.5)), gridColor)
		}
	}
	center := Rect{
		X:      panel.X + float32(halfColumns)*cellSize,
		Y:      panel.Y + float32(halfRows)*cellSize,
		Width:  cellSize,
		Height: cellSize,
	}
	displayList.StrokeRoundedRect(center, 0, scaled(2), Color{R: 41, G: 255, B: 114, A: 255})

	displayList.FillRect(Rect{X: panel.X, Y: panel.Y + previewHeight, Width: panel.Width, Height: scaled(1)}, Color{R: 255, G: 255, B: 255, A: 35})
	textColor := Color{R: 255, G: 255, B: 255, A: 255}
	secondaryTextColor := Color{R: 255, G: 255, B: 255, A: 165}
	infoTop := panel.Y + previewHeight
	globalX := int(math.Round(float64(desktopPixelOrigin.X))) + pixelX
	globalY := int(math.Round(float64(desktopPixelOrigin.Y))) + pixelY
	coordinate := fmt.Sprintf("%d, %d", globalX, globalY)
	coordinateWidth := screenshotEditorEstimatedTextWidth(coordinate, scaled(12))
	displayList.DrawText(coordinate, Rect{
		X: panel.X + (panel.Width-coordinateWidth)/2, Y: infoTop + scaled(9), Width: coordinateWidth, Height: scaled(18),
	}, TextStyle{Size: scaled(12), Weight: FontWeightSemibold}, secondaryTextColor)

	swatch := Rect{X: panel.X + scaled(12), Y: infoTop + scaled(38), Width: scaled(52), Height: scaled(52)}
	displayList.FillRoundedRect(swatch, scaled(8), Color{R: pixel.R, G: pixel.G, B: pixel.B, A: 255})
	displayList.StrokeRoundedRect(swatch, scaled(8), scaled(1), Color{R: 255, G: 255, B: 255, A: 190})
	drawColorRow := func(label, value, shortcut string, top float32) {
		labelLeft := panel.X + scaled(72)
		badge := Rect{X: panel.X + panel.Width - scaled(30), Y: top, Width: scaled(18), Height: scaled(18)}
		valueLeft := labelLeft + scaled(32)
		displayList.DrawText(label, Rect{X: labelLeft, Y: top + scaled(1), Width: scaled(28), Height: scaled(18)}, TextStyle{Size: scaled(11), Weight: FontWeightSemibold}, secondaryTextColor)
		displayList.DrawText(value, Rect{X: valueLeft, Y: top + scaled(1), Width: badge.X - valueLeft - scaled(4), Height: scaled(18)}, TextStyle{Size: scaled(11), Weight: FontWeightSemibold}, textColor)
		displayList.FillRoundedRect(badge, scaled(5), Color{R: 255, G: 255, B: 255, A: 25})
		displayList.DrawText(shortcut, Rect{X: badge.X + scaled(5), Y: badge.Y + scaled(1), Width: scaled(8), Height: scaled(16)}, TextStyle{Size: scaled(11), Weight: FontWeightSemibold}, secondaryTextColor)
	}
	drawColorRow("RGB", fmt.Sprintf("%d, %d, %d", pixel.R, pixel.G, pixel.B), "G", infoTop+scaled(40))
	drawColorRow("HEX", screenshotEditorColorText(pixel, screenshotEditorColorFormatHEX), "H", infoTop+scaled(69))
}

type screenshotEditorColorFormat uint8

const (
	screenshotEditorColorFormatRGB screenshotEditorColorFormat = iota
	screenshotEditorColorFormatHEX
)

func screenshotEditorColorText(pixel color.RGBA, format screenshotEditorColorFormat) string {
	if format == screenshotEditorColorFormatHEX {
		return fmt.Sprintf("#%02X%02X%02X", pixel.R, pixel.G, pixel.B)
	}
	return fmt.Sprintf("rgb(%d, %d, %d)", pixel.R, pixel.G, pixel.B)
}

// screenshotEditorPixelAtPoint maps the editor frame to the immutable desktop capture.
func screenshotEditorPixelAtPoint(source *Image, frame Size, point Point) (int, int, color.RGBA, bool) {
	if source == nil || source.Width <= 0 || source.Height <= 0 || frame.Width <= 0 || frame.Height <= 0 ||
		point.X < 0 || point.Y < 0 || point.X >= frame.Width || point.Y >= frame.Height {
		return 0, 0, color.RGBA{}, false
	}
	x := min(source.Width-1, int(math.Floor(float64(point.X*float32(source.Width)/frame.Width))))
	y := min(source.Height-1, int(math.Floor(float64(point.Y*float32(source.Height)/frame.Height))))
	return x, y, source.RGBAAt(x, y), true
}

// screenshotEditorInspectorRect keeps the floating inspector visible while preferring the pointer's lower-right side.
func screenshotEditorInspectorRect(frame Size, pointer Point, panel Size, uiScale float32) Rect {
	margin := 8 * uiScale
	offset := 20 * uiScale
	left := pointer.X + offset
	top := pointer.Y + offset
	if left+panel.Width > frame.Width-margin {
		left = pointer.X - offset - panel.Width
	}
	if top+panel.Height > frame.Height-margin {
		top = pointer.Y - offset - panel.Height
	}
	return Rect{
		X:      min(max(margin, left), max(margin, frame.Width-panel.Width-margin)),
		Y:      min(max(margin, top), max(margin, frame.Height-panel.Height-margin)),
		Width:  panel.Width,
		Height: panel.Height,
	}
}

// screenshotEditorDesktopPixelOrigin maps a logical desktop origin to the captured image's pixel coordinates.
func screenshotEditorDesktopPixelOrigin(bounds Rect, source image.Image) Point {
	if source == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return Point{}
	}
	return Point{
		X: bounds.X * float32(source.Bounds().Dx()) / bounds.Width,
		Y: bounds.Y * float32(source.Bounds().Dy()) / bounds.Height,
	}
}

// measureTextAnnotation uses the same native font metrics as DrawText so selection frames track rendered glyph advances.
func (state *screenshotEditorOverlayState) measureTextAnnotation(annotation *screenshotEditorAnnotation, uiScale float32) {
	if annotation == nil || annotation.tool != screenshotEditorToolText || annotation.text == "" || state.window == nil {
		return
	}
	renderedFontSize := screenshotEditorAnnotationRenderedFontSize(*annotation, uiScale)
	if annotation.textSize.Width > 0 && annotation.textSize.Height > 0 && math.Abs(float64(annotation.measuredSize-renderedFontSize)) < 0.01 {
		return
	}
	metrics, err := state.window.MeasureText(annotation.text, TextStyle{Size: renderedFontSize, Weight: FontWeightSemibold})
	if err != nil {
		return
	}
	annotation.textSize = Size{Width: max(float32(24), metrics.Size.Width), Height: max(renderedFontSize+4, metrics.Size.Height)}
	annotation.measuredSize = renderedFontSize
}

// measureNumberAnnotation uses the platform font width and line height to center marker labels precisely.
func (state *screenshotEditorOverlayState) measureNumberAnnotation(annotation *screenshotEditorAnnotation, uiScale float32) {
	if annotation == nil || annotation.tool != screenshotEditorToolNumber || annotation.number < 1 || state.window == nil {
		return
	}
	label, fontSize := screenshotEditorNumberLabel(*annotation, uiScale)
	if annotation.textSize.Width > 0 && annotation.textSize.Height > 0 && math.Abs(float64(annotation.measuredSize-fontSize)) < 0.01 {
		return
	}
	metrics, err := state.window.MeasureText(label, TextStyle{Size: fontSize, Weight: FontWeightSemibold})
	if err != nil {
		return
	}
	annotation.textSize = metrics.Size
	annotation.measuredSize = fontSize
}

func screenshotEditorToolTooltip(tool int, configured [screenshotEditorToolCount]string) string {
	if tool <= int(screenshotEditorToolSelect) || tool >= len(configured) {
		return ""
	}
	label := configured[tool]
	if label == "" {
		label = screenshotEditorDefaultTooltips[tool]
	}
	return fmt.Sprintf("%s (%s)", label, screenshotEditorToolShortcuts[tool])
}

func screenshotEditorActionTooltip(action screenshotEditorAction, configured ScreenshotActionTooltips, undo, scroll, cursor, pin, record, cancel, save, confirm Rect) (Rect, string) {
	switch action {
	case screenshotEditorActionUndo:
		return undo, screenshotEditorTooltipWithShortcut(configured.Undo, "Undo", "U")
	case screenshotEditorActionScrollingCapture:
		return scroll, screenshotEditorTooltipWithShortcut(configured.ScrollingCapture, "Long screenshot", "L")
	case screenshotEditorActionCursor:
		return cursor, screenshotEditorTooltipWithShortcut(configured.Cursor, "Show cursor", "C")
	case screenshotEditorActionPin:
		return pin, screenshotEditorTooltipWithShortcut(configured.Pin, "Pin to screen", "P")
	case screenshotEditorActionRecord:
		return record, screenshotEditorTooltipWithShortcut(configured.Record, "Record video", "V")
	case screenshotEditorActionCancel:
		return cancel, screenshotEditorTooltipWithShortcut(configured.Cancel, "Cancel", "Esc")
	case screenshotEditorActionSave:
		return save, screenshotEditorTooltipWithShortcut(configured.Save, "Save", screenshotEditorSaveShortcut())
	case screenshotEditorActionConfirm:
		return confirm, screenshotEditorTooltipWithShortcut(configured.Confirm, "Confirm", "Enter")
	default:
		return Rect{}, ""
	}
}

func screenshotEditorSaveShortcut() string {
	if runtime.GOOS == "darwin" {
		return "⌘S"
	}
	return "Ctrl+S"
}

func screenshotEditorTooltipWithShortcut(configured, fallback, shortcut string) string {
	if configured == "" {
		configured = fallback
	}
	return fmt.Sprintf("%s (%s)", configured, shortcut)
}

// screenshotEditorToolTooltipRect keeps the hint on the selection side of its tool, matching screenshot chrome.
func screenshotEditorToolTooltipRect(frame Size, anchor, selection Rect, text string, uiScale float32) Rect {
	if text == "" || anchor.Width <= 0 {
		return Rect{}
	}
	scaled := func(value float32) float32 { return value * uiScale }
	width := max(scaled(72), screenshotEditorEstimatedTextWidth(text, scaled(12))+scaled(20))
	height := scaled(28)
	left := min(max(scaled(8), anchor.X+anchor.Width/2-width/2), max(scaled(8), frame.Width-width-scaled(8)))
	top := max(scaled(8), anchor.Y-scaled(36))
	if selection.Height > 0 && anchor.Y+anchor.Height <= selection.Y {
		top = min(max(scaled(8), anchor.Y+anchor.Height+scaled(8)), max(scaled(8), frame.Height-height-scaled(8)))
	}
	return Rect{X: left, Y: top, Width: width, Height: height}
}

// drawScreenshotEditorToolTooltip renders one compact label beside its annotation tool.
func drawScreenshotEditorToolTooltip(displayList *DisplayList, frame Size, anchor, selection Rect, text string, uiScale float32) {
	drawScreenshotEditorToolTooltipAt(displayList, screenshotEditorToolTooltipRect(frame, anchor, selection, text, uiScale), text, uiScale)
}

// drawScreenshotEditorToolTooltipAt paints an already-placed toolbar hint.
func drawScreenshotEditorToolTooltipAt(displayList *DisplayList, rect Rect, text string, uiScale float32) {
	if text == "" || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	scaled := func(value float32) float32 { return value * uiScale }
	displayList.FillRoundedRect(rect, scaled(8), Color{R: 20, G: 18, B: 17, A: 240})
	textWidth := min(rect.Width-scaled(20), screenshotEditorEstimatedTextWidth(text, scaled(12)))
	displayList.DrawText(text, Rect{X: rect.X + (rect.Width-textWidth)/2, Y: rect.Y + scaled(6), Width: textWidth, Height: scaled(18)}, TextStyle{Size: scaled(12), Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: 255})
}

func screenshotEditorEstimatedTextWidth(text string, fontSize float32) float32 {
	width := float32(0)
	for _, character := range text {
		switch {
		case character > 0xFF:
			width += fontSize
		case character == ' ':
			width += fontSize * 0.32
		case strings.ContainsRune("ilI!.,':;|", character):
			width += fontSize * 0.28
		case strings.ContainsRune("mwMW@", character):
			width += fontSize * 0.82
		case character >= 'A' && character <= 'Z':
			width += fontSize * 0.62
		default:
			width += fontSize * 0.52
		}
	}
	return width
}

// screenshotEditorTextEditingValue inserts active IME composition at the retained rune caret.
func screenshotEditorTextEditingValue(text, marked string, caret int) (string, string) {
	runes := []rune(text)
	caret = min(max(0, caret), len(runes))
	prefix := string(runes[:caret]) + marked
	return prefix + string(runes[caret:]), prefix
}

// screenshotEditorTextCaretIndex resolves a click to the closest boundary between rendered runes.
func screenshotEditorTextCaretIndex(text string, offset, fontSize float32) int {
	if offset <= 0 {
		return 0
	}
	position := float32(0)
	for index, character := range []rune(text) {
		advance := screenshotEditorEstimatedTextWidth(string(character), fontSize)
		if offset < position+advance/2 {
			return index
		}
		position += advance
	}
	return utf8.RuneCountInString(text)
}

// drawEditBar places tool-specific creation and edit actions in a secondary row below the main toolbar.
func (state *screenshotEditorOverlayState) drawEditBar(
	displayList *DisplayList,
	frame Size,
	toolbar Rect,
	selection Rect,
	activeTool screenshotEditorTool,
	selected *screenshotEditorAnnotation,
	creationColor Color,
	creationMosaicRadius float32,
	creationTextSize float32,
	uiScale float32,
) {
	if selected == nil && activeTool == screenshotEditorToolSelect {
		return
	}

	isMosaic := activeTool == screenshotEditorToolMosaic
	isText := activeTool == screenshotEditorToolText
	color := creationColor
	mosaicRadius := creationMosaicRadius
	textSize := creationTextSize
	if selected != nil {
		isMosaic = selected.tool == screenshotEditorToolMosaic
		isText = selected.tool == screenshotEditorToolText
		color = screenshotEditorAnnotationDrawColor(*selected)
		mosaicRadius = screenshotEditorAnnotationMosaicRadius(*selected)
		textSize = screenshotEditorAnnotationFontSize(*selected)
	}

	scaled := func(value float32) float32 { return value * uiScale }
	width := scaled(192)
	if isMosaic {
		width = scaled(120)
	}
	if isText {
		width += scaled(136)
	}
	if selected != nil {
		width += scaled(54)
	}
	left := min(max(scaled(24), toolbar.X), max(scaled(24), frame.Width-width-scaled(24)))
	barHeight := scaled(56)
	top := screenshotEditorEditBarTop(toolbar, selection, barHeight, scaled(8), scaled(24))
	bar := Rect{X: left, Y: top, Width: width, Height: barHeight}
	displayList.FillRoundedRect(bar, scaled(18), Color{R: 27, G: 23, B: 21, A: 255})

	var colorRects [6]Rect
	var sizeRects [3]Rect
	decreaseRect, increaseRect, deleteRect := Rect{}, Rect{}, Rect{}
	cursorX := left + scaled(12)
	green := Color{R: 41, G: 255, B: 114, A: 255}
	if isMosaic {
		for index, radius := range screenshotEditorMosaicRadii {
			rect := Rect{X: cursorX, Y: top + scaled(12), Width: scaled(32), Height: scaled(32)}
			sizeRects[index] = rect
			visualRadius := scaled(4 + radius/screenshotEditorMosaicRadii[len(screenshotEditorMosaicRadii)-1]*6)
			strokeColor := Color{R: 255, G: 255, B: 255, A: 179}
			if math.Abs(float64(radius-mosaicRadius)) < 0.1 {
				strokeColor = green
			}
			circle := Rect{X: rect.X + rect.Width/2 - visualRadius, Y: rect.Y + rect.Height/2 - visualRadius, Width: visualRadius * 2, Height: visualRadius * 2}
			displayList.FillRoundedRect(circle, visualRadius, Color{R: strokeColor.R, G: strokeColor.G, B: strokeColor.B, A: 42})
			displayList.StrokeRoundedRect(circle, visualRadius, scaled(2), strokeColor)
			cursorX += scaled(32)
		}
	} else {
		for index, swatch := range screenshotEditorPalette {
			rect := Rect{X: cursorX, Y: top + scaled(18), Width: scaled(20), Height: scaled(20)}
			colorRects[index] = rect
			displayList.FillRoundedRect(rect, scaled(10), swatch)
			outline := Color{R: 255, G: 255, B: 255, A: 61}
			if swatch == color {
				outline = green
			}
			displayList.StrokeRoundedRect(rect, scaled(10), scaled(2), outline)
			cursorX += scaled(28)
		}
	}

	if isText {
		cursorX += scaled(8)
		displayList.FillRect(Rect{X: cursorX, Y: top + scaled(14), Width: scaled(1), Height: scaled(28)}, Color{R: 255, G: 255, B: 255, A: 34})
		cursorX += scaled(9)
		// Match the delete control: icon-only hit targets without a filled chip background.
		decreaseRect = Rect{X: cursorX, Y: top + scaled(7), Width: scaled(42), Height: scaled(42)}
		drawScreenshotEditorToolbarIcon(displayList, "control.remove", decreaseRect, Color{R: 255, G: 255, B: 255, A: 255}, uiScale)
		cursorX += scaled(42)
		sizeLabel := fmt.Sprintf("%.0f", textSize)
		sizeLabelWidth := scaled(40)
		sizeLabelHeight := scaled(18)
		sizeTextWidth := min(sizeLabelWidth, screenshotEditorEstimatedTextWidth(sizeLabel, scaled(12)))
		displayList.DrawText(sizeLabel, Rect{
			X: cursorX + (sizeLabelWidth-sizeTextWidth)/2, Y: top + (bar.Height-sizeLabelHeight)/2,
			Width: sizeTextWidth, Height: sizeLabelHeight,
		}, TextStyle{Size: scaled(12), Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: 255})
		cursorX += sizeLabelWidth
		increaseRect = Rect{X: cursorX, Y: top + scaled(7), Width: scaled(42), Height: scaled(42)}
		drawScreenshotEditorToolbarIcon(displayList, "control.add", increaseRect, Color{R: 255, G: 255, B: 255, A: 255}, uiScale)
		cursorX += scaled(42)
	}
	if selected != nil {
		cursorX += scaled(8)
		displayList.FillRect(Rect{X: cursorX, Y: top + scaled(14), Width: scaled(1), Height: scaled(28)}, Color{R: 255, G: 255, B: 255, A: 34})
		cursorX += scaled(3)
		deleteRect = Rect{X: cursorX, Y: top + scaled(7), Width: scaled(42), Height: scaled(42)}
		drawScreenshotEditorToolbarIconSized(displayList, "control.delete", deleteRect, Color{R: 255, G: 107, B: 107, A: 255}, uiScale, 20)
	}

	state.mu.Lock()
	state.editBarRect = bar
	state.editColorRects = colorRects
	state.editSizeRects = sizeRects
	state.editDecreaseRect = decreaseRect
	state.editIncreaseRect = increaseRect
	state.editDeleteRect = deleteRect
	state.mu.Unlock()
}

// screenshotEditorSizeLabelRect keeps the dimension chip on the free side of the selection edge.
func screenshotEditorSizeLabelRect(label string, selection, toolbar Rect, frame Size, uiScale float32) Rect {
	scaled := func(value float32) float32 { return value * max(float32(1), uiScale) }
	width := max(scaled(80), float32(len(label))*scaled(8)+scaled(16))
	height := scaled(26)
	left := min(max(scaled(8), selection.X+scaled(12)), max(scaled(8), frame.Width-width-scaled(8))) - scaled(8)
	top := max(scaled(8), selection.Y-scaled(32))
	if toolbar.Height > 0 && toolbar.Y+toolbar.Height <= selection.Y {
		top = min(max(scaled(8), selection.Y+scaled(8)), max(scaled(8), frame.Height-height-scaled(8)))
	}
	return Rect{X: left, Y: top, Width: width, Height: height}
}

// drawScreenshotEditorSizeLabel places a DPI-scaled dimension chip beside the selection.
func drawScreenshotEditorSizeLabel(displayList *DisplayList, label string, selection, toolbar Rect, frame Size, uiScale float32) {
	scaled := func(value float32) float32 { return value * max(float32(1), uiScale) }
	chip := screenshotEditorSizeLabelRect(label, selection, toolbar, frame, uiScale)
	displayList.FillRoundedRect(chip, scaled(10), Color{R: 23, G: 23, B: 23, A: 230})
	displayList.DrawText(label, Rect{X: chip.X + scaled(8), Y: chip.Y + scaled(5), Width: chip.Width - scaled(16), Height: scaled(18)}, TextStyle{Size: scaled(14), Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: 255})
}

// drawScreenshotEditorToolbarIcon renders the shared SVG at a consistent visual size.
func drawScreenshotEditorToolbarIcon(displayList *DisplayList, name string, rect Rect, color Color, uiScale float32) {
	drawScreenshotEditorToolbarIconSized(displayList, name, rect, color, uiScale, 24)
}

func drawScreenshotEditorToolbarIconSized(displayList *DisplayList, name string, rect Rect, color Color, uiScale, logicalSize float32) {
	size := logicalSize * uiScale
	inset := (rect.Width - size) / 2
	key := screenshotEditorIconCacheKey{name: name, color: color}
	if cached, ok := screenshotEditorIconCache.Load(key); ok {
		displayList.DrawImage(cached.(*Image), Rect{X: rect.X + inset, Y: rect.Y + inset, Width: size, Height: size})
		return
	}
	icon := common.UIIcon(name)
	if icon.ImageType != common.WoxImageTypeSvg || icon.ImageData == "" {
		return
	}
	rgba, err := woxsvg.Render(icon.ImageData, 48, 48)
	if err != nil {
		return
	}
	for index := 0; index < len(rgba.Pix); index += 4 {
		alpha := uint8((uint16(rgba.Pix[index+3])*uint16(color.A) + 127) / 255)
		rgba.Pix[index] = uint8((uint16(color.R)*uint16(alpha) + 127) / 255)
		rgba.Pix[index+1] = uint8((uint16(color.G)*uint16(alpha) + 127) / 255)
		rgba.Pix[index+2] = uint8((uint16(color.B)*uint16(alpha) + 127) / 255)
		rgba.Pix[index+3] = alpha
	}
	image, err := NewImage(rgba)
	if err != nil {
		return
	}
	actual, _ := screenshotEditorIconCache.LoadOrStore(key, image)
	displayList.DrawImage(actual.(*Image), Rect{X: rect.X + inset, Y: rect.Y + inset, Width: size, Height: size})
}

// drawScreenshotEditorCursor previews the captured pointer with the same marker exported to the image.
func drawScreenshotEditorCursor(displayList *DisplayList, hotspot Point, captured *screenshotEditorCapturedCursor, source *Image, frame Size) {
	if captured != nil && captured.preview != nil && source != nil && source.Width > 0 && source.Height > 0 {
		scaleX := frame.Width / float32(source.Width)
		scaleY := frame.Height / float32(source.Height)
		displayList.DrawImage(captured.preview, Rect{
			X:      hotspot.X - captured.hotspot.X*scaleX,
			Y:      hotspot.Y - captured.hotspot.Y*scaleY,
			Width:  float32(captured.preview.Width) * scaleX,
			Height: float32(captured.preview.Height) * scaleY,
		})
		return
	}
	screenshotEditorCursorPreviewOnce.Do(func() {
		rgba, err := renderScreenshotEditorCursorImage(56, 72)
		if err != nil {
			return
		}
		screenshotEditorCursorPreviewImage, _ = NewImage(rgba)
	})
	if screenshotEditorCursorPreviewImage == nil {
		return
	}
	displayList.DrawImage(screenshotEditorCursorPreviewImage, Rect{
		X: hotspot.X - screenshotEditorCursorHotspotX, Y: hotspot.Y - screenshotEditorCursorHotspotY,
		Width: screenshotEditorCursorWidth, Height: screenshotEditorCursorHeight,
	})
}

var screenshotEditorCursorCache struct {
	sync.Mutex
	images map[[2]int]*image.RGBA
}

// screenshotEditorCursorRaster reuses rasterized pointer images so recording does not re-render SVG every frame.
func screenshotEditorCursorRaster(width, height int) (*image.RGBA, error) {
	key := [2]int{width, height}
	screenshotEditorCursorCache.Lock()
	defer screenshotEditorCursorCache.Unlock()
	if screenshotEditorCursorCache.images == nil {
		screenshotEditorCursorCache.images = map[[2]int]*image.RGBA{}
	}
	if cached, ok := screenshotEditorCursorCache.images[key]; ok {
		return cached, nil
	}
	raster, err := renderScreenshotEditorCursorImage(width, height)
	if err != nil {
		return nil, err
	}
	screenshotEditorCursorCache.images[key] = raster
	return raster, nil
}

// renderScreenshotEditorCursorImage rasterizes the shared cursor marker at the requested export scale.
func renderScreenshotEditorCursorImage(width, height int) (*image.RGBA, error) {
	icon := common.UIIcon("screenshot.cursor")
	if icon.ImageType != common.WoxImageTypeSvg || icon.ImageData == "" {
		return nil, errors.New("screenshot cursor icon is unavailable")
	}
	return woxsvg.Render(icon.ImageData, width, height)
}

// screenshotEditorCursorLogicalPoint maps source pixels into the current editor frame.
func screenshotEditorCursorLogicalPoint(pixel Point, source *Image, frame Size) Point {
	if source == nil || source.Width <= 0 || source.Height <= 0 || frame.Width <= 0 || frame.Height <= 0 {
		return Point{}
	}
	return Point{
		X: pixel.X * frame.Width / float32(source.Width),
		Y: pixel.Y * frame.Height / float32(source.Height),
	}
}

// screenshotEditorCursorPixelFromDesktop maps a desktop coordinate into the captured source image.
func screenshotEditorCursorPixelFromDesktop(cursor Point, bounds Rect, source image.Image) *Point {
	if source == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return nil
	}
	if cursor.X < bounds.X || cursor.X >= bounds.X+bounds.Width || cursor.Y < bounds.Y || cursor.Y >= bounds.Y+bounds.Height {
		return nil
	}
	return &Point{
		X: (cursor.X - bounds.X) * float32(source.Bounds().Dx()) / bounds.Width,
		Y: (cursor.Y - bounds.Y) * float32(source.Bounds().Dy()) / bounds.Height,
	}
}

// screenshotEditorToolbarPlacement right-aligns the bar to the selection and prefers a 16px gap below, then above.
// stackHeight only decides whether the toolbar plus optional property bar fit below. The returned toolbar
// always uses toolbarHeight so the visible gap stays 16px on both sides instead of reserving empty stack space.
func screenshotEditorToolbarPlacement(selection, frame Rect, toolbarWidth, toolbarHeight, stackHeight, uiScale float32) Rect {
	if uiScale <= 0 {
		uiScale = 1
	}
	if stackHeight <= 0 {
		stackHeight = toolbarHeight
	}
	inset := 24 * uiScale
	gap := 16 * uiScale
	frameRight := frame.X + frame.Width
	frameBottom := frame.Y + frame.Height
	left := min(max(frame.X+inset, selection.X+selection.Width-toolbarWidth), max(frame.X+inset, frameRight-toolbarWidth-inset))
	top := selection.Y + selection.Height + gap
	if top+stackHeight > frameBottom-inset {
		top = max(frame.Y+inset, selection.Y-toolbarHeight-gap)
	}
	return Rect{X: left, Y: top, Width: toolbarWidth, Height: toolbarHeight}
}

// screenshotEditorEditBarTop grows the property bar away from the selection so the toolbar gap stays 16px.
func screenshotEditorEditBarTop(toolbar, selection Rect, barHeight, gap, inset float32) float32 {
	below := toolbar.Y + toolbar.Height + gap
	if toolbar.Y+toolbar.Height <= selection.Y {
		above := toolbar.Y - gap - barHeight
		if above >= inset {
			return above
		}
	}
	return below
}

func drawScreenshotEditorHandles(displayList *DisplayList, selection Rect, color Color, uiScale float32) {
	for _, point := range screenshotEditorRectHandlePoints(selection) {
		displayList.FillRoundedRect(Rect{X: point.X - 7*uiScale, Y: point.Y - 7*uiScale, Width: 14 * uiScale, Height: 14 * uiScale}, 4*uiScale, Color{A: 115})
		displayList.FillRoundedRect(Rect{X: point.X - 6*uiScale, Y: point.Y - 6*uiScale, Width: 12 * uiScale, Height: 12 * uiScale}, 4*uiScale, color)
	}
}

func screenshotEditorRectHandlePoints(rect Rect) []Point {
	return []Point{
		{X: rect.X, Y: rect.Y},
		{X: rect.X + rect.Width/2, Y: rect.Y},
		{X: rect.X + rect.Width, Y: rect.Y},
		{X: rect.X + rect.Width, Y: rect.Y + rect.Height/2},
		{X: rect.X + rect.Width, Y: rect.Y + rect.Height},
		{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height},
		{X: rect.X, Y: rect.Y + rect.Height},
		{X: rect.X, Y: rect.Y + rect.Height/2},
	}
}

func (state *screenshotEditorOverlayState) pointer(event PointerEvent) {
	if event.Button != PointerButtonPrimary && event.Kind != PointerMove && event.Kind != PointerLeave {
		return
	}
	switch event.Kind {
	case PointerDown:
		state.mu.Lock()
		state.pointerPosition = event.Position
		state.pointerInside = true
		confirm := state.hasSelection && screenshotEditorRectContains(state.confirmRect, event.Position)
		save := state.hasSelection && screenshotEditorRectContains(state.saveRect, event.Position)
		cancel := state.hasSelection && screenshotEditorRectContains(state.cancelRect, event.Position)
		pin := state.hasSelection && screenshotEditorRectContains(state.pinRect, event.Position)
		record := state.hasSelection && screenshotEditorRectContains(state.recordRect, event.Position)
		scroll := state.hasSelection && !state.scrolling && !state.scrollingStarting && screenshotEditorRectContains(state.scrollRect, event.Position)
		cursor := state.hasSelection && state.cursorPixel != nil && screenshotEditorRectContains(state.cursorRect, event.Position)
		if cursor {
			state.showCursor = !state.showCursor
		}
		if scroll {
			state.scrollingStarting = true
			state.scrollingDone = make(chan struct{})
		}
		undo := state.hasSelection && len(state.annotations) > 0 && screenshotEditorRectContains(state.undoRect, event.Position)
		toolbar := state.hasSelection && screenshotEditorRectContains(state.toolbarRect, event.Position)
		editBar := state.hasSelection && screenshotEditorRectContains(state.editBarRect, event.Position)
		editChanged := false
		for index, rect := range state.editColorRects {
			if !screenshotEditorRectContains(rect, event.Position) {
				continue
			}
			color := screenshotEditorPalette[index]
			if state.textEditing && state.hasEditingText {
				state.annotationColor = color
			} else if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
				state.annotations[state.selectedAnnotation].color = color
			} else {
				state.annotationColor = color
			}
			editChanged = true
			break
		}
		for index, rect := range state.editSizeRects {
			if !screenshotEditorRectContains(rect, event.Position) {
				continue
			}
			radius := screenshotEditorMosaicRadii[index]
			if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
				state.annotations[state.selectedAnnotation].mosaicRadius = radius
			} else {
				state.mosaicRadius = radius
			}
			editChanged = true
			break
		}
		if screenshotEditorRectContains(state.editDecreaseRect, event.Position) {
			if state.textEditing && state.hasEditingText {
				state.textFontSize = max(float32(12), state.textFontSize-2)
			} else if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) && state.annotations[state.selectedAnnotation].tool == screenshotEditorToolText {
				annotation := &state.annotations[state.selectedAnnotation]
				annotation.fontSize = max(float32(12), screenshotEditorAnnotationFontSize(*annotation)-2)
			} else if state.activeTool == screenshotEditorToolText {
				state.textFontSize = max(float32(12), state.textFontSize-2)
			}
			editChanged = true
		}
		if screenshotEditorRectContains(state.editIncreaseRect, event.Position) {
			if state.textEditing && state.hasEditingText {
				state.textFontSize = min(float32(48), state.textFontSize+2)
			} else if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) && state.annotations[state.selectedAnnotation].tool == screenshotEditorToolText {
				annotation := &state.annotations[state.selectedAnnotation]
				annotation.fontSize = min(float32(48), screenshotEditorAnnotationFontSize(*annotation)+2)
			} else if state.activeTool == screenshotEditorToolText {
				state.textFontSize = min(float32(48), state.textFontSize+2)
			}
			editChanged = true
		}
		if screenshotEditorRectContains(state.editDeleteRect, event.Position) && state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			state.annotations = append(state.annotations[:state.selectedAnnotation], state.annotations[state.selectedAnnotation+1:]...)
			state.hasSelectedMark = false
			editChanged = true
		}
		toolChanged := false
		for index, rect := range state.toolRects {
			if screenshotEditorRectContains(rect, event.Position) {
				state.commitTextLocked()
				state.activeTool = screenshotEditorTool(index)
				state.hasSelectedMark = false
				toolChanged = true
				break
			}
		}
		if !editChanged {
			if undo {
				state.annotations = state.annotations[:len(state.annotations)-1]
				state.hasSelectedMark = false
			} else if !toolbar && !editBar {
				if state.textEditing {
					state.commitTextLocked()
				}
				if state.activeTool == screenshotEditorToolText && screenshotEditorRectContains(state.selection, event.Position) {
					if !state.beginTextAnnotationEditAtLocked(event.Position) {
						state.hasSelectedMark = false
						state.hasHoveredMark = false
						state.hasEditingText = false
						state.textDraft = ""
						state.textMarked = ""
						state.textCaret = 0
						state.textPosition = event.Position
						state.textEditing = true
						state.showCaretLocked()
						state.pointerCursor = PointerCursorText
					}
				} else if state.beginAnnotationEditLocked(event.Position) {
					state.activeTool = screenshotEditorToolSelect
				} else {
					switch state.activeTool {
					case screenshotEditorToolSelect:
						state.commitTextLocked()
						if !state.beginSelectionEditLocked(event.Position) {
							state.start = event.Position
							state.selection = Rect{X: event.Position.X, Y: event.Position.Y}
							state.dragging = true
							state.colorInspectorDismissed = true
							state.hasSelection = false
							state.annotations = nil
							state.hasSelectedMark = false
						}
					case screenshotEditorToolNumber:
						if screenshotEditorRectContains(state.selection, event.Position) {
							state.addNumberAnnotationLocked(event.Position)
						}
					default:
						if screenshotEditorRectContains(state.selection, event.Position) {
							state.commitTextLocked()
							state.start = event.Position
							state.annotationDragging = true
							state.draft = &screenshotEditorAnnotation{
								tool:         state.activeTool,
								rect:         Rect{X: event.Position.X, Y: event.Position.Y},
								start:        event.Position,
								end:          event.Position,
								points:       []Point{event.Position},
								color:        state.annotationColor,
								mosaicRadius: state.mosaicRadius,
							}
						}
					}
				}
			}
		}
		textEditing := state.textEditing
		pointerCursor := state.pointerCursor
		state.mu.Unlock()
		state.setPointerCursor(pointerCursor)
		if scroll {
			state.invalidate()
			go state.startScrolling()
		} else if confirm {
			state.commitText()
			state.complete(false)
		} else if save {
			state.commitText()
			state.requestSave()
		} else if cancel {
			state.complete(true)
		} else if pin {
			state.commitText()
			state.completePin()
		} else if record {
			state.commitText()
			state.completeRecord()
		} else if cursor || editChanged || toolChanged || undo || (!toolbar && !editBar) {
			state.setTextInputEnabled(textEditing)
			state.invalidate()
		}
	case PointerMove:
		state.mu.Lock()
		pointerChanged := !state.pointerInside || state.pointerPosition != event.Position
		state.pointerPosition = event.Position
		state.pointerInside = true
		if state.dragging {
			state.selection = Rect{X: state.start.X, Y: state.start.Y, Width: event.Position.X - state.start.X, Height: event.Position.Y - state.start.Y}
		} else if state.editMode != screenshotEditorEditNone {
			state.updateSelectEditLocked(event.Position, event.Modifiers)
		} else if state.annotationDragging && state.draft != nil {
			position := clampScreenshotEditorPoint(event.Position, state.selection)
			switch state.draft.tool {
			case screenshotEditorToolRect, screenshotEditorToolEllipse:
				if event.Modifiers&KeyModifierShift != 0 {
					state.draft.rect = squareScreenshotEditorRectFromAnchor(state.start, position, state.selection)
				} else {
					state.draft.rect = normalizeScreenshotEditorRect(Rect{X: state.start.X, Y: state.start.Y, Width: position.X - state.start.X, Height: position.Y - state.start.Y}, state.frameSize)
				}
			case screenshotEditorToolArrow:
				state.draft.end = position
			case screenshotEditorToolMosaic:
				points := state.draft.points
				last := points[len(points)-1]
				if math.Hypot(float64(position.X-last.X), float64(position.Y-last.Y)) >= 7 {
					state.draft.points = append(points, position)
				}
			}
		} else {
			hoverChanged := state.updateHoverLocked(event.Position) || pointerChanged
			cursor := state.pointerCursor
			state.mu.Unlock()
			state.setPointerCursor(cursor)
			if hoverChanged {
				state.invalidate()
			}
			return
		}
		state.mu.Unlock()
		state.invalidate()
	case PointerLeave:
		state.mu.Lock()
		hoverChanged := state.pointerInside || state.hasHoveredMark || state.hasHoveredTool || state.hasHoveredAction || state.pointerCursor != PointerCursorDefault
		state.pointerInside = false
		state.hasHoveredMark = false
		state.hasHoveredTool = false
		state.hasHoveredAction = false
		state.pointerCursor = PointerCursorDefault
		state.mu.Unlock()
		state.setPointerCursor(PointerCursorDefault)
		if hoverChanged {
			state.invalidate()
		}
	case PointerUp:
		state.mu.Lock()
		if state.dragging {
			state.selection = normalizeScreenshotEditorRect(Rect{X: state.start.X, Y: state.start.Y, Width: event.Position.X - state.start.X, Height: event.Position.Y - state.start.Y}, state.frameSize)
			state.dragging = false
			state.hasSelection = state.selection.Width >= 2 && state.selection.Height >= 2
			autoConfirm := state.autoConfirm && state.hasSelection
			state.mu.Unlock()
			state.invalidate()
			if autoConfirm {
				state.complete(false)
			}
			return
		}
		if !state.annotationDragging || state.draft == nil {
			state.editMode = screenshotEditorEditNone
			state.mu.Unlock()
			state.invalidate()
			return
		}
		draft := *state.draft
		state.annotationDragging = false
		state.draft = nil
		if screenshotEditorAnnotationIsVisible(draft) {
			state.annotations = append(state.annotations, draft)
			state.selectedAnnotation = len(state.annotations) - 1
			state.hasSelectedMark = true
			state.hasHoveredMark = false
			// Keep the creation tool so the next drag draws another mark instead of moving the capture.
		}
		state.mu.Unlock()
		state.invalidate()
	}
}

func (state *screenshotEditorOverlayState) key(event KeyEvent) bool {
	if !event.Down {
		return false
	}
	if event.Key == KeyEscape {
		state.mu.Lock()
		if state.textEditing {
			state.textEditing = false
			state.textDraft = ""
			state.textMarked = ""
			state.textCaret = 0
			state.hasEditingText = false
			state.mu.Unlock()
			state.setTextInputEnabled(false)
			state.invalidate()
			return true
		}
		state.mu.Unlock()
		state.complete(true)
		return true
	}
	if event.Key == KeyBackspace || event.Key == KeyDelete {
		state.mu.Lock()
		if state.textEditing {
			runes := []rune(state.textDraft)
			state.textCaret = min(max(0, state.textCaret), len(runes))
			changed := false
			if event.Key == KeyBackspace && state.textCaret > 0 {
				runes = append(runes[:state.textCaret-1], runes[state.textCaret:]...)
				state.textCaret--
				changed = true
			} else if event.Key == KeyDelete && state.textCaret < len(runes) {
				runes = append(runes[:state.textCaret], runes[state.textCaret+1:]...)
				changed = true
			}
			state.textDraft = string(runes)
			state.textMarked = ""
			state.showCaretLocked()
			state.mu.Unlock()
			if changed {
				state.setTextInputEnabled(true)
				state.invalidate()
			}
			return changed
		}
		if !state.textEditing && state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			state.annotations = append(state.annotations[:state.selectedAnnotation], state.annotations[state.selectedAnnotation+1:]...)
			state.hasSelectedMark = false
			state.mu.Unlock()
			state.invalidate()
			return true
		}
		state.mu.Unlock()
	}
	if event.Key == KeyArrowLeft || event.Key == KeyArrowUp || event.Key == KeyArrowRight || event.Key == KeyArrowDown || event.Key == KeyHome || event.Key == KeyEnd {
		state.mu.Lock()
		if state.textEditing {
			if event.Key == KeyArrowUp || event.Key == KeyArrowDown {
				state.mu.Unlock()
				return false
			}
			length := utf8.RuneCountInString(state.textDraft)
			state.textCaret = min(max(0, state.textCaret), length)
			switch event.Key {
			case KeyArrowLeft:
				state.textCaret = max(0, state.textCaret-1)
			case KeyArrowRight:
				state.textCaret = min(length, state.textCaret+1)
			case KeyHome:
				state.textCaret = 0
			case KeyEnd:
				state.textCaret = length
			}
			state.textMarked = ""
			state.showCaretLocked()
			state.mu.Unlock()
			state.setTextInputEnabled(true)
			state.invalidate()
			return true
		}
		if event.Key == KeyArrowLeft || event.Key == KeyArrowUp || event.Key == KeyArrowRight || event.Key == KeyArrowDown {
			point, ok := screenshotEditorNudgedPoint(state.image, state.frameSize, state.pointerPosition, event.Key)
			setPointerPosition := state.setPointerPosition
			if ok && state.pointerInside && !state.colorInspectorDismissed {
				state.pointerPosition = point
			} else {
				ok = false
			}
			state.mu.Unlock()
			if !ok {
				return false
			}
			if setPointerPosition != nil {
				if err := setPointerPosition(point); err != nil {
					util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to nudge screenshot color pointer: %s", err.Error()))
				}
			}
			state.invalidate()
			return true
		}
		state.mu.Unlock()
	}
	if !event.Composing && event.Modifiers&(KeyModifierControl|KeyModifierAlt|KeyModifierMeta) == 0 {
		format, colorShortcut := screenshotEditorColorFormatRGB, true
		switch event.Key {
		case Key("g"):
		case Key("h"):
			format = screenshotEditorColorFormatHEX
		default:
			colorShortcut = false
		}
		if colorShortcut && state.copyInspectedColor(format) {
			return true
		}
	}
	if !event.Composing && event.Key == Key("s") && event.Modifiers.HasPrimary() {
		state.mu.Lock()
		hasSelection := state.hasSelection
		textEditing := state.textEditing
		state.mu.Unlock()
		if !hasSelection {
			return true
		}
		if textEditing {
			state.commitText()
		}
		state.requestSave()
		return true
	}
	if event.Key == KeyEnter {
		state.mu.Lock()
		if state.textEditing {
			state.commitTextLocked()
			state.mu.Unlock()
			state.setTextInputEnabled(false)
			state.invalidate()
			return true
		}
		hasSelection := state.hasSelection
		state.mu.Unlock()
		if hasSelection {
			state.complete(false)
		}
		return true
	}
	if event.Composing || event.Modifiers&(KeyModifierControl|KeyModifierAlt|KeyModifierMeta) != 0 {
		return false
	}
	state.mu.Lock()
	if state.textEditing || state.hideTools || !state.hasSelection {
		state.mu.Unlock()
		return false
	}
	tool, toolShortcut := screenshotEditorToolSelect, true
	switch event.Key {
	case Key("r"):
		tool = screenshotEditorToolRect
	case Key("e"):
		tool = screenshotEditorToolEllipse
	case Key("t"):
		tool = screenshotEditorToolText
	case Key("a"):
		tool = screenshotEditorToolArrow
	case Key("n"):
		tool = screenshotEditorToolNumber
	case Key("m"):
		tool = screenshotEditorToolMosaic
	default:
		toolShortcut = false
	}
	if toolShortcut {
		state.activeTool = tool
		state.hasSelectedMark = false
		state.mu.Unlock()
		state.invalidate()
		return true
	}
	switch event.Key {
	case Key("u"):
		if len(state.annotations) > 0 {
			state.annotations = state.annotations[:len(state.annotations)-1]
			state.hasSelectedMark = false
		}
		state.mu.Unlock()
		state.invalidate()
		return true
	case Key("c"):
		if state.cursorPixel != nil {
			state.showCursor = !state.showCursor
		}
		state.mu.Unlock()
		state.invalidate()
		return true
	case Key("l"):
		if state.scrolling || state.scrollingStarting {
			state.mu.Unlock()
			return true
		}
		state.scrollingStarting = true
		state.scrollingDone = make(chan struct{})
		state.mu.Unlock()
		state.invalidate()
		go state.startScrolling()
		return true
	case Key("p"):
		state.mu.Unlock()
		state.commitText()
		state.completePin()
		return true
	default:
		state.mu.Unlock()
		return false
	}
}

// copyInspectedColor writes the currently sampled color without changing the screenshot selection.
func (state *screenshotEditorOverlayState) copyInspectedColor(format screenshotEditorColorFormat) bool {
	state.mu.Lock()
	if state.textEditing || state.colorInspectorDismissed || !state.pointerInside || state.writeClipboardText == nil {
		state.mu.Unlock()
		return false
	}
	_, _, pixel, ok := screenshotEditorPixelAtPoint(state.image, state.frameSize, state.pointerPosition)
	writeClipboardText := state.writeClipboardText
	state.mu.Unlock()
	if !ok {
		return false
	}
	value := screenshotEditorColorText(pixel, format)
	if err := writeClipboardText(value); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to copy screenshot color: %s", err.Error()))
		return true
	}
	state.completeColorCopy(value)
	return true
}

// screenshotEditorNudgedPoint moves the sampling point to the center of one adjacent captured pixel.
func screenshotEditorNudgedPoint(source *Image, frame Size, point Point, key Key) (Point, bool) {
	x, y, _, ok := screenshotEditorPixelAtPoint(source, frame, point)
	if !ok {
		return Point{}, false
	}
	switch key {
	case KeyArrowLeft:
		x = max(0, x-1)
		return Point{X: (float32(x) + 0.5) * frame.Width / float32(source.Width), Y: point.Y}, true
	case KeyArrowUp:
		y = max(0, y-1)
		return Point{X: point.X, Y: (float32(y) + 0.5) * frame.Height / float32(source.Height)}, true
	case KeyArrowRight:
		x = min(source.Width-1, x+1)
		return Point{X: (float32(x) + 0.5) * frame.Width / float32(source.Width), Y: point.Y}, true
	case KeyArrowDown:
		y = min(source.Height-1, y+1)
		return Point{X: point.X, Y: (float32(y) + 0.5) * frame.Height / float32(source.Height)}, true
	default:
		return Point{}, false
	}
}

// addNumberAnnotationLocked places the next marker while keeping the number tool active for consecutive clicks.
func (state *screenshotEditorOverlayState) addNumberAnnotationLocked(point Point) {
	number := state.nextNumber
	if number < 1 {
		number = 1
	}
	state.annotations = append(state.annotations, screenshotEditorAnnotation{
		tool: screenshotEditorToolNumber, start: point, number: number, color: state.annotationColor,
	})
	state.nextNumber = number + 1
	state.selectedAnnotation = len(state.annotations) - 1
	state.hasSelectedMark = true
	state.hasHoveredMark = false
}

func (state *screenshotEditorOverlayState) textInput(event TextInputEvent) {
	state.mu.Lock()
	if !state.textEditing {
		state.mu.Unlock()
		return
	}
	if event.Kind == TextInputCompose {
		state.textMarked = event.Text
	} else {
		runes := []rune(state.textDraft)
		state.textCaret = min(max(0, state.textCaret), len(runes))
		inserted := []rune(event.Text)
		updated := make([]rune, 0, len(runes)+len(inserted))
		updated = append(updated, runes[:state.textCaret]...)
		updated = append(updated, inserted...)
		updated = append(updated, runes[state.textCaret:]...)
		state.textDraft = string(updated)
		state.textCaret += len(inserted)
		state.textMarked = ""
	}
	state.showCaretLocked()
	state.mu.Unlock()
	state.setTextInputEnabled(true)
	state.invalidate()
}

func (state *screenshotEditorOverlayState) commitText() {
	state.mu.Lock()
	state.commitTextLocked()
	state.mu.Unlock()
	state.setTextInputEnabled(false)
	state.invalidate()
}

func (state *screenshotEditorOverlayState) commitTextLocked() {
	if state.textEditing && state.textDraft != "" {
		if state.hasEditingText && state.editingTextIndex >= 0 && state.editingTextIndex < len(state.annotations) {
			annotation := &state.annotations[state.editingTextIndex]
			if annotation.text != state.textDraft {
				annotation.textSize = Size{}
				annotation.measuredSize = 0
			}
			annotation.text = state.textDraft
			annotation.start = state.textPosition
			annotation.color = state.annotationColor
			annotation.fontSize = state.textFontSize
			state.selectedAnnotation = state.editingTextIndex
		} else {
			state.annotations = append(state.annotations, screenshotEditorAnnotation{
				tool: screenshotEditorToolText, start: state.textPosition, text: state.textDraft,
				color: state.annotationColor, fontSize: state.textFontSize,
			})
			state.selectedAnnotation = len(state.annotations) - 1
		}
		state.hasSelectedMark = true
		state.hasHoveredMark = false
	}
	state.textEditing = false
	state.hasEditingText = false
	state.showCaretLocked()
	state.textDraft = ""
	state.textMarked = ""
	state.textCaret = 0
}

func (state *screenshotEditorOverlayState) setTextInputEnabled(enabled bool) {
	state.mu.Lock()
	window := state.window
	textPosition := state.textPosition
	textFontSize := state.textFontSize
	uiScale := state.uiScale
	_, caretPrefix := screenshotEditorTextEditingValue(state.textDraft, state.textMarked, state.textCaret)
	state.mu.Unlock()
	if window == nil {
		return
	}
	renderedFontSize := textFontSize * max(float32(1), uiScale)
	cursor := Rect{X: textPosition.X + screenshotEditorEstimatedTextWidth(caretPrefix, renderedFontSize), Y: textPosition.Y, Width: 1, Height: max(float32(24), renderedFontSize+4)}
	_ = window.SetTextInputState(TextInputState{Enabled: enabled, CursorRect: cursor})
}

func (state *screenshotEditorOverlayState) setPointerCursor(cursor PointerCursor) {
	if state.window != nil {
		_ = state.window.SetPointerCursor(cursor)
	}
}

func (state *screenshotEditorOverlayState) invalidate() {
	if state.window != nil {
		_ = state.window.Invalidate()
	}
}

func (state *screenshotEditorOverlayState) showCaretLocked() {
	state.caretVisible = true
	state.caretBlinkAt = time.Now()
}

// startCaretBlink refreshes only while text input is active, keeping the static editor idle otherwise.
func (state *screenshotEditorOverlayState) startCaretBlink() {
	state.mu.Lock()
	if state.caretBlinkStop != nil {
		state.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	state.caretBlinkStop = stop
	state.caretBlinkDone = done
	state.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				state.mu.Lock()
				editing := state.textEditing
				if editing && time.Since(state.caretBlinkAt) >= 500*time.Millisecond {
					state.caretVisible = !state.caretVisible
					state.caretBlinkAt = time.Now()
				} else {
					editing = false
				}
				state.mu.Unlock()
				if editing {
					state.invalidate()
				}
			case <-stop:
				return
			}
		}
	}()
}

// stopCaretBlink terminates the session ticker before its window is released.
func (state *screenshotEditorOverlayState) stopCaretBlink() {
	state.mu.Lock()
	stop, done := state.caretBlinkStop, state.caretBlinkDone
	state.caretBlinkStop = nil
	state.caretBlinkDone = nil
	state.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
}

func (state *screenshotEditorOverlayState) complete(cancelled bool) {
	state.once.Do(func() {
		state.result <- screenshotEditorOverlayOutcome{cancelled: cancelled}
	})
}

// completeSave finishes the overlay after the user chose a download path.
func (state *screenshotEditorOverlayState) completeSave(path string) {
	state.once.Do(func() {
		state.result <- screenshotEditorOverlayOutcome{saveAsPath: path}
	})
}

// requestSave asks where to download the image and keeps the editor open if the dialog is cancelled.
func (state *screenshotEditorOverlayState) requestSave() {
	state.mu.Lock()
	if state.saving || !state.hasSelection {
		state.mu.Unlock()
		return
	}
	state.saving = true
	state.mu.Unlock()
	path, err := state.chooseScreenshotSavePath()
	state.mu.Lock()
	state.saving = false
	state.mu.Unlock()
	if err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to choose screenshot save path: %s", err.Error()))
		return
	}
	if path == "" {
		return
	}
	state.completeSave(screenshotSaveAsExportPath(path))
}

// chooseScreenshotSavePath hides the overlay so the native Save As dialog is not covered.
func (state *screenshotEditorOverlayState) chooseScreenshotSavePath() (string, error) {
	if state.chooseSavePath != nil {
		return state.chooseSavePath()
	}
	window := state.window
	if window == nil {
		return "", errors.New("screenshot window is not initialized")
	}
	_ = window.Hide()
	title := strings.TrimSpace(state.actionTooltips.SaveTitle)
	if title == "" {
		title = "Save screenshot"
	}
	defaultName := time.Now().Format("20060102_150405") + "_wox_snapshots.jpg"
	path, err := window.SaveFile(SaveFileOptions{Title: title, DefaultFileName: defaultName, Extension: "jpg"})
	if path == "" || err != nil {
		if _, showErr := window.Show(); showErr != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to restore screenshot overlay after save dialog: %s", showErr.Error()))
		}
		_ = window.Invalidate()
	}
	return path, err
}

// screenshotSaveAsExportPath keeps a user-chosen destination and adds JPEG only when no extension was given.
func screenshotSaveAsExportPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.Ext(path) == "" {
		return path + ".jpg"
	}
	return path
}

// screenshotExportPathsEqual treats the history file and the download path as the same destination when they resolve to one file.
func screenshotExportPathsEqual(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return filepath.Clean(left) == filepath.Clean(right)
	}
	return strings.EqualFold(leftAbs, rightAbs)
}

func (state *screenshotEditorOverlayState) activeRecordingUI() *recordingToolbarState {
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.recordingUI
}

func (state *screenshotEditorOverlayState) completePin() {
	state.once.Do(func() {
		state.result <- screenshotEditorOverlayOutcome{pinned: true}
	})
}

func (state *screenshotEditorOverlayState) completeRecord() {
	state.once.Do(func() {
		state.result <- screenshotEditorOverlayOutcome{record: true}
	})
}

func (state *screenshotEditorOverlayState) completeColorCopy(value string) {
	state.once.Do(func() {
		state.result <- screenshotEditorOverlayOutcome{copiedColor: value}
	})
}

func normalizeScreenshotEditorRect(rect Rect, frame Size) Rect {
	left := min(rect.X, rect.X+rect.Width)
	top := min(rect.Y, rect.Y+rect.Height)
	right := max(rect.X, rect.X+rect.Width)
	bottom := max(rect.Y, rect.Y+rect.Height)
	left = min(max(float32(0), left), frame.Width)
	top = min(max(float32(0), top), frame.Height)
	right = min(max(float32(0), right), frame.Width)
	bottom = min(max(float32(0), bottom), frame.Height)
	return Rect{X: left, Y: top, Width: max(float32(0), right-left), Height: max(float32(0), bottom-top)}
}

// squareScreenshotEditorRectFromAnchor constrains a shape to equal sides without crossing its selection bounds.
func squareScreenshotEditorRectFromAnchor(anchor, point Point, bounds Rect) Rect {
	deltaX, deltaY := point.X-anchor.X, point.Y-anchor.Y
	directionX, directionY := float32(1), float32(1)
	if deltaX < 0 {
		directionX = -1
	}
	if deltaY < 0 {
		directionY = -1
	}
	maxWidth := bounds.X + bounds.Width - anchor.X
	if directionX < 0 {
		maxWidth = anchor.X - bounds.X
	}
	maxHeight := bounds.Y + bounds.Height - anchor.Y
	if directionY < 0 {
		maxHeight = anchor.Y - bounds.Y
	}
	absX := float32(math.Abs(float64(deltaX)))
	absY := float32(math.Abs(float64(deltaY)))
	side := min(max(absX, absY), min(maxWidth, maxHeight))
	end := Point{X: anchor.X + directionX*side, Y: anchor.Y + directionY*side}
	return Rect{X: min(anchor.X, end.X), Y: min(anchor.Y, end.Y), Width: side, Height: side}
}

func screenshotEditorRectContains(rect Rect, point Point) bool {
	return rect.Width > 0 && rect.Height > 0 && point.X >= rect.X && point.X < rect.X+rect.Width && point.Y >= rect.Y && point.Y < rect.Y+rect.Height
}

// startTextAnnotationEditLocked replaces a label in place while the shared text input handles new content.
func (state *screenshotEditorOverlayState) startTextAnnotationEditLocked(index int, point Point) {
	annotation := state.annotations[index]
	state.selectedAnnotation = index
	state.hasSelectedMark = true
	state.hasHoveredMark = false
	state.editMode = screenshotEditorEditNone
	state.textPosition = annotation.start
	state.textDraft = annotation.text
	state.textMarked = ""
	state.textCaret = screenshotEditorTextCaretIndex(annotation.text, point.X-annotation.start.X, screenshotEditorAnnotationRenderedFontSize(annotation, state.uiScale))
	state.textFontSize = screenshotEditorAnnotationFontSize(annotation)
	state.annotationColor = screenshotEditorAnnotationDrawColor(annotation)
	state.textEditing = true
	state.editingTextIndex = index
	state.hasEditingText = true
	state.showCaretLocked()
	state.pointerCursor = PointerCursorText
}

// beginTextAnnotationEditAtLocked preserves direct editing of labels while the text tool owns other shape interiors.
func (state *screenshotEditorOverlayState) beginTextAnnotationEditAtLocked(point Point) bool {
	if index, found := screenshotEditorTextAnnotationAt(state.annotations, point, state.uiScale); found &&
		screenshotEditorAnnotationContains(state.annotations[index], point, state.uiScale) {
		state.startTextAnnotationEditLocked(index, point)
		return true
	}
	return false
}

// beginAnnotationEditLocked gives existing marks priority over the active creation tool.
func (state *screenshotEditorOverlayState) beginAnnotationEditLocked(point Point) bool {
	state.start = point
	if state.hasHoveredMark && state.hoveredAnnotation >= 0 && state.hoveredAnnotation < len(state.annotations) {
		annotation := state.annotations[state.hoveredAnnotation]
		if handle, mode, found := screenshotEditorAnnotationHandleAt(annotation, point, state.uiScale); found {
			state.selectedAnnotation = state.hoveredAnnotation
			state.hasSelectedMark = true
			state.editMode = mode
			state.editHandle = handle
			state.editOriginalMark = annotation
			state.pointerCursor = screenshotEditorCursorForAnnotationHandle(handle, mode)
			return true
		}
	}
	if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
		annotation := state.annotations[state.selectedAnnotation]
		if handle, mode, found := screenshotEditorAnnotationHandleAt(annotation, point, state.uiScale); found {
			state.editMode = mode
			state.editHandle = handle
			state.editOriginalMark = annotation
			state.pointerCursor = screenshotEditorCursorForAnnotationHandle(handle, mode)
			return true
		}
	}
	if index, found := screenshotEditorAnnotationAt(state.annotations, point, state.uiScale); found {
		annotation := state.annotations[index]
		if annotation.tool == screenshotEditorToolText && screenshotEditorAnnotationContains(annotation, point, state.uiScale) {
			state.startTextAnnotationEditLocked(index, point)
			return true
		}
		state.selectedAnnotation = index
		state.hasSelectedMark = true
		state.editMode = screenshotEditorEditMoveAnnotation
		state.editOriginalMark = annotation
		state.pointerCursor = PointerCursorMove
		return true
	}
	state.hasSelectedMark = false
	return false
}

// beginSelectionEditLocked starts moving or resizing the capture selection.
func (state *screenshotEditorOverlayState) beginSelectionEditLocked(point Point) bool {
	state.start = point
	if handle, found := screenshotEditorHandleAt(state.selection, point, state.uiScale); found {
		state.editMode = screenshotEditorEditResizeSelection
		state.editHandle = handle
		state.editOriginalRect = state.selection
		state.pointerCursor = screenshotEditorCursorForHandle(handle)
		return true
	}
	if screenshotEditorRectContains(state.selection, point) {
		state.editMode = screenshotEditorEditMoveSelection
		state.editOriginalRect = state.selection
		state.pointerCursor = PointerCursorMove
		return true
	}
	return false
}

// screenshotEditorAnnotationHandleAt resolves shape handles and arrow endpoints to their edit modes.
func screenshotEditorAnnotationHandleAt(annotation screenshotEditorAnnotation, point Point, uiScale float32) (screenshotEditorHandle, screenshotEditorEditMode, bool) {
	switch annotation.tool {
	case screenshotEditorToolRect, screenshotEditorToolEllipse:
		if handle, found := screenshotEditorHandleAt(annotation.rect, point, uiScale); found {
			return handle, screenshotEditorEditResizeAnnotation, true
		}
	case screenshotEditorToolArrow:
		if screenshotEditorPointsNear(annotation.start, point, 12*max(float32(1), uiScale)) {
			return 0, screenshotEditorEditArrowStart, true
		}
		if screenshotEditorPointsNear(annotation.end, point, 12*max(float32(1), uiScale)) {
			return 0, screenshotEditorEditArrowEnd, true
		}
	}
	return 0, screenshotEditorEditNone, false
}

// updateHoverLocked keeps visible handles and the native cursor aligned with the topmost pointer target.
func (state *screenshotEditorOverlayState) updateHoverLocked(point Point) bool {
	previousAnnotation := state.hoveredAnnotation
	previousHasHoveredMark := state.hasHoveredMark
	previousHoveredTool := state.hoveredTool
	previousHasHoveredTool := state.hasHoveredTool
	previousHoveredAction := state.hoveredAction
	previousHasHoveredAction := state.hasHoveredAction
	previousCursor := state.pointerCursor
	state.hasHoveredMark = false
	state.hasHoveredTool = false
	state.hasHoveredAction = false
	state.pointerCursor = PointerCursorDefault
	for index := 1; index < len(state.toolRects); index++ {
		if screenshotEditorRectContains(state.toolRects[index], point) {
			state.hoveredTool = index
			state.hasHoveredTool = true
			return previousHasHoveredMark || previousHasHoveredAction || !previousHasHoveredTool || previousHoveredTool != index || previousCursor != PointerCursorDefault
		}
	}
	for _, target := range []struct {
		action screenshotEditorAction
		rect   Rect
	}{
		{screenshotEditorActionUndo, state.undoRect},
		{screenshotEditorActionScrollingCapture, state.scrollRect},
		{screenshotEditorActionCursor, state.cursorRect},
		{screenshotEditorActionPin, state.pinRect},
		{screenshotEditorActionRecord, state.recordRect},
		{screenshotEditorActionCancel, state.cancelRect},
		{screenshotEditorActionSave, state.saveRect},
		{screenshotEditorActionConfirm, state.confirmRect},
	} {
		if screenshotEditorRectContains(target.rect, point) {
			state.hoveredAction = target.action
			state.hasHoveredAction = true
			return previousHasHoveredMark || previousHasHoveredTool || !previousHasHoveredAction || previousHoveredAction != target.action || previousCursor != PointerCursorDefault
		}
	}
	toolHoverChanged := previousHasHoveredTool || previousHasHoveredAction
	if state.activeTool == screenshotEditorToolText && screenshotEditorRectContains(state.selection, point) {
		if index, found := screenshotEditorTextAnnotationAt(state.annotations, point, state.uiScale); found {
			state.hoveredAnnotation = index
			state.hasHoveredMark = true
			if screenshotEditorTextFrameBorderContains(state.annotations[index], point, state.uiScale) {
				state.pointerCursor = PointerCursorMove
			} else {
				state.pointerCursor = PointerCursorText
			}
			return toolHoverChanged || previousAnnotation != index || !previousHasHoveredMark || previousCursor != state.pointerCursor
		}
		state.pointerCursor = PointerCursorText
		return toolHoverChanged || previousHasHoveredMark || previousCursor != state.pointerCursor
	}

	if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
		index := state.selectedAnnotation
		if handle, mode, found := screenshotEditorAnnotationHandleAt(state.annotations[index], point, state.uiScale); found {
			state.hoveredAnnotation = index
			state.hasHoveredMark = true
			state.pointerCursor = screenshotEditorCursorForAnnotationHandle(handle, mode)
			return toolHoverChanged || previousAnnotation != index || !previousHasHoveredMark || previousCursor != state.pointerCursor
		}
	}
	if previousHasHoveredMark && state.hoveredAnnotation >= 0 && state.hoveredAnnotation < len(state.annotations) &&
		(!state.hasSelectedMark || state.hoveredAnnotation != state.selectedAnnotation) {
		index := state.hoveredAnnotation
		if handle, mode, found := screenshotEditorAnnotationHandleAt(state.annotations[index], point, state.uiScale); found {
			state.hoveredAnnotation = index
			state.hasHoveredMark = true
			state.pointerCursor = screenshotEditorCursorForAnnotationHandle(handle, mode)
			return toolHoverChanged || previousCursor != state.pointerCursor
		}
	}
	if index, found := screenshotEditorAnnotationAt(state.annotations, point, state.uiScale); found {
		annotation := state.annotations[index]
		state.hoveredAnnotation = index
		state.hasHoveredMark = true
		if annotation.tool == screenshotEditorToolText && !screenshotEditorTextFrameBorderContains(annotation, point, state.uiScale) {
			state.pointerCursor = PointerCursorText
		} else {
			state.pointerCursor = PointerCursorMove
		}
		return toolHoverChanged || previousAnnotation != index || !previousHasHoveredMark || previousCursor != state.pointerCursor
	}
	if (state.activeTool == screenshotEditorToolMosaic || state.activeTool == screenshotEditorToolNumber) && screenshotEditorRectContains(state.selection, point) {
		state.pointerCursor = PointerCursorCrosshair
	} else if state.activeTool == screenshotEditorToolText && screenshotEditorRectContains(state.selection, point) {
		state.pointerCursor = PointerCursorText
	} else if state.activeTool == screenshotEditorToolSelect {
		if handle, found := screenshotEditorHandleAt(state.selection, point, state.uiScale); found {
			state.pointerCursor = screenshotEditorCursorForHandle(handle)
		}
	}
	return toolHoverChanged || previousHasHoveredMark || previousCursor != state.pointerCursor
}

// screenshotEditorCursorForAnnotationHandle maps annotation edit affordances to native cursors.
func screenshotEditorCursorForAnnotationHandle(handle screenshotEditorHandle, mode screenshotEditorEditMode) PointerCursor {
	if mode == screenshotEditorEditArrowStart || mode == screenshotEditorEditArrowEnd {
		return PointerCursorCrosshair
	}
	return screenshotEditorCursorForHandle(handle)
}

// screenshotEditorCursorForHandle returns the directional cursor for one rectangular handle.
func screenshotEditorCursorForHandle(handle screenshotEditorHandle) PointerCursor {
	switch handle {
	case screenshotEditorHandleTop, screenshotEditorHandleBottom:
		return PointerCursorResizeVertical
	case screenshotEditorHandleLeft, screenshotEditorHandleRight:
		return PointerCursorResizeHorizontal
	case screenshotEditorHandleTopLeft, screenshotEditorHandleBottomRight:
		return PointerCursorResizeNWSE
	case screenshotEditorHandleTopRight, screenshotEditorHandleBottomLeft:
		return PointerCursorResizeNESW
	default:
		return PointerCursorDefault
	}
}

func (state *screenshotEditorOverlayState) updateSelectEditLocked(point Point, modifiers KeyModifiers) {
	delta := Point{X: point.X - state.start.X, Y: point.Y - state.start.Y}
	frameBounds := Rect{Width: state.frameSize.Width, Height: state.frameSize.Height}
	switch state.editMode {
	case screenshotEditorEditMoveSelection:
		state.selection = shiftScreenshotEditorRectWithinBounds(state.editOriginalRect, delta, frameBounds)
	case screenshotEditorEditResizeSelection:
		state.selection = resizeScreenshotEditorRect(state.editOriginalRect, state.editHandle, screenshotEditorDraggedHandlePoint(state.editOriginalRect, state.editHandle, delta), frameBounds)
	case screenshotEditorEditMoveAnnotation:
		if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			state.annotations[state.selectedAnnotation] = shiftScreenshotEditorAnnotationWithinBounds(state.editOriginalMark, delta, state.selection, state.uiScale)
		}
	case screenshotEditorEditResizeAnnotation:
		if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			annotation := state.editOriginalMark
			point := screenshotEditorDraggedHandlePoint(annotation.rect, state.editHandle, delta)
			if modifiers&KeyModifierShift != 0 && (annotation.tool == screenshotEditorToolRect || annotation.tool == screenshotEditorToolEllipse) {
				annotation.rect = resizeSquareScreenshotEditorRect(annotation.rect, state.editHandle, point, state.selection)
			} else {
				annotation.rect = resizeScreenshotEditorRect(annotation.rect, state.editHandle, point, state.selection)
			}
			state.annotations[state.selectedAnnotation] = annotation
		}
	case screenshotEditorEditArrowStart:
		if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			annotation := state.editOriginalMark
			annotation.start = clampScreenshotEditorPoint(point, state.selection)
			state.annotations[state.selectedAnnotation] = annotation
		}
	case screenshotEditorEditArrowEnd:
		if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			annotation := state.editOriginalMark
			annotation.end = clampScreenshotEditorPoint(point, state.selection)
			state.annotations[state.selectedAnnotation] = annotation
		}
	}
}

// screenshotEditorDraggedHandlePoint preserves where inside a resize handle the pointer was pressed.
func screenshotEditorDraggedHandlePoint(rect Rect, handle screenshotEditorHandle, delta Point) Point {
	point := screenshotEditorRectHandlePoints(rect)[int(handle)]
	return Point{X: point.X + delta.X, Y: point.Y + delta.Y}
}

// resizeSquareScreenshotEditorRect constrains any shape handle to an equal-sided result.
func resizeSquareScreenshotEditorRect(original Rect, handle screenshotEditorHandle, point Point, bounds Rect) Rect {
	point = clampScreenshotEditorPoint(point, bounds)
	left, top := original.X, original.Y
	right, bottom := original.X+original.Width, original.Y+original.Height
	switch handle {
	case screenshotEditorHandleTopLeft:
		return squareScreenshotEditorRectFromAnchor(Point{X: right, Y: bottom}, point, bounds)
	case screenshotEditorHandleTopRight:
		return squareScreenshotEditorRectFromAnchor(Point{X: left, Y: bottom}, point, bounds)
	case screenshotEditorHandleBottomRight:
		return squareScreenshotEditorRectFromAnchor(Point{X: left, Y: top}, point, bounds)
	case screenshotEditorHandleBottomLeft:
		return squareScreenshotEditorRectFromAnchor(Point{X: right, Y: top}, point, bounds)
	case screenshotEditorHandleTop, screenshotEditorHandleBottom:
		anchorY := top
		if handle == screenshotEditorHandleTop {
			anchorY = bottom
		}
		side := float32(math.Abs(float64(point.Y - anchorY)))
		centerX := left + original.Width/2
		side = min(side, 2*min(centerX-bounds.X, bounds.X+bounds.Width-centerX))
		y := anchorY
		if point.Y < anchorY {
			y -= side
		}
		return Rect{X: centerX - side/2, Y: y, Width: side, Height: side}
	case screenshotEditorHandleLeft, screenshotEditorHandleRight:
		anchorX := left
		if handle == screenshotEditorHandleLeft {
			anchorX = right
		}
		side := float32(math.Abs(float64(point.X - anchorX)))
		centerY := top + original.Height/2
		side = min(side, 2*min(centerY-bounds.Y, bounds.Y+bounds.Height-centerY))
		x := anchorX
		if point.X < anchorX {
			x -= side
		}
		return Rect{X: x, Y: centerY - side/2, Width: side, Height: side}
	default:
		return original
	}
}

func screenshotEditorHandleAt(rect Rect, point Point, uiScale float32) (screenshotEditorHandle, bool) {
	for index, handlePoint := range screenshotEditorRectHandlePoints(rect) {
		if screenshotEditorPointsNear(handlePoint, point, 12*max(float32(1), uiScale)) {
			return screenshotEditorHandle(index), true
		}
	}
	return 0, false
}

func screenshotEditorPointsNear(left, right Point, tolerance float32) bool {
	return math.Hypot(float64(left.X-right.X), float64(left.Y-right.Y)) <= float64(tolerance)
}

func resizeScreenshotEditorRect(original Rect, handle screenshotEditorHandle, point Point, bounds Rect) Rect {
	point = clampScreenshotEditorPoint(point, bounds)
	left, top := original.X, original.Y
	right, bottom := original.X+original.Width, original.Y+original.Height
	switch handle {
	case screenshotEditorHandleTopLeft:
		left, top = point.X, point.Y
	case screenshotEditorHandleTop:
		top = point.Y
	case screenshotEditorHandleTopRight:
		right, top = point.X, point.Y
	case screenshotEditorHandleRight:
		right = point.X
	case screenshotEditorHandleBottomRight:
		right, bottom = point.X, point.Y
	case screenshotEditorHandleBottom:
		bottom = point.Y
	case screenshotEditorHandleBottomLeft:
		left, bottom = point.X, point.Y
	case screenshotEditorHandleLeft:
		left = point.X
	}
	return Rect{X: min(left, right), Y: min(top, bottom), Width: max(float32(0), max(left, right)-min(left, right)), Height: max(float32(0), max(top, bottom)-min(top, bottom))}
}

func shiftScreenshotEditorRectWithinBounds(rect Rect, delta Point, bounds Rect) Rect {
	x := min(max(rect.X+delta.X, bounds.X), bounds.X+bounds.Width-rect.Width)
	y := min(max(rect.Y+delta.Y, bounds.Y), bounds.Y+bounds.Height-rect.Height)
	return Rect{X: x, Y: y, Width: rect.Width, Height: rect.Height}
}

func shiftScreenshotEditorAnnotationWithinBounds(annotation screenshotEditorAnnotation, delta Point, bounds Rect, uiScale float32) screenshotEditorAnnotation {
	annotationBounds := screenshotEditorAnnotationBounds(annotation, uiScale)
	shiftedBounds := shiftScreenshotEditorRectWithinBounds(annotationBounds, delta, bounds)
	clampedDelta := Point{X: shiftedBounds.X - annotationBounds.X, Y: shiftedBounds.Y - annotationBounds.Y}
	annotation.points = append([]Point(nil), annotation.points...)
	annotation.rect.X += clampedDelta.X
	annotation.rect.Y += clampedDelta.Y
	annotation.start.X += clampedDelta.X
	annotation.start.Y += clampedDelta.Y
	annotation.end.X += clampedDelta.X
	annotation.end.Y += clampedDelta.Y
	for index := range annotation.points {
		annotation.points[index].X += clampedDelta.X
		annotation.points[index].Y += clampedDelta.Y
	}
	return annotation
}

func clampScreenshotEditorPoint(point Point, bounds Rect) Point {
	return Point{
		X: min(max(point.X, bounds.X), bounds.X+bounds.Width),
		Y: min(max(point.Y, bounds.Y), bounds.Y+bounds.Height),
	}
}

func screenshotEditorAnnotationIsVisible(annotation screenshotEditorAnnotation) bool {
	switch annotation.tool {
	case screenshotEditorToolRect, screenshotEditorToolEllipse:
		return annotation.rect.Width >= 2 && annotation.rect.Height >= 2
	case screenshotEditorToolArrow:
		return math.Hypot(float64(annotation.end.X-annotation.start.X), float64(annotation.end.Y-annotation.start.Y)) >= 2
	case screenshotEditorToolMosaic:
		return len(annotation.points) > 0
	case screenshotEditorToolNumber:
		return annotation.number > 0
	case screenshotEditorToolText:
		return annotation.text != ""
	default:
		return false
	}
}
