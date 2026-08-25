package system

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestScreenshotHistoryThumbnailHasWidth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 400, 200))); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if !screenshotHistoryThumbnailHasWidth(path, 400) {
		t.Fatal("matching thumbnail width must remain valid")
	}
	if screenshotHistoryThumbnailHasWidth(path, 1024) {
		t.Fatal("stale thumbnail width must be invalidated")
	}
}

func TestScreenshotHistoryImageExtensions(t *testing.T) {
	for _, path := range []string{"capture.png", "capture.jpg", "capture.JPEG"} {
		if !isScreenshotHistoryImage(path) {
			t.Fatalf("screenshot history rejected %s", path)
		}
	}
	if isScreenshotHistoryImage("capture.webp") {
		t.Fatal("screenshot history accepted an unsupported image")
	}
}

func TestNewScreenshotActionAllowsLauncherHide(t *testing.T) {
	result := (&ScreenshotPlugin{}).newScreenshotResult()
	if len(result.Actions) != 1 {
		t.Fatalf("screenshot action count = %d", len(result.Actions))
	}
	if result.Actions[0].PreventHideAfterAction {
		t.Fatal("new screenshot action must allow the launcher to hide")
	}
}
