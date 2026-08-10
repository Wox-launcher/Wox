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

// Export the currently visible system cursor while preserving its pixel hotspot.
int32_t wox_screenshot_cursor_png(const char *path, float *hotspot_x, float *hotspot_y) {
  if (path == NULL || hotspot_x == NULL || hotspot_y == NULL) {
    return -1;
  }
  @autoreleasepool {
    __block int32_t result = -1;
    void (^capture_cursor)(void) = ^{
      NSCursor *cursor = [NSCursor currentSystemCursor];
      NSImage *cursor_image = cursor.image;
      if (cursor_image == nil || cursor_image.size.width <= 0 || cursor_image.size.height <= 0) {
        return;
      }
      NSRect proposed_rect = NSMakeRect(0, 0, cursor_image.size.width, cursor_image.size.height);
      CGImageRef cg_image = [cursor_image CGImageForProposedRect:&proposed_rect context:nil hints:nil];
      if (cg_image == NULL) {
        return;
      }
      NSBitmapImageRep *representation = [[NSBitmapImageRep alloc] initWithCGImage:cg_image];
      NSData *png = [representation representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
      if (png == nil || ![png writeToFile:[NSString stringWithUTF8String:path] atomically:YES]) {
        [representation release];
        return;
      }
      CGFloat scale_x = representation.pixelsWide / cursor_image.size.width;
      CGFloat scale_y = representation.pixelsHigh / cursor_image.size.height;
      *hotspot_x = (float)(cursor.hotSpot.x * scale_x);
      *hotspot_y = (float)(cursor.hotSpot.y * scale_y);
      [representation release];
      result = 0;
    };
    if ([NSThread isMainThread]) {
      capture_cursor();
    } else {
      dispatch_sync(dispatch_get_main_queue(), capture_cursor);
    }
    return result;
  }
}
