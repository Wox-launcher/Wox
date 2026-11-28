//go:build windows

package window

import (
	"image"
	"testing"
)

// TestWindowsSceneImageCache keeps several large atlases and the former medium-size gap warm.
func TestWindowsSceneImageCache(t *testing.T) {
	r, err := newWindowsOffscreenRenderer(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.destroy)
	d := &DisplayList{}
	var bytes int64
	for _, size := range []image.Point{{128, 128}, {1024, 512}, {1536, 1024}, {1774, 887}, {1254, 1254}} {
		bitmap, err := NewImage(image.NewRGBA(image.Rect(0, 0, size.X, size.Y)))
		if err != nil {
			t.Fatal(err)
		}
		bytes += int64(size.X * size.Y * 4)
		d.DrawImage(bitmap, Rect{Width: 64, Height: 64})
	}
	for frame := 0; frame < 120; frame++ {
		if err := r.render(d, 1.5); err != nil {
			t.Fatal(err)
		}
		stats := r.resourceStats()
		wantUploads, wantHits := 0, 5
		if frame == 0 {
			wantUploads, wantHits = 5, 0
		}
		if stats.ImageUploads != wantUploads || stats.CacheHits != wantHits || stats.ResidentBytes != bytes {
			t.Fatalf("frame %d: %+v, want uploads=%d hits=%d bytes=%d", frame, stats, wantUploads, wantHits, bytes)
		}
	}
	if err := r.clearImageCache(); err != nil {
		t.Fatal(err)
	}
	if r.residentBytes() != 0 {
		t.Fatal("clearing cache retained scene textures")
	}
	if err := r.render(d, 2); err != nil {
		t.Fatal(err)
	}
	if r.resourceStats().ImageUploads != 5 {
		t.Fatal("cleared cache did not recreate textures")
	}
}

// TestWindowsImageCacheBudget ensures eviction stays bounded and preserves same-frame reuse.
func TestWindowsImageCacheBudget(t *testing.T) {
	r, err := newWindowsOffscreenRenderer(32, 32)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.destroy)
	d := &DisplayList{}
	var first *Image
	for i := 0; i < 12; i++ {
		bitmap, err := NewImage(image.NewRGBA(image.Rect(0, 0, 1024, 1024)))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = bitmap
		}
		d.DrawImage(bitmap, Rect{Width: 32, Height: 32})
	}
	d.DrawImage(first, Rect{Width: 32, Height: 32})
	if err := r.render(d, 1); err != nil {
		t.Fatal(err)
	}
	stats := r.resourceStats()
	if stats.ImageUploads != 12 || stats.CacheHits != 1 || stats.CacheEvictions != 4 || stats.ResidentBytes != 32*1024*1024 {
		t.Fatalf("unexpected bounded cache activity: %+v", stats)
	}
}

// TestWindowsLargeImageCache retains up to two oversized screenshot bitmaps across frames without
// evicting normal textures or re-uploading every frame.
func TestWindowsLargeImageCache(t *testing.T) {
	r, err := newWindowsOffscreenRenderer(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.destroy)

	const width, height = 4096, 2160
	const largeBytes = int64(width * height * 4)
	first, err := NewImage(image.NewRGBA(image.Rect(0, 0, width, height)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewImage(image.NewRGBA(image.Rect(0, 0, width, height)))
	if err != nil {
		t.Fatal(err)
	}
	third, err := NewImage(image.NewRGBA(image.Rect(0, 0, width, height)))
	if err != nil {
		t.Fatal(err)
	}

	d := &DisplayList{}
	d.DrawImage(first, Rect{Width: 64, Height: 64})
	d.DrawImage(second, Rect{Width: 64, Height: 64})
	if err := r.render(d, 1); err != nil {
		t.Fatal(err)
	}
	stats := r.resourceStats()
	if stats.ImageUploads != 2 || stats.CacheHits != 0 || stats.ResidentBytes != 2*largeBytes {
		t.Fatalf("frame 0: %+v, want uploads=2 hits=0 bytes=%d", stats, 2*largeBytes)
	}

	if err := r.render(d, 1); err != nil {
		t.Fatal(err)
	}
	stats = r.resourceStats()
	if stats.ImageUploads != 0 || stats.CacheHits != 2 || stats.ResidentBytes != 2*largeBytes {
		t.Fatalf("frame 1: %+v, want uploads=0 hits=2 bytes=%d", stats, 2*largeBytes)
	}

	d2 := &DisplayList{}
	d2.DrawImage(third, Rect{Width: 64, Height: 64})
	if err := r.render(d2, 1); err != nil {
		t.Fatal(err)
	}
	stats = r.resourceStats()
	if stats.ImageUploads != 1 || stats.CacheEvictions != 1 || stats.ResidentBytes != 2*largeBytes {
		t.Fatalf("frame 2: %+v, want uploads=1 evictions=1 bytes=%d", stats, 2*largeBytes)
	}

	if err := r.clearImageCache(); err != nil {
		t.Fatal(err)
	}
	if r.residentBytes() != 0 {
		t.Fatalf("clearing cache retained large image: %d", r.residentBytes())
	}
}
