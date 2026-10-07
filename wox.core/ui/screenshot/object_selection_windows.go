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
	"math"
	"runtime"
	"runtime/cgo"
	"time"
)

// windowsScreenshotElements queries a frozen HWND's child controls without hit-testing Wox's covering overlay.
// Input and output use physical desktop pixels; callers map them into their capture surface.
func windowsScreenshotElements(ctx context.Context, hwnd uintptr, point Point) []Rect {
	if ctx.Err() != nil || hwnd == 0 {
		return nil
	}
	budget := 168 * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		budget = time.Until(deadline)
	}
	if budget < time.Millisecond {
		return nil
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cancellation := cgo.NewHandle(ctx)
	defer cancellation.Delete()
	var native [48]C.WoxScreenshotElementRect
	count := C.wox_windows_screenshot_elements(C.uintptr_t(hwnd), C.int32_t(math.Round(float64(point.X))), C.int32_t(math.Round(float64(point.Y))), C.uint32_t(budget.Milliseconds()), C.uintptr_t(cancellation), &native[0], C.int32_t(len(native)))
	rects := make([]Rect, 0, int(count))
	for index := 0; index < int(count); index++ {
		frame := native[index]
		rects = append(rects, Rect{X: float32(frame.left), Y: float32(frame.top), Width: float32(frame.right - frame.left), Height: float32(frame.bottom - frame.top)})
	}
	return rects
}

//export woxGoWindowsScreenshotCancelled
func woxGoWindowsScreenshotCancelled(handle C.uintptr_t) C.int32_t {
	if cgo.Handle(handle).Value().(context.Context).Err() != nil {
		return 1
	}
	return 0
}
