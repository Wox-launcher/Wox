#import <Cocoa/Cocoa.h>
#import <Dispatch/Dispatch.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

extern void overlayRequestCloseCallbackCGO(char *name);

static const CGFloat kImageOverlayCloseSize = 24.0;
static const CGFloat kImageOverlayCloseMargin = 8.0;
static const CGFloat kImageOverlayMinZoomSize = 64.0;
static const CGFloat kImageOverlayWheelZoomStep = 1.12;

@interface WoxImageOverlayView : NSView
@property(nonatomic, copy) NSString *name;
@property(nonatomic, strong) NSImage *image;
@property(nonatomic, assign) CGFloat cornerRadius;
@property(nonatomic, assign) BOOL closable;
@property(nonatomic, strong) NSButton *closeButton;
@end

@implementation WoxImageOverlayView

- (instancetype)initWithName:(NSString *)name image:(NSImage *)image cornerRadius:(CGFloat)cornerRadius closable:(BOOL)closable {
    self = [super initWithFrame:NSZeroRect];
    if (!self) {
        return nil;
    }
    self.name = name ?: @"";
    self.image = image;
    self.cornerRadius = cornerRadius;
    self.closable = closable;
    self.wantsLayer = YES;
    self.layer.backgroundColor = [NSColor clearColor].CGColor;
    self.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    NSButton *closeButton = [[NSButton alloc] initWithFrame:NSZeroRect];
    self.closeButton = closeButton;
    [closeButton release];
    self.closeButton.bezelStyle = NSBezelStyleRegularSquare;
    self.closeButton.buttonType = NSButtonTypeMomentaryLight;
    self.closeButton.bordered = NO;
    self.closeButton.focusRingType = NSFocusRingTypeNone;
    self.closeButton.hidden = !closable;
    self.closeButton.wantsLayer = YES;
    self.closeButton.layer.backgroundColor = [NSColor colorWithWhite:0.0 alpha:0.46].CGColor;
    self.closeButton.layer.cornerRadius = kImageOverlayCloseSize / 2.0;
    self.closeButton.target = self;
    self.closeButton.action = @selector(onCloseButtonClicked:);
    NSMutableAttributedString *closeTitle = [[NSMutableAttributedString alloc] initWithString:@"×"];
    [closeTitle addAttribute:NSForegroundColorAttributeName value:[NSColor whiteColor] range:NSMakeRange(0, closeTitle.length)];
    [closeTitle addAttribute:NSFontAttributeName value:[NSFont systemFontOfSize:16 weight:NSFontWeightBold] range:NSMakeRange(0, closeTitle.length)];
    [self.closeButton setAttributedTitle:closeTitle];
    [closeTitle release];
    [self addSubview:self.closeButton];
    return self;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES;
}

- (void)drawRect:(NSRect)dirtyRect {
    [super drawRect:dirtyRect];
    if (!self.image) {
        return;
    }

    NSRect bounds = self.bounds;
    if (NSIsEmptyRect(bounds)) {
        return;
    }

    [NSGraphicsContext saveGraphicsState];
    CGFloat radius = MAX(0, self.cornerRadius);
    if (radius > 0) {
        NSBezierPath *clipPath = [NSBezierPath bezierPathWithRoundedRect:bounds xRadius:radius yRadius:radius];
        [clipPath addClip];
    }

    [self.image drawInRect:bounds fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:1.0 respectFlipped:YES hints:@{NSImageHintInterpolation: @(NSImageInterpolationHigh)}];
    [NSGraphicsContext restoreGraphicsState];
}

- (void)layout {
    [super layout];
    if (self.closable) {
        self.closeButton.frame = NSMakeRect(MAX(0, self.bounds.size.width - kImageOverlayCloseSize - kImageOverlayCloseMargin), MAX(0, self.bounds.size.height - kImageOverlayCloseSize - kImageOverlayCloseMargin), kImageOverlayCloseSize, kImageOverlayCloseSize);
    }
}

- (void)onCloseButtonClicked:(id)sender {
    if (self.name.length == 0) {
        return;
    }
    char *nameCopy = strdup([self.name UTF8String]);
    if (!nameCopy) {
        return;
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        overlayRequestCloseCallbackCGO(nameCopy);
        free(nameCopy);
    });
}

- (void)scrollWheel:(NSEvent *)event {
    CGFloat delta = event.scrollingDeltaY != 0 ? event.scrollingDeltaY : event.deltaY;
    NSWindow *window = self.window;
    if (!window || delta == 0) {
        [super scrollWheel:event];
        return;
    }

    NSRect frame = window.frame;
    if (frame.size.width <= 0 || frame.size.height <= 0) {
        return;
    }

    CGFloat factor = delta > 0 ? kImageOverlayWheelZoomStep : 1.0 / kImageOverlayWheelZoomStep;
    CGFloat aspectRatio = frame.size.width / frame.size.height;
    CGFloat width = MAX(kImageOverlayMinZoomSize, frame.size.width * factor);
    CGFloat height = width / aspectRatio;
    if (height < kImageOverlayMinZoomSize) {
        height = kImageOverlayMinZoomSize;
        width = height * aspectRatio;
    }

    NSPoint point = event.locationInWindow;
    CGFloat anchorX = MIN(1.0, MAX(0.0, point.x / frame.size.width));
    CGFloat anchorY = MIN(1.0, MAX(0.0, point.y / frame.size.height));
    NSRect nextFrame = NSMakeRect(NSMinX(frame) + point.x - width * anchorX,
                                  NSMinY(frame) + point.y - height * anchorY,
                                  width,
                                  height);
    [window setFrame:nextFrame display:YES];
}

- (void)destroy {
    [self removeFromSuperview];
}

- (void)dealloc {
    [self destroy];
    self.name = nil;
    self.image = nil;
    self.closeButton = nil;
    [super dealloc];
}

@end

static NSImage *WoxImageOverlayImageFromBytes(unsigned char *data, int length) {
    if (!data || length <= 0) {
        return nil;
    }
    NSData *imageData = [NSData dataWithBytes:data length:(NSUInteger)length];
    return [[[NSImage alloc] initWithData:imageData] autorelease];
}

void *ImageOverlayCreateView(char *name, unsigned char *imageData, int imageLen, char *imageFilePath, float cornerRadius, bool closable) {
    NSImage *image = nil;
    if (imageFilePath) {
        NSString *path = [NSString stringWithUTF8String:imageFilePath];
        if (path.length > 0) {
            image = [[[NSImage alloc] initWithContentsOfFile:path] autorelease];
        }
    }
    if (!image) {
        image = WoxImageOverlayImageFromBytes(imageData, imageLen);
    }
    if (!image) {
        return NULL;
    }

    NSString *viewName = name ? [NSString stringWithUTF8String:name] : @"";
    WoxImageOverlayView *view = [[WoxImageOverlayView alloc] initWithName:viewName image:image cornerRadius:cornerRadius closable:closable ? YES : NO];
    return view;
}

void ImageOverlayDestroyView(void *viewHandle) {
    if (!viewHandle) {
        return;
    }
    WoxImageOverlayView *view = (WoxImageOverlayView *)viewHandle;
    [view destroy];
    [view release];
}
