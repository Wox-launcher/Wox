#import <Cocoa/Cocoa.h>

const unsigned char *GetFileTypeIconBytes(const char *extC, size_t *length) {
    @autoreleasepool {
        if (extC == NULL) return NULL;
        NSString *ext = [NSString stringWithUTF8String:extC];
        if ([ext hasPrefix:@"."]) {
            ext = [ext substringFromIndex:1];
        }
        NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFileType:ext];
        if (!icon) return NULL;

        CGImageRef cgRef = [icon CGImageForProposedRect:NULL context:nil hints:nil];
        if (!cgRef) return NULL;
        NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithCGImage:cgRef];
        [rep setSize:[icon size]];
        NSData *pngData = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        if (!pngData) return NULL;

        *length = [pngData length];
        unsigned char *bytes = (unsigned char *)malloc(*length);
        memcpy(bytes, [pngData bytes], *length);
        return bytes;
    }
}

