package widget

import (
	"image"
	"math"
	"sync"

	ui "wox/ui/runtime"
)

// BoxShadow paints a soft outer shadow without changing layout or pointer bounds.
// Sigma, Radius, and Offset are logical units. The original silhouette is cut out
// so transparent children do not acquire an unintended dark background.
type BoxShadow struct {
	Sigma  float32
	Radius float32
	Offset ui.Point
	Color  ui.Color
	Child  Widget
}

// layout keeps the child geometry intact and records only the decorative paint extent.
func (w BoxShadow) layout(ctx context, available constraints) *node {
	result := &node{}
	if w.Child != nil {
		child := available.constrainNode(w.Child.layout(ctx, available))
		result.bounds.Width, result.bounds.Height = child.bounds.Width, child.bounds.Height
		result.children = []*node{child}
	}
	result = available.constrainNode(result)
	if w.Color.A == 0 || w.Sigma <= 0 || result.bounds.Width <= 0 || result.bounds.Height <= 0 {
		return result
	}
	// Keep the generic primitive bounded even when used without protocol validation.
	if math.IsNaN(float64(w.Sigma+w.Radius+w.Offset.X+w.Offset.Y)) || math.IsInf(float64(w.Sigma+w.Radius+w.Offset.X+w.Offset.Y), 0) {
		return result
	}
	w.Sigma = min(w.Sigma, 128)
	w.Radius = min(max(float32(0), w.Radius), min(result.bounds.Width, result.bounds.Height)/2)
	w.Offset.X, w.Offset.Y = min(float32(384), max(float32(-384), w.Offset.X)), min(float32(384), max(float32(-384), w.Offset.Y))
	outsets := w.outsets()
	result.paintOutsets = outsets
	result.paint = func(list *ui.DisplayList, bounds ui.Rect) {
		key := shadowKey{sigma: w.Sigma, radius: w.Radius, offset: w.Offset, color: w.Color, scale: rasterScale(list.RasterScale)}
		// Only corners and edge cross-sections depend on size. Long straight edges
		// reuse the same texture as the box grows, including during animation.
		key.width = min(bounds.Width, 2*(w.Radius+3*w.Sigma+float32(math.Abs(float64(w.Offset.X))))+1)
		key.height = min(bounds.Height, 2*(w.Radius+3*w.Sigma+float32(math.Abs(float64(w.Offset.Y))))+1)
		texture := boxShadowCache.get(key)
		outer := ui.Rect{X: bounds.X - outsets.Left, Y: bounds.Y - outsets.Top,
			Width: bounds.Width + outsets.Left + outsets.Right, Height: bounds.Height + outsets.Top + outsets.Bottom}
		xs := surfaceEdges(outer.X, outer.Width, texture.insets.Left, texture.insets.Right)
		ys := surfaceEdges(outer.Y, outer.Height, texture.insets.Top, texture.insets.Bottom)
		for y := 0; y < 3; y++ {
			for x := 0; x < 3; x++ {
				if img := texture.tiles[y*3+x]; img != nil {
					list.DrawImage(img, ui.Rect{X: xs[x], Y: ys[y], Width: xs[x+1] - xs[x], Height: ys[y+1] - ys[y]})
				}
			}
		}
	}
	return result
}

func (w BoxShadow) outsets() Insets {
	pad := 3 * w.Sigma
	return Insets{Left: max(float32(0), pad-w.Offset.X), Right: max(float32(0), pad+w.Offset.X),
		Top: max(float32(0), pad-w.Offset.Y), Bottom: max(float32(0), pad+w.Offset.Y)}
}

type shadowKey struct {
	width, height, sigma, radius, scale float32
	offset                              ui.Point
	color                               ui.Color
}

type shadowTexture struct {
	tiles  [9]*ui.Image
	insets Insets
	bytes  int
}

type shadowCacheEntry struct {
	key     shadowKey
	texture *shadowTexture
}

// A short LRU slice avoids retaining one full-size bitmap for every card or
// animated dimension. Pixels are immutable; rasterization happens outside the lock.
type shadowCache struct {
	mu      sync.Mutex
	entries []shadowCacheEntry
	bytes   int
}

const shadowCacheBytes = 8 << 20
const shadowRasterLimit = 512

var boxShadowCache shadowCache

// get shares small nine-slice textures across windows, with bounded retained memory.
func (c *shadowCache) get(key shadowKey) *shadowTexture {
	c.mu.Lock()
	for i, entry := range c.entries {
		if entry.key == key {
			copy(c.entries[i:], c.entries[i+1:])
			c.entries[len(c.entries)-1] = entry
			c.mu.Unlock()
			return entry.texture
		}
	}
	c.mu.Unlock()
	texture := rasterShadow(key)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, entry := range c.entries {
		if entry.key == key {
			return entry.texture
		}
	}
	for len(c.entries) > 0 && (len(c.entries) >= 64 || c.bytes+texture.bytes > shadowCacheBytes) {
		c.bytes -= c.entries[0].texture.bytes
		copy(c.entries, c.entries[1:])
		c.entries = c.entries[:len(c.entries)-1]
	}
	c.entries = append(c.entries, shadowCacheEntry{key: key, texture: texture})
	c.bytes += texture.bytes
	return texture
}

// rasterShadow evaluates a Gaussian falloff around a rounded-rectangle distance
// field, then cuts the original silhouette out. Nine-slicing keeps large panels
// inexpensive; unusually large radii use a bounded lower-resolution texture.
func rasterShadow(key shadowKey) *shadowTexture {
	out := (BoxShadow{Sigma: key.sigma, Offset: key.offset}).outsets()
	w, h := key.width+out.Left+out.Right, key.height+out.Top+out.Bottom
	scale := min(key.scale, float32(shadowRasterLimit)/max(w, h))
	pw, ph := max(3, int(math.Ceil(float64(w*scale)))), max(3, int(math.Ceil(float64(h*scale))))
	sx, sy := float32(pw)/w, float32(ph)/h
	edgeX := min(key.width/2, key.radius+3*key.sigma+float32(math.Abs(float64(key.offset.X))))
	edgeY := min(key.height/2, key.radius+3*key.sigma+float32(math.Abs(float64(key.offset.Y))))
	x1 := min(pw-2, max(1, int(math.Round(float64((out.Left+edgeX)*sx)))))
	y1 := min(ph-2, max(1, int(math.Round(float64((out.Top+edgeY)*sy)))))
	x2 := min(pw-1, max(x1+1, int(math.Round(float64((out.Left+key.width-edgeX)*sx)))))
	y2 := min(ph-1, max(y1+1, int(math.Round(float64((out.Top+key.height-edgeY)*sy)))))
	xs, ys := [4]int{0, x1, x2, pw}, [4]int{0, y1, y2, ph}
	result := &shadowTexture{insets: Insets{Left: float32(x1) / sx, Right: float32(pw-x2) / sx, Top: float32(y1) / sy, Bottom: float32(ph-y2) / sy}}
	distance := func(x, y float32) float64 {
		dx := math.Abs(float64(x-key.width/2)) - float64(key.width/2-key.radius)
		dy := math.Abs(float64(y-key.height/2)) - float64(key.height/2-key.radius)
		return math.Hypot(max(dx, 0), max(dy, 0)) + min(max(dx, dy), 0) - float64(key.radius)
	}
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			pixels := image.NewRGBA(image.Rect(0, 0, xs[col+1]-xs[col], ys[row+1]-ys[row]))
			visible := false
			for y := ys[row]; y < ys[row+1]; y++ {
				for x := xs[col]; x < xs[col+1]; x++ {
					lx, ly := (float32(x)+0.5)/sx-out.Left, (float32(y)+0.5)/sy-out.Top
					// Smooth the knockout across one source pixel to keep rounded
					// corners stable on fractional display scales.
					outside := min(float64(1), max(float64(0), distance(lx, ly)*float64(min(sx, sy))+0.5))
					coverage := 0.5 * math.Erfc(distance(lx-key.offset.X, ly-key.offset.Y)/(float64(key.sigma)*math.Sqrt2)) * outside
					a := uint8(math.Round(float64(key.color.A) * coverage))
					if a == 0 {
						continue
					}
					visible = true
					i := (y-ys[row])*pixels.Stride + (x-xs[col])*4
					pixels.Pix[i], pixels.Pix[i+1], pixels.Pix[i+2], pixels.Pix[i+3] = uint8(uint32(key.color.R)*uint32(a)/255), uint8(uint32(key.color.G)*uint32(a)/255), uint8(uint32(key.color.B)*uint32(a)/255), a
				}
			}
			if visible {
				result.tiles[row*3+col], _ = ui.NewImageFromPackedRGBA(pixels)
				result.bytes += len(pixels.Pix)
			}
		}
	}
	return result
}
