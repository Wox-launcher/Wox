//go:build windows && cgo

#include "png_rows_windows.h"
#include <string.h>

// Normalize only premultiplied RGBA edges; NRGBA and 16-bit rows retain their exact channel bytes.
void wox_filter_png_row(uint8_t *destination, const uint8_t *source, size_t bytes, int premultiplied, size_t bytes_per_pixel) {
  memcpy(destination, source, bytes);
  if (premultiplied) {
    for (size_t x = 0; x < bytes; x += 4) {
      uint32_t alpha = destination[x + 3];
      if (alpha == 0) {
        destination[x] = destination[x + 1] = destination[x + 2] = 0;
      } else if (alpha != 255) {
        // Match Go's NRGBAModel precision before truncating to an eight-bit channel.
        for (size_t channel = x; channel < x + 3; ++channel) {
          destination[channel] = (uint8_t)((destination[channel] * 65535U / alpha) >> 8);
        }
      }
    }
  }
  // Reverse traversal keeps the left pixel intact until its predictor has been consumed.
  for (size_t x = bytes; x > bytes_per_pixel; --x) {
    destination[x - 1] -= destination[x - 1 - bytes_per_pixel];
  }
}
