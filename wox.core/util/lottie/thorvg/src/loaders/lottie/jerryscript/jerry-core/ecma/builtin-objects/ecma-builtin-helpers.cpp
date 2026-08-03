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

#include "ecma-builtin-helpers.h"

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-helpers-number.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"

#include "jmem.h"
#include "lit-char-helpers.h"
#include "lit-magic-strings.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltinhelpers ECMA builtin helper operations
 * @{
 */

/**
 * Helper function for Object.prototype.toString routine when
 * the @@toStringTag property is present
 *
 * See also:
 *          ECMA-262 v6, 19.1.3.6
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_helper_object_to_string_tag_helper (ecma_value_t tag_value) /**< string tag */
{
  JERRY_ASSERT (ecma_is_value_string (tag_value));

  ecma_string_t *tag_str_p = ecma_get_string_from_value (tag_value);
  lit_utf8_size_t tag_str_size = ecma_string_get_size (tag_str_p);
  ecma_string_t *ret_string_p;

  /* Building string "[object #@@toStringTag#]"
     The string size will be size("[object ") + size(#@@toStringTag#) + size ("]"). */
  const lit_utf8_size_t buffer_size = 9 + tag_str_size;
  JMEM_DEFINE_LOCAL_ARRAY (str_buffer, buffer_size, lit_utf8_byte_t);

  lit_utf8_byte_t *buffer_ptr = str_buffer;

  const lit_magic_string_id_t magic_string_ids[] = {
    LIT_MAGIC_STRING_LEFT_SQUARE_CHAR,
    LIT_MAGIC_STRING_OBJECT,
    LIT_MAGIC_STRING_SPACE_CHAR,
  };

  /* Copy to buffer the "[object " string */
  for (uint32_t i = 0; i < sizeof (magic_string_ids) / sizeof (lit_magic_string_id_t); ++i)
  {
    buffer_ptr = lit_copy_magic_string_to_buffer (magic_string_ids[i],
                                                  buffer_ptr,
                                                  (lit_utf8_size_t) ((str_buffer + buffer_size) - buffer_ptr));

    JERRY_ASSERT (buffer_ptr <= str_buffer + buffer_size);
  }

  /* Copy to buffer the #@@toStringTag# string */
  ecma_string_to_cesu8_bytes (tag_str_p, buffer_ptr, tag_str_size);
  buffer_ptr += tag_str_size;

  JERRY_ASSERT (buffer_ptr <= str_buffer + buffer_size);

  /* Copy to buffer the "]" string */
  buffer_ptr = lit_copy_magic_string_to_buffer (LIT_MAGIC_STRING_RIGHT_SQUARE_CHAR,
                                                buffer_ptr,
                                                (lit_utf8_size_t) ((str_buffer + buffer_size) - buffer_ptr));

  JERRY_ASSERT (buffer_ptr <= str_buffer + buffer_size);

  ret_string_p = ecma_new_ecma_string_from_utf8 (str_buffer, (lit_utf8_size_t) (buffer_ptr - str_buffer));

  JMEM_FINALIZE_LOCAL_ARRAY (str_buffer);
  ecma_deref_ecma_string (tag_str_p);

  return ecma_make_string_value (ret_string_p);
} /* ecma_builtin_helper_object_to_string_tag_helper */

/**
 * Common implementation of the Object.prototype.toString routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.4.2
 *
 * Used by:
 *         - The Object.prototype.toString routine.
 *         - The Array.prototype.toString routine as fallback.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */

ecma_value_t
ecma_builtin_helper_object_to_string (const ecma_value_t this_arg) /**< this argument */
{
  lit_magic_string_id_t builtin_tag;

  if (ecma_is_value_undefined (this_arg))
  {
    builtin_tag = LIT_MAGIC_STRING_UNDEFINED_UL;
  }
  else if (ecma_is_value_null (this_arg))
  {
    builtin_tag = LIT_MAGIC_STRING_NULL_UL;
  }
  else
  {
    ecma_value_t obj_this = ecma_op_to_object (this_arg);

    if (ECMA_IS_VALUE_ERROR (obj_this))
    {
      return obj_this;
    }

    JERRY_ASSERT (ecma_is_value_object (obj_this));

    ecma_object_t *obj_p = ecma_get_object_from_value (obj_this);

    builtin_tag = ecma_object_get_class_name (obj_p);

    ecma_value_t is_array = ecma_is_value_array (obj_this);

    if (ECMA_IS_VALUE_ERROR (is_array))
    {
      ecma_deref_object (obj_p);
      return is_array;
    }

    if (ecma_is_value_true (is_array))
    {
      builtin_tag = LIT_MAGIC_STRING_ARRAY_UL;
    }

    ecma_value_t tag = ecma_op_object_get_by_symbol_id (obj_p, LIT_GLOBAL_SYMBOL_TO_STRING_TAG);

    if (ECMA_IS_VALUE_ERROR (tag))
    {
      ecma_deref_object (obj_p);
      return tag;
    }

    if (ecma_is_value_string (tag))
    {
      ecma_deref_object (obj_p);
      return ecma_builtin_helper_object_to_string_tag_helper (tag);
    }
    else if (builtin_tag != LIT_MAGIC_STRING_ARGUMENTS_UL && builtin_tag != LIT_MAGIC_STRING_FUNCTION_UL
             && builtin_tag != LIT_MAGIC_STRING_ERROR_UL && builtin_tag != LIT_MAGIC_STRING_BOOLEAN_UL
             && builtin_tag != LIT_MAGIC_STRING_NUMBER_UL && builtin_tag != LIT_MAGIC_STRING_STRING_UL
             && builtin_tag != LIT_MAGIC_STRING_DATE_UL && builtin_tag != LIT_MAGIC_STRING_REGEXP_UL
             && builtin_tag != LIT_MAGIC_STRING_ARRAY_UL)
    {
      builtin_tag = LIT_MAGIC_STRING_OBJECT_UL;
    }

    ecma_free_value (tag);
    ecma_deref_object (obj_p);
  }

  ecma_stringbuilder_t builder = ecma_stringbuilder_create ();

  ecma_stringbuilder_append_magic (&builder, LIT_MAGIC_STRING_OBJECT_TO_STRING_UL);
  ecma_stringbuilder_append_magic (&builder, builtin_tag);
  ecma_stringbuilder_append_byte (&builder, LIT_CHAR_RIGHT_SQUARE);

  return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
} /* ecma_builtin_helper_object_to_string */

/**
 * The Array.prototype's 'toLocaleString' single element operation routine
 *
 * See also:
 *          ECMA-262 v5, 15.4.4.3 steps 6-8 and 10.b-d
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_string_t *
ecma_builtin_helper_get_to_locale_string_at_index (ecma_object_t *obj_p, /**< this object */
                                                   ecma_length_t index) /**< array index */
{
  ecma_value_t index_value = ecma_op_object_get_by_index (obj_p, index);

  if (ECMA_IS_VALUE_ERROR (index_value))
  {
    return NULL;
  }

  if (ecma_is_value_undefined (index_value) || ecma_is_value_null (index_value))
  {
    return ecma_get_magic_string (LIT_MAGIC_STRING__EMPTY);
  }

  ecma_value_t call_value = ecma_op_invoke_by_magic_id (index_value, LIT_MAGIC_STRING_TO_LOCALE_STRING_UL, NULL, 0);

  ecma_free_value (index_value);

  if (ECMA_IS_VALUE_ERROR (call_value))
  {
    return NULL;
  }

  ecma_string_t *ret_string_p = ecma_op_to_string (call_value);

  ecma_free_value (call_value);

  return ret_string_p;
} /* ecma_builtin_helper_get_to_locale_string_at_index */

/**
 * Helper function to normalizing an array index
 *
 * See also:
 *          ECMA-262 v5, 15.4.4.10 steps 5, 6, 7 part 2, 8
 *          ECMA-262 v5, 15.4.4.12 steps 5, 6
 *
 *          ECMA-262 v6, 22.1.3.6 steps 5 - 7, 8 part 2, 9, 10
 *          ECMA-262 v6, 22.1.3.3 steps 5 - 10, 11 part 2, 12, 13
 *
 * Used by:
 *         - The Array.prototype.slice routine.
 *         - The Array.prototype.splice routine.
 *         - The Array.prototype.fill routine.
 *         - The Array.prototype.copyWithin routine.
 *
 * @return ECMA_VALUE_EMPTY if successful
 *         conversion error otherwise
 */
ecma_value_t
ecma_builtin_helper_array_index_normalize (ecma_value_t arg, /**< index */
                                           ecma_length_t length, /**< array's length */
                                           ecma_length_t *number_p) /**< [out] ecma_length_t */
{
  ecma_number_t to_int;

  if (ECMA_IS_VALUE_ERROR (ecma_op_to_integer (arg, &to_int)))
  {
    return ECMA_VALUE_ERROR;
  }

  *number_p = ((to_int < 0) ? (ecma_length_t) JERRY_MAX (((ecma_number_t) length + to_int), 0)
                            : (ecma_length_t) JERRY_MIN (to_int, (ecma_number_t) length));

  return ECMA_VALUE_EMPTY;
} /* ecma_builtin_helper_array_index_normalize */

/**
 * Helper function to normalizing an uint32 index
 *
 * See also:
 *          ECMA-262 v5, 15.5.4.13 steps 4 - 7
 *          ECMA-262 v6, 22.2.3.5 steps 5 - 10, 11 part 2, 12, 13
 *          ECMA-262 v6, 22.2.3.23 steps 5 - 10
 *          ECMA-262 v6, 24.1.4.3 steps 6 - 8, 9 part 2, 10, 11
 *          ECMA-262 v6, 22.2.3.26 steps 7 - 9, 10 part 2, 11, 12
 *          ECMA-262 v6, 22.2.3.8 steps 5 - 7, 8 part 2, 9, 10
 *
 * Used by:
 *         - The String.prototype.slice routine.
 *         - The Array.prototype.copyWithin routine.
 *         - The TypedArray.prototype.copyWithin routine.
 *         - The TypedArray.prototype.slice routine.
 *         - The ArrayBuffer.prototype.slice routine.
 *         - The TypedArray.prototype.subarray routine.
 *         - The TypedArray.prototype.fill routine.
 *
 * @return ECMA_VALUE_EMPTY if successful
 *         conversion error otherwise
 */
ecma_value_t
ecma_builtin_helper_uint32_index_normalize (ecma_value_t arg, /**< index */
                                            uint32_t length, /**< array's length */
                                            uint32_t *number_p) /**< [out] uint32_t number */
{
  ecma_number_t to_int;

  if (ECMA_IS_VALUE_ERROR (ecma_op_to_integer (arg, &to_int)))
  {
    return ECMA_VALUE_ERROR;
  }

  *number_p = ((to_int < 0) ? (uint32_t) JERRY_MAX ((ecma_number_t) length + to_int, 0)
                            : (uint32_t) JERRY_MIN (to_int, (ecma_number_t) length));

  return ECMA_VALUE_EMPTY;
} /* ecma_builtin_helper_uint32_index_normalize */

/**
 * Helper function for concatenating an ecma_value_t to an Array.
 *
 * See also:
 *          ECMA-262 v5, 15.4.4.4 steps 5.b - 5.c
 *
 * Used by:
 *         - The Array.prototype.concat routine.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_helper_array_concat_value (ecma_object_t *array_obj_p, /**< array */
                                        ecma_length_t *length_p, /**< [in,out] array's length */
                                        ecma_value_t value) /**< value to concat */
{
  /* 5.b */
  ecma_value_t is_spreadable = ecma_op_is_concat_spreadable (value);

  if (ECMA_IS_VALUE_ERROR (is_spreadable))
  {
    return is_spreadable;
  }

  bool spread_object = is_spreadable == ECMA_VALUE_TRUE;
  /* ES11: 22.1.3.1.5.c.iv.3.b */
  const uint32_t prop_flags = ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE | JERRY_PROP_SHOULD_THROW;

  if (spread_object)
  {
    ecma_object_t *obj_p = ecma_get_object_from_value (value);

    ecma_length_t arg_len;
    ecma_value_t error = ecma_op_object_get_length (obj_p, &arg_len);

    if (ECMA_IS_VALUE_ERROR (error))
    {
      return error;
    }

    /* 4 . */
    if ((ecma_number_t) (*length_p + arg_len) > ECMA_NUMBER_MAX_SAFE_INTEGER)
    {
      return ecma_raise_type_error (ECMA_ERR_INVALID_ARRAY_LENGTH);
    }

    /* 5.b.iii */
    for (ecma_length_t array_index = 0; array_index < arg_len; array_index++)
    {
      /* 5.b.iii.2 */
      ecma_value_t get_value = ecma_op_object_find_by_index (obj_p, array_index);

      if (ECMA_IS_VALUE_ERROR (get_value))
      {
        return get_value;
      }

      if (!ecma_is_value_found (get_value))
      {
        continue;
      }

      /* 5.b.iii.3.b */
      /* This will always be a simple value since 'is_throw' is false, so no need to free. */
      ecma_value_t put_comp =
        ecma_builtin_helper_def_prop_by_index (array_obj_p, *length_p + array_index, get_value, prop_flags);
      ecma_free_value (get_value);

      if (ECMA_IS_VALUE_ERROR (put_comp))
      {
        return put_comp;
      }
    }

    *length_p += arg_len;
    return ECMA_VALUE_EMPTY;
  }

  /* 5.c.i */
  /* This will always be a simple value since 'is_throw' is false, so no need to free. */
  ecma_value_t put_comp = ecma_builtin_helper_def_prop_by_index (array_obj_p, (*length_p)++, value, prop_flags);

  if (ECMA_IS_VALUE_ERROR (put_comp))
  {
    return put_comp;
  }

  return ECMA_VALUE_EMPTY;
} /* ecma_builtin_helper_array_concat_value */

/**
 * Helper function to normalizing a string index
 *
 * This function clamps the given index to the [0, length] range.
 * If the index is negative, 0 value is used.
 * If the index is greater than the length of the string, the normalized index will be the length of the string.
 * NaN is mapped to zero or length depending on the nan_to_zero parameter.
 *
 * See also:
 *          ECMA-262 v5, 15.5.4.15
 *
 * Used by:
 *         - The String.prototype.substring routine.
 *         - The ecma_builtin_helper_string_prototype_object_index_of helper routine.
 *
 * @return lit_utf8_size_t - the normalized value of the index
 */
lit_utf8_size_t
ecma_builtin_helper_string_index_normalize (ecma_number_t index, /**< index */
                                            lit_utf8_size_t length, /**< string's length */
                                            bool nan_to_zero) /**< whether NaN is mapped to zero (t) or length (f) */
{
  uint32_t norm_index = 0;

  if (ecma_number_is_nan (index))
  {
    if (!nan_to_zero)
    {
      norm_index = length;
    }
  }
  else if (!ecma_number_is_negative (index))
  {
    if (ecma_number_is_infinity (index))
    {
      norm_index = length;
    }
    else
    {
      norm_index = ecma_number_to_uint32 (index);

      if (norm_index > length)
      {
        norm_index = length;
      }
    }
  }

  return norm_index;
} /* ecma_builtin_helper_string_index_normalize */

/**
 * Helper function for finding lastindex of a search string
 *
 * This function clamps the given index to the [0, length] range.
 * If the index is negative, 0 value is used.
 * If the index is greater than the length of the string, the normalized index will be the length of the string.
 *
 * See also:
 *          ECMA-262 v6, 21.1.3.9
 *
 * Used by:
 *         - The ecma_builtin_helper_string_prototype_object_index_of helper routine.
 *         - The ecma_builtin_string_prototype_object_replace_match helper routine.
 *
 * @return uint32_t - whether there is a match for the search string
 */
static uint32_t
ecma_builtin_helper_string_find_last_index (ecma_string_t *original_str_p, /**< original string */
                                            ecma_string_t *search_str_p, /**< search string */
                                            uint32_t position) /**< start_position */
{
  if (ecma_string_is_empty (search_str_p))
  {
    return position;
  }

  uint32_t original_length = ecma_string_get_length (original_str_p);

  ECMA_STRING_TO_UTF8_STRING (search_str_p, search_str_utf8_p, search_str_size);
  ECMA_STRING_TO_UTF8_STRING (original_str_p, original_str_utf8_p, original_str_size);

  uint32_t ret_value = UINT32_MAX;

  if (original_str_size >= search_str_size)
  {
    const lit_utf8_byte_t *end_p = original_str_utf8_p + original_str_size;
    const lit_utf8_byte_t *current_p = end_p;

    for (auto i = original_length; i > position; i--)
    {
      lit_utf8_decr (&current_p);
    }

    while (current_p + search_str_size > end_p)
    {
      lit_utf8_decr (&current_p);
      position--;
    }

    while (true)
    {
      if (memcmp (current_p, search_str_utf8_p, search_str_size) == 0)
      {
        ret_value = position;
        break;
      }

      if (position == 0)
      {
        break;
      }

      lit_utf8_decr (&current_p);
      position--;
    }
  }
  ECMA_FINALIZE_UTF8_STRING (original_str_utf8_p, original_str_size);
  ECMA_FINALIZE_UTF8_STRING (search_str_utf8_p, search_str_size);

  return ret_value;
} /* ecma_builtin_helper_string_find_last_index */

/**
 * Helper function for string indexOf, lastIndexOf, startsWith, includes, endsWith functions
 *
 * See also:
 *          ECMA-262 v5, 15.5.4.7
 *          ECMA-262 v5, 15.5.4.8
 *          ECMA-262 v6, 21.1.3.6
 *          ECMA-262 v6, 21.1.3.7
 *          ECMA-262 v6, 21.1.3.18
 *
 * Used by:
 *         - The String.prototype.indexOf routine.
 *         - The String.prototype.lastIndexOf routine.
 *         - The String.prototype.startsWith routine.
 *         - The String.prototype.includes routine.
 *         - The String.prototype.endsWith routine.
 *
 * @return ecma_value_t - Returns index (last index) or a
 *                        boolean value
 */
ecma_value_t
ecma_builtin_helper_string_prototype_object_index_of (ecma_string_t *original_str_p, /**< this argument */
                                                      ecma_value_t arg1, /**< routine's first argument */
                                                      ecma_value_t arg2, /**< routine's second argument */
                                                      ecma_string_index_of_mode_t mode) /**< routine's mode */
{
  /* 5 (indexOf) -- 6 (lastIndexOf) */
  const lit_utf8_size_t original_len = ecma_string_get_length (original_str_p);

  /* 4, 6 (startsWith, includes, endsWith) */
  if (mode >= ECMA_STRING_STARTS_WITH)
  {
    ecma_value_t regexp = ecma_op_is_regexp (arg1);

    if (ECMA_IS_VALUE_ERROR (regexp))
    {
      return regexp;
    }

    if (regexp == ECMA_VALUE_TRUE)
    {
      JERRY_ASSERT (ECMA_STRING_LAST_INDEX_OF < mode && mode <= ECMA_STRING_ENDS_WITH);
      return ecma_raise_type_error (ECMA_ERR_SEARCH_STRING_CANNOT_BE_OF_TYPE_REGEXP);
    }
  }

  /* 7, 8 */
  ecma_string_t *search_str_p = ecma_op_to_string (arg1);

  if (JERRY_UNLIKELY (search_str_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t ret_value;

  /* 4 (indexOf, lastIndexOf), 9 (startsWith, includes), 10 (endsWith) */
  ecma_number_t pos_num;

  if (mode > ECMA_STRING_LAST_INDEX_OF)
  {
    ret_value = ecma_op_to_integer (arg2, &pos_num);
  }
  else
  {
    ret_value = ecma_op_to_number (arg2, &pos_num);
  }

  /* 10 (startsWith, includes), 11 (endsWith) */
  if (ECMA_IS_VALUE_ERROR (ret_value))
  {
    ecma_deref_ecma_string (search_str_p);
    return ret_value;
  }

  bool use_first_index = mode != ECMA_STRING_LAST_INDEX_OF;

  /* 4b, 6 (indexOf) - 4b, 5, 7 (lastIndexOf) */
  lit_utf8_size_t start = ecma_builtin_helper_string_index_normalize (pos_num, original_len, use_first_index);

  ecma_number_t ret_num = ECMA_NUMBER_MINUS_ONE;

  ret_value = ECMA_VALUE_FALSE;

  switch (mode)
  {
    case ECMA_STRING_STARTS_WITH:
    {
      if (start > original_len)
      {
        break;
      }
      /* 15, 16 (startsWith) */
      uint32_t index = ecma_builtin_helper_string_find_index (original_str_p, search_str_p, start);
      ret_value = ecma_make_boolean_value (index == start);
      break;
    }
    case ECMA_STRING_INCLUDES:
    {
      if (ecma_builtin_helper_string_find_index (original_str_p, search_str_p, start) != UINT32_MAX)
      {
        ret_value = ECMA_VALUE_TRUE;
      }
      break;
    }
    case ECMA_STRING_ENDS_WITH:
    {
      if (start == 0)
      {
        start = original_len;
      }

      lit_utf8_size_t search_str_len = ecma_string_get_length (search_str_p);

      if (search_str_len == 0)
      {
        ret_value = ECMA_VALUE_TRUE;
        break;
      }

      int32_t start_ends_with = (int32_t) (start - search_str_len);

      if (start_ends_with < 0)
      {
        break;
      }
      uint32_t index = ecma_builtin_helper_string_find_index (original_str_p, search_str_p, (uint32_t) start_ends_with);
      ret_value = ecma_make_boolean_value (index == (uint32_t) start_ends_with);
      break;
    }
    case ECMA_STRING_INDEX_OF:
    {
      /* 8 (indexOf) -- 9 (lastIndexOf) */
      ecma_value_t find_index = ecma_builtin_helper_string_find_index (original_str_p, search_str_p, start);

      if (find_index != UINT32_MAX)
      {
        ret_num = ((ecma_number_t) find_index);
      }
      ret_value = ecma_make_number_value (ret_num);
      break;
    }

    case ECMA_STRING_LAST_INDEX_OF:
    {
      uint32_t index = ecma_builtin_helper_string_find_last_index (original_str_p, search_str_p, start);

      if (index != UINT32_MAX)
      {
        ret_num = ((ecma_number_t) index);
      }
      ret_value = ecma_make_number_value (ret_num);
      break;
    }

    default:
    {
      JERRY_UNREACHABLE ();
    }
  }

  ecma_deref_ecma_string (search_str_p);

  return ret_value;
} /* ecma_builtin_helper_string_prototype_object_index_of */

/**
 * Helper function for finding index of a search string
 *
 * This function clamps the given index to the [0, length] range.
 * If the index is negative, 0 value is used.
 * If the index is greater than the length of the string, the normalized index will be the length of the string.
 *
 * See also:
 *          ECMA-262 v6, 21.1.3.8
 *
 * Used by:
 *         - The ecma_builtin_helper_string_prototype_object_index_of helper routine.
 *         - The ecma_builtin_string_prototype_object_replace_match helper routine.
 *
 * @return uint32_t - whether there is a match for the search string
 */
uint32_t
ecma_builtin_helper_string_find_index (ecma_string_t *original_str_p, /**< index */
                                       ecma_string_t *search_str_p, /**< string's length */
                                       uint32_t start_pos) /**< start position */
{
  uint32_t match_found = UINT32_MAX;

  if (ecma_string_is_empty (search_str_p))
  {
    return start_pos;
  }

  ECMA_STRING_TO_UTF8_STRING (search_str_p, search_str_utf8_p, search_str_size);
  ECMA_STRING_TO_UTF8_STRING (original_str_p, original_str_utf8_p, original_str_size);

  const lit_utf8_byte_t *str_current_p = original_str_utf8_p;

  for (ecma_number_t i = 0; i < start_pos; i++)
  {
    lit_utf8_incr (&str_current_p);
  }

  const lit_utf8_byte_t *original_end_p = original_str_utf8_p + original_str_size;

  while (!((size_t) (original_end_p - str_current_p) < search_str_size))
  {
    if (memcmp (str_current_p, search_str_utf8_p, search_str_size) == 0)
    {
      match_found = start_pos;
      break;
    }

    lit_utf8_incr (&str_current_p);
    start_pos++;
  }

  ECMA_FINALIZE_UTF8_STRING (original_str_utf8_p, original_str_size);
  ECMA_FINALIZE_UTF8_STRING (search_str_utf8_p, search_str_size);

  return match_found;
} /* ecma_builtin_helper_string_find_index */

/**
 * Helper function for using [[DefineOwnProperty]] specialized for indexed property names
 *
 * Note: this method falls back to the general ecma_builtin_helper_def_prop
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_helper_def_prop_by_index (ecma_object_t *obj_p, /**< object */
                                       ecma_length_t index, /**< property index */
                                       ecma_value_t value, /**< value */
                                       uint32_t opts) /**< any combination of ecma_property_flag_t bits */
{
  if (JERRY_LIKELY (index <= ECMA_DIRECT_STRING_MAX_IMM))
  {
    return ecma_builtin_helper_def_prop (obj_p, ECMA_CREATE_DIRECT_UINT32_STRING (index), value, opts);
  }

  ecma_string_t *index_str_p = ecma_new_ecma_string_from_length (index);
  ecma_value_t ret_value = ecma_builtin_helper_def_prop (obj_p, index_str_p, value, opts);
  ecma_deref_ecma_string (index_str_p);

  return ret_value;
} /* ecma_builtin_helper_def_prop_by_index */

/**
 * Helper function for at() functions.
 *
 * See also:
 *          ECMA-262 Stage 3 Draft Relative Indexing Method proposal 3. 4. 5. 6.
 *
 * Used by:
 *         - The Array.prototype.at routine.
 *         - The String.prototype.at routine.
 *         - The TypedArray.prototype.at routine.
 *
 * @return ECMA_VALUE_ERROR - on conversion error
 *         ECMA_VALUE_UNDEFINED - if the requested index is not exist
 *         ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
ecma_builtin_helper_calculate_index (ecma_value_t index, /**< relative index argument */
                                     ecma_length_t length, /**< object's length */
                                     ecma_length_t *out_index) /**< calculated index */
{
  JERRY_ASSERT (out_index != NULL);

  ecma_number_t relative_index;
  ecma_value_t conversion_result = ecma_op_to_integer (index, &relative_index);

  /* 4. */
  if (ECMA_IS_VALUE_ERROR (conversion_result))
  {
    return ECMA_VALUE_ERROR;
  }

  /* 5. 6. */
  ecma_number_t k;

  if (relative_index >= 0)
  {
    k = relative_index;
  }
  else
  {
    k = ((ecma_number_t) length + relative_index);
  }

  /* 7. */
  if (k < 0 || k >= ((ecma_number_t) length))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  *out_index = (ecma_length_t) k;

  return ECMA_VALUE_EMPTY;
} /* ecma_builtin_helper_calculate_index */

/**
 * Helper function for using [[DefineOwnProperty]].
 *
 * See also:
 *          ECMA-262 v5, 8.12.9
 *          ECMA-262 v5, 15.4.5.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_helper_def_prop (ecma_object_t *obj_p, /**< object */
                              ecma_string_t *name_p, /**< name string */
                              ecma_value_t value, /**< value */
                              uint32_t opts) /**< any combination of ecma_property_descriptor_status_flags_t bits */
{
  ecma_property_descriptor_t prop_desc;

  prop_desc.flags = (uint16_t) (ECMA_NAME_DATA_PROPERTY_DESCRIPTOR_BITS | opts);

  prop_desc.value = value;

  return ecma_op_object_define_own_property (obj_p, name_p, &prop_desc);
} /* ecma_builtin_helper_def_prop */

/**
 * GetSubstitution abstract operation
 *
 * See:
 *     ECMA-262 v6.0 21.1.3.14.1
 */
void
ecma_builtin_replace_substitute (ecma_replace_context_t *ctx_p) /**< replace context */
{
  JERRY_ASSERT (ctx_p->string_p != NULL);
  JERRY_ASSERT (ctx_p->matched_p == NULL
                || (ctx_p->matched_p >= ctx_p->string_p && ctx_p->matched_p <= ctx_p->string_p + ctx_p->string_size));

  lit_utf8_size_t replace_size;
  uint8_t replace_flags = ECMA_STRING_FLAG_IS_ASCII;
  const lit_utf8_byte_t *replace_buf_p =
    ecma_string_get_chars (ctx_p->replace_str_p, &replace_size, NULL, NULL, &replace_flags);

  const lit_utf8_byte_t *const replace_end_p = replace_buf_p + replace_size;
  const lit_utf8_byte_t *curr_p = replace_buf_p;
  const lit_utf8_byte_t *last_inserted_end_p = replace_buf_p;

  while (curr_p < replace_end_p)
  {
    if (*curr_p++ == LIT_CHAR_DOLLAR_SIGN)
    {
      ecma_stringbuilder_append_raw (&(ctx_p->builder),
                                     last_inserted_end_p,
                                     (lit_utf8_size_t) (curr_p - last_inserted_end_p - 1));
      if (curr_p >= replace_end_p)
      {
        last_inserted_end_p = curr_p - 1;
        break;
      }

      const lit_utf8_byte_t c = *curr_p++;

      switch (c)
      {
        case LIT_CHAR_DOLLAR_SIGN:
        {
          ecma_stringbuilder_append_byte (&(ctx_p->builder), LIT_CHAR_DOLLAR_SIGN);
          break;
        }
        case LIT_CHAR_AMPERSAND:
        {
          if (JERRY_UNLIKELY (ctx_p->matched_p == NULL))
          {
            JERRY_ASSERT (ctx_p->capture_count == 0);
            JERRY_ASSERT (ctx_p->u.collection_p != NULL);
            JERRY_ASSERT (ctx_p->u.collection_p->item_count > 0);
            const ecma_value_t match_value = ctx_p->u.collection_p->buffer_p[0];

            JERRY_ASSERT (ecma_is_value_string (match_value));
            ecma_stringbuilder_append (&(ctx_p->builder), ecma_get_string_from_value (match_value));
            break;
          }

          JERRY_ASSERT (ctx_p->matched_p != NULL);
          ecma_stringbuilder_append_raw (&(ctx_p->builder), ctx_p->matched_p, ctx_p->matched_size);
          break;
        }
        case LIT_CHAR_GRAVE_ACCENT:
        {
          ecma_stringbuilder_append_raw (&(ctx_p->builder), ctx_p->string_p, ctx_p->match_byte_pos);
          break;
        }
        case LIT_CHAR_SINGLE_QUOTE:
        {
          if (JERRY_UNLIKELY (ctx_p->matched_p == NULL))
          {
            JERRY_ASSERT (ctx_p->capture_count == 0);
            JERRY_ASSERT (ctx_p->u.collection_p != NULL);
            JERRY_ASSERT (ctx_p->u.collection_p->item_count > 0);
            const ecma_value_t match_value = ctx_p->u.collection_p->buffer_p[0];

            JERRY_ASSERT (ecma_is_value_string (match_value));
            const ecma_string_t *const matched_p = ecma_get_string_from_value (match_value);
            const lit_utf8_size_t match_size = ecma_string_get_size (matched_p);
            const lit_utf8_byte_t *const begin_p = ctx_p->string_p + ctx_p->match_byte_pos + match_size;

            ecma_stringbuilder_append_raw (&(ctx_p->builder),
                                           begin_p,
                                           (lit_utf8_size_t) (ctx_p->string_p + ctx_p->string_size - begin_p));
            break;
          }

          JERRY_ASSERT (ctx_p->matched_p != NULL);
          ecma_stringbuilder_append_raw (&(ctx_p->builder),
                                         ctx_p->matched_p + ctx_p->matched_size,
                                         ctx_p->string_size - ctx_p->match_byte_pos - ctx_p->matched_size);
          break;
        }
        default:
        {
          const lit_utf8_byte_t *const number_begin_p = curr_p - 1;

          if (lit_char_is_decimal_digit (c))
          {
            uint32_t capture_count = ctx_p->capture_count;

            if (capture_count == 0 && ctx_p->u.collection_p != NULL)
            {
              capture_count = ctx_p->u.collection_p->item_count;
            }

            uint8_t idx = (uint8_t) (c - LIT_CHAR_0);
            if (curr_p < replace_end_p && lit_char_is_decimal_digit (*(curr_p)))
            {
              uint8_t two_digit_index = (uint8_t) (idx * 10 + (uint8_t) (*(curr_p) -LIT_CHAR_0));
              if (two_digit_index < capture_count)
              {
                idx = two_digit_index;
                curr_p++;
              }
            }

            if (idx > 0 && idx < capture_count)
            {
              if (ctx_p->capture_count > 0)
              {
#if JERRY_BUILTIN_REGEXP
                JERRY_ASSERT (ctx_p->u.captures_p != NULL);
                const ecma_regexp_capture_t *const capture_p = ctx_p->u.captures_p + idx;

                if (ECMA_RE_IS_CAPTURE_DEFINED (capture_p))
                {
                  ecma_stringbuilder_append_raw (&(ctx_p->builder),
                                                 capture_p->begin_p,
                                                 (lit_utf8_size_t) (capture_p->end_p - capture_p->begin_p));
                }

                break;
#endif /* JERRY_BUILTIN_REGEXP */
              }
              else if (ctx_p->u.collection_p != NULL)
              {
                const ecma_value_t capture_value = ctx_p->u.collection_p->buffer_p[idx];
                if (!ecma_is_value_undefined (capture_value))
                {
                  ecma_stringbuilder_append (&(ctx_p->builder), ecma_get_string_from_value (capture_value));
                }

                break;
              }
            }
          }

          ecma_stringbuilder_append_byte (&(ctx_p->builder), LIT_CHAR_DOLLAR_SIGN);
          curr_p = number_begin_p;
          break;
        }
      }

      last_inserted_end_p = curr_p;
    }
  }

  ecma_stringbuilder_append_raw (&(ctx_p->builder),
                                 last_inserted_end_p,
                                 (lit_utf8_size_t) (replace_end_p - last_inserted_end_p));

  if (replace_flags & ECMA_STRING_FLAG_MUST_BE_FREED)
  {
    jmem_heap_free_block ((void *) replace_buf_p, replace_size);
  }
} /* ecma_builtin_replace_substitute */

/**
 * Helper function to determine if method is the builtin exec method
 *
 * @return true, if function is the builtin exec method
 *         false, otherwise
 */
bool 
ecma_builtin_is_regexp_exec (ecma_extended_object_t *obj_p) /**< function object */
{
  return (ecma_get_object_type (&obj_p->object) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION
          && obj_p->u.built_in.routine_id == ECMA_REGEXP_PROTOTYPE_ROUTINE_EXEC);
} /* ecma_builtin_is_regexp_exec */

/**
 * @}
 * @}
 */
