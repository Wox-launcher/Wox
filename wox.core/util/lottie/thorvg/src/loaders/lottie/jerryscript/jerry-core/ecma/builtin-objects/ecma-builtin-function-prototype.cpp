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

#include "ecma-builtin-function-prototype.h"

#include "ecma-alloc.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-extended-info.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-proxy-object.h"

#include "jrt.h"

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
  ECMA_FUNCTION_PROTOTYPE_ROUTINE_START = 0,
  ECMA_FUNCTION_PROTOTYPE_TO_STRING,
  ECMA_FUNCTION_PROTOTYPE_CALL,
  ECMA_FUNCTION_PROTOTYPE_APPLY,
  ECMA_FUNCTION_PROTOTYPE_BIND,
  ECMA_FUNCTION_PROTOTYPE_SYMBOL_HAS_INSTANCE,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-function-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  function_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup functionprototype ECMA Function.prototype object built-in
 * @{
 */

/**
 * Maximum number of arguments for an apply function.
 */
#define ECMA_FUNCTION_APPLY_ARGUMENT_COUNT_LIMIT 65535

/**
 * The Function.prototype object's 'toString' routine
 *
 * See also:
 *          ECMA-262 v5, 15.3.4.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_function_prototype_object_to_string (ecma_object_t *func_obj_p) /**< this argument object */
{
  if (ecma_get_object_type (func_obj_p) != ECMA_OBJECT_TYPE_FUNCTION)
  {
    return ecma_make_magic_string_value (LIT_MAGIC_STRING_FUNCTION_TO_STRING_NATIVE);
  }

  return ecma_make_magic_string_value (LIT_MAGIC_STRING_FUNCTION_TO_STRING_ECMA);
} /* ecma_builtin_function_prototype_object_to_string */

/**
 * The Function.prototype object's 'apply' routine
 *
 * See also:
 *          ECMA-262 v5, 15.3.4.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_function_prototype_object_apply (ecma_object_t *func_obj_p, /**< this argument object */
                                              ecma_value_t arg1, /**< first argument */
                                              ecma_value_t arg2) /**< second argument */
{
  /* 2. */
  if (ecma_is_value_null (arg2) || ecma_is_value_undefined (arg2))
  {
    return ecma_op_function_call (func_obj_p, arg1, NULL, 0);
  }

  /* 3. */
  if (!ecma_is_value_object (arg2))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_IS_NOT_AN_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (arg2);

  /* 4-5. */
  ecma_length_t length;
  ecma_value_t len_value = ecma_op_object_get_length (obj_p, &length);

  if (ECMA_IS_VALUE_ERROR (len_value))
  {
    return len_value;
  }

  if (length >= ECMA_FUNCTION_APPLY_ARGUMENT_COUNT_LIMIT)
  {
    return ecma_raise_range_error (ECMA_ERR_TOO_MANY_ARGUMENTS_DECLARED_FOR_FUNCTION_APPLY);
  }

  /* 6. */
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;
  JMEM_DEFINE_LOCAL_ARRAY (arguments_list_p, length, ecma_value_t);
  ecma_length_t index = 0;

  /* 7. */
  for (index = 0; index < length; index++)
  {
    ecma_value_t get_value = ecma_op_object_get_by_index (obj_p, index);

    if (ECMA_IS_VALUE_ERROR (get_value))
    {
      ret_value = get_value;
      break;
    }

    arguments_list_p[index] = get_value;
  }

  if (ecma_is_value_empty (ret_value))
  {
    JERRY_ASSERT (index == length);
    ret_value = ecma_op_function_call (func_obj_p, arg1, arguments_list_p, (uint32_t) length);
  }

  for (uint32_t remove_index = 0; remove_index < index; remove_index++)
  {
    ecma_free_value (arguments_list_p[remove_index]);
  }

  JMEM_FINALIZE_LOCAL_ARRAY (arguments_list_p);

  return ret_value;
} /* ecma_builtin_function_prototype_object_apply */

/**
 * The Function.prototype object's 'call' routine
 *
 * See also:
 *          ECMA-262 v5, 15.3.4.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_function_prototype_object_call (ecma_object_t *func_obj_p, /**< this argument object */
                                             const ecma_value_t *arguments_list_p, /**< list of arguments */
                                             uint32_t arguments_number) /**< number of arguments */
{
  if (arguments_number == 0)
  {
    /* Even a 'this' argument is missing. */
    return ecma_op_function_call (func_obj_p, ECMA_VALUE_UNDEFINED, NULL, 0);
  }

  return ecma_op_function_call (func_obj_p,
                                arguments_list_p[0],
                                arguments_list_p + 1,
                                (uint32_t) (arguments_number - 1u));
} /* ecma_builtin_function_prototype_object_call */

/**
 * The Function.prototype object's 'bind' routine
 *
 * See also:
 *          ECMA-262 v5, 15.3.4.5
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_function_prototype_object_bind (ecma_object_t *this_arg_obj_p, /**< this argument object */
                                             const ecma_value_t *arguments_list_p, /**< list of arguments */
                                             uint32_t arguments_number) /**< number of arguments */
{
  /* 4. 11. 18. */
  ecma_object_t *prototype_obj_p;

#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (this_arg_obj_p))
  {
    ecma_value_t proto = ecma_proxy_object_get_prototype_of (this_arg_obj_p);

    if (ECMA_IS_VALUE_ERROR (proto))
    {
      return proto;
    }
    prototype_obj_p = ecma_is_value_null (proto) ? NULL : ecma_get_object_from_value (proto);
  }
  else
  {
#endif /* JERRY_BUILTIN_PROXY */
    jmem_cpointer_t proto_cp = ecma_op_ordinary_object_get_prototype_of (this_arg_obj_p);
    if (proto_cp != JMEM_CP_NULL)
    {
      prototype_obj_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, proto_cp);
      ecma_ref_object (prototype_obj_p);
    }
    else
    {
      prototype_obj_p = NULL;
    }
#if JERRY_BUILTIN_PROXY
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_object_t *function_p;
  ecma_bound_function_t *bound_func_p;

  if (arguments_number == 0 || (arguments_number == 1 && !ecma_is_value_integer_number (arguments_list_p[0])))
  {
    function_p = ecma_create_object (prototype_obj_p, sizeof (ecma_bound_function_t), ECMA_OBJECT_TYPE_BOUND_FUNCTION);

    /* 8. */
    bound_func_p = (ecma_bound_function_t *) function_p;
    ECMA_SET_NON_NULL_POINTER_TAG (bound_func_p->header.u.bound_function.target_function, this_arg_obj_p, 0);

    bound_func_p->header.u.bound_function.args_len_or_this = ECMA_VALUE_UNDEFINED;

    if (arguments_number != 0)
    {
      bound_func_p->header.u.bound_function.args_len_or_this = ecma_copy_value_if_not_object (arguments_list_p[0]);
    }
  }
  else
  {
    JERRY_ASSERT (arguments_number > 0);

    size_t obj_size = sizeof (ecma_bound_function_t) + (arguments_number * sizeof (ecma_value_t));

    function_p = ecma_create_object (prototype_obj_p, obj_size, ECMA_OBJECT_TYPE_BOUND_FUNCTION);

    /* 8. */
    bound_func_p = (ecma_bound_function_t *) function_p;
    ECMA_SET_NON_NULL_POINTER_TAG (bound_func_p->header.u.bound_function.target_function, this_arg_obj_p, 0);

    /* NOTE: This solution provides temporary false data about the object's size
       but prevents GC from freeing it until it's not fully initialized. */
    bound_func_p->header.u.bound_function.args_len_or_this = ECMA_VALUE_UNDEFINED;
    ecma_value_t *args_p = (ecma_value_t *) (bound_func_p + 1);

    for (uint32_t i = 0; i < arguments_number; i++)
    {
      *args_p++ = ecma_copy_value_if_not_object (arguments_list_p[i]);
    }

    ecma_value_t args_len_or_this = ecma_make_integer_value ((ecma_integer_value_t) arguments_number);
    bound_func_p->header.u.bound_function.args_len_or_this = args_len_or_this;
  }

  if (prototype_obj_p != NULL)
  {
    ecma_deref_object (prototype_obj_p);
  }

  bound_func_p->target_length = ecma_make_integer_value (0);

  ecma_string_t *len_string = ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH);
  ecma_property_descriptor_t prop_desc;
  ecma_value_t status = ecma_op_object_get_own_property_descriptor (this_arg_obj_p, len_string, &prop_desc);

#if JERRY_BUILTIN_PROXY
  if (ECMA_IS_VALUE_ERROR (status))
  {
    ecma_deref_object (function_p);
    return status;
  }
#endif /* JERRY_BUILTIN_PROXY */

  if (ecma_is_value_true (status))
  {
    ecma_free_property_descriptor (&prop_desc);
    ecma_value_t len_value = ecma_op_object_get (this_arg_obj_p, len_string);

    if (ECMA_IS_VALUE_ERROR (len_value))
    {
      ecma_deref_object (function_p);
      return len_value;
    }

    if (ecma_is_value_number (len_value))
    {
      ecma_number_t len_num;
      ecma_op_to_integer (len_value, &len_num);
      bound_func_p->target_length = ecma_make_number_value (len_num);
    }
    ecma_free_value (len_value);
  }

  /* 12. */
  ecma_value_t name_value = ecma_op_object_get_by_magic_id (this_arg_obj_p, LIT_MAGIC_STRING_NAME);
  if (ECMA_IS_VALUE_ERROR (name_value))
  {
    ecma_deref_object (function_p);
    return name_value;
  }

  ecma_string_t *name_p;

  if (ecma_is_value_string (name_value))
  {
    name_p = ecma_get_string_from_value (name_value);
  }
  else
  {
    ecma_free_value (name_value);
    name_p = ecma_get_magic_string (LIT_MAGIC_STRING__EMPTY);
  }

  ecma_value_t bound_function_name = ecma_op_function_form_name (name_p, (char*)"bound ", 6);

  ecma_deref_ecma_string (name_p);

  ecma_property_value_t *name_prop_value_p;
  name_prop_value_p = ecma_create_named_data_property (function_p,
                                                       ecma_get_magic_string (LIT_MAGIC_STRING_NAME),
                                                       ECMA_PROPERTY_FLAG_CONFIGURABLE,
                                                       NULL);

  name_prop_value_p->value = bound_function_name;

  /*
   * [[Class]] property is not stored explicitly for objects of ECMA_OBJECT_TYPE_FUNCTION type.
   *
   * See also: ecma_object_get_class_name
   */

  /* 22. */
  return ecma_make_object_value (function_p);
} /* ecma_builtin_function_prototype_object_bind */

/**
 * Handle calling [[Call]] of built-in Function.prototype object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_function_prototype_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                               uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ECMA_VALUE_UNDEFINED;
} /* ecma_builtin_function_prototype_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Function.prototype object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_function_prototype_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                                    uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_raise_type_error (ECMA_ERR_FUNCTION_PROTOTYPE_NOT_A_CONSTRUCTOR);
} /* ecma_builtin_function_prototype_dispatch_construct */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_function_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                                  ecma_value_t this_arg, /**< 'this' argument value */
                                                  const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                          *   passed to routine */
                                                  uint32_t arguments_number) /**< length of arguments' list */
{
  if (!ecma_op_is_callable (this_arg))
  {
    if (JERRY_UNLIKELY (builtin_routine_id == ECMA_FUNCTION_PROTOTYPE_SYMBOL_HAS_INSTANCE))
    {
      return ECMA_VALUE_FALSE;
    }

    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_FUNCTION);
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (this_arg);

  switch (builtin_routine_id)
  {
    case ECMA_FUNCTION_PROTOTYPE_TO_STRING:
    {
      return ecma_builtin_function_prototype_object_to_string (func_obj_p);
    }
    case ECMA_FUNCTION_PROTOTYPE_APPLY:
    {
      return ecma_builtin_function_prototype_object_apply (func_obj_p, arguments_list_p[0], arguments_list_p[1]);
    }
    case ECMA_FUNCTION_PROTOTYPE_CALL:
    {
      return ecma_builtin_function_prototype_object_call (func_obj_p, arguments_list_p, arguments_number);
    }
    case ECMA_FUNCTION_PROTOTYPE_BIND:
    {
      return ecma_builtin_function_prototype_object_bind (func_obj_p, arguments_list_p, arguments_number);
    }
    case ECMA_FUNCTION_PROTOTYPE_SYMBOL_HAS_INSTANCE:
    {
      return ecma_op_object_has_instance (func_obj_p, arguments_list_p[0]);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_function_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
