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

#include "ecma-builtin-handlers.h"

#include "ecma-globals.h"
#include "ecma-iterator-object.h"
#include "ecma-promise-object.h"

static const ecma_builtin_handler_t ecma_native_handlers[] = {
/** @cond doxygen_suppress */
#define ECMA_NATIVE_HANDLER(id, handler, length) handler,
#include "ecma-builtin-handlers.inc.h"
#undef ECMA_NATIVE_HANDLER
  /** @endcond */
};

static const uint8_t ecma_native_handler_lengths[] = {
/** @cond doxygen_suppress */
#define ECMA_NATIVE_HANDLER(id, handler, length) length,
#include "ecma-builtin-handlers.inc.h"
#undef ECMA_NATIVE_HANDLER
  /** @endcond */
};

/**
 * Get the native handler of a built-in handler type.
 *
 * return Function pointer of the handler
 */
ecma_builtin_handler_t
ecma_builtin_handler_get (ecma_native_handler_id_t id) /**< handler id */
{
  JERRY_ASSERT (id != ECMA_NATIVE_HANDLER_START && id < ECMA_NATIVE_HANDLER__COUNT);
  return ecma_native_handlers[id - 1];
} /* ecma_builtin_handler_get */

/**
 * Get the initial 'length' value of a built-in handler type.
 *
 * return 'length' value of the handler
 */
uint8_t
ecma_builtin_handler_get_length (ecma_native_handler_id_t id) /**< handler id */
{
  JERRY_ASSERT (id != ECMA_NATIVE_HANDLER_START && id < ECMA_NATIVE_HANDLER__COUNT);
  return ecma_native_handler_lengths[id - 1];
} /* ecma_builtin_handler_get_length */
