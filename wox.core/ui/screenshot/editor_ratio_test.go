package screenshot

import (
	"image"
	"math"
	"testing"

	"wox/common/icons"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestScreenshotRatioChevronScales keeps the catalog icon centered next to the measured label.
func TestScreenshotRatioChevronScales(t *testing.T) {
	surface := &screenshotTestSurface{}
	labels := ScreenshotSizeLabels{RatioFree: "Free"}
	for _, scale := range []float32{1, 1.25, 1.5, 2, 2.5} {
		style := TextStyle{Size: 14 * scale, Weight: FontWeightSemibold}
		metrics, _ := surface.MeasureText(labels.RatioFree, style)
		width := float32(math.Ceil(float64(metrics.Size.Width))) + 42*scale
		bounds := Rect{X: 100, Y: 100, Width: width, Height: 26 * scale}
		actual := &DisplayList{}
		drawScreenshotRatioTrigger(actual, surface, bounds, labels, 0, false, scale)
		expected := &DisplayList{}
		expected.FillRoundedRect(bounds, 10*scale, Color{R: 23, G: 23, B: 23, A: 255})
		white := Color{R: 255, G: 255, B: 255, A: 255}
		expected.DrawText(labels.RatioFree, Rect{X: bounds.X + 8*scale, Y: bounds.Y + (bounds.Height-metrics.Size.Height)/2, Width: metrics.Size.Width, Height: metrics.Size.Height}, style, white)
		icon := Rect{X: bounds.X + bounds.Width - 28*scale, Y: bounds.Y + (bounds.Height-20*scale)/2, Width: 20 * scale, Height: 20 * scale}
		drawScreenshotEditorToolbarIconSized(expected, icons.ControlChevronDown, icon, white, scale, 20)
		if err := actual.Compare(expected); err != nil {
			t.Fatalf("scale %v: dropdown icon alignment: %v", scale, err)
		}
	}
}

// TestScreenshotPillWidthsUseFontMetrics covers compact text, icons, translations, and menu content.
func TestScreenshotPillWidthsUseFontMetrics(t *testing.T) {
	for _, surface := range []woxwidget.HostServices{&screenshotTestSurface{}, &screenshotTooltipTestSurface{}} {
		for _, scale := range []float32{1, 1.25, 2} {
			frame := Size{Width: 3000, Height: 1500}
			selection := Rect{X: 100, Y: 100, Width: 800, Height: 500}
			sizeLabel := "720 x 1280"
			text, _ := surface.MeasureText(sizeLabel, TextStyle{Size: 14 * scale, Weight: FontWeightSemibold})
			chip := screenshotEditorSizeLabelRect(surface, sizeLabel, selection, Rect{}, frame, scale, false)
			locked := screenshotEditorSizeLabelRect(surface, sizeLabel, selection, Rect{}, frame, scale, true)
			if chip.Width != float32(math.Ceil(float64(text.Size.Width)))+16*scale || locked.Width != chip.Width+20*scale {
				t.Fatalf("scale=%v: size chip widths %v/%v, measured %v", scale, chip.Width, locked.Width, text.Size.Width)
			}
			for _, label := range []string{"Free", "9:16", "Personalizada", "自定义"} {
				labels := ScreenshotSizeLabels{RatioFree: label, RatioCustom: label}
				text, _ := surface.MeasureText(label, TextStyle{Size: 14 * scale, Weight: FontWeightSemibold})
				trigger := screenshotRatioTriggerRect(surface, chip, selection, Rect{Width: frame.Width, Height: frame.Height}, labels, 0, scale)
				if trigger.Width != float32(math.Ceil(float64(text.Size.Width)))+42*scale {
					t.Fatalf("scale=%v label=%q: trigger width %v for text %v", scale, label, trigger.Width, text.Size.Width)
				}
				menu := screenshotRatioMenuRect(surface, trigger, Rect{Width: frame.Width, Height: frame.Height}, labels, scale)
				menuText, _ := surface.MeasureText(label, TextStyle{Size: 13 * scale})
				if menu.Width < float32(math.Ceil(float64(menuText.Size.Width)))+20*scale {
					t.Fatalf("scale=%v label=%q: menu clips its widest entry", scale, label)
				}
			}
		}
	}
}

// TestScreenshotPillHover gives both controls the same outline and tooltip feedback.
func TestScreenshotPillHover(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 1000, 800), frameSize: Size{Width: 1000, Height: 800},
		selection: Rect{X: 250, Y: 250, Width: 400, Height: 300}, hasSelection: true, colorInspectorDismissed: true,
		sizeDialogOptions: ScreenshotOptions{SizeLabels: ScreenshotSizeLabels{Title: "Edit size", RatioFree: "Free", RatioPresets: "Aspect ratio presets"}},
	}
	frame := FrameInfo{Size: state.frameSize}
	baseline := &DisplayList{}
	state.draw(baseline, frame)
	for _, bounds := range []Rect{state.sizeLabelRect, state.sizeRatioRect} {
		state.pointerInside = true
		state.pointerPosition = Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
		list := &DisplayList{}
		state.draw(list, frame)
		if list.CommandCount() <= baseline.CommandCount()+1 {
			t.Fatalf("hover did not display a tooltip at %+v: commands=%d, baseline=%d", bounds, list.CommandCount(), baseline.CommandCount())
		}
		renderer, err := woxui.NewSoftwareRenderer(int(frame.Size.Width), int(frame.Size.Height))
		if err != nil {
			t.Fatal(err)
		}
		if err := renderer.Render(list); err != nil {
			t.Fatal(err)
		}
		pixel := renderer.RGBA().RGBAAt(int(bounds.X+bounds.Width/2), int(bounds.Y))
		if pixel.G <= pixel.R*2 {
			t.Fatalf("hover outline is missing at %+v: %+v", bounds, pixel)
		}
	}
}

// TestScreenshotRatioDropdown exercises direct selection, free resizing, keyboard navigation, and dismissal.
func TestScreenshotRatioDropdown(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 1200, 800), frameSize: Size{Width: 1200, Height: 800},
		selection: Rect{X: 200, Y: 200, Width: 300, Height: 200}, hasSelection: true,
		sizeDialogOptions: ScreenshotOptions{SizeLabels: ScreenshotSizeLabels{RatioFree: "Free", RatioCustom: "Custom"}},
		result:            make(chan screenshotEditorOverlayOutcome, 1),
	}
	draw := func() { state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize}) }
	click := func(point Point) {
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: point})
	}
	draw()
	trigger := state.sizeRatioRect
	if trigger.Width == 0 || screenshotEditorRectContains(state.sizeLabelRect, Point{X: trigger.X + trigger.Width/2, Y: trigger.Y + trigger.Height/2}) {
		t.Fatalf("ratio trigger must be separate from the size editor: %+v", trigger)
	}
	click(Point{X: trigger.X + trigger.Width/2, Y: trigger.Y + trigger.Height/2})
	if !state.sizeRatioMenuOpen || state.activeSizeDialog() != nil {
		t.Fatal("ratio trigger did not open the direct menu")
	}
	draw()
	menu := state.sizeRatioMenuRect
	rowHeight := menu.Height / float32(len(screenshotSizeRatioPresets)+1)
	click(Point{X: menu.X + menu.Width/2, Y: menu.Y + 4.5*rowHeight})
	if state.sizeRatioMenuOpen || state.selectionAspectRatio != 16.0/9 {
		t.Fatal("16:9 preset was not applied")
	}
	pixels, err := state.selectionPixelBoundsLocked()
	if err != nil || pixels != image.Rect(200, 200, 500, 369) {
		t.Fatalf("preset crop=%v err=%v", pixels, err)
	}
	state.editOriginalRect, state.editHandle, state.editMode = state.selection, screenshotEditorHandleBottomRight, screenshotEditorEditResizeSelection
	state.start = Point{}
	state.updateSelectEditLocked(Point{X: 100, Y: 20}, 0)
	if math.Abs(float64(state.selection.Width/state.selection.Height)-16.0/9) > 0.0001 {
		t.Fatalf("drag lost selected ratio: %+v", state.selection)
	}
	state.editMode = screenshotEditorEditNone
	draw()
	trigger = state.sizeRatioRect
	click(Point{X: trigger.X + trigger.Width/2, Y: trigger.Y + trigger.Height/2})
	state.key(KeyEvent{Key: KeyArrowUp, Down: true})
	state.key(KeyEvent{Key: KeyEnter, Down: true})
	if state.sizeRatioMenuOpen || state.selectionAspectRatio != 4.0/3 {
		t.Fatal("keyboard selection did not apply the previous preset")
	}
	draw()
	trigger = state.sizeRatioRect
	click(Point{X: trigger.X + trigger.Width/2, Y: trigger.Y + trigger.Height/2})
	draw()
	menu = state.sizeRatioMenuRect
	before := state.selection
	click(Point{X: menu.X + menu.Width/2, Y: menu.Y + menu.Height/16})
	if state.selectionAspectRatio != 0 || state.selection != before {
		t.Fatal("Free must unlock without changing the selection")
	}
	draw()
	trigger = state.sizeRatioRect
	click(Point{X: trigger.X + trigger.Width/2, Y: trigger.Y + trigger.Height/2})
	state.key(KeyEvent{Key: KeyEscape, Down: true})
	if state.sizeRatioMenuOpen || len(state.result) != 0 {
		t.Fatal("Escape must close only the ratio menu")
	}
	draw()
	trigger = state.sizeRatioRect
	click(Point{X: trigger.X + trigger.Width/2, Y: trigger.Y + trigger.Height/2})
	click(Point{X: 10, Y: 10})
	if state.sizeRatioMenuOpen || state.dragging || state.selection != before {
		t.Fatal("clicking outside must dismiss the menu without creating a selection")
	}
	state.selectionAspectRatio = 1.234
	draw()
	trigger = state.sizeRatioRect
	click(Point{X: trigger.X + trigger.Width/2, Y: trigger.Y + trigger.Height/2})
	state.key(KeyEvent{Key: KeyArrowUp, Down: true})
	state.key(KeyEvent{Key: KeyEnter, Down: true})
	if state.selectionAspectRatio != 9.0/16 {
		t.Fatal("Up from a custom ratio must select the last preset")
	}
}

// TestScreenshotRatioDropdownFitsPixels keeps a preset inside the capture at nonuniform pixel scales.
func TestScreenshotRatioDropdownFitsPixels(t *testing.T) {
	state := &screenshotEditorOverlayState{
		image: testScreenshotImage(t, 1000, 800), frameSize: Size{Width: 800, Height: 400},
		selection: Rect{X: 700, Y: 350, Width: 80, Height: 40}, hasSelection: true,
		desktopPixelOrigin: Point{X: -1000, Y: -800},
	}
	state.chooseScreenshotRatioLocked(7)
	pixels, err := state.selectionPixelBoundsLocked()
	if err != nil || pixels != image.Rect(875, 700, 931, 800) || state.selectionAspectRatio != 9.0/16 {
		t.Fatalf("edge preset crop=%v ratio=%v err=%v", pixels, state.selectionAspectRatio, err)
	}
}

// TestScreenshotRatioMenuBounds keeps the trigger and list inside offset, mixed-scale display bounds.
func TestScreenshotRatioMenuBounds(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, display := range []Rect{{X: -1200, Y: -800, Width: 1200, Height: 800}, {X: 800, Y: 300, Width: 1200, Height: 900}} {
			for _, chip := range []Rect{
				{X: display.X + 10, Y: display.Y + 10, Width: 100 * scale, Height: 26 * scale},
				{X: display.X + display.Width - 100*scale, Y: display.Y + display.Height - 26*scale, Width: 100 * scale, Height: 26 * scale},
				{X: display.X + 20, Y: display.Y - 32*scale, Width: 100 * scale, Height: 26 * scale},
			} {
				trigger := screenshotRatioTriggerRect(&screenshotTestSurface{}, chip, Rect{}, display, ScreenshotSizeLabels{RatioFree: "Free"}, 0, scale)
				menu := screenshotRatioMenuRect(&screenshotTestSurface{}, trigger, display, ScreenshotSizeLabels{RatioFree: "Free"}, scale)
				for _, rect := range []Rect{trigger, menu} {
					if rect.X < display.X || rect.Y < display.Y || rect.X+rect.Width > display.X+display.Width || rect.Y+rect.Height > display.Y+display.Height {
						t.Fatalf("scale=%v display=%+v: rectangle outside display: %+v", scale, display, rect)
					}
				}
			}
		}
	}
}
