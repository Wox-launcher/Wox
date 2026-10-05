package screenshot

import (
	"fmt"
	"testing"

	woxui "wox/ui/runtime"
)

// TestScreenshotSelectionBorderDuringDrag checks rendered edges before release in every drag direction.
func TestScreenshotSelectionBorderDuringDrag(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, direction := range []Point{{X: 1, Y: 1}, {X: -1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: -1}} {
			t.Run(fmt.Sprintf("scale=%v/direction=%v", scale, direction), func(t *testing.T) {
				frame := Size{Width: 1000, Height: 800}
				state := newScreenshotEditorOverlayState(ScreenshotOptions{}, testScreenshotImage(t, 1000, 800), screenshotEditorPlatform{
					frameSize: frame, chromeScale: func(Rect) float32 { return scale },
				})
				start := Point{X: 500, Y: 400}
				end := Point{X: start.X + 240*direction.X, Y: start.Y + 160*direction.Y}
				state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: start})
				state.pointer(PointerEvent{Kind: PointerMove, Position: end})
				if !state.dragging {
					t.Fatal("gesture did not enter selection creation")
				}
				renderer, err := woxui.NewSoftwareRenderer(1000, 800)
				if err != nil {
					t.Fatal(err)
				}
				checkBorder := func() {
					t.Helper()
					list := &DisplayList{}
					state.draw(list, FrameInfo{Size: frame})
					if err := renderer.Render(list); err != nil {
						t.Fatal(err)
					}
					pixels := renderer.RGBA()
					rect := normalizeScreenshotEditorRect(state.selection, frame)
					// Sample between handles so they cannot hide a missing border stroke.
					for _, point := range []Point{
						{X: rect.X + rect.Width/4, Y: rect.Y},
						{X: rect.X + rect.Width/4, Y: rect.Y + rect.Height - 1},
						{X: rect.X, Y: rect.Y + rect.Height/4},
						{X: rect.X + rect.Width - 1, Y: rect.Y + rect.Height/4},
					} {
						got := pixels.RGBAAt(int(point.X), int(point.Y))
						if got.R != 41 || got.G != 255 || got.B != 114 || got.A != 255 {
							t.Fatalf("selection edge missing at %+v: %+v", point, got)
						}
					}
				}
				checkBorder()
				state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: end})
				checkBorder()
				rect := state.selection
				grab := Point{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height/2}
				state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: grab})
				if state.editMode != screenshotEditorEditMoveSelection {
					t.Fatal("gesture did not enter selection movement")
				}
				end = Point{X: grab.X + 20, Y: grab.Y + 10}
				state.pointer(PointerEvent{Kind: PointerMove, Position: end})
				checkBorder()
				state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: end})
				rect = state.selection
				grab = Point{X: rect.X + rect.Width, Y: rect.Y + rect.Height}
				state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: grab})
				if state.editMode != screenshotEditorEditResizeSelection {
					t.Fatal("gesture did not enter selection resizing")
				}
				end = Point{X: grab.X + 40, Y: grab.Y + 20}
				state.pointer(PointerEvent{Kind: PointerMove, Position: end})
				checkBorder()
			})
		}
	}
}
