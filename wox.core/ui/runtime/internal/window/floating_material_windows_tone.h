#pragma once

#include <d2d1_1.h>

// Compress backdrop deviations around the authored tint, keeping a uniform matching
// background unchanged. Direct2D unpremultiplies before this matrix and premultiplies
// afterwards, so the alpha row stays identity rather than adding an opaque veil.
inline D2D1_MATRIX_5X4_F floating_material_tone_matrix(float red, float green, float blue, float brightness = 1.0f, float chroma = 1.0f) {
  constexpr float contrast = 0.25f;
  constexpr float saturation = 0.5f;
  constexpr float r = (1.0f - saturation) * 0.2126f;
  constexpr float g = (1.0f - saturation) * 0.7152f;
  constexpr float b = (1.0f - saturation) * 0.0722f;
  const float luminance = r * red + g * green + b * blue;
  D2D1_MATRIX_5X4_F result = {
      contrast * (r + saturation), contrast * r, contrast * r, 0,
      contrast * g, contrast * (g + saturation), contrast * g, 0,
      contrast * b, contrast * b, contrast * (b + saturation), 0,
      0, 0, 0, 1,
      red - contrast * (luminance + saturation * red),
      green - contrast * (luminance + saturation * green),
      blue - contrast * (luminance + saturation * blue), 0};
  // Transform RGB coefficients, including tint bias, without touching the alpha column.
  if (brightness != 1.0f || chroma != 1.0f) {
    for (int row = 0; row < 5; row++) {
      const float luma = 0.2126f * result.m[row][0] + 0.7152f * result.m[row][1] + 0.0722f * result.m[row][2];
      for (int channel = 0; channel < 3; channel++) {
        result.m[row][channel] = brightness * (luma + chroma * (result.m[row][channel] - luma));
      }
    }
  }
  return result;
}
