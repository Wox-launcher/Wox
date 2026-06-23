package ui

// CommandType identifies a single draw operation in a command list.
// The native backend receives these as a flat array and executes them in order.
type CommandType int32

const (
	CmdClear CommandType = iota
	CmdDrawRect
	CmdDrawRoundedRect
	CmdDrawText
	CmdDrawImage
	CmdDrawLine
	CmdPushClip
	CmdPopClip
	CmdSetClipRect
)

// DrawCommand is a single rendering instruction.
// All coordinates are in logical pixels (DIP) relative to the window's client area.
// The native backend scales by the window's DPI factor when painting.
type DrawCommand struct {
	Type CommandType

	// Geometry — used by all drawing commands except Clear.
	X float32
	Y float32
	W float32
	H float32

	// Color — RGBA, used by DrawRect, DrawRoundedRect, DrawText, DrawLine.
	// Components are 0.0–1.0.
	R, G, B, A float32

	// Radius — corner radius for DrawRoundedRect.
	Radius float32

	// StrokeWidth — line width for DrawLine (0 = hairline).
	StrokeWidth float32

	// Text — UTF-8 string for DrawText.
	// Transferred to native as a length-prefixed byte blob (see CommandList).
	Text string

	// FontSize — point size for DrawText.
	FontSize float32

	// FontFamily — optional font family name for DrawText.
	// Empty string means "system default UI font".
	FontFamily string

	// ImageData — raw PNG bytes for DrawImage.
	// The native layer decodes and caches textures by pointer+length.
	ImageData []byte

	// ImageWidth / ImageHeight — source dimensions for DrawImage.
	// If 0, the native layer uses the decoded image's natural size.
	ImageWidth  float32
	ImageHeight float32
}

// CommandList is a flat sequence of draw commands produced by the Go layout engine.
// The native backend iterates this list once per frame.
type CommandList struct {
	Commands []DrawCommand
}

// Clear appends a fill-entire-window command with the given color.
func (cl *CommandList) Clear(r, g, b, a float32) {
	cl.Commands = append(cl.Commands, DrawCommand{
		Type: CmdClear,
		R: r, G: g, B: b, A: a,
	})
}

// DrawRect appends a filled rectangle.
func (cl *CommandList) DrawRect(x, y, w, h, r, g, b, a float32) {
	cl.Commands = append(cl.Commands, DrawCommand{
		Type: CmdDrawRect,
		X: x, Y: y, W: w, H: h,
		R: r, G: g, B: b, A: a,
	})
}

// DrawRoundedRect appends a filled rounded rectangle.
func (cl *CommandList) DrawRoundedRect(x, y, w, h, radius, r, g, b, a float32) {
	cl.Commands = append(cl.Commands, DrawCommand{
		Type: CmdDrawRoundedRect,
		X: x, Y: y, W: w, H: h,
		Radius: radius,
		R: r, G: g, B: b, A: a,
	})
}

// DrawText appends a text rendering command.
func (cl *CommandList) DrawText(x, y, w, h, r, g, b, a float32, text string, fontSize float32, fontFamily string) {
	cl.Commands = append(cl.Commands, DrawCommand{
		Type:       CmdDrawText,
		X: x, Y: y, W: w, H: h,
		R: r, G: g, B: b, A: a,
		Text:       text,
		FontSize:   fontSize,
		FontFamily: fontFamily,
	})
}

// DrawImage appends an image rendering command.
func (cl *CommandList) DrawImage(x, y, w, h float32, pngData []byte) {
	cl.Commands = append(cl.Commands, DrawCommand{
		Type:       CmdDrawImage,
		X: x, Y: y, W: w, H: h,
		ImageData:  pngData,
	})
}

// DrawLine appends a stroked line segment.
func (cl *CommandList) DrawLine(x1, y1, x2, y2, width, r, g, b, a float32) {
	cl.Commands = append(cl.Commands, DrawCommand{
		Type:        CmdDrawLine,
		X: x1, Y: y1, W: x2, H: y2,
		StrokeWidth: width,
		R: r, G: g, B: b, A: a,
	})
}

// PushClip appends a clip region command — subsequent commands are clipped
// to the given rect until PopClip.
func (cl *CommandList) PushClip(x, y, w, h float32) {
	cl.Commands = append(cl.Commands, DrawCommand{
		Type: CmdPushClip,
		X: x, Y: y, W: w, H: h,
	})
}

// PopClip restores the previous clip region.
func (cl *CommandList) PopClip() {
	cl.Commands = append(cl.Commands, DrawCommand{
		Type: CmdPopClip,
	})
}

// Reset clears the command list for reuse.
func (cl *CommandList) Reset() {
	cl.Commands = cl.Commands[:0]
}