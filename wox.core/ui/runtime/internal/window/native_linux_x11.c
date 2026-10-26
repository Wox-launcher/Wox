//go:build linux

#include "native_linux_desktop.h"
#include "native_linux.h"

#include <stdio.h>
#include <string.h>

#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#endif

// GTK may call Xlib during initialization, so thread support must be enabled first.
bool wox_linux_desktop_init_threads(void) {
#ifdef GDK_WINDOWING_X11
  return XInitThreads() != 0;
#else
  return true;
#endif
}

// The adapter owns focus history through the GTK object's lifetime, not renderer state.
bool wox_linux_desktop_can_restore_focus(GtkWidget *widget) {
  return g_object_get_data(G_OBJECT(widget), "wox-linux-x11-focus") != NULL;
}

void wox_linux_desktop_clear_focus(GtkWidget *widget) {
  g_object_set_data(G_OBJECT(widget), "wox-linux-x11-focus", NULL);
}

#ifdef GDK_WINDOWING_X11
// Check the widget's actual backend, including XWayland inside a Wayland session.
static GdkDisplay *x11_gdk_display(GtkWidget *widget) {
  GdkWindow *gdk_window = gtk_widget_get_window(widget);
  if (gdk_window == NULL || !GDK_IS_X11_WINDOW(gdk_window)) {
    return NULL;
  }
  return gdk_window_get_display(gdk_window);
}

static Display *x11_display(GtkWidget *widget) {
  GdkDisplay *gdk_display = x11_gdk_display(widget);
  return gdk_display != NULL ? GDK_DISPLAY_XDISPLAY(gdk_display) : NULL;
}

unsigned long wox_linux_desktop_window_id(GtkWidget *widget) {
  GdkWindow *gdk_window = gtk_widget_get_window(widget);
  return gdk_window != NULL && GDK_IS_X11_WINDOW(gdk_window) ? GDK_WINDOW_XID(gdk_window) : None;
}

// Foreign XIDs can disappear before we query them. GDK's default X handler
// treats BadWindow as fatal, so this check must run inside an error trap.
static bool x11_window_is_viewable(GdkDisplay *gdk_display, Display *display, Window xid) {
  if (gdk_display == NULL || display == NULL || xid == None) {
    return false;
  }
  XWindowAttributes attributes;
  gdk_x11_display_error_trap_push(gdk_display);
  int ok = XGetWindowAttributes(display, xid, &attributes);
  if (gdk_x11_display_error_trap_pop(gdk_display) != 0 || ok == 0) {
    return false;
  }
  return attributes.map_state == IsViewable;
}

// Read the window manager's active window without taking ownership of the foreign XID.
static Window active_x11_window(GtkWidget *widget) {
  Display *display = x11_display(widget);
  if (display == NULL) {
    return None;
  }
  Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", True);
  if (property == None) {
    return None;
  }
  Atom actual_type = None;
  int actual_format = 0;
  unsigned long count = 0;
  unsigned long remaining = 0;
  unsigned char *data = NULL;
  Window active = None;
  if (XGetWindowProperty(display, DefaultRootWindow(display), property, 0, 1, False, XA_WINDOW, &actual_type, &actual_format, &count, &remaining, &data) == Success && actual_type == XA_WINDOW && actual_format == 32 && count == 1 && data != NULL) {
    active = *(Window *)data;
  }
  if (data != NULL) {
    XFree(data);
  }
  return active;
}

// Remember only a mapped foreign window; repeated shows must not replace it with ourselves.
void wox_linux_desktop_remember_focus(GtkWidget *widget) {
  GdkDisplay *gdk_display = x11_gdk_display(widget);
  Display *display = gdk_display != NULL ? GDK_DISPLAY_XDISPLAY(gdk_display) : NULL;
  Window current = wox_linux_desktop_window_id(widget);
  Window active = active_x11_window(widget);
  if (active != None && active != current && x11_window_is_viewable(gdk_display, display, active)) {
    Window *previous = g_new(Window, 1);
    *previous = active;
    g_object_set_data_full(G_OBJECT(widget), "wox-linux-x11-focus", previous, g_free);
  }
}

// Reapply utility-window hints after mapping, when the window manager can consume them.
void wox_linux_desktop_skip_taskbar(GtkWidget *widget) {
  GdkDisplay *gdk_display = x11_gdk_display(widget);
  Display *display = gdk_display != NULL ? GDK_DISPLAY_XDISPLAY(gdk_display) : NULL;
  Window xid = wox_linux_desktop_window_id(widget);
  if (gdk_display == NULL || display == NULL || xid == None) {
    return;
  }
  Atom state = XInternAtom(display, "_NET_WM_STATE", False);
  Atom skip_taskbar = XInternAtom(display, "_NET_WM_STATE_SKIP_TASKBAR", False);
  Atom skip_pager = XInternAtom(display, "_NET_WM_STATE_SKIP_PAGER", False);
  if (state == None || skip_taskbar == None || skip_pager == None) {
    return;
  }
  // KWin can drop SKIP_TASKBAR on remap/activate; re-add it after every show.
  XEvent event;
  memset(&event, 0, sizeof(event));
  event.xclient.type = ClientMessage;
  event.xclient.window = xid;
  event.xclient.message_type = state;
  event.xclient.format = 32;
  event.xclient.data.l[0] = 1;
  event.xclient.data.l[1] = (long)skip_taskbar;
  event.xclient.data.l[2] = (long)skip_pager;
  event.xclient.data.l[3] = 1;
  gdk_x11_display_error_trap_push(gdk_display);
  XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
  XFlush(display);
  gdk_x11_display_error_trap_pop_ignored(gdk_display);
}

// Ask the window manager to activate the mapped Wox window.
void wox_linux_desktop_activate(GtkWidget *widget) {
  Display *display = x11_display(widget);
  Window xid = wox_linux_desktop_window_id(widget);
  if (display == NULL || xid == None) {
    return;
  }
  XRaiseWindow(display, xid);
  Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
  if (property != None) {
    XEvent event;
    memset(&event, 0, sizeof(event));
    event.xclient.type = ClientMessage;
    event.xclient.window = xid;
    event.xclient.message_type = property;
    event.xclient.format = 32;
    event.xclient.data.l[0] = 2;
    event.xclient.data.l[1] = CurrentTime;
    XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
  }
  XFlush(display);
}

// Consume focus history once, tolerating foreign windows destroyed while Wox was visible.
void wox_linux_desktop_restore_focus(GtkWidget *widget) {
  GdkDisplay *gdk_display = x11_gdk_display(widget);
  Display *display = gdk_display != NULL ? GDK_DISPLAY_XDISPLAY(gdk_display) : NULL;
  Window *saved = g_object_get_data(G_OBJECT(widget), "wox-linux-x11-focus");
  Window previous = saved != NULL ? *saved : None;
  wox_linux_desktop_clear_focus(widget);
  if (display == NULL || !x11_window_is_viewable(gdk_display, display, previous)) {
    return;
  }
  Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
  if (property == None) {
    return;
  }
  XEvent event;
  memset(&event, 0, sizeof(event));
  event.xclient.type = ClientMessage;
  event.xclient.window = previous;
  event.xclient.message_type = property;
  event.xclient.format = 32;
  event.xclient.data.l[0] = 2;
  event.xclient.data.l[1] = CurrentTime;
  // The ClientMessage still names a foreign XID that may vanish after the
  // viewable check; keep the send inside a trap so GDK cannot abort on it.
  gdk_x11_display_error_trap_push(gdk_display);
  XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
  XFlush(display);
  gdk_x11_display_error_trap_pop_ignored(gdk_display);
}
#else
unsigned long wox_linux_desktop_window_id(GtkWidget *widget) {
  (void)widget;
  return 0;
}
void wox_linux_desktop_remember_focus(GtkWidget *widget) {
  (void)widget;
}
void wox_linux_desktop_skip_taskbar(GtkWidget *widget) {
  (void)widget;
}
void wox_linux_desktop_activate(GtkWidget *widget) {
  (void)widget;
}
void wox_linux_desktop_restore_focus(GtkWidget *widget) {
  (void)widget;
}
#endif

// wox_linux_desktop_compositor_present reports whether the current X screen owns the standard compositor selection.
int wox_linux_desktop_compositor_present(GtkWidget *widget) {
#ifdef GDK_WINDOWING_X11
  Display *display = x11_display(widget);
  if (display != NULL) {
    char selection_name[32];
    snprintf(selection_name, sizeof(selection_name), "_NET_WM_CM_S%d", DefaultScreen(display));
    Atom selection = XInternAtom(display, selection_name, True);
    return selection != None && XGetSelectionOwner(display, selection) != None ? 1 : 0;
  }
#else
  (void)widget;
#endif
  return -1;
}

// Openbox can keep the previous taller X window after GTK resize requests, leaving
// GtkGLArea with a black framebuffer. Synchronize the X window before drawing.
// Dimensions are forwarded unchanged to XResizeWindow, which consumes physical pixels.
void wox_linux_desktop_resize(GtkWidget *widget, int width, int height) {
#ifdef GDK_WINDOWING_X11
  GdkDisplay *gdk_display = x11_gdk_display(widget);
  Display *display = x11_display(widget);
  Window xid = wox_linux_desktop_window_id(widget);
  if (gdk_display != NULL && display != NULL && xid != None) {
    gdk_x11_display_error_trap_push(gdk_display);
    wox_linux_note_stage("x11-resize-flush");
    XResizeWindow(display, xid, (unsigned int)width, (unsigned int)height);
    XFlush(display);
    wox_linux_note_stage("x11-resize-flush-done");
    gdk_x11_display_error_trap_pop_ignored(gdk_display);
  }
#else
  (void)widget;
  (void)width;
  (void)height;
#endif
}
