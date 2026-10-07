#ifndef WOX_SCREENSHOT_OBJECT_SELECTION_WINDOWS_H
#define WOX_SCREENSHOT_OBJECT_SELECTION_WINDOWS_H
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
  int32_t left, top, right, bottom;
} WoxScreenshotElementRect;
int32_t wox_windows_screenshot_elements(uintptr_t window, int32_t x, int32_t y, uint32_t budget_ms, uintptr_t cancellation,
                                        WoxScreenshotElementRect *rects, int32_t capacity);
#ifdef __cplusplus
}
#endif
#endif
