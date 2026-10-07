package graphics

// Color stores a straight-alpha sRGB color.
type Color struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

// Size describes an area in logical pixels.
type Size struct {
	Width  float32
	Height float32
}

// PixelSize describes a drawable surface in physical pixels.
type PixelSize struct {
	Width  int
	Height int
}

// Rect describes a drawing region in logical pixels, with a top-left origin.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Point describes a position or delta in logical pixels.
type Point struct {
	X float32
	Y float32
}
