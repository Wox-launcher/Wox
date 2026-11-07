//go:build windows || darwin || linux

package window

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"testing"
)

type transformTestRenderer interface {
	render(*DisplayList, float32) error
}

// TestNativeRecordedTransforms exercises the public transform/clip stacks through native encoders.
func TestNativeRecordedTransforms(t *testing.T) {
	_, draw := nativeTransformTarget(t, 160, 120)
	for _, scale := range []float32{1, 1.25, 1.5, 2, 1} {
		d := &DisplayList{}
		recordedTransformScene(d)
		pixels := draw(d, scale)
		for _, p := range recordedTransformSamples {
			checkNativeTransformPixel(t, pixels, scale, p.x, p.y, p.color)
		}
	}
}

// checkNativeTransformPixel tolerates native alpha rounding, not geometry or channel changes.
func checkNativeTransformPixel(t *testing.T, pixels *image.RGBA, scale, x, y float32, want color.RGBA) {
	t.Helper()
	got := pixels.RGBAAt(int(x*scale), int(y*scale))
	for i, v := range []byte{got.R, got.G, got.B, got.A} {
		if delta := int(v) - int([]byte{want.R, want.G, want.B, want.A}[i]); delta < -1 || delta > 1 {
			t.Fatalf("scale %g pixel %g,%g: %+v want %+v", scale, x, y, got, want)
		}
	}
}

// TestNativeTransformClipAndOpacity exercises matrices and exact clips through the actual Go encoder.
func TestNativeTransformClipAndOpacity(t *testing.T) {
	_, draw := nativeTransformTarget(t, 128, 128)
	for _, scale := range []float32{1, 1.25, 1.5, 2, 1} {
		d := &DisplayList{}
		d.Clear(Color{B: 255, A: 255})
		d.PushClipRect(Rect{X: -10, Y: -10, Width: 20, Height: 20})
		d.BeginOpacity(Rect{X: -30, Y: -30, Width: 60, Height: 60}, 0.5)
		d.FillRect(Rect{X: -30, Y: -30, Width: 60, Height: 60}, Color{R: 255, A: 255})
		d.FillRect(Rect{Width: 30, Height: 30}, Color{G: 255, A: 255})
		d.EndOpacity(Rect{X: -30, Y: -30, Width: 60, Height: 60}, 0.5)
		d.PopClipRect()
		transformRecordedScene(d, RotationTransform(math.Pi/4).Then(TranslationTransform(30, 30)))
		d.FillRect(Rect{X: 50, Y: 50, Width: 8, Height: 8}, Color{R: 255, G: 255, A: 255})
		pixels := draw(d, scale)
		checkNativeTransformPixel(t, pixels, scale, 18, 18, color.RGBA{B: 255, A: 255})
		checkNativeTransformPixel(t, pixels, scale, 30, 23, color.RGBA{R: 128, B: 127, A: 255})
		checkNativeTransformPixel(t, pixels, scale, 30, 37, color.RGBA{G: 128, B: 127, A: 255})
		checkNativeTransformPixel(t, pixels, scale, 54, 54, color.RGBA{R: 255, G: 255, A: 255})
	}
}

// TestNativeTransformLayerBounds verifies the local composite rectangle rather than its rotated envelope.
func TestNativeTransformLayerBounds(t *testing.T) {
	_, draw := nativeTransformTarget(t, 128, 128)
	for _, fade := range []bool{false, true} {
		t.Run(fmt.Sprint(fade), func(t *testing.T) {
			d := &DisplayList{}
			d.Clear(Color{B: 255, A: 255})
			bounds := Rect{X: -10, Y: -10, Width: 20, Height: 20}
			if fade {
				d.BeginEdgeFade(bounds, 10, 0)
			} else {
				d.BeginOpacity(bounds, 0.5)
			}
			d.FillRect(Rect{Width: 64, Height: 64}, Color{R: 255, A: 255})
			if fade {
				d.EndEdgeFade(bounds, 10, 0)
			} else {
				d.EndOpacity(bounds, 0.5)
			}
			matrix := RotationTransform(math.Pi / 4).Then(TranslationTransform(30, 30))
			for i := range d.commands {
				if d.commands[i].kind != displayCommandFillRoundedRect {
					d.commands[i].transform, d.commands[i].transformed = matrix, true
				}
			}
			pixels := draw(d, 2)
			checkNativeTransformPixel(t, pixels, 2, 18, 18, color.RGBA{B: 255, A: 255})
			if !fade {
				checkNativeTransformPixel(t, pixels, 2, 30, 30, color.RGBA{R: 128, B: 127, A: 255})
			}
		})
	}
}

// TestNativeTransformDamageAndReset compares retained replay to a full native frame and rejects invalid matrices.
func TestNativeTransformDamageAndReset(t *testing.T) {
	r, draw := nativeTransformTarget(t, 128, 128)
	build := func(x float32) *DisplayList {
		d := &DisplayList{}
		d.Clear(Color{B: 100, A: 255})
		d.FillRoundedRect(Rect{X: -8, Y: -8, Width: 16, Height: 16}, 3, Color{R: 255, A: 255})
		transformRecordedScene(d, ScaleTransform(1.5, 0.75).Then(RotationTransform(0.4)).Then(TranslationTransform(x, 30)))
		return d
	}
	draw(build(25), 1.5)
	partial := build(40)
	partial.SetNativeDamage(Rect{X: 5, Y: 10, Width: 55, Height: 40})
	got := draw(partial, 1.5)
	want := draw(build(40), 1.5)
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("partial native frame differs from full replay")
	}
	invalid := build(10)
	invalid.commands[0].transform.TX = float32(math.NaN())
	if err := r.render(invalid, 1); err == nil {
		t.Fatal("accepted nonfinite transform")
	}
	d := &DisplayList{}
	d.FillRect(Rect{Width: 8, Height: 8}, Color{G: 255, A: 255})
	checkNativeTransformPixel(t, draw(d, 1), 1, 2, 2, color.RGBA{G: 255, A: 255})
}

// TestNativeTransformImageAndText checks native bitmap orientation and real native glyph pixels.
func TestNativeTransformImageAndText(t *testing.T) {
	_, draw := nativeTransformTarget(t, 128, 128)
	source := image.NewRGBA(image.Rect(0, 0, 2, 1))
	source.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	source.SetRGBA(1, 0, color.RGBA{G: 255, A: 255})
	raster, err := NewImage(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, scale := range []float32{1, 1.5, 2} {
		d := &DisplayList{}
		d.DrawImage(raster, Rect{Width: 20, Height: 10})
		d.DrawText("M", Rect{X: 25, Width: 15, Height: 16}, TextStyle{Size: 14}, Color{B: 255, A: 255})
		transformRecordedScene(d, RotationTransform(math.Pi/2).Then(TranslationTransform(30, 10)))
		pixels := draw(d, scale)
		checkNativeTransformPixel(t, pixels, scale, 25, 12, color.RGBA{R: 255, A: 255})
		checkNativeTransformPixel(t, pixels, scale, 25, 28, color.RGBA{G: 255, A: 255})
		blue := 0
		for y := 0; y < 128; y++ {
			for x := 0; x < 128; x++ {
				if pixels.RGBAAt(x, y).B < 16 {
					continue
				}
				blue++
				if float32(x) < 14*scale || float32(x) >= 31*scale || float32(y) < 35*scale || float32(y) >= 51*scale {
					t.Fatalf("native glyph escaped rotated bounds at %d,%d, scale %g", x, y, scale)
				}
			}
		}
		if blue < 10 {
			t.Fatalf("missing native glyph pixels: %d", blue)
		}
	}
}

// TestNativeTransformMaterial checks rotated sampling, exact replacement, alpha preservation and later matrix reset.
func TestNativeTransformMaterial(t *testing.T) {
	_, draw := nativeTransformTarget(t, 192, 192)
	for _, scale := range []float32{1, 1.5, 2} {
		for _, alpha := range []uint8{128, 255} {
			// A single image provides a uniformly translucent backdrop on both sides.
			// A sharp edge makes blur observable. Magnifying a two-pixel image creates
			// a linear gradient that a correct Gaussian may preserve exactly.
			backdrop := image.NewRGBA(image.Rect(0, 0, 96, 1))
			for x := 0; x < 96; x++ {
				pixel := color.RGBA{A: alpha}
				if x >= 48 {
					pixel.R, pixel.G, pixel.B = alpha, alpha, alpha
				}
				backdrop.SetRGBA(x, 0, pixel)
			}
			bitmap, err := NewImage(backdrop)
			if err != nil {
				t.Fatal(err)
			}
			d := &DisplayList{}
			d.DrawImage(bitmap, Rect{Width: 96, Height: 96})
			d.commands = append(d.commands, displayCommand{
				kind: displayCommandFloatingMaterial, rect: Rect{X: -20, Y: -20, Width: 40, Height: 40}, radius: 4,
				material:    FloatingMaterialStyle{Sigma: 4, Brightness: 1, Saturation: 1},
				transformed: true, transform: RotationTransform(math.Pi / 4).Then(TranslationTransform(48, 48)),
			})
			pixels := draw(d, scale)
			// Outside the diamond but inside its envelope, background must survive untouched.
			d.commands = d.commands[:1]
			original := draw(d, scale)
			for _, p := range []Point{{X: 24, Y: 24}, {X: 72, Y: 24}, {X: 48, Y: 10}} {
				want := original.RGBAAt(int(p.X*scale), int(p.Y*scale))
				checkNativeTransformPixel(t, pixels, scale, p.X, p.Y, want)
			}
			for _, x := range []float32{45, 51} {
				got := pixels.RGBAAt(int(x*scale), int(48*scale))
				if got.A < alpha-1 || got.A > alpha && alpha != 255 {
					t.Fatalf("material changed alpha at scale %g: %+v want %d", scale, got, alpha)
				}
				if got.R == original.RGBAAt(int(x*scale), int(48*scale)).R {
					t.Fatalf("material did not replace backdrop at scale %g: %+v", scale, got)
				}
			}
		}
	}
}

// TestNativeTransformSingularAndClipReplacement keeps collapsed geometry empty and clears exact clips.
func TestNativeTransformSingularAndClipReplacement(t *testing.T) {
	_, draw := nativeTransformTarget(t, 96, 96)
	for _, matrix := range []AffineTransform{{}, ScaleTransform(0, 1), {A: 1, B: 1, C: 1, D: 1}} {
		d := &DisplayList{}
		d.FillRect(Rect{Width: 60, Height: 60}, Color{R: 255, A: 255})
		transformRecordedScene(d, matrix)
		pixels := draw(d, 1)
		for _, v := range pixels.Pix {
			if v != 0 {
				t.Fatal("singular transform painted pixels")
			}
		}
	}
	d := &DisplayList{}
	points := make([]Point, 64)
	for i := range points {
		angle := float64(i) * 2 * math.Pi / float64(len(points))
		points[i] = Point{X: 30 + 10*float32(math.Cos(angle)), Y: 30 + 10*float32(math.Sin(angle))}
	}
	d.commands = append(d.commands, displayCommand{kind: displayCommandSetClipRect, rect: Rect{X: 20, Y: 20, Width: 20, Height: 20}, points: points})
	d.FillRect(Rect{Width: 96, Height: 96}, Color{R: 255, A: 255})
	d.PushClipRect(Rect{X: 50, Y: 50, Width: 10, Height: 10})
	d.FillRect(Rect{Width: 96, Height: 96}, Color{G: 255, A: 255})
	d.PopClipRect()
	d.FillRect(Rect{Width: 10, Height: 10}, Color{B: 255, A: 255})
	pixels := draw(d, 1)
	checkNativeTransformPixel(t, pixels, 1, 30, 30, color.RGBA{R: 255, A: 255})
	checkNativeTransformPixel(t, pixels, 1, 21, 21, color.RGBA{})
	checkNativeTransformPixel(t, pixels, 1, 55, 55, color.RGBA{G: 255, A: 255})
	checkNativeTransformPixel(t, pixels, 1, 5, 5, color.RGBA{B: 255, A: 255})
}

// TestNativeTransformMaterialInsideClip prevents exact-clip layers from silently suppressing the blur graph.
func TestNativeTransformMaterialInsideClip(t *testing.T) {
	_, draw := nativeTransformTarget(t, 96, 96)
	for _, nested := range []bool{false, true} {
		d := &DisplayList{}
		d.Clear(Color{A: 255})
		d.FillRect(Rect{X: 48, Width: 48, Height: 96}, Color{R: 255, G: 255, B: 255, A: 255})
		bounds := Rect{X: 10, Y: 10, Width: 76, Height: 76}
		if nested {
			d.BeginOpacity(bounds, 0.5)
			d.FillRect(Rect{X: 48, Width: 48, Height: 96}, Color{G: 255, A: 255})
		}
		d.commands = append(d.commands, displayCommand{kind: displayCommandSetClipRect, rect: bounds, points: []Point{{X: 48, Y: 10}, {X: 86, Y: 48}, {X: 48, Y: 86}, {X: 10, Y: 48}}})
		d.commands = append(d.commands, displayCommand{
			kind: displayCommandFloatingMaterial, rect: Rect{X: -20, Y: -20, Width: 40, Height: 40}, radius: 4,
			material:    FloatingMaterialStyle{Sigma: 4, Brightness: 1, Saturation: 1},
			transformed: true, transform: RotationTransform(math.Pi / 4).Then(TranslationTransform(48, 48)),
		})
		if nested {
			d.EndOpacity(bounds, 0.5)
		}
		pixels := draw(d, 1)
		left, right := pixels.RGBAAt(45, 48), pixels.RGBAAt(51, 48)
		if !nested && (left.R == 0 || right.R == 255) || nested && (left.G == 0 || right.G <= right.R) {
			t.Fatalf("nested %t: material graph skipped under exact clip: left %+v right %+v", nested, left, right)
		}
	}
}

// TestNativeBackdropMaterialAlphaAndDamage checks nested replacement on translucent glass and retained targets.
func TestNativeBackdropMaterialAlphaAndDamage(t *testing.T) {
	r, draw := nativeTransformTarget(t, 144, 144)
	build := func(x float32) *DisplayList {
		d := &DisplayList{}
		d.Clear(Color{R: 40, G: 80, B: 120, A: 128})
		bounds := Rect{X: -20, Y: -20, Width: 40, Height: 40}
		d.BeginOpacity(bounds, 0.5)
		d.BeginEdgeFade(bounds, 10, 0)
		d.PushClipRect(bounds)
		d.commands = append(d.commands, displayCommand{kind: displayCommandFloatingMaterial, rect: bounds, radius: 4, material: FloatingMaterialStyle{Sigma: 4, Brightness: 1, Saturation: 1}})
		d.PopClipRect()
		d.EndEdgeFade(bounds, 10, 0)
		d.EndOpacity(bounds, 0.5)
		transformRecordedScene(d, RotationTransform(0.3).Then(TranslationTransform(x, 48)))
		return d
	}
	pixels := draw(build(40), 1.5)
	for _, p := range []Point{{X: 40, Y: 48}, {X: 40, Y: 35}, {X: 4, Y: 4}} {
		got := pixels.RGBAAt(int(p.X*1.5), int(p.Y*1.5))
		if got.A < 127 || got.A > 129 {
			t.Fatalf("nested material duplicated backdrop alpha at %+v: %+v", p, got)
		}
	}
	partial := build(50)
	partial.SetNativeDamage(Rect{X: 5, Y: 10, Width: 85, Height: 75})
	got := draw(partial, 1.5)
	want := draw(build(50), 1.5)
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("nested material partial frame differs from complete replay")
	}
	invalid := build(40)
	invalid.commands[3].transform.TX = float32(math.Inf(1))
	if err := r.render(invalid, 1.5); err == nil {
		t.Fatal("accepted invalid transform inside readable groups")
	}
	got = draw(build(50), 1.5)
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("failed frame left composite targets or clips active")
	}
}

// TestNativeBackdropGroupMatchesNativeComposition verifies readable targets preserve ordinary group semantics.
func TestNativeBackdropGroupMatchesNativeComposition(t *testing.T) {
	_, draw := nativeTransformTarget(t, 128, 128)
	transform := RotationTransform(0.3).Then(TranslationTransform(32, 32))
	inverse, _ := transform.Inverse()
	for _, scale := range []float32{1, 1.5, 2} {
		for _, opacity := range []float32{0, 0.5, 1} {
			for _, fade := range []bool{false, true} {
				build := func(readable bool) *DisplayList {
					d := &DisplayList{}
					d.Clear(Color{B: 255, A: 128})
					bounds := Rect{X: -20, Y: -20, Width: 40, Height: 40}
					d.BeginOpacity(bounds, opacity)
					d.FillRect(Rect{X: -20, Y: -20, Width: 30, Height: 40}, Color{R: 255, A: 255})
					if fade {
						d.BeginEdgeFade(bounds, 10, 10)
					} else {
						d.BeginOpacity(bounds, 0.5)
					}
					d.FillRect(Rect{X: -10, Y: -20, Width: 30, Height: 40}, Color{G: 255, A: 255})
					if readable {
						// A transparent no-op material requests readable targets without changing the expected pixels.
						d.commands = append(d.commands, displayCommand{kind: displayCommandFloatingMaterial, rect: bounds, material: FloatingMaterialStyle{Brightness: 1, Saturation: 1}})
					}
					if fade {
						d.EndEdgeFade(bounds, 10, 10)
					} else {
						d.EndOpacity(bounds, 0.5)
					}
					d.EndOpacity(bounds, opacity)
					transformRecordedScene(d, transform)
					return d
				}
				want := draw(build(false), scale)
				got := draw(build(true), scale)
				for i := 0; i < len(got.Pix); i += 4 {
					x, y := i/4%128, i/4/128
					if runtime.GOOS == "darwin" {
						local := inverse.Point(Point{X: (float32(x) + 0.5) / scale, Y: (float32(y) + 0.5) / scale})
						// CoreGraphics antialiases a transparency layer and direct bitmap draws differently at rotated edges.
						nearEdge := false
						for _, edge := range []float32{-20, -10, 10, 20} {
							if math.Abs(float64((local.X-edge)*scale)) <= 1.5 || math.Abs(float64((local.Y-edge)*scale)) <= 1.5 {
								nearEdge = true
								break
							}
						}
						if nearEdge {
							continue
						}
					}
					for channel := range 4 {
						value := got.Pix[i+channel]
						if delta := int(value) - int(want.Pix[i+channel]); delta < -2 || delta > 2 {
							t.Fatalf("scale %g opacity %g fade %t pixel %d,%d channel %d: got %d want %d", scale, opacity, fade, x, y, channel, value, want.Pix[i+channel])
						}
					}
				}
			}
		}
	}
}
