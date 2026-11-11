#include "recording_pixels_windows.h"
#include <stddef.h>
#include <string.h>

#if defined(__x86_64__)
#include <tmmintrin.h>

__attribute__((target("ssse3")))
static int write_luma_ssse3(const uint8_t *pixels, uint8_t *y, int width) {
  // PMADDUBSW uses signed byte coefficients, so add the green coefficient in two parts.
  const __m128i coefficients = _mm_set1_epi32(25 | (65 << 8) | (66 << 16));
  const __m128i extra_green = _mm_set1_epi32(64 << 8);
  const __m128i round = _mm_set1_epi16(128);
  const __m128i offset = _mm_set1_epi16(16);
  int x = 0;
  for (; x + 8 <= width; x += 8) {
    __m128i first = _mm_loadu_si128((const __m128i *)(pixels + 4 * x));
    __m128i second = _mm_loadu_si128((const __m128i *)(pixels + 4 * x + 16));
    __m128i a = _mm_add_epi16(_mm_maddubs_epi16(first, coefficients), _mm_maddubs_epi16(first, extra_green));
    __m128i b = _mm_add_epi16(_mm_maddubs_epi16(second, coefficients), _mm_maddubs_epi16(second, extra_green));
    __m128i sum = _mm_hadd_epi16(a, b);
    __m128i luma = _mm_add_epi16(_mm_srli_epi16(_mm_add_epi16(sum, round), 8), offset);
    _mm_storel_epi64((__m128i *)(y + x), _mm_packus_epi16(luma, luma));
  }
  return x;
}

__attribute__((target("ssse3")))
static int write_chroma_ssse3(const uint8_t *pixels, uint8_t *u, uint8_t *v, int width) {
  const __m128i u_coeff = _mm_setr_epi8(112, -74, -38, 0, 112, -74, -38, 0, 112, -74, -38, 0, 112, -74, -38, 0);
  const __m128i v_coeff = _mm_setr_epi8(-18, -94, 112, 0, -18, -94, 112, 0, -18, -94, 112, 0, -18, -94, 112, 0);
  const __m128i round = _mm_set1_epi16(128);
  const __m128i even_low_bytes = _mm_setr_epi8(0, 4, 8, 12, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1);
  int x = 0;
  for (; x + 8 <= width; x += 8) {
    __m128i first = _mm_loadu_si128((const __m128i *)(pixels + 4 * x));
    __m128i second = _mm_loadu_si128((const __m128i *)(pixels + 4 * x + 16));
    __m128i us = _mm_hadd_epi16(_mm_maddubs_epi16(first, u_coeff), _mm_maddubs_epi16(second, u_coeff));
    __m128i vs = _mm_hadd_epi16(_mm_maddubs_epi16(first, v_coeff), _mm_maddubs_epi16(second, v_coeff));
    uint32_t packed_u = (uint32_t)_mm_cvtsi128_si32(_mm_shuffle_epi8(_mm_add_epi16(_mm_srai_epi16(_mm_add_epi16(us, round), 8), round), even_low_bytes));
    uint32_t packed_v = (uint32_t)_mm_cvtsi128_si32(_mm_shuffle_epi8(_mm_add_epi16(_mm_srai_epi16(_mm_add_epi16(vs, round), 8), round), even_low_bytes));
    memcpy(u + x / 2, &packed_u, sizeof(packed_u));
    memcpy(v + x / 2, &packed_v, sizeof(packed_v));
  }
  return x;
}
#endif

void wox_recording_bgra_to_i420(const uint8_t *restrict source, int stride, int width, int height, uint8_t *restrict destination) {
  size_t luma_size = (size_t)width * height;
  uint8_t *u_plane = destination + luma_size;
  uint8_t *v_plane = u_plane + luma_size / 4;
#if defined(__x86_64__)
  const int use_ssse3 = __builtin_cpu_supports("ssse3");
#endif
  for (int row = 0; row < height; ++row) {
    const uint8_t *pixels = source + (size_t)row * stride;
    uint8_t *y = destination + (size_t)row * width;
    int x = 0;
#if defined(__x86_64__)
    if (use_ssse3) {
      x = write_luma_ssse3(pixels, y, width);
    }
#endif
    for (; x < width; ++x) {
      int b = pixels[4 * x], g = pixels[4 * x + 1], r = pixels[4 * x + 2];
      y[x] = (uint8_t)(((66 * r + 129 * g + 25 * b + 128) >> 8) + 16);
    }
    if (row % 2 == 0) {
      x = 0;
#if defined(__x86_64__)
      if (use_ssse3) {
        x = write_chroma_ssse3(pixels, u_plane, v_plane, width);
      }
#endif
      for (; x < width; x += 2) {
        int b = pixels[4 * x], g = pixels[4 * x + 1], r = pixels[4 * x + 2];
        u_plane[x / 2] = (uint8_t)(((-38 * r - 74 * g + 112 * b + 128) >> 8) + 128);
        v_plane[x / 2] = (uint8_t)(((112 * r - 94 * g - 18 * b + 128) >> 8) + 128);
      }
      u_plane += width / 2;
      v_plane += width / 2;
    }
  }
}
