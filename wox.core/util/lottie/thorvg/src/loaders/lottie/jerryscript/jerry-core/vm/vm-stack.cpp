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

#include "vm-stack.h"

#include "ecma-alloc.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-objects.h"
#include "ecma-promise-object.h"

#include "jcontext.h"
#include "vm-defines.h"

/** \addtogroup vm Virtual machine
 * @{
 *
 * \addtogroup stack VM stack
 * @{
 */

JERRY_STATIC_ASSERT (((int)PARSER_WITH_CONTEXT_STACK_ALLOCATION == (int)PARSER_BLOCK_CONTEXT_STACK_ALLOCATION),
                     with_context_stack_allocation_must_be_equal_to_block_context_stack_allocation);

JERRY_STATIC_ASSERT (((int)PARSER_WITH_CONTEXT_STACK_ALLOCATION == (int)PARSER_TRY_CONTEXT_STACK_ALLOCATION),
                     with_context_stack_allocation_must_be_equal_to_block_context_stack_allocation);

JERRY_STATIC_ASSERT (((int)PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION == (int)PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION),
                     for_of_context_stack_allocation_must_be_equal_to_for_await_of_context_stack_allocation);

/**
 * Abort (finalize) the current variable length stack context, and remove it.
 *
 * @return new stack top
 */
ecma_value_t *
vm_stack_context_abort_variable_length (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                                        ecma_value_t *vm_stack_top_p, /**< current stack top */
                                        uint32_t context_stack_allocation) /**< 0 - if all context element
                                                                            *       should be released
                                                                            *   context stack allocation - otherwise */
{
  JERRY_ASSERT (VM_CONTEXT_IS_VARIABLE_LENGTH (VM_GET_CONTEXT_TYPE (vm_stack_top_p[-1])));

  uint32_t context_size = VM_GET_CONTEXT_END (vm_stack_top_p[-1]);
  VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, context_size);

  JERRY_ASSERT (context_size > 0);
  --vm_stack_top_p;

  if (context_stack_allocation == 0)
  {
    context_stack_allocation = context_size;
  }

  for (uint32_t i = 1; i < context_stack_allocation; i++)
  {
    ecma_free_value (*(--vm_stack_top_p));
  }

  return vm_stack_top_p;
} /* vm_stack_context_abort_variable_length */

/**
 * Abort (finalize) the current stack context, and remove it.
 *
 * @return new stack top
 */
ecma_value_t *
vm_stack_context_abort (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                        ecma_value_t *vm_stack_top_p) /**< current stack top */
{
  ecma_value_t context_info = vm_stack_top_p[-1];

  if (context_info & VM_CONTEXT_HAS_LEX_ENV)
  {
    ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;
    JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
    frame_ctx_p->lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
    ecma_deref_object (lex_env_p);
  }

  switch (VM_GET_CONTEXT_TYPE (context_info))
  {
    case VM_CONTEXT_FINALLY_THROW:
    case VM_CONTEXT_FINALLY_RETURN:
    {
      ecma_free_value (vm_stack_top_p[-2]);
      /* FALLTHRU */
    }
    case VM_CONTEXT_FINALLY_JUMP:
    {
      VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FINALLY_CONTEXT_STACK_ALLOCATION);
      vm_stack_top_p -= PARSER_FINALLY_CONTEXT_STACK_ALLOCATION;
      break;
    }
    case VM_CONTEXT_TRY:
    case VM_CONTEXT_CATCH:
    case VM_CONTEXT_BLOCK:
    case VM_CONTEXT_WITH:
    {
      VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_WITH_CONTEXT_STACK_ALLOCATION);
      vm_stack_top_p -= PARSER_WITH_CONTEXT_STACK_ALLOCATION;
      break;
    }
    case VM_CONTEXT_ITERATOR:
    case VM_CONTEXT_OBJ_INIT:
    case VM_CONTEXT_OBJ_INIT_REST:
    {
      vm_stack_top_p = vm_stack_context_abort_variable_length (frame_ctx_p, vm_stack_top_p, 0);
      break;
    }
    case VM_CONTEXT_FOR_OF:
    case VM_CONTEXT_FOR_AWAIT_OF:
    {
      ecma_free_value (vm_stack_top_p[-2]);
      ecma_free_value (vm_stack_top_p[-3]);
      ecma_free_value (vm_stack_top_p[-4]);

      VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION);
      vm_stack_top_p -= PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION;
      break;
    }
    default:
    {
      JERRY_ASSERT (VM_GET_CONTEXT_TYPE (vm_stack_top_p[-1]) == VM_CONTEXT_FOR_IN);

      ecma_collection_t *collection_p;
      collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, vm_stack_top_p[-2]);

      ecma_value_t *buffer_p = collection_p->buffer_p;

      for (uint32_t index = vm_stack_top_p[-3]; index < collection_p->item_count; index++)
      {
        ecma_free_value (buffer_p[index]);
      }

      ecma_collection_destroy (collection_p);

      ecma_free_value (vm_stack_top_p[-4]);

      VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION);
      vm_stack_top_p -= PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION;
      break;
    }
  }

  return vm_stack_top_p;
} /* vm_stack_context_abort */

/**
 * Decode branch offset.
 *
 * @return branch offset
 */
static uint32_t
vm_decode_branch_offset (const uint8_t *branch_offset_p, /**< start offset of byte code */
                         uint32_t length) /**< length of the branch */
{
  uint32_t branch_offset = *branch_offset_p;

  JERRY_ASSERT (length >= 1 && length <= 3);

  switch (length)
  {
    case 3:
    {
      branch_offset <<= 8;
      branch_offset |= *(++branch_offset_p);
      /* FALLTHRU */
    }
    case 2:
    {
      branch_offset <<= 8;
      branch_offset |= *(++branch_offset_p);
      break;
    }
  }

  return branch_offset;
} /* vm_decode_branch_offset */

/**
 * Byte code which resumes an executable object with throw
 */
static const uint8_t vm_stack_resume_executable_object_with_context_end[1] = { CBC_CONTEXT_END };

/**
 * Find a finally up to the end position.
 *
 * @return value specified in vm_stack_found_type
 */
vm_stack_found_type
vm_stack_find_finally (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                       ecma_value_t *stack_top_p, /**< current stack top */
                       vm_stack_context_type_t finally_type, /**< searching this finally */
                       uint32_t search_limit) /**< search up-to this byte code */
{
  JERRY_ASSERT (finally_type <= VM_CONTEXT_FINALLY_RETURN);

  if (finally_type != VM_CONTEXT_FINALLY_JUMP)
  {
    search_limit = 0xffffffffu;
  }

  while (frame_ctx_p->context_depth > 0)
  {
    vm_stack_context_type_t context_type = VM_GET_CONTEXT_TYPE (stack_top_p[-1]);
    uint32_t context_end = VM_GET_CONTEXT_END (stack_top_p[-1]);
    JERRY_ASSERT (!VM_CONTEXT_IS_VARIABLE_LENGTH (context_type) || finally_type != VM_CONTEXT_FINALLY_JUMP);

    if (!VM_CONTEXT_IS_VARIABLE_LENGTH (context_type) && search_limit < context_end)
    {
      frame_ctx_p->stack_top_p = stack_top_p;
      return VM_CONTEXT_FOUND_EXPECTED;
    }

    if (context_type == VM_CONTEXT_TRY || context_type == VM_CONTEXT_CATCH)
    {
      const uint8_t *byte_code_p;
      uint32_t branch_offset_length;
      uint32_t branch_offset;

      if (search_limit == context_end)
      {
        frame_ctx_p->stack_top_p = stack_top_p;
        return VM_CONTEXT_FOUND_EXPECTED;
      }

      if (stack_top_p[-1] & VM_CONTEXT_HAS_LEX_ENV)
      {
        ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;
        JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
        frame_ctx_p->lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
        ecma_deref_object (lex_env_p);
      }

      byte_code_p = frame_ctx_p->byte_code_start_p + context_end;

      if (context_type == VM_CONTEXT_TRY)
      {
        JERRY_ASSERT (byte_code_p[0] == CBC_EXT_OPCODE);

        if (byte_code_p[1] >= CBC_EXT_CATCH && byte_code_p[1] <= CBC_EXT_CATCH_3)
        {
          branch_offset_length = CBC_BRANCH_OFFSET_LENGTH (byte_code_p[1]);
          branch_offset = vm_decode_branch_offset (byte_code_p + 2, branch_offset_length);

          if (finally_type == VM_CONTEXT_FINALLY_THROW)
          {
            branch_offset += (uint32_t) (byte_code_p - frame_ctx_p->byte_code_start_p);

            stack_top_p[-1] = VM_CREATE_CONTEXT (VM_CONTEXT_CATCH, branch_offset);

            byte_code_p += 2 + branch_offset_length;
            frame_ctx_p->byte_code_p = byte_code_p;
            frame_ctx_p->stack_top_p = stack_top_p;
            return VM_CONTEXT_FOUND_FINALLY;
          }

          byte_code_p += branch_offset;

          if (*byte_code_p == CBC_CONTEXT_END)
          {
            VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_TRY_CONTEXT_STACK_ALLOCATION);
            stack_top_p -= PARSER_TRY_CONTEXT_STACK_ALLOCATION;
            continue;
          }
        }
      }
      else
      {
        JERRY_ASSERT (context_type == VM_CONTEXT_CATCH);

        if (byte_code_p[0] == CBC_CONTEXT_END)
        {
          VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_TRY_CONTEXT_STACK_ALLOCATION);
          stack_top_p -= PARSER_TRY_CONTEXT_STACK_ALLOCATION;
          continue;
        }
      }

      JERRY_ASSERT (byte_code_p[0] == CBC_EXT_OPCODE);

      VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FINALLY_CONTEXT_EXTRA_STACK_ALLOCATION);
      stack_top_p += PARSER_FINALLY_CONTEXT_EXTRA_STACK_ALLOCATION;

      if (JERRY_UNLIKELY (byte_code_p[1] == CBC_EXT_ASYNC_EXIT))
      {
        branch_offset = (uint32_t) (byte_code_p - frame_ctx_p->byte_code_start_p);
        stack_top_p[-1] = VM_CREATE_CONTEXT ((uint32_t) finally_type, branch_offset);

        frame_ctx_p->byte_code_p = byte_code_p;
        frame_ctx_p->stack_top_p = stack_top_p;
        return VM_CONTEXT_FOUND_FINALLY;
      }

      JERRY_ASSERT (byte_code_p[1] >= CBC_EXT_FINALLY && byte_code_p[1] <= CBC_EXT_FINALLY_3);

      branch_offset_length = CBC_BRANCH_OFFSET_LENGTH (byte_code_p[1]);
      branch_offset = vm_decode_branch_offset (byte_code_p + 2, branch_offset_length);

      branch_offset += (uint32_t) (byte_code_p - frame_ctx_p->byte_code_start_p);

      stack_top_p[-1] = VM_CREATE_CONTEXT ((uint32_t) finally_type, branch_offset);

      byte_code_p += 2 + branch_offset_length;
      frame_ctx_p->byte_code_p = byte_code_p;
      frame_ctx_p->stack_top_p = stack_top_p;
      return VM_CONTEXT_FOUND_FINALLY;
    }
    else if (stack_top_p[-1] & VM_CONTEXT_CLOSE_ITERATOR)
    {
      JERRY_ASSERT (context_type == VM_CONTEXT_FOR_OF || context_type == VM_CONTEXT_FOR_AWAIT_OF
                    || context_type == VM_CONTEXT_ITERATOR);
      JERRY_ASSERT (finally_type == VM_CONTEXT_FINALLY_THROW || !jcontext_has_pending_exception ());

      ecma_value_t exception = ECMA_VALUE_UNDEFINED;
      if (finally_type == VM_CONTEXT_FINALLY_THROW)
      {
        exception = jcontext_take_exception ();
      }

      ecma_value_t result;

      if (context_type == VM_CONTEXT_ITERATOR)
      {
        result = ecma_op_iterator_close (stack_top_p[-2]);
      }
      else
      {
        ecma_value_t iterator = stack_top_p[-3];
        result = ecma_op_get_method_by_magic_id (iterator, LIT_MAGIC_STRING_RETURN);

        if (!ECMA_IS_VALUE_ERROR (result) && !ecma_is_value_undefined (result))
        {
          ecma_object_t *return_obj_p = ecma_get_object_from_value (result);
          result = ecma_op_function_validated_call (result, iterator, NULL, 0);
          ecma_deref_object (return_obj_p);

          if (context_type == VM_CONTEXT_FOR_AWAIT_OF && !ECMA_IS_VALUE_ERROR (result))
          {
            ecma_extended_object_t *async_generator_object_p = VM_GET_EXECUTABLE_OBJECT (frame_ctx_p);

            result = ecma_promise_async_await (async_generator_object_p, result);

            if (!ECMA_IS_VALUE_ERROR (result))
            {
              uint16_t extra_flags =
                (ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD | (ECMA_AWAIT_FOR_CLOSE << ECMA_AWAIT_STATE_SHIFT));
              async_generator_object_p->u.cls.u2.executable_obj_flags |= extra_flags;

              stack_top_p = vm_stack_context_abort (frame_ctx_p, stack_top_p);

              VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FINALLY_CONTEXT_STACK_ALLOCATION);
              stack_top_p += PARSER_FINALLY_CONTEXT_STACK_ALLOCATION;

              stack_top_p[-1] = VM_CREATE_CONTEXT ((uint32_t) finally_type, context_end);
              if (finally_type == VM_CONTEXT_FINALLY_THROW)
              {
                stack_top_p[-2] = exception;
              }

              frame_ctx_p->call_operation = VM_EXEC_RETURN;
              frame_ctx_p->byte_code_p = vm_stack_resume_executable_object_with_context_end;
              frame_ctx_p->stack_top_p = stack_top_p;
              return VM_CONTEXT_FOUND_AWAIT;
            }
          }

          if (!ECMA_IS_VALUE_ERROR (result))
          {
            bool is_object = ecma_is_value_object (result);

            ecma_free_value (result);
            result = ECMA_VALUE_UNDEFINED;

            if (!is_object)
            {
              result = ecma_raise_type_error (ECMA_ERR_ITERATOR_RETURN_RESULT_IS_NOT_OBJECT);
            }
          }
        }
      }

      JERRY_ASSERT (ECMA_IS_VALUE_ERROR (result) || result == ECMA_VALUE_UNDEFINED);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        if (finally_type != VM_CONTEXT_FINALLY_THROW)
        {
          frame_ctx_p->stack_top_p = vm_stack_context_abort (frame_ctx_p, stack_top_p);
          return VM_CONTEXT_FOUND_ERROR;
        }

        ecma_free_value (jcontext_take_exception ());
        jcontext_raise_exception (exception);
      }
      else if (finally_type == VM_CONTEXT_FINALLY_THROW)
      {
        jcontext_raise_exception (exception);
      }
    }

    stack_top_p = vm_stack_context_abort (frame_ctx_p, stack_top_p);
  }

  frame_ctx_p->stack_top_p = stack_top_p;
  return VM_CONTEXT_FOUND_EXPECTED;
} /* vm_stack_find_finally */

/**
 * Get the offsets of ecma values corresponding to the passed context.
 *
 * @return array of offsets, last item represents the size of the context item
 */
uint32_t
vm_get_context_value_offsets (ecma_value_t *context_item_p) /**< any item of a context */
{
  switch (VM_GET_CONTEXT_TYPE (context_item_p[-1]))
  {
    case VM_CONTEXT_FINALLY_THROW:
    case VM_CONTEXT_FINALLY_RETURN:
    {
      return (PARSER_FINALLY_CONTEXT_STACK_ALLOCATION << VM_CONTEXT_OFFSET_SHIFT) | 2;
    }
    case VM_CONTEXT_FINALLY_JUMP:
    {
      return PARSER_FINALLY_CONTEXT_STACK_ALLOCATION;
    }
    case VM_CONTEXT_TRY:
    case VM_CONTEXT_CATCH:
    case VM_CONTEXT_BLOCK:
    case VM_CONTEXT_WITH:
    {
      return PARSER_WITH_CONTEXT_STACK_ALLOCATION;
    }
    case VM_CONTEXT_FOR_IN:
    {
      return (PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION << VM_CONTEXT_OFFSET_SHIFT) | 4;
    }
    default:
    {
      JERRY_ASSERT (VM_GET_CONTEXT_TYPE (context_item_p[-1]) == VM_CONTEXT_FOR_OF
                    || VM_GET_CONTEXT_TYPE (context_item_p[-1]) == VM_CONTEXT_FOR_AWAIT_OF);

      return ((PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION << (VM_CONTEXT_OFFSET_SHIFT * 3))
              | (4 << (VM_CONTEXT_OFFSET_SHIFT * 2)) | (3 << VM_CONTEXT_OFFSET_SHIFT) | 2);
    }
  }
} /* vm_get_context_value_offsets */

/**
 * Ref / deref lexical environments in the chain using the current context.
 */
void
vm_ref_lex_env_chain (ecma_object_t *lex_env_p, /**< top of lexical environment */
                      uint16_t context_depth, /**< depth of function context */
                      ecma_value_t *context_end_p, /**< end of function context */
                      bool do_ref) /**< ref or deref lexical environments */
{
  ecma_value_t *context_top_p = context_end_p + context_depth;
  JERRY_ASSERT (context_top_p > context_end_p);

  do
  {
    if (context_top_p[-1] & VM_CONTEXT_HAS_LEX_ENV)
    {
      JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
      ecma_object_t *next_lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);

      if (do_ref)
      {
        ecma_ref_object (lex_env_p);
      }
      else
      {
        ecma_deref_object (lex_env_p);
      }

      lex_env_p = next_lex_env_p;
    }

    if (VM_CONTEXT_IS_VARIABLE_LENGTH (VM_GET_CONTEXT_TYPE (context_top_p[-1])))
    {
      ecma_value_t *last_item_p = context_top_p - VM_GET_CONTEXT_END (context_top_p[-1]);
      JERRY_ASSERT (last_item_p >= context_end_p);
      context_top_p--;

      do
      {
        if (do_ref)
        {
          ecma_ref_if_object (*(--context_top_p));
        }
        else
        {
          ecma_deref_if_object (*(--context_top_p));
        }
      } while (context_top_p > last_item_p);

      continue;
    }

    uint32_t offsets = vm_get_context_value_offsets (context_top_p);

    while (VM_CONTEXT_HAS_NEXT_OFFSET (offsets))
    {
      int32_t offset = VM_CONTEXT_GET_NEXT_OFFSET (offsets);

      if (do_ref)
      {
        ecma_ref_if_object (context_top_p[offset]);
      }
      else
      {
        ecma_deref_if_object (context_top_p[offset]);
      }

      offsets >>= VM_CONTEXT_OFFSET_SHIFT;
    }

    JERRY_ASSERT (context_top_p >= context_end_p + offsets);
    context_top_p -= offsets;
  } while (context_top_p > context_end_p);
} /* vm_ref_lex_env_chain */

/**
 * @}
 * @}
 */
