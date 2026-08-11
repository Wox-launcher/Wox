//go:build linux

#include <X11/Xlib.h>
#include <stdint.h>

// Capture the X11 pointer in the same root coordinate space as the desktop image.
int32_t wox_screenshot_cursor_position(float *x, float *y) {
  if (x == NULL || y == NULL) {
    return -1;
  }
  Display *display = XOpenDisplay(NULL);
  if (display == NULL) {
    return -1;
  }
  Window root = DefaultRootWindow(display);
  Window root_return;
  Window child_return;
  int root_x;
  int root_y;
  int window_x;
  int window_y;
  unsigned int mask;
  if (!XQueryPointer(display, root, &root_return, &child_return, &root_x, &root_y, &window_x, &window_y, &mask)) {
    XCloseDisplay(display);
    return -1;
  }
  XCloseDisplay(display);
  *x = (float)root_x;
  *y = (float)root_y;
  return 0;
}

// Move the X11 pointer in root coordinates so keyboard nudging stays aligned with the visible cursor.
int32_t wox_screenshot_set_cursor_position(int32_t x, int32_t y) {
  Display *display = XOpenDisplay(NULL);
  if (display == NULL) {
    return -1;
  }
  XWarpPointer(display, None, DefaultRootWindow(display), 0, 0, 0, 0, x, y);
  XFlush(display);
  XCloseDisplay(display);
  return 0;
}
