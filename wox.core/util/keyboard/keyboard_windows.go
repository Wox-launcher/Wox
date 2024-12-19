package keyboard

/*
#include <windows.h>

int isKeyPressed(int vkCode) {
    return (GetAsyncKeyState(vkCode) & 0x8000) != 0;
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
*/
import "C"
import (
	"fmt"
	"time"
)

func simulateCopy() error {
	waitCtrlRelease()

	err := C.simulateCtrlC()
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to send Ctrl+C: %v", errMsg)
	}

	return nil
}

func simulatePaste() error {
	waitCtrlRelease()

	err := C.simulateCtrlV()
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to send Ctrl+V: %v", errMsg)
	}

	return nil
}

// when ctrl is pressed, we should wait until ctrl is released to simulate ctrl+c or ctrl+v
func waitCtrlRelease() {
	for i := 0; i < 20; i++ {
		if C.isKeyPressed(C.int(C.VK_CONTROL)) != 0 {
			time.Sleep(time.Millisecond * 50)
			continue
		} else {
			break
		}
	}
}
