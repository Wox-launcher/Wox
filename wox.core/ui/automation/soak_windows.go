//go:build wox_automation

package automation

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var getProcessHandleCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")

func processSoakCounters() (int64, int, error) {
	proc := windows.CurrentProcess()
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(proc, &created, &exited, &kernel, &user); err != nil {
		return 0, 0, err
	}
	var handles uint32
	if ok, _, err := getProcessHandleCount.Call(uintptr(proc), uintptr(unsafe.Pointer(&handles))); ok == 0 {
		return 0, 0, err
	}
	ticks := (uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)) + (uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime))
	return int64(ticks * 100), int(handles), nil
}
