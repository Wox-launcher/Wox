package keyboard

/*
#include <windows.h>

int isKeyPressed(int vkCode) {
    return (GetAsyncKeyState(vkCode) & 0x8000) != 0;
}

int isCapsLockEnabled() {
    return (GetKeyState(VK_CAPITAL) & 0x0001) != 0;
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

const char* simulateCapsLockTap() {
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

    return simulateCapsLockTap();
}
*/
import "C"
import (
	"fmt"
	"time"
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

func simulateCapsLockTap() error {
	err := C.simulateCapsLockTap()
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

// We need to wait for all modifiers to be released before simulating Ctrl+C/Ctrl+V.
// Otherwise, if the trigger hotkey includes Alt/Shift/Win, the simulated copy/paste
// may be interpreted as a different shortcut (e.g. Alt+Ctrl+C).
func waitModifiersRelease() {
	for i := 0; i < 20; i++ {
		isCtrlPressed := C.isKeyPressed(C.int(C.VK_CONTROL)) != 0
		isAltPressed := C.isKeyPressed(C.int(C.VK_MENU)) != 0
		isShiftPressed := C.isKeyPressed(C.int(C.VK_SHIFT)) != 0
		isLWinPressed := C.isKeyPressed(C.int(C.VK_LWIN)) != 0
		isRWinPressed := C.isKeyPressed(C.int(C.VK_RWIN)) != 0
		if isCtrlPressed || isAltPressed || isShiftPressed || isLWinPressed || isRWinPressed {
			time.Sleep(time.Millisecond * 50)
			continue
		} else {
			break
		}
	}
}
