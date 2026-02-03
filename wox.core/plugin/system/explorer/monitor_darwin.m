#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

extern void fileExplorerActivatedCallbackCGO(int pid);
extern void fileExplorerDeactivatedCallbackCGO(void);

static id gAppActivationObserver = nil;
static AXObserverRef gFinderWindowObserver = nil;
static pid_t gFinderPid = 0;

// AXObserver callback - called when Finder's focused window changes
static void finderWindowFocusCallback(AXObserverRef observer, AXUIElementRef element, CFStringRef notification, void *refcon) {
    if (gFinderPid > 0) {
        fileExplorerActivatedCallbackCGO(gFinderPid);
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
        if (gAppActivationObserver) return;

        gAppActivationObserver = [[NSWorkspace sharedWorkspace].notificationCenter
            addObserverForName:NSWorkspaceDidActivateApplicationNotification
                        object:nil
                         queue:[NSOperationQueue mainQueue]
                    usingBlock:^(NSNotification *notification) {
                        NSRunningApplication *app = [[notification userInfo] objectForKey:NSWorkspaceApplicationKey];
                        if (app && [[app bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
                            pid_t pid = [app processIdentifier];
                            fileExplorerActivatedCallbackCGO(pid);
                            // Start observing window focus changes within Finder
                            startFinderWindowObserver(pid);
                        } else {
                            // Stop observing when switching away from Finder
                            stopFinderWindowObserver();
                            fileExplorerDeactivatedCallbackCGO();
                        }
                    }];
        
        // Check if Finder is already active
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (activeApp && [[activeApp bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
            pid_t pid = [activeApp processIdentifier];
            fileExplorerActivatedCallbackCGO(pid);
            startFinderWindowObserver(pid);
        }
    }
}

void stopFileExplorerMonitor() {
    @autoreleasepool {
        stopFinderWindowObserver();
        if (gAppActivationObserver) {
            [[NSWorkspace sharedWorkspace].notificationCenter removeObserver:gAppActivationObserver];
            gAppActivationObserver = nil;
        }
    }
}
