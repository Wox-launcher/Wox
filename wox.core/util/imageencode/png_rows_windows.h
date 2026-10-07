#ifndef WOX_IMAGEENCODE_PNG_ROWS_WINDOWS_H
#define WOX_IMAGEENCODE_PNG_ROWS_WINDOWS_H
#include <stddef.h>
#include <stdint.h>
void wox_filter_png_row(uint8_t *destination, const uint8_t *source, size_t bytes, int premultiplied, size_t bytes_per_pixel);
#endif
