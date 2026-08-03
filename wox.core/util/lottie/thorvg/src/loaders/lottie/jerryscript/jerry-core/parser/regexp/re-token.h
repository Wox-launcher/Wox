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

#ifndef RE_TOKEN_H
#define RE_TOKEN_H

#include "ecma-globals.h"

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
 * RegExp token type definitions
 */
typedef enum
{
  RE_TOK_EOF, /**< EOF */
  RE_TOK_BACKREFERENCE, /**< "\[0..9]" */
  RE_TOK_ALTERNATIVE, /**< "|" */
  RE_TOK_ASSERT_START, /**< "^" */
  RE_TOK_ASSERT_END, /**< "$" */
  RE_TOK_PERIOD, /**< "." */
  RE_TOK_START_CAPTURE_GROUP, /**< "(" */
  RE_TOK_START_NON_CAPTURE_GROUP, /**< "(?:" */
  RE_TOK_END_GROUP, /**< ")" */
  RE_TOK_ASSERT_LOOKAHEAD, /**< "(?=" */
  RE_TOK_ASSERT_WORD_BOUNDARY, /**< "\b" */
  RE_TOK_ASSERT_NOT_WORD_BOUNDARY, /**< "\B" */
  RE_TOK_CLASS_ESCAPE, /**< "\d \D \w \W \s \S" */
  RE_TOK_CHAR_CLASS, /**< "[ ]" */
  RE_TOK_CHAR, /**< any character */
} re_token_type_t;

/**
 * RegExp token
 */
typedef struct
{
  uint32_t value; /**< value of the token */
  uint32_t qmin; /**< minimum number of token iterations */
  uint32_t qmax; /**< maximum number of token iterations */
  re_token_type_t type; /**< type of the token */
  bool greedy; /**< type of iteration */
} re_token_t;

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
#endif /* !RE_TOKEN_H */
