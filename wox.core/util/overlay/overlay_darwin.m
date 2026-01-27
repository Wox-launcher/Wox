#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>

static const CGFloat kWindowWidth = 400;
static const CGFloat kWindowHeight = 60;
static const NSTimeInterval kAutoCloseSeconds = 3.0;
static const CGFloat kIconSize = 32;
static const CGFloat kPadding = 16;

extern char* getActiveFinderWindowPath();
extern void overlayClickCallbackCGO();
extern void finderActivationCallbackCGO(int x, int y, int width, int height);

@interface ExplorerHintWindow : NSPanel
@property(nonatomic, strong) NSTimer *closeTimer;
@property(nonatomic, strong) NSImageView *iconView;
@property(nonatomic, strong) NSTextField *messageLabel;
@property(nonatomic, strong) NSVisualEffectView *backgroundView;
@property(nonatomic, assign) void (*clickCallback)();
- (void)setCornerRadius:(CGFloat)radius;
- (void)updateWithMessage:(NSString *)message icon:(NSImage *)icon;
@end

static ExplorerHintWindow *gExplorerHintWindow = nil;
static id gAppActivationObserver = nil;
static void (*gFinderActivationCallback)(int, int, int, int) = NULL;

@implementation ExplorerHintWindow

- (instancetype)initWithContentRect:(NSRect)contentRect styleMask:(NSWindowStyleMask)style backing:(NSBackingStoreType)backingStoreType defer:(BOOL)flag {
  self = [super initWithContentRect:contentRect styleMask:style backing:backingStoreType defer:flag];
  if (self) {
    [self setBackgroundColor:[NSColor clearColor]];
    [self setOpaque:NO];
    [self setHasShadow:YES];

    NSVisualEffectView *backgroundView = [[NSVisualEffectView alloc] initWithFrame:self.contentView.bounds];
    if (@available(macOS 10.14, *)) {
      backgroundView.material = NSVisualEffectMaterialHUDWindow;
    } else {
      backgroundView.material = NSVisualEffectMaterialPopover;
    }
    backgroundView.state = NSVisualEffectStateActive;
    backgroundView.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    if (@available(macOS 10.14, *)) {
      backgroundView.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    }
    [self.contentView addSubview:backgroundView positioned:NSWindowBelow relativeTo:nil];
    self.backgroundView = backgroundView;

    NSImageView *iconView = [[NSImageView alloc] initWithFrame:NSMakeRect(kPadding, (kWindowHeight - kIconSize) / 2, kIconSize, kIconSize)];
    iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
    iconView.hidden = YES;
    [self.contentView addSubview:iconView];
    self.iconView = iconView;

    NSTextField *messageLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(kPadding + kIconSize + 12, 0, kWindowWidth - kIconSize - 12 - kPadding * 2, kWindowHeight)];
    messageLabel.editable = NO;
    messageLabel.selectable = NO;
    messageLabel.bordered = NO;
    messageLabel.drawsBackground = NO;
    messageLabel.font = [NSFont systemFontOfSize:14];
    messageLabel.textColor = [NSColor whiteColor];
    messageLabel.alignment = NSTextAlignmentLeft;
    messageLabel.lineBreakMode = NSLineBreakByTruncatingTail;
    if (@available(macOS 10.14, *)) {
      messageLabel.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    }
    [self.contentView addSubview:messageLabel];
    self.messageLabel = messageLabel;

    NSTrackingArea *trackingArea = [[NSTrackingArea alloc] initWithRect:[self.contentView bounds] options:(NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect) owner:self userInfo:nil];
    [self.contentView addTrackingArea:trackingArea];

    NSClickGestureRecognizer *clickRecognizer = [[NSClickGestureRecognizer alloc] initWithTarget:self action:@selector(handleClick:)];
    [self.contentView addGestureRecognizer:clickRecognizer];
  }
  return self;
}

- (void)setCornerRadius:(CGFloat)radius {
  self.contentView.wantsLayer = YES;
  self.contentView.layer.cornerRadius = radius;
  self.contentView.layer.masksToBounds = YES;
}

- (BOOL)canBecomeKeyWindow {
  return NO;
}

- (void)close {
  [self.closeTimer invalidate];
  if (gExplorerHintWindow == self) {
    gExplorerHintWindow = nil;
  }
  [super close];
}

- (void)handleClick:(NSClickGestureRecognizer *)recognizer {
  if (self.clickCallback) {
    self.clickCallback();
  } else {
    overlayClickCallbackCGO();
  }
  [self close];
}

- (void)scheduleCloseTimer {
  self.closeTimer = [NSTimer scheduledTimerWithTimeInterval:kAutoCloseSeconds target:self selector:@selector(closeAfterDelay) userInfo:nil repeats:NO];
}

- (void)closeAfterDelay {
  [NSAnimationContext
      runAnimationGroup:^(NSAnimationContext *context) {
        context.duration = 0.3;
        self.animator.alphaValue = 0.0;
      }
      completionHandler:^{
        [self close];
      }];
}

- (void)updateWithMessage:(NSString *)message icon:(NSImage *)icon {
  if (message && [message length] > 0) {
    self.messageLabel.stringValue = message;
  }

  if (icon) {
    self.iconView.image = icon;
    self.iconView.hidden = NO;
  } else {
    self.iconView.hidden = YES;
  }

  self.backgroundView.frame = self.contentView.bounds;
}

@end

static NSImage* CreateNSImageFromData(const unsigned char* data, int len) {
  if (data == NULL || len == 0) {
    return nil;
  }
  NSData *nsData = [NSData dataWithBytes:data length:len];
  if (!nsData) {
    return nil;
  }
  NSImage *image = [[NSImage alloc] initWithData:nsData];
  return image;
}

void showExplorerHint(int x, int y, int width, int height, const char* message, const unsigned char* iconData, int iconLen, void (*callback)()) {
  if (message == NULL) {
    return;
  }

  @autoreleasepool {
    NSApplication *application = [NSApplication sharedApplication];
    [application setActivationPolicy:NSApplicationActivationPolicyAccessory];

    NSString *messageString = [NSString stringWithUTF8String:message];
    if (messageString == nil) {
      messageString = @"";
    }

    NSImage *iconImage = CreateNSImageFromData(iconData, iconLen);

    CGFloat windowX = x + (width - kWindowWidth) / 2;
    CGFloat windowY = y + height - kWindowHeight - 10;
    NSRect frame = NSMakeRect(windowX, windowY, kWindowWidth, kWindowHeight);

    ExplorerHintWindow *window = gExplorerHintWindow;
    BOOL needsShow = NO;
    if (!window || !window.isVisible) {
      window = [[ExplorerHintWindow alloc] initWithContentRect:frame styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];
      [window setCornerRadius:12.0];
      [window setLevel:NSFloatingWindowLevel];
      [window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorTransient];
      window.alphaValue = 0.0;
      window.clickCallback = callback;
      gExplorerHintWindow = window;
      needsShow = YES;
    }

    [window updateWithMessage:messageString icon:iconImage];
    [window setFrame:frame display:YES];

    if (needsShow) {
      [window orderFront:nil];
      [NSAnimationContext
          runAnimationGroup:^(NSAnimationContext *context) {
            context.duration = 0.3;
            window.animator.alphaValue = 1.0;
          }
          completionHandler:^{
            [window scheduleCloseTimer];
          }];
    } else {
      window.alphaValue = 1.0;
      [window.closeTimer invalidate];
      [window scheduleCloseTimer];
      [window orderFront:nil];
    }
  }
}

void hideExplorerHint() {
  @autoreleasepool {
    if (gExplorerHintWindow) {
      [gExplorerHintWindow close];
      gExplorerHintWindow = nil;
    }
  }
}

static void handleAppActivation(NSNotification *notification) {
  @autoreleasepool {
    NSRunningApplication *app = [[notification userInfo] objectForKey:NSWorkspaceApplicationKey];
    if (!app) {
      return;
    }

    NSString *bundleIdentifier = [app bundleIdentifier];
    if (!bundleIdentifier || ![bundleIdentifier isEqualToString:@"com.apple.finder"]) {
      return;
    }

    char* path = getActiveFinderWindowPath();
    if (!path || strlen(path) == 0) {
      if (path) free(path);
      return;
    }
    free(path);

    if (!AXIsProcessTrusted()) {
      return;
    }

    pid_t pid = [app processIdentifier];
    AXUIElementRef appElement = AXUIElementCreateApplication(pid);
    if (!appElement) {
      return;
    }

    AXUIElementRef window = NULL;
    AXError windowErr = AXUIElementCopyAttributeValue(appElement, kAXFocusedWindowAttribute, (CFTypeRef *)&window);
    if (windowErr != kAXErrorSuccess || !window) {
      CFRelease(appElement);
      return;
    }

    CFTypeRef positionValue = NULL;
    CFTypeRef sizeValue = NULL;
    CGPoint position = CGPointZero;
    CGSize size = CGSizeZero;

    if (AXUIElementCopyAttributeValue(window, kAXPositionAttribute, &positionValue) == kAXErrorSuccess && positionValue) {
      AXValueGetValue(positionValue, kAXValueCGPointType, &position);
      CFRelease(positionValue);
    }

    if (AXUIElementCopyAttributeValue(window, kAXSizeAttribute, &sizeValue) == kAXErrorSuccess && sizeValue) {
      AXValueGetValue(sizeValue, kAXValueCGSizeType, &size);
      CFRelease(sizeValue);
    }

    CFRelease(window);
    CFRelease(appElement);

    NSScreen *mainScreen = [NSScreen mainScreen];
    if (!mainScreen) {
      return;
    }
    CGFloat screenHeight = NSMaxY([mainScreen frame]);
    int cocoaY = (int)(screenHeight - position.y - size.height);

    if (gFinderActivationCallback) {
      gFinderActivationCallback((int)position.x, cocoaY, (int)size.width, (int)size.height);
    } else {
      finderActivationCallbackCGO((int)position.x, cocoaY, (int)size.width, (int)size.height);
    }
  }
}

void startAppActivationListener(void (*callback)(int x, int y, int width, int height)) {
  @autoreleasepool {
    gFinderActivationCallback = callback;

    if (gAppActivationObserver) {
      [[NSWorkspace sharedWorkspace].notificationCenter removeObserver:gAppActivationObserver];
      gAppActivationObserver = nil;
    }

    gAppActivationObserver = [[NSWorkspace sharedWorkspace].notificationCenter
        addObserverForName:NSWorkspaceDidActivateApplicationNotification
                    object:nil
                     queue:[NSOperationQueue mainQueue]
                usingBlock:^(NSNotification *notification) {
                  handleAppActivation(notification);
                }];
  }
}

void stopAppActivationListener() {
  @autoreleasepool {
    if (gAppActivationObserver) {
      [[NSWorkspace sharedWorkspace].notificationCenter removeObserver:gAppActivationObserver];
      gAppActivationObserver = nil;
    }
    gFinderActivationCallback = NULL;
  }
}
