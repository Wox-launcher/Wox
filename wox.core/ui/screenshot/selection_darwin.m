//go:build darwin

#import "selection_darwin.h"
#import "object_selection_darwin.h"
#import "../runtime/native_darwin_platform.h"

#import <QuartzCore/QuartzCore.h>
#include <dlfcn.h>
#include <math.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

extern void woxGoDarwinScreenshotDiagnostic(const char *detail);

enum {
  WOX_SCREENSHOT_INSPECTOR_COLUMNS = 17,
  WOX_SCREENSHOT_INSPECTOR_ROWS = 9,
  // Extra chrome width keeps 3-digit RGB values from colliding with the G/H shortcut badges.
  WOX_SCREENSHOT_INSPECTOR_INFO_EXTRA_WIDTH = 24,
  WOX_SCREENSHOT_INSPECTOR_VALUE_BADGE_GAP = 8,
};

// Map a logical overlay point onto the immutable captured bitmap, matching the portable editor.
static bool wox_screenshot_pixel_at_point(
    int32_t image_width,
    int32_t image_height,
    float frame_width,
    float frame_height,
    float x,
    float y,
    int32_t *pixel_x,
    int32_t *pixel_y) {
  if (pixel_x == NULL || pixel_y == NULL || image_width <= 0 || image_height <= 0 || frame_width <= 0.0f || frame_height <= 0.0f ||
      x < 0.0f || y < 0.0f || x >= frame_width || y >= frame_height) {
    return false;
  }
  *pixel_x = (int32_t)MIN(image_width - 1, (int)floor((double)x * (double)image_width / (double)frame_width));
  *pixel_y = (int32_t)MIN(image_height - 1, (int)floor((double)y * (double)image_height / (double)frame_height));
  return true;
}

// wox_screenshot_color_shortcut maps the same physical G/H keys as the portable Windows editor.
// Character matching is unreliable here because a CJK IME leaves charactersIgnoringModifiers empty.
static bool wox_screenshot_color_shortcut(unsigned short key_code, bool *as_hex) {
  if (as_hex == NULL) {
    return false;
  }
  if (key_code == 5) {
    *as_hex = false;
    return true;
  }
  if (key_code == 4) {
    *as_hex = true;
    return true;
  }
  return false;
}

// Keep the floating inspector on-screen while preferring the pointer's lower-right side.
static NSRect wox_screenshot_inspector_rect(NSSize frame, NSPoint pointer, NSSize panel, float ui_scale) {
  float margin = 8.0f * ui_scale;
  float offset = 20.0f * ui_scale;
  float left = (float)pointer.x + offset;
  float top = (float)pointer.y + offset;
  if (left + (float)panel.width > (float)frame.width - margin) {
    left = (float)pointer.x - offset - (float)panel.width;
  }
  if (top + (float)panel.height > (float)frame.height - margin) {
    top = (float)pointer.y - offset - (float)panel.height;
  }
  float max_left = fmaxf(margin, (float)frame.width - (float)panel.width - margin);
  float max_top = fmaxf(margin, (float)frame.height - (float)panel.height - margin);
  return NSMakeRect(fminf(fmaxf(margin, left), max_left), fminf(fmaxf(margin, top), max_top), panel.width, panel.height);
}

static void wox_screenshot_draw_text(NSString *text, NSRect rect, CGFloat size, NSColor *color, BOOL centered) {
  if (text.length == 0) {
    return;
  }
  NSDictionary *attributes = @{
    NSFontAttributeName : [NSFont systemFontOfSize:size weight:NSFontWeightSemibold],
    NSForegroundColorAttributeName : color,
  };
  NSSize measured = [text sizeWithAttributes:attributes];
  NSRect draw_rect = rect;
  draw_rect.size.width = MIN(measured.width, NSWidth(rect));
  draw_rect.size.height = MIN(measured.height, NSHeight(rect));
  if (centered) {
    draw_rect.origin.x = NSMinX(rect) + (NSWidth(rect) - NSWidth(draw_rect)) / 2.0;
  }
  draw_rect.origin.y = NSMinY(rect) + (NSHeight(rect) - NSHeight(draw_rect)) / 2.0;
  [text drawInRect:draw_rect withAttributes:attributes];
}

@interface WoxScreenshotDisplayCapture : NSObject {
@public
  CGDirectDisplayID display_id;
  NSRect logical_bounds;
  CGImageRef image;
  NSBitmapImageRep *_bitmap;
}
- (instancetype)initWithScreen:(NSScreen *)screen desktopTop:(CGFloat)desktop_top;
- (NSInteger)pixelWidth;
- (NSInteger)pixelHeight;
- (BOOL)samplePixelX:(int32_t)x y:(int32_t)y red:(uint8_t *)red green:(uint8_t *)green blue:(uint8_t *)blue;
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
  image = wox_darwin_copy_display_image(display_id);
  if (image == NULL) {
    [self release];
    return nil;
  }
  _bitmap = [[NSBitmapImageRep alloc] initWithCGImage:image];
  return self;
}

- (NSInteger)pixelWidth {
  return _bitmap != nil ? _bitmap.pixelsWide : (NSInteger)CGImageGetWidth(image);
}

- (NSInteger)pixelHeight {
  return _bitmap != nil ? _bitmap.pixelsHigh : (NSInteger)CGImageGetHeight(image);
}

// samplePixelX:y: reads one captured desktop pixel in sRGB 8-bit channels.
- (BOOL)samplePixelX:(int32_t)x y:(int32_t)y red:(uint8_t *)red green:(uint8_t *)green blue:(uint8_t *)blue {
  if (_bitmap == nil || red == NULL || green == NULL || blue == NULL || _bitmap.pixelsWide <= 0 || _bitmap.pixelsHigh <= 0) {
    return NO;
  }
  x = MIN(MAX(0, x), (int32_t)_bitmap.pixelsWide - 1);
  y = MIN(MAX(0, y), (int32_t)_bitmap.pixelsHigh - 1);
  NSColor *color = [[_bitmap colorAtX:x y:y] colorUsingColorSpace:NSColorSpace.sRGBColorSpace];
  if (color == nil) {
    return NO;
  }
  *red = (uint8_t)lround(color.redComponent * 255.0);
  *green = (uint8_t)lround(color.greenComponent * 255.0);
  *blue = (uint8_t)lround(color.blueComponent * 255.0);
  return YES;
}

- (void)dealloc {
  [_bitmap release];
  if (image != NULL) {
    CGImageRelease(image);
  }
  [super dealloc];
}
@end

@class WoxScreenshotSelectionSession;

// Present captured pixels directly. Drawing the full screenshot through AppKit's
// CGContext leaves a display-sized CA Whippet Drawable cached after window close.
@interface WoxScreenshotBackgroundView : NSView
@end

@implementation WoxScreenshotBackgroundView
- (BOOL)wantsUpdateLayer {
  return YES;
}

- (void)updateLayer {
  // The immutable screenshot is assigned once when the selection window is created.
}
@end

@class WoxScreenshotSelectionView;

// Only the small inspector needs a raster backing store; the desktop mask uses layers.
@interface WoxScreenshotInspectorView : NSView {
@public
  WoxScreenshotSelectionView *owner;
}
@end

@interface WoxScreenshotSelectionView : NSView {
  WoxScreenshotDisplayCapture *_capture;
  WoxScreenshotSelectionSession *_session;
  NSRect _global_selection;
  NSPoint _hover_local;
  BOOL _has_selection;
  BOOL _animate_selection;
  BOOL _selection_dirty;
  BOOL _has_presented_selection;
  BOOL _hover_visible;
  CALayer *_mask_layers[4];
  CALayer *_selection_border;
  WoxScreenshotInspectorView *_inspector;
}
- (void)drawColorInspector;
- (NSRect)inspectorRect;
- (instancetype)initWithCapture:(WoxScreenshotDisplayCapture *)capture;
- (void)setSession:(WoxScreenshotSelectionSession *)session;
- (void)setGlobalSelection:(NSRect)selection visible:(BOOL)visible animated:(BOOL)animated;
- (void)setHoverPoint:(NSPoint)global_point visible:(BOOL)visible;
@end

@interface WoxScreenshotSelectionSession : NSObject
- (BOOL)handleKeyEvent:(NSEvent *)event;
@end

@implementation WoxScreenshotInspectorView
- (BOOL)isFlipped {
  return YES;
}

// Inspector drawing keeps the selection view's logical coordinate system.
- (void)drawRect:(NSRect)dirty_rect {
  (void)dirty_rect;
  if (owner == nil) return;
  NSRectFillUsingOperation(self.bounds, NSCompositingOperationClear);
  CGContextRef context = NSGraphicsContext.currentContext.CGContext;
  CGContextSaveGState(context);
  CGContextTranslateCTM(context, -NSMinX(self.frame), -NSMinY(self.frame));
  [owner drawColorInspector];
  CGContextRestoreGState(context);
}
@end

@implementation WoxScreenshotSelectionView
- (instancetype)initWithCapture:(WoxScreenshotDisplayCapture *)capture {
  self = [super initWithFrame:NSMakeRect(0.0, 0.0, NSWidth(capture->logical_bounds), NSHeight(capture->logical_bounds))];
  if (self == nil) {
    return nil;
  }
  _capture = [capture retain];
  _session = nil;
  _global_selection = NSZeroRect;
  _hover_local = NSZeroPoint;
  _has_selection = NO;
  _selection_dirty = YES;
  _hover_visible = NO;
  self.wantsLayer = YES;
  self.layer.geometryFlipped = YES;
  for (NSUInteger index = 0; index < 4; index++) {
    _mask_layers[index] = [CALayer layer];
    _mask_layers[index].backgroundColor = [NSColor colorWithCalibratedWhite:0 alpha:0.46].CGColor;
    [self.layer addSublayer:_mask_layers[index]];
  }
  _selection_border = [CALayer layer];
  _selection_border.borderColor = [NSColor colorWithCalibratedRed:41.0 / 255.0 green:1.0 blue:114.0 / 255.0 alpha:1.0].CGColor;
  _selection_border.borderWidth = 2.0;
  [self.layer addSublayer:_selection_border];
  _inspector = [[WoxScreenshotInspectorView alloc] initWithFrame:NSZeroRect];
  _inspector->owner = self;
  _inspector.wantsLayer = YES;
  [self addSubview:_inspector];
  [_inspector release];
  return self;
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

- (void)setSession:(WoxScreenshotSelectionSession *)session {
  _session = session;
}

- (void)keyDown:(NSEvent *)event {
  if (_session != nil && [_session handleKeyEvent:event]) {
    return;
  }
  [super keyDown:event];
}

- (void)resetCursorRects {
  [self discardCursorRects];
  [self addCursorRect:self.bounds cursor:[NSCursor crosshairCursor]];
}

// Animate only hover targets; dragging and the committed crop always display their exact geometry.
- (void)setGlobalSelection:(NSRect)selection visible:(BOOL)visible animated:(BOOL)animated {
  if (animated && NSEqualRects(_global_selection, selection) && _has_selection == visible) return;
  // A fast native hit can arrive before the first AppKit redraw. Animate only from a selection already presented.
  _animate_selection = animated && _has_presented_selection && _has_selection && visible && !NSIsEmptyRect(_global_selection);
  _global_selection = selection;
  _has_selection = visible;
  _selection_dirty = YES;
  self.needsDisplay = YES;
}

// setHoverPoint:visible: tracks the pointer on this display so only one overlay draws the inspector.
- (void)setHoverPoint:(NSPoint)global_point visible:(BOOL)visible {
  BOOL on_display = visible && NSPointInRect(global_point, _capture->logical_bounds);
  NSPoint local = NSMakePoint(global_point.x - NSMinX(_capture->logical_bounds), global_point.y - NSMinY(_capture->logical_bounds));
  if (_hover_visible == on_display && NSEqualPoints(_hover_local, local)) {
    return;
  }
  _hover_visible = on_display;
  _hover_local = local;
  self.needsDisplay = YES;
}

// The panel frame stays in logical display coordinates; AppKit owns backing scale.
- (NSRect)inspectorRect {
  return wox_screenshot_inspector_rect(self.bounds.size, _hover_local,
      NSMakeSize(12.0f * WOX_SCREENSHOT_INSPECTOR_COLUMNS + WOX_SCREENSHOT_INSPECTOR_INFO_EXTRA_WIDTH,
                 12.0f * WOX_SCREENSHOT_INSPECTOR_ROWS + 104.0f), 1.0f);
}

// drawColorInspector mirrors the portable editor's pre-selection sampler. macOS selects on a
// native overlay first, so this UI has to live here or the color inspector never appears.
- (void)drawColorInspector {
  if (!_hover_visible || _capture == nil) {
    return;
  }
  int32_t pixel_x = 0;
  int32_t pixel_y = 0;
  if (!wox_screenshot_pixel_at_point(
          (int32_t)[_capture pixelWidth],
          (int32_t)[_capture pixelHeight],
          (float)NSWidth(self.bounds),
          (float)NSHeight(self.bounds),
          (float)_hover_local.x,
          (float)_hover_local.y,
          &pixel_x,
          &pixel_y)) {
    return;
  }
  uint8_t red = 0;
  uint8_t green = 0;
  uint8_t blue = 0;
  if (![_capture samplePixelX:pixel_x y:pixel_y red:&red green:&green blue:&blue]) {
    return;
  }

  const float ui_scale = 1.0f;
  const float cell = 12.0f * ui_scale;
  const float preview_width = cell * WOX_SCREENSHOT_INSPECTOR_COLUMNS;
  const float preview_height = cell * WOX_SCREENSHOT_INSPECTOR_ROWS;
  NSRect panel = [self inspectorRect];
  const float grid_x = (float)NSMinX(panel) + ((float)NSWidth(panel) - preview_width) / 2.0f;
  NSBezierPath *background = [NSBezierPath bezierPathWithRoundedRect:panel xRadius:10.0 * ui_scale yRadius:10.0 * ui_scale];
  [[NSColor colorWithCalibratedRed:20.0 / 255.0 green:18.0 / 255.0 blue:17.0 / 255.0 alpha:248.0 / 255.0] setFill];
  [background fill];

  int half_columns = WOX_SCREENSHOT_INSPECTOR_COLUMNS / 2;
  int half_rows = WOX_SCREENSHOT_INSPECTOR_ROWS / 2;
  NSColor *grid = [NSColor colorWithCalibratedWhite:0.0 alpha:55.0 / 255.0];
  for (int row = 0; row < WOX_SCREENSHOT_INSPECTOR_ROWS; row++) {
    for (int column = 0; column < WOX_SCREENSHOT_INSPECTOR_COLUMNS; column++) {
      int32_t sample_x = MIN(MAX(0, pixel_x + column - half_columns), (int32_t)[_capture pixelWidth] - 1);
      int32_t sample_y = MIN(MAX(0, pixel_y + row - half_rows), (int32_t)[_capture pixelHeight] - 1);
      uint8_t sample_red = 0;
      uint8_t sample_green = 0;
      uint8_t sample_blue = 0;
      [_capture samplePixelX:sample_x y:sample_y red:&sample_red green:&sample_green blue:&sample_blue];
      NSRect cell_rect = NSMakeRect(grid_x + column * cell, NSMinY(panel) + row * cell, cell, cell);
      [[NSColor colorWithCalibratedRed:sample_red / 255.0 green:sample_green / 255.0 blue:sample_blue / 255.0 alpha:1.0] setFill];
      NSRectFill(cell_rect);
      [grid setStroke];
      NSBezierPath *cell_path = [NSBezierPath bezierPathWithRect:cell_rect];
      cell_path.lineWidth = MAX(0.5, 0.5 * ui_scale);
      [cell_path stroke];
    }
  }

  NSRect center = NSMakeRect(grid_x + half_columns * cell, NSMinY(panel) + half_rows * cell, cell, cell);
  [[NSColor colorWithCalibratedRed:41.0 / 255.0 green:1.0 blue:114.0 / 255.0 alpha:1.0] setStroke];
  NSBezierPath *center_path = [NSBezierPath bezierPathWithRect:center];
  center_path.lineWidth = 2.0 * ui_scale;
  [center_path stroke];

  [[NSColor colorWithCalibratedWhite:1.0 alpha:35.0 / 255.0] setFill];
  NSRectFill(NSMakeRect(NSMinX(panel), NSMinY(panel) + preview_height, NSWidth(panel), 1.0 * ui_scale));

  NSColor *text_color = [NSColor colorWithCalibratedWhite:1.0 alpha:1.0];
  NSColor *secondary = [NSColor colorWithCalibratedWhite:1.0 alpha:165.0 / 255.0];
  float info_top = (float)NSMinY(panel) + preview_height;
  int origin_x = (int)lround((double)NSMinX(_capture->logical_bounds) * (double)[_capture pixelWidth] / (double)NSWidth(_capture->logical_bounds));
  int origin_y = (int)lround((double)NSMinY(_capture->logical_bounds) * (double)[_capture pixelHeight] / (double)NSHeight(_capture->logical_bounds));
  wox_screenshot_draw_text(
      [NSString stringWithFormat:@"%d, %d", origin_x + pixel_x, origin_y + pixel_y],
      NSMakeRect(NSMinX(panel), info_top + 9.0f * ui_scale, NSWidth(panel), 18.0f * ui_scale),
      12.0 * ui_scale,
      secondary,
      YES);

  NSRect swatch = NSMakeRect(NSMinX(panel) + 12.0f * ui_scale, info_top + 38.0f * ui_scale, 52.0f * ui_scale, 52.0f * ui_scale);
  NSBezierPath *swatch_path = [NSBezierPath bezierPathWithRoundedRect:swatch xRadius:8.0 * ui_scale yRadius:8.0 * ui_scale];
  [[NSColor colorWithCalibratedRed:red / 255.0 green:green / 255.0 blue:blue / 255.0 alpha:1.0] setFill];
  [swatch_path fill];
  [[NSColor colorWithCalibratedWhite:1.0 alpha:190.0 / 255.0] setStroke];
  swatch_path.lineWidth = 1.0 * ui_scale;
  [swatch_path stroke];

  void (^draw_color_row)(NSString *, NSString *, NSString *, float) = ^(NSString *label, NSString *value, NSString *shortcut, float top) {
    NSRect badge = NSMakeRect(NSMaxX(panel) - 30.0f * ui_scale, top, 18.0f * ui_scale, 18.0f * ui_scale);
    wox_screenshot_draw_text(label, NSMakeRect(NSMinX(panel) + 72.0f * ui_scale, top + 1.0f * ui_scale, 28.0f * ui_scale, 18.0f * ui_scale), 11.0 * ui_scale, secondary, NO);
    wox_screenshot_draw_text(
        value,
        NSMakeRect(NSMinX(panel) + 104.0f * ui_scale, top + 1.0f * ui_scale, NSMinX(badge) - NSMinX(panel) - (104.0f + WOX_SCREENSHOT_INSPECTOR_VALUE_BADGE_GAP) * ui_scale, 18.0f * ui_scale),
        11.0 * ui_scale,
        text_color,
        NO);
    NSBezierPath *badge_path = [NSBezierPath bezierPathWithRoundedRect:badge xRadius:5.0 * ui_scale yRadius:5.0 * ui_scale];
    [[NSColor colorWithCalibratedWhite:1.0 alpha:25.0 / 255.0] setFill];
    [badge_path fill];
    wox_screenshot_draw_text(shortcut, NSMakeRect(NSMinX(badge) + 5.0f * ui_scale, NSMinY(badge) + 1.0f * ui_scale, 8.0f * ui_scale, 16.0f * ui_scale), 11.0 * ui_scale, secondary, NO);
  };
  draw_color_row(@"RGB", [NSString stringWithFormat:@"%d, %d, %d", red, green, blue], @"G", info_top + 40.0f * ui_scale);
  draw_color_row(@"HEX", [NSString stringWithFormat:@"#%02X%02X%02X", red, green, blue], @"H", info_top + 69.0f * ui_scale);
}

- (BOOL)wantsUpdateLayer {
  return YES;
}

// Never rasterize a desktop-sized mask. AppKit retains a floating-point drawable
// after drawRect redraws even when the screenshot view and window are destroyed.
- (void)updateLayer {
  NSRect intersection = NSIntersectionRect(_capture->logical_bounds, _global_selection);
  BOOL selected = _has_selection && !NSIsEmptyRect(intersection);
  NSRect bands[4] = {self.bounds, NSZeroRect, NSZeroRect, NSZeroRect};
  NSRect local = NSZeroRect;
  if (selected) {
    local = NSOffsetRect(intersection, -NSMinX(_capture->logical_bounds), -NSMinY(_capture->logical_bounds));
    bands[0] = NSMakeRect(0, 0, NSWidth(self.bounds), NSMinY(local));
    bands[1] = NSMakeRect(0, NSMaxY(local), NSWidth(self.bounds), NSHeight(self.bounds) - NSMaxY(local));
    bands[2] = NSMakeRect(0, NSMinY(local), NSMinX(local), NSHeight(local));
    bands[3] = NSMakeRect(NSMaxX(local), NSMinY(local), NSWidth(self.bounds) - NSMaxX(local), NSHeight(local));
  }
  [CATransaction begin];
  [CATransaction setDisableActions:YES];
  // Inspector redraws follow every pointer event; they must not cancel or restart an ongoing selection transition.
  if (_selection_dirty) {
    for (NSUInteger index = 0; index < 4; index++) {
      wox_screenshot_set_selection_layer_frame(_mask_layers[index], NSRectToCGRect(bands[index]), _animate_selection);
      _mask_layers[index].hidden = NO;
    }
    wox_screenshot_set_selection_layer_frame(_selection_border, NSRectToCGRect(local), _animate_selection);
    _selection_border.hidden = !selected;
    _has_presented_selection = selected;
    _animate_selection = NO;
    _selection_dirty = NO;
  }
  _inspector.hidden = !_hover_visible;
  if (!_inspector.hidden) {
    _inspector.frame = [self inspectorRect];
    _inspector.needsDisplay = YES;
  }
  [CATransaction commit];
}

- (void)dealloc {
  if (_inspector != nil) {
    _inspector->owner = nil;
  }
  [_capture release];
  [super dealloc];
}
@end

// WindowServer's top-left origin belongs to the primary display; Wox's origin is the top of the virtual desktop.
// Keep both in logical points so a display above the primary display does not shift hit tests or window crops.
static NSArray *screenshot_window_candidates_in_desktop(NSArray *windows, CGFloat primary_offset_y) {
  NSMutableArray *mapped = [NSMutableArray arrayWithCapacity:windows.count];
  for (NSDictionary *info in windows) {
    CGRect bounds;
    CFDictionaryRef dictionary = (CFDictionaryRef)info[(id)kCGWindowBounds];
    if (dictionary == NULL || !CGRectMakeWithDictionaryRepresentation(dictionary, &bounds)) continue;
    bounds.origin.y += primary_offset_y;
    NSMutableDictionary *entry = [[info mutableCopy] autorelease];
    CFDictionaryRef coordinates = CGRectCreateDictionaryRepresentation(bounds);
    entry[(id)kCGWindowBounds] = (NSDictionary *)coordinates;
    CFRelease(coordinates);
    [mapped addObject:entry];
  }
  return mapped;
}

// Resolve WindowServer's front-to-back logical frames without touching live NSWindows or backing pixels.
static NSRect screenshot_window_at_point(NSArray *candidates, NSRect display_bounds, NSPoint point) {
  for (NSDictionary *info in candidates) {
    CGRect bounds;
    CFDictionaryRef dictionary = (CFDictionaryRef)info[(id)kCGWindowBounds];
    if (dictionary == NULL || !CGRectMakeWithDictionaryRepresentation(dictionary, &bounds)) continue;
    NSRect frame = NSRectFromCGRect(bounds);
    if (!NSPointInRect(point, frame)) continue;
    NSRect visible = NSIntersectionRect(frame, display_bounds);
    if (NSWidth(visible) >= 2.0 && NSHeight(visible) >= 2.0) return visible;
  }
  return NSZeroRect;
}

// Capture the selected WindowServer surface directly so its real alpha excludes the desktop behind rounded corners.
static CGImageRef screenshot_window_image(NSArray *candidates, NSRect selection, NSPoint point) {
  typedef CGImageRef (*CaptureWindow)(CGRect, CGWindowListOption, CGWindowID, CGWindowImageOption);
  CaptureWindow capture = (CaptureWindow)dlsym(RTLD_DEFAULT, "CGWindowListCreateImage");
  if (capture == NULL) return NULL;
  for (NSDictionary *info in candidates) {
    CGRect bounds;
    CFDictionaryRef dictionary = (CFDictionaryRef)info[(id)kCGWindowBounds];
    if (dictionary == NULL || !CGRectMakeWithDictionaryRepresentation(dictionary, &bounds) || !CGRectContainsPoint(bounds, NSPointToCGPoint(point))) continue;
    CGWindowID window = [info[(id)kCGWindowNumber] unsignedIntValue];
    CGImageRef image = capture(CGRectNull, kCGWindowListOptionIncludingWindow, window,
                              kCGWindowImageBoundsIgnoreFraming | kCGWindowImageBestResolution);
    if (image == NULL) return NULL;
    CGFloat scale_x = CGImageGetWidth(image) / bounds.size.width;
    CGFloat scale_y = CGImageGetHeight(image) / bounds.size.height;
    CGRect clip = CGRectMake((NSMinX(selection)-bounds.origin.x)*scale_x, (NSMinY(selection)-bounds.origin.y)*scale_y,
                             NSWidth(selection)*scale_x, NSHeight(selection)*scale_y);
    CGImageRef cropped = CGImageCreateWithImageInRect(image, clip);
    CGImageRelease(image);
    return cropped;
  }
  return NULL;
}

// Exercise native filtering, overlap order, and display clipping without opening a selector or requesting permission.
int32_t wox_darwin_test_screenshot_window_selection(void) {
  @autoreleasepool {
    NSRect front = NSMakeRect(-100, 40, 300, 200);
    NSRect back = NSMakeRect(-300, 0, 800, 600);
    __block NSUInteger identifier = 0;
    NSDictionary *(^candidate)(NSRect, NSInteger, double) = ^NSDictionary *(NSRect frame, NSInteger layer, double alpha) {
      NSDictionary *bounds = (NSDictionary *)CGRectCreateDictionaryRepresentation(NSRectToCGRect(frame));
      NSDictionary *info = @{
        (id)kCGWindowBounds: bounds,
        (id)kCGWindowNumber: @(++identifier),
        (id)kCGWindowOwnerPID: @(getpid()),
        (id)kCGWindowLayer: @(layer),
        (id)kCGWindowAlpha: @(alpha)
      };
      [bounds release];
      return info;
    };
    NSDictionary *overlay = candidate(back, CGShieldingWindowLevel(), 1);
    NSArray *windows = wox_screenshot_selectable_windows(@[overlay, candidate(back, -1, 1), candidate(back, 0, 0),
        candidate(front, NSFloatingWindowLevel, 1), candidate(back, 0, 1)], [NSSet setWithObject:overlay[(id)kCGWindowNumber]]);
    if (windows.count != 2) return 7;
    NSRect left_display = NSMakeRect(-500, 0, 500, 600);
    NSRect right_display = NSMakeRect(0, 0, 800, 600);
    if (!NSEqualRects(screenshot_window_at_point(windows, left_display, NSMakePoint(-50, 80)), NSMakeRect(-100, 40, 100, 200))) return 1;
    if (!NSEqualRects(screenshot_window_at_point(windows, right_display, NSMakePoint(50, 80)), NSMakeRect(0, 40, 200, 200))) return 2;
    if (!NSEqualRects(screenshot_window_at_point(windows, left_display, NSMakePoint(-200, 80)), NSMakeRect(-300, 0, 300, 600))) return 3;
    if (!NSIsEmptyRect(screenshot_window_at_point(windows, right_display, NSMakePoint(600, 80)))) return 4;
    if (!NSIsEmptyRect(screenshot_window_at_point(@[], right_display, NSMakePoint(50, 80)))) return 5;
    NSArray *above_primary = screenshot_window_candidates_in_desktop(windows, 600);
    NSRect shifted_display = NSOffsetRect(left_display, 0, 600);
    if (!NSEqualRects(screenshot_window_at_point(above_primary, shifted_display, NSMakePoint(-50, 680)), NSMakeRect(-100, 640, 100, 200))) return 6;
    // A non-Dock application palette can legitimately use the Dock's window level.
    NSArray *palettes = wox_screenshot_selectable_windows(@[candidate(front, CGWindowLevelForKey(kCGDockWindowLevelKey), 1)], nil);
    if (palettes.count != 1) return 8;
    if (wox_screenshot_selectable_windows(@[candidate(front, 0, NAN), candidate(NSZeroRect, 0, 1), @{}], nil).count != 0) return 9;
    return 0;
  }
}

@interface WoxScreenshotSelectionSession () {
  NSArray *_captures;
  NSArray *_windows;
  NSArray *_window_candidates;
  WoxScreenshotObjectSelector *_object_selector;
  id _event_monitor;
  dispatch_semaphore_t _completion;
  NSPoint _drag_start;
  NSPoint _hover_point;
  BOOL _dragging;
  BOOL _selection_dragged;
  BOOL _completed;
  BOOL _cancelled;
  BOOL _dismissed;
  BOOL _hover_visible;
  WoxScreenshotDisplayCapture *_drag_capture;
  WoxScreenshotDisplayCapture *_selected_capture;
  NSRect _selection;
  NSRect _pressed_window;
  CGImageRef _selected_window_image;
  NSString *_copied_color;
}
- (instancetype)initWithCaptures:(NSArray *)captures;
- (void)begin;
- (void)dismiss;
- (BOOL)cancelled;
- (NSString *)copiedColor;
- (WoxScreenshotDisplayCapture *)selectedCapture;
- (NSRect)selection;
- (CGImageRef)selectedWindowImage;
- (dispatch_semaphore_t)completion;
@end

@implementation WoxScreenshotSelectionSession
- (instancetype)initWithCaptures:(NSArray *)captures {
  self = [super init];
  if (self == nil) {
    return nil;
  }
  _captures = [captures copy];
  // Snapshot front-to-back WindowServer bounds before creating the selector panels.
  // These are global top-left logical points; capture backing pixels are mapped separately.
  NSArray *window_info = (NSArray *)CGWindowListCopyWindowInfo(
      kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
  // Exclude capture surfaces by identity, preserving Wox's launcher, management windows, and pinned images at their own levels.
  NSMutableSet *excluded = [NSMutableSet set];
  for (NSWindow *window in NSApp.windows) {
    if ([window isKindOfClass:[WoxOverlayPanel class]]) [excluded addObject:@(window.windowNumber)];
  }
  CGFloat primary_offset_y = wox_darwin_desktop_top() - NSMaxY([NSScreen screens].firstObject.frame);
  _window_candidates = [screenshot_window_candidates_in_desktop(wox_screenshot_selectable_windows(window_info, excluded), primary_offset_y) copy];
  // The selector stops its assign callback before this session is released; background work retains only the selector.
  __block WoxScreenshotSelectionSession *session = self;
  _object_selector = [[WoxScreenshotObjectSelector alloc] initWithCandidates:_window_candidates primaryOffset:primary_offset_y update:^(NSRect target) {
    if (session->_completed || session->_dismissed || session->_dragging) return;
    for (WoxOverlayPanel *window in session->_windows) {
      [(WoxScreenshotSelectionView *)window.contentView.subviews.firstObject setGlobalSelection:target visible:!NSIsEmptyRect(target) animated:YES];
    }
  }];
  [window_info release];
  _completion = dispatch_semaphore_create(0);
  NSMutableArray *windows = [NSMutableArray arrayWithCapacity:_captures.count];
  for (WoxScreenshotDisplayCapture *capture in _captures) {
    WoxOverlayPanel *window = [[WoxOverlayPanel alloc]
        initWithContentRect:NSMakeRect(
                                NSMinX(capture->logical_bounds),
                                wox_darwin_desktop_top() - NSMaxY(capture->logical_bounds),
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
    [view setSession:self];
    WoxScreenshotBackgroundView *background = [[WoxScreenshotBackgroundView alloc] initWithFrame:view.frame];
    background.wantsLayer = YES;
    background.layer.contents = (id)capture->image;
    background.layer.contentsGravity = kCAGravityResize;
    window.contentView = background;
    view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [background addSubview:view];
    [background release];
    [view release];
    [windows addObject:window];
    [window release];
  }
  _windows = [windows copy];
  return self;
}

- (NSPoint)topLeftMouseLocation {
  NSPoint location = NSEvent.mouseLocation;
  return NSMakePoint(location.x, wox_darwin_desktop_top() - location.y);
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
  for (WoxOverlayPanel *window in _windows) {
    [(WoxScreenshotSelectionView *)window.contentView.subviews.firstObject setGlobalSelection:selection visible:visible animated:NO];
  }
}

// windowAtPoint: returns only actual windows so a display fallback cannot trigger a window-alpha capture.
- (NSRect)windowAtPoint:(NSPoint)point {
  WoxScreenshotDisplayCapture *capture = [self captureAtPoint:point];
  if (capture == nil) return NSZeroRect;
  return screenshot_window_at_point(_window_candidates, capture->logical_bounds, point);
}

// updateHoverAt:visible: keeps the inspector on the display under the pointer until a drag starts.
- (void)updateHoverAt:(NSPoint)point visible:(BOOL)visible {
  _hover_point = point;
  _hover_visible = visible && [self captureAtPoint:point] != nil;
  for (WoxOverlayPanel *window in _windows) {
    [(WoxScreenshotSelectionView *)window.contentView.subviews.firstObject setHoverPoint:point visible:_hover_visible];
  }
  if (!_dragging) {
    WoxScreenshotDisplayCapture *capture = visible ? [self captureAtPoint:point] : nil;
    if (capture != nil) {
      [_object_selector hoverAt:point displayBounds:capture->logical_bounds];
    } else {
      [_object_selector hoverAt:point displayBounds:NSZeroRect];
      [self updateSelection:NSZeroRect visible:NO];
    }
  }
}

// sampleHoverRed:green:blue:pixelX:pixelY:capture: reads the captured pixel under the current pointer.
- (BOOL)sampleHoverRed:(uint8_t *)red green:(uint8_t *)green blue:(uint8_t *)blue pixelX:(int32_t *)pixel_x pixelY:(int32_t *)pixel_y capture:(WoxScreenshotDisplayCapture **)capture {
  WoxScreenshotDisplayCapture *hovered = [self captureAtPoint:_hover_point];
  if (!_hover_visible || hovered == nil) {
    return NO;
  }
  NSPoint local = NSMakePoint(_hover_point.x - NSMinX(hovered->logical_bounds), _hover_point.y - NSMinY(hovered->logical_bounds));
  int32_t x = 0;
  int32_t y = 0;
  if (!wox_screenshot_pixel_at_point(
          (int32_t)[hovered pixelWidth],
          (int32_t)[hovered pixelHeight],
          (float)NSWidth(hovered->logical_bounds),
          (float)NSHeight(hovered->logical_bounds),
          (float)local.x,
          (float)local.y,
          &x,
          &y)) {
    return NO;
  }
  if (![hovered samplePixelX:x y:y red:red green:green blue:blue]) {
    return NO;
  }
  if (pixel_x != NULL) {
    *pixel_x = x;
  }
  if (pixel_y != NULL) {
    *pixel_y = y;
  }
  if (capture != NULL) {
    *capture = hovered;
  }
  return YES;
}

// copyHoverColorAsHex writes the hovered pixel and closes the native selector without a region.
- (void)copyHoverColorAsHex:(BOOL)as_hex {
  uint8_t red = 0;
  uint8_t green = 0;
  uint8_t blue = 0;
  if (![self sampleHoverRed:&red green:&green blue:&blue pixelX:NULL pixelY:NULL capture:NULL]) {
    return;
  }
  NSString *value = as_hex ? [NSString stringWithFormat:@"#%02X%02X%02X", red, green, blue]
                           : [NSString stringWithFormat:@"rgb(%d, %d, %d)", red, green, blue];
  NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
  [pasteboard clearContents];
  [pasteboard setString:value forType:NSPasteboardTypeString];
  [self completeCancelled:NO selection:NSZeroRect copiedColor:value];
}

// makeKeyForPoint: keeps key events on the overlay under the pointer so G/H are not delivered to another app.
- (void)makeKeyForPoint:(NSPoint)point {
  WoxScreenshotDisplayCapture *capture = [self captureAtPoint:point];
  if (capture == nil) {
    return;
  }
  NSUInteger index = [_captures indexOfObject:capture];
  if (index == NSNotFound || index >= _windows.count) {
    return;
  }
  WoxOverlayPanel *window = _windows[index];
  if (window.isKeyWindow) {
    return;
  }
  [window makeKeyAndOrderFront:nil];
  [window makeFirstResponder:window.contentView.subviews.firstObject];
}

// handleKeyEvent copies the inspected color with the same G/H keys as the portable Windows editor.
- (BOOL)handleKeyEvent:(NSEvent *)event {
  if (_completed || event.type != NSEventTypeKeyDown) {
    return NO;
  }
  NSEventModifierFlags modifiers = event.modifierFlags & (NSEventModifierFlagCommand | NSEventModifierFlagControl | NSEventModifierFlagOption);
  if (event.keyCode == 53) {
    [self completeCancelled:YES selection:NSZeroRect];
    return YES;
  }
  if (modifiers != 0 || _dragging) {
    return YES;
  }
  bool as_hex = false;
  if (wox_screenshot_color_shortcut(event.keyCode, &as_hex)) {
    [self copyHoverColorAsHex:as_hex ? YES : NO];
    return YES;
  }
  if (event.keyCode == 123) {
    [self nudgeHoverByDeltaX:-1 deltaY:0];
    return YES;
  }
  if (event.keyCode == 124) {
    [self nudgeHoverByDeltaX:1 deltaY:0];
    return YES;
  }
  if (event.keyCode == 126) {
    [self nudgeHoverByDeltaX:0 deltaY:-1];
    return YES;
  }
  if (event.keyCode == 125) {
    [self nudgeHoverByDeltaX:0 deltaY:1];
    return YES;
  }
  return YES;
}

// nudgeHoverByDeltaX:deltaY: moves the pointer to the center of an adjacent captured pixel.
- (void)nudgeHoverByDeltaX:(int)delta_x deltaY:(int)delta_y {
  uint8_t red = 0;
  uint8_t green = 0;
  uint8_t blue = 0;
  int32_t pixel_x = 0;
  int32_t pixel_y = 0;
  WoxScreenshotDisplayCapture *capture = nil;
  if (![self sampleHoverRed:&red green:&green blue:&blue pixelX:&pixel_x pixelY:&pixel_y capture:&capture] || capture == nil) {
    return;
  }
  pixel_x = MIN(MAX(0, pixel_x + delta_x), (int32_t)[capture pixelWidth] - 1);
  pixel_y = MIN(MAX(0, pixel_y + delta_y), (int32_t)[capture pixelHeight] - 1);
  NSPoint next = NSMakePoint(
      NSMinX(capture->logical_bounds) + ((CGFloat)pixel_x + 0.5) * NSWidth(capture->logical_bounds) / (CGFloat)[capture pixelWidth],
      NSMinY(capture->logical_bounds) + ((CGFloat)pixel_y + 0.5) * NSHeight(capture->logical_bounds) / (CGFloat)[capture pixelHeight]);
  if (CGWarpMouseCursorPosition(CGPointMake(next.x, next.y)) != kCGErrorSuccess) {
    return;
  }
  [self updateHoverAt:next visible:YES];
}

- (void)completeCancelled:(BOOL)cancelled selection:(NSRect)selection {
  [self completeCancelled:cancelled selection:selection copiedColor:nil];
}

- (void)completeCancelled:(BOOL)cancelled selection:(NSRect)selection copiedColor:(NSString *)copied_color {
  if (_completed) {
    return;
  }
  if (copied_color.length == 0 && !cancelled && (NSWidth(selection) < 2.0 || NSHeight(selection) < 2.0)) {
    cancelled = YES;
  }
  wox_darwin_log_window_state("selector_finished", self, [NSString stringWithFormat:
      @"cancelled=%d copiedColor=%d selectionPoints=%@", cancelled, copied_color.length > 0, NSStringFromRect(selection)]);
  _completed = YES;
  [_object_selector stop];
  _cancelled = cancelled && copied_color.length == 0;
  _dragging = NO;
  [_copied_color release];
  _copied_color = [copied_color copy];
  if (_event_monitor != nil) {
    [NSEvent removeMonitor:_event_monitor];
    _event_monitor = nil;
  }
  if (!cancelled && copied_color.length == 0) {
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
                                NSEventMaskMouseMoved |
                                NSEventMaskScrollWheel |
                                NSEventMaskKeyDown
                                                        handler:^NSEvent *(NSEvent *event) {
    if (session->_completed) {
      return nil;
    }
    // A local monitor also sees Settings events when an overlay is missing from its Space.
    // Only the selector's own windows may start a selection or consume keyboard input.
    if (![session->_windows containsObject:event.window]) {
      return event;
    }
    if (event.type == NSEventTypeKeyDown) {
      if (event.keyCode == 53) {
        wox_darwin_log_window_state("selector_escape", session, [NSString stringWithFormat:@"eventWindow=%ld", (long)event.windowNumber]);
      }
      [session handleKeyEvent:event];
      return nil;
    }
    NSPoint mouse_location = [session topLeftMouseLocation];
    if (event.type == NSEventTypeMouseMoved && !session->_dragging) {
      [session updateHoverAt:mouse_location visible:YES];
      [session makeKeyForPoint:mouse_location];
      return nil;
    }
    if (event.type == NSEventTypeScrollWheel && !session->_dragging) {
      [session updateHoverAt:mouse_location visible:YES];
      if (event.scrollingDeltaY != 0) [session->_object_selector step:event.scrollingDeltaY > 0 ? 1 : -1];
      return nil;
    }
    if (event.type == NSEventTypeLeftMouseDown) {
      wox_darwin_log_window_state("selector_mouse_down", session, [NSString stringWithFormat:@"eventWindow=%ld point=%@", (long)event.windowNumber, NSStringFromPoint(mouse_location)]);
      session->_drag_capture = [session captureAtPoint:mouse_location];
      if (session->_drag_capture == nil) {
        return nil;
      }
      NSPoint point = [session clampPoint:mouse_location toBounds:session->_drag_capture->logical_bounds];
      session->_drag_start = point;
      [session updateHoverAt:point visible:YES];
      session->_pressed_window = session->_object_selector.target;
      [session->_object_selector stop];
      session->_selection_dragged = NO;
      session->_dragging = YES;
      [session updateHoverAt:point visible:NO];
      [session updateSelection:[session rectFromStart:point end:point] visible:YES];
      return nil;
    }
    if (event.type == NSEventTypeLeftMouseDragged && session->_dragging) {
      // The portable annotation editor owns one display after handoff. Keeping the native drag on
      // its starting display avoids presenting a cross-display selection that export would crop.
      NSPoint point = [session clampPoint:mouse_location toBounds:session->_drag_capture->logical_bounds];
      session->_selection_dragged = session->_selection_dragged ||
          hypot(point.x - session->_drag_start.x, point.y - session->_drag_start.y) >= 4.0;
      [session updateSelection:[session rectFromStart:session->_drag_start end:point] visible:YES];
      return nil;
    }
    if (event.type == NSEventTypeLeftMouseUp && session->_dragging) {
      NSPoint point = [session clampPoint:mouse_location toBounds:session->_drag_capture->logical_bounds];
      NSRect selection = [session rectFromStart:session->_drag_start end:point];
      // Ignore click jitter, but never snap a gesture that previously crossed the drag threshold.
      if (!session->_selection_dragged && hypot(point.x - session->_drag_start.x, point.y - session->_drag_start.y) < 4.0 &&
          !NSIsEmptyRect(session->_pressed_window)) {
        selection = session->_pressed_window;
        if (NSEqualRects(selection, [session windowAtPoint:session->_drag_start]))
          session->_selected_window_image = screenshot_window_image(session->_window_candidates, selection, session->_drag_start);
        if (session->_selected_window_image == NULL && NSEqualRects(selection, [session windowAtPoint:session->_drag_start])) {
          woxGoDarwinScreenshotDiagnostic("stage=window_alpha_capture_unavailable");
        }
      }
      [session updateSelection:selection visible:YES];
      [session completeCancelled:NO selection:selection];
      return nil;
    }
    return event;
  }];
  for (WoxOverlayPanel *window in _windows) {
    [window orderFrontRegardless];
  }
  NSPoint mouse_location = [self topLeftMouseLocation];
  [self updateHoverAt:mouse_location visible:YES];
  [self makeKeyForPoint:mouse_location];
  if (NSApp.keyWindow == nil && _windows.count > 0) {
    WoxOverlayPanel *first = _windows.firstObject;
    [first makeKeyAndOrderFront:nil];
    [first makeFirstResponder:first.contentView.subviews.firstObject];
  }
  wox_darwin_log_window_state("selector_shown", self, nil);
  // Space membership and occlusion can settle after ordering returns. The block retains
  // the session until this one-shot sample and skips selectors already handed off or dismissed.
  dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 150 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
    if (!session->_completed && !session->_dismissed) {
      wox_darwin_log_window_state("selector_settled", session, nil);
    }
  });
}

- (void)dismiss {
  if (_dismissed) {
    return;
  }
  _dismissed = YES;
  [_object_selector stop];
  if (_event_monitor != nil) {
    [NSEvent removeMonitor:_event_monitor];
    _event_monitor = nil;
  }
  // Drop the full-screen capture while the layer is still in the window, then
  // order out. orderOut while layer.contents still holds that image keeps a
  // display-sized mapping after the selector's own CGImage is released.
  [CATransaction begin];
  [CATransaction setDisableActions:YES];
  for (WoxOverlayPanel *window in _windows) {
    window.contentView.layer.contents = nil;
  }
  [CATransaction commit];
  [CATransaction flush];
  for (WoxOverlayPanel *window in _windows) {
    window.contentView = nil;
    [window orderOut:nil];
    [window close];
  }
}

- (BOOL)cancelled {
  return _cancelled;
}

- (NSString *)copiedColor {
  return _copied_color;
}

- (WoxScreenshotDisplayCapture *)selectedCapture {
  return _selected_capture;
}

- (NSRect)selection {
  return _selection;
}

- (CGImageRef)selectedWindowImage {
  return _selected_window_image;
}

- (dispatch_semaphore_t)completion {
  return _completion;
}

- (void)dealloc {
  [self dismiss];
  [_copied_color release];
  [_selected_capture release];
  [_windows release];
  [_window_candidates release];
  [_object_selector release];
  if (_selected_window_image != NULL) CGImageRelease(_selected_window_image);
  [_captures release];
#if !OS_OBJECT_USE_OBJC
  dispatch_release(_completion);
#endif
  [super dealloc];
}
@end

int32_t wox_darwin_select_screenshot_region(
    int32_t *pixel_width,
    int32_t *pixel_height,
    uintptr_t *session_handle,
    uint32_t *display_id,
    float *display_x,
    float *display_y,
    float *display_width,
    float *display_height,
    float *selection_x,
    float *selection_y,
    float *selection_width,
    float *selection_height,
    char **copied_color) {
  if (pixel_width == NULL || pixel_height == NULL || session_handle == NULL || display_id == NULL || display_x == NULL || display_y == NULL || display_width == NULL || display_height == NULL ||
      selection_x == NULL || selection_y == NULL || selection_width == NULL || selection_height == NULL || copied_color == NULL || [NSThread isMainThread]) {
    return -1;
  }
  *copied_color = NULL;
  if (@available(macOS 12.0, *)) {
    if (!CGPreflightScreenCaptureAccess()) {
      return -2;
    }
  }

  __block WoxScreenshotSelectionSession *session = nil;
  __block int32_t setup_result = 0;
  wox_darwin_dispatch_sync(^{
    wox_darwin_log_window_state("before_capture", NULL, nil);
    NSArray<NSScreen *> *screens = [NSScreen screens];
    NSMutableArray *captures = [NSMutableArray arrayWithCapacity:screens.count];
    CGFloat top = wox_darwin_desktop_top();
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
  if ([session copiedColor].length > 0) {
    const char *value = [session copiedColor].UTF8String;
    *copied_color = value != NULL ? strdup(value) : NULL;
    wox_darwin_dispatch_sync(^{
      [session dismiss];
    });
    [session release];
    return *copied_color != NULL ? 2 : -1;
  }
  if ([session cancelled]) {
    wox_darwin_dispatch_sync(^{
      [session dismiss];
    });
    [session release];
    return 1;
  }

  WoxScreenshotDisplayCapture *capture = [session selectedCapture];
  NSRect selection = [session selection];
  size_t width = CGImageGetWidth(capture->image);
  size_t height = CGImageGetHeight(capture->image);
  int32_t result = width > 0 && height > 0 && width <= INT32_MAX && height <= INT32_MAX ? 0 : -1;
  if (result == 0) {
    *pixel_width = (int32_t)width;
    *pixel_height = (int32_t)height;
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
    wox_darwin_dispatch_sync(^{
      [session dismiss];
    });
    [session release];
  }
  return result;
}

// Normalize cached display pixels to the editor's premultiplied sRGB RGBA layout.
static int32_t copy_screenshot_image_rgba(CGImageRef image, int32_t width, int32_t height, void *pixels) {
  CGColorSpaceRef color_space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
  CGContextRef context = CGBitmapContextCreate(pixels, width, height, 8, (size_t)width * 4, color_space,
                                              kCGBitmapByteOrder32Big | kCGImageAlphaPremultipliedLast);
  CGColorSpaceRelease(color_space);
  if (context == NULL) {
    return -1;
  }
  CGContextSetBlendMode(context, kCGBlendModeCopy);
  CGContextDrawImage(context, CGRectMake(0, 0, width, height), image);
  CGContextRelease(context);
  return 0;
}

// Transfer the cached display into Go-owned RGBA memory before dismissing the selector.
// Pixel dimensions come from CGImage, independently of the display's logical bounds.
int32_t wox_darwin_copy_screenshot_selection_rgba(uintptr_t session_handle, int32_t width, int32_t height, void *pixels) {
  if (session_handle == 0 || pixels == NULL || width <= 0 || height <= 0) {
    return -1;
  }
  WoxScreenshotDisplayCapture *capture = [(WoxScreenshotSelectionSession *)session_handle selectedCapture];
  if (capture == nil || CGImageGetWidth(capture->image) != (size_t)width || CGImageGetHeight(capture->image) != (size_t)height) {
    return -1;
  }
  return copy_screenshot_image_rgba(capture->image, width, height, pixels);
}

// Query or copy the native window image while the retained selector session owns its pixels.
int32_t wox_darwin_copy_screenshot_window_rgba(uintptr_t session_handle, int32_t *width, int32_t *height, void *pixels) {
  if (session_handle == 0 || width == NULL || height == NULL) return -1;
  CGImageRef image = [(WoxScreenshotSelectionSession *)session_handle selectedWindowImage];
  if (image == NULL) return 1;
  int32_t image_width = (int32_t)CGImageGetWidth(image), image_height = (int32_t)CGImageGetHeight(image);
  if (image_width <= 0 || image_height <= 0 || image_width > 16384 || image_height > 16384) return -1;
  if (pixels == NULL) {
    *width = image_width;
    *height = image_height;
    return 0;
  }
  if (*width != image_width || *height != image_height) return -1;
  return copy_screenshot_image_rgba(image, image_width, image_height, pixels);
}

// Exercise the production conversion with asymmetric colors and alpha without screen permissions.
int32_t wox_darwin_test_screenshot_rgba(void *pixels) {
  uint8_t source[] = {255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 255, 64, 32, 16, 128};
  CGColorSpaceRef color_space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
  CGContextRef context = CGBitmapContextCreate(source, 2, 2, 8, 8, color_space,
                                              kCGBitmapByteOrder32Big | kCGImageAlphaPremultipliedLast);
  CGColorSpaceRelease(color_space);
  if (context == NULL) return -1;
  CGImageRef image = CGBitmapContextCreateImage(context);
  CGContextRelease(context);
  if (image == NULL) return -1;
  int32_t result = copy_screenshot_image_rgba(image, 2, 2, pixels);
  CGImageRelease(image);
  return result;
}

void wox_darwin_dismiss_screenshot_selection(uintptr_t session_handle) {
  if (session_handle == 0) {
    return;
  }
  WoxScreenshotSelectionSession *session = (WoxScreenshotSelectionSession *)session_handle;
  wox_darwin_dispatch_sync(^{
    [session dismiss];
  });
  [session release];
}

uintptr_t wox_darwin_show_screenshot_border(float x, float y, float width, float height, float thickness) {
  if (width <= 0.0f || height <= 0.0f || thickness <= 0.0f) {
    return 0;
  }
  __block NSMutableArray *windows = nil;
  wox_darwin_dispatch_sync(^{
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
      NSRect frame = NSMakeRect(NSMinX(edge), wox_darwin_desktop_top() - NSMaxY(edge), NSWidth(edge), NSHeight(edge));
      WoxOverlayPanel *window = [[WoxOverlayPanel alloc]
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
  wox_darwin_dispatch_sync(^{
    for (WoxOverlayPanel *window in windows) {
      [window orderOut:nil];
      [window close];
    }
  });
  [windows release];
}

// wox_darwin_test_screenshot_pixel_at_point exposes the native selector's pixel mapping for Go tests.
int32_t wox_darwin_test_screenshot_pixel_at_point(
    int32_t image_width,
    int32_t image_height,
    float frame_width,
    float frame_height,
    float x,
    float y,
    int32_t *pixel_x,
    int32_t *pixel_y) {
  return wox_screenshot_pixel_at_point(image_width, image_height, frame_width, frame_height, x, y, pixel_x, pixel_y) ? 0 : -1;
}

// wox_darwin_test_screenshot_inspector_rect exposes the native selector's inspector placement for Go tests.
int32_t wox_darwin_test_screenshot_inspector_rect(
    float frame_width,
    float frame_height,
    float pointer_x,
    float pointer_y,
    float panel_width,
    float panel_height,
    float ui_scale,
    float *x,
    float *y,
    float *width,
    float *height) {
  if (x == NULL || y == NULL || width == NULL || height == NULL) {
    return -1;
  }
  NSRect panel = wox_screenshot_inspector_rect(
      NSMakeSize(frame_width, frame_height),
      NSMakePoint(pointer_x, pointer_y),
      NSMakeSize(panel_width, panel_height),
      ui_scale);
  *x = (float)NSMinX(panel);
  *y = (float)NSMinY(panel);
  *width = (float)NSWidth(panel);
  *height = (float)NSHeight(panel);
  return 0;
}

int32_t wox_darwin_test_screenshot_color_shortcut(uint16_t key_code, int32_t *as_hex) {
  if (as_hex == NULL) {
    return -1;
  }
  bool hex = false;
  if (!wox_screenshot_color_shortcut(key_code, &hex)) {
    return -1;
  }
  *as_hex = hex ? 1 : 0;
  return 0;
}

