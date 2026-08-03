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

#include "ecma-bigint.h"

#include "ecma-big-uint.h"
#include "ecma-exceptions.h"
#include "ecma-helpers-number.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"

#include "lit-char-helpers.h"

#if JERRY_BUILTIN_BIGINT

/**
 * Raise a not enough memory error
 *
 * @return ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_bigint_raise_memory_error (void)
{
  return ecma_raise_range_error (ECMA_ERR_ALLOCATE_BIGINT_VALUE);
} /* ecma_bigint_raise_memory_error */

/**
 * Create a single digit long BigInt value
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_bigint_create_from_digit (ecma_bigint_digit_t digit, /* single digit */
                               bool sign) /* set ECMA_BIGINT_SIGN if true */
{
  JERRY_ASSERT (digit != 0);

  ecma_extended_primitive_t *result_value_p = ecma_bigint_create (sizeof (ecma_bigint_digit_t));

  if (JERRY_UNLIKELY (result_value_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  if (sign)
  {
    result_value_p->u.bigint_sign_and_size |= ECMA_BIGINT_SIGN;
  }

  *ECMA_BIGINT_GET_DIGITS (result_value_p, 0) = digit;
  return ecma_make_extended_primitive_value (result_value_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_create_from_digit */

/**
 * Parse a string and create a BigInt value
 *
 * @return ecma BigInt value or a special value allowed by the option flags
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_parse_string (const lit_utf8_byte_t *string_p, /**< string representation of the BigInt */
                          lit_utf8_size_t size, /**< string size */
                          uint32_t options) /**< ecma_bigint_parse_string_options_t option bits */
{
  ecma_bigint_digit_t radix = 10;
  uint32_t sign = (options & ECMA_BIGINT_PARSE_SET_NEGATIVE) ? ECMA_BIGINT_SIGN : 0;
  bool allow_underscore = options & ECMA_BIGINT_PARSE_ALLOW_UNDERSCORE;

  const lit_utf8_byte_t *string_end_p = string_p + size;
  string_p = ecma_string_trim_front (string_p, string_p + size);
  size = (lit_utf8_size_t) (string_end_p - string_p);

  if (size >= 3 && string_p[0] == LIT_CHAR_0)
  {
    radix = lit_char_to_radix (string_p[1]);

    if (radix != 10)
    {
      string_p += 2;
      size -= 2;
    }
  }
  else if (size >= 2)
  {
    if (string_p[0] == LIT_CHAR_PLUS)
    {
      size--;
      string_p++;
    }
    else if (string_p[0] == LIT_CHAR_MINUS)
    {
      sign = ECMA_BIGINT_SIGN;
      size--;
      string_p++;
    }
  }
  else if (size == 0)
  {
    return ECMA_BIGINT_ZERO;
  }

  while (string_p < string_end_p && (*string_p == LIT_CHAR_0 || (*string_p == LIT_CHAR_UNDERSCORE && allow_underscore)))
  {
    string_p++;
  }

  ecma_extended_primitive_t *result_p = NULL;

  if (string_p == string_end_p)
  {
    return ECMA_BIGINT_ZERO;
  }

  do
  {
    ecma_bigint_digit_t digit = radix;

    if (*string_p >= LIT_CHAR_0 && *string_p <= LIT_CHAR_9)
    {
      digit = (ecma_bigint_digit_t) (*string_p - LIT_CHAR_0);
    }
    else if (*string_p == LIT_CHAR_UNDERSCORE && allow_underscore)
    {
      continue;
    }
    else
    {
      lit_utf8_byte_t character = (lit_utf8_byte_t) LEXER_TO_ASCII_LOWERCASE (*string_p);

      if (character >= LIT_CHAR_LOWERCASE_A && character <= LIT_CHAR_LOWERCASE_F)
      {
        digit = (ecma_bigint_digit_t) (character - (LIT_CHAR_LOWERCASE_A - 10));
      }
      else if (ecma_string_trim_front (string_p, string_end_p) == string_end_p)
      {
        string_p = string_end_p;
        break;
      }
    }
    if (digit >= radix)
    {
      if (result_p != NULL)
      {
        ecma_deref_bigint (result_p);
      }

      if (options & ECMA_BIGINT_PARSE_DISALLOW_SYNTAX_ERROR)
      {
        return ECMA_VALUE_FALSE;
      }
      return ecma_raise_syntax_error (ECMA_ERR_STRING_CANNOT_BE_CONVERTED_TO_BIGINT_VALUE);
    }

    result_p = ecma_big_uint_mul_digit (result_p, radix, digit);

    if (JERRY_UNLIKELY (result_p == NULL))
    {
      break;
    }
  } while (++string_p < string_end_p);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    if (options & ECMA_BIGINT_PARSE_DISALLOW_MEMORY_ERROR)
    {
      return ECMA_VALUE_NULL;
    }
    return ecma_bigint_raise_memory_error ();
  }

  result_p->u.bigint_sign_and_size |= sign;
  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_parse_string */

/**
 * Parse a string value and create a BigInt value
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_parse_string_value (ecma_value_t string, /**< ecma string */
                                uint32_t options) /**< ecma_bigint_parse_string_options_t option bits */
{
  JERRY_ASSERT (ecma_is_value_string (string));

  ECMA_STRING_TO_UTF8_STRING (ecma_get_string_from_value (string), string_buffer_p, string_buffer_size);

  ecma_value_t result = ecma_bigint_parse_string (string_buffer_p, string_buffer_size, options);
  ECMA_FINALIZE_UTF8_STRING (string_buffer_p, string_buffer_size);

  return result;
} /* ecma_bigint_parse_string_value */

/**
 * Create a string representation for a BigInt value
 *
 * @return ecma string or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_string_t *
ecma_bigint_to_string (ecma_value_t value, /**< BigInt value */
                       ecma_bigint_digit_t radix) /**< conversion radix */
{
  JERRY_ASSERT (ecma_is_value_bigint (value));

  if (value == ECMA_BIGINT_ZERO)
  {
    return ecma_new_ecma_string_from_code_unit (LIT_CHAR_0);
  }

  uint32_t char_start_p, char_size_p;
  ecma_extended_primitive_t *bigint_p = ecma_get_extended_primitive_from_value (value);
  lit_utf8_byte_t *string_buffer_p = ecma_big_uint_to_string (bigint_p, radix, &char_start_p, &char_size_p);

  if (JERRY_UNLIKELY (string_buffer_p == NULL))
  {
    ecma_raise_range_error (ECMA_ERR_ALLOCATE_BIGINT_STRING);
    return NULL;
  }

  JERRY_ASSERT (char_start_p > 0);

  if (bigint_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
  {
    string_buffer_p[--char_start_p] = LIT_CHAR_MINUS;
  }

  ecma_string_t *string_p;
  string_p = ecma_new_ecma_string_from_ascii (string_buffer_p + char_start_p, char_size_p - char_start_p);

  jmem_heap_free_block (string_buffer_p, char_size_p);
  return string_p;
} /* ecma_bigint_to_string */

/**
 * Get the size of zero digits from the result of ecma_bigint_number_to_digits
 */
#define ECMA_BIGINT_NUMBER_TO_DIGITS_GET_ZERO_SIZE(value) (((value) &0xffff) * (uint32_t) sizeof (ecma_bigint_digit_t))

/**
 * Get the number of digits from the result of ecma_bigint_number_to_digits
 */
#define ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS(value) ((value) >> 20)

/**
 * Get the size of digits from the result of ecma_bigint_number_to_digits
 */
#define ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS_SIZE(value) \
  (ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS (value) * (uint32_t) sizeof (ecma_bigint_digit_t))

/**
 * Set number of digits in the result of ecma_bigint_number_to_digits
 */
#define ECMA_BIGINT_NUMBER_TO_DIGITS_SET_DIGITS(value) ((uint32_t) (value) << 20)

/**
 * This flag is set when the number passed to ecma_bigint_number_to_digits has fraction part
 */
#define ECMA_BIGINT_NUMBER_TO_DIGITS_HAS_FRACTION 0x10000

/**
 * Convert a number to maximum of 3 digits and left shift
 *
 * @return packed value, ECMA_BIGINT_NUMBER_TO_DIGITS* macros can be used to decode it
 */
static uint32_t
ecma_bigint_number_to_digits (ecma_number_t number, /**< ecma number */
                              ecma_bigint_digit_t *digits_p) /**< [out] BigInt digits */
{
  if (ecma_number_is_zero (number))
  {
    return ECMA_BIGINT_NUMBER_TO_DIGITS_SET_DIGITS (0);
  }

  ecma_binary_num_t binary = ecma_number_to_binary (number);
  uint32_t biased_exp = ecma_number_biased_exp (binary);
  uint64_t fraction = ecma_number_fraction (binary);

  if (biased_exp < ((1 << (ECMA_NUMBER_BIASED_EXP_WIDTH - 1)) - 1))
  {
    /* Number is less than 1. */
    return ECMA_BIGINT_NUMBER_TO_DIGITS_SET_DIGITS (0) | ECMA_BIGINT_NUMBER_TO_DIGITS_HAS_FRACTION;
  }

  biased_exp -= ((1 << (ECMA_NUMBER_BIASED_EXP_WIDTH - 1)) - 1);
  fraction |= ((uint64_t) 1) << ECMA_NUMBER_FRACTION_WIDTH;

  if (biased_exp <= ECMA_NUMBER_FRACTION_WIDTH)
  {
    uint32_t has_fraction = 0;

    if (biased_exp < ECMA_NUMBER_FRACTION_WIDTH
        && (fraction << (biased_exp + ((8 * sizeof (uint64_t)) - ECMA_NUMBER_FRACTION_WIDTH))) != 0)
    {
      has_fraction |= ECMA_BIGINT_NUMBER_TO_DIGITS_HAS_FRACTION;
    }

    fraction >>= ECMA_NUMBER_FRACTION_WIDTH - biased_exp;
    digits_p[0] = (ecma_bigint_digit_t) fraction;

    return ECMA_BIGINT_NUMBER_TO_DIGITS_SET_DIGITS (1) | has_fraction;
  }

  digits_p[0] = (ecma_bigint_digit_t) fraction;

  biased_exp -= ECMA_NUMBER_FRACTION_WIDTH;

  uint32_t shift_left = biased_exp & ((8 * sizeof (ecma_bigint_digit_t)) - 1);
  biased_exp = biased_exp >> ECMA_BIGINT_DIGIT_SHIFT;

  if (shift_left == 0)
  {
    return biased_exp | ECMA_BIGINT_NUMBER_TO_DIGITS_SET_DIGITS (1);
  }

  uint32_t shift_right = (1 << ECMA_BIGINT_DIGIT_SHIFT) - shift_left;

  digits_p[1] = digits_p[0] >> shift_right;
  digits_p[0] <<= shift_left;

  return biased_exp | ECMA_BIGINT_NUMBER_TO_DIGITS_SET_DIGITS (digits_p[1] == 0 ? 1 : 2);
} /* ecma_bigint_number_to_digits */

/**
 * Convert an ecma number to BigInt value
 *
 * See also:
 *          ECMA-262 v11, 20.2.1.1.1
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_bigint_number_to_bigint (ecma_number_t number) /**< ecma number */
{
  if (ecma_number_is_nan (number) || ecma_number_is_infinity (number))
  {
    return ecma_raise_range_error (ECMA_ERR_INFINITY_OR_NAN_CANNOT_BE_CONVERTED_TO_BIGINT);
  }

  ecma_bigint_digit_t digits[3];
  uint32_t result = ecma_bigint_number_to_digits (number, digits);

  JERRY_ASSERT (ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS (result) == 0
                || digits[ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS (result) - 1] > 0);

  if (result & ECMA_BIGINT_NUMBER_TO_DIGITS_HAS_FRACTION)
  {
    return ecma_raise_range_error (ECMA_ERR_ONLY_INTEGER_NUMBERS_CAN_BE_CONVERTED_TO_BIGINT);
  }

  uint32_t digits_size = ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS_SIZE (result);

  if (digits_size == 0)
  {
    return ECMA_BIGINT_ZERO;
  }

  uint32_t zero_size = ECMA_BIGINT_NUMBER_TO_DIGITS_GET_ZERO_SIZE (result);

  ecma_extended_primitive_t *result_p = ecma_bigint_create (digits_size + zero_size);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  uint8_t *data_p = (uint8_t *) ECMA_BIGINT_GET_DIGITS (result_p, 0);
  memset (data_p, 0, zero_size);
  memcpy (data_p + zero_size, digits, digits_size);

  if (number < 0)
  {
    result_p->u.bigint_sign_and_size |= ECMA_BIGINT_SIGN;
  }

  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_number_to_bigint */

/**
 * Convert a value to BigInt value
 *
 * See also:
 *          ECMA-262 v11, 7.1.13
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_to_bigint (ecma_value_t value, /**< any value */
                       bool allow_numbers) /**< converting ecma numbers is allowed */
{
  bool free_value = false;

  if (ecma_is_value_object (value))
  {
    value = ecma_op_object_default_value (ecma_get_object_from_value (value), ECMA_PREFERRED_TYPE_NUMBER);
    free_value = true;

    if (ECMA_IS_VALUE_ERROR (value))
    {
      return value;
    }
  }

  ecma_value_t result;

  if (ecma_is_value_string (value))
  {
    result = ecma_bigint_parse_string_value (value, ECMA_BIGINT_PARSE_NO_OPTIONS);
  }
  else if (ecma_is_value_bigint (value))
  {
    result = value;

    if (!free_value && value != ECMA_BIGINT_ZERO)
    {
      ecma_ref_extended_primitive (ecma_get_extended_primitive_from_value (value));
    }
    else
    {
      free_value = false;
    }
  }
  else if (allow_numbers && ecma_is_value_number (value))
  {
    result = ecma_bigint_number_to_bigint (ecma_get_number_from_value (value));
  }
  else if (ecma_is_value_false (value))
  {
    result = ECMA_BIGINT_ZERO;
  }
  else if (ecma_is_value_true (value))
  {
    result = ecma_bigint_create_from_digit (1, false);
  }
  else
  {
    result = ecma_raise_type_error (ECMA_ERR_VALUE_CANNOT_BE_CONVERTED_TO_BIGINT);
  }

  if (free_value)
  {
    ecma_free_value (value);
  }

  return result;
} /* ecma_bigint_to_bigint */

/**
 * Convert a BigInt value to number value
 *
 * @return ecma number value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_to_number (ecma_value_t value) /**< BigInt value */
{
  JERRY_ASSERT (ecma_is_value_bigint (value));

  if (value == ECMA_BIGINT_ZERO)
  {
    return ecma_make_integer_value (0);
  }

  ecma_extended_primitive_t *value_p = ecma_get_extended_primitive_from_value (value);
  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);
  ecma_bigint_digit_t *digits_p = ECMA_BIGINT_GET_DIGITS (value_p, size);

  if (size == sizeof (ecma_bigint_digit_t))
  {
    if (!(value_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
    {
      if (digits_p[-1] <= ECMA_INTEGER_NUMBER_MAX)
      {
        return ecma_make_integer_value ((ecma_integer_value_t) digits_p[-1]);
      }
    }
    else if (digits_p[-1] <= -ECMA_INTEGER_NUMBER_MIN)
    {
      return ecma_make_integer_value (-(ecma_integer_value_t) digits_p[-1]);
    }
  }

  uint64_t fraction = 0;
  ecma_bigint_digit_t shift_left;

  if (digits_p[-1] == 1)
  {
    JERRY_ASSERT (size > sizeof (ecma_bigint_digit_t));

    fraction = ((uint64_t) digits_p[-2]) << (8 * sizeof (ecma_bigint_digit_t));
    shift_left = (uint32_t) (8 * sizeof (ecma_bigint_digit_t));

    if (size >= 3 * sizeof (ecma_bigint_digit_t))
    {
      fraction |= (uint64_t) digits_p[-3];
    }
  }
  else
  {
    shift_left = ecma_big_uint_count_leading_zero (digits_p[-1]) + 1;

    fraction = ((uint64_t) digits_p[-1]) << (8 * sizeof (ecma_bigint_digit_t) + shift_left);

    if (size >= 2 * sizeof (ecma_bigint_digit_t))
    {
      fraction |= ((uint64_t) digits_p[-2]) << shift_left;
    }

    if (size >= 3 * sizeof (ecma_bigint_digit_t))
    {
      fraction |= ((uint64_t) digits_p[-3]) >> (8 * sizeof (ecma_bigint_digit_t) - shift_left);
    }
  }

  uint32_t biased_exp = (uint32_t) (((1 << (ECMA_NUMBER_BIASED_EXP_WIDTH - 1)) - 1) + (size * 8 - shift_left));

  /* Rounding result. */
  const uint64_t rounding_bit = (((uint64_t) 1) << (8 * sizeof (uint64_t) - ECMA_NUMBER_FRACTION_WIDTH - 1));
  bool round_up = false;

  if (fraction & rounding_bit)
  {
    round_up = true;

    /* IEEE_754 roundTiesToEven mode: when rounding_bit is set, and all the remaining bits
     * are zero, the number needs to be rounded down the bit before rounding_bit is zero. */
    if ((fraction & ((rounding_bit << 2) - 1)) == rounding_bit)
    {
      round_up = false;

      if ((size >= (3 * sizeof (ecma_bigint_digit_t)))
          && (shift_left == (8 * sizeof (ecma_bigint_digit_t))
              || (digits_p[-3] & ((((ecma_bigint_digit_t) 1) << shift_left) - 1)) == 0))
      {
        ecma_bigint_digit_t *digits_start_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);

        digits_p -= 3;

        while (digits_p > digits_start_p)
        {
          if (digits_p[-1] != 0)
          {
            round_up = true;
            break;
          }
          digits_p--;
        }
      }
    }
  }

  if (round_up)
  {
    fraction += rounding_bit;
    fraction >>= (8 * sizeof (uint64_t) - ECMA_NUMBER_FRACTION_WIDTH);

    if (fraction == 0)
    {
      biased_exp++;
    }
  }
  else
  {
    fraction >>= (8 * sizeof (uint64_t) - ECMA_NUMBER_FRACTION_WIDTH);
  }

  bool sign = (value_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN);
  ecma_number_t result;

  if (biased_exp < (1u << ECMA_NUMBER_BIASED_EXP_WIDTH) - 1)
  {
    result = ecma_number_create (sign, biased_exp, fraction);
  }
  else
  {
    result = ecma_number_make_infinity (sign);
  }

  return ecma_make_number_value (result);
} /* ecma_bigint_to_number */

/**
 * Returns with a BigInt if the value is BigInt,
 * or the value is object, and its default value is BigInt
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_get_bigint (ecma_value_t value, /**< any value */
                        bool *free_result_p) /**< [out] result should be freed */
{
  *free_result_p = false;

  if (ecma_is_value_bigint (value))
  {
    return value;
  }

  if (ecma_is_value_object (value))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (value);
    ecma_value_t default_value = ecma_op_object_default_value (object_p, ECMA_PREFERRED_TYPE_NUMBER);

    if (ECMA_IS_VALUE_ERROR (default_value))
    {
      return default_value;
    }

    if (ecma_is_value_bigint (default_value))
    {
      *free_result_p = (default_value != ECMA_BIGINT_ZERO);
      return default_value;
    }

    ecma_free_value (default_value);
  }

  return ecma_raise_type_error (ECMA_ERR_CONVERT_BIGINT_TO_NUMBER);
} /* ecma_bigint_get_bigint */

/**
 * Create BigInt value from uint64 digits
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_create_from_digits (const uint64_t *digits_p, /**< BigInt digits */
                                uint32_t size, /**< number of BigInt digits */
                                bool sign) /**< sign bit, true if the result should be negative */
{
  const uint64_t *digits_end_p = digits_p + size;

  while (digits_end_p > digits_p && digits_end_p[-1] == 0)
  {
    digits_end_p--;
  }

  if (digits_p == digits_end_p)
  {
    return ECMA_BIGINT_ZERO;
  }

  size = (uint32_t) (digits_end_p - digits_p);

  if (size < ECMA_BIGINT_MAX_SIZE)
  {
    size *= (uint32_t) sizeof (uint64_t);
  }

  if ((digits_end_p[-1] >> (8 * sizeof (ecma_bigint_digit_t))) == 0)
  {
    size -= (uint32_t) sizeof (ecma_bigint_digit_t);
  }

  ecma_extended_primitive_t *result_value_p = ecma_bigint_create (size);

  if (JERRY_UNLIKELY (result_value_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  if (sign)
  {
    result_value_p->u.bigint_sign_and_size |= ECMA_BIGINT_SIGN;
  }

  ecma_bigint_digit_t *result_p = ECMA_BIGINT_GET_DIGITS (result_value_p, 0);

  while (digits_p < digits_end_p)
  {
    uint64_t digit = *digits_p++;

    result_p[0] = (ecma_bigint_digit_t) digit;
    result_p[1] = (ecma_bigint_digit_t) (digit >> (8 * sizeof (ecma_bigint_digit_t)));
    result_p += 2;
  }

  return ecma_make_extended_primitive_value (result_value_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_create_from_digits */

/**
 * Get the number of uint64 digits of a BigInt value
 *
 * @return number of uint64 digits
 */
uint32_t
ecma_bigint_get_size_in_digits (ecma_value_t value) /**< BigInt value */
{
  JERRY_ASSERT (ecma_is_value_bigint (value));

  if (value == ECMA_BIGINT_ZERO)
  {
    return 0;
  }

  ecma_extended_primitive_t *value_p = ecma_get_extended_primitive_from_value (value);
  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);

  return (size + (uint32_t) sizeof (ecma_bigint_digit_t)) / sizeof (uint64_t);
} /* ecma_bigint_get_size_in_digits */

/**
 * Get the uint64 digits of a BigInt value
 */
void
ecma_bigint_get_digits_and_sign (ecma_value_t value, /**< BigInt value */
                                 uint64_t *digits_p, /**< [out] buffer for digits */
                                 uint32_t size, /**< buffer size in digits */
                                 bool *sign_p) /**< [out] sign of BigInt */
{
  JERRY_ASSERT (ecma_is_value_bigint (value));

  if (value == ECMA_BIGINT_ZERO)
  {
    if (sign_p != NULL)
    {
      *sign_p = false;
    }
    memset (digits_p, 0, size * sizeof (uint64_t));
    return;
  }

  ecma_extended_primitive_t *value_p = ecma_get_extended_primitive_from_value (value);

  if (sign_p != NULL)
  {
    *sign_p = (value_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN) != 0;
  }

  uint32_t bigint_size = ECMA_BIGINT_GET_SIZE (value_p);
  uint32_t copy_size = bigint_size / sizeof (uint64_t);

  if (copy_size > size)
  {
    copy_size = size;
  }

  const uint64_t *digits_end_p = digits_p + copy_size;
  ecma_bigint_digit_t *source_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);

  while (digits_p < digits_end_p)
  {
    *digits_p++ = source_p[0] | (((uint64_t) source_p[1]) << (8 * sizeof (ecma_bigint_digit_t)));
    source_p += 2;
  }

  size -= copy_size;

  if (size == 0)
  {
    return;
  }

  if (ECMA_BIGINT_SIZE_IS_ODD (bigint_size))
  {
    *digits_p++ = source_p[0];
    size--;
  }

  if (size > 0)
  {
    memset (digits_p, 0, size * sizeof (uint64_t));
  }
} /* ecma_bigint_get_digits_and_sign */

/**
 * Compare two BigInt values
 *
 * @return true if they are the same, false otherwise
 */
bool
ecma_bigint_is_equal_to_bigint (ecma_value_t left_value, /**< left BigInt value */
                                ecma_value_t right_value) /**< right BigInt value */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value) && ecma_is_value_bigint (right_value));

  if (left_value == ECMA_BIGINT_ZERO)
  {
    return right_value == ECMA_BIGINT_ZERO;
  }
  else if (right_value == ECMA_BIGINT_ZERO)
  {
    return false;
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);

  if (left_p->u.bigint_sign_and_size != right_p->u.bigint_sign_and_size)
  {
    return false;
  }

  uint32_t size = ECMA_BIGINT_GET_SIZE (left_p);
  return memcmp (ECMA_BIGINT_GET_DIGITS (left_p, 0), ECMA_BIGINT_GET_DIGITS (right_p, 0), size) == 0;
} /* ecma_bigint_is_equal_to_bigint */

/**
 * Compare a BigInt value and a number
 *
 * @return true if they are the same, false otherwise
 */
bool
ecma_bigint_is_equal_to_number (ecma_value_t left_value, /**< left BigInt value */
                                ecma_number_t right_value) /**< right number value */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value));

  if (ecma_number_is_nan (right_value) || ecma_number_is_infinity (right_value))
  {
    return false;
  }

  if (left_value == ECMA_BIGINT_ZERO)
  {
    return right_value == 0;
  }

  ecma_extended_primitive_t *left_value_p = ecma_get_extended_primitive_from_value (left_value);

  /* Sign must be the same. */
  if (left_value_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
  {
    if (right_value > 0)
    {
      return false;
    }
  }
  else if (right_value < 0)
  {
    return false;
  }

  ecma_bigint_digit_t digits[3];
  uint32_t result = ecma_bigint_number_to_digits (right_value, digits);

  JERRY_ASSERT (ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS (result) == 0
                || digits[ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS (result) - 1] > 0);

  if (result & ECMA_BIGINT_NUMBER_TO_DIGITS_HAS_FRACTION)
  {
    return false;
  }

  uint32_t digits_size = ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS_SIZE (result);
  uint32_t zero_size = ECMA_BIGINT_NUMBER_TO_DIGITS_GET_ZERO_SIZE (result);

  if (ECMA_BIGINT_GET_SIZE (left_value_p) != digits_size + zero_size)
  {
    return false;
  }

  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);
  ecma_bigint_digit_t *left_end_p = (ecma_bigint_digit_t *) (((uint8_t *) left_p) + zero_size);

  /* Check value bits first. */
  if (memcmp (left_end_p, digits, digits_size) != 0)
  {
    return false;
  }

  while (left_p < left_end_p)
  {
    if (*left_p++ != 0)
    {
      return false;
    }
  }

  return true;
} /* ecma_bigint_is_equal_to_number */

/**
 * Convert 0 to 1, and 1 to -1. Useful for getting sign.
 */
#define ECMA_BIGINT_TO_SIGN(value) (1 - (((int) (value)) << 1))

/**
 * Convert 0 to -1, and 1 to 1. Useful for getting negated sign.
 */
#define ECMA_BIGINT_TO_NEGATED_SIGN(value) (-1 + (((int) (value)) << 1))

/**
 * Compare two BigInt values
 *
 * return -1, if left value < right value, 0 if they are equal, 1 otherwise
 */
int
ecma_bigint_compare_to_bigint (ecma_value_t left_value, /**< left BigInt value */
                               ecma_value_t right_value) /**< right BigInt value */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value) && ecma_is_value_bigint (right_value));

  if (left_value == ECMA_BIGINT_ZERO)
  {
    if (right_value == ECMA_BIGINT_ZERO)
    {
      return 0;
    }

    ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);
    return ECMA_BIGINT_TO_NEGATED_SIGN (right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN);
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  uint32_t left_sign = left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN;

  if (right_value == ECMA_BIGINT_ZERO)
  {
    return ECMA_BIGINT_TO_SIGN (left_sign);
  }

  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);
  uint32_t right_sign = right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN;

  if ((left_sign ^ right_sign) != 0)
  {
    return ECMA_BIGINT_TO_SIGN (left_sign);
  }

  if (left_sign == 0)
  {
    return ecma_big_uint_compare (left_p, right_p);
  }

  return -ecma_big_uint_compare (left_p, right_p);
} /* ecma_bigint_compare_to_bigint */

/**
 * Compare a BigInt value and a number
 *
 * return -1, if left value < right value, 0 if they are equal, 1 otherwise
 */
int
ecma_bigint_compare_to_number (ecma_value_t left_value, /**< left BigInt value */
                               ecma_number_t right_value) /**< right number value */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value));
  JERRY_ASSERT (!ecma_number_is_nan (right_value));

  int right_invert_sign = ECMA_BIGINT_TO_SIGN (right_value > 0);

  if (left_value == ECMA_BIGINT_ZERO)
  {
    if (right_value == 0)
    {
      return 0;
    }

    return right_invert_sign;
  }

  ecma_extended_primitive_t *left_value_p = ecma_get_extended_primitive_from_value (left_value);
  int left_sign = ECMA_BIGINT_TO_SIGN (left_value_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN);

  if (right_value == 0 || left_sign == right_invert_sign)
  {
    /* Second condition: a positive BigInt is always greater than any negative number, and the opposite is true. */
    return left_sign;
  }

  if (ecma_number_is_infinity (right_value))
  {
    /* Infinity is always bigger than any BigInt number. */
    return right_invert_sign;
  }

  ecma_bigint_digit_t digits[3];
  uint32_t result = ecma_bigint_number_to_digits (right_value, digits);

  JERRY_ASSERT (ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS (result) == 0
                || digits[ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS (result) - 1] > 0);

  uint32_t digits_size = ECMA_BIGINT_NUMBER_TO_DIGITS_GET_DIGITS_SIZE (result);

  if (digits_size == 0)
  {
    JERRY_ASSERT (result & ECMA_BIGINT_NUMBER_TO_DIGITS_HAS_FRACTION);
    /* The number is between [-1 .. 1] exclusive. */
    return left_sign;
  }

  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);
  uint32_t right_size = digits_size + ECMA_BIGINT_NUMBER_TO_DIGITS_GET_ZERO_SIZE (result);

  if (left_size != right_size)
  {
    return left_size > right_size ? left_sign : -left_sign;
  }

  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, right_size);
  ecma_bigint_digit_t *left_end_p = (ecma_bigint_digit_t *) (((uint8_t *) left_p) - digits_size);
  ecma_bigint_digit_t *digits_p = (ecma_bigint_digit_t *) (((uint8_t *) digits) + digits_size);

  do
  {
    ecma_bigint_digit_t left = *(--left_p);
    ecma_bigint_digit_t right = *(--digits_p);

    if (left != right)
    {
      return left > right ? left_sign : -left_sign;
    }
  } while (left_p > left_end_p);

  left_end_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);

  while (left_p > left_end_p)
  {
    if (*(--left_p) != 0)
    {
      return left_sign;
    }
  }

  return (result & ECMA_BIGINT_NUMBER_TO_DIGITS_HAS_FRACTION) ? -left_sign : 0;
} /* ecma_bigint_compare_to_number */

#undef ECMA_BIGINT_TO_SIGN

/**
 * Negate a non-zero BigInt value
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_negate (ecma_extended_primitive_t *value_p) /**< BigInt value */
{
  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);

  JERRY_ASSERT (size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (value_p, size) != 0);

  ecma_extended_primitive_t *result_p = ecma_bigint_create (size);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  memcpy (result_p + 1, value_p + 1, size);
  result_p->refs_and_type = ECMA_EXTENDED_PRIMITIVE_REF_ONE | ECMA_TYPE_BIGINT;
  result_p->u.bigint_sign_and_size = value_p->u.bigint_sign_and_size ^ ECMA_BIGINT_SIGN;

  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_negate */

/**
 * Invert all bits of a BigInt value
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_unary (ecma_value_t value, /**< BigInt value */
                   ecma_bigint_unary_operation_type type) /**< type of unary operation */
{
  JERRY_ASSERT (ecma_is_value_bigint (value));

  if (value == ECMA_BIGINT_ZERO)
  {
    return ecma_bigint_create_from_digit (1, type != ECMA_BIGINT_UNARY_INCREASE);
  }

  ecma_extended_primitive_t *value_p = ecma_get_extended_primitive_from_value (value);

  uint32_t sign = (type != ECMA_BIGINT_UNARY_DECREASE) ? ECMA_BIGINT_SIGN : 0;

  if ((value_p->u.bigint_sign_and_size == (uint32_t) (sizeof (ecma_bigint_digit_t) | sign))
      && *ECMA_BIGINT_GET_DIGITS (value_p, 0) == 1)
  {
    return ECMA_BIGINT_ZERO;
  }

  ecma_extended_primitive_t *result_p;

  if ((value_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN) == (sign ^ ECMA_BIGINT_SIGN))
  {
    result_p = ecma_big_uint_increase (value_p);

    if (type != ECMA_BIGINT_UNARY_INCREASE && result_p != NULL)
    {
      result_p->u.bigint_sign_and_size |= ECMA_BIGINT_SIGN;
    }
  }
  else
  {
    result_p = ecma_big_uint_decrease (value_p);

    if (type == ECMA_BIGINT_UNARY_INCREASE && result_p != NULL)
    {
      result_p->u.bigint_sign_and_size |= ECMA_BIGINT_SIGN;
    }
  }

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_unary */

/**
 * Add/subtract right BigInt value to/from left BigInt value
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_add_sub (ecma_value_t left_value, /**< left BigInt value */
                     ecma_value_t right_value, /**< right BigInt value */
                     bool is_add) /**< true if add operation should be performed */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value) && ecma_is_value_bigint (right_value));

  if (right_value == ECMA_BIGINT_ZERO)
  {
    return ecma_copy_value (left_value);
  }

  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);

  if (left_value == ECMA_BIGINT_ZERO)
  {
    if (!is_add)
    {
      return ecma_bigint_negate (right_p);
    }

    ecma_ref_extended_primitive (right_p);
    return right_value;
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  uint32_t sign = is_add ? 0 : ECMA_BIGINT_SIGN;

  if (((left_p->u.bigint_sign_and_size ^ right_p->u.bigint_sign_and_size) & ECMA_BIGINT_SIGN) == sign)
  {
    ecma_extended_primitive_t *result_p = ecma_big_uint_add (left_p, right_p);

    if (JERRY_UNLIKELY (result_p == NULL))
    {
      return ecma_bigint_raise_memory_error ();
    }

    result_p->u.bigint_sign_and_size |= left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN;
    return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
  }

  int compare_result = ecma_big_uint_compare (left_p, right_p);
  ecma_extended_primitive_t *result_p;

  if (compare_result == 0)
  {
    return ECMA_BIGINT_ZERO;
  }

  if (compare_result > 0)
  {
    sign = left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN;
    result_p = ecma_big_uint_sub (left_p, right_p);
  }
  else
  {
    sign = right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN;

    if (!is_add)
    {
      sign ^= ECMA_BIGINT_SIGN;
    }

    result_p = ecma_big_uint_sub (right_p, left_p);
  }

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  result_p->u.bigint_sign_and_size |= sign;
  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_add_sub */

/**
 * Multiply two BigInt values
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_mul (ecma_value_t left_value, /**< left BigInt value */
                 ecma_value_t right_value) /**< right BigInt value */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value) && ecma_is_value_bigint (right_value));

  if (left_value == ECMA_BIGINT_ZERO || right_value == ECMA_BIGINT_ZERO)
  {
    return ECMA_BIGINT_ZERO;
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);
  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_p);
  uint32_t right_size = ECMA_BIGINT_GET_SIZE (right_p);

  if (left_size == sizeof (ecma_bigint_digit_t)
      && ECMA_BIGINT_GET_LAST_DIGIT (left_p, sizeof (ecma_bigint_digit_t)) == 1)
  {
    if (left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
    {
      return ecma_bigint_negate (right_p);
    }

    ecma_ref_extended_primitive (right_p);
    return right_value;
  }

  if (right_size == sizeof (ecma_bigint_digit_t)
      && ECMA_BIGINT_GET_LAST_DIGIT (right_p, sizeof (ecma_bigint_digit_t)) == 1)
  {
    if (right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
    {
      return ecma_bigint_negate (left_p);
    }

    ecma_ref_extended_primitive (left_p);
    return left_value;
  }

  ecma_extended_primitive_t *result_p = ecma_big_uint_mul (left_p, right_p);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  uint32_t sign = (left_p->u.bigint_sign_and_size ^ right_p->u.bigint_sign_and_size) & ECMA_BIGINT_SIGN;
  result_p->u.bigint_sign_and_size |= sign;
  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_mul */

/**
 * Divide two BigInt values
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_div_mod (ecma_value_t left_value, /**< left BigInt value */
                     ecma_value_t right_value, /**< right BigInt value */
                     bool is_mod) /**< true if return with remainder */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value) && ecma_is_value_bigint (right_value));

  if (right_value == ECMA_BIGINT_ZERO)
  {
    return ecma_raise_range_error (ECMA_ERR_BIGINT_ZERO_DIVISION);
  }

  if (left_value == ECMA_BIGINT_ZERO)
  {
    return left_value;
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);

  int compare_result = ecma_big_uint_compare (left_p, right_p);
  ecma_extended_primitive_t *result_p;

  if (compare_result < 0)
  {
    if (!is_mod)
    {
      return ECMA_BIGINT_ZERO;
    }

    ecma_ref_extended_primitive (left_p);
    return left_value;
  }
  else if (compare_result == 0)
  {
    if (is_mod)
    {
      return ECMA_BIGINT_ZERO;
    }

    result_p = ecma_bigint_create (sizeof (ecma_bigint_digit_t));

    if (result_p != NULL)
    {
      *ECMA_BIGINT_GET_DIGITS (result_p, 0) = 1;
    }
  }
  else
  {
    result_p = ecma_big_uint_div_mod (left_p, right_p, is_mod);

    if (result_p == ECMA_BIGINT_POINTER_TO_ZERO)
    {
      return ECMA_BIGINT_ZERO;
    }
  }

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  if (is_mod)
  {
    result_p->u.bigint_sign_and_size |= left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN;
  }
  else
  {
    uint32_t sign = (left_p->u.bigint_sign_and_size ^ right_p->u.bigint_sign_and_size) & ECMA_BIGINT_SIGN;
    result_p->u.bigint_sign_and_size |= sign;
  }

  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_div_mod */

/**
 * Shift left BigInt value to left or right
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_shift (ecma_value_t left_value, /**< left BigInt value */
                   ecma_value_t right_value, /**< right BigInt value */
                   bool is_left) /**< true if left shift operation should be performed */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value) && ecma_is_value_bigint (right_value));

  if (left_value == ECMA_BIGINT_ZERO)
  {
    return ECMA_BIGINT_ZERO;
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);

  if (right_value == ECMA_BIGINT_ZERO)
  {
    ecma_ref_extended_primitive (left_p);
    return left_value;
  }

  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);

  if (right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
  {
    is_left = !is_left;
  }

  if (ECMA_BIGINT_GET_SIZE (right_p) > sizeof (ecma_bigint_digit_t))
  {
    if (is_left)
    {
      return ecma_bigint_raise_memory_error ();
    }
    else if (left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
    {
      /* Shifting a negative value with a very big number to the right should be -1. */
      return ecma_bigint_create_from_digit (1, true);
    }

    return ECMA_BIGINT_ZERO;
  }

  ecma_extended_primitive_t *result_p;
  ecma_bigint_digit_t shift = ECMA_BIGINT_GET_LAST_DIGIT (right_p, sizeof (ecma_bigint_digit_t));
  uint32_t left_sign = left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN;

  if (is_left)
  {
    result_p = ecma_big_uint_shift_left (left_p, shift);
  }
  else
  {
    /* -x >> y == ~(x - 1) >> y == ~((x - 1) >> y) == -(((x - 1) >> y) + 1)
     * When a non-zero bit is shifted out: (x - 1) >> y == x >> y, so the formula is -((x >> y) + 1)
     * When only zero bits are shifted out: (((x - 1) >> y) + 1) == x >> y so the formula is: -(x >> y) */
    result_p = ecma_big_uint_shift_right (left_p, shift, left_sign != 0);

    if (result_p == ECMA_BIGINT_POINTER_TO_ZERO)
    {
      return ECMA_BIGINT_ZERO;
    }
  }

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  result_p->u.bigint_sign_and_size |= left_sign;
  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_shift */

/**
 * Compute the left value raised to the power of right value
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_pow (ecma_value_t left_value, /**< left BigInt value */
                 ecma_value_t right_value) /**< right BigInt value */
{
  JERRY_ASSERT (ecma_is_value_bigint (left_value) && ecma_is_value_bigint (right_value));

  if (right_value == ECMA_BIGINT_ZERO)
  {
    return ecma_bigint_create_from_digit (1, false);
  }

  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);

  if (right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
  {
    return ecma_raise_range_error (ECMA_ERR_NEGATIVE_EXPONENT_IS_NOT_ALLOWED_FOR_BIGINTS);
  }

  if (left_value == ECMA_BIGINT_ZERO)
  {
    return ECMA_BIGINT_ZERO;
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  ecma_bigint_digit_t base = 0;

  if (ECMA_BIGINT_GET_SIZE (left_p) == sizeof (ecma_bigint_digit_t))
  {
    base = *ECMA_BIGINT_GET_DIGITS (left_p, 0);

    JERRY_ASSERT (base != 0);

    if (base == 1)
    {
      if (!(left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
          || ECMA_BIGINT_NUMBER_IS_ODD (*ECMA_BIGINT_GET_DIGITS (right_p, 0)))
      {
        ecma_ref_extended_primitive (left_p);
        return left_value;
      }

      return ecma_bigint_create_from_digit (1, false);
    }
  }

  if (JERRY_UNLIKELY (ECMA_BIGINT_GET_SIZE (right_p) > sizeof (ecma_bigint_digit_t)))
  {
    return ecma_bigint_raise_memory_error ();
  }

  ecma_bigint_digit_t power = *ECMA_BIGINT_GET_DIGITS (right_p, 0);

  if (power == 1)
  {
    ecma_ref_extended_primitive (left_p);
    return left_value;
  }

  ecma_extended_primitive_t *result_p;

  if (base == 2)
  {
    result_p = ecma_big_uint_shift_left (left_p, power - 1);
  }
  else
  {
    result_p = ecma_big_uint_pow (left_p, power);
  }

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  if ((left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN) && ECMA_BIGINT_NUMBER_IS_ODD (power))
  {
    result_p->u.bigint_sign_and_size |= ECMA_BIGINT_SIGN;
  }

  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_pow */

/**
 * Convert the result to an ecma value
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_bigint_bitwise_op (uint32_t operation_and_options, /**< bitwise operation type and options */
                        ecma_extended_primitive_t *left_value_p, /**< left BigInt value */
                        ecma_extended_primitive_t *right_value_p) /**< right BigInt value */
{
  ecma_extended_primitive_t *result_p;
  result_p = ecma_big_uint_bitwise_op (operation_and_options, left_value_p, right_value_p);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return ecma_bigint_raise_memory_error ();
  }

  if (result_p == ECMA_BIGINT_POINTER_TO_ZERO)
  {
    return ECMA_BIGINT_ZERO;
  }

  if (operation_and_options & ECMA_BIG_UINT_BITWISE_INCREASE_RESULT)
  {
    result_p->u.bigint_sign_and_size |= ECMA_BIGINT_SIGN;
  }

  return ecma_make_extended_primitive_value (result_p, ECMA_TYPE_BIGINT);
} /* ecma_bigint_bitwise_op */

/**
 * Perform bitwise 'and' operations on two BigUInt numbers
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_and (ecma_value_t left_value, /**< left BigInt value */
                 ecma_value_t right_value) /**< right BigInt value */
{
  if (left_value == ECMA_BIGINT_ZERO || right_value == ECMA_BIGINT_ZERO)
  {
    return ECMA_BIGINT_ZERO;
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);

  if (!(left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
  {
    if (!(right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
    {
      return ecma_bigint_bitwise_op (ECMA_BIG_UINT_BITWISE_AND, left_p, right_p);
    }

    /* x & (-y) == x & ~(y-1) == x &~ (y-1) */
    uint32_t operation_and_options = ECMA_BIG_UINT_BITWISE_AND_NOT | ECMA_BIG_UINT_BITWISE_DECREASE_RIGHT;
    return ecma_bigint_bitwise_op (operation_and_options, left_p, right_p);
  }

  if (!(right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
  {
    /* (-x) & y == ~(x-1) & y == y &~ (x-1) */
    uint32_t operation_and_options = ECMA_BIG_UINT_BITWISE_AND_NOT | ECMA_BIG_UINT_BITWISE_DECREASE_RIGHT;
    return ecma_bigint_bitwise_op (operation_and_options, right_p, left_p);
  }

  /* (-x) & (-y) == ~(x-1) & ~(y-1) == ~((x-1) | (y-1)) == -(((x-1) | (y-1)) + 1) */
  uint32_t operation_and_options =
    (ECMA_BIG_UINT_BITWISE_OR | ECMA_BIG_UINT_BITWISE_DECREASE_BOTH | ECMA_BIG_UINT_BITWISE_INCREASE_RESULT);
  return ecma_bigint_bitwise_op (operation_and_options, left_p, right_p);
} /* ecma_bigint_and */

/**
 * Perform bitwise 'or' operations on two BigUInt numbers
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_or (ecma_value_t left_value, /**< left BigInt value */
                ecma_value_t right_value) /**< right BigInt value */
{
  if (left_value == ECMA_BIGINT_ZERO)
  {
    return ecma_copy_value (right_value);
  }

  if (right_value == ECMA_BIGINT_ZERO)
  {
    return ecma_copy_value (left_value);
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);

  if (!(left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
  {
    if (!(right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
    {
      return ecma_bigint_bitwise_op (ECMA_BIG_UINT_BITWISE_OR, left_p, right_p);
    }

    /* x | (-y) == x | ~(y-1) == ~((y-1) &~ x) == -(((y-1) &~ x) + 1) */
    uint32_t operation_and_options =
      (ECMA_BIG_UINT_BITWISE_AND_NOT | ECMA_BIG_UINT_BITWISE_DECREASE_LEFT | ECMA_BIG_UINT_BITWISE_INCREASE_RESULT);
    return ecma_bigint_bitwise_op (operation_and_options, right_p, left_p);
  }

  if (!(right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
  {
    /* (-x) | y == ~(x-1) | y == ~((x-1) &~ y) == -(((x-1) &~ y) + 1) */
    uint32_t operation_and_options =
      (ECMA_BIG_UINT_BITWISE_AND_NOT | ECMA_BIG_UINT_BITWISE_DECREASE_LEFT | ECMA_BIG_UINT_BITWISE_INCREASE_RESULT);
    return ecma_bigint_bitwise_op (operation_and_options, left_p, right_p);
  }

  /* (-x) | (-y) == ~(x-1) | ~(y-1) == ~((x-1) & (y-1)) = -(((x-1) & (y-1)) + 1) */
  uint32_t operation_and_options =
    (ECMA_BIG_UINT_BITWISE_AND | ECMA_BIG_UINT_BITWISE_DECREASE_BOTH | ECMA_BIG_UINT_BITWISE_INCREASE_RESULT);
  return ecma_bigint_bitwise_op (operation_and_options, left_p, right_p);
} /* ecma_bigint_or */

/**
 * Perform bitwise 'xor' operations on two BigUInt numbers
 *
 * @return ecma BigInt value or ECMA_VALUE_ERROR
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_bigint_xor (ecma_value_t left_value, /**< left BigInt value */
                 ecma_value_t right_value) /**< right BigInt value */
{
  if (left_value == ECMA_BIGINT_ZERO)
  {
    return ecma_copy_value (right_value);
  }

  if (right_value == ECMA_BIGINT_ZERO)
  {
    return ecma_copy_value (left_value);
  }

  ecma_extended_primitive_t *left_p = ecma_get_extended_primitive_from_value (left_value);
  ecma_extended_primitive_t *right_p = ecma_get_extended_primitive_from_value (right_value);

  if (!(left_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
  {
    if (!(right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
    {
      return ecma_bigint_bitwise_op (ECMA_BIG_UINT_BITWISE_XOR, left_p, right_p);
    }

    /* x ^ (-y) == x ^ ~(y-1) == ~(x ^ (y-1)) == -((x ^ (y-1)) + 1) */
    uint32_t operation_and_options =
      (ECMA_BIG_UINT_BITWISE_XOR | ECMA_BIG_UINT_BITWISE_DECREASE_RIGHT | ECMA_BIG_UINT_BITWISE_INCREASE_RESULT);
    return ecma_bigint_bitwise_op (operation_and_options, left_p, right_p);
  }

  if (!(right_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN))
  {
    /* (-x) | y == ~(x-1) ^ y == ~((x-1) ^ y) == -(((x-1) ^ y) + 1) */
    uint32_t operation_and_options =
      (ECMA_BIG_UINT_BITWISE_XOR | ECMA_BIG_UINT_BITWISE_DECREASE_LEFT | ECMA_BIG_UINT_BITWISE_INCREASE_RESULT);
    return ecma_bigint_bitwise_op (operation_and_options, left_p, right_p);
  }

  /* (-x) ^ (-y) == ~(x-1) ^ ~(y-1) == (x-1) ^ (y-1) */
  uint32_t operation_and_options = ECMA_BIG_UINT_BITWISE_XOR | ECMA_BIG_UINT_BITWISE_DECREASE_BOTH;
  return ecma_bigint_bitwise_op (operation_and_options, left_p, right_p);
} /* ecma_bigint_xor */

#endif /* JERRY_BUILTIN_BIGINT */
