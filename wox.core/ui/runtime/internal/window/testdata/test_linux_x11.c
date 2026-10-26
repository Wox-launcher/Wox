#include "native_linux_desktop.h"
#include <gdk/gdkx.h>
#include <X11/Xatom.h>
#include <assert.h>
#include <stdio.h>

void wox_linux_note_stage(const char *stage) {
  (void)stage;
}

// Simulate the WM's active-window property without running a desktop session.
static void set_active(Display *display, Window window) {
  Atom active = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
  XChangeProperty(display, DefaultRootWindow(display), active, XA_WINDOW, 32,
                  PropModeReplace, (unsigned char *)&window, 1);
  XSync(display, False);
}

// Read the actual EWMH message, rather than inspecting adapter implementation state.
static XClientMessageEvent expect_message(Display *display, const char *name, Window target) {
  XSync(display, False);
  XEvent event;
  assert(XCheckTypedEvent(display, ClientMessage, &event));
  assert(event.xclient.message_type == XInternAtom(display, name, False));
  assert(event.xclient.window == target);
  assert(event.xclient.format == 32);
  return event.xclient;
}

static void expect_no_message(Display *display) {
  XSync(display, False);
  XEvent event;
  assert(!XCheckTypedEvent(display, ClientMessage, &event));
}

// GTK sends its own mapping messages; isolate those from the adapter operation.
static void discard_mapping_messages(Display *display) {
  XSync(display, False);
  XEvent event;
  while (XCheckTypedEvent(display, ClientMessage, &event)) {}
}

// Exercise native focus lifetime, mapping hints, activation and resize on Xvfb.
int main(int argc, char **argv) {
  assert(wox_linux_desktop_init_threads());
  assert(gtk_init_check(&argc, &argv));
  Display *display = GDK_DISPLAY_XDISPLAY(gdk_display_get_default());
  Window root = DefaultRootWindow(display);
  XSelectInput(display, root, SubstructureNotifyMask);

  GtkWidget *first = gtk_window_new(GTK_WINDOW_TOPLEVEL);
  GtkWidget *second = gtk_window_new(GTK_WINDOW_TOPLEVEL);
  g_object_ref_sink(first);
  g_object_ref_sink(second);
  // Unrealized widgets and non-X11 displays have no usable native window.
  assert(wox_linux_desktop_window_id(first) == None);
  assert(wox_linux_desktop_compositor_present(first) == -1);
  wox_linux_desktop_activate(first);
  wox_linux_desktop_skip_taskbar(first);
  wox_linux_desktop_resize(first, 200, 100);
  wox_linux_desktop_remember_focus(first);
  assert(!wox_linux_desktop_can_restore_focus(first));

  gtk_widget_show(first);
  gtk_widget_show(second);
  discard_mapping_messages(display);
  Window first_id = wox_linux_desktop_window_id(first);
  Window second_id = wox_linux_desktop_window_id(second);
  assert(first_id != None && second_id != None && first_id != second_id);
  Window foreign = XCreateSimpleWindow(display, root, 10, 10, 300, 200, 0, 0, 0);
  XMapWindow(display, foreign);
  set_active(display, foreign);
  wox_linux_desktop_remember_focus(first);
  assert(wox_linux_desktop_can_restore_focus(first));
  assert(!wox_linux_desktop_can_restore_focus(second));
  set_active(display, first_id);
  wox_linux_desktop_remember_focus(first);
  wox_linux_desktop_remember_focus(second);
  wox_linux_desktop_restore_focus(first);
  XClientMessageEvent activation = expect_message(display, "_NET_ACTIVE_WINDOW", foreign);
  assert(activation.data.l[0] == 2 && activation.data.l[1] == CurrentTime);
  assert(!wox_linux_desktop_can_restore_focus(first));
  assert(wox_linux_desktop_can_restore_focus(second));
  wox_linux_desktop_restore_focus(first);
  expect_no_message(display);
  wox_linux_desktop_restore_focus(second);
  expect_message(display, "_NET_ACTIVE_WINDOW", first_id);

  set_active(display, foreign);
  wox_linux_desktop_remember_focus(first);
  wox_linux_desktop_clear_focus(first);
  wox_linux_desktop_restore_focus(first);
  expect_no_message(display);
  // A closed or unmapped foreign target must never abort GTK or receive activation.
  wox_linux_desktop_remember_focus(first);
  XUnmapWindow(display, foreign);
  wox_linux_desktop_restore_focus(first);
  expect_no_message(display);
  XMapWindow(display, foreign);
  wox_linux_desktop_remember_focus(first);
  XDestroyWindow(display, foreign);
  wox_linux_desktop_restore_focus(first);
  expect_no_message(display);
  wox_linux_desktop_remember_focus(first);
  assert(!wox_linux_desktop_can_restore_focus(first));

  wox_linux_desktop_activate(first);
  activation = expect_message(display, "_NET_ACTIVE_WINDOW", first_id);
  assert(activation.data.l[0] == 2);
  for (int remap = 0; remap < 2; remap++) {
    gtk_widget_hide(first);
    gtk_widget_show(first);
    discard_mapping_messages(display);
    wox_linux_desktop_skip_taskbar(first);
    XClientMessageEvent state = expect_message(display, "_NET_WM_STATE", first_id);
    assert(state.data.l[0] == 1);
    assert(state.data.l[1] == (long)XInternAtom(display, "_NET_WM_STATE_SKIP_TASKBAR", False));
    assert(state.data.l[2] == (long)XInternAtom(display, "_NET_WM_STATE_SKIP_PAGER", False));
  }
  wox_linux_desktop_resize(first, 241, 137);
  XWindowAttributes attributes;
  assert(XGetWindowAttributes(display, first_id, &attributes));
  assert(attributes.width == 241 && attributes.height == 137);
  assert(wox_linux_desktop_compositor_present(first) == 0);
  char selection[32];
  snprintf(selection, sizeof(selection), "_NET_WM_CM_S%d", DefaultScreen(display));
  XSetSelectionOwner(display, XInternAtom(display, selection, False), second_id, CurrentTime);
  assert(wox_linux_desktop_compositor_present(first) == 1);

  // Adapter state is attached to the GTK object and must not retain it after close.
  set_active(display, second_id);
  wox_linux_desktop_remember_focus(first);
  gpointer weak_first = first;
  g_object_add_weak_pointer(G_OBJECT(first), &weak_first);
  gtk_widget_destroy(first);
  g_object_unref(first);
  assert(weak_first == NULL);
  gtk_widget_destroy(second);
  g_object_unref(second);
  puts("Linux X11 adapter: passed");
  return 0;
}
