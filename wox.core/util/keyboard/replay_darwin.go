package keyboard

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>
int woxDarwinHasRawKeyboardAccess(void);
unsigned int woxDarwinForegroundPID(void);
static int woxReplayTap(unsigned int code) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)code, 1);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)code, 0);
    if (!down || !up) {
        if (down) CFRelease(down);
        if (up) CFRelease(up);
        return 0;
    }
    CGEventSetIntegerValueField(down, kCGEventSourceUserData, 0x574f5854);
    CGEventSetIntegerValueField(up, kCGEventSourceUserData, 0x574f5854);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
    return 1;
}

*/
import "C"

import "fmt"

// ForegroundInputTarget identifies the application that would receive replayed input.
func ForegroundInputTarget() uint64 {
	return uint64(C.woxDarwinForegroundPID())
}

// ReplayStandaloneTap restores a consumed short press to the foreground application.
func ReplayStandaloneTap(nativeCode uint32) error {
	if C.woxReplayTap(C.uint(nativeCode)) == 0 {
		return fmt.Errorf("failed to replay key tap")
	}
	return nil
}

// SupportsStandaloneHold reports whether the platform can consume and replay this key.
func SupportsStandaloneHold(key Key) bool {
	_, err := keyToDarwinKeyCode(key)
	return err == nil && C.woxDarwinHasRawKeyboardAccess() != 0
}

// StandaloneHoldListenerReady rejects deferred listeners that cannot consume input yet.
func StandaloneHoldListenerReady() bool {
	managerMu.Lock()
	defer managerMu.Unlock()
	return rawHookIsEnabled
}
