package screenshot

import (
	"errors"
	"image"
	"testing"
)

// TestScreenshotClipboardReadyFollowsSuccessfulPublication rejects premature success and failed clipboard writes.
func TestScreenshotClipboardReadyFollowsSuccessfulPublication(t *testing.T) {
	for _, fail := range []bool{false, true} {
		source := image.NewRGBA(image.Rect(0, 0, 2, 2))
		encoded := []byte("prepared PNG")
		published, notified := false, false
		write := func(got image.Image, png []byte) error {
			if notified || got != source || &png[0] != &encoded[0] {
				t.Fatal("notification ran before publication or the clipboard payload changed")
			}
			if fail {
				return errors.New("clipboard busy")
			}
			published = true
			return nil
		}
		err := publishScreenshotClipboard(write, source, encoded, func() {
			if !published {
				t.Fatal("success reported before the image was pasteable")
			}
			notified = true
		})
		if (err != nil) != fail || notified == fail {
			t.Fatalf("failure=%t: error=%v notified=%t", fail, err, notified)
		}
	}
}
