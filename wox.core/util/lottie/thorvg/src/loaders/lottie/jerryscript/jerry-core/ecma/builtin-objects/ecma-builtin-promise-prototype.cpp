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

#include "ecma-globals.h"
#include "ecma-promise-object.h"

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
  ECMA_PROMISE_PROTOTYPE_ROUTINE_START = 0,
  ECMA_PROMISE_PROTOTYPE_ROUTINE_THEN,
  ECMA_PROMISE_PROTOTYPE_ROUTINE_CATCH,
  ECMA_PROMISE_PROTOTYPE_ROUTINE_FINALLY
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-promise-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  promise_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup promiseprototype ECMA Promise.prototype object built-in
 * @{
 */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_promise_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                                 ecma_value_t this_arg, /**< 'this' argument value */
                                                 const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                         *   passed to routine */
                                                 uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (arguments_number);

  switch (builtin_routine_id)
  {
    case ECMA_PROMISE_PROTOTYPE_ROUTINE_THEN:
    {
      return ecma_promise_then (this_arg, arguments_list_p[0], arguments_list_p[1]);
    }
    case ECMA_PROMISE_PROTOTYPE_ROUTINE_CATCH:
    {
      ecma_value_t args[] = { ECMA_VALUE_UNDEFINED, arguments_list_p[0] };
      return ecma_op_invoke_by_magic_id (this_arg, LIT_MAGIC_STRING_THEN, args, 2);
    }
    case ECMA_PROMISE_PROTOTYPE_ROUTINE_FINALLY:
    {
      return ecma_promise_finally (this_arg, arguments_list_p[0]);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_promise_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
