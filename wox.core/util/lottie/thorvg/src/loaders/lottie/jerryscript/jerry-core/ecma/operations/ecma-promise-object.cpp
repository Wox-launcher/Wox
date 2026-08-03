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

#include "ecma-promise-object.h"

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-boolean-object.h"
#include "ecma-builtin-handlers.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-jobqueue.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#include "jcontext.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmapromiseobject ECMA Promise object related routines
 * @{
 */

/**
 * Check if an object is promise.
 *
 * @return true - if the object is a promise.
 *         false - otherwise.
 */
bool
ecma_is_promise (ecma_object_t *obj_p) /**< points to object */
{
  return ecma_object_class_is (obj_p, ECMA_OBJECT_CLASS_PROMISE);
} /* ecma_is_promise */

/**
 * Get the result of the promise.
 *
 * @return ecma value of the promise result.
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_promise_get_result (ecma_object_t *obj_p) /**< points to promise object */
{
  JERRY_ASSERT (ecma_is_promise (obj_p));

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

  return ecma_copy_value (ext_object_p->u.cls.u3.value);
} /* ecma_promise_get_result */

/**
 * Set the PromiseResult of promise.
 *
 * @return void
 */
static inline void
ecma_promise_set_result (ecma_object_t *obj_p, /**< points to promise object */
                         ecma_value_t result) /**< the result value */
{
  JERRY_ASSERT (ecma_is_promise (obj_p));

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

  JERRY_ASSERT (ext_object_p->u.cls.u3.value == ECMA_VALUE_UNDEFINED);

  ext_object_p->u.cls.u3.value = result;
} /* ecma_promise_set_result */

/**
 * Get the PromiseState of promise.
 *
 * @return the state's enum value
 */
uint8_t
ecma_promise_get_flags (ecma_object_t *obj_p) /**< points to promise object */
{
  JERRY_ASSERT (ecma_is_promise (obj_p));

  return ((ecma_extended_object_t *) obj_p)->u.cls.u1.promise_flags;
} /* ecma_promise_get_flags */

/**
 * Set the PromiseState of promise.
 *
 * @return void
 */
static inline void
ecma_promise_set_state (ecma_object_t *obj_p, /**< points to promise object */
                        bool is_fulfilled) /**< new flags */
{
  JERRY_ASSERT (ecma_is_promise (obj_p));
  JERRY_ASSERT (ecma_promise_get_flags (obj_p) & ECMA_PROMISE_IS_PENDING);

  uint8_t flags_to_invert =
    (is_fulfilled ? (ECMA_PROMISE_IS_PENDING | ECMA_PROMISE_IS_FULFILLED) : ECMA_PROMISE_IS_PENDING);

  ((ecma_extended_object_t *) obj_p)->u.cls.u1.promise_flags ^= flags_to_invert;
} /* ecma_promise_set_state */

/**
 * Take a collection of Reactions and enqueue a new PromiseReactionJob for each Reaction.
 *
 * See also: ES2015 25.4.1.8
 */
static void
ecma_promise_trigger_reactions (ecma_collection_t *reactions, /**< lists of reactions */
                                ecma_value_t value, /**< value for resolve or reject */
                                bool is_reject) /**< true if promise is rejected, false otherwise */
{
  ecma_value_t *buffer_p = reactions->buffer_p;
  ecma_value_t *buffer_end_p = buffer_p + reactions->item_count;

  while (buffer_p < buffer_end_p)
  {
    ecma_value_t object_with_tag = *buffer_p++;
    ecma_object_t *object_p = ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_object_t, object_with_tag);
    ecma_value_t object = ecma_make_object_value (object_p);

    if (JMEM_CP_GET_THIRD_BIT_FROM_POINTER_TAG (object_with_tag))
    {
      ecma_enqueue_promise_async_reaction_job (object, value, is_reject);
      continue;
    }

    if (!is_reject)
    {
      ecma_value_t handler = ECMA_VALUE_TRUE;

      if (JMEM_CP_GET_FIRST_BIT_FROM_POINTER_TAG (object_with_tag))
      {
        handler = *buffer_p++;
      }

      ecma_enqueue_promise_reaction_job (object, handler, value);
    }
    else if (JMEM_CP_GET_FIRST_BIT_FROM_POINTER_TAG (object_with_tag))
    {
      buffer_p++;
    }

    if (is_reject)
    {
      ecma_value_t handler = ECMA_VALUE_FALSE;

      if (JMEM_CP_GET_SECOND_BIT_FROM_POINTER_TAG (object_with_tag))
      {
        handler = *buffer_p++;
      }

      ecma_enqueue_promise_reaction_job (object, handler, value);
    }
    else if (JMEM_CP_GET_SECOND_BIT_FROM_POINTER_TAG (object_with_tag))
    {
      buffer_p++;
    }
  }
} /* ecma_promise_trigger_reactions */

/**
 * Checks whether a resolver is called before.
 *
 * @return true if it was called before, false otherwise
 */
static inline bool
ecma_is_resolver_already_called (ecma_object_t *promise_obj_p) /**< promise */
{
  return (ecma_promise_get_flags (promise_obj_p) & ECMA_PROMISE_ALREADY_RESOLVED) != 0;
} /* ecma_is_resolver_already_called */

/**
 * Reject a Promise with a reason.
 *
 * See also: ES2015 25.4.1.7
 */
void
ecma_reject_promise (ecma_value_t promise, /**< promise */
                     ecma_value_t reason) /**< reason for reject */
{
  ecma_object_t *obj_p = ecma_get_object_from_value (promise);

  JERRY_ASSERT (ecma_promise_get_flags (obj_p) & ECMA_PROMISE_IS_PENDING);

  ecma_promise_set_state (obj_p, false);
  ecma_promise_set_result (obj_p, ecma_copy_value_if_not_object (reason));
  ecma_promise_object_t *promise_p = (ecma_promise_object_t *) obj_p;

  /* GC can be triggered by ecma_new_collection so freeing the collection
     first and creating a new one might cause a heap after use event. */
  ecma_collection_t *reactions = promise_p->reactions;

  /* Fulfill reactions will never be triggered. */
  ecma_promise_trigger_reactions (reactions, reason, true);

  promise_p->reactions = ecma_new_collection ();

  ecma_collection_destroy (reactions);
} /* ecma_reject_promise */

/**
 * Fulfill a Promise with a value.
 *
 * See also: ES2015 25.4.1.4
 */
void
ecma_fulfill_promise (ecma_value_t promise, /**< promise */
                      ecma_value_t value) /**< fulfilled value */
{
  ecma_object_t *obj_p = ecma_get_object_from_value (promise);

  JERRY_ASSERT (ecma_promise_get_flags (obj_p) & ECMA_PROMISE_IS_PENDING);

  if (promise == value)
  {
    ecma_raise_type_error (ECMA_ERR_PROMISE_RESOLVE_ITSELF);
    ecma_value_t exception = jcontext_take_exception ();
    ecma_reject_promise (promise, exception);
    ecma_free_value (exception);
    return;
  }

  if (ecma_is_value_object (value))
  {
    ecma_value_t then = ecma_op_object_get_by_magic_id (ecma_get_object_from_value (value), LIT_MAGIC_STRING_THEN);

    if (ECMA_IS_VALUE_ERROR (then))
    {
      then = jcontext_take_exception ();
      ecma_reject_promise (promise, then);
      ecma_free_value (then);
      return;
    }

    if (ecma_op_is_callable (then))
    {
      ecma_enqueue_promise_resolve_thenable_job (promise, value, then);
      ecma_free_value (then);
      return;
    }

    ecma_free_value (then);
  }

  ecma_promise_set_state (obj_p, true);
  ecma_promise_set_result (obj_p, ecma_copy_value_if_not_object (value));
  ecma_promise_object_t *promise_p = (ecma_promise_object_t *) obj_p;

  /* GC can be triggered by ecma_new_collection so freeing the collection
     first and creating a new one might cause a heap after use event. */
  ecma_collection_t *reactions = promise_p->reactions;

  /* Reject reactions will never be triggered. */
  ecma_promise_trigger_reactions (reactions, value, false);

  promise_p->reactions = ecma_new_collection ();

  ecma_collection_destroy (reactions);
} /* ecma_fulfill_promise */

/**
 * Reject a Promise with a reason. Sanity checks are performed before the reject.
 *
 * See also: ES2015 25.4.1.3.1
 *
 * @return ecma value of undefined.
 */
ecma_value_t
ecma_reject_promise_with_checks (ecma_value_t promise, /**< promise */
                                 ecma_value_t reason) /**< reason for reject */
{
  /* 1. */
  ecma_object_t *promise_obj_p = ecma_get_object_from_value (promise);
  JERRY_ASSERT (ecma_is_promise (promise_obj_p));

  /* 3., 4. */
  if (JERRY_UNLIKELY (ecma_is_resolver_already_called (promise_obj_p)))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  /* 5. */
  ((ecma_extended_object_t *) promise_obj_p)->u.cls.u1.promise_flags |= ECMA_PROMISE_ALREADY_RESOLVED;

  /* 6. */
  ecma_reject_promise (promise, reason);
  return ECMA_VALUE_UNDEFINED;
} /* ecma_reject_promise_with_checks */

/**
 * Fulfill a Promise with a value. Sanity checks are performed before the resolve.
 *
 * See also: ES2015 25.4.1.3.2
 *
 * @return ecma value of undefined.
 */
ecma_value_t
ecma_fulfill_promise_with_checks (ecma_value_t promise, /**< promise */
                                  ecma_value_t value) /**< fulfilled value */
{
  /* 1. */
  ecma_object_t *promise_obj_p = ecma_get_object_from_value (promise);
  JERRY_ASSERT (ecma_is_promise (promise_obj_p));

  /* 3., 4. */
  if (JERRY_UNLIKELY (ecma_is_resolver_already_called (promise_obj_p)))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  /* 5. */
  ((ecma_extended_object_t *) promise_obj_p)->u.cls.u1.promise_flags |= ECMA_PROMISE_ALREADY_RESOLVED;

  ecma_fulfill_promise (promise, value);
  return ECMA_VALUE_UNDEFINED;
} /* ecma_fulfill_promise_with_checks */

/**
 * Native handler for Promise Reject Function.
 *
 * @return ecma value of undefined.
 */
ecma_value_t
ecma_promise_reject_handler (ecma_object_t *function_obj_p, /**< function object */
                             const ecma_value_t args_p[], /**< argument list */
                             const uint32_t args_count) /**< argument number */
{
  ecma_promise_resolver_t *function_p = (ecma_promise_resolver_t *) function_obj_p;

  ecma_value_t reject_value = (args_count == 0) ? ECMA_VALUE_UNDEFINED : args_p[0];
  return ecma_reject_promise_with_checks (function_p->promise, reject_value);
} /* ecma_promise_reject_handler */

/**
 * Native handler for Promise Resolve Function.
 *
 * @return ecma value of undefined.
 */
ecma_value_t
ecma_promise_resolve_handler (ecma_object_t *function_obj_p, /**< function object */
                              const ecma_value_t args_p[], /**< argument list */
                              const uint32_t args_count) /**< argument number */
{
  ecma_promise_resolver_t *function_p = (ecma_promise_resolver_t *) function_obj_p;

  ecma_value_t fulfilled_value = (args_count == 0) ? ECMA_VALUE_UNDEFINED : args_p[0];
  return ecma_fulfill_promise_with_checks (function_p->promise, fulfilled_value);
} /* ecma_promise_resolve_handler */

/**
 * Helper function for PromiseCreateResolvingFunctions.
 *
 * See also: ES2015 25.4.1.3 2. - 7.
 *
 * @return pointer to the resolving function
 */
static ecma_object_t *
ecma_promise_create_resolving_function (ecma_object_t *promise_p, /**< Promise Object */
                                        ecma_native_handler_id_t id) /**< Callback handler */
{
  ecma_object_t *func_obj_p = ecma_op_create_native_handler (id, sizeof (ecma_promise_resolver_t));

  ecma_promise_resolver_t *resolver_p = (ecma_promise_resolver_t *) func_obj_p;
  resolver_p->promise = ecma_make_object_value (promise_p);

  return func_obj_p;
} /* ecma_promise_create_resolving_function */

/**
 * Helper function for running an executor.
 *
 * @return ecma value of the executor callable
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_promise_run_executor (ecma_object_t *promise_p, /**< Promise Object */
                           ecma_value_t executor, /**< executor function */
                           ecma_value_t this_value) /**< this value */
{
  ecma_object_t *resolve_func_p, *reject_func_p;
  resolve_func_p = ecma_promise_create_resolving_function (promise_p, ECMA_NATIVE_HANDLER_PROMISE_RESOLVE);
  reject_func_p = ecma_promise_create_resolving_function (promise_p, ECMA_NATIVE_HANDLER_PROMISE_REJECT);

  ecma_value_t argv[] = { ecma_make_object_value (resolve_func_p), ecma_make_object_value (reject_func_p) };
  ecma_value_t result = ecma_op_function_call (ecma_get_object_from_value (executor), this_value, argv, 2);
  ecma_deref_object (resolve_func_p);
  ecma_deref_object (reject_func_p);

  return result;
} /* ecma_promise_run_executor */

/**
 * Create a promise object.
 *
 * See also: ES2015 25.4.3.1
 *
 * @return ecma value of the new promise object
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_create_promise_object (ecma_value_t executor, /**< the executor function or ECMA_VALUE_EMPTY */
                               ecma_value_t parent, /**< parent promise if available */
                               ecma_object_t *new_target_p) /**< new.target value */
{
  JERRY_UNUSED (parent);

  if (new_target_p == NULL)
  {
    new_target_p = ecma_builtin_get (ECMA_BUILTIN_ID_PROMISE);
  }

  /* 3. */
  ecma_object_t *proto_p = ecma_op_get_prototype_from_constructor (new_target_p, ECMA_BUILTIN_ID_PROMISE_PROTOTYPE);

  if (JERRY_UNLIKELY (proto_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  /* Calling ecma_new_collection might trigger a GC call, so this
   * allocation is performed before the object is constructed. */
  ecma_collection_t *reactions = ecma_new_collection ();

  ecma_object_t *object_p = ecma_create_object (proto_p, sizeof (ecma_promise_object_t), ECMA_OBJECT_TYPE_CLASS);
  ecma_deref_object (proto_p);
  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_PROMISE;
  /* 5 */
  ext_object_p->u.cls.u1.promise_flags = ECMA_PROMISE_IS_PENDING;
  ext_object_p->u.cls.u3.value = ECMA_VALUE_UNDEFINED;

  /* 6-8. */
  ecma_promise_object_t *promise_object_p = (ecma_promise_object_t *) object_p;
  promise_object_p->reactions = reactions;

  /* 9. */
  ecma_value_t completion = ECMA_VALUE_UNDEFINED;

  if (executor != ECMA_VALUE_EMPTY)
  {
    JERRY_ASSERT (ecma_op_is_callable (executor));

    completion = ecma_promise_run_executor (object_p, executor, ECMA_VALUE_UNDEFINED);
  }

  ecma_value_t status = ECMA_VALUE_EMPTY;

  if (ECMA_IS_VALUE_ERROR (completion))
  {
    /* 10.a. */
    completion = jcontext_take_exception ();
    ecma_reject_promise_with_checks (ecma_make_object_value (object_p), completion);
  }

  ecma_free_value (completion);

  /* 10.b. */
  if (ECMA_IS_VALUE_ERROR (status))
  {
    ecma_deref_object (object_p);
    return status;
  }

  /* 11. */
  ecma_free_value (status);
  return ecma_make_object_value (object_p);
} /* ecma_op_create_promise_object */

/**
 * Helper function for increase or decrease the remaining count.
 *
 * @return the current remaining count after increase or decrease.
 */
uint32_t
ecma_promise_remaining_inc_or_dec (ecma_value_t remaining, /**< the remaining count */
                                   bool is_inc) /**< whether to increase the count */
{
  JERRY_ASSERT (ecma_is_value_object (remaining));

  ecma_object_t *remaining_p = ecma_get_object_from_value (remaining);
  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) remaining_p;

  JERRY_ASSERT (ext_object_p->u.cls.type == ECMA_OBJECT_CLASS_NUMBER);

  JERRY_ASSERT (ecma_is_value_integer_number (ext_object_p->u.cls.u3.value));

  uint32_t current = (uint32_t) ecma_get_integer_from_value (ext_object_p->u.cls.u3.value);

  if (is_inc)
  {
    current++;
  }
  else
  {
    current--;
  }
  ext_object_p->u.cls.u3.value = ecma_make_uint32_value (current);

  return current;
} /* ecma_promise_remaining_inc_or_dec */

/**
 * Native handler for Promise.all and Promise.allSettled Resolve Element Function.
 *
 * See also:
 *         ES2015 25.4.4.1.2
 *
 * @return ecma value of undefined.
 */
ecma_value_t
ecma_promise_all_or_all_settled_handler_cb (ecma_object_t *function_obj_p, /**< function object */
                                            const ecma_value_t args_p[], /**< argument list */
                                            const uint32_t args_count) /**< argument number */
{
  ecma_value_t arg = args_count > 0 ? args_p[0] : ECMA_VALUE_UNDEFINED;

  ecma_promise_all_executor_t *executor_p = (ecma_promise_all_executor_t *) function_obj_p;
  uint8_t promise_type = executor_p->header.u.built_in.u2.routine_flags;

  promise_type = (uint8_t) (promise_type >> ECMA_NATIVE_HANDLER_COMMON_FLAGS_SHIFT);

  /* 1 - 2. */
  if (executor_p->index == 0)
  {
    return ECMA_VALUE_UNDEFINED;
  }

  if (promise_type == ECMA_PROMISE_ALL_RESOLVE || promise_type == ECMA_PROMISE_ANY_REJECT)
  {
    /* 8. */
    ecma_op_object_put_by_index (ecma_get_object_from_value (executor_p->values),
                                 (uint32_t) (executor_p->index - 1),
                                 arg,
                                 false);
  }
  else
  {
    lit_magic_string_id_t status_property_val = LIT_MAGIC_STRING_REJECTED;
    lit_magic_string_id_t data_property_name = LIT_MAGIC_STRING_REASON;

    if (promise_type == ECMA_PROMISE_ALLSETTLED_RESOLVE)
    {
      status_property_val = LIT_MAGIC_STRING_FULFILLED;
      data_property_name = LIT_MAGIC_STRING_VALUE;
    }

    ecma_object_t *obj_p =
      ecma_create_object (ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE), 0, ECMA_OBJECT_TYPE_GENERAL);
    ecma_property_value_t *prop_value_p;
    prop_value_p = ecma_create_named_data_property (obj_p,
                                                    ecma_get_magic_string (LIT_MAGIC_STRING_STATUS),
                                                    ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                    NULL);

    prop_value_p->value = ecma_make_magic_string_value (status_property_val);

    prop_value_p = ecma_create_named_data_property (obj_p,
                                                    ecma_get_magic_string (data_property_name),
                                                    ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                    NULL);
    prop_value_p->value = ecma_copy_value_if_not_object (arg);

    ecma_value_t obj_val = ecma_make_object_value (obj_p);
    /* 12. */
    ecma_op_object_put_by_index (ecma_get_object_from_value (executor_p->values),
                                 (uint32_t) (executor_p->index - 1),
                                 obj_val,
                                 false);
    ecma_deref_object (obj_p);
  }
  /* 3. */
  executor_p->index = 0;

  /* 9-10. */
  ecma_value_t ret = ECMA_VALUE_UNDEFINED;
  if (ecma_promise_remaining_inc_or_dec (executor_p->remaining_elements, false) == 0)
  {
    ecma_value_t capability = executor_p->capability;
    ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) ecma_get_object_from_value (capability);
    if (promise_type == ECMA_PROMISE_ANY_REJECT)
    {
      ecma_value_t error_val = ecma_new_aggregate_error (executor_p->values, ECMA_VALUE_UNDEFINED);
      ret =
        ecma_op_function_call (ecma_get_object_from_value (capability_p->reject), ECMA_VALUE_UNDEFINED, &error_val, 1);
      ecma_free_value (error_val);
    }
    else
    {
      ret = ecma_op_function_call (ecma_get_object_from_value (capability_p->resolve),
                                   ECMA_VALUE_UNDEFINED,
                                   &executor_p->values,
                                   1);
    }
  }

  return ret;
} /* ecma_promise_all_or_all_settled_handler_cb */

/**
 * GetCapabilitiesExecutor Functions
 *
 * Checks and sets a promiseCapability's resolve and reject properties.
 *
 * See also: ES11 25.6.1.5.1
 *
 * @return ECMA_VALUE_UNDEFINED or TypeError
 *         returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_get_capabilities_executor_cb (ecma_object_t *function_obj_p, /**< function object */
                                      const ecma_value_t args_p[], /**< argument list */
                                      const uint32_t args_count) /**< argument number */
{
  /* 1. */
  ecma_promise_capability_executor_t *executor_p = (ecma_promise_capability_executor_t *) function_obj_p;

  /* 2-3. */
  ecma_object_t *capability_obj_p = ecma_get_object_from_value (executor_p->capability);
  JERRY_ASSERT (ecma_object_class_is (capability_obj_p, ECMA_OBJECT_CLASS_PROMISE_CAPABILITY));
  ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) capability_obj_p;

  /* 4. */
  if (!ecma_is_value_undefined (capability_p->resolve))
  {
    return ecma_raise_type_error (ECMA_ERR_RESOLVE_MUST_BE_UNDEFINED);
  }

  /* 5. */
  if (!ecma_is_value_undefined (capability_p->reject))
  {
    return ecma_raise_type_error (ECMA_ERR_REJECT_MUST_BE_UNDEFINED);
  }

  /* 6. */
  capability_p->resolve = (args_count > 0) ? args_p[0] : ECMA_VALUE_UNDEFINED;
  /* 7. */
  capability_p->reject = (args_count > 1) ? args_p[1] : ECMA_VALUE_UNDEFINED;

  /* 8. */
  return ECMA_VALUE_UNDEFINED;
} /* ecma_op_get_capabilities_executor_cb */

/**
 * Create a new PromiseCapability.
 *
 * See also: ES11 25.6.1.5
 *
 * @return NULL - if the operation raises error
 *         new PromiseCapability object - otherwise
 */
ecma_object_t *
ecma_promise_new_capability (ecma_value_t constructor, /**< constructor function */
                             ecma_value_t parent) /**< parent promise if available */
{
  /* 1. */
  if (!ecma_is_constructor (constructor))
  {
    ecma_raise_type_error (ECMA_ERR_INVALID_CAPABILITY);
    return NULL;
  }

  ecma_object_t *constructor_obj_p = ecma_get_object_from_value (constructor);

  /* 3. */
  ecma_object_t *capability_obj_p = ecma_create_object (ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE),
                                                        sizeof (ecma_promise_capability_t),
                                                        ECMA_OBJECT_TYPE_CLASS);

  ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) capability_obj_p;
  capability_p->header.u.cls.type = ECMA_OBJECT_CLASS_PROMISE_CAPABILITY;
  capability_p->header.u.cls.u3.promise = ECMA_VALUE_UNDEFINED;
  capability_p->resolve = ECMA_VALUE_UNDEFINED;
  capability_p->reject = ECMA_VALUE_UNDEFINED;

  /* 4-5. */
  ecma_object_t *executor_p = ecma_op_create_native_handler (ECMA_NATIVE_HANDLER_PROMISE_CAPABILITY_EXECUTOR,
                                                             sizeof (ecma_promise_capability_executor_t));

  /* 6. */
  ecma_promise_capability_executor_t *executor_func_p = (ecma_promise_capability_executor_t *) executor_p;
  executor_func_p->capability = ecma_make_object_value (capability_obj_p);

  /* 7. */
  ecma_value_t executor = ecma_make_object_value (executor_p);
  ecma_value_t promise;

  if (constructor_obj_p == ecma_builtin_get (ECMA_BUILTIN_ID_PROMISE))
  {
    promise = ecma_op_create_promise_object (executor, parent, constructor_obj_p);
  }
  else
  {
    promise = ecma_op_function_construct (constructor_obj_p, constructor_obj_p, &executor, 1);
  }

  ecma_deref_object (executor_p);

  if (ECMA_IS_VALUE_ERROR (promise))
  {
    ecma_deref_object (capability_obj_p);
    return NULL;
  }

  /* 8. */
  if (!ecma_op_is_callable (capability_p->resolve))
  {
    ecma_free_value (promise);
    ecma_deref_object (capability_obj_p);
    ecma_raise_type_error (ECMA_ERR_PARAMETER_RESOLVE_MUST_BE_CALLABLE);
    return NULL;
  }

  /* 9. */
  if (!ecma_op_is_callable (capability_p->reject))
  {
    ecma_free_value (promise);
    ecma_deref_object (capability_obj_p);
    ecma_raise_type_error (ECMA_ERR_PARAMETER_REJECT_MUST_BE_CALLABLE);
    return NULL;
  }

  /* 10. */
  capability_p->header.u.cls.u3.promise = promise;

  ecma_free_value (promise);

  /* 11. */
  return capability_obj_p;
} /* ecma_promise_new_capability */

/**
 * The common function for 'reject' and 'resolve'.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_promise_reject_or_resolve (ecma_value_t this_arg, /**< "this" argument */
                                ecma_value_t value, /**< rejected or resolved value */
                                bool is_resolve) /**< the operation is resolve */
{
  if (!ecma_is_value_object (this_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  if (is_resolve && ecma_is_value_object (value) && ecma_is_promise (ecma_get_object_from_value (value)))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (value);
    ecma_value_t constructor = ecma_op_object_get_by_magic_id (object_p, LIT_MAGIC_STRING_CONSTRUCTOR);

    if (ECMA_IS_VALUE_ERROR (constructor))
    {
      return constructor;
    }

    /* The this_arg must be an object. */
    bool is_same_value = (constructor == this_arg);
    ecma_free_value (constructor);

    if (is_same_value)
    {
      return ecma_copy_value (value);
    }
  }

  ecma_object_t *capability_obj_p = ecma_promise_new_capability (this_arg, ECMA_VALUE_UNDEFINED);

  if (JERRY_UNLIKELY (capability_obj_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) capability_obj_p;

  ecma_value_t func = is_resolve ? capability_p->resolve : capability_p->reject;

  ecma_value_t call_ret = ecma_op_function_call (ecma_get_object_from_value (func), ECMA_VALUE_UNDEFINED, &value, 1);

  if (ECMA_IS_VALUE_ERROR (call_ret))
  {
    ecma_deref_object (capability_obj_p);
    return call_ret;
  }

  ecma_free_value (call_ret);

  ecma_value_t promise = ecma_copy_value (capability_p->header.u.cls.u3.promise);
  ecma_deref_object (capability_obj_p);

  return promise;
} /* ecma_promise_reject_or_resolve */

/**
 * The common function for ecma_builtin_promise_prototype_then
 * and ecma_builtin_promise_prototype_catch.
 *
 * @return ecma value of a new promise object.
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_promise_then (ecma_value_t promise, /**< the promise which call 'then' */
                   ecma_value_t on_fulfilled, /**< on_fulfilled function */
                   ecma_value_t on_rejected) /**< on_rejected function */
{
  if (!ecma_is_value_object (promise))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_object_t *obj = ecma_get_object_from_value (promise);

  if (!ecma_is_promise (obj))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_PROMISE);
  }

  ecma_value_t species = ecma_op_species_constructor (obj, ECMA_BUILTIN_ID_PROMISE);
  if (ECMA_IS_VALUE_ERROR (species))
  {
    return species;
  }

  ecma_object_t *result_capability_obj_p = ecma_promise_new_capability (species, promise);
  ecma_free_value (species);

  if (JERRY_UNLIKELY (result_capability_obj_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t ret = ecma_promise_perform_then (promise, on_fulfilled, on_rejected, result_capability_obj_p);
  ecma_deref_object (result_capability_obj_p);

  return ret;
} /* ecma_promise_then */

/**
 * Definition of valueThunk function
 *
 * See also:
 *         ES2020 25.6.5.3.1 step 8.
 *
 * @return ecma value
 */
ecma_value_t
ecma_value_thunk_helper_cb (ecma_object_t *function_obj_p, /**< function object */
                            const ecma_value_t args_p[], /**< argument list */
                            const uint32_t args_count) /**< argument number */
{
  JERRY_UNUSED_2 (args_p, args_count);

  ecma_promise_value_thunk_t *value_thunk_obj_p = (ecma_promise_value_thunk_t *) function_obj_p;

  return ecma_copy_value (value_thunk_obj_p->value);
} /* ecma_value_thunk_helper_cb */

/**
 * Definition of thrower function
 *
 * See also:
 *         ES2020 25.6.5.3.2 step 8.
 *
 * @return ecma value
 */
ecma_value_t
ecma_value_thunk_thrower_cb (ecma_object_t *function_obj_p, /**< function object */
                             const ecma_value_t args_p[], /**< argument list */
                             const uint32_t args_count) /**< argument number */
{
  JERRY_UNUSED_2 (args_p, args_count);

  ecma_promise_value_thunk_t *value_thunk_obj_p = (ecma_promise_value_thunk_t *) function_obj_p;

  jcontext_raise_exception (ecma_copy_value (value_thunk_obj_p->value));

  return ECMA_VALUE_ERROR;
} /* ecma_value_thunk_thrower_cb */

/**
 * Helper function for Then Finally and Catch Finally common parts
 *
 * See also:
 *         ES2020 25.6.5.3.1
 *         ES2020 25.6.5.3.2
 *
 * @return ecma value
 */
static ecma_value_t
ecma_promise_then_catch_finally_helper (ecma_object_t *function_obj_p, /**< function object */
                                        ecma_native_handler_id_t id, /**< handler id */
                                        ecma_value_t arg) /**< callback function argument */
{
  /* 2. */
  ecma_promise_finally_function_t *finally_func_obj = (ecma_promise_finally_function_t *) function_obj_p;

  /* 3. */
  JERRY_ASSERT (ecma_op_is_callable (finally_func_obj->on_finally));

  /* 4. */
  ecma_value_t result =
    ecma_op_function_call (ecma_get_object_from_value (finally_func_obj->on_finally), ECMA_VALUE_UNDEFINED, NULL, 0);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  /* 6. */
  JERRY_ASSERT (ecma_is_constructor (finally_func_obj->constructor));

  /* 7. */
  ecma_value_t promise = ecma_promise_reject_or_resolve (finally_func_obj->constructor, result, true);

  ecma_free_value (result);

  if (ECMA_IS_VALUE_ERROR (promise))
  {
    return promise;
  }

  /* 8. */
  ecma_object_t *value_thunk_func_p;
  value_thunk_func_p = ecma_op_create_native_handler (id, sizeof (ecma_promise_value_thunk_t));

  ecma_promise_value_thunk_t *value_thunk_func_obj = (ecma_promise_value_thunk_t *) value_thunk_func_p;
  value_thunk_func_obj->value = ecma_copy_value_if_not_object (arg);

  /* 9. */
  ecma_value_t value_thunk = ecma_make_object_value (value_thunk_func_p);
  ecma_value_t ret_value = ecma_op_invoke_by_magic_id (promise, LIT_MAGIC_STRING_THEN, &value_thunk, 1);

  ecma_free_value (promise);
  ecma_deref_object (value_thunk_func_p);

  return ret_value;
} /* ecma_promise_then_catch_finally_helper */

/**
 * Definition of Then Finally Function
 *
 * See also:
 *         ES2020 25.6.5.3.1
 *
 * @return ecma value
 */
ecma_value_t
ecma_promise_then_finally_cb (ecma_object_t *function_obj_p, /**< function object */
                              const ecma_value_t args_p[], /**< argument list */
                              const uint32_t args_count) /**< argument number */
{
  JERRY_UNUSED (args_count);
  JERRY_ASSERT (args_count > 0);

  return ecma_promise_then_catch_finally_helper (function_obj_p, ECMA_NATIVE_HANDLER_VALUE_THUNK, args_p[0]);
} /* ecma_promise_then_finally_cb */

/**
 * Definition of Catch Finally Function
 *
 * See also:
 *         ES2020 25.6.5.3.2
 *
 * @return ecma value
 */
ecma_value_t
ecma_promise_catch_finally_cb (ecma_object_t *function_obj_p, /**< function object */
                               const ecma_value_t args_p[], /**< argument list */
                               const uint32_t args_count) /**< argument number */
{
  JERRY_UNUSED (args_count);
  JERRY_ASSERT (args_count > 0);

  return ecma_promise_then_catch_finally_helper (function_obj_p, ECMA_NATIVE_HANDLER_VALUE_THROWER, args_p[0]);
} /* ecma_promise_catch_finally_cb */

/**
 * The common function for ecma_builtin_promise_prototype_finally
 *
 * @return ecma value of a new promise object.
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_promise_finally (ecma_value_t promise, /**< the promise which call 'finally' */
                      ecma_value_t on_finally) /**< on_finally function */
{
  /* 2. */
  if (!ecma_is_value_object (promise))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_object_t *obj = ecma_get_object_from_value (promise);

  /* 3. */
  ecma_value_t species = ecma_op_species_constructor (obj, ECMA_BUILTIN_ID_PROMISE);

  if (ECMA_IS_VALUE_ERROR (species))
  {
    return species;
  }

  /* 4. */
  JERRY_ASSERT (ecma_is_constructor (species));

  /* 5. */
  if (!ecma_op_is_callable (on_finally))
  {
    ecma_free_value (species);
    ecma_value_t invoke_args[2] = { on_finally, on_finally };
    return ecma_op_invoke_by_magic_id (promise, LIT_MAGIC_STRING_THEN, invoke_args, 2);
  }

  /* 6.a-b */
  ecma_object_t *then_finally_obj_p;
  then_finally_obj_p =
    ecma_op_create_native_handler (ECMA_NATIVE_HANDLER_PROMISE_THEN_FINALLY, sizeof (ecma_promise_finally_function_t));

  /* 6.c-d */
  ecma_promise_finally_function_t *then_finally_func_obj_p = (ecma_promise_finally_function_t *) then_finally_obj_p;
  then_finally_func_obj_p->constructor = species;
  then_finally_func_obj_p->on_finally = on_finally;

  /* 6.e-f */
  ecma_object_t *catch_finally_obj_p;
  catch_finally_obj_p =
    ecma_op_create_native_handler (ECMA_NATIVE_HANDLER_PROMISE_CATCH_FINALLY, sizeof (ecma_promise_finally_function_t));

  /* 6.g-h */
  ecma_promise_finally_function_t *catch_finally_func_obj = (ecma_promise_finally_function_t *) catch_finally_obj_p;
  catch_finally_func_obj->constructor = species;
  catch_finally_func_obj->on_finally = on_finally;

  ecma_deref_object (ecma_get_object_from_value (species));

  /* 7. */
  ecma_value_t invoke_args[2] = { ecma_make_object_value (then_finally_obj_p),
                                  ecma_make_object_value (catch_finally_obj_p) };

  ecma_value_t ret_value = ecma_op_invoke_by_magic_id (promise, LIT_MAGIC_STRING_THEN, invoke_args, 2);

  ecma_deref_object (then_finally_obj_p);
  ecma_deref_object (catch_finally_obj_p);

  return ret_value;
} /* ecma_promise_finally */

/**
 * Resume the execution of an async function after the promise is resolved
 */
void
ecma_promise_async_then (ecma_value_t promise, /**< promise object */
                         ecma_value_t executable_object) /**< executable object of the async function */
{
  ecma_object_t *promise_obj_p = ecma_get_object_from_value (promise);
  uint16_t flags = ecma_promise_get_flags (promise_obj_p);

  if (flags & ECMA_PROMISE_IS_PENDING)
  {
    ecma_value_t executable_object_with_tag;
    ECMA_SET_NON_NULL_POINTER_TAG (executable_object_with_tag, ecma_get_object_from_value (executable_object), 0);
    ECMA_SET_THIRD_BIT_TO_POINTER_TAG (executable_object_with_tag);

    ecma_collection_push_back (((ecma_promise_object_t *) promise_obj_p)->reactions, executable_object_with_tag);
    return;
  }

  ecma_value_t value = ecma_promise_get_result (promise_obj_p);
  ecma_enqueue_promise_async_reaction_job (executable_object, value, !(flags & ECMA_PROMISE_IS_FULFILLED));
  ecma_free_value (value);

} /* ecma_promise_async_then */

/**
 * Resolves the value and resume the execution of an async function after the resolve is completed
 *
 * @return ECMA_VALUE_UNDEFINED if not error is occurred, an error otherwise
 */
ecma_value_t
ecma_promise_async_await (ecma_extended_object_t *async_generator_object_p, /**< async generator function */
                          ecma_value_t value) /**< value to be resolved (takes the reference) */
{
  ecma_value_t promise = ecma_make_object_value (ecma_builtin_get (ECMA_BUILTIN_ID_PROMISE));
  ecma_value_t result = ecma_promise_reject_or_resolve (promise, value, true);

  ecma_free_value (value);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  ecma_promise_async_then (result, ecma_make_object_value ((ecma_object_t *) async_generator_object_p));
  ecma_free_value (result);
  return ECMA_VALUE_UNDEFINED;
} /* ecma_promise_async_await */

/**
 * Reject the promise if the value is error.
 *
 * See also:
 *         ES2015 25.4.1.1.1
 *
 * @return ecma value of the new promise.
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_op_if_abrupt_reject_promise (ecma_value_t *value_p, /**< [in - out] completion value */
                                  ecma_object_t *capability_obj_p) /**< capability */
{
  JERRY_ASSERT (ecma_object_class_is (capability_obj_p, ECMA_OBJECT_CLASS_PROMISE_CAPABILITY));

  if (!ECMA_IS_VALUE_ERROR (*value_p))
  {
    return ECMA_VALUE_EMPTY;
  }

  ecma_value_t reason = jcontext_take_exception ();

  ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) capability_obj_p;
  ecma_value_t call_ret =
    ecma_op_function_call (ecma_get_object_from_value (capability_p->reject), ECMA_VALUE_UNDEFINED, &reason, 1);
  ecma_free_value (reason);

  if (ECMA_IS_VALUE_ERROR (call_ret))
  {
    *value_p = call_ret;
    return call_ret;
  }

  ecma_free_value (call_ret);
  *value_p = ecma_copy_value (capability_p->header.u.cls.u3.promise);

  return ECMA_VALUE_EMPTY;
} /* ecma_op_if_abrupt_reject_promise */

/**
 * It performs the "then" operation on promiFulfilled
 * and onRejected as its settlement actions.
 *
 * See also: 25.4.5.3.1
 *
 * @return ecma value of the new promise object
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_promise_perform_then (ecma_value_t promise, /**< the promise which call 'then' */
                           ecma_value_t on_fulfilled, /**< on_fulfilled function */
                           ecma_value_t on_rejected, /**< on_rejected function */
                           ecma_object_t *result_capability_obj_p) /**< promise capability */
{
  JERRY_ASSERT (ecma_object_class_is (result_capability_obj_p, ECMA_OBJECT_CLASS_PROMISE_CAPABILITY));

  ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) result_capability_obj_p;

  /* 3. boolean true indicates "identity" */
  if (!ecma_op_is_callable (on_fulfilled))
  {
    on_fulfilled = ECMA_VALUE_TRUE;
  }

  /* 4. boolean false indicates "thrower" */
  if (!ecma_op_is_callable (on_rejected))
  {
    on_rejected = ECMA_VALUE_FALSE;
  }

  ecma_object_t *promise_obj_p = ecma_get_object_from_value (promise);
  ecma_promise_object_t *promise_p = (ecma_promise_object_t *) promise_obj_p;

  uint16_t flags = ecma_promise_get_flags (promise_obj_p);

  if (flags & ECMA_PROMISE_IS_PENDING)
  {
    /* 7. */
    /* [ capability, (on_fulfilled), (on_rejected) ] */
    ecma_value_t reaction_values[3];
    ecma_value_t *reactions_p = reaction_values + 1;

    uint8_t tag = 0;

    if (on_fulfilled != ECMA_VALUE_TRUE)
    {
      tag |= JMEM_FIRST_TAG_BIT_MASK;
      *reactions_p++ = on_fulfilled;
    }

    if (on_rejected != ECMA_VALUE_FALSE)
    {
      tag |= JMEM_SECOND_TAG_BIT_MASK;
      *reactions_p++ = on_rejected;
    }

    ECMA_SET_NON_NULL_POINTER_TAG (reaction_values[0], result_capability_obj_p, tag);

    uint32_t value_count = (uint32_t) (reactions_p - reaction_values);
    ecma_collection_append (promise_p->reactions, reaction_values, value_count);
  }
  else if (flags & ECMA_PROMISE_IS_FULFILLED)
  {
    /* 8. */
    ecma_value_t value = ecma_promise_get_result (promise_obj_p);
    ecma_enqueue_promise_reaction_job (ecma_make_object_value (result_capability_obj_p), on_fulfilled, value);
    ecma_free_value (value);
  }
  else
  {
    /* 9. */
    ecma_value_t reason = ecma_promise_get_result (promise_obj_p);
    ecma_enqueue_promise_reaction_job (ecma_make_object_value (result_capability_obj_p), on_rejected, reason);
    ecma_free_value (reason);
  }

  /* 10. */
  return ecma_copy_value (capability_p->header.u.cls.u3.promise);
} /* ecma_promise_perform_then */

/**
 * @}
 * @}
 */
