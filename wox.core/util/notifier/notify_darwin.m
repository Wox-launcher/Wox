#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <CoreGraphics/CoreGraphics.h>

static const CGFloat kWindowWidth = 520;
static const NSInteger kMaxTextLines = 3;
static const NSTimeInterval kDefaultCloseSeconds = 3.0;

static const CGFloat kTextLeftPad = 20;
static const CGFloat kTextVertPad = 12;
static const CGFloat kTextRightGapClose = 10;

static const CGFloat kIconSize = 20;
static const CGFloat kIconGap = 12;

static const CGFloat kClosePad = 10;
static const CGFloat kCloseSize = 24;

static const CGFloat kCopyGap = 6;
static const CGFloat kCopyHeight = 24;
static const CGFloat kCopyWidth = 72;

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

@interface NotificationWindow : NSPanel
@property(nonatomic, strong) NSTimer *closeTimer;
@property(nonatomic, strong) NSButton *closeButton;
@property(nonatomic, strong) NSButton *clipboardButton;
@property(nonatomic, strong) NSImageView *iconView;
@property(nonatomic, strong) NSTextView *messageView;
@property(nonatomic, strong) NSVisualEffectView *backgroundView;
@property(nonatomic, copy) NSString *fullMessage;
@property(nonatomic, assign) BOOL showCopyLink;
@property(nonatomic, assign) NSTimeInterval remainingTime;
@property (nonatomic, assign) NSPoint initialLocation;
- (void)setCornerRadius:(CGFloat)radius;
- (void)updateWithMessage:(NSString *)message icon:(NSImage *)icon;
@end

static NotificationWindow *gNotificationWindow = nil;

@implementation NotificationWindow

- (instancetype)initWithContentRect:(NSRect)contentRect styleMask:(NSWindowStyleMask)style backing:(NSBackingStoreType)backingStoreType defer:(BOOL)flag {
  self = [super initWithContentRect:contentRect styleMask:style backing:backingStoreType defer:flag];
  if (self) {
    [self setBackgroundColor:[NSColor clearColor]];
    [self setOpaque:NO];
    [self setHasShadow:YES];

    self.fullMessage = @"";
    self.showCopyLink = NO;

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

    NSImageView *iconView = [[NSImageView alloc] initWithFrame:NSZeroRect];
    iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
    iconView.hidden = YES;
    [self.contentView addSubview:iconView];
    self.iconView = iconView;

    NSTextView *messageView = [[NSTextView alloc] initWithFrame:NSZeroRect];
    messageView.editable = NO;
    messageView.selectable = NO;
    messageView.drawsBackground = NO;
    messageView.textContainerInset = NSZeroSize;
    messageView.textContainer.lineFragmentPadding = 0;
    if (@available(macOS 10.14, *)) {
      messageView.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    }
    if (messageView.textContainer) {
      messageView.textContainer.widthTracksTextView = YES;
      messageView.textContainer.heightTracksTextView = YES;
      messageView.textContainer.lineBreakMode = NSLineBreakByTruncatingTail;
      if ([messageView.textContainer respondsToSelector:@selector(setMaximumNumberOfLines:)]) {
        messageView.textContainer.maximumNumberOfLines = kMaxTextLines;
      }
    }
    [self.contentView addSubview:messageView];
    self.messageView = messageView;

    HandCursorButton *clipboardButton = [[HandCursorButton alloc] initWithFrame:NSZeroRect];
    clipboardButton.bordered = NO;
    clipboardButton.buttonType = NSButtonTypeMomentaryLight;
    clipboardButton.font = [NSFont systemFontOfSize:12 weight:NSFontWeightRegular];
    clipboardButton.target = self;
    clipboardButton.action = @selector(copyToPasteboard);
    clipboardButton.hidden = YES;
    NSMutableAttributedString *copyTitle = [[NSMutableAttributedString alloc] initWithString:@"copy"];
    [copyTitle addAttribute:NSForegroundColorAttributeName value:[NSColor colorWithWhite:0.8 alpha:1.0] range:NSMakeRange(0, copyTitle.length)];
    [copyTitle addAttribute:NSUnderlineStyleAttributeName value:@(NSUnderlineStyleSingle) range:NSMakeRange(0, copyTitle.length)];
    clipboardButton.attributedTitle = copyTitle;
    [self.contentView addSubview:clipboardButton];
    self.clipboardButton = clipboardButton;

    HandCursorButton *closeButton = [[HandCursorButton alloc] initWithFrame:NSZeroRect];
    [closeButton setBezelStyle:NSBezelStyleRegularSquare];
    [closeButton setButtonType:NSButtonTypeMomentaryLight];
    [closeButton setTitle:@"×"];
    [closeButton setFont:[NSFont systemFontOfSize:16 weight:NSFontWeightBold]];
    [closeButton setTarget:self];
    [closeButton setAction:@selector(close)];
    [closeButton setHidden:NO];
    [closeButton setBordered:NO];
    [closeButton setWantsLayer:YES];
    closeButton.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.3].CGColor;
    closeButton.layer.cornerRadius = kCloseSize / 2;
    NSMutableAttributedString *attributedTitle = [[NSMutableAttributedString alloc] initWithString:@"×"];
    [attributedTitle addAttribute:NSForegroundColorAttributeName value:[NSColor whiteColor] range:NSMakeRange(0, attributedTitle.length)];
    [closeButton setAttributedTitle:attributedTitle];
    [self.contentView addSubview:closeButton];
    self.closeButton = closeButton;

    NSTrackingArea *trackingArea = [[NSTrackingArea alloc] initWithRect:[self.contentView bounds] options:(NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect) owner:self userInfo:nil];
    [self.contentView addTrackingArea:trackingArea];
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
  if (gNotificationWindow == self) {
    gNotificationWindow = nil;
  }
  [super close];
}

- (void)mouseEntered:(NSEvent *)event {
  // Buttons are always visible; only pause auto-close.
  self.remainingTime = [self.closeTimer.fireDate timeIntervalSinceNow];
  [self.closeTimer invalidate];
}

- (void)mouseExited:(NSEvent *)event {
  // Buttons are always visible; only resume auto-close.
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

- (CGFloat)lineHeightForFont:(NSFont *)font {
  if (!font) {
    return 18.0;
  }
  CGFloat h = ceil(font.ascender - font.descender + font.leading);
  return h > 0 ? h : 18.0;
}

- (NSInteger)countNewlinesInString:(NSString *)s {
  if (!s || s.length == 0) {
    return 0;
  }
  __block NSInteger n = 0;
  [s enumerateSubstringsInRange:NSMakeRange(0, s.length)
                        options:NSStringEnumerationByComposedCharacterSequences
                     usingBlock:^(NSString *_Nullable substring, NSRange substringRange, NSRange enclosingRange, BOOL *_Nonnull stop) {
                       (void)substringRange;
                       (void)enclosingRange;
                       (void)stop;
                       if (substring && [substring isEqualToString:@"\n"]) {
                         n++;
                       }
                     }];
  return n;
}

- (void)copyToPasteboard {
  NSString *text = self.fullMessage ?: @"";
  if (text.length == 0) {
    return;
  }
  NSPasteboard *pb = [NSPasteboard generalPasteboard];
  [pb clearContents];
  [pb setString:text forType:NSPasteboardTypeString];
  [self close];
}

- (NSString *)normalizedMessageForDisplay:(NSString *)s replaceNewlines:(BOOL)replaceNewlines {
  if (!s) {
    return @"";
  }
  NSMutableString *m = [s mutableCopy];
  [m replaceOccurrencesOfString:@"\r" withString:@"" options:0 range:NSMakeRange(0, m.length)];
  if (replaceNewlines) {
    [m replaceOccurrencesOfString:@"\n" withString:@" " options:0 range:NSMakeRange(0, m.length)];
    [m replaceOccurrencesOfString:@"\t" withString:@" " options:0 range:NSMakeRange(0, m.length)];

    // Collapse multiple spaces into one to save space in notifications
    static NSRegularExpression *regex;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
      regex = [[NSRegularExpression alloc] initWithPattern:@" {2,}" options:0 error:nil];
    });
    [regex replaceMatchesInString:m options:0 range:NSMakeRange(0, m.length) withTemplate:@" "];
  }
  return [m stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
}

- (CGFloat)measureHeightForString:(NSString *)s width:(CGFloat)width attributes:(NSDictionary *)attrs {
  return [self measureHeightForString:s width:width maxHeight:CGFLOAT_MAX reservedWidth:0 attributes:attrs];
}

- (CGFloat)measureHeightForString:(NSString *)s width:(CGFloat)width maxHeight:(CGFloat)maxHeight reservedWidth:(CGFloat)reservedWidth attributes:(NSDictionary *)attrs {
  if (!s || s.length == 0) {
    return 0;
  }
  if (width <= 0) {
    return 0;
  }
  NSTextStorage *ts = [[NSTextStorage alloc] initWithString:s attributes:attrs];
  NSTextContainer *tc = [[NSTextContainer alloc] initWithSize:NSMakeSize(width, maxHeight)];
  NSLayoutManager *lm = [[NSLayoutManager alloc] init];
  tc.lineFragmentPadding = 0;
  if (reservedWidth > 0 && maxHeight != CGFLOAT_MAX) {
    CGFloat lineHeight = [self lineHeightForFont:attrs[NSFontAttributeName]];
    NSRect exclusionRect = NSMakeRect(width - reservedWidth, maxHeight - lineHeight, reservedWidth, lineHeight);
    tc.exclusionPaths = @[[NSBezierPath bezierPathWithRect:exclusionRect]];
  }
  [lm addTextContainer:tc];
  [ts addLayoutManager:lm];
  [lm ensureLayoutForTextContainer:tc];
  return ceil([lm usedRectForTextContainer:tc].size.height);
}

- (NSString *)truncateMultilineTextToFit:(NSString *)text width:(CGFloat)width maxHeight:(CGFloat)maxHeight reservedWidth:(CGFloat)reservedWidth attributes:(NSDictionary *)attrs {
  if (!text) {
    return @"\u2026";
  }
  if (maxHeight <= 0 || width <= 0) {
    return @"\u2026";
  }

  CGFloat fullHeight = [self measureHeightForString:text width:width attributes:attrs];
  if (fullHeight <= maxHeight) {
    // Even if it fits in height, we must check if the last line respects reservedWidth
    // But for simplicity, if it fits in height at full width, we don't show copy anyway.
    return text;
  }

  NSString *ellipsis = @"\u2026";
  NSUInteger len = text.length;
  NSUInteger lo = 0;
  NSUInteger hi = len;
  NSUInteger best = 0;

  NSCharacterSet *trimSet = [NSCharacterSet whitespaceAndNewlineCharacterSet];

  while (lo <= hi) {
    NSUInteger mid = lo + (hi - lo) / 2;
    NSRange safe = [text rangeOfComposedCharacterSequencesForRange:NSMakeRange(0, mid)];
    NSString *prefix = [text substringWithRange:safe];
    prefix = [prefix stringByTrimmingCharactersInSet:trimSet];
    NSString *candidate = [prefix stringByAppendingString:ellipsis];
    CGFloat h = [self measureHeightForString:candidate width:width maxHeight:maxHeight reservedWidth:reservedWidth attributes:attrs];
    if (h <= maxHeight) {
      best = safe.length;
      lo = mid + 1;
    } else {
      if (mid == 0) {
        break;
      }
      hi = mid - 1;
    }
  }

  NSRange safeBest = [text rangeOfComposedCharacterSequencesForRange:NSMakeRange(0, best)];
  NSString *prefix = [text substringWithRange:safeBest];
  prefix = [prefix stringByTrimmingCharactersInSet:trimSet];
  if (prefix.length == 0) {
    return ellipsis;
  }
  return [prefix stringByAppendingString:ellipsis];
}

- (void)updateWithMessage:(NSString *)message icon:(NSImage *)icon {
  if (message == nil) {
    message = @"";
  }
  self.fullMessage = message;

  NSFont *messageFont = [NSFont systemFontOfSize:14];
  CGFloat lineHeight = [self lineHeightForFont:messageFont];

  CGFloat windowWidth = kWindowWidth;
  CGFloat textRight = windowWidth - kClosePad - kCloseSize - kTextRightGapClose;

  CGFloat leftPad = kTextLeftPad;
  if (icon) {
    self.iconView.hidden = NO;
    self.iconView.image = icon;
    leftPad += kIconSize + kIconGap;
  } else {
    self.iconView.hidden = YES;
    self.iconView.image = nil;
  }

  CGFloat baseTextWidth = textRight - leftPad;
  if (baseTextWidth < 120) {
    baseTextWidth = 120;
  }

  NSInteger maxLines = kMaxTextLines;
  if (maxLines < 1) {
    maxLines = 1;
  }

  // Layout like Windows: word-wrap up to 3 lines; only show copy when we actually need truncation.
  NSMutableParagraphStyle *style = [[NSMutableParagraphStyle alloc] init];
  style.lineBreakMode = NSLineBreakByWordWrapping;
  NSDictionary *attrs = @{
    NSFontAttributeName : messageFont,
    NSForegroundColorAttributeName : [NSColor whiteColor],
    NSParagraphStyleAttributeName : style,
  };

  NSFont *copyFont = [NSFont systemFontOfSize:12 weight:NSFontWeightRegular];
  NSSize copyTitleSize = [@"copy" sizeWithAttributes:@{NSFontAttributeName : copyFont}];
  CGFloat copyWidth = MAX(kCopyWidth, ceil(copyTitleSize.width + 2));

  CGFloat maxTextHeight = lineHeight * (CGFloat)maxLines;
  CGFloat fullHeight = [self measureHeightForString:message width:baseTextWidth attributes:attrs];
  BOOL showCopy = (fullHeight > (maxTextHeight + 0.5));
  self.showCopyLink = showCopy;

  CGFloat reservedWidth = 0;
  if (showCopy) {
    reservedWidth = kCopyGap + copyWidth;
  }

  NSString *renderText = message;
  if (showCopy) {
    NSString *normalized = [self normalizedMessageForDisplay:message replaceNewlines:YES];
    renderText = [self truncateMultilineTextToFit:normalized width:baseTextWidth maxHeight:maxTextHeight reservedWidth:reservedWidth attributes:attrs];
  }

  self.messageView.textStorage.attributedString = [[NSAttributedString alloc] initWithString:renderText attributes:attrs];
  if (self.messageView.textContainer) {
    self.messageView.textContainer.lineBreakMode = NSLineBreakByWordWrapping;
    if ([self.messageView.textContainer respondsToSelector:@selector(setMaximumNumberOfLines:)]) {
      self.messageView.textContainer.maximumNumberOfLines = maxLines;
    }
  }

  CGFloat measuredRenderHeight = [self measureHeightForString:renderText width:baseTextWidth maxHeight:maxTextHeight reservedWidth:reservedWidth attributes:attrs];
  CGFloat textHeight = measuredRenderHeight;
  if (textHeight < lineHeight) {
    textHeight = lineHeight;
  }
  if (textHeight > maxTextHeight) {
    textHeight = maxTextHeight;
  }

  CGFloat windowHeight = kTextVertPad * 2 + MAX(textHeight, icon ? kIconSize : 0);
  CGFloat minHeight = kClosePad * 2 + kCloseSize;
  if (windowHeight < minHeight) {
    windowHeight = minHeight;
  }

  NSRect f = self.frame;
  f.size = NSMakeSize(windowWidth, windowHeight);
  [self setFrame:f display:NO];

  self.backgroundView.frame = self.contentView.bounds;

  CGFloat contentY = kTextVertPad;
  CGFloat contentH = windowHeight - kTextVertPad * 2;
  CGFloat textY = contentY + (contentH - textHeight) / 2;
  if (textY < contentY) {
    textY = contentY;
  }

  self.messageView.frame = NSMakeRect(leftPad, textY, baseTextWidth, textHeight);
  if (showCopy) {
    NSRect exclusionRect = NSMakeRect(baseTextWidth - reservedWidth, textHeight - lineHeight, reservedWidth, lineHeight);
    self.messageView.textContainer.exclusionPaths = @[[NSBezierPath bezierPathWithRect:exclusionRect]];
  } else {
    self.messageView.textContainer.exclusionPaths = @[];
  }

  CGFloat closeY = (windowHeight - kCloseSize) / 2;
  self.closeButton.frame = NSMakeRect(windowWidth - kClosePad - kCloseSize, closeY, kCloseSize, kCloseSize);
  self.closeButton.layer.cornerRadius = kCloseSize / 2;

  if (!self.iconView.hidden) {
    CGFloat iconY = (windowHeight - kIconSize) / 2;
    self.iconView.frame = NSMakeRect(kTextLeftPad, iconY, kIconSize, kIconSize);
  }

  if (showCopy) {
    NSLayoutManager *lm = self.messageView.layoutManager;
    NSTextContainer *tc = self.messageView.textContainer;
    if (lm && tc) {
      [lm ensureLayoutForTextContainer:tc];
      NSRange glyphRange = [lm glyphRangeForTextContainer:tc];

      __block NSRect lastUsed = NSZeroRect;
      __block NSRange lastLineGlyphRange = NSMakeRange(NSNotFound, 0);
      [lm enumerateLineFragmentsForGlyphRange:glyphRange
                                  usingBlock:^(NSRect rect, NSRect usedRect, NSTextContainer *_Nonnull textContainer, NSRange lineGlyphRange, BOOL *_Nonnull stop) {
                                    (void)rect;
                                    (void)textContainer;
                                    lastUsed = usedRect;
                                    lastLineGlyphRange = lineGlyphRange;
                                  }];

      NSPoint origin = [self.messageView textContainerOrigin];
      NSRect lastBounds = lastUsed;
      if (lastLineGlyphRange.location != NSNotFound && lastLineGlyphRange.length > 0) {
        lastBounds = [lm boundingRectForGlyphRange:lastLineGlyphRange inTextContainer:tc];
      }

      // Use convertRect:toView: to handle flipped coordinates correctly
      NSRect lastUsedInTextView = NSOffsetRect(lastBounds, origin.x, origin.y);
      NSRect lastUsedInContentView = [self.messageView convertRect:lastUsedInTextView toView:self.contentView];

      CGFloat copyX = NSMaxX(lastUsedInContentView) + kCopyGap;
      CGFloat copyY = NSMidY(lastUsedInContentView) - kCopyHeight / 2.0;

      // Clamp X to not overlap close button area
      CGFloat maxCopyX = textRight - copyWidth;
      if (copyX > maxCopyX) {
        copyX = maxCopyX;
      }

      self.clipboardButton.frame = NSMakeRect(copyX, copyY, copyWidth, kCopyHeight);
    }
  }

  self.clipboardButton.hidden = !showCopy;
}

- (void)mouseDown:(NSEvent *)event {
    self.initialLocation = [event locationInWindow];
}

- (void)mouseDragged:(NSEvent *)event {
    NSPoint currentLocation = [self convertPointToScreen:[event locationInWindow]];
    NSPoint newOrigin = NSMakePoint(currentLocation.x - self.initialLocation.x,
                                    currentLocation.y - self.initialLocation.y);
    [self setFrameOrigin:newOrigin];
}
@end

static NSImage *CreateNSImageFromBGRA(const unsigned char *bgra, int width, int height) {
  if (!bgra || width <= 0 || height <= 0) {
    return nil;
  }

  size_t size = (size_t)width * (size_t)height * 4;
  NSData *data = [NSData dataWithBytes:bgra length:size];

  CGDataProviderRef provider = CGDataProviderCreateWithCFData((__bridge CFDataRef)data);
  if (!provider) {
    return nil;
  }

  CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
  if (!colorSpace) {
    CGDataProviderRelease(provider);
    return nil;
  }

  CGBitmapInfo bitmapInfo = kCGBitmapByteOrder32Little | kCGImageAlphaPremultipliedFirst;
  CGImageRef imgRef = CGImageCreate(width,
                                   height,
                                   8,
                                   32,
                                   (size_t)width * 4,
                                   colorSpace,
                                   bitmapInfo,
                                   provider,
                                   NULL,
                                   true,
                                   kCGRenderingIntentDefault);

  CGColorSpaceRelease(colorSpace);
  CGDataProviderRelease(provider);

  if (!imgRef) {
    return nil;
  }

  NSImage *img = [[NSImage alloc] initWithCGImage:imgRef size:NSMakeSize(width, height)];
  CGImageRelease(imgRef);
  return img;
}

static void ShowNotificationInternal(const char *message, const unsigned char *bgra, int width, int height) {
  if (message == NULL) {
    NSLog(@"Warning: Null message passed to showNotification");
    return;
  }

  @autoreleasepool {
    NSApplication *application = [NSApplication sharedApplication];
    [application setActivationPolicy:NSApplicationActivationPolicyAccessory];

    NSScreen *screen = [NSScreen mainScreen];
    if (!screen) {
      return;
    }
    NSRect screenRect = [screen visibleFrame];

    NSString *messageString = [NSString stringWithUTF8String:message];
    if (messageString == nil) {
      messageString = @"";
    }

    NSImage *iconImage = CreateNSImageFromBGRA(bgra, width, height);

    CGFloat windowWidth = kWindowWidth;
    CGFloat x = NSMidX(screenRect) - windowWidth / 2;
    CGFloat y = NSMinY(screenRect) + 60;
    NSRect frame = NSMakeRect(x, y, windowWidth, 60);

    NotificationWindow *window = gNotificationWindow;
    BOOL needsShow = NO;
    if (!window || !window.isVisible) {
      window = [[NotificationWindow alloc] initWithContentRect:frame styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];
      [window setCornerRadius:20.0];
      [window setLevel:NSFloatingWindowLevel];
      [window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorTransient];
      window.alphaValue = 0.0;
      gNotificationWindow = window;
      needsShow = YES;
    }

    [window updateWithMessage:messageString icon:iconImage];

    // Re-anchor after we know the final height.
    NSRect finalFrame = window.frame;
    finalFrame.origin.x = NSMidX(screenRect) - finalFrame.size.width / 2;
    finalFrame.origin.y = NSMinY(screenRect) + 60;
    [window setFrame:finalFrame display:YES];

    if (needsShow) {
      [window orderFront:nil];
      [NSAnimationContext
          runAnimationGroup:^(NSAnimationContext *context) {
            context.duration = 0.3;
            window.animator.alphaValue = 1.0;
          }
          completionHandler:^{
            window.remainingTime = kDefaultCloseSeconds;
            [window scheduleCloseTimer];
          }];
    } else {
      // Update existing window: reset close timer.
      window.alphaValue = 1.0;
      [window.closeTimer invalidate];
      window.remainingTime = kDefaultCloseSeconds;
      [window scheduleCloseTimer];
      [window orderFront:nil];
    }
  }
}

void showNotification(const char *message) {
  ShowNotificationInternal(message, NULL, 0, 0);
}

void showNotificationWithIcon(const char *message, const unsigned char *bgra, int width, int height) {
  ShowNotificationInternal(message, bgra, width, height);
}
