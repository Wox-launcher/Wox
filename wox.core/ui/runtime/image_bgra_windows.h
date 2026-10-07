#ifndef WOX_UI_IMAGE_BGRA_WINDOWS_H
#define WOX_UI_IMAGE_BGRA_WINDOWS_H
#include <stddef.h>
#include <stdint.h>
void wox_copy_bgrx_to_rgba(const uint8_t *source, uint8_t *destination, size_t pixels);
#endif
