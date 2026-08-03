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

#ifndef ECMA_ERRORS_H
#define ECMA_ERRORS_H

#include "lit-globals.h"

typedef enum
{
  ECMA_ERR_EMPTY,
/** @cond doxygen_suppress */
#if JERRY_ERROR_MESSAGES
#define ECMA_ERROR_DEF(id, ascii_zt_string) id,
#else /* !JERRY_ERROR_MESSAGES */
#define ECMA_ERROR_DEF(id, ascii_zt_string) id = ECMA_ERR_EMPTY,
#endif /* JERRY_ERROR_MESSAGES */
#include "ecma-error-messages.inc.h"
#undef ECMA_ERROR_DEF
  /** @endcond */
  ECMA_IS_VALID_CONSTRUCTOR /* used as return value when checking constructor */
} ecma_error_msg_t;

const char* ecma_get_error_msg (ecma_error_msg_t id);
lit_utf8_size_t ecma_get_error_size (ecma_error_msg_t id);

#endif /* !ECMA_ERRORS_H */
