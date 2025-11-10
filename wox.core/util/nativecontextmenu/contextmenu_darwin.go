package nativecontextmenu

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework AppKit

#import <Cocoa/Cocoa.h>
#import <Foundation/Foundation.h>

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

        // Convert mouse location to window coordinates
        // macOS uses bottom-left origin, so we need to flip Y coordinate
        CGFloat screenHeight = [targetScreen frame].size.height;
        NSPoint windowPoint = NSMakePoint(mouseLocation.x, screenHeight - mouseLocation.y);

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

        // Create the context menu
        NSMenu *menu = [[NSMenu alloc] initWithTitle:@""];
        [menu setAutoenablesItems:YES];

        // Get file URL
        NSURL *fileURL = [NSURL fileURLWithPath:filePath];

        // Add "Open" menu item
        NSMenuItem *openItem = [[NSMenuItem alloc] initWithTitle:@"Open"
                                                          action:@selector(openFile:)
                                                   keyEquivalent:@""];
        [openItem setRepresentedObject:fileURL];
        [openItem setTarget:[[NSWorkspace sharedWorkspace] class]];
        [menu addItem:openItem];

        // Add "Show in Finder" menu item
        NSMenuItem *showInFinderItem = [[NSMenuItem alloc] initWithTitle:@"Show in Finder"
                                                                  action:@selector(revealInFinder:)
                                                           keyEquivalent:@""];
        [showInFinderItem setRepresentedObject:fileURL];
        [showInFinderItem setTarget:[[NSWorkspace sharedWorkspace] class]];
        [menu addItem:showInFinderItem];

        [menu addItem:[NSMenuItem separatorItem]];

        // Add "Get Info" menu item
        NSMenuItem *getInfoItem = [[NSMenuItem alloc] initWithTitle:@"Get Info"
                                                             action:@selector(showFileInfo:)
                                                      keyEquivalent:@""];
        [getInfoItem setRepresentedObject:fileURL];
        [menu addItem:getInfoItem];

        // Add "Quick Look" menu item
        NSMenuItem *quickLookItem = [[NSMenuItem alloc] initWithTitle:@"Quick Look"
                                                               action:@selector(quickLook:)
                                                        keyEquivalent:@""];
        [quickLookItem setRepresentedObject:fileURL];
        [menu addItem:quickLookItem];

        [menu addItem:[NSMenuItem separatorItem]];

        // Add "Copy" menu item
        NSMenuItem *copyItem = [[NSMenuItem alloc] initWithTitle:@"Copy"
                                                          action:@selector(copyFile:)
                                                   keyEquivalent:@""];
        [copyItem setRepresentedObject:fileURL];
        [menu addItem:copyItem];

        // Add "Move to Trash" menu item
        NSMenuItem *trashItem = [[NSMenuItem alloc] initWithTitle:@"Move to Trash"
                                                           action:@selector(moveToTrash:)
                                                    keyEquivalent:@""];
        [trashItem setRepresentedObject:fileURL];
        [menu addItem:trashItem];

        // Display the menu
        // Convert screen coordinates to window coordinates
        NSPoint menuLocation = NSMakePoint(0, 0);

        // Show the menu
        [menu popUpMenuPositioningItem:nil atLocation:menuLocation inView:[window contentView]];

        // Keep the window alive while menu is shown
        // The menu will close automatically when user clicks outside or selects an item
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.1 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
            [window close];
        });

        return 0;
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
