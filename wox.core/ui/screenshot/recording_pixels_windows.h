#pragma once

#include <stdint.h>

void wox_recording_bgra_to_i420(const uint8_t *source, int stride, int width, int height, uint8_t *destination);
