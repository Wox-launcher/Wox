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
	normalized := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(normalized, normalized.Bounds(), img, bounds.Min, draw.Src)
	hasher := sha256.New()
	var dimensions [8]byte
	binary.LittleEndian.PutUint32(dimensions[0:4], uint32(bounds.Dx()))
	binary.LittleEndian.PutUint32(dimensions[4:8], uint32(bounds.Dy()))
	_, _ = hasher.Write(dimensions[:])
	_, _ = hasher.Write(normalized.Pix)
	return hex.EncodeToString(hasher.Sum(nil))
}
