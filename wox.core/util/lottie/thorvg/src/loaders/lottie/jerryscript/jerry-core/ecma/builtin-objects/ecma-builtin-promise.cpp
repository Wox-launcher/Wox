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
#include "ecma-builtin-handlers.h"
#include "ecma-builtin-helpers.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-iterator-object.h"
#include "ecma-number-object.h"
#include "ecma-promise-object.h"

#include "jcontext.h"

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
  ECMA_PROMISE_ROUTINE_START = 0,
  ECMA_PROMISE_ROUTINE_REJECT,
  ECMA_PROMISE_ROUTINE_RESOLVE,
  ECMA_PROMISE_ROUTINE_RACE,
  ECMA_PROMISE_ROUTINE_ALL,
  ECMA_PROMISE_ROUTINE_ALLSETTLED,
  ECMA_PROMISE_ROUTINE_ANY,
  ECMA_PROMISE_ROUTINE_SPECIES_GET
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-promise.inc.h"
#define BUILTIN_UNDERSCORED_ID  promise
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup promise ECMA Promise object built-in
 * @{
 */

/**
 * Runtime Semantics: PerformPromiseRace.
 *
 * See also:
 *         ES2020 25.6.4.4.1
 *
 * @return ecma value of the new promise.
 *         Returned value must be freed with ecma_free_value.
 */
static inline ecma_value_t
ecma_builtin_promise_perform_race (ecma_value_t iterator, /**< the iterator for race */
                                   ecma_value_t next_method, /**< next method */
                                   ecma_object_t *capability_obj_p, /**< PromiseCapability record */
                                   ecma_value_t ctor, /**< Constructor value */
                                   bool *done_p) /**< [out] iteratorRecord[[done]] */
{
  JERRY_ASSERT (ecma_is_value_object (iterator));
  JERRY_ASSERT (ecma_object_class_is (capability_obj_p, ECMA_OBJECT_CLASS_PROMISE_CAPABILITY));
  JERRY_ASSERT (ecma_is_constructor (ctor));

  ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) capability_obj_p;

  ecma_value_t resolve = ecma_op_object_get_by_magic_id (ecma_get_object_from_value (ctor), LIT_MAGIC_STRING_RESOLVE);

  if (ECMA_IS_VALUE_ERROR (resolve))
  {
    return resolve;
  }

  if (!ecma_op_is_callable (resolve))
  {
    ecma_free_value (resolve);
    return ecma_raise_type_error (ECMA_ERR_RESOLVE_METHOD_MUST_BE_CALLABLE);
  }

  ecma_object_t *resolve_func_p = ecma_get_object_from_value (resolve);
  ecma_value_t ret_value = ECMA_VALUE_ERROR;

  /* 5. */
  while (true)
  {
    /* a. */
    ecma_value_t next = ecma_op_iterator_step (iterator, next_method);
    /* b, c. */
    if (ECMA_IS_VALUE_ERROR (next))
    {
      goto done;
    }

    /* d. */
    if (ecma_is_value_false (next))
    {
      /* ii. */
      ret_value = ecma_copy_value (capability_p->header.u.cls.u3.promise);
      goto done;
    }

    /* e. */
    ecma_value_t next_val = ecma_op_iterator_value (next);
    ecma_free_value (next);

    /* f, g. */
    if (ECMA_IS_VALUE_ERROR (next_val))
    {
      goto done;
    }

    /* h. */
    ecma_value_t next_promise = ecma_op_function_call (resolve_func_p, ctor, &next_val, 1);
    ecma_free_value (next_val);

    if (ECMA_IS_VALUE_ERROR (next_promise))
    {
      goto exit;
    }

    /* i. */
    ecma_value_t args[2] = { capability_p->resolve, capability_p->reject };
    ecma_value_t result = ecma_op_invoke_by_magic_id (next_promise, LIT_MAGIC_STRING_THEN, args, 2);
    ecma_free_value (next_promise);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto exit;
    }

    ecma_free_value (result);
  }

done:
  *done_p = true;
exit:
  ecma_deref_object (resolve_func_p);

  return ret_value;
} /* ecma_builtin_promise_perform_race */

/**
 * Runtime Semantics: Perform Promise all, allSettled or any.
 *
 * See also:
 *         ES2020 25.6.4.1.1
 *
 * @return ecma value of the new promise.
 *         Returned value must be freed with ecma_free_value.
 */
static inline ecma_value_t
ecma_builtin_promise_perform (ecma_value_t iterator, /**< iteratorRecord */
                              ecma_value_t next_method, /**< next method */
                              ecma_object_t *capability_obj_p, /**< PromiseCapability record */
                              ecma_value_t ctor, /**< the caller of Promise.all */
                              uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                              bool *done_p) /**< [out] iteratorRecord[[done]] */
{
  /* 1. - 2. */
  JERRY_ASSERT (ecma_object_class_is (capability_obj_p, ECMA_OBJECT_CLASS_PROMISE_CAPABILITY));
  JERRY_ASSERT (ecma_is_constructor (ctor));

  ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) capability_obj_p;

  ecma_value_t resolve = ecma_op_object_get_by_magic_id (ecma_get_object_from_value (ctor), LIT_MAGIC_STRING_RESOLVE);

  if (ECMA_IS_VALUE_ERROR (resolve))
  {
    return resolve;
  }

  if (!ecma_op_is_callable (resolve))
  {
    ecma_free_value (resolve);
    return ecma_raise_type_error (ECMA_ERR_RESOLVE_METHOD_MUST_BE_CALLABLE);
  }

  ecma_object_t *resolve_func_p = ecma_get_object_from_value (resolve);

  /* 3. */
  ecma_object_t *values_array_obj_p = ecma_op_new_array_object (0);
  ecma_value_t values_array = ecma_make_object_value (values_array_obj_p);
  /* 4. */
  ecma_value_t remaining = ecma_op_create_number_object (ecma_make_integer_value (1));
  /* 5. */
  uint32_t idx = 0;

  ecma_value_t ret_value = ECMA_VALUE_ERROR;

  /* 6. */
  while (true)
  {
    /* a. */
    ecma_value_t next = ecma_op_iterator_step (iterator, next_method);
    /* b. - c. */
    if (ECMA_IS_VALUE_ERROR (next))
    {
      goto done;
    }

    /* d. */
    if (ecma_is_value_false (next))
    {
      /* ii. - iii. */
      if (ecma_promise_remaining_inc_or_dec (remaining, false) == 0)
      {
        if (builtin_routine_id == ECMA_PROMISE_ROUTINE_ANY)
        {
          ret_value = ecma_raise_aggregate_error (values_array, ECMA_VALUE_UNDEFINED);
          goto done;
        }

        /* 2. */
        ecma_value_t resolve_result = ecma_op_function_call (ecma_get_object_from_value (capability_p->resolve),
                                                             ECMA_VALUE_UNDEFINED,
                                                             &values_array,
                                                             1);
        /* 3. */
        if (ECMA_IS_VALUE_ERROR (resolve_result))
        {
          goto done;
        }

        ecma_free_value (resolve_result);
      }

      /* iv. */
      ret_value = ecma_copy_value (capability_p->header.u.cls.u3.promise);
      goto done;
    }

    /* e. */
    ecma_value_t next_value = ecma_op_iterator_value (next);
    ecma_free_value (next);

    /* f. - g. */
    if (ECMA_IS_VALUE_ERROR (next_value))
    {
      goto done;
    }

    /* h. */
    ecma_builtin_helper_def_prop_by_index (values_array_obj_p,
                                           idx,
                                           ECMA_VALUE_UNDEFINED,
                                           ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);

    /* i. */
    ecma_value_t next_promise = ecma_op_function_call (resolve_func_p, ctor, &next_value, 1);
    ecma_free_value (next_value);

    /* j. */
    if (ECMA_IS_VALUE_ERROR (next_promise))
    {
      goto exit;
    }

    if (JERRY_UNLIKELY (idx == UINT32_MAX - 1))
    {
      ecma_raise_range_error (ECMA_ERR_PROMISE_ALL_REMAINING_ELEMENTS_LIMIT_REACHED);
      goto exit;
    }

    idx++;
    ecma_value_t args[2];
    ecma_object_t *executor_func_p = NULL;

    if (builtin_routine_id != ECMA_PROMISE_ROUTINE_ANY)
    {
      /* k. */
      executor_func_p =
        ecma_op_create_native_handler (ECMA_NATIVE_HANDLER_PROMISE_ALL_HELPER, sizeof (ecma_promise_all_executor_t));

      ecma_promise_all_executor_t *executor_p = (ecma_promise_all_executor_t *) executor_func_p;

      /* m. + t. */
      executor_p->index = idx;

      /* n. */
      executor_p->values = values_array;

      /* o. */
      executor_p->capability = ecma_make_object_value (capability_obj_p);

      /* p. */
      executor_p->remaining_elements = remaining;

      uint8_t executor_type = ECMA_PROMISE_ALL_RESOLVE << ECMA_NATIVE_HANDLER_COMMON_FLAGS_SHIFT;

      if (builtin_routine_id == ECMA_PROMISE_ROUTINE_ALLSETTLED)
      {
        executor_type = ECMA_PROMISE_ALLSETTLED_RESOLVE << ECMA_NATIVE_HANDLER_COMMON_FLAGS_SHIFT;
      }

      executor_p->header.u.built_in.u2.routine_flags |= executor_type;

      args[0] = ecma_make_object_value (executor_func_p);
    }
    else
    {
      args[0] = capability_p->resolve;
    }

    /* q. */
    ecma_promise_remaining_inc_or_dec (remaining, true);
    ecma_value_t result;

    if (builtin_routine_id != ECMA_PROMISE_ROUTINE_ALL)
    {
      uint8_t executor_type = ECMA_PROMISE_ALLSETTLED_REJECT << ECMA_NATIVE_HANDLER_COMMON_FLAGS_SHIFT;

      if (builtin_routine_id == ECMA_PROMISE_ROUTINE_ANY)
      {
        executor_type = ECMA_PROMISE_ANY_REJECT << ECMA_NATIVE_HANDLER_COMMON_FLAGS_SHIFT;
      }

      ecma_object_t *reject_func_p =
        ecma_op_create_native_handler (ECMA_NATIVE_HANDLER_PROMISE_ALL_HELPER, sizeof (ecma_promise_all_executor_t));

      ecma_promise_all_executor_t *reject_p = (ecma_promise_all_executor_t *) reject_func_p;
      reject_p->index = idx;
      reject_p->values = values_array;
      reject_p->capability = ecma_make_object_value (capability_obj_p);
      reject_p->remaining_elements = remaining;
      reject_p->header.u.built_in.u2.routine_flags |= executor_type;
      args[1] = ecma_make_object_value (reject_func_p);
      result = ecma_op_invoke_by_magic_id (next_promise, LIT_MAGIC_STRING_THEN, args, 2);
      ecma_deref_object (reject_func_p);
    }
    else
    {
      args[1] = capability_p->reject;
      result = ecma_op_invoke_by_magic_id (next_promise, LIT_MAGIC_STRING_THEN, args, 2);
    }

    ecma_free_value (next_promise);

    if (builtin_routine_id != ECMA_PROMISE_ROUTINE_ANY)
    {
      ecma_deref_object (executor_func_p);
    }

    /* s. */
    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto exit;
    }

    ecma_free_value (result);
  }

done:
  *done_p = true;
exit:
  ecma_free_value (remaining);
  ecma_deref_object (values_array_obj_p);
  ecma_deref_object (resolve_func_p);

  return ret_value;
} /* ecma_builtin_promise_perform */

/**
 * The common function for Promise.race, Promise.all, Promise.any and Promise.allSettled.
 *
 * @return ecma value of the new promise.
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_promise_helper (ecma_value_t this_arg, /**< 'this' argument */
                             ecma_value_t iterable, /**< the items to be resolved */
                             uint8_t builtin_routine_id) /**< built-in wide routine identifier */
{
  ecma_object_t *capability_obj_p = ecma_promise_new_capability (this_arg, ECMA_VALUE_UNDEFINED);

  if (JERRY_UNLIKELY (capability_obj_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t next_method;
  ecma_value_t iterator = ecma_op_get_iterator (iterable, ECMA_VALUE_SYNC_ITERATOR, &next_method);
  ecma_value_t ret = ECMA_VALUE_ERROR;

  if (ECMA_IS_VALUE_ERROR (ecma_op_if_abrupt_reject_promise (&iterator, capability_obj_p)))
  {
    ecma_free_value (next_method);
    ecma_deref_object (capability_obj_p);
    return iterator;
  }

  bool is_done = false;

  if (builtin_routine_id == ECMA_PROMISE_ROUTINE_RACE)
  {
    ret = ecma_builtin_promise_perform_race (iterator, next_method, capability_obj_p, this_arg, &is_done);
  }
  else
  {
    ret =
      ecma_builtin_promise_perform (iterator, next_method, capability_obj_p, this_arg, builtin_routine_id, &is_done);
  }

  if (ECMA_IS_VALUE_ERROR (ret))
  {
    if (!is_done)
    {
      ret = ecma_op_iterator_close (iterator);
    }

    ecma_op_if_abrupt_reject_promise (&ret, capability_obj_p);
  }

  ecma_free_value (iterator);
  ecma_free_value (next_method);
  ecma_deref_object (capability_obj_p);

  return ret;
} /* ecma_builtin_promise_helper */

/**
 * Handle calling [[Call]] of built-in Promise object.
 *
 * ES2015 25.4.3 Promise is not intended to be called
 * as a function and will throw an exception when called
 * in that manner.
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_promise_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                    uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_raise_type_error (ECMA_ERR_CONSTRUCTOR_PROMISE_REQUIRES_NEW);
} /* ecma_builtin_promise_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Promise object.
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_promise_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                         uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  if (arguments_list_len == 0 || !ecma_op_is_callable (arguments_list_p[0]))
  {
    return ecma_raise_type_error (ECMA_ERR_FIRST_PARAMETER_MUST_BE_CALLABLE);
  }

  return ecma_op_create_promise_object (arguments_list_p[0],
                                        ECMA_VALUE_UNDEFINED,
                                        JERRY_CONTEXT (current_new_target_p));
} /* ecma_builtin_promise_dispatch_construct */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_promise_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine
                                                                    *   identifier */
                                       ecma_value_t this_arg, /**< 'this' argument value */
                                       const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                               *   passed to routine */
                                       uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (arguments_number);

  switch (builtin_routine_id)
  {
    case ECMA_PROMISE_ROUTINE_REJECT:
    case ECMA_PROMISE_ROUTINE_RESOLVE:
    {
      bool is_resolve = (builtin_routine_id == ECMA_PROMISE_ROUTINE_RESOLVE);
      return ecma_promise_reject_or_resolve (this_arg, arguments_list_p[0], is_resolve);
    }
    case ECMA_PROMISE_ROUTINE_RACE:
    case ECMA_PROMISE_ROUTINE_ALL:
    case ECMA_PROMISE_ROUTINE_ALLSETTLED:
    case ECMA_PROMISE_ROUTINE_ANY:
    {
      return ecma_builtin_promise_helper (this_arg, arguments_list_p[0], builtin_routine_id);
    }
    case ECMA_PROMISE_ROUTINE_SPECIES_GET:
    {
      return ecma_copy_value (this_arg);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_promise_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
