#ifndef WOX_SCREENSHOT_OBJECT_SELECTION_DARWIN_H
#define WOX_SCREENSHOT_OBJECT_SELECTION_DARWIN_H

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>

// Geometry uses Wox's global top-left logical points; AX coordinates use the primary display's top-left origin.
// Candidates are a filtered front-to-back snapshot, taken before the capture overlays appear.
@interface WoxScreenshotObjectSelector : NSObject
- (instancetype)initWithCandidates:(NSArray *)candidates primaryOffset:(CGFloat)offset update:(void (^)(NSRect))update;
- (void)hoverAt:(NSPoint)point displayBounds:(NSRect)bounds;
- (NSRect)target;
- (void)step:(NSInteger)direction;
- (void)stop;
@end

void wox_screenshot_set_selection_layer_frame(CALayer *layer, CGRect frame, BOOL animated);
NSArray *wox_screenshot_selectable_windows(NSArray *windows, NSSet *excluded);
int32_t wox_darwin_test_screenshot_object_selection(void);

#endif
