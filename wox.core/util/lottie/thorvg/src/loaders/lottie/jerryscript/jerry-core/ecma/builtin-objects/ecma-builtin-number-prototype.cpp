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

#include <math.h>

#include "ecma-alloc.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers-number.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-string-object.h"

#include "jrt-libc-includes.h"
#include "jrt.h"
#include "lit-char-helpers.h"

#if JERRY_BUILTIN_NUMBER

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

/**
 * This object has a custom dispatch function.
 */
#define BUILTIN_CUSTOM_DISPATCH

/**
 * List of built-in routine identifiers.
 */
enum
{
  ECMA_NUMBER_PROTOTYPE_ROUTINE_START = 0,
  ECMA_NUMBER_PROTOTYPE_VALUE_OF,
  ECMA_NUMBER_PROTOTYPE_TO_STRING,
  ECMA_NUMBER_PROTOTYPE_TO_LOCALE_STRING,
  ECMA_NUMBER_PROTOTYPE_TO_FIXED,
  ECMA_NUMBER_PROTOTYPE_TO_EXPONENTIAL,
  ECMA_NUMBER_PROTOTYPE_TO_PRECISION,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-number-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  number_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup numberprototype ECMA Number.prototype object built-in
 * @{
 */

/**
 * Helper for rounding numbers
 *
 * @return rounded number
 */
static inline lit_utf8_size_t 
ecma_builtin_number_prototype_helper_round (lit_utf8_byte_t *digits_p, /**< [in,out] number as a string in decimal
                                                                        *   form */
                                            lit_utf8_size_t num_digits, /**< length of the string representation */
                                            int32_t round_num, /**< number of digits to keep */
                                            int32_t *exponent_p, /**< [in, out] decimal exponent */
                                            bool zero) /**< true if digits_p represents zero */
{
  if (round_num == 0 && *exponent_p == 0)
  {
    if (digits_p[0] >= '5')
    {
      digits_p[0] = '1';
    }
    else
    {
      digits_p[0] = '0';
    }

    return 1;
  }

  if (round_num < 1)
  {
    return 0;
  }

  if ((lit_utf8_size_t) round_num >= num_digits || zero)
  {
    return num_digits;
  }

  if (digits_p[round_num] >= '5')
  {
    digits_p[round_num] = '0';

    int i = 1;

    /* Handle carry number. */
    for (; i <= round_num; i++)
    {
      if (++digits_p[round_num - i] <= '9')
      {
        break;
      }
      digits_p[round_num - i] = '0';
    }

    /* Prepend highest digit */
    if (i > round_num)
    {
      memmove (digits_p + 1, digits_p, num_digits);
      digits_p[0] = '1';
      *exponent_p += 1;
    }
  }

  return (lit_utf8_size_t) round_num;
} /* ecma_builtin_number_prototype_helper_round */

/**
 * Size of Number toString digit buffers.
 */
#define NUMBER_TO_STRING_MAX_DIGIT_COUNT 64u

/**
 * The Number.prototype object's 'toString' and 'toLocaleString' routines
 *
 * See also:
 *          ECMA-262 v5, 15.7.4.2
 *          ECMA-262 v5, 15.7.4.7
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_number_prototype_object_to_string (ecma_number_t this_arg_number, /**< this argument number */
                                                const ecma_value_t *arguments_list_p, /**< arguments list */
                                                uint32_t arguments_list_len) /**< number of arguments */
{
  static const lit_utf8_byte_t digit_chars[36] = { '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b',
                                                   'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n',
                                                   'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z' };

  uint32_t radix = 10;
  if (arguments_list_len > 0 && !ecma_is_value_undefined (arguments_list_p[0]))
  {
    ecma_number_t arg_num;

    if (ECMA_IS_VALUE_ERROR (ecma_op_to_integer (arguments_list_p[0], &arg_num)))
    {
      return ECMA_VALUE_ERROR;
    }

    radix = ecma_number_to_uint32 (arg_num);

    if (radix < 2 || radix > 36)
    {
      return ecma_raise_range_error (ECMA_ERR_RADIX_IS_OUT_OF_RANGE);
    }
  }

  if (ecma_number_is_nan (this_arg_number) || ecma_number_is_infinity (this_arg_number)
      || ecma_number_is_zero (this_arg_number) || radix == 10)
  {
    ecma_string_t *ret_str_p = ecma_new_ecma_string_from_number (this_arg_number);
    return ecma_make_string_value (ret_str_p);
  }

  uint8_t integer_digits[NUMBER_TO_STRING_MAX_DIGIT_COUNT];
  uint8_t fraction_digits[NUMBER_TO_STRING_MAX_DIGIT_COUNT];
  uint32_t integer_zeros = 0;
  uint32_t fraction_zeros = 0;
  bool is_number_negative = false;

  if (ecma_number_is_negative (this_arg_number))
  {
    this_arg_number = -this_arg_number;
    is_number_negative = true;
  }

  ecma_number_t integer_part = floor (this_arg_number);
  ecma_number_t fraction_part = this_arg_number - integer_part;

  uint8_t *integer_cursor_p = integer_digits + NUMBER_TO_STRING_MAX_DIGIT_COUNT;
  uint8_t *fraction_cursor_p = fraction_digits;

  if (fraction_part > (ecma_number_t) 0.0f)
  {
    uint8_t digit;
    ecma_number_t precision = (ecma_number_get_next (this_arg_number) - this_arg_number) * (ecma_number_t) 0.5f;
    precision = JERRY_MAX (precision, ECMA_NUMBER_MIN_VALUE);

    do
    {
      fraction_part *= radix;
      precision *= radix;

      digit = (uint8_t) floor (fraction_part);

      if (digit == 0 && fraction_cursor_p == fraction_digits)
      {
        fraction_zeros++;
        continue;
      }

      JERRY_ASSERT (fraction_cursor_p < fraction_digits + NUMBER_TO_STRING_MAX_DIGIT_COUNT);
      *fraction_cursor_p++ = digit;
      fraction_part -= (ecma_number_t) digit;
    } while (fraction_part >= precision);

    /* Round to even */
    if (fraction_part > (ecma_number_t) 0.5f || (fraction_part == (ecma_number_t) 0.5f && (digit & 1) != 0))
    {
      /* Add carry and remove overflowing trailing digits */
      while (true)
      {
        (*(--fraction_cursor_p))++;

        if (*fraction_cursor_p < radix)
        {
          /* Re-adjust cursor to point after the last significant digit */
          fraction_cursor_p++;
          break;
        }

        if (fraction_cursor_p == fraction_digits)
        {
          /* Carry overflowed to integer part */
          integer_part += 1;
          break;
        }
      }
    }

    /* Convert fraction digits to characters. */
    for (uint8_t *digit_p = fraction_digits; digit_p < fraction_cursor_p; digit_p++)
    {
      *digit_p = digit_chars[*digit_p];
    }
  }

  while (ecma_number_biased_exp (ecma_number_to_binary (integer_part / radix))
         > ECMA_NUMBER_EXPONENT_BIAS + ECMA_NUMBER_FRACTION_WIDTH)
  {
    integer_zeros++;
    integer_part /= radix;
  }

  uint64_t integer_u64 = (uint64_t) integer_part;

  do
  {
    uint64_t remainder = integer_u64 % radix;
    *(--integer_cursor_p) = (uint8_t) digit_chars[remainder];

    integer_u64 /= radix;
  } while (integer_u64 > 0);

  const uint32_t integer_digit_count =
    (uint32_t) (integer_digits + NUMBER_TO_STRING_MAX_DIGIT_COUNT - integer_cursor_p);
  JERRY_ASSERT (integer_digit_count > 0);

  ecma_stringbuilder_t builder = ecma_stringbuilder_create ();

  if (is_number_negative)
  {
    ecma_stringbuilder_append_byte (&builder, LIT_CHAR_MINUS);
  }

  ecma_stringbuilder_append_raw (&builder, integer_cursor_p, integer_digit_count);

  while (integer_zeros--)
  {
    ecma_stringbuilder_append_byte (&builder, LIT_CHAR_0);
  }

  if (fraction_cursor_p != fraction_digits)
  {
    ecma_stringbuilder_append_byte (&builder, LIT_CHAR_DOT);

    while (fraction_zeros--)
    {
      ecma_stringbuilder_append_byte (&builder, LIT_CHAR_0);
    }

    const uint32_t fraction_digit_count = (uint32_t) (fraction_cursor_p - fraction_digits);
    JERRY_ASSERT (fraction_digit_count > 0);

    ecma_stringbuilder_append_raw (&builder, fraction_digits, fraction_digit_count);
  }

  return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
} /* ecma_builtin_number_prototype_object_to_string */

/**
 * The Number.prototype object's 'valueOf' routine
 *
 * See also:
 *          ECMA-262 v5, 15.7.4.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_number_prototype_object_value_of (ecma_value_t this_arg) /**< this argument */
{
  if (ecma_is_value_number (this_arg))
  {
    return this_arg;
  }
  else if (ecma_is_value_object (this_arg))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (this_arg);

    if (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_NUMBER))
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      JERRY_ASSERT (ecma_is_value_number (ext_object_p->u.cls.u3.value));

      return ext_object_p->u.cls.u3.value;
    }
  }

  return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_NUMBER);
} /* ecma_builtin_number_prototype_object_value_of */

/**
 * Type of number routine
 */
typedef enum
{
  NUMBER_ROUTINE_TO_FIXED, /**< Number.prototype.toFixed: ECMA-262 v11, 20.1.3.3 */
  NUMBER_ROUTINE_TO_EXPONENTIAL, /**< Number.prototype.toExponential: ECMA-262 v11, 20.1.3.2 */
  NUMBER_ROUTINE_TO_PRECISION, /**< Number.prototype.toPrecision: ECMA-262 v11, 20.1.3.5 */
  NUMBER_ROUTINE__COUNT, /**< count of the modes */
} number_routine_mode_t;

/**
 * Helper method to convert a number based on the given routine.
 */
static ecma_value_t
ecma_builtin_number_prototype_object_to_number_convert (ecma_number_t this_num, /**< this argument number */
                                                        ecma_value_t arg, /**< routine's argument */
                                                        number_routine_mode_t mode) /**< number routine mode */
{
  if (ecma_is_value_undefined (arg) && mode == NUMBER_ROUTINE_TO_PRECISION)
  {
    return ecma_builtin_number_prototype_object_to_string (this_num, NULL, 0);
  }

  ecma_number_t arg_num;
  ecma_value_t to_integer = ecma_op_to_integer (arg, &arg_num);

  if (ECMA_IS_VALUE_ERROR (to_integer))
  {
    return to_integer;
  }

  /* Argument boundary check for toFixed method */
  if (mode == NUMBER_ROUTINE_TO_FIXED && (arg_num <= -1 || arg_num >= 101))
  {
    return ecma_raise_range_error (ECMA_ERR_FRACTION_DIGITS_OUT_OF_RANGE);
  }

  /* Handle NaN separately */
  if (ecma_number_is_nan (this_num))
  {
    return ecma_make_magic_string_value (LIT_MAGIC_STRING_NAN);
  }

  /* Get the parameters of the number */
  lit_utf8_byte_t digits[ECMA_MAX_CHARS_IN_STRINGIFIED_NUMBER];
  lit_utf8_size_t num_of_digits;
  int32_t exponent;
  int32_t arg_int = ecma_number_to_int32 (arg_num);
  bool is_zero = ecma_number_is_zero (this_num);
  bool is_negative = ecma_number_is_negative (this_num);

  ecma_stringbuilder_t builder = ecma_stringbuilder_create ();

  if (is_negative)
  {
    if (!is_zero)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_MINUS);
    }

    this_num *= -1;
  }

  /* Handle zero separately */
  if (is_zero)
  {
    if (mode == NUMBER_ROUTINE_TO_PRECISION)
    {
      arg_int--;
    }

    ecma_stringbuilder_append_char (&builder, LIT_CHAR_0);

    if (arg_int > 0)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_DOT);
    }

    for (int32_t i = 0; i < arg_int; i++)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_0);
    }

    if (mode == NUMBER_ROUTINE_TO_EXPONENTIAL)
    {
      ecma_stringbuilder_append_raw (&builder, (const lit_utf8_byte_t *) "e+0", 3);
    }

    return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
  }

  /* Handle infinity separately */
  if (ecma_number_is_infinity (this_num))
  {
    ecma_stringbuilder_append_magic (&builder, LIT_MAGIC_STRING_INFINITY_UL);
    return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
  }

  /* Argument boundary check for toExponential and toPrecision methods */
  if (mode == NUMBER_ROUTINE_TO_EXPONENTIAL && (arg_num <= -1 || arg_num >= 101))
  {
    ecma_stringbuilder_destroy (&builder);
    return ecma_raise_range_error (ECMA_ERR_FRACTION_DIGITS_OUT_OF_RANGE);
  }
  else if (mode == NUMBER_ROUTINE_TO_PRECISION && (arg_num < 1 || arg_num > 100))
  {
    ecma_stringbuilder_destroy (&builder);
    return ecma_raise_range_error (ECMA_ERR_PRECISION_DIGITS_MUST_BE_BETWEEN_IN_RANGE);
  }

  num_of_digits = ecma_number_to_decimal (this_num, digits, &exponent);

  /* Handle undefined argument */
  if (ecma_is_value_undefined (arg) && mode == NUMBER_ROUTINE_TO_EXPONENTIAL)
  {
    arg_int = (int32_t) num_of_digits - 1;
  }

  if (mode == NUMBER_ROUTINE_TO_FIXED && exponent > 21)
  {
    ecma_stringbuilder_destroy (&builder);

    if (is_negative)
    {
      this_num *= -1;
    }

    return ecma_builtin_number_prototype_object_to_string (this_num, NULL, 0);
  }

  int32_t digits_to_keep = arg_int;

  if (mode == NUMBER_ROUTINE_TO_FIXED)
  {
    digits_to_keep += exponent;
  }
  else if (mode == NUMBER_ROUTINE_TO_EXPONENTIAL)
  {
    digits_to_keep += 1;
  }

  num_of_digits = ecma_builtin_number_prototype_helper_round (digits, num_of_digits, digits_to_keep, &exponent, false);

  /* toExponent routine and toPrecision cases where the exponent > precision or exponent < -5 */
  if (mode == NUMBER_ROUTINE_TO_EXPONENTIAL
      || (mode == NUMBER_ROUTINE_TO_PRECISION && (exponent < -5 || exponent > arg_int)))
  {
    /* Append first digit */
    ecma_stringbuilder_append_byte (&builder, *digits);

    if (mode == NUMBER_ROUTINE_TO_PRECISION)
    {
      arg_int--;
    }

    if (arg_int > 0)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_DOT);
    }

    /* Append significant fraction digits */
    ecma_stringbuilder_append_raw (&builder, digits + 1, num_of_digits - 1);

    /* Append leading zeros */
    for (int32_t i = (int32_t) (num_of_digits); i < arg_int + 1; i++)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_0);
    }

    ecma_stringbuilder_append_char (&builder, LIT_CHAR_LOWERCASE_E);

    if (exponent <= 0)
    {
      exponent = (-exponent) + 1;
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_MINUS);
    }
    else
    {
      exponent -= 1;
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_PLUS);
    }

    /* Append exponent part */
    lit_utf8_size_t exp_size = ecma_uint32_to_utf8_string ((uint32_t) exponent, digits, 3);
    ecma_stringbuilder_append_raw (&builder, digits, exp_size);

    return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
  }

  /* toFixed routine and toPrecision cases where the exponent <= precision and exponent >= -5 */
  lit_utf8_size_t result_digits;

  if (mode == NUMBER_ROUTINE_TO_FIXED)
  {
    result_digits = ((exponent > 0) ? (lit_utf8_size_t) (exponent + arg_int) : (lit_utf8_size_t) (arg_int + 1));
  }
  else
  {
    result_digits = ((exponent <= 0) ? (lit_utf8_size_t) (1 - exponent + arg_int) : (lit_utf8_size_t) arg_int);
  }

  /* Number of digits we copied from digits array */
  lit_utf8_size_t copied_digits = 0;

  if (exponent == 0 && digits_to_keep == 0)
  {
    ecma_stringbuilder_append_char (&builder, *digits);
    return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
  }

  if (exponent <= 0)
  {
    ecma_stringbuilder_append_char (&builder, LIT_CHAR_0);
    result_digits--;

    if (result_digits > 0)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_DOT);

      /* Append leading zeros to the fraction part */
      for (int32_t i = 0; i < -exponent && result_digits > 0; i++)
      {
        ecma_stringbuilder_append_char (&builder, LIT_CHAR_0);
        result_digits--;
      }
    }
  }
  else
  {
    /* Append significant digits of integer part */
    copied_digits = JERRY_MIN (JERRY_MIN (num_of_digits, result_digits), (lit_utf8_size_t) exponent);
    ecma_stringbuilder_append_raw (&builder, digits, copied_digits);

    result_digits -= copied_digits;
    num_of_digits -= copied_digits;
    exponent -= (int32_t) copied_digits;

    /* Append zeros before decimal point */
    while (exponent > 0 && result_digits > 0)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_0);
      result_digits--;
      exponent--;
    }

    if (result_digits > 0)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_DOT);
    }
  }

  if (result_digits > 0)
  {
    /* Append significant digits to the fraction part */
    lit_utf8_size_t to_copy = JERRY_MIN (num_of_digits, result_digits);
    ecma_stringbuilder_append_raw (&builder, digits + copied_digits, to_copy);
    result_digits -= to_copy;

    /* Append leading zeros */
    while (result_digits > 0)
    {
      ecma_stringbuilder_append_char (&builder, LIT_CHAR_0);
      result_digits--;
    }
  }

  return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
} /* ecma_builtin_number_prototype_object_to_number_convert */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_number_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                                ecma_value_t this_arg, /**< 'this' argument value */
                                                const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                        *   passed to routine */
                                                uint32_t arguments_number) /**< length of arguments' list */
{
  ecma_value_t this_value = ecma_builtin_number_prototype_object_value_of (this_arg);

  if (ECMA_IS_VALUE_ERROR (this_value))
  {
    return this_value;
  }

  if (builtin_routine_id == ECMA_NUMBER_PROTOTYPE_VALUE_OF)
  {
    return ecma_copy_value (this_value);
  }

  ecma_number_t this_arg_number = ecma_get_number_from_value (this_value);

  switch (builtin_routine_id)
  {
    case ECMA_NUMBER_PROTOTYPE_TO_STRING:
    {
      return ecma_builtin_number_prototype_object_to_string (this_arg_number, arguments_list_p, arguments_number);
    }
    case ECMA_NUMBER_PROTOTYPE_TO_LOCALE_STRING:
    {
      return ecma_builtin_number_prototype_object_to_string (this_arg_number, NULL, 0);
    }
    case ECMA_NUMBER_PROTOTYPE_TO_FIXED:
    case ECMA_NUMBER_PROTOTYPE_TO_EXPONENTIAL:
    case ECMA_NUMBER_PROTOTYPE_TO_PRECISION:
    {
      const int option = NUMBER_ROUTINE_TO_FIXED + (builtin_routine_id - ECMA_NUMBER_PROTOTYPE_TO_FIXED);
      return ecma_builtin_number_prototype_object_to_number_convert (this_arg_number,
                                                                     arguments_list_p[0],
                                                                     (number_routine_mode_t) option);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_number_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_NUMBER */
