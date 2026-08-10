package screenshot

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"wox/common"
	woxsvg "wox/util/svg"
)

type screenshotEditorOverlayOutcome struct {
	cancelled bool
	pinned    bool
}

type screenshotEditorTool uint8

const (
	screenshotEditorToolSelect screenshotEditorTool = iota
	screenshotEditorToolRect
	screenshotEditorToolEllipse
	screenshotEditorToolText
	screenshotEditorToolArrow
	screenshotEditorToolMosaic
)

var screenshotEditorToolIconNames = [...]string{
	"screenshot.select",
	"screenshot.rectangle",
	"screenshot.ellipse",
	"screenshot.text",
	"screenshot.arrow",
	"screenshot.mosaic",
}

type screenshotEditorIconCacheKey struct {
	name  string
	color Color
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
	mu                 sync.Mutex
	once               sync.Once
	window             *Window
	image              *Image
	frameSize          Size
	workspaceSize      Size
	start              Point
	selection          Rect
	confirmRect        Rect
	cancelRect         Rect
	pinRect            Rect
	undoRect           Rect
	scrollRect         Rect
	cursorRect         Rect
	toolbarRect        Rect
	toolRects          [6]Rect
	editBarRect        Rect
	editColorRects     [6]Rect
	editSizeRects      [3]Rect
	editDecreaseRect   Rect
	editIncreaseRect   Rect
	editDeleteRect     Rect
	activeTool         screenshotEditorTool
	annotations        []screenshotEditorAnnotation
	draft              *screenshotEditorAnnotation
	editMode           screenshotEditorEditMode
	editHandle         screenshotEditorHandle
	editOriginalRect   Rect
	editOriginalMark   screenshotEditorAnnotation
	selectedAnnotation int
	hasSelectedMark    bool
	textPosition       Point
	textDraft          string
	textMarked         string
	textEditing        bool
	dragging           bool
	annotationDragging bool
	hasSelection       bool
	autoConfirm        bool
	hideTools          bool
	scrolling          bool
	scrollingStarting  bool
	scrollingFrames    []screenshotScrollingFrame
	scrollingPreview   *Image
	scrollingStop      chan struct{}
	scrollingDone      chan struct{}
	scrollingStopOnce  sync.Once
	scrollingOverlaps  bool
	startScrolling     func()
	annotationColor    Color
	mosaicRadius       float32
	textFontSize       float32
	cursorPixel        *Point
	showCursor         bool
	uiScale            float32
	chromeScale        func(selection Rect) float32
	result             chan screenshotEditorOverlayOutcome
}

type screenshotEditorPlatform struct {
	setWindowBounds  func(window *Window) error
	logicalSelection func(selection Rect, frameSize Size) Rect
	captureDesktop   func() (screenshotDesktopCapture, error)
	frameSize        Size
	initialSelection *Rect
	cursorPixel      *Point
	afterShow        func()
	chromeScale      func(selection Rect) float32
	preparedWindow   *ManagedWindow
	windowHost       *screenshotEditorWindowHost
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
		image:           uiImage,
		frameSize:       platform.frameSize,
		autoConfirm:     options.AutoConfirm,
		hideTools:       options.HideAnnotationToolbar,
		annotationColor: screenshotEditorAnnotationColor,
		mosaicRadius:    screenshotEditorMosaicRadius,
		textFontSize:    screenshotEditorTextFontSize,
		cursorPixel:     platform.cursorPixel,
		uiScale:         1,
		chromeScale:     platform.chromeScale,
		result:          make(chan screenshotEditorOverlayOutcome, 1),
		scrollingStop:   make(chan struct{}),
	}
	if platform.initialSelection != nil {
		state.selection = normalizeScreenshotEditorRect(*platform.initialSelection, platform.frameSize)
		state.hasSelection = state.selection.Width >= 2 && state.selection.Height >= 2
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
	if options.ExportFilePath == "" {
		return ScreenshotResult{}, errors.New("screenshot export file path is empty")
	}
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
		state.window = overlay
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
	state.mu.Unlock()
	if scrolling {
		stitched, stitchErr := stitchScreenshotScrollingFrames(scrollingFrames)
		if stitchErr != nil {
			return ScreenshotResult{}, stitchErr
		}
		if err := writeScreenshotPNG(options.ExportFilePath, stitched); err != nil {
			return ScreenshotResult{}, err
		}
	} else {
		pixelSelection, err := screenshotEditorPixelSelection(source.Bounds(), selection, frameSize)
		if err != nil {
			return ScreenshotResult{}, err
		}
		composited, err := renderScreenshotEditorAnnotations(source, annotations, selection, frameSize)
		if err != nil {
			return ScreenshotResult{}, err
		}
		if showCursor && cursorPixel != nil {
			if err := renderScreenshotEditorCursor(composited, *cursorPixel, selection, frameSize); err != nil {
				return ScreenshotResult{}, err
			}
		}
		if err := writeScreenshotEditorPNG(options.ExportFilePath, composited, pixelSelection); err != nil {
			return ScreenshotResult{}, err
		}
	}

	result := ScreenshotResult{
		PinToScreen:             outcome.pinned,
		ScreenshotPath:          options.ExportFilePath,
		ClipboardWriteSucceeded: outcome.pinned || !options.CopyToClipboard,
		LogicalSelection:        platform.logicalSelection(selection, frameSize),
	}
	if options.CopyToClipboard && !outcome.pinned {
		if err := overlay.WriteClipboardImageFile(options.ExportFilePath); err != nil {
			result.ClipboardWarningMessage = err.Error()
		} else {
			result.ClipboardWriteSucceeded = true
		}
	}
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

func writeScreenshotEditorPNG(path string, source image.Image, selection image.Rectangle) error {
	cropped := image.NewRGBA(image.Rect(0, 0, selection.Dx(), selection.Dy()))
	draw.Draw(cropped, cropped.Bounds(), source, selection.Min, draw.Src)
	return writeScreenshotPNG(path, cropped)
}

func writeScreenshotPNG(path string, source image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create screenshot export directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create screenshot export file: %w", err)
	}
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode screenshot PNG: %w", err)
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
		frameCount := len(state.scrollingFrames)
		uiScale := max(float32(1), state.uiScale)
		state.confirmRect = Rect{X: frame.Size.Width - 64*uiScale, Y: frame.Size.Height - 56*uiScale, Width: 40 * uiScale, Height: 40 * uiScale}
		state.cancelRect = Rect{X: 24 * uiScale, Y: frame.Size.Height - 56*uiScale, Width: 40 * uiScale, Height: 40 * uiScale}
		state.toolbarRect = Rect{Y: frame.Size.Height - 72*uiScale, Width: frame.Size.Width, Height: 72 * uiScale}
		state.mu.Unlock()
		drawScreenshotScrollingControls(displayList, frame.Size, preview, frameCount, uiScale)
		return
	}
	state.frameSize = frame.Size
	selection := normalizeScreenshotEditorRect(state.selection, frame.Size)
	hasSelection := state.hasSelection || state.dragging
	uiScale := float32(1)
	if hasSelection && state.chromeScale != nil {
		uiScale = max(float32(1), state.chromeScale(selection))
	}
	state.uiScale = uiScale
	scaled := func(value float32) float32 { return value * uiScale }
	dragging := state.dragging
	activeTool := state.activeTool
	hideTools := state.hideTools
	annotationColor := state.annotationColor
	mosaicRadius := state.mosaicRadius
	textFontSize := state.textFontSize
	scrollingStarting := state.scrollingStarting
	cursorPixel := state.cursorPixel
	showCursor := state.showCursor
	annotations := append([]screenshotEditorAnnotation(nil), state.annotations...)
	selectedAnnotation := state.selectedAnnotation
	hasSelectedMark := state.hasSelectedMark && selectedAnnotation >= 0 && selectedAnnotation < len(state.annotations)
	if state.draft != nil {
		annotations = append(annotations, *state.draft)
	}
	if state.textEditing && state.textDraft+state.textMarked != "" {
		annotations = append(annotations, screenshotEditorAnnotation{
			tool: state.activeTool, start: state.textPosition, text: state.textDraft + state.textMarked,
			color: annotationColor, fontSize: textFontSize,
		})
	}
	state.confirmRect = Rect{}
	state.cancelRect = Rect{}
	state.pinRect = Rect{}
	state.undoRect = Rect{}
	state.scrollRect = Rect{}
	state.cursorRect = Rect{}
	state.toolbarRect = Rect{}
	state.toolRects = [6]Rect{}
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
		return
	}
	displayList.FillRect(Rect{Width: frame.Size.Width, Height: selection.Y}, dim)
	displayList.FillRect(Rect{Y: selection.Y + selection.Height, Width: frame.Size.Width, Height: max(float32(0), frame.Size.Height-selection.Y-selection.Height)}, dim)
	displayList.FillRect(Rect{Y: selection.Y, Width: selection.X, Height: selection.Height}, dim)
	displayList.FillRect(Rect{X: selection.X + selection.Width, Y: selection.Y, Width: max(float32(0), frame.Size.Width-selection.X-selection.Width), Height: selection.Height}, dim)

	displayList.PushClipRect(selection)
	drawScreenshotEditorAnnotations(displayList, annotations, state.image, frame.Size)
	if showCursor && cursorPixel != nil {
		drawScreenshotEditorCursor(displayList, screenshotEditorCursorLogicalPoint(*cursorPixel, state.image, frame.Size))
	}
	if hasSelectedMark {
		drawScreenshotEditorAnnotationHandles(displayList, annotations[selectedAnnotation], uiScale)
	}
	displayList.PopClipRect()

	green := Color{R: 41, G: 255, B: 114, A: 255}
	displayList.StrokeRoundedRect(selection, 0, scaled(2), green)
	drawScreenshotEditorHandles(displayList, selection, green, uiScale)
	label := fmt.Sprintf("%.0f x %.0f", selection.Width, selection.Height)
	labelWidth := max(scaled(80), float32(len(label))*scaled(8)+scaled(16))
	labelLeft := min(max(scaled(8), selection.X+scaled(12)), max(scaled(8), frame.Size.Width-labelWidth-scaled(8)))
	labelTop := max(scaled(8), selection.Y-scaled(32))
	displayList.FillRoundedRect(Rect{X: labelLeft - scaled(8), Y: labelTop, Width: labelWidth, Height: scaled(26)}, scaled(10), Color{R: 23, G: 23, B: 23, A: 230})
	displayList.DrawText(label, Rect{X: labelLeft, Y: labelTop + scaled(5), Width: labelWidth - scaled(16), Height: scaled(18)}, TextStyle{Size: scaled(14), Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: 255})
	if dragging || state.autoConfirm {
		return
	}

	toolbarWidth := scaled(128)
	if !hideTools {
		toolbarWidth = scaled(632)
	}
	toolbarHeight := scaled(60)
	toolbarLeft := min(max(scaled(24), selection.X+selection.Width-toolbarWidth), max(scaled(24), frame.Size.Width-toolbarWidth-scaled(24)))
	toolbarTop := selection.Y + selection.Height + scaled(16)
	if toolbarTop+toolbarHeight > frame.Size.Height-scaled(24) {
		toolbarTop = max(scaled(24), selection.Y-toolbarHeight-scaled(16))
	}
	toolbarRect := Rect{X: toolbarLeft, Y: toolbarTop, Width: toolbarWidth, Height: toolbarHeight}
	slotLeft := toolbarLeft + scaled(16)
	var toolRects [6]Rect
	pinRect := Rect{}
	undoRect := Rect{}
	scrollRect := Rect{}
	cursorRect := Rect{}
	if !hideTools {
		for index := range toolRects {
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
		slotLeft += scaled(48)
	}
	cancelRect := Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
	slotLeft += scaled(48)
	confirmRect := Rect{X: slotLeft + scaled(4), Y: toolbarTop + scaled(10), Width: scaled(40), Height: scaled(40)}
	state.mu.Lock()
	state.toolbarRect = toolbarRect
	state.toolRects = toolRects
	state.pinRect = pinRect
	state.undoRect = undoRect
	state.scrollRect = scrollRect
	state.cursorRect = cursorRect
	state.cancelRect = cancelRect
	state.confirmRect = confirmRect
	state.mu.Unlock()

	displayList.FillRoundedRect(toolbarRect, scaled(18), Color{R: 30, G: 26, B: 24, A: 204})
	if !hideTools {
		for index, rect := range toolRects {
			selected := screenshotEditorTool(index) == activeTool
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
		drawScreenshotEditorToolbarIcon(displayList, "screenshot.cursor", cursorRect, cursorColor, uiScale)
		drawScreenshotEditorToolbarIcon(displayList, "screenshot.pin", pinRect, Color{R: 255, G: 255, B: 255, A: 255}, uiScale)
	}
	drawScreenshotEditorToolbarIcon(displayList, "control.close", cancelRect, Color{R: 255, G: 107, B: 107, A: 255}, uiScale)
	drawScreenshotEditorToolbarIcon(displayList, "control.check", confirmRect, Color{R: 48, G: 227, B: 122, A: 255}, uiScale)
	if !hideTools {
		var selectedMark *screenshotEditorAnnotation
		if hasSelectedMark {
			selectedMark = &annotations[selectedAnnotation]
		}
		state.drawEditBar(displayList, frame.Size, selection, activeTool, selectedMark, annotationColor, mosaicRadius, textFontSize, uiScale)
	}
}

// drawEditBar mirrors Flutter's creation settings and selected-annotation actions beside the selection.
func (state *screenshotEditorOverlayState) drawEditBar(
	displayList *DisplayList,
	frame Size,
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
	color := creationColor
	mosaicRadius := creationMosaicRadius
	textSize := creationTextSize
	anchor := selection
	if selected != nil {
		isMosaic = selected.tool == screenshotEditorToolMosaic
		color = screenshotEditorAnnotationDrawColor(*selected)
		mosaicRadius = screenshotEditorAnnotationMosaicRadius(*selected)
		textSize = screenshotEditorAnnotationFontSize(*selected)
		anchor = screenshotEditorAnnotationBounds(*selected)
	}

	scaled := func(value float32) float32 { return value * uiScale }
	width := scaled(92)
	height := scaled(100)
	if isMosaic {
		width, height = scaled(116), scaled(56)
	}
	if selected != nil {
		height += scaled(64)
		if selected.tool == screenshotEditorToolText {
			height += scaled(106)
		}
	}
	left := selection.X + selection.Width + scaled(16)
	if left+width > frame.Width-scaled(24) {
		left = max(scaled(24), selection.X-width-scaled(16))
	}
	top := min(max(scaled(24), anchor.Y+anchor.Height/2-height/2), max(scaled(24), frame.Height-height-scaled(24)))
	bar := Rect{X: left, Y: top, Width: width, Height: height}
	displayList.FillRoundedRect(bar, scaled(18), Color{R: 27, G: 23, B: 21, A: 217})

	var colorRects [6]Rect
	var sizeRects [3]Rect
	decreaseRect, increaseRect, deleteRect := Rect{}, Rect{}, Rect{}
	cursorY := top + scaled(12)
	green := Color{R: 41, G: 255, B: 114, A: 255}
	if isMosaic {
		for index, radius := range screenshotEditorMosaicRadii {
			rect := Rect{X: left + scaled(10+float32(index)*32), Y: cursorY, Width: scaled(32), Height: scaled(32)}
			sizeRects[index] = rect
			visualRadius := scaled(4 + radius/screenshotEditorMosaicRadii[len(screenshotEditorMosaicRadii)-1]*6)
			strokeColor := Color{R: 255, G: 255, B: 255, A: 179}
			if math.Abs(float64(radius-mosaicRadius)) < 0.1 {
				strokeColor = green
			}
			circle := Rect{X: rect.X + rect.Width/2 - visualRadius, Y: rect.Y + rect.Height/2 - visualRadius, Width: visualRadius * 2, Height: visualRadius * 2}
			displayList.FillRoundedRect(circle, visualRadius, Color{R: strokeColor.R, G: strokeColor.G, B: strokeColor.B, A: 42})
			displayList.StrokeRoundedRect(circle, visualRadius, scaled(2), strokeColor)
		}
		cursorY += scaled(42)
	} else {
		for index, swatch := range screenshotEditorPalette {
			rect := Rect{X: left + scaled(20+float32(index%2)*32), Y: cursorY + scaled(float32(index/2)*28), Width: scaled(20), Height: scaled(20)}
			colorRects[index] = rect
			displayList.FillRoundedRect(rect, scaled(10), swatch)
			outline := Color{R: 255, G: 255, B: 255, A: 61}
			if swatch == color {
				outline = green
			}
			displayList.StrokeRoundedRect(rect, scaled(10), scaled(2), outline)
		}
		cursorY += scaled(88)
	}

	if selected != nil && selected.tool == screenshotEditorToolText {
		decreaseRect = Rect{X: left + (width-scaled(42))/2, Y: cursorY, Width: scaled(42), Height: scaled(42)}
		cursorY += scaled(42)
		displayList.FillRoundedRect(decreaseRect, scaled(10), Color{R: 255, G: 255, B: 255, A: 34})
		drawScreenshotEditorToolbarIcon(displayList, "control.remove", decreaseRect, Color{R: 255, G: 255, B: 255, A: 255}, uiScale)
		displayList.DrawText(fmt.Sprintf("%.0f", textSize), Rect{X: left, Y: cursorY + scaled(2), Width: width, Height: scaled(18)}, TextStyle{Size: scaled(12), Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: 255})
		cursorY += scaled(22)
		increaseRect = Rect{X: left + (width-scaled(42))/2, Y: cursorY, Width: scaled(42), Height: scaled(42)}
		cursorY += scaled(52)
		displayList.FillRoundedRect(increaseRect, scaled(10), Color{R: 255, G: 255, B: 255, A: 34})
		drawScreenshotEditorToolbarIcon(displayList, "control.add", increaseRect, Color{R: 255, G: 255, B: 255, A: 255}, uiScale)
	}
	if selected != nil {
		deleteRect = Rect{X: left + (width-scaled(42))/2, Y: cursorY, Width: scaled(42), Height: scaled(42)}
		displayList.FillRoundedRect(deleteRect, scaled(10), Color{R: 255, G: 255, B: 255, A: 34})
		drawScreenshotEditorToolbarIcon(displayList, "control.delete", deleteRect, Color{R: 255, G: 107, B: 107, A: 255}, uiScale)
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

// drawScreenshotEditorToolbarIcon renders the shared SVG at a consistent visual size.
func drawScreenshotEditorToolbarIcon(displayList *DisplayList, name string, rect Rect, color Color, uiScale float32) {
	inset := 8 * uiScale
	size := 24 * uiScale
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

// drawScreenshotEditorCursor previews the captured pointer with the same marker exported to PNG.
func drawScreenshotEditorCursor(displayList *DisplayList, hotspot Point) {
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
	if event.Button != PointerButtonPrimary && event.Kind != PointerMove {
		return
	}
	switch event.Kind {
	case PointerDown:
		state.mu.Lock()
		confirm := state.hasSelection && screenshotEditorRectContains(state.confirmRect, event.Position)
		cancel := state.hasSelection && screenshotEditorRectContains(state.cancelRect, event.Position)
		pin := state.hasSelection && screenshotEditorRectContains(state.pinRect, event.Position)
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
			if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
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
		if screenshotEditorRectContains(state.editDecreaseRect, event.Position) && state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			annotation := &state.annotations[state.selectedAnnotation]
			annotation.fontSize = max(float32(12), screenshotEditorAnnotationFontSize(*annotation)-2)
			editChanged = true
		}
		if screenshotEditorRectContains(state.editIncreaseRect, event.Position) && state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			annotation := &state.annotations[state.selectedAnnotation]
			annotation.fontSize = min(float32(48), screenshotEditorAnnotationFontSize(*annotation)+2)
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
				switch state.activeTool {
				case screenshotEditorToolSelect:
					state.commitTextLocked()
					if !state.beginSelectEditLocked(event.Position) {
						state.start = event.Position
						state.selection = Rect{X: event.Position.X, Y: event.Position.Y}
						state.dragging = true
						state.hasSelection = false
						state.annotations = nil
						state.hasSelectedMark = false
					}
				case screenshotEditorToolText:
					if screenshotEditorRectContains(state.selection, event.Position) {
						state.commitTextLocked()
						state.textPosition = event.Position
						state.textEditing = true
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
		textEditing := state.textEditing
		state.mu.Unlock()
		if scroll {
			state.invalidate()
			go state.startScrolling()
		} else if confirm {
			state.commitText()
			state.complete(false)
		} else if cancel {
			state.complete(true)
		} else if pin {
			state.commitText()
			state.completePin()
		} else if cursor || editChanged || toolChanged || undo || (!toolbar && !editBar) {
			state.setTextInputEnabled(textEditing)
			state.invalidate()
		}
	case PointerMove:
		state.mu.Lock()
		if state.dragging {
			state.selection = Rect{X: state.start.X, Y: state.start.Y, Width: event.Position.X - state.start.X, Height: event.Position.Y - state.start.Y}
		} else if state.editMode != screenshotEditorEditNone {
			state.updateSelectEditLocked(event.Position)
		} else if state.annotationDragging && state.draft != nil {
			position := clampScreenshotEditorPoint(event.Position, state.selection)
			switch state.draft.tool {
			case screenshotEditorToolRect, screenshotEditorToolEllipse:
				state.draft.rect = normalizeScreenshotEditorRect(Rect{X: state.start.X, Y: state.start.Y, Width: position.X - state.start.X, Height: position.Y - state.start.Y}, state.frameSize)
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
			state.mu.Unlock()
			return
		}
		state.mu.Unlock()
		state.invalidate()
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
		if state.textEditing && state.textDraft != "" {
			_, size := utf8.DecodeLastRuneInString(state.textDraft)
			state.textDraft = state.textDraft[:len(state.textDraft)-size]
			state.mu.Unlock()
			state.invalidate()
			return true
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
	return false
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
		state.textDraft += event.Text
		state.textMarked = ""
	}
	state.mu.Unlock()
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
		state.annotations = append(state.annotations, screenshotEditorAnnotation{
			tool: state.activeTool, start: state.textPosition, text: state.textDraft,
			color: state.annotationColor, fontSize: state.textFontSize,
		})
	}
	state.textEditing = false
	state.textDraft = ""
	state.textMarked = ""
}

func (state *screenshotEditorOverlayState) setTextInputEnabled(enabled bool) {
	if state.window == nil {
		return
	}
	cursor := Rect{X: state.textPosition.X, Y: state.textPosition.Y, Width: 1, Height: 24}
	_ = state.window.SetTextInputState(TextInputState{Enabled: enabled, CursorRect: cursor})
}

func (state *screenshotEditorOverlayState) invalidate() {
	if state.window != nil {
		_ = state.window.Invalidate()
	}
}

func (state *screenshotEditorOverlayState) complete(cancelled bool) {
	state.once.Do(func() {
		state.result <- screenshotEditorOverlayOutcome{cancelled: cancelled}
	})
}

func (state *screenshotEditorOverlayState) completePin() {
	state.once.Do(func() {
		state.result <- screenshotEditorOverlayOutcome{pinned: true}
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

func screenshotEditorRectContains(rect Rect, point Point) bool {
	return rect.Width > 0 && rect.Height > 0 && point.X >= rect.X && point.X < rect.X+rect.Width && point.Y >= rect.Y && point.Y < rect.Y+rect.Height
}

func (state *screenshotEditorOverlayState) beginSelectEditLocked(point Point) bool {
	state.start = point
	if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
		annotation := state.annotations[state.selectedAnnotation]
		switch annotation.tool {
		case screenshotEditorToolRect, screenshotEditorToolEllipse:
			if handle, found := screenshotEditorHandleAt(annotation.rect, point, state.uiScale); found {
				state.editMode = screenshotEditorEditResizeAnnotation
				state.editHandle = handle
				state.editOriginalMark = annotation
				return true
			}
		case screenshotEditorToolArrow:
			if screenshotEditorPointsNear(annotation.start, point, 12) {
				state.editMode = screenshotEditorEditArrowStart
				state.editOriginalMark = annotation
				return true
			}
			if screenshotEditorPointsNear(annotation.end, point, 12) {
				state.editMode = screenshotEditorEditArrowEnd
				state.editOriginalMark = annotation
				return true
			}
		}
	}
	for index := len(state.annotations) - 1; index >= 0; index-- {
		if screenshotEditorAnnotationContains(state.annotations[index], point) {
			state.selectedAnnotation = index
			state.hasSelectedMark = true
			state.editMode = screenshotEditorEditMoveAnnotation
			state.editOriginalMark = state.annotations[index]
			return true
		}
	}
	state.hasSelectedMark = false
	if handle, found := screenshotEditorHandleAt(state.selection, point, state.uiScale); found {
		state.editMode = screenshotEditorEditResizeSelection
		state.editHandle = handle
		state.editOriginalRect = state.selection
		return true
	}
	if screenshotEditorRectContains(state.selection, point) {
		state.editMode = screenshotEditorEditMoveSelection
		state.editOriginalRect = state.selection
		return true
	}
	return false
}

func (state *screenshotEditorOverlayState) updateSelectEditLocked(point Point) {
	delta := Point{X: point.X - state.start.X, Y: point.Y - state.start.Y}
	frameBounds := Rect{Width: state.frameSize.Width, Height: state.frameSize.Height}
	switch state.editMode {
	case screenshotEditorEditMoveSelection:
		state.selection = shiftScreenshotEditorRectWithinBounds(state.editOriginalRect, delta, frameBounds)
	case screenshotEditorEditResizeSelection:
		state.selection = resizeScreenshotEditorRect(state.editOriginalRect, state.editHandle, point, frameBounds)
	case screenshotEditorEditMoveAnnotation:
		if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			state.annotations[state.selectedAnnotation] = shiftScreenshotEditorAnnotationWithinBounds(state.editOriginalMark, delta, state.selection)
		}
	case screenshotEditorEditResizeAnnotation:
		if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
			annotation := state.editOriginalMark
			annotation.rect = resizeScreenshotEditorRect(annotation.rect, state.editHandle, point, state.selection)
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

func shiftScreenshotEditorAnnotationWithinBounds(annotation screenshotEditorAnnotation, delta Point, bounds Rect) screenshotEditorAnnotation {
	annotationBounds := screenshotEditorAnnotationBounds(annotation)
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
	case screenshotEditorToolText:
		return annotation.text != ""
	default:
		return false
	}
}
