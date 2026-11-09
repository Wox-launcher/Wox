#import <AppKit/AppKit.h>
#include <stdbool.h>

@interface WoxPermissionPromptActions : NSObject
@end

@implementation WoxPermissionPromptActions
- (void)continueRequest:(id)sender {
    [NSApp stopModalWithCode:NSModalResponseOK];
}

- (void)cancelRequest:(id)sender {
    [NSApp stopModalWithCode:NSModalResponseCancel];
}
@end

static NSTextField *permissionPromptLabel(const char *text, NSFont *font, NSColor *color, CGFloat width) {
    NSTextField *label = [NSTextField wrappingLabelWithString:[NSString stringWithUTF8String:text]];
    label.font = font;
    label.textColor = color;
    label.alignment = NSTextAlignmentCenter;
    label.selectable = NO;
    label.preferredMaxLayoutWidth = width;
    return label;
}

// Keep the buttons visible even if a plugin supplies a long explanation.
static NSView *permissionPromptReasonView(NSTextField *label, CGFloat width) {
    const CGFloat maxHeight = 160;
    CGFloat textHeight = ceil(label.intrinsicContentSize.height);
    if (textHeight <= maxHeight) {
        return label;
    }

    NSScrollView *scroll = [[[NSScrollView alloc] initWithFrame:NSMakeRect(0, 0, width, maxHeight)] autorelease];
    scroll.drawsBackground = NO;
    scroll.hasVerticalScroller = YES;
    scroll.autohidesScrollers = YES;
    scroll.scrollerStyle = NSScrollerStyleOverlay;
    // Reserve space on both sides so the overlay scroller cannot cover the text.
    const CGFloat textInset = 18;
    label.preferredMaxLayoutWidth = width - 2 * textInset;
    textHeight = ceil(label.intrinsicContentSize.height);
    NSView *document = [[[NSView alloc] initWithFrame:NSMakeRect(0, 0, width, textHeight)] autorelease];
    [document addSubview:label];
    label.translatesAutoresizingMaskIntoConstraints = NO;
    [NSLayoutConstraint activateConstraints:@[
        [label.leadingAnchor constraintEqualToAnchor:document.leadingAnchor constant:textInset],
        [label.trailingAnchor constraintEqualToAnchor:document.trailingAnchor constant:-textInset],
        [label.topAnchor constraintEqualToAnchor:document.topAnchor],
        [label.bottomAnchor constraintEqualToAnchor:document.bottomAnchor],
    ]];
    scroll.documentView = document;
    [document scrollPoint:NSMakePoint(0, textHeight - maxHeight)];
    [scroll.heightAnchor constraintEqualToConstant:maxHeight].active = YES;
    return scroll;
}

bool woxConfirmOSPermissionReason(const char *title, const char *reason, const char *continueLabel, const char *cancelLabel) {
    @autoreleasepool {
        const CGFloat width = 360;
        const CGFloat padding = 26;
        const CGFloat innerWidth = width - 2 * padding;
        NSPanel *panel = [[NSPanel alloc] initWithContentRect:NSMakeRect(0, 0, width, 240)
                                                  styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskFullSizeContentView
                                                    backing:NSBackingStoreBuffered
                                                      defer:NO
                                                     screen:NSScreen.mainScreen];
        panel.title = [NSString stringWithUTF8String:title];
        panel.titleVisibility = NSWindowTitleHidden;
        panel.titlebarAppearsTransparent = YES;
        panel.releasedWhenClosed = NO;

        NSImageView *icon = [NSImageView imageViewWithImage:NSApp.applicationIconImage];
        icon.imageScaling = NSImageScaleProportionallyUpOrDown;
        [icon setAccessibilityElement:NO];
        [icon.widthAnchor constraintEqualToConstant:56].active = YES;
        [icon.heightAnchor constraintEqualToConstant:56].active = YES;

        NSTextField *heading = permissionPromptLabel(title, [NSFont systemFontOfSize:19 weight:NSFontWeightSemibold],
                                                     NSColor.labelColor, innerWidth);
        heading.maximumNumberOfLines = 3;
        NSTextField *explanation = permissionPromptLabel(reason, [NSFont systemFontOfSize:13],
                                                         NSColor.secondaryLabelColor, innerWidth);
        NSView *reasonView = permissionPromptReasonView(explanation, innerWidth);

        WoxPermissionPromptActions *actions = [[[WoxPermissionPromptActions alloc] init] autorelease];
        NSButton *cancel = [NSButton buttonWithTitle:[NSString stringWithUTF8String:cancelLabel]
                                             target:actions action:@selector(cancelRequest:)];
        NSButton *proceed = [NSButton buttonWithTitle:[NSString stringWithUTF8String:continueLabel]
                                              target:actions action:@selector(continueRequest:)];
        cancel.controlSize = NSControlSizeLarge;
        proceed.controlSize = NSControlSizeLarge;
        cancel.keyEquivalent = @"\033";
        proceed.keyEquivalent = @"\r";
        panel.defaultButtonCell = proceed.cell;
        panel.initialFirstResponder = proceed;

        NSStackView *buttons = [NSStackView stackViewWithViews:@[cancel, proceed]];
        buttons.distribution = NSStackViewDistributionFillEqually;
        buttons.spacing = 10;

        NSStackView *stack = [NSStackView stackViewWithViews:@[icon, heading, reasonView, buttons]];
        stack.orientation = NSUserInterfaceLayoutOrientationVertical;
        stack.alignment = NSLayoutAttributeCenterX;
        stack.distribution = NSStackViewDistributionFill;
        stack.spacing = 10;
        [stack setCustomSpacing:15 afterView:icon];
        [stack setCustomSpacing:22 afterView:reasonView];
        stack.translatesAutoresizingMaskIntoConstraints = NO;
        [NSLayoutConstraint activateConstraints:@[
            [stack.widthAnchor constraintEqualToConstant:innerWidth],
            [heading.widthAnchor constraintEqualToAnchor:stack.widthAnchor],
            [reasonView.widthAnchor constraintEqualToAnchor:stack.widthAnchor],
            [buttons.widthAnchor constraintEqualToAnchor:stack.widthAnchor],
        ]];

        [panel setContentSize:NSMakeSize(width, ceil(stack.fittingSize.height) + 2 * padding)];
        NSView *content = panel.contentView;
        [content addSubview:stack];
        [NSLayoutConstraint activateConstraints:@[
            [stack.centerXAnchor constraintEqualToAnchor:content.centerXAnchor],
            [stack.topAnchor constraintEqualToAnchor:content.topAnchor constant:padding],
            [stack.bottomAnchor constraintEqualToAnchor:content.bottomAnchor constant:-padding],
        ]];

        [panel center];
        [NSApp activateIgnoringOtherApps:YES];
        NSInteger response = [NSApp runModalForWindow:panel];
        [panel close];
        [panel release];
        return response == NSModalResponseOK;
    }
}
