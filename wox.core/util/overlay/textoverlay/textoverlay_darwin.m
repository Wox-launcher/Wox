#import <Cocoa/Cocoa.h>
#import <Dispatch/Dispatch.h>
#import <stdbool.h>
#include <stdlib.h>
#include <string.h>

extern bool overlayClickCallbackCGO(char *name);
extern void overlayRequestCloseCallbackCGO(char *name);

typedef struct {
    void *handle;
    float width;
    float height;
} TextOverlayAttachment;

static const CGFloat kTextDefaultContentWidth = 364.0;
static const CGFloat kTextMinContentWidth = 64.0;
static const CGFloat kTextIconGap = 8.0;
static const CGFloat kTextTooltipGap = 8.0;
static const CGFloat kTextCopyButtonSize = 28.0;
static const CGFloat kTextCopyButtonGap = 8.0;
static const CGFloat kTextCloseButtonSize = 20.0;
static const CGFloat kTextCloseButtonGap = 8.0;
static const NSStringDrawingOptions kTextDrawingOptions = NSStringDrawingUsesLineFragmentOrigin | NSStringDrawingUsesFontLeading;

static CGFloat WoxTextOverlayDefaultFontSize(void) {
    return [NSFont systemFontSize];
}

@interface WoxTextOverlayView : NSView
@property(nonatomic, copy) NSString *name;
@property(nonatomic, copy) NSString *message;
@property(nonatomic, assign) BOOL loading;
@property(nonatomic, assign) BOOL closable;
@property(nonatomic, assign) BOOL centerContent;
@property(nonatomic, assign) BOOL showCopyButton;
@property(nonatomic, assign) NSInteger autoCloseSeconds;
@property(nonatomic, assign) CGFloat fontSize;
@property(nonatomic, assign) CGFloat iconSize;
@property(nonatomic, assign) CGFloat tooltipIconSize;
@property(nonatomic, strong) NSImage *icon;
@property(nonatomic, strong) NSImage *tooltipIcon;
@property(nonatomic, copy) NSString *copyButtonTooltip;
@property(nonatomic, copy) NSString *copyButtonSuccessTooltip;
@property(nonatomic, strong) NSAttributedString *messageText;
@property(nonatomic, assign) NSRect messageRect;
@property(nonatomic, strong) NSImageView *iconView;
@property(nonatomic, strong) NSImageView *tooltipIconView;
@property(nonatomic, strong) NSProgressIndicator *loadingIndicator;
@property(nonatomic, strong) NSButton *closeButton;
@property(nonatomic, strong) NSButton *copyButton;
@property(nonatomic, strong) NSTimer *copyFeedbackTimer;
@property(nonatomic, strong) NSTimer *autoCloseTimer;
@end

@implementation WoxTextOverlayView

- (instancetype)initWithName:(NSString *)name
                     message:(NSString *)message
                        icon:(NSImage *)icon
                     loading:(BOOL)loading
                    closable:(BOOL)closable
               centerContent:(BOOL)centerContent
                    fontSize:(CGFloat)fontSize
                    iconSize:(CGFloat)iconSize
                 tooltipIcon:(NSImage *)tooltipIcon
             tooltipIconSize:(CGFloat)tooltipIconSize
              showCopyButton:(BOOL)showCopyButton
           copyButtonTooltip:(NSString *)copyButtonTooltip
    copyButtonSuccessTooltip:(NSString *)copyButtonSuccessTooltip
             autoCloseSeconds:(NSInteger)autoCloseSeconds
                       frame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (!self) {
        return nil;
    }

    self.name = name ?: @"";
    self.message = message ?: @"";
    self.icon = icon;
    self.loading = loading;
    self.closable = closable;
    self.centerContent = centerContent;
    self.showCopyButton = showCopyButton;
    self.autoCloseSeconds = autoCloseSeconds;
    self.fontSize = fontSize > 0 ? fontSize : WoxTextOverlayDefaultFontSize();
    self.iconSize = iconSize > 0 ? iconSize : 24.0;
    self.tooltipIcon = tooltipIcon;
    self.tooltipIconSize = tooltipIconSize > 0 ? tooltipIconSize : 18.0;
    self.copyButtonTooltip = copyButtonTooltip ?: @"";
    self.copyButtonSuccessTooltip = copyButtonSuccessTooltip ?: self.copyButtonTooltip;
    self.wantsLayer = YES;
    self.layer.backgroundColor = [NSColor clearColor].CGColor;
    NSDictionary *attrs = @{
        NSFontAttributeName: [NSFont systemFontOfSize:self.fontSize],
        NSForegroundColorAttributeName: [NSColor whiteColor],
    };
    self.messageText = [[[NSAttributedString alloc] initWithString:self.message attributes:attrs] autorelease];

    NSImageView *iconView = [[NSImageView alloc] initWithFrame:NSZeroRect];
    self.iconView = iconView;
    [iconView release];
    self.iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
    self.iconView.image = icon;
    self.iconView.hidden = (icon == nil || loading);
    [self addSubview:self.iconView];

    NSProgressIndicator *loadingIndicator = [[NSProgressIndicator alloc] initWithFrame:NSZeroRect];
    self.loadingIndicator = loadingIndicator;
    [loadingIndicator release];
    self.loadingIndicator.style = NSProgressIndicatorStyleSpinning;
    self.loadingIndicator.indeterminate = YES;
    self.loadingIndicator.displayedWhenStopped = NO;
    self.loadingIndicator.hidden = !loading;
    if (@available(macOS 10.14, *)) {
        self.loadingIndicator.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    }
    if (loading) {
        [self.loadingIndicator startAnimation:nil];
    }
    [self addSubview:self.loadingIndicator];

    NSImageView *tooltipIconView = [[NSImageView alloc] initWithFrame:NSZeroRect];
    self.tooltipIconView = tooltipIconView;
    [tooltipIconView release];
    self.tooltipIconView.imageScaling = NSImageScaleProportionallyUpOrDown;
    self.tooltipIconView.image = tooltipIcon;
    self.tooltipIconView.hidden = (tooltipIcon == nil);
    [self addSubview:self.tooltipIconView];

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
    self.closeButton.layer.cornerRadius = kTextCloseButtonSize / 2.0;
    self.closeButton.target = self;
    self.closeButton.action = @selector(onCloseButtonClicked:);
    NSMutableAttributedString *closeTitle = [[NSMutableAttributedString alloc] initWithString:@"×"];
    [closeTitle addAttribute:NSForegroundColorAttributeName value:[NSColor whiteColor] range:NSMakeRange(0, closeTitle.length)];
    [closeTitle addAttribute:NSFontAttributeName value:[NSFont systemFontOfSize:16 weight:NSFontWeightBold] range:NSMakeRange(0, closeTitle.length)];
    [self.closeButton setAttributedTitle:closeTitle];
    [closeTitle release];
    [self addSubview:self.closeButton];

    NSButton *copyButton = [[NSButton alloc] initWithFrame:NSZeroRect];
    self.copyButton = copyButton;
    [copyButton release];
    self.copyButton.bezelStyle = NSBezelStyleRegularSquare;
    self.copyButton.buttonType = NSButtonTypeMomentaryLight;
    self.copyButton.bordered = NO;
    self.copyButton.focusRingType = NSFocusRingTypeNone;
    self.copyButton.hidden = !showCopyButton;
    self.copyButton.toolTip = self.copyButtonTooltip;
    self.copyButton.wantsLayer = YES;
    self.copyButton.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.14].CGColor;
    self.copyButton.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:0.24].CGColor;
    self.copyButton.layer.borderWidth = 1;
    self.copyButton.layer.cornerRadius = 6;
    self.copyButton.target = self;
    self.copyButton.action = @selector(onCopyButtonClicked:);
    if (@available(macOS 11.0, *)) {
        self.copyButton.image = [NSImage imageWithSystemSymbolName:@"doc.on.doc" accessibilityDescription:@"Copy"];
        self.copyButton.imagePosition = NSImageOnly;
        self.copyButton.contentTintColor = [NSColor whiteColor];
    } else {
        [self.copyButton setTitle:@"Copy"];
        [self.copyButton setFont:[NSFont systemFontOfSize:11 weight:NSFontWeightSemibold]];
    }
    [self addSubview:self.copyButton];
    [self startAutoCloseTimerWithSeconds:self.autoCloseSeconds];

    return self;
}

- (BOOL)isFlipped {
    return YES;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES;
}

- (void)mouseUp:(NSEvent *)event {
    if (self.showCopyButton || self.name.length == 0) {
        return;
    }
    overlayClickCallbackCGO((char *)[self.name UTF8String]);
}

- (void)layout {
    [super layout];

    CGFloat width = self.bounds.size.width;
    CGFloat height = self.bounds.size.height;
    CGFloat leadingWidth = (self.loading || self.icon) ? self.iconSize : 0;
    CGFloat leadingGap = leadingWidth > 0 ? kTextIconGap : 0;
    CGFloat tooltipWidth = self.tooltipIcon ? self.tooltipIconSize : 0;
    CGFloat tooltipGap = tooltipWidth > 0 ? kTextTooltipGap : 0;
    CGFloat copyReserve = self.showCopyButton ? (kTextCopyButtonSize + kTextCopyButtonGap) : 0;
    CGFloat closeReserve = self.closable ? (kTextCloseButtonSize + kTextCloseButtonGap) : 0;
    CGFloat contentAreaWidth = MAX(1, width - closeReserve);
    CGFloat maxTextWidth = MAX(1, contentAreaWidth - leadingWidth - leadingGap - tooltipWidth - tooltipGap);

    NSRect textBounds = [self.messageText boundingRectWithSize:NSMakeSize(maxTextWidth, CGFLOAT_MAX) options:kTextDrawingOptions];
    CGFloat renderedTextWidth = MIN(maxTextWidth, MAX(1, ceil(textBounds.size.width)));
    CGFloat textLayoutWidth = self.centerContent ? renderedTextWidth : maxTextWidth;
    CGFloat textHeight = MAX(1, ceil(textBounds.size.height));
    CGFloat rowHeight = MAX(MAX(textHeight, leadingWidth), self.closable ? kTextCloseButtonSize : 0);
    CGFloat rowY = MAX(0, (height - copyReserve - rowHeight) / 2.0);

    CGFloat groupWidth = leadingWidth + leadingGap + textLayoutWidth + tooltipGap + tooltipWidth;
    CGFloat x = self.centerContent ? MAX(0, (contentAreaWidth - groupWidth) / 2.0) : 0;

    if (self.loading) {
        self.loadingIndicator.frame = NSMakeRect(x, rowY + (rowHeight - self.iconSize) / 2.0, self.iconSize, self.iconSize);
    } else if (self.icon) {
        self.iconView.frame = NSMakeRect(x, rowY + (rowHeight - self.iconSize) / 2.0, self.iconSize, self.iconSize);
    }

    CGFloat textX = x + leadingWidth + leadingGap;
    self.messageRect = NSMakeRect(textX, rowY + (rowHeight - textHeight) / 2.0, textLayoutWidth, textHeight);
    self.needsDisplay = YES;

    if (self.tooltipIcon) {
        self.tooltipIconView.frame = NSMakeRect(textX + textLayoutWidth + tooltipGap, rowY + (rowHeight - self.tooltipIconSize) / 2.0, self.tooltipIconSize, self.tooltipIconSize);
    }

    if (self.closable) {
        self.closeButton.frame = NSMakeRect(MAX(0, width - kTextCloseButtonSize), 0, kTextCloseButtonSize, kTextCloseButtonSize);
    }

    if (self.showCopyButton) {
        self.copyButton.frame = NSMakeRect(MAX(0, width - kTextCopyButtonSize), MAX(0, height - kTextCopyButtonSize), kTextCopyButtonSize, kTextCopyButtonSize);
    }
}

- (void)drawRect:(NSRect)dirtyRect {
    [super drawRect:dirtyRect];
    if (self.messageText.length == 0 || NSIsEmptyRect(self.messageRect)) {
        return;
    }
    [self.messageText drawWithRect:self.messageRect options:kTextDrawingOptions];
}

- (void)onCopyButtonClicked:(id)sender {
    if (self.name.length == 0) {
        return;
    }
    if (overlayClickCallbackCGO((char *)[self.name UTF8String])) {
        [self showCopyFeedback];
    }
}

- (void)onCloseButtonClicked:(id)sender {
    [self requestClose];
}

- (void)requestClose {
    if (self.name.length == 0) {
        return;
    }
    [self.autoCloseTimer invalidate];
    self.autoCloseTimer = nil;
    char *nameCopy = strdup([self.name UTF8String]);
    if (!nameCopy) {
        return;
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        overlayRequestCloseCallbackCGO(nameCopy);
        free(nameCopy);
    });
}

- (BOOL)isCursorInsideOverlayWindow {
    NSWindow *window = self.window;
    if (!window) {
        return NO;
    }
    return NSPointInRect([NSEvent mouseLocation], window.frame);
}

- (void)startAutoCloseTimerWithSeconds:(NSInteger)seconds {
    [self.autoCloseTimer invalidate];
    self.autoCloseTimer = nil;
    if (seconds <= 0) {
        return;
    }
    self.autoCloseTimer = [NSTimer scheduledTimerWithTimeInterval:(NSTimeInterval)seconds target:self selector:@selector(onAutoCloseTimerFired:) userInfo:nil repeats:NO];
}

- (void)onAutoCloseTimerFired:(NSTimer *)timer {
    if ([self isCursorInsideOverlayWindow]) {
        // Text overlays own hover-delayed notification close behavior because their native
        // attachment receives the mouse events, not the base overlay content view.
        self.autoCloseTimer = [NSTimer scheduledTimerWithTimeInterval:0.25 target:self selector:@selector(onPendingAutoCloseTimerFired:) userInfo:nil repeats:YES];
        return;
    }
    [self requestClose];
}

- (void)onPendingAutoCloseTimerFired:(NSTimer *)timer {
    if ([self isCursorInsideOverlayWindow]) {
        return;
    }
    [self requestClose];
}

- (void)showCopyFeedback {
    self.copyButton.toolTip = self.copyButtonSuccessTooltip.length > 0 ? self.copyButtonSuccessTooltip : self.copyButtonTooltip;
    if (@available(macOS 11.0, *)) {
        self.copyButton.image = [NSImage imageWithSystemSymbolName:@"checkmark" accessibilityDescription:@"Copied"];
    } else {
        [self.copyButton setTitle:@"OK"];
    }
    self.copyButton.layer.backgroundColor = [NSColor colorWithCalibratedRed:0.18 green:0.44 blue:0.32 alpha:0.86].CGColor;
    [self.copyFeedbackTimer invalidate];
    self.copyFeedbackTimer = [NSTimer scheduledTimerWithTimeInterval:1.2 target:self selector:@selector(resetCopyFeedback:) userInfo:nil repeats:NO];
}

- (void)resetCopyFeedback:(NSTimer *)timer {
    self.copyButton.toolTip = self.copyButtonTooltip;
    if (@available(macOS 11.0, *)) {
        self.copyButton.image = [NSImage imageWithSystemSymbolName:@"doc.on.doc" accessibilityDescription:@"Copy"];
    } else {
        [self.copyButton setTitle:@"Copy"];
    }
    self.copyButton.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.14].CGColor;
}

- (void)destroy {
    [self.copyFeedbackTimer invalidate];
    self.copyFeedbackTimer = nil;
    [self.autoCloseTimer invalidate];
    self.autoCloseTimer = nil;
    [self.loadingIndicator stopAnimation:nil];
    [self removeFromSuperview];
}

- (void)dealloc {
    [self destroy];
    self.name = nil;
    self.message = nil;
    self.icon = nil;
    self.tooltipIcon = nil;
    self.copyButtonTooltip = nil;
    self.copyButtonSuccessTooltip = nil;
    self.messageText = nil;
    self.iconView = nil;
    self.tooltipIconView = nil;
    self.loadingIndicator = nil;
    self.closeButton = nil;
    self.copyButton = nil;
    [super dealloc];
}

@end

static NSImage *WoxTextOverlayImageFromBytes(unsigned char *data, int length) {
    if (!data || length <= 0) {
        return nil;
    }
    NSData *imageData = [NSData dataWithBytes:data length:(NSUInteger)length];
    return [[[NSImage alloc] initWithData:imageData] autorelease];
}

static NSSize WoxTextOverlayMeasure(NSString *message,
                                    BOOL loading,
                                    BOOL hasIcon,
                                    BOOL hasTooltip,
                                    BOOL showCopyButton,
                                    BOOL closable,
                                    CGFloat fontSize,
                                    CGFloat iconSize,
                                    CGFloat tooltipIconSize,
                                    CGFloat windowWidth,
                                    CGFloat minWindowWidth,
                                    CGFloat maxWindowWidth,
                                    CGFloat windowHeight,
                                    CGFloat maxWindowHeight) {
    CGFloat closeReserve = closable ? (kTextCloseButtonSize + kTextCloseButtonGap) : 0.0;
    CGFloat leadingWidth = (loading || hasIcon) ? iconSize : 0;
    CGFloat leadingGap = leadingWidth > 0 ? kTextIconGap : 0;
    CGFloat tooltipWidth = hasTooltip ? tooltipIconSize : 0;
    CGFloat tooltipGap = tooltipWidth > 0 ? kTextTooltipGap : 0;
    CGFloat chromeWidth = 36.0;
    CGFloat chromeHeight = 24.0;

    NSDictionary *attrs = @{NSFontAttributeName: [NSFont systemFontOfSize:fontSize]};
    NSRect naturalTextRect = [message boundingRectWithSize:NSMakeSize(CGFLOAT_MAX, CGFLOAT_MAX)
                                                   options:kTextDrawingOptions
                                                attributes:attrs];
    CGFloat naturalTextWidth = MAX(1, ceil(naturalTextRect.size.width));
    CGFloat naturalContentWidth = leadingWidth + leadingGap + naturalTextWidth + tooltipGap + tooltipWidth + closeReserve;
    // Use the default 400-point window cap only when the caller does not provide an explicit maximum.
    CGFloat maxContentWidth = maxWindowWidth > 0 ? MAX(1, maxWindowWidth - chromeWidth) : kTextDefaultContentWidth;
    CGFloat contentWidth = MIN(MAX(naturalContentWidth, kTextMinContentWidth), maxContentWidth);

    if (windowWidth > 0) {
        contentWidth = MAX(1, windowWidth - chromeWidth);
    }
    if (minWindowWidth > 0) {
        contentWidth = MAX(contentWidth, MAX(1, minWindowWidth - chromeWidth));
    }

    CGFloat textWidth = MAX(1, contentWidth - leadingWidth - leadingGap - tooltipWidth - tooltipGap - closeReserve);
    NSRect wrappedTextRect = [message boundingRectWithSize:NSMakeSize(textWidth, CGFLOAT_MAX)
                                                   options:kTextDrawingOptions
                                                attributes:attrs];
    CGFloat textHeight = MAX(1, ceil(wrappedTextRect.size.height));
    CGFloat copyReserve = showCopyButton ? (kTextCopyButtonSize + kTextCopyButtonGap) : 0;
    CGFloat contentHeight = MAX(MAX(textHeight, leadingWidth), closable ? kTextCloseButtonSize : 0) + copyReserve;

    if (windowHeight > 0) {
        contentHeight = MAX(1, windowHeight - chromeHeight);
    } else if (maxWindowHeight > 0) {
        contentHeight = MIN(contentHeight, MAX(1, maxWindowHeight - chromeHeight));
    }

    return NSMakeSize(MAX(1, ceil(contentWidth)), MAX(1, ceil(contentHeight)));
}

TextOverlayAttachment TextOverlayCreateView(char *name,
                                            char *message,
                                            unsigned char *iconData,
                                            int iconLen,
                                            bool loading,
                                            bool centerContent,
                                            float fontSize,
                                            float iconSize,
                                            char *tooltip,
                                            unsigned char *tooltipIconData,
                                            int tooltipIconLen,
                                            float tooltipIconSize,
                                            bool showCopyButton,
                                            char *copyButtonTooltip,
                                            char *copyButtonSuccessTooltip,
                                            bool closable,
                                            int autoCloseSeconds,
                                            float windowWidth,
                                            float minWindowWidth,
                                            float maxWindowWidth,
                                            float windowHeight,
                                            float maxWindowHeight) {
    NSString *viewName = name ? [NSString stringWithUTF8String:name] : @"";
    NSString *viewMessage = message ? [NSString stringWithUTF8String:message] : @"";
    NSString *viewTooltip = tooltip ? [NSString stringWithUTF8String:tooltip] : @"";
    NSString *copyTooltip = copyButtonTooltip ? [NSString stringWithUTF8String:copyButtonTooltip] : @"";
    NSString *copySuccessTooltip = copyButtonSuccessTooltip ? [NSString stringWithUTF8String:copyButtonSuccessTooltip] : @"";

    CGFloat resolvedFontSize = fontSize > 0 ? fontSize : WoxTextOverlayDefaultFontSize();
    CGFloat resolvedIconSize = iconSize > 0 ? iconSize : 24.0;
    CGFloat resolvedTooltipIconSize = tooltipIconSize > 0 ? tooltipIconSize : 18.0;

    NSImage *icon = WoxTextOverlayImageFromBytes(iconData, iconLen);
    NSImage *tooltipImage = WoxTextOverlayImageFromBytes(tooltipIconData, tooltipIconLen);
    if (!tooltipImage && viewTooltip.length > 0) {
        tooltipImage = [NSImage imageNamed:NSImageNameInfo];
    }

    NSSize contentSize = WoxTextOverlayMeasure(viewMessage,
                                               loading,
                                               icon != nil,
                                               tooltipImage != nil,
                                               showCopyButton,
                                               closable,
                                               resolvedFontSize,
                                               resolvedIconSize,
                                               resolvedTooltipIconSize,
                                               windowWidth,
                                               minWindowWidth,
                                               maxWindowWidth,
                                               windowHeight,
                                               maxWindowHeight);

    WoxTextOverlayView *view = [[WoxTextOverlayView alloc] initWithName:viewName
                                                                 message:viewMessage
                                                                    icon:icon
                                                                 loading:loading
                                                                closable:closable
                                                           centerContent:centerContent
                                                                fontSize:resolvedFontSize
                                                                iconSize:resolvedIconSize
                                                             tooltipIcon:tooltipImage
                                                         tooltipIconSize:resolvedTooltipIconSize
                                                          showCopyButton:showCopyButton
                                                       copyButtonTooltip:copyTooltip
                                                copyButtonSuccessTooltip:copySuccessTooltip
                                                        autoCloseSeconds:autoCloseSeconds
                                                                   frame:NSMakeRect(0, 0, contentSize.width, contentSize.height)];
    TextOverlayAttachment result;
    result.handle = view;
    result.width = (float)contentSize.width;
    result.height = (float)contentSize.height;
    return result;
}

void TextOverlayDestroyView(void *viewHandle) {
    if (!viewHandle) {
        return;
    }
    WoxTextOverlayView *view = (WoxTextOverlayView *)viewHandle;
    [view destroy];
    [view release];
}
