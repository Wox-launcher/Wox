//go:build !windows || !cgo

package imageencode

// filterPNGRow converts premultiplied edges and applies PNG's lossless Sub predictor.
func filterPNGRow(destination, source []byte, premultiplied bool, bytesPerPixel int) {
	copy(destination, source)
	if premultiplied {
		for x := 0; x < len(destination); x += 4 {
			alpha := uint32(destination[x+3])
			if alpha == 0 {
				clear(destination[x : x+3])
			} else if alpha != 255 {
				// Match color.NRGBAModel, including its 16-bit division before truncation.
				for channel := x; channel < x+3; channel++ {
					destination[channel] = byte((uint32(destination[channel]) * 65535 / alpha) >> 8)
				}
			}
		}
	}
	// Work backwards so predictors still contain the original, unfiltered bytes.
	for x := len(destination) - 1; x >= bytesPerPixel; x-- {
		destination[x] -= destination[x-bytesPerPixel]
	}
}
