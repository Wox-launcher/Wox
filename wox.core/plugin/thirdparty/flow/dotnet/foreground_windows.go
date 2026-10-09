//go:build windows

package dotnet

import "golang.org/x/sys/windows"

var allowSetForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// allowHostSetForeground lets the host process bring its settings window forward.
// Windows ignores SetForegroundWindow from a process that is not already in front
// unless the foreground process grants this permission.
func allowHostSetForeground(pid int) {
	if pid <= 0 {
		return
	}
	_, _, _ = allowSetForegroundWindow.Call(uintptr(uint32(pid)))
}
