#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>

@interface NotificationWindow : NSPanel
@property(nonatomic, strong) NSTimer *closeTimer;
@property(nonatomic, strong) NSButton *closeButton;
@property(nonatomic, assign) NSTimeInterval remainingTime;
@property (nonatomic, assign) NSPoint initialLocation;
- (void)setCornerRadius:(CGFloat)radius;
@end

@implementation NotificationWindow

- (instancetype)initWithContentRect:(NSRect)contentRect styleMask:(NSWindowStyleMask)style backing:(NSBackingStoreType)backingStoreType defer:(BOOL)flag {
  self = [super initWithContentRect:contentRect styleMask:style backing:backingStoreType defer:flag];
  if (self) {
    [self setBackgroundColor:[NSColor clearColor]];
    [self setOpaque:NO];
    [self setHasShadow:YES];
  }
  return self;
}

- (void)setCornerRadius:(CGFloat)radius {
  self.contentView.wantsLayer = YES;
  self.contentView.layer.cornerRadius = radius;
  self.contentView.layer.masksToBounds = YES;
}

- (BOOL)canBecomeKeyWindow {
  return YES;
}
- (void)close {
  [self.closeTimer invalidate];
  [super close];
}

- (void)mouseEntered:(NSEvent *)event {
  [self.closeButton setHidden:NO];
  self.remainingTime = [self.closeTimer.fireDate timeIntervalSinceNow];
  [self.closeTimer invalidate];
}

- (void)mouseExited:(NSEvent *)event {
  [self.closeButton setHidden:YES];
  [self scheduleCloseTimer];
}

- (void)scheduleCloseTimer {
  self.closeTimer = [NSTimer scheduledTimerWithTimeInterval:self.remainingTime target:self selector:@selector(closeAfterDelay) userInfo:nil repeats:NO];
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

- (void)mouseDown:(NSEvent *)event {
    self.initialLocation = [event locationInWindow];
}

- (void)mouseDragged:(NSEvent *)event {
    NSPoint currentLocation = [self convertBaseToScreen:[event locationInWindow]];
    NSPoint newOrigin = NSMakePoint(currentLocation.x - self.initialLocation.x,
                                    currentLocation.y - self.initialLocation.y);
    [self setFrameOrigin:newOrigin];
}
@end

void showNotification(const char *message) {
  if (message == NULL) {
    NSLog(@"Warning: Null message passed to showNotification");
    return;
  }

  @autoreleasepool {
    NSApplication *application = [NSApplication sharedApplication];
    [application setActivationPolicy:NSApplicationActivationPolicyAccessory];

    NSScreen *screen = [NSScreen mainScreen];
    NSRect screenRect = [screen visibleFrame];

    CGFloat windowWidth = 380;
    CGFloat minWindowHeight = 20;
    CGFloat maxWindowHeight = 300;
    CGFloat verticalPadding = 10; 
    CGFloat closeButtonSize = 20;
    CGFloat horizontalPadding = closeButtonSize; 

    NSFont *messageFont = [NSFont systemFontOfSize:14];
    NSString *messageString = message ? [NSString stringWithUTF8String:message] : @"";
    if (messageString == nil) {
      messageString = @""; 
    }
    CGFloat messageWidth = windowWidth - (2 * horizontalPadding);
    CGFloat messageHeight = [messageString boundingRectWithSize:NSMakeSize(messageWidth, CGFLOAT_MAX) options:NSStringDrawingUsesLineFragmentOrigin attributes:@{NSFontAttributeName : messageFont}].size.height;

    CGFloat contentHeight = MAX(messageHeight, closeButtonSize) + (2 * verticalPadding);
    CGFloat windowHeight = MIN(MAX(minWindowHeight, contentHeight), maxWindowHeight);

    CGFloat yPosition = NSMinY(screenRect) + NSHeight(screenRect) * 0.2 - windowHeight / 2;
    NSRect frame = NSMakeRect(NSMidX(screenRect) - windowWidth / 2, yPosition, windowWidth, windowHeight);

    NotificationWindow *window = [[NotificationWindow alloc] initWithContentRect:frame styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];

    [window setCornerRadius:20.0];

    NSVisualEffectView *backgroundView = [[NSVisualEffectView alloc] initWithFrame:window.contentView.bounds];
    if (@available(macOS 10.14, *)) {
      backgroundView.material = NSVisualEffectMaterialHUDWindow;
    } else {
      backgroundView.material = NSVisualEffectMaterialDark;
    }
    backgroundView.state = NSVisualEffectStateActive;
    backgroundView.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    if (@available(macOS 10.14, *)) {
      backgroundView.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    }
    [window.contentView addSubview:backgroundView positioned:NSWindowBelow relativeTo:nil];

    NSView *leftSpacerView = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, horizontalPadding, windowHeight)];
    [[window contentView] addSubview:leftSpacerView];

    NSTextField *messageField = [[NSTextField alloc] initWithFrame:NSMakeRect(horizontalPadding, verticalPadding, messageWidth, messageHeight)];
    [messageField setBezeled:NO];
    [messageField setDrawsBackground:NO];
    [messageField setEditable:NO];
    [messageField setSelectable:NO];
    [messageField setStringValue:messageString];
    [messageField setFont:[NSFont systemFontOfSize:14]];
    [messageField setTextColor:[NSColor whiteColor]];
    if (@available(macOS 10.14, *)) {
     messageField.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    }
    [[window contentView] addSubview:messageField];

    NSButton *closeButton = [[NSButton alloc] initWithFrame:NSMakeRect(windowWidth - horizontalPadding - closeButtonSize, (windowHeight - closeButtonSize) / 2, closeButtonSize, closeButtonSize)];
    [closeButton setBezelStyle:NSBezelStyleRegularSquare];
    [closeButton setButtonType:NSButtonTypeMomentaryLight];
    [closeButton setTitle:@"×"];
    [closeButton setFont:[NSFont systemFontOfSize:16 weight:NSFontWeightBold]];
    [closeButton setTarget:window];
    [closeButton setAction:@selector(close)];
    [closeButton setHidden:YES];
    [closeButton setBordered:NO];
    [closeButton setWantsLayer:YES];
    closeButton.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.3].CGColor;
    closeButton.layer.cornerRadius = closeButtonSize / 2;
    NSMutableAttributedString *attributedTitle = [[NSMutableAttributedString alloc] initWithString:@"×"];
    [attributedTitle addAttribute:NSForegroundColorAttributeName value:[NSColor whiteColor] range:NSMakeRange(0, attributedTitle.length)];
    [closeButton setAttributedTitle:attributedTitle];
    [[window contentView] addSubview:closeButton];
    window.closeButton = closeButton;

    NSTrackingArea *trackingArea = [[NSTrackingArea alloc] initWithRect:[window.contentView bounds] options:(NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways) owner:window userInfo:nil];
    [window.contentView addTrackingArea:trackingArea];

    [window setLevel:NSFloatingWindowLevel];
    [window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorTransient];
    [window orderFront:nil]; 

    [NSAnimationContext
        runAnimationGroup:^(NSAnimationContext *context) {
          context.duration = 0.3;
          window.animator.alphaValue = 1.0;
        }
        completionHandler:^{
          window.remainingTime = 3.0;
          [window scheduleCloseTimer];
        }];
  }
}
