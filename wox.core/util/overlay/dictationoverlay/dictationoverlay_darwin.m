#import <Cocoa/Cocoa.h>
#import <Dispatch/Dispatch.h>
#include <math.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

extern void overlayRequestCloseCallbackCGO(char *name);

static const CGFloat kDictationCloseSize = 20.0;
static const CGFloat kDictationCloseGap = 8.0;

@interface DictationOverlayView : NSView
@property(nonatomic, copy) NSString *name;
@property(nonatomic, assign) BOOL closable;
@property(nonatomic, assign) BOOL active;
@property(nonatomic, assign) CGFloat phase;
@property(nonatomic, strong) NSTimer *animationTimer;
@property(nonatomic, strong) NSButton *closeButton;
- (instancetype)initWithName:(NSString *)name closable:(BOOL)closable;
- (void)setVoiceActive:(BOOL)active;
- (void)stopAnimation;
@end

@implementation DictationOverlayView
- (instancetype)initWithName:(NSString *)name closable:(BOOL)closable {
    self = [super initWithFrame:NSZeroRect];
    if (!self) {
        return nil;
    }
    self.name = name ?: @"";
    self.closable = closable;
    self.wantsLayer = YES;
    self.layer.backgroundColor = [NSColor clearColor].CGColor;
    NSButton *closeButton = [[NSButton alloc] initWithFrame:NSZeroRect];
    self.closeButton = closeButton;
    [closeButton release];
    self.closeButton.bezelStyle = NSBezelStyleRegularSquare;
    self.closeButton.buttonType = NSButtonTypeMomentaryLight;
    self.closeButton.bordered = NO;
    self.closeButton.focusRingType = NSFocusRingTypeNone;
    self.closeButton.hidden = !closable;
    self.closeButton.wantsLayer = YES;
    self.closeButton.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.12].CGColor;
    self.closeButton.layer.cornerRadius = kDictationCloseSize / 2.0;
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

- (BOOL)isOpaque {
    return NO;
}

- (void)layout {
    [super layout];
    if (self.closable) {
        self.closeButton.frame = NSMakeRect(MAX(0, self.bounds.size.width - kDictationCloseSize), (self.bounds.size.height - kDictationCloseSize) / 2.0, kDictationCloseSize, kDictationCloseSize);
    }
}

- (void)setVoiceActive:(BOOL)active {
    if (_active == active) return;
    _active = active;
    if (active) {
        if (!self.animationTimer) {
            self.animationTimer = [NSTimer scheduledTimerWithTimeInterval:(1.0 / 30.0) target:self selector:@selector(onAnimationTimer:) userInfo:nil repeats:YES];
        }
    } else {
        [self stopAnimation];
    }
    [self setNeedsDisplay:YES];
}

- (void)stopAnimation {
    [self.animationTimer invalidate];
    self.animationTimer = nil;
    self.phase = 0;
}

- (void)onAnimationTimer:(NSTimer *)timer {
    self.phase += 0.32;
    [self setNeedsDisplay:YES];
}

- (void)drawRect:(NSRect)dirtyRect {
    NSRect bounds = self.bounds;
    NSRectFillUsingOperation(bounds, NSCompositingOperationClear);
    CGFloat closeReserve = self.closable ? (kDictationCloseSize + kDictationCloseGap) : 0;
    bounds.size.width = MAX(1, bounds.size.width - closeReserve);

    NSInteger barCount = 7;
    CGFloat gap = 5;
    CGFloat barWidth = 4;
    CGFloat totalWidth = barCount * barWidth + (barCount - 1) * gap;
    CGFloat startX = bounds.origin.x + (bounds.size.width - totalWidth) / 2.0;
    CGFloat centerY = NSMidY(bounds);
    CGFloat maxHeight = MAX(8, bounds.size.height - 2);
    CGFloat idleScales[] = {0.32, 0.46, 0.36, 0.56, 0.36, 0.46, 0.32};

    [[NSColor colorWithWhite:1.0 alpha:0.9] setFill];
    for (NSInteger i = 0; i < barCount; i++) {
        CGFloat scale = idleScales[i];
        if (self.active) {
            scale = 0.28 + 0.72 * (0.5 + 0.5 * sin(self.phase + (CGFloat)i * 0.85));
        }
        CGFloat barHeight = MAX(5, maxHeight * scale);
        CGFloat x = startX + (barWidth + gap) * i;
        CGFloat y = centerY - barHeight / 2.0;
        NSBezierPath *path = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(x, y, barWidth, barHeight) xRadius:barWidth / 2.0 yRadius:barWidth / 2.0];
        [path fill];
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

- (void)dealloc {
    [self stopAnimation];
    self.name = nil;
    self.closeButton = nil;
    [super dealloc];
}
@end

void* DictationOverlayCreateView(char* name, bool closable) {
    @autoreleasepool {
        NSString *viewName = name ? [NSString stringWithUTF8String:name] : @"";
        DictationOverlayView *view = [[DictationOverlayView alloc] initWithName:viewName closable:closable ? YES : NO];
        view.hidden = NO;
        return view;
    }
}

void DictationOverlaySetActive(void* rawView, bool active) {
    @autoreleasepool {
        if (!rawView) return;
        DictationOverlayView *view = (DictationOverlayView *)rawView;
        [view setVoiceActive:active ? YES : NO];
    }
}

void DictationOverlayDestroyView(void* rawView) {
    @autoreleasepool {
        if (!rawView) return;
        DictationOverlayView *view = (DictationOverlayView *)rawView;
        [view stopAnimation];
        [view removeFromSuperview];
        [view release];
    }
}
