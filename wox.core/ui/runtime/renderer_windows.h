#ifndef WOX_UI_GO_RENDERER_WINDOWS_H
#define WOX_UI_GO_RENDERER_WINDOWS_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct WoxRenderer WoxRenderer;

int32_t wox_renderer_create(uintptr_t window_handle, uint32_t width, uint32_t height, WoxRenderer **renderer);
int32_t wox_renderer_resize(WoxRenderer *renderer, uint32_t width, uint32_t height);
int32_t wox_renderer_set_font_family(WoxRenderer *renderer, const char *font_family);
int32_t wox_renderer_begin_frame(WoxRenderer *renderer, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_fill_rounded_rect(WoxRenderer *renderer, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_stroke_rounded_rect(WoxRenderer *renderer, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_draw_text(WoxRenderer *renderer, const char *text, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_draw_image(WoxRenderer *renderer, const uint8_t *pixels, uint32_t image_width, uint32_t image_height, uint32_t row_stride, float x, float y, float width, float height);
int32_t wox_renderer_set_clip_rect(WoxRenderer *renderer, float x, float y, float width, float height);
int32_t wox_renderer_clear_clip(WoxRenderer *renderer);
int32_t wox_renderer_measure_text(WoxRenderer *renderer, const char *text, float font_size, uint8_t font_weight, float *width, float *height, float *baseline);
int32_t wox_renderer_end_frame(WoxRenderer *renderer);
void wox_renderer_destroy(WoxRenderer *renderer);

#ifdef __cplusplus
}
#endif

#endif
