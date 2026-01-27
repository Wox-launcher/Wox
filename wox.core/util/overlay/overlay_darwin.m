#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <CoreGraphics/CoreGraphics.h>
#import <CoreVideo/CoreVideo.h>
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
@end

@interface OverlayWindow ()
@property(nonatomic, strong) NSTrackingArea *trackingArea;
@property(nonatomic, assign) BOOL isMouseInside;
@property(nonatomic, assign) BOOL isAutoClosePending;
@property(nonatomic, assign) NSPoint initialLocation;
@property(nonatomic, assign) BOOL isMovable;
@property(nonatomic, assign) BOOL isDragging;
@property(nonatomic, assign) NSPoint initialWindowOrigin;
// AXObserver for tracking window movement
@property(nonatomic, assign) AXObserverRef axObserver;
@property(nonatomic, assign) AXUIElementRef trackedWindow;
@property(nonatomic, assign) OverlayOptions currentOpts;
// Timer for delayed show after window stops moving
@property(nonatomic, strong) NSTimer *showDelayTimer;
// Target window number for z-order management
@property(nonatomic, assign) CGWindowID stickyWindowNumber;
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

// -----------------------------------------------------------------------------
// Passthrough TextView - lets mouse events pass through to window for dragging
// -----------------------------------------------------------------------------
@interface PassthroughTextView : NSTextView
@end

@implementation PassthroughTextView
- (NSView *)hitTest:(NSPoint)point {
    return nil; // Let mouse events pass through to window
}
@end

// -----------------------------------------------------------------------------
// Passthrough ImageView - lets mouse events pass through to window for dragging
// -----------------------------------------------------------------------------
@interface PassthroughImageView : NSImageView
@end

@implementation PassthroughImageView
- (NSView *)hitTest:(NSPoint)point {
    return nil; // Let mouse events pass through to window
}
@end

// -----------------------------------------------------------------------------
// Passthrough VisualEffectView - lets mouse events pass through to window
// -----------------------------------------------------------------------------
@interface PassthroughVisualEffectView : NSVisualEffectView
@end

@implementation PassthroughVisualEffectView
- (NSView *)hitTest:(NSPoint)point {
    return nil; // Let mouse events pass through to window
}
@end

// -----------------------------------------------------------------------------
// Draggable Content View - accepts first mouse to enable immediate dragging
// -----------------------------------------------------------------------------
@interface DraggableContentView : NSView
@end

@implementation DraggableContentView
- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES; // Accept click even when window is not key
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
        // Allow first click to trigger mouseDown instead of just activating window
        [self setBecomesKeyOnlyIfNeeded:NO];

        // Set custom content view that accepts first mouse
        DraggableContentView *contentView = [[DraggableContentView alloc] initWithFrame:contentRect];
        [self setContentView:contentView];
        
        // Background - use PassthroughVisualEffectView for drag support
        PassthroughVisualEffectView *bg = [[PassthroughVisualEffectView alloc] initWithFrame:contentView.bounds];
        bg.material = NSVisualEffectMaterialHUDWindow;
        bg.state = NSVisualEffectStateActive;
        bg.blendingMode = NSVisualEffectBlendingModeBehindWindow;
        if (@available(macOS 10.14, *)) {
            bg.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
        }
        [self.contentView addSubview:bg positioned:NSWindowBelow relativeTo:nil];
        self.backgroundView = bg;

        // Icon - use PassthroughImageView for drag support
        self.iconView = [[PassthroughImageView alloc] initWithFrame:NSMakeRect(12, 0, kIconSize, kIconSize)]; 
        self.iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
        self.iconView.hidden = YES;
        [self.contentView addSubview:self.iconView];

        // Message (TextView for multiline) - use PassthroughTextView for drag support
        self.messageView = [[PassthroughTextView alloc] initWithFrame:NSZeroRect];
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
    self.initialLocation = [NSEvent mouseLocation];
    self.initialWindowOrigin = self.frame.origin;

    if (self.isMovable) {
        self.isDragging = YES;
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
    // Don't auto-close while dragging
    if (self.isAutoClosePending && !self.isDragging) {
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
    [self stopTrackingWindow];
    [self close];
    if (gOverlayWindows && self.name) {
        [gOverlayWindows removeObjectForKey:self.name];
    }
}

- (void)stopTrackingWindow {
    // Cancel show delay timer
    [self.showDelayTimer invalidate];
    self.showDelayTimer = nil;
    
    if (self.axObserver) {
        CFRunLoopRemoveSource(CFRunLoopGetMain(), 
                              AXObserverGetRunLoopSource(self.axObserver), 
                              kCFRunLoopDefaultMode);
        CFRelease(self.axObserver);
        self.axObserver = NULL;
    }
    if (self.trackedWindow) {
        CFRelease(self.trackedWindow);
        self.trackedWindow = NULL;
    }
}

// Get the focused window number for a given PID
- (CGWindowID)getWindowNumberForPid:(pid_t)pid {
    CGWindowID result = 0;
    
    // Get all windows
    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (!windowList) return 0;
    
    // Find the frontmost window for this PID
    for (CFIndex i = 0; i < CFArrayGetCount(windowList); i++) {
        NSDictionary *windowInfo = (NSDictionary *)CFArrayGetValueAtIndex(windowList, i);
        NSNumber *windowPid = windowInfo[(id)kCGWindowOwnerPID];
        NSNumber *windowNumber = windowInfo[(id)kCGWindowNumber];
        NSNumber *windowLayer = windowInfo[(id)kCGWindowLayer];
        
        // Only consider normal layer windows (layer 0)
        if ([windowPid intValue] == pid && [windowLayer intValue] == 0) {
            result = [windowNumber unsignedIntValue];
            break; // First one found is typically the frontmost
        }
    }
    
    CFRelease(windowList);
    return result;
}

// Order overlay window relative to sticky window
- (void)orderRelativeToStickyWindow {
    if (self.stickyWindowNumber > 0) {
        // Use normal window level and order above the target window
        [self setLevel:NSNormalWindowLevel];
        [self orderWindow:NSWindowAbove relativeTo:self.stickyWindowNumber];
    } else {
        // Fallback to floating level
        [self setLevel:NSFloatingWindowLevel];
        [self orderFront:nil];
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
    self.isMovable = opts.movable;
    self.isDragging = NO;
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
        
        // Get window number for z-order management
        self.stickyWindowNumber = [self getWindowNumberForPid:pid];
        
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
            
            // Find the screen containing the window center
            NSPoint windowCenter = NSMakePoint(pos.x + size.width / 2, pos.y + size.height / 2);
            NSScreen *targetScreen = nil;
            for (NSScreen *screen in [NSScreen screens]) {
                // Convert screen frame from Cocoa to CG coordinates for comparison
                NSRect screenFrame = screen.frame;
                CGFloat mainScreenH = [NSScreen mainScreen].frame.size.height;
                CGRect cgScreenFrame = CGRectMake(screenFrame.origin.x, 
                                                   mainScreenH - screenFrame.origin.y - screenFrame.size.height, 
                                                   screenFrame.size.width, 
                                                   screenFrame.size.height);
                if (CGRectContainsPoint(cgScreenFrame, windowCenter)) {
                    targetScreen = screen;
                    break;
                }
            }
            if (!targetScreen) targetScreen = [NSScreen mainScreen];
            
            // Convert CG coordinates to Cocoa coordinates using main screen height
            CGFloat mainScreenH = [NSScreen mainScreen].frame.size.height;
            CGFloat cocoaY = mainScreenH - pos.y - size.height;
            targetRect = CGRectMake(pos.x, cocoaY, size.width, size.height);
            CFRelease(posVal); CFRelease(sizeVal); CFRelease(frontWindow);
        } else {
             targetRect = [NSScreen mainScreen].frame;
        }
        CFRelease(app);
    } else {
        self.stickyWindowNumber = 0;
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
    
    // 5. Store options and setup window tracking
    self.currentOpts = opts;
    if (opts.stickyWindowPid > 0) {
        [self startTrackingWindowWithPid:opts.stickyWindowPid];
    } else {
        [self stopTrackingWindow];
    }
}

// AXObserver callback - called when tracked window moves or resizes
static void axObserverCallback(AXObserverRef observer, AXUIElementRef element, CFStringRef notification, void *refcon) {
    OverlayWindow *win = (__bridge OverlayWindow *)refcon;
    // Hide immediately when window is being moved
    [win handleTrackedWindowMoved];
}

- (void)handleTrackedWindowMoved {
    // Cancel any pending show timer
    [self.showDelayTimer invalidate];
    self.showDelayTimer = nil;
    
    // Hide the overlay immediately
    self.alphaValue = 0;
    
    // Schedule delayed show after window stops moving (500ms delay)
    self.showDelayTimer = [NSTimer scheduledTimerWithTimeInterval:0.5
                                                           target:self
                                                         selector:@selector(showAfterWindowStopped)
                                                         userInfo:nil
                                                          repeats:NO];
}

- (void)showAfterWindowStopped {
    self.showDelayTimer = nil;
    
    // Update stickyWindowNumber for z-order
    if (self.currentOpts.stickyWindowPid > 0) {
        self.stickyWindowNumber = [self getWindowNumberForPid:self.currentOpts.stickyWindowPid];
    }
    
    // Update position and show
    [self updatePositionFromTrackedWindow];
    [self orderRelativeToStickyWindow];
    self.alphaValue = 1.0;
}

- (void)startTrackingWindowWithPid:(pid_t)pid {
    // Stop any existing tracking first
    [self stopTrackingWindow];
    
    // Create AXUIElement for the application
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (!app) return;
    
    // Get the focused window
    AXUIElementRef frontWindow = NULL;
    AXError err = AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, (CFTypeRef *)&frontWindow);
    if (err != kAXErrorSuccess || !frontWindow) {
        CFRelease(app);
        return;
    }
    
    // Store the tracked window
    self.trackedWindow = frontWindow;
    
    // Create AXObserver
    AXObserverRef observer = NULL;
    err = AXObserverCreate(pid, axObserverCallback, &observer);
    if (err != kAXErrorSuccess || !observer) {
        CFRelease(app);
        CFRelease(frontWindow);
        self.trackedWindow = NULL;
        return;
    }
    
    self.axObserver = observer;
    
    // Add notifications for window movement and resize
    AXObserverAddNotification(observer, frontWindow, kAXMovedNotification, (__bridge void *)self);
    AXObserverAddNotification(observer, frontWindow, kAXResizedNotification, (__bridge void *)self);
    
    // Add observer to run loop
    CFRunLoopAddSource(CFRunLoopGetMain(), AXObserverGetRunLoopSource(observer), kCFRunLoopDefaultMode);
    
    CFRelease(app);
}

- (void)updatePositionFromTrackedWindow {
    if (self.currentOpts.stickyWindowPid <= 0) return;
    
    // Always get the current focused window (not the cached one)
    AXUIElementRef app = AXUIElementCreateApplication(self.currentOpts.stickyWindowPid);
    if (!app) return;
    
    AXUIElementRef frontWindow = NULL;
    AXError err = AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, (CFTypeRef *)&frontWindow);
    if (err != kAXErrorSuccess || !frontWindow) {
        CFRelease(app);
        return;
    }
    
    // Get current window position and size
    CFTypeRef posVal = NULL, sizeVal = NULL;
    CGPoint pos; CGSize size;
    
    AXError err1 = AXUIElementCopyAttributeValue(frontWindow, kAXPositionAttribute, &posVal);
    AXError err2 = AXUIElementCopyAttributeValue(frontWindow, kAXSizeAttribute, &sizeVal);
    
    CFRelease(frontWindow);
    CFRelease(app);
    
    if (err1 != kAXErrorSuccess || err2 != kAXErrorSuccess || !posVal || !sizeVal) {
        if (posVal) CFRelease(posVal);
        if (sizeVal) CFRelease(sizeVal);
        return;
    }
    
    AXValueGetValue(posVal, kAXValueCGPointType, &pos);
    AXValueGetValue(sizeVal, kAXValueCGSizeType, &size);
    
    CFRelease(posVal);
    CFRelease(sizeVal);
    
    // Convert AX coordinates (top-left origin, primary screen) to Cocoa coordinates (bottom-left origin, primary screen)
    // IMPORTANT: specific screen height must be the PRIMARY screen's height, not [NSScreen mainScreen] which changes based on focus
    NSScreen *primaryScreen = [[NSScreen screens] firstObject];
    CGFloat screenH = primaryScreen.frame.size.height;
    CGFloat cocoaY = screenH - pos.y - size.height;
    
    NSLog(@"[OverlayDebug] PID: %d, AXPos:(%f, %f), AXSize:(%f, %f), PrimaryScreenH:%f, CocoaY:%f", 
          self.currentOpts.stickyWindowPid, pos.x, pos.y, size.width, size.height, screenH, cocoaY);
    
    CGRect targetRect = CGRectMake(pos.x, cocoaY, size.width, size.height);
    
    // Calculate new position based on anchor
    OverlayOptions opts = self.currentOpts;
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
    
    CGFloat ow = self.frame.size.width;
    CGFloat oh = self.frame.size.height;
    CGFloat ox = 0, oy = 0;
    
    if (col == 0) ox = 0;
    else if (col == 1) ox = -ow/2;
    else ox = -ow;
    
    if (row == 0) oy = -oh;
    else if (row == 1) oy = -oh/2;
    else oy = 0;
    
    CGFloat finalX = px + ox + opts.offsetX;
    CGFloat finalY = py + oy + opts.offsetY;
    
    [self setFrameOrigin:NSMakePoint(finalX, finalY)];
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
        [win orderRelativeToStickyWindow];
        win.alphaValue = 1.0;
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
