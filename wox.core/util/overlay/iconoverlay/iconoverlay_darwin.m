#import <Cocoa/Cocoa.h>
#import <Dispatch/Dispatch.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

extern bool overlayClickCallbackCGO(char *name);

typedef struct {
    void *handle;
    float width;
    float height;
} IconOverlayAttachment;

@interface WoxIconOverlayView : NSView
@property(nonatomic, copy) NSString *name;
@property(nonatomic, strong) NSImage *icon;
@property(nonatomic, assign) CGFloat iconSize;
@property(nonatomic, strong) NSImageView *iconView;
@property(nonatomic, assign) BOOL pressed;
@end

@implementation WoxIconOverlayView

- (instancetype)initWithName:(NSString *)name icon:(NSImage *)icon iconSize:(CGFloat)iconSize frame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (!self) {
        return nil;
    }
    self.name = name ?: @"";
    self.icon = icon;
    self.iconSize = iconSize > 0 ? iconSize : MIN(frame.size.width, frame.size.height);
    self.wantsLayer = YES;
    self.layer.backgroundColor = [NSColor clearColor].CGColor;

    NSImageView *iconView = [[NSImageView alloc] initWithFrame:NSZeroRect];
    self.iconView = iconView;
    [iconView release];
    self.iconView.image = icon;
    self.iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
    self.iconView.imageAlignment = NSImageAlignCenter;
    self.iconView.wantsLayer = YES;
    self.iconView.layer.backgroundColor = [NSColor clearColor].CGColor;
    [self addSubview:self.iconView];
    return self;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES;
}

- (NSView *)hitTest:(NSPoint)point {
    return NSPointInRect(point, self.bounds) ? self : nil;
}

- (void)layout {
    [super layout];
    CGFloat size = MIN(self.iconSize, MIN(self.bounds.size.width, self.bounds.size.height));
    self.iconView.frame = NSMakeRect((self.bounds.size.width - size) / 2.0,
                                     (self.bounds.size.height - size) / 2.0,
                                     size,
                                     size);
}

- (void)mouseDown:(NSEvent *)event {
    self.pressed = YES;
}

- (void)mouseUp:(NSEvent *)event {
    BOOL shouldClick = self.pressed;
    self.pressed = NO;
    if (!shouldClick || self.name.length == 0) {
        return;
    }
    char *nameCopy = strdup([self.name UTF8String]);
    if (!nameCopy) {
        return;
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        overlayClickCallbackCGO(nameCopy);
        free(nameCopy);
    });
}

- (void)destroy {
    [self removeFromSuperview];
}

- (void)dealloc {
    [self destroy];
    self.name = nil;
    self.icon = nil;
    self.iconView = nil;
    [super dealloc];
}

@end

static NSImage *WoxIconOverlayImageFromBytes(unsigned char *data, int length) {
    if (!data || length <= 0) {
        return nil;
    }
    NSData *imageData = [NSData dataWithBytes:data length:(NSUInteger)length];
    return [[[NSImage alloc] initWithData:imageData] autorelease];
}

IconOverlayAttachment IconOverlayCreateView(char *name, unsigned char *iconData, int iconLen, float width, float height, float iconSize) {
    NSImage *icon = WoxIconOverlayImageFromBytes(iconData, iconLen);
    if (!icon) {
        IconOverlayAttachment empty = {0};
        return empty;
    }

    CGFloat resolvedWidth = width > 0 ? width : 1.0;
    CGFloat resolvedHeight = height > 0 ? height : resolvedWidth;
    CGFloat resolvedIconSize = iconSize > 0 ? iconSize : MIN(resolvedWidth, resolvedHeight);
    NSString *viewName = name ? [NSString stringWithUTF8String:name] : @"";
    WoxIconOverlayView *view = [[WoxIconOverlayView alloc] initWithName:viewName
                                                                     icon:icon
                                                                 iconSize:resolvedIconSize
                                                                     frame:NSMakeRect(0, 0, resolvedWidth, resolvedHeight)];
    IconOverlayAttachment result;
    result.handle = view;
    result.width = (float)resolvedWidth;
    result.height = (float)resolvedHeight;
    return result;
}

void IconOverlayDestroyView(void *viewHandle) {
    if (!viewHandle) {
        return;
    }
    WoxIconOverlayView *view = (WoxIconOverlayView *)viewHandle;
    [view destroy];
    [view release];
}
