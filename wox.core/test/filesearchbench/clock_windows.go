//go:build windows

package filesearchbench

import (
	"golang.org/x/sys/windows"
	"time"
	"unsafe"
)

var performanceCounter = windows.NewLazySystemDLL("kernel32.dll").NewProc("QueryPerformanceCounter")
var performanceFrequency = counterFrequency()

// counterFrequency uses QPC directly because coarse Windows wall clocks hide sub-millisecond queries.
func counterFrequency() int64 {
	var value int64
	result, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("QueryPerformanceFrequency").Call(uintptr(unsafe.Pointer(&value)))
	if result == 0 {
		return 0
	}
	return value
}

// queryClock avoids tick multiplication overflow on machines with long uptime.
func queryClock() int64 {
	var ticks int64
	result, _, _ := performanceCounter.Call(uintptr(unsafe.Pointer(&ticks)))
	if result == 0 || performanceFrequency == 0 {
		return time.Now().UnixNano()
	}
	return ticks/performanceFrequency*1e9 + ticks%performanceFrequency*1e9/performanceFrequency
}
