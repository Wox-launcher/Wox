#ifndef WOX_UI_NATIVE_LINUX_CLIPBOARD_H
#define WOX_UI_NATIVE_LINUX_CLIPBOARD_H

#include <gtk/gtk.h>

gboolean wox_linux_clipboard_set_png(GtkClipboard *clipboard, const guint8 *png, gsize length);
void wox_linux_clipboard_flush_png(void);

#endif
