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

#ifndef PARSER_ERRORS_H
#define PARSER_ERRORS_H

#include "lit-globals.h"

typedef enum
{
  PARSER_ERR_EMPTY,
/** @cond doxygen_suppress */
#if JERRY_ERROR_MESSAGES
#define PARSER_ERROR_DEF(id, ascii_zt_string) id,
#else /* !JERRY_ERROR_MESSAGES */
#define PARSER_ERROR_DEF(id, ascii_zt_string) id = PARSER_ERR_EMPTY,
#endif /* JERRY_ERROR_MESSAGES */
#include "parser-error-messages.inc.h"
#undef PARSER_ERROR_DEF
  /** @endcond */
  PARSER_ERR_OUT_OF_MEMORY,
  PARSER_ERR_INVALID_REGEXP,
#if (JERRY_STACK_LIMIT != 0)
  PARSER_ERR_STACK_OVERFLOW,
#endif /* JERRY_STACK_LIMIT != 0 */
  PARSER_ERR_NO_ERROR,
} parser_error_msg_t;

const lit_utf8_byte_t* parser_get_error_utf8 (uint32_t id);
lit_utf8_size_t parser_get_error_size (uint32_t id);

#endif /* !PARSER_ERRORS_H */
