package component

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"strings"
	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// ThemeSurfaceSet is prepared once when the palette changes, never inside a Boundary build.
type ThemeSurfaceSet struct {
	Items         map[string]*woxwidget.ImageSurface
	ContentInsets woxwidget.Insets
}

func (s *ThemeSurfaceSet) Get(name string) *woxwidget.ImageSurface {
	if s == nil {
		return nil
	}
	return s.Items[name]
}

// LoadThemeSurfaces decodes shared resources once and keeps invalid assets on the color fallback.
func LoadThemeSurfaces(definitions common.ThemeSurfaces, assets map[string][]byte) *ThemeSurfaceSet {
	if len(definitions) == 0 {
		return nil
	}
	set := &ThemeSurfaceSet{Items: map[string]*woxwidget.ImageSurface{}}
	decoded := map[string]*image.RGBA{}
	images := map[string]*woxui.Image{}
	load := func(source string) *image.RGBA {
		if img, ok := decoded[source]; ok {
			return img
		}
		data := assets[source]
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16<<20 {
			decoded[source] = nil
			return nil
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("decode theme image %s: %v", source, err))
			decoded[source] = nil
			return nil
		}
		// Tile layers and runtime images share one immutable packed source buffer.
		raster, ok := img.(*image.RGBA)
		if !ok {
			raster = image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
			draw.Draw(raster, raster.Bounds(), img, img.Bounds().Min, draw.Src)
		}
		decoded[source] = raster
		images[source], _ = woxui.NewImageFromPackedRGBA(raster)
		return raster
	}
	layer := func(def *common.ThemeSurfaceImage) *woxwidget.SurfaceImage {
		if def == nil {
			return nil
		}
		img := load(def.Source)
		if img == nil {
			return nil
		}
		result := &woxwidget.SurfaceImage{}
		switch def.Mode {
		case "stretch":
			result.Image = images[def.Source]
		case "tile":
			result.Tile = img
			result.TileSize = woxui.Size{Width: float32(img.Bounds().Dx()), Height: float32(img.Bounds().Dy())}
			if def.Size != nil {
				result.TileSize = woxui.Size{Width: float32(def.Size.Width), Height: float32(def.Size.Height)}
			}
		case "nineSlice":
			if def.Slice == nil || def.Insets == nil {
				return nil
			}
			s := def.Slice
			w, h := img.Bounds().Dx(), img.Bounds().Dy()
			if s.Left+s.Right >= w || s.Top+s.Bottom >= h {
				util.GetLogger().Warn(context.Background(), "theme nineSlice exceeds source image: "+def.Source)
				return nil
			}
			// Surfaces sharing a source keep one decoded raster; display-density slices are
			// derived at paint time instead of copying source pixels per surface.
			result.Source = img
			result.SliceColumns = [4]int{0, s.Left, w - s.Right, w}
			result.SliceRows = [4]int{0, s.Top, h - s.Bottom, h}
			result.Insets = themeSurfaceInsets(*def.Insets)
			if def.Repeat != nil {
				result.RepeatX = def.Repeat.X == "tile"
				result.RepeatY = def.Repeat.Y == "tile"
			}
			// The center uses the first nonzero authored border density as its pixel scale.
			result.PixelScale = 1
			for _, pair := range [][2]int{{s.Top, def.Insets.Top}, {s.Bottom, def.Insets.Bottom}, {s.Left, def.Insets.Left}, {s.Right, def.Insets.Right}} {
				if pair[0] > 0 && pair[1] > 0 {
					result.PixelScale = float32(pair[1]) / float32(pair[0])
					break
				}
			}
		}
		return result
	}
	for name, def := range definitions {
		if def == nil {
			continue
		}
		surface := &woxwidget.ImageSurface{Background: layer(def.Background), Frame: layer(def.Frame)}
		if shadow := def.InnerShadow; shadow != nil {
			color, _ := common.ParseThemeColor(shadow.Color)
			surface.InnerShadow = &woxwidget.SurfaceInnerShadow{Color: woxui.Color{R: color.R, G: color.G, B: color.B, A: color.A}, Width: float32(shadow.Width), Radius: float32(shadow.Radius), Insets: themeSurfaceInsets(shadow.Insets)}
		}
		for _, d := range def.Decorations {
			if load(d.Source) == nil {
				continue
			}
			h, v := float32(.5), float32(.5)
			if strings.HasSuffix(d.Anchor, "Left") {
				h = 0
			}
			if strings.HasSuffix(d.Anchor, "Right") {
				h = 1
			}
			if strings.HasPrefix(d.Anchor, "top") {
				v = 0
			}
			if strings.HasPrefix(d.Anchor, "bottom") {
				v = 1
			}
			surface.Decorations = append(surface.Decorations, woxwidget.SurfaceDecoration{Image: images[d.Source], Horizontal: h, Vertical: v, Offset: woxui.Point{X: float32(d.Offset.X), Y: float32(d.Offset.Y)}, Size: woxui.Size{Width: float32(d.Size.Width), Height: float32(d.Size.Height)}})
		}
		if name == "App" && def.ContentInsets != nil {
			set.ContentInsets = themeSurfaceInsets(*def.ContentInsets)
		}
		set.Items[name] = surface
	}
	return set
}

func themeSurfaceInsets(v common.ThemeInsets) woxwidget.Insets {
	return woxwidget.Insets{Top: float32(v.Top), Right: float32(v.Right), Bottom: float32(v.Bottom), Left: float32(v.Left)}
}
