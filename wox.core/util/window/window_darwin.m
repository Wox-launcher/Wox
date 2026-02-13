#include <ApplicationServices/ApplicationServices.h>
#include <Cocoa/Cocoa.h>
#include <ScriptingBridge/ScriptingBridge.h>
#include <stdlib.h>
#include <unistd.h>

static char* copyPathFromAXValue(CFTypeRef value);

int getActiveWindowIcon(unsigned char **iconData) {
    @autoreleasepool {
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return 0;
        }

        NSImage *icon = [activeApp icon];
        if (!icon) {
            return 0;
        }

        CGImageRef cgRef = [icon CGImageForProposedRect:NULL context:nil hints:nil];
        NSBitmapImageRep *newRep = [[NSBitmapImageRep alloc] initWithCGImage:cgRef];
        [newRep setSize:[icon size]];
        NSData *pngData = [newRep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        if (!pngData) {
            return 0;
        }

        NSUInteger length = [pngData length];
        void *buffer = malloc(length);
        if (!buffer) {
            return 0;
        }
        memcpy(buffer, [pngData bytes], length);

        *iconData = buffer;
        return (int)length;
    }
}

char* getActiveWindowName() {
    @autoreleasepool {
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return "";
        }

        return strdup([[activeApp localizedName] UTF8String]);
    }
}

char* getProcessBundleIdentifier(int pid) {
    @autoreleasepool {
        if (pid <= 0) {
            return strdup("");
        }

        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        if (!app) {
            return strdup("");
        }

        NSString *identifier = [app bundleIdentifier];
        if (identifier && [identifier length] > 0) {
            return strdup([identifier UTF8String]);
        }

        NSString *name = [app localizedName];
        if (name && [name length] > 0) {
            return strdup([name UTF8String]);
        }

        return strdup("");
    }
}

int getActiveWindowPid() {
    @autoreleasepool {
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return -1;
        }

        return [activeApp processIdentifier];
    }
}

static BOOL elementHasSubrole(AXUIElementRef element, CFStringRef subrole) {
    CFTypeRef subroleValue = NULL;
    BOOL matched = NO;
    if (AXUIElementCopyAttributeValue(element, kAXSubroleAttribute, &subroleValue) == kAXErrorSuccess && subroleValue) {
        if (CFGetTypeID(subroleValue) == CFStringGetTypeID() && CFStringCompare(subroleValue, subrole, 0) == kCFCompareEqualTo) {
            matched = YES;
        }
        CFRelease(subroleValue);
    }
    return matched;
}

static BOOL elementHasRole(AXUIElementRef element, CFStringRef role) {
    CFTypeRef roleValue = NULL;
    BOOL matched = NO;
    if (AXUIElementCopyAttributeValue(element, kAXRoleAttribute, &roleValue) == kAXErrorSuccess && roleValue) {
        if (CFGetTypeID(roleValue) == CFStringGetTypeID() && CFStringCompare(roleValue, role, 0) == kCFCompareEqualTo) {
            matched = YES;
        }
        CFRelease(roleValue);
    }
    return matched;
}

static BOOL isOpenSaveDialogWindowElement(AXUIElementRef windowElement) {
    if (!windowElement) {
        return NO;
    }

    if (elementHasRole(windowElement, CFSTR("AXSheet"))) {
        return YES;
    }
    if (elementHasSubrole(windowElement, CFSTR("AXDialog")) ||
        elementHasSubrole(windowElement, CFSTR("AXSystemDialog")) ||
        elementHasSubrole(windowElement, CFSTR("AXSheet"))) {
        return YES;
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

static BOOL isLikelyOpenSaveDialogWindowElement(AXUIElementRef windowElement) {
    if (!windowElement) {
        return NO;
    }

    if (isOpenSaveDialogWindowElement(windowElement)) {
        return YES;
    }

    BOOL hasFileList =
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXOutline"), 0) ||
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXBrowser"), 0) ||
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXTable"), 0);

    BOOL hasFileNameInput =
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXTextField"), 0) ||
        elementOrDescendantMatchesRole(windowElement, CFSTR("AXComboBox"), 0);

    return hasFileList && hasFileNameInput;
}

static AXUIElementRef copyFocusedWindowElement(AXUIElementRef appElement) {
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

        if (isLikelyOpenSaveDialogWindowElement(sheet)) {
            matchedSheet = (AXUIElementRef)CFRetain(sheet);
            break;
        }
    }

    CFRelease(sheets);
    return matchedSheet;
}

static AXUIElementRef copyDialogWindowFromAppWindows(AXUIElementRef appElement) {
    if (!appElement) {
        return NULL;
    }

    CFArrayRef windows = NULL;
    AXError windowsErr = AXUIElementCopyAttributeValue(appElement, kAXWindowsAttribute, (CFTypeRef *)&windows);
    if (windowsErr != kAXErrorSuccess || !windows) {
        if (windows) {
            CFRelease(windows);
        }
        return NULL;
    }

    AXUIElementRef matchedWindow = NULL;
    CFIndex count = CFArrayGetCount(windows);
    for (CFIndex i = 0; i < count; i++) {
        AXUIElementRef window = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
        if (!window || CFGetTypeID(window) != AXUIElementGetTypeID()) {
            continue;
        }

        if (isLikelyOpenSaveDialogWindowElement(window)) {
            matchedWindow = (AXUIElementRef)CFRetain(window);
            break;
        }

        AXUIElementRef sheet = copyDialogSheetFromWindow(window);
        if (sheet) {
            matchedWindow = sheet;
            break;
        }
    }

    CFRelease(windows);
    return matchedWindow;
}

static AXUIElementRef copyOpenSaveDialogWindowForActiveApp(AXUIElementRef appElement) {
    AXUIElementRef focusedWindow = copyFocusedWindowElement(appElement);
    if (!focusedWindow) {
        return copyDialogWindowFromAppWindows(appElement);
    }

    if (isLikelyOpenSaveDialogWindowElement(focusedWindow)) {
        return focusedWindow;
    }

    AXUIElementRef dialogWindow = copyDialogSheetFromWindow(focusedWindow);
    CFRelease(focusedWindow);
    if (dialogWindow) {
        return dialogWindow;
    }

    return copyDialogWindowFromAppWindows(appElement);
}

static char* normalizeToDirectoryPathCString(char *pathCString) {
    if (!pathCString || pathCString[0] == '\0') {
        if (pathCString) {
            free(pathCString);
        }
        return NULL;
    }

    NSString *path = [NSString stringWithUTF8String:pathCString];
    if (!path || [path length] == 0) {
        free(pathCString);
        return NULL;
    }

    NSFileManager *fileManager = [NSFileManager defaultManager];
    BOOL isDir = NO;
    if ([fileManager fileExistsAtPath:path isDirectory:&isDir]) {
        if (isDir) {
            return pathCString;
        }

        NSString *parent = [path stringByDeletingLastPathComponent];
        if (parent && [parent length] > 0) {
            BOOL parentIsDir = NO;
            if ([fileManager fileExistsAtPath:parent isDirectory:&parentIsDir] && parentIsDir) {
                free(pathCString);
                return strdup([parent UTF8String]);
            }
        }
    }

    free(pathCString);
    return NULL;
}

static char* copyNormalizedPathFromAXAttribute(AXUIElementRef element, CFStringRef attr) {
    if (!element || !attr) {
        return NULL;
    }

    CFTypeRef value = NULL;
    AXError err = AXUIElementCopyAttributeValue(element, attr, &value);
    if (err != kAXErrorSuccess || !value) {
        if (value) {
            CFRelease(value);
        }
        return NULL;
    }

    char *result = normalizeToDirectoryPathCString(copyPathFromAXValue(value));
    CFRelease(value);
    return result;
}

static char* findDirectoryPathInElementTree(AXUIElementRef element, int depth) {
    if (!element || depth > 10) {
        return NULL;
    }

    CFStringRef attrsToCheck[] = {
        kAXDocumentAttribute,
        kAXValueAttribute,
        CFSTR("AXURL"),
        CFSTR("AXFilename"),
        kAXTitleAttribute
    };
    const size_t attrsCount = sizeof(attrsToCheck) / sizeof(attrsToCheck[0]);
    for (size_t i = 0; i < attrsCount; i++) {
        char *result = copyNormalizedPathFromAXAttribute(element, attrsToCheck[i]);
        if (result) {
            return result;
        }
    }

    CFArrayRef children = NULL;
    AXError childrenErr = AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, (CFTypeRef *)&children);
    if (childrenErr != kAXErrorSuccess || !children) {
        if (children) {
            CFRelease(children);
        }
        return NULL;
    }

    char *foundPath = NULL;
    CFIndex count = CFArrayGetCount(children);
    for (CFIndex i = 0; i < count; i++) {
        AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
        if (!child || CFGetTypeID(child) != AXUIElementGetTypeID()) {
            continue;
        }

        foundPath = findDirectoryPathInElementTree(child, depth + 1);
        if (foundPath) {
            break;
        }
    }

    CFRelease(children);
    return foundPath;
}

static char* copyDirectoryPathFromDialogContext(AXUIElementRef dialogWindow, AXUIElementRef appElement) {
    if (dialogWindow) {
        char *result = copyNormalizedPathFromAXAttribute(dialogWindow, kAXDocumentAttribute);
        if (result) {
            return result;
        }

        result = findDirectoryPathInElementTree(dialogWindow, 0);
        if (result) {
            return result;
        }
    }

    if (appElement) {
        CFTypeRef focusedElementValue = NULL;
        if (AXUIElementCopyAttributeValue(appElement, kAXFocusedUIElementAttribute, &focusedElementValue) == kAXErrorSuccess &&
            focusedElementValue &&
            CFGetTypeID(focusedElementValue) == AXUIElementGetTypeID()) {
            AXUIElementRef focusedElement = (AXUIElementRef)focusedElementValue;

            char *result = copyNormalizedPathFromAXAttribute(focusedElement, kAXValueAttribute);
            if (result) {
                CFRelease(focusedElementValue);
                return result;
            }

            result = findDirectoryPathInElementTree(focusedElement, 0);
            CFRelease(focusedElementValue);
            if (result) {
                return result;
            }
        } else if (focusedElementValue) {
            CFRelease(focusedElementValue);
        }
    }

    return NULL;
}


int isOpenSaveDialog() {
    @autoreleasepool {
        if (!AXIsProcessTrusted()) {
            return 0;
        }

        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return 0;
        }

        pid_t pid = [activeApp processIdentifier];
        AXUIElementRef appElement = AXUIElementCreateApplication(pid);
        if (!appElement) {
            return 0;
        }

        AXUIElementRef dialogWindow = copyOpenSaveDialogWindowForActiveApp(appElement);
        BOOL isDialog = dialogWindow != NULL;
        if (dialogWindow) {
            CFRelease(dialogWindow);
        }
        CFRelease(appElement);
        return isDialog ? 1 : 0;
    }
}

static AXUIElementRef findTextFieldRecursive(AXUIElementRef element, int depth) {
    if (!element || depth > 6) {
        return NULL;
    }

    CFTypeRef role = NULL;
    if (AXUIElementCopyAttributeValue(element, kAXRoleAttribute, &role) == kAXErrorSuccess && role) {
        if (CFGetTypeID(role) == CFStringGetTypeID() &&
            (CFStringCompare(role, kAXTextFieldRole, 0) == kCFCompareEqualTo ||
             CFStringCompare(role, kAXComboBoxRole, 0) == kCFCompareEqualTo)) {
            CFRelease(role);
            return (AXUIElementRef)CFRetain(element);
        }
        CFRelease(role);
    }

    CFArrayRef children = NULL;
    if (AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, (CFTypeRef *)&children) != kAXErrorSuccess || !children) {
        return NULL;
    }

    CFIndex count = CFArrayGetCount(children);
    for (CFIndex i = 0; i < count; i++) {
        AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
        AXUIElementRef found = findTextFieldRecursive(child, depth + 1);
        if (found) {
            CFRelease(children);
            return found;
        }
    }

    CFRelease(children);
    return NULL;
}

static BOOL axValueMatchesTargetFileName(CFTypeRef value, NSString *targetName) {
    if (!value || !targetName || [targetName length] == 0) {
        return NO;
    }

    NSString *candidate = nil;
    if (CFGetTypeID(value) == CFURLGetTypeID()) {
        candidate = [(__bridge NSURL *)value path];
    } else if (CFGetTypeID(value) == CFStringGetTypeID()) {
        NSString *raw = (__bridge NSString *)value;
        if ([raw hasPrefix:@"file://"]) {
            NSURL *url = [NSURL URLWithString:raw];
            if (url) {
                candidate = [url path];
            }
        }
        if (!candidate) {
            candidate = raw;
        }
    }

    if (!candidate || [candidate length] == 0) {
        return NO;
    }

    NSString *trimmed = [candidate stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
    if ([trimmed length] == 0) {
        return NO;
    }

    if ([trimmed compare:targetName options:NSCaseInsensitiveSearch] == NSOrderedSame) {
        return YES;
    }

    NSString *basename = [trimmed lastPathComponent];
    if (basename && [basename length] > 0 &&
        [basename compare:targetName options:NSCaseInsensitiveSearch] == NSOrderedSame) {
        return YES;
    }

    return NO;
}

static BOOL elementOrDescendantMatchesFileName(AXUIElementRef element, NSString *targetName, int depth) {
    if (!element || !targetName || depth > 8) {
        return NO;
    }

    CFStringRef attrs[] = {
        kAXTitleAttribute,
        kAXValueAttribute,
        CFSTR("AXFilename"),
        kAXDescriptionAttribute
    };
    const size_t attrsCount = sizeof(attrs) / sizeof(attrs[0]);
    for (size_t i = 0; i < attrsCount; i++) {
        CFTypeRef value = NULL;
        AXError err = AXUIElementCopyAttributeValue(element, attrs[i], &value);
        if (err == kAXErrorSuccess && value) {
            BOOL matched = axValueMatchesTargetFileName(value, targetName);
            CFRelease(value);
            if (matched) {
                return YES;
            }
        } else if (value) {
            CFRelease(value);
        }
    }

    CFArrayRef children = NULL;
    AXError childrenErr = AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, (CFTypeRef *)&children);
    if (childrenErr != kAXErrorSuccess || !children) {
        if (children) {
            CFRelease(children);
        }
        return NO;
    }

    BOOL matched = NO;
    CFIndex count = CFArrayGetCount(children);
    for (CFIndex i = 0; i < count; i++) {
        AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
        if (!child || CFGetTypeID(child) != AXUIElementGetTypeID()) {
            continue;
        }
        if (elementOrDescendantMatchesFileName(child, targetName, depth + 1)) {
            matched = YES;
            break;
        }
    }

    CFRelease(children);
    return matched;
}

static BOOL selectMatchingRowInListElement(AXUIElementRef listElement, NSString *targetName) {
    if (!listElement || !targetName || [targetName length] == 0) {
        return NO;
    }

    CFArrayRef rows = NULL;
    AXError rowsErr = AXUIElementCopyAttributeValue(listElement, CFSTR("AXRows"), (CFTypeRef *)&rows);
    if (rowsErr != kAXErrorSuccess || !rows) {
        if (rows) {
            CFRelease(rows);
        }
        rows = NULL;
        if (AXUIElementCopyAttributeValue(listElement, kAXChildrenAttribute, (CFTypeRef *)&rows) != kAXErrorSuccess || !rows) {
            if (rows) {
                CFRelease(rows);
            }
            return NO;
        }
    }

    BOOL selected = NO;
    CFIndex count = CFArrayGetCount(rows);
    for (CFIndex i = 0; i < count; i++) {
        AXUIElementRef row = (AXUIElementRef)CFArrayGetValueAtIndex(rows, i);
        if (!row || CFGetTypeID(row) != AXUIElementGetTypeID()) {
            continue;
        }
        if (!elementOrDescendantMatchesFileName(row, targetName, 0)) {
            continue;
        }

        if (AXUIElementSetAttributeValue(row, kAXSelectedAttribute, kCFBooleanTrue) == kAXErrorSuccess) {
            selected = YES;
        }

        CFTypeRef selectedRow = (CFTypeRef)row;
        CFArrayRef selectedRows = CFArrayCreate(kCFAllocatorDefault, &selectedRow, 1, &kCFTypeArrayCallBacks);
        if (selectedRows) {
            if (AXUIElementSetAttributeValue(listElement, CFSTR("AXSelectedRows"), selectedRows) == kAXErrorSuccess) {
                selected = YES;
            }
            CFRelease(selectedRows);
        }

        AXUIElementSetAttributeValue(row, kAXFocusedAttribute, kCFBooleanTrue);
        break;
    }

    CFRelease(rows);
    return selected;
}

static BOOL selectItemInDialogTreeByName(AXUIElementRef element, NSString *targetName, int depth) {
    if (!element || !targetName || depth > 10) {
        return NO;
    }

    if (elementHasRole(element, CFSTR("AXOutline")) ||
        elementHasRole(element, CFSTR("AXTable")) ||
        elementHasRole(element, CFSTR("AXBrowser"))) {
        if (selectMatchingRowInListElement(element, targetName)) {
            return YES;
        }
    }

    CFArrayRef children = NULL;
    AXError childrenErr = AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, (CFTypeRef *)&children);
    if (childrenErr != kAXErrorSuccess || !children) {
        if (children) {
            CFRelease(children);
        }
        return NO;
    }

    BOOL selected = NO;
    CFIndex count = CFArrayGetCount(children);
    for (CFIndex i = 0; i < count; i++) {
        AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
        if (!child || CFGetTypeID(child) != AXUIElementGetTypeID()) {
            continue;
        }
        if (selectItemInDialogTreeByName(child, targetName, depth + 1)) {
            selected = YES;
            break;
        }
    }

    CFRelease(children);
    return selected;
}

int selectInActiveFileDialog(const char* path) {
    @autoreleasepool {
        if (path == NULL) {
            return 0;
        }
        if (!AXIsProcessTrusted()) {
            return 0;
        }

        NSString *pathStr = [NSString stringWithUTF8String:path];
        if (!pathStr || [pathStr length] == 0) {
            return 0;
        }

        NSString *targetName = [pathStr lastPathComponent];
        if (!targetName || [targetName length] == 0) {
            return 0;
        }

        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return 0;
        }

        AXUIElementRef appElement = AXUIElementCreateApplication([activeApp processIdentifier]);
        if (!appElement) {
            return 0;
        }

        AXUIElementRef dialogWindow = copyOpenSaveDialogWindowForActiveApp(appElement);
        if (!dialogWindow) {
            CFRelease(appElement);
            return 0;
        }

        AXUIElementPerformAction(dialogWindow, kAXRaiseAction);
        [activeApp activateWithOptions:NSApplicationActivateIgnoringOtherApps];
        usleep(80 * 1000);

        BOOL selected = selectItemInDialogTreeByName(dialogWindow, targetName, 0);

        CFRelease(dialogWindow);
        CFRelease(appElement);
        return selected ? 1 : 0;
    }
}

int navigateActiveFileDialog(const char* path) {
    @autoreleasepool {
        if (path == NULL) {
            return 0;
        }
        if (!AXIsProcessTrusted()) {
            return 0;
        }

        NSString *pathStr = [NSString stringWithUTF8String:path];
        if (!pathStr || [pathStr length] == 0) {
            return 0;
        }

        // Cmd+Shift+G to open "Go to the folder" in dialogs
        CGEventRef gDown = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)5, true);
        CGEventRef gUp = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)5, false);
        if (!gDown || !gUp) {
            if (gDown) CFRelease(gDown);
            if (gUp) CFRelease(gUp);
            return 0;
        }
        CGEventFlags flags = kCGEventFlagMaskCommand | kCGEventFlagMaskShift;
        CGEventSetFlags(gDown, flags);
        CGEventSetFlags(gUp, flags);
        CGEventPost(kCGHIDEventTap, gDown);
        CGEventPost(kCGHIDEventTap, gUp);
        CFRelease(gDown);
        CFRelease(gUp);

        // Wait briefly for the sheet to appear. 
        // 50ms might be enough for modern Macs. If it fails, we might need to retry or increase slightly.
        usleep(150 * 1000);

        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return 0;
        }

        AXUIElementRef appElement = AXUIElementCreateApplication([activeApp processIdentifier]);
        if (!appElement) {
            return 0;
        }

        AXUIElementRef focusedWindow = NULL;
        AXError windowErr = AXUIElementCopyAttributeValue(appElement, kAXFocusedWindowAttribute, (CFTypeRef *)&focusedWindow);
        if (windowErr != kAXErrorSuccess || !focusedWindow) {
            CFRelease(appElement);
            return 0;
        }

        AXUIElementRef targetWindow = focusedWindow;
        CFRetain(targetWindow);

        // If the focused window isn't the dialog, look through all windows
        if (!elementHasRole(targetWindow, kAXSheetRole) && 
            !elementHasSubrole(targetWindow, CFSTR("AXDialog")) && 
            !elementHasSubrole(targetWindow, CFSTR("AXSystemDialog"))) {
            
            CFArrayRef windows = NULL;
            if (AXUIElementCopyAttributeValue(appElement, kAXWindowsAttribute, (CFTypeRef *)&windows) == kAXErrorSuccess && windows) {
                CFIndex count = CFArrayGetCount(windows);
                for (CFIndex i = 0; i < count; i++) {
                    AXUIElementRef window = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
                    if (window && (elementHasRole(window, kAXSheetRole) || 
                                   elementHasSubrole(window, CFSTR("AXDialog")) || 
                                   elementHasSubrole(window, CFSTR("AXSystemDialog")))) {
                        CFRelease(targetWindow);
                        targetWindow = (AXUIElementRef)CFRetain(window);
                        break;
                    }
                }
                CFRelease(windows);
            }
        }

        AXUIElementPerformAction(targetWindow, kAXRaiseAction);
        AXUIElementRef textField = findTextFieldRecursive(targetWindow, 0);

        if (textField) {
            AXUIElementSetAttributeValue(textField, kAXValueAttribute, (CFTypeRef)pathStr);
            CFRelease(textField);
        }

        [activeApp activateWithOptions:NSApplicationActivateIgnoringOtherApps];
        usleep(10 * 1000);

        NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:@"tell application \"System Events\" to key code 36"];
        [appleScript executeAndReturnError:nil];

        CFRelease(targetWindow);
        CFRelease(focusedWindow);
        CFRelease(appElement);
        return 1;
    }
}

char* getActiveFileDialogPath() {
    @autoreleasepool {
        if (!AXIsProcessTrusted()) {
            return strdup("");
        }

        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return strdup("");
        }

        AXUIElementRef appElement = AXUIElementCreateApplication([activeApp processIdentifier]);
        if (!appElement) {
            return strdup("");
        }

        AXUIElementRef dialogWindow = copyOpenSaveDialogWindowForActiveApp(appElement);
        if (!dialogWindow) {
            char *fallbackPath = copyDirectoryPathFromDialogContext(NULL, appElement);
            CFRelease(appElement);
            if (fallbackPath) {
                return fallbackPath;
            }
            return strdup("");
        }

        char *resolvedPath = copyDirectoryPathFromDialogContext(dialogWindow, appElement);
        if (resolvedPath) {
            CFRelease(dialogWindow);
            CFRelease(appElement);
            return resolvedPath;
        }

        CFRelease(dialogWindow);
        CFRelease(appElement);
        return strdup("");
    }
}

char* getFileDialogPathByPid(int pid) {
    @autoreleasepool {
        if (pid <= 0) {
            return strdup("");
        }
        if (!AXIsProcessTrusted()) {
            return strdup("");
        }

        AXUIElementRef appElement = AXUIElementCreateApplication((pid_t)pid);
        if (!appElement) {
            return strdup("");
        }

        AXUIElementRef dialogWindow = copyOpenSaveDialogWindowForActiveApp(appElement);
        if (!dialogWindow) {
            char *fallbackPath = copyDirectoryPathFromDialogContext(NULL, appElement);
            CFRelease(appElement);
            if (fallbackPath) {
                return fallbackPath;
            }
            return strdup("");
        }

        char *resolvedPath = copyDirectoryPathFromDialogContext(dialogWindow, appElement);
        if (resolvedPath) {
            CFRelease(dialogWindow);
            CFRelease(appElement);
            return resolvedPath;
        }

        CFRelease(dialogWindow);
        CFRelease(appElement);
        return strdup("");
    }
}

int activateWindowByPid(int pid) {
    @autoreleasepool {
        if (pid <= 0) {
            return 0;
        }

        if (!AXIsProcessTrusted()) {
            return 0;
        }

        AXUIElementRef appElement = AXUIElementCreateApplication(pid);
        if (!appElement) {
            return 0;
        }

        AXError setFrontmostErr = AXUIElementSetAttributeValue(
            appElement,
            kAXFrontmostAttribute,
            kCFBooleanTrue
        );

        CFArrayRef windows = NULL;
        AXError windowsErr = AXUIElementCopyAttributeValue(
            appElement,
            kAXWindowsAttribute,
            (CFTypeRef *)&windows
        );

        BOOL raised = NO;
        if (windowsErr == kAXErrorSuccess && windows && CFArrayGetCount(windows) > 0) {
            AXUIElementRef window = (AXUIElementRef)CFArrayGetValueAtIndex(windows, 0);
            if (window) {
                AXUIElementSetAttributeValue(window, kAXMainAttribute, kCFBooleanTrue);
                AXUIElementSetAttributeValue(appElement, kAXFocusedWindowAttribute, window);
                AXUIElementPerformAction(window, kAXRaiseAction);
                raised = YES;
            }
        }

        CFIndex windowCount = 0;
        if (windows) {
            windowCount = CFArrayGetCount(windows);
            CFRelease(windows);
        }
        CFRelease(appElement);

        if (setFrontmostErr != kAXErrorSuccess) {
            return 0;
        }
        if (raised) {
            return 1;
        }
        return (windowsErr == kAXErrorSuccess && windowCount == 0) ? 1 : 0;
    }
}

int isFinder(int pid) {
    @autoreleasepool {
        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        if (app && [[app bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
            return 1;
        }
        return 0;
    }
}

static NSString* getFinderWindowPathValue(id window) {
    id target = nil;
    @try {
        target = [window valueForKey:@"target"];
    } @catch (NSException *exception) {
        target = nil;
    }

    if (!target) {
        return nil;
    }

    id urlValue = nil;
    @try {
        urlValue = [target valueForKey:@"URL"];
    } @catch (NSException *exception) {
        urlValue = nil;
    }

    NSString *path = nil;
    if ([urlValue isKindOfClass:[NSURL class]]) {
        path = [(NSURL *)urlValue path];
    } else if ([urlValue isKindOfClass:[NSString class]]) {
        NSString *stringValue = (NSString *)urlValue;
        if ([stringValue hasPrefix:@"file://"]) {
            NSURL *url = [NSURL URLWithString:stringValue];
            path = [url path];
        } else {
            path = stringValue;
        }
    }

    if (!path || [path length] == 0) {
        return nil;
    }
    return path;
}

static NSString* getFinderWindowNameValue(id window) {
    id nameValue = nil;
    @try {
        nameValue = [window valueForKey:@"name"];
    } @catch (NSException *exception) {
        nameValue = nil;
    }
    if (![nameValue isKindOfClass:[NSString class]]) {
        return nil;
    }
    NSString *name = (NSString *)nameValue;
    if ([name length] == 0) {
        return nil;
    }
    return name;
}

char* getOpenFinderWindowPaths() {
    @autoreleasepool {
        id finder = [SBApplication applicationWithBundleIdentifier:@"com.apple.finder"];
        if (!finder) {
            return strdup("");
        }

        id windows = [finder valueForKey:@"windows"];
        if (![windows isKindOfClass:[NSArray class]]) {
            return strdup("");
        }

        NSArray *windowList = (NSArray *)windows;
        NSMutableArray<NSString *> *paths = [NSMutableArray arrayWithCapacity:[windowList count]];
        for (id window in windowList) {
            NSString *path = getFinderWindowPathValue(window);
            if (path) {
                [paths addObject:path];
            }
        }

        if ([paths count] == 0) {
            return strdup("");
        }

        NSString *joined = [paths componentsJoinedByString:@"\n"];
        return strdup([joined UTF8String]);
    }
}

char* getActiveFinderWindowPath() {
    @autoreleasepool {
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp || ![[activeApp bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
            return strdup("");
        }

        id finder = [SBApplication applicationWithBundleIdentifier:@"com.apple.finder"];
        if (!finder) {
            return strdup("");
        }

        id windows = [finder valueForKey:@"windows"];
        if (![windows isKindOfClass:[NSArray class]]) {
            return strdup("");
        }

        NSArray *windowList = (NSArray *)windows;
        if ([windowList count] == 0) {
            return strdup("");
        }

        id window = [windowList objectAtIndex:0];
        id target = nil;
        @try {
            target = [window valueForKey:@"target"];
        } @catch (NSException *exception) {
            target = nil;
        }
        if (!target) {
            return strdup("");
        }

        id urlValue = nil;
        @try {
            urlValue = [target valueForKey:@"URL"];
        } @catch (NSException *exception) {
            urlValue = nil;
        }

        NSString *path = nil;
        if ([urlValue isKindOfClass:[NSURL class]]) {
            path = [(NSURL *)urlValue path];
        } else if ([urlValue isKindOfClass:[NSString class]]) {
            NSString *stringValue = (NSString *)urlValue;
            if ([stringValue hasPrefix:@"file://"]) {
                NSURL *url = [NSURL URLWithString:stringValue];
                path = [url path];
            } else {
                path = stringValue;
            }
        }

        if (!path || [path length] == 0) {
            return strdup("");
        }
        return strdup([path UTF8String]);
    }
}

static char* copyPathFromAXValue(CFTypeRef value) {
    if (!value) {
        return strdup("");
    }

    if (CFGetTypeID(value) == CFURLGetTypeID()) {
        CFURLRef url = (CFURLRef)value;
        CFStringRef path = CFURLCopyFileSystemPath(url, kCFURLPOSIXPathStyle);
        if (path) {
            NSString *pathStr = (__bridge NSString *)path;
            char *result = strdup([pathStr UTF8String]);
            CFRelease(path);
            return result;
        }
    }

    if (CFGetTypeID(value) == CFStringGetTypeID()) {
        NSString *stringValue = (__bridge NSString *)value;
        if ([stringValue hasPrefix:@"file://"]) {
            NSURL *url = [NSURL URLWithString:stringValue];
            if (url) {
                NSString *path = [url path];
                if (path && [path length] > 0) {
                    return strdup([path UTF8String]);
                }
            }
        }
        if ([stringValue length] > 0) {
            return strdup([stringValue UTF8String]);
        }
    }

    return strdup("");
}

char* getFinderWindowPathByPid(int pid) {
    @autoreleasepool {
        if (pid <= 0) {
            return strdup("");
        }
        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        if (!app || ![[app bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
            return strdup("");
        }
        id finder = [SBApplication applicationWithBundleIdentifier:@"com.apple.finder"];
        if (!finder) {
            return strdup("");
        }

        id windows = [finder valueForKey:@"windows"];
        if (![windows isKindOfClass:[NSArray class]]) {
            return strdup("");
        }

        NSArray *windowList = (NSArray *)windows;
        NSString *focusedTitle = nil;
        if (AXIsProcessTrusted()) {
            AXUIElementRef appElement = AXUIElementCreateApplication(pid);
            if (appElement) {
                AXUIElementRef window = NULL;
                AXError windowErr = AXUIElementCopyAttributeValue(appElement, kAXFocusedWindowAttribute, (CFTypeRef *)&window);
                if (windowErr == kAXErrorSuccess && window) {
                    CFTypeRef documentValue = NULL;
                    if (AXUIElementCopyAttributeValue(window, kAXDocumentAttribute, &documentValue) == kAXErrorSuccess && documentValue) {
                        char *result = copyPathFromAXValue(documentValue);
                        CFRelease(documentValue);
                        CFRelease(window);
                        CFRelease(appElement);
                        return result;
                    }

                    CFTypeRef titleValue = NULL;
                    if (AXUIElementCopyAttributeValue(window, kAXTitleAttribute, &titleValue) == kAXErrorSuccess && titleValue) {
                        if (CFGetTypeID(titleValue) == CFStringGetTypeID()) {
                            focusedTitle = [(__bridge NSString *)titleValue copy];
                        }
                        CFRelease(titleValue);
                    }
                    CFRelease(window);
                }
                CFRelease(appElement);
            }
        }

        if (focusedTitle) {
            for (id window in windowList) {
                NSString *name = getFinderWindowNameValue(window);
                if (!name) {
                    continue;
                }
                if (![name isEqualToString:focusedTitle]) {
                    NSString *path = getFinderWindowPathValue(window);
                    if (path && [[path lastPathComponent] isEqualToString:focusedTitle]) {
                        return strdup([path UTF8String]);
                    }
                    continue;
                }
                NSString *path = getFinderWindowPathValue(window);
                if (path) {
                    return strdup([path UTF8String]);
                }
            }
        }

        if ([windowList count] == 1) {
            NSString *path = getFinderWindowPathValue([windowList objectAtIndex:0]);
            if (path) {
                return strdup([path UTF8String]);
            }
        }

        for (id window in windowList) {
            NSString *path = getFinderWindowPathValue(window);
            if (path && [path length] > 0) {
                return strdup([path UTF8String]);
            }
        }

        return strdup("");
    }
}

int selectInFinder(const char* path) {
    @autoreleasepool {
        if (path == NULL) {
            return 0;
        }
        NSString *pathStr = [NSString stringWithUTF8String:path];
        if (!pathStr || [pathStr length] == 0) {
            return 0;
        }

        // Check if the file/folder exists
        BOOL isDir = NO;
        if (![[NSFileManager defaultManager] fileExistsAtPath:pathStr isDirectory:&isDir]) {
            return 0;
        }

        // Use AppleScript to reveal the item in the frontmost Finder window
        // 'reveal' navigates to the parent folder and selects the item
        NSString *escapedPath = [pathStr stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
        NSString *script = [NSString stringWithFormat:
            @"tell application \"Finder\"\n"
            @"  activate\n"
            @"  reveal POSIX file \"%@\"\n"
            @"end tell", escapedPath];
        
        NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:script];
        NSDictionary *errorInfo = nil;
        [appleScript executeAndReturnError:&errorInfo];
        
        if (errorInfo) {
            return 0;
        }
        
        return 1;
    }
}

int navigateInFinder(const char* path) {
    @autoreleasepool {
        if (path == NULL) {
            return 0;
        }

        NSString *pathStr = [NSString stringWithUTF8String:path];
        if (!pathStr || [pathStr length] == 0) {
            return 0;
        }

        BOOL isDir = NO;
        if (![[NSFileManager defaultManager] fileExistsAtPath:pathStr isDirectory:&isDir] || !isDir) {
            return 0;
        }

        NSString *escapedPath = [pathStr stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
        NSString *script = [NSString stringWithFormat:
            @"tell application \"Finder\"\n"
            @"  activate\n"
            @"  if (count of Finder windows) > 0 then\n"
            @"    set target of front Finder window to (POSIX file \"%@\" as alias)\n"
            @"  else\n"
            @"    open POSIX file \"%@\"\n"
            @"  end if\n"
            @"end tell", escapedPath, escapedPath];

        NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:script];
        NSDictionary *errorInfo = nil;
        [appleScript executeAndReturnError:&errorInfo];
        if (errorInfo) {
            return 0;
        }

        return 1;
    }
}
