#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>

@interface NotificationWindow : NSPanel
@property(nonatomic, strong) NSTimer *closeTimer;
@property(nonatomic, strong) NSButton *closeButton;
@property(nonatomic, assign) NSTimeInterval remainingTime;
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
@end

void showNotification(const char *message) {
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
    [[window contentView] addSubview:messageField];

    NSButton *closeButton = [[NSButton alloc] initWithFrame:NSMakeRect(windowWidth - horizontalPadding - closeButtonSize, (windowHeight - closeButtonSize) / 2, closeButtonSize, closeButtonSize)];
    [closeButton setBezelStyle:NSBezelStyleCircular];
    [closeButton setButtonType:NSButtonTypeMomentaryLight];
    [closeButton setTitle:@"×"];
    [closeButton setTarget:window];
    [closeButton setAction:@selector(close)];
    [closeButton setHidden:YES];
    [[window contentView] addSubview:closeButton];
    window.closeButton = closeButton;

    NSTrackingArea *trackingArea = [[NSTrackingArea alloc] initWithRect:[window.contentView bounds] options:(NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways) owner:window userInfo:nil];
    [window.contentView addTrackingArea:trackingArea];

    [window makeKeyAndOrderFront:nil];

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
