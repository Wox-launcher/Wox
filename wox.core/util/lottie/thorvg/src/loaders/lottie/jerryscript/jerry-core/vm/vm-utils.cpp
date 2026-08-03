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

#include "ecma-array-object.h"
#include "ecma-helpers.h"
#include "jcontext.h"
#include "lit-char-helpers.h"
#include "vm.h"

/**
 * Check whether currently executed code is strict mode code
 *
 * @return true - current code is executed in strict mode,
 *         false - otherwise
 */
bool
vm_is_strict_mode (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (JERRY_CONTEXT (vm_top_context_p) != NULL);

  return JERRY_CONTEXT (vm_top_context_p)->status_flags & VM_FRAME_CTX_IS_STRICT;
} /* vm_is_strict_mode */

/**
 * Check whether currently performed call (on top of call-stack) is performed in form,
 * meeting conditions of 'Direct Call to Eval' (see also: ECMA-262 v5, 15.1.2.1.1)
 *
 * Warning:
 *         the function should only be called from implementation
 *         of built-in 'eval' routine of Global object
 *
 * @return true - currently performed call is performed through 'eval' identifier,
 *                without 'this' argument,
 *         false - otherwise
 */
bool
vm_is_direct_eval_form_call (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  return (JERRY_CONTEXT (status_flags) & ECMA_STATUS_DIRECT_EVAL) != 0;
} /* vm_is_direct_eval_form_call */

/**
 * Get backtrace. The backtrace is an array of strings where
 * each string contains the position of the corresponding frame.
 * The array length is zero if the backtrace is not available.
 *
 * @return array ecma value
 */
ecma_value_t
vm_get_backtrace (uint32_t max_depth) /**< maximum backtrace depth, 0 = unlimited */
{
  JERRY_UNUSED (max_depth);

  return ecma_make_object_value (ecma_op_new_array_object (0));
} /* vm_get_backtrace */
