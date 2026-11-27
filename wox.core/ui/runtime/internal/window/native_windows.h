#ifndef WOX_UI_GO_NATIVE_WINDOWS_H
#define WOX_UI_GO_NATIVE_WINDOWS_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

int32_t wox_windows_pick_file(uintptr_t owner, int32_t directory, const wchar_t *filter, char **path);
int32_t wox_windows_save_file(uintptr_t owner, const char *title, const char *default_name, const char *extension, char **path);
int32_t wox_windows_start_file_drag(uintptr_t owner, const char *const *paths, int32_t path_count);
int32_t wox_windows_accessibility_has_active_client(uintptr_t owner);
// The payload is one little-endian blob describing every node; see encodeWindowsAccessibilityTree.
int32_t wox_windows_accessibility_update(uintptr_t owner, uint64_t generation, const uint8_t *payload, int32_t payload_len);
uintptr_t wox_windows_accessibility_get_object(uintptr_t owner, uintptr_t wparam, uintptr_t lparam);
void wox_windows_accessibility_remove(uintptr_t owner);

typedef struct WoxWindowsWebView WoxWindowsWebView;
int32_t wox_windows_webview_evaluate(WoxWindowsWebView *target, const char *script, uint64_t callback);
typedef struct WoxRenderer WoxRenderer;
int32_t wox_windows_webview_create(uintptr_t owner, WoxRenderer *renderer, WoxWindowsWebView **webview);
int32_t wox_windows_webview_show(WoxWindowsWebView *webview, const char *url, const char *html, const char *inject_css, const char *user_agent, int32_t cache_disabled, const char *cache_key, int32_t x, int32_t y, int32_t width, int32_t height, float corner_radius);
int32_t wox_windows_webview_hide(WoxWindowsWebView *webview);
int32_t wox_windows_webview_go_back(WoxWindowsWebView *webview);
int32_t wox_windows_webview_go_forward(WoxWindowsWebView *webview);
int32_t wox_windows_webview_reload(WoxWindowsWebView *webview);
int32_t wox_windows_webview_open_dev_tools(WoxWindowsWebView *webview);
int32_t wox_windows_webview_navigation_state(WoxWindowsWebView *webview, char **url, int32_t *can_go_back, int32_t *can_go_forward);
int32_t wox_windows_webview_pointer(WoxWindowsWebView *webview, int32_t kind, int32_t x, int32_t y, int32_t button, int32_t scroll_x, int32_t scroll_y, uint32_t modifiers);
int32_t wox_windows_webview_focus(WoxWindowsWebView *webview);
void wox_windows_webview_destroy(WoxWindowsWebView *webview);
int32_t wox_windows_webview_evict(WoxWindowsWebView *webview, const char *cache_key);
void wox_windows_free_string(char *value);

typedef struct WoxWindowsFilePreview WoxWindowsFilePreview;
int32_t wox_windows_file_preview_create(uintptr_t owner, const char *path, int32_t x, int32_t y, int32_t width, int32_t height, WoxWindowsFilePreview **preview);
int32_t wox_windows_file_preview_show(WoxWindowsFilePreview *preview, int32_t x, int32_t y, int32_t width, int32_t height);
int32_t wox_windows_file_preview_hide(WoxWindowsFilePreview *preview);
int32_t wox_windows_file_preview_set_clip(WoxWindowsFilePreview *preview, int32_t left, int32_t top, int32_t right, int32_t bottom, int32_t corner_diameter);
void wox_windows_file_preview_destroy(WoxWindowsFilePreview *preview);

#ifdef __cplusplus
}
#endif

#endif
