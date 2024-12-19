#import <Cocoa/Cocoa.h>

typedef struct {
    int width;
    int height;
    int x;
    int y;
} ScreenInfo;

ScreenInfo getMouseScreenSize() {
    NSPoint mouseLoc = [NSEvent mouseLocation];
    NSArray *screens = [NSScreen screens];

    for (NSScreen *screen in screens) {
        NSRect frame = [screen frame];
        if (NSMouseInRect(mouseLoc, frame, NO)) {
            return (ScreenInfo){.width = frame.size.width, .height = frame.size.height, .x = frame.origin.x, .y = frame.origin.y};
        }
    }
    return (ScreenInfo){.width = 0, .height = 0, .x = 0, .y = 0};
}
