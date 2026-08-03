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
#include "ecma-exceptions.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"

#include "lit-char-helpers.h"

#if JERRY_BUILTIN_DATE

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltinhelpers ECMA builtin helper operations
 * @{
 */

/**
 * Day names
 */
const char* day_names_p[7] = { "Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat" };

/**
 * Month names
 */
const char* month_names_p[12] = {
  "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"
};

/**
 * Calculate the elapsed days since Unix Epoch
 *
 * @return elapsed days since Unix Epoch
 */
int32_t
ecma_date_day_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  if (time < 0)
  {
    time -= ECMA_DATE_MS_PER_DAY - 1;
  }

  return (int32_t) (time / ECMA_DATE_MS_PER_DAY);
} /* ecma_date_day_from_time */

/**
 * Abstract operation: DayFromYear
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.3
 *
 * @return first of day in the given year
 */
static int32_t
ecma_date_day_from_year (int32_t year) /**< year value */
{
  if (JERRY_LIKELY (year >= 1970))
  {
    return (int32_t) (365 * (year - 1970) + ((year - 1969) / 4) - ((year - 1901) / 100) + ((year - 1601) / 400));
  }

  return (int32_t) (365 * (year - 1970) + floor ((year - 1969) / 4.0) - floor ((year - 1901) / 100.0)
                    + floor ((year - 1601) / 400.0));
} /* ecma_date_day_from_year */

/**
 * Abstract operation: DaysInYear
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.3
 *
 * @return number of days in the given year
 */
static int
ecma_date_days_in_year (int32_t year) /**< year */
{
  if (year % 4 != 0 || (year % 100 == 0 && (year % 400 != 0)))
  {
    return ECMA_DATE_DAYS_IN_YEAR;
  }

  return ECMA_DATE_DAYS_IN_LEAP_YEAR;
} /* ecma_date_days_in_year */

/**
 * Abstract operation: InLeapYear
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.3
 *
 * @return 1 - if the year is leap
 *         0 - otherwise
 */
static int32_t
ecma_date_in_leap_year (int32_t year) /**< time value */
{
  return ecma_date_days_in_year (year) - ECMA_DATE_DAYS_IN_YEAR;
} /* ecma_date_in_leap_year */

/**
 * First days of months in normal and leap years
 */
static const uint16_t first_day_in_month[2][12] = { {
                                                      0,
                                                      31,
                                                      59,
                                                      90,
                                                      120,
                                                      151,
                                                      181,
                                                      212,
                                                      243,
                                                      273,
                                                      304,
                                                      334, /* normal year */
                                                    },
                                                    {
                                                      0,
                                                      31,
                                                      60,
                                                      91,
                                                      121,
                                                      152,
                                                      182,
                                                      213,
                                                      244,
                                                      274,
                                                      305,
                                                      335 /* leap year */
                                                    } };

/**
 * Abstract operation: YearFromTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.3
 *
 * @return year corresponds to the given time
 */
int32_t
ecma_date_year_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  int32_t approx = (int32_t) (floor (time / ECMA_DATE_MS_PER_DAY / 365.2425) + 1970);
  int64_t year_ms = ecma_date_day_from_year (approx) * ((int64_t) ECMA_DATE_MS_PER_DAY);

  if ((ecma_number_t) year_ms > time)
  {
    approx--;
  }

  if ((ecma_number_t) (year_ms + ecma_date_days_in_year (approx) * ((int64_t) ECMA_DATE_MS_PER_DAY)) <= time)
  {
    approx++;
  }

  return approx;
} /* ecma_date_year_from_time */

/**
 * Abstract operation: MonthFromTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.4
 *
 * @return month corresponds to the given time
 */
int32_t
ecma_date_month_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  int32_t year = ecma_date_year_from_time (time);
  int32_t day_within_year = ecma_date_day_from_time (time) - ecma_date_day_from_year (year);

  JERRY_ASSERT (day_within_year >= 0 && day_within_year < ECMA_DATE_DAYS_IN_LEAP_YEAR);

  int32_t in_leap_year = ecma_date_in_leap_year (year);

  for (int i = 1; i < 12; i++)
  {
    if (day_within_year < first_day_in_month[in_leap_year][i])
    {
      return i - 1;
    }
  }

  return 11;
} /* ecma_date_month_from_time */

/**
 * Abstract operation: DateFromTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.4
 *
 * @return date corresponds to the given time
 */
int32_t
ecma_date_date_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  int32_t year = ecma_date_year_from_time (time);
  int32_t day_within_year = ecma_date_day_from_time (time) - ecma_date_day_from_year (year);

  JERRY_ASSERT (day_within_year >= 0 && day_within_year < ECMA_DATE_DAYS_IN_LEAP_YEAR);

  int32_t in_leap_year = ecma_date_in_leap_year (year);

  int32_t month = 11;

  for (int i = 1; i < 12; i++)
  {
    if (day_within_year < first_day_in_month[in_leap_year][i])
    {
      month = i - 1;
      break;
    }
  }

  return day_within_year + 1 - first_day_in_month[in_leap_year][month];
} /* ecma_date_date_from_time */

/**
 * Abstract operation: WeekDay
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.4
 *
 * @return weekday corresponds to the given time
 */
int32_t
ecma_date_week_day (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  int32_t day = ecma_date_day_from_time (time);

  int week_day = (day + 4) % 7;

  return week_day >= 0 ? week_day : week_day + 7;
} /* ecma_date_week_day */

/**
 * Abstract operation: LocalTZA
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.7
 *
 * @return local time zone adjustment
 */
int32_t
ecma_date_local_time_zone_adjustment (ecma_number_t time) /**< time value */
{
  return jerry_port_local_tza (time);
} /* ecma_date_local_time_zone_adjustment */

/**
 * Abstract operation: UTC
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.9
 *
 * @return UTC time
 */
ecma_number_t
ecma_date_utc (ecma_number_t time) /**< time value */
{
  return time - jerry_port_local_tza (time);
} /* ecma_date_utc */

/**
 * Calculate the time component from the given time
 *
 * @return time component of the given time
 */
int32_t
ecma_date_time_in_day_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  ecma_number_t day = ecma_date_day_from_time (time);

  return (int32_t) (time - (day * ECMA_DATE_MS_PER_DAY));
} /* ecma_date_time_in_day_from_time */

/**
 * Abstract operation: HourFromTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.10
 *
 * @return hours component of the given time
 */
int32_t
ecma_date_hour_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  int32_t time_in_day = ecma_date_time_in_day_from_time (time);

  return (int32_t) (time_in_day / ECMA_DATE_MS_PER_HOUR);
} /* ecma_date_hour_from_time */

/**
 * Abstract operation: HourFromTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.10
 *
 * @return minutes component of the given time
 */
int32_t
ecma_date_min_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  int32_t time_in_day = ecma_date_time_in_day_from_time (time);

  return ((int32_t) (time_in_day / ECMA_DATE_MS_PER_MINUTE)) % ECMA_DATE_MINUTES_PER_HOUR;
} /* ecma_date_min_from_time */

/**
 * Abstract operation: HourFromTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.10
 *
 * @return seconds component of the given time
 */
int32_t
ecma_date_sec_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  int32_t time_in_day = ecma_date_time_in_day_from_time (time);

  return ((int32_t) (time_in_day / ECMA_DATE_MS_PER_SECOND)) % ECMA_DATE_SECONDS_PER_MINUTE;
} /* ecma_date_sec_from_time */

/**
 * Abstract operation: HourFromTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.10
 *
 * @return milliseconds component of the given time
 */
int32_t
ecma_date_ms_from_time (ecma_number_t time) /**< time value */
{
  JERRY_ASSERT (!ecma_number_is_nan (time));

  int32_t time_in_day = ecma_date_time_in_day_from_time (time);

  return (int32_t) (time_in_day % ECMA_DATE_MS_PER_SECOND);
} /* ecma_date_ms_from_time */

/**
 * Abstract operation: MakeTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.11
 *
 * @return constructed time in milliseconds
 */
ecma_number_t
ecma_date_make_time (ecma_number_t hour, /**< hour value */
                     ecma_number_t min, /**< minute value */
                     ecma_number_t sec, /**< second value */
                     ecma_number_t ms) /**< millisecond value */
{
  if (!ecma_number_is_finite (hour) || !ecma_number_is_finite (min) || !ecma_number_is_finite (sec)
      || !ecma_number_is_finite (ms))
  {
    return ecma_number_make_nan ();
  }

  ecma_number_t h = ecma_number_trunc (hour);
  ecma_number_t m = ecma_number_trunc (min);
  ecma_number_t s = ecma_number_trunc (sec);
  ecma_number_t milli = ecma_number_trunc (ms);

  return h * ECMA_DATE_MS_PER_HOUR + m * ECMA_DATE_MS_PER_MINUTE + s * ECMA_DATE_MS_PER_SECOND + milli;
} /* ecma_date_make_time */

/**
 * Abstract operation: MakeDay
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.12
 *
 * @return elapsed number of days since Unix Epoch
 */
ecma_number_t
ecma_date_make_day (ecma_number_t year, /**< year value */
                    ecma_number_t month, /**< month value */
                    ecma_number_t date) /**< date value */
{
  /* 1. */
  if (!ecma_number_is_finite (year) || !ecma_number_is_finite (month) || !ecma_number_is_finite (date)
      || fabs (year) > INT32_MAX)
  {
    return ecma_number_make_nan ();
  }

  /* 2., 3., 4. */
  int32_t y = (int32_t) (year);
  ecma_number_t m = ecma_number_trunc (month);
  ecma_number_t dt = ecma_number_trunc (date);

  /* 5. */
  int32_t ym = y + (int32_t) (floor (m / 12));

  /* 6. */
  int32_t mn = (int32_t) fmod (m, 12);

  if (mn < 0)
  {
    mn += 12;
  }

  /* 7. */
  ecma_number_t days = (ecma_date_day_from_year (ym) + first_day_in_month[ecma_date_in_leap_year (ym)][mn] + (dt - 1));
  return days * ECMA_DATE_MS_PER_DAY;
} /* ecma_date_make_day */

/**
 * Abstract operation: MakeTime
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.13
 *
 * @return elapsed number of milliseconds since Unix Epoch
 */
ecma_number_t
ecma_date_make_date (ecma_number_t day, /**< day value */
                     ecma_number_t time) /**< time value */
{
  if (!ecma_number_is_finite (day) || !ecma_number_is_finite (time))
  {
    return ecma_number_make_nan ();
  }

  return day + time;
} /* ecma_date_make_date */

/**
 * Abstract operation: TimeClip
 *
 *  See also:
 *          ECMA-262 v11, 20.4.1.14
 *
 * @return elapsed number of milliseconds since Unix Epoch
 */
ecma_number_t
ecma_date_time_clip (ecma_number_t time) /**< time value */
{
  if (!ecma_number_is_finite (time) || fabs (time) > ECMA_DATE_MAX_VALUE)
  {
    return ecma_number_make_nan ();
  }

  return ecma_number_trunc (time);
} /* ecma_date_time_clip */

/**
 * Common function to convert date to string.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_date_to_string_format (ecma_number_t datetime_number, /**< datetime */
                            const char *format_p) /**< format buffer */
{
  const uint32_t date_buffer_length = 37;
  JERRY_VLA (lit_utf8_byte_t, date_buffer, date_buffer_length);

  lit_utf8_byte_t *dest_p = date_buffer;

  while (*format_p != LIT_CHAR_NULL)
  {
    if (*format_p != LIT_CHAR_DOLLAR_SIGN)
    {
      *dest_p++ = (lit_utf8_byte_t) *format_p++;
      continue;
    }

    format_p++;

    const char *str_p = NULL;
    int32_t number = 0;
    int32_t number_length = 0;

    switch (*format_p)
    {
      case LIT_CHAR_UPPERCASE_Y: /* Year. */
      {
        number = ecma_date_year_from_time (datetime_number);

        if (number >= 100000 || number <= -100000)
        {
          number_length = 6;
        }
        else if (number >= 10000 || number <= -10000)
        {
          number_length = 5;
        }
        else
        {
          number_length = 4;
        }
        break;
      }
      case LIT_CHAR_LOWERCASE_Y: /* ISO Year: -000001, 0000, 0001, 9999, +012345 */
      {
        number = ecma_date_year_from_time (datetime_number);
        if (0 <= number && number <= 9999)
        {
          number_length = 4;
        }
        else
        {
          number_length = 6;
        }
        break;
      }
      case LIT_CHAR_UPPERCASE_M: /* Month. */
      {
        int32_t month = ecma_date_month_from_time (datetime_number);

        JERRY_ASSERT (month >= 0 && month <= 11);

        str_p = month_names_p[month];
        break;
      }
      case LIT_CHAR_UPPERCASE_O: /* Month as number. */
      {
        /* The 'ecma_date_month_from_time' (ECMA 262 v5, 15.9.1.4) returns a
         * number from 0 to 11, but we have to print the month from 1 to 12
         * for ISO 8601 standard (ECMA 262 v5, 15.9.1.15). */
        number = ecma_date_month_from_time (datetime_number) + 1;
        number_length = 2;
        break;
      }
      case LIT_CHAR_UPPERCASE_D: /* Day. */
      {
        number = ecma_date_date_from_time (datetime_number);
        number_length = 2;
        break;
      }
      case LIT_CHAR_UPPERCASE_W: /* Day of week. */
      {
        int32_t day = ecma_date_week_day (datetime_number);

        JERRY_ASSERT (day >= 0 && day <= 6);

        str_p = day_names_p[day];
        break;
      }
      case LIT_CHAR_LOWERCASE_H: /* Hour. */
      {
        number = ecma_date_hour_from_time (datetime_number);
        number_length = 2;
        break;
      }
      case LIT_CHAR_LOWERCASE_M: /* Minutes. */
      {
        number = ecma_date_min_from_time (datetime_number);
        number_length = 2;
        break;
      }
      case LIT_CHAR_LOWERCASE_S: /* Seconds. */
      {
        number = ecma_date_sec_from_time (datetime_number);
        number_length = 2;
        break;
      }
      case LIT_CHAR_LOWERCASE_I: /* Milliseconds. */
      {
        number = ecma_date_ms_from_time (datetime_number);
        number_length = 3;
        break;
      }
      case LIT_CHAR_LOWERCASE_Z: /* Time zone hours part. */
      {
        int32_t time_zone = ecma_date_local_time_zone_adjustment (datetime_number);

        if (time_zone >= 0)
        {
          *dest_p++ = LIT_CHAR_PLUS;
        }
        else
        {
          *dest_p++ = LIT_CHAR_MINUS;
          time_zone = -time_zone;
        }

        number = time_zone / ECMA_DATE_MS_PER_HOUR;
        number_length = 2;
        break;
      }
      default:
      {
        JERRY_ASSERT (*format_p == LIT_CHAR_UPPERCASE_Z); /* Time zone minutes part. */

        int32_t time_zone = ecma_date_local_time_zone_adjustment (datetime_number);

        if (time_zone < 0)
        {
          time_zone = -time_zone;
        }

        number = (time_zone % ECMA_DATE_MS_PER_HOUR) / ECMA_DATE_MS_PER_MINUTE;
        number_length = 2;
        break;
      }
    }

    format_p++;

    if (str_p != NULL)
    {
      /* Print string values: month or day name which is always 3 characters */
      memcpy (dest_p, str_p, 3);
      dest_p += 3;
      continue;
    }

    /* Print right aligned number values. */
    JERRY_ASSERT (number_length > 0);

    if (number < 0)
    {
      number = -number;
      *dest_p++ = '-';
    }
    else if (*(format_p - 1) == LIT_CHAR_LOWERCASE_Y && number_length == 6)
    {
      /* positive sign is compulsory for extended years */
      *dest_p++ = '+';
    }

    dest_p += number_length;
    lit_utf8_byte_t *buffer_p = dest_p;

    do
    {
      buffer_p--;
      *buffer_p = (lit_utf8_byte_t) ((number % 10) + (int32_t) LIT_CHAR_0);
      number /= 10;
    } while (--number_length);
  }

  JERRY_ASSERT (dest_p <= date_buffer + date_buffer_length);

  return ecma_make_string_value (
    ecma_new_ecma_string_from_ascii (date_buffer, (lit_utf8_size_t) (dest_p - date_buffer)));
} /* ecma_date_to_string_format */

/**
 * Common function to create a time zone specific string from a numeric value.
 *
 * Used by:
 *        - The Date routine.
 *        - The Date.prototype.toString routine.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_date_value_to_string (ecma_number_t datetime_number) /**< datetime */
{
  datetime_number += ecma_date_local_time_zone_adjustment (datetime_number);
  return ecma_date_to_string_format (datetime_number, "$W $M $D $Y $h:$m:$s GMT$z$Z");
} /* ecma_date_value_to_string */

/**
 * Common function to create a time zone specific string from a numeric value.
 *
 * Used by:
 *        - The Date.prototype.toUTCString routine.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_date_value_to_utc_string (ecma_number_t datetime_number) /**< datetime */
{
  return ecma_date_to_string_format (datetime_number, "$W, $D $M $Y $h:$m:$s GMT");
} /* ecma_date_value_to_utc_string */

/**
 * Common function to create a ISO specific string from a numeric value.
 *
 * Used by:
 *        - The Date.prototype.toISOString routine.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_date_value_to_iso_string (ecma_number_t datetime_number) /**<datetime */
{
  return ecma_date_to_string_format (datetime_number, "$y-$O-$DT$h:$m:$s.$iZ");
} /* ecma_date_value_to_iso_string */

/**
 * Common function to create a date string from a numeric value.
 *
 * Used by:
 *        - The Date.prototype.toDateString routine.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_date_value_to_date_string (ecma_number_t datetime_number) /**<datetime */
{
  return ecma_date_to_string_format (datetime_number, "$W $M $D $Y");
} /* ecma_date_value_to_date_string */

/**
 * Common function to create a time string from a numeric value.
 *
 * Used by:
 *        - The Date.prototype.toTimeString routine.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_date_value_to_time_string (ecma_number_t datetime_number) /**<datetime */
{
  return ecma_date_to_string_format (datetime_number, "$h:$m:$s GMT$z$Z");
} /* ecma_date_value_to_time_string */

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_DATE */
