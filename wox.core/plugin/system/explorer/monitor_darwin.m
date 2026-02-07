#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <ctype.h>

extern void fileExplorerActivatedCallbackCGO(int pid, int isFileDialog, int x, int y, int w, int h);
extern void fileExplorerDeactivatedCallbackCGO(void);
extern void fileExplorerKeyDownCallbackCGO(char key);

static id gAppActivationObserver = nil;
static id gKeyDownObserver = nil;
static AXObserverRef gFinderWindowObserver = nil;
static pid_t gFinderPid = 0;

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

static void triggerFinderActivated(pid_t pid) {
    int x = 0;
    int y = 0;
    int w = 0;
    int h = 0;
    if (!getFrontmostFinderWindowRect(pid, &x, &y, &w, &h)) {
        fileExplorerDeactivatedCallbackCGO();
        return;
    }
    fileExplorerActivatedCallbackCGO(pid, 0, x, y, w, h);
}

// AXObserver callback - called when Finder's focused window changes
static void finderWindowFocusCallback(AXObserverRef observer, AXUIElementRef element, CFStringRef notification, void *refcon) {
    if (gFinderPid > 0) {
        triggerFinderActivated(gFinderPid);
    }
}

static void startFinderWindowObserver(pid_t pid) {
    // Stop existing observer if any
    if (gFinderWindowObserver) {
        CFRunLoopRemoveSource(CFRunLoopGetMain(), 
                              AXObserverGetRunLoopSource(gFinderWindowObserver), 
                              kCFRunLoopDefaultMode);
        CFRelease(gFinderWindowObserver);
        gFinderWindowObserver = nil;
    }
    
    gFinderPid = pid;
    
    // Create AXObserver for window focus changes
    AXObserverRef observer = NULL;
    AXError err = AXObserverCreate(pid, finderWindowFocusCallback, &observer);
    if (err != kAXErrorSuccess || !observer) {
        return;
    }
    
    gFinderWindowObserver = observer;
    
    // Get the application element and add notification
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (app) {
        AXObserverAddNotification(observer, app, kAXFocusedWindowChangedNotification, NULL);
        CFRelease(app);
    }
    
    // Add observer to run loop
    CFRunLoopAddSource(CFRunLoopGetMain(), AXObserverGetRunLoopSource(observer), kCFRunLoopDefaultMode);
}

static void stopFinderWindowObserver() {
    if (gFinderWindowObserver) {
        CFRunLoopRemoveSource(CFRunLoopGetMain(), 
                              AXObserverGetRunLoopSource(gFinderWindowObserver), 
                              kCFRunLoopDefaultMode);
        CFRelease(gFinderWindowObserver);
        gFinderWindowObserver = nil;
    }
    gFinderPid = 0;
}

void startFileExplorerMonitor() {
    @autoreleasepool {
        if (!gAppActivationObserver) {
            gAppActivationObserver = [[NSWorkspace sharedWorkspace].notificationCenter
                addObserverForName:NSWorkspaceDidActivateApplicationNotification
                            object:nil
                             queue:[NSOperationQueue mainQueue]
                        usingBlock:^(NSNotification *notification) {
                            NSRunningApplication *app = [[notification userInfo] objectForKey:NSWorkspaceApplicationKey];
                            if (app && [[app bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
                                pid_t pid = [app processIdentifier];
                                triggerFinderActivated(pid);
                                // Start observing window focus changes within Finder
                                startFinderWindowObserver(pid);
                            } else {
                                // Stop observing when switching away from Finder
                                stopFinderWindowObserver();
                                fileExplorerDeactivatedCallbackCGO();
                            }
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

                int x = 0;
                int y = 0;
                int w = 0;
                int h = 0;
                if (gFinderPid <= 0 || !getFrontmostFinderWindowRect(gFinderPid, &x, &y, &w, &h)) {
                    return;
                }

                fileExplorerKeyDownCallbackCGO((char)ch);
            }];
        }
        
        // Check if Finder is already active
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (activeApp && [[activeApp bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
            pid_t pid = [activeApp processIdentifier];
            triggerFinderActivated(pid);
            startFinderWindowObserver(pid);
        }
    }
}

void stopFileExplorerMonitor() {
    @autoreleasepool {
        stopFinderWindowObserver();
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
