// Copyright 2010 Google Inc. All Rights Reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the COPYING file in the root of the source
// tree. An additional intellectual property rights grant can be found
// in the file PATENTS. All contributing project authors may
// be found in the AUTHORS file in the root of the source tree.
// -----------------------------------------------------------------------------
//
//  Main decoding functions for WebP images.
//
// Author: Skal (pascal.massimino@gmail.com)

#ifndef WEBP_WEBP_DECODE_H_
#define WEBP_WEBP_DECODE_H_

#include "./types.h"

#ifdef __cplusplus
extern "C" {
#endif

#define WEBP_DECODER_ABI_VERSION 0x0205    // MAJOR(8b) + MINOR(8b)

// Note: forward declaring enumerations is not allowed in (strict) C and C++,
// the types are left here for reference.
// typedef enum VP8StatusCode VP8StatusCode;
// typedef enum WEBP_CSP_MODE WEBP_CSP_MODE;
typedef struct WebPRGBABuffer WebPRGBABuffer;
typedef struct WebPYUVABuffer WebPYUVABuffer;
typedef struct WebPDecBuffer WebPDecBuffer;
typedef struct WebPIDecoder WebPIDecoder;
typedef struct WebPBitstreamFeatures WebPBitstreamFeatures;
typedef struct WebPDecoderOptions WebPDecoderOptions;
typedef struct WebPDecoderConfig WebPDecoderConfig;

// Return the decoder's version number, packed in hexadecimal using 8bits for
// each of major/minor/revision. E.g: v2.5.7 is 0x020507.
WEBP_EXTERN(int) WebPGetDecoderVersion(void);

// Retrieve basic header information: width, height.
// This function will also validate the header and return 0 in
// case of formatting error.
// Pointers 'width' and 'height' can be passed NULL if deemed irrelevant.
WEBP_EXTERN(int) WebPGetInfo(const uint8_t* data, size_t data_size,
                             int* width, int* height);

// Decodes WebP images pointed to by 'data' and returns RGBA samples, along
// with the dimensions in *width and *height. The ordering of samples in
// memory is R, G, B, A, R, G, B, A... in scan order (endian-independent).
// The returned pointer should be deleted calling free().
// Returns NULL in case of error.
WEBP_EXTERN(uint8_t*) WebPDecodeRGBA(const uint8_t* data, size_t data_size,
                                     int* width, int* height);

// Same as WebPDecodeRGBA, but returning B, G, R, A, B, G, R, A... ordered data.
WEBP_EXTERN(uint8_t*) WebPDecodeBGRA(const uint8_t* data, size_t data_size,
                                     int* width, int* height);


//------------------------------------------------------------------------------
// Output colorspaces and buffer

// Colorspaces
// Note: the naming describes the byte-ordering of packed samples in memory.
// For instance, MODE_BGRA relates to samples ordered as B,G,R,A,B,G,R,A,...
// Non-capital names (e.g.:MODE_Argb) relates to pre-multiplied RGB channels.
// RGBA-4444 and RGB-565 colorspaces are represented by following byte-order:
// RGBA-4444: [r3 r2 r1 r0 g3 g2 g1 g0], [b3 b2 b1 b0 a3 a2 a1 a0], ...
// RGB-565: [r4 r3 r2 r1 r0 g5 g4 g3], [g2 g1 g0 b4 b3 b2 b1 b0], ...
// In the case WEBP_SWAP_16BITS_CSP is defined, the bytes are swapped for
// these two modes:
// RGBA-4444: [b3 b2 b1 b0 a3 a2 a1 a0], [r3 r2 r1 r0 g3 g2 g1 g0], ...
// RGB-565: [g2 g1 g0 b4 b3 b2 b1 b0], [r4 r3 r2 r1 r0 g5 g4 g3], ...

typedef enum WEBP_CSP_MODE {
  MODE_RGB = 0, MODE_RGBA = 1,
  MODE_BGR = 2, MODE_BGRA = 3,
  MODE_ARGB = 4, MODE_RGBA_4444 = 5,
  MODE_RGB_565 = 6,
  // RGB-premultiplied transparent modes (alpha value is preserved)
  MODE_rgbA = 7,
  MODE_bgrA = 8,
  MODE_Argb = 9,
  MODE_rgbA_4444 = 10,
  // YUV modes must come after RGB ones.
  MODE_YUV = 11, MODE_YUVA = 12,  // yuv 4:2:0
  MODE_LAST = 13
} WEBP_CSP_MODE;

// Some useful macros:
static WEBP_INLINE int WebPIsPremultipliedMode(WEBP_CSP_MODE mode) {
  return (mode == MODE_rgbA || mode == MODE_bgrA || mode == MODE_Argb ||
          mode == MODE_rgbA_4444);
}

static WEBP_INLINE int WebPIsAlphaMode(WEBP_CSP_MODE mode) {
  return (mode == MODE_RGBA || mode == MODE_BGRA || mode == MODE_ARGB ||
          mode == MODE_RGBA_4444 || mode == MODE_YUVA ||
          WebPIsPremultipliedMode(mode));
}

static WEBP_INLINE int WebPIsRGBMode(WEBP_CSP_MODE mode) {
  return (mode < MODE_YUV);
}

//------------------------------------------------------------------------------
// WebPDecBuffer: Generic structure for describing the output sample buffer.

struct WebPRGBABuffer {    // view as RGBA
  uint8_t* rgba;    // pointer to RGBA samples
  int stride;       // stride in bytes from one scanline to the next.
  size_t size;      // total size of the *rgba buffer.
};

struct WebPYUVABuffer {              // view as YUVA
  uint8_t* y, *u, *v, *a;     // pointer to luma, chroma U/V, alpha samples
  int y_stride;               // luma stride
  int u_stride, v_stride;     // chroma strides
  int a_stride;               // alpha stride
  size_t y_size;              // luma plane size
  size_t u_size, v_size;      // chroma planes size
  size_t a_size;              // alpha-plane size
};

// Output buffer
struct WebPDecBuffer {
  WEBP_CSP_MODE colorspace;  // Colorspace.
  int width, height;         // Dimensions.
  int is_external_memory;    // If true, 'internal_memory' pointer is not used.
  union {
    WebPRGBABuffer RGBA;
    WebPYUVABuffer YUVA;
  } u;                       // Nameless union of buffer parameters.
  uint32_t       pad[4];     // padding for later use

  uint8_t* private_memory;   // Internally allocated memory (only when
                             // is_external_memory is false). Should not be used
                             // externally, but accessed via the buffer union.
};

// Internal, version-checked, entry point
WEBP_EXTERN(int) WebPInitDecBufferInternal(WebPDecBuffer*, int);

// Initialize the structure as empty. Must be called before any other use.
// Returns false in case of version mismatch
static WEBP_INLINE int WebPInitDecBuffer(WebPDecBuffer* buffer) {
  return WebPInitDecBufferInternal(buffer, WEBP_DECODER_ABI_VERSION);
}

// Free any memory associated with the buffer. Must always be called last.
// Note: doesn't free the 'buffer' structure itself.
WEBP_EXTERN(void) WebPFreeDecBuffer(WebPDecBuffer* buffer);

//------------------------------------------------------------------------------
// Enumeration of the status codes

typedef enum VP8StatusCode {
  VP8_STATUS_OK = 0,
  VP8_STATUS_OUT_OF_MEMORY,
  VP8_STATUS_INVALID_PARAM,
  VP8_STATUS_BITSTREAM_ERROR,
  VP8_STATUS_UNSUPPORTED_FEATURE,
  VP8_STATUS_SUSPENDED,
  VP8_STATUS_USER_ABORT,
  VP8_STATUS_NOT_ENOUGH_DATA
} VP8StatusCode;

//------------------------------------------------------------------------------
// Advanced decoding parametrization
//
//  Code sample for using the advanced decoding API
/*
     // A) Init a configuration object
     WebPDecoderConfig config;
     CHECK(WebPInitDecoderConfig(&config));

     // B) optional: retrieve the bitstream's features.
     CHECK(WebPGetFeatures(data, data_size, &config.input) == VP8_STATUS_OK);

     // C) Adjust 'config', if needed
     config.no_fancy_upsampling = 1;
     config.output.colorspace = MODE_BGRA;
     // etc.

     // Note that you can also make config.output point to an externally
     // supplied memory buffer, provided it's big enough to store the decoded
     // picture. Otherwise, config.output will just be used to allocate memory
     // and store the decoded picture.

     // D) Decode!
     CHECK(WebPDecode(data, data_size, &config) == VP8_STATUS_OK);

     // E) Decoded image is now in config.output (and config.output.u.RGBA)

     // F) Reclaim memory allocated in config's object. It's safe to call
     // this function even if the memory is external and wasn't allocated
     // by WebPDecode().
     WebPFreeDecBuffer(&config.output);
*/

// Features gathered from the bitstream
struct WebPBitstreamFeatures {
  int width;          // Width in pixels, as read from the bitstream.
  int height;         // Height in pixels, as read from the bitstream.
  int has_alpha;      // True if the bitstream contains an alpha channel.
  int has_animation;  // True if the bitstream is an animation.
  int format;         // 0 = undefined (/mixed), 1 = lossy, 2 = lossless

  // Unused for now:
  int no_incremental_decoding;  // if true, using incremental decoding is not
                                // recommended.
  int rotate;                   // TODO(later)
  int uv_sampling;              // should be 0 for now. TODO(later)
  uint32_t pad[2];              // padding for later use
};

// Retrieve features from the bitstream. The *features structure is filled
// with information gathered from the bitstream.
// Returns VP8_STATUS_OK when the features are successfully retrieved. Returns
// VP8_STATUS_NOT_ENOUGH_DATA when more data is needed to retrieve the
// features from headers. Returns error in other cases.
WEBP_EXTERN(VP8StatusCode) WebPGetFeatures(const uint8_t*, size_t, WebPBitstreamFeatures*);

// Decoding options
struct WebPDecoderOptions {
  int bypass_filtering;               // if true, skip the in-loop filtering
  int no_fancy_upsampling;            // if true, use faster pointwise upsampler
  int use_cropping;                   // if true, cropping is applied _first_
  int crop_left, crop_top;            // top-left position for cropping.
                                      // Will be snapped to even values.
  int crop_width, crop_height;        // dimension of the cropping area
  int use_scaling;                    // if true, scaling is applied _afterward_
  int scaled_width, scaled_height;    // final resolution
  int use_threads;                    // if true, use multi-threaded decoding
  int dithering_strength;             // dithering strength (0=Off, 100=full)
  int flip;                           // flip output vertically
  int alpha_dithering_strength;       // alpha dithering strength in [0..100]

  // Unused for now:
  int force_rotation;                 // forced rotation (to be applied _last_)
  int no_enhancement;                 // if true, discard enhancement layer
  uint32_t pad[3];                    // padding for later use
};

#ifdef __cplusplus
}    // extern "C"
#endif

#endif  /* WEBP_WEBP_DECODE_H_ */
