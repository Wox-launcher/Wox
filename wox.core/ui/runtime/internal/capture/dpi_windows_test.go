//go:build windows

package capture

import "syscall"

// enablePerMonitorDPIAwareness makes opt-in hardware measurements use the same physical coordinates as the UI process.
func enablePerMonitorDPIAwareness() {
	user32 := syscall.NewLazyDLL("user32.dll")
	perMonitorV2 := ^uintptr(3)
	aware := false
	if setter := user32.NewProc("SetProcessDpiAwarenessContext"); setter.Find() == nil {
		result, _, _ := setter.Call(perMonitorV2)
		aware = result != 0
	}
	if !aware {
		if setter := user32.NewProc("SetProcessDPIAware"); setter.Find() == nil {
			_, _, _ = setter.Call()
		}
	}
	if setter := user32.NewProc("SetThreadDpiAwarenessContext"); setter.Find() == nil {
		_, _, _ = setter.Call(perMonitorV2)
	}
}
