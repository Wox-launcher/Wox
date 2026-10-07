#ifndef WOX_SCREENSHOT_OBJECT_SELECTION_X11_H
#define WOX_SCREENSHOT_OBJECT_SELECTION_X11_H
#include <stdint.h>
typedef struct {
  int32_t pid;
  int32_t x, y, width, height;
  int32_t client_x, client_y, client_width, client_height;
} WoxScreenshotX11Window;
int32_t wox_screenshot_x11_windows(WoxScreenshotX11Window *windows, int32_t capacity);
#endif
