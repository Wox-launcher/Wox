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

#ifndef RE_COMPILER_H
#define RE_COMPILER_H

#include "ecma-globals.h"

#include "re-bytecode.h"

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

re_compiled_code_t *re_compile_bytecode (ecma_string_t *pattern_str_p, uint16_t flags);

void re_cache_gc (void);

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
#endif /* !RE_COMPILER_H */
