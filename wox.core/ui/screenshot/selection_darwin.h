#ifndef WOX_SCREENSHOT_SELECTION_DARWIN_H
#define WOX_SCREENSHOT_SELECTION_DARWIN_H

#include <stdint.h>

int32_t wox_darwin_select_screenshot_region(int32_t *pixel_width, int32_t *pixel_height, uintptr_t *session_handle, uint32_t *display_id, float *display_x, float *display_y, float *display_width, float *display_height, float *selection_x, float *selection_y, float *selection_width, float *selection_height, char **copied_color);
int32_t wox_darwin_test_screenshot_rgba(void *pixels);
int32_t wox_darwin_test_screenshot_window_selection(void);
int32_t wox_darwin_copy_screenshot_selection_rgba(uintptr_t session_handle, int32_t width, int32_t height, void *pixels);
int32_t wox_darwin_copy_screenshot_window_rgba(uintptr_t session_handle, int32_t *width, int32_t *height, void *pixels);
void wox_darwin_dismiss_screenshot_selection(uintptr_t session_handle);
uintptr_t wox_darwin_show_screenshot_border(float x, float y, float width, float height, float thickness);
void wox_darwin_dismiss_screenshot_border(uintptr_t border_handle);
int32_t wox_darwin_test_screenshot_pixel_at_point(int32_t image_width, int32_t image_height, float frame_width, float frame_height, float x, float y, int32_t *pixel_x, int32_t *pixel_y);
int32_t wox_darwin_test_screenshot_inspector_rect(float frame_width, float frame_height, float pointer_x, float pointer_y, float panel_width, float panel_height, float ui_scale, float *x, float *y, float *width, float *height);
int32_t wox_darwin_test_screenshot_color_shortcut(uint16_t key_code, int32_t *as_hex);

#endif
