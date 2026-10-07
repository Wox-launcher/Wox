#ifndef WOX_UI_NATIVE_RENDERER_STATS_H
#define WOX_UI_NATIVE_RENDERER_STATS_H

#include <stdint.h>

typedef struct {
  int32_t text_rasterizations;
  int32_t image_creates;
  int32_t image_uploads;
  int32_t cache_hits;
  int32_t cache_evictions;
  int64_t resident_bytes;
} WoxRendererResourceStats;

#endif
