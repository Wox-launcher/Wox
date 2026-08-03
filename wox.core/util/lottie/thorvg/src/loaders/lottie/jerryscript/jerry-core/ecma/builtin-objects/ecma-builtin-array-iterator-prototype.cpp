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

#include "ecma-arraybuffer-object.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-iterator-object.h"
#include "ecma-typedarray-object.h"

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
  ECMA_ARRAY_ITERATOR_PROTOTYPE_ROUTINE_START = 0,
  ECMA_ARRAY_ITERATOR_PROTOTYPE_OBJECT_NEXT,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-array-iterator-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  array_iterator_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup %arrayiteratorprototype% ECMA %ArrayIteratorPrototype% object built-in
 * @{
 */

/**
 * The %ArrayIteratorPrototype% object's 'next' routine
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
ecma_builtin_array_iterator_prototype_object_next (ecma_value_t this_val) /**< this argument */
{
  /* 1 - 2. */
  if (!ecma_is_value_object (this_val))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (this_val);
  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) obj_p;

  /* 3. */
  if (!ecma_object_class_is (obj_p, ECMA_OBJECT_CLASS_ARRAY_ITERATOR))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_ITERATOR);
  }

  ecma_value_t iterated_value = ext_obj_p->u.cls.u3.iterated_value;

  /* 4 - 5 */
  if (ecma_is_value_empty (iterated_value))
  {
    return ecma_create_iter_result_object (ECMA_VALUE_UNDEFINED, ECMA_VALUE_TRUE);
  }

  ecma_object_t *array_object_p = ecma_get_object_from_value (iterated_value);

  /* 8. */
  ecma_length_t length;
  if (ecma_object_is_typedarray (array_object_p))
  {
    /* a. */
    ecma_object_t *arraybuffer_p = ecma_typedarray_get_arraybuffer (array_object_p);
    if (ecma_arraybuffer_is_detached (arraybuffer_p))
    {
      return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
    }

    /* b. */
    length = ecma_typedarray_get_length (array_object_p);
  }
  else
  {
    ecma_value_t len_value = ecma_op_object_get_length (array_object_p, &length);

    if (ECMA_IS_VALUE_ERROR (len_value))
    {
      return len_value;
    }
  }

  ecma_length_t index = ext_obj_p->u.cls.u2.iterator_index;

  if (JERRY_UNLIKELY (index == ECMA_ITERATOR_INDEX_LIMIT))
  {
    /* After the ECMA_ITERATOR_INDEX_LIMIT limit is reached the [[%Iterator%NextIndex]]
       property is stored as an internal property */
    ecma_string_t *prop_name_p = ecma_get_magic_string (LIT_INTERNAL_MAGIC_STRING_ITERATOR_NEXT_INDEX);
    ecma_value_t index_value = ecma_op_object_get (obj_p, prop_name_p);

    if (!ecma_is_value_undefined (index_value))
    {
      index = (ecma_length_t) (ecma_get_number_from_value (index_value) + 1);
    }

    ecma_op_object_put (obj_p, prop_name_p, ecma_make_length_value (index), true);
    ecma_free_value (index_value);
  }
  else
  {
    /* 11. */
    ext_obj_p->u.cls.u2.iterator_index++;
  }

  if (index >= length)
  {
    ext_obj_p->u.cls.u3.iterated_value = ECMA_VALUE_EMPTY;
    return ecma_create_iter_result_object (ECMA_VALUE_UNDEFINED, ECMA_VALUE_TRUE);
  }

  /* 7. */
  uint8_t iterator_kind = ext_obj_p->u.cls.u1.iterator_kind;

  if (iterator_kind == ECMA_ITERATOR_KEYS)
  {
    /* 12. */
    return ecma_create_iter_result_object (ecma_make_length_value (index), ECMA_VALUE_FALSE);
  }

  /* 14. */
  ecma_value_t get_value = ecma_op_object_get_by_index (array_object_p, index);

  /* 15. */
  if (ECMA_IS_VALUE_ERROR (get_value))
  {
    return get_value;
  }

  ecma_value_t result;

  /* 16. */
  if (iterator_kind == ECMA_ITERATOR_VALUES)
  {
    result = ecma_create_iter_result_object (get_value, ECMA_VALUE_FALSE);
  }
  else
  {
    /* 17.a */
    JERRY_ASSERT (iterator_kind == ECMA_ITERATOR_ENTRIES);

    /* 17.b */
    ecma_value_t entry_array_value;
    entry_array_value = ecma_create_array_from_iter_element (get_value, ecma_make_length_value (index));

    result = ecma_create_iter_result_object (entry_array_value, ECMA_VALUE_FALSE);
    ecma_free_value (entry_array_value);
  }

  ecma_free_value (get_value);

  return result;
} /* ecma_builtin_array_iterator_prototype_object_next */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_array_iterator_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in routine identifier */
                                                        ecma_value_t this_arg, /**< 'this' argument value */
                                                        const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                                *   passed to routine */
                                                        uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (arguments_list_p, arguments_number);

  switch (builtin_routine_id)
  {
    case ECMA_ARRAY_ITERATOR_PROTOTYPE_OBJECT_NEXT:
    {
      return ecma_builtin_array_iterator_prototype_object_next (this_arg);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_array_iterator_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
