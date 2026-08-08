package ime

/*
#cgo windows LDFLAGS: -limm32 -luser32
#include <windows.h>
#include <imm.h>
#include <stdint.h>
#include <stdlib.h>

uintptr_t CaptureForegroundKeyboardLayout() {
    HWND hwnd = GetForegroundWindow();
    if (!hwnd) {
        return 0;
    }

    DWORD threadID = GetWindowThreadProcessId(hwnd, NULL);
    return (uintptr_t)GetKeyboardLayout(threadID);
}

BOOL RequestSwitchToForegroundLayout(uintptr_t rawLayout) {
    HWND hwnd = GetForegroundWindow();
    if (!hwnd || !rawLayout) {
        return FALSE;
    }

    SendMessageW(hwnd, WM_INPUTLANGCHANGEREQUEST, 0, (LPARAM)rawLayout);
    return TRUE;
}

BOOL SwitchForegroundIMEToAlphanumeric() {
    HWND hwnd = GetForegroundWindow();
    if (!hwnd) {
        return FALSE;
    }

    HKL keyboardLayout = GetKeyboardLayout(GetWindowThreadProcessId(hwnd, NULL));
    if (!ImmIsIME(keyboardLayout)) {
        return FALSE;
    }

    // Wox uses a custom native window, so its default IME window is more reliable
    // than an HIMC that may not yet be associated with the window.
    HWND imeWindow = ImmGetDefaultIMEWnd(hwnd);
    if (imeWindow) {
        enum {
            imcGetConversionMode = 0x0001,
            imcSetConversionMode = 0x0002,
        };
        SendMessageW(imeWindow, WM_IME_CONTROL, imcSetConversionMode, IME_CMODE_ALPHANUMERIC);
        if (SendMessageW(imeWindow, WM_IME_CONTROL, imcGetConversionMode, 0) == IME_CMODE_ALPHANUMERIC) {
            return TRUE;
        }
    }

    HIMC context = ImmGetContext(hwnd);
    if (!context) {
        return FALSE;
    }

    DWORD conversion = 0;
    DWORD sentence = 0;
    BOOL statusRead = ImmGetConversionStatus(context, &conversion, &sentence);
    if (!statusRead) {
        ImmReleaseContext(hwnd, context);
        return FALSE;
    }

    // Clearing NATIVE keeps the active IME and changes only its conversion mode.
    conversion &= ~IME_CMODE_NATIVE;
    BOOL statusSet = ImmSetConversionStatus(context, conversion, sentence);
    ImmReleaseContext(hwnd, context);
    return statusSet;
}

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
	"sync"
	"unsafe"
)

var capturedForegroundKeyboardLayout struct {
	sync.Mutex
	value uintptr
}

// CaptureInputMethodBeforeActivation saves the previous foreground thread's input layout before Wox takes focus.
func CaptureInputMethodBeforeActivation() {
	layout := uintptr(C.CaptureForegroundKeyboardLayout())
	capturedForegroundKeyboardLayout.Lock()
	capturedForegroundKeyboardLayout.value = layout
	capturedForegroundKeyboardLayout.Unlock()
}

func takeCapturedForegroundKeyboardLayout() uintptr {
	capturedForegroundKeyboardLayout.Lock()
	defer capturedForegroundKeyboardLayout.Unlock()
	layout := capturedForegroundKeyboardLayout.value
	capturedForegroundKeyboardLayout.value = 0
	return layout
}

// SwitchInputMethodABC switches the current IME to alphanumeric mode and falls back to en-US when that is unavailable.
func SwitchInputMethodABC() error {
	if capturedLayout := takeCapturedForegroundKeyboardLayout(); capturedLayout != 0 {
		C.RequestSwitchToForegroundLayout(C.uintptr_t(capturedLayout))
	}

	if C.SwitchForegroundIMEToAlphanumeric() != C.FALSE {
		return nil
	}

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
