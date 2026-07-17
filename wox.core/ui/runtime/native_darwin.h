#ifndef WOX_UI_GO_NATIVE_DARWIN_H
#define WOX_UI_GO_NATIVE_DARWIN_H

#include <stdint.h>

typedef struct WoxDarwinWindow WoxDarwinWindow;

int32_t wox_darwin_run(uintptr_t context);
int32_t wox_darwin_call(uintptr_t context);
WoxDarwinWindow *wox_darwin_window_create(const char *title, float width, float height, int32_t hide_on_blur, int32_t application_window, uintptr_t context);
uint64_t wox_darwin_window_show(WoxDarwinWindow *window);
int32_t wox_darwin_window_hide(WoxDarwinWindow *window);
int32_t wox_darwin_window_set_bounds(WoxDarwinWindow *window, float x, float y, float width, float height);
int32_t wox_darwin_window_get_bounds(WoxDarwinWindow *window, float *x, float *y, float *width, float *height);
int32_t wox_darwin_window_capture_png(WoxDarwinWindow *window, const char *path);
int32_t wox_darwin_window_center(WoxDarwinWindow *window, float width, float height);
int32_t wox_darwin_window_start_dragging(WoxDarwinWindow *window);
int32_t wox_darwin_window_minimize(WoxDarwinWindow *window);
int32_t wox_darwin_window_set_hide_on_blur(WoxDarwinWindow *window, int32_t enabled);
int32_t wox_darwin_window_set_appearance(WoxDarwinWindow *window, int32_t is_dark);
int32_t wox_darwin_window_pick_file(WoxDarwinWindow *window, int32_t directory, char **path);
int32_t wox_darwin_window_open_external_url(WoxDarwinWindow *window, const char *url);
int32_t wox_darwin_window_show_webview(WoxDarwinWindow *window, const char *url, const char *html, const char *inject_css, int32_t cache_disabled, const char *cache_key, float x, float y, float width, float height);
int32_t wox_darwin_window_hide_webview(WoxDarwinWindow *window);
int32_t wox_darwin_window_write_clipboard_text(WoxDarwinWindow *window, const char *text);
int32_t wox_darwin_window_write_clipboard_image(WoxDarwinWindow *window, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride);
int32_t wox_darwin_window_invalidate(WoxDarwinWindow *window);
int32_t wox_darwin_window_set_text_input_state(WoxDarwinWindow *window, int32_t enabled, float x, float y, float width, float height);
int32_t wox_darwin_accessibility_begin(WoxDarwinWindow *window, uint64_t generation);
int32_t wox_darwin_accessibility_add_node(WoxDarwinWindow *window, uint64_t id, uint64_t parent_id, const uint64_t *children, int32_t child_count, const char *automation_id, const char *role, const char *label, const char *description, const char *value, float x, float y, float width, float height, uint32_t state_flags, uint32_t action_flags, int32_t live_region);
int32_t wox_darwin_accessibility_end(WoxDarwinWindow *window);
int32_t wox_darwin_window_measure_text(WoxDarwinWindow *window, const char *text, const char *font_family, float font_size, uint8_t font_weight, float *width, float *height, float *baseline);
int32_t wox_darwin_window_close(WoxDarwinWindow *window);
void *wox_darwin_autorelease_pool_push(void);
void wox_darwin_autorelease_pool_pop(void *pool);

int32_t wox_darwin_window_begin_frame(WoxDarwinWindow *window, float logical_width, float logical_height, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_darwin_window_fill_rounded_rect(WoxDarwinWindow *window, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_darwin_window_fill_convex_polygon(WoxDarwinWindow *window, const float *points, int32_t point_count, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_darwin_window_stroke_rounded_rect(WoxDarwinWindow *window, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_darwin_window_draw_text(WoxDarwinWindow *window, const char *text, const char *font_family, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_darwin_window_draw_image(WoxDarwinWindow *window, uint64_t image_id, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height);
int32_t wox_darwin_window_set_clip_rect(WoxDarwinWindow *window, float x, float y, float width, float height);
int32_t wox_darwin_window_clear_clip(WoxDarwinWindow *window);
int32_t wox_darwin_window_end_frame(WoxDarwinWindow *window, int32_t transactional);

#endif
