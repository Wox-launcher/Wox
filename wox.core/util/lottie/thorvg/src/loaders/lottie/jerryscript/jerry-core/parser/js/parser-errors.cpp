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

#include "parser-errors.h"

#if JERRY_ERROR_MESSAGES
/**
 * Struct to store parser error message with its size.
 */
typedef struct
{
  lit_utf8_byte_t *text; /* Text of parser error message. */
  uint8_t size; /* Size of parser error message. */
} parser_error_message_t;

/* Error message texts with size. */
static parser_error_message_t parser_error_messages[] JERRY_ATTR_CONST_DATA = {
  { (lit_utf8_byte_t *) "", 0 }, /* PARSER_ERR_EMPTY */
/** @cond doxygen_suppress */
#define PARSER_ERROR_DEF(id, utf8_string) { (lit_utf8_byte_t *) utf8_string, sizeof (utf8_string) - 1 },
#include "parser-error-messages.inc.h"
#undef PARSER_ERROR_DEF
  /** @endcond */
};
#endif /* JERRY_ERROR_MESSAGES */

/**
 * Get specified parser error as zero-terminated string
 *
 * @return pointer to zero-terminated parser error
 */
const lit_utf8_byte_t *
parser_get_error_utf8 (uint32_t id) /**< parser error id */
{
  JERRY_ASSERT (id < PARSER_ERR_OUT_OF_MEMORY);

#if JERRY_ERROR_MESSAGES
  return parser_error_messages[id].text;
#else /* !JERRY_ERROR_MESSAGES */
  return NULL;
#endif /* JERRY_ERROR_MESSAGES */
} /* parser_get_error_utf8 */

/**
 * Get size of specified parser error
 *
 * @return size in bytes
 */
lit_utf8_size_t
parser_get_error_size (uint32_t id) /**< parser error id */
{
  JERRY_ASSERT (id < PARSER_ERR_OUT_OF_MEMORY);

#if JERRY_ERROR_MESSAGES
  return parser_error_messages[id].size;
#else /* !JERRY_ERROR_MESSAGES */
  return 0;
#endif /* JERRY_ERROR_MESSAGES */
} /* parser_get_error_size */
