package ime

/*
#cgo windows LDFLAGS: -limm32 -luser32
#include <windows.h>
#include <imm.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

enum {
    imcGetOpenStatus = 0x0005,
    imcSetOpenStatus = 0x0006,
};

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

BOOL LayoutIsIME(uintptr_t rawLayout) {
    return rawLayout && ImmIsIME((HKL)rawLayout);
}

// IMESwitchDiag records what SwitchForegroundIMEToAlphanumeric observed so the
// Wox log can explain why an IME did or did not change state.
typedef struct {
    uintptr_t foreground;
    uintptr_t layout;
    BOOL foregroundIsCurrentProcess;
    BOOL isIME;
    BOOL hasContext;
    LONG openBefore;
    LONG openAfter;
    DWORD conversionBefore;
    DWORD conversionAfter;
} IMESwitchDiag;

// SwitchForegroundIMEToAlphanumeric keeps the active IME but puts it into its
// English/passthrough state. Returns FALSE when no IME state could be changed so
// the caller can fall back to a Latin keyboard layout.
//
// The IME open status is the switch that Chinese TSF IMEs actually honor:
// WeChat IME (WeType) ignores IME_CMODE_NATIVE changes entirely and treats
// "closed" as its English mode, while Microsoft Pinyin accepts both. Conversion
// mode is still cleared for IMEs that only look at that flag.
BOOL SwitchForegroundIMEToAlphanumeric(IMESwitchDiag *diag) {
    memset(diag, 0, sizeof(*diag));
    diag->openBefore = -1;
    diag->openAfter = -1;

    HWND hwnd = GetForegroundWindow();
    diag->foreground = (uintptr_t)hwnd;
    if (!hwnd) {
        return FALSE;
    }

    DWORD processID = 0;
    DWORD threadID = GetWindowThreadProcessId(hwnd, &processID);
    diag->foregroundIsCurrentProcess = processID == GetCurrentProcessId();
    if (!diag->foregroundIsCurrentProcess) {
        // Never touch another application's IME; Wox is not in front yet.
        return FALSE;
    }

    HKL keyboardLayout = GetKeyboardLayout(threadID);
    diag->layout = (uintptr_t)keyboardLayout;
    diag->isIME = ImmIsIME(keyboardLayout);
    if (!diag->isIME) {
        // A plain keyboard layout is already Latin input.
        return TRUE;
    }

    HIMC context = ImmGetContext(hwnd);
    diag->hasContext = context != NULL;
    if (!context) {
        // Wox's custom window may not have an HIMC associated yet; the default
        // IME window still accepts the open-status control message.
        HWND imeWindow = ImmGetDefaultIMEWnd(hwnd);
        if (!imeWindow) {
            return FALSE;
        }
        diag->openBefore = (LONG)SendMessageW(imeWindow, WM_IME_CONTROL, imcGetOpenStatus, 0);
        SendMessageW(imeWindow, WM_IME_CONTROL, imcSetOpenStatus, 0);
        diag->openAfter = (LONG)SendMessageW(imeWindow, WM_IME_CONTROL, imcGetOpenStatus, 0);
        return diag->openAfter == 0;
    }

    diag->openBefore = ImmGetOpenStatus(context);
    ImmSetOpenStatus(context, FALSE);

    DWORD conversion = 0;
    DWORD sentence = 0;
    BOOL conversionRead = ImmGetConversionStatus(context, &conversion, &sentence);
    diag->conversionBefore = conversion;
    if (conversionRead && (conversion & IME_CMODE_NATIVE)) {
        ImmSetConversionStatus(context, conversion & ~IME_CMODE_NATIVE, sentence);
        conversionRead = ImmGetConversionStatus(context, &conversion, &sentence);
    }
    diag->conversionAfter = conversion;

    diag->openAfter = ImmGetOpenStatus(context);
    BOOL closed = diag->openAfter == 0;
    BOOL alphanumeric = conversionRead && !(conversion & IME_CMODE_NATIVE);
    ImmReleaseContext(hwnd, context);
    return closed || alphanumeric;
}

// ForegroundIMEIsOpen reports whether a Wox-owned foreground window currently has an open IME.
BOOL ForegroundIMEIsOpen() {
    HWND hwnd = GetForegroundWindow();
    if (!hwnd) {
        return FALSE;
    }
    DWORD processID = 0;
    DWORD threadID = GetWindowThreadProcessId(hwnd, &processID);
    if (processID != GetCurrentProcessId() || !ImmIsIME(GetKeyboardLayout(threadID))) {
        return FALSE;
    }
    HIMC context = ImmGetContext(hwnd);
    if (!context) {
        return FALSE;
    }
    BOOL open = ImmGetOpenStatus(context);
    ImmReleaseContext(hwnd, context);
    return open;
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
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"wox/util"
	"wox/util/mainthread"
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

// latinReassertDelays are measured from the query-box focus notification.
//
// WeChat IME restores its remembered state for the window asynchronously a few
// tens of milliseconds after focus, silently undoing a close issued before that.
// Closing again immediately from IMN_SETOPENSTATUS does not help: IMM then reports
// the IME as closed while it still composes Chinese. A close issued after the
// restore has settled is honored, so the state is re-checked and, only when the
// IME is open again, closed once more.
var latinReassertDelays = []time.Duration{200 * time.Millisecond, 500 * time.Millisecond}

// SwitchInputMethodABC switches the current IME to its English state and falls back to en-US when that is unavailable.
func SwitchInputMethodABC() error {
	err := switchInputMethodABCOnce()
	util.Go(context.Background(), "reassert latin input method", func() {
		var elapsed time.Duration
		for _, delay := range latinReassertDelays {
			time.Sleep(delay - elapsed)
			elapsed = delay
			mainthread.Call(func() {
				if C.ForegroundIMEIsOpen() == C.FALSE {
					return
				}
				util.GetLogger().Debug(context.Background(), fmt.Sprintf("IME re-opened itself after %s, switching to ABC again", delay))
				if retryErr := switchInputMethodABCOnUIThread(); retryErr != nil {
					util.GetLogger().Warn(context.Background(), "reassert latin input method: "+retryErr.Error())
				}
			})
		}
	})
	return err
}

func switchInputMethodABCOnce() error {
	var switchErr error
	// IMM open-status changes are bridged to the TSF thread manager of the
	// window's own thread, so they must be issued from the UI thread.
	mainthread.Call(func() {
		defer util.GoRecover(context.Background(), "switch input method panic", func(err error) {
			switchErr = err
		})
		switchErr = switchInputMethodABCOnUIThread()
	})
	return switchErr
}

func switchInputMethodABCOnUIThread() error {
	if capturedLayout := takeCapturedForegroundKeyboardLayout(); capturedLayout != 0 && C.LayoutIsIME(C.uintptr_t(capturedLayout)) == C.FALSE {
		// The previous window was already on a Latin layout; keep using it instead
		// of re-activating the IME just to close it again.
		C.RequestSwitchToForegroundLayout(C.uintptr_t(capturedLayout))
	}

	var diag C.IMESwitchDiag
	switched := C.SwitchForegroundIMEToAlphanumeric(&diag) != C.FALSE
	util.GetLogger().Debug(context.Background(), fmt.Sprintf(
		"switch IME to ABC: switched=%t foreground=%#x sameProcess=%d layout=%#x isIME=%d hasContext=%d open=%d->%d conversion=%#x->%#x",
		switched, uintptr(diag.foreground), diag.foregroundIsCurrentProcess,
		uintptr(diag.layout), diag.isIME, diag.hasContext, diag.openBefore, diag.openAfter, diag.conversionBefore, diag.conversionAfter))
	if switched {
		return nil
	}
	if diag.foregroundIsCurrentProcess == C.FALSE {
		// Wox is not the foreground window yet; switching the layout now would
		// change the previous application's input method. The delayed re-check
		// handles the IME once Wox is in front.
		util.GetLogger().Debug(context.Background(), "skip switching input method: foreground window does not belong to wox")
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
