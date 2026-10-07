//go:build darwin

#import "browser_external_darwin.h"
#import <Cocoa/Cocoa.h>

// The caller dispatches to AppKit's owning thread; no Wox window type crosses this bridge.
int32_t wox_browser_open_external_url(const char *url) {
  if (url == NULL) {
    return -1;
  }
  @autoreleasepool {
    NSString *value = [NSString stringWithUTF8String:url];
    NSURL *target = value != nil ? [NSURL URLWithString:value] : nil;
    return target != nil && [[NSWorkspace sharedWorkspace] openURL:target] ? 0 : -1;
  }
}
