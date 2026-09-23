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
	scale := rasterScale(list.RasterScale)
	slices := s.sliceTextures(scale)
	// Device-pixel edges keep a corner and its border on the same pixel while the
	// launcher height changes during search. Independent rounding would open a crack.
	xs := snapEdges(surfaceEdges(bounds.X, bounds.Width, s.Insets.Left, s.Insets.Right), scale)
	ys := snapEdges(surfaceEdges(bounds.Y, bounds.Height, s.Insets.Top, s.Insets.Bottom), scale)
	// One device pixel of overlap covers the filtered texel at a nine-slice joint.
	pad := float32(1) / scale
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img := slices[y*3+x]
			if img == nil || xs[x+1] <= xs[x] || ys[y+1] <= ys[y] {
				continue
			}
			rect := woxui.Rect{X: xs[x], Y: ys[y], Width: xs[x+1] - xs[x], Height: ys[y+1] - ys[y]}
			// Repeated tiles keep their logical size regardless of texture density.
			tileWidth, tileHeight := s.sliceTileSize(x, y, rect.Width, rect.Height)
			left, top, right, bottom := borderJointOverlap(x, y, xs, ys, pad)
			paintRepeatedSlice(list, img, rect, s.RepeatX && x == 1, s.RepeatY && y == 1, tileWidth, tileHeight, scale, left, top, right, bottom)
		}
	}
}

// rasterScale is the display density used to snap slice geometry. An unknown scale is 1:1.
func rasterScale(scale float32) float32 {
	if scale <= 0 {
		return 1
	}
	return scale
}

// snapRaster rounds one logical coordinate onto the device-pixel grid.
func snapRaster(value, scale float32) float32 {
	scale = rasterScale(scale)
	return float32(math.Round(float64(value*scale))) / scale
}

// snapEdges snaps each nine-slice boundary and keeps the three cells in order.
func snapEdges(edges [4]float32, scale float32) [4]float32 {
	for i := range edges {
		edges[i] = snapRaster(edges[i], scale)
	}
	for i := 1; i < len(edges); i++ {
		if edges[i] < edges[i-1] {
			edges[i] = edges[i-1]
		}
	}
	return edges
}

// borderJointOverlap lets a border strip cover one device pixel of each corner.
// The center cell is left alone: image themes often leave it transparent, and
// overlapping it would erase the frame.
func borderJointOverlap(x, y int, xs, ys [4]float32, pad float32) (left, top, right, bottom float32) {
	if x == 1 && y != 1 {
		if xs[1] > xs[0] {
			left = pad
		}
		if xs[3] > xs[2] {
			right = pad
		}
	}
	if y == 1 && x != 1 {
		if ys[1] > ys[0] {
			top = pad
		}
		if ys[3] > ys[2] {
			bottom = pad
		}
	}
	return left, top, right, bottom
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

// paintRepeatedSlice draws one nine-slice cell. A repeated axis keeps the authored tile
// size and clips the leftover piece, so growing the launcher reveals more pattern instead
// of scaling every tile on that side. Tile edges snap to device pixels. overlap* lets the
// first and last tile cover one pixel of a corner joint; corners and the center pass zero.
func paintRepeatedSlice(list *woxui.DisplayList, img *woxui.Image, rect woxui.Rect, repeatX, repeatY bool, tileWidth, tileHeight, scale, overlapLeft, overlapTop, overlapRight, overlapBottom float32) {
	scale = rasterScale(scale)
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
	columns, rows := 1.0, 1.0
	if repeatX {
		columns = math.Ceil(float64(rect.Width / width))
	}
	if repeatY {
		rows = math.Ceil(float64(rect.Height / height))
	}
	// Bound commands for pathological tiny textures; ordinary theme tiles stay far below this.
	if columns*rows > 4096 {
		list.DrawImage(img, rect)
		return
	}
	clip := rect
	clip.X -= overlapLeft
	clip.Y -= overlapTop
	clip.Width += overlapLeft + overlapRight
	clip.Height += overlapTop + overlapBottom
	if columns > 1 || rows > 1 || overlapLeft != 0 || overlapTop != 0 || overlapRight != 0 || overlapBottom != 0 {
		list.PushClipRect(clip)
		defer list.PopClipRect()
	}
	rowCount, columnCount := int(rows), int(columns)
	for y := 0; y < rowCount; y++ {
		// Full tile size, including the piece the clip cuts off. Shrinking that
		// destination would squash the motif into the leftover span.
		y0 := snapRaster(rect.Y+float32(y)*height, scale)
		y1 := snapRaster(rect.Y+float32(y+1)*height, scale)
		if y == 0 {
			y0 -= overlapTop
		}
		if y == rowCount-1 {
			y1 += overlapBottom
		}
		for x := 0; x < columnCount; x++ {
			x0 := snapRaster(rect.X+float32(x)*width, scale)
			x1 := snapRaster(rect.X+float32(x+1)*width, scale)
			if x == 0 {
				x0 -= overlapLeft
			}
			if x == columnCount-1 {
				x1 += overlapRight
			}
			if x1 <= x0 || y1 <= y0 {
				continue
			}
			list.DrawImage(img, woxui.Rect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0})
		}
	}
}
