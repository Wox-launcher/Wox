package component

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestThemeSurfaceRendering exercises real slicing, layering and additive logical safe areas.
func TestThemeSurfaceRendering(t *testing.T) {
	raster := image.NewRGBA(image.Rect(0, 0, 12, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			raster.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), A: 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, raster); err != nil {
		t.Fatal(err)
	}
	definitions := common.ThemeSurfaces{"App": {
		Background:    &common.ThemeSurfaceImage{Source: "a.png", Mode: "stretch"},
		Frame:         &common.ThemeSurfaceImage{Source: "a.png", Mode: "nineSlice", Slice: &common.ThemeInsets{Top: 4, Right: 4, Bottom: 4, Left: 4}, Insets: &common.ThemeInsets{Top: 2, Right: 2, Bottom: 2, Left: 2}},
		Decorations:   []common.ThemeDecoration{{Source: "a.png", Anchor: "topCenter", Size: common.ThemeSize{Width: 6, Height: 6}}},
		ContentInsets: &common.ThemeInsets{Top: 10, Right: 3, Bottom: 4, Left: 5},
	}}
	definitions["Preview"] = &common.ThemeSurface{Background: &common.ThemeSurfaceImage{Source: "a.png", Mode: "tile"}}
	definitions["Toolbar"] = &common.ThemeSurface{Background: &common.ThemeSurfaceImage{Source: "a.png", Mode: "tile"}}
	surfaces := LoadThemeSurfaces(definitions, map[string][]byte{"a.png": data.Bytes()})
	if surfaces.Get("Preview").Background.Tile != surfaces.Get("Toolbar").Background.Tile {
		t.Fatal("shared tile source was duplicated")
	}
	app := surfaces.Get("App")
	if app.Frame.Source != surfaces.Get("Preview").Background.Tile || app.Frame.SliceColumns != [4]int{0, 4, 8, 12} || app.Frame.SliceRows != [4]int{0, 4, 8, 12} {
		t.Fatal("nine-slice did not share the decoded source or sliced it wrongly")
	}
	if app.Frame.Source.RGBAAt(8, 8) != (color.RGBA{R: 8, G: 8, A: 255}) {
		t.Fatal("bottom right source slice is wrong")
	}
	list := &woxui.DisplayList{}
	woxwidget.PaintStateless(nil, woxwidget.Container{Width: 100, Height: 60, Surface: app}, list, woxui.Rect{X: -120, Y: 24, Width: 100, Height: 60})
	if list.ImageDrawCount() != 11 {
		t.Fatalf("image draws=%d, want background + 9 slices + crest", list.ImageDrawCount())
	}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		bounds := LauncherContentBounds(100, 60, 2, surfaces)
		if bounds != (woxui.Rect{X: 7, Y: 12, Width: 88, Height: 42}) {
			t.Fatal(bounds)
		}
		if app.Frame.Insets.Left*scale != 2*scale || app.Frame.SliceColumns[1] != 4 {
			t.Fatal("source pixels changed with display scale")
		}
	}
}
