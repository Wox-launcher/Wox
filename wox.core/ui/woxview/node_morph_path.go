package woxview

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/vector"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// PathCommand is a normalized SVG-style path segment. Coordinates are relative to a 1x1 view box.
type PathCommand struct {
	Op     PathOp    `json:"op"`
	Values []float64 `json:"values,omitempty"`
}

type PathOp string

const (
	PathMove  PathOp = "M"
	PathLine  PathOp = "L"
	PathQuad  PathOp = "Q"
	PathCubic PathOp = "C"
	PathClose PathOp = "Z"
)

func (PathOp) enumValues() []string { return []string{"M", "L", "Q", "C", "Z"} }

// MorphPathProps interpolates compatible vector outlines using the shared animation clock.
type MorphPathProps struct {
	Styled
	Keyframes []MorphPathKeyframe `json:"keyframes"`
	Progress  float32             `json:"progress,omitempty"`
	Duration  *int                `json:"duration,omitempty"`
	Curve     TransitionCurve     `json:"curve,omitempty"`
	digest    [32]byte
	segments  []morphSegment
}

// MorphPathKeyframe is an outline at a normalized point on the shared timeline.
type MorphPathKeyframe struct {
	At   float64       `json:"at"`
	Path []PathCommand `json:"path"`
}

type morphSegment struct {
	start, end   float64
	coefficients [][6][4]float64
}

func (p *MorphPathProps) prepare() error {
	if err := p.Style.validate(); err != nil {
		return err
	}
	if len(p.Keyframes) < 2 || len(p.Keyframes) > 18 || p.Keyframes[0].At != 0 || p.Keyframes[len(p.Keyframes)-1].At != 1 {
		return fmt.Errorf("morph-path requires 2 to 18 keyframes spanning 0 to 1")
	}
	first := p.Keyframes[0].Path
	if len(first) == 0 || len(first) > 128 || first[0].Op != PathMove {
		return fmt.Errorf("morph-path requires paths of 1 to 128 commands starting with M")
	}
	open := false
	for i, a := range first {
		count := 0
		ok := true
		switch a.Op {
		case PathMove, PathLine:
			count = 2
		case PathQuad:
			count = 4
		case PathCubic:
			count = 6
		case PathClose:
		default:
			ok = false
		}
		if !ok || len(a.Values) != count {
			return fmt.Errorf("morph-path command %d has invalid operation or coordinates", i)
		}
		if a.Op == PathMove {
			open = true
		} else if !open {
			return fmt.Errorf("morph-path command %d requires an open subpath", i)
		} else if a.Op == PathClose {
			open = false
		}
	}
	previous := -1.0
	for i, frame := range p.Keyframes {
		// Progress is float32; closer positions can be indistinguishable or make slopes overflow.
		if math.IsNaN(frame.At) || (i > 0 && frame.At-previous < 1e-7) || frame.At > 1 || len(frame.Path) != len(first) {
			return fmt.Errorf("morph-path keyframes require increasing positions at least 1e-7 apart and matching paths")
		}
		previous = frame.At
		for i, command := range frame.Path {
			if command.Op != first[i].Op || len(command.Values) != len(first[i].Values) {
				return fmt.Errorf("morph-path keyframe command %d has mismatched operation or coordinates", i)
			}
			for _, value := range command.Values {
				if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 4 {
					return fmt.Errorf("morph-path coordinates must be between -4 and 4")
				}
			}
		}
	}
	if math.IsNaN(float64(p.Progress)) || p.Progress < 0 || p.Progress > 1 {
		return fmt.Errorf("morph-path progress must be between 0 and 1")
	}
	if p.Duration != nil && (*p.Duration < 0 || *p.Duration > 60000) {
		return fmt.Errorf("morph-path duration must be between 0 and 60000 milliseconds")
	}
	if p.Curve != "" && p.Curve != TransitionLinear && p.Curve != TransitionEaseInOut && p.Curve != TransitionEaseOutBack {
		return fmt.Errorf("unknown morph-path curve %q", p.Curve)
	}
	encoded, _ := json.Marshal(p.Keyframes)
	p.digest = sha256.Sum256(encoded)
	p.prepareSegments()
	return nil
}

// prepareSegments precomputes monotone cubic coefficients so frame rendering only evaluates polynomials.
func (p *MorphPathProps) prepareSegments() {
	count := len(p.Keyframes)
	slope := func(i, command, coordinate int) float64 {
		value := func(j int) float64 { return p.Keyframes[j].Path[command].Values[coordinate] }
		if count == 2 {
			return value(1) - value(0)
		}
		if i == 0 || i == count-1 {
			return 0
		}
		a, b, c := p.Keyframes[i-1], p.Keyframes[i], p.Keyframes[i+1]
		left, right := (value(i)-value(i-1))/(b.At-a.At), (value(i+1)-value(i))/(c.At-b.At)
		if left*right <= 0 {
			return 0
		}
		w1, w2 := 2*(c.At-b.At)+(b.At-a.At), (c.At-b.At)+2*(b.At-a.At)
		return (w1 + w2) / (w1/left + w2/right)
	}
	p.segments = make([]morphSegment, count-1)
	for i := range p.segments {
		first, last := p.Keyframes[i], p.Keyframes[i+1]
		h := last.At - first.At
		segment := morphSegment{start: first.At, end: last.At, coefficients: make([][6][4]float64, len(first.Path))}
		for command, item := range first.Path {
			for coordinate, y0 := range item.Values {
				y1 := last.Path[command].Values[coordinate]
				m0, m1 := h*slope(i, command, coordinate), h*slope(i+1, command, coordinate)
				segment.coefficients[command][coordinate] = [4]float64{2*y0 - 2*y1 + m0 + m1, -3*y0 + 3*y1 - 2*m0 - m1, m0, y0}
			}
		}
		p.segments[i] = segment
	}
}

func (p *MorphPathProps) segmentAt(progress float64) (*morphSegment, float64) {
	index := 0
	for index < len(p.segments)-1 && progress > p.segments[index].end {
		index++
	}
	segment := &p.segments[index]
	return segment, (progress - segment.start) / (segment.end - segment.start)
}

func (s *morphSegment) coordinate(command, coordinate int, t float64) float64 {
	c := s.coefficients[command][coordinate]
	return ((c[0]*t+c[1])*t+c[2])*t + c[3]
}

// buildMorphPath rasterizes paths directly, avoiding per-frame SVG generation and parsing.
func buildMorphPath(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*MorphPathProps)
	style := p.Style.resolve(b.theme())
	width, height := style.Width, style.Height
	if width <= 0 {
		width = 24
	}
	if height <= 0 {
		height = width
	}
	duration, curve := (Transition{Duration: p.Duration, Curve: p.Curve}).timing()
	return widget.AnimatedFloat{Key: widget.Key(fmt.Sprintf("%p/%s/morph-path", b.view, id)), Target: p.Progress, Duration: duration, Curve: curve, Builder: func(value float32) widget.Widget {
		value = max(float32(0), min(float32(1), value))
		paint := style.Color.resolve(b.theme(), b.theme().Text)
		size := max(32, int(math.Ceil(float64(max(min(width, 512), min(height, 512))*2))))
		var keyData [40]byte
		copy(keyData[:32], p.digest[:])
		binary.LittleEndian.PutUint32(keyData[32:36], math.Float32bits(value))
		keyData[36], keyData[37], keyData[38], keyData[39] = paint.R, paint.G, paint.B, paint.A
		key := imageCacheKey{digest: sha256.Sum256(keyData[:]), rasterSize: size}
		entry := b.view.images.entries[key]
		if entry == nil {
			raster := vector.NewRasterizer(size, size)
			open := false
			segment, t := p.segmentAt(float64(value))
			for i, command := range p.Keyframes[0].Path {
				var v [6]float32
				for j := range command.Values {
					v[j] = float32(segment.coordinate(i, j, t)) * float32(size)
				}
				switch command.Op {
				case PathMove:
					if open {
						raster.ClosePath()
					}
					raster.MoveTo(v[0], v[1])
					open = true
				case PathLine:
					raster.LineTo(v[0], v[1])
				case PathQuad:
					raster.QuadTo(v[0], v[1], v[2], v[3])
				case PathCubic:
					raster.CubeTo(v[0], v[1], v[2], v[3], v[4], v[5])
				case PathClose:
					raster.ClosePath()
					open = false
				}
			}
			if open {
				raster.ClosePath()
			}
			pixels := image.NewRGBA(image.Rect(0, 0, size, size))
			raster.Draw(pixels, pixels.Bounds(), image.NewUniform(color.NRGBA{R: paint.R, G: paint.G, B: paint.B, A: paint.A}), image.Point{})
			resource, _ := ui.NewImageFromPackedRGBA(pixels)
			entry = &imageCacheEntry{image: resource}
			b.view.images.entries[key] = entry
		}
		entry.used = true
		resource := entry.image
		return style.wrapOuter(b.theme(), widget.Image{Source: resource, Width: width, Height: height, Fit: widget.ImageFitFill})
	}}
}
