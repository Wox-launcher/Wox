package keyboard

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

const char* simulateCopy() {
    CGEventRef pressC = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)8, true);
    if (pressC == NULL) return "Unable to create press event for C";

    CGEventRef releaseC = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)8, false);
    if (releaseC == NULL) {
        CFRelease(pressC);
        return "Unable to create release event for C";
    }

    CGEventSetFlags(pressC, kCGEventFlagMaskCommand);
    CGEventSetFlags(releaseC, kCGEventFlagMaskCommand);

    CGEventPost(kCGHIDEventTap, pressC);
    CGEventPost(kCGHIDEventTap, releaseC);

    CFRelease(pressC);
    CFRelease(releaseC);

    return NULL;
}

const char* simulatePaste() {
    CGEventRef pressV = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)9, true);
    if (pressV == NULL) return "Unable to create press event for V";

    CGEventRef releaseV = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)9, false);
    if (releaseV == NULL) {
        CFRelease(pressV);
        return "Unable to create release event for V";
    }

    CGEventSetFlags(pressV, kCGEventFlagMaskCommand);
    CGEventSetFlags(releaseV, kCGEventFlagMaskCommand);

    CGEventPost(kCGHIDEventTap, pressV);
    CGEventPost(kCGHIDEventTap, releaseV);

    CFRelease(pressV);
    CFRelease(releaseV);

    return NULL;
}
*/
import "C"
import "fmt"

func simulateCopy() error {
	err := C.simulateCopy()
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to send Cmd+C: %v", errMsg)
	}

	return nil
}

func simulatePaste() error {
	err := C.simulatePaste()
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to send Cmd+V: %v", errMsg)
	}

	return nil
}
