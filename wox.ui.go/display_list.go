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

// DisplayList records the drawing commands for one frame.
type DisplayList struct {
	clearColor Color
	commands   []displayCommand
}

type displayCommandKind uint8

const (
	displayCommandFillRoundedRect displayCommandKind = iota
	displayCommandDrawText
)

type displayCommand struct {
	kind   displayCommandKind
	rect   Rect
	radius float32
	color  Color
	text   string
	style  TextStyle
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
