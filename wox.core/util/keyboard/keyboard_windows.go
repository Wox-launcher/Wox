package keyboard

/*
#include <windows.h>

int isKeyPressed(int vkCode) {
    return (GetAsyncKeyState(vkCode) & 0x8000) != 0;
}

int isCapsLockEnabled() {
    return (GetKeyState(VK_CAPITAL) & 0x0001) != 0;
}

static INPUT modifierKeyUp(WORD vk, DWORD extraFlags) {
    INPUT ip;
    ZeroMemory(&ip, sizeof(ip));
    ip.type = INPUT_KEYBOARD;
    ip.ki.wVk = vk;
    ip.ki.dwFlags = KEYEVENTF_KEYUP | extraFlags;
    return ip;
}

// releaseBlockingModifiers synthesizes key-ups for Shift/Alt/Win so Ctrl+C/V
// is not interpreted as Ctrl+Shift+C while the trigger chord is still held.
// Ctrl is left alone because simulateCtrlC/V send their own Control events.
const char* releaseBlockingModifiers() {
    INPUT ip[8];
    int n = 0;
    if (isKeyPressed(VK_SHIFT)) {
        ip[n++] = modifierKeyUp(VK_LSHIFT, 0);
        ip[n++] = modifierKeyUp(VK_RSHIFT, KEYEVENTF_EXTENDEDKEY);
    }
    if (isKeyPressed(VK_MENU)) {
        ip[n++] = modifierKeyUp(VK_LMENU, 0);
        ip[n++] = modifierKeyUp(VK_RMENU, KEYEVENTF_EXTENDEDKEY);
    }
    if (isKeyPressed(VK_LWIN)) {
        ip[n++] = modifierKeyUp(VK_LWIN, KEYEVENTF_EXTENDEDKEY);
    }
    if (isKeyPressed(VK_RWIN)) {
        ip[n++] = modifierKeyUp(VK_RWIN, KEYEVENTF_EXTENDEDKEY);
    }
    if (n == 0) {
        return NULL;
    }
    UINT res = SendInput(n, ip, sizeof(INPUT));
    if (res != (UINT)n) {
        return "Failed to release blocking modifiers";
    }
    return NULL;
}

const char* simulateCtrlC() {
    INPUT ip[4];
    ZeroMemory(ip, sizeof(ip));

    ip[0].type = INPUT_KEYBOARD;
    ip[0].ki.wVk = VK_CONTROL;

    ip[1].type = INPUT_KEYBOARD;
    ip[1].ki.wVk = 'C';

    ip[2].type = INPUT_KEYBOARD;
    ip[2].ki.wVk = 'C';
    ip[2].ki.dwFlags = KEYEVENTF_KEYUP;

    ip[3].type = INPUT_KEYBOARD;
    ip[3].ki.wVk = VK_CONTROL;
    ip[3].ki.dwFlags = KEYEVENTF_KEYUP;

    UINT res = SendInput(4, ip, sizeof(INPUT));
    if (res != 4) {
        return "Failed to send all input events";
    }

    return NULL;
}


const char* simulateCtrlV() {
    INPUT ip[4];
    ZeroMemory(ip, sizeof(ip));

    ip[0].type = INPUT_KEYBOARD;
    ip[0].ki.wVk = VK_CONTROL;

    ip[1].type = INPUT_KEYBOARD;
    ip[1].ki.wVk = 'V';

    ip[2].type = INPUT_KEYBOARD;
    ip[2].ki.wVk = 'V';
    ip[2].ki.dwFlags = KEYEVENTF_KEYUP;

    ip[3].type = INPUT_KEYBOARD;
    ip[3].ki.wVk = VK_CONTROL;
    ip[3].ki.dwFlags = KEYEVENTF_KEYUP;

    UINT res = SendInput(4, ip, sizeof(INPUT));
    if (res != 4) {
        return "Failed to send all input events";
    }

    return NULL;
}

const char* simulateCapsLockPress() {
    INPUT ip[2];
    ZeroMemory(ip, sizeof(ip));

    ip[0].type = INPUT_KEYBOARD;
    ip[0].ki.wVk = VK_CAPITAL;

    ip[1].type = INPUT_KEYBOARD;
    ip[1].ki.wVk = VK_CAPITAL;
    ip[1].ki.dwFlags = KEYEVENTF_KEYUP;

    UINT res = SendInput(2, ip, sizeof(INPUT));
    if (res != 2) {
        return "Failed to send all input events";
    }

    return NULL;
}

const char* setCapsLockState(int enabled) {
    if (isCapsLockEnabled() == (enabled != 0)) {
        return NULL;
    }

    return simulateCapsLockPress();
}

// simulateType sends Unicode text via SendInput with KEYEVENTF_UNICODE.
// Multiline dictation is pasted by the caller; Return keys can submit messages.
const char* simulateType(const unsigned short* codepoints, int count) {
    for (int i = 0; i < count; i++) {
        unsigned short cp = codepoints[i];

        INPUT inputs[2];
        ZeroMemory(inputs, sizeof(inputs));

        inputs[0].type = INPUT_KEYBOARD;
        inputs[0].ki.wScan = cp;
        inputs[0].ki.dwFlags = KEYEVENTF_UNICODE;

        inputs[1].type = INPUT_KEYBOARD;
        inputs[1].ki.wScan = cp;
        inputs[1].ki.dwFlags = KEYEVENTF_UNICODE | KEYEVENTF_KEYUP;

        UINT res = SendInput(2, inputs, sizeof(INPUT));
        if (res != 2) {
            return "Failed to send all input events for typing";
        }
    }

    return NULL;
}
*/
import "C"
import (
	"fmt"
	"strings"
	"time"
	"unicode/utf16"
)

func simulateCopy() error {
	waitModifiersRelease()

	err := C.simulateCtrlC()
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to send Ctrl+C: %v", errMsg)
	}

	return nil
}

func simulatePaste() error {
	waitModifiersRelease()

	err := C.simulateCtrlV()
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to send Ctrl+V: %v", errMsg)
	}

	return nil
}

// simulateBackspace is a no-op on Windows: the WH_KEYBOARD_LL hook consumes
// CapsLock combo events before the system sees them, so no stray character
// is typed and no backspace is needed.
func simulateBackspace() error {
	return nil
}

func simulateCapsLockPress() error {
	err := C.simulateCapsLockPress()
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to send CapsLock: %v", errMsg)
	}

	return nil
}

func setCapsLockState(enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}

	err := C.setCapsLockState(C.int(value))
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to set CapsLock state: %v", errMsg)
	}

	return nil
}

func isCapsLockEnabled() bool {
	return C.isCapsLockEnabled() != 0
}

func isKeyPressed(key Key) bool {
	vkCode, err := keyToWindowsVK(key)
	if err != nil {
		return false
	}

	return C.isKeyPressed(C.int(vkCode)) != 0
}

func simulateType(text string) error {
	if text == "" {
		return nil
	}
	if strings.ContainsAny(text, "\r\n") {
		return fmt.Errorf("multiline text requires clipboard paste on Windows")
	}
	waitModifiersRelease()
	if err := waitCtrlRelease(func() bool { return C.isKeyPressed(C.int(C.VK_CONTROL)) != 0 }); err != nil {
		return err
	}
	// Convert UTF-8 string to UTF-16 code units for KEYEVENTF_UNICODE.
	codepoints := utf16.Encode([]rune(text))
	if len(codepoints) == 0 {
		return nil
	}
	err := C.simulateType((*C.ushort)(&codepoints[0]), C.int(len(codepoints)))
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to type text: %v", errMsg)
	}
	return nil
}

// waitCtrlRelease protects Unicode typing, which does not send its own Ctrl
// events like copy/paste. The predicate keeps tests independent of real input.
func waitCtrlRelease(isPressed func() bool) error {
	deadline := time.Now().Add(time.Second)
	for isPressed() {
		if !time.Now().Before(deadline) {
			return fmt.Errorf("failed to type text: timed out waiting for Ctrl to be released")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

// waitModifiersRelease clears Shift/Alt/Win before simulated Ctrl+C/V/type.
// Waiting for the physical chord to come up made query hotkeys stall for hundreds
// of milliseconds. Synthesized key-ups are enough for the target app to see a
// plain Ctrl+C while the user is still holding the trigger. Ctrl is not released
// here because the simulated copy/paste sequence sends Control itself.
func waitModifiersRelease() {
	if err := C.releaseBlockingModifiers(); err == nil {
		return
	}
	for i := 0; i < 20; i++ {
		isAltPressed := C.isKeyPressed(C.int(C.VK_MENU)) != 0
		isShiftPressed := C.isKeyPressed(C.int(C.VK_SHIFT)) != 0
		isLWinPressed := C.isKeyPressed(C.int(C.VK_LWIN)) != 0
		isRWinPressed := C.isKeyPressed(C.int(C.VK_RWIN)) != 0
		if isAltPressed || isShiftPressed || isLWinPressed || isRWinPressed {
			time.Sleep(time.Millisecond * 5)
			continue
		}
		break
	}
}
