package clipboard

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"testing"

	"golang.org/x/image/tiff"
)

// TestImageSnapshotDefersDecodeAndChecksPixelLimit verifies headers alone are sufficient for capture.
func TestImageSnapshotDefersDecodeAndChecksPixelLimit(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	snapshot, err := encodedImageSnapshot(data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if img, err := snapshot.Decode(12); err != nil || img.Bounds() != image.Rect(0, 0, 4, 3) {
		t.Fatalf("decode image = %v, %v", img, err)
	}
	// A truncated PNG still has a valid IHDR but cannot decode its pixels.
	snapshot, err = encodedImageSnapshot(data.Bytes()[:33])
	if err != nil {
		t.Fatalf("capture must not decode pixels: %v", err)
	}
	if _, err := snapshot.Decode(11); !errors.Is(err, ErrImageTooManyPixels) {
		t.Fatalf("pixel limit must precede decode, got %v", err)
	}
	if _, err := snapshot.Decode(12); err == nil {
		t.Fatal("truncated image should fail in the decoder")
	}
}

func TestImageSnapshotSupportsMacTIFF(t *testing.T) {
	var data bytes.Buffer
	if err := tiff.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 3)), nil); err != nil {
		t.Fatal(err)
	}
	snapshot, err := encodedImageSnapshot(data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if img, err := snapshot.Decode(6); err != nil || img.Bounds() != image.Rect(0, 0, 2, 3) {
		t.Fatalf("decode TIFF = %v, %v", img, err)
	}
}
