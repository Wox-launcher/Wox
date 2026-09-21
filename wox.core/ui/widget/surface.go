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
	InnerShadow *SurfaceInnerShadow
	Background  *SurfaceImage
	Frame       *SurfaceImage
	Decorations []SurfaceDecoration
}

type SurfaceInnerShadow struct {
	Color         woxui.Color
	Width, Radius float32
	Insets        Insets
}

// PaintOverlay keeps the inset shadow above opaque child fills, including the toolbar.
func (s *ImageSurface) PaintOverlay(list *woxui.DisplayList, bounds woxui.Rect) {
	if s == nil || s.InnerShadow == nil {
		return
	}
	shadow := s.InnerShadow
	bounds.X += shadow.Insets.Left
	bounds.Y += shadow.Insets.Top
	bounds.Width -= shadow.Insets.Left + shadow.Insets.Right
	bounds.Height -= shadow.Insets.Top + shadow.Insets.Bottom
	width := min(shadow.Width, min(bounds.Width, bounds.Height)/2)
	if width <= 0 || shadow.Color.A == 0 {
		return
	}
	radius := min(shadow.Radius, min(bounds.Width, bounds.Height)/2)
	step := float32(1) / max(1, list.RasterScale)
	for inset := float32(0); inset < width; inset += step {
		color := shadow.Color
		fade := 1 - inset/width
		color.A = uint8(float32(color.A) * fade * fade)
		rect := woxui.Rect{X: bounds.X + inset, Y: bounds.Y + inset, Width: bounds.Width - 2*inset, Height: bounds.Height - 2*inset}
		list.StrokeRoundedRect(rect, max(0, radius-inset), min(step, width-inset), color)
	}
}

type SurfaceDecoration struct {
	Image                *woxui.Image
	Horizontal, Vertical float32
	Offset               woxui.Point
	Size                 woxui.Size
}

// SurfaceImage retains authored-resolution sources and derives textures at display density.
// Nine-slice themes often author borders far denser than the logical insets they paint into
// (a 250px slice drawn 40 logical units wide), so uploading source pixels every frame cost far
// more GPU staging memory than the theme's file size suggested. Only the latest display scale
// is cached, matching the repeated tile texture.
type SurfaceImage struct {
	RepeatX, RepeatY bool
	// PixelScale is the logical size of one center source pixel, derived from the border density.
	PixelScale float32
	Image      *woxui.Image
	Insets     Insets
	// Source is the shared decoded raster of a nine-slice; SliceColumns and SliceRows hold the
	// source pixel boundaries of its three columns and rows.
	Source                  *image.RGBA
	SliceColumns, SliceRows [4]int
	Tile                    *image.RGBA
	TileSize                woxui.Size
	mu                      sync.Mutex
	cached                  *woxui.Image
	cachedSize              image.Point
	cachedTileSize          image.Point
	slices                  [9]*woxui.Image
	slicesScale             float32
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
	if s.Source == nil {
		return
	}
	slices := s.sliceTextures(list.RasterScale)
	xs := surfaceEdges(bounds.X, bounds.Width, s.Insets.Left, s.Insets.Right)
	ys := surfaceEdges(bounds.Y, bounds.Height, s.Insets.Top, s.Insets.Bottom)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img := slices[y*3+x]
			if img == nil || xs[x+1] <= xs[x] || ys[y+1] <= ys[y] {
				continue
			}
			rect := woxui.Rect{X: xs[x], Y: ys[y], Width: xs[x+1] - xs[x], Height: ys[y+1] - ys[y]}
			// Repeated tiles keep their logical size regardless of texture density.
			tileWidth, tileHeight := s.sliceTileSize(x, y, rect.Width, rect.Height)
			paintRepeatedSlice(list, img, rect, s.RepeatX && x == 1, s.RepeatY && y == 1, tileWidth, tileHeight)
		}
	}
}

// sliceTileSize reports the logical size of one repeated tile for the slice at column x, row y
// when painted into a destination of the given size. Border rows and columns scale uniformly
// with their destination thickness; the center uses the authored PixelScale.
func (s *SurfaceImage) sliceTileSize(x, y int, width, height float32) (float32, float32) {
	sourceWidth := float32(s.SliceColumns[x+1] - s.SliceColumns[x])
	sourceHeight := float32(s.SliceRows[y+1] - s.SliceRows[y])
	scale := s.PixelScale
	if scale <= 0 {
		scale = 1
	}
	if y != 1 && sourceHeight > 0 {
		scale = height / sourceHeight
	} else if x != 1 && sourceWidth > 0 {
		scale = width / sourceWidth
	}
	return sourceWidth * scale, sourceHeight * scale
}

// sliceTextures returns the nine textures for one display scale. Every axis with a known
// destination (border thickness, repeated tile length) is downsampled to display density; an
// axis that stretches to the surface size keeps its source resolution because its destination
// is unknown here. Sources are never upsampled.
func (s *SurfaceImage) sliceTextures(scale float32) [9]*woxui.Image {
	s.mu.Lock()
	defer s.mu.Unlock()
	if scale <= 0 {
		scale = 1
	}
	if s.slicesScale == scale {
		return s.slices
	}
	insetWidths := [3]float32{s.Insets.Left, 0, s.Insets.Right}
	insetHeights := [3]float32{s.Insets.Top, 0, s.Insets.Bottom}
	var slices [9]*woxui.Image
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			source := image.Rect(s.SliceColumns[x], s.SliceRows[y], s.SliceColumns[x+1], s.SliceRows[y+1])
			if source.Empty() {
				continue
			}
			width, height := source.Dx(), source.Dy()
			targetWidth, targetHeight := s.sliceTileSize(x, y, insetWidths[x], insetHeights[y])
			if x != 1 {
				targetWidth = insetWidths[x]
			}
			if y != 1 {
				targetHeight = insetHeights[y]
			}
			if x != 1 || s.RepeatX {
				width = min(width, max(1, int(math.Ceil(float64(targetWidth*scale)))))
			}
			if y != 1 || s.RepeatY {
				height = min(height, max(1, int(math.Ceil(float64(targetHeight*scale)))))
			}
			raster := image.NewRGBA(image.Rect(0, 0, width, height))
			source = source.Add(s.Source.Bounds().Min)
			if width == source.Dx() && height == source.Dy() {
				draw.Draw(raster, raster.Bounds(), s.Source, source.Min, draw.Src)
			} else {
				draw.CatmullRom.Scale(raster, raster.Bounds(), s.Source, source, draw.Src, nil)
			}
			slices[y*3+x], _ = woxui.NewImageFromPackedRGBA(raster)
		}
	}
	s.slices = slices
	s.slicesScale = scale
	return slices
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

// paintRepeatedSlice clips partial final tiles instead of squeezing their motifs. Tile sizes are
// logical units; a non-repeated axis stretches to the destination.
func paintRepeatedSlice(list *woxui.DisplayList, img *woxui.Image, rect woxui.Rect, repeatX, repeatY bool, tileWidth, tileHeight float32) {
	if !repeatX && !repeatY {
		list.DrawImage(img, rect)
		return
	}
	width, height := rect.Width, rect.Height
	if repeatX {
		width = tileWidth
	}
	if repeatY {
		height = tileHeight
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
