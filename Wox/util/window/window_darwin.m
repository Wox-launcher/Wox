#include <Cocoa/Cocoa.h>

int getActiveWindowIcon(unsigned char **iconData) {
    @autoreleasepool {
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!activeApp) {
            return 0;
        }

        NSImage *icon = [activeApp icon];
        if (!icon) {
            return 0;
        }

        CGImageRef cgRef = [icon CGImageForProposedRect:NULL context:nil hints:nil];
        NSBitmapImageRep *newRep = [[NSBitmapImageRep alloc] initWithCGImage:cgRef];
        [newRep setSize:[icon size]];
        NSData *pngData = [newRep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        if (!pngData) {
            return 0;
        }

        NSUInteger length = [pngData length];
        void *buffer = malloc(length);
        if (!buffer) {
            return 0;
        }
        memcpy(buffer, [pngData bytes], length);

        *iconData = buffer;
        return (int)length;
    }
}
