//go:build wox_ui_smoke && darwin

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#include <stdint.h>
#include <string.h>

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

int woxSmokeForceTerminateApplication(int pid) {
    @autoreleasepool {
        NSRunningApplication *application = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        return application != nil && [application forceTerminate] ? 1 : 0;
    }
}

int woxSmokeFrontmostApplicationPid(void) {
    // Go tests do not pump AppKit's main run loop, so NSWorkspace can retain
    // an old foreground application. Process Manager gives a live PID without
    // requiring accessibility permission for each temporary Go test executable.
    ProcessSerialNumber process;
    pid_t pid = 0;
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    if (GetFrontProcess(&process) == noErr) {
        GetProcessPID(&process, &pid);
    }
#pragma clang diagnostic pop
    return pid;
}

char *woxSmokeFrontmostApplicationBundleID(void) {
    @autoreleasepool {
        NSRunningApplication *application = [NSRunningApplication runningApplicationWithProcessIdentifier:woxSmokeFrontmostApplicationPid()];
        if (application == nil || application.bundleIdentifier.length == 0) {
            return strdup("");
        }
        return strdup(application.bundleIdentifier.UTF8String);
    }
}

int woxSmokeSessionAllowsForegroundActivation(void) {
    @autoreleasepool {
        // A failed PID query is not evidence of a locked session.
        NSRunningApplication *application = [NSRunningApplication runningApplicationWithProcessIdentifier:woxSmokeFrontmostApplicationPid()];
        if ([application.bundleIdentifier isEqualToString:@"com.apple.loginwindow"]) {
            return 0;
        }
        return 1;
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
