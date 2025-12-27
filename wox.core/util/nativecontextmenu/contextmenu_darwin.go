package nativecontextmenu

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework AppKit

#import <Cocoa/Cocoa.h>
#import <Foundation/Foundation.h>

@interface FileMenuHandler : NSObject
@property (strong) NSURL *fileURL;
@property (strong) NSString *filePath;
- (void)openFile:(id)sender;
- (void)showInFinder:(id)sender;
- (void)moveToTrash:(id)sender;
- (void)getInfo:(id)sender;
- (void)renameFile:(id)sender;
- (void)compressFile:(id)sender;
- (void)duplicateFile:(id)sender;
- (void)makeAlias:(id)sender;
- (void)quickLook:(id)sender;
- (void)copyFile:(id)sender;
- (void)copyPath:(id)sender;
@end

@implementation FileMenuHandler
- (void)openFile:(id)sender {
    [[NSWorkspace sharedWorkspace] openURL:self.fileURL];
}

- (void)showInFinder:(id)sender {
    [[NSWorkspace sharedWorkspace] activateFileViewerSelectingURLs:@[self.fileURL]];
}

- (void)moveToTrash:(id)sender {
    [[NSFileManager defaultManager] trashItemAtURL:self.fileURL resultingItemURL:nil error:nil];
}

- (void)getInfo:(id)sender {
    // Use AppleScript to show Get Info window
    NSString *script = [NSString stringWithFormat:@"tell application \"Finder\" to open information window of (POSIX file \"%@\" as alias)", self.filePath];
    NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:script];
    [appleScript executeAndReturnError:nil];
}

- (void)renameFile:(id)sender {
    [[NSWorkspace sharedWorkspace] activateFileViewerSelectingURLs:@[self.fileURL]];
    // Trigger rename via AppleScript
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.2 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
        NSString *script = @"tell application \"System Events\" to tell process \"Finder\" to keystroke return";
        NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:script];
        [appleScript executeAndReturnError:nil];
    });
}

- (void)compressFile:(id)sender {
    NSTask *task = [[NSTask alloc] init];
    [task setLaunchPath:@"/usr/bin/ditto"];
    [task setArguments:@[@"-c", @"-k", @"--sequesterRsrc", @"--keepParent", self.filePath, [NSString stringWithFormat:@"%@.zip", self.filePath]]];
    [task launch];
}

- (void)duplicateFile:(id)sender {
    NSFileManager *fm = [NSFileManager defaultManager];
    NSString *directory = [self.filePath stringByDeletingLastPathComponent];
    NSString *filename = [[self.filePath lastPathComponent] stringByDeletingPathExtension];
    NSString *extension = [self.filePath pathExtension];
    NSString *newPath = [NSString stringWithFormat:@"%@/%@ copy.%@", directory, filename, extension];

    int counter = 1;
    while ([fm fileExistsAtPath:newPath]) {
        newPath = [NSString stringWithFormat:@"%@/%@ copy %d.%@", directory, filename, counter++, extension];
    }

    [fm copyItemAtPath:self.filePath toPath:newPath error:nil];
}

- (void)makeAlias:(id)sender {
    NSString *directory = [self.filePath stringByDeletingLastPathComponent];
    NSString *aliasPath = [NSString stringWithFormat:@"%@/%@ alias", directory, [self.filePath lastPathComponent]];
    [[NSFileManager defaultManager] createSymbolicLinkAtPath:aliasPath withDestinationPath:self.filePath error:nil];
}

- (void)quickLook:(id)sender {
    [[NSWorkspace sharedWorkspace] activateFileViewerSelectingURLs:@[self.fileURL]];
    // Trigger Quick Look via keystroke
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.2 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
        NSString *script = @"tell application \"System Events\" to keystroke \" \"";
        NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:script];
        [appleScript executeAndReturnError:nil];
    });
}

- (void)copyFile:(id)sender {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    [pasteboard writeObjects:@[self.fileURL]];
}

- (void)copyPath:(id)sender {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    [pasteboard setString:self.filePath forType:NSPasteboardTypeString];
}
@end

// ShowContextMenu displays the macOS context menu for a file or folder
// Returns 0 on success, non-zero on error
int ShowContextMenu(const char* path) {
    @autoreleasepool {
        NSString *filePath = [NSString stringWithUTF8String:path];
        if (!filePath) {
            return 1;
        }

        // Check if file exists
        NSFileManager *fileManager = [NSFileManager defaultManager];
        if (![fileManager fileExistsAtPath:filePath]) {
            return 2;
        }

        __block int result = 0;

        // All UI operations must be performed on the main thread
        dispatch_sync(dispatch_get_main_queue(), ^{
            @autoreleasepool {
                // Get the mouse location
                NSPoint mouseLocation = [NSEvent mouseLocation];

                // Find the screen containing the mouse
                NSScreen *targetScreen = nil;
                for (NSScreen *screen in [NSScreen screens]) {
                    if (NSPointInRect(mouseLocation, [screen frame])) {
                        targetScreen = screen;
                        break;
                    }
                }

                if (!targetScreen) {
                    targetScreen = [NSScreen mainScreen];
                }

                // Create a minimal window at the mouse position
                NSRect windowFrame = NSMakeRect(mouseLocation.x, mouseLocation.y, 1, 1);
                NSWindow *window = [[NSWindow alloc] initWithContentRect:windowFrame
                                                               styleMask:NSWindowStyleMaskBorderless
                                                                 backing:NSBackingStoreBuffered
                                                                   defer:NO];
                [window setLevel:NSPopUpMenuWindowLevel];
                [window setOpaque:NO];
                [window setBackgroundColor:[NSColor clearColor]];
                [window makeKeyAndOrderFront:nil];

                // Get file URL and attributes
                NSURL *fileURL = [NSURL fileURLWithPath:filePath];
                NSDictionary *attributes = [fileManager attributesOfItemAtPath:filePath error:nil];
                BOOL isDirectory = [[attributes objectForKey:NSFileType] isEqualToString:NSFileTypeDirectory];
                BOOL isAppBundle = isDirectory && [filePath hasSuffix:@".app"];

                // Create handler
                FileMenuHandler *handler = [[FileMenuHandler alloc] init];
                handler.fileURL = fileURL;
                handler.filePath = filePath;

                // Create the context menu
                NSMenu *menu = [[NSMenu alloc] initWithTitle:@""];
                [menu setAutoenablesItems:NO];

                // Open
                NSMenuItem *openItem = [[NSMenuItem alloc] initWithTitle:@"Open"
                                                                  action:@selector(openFile:)
                                                           keyEquivalent:@""];
                [openItem setTarget:handler];
                [openItem setEnabled:YES];
                [menu addItem:openItem];

                // Show Package Contents (for .app bundles)
                if (isAppBundle) {
                    NSMenuItem *showPackageItem = [[NSMenuItem alloc] initWithTitle:@"Show Package Contents"
                                                                              action:@selector(showInFinder:)
                                                                       keyEquivalent:@""];
                    [showPackageItem setTarget:handler];
                    [showPackageItem setEnabled:YES];
                    [menu addItem:showPackageItem];
                }

                [menu addItem:[NSMenuItem separatorItem]];

                // Move to Trash
                NSMenuItem *trashItem = [[NSMenuItem alloc] initWithTitle:@"Move to Trash"
                                                                   action:@selector(moveToTrash:)
                                                            keyEquivalent:@""];
                [trashItem setTarget:handler];
                [trashItem setEnabled:YES];
                [menu addItem:trashItem];

                [menu addItem:[NSMenuItem separatorItem]];

                // Get Info
                NSMenuItem *infoItem = [[NSMenuItem alloc] initWithTitle:@"Get Info"
                                                                  action:@selector(getInfo:)
                                                           keyEquivalent:@""];
                [infoItem setTarget:handler];
                [infoItem setEnabled:YES];
                [menu addItem:infoItem];

                // Rename
                NSMenuItem *renameItem = [[NSMenuItem alloc] initWithTitle:@"Rename"
                                                                    action:@selector(renameFile:)
                                                             keyEquivalent:@""];
                [renameItem setTarget:handler];
                [renameItem setEnabled:YES];
                [menu addItem:renameItem];

                // Compress
                NSString *compressTitle = [NSString stringWithFormat:@"Compress \"%@\"", [filePath lastPathComponent]];
                NSMenuItem *compressItem = [[NSMenuItem alloc] initWithTitle:compressTitle
                                                                      action:@selector(compressFile:)
                                                               keyEquivalent:@""];
                [compressItem setTarget:handler];
                [compressItem setEnabled:YES];
                [menu addItem:compressItem];

                // Duplicate
                NSMenuItem *duplicateItem = [[NSMenuItem alloc] initWithTitle:@"Duplicate"
                                                                       action:@selector(duplicateFile:)
                                                                keyEquivalent:@""];
                [duplicateItem setTarget:handler];
                [duplicateItem setEnabled:YES];
                [menu addItem:duplicateItem];

                // Make Alias
                NSMenuItem *aliasItem = [[NSMenuItem alloc] initWithTitle:@"Make Alias"
                                                                   action:@selector(makeAlias:)
                                                            keyEquivalent:@""];
                [aliasItem setTarget:handler];
                [aliasItem setEnabled:YES];
                [menu addItem:aliasItem];

                // Quick Look
                NSMenuItem *quickLookItem = [[NSMenuItem alloc] initWithTitle:@"Quick Look"
                                                                       action:@selector(quickLook:)
                                                                keyEquivalent:@""];
                [quickLookItem setTarget:handler];
                [quickLookItem setEnabled:YES];
                [menu addItem:quickLookItem];

                [menu addItem:[NSMenuItem separatorItem]];

                // Copy
                NSMenuItem *copyItem = [[NSMenuItem alloc] initWithTitle:@"Copy"
                                                                  action:@selector(copyFile:)
                                                           keyEquivalent:@""];
                [copyItem setTarget:handler];
                [copyItem setEnabled:YES];
                [menu addItem:copyItem];

                // Copy Path
                NSMenuItem *copyPathItem = [[NSMenuItem alloc] initWithTitle:@"Copy Path"
                                                                      action:@selector(copyPath:)
                                                               keyEquivalent:@""];
                [copyPathItem setTarget:handler];
                [copyPathItem setEnabled:YES];
                [menu addItem:copyPathItem];

                // Display the menu at mouse location
                NSPoint menuLocation = NSMakePoint(0, 0);
                [menu popUpMenuPositioningItem:nil atLocation:menuLocation inView:[window contentView]];

                // Keep the window and handler alive while menu is shown
                dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(5.0 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
                    [window close];
                    // Keep handler alive
                    (void)handler;
                });
            }
        });

        return result;
    }
}

// Helper method implementations would go in a separate Objective-C category
// For now, we'll use NSWorkspace methods directly
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// ShowContextMenu displays the system context menu for a file or folder on macOS
func ShowContextMenu(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	result := C.ShowContextMenu(cPath)
	if result != 0 {
		return fmt.Errorf("failed to show context menu, error code: %d", result)
	}

	return nil
}
