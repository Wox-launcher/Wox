#import <Cocoa/Cocoa.h>
#if __has_include(<UniformTypeIdentifiers/UniformTypeIdentifiers.h>)
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#endif

static NSImage *GetWorkspaceIconForExtension(NSString *extension) {
    NSWorkspace *workspace = [NSWorkspace sharedWorkspace];

    if (@available(macOS 11.0, *)) {
#if __has_include(<UniformTypeIdentifiers/UniformTypeIdentifiers.h>)
        if ([extension length] > 0) {
            UTType *contentType = [UTType typeWithFilenameExtension:extension];
            if (contentType != nil) {
                return [workspace iconForContentType:contentType];
            }
        }

        return [workspace iconForContentType:UTTypeData];
#endif
    }

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    return [workspace iconForFileType:extension];
#pragma clang diagnostic pop
}

static const unsigned char *RenderIconAsPNG(NSImage *icon, int targetPixels, size_t *length) {
    if (!icon) return NULL;
    if (targetPixels <= 0) return NULL;

    // Render into an explicit pixel-sized bitmap. CGImageForProposedRect can
    // hand back the small representation, which looks blurry when Wox shows
    // app icons in a grid result surface.
    NSBitmapImageRep *rep = [[NSBitmapImageRep alloc]
        initWithBitmapDataPlanes:NULL
                      pixelsWide:targetPixels
                      pixelsHigh:targetPixels
                   bitsPerSample:8
                 samplesPerPixel:4
                        hasAlpha:YES
                        isPlanar:NO
                  colorSpaceName:NSDeviceRGBColorSpace
                     bytesPerRow:0
                    bitsPerPixel:0];
    if (!rep) return NULL;

    [rep setSize:NSMakeSize(targetPixels, targetPixels)];
    NSGraphicsContext *context = [NSGraphicsContext graphicsContextWithBitmapImageRep:rep];
    if (!context) {
        [rep release];
        return NULL;
    }

    [NSGraphicsContext saveGraphicsState];
    [NSGraphicsContext setCurrentContext:context];
    [[NSGraphicsContext currentContext] setImageInterpolation:NSImageInterpolationHigh];
    [[NSColor clearColor] setFill];
    NSRectFill(NSMakeRect(0, 0, targetPixels, targetPixels));
    [icon drawInRect:NSMakeRect(0, 0, targetPixels, targetPixels)
            fromRect:NSZeroRect
           operation:NSCompositingOperationSourceOver
            fraction:1.0];
    [NSGraphicsContext restoreGraphicsState];

    NSData *pngData = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
    if (!pngData) {
        [rep release];
        return NULL;
    }

    *length = [pngData length];
    unsigned char *bytes = (unsigned char *)malloc(*length);
    memcpy(bytes, [pngData bytes], *length);
    // Bug fix: the Objective-C helper is not built with ARC. Release the
    // explicit bitmap rep after copying PNG bytes so icon extraction does not
    // retain native CG image memory beyond the cache write.
    [rep release];
    return bytes;
}

const unsigned char *GetFileIconBytes(const char *pathC, int size, size_t *length) {
    @autoreleasepool {
        if (pathC == NULL) return NULL;
        NSString *path = [NSString stringWithUTF8String:pathC];
        if ([path length] == 0) return NULL;
        NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:path];
        if (!icon) return NULL;

        return RenderIconAsPNG(icon, size, length);
    }
}

const unsigned char *GetFileTypeIconBytes(const char *extC, int size, size_t *length) {
    @autoreleasepool {
        if (extC == NULL) return NULL;
        NSString *ext = [NSString stringWithUTF8String:extC];
        if ([ext hasPrefix:@"."]) {
            ext = [ext substringFromIndex:1];
        }
        NSImage *icon = GetWorkspaceIconForExtension(ext);
        if (!icon) return NULL;

        return RenderIconAsPNG(icon, size, length);
    }
}
