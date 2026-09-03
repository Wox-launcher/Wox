#ifndef WOX_UI_GO_RENDERER_WINDOWS_H
#define WOX_UI_GO_RENDERER_WINDOWS_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct WoxRenderer WoxRenderer;

// WoxRendererGpuMemory carries what the graphics driver attributes to this process, already split
// by where the memory physically lives. System memory the driver holds on our behalf shows up in
// the process private working set; dedicated video memory does not. A unified memory adapter has
// no dedicated video memory at all, so its whole footprint counts against the process.
typedef struct WoxRendererGpuMemory {
  int64_t system_bytes;
  int64_t system_budget_bytes;
  int64_t dedicated_bytes;
  int64_t dedicated_budget_bytes;
  int32_t unified_memory;
} WoxRendererGpuMemory;

int32_t wox_renderer_process_gpu_memory(WoxRendererGpuMemory *memory);
int32_t wox_renderer_create(uintptr_t window_handle, uint32_t width, uint32_t height, int32_t enable_embedded_surface_overlay, int32_t force_warp, WoxRenderer **renderer);
int32_t wox_renderer_get_diagnostics(WoxRenderer *renderer, char *buffer, uint32_t buffer_size);
int64_t wox_renderer_resident_bytes(WoxRenderer *renderer);
int32_t wox_renderer_resize(WoxRenderer *renderer, uint32_t width, uint32_t height);
int32_t wox_renderer_set_font_family(WoxRenderer *renderer, const char *font_family);
int32_t wox_renderer_clear_image_cache(WoxRenderer *renderer);
int32_t wox_renderer_trim(WoxRenderer *renderer);
int32_t wox_renderer_begin_frame(WoxRenderer *renderer, float scale, float damage_x, float damage_y, float damage_width, float damage_height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_fill_rounded_rect(WoxRenderer *renderer, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_fill_convex_polygon(WoxRenderer *renderer, const float *points, int32_t point_count, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_stroke_rounded_rect(WoxRenderer *renderer, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_draw_text(WoxRenderer *renderer, const char *text, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha);
int32_t wox_renderer_draw_image(WoxRenderer *renderer, uint64_t image_id, const uint8_t *pixels, uint32_t image_width, uint32_t image_height, uint32_t row_stride, uint8_t pixel_format, float x, float y, float width, float height, float rotation_radians, float corner_radius);
int32_t wox_renderer_begin_embedded_surface_overlay(WoxRenderer *renderer);
int32_t wox_renderer_create_webview_visual(WoxRenderer *renderer, void **visual, void **root_visual_target);
int32_t wox_renderer_set_webview_visual_bounds(WoxRenderer *renderer, void *visual, float x, float y, float width, float height, float corner_radius);
int32_t wox_renderer_remove_webview_visual(WoxRenderer *renderer, void *visual);
int32_t wox_renderer_set_clip_rect(WoxRenderer *renderer, float x, float y, float width, float height);
int32_t wox_renderer_clear_clip(WoxRenderer *renderer);
int32_t wox_renderer_measure_text(WoxRenderer *renderer, const char *text, float font_size, uint8_t font_weight, uint8_t font_family, uint8_t italic, float *width, float *height, float *baseline);
int32_t wox_renderer_end_frame(WoxRenderer *renderer);
int32_t wox_renderer_simulate_device_removed(WoxRenderer *renderer);
void wox_renderer_destroy(WoxRenderer *renderer);

#ifdef __cplusplus
}
#endif

#endif
