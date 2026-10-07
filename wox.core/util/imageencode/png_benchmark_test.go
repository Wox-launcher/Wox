package imageencode

import (
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"testing"
)

// BenchmarkPNGScreenshot measures a large desktop-like raster without filesystem or clipboard contention.
// WOX_BENCH_SCREENSHOT optionally supplies a real export for representative compression costs.
func BenchmarkPNGScreenshot(b *testing.B) {
	var source *image.RGBA
	if path := os.Getenv("WOX_BENCH_SCREENSHOT"); path != "" {
		file, err := os.Open(path)
		if err != nil {
			b.Fatal(err)
		}
		raster, decodeErr := png.Decode(file)
		file.Close()
		if decodeErr != nil {
			b.Fatal(decodeErr)
		}
		source = image.NewRGBA(raster.Bounds())
		draw.Draw(source, source.Bounds(), raster, raster.Bounds().Min, draw.Src)
	} else {
		source = image.NewRGBA(image.Rect(0, 0, 5136, 2792))
		for y := 0; y < source.Rect.Dy(); y++ {
			row := source.Pix[y*source.Stride : (y+1)*source.Stride]
			for x := 0; x < len(row); x += 4 {
				row[x], row[x+1], row[x+2], row[x+3] = byte(x/256), byte(y/32), byte((x/64+y/16)%256), 255
			}
		}
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(source.Pix)))
	b.ResetTimer()
	for b.Loop() {
		if err := PNG(io.Discard, source); err != nil {
			b.Fatal(err)
		}
	}
}
