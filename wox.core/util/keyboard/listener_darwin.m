#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#import <ApplicationServices/ApplicationServices.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdio.h>

extern void keyboardHotkeyTriggeredCGO(int id);
extern int keyboardHookEventCGO(int eventKind, unsigned int keyCode, unsigned int modifiers, unsigned int character);

static EventHandlerRef gHotkeyHandler = NULL;
static NSMutableDictionary<NSNumber *, NSValue *> *gHotkeyRefs = nil;
static CFMachPortRef gRawKeyboardEventTap = NULL;
static CFRunLoopSourceRef gRawKeyboardEventTapSource = NULL;

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
    char buffer[192];
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
            if (gRawKeyboardEventTap) {
                CGEventTapEnable(gRawKeyboardEventTap, true);
            }
            return event;
        }

        if (type != kCGEventKeyDown && type != kCGEventKeyUp && type != kCGEventFlagsChanged) {
            return event;
        }

        unsigned short keyCode = (unsigned short)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
        CGEventFlags flags = CGEventGetFlags(event);
        unsigned int modifiers = currentModifierMaskFromCGFlags(flags);
        unsigned int character = 0;
        int eventKind = -1;

        if (type == kCGEventFlagsChanged) {
            if (!isModifierKeyCode(keyCode)) {
                return event;
            }
            eventKind = modifierKeyPressedFromCGFlags(keyCode, flags) ? 0 : 1;
        } else if (type == kCGEventKeyDown) {
            NSEvent *nsEvent = [NSEvent eventWithCGEvent:event];
            if (!nsEvent) {
                return event;
            }
            eventKind = 0;
            character = currentCharacterCode(nsEvent);
        } else if (type == kCGEventKeyUp) {
            eventKind = 1;
        }

        if (eventKind == -1) {
            return event;
        }

        int consume = keyboardHookEventCGO(eventKind, keyCode, modifiers, character);
        if (consume != 0) {
            return NULL;
        }
        return event;
    }
}

int woxDarwinSetRawKeyboardHookEnabled(int enabled, char **errorOut) {
    @autoreleasepool {
        if (enabled) {
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
        return 1;
    }
}
