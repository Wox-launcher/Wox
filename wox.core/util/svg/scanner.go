package svg

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
)

const antialiasSamples = 8

type scanEdge struct {
	x1, y1 float64
	x2, y2 float64
}

type scanCrossing struct {
	x     float64
	delta int
}

// windingScanner rasterizes flattened paths with either SVG winding rule.
// rasterx.ScannerGV ignores even-odd winding, so Wox owns this final scan step.
type windingScanner struct {
	width, height int
	dest          draw.Image
	clip          image.Rectangle
	source        image.Image
	edges         []scanEdge
	current       fixed.Point26_6
	hasCurrent    bool
	nonZero       bool
	extent        fixed.Rectangle26_6
	hasExtent     bool
}

func newWindingScanner(width, height int, dest draw.Image) *windingScanner {
	s := &windingScanner{dest: dest, nonZero: true}
	s.SetBounds(width, height)
	s.SetColor(color.Black)
	return s
}

func (s *windingScanner) Start(point fixed.Point26_6) {
	s.current = point
	s.hasCurrent = true
	s.include(point)
}

func (s *windingScanner) Line(point fixed.Point26_6) {
	if !s.hasCurrent {
		s.Start(point)
		return
	}
	s.edges = append(s.edges, scanEdge{
		x1: float64(s.current.X) / 64,
		y1: float64(s.current.Y) / 64,
		x2: float64(point.X) / 64,
		y2: float64(point.Y) / 64,
	})
	s.current = point
	s.include(point)
}

func (s *windingScanner) Draw() {
	if len(s.edges) == 0 || s.source == nil {
		return
	}

	renderBounds := image.Rect(0, 0, s.width, s.height)
	if s.clip != image.ZR {
		renderBounds = renderBounds.Intersect(s.clip)
	}
	if renderBounds.Empty() {
		return
	}

	mask := image.NewAlpha(image.Rect(0, 0, s.width, s.height))
	coverage := make([]float64, renderBounds.Dx())
	crossings := make([]scanCrossing, 0, len(s.edges))
	for y := renderBounds.Min.Y; y < renderBounds.Max.Y; y++ {
		clear(coverage)
		for sample := 0; sample < antialiasSamples; sample++ {
			sampleY := float64(y) + (float64(sample)+0.5)/antialiasSamples
			crossings = crossings[:0]
			for _, edge := range s.edges {
				if edge.y1 == edge.y2 {
					continue
				}
				minimumY, maximumY := edge.y1, edge.y2
				delta := 1
				if minimumY > maximumY {
					minimumY, maximumY = maximumY, minimumY
					delta = -1
				}
				// The half-open interval avoids counting a shared vertex twice.
				if sampleY < minimumY || sampleY >= maximumY {
					continue
				}
				x := edge.x1 + (sampleY-edge.y1)*(edge.x2-edge.x1)/(edge.y2-edge.y1)
				crossings = append(crossings, scanCrossing{x: x, delta: delta})
			}
			if len(crossings) < 2 {
				continue
			}
			sort.Slice(crossings, func(i, j int) bool { return crossings[i].x < crossings[j].x })
			s.accumulateScanline(coverage, renderBounds.Min.X, renderBounds.Max.X, crossings)
		}
		for x, value := range coverage {
			if value <= 0 {
				continue
			}
			if value > 1 {
				value = 1
			}
			mask.SetAlpha(renderBounds.Min.X+x, y, color.Alpha{A: uint8(math.Round(value * 255))})
		}
	}

	draw.DrawMask(s.dest, renderBounds, s.source, renderBounds.Min, mask, renderBounds.Min, draw.Over)
}

func (s *windingScanner) accumulateScanline(coverage []float64, minimumX, maximumX int, crossings []scanCrossing) {
	winding := 0
	spanStart := 0.0
	for index := 0; index < len(crossings); {
		x := crossings[index].x
		delta := 0
		count := 0
		for index < len(crossings) && math.Abs(crossings[index].x-x) < 1e-9 {
			delta += crossings[index].delta
			count++
			index++
		}

		wasInside := winding != 0
		if s.nonZero {
			winding += delta
		} else if count%2 != 0 {
			winding ^= 1
		}
		isInside := winding != 0
		if !wasInside && isInside {
			spanStart = x
		} else if wasInside && !isInside {
			accumulateSpan(coverage, minimumX, maximumX, spanStart, x)
		}
	}
}

func accumulateSpan(coverage []float64, minimumX, maximumX int, start, end float64) {
	if end < start {
		start, end = end, start
	}
	start = math.Max(start, float64(minimumX))
	end = math.Min(end, float64(maximumX))
	if end <= start {
		return
	}
	first := max(int(math.Floor(start)), minimumX)
	last := min(int(math.Ceil(end)), maximumX)
	for x := first; x < last; x++ {
		overlap := math.Min(end, float64(x+1)) - math.Max(start, float64(x))
		if overlap > 0 {
			coverage[x-minimumX] += overlap / antialiasSamples
		}
	}
}

func (s *windingScanner) GetPathExtent() fixed.Rectangle26_6 {
	if !s.hasExtent {
		return fixed.Rectangle26_6{}
	}
	return s.extent
}

func (s *windingScanner) SetBounds(width, height int) {
	s.width = width
	s.height = height
}

func (s *windingScanner) SetColor(value interface{}) {
	bounds := image.Rect(0, 0, s.width, s.height)
	switch typed := value.(type) {
	case color.Color:
		s.source = image.NewUniform(typed)
	case rasterx.ColorFunc:
		s.source = colorFunctionImage{bounds: bounds, colorAt: typed}
	case func(int, int) color.Color:
		s.source = colorFunctionImage{bounds: bounds, colorAt: typed}
	default:
		s.source = nil
	}
}

func (s *windingScanner) SetWinding(useNonZeroWinding bool) {
	s.nonZero = useNonZeroWinding
}

func (s *windingScanner) Clear() {
	s.edges = s.edges[:0]
	s.hasCurrent = false
	s.hasExtent = false
}

func (s *windingScanner) SetClip(rect image.Rectangle) {
	s.clip = rect
}

func (s *windingScanner) include(point fixed.Point26_6) {
	if !s.hasExtent {
		s.extent = fixed.Rectangle26_6{Min: point, Max: point}
		s.hasExtent = true
		return
	}
	if point.X < s.extent.Min.X {
		s.extent.Min.X = point.X
	}
	if point.Y < s.extent.Min.Y {
		s.extent.Min.Y = point.Y
	}
	if point.X > s.extent.Max.X {
		s.extent.Max.X = point.X
	}
	if point.Y > s.extent.Max.Y {
		s.extent.Max.Y = point.Y
	}
}

type colorFunctionImage struct {
	bounds  image.Rectangle
	colorAt func(int, int) color.Color
}

func (i colorFunctionImage) ColorModel() color.Model { return color.RGBAModel }
func (i colorFunctionImage) Bounds() image.Rectangle { return i.bounds }
func (i colorFunctionImage) At(x, y int) color.Color { return i.colorAt(x, y) }
