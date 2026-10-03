package screenshot

import (
	"fmt"
	"image"
	"testing"

	woxui "wox/ui/runtime"
)

// TestScreenshotEditorMosaicCursorMatchesPaintedHeight measures actual preview pixels instead of assuming a nominal brush diameter.
func TestScreenshotEditorMosaicCursorMatchesPaintedHeight(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, radius := range screenshotEditorMosaicRadii {
			t.Run(fmt.Sprintf("scale_%g_radius_%g", scale, radius), func(t *testing.T) {
				point := Point{X: 128, Y: 128}
				preview := &DisplayList{}
				drawScreenshotEditorMosaicPreview(preview, []Point{point}, radius, testScreenshotImage(t, int(256*scale), int(256*scale)), Size{Width: 256, Height: 256})
				cursor := &DisplayList{}
				drawScreenshotEditorMosaicCursor(cursor, point, radius, scale)
				var bounds [2]image.Rectangle
				for index, list := range []*DisplayList{preview, cursor} {
					renderer, err := woxui.NewSoftwareRenderer(256, 256)
					if err != nil {
						t.Fatal(err)
					}
					if err := renderer.Render(list); err != nil {
						t.Fatal(err)
					}
					raster := renderer.RGBA()
					for y := 0; y < 256; y++ {
						for x := 0; x < 256; x++ {
							if raster.RGBAAt(x, y).A > 0 {
								bounds[index] = bounds[index].Union(image.Rect(x, y, x+1, y+1))
							}
						}
					}
					if index == 1 && raster.RGBAAt(128, 128).A != 0 {
						t.Fatal("brush cursor must leave its center transparent")
					}
				}
				if bounds[0].Empty() || bounds[0] != bounds[1] {
					t.Fatalf("painted brush bounds = %v, cursor bounds = %v", bounds[0], bounds[1])
				}
			})
		}
	}
}

// TestScreenshotEditorMosaicCursorLifecycle covers tool shortcuts, painting, editing, chrome, and leaving the overlay.
func TestScreenshotEditorMosaicCursorLifecycle(t *testing.T) {
	state := &screenshotEditorOverlayState{
		frameSize: Size{Width: 800, Height: 600}, selection: Rect{X: 20, Y: 20, Width: 700, Height: 500},
		hasSelection: true, mosaicRadius: 18,
		toolbarRect:   Rect{X: 40, Y: 400, Width: 600, Height: 48},
		editBarRect:   Rect{X: 40, Y: 460, Width: 120, Height: 56},
		sizeLabelRect: Rect{X: 30, Y: 30, Width: 80, Height: 28},
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 200, Y: 200}})
	if !state.key(KeyEvent{Key: Key("m"), Down: true}) || state.pointerCursor != PointerCursorHidden {
		t.Fatal("mosaic shortcut must replace the cursor without waiting for a pointer move")
	}
	for _, target := range []struct {
		point Point
		want  PointerCursor
	}{
		{Point{X: 10, Y: 10}, PointerCursorDefault},
		{Point{X: 42, Y: 402}, PointerCursorDefault},
		{Point{X: 42, Y: 462}, PointerCursorDefault},
		{Point{X: 50, Y: 40}, PointerCursorHand},
		{Point{X: 200, Y: 200}, PointerCursorHidden},
	} {
		state.pointer(PointerEvent{Kind: PointerMove, Position: target.point})
		if state.pointerCursor != target.want {
			t.Fatalf("cursor at %+v = %v, want %v", target.point, state.pointerCursor, target.want)
		}
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 200, Y: 200}})
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 250, Y: 200}})
	if !state.annotationDragging || state.pointerCursor != PointerCursorHidden {
		t.Fatal("painting must retain the brush cursor")
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 750, Y: 200}})
	if state.pointerCursor != PointerCursorDefault {
		t.Fatal("dragging outside the selection must restore the native cursor")
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 300, Y: 200}})
	if state.pointerCursor != PointerCursorHidden {
		t.Fatal("re-entering the selection during a stroke must restore the brush cursor")
	}
	state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 300, Y: 200}})
	if len(state.annotations) != 1 || state.pointerCursor != PointerCursorMove {
		t.Fatal("the completed stroke must retain its existing move affordance")
	}
	state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 200, Y: 300}})
	if state.pointerCursor != PointerCursorHidden {
		t.Fatal("an empty canvas target must show the brush again")
	}
	state.key(KeyEvent{Key: Key("n"), Down: true})
	if state.pointerCursor != PointerCursorCrosshair {
		t.Fatal("switching tools must restore the new tool's cursor immediately")
	}
	state.key(KeyEvent{Key: Key("m"), Down: true})
	state.pointer(PointerEvent{Kind: PointerLeave})
	if state.pointerInside || state.pointerCursor != PointerCursorDefault {
		t.Fatal("leaving the overlay must restore the native cursor")
	}
}

// TestScreenshotEditorMosaicCursorUsesChosenSize keeps subsequent strokes consistent with the size selector across display transitions.
func TestScreenshotEditorMosaicCursorUsesChosenSize(t *testing.T) {
	scale := float32(1)
	state := &screenshotEditorOverlayState{
		image:     testScreenshotImage(t, 1600, 1200),
		selection: Rect{X: 100, Y: 100, Width: 900, Height: 400}, hasSelection: true,
		activeTool: screenshotEditorToolMosaic, mosaicRadius: 18, desktopPixelOrigin: Point{X: -1600, Y: -200},
		chromeScale:     func(Rect) float32 { return scale },
		annotations:     []screenshotEditorAnnotation{{tool: screenshotEditorToolMosaic, points: []Point{{X: 200, Y: 200}}, mosaicRadius: 18}},
		hasSelectedMark: true, colorInspectorDismissed: true,
	}
	frame := FrameInfo{Size: Size{Width: 1200, Height: 900}}
	for _, displayScale := range []float32{1, 1.5, 2, 1.25} {
		scale = displayScale
		state.draw(&DisplayList{}, frame)
		for index, radius := range screenshotEditorMosaicRadii {
			button := state.editSizeRects[index]
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: button.X + button.Width/2, Y: button.Y + button.Height/2}})
			state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: 500, Y: 300}})
			if state.mosaicRadius != radius || state.annotations[0].mosaicRadius != radius || state.pointerCursor != PointerCursorHidden {
				t.Fatalf("scale %g: size selection did not update both brush and selected stroke: brush=%g annotation=%g cursor=%v", scale, state.mosaicRadius, state.annotations[0].mosaicRadius, state.pointerCursor)
			}
			withCursor := &DisplayList{}
			state.draw(withCursor, frame)
			state.pointerInside = false
			withoutCursor := &DisplayList{}
			state.draw(withoutCursor, frame)
			if withCursor.CommandCount() != withoutCursor.CommandCount()+2 {
				t.Fatal("the brush outline must appear only while the pointer is inside")
			}
		}
	}
	state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 500, Y: 300}})
	if state.draft == nil || state.draft.mosaicRadius != 28 {
		t.Fatal("the next stroke must use the last chosen brush size")
	}
}
