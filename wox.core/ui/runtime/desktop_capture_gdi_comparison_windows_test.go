//go:build windows

package woxui

import (
	"fmt"
	"image"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"
	"unsafe"
	"wox/util/screen"

	"github.com/lxn/win"
)

type gdiCaptureMeasurement struct {
	setup, blit, readback, total time.Duration
	sample                       uint64
}

// measureGDICapture compares DIB and compatible bitmap destinations without retaining resources.
// Tiled captures write straight into one desktop-sized bitmap, avoiding extra per-screen buffers.
func measureGDICapture(bounds image.Rectangle, regions []image.Rectangle, compatible bool) (measurement gdiCaptureMeasurement, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	started := time.Now()
	defer func() { measurement.total = time.Since(started) }()
	source := win.GetDC(0)
	if source == 0 {
		return measurement, fmt.Errorf("GetDC failed")
	}
	defer win.ReleaseDC(0, source)
	destination := win.CreateCompatibleDC(source)
	if destination == 0 {
		return measurement, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(destination)
	info := win.BITMAPINFO{BmiHeader: win.BITMAPINFOHEADER{
		BiSize: uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})), BiWidth: int32(bounds.Dx()),
		BiHeight: -int32(bounds.Dy()), BiPlanes: 1, BiBitCount: 32, BiCompression: win.BI_RGB,
	}}
	var bits unsafe.Pointer
	var bitmap win.HBITMAP
	if compatible {
		bitmap = win.CreateCompatibleBitmap(source, int32(bounds.Dx()), int32(bounds.Dy()))
	} else {
		bitmap = win.CreateDIBSection(source, &info.BmiHeader, win.DIB_RGB_COLORS, &bits, 0, 0)
	}
	if bitmap == 0 || (!compatible && bits == nil) {
		return measurement, fmt.Errorf("create bitmap failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))
	previous := win.SelectObject(destination, win.HGDIOBJ(bitmap))
	if previous == 0 {
		return measurement, fmt.Errorf("SelectObject failed")
	}
	defer win.SelectObject(destination, previous)
	// Clear only gaps, not pixels about to be overwritten by capture. Clearing the
	// whole 82 MB destination otherwise hides the benefit of skipping desktop gaps.
	if len(regions) > 1 {
		gaps := win.CreateRectRgn(0, 0, int32(bounds.Dx()), int32(bounds.Dy()))
		if gaps == 0 {
			return measurement, fmt.Errorf("create gap region failed")
		}
		defer win.DeleteObject(win.HGDIOBJ(gaps))
		for _, region := range regions {
			local := region.Sub(bounds.Min)
			covered := win.CreateRectRgn(int32(local.Min.X), int32(local.Min.Y), int32(local.Max.X), int32(local.Max.Y))
			if covered == 0 {
				return measurement, fmt.Errorf("create monitor region failed")
			}
			result := win.CombineRgn(gaps, gaps, covered, win.RGN_DIFF)
			win.DeleteObject(win.HGDIOBJ(covered))
			if result == 0 {
				return measurement, fmt.Errorf("subtract monitor region failed")
			}
		}
		if !win.FillRgn(destination, gaps, win.HBRUSH(win.GetStockObject(win.BLACK_BRUSH))) {
			return measurement, fmt.Errorf("clear gaps failed")
		}
	}
	measurement.setup = time.Since(started)
	blitStarted := time.Now()
	for _, region := range regions {
		offset := region.Min.Sub(bounds.Min)
		if !win.BitBlt(destination, int32(offset.X), int32(offset.Y), int32(region.Dx()), int32(region.Dy()), source,
			int32(region.Min.X), int32(region.Min.Y), win.SRCCOPY|windowsCaptureBlt) {
			return measurement, fmt.Errorf("BitBlt failed for %v", region)
		}
	}
	measurement.blit = time.Since(blitStarted)
	readStarted := time.Now()
	// GetDIBits requires the bitmap to be deselected. DIB reads need GDI completion too;
	// include synchronization so a faster submission alone cannot masquerade as a gain.
	win.SelectObject(destination, previous)
	var pixels []byte
	if compatible {
		pixels = make([]byte, bounds.Dx()*bounds.Dy()*4)
		if rows := win.GetDIBits(source, bitmap, 0, uint32(bounds.Dy()), &pixels[0], &info, win.DIB_RGB_COLORS); rows != int32(bounds.Dy()) {
			return measurement, fmt.Errorf("GetDIBits returned %d rows", rows)
		}
	} else {
		if !win.GdiFlush() {
			return measurement, fmt.Errorf("GdiFlush failed")
		}
		pixels = unsafe.Slice((*byte)(bits), bounds.Dx()*bounds.Dy()*4)
	}
	measurement.readback = time.Since(readStarted)
	// Sample BGR channels from every monitor while the native pixel storage is still alive.
	for _, region := range regions {
		for y := region.Min.Y; y < region.Max.Y; y += 61 {
			for x := region.Min.X; x < region.Max.X; x += 61 {
				i := ((y-bounds.Min.Y)*bounds.Dx() + x - bounds.Min.X) * 4
				measurement.sample += uint64(pixels[i]) + uint64(pixels[i+1]) + uint64(pixels[i+2])
			}
		}
	}
	runtime.KeepAlive(pixels)
	return measurement, nil
}

// TestWindowsGDICaptureComparison measures complete, uncached CPU-readable captures on real hardware.
func TestWindowsGDICaptureComparison(t *testing.T) {
	if os.Getenv("WOX_CAPTURE_COMPARE") != "1" {
		t.Skip("set WOX_CAPTURE_COMPARE=1 to capture the local desktop")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	enablePerMonitorDPIAwareness()
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		t.Fatalf("list displays: %v", err)
	}
	var bounds image.Rectangle
	var regions []image.Rectangle
	for _, display := range displays {
		p := display.PixelBounds
		rect := image.Rect(p.X, p.Y, p.Right(), p.Bottom())
		regions = append(regions, rect)
		bounds = bounds.Union(rect)
		t.Logf("display=%s physical=%v scale=%.2f", display.Name, rect, display.Scale)
	}
	cases := []struct {
		name       string
		regions    []image.Rectangle
		compatible bool
	}{
		{"desktop-DIB", []image.Rectangle{bounds}, false},
		{"desktop-DDB", []image.Rectangle{bounds}, true},
		{"per-screen-DIB", regions, false},
		{"per-screen-DDB", regions, true},
	}
	measurements := make([][]gdiCaptureMeasurement, len(cases))
	for trial := 0; trial < 8; trial++ {
		for order := range cases {
			index := (trial + order) % len(cases)
			variant := cases[index]
			// Match the screenshot path's DWM synchronization, outside the capture measurement.
			FlushWindowsDesktopComposition()
			result, err := measureGDICapture(bounds, variant.regions, variant.compatible)
			if err != nil {
				t.Fatalf("%s: %v", variant.name, err)
			}
			if result.sample == 0 {
				t.Fatalf("%s returned an entirely black sample", variant.name)
			}
			measurements[index] = append(measurements[index], result)
			t.Logf("trial=%d path=%s total=%s setup=%s blit=%s readback=%s", trial+1, variant.name, result.total, result.setup, result.blit, result.readback)
		}
	}
	for index, results := range measurements {
		var totals, blits, readbacks []time.Duration
		for _, result := range results {
			totals = append(totals, result.total)
			blits = append(blits, result.blit)
			readbacks = append(readbacks, result.readback)
		}
		slices.Sort(totals)
		slices.Sort(blits)
		slices.Sort(readbacks)
		t.Logf("SUMMARY path=%s median=%s min=%s max=%s blitMedian=%s readbackMedian=%s", cases[index].name,
			(totals[3]+totals[4])/2, totals[0], totals[7], (blits[3]+blits[4])/2, (readbacks[3]+readbacks[4])/2)
	}
}
