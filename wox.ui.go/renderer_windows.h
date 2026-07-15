#ifndef WOX_UI_GO_RENDERER_WINDOWS_H
#define WOX_UI_GO_RENDERER_WINDOWS_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct WoxRenderer WoxRenderer;

int32_t wox_renderer_create(uintptr_t window_handle, uint32_t width, uint32_t height, WoxRenderer **renderer);
int32_t wox_renderer_resize(WoxRenderer *renderer, uint32_t width, uint32_t height);
int32_t wox_renderer_begin_frame(WoxRenderer *renderer, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_fill_rounded_rect(WoxRenderer *renderer, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_draw_text(WoxRenderer *renderer, const char *text, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_end_frame(WoxRenderer *renderer);
void wox_renderer_destroy(WoxRenderer *renderer);

#ifdef __cplusplus
}
#endif

#endif
