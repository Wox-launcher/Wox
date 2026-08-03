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
#include "ecma-conversion.h"
#include "ecma-eval.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-lex-env.h"

#include "js-parser.h"
#include "lit-magic-strings.h"

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-function.inc.h"
#define BUILTIN_UNDERSCORED_ID  function
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup function ECMA Function object built-in
 * @{
 */

/**
 * Handle calling [[Call]] of built-in Function object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_function_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                     uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_builtin_function_dispatch_construct (arguments_list_p, arguments_list_len);
} /* ecma_builtin_function_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Function object
 *
 * See also:
 *          ECMA-262 v5, 15.3.
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_function_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                          uint32_t arguments_list_len) /**< number of arguments */
{
  return ecma_op_create_dynamic_function (arguments_list_p, arguments_list_len, ECMA_PARSE_NO_OPTS);
} /* ecma_builtin_function_dispatch_construct */

/**
 * @}
 * @}
 * @}
 */
