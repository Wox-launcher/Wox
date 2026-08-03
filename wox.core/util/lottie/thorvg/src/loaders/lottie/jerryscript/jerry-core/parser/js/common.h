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

#ifndef COMMON_H
#define COMMON_H

#include <inttypes.h>
#include <setjmp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_utils Utility
 * @{
 */

#include "ecma-globals.h"
#include "ecma-regexp-object.h"

#include "jerry-config.h"
#include "jmem.h"

/* Immediate management. */

/**
 * Literal types.
 *
 * The LEXER_UNUSED_LITERAL type is internal and
 * used for various purposes.
 */
typedef enum
{
  /* The LEXER_IS_IDENT_OR_STRING macro must be updated if the order is changed. */
  LEXER_IDENT_LITERAL = 0, /**< identifier literal */
  LEXER_STRING_LITERAL = 1, /**< string literal */
  LEXER_NUMBER_LITERAL = 2, /**< number literal */
  LEXER_FUNCTION_LITERAL = 3, /**< function literal */
  LEXER_REGEXP_LITERAL = 4, /**< regexp literal */
  LEXER_UNUSED_LITERAL = 5, /**< unused literal, can only be
                                 used by the byte code generator. */
  LEXER_NEW_IDENT_LITERAL = 6, /**< new local variable, can only be
                                    used by the byte code generator. */
} lexer_literal_type_t;

/**
 * Checks whether the literal type is identifier or string.
 */
#define LEXER_IS_IDENT_OR_STRING(literal_type) ((literal_type) <= LEXER_STRING_LITERAL)

/**
 * Flag bits for status_flags member of lexer_literal_t.
 */
typedef enum
{
  LEXER_FLAG_USED = (1 << 0), /**< this local identifier needs to be stored in the constant pool */
  LEXER_FLAG_FUNCTION_ARGUMENT = (1 << 1), /**< this local identifier is a function argument */
  LEXER_FLAG_SOURCE_PTR = (1 << 2), /**< the literal is directly referenced in the source code
                                     *   (no need to allocate memory) */
  LEXER_FLAG_LATE_INIT = (1 << 3), /**< initialize this variable after the byte code is freed */
  LEXER_FLAG_ASCII = (1 << 4), /**< the literal contains only ascii characters */
  LEXER_FLAG_GLOBAL = (1 << 5), /**< this local identifier is not a let or const declaration */
} lexer_literal_status_flags_t;

/**
 * Type of property length.
 */
typedef uint16_t prop_length_t;

/**
 * Literal data.
 */
typedef struct
{
  union
  {
    ecma_value_t value; /**< literal value (not processed by the parser) */
    const uint8_t *char_p; /**< character value */
    ecma_compiled_code_t *bytecode_p; /**< compiled function or regexp pointer */
    uint32_t source_data; /**< encoded source literal */
  } u;

#if JERRY_PARSER_DUMP_BYTE_CODE
  struct
#else /* !JERRY_PARSER_DUMP_BYTE_CODE */
  union
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */
  {
    prop_length_t length; /**< length of ident / string literal */
    uint16_t index; /**< real index during post processing */
  } prop;

  uint8_t type; /**< type of the literal */
  uint8_t status_flags; /**< status flags */
} lexer_literal_t;

void util_free_literal (lexer_literal_t *literal_p);

#if JERRY_PARSER_DUMP_BYTE_CODE
void util_print_literal (lexer_literal_t *);
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

/**
 * Source code line counter type.
 */
typedef uint32_t parser_line_counter_t;

/**
 * Source code as character data.
 */
typedef struct
{
  const uint8_t *source_p; /**< valid UTF-8 source code */
  size_t source_size; /**< size of the source code */
} parser_source_char_t;

/* TRY/CATCH block */

#define PARSER_TRY_CONTEXT(context_name) jmp_buf context_name

#define PARSER_THROW(context_name) longjmp (context_name, 1);

#define PARSER_TRY(context_name) \
  {                              \
    if (!setjmp (context_name))  \
    {
#define PARSER_CATCH \
  }                  \
  else               \
  {
#define PARSER_TRY_END \
  }                    \
  }

/**
 * @}
 * @}
 * @}
 */

#endif /* !COMMON_H */
