#import <Cocoa/Cocoa.h>

typedef struct {
  int width;
  int height;
  int x;
  int y;
} ScreenInfo;

ScreenInfo getMouseScreenSize() {
  NSPoint mouseLoc = [NSEvent mouseLocation];
  NSArray *screens = [NSScreen screens];

  for (NSScreen *screen in screens) {
    NSRect frame = [screen frame];
    if (NSMouseInRect(mouseLoc, frame, NO)) {
      // IMPORTANT: Use visibleFrame instead of frame to exclude menu bar and
      // dock areas This ensures window positioning calculations use only the
      // available workspace area
      NSRect visibleFrame = [screen visibleFrame];

      // Convert from AppKit's bottom-left origin to top-left origin coordinate
      // system
      //
      // Why this conversion is needed:
      // - AppKit uses bottom-left origin with Y-axis pointing up
      // - Go backend expects top-left origin with Y-axis pointing down
      // (standard for most UI frameworks)
      // - We need to return the Y offset from the physical screen top to the
      // visible area top
      //
      // Calculation:
      // - frame.size.height = total screen height (e.g., 1080 pixels)
      // - visibleFrame.size.height = available height excluding menu bar (e.g.,
      // 1055 pixels)
      // - topY = frame.size.height - visibleFrame.size.height = menu bar height
      // (e.g., 25 pixels)
      //
      // This topY value tells Go backend: "the usable area starts 25 pixels
      // from the screen top"
      int topY = frame.size.height - visibleFrame.size.height;

      return (ScreenInfo){.width = visibleFrame.size.width,
                          .height = visibleFrame.size.height,
                          .x = visibleFrame.origin.x,
                          .y = topY};
    }
  }
  return (ScreenInfo){.width = 0, .height = 0, .x = 0, .y = 0};
}

ScreenInfo getPrimaryScreenSize() {
  NSScreen *primaryScreen = [NSScreen mainScreen];
  NSRect frame = [primaryScreen frame];
  // Use visibleFrame to exclude menu bar and dock areas
  NSRect visibleFrame = [primaryScreen visibleFrame];
  // Convert from AppKit's bottom-left origin to top-left origin
  // topY = distance from physical screen top to visible area top (menu bar
  // height)
  int topY = frame.size.height - visibleFrame.size.height;
  return (ScreenInfo){.width = visibleFrame.size.width,
                      .height = visibleFrame.size.height,
                      .x = visibleFrame.origin.x,
                      .y = topY};
}

ScreenInfo getActiveScreenSize() {
  NSWindow *keyWindow = [[NSApplication sharedApplication] keyWindow];
  if (!keyWindow) {
    // Fallback to primary screen if no active window
    return getPrimaryScreenSize();
  }

  NSScreen *activeScreen = [keyWindow screen];
  if (!activeScreen) {
    // Fallback to primary screen if window's screen not found
    return getPrimaryScreenSize();
  }

  NSRect frame = [activeScreen frame];
  // Use visibleFrame to exclude menu bar and dock areas
  NSRect visibleFrame = [activeScreen visibleFrame];
  // Convert from AppKit's bottom-left origin to top-left origin
  // topY = distance from physical screen top to visible area top (menu bar
  // height)
  int topY = frame.size.height - visibleFrame.size.height;
  return (ScreenInfo){.width = visibleFrame.size.width,
                      .height = visibleFrame.size.height,
                      .x = visibleFrame.origin.x,
                      .y = topY};
}
