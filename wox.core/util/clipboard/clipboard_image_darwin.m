//go:build darwin

#import "clipboard_image_darwin.h"
#import <Cocoa/Cocoa.h>
#import <ImageIO/ImageIO.h>
#import <dispatch/dispatch.h>

// The pasteboard server owns the PNG. The provider retains only a revision, never a full-size image.
@interface WoxClipboardImageOwner : NSObject {
@public
  NSInteger revision;
}
@end

static WoxClipboardImageOwner *clipboard_image_owner = nil;

// Materialize TIFF only for consumers that cannot read PNG, while respecting subsequent clipboard writes.
static BOOL provide_clipboard_tiff(NSPasteboard *pasteboard) {
  NSData *png = [pasteboard dataForType:NSPasteboardTypePNG];
  if (png == nil) {
    return NO;
  }
  CGImageSourceRef source = CGImageSourceCreateWithData((CFDataRef)png, NULL);
  if (source == NULL) {
    return NO;
  }
  CGImageRef image = CGImageSourceCreateImageAtIndex(source, 0, NULL);
  CFRelease(source);
  if (image == NULL) {
    return NO;
  }
  NSMutableData *tiff = [NSMutableData data];
  CGImageDestinationRef destination = CGImageDestinationCreateWithData((CFMutableDataRef)tiff, CFSTR("public.tiff"), 1, NULL);
  BOOL result = NO;
  if (destination != NULL) {
    CGImageDestinationAddImage(destination, image, NULL);
    if (CGImageDestinationFinalize(destination) && pasteboard.changeCount == clipboard_image_owner->revision) {
      result = [pasteboard setData:tiff forType:NSPasteboardTypeTIFF];
    }
    CFRelease(destination);
  }
  CGImageRelease(image);
  return result;
}

@implementation WoxClipboardImageOwner
- (void)pasteboard:(NSPasteboard *)pasteboard provideDataForType:(NSPasteboardType)type {
  @autoreleasepool {
    if ([type isEqualToString:NSPasteboardTypeTIFF] && pasteboard.changeCount == revision) {
      provide_clipboard_tiff(pasteboard);
    }
  }
}
@end

// Serialize publication and promised-format callbacks on AppKit's owning thread.
static void clipboard_on_main_sync(dispatch_block_t block) {
  if ([NSThread isMainThread]) {
    block();
  } else {
    dispatch_sync(dispatch_get_main_queue(), block);
  }
}

// Publish encoded bytes without normalizing, copying, or retaining an uncompressed raster.
int32_t wox_clipboard_darwin_write_png(const uint8_t *png, size_t length) {
  if (png == NULL || length == 0) {
    return -1;
  }
  __block int32_t result = -1;
  clipboard_on_main_sync(^{
    @autoreleasepool {
      if (clipboard_image_owner == nil) {
        clipboard_image_owner = [[WoxClipboardImageOwner alloc] init];
      }
      NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
      clipboard_image_owner->revision = [pasteboard declareTypes:@[NSPasteboardTypePNG, NSPasteboardTypeTIFF] owner:clipboard_image_owner];
      NSData *data = [NSData dataWithBytes:png length:length];
      if ([pasteboard setData:data forType:NSPasteboardTypePNG]) {
        result = 0;
      }
    }
  });
  return result;
}

// Complete outstanding promises before os.Exit, so TIFF remains pasteable without the provider process.
int32_t wox_clipboard_darwin_flush(void) {
  __block int32_t result = 0;
  clipboard_on_main_sync(^{
    @autoreleasepool {
      NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
      if (clipboard_image_owner != nil && pasteboard.changeCount == clipboard_image_owner->revision &&
          [pasteboard dataForType:NSPasteboardTypeTIFF] == nil) {
        result = -1;
      }
    }
  });
  return result;
}

int32_t wox_clipboard_darwin_write_text(const char *text) {
  if (text == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  clipboard_on_main_sync(^{
    NSString *value = [NSString stringWithUTF8String:text];
    if (value == nil) {
      result = -1;
      return;
    }
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    if (![pasteboard setString:value forType:NSPasteboardTypeString]) {
      result = -1;
    }
  });
  return result;
}

int32_t wox_clipboard_darwin_write_pixels(const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride) {
  if (pixels == NULL || width <= 0 || height <= 0 || row_stride < width * 4) {
    return -1;
  }
  __block int32_t result = 0;
  clipboard_on_main_sync(^{
    NSBitmapImageRep *representation = [[NSBitmapImageRep alloc]
        initWithBitmapDataPlanes:NULL
                  pixelsWide:width
                  pixelsHigh:height
               bitsPerSample:8
             samplesPerPixel:4
                    hasAlpha:YES
                    isPlanar:NO
              colorSpaceName:NSCalibratedRGBColorSpace
                 bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
                  bytesPerRow:row_stride
                 bitsPerPixel:32];
    if (representation == nil || representation.bitmapData == NULL) {
      [representation release];
      result = -1;
      return;
    }
    memcpy(representation.bitmapData, pixels, (size_t)row_stride * (size_t)height);
    NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(width, height)];
    [image addRepresentation:representation];
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    if (![pasteboard writeObjects:@[ image ]]) {
      result = -1;
    }
    [image release];
    [representation release];
  });
  return result;
}

