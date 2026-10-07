//go:build windows

package screenshot

/*
#cgo CXXFLAGS: -std=c++17 -D_WIN32_WINNT=0x0A00
#cgo LDFLAGS: -lole32 -loleaut32 -luiautomationcore -luuid -lstdc++
#include "object_selection_windows.h"
*/
import "C"

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"runtime/cgo"
	"time"

	"wox/util"
)

// windowsScreenshotObjectSelector owns a bounded cache on one worker's MTA for the capture session.
type windowsScreenshotObjectSelector struct {
	native      *C.WoxScreenshotObjectSelector
	initialized bool
}

// elements queries a frozen HWND's child controls without hit-testing Wox's covering overlay.
// Input and output use physical desktop pixels; callers map them into their capture surface.
func (selector *windowsScreenshotObjectSelector) elements(ctx context.Context, hwnd uintptr, point Point) []Rect {
	if ctx.Err() != nil || hwnd == 0 {
		return nil
	}
	budget := screenshotObjectForegroundBudget
	if deadline, ok := ctx.Deadline(); ok {
		budget = time.Until(deadline)
	}
	if budget < time.Millisecond {
		return nil
	}
	started := time.Now()
	if !selector.initialized {
		// Keep COM initialization, cached provider references, and cleanup on the same OS thread.
		runtime.LockOSThread()
		selector.initialized = true
	}
	if selector.native == nil {
		selector.native = C.wox_windows_screenshot_selector_create()
	}
	if selector.native == nil {
		return nil
	}
	cancellation := cgo.NewHandle(ctx)
	defer cancellation.Delete()
	var native [48]C.WoxScreenshotElementRect
	var refinement C.int32_t
	if budget > screenshotObjectForegroundBudget {
		// The longer dwell query refreshes an initially coarse provider tree once per window.
		refinement = 1
	}
	count := C.wox_windows_screenshot_elements(selector.native, C.uintptr_t(hwnd),
		C.int32_t(math.Round(float64(point.X))), C.int32_t(math.Round(float64(point.Y))),
		C.uint32_t(budget.Milliseconds()), refinement, C.uintptr_t(cancellation), &native[0], C.int32_t(len(native)))
	if elapsed := time.Since(started); elapsed >= 50*time.Millisecond {
		util.GetLogger().Debug(context.Background(), fmt.Sprintf("screenshot object query: duration=%s budget=%s hwnd=%d depth=%d cancelled=%t", elapsed, budget, hwnd, count, ctx.Err() != nil))
	}
	rects := make([]Rect, 0, int(count))
	for index := 0; index < int(count); index++ {
		frame := native[index]
		rects = append(rects, Rect{X: float32(frame.left), Y: float32(frame.top), Width: float32(frame.right - frame.left), Height: float32(frame.bottom - frame.top)})
	}
	return rects
}

// reset discards geometry when the underlying window no longer matches the capture.
func (selector *windowsScreenshotObjectSelector) reset() {
	if selector.native != nil {
		C.wox_windows_screenshot_selector_reset(selector.native)
	}
}

// close runs on the query worker, after any foreign provider call has returned.
func (selector *windowsScreenshotObjectSelector) close() {
	if selector.initialized {
		C.wox_windows_screenshot_selector_destroy(selector.native)
		selector.native = nil
		selector.initialized = false
		runtime.UnlockOSThread()
	}
}

//export woxGoWindowsScreenshotCancelled
func woxGoWindowsScreenshotCancelled(handle C.uintptr_t) C.int32_t {
	if cgo.Handle(handle).Value().(context.Context).Err() != nil {
		return 1
	}
	return 0
}
