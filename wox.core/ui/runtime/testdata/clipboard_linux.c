#define _POSIX_C_SOURCE 200809L
#include "../native_linux_clipboard.h"
#include <signal.h>
#include <stdio.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

// Keep failures in the integration-test process rather than application logging.
static int fail(const char *message) {
  fprintf(stderr, "%s\n", message);
  return 1;
}

// Check antialiased edges after GTK decodes PNG or a requested compatibility representation.
static gboolean image_matches(GdkPixbuf *image) {
  if (image == NULL || gdk_pixbuf_get_width(image) != 2 || gdk_pixbuf_get_height(image) != 1 ||
      !gdk_pixbuf_get_has_alpha(image) || gdk_pixbuf_get_n_channels(image) != 4) {
    return FALSE;
  }
  const guchar *pixels = gdk_pixbuf_read_pixels(image);
  return pixels[3] == 0 && pixels[4] == 80 && pixels[5] == 40 && pixels[6] == 20 && pixels[7] == 128;
}

// Verify canonical bytes and request a non-PNG format when the installed pixbuf codecs offer TIFF.
static gboolean clipboard_matches(GtkClipboard *clipboard, const guint8 *png, gsize length) {
  GtkSelectionData *selection = gtk_clipboard_wait_for_contents(clipboard, gdk_atom_intern_static_string("image/png"));
  gboolean matches = selection != NULL && gtk_selection_data_get_length(selection) == (gint)length &&
                     memcmp(gtk_selection_data_get_data(selection), png, length) == 0;
  if (selection != NULL) {
    gtk_selection_data_free(selection);
  }
  GdkPixbuf *image = gtk_clipboard_wait_for_image(clipboard);
  matches = matches && image_matches(image);
  if (image != NULL) {
    g_object_unref(image);
  }
  GdkAtom tiff = gdk_atom_intern_static_string("image/tiff");
  if (gtk_clipboard_wait_is_target_available(clipboard, tiff)) {
    selection = gtk_clipboard_wait_for_contents(clipboard, tiff);
    GdkPixbufLoader *loader = gdk_pixbuf_loader_new();
    matches = selection != NULL && gtk_selection_data_get_length(selection) > 0 &&
              gdk_pixbuf_loader_write(loader, gtk_selection_data_get_data(selection),
                                      (gsize)gtk_selection_data_get_length(selection), NULL) &&
              gdk_pixbuf_loader_close(loader, NULL) && image_matches(gdk_pixbuf_loader_get_pixbuf(loader)) && matches;
    if (selection != NULL) {
      gtk_selection_data_free(selection);
    }
    g_object_unref(loader);
  }
  return matches;
}

// Service real cross-process selection requests while the publishing process owns the PNG.
static gboolean run_reader(const char *binary, const char *path) {
  pid_t child = fork();
  if (child < 0) {
    return FALSE;
  }
  if (child == 0) {
    execl(binary, binary, "read", path, (char *)NULL);
    _exit(127);
  }
  gint64 deadline = g_get_monotonic_time() + 5 * G_TIME_SPAN_SECOND;
  int status = 0;
  while (g_get_monotonic_time() < deadline) {
    pid_t result = waitpid(child, &status, WNOHANG);
    if (result != 0) {
      return result == child && WIFEXITED(status) && WEXITSTATUS(status) == 0;
    }
    while (g_main_context_iteration(NULL, FALSE)) {
    }
    g_usleep(1000);
  }
  kill(child, SIGTERM);
  waitpid(child, &status, 0);
  return FALSE;
}

// Exercise publication without a live Go buffer, replacement cleanup, and normal-exit persistence.
int main(int argc, char *argv[]) {
  if (argc != 3 || !gtk_init_check(&argc, &argv)) {
    return fail("expected mode and PNG path under a Linux display");
  }
  gchar *png = NULL;
  gsize length = 0;
  if (!g_file_get_contents(argv[2], &png, &length, NULL)) {
    return fail("cannot read PNG fixture");
  }
  GtkClipboard *clipboard = gtk_clipboard_get_default(gdk_display_get_default());
  if (strcmp(argv[1], "read") == 0) {
    gboolean matches = clipboard_matches(clipboard, (const guint8 *)png, length);
    g_free(png);
    return matches ? 0 : fail("PNG or compatibility format changed pixels");
  }
  guint8 *temporary = g_malloc(length);
  memcpy(temporary, png, length);
  if (!wox_linux_clipboard_set_png(clipboard, temporary, length)) {
    return fail("cannot publish PNG");
  }
  memset(temporary, 0, length);
  g_free(temporary);
  if (wox_linux_clipboard_set_png(clipboard, NULL, length) ||
      wox_linux_clipboard_set_png(clipboard, (const guint8 *)png, 0) ||
      wox_linux_clipboard_set_png(clipboard, (const guint8 *)png, (gsize)G_MAXINT + 1) ||
      !run_reader(argv[0], argv[2])) {
    return fail("publication retained caller pixels or failed to serve another process");
  }
  gtk_clipboard_set_text(clipboard, "new clipboard owner", -1);
  wox_linux_clipboard_flush_png();
  gchar *text = gtk_clipboard_wait_for_text(clipboard);
  gboolean replaced = g_strcmp0(text, "new clipboard owner") == 0;
  g_free(text);
  if (!replaced || !wox_linux_clipboard_set_png(clipboard, (const guint8 *)png, length)) {
    return fail("flush changed a newer clipboard write or republishing failed");
  }
  wox_linux_clipboard_flush_png();
  printf("persistence=%d\n", gdk_display_supports_clipboard_persistence(gdk_display_get_default()));
  g_free(png);
  return 0;
}
