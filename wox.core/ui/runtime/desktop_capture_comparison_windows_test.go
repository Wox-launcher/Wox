//go:build windows

package woxui

import (
	"image"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
	"wox/util/screen"
)

// TestWindowsDesktopCaptureComparison is an opt-in hardware experiment, not a CI timing assertion.
// Cold DXGI includes per-display device creation, capture, composition, and cleanup;
// reused DXGI measures the existing recording capturer with devices retained between frames.
func TestWindowsDesktopCaptureComparison(t *testing.T) {
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
	virtual := screen.GetVirtualPixelBounds(displays)
	bounds := image.Rect(virtual.X, virtual.Y, virtual.Right(), virtual.Bottom())
	for _, display := range displays {
		t.Logf("display=%s physical=%+v scale=%.2f", display.Name, display.PixelBounds, display.Scale)
	}

	// Composition uses physical offsets, including negative desktop origins. Copy raw
	// bytes: the existing DXGI wrapper returns BGRX despite using image.RGBA as its container.
	captureDXGI := func(reused []*windowsDXGIRectCapturer) error {
		output := make([]byte, bounds.Dx()*bounds.Dy()*4)
		errors := make([]error, len(displays))
		frames := make([]*image.RGBA, len(displays))
		setupTimes := make([]time.Duration, len(displays))
		captureTimes := make([]time.Duration, len(displays))
		var workers sync.WaitGroup
		for index, display := range displays {
			workers.Add(1)
			go func() {
				defer workers.Done()
				pixels := display.PixelBounds
				rect := image.Rect(pixels.X, pixels.Y, pixels.Right(), pixels.Bottom())
				var capturer *windowsDXGIRectCapturer
				if reused != nil {
					capturer = reused[index]
				} else {
					started := time.Now()
					capturer, errors[index] = newWindowsDXGIRectCapturer(rect)
					setupTimes[index] = time.Since(started)
					if errors[index] != nil {
						return
					}
					defer capturer.Close()
				}
				started := time.Now()
				frame, err := capturer.Capture()
				captureTimes[index] = time.Since(started)
				if err != nil {
					errors[index] = err
					return
				}
				frames[index] = frame
			}()
		}
		workers.Wait()
		for index, err := range errors {
			if err != nil {
				return err
			}
			pixels := displays[index].PixelBounds
			frame := frames[index]
			offset := image.Pt(pixels.X, pixels.Y).Sub(bounds.Min)
			// Compose after workers finish so mirrored/overlapping displays cannot race.
			for row := 0; row < pixels.Height; row++ {
				destination := ((offset.Y+row)*bounds.Dx() + offset.X) * 4
				copy(output[destination:destination+pixels.Width*4], frame.Pix[row*frame.Stride:row*frame.Stride+pixels.Width*4])
			}
			if reused == nil {
				t.Logf("DXGI display=%s setup=%s capture=%s", displays[index].Name, setupTimes[index], captureTimes[index])
			}
		}
		runtime.KeepAlive(output)
		return nil
	}
	for trial := 0; trial < 5; trial++ {
		// Alternate order to avoid always giving one backend a warmed desktop.
		for order := 0; order < 2; order++ {
			FlushWindowsDesktopComposition()
			started := time.Now()
			if (trial+order)%2 == 0 {
				capture, err := CaptureWindowsVirtualDesktop()
				if err != nil {
					t.Fatal(err)
				}
				if capture.Bounds != bounds {
					t.Errorf("GDI bounds=%v, DXGI bounds=%v", capture.Bounds, bounds)
				}
				capture.Close()
				t.Logf("trial=%d backend=GDI total=%s", trial+1, time.Since(started))
			} else {
				if err := captureDXGI(nil); err != nil {
					t.Fatal(err)
				}
				t.Logf("trial=%d backend=DXGI-cold total=%s", trial+1, time.Since(started))
			}
		}
	}
	reused := make([]*windowsDXGIRectCapturer, len(displays))
	started := time.Now()
	for index, display := range displays {
		pixels := display.PixelBounds
		reused[index], err = newWindowsDXGIRectCapturer(image.Rect(pixels.X, pixels.Y, pixels.Right(), pixels.Bottom()))
		if err != nil {
			t.Fatal(err)
		}
		defer reused[index].Close()
	}
	t.Logf("backend=DXGI-retained setup=%s", time.Since(started))
	for trial := 0; trial < 5; trial++ {
		FlushWindowsDesktopComposition()
		started := time.Now()
		if err := captureDXGI(reused); err != nil {
			t.Fatal(err)
		}
		t.Logf("trial=%d backend=DXGI-retained total=%s", trial+1, time.Since(started))
	}
}
