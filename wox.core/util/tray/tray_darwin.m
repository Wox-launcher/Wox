#include "_cgo_export.h"
#import <Cocoa/Cocoa.h>
#import <ImageIO/ImageIO.h>

static NSStatusItem *globalStatusItem = nil;
static NSImage *globalStatusItemImage = nil;
static NSMenu *globalMenu = nil;
static NSMutableArray<NSStatusItem *> *queryStatusItems = nil;
static NSMutableArray *queryTargets = nil;
static BOOL globalStatusItemShowsQuery = NO;

extern void reportLeftClick();

static void restoreGlobalStatusItem(void);

static void showGlobalStatusItemMenu(void) {
  if (globalStatusItem == nil || globalMenu == nil ||
      globalStatusItem.button == nil) {
    return;
  }

  NSStatusBarButton *button = globalStatusItem.button;
  [button highlight:YES];
  [globalMenu popUpMenuPositioningItem:nil
                            atLocation:NSMakePoint(0, NSHeight(button.bounds))
                                inView:button];
  [button highlight:NO];
}

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
  if (event.type == NSEventTypeRightMouseUp ||
      (event.type == NSEventTypeLeftMouseUp &&
       (event.modifierFlags & NSEventModifierFlagControl))) {
    showGlobalStatusItemMenu();
  } else {
    reportLeftClick();
  }
}
@end

static MenuItemTarget *globalTarget = nil;

@interface QueryTrayTarget : NSObject
@property(nonatomic, assign) NSInteger tag;
@property(nonatomic, retain) NSMenu *menu;
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

- (void)dealloc {
  [_menu release];
  [super dealloc];
}

- (void)queryTrayClick:(id)sender {
  NSEvent *event = [NSApp currentEvent];
  BOOL isContextClick = event.type == NSEventTypeRightMouseUp ||
                        (event.type == NSEventTypeLeftMouseUp &&
                         (event.modifierFlags & NSEventModifierFlagControl));
  if (isContextClick && self.menu != nil &&
      [sender isKindOfClass:[NSStatusBarButton class]]) {
    NSStatusBarButton *button = (NSStatusBarButton *)sender;
    [button highlight:YES];
    [self.menu popUpMenuPositioningItem:nil
                             atLocation:NSMakePoint(0, NSHeight(button.bounds))
                                 inView:button];
    [button highlight:NO];
    return;
  }

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

  GoQueryTrayCallback((GoInt)self.tag, screenRect.origin.x, topY,
                      screenRect.size.width, screenRect.size.height);
}
@end

void clearQueryTrayIcons() {
  @autoreleasepool {
    NSStatusBar *bar = [NSStatusBar systemStatusBar];

    restoreGlobalStatusItem();

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

static NSStatusItem *createQueryStatusItem(int tag, const char *identifier,
                                           const char *tooltip, int menuTag,
                                           const char *menuTitle) {
  NSStatusBar *bar = [NSStatusBar systemStatusBar];
  NSStatusItem *statusItem =
      [bar statusItemWithLength:NSSquareStatusItemLength];
  if (statusItem == nil) {
    return nil;
  }
  statusItem.visible = NO;
  if (identifier != nil && identifier[0] != '\0') {
    NSString *identifierString = [NSString stringWithUTF8String:identifier];
    statusItem.autosaveName =
        [@"Wox.TrayQuery." stringByAppendingString:identifierString];
  }

  if (tooltip != nil) {
    statusItem.button.toolTip = [NSString stringWithUTF8String:tooltip];
  }

  QueryTrayTarget *target = [[QueryTrayTarget alloc] initWithTag:tag];
  if (menuTag >= 0 && menuTitle != nil && menuTitle[0] != '\0' &&
      globalTarget != nil) {
    NSString *itemTitle = [NSString stringWithUTF8String:menuTitle];
    NSMenu *menu = [[NSMenu alloc] init];
    NSMenuItem *menuItem =
        [[NSMenuItem alloc] initWithTitle:itemTitle
                                   action:@selector(menuItemAction:)
                            keyEquivalent:@""];
    menuItem.tag = menuTag;
    menuItem.target = globalTarget;
    [menu addItem:menuItem];
    [menuItem release];
    target.menu = menu;
    [menu release];
  }

  [statusItem.button setAction:@selector(queryTrayClick:)];
  [statusItem.button setTarget:target];
  [statusItem.button
      sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];

  [queryStatusItems addObject:statusItem];
  [queryTargets addObject:target];
  [target release];
  return statusItem;
}

// trayImageFromBytes forces ImageIO to decode a concrete bitmap representation
// while the C buffer is valid, avoiding NSImage's lazy data decoding.
static NSImage *trayImageFromBytes(const char *iconBytes, int length) {
  if (iconBytes == NULL || length <= 0) {
    return nil;
  }

  CFDataRef iconData =
      CFDataCreate(kCFAllocatorDefault, (const UInt8 *)iconBytes, length);
  if (iconData == NULL) {
    return nil;
  }
  CGImageSourceRef source = CGImageSourceCreateWithData(iconData, NULL);
  CFRelease(iconData);
  if (source == NULL) {
    return nil;
  }
  CGImageRef cgImage = CGImageSourceCreateImageAtIndex(source, 0, NULL);
  CFRelease(source);
  if (cgImage == NULL) {
    return nil;
  }

  NSImage *image = [[NSImage alloc] initWithCGImage:cgImage
                                               size:NSMakeSize(16, 16)];
  CGImageRelease(cgImage);
  [image setTemplate:NO];
  return image;
}

static void setTrayStatusItemImage(NSStatusItem *statusItem, NSImage *image) {
  if (statusItem == nil || statusItem.button == nil || image == nil) {
    return;
  }

  statusItem.button.title = @"";
  statusItem.button.imagePosition = NSImageOnly;
  statusItem.button.imageScaling = NSImageScaleProportionallyDown;
  statusItem.button.image = image;
  statusItem.visible = YES;
  [statusItem.button setNeedsDisplay:YES];
}

static void restoreGlobalStatusItem(void) {
  if (!globalStatusItemShowsQuery || globalStatusItem == nil ||
      globalStatusItem.button == nil) {
    return;
  }

  setTrayStatusItemImage(globalStatusItem, globalStatusItemImage);
  globalStatusItem.button.toolTip = nil;
  [globalStatusItem.button setAction:@selector(trayClick:)];
  [globalStatusItem.button setTarget:globalTarget];
  [globalStatusItem.button
      sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
  globalStatusItemShowsQuery = NO;
}

// macOS reports status items as visible even when menu bar layout hides them.
// On notched displays, Wox only treats the right auxiliary area as reliably
// renderable because items placed on the left can be obscured by app menus.
static BOOL queryStatusItemHasRenderablePosition(NSStatusItem *statusItem) {
  if (statusItem == nil || statusItem.button == nil ||
      statusItem.button.window == nil) {
    return YES;
  }

  NSStatusBarButton *button = statusItem.button;
  NSRect windowRect = [button convertRect:button.bounds toView:nil];
  NSRect screenRect = [button.window convertRectToScreen:windowRect];
  NSScreen *screen = button.window.screen ?: NSScreen.mainScreen;
  if (screen == nil) {
    return YES;
  }

  if (@available(macOS 12.0, *)) {
    NSRect rightArea = screen.auxiliaryTopRightArea;
    if (!NSIsEmptyRect(rightArea)) {
      return NSIntersectsRect(screenRect, rightArea);
    }
  }
  return YES;
}

// promoteQueryStatusItemToGlobal reuses Wox's visible main status slot when
// macOS places a query item outside the reliably renderable menu bar area.
static BOOL promoteQueryStatusItemToGlobal(NSStatusItem *statusItem,
                                           NSImage *image) {
  if (globalStatusItemShowsQuery || globalStatusItem == nil ||
      globalStatusItem.button == nil || queryTargets.count == 0 ||
      queryStatusItemHasRenderablePosition(statusItem)) {
    return NO;
  }

  QueryTrayTarget *target = (QueryTrayTarget *)statusItem.button.target;
  if (target == nil) {
    return NO;
  }
  NSString *tooltip = [statusItem.button.toolTip copy];
  [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
  [queryStatusItems removeObjectIdenticalTo:statusItem];

  setTrayStatusItemImage(globalStatusItem, image);
  globalStatusItem.button.toolTip = tooltip;
  [tooltip release];
  [globalStatusItem.button setAction:@selector(queryTrayClick:)];
  [globalStatusItem.button setTarget:target];
  [globalStatusItem.button
      sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
  globalStatusItemShowsQuery = YES;
  return YES;
}

// Control Center hosts status items asynchronously on recent macOS versions,
// so the initial button window frame can be a transient overflow position.
static void scheduleQueryStatusItemFallback(NSStatusItem *statusItem, int tag) {
  [statusItem retain];
  dispatch_after(
      dispatch_time(DISPATCH_TIME_NOW, 500 * NSEC_PER_MSEC),
      dispatch_get_main_queue(), ^{
        if ([queryStatusItems containsObject:statusItem]) {
          NSImage *image = statusItem.button.image;
          if (promoteQueryStatusItemToGlobal(statusItem, image)) {
            reportQueryTrayFallback(tag);
          }
        }
        [statusItem release];
      });
}

int addQueryTray(const char *iconBytes, int length, int tag,
                 const char *identifier, const char *tooltip, int menuTag,
                 const char *menuTitle) {
  @autoreleasepool {
    [NSApplication sharedApplication];

    if (queryStatusItems == nil) {
      queryStatusItems = [[NSMutableArray alloc] init];
    }
    if (queryTargets == nil) {
      queryTargets = [[NSMutableArray alloc] init];
    }

    NSStatusItem *statusItem =
        createQueryStatusItem(tag, identifier, tooltip, menuTag, menuTitle);
    if (statusItem == nil) {
      return 0;
    }

    NSImage *icon = trayImageFromBytes(iconBytes, length);
    if (icon == nil) {
      return -1;
    }
    setTrayStatusItemImage(statusItem, icon);
    BOOL imageSet = statusItem.button.image != nil;
    if (imageSet) {
      scheduleQueryStatusItemFallback(statusItem, tag);
    }
    [icon release];
    if (!imageSet) {
      return -2;
    }
    return 1;
  }
}

void createTray(const char *iconBytes, int length) {
  @autoreleasepool {
    [NSApplication sharedApplication];

    NSStatusBar *bar = [NSStatusBar systemStatusBar];

    globalStatusItem = [bar statusItemWithLength:NSVariableStatusItemLength];
    [globalStatusItem retain];
    globalStatusItem.visible = NO;
    // Stable names let Control Center reuse hosted slots after abrupt debug
    // termination instead of accumulating unnamed blank placeholders.
    globalStatusItem.autosaveName = @"Wox.MainTray";

    NSImage *icon = trayImageFromBytes(iconBytes, length);
    if (icon != nil) {
      [globalStatusItemImage release];
      globalStatusItemImage = [icon retain];
      setTrayStatusItemImage(globalStatusItem, icon);
      [icon release];
    }

    globalMenu = [[NSMenu alloc] init];
    globalTarget = [[MenuItemTarget alloc] init];

    [globalStatusItem.button setAction:@selector(trayClick:)];
    [globalStatusItem.button setTarget:globalTarget];
    [globalStatusItem.button
        sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
  }
}

void addMenuItem(const char *title, int tag) {
  @autoreleasepool {
    if (globalMenu != nil) {
      NSString *itemTitle = [NSString stringWithUTF8String:title];
      NSMenuItem *item =
          [[NSMenuItem alloc] initWithTitle:itemTitle
                                     action:@selector(menuItemAction:)
                              keyEquivalent:@""];
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
    if (globalStatusItemImage != nil) {
      [globalStatusItemImage release];
      globalStatusItemImage = nil;
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
