//go:build darwin

#include <stdint.h>
#import <Cocoa/Cocoa.h>

extern void dispatchMainFuncs(void);

void wakeupMainThread(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		dispatchMainFuncs();
	});
}

void os_main(void) {
	[NSApplication sharedApplication];
	[NSApp disableRelaunchOnLogin];
	[NSApp run];
}
