package ime

/*
#include <windows.h>

HKL LoadKL(LPCSTR pwszKLID, UINT Flags) {
    return LoadKeyboardLayoutA(pwszKLID, Flags);
}

BOOL RequestSwitchToForeground(HKL hkl) {
    HWND hwnd = GetForegroundWindow();
    if (!hwnd) return FALSE;
    return PostMessage(hwnd, WM_INPUTLANGCHANGEREQUEST, 0, (LPARAM)hkl);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// SwitchInputMethodABC tries to switch the foreground window's input method to en-US (US keyboard) on Windows.
func SwitchInputMethodABC() error {
	kbLayoutID := "00000409" // en-US
	cStr := C.CString(kbLayoutID)
	defer C.free(unsafe.Pointer(cStr))

	hkl := C.LoadKL(cStr, C.KLF_ACTIVATE)
	if hkl == nil {
		return fmt.Errorf("load keyboard layout failed")
	}

	if C.RequestSwitchToForeground(hkl) == C.FALSE {
		return fmt.Errorf("request switch input language to foreground window failed")
	}
	return nil
}
