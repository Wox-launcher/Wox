package ui

type PositionType string

const (
	PositionTypeMouseScreen  PositionType = "MouseScreen"
	PositionTypeLastLocation PositionType = "LastLocation"
)

type Position struct {
	Type PositionType
	X    int
	Y    int
}

func NewMouseScreenPosition() Position {
	x, y := getWindowShowLocation()
	return Position{
		Type: PositionTypeMouseScreen,
		X:    x,
		Y:    y,
	}
}
