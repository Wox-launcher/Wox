// Copyright 2011 Google Inc. All Rights Reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the COPYING file in the root of the source
// tree. An additional intellectual property rights grant can be found
// in the file PATENTS. All contributing project authors may
// be found in the AUTHORS file in the root of the source tree.
// -----------------------------------------------------------------------------
//
//   Speed-critical functions.
//
// Author: Skal (pascal.massimino@gmail.com)

#ifndef WEBP_DSP_DSP_H_
#define WEBP_DSP_DSP_H_

#ifdef HAVE_CONFIG_H
#include "../webp/config.h"
#endif

#include "../webp/types.h"

#ifdef __cplusplus
extern "C" {
#endif

#define BPS 32   // this is the common stride for enc/dec

//------------------------------------------------------------------------------
// CPU detection

#if defined(__GNUC__)
# define LOCAL_GCC_VERSION ((__GNUC__ << 8) | __GNUC_MINOR__)
# define LOCAL_GCC_PREREQ(maj, min) \
    (LOCAL_GCC_VERSION >= (((maj) << 8) | (min)))
#else
# define LOCAL_GCC_VERSION 0
# define LOCAL_GCC_PREREQ(maj, min) 0
#endif

#ifdef __clang__
# define LOCAL_CLANG_VERSION ((__clang_major__ << 8) | __clang_minor__)
# define LOCAL_CLANG_PREREQ(maj, min) \
    (LOCAL_CLANG_VERSION >= (((maj) << 8) | (min)))
#else
# define LOCAL_CLANG_VERSION 0
# define LOCAL_CLANG_PREREQ(maj, min) 0
#endif  // __clang__

#if defined(_MSC_VER) && _MSC_VER > 1310 && \
    (defined(_M_X64) || defined(_M_IX86))
#define WEBP_MSC_SSE2  // Visual C++ SSE2 targets
#endif

// WEBP_HAVE_* are used to indicate the presence of the instruction set in dsp
// files without intrinsics, allowing the corresponding Init() to be called.
// Files containing intrinsics will need to be built targeting the instruction
// set so should succeed on one of the earlier tests.
#if defined(__SSE2__) || defined(WEBP_MSC_SSE2) || defined(WEBP_HAVE_SSE2)
//#define WEBP_USE_SSE2
#endif

// This macro prevents thread_sanitizer from reporting known concurrent writes.
#define WEBP_TSAN_IGNORE_FUNCTION
#if defined(__has_feature)
#if __has_feature(thread_sanitizer)
#undef WEBP_TSAN_IGNORE_FUNCTION
#define WEBP_TSAN_IGNORE_FUNCTION __attribute__((no_sanitize_thread))
#endif
#endif

typedef enum {
  kSSE2,
} CPUFeature;
// returns true if the CPU supports the feature.
typedef int (*VP8CPUInfo)(CPUFeature feature);
WEBP_EXTERN(VP8CPUInfo) VP8GetCPUInfo;

//------------------------------------------------------------------------------
// Init stub generator

// Defines an init function stub to ensure each module exposes a symbol,
// avoiding a compiler warning.
#define WEBP_DSP_INIT_STUB(func) \
  extern void func(void); \
  WEBP_TSAN_IGNORE_FUNCTION void func(void) {}

//------------------------------------------------------------------------------
// Decoding

typedef void (*VP8DecIdct)(const int16_t* coeffs, uint8_t* dst);
// when doing two transforms, coeffs is actually int16_t[2][16].
typedef void (*VP8DecIdct2)(const int16_t* coeffs, uint8_t* dst, int do_two);
extern VP8DecIdct2 VP8Transform;
extern VP8DecIdct VP8TransformAC3;
extern VP8DecIdct VP8TransformUV;
extern VP8DecIdct VP8TransformDC;
extern VP8DecIdct VP8TransformDCUV;

// *dst is the destination block, with stride BPS. Boundary samples are
// assumed accessible when needed.
typedef void (*VP8PredFunc)(uint8_t* dst);
extern VP8PredFunc VP8PredLuma16[/* NUM_B_DC_MODES */];
extern VP8PredFunc VP8PredChroma8[/* NUM_B_DC_MODES */];
extern VP8PredFunc VP8PredLuma4[/* NUM_BMODES */];

// clipping tables (for filtering)
extern const int8_t* const VP8ksclip1;  // clips [-1020, 1020] to [-128, 127]
extern const int8_t* const VP8ksclip2;  // clips [-112, 112] to [-16, 15]
extern const uint8_t* const VP8kclip1;  // clips [-255,511] to [0,255]
extern const uint8_t* const VP8kabs0;   // abs(x) for x in [-255,255]
// must be called first
void VP8InitClipTables(void);

// simple filter (only for luma)
typedef void (*VP8SimpleFilterFunc)(uint8_t* p, int stride, int thresh);
extern VP8SimpleFilterFunc VP8SimpleVFilter16;
extern VP8SimpleFilterFunc VP8SimpleHFilter16;
extern VP8SimpleFilterFunc VP8SimpleVFilter16i;  // filter 3 inner edges
extern VP8SimpleFilterFunc VP8SimpleHFilter16i;

// regular filter (on both macroblock edges and inner edges)
typedef void (*VP8LumaFilterFunc)(uint8_t* luma, int stride,
                                  int thresh, int ithresh, int hev_t);
typedef void (*VP8ChromaFilterFunc)(uint8_t* u, uint8_t* v, int stride,
                                    int thresh, int ithresh, int hev_t);
// on outer edge
extern VP8LumaFilterFunc VP8VFilter16;
extern VP8LumaFilterFunc VP8HFilter16;
extern VP8ChromaFilterFunc VP8VFilter8;
extern VP8ChromaFilterFunc VP8HFilter8;

// on inner edge
extern VP8LumaFilterFunc VP8VFilter16i;   // filtering 3 inner edges altogether
extern VP8LumaFilterFunc VP8HFilter16i;
extern VP8ChromaFilterFunc VP8VFilter8i;  // filtering u and v altogether
extern VP8ChromaFilterFunc VP8HFilter8i;

// must be called before anything using the above
void VP8DspInit(void);

//------------------------------------------------------------------------------
// WebP I/O

//#define FANCY_UPSAMPLING   // undefined to remove fancy upsampling support

// Convert a pair of y/u/v lines together to the output rgb/a colorspace.
// bottom_y can be NULL if only one line of output is needed (at top/bottom).
typedef void (*WebPUpsampleLinePairFunc)(
    const uint8_t* top_y, const uint8_t* bottom_y,
    const uint8_t* top_u, const uint8_t* top_v,
    const uint8_t* cur_u, const uint8_t* cur_v,
    uint8_t* top_dst, uint8_t* bottom_dst, int len);

#ifdef FANCY_UPSAMPLING

// Fancy upsampling functions to convert YUV to RGB(A) modes
extern WebPUpsampleLinePairFunc WebPUpsamplers[/* MODE_LAST */];

#endif    // FANCY_UPSAMPLING

// Per-row point-sampling methods.
typedef void (*WebPSamplerRowFunc)(const uint8_t* y,
                                   const uint8_t* u, const uint8_t* v,
                                   uint8_t* dst, int len);
// Generic function to apply 'WebPSamplerRowFunc' to the whole plane:
void WebPSamplerProcessPlane(const uint8_t* y, int y_stride,
                             const uint8_t* u, const uint8_t* v, int uv_stride,
                             uint8_t* dst, int dst_stride,
                             int width, int height, WebPSamplerRowFunc func);

// Sampling functions to convert rows of YUV to RGB(A)
extern WebPSamplerRowFunc WebPSamplers[/* MODE_LAST */];

// General function for converting two lines of ARGB or RGBA.
// 'alpha_is_last' should be true if 0xff000000 is stored in memory as
// as 0x00, 0x00, 0x00, 0xff (little endian).
WebPUpsampleLinePairFunc WebPGetLinePairConverter(int alpha_is_last);

// YUV444->RGB converters
typedef void (*WebPYUV444Converter)(const uint8_t* y,
                                    const uint8_t* u, const uint8_t* v,
                                    uint8_t* dst, int len);

extern WebPYUV444Converter WebPYUV444Converters[/* MODE_LAST */];

// Must be called before using the WebPUpsamplers[] (and for premultiplied
// colorspaces like rgbA, rgbA4444, etc)
void WebPInitUpsamplers(void);
// Must be called before using WebPSamplers[]
void WebPInitSamplers(void);
// Must be called before using WebPYUV444Converters[]
void WebPInitYUV444Converters(void);

//------------------------------------------------------------------------------
// Rescaler

struct WebPRescaler;

// Import a row of data and save its contribution in the rescaler.
// 'channel' denotes the channel number to be imported.
extern void (*WebPRescalerImportRow)(struct WebPRescaler* const wrk,
                                     const uint8_t* const src, int channel);

// Export one row (starting at x_out position) from rescaler.
extern void (*WebPRescalerExportRow)(struct WebPRescaler* const wrk, int x_out);

// Plain-C implementation, as fall-back.
extern void WebPRescalerExportRowC(struct WebPRescaler* const wrk, int x_out);

// Must be called first before using the above.
void WebPRescalerDspInit(void);

//------------------------------------------------------------------------------
// Utilities for processing transparent channel.

// Apply alpha pre-multiply on an rgba, bgra or argb plane of size w * h.
// alpha_first should be 0 for argb, 1 for rgba or bgra (where alpha is last).
extern void (*WebPApplyAlphaMultiply)(
    uint8_t* rgba, int alpha_first, int w, int h, int stride);

// Same, buf specifically for RGBA4444 format
extern void (*WebPApplyAlphaMultiply4444)(
    uint8_t* rgba4444, int w, int h, int stride);

// Dispatch the values from alpha[] plane to the ARGB destination 'dst'.
// Returns true if alpha[] plane has non-trivial values different from 0xff.
extern int (*WebPDispatchAlpha)(const uint8_t* alpha, int alpha_stride,
                                int width, int height,
                                uint8_t* dst, int dst_stride);

// Transfer packed 8b alpha[] values to green channel in dst[], zero'ing the
// A/R/B values. 'dst_stride' is the stride for dst[] in uint32_t units.
extern void (*WebPDispatchAlphaToGreen)(const uint8_t* alpha, int alpha_stride,
                                        int width, int height,
                                        uint32_t* dst, int dst_stride);

// Extract the alpha values from 32b values in argb[] and pack them into alpha[]
// (this is the opposite of WebPDispatchAlpha).
// Returns true if there's only trivial 0xff alpha values.
extern int (*WebPExtractAlpha)(const uint8_t* argb, int argb_stride,
                               int width, int height,
                               uint8_t* alpha, int alpha_stride);

// Pre-Multiply operation transforms x into x * A / 255  (where x=Y,R,G or B).
// Un-Multiply operation transforms x into x * 255 / A.

// Pre-Multiply or Un-Multiply (if 'inverse' is true) argb values in a row.
extern void (*WebPMultARGBRow)(uint32_t* const ptr, int width, int inverse);

// Same a WebPMultARGBRow(), but for several rows.
void WebPMultARGBRows(uint8_t* ptr, int stride, int width, int num_rows,
                      int inverse);

// Same for a row of single values, with side alpha values.
extern void (*WebPMultRow)(uint8_t* const ptr, const uint8_t* const alpha,
                           int width, int inverse);

// Same a WebPMultRow(), but for several 'num_rows' rows.
void WebPMultRows(uint8_t* ptr, int stride,
                  const uint8_t* alpha, int alpha_stride,
                  int width, int num_rows, int inverse);

// Plain-C versions, used as fallback by some implementations.
void WebPMultRowC(uint8_t* const ptr, const uint8_t* const alpha,
                  int width, int inverse);
void WebPMultARGBRowC(uint32_t* const ptr, int width, int inverse);

// To be called first before using the above.
void WebPInitAlphaProcessing(void);

// ARGB packing function: a/r/g/b input is rgba or bgra order.
extern void (*VP8PackARGB)(const uint8_t* a, const uint8_t* r,
                           const uint8_t* g, const uint8_t* b, int len,
                           uint32_t* out);

// RGB packing function. 'step' can be 3 or 4. r/g/b input is rgb or bgr order.
extern void (*VP8PackRGB)(const uint8_t* r, const uint8_t* g, const uint8_t* b,
                          int len, int step, uint32_t* out);


//------------------------------------------------------------------------------
// Filter functions

typedef enum {     // Filter types.
  WEBP_FILTER_NONE = 0,
  WEBP_FILTER_HORIZONTAL,
  WEBP_FILTER_VERTICAL,
  WEBP_FILTER_GRADIENT,
  WEBP_FILTER_LAST = WEBP_FILTER_GRADIENT + 1,  // end marker
  WEBP_FILTER_BEST,    // meta-types
  WEBP_FILTER_FAST
} WEBP_FILTER_TYPE;

typedef void (*WebPFilterFunc)(const uint8_t* in, int width, int height,
                               int stride, uint8_t* out);
typedef void (*WebPUnfilterFunc)(int width, int height, int stride,
                                 int row, int num_rows, uint8_t* data);

// Filter the given data using the given predictor.
// 'in' corresponds to a 2-dimensional pixel array of size (stride * height)
// in raster order.
// 'stride' is number of bytes per scan line (with possible padding).
// 'out' should be pre-allocated.
extern WebPFilterFunc WebPFilters[WEBP_FILTER_LAST];

// In-place reconstruct the original data from the given filtered data.
// The reconstruction will be done for 'num_rows' rows starting from 'row'
// (assuming rows upto 'row - 1' are already reconstructed).
extern WebPUnfilterFunc WebPUnfilters[WEBP_FILTER_LAST];

// To be called first before using the above.
void VP8FiltersInit(void);

#ifdef __cplusplus
}    // extern "C"
#endif

#endif  /* WEBP_DSP_DSP_H_ */
