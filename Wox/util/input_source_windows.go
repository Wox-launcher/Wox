package util

/*
#include <windows.h>

HKL LoadKL(LPCSTR pwszKLID, UINT Flags) {
    return LoadKeyboardLayoutA(pwszKLID, Flags);
}

BOOL ActivateKL(HKL hkl) {
    return ActivateKeyboardLayout(hkl, KLF_ACTIVATE) == hkl;
}
*/
import "C"

// seems not work properly, this can only change current process's input method, not tauri's, need another way
func SwitchInputMethodABC() {
	kbLayoutID := "00000409" // en-US
	hkl := C.LoadKL(C.CString(kbLayoutID), C.KLF_ACTIVATE)
	if hkl == nil {
		GetLogger().Error(NewTraceContext(), "Failed to load keyboard layout.")
		return
	}

	success := C.ActivateKL(hkl)
	if success == C.FALSE {
		GetLogger().Error(NewTraceContext(), "activate keyboard failed")
		return
	}
}
