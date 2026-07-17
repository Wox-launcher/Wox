//go:build windows

package woxui

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

const captureBlt = uint32(0x40000000)

type screenshotOverlayOutcome struct {
	cancelled bool
}

type screenshotOverlayState struct {
	mu           sync.Mutex
	once         sync.Once
	window       *Window
	image        *Image
	frameSize    Size
	start        Point
	selection    Rect
	confirmRect  Rect
	cancelRect   Rect
	dragging     bool
	hasSelection bool
	autoConfirm  bool
	result       chan screenshotOverlayOutcome
}

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	if options.ExportFilePath == "" {
		return ScreenshotResult{}, errors.New("screenshot export file path is empty")
	}
	// Give DWM one frame to remove the launcher before copying the desktop pixels.
	time.Sleep(80 * time.Millisecond)
	source, virtualBounds, err := captureWindowsVirtualDesktop()
	if err != nil {
		return ScreenshotResult{}, err
	}
	uiImage, err := NewImage(source)
	if err != nil {
		return ScreenshotResult{}, fmt.Errorf("prepare screenshot overlay image: %w", err)
	}
	state := &screenshotOverlayState{image: uiImage, autoConfirm: options.AutoConfirm, result: make(chan screenshotOverlayOutcome, 1)}
	manager := options.WindowManager
	if manager == nil {
		manager = NewWindowManager()
	}
	var managed *ManagedWindow
	var created bool
	var openErr error
	err = Call(func() {
		managed, created, openErr = manager.Open(ScreenshotWindowID, WindowOptions{
			Title:      "Wox Screenshot",
			Size:       Size{Width: 100, Height: 100},
			HideOnBlur: false,
			OnFrame:    state.draw,
			OnPointer:  state.pointer,
			OnKey:      state.key,
			OnClosed:   func() { state.complete(true) },
		})
		if openErr != nil {
			return
		}
		if !created {
			openErr = errors.New("a screenshot window is already active")
			return
		}
		overlay := managed.Window()
		state.window = overlay
		openErr = overlay.native.setPhysicalBounds(Rect{X: float32(virtualBounds.Min.X), Y: float32(virtualBounds.Min.Y), Width: float32(virtualBounds.Dx()), Height: float32(virtualBounds.Dy())})
		if openErr == nil {
			_, openErr = managed.Show()
		}
	})
	if err != nil {
		if created && managed != nil {
			_ = managed.Close()
		}
		return ScreenshotResult{}, err
	}
	if openErr != nil {
		if created && managed != nil {
			_ = managed.Close()
		}
		return ScreenshotResult{}, openErr
	}
	overlay := managed.Window()
	defer managed.Close()

	var outcome screenshotOverlayOutcome
	select {
	case outcome = <-state.result:
	case <-time.After(175 * time.Second):
		outcome.cancelled = true
	}
	if outcome.cancelled {
		return ScreenshotResult{Cancelled: true}, nil
	}

	state.mu.Lock()
	selection := state.selection
	frameSize := state.frameSize
	state.mu.Unlock()
	if selection.Width <= 0 || selection.Height <= 0 || frameSize.Width <= 0 || frameSize.Height <= 0 {
		return ScreenshotResult{}, errors.New("screenshot selection is empty")
	}
	scaleX := float32(source.Bounds().Dx()) / frameSize.Width
	scaleY := float32(source.Bounds().Dy()) / frameSize.Height
	pixelSelection := image.Rect(
		max(0, int(math.Floor(float64(selection.X*scaleX)))),
		max(0, int(math.Floor(float64(selection.Y*scaleY)))),
		min(source.Bounds().Dx(), int(math.Ceil(float64((selection.X+selection.Width)*scaleX)))),
		min(source.Bounds().Dy(), int(math.Ceil(float64((selection.Y+selection.Height)*scaleY)))),
	)
	if pixelSelection.Empty() {
		return ScreenshotResult{}, errors.New("screenshot pixel selection is empty")
	}
	if err := writeScreenshotPNG(options.ExportFilePath, source.SubImage(pixelSelection)); err != nil {
		return ScreenshotResult{}, err
	}
	result := ScreenshotResult{
		ScreenshotPath:          options.ExportFilePath,
		ClipboardWriteSucceeded: !options.CopyToClipboard,
		LogicalSelection: Rect{
			X:      float32(virtualBounds.Min.X)/scaleX + selection.X,
			Y:      float32(virtualBounds.Min.Y)/scaleY + selection.Y,
			Width:  selection.Width,
			Height: selection.Height,
		},
	}
	if options.CopyToClipboard {
		if err := overlay.WriteClipboardImageFile(options.ExportFilePath); err != nil {
			result.ClipboardWarningMessage = err.Error()
		} else {
			result.ClipboardWriteSucceeded = true
		}
	}
	return result, nil
}

func captureWindowsVirtualDesktop() (*image.RGBA, image.Rectangle, error) {
	x := win.GetSystemMetrics(win.SM_XVIRTUALSCREEN)
	y := win.GetSystemMetrics(win.SM_YVIRTUALSCREEN)
	width := win.GetSystemMetrics(win.SM_CXVIRTUALSCREEN)
	height := win.GetSystemMetrics(win.SM_CYVIRTUALSCREEN)
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return nil, image.Rectangle{}, fmt.Errorf("invalid virtual desktop size: %dx%d", width, height)
	}
	screenDC := win.GetDC(0)
	if screenDC == 0 {
		return nil, image.Rectangle{}, errors.New("failed to open the Windows desktop device context")
	}
	defer win.ReleaseDC(0, screenDC)
	memoryDC := win.CreateCompatibleDC(screenDC)
	if memoryDC == 0 {
		return nil, image.Rectangle{}, errors.New("failed to create the screenshot memory device context")
	}
	defer win.DeleteDC(memoryDC)
	bitmap := win.CreateCompatibleBitmap(screenDC, width, height)
	if bitmap == 0 {
		return nil, image.Rectangle{}, errors.New("failed to create the screenshot bitmap")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))
	previous := win.SelectObject(memoryDC, win.HGDIOBJ(bitmap))
	if previous == 0 {
		return nil, image.Rectangle{}, errors.New("failed to select the screenshot bitmap")
	}
	if !win.BitBlt(memoryDC, 0, 0, width, height, screenDC, x, y, win.SRCCOPY|captureBlt) {
		win.SelectObject(memoryDC, previous)
		return nil, image.Rectangle{}, errors.New("failed to copy Windows desktop pixels")
	}
	win.SelectObject(memoryDC, previous)

	bitmapInfo := win.BITMAPINFO{BmiHeader: win.BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
		BiWidth:       width,
		BiHeight:      -height,
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: win.BI_RGB,
	}}
	bgra := make([]byte, int(width)*int(height)*4)
	if win.GetDIBits(memoryDC, bitmap, 0, uint32(height), &bgra[0], &bitmapInfo, win.DIB_RGB_COLORS) == 0 {
		return nil, image.Rectangle{}, errors.New("failed to read Windows screenshot pixels")
	}
	rgba := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for offset := 0; offset < len(bgra); offset += 4 {
		rgba.Pix[offset] = bgra[offset+2]
		rgba.Pix[offset+1] = bgra[offset+1]
		rgba.Pix[offset+2] = bgra[offset]
		rgba.Pix[offset+3] = 255
	}
	return rgba, image.Rect(int(x), int(y), int(x+width), int(y+height)), nil
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

func (state *screenshotOverlayState) draw(displayList *DisplayList, frame FrameInfo) {
	state.mu.Lock()
	state.frameSize = frame.Size
	selection := normalizedScreenshotRect(state.selection, frame.Size)
	hasSelection := state.hasSelection || state.dragging
	dragging := state.dragging
	state.confirmRect = Rect{}
	state.cancelRect = Rect{}
	state.mu.Unlock()

	displayList.Clear(Color{A: 255})
	displayList.DrawImage(state.image, Rect{Width: frame.Size.Width, Height: frame.Size.Height})
	dim := Color{A: 108}
	if !hasSelection || selection.Width <= 0 || selection.Height <= 0 {
		displayList.FillRect(Rect{Width: frame.Size.Width, Height: frame.Size.Height}, dim)
		instructionWidth := min(float32(390), max(float32(160), frame.Size.Width-32))
		instructionLeft := max(float32(16), (frame.Size.Width-instructionWidth)/2)
		displayList.FillRoundedRect(Rect{X: instructionLeft, Y: 24, Width: instructionWidth, Height: 38}, 8, Color{R: 26, G: 28, B: 32, A: 230})
		displayList.DrawText("Drag to select a region  ·  Esc to cancel", Rect{X: instructionLeft + 16, Y: 34, Width: instructionWidth - 32, Height: 18}, TextStyle{Size: 12, Weight: FontWeightSemibold}, Color{R: 245, G: 247, B: 250, A: 255})
		return
	}
	displayList.FillRect(Rect{Width: frame.Size.Width, Height: selection.Y}, dim)
	displayList.FillRect(Rect{Y: selection.Y + selection.Height, Width: frame.Size.Width, Height: max(float32(0), frame.Size.Height-selection.Y-selection.Height)}, dim)
	displayList.FillRect(Rect{Y: selection.Y, Width: selection.X, Height: selection.Height}, dim)
	displayList.FillRect(Rect{X: selection.X + selection.Width, Y: selection.Y, Width: max(float32(0), frame.Size.Width-selection.X-selection.Width), Height: selection.Height}, dim)
	displayList.StrokeRoundedRect(selection, 0, 2, Color{R: 65, G: 148, B: 255, A: 255})
	if dragging || state.autoConfirm {
		return
	}
	toolbarWidth := min(float32(238), max(float32(180), frame.Size.Width-16))
	toolbarHeight := float32(38)
	toolbarLeft := min(max(float32(8), selection.X+selection.Width-toolbarWidth), max(float32(8), frame.Size.Width-toolbarWidth-8))
	toolbarTop := selection.Y + selection.Height + 8
	if toolbarTop+toolbarHeight > frame.Size.Height-8 {
		toolbarTop = max(float32(8), selection.Y-toolbarHeight-8)
	}
	cancelRect := Rect{X: toolbarLeft, Y: toolbarTop, Width: toolbarWidth * 0.43, Height: toolbarHeight}
	confirmRect := Rect{X: cancelRect.X + cancelRect.Width, Y: toolbarTop, Width: toolbarWidth - cancelRect.Width, Height: toolbarHeight}
	state.mu.Lock()
	state.cancelRect = cancelRect
	state.confirmRect = confirmRect
	state.mu.Unlock()
	displayList.FillRoundedRect(Rect{X: toolbarLeft, Y: toolbarTop, Width: toolbarWidth, Height: toolbarHeight}, 8, Color{R: 28, G: 30, B: 35, A: 244})
	displayList.FillRoundedRect(Rect{X: confirmRect.X + 3, Y: confirmRect.Y + 3, Width: confirmRect.Width - 6, Height: confirmRect.Height - 6}, 6, Color{R: 54, G: 126, B: 232, A: 255})
	displayList.DrawText("Esc  Cancel", Rect{X: cancelRect.X + 13, Y: cancelRect.Y + 11, Width: cancelRect.Width - 20, Height: 17}, TextStyle{Size: 11}, Color{R: 218, G: 222, B: 230, A: 255})
	displayList.DrawText("Enter  Copy", Rect{X: confirmRect.X + 15, Y: confirmRect.Y + 11, Width: confirmRect.Width - 22, Height: 17}, TextStyle{Size: 11, Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: 255})
}

func (state *screenshotOverlayState) pointer(event PointerEvent) {
	if event.Button != PointerButtonPrimary && event.Kind != PointerMove {
		return
	}
	switch event.Kind {
	case PointerDown:
		state.mu.Lock()
		confirm := state.hasSelection && screenshotRectContains(state.confirmRect, event.Position)
		cancel := state.hasSelection && screenshotRectContains(state.cancelRect, event.Position)
		if !confirm && !cancel {
			state.start = event.Position
			state.selection = Rect{X: event.Position.X, Y: event.Position.Y}
			state.dragging = true
			state.hasSelection = false
		}
		state.mu.Unlock()
		if confirm {
			state.complete(false)
		} else if cancel {
			state.complete(true)
		} else {
			_ = state.window.Invalidate()
		}
	case PointerMove:
		state.mu.Lock()
		if !state.dragging {
			state.mu.Unlock()
			return
		}
		state.selection = Rect{X: state.start.X, Y: state.start.Y, Width: event.Position.X - state.start.X, Height: event.Position.Y - state.start.Y}
		state.mu.Unlock()
		_ = state.window.Invalidate()
	case PointerUp:
		state.mu.Lock()
		if !state.dragging {
			state.mu.Unlock()
			return
		}
		state.selection = normalizedScreenshotRect(Rect{X: state.start.X, Y: state.start.Y, Width: event.Position.X - state.start.X, Height: event.Position.Y - state.start.Y}, state.frameSize)
		state.dragging = false
		state.hasSelection = state.selection.Width >= 2 && state.selection.Height >= 2
		autoConfirm := state.autoConfirm && state.hasSelection
		state.mu.Unlock()
		_ = state.window.Invalidate()
		if autoConfirm {
			state.complete(false)
		}
	}
}

func (state *screenshotOverlayState) key(event KeyEvent) bool {
	if !event.Down {
		return false
	}
	if event.Key == KeyEscape {
		state.complete(true)
		return true
	}
	if event.Key == KeyEnter {
		state.mu.Lock()
		hasSelection := state.hasSelection
		state.mu.Unlock()
		if hasSelection {
			state.complete(false)
		}
		return true
	}
	return false
}

func (state *screenshotOverlayState) complete(cancelled bool) {
	state.once.Do(func() {
		state.result <- screenshotOverlayOutcome{cancelled: cancelled}
	})
}

func normalizedScreenshotRect(rect Rect, frame Size) Rect {
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

func screenshotRectContains(rect Rect, point Point) bool {
	return rect.Width > 0 && rect.Height > 0 && point.X >= rect.X && point.X < rect.X+rect.Width && point.Y >= rect.Y && point.Y < rect.Y+rect.Height
}
