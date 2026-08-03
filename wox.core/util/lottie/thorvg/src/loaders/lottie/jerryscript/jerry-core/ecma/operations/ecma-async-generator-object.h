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

#ifndef ECMA_ASYNC_GENERATOR_OBJECT_H
#define ECMA_ASYNC_GENERATOR_OBJECT_H

#include "ecma-globals.h"

#include "vm-defines.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaasyncgeneratorobject ECMA AsyncGenerator object related routines
 * @{
 */

/**
 * AsyncGenerator command types.
 */
typedef enum
{
  ECMA_ASYNC_GENERATOR_DO_NEXT, /**< async generator next operation */
  ECMA_ASYNC_GENERATOR_DO_THROW, /**< async generator throw operation */
  ECMA_ASYNC_GENERATOR_DO_RETURN, /**< async generator return operation */
} ecma_async_generator_operation_type_t;

/**
 * Get the state of an async yield iterator.
 */
#define ECMA_AWAIT_GET_STATE(async_generator_object_p) \
  ((async_generator_object_p)->extended_object.u.cls.u2.executable_obj_flags >> ECMA_AWAIT_STATE_SHIFT)

/**
 * Set the state of an async yield iterator.
 */
#define ECMA_AWAIT_SET_STATE(async_generator_object_p, to)                                           \
  do                                                                                                 \
  {                                                                                                  \
    uint16_t extra_info = (async_generator_object_p)->extended_object.u.cls.u2.executable_obj_flags; \
    extra_info &= ((1 << ECMA_AWAIT_STATE_SHIFT) - 1);                                               \
    extra_info |= (ECMA_AWAIT_##to) << ECMA_AWAIT_STATE_SHIFT;                                       \
    (async_generator_object_p)->extended_object.u.cls.u2.executable_obj_flags = extra_info;          \
  } while (false)

/**
 * Mask for clearing all ASYNC_AWAIT status bits
 */
#define ECMA_AWAIT_CLEAR_MASK (((1 << ECMA_AWAIT_STATE_SHIFT) - 1) - ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD)

/**
 * Helper macro for ECMA_AWAIT_CHANGE_STATE.
 */
#define ECMA_AWAIT_CS_HELPER(from, to) (((ECMA_AWAIT_##from) ^ (ECMA_AWAIT_##to)) << ECMA_AWAIT_STATE_SHIFT)

/**
 * Change the state of an async yield iterator.
 */
#define ECMA_AWAIT_CHANGE_STATE(async_generator_object_p, from, to) \
  ((async_generator_object_p)->extended_object.u.cls.u2.executable_obj_flags ^= ECMA_AWAIT_CS_HELPER (from, to))

ecma_value_t ecma_async_generator_enqueue (vm_executable_object_t *async_generator_object_p,
                                           ecma_async_generator_operation_type_t operation,
                                           ecma_value_t value);

ecma_value_t ecma_async_generator_run (vm_executable_object_t *async_generator_object_p);
void ecma_async_generator_finalize (vm_executable_object_t *async_generator_object_p, ecma_value_t value);

ecma_value_t ecma_await_continue (vm_executable_object_t *async_generator_object_p, ecma_value_t value);

/**
 * @}
 * @}
 */

#endif /* !ECMA_ASYNC_GENERATOR_OBJECT_H */
