//go:build darwin

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static NSWindow *smoke_window;
static NSView *smoke_render_view;
static WKWebView *smoke_web_view;
// 0 = pending, 1 = transparent snapshot, -1 = snapshot missing expected pixels.
static int smoke_snapshot_result;

static NSView *find_view(NSView *view, Class cls) {
  if ([view isKindOfClass:cls]) return view;
  for (NSView *child in view.subviews) {
    NSView *found = find_view(child, cls);
    if (found != nil) return found;
  }
  return nil;
}

// Exercise native callbacks without assuming NSApp.currentEvent is the tracking
// event. AppKit can synthesize enter/exit callbacks outside queued event delivery.
int woxSmokeInstallInput(const char *title) {
  NSString *name = [NSString stringWithUTF8String:title];
  for (NSWindow *window in NSApp.windows) {
    if ([window.title isEqualToString:name]) {
      smoke_window = window;
      break;
    }
  }
  if (smoke_window == nil) return 0;
  smoke_render_view = find_view(smoke_window.contentView, NSClassFromString(@"WoxRenderView"));
  smoke_web_view = (WKWebView *)find_view(smoke_window.contentView, [WKWebView class]);
  if (smoke_render_view == nil || smoke_web_view == nil) return 0;
  return 1;
}

// Dispatch a callback and verify the nonactivating window never becomes key.
int woxSmokeDispatchPointer(int kind, double x, double y) {
  if (smoke_window == nil || smoke_render_view == nil || smoke_window.isKeyWindow) return 0;
  NSPoint location = [smoke_render_view convertPoint:NSMakePoint(x, y) toView:nil];
  NSTimeInterval now = [NSProcessInfo processInfo].systemUptime;
  NSEvent *event = nil;
  if (kind == 1 || kind == 4) {
    event = [NSEvent enterExitEventWithType:kind == 1 ? NSEventTypeMouseExited : NSEventTypeMouseEntered location:location modifierFlags:0
                                 timestamp:now windowNumber:smoke_window.windowNumber context:nil
                               eventNumber:0 trackingNumber:0 userData:NULL];
  } else {
    NSEventType type = kind == 0 ? NSEventTypeMouseMoved : kind == 2 ? NSEventTypeLeftMouseDown : NSEventTypeLeftMouseUp;
    event = [NSEvent mouseEventWithType:type location:location modifierFlags:0 timestamp:now
                           windowNumber:smoke_window.windowNumber context:nil eventNumber:0
                             clickCount:1 pressure:kind == 2 ? 1.0 : 0.0];
  }
  if (event == nil) return 0;
  switch (event.type) {
  case NSEventTypeMouseMoved: [smoke_render_view mouseMoved:event]; break;
  case NSEventTypeMouseEntered: [smoke_render_view mouseEntered:event]; break;
  case NSEventTypeMouseExited: [smoke_render_view mouseExited:event]; break;
  case NSEventTypeLeftMouseDown: [smoke_render_view mouseDown:event]; break;
  case NSEventTypeLeftMouseUp: [smoke_render_view mouseUp:event]; break;
  default: return 0;
  }
  return !smoke_window.isKeyWindow;
}

// Inspect the rendered WebKit image rather than relying on the background setting alone.
int woxSmokeStartSnapshot(void) {
  if (smoke_web_view == nil) return 0;
  smoke_snapshot_result = 0;
  [smoke_web_view takeSnapshotWithConfiguration:nil completionHandler:^(NSImage *image, NSError *error) {
    smoke_snapshot_result = -1;
    if (image == nil || error != nil) return;
    NSRect rect = NSMakeRect(0, 0, image.size.width, image.size.height);
    CGImageRef cg_image = [image CGImageForProposedRect:&rect context:nil hints:nil];
    if (cg_image == NULL) return;
    NSBitmapImageRep *bitmap = [[[NSBitmapImageRep alloc] initWithCGImage:cg_image] autorelease];
    BOOL transparent = NO;
    BOOL red = NO;
    for (NSInteger y = 0; y < bitmap.pixelsHigh; y += 2) {
      for (NSInteger x = 0; x < bitmap.pixelsWide; x += 2) {
        NSColor *color = [[bitmap colorAtX:x y:y] colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
        if (color == nil) continue;
        if (color.alphaComponent < 0.05) transparent = YES;
        // Color-managed snapshots can give CSS red a small green component on wide-gamut displays.
        if (color.alphaComponent > 0.95 && color.redComponent > 0.9 && color.greenComponent < 0.25 && color.blueComponent < 0.25) red = YES;
      }
    }
    if (transparent && red) smoke_snapshot_result = 1;
  }];
  return 1;
}

int woxSmokeSnapshotResult(void) { return smoke_snapshot_result; }

void woxSmokeRemoveInput(void) {
  smoke_window = nil;
  smoke_render_view = nil;
  smoke_web_view = nil;
}
