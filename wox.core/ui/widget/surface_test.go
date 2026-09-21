package widget

import (
	"image"
	"image/color"
	"testing"
	woxui "wox/ui/runtime"
)

// TestSurfaceGeometryAndTexture checks negative origins, authored density and bounded size replacement.
func TestSurfaceGeometryAndTexture(t *testing.T) {
	for _, origin := range []float32{-1920, 0, 1280} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			edges := surfaceEdges(origin, 100, 12, 18)
			if (edges[1]-edges[0])*scale != 12*scale || edges[3]-edges[0] != 100 {
				t.Fatal(edges)
			}
		}
	}
	if got := surfaceEdges(0, 10, 12, 18); got != ([4]float32{0, 4, 4, 10}) {
		t.Fatal(got)
	}
	tile := image.NewRGBA(image.Rect(0, 0, 4, 4))
	tile.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	layer := &SurfaceImage{Tile: tile, TileSize: woxui.Size{Width: 2, Height: 2}}
	bounds := woxui.Rect{Width: 6, Height: 4}
	first := layer.tiled(bounds, 2)
	if first.Width != 12 || first.Height != 8 || first.RGBAAt(4, 4).R != 255 {
		t.Fatal("texture density or repetition changed")
	}
	if layer.tiled(bounds, 2) != first {
		t.Fatal("unchanged size recreated image")
	}
	if layer.tiled(woxui.Rect{Width: 8, Height: 4}, 2) == first {
		t.Fatal("resize did not replace cached image")
	}
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Container{Width: 6, Height: 4, Surface: &ImageSurface{Background: layer}, Radius: 1}
	})
	host.AttachServices(&fakeHostServices{})
	for _, scale := range []float32{1, 1.5, 2, 3, 1} {
		var list woxui.DisplayList
		host.Frame(&list, woxui.FrameInfo{Size: woxui.Size{Width: 6, Height: 4}, Scale: scale})
		want := min(scale, 2)
		if layer.cached.Width != int(6*want) || layer.cached.Height != int(4*want) {
			t.Fatalf("scale %v: cache is %dx%d", scale, layer.cached.Width, layer.cached.Height)
		}
		period := int(2 * want)
		if layer.cached.RGBAAt(0, 0) != layer.cached.RGBAAt(period, period) {
			t.Fatal("display-density texture lost its repetition")
		}
	}
}

// TestTransparentFrameSkipsFloatingMaterial keeps alpha corners free of rectangular blur.
func TestTransparentFrameSkipsFloatingMaterial(t *testing.T) {
	surface := &ImageSurface{Frame: &SurfaceImage{}}
	panel := (Container{Width: 100, Height: 100, Floating: true, Surface: surface}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 100})
	var actual, expected woxui.DisplayList
	panel.paint(&actual, panel.bounds)
	surface.Paint(&expected, panel.bounds, 0)
	if !panel.floating || actual.CommandCount() != expected.CommandCount() || len(actual.RenderedFloatingMaterialRects()) != 0 {
		t.Fatal("transparent frame gained a rectangular material or lost floating ordering")
	}
}

// TestRepeatedSliceClipsPartialTiles covers mixed axes, negative origins and bounded work.
func TestRepeatedSliceClipsPartialTiles(t *testing.T) {
	img, err := woxui.NewImage(image.NewRGBA(image.Rect(0, 0, 10, 8)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		x, y  bool
		scale float32
		want  int
	}{
		{false, false, 1, 1}, {true, false, 1, 3}, {false, true, 1, 3}, {true, true, 1, 9}, {true, true, .5, 25}, {true, true, .0001, 1},
	} {
		var list woxui.DisplayList
		paintRepeatedSlice(&list, img, woxui.Rect{X: -1920, Y: -50, Width: 23, Height: 19}, tc.x, tc.y, tc.scale)
		if got := list.ImageDrawCount(); got != tc.want {
			t.Fatalf("%+v: image count %d", tc, got)
		}
	}
}
