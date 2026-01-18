#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"

static NSStatusItem *globalStatusItem = nil;
static NSMenu *globalMenu = nil;

extern void reportLeftClick();

@interface MenuItemTarget : NSObject
@end

@implementation MenuItemTarget
- (void)menuItemAction:(id)sender {
    if ([sender isKindOfClass:[NSMenuItem class]]) {
        NSMenuItem *menuItem = (NSMenuItem *)sender;
        GoMenuItemCallback((GoInt)(menuItem.tag));
    }
}

- (void)trayClick:(id)sender {
    NSEvent *event = [NSApp currentEvent];
    if (event.type == NSEventTypeRightMouseUp || (event.type == NSEventTypeLeftMouseUp && (event.modifierFlags & NSEventModifierFlagControl))) {
        if (globalStatusItem != nil && globalMenu != nil) {
            [globalStatusItem popUpStatusItemMenu:globalMenu];
        }
    } else {
        reportLeftClick();
    }
}
@end

static MenuItemTarget *globalTarget = nil;

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

        globalMenu = [[NSMenu alloc] init];
        globalTarget = [[MenuItemTarget alloc] init];

        [globalStatusItem.button setAction:@selector(trayClick:)];
        [globalStatusItem.button setTarget:globalTarget];
        [globalStatusItem.button sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
    }
}

void addMenuItem(const char *title, int tag) {
    @autoreleasepool {
        if (globalMenu != nil) {
            NSString *itemTitle = [NSString stringWithUTF8String:title];
            NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:itemTitle action:@selector(menuItemAction:) keyEquivalent:@""];
            item.tag = tag;
            item.target = globalTarget;
            [globalMenu addItem:item];
        }
    }
}

void removeTray() {
    @autoreleasepool {
        NSStatusBar *bar = [NSStatusBar systemStatusBar];

        if (globalStatusItem != nil) {
            [bar removeStatusItem:globalStatusItem];
            [globalStatusItem release];
            globalStatusItem = nil;
        }
        if (globalMenu != nil) {
            [globalMenu release];
            globalMenu = nil;
        }
        if (globalTarget != nil) {
            [globalTarget release];
            globalTarget = nil;
        }
    }
}
