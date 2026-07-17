#ifndef WOX_UI_GO_NATIVE_LINUX_H
#define WOX_UI_GO_NATIVE_LINUX_H

#include <stdint.h>

typedef struct WoxLinuxWindow WoxLinuxWindow;

int32_t wox_linux_run(uintptr_t context);
int32_t wox_linux_call(uintptr_t context);
WoxLinuxWindow *wox_linux_window_create(const char *title, float width, float height, int32_t hide_on_blur, int32_t application_window, uintptr_t context);
uint64_t wox_linux_window_show(WoxLinuxWindow *window);
int32_t wox_linux_window_hide(WoxLinuxWindow *window);
int32_t wox_linux_window_set_bounds(WoxLinuxWindow *window, float x, float y, float width, float height);
int32_t wox_linux_window_get_bounds(WoxLinuxWindow *window, float *x, float *y, float *width, float *height);
int32_t wox_linux_window_capture_png(WoxLinuxWindow *window, const char *path);
int32_t wox_linux_window_center(WoxLinuxWindow *window, float width, float height);
int32_t wox_linux_window_start_dragging(WoxLinuxWindow *window);
int32_t wox_linux_window_minimize(WoxLinuxWindow *window);
int32_t wox_linux_window_set_hide_on_blur(WoxLinuxWindow *window, int32_t enabled);
int32_t wox_linux_window_pick_file(WoxLinuxWindow *window, int32_t directory, char **path);
int32_t wox_linux_window_open_external_url(WoxLinuxWindow *window, const char *url);
int32_t wox_linux_window_show_webview(WoxLinuxWindow *window, const char *url, const char *html, const char *inject_css, int32_t cache_disabled, const char *cache_key, float x, float y, float width, float height);
int32_t wox_linux_window_hide_webview(WoxLinuxWindow *window);
void wox_linux_free_string(char *value);
int32_t wox_linux_window_write_clipboard_text(WoxLinuxWindow *window, const char *text);
int32_t wox_linux_window_write_clipboard_image(WoxLinuxWindow *window, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride);
int32_t wox_linux_window_invalidate(WoxLinuxWindow *window);
int32_t wox_linux_window_set_text_input_state(WoxLinuxWindow *window, int32_t enabled, float x, float y, float width, float height);
int32_t wox_linux_accessibility_begin(WoxLinuxWindow *window, uint64_t generation);
int32_t wox_linux_accessibility_add_node(WoxLinuxWindow *window, uint64_t id, uint64_t parent_id, const uint64_t *children, int32_t child_count, const char *automation_id, const char *role, const char *label, const char *description, const char *value, float x, float y, float width, float height, uint32_t state_flags, uint32_t action_flags, int32_t live_region);
int32_t wox_linux_accessibility_end(WoxLinuxWindow *window);
int32_t wox_linux_window_measure_text(WoxLinuxWindow *window, const char *text, const char *font_family, float font_size, uint8_t font_weight, float *width, float *height, float *baseline);
int32_t wox_linux_window_close(WoxLinuxWindow *window);

int32_t wox_linux_window_begin_frame(WoxLinuxWindow *window, float logical_width, float logical_height, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_fill_rounded_rect(WoxLinuxWindow *window, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_fill_convex_polygon(WoxLinuxWindow *window, const float *points, int32_t point_count, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_stroke_rounded_rect(WoxLinuxWindow *window, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_draw_text(WoxLinuxWindow *window, const char *text, const char *font_family, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_draw_image(WoxLinuxWindow *window, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height);
int32_t wox_linux_window_set_clip_rect(WoxLinuxWindow *window, float x, float y, float width, float height);
int32_t wox_linux_window_clear_clip(WoxLinuxWindow *window);
int32_t wox_linux_window_end_frame(WoxLinuxWindow *window);

#endif
