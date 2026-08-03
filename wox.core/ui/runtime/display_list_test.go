package woxui

import (
	"image"
	"math"
	"testing"
)

func TestDrawRotatedImageRecordsCenterRotation(t *testing.T) {
	image, err := NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	displayList := &DisplayList{}
	displayList.DrawRotatedImage(image, Rect{X: 10, Y: 20, Width: 30, Height: 40}, math.Pi/2)
	if len(displayList.commands) != 1 || displayList.commands[0].kind != displayCommandDrawImage || displayList.commands[0].rotation != math.Pi/2 {
		t.Fatalf("rotated image command = %+v, want one pi/2 image draw", displayList.commands)
	}
}

func TestDrawRotatedRoundedImageClampsCornerRadius(t *testing.T) {
	image, err := NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	displayList := &DisplayList{}
	displayList.DrawRotatedRoundedImage(image, Rect{Width: 30, Height: 40}, math.Pi/2, 100)
	if len(displayList.commands) != 1 || displayList.commands[0].radius != 15 {
		t.Fatalf("rounded image command = %+v, want radius clamped to 15", displayList.commands)
	}
}
