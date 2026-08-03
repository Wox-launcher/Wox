// Copyright 2012 Google Inc. All Rights Reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the COPYING file in the root of the source
// tree. An additional intellectual property rights grant can be found
// in the file PATENTS. All contributing project authors may
// be found in the AUTHORS file in the root of the source tree.
// -----------------------------------------------------------------------------
//
// Rescaling functions
//
// Author: Skal (pascal.massimino@gmail.com)

#include <assert.h>
#include <stdlib.h>
#include "../dsp/dsp.h"
#include "./rescaler.h"

//------------------------------------------------------------------------------

void WebPRescalerInit(WebPRescaler* const wrk, int src_width, int src_height,
                      uint8_t* const dst, int dst_width, int dst_height,
                      int dst_stride, int num_channels, int x_add, int x_sub,
                      int y_add, int y_sub, int32_t* const work) {
  wrk->x_expand = (src_width < dst_width);
  wrk->src_width = src_width;
  wrk->src_height = src_height;
  wrk->dst_width = dst_width;
  wrk->dst_height = dst_height;
  wrk->dst = dst;
  wrk->dst_stride = dst_stride;
  wrk->num_channels = num_channels;
  // for 'x_expand', we use bilinear interpolation
  wrk->x_add = wrk->x_expand ? (x_sub - 1) : x_add - x_sub;
  wrk->x_sub = wrk->x_expand ? (x_add - 1) : x_sub;
  wrk->y_accum = y_add;
  wrk->y_add = y_add;
  wrk->y_sub = y_sub;
  wrk->fx_scale = (1 << WEBP_RESCALER_RFIX) / x_sub;
  wrk->fy_scale = (1 << WEBP_RESCALER_RFIX) / y_sub;
  wrk->fxy_scale = wrk->x_expand ?
      ((int64_t)dst_height << WEBP_RESCALER_RFIX) / (x_sub * src_height) :
      ((int64_t)dst_height << WEBP_RESCALER_RFIX) / (x_add * src_height);
  wrk->irow = work;
  wrk->frow = work + num_channels * dst_width;

  WebPRescalerDspInit();
}

//------------------------------------------------------------------------------
// all-in-one calls

int WebPRescaleNeededLines(const WebPRescaler* const wrk, int max_num_lines) {
  const int num_lines = (wrk->y_accum + wrk->y_sub - 1) / wrk->y_sub;
  return (num_lines > max_num_lines) ? max_num_lines : num_lines;
}

int WebPRescalerImport(WebPRescaler* const wrk, int num_lines,
                       const uint8_t* src, int src_stride) {
  int total_imported = 0;
  while (total_imported < num_lines && wrk->y_accum > 0) {
    int channel;
    for (channel = 0; channel < wrk->num_channels; ++channel) {
      WebPRescalerImportRow(wrk, src, channel);
    }
    src += src_stride;
    ++total_imported;
    wrk->y_accum -= wrk->y_sub;
  }
  return total_imported;
}

int WebPRescalerExport(WebPRescaler* const rescaler) {
  int total_exported = 0;
  while (WebPRescalerHasPendingOutput(rescaler)) {
    WebPRescalerExportRow(rescaler, 0);
    ++total_exported;
  }
  return total_exported;
}

//------------------------------------------------------------------------------
