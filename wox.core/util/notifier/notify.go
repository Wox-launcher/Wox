package notifier

import (
	"image"
	"image/color"
	"wox/common"
	"wox/util"

	"github.com/disintegration/imaging"
)

const notificationIconSize = 64

func Notify(icon image.Image, message string) {
	if message == "" {
		return
	}
	if icon == nil {
		img, _ := common.WoxIcon.ToImage()
		icon = img
	}

	util.Go(util.NewTraceContext(), "notifier.Notify", func() {
		ShowNotification(icon, message)
	})
}

func iconToBGRA(src image.Image, size int) ([]byte, int, int) {
	if src == nil || size <= 0 {
		return nil, 0, 0
	}

	resized := imaging.Resize(src, size, size, imaging.Lanczos)
	if resized == nil {
		return nil, 0, 0
	}

	b := resized.Bounds()
	if b.Dx() != size || b.Dy() != size {
		return nil, 0, 0
	}

	out := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := color.NRGBAModel.Convert(resized.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			a := uint32(c.A)
			r := uint32(c.R) * a / 255
			g := uint32(c.G) * a / 255
			bl := uint32(c.B) * a / 255

			i := (y*size + x) * 4
			out[i+0] = uint8(bl) // B (premultiplied)
			out[i+1] = uint8(g)  // G (premultiplied)
			out[i+2] = uint8(r)  // R (premultiplied)
			out[i+3] = c.A       // A
		}
	}

	return out, size, size
}
