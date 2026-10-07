#ifndef WOX_UI_DESKTOP_DUPLICATION_WINDOWS_H
#define WOX_UI_DESKTOP_DUPLICATION_WINDOWS_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct WoxDXGIRectCapturer WoxDXGIRectCapturer;

int32_t wox_dxgi_rect_capturer_create(int32_t x, int32_t y, int32_t width, int32_t height, WoxDXGIRectCapturer **capturer);
int32_t wox_dxgi_rect_capturer_capture(WoxDXGIRectCapturer *capturer, uint8_t *bgra, int32_t stride);
void wox_dxgi_rect_capturer_destroy(WoxDXGIRectCapturer *capturer);

#ifdef __cplusplus
}
#endif

#endif
