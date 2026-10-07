//go:build windows

#include "image_bgra_windows.h"

// Desktop capture's fourth byte is padding, so detached RGBA must always be opaque.
void wox_copy_bgrx_to_rgba(const uint8_t *source, uint8_t *destination, size_t pixels) {
  for (size_t x = 0; x < pixels; ++x) {
    destination[x * 4] = source[x * 4 + 2];
    destination[x * 4 + 1] = source[x * 4 + 1];
    destination[x * 4 + 2] = source[x * 4];
    destination[x * 4 + 3] = 255;
  }
}
