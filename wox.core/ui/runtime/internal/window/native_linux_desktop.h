#ifndef WOX_UI_NATIVE_LINUX_DESKTOP_H
#define WOX_UI_NATIVE_LINUX_DESKTOP_H

#include <gtk/gtk.h>
#include <stdbool.h>

// Desktop adapters borrow GTK widgets on the owning thread. Any retained focus
// state belongs to the adapter and is released with the widget, not WoxLinuxWindow.
bool wox_linux_desktop_init_threads(void);
void wox_linux_desktop_remember_focus(GtkWidget *widget);
bool wox_linux_desktop_can_restore_focus(GtkWidget *widget);
void wox_linux_desktop_restore_focus(GtkWidget *widget);
void wox_linux_desktop_clear_focus(GtkWidget *widget);
void wox_linux_desktop_skip_taskbar(GtkWidget *widget);
void wox_linux_desktop_activate(GtkWidget *widget);
void wox_linux_desktop_resize(GtkWidget *widget, int width, int height);
unsigned long wox_linux_desktop_window_id(GtkWidget *widget);
int wox_linux_desktop_compositor_present(GtkWidget *widget);

#endif
