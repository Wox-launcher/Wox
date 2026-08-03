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

#include "ecma-array-object.h"
#include "ecma-arraybuffer-object.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-container-object.h"
#include "ecma-gc.h"
#include "ecma-helpers.h"
#include "ecma-string-object.h"
#include "ecma-typedarray-object.h"

#include "lit-char-helpers.h"

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
  ECMA_INTRINSIC_ROUTINE_START = 0,
  ECMA_INTRINSIC_ARRAY_PROTOTYPE_VALUES,
  ECMA_INTRINSIC_TYPEDARRAY_PROTOTYPE_VALUES,
  ECMA_INTRINSIC_MAP_PROTOTYPE_ENTRIES,
  ECMA_INTRINSIC_SET_PROTOTYPE_VALUES,
  ECMA_INTRINSIC_ARRAY_TO_STRING,
  ECMA_INTRINSIC_DATE_TO_UTC_STRING,
  ECMA_INTRINSIC_PARSE_FLOAT,
  ECMA_INTRINSIC_PARSE_INT,
  ECMA_INTRINSIC_STRING_TRIM_START,
  ECMA_INTRINSIC_STRING_TRIM_END,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-intrinsic.inc.h"
#define BUILTIN_UNDERSCORED_ID  intrinsic
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup intrinsic ECMA Intrinsic object built-in
 * @{
 */

/**
 * The %ArrayProto_values% intrinsic routine
 *
 * See also:
 *          ECMA-262 v5, 15.4.4.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_intrinsic_array_prototype_values (ecma_value_t this_value) /**< this argument */
{
  ecma_value_t this_obj = ecma_op_to_object (this_value);

  if (ECMA_IS_VALUE_ERROR (this_obj))
  {
    return this_obj;
  }

  ecma_object_t *this_obj_p = ecma_get_object_from_value (this_obj);

  ecma_value_t ret_value = ecma_op_create_array_iterator (this_obj_p, ECMA_ITERATOR_VALUES);

  ecma_deref_object (this_obj_p);

  return ret_value;
} /* ecma_builtin_intrinsic_array_prototype_values */

/**
 * The Map.prototype entries and [@@iterator] routines
 *
 * See also:
 *          ECMA-262 v6, 23.1.3.4
 *          ECMA-262 v6, 23.1.3.12
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_intrinsic_map_prototype_entries (ecma_value_t this_value) /**< this value */
{
  ecma_extended_object_t *map_object_p = ecma_op_container_get_object (this_value, LIT_MAGIC_STRING_MAP_UL);

  if (map_object_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  return ecma_op_container_create_iterator (this_value,
                                            ECMA_BUILTIN_ID_MAP_ITERATOR_PROTOTYPE,
                                            ECMA_OBJECT_CLASS_MAP_ITERATOR,
                                            ECMA_ITERATOR_ENTRIES);
} /* ecma_builtin_intrinsic_map_prototype_entries */

/**
 * The Set.prototype values, keys and [@@iterator] routines
 *
 * See also:
 *          ECMA-262 v6, 23.2.3.8
 *          ECMA-262 v6, 23.2.3.10
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_intrinsic_set_prototype_values (ecma_value_t this_value) /**< this value */
{
  ecma_extended_object_t *map_object_p = ecma_op_container_get_object (this_value, LIT_MAGIC_STRING_SET_UL);

  if (map_object_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  return ecma_op_container_create_iterator (this_value,
                                            ECMA_BUILTIN_ID_SET_ITERATOR_PROTOTYPE,
                                            ECMA_OBJECT_CLASS_SET_ITERATOR,
                                            ECMA_ITERATOR_VALUES);
} /* ecma_builtin_intrinsic_set_prototype_values */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_intrinsic_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                         ecma_value_t this_arg, /**< 'this' argument value */
                                         const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                 *   passed to routine */
                                         uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (arguments_number);

  switch (builtin_routine_id)
  {
#if JERRY_BUILTIN_ARRAY
    case ECMA_INTRINSIC_ARRAY_PROTOTYPE_VALUES:
    {
      return ecma_builtin_intrinsic_array_prototype_values (this_arg);
    }
    case ECMA_INTRINSIC_ARRAY_TO_STRING:
    {
      ecma_value_t this_obj = ecma_op_to_object (this_arg);
      if (ECMA_IS_VALUE_ERROR (this_obj))
      {
        return this_obj;
      }

      ecma_value_t result = ecma_array_object_to_string (this_obj);
      ecma_deref_object (ecma_get_object_from_value (this_obj));

      return result;
    }
#endif /* JERRY_BUILTIN_ARRAY */
#if JERRY_BUILTIN_TYPEDARRAY
    case ECMA_INTRINSIC_TYPEDARRAY_PROTOTYPE_VALUES:
    {
      if (!ecma_is_typedarray (this_arg))
      {
        return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_TYPED_ARRAY);
      }

      if (ecma_arraybuffer_is_detached (ecma_typedarray_get_arraybuffer (ecma_get_object_from_value (this_arg))))
      {
        return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
      }

      return ecma_typedarray_iterators_helper (this_arg, ECMA_ITERATOR_VALUES);
    }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_BUILTIN_CONTAINER
    case ECMA_INTRINSIC_SET_PROTOTYPE_VALUES:
    {
      return ecma_builtin_intrinsic_set_prototype_values (this_arg);
    }
    case ECMA_INTRINSIC_MAP_PROTOTYPE_ENTRIES:
    {
      return ecma_builtin_intrinsic_map_prototype_entries (this_arg);
    }
#endif /* JERRY_BUILTIN_CONTAINER */
#if JERRY_BUILTIN_DATE
    case ECMA_INTRINSIC_DATE_TO_UTC_STRING:
    {
      if (!ecma_is_value_object (this_arg)
          || !ecma_object_class_is (ecma_get_object_from_value (this_arg), ECMA_OBJECT_CLASS_DATE))
      {
        return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_DATE_OBJECT);
      }

      ecma_number_t *date_value_p = &((ecma_date_object_t *) ecma_get_object_from_value (this_arg))->date_value;

      if (ecma_number_is_nan (*date_value_p))
      {
        return ecma_make_magic_string_value (LIT_MAGIC_STRING_INVALID_DATE_UL);
      }

      return ecma_date_value_to_utc_string (*date_value_p);
    }
#endif /* JERRY_BUILTIN_DATE */
#if JERRY_BUILTIN_STRING
    case ECMA_INTRINSIC_STRING_TRIM_START:
    case ECMA_INTRINSIC_STRING_TRIM_END:
    {
      if (!ecma_op_require_object_coercible (this_arg))
      {
        return ECMA_VALUE_ERROR;
      }

      ecma_string_t *to_str_p = ecma_op_to_string (this_arg);
      if (to_str_p == NULL)
      {
        return ECMA_VALUE_ERROR;
      }

      ECMA_STRING_TO_UTF8_STRING (to_str_p, start_p, input_start_size);

      lit_utf8_size_t size;
      const lit_utf8_byte_t *input_start_p = start_p;
      const lit_utf8_byte_t *input_str_end_p = start_p + input_start_size;

      ecma_string_t *ret_str_p;
      if (builtin_routine_id == ECMA_INTRINSIC_STRING_TRIM_START)
      {
        const lit_utf8_byte_t *new_start_p = ecma_string_trim_front (input_start_p, input_str_end_p);
        size = (lit_utf8_size_t) (input_str_end_p - new_start_p);
        ret_str_p = ecma_new_ecma_string_from_utf8 (new_start_p, size);
      }
      else
      {
        const lit_utf8_byte_t *new_end_p = ecma_string_trim_back (input_start_p, input_str_end_p);
        size = (lit_utf8_size_t) (new_end_p - input_start_p);
        ret_str_p = ecma_new_ecma_string_from_utf8 (input_start_p, size);
      }

      ECMA_FINALIZE_UTF8_STRING (start_p, input_start_size);
      ecma_value_t result = ecma_make_string_value (ret_str_p);
      ecma_deref_ecma_string (to_str_p);
      return result;
    }
#endif /* JERRY_BUILTIN_STRING */
    default:
    {
      JERRY_ASSERT (builtin_routine_id == ECMA_INTRINSIC_PARSE_INT || builtin_routine_id == ECMA_INTRINSIC_PARSE_FLOAT);

      ecma_string_t *str_p = ecma_op_to_string (arguments_list_p[0]);

      if (JERRY_UNLIKELY (str_p == NULL))
      {
        return ECMA_VALUE_ERROR;
      }

      ecma_value_t result;
      ECMA_STRING_TO_UTF8_STRING (str_p, string_buff, string_buff_size);

      if (builtin_routine_id == ECMA_INTRINSIC_PARSE_INT)
      {
        result = ecma_number_parse_int (string_buff, string_buff_size, arguments_list_p[1]);
      }
      else
      {
        JERRY_ASSERT (builtin_routine_id == ECMA_INTRINSIC_PARSE_FLOAT);
        result = ecma_number_parse_float (string_buff, string_buff_size);
      }

      ECMA_FINALIZE_UTF8_STRING (string_buff, string_buff_size);
      ecma_deref_ecma_string (str_p);
      return result;
    }
  }
} /* ecma_builtin_intrinsic_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
