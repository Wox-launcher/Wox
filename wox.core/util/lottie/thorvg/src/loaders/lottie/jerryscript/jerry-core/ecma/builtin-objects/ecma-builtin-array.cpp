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
#include "ecma-array-object.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-objects.h"

#include "jcontext.h"
#include "jrt.h"

#if JERRY_BUILTIN_ARRAY

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
  ECMA_ARRAY_ROUTINE_START = 0,
  ECMA_ARRAY_ROUTINE_IS_ARRAY,
  ECMA_ARRAY_ROUTINE_FROM,
  ECMA_ARRAY_ROUTINE_OF,
  ECMA_ARRAY_ROUTINE_SPECIES_GET
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-array.inc.h"
#define BUILTIN_UNDERSCORED_ID  array
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup array ECMA Array object built-in
 * @{
 */

/**
 * The Array object's 'from' routine
 *
 * See also:
 *          ECMA-262 v6, 22.1.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_array_object_from (ecma_value_t this_arg, /**< 'this' argument */
                                const ecma_value_t *arguments_list_p, /**< arguments list */
                                uint32_t arguments_list_len) /**< number of arguments */
{
  /* 1. */
  ecma_value_t constructor = this_arg;
  ecma_value_t call_this_arg = ECMA_VALUE_UNDEFINED;
  ecma_value_t items = arguments_list_p[0];
  ecma_value_t mapfn = (arguments_list_len > 1) ? arguments_list_p[1] : ECMA_VALUE_UNDEFINED;
  ecma_length_t k = 0;
  ecma_value_t set_status;

  /* 2. */
  ecma_object_t *mapfn_obj_p = NULL;

  /* 3. */
  if (!ecma_is_value_undefined (mapfn))
  {
    /* 3.a */
    if (!ecma_op_is_callable (mapfn))
    {
      return ecma_raise_type_error (ECMA_ERR_CALLBACK_IS_NOT_CALLABLE);
    }

    /* 3.b */
    if (arguments_list_len > 2)
    {
      call_this_arg = arguments_list_p[2];
    }

    /* 3.c */
    mapfn_obj_p = ecma_get_object_from_value (mapfn);
  }

  /* 4. */
  ecma_value_t using_iterator = ecma_op_get_method_by_symbol_id (items, LIT_GLOBAL_SYMBOL_ITERATOR);

  /* 5. */
  if (ECMA_IS_VALUE_ERROR (using_iterator))
  {
    return using_iterator;
  }

  ecma_value_t ret_value = ECMA_VALUE_ERROR;

  /* 6. */
  if (!ecma_is_value_undefined (using_iterator))
  {
    ecma_object_t *array_obj_p;

    /* 6.a */
    if (ecma_is_constructor (constructor))
    {
      ecma_object_t *constructor_obj_p = ecma_get_object_from_value (constructor);

      ecma_value_t array = ecma_op_function_construct (constructor_obj_p, constructor_obj_p, NULL, 0);

      if (ecma_is_value_undefined (array) || ecma_is_value_null (array))
      {
        ecma_free_value (using_iterator);
        return ecma_raise_type_error (ECMA_ERR_CANNOT_CONVERT_TO_OBJECT);
      }

      /* 6.c */
      if (ECMA_IS_VALUE_ERROR (array))
      {
        ecma_free_value (using_iterator);
        return array;
      }

      array_obj_p = ecma_get_object_from_value (array);
    }
    else
    {
      /* 6.b */
      array_obj_p = ecma_op_new_array_object (0);
    }

    /* 6.d */
    ecma_value_t next_method;
    ecma_value_t iterator = ecma_op_get_iterator (items, using_iterator, &next_method);
    ecma_free_value (using_iterator);

    /* 6.e */
    if (ECMA_IS_VALUE_ERROR (iterator))
    {
      ecma_deref_object (array_obj_p);
      return iterator;
    }

    /* 6.f */
    uint32_t k = 0;

    /* 6.g */
    while (true)
    {
      /* 6.g.ii */
      ecma_value_t next = ecma_op_iterator_step (iterator, next_method);

      /* 6.g.iii */
      if (ECMA_IS_VALUE_ERROR (next))
      {
        goto iterator_cleanup;
      }

      /* 6.g.iii */
      if (ecma_is_value_false (next))
      {
        /* 6.g.iv.1 */
        ecma_value_t len_value = ecma_make_uint32_value (k);
        ecma_value_t set_status =
          ecma_op_object_put (array_obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH), len_value, true);
        ecma_free_value (len_value);

        /* 6.g.iv.2 */
        if (ECMA_IS_VALUE_ERROR (set_status))
        {
          goto iterator_cleanup;
        }

        ecma_free_value (iterator);
        ecma_free_value (next_method);
        /* 6.g.iv.3 */
        return ecma_make_object_value (array_obj_p);
      }

      /* 6.g.v */
      ecma_value_t next_value = ecma_op_iterator_value (next);

      ecma_free_value (next);

      /* 6.g.vi */
      if (ECMA_IS_VALUE_ERROR (next_value))
      {
        goto iterator_cleanup;
      }

      ecma_value_t mapped_value;
      /* 6.g.vii */
      if (mapfn_obj_p != NULL)
      {
        /* 6.g.vii.1 */
        ecma_value_t args_p[2] = { next_value, ecma_make_uint32_value (k) };
        /* 6.g.vii.3 */
        mapped_value = ecma_op_function_call (mapfn_obj_p, call_this_arg, args_p, 2);
        ecma_free_value (args_p[1]);
        ecma_free_value (next_value);

        /* 6.g.vii.2 */
        if (ECMA_IS_VALUE_ERROR (mapped_value))
        {
          ecma_op_iterator_close (iterator);
          goto iterator_cleanup;
        }
      }
      else
      {
        /* 6.g.viii */
        mapped_value = next_value;
      }

      /* 6.g.ix */
      const uint32_t flags = ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE | JERRY_PROP_SHOULD_THROW;
      ecma_value_t set_status = ecma_builtin_helper_def_prop_by_index (array_obj_p, k, mapped_value, flags);

      ecma_free_value (mapped_value);

      /* 6.g.x */
      if (ECMA_IS_VALUE_ERROR (set_status))
      {
        ecma_op_iterator_close (iterator);
        goto iterator_cleanup;
      }

      /* 6.g.xi */
      k++;
    }

iterator_cleanup:
    ecma_free_value (iterator);
    ecma_free_value (next_method);
    ecma_deref_object (array_obj_p);

    return ret_value;
  }

  /* 8. */
  ecma_value_t array_like = ecma_op_to_object (items);

  /* 9. */
  if (ECMA_IS_VALUE_ERROR (array_like))
  {
    return array_like;
  }

  ecma_object_t *array_like_obj_p = ecma_get_object_from_value (array_like);

  /* 10. */
  ecma_length_t len;
  ecma_value_t len_value = ecma_op_object_get_length (array_like_obj_p, &len);

  /* 11. */
  if (ECMA_IS_VALUE_ERROR (len_value))
  {
    goto cleanup;
  }

  /* 12. */
  ecma_object_t *array_obj_p;

  /* 12.a */
  if (ecma_is_constructor (constructor))
  {
    ecma_object_t *constructor_obj_p = ecma_get_object_from_value (constructor);

    len_value = ecma_make_length_value (len);
    ecma_value_t array = ecma_op_function_construct (constructor_obj_p, constructor_obj_p, &len_value, 1);
    ecma_free_value (len_value);

    if (ecma_is_value_undefined (array) || ecma_is_value_null (array))
    {
      ecma_raise_type_error (ECMA_ERR_CANNOT_CONVERT_TO_OBJECT);
      goto cleanup;
    }

    /* 14. */
    if (ECMA_IS_VALUE_ERROR (array))
    {
      goto cleanup;
    }

    array_obj_p = ecma_get_object_from_value (array);
  }
  else
  {
    /* 13.a */
    array_obj_p = ecma_op_new_array_object_from_length (len);

    if (JERRY_UNLIKELY (array_obj_p == NULL))
    {
      goto cleanup;
    }
  }

  /* 15. */
  k = 0;

  /* 16. */
  while (k < len)
  {
    /* 16.b */
    ecma_value_t k_value = ecma_op_object_get_by_index (array_like_obj_p, k);

    /* 16.c */
    if (ECMA_IS_VALUE_ERROR (k_value))
    {
      goto construct_cleanup;
    }

    ecma_value_t mapped_value;
    /* 16.d */
    if (mapfn_obj_p != NULL)
    {
      /* 16.d.i */
      ecma_value_t args_p[2] = { k_value, ecma_make_length_value (k) };
      mapped_value = ecma_op_function_call (mapfn_obj_p, call_this_arg, args_p, 2);
      ecma_free_value (args_p[1]);
      ecma_free_value (k_value);

      /* 16.d.ii */
      if (ECMA_IS_VALUE_ERROR (mapped_value))
      {
        goto construct_cleanup;
      }
    }
    else
    {
      /* 16.e */
      mapped_value = k_value;
    }

    /* 16.f */
    const uint32_t flags = ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE | JERRY_PROP_SHOULD_THROW;
    ecma_value_t set_status = ecma_builtin_helper_def_prop_by_index (array_obj_p, k, mapped_value, flags);

    ecma_free_value (mapped_value);

    /* 16.g */
    if (ECMA_IS_VALUE_ERROR (set_status))
    {
      goto construct_cleanup;
    }

    /* 16.h */
    k++;
  }

  /* 17. */
  len_value = ecma_make_length_value (k);
  set_status = ecma_op_object_put (array_obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH), len_value, true);
  ecma_free_value (len_value);

  /* 18. */
  if (ECMA_IS_VALUE_ERROR (set_status))
  {
    goto construct_cleanup;
  }

  /* 19. */
  ecma_deref_object (array_like_obj_p);
  return ecma_make_object_value (array_obj_p);

construct_cleanup:
  ecma_deref_object (array_obj_p);
cleanup:
  ecma_deref_object (array_like_obj_p);
  return ret_value;
} /* ecma_builtin_array_object_from */

/**
 * The Array object's 'of' routine
 *
 * See also:
 *          ECMA-262 v6, 22.1.2.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_array_object_of (ecma_value_t this_arg, /**< 'this' argument */
                              const ecma_value_t *arguments_list_p, /**< arguments list */
                              uint32_t arguments_list_len) /**< number of arguments */
{
  if (!ecma_is_constructor (this_arg))
  {
    return ecma_op_new_array_object_from_buffer (arguments_list_p, arguments_list_len);
  }

  ecma_value_t len = ecma_make_uint32_value (arguments_list_len);

  ecma_value_t ret_val =
    ecma_op_function_construct (ecma_get_object_from_value (this_arg), ecma_get_object_from_value (this_arg), &len, 1);

  if (ECMA_IS_VALUE_ERROR (ret_val))
  {
    ecma_free_value (len);
    return ret_val;
  }

  uint32_t k = 0;
  ecma_object_t *obj_p = ecma_get_object_from_value (ret_val);
  const uint32_t prop_status_flags = ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE | JERRY_PROP_SHOULD_THROW;

  while (k < arguments_list_len)
  {
    ecma_value_t define_status =
      ecma_builtin_helper_def_prop_by_index (obj_p, k, arguments_list_p[k], prop_status_flags);

    if (ECMA_IS_VALUE_ERROR (define_status))
    {
      ecma_free_value (len);
      ecma_deref_object (obj_p);
      return define_status;
    }

    k++;
  }

  ret_val = ecma_op_object_put (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH), len, true);

  ecma_free_value (len);

  if (ECMA_IS_VALUE_ERROR (ret_val))
  {
    ecma_deref_object (obj_p);
    return ret_val;
  }

  return ecma_make_object_value (obj_p);
} /* ecma_builtin_array_object_of */

/**
 * Handle calling [[Call]] of built-in Array object
 *
 * @return ECMA_VALUE_ERROR - if the array construction fails
 *         constructed array object - otherwise
 */
ecma_value_t
ecma_builtin_array_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                  uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  if (arguments_list_len != 1 || !ecma_is_value_number (arguments_list_p[0]))
  {
    return ecma_op_new_array_object_from_buffer (arguments_list_p, arguments_list_len);
  }

  ecma_number_t num = ecma_get_number_from_value (arguments_list_p[0]);
  uint32_t num_uint32 = ecma_number_to_uint32 (num);

  if (num != ((ecma_number_t) num_uint32))
  {
    return ecma_raise_range_error (ECMA_ERR_INVALID_ARRAY_LENGTH);
  }

  return ecma_make_object_value (ecma_op_new_array_object (num_uint32));
} /* ecma_builtin_array_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Array object
 *
 * @return ECMA_VALUE_ERROR - if the array construction fails
 *         constructed array object - otherwise
 */
ecma_value_t
ecma_builtin_array_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                       uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  ecma_object_t *proto_p =
    ecma_op_get_prototype_from_constructor (JERRY_CONTEXT (current_new_target_p), ECMA_BUILTIN_ID_ARRAY_PROTOTYPE);

  if (proto_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t result = ecma_builtin_array_dispatch_call (arguments_list_p, arguments_list_len);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    ecma_deref_object (proto_p);
    return result;
  }

  ecma_object_t *object_p = ecma_get_object_from_value (result);
  ECMA_SET_NON_NULL_POINTER (object_p->u2.prototype_cp, proto_p);
  ecma_deref_object (proto_p);
  return result;
} /* ecma_builtin_array_dispatch_construct */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_array_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                     ecma_value_t this_arg, /**< 'this' argument value */
                                     const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                             *   passed to routine */
                                     uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (arguments_number);

  switch (builtin_routine_id)
  {
    case ECMA_ARRAY_ROUTINE_IS_ARRAY:
    {
      JERRY_UNUSED (this_arg);

      return ecma_is_value_array (arguments_list_p[0]);
    }
    case ECMA_ARRAY_ROUTINE_FROM:
    {
      return ecma_builtin_array_object_from (this_arg, arguments_list_p, arguments_number);
    }
    case ECMA_ARRAY_ROUTINE_OF:
    {
      return ecma_builtin_array_object_of (this_arg, arguments_list_p, arguments_number);
    }
    case ECMA_ARRAY_ROUTINE_SPECIES_GET:
    {
      return ecma_copy_value (this_arg);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_array_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_ARRAY */
