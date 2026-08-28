//go:build darwin

package screenshot

import (
	"image"
	"image/color"
	"testing"
)

func TestDarwinScreenshotPixelAtPointMatchesPortableEditor(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 2))
	source.SetRGBA(1, 1, color.RGBA{R: 203, G: 174, B: 140, A: 255})
	prepared, err := NewImage(source)
	if err != nil {
		t.Fatalf("prepare screenshot image: %v", err)
	}

	frame := Size{Width: 100, Height: 100}
	wantX, wantY, _, wantOK := screenshotEditorPixelAtPoint(prepared, frame, Point{X: 25, Y: 50})
	gotX, gotY, gotOK := darwinScreenshotPixelAtPoint(prepared.Width, prepared.Height, frame, Point{X: 25, Y: 50})
	if gotOK != wantOK || gotX != wantX || gotY != wantY {
		t.Fatalf("native pixel = (%d, %d, %t), portable = (%d, %d, %t)", gotX, gotY, gotOK, wantX, wantY, wantOK)
	}

	if _, _, ok := darwinScreenshotPixelAtPoint(prepared.Width, prepared.Height, frame, Point{X: 100, Y: 50}); ok {
		t.Fatal("point outside the frame should not resolve a pixel")
	}
}

func TestDarwinScreenshotPixelAtPointUsesCaptureScale(t *testing.T) {
	x, y, ok := darwinScreenshotPixelAtPoint(2000, 1000, Size{Width: 1000, Height: 500}, Point{X: 400, Y: 300})
	if !ok || x != 800 || y != 600 {
		t.Fatalf("retina mapped pixel = (%d, %d, %t), want (800, 600, true)", x, y, ok)
	}
}

func TestDarwinScreenshotColorShortcutMatchesPortableEditorKeys(t *testing.T) {
	if asHex, ok := darwinScreenshotColorShortcut(5); !ok || asHex {
		t.Fatalf("G key code should copy RGB, got hex=%t ok=%t", asHex, ok)
	}
	if asHex, ok := darwinScreenshotColorShortcut(4); !ok || !asHex {
		t.Fatalf("H key code should copy HEX, got hex=%t ok=%t", asHex, ok)
	}
	if _, ok := darwinScreenshotColorShortcut(0); ok {
		t.Fatal("unrelated key codes should not copy a color")
	}
}

func TestDarwinScreenshotInspectorRectMatchesPortableEditor(t *testing.T) {
	panel := Size{Width: 150, Height: 138}
	frame := Size{Width: 800, Height: 600}
	for _, testCase := range []struct {
		name    string
		pointer Point
	}{
		{name: "lower-right", pointer: Point{X: 100, Y: 100}},
		{name: "flipped", pointer: Point{X: 790, Y: 590}},
	} {
		want := screenshotEditorInspectorRect(frame, testCase.pointer, panel, 1)
		got := darwinScreenshotInspectorRect(frame, testCase.pointer, panel, 1)
		if got != want {
			t.Fatalf("%s inspector = %+v, want %+v", testCase.name, got, want)
		}
	}
}
