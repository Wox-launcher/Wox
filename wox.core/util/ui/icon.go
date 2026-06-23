package ui

import (
	"bytes"
	"image"
	"image/png"
	"strconv"
	"sync"

	"wox/common"
)

// iconCache caches rasterized WoxImage PNG bytes by hash to avoid
// re-decoding the same SVG/emoji on every frame.
var iconCache = struct {
	mu    sync.RWMutex
	items map[string][]byte
}{
	items: make(map[string][]byte),
}

// RasterizeWoxImage converts a WoxImage to PNG bytes suitable for the
// DrawImage draw command. Results are cached by the image hash.
// Returns nil if the image is empty or rasterization fails.
func RasterizeWoxImage(img common.WoxImage) []byte {
	if img.IsEmpty() {
		return nil
	}

	hash := img.Hash()

	iconCache.mu.RLock()
	if cached, ok := iconCache.items[hash]; ok {
		iconCache.mu.RUnlock()
		return cached
	}
	iconCache.mu.RUnlock()

	rasterized, err := img.ToImage()
	if err != nil || rasterized == nil {
		return nil
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, rasterized); err != nil {
		return nil
	}

	pngBytes := buf.Bytes()

	iconCache.mu.Lock()
	iconCache.items[hash] = pngBytes
	iconCache.mu.Unlock()

	return pngBytes
}

// RasterizeWoxImageWithSize converts a WoxImage to PNG bytes at the given
// pixel size. Used when a specific icon size is needed.
func RasterizeWoxImageWithSize(img common.WoxImage, size int) []byte {
	pngBytes, _ := RasterizeWoxImageWithSizeAndKey(img, size)
	return pngBytes
}

// RasterizeWoxImageWithSizeAndKey converts a WoxImage and returns the cache key
// used by both the Go PNG cache and the native bitmap cache.
func RasterizeWoxImageWithSizeAndKey(img common.WoxImage, size int) ([]byte, string) {
	if img.IsEmpty() {
		return nil, ""
	}

	// Use hash + size as cache key to avoid re-scaling
	hash := img.Hash() + ":" + strconv.Itoa(size)

	iconCache.mu.RLock()
	if cached, ok := iconCache.items[hash]; ok {
		iconCache.mu.RUnlock()
		return cached, hash
	}
	iconCache.mu.RUnlock()

	rasterized, err := img.ToImage()
	if err != nil || rasterized == nil {
		return nil, ""
	}

	// Scale to requested size if needed
	bounds := rasterized.Bounds()
	if bounds.Dx() != size || bounds.Dy() != size {
		rasterized = scaleImage(rasterized, size)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, rasterized); err != nil {
		return nil, ""
	}

	pngBytes := buf.Bytes()

	iconCache.mu.Lock()
	iconCache.items[hash] = pngBytes
	iconCache.mu.Unlock()

	return pngBytes, hash
}

// ClearIconCache releases cached PNG bytes. The launcher calls this when hidden
// so result icons do not keep memory after the visible fast path no longer needs them.
func ClearIconCache() {
	iconCache.mu.Lock()
	iconCache.items = make(map[string][]byte)
	iconCache.mu.Unlock()
}

// scaleImage resizes an image to the given square size using nearest-neighbor.
func scaleImage(src image.Image, size int) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		sy := y * bounds.Dy() / size
		for x := 0; x < size; x++ {
			sx := x * bounds.Dx() / size
			dst.Set(x, y, src.At(bounds.Min.X+sx, bounds.Min.Y+sy))
		}
	}
	return dst
}
