//go:build darwin

#import "native_darwin.h"

#import <Cocoa/Cocoa.h>
#import <CoreText/CoreText.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#import <WebKit/WebKit.h>
#import <dispatch/dispatch.h>
#import <simd/simd.h>

#include <math.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

extern int32_t woxGoDarwinStart(uintptr_t context);
extern void woxGoDarwinCall(uintptr_t context);
extern void woxGoDarwinFrame(uintptr_t context, float width, float height, int32_t pixel_width, int32_t pixel_height, float scale);
extern void woxGoDarwinFocus(uintptr_t context, uint64_t epoch, int32_t active);
extern int32_t woxGoDarwinKey(uintptr_t context, const char *key, uint8_t modifiers, int32_t down, int32_t repeat, int32_t composing);
extern void woxGoDarwinTextInput(uintptr_t context, uint8_t kind, const char *text);
extern void woxGoDarwinPointer(uintptr_t context, uint8_t kind, float x, float y, uint8_t button, float scroll_x, float scroll_y, uint8_t modifiers);

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
};

typedef struct WoxDarwinRenderer WoxDarwinRenderer;
@class WoxMetalView;
@class WoxWindowDelegate;

struct WoxDarwinWindow {
  NSWindow *window;
  WoxMetalView *view;
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
  bool native_dialog_active;
  bool input_enabled;
  bool closed;
  NSRect input_cursor_rect;
};

struct WoxDarwinRenderer {
  id<MTLDevice> device;
  id<MTLCommandQueue> queue;
  id<MTLRenderPipelineState> rect_pipeline;
  id<MTLRenderPipelineState> texture_pipeline;
  CAMetalLayer *layer;
  id<CAMetalDrawable> drawable;
  id<MTLCommandBuffer> command_buffer;
  id<MTLRenderCommandEncoder> encoder;
  vector_float2 viewport_size;
  float scale;
  bool frame_open;
};

typedef struct {
  vector_float2 viewport_size;
  vector_float4 rect;
  vector_float4 color;
  float radius;
  float stroke_width;
} WoxRectUniforms;

typedef struct {
  vector_float2 viewport_size;
  vector_float4 rect;
  vector_float4 color;
} WoxTextureUniforms;

static NSInteger wox_open_window_count = 0;

static const char *const wox_metal_source =
    "#include <metal_stdlib>\n"
     "using namespace metal;\n"
     "struct RectUniforms {\n"
     "  float2 viewport_size;\n"
     "  float4 rect;\n"
     "  float4 color;\n"
     "  float radius;\n"
     "  float stroke_width;\n"
     "};\n"
     "struct TextureUniforms {\n"
     "  float2 viewport_size;\n"
     "  float4 rect;\n"
     "  float4 color;\n"
     "};\n"
     "struct VertexOut {\n"
     "  float4 position [[position]];\n"
     "  float2 local;\n"
     "};\n"
     "vertex VertexOut rect_vertex(uint vertex_id [[vertex_id]], constant RectUniforms &uniforms [[buffer(0)]]) {\n"
     "  const float2 corners[4] = {float2(0.0, 0.0), float2(1.0, 0.0), float2(0.0, 1.0), float2(1.0, 1.0)};\n"
     "  float2 corner = corners[vertex_id];\n"
     "  float2 point = uniforms.rect.xy + corner * uniforms.rect.zw;\n"
     "  VertexOut output;\n"
     "  output.position = float4(point.x / uniforms.viewport_size.x * 2.0 - 1.0, 1.0 - point.y / uniforms.viewport_size.y * 2.0, 0.0, 1.0);\n"
     "  output.local = corner * uniforms.rect.zw;\n"
     "  return output;\n"
     "}\n"
     "fragment float4 rect_fragment(VertexOut input [[stage_in]], constant RectUniforms &uniforms [[buffer(0)]]) {\n"
     "  float radius = clamp(uniforms.radius, 0.0, min(uniforms.rect.z, uniforms.rect.w) * 0.5);\n"
     "  float2 half_size = uniforms.rect.zw * 0.5;\n"
     "  float2 edge = abs(input.local - half_size) - (half_size - radius);\n"
     "  float distance = length(max(edge, float2(0.0))) + min(max(edge.x, edge.y), 0.0) - radius;\n"
     "  float antialias = max(fwidth(distance), 0.001);\n"
     "  float outer_coverage = 1.0 - smoothstep(-antialias * 0.5, antialias * 0.5, distance);\n"
     "  if (uniforms.stroke_width <= 0.0) { return uniforms.color * outer_coverage; }\n"
     "  float inner_radius = max(radius - uniforms.stroke_width, 0.0);\n"
     "  float2 inner_half = max(half_size - uniforms.stroke_width, float2(0.0));\n"
     "  float2 inner_edge = abs(input.local - half_size) - max(inner_half - inner_radius, float2(0.0));\n"
     "  float inner_distance = length(max(inner_edge, float2(0.0))) + min(max(inner_edge.x, inner_edge.y), 0.0) - inner_radius;\n"
     "  float inner_antialias = max(fwidth(inner_distance), 0.001);\n"
     "  float inner_coverage = 1.0 - smoothstep(-inner_antialias * 0.5, inner_antialias * 0.5, inner_distance);\n"
     "  float coverage = clamp(outer_coverage - inner_coverage, 0.0, 1.0);\n"
     "  return uniforms.color * coverage;\n"
     "}\n"
     "struct TextureVertexOut {\n"
     "  float4 position [[position]];\n"
     "  float2 uv;\n"
     "};\n"
     "vertex TextureVertexOut texture_vertex(uint vertex_id [[vertex_id]], constant TextureUniforms &uniforms [[buffer(0)]]) {\n"
     "  const float2 corners[4] = {float2(0.0, 0.0), float2(1.0, 0.0), float2(0.0, 1.0), float2(1.0, 1.0)};\n"
     "  float2 corner = corners[vertex_id];\n"
     "  float2 point = uniforms.rect.xy + corner * uniforms.rect.zw;\n"
     "  TextureVertexOut output;\n"
     "  output.position = float4(point.x / uniforms.viewport_size.x * 2.0 - 1.0, 1.0 - point.y / uniforms.viewport_size.y * 2.0, 0.0, 1.0);\n"
     "  output.uv = corner;\n"
     "  return output;\n"
     "}\n"
     "fragment float4 texture_fragment(TextureVertexOut input [[stage_in]], texture2d<float> image [[texture(0)]], constant TextureUniforms &uniforms [[buffer(0)]]) {\n"
     "  constexpr sampler texture_sampler(address::clamp_to_zero, filter::linear);\n"
     "  return image.sample(texture_sampler, input.uv) * uniforms.color;\n"
     "}\n";

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

@interface WoxMetalView : NSView <NSTextInputClient> {
@public
  WoxDarwinWindow *_owner;
  NSString *_marked_text;
  NSRange _marked_selection;
  NSTrackingArea *_tracking_area;
}
- (void)updateDrawableSize;
- (void)renderFrame;
@end

@interface WoxWindowDelegate : NSObject <NSWindowDelegate> {
@public
  WoxDarwinWindow *_owner;
}
@end

// run_on_main_sync serializes all AppKit access while allowing UI callbacks to reenter directly.
static void run_on_main_sync(dispatch_block_t block) {
  if ([NSThread isMainThread]) {
    block();
  } else {
    dispatch_sync(dispatch_get_main_queue(), block);
  }
}

static vector_float4 premultiplied_color(uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  float a = (float)alpha / 255.0f;
  return (vector_float4){(float)red / 255.0f * a, (float)green / 255.0f * a, (float)blue / 255.0f * a, a};
}

static void configure_blend(MTLRenderPipelineColorAttachmentDescriptor *attachment) {
  attachment.blendingEnabled = YES;
  attachment.rgbBlendOperation = MTLBlendOperationAdd;
  attachment.alphaBlendOperation = MTLBlendOperationAdd;
  attachment.sourceRGBBlendFactor = MTLBlendFactorOne;
  attachment.sourceAlphaBlendFactor = MTLBlendFactorOne;
  attachment.destinationRGBBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
  attachment.destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
}

// create_renderer builds the two tiny pipelines used by the backend proof.
static WoxDarwinRenderer *create_renderer(CAMetalLayer *layer) {
  id<MTLDevice> device = MTLCreateSystemDefaultDevice();
  if (device == nil) {
    NSLog(@"Wox Go UI: Metal is unavailable");
    return NULL;
  }

  WoxDarwinRenderer *renderer = calloc(1, sizeof(WoxDarwinRenderer));
  renderer->device = [device retain];
  renderer->queue = [device newCommandQueue];
  renderer->layer = layer;
  renderer->scale = 1.0f;
  layer.device = device;

  NSError *error = nil;
  NSString *metal_source = [NSString stringWithUTF8String:wox_metal_source];
  id<MTLLibrary> library = [device newLibraryWithSource:metal_source options:nil error:&error];
  if (library == nil) {
    NSLog(@"Wox Go UI: Metal shader compilation failed: %@", error);
    [renderer->queue release];
    [renderer->device release];
    free(renderer);
    return NULL;
  }

  id<MTLFunction> rect_vertex = [library newFunctionWithName:@"rect_vertex"];
  id<MTLFunction> rect_fragment = [library newFunctionWithName:@"rect_fragment"];
  id<MTLFunction> texture_vertex = [library newFunctionWithName:@"texture_vertex"];
  id<MTLFunction> texture_fragment = [library newFunctionWithName:@"texture_fragment"];

  MTLRenderPipelineDescriptor *descriptor = [[MTLRenderPipelineDescriptor alloc] init];
  descriptor.vertexFunction = rect_vertex;
  descriptor.fragmentFunction = rect_fragment;
  descriptor.colorAttachments[0].pixelFormat = layer.pixelFormat;
  configure_blend(descriptor.colorAttachments[0]);
  renderer->rect_pipeline = [device newRenderPipelineStateWithDescriptor:descriptor error:&error];

  descriptor.vertexFunction = texture_vertex;
  descriptor.fragmentFunction = texture_fragment;
  renderer->texture_pipeline = [device newRenderPipelineStateWithDescriptor:descriptor error:&error];
  if (renderer->rect_pipeline == nil || renderer->texture_pipeline == nil) {
    NSLog(@"Wox Go UI: Metal pipeline creation failed: %@", error);
  }

  [descriptor release];
  [rect_vertex release];
  [rect_fragment release];
  [texture_vertex release];
  [texture_fragment release];
  [library release];

  if (renderer->rect_pipeline == nil || renderer->texture_pipeline == nil) {
    [renderer->texture_pipeline release];
    [renderer->rect_pipeline release];
    [renderer->queue release];
    [renderer->device release];
    free(renderer);
    return NULL;
  }
  return renderer;
}

static void destroy_renderer(WoxDarwinRenderer *renderer) {
  if (renderer == NULL) {
    return;
  }
  if (renderer->frame_open) {
    [renderer->encoder endEncoding];
  }
  [renderer->encoder release];
  [renderer->command_buffer release];
  [renderer->drawable release];
  [renderer->texture_pipeline release];
  [renderer->rect_pipeline release];
  [renderer->queue release];
  [renderer->device release];
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

@implementation WoxMetalView
- (CALayer *)makeBackingLayer {
  CAMetalLayer *layer = [CAMetalLayer layer];
  layer.pixelFormat = MTLPixelFormatBGRA8Unorm;
  layer.framebufferOnly = YES;
  layer.opaque = NO;
  layer.needsDisplayOnBoundsChange = YES;
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

- (void)updateDrawableSize {
  if (_owner == NULL || _owner->closed || self.window == nil) {
    return;
  }
  CGFloat scale = self.window.backingScaleFactor;
  NSSize size = self.bounds.size;
  CAMetalLayer *layer = (CAMetalLayer *)self.layer;
  layer.contentsScale = scale;
  layer.drawableSize = CGSizeMake(ceil(size.width * scale), ceil(size.height * scale));
}

- (void)viewDidMoveToWindow {
  [super viewDidMoveToWindow];
  [self updateDrawableSize];
}

- (void)viewDidChangeBackingProperties {
  [super viewDidChangeBackingProperties];
  [self updateDrawableSize];
  [self renderFrame];
}

- (void)setFrameSize:(NSSize)newSize {
  [super setFrameSize:newSize];
  [self updateDrawableSize];
  [self renderFrame];
}

- (void)updateLayer {
  [self renderFrame];
}

- (void)renderFrame {
  WoxDarwinWindow *owner = _owner;
  if (owner == NULL || owner->closed || !owner->visible || owner->context == 0) {
    return;
  }
  [self updateDrawableSize];
  NSSize size = self.bounds.size;
  CGFloat scale = self.window.backingScaleFactor;
  int32_t pixel_width = (int32_t)ceil(size.width * scale);
  int32_t pixel_height = (int32_t)ceil(size.height * scale);
  if (size.width > 0.0 && size.height > 0.0 && pixel_width > 0 && pixel_height > 0) {
    woxGoDarwinFrame(owner->context, (float)size.width, (float)size.height, pixel_width, pixel_height, (float)scale);
  }
}
@end

@implementation WoxWindowDelegate
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
    [owner->window orderOut:nil];
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

WoxDarwinWindow *wox_darwin_window_create(const char *title, float width, float height, int32_t hide_on_blur, uintptr_t context) {
  if (![NSThread isMainThread] || width <= 0.0f || height <= 0.0f || context == 0) {
    return NULL;
  }

  @autoreleasepool {
    NSRect frame = NSMakeRect(0.0, 0.0, width, height);
    WoxNativeWindow *native_window = [[WoxNativeWindow alloc]
        initWithContentRect:frame
                  styleMask:NSWindowStyleMaskBorderless
                    backing:NSBackingStoreBuffered
                      defer:NO];
    native_window.releasedWhenClosed = NO;
    native_window.opaque = NO;
    native_window.backgroundColor = [NSColor clearColor];
    native_window.hasShadow = YES;
    native_window.acceptsMouseMovedEvents = YES;
    native_window.level = NSFloatingWindowLevel;
    native_window.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
    if (title != NULL) {
      NSString *window_title = [NSString stringWithUTF8String:title];
      if (window_title != nil) {
        native_window.title = window_title;
      }
    }

    WoxDarwinWindow *window = calloc(1, sizeof(WoxDarwinWindow));
    WoxMetalView *view = [[WoxMetalView alloc] initWithFrame:frame];
    view->_owner = window;
    view->_marked_selection = NSMakeRange(NSNotFound, 0);
    view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    view.wantsLayer = YES;
    CAMetalLayer *layer = (CAMetalLayer *)view.layer;

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
    // Match Flutter's launcher material instead of compositing the transparent Metal surface directly over the desktop.
    NSVisualEffectView *effect_view = [[NSVisualEffectView alloc] initWithFrame:frame];
    effect_view.material = NSVisualEffectMaterialPopover;
    effect_view.state = NSVisualEffectStateActive;
    effect_view.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    effect_view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    effect_view.wantsLayer = YES;
    effect_view.layer.cornerRadius = 14.0;
    effect_view.layer.masksToBounds = YES;
    [effect_view addSubview:view];
    native_window.contentView = effect_view;
    [effect_view release];
    native_window.delegate = delegate;
    [native_window center];
    [view updateDrawableSize];
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
      emit_focus(window, false);
      if (window->closed) {
        return;
      }
    }
    window->epoch++;
    epoch = window->epoch;
    window->visible = true;
    [NSApp activateIgnoringOtherApps:YES];
    [window->window makeKeyAndOrderFront:nil];
    [window->window makeFirstResponder:window->view];
    if (!window->closed && window->window.isKeyWindow) {
      emit_focus(window, true);
    }
    if (!window->closed) {
      // CAMetalLayer rendering is explicit; AppKit does not reliably deliver updateLayer for the first frame.
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
      [window->window orderOut:nil];
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
    [window->window setFrame:frame display:window->visible];
    if (window->visible) {
      [window->view renderFrame];
    }
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
    [window->window setFrame:frame display:window->visible];
    if (window->visible) {
      [window->view renderFrame];
    }
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
    [window->view renderFrame];
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
    window->web_view_cache = nil;
    window->web_view_signatures = nil;
    window->web_view_content_keys = nil;
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

int32_t wox_darwin_window_begin_frame(WoxDarwinWindow *window, float logical_width, float logical_height, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->closed || window->renderer == NULL || logical_width <= 0.0f || logical_height <= 0.0f || scale <= 0.0f) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  if (renderer->frame_open) {
    return -1;
  }

  id<CAMetalDrawable> drawable = [renderer->layer nextDrawable];
  if (drawable == nil) {
    return 1;
  }
  id<MTLCommandBuffer> command_buffer = [renderer->queue commandBuffer];
  if (command_buffer == nil) {
    return -1;
  }

  vector_float4 clear = premultiplied_color(red, green, blue, alpha);
  MTLRenderPassDescriptor *pass = [MTLRenderPassDescriptor renderPassDescriptor];
  pass.colorAttachments[0].texture = drawable.texture;
  pass.colorAttachments[0].loadAction = MTLLoadActionClear;
  pass.colorAttachments[0].storeAction = MTLStoreActionStore;
  pass.colorAttachments[0].clearColor = MTLClearColorMake(clear.x, clear.y, clear.z, clear.w);
  id<MTLRenderCommandEncoder> encoder = [command_buffer renderCommandEncoderWithDescriptor:pass];
  if (encoder == nil) {
    return -1;
  }

  renderer->drawable = [drawable retain];
  renderer->command_buffer = [command_buffer retain];
  renderer->encoder = [encoder retain];
  renderer->viewport_size = (vector_float2){logical_width, logical_height};
  renderer->scale = scale;
  renderer->frame_open = true;
  return 0;
}

int32_t wox_darwin_window_fill_rounded_rect(WoxDarwinWindow *window, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f) {
    return 0;
  }

  WoxDarwinRenderer *renderer = window->renderer;
  WoxRectUniforms uniforms = {
      .viewport_size = renderer->viewport_size,
      .rect = (vector_float4){x, y, width, height},
      .color = premultiplied_color(red, green, blue, alpha),
      .radius = radius,
      .stroke_width = 0.0f,
  };
  [renderer->encoder setRenderPipelineState:renderer->rect_pipeline];
  [renderer->encoder setVertexBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder setFragmentBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
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
  WoxRectUniforms uniforms = {
      .viewport_size = renderer->viewport_size,
      .rect = (vector_float4){x, y, width, height},
      .color = premultiplied_color(red, green, blue, alpha),
      .radius = radius,
      .stroke_width = stroke_width,
  };
  [renderer->encoder setRenderPipelineState:renderer->rect_pipeline];
  [renderer->encoder setVertexBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder setFragmentBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
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

  NSUInteger pixel_width = (NSUInteger)ceil(width * renderer->scale);
  NSUInteger pixel_height = (NSUInteger)ceil(height * renderer->scale);
  if (pixel_width == 0 || pixel_height == 0 || pixel_width > 16384 || pixel_height > 16384) {
    return -1;
  }
  // ponytail: rasterize text per invalidated frame; cache textures when animated text makes this measurable.
  size_t row_bytes = pixel_width * 4;
  void *pixels = calloc(pixel_height, row_bytes);
  if (pixels == NULL) {
    return -1;
  }

  CGColorSpaceRef color_space = CGColorSpaceCreateDeviceRGB();
  CGContextRef context = CGBitmapContextCreate(
      pixels,
      pixel_width,
      pixel_height,
      8,
      row_bytes,
      color_space,
      kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big);
  CGColorSpaceRelease(color_space);
  if (context == NULL) {
    free(pixels);
    return -1;
  }

  NSString *string = [[NSString alloc] initWithUTF8String:text];
  if (string == nil) {
    CGContextRelease(context);
    free(pixels);
    return -1;
  }
  NSFont *font = wox_font(font_family, font_size * renderer->scale, font_weight);
  NSDictionary *attributes = [NSDictionary dictionaryWithObjectsAndKeys:
      font, (id)kCTFontAttributeName,
      (id)[[NSColor whiteColor] CGColor], (id)kCTForegroundColorAttributeName,
      nil];
  NSAttributedString *attributed = [[NSAttributedString alloc] initWithString:string attributes:attributes];
  CTLineRef line = CTLineCreateWithAttributedString((CFAttributedStringRef)attributed);
  CGFloat ascent = 0.0;
  CTLineGetTypographicBounds(line, &ascent, NULL, NULL);
  CGContextSetTextMatrix(context, CGAffineTransformIdentity);
  CGContextSetShouldAntialias(context, true);
  CGContextSetTextPosition(context, 0.0, fmax(0.0, (CGFloat)pixel_height - ascent));
  CTLineDraw(line, context);

  MTLTextureDescriptor *texture_descriptor = [MTLTextureDescriptor
      texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA8Unorm
                                   width:pixel_width
                                  height:pixel_height
                               mipmapped:NO];
  texture_descriptor.usage = MTLTextureUsageShaderRead;
  id<MTLTexture> texture = [renderer->device newTextureWithDescriptor:texture_descriptor];
  if (texture != nil) {
    [texture replaceRegion:MTLRegionMake2D(0, 0, pixel_width, pixel_height)
               mipmapLevel:0
                 withBytes:pixels
               bytesPerRow:row_bytes];
  }

  CFRelease(line);
  [attributed release];
  [string release];
  CGContextRelease(context);
  free(pixels);
  if (texture == nil) {
    return -1;
  }

  WoxTextureUniforms uniforms = {
      .viewport_size = renderer->viewport_size,
      .rect = (vector_float4){x, y, width, height},
      .color = premultiplied_color(red, green, blue, alpha),
  };
  [renderer->encoder setRenderPipelineState:renderer->texture_pipeline];
  [renderer->encoder setVertexBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder setFragmentTexture:texture atIndex:0];
  [renderer->encoder setFragmentBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
  [texture release];
  return 0;
}

int32_t wox_darwin_window_draw_image(WoxDarwinWindow *window, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open || pixels == NULL || image_width <= 0 || image_height <= 0 || row_stride < image_width * 4 || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  MTLTextureDescriptor *descriptor = [MTLTextureDescriptor
      texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA8Unorm
                                   width:(NSUInteger)image_width
                                  height:(NSUInteger)image_height
                               mipmapped:NO];
  descriptor.usage = MTLTextureUsageShaderRead;
  id<MTLTexture> texture = [renderer->device newTextureWithDescriptor:descriptor];
  if (texture == nil) {
    return -1;
  }
  [texture replaceRegion:MTLRegionMake2D(0, 0, (NSUInteger)image_width, (NSUInteger)image_height)
             mipmapLevel:0
               withBytes:pixels
             bytesPerRow:(NSUInteger)row_stride];

  WoxTextureUniforms uniforms = {
      .viewport_size = renderer->viewport_size,
      .rect = (vector_float4){x, y, width, height},
      .color = premultiplied_color(255, 255, 255, 255),
  };
  [renderer->encoder setRenderPipelineState:renderer->texture_pipeline];
  [renderer->encoder setVertexBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder setFragmentTexture:texture atIndex:0];
  [renderer->encoder setFragmentBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
  [texture release];
  return 0;
}

int32_t wox_darwin_window_set_clip_rect(WoxDarwinWindow *window, float x, float y, float width, float height) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  float max_width = renderer->viewport_size.x;
  float max_height = renderer->viewport_size.y;
  float left = fmaxf(0.0f, fminf(max_width, x));
  float top = fmaxf(0.0f, fminf(max_height, y));
  float right = fmaxf(left, fminf(max_width, x + fmaxf(0.0f, width)));
  float bottom = fmaxf(top, fminf(max_height, y + fmaxf(0.0f, height)));
  NSUInteger pixel_left = (NSUInteger)floorf(left * renderer->scale);
  NSUInteger pixel_top = (NSUInteger)floorf(top * renderer->scale);
  NSUInteger pixel_right = (NSUInteger)ceilf(right * renderer->scale);
  NSUInteger pixel_bottom = (NSUInteger)ceilf(bottom * renderer->scale);
  [renderer->encoder setScissorRect:(MTLScissorRect){pixel_left, pixel_top, pixel_right - pixel_left, pixel_bottom - pixel_top}];
  return 0;
}

int32_t wox_darwin_window_clear_clip(WoxDarwinWindow *window) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  NSUInteger width = (NSUInteger)ceilf(renderer->viewport_size.x * renderer->scale);
  NSUInteger height = (NSUInteger)ceilf(renderer->viewport_size.y * renderer->scale);
  [renderer->encoder setScissorRect:(MTLScissorRect){0, 0, width, height}];
  return 0;
}

int32_t wox_darwin_window_end_frame(WoxDarwinWindow *window) {
  if (window == NULL || window->renderer == NULL || !window->renderer->frame_open) {
    return -1;
  }
  WoxDarwinRenderer *renderer = window->renderer;
  [renderer->encoder endEncoding];
  [renderer->command_buffer presentDrawable:renderer->drawable];
  [renderer->command_buffer commit];
  [renderer->encoder release];
  [renderer->command_buffer release];
  [renderer->drawable release];
  renderer->encoder = nil;
  renderer->command_buffer = nil;
  renderer->drawable = nil;
  renderer->frame_open = false;
  return 0;
}
