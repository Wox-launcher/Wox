package woxui

import (
	"fmt"
	stdimage "image"
	"math"
	"unicode/utf8"
)

// SoftwareRenderer is a deterministic retained RGBA target for damage correctness tests.
// It intentionally approximates platform text antialiasing while preserving command geometry,
// clipping, alpha compositing, and partial-frame retention.
type SoftwareRenderer struct {
	pixels *stdimage.RGBA
}

// NewSoftwareRenderer creates one persistent reference surface.
func NewSoftwareRenderer(width, height int) (*SoftwareRenderer, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("software renderer dimensions are invalid: %dx%d", width, height)
	}
	return &SoftwareRenderer{pixels: stdimage.NewRGBA(stdimage.Rect(0, 0, width, height))}, nil
}

// RGBA returns a detached snapshot of the current retained surface.
func (r *SoftwareRenderer) RGBA() *stdimage.RGBA {
	if r == nil || r.pixels == nil {
		return nil
	}
	clone := stdimage.NewRGBA(r.pixels.Bounds())
	copy(clone.Pix, r.pixels.Pix)
	return clone
}

// Render applies a full or damage-scoped display list to the retained surface.
func (r *SoftwareRenderer) Render(displayList *DisplayList) error {
	if r == nil || r.pixels == nil {
		return fmt.Errorf("software renderer is not initialized")
	}
	if displayList == nil {
		return fmt.Errorf("display list is nil")
	}
	surface := Rect{Width: float32(r.pixels.Bounds().Dx()), Height: float32(r.pixels.Bounds().Dy())}
	damage := displayList.damage
	if damage.Width <= 0 || damage.Height <= 0 {
		damage = surface
	} else {
		damage = intersectRects(surface, damage)
	}
	r.clear(damage, displayList.clearColor)
	var clip *Rect
	var renderErr error
	displayList.forEachCommand(func(command displayCommand) bool {
		switch command.kind {
		case displayCommandSetClipRect:
			value := command.rect
			clip = &value
		case displayCommandClearClip:
			clip = nil
		case displayCommandFillRoundedRect:
			r.fillRoundedRect(command.rect, command.radius, command.color, damage, clip)
		case displayCommandStrokeRoundedRect:
			r.strokeRoundedRect(command.rect, command.radius, command.stroke, command.color, damage, clip)
		case displayCommandFillConvexPolygon:
			r.fillConvexPolygon(command.points, command.rect, command.color, damage, clip)
		case displayCommandDrawText:
			r.drawReferenceText(command, damage, clip)
		case displayCommandDrawImage:
			r.drawImage(command, damage, clip)
		case displayCommandBeginEmbeddedSurfaceOverlay:
			// Native composition surfaces are not part of deterministic software output.
		default:
			renderErr = fmt.Errorf("unsupported display command kind %d", command.kind)
			return false
		}
		return true
	})
	return renderErr
}

func (r *SoftwareRenderer) fill(rect Rect, color Color, clip *Rect) {
	r.forEachPixel(rect, Rect{Width: float32(r.pixels.Bounds().Dx()), Height: float32(r.pixels.Bounds().Dy())}, clip, func(_, _ float32, offset int) {
		r.blendColor(offset, color)
	})
}

func (r *SoftwareRenderer) clear(rect Rect, color Color) {
	alpha := uint32(color.A)
	red := byte(uint32(color.R) * alpha / 255)
	green := byte(uint32(color.G) * alpha / 255)
	blue := byte(uint32(color.B) * alpha / 255)
	r.forEachPixel(rect, Rect{Width: float32(r.pixels.Bounds().Dx()), Height: float32(r.pixels.Bounds().Dy())}, nil, func(_, _ float32, offset int) {
		r.pixels.Pix[offset] = red
		r.pixels.Pix[offset+1] = green
		r.pixels.Pix[offset+2] = blue
		r.pixels.Pix[offset+3] = color.A
	})
}

func (r *SoftwareRenderer) fillRoundedRect(rect Rect, radius float32, color Color, damage Rect, clip *Rect) {
	radius = min(max(float32(0), radius), min(rect.Width, rect.Height)/2)
	r.forEachPixel(rect, damage, clip, func(x, y float32, offset int) {
		if pointInRoundedRect(x, y, rect, radius) {
			r.blendColor(offset, color)
		}
	})
}

func (r *SoftwareRenderer) strokeRoundedRect(rect Rect, radius, width float32, color Color, damage Rect, clip *Rect) {
	inner := Rect{X: rect.X + width, Y: rect.Y + width, Width: rect.Width - 2*width, Height: rect.Height - 2*width}
	outerRadius := min(max(float32(0), radius), min(rect.Width, rect.Height)/2)
	innerRadius := max(float32(0), outerRadius-width)
	r.forEachPixel(rect, damage, clip, func(x, y float32, offset int) {
		if pointInRoundedRect(x, y, rect, outerRadius) && (inner.Width <= 0 || inner.Height <= 0 || !pointInRoundedRect(x, y, inner, innerRadius)) {
			r.blendColor(offset, color)
		}
	})
}

func (r *SoftwareRenderer) fillConvexPolygon(points []Point, bounds Rect, color Color, damage Rect, clip *Rect) {
	r.forEachPixel(bounds, damage, clip, func(x, y float32, offset int) {
		if pointInConvexPolygon(x, y, points) {
			r.blendColor(offset, color)
		}
	})
}

// drawReferenceText uses deterministic bars instead of platform glyph rasterization.
func (r *SoftwareRenderer) drawReferenceText(command displayCommand, damage Rect, clip *Rect) {
	count := utf8.RuneCountInString(command.text)
	if count == 0 {
		return
	}
	barWidth := min(command.rect.Width/float32(count), max(float32(1), command.style.Size*0.5))
	barHeight := min(command.rect.Height, max(float32(1), command.style.Size*0.75))
	for index := 0; index < count; index++ {
		bar := Rect{X: command.rect.X + float32(index)*barWidth, Y: command.rect.Y + (command.rect.Height-barHeight)/2, Width: max(float32(1), barWidth-1), Height: barHeight}
		r.fillRoundedRect(bar, 0, command.color, damage, clip)
	}
}

func (r *SoftwareRenderer) drawImage(command displayCommand, damage Rect, clip *Rect) {
	bounds := rotatedRectBounds(command.rect, command.rotation)
	centerX := command.rect.X + command.rect.Width/2
	centerY := command.rect.Y + command.rect.Height/2
	sine := float32(math.Sin(float64(-command.rotation)))
	cosine := float32(math.Cos(float64(-command.rotation)))
	r.forEachPixel(bounds, damage, clip, func(x, y float32, offset int) {
		deltaX := x - centerX
		deltaY := y - centerY
		localX := deltaX*cosine - deltaY*sine + command.rect.Width/2
		localY := deltaX*sine + deltaY*cosine + command.rect.Height/2
		localRect := Rect{Width: command.rect.Width, Height: command.rect.Height}
		if !pointInRoundedRect(localX, localY, localRect, command.radius) {
			return
		}
		sourceX := min(command.image.Width-1, max(0, int(localX/command.rect.Width*float32(command.image.Width))))
		sourceY := min(command.image.Height-1, max(0, int(localY/command.rect.Height*float32(command.image.Height))))
		sourceOffset := (sourceY*command.image.Width + sourceX) * 4
		r.blendPremultiplied(offset, command.image.pixels[sourceOffset], command.image.pixels[sourceOffset+1], command.image.pixels[sourceOffset+2], command.image.pixels[sourceOffset+3])
	})
}

func (r *SoftwareRenderer) forEachPixel(bounds, damage Rect, clip *Rect, visit func(x, y float32, offset int)) {
	bounds = intersectRects(bounds, damage)
	if clip != nil {
		bounds = intersectRects(bounds, *clip)
	}
	left := max(0, int(math.Floor(float64(bounds.X))))
	top := max(0, int(math.Floor(float64(bounds.Y))))
	right := min(r.pixels.Bounds().Dx(), int(math.Ceil(float64(bounds.X+bounds.Width))))
	bottom := min(r.pixels.Bounds().Dy(), int(math.Ceil(float64(bounds.Y+bounds.Height))))
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			visit(float32(x)+0.5, float32(y)+0.5, y*r.pixels.Stride+x*4)
		}
	}
}

func (r *SoftwareRenderer) blendColor(offset int, color Color) {
	alpha := uint32(color.A)
	r.blendPremultiplied(offset, byte(uint32(color.R)*alpha/255), byte(uint32(color.G)*alpha/255), byte(uint32(color.B)*alpha/255), color.A)
}

func (r *SoftwareRenderer) blendPremultiplied(offset int, red, green, blue, alpha byte) {
	inverse := uint32(255 - alpha)
	r.pixels.Pix[offset] = byte(min(uint32(255), uint32(red)+uint32(r.pixels.Pix[offset])*inverse/255))
	r.pixels.Pix[offset+1] = byte(min(uint32(255), uint32(green)+uint32(r.pixels.Pix[offset+1])*inverse/255))
	r.pixels.Pix[offset+2] = byte(min(uint32(255), uint32(blue)+uint32(r.pixels.Pix[offset+2])*inverse/255))
	r.pixels.Pix[offset+3] = byte(min(uint32(255), uint32(alpha)+uint32(r.pixels.Pix[offset+3])*inverse/255))
}

func pointInRoundedRect(x, y float32, rect Rect, radius float32) bool {
	if x < rect.X || y < rect.Y || x >= rect.X+rect.Width || y >= rect.Y+rect.Height {
		return false
	}
	if radius <= 0 || (x >= rect.X+radius && x < rect.X+rect.Width-radius) || (y >= rect.Y+radius && y < rect.Y+rect.Height-radius) {
		return true
	}
	centerX := rect.X + radius
	if x >= rect.X+rect.Width-radius {
		centerX = rect.X + rect.Width - radius
	}
	centerY := rect.Y + radius
	if y >= rect.Y+rect.Height-radius {
		centerY = rect.Y + rect.Height - radius
	}
	deltaX := x - centerX
	deltaY := y - centerY
	return deltaX*deltaX+deltaY*deltaY <= radius*radius
}

func pointInConvexPolygon(x, y float32, points []Point) bool {
	direction := float32(0)
	for index, current := range points {
		next := points[(index+1)%len(points)]
		cross := (next.X-current.X)*(y-current.Y) - (next.Y-current.Y)*(x-current.X)
		if cross == 0 {
			continue
		}
		if direction != 0 && direction*cross < 0 {
			return false
		}
		direction = cross
	}
	return true
}
