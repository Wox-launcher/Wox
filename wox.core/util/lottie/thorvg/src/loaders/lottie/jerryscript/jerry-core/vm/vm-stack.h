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

#ifndef VM_STACK_H
#define VM_STACK_H

#include "ecma-globals.h"

#include "vm-defines.h"

/** \addtogroup vm Virtual machine
 * @{
 *
 * \addtogroup stack VM stack
 * @{
 */

/**
 * Create context on the vm stack.
 */
#define VM_CREATE_CONTEXT(type, end_offset) ((ecma_value_t) ((type) | ((end_offset) << 7)))

/**
 * Create context on the vm stack with environment.
 */
#define VM_CREATE_CONTEXT_WITH_ENV(type, end_offset) (VM_CREATE_CONTEXT ((type), (end_offset)) | VM_CONTEXT_HAS_LEX_ENV)

/**
 * Get type of a vm context.
 */
#define VM_GET_CONTEXT_TYPE(value) ((vm_stack_context_type_t) ((value) &0x1f))

/**
 * Get the end position of a vm context.
 */
#define VM_GET_CONTEXT_END(value) ((value) >> 7)

/**
 * This flag is set if the context has a lexical environment.
 */
#define VM_CONTEXT_HAS_LEX_ENV 0x20

/**
 * This flag is set if the iterator close operation should be invoked during a for-of context break.
 */
#define VM_CONTEXT_CLOSE_ITERATOR 0x40

/**
 * Context types for the vm stack.
 */
typedef enum
{
  /* Update VM_CONTEXT_IS_FINALLY macro if the following three values are changed. */
  VM_CONTEXT_FINALLY_JUMP, /**< finally context with a jump */
  VM_CONTEXT_FINALLY_THROW, /**< finally context with a throw */
  VM_CONTEXT_FINALLY_RETURN, /**< finally context with a return */
  VM_CONTEXT_TRY, /**< try context */
  VM_CONTEXT_CATCH, /**< catch context */
  VM_CONTEXT_BLOCK, /**< block context */
  VM_CONTEXT_WITH, /**< with context */
  VM_CONTEXT_FOR_IN, /**< for-in context */
  VM_CONTEXT_FOR_OF, /**< for-of context */
  VM_CONTEXT_FOR_AWAIT_OF, /**< for-await-of context */

  /* contexts with variable length */
  VM_CONTEXT_ITERATOR, /**< iterator context */
  VM_CONTEXT_OBJ_INIT, /**< object-initializer context */
  VM_CONTEXT_OBJ_INIT_REST, /**< object-initializer-rest context */
} vm_stack_context_type_t;

/**
 * Return types for vm_stack_find_finally.
 */
typedef enum
{
  VM_CONTEXT_FOUND_FINALLY, /**< found finally */
  VM_CONTEXT_FOUND_ERROR, /**< found an error */
  VM_CONTEXT_FOUND_AWAIT, /**< found an await operation */
  VM_CONTEXT_FOUND_EXPECTED, /**< found the type specified in finally_type */
} vm_stack_found_type;

/**
 * Checks whether the context has variable context size
 *
 * Layout:
 * - [context descriptor]
 * - [JS values belong to the context]
 * - [previous JS values stored by the VM stack]
 */
#define VM_CONTEXT_IS_VARIABLE_LENGTH(context_type) ((context_type) >= VM_CONTEXT_ITERATOR)

/**
 * Checks whether the context type is a finally type.
 */
#define VM_CONTEXT_IS_FINALLY(context_type) ((context_type) <= VM_CONTEXT_FINALLY_RETURN)

/**
 * Shift needs to be applied to get the next item of the offset array.
 */
#define VM_CONTEXT_OFFSET_SHIFT 4

/**
 * Checks whether an offset is available.
 */
#define VM_CONTEXT_HAS_NEXT_OFFSET(offsets) ((offsets) >= (1 << VM_CONTEXT_OFFSET_SHIFT))

/**
 * Get the next offset from the offset array.
 */
#define VM_CONTEXT_GET_NEXT_OFFSET(offsets) (-((int32_t) ((offsets) & ((1 << VM_CONTEXT_OFFSET_SHIFT) - 1))))

ecma_value_t *vm_stack_context_abort_variable_length (vm_frame_ctx_t *frame_ctx_p,
                                                      ecma_value_t *vm_stack_top_p,
                                                      uint32_t context_stack_allocation);
ecma_value_t *vm_stack_context_abort (vm_frame_ctx_t *frame_ctx_p, ecma_value_t *vm_stack_top_p);
vm_stack_found_type vm_stack_find_finally (vm_frame_ctx_t *frame_ctx_p,
                                           ecma_value_t *stack_top_p,
                                           vm_stack_context_type_t finally_type,
                                           uint32_t search_limit);
uint32_t vm_get_context_value_offsets (ecma_value_t *context_item_p);
void vm_ref_lex_env_chain (ecma_object_t *lex_env_p, uint16_t context_depth, ecma_value_t *context_end_p, bool do_ref);

/**
 * @}
 * @}
 */

#endif /* !VM_STACK_H */
