#import <Cocoa/Cocoa.h>
#import <Foundation/Foundation.h>

static NSInteger lastChangeCount = 0;

_Bool hasClipboardChanged() {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    NSInteger currentChangeCount = [pasteboard changeCount];

    if (currentChangeCount != lastChangeCount) {
        lastChangeCount = currentChangeCount;
        return 1;
    }

    return 0;
}

// GetClipboardContentType returns 0=empty, 1=text, 2=image, 3=file
int GetClipboardContentType() {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];

    // Check for file URLs first (highest priority)
    NSArray *fileClasses = [NSArray arrayWithObject:[NSURL class]];
    if ([pasteboard canReadObjectForClasses:fileClasses options:nil]) {
        return 3;
    }

    // Check for image types
    NSArray *imageTypes = [NSArray arrayWithObjects:NSPasteboardTypePNG, NSPasteboardTypeTIFF, nil];
    if ([pasteboard availableTypeFromArray:imageTypes] != nil) {
        return 2;
    }

    // Check for text
    if ([pasteboard availableTypeFromArray:[NSArray arrayWithObject:NSPasteboardTypeString]] != nil) {
        return 1;
    }

    return 0;
}

const char* GetClipboardText() {
    @try {
        NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
        NSString *string = [pasteboard stringForType:NSPasteboardTypeString];

        if (string != nil) {
            return [string UTF8String];
        } else {
            return NULL;
        }
    }
    @catch (NSException *exception) {
        NSLog(@"Exception occurred: %@, %@", exception, [exception userInfo]);
        return NULL;
    }
}

char* GetAllClipboardFilePaths() {
    @try {
        NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
        NSArray *classArray = [NSArray arrayWithObject:[NSURL class]];
        NSDictionary *options = [NSDictionary dictionary];

        if (![pasteboard canReadObjectForClasses:classArray options:options]) {
            return NULL; // No file in clipboard
        }

        NSArray *urls = [pasteboard readObjectsForClasses:classArray options:options];
        if (urls == nil || [urls count] == 0) {
            return NULL;
        }

        NSMutableString *allPaths = [[NSMutableString alloc] init];
        for (NSURL *url in urls) {
            [allPaths appendString:[url path]];
            [allPaths appendString:@"\n"];
        }

        return strdup([allPaths UTF8String]);
    }
    @catch (NSException *exception) {
        NSLog(@"Exception occurred: %@, %@", exception, [exception userInfo]);
        return NULL;
    }
}

unsigned char *GetClipboardImage(size_t *length) {
    @try {
        NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
        NSImage *image = [[NSImage alloc] initWithPasteboard:pasteboard];
        if (image == nil) {
            return NULL;
        }

        NSData *tiffData = [image TIFFRepresentation];
        NSBitmapImageRep *imageRep = [NSBitmapImageRep imageRepWithData:tiffData];
        NSDictionary *imageProps = [NSDictionary dictionaryWithObject:[NSNumber numberWithFloat:1.0] forKey:NSImageCompressionFactor];
        NSData *pngData = [imageRep representationUsingType:NSBitmapImageFileTypePNG properties:imageProps];

        *length = [pngData length];
        unsigned char *bytes = (unsigned char *)malloc(*length);
        memcpy(bytes, [pngData bytes], *length);

        return bytes;
    }
    @catch (NSException *exception) {
        NSLog(@"Exception occurred: %@, %@", exception, [exception userInfo]);
        return NULL;
    }
}

void WriteClipboardText(const char *text) {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    NSString *string = [NSString stringWithUTF8String:text];
    [pasteboard setString:string forType:NSPasteboardTypeString];
}

void WriteClipboardFiles(const char **filePaths, int count) {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];

    NSMutableArray *urls = [NSMutableArray arrayWithCapacity:count];
    for (int i = 0; i < count; i++) {
        if (filePaths[i] == NULL) {
            continue;
        }

        NSString *path = [NSString stringWithUTF8String:filePaths[i]];
        if (path == nil || [path length] == 0) {
            continue;
        }

        NSURL *url = [NSURL fileURLWithPath:path];
        if (url != nil) {
            [urls addObject:url];
        }
    }

    if ([urls count] > 0) {
        [pasteboard writeObjects:urls];
    }
}

void WriteClipboardImage(const char *imageData, int length) {
    NSData *data = [NSData dataWithBytes:imageData length:length];
    NSImage *image = [[NSImage alloc] initWithData:data];
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    [pasteboard writeObjects:@[image]];
}