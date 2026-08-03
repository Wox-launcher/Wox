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
#include "ecma-boolean-object.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"

#include "jrt.h"

#if JERRY_BUILTIN_BOOLEAN

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-boolean.inc.h"
#define BUILTIN_UNDERSCORED_ID  boolean
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup boolean ECMA Boolean object built-in
 * @{
 */

/**
 * Handle calling [[Call]] of built-in Boolean object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_boolean_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                    uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  ecma_value_t arg_value;

  if (arguments_list_len == 0)
  {
    arg_value = ECMA_VALUE_UNDEFINED;
  }
  else
  {
    arg_value = arguments_list_p[0];
  }

  return ecma_make_boolean_value (ecma_op_to_boolean (arg_value));
} /* ecma_builtin_boolean_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Boolean object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_boolean_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                         uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  if (arguments_list_len == 0)
  {
    return ecma_op_create_boolean_object (ECMA_VALUE_FALSE);
  }
  else
  {
    return ecma_op_create_boolean_object (arguments_list_p[0]);
  }
} /* ecma_builtin_boolean_dispatch_construct */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_BOOLEAN */
