package overlay

// Rect represents the position and size of a window or UI element.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// OverlayClickCallback is the callback function type for overlay clicks.
type OverlayClickCallback func()
