//go:build linux && cgo

#include "browser_external_linux_gtk.h"
#include <gtk/gtk.h>

// Preserve GTK's owner-aware URI dispatch for desktop portals and activation context.
int32_t wox_browser_open_external_url(const char *url, uintptr_t owner) {
  if (url == NULL) {
    return -1;
  }
  GError *error = NULL;
  gboolean opened = gtk_show_uri_on_window((GtkWindow *)owner, url, GDK_CURRENT_TIME, &error);
  if (error != NULL) {
    g_error_free(error);
  }
  return opened ? 0 : -1;
}
