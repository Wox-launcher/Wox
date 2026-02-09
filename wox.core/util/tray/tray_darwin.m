#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"

static NSStatusItem *globalStatusItem = nil;
static NSMenu *globalMenu = nil;
static NSMutableArray<NSStatusItem *> *queryStatusItems = nil;
static NSMutableArray *queryTargets = nil;

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

@interface QueryTrayTarget : NSObject
@property(nonatomic, assign) NSInteger tag;
- (instancetype)initWithTag:(NSInteger)tag;
- (void)queryTrayClick:(id)sender;
@end

@implementation QueryTrayTarget
- (instancetype)initWithTag:(NSInteger)tag {
    self = [super init];
    if (self) {
        _tag = tag;
    }
    return self;
}

- (void)queryTrayClick:(id)sender {
    if (![sender isKindOfClass:[NSStatusBarButton class]]) {
        GoQueryTrayCallback((GoInt)self.tag, 0, 0, 0, 0);
        return;
    }

    NSStatusBarButton *button = (NSStatusBarButton *)sender;
    NSWindow *buttonWindow = button.window;
    if (buttonWindow == nil) {
        GoQueryTrayCallback((GoInt)self.tag, 0, 0, 0, 0);
        return;
    }

    NSRect screenRect = [buttonWindow convertRectToScreen:button.frame];
    NSPoint midPoint = NSMakePoint(NSMidX(screenRect), NSMidY(screenRect));
    NSScreen *targetScreen = nil;
    for (NSScreen *screen in [NSScreen screens]) {
        if (NSPointInRect(midPoint, screen.frame)) {
            targetScreen = screen;
            break;
        }
    }
    if (targetScreen == nil) {
        targetScreen = buttonWindow.screen ?: NSScreen.mainScreen;
    }

    NSRect targetFrame = targetScreen ? targetScreen.frame : NSZeroRect;
    CGFloat topY = NSMaxY(targetFrame) - NSMaxY(screenRect);
    if (topY < 0) {
        topY = 0;
    }

    GoQueryTrayCallback((GoInt)self.tag, screenRect.origin.x, topY, screenRect.size.width, screenRect.size.height);
}
@end

void clearQueryTrayIcons() {
    @autoreleasepool {
        NSStatusBar *bar = [NSStatusBar systemStatusBar];

        if (queryStatusItems != nil) {
            for (NSStatusItem *item in queryStatusItems) {
                [bar removeStatusItem:item];
            }
            [queryStatusItems removeAllObjects];
        }
        if (queryTargets != nil) {
            [queryTargets removeAllObjects];
        }
    }
}

void addQueryTray(const char *iconBytes, int length, int tag, const char *tooltip) {
    @autoreleasepool {
        [NSApplication sharedApplication];

        if (queryStatusItems == nil) {
            queryStatusItems = [[NSMutableArray alloc] init];
        }
        if (queryTargets == nil) {
            queryTargets = [[NSMutableArray alloc] init];
        }

        NSStatusBar *bar = [NSStatusBar systemStatusBar];
        NSStatusItem *statusItem = [bar statusItemWithLength:NSVariableStatusItemLength];
        if (statusItem == nil) {
            return;
        }

        NSData *iconData = [NSData dataWithBytesNoCopy:(void *)iconBytes length:length freeWhenDone:NO];
        NSImage *icon = [[NSImage alloc] initWithData:iconData];
        if (icon != nil) {
            [icon setSize:NSMakeSize(16, 16)];
            statusItem.button.image = icon;
            [icon release];
        }

        if (tooltip != nil) {
            statusItem.button.toolTip = [NSString stringWithUTF8String:tooltip];
        }

        QueryTrayTarget *target = [[QueryTrayTarget alloc] initWithTag:tag];
        [statusItem.button setAction:@selector(queryTrayClick:)];
        [statusItem.button setTarget:target];
        [statusItem.button sendActionOn:(NSEventMaskLeftMouseUp)];

        [queryStatusItems addObject:statusItem];
        [queryTargets addObject:target];
        [target release];
    }
}

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

        clearQueryTrayIcons();

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
        if (queryStatusItems != nil) {
            [queryStatusItems release];
            queryStatusItems = nil;
        }
        if (queryTargets != nil) {
            [queryTargets release];
            queryTargets = nil;
        }
    }
}
