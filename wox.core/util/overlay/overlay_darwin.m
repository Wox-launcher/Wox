#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>
#include <math.h>
#include <stdlib.h>

// -----------------------------------------------------------------------------
// Options Struct (Must match CGO / Go definition)
// -----------------------------------------------------------------------------
typedef struct {
    char* name;
    bool transparent;
    bool hitTestIconOnly;
    bool closeOnEscape;
    bool takeFocus;
    bool nativeAttachment;
    int nativeAttachmentKind;
    void* nativeAttachmentHandle;
    float nativeAttachmentWidth;
    float nativeAttachmentHeight;
    bool topmost;
    bool absolutePosition;
    bool preservePosition;
    int stickyWindowPid; // 0 = Screen, >0 = Window
    int anchor;          // 0-8: TL,TC,TR, LC,C,RC, BL,BC,BR
    bool movable;
    bool resizable;
    float cornerRadius;
    float aspectRatio;
    float offsetX;
    float offsetY;
    float width;         // 0 = auto
    float minWidth;      // 0 = platform default minimum width
    float maxWidth;      // 0 = no cap for auto width
    float height;        // 0 = auto
    float maxHeight;     // 0 = no cap for auto height
} OverlayOptions;

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------
static const CGFloat kDefaultWindowWidth = 400;
static const CGFloat kStickyPredictiveCorrectionThreshold = 48;
static const CGFloat kResizeGripSize = 10;
static const CGFloat kResizeMinSize = 64;
static const int kNativeAttachmentKindView = 1;

typedef NS_OPTIONS(NSUInteger, OverlayResizeEdges) {
    OverlayResizeEdgeNone = 0,
    OverlayResizeEdgeLeft = 1 << 0,
    OverlayResizeEdgeRight = 1 << 1,
    OverlayResizeEdgeBottom = 1 << 2,
    OverlayResizeEdgeTop = 1 << 3,
};

extern bool overlayClickCallbackCGO(char* name);
extern void overlayCloseCallbackCGO(char* name);

// -----------------------------------------------------------------------------
// Overlay Window
// -----------------------------------------------------------------------------
@interface OverlayWindow : NSPanel
@property(nonatomic, strong) NSString *name; // Store the ID
@property(nonatomic, strong) NSView *nativeAttachmentView;
@property(nonatomic, strong) NSVisualEffectView *backgroundView;
@property(nonatomic, assign) int stickyPid;
@end

@interface OverlayWindow ()
@property(nonatomic, strong) NSTrackingArea *trackingArea;
@property(nonatomic, assign) BOOL isMouseInside;
@property(nonatomic, assign) NSPoint initialLocation;
@property(nonatomic, assign) BOOL isMovable;
@property(nonatomic, assign) BOOL isResizable;
@property(nonatomic, assign) BOOL isDragging;
@property(nonatomic, assign) BOOL isResizing;
@property(nonatomic, assign) NSPoint initialWindowOrigin;
@property(nonatomic, assign) NSRect initialResizeFrame;
@property(nonatomic, assign) NSUInteger activeResizeEdges;
@property(nonatomic, assign) CGFloat resizeAspectRatio;
// AXObserver for tracking window movement
@property(nonatomic, assign) AXObserverRef axObserver;
@property(nonatomic, assign) AXUIElementRef trackedWindow;
@property(nonatomic, assign) pid_t trackedPid;
@property(nonatomic, assign) OverlayOptions currentOpts;
// Target window number for z-order management
@property(nonatomic, assign) CGWindowID stickyWindowNumber;
@property(nonatomic, assign) BOOL transparentMode;
@property(nonatomic, assign) BOOL hitTestIconOnly;
@property(nonatomic, assign) BOOL closeOnEscape;
@property(nonatomic, assign) NSRect hitTestRect;
@property(nonatomic, strong) NSTimer *stickyLiveFollowTimer;
@property(nonatomic, assign) BOOL hasStickyPredictiveAnchor;
@property(nonatomic, assign) CGRect stickyPredictiveAnchorTargetRect;
@property(nonatomic, assign) NSPoint stickyPredictiveAnchorMouse;
@property(nonatomic, strong) NSRunningApplication *previousFrontmostApplication;
@end

static NSMutableDictionary<NSString*, OverlayWindow*> *gOverlayWindows = nil;

// -----------------------------------------------------------------------------
// Helper Classes
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
- (BOOL)isOpaque {
    return NO;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES; // Accept click even when window is not key
}

- (NSView *)hitTest:(NSPoint)point {
    OverlayWindow *overlay = [self.window isKindOfClass:[OverlayWindow class]] ? (OverlayWindow *)self.window : nil;
    if (overlay && overlay.transparentMode && overlay.hitTestIconOnly && !NSPointInRect(point, overlay.hitTestRect)) {
        return nil;
    }
    return [super hitTest:point];
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
        [self setAcceptsMouseMovedEvents:YES];

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
        [bg release];

        // Tracking Area setup
        [self setupTrackingArea];
        [contentView release];
    }
    return self;
}

- (void)setFrame:(NSRect)frameRect display:(BOOL)flag {
    [super setFrame:frameRect display:flag];
    [self refreshResizableOverlayAfterFrameChange];
}

- (void)setFrame:(NSRect)frameRect display:(BOOL)flag animate:(BOOL)animateFlag {
    [super setFrame:frameRect display:flag animate:animateFlag];
    [self refreshResizableOverlayAfterFrameChange];
}

- (void)refreshResizableOverlayAfterFrameChange {
    if (!self.isResizable || !self.transparentMode) return;
    [self.contentView layoutSubtreeIfNeeded];
    [self updateResizableContentFrame];
}

- (void)mouseDown:(NSEvent *)event {
    if (self.closeOnEscape) {
        // Focus-sensitive overlays should close only when they own keyboard focus. Make the clicked
        // overlay key here so Escape targets this window instead
        // of relying on a process-wide shortcut that would dismiss unrelated overlays.
        [self makeKeyWindow];
    }
    self.initialLocation = [NSEvent mouseLocation];
    self.initialWindowOrigin = self.frame.origin;

    if (self.isResizable) {
        NSUInteger resizeEdges = [self resizeEdgesForPoint:[event locationInWindow]];
        if (resizeEdges != OverlayResizeEdgeNone) {
            // Feature change: borderless overlays have no system resize frame, so edge
            // dragging is handled here while ordinary interior dragging still uses the existing
            // movable overlay path.
            self.isResizing = YES;
            self.activeResizeEdges = resizeEdges;
            self.initialResizeFrame = self.frame;
            return;
        }
    }

    if (self.isMovable) {
        self.isDragging = YES;
    }
}

- (void)mouseDragged:(NSEvent *)event {
    if (self.isResizing) {
        [self resizeFromCurrentMouseLocation];
        return;
    }
    if (!self.isDragging) return;
    
    NSPoint currentLocation = [NSEvent mouseLocation];
    CGFloat dx = currentLocation.x - self.initialLocation.x;
    CGFloat dy = currentLocation.y - self.initialLocation.y;
    
    NSPoint newOrigin = NSMakePoint(self.initialWindowOrigin.x + dx,
                                    self.initialWindowOrigin.y + dy);
    [self setFrameOrigin:newOrigin];
}

- (void)mouseMoved:(NSEvent *)event {
    [self updateResizeCursorForPoint:[event locationInWindow]];
}

- (void)mouseUp:(NSEvent *)event {
    BOOL wasResizing = self.isResizing;
    self.isResizing = NO;
    self.activeResizeEdges = OverlayResizeEdgeNone;
    if (wasResizing) {
        [self updateResizeCursorForPoint:[event locationInWindow]];
        return;
    }

    self.isDragging = NO;
    
    NSPoint currentLocation = [NSEvent mouseLocation];
    CGFloat dx = currentLocation.x - self.initialLocation.x;
    CGFloat dy = currentLocation.y - self.initialLocation.y;
    
    // If movement is small, treat as click
    if (dx*dx + dy*dy < 25.0) {
        [self onClick];
    }
    
    [self updateResizeCursorForPoint:[event locationInWindow]];
}

- (NSUInteger)resizeEdgesForPoint:(NSPoint)point {
    if (!self.isResizable) return OverlayResizeEdgeNone;
    NSRect bounds = self.contentView.bounds;
    NSUInteger edges = OverlayResizeEdgeNone;
    if (point.x <= kResizeGripSize) edges |= OverlayResizeEdgeLeft;
    if (point.x >= NSMaxX(bounds) - kResizeGripSize) edges |= OverlayResizeEdgeRight;
    if (point.y <= kResizeGripSize) edges |= OverlayResizeEdgeBottom;
    if (point.y >= NSMaxY(bounds) - kResizeGripSize) edges |= OverlayResizeEdgeTop;
    return edges;
}

- (void)resizeFromCurrentMouseLocation {
    NSPoint currentLocation = [NSEvent mouseLocation];
    CGFloat dx = currentLocation.x - self.initialLocation.x;
    CGFloat dy = currentLocation.y - self.initialLocation.y;
    NSRect frame = self.initialResizeFrame;

    if (self.resizeAspectRatio <= 0) {
        if (self.activeResizeEdges & OverlayResizeEdgeLeft) {
            CGFloat width = MAX(kResizeMinSize, self.initialResizeFrame.size.width - dx);
            frame.origin.x = NSMaxX(self.initialResizeFrame) - width;
            frame.size.width = width;
        } else if (self.activeResizeEdges & OverlayResizeEdgeRight) {
            frame.size.width = MAX(kResizeMinSize, self.initialResizeFrame.size.width + dx);
        }

        if (self.activeResizeEdges & OverlayResizeEdgeBottom) {
            CGFloat height = MAX(kResizeMinSize, self.initialResizeFrame.size.height - dy);
            frame.origin.y = NSMaxY(self.initialResizeFrame) - height;
            frame.size.height = height;
        } else if (self.activeResizeEdges & OverlayResizeEdgeTop) {
            frame.size.height = MAX(kResizeMinSize, self.initialResizeFrame.size.height + dy);
        }

        [self setFrame:frame display:YES];
        [self updateResizableContentFrame];
        return;
    }

    BOOL hasHorizontalEdge = (self.activeResizeEdges & (OverlayResizeEdgeLeft | OverlayResizeEdgeRight)) != 0;
    BOOL hasVerticalEdge = (self.activeResizeEdges & (OverlayResizeEdgeTop | OverlayResizeEdgeBottom)) != 0;
    CGFloat requestedWidth = self.initialResizeFrame.size.width;
    CGFloat requestedHeight = self.initialResizeFrame.size.height;

    if (self.activeResizeEdges & OverlayResizeEdgeLeft) {
        requestedWidth = self.initialResizeFrame.size.width - dx;
    } else if (self.activeResizeEdges & OverlayResizeEdgeRight) {
        requestedWidth = self.initialResizeFrame.size.width + dx;
    }
    if (self.activeResizeEdges & OverlayResizeEdgeBottom) {
        requestedHeight = self.initialResizeFrame.size.height - dy;
    } else if (self.activeResizeEdges & OverlayResizeEdgeTop) {
        requestedHeight = self.initialResizeFrame.size.height + dy;
    }

    CGFloat baseWidth = MAX(1, self.initialResizeFrame.size.width);
    CGFloat baseHeight = MAX(1, self.initialResizeFrame.size.height);
    CGFloat scaleFromWidth = requestedWidth / baseWidth;
    CGFloat scaleFromHeight = requestedHeight / baseHeight;
    CGFloat scale = 1.0;
    if (hasHorizontalEdge && hasVerticalEdge) {
        scale = fabs(scaleFromWidth - 1.0) >= fabs(scaleFromHeight - 1.0) ? scaleFromWidth : scaleFromHeight;
    } else if (hasHorizontalEdge) {
        scale = scaleFromWidth;
    } else if (hasVerticalEdge) {
        scale = scaleFromHeight;
    }

    CGFloat width = MAX(kResizeMinSize, self.initialResizeFrame.size.width * scale);
    CGFloat height = width / self.resizeAspectRatio;
    if (height < kResizeMinSize) {
        height = kResizeMinSize;
        width = height * self.resizeAspectRatio;
    }

    // Feature change: transparent overlays may request aspect-locked resize. The manual
    // borderless resize loop therefore derives the second dimension from the source aspect ratio
    // while preserving the dragged edge as the fixed anchor.
    frame.size.width = width;
    frame.size.height = height;

    if (hasHorizontalEdge) {
        if (self.activeResizeEdges & OverlayResizeEdgeLeft) {
            frame.origin.x = NSMaxX(self.initialResizeFrame) - width;
        } else {
            frame.origin.x = self.initialResizeFrame.origin.x;
        }
    } else {
        frame.origin.x = NSMidX(self.initialResizeFrame) - width / 2;
    }

    if (hasVerticalEdge) {
        if (self.activeResizeEdges & OverlayResizeEdgeBottom) {
            frame.origin.y = NSMaxY(self.initialResizeFrame) - height;
        } else {
            frame.origin.y = self.initialResizeFrame.origin.y;
        }
    } else {
        frame.origin.y = NSMidY(self.initialResizeFrame) - height / 2;
    }

    [self setFrame:frame display:YES];
    [self updateResizableContentFrame];
}

- (NSCursor *)cursorForResizeEdges:(NSUInteger)edges {
    if ((edges & (OverlayResizeEdgeLeft | OverlayResizeEdgeRight)) &&
        (edges & (OverlayResizeEdgeTop | OverlayResizeEdgeBottom))) {
        // Feature change: AppKit only exposes diagonal frame cursors on newer macOS versions.
        // A tiny custom cursor keeps the borderless overlay resize affordance consistent with the
        // hand-written borderless resize logic while preserving the 10.15 deployment target.
        return [self diagonalResizeCursorForEdges:edges];
    }
    if (edges & (OverlayResizeEdgeLeft | OverlayResizeEdgeRight)) return [NSCursor resizeLeftRightCursor];
    if (edges & (OverlayResizeEdgeTop | OverlayResizeEdgeBottom)) return [NSCursor resizeUpDownCursor];
    return [NSCursor arrowCursor];
}

- (NSCursor *)diagonalResizeCursorForEdges:(NSUInteger)edges {
    static NSCursor *nwseCursor = nil;
    static NSCursor *neswCursor = nil;
    BOOL nwse = ((edges & OverlayResizeEdgeTop) && (edges & OverlayResizeEdgeLeft)) ||
                ((edges & OverlayResizeEdgeBottom) && (edges & OverlayResizeEdgeRight));
    NSCursor **slot = nwse ? &nwseCursor : &neswCursor;
    if (*slot) return *slot;

    NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(18, 18)];
    [image lockFocus];
    [[NSColor clearColor] setFill];
    NSRectFill(NSMakeRect(0, 0, 18, 18));
    NSBezierPath *path = [NSBezierPath bezierPath];
    path.lineWidth = 2;
    path.lineCapStyle = NSLineCapStyleRound;
    if (nwse) {
        [path moveToPoint:NSMakePoint(4, 14)];
        [path lineToPoint:NSMakePoint(14, 4)];
        [path moveToPoint:NSMakePoint(4, 14)];
        [path lineToPoint:NSMakePoint(4, 9)];
        [path moveToPoint:NSMakePoint(4, 14)];
        [path lineToPoint:NSMakePoint(9, 14)];
        [path moveToPoint:NSMakePoint(14, 4)];
        [path lineToPoint:NSMakePoint(14, 9)];
        [path moveToPoint:NSMakePoint(14, 4)];
        [path lineToPoint:NSMakePoint(9, 4)];
    } else {
        [path moveToPoint:NSMakePoint(4, 4)];
        [path lineToPoint:NSMakePoint(14, 14)];
        [path moveToPoint:NSMakePoint(4, 4)];
        [path lineToPoint:NSMakePoint(4, 9)];
        [path moveToPoint:NSMakePoint(4, 4)];
        [path lineToPoint:NSMakePoint(9, 4)];
        [path moveToPoint:NSMakePoint(14, 14)];
        [path lineToPoint:NSMakePoint(14, 9)];
        [path moveToPoint:NSMakePoint(14, 14)];
        [path lineToPoint:NSMakePoint(9, 14)];
    }
    [[NSColor colorWithWhite:0 alpha:0.55] setStroke];
    [path stroke];
    [path setLineWidth:1];
    [[NSColor whiteColor] setStroke];
    [path stroke];
    [image unlockFocus];

    *slot = [[NSCursor alloc] initWithImage:image hotSpot:NSMakePoint(9, 9)];
    [image release];
    return *slot;
}

- (void)updateResizeCursorForPoint:(NSPoint)point {
    if (!self.isResizable) return;
    if (self.isDragging || self.isResizing) return;
    NSUInteger edges = [self resizeEdgesForPoint:point];
    [[self cursorForResizeEdges:edges] set];
}

- (void)updateResizableContentFrame {
    if (!self.isResizable || !self.transparentMode) return;
    if (self.nativeAttachmentView) {
        self.nativeAttachmentView.frame = self.contentView.bounds;
    }
    self.hitTestRect = self.nativeAttachmentView ? self.contentView.bounds : NSZeroRect;
}

- (void)keyDown:(NSEvent *)event {
    if (self.closeOnEscape && event.keyCode == 53) {
        // Escape is scoped to the focused overlay window, so one focused overlay closes at a time.
        [self onClose];
        return;
    }
    [super keyDown:event];
}

- (void)setupTrackingArea {
    if (self.trackingArea) {
        [self.contentView removeTrackingArea:self.trackingArea];
    }
    
    NSTrackingAreaOptions options = NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect;
    NSTrackingArea *trackingArea = [[NSTrackingArea alloc] initWithRect:NSZeroRect // Ignored by InVisibleRect
                                                                 options:options
                                                                   owner:self
                                                                userInfo:nil];
    self.trackingArea = trackingArea;
    [trackingArea release];
    [self.contentView addTrackingArea:self.trackingArea];
}

- (void)mouseEntered:(NSEvent *)event {
    self.isMouseInside = YES;
}

- (void)mouseExited:(NSEvent *)event {
    self.isMouseInside = NO;
    if (!self.isDragging && !self.isResizing) {
        [[NSCursor arrowCursor] set];
    }
}

- (void)setCornerRadius:(CGFloat)radius {
    self.contentView.wantsLayer = YES;
    self.contentView.layer.cornerRadius = radius;
    self.contentView.layer.masksToBounds = YES;
}

- (void)onClose {
    [self stopTrackingWindow];
    // Notify the Go layer that a base-window close action occurred so callers
    // like the dictation plugin can cancel their operation.
    if (self.name) {
        overlayCloseCallbackCGO((char*)[self.name UTF8String]);
    }
    [self close];
    if (gOverlayWindows && self.name) {
        [gOverlayWindows removeObjectForKey:self.name];
    }
}

- (void)stopTrackingWindow {
    [self stopStickyLiveFollowTimer];
    self.hasStickyPredictiveAnchor = NO;

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
    self.trackedPid = 0;
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

- (BOOL)getWindowFrameForPid:(pid_t)pid outRect:(CGRect *)outRect {
    if (pid <= 0 || !outRect) return NO;

    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (!windowList) return NO;

    BOOL found = NO;
    for (CFIndex i = 0; i < CFArrayGetCount(windowList); i++) {
        NSDictionary *windowInfo = (NSDictionary *)CFArrayGetValueAtIndex(windowList, i);
        NSNumber *windowPid = windowInfo[(id)kCGWindowOwnerPID];
        NSNumber *windowLayer = windowInfo[(id)kCGWindowLayer];
        NSNumber *windowAlpha = windowInfo[(id)kCGWindowAlpha];
        NSDictionary *boundsDict = windowInfo[(id)kCGWindowBounds];

        if ([windowPid intValue] != pid || [windowLayer intValue] != 0 || !boundsDict) {
            continue;
        }
        if (windowAlpha && [windowAlpha doubleValue] <= 0.01) {
            continue;
        }

        CGRect cgBounds = CGRectZero;
        if (!CGRectMakeWithDictionaryRepresentation((CFDictionaryRef)boundsDict, &cgBounds)) {
            continue;
        }
        if (cgBounds.size.width <= 1 || cgBounds.size.height <= 1) {
            continue;
        }

        // Bug fix: AX focused-window lookup can fail for debug builds or stale
        // focus state. Sticky overlays must stay attached to their target window;
        // falling back to the whole screen can place them offscreen. CGWindowList
        // gives the native target rect without requiring AX, while preserving the
        // existing AX observer path when accessibility is available.
        CGFloat mainScreenH = [NSScreen mainScreen].frame.size.height;
        CGFloat cocoaY = mainScreenH - cgBounds.origin.y - cgBounds.size.height;
        *outRect = CGRectMake(cgBounds.origin.x, cocoaY, cgBounds.size.width, cgBounds.size.height);
        found = YES;
        break;
    }

    CFRelease(windowList);
    return found;
}

// Order overlay window relative to sticky window
- (void)orderRelativeToStickyWindow {
    if (self.currentOpts.topmost) {
        // Bug fix: Wox's launcher window uses the pop-up-menu level on macOS, so the previous
        // floating overlay level could be visually underneath Wox. Topmost overlays are explicit
        // user surfaces such as enlarged image previews, so lift them above the launcher without
        // changing ordinary notification overlays.
        [self setLevel:NSPopUpMenuWindowLevel + 1];
        [self orderFrontRegardless];
        return;
    }

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

- (void)focusForKeyboardDismissalIfNeeded {
    if (!self.closeOnEscape) {
        return;
    }

    // Bug fix: showing a panel with orderFront/orderFrontRegardless only changes visibility and
    // z-order; it does not guarantee keyboard focus. Overlays that advertise Escape-to-close must
    // become the key window as soon as they appear so Escape works without an extra click.
    NSRunningApplication *frontmostApplication = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (frontmostApplication && frontmostApplication.processIdentifier != [[NSProcessInfo processInfo] processIdentifier]) {
        // Preserve the original target only once. Overlay content updates happen after Wox becomes
        // frontmost and must not replace the application that should regain focus on close.
        if (!self.previousFrontmostApplication) {
            self.previousFrontmostApplication = frontmostApplication;
        }
    }
    [NSApp activateIgnoringOtherApps:YES];
    [self makeKeyAndOrderFront:nil];
}

- (void)restorePreviousFrontmostApplication {
    NSRunningApplication *application = [[self.previousFrontmostApplication retain] autorelease];
    self.previousFrontmostApplication = nil;
    if (!application || application.terminated) {
        return;
    }

    NSRunningApplication *frontmostApplication = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (frontmostApplication && frontmostApplication.processIdentifier != [[NSProcessInfo processInfo] processIdentifier]) {
        // Do not override a focus change the user made while the overlay was visible.
        return;
    }

    if (@available(macOS 14.0, *)) {
        [application activateWithOptions:0];
        return;
    }

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    [application activateWithOptions:NSApplicationActivateIgnoringOtherApps];
#pragma clang diagnostic pop
}

- (void)onClick {
    if (self.name) {
       overlayClickCallbackCGO((char*)[self.name UTF8String]);
    }
}

- (void)detachNativeAttachment {
    if (!self.nativeAttachmentView) return;
    [self.nativeAttachmentView removeFromSuperview];
    self.nativeAttachmentView = nil;
}

- (void)close {
    [self detachNativeAttachment];
    [super close];
    [self restorePreviousFrontmostApplication];
}

- (void)dealloc {
    [self stopTrackingWindow];
    [self detachNativeAttachment];
    if (self.trackingArea) {
        [self.contentView removeTrackingArea:self.trackingArea];
    }
    self.name = nil;
    self.nativeAttachmentView = nil;
    self.backgroundView = nil;
    self.trackingArea = nil;
    self.previousFrontmostApplication = nil;
    [super dealloc];
}

- (BOOL)canBecomeKeyWindow {
    return YES; // Allow interaction
}

- (void)updateLayoutWithOptions:(OverlayOptions)opts {
    // 0. Reset State
    self.isMovable = opts.movable;
    self.isResizable = opts.resizable;
    self.resizeAspectRatio = (opts.resizable && opts.aspectRatio > 0) ? opts.aspectRatio : 0;
    self.isDragging = NO;
    self.isResizing = NO;
    self.activeResizeEdges = OverlayResizeEdgeNone;
    NSWindowStyleMask styleMask = NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel;
    if (opts.resizable) {
        // Bug fix: NSWindowStyleMaskResizable lets AppKit run its own borderless resize loop. Keep
        // the panel borderless and use the explicit edge-drag resize path below.
        self.minSize = NSMakeSize(kResizeMinSize, kResizeMinSize);
    }
    self.styleMask = styleMask;

    // 1. Content Update
    self.closeOnEscape = opts.closeOnEscape;
    self.hitTestRect = NSZeroRect;

    // 2. Measure & Layout
    CGFloat windowWidth = (opts.width > 0) ? opts.width : MAX(1, opts.nativeAttachmentWidth);
    CGFloat windowHeight = (opts.height > 0) ? opts.height : MAX(1, opts.nativeAttachmentHeight);
    BOOL hasNativeAttachmentLayout = NO;
    NSRect requestedNativeAttachmentFrame = NSZeroRect;
    self.transparentMode = opts.transparent;
    self.hitTestIconOnly = opts.hitTestIconOnly;
    self.backgroundView.hidden = opts.transparent;
    [self setHasShadow:!opts.transparent];

    if (opts.nativeAttachment && opts.nativeAttachmentKind == kNativeAttachmentKindView && opts.nativeAttachmentHandle) {
        BOOL transparentAttachment = opts.transparent;
        // Native attachments report their content size; base window chrome must be added around it.
        CGFloat horizontalChrome = transparentAttachment ? 0 : 36;
        CGFloat verticalChrome = transparentAttachment ? 0 : 24;
        windowWidth = (opts.width > 0) ? opts.width : MAX(transparentAttachment ? 1 : 64, opts.nativeAttachmentWidth + horizontalChrome);
        windowHeight = (opts.height > 0) ? opts.height : MAX(transparentAttachment ? 1 : 40, opts.nativeAttachmentHeight + verticalChrome);

        self.hitTestRect = transparentAttachment ? self.contentView.bounds : NSZeroRect;

        NSView *attachmentView = (__bridge NSView *)opts.nativeAttachmentHandle;
        if (self.nativeAttachmentView != attachmentView) {
            [self detachNativeAttachment];
            self.nativeAttachmentView = attachmentView;
        }
        if (self.nativeAttachmentView.superview != self.contentView) {
            [self.nativeAttachmentView removeFromSuperview];
            [self.contentView addSubview:self.nativeAttachmentView positioned:NSWindowAbove relativeTo:nil];
        }

        CGFloat padLeft = transparentAttachment ? 0 : 18;
        CGFloat padRight = transparentAttachment ? 0 : 18;
        CGFloat attachmentWidth = transparentAttachment ? windowWidth : MAX(48, windowWidth - padLeft - padRight);
        CGFloat attachmentHeight = transparentAttachment ? windowHeight : ((opts.nativeAttachmentHeight > 0) ? opts.nativeAttachmentHeight : MAX(1, windowHeight - 20));
        CGFloat attachmentY = transparentAttachment ? 0 : (windowHeight - attachmentHeight) / 2.0;
        self.nativeAttachmentView.hidden = NO;
        // The content view can still be zero-sized before the panel frame is applied. Non-transparent
        // HUD attachments are measured content, so autoresizing here would stretch them by the full
        // window delta during setFrame. Transparent surfaces still need autoresizing for resize/fill.
        self.nativeAttachmentView.autoresizingMask = transparentAttachment ? (NSViewWidthSizable | NSViewHeightSizable) : NSViewNotSizable;
        requestedNativeAttachmentFrame = NSMakeRect(padLeft, attachmentY, attachmentWidth, attachmentHeight);
        hasNativeAttachmentLayout = YES;
        self.nativeAttachmentView.frame = requestedNativeAttachmentFrame;

    } else {
        [self detachNativeAttachment];
        windowWidth = (opts.width > 0) ? opts.width : MAX(1, kDefaultWindowWidth);
        windowHeight = (opts.height > 0) ? opts.height : 40;
        if (opts.minWidth > 0) {
            windowWidth = MAX(windowWidth, opts.minWidth);
        }
        if (opts.height <= 0 && opts.maxHeight > 0 && windowHeight > opts.maxHeight) {
            windowHeight = opts.maxHeight;
        }
    }

    // 3. Position Calculation (Anchor)
    CGRect targetRect;
    BOOL preserveLiveFollowFrame = opts.stickyWindowPid > 0 && self.stickyLiveFollowTimer != nil;
    NSPoint liveFollowOrigin = self.frame.origin;
    
    if (opts.absolutePosition) {
        // Feature addition: absolute overlays already pass desktop top-left coordinates from Go.
        // Keep them independent from the primary screen work-area anchor used
        // by notifications.
        self.stickyWindowNumber = 0;
        targetRect = CGRectZero;
    } else if (preserveLiveFollowFrame) {
        // Bug fix: content refreshes can arrive while a sticky overlay is being
        // live-followed. Re-anchoring from AX here can use stale geometry and pull
        // the overlay behind the dragged window, so preserve the live-followed
        // origin and let the poller own position updates during the active drag.
        self.stickyWindowNumber = [self getWindowNumberForPid:(pid_t)opts.stickyWindowPid];
        targetRect = CGRectMake(liveFollowOrigin.x, liveFollowOrigin.y, windowWidth, windowHeight);
    } else if (opts.stickyWindowPid > 0) {
        pid_t pid = (pid_t)opts.stickyWindowPid;
        
        // Get window number for z-order management
        self.stickyWindowNumber = [self getWindowNumberForPid:pid];

        BOOL targetFound = NO;
        AXUIElementRef app = AXUIElementCreateApplication(pid);
        AXUIElementRef frontWindow = NULL;
        AXError err = app ? AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, (CFTypeRef *)&frontWindow) : kAXErrorFailure;
        if (err == kAXErrorSuccess && frontWindow) {
            CFTypeRef posVal = NULL, sizeVal = NULL;
            CGPoint pos; CGSize size;
            AXError posErr = AXUIElementCopyAttributeValue(frontWindow, kAXPositionAttribute, &posVal);
            AXError sizeErr = AXUIElementCopyAttributeValue(frontWindow, kAXSizeAttribute, &sizeVal);
            if (posErr == kAXErrorSuccess && sizeErr == kAXErrorSuccess && posVal && sizeVal) {
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
                targetFound = YES;
            }
            if (posVal) CFRelease(posVal);
            if (sizeVal) CFRelease(sizeVal);
            CFRelease(frontWindow);
        }
        if (app) CFRelease(app);
        if (!targetFound) {
            if (![self getWindowFrameForPid:pid outRect:&targetRect]) {
                targetRect = [NSScreen mainScreen].visibleFrame;
            }
        }
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
    if (opts.absolutePosition) {
        // Match the Windows absolute-position contract: offset values name the
        // requested anchor point in top-left desktop coordinates, not always the
        // overlay's top-left corner. Callers can use BottomCenter/LeftCenter
        // anchors to place overlays above or beside a desktop point.
        CGFloat absoluteTopLeftX = opts.offsetX;
        if (col == 1) absoluteTopLeftX -= windowWidth / 2;
        else if (col == 2) absoluteTopLeftX -= windowWidth;

        CGFloat absoluteTopLeftY = opts.offsetY;
        if (row == 1) absoluteTopLeftY -= windowHeight / 2;
        else if (row == 2) absoluteTopLeftY -= windowHeight;

        CGFloat mainScreenH = [NSScreen mainScreen].frame.size.height;
        finalX = absoluteTopLeftX;
        finalY = mainScreenH - absoluteTopLeftY - windowHeight;
    }
    if (preserveLiveFollowFrame) {
        finalX = liveFollowOrigin.x;
        finalY = liveFollowOrigin.y;
    } else if (opts.preservePosition) {
        // AppKit frame origins are bottom-left. Preserve the visible top-left corner so
        // content refreshes grow downward like the Windows overlay path.
        finalX = self.frame.origin.x;
        finalY = NSMaxY(self.frame) - windowHeight;
    }
    if (opts.stickyWindowPid > 0 && !preserveLiveFollowFrame && CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState, kCGMouseButtonLeft)) {
        // Optimization: layout refresh can detect mouse-down before the first AX
        // move notification. Seeding the predictive anchor here gives the live
        // poller a usable baseline instead of waiting for the low-frequency AX
        // movement event.
        [self refreshStickyPredictiveAnchorWithTargetRect:targetRect];
    }

    [self setFrame:NSMakeRect(finalX, finalY, windowWidth, windowHeight) display:YES];
    self.backgroundView.frame = self.contentView.bounds;
    [self updateResizableContentFrame];
    if (hasNativeAttachmentLayout) {
        // Reapply after setFrame so measured HUD attachments keep their intended padding inside the
        // now-sized content view, instead of inheriting any intermediate AppKit resize adjustment.
        self.nativeAttachmentView.frame = requestedNativeAttachmentFrame;
        if (opts.transparent) {
            self.hitTestRect = self.contentView.bounds;
        }
    }
    [self setCornerRadius:(opts.transparent ? 0 : 10.0)];
    
    // 4. Store options and setup window tracking
    self.currentOpts = opts;
    if (opts.stickyWindowPid > 0) {
        [self startTrackingWindowWithPid:opts.stickyWindowPid];
        // Optimization: animation/content refreshes can observe the mouse-down
        // state before the first coalesced AX move notification arrives. Starting
        // live follow from this generic refresh path reduces the initial sticky
        // lag without requiring callers to know when native dragging begins.
        [self startStickyLiveFollowTimerIfNeeded];
    } else {
        [self stopTrackingWindow];
    }

}

// AXObserver callback - called when tracked window moves or resizes
static void axObserverCallback(AXObserverRef observer, AXUIElementRef element, CFStringRef notification, void *refcon) {
    OverlayWindow *win = (__bridge OverlayWindow *)refcon;
    [win handleTrackedWindowMoved];
}

- (void)handleTrackedWindowMoved {
    // Sticky overlays are a generic base capability. The earlier implementation
    // hid overlays until the user released the target window, which made attached
    // surfaces flicker and lag. Always live-follow here so every module gets the
    // same stable window attachment behavior.
    [self updatePositionFromTrackedWindow];
    [self startStickyLiveFollowTimerIfNeeded];
    [self orderRelativeToStickyWindow];
    self.alphaValue = 1.0;
}

- (void)startStickyLiveFollowTimerIfNeeded {
    if (self.stickyLiveFollowTimer || self.currentOpts.stickyWindowPid <= 0) {
        return;
    }
    if (!CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState, kCGMouseButtonLeft)) {
        return;
    }

    // Bug fix: AX moved notifications are coalesced by macOS and can arrive only
    // every 90-120ms while the target window is dragged. Polling during the active
    // drag keeps sticky overlays attached at frame cadence without changing the
    // generic overlay API or adding module-specific follow modes.
    self.stickyLiveFollowTimer = [NSTimer timerWithTimeInterval:(1.0 / 60.0)
                                                         target:self
                                                       selector:@selector(handleStickyLiveFollowTimer:)
                                                       userInfo:nil
                                                        repeats:YES];
    [[NSRunLoop mainRunLoop] addTimer:self.stickyLiveFollowTimer forMode:NSRunLoopCommonModes];
}

- (void)stopStickyLiveFollowTimer {
    if (!self.stickyLiveFollowTimer) {
        return;
    }
    [self.stickyLiveFollowTimer invalidate];
    self.stickyLiveFollowTimer = nil;
    self.hasStickyPredictiveAnchor = NO;
}

- (void)refreshStickyPredictiveAnchorWithTargetRect:(CGRect)targetRect {
    if (!CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState, kCGMouseButtonLeft)) {
        return;
    }
    // Predictive follow uses true sticky samples as anchors and then applies
    // mouse deltas between those samples. This keeps the overlay moving at timer
    // cadence even when AX/CG window geometry updates arrive at only ~10Hz.
    self.stickyPredictiveAnchorTargetRect = targetRect;
    self.stickyPredictiveAnchorMouse = [NSEvent mouseLocation];
    self.hasStickyPredictiveAnchor = YES;
}

- (BOOL)getStickyPredictiveTargetRect:(CGRect *)outRect {
    if (!self.hasStickyPredictiveAnchor || !outRect) {
        return NO;
    }
    NSPoint mouse = [NSEvent mouseLocation];
    CGFloat dx = mouse.x - self.stickyPredictiveAnchorMouse.x;
    CGFloat dy = mouse.y - self.stickyPredictiveAnchorMouse.y;
    *outRect = CGRectOffset(self.stickyPredictiveAnchorTargetRect, dx, dy);
    return YES;
}

- (void)handleStickyLiveFollowTimer:(NSTimer *)timer {
    if (!CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState, kCGMouseButtonLeft)) {
        [self stopStickyLiveFollowTimer];
        return;
    }

    [self updatePositionFromTrackedWindowPreferCGWindowList:YES];
    [self orderRelativeToStickyWindow];
    self.alphaValue = 1.0;
}

- (void)startTrackingWindowWithPid:(pid_t)pid {
    if (pid > 0 && self.trackedPid == pid && self.axObserver && self.trackedWindow) {
        // Optimization: reused overlays can refresh their content frequently.
        // Reusing the existing AX observer avoids tearing down native tracking on
        // every update, which keeps live-follow reliable for all overlay modules.
        return;
    }

    // Stop any existing tracking first
    [self stopTrackingWindow];
    // Create AXUIElement for the application
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (!app)
        return;
    
    // Get the focused window
    AXUIElementRef frontWindow = NULL;
    AXError err = AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, (CFTypeRef *)&frontWindow);
    if (err != kAXErrorSuccess || !frontWindow) {
        CFRelease(app);
        return;
    }
    
    // Store the tracked window
    self.trackedWindow = frontWindow;
    self.trackedPid = pid;
    
    // Create AXObserver
    AXObserverRef observer = NULL;
    err = AXObserverCreate(pid, axObserverCallback, &observer);
    if (err != kAXErrorSuccess || !observer) {
        CFRelease(app);
        CFRelease(frontWindow);
        self.trackedWindow = NULL;
        self.trackedPid = 0;
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

- (BOOL)updatePositionFromTrackedWindow {
    return [self updatePositionFromTrackedWindowPreferCGWindowList:NO];
}

- (BOOL)updatePositionFromTrackedWindowPreferCGWindowList:(BOOL)preferCGWindowList {
    if (self.currentOpts.stickyWindowPid <= 0) return NO;

    CGRect targetRect;
    BOOL targetFound = NO;
    CFTypeRef posVal = NULL, sizeVal = NULL;
    CGPoint pos; CGSize size;
    BOOL preserveSmallPredictiveCorrection = self.stickyLiveFollowTimer != nil &&
                                             self.hasStickyPredictiveAnchor &&
                                             !preferCGWindowList &&
                                             CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState, kCGMouseButtonLeft);
    NSPoint predictedOriginBeforeRealSample = self.frame.origin;

    if (preferCGWindowList && [self getStickyPredictiveTargetRect:&targetRect]) {
        targetFound = YES;
    }

    if (!targetFound && preferCGWindowList) {
        // Bug fix: during live dragging, AX position attributes can stay stale
        // between coalesced move notifications. CGWindowList is queried first for
        // the polling path because it reflects compositor window bounds without
        // waiting for the next AX notification.
        if ([self getWindowFrameForPid:(pid_t)self.currentOpts.stickyWindowPid outRect:&targetRect]) {
            targetFound = YES;
        }
    }

    if (!targetFound && self.trackedWindow) {
        AXError posErr = AXUIElementCopyAttributeValue(self.trackedWindow, kAXPositionAttribute, &posVal);
        AXError sizeErr = AXUIElementCopyAttributeValue(self.trackedWindow, kAXSizeAttribute, &sizeVal);
        if (posErr == kAXErrorSuccess && sizeErr == kAXErrorSuccess && posVal && sizeVal) {
            AXValueGetValue(posVal, kAXValueCGPointType, &pos);
            AXValueGetValue(sizeVal, kAXValueCGSizeType, &size);

            // Bug fix: use the tracked AX window instead of asking for the current
            // focused window on every move. During drag, focus can lag behind the
            // window geometry, while the observer element is the window that moved.
            CGFloat mainScreenH = [NSScreen mainScreen].frame.size.height;
            CGFloat cocoaY = mainScreenH - pos.y - size.height;
            targetRect = CGRectMake(pos.x, cocoaY, size.width, size.height);
            targetFound = YES;
        }
        if (posVal) CFRelease(posVal);
        if (sizeVal) CFRelease(sizeVal);
    }

    if (!targetFound) {
        if ([self getWindowFrameForPid:(pid_t)self.currentOpts.stickyWindowPid outRect:&targetRect]) {
            targetFound = YES;
        } else {
            return NO;
        }
    }

    if (targetFound && !preferCGWindowList) {
        [self refreshStickyPredictiveAnchorWithTargetRect:targetRect];
    }

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

    if (preserveSmallPredictiveCorrection) {
        CGFloat correctionX = finalX - predictedOriginBeforeRealSample.x;
        CGFloat correctionY = finalY - predictedOriginBeforeRealSample.y;
        if (fabs(correctionX) <= kStickyPredictiveCorrectionThreshold &&
            fabs(correctionY) <= kStickyPredictiveCorrectionThreshold) {
            // Optimization: low-frequency AX samples are still used to refresh the
            // predictive anchor, but small real-sample corrections should not pull
            // the overlay back to an older geometry point during an active drag.
            // Preserving the timer-driven frame removes the visible snap while the
            // threshold still allows large corrections for window snapping, screen
            // edge changes, or other cases where prediction has genuinely drifted.
            finalX = predictedOriginBeforeRealSample.x;
            finalY = predictedOriginBeforeRealSample.y;
        }
    }
    
    [self setFrameOrigin:NSMakePoint(finalX, finalY)];
    return YES;
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
        [win focusForKeyboardDismissalIfNeeded];
        win.alphaValue = 1.0;
    }
}

void CloseOverlay(char* name) {
    @autoreleasepool {
        if (!gOverlayWindows) return;
        NSString *key = [NSString stringWithUTF8String:name];
        OverlayWindow *win = [gOverlayWindows objectForKey:key];
        if (win) {
            [win stopTrackingWindow];
            [win close];
            [gOverlayWindows removeObjectForKey:key];
        }
    }
}
