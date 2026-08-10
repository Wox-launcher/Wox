//go:build darwin

#import <Cocoa/Cocoa.h>
#include <dispatch/dispatch.h>
#include <math.h>
#include <stdint.h>

// Capture the pointer in the top-left desktop coordinate space used by the screenshot editor.
int32_t wox_screenshot_cursor_position(float *x, float *y) {
  if (x == NULL || y == NULL) {
    return -1;
  }
  @autoreleasepool {
    __block CGFloat desktop_top = 0.0;
    __block NSPoint location = NSZeroPoint;
    void (^capture_position)(void) = ^{
      for (NSScreen *screen in [NSScreen screens]) {
        desktop_top = fmax(desktop_top, NSMaxY(screen.frame));
      }
      location = [NSEvent mouseLocation];
    };
    if ([NSThread isMainThread]) {
      capture_position();
    } else {
      dispatch_sync(dispatch_get_main_queue(), capture_position);
    }
    *x = (float)location.x;
    *y = (float)(desktop_top - location.y);
    return 0;
  }
}
