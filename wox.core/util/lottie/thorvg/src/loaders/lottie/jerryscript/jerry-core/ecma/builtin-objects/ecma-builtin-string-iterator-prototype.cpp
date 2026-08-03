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
#include "ecma-builtins.h"
#include "ecma-iterator-object.h"

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
  ECMA_BUILTIN_STRING_ITERATOR_PROTOTYPE_ROUTINE_START = 0,
  ECMA_BUILTIN_STRING_ITERATOR_PROTOTYPE_OBJECT_NEXT,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-string-iterator-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  string_iterator_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup %stringiteratorprototype% ECMA %ArrayIteratorPrototype% object built-in
 * @{
 */

/**
 * The %StringIteratorPrototype% object's 'next' routine
 *
 * See also:
 *          ECMA-262 v6, 22.1.5.2.1
 *
 * Note:
 *     Returned value must be freed with ecma_free_value.
 *
 * @return iterator result object, if success
 *         error - otherwise
 */
static ecma_value_t
ecma_builtin_string_iterator_prototype_object_next (ecma_value_t this_val) /**< this argument */
{
  /* 1 - 2. */
  if (!ecma_is_value_object (this_val))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (this_val);
  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) obj_p;

  /* 3. */
  if (!ecma_object_class_is (obj_p, ECMA_OBJECT_CLASS_STRING_ITERATOR))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_ITERATOR);
  }

  ecma_value_t iterated_value = ext_obj_p->u.cls.u3.iterated_value;

  /* 4 - 5 */
  if (ecma_is_value_empty (iterated_value))
  {
    return ecma_create_iter_result_object (ECMA_VALUE_UNDEFINED, ECMA_VALUE_TRUE);
  }

  JERRY_ASSERT (ecma_is_value_string (iterated_value));

  ecma_string_t *string_p = ecma_get_string_from_value (iterated_value);

  /* 6. */
  lit_utf8_size_t position = ext_obj_p->u.cls.u2.iterator_index;

  if (JERRY_UNLIKELY (position == ECMA_ITERATOR_INDEX_LIMIT))
  {
    /* After the ECMA_ITERATOR_INDEX_LIMIT limit is reached the [[%Iterator%NextIndex]]
       property is stored as an internal property */
    ecma_string_t *prop_name_p = ecma_get_magic_string (LIT_INTERNAL_MAGIC_STRING_ITERATOR_NEXT_INDEX);
    ecma_value_t position_value = ecma_op_object_get (obj_p, prop_name_p);

    position = (lit_utf8_size_t) (ecma_get_number_from_value (position_value));
    ecma_free_value (position_value);
  }

  /* 7. */
  lit_utf8_size_t len = ecma_string_get_length (string_p);

  /* 8. */
  if (position >= len)
  {
    ecma_deref_ecma_string (string_p);
    ext_obj_p->u.cls.u3.iterated_value = ECMA_VALUE_EMPTY;
    return ecma_create_iter_result_object (ECMA_VALUE_UNDEFINED, ECMA_VALUE_TRUE);
  }

  /* 9. */
  ecma_char_t first = ecma_string_get_char_at_pos (string_p, position);

  ecma_string_t *result_str_p;
  lit_utf8_size_t result_size = 1;

  /* 10. */
  if (first < LIT_UTF16_HIGH_SURROGATE_MIN || first > LIT_UTF16_HIGH_SURROGATE_MAX || (position + 1 == len))
  {
    result_str_p = ecma_new_ecma_string_from_code_unit (first);
  }
  /* 11. */
  else
  {
    /* 11.a */
    ecma_char_t second = ecma_string_get_char_at_pos (string_p, position + 1);

    /* 11.b */
    if (second < LIT_UTF16_LOW_SURROGATE_MIN || second > LIT_UTF16_LOW_SURROGATE_MAX)
    {
      result_str_p = ecma_new_ecma_string_from_code_unit (first);
    }
    /* 11.c */
    else
    {
      result_str_p = ecma_new_ecma_string_from_code_units (first, second);
      result_size = 2;
    }
  }

  /* 13. */
  if (position + result_size < ECMA_ITERATOR_INDEX_LIMIT)
  {
    ext_obj_p->u.cls.u2.iterator_index = (uint16_t) (position + result_size);
  }
  else
  {
    ext_obj_p->u.cls.u2.iterator_index = ECMA_ITERATOR_INDEX_LIMIT;

    ecma_string_t *prop_name_p = ecma_get_magic_string (LIT_INTERNAL_MAGIC_STRING_ITERATOR_NEXT_INDEX);
    ecma_op_object_put (obj_p, prop_name_p, ecma_make_length_value (position + result_size), true);
  }

  /* 14. */
  ecma_value_t result = ecma_create_iter_result_object (ecma_make_string_value (result_str_p), ECMA_VALUE_FALSE);
  ecma_deref_ecma_string (result_str_p);

  return result;
} /* ecma_builtin_string_iterator_prototype_object_next */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_string_iterator_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide
                                                                                      *   routine identifier */
                                                         ecma_value_t this_arg, /**< 'this' argument value */
                                                         const ecma_value_t arguments_list_p[], /**<
                                                                                                 * list of arguments
                                                                                                 * passed to routine */
                                                         uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (arguments_list_p, arguments_number);

  switch (builtin_routine_id)
  {
    case ECMA_BUILTIN_STRING_ITERATOR_PROTOTYPE_OBJECT_NEXT:
    {
      return ecma_builtin_string_iterator_prototype_object_next (this_arg);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_string_iterator_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
