package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const fadingGridSpacing = float32(28)
const fadingGridSegment = float32(7)

// FadingGridProps defines the grid bounds and its elliptical focal area.
type FadingGridProps struct {
	Width   float32
	Height  float32
	CenterX float32
	CenterY float32
	RadiusX float32
	RadiusY float32
	Color   woxui.Color
}

// FadingGrid draws a quiet grid whose outer rows are fully transparent.
func FadingGrid(props FadingGridProps) woxwidget.Widget {
	centerX := props.CenterX
	if centerX == 0 {
		centerX = props.Width / 2
	}
	centerY := props.CenterY
	if centerY == 0 {
		centerY = props.Height / 2
	}
	radiusX := props.RadiusX
	if radiusX == 0 {
		radiusX = fadingGridRadius(centerX, props.Width)
	}
	radiusY := props.RadiusY
	if radiusY == 0 {
		radiusY = fadingGridRadius(centerY, props.Height)
	}
	return woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		gridCenterX := bounds.X + centerX
		gridCenterY := bounds.Y + centerY
		for x := bounds.X; x <= bounds.X+bounds.Width; x += fadingGridSpacing {
			for y := bounds.Y; y < bounds.Y+bounds.Height; y += fadingGridSpacing {
				for offset := float32(0); offset < fadingGridSpacing; offset += fadingGridSegment {
					verticalHeight := min(fadingGridSegment, bounds.Y+bounds.Height-y-offset)
					if verticalHeight > 0 {
						alpha := fadingGridAlpha(x-gridCenterX, y+offset+3-gridCenterY, radiusX, radiusY)
						displayList.FillRect(woxui.Rect{X: x, Y: y + offset, Width: 1, Height: verticalHeight}, withAlpha(props.Color, alpha))
					}
					horizontalWidth := min(fadingGridSegment, bounds.X+bounds.Width-x-offset)
					if horizontalWidth > 0 {
						alpha := fadingGridAlpha(x+offset+3-gridCenterX, y-gridCenterY, radiusX, radiusY)
						displayList.FillRect(woxui.Rect{X: x + offset, Y: y, Width: horizontalWidth, Height: 1}, withAlpha(props.Color, alpha))
					}
				}
			}
		}
	}}
}

func fadingGridRadius(center, length float32) float32 {
	return max(fadingGridSpacing, min(center, length-center)-fadingGridSpacing*2)
}

func fadingGridAlpha(offsetX, offsetY, radiusX, radiusY float32) uint8 {
	normalizedX := offsetX / radiusX
	normalizedY := offsetY / radiusY
	strength := max(float32(0), 1-normalizedX*normalizedX-normalizedY*normalizedY)
	return uint8(72 * strength * strength)
}
