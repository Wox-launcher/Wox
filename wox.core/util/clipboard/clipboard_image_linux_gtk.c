//go:build linux && cgo

#include "clipboard_image_linux_gtk.h"
#include <gio/gio.h>
#include <string.h>

// Display identity comes from the UI adapter; no window or runtime symbols enter this package.
static GtkClipboard *display_clipboard(uintptr_t display) {
  return display ? gtk_clipboard_get_default((GdkDisplay *)display) : NULL;
}

int32_t wox_clipboard_gtk_write_text(uintptr_t display, const char *text) {
  GtkClipboard *clipboard = display_clipboard(display);
  if (clipboard == NULL || text == NULL) {
    return -1;
  }
  gtk_clipboard_set_text(clipboard, text, -1);
  gtk_clipboard_store(clipboard);
  return 0;
}

static void free_pixbuf_pixels(guchar *pixels, gpointer data) {
  (void)data;
  g_free(pixels);
}

// GTK owns a native copy of fallback pixels after the Go call returns.
int32_t wox_clipboard_gtk_write_pixels(uintptr_t display, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride) {
  GtkClipboard *clipboard = display_clipboard(display);
  if (clipboard == NULL || pixels == NULL || width <= 0 || height <= 0 || row_stride < width * 4) {
    return -1;
  }
  size_t size = (size_t)row_stride * height;
  guchar *copy = g_malloc(size);
  if (copy == NULL) {
    return -1;
  }
  memcpy(copy, pixels, size);
  GdkPixbuf *pixbuf = gdk_pixbuf_new_from_data(copy, GDK_COLORSPACE_RGB, TRUE, 8, width, height, row_stride, free_pixbuf_pixels, NULL);
  if (pixbuf == NULL) {
    g_free(copy);
    return -1;
  }
  gtk_clipboard_set_image(clipboard, pixbuf);
  gtk_clipboard_store(clipboard);
  g_object_unref(pixbuf);
  return 0;
}

int32_t wox_clipboard_gtk_write_png(uintptr_t display, const uint8_t *png, size_t length) {
  return wox_clipboard_gtk_set_png(display_clipboard(display), png, length) ? 0 : -1;
}

// All access is on GTK's main thread. Retain compressed bytes only until clipboard ownership changes.
typedef struct {
  GtkClipboard *clipboard;
  GBytes *png;
} WoxLinuxClipboardPNG;

static WoxLinuxClipboardPNG *clipboard_png = NULL;

// Serve the canonical PNG directly; decode a temporary pixbuf only for a different requested format.
static void provide_clipboard_png(GtkClipboard *clipboard, GtkSelectionData *selection, guint info, gpointer data) {
  (void)clipboard;
  (void)info;
  WoxLinuxClipboardPNG *image = data;
  GdkAtom target = gtk_selection_data_get_target(selection);
  if (target == gdk_atom_intern_static_string("image/png")) {
    gsize length = 0;
    const guint8 *png = g_bytes_get_data(image->png, &length);
    gtk_selection_data_set(selection, target, 8, png, (gint)length);
    return;
  }
  GInputStream *input = g_memory_input_stream_new_from_bytes(image->png);
  GdkPixbuf *pixbuf = gdk_pixbuf_new_from_stream(input, NULL, NULL);
  g_object_unref(input);
  if (pixbuf != NULL) {
    gtk_selection_data_set_pixbuf(selection, pixbuf);
    g_object_unref(pixbuf);
  }
}

// Replacement by text, an external owner, or another image releases the previous encoded payload.
static void clear_clipboard_png(GtkClipboard *clipboard, gpointer data) {
  (void)clipboard;
  WoxLinuxClipboardPNG *image = data;
  if (clipboard_png == image) {
    clipboard_png = NULL;
  }
  g_bytes_unref(image->png);
  g_free(image);
}

// Persist PNG immediately without asking a clipboard manager to encode every compatibility format.
gboolean wox_clipboard_gtk_set_png(GtkClipboard *clipboard, const guint8 *png, gsize length) {
  if (clipboard == NULL || png == NULL || length == 0 || length > G_MAXINT) {
    return FALSE;
  }
  GtkTargetEntry png_target = {"image/png", 0, 0};
  GtkTargetList *list = gtk_target_list_new(&png_target, 1);
  gtk_target_list_add_image_targets(list, 0, TRUE);
  gint count = 0;
  GtkTargetEntry *targets = gtk_target_table_new_from_list(list, &count);
  WoxLinuxClipboardPNG *image = g_new0(WoxLinuxClipboardPNG, 1);
  image->clipboard = clipboard;
  // The cgo buffer belongs to Go and cannot outlive this call. Copy only its compressed representation.
  image->png = g_bytes_new(png, length);
  gboolean result = gtk_clipboard_set_with_data(clipboard, targets, count, provide_clipboard_png, clear_clipboard_png, image);
  gtk_target_table_free(targets, count);
  gtk_target_list_unref(list);
  if (!result) {
    clear_clipboard_png(clipboard, image);
    return FALSE;
  }
  clipboard_png = image;
  gtk_clipboard_set_can_store(clipboard, &png_target, 1);
  gtk_clipboard_store(clipboard);
  return TRUE;
}

// Complete legacy-format persistence on normal exit, only while this process still owns its PNG.
void wox_clipboard_gtk_flush(void) {
  if (clipboard_png == NULL) {
    return;
  }
  GtkClipboard *clipboard = clipboard_png->clipboard;
  gtk_clipboard_set_can_store(clipboard, NULL, 0);
  gtk_clipboard_store(clipboard);
}
