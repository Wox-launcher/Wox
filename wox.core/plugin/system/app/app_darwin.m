#import <Cocoa/Cocoa.h>
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <stdio.h>
#include <sys/sysctl.h>
#include <libproc.h>

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

int get_process_list(struct kinfo_proc **procList, size_t *procCount) {
    int err;
    size_t length;
    static const int name[] = { CTL_KERN, KERN_PROC, KERN_PROC_ALL, 0 };

    if (sysctl((int *)name, 4, NULL, &length, NULL, 0) == -1) {
        return -1;
    }

    *procList = malloc(length);
    if (!*procList) {
        return -1;
    }

    if (sysctl((int *)name, 4, *procList, &length, NULL, 0) == -1) {
        free(*procList);
        return -1;
    }

    *procCount = length / sizeof(struct kinfo_proc);

    return 0;
}

char* get_process_path(pid_t pid) {
    char *path = (char *)malloc(PROC_PIDPATHINFO_MAXSIZE);
    if (!path) {
        return NULL;
    }

    if (proc_pidpath(pid, path, PROC_PIDPATHINFO_MAXSIZE) <= 0) {
        free(path);
        return NULL;
    }

    return path;
}