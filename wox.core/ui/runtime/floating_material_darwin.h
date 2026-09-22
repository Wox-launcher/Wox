#ifndef WOX_FLOATING_MATERIAL_DARWIN_H
#define WOX_FLOATING_MATERIAL_DARWIN_H

#include <CoreGraphics/CoreGraphics.h>
#include <stdbool.h>
#include <stdint.h>

bool wox_darwin_blur_material(CGContextRef context, float scale, CGRect bounds, float radius, float sigma, float margin, uint8_t red, uint8_t green, uint8_t blue);

#endif
