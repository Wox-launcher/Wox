package screen

var lastMouseScreenDebug string

// LastMouseScreenDebug returns details about the latest Linux mouse-screen lookup.
func LastMouseScreenDebug() string {
	return lastMouseScreenDebug
}

// setLastMouseScreenDebug stores lookup details for nearby positioning logs.
func setLastMouseScreenDebug(debug string) {
	lastMouseScreenDebug = debug
}

type Size struct {
	Width  int
	Height int
	X      int
	Y      int
}

type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func (r Rect) Right() int {
	return r.X + r.Width
}

func (r Rect) Bottom() int {
	return r.Y + r.Height
}

func (r Rect) IsEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

type Display struct {
	ID            string
	Name          string
	Bounds        Rect
	WorkArea      Rect
	PixelBounds   Rect
	PixelWorkArea Rect
	Scale         float64
	Primary       bool
}

func ListDisplays() ([]Display, error) {
	return listDisplays()
}

func GetVirtualBounds(displays []Display) Rect {
	if len(displays) == 0 {
		return Rect{}
	}

	minX := displays[0].Bounds.X
	minY := displays[0].Bounds.Y
	maxRight := displays[0].Bounds.Right()
	maxBottom := displays[0].Bounds.Bottom()

	for i := 1; i < len(displays); i++ {
		bounds := displays[i].Bounds
		if bounds.X < minX {
			minX = bounds.X
		}
		if bounds.Y < minY {
			minY = bounds.Y
		}
		if bounds.Right() > maxRight {
			maxRight = bounds.Right()
		}
		if bounds.Bottom() > maxBottom {
			maxBottom = bounds.Bottom()
		}
	}

	return Rect{
		X:      minX,
		Y:      minY,
		Width:  maxRight - minX,
		Height: maxBottom - minY,
	}
}

func GetVirtualPixelBounds(displays []Display) Rect {
	if len(displays) == 0 {
		return Rect{}
	}

	minX := displays[0].PixelBounds.X
	minY := displays[0].PixelBounds.Y
	maxRight := displays[0].PixelBounds.Right()
	maxBottom := displays[0].PixelBounds.Bottom()

	for i := 1; i < len(displays); i++ {
		bounds := displays[i].PixelBounds
		if bounds.X < minX {
			minX = bounds.X
		}
		if bounds.Y < minY {
			minY = bounds.Y
		}
		if bounds.Right() > maxRight {
			maxRight = bounds.Right()
		}
		if bounds.Bottom() > maxBottom {
			maxBottom = bounds.Bottom()
		}
	}

	return Rect{
		X:      minX,
		Y:      minY,
		Width:  maxRight - minX,
		Height: maxBottom - minY,
	}
}
