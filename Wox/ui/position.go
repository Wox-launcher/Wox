package ui

import "wox/util/screen"

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
	x, y := getWindowMouseScreenLocation(800)
	return Position{
		Type: PositionTypeMouseScreen,
		X:    x,
		Y:    y,
	}
}

func getWindowMouseScreenLocation(windowWidth int) (int, int) {
	size := screen.GetMouseScreen()
	x := size.X + (size.Width-windowWidth)/2
	y := size.Height / 6
	return x, y
}
