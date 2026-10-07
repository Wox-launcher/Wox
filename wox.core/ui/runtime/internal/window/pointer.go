package window

func pointerPositionChanged(previous Point, current Point, known bool) bool {
	return !known || previous != current
}
