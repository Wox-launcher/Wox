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

#ifndef RE_COMPILER_CONTEXT_H
#define RE_COMPILER_CONTEXT_H

#include "re-token.h"

#if JERRY_BUILTIN_REGEXP

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup regexparser Regular expression
 * @{
 *
 * \addtogroup regexparser_compiler Compiler
 * @{
 */

/**
 * RegExp compiler context
 */
typedef struct
{
  const lit_utf8_byte_t *input_start_p; /**< start of input pattern */
  const lit_utf8_byte_t *input_curr_p; /**< current position in input pattern */
  const lit_utf8_byte_t *input_end_p; /**< end of input pattern */

  uint8_t *bytecode_start_p; /**< start of bytecode block */
  size_t bytecode_size; /**< size of bytecode */

  uint32_t captures_count; /**< number of capture groups */
  uint32_t non_captures_count; /**< number of non-capture groups */

  int groups_count; /**< number of groups */
  uint16_t flags; /**< RegExp flags */
  re_token_t token; /**< current token */
} re_compiler_ctx_t;

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
#endif /* !RE_COMPILER_CONTEXT_H */
