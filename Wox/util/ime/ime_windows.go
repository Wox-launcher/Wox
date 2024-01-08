package ime

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
import (
	"fmt"
)

// seems not work properly, this can only change current process's input method, not tauri's, need another way
func SwitchInputMethodABC() error {
	kbLayoutID := "00000409" // en-US
	hkl := C.LoadKL(C.CString(kbLayoutID), C.KLF_ACTIVATE)
	if hkl == nil {
		return fmt.Errorf("load keyboard layout failed")
	}

	success := C.ActivateKL(hkl)
	if success == C.FALSE {
		return fmt.Errorf("activate keyboard layout failed")
	}

	return nil
}
