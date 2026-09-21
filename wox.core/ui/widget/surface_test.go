package widget

import (
	"image"
	"image/color"
	"testing"
	woxui "wox/ui/runtime"
)

// TestSurfaceInnerShadow stays above child fills and fades without shading the exterior.
func TestSurfaceInnerShadow(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		surface := &ImageSurface{InnerShadow: &SurfaceInnerShadow{Color: woxui.Color{A: 80}, Width: 6, Radius: 8, Insets: Insets{Top: 10, Right: 10, Bottom: 10, Left: 10}}}
		host := NewHost(func(woxui.FrameInfo) Widget {
			return Container{Width: 100, Height: 80, Surface: surface, Child: Container{Width: 100, Height: 80, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255}}}
		})
		host.AttachServices(&fakeHostServices{})
		var list woxui.DisplayList
		host.Frame(&list, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 80}, Scale: scale})
		renderer, _ := woxui.NewSoftwareRenderer(100, 80)
		if err := renderer.Render(&list); err != nil {
			t.Fatal(err)
		}
		img := renderer.RGBA()
		if img.RGBAAt(50, 10).R >= img.RGBAAt(50, 13).R || img.RGBAAt(50, 13).R >= 255 || img.RGBAAt(50, 25).R != 255 || img.RGBAAt(50, 9).R != 255 || img.RGBAAt(10, 10).R != 255 {
			t.Fatalf("scale %v: inner shadow lost its fade, rounded corner or paint order", scale)
		}
	}
}

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
		paintRepeatedSlice(&list, img, woxui.Rect{X: -1920, Y: -50, Width: 23, Height: 19}, tc.x, tc.y, float32(img.Width)*tc.scale, float32(img.Height)*tc.scale)
		if got := list.ImageDrawCount(); got != tc.want {
			t.Fatalf("%+v: image count %d", tc, got)
		}
	}
}

// TestNineSliceTexturesFollowDisplayDensity keeps slice uploads at display density without
// upsampling the source, while repeated tiles keep their logical size.
func TestNineSliceTexturesFollowDisplayDensity(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 12, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), A: 255})
		}
	}
	layer := &SurfaceImage{Source: source, SliceColumns: [4]int{0, 4, 8, 12}, SliceRows: [4]int{0, 4, 8, 12}, Insets: Insets{Top: 2, Right: 2, Bottom: 2, Left: 2}, PixelScale: .5, RepeatX: true, RepeatY: true}
	for _, tc := range []struct {
		scale float32
		want  int
	}{{1, 2}, {1.5, 3}, {2, 4}, {3, 4}} {
		slices := layer.sliceTextures(tc.scale)
		for index, img := range slices {
			if img.Width != tc.want || img.Height != tc.want {
				t.Fatalf("scale %v slice %d: %dx%d, want %d", tc.scale, index, img.Width, img.Height, tc.want)
			}
		}
		if layer.sliceTextures(tc.scale) != slices {
			t.Fatalf("scale %v: unchanged scale recreated textures", tc.scale)
		}
	}
	bottomRight := layer.sliceTextures(2)[8]
	if got := bottomRight.RGBAAt(3, 3); got.R < 200 || got.G < 200 {
		t.Fatalf("bottom right corner lost its source region: %+v", got)
	}
	if w, h := layer.sliceTileSize(1, 1, 96, 56); w != 2 || h != 2 {
		t.Fatalf("center tile %vx%v", w, h)
	}
	if w, h := layer.sliceTileSize(1, 0, 96, 2); w != 2 || h != 2 {
		t.Fatalf("top tile %vx%v", w, h)
	}

	// Axes that stretch to the surface keep source pixels because their destination is unknown.
	stretched := &SurfaceImage{Source: source, SliceColumns: layer.SliceColumns, SliceRows: layer.SliceRows, Insets: layer.Insets, PixelScale: .5}
	slices := stretched.sliceTextures(1)
	if slices[4].Width != 4 || slices[4].Height != 4 || slices[1].Width != 4 || slices[1].Height != 2 || slices[3].Width != 2 || slices[3].Height != 4 || slices[0].Width != 2 {
		t.Fatalf("stretch axes changed density: center %dx%d top %dx%d left %dx%d corner %dx%d", slices[4].Width, slices[4].Height, slices[1].Width, slices[1].Height, slices[3].Width, slices[3].Height, slices[0].Width, slices[0].Height)
	}

	var list woxui.DisplayList
	list.RasterScale = 2
	layer.paint(&list, woxui.Rect{X: -1920, Y: 24, Width: 10, Height: 8}, 0)
	// 4 corners + 3 top and 3 bottom tiles + 2 left and 2 right tiles + 3x2 center tiles.
	if got := list.ImageDrawCount(); got != 4+6+4+6 {
		t.Fatalf("image draws=%d", got)
	}
}
