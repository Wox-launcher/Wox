package window


#import <Cocoa/Cocoa.h>

int getActiveWindowIcon(unsigned char **iconData) {
    NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
    NSImage *icon = [activeApp icon];
    NSData *tiffData = [icon TIFFRepresentation];
    NSData *pngData = [NSBitmapImageRep imageRepWithData:tiffData].representationUsingType:NSPNGFileType properties:@{}];

    *iconData = (unsigned char *)[pngData bytes];
    return (int)[pngData length];
}