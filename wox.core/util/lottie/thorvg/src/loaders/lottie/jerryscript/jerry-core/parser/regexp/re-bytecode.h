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

#ifndef RE_BYTECODE_H
#define RE_BYTECODE_H

#include "ecma-globals.h"

#include "re-compiler-context.h"

#if JERRY_BUILTIN_REGEXP

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup regexparser Regular expression
 * @{
 *
 * \addtogroup regexparser_bytecode Bytecode
 * @{
 */

/**
 * Size of the RegExp bytecode cache
 */
#define RE_CACHE_SIZE 8u

/**
 * Maximum value that can be encoded in the RegExp bytecode as a single byte.
 */
#define RE_VALUE_1BYTE_MAX 0xFE

/**
 * Marker that signals that the actual value is encoded in the following 4 bytes in the bytecode.
 */
#define RE_VALUE_4BYTE_MARKER 0xFF

/**
 * RegExp opcodes
 */
typedef enum
{
  RE_OP_EOF, /**< end of pattern */

  RE_OP_ALTERNATIVE_START, /**< start of alternatives */
  RE_OP_ALTERNATIVE_NEXT, /**< next alternative */
  RE_OP_NO_ALTERNATIVE, /**< no alternative */

  RE_OP_CAPTURING_GROUP_START, /**< start of a capturing group */
  RE_OP_NON_CAPTURING_GROUP_START, /**< start of a non-capturing group */

  RE_OP_GREEDY_CAPTURING_GROUP_END, /**< end of a greedy capturing group */
  RE_OP_GREEDY_NON_CAPTURING_GROUP_END, /**< end of a greedy non-capturing group */
  RE_OP_LAZY_CAPTURING_GROUP_END, /**< end of a lazy capturing group */
  RE_OP_LAZY_NON_CAPTURING_GROUP_END, /**< end of a lazy non-capturing group */

  RE_OP_GREEDY_ITERATOR, /**< greedy iterator */
  RE_OP_LAZY_ITERATOR, /**< lazy iterator */
  RE_OP_ITERATOR_END, /*** end of an iterator */

  RE_OP_BACKREFERENCE, /**< backreference */

  RE_OP_ASSERT_LINE_START, /**< line start assertion */
  RE_OP_ASSERT_LINE_END, /**< line end assertion */
  RE_OP_ASSERT_WORD_BOUNDARY, /**< word boundary assertion */
  RE_OP_ASSERT_NOT_WORD_BOUNDARY, /**< not word boundary assertion */
  RE_OP_ASSERT_LOOKAHEAD_POS, /**< positive lookahead assertion */
  RE_OP_ASSERT_LOOKAHEAD_NEG, /**< negative lookahead assertion */
  RE_OP_ASSERT_END, /**< end of an assertion */

  RE_OP_CLASS_ESCAPE, /**< class escape */
  RE_OP_CHAR_CLASS, /**< character class */
  RE_OP_UNICODE_PERIOD, /**< period in full unicode mode */
  RE_OP_PERIOD, /**< period in non-unicode mode */
  RE_OP_CHAR, /**< any code point */
  RE_OP_BYTE, /**< 1-byte utf8 character */
} re_opcode_t;

/**
 * Compiled byte code data.
 */
typedef struct
{
  ecma_compiled_code_t header; /**< compiled code header */
  uint32_t captures_count; /**< number of capturing groups */
  uint32_t non_captures_count; /**< number of non-capturing groups */
  ecma_value_t source; /**< original RegExp pattern */
} re_compiled_code_t;

void re_initialize_regexp_bytecode (re_compiler_ctx_t *re_ctx_p);
uint32_t re_bytecode_size (re_compiler_ctx_t *re_ctx_p);

void re_append_opcode (re_compiler_ctx_t *re_ctx_p, const re_opcode_t opcode);
void re_append_byte (re_compiler_ctx_t *re_ctx_p, const uint8_t byte);
void re_append_char (re_compiler_ctx_t *re_ctx_p, const lit_code_point_t cp);
void re_append_value (re_compiler_ctx_t *re_ctx_p, const uint32_t value);

void re_insert_opcode (re_compiler_ctx_t *re_ctx_p, const uint32_t offset, const re_opcode_t opcode);
void re_insert_byte (re_compiler_ctx_t *re_ctx_p, const uint32_t offset, const uint8_t byte);
void re_insert_char (re_compiler_ctx_t *re_ctx_p, const uint32_t offset, const lit_code_point_t cp);
void re_insert_value (re_compiler_ctx_t *re_ctx_p, const uint32_t offset, const uint32_t value);

re_opcode_t re_get_opcode (const uint8_t **bc_p);
uint8_t re_get_byte (const uint8_t **bc_p);
lit_code_point_t re_get_char (const uint8_t **bc_p, bool unicode);
uint32_t re_get_value (const uint8_t **bc_p);

#if JERRY_REGEXP_DUMP_BYTE_CODE
void re_dump_bytecode (re_compiler_ctx_t *bc_ctx);
#endif /* JERRY_REGEXP_DUMP_BYTE_CODE */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
#endif /* !RE_BYTECODE_H */
