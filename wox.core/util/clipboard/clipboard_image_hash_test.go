package clipboard

import (
	"image"
	"image/color"
	"testing"
)

func TestImageHashPreservesClipboardFingerprint(t *testing.T) {
	// Keep the historical little-endian dimensions + straight-alpha pixels hash stable,
	// including nonzero origins and parent-image stride.
	parent := image.NewNRGBA(image.Rect(-5, -3, 5, 4))
	parent.SetNRGBA(-2, -1, color.NRGBA{R: 255, A: 255})
	parent.SetNRGBA(-1, -1, color.NRGBA{G: 255, A: 128})
	img := parent.SubImage(image.Rect(-2, -1, 0, 0))
	if got := ImageHash(img); got != "70af0d8331da0ad16cfa3f85ee128e4a9100f1465297a9629a633e8904c74b84" {
		t.Fatalf("clipboard fingerprint changed: %s", got)
	}
	if ImageHash(nil) != "" || ImageHash(image.NewRGBA(image.Rectangle{})) != "" {
		t.Fatal("empty image gained a fingerprint")
	}
}
