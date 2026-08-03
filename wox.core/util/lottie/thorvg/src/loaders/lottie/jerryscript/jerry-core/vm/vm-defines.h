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
#ifndef VM_DEFINES_H
#define VM_DEFINES_H

#include "ecma-globals.h"

#include "byte-code.h"

/** \addtogroup vm Virtual machine
 * @{
 *
 * \addtogroup vm_executor Executor
 * @{
 */

/**
 * Helper for += on uint16_t values.
 */
#define VM_PLUS_EQUAL_U16(base, value) (base) = (uint16_t) ((base) + (value))

/**
 * Helper for -= on uint16_t values.
 */
#define VM_MINUS_EQUAL_U16(base, value) (base) = (uint16_t) ((base) - (value))

/**
 * Flag bits of vm_frame_ctx_shared_t
 */
typedef enum
{
  VM_FRAME_CTX_SHARED_HAS_ARG_LIST = (1 << 0), /**< has argument list */
  VM_FRAME_CTX_SHARED_DIRECT_EVAL = (1 << 1), /**< direct eval call */
  VM_FRAME_CTX_SHARED_FREE_THIS = (1 << 2), /**< free this binding */
  VM_FRAME_CTX_SHARED_FREE_LOCAL_ENV = (1 << 3), /**< free local environment */
  VM_FRAME_CTX_SHARED_NON_ARROW_FUNC = (1 << 4), /**< non-arrow function */
  VM_FRAME_CTX_SHARED_HERITAGE_PRESENT = (1 << 5), /**< class heritage present */
  VM_FRAME_CTX_SHARED_HAS_CLASS_FIELDS = (1 << 6), /**< has class fields */
  VM_FRAME_CTX_SHARED_EXECUTABLE = (1 << 7), /**< frame is an executable object constructed
                                              *   with opfunc_create_executable_object */
} vm_frame_ctx_shared_flags_t;

/**
 * Shared data between the interpreter and the caller
 */
typedef struct
{
  const ecma_compiled_code_t *bytecode_header_p; /**< currently executed byte-code data */
  ecma_object_t *function_object_p; /**< function obj */
  uint32_t status_flags; /**< combination of vm_frame_ctx_shared_flags_t bits */
} vm_frame_ctx_shared_t;

/**
 * Shared data extended with arguments
 */
typedef struct
{
  vm_frame_ctx_shared_t header; /**< shared data header */
  const ecma_value_t *arg_list_p; /**< arguments list */
  uint32_t arg_list_len; /**< arguments list length */
} vm_frame_ctx_shared_args_t;

/**
 * Shared data extended with computed class fields
 */
typedef struct
{
  vm_frame_ctx_shared_t header; /**< shared data header */
  ecma_value_t *computed_class_fields_p; /**< names of the computed class fields */
} vm_frame_ctx_shared_class_fields_t;

/**
 * Get the computed class field
 */
#define VM_GET_COMPUTED_CLASS_FIELDS(frame_ctx_p) \
  (((vm_frame_ctx_shared_class_fields_t *) ((frame_ctx_p)->shared_p))->computed_class_fields_p)

/**
 * Flag bits of vm_frame_ctx_t
 */
typedef enum
{
  VM_FRAME_CTX_DIRECT_EVAL = (1 << 1), /**< direct eval call */
  VM_FRAME_CTX_IS_STRICT = (1 << 2), /**< strict mode */
} vm_frame_ctx_flags_t;

/**
 * Context of interpreter, related to a JS stack frame
 */
typedef struct vm_frame_ctx_t
{
  vm_frame_ctx_shared_t *shared_p; /**< shared information */
  const uint8_t *byte_code_p; /**< current byte code pointer */
  const uint8_t *byte_code_start_p; /**< byte code start pointer */
  ecma_value_t *stack_top_p; /**< stack top pointer */
  ecma_value_t *literal_start_p; /**< literal list start pointer */
  ecma_object_t *lex_env_p; /**< current lexical environment */
  struct vm_frame_ctx_t *prev_context_p; /**< previous context */
  ecma_value_t this_binding; /**< this binding */
  uint16_t context_depth; /**< current context depth */
  uint8_t status_flags; /**< combination of vm_frame_ctx_flags_t bits */
  uint8_t call_operation; /**< perform a call or construct operation */
  /* Registers start immediately after the frame context. */
} vm_frame_ctx_t;

/**
 * Get register list corresponding to the frame context.
 */
#define VM_GET_REGISTERS(frame_ctx_p) ((ecma_value_t *) ((frame_ctx_p) + 1))

/**
 * Read or write a specific register.
 */
#define VM_GET_REGISTER(frame_ctx_p, i) (((ecma_value_t *) ((frame_ctx_p) + 1))[i])

/**
 * Calculate the executable object from a vm_executable_object frame context.
 */
#define VM_GET_EXECUTABLE_OBJECT(frame_ctx_p) \
  ((ecma_extended_object_t *) ((uintptr_t) (frame_ctx_p) - (uintptr_t) offsetof (vm_executable_object_t, frame_ctx)))

/**
 * Calculate the shared_part from a vm_executable_object frame context.
 */
#define VM_GET_EXECUTABLE_ITERATOR(frame_ctx_p)                                                           \
  ((ecma_value_t *) ((uintptr_t) (frame_ctx_p) - (uintptr_t) offsetof (vm_executable_object_t, frame_ctx) \
                     + (uintptr_t) offsetof (vm_executable_object_t, iterator)))

/**
 * Generator frame context.
 */
typedef struct
{
  ecma_extended_object_t extended_object; /**< extended object part */
  vm_frame_ctx_shared_t shared; /**< shared part */
  ecma_value_t iterator; /**< executable object's iterator */
  vm_frame_ctx_t frame_ctx; /**< frame context part */
} vm_executable_object_t;

/**
 * Real backtrace frame data passed to the jerry_backtrace_cb_t handler.
 */
struct jerry_frame_internal_t
{
  vm_frame_ctx_t *context_p; /**< context pointer */
  uint8_t frame_type; /**< frame type */
  jerry_frame_location_t location; /**< location information */
  ecma_value_t function; /**< function reference */
  ecma_value_t this_binding; /**< this binding passed to the function */
};

/**
 * @}
 * @}
 */

#endif /* !VM_DEFINES_H */
