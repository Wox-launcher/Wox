#include <ApplicationServices/ApplicationServices.h>
#include <Cocoa/Cocoa.h>

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
