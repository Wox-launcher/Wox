package screenshot

import (
	"fmt"
	"math"
	"testing"

	woxui "wox/ui/runtime"
)

// TestScreenshotSelectionAspectLock covers dialog commit, all handles, bounds, and nonuniform capture scaling.
func TestScreenshotSelectionAspectLock(t *testing.T) {
	for _, frame := range []Size{{Width: 1000, Height: 800}, {Width: 800, Height: 600}} {
		state := &screenshotEditorOverlayState{
			image: testScreenshotImage(t, 1000, 800), frameSize: frame,
			selection: Rect{X: 200, Y: 200, Width: 300, Height: 200}, hasSelection: true,
		}
		if !state.openSizeDialog() {
			t.Fatal("size dialog did not open")
		}
		dialog := state.activeSizeDialog()
		dialog.width.SetText("320", false)
		dialog.height.SetText("160", false)
		dialog.setRatioLocked(true)
		ratio := dialog.aspectRatio
		dialog.apply()
		if ratio != 2 || state.selectionAspectRatio != ratio {
			t.Fatal("the applied selection must use the entered 2:1 ratio")
		}
		original := state.selection
		for handle := screenshotEditorHandleTopLeft; handle <= screenshotEditorHandleLeft; handle++ {
			for _, delta := range []Point{{X: 65, Y: 40}, {X: -65, Y: -40}, {X: 2000, Y: 2000}, {X: -2000, Y: -2000}} {
				state.selection = original
				state.editOriginalRect, state.editHandle, state.editMode = original, handle, screenshotEditorEditResizeSelection
				state.start = Point{}
				state.updateSelectEditLocked(delta, 0)
				got := state.selection
				if got.X < -0.001 || got.Y < -0.001 || got.X+got.Width > frame.Width+0.001 || got.Y+got.Height > frame.Height+0.001 {
					t.Fatalf("frame=%+v handle=%v delta=%+v: outside frame: %+v", frame, handle, delta, got)
				}
				near := func(a, b float32) bool { return math.Abs(float64(a-b)) < 0.001 }
				switch handle {
				case screenshotEditorHandleTop, screenshotEditorHandleBottom:
					if !near(got.X+got.Width/2, original.X+original.Width/2) {
						t.Fatalf("vertical handle moved horizontal center: %+v", got)
					}
				case screenshotEditorHandleLeft, screenshotEditorHandleRight:
					if !near(got.Y+got.Height/2, original.Y+original.Height/2) {
						t.Fatalf("horizontal handle moved vertical center: %+v", got)
					}
				default:
					anchor := screenshotEditorRectHandlePoints(original)[(int(handle)+4)%8]
					if !(near(got.X, anchor.X) || near(got.X+got.Width, anchor.X)) || !(near(got.Y, anchor.Y) || near(got.Y+got.Height, anchor.Y)) {
						t.Fatalf("corner handle moved opposite corner %+v: %+v", anchor, got)
					}
				}
				if got.Width > 0 && got.Height > 0 {
					pixelRatio := float64(got.Width*1000/frame.Width) / float64(got.Height*800/frame.Height)
					if math.Abs(pixelRatio-ratio) > 0.0001 {
						t.Fatalf("frame=%+v handle=%v delta=%+v: ratio=%v want %v", frame, handle, delta, pixelRatio, ratio)
					}
					pixels, err := state.selectionPixelBoundsLocked()
					if err != nil || math.Abs(float64(pixels.Dx())-ratio*float64(pixels.Dy())) > 1+ratio {
						t.Fatalf("pixel rounding lost ratio: crop=%v ratio=%v err=%v", pixels, ratio, err)
					}
				}
			}
		}
		state.selection = original
		state.editMode = screenshotEditorEditNone
		state.openSizeDialog()
		dialog = state.activeSizeDialog()
		if !dialog.lockRatio || dialog.aspectRatio != ratio {
			t.Fatal("reopening the dialog lost the applied lock")
		}
		dialog.setRatioLocked(false)
		state.closeSizeDialog(false)
		if state.selectionAspectRatio != ratio {
			t.Fatal("cancel changed the applied lock")
		}
		state.openSizeDialog()
		dialog = state.activeSizeDialog()
		dialog.setRatioLocked(false)
		dialog.apply()
		state.editOriginalRect, state.editHandle, state.editMode = original, screenshotEditorHandleRight, screenshotEditorEditResizeSelection
		state.updateSelectEditLocked(Point{X: 40}, 0)
		if state.selection.Width != original.Width+40 || state.selection.Height != original.Height {
			t.Fatalf("unlocked resize changed behavior: %+v", state.selection)
		}
		state.selection = original
		state.editMode = screenshotEditorEditNone
		state.openSizeDialog()
		dialog = state.activeSizeDialog()
		dialog.setRatioLocked(true)
		dialog.swapDimensions()
		dialog.apply()
		if math.Abs(state.selectionAspectRatio-1/ratio) > 0.0001 {
			t.Fatal("swapping dimensions did not rotate the applied lock")
		}
		state.beginPointerToolActionLocked(PointerEvent{Position: Point{X: 10, Y: 10}})
		if state.selectionAspectRatio != 0 {
			t.Fatal("a new selection inherited the previous selection's lock")
		}
	}
}

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
