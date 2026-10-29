package component

import (
	"math"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ReorderGhost draws the noninteractive label that follows a dragged item.
func ReorderGhost(label string, width, iconSize, scale float32, style woxui.TextStyle, textColor woxui.Color, theme ControlTheme) woxwidget.Widget {
	if scale <= 0 {
		scale = 1
	}
	metric := func(value float32) float32 { return float32(math.Round(float64(value * scale))) }
	background := theme.InputBackground
	if background.A == 0 {
		background = theme.Background
	}
	background.A = 230
	iconColor := theme.TextSecondary
	if iconColor.A == 0 {
		iconColor = theme.BodyText
	}
	return woxwidget.Container{
		Width: width, Height: iconSize + 2*metric(2), Radius: metric(6), Color: background,
		Padding: woxwidget.Insets{Left: metric(4), Right: metric(8), Top: metric(2), Bottom: metric(2)},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: metric(6), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Align{Width: iconSize, Height: iconSize, Horizontal: .5, Vertical: .5, Child: GripDotsGlyph(iconSize, iconColor)},
			woxwidget.Expanded{Child: woxwidget.TextBlock{Value: label, MaxLines: 1, ShrinkWrap: true, AlignmentY: .5, Style: style, Color: textColor}},
		}},
	}
}
