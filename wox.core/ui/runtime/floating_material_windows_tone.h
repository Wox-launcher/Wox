#pragma once

#include <d2d1_1.h>

// Compress backdrop deviations around the authored tint, keeping a uniform matching
// background unchanged. Direct2D unpremultiplies before this matrix and premultiplies
// afterwards, so the alpha row stays identity rather than adding an opaque veil.
inline D2D1_MATRIX_5X4_F floating_material_tone_matrix(float red, float green, float blue) {
  constexpr float contrast = 0.25f;
  constexpr float saturation = 0.5f;
  constexpr float r = (1.0f - saturation) * 0.2126f;
  constexpr float g = (1.0f - saturation) * 0.7152f;
  constexpr float b = (1.0f - saturation) * 0.0722f;
  const float luminance = r * red + g * green + b * blue;
  return {
      contrast * (r + saturation), contrast * r, contrast * r, 0,
      contrast * g, contrast * (g + saturation), contrast * g, 0,
      contrast * b, contrast * b, contrast * (b + saturation), 0,
      0, 0, 0, 1,
      red - contrast * (luminance + saturation * red),
      green - contrast * (luminance + saturation * green),
      blue - contrast * (luminance + saturation * blue), 0};
}
