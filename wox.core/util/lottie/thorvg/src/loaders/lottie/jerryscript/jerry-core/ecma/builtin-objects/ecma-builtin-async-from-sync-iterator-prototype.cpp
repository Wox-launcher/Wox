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

#include "jerryscript-types.h"

#include "ecma-builtin-handlers.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-objects.h"
#include "ecma-promise-object.h"

#include "jcontext.h"
#include "jrt.h"
#include "lit-magic-strings.h"
#include "lit-strings.h"
#include "opcodes.h"
#include "vm-defines.h"

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
  ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_START = 0, /**< builtin routine start id */
  ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_NEXT, /**< 'next' routine v11, 25.1.4.2.1  */
  ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_RETURN, /**< 'return' routine v11, 25.1.4.2.2  */
  ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_THROW /**< 'throw' routine v11, 25.1.4.2.3  */
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-async-from-sync-iterator-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  async_from_sync_iterator_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup asyncfromsynciteratorprototype ECMA %AsyncFromSyncIteratorPrototype% object built-in
 * @{
 */

/**
 * AsyncFromSyncIteratorContinuation operation
 *
 * See also:
 *         ECMAScript v11, 25.1.4.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_op_async_from_sync_iterator_prototype_continuation (ecma_value_t result, /**< routine's 'result' argument */
                                                         ecma_object_t *capability_obj_p) /**< promise capability */
{
  /* 1. */
  ecma_value_t done = ecma_op_iterator_complete (result);

  /* 2. */
  if (ECMA_IS_VALUE_ERROR (ecma_op_if_abrupt_reject_promise (&done, capability_obj_p)))
  {
    return done;
  }

  uint16_t done_flag = ecma_is_value_false (done) ? 0 : (1 << ECMA_NATIVE_HANDLER_COMMON_FLAGS_SHIFT);
  ecma_free_value (done);

  /* 3. */
  ecma_value_t value = ecma_op_iterator_value (result);

  /* 4. */
  if (ECMA_IS_VALUE_ERROR (ecma_op_if_abrupt_reject_promise (&value, capability_obj_p)))
  {
    return value;
  }

  /* 5. */
  ecma_value_t builtin_promise = ecma_make_object_value (ecma_builtin_get (ECMA_BUILTIN_ID_PROMISE));
  ecma_value_t value_wrapper = ecma_promise_reject_or_resolve (builtin_promise, value, true);
  ecma_free_value (value);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (ecma_op_if_abrupt_reject_promise (&value_wrapper, capability_obj_p)))
  {
    return value_wrapper;
  }

  /* 8 - 9. */
  ecma_object_t *on_fulfilled = ecma_op_create_native_handler (ECMA_NATIVE_HANDLER_ASYNC_FROM_SYNC_ITERATOR_UNWRAP,
                                                                sizeof (ecma_extended_object_t));
  ((ecma_extended_object_t *) on_fulfilled)->u.built_in.u2.routine_flags = (uint8_t) done_flag;

  /* 10. */
  ecma_value_t then_result = ecma_promise_perform_then (value_wrapper,
                                                        ecma_make_object_value (on_fulfilled),
                                                        ECMA_VALUE_UNDEFINED,
                                                        capability_obj_p);

  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (then_result));
  ecma_deref_object (on_fulfilled);
  ecma_free_value (value_wrapper);

  /* 11. */
  return then_result;
} /* ecma_op_async_from_sync_iterator_prototype_continuation */

/**
 * The %AsyncFromSyncIteratorPrototype% object's 'next' routine
 *
 * See also:
 *         ECMAScript v11, 25.1.4.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_async_from_sync_iterator_prototype_next (ecma_async_from_sync_iterator_object_t *iter_p, /**< iterator
                                                                                                       *   record*/
                                                      ecma_object_t *capability_p, /**< promise capability */
                                                      ecma_value_t value) /**< routine's 'value' argument */
{
  /* 5. */
  ecma_value_t next_result =
    ecma_op_iterator_next (iter_p->header.u.cls.u3.sync_iterator, iter_p->sync_next_method, value);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (ecma_op_if_abrupt_reject_promise (&next_result, capability_p)))
  {
    return next_result;
  }

  /* 7. */
  ecma_value_t result = ecma_op_async_from_sync_iterator_prototype_continuation (next_result, capability_p);
  ecma_free_value (next_result);

  return result;
} /* ecma_builtin_async_from_sync_iterator_prototype_next */

/**
 * The %AsyncFromSyncIteratorPrototype% object's 'return' and 'throw' routines
 *
 * See also:
 *         ECMAScript v11, 25.1.4.2.2
 *         ECMAScript v11, 25.1.4.2.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_async_from_sync_iterator_prototype_do (ecma_async_from_sync_iterator_object_t *iter_p, /**< iterator
                                                                                                     *   record*/
                                                    ecma_object_t *capability_obj_p, /**< promise capability */
                                                    ecma_value_t value, /**< routine's 'value' argument */
                                                    lit_magic_string_id_t method_id) /**< method id */
{
  /* 5. */
  ecma_value_t sync_iterator = iter_p->header.u.cls.u3.sync_iterator;
  ecma_value_t method = ecma_op_get_method_by_magic_id (sync_iterator, method_id);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (ecma_op_if_abrupt_reject_promise (&method, capability_obj_p)))
  {
    return method;
  }

  ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) capability_obj_p;

  ecma_value_t call_arg;
  uint32_t arg_size;

  if (ecma_is_value_empty (value))
  {
    arg_size = 0;
    call_arg = ECMA_VALUE_UNDEFINED;
  }
  else
  {
    arg_size = 1;
    call_arg = value;
  }

  /* 7. */
  if (ecma_is_value_undefined (method))
  {
    ecma_value_t func_obj;

    if (method_id == LIT_MAGIC_STRING_RETURN)
    {
      /* 7.a. */
      call_arg = ecma_create_iter_result_object (call_arg, ECMA_VALUE_TRUE);
      arg_size = 1;
      func_obj = capability_p->resolve;
    }
    else
    {
      func_obj = capability_p->reject;
    }

    /* 7.b. */
    ecma_value_t resolve =
      ecma_op_function_call (ecma_get_object_from_value (func_obj), ECMA_VALUE_UNDEFINED, &call_arg, arg_size);
    JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (resolve));
    ecma_free_value (resolve);

    if (method_id == LIT_MAGIC_STRING_RETURN)
    {
      ecma_free_value (call_arg);
    }

    /* 7.c. */
    return ecma_copy_value (capability_p->header.u.cls.u3.promise);
  }

  /* 8. */
  ecma_value_t call_result = ecma_op_function_validated_call (method, sync_iterator, &call_arg, arg_size);
  ecma_free_value (method);

  /* 9. */
  if (ECMA_IS_VALUE_ERROR (ecma_op_if_abrupt_reject_promise (&call_result, capability_obj_p)))
  {
    return call_result;
  }

  /* 10. */
  if (!ecma_is_value_object (call_result))
  {
    ecma_free_value (call_result);

#if JERRY_ERROR_MESSAGES
    const lit_utf8_byte_t *msg_p = (lit_utf8_byte_t *) ecma_get_error_msg (ECMA_ERR_ARGUMENT_IS_NOT_AN_OBJECT);
    lit_utf8_size_t msg_size = ecma_get_error_size (ECMA_ERR_ARGUMENT_IS_NOT_AN_OBJECT);
    ecma_string_t *error_msg_p = ecma_new_ecma_string_from_ascii (msg_p, msg_size);
#else /* !JERRY_ERROR_MESSAGES */
    ecma_string_t *error_msg_p = ecma_get_magic_string (LIT_MAGIC_STRING__EMPTY);
#endif /* JERRY_ERROR_MESSAGES */

    ecma_object_t *type_error_obj_p = ecma_new_standard_error (JERRY_ERROR_TYPE, error_msg_p);

#if JERRY_ERROR_MESSAGES
    ecma_deref_ecma_string (error_msg_p);
#endif /* JERRY_ERROR_MESSAGES */

    ecma_value_t type_error = ecma_make_object_value (type_error_obj_p);

    /* 10.a. */
    ecma_value_t reject =
      ecma_op_function_call (ecma_get_object_from_value (capability_p->reject), ECMA_VALUE_UNDEFINED, &type_error, 1);
    JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (reject));
    ecma_deref_object (type_error_obj_p);
    ecma_free_value (reject);

    /* 10.b. */
    return ecma_copy_value (capability_p->header.u.cls.u3.promise);
  }

  ecma_value_t result = ecma_op_async_from_sync_iterator_prototype_continuation (call_result, capability_obj_p);
  ecma_free_value (call_result);

  return result;
} /* ecma_builtin_async_from_sync_iterator_prototype_do */

/**
 * Dispatcher of the %AsyncFromSyncIteratorPrototype% built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_async_from_sync_iterator_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide
                                                                                               *   routine
                                                                                               *   identifier */
                                                                  ecma_value_t this_arg, /**< 'this' argument value */
                                                                  const ecma_value_t arguments_list_p[], /**< list of
                                                                                                          *   arguments
                                                                                                          *   passed to
                                                                                                          *   routine */
                                                                  uint32_t arguments_number) /**< length of
                                                                                              *   arguments' list */
{
  JERRY_UNUSED (arguments_number);
  JERRY_ASSERT (ecma_is_value_object (this_arg));

  ecma_object_t *this_obj_p = ecma_get_object_from_value (this_arg);

  JERRY_ASSERT (ecma_object_class_is (this_obj_p, ECMA_OBJECT_CLASS_ASYNC_FROM_SYNC_ITERATOR));

  ecma_async_from_sync_iterator_object_t *iter_p = (ecma_async_from_sync_iterator_object_t *) this_obj_p;

  ecma_value_t builtin_promise = ecma_make_object_value (ecma_builtin_get (ECMA_BUILTIN_ID_PROMISE));
  ecma_object_t *capability_p = ecma_promise_new_capability (builtin_promise, ECMA_VALUE_UNDEFINED);
  JERRY_ASSERT (capability_p != NULL);

  ecma_value_t result;
  ecma_value_t arg = (arguments_number == 0 ? ECMA_VALUE_EMPTY : arguments_list_p[0]);

  switch (builtin_routine_id)
  {
    case ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_NEXT:
    {
      result = ecma_builtin_async_from_sync_iterator_prototype_next (iter_p, capability_p, arg);
      break;
    }
    case ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_RETURN:
    {
      result = ecma_builtin_async_from_sync_iterator_prototype_do (iter_p, capability_p, arg, LIT_MAGIC_STRING_RETURN);
      break;
    }
    case ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_THROW:
    {
      result = ecma_builtin_async_from_sync_iterator_prototype_do (iter_p, capability_p, arg, LIT_MAGIC_STRING_THROW);
      break;
    }
    default:
    {
      JERRY_UNREACHABLE ();
      break;
    }
  }

  ecma_deref_object (capability_p);

  return result;
} /* ecma_builtin_async_from_sync_iterator_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
