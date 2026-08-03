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

#ifndef ECMA_BUILTIN_HELPERS_H
#define ECMA_BUILTIN_HELPERS_H

#include "ecma-exceptions.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-regexp-object.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltinhelpers ECMA builtin helper operations
 * @{
 */

/**
 * List of built-in routine identifiers.
 */
enum
{
  /** These routines must be in this order */
  ECMA_REGEXP_PROTOTYPE_ROUTINE_START = 0,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_EXEC,
#if JERRY_BUILTIN_ANNEXB
  ECMA_REGEXP_PROTOTYPE_ROUTINE_COMPILE,
#endif /* JERRY_BUILTIN_ANNEXB */

  ECMA_REGEXP_PROTOTYPE_ROUTINE_TEST,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_TO_STRING,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_GET_SOURCE,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_GET_FLAGS,

  ECMA_REGEXP_PROTOTYPE_ROUTINE_GET_GLOBAL,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_GET_IGNORE_CASE,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_GET_MULTILINE,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_GET_STICKY,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_GET_UNICODE,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_GET_DOT_ALL,

  ECMA_REGEXP_PROTOTYPE_ROUTINE_SYMBOL_SEARCH,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_SYMBOL_MATCH,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_SYMBOL_REPLACE,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_SYMBOL_SPLIT,
  ECMA_REGEXP_PROTOTYPE_ROUTINE_SYMBOL_MATCH_ALL,
};

/**
 * Mode of string index routine.
 */
typedef enum
{
  /** These routines must be in this order */
  ECMA_STRING_LAST_INDEX_OF, /**< String.lastIndexOf: ECMA-262 v5, 15.5.4.8 */
  ECMA_STRING_INDEX_OF, /**< String.indexOf: ECMA-262 v5, 15.5.4.7 */
  ECMA_STRING_STARTS_WITH, /**< String.startsWith: ECMA-262 v6, 21.1.3.18 */
  ECMA_STRING_INCLUDES, /**< String.includes: ECMA-262 v6, 21.1.3.7 */
  ECMA_STRING_ENDS_WITH /**< String.includes: ECMA-262 v6, 21.1.3.6 */
} ecma_string_index_of_mode_t;

ecma_value_t ecma_builtin_helper_object_to_string (const ecma_value_t this_arg);
ecma_string_t *ecma_builtin_helper_get_to_locale_string_at_index (ecma_object_t *obj_p, ecma_length_t index);
ecma_value_t ecma_builtin_helper_array_concat_value (ecma_object_t *obj_p, ecma_length_t *length_p, ecma_value_t value);
ecma_value_t ecma_builtin_helper_uint32_index_normalize (ecma_value_t arg, uint32_t length, uint32_t *number_p);
ecma_value_t
ecma_builtin_helper_array_index_normalize (ecma_value_t arg, ecma_length_t length, ecma_length_t *number_p);
ecma_value_t ecma_builtin_helper_string_index_normalize (ecma_number_t index, uint32_t length, bool nan_to_zero);
ecma_value_t ecma_builtin_helper_string_prototype_object_index_of (ecma_string_t *original_str_p,
                                                                   ecma_value_t arg1,
                                                                   ecma_value_t arg2,
                                                                   ecma_string_index_of_mode_t mode);
uint32_t
ecma_builtin_helper_string_find_index (ecma_string_t *original_str_p, ecma_string_t *search_str_p, uint32_t start_pos);
ecma_value_t
ecma_builtin_helper_def_prop (ecma_object_t *obj_p, ecma_string_t *name_p, ecma_value_t value, uint32_t opts);

ecma_value_t
ecma_builtin_helper_def_prop_by_index (ecma_object_t *obj_p, ecma_length_t index, ecma_value_t value, uint32_t opts);
ecma_value_t ecma_builtin_helper_calculate_index (ecma_value_t index, ecma_length_t length, ecma_length_t *out_index);

/**
 * Context for replace substitutions
 */
typedef struct
{
  ecma_stringbuilder_t builder; /**< result string builder */
  const lit_utf8_byte_t *string_p; /**< source string */
  lit_utf8_size_t string_size; /**< source string size */
  const lit_utf8_byte_t *matched_p; /**< matched string */
  lit_utf8_size_t matched_size; /**< matched string size */
  lit_utf8_size_t match_byte_pos; /**< byte position of the match in the source string */
  uint16_t flags; /**< replace flags */

  /**
   * Capture results
   */
  union
  {
#if JERRY_BUILTIN_REGEXP
    const ecma_regexp_capture_t *captures_p; /**< array of regexp capturing groups */
#endif /* JERRY_BUILTIN_REGEXP */
    const ecma_collection_t *collection_p; /**< collection of captured substrings */
  } u;

  uint32_t capture_count; /**< number of captures in the capturing group array */
  ecma_string_t *replace_str_p; /**< replacement string */
} ecma_replace_context_t;

void ecma_builtin_replace_substitute (ecma_replace_context_t *ctx_p);
bool ecma_builtin_is_regexp_exec (ecma_extended_object_t *obj_p);

#if JERRY_BUILTIN_DATE

/**
 * Time range defines for helper functions.
 *
 * See also:
 *          ECMA-262 v5, 15.9.1.1, 15.9.1.10
 */

/** Hours in a day. */
#define ECMA_DATE_HOURS_PER_DAY (24)

/** Minutes in an hour. */
#define ECMA_DATE_MINUTES_PER_HOUR (60)

/** Seconds in a minute. */
#define ECMA_DATE_SECONDS_PER_MINUTE (60)

/** Milliseconds in a second. */
#define ECMA_DATE_MS_PER_SECOND (1000)

/** ECMA_DATE_MS_PER_MINUTE == 60000 */
#define ECMA_DATE_MS_PER_MINUTE (ECMA_DATE_MS_PER_SECOND * ECMA_DATE_SECONDS_PER_MINUTE)

/** ECMA_DATE_MS_PER_HOUR == 3600000 */
#define ECMA_DATE_MS_PER_HOUR (ECMA_DATE_MS_PER_MINUTE * ECMA_DATE_MINUTES_PER_HOUR)

/** ECMA_DATE_MS_PER_DAY == 86400000 */
#define ECMA_DATE_MS_PER_DAY ((ECMA_DATE_MS_PER_HOUR * ECMA_DATE_HOURS_PER_DAY))

#define ECMA_DATE_DAYS_IN_YEAR (365)

#define ECMA_DATE_DAYS_IN_LEAP_YEAR (366)

/**
 * This gives a range of 8,640,000,000,000,000 milliseconds
 * to either side of 01 January, 1970 UTC.
 */
#define ECMA_DATE_MAX_VALUE 8.64e15

/**
 * Timezone type.
 */
typedef enum
{
  ECMA_DATE_UTC, /**< date value is in UTC */
  ECMA_DATE_LOCAL /**< date value is in local time */
} ecma_date_timezone_t;

/* ecma-builtin-helpers-date.c */
extern const char* day_names_p[7];
extern const char* month_names_p[12];

int32_t ecma_date_day_from_time (ecma_number_t time);
int32_t ecma_date_year_from_time (ecma_number_t time);
int32_t ecma_date_month_from_time (ecma_number_t time);
int32_t ecma_date_date_from_time (ecma_number_t time);
int32_t ecma_date_week_day (ecma_number_t time);
int32_t ecma_date_hour_from_time (ecma_number_t time);
int32_t ecma_date_min_from_time (ecma_number_t time);
int32_t ecma_date_sec_from_time (ecma_number_t time);
int32_t ecma_date_ms_from_time (ecma_number_t time);
int32_t ecma_date_time_in_day_from_time (ecma_number_t time);

int32_t ecma_date_local_time_zone_adjustment (ecma_number_t time);
ecma_number_t ecma_date_utc (ecma_number_t time);
ecma_number_t ecma_date_make_time (ecma_number_t hour, ecma_number_t min, ecma_number_t sec, ecma_number_t ms);
ecma_number_t ecma_date_make_day (ecma_number_t year, ecma_number_t month, ecma_number_t date);
ecma_number_t ecma_date_make_date (ecma_number_t day, ecma_number_t time);
ecma_number_t ecma_date_time_clip (ecma_number_t time);

ecma_value_t ecma_date_value_to_string (ecma_number_t datetime_number);
ecma_value_t ecma_date_value_to_utc_string (ecma_number_t datetime_number);
ecma_value_t ecma_date_value_to_iso_string (ecma_number_t datetime_number);
ecma_value_t ecma_date_value_to_date_string (ecma_number_t datetime_number);
ecma_value_t ecma_date_value_to_time_string (ecma_number_t datetime_number);

#endif /* JERRY_BUILTIN_DATE */

/* ecma-builtin-helper-error.c */

ecma_value_t ecma_builtin_helper_error_dispatch_call (jerry_error_t error_type,
                                                      const ecma_value_t *arguments_list_p,
                                                      uint32_t arguments_list_len);

/* ecma-builtin-helpers-sort.c */

/**
 * Comparison callback function header for sorting helper routines.
 */
typedef ecma_value_t (*ecma_builtin_helper_sort_compare_fn_t) (ecma_value_t lhs, /**< left value */
                                                               ecma_value_t rhs, /**< right value */
                                                               ecma_value_t compare_func, /**< compare function */
                                                               ecma_object_t *array_buffer_p /**< arrayBuffer */);

ecma_value_t ecma_builtin_helper_array_merge_sort_helper (ecma_value_t *array_p,
                                                          uint32_t length,
                                                          ecma_value_t compare_func,
                                                          const ecma_builtin_helper_sort_compare_fn_t sort_cb,
                                                          ecma_object_t *array_buffer_p);

/**
 * @}
 * @}
 */

#endif /* !ECMA_BUILTIN_HELPERS_H */
