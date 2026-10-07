//go:build darwin

#import "object_selection_darwin.h"
#import <ApplicationServices/ApplicationServices.h>
#include <stdatomic.h>
#include <libproc.h>
#include <math.h>
#include <string.h>
#include <unistd.h>

extern void woxGoDarwinScreenshotDiagnostic(const char *message);

// Keep floating application surfaces in WindowServer order; level alone does not identify a capture overlay or the Dock.
NSArray *wox_screenshot_selectable_windows(NSArray *windows, NSSet *excluded) {
  NSMutableArray *candidates = [NSMutableArray arrayWithCapacity:windows.count];
  NSMutableDictionary *dock_owners = [NSMutableDictionary dictionary];
  NSInteger dock_level = CGWindowLevelForKey(kCGDockWindowLevelKey);
  for (NSDictionary *entry in windows) {
    NSNumber *identifier = entry[(id)kCGWindowNumber];
    NSNumber *pid = entry[(id)kCGWindowOwnerPID];
    NSNumber *layer = entry[(id)kCGWindowLayer];
    double alpha = [entry[(id)kCGWindowAlpha] doubleValue];
    CFDictionaryRef bounds = (CFDictionaryRef)entry[(id)kCGWindowBounds];
    CGRect frame;
    if (identifier.unsignedIntValue == 0 || pid.intValue <= 0 || layer == nil || layer.integerValue < 0 ||
        !isfinite(alpha) || alpha <= 0 || [excluded containsObject:identifier] ||
        bounds == NULL || !CGRectMakeWithDictionaryRepresentation(bounds, &frame) ||
        !isfinite(frame.origin.x) || !isfinite(frame.origin.y) || !isfinite(frame.size.width) || !isfinite(frame.size.height) ||
        frame.size.width < 2 || frame.size.height < 2) continue;
    if (layer.integerValue == dock_level) {
      NSNumber *dock = dock_owners[pid];
      if (dock == nil) {
        char path[PROC_PIDPATHINFO_MAXSIZE] = {0};
        dock = @(proc_pidpath(pid.intValue, path, sizeof(path)) > 0 &&
            strcmp(path, "/System/Library/CoreServices/Dock.app/Contents/MacOS/Dock") == 0);
        dock_owners[pid] = dock;
      }
      if (dock.boolValue) continue;
    }
    [candidates addObject:entry];
  }
  return candidates;
}

// Retarget layer geometry from its presentation state so interrupted previews never jump backwards.
void wox_screenshot_set_selection_layer_frame(CALayer *layer, CGRect frame, BOOL animated) {
  CALayer *presented = layer.presentationLayer ?: layer;
  CGRect old_bounds = presented.bounds;
  CGPoint old_position = presented.position;
  layer.frame = frame;
  for (NSString *key in @[@"bounds", @"position"]) {
    if (!animated) {
      [layer removeAnimationForKey:key];
      continue;
    }
    CABasicAnimation *animation = [CABasicAnimation animationWithKeyPath:key];
    animation.fromValue = [key isEqualToString:@"bounds"] ? [NSValue valueWithRect:NSRectFromCGRect(old_bounds)] : [NSValue valueWithPoint:NSPointFromCGPoint(old_position)];
    animation.toValue = [layer valueForKeyPath:key];
    animation.duration = 0.101;
    animation.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseOut];
    [layer addAnimation:animation forKey:key];
  }
}

// Each AX call receives its own remaining timeout; no process-wide accessibility timeout is changed.
static BOOL selector_time_left(AXUIElementRef element, CFTimeInterval deadline, atomic_uint *generation, unsigned expected) {
  CFTimeInterval remaining = deadline - CACurrentMediaTime();
  if (remaining <= 0 || atomic_load(generation) != expected) return NO;
  return AXUIElementSetMessagingTimeout(element, (float)MIN(remaining, 0.05)) == kAXErrorSuccess;
}

// Read only geometry and hierarchy; target application text is neither retained nor logged.
static CFTypeRef selector_attribute(AXUIElementRef element, CFStringRef name, CFTimeInterval deadline, atomic_uint *generation, unsigned expected) {
  if (!selector_time_left(element, deadline, generation, expected)) return NULL;
  CFTypeRef value = NULL;
  if (AXUIElementCopyAttributeValue(element, name, &value) != kAXErrorSuccess) {
    if (value != NULL) CFRelease(value);
    return NULL;
  }
  return value;
}

static NSRect selector_frame(AXUIElementRef element, CGFloat offset, CFTimeInterval deadline, atomic_uint *generation, unsigned expected) {
  CFTypeRef position = selector_attribute(element, kAXPositionAttribute, deadline, generation, expected);
  CFTypeRef size = selector_attribute(element, kAXSizeAttribute, deadline, generation, expected);
  CGPoint point = CGPointZero;
  CGSize extent = CGSizeZero;
  BOOL valid = position != NULL && size != NULL && CFGetTypeID(position) == AXValueGetTypeID() && CFGetTypeID(size) == AXValueGetTypeID() &&
      AXValueGetValue(position, kAXValueCGPointType, &point) && AXValueGetValue(size, kAXValueCGSizeType, &extent);
  if (position != NULL) CFRelease(position);
  if (size != NULL) CFRelease(size);
  if (!valid || !isfinite(point.x) || !isfinite(point.y) || !isfinite(extent.width) || !isfinite(extent.height)) return NSZeroRect;
  return NSMakeRect(point.x, point.y + offset, extent.width, extent.height);
}

// Normalize a leaf-to-window path without duplicating equal structural containers.
static NSArray *selector_bounded_path(NSArray *frames, NSRect window, NSPoint point) {
  NSMutableArray *path = [NSMutableArray array];
  for (NSValue *value in frames) {
    NSRect rect = NSIntersectionRect(value.rectValue, window);
    if (NSWidth(rect) < 2 || NSHeight(rect) < 2 || !NSPointInRect(point, rect) || [path containsObject:[NSValue valueWithRect:rect]]) continue;
    if (path.count != 0 && !NSContainsRect(rect, [path.lastObject rectValue])) continue;
    [path addObject:[NSValue valueWithRect:rect]];
  }
  NSValue *outer = [NSValue valueWithRect:window];
  if (!NSIsEmptyRect(window) && ![path containsObject:outer]) [path addObject:outer];
  return path;
}

// PID start time prevents cleanup from changing a replacement process after the original application exits.
static uint64_t selector_process_start(pid_t pid) {
  struct proc_bsdinfo info = {0};
  if (proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info)) != sizeof(info)) return 0;
  return info.pbi_start_tvsec * 1000000 + info.pbi_start_tvusec;
}

// All capture sessions share the AX worker and lease table, so one session cannot restore a flag still used by another.
static dispatch_queue_t selector_worker(void) {
  static dispatch_queue_t worker;
  static dispatch_once_t once;
  dispatch_once(&once, ^{ worker = dispatch_queue_create("wox.screenshot.objects", DISPATCH_QUEUE_SERIAL); });
  return worker;
}

static NSMutableDictionary *selector_leases(void) {
  static NSMutableDictionary *leases;
  static dispatch_once_t once;
  dispatch_once(&once, ^{ leases = [[NSMutableDictionary alloc] init]; });
  return leases;
}

// Electron and Firefox can expose their fine-grained AX tree only after a temporary accessibility flag is enabled.
// Retain value-only leases for this capture; leave flags already enabled by another client untouched.
static void selector_activate(AXUIElementRef application, pid_t pid, NSMutableDictionary *leases,
                              CFTimeInterval deadline, atomic_uint *generation, unsigned expected) {
  if (leases[@(pid)] != nil) return;
  uint64_t start = selector_process_start(pid);
  if (start == 0) return;
  NSMutableDictionary *shared = selector_leases()[@(pid)];
  if (shared != nil && [shared[@"start"] unsignedLongLongValue] == start) {
    shared[@"owners"] = @([shared[@"owners"] unsignedIntegerValue]+1);
    leases[@(pid)] = shared;
    return;
  }
  for (NSString *attribute in @[@"AXManualAccessibility", @"AXEnhancedUserInterface"]) {
    CFTypeRef old = selector_attribute(application, (CFStringRef)attribute, deadline, generation, expected);
    if (old != NULL && CFEqual(old, kCFBooleanTrue)) { CFRelease(old); return; }
    if (old != NULL && CFGetTypeID(old) != CFBooleanGetTypeID()) { CFRelease(old); continue; }
    Boolean settable = false;
    if (!selector_time_left(application, deadline, generation, expected) ||
        AXUIElementIsAttributeSettable(application, (CFStringRef)attribute, &settable) != kAXErrorSuccess || !settable) {
      if (old != NULL) CFRelease(old);
      continue;
    }
    if (!selector_time_left(application, deadline, generation, expected)) { if (old != NULL) CFRelease(old); return; }
    // Register cleanup before the write: a timed-out provider can still apply it after replying.
    NSMutableDictionary *lease = [@{ @"attribute": attribute, @"original": (id)(old ?: kCFBooleanFalse), @"start": @(start), @"owners": @1 } mutableCopy];
    leases[@(pid)] = lease;
    selector_leases()[@(pid)] = lease;
    [lease release];
    AXError changed = AXUIElementSetAttributeValue(application, (CFStringRef)attribute, kCFBooleanTrue);
    // Some providers apply the write but return NotImplemented. Verify it before accepting that reply.
    CFTypeRef actual = selector_attribute(application, (CFStringRef)attribute, deadline, generation, expected);
    BOOL enabled = actual != NULL && CFEqual(actual, kCFBooleanTrue);
    if (actual != NULL) CFRelease(actual);
    if (changed == kAXErrorSuccess || enabled) {
      if (old != NULL) CFRelease(old);
      return;
    }
    if (old != NULL) CFRelease(old);
    return;
  }
}

// Best-effort restoration runs on the same serial worker after cancelled queries finish, never during UI teardown.
static void selector_restore(NSMutableDictionary *leases) {
  for (NSNumber *pid in leases) {
    NSMutableDictionary *lease = leases[pid];
    NSUInteger owners = [lease[@"owners"] unsignedIntegerValue];
    lease[@"owners"] = @(owners-1);
    if (owners > 1) continue;
    if (selector_leases()[pid] == lease) [selector_leases() removeObjectForKey:pid];
    if (selector_process_start(pid.intValue) != [lease[@"start"] unsignedLongLongValue]) continue;
    AXUIElementRef application = AXUIElementCreateApplication(pid.intValue);
    AXUIElementSetMessagingTimeout(application, 0.05);
    AXError restored = AXUIElementSetAttributeValue(application, (CFStringRef)lease[@"attribute"], (CFTypeRef)lease[@"original"]);
    if (restored != kAXErrorSuccess)
      woxGoDarwinScreenshotDiagnostic([NSString stringWithFormat:@"stage=object_selector_restore_failed pid=%d status=%d", pid.intValue, restored].UTF8String);
    CFRelease(application);
  }
  [leases removeAllObjects];
}

// Native hits establish precedence. Bounded child searches refine providers that initially return a coarse container.
static NSArray *selector_hit_path(pid_t pid, NSRect frozen_frame, NSRect visible, NSPoint point, CGFloat offset,
                                  CFTimeInterval budget, atomic_uint *generation, unsigned expected, NSMutableDictionary *leases) {
  if (!AXIsProcessTrusted() || pid <= 0 || atomic_load(generation) != expected) return @[];
  CFTimeInterval deadline = CACurrentMediaTime() + budget;
  AXUIElementRef application = AXUIElementCreateApplication(pid);
  selector_activate(application, pid, leases, deadline, generation, expected);
  AXUIElementRef hit = NULL;
  if (selector_time_left(application, deadline, generation, expected))
    AXUIElementCopyElementAtPosition(application, (float)point.x, (float)(point.y-offset), &hit);
  CFRelease(application);
  if (hit == NULL) return @[];
  NSMutableArray *frames = [NSMutableArray array];
  NSMutableArray *ancestors = [NSMutableArray array];
  AXUIElementRef current = hit;
  BOOL matched_window = NO;
  for (NSUInteger depth = 0; current != NULL && depth < 64; depth++) {
    if ([ancestors containsObject:(id)current]) { CFRelease(current); current = NULL; break; }
    [ancestors addObject:(id)current];
    pid_t owner = 0;
    AXUIElementGetPid(current, &owner);
    if (owner != pid) { CFRelease(current); current = NULL; break; }
    NSRect frame = selector_frame(current, offset, deadline, generation, expected);
    if (!NSIsEmptyRect(frame)) [frames addObject:[NSValue valueWithRect:frame]];
    CFTypeRef role = selector_attribute(current, kAXRoleAttribute, deadline, generation, expected);
    BOOL window = role != NULL && CFEqual(role, kAXWindowRole);
    if (role != NULL) CFRelease(role);
    if (window) {
      // Reject another window or a moved provider tree; the pixels belong to the frozen snapshot.
      matched_window = fabs(NSMinX(frame)-NSMinX(frozen_frame)) <= 2 && fabs(NSMinY(frame)-NSMinY(frozen_frame)) <= 2 &&
          fabs(NSWidth(frame)-NSWidth(frozen_frame)) <= 2 && fabs(NSHeight(frame)-NSHeight(frozen_frame)) <= 2;
      CFRelease(current); current = NULL; break;
    }
    CFTypeRef parent = selector_attribute(current, kAXParentAttribute, deadline, generation, expected);
    CFRelease(current);
    current = parent != NULL && CFGetTypeID(parent) == AXUIElementGetTypeID() ? (AXUIElementRef)parent : NULL;
    if (parent != NULL && current == NULL) CFRelease(parent);
  }
  if (current != NULL) CFRelease(current);
  // ancestors owns hit while the child search runs, without transferring AX references to the UI thread.
  if (!matched_window || ancestors.count == 0) return @[];
  NSMutableArray *queue = [NSMutableArray arrayWithObject:@{ @"element": ancestors.firstObject, @"depth": @0, @"path": @[] }];
  NSArray *best = @[];
  NSUInteger visited = 0, best_depth = 0;
  CGFloat best_area = CGFLOAT_MAX;
  // Reserve two bounded attribute calls for the final window-position/size check.
  CFTimeInterval child_deadline = deadline - 0.1;
  while (queue.count != 0 && visited++ < 512 && atomic_load(generation) == expected && CACurrentMediaTime() < child_deadline) {
    NSDictionary *entry = [[queue.lastObject retain] autorelease];
    [queue removeLastObject];
    AXUIElementRef element = (AXUIElementRef)entry[@"element"];
    NSUInteger depth = [entry[@"depth"] unsignedIntegerValue];
    NSRect rect = selector_frame(element, offset, child_deadline, generation, expected);
    NSArray *branch = entry[@"path"];
    if (!NSIsEmptyRect(rect)) {
      if (!NSPointInRect(point, rect)) continue;
      branch = [branch arrayByAddingObject:[NSValue valueWithRect:rect]];
      CGFloat area = NSWidth(rect)*NSHeight(rect);
      if (depth > best_depth || (depth == best_depth && area < best_area)) {
        best = branch; best_depth = depth; best_area = area;
      }
    }
    if (depth >= 64) continue;
    // Pages keep large web documents bounded even when the provider exposes thousands of children.
    CFStringRef attribute = CFSTR("AXVisibleChildren");
    for (CFIndex start = 0; start < 512 && queue.count + visited < 512; start += 32) {
      if (!selector_time_left(element, child_deadline, generation, expected)) break;
      CFArrayRef children = NULL;
      AXError error = AXUIElementCopyAttributeValues(element, attribute, start, 32, &children);
      if (error == kAXErrorAttributeUnsupported && start == 0) {
        attribute = kAXChildrenAttribute;
        if (selector_time_left(element, child_deadline, generation, expected))
          error = AXUIElementCopyAttributeValues(element, attribute, start, 32, &children);
      }
      if (error != kAXErrorSuccess || children == NULL) { if (children != NULL) CFRelease(children); break; }
      CFIndex count = CFArrayGetCount(children);
      for (CFIndex i = count; i > 0 && queue.count + visited < 512; i--) {
        CFTypeRef child = CFArrayGetValueAtIndex(children, i-1);
        if (CFGetTypeID(child) == AXUIElementGetTypeID() && !CFEqual(child, element))
          [queue addObject:@{ @"element": (id)child, @"depth": @(depth+1), @"path": branch }];
      }
      CFRelease(children);
      if (count < 32) break;
    }
  }
  // The provider can move while refinement is running. Revalidate the enclosing AX window before publishing any child geometry.
  NSRect final_frame = selector_frame((AXUIElementRef)ancestors.lastObject, offset, deadline, generation, expected);
  if (!NSEqualRects(final_frame, [frames.lastObject rectValue])) return @[];
  NSMutableArray *all = [NSMutableArray arrayWithArray:[[best reverseObjectEnumerator] allObjects]];
  [all addObjectsFromArray:frames];
  return selector_bounded_path(all, visible, point);
}

// Main-thread state holds rectangle snapshots only. One serial worker coalesces queries and never blocks closing.
@implementation WoxScreenshotObjectSelector {
  NSArray *_candidates;
  NSArray *_path;
  NSMutableDictionary *_activations;
  NSInteger _index;
  BOOL _explicit;
  CGFloat _offset;
  CFTimeInterval _target_changed;
  NSPoint _point;
  NSRect _bounds;
  BOOL _has_point, _running, _stopped;
  atomic_uint _generation;
  NSTimer *_dwell;
  dispatch_queue_t _worker;
  void (^_update)(NSRect);
}

- (instancetype)initWithCandidates:(NSArray *)candidates primaryOffset:(CGFloat)offset update:(void (^)(NSRect))update {
  self = [super init];
  if (self == nil) return nil;
  _candidates = [candidates copy];
  _activations = [[NSMutableDictionary alloc] init];
  _offset = offset;
  _update = [update copy];
  atomic_init(&_generation, 0);
  _worker = selector_worker();
  return self;
}

- (NSRect)target {
  return _index >= 0 && _index < (NSInteger)_path.count ? [_path[(NSUInteger)_index] rectValue] : NSZeroRect;
}

// hoverAt keeps a valid logical crop while foreground hits preserve the preceding visual preview within the same window.
- (void)hoverAt:(NSPoint)point displayBounds:(NSRect)bounds {
  if (_stopped || (_has_point && NSEqualPoints(point, _point) && NSEqualRects(bounds, _bounds))) return;
  _has_point = YES;
  _target_changed = CACurrentMediaTime();
  _point = point; _bounds = bounds; _explicit = NO;
  atomic_fetch_add(&_generation, 1);
  [_dwell invalidate]; [_dwell release]; _dwell = nil;
  NSRect frame = NSZeroRect;
  for (NSDictionary *entry in _candidates) {
    CGRect rect;
    if (!CGRectMakeWithDictionaryRepresentation((CFDictionaryRef)entry[(id)kCGWindowBounds], &rect)) continue;
    if (NSPointInRect(point, NSRectFromCGRect(rect))) { frame = NSIntersectionRect(NSRectFromCGRect(rect), bounds); break; }
  }
  // Desktop gaps still select the pointer's display, never the union of differently scaled displays.
  if (!NSPointInRect(point, bounds)) frame = NSZeroRect;
  else if (NSIsEmptyRect(frame)) frame = bounds;
  BOOL same_window = _path.count > 0 && NSEqualRects([_path.lastObject rectValue], frame);
  if (!same_window || !NSPointInRect(point, [_path.firstObject rectValue])) {
    [_path release];
    _path = NSIsEmptyRect(frame) ? nil : [@[[NSValue valueWithRect:frame]] retain];
    _index = 0;
  }
  // A foreground hit normally takes only a few milliseconds. Keep the previous visual target during that lookup,
  // rather than expanding to the full window and shrinking again whenever the pointer crosses a control boundary.
  if (_update != nil && (!same_window || _path.count > 1)) _update(self.target);
  [self queryWithBudget:0.168];
}

// Pointer dwell gates only background refinement; foreground hits start immediately and coalesce while the worker is busy.
- (void)scheduleRefinement {
  [_dwell invalidate]; [_dwell release]; _dwell = nil;
  if (_stopped || _running || NSIsEmptyRect(self.target)) return;
  CFTimeInterval remaining = MAX(0, 0.08-(CACurrentMediaTime()-_target_changed));
  _dwell = [[NSTimer scheduledTimerWithTimeInterval:remaining repeats:NO block:^(NSTimer *timer) {
    (void)timer;
    [self queryWithBudget:1.5];
  }] retain];
}

// queryWithBudget only transfers value rectangles back to AppKit; generation also cancels bounded native traversal.
- (void)queryWithBudget:(CFTimeInterval)budget {
  if (_stopped || _running || !NSPointInRect(_point, _bounds)) return;
  unsigned generation = atomic_load(&_generation);
  NSPoint point = _point;
  NSRect visible = _bounds;
  NSDictionary *candidate = nil;
  NSRect frame = NSZeroRect;
  for (NSDictionary *entry in _candidates) {
    CGRect rect;
    if (!CGRectMakeWithDictionaryRepresentation((CFDictionaryRef)entry[(id)kCGWindowBounds], &rect)) continue;
    if (NSPointInRect(point, NSRectFromCGRect(rect))) { candidate = entry; frame = NSRectFromCGRect(rect); break; }
  }
  if (candidate == nil) return;
  pid_t pid = [candidate[(id)kCGWindowOwnerPID] intValue];
  // Our own AX hit would resolve the selector panel covering the frozen window. Keep its exact window fallback without self IPC.
  if (pid == getpid()) return;
  _running = YES;
  dispatch_async(_worker, ^{
    @autoreleasepool {
      NSArray *path = [selector_hit_path(pid, frame, NSIntersectionRect(frame, visible), point, self->_offset, budget, &self->_generation, generation, self->_activations) copy];
      dispatch_async(dispatch_get_main_queue(), ^{
        self->_running = NO;
        if (!self->_stopped && atomic_load(&self->_generation) == generation) {
          NSRect previous = self.target;
          NSInteger index = self->_explicit ? [path indexOfObject:[NSValue valueWithRect:previous]] : 0;
          BOOL compatible = YES;
          if (budget > 1) {
            NSUInteger matched = 0;
            for (NSValue *rect in path) if (matched < self->_path.count && [rect isEqual:self->_path[matched]]) matched++;
            compatible = matched == self->_path.count && path.count > self->_path.count;
          }
          if (path.count > 0 && compatible && (!self->_explicit || index != NSNotFound)) {
            // Moving within the same path must preserve the ancestor chosen with the wheel.
            if (![path isEqualToArray:self->_path]) {
              [self->_path release]; self->_path = [path copy]; self->_index = index;
            }
            if (self->_update != nil) self->_update(self.target);
          }
          if (budget < 1) {
            if (path.count == 0 && self->_update != nil) self->_update(self.target);
            [self scheduleRefinement];
          }
        } else if (!self->_stopped) {
          [self queryWithBudget:0.168];
        }
      });
      [path release];
    }
  });
}

- (void)step:(NSInteger)direction {
  if (_stopped || _path.count == 0) return;
  _index = MIN(MAX(0, _index+direction), (NSInteger)_path.count-1);
  _explicit = YES;
  if (_update != nil) _update(self.target);
}

// Stop callbacks and invalidate work before the session's assign callback target can disappear.
- (void)stop {
  if (_stopped) return;
  _stopped = YES;
  atomic_fetch_add(&_generation, 1);
  [_dwell invalidate]; [_dwell release]; _dwell = nil;
  [_update release]; _update = nil;
  dispatch_async(_worker, ^{
    @autoreleasepool { selector_restore(self->_activations); }
  });
}

- (void)dealloc {
  [_dwell invalidate]; [_dwell release];
  [_update release];
  [_path release]; [_candidates release]; [_activations release];
  [super dealloc];
}
@end

// Verify nested geometry, duplicate containers, clipping, and interruption without starting an AX provider or a desktop selector.
int32_t wox_darwin_test_screenshot_object_selection(void) {
  @autoreleasepool {
    NSRect window = NSMakeRect(-200, 40, 400, 300);
    NSRect leaf = NSMakeRect(-50, 70, 40, 30);
    NSArray *path = selector_bounded_path(@[[NSValue valueWithRect:leaf], [NSValue valueWithRect:leaf],
        [NSValue valueWithRect:NSMakeRect(-100, 50, 200, 100)], [NSValue valueWithRect:window]], window, NSMakePoint(-30, 80));
    if (path.count != 3 || !NSEqualRects([path.firstObject rectValue], leaf)) return 1;
    NSRect clipped = NSMakeRect(0, 40, 200, 300);
    path = selector_bounded_path(@[[NSValue valueWithRect:window]], clipped, NSMakePoint(20, 80));
    if (path.count != 1 || !NSEqualRects([path.firstObject rectValue], clipped)) return 2;
    CALayer *layer = [CALayer layer];
    [CATransaction begin]; [CATransaction setDisableActions:YES];
    wox_screenshot_set_selection_layer_frame(layer, NSRectToCGRect(leaf), NO);
    wox_screenshot_set_selection_layer_frame(layer, NSRectToCGRect(window), YES);
    if (fabs([layer animationForKey:@"bounds"].duration-0.101) > 0.00001 || !CGRectEqualToRect(layer.frame, NSRectToCGRect(window))) return 3;
    wox_screenshot_set_selection_layer_frame(layer, NSRectToCGRect(leaf), NO);
    if ([layer animationForKey:@"bounds"] != nil) return 4;
    [CATransaction commit];
    return 0;
  }
}
