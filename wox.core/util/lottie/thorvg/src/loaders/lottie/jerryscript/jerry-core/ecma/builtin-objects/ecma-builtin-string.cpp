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

#include "ecma-alloc.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-string-object.h"
#include "ecma-symbol-object.h"

#include "jrt.h"
#include "lit-strings.h"

#if JERRY_BUILTIN_STRING

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
  ECMA_BUILTIN_STRING_ROUTINE_START = 0,
  ECMA_BUILTIN_STRING_OBJECT_FROM_CHAR_CODE,
  ECMA_BUILTIN_STRING_OBJECT_FROM_CODE_POINT,
  ECMA_BUILTIN_STRING_OBJECT_RAW,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-string.inc.h"
#define BUILTIN_UNDERSCORED_ID  string
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup string ECMA String object built-in
 * @{
 */

/**
 * The String object's 'fromCharCode' routine
 *
 * See also:
 *          ECMA-262 v5, 15.5.3.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_string_object_from_char_code (const ecma_value_t args[], /**< arguments list */
                                           uint32_t args_number) /**< number of arguments */
{
  if (args_number == 0)
  {
    return ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY);
  }

  lit_utf8_size_t utf8_buf_size = args_number * LIT_CESU8_MAX_BYTES_IN_CODE_UNIT;
  ecma_string_t *ret_string_p = NULL;
  bool isError = false;

  JMEM_DEFINE_LOCAL_ARRAY (utf8_buf_p, utf8_buf_size, lit_utf8_byte_t);

  lit_utf8_size_t utf8_buf_used = 0;

  for (uint32_t arg_index = 0; arg_index < args_number; arg_index++)
  {
    ecma_number_t arg_num;

    if (ECMA_IS_VALUE_ERROR (ecma_op_to_number (args[arg_index], &arg_num)))
    {
      isError = true;
      break;
    }

    uint32_t uint32_char_code = ecma_number_to_uint32 (arg_num);
    ecma_char_t code_unit = (uint16_t) uint32_char_code;

    JERRY_ASSERT (utf8_buf_used <= utf8_buf_size - LIT_UTF8_MAX_BYTES_IN_CODE_UNIT);
    utf8_buf_used += lit_code_unit_to_utf8 (code_unit, utf8_buf_p + utf8_buf_used);
    JERRY_ASSERT (utf8_buf_used <= utf8_buf_size);
  }

  if (!isError)
  {
    ret_string_p = ecma_new_ecma_string_from_utf8 (utf8_buf_p, utf8_buf_used);
  }

  JMEM_FINALIZE_LOCAL_ARRAY (utf8_buf_p);

  return isError ? ECMA_VALUE_ERROR : ecma_make_string_value (ret_string_p);
} /* ecma_builtin_string_object_from_char_code */

/**
 * The String object's 'raw' routine
 *
 * See also:
 *          ECMA-262 v6, 21.1.2.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_string_object_raw (const ecma_value_t args[], /**< arguments list */
                                uint32_t args_number) /**< number of arguments */
{
  /* 1 - 2. */
  const ecma_value_t *substitutions;
  uint32_t number_of_substitutions;
  ecma_length_t next_index;
  ecma_stringbuilder_t builder;

  if (args_number > 1)
  {
    substitutions = args + 1;
    number_of_substitutions = args_number - 1;
  }
  else
  {
    substitutions = NULL;
    number_of_substitutions = 0;
  }

  /* 3. */
  ecma_value_t templated = args_number > 0 ? args[0] : ECMA_VALUE_UNDEFINED;

  ecma_value_t cooked = ecma_op_to_object (templated);

  /* 4. */
  if (ECMA_IS_VALUE_ERROR (cooked))
  {
    return cooked;
  }

  ecma_object_t *cooked_obj_p = ecma_get_object_from_value (cooked);

  /* 5. */
  ecma_value_t raw = ecma_op_object_get_by_magic_id (cooked_obj_p, LIT_MAGIC_STRING_RAW);

  ecma_deref_object (cooked_obj_p);

  if (ECMA_IS_VALUE_ERROR (raw))
  {
    return raw;
  }

  ecma_value_t raw_obj = ecma_op_to_object (raw);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (raw_obj))
  {
    ecma_free_value (raw);
    return raw_obj;
  }

  ecma_object_t *raw_obj_p = ecma_get_object_from_value (raw_obj);

  ecma_value_t ret_value = ECMA_VALUE_ERROR;

  /* 7 - 8. */
  ecma_length_t literal_segments;
  if (ECMA_IS_VALUE_ERROR (ecma_op_object_get_length (raw_obj_p, &literal_segments)))
  {
    goto cleanup;
  }

  /* 9. */
  if (literal_segments == 0)
  {
    ret_value = ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY);
    goto cleanup;
  }

  /* 10. */
  builder = ecma_stringbuilder_create ();

  /* 11. */
  next_index = 0;

  /* 12. */
  while (true)
  {
    /* 12.a,b */
    ecma_value_t next_seg = ecma_op_object_get_by_index (raw_obj_p, next_index);

    if (ECMA_IS_VALUE_ERROR (next_seg))
    {
      goto builder_cleanup;
    }

    ecma_string_t *next_seg_srt_p = ecma_op_to_string (next_seg);

    /* 12.c */
    if (JERRY_UNLIKELY (next_seg_srt_p == NULL))
    {
      ecma_free_value (next_seg);
      goto builder_cleanup;
    }

    /* 12.d */
    ecma_stringbuilder_append (&builder, next_seg_srt_p);

    ecma_deref_ecma_string (next_seg_srt_p);
    ecma_free_value (next_seg);

    /* 12.e */
    if (next_index + 1 == literal_segments)
    {
      ret_value = ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
      goto cleanup;
    }

    /* 12.f-g */
    if (next_index >= number_of_substitutions)
    {
      next_index++;
      continue;
    }

    /* 12.h */
    ecma_string_t *next_sub_p = ecma_op_to_string (substitutions[next_index]);

    /* 12.i */
    if (JERRY_UNLIKELY (next_sub_p == NULL))
    {
      goto builder_cleanup;
    }

    /* 12.j */
    ecma_stringbuilder_append (&builder, next_sub_p);
    ecma_deref_ecma_string (next_sub_p);

    /* 12.k */
    next_index++;
  }

builder_cleanup:
  ecma_stringbuilder_destroy (&builder);

cleanup:
  ecma_deref_object (raw_obj_p);
  ecma_free_value (raw);

  return ret_value;
} /* ecma_builtin_string_object_raw */

/**
 * The String object's 'fromCodePoint' routine
 *
 * See also:
 *          ECMA-262 v6, 21.1.2.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_string_object_from_code_point (const ecma_value_t args[], /**< arguments list */
                                            uint32_t args_number) /**< number of arguments */
{
  if (args_number == 0)
  {
    return ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY);
  }

  ecma_stringbuilder_t builder = ecma_stringbuilder_create ();

  for (uint32_t index = 0; index < args_number; index++)
  {
    ecma_number_t to_number_num;
    ecma_value_t to_number_value = ecma_op_to_number (args[index], &to_number_num);

    if (ECMA_IS_VALUE_ERROR (to_number_value))
    {
      ecma_stringbuilder_destroy (&builder);
      return to_number_value;
    }

    if (!ecma_op_is_integer (to_number_num))
    {
      ecma_stringbuilder_destroy (&builder);
      return ecma_raise_range_error (ECMA_ERR_INVALID_CODE_POINT_ERROR);
    }

    ecma_free_value (to_number_value);

    if (to_number_num < 0 || to_number_num > LIT_UNICODE_CODE_POINT_MAX)
    {
      ecma_stringbuilder_destroy (&builder);
      return ecma_raise_range_error (ECMA_ERR_INVALID_CODE_POINT);
    }

    lit_code_point_t code_point = (lit_code_point_t) to_number_num;

    ecma_char_t converted_cp[2];
    uint8_t encoded_size = lit_utf16_encode_code_point (code_point, converted_cp);

    for (uint8_t i = 0; i < encoded_size; i++)
    {
      ecma_stringbuilder_append_char (&builder, converted_cp[i]);
    }
  }

  ecma_string_t *ret_str_p = ecma_stringbuilder_finalize (&builder);

  return ecma_make_string_value (ret_str_p);
} /* ecma_builtin_string_object_from_code_point */

/**
 * Handle calling [[Call]] of built-in String object
 *
 * See also:
 *          ECMA-262 v6, 21.1.1.1
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_string_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                   uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  ecma_value_t ret_value = ECMA_VALUE_EMPTY;

  /* 1. */
  if (arguments_list_len == 0)
  {
    ret_value = ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY);
  }
  /* 2.a */
  else if (ecma_is_value_symbol (arguments_list_p[0]))
  {
    ret_value = ecma_get_symbol_descriptive_string (arguments_list_p[0]);
  }
  /* 2.b */
  else
  {
    ecma_string_t *str_p = ecma_op_to_string (arguments_list_p[0]);
    if (JERRY_UNLIKELY (str_p == NULL))
    {
      return ECMA_VALUE_ERROR;
    }

    ret_value = ecma_make_string_value (str_p);
  }

  return ret_value;
} /* ecma_builtin_string_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in String object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_string_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                        uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_op_create_string_object (arguments_list_p, arguments_list_len);
} /* ecma_builtin_string_dispatch_construct */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_string_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                      ecma_value_t this_arg, /**< 'this' argument value */
                                      const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                              *   passed to routine */
                                      uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (this_arg);

  switch (builtin_routine_id)
  {
    case ECMA_BUILTIN_STRING_OBJECT_FROM_CHAR_CODE:
    {
      return ecma_builtin_string_object_from_char_code (arguments_list_p, arguments_number);
    }
    case ECMA_BUILTIN_STRING_OBJECT_FROM_CODE_POINT:
    {
      return ecma_builtin_string_object_from_code_point (arguments_list_p, arguments_number);
    }
    case ECMA_BUILTIN_STRING_OBJECT_RAW:
    {
      return ecma_builtin_string_object_raw (arguments_list_p, arguments_number);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_string_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_STRING */
