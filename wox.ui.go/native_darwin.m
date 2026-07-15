//go:build darwin

#import "native_darwin.h"

#import <Cocoa/Cocoa.h>
#import <CoreText/CoreText.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#import <dispatch/dispatch.h>
#import <simd/simd.h>

#include <math.h>
#include <stdbool.h>
#include <stdlib.h>

extern int32_t woxGoDarwinStart(uintptr_t context);
extern void woxGoDarwinFrame(uintptr_t context, float width, float height, int32_t pixel_width, int32_t pixel_height, float scale);
extern void woxGoDarwinFocus(uintptr_t context, uint64_t epoch, int32_t active);

typedef struct WoxDarwinRenderer WoxDarwinRenderer;
@class WoxMetalView;
@class WoxWindowDelegate;

struct WoxDarwinWindow {
  NSWindow *window;
  WoxMetalView *view;
  WoxWindowDelegate *delegate;
  WoxDarwinRenderer *renderer;
  uintptr_t context;
  uint64_t epoch;
  bool visible;
  bool active;
  bool hide_on_blur;
  bool closed;
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
     "  if (radius == 0.0) {\n"
     "    return uniforms.color;\n"
     "  }\n"
     "  float2 half_size = uniforms.rect.zw * 0.5;\n"
     "  float2 edge = abs(input.local - half_size) - (half_size - radius);\n"
     "  float distance = length(max(edge, float2(0.0))) + min(max(edge.x, edge.y), 0.0) - radius;\n"
     "  float antialias = max(fwidth(distance), 0.001);\n"
     "  float coverage = 1.0 - smoothstep(-antialias * 0.5, antialias * 0.5, distance);\n"
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

@interface WoxMetalView : NSView {
@public
  WoxDarwinWindow *_owner;
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
    window->context = context;
    window->hide_on_blur = hide_on_blur != 0;
    native_window.contentView = view;
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
  };
  [renderer->encoder setRenderPipelineState:renderer->rect_pipeline];
  [renderer->encoder setVertexBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder setFragmentBytes:&uniforms length:sizeof(uniforms) atIndex:0];
  [renderer->encoder drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
  return 0;
}

int32_t wox_darwin_window_draw_text(WoxDarwinWindow *window, const char *text, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
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
  NSFontWeight weight = font_weight == 1 ? NSFontWeightSemibold : NSFontWeightRegular;
  NSFont *font = [NSFont systemFontOfSize:font_size * renderer->scale weight:weight];
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
