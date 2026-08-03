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

#include "ecma-eval.h"

#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-lex-env.h"

#include "jcontext.h"
#include "js-parser.h"
#include "vm.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup eval eval
 * @{
 */

/**
 * Perform 'eval' with code stored in ecma-string
 *
 * See also:
 *          ecma_op_eval_chars_buffer
 *          ECMA-262 v5, 15.1.2.1 (steps 2 to 8)
 *
 * @return ecma value
 */
ecma_value_t
ecma_op_eval (ecma_value_t source_code, /**< source code */
              uint32_t parse_opts) /**< ecma_parse_opts_t option bits */
{
  JERRY_ASSERT (ecma_is_value_string (source_code));

  if (ecma_is_value_magic_string (source_code, LIT_MAGIC_STRING__EMPTY))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  return ecma_op_eval_chars_buffer ((void *) &source_code, parse_opts | ECMA_PARSE_HAS_SOURCE_VALUE);
} /* ecma_op_eval */

/**
 * Perform 'eval' with code stored in continuous character buffer
 *
 * See also:
 *          ecma_op_eval
 *          ECMA-262 v5, 15.1.2.1 (steps 2 to 8)
 *
 * @return ecma value
 */
ecma_value_t
ecma_op_eval_chars_buffer (void *source_p, /**< source code */
                           uint32_t parse_opts) /**< ecma_parse_opts_t option bits */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if JERRY_PARSER
  JERRY_ASSERT (source_p != NULL);

  uint32_t is_strict_call = ECMA_PARSE_STRICT_MODE | ECMA_PARSE_DIRECT_EVAL;

  if ((parse_opts & is_strict_call) != is_strict_call)
  {
    parse_opts &= (uint32_t) ~ECMA_PARSE_STRICT_MODE;
  }

  parse_opts |= ECMA_PARSE_EVAL;

  ECMA_CLEAR_LOCAL_PARSE_OPTS ();

  ecma_compiled_code_t *bytecode_p = parser_parse_script (source_p, parse_opts, NULL);

  if (JERRY_UNLIKELY (bytecode_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  return vm_run_eval (bytecode_p, parse_opts);
#endif /* JERRY_PARSER */
} /* ecma_op_eval_chars_buffer */

/**
 * @}
 * @}
 */
