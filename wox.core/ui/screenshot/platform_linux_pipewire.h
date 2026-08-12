#ifndef WOX_SCREENSHOT_PLATFORM_LINUX_PIPEWIRE_H
#define WOX_SCREENSHOT_PLATFORM_LINUX_PIPEWIRE_H

#include <stdint.h>

typedef struct {
  uint32_t width;
  uint32_t height;
  uint32_t stride;
  uint8_t *pixels;
} WoxPipeWireFrame;

typedef struct WoxPipeWireCapture WoxPipeWireCapture;

WoxPipeWireCapture *wox_screenshot_pipewire_create(int32_t remote_fd, const uint32_t *node_ids, int32_t node_count);
int32_t wox_screenshot_pipewire_capture(WoxPipeWireCapture *capture, WoxPipeWireFrame *frames, int32_t frame_count, int32_t timeout_seconds);
void wox_screenshot_pipewire_destroy(WoxPipeWireCapture *capture);
void wox_screenshot_pipewire_free_frames(WoxPipeWireFrame *frames, int32_t frame_count);

#endif
