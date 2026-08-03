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

#include "ecma-async-generator-object.h"

#include "ecma-alloc.h"
#include "ecma-builtins.h"
#include "ecma-errors.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-objects.h"
#include "ecma-promise-object.h"

#include "jcontext.h"
#include "opcodes.h"
#include "vm-stack.h"
#include "vm.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaasyncgeneratorobject ECMA AsyncGenerator object related routines
 * @{
 */

/**
 * Enqueue a task into the command queue of an async generator
 *
 * @return ecma Promise value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_async_generator_enqueue (vm_executable_object_t *async_generator_object_p, /**< async generator */
                              ecma_async_generator_operation_type_t operation, /**< operation */
                              ecma_value_t value) /**< value argument of operation */
{
  ecma_async_generator_task_t *task_p = (ecma_async_generator_task_t *) jmem_heap_alloc_block (sizeof (ecma_async_generator_task_t));

  ECMA_SET_INTERNAL_VALUE_ANY_POINTER (task_p->next, NULL);
  task_p->operation_value = ecma_copy_value_if_not_object (value);
  task_p->operation_type = (uint8_t) operation;

  ecma_value_t result = ecma_op_create_promise_object (ECMA_VALUE_EMPTY, ECMA_VALUE_UNDEFINED, NULL);
  task_p->promise = result;

  ecma_value_t head = async_generator_object_p->extended_object.u.cls.u3.head;

  if (ECMA_IS_INTERNAL_VALUE_NULL (head))
  {
    ECMA_SET_INTERNAL_VALUE_POINTER (async_generator_object_p->extended_object.u.cls.u3.head, task_p);

    if (async_generator_object_p->extended_object.u.cls.u2.executable_obj_flags & ECMA_ASYNC_GENERATOR_CALLED)
    {
      ecma_value_t executable_object = ecma_make_object_value ((ecma_object_t *) async_generator_object_p);
      ecma_enqueue_promise_async_generator_job (executable_object);
      return result;
    }

    async_generator_object_p->extended_object.u.cls.u2.executable_obj_flags |= ECMA_ASYNC_GENERATOR_CALLED;
    ecma_async_generator_run (async_generator_object_p);
    return result;
  }

  /* Append the new task at the end. */
  ecma_async_generator_task_t *prev_task_p;
  prev_task_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_async_generator_task_t, head);

  while (!ECMA_IS_INTERNAL_VALUE_NULL (prev_task_p->next))
  {
    prev_task_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_async_generator_task_t, prev_task_p->next);
  }

  ECMA_SET_INTERNAL_VALUE_POINTER (prev_task_p->next, task_p);
  return result;
} /* ecma_async_generator_enqueue */

/**
 * Call a function and await its return value
 *
 * @return ECMA_VALUE_UNDEFINED on success, error otherwise
 */
static ecma_value_t
ecma_async_yield_call (ecma_value_t function, /**< function (takes reference) */
                       vm_executable_object_t *async_generator_object_p, /**< async generator */
                       ecma_value_t argument) /**< argument passed to the function */
{
  ecma_value_t iterator = async_generator_object_p->iterator;
  ecma_value_t result;

  if (argument == ECMA_VALUE_EMPTY)
  {
    result = ecma_op_function_validated_call (function, iterator, NULL, 0);
  }
  else
  {
    result = ecma_op_function_validated_call (function, iterator, &argument, 1);
  }

  ecma_free_value (function);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  return ecma_promise_async_await ((ecma_extended_object_t *) async_generator_object_p, result);
} /* ecma_async_yield_call */

/**
 * Perform an exception throw and call the appropriate handler
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_async_yield_throw (vm_executable_object_t *async_generator_object_p, /**< async generator */
                        ecma_value_t value) /**< thrown value */
{
  ecma_object_t *obj_p = ecma_get_object_from_value (async_generator_object_p->iterator);
  ecma_value_t result = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_THROW);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  if (result == ECMA_VALUE_UNDEFINED)
  {
    result = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_RETURN);

    if (result == ECMA_VALUE_UNDEFINED)
    {
      return ecma_raise_type_error (ECMA_ERR_ITERATOR_THROW_IS_NOT_AVAILABLE);
    }

    result = ecma_async_yield_call (result, async_generator_object_p, ECMA_VALUE_EMPTY);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      return result;
    }

    ECMA_AWAIT_CHANGE_STATE (async_generator_object_p, YIELD_OPERATION, YIELD_CLOSE);
    return ECMA_VALUE_UNDEFINED;
  }

  result = ecma_async_yield_call (result, async_generator_object_p, value);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  ECMA_AWAIT_CHANGE_STATE (async_generator_object_p, YIELD_OPERATION, YIELD_NEXT);
  return ECMA_VALUE_UNDEFINED;
} /* ecma_async_yield_throw */

/**
 * Execute the next task in the command queue of the async generator
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_async_generator_run (vm_executable_object_t *async_generator_object_p) /**< async generator */
{
  JERRY_ASSERT (async_generator_object_p->extended_object.u.cls.type == ECMA_OBJECT_CLASS_ASYNC_GENERATOR);
  JERRY_ASSERT (!ECMA_IS_INTERNAL_VALUE_NULL (async_generator_object_p->extended_object.u.cls.u3.head));

  ecma_value_t head = async_generator_object_p->extended_object.u.cls.u3.head;
  ecma_async_generator_task_t *task_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_async_generator_task_t, head);
  ecma_value_t result;

  if (async_generator_object_p->extended_object.u.cls.u2.executable_obj_flags
      & ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD)
  {
    switch (task_p->operation_type)
    {
      case ECMA_ASYNC_GENERATOR_DO_NEXT:
      {
        result = ecma_op_iterator_next (async_generator_object_p->iterator,
                                        async_generator_object_p->frame_ctx.stack_top_p[-1],
                                        task_p->operation_value);

        if (ECMA_IS_VALUE_ERROR (result))
        {
          break;
        }

        result = ecma_promise_async_await ((ecma_extended_object_t *) async_generator_object_p, result);

        if (ECMA_IS_VALUE_ERROR (result))
        {
          break;
        }

        ECMA_AWAIT_CHANGE_STATE (async_generator_object_p, YIELD_OPERATION, YIELD_NEXT);
        break;
      }
      case ECMA_ASYNC_GENERATOR_DO_THROW:
      {
        result = ecma_async_yield_throw (async_generator_object_p, task_p->operation_value);
        break;
      }
      default:
      {
        JERRY_ASSERT (task_p->operation_type == ECMA_ASYNC_GENERATOR_DO_RETURN);

        result = ecma_copy_value (task_p->operation_value);
        result = ecma_promise_async_await ((ecma_extended_object_t *) async_generator_object_p, result);

        if (ECMA_IS_VALUE_ERROR (result))
        {
          break;
        }

        ECMA_AWAIT_CHANGE_STATE (async_generator_object_p, YIELD_OPERATION, YIELD_RETURN);
        break;
      }
    }

    ecma_free_value_if_not_object (task_p->operation_value);
    task_p->operation_value = ECMA_VALUE_UNDEFINED;

    if (result == ECMA_VALUE_UNDEFINED)
    {
      return ECMA_VALUE_UNDEFINED;
    }

    JERRY_ASSERT (ECMA_IS_VALUE_ERROR (result));

    async_generator_object_p->extended_object.u.cls.u2.executable_obj_flags &= ECMA_AWAIT_CLEAR_MASK;
    async_generator_object_p->iterator = ECMA_VALUE_UNDEFINED;
    async_generator_object_p->frame_ctx.byte_code_p = opfunc_resume_executable_object_with_throw;

    JERRY_ASSERT (async_generator_object_p->frame_ctx.stack_top_p[-1] == ECMA_VALUE_UNDEFINED
                  || ecma_is_value_object (async_generator_object_p->frame_ctx.stack_top_p[-1]));
    async_generator_object_p->frame_ctx.stack_top_p--;

    result = jcontext_take_exception ();
  }
  else
  {
    if (task_p->operation_type == ECMA_ASYNC_GENERATOR_DO_RETURN)
    {
      async_generator_object_p->frame_ctx.byte_code_p = opfunc_resume_executable_object_with_return;
    }
    else if (task_p->operation_type == ECMA_ASYNC_GENERATOR_DO_THROW)
    {
      async_generator_object_p->frame_ctx.byte_code_p = opfunc_resume_executable_object_with_throw;
    }

    result = task_p->operation_value;
    ecma_ref_if_object (result);
    task_p->operation_value = ECMA_VALUE_UNDEFINED;
  }

  result = opfunc_resume_executable_object (async_generator_object_p, result);

  if (async_generator_object_p->extended_object.u.cls.u2.executable_obj_flags & ECMA_EXECUTABLE_OBJECT_COMPLETED)
  {
    JERRY_ASSERT (head == async_generator_object_p->extended_object.u.cls.u3.head);
    ecma_async_generator_finalize (async_generator_object_p, result);
    result = ECMA_VALUE_UNDEFINED;
  }

  return result;
} /* ecma_async_generator_run */

/**
 * Finalize the promises of an executable generator
 */
void
ecma_async_generator_finalize (vm_executable_object_t *async_generator_object_p, /**< async generator */
                               ecma_value_t value) /**< final value (takes reference) */
{
  ecma_value_t next = async_generator_object_p->extended_object.u.cls.u3.head;
  ecma_async_generator_task_t *task_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_async_generator_task_t, next);

  if (ECMA_IS_VALUE_ERROR (value))
  {
    value = jcontext_take_exception ();
    ecma_reject_promise (task_p->promise, value);
  }
  else
  {
    ecma_value_t result = ecma_create_iter_result_object (value, ECMA_VALUE_TRUE);
    ecma_fulfill_promise (task_p->promise, result);
    ecma_free_value (result);
  }

  ecma_free_value (value);

  next = task_p->next;
  async_generator_object_p->extended_object.u.cls.u3.head = next;
  jmem_heap_free_block (task_p, sizeof (ecma_async_generator_task_t));

  while (!ECMA_IS_INTERNAL_VALUE_NULL (next))
  {
    task_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_async_generator_task_t, next);

    if (task_p->operation_type != ECMA_ASYNC_GENERATOR_DO_THROW)
    {
      value = ecma_create_iter_result_object (ECMA_VALUE_UNDEFINED, ECMA_VALUE_TRUE);
      ecma_fulfill_promise (task_p->promise, value);
      ecma_free_value (value);
    }
    else
    {
      ecma_reject_promise (task_p->promise, task_p->operation_value);
    }

    ecma_free_value_if_not_object (task_p->operation_value);

    next = task_p->next;
    async_generator_object_p->extended_object.u.cls.u3.head = next;
    jmem_heap_free_block (task_p, sizeof (ecma_async_generator_task_t));
  }
} /* ecma_async_generator_finalize */

/**
 * Continue after an await operation is completed.
 *
 * @return an updated value for the value argument
 */
ecma_value_t
ecma_await_continue (vm_executable_object_t *executable_object_p, /**< executable object */
                     ecma_value_t value) /**< job value (takes reference) */
{
  ecma_await_states_t state = (ecma_await_states_t) ECMA_AWAIT_GET_STATE (executable_object_p);

  switch (state)
  {
    case ECMA_AWAIT_YIELD_NEXT:
    case ECMA_AWAIT_YIELD_NEXT_RETURN:
    {
      if (!ecma_is_value_object (value))
      {
        ecma_free_value (value);
        return ecma_raise_type_error (ECMA_ERR_VALUE_RECEIVED_BY_YIELD_IS_NOT_OBJECT);
      }

      ecma_object_t *result_obj_p = ecma_get_object_from_value (value);
      ecma_value_t result = ecma_op_object_get_by_magic_id (result_obj_p, LIT_MAGIC_STRING_DONE);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        ecma_deref_object (result_obj_p);
        return result;
      }

      bool done = ecma_op_to_boolean (result);
      ecma_free_value (result);
      result = ecma_op_object_get_by_magic_id (result_obj_p, LIT_MAGIC_STRING_VALUE);
      ecma_deref_object (result_obj_p);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        return result;
      }

      if (!done)
      {
        ECMA_AWAIT_SET_STATE (executable_object_p, YIELD_NEXT_VALUE);
        return ecma_promise_async_await ((ecma_extended_object_t *) executable_object_p, result);
      }

      ECMA_EXECUTABLE_OBJECT_RESUME_EXEC (executable_object_p);

      if (state == ECMA_AWAIT_YIELD_NEXT_RETURN)
      {
        executable_object_p->frame_ctx.byte_code_p = opfunc_resume_executable_object_with_return;
      }
      return result;
    }
    case ECMA_AWAIT_YIELD_RETURN:
    {
      ecma_object_t *obj_p = ecma_get_object_from_value (executable_object_p->iterator);
      ecma_value_t result = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_RETURN);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        ecma_free_value (value);
        return result;
      }

      if (result == ECMA_VALUE_UNDEFINED)
      {
        ECMA_EXECUTABLE_OBJECT_RESUME_EXEC (executable_object_p);
        executable_object_p->frame_ctx.byte_code_p = opfunc_resume_executable_object_with_return;
        return value;
      }

      result = ecma_async_yield_call (result, executable_object_p, value);
      ecma_free_value (value);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        return result;
      }

      JERRY_ASSERT (result == ECMA_VALUE_UNDEFINED);
      ECMA_AWAIT_CHANGE_STATE (executable_object_p, YIELD_RETURN, YIELD_NEXT_RETURN);
      return ECMA_VALUE_UNDEFINED;
    }
    case ECMA_AWAIT_YIELD_NEXT_VALUE:
    {
      ECMA_AWAIT_CHANGE_STATE (executable_object_p, YIELD_NEXT_VALUE, YIELD_OPERATION);
      opfunc_async_generator_yield ((ecma_extended_object_t *) executable_object_p, value);
      return ECMA_VALUE_UNDEFINED;
    }
    case ECMA_AWAIT_YIELD_OPERATION:
    {
      /* Currently this is always a throw exception case. */
      ecma_value_t result = ecma_async_yield_throw (executable_object_p, value);
      ecma_free_value (value);
      return result;
    }
    case ECMA_AWAIT_YIELD_CLOSE:
    {
      ecma_error_msg_t msg = (ecma_is_value_object (value) ? ECMA_ERR_ITERATOR_THROW_IS_NOT_AVAILABLE
                                                           : ECMA_ERR_VALUE_RECEIVED_BY_YIELD_IS_NOT_OBJECT);

      ecma_free_value (value);
      return ecma_raise_type_error (msg);
    }
    case ECMA_AWAIT_FOR_CLOSE:
    {
      bool is_value_object = ecma_is_value_object (value);
      ecma_free_value (value);
      ECMA_EXECUTABLE_OBJECT_RESUME_EXEC (executable_object_p);

      if (!is_value_object
          && VM_GET_CONTEXT_TYPE (executable_object_p->frame_ctx.stack_top_p[-1]) != VM_CONTEXT_FINALLY_THROW)
      {
        return ecma_raise_type_error (ECMA_ERR_ITERATOR_RETURN_RESULT_IS_NOT_OBJECT);
      }
      return ECMA_VALUE_EMPTY;
    }
    default:
    {
      JERRY_ASSERT (state == ECMA_AWAIT_FOR_NEXT);
      JERRY_ASSERT (VM_GET_CONTEXT_TYPE (executable_object_p->frame_ctx.stack_top_p[-1]) == VM_CONTEXT_FOR_AWAIT_OF);
      JERRY_ASSERT (!(executable_object_p->frame_ctx.stack_top_p[-1] & VM_CONTEXT_CLOSE_ITERATOR));

      if (!ecma_is_value_object (value))
      {
        ecma_free_value (value);
        return ecma_raise_type_error (ECMA_ERR_VALUE_RECEIVED_BY_FOR_ASYNC_OF_IS_NOT_OBJECT);
      }

      ecma_object_t *result_obj_p = ecma_get_object_from_value (value);
      ecma_value_t result = ecma_op_object_get_by_magic_id (result_obj_p, LIT_MAGIC_STRING_DONE);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        ecma_deref_object (result_obj_p);
        return result;
      }

      bool done = ecma_op_to_boolean (result);
      ecma_free_value (result);

      ecma_value_t *stack_top_p = executable_object_p->frame_ctx.stack_top_p;
      JERRY_ASSERT (stack_top_p[-2] == ECMA_VALUE_UNDEFINED);
      JERRY_ASSERT (ecma_is_value_object (stack_top_p[-3]));
      JERRY_ASSERT (stack_top_p[-4] == ECMA_VALUE_UNDEFINED || ecma_is_value_object (stack_top_p[-4]));

      if (!done)
      {
        result = ecma_op_object_get_by_magic_id (result_obj_p, LIT_MAGIC_STRING_VALUE);
        ecma_deref_object (result_obj_p);

        if (ECMA_IS_VALUE_ERROR (result))
        {
          return result;
        }

        /* It seems browsers call Await(result) here, although the standard does not
         * requests to do so. The following code might follow browsers in the future. */
        ecma_deref_if_object (result);
        stack_top_p[-1] |= VM_CONTEXT_CLOSE_ITERATOR;
        stack_top_p[-2] = result;
        ECMA_EXECUTABLE_OBJECT_RESUME_EXEC (executable_object_p);
        return ECMA_VALUE_EMPTY;
      }

      ecma_deref_object (result_obj_p);

      /* This code jumps to the end regardless of the byte code which triggered this await. */
      uint32_t context_end = VM_GET_CONTEXT_END (stack_top_p[-1]);
      executable_object_p->frame_ctx.byte_code_p = executable_object_p->frame_ctx.byte_code_start_p + context_end;

      VM_MINUS_EQUAL_U16 (executable_object_p->frame_ctx.context_depth, PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION);
      stack_top_p -= PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION;
      executable_object_p->frame_ctx.stack_top_p = stack_top_p;

      ECMA_EXECUTABLE_OBJECT_RESUME_EXEC (executable_object_p);
      return ECMA_VALUE_EMPTY;
    }
  }
} /* ecma_await_continue */

/**
 * @}
 * @}
 */
