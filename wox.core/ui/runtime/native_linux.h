#ifndef WOX_UI_GO_NATIVE_LINUX_H
#define WOX_UI_GO_NATIVE_LINUX_H

#include <stdint.h>

#include "native_renderer_stats.h"

typedef struct WoxLinuxWindow WoxLinuxWindow;

int32_t wox_linux_run(uintptr_t context);
int32_t wox_linux_call(uintptr_t context);
// wox_linux_set_app_identity records the desktop id, X11 class, and icon path before gtk_init.
void wox_linux_set_app_identity(const char *app_id, const char *wm_class, const char *icon_path);
void wox_linux_set_render_trace(int32_t enabled);
WoxLinuxWindow *wox_linux_window_create(const char *title, float width, float height, int32_t hide_on_blur, int32_t window_role, int32_t nonactivating, int32_t resizable, float aspect_ratio, uintptr_t context);
uint64_t wox_linux_window_show(WoxLinuxWindow *window);
int32_t wox_linux_window_hide(WoxLinuxWindow *window);
int32_t wox_linux_window_set_bounds(WoxLinuxWindow *window, float x, float y, float width, float height);
int32_t wox_linux_window_get_bounds(WoxLinuxWindow *window, float *x, float *y, float *width, float *height);
int32_t wox_linux_window_capture_png(WoxLinuxWindow *window, const char *path);
int32_t wox_linux_capture_desktop_png(const char *path, float *x, float *y, float *width, float *height);
int32_t wox_linux_window_center(WoxLinuxWindow *window, float width, float height);
int32_t wox_linux_window_start_dragging(WoxLinuxWindow *window);
int32_t wox_linux_window_minimize(WoxLinuxWindow *window);
int32_t wox_linux_window_set_hide_on_blur(WoxLinuxWindow *window, int32_t enabled);
int32_t wox_linux_window_set_topmost(WoxLinuxWindow *window, int32_t enabled);
int32_t wox_linux_window_set_min_size(WoxLinuxWindow *window, float width, float height);
int32_t wox_linux_window_set_icon(WoxLinuxWindow *window, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride);
int32_t wox_linux_window_pick_file(WoxLinuxWindow *window, int32_t directory, char **path);
int32_t wox_linux_window_save_file(WoxLinuxWindow *window, const char *title, const char *default_name, const char *extension, char **path);
int32_t wox_linux_window_set_pointer_passthrough(WoxLinuxWindow *window, int32_t enabled);
int32_t wox_linux_window_open_external_url(WoxLinuxWindow *window, const char *url);
int32_t wox_linux_window_show_webview(WoxLinuxWindow *window, const char *url, const char *html, const char *inject_css, const char *user_agent, int32_t cache_disabled, const char *cache_key, float x, float y, float width, float height, float corner_radius);
int32_t wox_linux_window_hide_webview(WoxLinuxWindow *window);
int32_t wox_linux_window_reset_webview(WoxLinuxWindow *window);
int32_t wox_linux_window_forward_embedded_surface_pointer(WoxLinuxWindow *window, uint8_t kind, float x, float y);
void wox_linux_free_string(char *value);
int32_t wox_linux_window_write_clipboard_text(WoxLinuxWindow *window, const char *text);
int32_t wox_linux_window_write_clipboard_image(WoxLinuxWindow *window, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride);
int32_t wox_linux_window_invalidate(WoxLinuxWindow *window);
int32_t wox_linux_window_request_animation_frame(WoxLinuxWindow *window);
int32_t wox_linux_window_stop_animation_frames(WoxLinuxWindow *window);
int32_t wox_linux_window_set_text_input_state(WoxLinuxWindow *window, int32_t enabled, float x, float y, float width, float height);
int32_t wox_linux_window_set_pointer_cursor(WoxLinuxWindow *window, uint8_t cursor);
int32_t wox_linux_accessibility_begin(WoxLinuxWindow *window, uint64_t generation);
int32_t wox_linux_accessibility_add_node(WoxLinuxWindow *window, uint64_t id, uint64_t parent_id, const uint64_t *children, int32_t child_count, const char *automation_id, const char *role, const char *label, const char *description, const char *value, float x, float y, float width, float height, uint32_t state_flags, uint32_t action_flags, int32_t live_region);
int32_t wox_linux_accessibility_end(WoxLinuxWindow *window);
int32_t wox_linux_window_measure_text(WoxLinuxWindow *window, const char *text, const char *font_family, float font_size, uint8_t font_weight, uint8_t italic, float *width, float *height, float *baseline);
int32_t wox_linux_window_close(WoxLinuxWindow *window);

int32_t wox_linux_window_begin_frame(WoxLinuxWindow *window, uint64_t frame_id, float logical_width, float logical_height, float scale, float damage_x, float damage_y, float damage_width, float damage_height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_begin_embedded_surface_overlay(WoxLinuxWindow *window);
int32_t wox_linux_window_fill_rounded_rect(WoxLinuxWindow *window, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_fill_convex_polygon(WoxLinuxWindow *window, const float *points, int32_t point_count, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_stroke_rounded_rect(WoxLinuxWindow *window, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_draw_text(WoxLinuxWindow *window, const char *text, const char *font_family, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t italic, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_linux_window_draw_image(WoxLinuxWindow *window, uint64_t image_id, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height, float rotation_radians, float corner_radius);
int32_t wox_linux_window_set_clip_rect(WoxLinuxWindow *window, float x, float y, float width, float height);
int32_t wox_linux_window_clear_clip(WoxLinuxWindow *window);
void wox_linux_window_trace_encode(WoxLinuxWindow *window);
int32_t wox_linux_window_end_frame(WoxLinuxWindow *window);
int32_t wox_linux_window_take_frame_resource_stats(WoxLinuxWindow *window, WoxRendererResourceStats *out);
int32_t wox_linux_test_resource_cache_generation(void);
int32_t wox_linux_test_resize_hit(float x, float y, int32_t width, int32_t height, int32_t grip);
int32_t wox_linux_test_layer_shell_stack_layer(int32_t topmost, int32_t screenshot);

#endif
