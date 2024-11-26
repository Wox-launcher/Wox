#include "gtk_window.h"

bool is_gtk_available() {
  // Check if we're running under GTK
  const char* xdg_session_type = g_getenv("XDG_SESSION_TYPE");
  const char* desktop_session = g_getenv("DESKTOP_SESSION");
  const char* current_desktop = g_getenv("XDG_CURRENT_DESKTOP");
  
  if (xdg_session_type != nullptr && 
      (g_strcmp0(xdg_session_type, "wayland") == 0 || g_strcmp0(xdg_session_type, "x11") == 0)) {
    if (current_desktop != nullptr && 
        (g_strstr_len(current_desktop, -1, "GNOME") != nullptr || 
         g_strstr_len(current_desktop, -1, "Unity") != nullptr ||
         g_strstr_len(current_desktop, -1, "XFCE") != nullptr ||
         g_strstr_len(current_desktop, -1, "Pantheon") != nullptr ||
         g_strstr_len(current_desktop, -1, "MATE") != nullptr ||
         g_strstr_len(current_desktop, -1, "Cinnamon") != nullptr)) {
      return true;
    }
    if (desktop_session != nullptr &&
        (g_strstr_len(desktop_session, -1, "gnome") != nullptr ||
         g_strstr_len(desktop_session, -1, "unity") != nullptr ||
         g_strstr_len(desktop_session, -1, "xfce") != nullptr ||
         g_strstr_len(desktop_session, -1, "mate") != nullptr ||
         g_strstr_len(desktop_session, -1, "cinnamon") != nullptr)) {
      return true;
    }
  }
  return false;
}

void resize_gtk_window(GtkWindow* window, int width, int height) {
  if (window == nullptr) {
    return;
  }
  
  // Set minimum size to prevent window from becoming too small
  gtk_window_set_resizable(window, TRUE);
  gtk_window_set_default_size(window, width, height);
  gtk_window_resize(window, width, height);
  
  // Force the window to process the resize
  gtk_widget_queue_resize(GTK_WIDGET(window));
} 