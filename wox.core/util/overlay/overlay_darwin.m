#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>

// -----------------------------------------------------------------------------
// Options Struct (Must match CGO / Go definition)
// -----------------------------------------------------------------------------
typedef struct {
    char* name;
    char* title;
    char* message;
    unsigned char* iconData;
    int iconLen;
    bool closable;
    int stickyWindowPid; // 0 = Screen, >0 = Window
    int anchor;          // 0-8: TL,TC,TR, LC,C,RC, BL,BC,BR
    int autoCloseSeconds;
    bool movable;
    float offsetX;
    float offsetY;
    float width;         // 0 = auto
    float height;        // 0 = auto
} OverlayOptions;

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------
static const CGFloat kDefaultWindowWidth = 400;
static const CGFloat kIconSize = 24;
static const CGFloat kCloseSize = 20;

extern void overlayClickCallbackCGO(char* name);

// -----------------------------------------------------------------------------
// Overlay Window
// -----------------------------------------------------------------------------
@interface OverlayWindow : NSPanel
@property(nonatomic, strong) NSString *name; // Store the ID
@property(nonatomic, strong) NSTimer *closeTimer;
@property(nonatomic, strong) NSImageView *iconView;
@property(nonatomic, strong) NSTextField *messageLabel;
// Simplified text view for now, or use full NSTextView from notifier if needed for multiline.
// Plan said "use NotificationWindow's robust text logic". So I should use NSTextView.
@property(nonatomic, strong) NSTextView *messageView;
@property(nonatomic, strong) NSButton *closeButton;
@property(nonatomic, strong) NSVisualEffectView *backgroundView;
@property(nonatomic, assign) int stickyPid;
@property(nonatomic, assign) NSWindow *trackedWindow; // If we find the NSWindow object (rare for other apps)
// For external apps, we might need a timer to poll position if we really want "sticky" without addChildWindow (which requires same process usually, unless using specialized API).
// Actually `addChildWindow` works for windows in same app. For external, we can't easily "attach".
// Plan Scheme A: "Overlay tracks target window".
// Realistically, polling is needed for external windows. Or just update on Show().
// Previous implementation `monitor` was sending updates.
// Here we just set position ONCE when Show() is called. The `explorer` plugin's monitor will call Show() repeatedly or on move?
// User said: "monitor_darwin.m Listen for Finder activation... call overlay.Show".
// If Finder moves, does overlay move?
// If we implemented the monitor to track `kAXMovedNotification` etc, we could call Show() again.
// For now, `Show` sets the position.
@end

@interface OverlayWindow ()
@property(nonatomic, strong) NSTrackingArea *trackingArea;
@property(nonatomic, assign) BOOL isMouseInside;
@property(nonatomic, assign) BOOL isAutoClosePending;
@property(nonatomic, assign) NSPoint initialLocation;
@property(nonatomic, assign) BOOL isMovable;
@property(nonatomic, assign) BOOL isDragging;
@property(nonatomic, assign) NSPoint initialWindowOrigin;
@end

static NSMutableDictionary<NSString*, OverlayWindow*> *gOverlayWindows = nil;

// -----------------------------------------------------------------------------
// Helper Classes
// -----------------------------------------------------------------------------
@interface HandCursorButton : NSButton
@end

@implementation HandCursorButton
- (void)updateTrackingAreas {
  [super updateTrackingAreas];
  for (NSTrackingArea *area in self.trackingAreas) {
    [self removeTrackingArea:area];
  }
  NSTrackingArea *area = [[NSTrackingArea alloc] initWithRect:self.bounds
                                                      options:NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect | NSTrackingCursorUpdate
                                                        owner:self
                                                     userInfo:nil];
  [self addTrackingArea:area];
}

- (void)mouseEntered:(NSEvent *)event {
  [[NSCursor pointingHandCursor] set];
}

- (void)mouseExited:(NSEvent *)event {
  [[NSCursor arrowCursor] set];
}

- (void)cursorUpdate:(NSEvent *)event {
  [[NSCursor pointingHandCursor] set];
}
@end

@implementation OverlayWindow

- (instancetype)initWithContentRect:(NSRect)contentRect styleMask:(NSWindowStyleMask)style backing:(NSBackingStoreType)backingStoreType defer:(BOOL)flag {
    self = [super initWithContentRect:contentRect styleMask:style backing:backingStoreType defer:flag];
    if (self) {
        [self setBackgroundColor:[NSColor clearColor]];
        // ... (Keep existing setup)
        [self setOpaque:NO];
        [self setHasShadow:YES];
        [self setLevel:NSFloatingWindowLevel];
        [self setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorTransient];

        // Background
        NSVisualEffectView *bg = [[NSVisualEffectView alloc] initWithFrame:self.contentView.bounds];
        bg.material = NSVisualEffectMaterialHUDWindow;
        bg.state = NSVisualEffectStateActive;
        bg.blendingMode = NSVisualEffectBlendingModeBehindWindow;
        if (@available(macOS 10.14, *)) {
            bg.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
        }
        [self.contentView addSubview:bg positioned:NSWindowBelow relativeTo:nil];
        self.backgroundView = bg;

        // Icon
        self.iconView = [[NSImageView alloc] initWithFrame:NSMakeRect(12, 0, kIconSize, kIconSize)]; 
        self.iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
        self.iconView.hidden = YES;
        [self.contentView addSubview:self.iconView];

        // Message (TextView for multiline)
        self.messageView = [[NSTextView alloc] initWithFrame:NSZeroRect];
        self.messageView.editable = NO;
        self.messageView.selectable = NO;
        self.messageView.drawsBackground = NO;
        if (@available(macOS 10.14, *)) {
            self.messageView.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
        }
        [self.contentView addSubview:self.messageView];

        // Close Button (HandCursorButton)
        self.closeButton = [[HandCursorButton alloc] initWithFrame:NSMakeRect(0, 0, kCloseSize, kCloseSize)];
        [self.closeButton setBezelStyle:NSBezelStyleRegularSquare];
        [self.closeButton setButtonType:NSButtonTypeMomentaryLight];
        [self.closeButton setTitle:@"×"];
        [self.closeButton setFont:[NSFont systemFontOfSize:16 weight:NSFontWeightBold]];
        [self.closeButton setTarget:self];
        [self.closeButton setAction:@selector(onClose)];
        [self.closeButton setHidden:NO];
        [self.closeButton setBordered:NO];
        [self.closeButton setWantsLayer:YES];
        self.closeButton.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.3].CGColor;
        self.closeButton.layer.cornerRadius = kCloseSize / 2;
        
        NSMutableAttributedString *attributedTitle = [[NSMutableAttributedString alloc] initWithString:@"×"];
        [attributedTitle addAttribute:NSForegroundColorAttributeName value:[NSColor whiteColor] range:NSMakeRange(0, attributedTitle.length)];
        [self.closeButton setAttributedTitle:attributedTitle];
        
        [self.contentView addSubview:self.closeButton];

        // Tracking Area setup
        [self setupTrackingArea];
    }
    return self;
}

- (void)mouseDown:(NSEvent *)event {
    if (self.isMovable) {
        self.isDragging = YES;
        self.initialLocation = [NSEvent mouseLocation];
        self.initialWindowOrigin = self.frame.origin;
    }
}

- (void)mouseDragged:(NSEvent *)event {
    if (!self.isDragging) return;
    
    NSPoint currentLocation = [NSEvent mouseLocation];
    CGFloat dx = currentLocation.x - self.initialLocation.x;
    CGFloat dy = currentLocation.y - self.initialLocation.y;
    
    NSPoint newOrigin = NSMakePoint(self.initialWindowOrigin.x + dx,
                                    self.initialWindowOrigin.y + dy);
    [self setFrameOrigin:newOrigin];
}

- (void)mouseUp:(NSEvent *)event {
    self.isDragging = NO;
    
    NSPoint currentLocation = [NSEvent mouseLocation];
    CGFloat dx = currentLocation.x - self.initialLocation.x;
    CGFloat dy = currentLocation.y - self.initialLocation.y;
    
    // If movement is small, treat as click
    if (dx*dx + dy*dy < 25.0) {
        [self onClick];
    }
    
    // If auto-close passed while dragging, and we are not inside (or maybe we should just re-check pending state)
    // Actually, if we release drag, and pending is YES, we should check if we are currently inside.
    // But `isMouseInside` state is maintained by Enter/Exit events.
    if (self.isAutoClosePending && !self.isMouseInside) {
        [self onClose];
    }
}

- (void)setupTrackingArea {
    if (self.trackingArea) {
        [self.contentView removeTrackingArea:self.trackingArea];
    }
    
    NSTrackingAreaOptions options = NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect;
    self.trackingArea = [[NSTrackingArea alloc] initWithRect:NSZeroRect // Ignored by InVisibleRect
                                                     options:options
                                                       owner:self
                                                    userInfo:nil];
    [self.contentView addTrackingArea:self.trackingArea];
}

- (void)mouseEntered:(NSEvent *)event {
    self.isMouseInside = YES;
}

- (void)mouseExited:(NSEvent *)event {
    self.isMouseInside = NO;
    if (self.isAutoClosePending) {
        [self onClose];
    }
}

// ... (Timer methods remain same) ...

- (void)startAutoCloseTimer:(NSTimeInterval)seconds {
    [self stopAutoCloseTimer];
    if (seconds > 0) {
        self.closeTimer = [NSTimer scheduledTimerWithTimeInterval:seconds
                                                           target:self
                                                         selector:@selector(onAutoCloseTimerFired:)
                                                         userInfo:nil
                                                          repeats:NO];
    }
}

- (void)stopAutoCloseTimer {
    if (self.closeTimer) {
        [self.closeTimer invalidate];
        self.closeTimer = nil;
    }
    self.isAutoClosePending = NO;
}

- (void)onAutoCloseTimerFired:(NSTimer*)timer {
    if (self.isMouseInside || self.isDragging) {
        self.isAutoClosePending = YES;
    } else {
        [self onClose];
    }
}

- (void)setCornerRadius:(CGFloat)radius {
    self.contentView.wantsLayer = YES;
    self.contentView.layer.cornerRadius = radius;
    self.contentView.layer.masksToBounds = YES;
}

- (void)onClose {
    [self stopAutoCloseTimer];
    [self close];
    if (gOverlayWindows && self.name) {
        [gOverlayWindows removeObjectForKey:self.name];
    }
}

- (void)onClick {
    if (self.name) {
       overlayClickCallbackCGO((char*)[self.name UTF8String]);
    }
}

- (BOOL)canBecomeKeyWindow {
    return YES; // Allow interaction
}

- (void)updateLayoutWithOptions:(OverlayOptions)opts {
    // 0. Reset State
    self.isMouseInside = NO; 
    self.isMovable = opts.movable;
    [self stopAutoCloseTimer];

    // 1. Content Update
    NSString *msg = opts.message ? [NSString stringWithUTF8String:opts.message] : @"";
    NSImage *icon = nil;
    if (opts.iconData && opts.iconLen > 0) {
        NSData *data = [NSData dataWithBytes:opts.iconData length:opts.iconLen];
        icon = [[NSImage alloc] initWithData:data];
    }

    self.iconView.image = icon;
    self.iconView.hidden = (icon == nil);
    
    self.closeButton.hidden = !opts.closable;

    // 2. Measure & Layout
    CGFloat windowWidth = (opts.width > 0) ? opts.width : kDefaultWindowWidth;
    // Paddings
    CGFloat padLeft = 12;
    CGFloat padRight = 12;
    CGFloat padTop = 10;
    CGFloat padBottom = 10;
    
    if (!self.iconView.hidden) padLeft += kIconSize + 8;
    if (!self.closeButton.hidden) padRight += kCloseSize + 4;

    CGFloat contentWidth = windowWidth - padLeft - padRight;
    
    // Setup TextView string
    NSDictionary *attrs = @{
        NSFontAttributeName: [NSFont systemFontOfSize:14],
        NSForegroundColorAttributeName: [NSColor whiteColor]
    };
    NSAttributedString *attrStr = [[NSAttributedString alloc] initWithString:msg attributes:attrs];
    [self.messageView.textStorage setAttributedString:attrStr];
    
    // Measure Height
    NSSize textSize = [self.messageView.layoutManager usedRectForTextContainer:self.messageView.textContainer].size; 
    NSTextContainer *tc = self.messageView.textContainer;
    tc.containerSize = NSMakeSize(contentWidth, CGFLOAT_MAX);
    tc.widthTracksTextView = NO;
    [self.messageView.layoutManager ensureLayoutForTextContainer:tc];
    textSize = [self.messageView.layoutManager usedRectForTextContainer:tc].size;

    CGFloat textHeight = textSize.height;
    CGFloat windowHeight = (opts.height > 0) ? opts.height : (textHeight + padTop + padBottom);
    if (windowHeight < 40) windowHeight = 40; // Min height

    // Update Frames
    CGFloat currentY = (windowHeight - textHeight) / 2; // Center Vertically
    if (currentY < padTop) currentY = padTop;

    self.messageView.frame = NSMakeRect(padLeft, currentY, contentWidth, textHeight);
    
    if (!self.iconView.hidden) {
        self.iconView.frame = NSMakeRect(12, (windowHeight - kIconSize)/2, kIconSize, kIconSize);
    }
    if (!self.closeButton.hidden) {
        self.closeButton.frame = NSMakeRect(windowWidth - kCloseSize - 6, (windowHeight - kCloseSize)/2, kCloseSize, kCloseSize);
    }

    // 3. Position Calculation (Anchor)
    CGRect targetRect;
    
    if (opts.stickyWindowPid > 0) {
        pid_t pid = (pid_t)opts.stickyWindowPid;
        AXUIElementRef app = AXUIElementCreateApplication(pid);
        AXUIElementRef frontWindow = NULL;
        AXError err = AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, (CFTypeRef *)&frontWindow);
        if (err == kAXErrorSuccess && frontWindow) {
            CFTypeRef posVal, sizeVal;
            CGPoint pos; CGSize size;
            AXUIElementCopyAttributeValue(frontWindow, kAXPositionAttribute, &posVal);
            AXUIElementCopyAttributeValue(frontWindow, kAXSizeAttribute, &sizeVal);
            AXValueGetValue(posVal, kAXValueCGPointType, &pos);
            AXValueGetValue(sizeVal, kAXValueCGSizeType, &size);
            NSScreen *mainScreen = [NSScreen mainScreen];
            CGFloat screenH = mainScreen.frame.size.height;
            CGFloat cocoaY = screenH - pos.y - size.height;
            targetRect = CGRectMake(pos.x, cocoaY, size.width, size.height);
            CFRelease(posVal); CFRelease(sizeVal); CFRelease(frontWindow);
        } else {
             targetRect = [NSScreen mainScreen].frame;
        }
        CFRelease(app);
    } else {
        targetRect = [NSScreen mainScreen].frame;
        targetRect = [NSScreen mainScreen].visibleFrame;
    }

    CGFloat ax = targetRect.origin.x;
    CGFloat ay = targetRect.origin.y;
    CGFloat aw = targetRect.size.width;
    CGFloat ah = targetRect.size.height;

    CGFloat px, py; 
    int col = opts.anchor % 3; 
    if (col == 0) px = ax;
    else if (col == 1) px = ax + aw / 2;
    else px = ax + aw;

    int row = opts.anchor / 3; 
    if (row == 0) py = ay + ah; 
    else if (row == 1) py = ay + ah / 2; 
    else py = ay; 

    CGFloat ox = 0;
    CGFloat oy = 0;
    CGFloat ow = windowWidth;
    CGFloat oh = windowHeight;

    if (col == 0) ox = 0;           
    else if (col == 1) ox = -ow/2;  
    else ox = -ow;                  

    if (row == 0) oy = -oh;         
    else if (row == 1) oy = -oh/2;  
    else oy = 0;                    

    CGFloat finalX = px + ox + opts.offsetX;
    CGFloat finalY = py + oy + opts.offsetY;

    [self setFrame:NSMakeRect(finalX, finalY, windowWidth, windowHeight) display:YES];
    self.backgroundView.frame = self.contentView.bounds;
    [self setCornerRadius:10.0];
    
    // 4. Auto Close (Timer)
    [self startAutoCloseTimer:(NSTimeInterval)opts.autoCloseSeconds];
}

@end

// -----------------------------------------------------------------------------
// C Exported Functions
// -----------------------------------------------------------------------------

void ShowOverlay(OverlayOptions opts) {
    @autoreleasepool {
        if (!gOverlayWindows) {
            gOverlayWindows = [[NSMutableDictionary alloc] init];
        }

        NSString *key = [NSString stringWithUTF8String:opts.name];
        OverlayWindow *win = [gOverlayWindows objectForKey:key];
        
        if (!win) {
            // Create new
            NSRect frame = NSZeroRect; // Will be set in updateLayout
            win = [[OverlayWindow alloc] initWithContentRect:frame 
                                                   styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel 
                                                     backing:NSBackingStoreBuffered 
                                                       defer:NO];
            win.name = key;
            [gOverlayWindows setObject:win forKey:key];
        }

        [win updateLayoutWithOptions:opts];
        [win orderFront:nil];
        win.alphaValue = 0;
        [NSAnimationContext runAnimationGroup:^(NSAnimationContext *ctx){
            ctx.duration = 0.2;
            win.animator.alphaValue = 1.0;
        } completionHandler:nil];
    }
}

void CloseOverlay(char* name) {
    @autoreleasepool {
        if (!gOverlayWindows) return;
        NSString *key = [NSString stringWithUTF8String:name];
        OverlayWindow *win = [gOverlayWindows objectForKey:key];
        if (win) {
            // Don't close if user is dragging the overlay
            if (win.isDragging) return;
            [win close];
            [gOverlayWindows removeObjectForKey:key];
        }
    }
}
