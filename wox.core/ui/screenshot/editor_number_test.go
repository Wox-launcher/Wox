package screenshot

import (
	"image"
	"image/color"
	"math"
	"testing"

	woxui "wox/ui/runtime"
)

// TestScreenshotEditorNumberSizeSlider covers creation defaults, consecutive markers, and isolated selection edits through the property bar.
func TestScreenshotEditorNumberSizeSlider(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		state := &screenshotEditorOverlayState{
			image: testScreenshotImage(t, 1, 1), frameSize: Size{Width: 2400, Height: 1400},
			selection: Rect{X: 100, Y: 100, Width: 1000, Height: 600}, hasSelection: true,
			activeTool: screenshotEditorToolNumber, textFontSize: 20,
			chromeScale: func(Rect) float32 { return scale },
		}
		draw := func() { state.draw(&DisplayList{}, FrameInfo{Size: state.frameSize}) }
		click := func(point Point) {
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: point})
			state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: point})
			draw()
		}
		draw()
		if state.editFontSizeRect.Width <= 0 || state.fontSizeLocked() != 14 {
			t.Fatal("number creation did not expose its default size slider")
		}
		slider := state.editFontSizeRect
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: slider.X + slider.Width/2, Y: slider.Y + slider.Height/2}})
		if state.pointerCursor != PointerCursorHand {
			t.Fatal("number size slider has no hover cursor")
		}
		click(Point{X: slider.X + slider.Width - 1, Y: slider.Y + slider.Height/2})
		click(Point{X: 250, Y: 250})
		if len(state.annotations) != 1 || state.annotations[0].fontSize != 48 {
			t.Fatalf("scale %v: new number ignored the creation slider: default=%v annotations=%+v slider=%+v", scale, state.numberFontSize, state.annotations, slider)
		}
		slider = state.editFontSizeRect
		grab := Point{X: slider.X + slider.Width/2, Y: slider.Y + slider.Height/2}
		state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: grab})
		state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: slider.X - 50, Y: grab.Y}})
		if !state.fontSizeDragging || state.annotations[0].fontSize != 12 || state.numberFontSize != 12 {
			t.Fatal("number drag did not preview or clamp the size")
		}
		state.pointer(PointerEvent{Kind: PointerMove, Position: grab})
		state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: grab})
		draw()
		click(Point{X: 500, Y: 250})
		if len(state.annotations) != 2 || state.annotations[0].fontSize != 30 || state.annotations[1].fontSize != 30 || state.textFontSize != 20 {
			t.Fatal("consecutive numbers did not share the new size independently of text")
		}
		click(Point{X: 250, Y: 250})
		if state.activeTool != screenshotEditorToolSelect {
			t.Fatal("existing number was not selected for editing")
		}
		slider = state.editFontSizeRect
		click(Point{X: slider.X, Y: slider.Y + slider.Height/2})
		state.key(KeyEvent{Key: KeyArrowRight, Down: true})
		if state.annotations[0].fontSize != 14 {
			t.Fatal("number slider did not respond to the keyboard")
		}
		if err := state.fontSizeAccessibilityAction(woxui.AccessibilityActionSetValue, "24"); err != nil {
			t.Fatal(err)
		}
		if state.annotations[0].fontSize != 24 || state.annotations[1].fontSize != 30 || state.numberFontSize != 30 || state.textFontSize != 20 {
			t.Fatal("selected-number resize changed another marker or a creation preference")
		}
		tool := state.toolRects[screenshotEditorToolNumber]
		click(Point{X: tool.X + tool.Width/2, Y: tool.Y + tool.Height/2})
		click(Point{X: 750, Y: 250})
		if len(state.annotations) != 3 || state.annotations[2].fontSize != 30 || state.annotations[2].number != 3 {
			t.Fatal("returning to numbering lost its size or sequence")
		}
	}
}

// TestScreenshotEditorNumberSizeGeometry preserves proportions and hit bounds across display changes and negative desktop origins.
func TestScreenshotEditorNumberSizeGeometry(t *testing.T) {
	for _, size := range []float32{0, 12, 28, 48} {
		for _, scale := range []float32{1, 2, 1.25, 1.5, 1} {
			annotation := screenshotEditorAnnotation{tool: screenshotEditorToolNumber, start: Point{X: -200, Y: -150}, number: 123, fontSize: size,
				textSize: Size{Width: 15, Height: 17}, measuredSize: 14}
			base := size
			if base == 0 {
				base = 14
			}
			bounds := screenshotEditorNumberBounds(annotation, scale)
			if bounds.Width != 2*base*scale || bounds.Height != bounds.Width || bounds.X+bounds.Width/2 != annotation.start.X {
				t.Fatalf("size %v scale %v: marker bounds = %+v", size, scale, bounds)
			}
			_, fontSize, label := screenshotEditorNumberTextLayout(annotation, scale)
			if math.Abs(float64(fontSize-base*scale*11/14)) > 0.001 || math.Abs(float64(label.X+label.Width/2-annotation.start.X)) > 0.001 || math.Abs(float64(label.Y+label.Height/2-annotation.start.Y)) > 0.001 {
				t.Fatalf("resizing lost the centered three-digit label: size=%v label=%+v", fontSize, label)
			}
			if !screenshotEditorAnnotationContains(annotation, Point{X: annotation.start.X + base*scale - 1, Y: annotation.start.Y}, scale) ||
				screenshotEditorAnnotationContains(annotation, Point{X: annotation.start.X + base*scale + 1, Y: annotation.start.Y}, scale) {
				t.Fatal("number hit testing did not follow the resized badge")
			}
			moved := shiftScreenshotEditorAnnotationWithinBounds(annotation, Point{X: 1000, Y: 1000}, Rect{X: -400, Y: -300, Width: 400, Height: 300}, scale)
			if moved.start != (Point{X: -base * scale, Y: -base * scale}) || moved.fontSize != size {
				t.Fatalf("resized marker escaped movement bounds: %+v", moved)
			}
		}
	}
}

// TestScreenshotEditorNumberSizeRender checks that preview and export resize both the badge and its label at independent capture scales.
func TestScreenshotEditorNumberSizeRender(t *testing.T) {
	frame := Size{Width: 400, Height: 300}
	ink := color.RGBA{R: 255, G: 91, B: 54, A: 255}
	for _, size := range []float32{12, 28, 48} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			annotation := screenshotEditorAnnotation{tool: screenshotEditorToolNumber, start: Point{X: 200, Y: 150}, number: 123, fontSize: size}
			list := &DisplayList{}
			drawScreenshotEditorNumber(list, annotation, scale)
			renderer, err := woxui.NewSoftwareRenderer(400, 300)
			if err != nil {
				t.Fatal(err)
			}
			if err := renderer.Render(list); err != nil {
				t.Fatal(err)
			}
			inside := Point{X: 200, Y: 150 + size*scale*0.8}
			outside := Point{X: 200, Y: 150 + size*scale + 3}
			preview := renderer.RGBA()
			if preview.RGBAAt(int(inside.X), int(inside.Y)) != ink || preview.RGBAAt(int(outside.X), int(outside.Y)).A != 0 {
				t.Fatalf("size %v scale %v: preview badge did not resize", size, scale)
			}
			for _, capture := range []Size{{Width: 400, Height: 300}, {Width: 500, Height: 375}, {Width: 800, Height: 600}, {Width: 600, Height: 454}} {
				source := image.NewRGBA(image.Rect(0, 0, int(capture.Width), int(capture.Height)))
				output, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{annotation}, Rect{X: 50, Y: 25, Width: 300, Height: 250}, frame, scale)
				if err != nil {
					t.Fatal(err)
				}
				a := screenshotEditorScalePoint(inside, capture.Width/frame.Width, capture.Height/frame.Height)
				b := screenshotEditorScalePoint(outside, capture.Width/frame.Width, capture.Height/frame.Height)
				if output.RGBAAt(a.X, a.Y) != ink || output.RGBAAt(b.X, b.Y).A != 0 {
					t.Fatalf("size %v chrome %v capture %+v: exported badge did not resize", size, scale, capture)
				}
				white := image.Rectangle{}
				for y := output.Bounds().Min.Y; y < output.Bounds().Max.Y; y++ {
					for x := output.Bounds().Min.X; x < output.Bounds().Max.X; x++ {
						pixel := output.RGBAAt(x, y)
						if pixel.G > ink.G+20 && pixel.B > ink.B+20 {
							white = white.Union(image.Rect(x, y, x+1, y+1))
						}
					}
				}
				center := screenshotEditorScalePoint(annotation.start, capture.Width/frame.Width, capture.Height/frame.Height)
				if white.Dy() < int(size*scale*capture.Height/frame.Height*0.35) || math.Abs(float64(white.Min.X+white.Max.X-2*center.X)) > 2 || math.Abs(float64(white.Min.Y+white.Max.Y-2*center.Y)) > 2 {
					t.Fatalf("size %v scale %v capture %+v: exported digits did not resize around their center: %+v", size, scale, capture, white)
				}
			}
		}
	}
}
