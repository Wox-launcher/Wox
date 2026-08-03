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

#include "ecma-helpers-number.h"

#include <math.h>

#include "ecma-conversion.h"

#include "lit-char-helpers.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmahelpers Helpers for operations with ECMA data types
 * @{
 */

JERRY_STATIC_ASSERT ((sizeof (ecma_value_t) == sizeof (ecma_integer_value_t)),
                     size_of_ecma_value_t_must_be_equal_to_the_size_of_ecma_integer_value_t);

JERRY_STATIC_ASSERT ((((int)ECMA_DIRECT_SHIFT) == (((int)ECMA_VALUE_SHIFT) + 1)), currently_directly_encoded_values_has_one_extra_flag);

JERRY_STATIC_ASSERT ((((1 << (ECMA_DIRECT_SHIFT - 1)) | ECMA_TYPE_DIRECT) == ECMA_DIRECT_TYPE_SIMPLE_VALUE),
                     currently_directly_encoded_values_start_after_direct_type_simple_value);

JERRY_STATIC_ASSERT ((sizeof (ecma_number_t) == sizeof (ecma_binary_num_t)),
                     size_of_ecma_number_t_must_be_equal_to_binary_representation);

/**
 * Convert an ecma-number to it's binary representation.
 *
 * @return binary representation
 */
ecma_binary_num_t JERRY_ATTR_CONST
ecma_number_to_binary (ecma_number_t number) /**< ecma number */
{
  ecma_number_accessor_t f;
  f.as_number = number;

  return f.as_binary;
} /* ecma_number_to_binary */

/**
 * Convert a binary representation to the corresponding ecma-number.
 *
 * @return ecma-number
 */
ecma_number_t JERRY_ATTR_CONST
ecma_number_from_binary (ecma_binary_num_t binary) /**< binary representation */
{
  ecma_number_accessor_t f;
  f.as_binary = binary;

  return f.as_number;
} /* ecma_number_from_binary */

/**
 * Check signedness of the binary number.
 *
 * @return true  - if sign bit is set
 *         false - otherwise
 */
bool JERRY_ATTR_CONST
ecma_number_sign (ecma_binary_num_t binary) /**< binary representation */
{
  return (binary & ECMA_NUMBER_SIGN_BIT) != 0;
} /* ecma_number_sign */

/**
 * Get biased exponent field of the binary number.
 *
 * @return unsigned integer value of the biased exponent field
 */
uint32_t JERRY_ATTR_CONST
ecma_number_biased_exp (ecma_binary_num_t binary) /**< binary representation */
{
  return (uint32_t) ((binary & ~ECMA_NUMBER_SIGN_BIT) >> ECMA_NUMBER_FRACTION_WIDTH);
} /* ecma_number_biased_exp */

/**
 * Get fraction field of the binary number.
 *
 * @return unsigned integer value of the fraction field
 */
uint64_t JERRY_ATTR_CONST
ecma_number_fraction (ecma_binary_num_t binary) /**< binary representation */
{
  return binary & ((1ull << ECMA_NUMBER_FRACTION_WIDTH) - 1);
} /* ecma_number_fraction */

/**
 * Packing sign, fraction and biased exponent to ecma-number
 *
 * @return ecma-number with specified sign, biased_exponent and fraction
 */
ecma_number_t
ecma_number_create (bool sign, /**< sign */
                    uint32_t biased_exp, /**< biased exponent */
                    uint64_t fraction) /**< fraction */
{
  JERRY_ASSERT ((biased_exp & ~((1u << ECMA_NUMBER_BIASED_EXP_WIDTH) - 1)) == 0);
  JERRY_ASSERT ((fraction & ~((1ull << ECMA_NUMBER_FRACTION_WIDTH) - 1)) == 0);

  ecma_binary_num_t binary = biased_exp;
  binary <<= ECMA_NUMBER_FRACTION_WIDTH;

  binary |= fraction;

  if (sign)
  {
    binary |= ECMA_NUMBER_SIGN_BIT;
  }

  return ecma_number_from_binary (binary);
} /* ecma_number_create */

/**
 * Check if ecma-number is NaN
 *
 * @return true - if biased exponent is filled with 1 bits and
                  fraction is filled with anything but not all zero bits,
 *         false - otherwise
 */
bool
ecma_number_is_nan (ecma_number_t num) /**< ecma-number */
{
  bool is_nan = (num != num);

#ifndef JERRY_NDEBUG
  /* IEEE-754 2008, 3.4, a */
  ecma_binary_num_t binary = ecma_number_to_binary (num);
  bool is_nan_exponent = (ecma_number_biased_exp (binary) == (1 << ECMA_NUMBER_BIASED_EXP_WIDTH) - 1);
  bool is_nan_fraction = (ecma_number_fraction (binary) > 0);

  bool is_nan_ieee754 = is_nan_exponent && is_nan_fraction;
  JERRY_ASSERT (is_nan == is_nan_ieee754);
#endif /* !JERRY_NDEBUG */

  return is_nan;
} /* ecma_number_is_nan */

/**
 * Make a NaN.
 *
 * @return NaN value
 */
ecma_number_t JERRY_ATTR_CONST
ecma_number_make_nan (void)
{
  ecma_number_accessor_t f;
  f.as_binary = ECMA_NUMBER_BINARY_QNAN;

  return f.as_number;
} /* ecma_number_make_nan */

/**
 * Make an Infinity.
 *
 * @return if !sign - +Infinity value,
 *         else - -Infinity value.
 */
ecma_number_t JERRY_ATTR_CONST
ecma_number_make_infinity (bool sign) /**< sign of the value */
{
  ecma_number_accessor_t f;
  f.as_binary = ECMA_NUMBER_BINARY_INF;

  if (sign)
  {
    f.as_binary |= ECMA_NUMBER_SIGN_BIT;
  }

  return f.as_number;
} /* ecma_number_make_infinity */

/**
 * Check if ecma-number is negative
 *
 * @return true - if sign bit of ecma-number is set
 *         false - otherwise
 */
bool JERRY_ATTR_CONST
ecma_number_is_negative (ecma_number_t num) /**< ecma-number */
{
  JERRY_ASSERT (!ecma_number_is_nan (num));

  return (ecma_number_to_binary (num) & ECMA_NUMBER_SIGN_BIT) != 0;
} /* ecma_number_is_negative */

/**
 * Check if ecma-number is zero
 *
 * @return true - if fraction is zero and biased exponent is zero,
 *         false - otherwise
 */
bool JERRY_ATTR_CONST
ecma_number_is_zero (ecma_number_t num) /**< ecma-number */
{
  bool is_zero = (num == ECMA_NUMBER_ZERO);

#ifndef JERRY_NDEBUG
  bool is_zero_ieee754 = ((ecma_number_to_binary (num) & ~ECMA_NUMBER_SIGN_BIT) == 0);
  JERRY_ASSERT (is_zero == is_zero_ieee754);
#endif /* !JERRY_NDEBUG */

  return is_zero;
} /* ecma_number_is_zero */

/**
 * Check if number is infinity
 *
 * @return true - if biased exponent is filled with 1 bits and
 *                fraction is filled with zero bits,
 *         false - otherwise
 */
bool JERRY_ATTR_CONST
ecma_number_is_infinity (ecma_number_t num) /**< ecma-number */
{
  return (ecma_number_to_binary (num) & ~ECMA_NUMBER_SIGN_BIT) == ECMA_NUMBER_BINARY_INF;
} /* ecma_number_is_infinity */

/**
 * Check if number is finite
 *
 * @return true  - if number is finite
 *         false - if number is NaN or infinity
 */
bool JERRY_ATTR_CONST
ecma_number_is_finite (ecma_number_t num) /**< ecma-number */
{
#if defined(__GNUC__) || defined(__clang__)
  return __builtin_isfinite (num);
#elif defined(_WIN32)
  return isfinite (num);
#else /* !(defined(__GNUC__) || defined(__clang__) || defined(_WIN32)) */
  return !ecma_number_is_nan (num) && !ecma_number_is_infinity (num);
#endif /* defined (__GNUC__) || defined (__clang__) */
} /* ecma_number_is_finite */

/**
 * Get previous representable ecma-number
 *
 * @return maximum ecma-number that is less compared to passed argument
 */
ecma_number_t JERRY_ATTR_CONST
ecma_number_get_prev (ecma_number_t num) /**< ecma-number */
{
#if defined(__GNUC__) || defined(__clang__)
  return __builtin_nextafter ((double)num, (double)-INFINITY);
#else /* !defined (__GNUC__) && !defined (__clang__) */
  JERRY_ASSERT (!ecma_number_is_nan (num));
  ecma_binary_num_t binary = ecma_number_to_binary (num);

  /* If -Infinity, return self */
  if (binary == (ECMA_NUMBER_SIGN_BIT | ECMA_NUMBER_BINARY_INF))
  {
    return num;
  }

  /* If +0.0, return -0.0 */
  if (binary == ECMA_NUMBER_BINARY_ZERO)
  {
    return -num;
  }

  if (ecma_number_sign (binary))
  {
    return ecma_number_from_binary (binary + 1);
  }

  return ecma_number_from_binary (binary - 1);
#endif /* !defined (__GNUC__) && !defined (__clang__) */
} /* ecma_number_get_prev */

/**
 * Get next representable ecma-number
 *
 * @return minimum ecma-number that is greater compared to passed argument
 */
ecma_number_t JERRY_ATTR_CONST
ecma_number_get_next (ecma_number_t num) /**< ecma-number */
{
#if defined(__GNUC__) || defined(__clang__)
  return __builtin_nextafter ((double)num, (double)INFINITY);
#else /* !defined (__GNUC__) && !defined (__clang__) */
  JERRY_ASSERT (!ecma_number_is_nan (num));
  ecma_binary_num_t binary = ecma_number_to_binary (num);

  /* If +Infinity, return self */
  if (binary == ECMA_NUMBER_BINARY_INF)
  {
    return num;
  }

  /* If -0.0, return +0.0 */
  if (binary == (ECMA_NUMBER_SIGN_BIT | ECMA_NUMBER_BINARY_ZERO))
  {
    return -num;
  }

  if (ecma_number_sign (binary))
  {
    return ecma_number_from_binary (binary - 1);
  }

  return ecma_number_from_binary (binary + 1);
#endif /* !defined (__GNUC__) && !defined (__clang__) */
} /* ecma_number_get_next */

/**
 * Truncate fractional part of the number
 *
 * @return integer part of the number
 */
ecma_number_t JERRY_ATTR_CONST
ecma_number_trunc (ecma_number_t num) /**< ecma-number */
{
  JERRY_ASSERT (!ecma_number_is_nan (num));

  ecma_binary_num_t binary = ecma_number_to_binary (num);
  uint32_t exponent = ecma_number_biased_exp (binary);

  if (exponent < ECMA_NUMBER_EXPONENT_BIAS)
  {
    return ECMA_NUMBER_ZERO;
  }

  uint32_t unbiased_exp = exponent - ECMA_NUMBER_EXPONENT_BIAS;

  if (unbiased_exp >= ECMA_NUMBER_FRACTION_WIDTH)
  {
    return num;
  }

  binary &= ~((1ull << (ECMA_NUMBER_FRACTION_WIDTH - unbiased_exp)) - 1);
  return ecma_number_from_binary (binary);
} /* ecma_number_trunc */

/**
 * Calculate remainder of division of two numbers,
 * as specified in ECMA-262 v5, 11.5.3, item 6.
 *
 * Note:
 *      operands shouldn't contain NaN, Infinity, or zero.
 *
 * @return number - calculated remainder.
 */
ecma_number_t JERRY_ATTR_CONST
ecma_number_remainder (ecma_number_t left_num, /**< left operand */
                       ecma_number_t right_num) /**< right operand */
{
  JERRY_ASSERT (ecma_number_is_finite (left_num) && !ecma_number_is_zero (left_num));
  JERRY_ASSERT (ecma_number_is_finite (right_num) && !ecma_number_is_zero (right_num));

  const ecma_number_t q = ecma_number_trunc (left_num / right_num);
  ecma_number_t r = left_num - right_num * q;

  if (ecma_number_is_zero (r) && ecma_number_is_negative (left_num))
  {
    r = -r;
  }

  return r;
} /* ecma_number_remainder */

/**
 * Compute power operation according to the ES standard.
 *
 * @return x ** y
 */
ecma_number_t JERRY_ATTR_CONST
ecma_number_pow (ecma_number_t x, /**< left operand */
                 ecma_number_t y) /**< right operand */
{
  if (ecma_number_is_nan (y) || (ecma_number_is_infinity (y) && (x == ECMA_NUMBER_ONE || x == ECMA_NUMBER_MINUS_ONE)))
  {
    /* Handle differences between ES5.1 and ISO C standards for pow. */
    return ecma_number_make_nan ();
  }

  if (ecma_number_is_zero (y))
  {
    /* Handle differences between ES5.1 and ISO C standards for pow. */
    return ECMA_NUMBER_ONE;
  }

  return DOUBLE_TO_ECMA_NUMBER_T (pow (x, y));
} /* ecma_number_pow */

/**
 * ECMA-integer number multiplication.
 *
 * @return number - result of multiplication.
 */
ecma_value_t JERRY_ATTR_CONST
ecma_integer_multiply (ecma_integer_value_t left_integer, /**< left operand */
                       ecma_integer_value_t right_integer) /**< right operand */
{
#if defined(__GNUC__) || defined(__clang__)
  /* Check if either integer is power of 2 */
  if (JERRY_UNLIKELY ((left_integer & (left_integer - 1)) == 0))
  {
    /* Right shift right_integer with log2 (left_integer) */
    return ecma_make_integer_value (
      (int32_t) ((uint32_t) right_integer << (__builtin_ctz ((unsigned int) left_integer))));
  }

  if (JERRY_UNLIKELY ((right_integer & (right_integer - 1)) == 0))
  {
    /* Right shift left_integer with log2 (right_integer) */
    return ecma_make_integer_value (
      (int32_t) ((uint32_t) left_integer << (__builtin_ctz ((unsigned int) right_integer))));
  }
#endif /* defined (__GNUC__) || defined (__clang__) */

  return ecma_make_integer_value (left_integer * right_integer);
} /* ecma_integer_multiply */

/**
 * The Number object's 'parseInt' routine
 *
 * See also:
 *          ECMA-262 v5, 15.1.2.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_number_parse_int (const lit_utf8_byte_t *str_p, /**< routine's first argument's
                                                      *   string buffer */
                       lit_utf8_size_t str_size, /**< routine's first argument's
                                                  *   string buffer's size */
                       ecma_value_t radix_value) /**< routine's second argument */
{
  /* 2. Remove leading whitespace. */
  ecma_string_trim_helper (&str_p, &str_size);

  if (str_size == 0)
  {
    return ecma_make_nan_value ();
  }

  const lit_utf8_byte_t *str_end_p = str_p + str_size;

  /* 3. */
  bool sign = false;

  /* 4. */
  if (*str_p == LIT_CHAR_MINUS)
  {
    sign = true;
    str_p++;
  }
  /* 5. */
  else if (*str_p == LIT_CHAR_PLUS)
  {
    str_p++;
  }

  /* 6. */
  ecma_number_t radix_num;
  radix_value = ecma_op_to_number (radix_value, &radix_num);

  if (ECMA_IS_VALUE_ERROR (radix_value))
  {
    return ECMA_VALUE_ERROR;
  }

  int32_t radix = ecma_number_to_int32 (radix_num);

  /* 7.*/
  bool strip_prefix = true;

  /* 8. */
  if (radix != 0)
  {
    /* 8.a */
    if (radix < 2 || radix > 36)
    {
      return ecma_make_nan_value ();
    }
    /* 8.b */
    else if (radix != 16)
    {
      strip_prefix = false;
    }
  }
  /* 9. */
  else
  {
    radix = 10;
  }

  /* 10. */
  if (strip_prefix && ((str_end_p - str_p) >= 2) && (str_p[0] == LIT_CHAR_0)
      && (LEXER_TO_ASCII_LOWERCASE (str_p[1]) == LIT_CHAR_LOWERCASE_X))
  {
    str_p += 2;
    radix = 16;
  }

  ecma_number_t value = ECMA_NUMBER_ZERO;
  const lit_utf8_byte_t *digit_start_p = str_p;

  /* 11. Check if characters are in [0, Radix - 1]. We also convert them to number values in the process. */
  while (str_p < str_end_p)
  {
    ecma_char_t ch = *str_p;

    int32_t digit = 0;

    if (lit_char_is_decimal_digit (ch))
    {
      digit = ch - LIT_CHAR_0;
    }
    else if (LEXER_TO_ASCII_LOWERCASE (ch) >= LIT_CHAR_LOWERCASE_A
             && LEXER_TO_ASCII_LOWERCASE (ch) <= LIT_CHAR_LOWERCASE_Z)
    {
      digit = LEXER_TO_ASCII_LOWERCASE (ch) - LIT_CHAR_LOWERCASE_A + 10;
    }
    else
    {
      /* Not a valid digit char, set to invalid value */
      digit = radix;
    }

    if (digit >= radix)
    {
      break;
    }

    value *= radix;
    value += digit;

    str_p++;
  }

  /* 12. */
  if (str_p == digit_start_p)
  {
    return ecma_make_nan_value ();
  }

  /* 15. */
  if (sign)
  {
    value *= ECMA_NUMBER_MINUS_ONE;
  }

  return ecma_make_number_value (value);
} /* ecma_number_parse_int */

/**
 * The Number object's 'parseFloat' routine
 *
 * See also:
 *          ECMA-262 v5, 15.1.2.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_number_parse_float (const lit_utf8_byte_t *str_p, /**< routine's first argument's
                                                        *   string buffer */
                         lit_utf8_size_t str_size) /**< routine's first argument's
                                                    *   string buffer's size */
{
  /* 2. Remove leading whitespace. */
  ecma_string_trim_helper (&str_p, &str_size);

  const lit_utf8_byte_t *str_end_p = str_p + str_size;
  bool sign = false;

  if (str_size == 0)
  {
    return ecma_make_nan_value ();
  }

  if (*str_p == LIT_CHAR_PLUS)
  {
    str_p++;
  }
  else if (*str_p == LIT_CHAR_MINUS)
  {
    sign = true;
    str_p++;
  }

  /* Check if string is equal to "Infinity". */
  const lit_utf8_byte_t *infinity_str_p = lit_get_magic_string_utf8 (LIT_MAGIC_STRING_INFINITY_UL);
  const lit_utf8_size_t infinity_length = lit_get_magic_string_size (LIT_MAGIC_STRING_INFINITY_UL);

  /* The input string should be at least the length of "Infinity" to be correctly processed as
   * the infinity value.
   */
  if ((lit_utf8_size_t) (str_end_p - str_p) >= infinity_length && memcmp (infinity_str_p, str_p, infinity_length) == 0)
  {
    return ecma_make_number_value (ecma_number_make_infinity (sign));
  }

  const lit_utf8_byte_t *num_start_p = str_p;
  const lit_utf8_byte_t *num_end_p = str_p;

  while (str_p < str_end_p && lit_char_is_decimal_digit (*str_p))
  {
    str_p++;
  }

  if (str_p < str_end_p && *str_p == LIT_CHAR_DOT)
  {
    str_p++;

    while (str_p < str_end_p && lit_char_is_decimal_digit (*str_p))
    {
      str_p++;
    }
  }

  num_end_p = str_p;

  if (str_p < str_end_p && LEXER_TO_ASCII_LOWERCASE (*str_p) == LIT_CHAR_LOWERCASE_E)
  {
    str_p++;

    if (str_p < str_end_p && (*str_p == LIT_CHAR_PLUS || *str_p == LIT_CHAR_MINUS))
    {
      str_p++;
    }

    if (str_p < str_end_p && lit_char_is_decimal_digit (*str_p))
    {
      str_p++;

      while (str_p < str_end_p && lit_char_is_decimal_digit (*str_p))
      {
        str_p++;
      }

      num_end_p = str_p;
    }
  }

  lit_utf8_size_t num_size = (lit_utf8_size_t) (num_end_p - num_start_p);

  if (num_size == 0)
  {
    return ecma_make_nan_value ();
  }

  /* 5. */
  ecma_number_t ret_num = ecma_utf8_string_to_number (num_start_p, num_size, 0);

  if (sign)
  {
    ret_num *= ECMA_NUMBER_MINUS_ONE;
  }

  return ecma_make_number_value (ret_num);
} /* ecma_number_parse_float */

/**
 * @}
 * @}
 */
