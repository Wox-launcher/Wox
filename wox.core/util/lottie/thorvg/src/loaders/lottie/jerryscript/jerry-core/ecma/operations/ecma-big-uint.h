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

#ifndef ECMA_BIG_UINT_H
#define ECMA_BIG_UINT_H

#include "ecma-globals.h"

#if JERRY_BUILTIN_BIGINT

/**
 * Limit of BigUInt memory allocation in JerryScript.
 */
#define ECMA_BIGINT_MAX_SIZE 0x10000

/**
 * Unsigned type which can hold two digits.
 */
typedef uint64_t ecma_bigint_two_digits_t;

/**
 * Shift used by left/right shifting of a value.
 */
#define ECMA_BIGINT_DIGIT_SHIFT 5

/**
 * Return with the digits of a BigInt value.
 */
#define ECMA_BIGINT_GET_DIGITS(value_p, offset) \
  ((ecma_bigint_digit_t *) (((uint8_t *) (value_p)) + sizeof (ecma_extended_primitive_t) + (offset)))

/**
 * Return with the digits of a BigInt value.
 */
#define ECMA_BIGINT_GET_LAST_DIGIT(value_p, size) *ECMA_BIGINT_GET_DIGITS (value_p, size - sizeof (ecma_bigint_digit_t))

/**
 * Returns true if size is an odd number.
 */
#define ECMA_BIGINT_SIZE_IS_ODD(size) (((size) & sizeof (ecma_bigint_digit_t)) != 0)

/**
 * Returns a two digit value where the high digit is set to the passed digit.
 */
#define ECMA_BIGINT_HIGH_DIGIT(digit) (((ecma_bigint_two_digits_t) digit) << (8 * sizeof (ecma_bigint_digit_t)))

/**
 * Tells whether a number (usually a digit or uint32_t value) is an odd number.
 */
#define ECMA_BIGINT_NUMBER_IS_ODD(number) ((number & 0x1) != 0)

/**
 * Bitwise operation types.
 */
typedef enum
{
  ECMA_BIG_UINT_BITWISE_AND, /**< bitwise 'and' operation */
  ECMA_BIG_UINT_BITWISE_OR, /**< bitwise 'or' operation */
  ECMA_BIG_UINT_BITWISE_XOR, /**< bitwise 'xor' operation */
  ECMA_BIG_UINT_BITWISE_AND_NOT, /**< bitwise 'and not' operation */
} ecma_big_uint_bitwise_operation_types_t;

/**
 * Returns with the type of the operation.
 */
#define ECMA_BIGINT_BITWISE_GET_OPERATION_TYPE(operation_and_options) ((operation_and_options) &0xf)

/**
 * Options for bitwise operations.
 */
typedef enum
{
  ECMA_BIG_UINT_BITWISE_DECREASE_LEFT = (1 << 4), /**< subtract 1 from left value */
  ECMA_BIG_UINT_BITWISE_DECREASE_RIGHT = (1 << 5), /**< subtract 1 from right value */
  ECMA_BIG_UINT_BITWISE_INCREASE_RESULT = (1 << 6), /**< add 1 to the result */
} ecma_big_uint_bitwise_options_t;

/**
 * Subtract 1 from both left and right values.
 */
#define ECMA_BIG_UINT_BITWISE_DECREASE_BOTH (ECMA_BIG_UINT_BITWISE_DECREASE_LEFT | ECMA_BIG_UINT_BITWISE_DECREASE_RIGHT)

ecma_extended_primitive_t *ecma_bigint_create (uint32_t size);
ecma_extended_primitive_t *ecma_big_uint_extend (ecma_extended_primitive_t *value_p, ecma_bigint_digit_t digit);

ecma_bigint_digit_t ecma_big_uint_count_leading_zero (ecma_bigint_digit_t digit);

int ecma_big_uint_compare (ecma_extended_primitive_t *left_value_p, ecma_extended_primitive_t *right_value_p);

ecma_extended_primitive_t *
ecma_big_uint_mul_digit (ecma_extended_primitive_t *value_p, ecma_bigint_digit_t mul, ecma_bigint_digit_t add);

uint8_t *ecma_big_uint_to_string (ecma_extended_primitive_t *value_p,
                                  uint32_t radix,
                                  uint32_t *char_start_p,
                                  uint32_t *char_size_p);

ecma_extended_primitive_t *ecma_big_uint_increase (ecma_extended_primitive_t *value_p);
ecma_extended_primitive_t *ecma_big_uint_decrease (ecma_extended_primitive_t *value_p);

ecma_extended_primitive_t *ecma_big_uint_add (ecma_extended_primitive_t *left_value_p,
                                              ecma_extended_primitive_t *right_value_p);
ecma_extended_primitive_t *ecma_big_uint_sub (ecma_extended_primitive_t *left_value_p,
                                              ecma_extended_primitive_t *right_value_p);
ecma_extended_primitive_t *ecma_big_uint_mul (ecma_extended_primitive_t *left_value_p,
                                              ecma_extended_primitive_t *right_value_p);
ecma_extended_primitive_t *ecma_big_uint_div_mod (ecma_extended_primitive_t *dividend_value_p,
                                                  ecma_extended_primitive_t *divisor_value_p,
                                                  bool is_mod);

ecma_extended_primitive_t *ecma_big_uint_shift_left (ecma_extended_primitive_t *left_value_p, uint32_t right_value);
ecma_extended_primitive_t *
ecma_big_uint_shift_right (ecma_extended_primitive_t *left_value_p, uint32_t right_value, bool increase_result);

ecma_extended_primitive_t *ecma_big_uint_pow (ecma_extended_primitive_t *left_value_p, uint32_t right_value);

ecma_extended_primitive_t *ecma_big_uint_bitwise_op (uint32_t operation_and_options,
                                                     ecma_extended_primitive_t *left_value_p,
                                                     ecma_extended_primitive_t *right_value_p);

#endif /* JERRY_BUILTIN_BIGINT */

#endif /* ECMA_BIG_UINT_H */
