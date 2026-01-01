#include <ApplicationServices/ApplicationServices.h>
#include <Cocoa/Cocoa.h>
#include <unistd.h>

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

int getActiveWindowPid() {
    @autoreleasepool {
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return -1;
        }

        return [activeApp processIdentifier];
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

        AXUIElementRef window = NULL;
        AXError windowErr = AXUIElementCopyAttributeValue(appElement, kAXFocusedWindowAttribute, (CFTypeRef *)&window);
        if (windowErr != kAXErrorSuccess || !window) {
            CFRelease(appElement);
            return 0;
        }

        BOOL isDialog = NO;
        CFTypeRef role = NULL;
        if (AXUIElementCopyAttributeValue(window, kAXRoleAttribute, &role) == kAXErrorSuccess && role) {
            if (CFGetTypeID(role) == CFStringGetTypeID()) {
                if (CFStringCompare(role, CFSTR("AXSheet"), 0) == kCFCompareEqualTo) {
                    isDialog = YES;
                }
            }
            CFRelease(role);
        }

        if (!isDialog) {
            CFTypeRef subrole = NULL;
            if (AXUIElementCopyAttributeValue(window, kAXSubroleAttribute, &subrole) == kAXErrorSuccess && subrole) {
                if (CFGetTypeID(subrole) == CFStringGetTypeID()) {
                    if (CFStringCompare(subrole, CFSTR("AXDialog"), 0) == kCFCompareEqualTo ||
                        CFStringCompare(subrole, CFSTR("AXSystemDialog"), 0) == kCFCompareEqualTo ||
                        CFStringCompare(subrole, CFSTR("AXSheet"), 0) == kCFCompareEqualTo) {
                        isDialog = YES;
                    }
                }
                CFRelease(subrole);
            }
        }

        CFRelease(window);
        CFRelease(appElement);
        return isDialog ? 1 : 0;
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
