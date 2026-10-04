package component

import (
	"testing"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestImageThumbnailSurfaceAndScaling(t *testing.T) {
	image := &woxui.Image{Width: 571, Height: 121}
	for _, foreground := range []woxui.Color{{A: 255}, {R: 255, G: 255, B: 255, A: 255}, {R: 220, G: 230, B: 255, A: 255}} {
		for _, size := range []float32{32, 40, 48, 64} {
			tile := WoxImageThumbnail(image, size, foreground).(woxwidget.Container)
			if tile.Color.R != foreground.R || tile.BorderColor.B != foreground.B || tile.Color.A == 0 || tile.BorderColor.A <= tile.Color.A {
				t.Fatal("thumbnail chrome must follow the surface foreground with a stronger outline")
			}
			aligned := tile.Child.(woxwidget.Align)
			content := aligned.Child.(woxwidget.Image)
			if tile.Width != size || tile.Radius != size/8 || tile.BorderWidth != size/32 || aligned.Horizontal != 0.5 || aligned.Vertical != 0.5 || content.Width != size*0.75 || content.Fit != woxwidget.ImageFitContain || content.Source != image {
				t.Fatal("thumbnail geometry must scale and preserve centered, uncropped content")
			}
		}
	}
}
