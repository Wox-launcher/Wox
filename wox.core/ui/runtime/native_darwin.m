//go:build darwin

#import "native_darwin.h"

#import <Cocoa/Cocoa.h>
#import <CoreText/CoreText.h>
#import <CoreVideo/CoreVideo.h>
#import <IOSurface/IOSurface.h>
#import <QuartzCore/CALayer.h>
#import <QuartzCore/CAMediaTimingFunction.h>
#import <QuartzCore/CATransaction.h>
#import <WebKit/WebKit.h>
#import <dispatch/dispatch.h>

#include <dlfcn.h>
#include <math.h>
#include <stdbool.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

extern int32_t woxGoDarwinStart(uintptr_t context);
extern void woxGoDarwinCloseRequested(uintptr_t context);
extern void woxGoDarwinProtocolURL(uintptr_t context, const char *url);
extern void woxGoDarwinWebViewHideRequested(uintptr_t context);
extern void woxGoDarwinWebViewTooltip(uintptr_t context, int32_t visible, const char *text, float x, float y, float width, float height);
extern void woxGoDarwinWebViewNavigationChanged(uintptr_t context, const char *url, int32_t can_go_back, int32_t can_go_forward);
extern void woxGoDarwinCall(uintptr_t context);
extern void woxGoDarwinFrame(uintptr_t context, float width, float height, int32_t pixel_width, int32_t pixel_height, float scale);
extern void woxGoDarwinFrameSync(uintptr_t context, float width, float height, int32_t pixel_width, int32_t pixel_height, float scale, int32_t transactional);
extern void woxGoDarwinPresentationDiagnostic(uintptr_t context, uint64_t frame_id, uint8_t event, uint8_t renderer_kind, uint64_t sequence, uint64_t generation, uint64_t current_generation);
extern void woxGoDarwinFocus(uintptr_t context, uint64_t epoch, int32_t active);
extern int32_t woxGoDarwinKey(uintptr_t context, const char *key, uint8_t modifiers, int32_t down, int32_t repeat, int32_t composing);
extern void woxGoDarwinWebViewEscapeDiagnostic(uintptr_t context, const char *detail);
extern void woxGoDarwinTextInput(uintptr_t context, uint8_t kind, const char *text);
extern void woxGoDarwinPointer(uintptr_t context, uint8_t kind, float x, float y, uint8_t button, float scroll_x, float scroll_y, uint8_t modifiers);
extern void woxGoDarwinFileDrop(uintptr_t context, const char *paths);
extern void woxGoDarwinFileDragEnded(uintptr_t context, int32_t status);
extern int32_t woxGoDarwinAccessibilityAction(uintptr_t context, uint64_t node_id, const char *action, const char *value);

enum {
  WOX_KEY_MODIFIER_SHIFT = 1 << 0,
  WOX_KEY_MODIFIER_CONTROL = 1 << 1,
  WOX_KEY_MODIFIER_ALT = 1 << 2,
  WOX_KEY_MODIFIER_META = 1 << 3,
  WOX_TEXT_INPUT_COMMIT = 0,
  WOX_TEXT_INPUT_COMPOSE = 1,
  WOX_POINTER_MOVE = 0,
  WOX_POINTER_ENTER = 1,
  WOX_POINTER_LEAVE = 2,
  WOX_POINTER_DOWN = 3,
  WOX_POINTER_UP = 4,
  WOX_POINTER_SCROLL = 5,
  WOX_ACCESSIBILITY_STATE_ENABLED = 1 << 0,
  WOX_ACCESSIBILITY_STATE_FOCUSABLE = 1 << 1,
  WOX_ACCESSIBILITY_STATE_FOCUSED = 1 << 2,
  WOX_ACCESSIBILITY_STATE_SELECTED = 1 << 3,
  WOX_ACCESSIBILITY_STATE_CHECKED = 1 << 4,
  WOX_ACCESSIBILITY_STATE_EXPANDED = 1 << 5,
  WOX_ACCESSIBILITY_STATE_READ_ONLY = 1 << 6,
  WOX_ACCESSIBILITY_STATE_PROTECTED = 1 << 7,
  WOX_ACCESSIBILITY_STATE_HIDDEN = 1 << 8,
  WOX_ACCESSIBILITY_ACTION_FOCUS = 1 << 0,
  WOX_ACCESSIBILITY_ACTION_ACTIVATE = 1 << 1,
  WOX_ACCESSIBILITY_ACTION_SET_VALUE = 1 << 2,
  WOX_ACCESSIBILITY_ACTION_TOGGLE = 1 << 3,
  WOX_ACCESSIBILITY_ACTION_INCREMENT = 1 << 4,
  WOX_ACCESSIBILITY_ACTION_DECREMENT = 1 << 5,
  WOX_ACCESSIBILITY_ACTION_SCROLL = 1 << 6,
  WOX_ACCESSIBILITY_ACTION_DISMISS = 1 << 7,
  WOX_RENDER_DIAGNOSTIC_WINDOW_UNAVAILABLE = 1,
  WOX_RENDER_DIAGNOSTIC_RENDERER_REPLACED = 2,
  WOX_RENDER_DIAGNOSTIC_GENERATION_MISMATCH = 3,
  WOX_RENDER_DIAGNOSTIC_STALE_SEQUENCE = 4,
  WOX_RENDER_DIAGNOSTIC_RECOVERED = 5,
  WOX_RENDERER_BACKGROUND = 0,
  WOX_RENDERER_OVERLAY = 1,
};

typedef struct WoxDarwinRenderer WoxDarwinRenderer;
@class WoxDarwinSurface;
@class WoxRenderView;
@class WoxWindowDelegate;
@class WoxWebViewToolbar;
@class WoxResultDragSource;

typedef struct {
  uint64_t image_id;
  uint64_t byte_size;
  uint64_t last_used;
  CGImageRef image;
} WoxCachedCGImage;

struct WoxDarwinWindow {
  NSWindow *window;
  WoxRenderView *view;
  WoxWindowDelegate *delegate;
  WoxDarwinRenderer *renderer;
  WoxDarwinRenderer *overlay_renderer;
  WoxDarwinRenderer *active_renderer;
  NSMutableDictionary *web_view_cache;
  NSMutableDictionary *web_view_signatures;
  NSMutableDictionary *web_view_content_keys;
  WKWebView *active_web_view;
  WoxWebViewToolbar *web_view_toolbar;
  NSString *active_web_view_key;
  NSString *active_web_view_signature;
  NSString *active_web_view_content_key;
  bool active_web_view_transient;
  uintptr_t context;
  uint64_t epoch;
  bool visible;
  bool active;
  NSRunningApplication *previous_active_app;
  bool restore_previous_app_on_hide;
  bool hide_on_blur;
  bool screenshot_window;
  bool nonactivating;
  bool native_dialog_active;
  bool input_enabled;
  uint8_t pointer_cursor;
  NSCursor *web_view_cursor;
  bool pointer_over_web_view;
  bool closed;
  bool render_scheduled;
  bool animation_frame_pending;
  CVDisplayLinkRef display_link;
  CGDirectDisplayID display_link_display;
  bool suppress_resize_render;
  bool synchronous_frame;
  bool embedded_surface_overlay_active;
  bool forwarding_embedded_pointer;
  atomic_uint_fast64_t presentation_generation;
  NSRect input_cursor_rect;
  NSMutableDictionary *accessibility_elements;
  NSMutableDictionary *accessibility_child_ids;
  NSMutableArray *accessibility_roots;
  uint64_t accessibility_generation;
  WoxResultDragSource *result_drag_source;
  WoxCachedCGImage cached_images[32];
  int32_t cached_image_count;
  uint64_t cached_image_bytes;
  uint64_t cached_image_use_serial;
  CGImageRef cached_large_image;
  uint64_t cached_large_image_id;
  uint64_t cached_large_image_bytes;
  uint64_t large_image_candidate_id;
  int32_t large_image_candidate_frames;
  bool cache_large_images;
  WoxRendererResourceStats frame_resource_stats;
};

typedef struct {
  uint64_t sequence;
  CGRect damage;
  bool full;
} WoxDarwinDamageRecord;

struct WoxDarwinRenderer {
  CALayer *layer;
  CALayer *content_layer;
  NSMutableArray *render_surfaces;
  WoxDarwinSurface *frame_surface;
  WoxDarwinSurface *front_surface;
  CGContextRef context;
  CGSize viewport_size;
  float scale;
  uint64_t frame_id;
  uint64_t frame_generation;
  uint64_t submission_sequence;
  uint64_t presented_sequence;
  uint64_t frame_sequence;
  CGRect frame_requested_damage;
  bool frame_requested_full;
  WoxDarwinDamageRecord damage_history[64];
  bool frame_open;
  bool clip_active;
  CGRect clip_rect;
  bool damage_clip_active;
  bool diagnostic_recovery_pending;
};

static NSInteger wox_open_window_count = 0;
static const CGFloat wox_window_corner_radius = 14.0;
static CGFloat desktop_top(void);

// save_previous_active_app_if_needed keeps the app-level focus target used by the legacy Flutter launcher.
static void save_previous_active_app_if_needed(WoxDarwinWindow *window) {
  if (window == NULL) {
    return;
  }

  NSRunningApplication *front_app = [[NSWorkspace sharedWorkspace] frontmostApplication];
  NSRunningApplication *current_app = [NSRunningApplication currentApplication];
  if (front_app == nil || front_app == current_app || front_app.isTerminated) {
    return;
  }

  [window->previous_active_app release];
  window->previous_active_app = [front_app retain];
  window->restore_previous_app_on_hide = true;
}

// clear_previous_active_app releases a saved app when its restore lifetime ends.
static void clear_previous_active_app(WoxDarwinWindow *window) {
  if (window == NULL) {
    return;
  }
  [window->previous_active_app release];
  window->previous_active_app = nil;
  window->restore_previous_app_on_hide = false;
}

// restore_previous_active_app returns focus to the app that was frontmost before Wox was shown.
static void restore_previous_active_app(WoxDarwinWindow *window, BOOL was_wox_frontmost) {
  if (window == NULL) {
    return;
  }

  if (was_wox_frontmost && window->restore_previous_app_on_hide) {
    NSRunningApplication *previous_app = window->previous_active_app;
    NSRunningApplication *current_app = [NSRunningApplication currentApplication];
    if (previous_app != nil && previous_app != current_app && !previous_app.isTerminated) {
      if (@available(macOS 14.0, *)) {
        [previous_app activateWithOptions:0];
      } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        [previous_app activateWithOptions:NSApplicationActivateIgnoringOtherApps];
#pragma clang diagnostic pop
      }
    }
  }
  clear_previous_active_app(window);
}

// Resolve the compositor capture entry point dynamically because newer SDKs hide the declaration
// even though supported Wox targets still expose it at runtime.
static CGImageRef capture_display_image(CGDirectDisplayID display_id) {
  typedef CGImageRef (*WoxDesktopCaptureFunction)(CGRect, CGWindowListOption, CGWindowID, CGWindowImageOption);
  WoxDesktopCaptureFunction capture = (WoxDesktopCaptureFunction)dlsym(RTLD_DEFAULT, "CGWindowListCreateImage");
  return capture == NULL ? NULL : capture(CGDisplayBounds(display_id), kCGWindowListOptionOnScreenOnly, kCGNullWindowID, kCGWindowImageBestResolution);
}

@interface WoxNativeWindow : NSWindow
@property(nonatomic, assign) BOOL woxNonactivating;
@end

@implementation WoxNativeWindow
- (BOOL)canBecomeKeyWindow {
  return !self.woxNonactivating;
}

- (BOOL)canBecomeMainWindow {
  return !self.woxNonactivating;
}
@end

@interface WoxApplicationDelegate : NSObject <NSApplicationDelegate> {
  uintptr_t _context;
}
- (instancetype)initWithContext:(uintptr_t)context;
@end

@implementation WoxApplicationDelegate
- (instancetype)initWithContext:(uintptr_t)context {
  self = [super init];
  if (self != nil) {
    _context = context;
  }
  return self;
}

- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls {
  (void)application;
  for (NSURL *url in urls) {
    if (![[url.scheme lowercaseString] isEqualToString:@"wox"]) {
      continue;
    }
    NSString *absolute_string = url.absoluteString;
    if (absolute_string.length > 0) {
      woxGoDarwinProtocolURL(_context, absolute_string.UTF8String);
    }
  }
}
@end

@interface WoxScreenshotDisplayCapture : NSObject {
@public
  CGDirectDisplayID display_id;
  NSRect logical_bounds;
  CGImageRef image;
}
- (instancetype)initWithScreen:(NSScreen *)screen desktopTop:(CGFloat)desktop_top;
@end

@implementation WoxScreenshotDisplayCapture
- (instancetype)initWithScreen:(NSScreen *)screen desktopTop:(CGFloat)desktop_top_value {
  self = [super init];
  if (self == nil) {
    return nil;
  }
  NSNumber *screen_number = [screen.deviceDescription objectForKey:@"NSScreenNumber"];
  if (screen_number == nil) {
    [self release];
    return nil;
  }
  display_id = (CGDirectDisplayID)screen_number.unsignedIntValue;
  NSRect frame = screen.frame;
  logical_bounds = NSMakeRect(NSMinX(frame), desktop_top_value - NSMaxY(frame), NSWidth(frame), NSHeight(frame));
  image = capture_display_image(display_id);
  if (image == NULL) {
    [self release];
    return nil;
  }
  return self;
}

- (void)dealloc {
  if (image != NULL) {
    CGImageRelease(image);
  }
  [super dealloc];
}
@end

@interface WoxScreenshotSelectionView : NSView {
  WoxScreenshotDisplayCapture *_capture;
  NSRect _global_selection;
  BOOL _has_selection;
}
- (instancetype)initWithCapture:(WoxScreenshotDisplayCapture *)capture;
- (void)setGlobalSelection:(NSRect)selection visible:(BOOL)visible;
@end

@implementation WoxScreenshotSelectionView
- (instancetype)initWithCapture:(WoxScreenshotDisplayCapture *)capture {
  self = [super initWithFrame:NSMakeRect(0.0, 0.0, NSWidth(capture->logical_bounds), NSHeight(capture->logical_bounds))];
  if (self == nil) {
    return nil;
  }
  _capture = [capture retain];
  _global_selection = NSZeroRect;
  _has_selection = NO;
  return self;
}

- (BOOL)isFlipped {
  return YES;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
  (void)event;
  return YES;
}

- (void)resetCursorRects {
  [self discardCursorRects];
  [self addCursorRect:self.bounds cursor:[NSCursor crosshairCursor]];
}

- (void)setGlobalSelection:(NSRect)selection visible:(BOOL)visible {
  _global_selection = selection;
  _has_selection = visible;
  self.needsDisplay = YES;
}

- (void)drawRect:(NSRect)dirty_rect {
  (void)dirty_rect;
  CGContextRef context = NSGraphicsContext.currentContext.CGContext;
  if (context == NULL || _capture->image == NULL) {
    return;
  }

  CGContextSaveGState(context);
  CGContextSetInterpolationQuality(context, kCGInterpolationHigh);
  CGContextTranslateCTM(context, 0.0, NSHeight(self.bounds));
  CGContextScaleCTM(context, 1.0, -1.0);
  CGContextDrawImage(context, NSMakeRect(0.0, 0.0, NSWidth(self.bounds), NSHeight(self.bounds)), _capture->image);
  CGContextRestoreGState(context);

  NSBezierPath *mask = [NSBezierPath bezierPathWithRect:self.bounds];
  NSRect intersection = NSIntersectionRect(_capture->logical_bounds, _global_selection);
  if (_has_selection && !NSIsEmptyRect(intersection)) {
    NSRect local_selection = NSMakeRect(
        NSMinX(intersection) - NSMinX(_capture->logical_bounds),
        NSMinY(intersection) - NSMinY(_capture->logical_bounds),
        NSWidth(intersection),
        NSHeight(intersection));
    [mask appendBezierPathWithRect:local_selection];
    mask.windingRule = NSEvenOddWindingRule;
    [[NSColor colorWithCalibratedWhite:0.0 alpha:0.46] setFill];
    [mask fill];
    [[NSColor colorWithCalibratedRed:41.0 / 255.0 green:1.0 blue:114.0 / 255.0 alpha:1.0] setStroke];
    NSBezierPath *border = [NSBezierPath bezierPathWithRect:NSInsetRect(local_selection, 1.0, 1.0)];
    border.lineWidth = 2.0;
    [border stroke];
    return;
  }

  [[NSColor colorWithCalibratedWhite:0.0 alpha:0.46] setFill];
  [mask fill];
}

- (void)dealloc {
  [_capture release];
  [super dealloc];
}
@end

@interface WoxScreenshotSelectionWindow : WoxNativeWindow
@end

@implementation WoxScreenshotSelectionWindow
@end

@interface WoxScreenshotSelectionSession : NSObject {
  NSArray *_captures;
  NSArray *_windows;
  id _event_monitor;
  dispatch_semaphore_t _completion;
  NSPoint _drag_start;
  BOOL _dragging;
  BOOL _completed;
  BOOL _cancelled;
  BOOL _dismissed;
  WoxScreenshotDisplayCapture *_drag_capture;
  WoxScreenshotDisplayCapture *_selected_capture;
  NSRect _selection;
}
- (instancetype)initWithCaptures:(NSArray *)captures;
- (void)begin;
- (void)dismiss;
- (BOOL)cancelled;
- (WoxScreenshotDisplayCapture *)selectedCapture;
- (NSRect)selection;
- (dispatch_semaphore_t)completion;
@end

@implementation WoxScreenshotSelectionSession
- (instancetype)initWithCaptures:(NSArray *)captures {
  self = [super init];
  if (self == nil) {
    return nil;
  }
  _captures = [captures copy];
  _completion = dispatch_semaphore_create(0);
  NSMutableArray *windows = [NSMutableArray arrayWithCapacity:_captures.count];
  for (WoxScreenshotDisplayCapture *capture in _captures) {
    WoxScreenshotSelectionWindow *window = [[WoxScreenshotSelectionWindow alloc]
        initWithContentRect:NSMakeRect(
                                NSMinX(capture->logical_bounds),
                                desktop_top() - NSMaxY(capture->logical_bounds),
                                NSWidth(capture->logical_bounds),
                                NSHeight(capture->logical_bounds))
                  styleMask:NSWindowStyleMaskBorderless
                    backing:NSBackingStoreBuffered
                      defer:NO];
    window.releasedWhenClosed = NO;
    window.opaque = NO;
    window.backgroundColor = [NSColor clearColor];
    window.hasShadow = NO;
    window.acceptsMouseMovedEvents = YES;
    window.animationBehavior = NSWindowAnimationBehaviorNone;
    window.level = MAX(NSScreenSaverWindowLevel, CGShieldingWindowLevel());
    NSWindowCollectionBehavior behavior =
        NSWindowCollectionBehaviorCanJoinAllSpaces |
        NSWindowCollectionBehaviorFullScreenAuxiliary |
        NSWindowCollectionBehaviorStationary |
        NSWindowCollectionBehaviorIgnoresCycle;
    if (@available(macOS 13.0, *)) {
      behavior |= NSWindowCollectionBehaviorCanJoinAllApplications;
    }
    window.collectionBehavior = behavior;
    WoxScreenshotSelectionView *view = [[WoxScreenshotSelectionView alloc] initWithCapture:capture];
    window.contentView = view;
    [view release];
    [windows addObject:window];
    [window release];
  }
  _windows = [windows copy];
  return self;
}

- (NSPoint)topLeftMouseLocation {
  NSPoint location = NSEvent.mouseLocation;
  return NSMakePoint(location.x, desktop_top() - location.y);
}

- (NSPoint)clampPoint:(NSPoint)point toBounds:(NSRect)bounds {
  return NSMakePoint(
      MIN(MAX(point.x, NSMinX(bounds)), NSMaxX(bounds)),
      MIN(MAX(point.y, NSMinY(bounds)), NSMaxY(bounds)));
}

- (WoxScreenshotDisplayCapture *)captureAtPoint:(NSPoint)point {
  for (WoxScreenshotDisplayCapture *capture in _captures) {
    if (NSPointInRect(point, capture->logical_bounds)) {
      return capture;
    }
  }
  return nil;
}

- (NSRect)rectFromStart:(NSPoint)start end:(NSPoint)end {
  return NSMakeRect(
      MIN(start.x, end.x),
      MIN(start.y, end.y),
      fabs(end.x - start.x),
      fabs(end.y - start.y));
}

- (void)updateSelection:(NSRect)selection visible:(BOOL)visible {
  for (WoxScreenshotSelectionWindow *window in _windows) {
    [(WoxScreenshotSelectionView *)window.contentView setGlobalSelection:selection visible:visible];
  }
}

- (void)completeCancelled:(BOOL)cancelled selection:(NSRect)selection {
  if (_completed) {
    return;
  }
  if (!cancelled && (NSWidth(selection) < 2.0 || NSHeight(selection) < 2.0)) {
    cancelled = YES;
  }
  _completed = YES;
  _cancelled = cancelled;
  _dragging = NO;
  if (_event_monitor != nil) {
    [NSEvent removeMonitor:_event_monitor];
    _event_monitor = nil;
  }
  if (!cancelled) {
    WoxScreenshotDisplayCapture *capture = _drag_capture;
    NSRect local_selection = NSIntersectionRect(capture->logical_bounds, selection);
    _selected_capture = [capture retain];
    _selection = NSMakeRect(
        NSMinX(local_selection) - NSMinX(capture->logical_bounds),
        NSMinY(local_selection) - NSMinY(capture->logical_bounds),
        NSWidth(local_selection),
        NSHeight(local_selection));
  }
  dispatch_semaphore_signal(_completion);
}

- (void)begin {
  WoxScreenshotSelectionSession *session = self;
  _event_monitor = [NSEvent addLocalMonitorForEventsMatchingMask:
                                NSEventMaskLeftMouseDown |
                                NSEventMaskLeftMouseDragged |
                                NSEventMaskLeftMouseUp |
                                NSEventMaskKeyDown
                                                        handler:^NSEvent *(NSEvent *event) {
    if (session->_completed) {
      return nil;
    }
    if (event.type == NSEventTypeKeyDown) {
      if (event.keyCode == 53) {
        [session completeCancelled:YES selection:NSZeroRect];
      }
      return nil;
    }
    NSPoint mouse_location = [session topLeftMouseLocation];
    if (event.type == NSEventTypeLeftMouseDown) {
      session->_drag_capture = [session captureAtPoint:mouse_location];
      if (session->_drag_capture == nil) {
        return nil;
      }
      NSPoint point = [session clampPoint:mouse_location toBounds:session->_drag_capture->logical_bounds];
      session->_drag_start = point;
      session->_dragging = YES;
      [session updateSelection:[session rectFromStart:point end:point] visible:YES];
      return nil;
    }
    if (event.type == NSEventTypeLeftMouseDragged && session->_dragging) {
      // The portable annotation editor owns one display after handoff. Keeping the native drag on
      // its starting display avoids presenting a cross-display selection that export would crop.
      NSPoint point = [session clampPoint:mouse_location toBounds:session->_drag_capture->logical_bounds];
      [session updateSelection:[session rectFromStart:session->_drag_start end:point] visible:YES];
      return nil;
    }
    if (event.type == NSEventTypeLeftMouseUp && session->_dragging) {
      NSPoint point = [session clampPoint:mouse_location toBounds:session->_drag_capture->logical_bounds];
      NSRect selection = [session rectFromStart:session->_drag_start end:point];
      [session updateSelection:selection visible:YES];
      [session completeCancelled:NO selection:selection];
      return nil;
    }
    return event;
  }];
  for (WoxScreenshotSelectionWindow *window in _windows) {
    [window orderFrontRegardless];
  }
  WoxScreenshotSelectionWindow *first = _windows.firstObject;
  [first makeKeyAndOrderFront:nil];
  [NSApp activateIgnoringOtherApps:YES];
}

- (void)dismiss {
  if (_dismissed) {
    return;
  }
  _dismissed = YES;
  if (_event_monitor != nil) {
    [NSEvent removeMonitor:_event_monitor];
    _event_monitor = nil;
  }
  for (WoxScreenshotSelectionWindow *window in _windows) {
    [window orderOut:nil];
    window.contentView = nil;
    [window close];
  }
}

- (BOOL)cancelled {
  return _cancelled;
}

- (WoxScreenshotDisplayCapture *)selectedCapture {
  return _selected_capture;
}

- (NSRect)selection {
  return _selection;
}

- (dispatch_semaphore_t)completion {
  return _completion;
}

- (void)dealloc {
  [self dismiss];
  [_selected_capture release];
  [_windows release];
  [_captures release];
#if !OS_OBJECT_USE_OBJC
  dispatch_release(_completion);
#endif
  [super dealloc];
}
@end

@interface WoxDarwinSurface : NSObject {
@public
  IOSurfaceRef io_surface;
  NSUInteger width;
  NSUInteger height;
  atomic_uint presentation_references;
  uint64_t content_sequence;
}
- (instancetype)initWithWidth:(NSUInteger)width height:(NSUInteger)height;
@end

@implementation WoxDarwinSurface
// initWithWidth creates the shared CPU/Core Animation backing store for one window size.
- (instancetype)initWithWidth:(NSUInteger)surface_width height:(NSUInteger)surface_height {
  self = [super init];
  if (self == nil) {
    return nil;
  }
  width = surface_width;
  height = surface_height;
  atomic_init(&presentation_references, 0);

  const size_t bytes_per_element = 4;
  size_t bytes_per_row = IOSurfaceAlignProperty(kIOSurfaceBytesPerRow, width * bytes_per_element);
  size_t allocation_size = IOSurfaceAlignProperty(kIOSurfaceAllocSize, height * bytes_per_row);
  NSDictionary *properties = @{
    (id)kIOSurfaceWidth : @(width),
    (id)kIOSurfaceHeight : @(height),
    (id)kIOSurfacePixelFormat : @(kCVPixelFormatType_32BGRA),
    (id)kIOSurfaceBytesPerElement : @(bytes_per_element),
    (id)kIOSurfaceBytesPerRow : @(bytes_per_row),
    (id)kIOSurfaceAllocSize : @(allocation_size),
  };
  io_surface = IOSurfaceCreate((CFDictionaryRef)properties);
  if (io_surface == NULL) {
    [self release];
    return nil;
  }
  CGColorSpaceRef color_space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
  if (color_space != NULL) {
    CFPropertyListRef color_space_properties = CGColorSpaceCopyPropertyList(color_space);
    if (color_space_properties != NULL) {
      IOSurfaceSetValue(io_surface, kIOSurfaceColorSpace, color_space_properties);
      CFRelease(color_space_properties);
    }
    CGColorSpaceRelease(color_space);
  }
  return self;
}

- (void)dealloc {
  if (io_surface != NULL) {
    CFRelease(io_surface);
  }
  [super dealloc];
}
@end

@interface WoxRenderView : NSView <NSTextInputClient, NSDraggingDestination> {
@public
  WoxDarwinWindow *_owner;
  NSString *_marked_text;
  NSRange _marked_selection;
  NSTrackingArea *_tracking_area;
  NSArray *_accessibility_children;
}
- (void)updateBackingScale;
- (void)renderFrame;
- (void)renderFrameSynchronously:(BOOL)transactional;
- (void)setWoxAccessibilityChildren:(NSArray *)children;
@end

@interface WoxResultDragSource : NSObject <NSDraggingSource> {
@public
  WoxDarwinWindow *_owner;
}
- (instancetype)initWithOwner:(WoxDarwinWindow *)owner;
@end

@implementation WoxResultDragSource
- (instancetype)initWithOwner:(WoxDarwinWindow *)owner {
  self = [super init];
  if (self != nil) {
    _owner = owner;
  }
  return self;
}

- (NSDragOperation)draggingSession:(NSDraggingSession *)session sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
  (void)session;
  (void)context;
  return NSDragOperationCopy;
}

- (void)draggingSession:(NSDraggingSession *)session endedAtPoint:(NSPoint)screenPoint operation:(NSDragOperation)operation {
  (void)session;
  if (_owner == NULL || _owner->closed) {
    [self release];
    return;
  }
  NSRect source_frame = _owner->window.frame;
  int32_t status = NSPointInRect(screenPoint, source_frame) ? 2 : (operation & NSDragOperationCopy) != 0 ? 0 : 1;
  if (status != 2) {
    [_owner->window orderOut:nil];
  }
  woxGoDarwinFileDragEnded(_owner->context, status);
  _owner->result_drag_source = NULL;
  [self release];
}
@end

@interface WoxAccessibilityElement : NSAccessibilityElement {
@public
  WoxDarwinWindow *_owner;
  uint64_t _node_id;
  uint32_t _action_flags;
  BOOL _configuring;
}
@end

@interface WoxWindowDelegate : NSObject <NSWindowDelegate> {
@public
  WoxDarwinWindow *_owner;
}
@end

static void stop_darwin_display_link(WoxDarwinWindow *window);
static void bind_darwin_display_link(WoxDarwinWindow *window);

// schedule_render coalesces ordinary state changes into one frame on the next main-queue turn.
static void schedule_render(WoxDarwinWindow *window) {
  if (window == NULL) {
    return;
  }
  dispatch_block_t request = ^{
    if (window->closed || !window->visible || window->view == nil || window->context == 0 || window->render_scheduled) {
      return;
    }
    window->render_scheduled = true;
    dispatch_async(dispatch_get_main_queue(), ^{
      if (!window->render_scheduled) {
        return;
      }
      window->render_scheduled = false;
      if (!window->closed && window->visible && window->view != nil && window->context != 0) {
        [window->view renderFrame];
      }
    });
  };
  if ([NSThread isMainThread]) {
    request();
  } else {
    dispatch_async(dispatch_get_main_queue(), request);
  }
}

// render_resize_frame presents the target-size surface in the current Core Animation transaction.
static void render_resize_frame(WoxDarwinWindow *window) {
  if (window == NULL || window->closed || !window->visible || window->view == nil || window->context == 0) {
    return;
  }
  window->render_scheduled = false;
  atomic_fetch_add_explicit(&window->presentation_generation, 1, memory_order_relaxed);
  [window->view renderFrameSynchronously:YES];
}

// run_on_main_sync serializes all AppKit access while allowing UI callbacks to reenter directly.
static void run_on_main_sync(dispatch_block_t block) {
  if ([NSThread isMainThread]) {
    block();
  } else {
    dispatch_sync(dispatch_get_main_queue(), block);
  }
}

// create_renderer owns a bounded IOSurface pool without creating a Metal command queue.
static WoxDarwinRenderer *create_renderer(CALayer *layer) {
  WoxDarwinRenderer *renderer = calloc(1, sizeof(WoxDarwinRenderer));
  if (renderer == NULL) {
    return NULL;
  }
  renderer->layer = layer;
  renderer->render_surfaces = [[NSMutableArray alloc] init];
  renderer->content_layer = [[CALayer alloc] init];
  renderer->content_layer.opaque = NO;
  renderer->content_layer.needsDisplayOnBoundsChange = NO;
  renderer->scale = 1.0f;
  [layer addSublayer:renderer->content_layer];
  return renderer;
}

static void clear_renderer_surfaces(WoxDarwinRenderer *renderer) {
  renderer->content_layer.contents = nil;
  if (renderer->front_surface != nil) {
    atomic_fetch_sub_explicit(&renderer->front_surface->presentation_references, 1, memory_order_relaxed);
    renderer->front_surface = nil;
  }
  [renderer->render_surfaces removeAllObjects];
}

static const size_t wox_cached_image_max_count = 32;
static const uint64_t wox_cached_image_max_bytes = 8ULL * 1024ULL * 1024ULL;
static const uint64_t wox_cached_image_max_entry_bytes = 1ULL * 1024ULL * 1024ULL;
static const uint64_t wox_cached_large_image_max_bytes = 32ULL * 1024ULL * 1024ULL;

static void release_owned_pixels(void *info, const void *data, size_t size) {
  (void)info;
  (void)size;
  free((void *)data);
}

static void release_cached_cgimage(WoxCachedCGImage *entry) {
  if (entry == NULL || entry->image == NULL) {
    return;
  }
  CGImageRelease(entry->image);
  entry->image = NULL;
  entry->image_id = 0;
  entry->byte_size = 0;
  entry->last_used = 0;
}

static void clear_cached_large_image(WoxDarwinWindow *window) {
  if (window->cached_large_image != NULL) {
    CGImageRelease(window->cached_large_image);
    window->cached_large_image = NULL;
  }
  window->cached_large_image_id = 0;
  window->cached_large_image_bytes = 0;
}

// clear_cached_images drops every retained CGImage so hidden windows do not keep decoded pixels.
static void clear_cached_images(WoxDarwinWindow *window) {
  if (window == NULL) {
    return;
  }
  for (int32_t index = 0; index < window->cached_image_count; index++) {
    release_cached_cgimage(&window->cached_images[index]);
  }
  window->cached_image_count = 0;
  window->cached_image_bytes = 0;
  clear_cached_large_image(window);
  window->large_image_candidate_id = 0;
  window->large_image_candidate_frames = 0;
  window->cache_large_images = false;
}

static CGImageRef find_cached_cgimage(WoxDarwinWindow *window, uint64_t image_id) {
  for (int32_t index = 0; index < window->cached_image_count; index++) {
    if (window->cached_images[index].image_id == image_id) {
      window->cached_images[index].last_used = ++window->cached_image_use_serial;
      return window->cached_images[index].image;
    }
  }
  return NULL;
}

static void evict_oldest_cached_cgimage(WoxDarwinWindow *window) {
  if (window->cached_image_count <= 0) {
    return;
  }
  int32_t oldest = 0;
  for (int32_t index = 1; index < window->cached_image_count; index++) {
    if (window->cached_images[index].last_used < window->cached_images[oldest].last_used) {
      oldest = index;
    }
  }
  window->cached_image_bytes -= window->cached_images[oldest].byte_size;
  release_cached_cgimage(&window->cached_images[oldest]);
  window->cached_images[oldest] = window->cached_images[window->cached_image_count - 1];
  window->cached_image_count--;
  window->frame_resource_stats.cache_evictions++;
}

static bool cache_cgimage(WoxDarwinWindow *window, uint64_t image_id, uint64_t byte_size, CGImageRef image) {
  if (window == NULL || image_id == 0 || image == NULL || byte_size == 0 || byte_size > wox_cached_image_max_entry_bytes) {
    return false;
  }
  while (window->cached_image_count > 0 &&
         (window->cached_image_count >= (int32_t)wox_cached_image_max_count ||
          window->cached_image_bytes + byte_size > wox_cached_image_max_bytes)) {
    evict_oldest_cached_cgimage(window);
  }
  if (window->cached_image_count >= (int32_t)wox_cached_image_max_count ||
      window->cached_image_bytes + byte_size > wox_cached_image_max_bytes) {
    return false;
  }
  window->cached_images[window->cached_image_count++] = (WoxCachedCGImage){
      .image_id = image_id,
      .byte_size = byte_size,
      .last_used = ++window->cached_image_use_serial,
      .image = image,
  };
  window->cached_image_bytes += byte_size;
  return true;
}

// note_large_image_repeat enables one 32MiB preview slot after the same oversized image is
// redrawn for three consecutive frames.
//
// This deliberately has no timing gate, unlike the Linux path where the measured window wraps a
// real glTexImage2D upload. CGImageCreate only builds a lazy data provider, so a create-time
// measurement here costs microseconds regardless of image size and can never distinguish an
// expensive image from a cheap one; the pixel work happens later inside CGContextDrawImage.
// Gating on it would also be self-locking, because the only path that copies pixels up front is
// the one taken after caching is already enabled.
// Admission is bound to the current candidate. A different oversized image must re-earn the slot,
// otherwise it would inherit the previous winner's verdict and copy and evict on first sight, and
// two alternating previews would each pay a full pixel copy every frame.
static void note_large_image_repeat(WoxDarwinWindow *window, uint64_t image_id) {
  if (window->large_image_candidate_id != image_id) {
    window->large_image_candidate_id = image_id;
    window->large_image_candidate_frames = 1;
    window->cache_large_images = false;
    return;
  }
  window->large_image_candidate_frames++;
  if (window->large_image_candidate_frames >= 3) {
    window->cache_large_images = true;
  }
}

static CGImageRef create_cgimage_from_pixels(const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, bool copy_pixels) {
  size_t data_size = (size_t)row_stride * (size_t)image_height;
  const uint8_t *source = pixels;
  CGDataProviderReleaseDataCallback release = NULL;
  if (copy_pixels) {
    uint8_t *owned = malloc(data_size);
    if (owned == NULL) {
      return NULL;
    }
    memcpy(owned, pixels, data_size);
    source = owned;
    release = release_owned_pixels;
  }
  CGDataProviderRef provider = CGDataProviderCreateWithData(NULL, source, data_size, release);
  if (provider == NULL) {
    if (copy_pixels) {
      free((void *)source);
    }
    return NULL;
  }
  CGColorSpaceRef color_space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
  if (color_space == NULL) {
    CGDataProviderRelease(provider);
    return NULL;
  }
  CGImageRef image = CGImageCreate(
      (size_t)image_width,
      (size_t)image_height,
      8,
      32,
      (size_t)row_stride,
      color_space,
      kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big,
      provider,
      NULL,
      false,
      kCGRenderingIntentDefault);
  CGColorSpaceRelease(color_space);
  CGDataProviderRelease(provider);
  return image;
}

static void draw_cached_cgimage(WoxDarwinRenderer *renderer, CGImageRef image, float x, float y, float width, float height, float rotation_radians, float corner_radius) {
  CGContextSaveGState(renderer->context);
  CGContextTranslateCTM(renderer->context, x + width * 0.5f, y + height * 0.5f);
  CGContextRotateCTM(renderer->context, rotation_radians);
  if (corner_radius > 0.0f) {
    float radius = fminf(corner_radius, fminf(width, height) * 0.5f);
    CGPathRef clip_path = CGPathCreateWithRoundedRect(CGRectMake(-width * 0.5f, -height * 0.5f, width, height), radius, radius, NULL);
    CGContextAddPath(renderer->context, clip_path);
    CGContextClip(renderer->context);
    CGPathRelease(clip_path);
  }
  CGContextScaleCTM(renderer->context, 1.0, -1.0);
  CGContextDrawImage(renderer->context, CGRectMake(-width * 0.5f, -height * 0.5f, width, height), image);
  CGContextRestoreGState(renderer->context);
}

// Hidden windows keep their AppKit state but release every IOSurface so the launcher has no
// display-sized backing allocation while idle. A later frame recreates the pool on demand.
static void hide_window_and_release_surfaces(WoxDarwinWindow *window) {
  window->visible = false;
  atomic_fetch_add_explicit(&window->presentation_generation, 1, memory_order_relaxed);
  // Detach the presented IOSurfaces inside an explicitly flushed transaction
  // before ordering the window out. The window server keeps referencing the
  // surfaces that are still attached at orderOut time for the whole hidden
  // period, which pins two window-sized buffers even though the pool itself
  // is released. Flushing the cleared contents first drops that reference;
  // both messages land within one compositor frame, so no blank flash shows.
  [CATransaction begin];
  [CATransaction setDisableActions:YES];
  clear_renderer_surfaces(window->renderer);
  clear_renderer_surfaces(window->overlay_renderer);
  clear_cached_images(window);
  [CATransaction commit];
  [CATransaction flush];
  [window->window orderOut:nil];
}

// acquire_render_surface returns a retained backing store no longer owned by Core Animation.
static WoxDarwinSurface *acquire_render_surface(WoxDarwinRenderer *renderer, NSUInteger width, NSUInteger height) {
  NSMutableArray *stale_surfaces = [NSMutableArray array];
  NSUInteger matching_count = 0;
  for (WoxDarwinSurface *surface in renderer->render_surfaces) {
    if (surface->width == width && surface->height == height) {
      matching_count++;
      if (atomic_load_explicit(&surface->presentation_references, memory_order_relaxed) == 0 &&
          !IOSurfaceIsInUse(surface->io_surface)) {
        return [surface retain];
      }
    } else if (atomic_load_explicit(&surface->presentation_references, memory_order_relaxed) == 0 &&
               !IOSurfaceIsInUse(surface->io_surface)) {
      [stale_surfaces addObject:surface];
    }
  }
  [renderer->render_surfaces removeObjectsInArray:stale_surfaces];

  // Triple buffering absorbs compositor stalls while keeping the visible backing store bounded.
  if (matching_count >= 3) {
    renderer->diagnostic_recovery_pending = true;
    return nil;
  }
  WoxDarwinSurface *surface = [[WoxDarwinSurface alloc] initWithWidth:width height:height];
  if (surface != nil) {
    [renderer->render_surfaces addObject:surface];
  }
  return surface;
}

// renderer_kind identifies which bounded IOSurface pool produced a diagnostic event.
static uint8_t renderer_kind(WoxDarwinWindow *window, WoxDarwinRenderer *renderer) {
  return window->overlay_renderer == renderer ? WOX_RENDERER_OVERLAY : WOX_RENDERER_BACKGROUND;
}

// report_presentation_diagnostic forwards only rejected or recovered asynchronous presentations to Go logging.
static void report_presentation_diagnostic(WoxDarwinWindow *window, WoxDarwinRenderer *renderer, uint64_t frame_id, uint8_t event, uint64_t sequence, uint64_t generation) {
  if (window == NULL || renderer == NULL || window->context == 0) {
    return;
  }
  woxGoDarwinPresentationDiagnostic(
      window->context,
      frame_id,
      event,
      renderer_kind(window, renderer),
      sequence,
      generation,
      atomic_load_explicit(&window->presentation_generation, memory_order_relaxed));
}

// present_render_surface joins resize frames to the caller's transaction and owns ordinary frame transactions.
static void present_render_surface(WoxDarwinWindow *window, WoxDarwinRenderer *renderer, WoxDarwinSurface *surface, uint64_t frame_id, uint64_t sequence, uint64_t generation, bool transactional) {
  uint8_t rejected_event = 0;
  if (window->closed || !window->visible) {
    rejected_event = WOX_RENDER_DIAGNOSTIC_WINDOW_UNAVAILABLE;
  } else if (window->renderer != renderer && window->overlay_renderer != renderer) {
    rejected_event = WOX_RENDER_DIAGNOSTIC_RENDERER_REPLACED;
  } else if (atomic_load_explicit(&window->presentation_generation, memory_order_relaxed) != generation) {
    rejected_event = WOX_RENDER_DIAGNOSTIC_GENERATION_MISMATCH;
  } else if (sequence <= renderer->presented_sequence) {
    rejected_event = WOX_RENDER_DIAGNOSTIC_STALE_SEQUENCE;
  }
  if (rejected_event != 0) {
    atomic_fetch_sub_explicit(&surface->presentation_references, 1, memory_order_relaxed);
    renderer->diagnostic_recovery_pending = true;
    report_presentation_diagnostic(window, renderer, frame_id, rejected_event, sequence, generation);
    return;
  }

  if (renderer->front_surface == surface) {
    atomic_fetch_sub_explicit(&surface->presentation_references, 1, memory_order_relaxed);
  } else {
    if (renderer->front_surface != nil) {
      atomic_fetch_sub_explicit(&renderer->front_surface->presentation_references, 1, memory_order_relaxed);
    }
    renderer->front_surface = surface;
  }
  renderer->presented_sequence = sequence;
  if (!transactional) {
    [CATransaction begin];
    [CATransaction setDisableActions:YES];
  }
  renderer->content_layer.frame = renderer->layer.bounds;
  renderer->content_layer.contentsScale = renderer->layer.contentsScale;
  renderer->content_layer.contents = (__bridge id)surface->io_surface;
  if (!transactional) {
    [CATransaction commit];
  }
  if (renderer->diagnostic_recovery_pending) {
    renderer->diagnostic_recovery_pending = false;
    report_presentation_diagnostic(window, renderer, frame_id, WOX_RENDER_DIAGNOSTIC_RECOVERED, sequence, generation);
  }
}

static void destroy_renderer(WoxDarwinRenderer *renderer) {
  if (renderer == NULL) {
    return;
  }
  if (renderer->frame_open) {
    if (renderer->clip_active) {
      CGContextRestoreGState(renderer->context);
    }
    if (renderer->damage_clip_active) {
      CGContextRestoreGState(renderer->context);
    }
    CGContextRelease(renderer->context);
    IOSurfaceUnlock(renderer->frame_surface->io_surface, 0, NULL);
    [renderer->frame_surface release];
    renderer->frame_surface = nil;
  }
  clear_renderer_surfaces(renderer);
  [renderer->content_layer removeFromSuperlayer];
  [renderer->content_layer release];
  [renderer->render_surfaces release];
  free(renderer);
}

static void emit_focus(WoxDarwinWindow *window, bool active) {
  if (window == NULL || window->closed || window->active == active) {
    return;
  }
  window->active = active;
  uintptr_t context = window->context;
  if (context != 0) {
    woxGoDarwinFocus(context, window->epoch, active ? 1 : 0);
  }
}

static NSString *web_view_string(const char *value) {
  if (value == NULL || value[0] == '\0') {
    return @"";
  }
  return [NSString stringWithUTF8String:value] ?: @"";
}

static CGFloat desktop_top(void);

static NSString *web_view_css_script(NSString *css) {
  if (css.length == 0) {
    return nil;
  }
  NSData *json_data = [NSJSONSerialization dataWithJSONObject:@[ css ] options:0 error:nil];
  if (json_data == nil) {
    return nil;
  }
  NSString *json = [[[NSString alloc] initWithData:json_data encoding:NSUTF8StringEncoding] autorelease];
  return [NSString stringWithFormat:
                       @"(()=>{const c=%@[0];const apply=()=>{const root=document.head||document.documentElement;"
                        "if(!root)return false;let s=document.getElementById('wox-webview-preview-style');"
                        "if(!s){s=document.createElement('style');s.id='wox-webview-preview-style';root.appendChild(s)}"
                        "s.textContent=c;return true};if(!apply()){const observer=new MutationObserver(()=>{"
                        "if(apply())observer.disconnect()});observer.observe(document,{childList:true})}})()",
                       json];
}

@interface WoxWebViewToolbarBackground : NSVisualEffectView {
@public
  WoxDarwinWindow *_owner;
}
@end

@implementation WoxWebViewToolbarBackground
- (BOOL)acceptsFirstMouse:(NSEvent *)event {
  (void)event;
  return YES;
}

- (void)mouseDown:(NSEvent *)event {
  if (_owner != NULL && !_owner->closed) {
    [_owner->window performWindowDragWithEvent:event];
  }
}

- (void)resetCursorRects {
  [super resetCursorRects];
  [self addCursorRect:self.bounds cursor:[NSCursor arrowCursor]];
}
@end

@interface WoxWebViewToolbar : NSView {
@public
  WoxDarwinWindow *_owner;
  WKWebView *_web_view;
  WoxWebViewToolbarBackground *_background;
  NSButton *_back_button;
  NSButton *_refresh_button;
  NSButton *_forward_button;
  NSButton *_open_button;
  NSButton *_hide_button;
  NSTrackingArea *_tracking_area;
  NSMutableArray *_button_tracking_areas;
  NSButton *_tooltip_button;
  NSTimer *_hide_timer;
  float _shadow_opacity;
}
- (instancetype)initWithOwner:(WoxDarwinWindow *)owner;
- (void)setWebView:(WKWebView *)web_view;
- (void)setLabelsBack:(NSString *)back
              refresh:(NSString *)refresh
              forward:(NSString *)forward
        openInBrowser:(NSString *)open_in_browser
              hideWox:(NSString *)hide_wox;
- (void)positionOverWebViewFrame:(NSRect)frame;
- (void)showTemporarily;
@end

static void *wox_web_view_toolbar_back_context = &wox_web_view_toolbar_back_context;
static void *wox_web_view_toolbar_forward_context = &wox_web_view_toolbar_forward_context;

@implementation WoxWebViewToolbar
- (instancetype)initWithOwner:(WoxDarwinWindow *)owner {
  self = [super initWithFrame:NSMakeRect(0.0, 0.0, 288.0, 72.0)];
  if (self == nil) {
    return nil;
  }
  _owner = owner;
  self.wantsLayer = YES;
  self.layer.shadowColor = NSColor.blackColor.CGColor;
  self.layer.shadowRadius = 20.0;
  self.layer.shadowOffset = CGSizeMake(0.0, -8.0);
  CGPathRef shadow_path = CGPathCreateWithRoundedRect(CGRectMake(18.0, 18.0, 252.0, 36.0), 18.0, 18.0, NULL);
  self.layer.shadowPath = shadow_path;
  CGPathRelease(shadow_path);

  _background = [[WoxWebViewToolbarBackground alloc] initWithFrame:NSMakeRect(18.0, 18.0, 252.0, 36.0)];
  _background->_owner = owner;
  _background.material = NSVisualEffectMaterialLight;
  _background.appearance = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
  _background.state = NSVisualEffectStateActive;
  _background.blendingMode = NSVisualEffectBlendingModeWithinWindow;
  _background.wantsLayer = YES;
  _background.layer.cornerRadius = 18.0;
  _background.layer.borderWidth = 1.0;
  _background.layer.masksToBounds = YES;
  [self addSubview:_background];

  _back_button = [[self toolbarButtonWithSymbol:@"arrow.left" action:@selector(goBack:)] retain];
  _refresh_button = [[self toolbarButtonWithSymbol:@"arrow.clockwise" action:@selector(refresh:)] retain];
  _forward_button = [[self toolbarButtonWithSymbol:@"arrow.right" action:@selector(goForward:)] retain];
  _open_button = [[self toolbarButtonWithSymbol:@"arrow.up.forward.app" action:@selector(openInBrowser:)] retain];
  _hide_button = [[self toolbarButtonWithSymbol:@"eye.slash" action:@selector(hideWox:)] retain];
  _button_tracking_areas = [[NSMutableArray alloc] init];
  NSArray *buttons = @[ _back_button, _refresh_button, _forward_button, _open_button, _hide_button ];
  for (NSUInteger index = 0; index < buttons.count; index++) {
    NSButton *button = buttons[index];
    button.frame = NSMakeRect(46.0 + index * 32.0, 2.0, 32.0, 32.0);
    [_background addSubview:button];
  }
  [self updateStyle];
  return self;
}

- (BOOL)isFlipped {
  return YES;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
  (void)event;
  return YES;
}

- (void)dealloc {
  [self cancelHideTimer];
  [self hideTooltip];
  [self setWebView:nil];
  [self clearButtonTrackingAreas];
  [_button_tracking_areas release];
  [_tracking_area release];
  [_back_button release];
  [_refresh_button release];
  [_forward_button release];
  [_open_button release];
  [_hide_button release];
  [_background release];
  [super dealloc];
}

- (void)updateTrackingAreas {
  [super updateTrackingAreas];
  if (_tracking_area != nil) {
    [self removeTrackingArea:_tracking_area];
    [_tracking_area release];
  }
  _tracking_area = [[NSTrackingArea alloc]
      initWithRect:NSZeroRect
           options:NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect
             owner:self
          userInfo:nil];
  [self addTrackingArea:_tracking_area];
  [self clearButtonTrackingAreas];
  for (NSButton *button in @[ _back_button, _refresh_button, _forward_button, _open_button, _hide_button ]) {
    NSTrackingArea *tracking_area = [[NSTrackingArea alloc]
        initWithRect:NSZeroRect
             options:NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect
               owner:self
            userInfo:@{ @"button" : button }];
    [button addTrackingArea:tracking_area];
    [_button_tracking_areas addObject:tracking_area];
    [tracking_area release];
  }
}

- (void)mouseEntered:(NSEvent *)event {
  [self showToolbarKeepingVisible:YES];
  NSButton *button = event.trackingArea.userInfo[@"button"];
  if (button != nil) {
    [self setButtonHighlighted:button highlighted:YES];
    [self showTooltipForButton:button];
  }
}

- (void)mouseExited:(NSEvent *)event {
  NSButton *button = event.trackingArea.userInfo[@"button"];
  if (button != nil) {
    [self setButtonHighlighted:button highlighted:NO];
    [self hideTooltip];
    return;
  }
  [self hideTooltip];
  [self scheduleHide];
}

// toolbarButtonWithSymbol creates one fixed Flutter-sized native toolbar action.
- (NSButton *)toolbarButtonWithSymbol:(NSString *)symbol action:(SEL)action {
  NSButton *button = [NSButton buttonWithImage:[NSImage imageWithSystemSymbolName:symbol accessibilityDescription:nil] target:self action:action];
  button.bordered = NO;
  button.imagePosition = NSImageOnly;
  button.imageScaling = NSImageScaleProportionallyDown;
  button.focusRingType = NSFocusRingTypeNone;
  button.wantsLayer = YES;
  return button;
}

// setButtonHighlighted paints a rounded hover backdrop on one toolbar button,
// matching the toolbar's light/dark appearance.
- (void)setButtonHighlighted:(NSButton *)button highlighted:(BOOL)highlighted {
  if (!button.enabled) {
    return;
  }
  button.layer.cornerRadius = 7.0;
  NSString *appearance = [self.effectiveAppearance bestMatchFromAppearancesWithNames:@[ NSAppearanceNameDarkAqua, NSAppearanceNameAqua ]];
  BOOL dark = [appearance isEqualToString:NSAppearanceNameDarkAqua];
  if (highlighted) {
    button.layer.backgroundColor = [NSColor colorWithWhite:0.0 alpha:dark ? 0.18 : 0.08].CGColor;
  } else {
    button.layer.backgroundColor = nil;
  }
}

- (void)setWebView:(WKWebView *)web_view {
  if (_web_view == web_view) {
    [self updateNavigationButtons];
    return;
  }
  if (_web_view != nil) {
    [_web_view removeObserver:self forKeyPath:@"canGoBack" context:wox_web_view_toolbar_back_context];
    [_web_view removeObserver:self forKeyPath:@"canGoForward" context:wox_web_view_toolbar_forward_context];
  }
  _web_view = web_view;
  if (_web_view != nil) {
    [_web_view addObserver:self forKeyPath:@"canGoBack" options:NSKeyValueObservingOptionInitial context:wox_web_view_toolbar_back_context];
    [_web_view addObserver:self forKeyPath:@"canGoForward" options:NSKeyValueObservingOptionInitial context:wox_web_view_toolbar_forward_context];
  } else {
    [self hideTooltip];
    [self updateNavigationButtons];
  }
}

- (void)observeValueForKeyPath:(NSString *)key_path
                      ofObject:(id)object
                        change:(NSDictionary<NSKeyValueChangeKey, id> *)change
                       context:(void *)context {
  (void)key_path;
  (void)object;
  (void)change;
  if (context == wox_web_view_toolbar_back_context || context == wox_web_view_toolbar_forward_context) {
    [self updateNavigationButtons];
    return;
  }
  [super observeValueForKeyPath:key_path ofObject:object change:change context:context];
}

- (void)setLabelsBack:(NSString *)back
              refresh:(NSString *)refresh
              forward:(NSString *)forward
        openInBrowser:(NSString *)open_in_browser
              hideWox:(NSString *)hide_wox {
  [self setLabel:back fallback:@"Go Back" forButton:_back_button];
  [self setLabel:refresh fallback:@"Refresh" forButton:_refresh_button];
  [self setLabel:forward fallback:@"Go Forward" forButton:_forward_button];
  [self setLabel:open_in_browser fallback:@"Open in Browser" forButton:_open_button];
  [self setLabel:hide_wox fallback:@"Hide Wox" forButton:_hide_button];
}

- (void)positionOverWebViewFrame:(NSRect)frame {
  self.frame = NSMakeRect(NSMidX(frame) - 144.0, NSMaxY(frame) - 114.0, 288.0, 72.0);
}

- (void)showTemporarily {
  [self showToolbarKeepingVisible:NO];
}

- (void)goBack:(id)sender {
  (void)sender;
  if (_web_view.canGoBack) {
    [_web_view goBack];
  }
}

- (void)refresh:(id)sender {
  (void)sender;
  [_web_view reload];
}

- (void)goForward:(id)sender {
  (void)sender;
  if (_web_view.canGoForward) {
    [_web_view goForward];
  }
}

- (void)openInBrowser:(id)sender {
  (void)sender;
  NSURL *url = _web_view.URL;
  NSString *scheme = url.scheme.lowercaseString;
  if (url.host.length > 0 && ([scheme isEqualToString:@"http"] || [scheme isEqualToString:@"https"])) {
    [[NSWorkspace sharedWorkspace] openURL:url];
  }
}

- (void)hideWox:(id)sender {
  (void)sender;
  if (_owner != NULL && !_owner->closed && _owner->context != 0) {
    woxGoDarwinWebViewHideRequested(_owner->context);
  }
}

- (void)setLabel:(NSString *)label fallback:(NSString *)fallback forButton:(NSButton *)button {
  NSString *resolved = label.length > 0 ? label : fallback;
  button.accessibilityLabel = resolved;
  button.accessibilityHelp = resolved;
}

- (void)clearButtonTrackingAreas {
  for (NSTrackingArea *tracking_area in _button_tracking_areas) {
    NSButton *button = tracking_area.userInfo[@"button"];
    [button removeTrackingArea:tracking_area];
  }
  [_button_tracking_areas removeAllObjects];
}

- (void)showTooltipForButton:(NSButton *)button {
  NSString *text = button.accessibilityHelp;
  if (button == nil || text.length == 0 || _tooltip_button == button || _owner == NULL || _owner->closed || _owner->context == 0 || button.window == nil) {
    return;
  }
  [self hideTooltip];
  _tooltip_button = button;
  NSRect window_rect = [button convertRect:button.bounds toView:nil];
  NSRect screen_rect = [button.window convertRectToScreen:window_rect];
  woxGoDarwinWebViewTooltip(
      _owner->context,
      1,
      text.UTF8String,
      (float)NSMinX(screen_rect),
      (float)(desktop_top() - NSMaxY(screen_rect)),
      (float)NSWidth(screen_rect),
      (float)NSHeight(screen_rect));
}

- (void)hideTooltip {
  if (_tooltip_button == nil) {
    return;
  }
  _tooltip_button = nil;
  if (_owner != NULL && !_owner->closed && _owner->context != 0) {
    woxGoDarwinWebViewTooltip(_owner->context, 0, "", 0.0f, 0.0f, 0.0f, 0.0f);
  }
}

- (void)updateNavigationButtons {
  _back_button.enabled = _web_view != nil && _web_view.canGoBack;
  _forward_button.enabled = _web_view != nil && _web_view.canGoForward;
}

- (void)updateStyle {
  NSString *appearance = [self.effectiveAppearance bestMatchFromAppearancesWithNames:@[ NSAppearanceNameDarkAqua, NSAppearanceNameAqua ]];
  BOOL dark = [appearance isEqualToString:NSAppearanceNameDarkAqua];
  NSColor *icon_color = [NSColor colorWithWhite:0.0 alpha:dark ? 0.82 : 0.72];
  for (NSButton *button in @[ _back_button, _refresh_button, _forward_button, _open_button, _hide_button ]) {
    button.contentTintColor = icon_color;
  }
  _background.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:dark ? 0.58 : 0.72].CGColor;
  _background.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:dark ? 0.28 : 0.46].CGColor;
  _shadow_opacity = dark ? 0.22f : 0.12f;
  self.layer.shadowOpacity = _background.alphaValue > 0.0 ? _shadow_opacity : 0.0f;
}

- (void)viewDidChangeEffectiveAppearance {
  [super viewDidChangeEffectiveAppearance];
  [self updateStyle];
}

- (void)showToolbarKeepingVisible:(BOOL)keep_visible {
  [self cancelHideTimer];
  self.hidden = NO;
  self.layer.shadowOpacity = _shadow_opacity;
  [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
    context.duration = 0.18;
    context.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseOut];
    _background.animator.alphaValue = 1.0;
  }
      completionHandler:nil];
  if (!keep_visible) {
    [self scheduleHide];
  }
}

- (void)scheduleHide {
  [self cancelHideTimer];
  _hide_timer = [[NSTimer scheduledTimerWithTimeInterval:1.2 target:self selector:@selector(hideTimerFired:) userInfo:nil repeats:NO] retain];
}

- (void)cancelHideTimer {
  if (_hide_timer == nil) {
    return;
  }
  [_hide_timer invalidate];
  [_hide_timer release];
  _hide_timer = nil;
}

- (void)hideTimerFired:(NSTimer *)timer {
  if (timer != _hide_timer) {
    return;
  }
  [self cancelHideTimer];
  self.layer.shadowOpacity = 0.0f;
  [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
    context.duration = 0.18;
    context.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseOut];
    _background.animator.alphaValue = 0.0;
  }
      completionHandler:nil];
}
@end

static void notify_darwin_webview_navigation(WoxDarwinWindow *window, WKWebView *web_view);

// darwin_web_view_cursor maps the page's normalized CSS cursor to the closest AppKit cursor.
static NSCursor *darwin_web_view_cursor(NSString *value) {
  if ([value isEqualToString:@"text"]) return [NSCursor IBeamCursor];
  if ([value isEqualToString:@"vertical-text"]) return [NSCursor IBeamCursorForVerticalLayout];
  if ([value isEqualToString:@"pointer"]) return [NSCursor pointingHandCursor];
  if ([value isEqualToString:@"crosshair"] || [value isEqualToString:@"cell"]) return [NSCursor crosshairCursor];
  if ([value isEqualToString:@"alias"] || [value isEqualToString:@"context-menu"] || [value isEqualToString:@"help"]) return [NSCursor contextualMenuCursor];
  if ([value isEqualToString:@"copy"]) return [NSCursor dragCopyCursor];
  if ([value isEqualToString:@"grab"] || [value isEqualToString:@"move"] || [value isEqualToString:@"all-scroll"]) return [NSCursor openHandCursor];
  if ([value isEqualToString:@"grabbing"]) return [NSCursor closedHandCursor];
  if ([value isEqualToString:@"not-allowed"] || [value isEqualToString:@"no-drop"]) return [NSCursor operationNotAllowedCursor];
  if ([value isEqualToString:@"col-resize"] || [value isEqualToString:@"e-resize"] || [value isEqualToString:@"w-resize"] || [value isEqualToString:@"ew-resize"]) return [NSCursor resizeLeftRightCursor];
  if ([value isEqualToString:@"row-resize"] || [value isEqualToString:@"n-resize"] || [value isEqualToString:@"s-resize"] || [value isEqualToString:@"ns-resize"]) return [NSCursor resizeUpDownCursor];
  if ([value isEqualToString:@"zoom-in"]) return [NSCursor zoomInCursor];
  if ([value isEqualToString:@"zoom-out"]) return [NSCursor zoomOutCursor];
  if ([value isEqualToString:@"none"]) {
    static NSCursor *hidden_cursor = nil;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
      NSImage *image = [[[NSImage alloc] initWithSize:NSMakeSize(1.0, 1.0)] autorelease];
      hidden_cursor = [[NSCursor alloc] initWithImage:image hotSpot:NSZeroPoint];
    });
    return hidden_cursor;
  }
  return [NSCursor arrowCursor];
}

// apply_darwin_pointer_cursor lets the active page cursor override the Go-rendered host cursor.
static NSCursor *darwin_host_pointer_cursor(uint8_t cursor) {
  switch (cursor) {
    case 1: return [NSCursor IBeamCursor];
    case 2: return [NSCursor openHandCursor];
    case 3: return [NSCursor crosshairCursor];
    case 4: return [NSCursor resizeLeftRightCursor];
    case 5: return [NSCursor resizeUpDownCursor];
    // AppKit has no public diagonal resize cursor, so use the precise crosshair fallback.
    case 6:
    case 7: return [NSCursor crosshairCursor];
    default: return [NSCursor arrowCursor];
  }
}

static void apply_darwin_pointer_cursor(WoxDarwinWindow *window) {
  if (window == NULL || window->closed) {
    return;
  }
  NSCursor *cursor = window->pointer_over_web_view && window->web_view_cursor != nil
                         ? window->web_view_cursor
                         : darwin_host_pointer_cursor(window->pointer_cursor);
  [cursor set];
  [window->window invalidateCursorRectsForView:window->view];
}

// web_view_cursor_script reports computed CSS cursors because WoxRenderView owns native hit testing above WKWebView.
static NSString *web_view_cursor_script(void) {
  return @"(()=>{if(window.__woxCursorBridgeInstalled__)return;window.__woxCursorBridgeInstalled__=true;let last='';"
          "const allowed=new Set(['auto','default','none','context-menu','help','pointer','progress','wait','cell','crosshair','text','vertical-text','alias','copy','move','no-drop','not-allowed','grab','grabbing','all-scroll','col-resize','row-resize','n-resize','e-resize','s-resize','w-resize','ne-resize','nw-resize','se-resize','sw-resize','ew-resize','ns-resize','nesw-resize','nwse-resize','zoom-in','zoom-out']);"
          "const publish=e=>{const n=e.target&&e.target.nodeType===1?e.target:document.documentElement;if(!n)return;const raw=getComputedStyle(n).cursor||'auto';const fallback=raw.split(',').pop().trim();const value=allowed.has(fallback)?fallback:'default';if(value===last)return;last=value;window.webkit.messageHandlers.woxWebViewCursor.postMessage(value)};"
          "document.addEventListener('mousemove',publish,true);document.addEventListener('mouseover',publish,true)})()";
}

@interface WoxWebViewMessageHandler : NSObject <WKScriptMessageHandler, WKNavigationDelegate, WKUIDelegate> {
@public
  WoxDarwinWindow *_owner;
}
@end

@implementation WoxWebViewMessageHandler
- (void)userContentController:(WKUserContentController *)userContentController didReceiveScriptMessage:(WKScriptMessage *)message {
  (void)userContentController;
  WoxDarwinWindow *owner = _owner;
  if (owner == NULL || owner->closed || owner->context == 0) {
    return;
  }
  if ([message.name isEqualToString:@"woxWebViewEscapeDiagnostic"]) {
    NSString *detail = [message.body isKindOfClass:[NSString class]] ? message.body : @"page-unknown";
    woxGoDarwinWebViewEscapeDiagnostic(owner->context, detail.UTF8String);
  } else if ([message.name isEqualToString:@"woxWebViewPreview"]) {
    BOOL focused = [owner->window makeFirstResponder:owner->view];
    woxGoDarwinWebViewEscapeDiagnostic(owner->context, focused ? "native-focus-restored" : "native-focus-missing");
    int32_t handled = woxGoDarwinKey(owner->context, "escape", 0, 1, 0, 0);
    woxGoDarwinWebViewEscapeDiagnostic(owner->context, handled != 0 ? "host-dispatch handled=true" : "host-dispatch handled=false");
  } else if ([message.name isEqualToString:@"woxWebViewActionPanel"]) {
    [owner->window makeFirstResponder:owner->view];
    woxGoDarwinKey(owner->context, "j", WOX_KEY_MODIFIER_META, 1, 0, 0);
  } else if ([message.name isEqualToString:@"woxWebViewCursor"] && message.webView == owner->active_web_view && [message.body isKindOfClass:[NSString class]]) {
    NSCursor *cursor = darwin_web_view_cursor(message.body);
    if (cursor != owner->web_view_cursor) {
      [cursor retain];
      [owner->web_view_cursor release];
      owner->web_view_cursor = cursor;
    }
    if (owner->pointer_over_web_view) {
      apply_darwin_pointer_cursor(owner);
    }
  }
}

- (WKWebView *)webView:(WKWebView *)webView
    createWebViewWithConfiguration:(WKWebViewConfiguration *)configuration
               forNavigationAction:(WKNavigationAction *)navigationAction
                    windowFeatures:(WKWindowFeatures *)windowFeatures {
  (void)configuration;
  (void)windowFeatures;
  if (navigationAction.targetFrame == nil && navigationAction.request.URL != nil) {
    [webView loadRequest:navigationAction.request];
  }
  return nil;
}

- (void)webView:(WKWebView *)webView
    decidePolicyForNavigationAction:(WKNavigationAction *)navigationAction
                    decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
  (void)webView;
  NSString *scheme = navigationAction.request.URL.scheme.lowercaseString;
  // Mobile sites use Safari-only schemes to escape embedded browsers; keep preview navigation in Wox.
  if ([scheme isEqualToString:@"x-safari-http"] || [scheme isEqualToString:@"x-safari-https"]) {
    decisionHandler(WKNavigationActionPolicyCancel);
    return;
  }
  decisionHandler(WKNavigationActionPolicyAllow);
}

- (void)webView:(WKWebView *)webView didCommitNavigation:(WKNavigation *)navigation {
  (void)navigation;
  notify_darwin_webview_navigation(_owner, webView);
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
  (void)navigation;
  notify_darwin_webview_navigation(_owner, webView);
}
@end

static NSString *web_view_shortcut_script(void) {
  // Global page routers may always prevent Escape, so only an observable page transition claims it.
  return @"(()=>{if(window.__woxLauncherShortcutsInstalled__)return;window.__woxLauncherShortcutsInstalled__=true;"
          "document.addEventListener('keydown',e=>{if(e.repeat)return;if(e.metaKey&&!e.ctrlKey&&!e.altKey&&!e.shiftKey&&e.key.toLowerCase()==='j'){"
          "e.preventDefault();e.stopImmediatePropagation();window.webkit.messageHandlers.woxWebViewActionPanel.postMessage('action-panel');return}"
          "if(e.key!=='Escape')return;const f=document.activeElement;const d=n=>!n?'none':(n.tagName||'node').toLowerCase()+(n.type?'[type='+n.type+']':'');let m=false;"
          "const o=new MutationObserver(()=>{m=true});if(document.documentElement)o.observe(document.documentElement,{attributes:true,childList:true,characterData:true,subtree:true});setTimeout(()=>{o.disconnect();"
          "const a=document.activeElement;const r=(f&&f!==a)?'page-focus-changed':m?'page-dom-changed':e.defaultPrevented?'page-prevented-no-change-forwarded':'page-forwarded';"
          "window.webkit.messageHandlers.woxWebViewEscapeDiagnostic.postMessage(r+' before='+d(f)+' after='+d(a));if(r==='page-forwarded'||r==='page-prevented-no-change-forwarded')window.webkit.messageHandlers.woxWebViewPreview.postMessage('escape')},0)},true)})()";
}

static WKWebView *create_web_view(WoxDarwinWindow *window, NSString *inject_css, NSString *user_agent) {
  WKWebViewConfiguration *configuration = [[[WKWebViewConfiguration alloc] init] autorelease];
  configuration.websiteDataStore = [WKWebsiteDataStore defaultDataStore];
  WoxWebViewMessageHandler *message_handler = [[WoxWebViewMessageHandler alloc] init];
  message_handler->_owner = window;
  [configuration.userContentController addScriptMessageHandler:message_handler name:@"woxWebViewEscapeDiagnostic"];
  [configuration.userContentController addScriptMessageHandler:message_handler name:@"woxWebViewPreview"];
  [configuration.userContentController addScriptMessageHandler:message_handler name:@"woxWebViewActionPanel"];
  [configuration.userContentController addScriptMessageHandler:message_handler name:@"woxWebViewCursor"];
  WKUserScript *shortcut_script = [[[WKUserScript alloc] initWithSource:web_view_shortcut_script() injectionTime:WKUserScriptInjectionTimeAtDocumentStart forMainFrameOnly:YES] autorelease];
  [configuration.userContentController addUserScript:shortcut_script];
  WKUserScript *cursor_script = [[[WKUserScript alloc] initWithSource:web_view_cursor_script() injectionTime:WKUserScriptInjectionTimeAtDocumentStart forMainFrameOnly:NO] autorelease];
  [configuration.userContentController addUserScript:cursor_script];
  NSString *script = web_view_css_script(inject_css);
  if (script != nil) {
    // The CSS script waits for the root node, so document-start injection avoids a visible unstyled frame.
    WKUserScript *user_script = [[[WKUserScript alloc] initWithSource:script injectionTime:WKUserScriptInjectionTimeAtDocumentStart forMainFrameOnly:YES] autorelease];
    [configuration.userContentController addUserScript:user_script];
  }
  WKWebView *web_view = [[WKWebView alloc] initWithFrame:NSZeroRect configuration:configuration];
  web_view.navigationDelegate = message_handler;
  web_view.UIDelegate = message_handler;
  [message_handler release];
  web_view.autoresizingMask = NSViewNotSizable;
  web_view.wantsLayer = YES;
  web_view.layer.masksToBounds = YES;
  if (user_agent.length > 0) {
    web_view.customUserAgent = user_agent;
  }
  if (@available(macOS 13.3, *)) {
    web_view.inspectable = YES;
  }
  return web_view;
}

static void clear_active_web_view(WoxDarwinWindow *window, bool discard_transient) {
  if (window->web_view_toolbar != nil) {
    [window->web_view_toolbar setWebView:nil];
    [window->web_view_toolbar removeFromSuperview];
  }
  if (window->active_web_view != nil) {
    [window->active_web_view removeFromSuperview];
    if (window->active_web_view_transient && discard_transient) {
      [window->active_web_view stopLoading];
      [window->active_web_view release];
    }
  }
  window->active_web_view = nil;
  window->active_web_view_transient = false;
  [window->active_web_view_key release];
  [window->active_web_view_signature release];
  [window->active_web_view_content_key release];
  window->active_web_view_key = nil;
  window->active_web_view_signature = nil;
  window->active_web_view_content_key = nil;
  window->pointer_over_web_view = false;
  [window->web_view_cursor release];
  window->web_view_cursor = nil;
  apply_darwin_pointer_cursor(window);
}

// desktop_top returns the AppKit Y coordinate used to map Wox's top-left virtual desktop space.
static CGFloat desktop_top(void) {
  CGFloat top = 0.0;
  for (NSScreen *screen in [NSScreen screens]) {
    top = MAX(top, NSMaxY(screen.frame));
  }
  return top;
}

static uint8_t portable_modifiers(NSEventModifierFlags flags) {
  uint8_t modifiers = 0;
  if ((flags & NSEventModifierFlagShift) != 0) {
    modifiers |= WOX_KEY_MODIFIER_SHIFT;
  }
  if ((flags & NSEventModifierFlagControl) != 0) {
    modifiers |= WOX_KEY_MODIFIER_CONTROL;
  }
  if ((flags & NSEventModifierFlagOption) != 0) {
    modifiers |= WOX_KEY_MODIFIER_ALT;
  }
  if ((flags & NSEventModifierFlagCommand) != 0) {
    modifiers |= WOX_KEY_MODIFIER_META;
  }
  return modifiers;
}

// portable_key keeps AppKit function-key values out of the shared Go input contract.
static const char *portable_key(NSEvent *event) {
  NSString *characters = [[event charactersIgnoringModifiers] lowercaseString];
  if (characters.length == 0) {
    return "";
  }
  switch ([characters characterAtIndex:0]) {
  case NSBackspaceCharacter:
  case NSDeleteCharacter:
    return "backspace";
  case NSTabCharacter:
  case NSBackTabCharacter:
    return "tab";
  case NSCarriageReturnCharacter:
  case NSEnterCharacter:
    return "enter";
  case 0x1B:
    return "escape";
  case 0x20:
    return "space";
  case NSPageUpFunctionKey:
    return "page-up";
  case NSPageDownFunctionKey:
    return "page-down";
  case NSEndFunctionKey:
    return "end";
  case NSHomeFunctionKey:
    return "home";
  case NSLeftArrowFunctionKey:
    return "arrow-left";
  case NSUpArrowFunctionKey:
    return "arrow-up";
  case NSRightArrowFunctionKey:
    return "arrow-right";
  case NSDownArrowFunctionKey:
    return "arrow-down";
  case NSDeleteFunctionKey:
    return "delete";
  default:
    return characters.UTF8String;
  }
}

static NSString *plain_text(id value) {
  if ([value isKindOfClass:[NSAttributedString class]]) {
    return [(NSAttributedString *)value string];
  }
  if ([value isKindOfClass:[NSString class]]) {
    return (NSString *)value;
  }
  return [value description];
}

static BOOL accessibility_action(WoxAccessibilityElement *element, const char *action, NSString *value) {
  if (element == nil || element->_owner == NULL || element->_owner->closed || element->_owner->context == 0) {
    return NO;
  }
  const char *utf8 = value != nil ? value.UTF8String : "";
  return woxGoDarwinAccessibilityAction(element->_owner->context, element->_node_id, action, utf8) != 0;
}

@implementation WoxAccessibilityElement
- (BOOL)accessibilityPerformPress {
  if ((_action_flags & WOX_ACCESSIBILITY_ACTION_ACTIVATE) != 0) {
    return accessibility_action(self, "activate", nil);
  }
  if ((_action_flags & WOX_ACCESSIBILITY_ACTION_TOGGLE) != 0) {
    return accessibility_action(self, "toggle", nil);
  }
  return NO;
}

- (BOOL)accessibilityPerformIncrement {
  return (_action_flags & WOX_ACCESSIBILITY_ACTION_INCREMENT) != 0 && accessibility_action(self, "increment", nil);
}

- (BOOL)accessibilityPerformDecrement {
  return (_action_flags & WOX_ACCESSIBILITY_ACTION_DECREMENT) != 0 && accessibility_action(self, "decrement", nil);
}

- (void)setAccessibilityFocused:(BOOL)focused {
  if (!_configuring && focused && (_action_flags & WOX_ACCESSIBILITY_ACTION_FOCUS) != 0) {
    accessibility_action(self, "focus", nil);
  }
  [super setAccessibilityFocused:focused];
}

- (void)setAccessibilityValue:(id)value {
  if (!_configuring && (_action_flags & WOX_ACCESSIBILITY_ACTION_SET_VALUE) != 0) {
    accessibility_action(self, "set_value", plain_text(value));
    return;
  }
  [super setAccessibilityValue:value];
}
@end

static uint8_t portable_pointer_button(NSEvent *event) {
  switch (event.buttonNumber) {
  case 0:
    return 1;
  case 1:
    return 2;
  case 2:
    return 3;
  default:
    return 0;
  }
}

@implementation WoxRenderView
- (NSView *)hitTest:(NSPoint)point {
  if (_owner != NULL && !_owner->closed && _owner->active_web_view != nil && !_owner->active_web_view.hidden && NSPointInRect(point, _owner->active_web_view.frame)) {
    return self;
  }
  return [super hitTest:point];
}

- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
  NSArray *urls = [sender.draggingPasteboard readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
  return urls.count > 0 ? NSDragOperationCopy : NSDragOperationNone;
}

- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
  NSArray *urls = [sender.draggingPasteboard readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
  NSMutableString *paths = [NSMutableString string];
  for (NSURL *url in urls) {
    if (!url.isFileURL || url.path.length == 0) {
      continue;
    }
    if (paths.length > 0) {
      [paths appendString:@"\n"];
    }
    [paths appendString:url.path];
  }
  if (paths.length == 0 || _owner == NULL || _owner->context == 0) {
    return NO;
  }
  woxGoDarwinFileDrop(_owner->context, paths.UTF8String);
  return YES;
}

- (void)resetCursorRects {
  [super resetCursorRects];
  NSCursor *cursor = _owner != NULL && _owner->pointer_over_web_view && _owner->web_view_cursor != nil
                         ? _owner->web_view_cursor
                         : (_owner != NULL ? darwin_host_pointer_cursor(_owner->pointer_cursor) : [NSCursor arrowCursor]);
  [self addCursorRect:self.bounds cursor:cursor];
}

- (CALayer *)makeBackingLayer {
  CALayer *layer = [CALayer layer];
  layer.opaque = NO;
  layer.needsDisplayOnBoundsChange = NO;
  return layer;
}

- (BOOL)wantsUpdateLayer {
  return YES;
}

- (BOOL)isFlipped {
  return YES;
}

- (BOOL)acceptsFirstResponder {
  return YES;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
  (void)event;
  return YES;
}

- (void)dealloc {
  [_marked_text release];
  [_tracking_area release];
  [_accessibility_children release];
  [super dealloc];
}

- (BOOL)isAccessibilityElement {
  return NO;
}

- (NSArray *)accessibilityChildren {
  return _accessibility_children ?: @[];
}

- (void)setWoxAccessibilityChildren:(NSArray *)children {
  if (_accessibility_children == children) {
    return;
  }
  [_accessibility_children release];
  _accessibility_children = [children copy];
}

- (void)updateTrackingAreas {
  [super updateTrackingAreas];
  if (_tracking_area != nil) {
    [self removeTrackingArea:_tracking_area];
    [_tracking_area release];
  }
  _tracking_area = [[NSTrackingArea alloc]
      initWithRect:NSZeroRect
           options:NSTrackingMouseEnteredAndExited | NSTrackingMouseMoved | NSTrackingActiveAlways | NSTrackingInVisibleRect
             owner:self
          userInfo:nil];
  [self addTrackingArea:_tracking_area];
}

- (void)emitPointer:(NSEvent *)event kind:(uint8_t)kind button:(uint8_t)button scrollX:(float)scroll_x scrollY:(float)scroll_y {
  WoxDarwinWindow *owner = _owner;
  if (owner == NULL || owner->closed || owner->context == 0 || owner->forwarding_embedded_pointer) {
    return;
  }
  NSPoint position = [self convertPoint:event.locationInWindow fromView:nil];
  woxGoDarwinPointer(owner->context, kind, (float)position.x, (float)position.y, button, scroll_x, scroll_y, portable_modifiers(event.modifierFlags));
}

- (void)mouseEntered:(NSEvent *)event {
  [self emitPointer:event kind:WOX_POINTER_ENTER button:0 scrollX:0.0f scrollY:0.0f];
}

- (void)mouseExited:(NSEvent *)event {
  [self emitPointer:event kind:WOX_POINTER_LEAVE button:0 scrollX:0.0f scrollY:0.0f];
}

- (void)mouseMoved:(NSEvent *)event {
  [self emitPointer:event kind:WOX_POINTER_MOVE button:0 scrollX:0.0f scrollY:0.0f];
}

- (void)mouseDragged:(NSEvent *)event {
  [self mouseMoved:event];
}

- (void)rightMouseDragged:(NSEvent *)event {
  [self mouseMoved:event];
}

- (void)otherMouseDragged:(NSEvent *)event {
  [self mouseMoved:event];
}

- (void)mouseDown:(NSEvent *)event {
  [self.window makeFirstResponder:self];
  [self emitPointer:event kind:WOX_POINTER_DOWN button:portable_pointer_button(event) scrollX:0.0f scrollY:0.0f];
}

- (void)mouseUp:(NSEvent *)event {
  [self emitPointer:event kind:WOX_POINTER_UP button:portable_pointer_button(event) scrollX:0.0f scrollY:0.0f];
}

- (void)rightMouseDown:(NSEvent *)event {
  [self mouseDown:event];
}

- (void)rightMouseUp:(NSEvent *)event {
  [self mouseUp:event];
}

- (void)otherMouseDown:(NSEvent *)event {
  [self mouseDown:event];
}

- (void)otherMouseUp:(NSEvent *)event {
  [self mouseUp:event];
}

- (void)scrollWheel:(NSEvent *)event {
  CGFloat unit = event.hasPreciseScrollingDeltas ? 1.0 : 40.0;
  [self emitPointer:event kind:WOX_POINTER_SCROLL button:0 scrollX:(float)(event.scrollingDeltaX * unit) scrollY:(float)(event.scrollingDeltaY * unit)];
}

- (void)keyDown:(NSEvent *)event {
  WoxDarwinWindow *owner = _owner;
  if (owner == NULL || owner->closed || owner->context == 0) {
    [super keyDown:event];
    return;
  }
  int32_t handled = woxGoDarwinKey(owner->context, portable_key(event), portable_modifiers(event.modifierFlags), 1, event.isARepeat ? 1 : 0, _marked_text.length > 0 ? 1 : 0);
  if (handled != 0) {
    return;
  }
  if (owner->input_enabled) {
    [self interpretKeyEvents:@[ event ]];
  } else {
    [super keyDown:event];
  }
}

- (void)keyUp:(NSEvent *)event {
  WoxDarwinWindow *owner = _owner;
  if (owner != NULL && !owner->closed && owner->context != 0) {
    int32_t handled = woxGoDarwinKey(owner->context, portable_key(event), portable_modifiers(event.modifierFlags), 0, 0, _marked_text.length > 0 ? 1 : 0);
    if (handled != 0) {
      return;
    }
  }
  [super keyUp:event];
}

// NSTextInputClient keeps marked text separate from committed UTF-8 text.
- (void)insertText:(id)value replacementRange:(NSRange)replacement_range {
  (void)replacement_range;
  WoxDarwinWindow *owner = _owner;
  NSString *text = plain_text(value);
  [_marked_text release];
  _marked_text = nil;
  _marked_selection = NSMakeRange(NSNotFound, 0);
  if (owner != NULL && !owner->closed && owner->input_enabled && owner->context != 0 && text.length > 0) {
    woxGoDarwinTextInput(owner->context, WOX_TEXT_INPUT_COMMIT, text.UTF8String);
  }
}

- (void)setMarkedText:(id)value selectedRange:(NSRange)selected_range replacementRange:(NSRange)replacement_range {
  (void)replacement_range;
  NSString *text = plain_text(value);
  [_marked_text release];
  _marked_text = [text copy];
  _marked_selection = selected_range;
  WoxDarwinWindow *owner = _owner;
  if (owner != NULL && !owner->closed && owner->input_enabled && owner->context != 0) {
    woxGoDarwinTextInput(owner->context, WOX_TEXT_INPUT_COMPOSE, text.UTF8String);
  }
}

- (void)unmarkText {
  bool had_marked_text = _marked_text.length > 0;
  [_marked_text release];
  _marked_text = nil;
  _marked_selection = NSMakeRange(NSNotFound, 0);
  WoxDarwinWindow *owner = _owner;
  if (had_marked_text && owner != NULL && !owner->closed && owner->input_enabled && owner->context != 0) {
    woxGoDarwinTextInput(owner->context, WOX_TEXT_INPUT_COMPOSE, "");
  }
}

- (BOOL)hasMarkedText {
  return _marked_text.length > 0;
}

- (NSRange)markedRange {
  return _marked_text.length > 0 ? NSMakeRange(0, _marked_text.length) : NSMakeRange(NSNotFound, 0);
}

- (NSRange)selectedRange {
  return _marked_selection;
}

- (NSArray<NSAttributedStringKey> *)validAttributesForMarkedText {
  return @[];
}

- (NSAttributedString *)attributedSubstringForProposedRange:(NSRange)range actualRange:(NSRangePointer)actual_range {
  (void)range;
  if (actual_range != NULL) {
    *actual_range = NSMakeRange(NSNotFound, 0);
  }
  return nil;
}

- (NSRect)firstRectForCharacterRange:(NSRange)range actualRange:(NSRangePointer)actual_range {
  (void)range;
  if (actual_range != NULL) {
    *actual_range = NSMakeRange(NSNotFound, 0);
  }
  WoxDarwinWindow *owner = _owner;
  if (owner == NULL || owner->closed || self.window == nil) {
    return NSZeroRect;
  }
  NSRect window_rect = [self convertRect:owner->input_cursor_rect toView:nil];
  return [self.window convertRectToScreen:window_rect];
}

- (NSUInteger)characterIndexForPoint:(NSPoint)point {
  (void)point;
  return 0;
}

- (void)doCommandBySelector:(SEL)selector {
  (void)selector;
}

- (void)updateBackingScale {
  if (_owner == NULL || _owner->closed || self.window == nil) {
    return;
  }
  self.layer.contentsScale = self.window.backingScaleFactor;
}

- (void)viewDidMoveToWindow {
  [super viewDidMoveToWindow];
  [self updateBackingScale];
}

- (void)viewDidChangeBackingProperties {
  [super viewDidChangeBackingProperties];
  [self updateBackingScale];
  if (_owner != NULL && !_owner->closed) {
    atomic_fetch_add_explicit(&_owner->presentation_generation, 1, memory_order_relaxed);
  }
  schedule_render(_owner);
}

- (void)setFrameSize:(NSSize)newSize {
  NSSize old_size = self.frame.size;
  [super setFrameSize:newSize];
  [self updateBackingScale];
  if (_owner != NULL && !_owner->suppress_resize_render) {
    if (!NSEqualSizes(old_size, newSize)) {
      atomic_fetch_add_explicit(&_owner->presentation_generation, 1, memory_order_relaxed);
    }
    schedule_render(_owner);
  }
}

- (void)updateLayer {
  schedule_render(_owner);
}

- (void)renderFrameWithSynchronousMode:(BOOL)synchronous transactional:(BOOL)transactional {
  WoxDarwinWindow *owner = _owner;
  if (owner == NULL || owner->closed || !owner->visible || owner->context == 0) {
    return;
  }
  owner->render_scheduled = false;
  [self updateBackingScale];
  NSSize size = self.bounds.size;
  CGFloat scale = self.window.backingScaleFactor;
  int32_t pixel_width = (int32_t)ceil(size.width * scale);
  int32_t pixel_height = (int32_t)ceil(size.height * scale);
  if (size.width > 0.0 && size.height > 0.0 && pixel_width > 0 && pixel_height > 0) {
    if (synchronous) {
      woxGoDarwinFrameSync(owner->context, (float)size.width, (float)size.height, pixel_width, pixel_height, (float)scale, transactional ? 1 : 0);
    } else {
      woxGoDarwinFrame(owner->context, (float)size.width, (float)size.height, pixel_width, pixel_height, (float)scale);
    }
  }
}

- (void)renderFrame {
  [self renderFrameWithSynchronousMode:NO transactional:NO];
}

- (void)renderFrameSynchronously:(BOOL)transactional {
  [self renderFrameWithSynchronousMode:YES transactional:transactional];
}
@end

@implementation WoxWindowDelegate
- (BOOL)windowShouldClose:(NSWindow *)sender {
  (void)sender;
  WoxDarwinWindow *owner = _owner;
  if (owner == NULL || owner->closed || owner->context == 0) {
    return YES;
  }
  // Native traffic-light closes still flow through Go so the named-window lifecycle and host cleanup run first.
  woxGoDarwinCloseRequested(owner->context);
  return NO;
}

- (void)windowDidBecomeKey:(NSNotification *)notification {
  (void)notification;
  if (_owner != NULL && !_owner->closed && _owner->visible) {
    emit_focus(_owner, true);
  }
}

- (void)windowDidChangeScreen:(NSNotification *)notification {
  (void)notification;
  if (_owner != NULL && !_owner->closed) {
    bind_darwin_display_link(_owner);
  }
}

- (void)windowDidResignKey:(NSNotification *)notification {
  (void)notification;
  WoxDarwinWindow *owner = _owner;
  if (owner == NULL || owner->closed) {
    return;
  }
  if (owner->native_dialog_active) {
    return;
  }
  emit_focus(owner, false);
  if (!owner->closed && owner->hide_on_blur && owner->visible) {
    hide_window_and_release_surfaces(owner);
  }
}
@end

int32_t wox_darwin_run(uintptr_t context) {
  if (![NSThread isMainThread]) {
    return -2;
  }
  @autoreleasepool {
    NSApplication *application = [NSApplication sharedApplication];
    [application setActivationPolicy:NSApplicationActivationPolicyAccessory];
    WoxApplicationDelegate *application_delegate = [[WoxApplicationDelegate alloc] initWithContext:context];
    [application setDelegate:application_delegate];
    [application finishLaunching];
    if (woxGoDarwinStart(context) != 0) {
      [application setDelegate:nil];
      [application_delegate release];
      return -1;
    }
    if (wox_open_window_count == 0) {
      [application setDelegate:nil];
      [application_delegate release];
      return 0;
    }
    [application run];
    [application setDelegate:nil];
    [application_delegate release];
  }
  return 0;
}

WoxDarwinWindow *wox_darwin_window_create(const char *title, float width, float height, int32_t hide_on_blur, int32_t window_role, int32_t nonactivating, int32_t resizable, float aspect_ratio, uintptr_t context) {
  if (![NSThread isMainThread] || width <= 0.0f || height <= 0.0f || context == 0) {
    return NULL;
  }

  @autoreleasepool {
    NSRect frame = NSMakeRect(0.0, 0.0, width, height);
    bool is_application_window = window_role == 1;
    bool is_screenshot_window = window_role == 2;
    bool is_nonactivating = nonactivating != 0;
    bool is_overlay_style = is_screenshot_window || is_nonactivating;
    // Transparent borderless windows make AppKit derive shadows from the
    // asynchronously presented IOSurface, which produces rectangular flashes
    // and streaks. A hidden full-size title bar keeps nonactivating overlays
    // visually frameless while letting AppKit own the stable shadow outline.
    NSWindowStyleMask style_mask = NSWindowStyleMaskTitled | NSWindowStyleMaskFullSizeContentView;
    if (is_screenshot_window) {
      style_mask = NSWindowStyleMaskBorderless;
    }
    if (is_application_window) {
      style_mask |= NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable;
    }
    if (resizable != 0) {
      style_mask |= NSWindowStyleMaskResizable;
    }
    WoxNativeWindow *native_window = [[WoxNativeWindow alloc]
        initWithContentRect:frame
                  styleMask:style_mask
                    backing:NSBackingStoreBuffered
                      defer:NO];
    native_window.releasedWhenClosed = NO;
    native_window.woxNonactivating = is_nonactivating;
    native_window.opaque = NO;
    native_window.backgroundColor = [NSColor clearColor];
    // Every window floats with a native shadow except the screenshot surface,
    // which covers the desktop and must not cast an outline over it.
    native_window.hasShadow = !is_screenshot_window;
    if (aspect_ratio > 0.0f) {
      native_window.contentAspectRatio = NSMakeSize(aspect_ratio, 1.0);
    }
    native_window.acceptsMouseMovedEvents = YES;
    if (!is_screenshot_window) {
      native_window.titlebarAppearsTransparent = YES;
      native_window.titleVisibility = NSWindowTitleHidden;
      [[native_window standardWindowButton:NSWindowCloseButton] setHidden:YES];
      [[native_window standardWindowButton:NSWindowMiniaturizeButton] setHidden:YES];
      [[native_window standardWindowButton:NSWindowZoomButton] setHidden:YES];
    }
    // Management windows keep their titled style while sharing the launcher's cross-space activation behavior.
    if (is_application_window) {
      native_window.level = NSNormalWindowLevel;
      native_window.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
    } else if (is_overlay_style) {
      NSWindowCollectionBehavior behavior =
          NSWindowCollectionBehaviorCanJoinAllSpaces |
          NSWindowCollectionBehaviorFullScreenAuxiliary |
          NSWindowCollectionBehaviorStationary |
          NSWindowCollectionBehaviorIgnoresCycle;
      if (@available(macOS 13.0, *)) {
        behavior |= NSWindowCollectionBehaviorCanJoinAllApplications;
      }
      native_window.level = MAX(NSScreenSaverWindowLevel, CGShieldingWindowLevel());
      native_window.collectionBehavior = behavior;
      native_window.animationBehavior = NSWindowAnimationBehaviorNone;
    } else {
      native_window.level = NSFloatingWindowLevel;
      native_window.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
    }
    if (title != NULL) {
      NSString *window_title = [NSString stringWithUTF8String:title];
      if (window_title != nil) {
        native_window.title = window_title;
      }
    }

    WoxDarwinWindow *window = calloc(1, sizeof(WoxDarwinWindow));
    window->nonactivating = is_nonactivating;
    atomic_init(&window->presentation_generation, 0);
    WoxRenderView *view = [[WoxRenderView alloc] initWithFrame:frame];
    view->_owner = window;
    view->_marked_selection = NSMakeRange(NSNotFound, 0);
    view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    view.wantsLayer = YES;
    CALayer *layer = view.layer;

    WoxDarwinRenderer *renderer = create_renderer(layer);
    if (renderer == NULL) {
      view->_owner = NULL;
      [view release];
      [native_window release];
      free(window);
      return NULL;
    }
    WoxDarwinRenderer *overlay_renderer = create_renderer(layer);
    if (overlay_renderer == NULL) {
      destroy_renderer(renderer);
      view->_owner = NULL;
      [view release];
      [native_window release];
      free(window);
      return NULL;
    }
    renderer->content_layer.zPosition = 0.0;
    overlay_renderer->content_layer.zPosition = 2.0;

    WoxWindowDelegate *delegate = [[WoxWindowDelegate alloc] init];
    delegate->_owner = window;
    window->window = native_window;
    window->view = view;
    window->delegate = delegate;
    window->renderer = renderer;
    window->overlay_renderer = overlay_renderer;
    window->web_view_cache = [[NSMutableDictionary alloc] init];
    window->web_view_signatures = [[NSMutableDictionary alloc] init];
    window->web_view_content_keys = [[NSMutableDictionary alloc] init];
    window->context = context;
    window->hide_on_blur = hide_on_blur != 0;
    window->screenshot_window = is_screenshot_window;
    if (is_screenshot_window) {
      // Recording border and countdown windows intentionally clear their
      // IOSurfaces to reveal the live desktop. A visual-effect container would
      // turn those transparent pixels into a fullscreen blur.
      native_window.contentView = view;
    } else {
      NSVisualEffectView *effect_view = [[NSVisualEffectView alloc] initWithFrame:frame];
      effect_view.material = NSVisualEffectMaterialPopover;
      effect_view.state = NSVisualEffectStateActive;
      effect_view.blendingMode = NSVisualEffectBlendingModeBehindWindow;
      effect_view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
      effect_view.wantsLayer = YES;
      effect_view.layer.cornerRadius = wox_window_corner_radius;
      effect_view.layer.masksToBounds = YES;
      [effect_view addSubview:view];
      native_window.contentView = effect_view;
      [effect_view release];
    }
    native_window.delegate = delegate;
    [view registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
    [native_window center];
    [view updateBackingScale];
    wox_open_window_count++;
    return window;
  }
}

uint64_t wox_darwin_window_show(WoxDarwinWindow *window) {
  if (window == NULL) {
    return 0;
  }
  __block uint64_t epoch = 0;
  run_on_main_sync(^{
    if (window->closed) {
      return;
    }
    if (window->active) {
      // Starting a new focus epoch is not a real focus loss.
      window->active = false;
    }
    if (!window->nonactivating) {
      save_previous_active_app_if_needed(window);
    }
    window->epoch++;
    atomic_fetch_add_explicit(&window->presentation_generation, 1, memory_order_relaxed);
    epoch = window->epoch;
    window->visible = true;
    [window->view updateBackingScale];
    if (window->window.isMiniaturized) {
      [window->window deminiaturize:nil];
    }
    if (window->nonactivating) {
      [window->window orderFrontRegardless];
    } else {
      [NSApp activateIgnoringOtherApps:YES];
      [window->window makeKeyAndOrderFront:nil];
      [window->window makeFirstResponder:window->view];
    }
    if (!window->closed && window->window.isKeyWindow) {
      emit_focus(window, true);
    }
    if (!window->closed && window->screenshot_window) {
      // Screenshot handoff keeps native selection overlays visible until this first complete frame
      // is ready, avoiding a transparent gap between the native selector and portable editor.
      [window->view renderFrameSynchronously:YES];
    } else if (!window->closed) {
      // Rendering is explicit; AppKit does not reliably deliver updateLayer for the first frame.
      [window->view renderFrame];
    }
  });
  return epoch;
}

int32_t wox_darwin_window_hide(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    BOOL was_wox_frontmost = [NSApp isActive] || [[NSWorkspace sharedWorkspace] frontmostApplication] == [NSRunningApplication currentApplication];
    emit_focus(window, false);
    if (!window->closed) {
      stop_darwin_display_link(window);
      hide_window_and_release_surfaces(window);
      restore_previous_active_app(window, was_wox_frontmost);
    }
  });
  return result;
}

int32_t wox_darwin_window_set_bounds(WoxDarwinWindow *window, float x, float y, float width, float height) {
  if (window == NULL || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    NSRect frame = NSMakeRect(x, desktop_top() - y - height, width, height);
    [CATransaction begin];
    [CATransaction setDisableActions:YES];
    window->suppress_resize_render = true;
    [window->window setFrame:frame display:NO];
    window->suppress_resize_render = false;
    render_resize_frame(window);
    [CATransaction commit];
  });
  return result;
}

int32_t wox_darwin_window_get_bounds(WoxDarwinWindow *window, float *x, float *y, float *width, float *height) {
  if (window == NULL || x == NULL || y == NULL || width == NULL || height == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    NSRect frame = window->window.frame;
    *x = (float)NSMinX(frame);
    *y = (float)(desktop_top() - NSMaxY(frame));
    *width = (float)NSWidth(frame);
    *height = (float)NSHeight(frame);
  });
  return result;
}

int32_t wox_darwin_window_capture_png(WoxDarwinWindow *window, const char *path) {
  if (window == NULL || path == NULL || path[0] == '\0') {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || !window->visible) {
      result = -1;
      return;
    }
    // Captures need the freshly rendered surface, while normal UI frames remain asynchronous.
    window->synchronous_frame = true;
    [window->view renderFrameSynchronously:NO];
    window->synchronous_frame = false;
    [CATransaction flush];
    typedef CGImageRef (*WoxWindowCaptureFunction)(CGRect, CGWindowListOption, CGWindowID, CGWindowImageOption);
    WoxWindowCaptureFunction capture = (WoxWindowCaptureFunction)dlsym(RTLD_DEFAULT, "CGWindowListCreateImage");
    if (capture == NULL) {
      result = -1;
      return;
    }
    CGWindowID window_id = (CGWindowID)window->window.windowNumber;
    CGImageRef image = capture(CGRectNull, kCGWindowListOptionIncludingWindow, window_id, kCGWindowImageBoundsIgnoreFraming | kCGWindowImageBestResolution);
    if (image == NULL) {
      result = -1;
      return;
    }
    NSBitmapImageRep *representation = [[NSBitmapImageRep alloc] initWithCGImage:image];
    CGImageRelease(image);
    NSData *png = [representation representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
    NSString *file_path = [NSString stringWithUTF8String:path];
    if (png == nil || file_path == nil || ![png writeToFile:file_path atomically:YES]) {
      result = -1;
    }
    [representation release];
  });
  return result;
}

static int32_t write_cgimage_png(CGImageRef image, const char *path) {
  if (image == NULL || path == NULL || path[0] == '\0') {
    return -1;
  }
  @autoreleasepool {
    NSBitmapImageRep *representation = [[NSBitmapImageRep alloc] initWithCGImage:image];
    NSData *png = [representation representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
    NSString *file_path = [NSString stringWithUTF8String:path];
    int32_t result = png != nil && file_path != nil && [png writeToFile:file_path atomically:YES] ? 0 : -1;
    [representation release];
    return result;
  }
}

int32_t wox_darwin_select_screenshot_region(
    const char *path,
    uintptr_t *session_handle,
    uint32_t *display_id,
    float *display_x,
    float *display_y,
    float *display_width,
    float *display_height,
    float *selection_x,
    float *selection_y,
    float *selection_width,
    float *selection_height) {
  if (path == NULL || path[0] == '\0' || session_handle == NULL || display_id == NULL || display_x == NULL || display_y == NULL || display_width == NULL || display_height == NULL ||
      selection_x == NULL || selection_y == NULL || selection_width == NULL || selection_height == NULL || [NSThread isMainThread]) {
    return -1;
  }
  if (@available(macOS 12.0, *)) {
    if (!CGPreflightScreenCaptureAccess()) {
      return -2;
    }
  }

  __block WoxScreenshotSelectionSession *session = nil;
  __block int32_t setup_result = 0;
  run_on_main_sync(^{
    NSArray<NSScreen *> *screens = [NSScreen screens];
    NSMutableArray *captures = [NSMutableArray arrayWithCapacity:screens.count];
    CGFloat top = desktop_top();
    for (NSScreen *screen in screens) {
      WoxScreenshotDisplayCapture *capture = [[WoxScreenshotDisplayCapture alloc] initWithScreen:screen desktopTop:top];
      if (capture == nil) {
        setup_result = -1;
        break;
      }
      [captures addObject:capture];
      [capture release];
    }
    if (setup_result != 0 || captures.count == 0) {
      return;
    }
    session = [[WoxScreenshotSelectionSession alloc] initWithCaptures:captures];
    if (session == nil) {
      setup_result = -1;
      return;
    }
    [session begin];
  });
  if (setup_result != 0 || session == nil) {
    [session release];
    return -1;
  }

  dispatch_semaphore_wait([session completion], DISPATCH_TIME_FOREVER);
  if ([session cancelled]) {
    run_on_main_sync(^{
      [session dismiss];
    });
    [session release];
    return 1;
  }

  WoxScreenshotDisplayCapture *capture = [session selectedCapture];
  NSRect selection = [session selection];
  int32_t result = write_cgimage_png(capture->image, path);
  if (result == 0) {
    *session_handle = (uintptr_t)session;
    *display_id = capture->display_id;
    *display_x = (float)NSMinX(capture->logical_bounds);
    *display_y = (float)NSMinY(capture->logical_bounds);
    *display_width = (float)NSWidth(capture->logical_bounds);
    *display_height = (float)NSHeight(capture->logical_bounds);
    *selection_x = (float)NSMinX(selection);
    *selection_y = (float)NSMinY(selection);
    *selection_width = (float)NSWidth(selection);
    *selection_height = (float)NSHeight(selection);
  }
  if (result != 0) {
    run_on_main_sync(^{
      [session dismiss];
    });
    [session release];
  }
  return result;
}

void wox_darwin_dismiss_screenshot_selection(uintptr_t session_handle) {
  if (session_handle == 0) {
    return;
  }
  WoxScreenshotSelectionSession *session = (WoxScreenshotSelectionSession *)session_handle;
  run_on_main_sync(^{
    [session dismiss];
  });
  [session release];
}

uintptr_t wox_darwin_show_screenshot_border(float x, float y, float width, float height, float thickness) {
  if (width <= 0.0f || height <= 0.0f || thickness <= 0.0f) {
    return 0;
  }
  __block NSMutableArray *windows = nil;
  run_on_main_sync(^{
    NSRect edges[] = {
        NSMakeRect(x - thickness, y - thickness, width + thickness * 2.0f, thickness),
        NSMakeRect(x + width, y, thickness, height),
        NSMakeRect(x - thickness, y + height, width + thickness * 2.0f, thickness),
        NSMakeRect(x - thickness, y, thickness, height),
    };
    windows = [[NSMutableArray alloc] initWithCapacity:4];
    NSColor *green = [NSColor colorWithCalibratedRed:41.0 / 255.0 green:1.0 blue:114.0 / 255.0 alpha:1.0];
    for (NSUInteger index = 0; index < 4; index++) {
      NSRect edge = edges[index];
      NSRect frame = NSMakeRect(NSMinX(edge), desktop_top() - NSMaxY(edge), NSWidth(edge), NSHeight(edge));
      WoxNativeWindow *window = [[WoxNativeWindow alloc]
          initWithContentRect:frame
                    styleMask:NSWindowStyleMaskBorderless
                      backing:NSBackingStoreBuffered
                        defer:NO];
      window.releasedWhenClosed = NO;
      window.woxNonactivating = YES;
      window.opaque = YES;
      window.backgroundColor = green;
      window.hasShadow = NO;
      window.ignoresMouseEvents = YES;
      window.animationBehavior = NSWindowAnimationBehaviorNone;
      window.level = MAX(NSScreenSaverWindowLevel, CGShieldingWindowLevel());
      NSWindowCollectionBehavior behavior =
          NSWindowCollectionBehaviorCanJoinAllSpaces |
          NSWindowCollectionBehaviorFullScreenAuxiliary |
          NSWindowCollectionBehaviorStationary |
          NSWindowCollectionBehaviorIgnoresCycle;
      if (@available(macOS 13.0, *)) {
        behavior |= NSWindowCollectionBehaviorCanJoinAllApplications;
      }
      window.collectionBehavior = behavior;
      [window orderFrontRegardless];
      [windows addObject:window];
      [window release];
    }
  });
  return (uintptr_t)windows;
}

void wox_darwin_dismiss_screenshot_border(uintptr_t border_handle) {
  if (border_handle == 0) {
    return;
  }
  NSMutableArray *windows = (NSMutableArray *)border_handle;
  run_on_main_sync(^{
    for (WoxNativeWindow *window in windows) {
      [window orderOut:nil];
      [window close];
    }
  });
  [windows release];
}

int32_t wox_darwin_capture_display_png(uint32_t display_id, const char *path) {
  if (display_id == 0 || path == NULL || path[0] == '\0') {
    return -1;
  }
  CGImageRef image = capture_display_image((CGDirectDisplayID)display_id);
  if (image == NULL) {
    return -1;
  }
  int32_t result = write_cgimage_png(image, path);
  CGImageRelease(image);
  return result;
}

int32_t wox_darwin_window_center(WoxDarwinWindow *window, float width, float height) {
  if (window == NULL || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    NSScreen *screen = window->window.screen ?: [NSScreen mainScreen];
    if (screen == nil) {
      result = -1;
      return;
    }
    NSRect work_area = screen.visibleFrame;
    float clamped_width = fmin(width, NSWidth(work_area));
    float clamped_height = fmin(height, NSHeight(work_area));
    NSRect frame = NSMakeRect(NSMidX(work_area) - clamped_width * 0.5, NSMidY(work_area) - clamped_height * 0.5, clamped_width, clamped_height);
    [CATransaction begin];
    [CATransaction setDisableActions:YES];
    window->suppress_resize_render = true;
    [window->window setFrame:frame display:NO];
    window->suppress_resize_render = false;
    render_resize_frame(window);
    [CATransaction commit];
  });
  return result;
}

int32_t wox_darwin_window_start_dragging(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    NSEvent *event = [NSApp currentEvent];
    if (window->closed || event == nil) {
      result = -1;
      return;
    }
    [window->window performWindowDragWithEvent:event];
  });
  return result;
}

int32_t wox_darwin_window_start_file_drag(WoxDarwinWindow *window, const char *paths) {
  if (window == NULL || paths == NULL || window->closed || window->result_drag_source != NULL) {
    return -1;
  }
  __block int32_t result = -1;
  run_on_main_sync(^{
    if (window->closed || window->result_drag_source != NULL) {
      return;
    }
    NSEvent *event = window->window.currentEvent != nil ? window->window.currentEvent : NSApp.currentEvent;
    NSString *payload = [NSString stringWithUTF8String:paths];
    if (event == nil || payload == nil) {
      return;
    }
    NSArray *raw_paths = [payload componentsSeparatedByString:@"\n"];
    NSMutableArray *items = [NSMutableArray arrayWithCapacity:raw_paths.count];
    for (NSString *raw_path in raw_paths) {
      NSString *path = [raw_path stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
      if (path.length == 0 || ![[NSFileManager defaultManager] fileExistsAtPath:path]) {
        continue;
      }
      NSURL *url = [NSURL fileURLWithPath:path];
      NSDraggingItem *item = [[NSDraggingItem alloc] initWithPasteboardWriter:url];
      NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:path];
      icon = [icon copy];
      icon.size = NSMakeSize(40.0, 40.0);
      CGFloat offset = (CGFloat)items.count * 4.0;
      NSPoint point = [window->view convertPoint:event.locationInWindow fromView:nil];
      [item setDraggingFrame:NSMakeRect(point.x - 20.0 + offset, point.y - 20.0 - offset, 40.0, 40.0) contents:icon];
      [icon release];
      [items addObject:item];
      [item release];
    }
    if (items.count == 0) {
      return;
    }
    WoxResultDragSource *source = [[WoxResultDragSource alloc] initWithOwner:window];
    window->result_drag_source = source;
    NSDraggingSession *session = [window->view beginDraggingSessionWithItems:items event:event source:source];
    if (session == nil) {
      window->result_drag_source = NULL;
      [source release];
      return;
    }
    session.draggingFormation = NSDraggingFormationPile;
    session.animatesToStartingPositionsOnCancelOrFail = YES;
    result = 3;
  });
  return result;
}

int32_t wox_darwin_window_minimize(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    [window->window miniaturize:nil];
  });
  return result;
}

int32_t wox_darwin_window_set_hide_on_blur(WoxDarwinWindow *window, int32_t enabled) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    window->hide_on_blur = enabled != 0;
  });
  return result;
}

// wox_darwin_window_set_topmost raises activating utility windows above the
// launcher's NSFloatingWindowLevel so preview overlays cannot open behind Wox.
int32_t wox_darwin_window_set_topmost(WoxDarwinWindow *window, int32_t topmost) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || window->window == nil) {
      result = -1;
      return;
    }
    if (window->screenshot_window || window->nonactivating) {
      return;
    }
    window->window.level = topmost != 0 ? NSModalPanelWindowLevel : NSFloatingWindowLevel;
  });
  return result;
}

// wox_darwin_window_set_appearance keeps AppKit materials aligned with the active Wox theme.
int32_t wox_darwin_window_set_appearance(WoxDarwinWindow *window, int32_t is_dark) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    NSAppearance *appearance = [NSAppearance appearanceNamed:is_dark != 0 ? NSAppearanceNameDarkAqua : NSAppearanceNameAqua];
    window->window.appearance = appearance;
    NSApp.appearance = appearance;
  });
  return result;
}

int32_t wox_darwin_window_pick_file(WoxDarwinWindow *window, int32_t directory, char **path) {
  if (window == NULL || path == NULL) {
    return -1;
  }
  *path = NULL;
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }

    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseDirectories = directory != 0;
    panel.canChooseFiles = directory == 0;
    panel.allowsMultipleSelection = NO;
    panel.resolvesAliases = YES;

    // Keep the native picker inside the Wox focus domain so hide-on-blur does not close its owner.
    window->native_dialog_active = true;
    NSInteger response = [panel runModal];
    window->native_dialog_active = false;

    if (response == NSModalResponseOK) {
      const char *selected_path = panel.URL.path.fileSystemRepresentation;
      if (selected_path == NULL) {
        result = -1;
      } else {
        *path = strdup(selected_path);
        if (*path == NULL) {
          result = -1;
        }
      }
    } else {
      result = 1;
    }

    if (!window->closed && window->visible) {
      [NSApp activateIgnoringOtherApps:YES];
      [window->window makeKeyAndOrderFront:nil];
      [window->window makeFirstResponder:window->view];
    }
  });
  return result;
}

int32_t wox_darwin_window_save_file(WoxDarwinWindow *window, const char *title, const char *default_name, const char *extension, char **path) {
  if (window == NULL || path == NULL) {
    return -1;
  }
  *path = NULL;
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    NSSavePanel *panel = [NSSavePanel savePanel];
    if (title != NULL && title[0] != '\0') {
      panel.title = [NSString stringWithUTF8String:title];
    }
    if (default_name != NULL && default_name[0] != '\0') {
      panel.nameFieldStringValue = [NSString stringWithUTF8String:default_name];
    }
    if (extension != NULL && extension[0] != '\0') {
      panel.allowedFileTypes = @[[NSString stringWithUTF8String:extension]];
      panel.allowsOtherFileTypes = NO;
    }
    window->native_dialog_active = true;
    NSInteger response = [panel runModal];
    window->native_dialog_active = false;
    if (response == NSModalResponseOK) {
      const char *selected_path = panel.URL.path.fileSystemRepresentation;
      if (selected_path == NULL) {
        result = -1;
      } else {
        *path = strdup(selected_path);
        if (*path == NULL) {
          result = -1;
        }
      }
    } else {
      result = 1;
    }
  });
  return result;
}

int32_t wox_darwin_window_set_pointer_passthrough(WoxDarwinWindow *window, int32_t enabled) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    window->window.ignoresMouseEvents = enabled != 0;
  });
  return result;
}

int32_t wox_darwin_window_open_external_url(WoxDarwinWindow *window, const char *url) {
  if (window == NULL || url == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    NSString *value = [NSString stringWithUTF8String:url];
    NSURL *target = value != nil ? [NSURL URLWithString:value] : nil;
    if (target == nil || ![[NSWorkspace sharedWorkspace] openURL:target]) {
      result = -1;
    }
  });
  return result;
}

static void notify_darwin_webview_navigation(WoxDarwinWindow *window, WKWebView *web_view);

int32_t wox_darwin_window_show_webview(WoxDarwinWindow *window, const char *url, const char *html, const char *inject_css, const char *user_agent, int32_t cache_disabled, const char *cache_key, float x, float y, float width, float height, float corner_radius) {
  if (window == NULL || url == NULL || html == NULL || inject_css == NULL || user_agent == NULL || cache_key == NULL || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    NSString *url_value = web_view_string(url);
    NSString *html_value = web_view_string(html);
    NSString *css_value = web_view_string(inject_css);
    NSString *user_agent_value = web_view_string(user_agent);
    NSString *key_value = web_view_string(cache_key);
    bool use_cache = cache_disabled == 0 && key_value.length > 0;
    NSString *signature = [NSString stringWithFormat:@"%@\nuser-agent|%@", css_value, user_agent_value];
    NSString *content_key = html_value.length > 0 ? [@"html|" stringByAppendingString:html_value] : [@"url|" stringByAppendingString:url_value];

    WKWebView *web_view = nil;
    bool should_load = true;
    if (use_cache) {
      NSString *cached_signature = [window->web_view_signatures objectForKey:key_value];
      if ([cached_signature isEqualToString:signature]) {
        web_view = [window->web_view_cache objectForKey:key_value];
        should_load = ![[window->web_view_content_keys objectForKey:key_value] isEqualToString:content_key];
      } else {
        WKWebView *stale = [window->web_view_cache objectForKey:key_value];
        [stale stopLoading];
        [stale removeFromSuperview];
        [window->web_view_cache removeObjectForKey:key_value];
        [window->web_view_signatures removeObjectForKey:key_value];
        [window->web_view_content_keys removeObjectForKey:key_value];
      }
      if (web_view == nil) {
        web_view = create_web_view(window, css_value, user_agent_value);
        [window->web_view_cache setObject:web_view forKey:key_value];
        [window->web_view_signatures setObject:signature forKey:key_value];
        [web_view release];
      }
      [window->web_view_content_keys setObject:content_key forKey:key_value];
    } else if (window->active_web_view_transient && [window->active_web_view_signature isEqualToString:signature] && [window->active_web_view_content_key isEqualToString:content_key]) {
      web_view = window->active_web_view;
      should_load = false;
    } else {
      web_view = create_web_view(window, css_value, user_agent_value);
    }

    bool same_active = web_view == window->active_web_view;
    if (!same_active) {
      clear_active_web_view(window, true);
      window->active_web_view = web_view;
      window->active_web_view_transient = !use_cache;
      window->active_web_view_key = [key_value copy];
      window->active_web_view_signature = [signature copy];
      window->active_web_view_content_key = [content_key copy];
      [window->view addSubview:web_view positioned:NSWindowAbove relativeTo:nil];
    } else if (web_view.superview == nil) {
      [window->view addSubview:web_view positioned:NSWindowAbove relativeTo:nil];
    }
    web_view.frame = NSMakeRect(x, y, width, height);
    web_view.wantsLayer = YES;
    // Use the preview-shell radius from Go, not the window chrome radius. The
    // native surface sits inside the 1px preview border and must stay concentric.
    web_view.layer.cornerRadius = fmaxf(0.0f, fminf(corner_radius, fminf(width, height) * 0.5f));
    web_view.layer.masksToBounds = YES;
    web_view.layer.zPosition = 1.0;
    web_view.hidden = NO;
    // Floating overlay toolbar is replaced by the Go UI WebView title bar.
    if (window->web_view_toolbar != nil) {
      [window->web_view_toolbar setWebView:nil];
      [window->web_view_toolbar removeFromSuperview];
    }
    notify_darwin_webview_navigation(window, web_view);

    if (!should_load) {
      return;
    }
    if (html_value.length > 0) {
      [web_view loadHTMLString:html_value baseURL:nil];
      return;
    }
    NSURL *target = [NSURL URLWithString:url_value];
    if (target == nil) {
      result = -1;
      return;
    }
    [web_view loadRequest:[NSURLRequest requestWithURL:target]];
  });
  return result;
}


static void notify_darwin_webview_navigation(WoxDarwinWindow *window, WKWebView *web_view) {
  if (window == NULL || window->closed || window->context == 0 || web_view == nil) {
    return;
  }
  NSString *url = web_view.URL.absoluteString ?: @"";
  woxGoDarwinWebViewNavigationChanged(window->context, url.UTF8String, web_view.canGoBack ? 1 : 0, web_view.canGoForward ? 1 : 0);
}

int32_t wox_darwin_window_webview_go_back(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || window->active_web_view == nil) {
      result = -1;
      return;
    }
    if (window->active_web_view.canGoBack) {
      [window->active_web_view goBack];
    }
  });
  return result;
}

int32_t wox_darwin_window_webview_go_forward(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || window->active_web_view == nil) {
      result = -1;
      return;
    }
    if (window->active_web_view.canGoForward) {
      [window->active_web_view goForward];
    }
  });
  return result;
}

int32_t wox_darwin_window_webview_reload(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || window->active_web_view == nil) {
      result = -1;
      return;
    }
    [window->active_web_view reload];
  });
  return result;
}

int32_t wox_darwin_window_webview_open_dev_tools(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    WKWebView *web_view = window->active_web_view;
    if (window->closed || web_view == nil) {
      result = -1;
      return;
    }
    if (@available(macOS 13.3, *)) {
      web_view.inspectable = YES;
    }
    // isInspectable only exposes Safari's Develop menu; Wox's action must open the inspector directly.
    [web_view.configuration.preferences setValue:@YES forKey:@"developerExtrasEnabled"];
    id inspector = [web_view valueForKey:@"_inspector"];
    SEL show_selector = NSSelectorFromString(@"show");
    if (inspector != nil && [inspector respondsToSelector:show_selector]) {
      [inspector performSelector:show_selector];
      SEL detach_selector = NSSelectorFromString(@"detach");
      if ([inspector respondsToSelector:detach_selector]) {
        [inspector performSelector:detach_selector];
      }
      return;
    }
    SEL fallback_selector = NSSelectorFromString(@"_showWebInspector");
    if ([web_view respondsToSelector:fallback_selector]) {
      [web_view performSelector:fallback_selector];
      return;
    }
    result = -1;
  });
  return result;
}

int32_t wox_darwin_window_webview_open_in_browser(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || window->active_web_view == nil) {
      result = -1;
      return;
    }
    NSURL *url = window->active_web_view.URL;
    NSString *scheme = url.scheme.lowercaseString;
    if (url.host.length > 0 && ([scheme isEqualToString:@"http"] || [scheme isEqualToString:@"https"])) {
      [[NSWorkspace sharedWorkspace] openURL:url];
      return;
    }
    result = -1;
  });
  return result;
}

int32_t wox_darwin_window_webview_navigation_state(WoxDarwinWindow *window, char **url, int32_t *can_go_back, int32_t *can_go_forward) {
  if (window == NULL || url == NULL || can_go_back == NULL || can_go_forward == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    *url = NULL;
    *can_go_back = 0;
    *can_go_forward = 0;
    if (window->closed || window->active_web_view == nil) {
      result = -1;
      return;
    }
    NSString *value = window->active_web_view.URL.absoluteString ?: @"";
    *url = strdup(value.UTF8String);
    *can_go_back = window->active_web_view.canGoBack ? 1 : 0;
    *can_go_forward = window->active_web_view.canGoForward ? 1 : 0;
  });
  return result;
}

int32_t wox_darwin_window_hide_webview(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    clear_active_web_view(window, true);
  });
  return result;
}

int32_t wox_darwin_window_reset_webview(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    clear_active_web_view(window, true);
    for (WKWebView *web_view in [window->web_view_cache allValues]) {
      [web_view stopLoading];
      [web_view removeFromSuperview];
    }
    [window->web_view_cache removeAllObjects];
    [window->web_view_signatures removeAllObjects];
    [window->web_view_content_keys removeAllObjects];
  });
  return result;
}

int32_t wox_darwin_window_forward_embedded_surface_pointer(WoxDarwinWindow *window, uint8_t kind) {
  if (window == NULL || window->closed || window->active_web_view == nil || window->forwarding_embedded_pointer) {
    return -1;
  }
  window->pointer_over_web_view = kind != WOX_POINTER_LEAVE;
  if (kind == WOX_POINTER_LEAVE) {
    apply_darwin_pointer_cursor(window);
    return 0;
  }
  NSEvent *event = NSApp.currentEvent;
  if (event == nil) {
    return -1;
  }
  NSPoint local = [window->active_web_view convertPoint:event.locationInWindow fromView:nil];
  NSView *target = [window->active_web_view hitTest:local];
  if (target == nil) {
    return -1;
  }
  // WebKit can bubble an unhandled event back to WoxRenderView. Suppress that callback while
  // delivering the current event so it cannot synchronously reenter this forwarding path.
  window->forwarding_embedded_pointer = true;
  int32_t result = 0;
  switch (event.type) {
  case NSEventTypeLeftMouseDown:
    [window->window makeFirstResponder:window->active_web_view];
    [target mouseDown:event];
    break;
  case NSEventTypeRightMouseDown:
    [window->window makeFirstResponder:window->active_web_view];
    [target rightMouseDown:event];
    break;
  case NSEventTypeOtherMouseDown:
    [window->window makeFirstResponder:window->active_web_view];
    [target otherMouseDown:event];
    break;
  case NSEventTypeLeftMouseUp:
    [target mouseUp:event];
    break;
  case NSEventTypeRightMouseUp:
    [target rightMouseUp:event];
    break;
  case NSEventTypeOtherMouseUp:
    [target otherMouseUp:event];
    break;
  case NSEventTypeScrollWheel:
    [target scrollWheel:event];
    break;
  case NSEventTypeMouseMoved:
  case NSEventTypeMouseEntered:
  case NSEventTypeMouseExited:
    [target mouseMoved:event];
    break;
  case NSEventTypeLeftMouseDragged:
    [target mouseDragged:event];
    break;
  case NSEventTypeRightMouseDragged:
    [target rightMouseDragged:event];
    break;
  case NSEventTypeOtherMouseDragged:
    [target otherMouseDragged:event];
    break;
  default:
    result = -1;
    break;
  }
  window->forwarding_embedded_pointer = false;
  apply_darwin_pointer_cursor(window);
  return result;
}

int32_t wox_darwin_window_write_clipboard_text(WoxDarwinWindow *window, const char *text) {
  if (window == NULL || text == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    NSString *value = [NSString stringWithUTF8String:text];
    if (value == nil) {
      result = -1;
      return;
    }
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    if (![pasteboard setString:value forType:NSPasteboardTypeString]) {
      result = -1;
    }
  });
  return result;
}

int32_t wox_darwin_window_write_clipboard_image(WoxDarwinWindow *window, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride) {
  if (window == NULL || pixels == NULL || width <= 0 || height <= 0 || row_stride < width * 4) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }

    NSBitmapImageRep *representation = [[NSBitmapImageRep alloc]
        initWithBitmapDataPlanes:NULL
                  pixelsWide:width
                  pixelsHigh:height
               bitsPerSample:8
             samplesPerPixel:4
                    hasAlpha:YES
                    isPlanar:NO
              colorSpaceName:NSCalibratedRGBColorSpace
                 bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
                  bytesPerRow:row_stride
                 bitsPerPixel:32];
    if (representation == nil || representation.bitmapData == NULL) {
      [representation release];
      result = -1;
      return;
    }
    memcpy(representation.bitmapData, pixels, (size_t)row_stride * (size_t)height);
    NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(width, height)];
    [image addRepresentation:representation];
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    if (![pasteboard writeObjects:@[ image ]]) {
      result = -1;
    }
    [image release];
    [representation release];
  });
  return result;
}

int32_t wox_darwin_window_invalidate(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    schedule_render(window);
  });
  return result;
}

static CVReturn darwin_display_link_tick(CVDisplayLinkRef display_link, const CVTimeStamp *now, const CVTimeStamp *output, CVOptionFlags flags, CVOptionFlags *flags_out, void *context) {
  (void)display_link;
  (void)now;
  (void)output;
  (void)flags;
  (void)flags_out;
  WoxDarwinWindow *window = context;
  dispatch_async(dispatch_get_main_queue(), ^{
    if (window == NULL || window->closed || !window->visible || !window->animation_frame_pending) {
      return;
    }
    window->animation_frame_pending = false;
    schedule_render(window);
  });
  return kCVReturnSuccess;
}

// darwin_window_display_id reads the NSScreenNumber for the window's current display.
static CGDirectDisplayID darwin_window_display_id(WoxDarwinWindow *window) {
  if (window == NULL || window->window == nil) {
    return kCGNullDirectDisplay;
  }
  NSScreen *screen = window->window.screen ?: [NSScreen mainScreen];
  NSNumber *screen_number = [screen.deviceDescription objectForKey:@"NSScreenNumber"];
  if (screen_number == nil) {
    return kCGNullDirectDisplay;
  }
  return (CGDirectDisplayID)screen_number.unsignedIntValue;
}

// bind_darwin_display_link pins CVDisplayLink to the window's current display so mixed-refresh layouts stay in sync.
static void bind_darwin_display_link(WoxDarwinWindow *window) {
  if (window == NULL || window->display_link == NULL) {
    return;
  }
  CGDirectDisplayID display_id = darwin_window_display_id(window);
  if (display_id == kCGNullDirectDisplay || display_id == window->display_link_display) {
    return;
  }
  if (CVDisplayLinkSetCurrentCGDisplay(window->display_link, display_id) == kCVReturnSuccess) {
    window->display_link_display = display_id;
  }
}

static void stop_darwin_display_link(WoxDarwinWindow *window) {
  if (window == NULL || window->display_link == NULL) {
    return;
  }
  CVDisplayLinkStop(window->display_link);
  CVDisplayLinkRelease(window->display_link);
  window->display_link = NULL;
  window->display_link_display = kCGNullDirectDisplay;
  window->animation_frame_pending = false;
}

int32_t wox_darwin_window_request_animation_frame(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || !window->visible) {
      result = -1;
      return;
    }
    window->animation_frame_pending = true;
    if (window->display_link != NULL) {
      bind_darwin_display_link(window);
      return;
    }
    if (CVDisplayLinkCreateWithActiveCGDisplays(&window->display_link) != kCVReturnSuccess) {
      window->display_link = NULL;
      result = -1;
      return;
    }
    window->display_link_display = kCGNullDirectDisplay;
    bind_darwin_display_link(window);
    CVDisplayLinkSetOutputCallback(window->display_link, darwin_display_link_tick, window);
    if (CVDisplayLinkStart(window->display_link) != kCVReturnSuccess) {
      stop_darwin_display_link(window);
      result = -1;
    }
  });
  return result;
}

int32_t wox_darwin_window_stop_animation_frames(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  run_on_main_sync(^{
    stop_darwin_display_link(window);
  });
  return 0;
}

// wox_darwin_window_set_text_input_state updates AppKit's candidate position on its owning thread.
int32_t wox_darwin_window_set_text_input_state(WoxDarwinWindow *window, int32_t enabled, float x, float y, float width, float height) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    window->input_enabled = enabled != 0;
    window->input_cursor_rect = NSMakeRect(x, y, fmaxf(width, 1.0f), fmaxf(height, 1.0f));
    if (!window->input_enabled) {
      [window->view unmarkText];
    } else if (window->window.isKeyWindow) {
      [window->window makeFirstResponder:window->view];
    }
    [[window->view inputContext] invalidateCharacterCoordinates];
  });
  return result;
}

int32_t wox_darwin_window_set_pointer_cursor(WoxDarwinWindow *window, uint8_t cursor) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    window->pointer_cursor = cursor;
    apply_darwin_pointer_cursor(window);
  });
  return result;
}

static NSString *accessibility_role(const char *role) {
  NSString *value = role != NULL ? [NSString stringWithUTF8String:role] : @"";
  if ([value isEqualToString:@"window"] || [value isEqualToString:@"dialog"]) return NSAccessibilityGroupRole;
  if ([value isEqualToString:@"text"] || [value isEqualToString:@"heading"]) return NSAccessibilityStaticTextRole;
  if ([value isEqualToString:@"button"]) return NSAccessibilityButtonRole;
  if ([value isEqualToString:@"text_field"]) return NSAccessibilityTextFieldRole;
  if ([value isEqualToString:@"checkbox"]) return NSAccessibilityCheckBoxRole;
  if ([value isEqualToString:@"radio_button"]) return NSAccessibilityRadioButtonRole;
  if ([value isEqualToString:@"list"]) return NSAccessibilityListRole;
  if ([value isEqualToString:@"list_item"]) return NSAccessibilityRowRole;
  if ([value isEqualToString:@"image"]) return NSAccessibilityImageRole;
  if ([value isEqualToString:@"progress_bar"]) return NSAccessibilityProgressIndicatorRole;
  if ([value isEqualToString:@"link"]) return NSAccessibilityLinkRole;
  if ([value isEqualToString:@"menu"]) return NSAccessibilityMenuRole;
  if ([value isEqualToString:@"menu_item"]) return NSAccessibilityMenuItemRole;
  return NSAccessibilityGroupRole;
}

int32_t wox_darwin_accessibility_begin(WoxDarwinWindow *window, uint64_t generation) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed) {
      result = -1;
      return;
    }
    [window->accessibility_elements release];
    [window->accessibility_child_ids release];
    [window->accessibility_roots release];
    window->accessibility_elements = [[NSMutableDictionary alloc] init];
    window->accessibility_child_ids = [[NSMutableDictionary alloc] init];
    window->accessibility_roots = [[NSMutableArray alloc] init];
    window->accessibility_generation = generation;
  });
  return result;
}

int32_t wox_darwin_accessibility_add_node(WoxDarwinWindow *window, uint64_t id, uint64_t parent_id, const uint64_t *children, int32_t child_count, const char *automation_id, const char *role, const char *label, const char *description, const char *value, float x, float y, float width, float height, uint32_t state_flags, uint32_t action_flags, int32_t live_region) {
  if (window == NULL || id == 0 || child_count < 0) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || window->accessibility_elements == nil) {
      result = -1;
      return;
    }
    NSString *native_label = label != NULL ? [NSString stringWithUTF8String:label] : @"";
    NSRect local = NSMakeRect(x, y, fmaxf(width, 0.0f), fmaxf(height, 0.0f));
    NSRect window_rect = [window->view convertRect:local toView:nil];
    NSRect screen_rect = [window->window convertRectToScreen:window_rect];
    WoxAccessibilityElement *element = [[WoxAccessibilityElement alloc] init];
    element->_owner = window;
    element->_node_id = id;
    element->_action_flags = action_flags;
    element->_configuring = YES;
    NSString *identifier = automation_id != NULL ? [NSString stringWithUTF8String:automation_id] : @"";
    NSString *help = description != NULL ? [NSString stringWithUTF8String:description] : @"";
    NSString *native_value = value != NULL ? [NSString stringWithUTF8String:value] : @"";
    [element setAccessibilityRole:accessibility_role(role)];
    [element setAccessibilityFrame:screen_rect];
    [element setAccessibilityLabel:native_label];
    [element setAccessibilityParent:window->view];
    [element setAccessibilityIdentifier:identifier];
    [element setAccessibilityHelp:help];
    [element setAccessibilityEnabled:(state_flags & WOX_ACCESSIBILITY_STATE_ENABLED) != 0];
    [element setAccessibilityFocused:(state_flags & WOX_ACCESSIBILITY_STATE_FOCUSED) != 0];
    [element setAccessibilitySelected:(state_flags & WOX_ACCESSIBILITY_STATE_SELECTED) != 0];
    [element setAccessibilityExpanded:(state_flags & WOX_ACCESSIBILITY_STATE_EXPANDED) != 0];
    if ([accessibility_role(role) isEqualToString:NSAccessibilityCheckBoxRole] || [accessibility_role(role) isEqualToString:NSAccessibilityRadioButtonRole]) {
      [element setAccessibilityValue:@((state_flags & WOX_ACCESSIBILITY_STATE_CHECKED) != 0)];
    } else {
      [element setAccessibilityValue:(state_flags & WOX_ACCESSIBILITY_STATE_PROTECTED) != 0 ? @"" : native_value];
    }
    [element setAccessibilityHidden:(state_flags & WOX_ACCESSIBILITY_STATE_HIDDEN) != 0];
    element->_configuring = NO;
    NSNumber *node_key = @(id);
    [window->accessibility_elements setObject:element forKey:node_key];
    [element release];

    NSMutableArray *child_ids = [NSMutableArray arrayWithCapacity:(NSUInteger)child_count];
    for (int32_t index = 0; index < child_count; index++) {
      [child_ids addObject:@(children[index])];
    }
    [window->accessibility_child_ids setObject:child_ids forKey:node_key];
    if (parent_id == 0) {
      [window->accessibility_roots addObject:node_key];
    }
    (void)live_region;
  });
  return result;
}

int32_t wox_darwin_accessibility_end(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (window->closed || window->accessibility_elements == nil) {
      result = -1;
      return;
    }
    for (NSNumber *node_key in window->accessibility_elements) {
      WoxAccessibilityElement *element = [window->accessibility_elements objectForKey:node_key];
      NSArray *child_ids = [window->accessibility_child_ids objectForKey:node_key] ?: @[];
      NSMutableArray *children = [NSMutableArray arrayWithCapacity:child_ids.count];
      for (NSNumber *child_id in child_ids) {
        WoxAccessibilityElement *child = [window->accessibility_elements objectForKey:child_id];
        if (child != nil) {
          [child setAccessibilityParent:element];
          [children addObject:child];
        }
      }
      [element setAccessibilityChildren:children];
    }
    NSMutableArray *roots = [NSMutableArray arrayWithCapacity:window->accessibility_roots.count];
    for (NSNumber *root_id in window->accessibility_roots) {
      WoxAccessibilityElement *root = [window->accessibility_elements objectForKey:root_id];
      if (root != nil) {
        [root setAccessibilityParent:window->view];
        [roots addObject:root];
      }
    }
    [window->view setWoxAccessibilityChildren:roots];
    NSAccessibilityPostNotification(window->view, NSAccessibilityLayoutChangedNotification);
  });
  return result;
}

static NSFont *wox_font(const char *font_family, CGFloat size, uint8_t font_weight) {
  NSFontWeight weight = font_weight == 1 ? NSFontWeightSemibold : NSFontWeightRegular;
  if (font_family != NULL && font_family[0] != '\0') {
    NSString *family = [NSString stringWithUTF8String:font_family];
    if (family != nil) {
      NSFontDescriptor *descriptor = [NSFontDescriptor fontDescriptorWithFontAttributes:@{
        NSFontFamilyAttribute: family,
        NSFontTraitsAttribute: @{NSFontWeightTrait: @(weight)},
      }];
      NSFont *font = [NSFont fontWithDescriptor:descriptor size:size];
      if (font != nil) {
        return font;
      }
    }
  }
  return [NSFont systemFontOfSize:size weight:weight];
}

// wox_darwin_window_measure_text returns logical CoreText metrics for the configured UI font.
int32_t wox_darwin_window_measure_text(WoxDarwinWindow *window, const char *text, const char *font_family, float font_size, uint8_t font_weight, float *width, float *height, float *baseline) {
  if (window == NULL || text == NULL || width == NULL || height == NULL || baseline == NULL || font_size <= 0.0f || font_weight > 1) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    *width = 0.0f;
    *height = 0.0f;
    *baseline = 0.0f;
    if (window->closed || text[0] == '\0') {
      result = window->closed ? -1 : 0;
      return;
    }
    NSString *string = [[NSString alloc] initWithUTF8String:text];
    if (string == nil) {
      result = -1;
      return;
    }
    NSFont *font = wox_font(font_family, font_size, font_weight);
    NSAttributedString *attributed = [[NSAttributedString alloc] initWithString:string attributes:@{(id)kCTFontAttributeName : font}];
    CTLineRef line = CTLineCreateWithAttributedString((CFAttributedStringRef)attributed);
    CGFloat ascent = 0.0;
    CGFloat descent = 0.0;
    CGFloat leading = 0.0;
    double measured_width = CTLineGetTypographicBounds(line, &ascent, &descent, &leading);
    *width = (float)measured_width;
    *height = (float)(ascent + descent + leading);
    *baseline = (float)ascent;
    CFRelease(line);
    [attributed release];
    [string release];
  });
  return result;
}

int32_t wox_darwin_window_close(WoxDarwinWindow *window) {
  if (window == NULL) {
    return -1;
  }
  run_on_main_sync(^{
    if (window->closed) {
      return;
    }
    uintptr_t context = window->context;
    uint64_t epoch = window->epoch;
    bool was_active = window->active;
    window->closed = true;
    window->visible = false;
    window->active = false;
    stop_darwin_display_link(window);
    window->context = 0;
    clear_previous_active_app(window);
    if (was_active && context != 0) {
      woxGoDarwinFocus(context, epoch, 0);
    }

    window->view->_owner = NULL;
    window->delegate->_owner = NULL;
    clear_active_web_view(window, true);
    [window->web_view_toolbar release];
    window->web_view_toolbar = nil;
    [window->web_view_cache removeAllObjects];
    [window->web_view_signatures removeAllObjects];
    [window->web_view_content_keys removeAllObjects];
    [window->web_view_cache release];
    [window->web_view_signatures release];
    [window->web_view_content_keys release];
    [window->accessibility_elements release];
    [window->accessibility_child_ids release];
    [window->accessibility_roots release];
    window->web_view_cache = nil;
    window->web_view_signatures = nil;
    window->web_view_content_keys = nil;
    window->accessibility_elements = nil;
    window->accessibility_child_ids = nil;
    window->accessibility_roots = nil;
    [window->view setWoxAccessibilityChildren:@[]];
    window->window.delegate = nil;
    [window->window close];
    clear_cached_images(window);
    destroy_renderer(window->renderer);
    destroy_renderer(window->overlay_renderer);
    window->renderer = NULL;
    window->overlay_renderer = NULL;
    // The AppKit run loop is wrapped by one process-lifetime autorelease pool. Closed
    // management windows must release their owned objects here or every reopen keeps a
    // complete NSWindow hierarchy alive until the application exits.
    [window->delegate release];
    [window->view release];
    [window->window release];
    window->delegate = nil;
    window->view = nil;
    window->window = nil;

    if (wox_open_window_count > 0) {
      wox_open_window_count--;
    }
    if (wox_open_window_count == 0) {
      [NSApp stop:nil];
      NSEvent *wake_event = [NSEvent otherEventWithType:NSApplicationDefined
                                               location:NSZeroPoint
                                          modifierFlags:0
                                              timestamp:0
                                           windowNumber:0
                                                context:nil
                                                subtype:0
                                                  data1:0
                                                  data2:0];
      [NSApp postEvent:wake_event atStart:NO];
    }
    // ponytail: retain the small closed handle as a tombstone; add reference-counted destruction only if windows are created repeatedly.
  });
  return 0;
}

void *wox_darwin_autorelease_pool_push(void) {
  return [[NSAutoreleasePool alloc] init];
}

void wox_darwin_autorelease_pool_pop(void *pool) {
  [(NSAutoreleasePool *)pool drain];
}

static int32_t begin_darwin_renderer_frame(WoxDarwinWindow *window, WoxDarwinRenderer *renderer, uint64_t frame_id, float logical_width, float logical_height, float scale, float damage_x, float damage_y, float damage_width, float damage_height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->closed || renderer == NULL || logical_width <= 0.0f || logical_height <= 0.0f || scale <= 0.0f) {
    return -1;
  }
  // A queued frame can reach the renderer after AppKit hides on blur without crossing Go's hide
  // path. Skip it so it cannot recreate the IOSurface pool behind an invisible window.
  if (!window->visible) {
    return WOX_DARWIN_FRAME_SKIPPED;
  }
  if (renderer->frame_open) {
    return -1;
  }

  NSUInteger pixel_width = (NSUInteger)ceilf(logical_width * scale);
  NSUInteger pixel_height = (NSUInteger)ceilf(logical_height * scale);
  if (pixel_width == 0 || pixel_height == 0 || pixel_width > 16384 || pixel_height > 16384) {
    return -1;
  }
  WoxDarwinSurface *surface = acquire_render_surface(renderer, pixel_width, pixel_height);
  if (surface == nil) {
    return WOX_DARWIN_FRAME_SURFACE_BUSY;
  }
  if (IOSurfaceLock(surface->io_surface, 0, NULL) != kIOReturnSuccess) {
    [surface release];
    return -1;
  }

  CGColorSpaceRef color_space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
  if (color_space == NULL) {
    IOSurfaceUnlock(surface->io_surface, 0, NULL);
    [surface release];
    return -1;
  }
  CGContextRef context = CGBitmapContextCreate(
      IOSurfaceGetBaseAddress(surface->io_surface),
      pixel_width,
      pixel_height,
      8,
      IOSurfaceGetBytesPerRow(surface->io_surface),
      color_space,
      kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little);
  CGColorSpaceRelease(color_space);
  if (context == NULL) {
    IOSurfaceUnlock(surface->io_surface, 0, NULL);
    [surface release];
    return -1;
  }

  CGContextTranslateCTM(context, 0.0, pixel_height);
  CGContextScaleCTM(context, scale, -scale);
  uint64_t frame_sequence = renderer->submission_sequence + 1;
  bool requested_full = damage_width <= 0.0f || damage_height <= 0.0f;
  CGRect requested_damage = CGRectMake(damage_x, damage_y, fmaxf(0.0f, damage_width), fmaxf(0.0f, damage_height));
  CGRect effective_damage = requested_damage;
  bool effective_full = requested_full || surface->content_sequence == 0 || surface->content_sequence >= frame_sequence || frame_sequence - surface->content_sequence > 64;
  if (!effective_full) {
    for (uint64_t sequence = surface->content_sequence + 1; sequence < frame_sequence; sequence++) {
      const WoxDarwinDamageRecord record = renderer->damage_history[sequence % 64];
      if (record.sequence != sequence || record.full) {
        effective_full = true;
        break;
      }
      effective_damage = CGRectUnion(effective_damage, record.damage);
    }
  }

  // On M3, the first Metal render pass reserves about 200 MB of driver memory; drawing the same IOSurface on the CPU avoids that fixed visible-state cost.
  CGContextSetBlendMode(context, kCGBlendModeCopy);
  CGContextSetRGBFillColor(context, red / 255.0, green / 255.0, blue / 255.0, alpha / 255.0);
  if (effective_full) {
    CGContextFillRect(context, CGRectMake(0.0, 0.0, logical_width, logical_height));
  } else {
    CGContextSaveGState(context);
    CGContextClipToRect(context, effective_damage);
    CGContextFillRect(context, effective_damage);
  }
  CGContextSetBlendMode(context, kCGBlendModeNormal);
  CGContextSetShouldAntialias(context, true);
  CGContextSetInterpolationQuality(context, kCGInterpolationHigh);

  renderer->frame_surface = surface;
  renderer->context = context;
  renderer->viewport_size = CGSizeMake(logical_width, logical_height);
  renderer->scale = scale;
  renderer->frame_id = frame_id;
  renderer->frame_generation = atomic_load_explicit(&window->presentation_generation, memory_order_relaxed);
  renderer->frame_sequence = frame_sequence;
  renderer->frame_requested_damage = requested_damage;
  renderer->frame_requested_full = requested_full;
  renderer->frame_open = true;
  renderer->clip_active = false;
  renderer->damage_clip_active = !effective_full;
  return 0;
}

int32_t wox_darwin_window_begin_frame(WoxDarwinWindow *window, uint64_t frame_id, float logical_width, float logical_height, float scale, float damage_x, float damage_y, float damage_width, float damage_height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->renderer == NULL || window->overlay_renderer == NULL) {
    return -1;
  }
  window->embedded_surface_overlay_active = false;
  window->active_renderer = window->renderer;
  memset(&window->frame_resource_stats, 0, sizeof(window->frame_resource_stats));
  return begin_darwin_renderer_frame(window, window->renderer, frame_id, logical_width, logical_height, scale, damage_x, damage_y, damage_width, damage_height, red, green, blue, alpha);
}

// wox_darwin_window_trim_render_surfaces keeps the front buffer and one reusable current-size back buffer.
int32_t wox_darwin_window_trim_render_surfaces(WoxDarwinWindow *window, int32_t max_surfaces) {
  if (window == NULL || window->closed || window->renderer == NULL || max_surfaces < 1) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  if (renderer->frame_open || renderer->render_surfaces.count <= (NSUInteger)max_surfaces) {
    return (int32_t)renderer->render_surfaces.count;
  }

  NSMutableArray *stale_surfaces = [NSMutableArray array];
  NSMutableArray *matching_surfaces = [NSMutableArray array];
  NSUInteger current_width = (NSUInteger)ceilf(renderer->viewport_size.width * renderer->scale);
  NSUInteger current_height = (NSUInteger)ceilf(renderer->viewport_size.height * renderer->scale);
  for (WoxDarwinSurface *surface in renderer->render_surfaces) {
    if (atomic_load_explicit(&surface->presentation_references, memory_order_relaxed) != 0 ||
        IOSurfaceIsInUse(surface->io_surface)) {
      continue;
    }
    if (surface->width == current_width && surface->height == current_height) {
      [matching_surfaces addObject:surface];
    } else {
      [stale_surfaces addObject:surface];
    }
  }

  for (WoxDarwinSurface *surface in stale_surfaces) {
    if (renderer->render_surfaces.count <= (NSUInteger)max_surfaces) {
      break;
    }
    [renderer->render_surfaces removeObjectIdenticalTo:surface];
  }
  NSUInteger removable_matching_count = matching_surfaces.count > 1 ? matching_surfaces.count - 1 : 0;
  for (NSUInteger index = 0; index < removable_matching_count && renderer->render_surfaces.count > (NSUInteger)max_surfaces; index++) {
    [renderer->render_surfaces removeObjectIdenticalTo:matching_surfaces[index]];
  }
  return (int32_t)renderer->render_surfaces.count;
}

int32_t wox_darwin_window_fill_rounded_rect(WoxDarwinWindow *window, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f) {
    return 0;
  }

  WoxDarwinRenderer *renderer = window->active_renderer;
  CGFloat clamped_radius = fmaxf(0.0f, fminf(radius, fminf(width, height) / 2.0f));
  CGPathRef path = CGPathCreateWithRoundedRect(CGRectMake(x, y, width, height), clamped_radius, clamped_radius, NULL);
  CGContextSetRGBFillColor(renderer->context, red / 255.0, green / 255.0, blue / 255.0, alpha / 255.0);
  CGContextAddPath(renderer->context, path);
  CGContextFillPath(renderer->context);
  CGPathRelease(path);
  return 0;
}

int32_t wox_darwin_window_fill_convex_polygon(WoxDarwinWindow *window, const float *points, int32_t point_count, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  if (points == NULL || point_count < 3 || point_count > 16) {
    return -1;
  }

  WoxDarwinRenderer *renderer = window->active_renderer;
  CGContextBeginPath(renderer->context);
  CGContextMoveToPoint(renderer->context, points[0], points[1]);
  for (int32_t index = 1; index < point_count; index++) {
    CGContextAddLineToPoint(renderer->context, points[index * 2], points[index * 2 + 1]);
  }
  CGContextClosePath(renderer->context);
  CGContextSetRGBFillColor(renderer->context, red / 255.0, green / 255.0, blue / 255.0, alpha / 255.0);
  CGContextFillPath(renderer->context);
  return 0;
}

int32_t wox_darwin_call(uintptr_t context) {
  if (context == 0 || [NSApplication sharedApplication] == nil) {
    return -1;
  }
  if ([NSThread isMainThread]) {
    woxGoDarwinCall(context);
    return 0;
  }
  dispatch_sync(dispatch_get_main_queue(), ^{
    woxGoDarwinCall(context);
  });
  return 0;
}

int32_t wox_darwin_window_stroke_rounded_rect(WoxDarwinWindow *window, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f || stroke_width <= 0.0f) {
    return 0;
  }

  WoxDarwinRenderer *renderer = window->active_renderer;
  CGFloat inset = stroke_width / 2.0f;
  if (width <= stroke_width || height <= stroke_width) {
    return 0;
  }
  CGRect stroke_rect = CGRectInset(CGRectMake(x, y, width, height), inset, inset);
  CGFloat stroke_radius = fmaxf(0.0f, fminf(radius - inset, fminf(stroke_rect.size.width, stroke_rect.size.height) / 2.0f));
  CGPathRef path = CGPathCreateWithRoundedRect(stroke_rect, stroke_radius, stroke_radius, NULL);
  CGContextSetRGBStrokeColor(renderer->context, red / 255.0, green / 255.0, blue / 255.0, alpha / 255.0);
  CGContextSetLineWidth(renderer->context, stroke_width);
  CGContextAddPath(renderer->context, path);
  CGContextStrokePath(renderer->context);
  CGPathRelease(path);
  return 0;
}

int32_t wox_darwin_window_draw_text(WoxDarwinWindow *window, const char *text, const char *font_family, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open || text == NULL) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->active_renderer;
  if (text[0] == '\0' || width <= 0.0f || height <= 0.0f || font_size <= 0.0f || !isfinite(width) || !isfinite(height) || !isfinite(font_size)) {
    return 0;
  }

  NSString *string = [[NSString alloc] initWithUTF8String:text];
  if (string == nil) {
    return -1;
  }
  NSFont *font = wox_font(font_family, font_size, font_weight);
  NSColor *color = [NSColor colorWithSRGBRed:red / 255.0 green:green / 255.0 blue:blue / 255.0 alpha:alpha / 255.0];
  NSDictionary *attributes = [NSDictionary dictionaryWithObjectsAndKeys:
      font, (id)kCTFontAttributeName,
      (id)color.CGColor, (id)kCTForegroundColorAttributeName,
      nil];
  NSAttributedString *attributed = [[NSAttributedString alloc] initWithString:string attributes:attributes];
  CTLineRef line = CTLineCreateWithAttributedString((CFAttributedStringRef)attributed);
  CGFloat ascent = 0.0;
  CTLineGetTypographicBounds(line, &ascent, NULL, NULL);
  CGContextSaveGState(renderer->context);
  CGContextClipToRect(renderer->context, CGRectMake(x, y, width, height));
  CGContextSetTextMatrix(renderer->context, CGAffineTransformMakeScale(1.0, -1.0));
  CGContextSetTextPosition(renderer->context, x, y + ascent);
  CTLineDraw(line, renderer->context);
  CGContextRestoreGState(renderer->context);

  CFRelease(line);
  [attributed release];
  [string release];
  window->frame_resource_stats.text_rasterizations++;
  return 0;
}

int32_t wox_darwin_window_draw_image(WoxDarwinWindow *window, uint64_t image_id, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height, float rotation_radians, float corner_radius) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open || image_id == 0 || pixels == NULL || image_width <= 0 || image_height <= 0 || row_stride < image_width * 4 || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->active_renderer;
  uint64_t image_bytes = (uint64_t)row_stride * (uint64_t)image_height;
  bool large_image = image_bytes > wox_cached_image_max_entry_bytes;
  CGImageRef image = NULL;
  bool release_image = true;
  // Lookup is deliberately not gated on cache_large_images: that flag governs admission only, so a
  // slot already holding this image keeps serving hits while an alternating image churns candidacy.
  if (large_image && window->cached_large_image_id == image_id && window->cached_large_image != NULL) {
    image = window->cached_large_image;
    release_image = false;
    window->frame_resource_stats.cache_hits++;
  } else if (!large_image && (image = find_cached_cgimage(window, image_id)) != NULL) {
    release_image = false;
    window->frame_resource_stats.cache_hits++;
  } else {
    // Settle the admission verdict before creating, because it decides whether the CGImage must
    // own a copy of the pixels. Deciding afterwards would either copy for an image that is then
    // rejected, or force a second create for one that is accepted.
    if (large_image) {
      note_large_image_repeat(window, image_id);
    }
    bool cache_regular = !large_image;
    bool cache_large = large_image && window->cache_large_images && image_bytes <= wox_cached_large_image_max_bytes;
    image = create_cgimage_from_pixels(pixels, image_width, image_height, row_stride, cache_regular || cache_large);
    if (image == NULL) {
      return -1;
    }
    window->frame_resource_stats.image_creates++;
    if (cache_large) {
      clear_cached_large_image(window);
      window->cached_large_image = image;
      window->cached_large_image_id = image_id;
      window->cached_large_image_bytes = image_bytes;
      release_image = false;
    } else if (cache_regular && cache_cgimage(window, image_id, image_bytes, image)) {
      release_image = false;
    }
  }
  draw_cached_cgimage(renderer, image, x, y, width, height, rotation_radians, corner_radius);
  if (release_image) {
    CGImageRelease(image);
  }
  return 0;
}

int32_t wox_darwin_window_set_clip_rect(WoxDarwinWindow *window, float x, float y, float width, float height) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->active_renderer;
  float max_width = renderer->viewport_size.width;
  float max_height = renderer->viewport_size.height;
  float left = fmaxf(0.0f, fminf(max_width, x));
  float top = fmaxf(0.0f, fminf(max_height, y));
  float right = fmaxf(left, fminf(max_width, x + fmaxf(0.0f, width)));
  float bottom = fmaxf(top, fminf(max_height, y + fmaxf(0.0f, height)));
  left = floorf(left * renderer->scale) / renderer->scale;
  top = floorf(top * renderer->scale) / renderer->scale;
  right = ceilf(right * renderer->scale) / renderer->scale;
  bottom = ceilf(bottom * renderer->scale) / renderer->scale;
  if (renderer->clip_active) {
    CGContextRestoreGState(renderer->context);
  }
  CGContextSaveGState(renderer->context);
  CGContextClipToRect(renderer->context, CGRectMake(left, top, right - left, bottom - top));
  renderer->clip_active = true;
  renderer->clip_rect = CGRectMake(left, top, right - left, bottom - top);
  return 0;
}

int32_t wox_darwin_window_clear_clip(WoxDarwinWindow *window) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->active_renderer;
  if (renderer->clip_active) {
    CGContextRestoreGState(renderer->context);
    renderer->clip_active = false;
  }
  return 0;
}

static int32_t finish_darwin_renderer_frame(WoxDarwinWindow *window, WoxDarwinRenderer *renderer, int32_t transactional) {
  if (window == NULL || renderer == NULL || !renderer->frame_open) {
    return -1;
  }
  if (renderer->clip_active) {
    CGContextRestoreGState(renderer->context);
    renderer->clip_active = false;
  }
  if (renderer->damage_clip_active) {
    CGContextRestoreGState(renderer->context);
    renderer->damage_clip_active = false;
  }
  CGContextFlush(renderer->context);
  CGContextRelease(renderer->context);
  renderer->context = NULL;
  WoxDarwinSurface *surface = renderer->frame_surface;
  IOSurfaceUnlock(surface->io_surface, 0, NULL);
  renderer->frame_surface = nil;
  renderer->frame_open = false;

  uint64_t sequence = renderer->frame_sequence;
  uint64_t frame_id = renderer->frame_id;
  renderer->submission_sequence = sequence;
  renderer->damage_history[sequence % 64].sequence = sequence;
  renderer->damage_history[sequence % 64].damage = renderer->frame_requested_damage;
  renderer->damage_history[sequence % 64].full = renderer->frame_requested_full;
  surface->content_sequence = sequence;
  uint64_t generation = renderer->frame_generation;
  atomic_fetch_add_explicit(&surface->presentation_references, 1, memory_order_relaxed);
  if ([NSThread isMainThread]) {
    present_render_surface(window, renderer, surface, frame_id, sequence, generation, transactional != 0);
  } else {
    WoxDarwinSurface *present_surface = [surface retain];
    dispatch_async(dispatch_get_main_queue(), ^{
      present_render_surface(window, renderer, present_surface, frame_id, sequence, generation, false);
      [present_surface release];
    });
  }
  [surface release];
  return 0;
}

int32_t wox_darwin_window_begin_embedded_surface_overlay(WoxDarwinWindow *window) {
  if (window == NULL || window->active_renderer != window->renderer || !window->renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *background = window->renderer;
  bool restore_clip = background->clip_active;
  CGRect clip_rect = background->clip_rect;
  CGSize viewport = background->viewport_size;
  float scale = background->scale;
  int32_t result = finish_darwin_renderer_frame(window, background, 0);
  if (result != 0) {
    return result;
  }
  result = begin_darwin_renderer_frame(window, window->overlay_renderer, background->frame_id, viewport.width, viewport.height, scale, 0.0f, 0.0f, 0.0f, 0.0f, 0, 0, 0, 0);
  if (result != 0) {
    return result;
  }
  window->active_renderer = window->overlay_renderer;
  window->embedded_surface_overlay_active = true;
  if (restore_clip) {
    return wox_darwin_window_set_clip_rect(window, clip_rect.origin.x, clip_rect.origin.y, clip_rect.size.width, clip_rect.size.height);
  }
  return 0;
}

int32_t wox_darwin_window_end_frame(WoxDarwinWindow *window, int32_t transactional) {
  if (window == NULL || window->active_renderer == NULL) {
    return -1;
  }
  int32_t result = finish_darwin_renderer_frame(window, window->active_renderer, transactional);
  if (result != 0 || window->embedded_surface_overlay_active) {
    window->active_renderer = NULL;
    return result;
  }
  WoxDarwinRenderer *background = window->renderer;
  result = begin_darwin_renderer_frame(window, window->overlay_renderer, background->frame_id, background->viewport_size.width, background->viewport_size.height, background->scale, 0.0f, 0.0f, 0.0f, 0.0f, 0, 0, 0, 0);
  if (result == 0) {
    result = finish_darwin_renderer_frame(window, window->overlay_renderer, transactional);
  }
  window->active_renderer = NULL;
  return result;
}

int32_t wox_darwin_window_take_frame_resource_stats(WoxDarwinWindow *window, WoxRendererResourceStats *out) {
  if (window == NULL || out == NULL) {
    return -1;
  }
  *out = window->frame_resource_stats;
  out->resident_bytes = (int64_t)(window->cached_image_bytes + window->cached_large_image_bytes);
  return 0;
}

// wox_darwin_test_large_image_admission proves the single large-image slot is bound to one
// candidate, so an alternating oversized image cannot inherit an earlier image's admission.
int32_t wox_darwin_test_large_image_admission(void) {
  WoxDarwinWindow window;
  memset(&window, 0, sizeof(window));
  note_large_image_repeat(&window, 99);
  note_large_image_repeat(&window, 99);
  if (window.cache_large_images) {
    return -1;
  }
  note_large_image_repeat(&window, 99);
  if (!window.cache_large_images) {
    return -1;
  }
  note_large_image_repeat(&window, 100);
  if (window.cache_large_images || window.large_image_candidate_frames != 1) {
    return -1;
  }
  return 0;
}

// wox_darwin_test_cached_image_owns_pixels proves a cached CGImage keeps a native pixel copy.
int32_t wox_darwin_test_cached_image_owns_pixels(void) {
  uint8_t pixels[16] = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16};
  CGImageRef image = create_cgimage_from_pixels(pixels, 2, 2, 8, true);
  if (image == NULL) {
    return -1;
  }
  memset(pixels, 0xFF, sizeof(pixels));
  CGDataProviderRef provider = CGImageGetDataProvider(image);
  CFDataRef data = CGDataProviderCopyData(provider);
  if (data == NULL) {
    CGImageRelease(image);
    return -1;
  }
  const uint8_t *cached = CFDataGetBytePtr(data);
  int32_t result = cached != NULL && cached[0] == 1 && cached[1] == 2 && cached[2] == 3 && cached[3] == 4 ? 0 : -1;
  CFRelease(data);
  CGImageRelease(image);
  return result;
}
