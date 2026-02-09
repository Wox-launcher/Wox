#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <ctype.h>

extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog, int x, int y, int w, int h);
extern void fileExplorerDeactivatedCallbackCGO(void);
extern void fileExplorerKeyDownCallbackCGO(char key);

typedef NS_ENUM(NSInteger, MonitorContextState) {
    MonitorContextStateNone = 0,
    MonitorContextStateExplorer,
    MonitorContextStateDialog
};

static id gAppActivationObserver = nil;
static id gKeyDownObserver = nil;
static AXObserverRef gFrontmostWindowObserver = NULL;
static pid_t gObservedPid = 0;
static pid_t gCurrentPid = 0;
static MonitorContextState gCurrentState = MonitorContextStateNone;
static int gCurrentX = 0;
static int gCurrentY = 0;
static int gCurrentW = 0;
static int gCurrentH = 0;
static CFStringRef gAXWindowNumberAttribute = CFSTR("AXWindowNumber");

static BOOL getFinderWindowRectByWindowID(pid_t pid, uint32_t targetWindowID, int *x, int *y, int *w, int *h) {
    if (pid <= 0 || targetWindowID == 0) {
        return NO;
    }

    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (!windowList) {
        return NO;
    }

    BOOL found = NO;
    CFIndex count = CFArrayGetCount(windowList);
    for (CFIndex i = 0; i < count; i++) {
        NSDictionary *windowInfo = (__bridge NSDictionary *)CFArrayGetValueAtIndex(windowList, i);
        if (![windowInfo isKindOfClass:[NSDictionary class]]) {
            continue;
        }

        NSNumber *windowPid = windowInfo[(id)kCGWindowOwnerPID];
        if (!windowPid || [windowPid intValue] != pid) {
            continue;
        }

        NSNumber *windowNumber = windowInfo[(id)kCGWindowNumber];
        if (!windowNumber || [windowNumber unsignedIntValue] != targetWindowID) {
            continue;
        }

        NSNumber *windowLayer = windowInfo[(id)kCGWindowLayer];
        if (windowLayer && [windowLayer intValue] != 0) {
            continue;
        }

        CFDictionaryRef boundsDict = (__bridge CFDictionaryRef)windowInfo[(id)kCGWindowBounds];
        CGRect bounds = CGRectZero;
        if (!boundsDict || !CGRectMakeWithDictionaryRepresentation(boundsDict, &bounds)) {
            continue;
        }

        if (bounds.size.width <= 0 || bounds.size.height <= 0) {
            continue;
        }

        *x = (int)bounds.origin.x;
        *y = (int)bounds.origin.y;
        *w = (int)bounds.size.width;
        *h = (int)bounds.size.height;
        found = YES;
        break;
    }

    CFRelease(windowList);
    return found;
}

static BOOL getFrontmostFinderWindowRect(pid_t pid, int *x, int *y, int *w, int *h) {
    if (pid <= 0) {
        return NO;
    }

    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (!windowList) {
        return NO;
    }

    BOOL found = NO;
    CFIndex count = CFArrayGetCount(windowList);
    for (CFIndex i = 0; i < count; i++) {
        NSDictionary *windowInfo = (__bridge NSDictionary *)CFArrayGetValueAtIndex(windowList, i);
        if (![windowInfo isKindOfClass:[NSDictionary class]]) {
            continue;
        }

        NSNumber *windowLayer = windowInfo[(id)kCGWindowLayer];
        if (windowLayer && [windowLayer intValue] != 0) {
            continue;
        }

        NSNumber *windowPid = windowInfo[(id)kCGWindowOwnerPID];
        if (!windowPid || [windowPid intValue] != pid) {
            continue;
        }

        CFDictionaryRef boundsDict = (__bridge CFDictionaryRef)windowInfo[(id)kCGWindowBounds];
        CGRect bounds = CGRectZero;
        if (!boundsDict || !CGRectMakeWithDictionaryRepresentation(boundsDict, &bounds)) {
            continue;
        }

        if (bounds.size.width <= 0 || bounds.size.height <= 0) {
            continue;
        }

        *x = (int)bounds.origin.x;
        *y = (int)bounds.origin.y;
        *w = (int)bounds.size.width;
        *h = (int)bounds.size.height;
        found = YES;
        break;
    }

    CFRelease(windowList);
    return found;
}

static BOOL isEligibleFinderWindow(AXUIElementRef windowElement) {
    if (!windowElement) {
        return NO;
    }

    BOOL isEligible = NO;
    CFTypeRef roleValue = NULL;
    CFTypeRef subroleValue = NULL;
    AXError roleErr = AXUIElementCopyAttributeValue(windowElement, kAXRoleAttribute, &roleValue);
    AXError subroleErr = AXUIElementCopyAttributeValue(windowElement, kAXSubroleAttribute, &subroleValue);

    if (roleErr == kAXErrorSuccess && roleValue && CFGetTypeID(roleValue) == CFStringGetTypeID() &&
        CFStringCompare((CFStringRef)roleValue, kAXWindowRole, 0) == kCFCompareEqualTo) {
        isEligible = YES;

        if (subroleErr == kAXErrorSuccess && subroleValue && CFGetTypeID(subroleValue) == CFStringGetTypeID()) {
            if (CFStringCompare((CFStringRef)subroleValue, kAXStandardWindowSubrole, 0) == kCFCompareEqualTo) {
                isEligible = YES;
            } else if (CFStringCompare((CFStringRef)subroleValue, CFSTR("AXDesktop"), 0) == kCFCompareEqualTo ||
                       CFStringCompare((CFStringRef)subroleValue, CFSTR("AXDesktopWindow"), 0) == kCFCompareEqualTo) {
                isEligible = NO;
            }
        }
    }

    if (roleValue) {
        CFRelease(roleValue);
    }
    if (subroleValue) {
        CFRelease(subroleValue);
    }

    return isEligible;
}

static BOOL getFocusedFinderWindowRect(pid_t pid, int *x, int *y, int *w, int *h) {
    if (pid <= 0) {
        return NO;
    }

    BOOL found = NO;
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (app) {
        CFTypeRef focusedWindowValue = NULL;
        AXError focusedErr = AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, &focusedWindowValue);
        if (focusedErr == kAXErrorSuccess && focusedWindowValue && CFGetTypeID(focusedWindowValue) == AXUIElementGetTypeID()) {
            AXUIElementRef focusedWindow = (AXUIElementRef)focusedWindowValue;
            if (isEligibleFinderWindow(focusedWindow)) {
                CFTypeRef windowNumberValue = NULL;
                AXError numberErr = AXUIElementCopyAttributeValue(focusedWindow, gAXWindowNumberAttribute, &windowNumberValue);
                if (numberErr == kAXErrorSuccess && windowNumberValue && CFGetTypeID(windowNumberValue) == CFNumberGetTypeID()) {
                    int32_t focusedWindowNumber = 0;
                    if (CFNumberGetValue((CFNumberRef)windowNumberValue, kCFNumberSInt32Type, &focusedWindowNumber) && focusedWindowNumber > 0) {
                        found = getFinderWindowRectByWindowID(pid, (uint32_t)focusedWindowNumber, x, y, w, h);
                    }
                }
                if (windowNumberValue) {
                    CFRelease(windowNumberValue);
                }

                if (!found) {
                    found = getFrontmostFinderWindowRect(pid, x, y, w, h);
                }
            }
        }
        if (focusedWindowValue) {
            CFRelease(focusedWindowValue);
        }
        CFRelease(app);
    }

    return found;
}

static BOOL isOpenSaveDialogWindow(AXUIElementRef windowElement) {
    if (!windowElement) {
        return NO;
    }

    CFTypeRef roleValue = NULL;
    if (AXUIElementCopyAttributeValue(windowElement, kAXRoleAttribute, &roleValue) == kAXErrorSuccess && roleValue) {
        if (CFGetTypeID(roleValue) == CFStringGetTypeID() &&
            CFStringCompare((CFStringRef)roleValue, CFSTR("AXSheet"), 0) == kCFCompareEqualTo) {
            CFRelease(roleValue);
            return YES;
        }
        CFRelease(roleValue);
    }

    CFTypeRef subroleValue = NULL;
    if (AXUIElementCopyAttributeValue(windowElement, kAXSubroleAttribute, &subroleValue) == kAXErrorSuccess && subroleValue) {
        if (CFGetTypeID(subroleValue) == CFStringGetTypeID()) {
            if (CFStringCompare((CFStringRef)subroleValue, CFSTR("AXDialog"), 0) == kCFCompareEqualTo ||
                CFStringCompare((CFStringRef)subroleValue, CFSTR("AXSystemDialog"), 0) == kCFCompareEqualTo ||
                CFStringCompare((CFStringRef)subroleValue, CFSTR("AXSheet"), 0) == kCFCompareEqualTo) {
                CFRelease(subroleValue);
                return YES;
            }
        }
        CFRelease(subroleValue);
    }

    return NO;
}

static BOOL elementMatchesRole(AXUIElementRef element, CFStringRef expectedRole) {
    if (!element || !expectedRole) {
        return NO;
    }

    CFTypeRef roleValue = NULL;
    BOOL matched = NO;
    if (AXUIElementCopyAttributeValue(element, kAXRoleAttribute, &roleValue) == kAXErrorSuccess && roleValue) {
        if (CFGetTypeID(roleValue) == CFStringGetTypeID() && CFStringCompare((CFStringRef)roleValue, expectedRole, 0) == kCFCompareEqualTo) {
            matched = YES;
        }
        CFRelease(roleValue);
    }
    return matched;
}

static BOOL elementOrDescendantMatchesRole(AXUIElementRef element, CFStringRef expectedRole, int depth) {
    if (!element || !expectedRole || depth > 8) {
        return NO;
    }

    if (elementMatchesRole(element, expectedRole)) {
        return YES;
    }

    CFArrayRef children = NULL;
    AXError childrenErr = AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, (CFTypeRef *)&children);
    if (childrenErr != kAXErrorSuccess || !children) {
        if (children) {
            CFRelease(children);
        }
        return NO;
    }

    BOOL found = NO;
    CFIndex count = CFArrayGetCount(children);
    for (CFIndex i = 0; i < count; i++) {
        AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
        if (!child || CFGetTypeID(child) != AXUIElementGetTypeID()) {
            continue;
        }
        if (elementOrDescendantMatchesRole(child, expectedRole, depth + 1)) {
            found = YES;
            break;
        }
    }

    CFRelease(children);
    return found;
}

static BOOL isLikelyOpenSaveDialogWindow(AXUIElementRef windowElement) {
    if (!isOpenSaveDialogWindow(windowElement)) {
        return NO;
    }

    // Narrow detection to file-picking dialogs to avoid false positives
    // from IME candidate windows and generic dialogs.
    BOOL hasFileList =
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXOutline"), 0) ||
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXBrowser"), 0) ||
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXTable"), 0);

    BOOL hasFileNameInput =
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXTextField"), 0) ||
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXComboBox"), 0);

    return hasFileList && hasFileNameInput;
}

static BOOL getAXWindowRect(AXUIElementRef windowElement, int *x, int *y, int *w, int *h) {
    if (!windowElement) {
        return NO;
    }

    CFTypeRef positionValue = NULL;
    CFTypeRef sizeValue = NULL;
    CGPoint position = CGPointZero;
    CGSize size = CGSizeZero;

    AXError posErr = AXUIElementCopyAttributeValue(windowElement, kAXPositionAttribute, &positionValue);
    AXError sizeErr = AXUIElementCopyAttributeValue(windowElement, kAXSizeAttribute, &sizeValue);

    BOOL ok = NO;
    if (posErr == kAXErrorSuccess && sizeErr == kAXErrorSuccess && positionValue && sizeValue &&
        CFGetTypeID(positionValue) == AXValueGetTypeID() &&
        CFGetTypeID(sizeValue) == AXValueGetTypeID() &&
        AXValueGetType((AXValueRef)positionValue) == kAXValueCGPointType &&
        AXValueGetType((AXValueRef)sizeValue) == kAXValueCGSizeType &&
        AXValueGetValue((AXValueRef)positionValue, kAXValueCGPointType, &position) &&
        AXValueGetValue((AXValueRef)sizeValue, kAXValueCGSizeType, &size) &&
        size.width > 0 && size.height > 0) {
        *x = (int)position.x;
        *y = (int)position.y;
        *w = (int)size.width;
        *h = (int)size.height;
        ok = YES;
    }

    if (positionValue) {
        CFRelease(positionValue);
    }
    if (sizeValue) {
        CFRelease(sizeValue);
    }

    return ok;
}

static AXUIElementRef copyFocusedWindow(AXUIElementRef appElement) {
    if (!appElement) {
        return NULL;
    }

    CFTypeRef focusedWindowValue = NULL;
    AXError err = AXUIElementCopyAttributeValue(appElement, kAXFocusedWindowAttribute, &focusedWindowValue);
    if (err != kAXErrorSuccess || !focusedWindowValue || CFGetTypeID(focusedWindowValue) != AXUIElementGetTypeID()) {
        if (focusedWindowValue) {
            CFRelease(focusedWindowValue);
        }
        return NULL;
    }

    return (AXUIElementRef)focusedWindowValue;
}

static AXUIElementRef copyDialogSheetFromWindow(AXUIElementRef parentWindow) {
    if (!parentWindow) {
        return NULL;
    }

    CFArrayRef sheets = NULL;
    AXError sheetsErr = AXUIElementCopyAttributeValue(parentWindow, CFSTR("AXSheets"), (CFTypeRef *)&sheets);
    if (sheetsErr != kAXErrorSuccess || !sheets) {
        if (sheets) {
            CFRelease(sheets);
        }
        return NULL;
    }

    AXUIElementRef matchedSheet = NULL;
    CFIndex count = CFArrayGetCount(sheets);
    for (CFIndex i = 0; i < count; i++) {
        AXUIElementRef sheet = (AXUIElementRef)CFArrayGetValueAtIndex(sheets, i);
        if (!sheet || CFGetTypeID(sheet) != AXUIElementGetTypeID()) {
            continue;
        }
        if (isLikelyOpenSaveDialogWindow(sheet)) {
            matchedSheet = (AXUIElementRef)CFRetain(sheet);
            break;
        }
    }

    CFRelease(sheets);
    return matchedSheet;
}

static AXUIElementRef copyOpenSaveDialogWindow(AXUIElementRef appElement) {
    AXUIElementRef focusedWindow = copyFocusedWindow(appElement);
    if (!focusedWindow) {
        return NULL;
    }

    if (isLikelyOpenSaveDialogWindow(focusedWindow)) {
        return focusedWindow;
    }

    AXUIElementRef dialogWindow = copyDialogSheetFromWindow(focusedWindow);
    CFRelease(focusedWindow);
    return dialogWindow;
}

static BOOL getOpenSaveDialogRect(pid_t pid, int *x, int *y, int *w, int *h) {
    if (pid <= 0 || !AXIsProcessTrusted()) {
        return NO;
    }

    AXUIElementRef appElement = AXUIElementCreateApplication(pid);
    if (!appElement) {
        return NO;
    }

    BOOL found = NO;
    AXUIElementRef dialogWindow = copyOpenSaveDialogWindow(appElement);
    if (dialogWindow) {
        found = getAXWindowRect(dialogWindow, x, y, w, h);
        CFRelease(dialogWindow);
    }

    CFRelease(appElement);
    return found;
}

static void deactivateIfNeeded() {
    if (gCurrentState != MonitorContextStateNone) {
        fileExplorerDeactivatedCallbackCGO();
    }
    gCurrentState = MonitorContextStateNone;
    gCurrentPid = 0;
    gCurrentX = 0;
    gCurrentY = 0;
    gCurrentW = 0;
    gCurrentH = 0;
}

static void activateIfNeeded(pid_t pid, MonitorContextState state, int x, int y, int w, int h) {
    if (pid <= 0 || state == MonitorContextStateNone) {
        deactivateIfNeeded();
        return;
    }

    if (gCurrentState == state && gCurrentPid == pid &&
        gCurrentX == x && gCurrentY == y && gCurrentW == w && gCurrentH == h) {
        return;
    }

    gCurrentState = state;
    gCurrentPid = pid;
    gCurrentX = x;
    gCurrentY = y;
    gCurrentW = w;
    gCurrentH = h;
    fileExplorerActivatedCallbackCGO((int)pid, state == MonitorContextStateDialog ? 1 : 0, x, y, w, h);
}

static void evaluateFrontmostApplicationState() {
    NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (!activeApp) {
        deactivateIfNeeded();
        return;
    }

    pid_t pid = [activeApp processIdentifier];
    if (pid <= 0) {
        deactivateIfNeeded();
        return;
    }

    NSString *bundleId = [activeApp bundleIdentifier];
    int x = 0;
    int y = 0;
    int w = 0;
    int h = 0;

    if (bundleId && [bundleId isEqualToString:@"com.apple.finder"]) {
        if (getFocusedFinderWindowRect(pid, &x, &y, &w, &h)) {
            activateIfNeeded(pid, MonitorContextStateExplorer, x, y, w, h);
        } else {
            deactivateIfNeeded();
        }
        return;
    }

    if (getOpenSaveDialogRect(pid, &x, &y, &w, &h)) {
        activateIfNeeded(pid, MonitorContextStateDialog, x, y, w, h);
        return;
    }

    deactivateIfNeeded();
}

static void frontmostWindowFocusCallback(AXObserverRef observer, AXUIElementRef element, CFStringRef notification, void *refcon) {
    evaluateFrontmostApplicationState();
}

static void stopFrontmostWindowObserver() {
    if (gFrontmostWindowObserver) {
        CFRunLoopRemoveSource(CFRunLoopGetMain(), AXObserverGetRunLoopSource(gFrontmostWindowObserver), kCFRunLoopDefaultMode);
        CFRelease(gFrontmostWindowObserver);
        gFrontmostWindowObserver = NULL;
    }
    gObservedPid = 0;
}

static void startFrontmostWindowObserver(pid_t pid) {
    if (pid <= 0 || !AXIsProcessTrusted()) {
        stopFrontmostWindowObserver();
        return;
    }

    if (gFrontmostWindowObserver && gObservedPid == pid) {
        return;
    }

    stopFrontmostWindowObserver();

    AXObserverRef observer = NULL;
    AXError err = AXObserverCreate(pid, frontmostWindowFocusCallback, &observer);
    if (err != kAXErrorSuccess || !observer) {
        return;
    }

    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (!app) {
        CFRelease(observer);
        return;
    }

    AXObserverAddNotification(observer, app, kAXFocusedWindowChangedNotification, NULL);
    AXObserverAddNotification(observer, app, kAXMainWindowChangedNotification, NULL);
    AXObserverAddNotification(observer, app, kAXCreatedNotification, NULL);

    CFRunLoopAddSource(CFRunLoopGetMain(), AXObserverGetRunLoopSource(observer), kCFRunLoopDefaultMode);

    CFRelease(app);
    gFrontmostWindowObserver = observer;
    gObservedPid = pid;
}

static void syncFrontmostWindowObserver() {
    NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (!activeApp) {
        stopFrontmostWindowObserver();
        return;
    }

    pid_t pid = [activeApp processIdentifier];
    startFrontmostWindowObserver(pid);
}

void startFileExplorerMonitor() {
    @autoreleasepool {
        if (!gAppActivationObserver) {
            gAppActivationObserver = [[NSWorkspace sharedWorkspace].notificationCenter
                addObserverForName:NSWorkspaceDidActivateApplicationNotification
                            object:nil
                             queue:[NSOperationQueue mainQueue]
                        usingBlock:^(NSNotification *notification) {
                syncFrontmostWindowObserver();
                evaluateFrontmostApplicationState();
            }];
        }

        if (!gKeyDownObserver) {
            gKeyDownObserver = [NSEvent addGlobalMonitorForEventsMatchingMask:NSEventMaskKeyDown
                                                                      handler:^(NSEvent *event) {
                if (!event) {
                    return;
                }

                NSEventModifierFlags flags = event.modifierFlags & NSEventModifierFlagDeviceIndependentFlagsMask;
                if ((flags & NSEventModifierFlagControl) ||
                    (flags & NSEventModifierFlagOption) ||
                    (flags & NSEventModifierFlagCommand)) {
                    return;
                }

                NSString *chars = event.charactersIgnoringModifiers;
                if (!chars || chars.length == 0) {
                    return;
                }

                unichar ch = [chars characterAtIndex:0];
                if (ch > 0x7F || !isalnum((int)ch)) {
                    return;
                }

                syncFrontmostWindowObserver();
                evaluateFrontmostApplicationState();
                if (gCurrentState == MonitorContextStateNone) {
                    return;
                }

                fileExplorerKeyDownCallbackCGO((char)ch);
            }];
        }

        syncFrontmostWindowObserver();
        evaluateFrontmostApplicationState();
    }
}

int getCurrentFinderWindowRect(int *x, int *y, int *w, int *h) {
    @autoreleasepool {
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp || ![[activeApp bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
            return 0;
        }

        pid_t pid = [activeApp processIdentifier];
        if (getFocusedFinderWindowRect(pid, x, y, w, h)) {
            return 1;
        }
    }
    return 0;
}

void stopFileExplorerMonitor() {
    @autoreleasepool {
        stopFrontmostWindowObserver();
        gCurrentPid = 0;
        gCurrentState = MonitorContextStateNone;
        gCurrentX = 0;
        gCurrentY = 0;
        gCurrentW = 0;
        gCurrentH = 0;

        if (gKeyDownObserver) {
            [NSEvent removeMonitor:gKeyDownObserver];
            gKeyDownObserver = nil;
        }
        if (gAppActivationObserver) {
            [[NSWorkspace sharedWorkspace].notificationCenter removeObserver:gAppActivationObserver];
            gAppActivationObserver = nil;
        }
    }
}
