//go:build darwin

#import <Cocoa/Cocoa.h>
#include <dlfcn.h>
#include "native_darwin.h"

typedef int32_t (*WoxCursorConnectionID)(void);
typedef CGError (*WoxCopyCursorProperty)(int32_t, int32_t, CFStringRef, CFTypeRef *);
typedef CGError (*WoxSetCursorProperty)(int32_t, int32_t, CFStringRef, CFTypeRef);

// These leases belong to AppKit's main thread. Selector, editor, and recording panels overlap
// during handoff, so only the last panel may restore the connection's previous cursor policy.
static NSUInteger overlay_cursor_users = 0;
static int32_t overlay_cursor_connection = 0;
static CFTypeRef overlay_previous_cursor_policy = NULL;
static CGError overlay_cursor_status = kCGErrorSuccess;
static WoxCursorConnectionID cursor_connection_id = NULL;
static WoxCopyCursorProperty copy_cursor_property = NULL;
static WoxSetCursorProperty set_cursor_property = NULL;

// Nonactivating panels can own keyboard focus without becoming the WindowServer's foreground
// application. NSCursor.set then changes AppKit's cached cursor but leaves the system arrow visible.
// Resolve the optional WindowServer bridge dynamically so unavailable symbols do not prevent startup.
int32_t wox_darwin_acquire_overlay_cursor(void) {
  if (overlay_cursor_users++ > 0) {
    return overlay_cursor_status;
  }
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    cursor_connection_id = (WoxCursorConnectionID)dlsym(RTLD_DEFAULT, "CGSMainConnectionID");
    copy_cursor_property = (WoxCopyCursorProperty)dlsym(RTLD_DEFAULT, "CGSCopyConnectionProperty");
    set_cursor_property = (WoxSetCursorProperty)dlsym(RTLD_DEFAULT, "CGSSetConnectionProperty");
  });
  if (cursor_connection_id == NULL || copy_cursor_property == NULL || set_cursor_property == NULL) {
    overlay_cursor_status = kCGErrorNotImplemented;
    return overlay_cursor_status;
  }
  overlay_cursor_connection = cursor_connection_id();
  overlay_cursor_status = copy_cursor_property(overlay_cursor_connection, overlay_cursor_connection,
                                               CFSTR("SetsCursorInBackground"), &overlay_previous_cursor_policy);
  if (overlay_cursor_status == kCGErrorSuccess) {
    overlay_cursor_status = set_cursor_property(overlay_cursor_connection, overlay_cursor_connection,
                                                CFSTR("SetsCursorInBackground"), kCFBooleanTrue);
  }
  return overlay_cursor_status;
}

// Restore the original process-local policy, including a preexisting enabled value.
int32_t wox_darwin_release_overlay_cursor(void) {
  if (overlay_cursor_users == 0 || --overlay_cursor_users > 0) {
    return kCGErrorSuccess;
  }
  CGError result = kCGErrorSuccess;
  if (overlay_cursor_status == kCGErrorSuccess) {
    result = set_cursor_property(overlay_cursor_connection, overlay_cursor_connection,
                                 CFSTR("SetsCursorInBackground"), overlay_previous_cursor_policy ?: kCFBooleanFalse);
  }
  if (overlay_previous_cursor_policy != NULL) {
    CFRelease(overlay_previous_cursor_policy);
    overlay_previous_cursor_policy = NULL;
  }
  return result;
}
