//go:build linux

#include "object_selection_linux_x11.h"
#include <gtk/gtk.h>
#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#include <X11/Xatom.h>
#include <unistd.h>

// Read bounded EWMH properties under GDK's error trap; closing a client during enumeration must not terminate Wox.
static unsigned long *selection_x11_property(Display *display, Window window, const char *name, Atom type, unsigned long *count) {
  Atom actual;
  int format;
  unsigned long remaining;
  unsigned char *data = NULL;
  *count = 0;
  if (XGetWindowProperty(display, window, XInternAtom(display, name, True), 0, 512, False, type,
                         &actual, &format, count, &remaining, &data) != Success || actual != type || format != 32) {
    if (data != NULL) XFree(data);
    *count = 0;
    return NULL;
  }
  return (unsigned long *)data;
}
#endif

// Snapshot normal visible clients in front-to-back order before the screenshot overlay exists. Output is in physical X11 root pixels.
int32_t wox_screenshot_x11_windows(WoxScreenshotX11Window *windows, int32_t capacity) {
#ifdef GDK_WINDOWING_X11
  GdkDisplay *display = gdk_display_get_default();
  if (windows == NULL || capacity <= 0 || display == NULL || !GDK_IS_X11_DISPLAY(display)) return 0;
  Display *xdisplay = GDK_DISPLAY_XDISPLAY(display);
  Window root = DefaultRootWindow(xdisplay);
  gdk_x11_display_error_trap_push(display);
  unsigned long length = 0;
  unsigned long *clients = selection_x11_property(xdisplay, root, "_NET_CLIENT_LIST_STACKING", XA_WINDOW, &length);
  int32_t count = 0;
  Atom desktop = XInternAtom(xdisplay, "_NET_WM_WINDOW_TYPE_DESKTOP", True);
  Atom dock = XInternAtom(xdisplay, "_NET_WM_WINDOW_TYPE_DOCK", True);
  for (unsigned long index = length; index > 0 && count < capacity; --index) {
    Window client = (Window)clients[index-1];
    XWindowAttributes attributes;
    if (!XGetWindowAttributes(xdisplay, client, &attributes) || attributes.map_state != IsViewable || attributes.override_redirect) continue;
    unsigned long size = 0;
    unsigned long *types = selection_x11_property(xdisplay, client, "_NET_WM_WINDOW_TYPE", XA_ATOM, &size);
    gboolean excluded = FALSE;
    for (unsigned long i = 0; i < size; i++) if (types[i] == desktop || types[i] == dock) excluded = TRUE;
    if (types != NULL) XFree(types);
    if (excluded) continue;
    Window child;
    int x, y;
    if (!XTranslateCoordinates(xdisplay, client, root, 0, 0, &x, &y, &child)) continue;
    WoxScreenshotX11Window frame = {.x = x, .y = y, .width = attributes.width, .height = attributes.height,
      .client_x = x, .client_y = y, .client_width = attributes.width, .client_height = attributes.height};
    unsigned long *pid = selection_x11_property(xdisplay, client, "_NET_WM_PID", XA_CARDINAL, &size);
    if (size == 1) frame.pid = (int32_t)pid[0];
    if (pid != NULL) XFree(pid);
    unsigned long *extents = selection_x11_property(xdisplay, client, "_NET_FRAME_EXTENTS", XA_CARDINAL, &size);
    if (size == 4 && extents[0] < 1024 && extents[1] < 1024 && extents[2] < 1024 && extents[3] < 1024) {
      frame.x -= (int32_t)extents[0]; frame.y -= (int32_t)extents[2];
      frame.width += (int32_t)(extents[0]+extents[1]); frame.height += (int32_t)(extents[2]+extents[3]);
    }
    if (extents != NULL) XFree(extents);
    if (frame.width >= 2 && frame.height >= 2) windows[count++] = frame;
  }
  if (clients != NULL) XFree(clients);
  gdk_x11_display_error_trap_pop_ignored(display);
  return count;
#else
  (void)windows; (void)capacity;
  return 0;
#endif
}
