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

#ifndef ECMA_ATOMICS_OBJECT_H
#define ECMA_ATOMICS_OBJECT_H

#include "ecma-globals.h"

#if JERRY_BUILTIN_ATOMICS

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaatomicsobject ECMA builtin Atomics helper functions
 * @{
 */

/**
 * Atomics flags.
 */
typedef enum
{
  ECMA_ATOMICS_ADD, /**< Atomics add operation */
  ECMA_ATOMICS_SUBTRACT, /**< Atomics subtract operation */
  ECMA_ATOMICS_AND, /**< Atomics and operation */
  ECMA_ATOMICS_OR, /**< Atomics or operation */
  ECMA_ATOMICS_XOR, /**< Atomics xor operation */
  ECMA_ATOMICS_EXCHANGE, /**< Atomics exchange operation */
  ECMA_ATOMICS_COMPARE_EXCHANGE /**< Atomics compare exchange operation */
} ecma_atomics_op_t;

ecma_value_t ecma_validate_shared_integer_typedarray (ecma_value_t typedarray, bool waitable);
ecma_value_t ecma_validate_atomic_access (ecma_value_t typedarray, ecma_value_t request_index);
ecma_value_t
ecma_atomic_read_modify_write (ecma_value_t typedarray, ecma_value_t index, ecma_value_t value, ecma_atomics_op_t op);
ecma_value_t ecma_atomic_load (ecma_value_t typedarray, ecma_value_t index);
/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_ATOMICS */

#endif /* ECMA_ATOMICS_OBJECT_H */
