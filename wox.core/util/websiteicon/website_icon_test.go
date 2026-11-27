package websiteicon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"wox/common"
	"wox/util"
)

func buildPNGWithColor(t *testing.T, width, height int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func buildICOWithPayloads(entries []struct {
	w, h, bpp int
	payload   []byte
}) []byte {
	headerLen := 6 + len(entries)*16
	out := make([]byte, headerLen)
	binary.LittleEndian.PutUint16(out[0:2], 0)
	binary.LittleEndian.PutUint16(out[2:4], 1)
	binary.LittleEndian.PutUint16(out[4:6], uint16(len(entries)))

	offset := headerLen
	for i, e := range entries {
		off := 6 + i*16
		wByte := byte(e.w)
		if e.w >= 256 {
			wByte = 0
		}
		hByte := byte(e.h)
		if e.h >= 256 {
			hByte = 0
		}
		out[off] = wByte
		out[off+1] = hByte
		binary.LittleEndian.PutUint16(out[off+4:off+6], 1)
		binary.LittleEndian.PutUint16(out[off+6:off+8], uint16(e.bpp))
		binary.LittleEndian.PutUint32(out[off+8:off+12], uint32(len(e.payload)))
		binary.LittleEndian.PutUint32(out[off+12:off+16], uint32(offset))
		out = append(out, e.payload...)
		offset += len(e.payload)
	}
	return out
}

func buildDIB32Payload(w, h int, c color.NRGBA) []byte {
	xorStride := w * 4
	andStride := ((w + 31) / 32) * 4
	payload := make([]byte, 40+xorStride*h+andStride*h)
	binary.LittleEndian.PutUint32(payload[0:4], 40)
	binary.LittleEndian.PutUint32(payload[4:8], uint32(w))
	binary.LittleEndian.PutUint32(payload[8:12], uint32(h*2))
	binary.LittleEndian.PutUint16(payload[12:14], 1)
	binary.LittleEndian.PutUint16(payload[14:16], 32)
	for row := 0; row < h; row++ {
		for x := 0; x < w; x++ {
			off := 40 + row*xorStride + x*4
			payload[off] = c.B
			payload[off+1] = c.G
			payload[off+2] = c.R
			payload[off+3] = c.A
		}
	}
	return payload
}

func buildDIB24WithMaskPayload(w, h int, c color.RGBA, transparentTopLeft bool) []byte {
	xorStride := ((w*24 + 31) / 32) * 4
	andStride := ((w + 31) / 32) * 4
	payload := make([]byte, 40+xorStride*h+andStride*h)
	binary.LittleEndian.PutUint32(payload[0:4], 40)
	binary.LittleEndian.PutUint32(payload[4:8], uint32(w))
	binary.LittleEndian.PutUint32(payload[8:12], uint32(h*2))
	binary.LittleEndian.PutUint16(payload[12:14], 1)
	binary.LittleEndian.PutUint16(payload[14:16], 24)
	for row := 0; row < h; row++ {
		for x := 0; x < w; x++ {
			off := 40 + row*xorStride + x*3
			payload[off] = c.B
			payload[off+1] = c.G
			payload[off+2] = c.R
		}
	}
	if transparentTopLeft {
		// Top row in bottom-up DIB is row = h - 1; bit 0x80 masks x = 0.
		andOffset := 40 + xorStride*h + (h-1)*andStride
		payload[andOffset] = 0x80
	}
	return payload
}

func TestDecodeICOVariantsAndErrors(t *testing.T) {
	t.Run("multi-entry chooses largest PNG", func(t *testing.T) {
		small := buildPNGWithColor(t, 16, 16, color.RGBA{R: 255, A: 255})
		large := buildPNGWithColor(t, 64, 64, color.RGBA{G: 200, A: 255})
		ico := buildICOWithPayloads([]struct {
			w, h, bpp int
			payload   []byte
		}{
			{16, 16, 32, small},
			{64, 64, 32, large},
		})
		img, err := decodeICO(ico)
		if err != nil {
			t.Fatalf("decodeICO() error = %v", err)
		}
		if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
			t.Fatalf("bounds = %v, want 64x64", img.Bounds())
		}
	})

	t.Run("32-bit BGRA DIB", func(t *testing.T) {
		ico := buildICOWithPayloads([]struct {
			w, h, bpp int
			payload   []byte
		}{
			{4, 4, 32, buildDIB32Payload(4, 4, color.NRGBA{R: 10, G: 20, B: 30, A: 200})},
		})
		img, err := decodeICO(ico)
		if err != nil {
			t.Fatalf("decodeICO() error = %v", err)
		}
		got := color.NRGBAModel.Convert(img.At(1, 1)).(color.NRGBA)
		if got != (color.NRGBA{R: 10, G: 20, B: 30, A: 200}) {
			t.Fatalf("pixel = %+v", got)
		}
	})

	t.Run("24-bit BGR DIB with AND mask", func(t *testing.T) {
		ico := buildICOWithPayloads([]struct {
			w, h, bpp int
			payload   []byte
		}{
			{4, 4, 24, buildDIB24WithMaskPayload(4, 4, color.RGBA{R: 12, G: 34, B: 56, A: 255}, true)},
		})
		img, err := decodeICO(ico)
		if err != nil {
			t.Fatalf("decodeICO() error = %v", err)
		}
		topLeft := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
		opaque := color.NRGBAModel.Convert(img.At(1, 0)).(color.NRGBA)
		if topLeft.A != 0 || opaque.A != 255 || opaque.R != 12 || opaque.G != 34 || opaque.B != 56 {
			t.Fatalf("topLeft=%+v opaque=%+v", topLeft, opaque)
		}
	})

	t.Run("malformed ICO does not panic", func(t *testing.T) {
		for _, bad := range [][]byte{
			nil,
			{0, 0, 1, 0, 1, 0},
			{0, 0, 1, 0, 1, 0, 16, 16, 0, 0, 1, 0, 32, 0, 200, 0, 0, 0, 22, 0, 0, 0},
		} {
			if _, err := decodeICO(bad); err == nil {
				t.Fatalf("expected error for malformed ICO len=%d", len(bad))
			}
		}
	})
}

func TestFetchDirectImageDecodesICO(t *testing.T) {
	ico := buildICOWithPayloads([]struct {
		w, h, bpp int
		payload   []byte
	}{
		{16, 16, 32, buildDIB32Payload(16, 16, color.NRGBA{R: 40, G: 80, B: 120, A: 255})},
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		_, _ = w.Write(ico)
	}))
	t.Cleanup(server.Close)

	icon, err := FetchDirectImage(context.Background(), server.URL+"/favicon.ico")
	if err != nil {
		t.Fatalf("FetchDirectImage() error = %v", err)
	}
	if icon.ImageType != common.WoxImageTypeBase64 || icon.ImageData == "" {
		t.Fatalf("unexpected icon: %+v", icon)
	}
}

func TestFetchSiteFaviconHTMLRankingBaseAndFallbacks(t *testing.T) {
	smallICO := buildICOWithPayloads([]struct {
		w, h, bpp int
		payload   []byte
	}{{16, 16, 32, buildDIB32Payload(16, 16, color.NRGBA{R: 255, A: 255})}})
	largePNG := buildPNGWithColor(t, 180, 180, color.RGBA{B: 255, A: 255})

	t.Run("prefers 180x180 apple-touch-icon via base href over 16x16 ico", func(t *testing.T) {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `<!doctype html><html><head>
					<base href="%s/assets/">
					<link rel="icon" sizes="16x16" href="small.ico">
					<link rel="apple-touch-icon" sizes="180x180" href="apple.png">
				</head><body>ok</body></html>`, server.URL)
			case "/assets/small.ico":
				w.Header().Set("Content-Type", "image/x-icon")
				_, _ = w.Write(smallICO)
			case "/assets/apple.png":
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(largePNG)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(server.Close)

		img, err := fetchSiteFavicon(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("fetchSiteFavicon() error = %v", err)
		}
		if img.Bounds().Dx() != 180 {
			t.Fatalf("width = %d, want 180", img.Bounds().Dx())
		}
	})

	t.Run("inline base64 and svg data URI link", func(t *testing.T) {
		b64 := base64.StdEncoding.EncodeToString(buildPNGWithColor(t, 32, 32, color.RGBA{R: 10, G: 200, A: 255}))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><head><link rel="icon" sizes="32x32" href="data:image/png;base64,%s"></head></html>`, b64)
		}))
		t.Cleanup(server.Close)

		img, err := fetchSiteFavicon(context.Background(), server.URL)
		if err != nil || img.Bounds().Dx() != 32 {
			t.Fatalf("data png fetch failed: img=%v err=%v", img, err)
		}

		svgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A//www.w3.org/2000/svg%22%20width%3D%2216%22%20height%3D%2216%22%3E%3Crect%20width%3D%2216%22%20height%3D%2216%22%20fill%3D%22%23ff0000%22/%3E%3C/svg%3E"></head></html>`))
		}))
		t.Cleanup(svgServer.Close)

		svgImg, err := fetchSiteFavicon(context.Background(), svgServer.URL)
		if err != nil || svgImg.Bounds().Dx() == 0 {
			t.Fatalf("data svg fetch failed: img=%v err=%v", svgImg, err)
		}
	})

	t.Run("fallback to /favicon.ico when homepage returns 403", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/favicon.ico" {
				w.Header().Set("Content-Type", "image/x-icon")
				_, _ = w.Write(smallICO)
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		}))
		t.Cleanup(server.Close)

		img, err := fetchSiteFavicon(context.Background(), server.URL)
		if err != nil || img.Bounds().Dx() != 16 {
			t.Fatalf("fallback /favicon.ico failed: img=%v err=%v", img, err)
		}
	})
}

func TestFetchCachesResolvedFaviconOnDisk(t *testing.T) {
	dir := t.TempDir()
	prev := util.GetLocation().GetUserDataDirectory()
	util.GetLocation().UpdateUserDataDirectory(dir)
	t.Cleanup(func() { util.GetLocation().UpdateUserDataDirectory(prev) })
	if err := os.MkdirAll(util.GetLocation().GetImageCacheDirectory(), 0755); err != nil {
		t.Fatal(err)
	}

	var hits atomic.Int32
	pngBytes := buildPNGWithColor(t, 48, 48, color.RGBA{R: 50, G: 100, B: 150, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/site.png"></head></html>`))
		case "/site.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	first, err := Fetch(context.Background(), server.URL+"/some/page")
	if err != nil {
		t.Fatalf("first Fetch() error = %v", err)
	}
	if first.ImageType != common.WoxImageTypeBase64 || first.ImageData == "" {
		t.Fatalf("unexpected first icon: %+v", first)
	}
	firstHits := hits.Load()

	second, err := Fetch(context.Background(), server.URL+"/other/page")
	if err != nil {
		t.Fatalf("second Fetch() error = %v", err)
	}
	if second.ImageType != common.WoxImageTypeAbsolutePath || hits.Load() != firstHits {
		t.Fatalf("expected cache hit on second Fetch, second=%+v hits=%d->%d", second, firstHits, hits.Load())
	}
}
