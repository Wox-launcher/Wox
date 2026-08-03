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
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#if JERRY_BUILTIN_DATE

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

/**
 * This object has a custom dispatch function.
 */
#define BUILTIN_CUSTOM_DISPATCH

/**
 * Checks whether the function uses UTC time zone.
 */
#define BUILTIN_DATE_FUNCTION_IS_UTC(builtin_routine_id) \
  (((builtin_routine_id) -ECMA_DATE_PROTOTYPE_GET_FULL_YEAR) & 0x1)

/**
 * List of built-in routine identifiers.
 */
enum
{
  ECMA_DATE_PROTOTYPE_ROUTINE_START = 0,

  ECMA_DATE_PROTOTYPE_GET_FULL_YEAR, /* ECMA-262 v5 15.9.5.10 */
  ECMA_DATE_PROTOTYPE_GET_UTC_FULL_YEAR, /* ECMA-262 v5 15.9.5.11 */
#if JERRY_BUILTIN_ANNEXB
  ECMA_DATE_PROTOTYPE_GET_YEAR, /* ECMA-262 v5, AnnexB.B.2.4 */
  ECMA_DATE_PROTOTYPE_GET_UTC_YEAR, /* has no UTC variant */
#endif /* JERRY_BUILTIN_ANNEXB */
  ECMA_DATE_PROTOTYPE_GET_MONTH, /* ECMA-262 v5 15.9.5.12 */
  ECMA_DATE_PROTOTYPE_GET_UTC_MONTH, /* ECMA-262 v5 15.9.5.13 */
  ECMA_DATE_PROTOTYPE_GET_DATE, /* ECMA-262 v5 15.9.5.14 */
  ECMA_DATE_PROTOTYPE_GET_UTC_DATE, /* ECMA-262 v5 15.9.5.15 */
  ECMA_DATE_PROTOTYPE_GET_DAY, /* ECMA-262 v5 15.9.5.16 */
  ECMA_DATE_PROTOTYPE_GET_UTC_DAY, /* ECMA-262 v5 15.9.5.17 */
  ECMA_DATE_PROTOTYPE_GET_HOURS, /* ECMA-262 v5 15.9.5.18 */
  ECMA_DATE_PROTOTYPE_GET_UTC_HOURS, /* ECMA-262 v5 15.9.5.19 */
  ECMA_DATE_PROTOTYPE_GET_MINUTES, /* ECMA-262 v5 15.9.5.20 */
  ECMA_DATE_PROTOTYPE_GET_UTC_MINUTES, /* ECMA-262 v5 15.9.5.21 */
  ECMA_DATE_PROTOTYPE_GET_SECONDS, /* ECMA-262 v5 15.9.5.22 */
  ECMA_DATE_PROTOTYPE_GET_UTC_SECONDS, /* ECMA-262 v5 15.9.5.23 */
  ECMA_DATE_PROTOTYPE_GET_MILLISECONDS, /* ECMA-262 v5 15.9.5.24 */
  ECMA_DATE_PROTOTYPE_GET_UTC_MILLISECONDS, /* ECMA-262 v5 15.9.5.25 */
  ECMA_DATE_PROTOTYPE_GET_TIMEZONE_OFFSET, /* has no local time zone variant */
  ECMA_DATE_PROTOTYPE_GET_UTC_TIMEZONE_OFFSET, /* ECMA-262 v5 15.9.5.26 */

  ECMA_DATE_PROTOTYPE_SET_FULL_YEAR, /* ECMA-262 v5, 15.9.5.40 */
  ECMA_DATE_PROTOTYPE_SET_UTC_FULL_YEAR, /* ECMA-262 v5, 15.9.5.41 */
#if JERRY_BUILTIN_ANNEXB
  ECMA_DATE_PROTOTYPE_SET_YEAR, /* ECMA-262 v5, ECMA-262 v5, AnnexB.B.2.5 */
  ECMA_DATE_PROTOTYPE_SET_UTC_YEAR, /* has no UTC variant */
#endif /* JERRY_BUILTIN_ANNEXB */
  ECMA_DATE_PROTOTYPE_SET_MONTH, /* ECMA-262 v5, 15.9.5.38 */
  ECMA_DATE_PROTOTYPE_SET_UTC_MONTH, /* ECMA-262 v5, 15.9.5.39 */
  ECMA_DATE_PROTOTYPE_SET_DATE, /* ECMA-262 v5, 15.9.5.36 */
  ECMA_DATE_PROTOTYPE_SET_UTC_DATE, /* ECMA-262 v5, 15.9.5.37 */
  ECMA_DATE_PROTOTYPE_SET_HOURS, /* ECMA-262 v5, 15.9.5.34 */
  ECMA_DATE_PROTOTYPE_SET_UTC_HOURS, /* ECMA-262 v5, 15.9.5.35 */
  ECMA_DATE_PROTOTYPE_SET_MINUTES, /* ECMA-262 v5, 15.9.5.32 */
  ECMA_DATE_PROTOTYPE_SET_UTC_MINUTES, /* ECMA-262 v5, 15.9.5.33 */
  ECMA_DATE_PROTOTYPE_SET_SECONDS, /* ECMA-262 v5, 15.9.5.30 */
  ECMA_DATE_PROTOTYPE_SET_UTC_SECONDS, /* ECMA-262 v5, 15.9.5.31 */
  ECMA_DATE_PROTOTYPE_SET_MILLISECONDS, /* ECMA-262 v5, 15.9.5.28 */
  ECMA_DATE_PROTOTYPE_SET_UTC_MILLISECONDS, /* ECMA-262 v5, 15.9.5.29 */

  ECMA_DATE_PROTOTYPE_TO_STRING, /* ECMA-262 v5, 15.9.5.2 */
  ECMA_DATE_PROTOTYPE_TO_DATE_STRING, /* ECMA-262 v5, 15.9.5.3 */
  ECMA_DATE_PROTOTYPE_TO_TIME_STRING, /* ECMA-262 v5, 15.9.5.4 */
  ECMA_DATE_PROTOTYPE_TO_ISO_STRING, /* ECMA-262 v5, 15.9.5.43 */

  ECMA_DATE_PROTOTYPE_GET_TIME, /* ECMA-262 v5, 15.9.5.9 */
  ECMA_DATE_PROTOTYPE_SET_TIME, /* ECMA-262 v5, 15.9.5.27 */
  ECMA_DATE_PROTOTYPE_TO_JSON, /* ECMA-262 v5, 15.9.5.44 */

  ECMA_DATE_PROTOTYPE_TO_PRIMITIVE, /*  ECMA-262 v6 20.3.4.45 */
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-date-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  date_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup dateprototype ECMA Date.prototype object built-in
 * @{
 */

/**
 * The Date.prototype object's 'toJSON' routine
 *
 * See also:
 *          ECMA-262 v5, 15.9.5.44
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_date_prototype_to_json (ecma_value_t this_arg) /**< this argument */
{
  /* 1. */
  ecma_value_t obj = ecma_op_to_object (this_arg);

  if (ECMA_IS_VALUE_ERROR (obj))
  {
    return obj;
  }

  /* 2. */
  ecma_value_t tv = ecma_op_to_primitive (obj, ECMA_PREFERRED_TYPE_NUMBER);

  if (ECMA_IS_VALUE_ERROR (tv))
  {
    ecma_free_value (obj);
    return tv;
  }

  /* 3. */
  if (ecma_is_value_number (tv))
  {
    ecma_number_t num_value = ecma_get_number_from_value (tv);

    ecma_free_value (tv);

    if (ecma_number_is_nan (num_value) || ecma_number_is_infinity (num_value))
    {
      ecma_free_value (obj);
      return ECMA_VALUE_NULL;
    }
  }
  else
  {
    ecma_free_value (tv);
  }

  ecma_object_t *value_obj_p = ecma_get_object_from_value (obj);

  /* 4. */
  ecma_value_t ret_value = ecma_op_invoke_by_magic_id (obj, LIT_MAGIC_STRING_TO_ISO_STRING_UL, NULL, 0);

  ecma_deref_object (value_obj_p);

  return ret_value;
} /* ecma_builtin_date_prototype_to_json */

/**
 * The Date.prototype object's toPrimitive routine
 *
 * See also:
 *          ECMA-262 v6, 20.3.4.45
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_date_prototype_to_primitive (ecma_value_t this_arg, /**< this argument */
                                          ecma_value_t hint_arg) /**< {"default", "number", "string"} */
{
  if (ecma_is_value_object (this_arg) && ecma_is_value_string (hint_arg))
  {
    ecma_string_t *hint_str_p = ecma_get_string_from_value (hint_arg);

    ecma_preferred_type_hint_t hint = ECMA_PREFERRED_TYPE_NO;

    if (hint_str_p == ecma_get_magic_string (LIT_MAGIC_STRING_STRING)
        || hint_str_p == ecma_get_magic_string (LIT_MAGIC_STRING_DEFAULT))
    {
      hint = ECMA_PREFERRED_TYPE_STRING;
    }
    else if (hint_str_p == ecma_get_magic_string (LIT_MAGIC_STRING_NUMBER))
    {
      hint = ECMA_PREFERRED_TYPE_NUMBER;
    }

    if (hint != ECMA_PREFERRED_TYPE_NO)
    {
      return ecma_op_general_object_ordinary_value (ecma_get_object_from_value (this_arg), hint);
    }
  }

  return ecma_raise_type_error (ECMA_ERR_INVALID_ARGUMENT_TYPE_IN_TOPRIMITIVE);
} /* ecma_builtin_date_prototype_to_primitive */

/**
 * Dispatch get date functions
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_date_prototype_dispatch_get (uint16_t builtin_routine_id, /**< built-in wide routine
                                                                        *   identifier */
                                          ecma_number_t date_value) /**< date converted to number */
{
  if (ecma_number_is_nan (date_value))
  {
    return ecma_make_nan_value ();
  }

  int32_t result;

  switch (builtin_routine_id)
  {
    case ECMA_DATE_PROTOTYPE_GET_FULL_YEAR:
    case ECMA_DATE_PROTOTYPE_GET_UTC_FULL_YEAR:
    {
      result = ecma_date_year_from_time (date_value);
      break;
    }
#if JERRY_BUILTIN_ANNEXB
    case ECMA_DATE_PROTOTYPE_GET_YEAR:
    {
      result = (ecma_date_year_from_time (date_value) - 1900);
      break;
    }
#endif /* JERRY_BUILTIN_ANNEXB */
    case ECMA_DATE_PROTOTYPE_GET_MONTH:
    case ECMA_DATE_PROTOTYPE_GET_UTC_MONTH:
    {
      result = ecma_date_month_from_time (date_value);
      break;
    }
    case ECMA_DATE_PROTOTYPE_GET_DATE:
    case ECMA_DATE_PROTOTYPE_GET_UTC_DATE:
    {
      result = ecma_date_date_from_time (date_value);
      break;
    }
    case ECMA_DATE_PROTOTYPE_GET_DAY:
    case ECMA_DATE_PROTOTYPE_GET_UTC_DAY:
    {
      result = ecma_date_week_day (date_value);
      break;
    }
    case ECMA_DATE_PROTOTYPE_GET_HOURS:
    case ECMA_DATE_PROTOTYPE_GET_UTC_HOURS:
    {
      result = ecma_date_hour_from_time (date_value);
      break;
    }
    case ECMA_DATE_PROTOTYPE_GET_MINUTES:
    case ECMA_DATE_PROTOTYPE_GET_UTC_MINUTES:
    {
      result = ecma_date_min_from_time (date_value);
      break;
    }
    case ECMA_DATE_PROTOTYPE_GET_SECONDS:
    case ECMA_DATE_PROTOTYPE_GET_UTC_SECONDS:
    {
      result = ecma_date_sec_from_time (date_value);
      break;
    }
    case ECMA_DATE_PROTOTYPE_GET_MILLISECONDS:
    case ECMA_DATE_PROTOTYPE_GET_UTC_MILLISECONDS:
    {
      result = ecma_date_ms_from_time (date_value);
      break;
    }
    default:
    {
      JERRY_ASSERT (builtin_routine_id == ECMA_DATE_PROTOTYPE_GET_UTC_TIMEZONE_OFFSET);

      result = -ecma_date_local_time_zone_adjustment (date_value) / ECMA_DATE_MS_PER_MINUTE;
      break;
    }
  }

  return ecma_make_int32_value (result);
} /* ecma_builtin_date_prototype_dispatch_get */

#if JERRY_BUILTIN_ANNEXB

/**
 * Returns true, if the built-in id sets a year.
 */
#define ECMA_DATE_PROTOTYPE_IS_SET_YEAR_ROUTINE(builtin_routine_id) \
  ((builtin_routine_id) == ECMA_DATE_PROTOTYPE_SET_FULL_YEAR        \
   || (builtin_routine_id) == ECMA_DATE_PROTOTYPE_SET_UTC_FULL_YEAR \
   || (builtin_routine_id) == ECMA_DATE_PROTOTYPE_SET_YEAR)

#else /* !JERRY_BUILTIN_ANNEXB */

/**
 * Returns true, if the built-in id sets a year.
 */
#define ECMA_DATE_PROTOTYPE_IS_SET_YEAR_ROUTINE(builtin_routine_id) \
  ((builtin_routine_id) == ECMA_DATE_PROTOTYPE_SET_FULL_YEAR        \
   || (builtin_routine_id) == ECMA_DATE_PROTOTYPE_SET_UTC_FULL_YEAR)

#endif /* JERRY_BUILTIN_ANNEXB */

/**
 * Dispatch set date functions
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_date_prototype_dispatch_set (uint16_t builtin_routine_id, /**< built-in wide routine
                                                                        *   identifier */
                                          ecma_object_t *object_p, /**< date object */
                                          const ecma_value_t arguments_list[], /**< list of arguments
                                                                                *   passed to routine */
                                          uint32_t arguments_number) /**< length of arguments' list */
{
  ecma_number_t converted_number[4];
  uint32_t conversions = 0;

  /* If the first argument is not specified, it is always converted to NaN. */
  converted_number[0] = ecma_number_make_nan ();

  switch (builtin_routine_id)
  {
#if JERRY_BUILTIN_ANNEXB
    case ECMA_DATE_PROTOTYPE_SET_YEAR:
#endif /* JERRY_BUILTIN_ANNEXB */
    case ECMA_DATE_PROTOTYPE_SET_DATE:
    case ECMA_DATE_PROTOTYPE_SET_UTC_DATE:
    case ECMA_DATE_PROTOTYPE_SET_UTC_MILLISECONDS:
    case ECMA_DATE_PROTOTYPE_SET_MILLISECONDS:
    {
      conversions = 1;
      break;
    }
    case ECMA_DATE_PROTOTYPE_SET_MONTH:
    case ECMA_DATE_PROTOTYPE_SET_UTC_MONTH:
    case ECMA_DATE_PROTOTYPE_SET_UTC_SECONDS:
    case ECMA_DATE_PROTOTYPE_SET_SECONDS:
    {
      conversions = 2;
      break;
    }
    case ECMA_DATE_PROTOTYPE_SET_FULL_YEAR:
    case ECMA_DATE_PROTOTYPE_SET_UTC_FULL_YEAR:
    case ECMA_DATE_PROTOTYPE_SET_MINUTES:
    case ECMA_DATE_PROTOTYPE_SET_UTC_MINUTES:
    {
      conversions = 3;
      break;
    }
    default:
    {
      JERRY_ASSERT (builtin_routine_id == ECMA_DATE_PROTOTYPE_SET_HOURS
                    || builtin_routine_id == ECMA_DATE_PROTOTYPE_SET_UTC_HOURS);

      conversions = 4;
      break;
    }
  }

  if (conversions > arguments_number)
  {
    conversions = arguments_number;
  }

  for (uint32_t i = 0; i < conversions; i++)
  {
    ecma_value_t value = ecma_op_to_number (arguments_list[i], &converted_number[i]);

    if (ECMA_IS_VALUE_ERROR (value))
    {
      return value;
    }
  }

  ecma_date_object_t *date_object_p = (ecma_date_object_t *) object_p;
  ecma_number_t *date_value_p = &date_object_p->date_value;
  ecma_number_t date_value = *date_value_p;

  if (!BUILTIN_DATE_FUNCTION_IS_UTC (builtin_routine_id))
  {
    ecma_number_t local_tza;

    if (date_object_p->header.u.cls.u1.date_flags & ECMA_DATE_TZA_SET)
    {
      local_tza = date_object_p->header.u.cls.u3.tza;
      JERRY_ASSERT (local_tza == ecma_date_local_time_zone_adjustment (date_value));
    }
    else
    {
      local_tza = ecma_date_local_time_zone_adjustment (date_value);
    }

    date_value += local_tza;
  }

  ecma_number_t day_part;
  ecma_number_t time_part;

  if (builtin_routine_id <= ECMA_DATE_PROTOTYPE_SET_UTC_DATE)
  {
    if (ecma_number_is_nan (date_value))
    {
      if (!ECMA_DATE_PROTOTYPE_IS_SET_YEAR_ROUTINE (builtin_routine_id))
      {
        return ecma_make_number_value (date_value);
      }

      date_value = ECMA_NUMBER_ZERO;
    }

    time_part = ecma_date_time_in_day_from_time (date_value);

    ecma_number_t year = ecma_date_year_from_time (date_value);
    ecma_number_t month = ecma_date_month_from_time (date_value);
    ecma_number_t day = ecma_date_date_from_time (date_value);

    switch (builtin_routine_id)
    {
      case ECMA_DATE_PROTOTYPE_SET_FULL_YEAR:
      case ECMA_DATE_PROTOTYPE_SET_UTC_FULL_YEAR:
      {
        year = converted_number[0];
        if (conversions >= 2)
        {
          month = converted_number[1];
        }
        if (conversions >= 3)
        {
          day = converted_number[2];
        }
        break;
      }
#if JERRY_BUILTIN_ANNEXB
      case ECMA_DATE_PROTOTYPE_SET_YEAR:
      {
        if (ecma_number_is_nan (converted_number[0]))
        {
          *date_value_p = converted_number[0];
          date_object_p->header.u.cls.u1.date_flags &= (uint8_t) ~ECMA_DATE_TZA_SET;
          return ecma_make_number_value (converted_number[0]);
        }

        year = ecma_number_trunc (converted_number[0]);
        if (year >= 0 && year <= 99)
        {
          year += 1900;
        }
        break;
      }
#endif /* JERRY_BUILTIN_ANNEXB */
      case ECMA_DATE_PROTOTYPE_SET_MONTH:
      case ECMA_DATE_PROTOTYPE_SET_UTC_MONTH:
      {
        month = converted_number[0];
        if (conversions >= 2)
        {
          day = converted_number[1];
        }
        break;
      }
      default:
      {
        JERRY_ASSERT (builtin_routine_id == ECMA_DATE_PROTOTYPE_SET_DATE
                      || builtin_routine_id == ECMA_DATE_PROTOTYPE_SET_UTC_DATE);

        day = converted_number[0];
        break;
      }
    }

    day_part = ecma_date_make_day (year, month, day);

#if JERRY_BUILTIN_ANNEXB
    if (builtin_routine_id == ECMA_DATE_PROTOTYPE_SET_YEAR)
    {
      if (ecma_number_is_nan (converted_number[0]))
      {
        day_part = 0;
        time_part = converted_number[0];
      }
    }
#endif /* JERRY_BUILTIN_ANNEXB */
  }
  else
  {
    if (ecma_number_is_nan (date_value))
    {
      return ecma_make_number_value (date_value);
    }

    day_part = ecma_date_day_from_time (date_value) * (ecma_number_t) ECMA_DATE_MS_PER_DAY;

    ecma_number_t hour = ecma_date_hour_from_time (date_value);
    ecma_number_t min = ecma_date_min_from_time (date_value);
    ecma_number_t sec = ecma_date_sec_from_time (date_value);
    ecma_number_t ms = ecma_date_ms_from_time (date_value);

    switch (builtin_routine_id)
    {
      case ECMA_DATE_PROTOTYPE_SET_HOURS:
      case ECMA_DATE_PROTOTYPE_SET_UTC_HOURS:
      {
        hour = converted_number[0];
        if (conversions >= 2)
        {
          min = converted_number[1];
        }
        if (conversions >= 3)
        {
          sec = converted_number[2];
        }
        if (conversions >= 4)
        {
          ms = converted_number[3];
        }
        break;
      }
      case ECMA_DATE_PROTOTYPE_SET_MINUTES:
      case ECMA_DATE_PROTOTYPE_SET_UTC_MINUTES:
      {
        min = converted_number[0];
        if (conversions >= 2)
        {
          sec = converted_number[1];
        }
        if (conversions >= 3)
        {
          ms = converted_number[2];
        }
        break;
      }
      case ECMA_DATE_PROTOTYPE_SET_UTC_SECONDS:
      case ECMA_DATE_PROTOTYPE_SET_SECONDS:
      {
        sec = converted_number[0];
        if (conversions >= 2)
        {
          ms = converted_number[1];
        }
        break;
      }
      default:
      {
        JERRY_ASSERT (builtin_routine_id == ECMA_DATE_PROTOTYPE_SET_UTC_MILLISECONDS
                      || builtin_routine_id == ECMA_DATE_PROTOTYPE_SET_MILLISECONDS);

        ms = converted_number[0];
        break;
      }
    }

    time_part = ecma_date_make_time (hour, min, sec, ms);
  }

  bool is_utc = BUILTIN_DATE_FUNCTION_IS_UTC (builtin_routine_id);

  ecma_number_t full_date = ecma_date_make_date (day_part, time_part);

  if (!is_utc)
  {
    full_date = ecma_date_utc (full_date);
  }

  full_date = ecma_date_time_clip (full_date);

  *date_value_p = full_date;

  date_object_p->header.u.cls.u1.date_flags &= (uint8_t) ~ECMA_DATE_TZA_SET;

  return ecma_make_number_value (full_date);
} /* ecma_builtin_date_prototype_dispatch_set */

#undef ECMA_DATE_PROTOTYPE_IS_SET_YEAR_ROUTINE

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_date_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine
                                                                           *   identifier */
                                              ecma_value_t this_arg, /**< 'this' argument value */
                                              const ecma_value_t arguments_list[], /**< list of arguments
                                                                                    *   passed to routine */
                                              uint32_t arguments_number) /**< length of arguments' list */
{
  if (JERRY_UNLIKELY (builtin_routine_id == ECMA_DATE_PROTOTYPE_TO_JSON))
  {
    return ecma_builtin_date_prototype_to_json (this_arg);
  }

  if (JERRY_UNLIKELY (builtin_routine_id == ECMA_DATE_PROTOTYPE_TO_PRIMITIVE))
  {
    return ecma_builtin_date_prototype_to_primitive (this_arg, arguments_list[0]);
  }

  if (!ecma_is_value_object (this_arg)
      || !ecma_object_class_is (ecma_get_object_from_value (this_arg), ECMA_OBJECT_CLASS_DATE))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_DATE_OBJECT);
  }

  ecma_object_t *this_obj_p = ecma_get_object_from_value (this_arg);

  ecma_date_object_t *date_object_p = (ecma_date_object_t *) this_obj_p;
  ecma_number_t *date_value_p = &date_object_p->date_value;
  ecma_number_t date_value = *date_value_p;

  if (builtin_routine_id == ECMA_DATE_PROTOTYPE_GET_TIME)
  {
    return ecma_make_number_value (date_value);
  }

  if (builtin_routine_id == ECMA_DATE_PROTOTYPE_SET_TIME)
  {
    ecma_number_t time_num;

    if (ECMA_IS_VALUE_ERROR (ecma_op_to_number (arguments_list[0], &time_num)))
    {
      return ECMA_VALUE_ERROR;
    }

    *date_value_p = ecma_date_time_clip (time_num);
    date_object_p->header.u.cls.u1.date_flags &= (uint8_t) ~ECMA_DATE_TZA_SET;

    return ecma_make_number_value (*date_value_p);
  }

  if (builtin_routine_id <= ECMA_DATE_PROTOTYPE_SET_UTC_MILLISECONDS)
  {
    if (builtin_routine_id <= ECMA_DATE_PROTOTYPE_GET_UTC_TIMEZONE_OFFSET)
    {
      if (!BUILTIN_DATE_FUNCTION_IS_UTC (builtin_routine_id))
      {
        ecma_number_t local_tza;

        if (date_object_p->header.u.cls.u1.date_flags & ECMA_DATE_TZA_SET)
        {
          local_tza = date_object_p->header.u.cls.u3.tza;
          JERRY_ASSERT (local_tza == ecma_date_local_time_zone_adjustment (date_value));
        }
        else
        {
          local_tza = ecma_date_local_time_zone_adjustment (date_value);
          JERRY_ASSERT (local_tza <= INT32_MAX && local_tza >= INT32_MIN);
          date_object_p->header.u.cls.u3.tza = (int32_t) local_tza;
          date_object_p->header.u.cls.u1.date_flags |= ECMA_DATE_TZA_SET;
        }

        date_value += local_tza;
      }

      return ecma_builtin_date_prototype_dispatch_get (builtin_routine_id, date_value);
    }

    return ecma_builtin_date_prototype_dispatch_set (builtin_routine_id, this_obj_p, arguments_list, arguments_number);
  }

  if (builtin_routine_id == ECMA_DATE_PROTOTYPE_TO_ISO_STRING)
  {
    if (ecma_number_is_nan (date_value))
    {
      return ecma_raise_range_error (ECMA_ERR_DATE_MUST_BE_A_FINITE_NUMBER);
    }

    return ecma_date_value_to_iso_string (date_value);
  }

  if (ecma_number_is_nan (date_value))
  {
    return ecma_make_magic_string_value (LIT_MAGIC_STRING_INVALID_DATE_UL);
  }

  switch (builtin_routine_id)
  {
    case ECMA_DATE_PROTOTYPE_TO_STRING:
    {
      return ecma_date_value_to_string (date_value);
    }
    case ECMA_DATE_PROTOTYPE_TO_DATE_STRING:
    {
      return ecma_date_value_to_date_string (date_value);
    }
    default:
    {
      JERRY_ASSERT (builtin_routine_id == ECMA_DATE_PROTOTYPE_TO_TIME_STRING);

      return ecma_date_value_to_time_string (date_value);
    }
  }
} /* ecma_builtin_date_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_DATE */
