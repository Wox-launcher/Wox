//go:build darwin

#import "native_darwin.h"

#import <Cocoa/Cocoa.h>
#import <CoreText/CoreText.h>
#import <CoreVideo/CoreVideo.h>
#import <IOSurface/IOSurface.h>
#import <QuartzCore/CALayer.h>
#import <QuartzCore/CATransaction.h>
#import <WebKit/WebKit.h>
#import <dispatch/dispatch.h>

#include <dlfcn.h>
#include <math.h>
#include <stdbool.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>

extern int32_t woxGoDarwinStart(uintptr_t context);
extern void woxGoDarwinCloseRequested(uintptr_t context);
extern void woxGoDarwinCall(uintptr_t context);
extern void woxGoDarwinFrame(uintptr_t context, float width, float height, int32_t pixel_width, int32_t pixel_height, float scale);
extern void woxGoDarwinFrameSync(uintptr_t context, float width, float height, int32_t pixel_width, int32_t pixel_height, float scale, int32_t transactional);
extern void woxGoDarwinFocus(uintptr_t context, uint64_t epoch, int32_t active);
extern int32_t woxGoDarwinKey(uintptr_t context, const char *key, uint8_t modifiers, int32_t down, int32_t repeat, int32_t composing);
extern void woxGoDarwinTextInput(uintptr_t context, uint8_t kind, const char *text);
extern void woxGoDarwinPointer(uintptr_t context, uint8_t kind, float x, float y, uint8_t button, float scroll_x, float scroll_y, uint8_t modifiers);
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
};

typedef struct WoxDarwinRenderer WoxDarwinRenderer;
@class WoxDarwinSurface;
@class WoxRenderView;
@class WoxWindowDelegate;

struct WoxDarwinWindow {
  NSWindow *window;
  WoxRenderView *view;
  WoxWindowDelegate *delegate;
  WoxDarwinRenderer *renderer;
  NSMutableDictionary *web_view_cache;
  NSMutableDictionary *web_view_signatures;
  NSMutableDictionary *web_view_content_keys;
  WKWebView *active_web_view;
  NSString *active_web_view_key;
  NSString *active_web_view_signature;
  NSString *active_web_view_content_key;
  bool active_web_view_transient;
  uintptr_t context;
  uint64_t epoch;
  bool visible;
  bool active;
  bool hide_on_blur;
  bool application_window;
  bool native_dialog_active;
  bool input_enabled;
  bool closed;
  bool render_scheduled;
  bool suppress_resize_render;
  bool synchronous_frame;
  atomic_uint_fast64_t presentation_generation;
  NSRect input_cursor_rect;
  NSMutableDictionary *accessibility_elements;
  NSMutableDictionary *accessibility_child_ids;
  NSMutableArray *accessibility_roots;
  uint64_t accessibility_generation;
};

struct WoxDarwinRenderer {
  CALayer *layer;
  CALayer *content_layer;
  NSMutableArray *render_surfaces;
  WoxDarwinSurface *frame_surface;
  WoxDarwinSurface *front_surface;
  CGContextRef context;
  CGSize viewport_size;
  float scale;
  uint64_t frame_generation;
  uint64_t submission_sequence;
  uint64_t presented_sequence;
  bool frame_open;
  bool clip_active;
};

static NSInteger wox_open_window_count = 0;
static NSInteger wox_application_window_count = 0;

@interface WoxNativeWindow : NSWindow
@end

@implementation WoxNativeWindow
- (BOOL)canBecomeKeyWindow {
  return YES;
}

- (BOOL)canBecomeMainWindow {
  return YES;
}
@end

@interface WoxDarwinSurface : NSObject {
@public
  IOSurfaceRef io_surface;
  NSUInteger width;
  NSUInteger height;
  atomic_uint presentation_references;
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

@interface WoxRenderView : NSView <NSTextInputClient> {
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
    return nil;
  }
  WoxDarwinSurface *surface = [[WoxDarwinSurface alloc] initWithWidth:width height:height];
  if (surface != nil) {
    [renderer->render_surfaces addObject:surface];
  }
  return surface;
}

// present_render_surface rejects delayed frames after hide, resize, or a newer submission.
static void present_render_surface(WoxDarwinWindow *window, WoxDarwinRenderer *renderer, WoxDarwinSurface *surface, uint64_t sequence, uint64_t generation) {
  if (window->closed || !window->visible || window->renderer != renderer ||
      atomic_load_explicit(&window->presentation_generation, memory_order_relaxed) != generation ||
      sequence <= renderer->presented_sequence) {
    atomic_fetch_sub_explicit(&surface->presentation_references, 1, memory_order_relaxed);
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
  [CATransaction begin];
  [CATransaction setDisableActions:YES];
  renderer->content_layer.frame = renderer->layer.bounds;
  renderer->content_layer.contentsScale = renderer->layer.contentsScale;
  renderer->content_layer.contents = (__bridge id)surface->io_surface;
  [CATransaction commit];
}

static void destroy_renderer(WoxDarwinRenderer *renderer) {
  if (renderer == NULL) {
    return;
  }
  if (renderer->frame_open) {
    if (renderer->clip_active) {
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
                       @"(()=>{const c=%@[0];let s=document.getElementById('wox-webview-preview-style');"
                        "if(!s){s=document.createElement('style');s.id='wox-webview-preview-style';"
                        "(document.head||document.documentElement).appendChild(s)}s.textContent=c})()",
                       json];
}

@interface WoxWebViewMessageHandler : NSObject <WKScriptMessageHandler> {
@public
  WoxDarwinWindow *_owner;
}
@end

@implementation WoxWebViewMessageHandler
- (void)userContentController:(WKUserContentController *)userContentController didReceiveScriptMessage:(WKScriptMessage *)message {
  (void)userContentController;
  WoxDarwinWindow *owner = _owner;
  if (![message.name isEqualToString:@"woxWebViewPreview"] || owner == NULL || owner->closed || owner->context == 0) {
    return;
  }
  woxGoDarwinKey(owner->context, "escape", 0, 1, 0, 0);
}
@end

static NSString *web_view_escape_script(void) {
  return @"(()=>{if(window.__woxUnhandledEscapeInstalled__)return;window.__woxUnhandledEscapeInstalled__=true;"
          "document.addEventListener('keydown',e=>{if(e.key!=='Escape'||e.repeat)return;setTimeout(()=>{"
          "if(e.defaultPrevented||e.cancelBubble)return;window.webkit.messageHandlers.woxWebViewPreview.postMessage('escape')},0)},true)})()";
}

static WKWebView *create_web_view(WoxDarwinWindow *window, NSString *inject_css) {
  WKWebViewConfiguration *configuration = [[[WKWebViewConfiguration alloc] init] autorelease];
  configuration.websiteDataStore = [WKWebsiteDataStore defaultDataStore];
  WoxWebViewMessageHandler *message_handler = [[WoxWebViewMessageHandler alloc] init];
  message_handler->_owner = window;
  [configuration.userContentController addScriptMessageHandler:message_handler name:@"woxWebViewPreview"];
  [message_handler release];
  WKUserScript *escape_script = [[[WKUserScript alloc] initWithSource:web_view_escape_script() injectionTime:WKUserScriptInjectionTimeAtDocumentStart forMainFrameOnly:YES] autorelease];
  [configuration.userContentController addUserScript:escape_script];
  NSString *script = web_view_css_script(inject_css);
  if (script != nil) {
    WKUserScript *user_script = [[[WKUserScript alloc] initWithSource:script injectionTime:WKUserScriptInjectionTimeAtDocumentEnd forMainFrameOnly:YES] autorelease];
    [configuration.userContentController addUserScript:user_script];
  }
  WKWebView *web_view = [[WKWebView alloc] initWithFrame:NSZeroRect configuration:configuration];
  web_view.autoresizingMask = NSViewNotSizable;
  web_view.customUserAgent = @"Mozilla/5.0 (iPhone; CPU iPhone OS 18_7_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1";
  if (@available(macOS 13.3, *)) {
    web_view.inspectable = YES;
  }
  return web_view;
}

static void clear_active_web_view(WoxDarwinWindow *window, bool discard_transient) {
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
  if (owner == NULL || owner->closed || owner->context == 0) {
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
    owner->visible = false;
    atomic_fetch_add_explicit(&owner->presentation_generation, 1, memory_order_relaxed);
    [owner->window orderOut:nil];
    owner->renderer->content_layer.contents = nil;
    if (owner->renderer->front_surface != nil) {
      atomic_fetch_sub_explicit(&owner->renderer->front_surface->presentation_references, 1, memory_order_relaxed);
      owner->renderer->front_surface = nil;
    }
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
    [application finishLaunching];
    if (woxGoDarwinStart(context) != 0) {
      return -1;
    }
    if (wox_open_window_count == 0) {
      return 0;
    }
    [application run];
  }
  return 0;
}

WoxDarwinWindow *wox_darwin_window_create(const char *title, float width, float height, int32_t hide_on_blur, int32_t window_role, uintptr_t context) {
  if (![NSThread isMainThread] || width <= 0.0f || height <= 0.0f || context == 0) {
    return NULL;
  }

  @autoreleasepool {
    NSRect frame = NSMakeRect(0.0, 0.0, width, height);
    bool is_application_window = window_role == 1;
    bool is_screenshot_window = window_role == 2;
    NSWindowStyleMask style_mask = NSWindowStyleMaskBorderless;
    if (is_application_window) {
      style_mask = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskFullSizeContentView;
    }
    WoxNativeWindow *native_window = [[WoxNativeWindow alloc]
        initWithContentRect:frame
                  styleMask:style_mask
                    backing:NSBackingStoreBuffered
                      defer:NO];
    native_window.releasedWhenClosed = NO;
    native_window.opaque = NO;
    native_window.backgroundColor = [NSColor clearColor];
    native_window.hasShadow = !is_screenshot_window;
    native_window.acceptsMouseMovedEvents = YES;
    // Management windows participate in normal app switching while launcher surfaces remain cross-space utilities.
    if (is_application_window) {
      native_window.level = NSNormalWindowLevel;
      native_window.collectionBehavior = NSWindowCollectionBehaviorDefault;
      native_window.titlebarAppearsTransparent = YES;
      native_window.titleVisibility = NSWindowTitleHidden;
      [[native_window standardWindowButton:NSWindowCloseButton] setHidden:YES];
      [[native_window standardWindowButton:NSWindowMiniaturizeButton] setHidden:YES];
      [[native_window standardWindowButton:NSWindowZoomButton] setHidden:YES];
    } else if (is_screenshot_window) {
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

    WoxWindowDelegate *delegate = [[WoxWindowDelegate alloc] init];
    delegate->_owner = window;
    window->window = native_window;
    window->view = view;
    window->delegate = delegate;
    window->renderer = renderer;
    window->web_view_cache = [[NSMutableDictionary alloc] init];
    window->web_view_signatures = [[NSMutableDictionary alloc] init];
    window->web_view_content_keys = [[NSMutableDictionary alloc] init];
    window->context = context;
    window->hide_on_blur = hide_on_blur != 0;
    window->application_window = is_application_window;
    // Use launcher material instead of compositing the transparent UI surface directly over the desktop.
    NSVisualEffectView *effect_view = [[NSVisualEffectView alloc] initWithFrame:frame];
    effect_view.material = NSVisualEffectMaterialPopover;
    effect_view.state = NSVisualEffectStateActive;
    effect_view.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    effect_view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    effect_view.wantsLayer = YES;
    effect_view.layer.cornerRadius = is_screenshot_window ? 0.0 : 14.0;
    effect_view.layer.masksToBounds = YES;
    [effect_view addSubview:view];
    native_window.contentView = effect_view;
    [effect_view release];
    native_window.delegate = delegate;
    [native_window center];
    [view updateBackingScale];
    wox_open_window_count++;
    if (window->application_window) {
      wox_application_window_count++;
      [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    }
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
      emit_focus(window, false);
      if (window->closed) {
        return;
      }
    }
    window->epoch++;
    atomic_fetch_add_explicit(&window->presentation_generation, 1, memory_order_relaxed);
    epoch = window->epoch;
    window->visible = true;
    [window->view updateBackingScale];
    [NSApp activateIgnoringOtherApps:YES];
    if (window->window.isMiniaturized) {
      [window->window deminiaturize:nil];
    }
    [window->window makeKeyAndOrderFront:nil];
    [window->window makeFirstResponder:window->view];
    if (!window->closed && window->window.isKeyWindow) {
      emit_focus(window, true);
    }
    if (!window->closed) {
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
    emit_focus(window, false);
    if (!window->closed) {
      window->visible = false;
      atomic_fetch_add_explicit(&window->presentation_generation, 1, memory_order_relaxed);
      [window->window orderOut:nil];
      clear_renderer_surfaces(window->renderer);
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

int32_t wox_darwin_capture_desktop_png(const char *path, float *x, float *y, float *width, float *height) {
  if (path == NULL || path[0] == '\0' || x == NULL || y == NULL || width == NULL || height == NULL) {
    return -1;
  }
  __block int32_t result = 0;
  run_on_main_sync(^{
    if (@available(macOS 10.15, *)) {
      if (!CGPreflightScreenCaptureAccess()) {
        result = -2;
        return;
      }
    }
    NSArray<NSScreen *> *screens = [NSScreen screens];
    if (screens.count == 0) {
      result = -1;
      return;
    }
    CGFloat top = desktop_top();
    NSRect desktop_bounds = NSZeroRect;
    for (NSScreen *screen in screens) {
      NSRect frame = screen.frame;
      NSRect logical_frame = NSMakeRect(NSMinX(frame), top - NSMaxY(frame), NSWidth(frame), NSHeight(frame));
      desktop_bounds = NSIsEmptyRect(desktop_bounds) ? logical_frame : NSUnionRect(desktop_bounds, logical_frame);
    }

    typedef CGImageRef (*WoxDesktopCaptureFunction)(CGRect, CGWindowListOption, CGWindowID, CGWindowImageOption);
    WoxDesktopCaptureFunction capture = (WoxDesktopCaptureFunction)dlsym(RTLD_DEFAULT, "CGWindowListCreateImage");
    if (capture == NULL) {
      result = -1;
      return;
    }
    CGImageRef image = capture(CGRectInfinite, kCGWindowListOptionOnScreenOnly, kCGNullWindowID, kCGWindowImageBestResolution);
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
    } else {
      *x = (float)NSMinX(desktop_bounds);
      *y = (float)NSMinY(desktop_bounds);
      *width = (float)NSWidth(desktop_bounds);
      *height = (float)NSHeight(desktop_bounds);
    }
    [representation release];
  });
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

int32_t wox_darwin_window_show_webview(WoxDarwinWindow *window, const char *url, const char *html, const char *inject_css, int32_t cache_disabled, const char *cache_key, float x, float y, float width, float height) {
  if (window == NULL || url == NULL || html == NULL || inject_css == NULL || cache_key == NULL || width <= 0.0f || height <= 0.0f) {
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
    NSString *key_value = web_view_string(cache_key);
    bool use_cache = cache_disabled == 0 && key_value.length > 0;
    NSString *signature = css_value;
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
        web_view = create_web_view(window, css_value);
        [window->web_view_cache setObject:web_view forKey:key_value];
        [window->web_view_signatures setObject:signature forKey:key_value];
        [web_view release];
      }
      [window->web_view_content_keys setObject:content_key forKey:key_value];
    } else if (window->active_web_view_transient && [window->active_web_view_signature isEqualToString:signature] && [window->active_web_view_content_key isEqualToString:content_key]) {
      web_view = window->active_web_view;
      should_load = false;
    } else {
      web_view = create_web_view(window, css_value);
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
    web_view.hidden = NO;

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
  if ([value isEqualToString:@"web_view"]) return NSAccessibilityWebAreaRole;
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
    window->context = 0;
    if (was_active && context != 0) {
      woxGoDarwinFocus(context, epoch, 0);
    }

    window->view->_owner = NULL;
    window->delegate->_owner = NULL;
    clear_active_web_view(window, true);
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
    destroy_renderer(window->renderer);
    window->renderer = NULL;
    [window->delegate autorelease];
    [window->view autorelease];
    [window->window autorelease];
    window->delegate = nil;
    window->view = nil;
    window->window = nil;

    if (wox_open_window_count > 0) {
      wox_open_window_count--;
    }
    if (window->application_window && wox_application_window_count > 0) {
      wox_application_window_count--;
      if (wox_application_window_count == 0) {
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
      }
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

int32_t wox_darwin_window_begin_frame(WoxDarwinWindow *window, float logical_width, float logical_height, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->closed || window->renderer == NULL || logical_width <= 0.0f || logical_height <= 0.0f || scale <= 0.0f) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
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
    return 1;
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

  // On M3, the first Metal render pass reserves about 200 MB of driver memory; drawing the same IOSurface on the CPU avoids that fixed visible-state cost.
  CGContextSetBlendMode(context, kCGBlendModeCopy);
  CGContextSetRGBFillColor(context, red / 255.0, green / 255.0, blue / 255.0, alpha / 255.0);
  CGContextFillRect(context, CGRectMake(0.0, 0.0, pixel_width, pixel_height));
  CGContextSetBlendMode(context, kCGBlendModeNormal);
  CGContextTranslateCTM(context, 0.0, pixel_height);
  CGContextScaleCTM(context, scale, -scale);
  CGContextSetShouldAntialias(context, true);
  CGContextSetInterpolationQuality(context, kCGInterpolationHigh);

  renderer->frame_surface = surface;
  renderer->context = context;
  renderer->viewport_size = CGSizeMake(logical_width, logical_height);
  renderer->scale = scale;
  renderer->frame_generation = atomic_load_explicit(&window->presentation_generation, memory_order_relaxed);
  renderer->frame_open = true;
  renderer->clip_active = false;
  return 0;
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
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f) {
    return 0;
  }

  WoxDarwinRenderer *renderer = window->renderer;
  CGFloat clamped_radius = fmaxf(0.0f, fminf(radius, fminf(width, height) / 2.0f));
  CGPathRef path = CGPathCreateWithRoundedRect(CGRectMake(x, y, width, height), clamped_radius, clamped_radius, NULL);
  CGContextSetRGBFillColor(renderer->context, red / 255.0, green / 255.0, blue / 255.0, alpha / 255.0);
  CGContextAddPath(renderer->context, path);
  CGContextFillPath(renderer->context);
  CGPathRelease(path);
  return 0;
}

int32_t wox_darwin_window_fill_convex_polygon(WoxDarwinWindow *window, const float *points, int32_t point_count, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  if (points == NULL || point_count < 3 || point_count > 16) {
    return -1;
  }

  WoxDarwinRenderer *renderer = window->renderer;
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
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f || stroke_width <= 0.0f) {
    return 0;
  }

  WoxDarwinRenderer *renderer = window->renderer;
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
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open || text == NULL) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
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
  return 0;
}

int32_t wox_darwin_window_draw_image(WoxDarwinWindow *window, uint64_t image_id, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open || image_id == 0 || pixels == NULL || image_width <= 0 || image_height <= 0 || row_stride < image_width * 4 || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  size_t data_size = (size_t)row_stride * (size_t)image_height;
  CGDataProviderRef provider = CGDataProviderCreateWithData(NULL, pixels, data_size, NULL);
  if (provider == NULL) {
    return -1;
  }
  CGColorSpaceRef color_space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
  if (color_space == NULL) {
    CGDataProviderRelease(provider);
    return -1;
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
  if (image == NULL) {
    return -1;
  }

  CGContextSaveGState(renderer->context);
  CGContextTranslateCTM(renderer->context, x, y + height);
  CGContextScaleCTM(renderer->context, 1.0, -1.0);
  CGContextDrawImage(renderer->context, CGRectMake(0.0, 0.0, width, height), image);
  CGContextRestoreGState(renderer->context);
  CGImageRelease(image);
  return 0;
}

int32_t wox_darwin_window_set_clip_rect(WoxDarwinWindow *window, float x, float y, float width, float height) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
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
  return 0;
}

int32_t wox_darwin_window_clear_clip(WoxDarwinWindow *window) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  if (renderer->clip_active) {
    CGContextRestoreGState(renderer->context);
    renderer->clip_active = false;
  }
  return 0;
}

int32_t wox_darwin_window_end_frame(WoxDarwinWindow *window, int32_t transactional) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  if (renderer->clip_active) {
    CGContextRestoreGState(renderer->context);
    renderer->clip_active = false;
  }
  CGContextFlush(renderer->context);
  CGContextRelease(renderer->context);
  renderer->context = NULL;
  WoxDarwinSurface *surface = renderer->frame_surface;
  IOSurfaceUnlock(surface->io_surface, 0, NULL);
  renderer->frame_surface = nil;
  renderer->frame_open = false;

  uint64_t sequence = ++renderer->submission_sequence;
  uint64_t generation = renderer->frame_generation;
  atomic_fetch_add_explicit(&surface->presentation_references, 1, memory_order_relaxed);
  if ([NSThread isMainThread]) {
    present_render_surface(window, renderer, surface, sequence, generation);
  } else {
    WoxDarwinSurface *present_surface = [surface retain];
    dispatch_async(dispatch_get_main_queue(), ^{
      present_render_surface(window, renderer, present_surface, sequence, generation);
      [present_surface release];
    });
  }
  [surface release];
  (void)transactional;
  return 0;
}
