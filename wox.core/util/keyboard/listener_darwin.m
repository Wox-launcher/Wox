#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#import <ApplicationServices/ApplicationServices.h>
#import <IOKit/hid/IOHIDLib.h>
#import <IOKit/hid/IOHIDKeys.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdio.h>
#include <unistd.h>

extern void keyboardHotkeyTriggeredCGO(int id);
extern void keyboardHookResetCGO(void);
extern int keyboardHookEventCGO(int eventKind, unsigned int keyCode, unsigned int modifiers, unsigned int character, unsigned int text, int nativeEventType, unsigned long long nativeFlags, int nativeCapsLockStateAvailable, int nativeCapsLockPressed);

unsigned int woxDarwinForegroundPID(void) {
    @autoreleasepool {
        NSRunningApplication *application = [[NSWorkspace sharedWorkspace] frontmostApplication];
        return application ? (unsigned int)application.processIdentifier : 0;
    }
}

static EventHandlerRef gHotkeyHandler = NULL;
static NSMutableDictionary<NSNumber *, NSValue *> *gHotkeyRefs = nil;
static CFMachPortRef gRawKeyboardEventTap = NULL;
static CFRunLoopSourceRef gRawKeyboardEventTapSource = NULL;
static IOHIDManagerRef gPhysicalKeyboardManager = NULL;
static BOOL gPhysicalKeyboardManagerReady = NO;
static BOOL gPhysicalCapsLockPressed = NO;
// Aggregate event counts locate delivery failures without retaining key codes or text.
static unsigned long long gRawDownCount, gRawUpCount, gRawFlagsCount;
static unsigned long long gRawTimeoutCount, gRawDisabledCount, gRawConversionFailureCount;

// Called on the main thread, alongside the event tap callback.
char *woxDarwinKeyboardDiagnostics(void) {
    @autoreleasepool {
        char *buffer = calloc(4096, 1);
        if (!buffer) {
            return NULL;
        }
        TISInputSourceRef source = TISCopyCurrentKeyboardInputSource();
        NSString *sourceID = source ? (__bridge NSString *)TISGetInputSourceProperty(source, kTISPropertyInputSourceID) : nil;
        snprintf(buffer, 4096,
                 "secureInput=%d accessibility=%d listenAccess=%d tapExists=%d tapEnabled=%d physicalMonitor=%d appActive=%d frontPid=%d frontApp=%s inputSource=%s down=%llu up=%llu flags=%llu timeouts=%llu disabled=%llu conversionFailures=%llu",
                 IsSecureEventInputEnabled(), AXIsProcessTrusted(), CGPreflightListenEventAccess(),
                 gRawKeyboardEventTap != NULL, gRawKeyboardEventTap && CGEventTapIsEnabled(gRawKeyboardEventTap),
                 gPhysicalKeyboardManagerReady, NSApp.active, NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier,
                 NSWorkspace.sharedWorkspace.frontmostApplication.bundleIdentifier.UTF8String ?: "unknown", sourceID.UTF8String ?: "unknown",
                 gRawDownCount, gRawUpCount, gRawFlagsCount, gRawTimeoutCount, gRawDisabledCount, gRawConversionFailureCount);
        // Report every tap owned by this process; a successful creation can still
        // have unauthorized event types removed from its requested mask.
        CGEventTapInformation taps[128];
        uint32_t tapCount = 0;
        CGError tapListError = CGGetEventTapList(128, taps, &tapCount);
        size_t used = strlen(buffer);
        snprintf(buffer + used, 4096 - used, " tapListError=%d tapListCount=%u tapListAtCapacity=%d requestedKeyboardMask=0x%llx processTaps=[",
                 tapListError, tapCount, tapCount >= 128,
                 (unsigned long long)(CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventKeyUp) | CGEventMaskBit(kCGEventFlagsChanged)));
        if (tapListError == kCGErrorSuccess) {
            for (uint32_t index = 0; index < tapCount && index < 128; index++) {
                CGEventTapInformation tap = taps[index];
                if (tap.tappingProcess != getpid()) continue;
                used = strlen(buffer);
                snprintf(buffer + used, 4096 - used,
                         "{id=%u point=%u options=%u targetPid=%d enabled=%d mask=0x%llx keyDown=%d keyUp=%d flagsChanged=%d}",
                         tap.eventTapID, (unsigned int)tap.tapPoint, (unsigned int)tap.options, tap.processBeingTapped, tap.enabled,
                         (unsigned long long)tap.eventsOfInterest,
                         !!(tap.eventsOfInterest & CGEventMaskBit(kCGEventKeyDown)),
                         !!(tap.eventsOfInterest & CGEventMaskBit(kCGEventKeyUp)),
                         !!(tap.eventsOfInterest & CGEventMaskBit(kCGEventFlagsChanged)));
            }
        }
        used = strlen(buffer);
        snprintf(buffer + used, 4096 - used, "]");
        if (source) {
            CFRelease(source);
        }
        return buffer;
    }
}

static char *copyErrorMessage(const char *message) {
    if (!message) {
        return NULL;
    }
    size_t len = strlen(message) + 1;
    char *copy = malloc(len);
    if (!copy) {
        return NULL;
    }
    memcpy(copy, message, len);
    return copy;
}

static char *copyStatusErrorMessage(const char *message, OSStatus status) {
    char buffer[128];
    snprintf(buffer, sizeof(buffer), "%s (status=%d)", message, (int)status);
    return copyErrorMessage(buffer);
}

static char *copyRawEventTapErrorMessage(const char *message) {
    BOOL accessibilityTrusted = AXIsProcessTrusted();
    char buffer[160];
    snprintf(buffer, sizeof(buffer), "%s (accessibilityTrusted=%d)", message, accessibilityTrusted ? 1 : 0);
    return copyErrorMessage(buffer);
}

static UInt32 toCarbonModifiers(unsigned int modifiers) {
    UInt32 carbon = 0;
    if (modifiers & 1) {
        carbon |= controlKey;
    }
    if (modifiers & 2) {
        carbon |= shiftKey;
    }
    if (modifiers & 4) {
        carbon |= optionKey;
    }
    if (modifiers & 8) {
        carbon |= cmdKey;
    }
    return carbon;
}

static unsigned int currentModifierMaskFromCGFlags(CGEventFlags flags) {
    unsigned int modifiers = 0;
    if (flags & kCGEventFlagMaskControl) {
        modifiers |= 1;
    }
    if (flags & kCGEventFlagMaskShift) {
        modifiers |= 2;
    }
    if (flags & kCGEventFlagMaskAlternate) {
        modifiers |= 4;
    }
    if (flags & kCGEventFlagMaskCommand) {
        modifiers |= 8;
    }
    return modifiers;
}

static BOOL isModifierKeyCode(unsigned short keyCode) {
    switch (keyCode) {
        case 54:
        case 55:
        case 57:
        case 56:
        case 58:
        case 59:
        case 60:
        case 61:
        case 62:
            return YES;
        default:
            return NO;
    }
}

static BOOL modifierKeyPressedFromCGFlags(unsigned short keyCode, CGEventFlags flags) {
    switch (keyCode) {
        case 54:
        case 55:
            return (flags & kCGEventFlagMaskCommand) != 0;
        case 57:
            return (flags & kCGEventFlagMaskAlphaShift) != 0;
        case 56:
        case 60:
            return (flags & kCGEventFlagMaskShift) != 0;
        case 58:
        case 61:
            return (flags & kCGEventFlagMaskAlternate) != 0;
        case 59:
        case 62:
            return (flags & kCGEventFlagMaskControl) != 0;
        default:
            return NO;
    }
}

static unsigned int currentCharacterCode(NSEvent *event) {
    if (!event) {
        return 0;
    }

    // Use the character produced by the active keyboard layout so raw-key
    // consumers such as Explorer type-to-search see the same text as the user.
    NSString *chars = event.charactersIgnoringModifiers;
    if (!chars || chars.length == 0) {
        return 0;
    }

    unichar ch = [chars characterAtIndex:0];
    if (ch > 0x7F || !isalnum((int)ch)) {
        return 0;
    }

    return (unsigned int)ch;
}

static void physicalKeyboardValueCallback(void *context, IOReturn result, void *sender, IOHIDValueRef value) {
    if (result != kIOReturnSuccess || !value) {
        return;
    }

    IOHIDElementRef element = IOHIDValueGetElement(value);
    if (!element) {
        return;
    }

    if (IOHIDElementGetUsagePage(element) != kHIDPage_KeyboardOrKeypad || IOHIDElementGetUsage(element) != kHIDUsage_KeyboardCapsLock) {
        return;
    }

    gPhysicalCapsLockPressed = IOHIDValueGetIntegerValue(value) != 0;
    gPhysicalKeyboardManagerReady = YES;
}

static void ensurePhysicalKeyboardMonitor(void) {
    if (gPhysicalKeyboardManager) {
        return;
    }

    gPhysicalKeyboardManager = IOHIDManagerCreate(kCFAllocatorDefault, kIOHIDOptionsTypeNone);
    if (!gPhysicalKeyboardManager) {
        return;
    }

    NSDictionary *keyboardMatch = @{
        @kIOHIDDeviceUsagePageKey: @(kHIDPage_GenericDesktop),
        @kIOHIDDeviceUsageKey: @(kHIDUsage_GD_Keyboard),
    };
    IOHIDManagerSetDeviceMatching(gPhysicalKeyboardManager, (__bridge CFDictionaryRef)keyboardMatch);
    IOHIDManagerRegisterInputValueCallback(gPhysicalKeyboardManager, physicalKeyboardValueCallback, NULL);
    IOHIDManagerScheduleWithRunLoop(gPhysicalKeyboardManager, CFRunLoopGetMain(), kCFRunLoopCommonModes);
    if (IOHIDManagerOpen(gPhysicalKeyboardManager, kIOHIDOptionsTypeNone) != kIOReturnSuccess) {
        IOHIDManagerUnscheduleFromRunLoop(gPhysicalKeyboardManager, CFRunLoopGetMain(), kCFRunLoopCommonModes);
        CFRelease(gPhysicalKeyboardManager);
        gPhysicalKeyboardManager = NULL;
        gPhysicalKeyboardManagerReady = NO;
        gPhysicalCapsLockPressed = NO;
    }
}

static void stopPhysicalKeyboardMonitor(void) {
    if (!gPhysicalKeyboardManager) {
        return;
    }

    IOHIDManagerUnscheduleFromRunLoop(gPhysicalKeyboardManager, CFRunLoopGetMain(), kCFRunLoopCommonModes);
    IOHIDManagerClose(gPhysicalKeyboardManager, kIOHIDOptionsTypeNone);
    CFRelease(gPhysicalKeyboardManager);
    gPhysicalKeyboardManager = NULL;
    gPhysicalKeyboardManagerReady = NO;
    gPhysicalCapsLockPressed = NO;
}

int woxDarwinIsPhysicalCapsLockPressed(int *available) {
    if (available) {
        *available = gPhysicalKeyboardManagerReady ? 1 : 0;
    }
    return gPhysicalCapsLockPressed ? 1 : 0;
}

static OSStatus hotkeyHandler(EventHandlerCallRef nextHandler, EventRef event, void *userData) {
    EventHotKeyID hotkeyID;
    GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hotkeyID), NULL, &hotkeyID);
    keyboardHotkeyTriggeredCGO((int)hotkeyID.id);
    return noErr;
}

int woxDarwinEnsureKeyboardReady(char **errorOut) {
    @autoreleasepool {
        if (!gHotkeyRefs) {
            gHotkeyRefs = [[NSMutableDictionary alloc] init];
        }

        if (!gHotkeyHandler) {
            EventTypeSpec eventType;
            eventType.eventClass = kEventClassKeyboard;
            eventType.eventKind = kEventHotKeyPressed;
            OSStatus status = InstallApplicationEventHandler(&hotkeyHandler, 1, &eventType, NULL, &gHotkeyHandler);
            if (status != noErr) {
                if (errorOut) {
                    *errorOut = copyStatusErrorMessage("failed to install macOS hotkey handler", status);
                }
                return 0;
            }
        }

        return 1;
    }
}

int woxDarwinHasRawKeyboardAccess(void) {
    return AXIsProcessTrusted() ? 1 : 0;
}

int woxDarwinRegisterHotkey(int id, unsigned int modifiers, unsigned int keyCode, char **errorOut) {
    @autoreleasepool {
        EventHotKeyRef hotkeyRef = NULL;
        EventHotKeyID hotkeyID;
        hotkeyID.signature = 'WOXK';
        hotkeyID.id = (UInt32)id;

        OSStatus status = RegisterEventHotKey((UInt32)keyCode, toCarbonModifiers(modifiers), hotkeyID, GetApplicationEventTarget(), 0, &hotkeyRef);
        if (status != noErr || hotkeyRef == NULL) {
            if (errorOut) {
                *errorOut = copyStatusErrorMessage("failed to register macOS hotkey", status);
            }
            return 0;
        }

        if (!gHotkeyRefs) {
            gHotkeyRefs = [[NSMutableDictionary alloc] init];
        }
        gHotkeyRefs[@(id)] = [NSValue valueWithPointer:hotkeyRef];
        return 1;
    }
}

int woxDarwinUnregisterHotkey(int id, char **errorOut) {
    @autoreleasepool {
        NSValue *value = gHotkeyRefs[@(id)];
        if (!value) {
            return 1;
        }

        EventHotKeyRef hotkeyRef = (EventHotKeyRef)[value pointerValue];
        OSStatus status = UnregisterEventHotKey(hotkeyRef);
        [gHotkeyRefs removeObjectForKey:@(id)];
        if (status != noErr) {
            if (errorOut) {
                *errorOut = copyStatusErrorMessage("failed to unregister macOS hotkey", status);
            }
            return 0;
        }
        return 1;
    }
}

static CGEventRef rawKeyboardEventTapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    @autoreleasepool {
        if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
            if (type == kCGEventTapDisabledByTimeout) gRawTimeoutCount++;
            else gRawDisabledCount++;
            keyboardHookResetCGO();
            if (gRawKeyboardEventTap) {
                CGEventTapEnable(gRawKeyboardEventTap, true);
            }
            return event;
        }

        // Replayed taps must pass through without entering the hold recognizer again.
        if (CGEventGetIntegerValueField(event, kCGEventSourceUserData) == 0x574f5854) {
            return event;
        }
        if (type != kCGEventKeyDown && type != kCGEventKeyUp && type != kCGEventFlagsChanged) {
            return event;
        }

        unsigned short keyCode = (unsigned short)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
        CGEventFlags flags = CGEventGetFlags(event);
        unsigned int modifiers = currentModifierMaskFromCGFlags(flags);
        unsigned int character = 0;
        unsigned int text = 0;
        int eventKind = -1;

        if (type == kCGEventFlagsChanged) {
            gRawFlagsCount++;
            if (!isModifierKeyCode(keyCode)) {
                return event;
            }
            eventKind = modifierKeyPressedFromCGFlags(keyCode, flags) ? 0 : 1;
        } else if (type == kCGEventKeyDown) {
            gRawDownCount++;
            NSEvent *nsEvent = [NSEvent eventWithCGEvent:event];
            if (!nsEvent) {
                gRawConversionFailureCount++;
                keyboardHookResetCGO();
                return event;
            }
            eventKind = 0;
            character = currentCharacterCode(nsEvent);
            UniChar chars[2] = {0};
            UniCharCount length = 0;
            CGEventKeyboardGetUnicodeString(event, 2, &length, chars);
            if (length == 1) text = chars[0];
        } else if (type == kCGEventKeyUp) {
            gRawUpCount++;
            eventKind = 1;
        }

        if (eventKind == -1) {
            return event;
        }

        int nativeCapsLockStateAvailable = gPhysicalKeyboardManagerReady ? 1 : 0;
        int nativeCapsLockPressed = gPhysicalKeyboardManagerReady && gPhysicalCapsLockPressed ? 1 : 0;
        int consume = keyboardHookEventCGO(eventKind, keyCode, modifiers, character, text, (int)type, (unsigned long long)flags, nativeCapsLockStateAvailable, nativeCapsLockPressed);
        if (consume != 0) {
            return NULL;
        }
        return event;
    }
}

int woxDarwinSetRawKeyboardHookEnabled(int enabled, char **errorOut) {
    @autoreleasepool {
        if (enabled) {
            // Go gates this call with a fresh-process permission probe. Repeating
            // the process-cached preflight here would reject permissions granted
            // after Wox started, even though WindowServer already accepts them.
            if (!gRawKeyboardEventTap) {
                CGEventMask mask = CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventKeyUp) | CGEventMaskBit(kCGEventFlagsChanged);
                gRawKeyboardEventTap = CGEventTapCreate(kCGSessionEventTap,
                                                         kCGHeadInsertEventTap,
                                                         kCGEventTapOptionDefault,
                                                         mask,
                                                         rawKeyboardEventTapCallback,
                                                         NULL);
                if (!gRawKeyboardEventTap) {
                    if (errorOut) {
                        *errorOut = copyRawEventTapErrorMessage("failed to create macOS raw keyboard event tap");
                    }
                    return 0;
                }

                gRawKeyboardEventTapSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, gRawKeyboardEventTap, 0);
                if (!gRawKeyboardEventTapSource) {
                    CFRelease(gRawKeyboardEventTap);
                    gRawKeyboardEventTap = NULL;
                    if (errorOut) {
                        *errorOut = copyErrorMessage("failed to create macOS raw keyboard event tap source");
                    }
                    return 0;
                }

                CFRunLoopAddSource(CFRunLoopGetMain(), gRawKeyboardEventTapSource, kCFRunLoopCommonModes);
                CGEventTapEnable(gRawKeyboardEventTap, true);
            }
            ensurePhysicalKeyboardMonitor();
            return 1;
        }

        if (gRawKeyboardEventTapSource) {
            CFRunLoopRemoveSource(CFRunLoopGetMain(), gRawKeyboardEventTapSource, kCFRunLoopCommonModes);
            CFRelease(gRawKeyboardEventTapSource);
            gRawKeyboardEventTapSource = NULL;
        }

        if (gRawKeyboardEventTap) {
            CGEventTapEnable(gRawKeyboardEventTap, false);
            CFRelease(gRawKeyboardEventTap);
            gRawKeyboardEventTap = NULL;
        }
        stopPhysicalKeyboardMonitor();
        return 1;
    }
}
