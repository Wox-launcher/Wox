//go:build darwin

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static const NSInteger smoke_event_number = 0x574f58;
static NSWindow *smoke_window;
static NSView *smoke_render_view;
static WKWebView *smoke_web_view;
static id smoke_monitor;
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

// Route synthetic AppKit events to the real render view while NSApp.currentEvent
// is set, without moving the desktop pointer or requiring accessibility access.
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
  NSEventMask mask = NSEventMaskMouseMoved | NSEventMaskMouseExited | NSEventMaskLeftMouseDown | NSEventMaskLeftMouseUp;
  smoke_monitor = [NSEvent addLocalMonitorForEventsMatchingMask:mask handler:^NSEvent *(NSEvent *event) {
    if (event.windowNumber != smoke_window.windowNumber || event.eventNumber != smoke_event_number) return event;
    switch (event.type) {
    case NSEventTypeMouseMoved: [smoke_render_view mouseMoved:event]; break;
    case NSEventTypeMouseExited: [smoke_render_view mouseExited:event]; break;
    case NSEventTypeLeftMouseDown: [smoke_render_view mouseDown:event]; break;
    case NSEventTypeLeftMouseUp: [smoke_render_view mouseUp:event]; break;
    default: return event;
    }
    return nil;
  }];
  return smoke_monitor != nil;
}

int woxSmokePostPointer(int kind, double x, double y) {
  if (smoke_window == nil || smoke_render_view == nil) return 0;
  NSPoint location = [smoke_render_view convertPoint:NSMakePoint(x, y) toView:nil];
  NSTimeInterval now = [NSProcessInfo processInfo].systemUptime;
  NSEvent *event = nil;
  if (kind == 1) {
    event = [NSEvent enterExitEventWithType:NSEventTypeMouseExited location:location modifierFlags:0
                                 timestamp:now windowNumber:smoke_window.windowNumber context:nil
                               eventNumber:smoke_event_number trackingNumber:0 userData:NULL];
  } else {
    NSEventType type = kind == 0 ? NSEventTypeMouseMoved : kind == 2 ? NSEventTypeLeftMouseDown : NSEventTypeLeftMouseUp;
    event = [NSEvent mouseEventWithType:type location:location modifierFlags:0 timestamp:now
                           windowNumber:smoke_window.windowNumber context:nil eventNumber:smoke_event_number
                             clickCount:1 pressure:kind == 2 ? 1.0 : 0.0];
  }
  if (event == nil) return 0;
  [NSApp postEvent:event atStart:NO];
  return 1;
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
        NSColor *color = [[bitmap colorAtX:x y:y] colorUsingColorSpace:[NSColorSpace deviceRGBColorSpace]];
        if (color == nil) continue;
        if (color.alphaComponent < 0.05) transparent = YES;
        if (color.alphaComponent > 0.95 && color.redComponent > 0.9 && color.greenComponent < 0.1 && color.blueComponent < 0.1) red = YES;
      }
    }
    if (transparent && red) smoke_snapshot_result = 1;
  }];
  return 1;
}

int woxSmokeSnapshotResult(void) { return smoke_snapshot_result; }

void woxSmokeRemoveInput(void) {
  if (smoke_monitor != nil) [NSEvent removeMonitor:smoke_monitor];
  smoke_monitor = nil;
  smoke_window = nil;
  smoke_render_view = nil;
  smoke_web_view = nil;
}
