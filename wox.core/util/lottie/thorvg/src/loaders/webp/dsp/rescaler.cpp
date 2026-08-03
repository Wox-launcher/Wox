// Copyright 2014 Google Inc. All Rights Reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the COPYING file in the root of the source
// tree. An additional intellectual property rights grant can be found
// in the file PATENTS. All contributing project authors may
// be found in the AUTHORS file in the root of the source tree.
// -----------------------------------------------------------------------------
//
// Rescaling functions

#include "./dsp.h"
#include "../utils/rescaler.h"

//------------------------------------------------------------------------------
// Implementations of critical functions ImportRow / ExportRow

#define ROUNDER (1 << (WEBP_RESCALER_RFIX - 1))
#define MULT_FIX(x, y) (((int64_t)(x) * (y) + ROUNDER) >> WEBP_RESCALER_RFIX)

static void RescalerImportRowC(WebPRescaler* const wrk,
                               const uint8_t* const src, int channel) {
  const int x_stride = wrk->num_channels;
  const int x_out_max = wrk->dst_width * wrk->num_channels;
  int x_in = channel;
  int x_out;
  int accum = 0;
  if (!wrk->x_expand) {
    int sum = 0;
    for (x_out = channel; x_out < x_out_max; x_out += x_stride) {
      accum += wrk->x_add;
      for (; accum > 0; accum -= wrk->x_sub) {
        sum += src[x_in];
        x_in += x_stride;
      }
      {        // Emit next horizontal pixel.
        const int32_t base = src[x_in];
        const int32_t frac = base * (-accum);
        x_in += x_stride;
        wrk->frow[x_out] = (sum + base) * wrk->x_sub - frac;
        // fresh fractional start for next pixel
        sum = (int)MULT_FIX(frac, wrk->fx_scale);
      }
    }
  } else {        // simple bilinear interpolation
    int left = src[channel], right = src[channel];
    for (x_out = channel; x_out < x_out_max; x_out += x_stride) {
      if (accum < 0) {
        left = right;
        x_in += x_stride;
        right = src[x_in];
        accum += wrk->x_add;
      }
      wrk->frow[x_out] = right * wrk->x_add + (left - right) * accum;
      accum -= wrk->x_sub;
    }
  }
  // Accumulate the contribution of the new row.
  for (x_out = channel; x_out < x_out_max; x_out += x_stride) {
    wrk->irow[x_out] += wrk->frow[x_out];
  }
}

void WebPRescalerExportRowC(WebPRescaler* const wrk, int x_out) {
  if (wrk->y_accum <= 0) {
    uint8_t* const dst = wrk->dst;
    int32_t* const irow = wrk->irow;
    const int32_t* const frow = wrk->frow;
    const int yscale = wrk->fy_scale * (-wrk->y_accum);
    const int x_out_max = wrk->dst_width * wrk->num_channels;
    for (; x_out < x_out_max; ++x_out) {
      const int frac = (int)MULT_FIX(frow[x_out], yscale);
      const int v = (int)MULT_FIX(irow[x_out] - frac, wrk->fxy_scale);
      dst[x_out] = (!(v & ~0xff)) ? v : (v < 0) ? 0 : 255;
      irow[x_out] = frac;   // new fractional start
    }
    wrk->y_accum += wrk->y_add;
    wrk->dst += wrk->dst_stride;
  }
}

#undef MULT_FIX
#undef ROUNDER

//------------------------------------------------------------------------------

void (*WebPRescalerImportRow)(struct WebPRescaler* const wrk,
                              const uint8_t* const src, int channel);
void (*WebPRescalerExportRow)(struct WebPRescaler* const wrk, int x_out);

extern void WebPRescalerDspInitMIPS32(void);
extern void WebPRescalerDspInitMIPSdspR2(void);

static volatile VP8CPUInfo rescaler_last_cpuinfo_used =
    (VP8CPUInfo)&rescaler_last_cpuinfo_used;

WEBP_TSAN_IGNORE_FUNCTION void WebPRescalerDspInit(void) {
  if (rescaler_last_cpuinfo_used == VP8GetCPUInfo) return;

  WebPRescalerImportRow = RescalerImportRowC;
  WebPRescalerExportRow = WebPRescalerExportRowC;
  if (VP8GetCPUInfo != NULL) {
#if defined(WEBP_USE_MIPS32)
    if (VP8GetCPUInfo(kMIPS32)) {
      WebPRescalerDspInitMIPS32();
    }
#endif
#if defined(WEBP_USE_MIPS_DSP_R2)
    if (VP8GetCPUInfo(kMIPSdspR2)) {
      WebPRescalerDspInitMIPSdspR2();
    }
#endif
  }
  rescaler_last_cpuinfo_used = VP8GetCPUInfo;
}
