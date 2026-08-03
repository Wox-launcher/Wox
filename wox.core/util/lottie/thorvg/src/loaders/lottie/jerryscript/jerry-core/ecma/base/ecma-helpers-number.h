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
 */

#ifndef ECMA_HELPERS_NUMBER_H
#define ECMA_HELPERS_NUMBER_H

#include "ecma-globals.h"

/**
 * Binary representation of an ecma-number
 */
typedef uint32_t ecma_binary_num_t;

/**
 * Makes it possible to read/write the binary representation of an ecma_number_t
 * without strict aliasing rule violation.
 */
typedef union
{
  ecma_number_t as_number; /**< ecma-number */
  ecma_binary_num_t as_binary; /**< binary representation */
} ecma_number_accessor_t;

ecma_binary_num_t ecma_number_to_binary (ecma_number_t number);
ecma_number_t ecma_number_from_binary (ecma_binary_num_t binary);

bool ecma_number_sign (ecma_binary_num_t binary);
uint32_t ecma_number_biased_exp (ecma_binary_num_t binary);
uint64_t ecma_number_fraction (ecma_binary_num_t binary);
ecma_number_t ecma_number_create (bool sign, uint32_t biased_exp, uint64_t fraction);

/**
 * Maximum number of significant decimal digits that an ecma-number can store
 */
#define ECMA_NUMBER_MAX_DIGITS (9)

/**
 * Width of sign field
 *
 * See also:
 *          IEEE-754 2008, 3.6, Table 3.5
 */
#define ECMA_NUMBER_SIGN_WIDTH (1)

/**
 * Width of biased exponent field
 *
 * See also:
 *          IEEE-754 2008, 3.6, Table 3.5
 */
#define ECMA_NUMBER_BIASED_EXP_WIDTH (8)

/**
 * Exponent bias
 */
#define ECMA_NUMBER_EXPONENT_BIAS (127)

/**
 * Width of fraction field
 *
 * See also:
 *          IEEE-754 2008, 3.6, Table 3.5
 */
#define ECMA_NUMBER_FRACTION_WIDTH (23)

/**
 * Sign bit in ecma-numbers
 */
#define ECMA_NUMBER_SIGN_BIT 0x80000000u

/**
 * Binary representation of an IEEE-754 QNaN value.
 */
#define ECMA_NUMBER_BINARY_QNAN 0x7fc00000u

/**
 * Binary representation of an IEEE-754 Infinity value.
 */
#define ECMA_NUMBER_BINARY_INF 0x7f800000u

/**
 * Binary representation of an IEEE-754 zero value.
 */
#define ECMA_NUMBER_BINARY_ZERO 0x0ull

/**
 * Number.MIN_VALUE (i.e., the smallest positive value of ecma-number)
 *
 * See also: ECMA_262 v5, 15.7.3.3
 */
#define ECMA_NUMBER_MIN_VALUE (FLT_MIN)

/**
 * Number.MAX_VALUE (i.e., the maximum value of ecma-number)
 *
 * See also: ECMA_262 v5, 15.7.3.2
 */
#define ECMA_NUMBER_MAX_VALUE (FLT_MAX)

/**
 * Number.EPSILON
 *
 * See also: ECMA_262 v6, 20.1.2.1
 */
#define ECMA_NUMBER_EPSILON ((ecma_number_t) 1.1920928955078125e-7)

/**
 * Number.MAX_SAFE_INTEGER
 *
 * See also: ECMA_262 v6, 20.1.2.6
 */
#define ECMA_NUMBER_MAX_SAFE_INTEGER ((ecma_number_t) 0xFFFFFF)

/**
 * Number.MIN_SAFE_INTEGER
 *
 * See also: ECMA_262 v6, 20.1.2.8
 */
#define ECMA_NUMBER_MIN_SAFE_INTEGER ((ecma_number_t) -0xFFFFFF)

/**
 * Number.MAX_VALUE exponent part
 */
#define NUMBER_MAX_DECIMAL_EXPONENT 38

/**
 * Number.MIN_VALUE exponent part
 */
#define NUMBER_MIN_DECIMAL_EXPONENT -45

/**
 * Euler number
 */
#define ECMA_NUMBER_E ((ecma_number_t) 2.7182818284590452354)

/**
 * Natural logarithm of 10
 */
#define ECMA_NUMBER_LN10 ((ecma_number_t) 2.302585092994046)

/**
 * Natural logarithm of 2
 */
#define ECMA_NUMBER_LN2 ((ecma_number_t) 0.6931471805599453)

/**
 * Logarithm base 2 of the Euler number
 */
#define ECMA_NUMBER_LOG2E ((ecma_number_t) 1.4426950408889634)

/**
 * Logarithm base 10 of the Euler number
 */
#define ECMA_NUMBER_LOG10E ((ecma_number_t) 0.4342944819032518)

/**
 * Pi number
 */
#define ECMA_NUMBER_PI ((ecma_number_t) 3.1415926535897932)

/**
 * Square root of 0.5
 */
#define ECMA_NUMBER_SQRT_1_2 ((ecma_number_t) 0.7071067811865476)

/**
 * Square root of 2
 */
#define ECMA_NUMBER_SQRT2 ((ecma_number_t) 1.4142135623730951)

#endif /* !ECMA_HELPERS_NUMBER_H */
