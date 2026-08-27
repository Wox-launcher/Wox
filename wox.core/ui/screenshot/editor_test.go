package screenshot

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// screenshotTestSurface supplies nonzero font metrics without opening a native window.
type screenshotTestSurface struct{ *Window }

func (*screenshotTestSurface) MeasureText(text string, style TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: Size{Width: float32(len([]rune(text))) * style.Size / 2, Height: style.Size + 6}}, nil
}

// TestScreenshotSizeLabelCentersOpaqueChrome covers the stateless widget path at fractional DPI scales.
func TestScreenshotSizeLabelCentersOpaqueChrome(t *testing.T) {
	surface := &screenshotTestSurface{}
	for _, scale := range []float32{1, 1.25, 1.5, 2.5} {
		label := "1814 x 1280"
		selection, frame := Rect{X: 200, Y: 300, Width: 400, Height: 200}, Size{Width: 1600, Height: 1000}
		chip := screenshotEditorSizeLabelRect(label, selection, Rect{}, frame, scale)
		actual := &DisplayList{}
		drawScreenshotEditorSizeLabel(actual, surface, label, selection, Rect{}, frame, scale)
		style := TextStyle{Size: 14 * scale, Weight: FontWeightSemibold}
		metrics, _ := surface.MeasureText(label, style)
		expected := &DisplayList{}
		expected.FillRoundedRect(chip, 10*scale, Color{R: 23, G: 23, B: 23, A: 255})
		expected.DrawText(label, Rect{X: chip.X + (chip.Width-metrics.Size.Width)/2, Y: chip.Y + (chip.Height-metrics.Size.Height)/2, Width: metrics.Size.Width, Height: metrics.Size.Height}, style, Color{R: 255, G: 255, B: 255, A: 255})
		if err := actual.Compare(expected); err != nil {
			t.Fatalf("scale %v: size label is not centered and opaque: %v", scale, err)
		}
	}
}

// TestScreenshotSelectionPixelSize covers fractional capture scales independently from desktop origins and chrome DPI.
func TestScreenshotSelectionPixelSize(t *testing.T) {
	for _, frame := range []Size{{Width: 1920, Height: 1080}, {Width: 1536, Height: 864}, {Width: 1280, Height: 720}, {Width: 960, Height: 540}, {Width: 1417, Height: 833}} {
		for _, origin := range []Point{{X: 10, Y: 20}, {X: 100.3, Y: 50.7}, {X: 0.1, Y: 0.2}} {
			original := Rect{X: origin.X, Y: origin.Y, Width: 200, Height: 100}
			bounds := image.Rect(-1920, -1080, 0, 0)
			before, err := screenshotEditorPixelSelection(bounds, original, frame)
			if err != nil {
				t.Fatal(err)
			}
			for _, size := range []image.Point{image.Pt(1, 1), image.Pt(101, 73), image.Pt(800, 400), bounds.Max.Sub(before.Min)} {
				selection := screenshotEditorSelectionWithPixelSize(original, frame, bounds.Size(), size.X, size.Y)
				got, err := screenshotEditorPixelSelection(bounds, selection, frame)
				if err != nil || got.Size() != size || got.Min != before.Min || selection.X != original.X || selection.Y != original.Y {
					t.Fatalf("frame=%+v origin=%+v size=%v: selection=%+v crop=%v err=%v", frame, origin, size, selection, got, err)
				}
			}
		}
	}
}

// TestScreenshotSizeDialog exercises the rendered chip, draft validation, modal input, and the applied crop.
func TestScreenshotSizeDialog(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 1200, 800), frameSize: Size{Width: 1200, Height: 800},
		selection: Rect{X: 200, Y: 200, Width: 300, Height: 200}, hasSelection: true,
		chromeScale: func(Rect) float32 { return 1.5 }, desktopPixelOrigin: Point{X: -1200, Y: -800},
		annotations: []screenshotEditorAnnotation{{tool: screenshotEditorToolRect, rect: Rect{X: 220, Y: 220, Width: 20, Height: 20}}},
		result:      make(chan screenshotEditorOverlayOutcome, 1),
	}
	frame := FrameInfo{Size: state.frameSize}
	state.annotationDragging = true
	if state.openSizeDialog() {
		t.Fatal("size editing must not interrupt an annotation drag")
	}
	state.annotationDragging = false
	state.editMode = screenshotEditorEditMoveSelection
	if state.openSizeDialog() {
		t.Fatal("size editing must not interrupt a selection drag")
	}
	state.editMode = screenshotEditorEditNone
	state.draw(&DisplayList{}, frame)
	chip := state.sizeLabelRect
	point := Point{X: chip.X + chip.Width/2, Y: chip.Y + chip.Height/2}
	state.pointer(PointerEvent{Kind: PointerMove, Position: point})
	if state.pointerCursor != PointerCursorHand {
		t.Fatal("size chip must show an actionable cursor")
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
	dialog := state.activeSizeDialog()
	if dialog == nil || dialog.width.Text() != "300" || dialog.height.Text() != "200" || state.dragging {
		t.Fatalf("click did not open the size draft: %+v", dialog)
	}
	original := state.selection
	for _, values := range [][2]string{{"", "200"}, {"0", "200"}, {"-1", "200"}, {"1.5", "200"}, {"abc", "200"}, {"99999999999999999999999", "200"}, {"1001", "200"}, {"300", "601"}} {
		dialog.width.SetText(values[0], false)
		dialog.height.SetText(values[1], false)
		dialog.apply()
		if !dialog.invalid || state.selection != original || state.activeSizeDialog() != dialog {
			t.Fatalf("invalid draft %v changed capture or closed dialog", values)
		}
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 10, Y: 10}})
	state.key(KeyEvent{Key: Key("r"), Down: true})
	state.textInput(TextInputEvent{Kind: TextInputCommit, Text: "annotation"})
	if state.selection != original || state.dragging || state.activeTool != screenshotEditorToolSelect || len(state.annotations) != 1 {
		t.Fatal("dialog input leaked to the capture editor")
	}
	state.key(KeyEvent{Key: KeyEscape, Down: true})
	if state.activeSizeDialog() != nil || state.selection != original || len(state.result) != 0 {
		t.Fatal("Escape should discard only the size draft")
	}
	state.key(KeyEvent{Key: Key("s"), Down: true})
	dialog = state.activeSizeDialog()
	if dialog == nil {
		t.Fatal("S must reopen the size editor")
	}
	dialog.width.SetText(" 501 ", false)
	dialog.height.SetText("303", false)
	dialog.apply()
	if state.activeSizeDialog() != nil || state.selection != (Rect{X: 200, Y: 200, Width: 501, Height: 303}) || len(state.annotations) != 1 {
		t.Fatalf("apply changed the anchor or annotations: %+v", state.selection)
	}
	state.chromeScale = func(Rect) float32 { return 2.5 }
	state.draw(&DisplayList{}, frame)
	if state.sizeLabelRect.Height <= chip.Height {
		t.Fatal("size chip did not follow display DPI transition")
	}
}

// TestScreenshotSizeDialogKeyboardAndLayout uses the retained controls without opening a native window.
func TestScreenshotSizeDialogKeyboardAndLayout(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 1200, 800), frameSize: Size{Width: 600, Height: 400},
		selection: Rect{X: 20, Y: 30, Width: 200, Height: 100}, hasSelection: true,
		sizeDialogOptions: ScreenshotOptions{SizeLabels: ScreenshotSizeLabels{Title: "修改选区大小", Width: "宽度（像素）", Height: "高度（像素）", Apply: "应用", Cancel: "取消", InvalidSize: "请输入整数：宽度 1–%d，高度 1–%d。", LockAspectRatio: "固定宽高比", Swap: "互换宽度和高度"}},
	}
	state.openSizeDialog()
	dialog := state.activeSizeDialog()
	dialog.host = woxwidget.NewHost(dialog.build)
	dialog.host.AttachServices(&screenshotTestSurface{})
	defer dialog.host.Dispose()
	frame := FrameInfo{Size: dialog.size(), Scale: 1.5}
	draw := func() {
		dialog.host.Frame(&DisplayList{}, frame)
	}
	draw()
	draw()
	fieldCount := 0
	var heightBounds Rect
	for _, node := range dialog.host.Snapshot().Tree.Nodes {
		if node.AutomationID == "screenshot.size.width" && !node.Focused {
			t.Fatal("width should receive initial focus")
		}
		if node.Role == woxui.AccessibilityRoleTextField || node.Role == woxui.AccessibilityRoleButton {
			if node.Bounds.Height != 32 || node.Bounds.Y+node.Bounds.Height > frame.Size.Height {
				t.Fatalf("control outside standard size or dialog: %+v", node)
			}
		}
		if node.Role == woxui.AccessibilityRoleTextField {
			fieldCount++
			if node.Bounds.Width < 100 {
				t.Fatalf("field has no usable input width: %+v", node)
			}
			if node.AutomationID == "screenshot.size.height" {
				heightBounds = node.Bounds
			}
		}
	}
	if fieldCount != 2 {
		t.Fatalf("editable fields = %d, want 2", fieldCount)
	}
	if runtime.GOOS == "windows" {
		displayList := &DisplayList{}
		dialog.draw(displayList, frame)
		renderer, _ := woxui.NewSoftwareRenderer(int(frame.Size.Width), int(frame.Size.Height))
		if err := renderer.Render(displayList); err != nil {
			t.Fatal(err)
		}
		if renderer.RGBA().RGBAAt(0, 0).A != 255 {
			t.Fatal("Windows client corners must be opaque so only DWM draws the outer rounding")
		}
	}
	dialog.host.TextInput(TextInputEvent{Kind: TextInputCommit, Text: "301"})
	point := Point{X: heightBounds.X + heightBounds.Width/2, Y: heightBounds.Y + heightBounds.Height/2}
	dialog.host.Pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
	dialog.host.Pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: point})
	primaryModifier := KeyModifierControl
	if !primaryModifier.HasPrimary() {
		primaryModifier = KeyModifierMeta
	}
	dialog.key(KeyEvent{Key: Key("a"), Modifiers: primaryModifier, Down: true})
	dialog.host.TextInput(TextInputEvent{Kind: TextInputCommit, Text: "151"})
	dialog.key(KeyEvent{Key: KeyEnter, Down: true})
	pixels, err := screenshotEditorPixelSelection(image.Rect(0, 0, 1200, 800), state.selection, state.frameSize)
	if err != nil || state.activeSizeDialog() != nil || pixels.Size() != image.Pt(301, 151) {
		t.Fatalf("keyboard apply: crop=%v err=%v open=%v", pixels, err, state.activeSizeDialog() != nil)
	}
	state.openSizeDialog()
	next := state.activeSizeDialog()
	dialog.closed()
	if state.activeSizeDialog() != next {
		t.Fatal("a late close callback discarded the newly opened dialog")
	}
	next.closed()
	if state.activeSizeDialog() != nil {
		t.Fatal("external dialog close left the screenshot modal")
	}
}

// TestScreenshotSizeDialogRatioAndSwap routes pointer and text events through the actual form controls.
func TestScreenshotSizeDialogRatioAndSwap(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 1200, 800), frameSize: Size{Width: 1200, Height: 800},
		selection: Rect{X: 200, Y: 200, Width: 300, Height: 200}, hasSelection: true,
		sizeDialogOptions: ScreenshotOptions{SizeLabels: ScreenshotSizeLabels{Title: "Edit size", Width: "Width", Height: "Height", Apply: "Apply", Cancel: "Cancel", LockAspectRatio: "Lock aspect ratio", Swap: "Swap", InvalidSize: "Width 1–%d, height 1–%d"}},
	}
	state.openSizeDialog()
	dialog := state.activeSizeDialog()
	dialog.host = woxwidget.NewHost(dialog.build)
	dialog.host.AttachServices(&screenshotTestSurface{})
	defer dialog.host.Dispose()
	frame := FrameInfo{}
	draw := func() {
		t.Helper()
		frame.Size = dialog.size()
		dialog.draw(&DisplayList{}, frame)
		var lock, apply, errorBounds Rect
		for _, node := range dialog.host.Snapshot().Tree.Nodes {
			switch node.AutomationID {
			case "screenshot.size.lock-row":
				lock = node.Bounds
			case "screenshot.size.apply":
				apply = node.Bounds
			case "screenshot.size.error":
				errorBounds = node.Bounds
			}
		}
		if bottom := frame.Size.Height - apply.Y - apply.Height; bottom != 20 {
			t.Fatalf("footer bottom padding = %v, want 20", bottom)
		}
		previous := lock
		if dialog.invalid {
			if errorBounds.Height != 36 || errorBounds.Y-lock.Y-lock.Height != 12 {
				t.Fatalf("validation must fit between checkbox and actions: lock=%+v error=%+v", lock, errorBounds)
			}
			previous = errorBounds
		} else if errorBounds != (Rect{}) {
			t.Fatal("valid draft must not reserve validation space")
		}
		if gap := apply.Y - previous.Y - previous.Height; gap != 12 {
			t.Fatalf("footer gap = %v, want 12", gap)
		}
	}
	draw()
	draw()
	click := func(id string) {
		t.Helper()
		for _, node := range dialog.host.Snapshot().Tree.Nodes {
			if node.AutomationID == "screenshot.size."+id {
				if node.Bounds.X < 0 || node.Bounds.Y < 0 || node.Bounds.X+node.Bounds.Width > frame.Size.Width || node.Bounds.Y+node.Bounds.Height > frame.Size.Height {
					t.Fatalf("control %s exceeds dialog: %+v", id, node.Bounds)
				}
				point := Point{X: node.Bounds.X + node.Bounds.Width/2, Y: node.Bounds.Y + node.Bounds.Height/2}
				dialog.host.Pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
				dialog.host.Pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: point})
				draw()
				return
			}
		}
		t.Fatalf("missing control %s", id)
	}
	typeValue := func(id, value string) {
		t.Helper()
		click(id)
		primary := KeyModifierControl
		if !primary.HasPrimary() {
			primary = KeyModifierMeta
		}
		dialog.key(KeyEvent{Key: Key("a"), Down: true, Modifiers: primary})
		dialog.host.TextInput(TextInputEvent{Kind: TextInputCommit, Text: value})
		draw()
	}
	assertSize := func(width, height string) {
		t.Helper()
		if dialog.width.Text() != width || dialog.height.Text() != height {
			t.Fatalf("draft = %s x %s, want %s x %s", dialog.width.Text(), dialog.height.Text(), width, height)
		}
	}
	typeValue("width", "450")
	typeValue("height", "100")
	click("lock-row")
	assertSize("450", "300")
	if !dialog.lockRatio || dialog.aspectRatio != 1.5 {
		t.Fatal("lock must use the selected region's 3:2 ratio")
	}
	typeValue("width", "501")
	assertSize("501", "334")
	typeValue("height", "333")
	assertSize("500", "333")
	click("swap")
	assertSize("333", "500")
	typeValue("width", "200")
	assertSize("200", "300")
	click("lock")
	typeValue("width", "250")
	assertSize("250", "300")
	click("lock")
	assertSize("250", "167")
	if dialog.aspectRatio != 1.5 {
		t.Fatal("relocking must return to the actual selection ratio")
	}
	original := state.selection
	typeValue("width", "1001")
	click("apply")
	if !dialog.invalid || state.activeSizeDialog() != dialog || state.selection != original {
		t.Fatalf("out-of-bounds linked dimensions: invalid=%v open=%v selection=%+v draft=%s x %s", dialog.invalid, state.activeSizeDialog() == dialog, state.selection, dialog.width.Text(), dialog.height.Text())
	}
	typeValue("height", "0")
	click("apply")
	if !dialog.invalid {
		t.Fatal("zero dimensions must remain invalid while ratio is locked")
	}
	click("cancel")
	if state.activeSizeDialog() != nil || state.selection != original {
		t.Fatal("cancel changed the capture selection")
	}
}

func TestWriteScreenshotImageUsesJPEGForJPG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.jpg")
	source := image.NewRGBA(image.Rect(0, 0, 32, 24))
	draw.Draw(source, source.Bounds(), &image.Uniform{C: color.RGBA{R: 40, G: 80, B: 120, A: 255}}, image.Point{}, draw.Src)
	if err := writeScreenshotImage(path, source); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, err := jpeg.Decode(file)
	if err != nil {
		t.Fatalf("decode screenshot JPEG: %v", err)
	}
	if decoded.Bounds().Size() != source.Bounds().Size() {
		t.Fatalf("JPEG size = %v, want %v", decoded.Bounds().Size(), source.Bounds().Size())
	}
}

func TestNewScreenshotEditorImageSharesPackedRGBA(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 2))
	source.SetRGBA(1, 1, color.RGBA{R: 20, G: 40, B: 60, A: 255})

	prepared, err := newScreenshotEditorImage(source)
	if err != nil {
		t.Fatalf("prepare screenshot image: %v", err)
	}
	source.SetRGBA(1, 1, color.RGBA{R: 80, G: 100, B: 120, A: 255})
	if pixel := prepared.RGBAAt(1, 1); pixel.R != 80 || pixel.G != 100 || pixel.B != 120 {
		t.Fatalf("packed screenshot pixels were copied: got %+v", pixel)
	}
	if prepared.Width != 4 || prepared.Height != 2 {
		t.Fatalf("prepared size = %dx%d, want 4x2", prepared.Width, prepared.Height)
	}
}

func TestScreenshotEditorWindowHostSerializesCallbacks(t *testing.T) {
	host := &screenshotEditorWindowHost{}
	first := &screenshotEditorOverlayState{}
	second := &screenshotEditorOverlayState{}
	if err := host.begin(first); err != nil {
		t.Fatalf("begin first screenshot session: %v", err)
	}
	if err := host.begin(second); err == nil {
		t.Fatal("second screenshot session should be rejected while the first is active")
	}
	host.end(second)
	if host.current() != first {
		t.Fatal("ending a different session cleared the active screenshot")
	}
	host.end(first)
	if host.current() != nil {
		t.Fatal("active screenshot session was not cleared")
	}
}

func TestScreenshotHostKeepsScreenshotChromeUntilRecordingToolbarFits(t *testing.T) {
	host := &screenshotEditorWindowHost{}
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 400, 300),
		selection:    Rect{X: 40, Y: 40, Width: 200, Height: 120},
		hasSelection: true,
		frameSize:    Size{Width: 400, Height: 300},
		result:       make(chan screenshotEditorOverlayOutcome, 1),
	}
	if err := host.begin(state); err != nil {
		t.Fatal(err)
	}
	defer host.end(state)
	state.recordingUI = &recordingToolbarState{editor: state}

	fullscreen := &DisplayList{}
	host.draw(fullscreen, FrameInfo{Size: Size{Width: 400, Height: 300}})
	screenshotOnly := &DisplayList{}
	state.recordingUI = nil
	host.draw(screenshotOnly, FrameInfo{Size: Size{Width: 400, Height: 300}})
	if err := fullscreen.Compare(screenshotOnly); err != nil {
		t.Fatalf("fullscreen recording transition should keep screenshot chrome: %v", err)
	}

	state.recordingUI = &recordingToolbarState{editor: state}
	toolbar := &DisplayList{}
	host.draw(toolbar, FrameInfo{Size: Size{Width: recordingToolbarWidth, Height: recordingToolbarHeight}})
	if state.frameSize != (Size{Width: 400, Height: 300}) {
		t.Fatalf("recording toolbar frame replaced desktop frame size: %+v", state.frameSize)
	}
	if err := toolbar.Compare(screenshotOnly); err == nil {
		t.Fatal("toolbar-sized frame should draw recording chrome instead of screenshot overlay")
	}
}

func TestNewScreenshotEditorImageCopiesStridedRGBA(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 2))
	subImage := source.SubImage(image.Rect(1, 0, 3, 2)).(*image.RGBA)
	subImage.SetRGBA(1, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})

	prepared, err := newScreenshotEditorImage(subImage)
	if err != nil {
		t.Fatalf("prepare screenshot subimage: %v", err)
	}
	subImage.SetRGBA(1, 0, color.RGBA{R: 90, G: 100, B: 110, A: 255})
	if pixel := prepared.RGBAAt(0, 0); pixel.R != 10 || pixel.G != 20 || pixel.B != 30 {
		t.Fatalf("strided screenshot pixels did not use the normalized copy path: got %+v", pixel)
	}
	if prepared.Width != 2 || prepared.Height != 2 {
		t.Fatalf("prepared size = %dx%d, want 2x2", prepared.Width, prepared.Height)
	}
}

func TestScreenshotEditorSelectionMapsLogicalPointsToPixels(t *testing.T) {
	state := &screenshotEditorOverlayState{frameSize: Size{Width: 1000, Height: 500}}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 400, Y: 300}})
	if !state.colorInspectorDismissed {
		t.Fatal("starting a screenshot selection should permanently dismiss the color inspector")
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 100, Y: 50}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 100, Y: 50}})

	wantSelection := Rect{X: 100, Y: 50, Width: 300, Height: 250}
	if state.selection != wantSelection {
		t.Fatalf("selection = %+v, want %+v", state.selection, wantSelection)
	}
	pixels, err := screenshotEditorPixelSelection(image.Rect(0, 0, 2000, 1000), state.selection, state.frameSize)
	if err != nil {
		t.Fatalf("map selection: %v", err)
	}
	wantPixels := image.Rect(200, 100, 800, 600)
	if pixels != wantPixels {
		t.Fatalf("pixels = %v, want %v", pixels, wantPixels)
	}
}

func TestScreenshotEditorColorInspectorMapsPointerToCapturedPixel(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 2))
	source.SetRGBA(1, 1, color.RGBA{R: 203, G: 174, B: 140, A: 255})
	prepared, err := NewImage(source)
	if err != nil {
		t.Fatalf("prepare color inspector image: %v", err)
	}

	x, y, pixel, ok := screenshotEditorPixelAtPoint(prepared, Size{Width: 100, Height: 100}, Point{X: 25, Y: 50})
	if !ok || x != 1 || y != 1 {
		t.Fatalf("mapped pixel = (%d, %d, %t), want (1, 1, true)", x, y, ok)
	}
	if pixel != (color.RGBA{R: 203, G: 174, B: 140, A: 255}) {
		t.Fatalf("sampled color = %+v", pixel)
	}
	if _, _, _, ok := screenshotEditorPixelAtPoint(prepared, Size{Width: 100, Height: 100}, Point{X: 100, Y: 50}); ok {
		t.Fatal("point outside the frame should not resolve a pixel")
	}
}

func TestScreenshotEditorColorInspectorLayoutAvoidsFrameEdges(t *testing.T) {
	panel := Size{Width: 150, Height: 138}
	if got := screenshotEditorInspectorRect(Size{Width: 800, Height: 600}, Point{X: 100, Y: 100}, panel, 1); got != (Rect{X: 120, Y: 120, Width: 150, Height: 138}) {
		t.Fatalf("lower-right inspector = %+v", got)
	}
	if got := screenshotEditorInspectorRect(Size{Width: 800, Height: 600}, Point{X: 790, Y: 590}, panel, 1); got != (Rect{X: 620, Y: 432, Width: 150, Height: 138}) {
		t.Fatalf("flipped inspector = %+v", got)
	}
}

func TestScreenshotEditorDesktopPixelOriginUsesCaptureScale(t *testing.T) {
	origin := screenshotEditorDesktopPixelOrigin(Rect{X: -100, Y: 50, Width: 1000, Height: 500}, image.NewRGBA(image.Rect(0, 0, 2000, 1000)))
	if origin != (Point{X: -200, Y: 100}) {
		t.Fatalf("desktop pixel origin = %+v", origin)
	}
}

func TestScreenshotEditorPointerMovementTracksColorInspector(t *testing.T) {
	state := &screenshotEditorOverlayState{}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 45, Y: 67}})
	if !state.pointerInside || state.pointerPosition != (Point{X: 45, Y: 67}) {
		t.Fatalf("tracked pointer = %+v inside=%t", state.pointerPosition, state.pointerInside)
	}
	state.pointer(PointerEvent{Kind: PointerLeave})
	if state.pointerInside {
		t.Fatal("pointer leave should hide the color inspector")
	}
}

func TestScreenshotEditorColorInspectorUsesPointerDisplayScaleBeforeSelection(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:           testScreenshotImage(t, 10, 10),
		pointerInside:   true,
		pointerPosition: Point{X: 500, Y: 300},
		chromeScale: func(rect Rect) float32 {
			if rect != (Rect{X: 500, Y: 300, Width: 1, Height: 1}) {
				t.Fatalf("scale rect = %+v, want pointer display probe", rect)
			}
			return 2
		},
	}
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 1000, Height: 600}})
	if state.uiScale != 2 {
		t.Fatalf("pre-selection UI scale = %v, want 2", state.uiScale)
	}
}

func TestScreenshotEditorColorShortcutsCopyIndependentFormats(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	source.SetRGBA(1, 1, color.RGBA{R: 203, G: 174, B: 140, A: 255})
	prepared, err := NewImage(source)
	if err != nil {
		t.Fatalf("prepare shortcut image: %v", err)
	}
	for _, testCase := range []struct {
		key  Key
		want string
	}{
		{key: Key("g"), want: "rgb(203, 174, 140)"},
		{key: Key("h"), want: "#CBAE8C"},
	} {
		var copied string
		state := &screenshotEditorOverlayState{
			image: prepared, frameSize: Size{Width: 2, Height: 2}, pointerInside: true, pointerPosition: Point{X: 1, Y: 1},
			result: make(chan screenshotEditorOverlayOutcome, 1),
			writeClipboardText: func(text string) error {
				copied = text
				return nil
			},
		}
		if !state.key(KeyEvent{Key: testCase.key, Down: true}) {
			t.Fatalf("color copy shortcut %q was not handled", testCase.key)
		}
		outcome := <-state.result
		if copied != testCase.want || outcome.copiedColor != testCase.want {
			t.Fatalf("shortcut %q copied=%q outcome=%q, want %q", testCase.key, copied, outcome.copiedColor, testCase.want)
		}
	}
}

func TestScreenshotEditorSelectionDisablesColorInteraction(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 2, 2), frameSize: Size{Width: 2, Height: 2},
		pointerInside: true, pointerPosition: Point{X: 1, Y: 1}, colorInspectorDismissed: true,
		writeClipboardText: func(string) error {
			t.Fatal("dismissed color inspector should not write to the clipboard")
			return nil
		},
	}
	if state.key(KeyEvent{Key: Key("h"), Down: true}) {
		t.Fatal("color shortcut should not be handled after selection starts")
	}
	if state.key(KeyEvent{Key: KeyArrowDown, Down: true}) {
		t.Fatal("color nudge should not be handled after selection starts")
	}
}

func TestScreenshotEditorArrowKeysNudgeOneCapturedPixel(t *testing.T) {
	prepared := testScreenshotImage(t, 4, 4)
	var moved Point
	state := &screenshotEditorOverlayState{
		image: prepared, frameSize: Size{Width: 8, Height: 8}, pointerInside: true, pointerPosition: Point{X: 3.25, Y: 3.75},
		setPointerPosition: func(point Point) error {
			moved = point
			return nil
		},
	}
	if !state.key(KeyEvent{Key: KeyArrowDown, Down: true}) {
		t.Fatal("down arrow nudge was not handled")
	}
	if moved != (Point{X: 3.25, Y: 5}) || state.pointerPosition != moved {
		t.Fatalf("down nudge = %+v pointer=%+v, want only Y to move one source pixel", moved, state.pointerPosition)
	}
	if !state.key(KeyEvent{Key: KeyArrowLeft, Down: true}) {
		t.Fatal("left arrow nudge was not handled")
	}
	if moved != (Point{X: 1, Y: 5}) {
		t.Fatalf("left nudge = %+v, want only X to move one source pixel", moved)
	}
}

func TestNewScreenshotEditorOverlayStateAppliesNativeSelection(t *testing.T) {
	initial := Rect{X: -10, Y: 20, Width: 160, Height: 90}
	state := newScreenshotEditorOverlayState(
		ScreenshotOptions{AutoConfirm: true, HideAnnotationToolbar: true},
		&Image{},
		screenshotEditorPlatform{
			frameSize:        Size{Width: 120, Height: 80},
			initialSelection: &initial,
		},
	)

	if state.selection != (Rect{X: 0, Y: 20, Width: 120, Height: 60}) {
		t.Fatalf("selection = %+v, want clamped native selection", state.selection)
	}
	if !state.hasSelection {
		t.Fatal("native selection should be active")
	}
	if !state.colorInspectorDismissed {
		t.Fatal("a native selection should start with the color inspector dismissed")
	}
	if !state.autoConfirm || !state.hideTools {
		t.Fatalf("options were not preserved: autoConfirm=%t hideTools=%t", state.autoConfirm, state.hideTools)
	}
	if state.frameSize != (Size{Width: 120, Height: 80}) {
		t.Fatalf("frame size = %+v", state.frameSize)
	}
}

func TestScreenshotEditorToolbarUsesCompactCreationTools(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 1, 1),
		selection:    Rect{X: 100, Y: 100, Width: 900, Height: 400},
		hasSelection: true,
		result:       make(chan screenshotEditorOverlayOutcome, 1),
	}
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 1200, Height: 700}})

	if state.toolbarRect.Width != 686 || state.toolbarRect.Height != 60 {
		t.Fatalf("toolbar bounds = %+v, want 686x60", state.toolbarRect)
	}
	if state.toolbarRect.X != state.selection.X+state.selection.Width-state.toolbarRect.Width {
		t.Fatalf("toolbar left = %v, want right-aligned to selection", state.toolbarRect.X)
	}
	if state.toolbarRect.Y != state.selection.Y+state.selection.Height+16 {
		t.Fatalf("toolbar top = %v, want 16px below selection", state.toolbarRect.Y)
	}
	if state.pinRect.Width != 40 || state.cancelRect.Width != 40 || state.saveRect.Width != 40 || state.confirmRect.Width != 40 {
		t.Fatalf("action bounds = pin %+v cancel %+v save %+v confirm %+v", state.pinRect, state.cancelRect, state.saveRect, state.confirmRect)
	}
	if state.toolRects[screenshotEditorToolSelect] != (Rect{}) {
		t.Fatalf("select tool should not occupy toolbar space: %+v", state.toolRects[screenshotEditorToolSelect])
	}
	if state.toolRects[screenshotEditorToolRect].Width != 40 || state.toolRects[screenshotEditorToolMosaic].Width != 40 {
		t.Fatalf("creation tool bounds = rect %+v mosaic %+v", state.toolRects[screenshotEditorToolRect], state.toolRects[screenshotEditorToolMosaic])
	}
	for tool := screenshotEditorToolRect + 1; tool < screenshotEditorToolCount; tool++ {
		if gap := state.toolRects[tool].X - state.toolRects[tool-1].X; gap != 48 {
			t.Fatalf("tool %d slot gap = %v, want 48", tool, gap)
		}
	}
	rectTool := state.toolRects[screenshotEditorToolRect]
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: rectTool.X + 20, Y: rectTool.Y + 20}})
	if !state.hasHoveredTool || state.hoveredTool != int(screenshotEditorToolRect) {
		t.Fatalf("hovered tool = active:%t index:%d, want rectangle", state.hasHoveredTool, state.hoveredTool)
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: rectTool.X + 20, Y: rectTool.Y + 20}})
	if state.activeTool != screenshotEditorToolRect {
		t.Fatalf("active tool = %d, want rectangle", state.activeTool)
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: state.scrollRect.X + 20, Y: state.scrollRect.Y + 20}})
	if !state.hasHoveredAction || state.hoveredAction != screenshotEditorActionScrollingCapture {
		t.Fatalf("hovered action = active:%t action:%d, want scrolling capture", state.hasHoveredAction, state.hoveredAction)
	}
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 1200, Height: 700}})
	if state.editBarRect.Y != state.toolbarRect.Y+state.toolbarRect.Height+8 {
		t.Fatalf("secondary toolbar = %+v, main toolbar = %+v", state.editBarRect, state.toolbarRect)
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.pinRect.X + 20, Y: state.pinRect.Y + 20}})
	if outcome := <-state.result; !outcome.pinned || outcome.cancelled {
		t.Fatalf("pin outcome = %+v", outcome)
	}
}

func TestScreenshotEditorToolbarShowsRecordingOnlyWhenAllowed(t *testing.T) {
	state := newScreenshotEditorOverlayState(ScreenshotOptions{AllowVideoRecording: true}, testScreenshotImage(t, 10, 10), screenshotEditorPlatform{})
	state.selection = Rect{X: 100, Y: 100, Width: 900, Height: 400}
	state.hasSelection = true
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 1200, Height: 700}})
	if state.toolbarRect.Width != 740 || state.recordRect.Width != 40 {
		t.Fatalf("recording toolbar=%+v button=%+v", state.toolbarRect, state.recordRect)
	}

	imageOnly := newScreenshotEditorOverlayState(ScreenshotOptions{}, testScreenshotImage(t, 10, 10), screenshotEditorPlatform{})
	imageOnly.selection = state.selection
	imageOnly.hasSelection = true
	imageOnly.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 1200, Height: 700}})
	if imageOnly.toolbarRect.Width != 686 || imageOnly.recordRect != (Rect{}) {
		t.Fatalf("image-only toolbar=%+v button=%+v", imageOnly.toolbarRect, imageOnly.recordRect)
	}
}

func TestScreenshotEditorSaveActionDownloadsToChosenPath(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 1, 1),
		selection:    Rect{X: 100, Y: 100, Width: 900, Height: 400},
		hasSelection: true,
		result:       make(chan screenshotEditorOverlayOutcome, 1),
	}
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 1200, Height: 700}})
	if state.saveRect.Width != 40 {
		t.Fatalf("save button = %+v", state.saveRect)
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: state.saveRect.X + 20, Y: state.saveRect.Y + 20}})
	if !state.hasHoveredAction || state.hoveredAction != screenshotEditorActionSave {
		t.Fatalf("hovered action = active:%t action:%d, want save", state.hasHoveredAction, state.hoveredAction)
	}

	cancelled := 0
	state.chooseSavePath = func() (string, error) {
		cancelled++
		return "", nil
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.saveRect.X + 20, Y: state.saveRect.Y + 20}})
	select {
	case outcome := <-state.result:
		t.Fatalf("cancelled save dialog completed overlay: %+v", outcome)
	default:
	}
	if cancelled != 1 {
		t.Fatalf("save dialog calls = %d", cancelled)
	}

	state.chooseSavePath = func() (string, error) {
		return filepath.Join(t.TempDir(), "shot"), nil
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.saveRect.X + 20, Y: state.saveRect.Y + 20}})
	outcome := <-state.result
	if outcome.cancelled || !strings.HasSuffix(outcome.saveAsPath, "shot.jpg") {
		t.Fatalf("save outcome = %+v", outcome)
	}
}

func TestScreenshotEditorSaveShortcutUsesPrimaryModifier(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 1, 1),
		selection:    Rect{X: 100, Y: 100, Width: 900, Height: 400},
		hasSelection: true,
		result:       make(chan screenshotEditorOverlayOutcome, 1),
		chooseSavePath: func() (string, error) {
			return filepath.Join(t.TempDir(), "shot.png"), nil
		},
	}
	if !state.key(KeyEvent{Key: Key("s"), Down: true, Modifiers: KeyModifierControl | KeyModifierMeta}) {
		t.Fatal("primary save shortcut was not handled")
	}
	outcome := <-state.result
	if !strings.HasSuffix(outcome.saveAsPath, "shot.png") {
		t.Fatalf("save shortcut path = %q", outcome.saveAsPath)
	}
}

func TestScreenshotSaveAsExportPathAddsJPEGWhenMissing(t *testing.T) {
	if got := screenshotSaveAsExportPath("shot"); got != "shot.jpg" {
		t.Fatalf("path = %q", got)
	}
	if got := screenshotSaveAsExportPath("shot.png"); got != "shot.png" {
		t.Fatalf("png path = %q", got)
	}
	if screenshotSaveAsExportPath("  ") != "" {
		t.Fatal("blank path should stay empty")
	}
}

func TestScreenshotEditorToolbarPlacementKeepsSixteenPixelGap(t *testing.T) {
	frame := Rect{Width: 1200, Height: 700}
	belowSelection := Rect{X: 100, Y: 100, Width: 900, Height: 400}
	below := screenshotEditorToolbarPlacement(belowSelection, frame, 686, 60, 124, 1)
	if below.Y != belowSelection.Y+belowSelection.Height+16 {
		t.Fatalf("below toolbar = %+v, want 16px under selection", below)
	}
	if got := screenshotEditorEditBarTop(below, belowSelection, 56, 8, 24); got != below.Y+below.Height+8 {
		t.Fatalf("below property bar Y = %v, want under toolbar", got)
	}

	aboveSelection := Rect{X: 100, Y: 300, Width: 900, Height: 290}
	above := screenshotEditorToolbarPlacement(aboveSelection, frame, 686, 60, 124, 1)
	if above.Y+above.Height != aboveSelection.Y-16 {
		t.Fatalf("above toolbar = %+v, want 16px over selection", above)
	}
	if got := screenshotEditorEditBarTop(above, aboveSelection, 56, 8, 24); got != above.Y-8-56 {
		t.Fatalf("above property bar Y = %v, want over toolbar", got)
	}

	aboveLabel := screenshotEditorSizeLabelRect("900 x 290", aboveSelection, above, Size{Width: 1200, Height: 700}, 1)
	if aboveLabel.Y != aboveSelection.Y+8 {
		t.Fatalf("above size label = %+v, want inside selection below the toolbar", aboveLabel)
	}
	if aboveLabel.Y < above.Y+above.Height && aboveLabel.Y+aboveLabel.Height > above.Y {
		t.Fatalf("size label %+v overlaps toolbar %+v", aboveLabel, above)
	}

	belowLabel := screenshotEditorSizeLabelRect("900 x 400", belowSelection, below, Size{Width: 1200, Height: 700}, 1)
	if belowLabel.Y != belowSelection.Y-32 {
		t.Fatalf("below size label = %+v, want above selection", belowLabel)
	}
}

func TestScreenshotEditorToolbarKeepsSameGapAboveAndBelowSelection(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 1, 1),
		selection:    Rect{X: 100, Y: 300, Width: 900, Height: 290},
		hasSelection: true,
		result:       make(chan screenshotEditorOverlayOutcome, 1),
	}
	frame := FrameInfo{Size: Size{Width: 1200, Height: 700}}
	state.draw(&DisplayList{}, frame)
	initialToolbar := state.toolbarRect
	if initialToolbar.Y+initialToolbar.Height != state.selection.Y-16 {
		t.Fatalf("toolbar = %+v, want 16px above selection %+v", initialToolbar, state.selection)
	}

	rectTool := state.toolRects[screenshotEditorToolRect]
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: rectTool.X + 20, Y: rectTool.Y + 20}})
	state.draw(&DisplayList{}, frame)
	if state.toolbarRect != initialToolbar {
		t.Fatalf("toolbar jumped after property bar appeared: before %+v, after %+v", initialToolbar, state.toolbarRect)
	}
	wantEditBarY := screenshotEditorEditBarTop(state.toolbarRect, state.selection, 56, 8, 24)
	if state.editBarRect.Y != wantEditBarY || state.editBarRect.Y+state.editBarRect.Height+8 != state.toolbarRect.Y {
		t.Fatalf("property bar = %+v, toolbar = %+v, want above toolbar at Y=%v", state.editBarRect, state.toolbarRect, wantEditBarY)
	}
}

func TestScreenshotEditorToolbarIconsRenderFromSharedSVGs(t *testing.T) {
	names := append(screenshotEditorToolIconNames[1:],
		"control.undo",
		"screenshot.scrolling-capture",
		"screenshot.cursor",
		"screenshot.pin",
		"control.close",
		"control.download",
		"control.check",
		"control.remove",
		"control.add",
		"control.delete",
	)
	for _, name := range names {
		displayList := &DisplayList{}
		drawScreenshotEditorToolbarIcon(displayList, name, Rect{Width: 40, Height: 40}, Color{R: 255, G: 255, B: 255, A: 255}, 1)
		if displayList.CommandCount() != 1 {
			t.Fatalf("toolbar icon %q did not render as an SVG image", name)
		}
	}
}

func TestScreenshotEditorAnnotationToolsHaveTooltips(t *testing.T) {
	configured := [screenshotEditorToolCount]string{"", "Localized rectangle"}
	for tool := int(screenshotEditorToolRect); tool <= int(screenshotEditorToolMosaic); tool++ {
		if tooltip := screenshotEditorToolTooltip(tool, configured); tooltip == "" {
			t.Fatalf("tool %d tooltip is empty", tool)
		}
	}
	if got := screenshotEditorToolTooltip(int(screenshotEditorToolRect), configured); got != "Localized rectangle (R)" {
		t.Fatalf("configured tooltip = %q", got)
	}
	anchor, actionTooltip := screenshotEditorActionTooltip(screenshotEditorActionCursor, ScreenshotActionTooltips{Cursor: "Localized cursor"}, Rect{}, Rect{}, Rect{X: 10, Y: 20, Width: 40, Height: 40}, Rect{}, Rect{}, Rect{}, Rect{}, Rect{})
	if anchor.X != 10 || actionTooltip != "Localized cursor (C)" {
		t.Fatalf("cursor tooltip = anchor:%+v text:%q", anchor, actionTooltip)
	}
	saveAnchor, saveTooltip := screenshotEditorActionTooltip(screenshotEditorActionSave, ScreenshotActionTooltips{Save: "Localized save"}, Rect{}, Rect{}, Rect{}, Rect{}, Rect{}, Rect{}, Rect{X: 40, Y: 20, Width: 40, Height: 40}, Rect{})
	if saveAnchor.X != 40 || !strings.HasPrefix(saveTooltip, "Localized save (") {
		t.Fatalf("save tooltip = anchor:%+v text:%q", saveAnchor, saveTooltip)
	}
	if got := screenshotEditorEstimatedTextWidth("椭圆", 12); got != 24 {
		t.Fatalf("estimated CJK tooltip width = %v, want 24", got)
	}
	displayList := &DisplayList{}
	drawScreenshotEditorToolTooltip(displayList, Size{Width: 400, Height: 240}, Rect{X: 100, Y: 100, Width: 40, Height: 40}, Rect{}, "Rectangle", 1)
	if displayList.CommandCount() != 2 {
		t.Fatalf("tooltip commands = %d, want background and text", displayList.CommandCount())
	}
}

func TestScreenshotEditorToolTooltipStaysOnSelectionSide(t *testing.T) {
	frame := Size{Width: 1200, Height: 700}
	anchor := Rect{X: 400, Y: 100, Width: 40, Height: 40}
	belowSelection := Rect{X: 100, Y: 20, Width: 900, Height: 60}
	above := screenshotEditorToolTooltipRect(frame, anchor, belowSelection, "Arrow (A)", 1)
	if above.Y != 64 {
		t.Fatalf("below-toolbar tooltip = %+v, want 8px above the icon", above)
	}

	aboveSelection := Rect{X: 100, Y: 160, Width: 900, Height: 200}
	below := screenshotEditorToolTooltipRect(frame, anchor, aboveSelection, "Arrow (A)", 1)
	if below.Y != 148 {
		t.Fatalf("above-toolbar tooltip = %+v, want 8px below the icon", below)
	}
	editBar := Rect{X: 400, Y: 36, Width: 192, Height: 56}
	if below.Y < editBar.Y+editBar.Height && below.Y+below.Height > editBar.Y {
		t.Fatalf("tooltip %+v overlaps property bar %+v", below, editBar)
	}
}

func TestScreenshotEditorChromeUsesSelectionMonitorScale(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 2000, 1200),
		selection:    Rect{X: 200, Y: 100, Width: 1400, Height: 800},
		hasSelection: true,
		chromeScale:  func(Rect) float32 { return 1.5 },
	}
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 2000, Height: 1200}})
	if state.uiScale != 1.5 {
		t.Fatalf("chrome scale = %.2f, want 1.5", state.uiScale)
	}
	if state.toolbarRect.Width != 1029 || state.toolbarRect.Height != 90 {
		t.Fatalf("scaled toolbar = %+v, want 1029x90", state.toolbarRect)
	}
	if state.confirmRect.Width != 60 || state.confirmRect.Height != 60 {
		t.Fatalf("scaled confirm action = %+v, want 60x60", state.confirmRect)
	}
}

func TestScreenshotEditorCursorToggleAndExport(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 400, 200))
	mapped := screenshotEditorCursorPixelFromDesktop(Point{X: -50, Y: 25}, Rect{X: -100, Width: 200, Height: 100}, source)
	if mapped == nil || *mapped != (Point{X: 100, Y: 50}) {
		t.Fatalf("mapped cursor = %+v, want source pixel 100,50", mapped)
	}
	if outside := screenshotEditorCursorPixelFromDesktop(Point{X: 100, Y: 25}, Rect{X: -100, Width: 200, Height: 100}, source); outside != nil {
		t.Fatalf("outside cursor should be unavailable, got %+v", outside)
	}

	cursorPixel := Point{X: 80, Y: 60}
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 200, 120),
		frameSize:    Size{Width: 200, Height: 120},
		selection:    Rect{X: 20, Y: 20, Width: 140, Height: 80},
		hasSelection: true,
		cursorPixel:  &cursorPixel,
	}
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.cursorRect.X + 20, Y: state.cursorRect.Y + 20}})
	if !state.showCursor {
		t.Fatal("cursor toolbar action did not enable the captured pointer")
	}

	target := image.NewRGBA(image.Rect(0, 0, 200, 120))
	draw.Draw(target, target.Bounds(), image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}), image.Point{}, draw.Src)
	if err := renderScreenshotEditorCursor(target, cursorPixel, state.selection, state.frameSize, nil); err != nil {
		t.Fatalf("render cursor: %v", err)
	}
	foundDarkPixel := false
	for y := 55; y < 90 && !foundDarkPixel; y++ {
		for x := 75; x < 115; x++ {
			pixel := target.RGBAAt(x, y)
			if pixel.R < 80 && pixel.G < 80 && pixel.B < 80 && pixel.A > 0 {
				foundDarkPixel = true
				break
			}
		}
	}
	if !foundDarkPixel {
		t.Fatal("exported cursor marker did not contain its dark outline")
	}

	nativeCursor := image.NewRGBA(image.Rect(0, 0, 3, 4))
	nativeCursor.SetRGBA(1, 2, color.RGBA{R: 200, G: 30, B: 20, A: 255})
	nativeTarget := image.NewRGBA(image.Rect(0, 0, 200, 120))
	if err := renderScreenshotEditorCursor(nativeTarget, Point{X: 80, Y: 60}, state.selection, state.frameSize, &screenshotEditorCapturedCursor{
		raster: nativeCursor, hotspot: Point{X: 1, Y: 2},
	}); err != nil {
		t.Fatalf("render captured native cursor: %v", err)
	}
	if got := nativeTarget.RGBAAt(80, 60); got.R != 200 || got.G != 30 || got.B != 20 || got.A != 255 {
		t.Fatalf("native cursor hotspot pixel = %+v", got)
	}
}

func TestScreenshotEditorAnnotationDrawUndoAndExport(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:        testScreenshotImage(t, 1, 1),
		frameSize:    Size{Width: 100, Height: 50},
		selection:    Rect{Width: 100, Height: 50},
		hasSelection: true,
		activeTool:   screenshotEditorToolRect,
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 20, Y: 10}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 50, Y: 30}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 50, Y: 30}})
	if len(state.annotations) != 1 || state.annotations[0].rect != (Rect{X: 20, Y: 10, Width: 30, Height: 20}) {
		t.Fatalf("annotations = %+v", state.annotations)
	}

	source := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			source.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	composited, err := renderScreenshotEditorAnnotations(source, state.annotations, state.selection, state.frameSize, 1)
	if err != nil {
		t.Fatalf("render annotations: %v", err)
	}
	if pixel := composited.RGBAAt(40, 20); pixel.R != 255 || pixel.G != 91 || pixel.B != 54 {
		t.Fatalf("annotation pixel = %+v", pixel)
	}

	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: state.undoRect.X + 20, Y: state.undoRect.Y + 20}})
	if len(state.annotations) != 0 {
		t.Fatalf("undo left %d annotations", len(state.annotations))
	}
}

func TestScreenshotEditorCreatesEveryAnnotationTool(t *testing.T) {
	for _, tool := range []screenshotEditorTool{
		screenshotEditorToolRect,
		screenshotEditorToolEllipse,
		screenshotEditorToolArrow,
		screenshotEditorToolMosaic,
	} {
		state := &screenshotEditorOverlayState{
			frameSize:    Size{Width: 200, Height: 100},
			selection:    Rect{Width: 200, Height: 100},
			hasSelection: true,
			activeTool:   tool,
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 20, Y: 20}})
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 80, Y: 60}})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 60}})
		if len(state.annotations) != 1 || state.annotations[0].tool != tool {
			t.Fatalf("tool %d annotations = %+v", tool, state.annotations)
		}
		if !state.hasSelectedMark || state.selectedAnnotation != 0 || state.activeTool != tool {
			t.Fatalf("tool %d selection = selected:%t index:%d active:%d, want tool kept", tool, state.hasSelectedMark, state.selectedAnnotation, state.activeTool)
		}
	}

	textState := &screenshotEditorOverlayState{
		frameSize:    Size{Width: 200, Height: 100},
		selection:    Rect{Width: 200, Height: 100},
		hasSelection: true,
		activeTool:   screenshotEditorToolText,
	}
	textState.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 20, Y: 20}})
	textState.caretVisible = false
	textState.textInput(TextInputEvent{Kind: TextInputCommit, Text: "中文"})
	if !textState.caretVisible {
		t.Fatal("text input did not immediately reveal the caret")
	}
	textState.key(KeyEvent{Key: KeyEnter, Down: true})
	if textState.activeTool != screenshotEditorToolText {
		t.Fatalf("text tool = %d, want kept after commit", textState.activeTool)
	}
	if len(textState.annotations) != 1 || textState.annotations[0].text != "中文" {
		t.Fatalf("text annotations = %+v", textState.annotations)
	}
	if !textState.hasSelectedMark || textState.selectedAnnotation != 0 {
		t.Fatalf("committed text selection = selected:%t index:%d", textState.hasSelectedMark, textState.selectedAnnotation)
	}
}

func TestScreenshotEditorKeepsDrawingToolForConsecutiveAnnotations(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize:    Size{Width: 400, Height: 240},
		selection:    Rect{X: 20, Y: 20, Width: 360, Height: 200},
		hasSelection: true,
		activeTool:   screenshotEditorToolArrow,
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 60, Y: 60}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 140, Y: 100}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 140, Y: 100}})
	if state.activeTool != screenshotEditorToolArrow || len(state.annotations) != 1 {
		t.Fatalf("after first arrow: tool=%d annotations=%d", state.activeTool, len(state.annotations))
	}

	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 200, Y: 80}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 280, Y: 140}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 280, Y: 140}})
	if state.activeTool != screenshotEditorToolArrow {
		t.Fatalf("tool = %d, want arrow kept for the second stroke", state.activeTool)
	}
	if len(state.annotations) != 2 || state.annotations[1].tool != screenshotEditorToolArrow {
		t.Fatalf("second stroke annotations = %+v, want a second arrow instead of moving the selection", state.annotations)
	}
	if state.selection != (Rect{X: 20, Y: 20, Width: 360, Height: 200}) {
		t.Fatalf("selection = %+v, consecutive drawing should not move the capture", state.selection)
	}
}

func TestScreenshotEditorNumberToolCreatesConsecutiveMovableMarkers(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize:       Size{Width: 240, Height: 140},
		selection:       Rect{Width: 240, Height: 140},
		hasSelection:    true,
		activeTool:      screenshotEditorToolNumber,
		annotationColor: screenshotEditorAnnotationColor,
		nextNumber:      1,
	}
	points := []Point{{X: 40, Y: 40}, {X: 100, Y: 60}, {X: 160, Y: 90}}
	for index, point := range points {
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: point})
		if len(state.annotations) != index+1 || state.annotations[index].number != index+1 || state.annotations[index].start != point {
			t.Fatalf("number click %d annotations = %+v", index+1, state.annotations)
		}
		if state.activeTool != screenshotEditorToolNumber {
			t.Fatalf("number tool stopped after click %d: active=%d", index+1, state.activeTool)
		}
	}

	state.pointer(PointerEvent{Kind: PointerMove, Position: points[1]})
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: points[1]})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 120, Y: 80}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 120, Y: 80}})
	if state.annotations[1].start != (Point{X: 120, Y: 80}) {
		t.Fatalf("moved number marker = %+v", state.annotations[1])
	}
}

func TestScreenshotEditorNumberAnnotationDrawsAndExports(t *testing.T) {
	annotation := screenshotEditorAnnotation{
		tool: screenshotEditorToolNumber, start: Point{X: 40, Y: 30}, number: 12, color: screenshotEditorAnnotationColor,
		textSize: Size{Width: 15, Height: 17}, measuredSize: screenshotEditorNumberFontSize,
	}
	_, _, textRect := screenshotEditorNumberTextLayout(annotation, 1)
	if textRect.X+textRect.Width/2 != annotation.start.X || textRect.Y+textRect.Height/2 != annotation.start.Y {
		t.Fatalf("number text rect = %+v, want center %+v", textRect, annotation.start)
	}
	displayList := &DisplayList{}
	drawScreenshotEditorAnnotations(displayList, []screenshotEditorAnnotation{annotation}, nil, Size{Width: 80, Height: 60}, 1)
	if displayList.CommandCount() != 2 {
		t.Fatalf("number preview commands = %d, want circle and text", displayList.CommandCount())
	}

	source := image.NewRGBA(image.Rect(0, 0, 80, 60))
	exported, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{annotation}, Rect{Width: 80, Height: 60}, Size{Width: 80, Height: 60}, 1)
	if err != nil {
		t.Fatalf("export number annotation: %v", err)
	}
	if pixel := exported.RGBAAt(28, 30); pixel.R != 255 || pixel.G != 91 || pixel.B != 54 {
		t.Fatalf("number marker fill = %+v", pixel)
	}
	if !screenshotEditorAnnotationContains(annotation, Point{X: 40, Y: 30}, 1) {
		t.Fatal("number marker center was not interactive")
	}
}

func TestScreenshotEditorTextToolUsesTextCursorInsideSelection(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize:    Size{Width: 240, Height: 140},
		selection:    Rect{X: 20, Y: 20, Width: 180, Height: 100},
		hasSelection: true,
		activeTool:   screenshotEditorToolText,
		annotations: []screenshotEditorAnnotation{{
			tool: screenshotEditorToolEllipse, rect: Rect{X: 50, Y: 35, Width: 80, Height: 60}, color: screenshotEditorAnnotationColor,
		}},
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 80, Y: 60}})
	if state.pointerCursor != PointerCursorText {
		t.Fatalf("text tool cursor = %d, want text", state.pointerCursor)
	}
	if state.hasHoveredMark {
		t.Fatal("text tool should not reveal shape controls under its input target")
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 60}})
	if !state.textEditing || state.hasSelectedMark || len(state.annotations) != 1 {
		t.Fatalf("text over shape = editing:%t selected:%t annotations:%d", state.textEditing, state.hasSelectedMark, len(state.annotations))
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 10, Y: 10}})
	if state.pointerCursor != PointerCursorDefault {
		t.Fatalf("text tool cursor outside selection = %d, want default", state.pointerCursor)
	}
}

func TestScreenshotEditorTextUsesDisplayScaleAndClickEditsInPlace(t *testing.T) {
	annotation := screenshotEditorAnnotation{
		tool: screenshotEditorToolText, start: Point{X: 30, Y: 30}, text: "Before", color: screenshotEditorAnnotationColor, fontSize: 20,
		textSize: Size{Width: 70, Height: 28}, measuredSize: 20,
	}
	if got := screenshotEditorAnnotationRenderedFontSize(annotation, 1.5); got != 30 {
		t.Fatalf("rendered text size = %v, want 30", got)
	}
	if !screenshotEditorAnnotationContains(annotation, Point{X: 40, Y: 60}, 1.5) || screenshotEditorAnnotationContains(annotation, Point{X: 40, Y: 60}, 1) {
		t.Fatal("text hit bounds did not follow display scale")
	}
	state := &screenshotEditorOverlayState{
		frameSize: Size{Width: 240, Height: 140}, selection: Rect{Width: 240, Height: 140}, hasSelection: true,
		activeTool: screenshotEditorToolSelect, annotations: []screenshotEditorAnnotation{annotation}, uiScale: 1.5,
	}
	click := Point{X: 45, Y: 42}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: click})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: click})
	if !state.textEditing || !state.hasEditingText || state.editingTextIndex != 0 || state.textDraft != "Before" {
		t.Fatalf("click edit state = editing:%t existing:%t index:%d draft:%q", state.textEditing, state.hasEditingText, state.editingTextIndex, state.textDraft)
	}
	state.textDraft = "After"
	state.key(KeyEvent{Key: KeyEnter, Down: true})
	if len(state.annotations) != 1 || state.annotations[0].text != "After" {
		t.Fatalf("edited text annotations = %+v", state.annotations)
	}
	if state.annotations[0].textSize != (Size{}) || state.annotations[0].measuredSize != 0 {
		t.Fatalf("edited text retained stale metrics: %+v", state.annotations[0])
	}
}

func TestScreenshotEditorTextBoundsPreferMeasuredMetrics(t *testing.T) {
	annotation := screenshotEditorAnnotation{
		tool: screenshotEditorToolText, start: Point{X: 20, Y: 30}, text: "撒旦发射点发", fontSize: 48,
		textSize: Size{Width: 302, Height: 58}, measuredSize: 48,
	}
	if got := screenshotEditorAnnotationBounds(annotation, 1); got != (Rect{X: 20, Y: 30, Width: 302, Height: 58}) {
		t.Fatalf("measured text bounds = %+v", got)
	}
	if got := screenshotEditorTextFrame(annotation, 1); got != (Rect{X: 16, Y: 26, Width: 310, Height: 66}) {
		t.Fatalf("measured text frame = %+v", got)
	}
}

func TestScreenshotEditorCaretEstimateTracksMixedGlyphWidths(t *testing.T) {
	fontSize := float32(40)
	text := "asdfasfsdfsaf"
	width := screenshotEditorEstimatedTextWidth(text, fontSize)
	if width >= float32(len(text))*fontSize*0.6 || width <= float32(len(text))*fontSize*0.4 {
		t.Fatalf("caret width = %v, want a compact mixed-lowercase estimate", width)
	}
	if cjk := screenshotEditorEstimatedTextWidth("文字", fontSize); cjk != 2*fontSize {
		t.Fatalf("CJK caret width = %v, want %v", cjk, 2*fontSize)
	}
}

func TestScreenshotEditorClickPositionsCaretInsideExistingText(t *testing.T) {
	annotation := screenshotEditorAnnotation{
		tool: screenshotEditorToolText, start: Point{X: 30, Y: 30}, text: "测试一下", color: screenshotEditorAnnotationColor, fontSize: 20,
	}
	state := &screenshotEditorOverlayState{
		frameSize: Size{Width: 240, Height: 140}, selection: Rect{Width: 240, Height: 140}, hasSelection: true,
		activeTool: screenshotEditorToolText, annotations: []screenshotEditorAnnotation{annotation}, uiScale: 1,
	}
	clickAfterFirstRune := Point{X: 49, Y: 40}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: clickAfterFirstRune})
	if !state.textEditing || state.textCaret != 1 {
		t.Fatalf("positioned edit = editing:%t caret:%d, want caret 1", state.textEditing, state.textCaret)
	}
	state.textInput(TextInputEvent{Kind: TextInputCommit, Text: "含"})
	if state.textDraft != "测含试一下" || state.textCaret != 2 {
		t.Fatalf("inserted text = %q caret:%d", state.textDraft, state.textCaret)
	}
	state.key(KeyEvent{Key: KeyEnter, Down: true})
	if len(state.annotations) != 1 || state.annotations[0].text != "测含试一下" {
		t.Fatalf("committed positioned edit = %+v", state.annotations)
	}
}

func TestScreenshotEditorCompositionRendersAtCaret(t *testing.T) {
	value, caretPrefix := screenshotEditorTextEditingValue("测试", "含", 1)
	if value != "测含试" || caretPrefix != "测含" {
		t.Fatalf("composition value = %q prefix = %q", value, caretPrefix)
	}
}

func TestScreenshotEditorTextFrameSeparatesEditingAndDragging(t *testing.T) {
	annotation := screenshotEditorAnnotation{
		tool: screenshotEditorToolText, start: Point{X: 50, Y: 40}, text: "Wox", color: screenshotEditorAnnotationColor, fontSize: 20,
	}
	state := &screenshotEditorOverlayState{
		frameSize: Size{Width: 240, Height: 140}, selection: Rect{Width: 240, Height: 140}, hasSelection: true,
		activeTool: screenshotEditorToolSelect, annotations: []screenshotEditorAnnotation{annotation}, uiScale: 1,
	}
	bodyPoint := Point{X: 60, Y: 50}
	state.pointer(PointerEvent{Kind: PointerMove, Position: bodyPoint})
	if !state.hasHoveredMark || state.pointerCursor != PointerCursorText {
		t.Fatalf("text content hover = hovered:%t cursor:%d, want text", state.hasHoveredMark, state.pointerCursor)
	}

	frame := screenshotEditorTextFrame(annotation, 1)
	borderPoint := Point{X: frame.X + frame.Width/2, Y: frame.Y}
	state.pointer(PointerEvent{Kind: PointerMove, Position: borderPoint})
	if !state.hasHoveredMark || state.pointerCursor != PointerCursorMove {
		t.Fatalf("text border hover = hovered:%t cursor:%d, want move", state.hasHoveredMark, state.pointerCursor)
	}
	displayList := &DisplayList{}
	drawScreenshotEditorAnnotationHandles(displayList, annotation, 1)
	if displayList.CommandCount() == 0 {
		t.Fatal("hovered text did not draw its dashed frame")
	}

	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: borderPoint})
	movedPoint := Point{X: borderPoint.X + 20, Y: borderPoint.Y + 10}
	state.pointer(PointerEvent{Kind: PointerMove, Position: movedPoint})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: movedPoint})
	if got := state.annotations[0].start; got != (Point{X: 70, Y: 50}) {
		t.Fatalf("dragged text start = %+v, want {70 50}", got)
	}

	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 60}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 60}})
	if !state.textEditing || state.textDraft != "Wox" {
		t.Fatalf("text click did not enter editing: editing:%t draft:%q", state.textEditing, state.textDraft)
	}
}

func TestScreenshotEditorArrowShaftStopsInsideHead(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 120, 80))
	draw.Draw(source, source.Bounds(), image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}), image.Point{}, draw.Src)
	output, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{{
		tool: screenshotEditorToolArrow, start: Point{X: 20, Y: 40}, end: Point{X: 80, Y: 40}, color: screenshotEditorAnnotationColor,
	}}, Rect{Width: 120, Height: 80}, Size{Width: 120, Height: 80}, 1)
	if err != nil {
		t.Fatalf("render arrow: %v", err)
	}
	if got := output.RGBAAt(81, 40); got.R != 255 || got.G != 255 || got.B != 255 {
		t.Fatalf("arrow protruded beyond tip: %+v", got)
	}
	if got := output.RGBAAt(79, 40); got.R != screenshotEditorAnnotationColor.R || got.G != screenshotEditorAnnotationColor.G || got.B != screenshotEditorAnnotationColor.B {
		t.Fatalf("arrow tip was not filled: %+v", got)
	}
}

func TestScreenshotEditorShiftConstrainsNewShapes(t *testing.T) {
	for _, tool := range []screenshotEditorTool{screenshotEditorToolRect, screenshotEditorToolEllipse} {
		state := &screenshotEditorOverlayState{
			frameSize:    Size{Width: 200, Height: 120},
			selection:    Rect{Width: 200, Height: 120},
			hasSelection: true,
			activeTool:   tool,
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 20, Y: 20}})
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 80, Y: 50}, Modifiers: KeyModifierShift})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 50}, Modifiers: KeyModifierShift})
		if len(state.annotations) != 1 {
			t.Fatalf("tool %d annotations = %+v", tool, state.annotations)
		}
		if got := state.annotations[0].rect; got != (Rect{X: 20, Y: 20, Width: 60, Height: 60}) {
			t.Fatalf("tool %d constrained rect = %+v, want 60x60", tool, got)
		}
	}
}

func TestScreenshotEditorShiftConstrainsResizedShapes(t *testing.T) {
	for _, tool := range []screenshotEditorTool{screenshotEditorToolRect, screenshotEditorToolEllipse} {
		state := &screenshotEditorOverlayState{
			frameSize:          Size{Width: 240, Height: 160},
			selection:          Rect{X: 10, Y: 10, Width: 210, Height: 130},
			hasSelection:       true,
			activeTool:         screenshotEditorToolSelect,
			annotations:        []screenshotEditorAnnotation{{tool: tool, rect: Rect{X: 50, Y: 40, Width: 40, Height: 30}}},
			selectedAnnotation: 0,
			hasSelectedMark:    true,
		}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 90, Y: 70}})
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 120, Y: 90}, Modifiers: KeyModifierShift})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 120, Y: 90}, Modifiers: KeyModifierShift})
		if got := state.annotations[0].rect; got != (Rect{X: 50, Y: 40, Width: 70, Height: 70}) {
			t.Fatalf("tool %d shifted resize = %+v, want 70x70", tool, got)
		}
	}
}

func TestScreenshotEditorToolbarKeyboardShortcuts(t *testing.T) {
	cursor := Point{X: 20, Y: 20}
	state := &screenshotEditorOverlayState{
		selection: Rect{Width: 200, Height: 100}, hasSelection: true, cursorPixel: &cursor,
		annotations: []screenshotEditorAnnotation{{tool: screenshotEditorToolRect, rect: Rect{Width: 20, Height: 20}}},
	}
	for key, tool := range map[Key]screenshotEditorTool{
		Key("r"): screenshotEditorToolRect,
		Key("e"): screenshotEditorToolEllipse,
		Key("t"): screenshotEditorToolText,
		Key("a"): screenshotEditorToolArrow,
		Key("n"): screenshotEditorToolNumber,
		Key("m"): screenshotEditorToolMosaic,
	} {
		if !state.key(KeyEvent{Key: key, Down: true}) || state.activeTool != tool {
			t.Fatalf("shortcut %q selected tool %d, want %d", key, state.activeTool, tool)
		}
	}
	if !state.key(KeyEvent{Key: Key("c"), Down: true}) || !state.showCursor {
		t.Fatal("cursor shortcut did not toggle captured cursor")
	}
	if !state.key(KeyEvent{Key: Key("u"), Down: true}) || len(state.annotations) != 0 {
		t.Fatalf("undo shortcut left annotations = %+v", state.annotations)
	}
	state.textEditing = true
	state.activeTool = screenshotEditorToolText
	if state.key(KeyEvent{Key: Key("r"), Down: true}) || state.activeTool != screenshotEditorToolText {
		t.Fatal("tool shortcut should not run while entering annotation text")
	}
}

func TestScreenshotEditorSelectMovesResizesAndDeletesAnnotation(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize:    Size{Width: 240, Height: 140},
		selection:    Rect{X: 20, Y: 20, Width: 160, Height: 100},
		hasSelection: true,
		activeTool:   screenshotEditorToolSelect,
		annotations:  []screenshotEditorAnnotation{{tool: screenshotEditorToolRect, rect: Rect{X: 50, Y: 40, Width: 40, Height: 30}}},
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 65, Y: 55}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 85, Y: 65}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 85, Y: 65}})
	if got := state.annotations[0].rect; got != (Rect{X: 70, Y: 50, Width: 40, Height: 30}) {
		t.Fatalf("moved annotation = %+v", got)
	}

	bottomRight := Point{X: 110, Y: 80}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: bottomRight})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 130, Y: 95}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 130, Y: 95}})
	if got := state.annotations[0].rect; got != (Rect{X: 70, Y: 50, Width: 60, Height: 45}) {
		t.Fatalf("resized annotation = %+v", got)
	}

	if !state.key(KeyEvent{Key: KeyDelete, Down: true}) || len(state.annotations) != 0 {
		t.Fatalf("delete left annotations = %+v", state.annotations)
	}

	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 60, Y: 70}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 80, Y: 80}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 80}})
	if got := state.selection; got != (Rect{X: 40, Y: 30, Width: 160, Height: 100}) {
		t.Fatalf("moved selection = %+v", got)
	}
}

func TestScreenshotEditorHoverShowsHandlesAndMovesAnnotationFromDrawingTool(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize:    Size{Width: 240, Height: 140},
		selection:    Rect{X: 20, Y: 20, Width: 180, Height: 100},
		hasSelection: true,
		activeTool:   screenshotEditorToolRect,
		annotations:  []screenshotEditorAnnotation{{tool: screenshotEditorToolRect, rect: Rect{X: 50, Y: 40, Width: 60, Height: 40}}},
	}

	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 80, Y: 60}})
	if !state.hasHoveredMark || state.hoveredAnnotation != 0 || state.pointerCursor != PointerCursorMove {
		t.Fatalf("annotation hover = hovered:%t index:%d cursor:%d", state.hasHoveredMark, state.hoveredAnnotation, state.pointerCursor)
	}

	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 50, Y: 40}})
	if state.pointerCursor != PointerCursorResizeNWSE {
		t.Fatalf("top-left handle cursor = %d, want northwest-southeast resize", state.pointerCursor)
	}

	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 43, Y: 50}})
	if state.hasHoveredMark || state.pointerCursor != PointerCursorDefault {
		t.Fatalf("cursor outside annotation = hovered:%t cursor:%d, want default", state.hasHoveredMark, state.pointerCursor)
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 160, Y: 80}})
	if state.pointerCursor != PointerCursorDefault {
		t.Fatalf("cursor in selection but outside annotation = %d, want default", state.pointerCursor)
	}

	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 80, Y: 60}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 100, Y: 70}})
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 100, Y: 70}})
	if state.activeTool != screenshotEditorToolSelect {
		t.Fatalf("active tool = %d, want select after picking an existing annotation", state.activeTool)
	}
	if got := state.annotations[0].rect; got != (Rect{X: 70, Y: 50, Width: 60, Height: 40}) {
		t.Fatalf("moved annotation = %+v", got)
	}
}

func TestScreenshotEditorHoverAndClickPreferNestedTextOverEnclosingShape(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize:          Size{Width: 320, Height: 220},
		selection:          Rect{X: 10, Y: 10, Width: 300, Height: 200},
		hasSelection:       true,
		activeTool:         screenshotEditorToolSelect,
		selectedAnnotation: 1,
		hasSelectedMark:    true,
		annotations: []screenshotEditorAnnotation{
			{tool: screenshotEditorToolText, start: Point{X: 110, Y: 90}, text: "Nested", fontSize: 20},
			{tool: screenshotEditorToolRect, rect: Rect{X: 40, Y: 40, Width: 240, Height: 140}},
		},
	}
	point := Point{X: 125, Y: 100}

	state.pointer(PointerEvent{Kind: PointerMove, Position: point})
	if !state.hasHoveredMark || state.hoveredAnnotation != 0 || state.pointerCursor != PointerCursorText {
		t.Fatalf("nested text hover = hovered:%t index:%d cursor:%d", state.hasHoveredMark, state.hoveredAnnotation, state.pointerCursor)
	}

	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
	if !state.textEditing || !state.hasEditingText || state.editingTextIndex != 0 {
		t.Fatalf("nested text click = editing:%t hasIndex:%t index:%d", state.textEditing, state.hasEditingText, state.editingTextIndex)
	}
}

func TestScreenshotEditorTextToolHoverShowsNestedTextWithoutSelectingShape(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize:    Size{Width: 320, Height: 220},
		selection:    Rect{X: 10, Y: 10, Width: 300, Height: 200},
		hasSelection: true,
		activeTool:   screenshotEditorToolText,
		annotations: []screenshotEditorAnnotation{
			{tool: screenshotEditorToolText, start: Point{X: 110, Y: 90}, text: "Nested", fontSize: 20},
			{tool: screenshotEditorToolEllipse, rect: Rect{X: 40, Y: 40, Width: 240, Height: 140}},
		},
	}

	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 125, Y: 100}})
	if !state.hasHoveredMark || state.hoveredAnnotation != 0 || state.pointerCursor != PointerCursorText {
		t.Fatalf("text-tool hover = hovered:%t index:%d cursor:%d", state.hasHoveredMark, state.hoveredAnnotation, state.pointerCursor)
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 70, Y: 70}})
	if state.hasHoveredMark || state.pointerCursor != PointerCursorText {
		t.Fatalf("shape interior should remain a text creation target: hovered:%t cursor:%d", state.hasHoveredMark, state.pointerCursor)
	}
}

func TestScreenshotEditorAnnotationHandlesMatchFlutterContract(t *testing.T) {
	displayList := &DisplayList{}
	drawScreenshotEditorAnnotationHandles(displayList, screenshotEditorAnnotation{
		tool: screenshotEditorToolRect,
		rect: Rect{X: 20, Y: 20, Width: 80, Height: 50},
	}, 1)
	if displayList.CommandCount() != 8 {
		t.Fatalf("rectangle handle commands = %d, want 8", displayList.CommandCount())
	}
	expected := &DisplayList{}
	for _, point := range screenshotEditorRectHandlePoints(Rect{X: 20, Y: 20, Width: 80, Height: 50}) {
		expected.StrokeRoundedRect(Rect{X: point.X - 5, Y: point.Y - 5, Width: 10, Height: 10}, 5, 1.5, Color{R: 255, G: 255, B: 255, A: 255})
	}
	if err := displayList.Compare(expected); err != nil {
		t.Fatalf("rectangle handles are not uniform small white circles: %v", err)
	}
	arrowDisplayList := &DisplayList{}
	drawScreenshotEditorAnnotationHandles(arrowDisplayList, screenshotEditorAnnotation{
		tool: screenshotEditorToolArrow, start: Point{X: 20, Y: 20}, end: Point{X: 80, Y: 50}, color: screenshotEditorAnnotationColor,
	}, 1)
	arrowExpected := &DisplayList{}
	for _, point := range []Point{{X: 20, Y: 20}, {X: 80, Y: 50}} {
		arrowExpected.StrokeRoundedRect(Rect{X: point.X - 5, Y: point.Y - 5, Width: 10, Height: 10}, 5, 1.5, Color{R: 255, G: 255, B: 255, A: 255})
	}
	if err := arrowDisplayList.Compare(arrowExpected); err != nil {
		t.Fatalf("arrow handles are not uniform small white circles: %v", err)
	}
	if screenshotEditorAnnotationStroke != 3 {
		t.Fatalf("logical annotation stroke = %v, want 3", screenshotEditorAnnotationStroke)
	}
	if got := screenshotEditorAnnotationPreviewStroke(1); got != 3 {
		t.Fatalf("1x preview stroke = %v, want 3", got)
	}
	if got := screenshotEditorAnnotationPreviewStroke(1.5); got != 4.5 {
		t.Fatalf("1.5x preview stroke = %v, want 4.5", got)
	}
}

func TestScreenshotEditorEditBarUpdatesCreationAndSelectedAnnotation(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image:           testScreenshotImage(t, 1, 1),
		frameSize:       Size{Width: 800, Height: 600},
		selection:       Rect{X: 100, Y: 100, Width: 400, Height: 300},
		hasSelection:    true,
		activeTool:      screenshotEditorToolRect,
		annotationColor: screenshotEditorAnnotationColor,
		mosaicRadius:    screenshotEditorMosaicRadius,
		textFontSize:    screenshotEditorTextFontSize,
	}
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	if state.editBarRect.Height != 56 || state.editBarRect.Y != state.toolbarRect.Y+state.toolbarRect.Height+8 {
		t.Fatalf("secondary toolbar = %+v, main toolbar = %+v", state.editBarRect, state.toolbarRect)
	}
	greenRect := state.editColorRects[2]
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: greenRect.X + 10, Y: greenRect.Y + 10}})
	if state.annotationColor != screenshotEditorPalette[2] {
		t.Fatalf("creation color = %+v, want %+v", state.annotationColor, screenshotEditorPalette[2])
	}

	state.activeTool = screenshotEditorToolMosaic
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	largeRect := state.editSizeRects[2]
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: largeRect.X + 16, Y: largeRect.Y + 16}})
	if state.mosaicRadius != 28 {
		t.Fatalf("mosaic radius = %v, want 28", state.mosaicRadius)
	}

	state.activeTool = screenshotEditorToolText
	state.hasSelectedMark = false
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	increaseCreationTextRect := state.editIncreaseRect
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: increaseCreationTextRect.X + 21, Y: increaseCreationTextRect.Y + 21}})
	if state.textFontSize != 22 {
		t.Fatalf("creation text font size = %v, want 22", state.textFontSize)
	}

	state.activeTool = screenshotEditorToolSelect
	state.annotations = []screenshotEditorAnnotation{{tool: screenshotEditorToolText, start: Point{X: 200, Y: 180}, text: "Wox", fontSize: 20}}
	state.selectedAnnotation = 0
	state.hasSelectedMark = true
	state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize})
	increaseRect := state.editIncreaseRect
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: increaseRect.X + 21, Y: increaseRect.Y + 21}})
	if state.annotations[0].fontSize != 22 {
		t.Fatalf("text font size = %v, want 22", state.annotations[0].fontSize)
	}
	deleteRect := state.editDeleteRect
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: deleteRect.X + 21, Y: deleteRect.Y + 21}})
	if len(state.annotations) != 0 {
		t.Fatalf("delete left annotations = %+v", state.annotations)
	}
}

func testScreenshotImage(t *testing.T, width, height int) *Image {
	t.Helper()
	prepared, err := NewImage(image.NewRGBA(image.Rect(0, 0, width, height)))
	if err != nil {
		t.Fatalf("prepare test screenshot image: %v", err)
	}
	return prepared
}

func TestScreenshotScrollingCaptureMatchesAndStitchesBothDirections(t *testing.T) {
	document := image.NewRGBA(image.Rect(0, 0, 120, 900))
	for y := 0; y < document.Bounds().Dy(); y++ {
		for x := 0; x < document.Bounds().Dx(); x++ {
			document.SetRGBA(x, y, color.RGBA{
				R: uint8((x*7 + y*3) % 251),
				G: uint8((x*11 + y*5) % 253),
				B: uint8((x*13 + y*17) % 255),
				A: 255,
			})
		}
	}
	viewport := func(top int) *image.RGBA {
		frame := image.NewRGBA(image.Rect(0, 0, 120, 300))
		draw.Draw(frame, frame.Bounds(), document, image.Pt(0, top), draw.Src)
		return frame
	}

	frames, accepted := appendScreenshotScrollingFrame(nil, viewport(180))
	if !accepted {
		t.Fatal("first frame was rejected")
	}
	frames, accepted = appendScreenshotScrollingFrame(frames, viewport(0))
	if !accepted || len(frames) != 2 {
		t.Fatalf("prepend accepted=%v frames=%d", accepted, len(frames))
	}
	frames, accepted = appendScreenshotScrollingFrame(frames, viewport(360))
	if !accepted || len(frames) != 3 {
		t.Fatalf("append accepted=%v frames=%d", accepted, len(frames))
	}
	if _, accepted = appendScreenshotScrollingFrame(frames, viewport(360)); accepted {
		t.Fatal("duplicate frame was accepted")
	}

	stitched, err := stitchScreenshotScrollingFrames(frames)
	if err != nil {
		t.Fatalf("stitch frames: %v", err)
	}
	if stitched.Bounds() != image.Rect(0, 0, 120, 660) {
		t.Fatalf("stitched bounds = %v, want 120x660", stitched.Bounds())
	}
	for _, point := range []image.Point{{X: 17, Y: 0}, {X: 71, Y: 299}, {X: 23, Y: 479}, {X: 99, Y: 659}} {
		if got, want := stitched.RGBAAt(point.X, point.Y), document.RGBAAt(point.X, point.Y); got != want {
			t.Fatalf("pixel %v = %+v, want %+v", point, got, want)
		}
	}
}

func TestScreenshotScrollingControlLayoutIncludesSave(t *testing.T) {
	toolbar, cancel, save, confirm := screenshotScrollingControlLayout(Size{Width: 216, Height: 300}, 1)
	if toolbar != (Rect{X: 22, Y: 244, Width: 172, Height: 56}) {
		t.Fatalf("toolbar = %+v", toolbar)
	}
	if cancel != (Rect{X: 40, Y: 252, Width: 40, Height: 40}) {
		t.Fatalf("cancel = %+v", cancel)
	}
	if save != (Rect{X: 88, Y: 252, Width: 40, Height: 40}) {
		t.Fatalf("save = %+v", save)
	}
	if confirm != (Rect{X: 136, Y: 252, Width: 40, Height: 40}) {
		t.Fatalf("confirm = %+v", confirm)
	}

	toolbar, cancel, save, confirm = screenshotScrollingControlLayout(Size{Width: 324, Height: 450}, 1.5)
	if toolbar != (Rect{X: 33, Y: 366, Width: 258, Height: 84}) {
		t.Fatalf("scaled toolbar = %+v", toolbar)
	}
	if cancel != (Rect{X: 60, Y: 378, Width: 60, Height: 60}) {
		t.Fatalf("scaled cancel = %+v", cancel)
	}
	if save != (Rect{X: 132, Y: 378, Width: 60, Height: 60}) {
		t.Fatalf("scaled save = %+v", save)
	}
	if confirm != (Rect{X: 204, Y: 378, Width: 60, Height: 60}) {
		t.Fatalf("scaled confirm = %+v", confirm)
	}
}
