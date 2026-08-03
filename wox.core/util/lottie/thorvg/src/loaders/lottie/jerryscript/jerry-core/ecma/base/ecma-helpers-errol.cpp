/* Copyright JS Foundation and other contributors, http://js.foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * This file is based on work under the following copyright and permission
 * notice:
 *
 *   Copyright (c) 2016 Marc Andrysco
 *
 *   Permission is hereby granted, free of charge, to any person obtaining a copy
 *   of this software and associated documentation files (the "Software"), to deal
 *   in the Software without restriction, including without limitation the rights
 *   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *   copies of the Software, and to permit persons to whom the Software is
 *   furnished to do so, subject to the following conditions:
 *
 *   The above copyright notice and this permission notice shall be included in all
 *   copies or substantial portions of the Software.
 *
 *   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *   SOFTWARE.
 */

#include <math.h>

#include "ecma-helpers.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmahelpers Helpers for operations with ECMA data types
 * @{
 */

/**
 * Printing Floating-Point Numbers
 *
 * available at http://cseweb.ucsd.edu/~mandrysc/pub/dtoa.pdf
 */

/**
 * Floating point format definitions (next float value)
 */
#define ECMA_NEXT_FLOAT(value) (nextafter ((value), INFINITY))
/**
 * Floating point format definitions (previous float value)
 */
#define ECMA_PREV_FLOAT(value) (nextafter ((value), -INFINITY))

/**
 * Value of epsilon
 */
#define ERROL0_EPSILON 0.0000001

/**
 * High-precision data structure.
 */
typedef struct
{
  double value; /**< value */
  double offset; /**< offset */
} ecma_high_prec_t;

/**
 * Normalize the number by factoring in the error.
 *
 * @return void
 */
static inline void
ecma_normalize_high_prec_data (ecma_high_prec_t *hp_data_p) /**< [in, out] float pair */
{
  double val = hp_data_p->value;

  hp_data_p->value += hp_data_p->offset;
  hp_data_p->offset += val - hp_data_p->value;
} /* ecma_normalize_high_prec_data */

/**
 * Multiply the high-precision number by ten.
 *
 * @return void
 */
static inline void
ecma_multiply_high_prec_by_10 (ecma_high_prec_t *hp_data_p) /**< [in, out] high-precision number */
{
  double value = hp_data_p->value;

  hp_data_p->value *= 10.0;
  hp_data_p->offset *= 10.0;

  double offset = hp_data_p->value;

  offset -= value * 8.0;
  offset -= value * 2.0;

  hp_data_p->offset -= offset;

  ecma_normalize_high_prec_data (hp_data_p);
} /* ecma_multiply_high_prec_by_10 */

/**
 * Divide the high-precision number by ten.
 */
static void
ecma_divide_high_prec_by_10 (ecma_high_prec_t *hp_data_p) /**< [in, out] high-precision number */
{
  double value = hp_data_p->value;

  hp_data_p->value /= 10.0;
  hp_data_p->offset /= 10.0;

  value -= hp_data_p->value * 8.0;
  value -= hp_data_p->value * 2.0;

  hp_data_p->offset += value / 10.0;

  ecma_normalize_high_prec_data (hp_data_p);
} /* ecma_divide_high_prec_by_10 */

/**
 * Errol0 double to ASCII conversion, guaranteed correct but possibly not optimal.
 *
 * @return number of generated digits
 */
lit_utf8_size_t
ecma_errol0_dtoa (double val, /**< ecma number */
                  lit_utf8_byte_t *buffer_p, /**< buffer to generate digits into */
                  int32_t *exp_p) /**< [out] exponent */
{
  double power_of_10 = 1.0;
  int32_t exponent = 1;

  /* normalize the midpoint */
  ecma_high_prec_t mid;

  mid.value = val;
  mid.offset = 0.0;

  while (((mid.value > 10.0) || ((mid.value == 10.0) && (mid.offset >= 0.0))) && (exponent < 308))
  {
    exponent++;
    ecma_divide_high_prec_by_10 (&mid);
    power_of_10 /= 10.0;
  }

  while (((mid.value < 1.0) || ((mid.value == 1.0) && (mid.offset < 0.0))) && (exponent > -307))
  {
    exponent--;
    ecma_multiply_high_prec_by_10 (&mid);
    power_of_10 *= 10.0;
  }

  ecma_high_prec_t high_bound, low_bound;

  high_bound.value = mid.value;
  high_bound.offset = mid.offset;

  if ((float) ECMA_NEXT_FLOAT (val) != (float) INFINITY)
  {
    high_bound.offset += (ECMA_NEXT_FLOAT (val) - val) * power_of_10 / (2.0 + ERROL0_EPSILON);
  }

  low_bound.value = mid.value;
  low_bound.offset = mid.offset + (ECMA_PREV_FLOAT (val) - val) * power_of_10 / (2.0 + ERROL0_EPSILON);

  ecma_normalize_high_prec_data (&high_bound);
  ecma_normalize_high_prec_data (&low_bound);

  /* normalized boundaries */

  while (high_bound.value > 10.0 || (high_bound.value == 10.0 && (high_bound.offset >= 0.0)))
  {
    exponent++;
    ecma_divide_high_prec_by_10 (&high_bound);
    ecma_divide_high_prec_by_10 (&low_bound);
  }

  while (high_bound.value < 1.0 || (high_bound.value == 1.0 && (high_bound.offset < 0.0)))
  {
    exponent--;
    ecma_multiply_high_prec_by_10 (&high_bound);
    ecma_multiply_high_prec_by_10 (&low_bound);
  }

  /* digit generation */

  lit_utf8_byte_t *dst_p = buffer_p;

  while (high_bound.value != 0.0 || high_bound.offset != 0.0)
  {
    uint8_t high_digit = (uint8_t) high_bound.value;

    if ((high_bound.value == high_digit) && (high_bound.offset < 0))
    {
      high_digit = (uint8_t) (high_digit - 1u);
    }

    uint8_t low_digit = (uint8_t) low_bound.value;

    if ((low_bound.value == low_digit) && (low_bound.offset < 0))
    {
      low_digit = (uint8_t) (low_digit - 1u);
    }

    if (low_digit != high_digit)
    {
      break;
    }

    *dst_p++ = (lit_utf8_byte_t) ('0' + high_digit);

    high_bound.value -= high_digit;
    ecma_multiply_high_prec_by_10 (&high_bound);

    low_bound.value -= low_digit;
    ecma_multiply_high_prec_by_10 (&low_bound);
  }

  double mdig = (high_bound.value + low_bound.value) / 2.0 + 0.5;
  *dst_p++ = (lit_utf8_byte_t) ('0' + (uint8_t) mdig);

  *exp_p = exponent;

  return (lit_utf8_size_t) (dst_p - buffer_p);
} /* ecma_errol0_dtoa */

/**
 * @}
 * @}
 */
