//go:build windows

package emojiimage

import (
	"runtime"
	"sync"
	"testing"
)

// TestRenderConcurrent exercises native ownership while requests queue and resume.
func TestRenderConcurrent(t *testing.T) {
	var pending sync.WaitGroup
	for range 24 {
		pending.Go(func() {
			img, err := Render("😀", 32)
			if err != nil {
				t.Errorf("render: %v", err)
				return
			}
			if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
				t.Errorf("unexpected image bounds: %v", img.Bounds())
			}
		})
	}
	pending.Wait()
}

// TestRenderPreservesExistingApartment catches unbalanced COM teardown on failure.
func TestRenderPreservesExistingApartment(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := coInitializeEx.Call(0, 0) // COINIT_MULTITHREADED
	if int32(hr) < 0 {
		t.Fatalf("initialize MTA: 0x%08X", uint32(hr))
	}
	defer coUninitialize.Call()
	if _, err := Render("😀", 32); err != nil {
		t.Fatal(err)
	}
	hr, _, _ = coInitializeEx.Call(0, 0)
	if int32(hr) >= 0 {
		defer coUninitialize.Call()
	}
	if hr != 1 { // S_FALSE means the caller's apartment still exists.
		t.Fatalf("render changed the caller's COM ownership: 0x%08X", uint32(hr))
	}
}
