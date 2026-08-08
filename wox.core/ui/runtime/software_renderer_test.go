package woxui

import (
	"bytes"
	"image"
	"testing"
)

func TestSoftwareRendererPartialFrameMatchesFullRedraw(t *testing.T) {
	background := Color{R: 12, G: 18, B: 24, A: 255}
	oldFrame := &DisplayList{}
	oldFrame.Clear(background)
	recordSoftwareRendererScene(oldFrame, Color{R: 220, G: 40, B: 20, A: 255})

	partialRenderer, err := NewSoftwareRenderer(64, 48)
	if err != nil {
		t.Fatal(err)
	}
	if err := partialRenderer.Render(oldFrame); err != nil {
		t.Fatal(err)
	}
	partial := &DisplayList{}
	partial.Clear(background)
	partial.SetDamage(Rect{X: 8, Y: 8, Width: 24, Height: 24})
	recordSoftwareRendererScene(partial, Color{R: 20, G: 180, B: 90, A: 255})
	if err := partialRenderer.Render(partial); err != nil {
		t.Fatal(err)
	}

	fullRenderer, err := NewSoftwareRenderer(64, 48)
	if err != nil {
		t.Fatal(err)
	}
	full := &DisplayList{}
	full.Clear(background)
	recordSoftwareRendererScene(full, Color{R: 20, G: 180, B: 90, A: 255})
	if err := fullRenderer.Render(full); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(partialRenderer.RGBA().Pix, fullRenderer.RGBA().Pix) {
		t.Fatal("partial retained rendering differs from a complete redraw")
	}
}

func recordSoftwareRendererScene(displayList *DisplayList, changing Color) {
	displayList.FillRoundedRect(Rect{X: 8, Y: 8, Width: 24, Height: 24}, 5, changing)
	displayList.StrokeRoundedRect(Rect{X: 10, Y: 10, Width: 20, Height: 20}, 3, 2, Color{R: 255, G: 255, B: 255, A: 180})
	displayList.FillConvexPolygon([]Point{{X: 38, Y: 8}, {X: 56, Y: 16}, {X: 40, Y: 28}}, Color{R: 40, G: 80, B: 220, A: 255})
	displayList.DrawText("Wox", Rect{X: 36, Y: 32, Width: 20, Height: 10}, TextStyle{Size: 10}, Color{R: 240, G: 240, B: 240, A: 255})
	bitmap := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for offset := 0; offset < len(bitmap.Pix); offset += 4 {
		copy(bitmap.Pix[offset:offset+4], []byte{255, 180, 20, 255})
	}
	portableImage, _ := NewImage(bitmap)
	displayList.DrawRotatedRoundedImage(portableImage, Rect{X: 2, Y: 36, Width: 12, Height: 8}, 0.2, 2)
}

func TestSoftwareRendererRGBAIsDetached(t *testing.T) {
	renderer, err := NewSoftwareRenderer(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	frame := &DisplayList{}
	frame.Clear(Color{R: 10, A: 255})
	if err := renderer.Render(frame); err != nil {
		t.Fatal(err)
	}
	first := renderer.RGBA()
	first.Pix[0] = 200
	if renderer.RGBA().Pix[0] != 10 {
		t.Fatal("RGBA exposed mutable retained pixels")
	}
}

func TestSoftwareRendererDamageClearReplacesTransparentPixels(t *testing.T) {
	renderer, err := NewSoftwareRenderer(4, 2)
	if err != nil {
		t.Fatal(err)
	}
	opaque := &DisplayList{}
	opaque.Clear(Color{R: 255, A: 255})
	if err := renderer.Render(opaque); err != nil {
		t.Fatal(err)
	}
	transparent := &DisplayList{}
	transparent.SetDamage(Rect{Width: 2, Height: 2})
	transparent.Clear(Color{})
	if err := renderer.Render(transparent); err != nil {
		t.Fatal(err)
	}
	pixels := renderer.RGBA().Pix
	if pixels[3] != 0 || pixels[2*4+3] != 255 {
		t.Fatalf("damage clear alpha = left %d right %d, want 0/255", pixels[3], pixels[2*4+3])
	}
}
