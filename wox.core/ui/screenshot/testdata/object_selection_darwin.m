#import <ApplicationServices/ApplicationServices.h>
#import "../object_selection_darwin.h"
#include <stdatomic.h>
#include <unistd.h>

// Real AX reference types exercise ownership while a deterministic provider avoids permission prompts and foreign IPC.
static AXUIElementRef fixture_window, fixture_group, fixture_button;
static NSRect fixture_window_frame;
static BOOL fixture_enabled, fixture_move_during_query, fixture_cancel_during_query;
static NSUInteger fixture_writes;
static NSUInteger fixture_hits;
static pid_t fixture_owner;
static atomic_uint *fixture_generation;

static Boolean fixture_trusted(void) { return true; }
static AXError fixture_timeout(AXUIElementRef element, float timeout) { (void)element; (void)timeout; return kAXErrorSuccess; }

// Supply only the geometry, parent links, and application flags consumed by the production selector.
static AXError fixture_attribute(AXUIElementRef element, CFStringRef name, CFTypeRef *value) {
  *value = NULL;
  if (CFEqual(name, CFSTR("AXManualAccessibility"))) {
    *value = CFRetain(fixture_enabled ? kCFBooleanTrue : kCFBooleanFalse);
    return kAXErrorSuccess;
  }
  NSRect frame = CFEqual(element, fixture_window) ? fixture_window_frame :
      (CFEqual(element, fixture_group) ? NSMakeRect(-100, 50, 200, 100) : NSMakeRect(-50, 70, 40, 30));
  if (CFEqual(name, kAXPositionAttribute)) {
    CGPoint point = NSPointToCGPoint(frame.origin);
    *value = AXValueCreate(kAXValueCGPointType, &point);
  } else if (CFEqual(name, kAXSizeAttribute)) {
    CGSize size = NSSizeToCGSize(frame.size);
    *value = AXValueCreate(kAXValueCGSizeType, &size);
  } else if (CFEqual(name, kAXRoleAttribute)) {
    *value = CFRetain(CFEqual(element, fixture_window) ? kAXWindowRole : kAXGroupRole);
  } else if (CFEqual(name, kAXParentAttribute)) {
    if (!CFEqual(element, fixture_window)) *value = CFRetain(CFEqual(element, fixture_group) ? fixture_window : fixture_group);
  }
  return *value == NULL ? kAXErrorAttributeUnsupported : kAXErrorSuccess;
}

static AXError fixture_settable(AXUIElementRef element, CFStringRef name, Boolean *settable) {
  (void)element;
  *settable = CFEqual(name, CFSTR("AXManualAccessibility"));
  return kAXErrorSuccess;
}

// Simulate providers that successfully change the flag but return NotImplemented.
static AXError fixture_set(AXUIElementRef element, CFStringRef name, CFTypeRef value) {
  (void)element; (void)name;
  fixture_enabled = CFEqual(value, kCFBooleanTrue);
  fixture_writes++;
  return fixture_enabled ? kAXErrorNotImplemented : kAXErrorSuccess;
}

static AXError fixture_hit(AXUIElementRef application, float x, float y, AXUIElementRef *hit) {
  (void)application; (void)x; (void)y;
  fixture_hits++;
  *hit = (AXUIElementRef)CFRetain(fixture_group);
  return kAXErrorSuccess;
}

static AXError fixture_pid(AXUIElementRef element, pid_t *pid) { (void)element; *pid = fixture_owner; return kAXErrorSuccess; }

// The coarse hit exposes a smaller child; mutations simulate movement or cancellation during refinement.
static AXError fixture_children(AXUIElementRef element, CFStringRef name, CFIndex start, CFIndex limit, CFArrayRef *children) {
  (void)name; (void)limit;
  if (fixture_move_during_query) fixture_window_frame.origin.x += 10;
  if (fixture_cancel_during_query) atomic_fetch_add(fixture_generation, 1);
  const void *child = fixture_button;
  *children = CFArrayCreate(NULL, &child, CFEqual(element, fixture_group) && start == 0 ? 1 : 0, &kCFTypeArrayCallBacks);
  return kAXErrorSuccess;
}

#define AXIsProcessTrusted fixture_trusted
#define AXUIElementSetMessagingTimeout fixture_timeout
#define AXUIElementCopyAttributeValue fixture_attribute
#define AXUIElementIsAttributeSettable fixture_settable
#define AXUIElementSetAttributeValue fixture_set
#define AXUIElementCopyElementAtPosition fixture_hit
#define AXUIElementGetPid fixture_pid
#define AXUIElementCopyAttributeValues fixture_children
#import "../object_selection_darwin.m"

void woxGoDarwinScreenshotDiagnostic(const char *message) { (void)message; }

// Check bounded native refinement, origin conversion, stale rejection, and shared flag restoration using the production implementation.
static int fixture_provider_tests(void) {
  fixture_owner = getpid();
  fixture_window = AXUIElementCreateApplication(getpid()+1);
  fixture_group = AXUIElementCreateApplication(getpid()+2);
  fixture_button = AXUIElementCreateApplication(getpid()+3);
  fixture_window_frame = NSMakeRect(-200, 40, 400, 300);
  atomic_uint generation;
  atomic_init(&generation, 1);
  fixture_generation = &generation;
  NSMutableDictionary *first = [NSMutableDictionary dictionary], *second = [NSMutableDictionary dictionary];
  AXUIElementRef application = AXUIElementCreateApplication(getpid());
  selector_activate(application, getpid(), first, CACurrentMediaTime()+1, &generation, 1);
  selector_activate(application, getpid(), second, CACurrentMediaTime()+1, &generation, 1);
  if (!fixture_enabled || fixture_writes != 1 || selector_leases().count != 1) return 10;
  NSRect frozen = NSOffsetRect(fixture_window_frame, 0, 600);
  NSPoint point = NSMakePoint(-30, 680);
  NSArray *path = selector_hit_path(getpid(), frozen, frozen, point, 600, 1.5, &generation, 1, first);
  if (path.count != 3 || !NSEqualRects([path.firstObject rectValue], NSMakeRect(-50, 670, 40, 30))) return 11;
  fixture_move_during_query = YES;
  if (selector_hit_path(getpid(), frozen, frozen, point, 600, 1.5, &generation, 1, first).count != 0) return 12;
  fixture_move_during_query = NO;
  fixture_window_frame = NSMakeRect(-200, 40, 400, 300);
  fixture_cancel_during_query = YES;
  if (selector_hit_path(getpid(), frozen, frozen, point, 600, 1.5, &generation, 1, first).count != 0) return 13;
  fixture_cancel_during_query = NO;
  selector_restore(first);
  if (!fixture_enabled || fixture_writes != 1) return 14;
  selector_restore(second);
  if (fixture_enabled || fixture_writes != 2 || selector_leases().count != 0) return 15;
  fixture_enabled = YES;
  selector_activate(application, getpid(), first, CACurrentMediaTime()+1, &generation, 2);
  selector_restore(first);
  if (!fixture_enabled || fixture_writes != 2) return 16;

  // Drive the real AppKit coordinator through two controls. The first foreground hit must precede the 80ms refinement timer.
  fixture_owner = getpid()+1;
  NSDictionary *bounds = (NSDictionary *)CGRectCreateDictionaryRepresentation(NSRectToCGRect(frozen));
  NSArray *candidates = wox_screenshot_selectable_windows(@[@{(id)kCGWindowBounds: bounds, (id)kCGWindowOwnerPID: @(fixture_owner),
      (id)kCGWindowNumber: @1, (id)kCGWindowLayer: @(NSFloatingWindowLevel), (id)kCGWindowAlpha: @1}], nil);
  [bounds release];
  NSMutableArray *updates = [NSMutableArray array];
  WoxScreenshotObjectSelector *selector = [[WoxScreenshotObjectSelector alloc] initWithCandidates:candidates primaryOffset:600 update:^(NSRect rect) {
    [updates addObject:[NSValue valueWithRect:rect]];
  }];
  [selector hoverAt:point displayBounds:frozen];
  CFTimeInterval deadline = CACurrentMediaTime()+0.06;
  while (updates.count < 2 && CACurrentMediaTime() < deadline)
    [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.001]];
  if (updates.count != 2 || !NSEqualRects([updates.lastObject rectValue], NSMakeRect(-50, 670, 40, 30))) return 17;
  [selector hoverAt:NSMakePoint(-60, 680) displayBounds:frozen];
  if (updates.count != 2) return 18;
  deadline = CACurrentMediaTime()+0.06;
  while (updates.count < 3 && CACurrentMediaTime() < deadline)
    [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.001]];
  if (updates.count != 3 || !NSEqualRects([updates.lastObject rectValue], NSMakeRect(-100, 650, 200, 100))) return 19;
  [selector step:1];
  [selector hoverAt:NSMakePoint(-70, 680) displayBounds:frozen];
  deadline = CACurrentMediaTime()+0.06;
  while (updates.count < 6 && CACurrentMediaTime() < deadline)
    [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.001]];
  if (updates.count < 6 || !NSEqualRects(selector.target, frozen)) return 20;
  [selector stop];
  dispatch_sync(selector_worker(), ^{});
  [selector release];
  // A visible window owned by this process must select its frozen frame without querying the covering selector through AX.
  NSMutableDictionary *own_window = [[candidates.firstObject mutableCopy] autorelease];
  own_window[(id)kCGWindowOwnerPID] = @(getpid());
  NSUInteger hits = fixture_hits;
  selector = [[WoxScreenshotObjectSelector alloc] initWithCandidates:@[own_window] primaryOffset:600 update:nil];
  [selector hoverAt:point displayBounds:frozen];
  if (!NSEqualRects(selector.target, frozen)) return 21;
  deadline = CACurrentMediaTime()+0.1;
  while (CACurrentMediaTime() < deadline)
    [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.001]];
  dispatch_sync(selector_worker(), ^{});
  if (fixture_hits != hits || !NSEqualRects(selector.target, frozen)) return 22;
  [selector stop];
  dispatch_sync(selector_worker(), ^{});
  [selector release];
  CFRelease(application); CFRelease(fixture_window); CFRelease(fixture_group); CFRelease(fixture_button);
  return 0;
}

// Empty desktop selects only the pointer's logical display; gaps and points outside that display stay unselectable.
static int fixture_display_fallback_tests(void) {
  NSMutableArray *updates = [NSMutableArray array];
  WoxScreenshotObjectSelector *selector = [[WoxScreenshotObjectSelector alloc] initWithCandidates:@[] primaryOffset:600 update:^(NSRect rect) {
    [updates addObject:[NSValue valueWithRect:rect]];
  }];
  NSRect left = NSMakeRect(-1600, 100, 1600, 900), right = NSMakeRect(0, -600, 1800, 1400);
  [selector hoverAt:NSMakePoint(-1500, 120) displayBounds:left];
  if (!NSEqualRects(selector.target, left) || updates.count != 1) return 30;
  [selector step:1];
  if (!NSEqualRects(selector.target, left)) return 31;
  [selector hoverAt:NSMakePoint(100, -500) displayBounds:right];
  if (!NSEqualRects(selector.target, right) || !NSEqualRects([updates.lastObject rectValue], right)) return 32;
  [selector hoverAt:NSMakePoint(-100, -500) displayBounds:right];
  if (!NSIsEmptyRect(selector.target) || !NSIsEmptyRect([updates.lastObject rectValue])) return 33;
  [selector stop];
  dispatch_sync(selector_worker(), ^{});
  [selector release];
  return 0;
}

int main(void) {
  @autoreleasepool {
    int status = wox_darwin_test_screenshot_object_selection();
    if (status == 0) status = fixture_display_fallback_tests();
    return status == 0 ? fixture_provider_tests() : status;
  }
}
