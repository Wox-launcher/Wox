#import <AppKit/AppKit.h>

extern void woxPowerDidWake(void);

void *woxPowerWatch(void) {
    @autoreleasepool {
        id observer = [[[NSWorkspace sharedWorkspace] notificationCenter]
            addObserverForName:NSWorkspaceDidWakeNotification object:nil queue:nil
            usingBlock:^(NSNotification *notification) { woxPowerDidWake(); }];
        return [observer retain];
    }
}

void woxPowerUnwatch(void *observer) {
    @autoreleasepool {
        [[[NSWorkspace sharedWorkspace] notificationCenter] removeObserver:(id)observer];
        [(id)observer release];
    }
}
