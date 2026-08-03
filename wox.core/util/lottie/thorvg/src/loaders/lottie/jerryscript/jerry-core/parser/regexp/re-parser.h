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

#ifndef RE_PARSER_H
#define RE_PARSER_H

#include "re-compiler-context.h"

#if JERRY_BUILTIN_REGEXP

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup regexparser Regular expression
 * @{
 *
 * \addtogroup regexparser_parser Parser
 * @{
 */

/**
 * @}
 *
 * \addtogroup regexparser_parser Parser
 * @{
 */

/**
 * Value used for infinite quantifier.
 */
#define RE_INFINITY UINT32_MAX

/**
 * Maximum decimal value of an octal escape
 */
#define RE_MAX_OCTAL_VALUE 0xff

ecma_value_t re_parse_alternative (re_compiler_ctx_t *re_ctx_p, bool expect_eof);

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
#endif /* !RE_PARSER_H */
