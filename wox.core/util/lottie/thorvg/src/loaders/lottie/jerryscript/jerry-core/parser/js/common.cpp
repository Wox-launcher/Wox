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

#include "common.h"

#include "ecma-big-uint.h"
#include "ecma-bigint.h"
#include "ecma-extended-info.h"
#include "ecma-helpers.h"

#include "js-parser-internal.h"
#include "lit-char-helpers.h"

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_utils Utility
 * @{
 */

#if JERRY_PARSER

/**
 * Free literal.
 */
void
util_free_literal (lexer_literal_t *literal_p) /**< literal */
{
  if (literal_p->type == LEXER_IDENT_LITERAL || literal_p->type == LEXER_STRING_LITERAL)
  {
    if (!(literal_p->status_flags & LEXER_FLAG_SOURCE_PTR))
    {
      jmem_heap_free_block ((void *) literal_p->u.char_p, literal_p->prop.length);
    }
  }
  else if ((literal_p->type == LEXER_FUNCTION_LITERAL) || (literal_p->type == LEXER_REGEXP_LITERAL))
  {
    ecma_bytecode_deref (literal_p->u.bytecode_p);
  }
} /* util_free_literal */

#endif /* JERRY_PARSER */

#if JERRY_PARSER_DUMP_BYTE_CODE

/**
 * Debug utility to print a character sequence.
 */
static void
util_print_chars (const uint8_t *char_p, /**< character pointer */
                  size_t size) /**< size */
{
  while (size > 0)
  {
    JERRY_DEBUG_MSG ("%c", *char_p++);
    size--;
  }
} /* util_print_chars */

/**
 * Debug utility to print a number.
 */
static void
util_print_number (ecma_number_t num_p) /**< number to print */
{
  lit_utf8_byte_t str_buf[ECMA_MAX_CHARS_IN_STRINGIFIED_NUMBER];
  lit_utf8_size_t str_size = ecma_number_to_utf8_string (num_p, str_buf, sizeof (str_buf));
  str_buf[str_size] = 0;
  JERRY_DEBUG_MSG ("%s", str_buf);
} /* util_print_number */

#if JERRY_BUILTIN_BIGINT

/**
 * Debug utility to print a bigint.
 */
static void
util_print_bigint (ecma_value_t bigint) /**< bigint to print */
{
  if (bigint == ECMA_BIGINT_ZERO)
  {
    JERRY_DEBUG_MSG ("0");
    return;
  }

  ecma_extended_primitive_t *bigint_p = ecma_get_extended_primitive_from_value (bigint);
  uint32_t char_start_p, char_size_p;
  lit_utf8_byte_t *string_buffer_p = ecma_big_uint_to_string (bigint_p, 10, &char_start_p, &char_size_p);

  if (JERRY_UNLIKELY (string_buffer_p == NULL))
  {
    JERRY_DEBUG_MSG ("<out-of-memory>");
    return;
  }

  JERRY_ASSERT (char_start_p > 0);

  if (bigint_p->u.bigint_sign_and_size & ECMA_BIGINT_SIGN)
  {
    string_buffer_p[--char_start_p] = LIT_CHAR_MINUS;
  }

  util_print_chars (string_buffer_p + char_start_p, char_size_p - char_start_p);
  jmem_heap_free_block (string_buffer_p, char_size_p);
} /* util_print_bigint */

#endif /* JERRY_BUILTIN_BIGINT */

/**
 * Print literal
 */
void
util_print_literal (lexer_literal_t *literal_p) /**< literal */
{
  switch (literal_p->type)
  {
    case LEXER_IDENT_LITERAL:
    {
      JERRY_DEBUG_MSG ("ident(");
      util_print_chars (literal_p->u.char_p, literal_p->prop.length);
      break;
    }
    case LEXER_FUNCTION_LITERAL:
    {
      JERRY_DEBUG_MSG ("function");
      return;
    }
    case LEXER_STRING_LITERAL:
    {
      JERRY_DEBUG_MSG ("string(");
      util_print_chars (literal_p->u.char_p, literal_p->prop.length);
      break;
    }
    case LEXER_NUMBER_LITERAL:
    {
#if JERRY_BUILTIN_BIGINT
      if (ecma_is_value_bigint (literal_p->u.value))
      {
        JERRY_DEBUG_MSG ("bigint(");
        util_print_bigint (literal_p->u.value);
        break;
      }
#endif /* JERRY_BUILTIN_BIGINT */
      JERRY_DEBUG_MSG ("number(");
      util_print_number (ecma_get_number_from_value (literal_p->u.value));
      break;
    }
    case LEXER_REGEXP_LITERAL:
    {
      JERRY_DEBUG_MSG ("regexp");
      return;
    }
    default:
    {
      JERRY_DEBUG_MSG ("unknown");
      return;
    }
  }

  JERRY_DEBUG_MSG (")");
} /* util_print_literal */

/**
 * Print literal.
 */
static void
util_print_literal_value (ecma_compiled_code_t *compiled_code_p, /**< compiled code */
                          uint16_t literal_index) /**< literal index */
{
  uint16_t argument_end;
  uint16_t register_end;
  uint16_t ident_end;
  uint16_t const_literal_end;
  ecma_value_t *literal_start_p;

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) compiled_code_p;
    argument_end = args_p->argument_end;
    register_end = args_p->register_end;
    ident_end = args_p->ident_end;
    const_literal_end = args_p->const_literal_end;
    literal_start_p = (ecma_value_t *) (args_p + 1);
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) compiled_code_p;
    argument_end = args_p->argument_end;
    register_end = args_p->register_end;
    ident_end = args_p->ident_end;
    const_literal_end = args_p->const_literal_end;
    literal_start_p = (ecma_value_t *) (args_p + 1);
  }

  if (literal_index < argument_end)
  {
    JERRY_DEBUG_MSG (" arg:%d", literal_index);
    return;
  }

  if (literal_index < register_end)
  {
    JERRY_DEBUG_MSG (" reg:%d", literal_index);
    return;
  }

  if (literal_index >= const_literal_end)
  {
    JERRY_DEBUG_MSG (" lit:%d", literal_index);
    return;
  }

  if (literal_index < ident_end)
  {
    JERRY_DEBUG_MSG (" ident:%d->", literal_index);
  }
  else
  {
    JERRY_DEBUG_MSG (" const:%d->", literal_index);
  }

  ecma_value_t value = literal_start_p[literal_index - register_end];

  if (ecma_is_value_number (value))
  {
    JERRY_DEBUG_MSG ("number(");
    util_print_number (ecma_get_number_from_value (value));
  }
#if JERRY_BUILTIN_BIGINT
  else if (ecma_is_value_bigint (value))
  {
    JERRY_DEBUG_MSG ("bigint(");
    util_print_bigint (value);
  }
#endif /* JERRY_BUILTIN_BIGINT */
  else
  {
    ecma_string_t *literal_p = ecma_get_string_from_value (value);

    JERRY_DEBUG_MSG ("string(");

    ECMA_STRING_TO_UTF8_STRING (literal_p, chars_p, literal_size);
    util_print_chars (chars_p, literal_size);
    ECMA_FINALIZE_UTF8_STRING (chars_p, literal_size);
  }

  JERRY_DEBUG_MSG (")");
} /* util_print_literal_value */

#define PARSER_READ_IDENTIFIER_INDEX(name)                               \
  name = *byte_code_p++;                                                 \
  if (name >= encoding_limit)                                            \
  {                                                                      \
    name = (uint16_t) (((name << 8) | byte_code_p[0]) - encoding_delta); \
    byte_code_p++;                                                       \
  }

/**
 * Print byte code.
 */
void
util_print_cbc (ecma_compiled_code_t *compiled_code_p) /**< compiled code */
{
  uint8_t flags;
  uint8_t *byte_code_start_p;
  uint8_t *byte_code_end_p;
  uint8_t *byte_code_p;
  uint16_t encoding_limit;
  uint16_t encoding_delta;
  uint16_t stack_limit;
  uint16_t argument_end;
  uint16_t register_end;
  uint16_t ident_end;
  uint16_t const_literal_end;
  uint16_t literal_end;
  size_t size = ((size_t) compiled_code_p->size) << JMEM_ALIGNMENT_LOG;

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args = (cbc_uint16_arguments_t *) compiled_code_p;
    stack_limit = args->stack_limit;
    argument_end = args->argument_end;
    register_end = args->register_end;
    ident_end = args->ident_end;
    const_literal_end = args->const_literal_end;
    literal_end = args->literal_end;
  }
  else
  {
    cbc_uint8_arguments_t *args = (cbc_uint8_arguments_t *) compiled_code_p;
    stack_limit = args->stack_limit;
    argument_end = args->argument_end;
    register_end = args->register_end;
    ident_end = args->ident_end;
    const_literal_end = args->const_literal_end;
    literal_end = args->literal_end;
  }

  JERRY_DEBUG_MSG ("\nByte code dump:\n\n  Maximum stack depth: %d\n  Flags: [", (int) (stack_limit + register_end));

  if (!(compiled_code_p->status_flags & CBC_CODE_FLAGS_FULL_LITERAL_ENCODING))
  {
    JERRY_DEBUG_MSG ("small_lit_enc");
    encoding_limit = CBC_SMALL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_SMALL_LITERAL_ENCODING_DELTA;
  }
  else
  {
    JERRY_DEBUG_MSG ("full_lit_enc");
    encoding_limit = CBC_FULL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_FULL_LITERAL_ENCODING_DELTA;
  }

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    JERRY_DEBUG_MSG (",uint16_arguments");
  }

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_STRICT_MODE)
  {
    JERRY_DEBUG_MSG (",strict_mode");
  }

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED)
  {
    JERRY_DEBUG_MSG (",mapped_arguments_needed");
    size -= argument_end * sizeof (ecma_value_t);
  }

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_LEXICAL_ENV_NOT_NEEDED)
  {
    JERRY_DEBUG_MSG (",no_lexical_env");
  }

  switch (CBC_FUNCTION_GET_TYPE (compiled_code_p->status_flags))
  {
    case CBC_FUNCTION_CONSTRUCTOR:
    {
      JERRY_DEBUG_MSG (",constructor");
      break;
    }
    case CBC_FUNCTION_GENERATOR:
    {
      JERRY_DEBUG_MSG (",generator");
      break;
    }
    case CBC_FUNCTION_ASYNC:
    {
      JERRY_DEBUG_MSG (",async");
      break;
    }
    case CBC_FUNCTION_ASYNC_GENERATOR:
    {
      JERRY_DEBUG_MSG (",async_generator");
      break;
    }
    case CBC_FUNCTION_ACCESSOR:
    {
      JERRY_DEBUG_MSG (",accessor");
      break;
    }
    case CBC_FUNCTION_ARROW:
    {
      JERRY_DEBUG_MSG (",arrow");
      break;
    }
    case CBC_FUNCTION_ASYNC_ARROW:
    {
      JERRY_DEBUG_MSG (",async_arrow");
      break;
    }
  }

  JERRY_DEBUG_MSG ("]\n");

  JERRY_DEBUG_MSG ("  Argument range end: %d\n", (int) argument_end);
  JERRY_DEBUG_MSG ("  Register range end: %d\n", (int) register_end);
  JERRY_DEBUG_MSG ("  Identifier range end: %d\n", (int) ident_end);
  JERRY_DEBUG_MSG ("  Const literal range end: %d\n", (int) const_literal_end);
  JERRY_DEBUG_MSG ("  Literal range end: %d\n\n", (int) literal_end);

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_HAS_EXTENDED_INFO)
  {
    uint8_t *extended_info_p = ecma_compiled_code_resolve_extended_info (compiled_code_p);
    uint8_t *extended_info_start_p = extended_info_p + sizeof (uint8_t);
    uint8_t extended_info = *extended_info_p;

    if (extended_info & CBC_EXTENDED_CODE_FLAGS_HAS_ARGUMENT_LENGTH)
    {
      uint32_t argument_length = ecma_extended_info_decode_vlq (&extended_info_p);
      JERRY_DEBUG_MSG ("  [Extended] Argument length: %d\n", (int) argument_length);
    }

    if (extended_info & CBC_EXTENDED_CODE_FLAGS_HAS_SOURCE_CODE_RANGE)
    {
      uint32_t range_start = ecma_extended_info_decode_vlq (&extended_info_p);
      uint32_t range_end = ecma_extended_info_decode_vlq (&extended_info_p) + range_start;
      JERRY_DEBUG_MSG ("  [Extended] Source code range: %d - %d\n", (int) range_start, (int) range_end);
    }

    JERRY_DEBUG_MSG ("\n");

    size -= (size_t) (extended_info_start_p - extended_info_p);
  }

  byte_code_start_p = (uint8_t *) compiled_code_p;

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    byte_code_start_p += sizeof (cbc_uint16_arguments_t);
  }
  else
  {
    byte_code_start_p += sizeof (cbc_uint8_arguments_t);
  }

  byte_code_start_p += (unsigned int) (literal_end - register_end) * sizeof (ecma_value_t);

  if (CBC_FUNCTION_GET_TYPE (compiled_code_p->status_flags) != CBC_FUNCTION_CONSTRUCTOR)
  {
    size -= sizeof (ecma_value_t);
  }

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_HAS_TAGGED_LITERALS)
  {
    size -= sizeof (ecma_value_t);
  }

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_HAS_LINE_INFO)
  {
    size -= sizeof (ecma_value_t);
  }

  byte_code_end_p = ((uint8_t *) compiled_code_p) + size;
  byte_code_p = byte_code_start_p;

  while (byte_code_p < byte_code_end_p)
  {
    cbc_opcode_t opcode = (cbc_opcode_t) *byte_code_p;
    cbc_ext_opcode_t ext_opcode = CBC_EXT_NOP;
    size_t cbc_offset = (size_t) (byte_code_p - byte_code_start_p);

    if (opcode != CBC_EXT_OPCODE)
    {
      flags = cbc_flags[opcode];
      JERRY_DEBUG_MSG (" %3d : %s", (int) cbc_offset, cbc_names[opcode]);
      byte_code_p++;
    }
    else
    {
      if (byte_code_p + 1 >= byte_code_end_p)
      {
        break;
      }

      ext_opcode = (cbc_ext_opcode_t) byte_code_p[1];

      if (ext_opcode == CBC_EXT_NOP)
      {
        break;
      }

      flags = cbc_ext_flags[ext_opcode];
      JERRY_DEBUG_MSG (" %3d : %s", (int) cbc_offset, cbc_ext_names[ext_opcode]);
      byte_code_p += 2;
    }

    if (flags & (CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2))
    {
      uint16_t literal_index;

      PARSER_READ_IDENTIFIER_INDEX (literal_index);
      util_print_literal_value (compiled_code_p, literal_index);
    }

    if (flags & CBC_HAS_LITERAL_ARG2)
    {
      uint16_t literal_index;

      PARSER_READ_IDENTIFIER_INDEX (literal_index);
      util_print_literal_value (compiled_code_p, literal_index);

      if (!(flags & CBC_HAS_LITERAL_ARG))
      {
        PARSER_READ_IDENTIFIER_INDEX (literal_index);
        util_print_literal_value (compiled_code_p, literal_index);
      }
    }

    if (flags & CBC_HAS_BYTE_ARG)
    {
      if (opcode == CBC_PUSH_NUMBER_POS_BYTE || opcode == CBC_PUSH_LITERAL_PUSH_NUMBER_POS_BYTE)
      {
        JERRY_DEBUG_MSG (" number:%d", (int) *byte_code_p + 1);
      }
      else if (opcode == CBC_PUSH_NUMBER_NEG_BYTE || opcode == CBC_PUSH_LITERAL_PUSH_NUMBER_NEG_BYTE)
      {
        JERRY_DEBUG_MSG (" number:%d", -((int) *byte_code_p + 1));
      }
      else
      {
        JERRY_DEBUG_MSG (" byte_arg:%d", *byte_code_p);
      }
      byte_code_p++;
    }

    if (flags & CBC_HAS_BRANCH_ARG)
    {
      size_t branch_offset_length =
        (opcode != CBC_EXT_OPCODE ? CBC_BRANCH_OFFSET_LENGTH (opcode) : CBC_BRANCH_OFFSET_LENGTH (ext_opcode));
      size_t offset = 0;

      do
      {
        offset = (offset << 8) | *byte_code_p++;
      } while (--branch_offset_length > 0);

      JERRY_DEBUG_MSG (" offset:%d(->%d)",
                       (int) offset,
                       (int) (cbc_offset + (CBC_BRANCH_IS_FORWARD (flags) ? offset : -offset)));
    }

    JERRY_DEBUG_MSG ("\n");
  }
} /* util_print_cbc */

#undef PARSER_READ_IDENTIFIER_INDEX

#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

/**
 * @}
 * @}
 * @}
 */
