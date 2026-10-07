//go:build windows

package clipboard

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// TestPreparedImagePublishAndTeardown serializes invalid-owner commits against cleanup without touching the live clipboard.
func TestPreparedImagePublishAndTeardown(t *testing.T) {
	prepared, err := PrepareImage(image.NewRGBA(image.Rect(0, 0, 2, 2)), nil)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() { _ = prepared.Publish(0) })
		workers.Go(prepared.Close)
	}
	workers.Wait()
	if prepared.native != nil || prepared.payload != nil {
		t.Fatal("teardown retained prepared native formats")
	}
	if err := prepared.Publish(0); err == nil {
		t.Fatal("closed formats were published")
	}
	if err := Flush(nil); err != nil {
		t.Fatal("eager Windows formats required a running UI dispatcher at exit")
	}
}

// TestWindowsClipboardPreparationLifetime rejects short rows and frees prepared handles once across teardown callers.
func TestWindowsClipboardPreparationLifetime(t *testing.T) {
	for _, payload := range []*clipboardImage{
		nil,
		{width: 0, height: 1, stride: 4, pixels: make([]byte, 4)},
		{width: 1, height: 0, stride: 4, pixels: make([]byte, 4)},
		{width: 16385, height: 1, stride: 16385 * 4, pixels: make([]byte, 16385*4)},
		{width: 1, height: 1, stride: 3, pixels: make([]byte, 4)},
		{width: 1, height: 2, stride: 8, pixels: make([]byte, 11)},
		{width: 1, height: 3, stride: int(^uint(0) >> 1), pixels: make([]byte, 4)},
	} {
		if prepared, err := prepareImagePayload(payload); err == nil || prepared != nil {
			t.Fatal("invalid clipboard buffer reached native preparation")
		}
	}
	prepared, err := prepareImagePayload(&clipboardImage{width: 1, height: 2, stride: 8, pixels: make([]byte, 12)})
	if err != nil {
		t.Fatal(err)
	}
	var closed sync.WaitGroup
	for range 8 {
		closed.Go(prepared.Close)
	}
	closed.Wait()
	if prepared.native != nil {
		t.Fatal("closed clipboard preparation retained native allocations")
	}
}

// TestWindowsClipboardDIB checks native row order and alpha against Go's existing conversion, without clipboard publication.
func TestWindowsClipboardDIB(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "clipboard-formats.exe")
	command := exec.Command("g++", "-std=c++17", "-O2", "testdata/clipboard_image_windows.cpp", "-luser32", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native clipboard test: %v\n%s", err, output)
	}
	for _, straight := range []bool{false, true} {
		bounds := image.Rect(-256, -3, 0, 0)
		var source image.Image
		var pixels []byte
		if straight {
			raster := image.NewNRGBA(image.Rect(-256, -3, 5, 0)).SubImage(bounds).(*image.NRGBA)
			source, pixels = raster, raster.Pix
		} else {
			raster := image.NewRGBA(image.Rect(-256, -3, 5, 0)).SubImage(bounds).(*image.RGBA)
			source, pixels = raster, raster.Pix
		}
		for y := 0; y < 3; y++ {
			for x := 0; x < 256; x++ {
				offset := y*261*4 + x*4
				if straight {
					copy(pixels[offset:offset+4], []byte{byte(255 - x), byte(x / 2), byte(y * 100), byte(x)})
				} else {
					copy(pixels[offset:offset+4], []byte{byte(x / (y + 1)), byte(x / 2), byte(x), byte(x)})
				}
			}
		}
		// Check the platform handoff retains cropped origins, stride, and alpha semantics.
		payload, err := newClipboardImage(source, []byte{137, 80, 78, 71})
		if err != nil {
			t.Fatal(err)
		}
		if payload.stride != 261*4 || payload.premultiplied == straight || &payload.pixels[0] != &pixels[0] {
			t.Fatalf("packed clipboard handoff lost stride or alpha semantics: straight=%t", straight)
		}
		command = exec.Command(binary)
		if straight {
			command.Args = append(command.Args, "straight")
		}
		command.Stdin = bytes.NewReader(payload.pixels[:(3-1)*payload.stride+256*4])
		got, err := command.Output()
		if err != nil {
			t.Fatalf("prepare native clipboard formats: %v", err)
		}
		want := image.NewNRGBA(image.Rect(0, 0, 256, 3))
		draw.Draw(want, want.Bounds(), source, bounds.Min, draw.Src)
		if len(got) != len(want.Pix) {
			t.Fatalf("DIB pixel length = %d, want %d", len(got), len(want.Pix))
		}
		for y := 0; y < 3; y++ {
			for x := 0; x < 256; x++ {
				offset := (2-y)*256*4 + x*4
				pixel := color.NRGBA{R: got[offset+2], G: got[offset+1], B: got[offset], A: got[offset+3]}
				if pixel != want.NRGBAAt(x, y) {
					t.Fatalf("straight=%t pixel (%d,%d) = %v, want %v", straight, x, y, pixel, want.NRGBAAt(x, y))
				}
			}
		}
	}
}

// BenchmarkWindowsClipboardPreparation isolates the former intermediate conversion from native format preparation.
func BenchmarkWindowsClipboardPreparation(b *testing.B) {
	source := image.NewRGBA(image.Rect(0, 0, 5136, 2792))
	draw.Draw(source, source.Bounds(), image.NewUniform(color.RGBA{R: 77, G: 91, B: 128, A: 255}), image.Point{}, draw.Src)
	for _, normalize := range []bool{true, false} {
		name := "packed"
		if normalize {
			name = "intermediate_NRGBA"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				payload := &clipboardImage{width: source.Rect.Dx(), height: source.Rect.Dy(), stride: source.Stride, pixels: source.Pix, premultiplied: true}
				if normalize {
					raster := image.NewNRGBA(source.Rect)
					draw.Draw(raster, raster.Bounds(), source, source.Rect.Min, draw.Src)
					payload.pixels, payload.stride, payload.premultiplied = raster.Pix, raster.Stride, false
				}
				prepared, err := prepareImagePayload(payload)
				if err != nil {
					b.Fatal(err)
				}
				prepared.Close()
			}
		})
	}
}
