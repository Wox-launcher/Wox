package woxview

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/common"
	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestSVGFitPreservesSourceAspect checks actual paint at fractional and integer display scales.
func TestSVGFitPreservesSourceAspect(t *testing.T) {
	for _, fit := range []string{"contain", "cover", "fill"} {
		for _, scale := range []float32{1, 1.25, 1.5, 2, 3} {
			t.Run(fmt.Sprintf("%s/%g", fit, scale), func(t *testing.T) {
				v := NewView()
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"image","props":{"source":{"imageType":"svg","imageData":"<svg viewBox='0 0 20 10'><rect width='20' height='10' fill='#ff0000'/></svg>"},"fit":%q,"style":{"width":80,"height":40}}}]}}`, fit)))
				host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
					return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height})
				})
				host.AttachServices(fakeServices{})
				defer host.Dispose()
				img := animationFrameImage(t, host, scale)
				red := 0
				for i := 0; i < len(img.Pix); i += 4 {
					if img.Pix[i] > 250 && img.Pix[i+1] < 5 && img.Pix[i+3] > 250 {
						red++
					}
				}
				if want := int(80*scale) * int(40*scale); red != want {
					t.Fatalf("red area = %d, want %d", red, want)
				}
			})
		}
	}
}

// TestSourceSVGUsesExplicitAppearance prevents custom foreground colors from flipping icon paint.
func TestSourceSVGUsesExplicitAppearance(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"image","props":{"source":{"imageType":"svg","imageData":"<svg viewBox='0 0 10 10'><rect width='10' height='10' fill='var(--wox-theme-icon-color)'/></svg>"},"style":{"width":20,"height":20}}}]}}`))
	theme := component.ControlTheme{Dark: true, Text: ui.Color{A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for _, dark := range []bool{true, false, true} {
		theme.Dark = dark
		theme.Text = ui.Color{A: 255}
		if !dark {
			theme.Text = ui.Color{R: 255, G: 255, B: 255, A: 255}
		}
		img := animationFrameImage(t, host, 1.5)
		color := img.RGBAAt(15, 15)
		if color.A != 255 || dark && color.R != 255 || !dark && color.R != 0 {
			t.Fatalf("dark=%v pixel=%v", dark, color)
		}
	}
}

// TestImageDecodeBudgets rejects oversized headers before decoding and caps large SVG viewports.
func TestImageDecodeBudgets(t *testing.T) {
	svg := ImageSource{ImageType: common.WoxImageTypeSvg, ImageData: `<svg viewBox="0 0 20 10"/>`}
	decoded, _, err := decodeImage(svg, 32768, maxImageCacheBytes)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Width != maxSVGRasterSize || decoded.Height != maxSVGRasterSize/2 {
		t.Fatalf("unbounded or distorted SVG: %dx%d", decoded.Width, decoded.Height)
	}
	if _, _, err := decodeImage(svg, 32, 1); err != errImageFrameBudget {
		t.Fatalf("frame budget: %v", err)
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	// A valid header with a tiny compressed body must be rejected on dimensions,
	// not after allocating the advertised image or attempting to decode its pixels.
	binary.BigEndian.PutUint32(data[16:20], 8192)
	binary.BigEndian.PutUint32(data[20:24], 8192)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	path := filepath.Join(t.TempDir(), "oversized.png")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []ImageSource{
		{ImageType: common.WoxImageTypeBase64, ImageData: "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)},
		{ImageType: common.WoxImageTypeAbsolutePath, ImageData: path},
	} {
		if _, _, err := decodeImage(source, 32, maxImageCacheBytes); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("oversized %s decoded: %v", source.ImageType, err)
		}
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "oversized.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxImageSourceBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()
	if _, _, err := decodeImage(ImageSource{ImageType: common.WoxImageTypeAbsolutePath, ImageData: file.Name()}, 32, maxImageCacheBytes); err == nil || !strings.Contains(err.Error(), "32 MiB") {
		t.Fatalf("unbounded file input: %v", err)
	}
}

// TestImageCacheBudgetRecovers checks aggregate live bytes, reuse, and admission after removal.
func TestImageCacheBudgetRecovers(t *testing.T) {
	cache := NewView().images
	var props []*ImageProps
	for i := range 5 {
		p := &ImageProps{Source: &ImageSource{ImageType: common.WoxImageTypeSvg, ImageData: fmt.Sprintf(`<svg viewBox="0 0 1 1"><!-- %d --></svg>`, i)}}
		if err := p.prepare(); err != nil {
			t.Fatal(err)
		}
		props = append(props, p)
	}
	cache.beginBuild()
	for i, p := range props {
		got := cache.get(p, false, 16384, 16384)
		if (got != nil) != (i < 4) {
			t.Fatalf("image %d admission=%v", i, got != nil)
		}
	}
	if cache.bytes > maxImageCacheBytes || cache.frameBytes != maxImageCacheBytes {
		t.Fatalf("cache accounting: %+v", cache)
	}
	cache.endBuild()
	cache.beginBuild()
	if cache.get(props[4], false, 16384, 16384) == nil {
		t.Fatal("removing images did not restore capacity")
	}
	if cache.bytes > maxImageCacheBytes {
		t.Fatal("unused entries exceeded the cache budget")
	}
	cache.endBuild()
	if len(cache.entries) != 1 || cache.bytes != maxSVGRasterSize*maxSVGRasterSize*4 {
		t.Fatal("unused pixels retained")
	}
	cache.beginBuild()
	cache.endBuild()
	if cache.bytes != 0 || len(cache.entries) != 0 {
		t.Fatal("empty document retained images")
	}
}

// TestBitmapCacheSharesDisplaySizes avoids decoding the same bitmap for every logical size.
func TestBitmapCacheSharesDisplaySizes(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "shared.png")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []ImageSource{
		{ImageType: common.WoxImageTypeBase64, ImageData: "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())},
		{ImageType: common.WoxImageTypeAbsolutePath, ImageData: path},
	} {
		cache := NewView().images
		props := &ImageProps{Source: &source}
		if err := props.prepare(); err != nil {
			t.Fatal(err)
		}
		cache.beginBuild()
		first := cache.get(props, false, 16, 16)
		second := cache.get(props, false, 100, 100)
		if first == nil || first != second || cache.bytes != 4*2*4 || cache.frameBytes != cache.bytes {
			t.Fatal("bitmap decoded twice for distinct display sizes")
		}
	}
}
