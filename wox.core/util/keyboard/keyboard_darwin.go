package keyboard

/*
#cgo LDFLAGS: -framework ApplicationServices -framework IOKit
#include <ApplicationServices/ApplicationServices.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/hidsystem/IOHIDLib.h>
#include <IOKit/hidsystem/IOHIDParameter.h>
#include <IOKit/hidsystem/IOHIDShared.h>
#include <mach/mach.h>
#include <stdbool.h>
#include <stdio.h>

static char gCapsLockStateError[256];

static const char* capsLockStateError(const char* message, kern_return_t status) {
    snprintf(gCapsLockStateError, sizeof(gCapsLockStateError), "%s (status=%d)", message, status);
    return gCapsLockStateError;
}

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

const char* simulateCapsLockTap() {
    CGEventRef pressCaps = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)57, true);
    if (pressCaps == NULL) return "Unable to create press event for Caps Lock";

    CGEventRef releaseCaps = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)57, false);
    if (releaseCaps == NULL) {
        CFRelease(pressCaps);
        return "Unable to create release event for Caps Lock";
    }

    CGEventPost(kCGHIDEventTap, pressCaps);
    CGEventPost(kCGHIDEventTap, releaseCaps);

    CFRelease(pressCaps);
    CFRelease(releaseCaps);

    return NULL;
}

const char* setCapsLockState(int enabled) {
    io_service_t service = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching(kIOHIDSystemClass));
    if (!service) {
        return "Unable to find IOHIDSystem";
    }

    io_connect_t connect = IO_OBJECT_NULL;
    kern_return_t status = IOServiceOpen(service, mach_task_self(), kIOHIDParamConnectType, &connect);
    IOObjectRelease(service);
    if (status != KERN_SUCCESS) {
        return capsLockStateError("Unable to open IOHIDSystem", status);
    }

    status = IOHIDSetModifierLockState(connect, kIOHIDCapsLockState, enabled != 0);
    IOServiceClose(connect);
    if (status != KERN_SUCCESS) {
        return capsLockStateError("Unable to set Caps Lock state", status);
    }

    return NULL;
}

int isKeyPressed(unsigned short keyCode) {
    return CGEventSourceKeyState(kCGEventSourceStateHIDSystemState, (CGKeyCode)keyCode) ? 1 : 0;
}

int isCapsLockEnabled() {
    return (CGEventSourceFlagsState(kCGEventSourceStateHIDSystemState) & kCGEventFlagMaskAlphaShift) != 0;
}

int woxDarwinIsPhysicalCapsLockPressed(int *available);

// simulateType injects Unicode text through CGEventKeyboardSetUnicodeString.
// Each rune is sent as a key press+release pair with the Unicode string payload
// attached to the press event. The virtual keyCode 0 is a placeholder; the OS
// resolves the actual character from the Unicode string, not the keyCode.
const char* simulateType(const char* text) {
    // CGEventKeyboardSetUnicodeString accepts a UniChar buffer (UTF-16).
    // Convert the UTF-8 input string to UTF-16 for the CGEvent API.
    CFStringRef cfStr = CFStringCreateWithCString(NULL, text, kCFStringEncodingUTF8);
    if (cfStr == NULL) {
        return "failed to create CFString from text";
    }
    CFIndex length = CFStringGetLength(cfStr);

    // Process in chunks of up to 20 characters (CGEventKeyboardSetUnicodeString
    // supports a max of 20 UniChar per call).
    const CFIndex chunkSize = 20;
    for (CFIndex i = 0; i < length; i += chunkSize) {
        CFIndex remaining = length - i;
        if (remaining > chunkSize) {
            remaining = chunkSize;
        }

        UniChar buffer[chunkSize];
        CFStringGetCharacters(cfStr, CFRangeMake(i, remaining), buffer);

        CGEventRef keyDown = CGEventCreateKeyboardEvent(NULL, 0, true);
        if (keyDown == NULL) {
            CFRelease(cfStr);
            return "failed to create key down event";
        }
        CGEventKeyboardSetUnicodeString(keyDown, (UniCharCount)remaining, buffer);
        CGEventPost(kCGHIDEventTap, keyDown);
        CFRelease(keyDown);

        CGEventRef keyUp = CGEventCreateKeyboardEvent(NULL, 0, false);
        if (keyUp == NULL) {
            CFRelease(cfStr);
            return "failed to create key up event";
        }
        // Attach the same unicode string to the release event so IME
        // compositions commit correctly on some apps.
        CGEventKeyboardSetUnicodeString(keyUp, (UniCharCount)remaining, buffer);
        CGEventPost(kCGHIDEventTap, keyUp);
        CFRelease(keyUp);
    }

    CFRelease(cfStr);
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

// setCapsLockState uses IOHIDSystem so Caps Lock combos can undo Caps Lock state without posting another key event.
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

// isKeyPressed queries the hardware key state, which differs from the Caps Lock toggle state on macOS.
func isKeyPressed(key Key) bool {
	if key == KeyCapsLock {
		available := C.int(0)
		pressed := C.woxDarwinIsPhysicalCapsLockPressed(&available)
		return available != 0 && pressed != 0
	}

	keyCode, err := keyToDarwinKeyCode(key)
	if err != nil {
		return false
	}

	return C.isKeyPressed(C.ushort(keyCode)) != 0
}

func simulatePaste() error {
	err := C.simulatePaste()
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to send Cmd+V: %v", errMsg)
	}

	return nil
}

// simulateBackspace is a no-op on macOS: the CGEventTap consumes CapsLock
// combo events before the system sees them, so no stray character is typed
// and no backspace is needed.
func simulateBackspace() error {
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

func simulateType(text string) error {
	if text == "" {
		return nil
	}
	err := C.simulateType(C.CString(text))
	if err != nil {
		errMsg := C.GoString(err)
		return fmt.Errorf("failed to type text: %v", errMsg)
	}
	return nil
}
