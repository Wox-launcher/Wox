package ui

// Color is an RGBA color with components in 0.0–1.0 range.
type Color struct {
	R, G, B, A float32
}

// RGB creates an opaque color.
func RGB(r, g, b float32) Color {
	return Color{R: r, G: g, B: b, A: 1.0}
}

// RGBA creates a color with alpha.
func RGBA(r, g, b, a float32) Color {
	return Color{R: r, G: g, B: b, A: a}
}

// Common color helpers for launcher UI.
var (
	ColorBackground     = RGB(0.094, 0.094, 0.094) // #181818
	ColorSurface        = RGB(0.118, 0.118, 0.118) // #1E1E1E
	ColorBorder         = RGBA(1, 1, 1, 0.08)
	ColorTextPrimary    = RGBA(1, 1, 1, 0.95)
	ColorTextSecondary  = RGBA(1, 1, 1, 0.40)
	ColorTextPlaceholder = RGBA(1, 1, 1, 0.30)
	ColorAccent         = RGB(0.2, 0.4, 0.8)
	ColorSelected       = RGBA(0.2, 0.4, 0.8, 0.3)
	ColorCursor         = RGBA(1, 1, 1, 0.8)
	ColorTransparent    = RGBA(0, 0, 0, 0)
)

// Theme holds the visual parameters that drive the draw command generation.
// Future versions will load this from Wox's JSON theme format.
type Theme struct {
	WindowBg       Color
	WindowRadius   float32
	QueryBoxBg     Color
	QueryBoxRadius float32
	QueryBoxHeight float32
	ListItemHeight float32
	ListItemGap    float32
	SelectedBg     Color
	TextPrimary    Color
	TextSecondary  Color
	TextPlaceholder Color
	CursorColor    Color
	FontSize       float32
	FontFamily     string
}

// DefaultTheme returns a dark theme matching the current Wox launcher look.
func DefaultTheme() Theme {
	return Theme{
		WindowBg:        ColorBackground,
		WindowRadius:    12,
		QueryBoxBg:      RGBA(1, 1, 1, 0.06),
		QueryBoxRadius:  8,
		QueryBoxHeight:  44,
		ListItemHeight:  48,
		ListItemGap:     0,
		SelectedBg:      ColorSelected,
		TextPrimary:     ColorTextPrimary,
		TextSecondary:   ColorTextSecondary,
		TextPlaceholder: ColorTextPlaceholder,
		CursorColor:     ColorCursor,
		FontSize:        16,
		FontFamily:      "", // system default
	}
}