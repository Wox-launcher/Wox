package woxui

import (
	"bytes"
	"fmt"
	"math"
)

// FontWeight names portable text weights without exposing platform numeric values.
type FontWeight uint8

const (
	FontWeightRegular FontWeight = iota
	FontWeightSemibold
)

// TextStyle describes the portable subset needed by the initial text renderer.
type TextStyle struct {
	Size   float32
	Weight FontWeight
}

// TextMetrics describes one shaped line in logical pixels.
type TextMetrics struct {
	Size     Size
	Baseline float32
}

// DisplayList records the drawing commands for one frame.
type DisplayList struct {
	clearColor   Color
	commands     []displayCommand
	clipStack    []Rect
	frameID      uint64
	damage       Rect
	nativeDamage Rect
	frozen       bool
	stats        paintCommandStats
}

const displayListFloatTolerance = float32(1e-4)

// FrameMetricsID identifies this display list in its window's frame metrics stream.
func (d *DisplayList) FrameMetricsID() uint64 {
	if d == nil {
		return 0
	}
	return d.frameID
}

// AttachFrameMetricsID binds this display list to an already-started metrics frame.
func (d *DisplayList) AttachFrameMetricsID(frameID uint64) {
	if d != nil {
		d.frameID = frameID
	}
}

// CommandCount reports the recorded portable drawing command count.
func (d *DisplayList) CommandCount() int {
	if d == nil {
		return 0
	}
	return d.stats.commands
}

// TextDrawCount reports recorded DrawText commands.
func (d *DisplayList) TextDrawCount() int {
	if d == nil {
		return 0
	}
	return d.stats.texts
}

// ImageDrawCount reports recorded DrawImage commands.
func (d *DisplayList) ImageDrawCount() int {
	if d == nil {
		return 0
	}
	return d.stats.images
}

func (d *DisplayList) appendCommand(command displayCommand) {
	if d == nil {
		return
	}
	d.commands = append(d.commands, command)
	d.stats.add(command)
}

// EncodedRendererResources reports the current uncached encode cost of this command stream.
// Native caches later replace these baseline create/upload counts with hit/miss accounting.
func (d *DisplayList) EncodedRendererResources() FrameRendererResourceMetrics {
	text := d.TextDrawCount()
	images := d.ImageDrawCount()
	return FrameRendererResourceMetrics{
		TextRasterizations: text,
		ImageCreates:       images,
		ImageUploads:       images,
	}
}

// SetDamage limits subsequent drawing commands to one logical redraw region; zero means full frame.
func (d *DisplayList) SetDamage(rect Rect) {
	if d == nil {
		return
	}
	d.damage = rect
}

// Damage returns the logical redraw region associated with this command stream.
func (d *DisplayList) Damage() Rect {
	if d == nil {
		return Rect{}
	}
	return d.damage
}

// SetNativeDamage stores the final logical region consumed by a retaining native surface.
func (d *DisplayList) SetNativeDamage(rect Rect) {
	if d != nil {
		d.nativeDamage = rect
	}
}

// NativeDamage returns the final native redraw region; zero means full frame.
func (d *DisplayList) NativeDamage() Rect {
	if d == nil {
		return Rect{}
	}
	return d.nativeDamage
}

// Compare reports the first rendering difference between two portable command streams.
func (d *DisplayList) Compare(other *DisplayList) error {
	if d == nil || other == nil {
		if d == other {
			return nil
		}
		return fmt.Errorf("one display list is nil")
	}
	if d.clearColor != other.clearColor {
		return fmt.Errorf("clear colors differ: %+v != %+v", d.clearColor, other.clearColor)
	}
	left := d.flattenedCommands()
	right := other.flattenedCommands()
	if len(left) != len(right) {
		return fmt.Errorf("command counts differ: %d != %d", len(left), len(right))
	}
	for index := range left {
		if !displayCommandsEqual(left[index], right[index]) {
			return fmt.Errorf("command %d differs", index)
		}
	}
	return nil
}

func displayCommandsEqual(left, right displayCommand) bool {
	if left.kind != right.kind || !displayListRectsEqual(left.rect, right.rect) ||
		!displayListFloatsEqual(left.radius, right.radius) || !displayListFloatsEqual(left.stroke, right.stroke) ||
		left.color != right.color || left.text != right.text || left.style.Weight != right.style.Weight ||
		!displayListFloatsEqual(left.style.Size, right.style.Size) || !displayListFloatsEqual(left.rotation, right.rotation) {
		return false
	}
	if len(left.points) != len(right.points) {
		return false
	}
	for index := range left.points {
		if !displayListFloatsEqual(left.points[index].X, right.points[index].X) || !displayListFloatsEqual(left.points[index].Y, right.points[index].Y) {
			return false
		}
	}
	return imagesRenderEqual(left.image, right.image)
}

// displayListRectsEqual ignores sub-pixel float32 drift that cannot affect rendered output.
func displayListRectsEqual(left, right Rect) bool {
	return displayListFloatsEqual(left.X, right.X) && displayListFloatsEqual(left.Y, right.Y) &&
		displayListFloatsEqual(left.Width, right.Width) && displayListFloatsEqual(left.Height, right.Height)
}

func displayListFloatsEqual(left, right float32) bool {
	return float32(math.Abs(float64(left-right))) <= displayListFloatTolerance
}

func imagesRenderEqual(left, right *Image) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Width == right.Width && left.Height == right.Height && bytes.Equal(left.pixels, right.pixels)
}

type displayCommandKind uint8

// MaxConvexPolygonPoints is the portable vertex limit shared by every native renderer.
const MaxConvexPolygonPoints = 16

const (
	displayCommandFillRoundedRect displayCommandKind = iota
	displayCommandFillConvexPolygon
	displayCommandStrokeRoundedRect
	displayCommandDrawText
	displayCommandDrawImage
	displayCommandBeginEmbeddedSurfaceOverlay
	displayCommandSetClipRect
	displayCommandClearClip
	displayCommandPaintSegment
)

type displayCommand struct {
	kind     displayCommandKind
	rect     Rect
	radius   float32
	stroke   float32
	color    Color
	text     string
	style    TextStyle
	image    *Image
	rotation float32
	points   []Point
	segment  *PaintSegment
}

// BeginEmbeddedSurfaceOverlay splits portable drawing around a platform-owned composition surface.
func (d *DisplayList) BeginEmbeddedSurfaceOverlay(rect Rect) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	d.appendCommand(displayCommand{kind: displayCommandBeginEmbeddedSurfaceOverlay, rect: rect})
}

// FillConvexPolygon fills 3 to MaxConvexPolygonPoints ordered vertices with portable edge antialiasing.
func (d *DisplayList) FillConvexPolygon(points []Point, color Color) {
	if len(points) < 3 || len(points) > MaxConvexPolygonPoints {
		return
	}
	minX, maxX := points[0].X, points[0].X
	minY, maxY := points[0].Y, points[0].Y
	turn := float32(0)
	for index, current := range points {
		next := points[(index+1)%len(points)]
		after := points[(index+2)%len(points)]
		if current == next {
			return
		}
		cross := (next.X-current.X)*(after.Y-next.Y) - (next.Y-current.Y)*(after.X-next.X)
		if cross != 0 {
			if turn != 0 && turn*cross < 0 {
				return
			}
			turn = cross
		}
		minX = min(minX, current.X)
		maxX = max(maxX, current.X)
		minY = min(minY, current.Y)
		maxY = max(maxY, current.Y)
	}
	if turn == 0 || maxX <= minX || maxY <= minY {
		return
	}
	bounds := Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
	if !d.shouldRecord(bounds) {
		return
	}
	immutablePoints := append([]Point(nil), points...)
	d.appendCommand(displayCommand{
		kind: displayCommandFillConvexPolygon, rect: bounds, color: color, points: immutablePoints,
	})
}

// StrokeRoundedRect draws an inset border without filling the interior.
func (d *DisplayList) StrokeRoundedRect(rect Rect, radius, width float32, color Color) {
	if rect.Width <= 0 || rect.Height <= 0 || width <= 0 || !d.shouldRecord(rect) {
		return
	}
	d.appendCommand(displayCommand{
		kind:   displayCommandStrokeRoundedRect,
		rect:   rect,
		radius: max(float32(0), radius),
		stroke: min(width, min(rect.Width, rect.Height)/2),
		color:  color,
	})
}

// PushClipRect intersects rect with the active clip for subsequent commands.
func (d *DisplayList) PushClipRect(rect Rect) {
	if len(d.clipStack) > 0 {
		rect = intersectRects(d.clipStack[len(d.clipStack)-1], rect)
	}
	d.clipStack = append(d.clipStack, rect)
	d.appendCommand(displayCommand{kind: displayCommandSetClipRect, rect: rect})
}

// PopClipRect restores the previous clip rectangle.
func (d *DisplayList) PopClipRect() {
	if len(d.clipStack) == 0 {
		return
	}
	d.clipStack = d.clipStack[:len(d.clipStack)-1]
	if len(d.clipStack) == 0 {
		d.appendCommand(displayCommand{kind: displayCommandClearClip})
		return
	}
	d.appendCommand(displayCommand{kind: displayCommandSetClipRect, rect: d.clipStack[len(d.clipStack)-1]})
}

// ClipRect returns the effective clip while widgets record the current subtree.
func (d *DisplayList) ClipRect() (Rect, bool) {
	if len(d.clipStack) == 0 {
		return Rect{}, false
	}
	return d.clipStack[len(d.clipStack)-1], true
}

func intersectRects(left, right Rect) Rect {
	x := max(left.X, right.X)
	y := max(left.Y, right.Y)
	rightEdge := min(left.X+left.Width, right.X+right.Width)
	bottomEdge := min(left.Y+left.Height, right.Y+right.Height)
	return Rect{X: x, Y: y, Width: max(float32(0), rightEdge-x), Height: max(float32(0), bottomEdge-y)}
}

// Clear replaces the entire frame with color.
func (d *DisplayList) Clear(color Color) {
	d.clearColor = color
}

// FillRect fills an axis-aligned rectangle.
func (d *DisplayList) FillRect(rect Rect, color Color) {
	d.FillRoundedRect(rect, 0, color)
}

// FillRoundedRect fills an axis-aligned rectangle with a uniform corner radius.
func (d *DisplayList) FillRoundedRect(rect Rect, radius float32, color Color) {
	if rect.Width <= 0 || rect.Height <= 0 || !d.shouldRecord(rect) {
		return
	}
	if radius < 0 {
		radius = 0
	}
	d.appendCommand(displayCommand{
		kind:   displayCommandFillRoundedRect,
		rect:   rect,
		radius: radius,
		color:  color,
	})
}

// DrawText draws one non-wrapping line using the platform UI font.
func (d *DisplayList) DrawText(text string, rect Rect, style TextStyle, color Color) {
	if text == "" || rect.Width <= 0 || rect.Height <= 0 || style.Size <= 0 || !d.shouldRecord(rect) {
		return
	}
	if style.Weight != FontWeightRegular && style.Weight != FontWeightSemibold {
		style.Weight = FontWeightRegular
	}
	d.appendCommand(displayCommand{
		kind:  displayCommandDrawText,
		rect:  rect,
		color: color,
		text:  text,
		style: style,
	})
}

// DrawImage scales one immutable raster image into the destination rectangle.
func (d *DisplayList) DrawImage(image *Image, rect Rect) {
	d.DrawRotatedImage(image, rect, 0)
}

// DrawRotatedImage scales an image into rect and rotates it around the destination center.
func (d *DisplayList) DrawRotatedImage(image *Image, rect Rect, radians float32) {
	d.DrawRotatedRoundedImage(image, rect, radians, 0)
}

// DrawRotatedRoundedImage scales, clips, and rotates an image around the destination center.
func (d *DisplayList) DrawRotatedRoundedImage(image *Image, rect Rect, radians, radius float32) {
	if image == nil || image.Width <= 0 || image.Height <= 0 || len(image.pixels) == 0 || rect.Width <= 0 || rect.Height <= 0 || !d.shouldRecord(rotatedRectBounds(rect, radians)) {
		return
	}
	d.appendCommand(displayCommand{kind: displayCommandDrawImage, rect: rect, image: image, rotation: radians, radius: min(max(float32(0), radius), min(rect.Width, rect.Height)/2)})
}

func (d *DisplayList) shouldRecord(rect Rect) bool {
	if d == nil || d.damage.Width <= 0 || d.damage.Height <= 0 {
		return true
	}
	if !rectsOverlap(rect, d.damage) {
		return false
	}
	if len(d.clipStack) > 0 && !rectsOverlap(rect, d.clipStack[len(d.clipStack)-1]) {
		return false
	}
	return true
}

func rectsOverlap(left, right Rect) bool {
	return left.Width > 0 && left.Height > 0 && right.Width > 0 && right.Height > 0 && left.X < right.X+right.Width && right.X < left.X+left.Width && left.Y < right.Y+right.Height && right.Y < left.Y+left.Height
}

func rotatedRectBounds(rect Rect, radians float32) Rect {
	if radians == 0 {
		return rect
	}
	sine := math.Abs(math.Sin(float64(radians)))
	cosine := math.Abs(math.Cos(float64(radians)))
	width := float32(float64(rect.Width)*cosine + float64(rect.Height)*sine)
	height := float32(float64(rect.Width)*sine + float64(rect.Height)*cosine)
	centerX := rect.X + rect.Width/2
	centerY := rect.Y + rect.Height/2
	return Rect{X: centerX - width/2, Y: centerY - height/2, Width: width, Height: height}
}
