package woxui

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
	clearColor Color
	commands   []displayCommand
	clipStack  []Rect
}

type displayCommandKind uint8

const (
	displayCommandFillRoundedRect displayCommandKind = iota
	displayCommandDrawText
	displayCommandDrawImage
	displayCommandSetClipRect
	displayCommandClearClip
)

type displayCommand struct {
	kind   displayCommandKind
	rect   Rect
	radius float32
	color  Color
	text   string
	style  TextStyle
	image  *Image
}

// PushClipRect intersects rect with the active clip for subsequent commands.
func (d *DisplayList) PushClipRect(rect Rect) {
	if len(d.clipStack) > 0 {
		rect = intersectRects(d.clipStack[len(d.clipStack)-1], rect)
	}
	d.clipStack = append(d.clipStack, rect)
	d.commands = append(d.commands, displayCommand{kind: displayCommandSetClipRect, rect: rect})
}

// PopClipRect restores the previous clip rectangle.
func (d *DisplayList) PopClipRect() {
	if len(d.clipStack) == 0 {
		return
	}
	d.clipStack = d.clipStack[:len(d.clipStack)-1]
	if len(d.clipStack) == 0 {
		d.commands = append(d.commands, displayCommand{kind: displayCommandClearClip})
		return
	}
	d.commands = append(d.commands, displayCommand{kind: displayCommandSetClipRect, rect: d.clipStack[len(d.clipStack)-1]})
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
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	if radius < 0 {
		radius = 0
	}
	d.commands = append(d.commands, displayCommand{
		kind:   displayCommandFillRoundedRect,
		rect:   rect,
		radius: radius,
		color:  color,
	})
}

// DrawText draws one non-wrapping line using the platform UI font.
func (d *DisplayList) DrawText(text string, rect Rect, style TextStyle, color Color) {
	if text == "" || rect.Width <= 0 || rect.Height <= 0 || style.Size <= 0 {
		return
	}
	if style.Weight != FontWeightRegular && style.Weight != FontWeightSemibold {
		style.Weight = FontWeightRegular
	}
	d.commands = append(d.commands, displayCommand{
		kind:  displayCommandDrawText,
		rect:  rect,
		color: color,
		text:  text,
		style: style,
	})
}

// DrawImage scales one immutable raster image into the destination rectangle.
func (d *DisplayList) DrawImage(image *Image, rect Rect) {
	if image == nil || image.Width <= 0 || image.Height <= 0 || len(image.pixels) == 0 || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	d.commands = append(d.commands, displayCommand{kind: displayCommandDrawImage, rect: rect, image: image})
}
