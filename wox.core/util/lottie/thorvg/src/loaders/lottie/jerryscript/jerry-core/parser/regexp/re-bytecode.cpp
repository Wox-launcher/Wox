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

#include "re-bytecode.h"

#include "ecma-globals.h"
#include "ecma-regexp-object.h"

#include "lit-strings.h"

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

void
re_initialize_regexp_bytecode (re_compiler_ctx_t *re_ctx_p) /**< RegExp bytecode context */
{
  const size_t initial_size = sizeof (re_compiled_code_t);
  re_ctx_p->bytecode_start_p = (uint8_t *) jmem_heap_alloc_block (initial_size);
  re_ctx_p->bytecode_size = initial_size;
} /* re_initialize_regexp_bytecode */

uint32_t
re_bytecode_size (re_compiler_ctx_t *re_ctx_p) /**< RegExp bytecode context */
{
  return (uint32_t) re_ctx_p->bytecode_size;
} /* re_bytecode_size */

/**
 * Append a new bytecode to the and of the bytecode container
 */
static uint8_t *
re_bytecode_reserve (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                     const size_t size) /**< size */
{
  const size_t old_size = re_ctx_p->bytecode_size;
  const size_t new_size = old_size + size;
  re_ctx_p->bytecode_start_p = reinterpret_cast<uint8_t *>(jmem_heap_realloc_block (re_ctx_p->bytecode_start_p, old_size, new_size));
  re_ctx_p->bytecode_size = new_size;
  return re_ctx_p->bytecode_start_p + old_size;
} /* re_bytecode_reserve */

/**
 * Insert a new bytecode to the bytecode container
 */
static uint8_t *
re_bytecode_insert (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                    const size_t offset, /**< distance from the start of the container */
                    const size_t size) /**< size */
{
  const size_t tail_size = re_ctx_p->bytecode_size - offset;
  re_bytecode_reserve (re_ctx_p, size);

  uint8_t *dest_p = re_ctx_p->bytecode_start_p + offset;
  memmove (dest_p + size, dest_p, tail_size);

  return dest_p;
} /* re_bytecode_insert */

/**
 * Append a byte
 */
void
re_append_byte (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                const uint8_t byte) /**< byte value */
{
  uint8_t *dest_p = re_bytecode_reserve (re_ctx_p, sizeof (uint8_t));
  *dest_p = byte;
} /* re_append_byte */

/**
 * Insert a byte value
 */
void
re_insert_byte (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                const uint32_t offset, /**< distance from the start of the container */
                const uint8_t byte) /**< byte value */
{
  uint8_t *dest_p = re_bytecode_insert (re_ctx_p, offset, sizeof (uint8_t));
  *dest_p = byte;
} /* re_insert_byte */

/**
 * Get a single byte and increase bytecode position.
 */
uint8_t
re_get_byte (const uint8_t **bc_p) /**< pointer to bytecode start */
{
  return *((*bc_p)++);
} /* re_get_byte */

/**
 * Append a RegExp opcode
 */
void
re_append_opcode (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                  const re_opcode_t opcode) /**< input opcode */
{
  re_append_byte (re_ctx_p, (uint8_t) opcode);
} /* re_append_opcode */

/**
 * Insert a RegExp opcode
 */
void
re_insert_opcode (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                  const uint32_t offset, /**< distance from the start of the container */
                  const re_opcode_t opcode) /**< input opcode */
{
  re_insert_byte (re_ctx_p, offset, (uint8_t) opcode);
} /* re_insert_opcode */

/**
 * Get a RegExp opcode and increase the bytecode position
 *
 * @return current RegExp opcode
 */
re_opcode_t
re_get_opcode (const uint8_t **bc_p) /**< pointer to bytecode start */
{
  return (re_opcode_t) re_get_byte (bc_p);
} /* re_get_opcode */

/**
 * Encode 2 byte unsigned integer into the bytecode
 */
static void
re_encode_u16 (uint8_t *dest_p, /**< destination */
               const uint16_t value) /**< value */
{
  *dest_p++ = (uint8_t) ((value >> 8) & 0xFF);
  *dest_p = (uint8_t) (value & 0xFF);
} /* re_encode_u16 */

/**
 * Encode 4 byte unsigned integer into the bytecode
 */
static void
re_encode_u32 (uint8_t *dest_p, /**< destination */
               const uint32_t value) /**< value */
{
  *dest_p++ = (uint8_t) ((value >> 24) & 0xFF);
  *dest_p++ = (uint8_t) ((value >> 16) & 0xFF);
  *dest_p++ = (uint8_t) ((value >> 8) & 0xFF);
  *dest_p = (uint8_t) (value & 0xFF);
} /* re_encode_u32 */

/**
 * Decode 2 byte unsigned integer from bytecode
 *
 * @return uint16_t value
 */
static uint16_t
re_decode_u16 (const uint8_t *src_p) /**< source */
{
  uint16_t value = (uint16_t) (((uint16_t) *src_p++) << 8);
  value = (uint16_t) (value + *src_p++);
  return value;
} /* re_decode_u16 */

/**
 * Decode 4 byte unsigned integer from bytecode
 *
 * @return uint32_t value
 */
static uint32_t JERRY_ATTR_NOINLINE
re_decode_u32 (const uint8_t *src_p) /**< source */
{
  uint32_t value = (uint32_t) (((uint32_t) *src_p++) << 24);
  value += (uint32_t) (((uint32_t) *src_p++) << 16);
  value += (uint32_t) (((uint32_t) *src_p++) << 8);
  value += (uint32_t) (*src_p++);
  return value;
} /* re_decode_u32 */

/**
 * Get the encoded size of an uint32_t value.
 *
 * @return encoded value size
 */
static inline size_t
re_get_encoded_value_size (uint32_t value) /**< value */
{
  if (JERRY_LIKELY (value <= RE_VALUE_1BYTE_MAX))
  {
    return 1;
  }

  return 5;
} /* re_get_encoded_value_size */

/*
 * Encode a value to the specified position in the bytecode.
 */
static void
re_encode_value (uint8_t *dest_p, /**< position in bytecode */
                 const uint32_t value) /**< value */
{
  if (JERRY_LIKELY (value <= RE_VALUE_1BYTE_MAX))
  {
    *dest_p = (uint8_t) value;
    return;
  }

  *dest_p++ = (uint8_t) (RE_VALUE_4BYTE_MARKER);
  re_encode_u32 (dest_p, value);
} /* re_encode_value */

/**
 * Append a value to the end of the bytecode.
 */
void
re_append_value (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                 const uint32_t value) /**< value */
{
  const size_t size = re_get_encoded_value_size (value);
  uint8_t *dest_p = re_bytecode_reserve (re_ctx_p, size);
  re_encode_value (dest_p, value);
} /* re_append_value */

/**
 * Insert a value into the bytecode at a specific offset.
 */
void
re_insert_value (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                 const uint32_t offset, /**< bytecode offset */
                 const uint32_t value) /**< value */
{
  const size_t size = re_get_encoded_value_size (value);
  uint8_t *dest_p = re_bytecode_insert (re_ctx_p, offset, size);
  re_encode_value (dest_p, value);
} /* re_insert_value */

/**
 * Read an encoded value from the bytecode.
 *
 * @return decoded value
 */
uint32_t
re_get_value (const uint8_t **bc_p) /** refence to bytecode pointer */
{
  uint32_t value = *(*bc_p)++;
  if (JERRY_LIKELY (value <= RE_VALUE_1BYTE_MAX))
  {
    return value;
  }

  value = re_decode_u32 (*bc_p);
  *bc_p += sizeof (uint32_t);
  return value;
} /* re_get_value */

/**
 * Append a character to the RegExp bytecode
 */
void
re_append_char (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                const lit_code_point_t cp) /**< code point */
{
  const size_t size = (re_ctx_p->flags & RE_FLAG_UNICODE) ? sizeof (lit_code_point_t) : sizeof (ecma_char_t);
  uint8_t *dest_p = re_bytecode_reserve (re_ctx_p, size);

  if (re_ctx_p->flags & RE_FLAG_UNICODE)
  {
    re_encode_u32 (dest_p, cp);
    return;
  }

  JERRY_ASSERT (cp <= LIT_UTF16_CODE_UNIT_MAX);
  re_encode_u16 (dest_p, (ecma_char_t) cp);
} /* re_append_char */

/**
 * Append a character to the RegExp bytecode
 */
void
re_insert_char (re_compiler_ctx_t *re_ctx_p, /**< RegExp bytecode context */
                const uint32_t offset, /**< bytecode offset */
                const lit_code_point_t cp) /**< code point*/
{
  const size_t size = (re_ctx_p->flags & RE_FLAG_UNICODE) ? sizeof (lit_code_point_t) : sizeof (ecma_char_t);

  uint8_t *dest_p = re_bytecode_insert (re_ctx_p, offset, size);

  if (re_ctx_p->flags & RE_FLAG_UNICODE)
  {
    re_encode_u32 (dest_p, cp);
    return;
  }

  JERRY_ASSERT (cp <= LIT_UTF16_CODE_UNIT_MAX);
  re_encode_u16 (dest_p, (ecma_char_t) cp);
} /* re_insert_char */

/**
 * Decode a character from the bytecode.
 *
 * @return decoded character
 */
lit_code_point_t
re_get_char (const uint8_t **bc_p, /**< reference to bytecode pointer */
             bool unicode) /**< full unicode mode */
{
  lit_code_point_t cp;

  if (unicode)
  {
    cp = re_decode_u32 (*bc_p);
    *bc_p += sizeof (lit_code_point_t);
  }
  else
  {
    cp = re_decode_u16 (*bc_p);
    *bc_p += sizeof (ecma_char_t);
  }

  return cp;
} /* re_get_char */

#if JERRY_REGEXP_DUMP_BYTE_CODE
static uint32_t
re_get_bytecode_offset (const uint8_t *start_p, /**< bytecode start pointer */
                        const uint8_t *current_p) /**< current bytecode pointer */
{
  return (uint32_t) ((uintptr_t) current_p - (uintptr_t) start_p);
} /* re_get_bytecode_offset */

/**
 * RegExp bytecode dumper
 */
void
re_dump_bytecode (re_compiler_ctx_t *re_ctx_p) /**< RegExp bytecode context */
{
  static const char escape_chars[] = { 'd', 'D', 'w', 'W', 's', 'S' };

  re_compiled_code_t *compiled_code_p = (re_compiled_code_t *) re_ctx_p->bytecode_start_p;
  JERRY_DEBUG_MSG ("Flags: 0x%x ", compiled_code_p->header.status_flags);
  JERRY_DEBUG_MSG ("Capturing groups: %d ", compiled_code_p->captures_count);
  JERRY_DEBUG_MSG ("Non-capturing groups: %d\n", compiled_code_p->non_captures_count);

  const uint8_t *bytecode_start_p = (const uint8_t *) (compiled_code_p + 1);
  const uint8_t *bytecode_p = bytecode_start_p;

  while (true)
  {
    JERRY_DEBUG_MSG ("[%3u] ", (uint32_t) ((uintptr_t) bytecode_p - (uintptr_t) bytecode_start_p));
    re_opcode_t op = *bytecode_p++;
    switch (op)
    {
      case RE_OP_ALTERNATIVE_START:
      {
        JERRY_DEBUG_MSG ("ALTERNATIVE_START ");
        const uint32_t offset = re_get_value (&bytecode_p) + re_get_bytecode_offset (bytecode_start_p, bytecode_p);
        JERRY_DEBUG_MSG ("tail offset: [%3u]\n", offset);
        break;
      }
      case RE_OP_ALTERNATIVE_NEXT:
      {
        JERRY_DEBUG_MSG ("ALTERNATIVE_NEXT ");
        const uint32_t offset = re_get_value (&bytecode_p) + re_get_bytecode_offset (bytecode_start_p, bytecode_p);
        JERRY_DEBUG_MSG ("tail offset: [%3u]\n", offset);
        break;
      }
      case RE_OP_NO_ALTERNATIVE:
      {
        JERRY_DEBUG_MSG ("NO_ALTERNATIVES\n");
        break;
      }
      case RE_OP_CAPTURING_GROUP_START:
      {
        JERRY_DEBUG_MSG ("CAPTURING_GROUP_START ");
        JERRY_DEBUG_MSG ("idx: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("capture count: %u, ", re_get_value (&bytecode_p));

        const uint32_t qmin = re_get_value (&bytecode_p);
        JERRY_DEBUG_MSG ("qmin: %u", qmin);
        if (qmin == 0)
        {
          const uint32_t offset = re_get_value (&bytecode_p) + re_get_bytecode_offset (bytecode_start_p, bytecode_p);
          JERRY_DEBUG_MSG (", tail offset: [%3u]\n", offset);
        }
        else
        {
          JERRY_DEBUG_MSG ("\n");
        }

        break;
      }
      case RE_OP_NON_CAPTURING_GROUP_START:
      {
        JERRY_DEBUG_MSG ("NON_CAPTURING_GROUP_START ");
        JERRY_DEBUG_MSG ("idx: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("capture start: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("capture count: %u, ", re_get_value (&bytecode_p));

        const uint32_t qmin = re_get_value (&bytecode_p);
        JERRY_DEBUG_MSG ("qmin: %u", qmin);
        if (qmin == 0)
        {
          const uint32_t offset = re_get_value (&bytecode_p) + re_get_bytecode_offset (bytecode_start_p, bytecode_p);
          JERRY_DEBUG_MSG (", tail offset: [%3u]\n", offset);
        }
        else
        {
          JERRY_DEBUG_MSG ("\n");
        }

        break;
      }
      case RE_OP_GREEDY_CAPTURING_GROUP_END:
      {
        JERRY_DEBUG_MSG ("GREEDY_CAPTURING_GROUP_END ");
        JERRY_DEBUG_MSG ("idx: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmin: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmax: %u\n", re_get_value (&bytecode_p) - RE_QMAX_OFFSET);
        break;
      }
      case RE_OP_LAZY_CAPTURING_GROUP_END:
      {
        JERRY_DEBUG_MSG ("LAZY_CAPTURING_GROUP_END ");
        JERRY_DEBUG_MSG ("idx: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmin: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmax: %u\n", re_get_value (&bytecode_p) - RE_QMAX_OFFSET);
        break;
      }
      case RE_OP_GREEDY_NON_CAPTURING_GROUP_END:
      {
        JERRY_DEBUG_MSG ("GREEDY_NON_CAPTURING_GROUP_END ");
        JERRY_DEBUG_MSG ("idx: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmin: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmax: %u\n", re_get_value (&bytecode_p) - RE_QMAX_OFFSET);
        break;
      }
      case RE_OP_LAZY_NON_CAPTURING_GROUP_END:
      {
        JERRY_DEBUG_MSG ("LAZY_NON_CAPTURING_GROUP_END ");
        JERRY_DEBUG_MSG ("idx: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmin: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmax: %u\n", re_get_value (&bytecode_p) - RE_QMAX_OFFSET);
        break;
      }
      case RE_OP_GREEDY_ITERATOR:
      {
        JERRY_DEBUG_MSG ("GREEDY_ITERATOR ");
        JERRY_DEBUG_MSG ("qmin: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmax: %u, ", re_get_value (&bytecode_p) - RE_QMAX_OFFSET);
        const uint32_t offset = re_get_value (&bytecode_p) + re_get_bytecode_offset (bytecode_start_p, bytecode_p);
        JERRY_DEBUG_MSG ("tail offset: [%3u]\n", offset);
        break;
      }
      case RE_OP_LAZY_ITERATOR:
      {
        JERRY_DEBUG_MSG ("LAZY_ITERATOR ");
        JERRY_DEBUG_MSG ("qmin: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("qmax: %u, ", re_get_value (&bytecode_p) - RE_QMAX_OFFSET);
        const uint32_t offset = re_get_value (&bytecode_p) + re_get_bytecode_offset (bytecode_start_p, bytecode_p);
        JERRY_DEBUG_MSG ("tail offset: [%3u]\n", offset);
        break;
      }
      case RE_OP_ITERATOR_END:
      {
        JERRY_DEBUG_MSG ("ITERATOR_END\n");
        break;
      }
      case RE_OP_BACKREFERENCE:
      {
        JERRY_DEBUG_MSG ("BACKREFERENCE ");
        JERRY_DEBUG_MSG ("idx: %d\n", re_get_value (&bytecode_p));
        break;
      }
      case RE_OP_ASSERT_LINE_START:
      {
        JERRY_DEBUG_MSG ("ASSERT_LINE_START\n");
        break;
      }
      case RE_OP_ASSERT_LINE_END:
      {
        JERRY_DEBUG_MSG ("ASSERT_LINE_END\n");
        break;
      }
      case RE_OP_ASSERT_LOOKAHEAD_POS:
      {
        JERRY_DEBUG_MSG ("ASSERT_LOOKAHEAD_POS ");
        JERRY_DEBUG_MSG ("qmin: %u, ", *bytecode_p++);
        JERRY_DEBUG_MSG ("capture start: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("capture count: %u, ", re_get_value (&bytecode_p));
        const uint32_t offset = re_get_value (&bytecode_p) + re_get_bytecode_offset (bytecode_start_p, bytecode_p);
        JERRY_DEBUG_MSG ("tail offset: [%3u]\n", offset);
        break;
      }
      case RE_OP_ASSERT_LOOKAHEAD_NEG:
      {
        JERRY_DEBUG_MSG ("ASSERT_LOOKAHEAD_NEG ");
        JERRY_DEBUG_MSG ("qmin: %u, ", *bytecode_p++);
        JERRY_DEBUG_MSG ("capture start: %u, ", re_get_value (&bytecode_p));
        JERRY_DEBUG_MSG ("capture count: %u, ", re_get_value (&bytecode_p));
        const uint32_t offset = re_get_value (&bytecode_p) + re_get_bytecode_offset (bytecode_start_p, bytecode_p);
        JERRY_DEBUG_MSG ("tail offset: [%3u]\n", offset);
        break;
      }
      case RE_OP_ASSERT_END:
      {
        JERRY_DEBUG_MSG ("ASSERT_END\n");
        break;
      }
      case RE_OP_ASSERT_WORD_BOUNDARY:
      {
        JERRY_DEBUG_MSG ("ASSERT_WORD_BOUNDARY\n");
        break;
      }
      case RE_OP_ASSERT_NOT_WORD_BOUNDARY:
      {
        JERRY_DEBUG_MSG ("ASSERT_NOT_WORD_BOUNDARY\n");
        break;
      }
      case RE_OP_CLASS_ESCAPE:
      {
        ecma_class_escape_t escape = (ecma_class_escape_t) *bytecode_p++;
        JERRY_DEBUG_MSG ("CLASS_ESCAPE \\%c\n", escape_chars[escape]);
        break;
      }
      case RE_OP_CHAR_CLASS:
      {
        JERRY_DEBUG_MSG ("CHAR_CLASS ");
        uint8_t flags = *bytecode_p++;
        uint32_t char_count = (flags & RE_CLASS_HAS_CHARS) ? re_get_value (&bytecode_p) : 0;
        uint32_t range_count = (flags & RE_CLASS_HAS_RANGES) ? re_get_value (&bytecode_p) : 0;

        if (flags & RE_CLASS_INVERT)
        {
          JERRY_DEBUG_MSG ("inverted ");
        }

        JERRY_DEBUG_MSG ("escapes: ");
        uint8_t escape_count = flags & RE_CLASS_ESCAPE_COUNT_MASK;
        while (escape_count--)
        {
          JERRY_DEBUG_MSG ("\\%c, ", escape_chars[*bytecode_p++]);
        }

        JERRY_DEBUG_MSG ("chars: ");
        while (char_count--)
        {
          JERRY_DEBUG_MSG ("\\u%04x, ", re_get_char (&bytecode_p, re_ctx_p->flags & RE_FLAG_UNICODE));
        }

        JERRY_DEBUG_MSG ("ranges: ");
        while (range_count--)
        {
          const lit_code_point_t begin = re_get_char (&bytecode_p, re_ctx_p->flags & RE_FLAG_UNICODE);
          const lit_code_point_t end = re_get_char (&bytecode_p, re_ctx_p->flags & RE_FLAG_UNICODE);
          JERRY_DEBUG_MSG ("\\u%04x-\\u%04x, ", begin, end);
        }

        JERRY_DEBUG_MSG ("\n");
        break;
      }
      case RE_OP_UNICODE_PERIOD:
      {
        JERRY_DEBUG_MSG ("UNICODE_PERIOD\n");
        break;
      }
      case RE_OP_PERIOD:
      {
        JERRY_DEBUG_MSG ("PERIOD\n");
        break;
      }
      case RE_OP_CHAR:
      {
        JERRY_DEBUG_MSG ("CHAR \\u%04x\n", re_get_char (&bytecode_p, re_ctx_p->flags & RE_FLAG_UNICODE));
        break;
      }
      case RE_OP_BYTE:
      {
        const uint8_t ch = *bytecode_p++;
        JERRY_DEBUG_MSG ("BYTE \\u%04x '%c'\n", ch, (char) ch);
        break;
      }
      case RE_OP_EOF:
      {
        JERRY_DEBUG_MSG ("EOF\n");
        return;
      }
      default:
      {
        JERRY_DEBUG_MSG ("UNKNOWN(%d)\n", (uint32_t) op);
        break;
      }
    }
  }
} /* re_dump_bytecode */
#endif /* JERRY_REGEXP_DUMP_BYTE_CODE */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
