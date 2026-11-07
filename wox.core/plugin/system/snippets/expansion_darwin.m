#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#import <Carbon/Carbon.h>
#include <stdint.h>
#include <stdatomic.h>

// Shared keyboard replay events carry this tag and are ignored by Wox's event tap.
static const int64_t snippetEventTag = 0x574f5854;

int snippetMacTrusted(void) { return AXIsProcessTrusted() ? 1 : 0; }

// Accessibility calls run on this observer thread, never on the event-tap thread.
static _Atomic uint64_t snippetCachedTarget = 0;
static NSThread *snippetFocusThread;
static CFRunLoopRef snippetFocusLoop;
static dispatch_semaphore_t snippetFocusReady;
static dispatch_semaphore_t snippetFocusStopped;
static AXObserverRef snippetFocusObserver;
static id snippetActivationObserver;

static void snippetRefreshFocus(void) {
    AXUIElementRef system = AXUIElementCreateSystemWide();
    CFTypeRef focused = NULL;
    AXError error = AXUIElementCopyAttributeValue(system, kAXFocusedUIElementAttribute, &focused);
    CFRelease(system);
    if (error != kAXErrorSuccess || !focused || CFGetTypeID(focused) != AXUIElementGetTypeID()) {
        if (focused) CFRelease(focused);
        return;
    }
    CFTypeRef subrole = NULL;
    AXError subroleError = AXUIElementCopyAttributeValue((AXUIElementRef)focused, kAXSubroleAttribute, &subrole);
    BOOL secure = subrole && CFEqual(subrole, CFSTR("AXSecureTextField"));
    if (subrole) CFRelease(subrole);
    pid_t pid = 0;
    AXError pidError = AXUIElementGetPid((AXUIElementRef)focused, &pid);
    if (secure) {
        atomic_store(&snippetCachedTarget, 0);
    } else if ((subroleError == kAXErrorSuccess || subroleError == kAXErrorNoValue) && pidError == kAXErrorSuccess && pid > 0) {
        atomic_store(&snippetCachedTarget, ((uint64_t)(uint32_t)pid << 32) | (uint32_t)CFHash(focused));
    }
    CFRelease(focused);
}

static void snippetFocusChanged(AXObserverRef observer, AXUIElementRef element, CFStringRef notification, void *context) {
    snippetRefreshFocus();
}

static void snippetObserveFrontmost(void) {
    if (snippetFocusObserver) {
        CFRunLoopRemoveSource(CFRunLoopGetCurrent(), AXObserverGetRunLoopSource(snippetFocusObserver), kCFRunLoopDefaultMode);
        CFRelease(snippetFocusObserver);
        snippetFocusObserver = NULL;
    }
    pid_t pid = [[[NSWorkspace sharedWorkspace] frontmostApplication] processIdentifier];
    AXObserverRef observer = NULL;
    if (pid > 0 && AXObserverCreate(pid, snippetFocusChanged, &observer) == kAXErrorSuccess) {
        AXUIElementRef app = AXUIElementCreateApplication(pid);
        if (AXObserverAddNotification(observer, app, kAXFocusedUIElementChangedNotification, NULL) == kAXErrorSuccess) {
            snippetFocusObserver = observer;
            CFRunLoopAddSource(CFRunLoopGetCurrent(), AXObserverGetRunLoopSource(observer), kCFRunLoopDefaultMode);
        } else {
            CFRelease(observer);
        }
        CFRelease(app);
    }
    snippetRefreshFocus();
}

int snippetMacStartFocusObserver(void) {
    snippetFocusReady = dispatch_semaphore_create(0);
    snippetFocusStopped = dispatch_semaphore_create(0);
    snippetFocusThread = [[NSThread alloc] initWithBlock:^{
        @autoreleasepool {
            snippetFocusLoop = CFRunLoopGetCurrent();
            CFRetain(snippetFocusLoop);
            [[NSRunLoop currentRunLoop] addPort:[NSPort port] forMode:NSDefaultRunLoopMode];
            snippetActivationObserver = [[[NSWorkspace sharedWorkspace] notificationCenter]
                addObserverForName:NSWorkspaceDidActivateApplicationNotification object:nil queue:nil usingBlock:^(NSNotification *notification) {
                    CFRunLoopPerformBlock(snippetFocusLoop, kCFRunLoopDefaultMode, ^{ snippetObserveFrontmost(); });
                    CFRunLoopWakeUp(snippetFocusLoop);
                }];
            // Do not wait for the initial AX request before the keyboard hook starts.
            CFRunLoopPerformBlock(snippetFocusLoop, kCFRunLoopDefaultMode, ^{ snippetObserveFrontmost(); });
            dispatch_semaphore_signal(snippetFocusReady);
            CFRunLoopRun();
            [[[NSWorkspace sharedWorkspace] notificationCenter] removeObserver:snippetActivationObserver];
            snippetActivationObserver = nil;
            if (snippetFocusObserver) {
                CFRunLoopRemoveSource(snippetFocusLoop, AXObserverGetRunLoopSource(snippetFocusObserver), kCFRunLoopDefaultMode);
                CFRelease(snippetFocusObserver);
                snippetFocusObserver = NULL;
            }
            CFRelease(snippetFocusLoop);
            snippetFocusLoop = NULL;
            dispatch_semaphore_signal(snippetFocusStopped);
        }
    }];
    [snippetFocusThread start];
    dispatch_semaphore_wait(snippetFocusReady, DISPATCH_TIME_FOREVER);
    return 1;
}

void snippetMacStopFocusObserver(void) {
    CFRunLoopPerformBlock(snippetFocusLoop, kCFRunLoopDefaultMode, ^{ CFRunLoopStop(snippetFocusLoop); });
    CFRunLoopWakeUp(snippetFocusLoop);
    dispatch_semaphore_wait(snippetFocusStopped, DISPATCH_TIME_FOREVER);
    snippetFocusThread = nil;
    atomic_store(&snippetCachedTarget, 0);
}

static int snippetMacReadInputSource(void) {
    TISInputSourceRef source = TISCopyCurrentKeyboardInputSource();
    if (source) {
        CFTypeRef type = TISGetInputSourceProperty(source, kTISPropertyInputSourceType);
        BOOL isInputMode = type && CFEqual(type, kTISTypeKeyboardInputMode);
        CFRelease(source);
        if (isInputMode) return 0;
    }
    return 1;
}

int snippetMacInputSourceAllowsExpansion(void) {
    if ([NSThread isMainThread]) return snippetMacReadInputSource();

    // HIToolbox asserts that input source properties are read on the main queue.
    // A bounded wait also lets shutdown finish if the main thread is waiting for the worker.
    dispatch_semaphore_t ready = dispatch_semaphore_create(0);
    dispatch_retain(ready);
    __block int allowed = 0;
    dispatch_async(dispatch_get_main_queue(), ^{
        allowed = snippetMacReadInputSource();
        dispatch_semaphore_signal(ready);
        dispatch_release(ready);
    });
    long result = dispatch_semaphore_wait(ready, dispatch_time(DISPATCH_TIME_NOW, 100 * NSEC_PER_MSEC));
    dispatch_release(ready);
    return result == 0 ? allowed : 0;
}

uint64_t snippetMacTarget(void) {
    if (!AXIsProcessTrusted() || IsSecureEventInputEnabled() || !snippetMacInputSourceAllowsExpansion()) return 0;
    return atomic_load(&snippetCachedTarget);
}

int snippetMacModifiersHeld(void) {
    CGEventFlags flags = CGEventSourceFlagsState(kCGEventSourceStateCombinedSessionState);
    return (flags & (kCGEventFlagMaskShift | kCGEventFlagMaskControl | kCGEventFlagMaskAlternate | kCGEventFlagMaskCommand)) != 0;
}

static int snippetMacPostKey(CGKeyCode key, bool down, CGEventFlags flags) {
    CGEventRef event = CGEventCreateKeyboardEvent(NULL, key, down);
    if (!event) return 0;
    CGEventSetFlags(event, flags);
    CGEventSetIntegerValueField(event, kCGEventSourceUserData, snippetEventTag);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
    return 1;
}

int snippetMacBackspaces(unsigned int count) {
    for (unsigned int i = 0; i < count; i++) {
        if (!snippetMacPostKey(0x33, true, 0) || !snippetMacPostKey(0x33, false, 0)) return 0;
    }
    return 1;
}

int snippetMacType(const uint16_t *text, unsigned int length) {
    for (unsigned int offset = 0; offset < length;) {
        unsigned int count = length - offset < 20 ? length - offset : 20;
        if (offset + count < length && CFStringIsSurrogateHighCharacter(text[offset + count - 1])) count--;
        for (int down = 1; down >= 0; down--) {
            CGEventRef event = CGEventCreateKeyboardEvent(NULL, 0, down);
            if (!event) return 0;
            CGEventKeyboardSetUnicodeString(event, count, text + offset);
            CGEventSetIntegerValueField(event, kCGEventSourceUserData, snippetEventTag);
            CGEventPost(kCGHIDEventTap, event);
            CFRelease(event);
        }
        offset += count;
    }
    return 1;
}

int snippetMacPaste(void) {
    return snippetMacPostKey(0x09, true, kCGEventFlagMaskCommand) &&
           snippetMacPostKey(0x09, false, kCGEventFlagMaskCommand);
}

int64_t snippetMacClipboardSequence(void) {
    @autoreleasepool { return (int64_t)[[NSPasteboard generalPasteboard] changeCount]; }
}
