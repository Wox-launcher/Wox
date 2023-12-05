#import <Cocoa/Cocoa.h>
#import <Foundation/Foundation.h>

const unsigned char *GetPrefPaneIcon(const char *prefPanePath, size_t *length) {
    @autoreleasepool {
        NSString *path = [NSString stringWithUTF8String:prefPanePath];
        NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:path];

        if (icon == nil) {
            return NULL;
        }

        NSData *tiffData = [icon TIFFRepresentation];
        NSBitmapImageRep *imageRep = [NSBitmapImageRep imageRepWithData:tiffData];
        NSDictionary *imageProps = [NSDictionary dictionaryWithObject:[NSNumber numberWithFloat:1.0] forKey:NSImageCompressionFactor];
        NSData *pngData = [imageRep representationUsingType:NSBitmapImageFileTypePNG properties:imageProps];

        *length = [pngData length];
        unsigned char *bytes = (unsigned char *)malloc(*length);
        memcpy(bytes, [pngData bytes], *length);

        return bytes;
    }
}