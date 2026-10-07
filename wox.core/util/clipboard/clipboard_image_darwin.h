#ifndef WOX_CLIPBOARD_IMAGE_DARWIN_H
#define WOX_CLIPBOARD_IMAGE_DARWIN_H
#include <stdint.h>
#include <stddef.h>
int32_t wox_clipboard_darwin_write_text(const char *text);
int32_t wox_clipboard_darwin_write_pixels(const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride);
int32_t wox_clipboard_darwin_write_png(const uint8_t *png, size_t length);
int32_t wox_clipboard_darwin_flush(void);
#endif
