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
#include "ecma-builtin-helpers.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"

#include "jcontext.h"
#include "lit-char-helpers.h"

#if JERRY_BUILTIN_DATE

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
  ECMA_DATE_ROUTINE_START = 0,
  ECMA_DATE_ROUTINE_PARSE,
  ECMA_DATE_ROUTINE_UTC,
  ECMA_DATE_ROUTINE_NOW,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-date.inc.h"
#define BUILTIN_UNDERSCORED_ID  date
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup date ECMA Date object built-in
 * @{
 */

/**
 * Encode minimum/maximum limits
 *
 * See: ecma_date_parse_date_chars
 *
 * @param min: 8 bits unsigned number
 * @param max: 24 bits unsigned number
 */
#define ECMA_DATE_LIMIT(min, max) (min << 24 | max)

/**
 * Decode the minimum value from the encoded limit
 */
#define ECMA_DATE_LIMIT_MIN(limit) (limit >> 24)

/**
 * Decode the maximum value from the encoded limit
 */
#define ECMA_DATE_LIMIT_MAX(limit) (limit & ((1 << 24) - 1))

/**
 * Helper function to try to parse a part of a date string
 *
 * @return NaN if cannot read from string, ToNumber() otherwise
 */
static ecma_number_t
ecma_date_parse_date_chars (const lit_utf8_byte_t **str_p, /**< pointer to the cesu8 string */
                            const lit_utf8_byte_t *str_end_p, /**< pointer to the end of the string */
                            uint32_t num_of_chars, /**< number of characters to read and convert */
                            uint32_t limit) /**< minimum/maximum valid value */
{
  JERRY_ASSERT (num_of_chars > 0 && num_of_chars <= 6);

  if (*str_p + num_of_chars > str_end_p)
  {
    return ecma_number_make_nan ();
  }

  str_end_p = *str_p + num_of_chars;

  uint32_t num = 0;

  while (num_of_chars--)
  {
    lit_utf8_byte_t c = **str_p;
    if (!lit_char_is_decimal_digit (c))
    {
      return ecma_number_make_nan ();
    }

    num = (num * 10) + (uint32_t) ((c - LIT_CHAR_0));
    (*str_p)++;
  }

  if (num >= ECMA_DATE_LIMIT_MIN (limit) && num <= ECMA_DATE_LIMIT_MAX (limit))
  {
    return (ecma_number_t) num;
  }

  return ecma_number_make_nan ();
} /* ecma_date_parse_date_chars */

/**
 * Helper function to try to parse a special character (+,-,T,Z,:,.) in a date string
 *
 * @return true if the first character is same as the expected, false otherwise
 */
static bool
ecma_date_parse_special_char (const lit_utf8_byte_t **str_p, /**< pointer to the cesu8 string */
                              const lit_utf8_byte_t *str_end_p, /**< pointer to the end of the string */
                              ecma_char_t expected_char) /**< expected character */
{
  if ((*str_p < str_end_p) && (**str_p == expected_char))
  {
    (*str_p)++;
    return true;
  }

  return false;
} /* ecma_date_parse_special_char */

static inline bool
ecma_date_check_two_chars (const lit_utf8_byte_t *str_p, /**< pointer to the cesu8 string */
                           const lit_utf8_byte_t *str_end_p, /**< pointer to the end of the string */
                           ecma_char_t expected_char1, /**< first expected character */
                           ecma_char_t expected_char2) /**< second expected character */
{
  return (str_p < str_end_p && (*str_p == expected_char1 || *str_p == expected_char2));
} /* ecma_date_check_two_chars */

/**
 * Helper function to try to parse a 4-5-6 digit year with optional negative sign in a date string
 *
 * Date.prototype.toString() and Date.prototype.toUTCString() emits year
 * in this format and Date.parse() should parse this format too.
 *
 * @return the parsed year or NaN.
 */
static ecma_number_t
ecma_date_parse_year (const lit_utf8_byte_t **str_p, /**< pointer to the cesu8 string */
                      const lit_utf8_byte_t *str_end_p) /**< pointer to the end of the string */
{
  bool is_year_sign_negative = ecma_date_parse_special_char (str_p, str_end_p, LIT_CHAR_MINUS);
  const lit_utf8_byte_t *str_start_p = *str_p;
  int32_t parsed_year = 0;

  while ((str_start_p - *str_p < 6) && (str_start_p < str_end_p) && lit_char_is_decimal_digit (*str_start_p))
  {
    parsed_year = 10 * parsed_year + *str_start_p - LIT_CHAR_0;
    str_start_p++;
  }

  if (str_start_p - *str_p >= 4)
  {
    *str_p = str_start_p;
    return is_year_sign_negative ? -parsed_year : parsed_year;
  }

  return ecma_number_make_nan ();
} /* ecma_date_parse_year */

/**
 * Helper function to try to parse a day name in a date string
 * Valid day names: Sun, Mon, Tue, Wed, Thu, Fri, Sat
 * See also:
 *          ECMA-262 v9, 20.3.4.41.2 Table 46
 *
 * @return true if the string starts with a valid day name, false otherwise
 */
static bool
ecma_date_parse_day_name (const lit_utf8_byte_t **str_p, /**< pointer to the cesu8 string */
                          const lit_utf8_byte_t *str_end_p) /**< pointer to the end of the string */
{
  if (*str_p + 3 < str_end_p)
  {
    for (uint32_t i = 0; i < 7; i++)
    {
      if (!memcmp (day_names_p[i], *str_p, 3))
      {
        (*str_p) += 3;
        return true;
      }
    }
  }
  return false;
} /* ecma_date_parse_day_name */

/**
 * Helper function to try to parse a month name in a date string
 * Valid month names: Jan, Feb, Mar, Apr, May, Jun, Jul, Aug, Sep, Oct, Nov, Dec
 * See also:
 *          ECMA-262 v9, 20.3.4.41.2 Table 47
 *
 * @return number of the month if the string starts with a valid month name, 0 otherwise
 */
static uint32_t
ecma_date_parse_month_name (const lit_utf8_byte_t **str_p, /**< pointer to the cesu8 string */
                            const lit_utf8_byte_t *str_end_p) /**< pointer to the end of the string */
{
  if (*str_p + 3 < str_end_p)
  {
    for (uint32_t i = 0; i < 12; i++)
    {
      if (!memcmp (month_names_p[i], *str_p, 3))
      {
        (*str_p) += 3;
        return (i + 1);
      }
    }
  }
  return 0;
} /* ecma_date_parse_month_name */

/**
 * Calculate MakeDate(MakeDay(yr, m, dt), MakeTime(h, min, s, milli)) for Date constructor and UTC
 *
 * See also:
 *          ECMA-262 v11, 20.4.3.4
 *
 * @return false - if the operation fails
 *         true - otherwise
 */
static bool
ecma_date_construct_helper (const ecma_value_t *args, /**< arguments passed to the Date constructor */
                            uint32_t args_len, /**< number of arguments */
                            ecma_number_t *tv_p) /**< [out] time value */
{
  ecma_number_t date_nums[7] = {
    ECMA_NUMBER_ZERO, /* year */
    ECMA_NUMBER_ZERO, /* month */
    ECMA_NUMBER_ONE, /* date */
    ECMA_NUMBER_ZERO, /* hours */
    ECMA_NUMBER_ZERO, /* minutes */
    ECMA_NUMBER_ZERO, /* seconds */
    ECMA_NUMBER_ZERO /* milliseconds */
  };

  args_len = JERRY_MIN (args_len, sizeof (date_nums) / sizeof (date_nums[0]));

  /* 1-7. */
  for (uint32_t i = 0; i < args_len; i++)
  {
    ecma_value_t status = ecma_op_to_number (args[i], date_nums + i);

    if (ECMA_IS_VALUE_ERROR (status))
    {
      return false;
    }
  }

  /* 8. */
  if (!ecma_number_is_nan (date_nums[0]))
  {
    /* 9.a */
    ecma_number_t yi = ecma_number_trunc (date_nums[0]);

    /* 9.b */
    if (yi >= 0 && yi <= 99)
    {
      date_nums[0] = 1900 + yi;
    }
  }

  /* 10. */
  *tv_p = ecma_date_make_date (ecma_date_make_day (date_nums[0], date_nums[1], date_nums[2]),
                               ecma_date_make_time (date_nums[3], date_nums[4], date_nums[5], date_nums[6]));
  return true;
} /* ecma_date_construct_helper */

/**
 * Helper function used by ecma_builtin_date_parse
 *
 * See also:
 *          ECMA-262 v5, 15.9.4.2  Date.parse (string)
 *          ECMA-262 v5, 15.9.1.15 Date Time String Format
 *
 * @return the parsed date as ecma_number_t or NaN otherwise
 */
static ecma_number_t
ecma_builtin_date_parse_basic (const lit_utf8_byte_t *date_str_curr_p, /**< date string start */
                               const lit_utf8_byte_t *date_str_end_p) /**< date string end */
{
  /* 1. read year */

  uint32_t year_digits = 4;
  uint32_t year_limit = 9999;

  bool is_year_sign_negative = false;

  if (ecma_date_check_two_chars (date_str_curr_p, date_str_end_p, LIT_CHAR_MINUS, LIT_CHAR_PLUS))
  {
    is_year_sign_negative = (*date_str_curr_p++ == LIT_CHAR_MINUS);
    year_digits = 6;
    year_limit = 999999;
  }

  ecma_number_t year =
    ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, year_digits, ECMA_DATE_LIMIT (0, year_limit));
  if (is_year_sign_negative)
  {
    year = -year;
  }

  if (ecma_number_is_nan (year))
  {
    return year;
  }

  ecma_number_t month = ECMA_NUMBER_ONE;
  ecma_number_t day = ECMA_NUMBER_ONE;
  ecma_number_t time = ECMA_NUMBER_ZERO;

  /* 2. read month if any */
  if (ecma_date_check_two_chars (date_str_curr_p, date_str_end_p, LIT_CHAR_MINUS, LIT_CHAR_SLASH))
  {
    lit_utf8_byte_t separator = *date_str_curr_p++;
    month = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (1, 12));

    /* 3. read day if any */
    if (ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, separator))
    {
      day = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (1, 31));
    }
  }

  bool is_utc = true;
  /* 4. read time if any */
  if (ecma_date_check_two_chars (date_str_curr_p, date_str_end_p, LIT_CHAR_UPPERCASE_T, LIT_CHAR_SP))
  {
    date_str_curr_p++;

    ecma_number_t hours = ECMA_NUMBER_ZERO;
    ecma_number_t minutes = ECMA_NUMBER_ZERO;
    ecma_number_t seconds = ECMA_NUMBER_ZERO;
    ecma_number_t milliseconds = ECMA_NUMBER_ZERO;

    /* 'HH:mm' must present */
    if (date_str_end_p - date_str_curr_p < 5)
    {
      return ecma_number_make_nan ();
    }

    /* 4.1 read hours and minutes */
    hours = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 24));

    if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_COLON))
    {
      return ecma_number_make_nan ();
    }

    minutes = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 59));

    /* 4.2 read seconds if any */
    if (ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_COLON))
    {
      seconds = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 59));

      /* 4.3 read milliseconds if any */
      if (ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_DOT))
      {
        milliseconds = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 3, ECMA_DATE_LIMIT (0, 999));
      }
    }

    if (hours == 24 && (minutes != 0 || seconds != 0 || milliseconds != 0))
    {
      return ecma_number_make_nan ();
    }

    time = ecma_date_make_time (hours, minutes, seconds, milliseconds);

    if (ecma_number_is_nan (time))
    {
      return time;
    }

    /* 4.4 read timezone if any */
    if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_UPPERCASE_Z))
    {
      if ((date_str_end_p - date_str_curr_p) == 6
          && (*date_str_curr_p == LIT_CHAR_MINUS || *date_str_curr_p == LIT_CHAR_PLUS))
      {
        bool is_timezone_sign_negative = (*date_str_curr_p++ == LIT_CHAR_MINUS);
        /* read hours and minutes */
        hours = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 24));

        if (hours == 24)
        {
          hours = ECMA_NUMBER_ZERO;
        }

        if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_COLON))
        {
          return ecma_number_make_nan ();
        }

        minutes = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 59));
        ecma_number_t timezone_offset = ecma_date_make_time (hours, minutes, ECMA_NUMBER_ZERO, ECMA_NUMBER_ZERO);
        time += is_timezone_sign_negative ? timezone_offset : -timezone_offset;
      }
      else
      {
        is_utc = false;
      }
    }
  }

  if (date_str_curr_p < date_str_end_p)
  {
    return ecma_number_make_nan ();
  }

  ecma_number_t date = ecma_date_make_day (year, month - 1, day);
  ecma_number_t result_date = ecma_date_make_date (date, time);

  if (!is_utc)
  {
    result_date = ecma_date_utc (result_date);
  }

  return result_date;
} /* ecma_builtin_date_parse_basic */

/**
 * Helper function used by ecma_builtin_date_parse
 *
 * See also:
 *          ECMA-262 v5, 15.9.4.2  Date.parse (string)
 *          ECMA-262 v9, 20.3.4.41 Date.prototype.toString ()
 *          ECMA-262 v9, 20.3.4.43 Date.prototype.toUTCString ()
 *
 * Used by: ecma_builtin_date_parse
 *
 * @return the parsed date as ecma_number_t or NaN otherwise
 */
static ecma_number_t
ecma_builtin_date_parse_toString_formats (const lit_utf8_byte_t *date_str_curr_p, const lit_utf8_byte_t *date_str_end_p)
{
  const ecma_number_t nan = ecma_number_make_nan ();

  if (!ecma_date_parse_day_name (&date_str_curr_p, date_str_end_p))
  {
    return nan;
  }

  const bool is_toUTCString_format = ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_COMMA);

  if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_SP))
  {
    return nan;
  }

  ecma_number_t month = 0;
  ecma_number_t day = 0;
  if (is_toUTCString_format)
  {
    day = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 31));

    if (ecma_number_is_nan (day))
    {
      return nan;
    }

    if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_SP))
    {
      return nan;
    }

    month = ecma_date_parse_month_name (&date_str_curr_p, date_str_end_p);

    if (month == 0)
    {
      return ecma_number_make_nan ();
    }
  }
  else
  {
    month = ecma_date_parse_month_name (&date_str_curr_p, date_str_end_p);

    if (month == 0)
    {
      return nan;
    }

    if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_SP))
    {
      return nan;
    }

    day = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 31));

    if (ecma_number_is_nan (day))
    {
      return nan;
    }
  }

  if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_SP))
  {
    return nan;
  }

  ecma_number_t year = ecma_date_parse_year (&date_str_curr_p, date_str_end_p);

  if (ecma_number_is_nan (year))
  {
    return nan;
  }

  if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_SP))
  {
    return nan;
  }

  ecma_number_t hours = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 24));

  if (ecma_number_is_nan (hours))
  {
    return nan;
  }

  if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_COLON))
  {
    return nan;
  }

  ecma_number_t minutes = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 59));

  if (ecma_number_is_nan (minutes))
  {
    return nan;
  }

  if (!ecma_date_parse_special_char (&date_str_curr_p, date_str_end_p, LIT_CHAR_COLON))
  {
    return nan;
  }

  ecma_number_t seconds = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 59));

  if (ecma_number_is_nan (seconds))
  {
    return nan;
  }

  if (hours == 24 && (minutes != 0 || seconds != 0))
  {
    return nan;
  }

  const char gmt_p[] = " GMT";
  if (date_str_end_p - date_str_curr_p < 4 || memcmp (date_str_curr_p, gmt_p, 4) != 0)
  {
    return nan;
  }

  date_str_curr_p += 4;

  ecma_number_t time = ecma_date_make_time (hours, minutes, seconds, 0);

  if (!is_toUTCString_format)
  {
    if (!ecma_date_check_two_chars (date_str_curr_p, date_str_end_p, LIT_CHAR_MINUS, LIT_CHAR_PLUS))
    {
      return nan;
    }

    bool is_timezone_sign_negative = (*date_str_curr_p++ == LIT_CHAR_MINUS);

    hours = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 24));

    if (ecma_number_is_nan (hours))
    {
      return nan;
    }

    if (hours == 24)
    {
      hours = ECMA_NUMBER_ZERO;
    }

    minutes = ecma_date_parse_date_chars (&date_str_curr_p, date_str_end_p, 2, ECMA_DATE_LIMIT (0, 59));

    if (ecma_number_is_nan (minutes))
    {
      return nan;
    }

    ecma_number_t timezone_offset = ecma_date_make_time (hours, minutes, ECMA_NUMBER_ZERO, ECMA_NUMBER_ZERO);

    time += is_timezone_sign_negative ? timezone_offset : -timezone_offset;
  }

  if (date_str_curr_p < date_str_end_p)
  {
    return nan;
  }

  ecma_number_t date = ecma_date_make_day (year, month - 1, day);
  return ecma_date_make_date (date, time);
} /* ecma_builtin_date_parse_toString_formats */

/**
 * The Date object's 'parse' routine
 *
 * See also:
 *          ECMA-262 v5, 15.9.4.2  Date.parse (string)
 *          ECMA-262 v5, 15.9.1.15 Date Time String Format
 *          ECMA-262 v9, 20.3.4.41 Date.prototype.toString ()
 *          ECMA-262 v9, 20.3.4.43 Date.prototype.toUTCString ()
 *
 * @return parsed time
 */
static ecma_number_t
ecma_builtin_date_parse (ecma_string_t *string_p) /**< string */
{
  ECMA_STRING_TO_UTF8_STRING (string_p, str_p, str_size);
  const lit_utf8_byte_t *date_str_curr_p = str_p;
  const lit_utf8_byte_t *date_str_end_p = str_p + str_size;

  /* try to parse date string as ISO string - ECMA-262 v5, 15.9.1.15 */
  ecma_number_t tv = ecma_builtin_date_parse_basic (date_str_curr_p, date_str_end_p);

  if (ecma_number_is_nan (tv))
  {
    /* try to parse date string in Date.prototype.toString() or toUTCString() format */
    tv = ecma_builtin_date_parse_toString_formats (date_str_curr_p, date_str_end_p);
  }

  ECMA_FINALIZE_UTF8_STRING (str_p, str_size);

  return tv;
} /* ecma_builtin_date_parse */

/**
 * The Date object's 'UTC' routine
 *
 * See also:
 *          ECMA-262 v5, 15.9.4.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_date_utc (const ecma_value_t args[], /**< arguments list */
                       uint32_t args_number) /**< number of arguments */
{
  if (args_number < 1)
  {
    return ecma_make_nan_value ();
  }

  ecma_number_t tv;

  if (!ecma_date_construct_helper (args, args_number, &tv))
  {
    return ECMA_VALUE_ERROR;
  }

  return ecma_make_number_value ((ecma_number_t) ecma_date_time_clip (tv));
} /* ecma_builtin_date_utc */

/**
 * Helper method to get the current time
 *
 * @return ecma_number_t
 */
static ecma_number_t
ecma_builtin_date_now_helper (void)
{
  return floor (DOUBLE_TO_ECMA_NUMBER_T (jerry_port_current_time ()));
} /* ecma_builtin_date_now_helper */

/**
 * Construct a date object with the given [[DateValue]]
 *
 * Note: New target must be a valid object
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         constructed date object - otherwise
 */
static ecma_value_t
ecma_builtin_date_create (ecma_number_t tv)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (JERRY_CONTEXT (current_new_target_p) != NULL);

  ecma_object_t *prototype_obj_p =
    ecma_op_get_prototype_from_constructor (JERRY_CONTEXT (current_new_target_p), ECMA_BUILTIN_ID_DATE_PROTOTYPE);

  if (JERRY_UNLIKELY (prototype_obj_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_object_t *obj_p = ecma_create_object (prototype_obj_p, sizeof (ecma_date_object_t), ECMA_OBJECT_TYPE_CLASS);
  ecma_deref_object (prototype_obj_p);

  ecma_date_object_t *date_object_p = (ecma_date_object_t *) obj_p;
  date_object_p->header.u.cls.type = ECMA_OBJECT_CLASS_DATE;
  date_object_p->header.u.cls.u1.date_flags = ECMA_DATE_TZA_NONE;
  date_object_p->header.u.cls.u3.tza = 0;
  date_object_p->date_value = tv;

  return ecma_make_object_value (obj_p);
} /* ecma_builtin_date_create */

/**
 * Handle calling [[Call]] of built-in Date object
 *
 * See also:
 *          ECMA-262 v5, 15.9.2.1
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_date_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                 uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_UNUSED (arguments_list_p);
  JERRY_UNUSED (arguments_list_len);

  return ecma_date_value_to_string (ecma_builtin_date_now_helper ());
} /* ecma_builtin_date_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Date object
 *
 * See also:
 *          ECMA-262 v5, 15.9.3.1
 *          ECMA-262 v11, 20.4.2
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_date_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                      uint32_t arguments_list_len) /**< number of arguments */
{
  /* 20.4.2.3 */
  if (arguments_list_len == 0)
  {
    return ecma_builtin_date_create (ecma_builtin_date_now_helper ());
  }

  ecma_number_t tv;
  /* 20.4.2.2 */
  if (arguments_list_len == 1)
  {
    ecma_value_t argument = arguments_list_p[0];

    /* 4.a */
    if (ecma_is_value_object (argument)
        && ecma_object_class_is (ecma_get_object_from_value (argument), ECMA_OBJECT_CLASS_DATE))
    {
      return ecma_builtin_date_create (((ecma_date_object_t *) ecma_get_object_from_value (argument))->date_value);
    }
    /* 4.b */
    ecma_value_t primitive = ecma_op_to_primitive (argument, ECMA_PREFERRED_TYPE_NO);

    if (ECMA_IS_VALUE_ERROR (primitive))
    {
      return primitive;
    }

    if (ecma_is_value_string (primitive))
    {
      ecma_string_t *prim_str_p = ecma_get_string_from_value (primitive);
      tv = ecma_builtin_date_parse (prim_str_p);
      ecma_deref_ecma_string (prim_str_p);
    }
    else
    {
      ecma_value_t prim_value = ecma_op_to_number (primitive, &tv);
      ecma_free_value (primitive);

      if (ECMA_IS_VALUE_ERROR (prim_value))
      {
        return prim_value;
      }
    }
  }
  /* 20.4.2.1 */
  else if (ecma_date_construct_helper (arguments_list_p, arguments_list_len, &tv))
  {
    tv = ecma_date_utc (tv);
  }
  else
  {
    return ECMA_VALUE_ERROR;
  }

  return ecma_builtin_date_create (ecma_date_time_clip (tv));
} /* ecma_builtin_date_dispatch_construct */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_date_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine  identifier */
                                    ecma_value_t this_arg, /**< 'this' argument value */
                                    const ecma_value_t arguments_list_p[], /**< list of arguments passed to routine */
                                    uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (this_arg);

  switch (builtin_routine_id)
  {
    case ECMA_DATE_ROUTINE_NOW:
    {
      return ecma_make_number_value (ecma_builtin_date_now_helper ());
    }
    case ECMA_DATE_ROUTINE_UTC:
    {
      return ecma_builtin_date_utc (arguments_list_p, arguments_number);
    }
    case ECMA_DATE_ROUTINE_PARSE:
    {
      if (arguments_number < 1)
      {
        return ecma_make_nan_value ();
      }

      ecma_string_t *str_p = ecma_op_to_string (arguments_list_p[0]);

      if (JERRY_UNLIKELY (str_p == NULL))
      {
        return ECMA_VALUE_ERROR;
      }

      ecma_value_t result = ecma_make_number_value (ecma_date_time_clip (ecma_builtin_date_parse (str_p)));
      ecma_deref_ecma_string (str_p);

      return result;
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_date_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_DATE */
