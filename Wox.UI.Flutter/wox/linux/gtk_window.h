#ifndef GTK_WINDOW_H_
#define GTK_WINDOW_H_

#include <gtk/gtk.h>

bool is_gtk_available();
void resize_gtk_window(GtkWindow* window, int width, int height);

#endif  // GTK_WINDOW_H_ 