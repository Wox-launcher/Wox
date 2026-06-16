/*
 * Copyright 2016 Google Inc.
 *
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

/*
 * see skia/src/opts/SkSwizzler_opts.h
 */

#pragma once

#include "cpuid/cpuinfo.h"

/**
 *  SK_CPU_SSE_LEVEL
 *
 *  If defined, SK_CPU_SSE_LEVEL should be set to the highest supported level.
 *  On non-intel CPU this should be undefined.
 */
#define SK_CPU_SSE_LEVEL_SSE1 10
#define SK_CPU_SSE_LEVEL_SSE2 20
#define SK_CPU_SSE_LEVEL_SSE3 30
#define SK_CPU_SSE_LEVEL_SSSE3 31
#define SK_CPU_SSE_LEVEL_SSE41 41
#define SK_CPU_SSE_LEVEL_SSE42 42
#define SK_CPU_SSE_LEVEL_AVX 51
#define SK_CPU_SSE_LEVEL_AVX2 52
#define SK_CPU_SSE_LEVEL_SKX 60

// Are we in GCC/Clang?
#ifndef SK_CPU_SSE_LEVEL
// These checks must be done in descending order to ensure we set the highest
// available SSE level.
#if defined(__AVX512F__) && defined(__AVX512DQ__) && defined(__AVX512CD__) && \
    defined(__AVX512BW__) && defined(__AVX512VL__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SKX
#elif defined(__AVX2__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_AVX2
#elif defined(__AVX__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_AVX
#elif defined(__SSE4_2__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SSE42
#elif defined(__SSE4_1__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SSE41
#elif defined(__SSSE3__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SSSE3
#elif defined(__SSE3__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SSE3
#elif defined(__SSE2__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SSE2
#endif
#endif

// Are we in VisualStudio?
#ifndef SK_CPU_SSE_LEVEL
// These checks must be done in descending order to ensure we set the highest
// available SSE level. 64-bit intel guarantees at least SSE2 support.
#if defined(__AVX512F__) && defined(__AVX512DQ__) && defined(__AVX512CD__) && \
    defined(__AVX512BW__) && defined(__AVX512VL__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SKX
#elif defined(__AVX2__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_AVX2
#elif defined(__AVX__)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_AVX
#elif defined(_M_X64) || defined(_M_AMD64)
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SSE2
#elif defined(_M_IX86_FP)
#if _M_IX86_FP >= 2
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SSE2
#elif _M_IX86_FP == 1
#define SK_CPU_SSE_LEVEL SK_CPU_SSE_LEVEL_SSE1
#endif
#endif
#endif

inline void RGBA_to_BGRA_portable(uint32_t* dst, const uint32_t* src,
                                  int height, int src_stride, int dst_stride) {
  auto width = std::min<int>(src_stride, dst_stride);

  for (int y = 0; y < height; y++) {
    for (int x = 0; x < width; x++) {
      uint8_t a = (src[x] >> 24) & 0xFF, b = (src[x] >> 16) & 0xFF,
              g = (src[x] >> 8) & 0xFF, r = (src[x] >> 0) & 0xFF;
      dst[x] = (uint32_t)a << 24 | (uint32_t)r << 16 | (uint32_t)g << 8 |
               (uint32_t)b << 0;
    }

    src += src_stride;
    dst += dst_stride;
  }
}

#if SK_CPU_SSE_LEVEL >= SK_CPU_SSE_LEVEL_SKX

inline void RGBA_to_BGRA_SKX(uint32_t* dst, const uint32_t* src, int height,
                             int src_stride, int dst_stride) {
  const uint8_t mask[64] = {2,  1,  0,  3,  6,  5,  4,  7,  10, 9,  8,  11, 14,
                            13, 12, 15, 2,  1,  0,  3,  6,  5,  4,  7,  10, 9,
                            8,  11, 14, 13, 12, 15, 2,  1,  0,  3,  6,  5,  4,
                            7,  10, 9,  8,  11, 14, 13, 12, 15, 2,  1,  0,  3,
                            6,  5,  4,  7,  10, 9,  8,  11, 14, 13, 12, 15};
  const __m512i swapRB = _mm512_loadu_si512(mask);

  auto width = std::min<int>(src_stride, dst_stride);

  for (int y = 0; y < height; y++) {
    auto cw = width;
    auto rptr = src;
    auto dptr = dst;
    while (cw >= 16) {
      __m512i rgba = _mm512_loadu_si512((const __m512i*)rptr);
      __m512i bgra = _mm512_shuffle_epi8(rgba, swapRB);
      _mm512_storeu_si512((__m512i*)dptr, bgra);

      rptr += 16;
      dptr += 16;
      cw -= 16;
    }

    for (auto x = 0; x < cw; x++) {
      uint8_t a = (rptr[x] >> 24) & 0xFF, b = (rptr[x] >> 16) & 0xFF,
              g = (rptr[x] >> 8) & 0xFF, r = (rptr[x] >> 0) & 0xFF;
      dptr[x] = (uint32_t)a << 24 | (uint32_t)r << 16 | (uint32_t)g << 8 |
                (uint32_t)b << 0;
    }

    src += src_stride;
    dst += dst_stride;
  }
}

#endif

#if SK_CPU_SSE_LEVEL >= SK_CPU_SSE_LEVEL_AVX2

inline void RGBA_to_BGRA_AVX2(uint32_t* dst, const uint32_t* src, int height,
                              int src_stride, int dst_stride) {
  const __m256i swapRB =
      _mm256_setr_epi8(2, 1, 0, 3, 6, 5, 4, 7, 10, 9, 8, 11, 14, 13, 12, 15, 2,
                       1, 0, 3, 6, 5, 4, 7, 10, 9, 8, 11, 14, 13, 12, 15);

  auto width = std::min<int>(src_stride, dst_stride);

  for (int y = 0; y < height; y++) {
    auto cw = width;
    auto rptr = src;
    auto dptr = dst;
    while (cw >= 8) {
      __m256i rgba = _mm256_loadu_si256((const __m256i*)rptr);
      __m256i bgra = _mm256_shuffle_epi8(rgba, swapRB);
      _mm256_storeu_si256((__m256i*)dptr, bgra);

      rptr += 8;
      dptr += 8;
      cw -= 8;
    }

    for (auto x = 0; x < cw; x++) {
      uint8_t a = (rptr[x] >> 24) & 0xFF, b = (rptr[x] >> 16) & 0xFF,
              g = (rptr[x] >> 8) & 0xFF, r = (rptr[x] >> 0) & 0xFF;
      dptr[x] = (uint32_t)a << 24 | (uint32_t)r << 16 | (uint32_t)g << 8 |
                (uint32_t)b << 0;
    }

    src += src_stride;
    dst += dst_stride;
  }
}

#endif

inline void RGBA_to_BGRA(uint32_t* dst, const uint32_t* src, int height,
                         int src_stride, int dst_stride) {
  static cpuid::cpuinfo info;

#if SK_CPU_SSE_LEVEL >= SK_CPU_SSE_LEVEL_SKX
  if (info.has_avx512_f() && info.has_avx512_dq() && info.has_avx512_cd() &&
      info.has_avx512_bw() && info.has_avx512_vl()) {
    return RGBA_to_BGRA_SKX(dst, src, height, src_stride, dst_stride);
  }
#endif

#if SK_CPU_SSE_LEVEL >= SK_CPU_SSE_LEVEL_AVX2
  if (info.has_avx2()) {
    return RGBA_to_BGRA_AVX2(dst, src, height, src_stride, dst_stride);
  }
#endif

  RGBA_to_BGRA_portable(dst, src, height, src_stride, dst_stride);
}
