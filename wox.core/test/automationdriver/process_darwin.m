//go:build darwin

#import <Cocoa/Cocoa.h>

int woxAutomationTerminateApplication(int pid) {
    @autoreleasepool {
        NSRunningApplication *application = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        return application != nil && [application terminate] ? 1 : 0;
    }
}
