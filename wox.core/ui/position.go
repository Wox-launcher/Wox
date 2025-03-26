package ui

import (
	"wox/setting"
	"wox/util/screen"
)

type Position struct {
	Type setting.PositionType
	X    int
	Y    int
}

func NewMouseScreenPosition(windowWidth int) Position {
	x, y := getWindowMouseScreenLocation(windowWidth)
	return Position{
		Type: setting.PositionTypeMouseScreen,
		X:    x,
		Y:    y,
	}
}

func NewActiveScreenPosition(windowWidth int) Position {
	x, y := getWindowActiveScreenLocation(windowWidth)
	return Position{
		Type: setting.PositionTypeActiveScreen,
		X:    x,
		Y:    y,
	}
}

func NewLastLocationPosition(x, y int) Position {
	return Position{
		Type: setting.PositionTypeLastLocation,
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

func getWindowActiveScreenLocation(windowWidth int) (int, int) {
	size := screen.GetActiveScreen()
	x := size.X + (size.Width-windowWidth)/2
	y := size.Height / 6
	return x, y
}
