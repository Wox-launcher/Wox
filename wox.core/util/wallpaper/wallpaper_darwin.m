#import <Cocoa/Cocoa.h>
#include <string.h>

char *woxGetSystemWallpaperPath(void) {
  __block char *result = NULL;
  void (^resolve)(void) = ^{
    NSScreen *screen = [NSScreen mainScreen];
    if (screen == nil) {
      return;
    }
    NSURL *url = [[NSWorkspace sharedWorkspace] desktopImageURLForScreen:screen];
    NSString *path = url.path;
    if (path == nil || path.length == 0) {
      return;
    }
    result = strdup(path.fileSystemRepresentation);
  };
  if ([NSThread isMainThread]) {
    resolve();
  } else {
    dispatch_sync(dispatch_get_main_queue(), resolve);
  }
  return result;
}
