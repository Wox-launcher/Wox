package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WoxImageThumbnail draws image chrome using the current surface's foreground,
// including selected-row colors. Size is already scaled in logical units.
func WoxImageThumbnail(image *woxui.Image, size float32, foreground woxui.Color) woxwidget.Widget {
	background, border := foreground, foreground
	background.A = uint8(float32(foreground.A) * 0.08)
	border.A = uint8(float32(foreground.A) * 0.18)
	padding := size / 8
	contentSize := max(float32(0), size-2*padding)
	return woxwidget.Container{
		Width: size, Height: size, Radius: size / 8, Color: background,
		BorderColor: border, BorderWidth: size / 32,
		Child: woxwidget.Align{Width: size, Height: size, Horizontal: 0.5, Vertical: 0.5,
			Child: woxwidget.Image{Source: image, Width: contentSize, Height: contentSize, Fit: woxwidget.ImageFitContain},
		},
	}
}
