package screenshot

import "testing"

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
