package screenshot

import (
	"image/color"
	"testing"
	"wox/common"
	"wox/common/icons"
	woxui "wox/ui/runtime"
)

func TestScreenshotToolbarOutlinesHoveredButtons(t *testing.T) {
	frame := FrameInfo{Size: Size{Width: 1400, Height: 500}}
	for _, scale := range []float32{1, 1.5} {
		state := newScreenshotEditorOverlayState(ScreenshotOptions{
			ExtraActions: []common.ScreenshotExtraAction{{ID: "ai", Icon: icons.ScreenshotAIChat}},
		}, testScreenshotImage(t, 1, 1), screenshotEditorPlatform{})
		state.selection = Rect{X: 50, Y: 50, Width: 1280, Height: 300}
		state.hasSelection = true
		state.colorInspectorDismissed = true
		state.chromeScale = func(Rect) float32 { return scale }
		state.draw(&DisplayList{}, frame)

		renderer, err := woxui.NewSoftwareRenderer(int(frame.Size.Width), int(frame.Size.Height))
		if err != nil {
			t.Fatal(err)
		}
		checkOutline := func(name string, rect Rect, want bool) {
			t.Helper()
			list := &DisplayList{}
			state.draw(list, frame)
			if err := renderer.Render(list); err != nil {
				t.Fatal(err)
			}
			pixel := renderer.RGBA().RGBAAt(int(rect.X+rect.Width/2), int(rect.Y))
			if outlined := pixel == (color.RGBA{R: 41, G: 255, B: 114, A: 255}); outlined != want {
				t.Fatalf("scale %g: %s outline = %t (%v), want %t", scale, name, outlined, pixel, want)
			}
		}
		for _, target := range []struct {
			name    string
			rect    Rect
			outline bool
		}{
			{"annotation tool", state.toolRects[screenshotEditorToolRect], true},
			{"disabled undo", state.undoRect, false},
			{"background", state.backgroundRect, true},
			{"confirm", state.confirmRect, true},
			{"AI", state.extraActionRects[0], true},
		} {
			state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: target.rect.X + target.rect.Width/2, Y: target.rect.Y + target.rect.Height/2}})
			checkOutline(target.name, target.rect, target.outline)
		}
		state.pointer(PointerEvent{Kind: PointerLeave})
		checkOutline("AI after pointer leave", state.extraActionRects[0], false)
	}
}

// TestScreenshotToolbarStaysOnDisplay covers full-height selections on offset, scaled displays.
func TestScreenshotToolbarStaysOnDisplay(t *testing.T) {
	displays := []Rect{
		{X: -1600, Y: 1120, Width: 2560, Height: 2760},
		{X: 960, Y: 0, Width: 1920, Height: 1200},
	}
	desktop := Rect{X: -1600, Width: 4480, Height: 3880}
	for _, scale := range []float32{1, 1.5, 2.5} {
		for _, display := range displays {
			selection := Rect{X: display.X + 100, Y: display.Y, Width: display.Width - 200, Height: display.Height}
			frame := screenshotEditorToolbarFrame(selection, desktop, displays)
			if frame != display {
				t.Fatalf("frame = %+v, want %+v", frame, display)
			}
			size, _ := screenshotEditorToolbarLayout(frame.Width, scale, false, true, true, true, 0)
			_, editHeight, _ := screenshotEditorEditBarSize(frame.Width, screenshotEditorToolText, false, scale)
			toolbar := screenshotEditorToolbarPlacement(selection, frame, size.Width, size.Height, size.Height+8*scale+editHeight, scale)
			if toolbar.Y < selection.Y || toolbar.Y+toolbar.Height > selection.Y+selection.Height {
				t.Fatalf("scale=%v: toolbar outside full-height selection: %+v", scale, toolbar)
			}
			state := &screenshotEditorOverlayState{}
			state.drawEditBar(&DisplayList{}, frame, toolbar, selection, screenshotEditorToolText, nil, screenshotEditorAnnotationColor, 18, 20, scale)
			for _, rect := range []Rect{toolbar, state.editBarRect} {
				if rect.X < frame.X || rect.Y < frame.Y || rect.X+rect.Width > frame.X+frame.Width || rect.Y+rect.Height > frame.Y+frame.Height {
					t.Fatalf("scale=%v: chrome %+v outside display %+v", scale, rect, frame)
				}
			}
		}
	}
	if got := screenshotEditorToolbarFrame(Rect{}, desktop, nil); got != desktop {
		t.Fatalf("missing display fallback = %+v, want %+v", got, desktop)
	}
}

// TestScreenshotToolbarDrawAvoidsMonitorGap checks the actual editor path on a staggered desktop.
func TestScreenshotToolbarDrawAvoidsMonitorGap(t *testing.T) {
	display := Rect{Y: 1120, Width: 2560, Height: 2760}
	state := &screenshotEditorOverlayState{
		image:             testScreenshotImage(t, 1, 1),
		selection:         Rect{X: 200, Y: 1120, Width: 2096, Height: 2760},
		hasSelection:      true,
		displayCandidates: []Rect{display, {X: 2560, Width: 1920, Height: 1200}},
		chromeScale:       func(Rect) float32 { return 1.5 },
		activeTool:        screenshotEditorToolText,
	}
	state.draw(&DisplayList{}, FrameInfo{Size: Size{Width: 4480, Height: 3880}})
	for _, rect := range []Rect{state.toolbarRect, state.editBarRect} {
		if rect.Width <= 0 || rect.Y < display.Y || rect.Y+rect.Height > display.Y+display.Height {
			t.Fatalf("chrome placed in monitor gap: %+v", rect)
		}
	}
}
