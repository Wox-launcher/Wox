#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

extern void finderActivatedCallbackCGO(int pid);

static id gAppActivationObserver = nil;

void startFinderMonitor() {
    @autoreleasepool {
        if (gAppActivationObserver) return;

        gAppActivationObserver = [[NSWorkspace sharedWorkspace].notificationCenter
            addObserverForName:NSWorkspaceDidActivateApplicationNotification
                        object:nil
                         queue:[NSOperationQueue mainQueue]
                    usingBlock:^(NSNotification *notification) {
                        NSRunningApplication *app = [[notification userInfo] objectForKey:NSWorkspaceApplicationKey];
                        if (app && [[app bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
                            finderActivatedCallbackCGO([app processIdentifier]);
                        }
                    }];
    }
}

void stopFinderMonitor() {
    @autoreleasepool {
        if (gAppActivationObserver) {
            [[NSWorkspace sharedWorkspace].notificationCenter removeObserver:gAppActivationObserver];
            gAppActivationObserver = nil;
        }
    }
}
