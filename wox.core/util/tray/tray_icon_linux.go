package tray

import (
	"bytes"
	"image"
	"image/png"

	"github.com/disintegration/imaging"
)

// linuxTrayIconPixelSizes covers 1x and 2x tray slots. StatusNotifier hosts pick
// the pixmap whose pixel size best matches the current display scale.
var linuxTrayIconPixelSizes = []int{22, 24, 48, 64}

func decodeTrayIcon(data []byte) image.Image {
	if len(data) == 0 {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return img
}

func buildSNIIconPixmaps(img image.Image) []sniIconPixmap {
	if img == nil {
		return []sniIconPixmap{}
	}

	pixmaps := make([]sniIconPixmap, 0, len(linuxTrayIconPixelSizes))
	for _, size := range linuxTrayIconPixelSizes {
		resized := img
		if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
			resized = imaging.Resize(img, size, size, imaging.Lanczos)
		}
		pixels := encodeSNIIconPixels(resized)
		if len(pixels) != size*size*4 {
			continue
		}
		pixmaps = append(pixmaps, sniIconPixmap{
			Width:  int32(size),
			Height: int32(size),
			Pixels: pixels,
		})
	}
	return pixmaps
}

// encodeSNIIconPixels writes ARGB32 pixels in network byte order, as required by
// StatusNotifierItem IconPixmap.
func encodeSNIIconPixels(img image.Image) []byte {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	out := make([]byte, width*height*4)
	offset := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			out[offset] = byte(a >> 8)
			out[offset+1] = byte(r >> 8)
			out[offset+2] = byte(g >> 8)
			out[offset+3] = byte(b >> 8)
			offset += 4
		}
	}
	return out
}
