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
static BOOL appHasFocusedTextInput(AXUIElementRef appElement);

// Keep this function shape for temporary native diagnostics.
// We intentionally keep it as a no-op to avoid C->Go callback instability.
static void logMessage(NSString *format, ...) {}

static BOOL isSystemOverlayOwnerName(NSString *ownerName) {
    if (!ownerName || ownerName.length == 0) {
        return NO;
    }

    return [ownerName isEqualToString:@"Window Server"] ||
           [ownerName isEqualToString:@"Dock"] ||
           [ownerName isEqualToString:@"Control Center"] ||
           [ownerName isEqualToString:@"Notification Center"] ||
           [ownerName isEqualToString:@"SystemUIServer"];
}

static BOOL isVisualFrontmostWindowOwnedByPid(pid_t pid, pid_t *frontPidOut, int *frontLayerOut, NSString **frontOwnerNameOut) {
    if (pid <= 0) {
        return NO;
    }

    if (frontPidOut) {
        *frontPidOut = 0;
    }
    if (frontLayerOut) {
        *frontLayerOut = 0;
    }
    if (frontOwnerNameOut) {
        *frontOwnerNameOut = nil;
    }

    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (!windowList) {
        // Avoid false negatives when we cannot inspect the window stack.
        logMessage(@"visual-front: window list unavailable, allow pid=%d", (int)pid);
        return YES;
    }

    CFIndex count = CFArrayGetCount(windowList);
    for (CFIndex i = 0; i < count; i++) {
        NSDictionary *windowInfo = (__bridge NSDictionary *)CFArrayGetValueAtIndex(windowList, i);
        if (![windowInfo isKindOfClass:[NSDictionary class]]) {
            continue;
        }

        NSNumber *windowAlpha = windowInfo[(id)kCGWindowAlpha];
        if (windowAlpha && [windowAlpha doubleValue] <= 0.01) {
            continue;
        }

        CFDictionaryRef boundsDict = (__bridge CFDictionaryRef)windowInfo[(id)kCGWindowBounds];
        CGRect bounds = CGRectZero;
        if (!boundsDict || !CGRectMakeWithDictionaryRepresentation(boundsDict, &bounds)) {
            continue;
        }
        if (bounds.size.width <= 1 || bounds.size.height <= 1) {
            continue;
        }

        NSNumber *ownerPid = windowInfo[(id)kCGWindowOwnerPID];
        if (!ownerPid || [ownerPid intValue] <= 0) {
            continue;
        }
        NSString *ownerName = windowInfo[(id)kCGWindowOwnerName];

        pid_t frontPid = (pid_t)[ownerPid intValue];
        NSNumber *windowLayer = windowInfo[(id)kCGWindowLayer];
        int layer = windowLayer ? [windowLayer intValue] : 0;
        if (frontPidOut) {
            *frontPidOut = frontPid;
        }
        if (frontLayerOut) {
            *frontLayerOut = layer;
        }
        if (frontOwnerNameOut) {
            *frontOwnerNameOut = ownerName;
        }
        if (frontPid == pid) {
            CFRelease(windowList);
            return YES;
        }

        if (layer == 0) {
            // Another app owns the top normal window.
            CFRelease(windowList);
            return NO;
        }

        // Floating/system layers are noisy on macOS (menu bar, overlays, HUDs).
        // We only treat a mismatched normal app window as decisive foreground.
        if (layer > 0) {
            continue;
        }
    }

    CFRelease(windowList);
    // Keep behavior permissive when no decisive foreground window is found.
    return YES;
}

static pid_t getWindowOwnerPidByWindowID(uint32_t windowID) {
    if (windowID == 0) {
        return 0;
    }

    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionIncludingWindow, windowID);
    if (!windowList) {
        return 0;
    }

    pid_t ownerPid = 0;
    CFIndex count = CFArrayGetCount(windowList);
    for (CFIndex i = 0; i < count; i++) {
        NSDictionary *windowInfo = (__bridge NSDictionary *)CFArrayGetValueAtIndex(windowList, i);
        if (![windowInfo isKindOfClass:[NSDictionary class]]) {
            continue;
        }

        NSNumber *windowNumber = windowInfo[(id)kCGWindowNumber];
        if (!windowNumber || [windowNumber unsignedIntValue] != windowID) {
            continue;
        }

        NSNumber *pidNumber = windowInfo[(id)kCGWindowOwnerPID];
        if (!pidNumber) {
            continue;
        }

        ownerPid = (pid_t)[pidNumber intValue];
        break;
    }

    CFRelease(windowList);
    return ownerPid;
}

// Returns the app that currently receives keyboard input. This is more
// reliable than frontmostApplication for non-activating floating windows.
static pid_t getKeyboardFocusedApplicationPid() {
    if (!AXIsProcessTrusted()) {
        return 0;
    }

    AXUIElementRef systemElement = AXUIElementCreateSystemWide();
    if (!systemElement) {
        return 0;
    }

    CFTypeRef focusedAppValue = NULL;
    AXError err = AXUIElementCopyAttributeValue(systemElement, kAXFocusedApplicationAttribute, &focusedAppValue);
    CFRelease(systemElement);
    if (err != kAXErrorSuccess || !focusedAppValue || CFGetTypeID(focusedAppValue) != AXUIElementGetTypeID()) {
        if (focusedAppValue) {
            CFRelease(focusedAppValue);
        }
        return 0;
    }

    pid_t focusedPid = 0;
    AXUIElementGetPid((AXUIElementRef)focusedAppValue, &focusedPid);
    CFRelease(focusedAppValue);
    return focusedPid;
}

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
        // Ignore Finder "type to search" when focus is in any text input
        // (e.g. the top-right Finder search field, rename input, etc.).
        if (appHasFocusedTextInput(app)) {
            CFRelease(app);
            return NO;
        }

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

static BOOL elementMatchesSubrole(AXUIElementRef element, CFStringRef expectedSubrole) {
    if (!element || !expectedSubrole) {
        return NO;
    }

    CFTypeRef subroleValue = NULL;
    BOOL matched = NO;
    if (AXUIElementCopyAttributeValue(element, kAXSubroleAttribute, &subroleValue) == kAXErrorSuccess && subroleValue) {
        if (CFGetTypeID(subroleValue) == CFStringGetTypeID() && CFStringCompare((CFStringRef)subroleValue, expectedSubrole, 0) == kCFCompareEqualTo) {
            matched = YES;
        }
        CFRelease(subroleValue);
    }
    return matched;
}

static BOOL elementIsTextInput(AXUIElementRef element) {
    if (!element) {
        return NO;
    }

    if (elementMatchesRole(element, kAXTextFieldRole) ||
        elementMatchesRole(element, kAXComboBoxRole) ||
        elementMatchesRole(element, CFSTR("AXSearchField")) ||
        elementMatchesRole(element, CFSTR("AXTextArea"))) {
        return YES;
    }

    if (elementMatchesSubrole(element, CFSTR("AXSearchField")) ||
        elementMatchesSubrole(element, CFSTR("AXSecureTextField"))) {
        return YES;
    }

    CFTypeRef editableValue = NULL;
    BOOL editable = NO;
    if (AXUIElementCopyAttributeValue(element, CFSTR("AXEditable"), &editableValue) == kAXErrorSuccess && editableValue) {
        if (CFGetTypeID(editableValue) == CFBooleanGetTypeID()) {
            editable = CFBooleanGetValue((CFBooleanRef)editableValue);
        }
        CFRelease(editableValue);
    }

    return editable;
}

static BOOL appHasFocusedTextInput(AXUIElementRef appElement) {
    if (!appElement) {
        return NO;
    }

    AXUIElementRef focusedElement = NULL;
    AXError err = AXUIElementCopyAttributeValue(appElement, kAXFocusedUIElementAttribute, (CFTypeRef *)&focusedElement);
    if (err != kAXErrorSuccess || !focusedElement || CFGetTypeID(focusedElement) != AXUIElementGetTypeID()) {
        if (focusedElement) {
            CFRelease(focusedElement);
        }
        return NO;
    }

    BOOL isTextInput = elementIsTextInput(focusedElement);
    CFRelease(focusedElement);
    return isTextInput;
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
        if (!appHasFocusedTextInput(appElement)) {
            found = getAXWindowRect(dialogWindow, x, y, w, h);
        }
        CFRelease(dialogWindow);
    }

    CFRelease(appElement);
    return found;
}

static void deactivateIfNeeded() {
    if (gCurrentState != MonitorContextStateNone) {
        logMessage(@"state: deactivate pid=%d state=%ld", (int)gCurrentPid, (long)gCurrentState);
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

    logMessage(@"state: activate pid=%d state=%ld rect=(%d,%d,%d,%d)", (int)pid, (long)state, x, y, w, h);
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

    // Non-activating panels (e.g. iTerm2 hotkey window) may receive keyboard
    // focus while Finder remains frontmost. Prefer the focused app pid.
    pid_t focusedPid = getKeyboardFocusedApplicationPid();
    if (focusedPid > 0 && focusedPid != pid) {
        NSRunningApplication *focusedApp = [NSRunningApplication runningApplicationWithProcessIdentifier:focusedPid];
        if (focusedApp) {
            activeApp = focusedApp;
            pid = focusedPid;
        } else {
            deactivateIfNeeded();
            return;
        }
    }

    pid_t visualFrontPid = 0;
    int visualFrontLayer = 0;
    NSString *visualFrontOwnerName = nil;
    // Keep a visual foreground sanity check for normal app switches when AX
    // focus info is missing or delayed.
    if (!isVisualFrontmostWindowOwnedByPid(pid, &visualFrontPid, &visualFrontLayer, &visualFrontOwnerName)) {
        NSString *bundleId = [activeApp bundleIdentifier];
        logMessage(@"evaluate: visual mismatch activePid=%d bundle=%@ frontPid=%d frontLayer=%d frontOwner=%@", (int)pid, bundleId ? bundleId : @"<nil>", (int)visualFrontPid, visualFrontLayer, visualFrontOwnerName ? visualFrontOwnerName : @"<nil>");
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
                    logMessage(@"key: ignored key=%c state=none pid=%d", (char)ch, (int)gCurrentPid);
                    return;
                }

                NSInteger eventWindowNumber = [event windowNumber];
                if (eventWindowNumber > 0) {
                    pid_t eventOwnerPid = getWindowOwnerPidByWindowID((uint32_t)eventWindowNumber);
                    // Drop keys from a different window owner (e.g. iTerm2
                    // hotkey/floating window) even if Finder stays frontmost.
                    if (eventOwnerPid > 0 && gCurrentPid > 0 && eventOwnerPid != gCurrentPid) {
                        deactivateIfNeeded();
                        return;
                    }
                }

                pid_t focusedPid = getKeyboardFocusedApplicationPid();
                // Extra guard for windows that do not expose a stable window id.
                if (focusedPid > 0 && gCurrentPid > 0 && focusedPid != gCurrentPid) {
                    deactivateIfNeeded();
                    return;
                }

                logMessage(@"key: forward key=%c state=%ld pid=%d", (char)ch, (long)gCurrentState, (int)gCurrentPid);
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
