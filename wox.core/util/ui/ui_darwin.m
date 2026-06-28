//go:build darwin && cgo

// Single source of truth for the Go<->C ABI: shared struct definitions,
// command/event/key enums, and function declarations.
#include "ui_native.h"

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <CoreText/CoreText.h>
#import <ApplicationServices/ApplicationServices.h>
#include <stdlib.h>
#include <string.h>

// Map macOS keyCode to our Key enum. Values are the Carbon kVK_* virtual key
// codes (see HIToolbox/Events.h); we inline them to avoid importing the
// deprecated Carbon framework.
enum {
    kVK_Escape_v           = 0x35,
    kVK_Return_v           = 0x24,
    kVK_ANSI_KeypadEnter_v = 0x4C,
    kVK_Delete_v           = 0x33,   // Backspace (delete left)
    kVK_ForwardDelete_v    = 0x75,   // Fn+Delete (delete right)
    kVK_Tab_v              = 0x30,
    kVK_Space_v            = 0x31,
    kVK_UpArrow_v          = 0x7E,
    kVK_DownArrow_v        = 0x7D,
    kVK_LeftArrow_v        = 0x7B,
    kVK_RightArrow_v       = 0x7C,
    kVK_Home_v             = 0x73,
    kVK_End_v              = 0x77,
    kVK_PageUp_v           = 0x74,
    kVK_PageDown_v         = 0x79,
};

// drawRect: calls into Go to retrieve the latest command list and execute it
// immediately on the current graphics context.
extern void uiDarwinOnDraw(int32_t windowId);

// ── UIImage cache keyed by imageKey ─────────────────────────────────────

typedef struct UIBitmapCacheEntry {
    char* key;
    int32_t keyLen;
    CGImageRef image;
    struct UIBitmapCacheEntry* next;
} UIBitmapCacheEntry;

// ── WoxRenderView: custom NSView with NSTextInputClient ─────────────────

@interface WoxRenderView : NSView <NSTextInputClient> {
    @public
    int32_t windowId;
    bool hasMarked;
    NSRange markedRange;
    NSRange selectedRange;
}
@end

@implementation WoxRenderView

- (BOOL)acceptsFirstResponder { return YES; }

- (BOOL)acceptsFirstMouse:(NSEvent *)event { return YES; }

// Map macOS keyCode to our Key enum. Only the keys we care about for launcher
// navigation are translated; the rest fall through to insertText:.
- (int32_t)mapKeyCode:(uint16)keyCode {
    switch (keyCode) {
        case kVK_Escape_v:        return KeyEscape;
        case kVK_Return_v:
        case kVK_ANSI_KeypadEnter_v: return KeyEnter;
        case kVK_Delete_v:         return KeyBackspace;
        case kVK_ForwardDelete_v:  return KeyDelete;
        case kVK_Tab_v:            return KeyTab;
        case kVK_Space_v:          return KeySpace;
        case kVK_UpArrow_v:        return KeyUp;
        case kVK_DownArrow_v:      return KeyDown;
        case kVK_LeftArrow_v:      return KeyLeft;
        case kVK_RightArrow_v:     return KeyRight;
        case kVK_Home_v:           return KeyHome;
        case kVK_End_v:            return KeyEnd;
        case kVK_PageUp_v:         return KeyPageUp;
        case kVK_PageDown_v:       return KeyPageDown;
        default:                   return 0; // KeyUnknown — let IME handle text input
    }
}

- (int32_t)currentMods {
    NSEventModifierFlags flags = [NSEvent modifierFlags];
    int32_t mods = 0;
    if (flags & NSEventModifierFlagShift)    mods |= 1; // ModShift
    if (flags & NSEventModifierFlagControl)  mods |= 2; // ModControl
    if (flags & NSEventModifierFlagOption)   mods |= 4; // ModAlt
    if (flags & NSEventModifierFlagCommand)  mods |= 8; // ModSuper
    return mods;
}

- (void)keyDown:(NSEvent *)event {
    int32_t key = [self mapKeyCode:event.keyCode];
    int32_t mods = [self currentMods];
    NSLog(@"[keyDown:] keyCode=%u mappedKey=%d mods=%d chars=%@",
          event.keyCode, key, mods, event.characters);

    // For navigation keys (arrows, escape, enter, backspace, etc.), bypass IME
    // and dispatch directly. For other keys, let interpretKeyEvents: route to
    // insertText:/setMarkedText: for IME composition support.
    if (key != 0) {
        uiEventCallback(windowId, EventKeyPress, key, mods,
            NULL, 0, NULL, 0, 0, 0, 0, 0, 0, 0);
        return;
    }

    // Let the input method interpret the key. This drives insertText: /
    // setMarkedText: / unmarkText for CJK input.
    [self interpretKeyEvents:@[event]];
}

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange {
    NSString *text = [string isKindOfClass:[NSAttributedString class]]
        ? [(NSAttributedString *)string string]
        : (NSString *)string;

    if (text.length == 0) return;

    const char *utf8 = [text UTF8String];
    int32_t len = (int32_t)strlen(utf8);
    if (len <= 0) return;

    char *buf = (char *)malloc(len);
    memcpy(buf, utf8, len);

    uiEventCallback(windowId, EventTextInput, 0, 0,
        buf, len, NULL, 0, 0, 0, 0, 0, 0, 0);

    free(buf);

    hasMarked = false;
    markedRange = NSMakeRange(NSNotFound, 0);
    selectedRange = NSMakeRange(0, 0);
}

- (void)setMarkedText:(id)string selectedRange:(NSRange)selected replacementRange:(NSRange)replacement {
    NSString *text = [string isKindOfClass:[NSAttributedString class]]
        ? [(NSAttributedString *)string string]
        : (NSString *)string;

    selectedRange = selected;

    if (text.length == 0) {
        hasMarked = false;
        markedRange = NSMakeRange(NSNotFound, 0);
        uiEventCallback(windowId, EventIMECompose, 0, 0,
            NULL, 0, NULL, 0, 0, 0, 0, 0, 0, 0);
        [self setNeedsDisplay:YES];
        return;
    }

    hasMarked = true;
    markedRange = NSMakeRange(0, text.length);
    const char *utf8 = [text UTF8String];
    int32_t len = (int32_t)strlen(utf8);
    char *buf = (char *)malloc(len);
    memcpy(buf, utf8, len);
    int32_t cursor = (int32_t)selected.location;

    uiEventCallback(windowId, EventIMECompose, 0, 0,
        NULL, 0, buf, len, cursor, 0, 0, 0, 0, 0);

    free(buf);
    [self setNeedsDisplay:YES];
}

- (void)unmarkText {
    if (hasMarked) {
        hasMarked = false;
        markedRange = NSMakeRange(NSNotFound, 0);
        uiEventCallback(windowId, EventIMECompose, 0, 0,
            NULL, 0, NULL, 0, 0, 0, 0, 0, 0, 0);
    }
}

- (BOOL)hasMarkedText { return hasMarked; }

- (NSRange)markedRange { return markedRange; }
- (NSRange)selectedRange { return selectedRange; }

- (NSArray<NSString *> *)validAttributesForMarkedText {
    return @[];
}

// Required NSTextInputClient methods we don't really use; return empty/zero
// so the protocol is fully satisfied and the input method doesn't crash.
- (NSAttributedString *)attributedSubstringForProposedRange:(NSRange)range actualRange:(NSRangePointer)actual {
    if (actual) *actual = NSMakeRange(NSNotFound, 0);
    return nil;
}

- (NSUInteger)characterIndexForPoint:(NSPoint)point {
    return NSNotFound;
}

- (NSRect)firstRectForCharacterRange:(NSRange)range actualRange:(NSRangePointer)actual {
    // Position the IME candidate window near the top-left of the query box.
    NSRect viewRect = self.bounds;
    // Convert to screen coordinates (flip Y since NSView origin is bottom-left).
    CGFloat screenX = [[self window] frame].origin.x + viewRect.origin.x + 12;
    CGFloat screenY = [[self window] frame].origin.y + viewRect.origin.y + viewRect.size.height - 40;
    NSRect rect = NSMakeRect(screenX, screenY, 0, 0);
    if (actual) *actual = range;
    return rect;
}

- (void)doCommandBySelector:(SEL)selector {
    // Some keys (e.g. Tab, Escape) may come through doCommandBySelector:
    // rather than keyDown's interpretKeyEvents:. No-op here — we handle them
    // in keyDown before interpretKeyEvents:.
}

- (void)scrollWheel:(NSEvent *)event {
    float deltaY = (float)[event deltaY];
    NSPoint pos = [self convertPoint:[event locationInWindow] fromView:nil];
    // Convert from bottom-left origin to top-left origin (Y-down).
    CGFloat h = self.bounds.size.height;
    float x = (float)pos.x;
    float y = (float)(h - pos.y);
    int32_t winW = (int32_t)self.bounds.size.width;
    int32_t winH = (int32_t)self.bounds.size.height;
    uiEventCallback(windowId, EventScroll, 0, 0,
        NULL, 0, NULL, 0, 0, x, y, deltaY, winW, winH);
}

- (void)mouseDown:(NSEvent *)event {
    NSPoint pos = [self convertPoint:[event locationInWindow] fromView:nil];
    CGFloat h = self.bounds.size.height;
    float x = (float)pos.x;
    float y = (float)(h - pos.y);
    uiEventCallback(windowId, EventClick, 0, 0,
        NULL, 0, NULL, 0, 0, x, y, 0, 0, 0);
}

- (void)drawRect:(NSRect)dirtyRect {
    NSLog(@"[drawRect:] called for windowId=%d bounds=%f x %f", windowId,
          self.bounds.size.width, self.bounds.size.height);
    [NSGraphicsContext saveGraphicsState];
    CGContextRef ctx = [[NSGraphicsContext currentContext] CGContext];

    if (ctx) {
        CGContextSaveGState(ctx);

        // Pull the latest command list from Go and execute it on this context.
        // Draw commands use top-left coordinates; ExecuteCommands converts
        // them to Cocoa's bottom-left coordinate system per primitive.
        uiDarwinOnDraw(windowId);

        CGContextRestoreGState(ctx);
    } else {
        NSLog(@"[drawRect:] ctx is NULL!");
    }

    [NSGraphicsContext restoreGraphicsState];
}

- (void)viewDidEndLiveResize {
    int32_t w = (int32_t)self.bounds.size.width;
    int32_t h = (int32_t)self.bounds.size.height;
    uiEventCallback(windowId, EventResize, 0, 0,
        NULL, 0, NULL, 0, 0, 0, 0, 0, w, h);
    [super viewDidEndLiveResize];
}

@end

@interface WoxPanel : NSPanel
@end

@implementation WoxPanel

- (BOOL)canBecomeKeyWindow {
    return YES;
}

- (BOOL)canBecomeMainWindow {
    return YES;
}

@end

// ── WoxWindow: holds the NSPanel and its render view ─────────────────────

typedef struct {
    int32_t id;             // 1-based window id (matches g_windows index+1)
    NSPanel *panel;
    NSVisualEffectView *effectView;
    WoxRenderView *renderView;
    int32_t width;
    int32_t height;
    float cornerRadius;
    bool transparent;
    bool darkMode;
    bool visible;

    UIBitmapCacheEntry* bitmapCache;
} UIWindow;

static UIWindow* g_windows[16];
static int g_windowCount = 0;

static UIWindow* FindWindowById(int32_t id) {
    for (int i = 0; i < g_windowCount; i++) {
        if (g_windows[i] && g_windows[i]->id == id)
            return g_windows[i];
    }
    return NULL;
}

// ── Image cache helpers ─────────────────────────────────────────────────

static CGImageRef FindCachedImage(UIWindow* win, const char* key, int32_t keyLen) {
    if (!win || !key || keyLen <= 0) return NULL;
    for (UIBitmapCacheEntry* e = win->bitmapCache; e; e = e->next) {
        if (e->keyLen == keyLen && memcmp(e->key, key, keyLen) == 0) {
            return e->image;
        }
    }
    return NULL;
}

static void CacheImage(UIWindow* win, const char* key, int32_t keyLen, CGImageRef image) {
    if (!win || !key || keyLen <= 0 || !image || FindCachedImage(win, key, keyLen)) return;
    UIBitmapCacheEntry* entry = (UIBitmapCacheEntry*)calloc(1, sizeof(UIBitmapCacheEntry));
    entry->key = (char*)malloc(keyLen);
    memcpy(entry->key, key, keyLen);
    entry->keyLen = keyLen;
    entry->image = (CGImageRef)image;
    CGImageRetain(image);
    entry->next = win->bitmapCache;
    win->bitmapCache = entry;
}

static void ClearBitmapCache(UIWindow* win) {
    if (!win) return;
    UIBitmapCacheEntry* e = win->bitmapCache;
    while (e) {
        UIBitmapCacheEntry* next = e->next;
        CGImageRelease(e->image);
        free(e->key);
        free(e);
        e = next;
    }
    win->bitmapCache = NULL;
}

// ── Color and rect helpers ──────────────────────────────────────────────

static void SetFillColor(CGContextRef ctx, float r, float g, float b, float a) {
    CGContextSetRGBFillColor(ctx, r, g, b, a);
}

static CGFloat TopLeftYToBottomY(UIWindow* win, float y) {
    return (CGFloat)win->height - (CGFloat)y;
}

static CGRect MakeTopLeftRect(UIWindow* win, float x, float y, float w, float h) {
    return CGRectMake((CGFloat)x, (CGFloat)win->height - (CGFloat)y - (CGFloat)h, (CGFloat)w, (CGFloat)h);
}

// ── ExecuteCommands: CoreGraphics draw loop ─────────────────────────────

static void ExecuteCommands(UIWindow* win, CGContextRef ctx, const CDrawCommand* cmds, int32_t count) {
    if (!win || !ctx || !cmds || count <= 0) return;

    for (int32_t i = 0; i < count; i++) {
        const CDrawCommand* cmd = &cmds[i];

        switch (cmd->cmd_type) {
        case CmdClear: {
            // Clear to transparent first so the vibrancy backdrop shows through,
            // then fill with the theme background (semi-transparent if alpha < 1).
            CGContextClearRect(ctx, CGRectMake(0, 0, win->width, win->height));
            if (cmd->a > 0) {
                SetFillColor(ctx, cmd->r, cmd->g, cmd->b, cmd->a);
                CGContextFillRect(ctx, CGRectMake(0, 0, win->width, win->height));
            }
            break;
        }

        case CmdDrawRect: {
            SetFillColor(ctx, cmd->r, cmd->g, cmd->b, cmd->a);
            CGContextFillRect(ctx, MakeTopLeftRect(win, cmd->x, cmd->y, cmd->w, cmd->h));
            break;
        }

        case CmdDrawRoundedRect: {
            SetFillColor(ctx, cmd->r, cmd->g, cmd->b, cmd->a);
            CGPathRef path = CGPathCreateWithRoundedRect(
                MakeTopLeftRect(win, cmd->x, cmd->y, cmd->w, cmd->h),
                cmd->radius, cmd->radius, NULL);
            CGContextAddPath(ctx, path);
            CGContextFillPath(ctx);
            CGPathRelease(path);
            break;
        }

        case CmdDrawText: {
            if (!cmd->text || cmd->textLen <= 0) break;

            CFStringRef textStr = CFStringCreateWithBytes(
                kCFAllocatorDefault, (const UInt8*)cmd->text, cmd->textLen,
                kCFStringEncodingUTF8, false);
            if (!textStr) break;

            // Font family: use provided family or default to PingFang SC
            // (CJK-capable, symmetric to Windows' "Microsoft YaHei").
            CFStringRef familyStr = NULL;
            if (cmd->fontFamily && cmd->fontFamilyLen > 0) {
                familyStr = CFStringCreateWithBytes(
                    kCFAllocatorDefault, (const UInt8*)cmd->fontFamily, cmd->fontFamilyLen,
                    kCFStringEncodingUTF8, false);
            }
            if (!familyStr) {
                familyStr = CFSTR("PingFang SC");
            }

            CGFloat fontSize = cmd->fontSize > 0 ? cmd->fontSize : 16.0f;
            CTFontRef font = CTFontCreateWithName(familyStr, fontSize, NULL);

            // Build attributed string with color.
            CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
            CGFloat comps[4] = { cmd->r, cmd->g, cmd->b, cmd->a };
            CGColorRef color = CGColorCreate(cs, comps);
            CGColorSpaceRelease(cs);

            CFStringRef keys[] = { kCTFontAttributeName, kCTForegroundColorAttributeName };
            CFTypeRef values[] = { font, color };
            CFDictionaryRef attrs = CFDictionaryCreate(
                kCFAllocatorDefault, (const void**)keys, (const void**)values, 2,
                &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
            CFAttributedStringRef attrStr = CFAttributedStringCreate(
                kCFAllocatorDefault, textStr, attrs);

            CTLineRef line = CTLineCreateWithAttributedString(attrStr);

            // Vertically center the text in the command's rect.
            CGFloat ascent, descent, leading;
            CTLineGetTypographicBounds(line, &ascent, &descent, &leading);
            CGFloat textH = ascent + descent;

            CGContextSaveGState(ctx);
            CGContextSetTextMatrix(ctx, CGAffineTransformIdentity);
            CGFloat rectBottomY = win->height - (cmd->y + cmd->h);
            CGFloat baselineY = rectBottomY + (cmd->h - textH) / 2 + descent;
            CGContextSetTextPosition(ctx, cmd->x, baselineY);
            CTLineDraw(line, ctx);
            CGContextRestoreGState(ctx);

            CFRelease(line);
            CFRelease(attrStr);
            CFRelease(attrs);
            CGColorRelease(color);
            CFRelease(font);
            if (familyStr != CFSTR("PingFang SC")) CFRelease(familyStr);
            CFRelease(textStr);
            break;
        }

        case CmdDrawImage: {
            CGImageRef image = FindCachedImage(win, cmd->imageKey, cmd->imageKeyLen);
            bool fromCache = (image != NULL);

            if (!image && cmd->imageData && cmd->imageLen > 0) {
                // Decoding a PNG from memory: copy to a retained data provider
                // since cmd->imageData is owned by Go and only valid during this call.
                CFDataRef dataRef = CFDataCreate(kCFAllocatorDefault,
                    cmd->imageData, (CFIndex)cmd->imageLen);
                CGDataProviderRef provider = CGDataProviderCreateWithCFData(dataRef);
                if (provider) {
                    image = CGImageCreateWithPNGDataProvider(provider, NULL, false,
                        kCGRenderingIntentDefault);
                    CGDataProviderRelease(provider);
                }
                CFRelease(dataRef);
            }
            if (!image) break;

            CGFloat w = cmd->w > 0 ? cmd->w : CGImageGetWidth(image);
            CGFloat h = cmd->h > 0 ? cmd->h : CGImageGetHeight(image);
            CGRect dest = MakeTopLeftRect(win, cmd->x, cmd->y, w, h);
            CGContextDrawImage(ctx, dest, image);

            if (!fromCache) {
                if (cmd->imageKey && cmd->imageKeyLen > 0) {
                    CacheImage(win, cmd->imageKey, cmd->imageKeyLen, image);
                }
                CGImageRelease(image);
            }
            break;
        }

        case CmdDrawLine: {
            // Note: our Go DrawLine encodes start=(X,Y) end=(W,H).
            CGContextSetLineWidth(ctx, cmd->strokeWidth > 0 ? cmd->strokeWidth : 1.0f);
            CGContextSetRGBStrokeColor(ctx, cmd->r, cmd->g, cmd->b, cmd->a);
            CGContextMoveToPoint(ctx, cmd->x, TopLeftYToBottomY(win, cmd->y));
            CGContextAddLineToPoint(ctx, cmd->w, TopLeftYToBottomY(win, cmd->h));
            CGContextStrokePath(ctx);
            break;
        }

        case CmdPushClip: {
            CGContextSaveGState(ctx);
            CGContextClipToRect(ctx, MakeTopLeftRect(win, cmd->x, cmd->y, cmd->w, cmd->h));
            break;
        }

        case CmdPopClip: {
            CGContextRestoreGState(ctx);
            break;
        }
        }
    }
}

// ── Window creation ─────────────────────────────────────────────────────

int32_t uiWindowCreate(CWindowConfig config) {
    NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];

    UIWindow* win = (UIWindow*)calloc(1, sizeof(UIWindow));
    if (!win) {
        [pool drain];
        return 0;
    }

    win->width = config.width;
    win->height = config.height;
    win->cornerRadius = config.cornerRadius;
    win->transparent = config.transparent;
    win->darkMode = config.darkMode;
    win->visible = false;

    // Borderless floating panel — symmetric to Windows
    // WS_POPUP | WS_EX_TOOLWINDOW | WS_EX_TOPMOST.
    NSUInteger styleMask = NSWindowStyleMaskBorderless;
    NSPanel *panel = [[WoxPanel alloc] initWithContentRect:NSMakeRect(0, 0, config.width, config.height)
        styleMask:styleMask backing:NSBackingStoreBuffered defer:NO];
    if (!panel) {
        free(win);
        [pool drain];
        return 0;
    }

    [panel setFloatingPanel:YES];                        // always on top of normal windows
    [panel setBecomesKeyOnlyIfNeeded:NO];                 // become key when activated
    [panel setHidesOnDeactivate:NO];                      // we manage hide ourselves
    [panel setHasShadow:YES];
    [panel setOpaque:NO];
    [panel setBackgroundColor:[NSColor clearColor]];
    [panel setExcludedFromWindowsMenu:YES];
    [panel setLevel:NSPopUpMenuWindowLevel];             // ~ WS_EX_TOPMOST
    [panel setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
        NSWindowCollectionBehaviorFullScreenAuxiliary];

    // Vibrancy backdrop — symmetric to Windows Mica/Acrylic.
    NSVisualEffectView *effectView = [[NSVisualEffectView alloc]
        initWithFrame:NSMakeRect(0, 0, config.width, config.height)];
    [effectView setBlendingMode:NSVisualEffectBlendingModeBehindWindow];
    [effectView setState:NSVisualEffectStateActive];
    [effectView setMaterial:NSVisualEffectMaterialMenu];
    if (config.darkMode) {
        [effectView setAppearance:[NSAppearance appearanceNamed:NSAppearanceNameVibrantDark]];
    } else {
        [effectView setAppearance:[NSAppearance appearanceNamed:NSAppearanceNameVibrantLight]];
    }

    // Rounded corners via the visual effect view's backing layer.
    if (config.cornerRadius > 0) {
        [effectView setWantsLayer:YES];
        [[effectView layer] setCornerRadius:config.cornerRadius];
        [[effectView layer] setMasksToBounds:YES];
    }

    [panel setContentView:effectView];

    // Custom render view fills the effect view.
    WoxRenderView *renderView = [[WoxRenderView alloc]
        initWithFrame:NSMakeRect(0, 0, config.width, config.height)];
    // Layer-backed views DO get drawRect: when setNeedsDisplay: is called,
    // and using a layer makes the view participate in the normal compositing
    // pipeline alongside the NSVisualEffectView. We need wantsLayer=YES so
    // the render view is not clipped or hidden by the vibrancy backdrop.
    [renderView setWantsLayer:YES];
    // Ensure the render view has a transparent background so the vibrancy
    // backdrop shows through; drawRect: will paint content on top.
    [[renderView layer] setBackgroundColor:[NSColor clearColor].CGColor];
    // Attach render view to the view hierarchy so drawRect: and key events are delivered.
    [effectView addSubview:renderView positioned:NSWindowAbove relativeTo:nil];

    win->panel = panel;
    win->effectView = effectView;
    win->renderView = renderView;

    if (g_windowCount < 16) {
        g_windows[g_windowCount++] = win;
    } else {
        [renderView release];
        [effectView release];
        [panel release];
        free(win);
        [pool drain];
        return 0;
    }

    // Use the index+1 as the id (0 is reserved for "failure").
    int32_t id = g_windowCount; // 1-based
    win->id = id;
    renderView->windowId = id;
    [pool drain];
    return id;
}

// ── Window lifecycle ────────────────────────────────────────────────────

void uiWindowShow(int32_t windowId) {
    NSLog(@"[uiWindowShow] called for windowId=%d", windowId);
    dispatch_async(dispatch_get_main_queue(), ^{
        UIWindow* win = FindWindowById(windowId);
        if (!win) {
            NSLog(@"[uiWindowShow] window not found!");
            return;
        }
        // LSUIElement apps run as NSApplicationActivationPolicyAccessory, which
        // cannot become the active app. Without active status the input method
        // manager (TSM) will not route keyboard events through
        // interpretKeyEvents: → insertText:/setMarkedText:, so NSTextInputClient
        // never receives committed or composed text and the query box stays
        // unresponsive to character input. Promote to Regular while visible so
        // IME works; demote back to Accessory in uiWindowHide to keep Wox out of
        // the Dock when hidden.
        if ([NSApp activationPolicy] != NSApplicationActivationPolicyRegular) {
            [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        }
        [NSApp activateIgnoringOtherApps:YES];
        [win->panel center];
        [win->panel makeKeyAndOrderFront:nil];
        [win->panel makeFirstResponder:win->renderView];
        win->visible = true;
        NSLog(@"[uiWindowShow] panel shown, renderView=%p isFirstResponder=%d",
              win->renderView, [[win->panel firstResponder] isEqual:win->renderView]);
        // Force an immediate display so drawRect: runs right after show.
        [win->renderView setNeedsDisplay:YES];
        [win->renderView displayIfNeeded];
    });
}

void uiWindowHide(int32_t windowId) {
    dispatch_async(dispatch_get_main_queue(), ^{
        UIWindow* win = FindWindowById(windowId);
        if (!win) return;
        [win->panel orderOut:nil];
        win->visible = false;
        // Return to Accessory policy so Wox leaves the Dock while hidden,
        // matching the LSUIElement background-app behavior.
        if ([NSApp activationPolicy] != NSApplicationActivationPolicyAccessory) {
            [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        }
    });
}

void uiWindowSetDarkMode(int32_t windowId, bool darkMode) {
    UIWindow* win = FindWindowById(windowId);
    if (!win) return;
    win->darkMode = darkMode;
    if (darkMode) {
        [win->effectView setAppearance:[NSAppearance appearanceNamed:NSAppearanceNameVibrantDark]];
    } else {
        [win->effectView setAppearance:[NSAppearance appearanceNamed:NSAppearanceNameVibrantLight]];
    }
}

void uiWindowReleaseMemory(int32_t windowId) {
    UIWindow* win = FindWindowById(windowId);
    if (!win) return;
    ClearBitmapCache(win);
}

void uiWindowSetPosition(int32_t windowId, int32_t x, int32_t y) {
    dispatch_async(dispatch_get_main_queue(), ^{
        UIWindow* win = FindWindowById(windowId);
        if (!win) return;
        // Screen coordinates are top-left in our convention; Cocoa uses bottom-left.
        NSScreen *screen = [NSScreen mainScreen];
        CGFloat screenHeight = screen.frame.size.height;
        NSRect frame = [win->panel frame];
        frame.origin.x = x;
        frame.origin.y = screenHeight - y - frame.size.height;
        [win->panel setFrameOrigin:frame.origin];
    });
}

void uiWindowSetSize(int32_t windowId, int32_t w, int32_t h) {
    dispatch_async(dispatch_get_main_queue(), ^{
        UIWindow* win = FindWindowById(windowId);
        if (!win) return;
        NSRect frame = [win->panel frame];
        CGFloat topY = NSMaxY(frame);
        frame.size.width = w;
        frame.size.height = h;
        // Go and Windows size changes are top-left anchored. Cocoa frames are
        // bottom-left anchored, so keep the top edge stable while the launcher
        // grows or shrinks downward as result rows appear/disappear.
        frame.origin.y = topY - frame.size.height;
        [win->panel setFrame:frame display:YES animate:NO];
        win->width = w;
        win->height = h;
        [win->effectView setFrame:NSMakeRect(0, 0, w, h)];
        [win->renderView setFrame:NSMakeRect(0, 0, w, h)];
    });
}

bool uiWindowIsVisible(int32_t windowId) {
    UIWindow* win = FindWindowById(windowId);
    if (!win) return false;
    return win->visible;
}

void uiWindowGetSize(int32_t windowId, int32_t* outW, int32_t* outH) {
    UIWindow* win = FindWindowById(windowId);
    if (!win || !outW || !outH) {
        if (outW) *outW = 0;
        if (outH) *outH = 0;
        return;
    }
    *outW = win->width;
    *outH = win->height;
}

void uiWindowDestroy(int32_t windowId) {
    for (int i = 0; i < g_windowCount; i++) {
        if (g_windows[i] && g_windows[i]->id == windowId) {
            UIWindow* win = g_windows[i];
            g_windows[i] = g_windows[g_windowCount - 1];
            g_windows[g_windowCount - 1] = NULL;
            g_windowCount--;

            ClearBitmapCache(win);
            [win->renderView release];
            [win->effectView release];
            [win->panel close];
            [win->panel release];
            free(win);
            return;
        }
    }
}

void uiWindowInvalidate(int32_t windowId) {
    // setNeedsDisplay: must be called on the main thread. dispatch_async to
    // the main queue makes this safe from any goroutine without requiring the
    // Go caller to use mainthread.Call (which would be too expensive for the
    // high-frequency repaint path).
    dispatch_async(dispatch_get_main_queue(), ^{
        UIWindow* win = FindWindowById(windowId);
        if (!win) return;
        [win->renderView setNeedsDisplay:YES];
    });
}

// Called by the Go side (flattenAndExecute) to actually draw commands.
// Runs on the main thread inside drawRect: (via uiDarwinOnDraw) or from the
// Go Render method (rare path).
void uiWindowRender(int32_t windowId, const CDrawCommand* commands, int32_t count) {
    UIWindow* win = FindWindowById(windowId);
    if (!win) return;
    NSGraphicsContext *nsCtx = [NSGraphicsContext currentContext];
    CGContextRef ctx = nsCtx ? [nsCtx CGContext] : NULL;
    if (!ctx) return;
    ExecuteCommands(win, ctx, commands, count);
}

// ── Text measurement (CoreText) ─────────────────────────────────────────

CMeasureResult uiMeasureText(const char* text, int32_t textLen, float fontSize, const char* fontFamily, int32_t familyLen) {
    CMeasureResult result = { 0, fontSize * 1.2f };
    if (!text || textLen <= 0) return result;

    CFStringRef textStr = CFStringCreateWithBytes(kCFAllocatorDefault,
        (const UInt8*)text, textLen, kCFStringEncodingUTF8, false);
    if (!textStr) return result;

    CFStringRef familyStr = NULL;
    if (fontFamily && familyLen > 0) {
        familyStr = CFStringCreateWithBytes(kCFAllocatorDefault,
            (const UInt8*)fontFamily, familyLen, kCFStringEncodingUTF8, false);
    }
    if (!familyStr) {
        familyStr = CFSTR("PingFang SC");
    }

    CGFloat size = fontSize > 0 ? fontSize : 16.0f;
    CTFontRef font = CTFontCreateWithName(familyStr, size, NULL);

    CFStringRef keys[] = { kCTFontAttributeName };
    CFTypeRef values[] = { font };
    CFDictionaryRef attrs = CFDictionaryCreate(kCFAllocatorDefault,
        (const void**)keys, (const void**)values, 1,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFAttributedStringRef attrStr = CFAttributedStringCreate(kCFAllocatorDefault, textStr, attrs);

    CTLineRef line = CTLineCreateWithAttributedString(attrStr);

    CGFloat ascent, descent, leading;
    result.width = (float)CTLineGetTypographicBounds(line, &ascent, &descent, &leading);
    result.height = (float)(ascent + descent);

    CFRelease(line);
    CFRelease(attrStr);
    CFRelease(attrs);
    CFRelease(font);
    if (familyStr != CFSTR("PingFang SC")) CFRelease(familyStr);
    CFRelease(textStr);
    return result;
}
