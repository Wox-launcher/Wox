#ifndef WOX_UI_GO_NATIVE_WINDOWS_H
#define WOX_UI_GO_NATIVE_WINDOWS_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

int32_t wox_windows_pick_file(uintptr_t owner, int32_t directory, char **path);
int32_t wox_windows_write_clipboard_text(uintptr_t owner, const char *text);
int32_t wox_windows_write_clipboard_image(uintptr_t owner, const uint8_t *pixels, uint32_t width, uint32_t height, uint32_t row_stride, const uint8_t *png, uint32_t png_size);
int32_t wox_windows_accessibility_begin(uintptr_t owner, uint64_t generation);
int32_t wox_windows_accessibility_add_node(uintptr_t owner, uint64_t id, uint64_t parent_id, const uint64_t *children, int32_t child_count, const char *automation_id, const char *role, const char *label, const char *description, const char *value, float x, float y, float width, float height, uint32_t state_flags, uint32_t action_flags, int32_t live_region);
int32_t wox_windows_accessibility_end(uintptr_t owner);
uintptr_t wox_windows_accessibility_get_object(uintptr_t owner, uintptr_t wparam, uintptr_t lparam);
void wox_windows_accessibility_remove(uintptr_t owner);

typedef struct WoxWindowsWebView WoxWindowsWebView;
int32_t wox_windows_webview_create(uintptr_t owner, WoxWindowsWebView **webview);
int32_t wox_windows_webview_show(WoxWindowsWebView *webview, const char *url, const char *html, const char *inject_css, int32_t cache_disabled, const char *cache_key, int32_t x, int32_t y, int32_t width, int32_t height);
int32_t wox_windows_webview_hide(WoxWindowsWebView *webview);
void wox_windows_webview_destroy(WoxWindowsWebView *webview);
void wox_windows_free_string(char *value);

#ifdef __cplusplus
}
#endif

#endif
