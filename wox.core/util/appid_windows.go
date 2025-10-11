package util

import (
    "unsafe"
    "golang.org/x/sys/windows"
)

// SetAppUserModelID sets a shared AppUserModelID so the shell can
// group Wox core and UI under one app in Task Manager. This does not
// create a taskbar icon by itself.
func SetAppUserModelID(id string) error {
    mod := windows.NewLazySystemDLL("Shell32.dll")
    proc := mod.NewProc("SetCurrentProcessExplicitAppUserModelID")
    p, err := windows.UTF16PtrFromString(id)
    if err != nil { return err }
    r1, _, callErr := proc.Call(uintptr(unsafe.Pointer(p)))
    if r1 != 0 { // HRESULT failure
        if callErr != nil { return callErr }
        return windows.Errno(r1)
    }
    return nil
}

