#ifndef WOX_UI_GO_NATIVE_DARWIN_H
#define WOX_UI_GO_NATIVE_DARWIN_H

#include <stdint.h>

typedef struct WoxDarwinWindow WoxDarwinWindow;

int32_t wox_darwin_run(uintptr_t context);
WoxDarwinWindow *wox_darwin_window_create(const char *title, float width, float height, int32_t hide_on_blur, uintptr_t context);
uint64_t wox_darwin_window_show(WoxDarwinWindow *window);
int32_t wox_darwin_window_hide(WoxDarwinWindow *window);
int32_t wox_darwin_window_invalidate(WoxDarwinWindow *window);
int32_t wox_darwin_window_close(WoxDarwinWindow *window);

int32_t wox_darwin_window_begin_frame(WoxDarwinWindow *window, float logical_width, float logical_height, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_darwin_window_fill_rounded_rect(WoxDarwinWindow *window, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_darwin_window_draw_text(WoxDarwinWindow *window, const char *text, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_darwin_window_end_frame(WoxDarwinWindow *window);

#endif
