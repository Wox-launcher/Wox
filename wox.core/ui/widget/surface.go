package widget

import (
	"golang.org/x/image/draw"
	"image"
	"math"
	"sync"
	woxui "wox/ui/runtime"
)

// ImageSurface is immutable decoration shared by cached widget descriptions.
// Layers paint inside the existing surface and never introduce input targets.
type ImageSurface struct {
	Background  *SurfaceImage
	Frame       *SurfaceImage
	Decorations []SurfaceDecoration
}

type SurfaceDecoration struct {
	Image                *woxui.Image
	Horizontal, Vertical float32
	Offset               woxui.Point
	Size                 woxui.Size
}

// SurfaceImage retains source-resolution slices; only repeated textures need a size cache.
type SurfaceImage struct {
	RepeatX, RepeatY bool
	PixelScale       float32
	Image            *woxui.Image
	Slices           [9]*woxui.Image
	Insets           Insets
	Tile             *image.RGBA
	TileSize         woxui.Size
	mu               sync.Mutex
	cached           *woxui.Image
	cachedSize       image.Point
	cachedTileSize   image.Point
}

// Paint paints decoration over the surface fill, before its interactive contents.
func (s *ImageSurface) Paint(list *woxui.DisplayList, bounds woxui.Rect, radius float32) {
	if s == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	list.PushClipRect(bounds)
	defer list.PopClipRect()
	s.Background.paint(list, bounds, radius)
	s.Frame.paint(list, bounds, 0)
	for _, d := range s.Decorations {
		r := woxui.Rect{X: bounds.X + (bounds.Width-d.Size.Width)*d.Horizontal + d.Offset.X, Y: bounds.Y + (bounds.Height-d.Size.Height)*d.Vertical + d.Offset.Y, Width: d.Size.Width, Height: d.Size.Height}
		list.DrawImage(d.Image, r)
	}
}

// paint keeps slice destinations in logical units, independent of desktop origin and DPI.
func (s *SurfaceImage) paint(list *woxui.DisplayList, bounds woxui.Rect, radius float32) {
	if s == nil {
		return
	}
	if s.Tile != nil {
		if img := s.tiled(bounds, list.RasterScale); img != nil {
			list.DrawRotatedRoundedImage(img, bounds, 0, radius)
		}
		return
	}
	if s.Image != nil {
		list.DrawRotatedRoundedImage(s.Image, bounds, 0, radius)
		return
	}
	xs := surfaceEdges(bounds.X, bounds.Width, s.Insets.Left, s.Insets.Right)
	ys := surfaceEdges(bounds.Y, bounds.Height, s.Insets.Top, s.Insets.Bottom)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if img := s.Slices[y*3+x]; img != nil && xs[x+1] > xs[x] && ys[y+1] > ys[y] {
				rect := woxui.Rect{X: xs[x], Y: ys[y], Width: xs[x+1] - xs[x], Height: ys[y+1] - ys[y]}
				scale := s.PixelScale
				if scale <= 0 {
					scale = 1
				}
				if y != 1 {
					scale = rect.Height / float32(img.Height)
				} else if x != 1 {
					scale = rect.Width / float32(img.Width)
				}
				paintRepeatedSlice(list, img, rect, s.RepeatX && x == 1, s.RepeatY && y == 1, scale)
			}
		}
	}
}

// surfaceEdges proportionally shrinks borders when a surface is smaller than its corners.
func surfaceEdges(origin, length, before, after float32) [4]float32 {
	if before+after > length {
		scale := length / (before + after)
		before *= scale
		after *= scale
	}
	return [4]float32{origin, origin + before, origin + length - after, origin + length}
}

// tiled retains only the latest size at display density, capped by source resolution.
// Native rounded-image sampling clips the whole tiled background, including its corners.
func (s *SurfaceImage) tiled(bounds woxui.Rect, scale float32) *woxui.Image {
	s.mu.Lock()
	defer s.mu.Unlock()
	if scale <= 0 {
		scale = 1
	}
	if s.TileSize.Width <= 0 || s.TileSize.Height <= 0 {
		return nil
	}
	tw := min(s.Tile.Bounds().Dx(), max(1, int(math.Ceil(float64(s.TileSize.Width*scale)))))
	th := min(s.Tile.Bounds().Dy(), max(1, int(math.Ceil(float64(s.TileSize.Height*scale)))))
	w := int(math.Ceil(float64(bounds.Width/s.TileSize.Width) * float64(tw)))
	h := int(math.Ceil(float64(bounds.Height/s.TileSize.Height) * float64(th)))
	if w <= 0 || h <= 0 || int64(w)*int64(h) > 16<<20 {
		return nil
	}
	if s.cached != nil && s.cachedSize == image.Pt(w, h) && s.cachedTileSize == image.Pt(tw, th) {
		return s.cached
	}
	raster := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y += th {
		for x := 0; x < w; x += tw {
			draw.BiLinear.Scale(raster, image.Rect(x, y, x+tw, y+th), s.Tile, s.Tile.Bounds(), draw.Src, nil)
		}
	}
	s.cached, _ = woxui.NewImageFromPackedRGBA(raster)
	s.cachedSize = image.Pt(w, h)
	s.cachedTileSize = image.Pt(tw, th)
	return s.cached
}

// paintRepeatedSlice clips partial final tiles instead of squeezing their motifs.
func paintRepeatedSlice(list *woxui.DisplayList, img *woxui.Image, rect woxui.Rect, repeatX, repeatY bool, scale float32) {
	if !repeatX && !repeatY {
		list.DrawImage(img, rect)
		return
	}
	width, height := rect.Width, rect.Height
	if repeatX {
		width = float32(img.Width) * scale
	}
	if repeatY {
		height = float32(img.Height) * scale
	}
	if width <= 0 || height <= 0 {
		return
	}
	columns, rows := math.Ceil(float64(rect.Width/width)), math.Ceil(float64(rect.Height/height))
	// Bound commands for pathological tiny textures; ordinary theme tiles stay far below this.
	if columns*rows > 4096 {
		list.DrawImage(img, rect)
		return
	}
	list.PushClipRect(rect)
	defer list.PopClipRect()
	for y := 0; y < int(rows); y++ {
		for x := 0; x < int(columns); x++ {
			list.DrawImage(img, woxui.Rect{X: rect.X + float32(x)*width, Y: rect.Y + float32(y)*height, Width: width, Height: height})
		}
	}
}
