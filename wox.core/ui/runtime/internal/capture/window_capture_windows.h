#pragma once
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
int32_t wox_capture_windows_window(uintptr_t hwnd, int32_t width, int32_t height, uint8_t *rgba);
#ifdef __cplusplus
}
#endif
