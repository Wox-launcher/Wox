#ifndef WOX_NATIVE_DARWIN_PLATFORM_H
#define WOX_NATIVE_DARWIN_PLATFORM_H

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>

// Nonactivating fullscreen overlays share AppKit focus and cursor ownership with runtime-managed windows.
@interface WoxOverlayPanel : NSPanel {
  BOOL _owns_cursor_control;
}
@property(nonatomic, assign) BOOL woxNonactivating;
@end

// Call these AppKit primitives on the main thread, using dispatch_sync when entering from a worker.
void wox_darwin_dispatch_sync(dispatch_block_t block);
CGFloat wox_darwin_desktop_top(void);
// Returns a retained image; the feature owning the capture must release it.
CGImageRef wox_darwin_copy_display_image(CGDirectDisplayID display_id);
void wox_darwin_log_window_state(const char *stage, const void *owner, NSString *detail);

#endif
