//go:build wox_ui_smoke && darwin

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#include <stdint.h>

int woxSmokeActivateApplication(int pid) {
    @autoreleasepool {
        NSRunningApplication *application = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        if (application == nil) {
            return 0;
        }
        return [application activateWithOptions:NSApplicationActivateAllWindows] ? 1 : 0;
    }
}

int woxSmokeTerminateApplication(int pid) {
    @autoreleasepool {
        NSRunningApplication *application = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        return application != nil && [application terminate] ? 1 : 0;
    }
}

int woxSmokeFrontmostApplicationPid(void) {
    @autoreleasepool {
        NSRunningApplication *application = [[NSWorkspace sharedWorkspace] frontmostApplication];
        return application == nil ? 0 : application.processIdentifier;
    }
}

int woxSmokePostKeyboardChord(uint16_t modifierKeyCode, uint64_t flags, uint16_t keyCode) {
    CGEventRef modifierDown = CGEventCreateKeyboardEvent(NULL, modifierKeyCode, true);
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, keyCode, true);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, keyCode, false);
    CGEventRef modifierUp = CGEventCreateKeyboardEvent(NULL, modifierKeyCode, false);
    if (modifierDown == NULL || down == NULL || up == NULL || modifierUp == NULL) {
        if (modifierDown != NULL) CFRelease(modifierDown);
        if (down != NULL) CFRelease(down);
        if (up != NULL) CFRelease(up);
        if (modifierUp != NULL) CFRelease(modifierUp);
        return 0;
    }
    CGEventSetType(modifierDown, kCGEventFlagsChanged);
    CGEventSetFlags(modifierDown, flags);
    CGEventSetFlags(down, flags);
    CGEventSetFlags(up, flags);
    CGEventSetType(modifierUp, kCGEventFlagsChanged);
    CGEventSetFlags(modifierUp, 0);
    CGEventPost(kCGHIDEventTap, modifierDown);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CGEventPost(kCGHIDEventTap, modifierUp);
    CFRelease(modifierDown);
    CFRelease(down);
    CFRelease(up);
    CFRelease(modifierUp);
    return 1;
}
