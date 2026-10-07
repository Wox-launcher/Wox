#ifndef WOX_CLIPBOARD_IMAGE_WINDOWS_H
#define WOX_CLIPBOARD_IMAGE_WINDOWS_H
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
int32_t wox_windows_write_clipboard_text(uintptr_t owner, const char *text);
typedef struct WoxWindowsClipboardImage WoxWindowsClipboardImage;
int32_t wox_windows_prepare_clipboard_image(const uint8_t *pixels, uint32_t width, uint32_t height,
                                          uint32_t row_stride, int32_t premultiplied,
                                          const uint8_t *png, uint32_t png_size, WoxWindowsClipboardImage **image);
int32_t wox_windows_publish_clipboard_image(uintptr_t owner, WoxWindowsClipboardImage *image);
void wox_windows_destroy_clipboard_image(WoxWindowsClipboardImage *image);
#ifdef __cplusplus
}
#endif
#endif
