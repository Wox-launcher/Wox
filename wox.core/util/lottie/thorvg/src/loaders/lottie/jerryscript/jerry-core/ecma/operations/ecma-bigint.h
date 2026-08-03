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

#ifndef ECMA_BIG_INT_H
#define ECMA_BIG_INT_H

#include "ecma-globals.h"

#if JERRY_BUILTIN_BIGINT

/**
 * Sign bit of a BigInt value. The number is negative, if this bit is set.
 */
#define ECMA_BIGINT_SIGN 0x1

/**
 * Flags for ecma_bigint_parse_string.
 */
typedef enum
{
  ECMA_BIGINT_PARSE_NO_OPTIONS = 0, /**< no options */
  ECMA_BIGINT_PARSE_SET_NEGATIVE = (1 << 0), /**< return with a negative BigInt value */
  ECMA_BIGINT_PARSE_DISALLOW_SYNTAX_ERROR = (1 << 1), /**< don't throw SyntaxError,
                                                       *   return with ECMA_VALUE_FALSE */
  ECMA_BIGINT_PARSE_DISALLOW_MEMORY_ERROR = (1 << 2), /**< don't throw out-of-memory error,
                                                       *   return with ECMA_VALUE_NULL instead */
  ECMA_BIGINT_PARSE_ALLOW_UNDERSCORE = (1 << 3), /** allow parse underscore characters */
} ecma_bigint_parse_string_options_t;

/**
 * Types for unary operations
 */
typedef enum
{
  ECMA_BIGINT_UNARY_BITWISE_NOT, /**< bitwise not operation */
  ECMA_BIGINT_UNARY_INCREASE, /**< increase operation */
  ECMA_BIGINT_UNARY_DECREASE, /**< decrease operation */
} ecma_bigint_unary_operation_type;

ecma_value_t ecma_bigint_parse_string (const lit_utf8_byte_t *string_p, lit_utf8_size_t size, uint32_t options);
ecma_value_t ecma_bigint_parse_string_value (ecma_value_t string, uint32_t options);
ecma_string_t *ecma_bigint_to_string (ecma_value_t value, ecma_bigint_digit_t radix);
ecma_value_t ecma_bigint_to_bigint (ecma_value_t value, bool allow_numbers);
ecma_value_t ecma_bigint_to_number (ecma_value_t value);
ecma_value_t ecma_bigint_get_bigint (ecma_value_t value, bool *free_result_p);
ecma_value_t ecma_bigint_create_from_digits (const uint64_t *digits_p, uint32_t size, bool sign);
uint32_t ecma_bigint_get_size_in_digits (ecma_value_t value);
void ecma_bigint_get_digits_and_sign (ecma_value_t value, uint64_t *digits_p, uint32_t size, bool *sign_p);

bool ecma_bigint_is_equal_to_bigint (ecma_value_t left_value, ecma_value_t right_value);
bool ecma_bigint_is_equal_to_number (ecma_value_t left_value, ecma_number_t right_value);
int ecma_bigint_compare_to_bigint (ecma_value_t left_value, ecma_value_t right_value);
int ecma_bigint_compare_to_number (ecma_value_t left_value, ecma_number_t right_value);

ecma_value_t ecma_bigint_negate (ecma_extended_primitive_t *value_p);
ecma_value_t ecma_bigint_unary (ecma_value_t value, ecma_bigint_unary_operation_type type);
ecma_value_t ecma_bigint_add_sub (ecma_value_t left_value, ecma_value_t right_value, bool is_add);
ecma_value_t ecma_bigint_mul (ecma_value_t left_value, ecma_value_t right_value);
ecma_value_t ecma_bigint_div_mod (ecma_value_t left_value, ecma_value_t right_value, bool is_mod);
ecma_value_t ecma_bigint_shift (ecma_value_t left_value, ecma_value_t right_value, bool is_left);
ecma_value_t ecma_bigint_pow (ecma_value_t left_value, ecma_value_t right_value);

ecma_value_t ecma_bigint_and (ecma_value_t left_value, ecma_value_t right_value);
ecma_value_t ecma_bigint_or (ecma_value_t left_value, ecma_value_t right_value);
ecma_value_t ecma_bigint_xor (ecma_value_t left_value, ecma_value_t right_value);

#endif /* JERRY_BUILTIN_BIGINT */

#endif /* ECMA_BIG_INT_H */
