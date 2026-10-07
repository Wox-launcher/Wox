#ifndef WOX_CLIPBOARD_IMAGE_LINUX_GTK_H
#define WOX_CLIPBOARD_IMAGE_LINUX_GTK_H

#include <gtk/gtk.h>
#include <stdint.h>
#include <stddef.h>

int32_t wox_clipboard_gtk_write_text(uintptr_t display, const char *text);
int32_t wox_clipboard_gtk_write_pixels(uintptr_t display, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride);
int32_t wox_clipboard_gtk_write_png(uintptr_t display, const uint8_t *png, size_t length);

gboolean wox_clipboard_gtk_set_png(GtkClipboard *clipboard, const guint8 *png, gsize length);
void wox_clipboard_gtk_flush(void);

#endif
