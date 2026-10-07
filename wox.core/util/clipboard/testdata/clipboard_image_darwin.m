#import "../clipboard_image_darwin.h"
#import <Cocoa/Cocoa.h>
#include <math.h>
#include <stdio.h>
#include <string.h>

// Report integration failures from the standalone test process.
static int fail(const char *message) {
  fprintf(stderr, "%s\n", message);
  return 1;
}

// Validate both canonical bytes and translucent edge alpha without relying on a particular TIFF encoding.
static BOOL clipboard_matches(NSData *png) {
  NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
  if (![[pasteboard dataForType:NSPasteboardTypePNG] isEqualToData:png]) return NO;
  NSData *tiff = [pasteboard dataForType:NSPasteboardTypeTIFF];
  NSBitmapImageRep *image = [NSBitmapImageRep imageRepWithData:tiff];
  if (image == nil || image.pixelsWide != 2 || image.pixelsHigh != 1) return NO;
  return [image colorAtX:0 y:0].alphaComponent == 0 &&
         fabs([image colorAtX:1 y:0].alphaComponent - 128.0 / 255.0) < 0.001;
}

// Read after the publishing process exits to ensure shutdown materialized its promised TIFF representation.
int main(int argc, const char *argv[]) {
  @autoreleasepool {
    if (argc != 3) return fail("expected mode and PNG path");
    [NSApplication sharedApplication];
    NSData *png = [NSData dataWithContentsOfFile:[NSString stringWithUTF8String:argv[2]]];
    if (png == nil) return fail("cannot read PNG fixture");
    if (strcmp(argv[1], "read") == 0) {
      return clipboard_matches(png) ? 0 : fail("PNG or TIFF is unavailable");
    }
    if (wox_clipboard_darwin_write_png(png.bytes, png.length) != 0) {
      return fail("cannot publish PNG");
    }
    // Let another process request TIFF while the provider services AppKit callbacks.
    NSTask *reader = [[[NSTask alloc] init] autorelease];
    reader.executableURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:argv[0]]];
    reader.arguments = @[@"read", [NSString stringWithUTF8String:argv[2]]];
    NSError *error = nil;
    if (![reader launchAndReturnError:&error]) return fail("cannot launch clipboard reader");
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:5];
    while (reader.running && deadline.timeIntervalSinceNow > 0) {
      [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.01]];
    }
    if (reader.running) {
      [reader terminate];
      return fail("external TIFF request timed out");
    }
    if (reader.terminationStatus != 0 || !clipboard_matches(png)) {
      return fail("external PNG or on-demand TIFF changed pixels");
    }
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    [pasteboard setString:@"new clipboard owner" forType:NSPasteboardTypeString];
    if (wox_clipboard_darwin_flush() != 0 ||
        ![[pasteboard stringForType:NSPasteboardTypeString] isEqualToString:@"new clipboard owner"]) {
      return fail("flush changed a newer clipboard write");
    }
    if (wox_clipboard_darwin_write_png(png.bytes, png.length) != 0 || wox_clipboard_darwin_flush() != 0) {
      return fail("cannot materialize TIFF before exit");
    }
    return 0;
  }
}
