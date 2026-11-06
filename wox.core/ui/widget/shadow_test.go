package widget

import (
	"bytes"
	"fmt"
	"math"
	"sync"
	"testing"

	ui "wox/ui/runtime"
)

// TestShadowPixels checks the silhouette knockout, directional falloff, opacity,
// clipping and slice joins in physical pixels as the display scale changes.
func TestShadowPixels(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2, 1} {
		for _, alpha := range []float32{1, 0.5} {
			shadow := BoxShadow{Sigma: 4, Radius: 8, Offset: ui.Point{Y: 2}, Color: ui.Color{A: 200}, Child: Container{Width: 100, Height: 60}}
			n := (Opacity{Value: alpha, Child: shadow}).layout(context{}, constraints{width: 160, height: 120})
			if n.bounds.Width != 100 || n.bounds.Height != 60 {
				t.Fatal("shadow changed layout")
			}
			d := &ui.DisplayList{RasterScale: scale}
			n.drawAt(d, ui.Point{X: 30, Y: 30}, 0, 0, false, false, false, nil)
			d.TransformScene(scale, ui.Point{})
			r, _ := ui.NewSoftwareRenderer(int(160*scale), int(120*scale))
			if err := r.Render(d); err != nil {
				t.Fatal(err)
			}
			img := r.RGBA()
			at := func(x, y float32) uint8 { return img.RGBAAt(int(x*scale), int(y*scale)).A }
			if at(80, 50) != 0 || at(36, 50) != 0 {
				t.Fatal("shadow tinted the transparent child")
			}
			near, far := at(80, 92), at(80, 99)
			if near < uint8(50*alpha) || far >= near || at(80, 27) >= near {
				t.Fatalf("scale %v opacity %v: falloff %d/%d/top %d", scale, alpha, near, far, at(80, 27))
			}
			if at(30, 30) == 0 {
				t.Fatal("rounded corner lost its outer shadow")
			}
			// The middle of a long edge must remain flat across both slice seams.
			for x := float32(50); x < 110; x += 1 / scale {
				if math.Abs(float64(at(x, 92))-float64(near)) > 1 {
					t.Fatalf("slice seam at %v scale %v", x, scale)
				}
			}
			clipped := (Clip{SizeToChild: true, Child: shadow}).layout(context{}, constraints{width: 160, height: 120})
			d = &ui.DisplayList{RasterScale: scale}
			clipped.drawAt(d, ui.Point{X: 30, Y: 30}, 0, 0, false, false, false, nil)
			d.TransformScene(scale, ui.Point{})
			if err := r.Render(d); err != nil {
				t.Fatal(err)
			}
			if r.RGBA().RGBAAt(int(80*scale), int(92*scale)).A != 0 {
				t.Fatal("ancestor clip did not contain shadow")
			}
		}
	}
}

type shadowBoundaryProps struct{ width float32 }

func (p shadowBoundaryProps) Equal(other shadowBoundaryProps) bool { return p == other }

// TestShadowKeyInvalidation clears the old halo even without a cached Boundary.
func TestShadowKeyInvalidation(t *testing.T) {
	visible := true
	host := NewHost(func(ui.FrameInfo) Widget {
		var child Widget = Container{Width: 50, Height: 30}
		if visible {
			child = BoxShadow{Sigma: 8, Color: ui.Color{A: 200}, Child: child}
		}
		return Stack{Width: 140, Height: 120, Children: []StackChild{{Left: 40, Top: 40, Child: Gesture{ID: "panel", Child: child}}}}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)
	defer host.Dispose()
	r, _ := ui.NewSoftwareRenderer(140, 120)
	frame := ui.FrameInfo{Size: ui.Size{Width: 140, Height: 120}, Scale: 1}
	d := &ui.DisplayList{}
	host.Frame(d, frame)
	if err := r.Render(d); err != nil {
		t.Fatal(err)
	}
	if r.RGBA().RGBAAt(65, 75).A == 0 {
		t.Fatal("missing initial shadow")
	}
	visible = false
	if !host.InvalidateKey("panel") {
		t.Fatal("key not found")
	}
	if services.invalidatedRect != (ui.Rect{X: 16, Y: 16, Width: 98, Height: 78}) {
		t.Fatalf("missing old halo: %+v", services.invalidatedRect)
	}
	if bounds, _ := host.BoundsForKey("panel"); bounds != (ui.Rect{X: 40, Y: 40, Width: 50, Height: 30}) {
		t.Fatal("public layout bounds include shadow")
	}
	frame.Damage = services.invalidatedRect
	d = &ui.DisplayList{}
	host.Frame(d, frame)
	if err := r.Render(d); err != nil {
		t.Fatal(err)
	}
	if r.RGBA().RGBAAt(65, 75).A != 0 {
		t.Fatal("key invalidation left stale shadow")
	}
}

// TestShadowBoundaryDamage compares retained partial frames with full rendering
// when a cached panel moves, shrinks, leaves the viewport, and is removed.
func TestShadowBoundaryDamage(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			x, width, present := float32(30), float32(70), true
			host := NewHost(func(ui.FrameInfo) Widget {
				children := []StackChild{{Child: Container{Width: 240, Height: 160, Color: ui.Color{R: 210, G: 210, B: 210, A: 255}}}}
				if present {
					children = append(children, StackChild{Left: x, Top: 40, Child: Boundary[shadowBoundaryProps]{Key: "panel", Props: shadowBoundaryProps{width}, Build: func(p shadowBoundaryProps) Widget {
						return Opacity{Value: 0.6, Child: BoxShadow{Sigma: 8, Radius: 6, Offset: ui.Point{Y: 4}, Color: ui.Color{A: 200}, Child: Container{Width: p.width, Height: 45, Color: ui.Color{R: 255, A: 255}}}}
					}}})
				}
				return Stack{Width: 240, Height: 160, Children: children}
			})
			host.AttachServices(&fakeHostServices{})
			defer host.Dispose()
			retained, _ := ui.NewSoftwareRenderer(int(240*scale), int(160*scale))
			for step := 0; step < 5; step++ {
				switch step {
				case 1:
					x = 115
				case 2:
					width = 25
				case 3:
					x = -10
				case 4:
					present = false
				}
				d := &ui.DisplayList{}
				frame := ui.FrameInfo{Size: ui.Size{Width: 240, Height: 160}, PixelSize: ui.PixelSize{Width: int(240 * scale), Height: int(160 * scale)}, Scale: scale}
				if step > 0 {
					frame.Damage = ui.Rect{X: 1, Y: 1, Width: 1, Height: 1}
				}
				host.Frame(d, frame)
				d.TransformScene(scale, ui.Point{})
				if err := retained.Render(d); err != nil {
					t.Fatal(err)
				}
				d.SetDamage(ui.Rect{})
				full, _ := ui.NewSoftwareRenderer(int(240*scale), int(160*scale))
				if err := full.Render(d); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(retained.RGBA().Pix, full.RGBA().Pix) {
					t.Fatalf("step %d left shadow damage", step)
				}
			}
		})
	}
}

// TestShadowBoundsAndCache checks paint-only geometry and bounded concurrent raster reuse.
func TestShadowBoundsAndCache(t *testing.T) {
	shadow := BoxShadow{Sigma: 4, Offset: ui.Point{X: -2, Y: 3}, Color: ui.Color{A: 64}, Child: Container{Width: 80, Height: 40}}
	n := shadow.layout(context{}, constraints{width: 200, height: 200})
	n.place(-20, 10)
	prepareNodeTree(n)
	want := ui.Rect{X: -34, Y: 1, Width: 104, Height: 64}
	if got := globalPaintBounds(n); got != want {
		t.Fatalf("paint bounds %+v, want %+v", got, want)
	}
	if globalRect(n) != (ui.Rect{X: -20, Y: 10, Width: 80, Height: 40}) {
		t.Fatal("paint outsets changed hit bounds")
	}
	cache := &shadowCache{}
	key := shadowKey{width: 60, height: 40, sigma: 4, radius: 6, scale: 1.5, color: ui.Color{A: 100}}
	results := make(chan *shadowTexture, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- cache.get(key)
		}()
	}
	wg.Wait()
	close(results)
	first := cache.get(key)
	for result := range results {
		if result != first {
			t.Fatal("concurrent misses retained duplicate textures")
		}
	}
	for i := 0; i < 100; i++ {
		key.radius = float32(i)
		cache.get(key)
	}
	if len(cache.entries) > 64 || cache.bytes > shadowCacheBytes {
		t.Fatal("unbounded shadow cache")
	}
	key.width, key.height, key.scale = 3000, 3000, 4
	if got := cache.get(key); got.bytes > shadowRasterLimit*shadowRasterLimit*4 {
		t.Fatal("unbounded texture")
	}
}

// TestShadowResizingReusesEdgeTextures prevents panel size animations from
// allocating a different full-sized raster on every frame.
func TestShadowResizingReusesEdgeTextures(t *testing.T) {
	color := ui.Color{R: 67, G: 83, B: 97, A: 121}
	for _, size := range []float32{200, 300, 500} {
		n := (BoxShadow{Sigma: 4, Radius: 8, Color: color, Child: Container{Width: size, Height: size}}).layout(context{}, constraints{width: 600, height: 600})
		n.draw(&ui.DisplayList{RasterScale: 1.5}, 0, 0, false, false, false, nil)
	}
	boxShadowCache.mu.Lock()
	defer boxShadowCache.mu.Unlock()
	matches := 0
	for _, entry := range boxShadowCache.entries {
		if entry.key.color == color {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("resizing retained %d shadow rasters", matches)
	}
}
