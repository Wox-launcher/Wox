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

#include "ecma-iterator-object.h"

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#include "jcontext.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaiteratorobject ECMA iterator object related routines
 * @{
 */

/**
 * Implementation of 'CreateArrayFromList' specialized for iterators
 *
 * See also:
 *          ECMA-262 v6, 7.3.16.
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return new array object
 */
ecma_value_t
ecma_create_array_from_iter_element (ecma_value_t value, /**< value */
                                     ecma_value_t index_value) /**< iterator index */
{
  auto new_array_p = ecma_op_new_array_object (0);
  ecma_builtin_helper_def_prop_by_index (new_array_p, 0, index_value, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
  ecma_builtin_helper_def_prop_by_index (new_array_p, 1, value, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
  return ecma_make_object_value (new_array_p);
}

/**
 * CreateIterResultObject operation
 *
 * See also:
 *          ECMA-262 v6, 7.4.7.
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator result object
 */
ecma_value_t
ecma_create_iter_result_object (ecma_value_t value, /**< value */
                                ecma_value_t done) /**< ECMA_VALUE_{TRUE,FALSE} based
                                                    *   on the iterator index */
{
  /* 1. */
  JERRY_ASSERT (ecma_is_value_boolean (done));

  /* 2. */
  ecma_object_t *object_p =
    ecma_create_object (ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE), 0, ECMA_OBJECT_TYPE_GENERAL);

  /* 3. */
  ecma_property_value_t *prop_value_p;
  prop_value_p = ecma_create_named_data_property (object_p,
                                                  ecma_get_magic_string (LIT_MAGIC_STRING_VALUE),
                                                  ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                  NULL);

  prop_value_p->value = ecma_copy_value_if_not_object (value);

  /* 4. */
  prop_value_p = ecma_create_named_data_property (object_p,
                                                  ecma_get_magic_string (LIT_MAGIC_STRING_DONE),
                                                  ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                  NULL);
  prop_value_p->value = done;

  /* 5. */
  return ecma_make_object_value (object_p);
} /* ecma_create_iter_result_object */

/**
 * General iterator object creation operation.
 *
 * See also: ECMA-262 v6, 21.1.5.1, 22.1.5.1, 23.1.5.1
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator object
 */
ecma_value_t
ecma_op_create_iterator_object (ecma_value_t iterated_value, /**< value from create iterator */
                                ecma_object_t *prototype_obj_p, /**< prototype object */
                                ecma_object_class_type_t iterator_type, /**< iterator type */
                                ecma_iterator_kind_t kind) /**< iterator kind*/
{
  /* 1. */
#ifdef JERRY_BUILTIN_REGEXP
  JERRY_ASSERT (iterator_type == ECMA_OBJECT_CLASS_ARRAY_ITERATOR || iterator_type == ECMA_OBJECT_CLASS_SET_ITERATOR || iterator_type == ECMA_OBJECT_CLASS_MAP_ITERATOR ||
                iterator_type == ECMA_OBJECT_CLASS_REGEXP_STRING_ITERATOR || iterator_type == ECMA_OBJECT_CLASS_STRING_ITERATOR);
#else
  JERRY_ASSERT (iterator_type == ECMA_OBJECT_CLASS_ARRAY_ITERATOR || iterator_type == ECMA_OBJECT_CLASS_SET_ITERATOR || iterator_type == ECMA_OBJECT_CLASS_MAP_ITERATOR ||
                iterator_type == ECMA_OBJECT_CLASS_STRING_ITERATOR);
#endif

  /* 2. */
  ecma_object_t *object_p =
    ecma_create_object (prototype_obj_p, sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
  ext_obj_p->u.cls.type = (uint8_t) iterator_type;

  /* 3. */
  ext_obj_p->u.cls.u3.iterated_value = iterated_value;
  /* 4. */
  ext_obj_p->u.cls.u2.iterator_index = 0;
  /* 5. */
  ext_obj_p->u.cls.u1.iterator_kind = (uint8_t) kind;

  /* 6. */
  return ecma_make_object_value (object_p);
} /* ecma_op_create_iterator_object */

/**
 * GetIterator operation
 *
 * See also: ECMA-262 v10, 7.4.1
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator object - if success
 *         raised error - otherwise
 */
ecma_value_t
ecma_op_get_iterator (ecma_value_t value, /**< value to get iterator from */
                      ecma_value_t method, /**< provided method argument */
                      ecma_value_t *next_method_p) /**< [out] next method */
{
  JERRY_ASSERT (next_method_p != NULL);
  *next_method_p = ECMA_VALUE_UNDEFINED;

  /* 1. */
  if (ECMA_IS_VALUE_ERROR (value))
  {
    return value;
  }

  bool use_default_method = false;

  /* 2. */
  if (method == ECMA_VALUE_SYNC_ITERATOR)
  {
    /* 2.a */
    use_default_method = true;
    method = ecma_op_get_method_by_symbol_id (value, LIT_GLOBAL_SYMBOL_ITERATOR);

    /* 2.b */
    if (ECMA_IS_VALUE_ERROR (method))
    {
      return method;
    }
  }
  /* 3.a */
  else if (method == ECMA_VALUE_ASYNC_ITERATOR)
  {
    use_default_method = true;

    /* 3.a.i */
    method = ecma_op_get_method_by_symbol_id (value, LIT_GLOBAL_SYMBOL_ASYNC_ITERATOR);

    if (ECMA_IS_VALUE_ERROR (method))
    {
      return method;
    }

    /* 3.a.ii */

    if (ecma_is_value_undefined (method))
    {
      method = ecma_op_get_method_by_symbol_id (value, LIT_GLOBAL_SYMBOL_ITERATOR);

      if (ECMA_IS_VALUE_ERROR (method))
      {
        return method;
      }

      ecma_value_t sync_next_method;
      ecma_value_t sync_iterator = ecma_op_get_iterator (value, method, &sync_next_method);

      if (ECMA_IS_VALUE_ERROR (sync_iterator))
      {
        ecma_free_value (method);
        return sync_iterator;
      }

      ecma_value_t async_iterator =
        ecma_op_create_async_from_sync_iterator (sync_iterator, sync_next_method, next_method_p);

      ecma_free_value (method);
      ecma_free_value (sync_iterator);
      ecma_free_value (sync_next_method);

      return async_iterator;
    }
  }

  /* 3. */
  ecma_value_t iterator = ecma_op_function_validated_call (method, value, NULL, 0);

  if (use_default_method)
  {
    ecma_free_value (method);
  }

  /* 4. */
  if (ECMA_IS_VALUE_ERROR (iterator))
  {
    return iterator;
  }

  /* 5. */
  if (!ecma_is_value_object (iterator))
  {
    ecma_free_value (iterator);
    return ecma_raise_type_error (ECMA_ERR_ITERATOR_IS_NOT_AN_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (iterator);
  ecma_value_t next_method = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_NEXT);

  if (ECMA_IS_VALUE_ERROR (next_method))
  {
    ecma_free_value (iterator);
    return next_method;
  }

  if (ecma_op_is_callable (next_method))
  {
    *next_method_p = next_method;
  }
  else
  {
    ecma_free_value (next_method);
  }

  /* 6. */
  return iterator;
} /* ecma_op_get_iterator */

/**
 * IteratorNext operation
 *
 * See also: ECMA-262 v10, 7.4.2
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator result object - if success
 *         raised error - otherwise
 */
ecma_value_t
ecma_op_iterator_next (ecma_value_t iterator, /**< iterator value */
                       ecma_value_t next_method, /**< next method */
                       ecma_value_t value) /**< the routines's value argument */
{
  JERRY_ASSERT (ecma_is_value_object (iterator));

  /* 1 - 2. */
  if (next_method == ECMA_VALUE_UNDEFINED)
  {
    return ecma_raise_type_error (ECMA_ERR_ITERATOR_NEXT_IS_NOT_CALLABLE);
  }

  ecma_object_t *next_method_obj_p = ecma_get_object_from_value (next_method);

  bool has_value = !ecma_is_value_empty (value);

  if (has_value)
  {
    return ecma_op_function_call (next_method_obj_p, iterator, &value, 1);
  }

  return ecma_op_function_call (next_method_obj_p, iterator, NULL, 0);
} /* ecma_op_iterator_next */

/**
 * IteratorReturn operation
 *
 * See also: ECMA-262 v6, 14.4.14 (last part)
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator result object - if success
 *         raised error - otherwise
 */
static ecma_value_t
ecma_op_iterator_return (ecma_value_t iterator, /**< iterator value */
                         ecma_value_t value) /**< the routines's value argument */
{
  JERRY_ASSERT (ecma_is_value_object (iterator));

  ecma_object_t *obj_p = ecma_get_object_from_value (iterator);
  ecma_value_t func_return = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_RETURN);

  if (ECMA_IS_VALUE_ERROR (func_return))
  {
    return func_return;
  }

  if (func_return == ECMA_VALUE_UNDEFINED)
  {
    return ecma_create_iter_result_object (value, ECMA_VALUE_TRUE);
  }

  ecma_value_t result = ecma_op_function_validated_call (func_return, iterator, &value, 1);
  ecma_free_value (func_return);

  return result;
} /* ecma_op_iterator_return */

/**
 * IteratorThrow operation
 *
 * See also: ECMA-262 v6, 14.4.14 (last part)
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator result object - if success
 *         raised error - otherwise
 */
static ecma_value_t
ecma_op_iterator_throw (ecma_value_t iterator, /**< iterator value */
                        ecma_value_t value) /**< the routines's value argument */
{
  JERRY_ASSERT (ecma_is_value_object (iterator));

  ecma_object_t *obj_p = ecma_get_object_from_value (iterator);
  ecma_value_t func_throw = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_THROW);

  if (ECMA_IS_VALUE_ERROR (func_throw))
  {
    return func_throw;
  }

  if (func_throw == ECMA_VALUE_UNDEFINED)
  {
    ecma_value_t result = ecma_op_iterator_close (iterator);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      return result;
    }

    ecma_free_value (result);
    return ecma_raise_type_error (ECMA_ERR_ITERATOR_THROW_IS_NOT_AVAILABLE);
  }

  ecma_value_t result = ecma_op_function_validated_call (func_throw, iterator, &value, 1);
  ecma_free_value (func_throw);

  return result;
} /* ecma_op_iterator_throw */

/**
 * IteratorComplete operation
 *
 * See also: ECMA-262 v10, 7.4.3
 *
 * @return true/false - whether the iteration ended
 */
ecma_value_t
ecma_op_iterator_complete (ecma_value_t iter_result) /**< iterator value */
{
  /* 1. */
  JERRY_ASSERT (ecma_is_value_object (iter_result));

  /* 2. */
  ecma_object_t *obj_p = ecma_get_object_from_value (iter_result);

  ecma_value_t done = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_DONE);

  if (ECMA_IS_VALUE_ERROR (done))
  {
    return done;
  }

  ecma_value_t res = ecma_make_boolean_value (ecma_op_to_boolean (done));
  ecma_free_value (done);

  return res;
} /* ecma_op_iterator_complete */

/**
 * IteratorValue operation
 *
 * See also: ECMA-262 v6, 7.4.4
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return value of the iterator result object
 */
ecma_value_t
ecma_op_iterator_value (ecma_value_t iter_result) /**< iterator value */
{
  /* 1. */
  JERRY_ASSERT (ecma_is_value_object (iter_result));

  /* 2. */
  ecma_object_t *obj_p = ecma_get_object_from_value (iter_result);
  return ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_VALUE);
} /* ecma_op_iterator_value */

/**
 * IteratorClose operation
 *
 * See also: ECMA-262 v6, 7.4.6
 *
 * @return ECMA_VALUE_EMPTY - if "return" is successfully invoked,
 *                            and the operation is called with normal completion
 *         ECMA_VALUE_ERROR - otherwise
 */
ecma_value_t
ecma_op_iterator_close (ecma_value_t iterator) /**< iterator value */
{
  /* 1. */
  JERRY_ASSERT (ecma_is_value_object (iterator));

  /* 2. */
  ecma_value_t completion = ECMA_VALUE_EMPTY;

  if (jcontext_has_pending_exception ())
  {
    completion = jcontext_take_exception ();
  }

  /* 3. */
  ecma_value_t return_method = ecma_op_get_method_by_magic_id (iterator, LIT_MAGIC_STRING_RETURN);

  /* 4. */
  if (ECMA_IS_VALUE_ERROR (return_method))
  {
    ecma_free_value (completion);
    return return_method;
  }

  /* 5. */
  if (ecma_is_value_undefined (return_method))
  {
    if (ecma_is_value_empty (completion))
    {
      return ECMA_VALUE_UNDEFINED;
    }

    jcontext_raise_exception (completion);
    return ECMA_VALUE_ERROR;
  }

  /* 6. */
  ecma_object_t *return_obj_p = ecma_get_object_from_value (return_method);
  ecma_value_t inner_result = ecma_op_function_call (return_obj_p, iterator, NULL, 0);
  ecma_deref_object (return_obj_p);

  /* 7. */
  if (!ecma_is_value_empty (completion))
  {
    if (ECMA_IS_VALUE_ERROR (inner_result))
    {
      jcontext_release_exception ();
    }
    else
    {
      ecma_free_value (inner_result);
    }

    jcontext_raise_exception (completion);
    return ECMA_VALUE_ERROR;
  }

  /* 8. */
  if (ECMA_IS_VALUE_ERROR (inner_result))
  {
    ecma_free_value (completion);
    return inner_result;
  }

  /* 9. */
  bool is_object = ecma_is_value_object (inner_result);
  ecma_free_value (inner_result);

  if (!is_object)
  {
    ecma_free_value (completion);
    return ecma_raise_type_error (ECMA_ERR_METHOD_RETURN_IS_NOT_CALLABLE);
  }

  /* 10. */
  if (ecma_is_value_empty (completion))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  jcontext_raise_exception (completion);
  return ECMA_VALUE_ERROR;
} /* ecma_op_iterator_close */

/**
 * IteratorStep operation
 *
 * See also: ECMA-262 v6, 7.4.5
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator object or ECMA_VALUE_FALSE - if success
 *         raised error - otherwise
 */
ecma_value_t
ecma_op_iterator_step (ecma_value_t iterator, /**< iterator value */
                       ecma_value_t next_method) /**< next method */
{
  /* 1. */
  ecma_value_t result = ecma_op_iterator_next (iterator, next_method, ECMA_VALUE_EMPTY);

  /* 2. */
  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  if (!ecma_is_value_object (result))
  {
    ecma_free_value (result);
    return ecma_raise_type_error (ECMA_ERR_ITERATOR_RESULT_IS_NOT_AN_OBJECT);
  }

  /* 3. */
  ecma_object_t *obj_p = ecma_get_object_from_value (result);
  ecma_value_t done = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_DONE);

  /* 4. */
  if (ECMA_IS_VALUE_ERROR (done))
  {
    ecma_free_value (result);
    return done;
  }

  bool is_done = ecma_op_to_boolean (done);
  ecma_free_value (done);

  /* 5. */
  if (is_done)
  {
    ecma_free_value (result);
    return ECMA_VALUE_FALSE;
  }

  /* 6. */
  return result;
} /* ecma_op_iterator_step */

/**
 * Perform a command specified by the command argument
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator object - if success
 *         raised error - otherwise
 */
ecma_value_t
ecma_op_iterator_do (ecma_iterator_command_type_t command, /**< command to be executed */
                     ecma_value_t iterator, /**< iterator object */
                     ecma_value_t next_method, /**< next method */
                     ecma_value_t value, /**< the routines's value argument */
                     bool *done_p) /**< it contains the logical value of the done property */
{
  ecma_value_t result;

  if (command == ECMA_ITERATOR_NEXT)
  {
    result = ecma_op_iterator_next (iterator, next_method, value);
  }
  else if (command == ECMA_ITERATOR_THROW)
  {
    result = ecma_op_iterator_throw (iterator, value);
  }
  else
  {
    JERRY_ASSERT (command == ECMA_ITERATOR_RETURN);
    result = ecma_op_iterator_return (iterator, value);
  }

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  if (!ecma_is_value_object (result))
  {
    ecma_free_value (result);
    return ecma_raise_type_error (ECMA_ERR_ITERATOR_RESULT_IS_NOT_AN_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (result);
  ecma_value_t done = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_DONE);

  if (ECMA_IS_VALUE_ERROR (done))
  {
    ecma_free_value (result);
    return done;
  }

  *done_p = ecma_op_to_boolean (done);
  ecma_free_value (done);

  return result;
} /* ecma_op_iterator_do */

/**
 * CreateAsyncFromSyncIterator operation
 *
 * See also: ECMA-262 v10, 25.1.4.1
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return async from sync iterator object
 */
ecma_value_t
ecma_op_create_async_from_sync_iterator (ecma_value_t sync_iterator, /**< sync iterator */
                                         ecma_value_t sync_next_method, /**< sync iterator next method */
                                         ecma_value_t *async_next_method_p) /**< [out] async next method */
{
  JERRY_ASSERT (ecma_is_value_object (sync_iterator));
  JERRY_ASSERT (ecma_is_value_object (sync_next_method) || ecma_is_value_undefined (sync_next_method));

  /* 1. */
  ecma_object_t *obj_p = ecma_create_object (ecma_builtin_get (ECMA_BUILTIN_ID_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE),
                                             sizeof (ecma_async_from_sync_iterator_object_t),
                                             ECMA_OBJECT_TYPE_CLASS);

  ecma_async_from_sync_iterator_object_t *ext_obj_p = (ecma_async_from_sync_iterator_object_t *) obj_p;

  /* 2. */
  ext_obj_p->sync_next_method = sync_next_method;
  ext_obj_p->header.u.cls.u3.sync_iterator = sync_iterator;
  ext_obj_p->header.u.cls.type = ECMA_OBJECT_CLASS_ASYNC_FROM_SYNC_ITERATOR;

  /* 3. */
  *async_next_method_p = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_NEXT);
  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (*async_next_method_p));

  /* 4. */
  return ecma_make_object_value (obj_p);
} /* ecma_op_create_async_from_sync_iterator */

/**
 * Async-from-Sync Iterator Value Unwrap Functions
 *
 * See also: ES11 25.1.4.2.4
 *
 * @return iterator result object
 */
ecma_value_t
ecma_async_from_sync_iterator_unwrap_cb (ecma_object_t *function_obj_p, /**< function object */
                                         const ecma_value_t args_p[], /**< argument list */
                                         const uint32_t args_count) /**< argument number */
{
  ecma_extended_object_t *unwrap_p = (ecma_extended_object_t *) function_obj_p;

  /* 2. */
  ecma_value_t arg = args_count > 0 ? args_p[0] : ECMA_VALUE_UNDEFINED;
  ecma_value_t done =
    ecma_make_boolean_value (unwrap_p->u.built_in.u2.routine_flags >> ECMA_NATIVE_HANDLER_COMMON_FLAGS_SHIFT);

  return ecma_create_iter_result_object (arg, done);
} /* ecma_async_from_sync_iterator_unwrap_cb */

/**
 * @}
 * @}
 */
