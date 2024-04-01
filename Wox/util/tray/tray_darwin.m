#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"

static NSStatusItem *globalStatusItem = nil;

@interface MenuItemTarget : NSObject
@end

@implementation MenuItemTarget
- (void)menuItemAction:(id)sender {
    if ([sender isKindOfClass:[NSMenuItem class]]) {
        NSMenuItem *menuItem = (NSMenuItem *)sender;
        GoMenuItemCallback((GoInt)(menuItem.tag));
    }
}
@end

void createTray(const char *iconBytes, int length) {
    @autoreleasepool {
        [NSApplication sharedApplication];

        NSStatusBar *bar = [NSStatusBar systemStatusBar];

        globalStatusItem = [bar statusItemWithLength:NSVariableStatusItemLength];
        [globalStatusItem retain];

        NSData *iconData = [NSData dataWithBytesNoCopy:(void *)iconBytes length:length freeWhenDone:NO];
        NSImage *icon = [[NSImage alloc] initWithData:iconData];

        [icon setSize:NSMakeSize(16, 16)];

        globalStatusItem.button.image = icon;

        NSMenu *menu = [[NSMenu alloc] init];
        [globalStatusItem setMenu:menu];
    }
}

void addMenuItem(const char *title, int tag) {
    @autoreleasepool {
        if (globalStatusItem) {
            NSMenu *menu = globalStatusItem.menu;
            NSString *itemTitle = [NSString stringWithUTF8String:title];
            MenuItemTarget *target = [[MenuItemTarget alloc] init];
            NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:itemTitle action:@selector(menuItemAction:) keyEquivalent:@""];
            item.tag = tag;
            item.target = target;
            [menu addItem:item];
        }
    }
}

void removeTray() {
    @autoreleasepool {
        NSStatusBar *bar = [NSStatusBar systemStatusBar];

        if (globalStatusItem != nil) {
            [bar removeStatusItem:globalStatusItem];
            globalStatusItem = nil;
        }
    }
}