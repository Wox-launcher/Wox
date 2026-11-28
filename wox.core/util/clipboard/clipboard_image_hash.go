package clipboard

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/draw"
)

// ImageHash fingerprints normalized pixels, independent of encoding and image origin.
func ImageHash(img image.Image) string {
	if img == nil || img.Bounds().Empty() {
		return ""
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	hasher := sha256.New()
	var dimensions [8]byte
	binary.LittleEndian.PutUint32(dimensions[0:4], uint32(width))
	binary.LittleEndian.PutUint32(dimensions[4:8], uint32(height))
	_, _ = hasher.Write(dimensions[:])

	rowBytes := width * 4
	switch src := img.(type) {
	case *image.NRGBA:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			offset := src.PixOffset(bounds.Min.X, y)
			_, _ = hasher.Write(src.Pix[offset : offset+rowBytes])
		}
	case *image.RGBA:
		// Unpremultiply row by row so large screenshots do not allocate a second full-frame NRGBA buffer.
		row := make([]byte, rowBytes)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			offset := src.PixOffset(bounds.Min.X, y)
			pix := src.Pix[offset : offset+rowBytes]
			for x := 0; x < rowBytes; x += 4 {
				alpha := uint32(pix[x+3])
				if alpha == 0 {
					row[x], row[x+1], row[x+2], row[x+3] = 0, 0, 0, 0
				} else if alpha == 255 {
					row[x], row[x+1], row[x+2], row[x+3] = pix[x], pix[x+1], pix[x+2], 255
				} else {
					row[x] = byte((uint32(pix[x]) * 65535 / alpha) >> 8)
					row[x+1] = byte((uint32(pix[x+1]) * 65535 / alpha) >> 8)
					row[x+2] = byte((uint32(pix[x+2]) * 65535 / alpha) >> 8)
					row[x+3] = byte(alpha)
				}
			}
			_, _ = hasher.Write(row)
		}
	default:
		row := image.NewNRGBA(image.Rect(0, 0, width, 1))
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			draw.Draw(row, row.Bounds(), img, image.Pt(bounds.Min.X, y), draw.Src)
			_, _ = hasher.Write(row.Pix)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
