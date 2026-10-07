#import <Cocoa/Cocoa.h>
#include <dlfcn.h>
#include "../native_darwin.h"

typedef int32_t (*ConnectionID)(void);
typedef CGError (*CopyProperty)(int32_t, int32_t, CFStringRef, CFTypeRef *);
typedef CGError (*SetProperty)(int32_t, int32_t, CFStringRef, CFTypeRef);

// Normalize NSImage representations so the comparison includes the visible system cursor pixels.
static NSData *cursor_png(NSCursor *cursor) {
  NSImage *image = cursor.image;
  NSRect rect = NSMakeRect(0, 0, image.size.width, image.size.height);
  CGImageRef cg_image = [image CGImageForProposedRect:&rect context:nil hints:nil];
  if (cg_image == NULL) return nil;
  NSBitmapImageRep *bitmap = [[[NSBitmapImageRep alloc] initWithCGImage:cg_image] autorelease];
  return [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
}

// AppKit caches the requested cursor even when WindowServer refuses it; inspect the system image instead.
static BOOL system_cursor_matches(NSCursor *expected) {
  [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.03]];
  NSCursor *actual = [NSCursor currentSystemCursor];
  return NSEqualPoints(actual.hotSpot, expected.hotSpot) && [cursor_png(actual) isEqualToData:cursor_png(expected)];
}

// Report failures only from this test process, without adding native logging to the application.
static int fail(NSString *message) {
  NSData *data = [[message stringByAppendingString:@"\n"] dataUsingEncoding:NSUTF8StringEncoding];
  [[NSFileHandle fileHandleWithStandardError] writeData:data];
  return 1;
}

// A second process replaces the visible cursor while this process still caches the hand shape.
static BOOL cursor_recovers_after_external_reset(const char *executable) {
  NSTask *reset = [[[NSTask alloc] init] autorelease];
  NSPipe *ready = [NSPipe pipe];
  NSPipe *stop = [NSPipe pipe];
  reset.executableURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:executable]];
  reset.arguments = @[@"--reset-cursor"];
  reset.standardOutput = ready;
  reset.standardInput = stop;
  if (![reset launchAndReturnError:nil]) return NO;
  BOOL matches = NO;
  @try {
    if ([[ready fileHandleForReading] readDataOfLength:1].length == 1 && system_cursor_matches([NSCursor arrowCursor])) {
      BOOL cached_hand = [cursor_png([NSCursor currentCursor]) isEqualToData:cursor_png([NSCursor openHandCursor])];
      [[NSCursor openHandCursor] set];
      matches = cached_hand && system_cursor_matches([NSCursor openHandCursor]);
    }
  } @finally {
    [[stop fileHandleForWriting] closeFile];
    [reset waitUntilExit];
  }
  return matches && reset.terminationStatus == 0;
}

// Exercise the production lease implementation without activating an app or opening a window.
int main(int argc, const char *argv[]) {
  @autoreleasepool {
    [NSApplication sharedApplication];
    if (argc == 2 && strcmp(argv[1], "--reset-cursor") == 0) {
      if (wox_darwin_acquire_overlay_cursor() != 0) return fail(@"external cursor reset lease failed");
      [[NSCursor arrowCursor] set];
      [[NSFileHandle fileHandleWithStandardOutput] writeData:[@"1" dataUsingEncoding:NSUTF8StringEncoding]];
      [[NSFileHandle fileHandleWithStandardInput] readDataToEndOfFile];
      wox_darwin_release_overlay_cursor();
      return 0;
    }
    ConnectionID connection_id = (ConnectionID)dlsym(RTLD_DEFAULT, "CGSMainConnectionID");
    CopyProperty copy_property = (CopyProperty)dlsym(RTLD_DEFAULT, "CGSCopyConnectionProperty");
    SetProperty set_property = (SetProperty)dlsym(RTLD_DEFAULT, "CGSSetConnectionProperty");
    if (connection_id == NULL || copy_property == NULL || set_property == NULL) return fail(@"cursor bridge unavailable");
    int32_t connection = connection_id();
    NSCursor *original = [[NSCursor currentSystemCursor] retain];
    int result = 0;
    int leases = 0;
    @try {
      for (NSNumber *initial in @[@NO, @YES]) {
        if (set_property(connection, connection, CFSTR("SetsCursorInBackground"), (CFTypeRef)initial) != 0) {
          result = fail(@"cannot initialize test cursor policy");
          break;
        }
        // Selector and recording border coexist during handoff.
        leases++;
        if (wox_darwin_acquire_overlay_cursor() != 0) { result = fail(@"first panel lease failed"); break; }
        leases++;
        if (wox_darwin_acquire_overlay_cursor() != 0) { result = fail(@"overlapping panel lease failed"); break; }
        for (NSCursor *cursor in @[[NSCursor resizeLeftRightCursor], [NSCursor resizeUpDownCursor], [NSCursor openHandCursor]]) {
          [cursor set];
          if (!system_cursor_matches(cursor)) { result = fail(@"system cursor pixels do not match requested resize/move cursor"); break; }
        }
        if (result != 0) break;
        if (!cursor_recovers_after_external_reset(argv[0])) {
          result = fail(@"reapplying the same hand cursor did not recover from another process's reset");
          break;
        }
        wox_darwin_release_overlay_cursor();
        leases--;
        [[NSCursor crosshairCursor] set];
        if (!system_cursor_matches([NSCursor crosshairCursor])) { result = fail(@"closing selector disabled the remaining panel cursor"); break; }
        [original set];
        if (wox_darwin_release_overlay_cursor() != 0) { result = fail(@"last panel policy restoration failed"); leases--; break; }
        leases--;
        CFTypeRef restored = NULL;
        CGError status = copy_property(connection, connection, CFSTR("SetsCursorInBackground"), &restored);
        BOOL matches = status == 0 && restored != NULL && CFEqual(restored, (CFTypeRef)initial);
        if (restored != NULL) CFRelease(restored);
        if (!matches) { result = fail(@"last panel did not restore the previous cursor policy"); break; }
      }
    } @finally {
      // Restore the real pointer even when a pixel comparison fails.
      set_property(connection, connection, CFSTR("SetsCursorInBackground"), kCFBooleanTrue);
      [original set];
      while (leases-- > 0) wox_darwin_release_overlay_cursor();
      [original release];
    }
    return result;
  }
}
