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

func TestDisplayListCompareUsesRenderedImageContent(t *testing.T) {
	leftImage, err := NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	rightImage, err := NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	left := &DisplayList{}
	right := &DisplayList{}
	left.DrawImage(leftImage, Rect{Width: 10, Height: 10})
	right.DrawImage(rightImage, Rect{Width: 10, Height: 10})
	if err := left.Compare(right); err != nil {
		t.Fatalf("equivalent image commands differ: %v", err)
	}

	right.FillRect(Rect{Width: 1, Height: 1}, Color{R: 255, A: 255})
	if err := left.Compare(right); err == nil {
		t.Fatal("different command streams compared equal")
	}
}

func TestDisplayListCompareToleratesSubpixelFloatDrift(t *testing.T) {
	left := &DisplayList{commands: []displayCommand{{
		kind: displayCommandDrawText, rect: Rect{X: 37, Y: 18, Width: 172.104, Height: 17.291016}, radius: 4, stroke: 1,
		text: "echo command", style: TextStyle{Size: 13, Weight: FontWeightSemibold}, rotation: 0.25, points: []Point{{X: 10, Y: 20}},
	}}}
	right := &DisplayList{commands: []displayCommand{{
		kind: displayCommandDrawText, rect: Rect{X: 36.999985, Y: 18.000015, Width: 172.104, Height: 17.291016}, radius: 4.000015, stroke: 0.999985,
		text: "echo command", style: TextStyle{Size: 13.000015, Weight: FontWeightSemibold}, rotation: 0.250015, points: []Point{{X: 10.000015, Y: 19.999985}},
	}}}
	if err := left.Compare(right); err != nil {
		t.Fatalf("subpixel-equivalent command streams differ: %v", err)
	}

	right.commands[0].rect.X = 36.99
	if err := left.Compare(right); err == nil {
		t.Fatal("out-of-tolerance command geometry compared equal")
	}
}

func TestDisplayListCompareIncludesPortableFontTraits(t *testing.T) {
	base := &DisplayList{}
	base.DrawText("note", Rect{Width: 40, Height: 20}, TextStyle{Size: 13}, Color{A: 255})
	italic := &DisplayList{}
	italic.DrawText("note", Rect{Width: 40, Height: 20}, TextStyle{Size: 13, Italic: true}, Color{A: 255})
	monospace := &DisplayList{}
	monospace.DrawText("note", Rect{Width: 40, Height: 20}, TextStyle{Size: 13, Family: FontFamilyMonospace}, Color{A: 255})
	if base.Compare(italic) == nil || base.Compare(monospace) == nil {
		t.Fatal("font family and italic must invalidate retained text commands")
	}
}

func TestDisplayListCountsTextAndImageDraws(t *testing.T) {
	image, err := NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	displayList := &DisplayList{}
	displayList.FillRect(Rect{Width: 10, Height: 10}, Color{A: 255})
	displayList.DrawText("one", Rect{Width: 20, Height: 10}, TextStyle{Size: 12}, Color{A: 255})
	displayList.DrawText("two", Rect{X: 20, Width: 20, Height: 10}, TextStyle{Size: 12}, Color{A: 255})
	displayList.DrawImage(image, Rect{Width: 8, Height: 8})

	if displayList.TextDrawCount() != 2 || displayList.ImageDrawCount() != 1 {
		t.Fatalf("draw counts text=%d image=%d, want 2/1", displayList.TextDrawCount(), displayList.ImageDrawCount())
	}
	resources := displayList.EncodedRendererResources()
	if resources.TextRasterizations != 2 || resources.ImageCreates != 1 || resources.ImageUploads != 1 {
		t.Fatalf("encoded resources = %+v, want 2 text and 1 image", resources)
	}
}

func TestDisplayListDamageCullsNonIntersectingCommands(t *testing.T) {
	displayList := &DisplayList{}
	displayList.SetDamage(Rect{X: 10, Y: 10, Width: 20, Height: 20})
	displayList.FillRect(Rect{Width: 5, Height: 5}, Color{R: 255, A: 255})
	displayList.FillRect(Rect{X: 15, Y: 15, Width: 5, Height: 5}, Color{G: 255, A: 255})
	displayList.DrawText("outside", Rect{X: 40, Y: 40, Width: 20, Height: 10}, TextStyle{Size: 12}, Color{A: 255})
	displayList.DrawText("inside", Rect{X: 20, Y: 20, Width: 20, Height: 10}, TextStyle{Size: 12}, Color{A: 255})

	if len(displayList.commands) != 2 || displayList.commands[0].kind != displayCommandFillRoundedRect || displayList.commands[1].text != "inside" {
		t.Fatalf("damage commands = %+v, want only intersecting fill and text", displayList.commands)
	}
}

func TestEmbeddedSurfaceOverlayAlwaysRecordsSceneBoundary(t *testing.T) {
	displayList := &DisplayList{}
	displayList.SetDamage(Rect{X: 200, Y: 200, Width: 10, Height: 10})
	displayList.BeginEmbeddedSurfaceOverlay(Rect{Width: 100, Height: 100})
	if len(displayList.commands) != 1 || displayList.commands[0].kind != displayCommandBeginEmbeddedSurfaceOverlay {
		t.Fatalf("commands = %+v, want WebView scene boundary despite non-intersecting damage", displayList.commands)
	}
}

func TestDisplayListDamageHonorsCurrentClip(t *testing.T) {
	displayList := &DisplayList{}
	displayList.SetDamage(Rect{Width: 100, Height: 100})
	displayList.PushClipRect(Rect{Width: 10, Height: 10})
	displayList.FillRect(Rect{X: 20, Y: 20, Width: 10, Height: 10}, Color{A: 255})
	displayList.FillRect(Rect{X: 5, Y: 5, Width: 10, Height: 10}, Color{A: 255})
	displayList.PopClipRect()

	if len(displayList.commands) != 3 || displayList.commands[1].kind != displayCommandFillRoundedRect {
		t.Fatalf("clipped damage commands = %+v, want clip, intersecting fill, clear clip", displayList.commands)
	}
}

func TestDisplayListDamageUsesRotatedImageBounds(t *testing.T) {
	bitmap, err := NewImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	displayList := &DisplayList{}
	displayList.SetDamage(Rect{X: 19, Y: 5, Width: 2, Height: 2})
	displayList.DrawRotatedImage(bitmap, Rect{X: 10, Y: 10, Width: 20, Height: 2}, math.Pi/2)

	if len(displayList.commands) != 1 || displayList.commands[0].kind != displayCommandDrawImage {
		t.Fatalf("rotated damage commands = %+v, want conservative image command", displayList.commands)
	}
}

func TestDisplayListZeroDamageRecordsFullFrame(t *testing.T) {
	displayList := &DisplayList{}
	displayList.FillRect(Rect{X: 500, Y: 500, Width: 10, Height: 10}, Color{A: 255})
	if len(displayList.commands) != 1 {
		t.Fatalf("zero damage command count = %d, want 1", len(displayList.commands))
	}
}
