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

#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-iterator-object.h"

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
  ECMA_BUILTIN_ASYNC_ITERATOR_PROTOTYPE_ROUTINE_START = 0,
  ECMA_BUILTIN_ASYNC_ITERATOR_PROTOTYPE_OBJECT_ASYNC_ITERATOR,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-async-iterator-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  async_iterator_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup %asynciteratorprototype% ECMA %AsyncIteratorPrototype% object built-in
 * @{
 */

/**
 * The %AsyncIteratorPrototype% object's '@@asyncIterator' routine
 *
 * See also:
 *          ECMA-262 v10, 25.1.3.1
 *
 * Note:
 *     Returned value must be freed with ecma_free_value.
 *
 * @return the given this value
 */
static ecma_value_t
ecma_builtin_async_iterator_prototype_object_async_iterator (ecma_value_t this_val) /**< this argument */
{
  /* 1. */
  return ecma_copy_value (this_val);
} /* ecma_builtin_async_iterator_prototype_object_async_iterator */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_async_iterator_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide
                                                                                     *   routine identifier */
                                                        ecma_value_t this_arg, /**< 'this' argument value */
                                                        const ecma_value_t arguments_list_p[], /**<
                                                                                                * list of arguments
                                                                                                * passed to routine */
                                                        uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (arguments_list_p, arguments_number);

  switch (builtin_routine_id)
  {
    case ECMA_BUILTIN_ASYNC_ITERATOR_PROTOTYPE_OBJECT_ASYNC_ITERATOR:
    {
      return ecma_builtin_async_iterator_prototype_object_async_iterator (this_arg);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_async_iterator_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
