//go:build darwin

#import <Cocoa/Cocoa.h>
#include <dlfcn.h>
#include "native_darwin.h"

typedef int32_t (*WoxCursorConnectionID)(void);
typedef CGError (*WoxCopyCursorProperty)(int32_t, int32_t, CFStringRef, CFTypeRef *);
typedef CGError (*WoxSetCursorProperty)(int32_t, int32_t, CFStringRef, CFTypeRef);

// These leases belong to AppKit's main thread. Selector, editor, and recording panels overlap
// during handoff, so only the last panel may restore the connection's previous cursor policy.
static NSUInteger screenshot_cursor_users = 0;
static int32_t screenshot_cursor_connection = 0;
static CFTypeRef screenshot_previous_cursor_policy = NULL;
static CGError screenshot_cursor_status = kCGErrorSuccess;
static WoxCursorConnectionID cursor_connection_id = NULL;
static WoxCopyCursorProperty copy_cursor_property = NULL;
static WoxSetCursorProperty set_cursor_property = NULL;

// Nonactivating panels can own keyboard focus without becoming the WindowServer's foreground
// application. NSCursor.set then changes AppKit's cached cursor but leaves the system arrow visible.
// Resolve the optional WindowServer bridge dynamically so unavailable symbols do not prevent startup.
int32_t wox_darwin_acquire_screenshot_cursor(void) {
  if (screenshot_cursor_users++ > 0) {
    return screenshot_cursor_status;
  }
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    cursor_connection_id = (WoxCursorConnectionID)dlsym(RTLD_DEFAULT, "CGSMainConnectionID");
    copy_cursor_property = (WoxCopyCursorProperty)dlsym(RTLD_DEFAULT, "CGSCopyConnectionProperty");
    set_cursor_property = (WoxSetCursorProperty)dlsym(RTLD_DEFAULT, "CGSSetConnectionProperty");
  });
  if (cursor_connection_id == NULL || copy_cursor_property == NULL || set_cursor_property == NULL) {
    screenshot_cursor_status = kCGErrorNotImplemented;
    return screenshot_cursor_status;
  }
  screenshot_cursor_connection = cursor_connection_id();
  screenshot_cursor_status = copy_cursor_property(screenshot_cursor_connection, screenshot_cursor_connection,
                                                  CFSTR("SetsCursorInBackground"), &screenshot_previous_cursor_policy);
  if (screenshot_cursor_status == kCGErrorSuccess) {
    screenshot_cursor_status = set_cursor_property(screenshot_cursor_connection, screenshot_cursor_connection,
                                                   CFSTR("SetsCursorInBackground"), kCFBooleanTrue);
  }
  return screenshot_cursor_status;
}

// Restore the original process-local policy, including a preexisting enabled value.
int32_t wox_darwin_release_screenshot_cursor(void) {
  if (screenshot_cursor_users == 0 || --screenshot_cursor_users > 0) {
    return kCGErrorSuccess;
  }
  CGError result = kCGErrorSuccess;
  if (screenshot_cursor_status == kCGErrorSuccess) {
    result = set_cursor_property(screenshot_cursor_connection, screenshot_cursor_connection,
                                 CFSTR("SetsCursorInBackground"), screenshot_previous_cursor_policy ?: kCFBooleanFalse);
  }
  if (screenshot_previous_cursor_policy != NULL) {
    CFRelease(screenshot_previous_cursor_policy);
    screenshot_previous_cursor_policy = NULL;
  }
  return result;
}
