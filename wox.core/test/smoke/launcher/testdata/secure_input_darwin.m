#import <Carbon/Carbon.h>
#import <Cocoa/Cocoa.h>

int main(void) {
  @autoreleasepool {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    [NSApp finishLaunching];

    NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 340, 130)
                                                  styleMask:NSWindowStyleMaskTitled
                                                    backing:NSBackingStoreBuffered
                                                      defer:NO];
    window.title = @"Wox Secure Input test";
    NSSecureTextField *field = [[NSSecureTextField alloc] initWithFrame:NSMakeRect(20, 48, 300, 25)];
    [window.contentView addSubview:field];
    [window center];
    [NSApp activateIgnoringOtherApps:YES];
    [window makeKeyAndOrderFront:nil];
    [window makeFirstResponder:field];

    // Wait for foreground activation to settle before exercising an already focused password field.
    __block NSUInteger focused_samples = 0;
    [NSTimer scheduledTimerWithTimeInterval:0.1 repeats:YES block:^(NSTimer *timer) {
      if (NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier != getpid() ||
          !NSApp.isActive || !window.isKeyWindow || field.currentEditor == nil || !IsSecureEventInputEnabled()) {
        focused_samples = 0;
        return;
      }
      if (++focused_samples < 3) return;
      puts("secure input ready");
      fflush(stdout);
      [timer invalidate];
    }];
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 3 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{
      if (focused_samples < 3) exit(1);
    });
    [NSApp run];
  }
  return 0;
}
