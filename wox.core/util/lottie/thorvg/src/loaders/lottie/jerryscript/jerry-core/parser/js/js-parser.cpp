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

#include "ecma-exceptions.h"
#include "ecma-extended-info.h"
#include "ecma-helpers.h"
#include "ecma-literal-storage.h"
#include "ecma-module.h"

#include "jcontext.h"
#include "js-parser-internal.h"
#include "lit-char-helpers.h"

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_parser Parser
 * @{
 */

#if JERRY_PARSER

JERRY_STATIC_ASSERT (((int) ECMA_PARSE_STRICT_MODE == (int) PARSER_IS_STRICT),
                     ecma_parse_strict_mode_must_be_equal_to_parser_is_strict);

JERRY_STATIC_ASSERT (PARSER_SAVE_STATUS_FLAGS (PARSER_ALLOW_SUPER) == 0x1, incorrect_saving_of_ecma_parse_allow_super);
JERRY_STATIC_ASSERT ((PARSER_RESTORE_STATUS_FLAGS (ECMA_PARSE_ALLOW_SUPER) == PARSER_ALLOW_SUPER),
                     incorrect_restoring_of_ecma_parse_allow_super);

JERRY_STATIC_ASSERT ((PARSER_RESTORE_STATUS_FLAGS (ECMA_PARSE_FUNCTION_CONTEXT) == 0),
                     ecma_parse_function_context_must_not_be_transformed);

/**
 * Compute real literal indices.
 *
 * @return length of the prefix opcodes
 */
static void
parser_compute_indices (parser_context_t *context_p, /**< context */
                         uint16_t *ident_end, /**< end of the identifier group */
                         uint16_t *const_literal_end) /**< end of the const literal group */
{
  parser_list_iterator_t literal_iterator;
  lexer_literal_t *literal_p;

  uint16_t ident_count = 0;
  uint16_t const_literal_count = 0;

  uint16_t ident_index;
  uint16_t const_literal_index;
  uint16_t literal_index;

  /* First phase: count the number of items in each group. */
  parser_list_iterator_init (&context_p->literal_pool, &literal_iterator);
  while ((literal_p = (lexer_literal_t *) parser_list_iterator_next (&literal_iterator)))
  {
    switch (literal_p->type)
    {
      case LEXER_IDENT_LITERAL:
      {
        if (literal_p->status_flags & LEXER_FLAG_USED)
        {
          ident_count++;
          break;
        }
        else if (!(literal_p->status_flags & LEXER_FLAG_SOURCE_PTR))
        {
          jmem_heap_free_block ((void *) literal_p->u.char_p, literal_p->prop.length);
          /* This literal should not be freed even if an error is encountered later. */
          literal_p->status_flags |= LEXER_FLAG_SOURCE_PTR;
        }
        continue;
      }
      case LEXER_STRING_LITERAL:
      {
        const_literal_count++;
        break;
      }
      case LEXER_NUMBER_LITERAL:
      {
        const_literal_count++;
        continue;
      }
      case LEXER_FUNCTION_LITERAL:
      case LEXER_REGEXP_LITERAL:
      {
        continue;
      }
      default:
      {
        JERRY_ASSERT (literal_p->type == LEXER_UNUSED_LITERAL);
        continue;
      }
    }

    const uint8_t *char_p = literal_p->u.char_p;
    uint32_t status_flags = context_p->status_flags;

    if ((literal_p->status_flags & LEXER_FLAG_SOURCE_PTR) && literal_p->prop.length < 0xfff)
    {
      size_t bytes_to_end = (size_t) (context_p->source_end_p - char_p);

      if (bytes_to_end < 0xfffff)
      {
        literal_p->u.source_data = ((uint32_t) bytes_to_end) | (((uint32_t) literal_p->prop.length) << 20);
        literal_p->status_flags |= LEXER_FLAG_LATE_INIT;
        status_flags |= PARSER_HAS_LATE_LIT_INIT;
        context_p->status_flags = status_flags;
        char_p = NULL;
      }
    }

    if (char_p != NULL)
    {
      literal_p->u.value = ecma_find_or_create_literal_string (char_p,
                                                               literal_p->prop.length,
                                                               (literal_p->status_flags & LEXER_FLAG_ASCII) != 0);

      if (!(literal_p->status_flags & LEXER_FLAG_SOURCE_PTR))
      {
        jmem_heap_free_block ((void *) char_p, literal_p->prop.length);
        /* This literal should not be freed even if an error is encountered later. */
        literal_p->status_flags |= LEXER_FLAG_SOURCE_PTR;
      }
    }
  }

  ident_index = context_p->register_count;
  const_literal_index = (uint16_t) (ident_index + ident_count);
  literal_index = (uint16_t) (const_literal_index + const_literal_count);

  /* Second phase: Assign an index to each literal. */
  parser_list_iterator_init (&context_p->literal_pool, &literal_iterator);

  while ((literal_p = (lexer_literal_t *) parser_list_iterator_next (&literal_iterator)))
  {
    switch (literal_p->type)
    {
      case LEXER_IDENT_LITERAL:
      {
        if (literal_p->status_flags & LEXER_FLAG_USED)
        {
          literal_p->prop.index = ident_index;
          ident_index++;
        }
        break;
      }
      case LEXER_STRING_LITERAL:
      case LEXER_NUMBER_LITERAL:
      {
        JERRY_ASSERT ((literal_p->status_flags & ~(LEXER_FLAG_SOURCE_PTR | LEXER_FLAG_LATE_INIT)) == 0);
        literal_p->prop.index = const_literal_index;
        const_literal_index++;
        break;
      }
      case LEXER_FUNCTION_LITERAL:
      case LEXER_REGEXP_LITERAL:
      {
        JERRY_ASSERT (literal_p->status_flags == 0);

        literal_p->prop.index = literal_index;
        literal_index++;
        break;
      }
      default:
      {
        JERRY_ASSERT (literal_p->type == LEXER_UNUSED_LITERAL
                      && literal_p->status_flags == LEXER_FLAG_FUNCTION_ARGUMENT);
        break;
      }
    }
  }

  JERRY_ASSERT (ident_index == context_p->register_count + ident_count);
  JERRY_ASSERT (const_literal_index == ident_index + const_literal_count);
  JERRY_ASSERT (literal_index <= context_p->register_count + context_p->literal_count);

  context_p->literal_count = literal_index;

  *ident_end = ident_index;
  *const_literal_end = const_literal_index;
} /* parser_compute_indices */

/**
 * Initialize literal pool.
 */
static void
parser_init_literal_pool (parser_context_t *context_p, /**< context */
                          ecma_value_t *literal_pool_p) /**< start of literal pool */
{
  parser_list_iterator_t literal_iterator;
  lexer_literal_t *literal_p;

  parser_list_iterator_init (&context_p->literal_pool, &literal_iterator);

  while ((literal_p = (lexer_literal_t *) parser_list_iterator_next (&literal_iterator)))
  {
    switch (literal_p->type)
    {
      case LEXER_IDENT_LITERAL:
      {
        if (!(literal_p->status_flags & LEXER_FLAG_USED))
        {
          break;
        }
        /* FALLTHRU */
      }
      case LEXER_STRING_LITERAL:
      {
        ecma_value_t lit_value = literal_p->u.value;

        JERRY_ASSERT (literal_p->prop.index >= context_p->register_count);
        literal_pool_p[literal_p->prop.index] = lit_value;
        break;
      }
      case LEXER_NUMBER_LITERAL:
      {
        JERRY_ASSERT (literal_p->prop.index >= context_p->register_count);

        literal_pool_p[literal_p->prop.index] = literal_p->u.value;
        break;
      }
      case LEXER_FUNCTION_LITERAL:
      case LEXER_REGEXP_LITERAL:
      {
        JERRY_ASSERT (literal_p->prop.index >= context_p->register_count);

        ECMA_SET_INTERNAL_VALUE_POINTER (literal_pool_p[literal_p->prop.index], literal_p->u.bytecode_p);
        break;
      }
      default:
      {
        JERRY_ASSERT (literal_p->type == LEXER_UNUSED_LITERAL);
        break;
      }
    }
  }
} /* parser_init_literal_pool */

/*
 * During byte code post processing certain bytes are not
 * copied into the final byte code buffer. For example, if
 * one byte is enough for encoding a literal index, the
 * second byte is not copied. However, when a byte is skipped,
 * the offsets of those branches which crosses (jumps over)
 * that byte code should also be decreased by one. Instead
 * of finding these jumps every time when a byte is skipped,
 * all branch offset updates are computed in one step.
 *
 * Branch offset mapping example:
 *
 * Let's assume that each parser_mem_page of the byte_code
 * buffer is 8 bytes long and only 4 bytes are kept for a
 * given page:
 *
 * +---+---+---+---+---+---+---+---+
 * | X | 1 | 2 | 3 | X | 4 | X | X |
 * +---+---+---+---+---+---+---+---+
 *
 * X marks those bytes which are removed. The resulting
 * offset mapping is the following:
 *
 * +---+---+---+---+---+---+---+---+
 * | 0 | 1 | 2 | 3 | 3 | 4 | 4 | 4 |
 * +---+---+---+---+---+---+---+---+
 *
 * Each X is simply replaced by the index of the previous
 * index starting from zero. This shows the number of
 * copied bytes before a given byte including the byte
 * itself. The last byte always shows the number of bytes
 * copied from this page.
 *
 * This mapping allows recomputing all branch targets,
 * since mapping[to] - mapping[from] is the new argument
 * for forward branches. As for backward branches, the
 * equation is reversed to mapping[from] - mapping[to].
 *
 * The mapping is relative to one page, so distance
 * computation affecting multiple pages requires a loop.
 * We should also note that only argument bytes can
 * be skipped, so removed bytes cannot be targeted by
 * branches. Valid branches always target instruction
 * starts only.
 */

/**
 * Recompute the argument of a forward branch.
 *
 * @return the new distance
 */
static size_t
parser_update_forward_branch (parser_mem_page_t *page_p, /**< current page */
                              size_t full_distance, /**< full distance */
                              uint8_t bytes_copied_before_jump) /**< bytes copied before jump */
{
  size_t new_distance = 0;

  while (full_distance > PARSER_CBC_STREAM_PAGE_SIZE)
  {
    new_distance += page_p->bytes[PARSER_CBC_STREAM_PAGE_SIZE - 1] & CBC_LOWER_SEVEN_BIT_MASK;
    full_distance -= PARSER_CBC_STREAM_PAGE_SIZE;
    page_p = page_p->next_p;
  }

  new_distance += page_p->bytes[full_distance - 1] & CBC_LOWER_SEVEN_BIT_MASK;
  return new_distance - bytes_copied_before_jump;
} /* parser_update_forward_branch */

/**
 * Recompute the argument of a backward branch.
 *
 * @return the new distance
 */
static size_t
parser_update_backward_branch (parser_mem_page_t *page_p, /**< current page */
                               size_t full_distance, /**< full distance */
                               uint8_t bytes_copied_before_jump) /**< bytes copied before jump */
{
  size_t new_distance = bytes_copied_before_jump;

  while (full_distance >= PARSER_CBC_STREAM_PAGE_SIZE)
  {
    JERRY_ASSERT (page_p != NULL);
    new_distance += page_p->bytes[PARSER_CBC_STREAM_PAGE_SIZE - 1] & CBC_LOWER_SEVEN_BIT_MASK;
    full_distance -= PARSER_CBC_STREAM_PAGE_SIZE;
    page_p = page_p->next_p;
  }

  if (full_distance > 0)
  {
    size_t offset = PARSER_CBC_STREAM_PAGE_SIZE - full_distance;

    JERRY_ASSERT (page_p != NULL);

    new_distance += page_p->bytes[PARSER_CBC_STREAM_PAGE_SIZE - 1] & CBC_LOWER_SEVEN_BIT_MASK;
    new_distance -= page_p->bytes[offset - 1] & CBC_LOWER_SEVEN_BIT_MASK;
  }

  return new_distance;
} /* parser_update_backward_branch */

/**
 * Update targets of all branches in one step.
 */
static void
parse_update_branches (parser_context_t *context_p, /**< context */
                       uint8_t *byte_code_p) /**< byte code */
{
  parser_mem_page_t *page_p = context_p->byte_code.first_p;
  parser_mem_page_t *prev_page_p = NULL;
  parser_mem_page_t *last_page_p = context_p->byte_code.last_p;
  size_t last_position = context_p->byte_code.last_position;
  size_t offset = 0;
  size_t bytes_copied = 0;

  if (last_position >= PARSER_CBC_STREAM_PAGE_SIZE)
  {
    last_page_p = NULL;
    last_position = 0;
  }

  while (page_p != last_page_p || offset < last_position)
  {
    /* Branch instructions are marked to improve search speed. */
    if (page_p->bytes[offset] & CBC_HIGHEST_BIT_MASK)
    {
      uint8_t *bytes_p = byte_code_p + bytes_copied;
      uint8_t flags;
      uint8_t bytes_copied_before_jump = 0;
      size_t branch_argument_length;
      size_t target_distance;
      size_t length;

      if (offset > 0)
      {
        bytes_copied_before_jump = page_p->bytes[offset - 1] & CBC_LOWER_SEVEN_BIT_MASK;
      }
      bytes_p += bytes_copied_before_jump;

      if (*bytes_p == CBC_EXT_OPCODE)
      {
        bytes_p++;
        flags = cbc_ext_flags[*bytes_p];
      }
      else
      {
        flags = cbc_flags[*bytes_p];
      }

      JERRY_ASSERT (flags & CBC_HAS_BRANCH_ARG);
      branch_argument_length = CBC_BRANCH_OFFSET_LENGTH (*bytes_p);
      bytes_p++;

      /* Decoding target. */
      length = branch_argument_length;
      target_distance = 0;
      do
      {
        target_distance = (target_distance << 8) | *bytes_p;
        bytes_p++;
      } while (--length > 0);

      if (CBC_BRANCH_IS_FORWARD (flags))
      {
        /* Branch target was not set. */
        JERRY_ASSERT (target_distance > 0);

        target_distance = parser_update_forward_branch (page_p, offset + target_distance, bytes_copied_before_jump);
      }
      else
      {
        if (target_distance < offset)
        {
          uint8_t bytes_copied_before_target = page_p->bytes[offset - target_distance - 1];
          bytes_copied_before_target = bytes_copied_before_target & CBC_LOWER_SEVEN_BIT_MASK;

          target_distance = (size_t) (bytes_copied_before_jump - bytes_copied_before_target);
        }
        else if (target_distance == offset)
        {
          target_distance = bytes_copied_before_jump;
        }
        else
        {
          target_distance =
            parser_update_backward_branch (prev_page_p, target_distance - offset, bytes_copied_before_jump);
        }
      }

      /* Encoding target again. */
      do
      {
        bytes_p--;
        *bytes_p = (uint8_t) (target_distance & 0xff);
        target_distance >>= 8;
      } while (--branch_argument_length > 0);
    }

    offset++;
    if (offset >= PARSER_CBC_STREAM_PAGE_SIZE)
    {
      parser_mem_page_t *next_p = page_p->next_p;

      /* We reverse the pages before the current page. */
      page_p->next_p = prev_page_p;
      prev_page_p = page_p;

      bytes_copied += page_p->bytes[PARSER_CBC_STREAM_PAGE_SIZE - 1] & CBC_LOWER_SEVEN_BIT_MASK;
      page_p = next_p;
      offset = 0;
    }
  }

  /* After this point the pages of the byte code stream are
   * not used anymore. However, they needs to be freed during
   * cleanup, so the first and last pointers of the stream
   * descriptor are reversed as well. */
  if (last_page_p != NULL)
  {
    JERRY_ASSERT (last_page_p == context_p->byte_code.last_p);
    last_page_p->next_p = prev_page_p;
  }
  else
  {
    last_page_p = context_p->byte_code.last_p;
  }

  context_p->byte_code.last_p = context_p->byte_code.first_p;
  context_p->byte_code.first_p = last_page_p;
} /* parse_update_branches */

/**
 * Forward iterator: move to the next byte code
 *
 * @param page_p page
 * @param offset offset
 */
#define PARSER_NEXT_BYTE(page_p, offset)           \
  do                                               \
  {                                                \
    if (++(offset) >= PARSER_CBC_STREAM_PAGE_SIZE) \
    {                                              \
      offset = 0;                                  \
      page_p = page_p->next_p;                     \
    }                                              \
  } while (0)

/**
 * Forward iterator: move to the next byte code. Also updates the offset of the previous byte code.
 *
 * @param page_p page
 * @param offset offset
 * @param real_offset real offset
 */
#define PARSER_NEXT_BYTE_UPDATE(page_p, offset, real_offset) \
  do                                                         \
  {                                                          \
    page_p->bytes[offset] = real_offset;                     \
    if (++(offset) >= PARSER_CBC_STREAM_PAGE_SIZE)           \
    {                                                        \
      offset = 0;                                            \
      real_offset = 0;                                       \
      page_p = page_p->next_p;                               \
    }                                                        \
  } while (0)

/**
 * Post processing main function.
 *
 * @return compiled code
 */
static ecma_compiled_code_t *
parser_post_processing (parser_context_t *context_p) /**< context */
{
  uint16_t literal_one_byte_limit;
  uint16_t ident_end;
  uint16_t const_literal_end;
  parser_mem_page_t *page_p;
  parser_mem_page_t *last_page_p;
  size_t last_position;
  size_t offset;
  size_t length;
  size_t literal_length;
  size_t total_size;
  uint8_t real_offset;
  uint8_t *byte_code_p;
  bool needs_uint16_arguments;
  cbc_opcode_t last_opcode = CBC_EXT_OPCODE;
  ecma_compiled_code_t *compiled_code_p;
  ecma_value_t *literal_pool_p;
  uint8_t *dst_p;

  if ((context_p->status_flags & (PARSER_IS_FUNCTION | PARSER_LEXICAL_BLOCK_NEEDED))
      == (PARSER_IS_FUNCTION | PARSER_LEXICAL_BLOCK_NEEDED))
  {
    PARSER_MINUS_EQUAL_U16 (context_p->stack_depth, PARSER_BLOCK_CONTEXT_STACK_ALLOCATION);
#ifndef JERRY_NDEBUG
    PARSER_MINUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_BLOCK_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

    context_p->status_flags &= (uint32_t) ~PARSER_LEXICAL_BLOCK_NEEDED;

    parser_emit_cbc (context_p, CBC_CONTEXT_END);

    parser_branch_t branch;
    parser_stack_pop (context_p, &branch, sizeof (parser_branch_t));
    parser_set_branch_to_current_position (context_p, &branch);

    JERRY_ASSERT (!(context_p->status_flags & PARSER_NO_END_LABEL));
  }

  if (PARSER_IS_NORMAL_ASYNC_FUNCTION (context_p->status_flags))
  {
    PARSER_MINUS_EQUAL_U16 (context_p->stack_depth, PARSER_TRY_CONTEXT_STACK_ALLOCATION);
#ifndef JERRY_NDEBUG
    PARSER_MINUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_TRY_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

    if (context_p->stack_limit < PARSER_FINALLY_CONTEXT_STACK_ALLOCATION)
    {
      context_p->stack_limit = PARSER_FINALLY_CONTEXT_STACK_ALLOCATION;
    }

    parser_branch_t branch;

    parser_stack_pop (context_p, &branch, sizeof (parser_branch_t));
    parser_set_branch_to_current_position (context_p, &branch);

    JERRY_ASSERT (!(context_p->status_flags & PARSER_NO_END_LABEL));
  }

  JERRY_ASSERT (context_p->stack_depth == 0);
#ifndef JERRY_NDEBUG
  JERRY_ASSERT (context_p->context_stack_depth == 0);
#endif /* !JERRY_NDEBUG */

  if ((size_t) context_p->stack_limit + (size_t) context_p->register_count > PARSER_MAXIMUM_STACK_LIMIT)
  {
    parser_raise_error (context_p, PARSER_ERR_STACK_LIMIT_REACHED);
  }

  if (JERRY_UNLIKELY (context_p->script_p->refs_and_type >= CBC_SCRIPT_REF_MAX))
  {
    /* This is probably never happens in practice. */
    jerry_fatal (JERRY_FATAL_REF_COUNT_LIMIT);
  }

  context_p->script_p->refs_and_type += CBC_SCRIPT_REF_ONE;

  JERRY_ASSERT (context_p->literal_count <= PARSER_MAXIMUM_NUMBER_OF_LITERALS);

  parser_compute_indices (context_p, &ident_end, &const_literal_end);

  if (context_p->literal_count <= CBC_MAXIMUM_SMALL_VALUE)
  {
    literal_one_byte_limit = CBC_MAXIMUM_BYTE_VALUE - 1;
  }
  else
  {
    literal_one_byte_limit = CBC_LOWER_SEVEN_BIT_MASK;
  }

  last_page_p = context_p->byte_code.last_p;
  last_position = context_p->byte_code.last_position;

  if (last_position >= PARSER_CBC_STREAM_PAGE_SIZE)
  {
    last_page_p = NULL;
    last_position = 0;
  }

  page_p = context_p->byte_code.first_p;
  offset = 0;
  length = 0;

  while (page_p != last_page_p || offset < last_position)
  {
    uint8_t *opcode_p;
    uint8_t flags;
    size_t branch_offset_length;

    opcode_p = page_p->bytes + offset;
    last_opcode = (cbc_opcode_t) (*opcode_p);
    PARSER_NEXT_BYTE (page_p, offset);
    branch_offset_length = CBC_BRANCH_OFFSET_LENGTH (last_opcode);
    flags = cbc_flags[last_opcode];
    length++;

    switch (last_opcode)
    {
      case CBC_EXT_OPCODE:
      {
        cbc_ext_opcode_t ext_opcode;

        ext_opcode = (cbc_ext_opcode_t) page_p->bytes[offset];
        branch_offset_length = CBC_BRANCH_OFFSET_LENGTH (ext_opcode);
        flags = cbc_ext_flags[ext_opcode];
        PARSER_NEXT_BYTE (page_p, offset);
        length++;
        break;
      }
      case CBC_POST_DECR:
      {
        *opcode_p = CBC_PRE_DECR;
        break;
      }
      case CBC_POST_INCR:
      {
        *opcode_p = CBC_PRE_INCR;
        break;
      }
      case CBC_POST_DECR_IDENT:
      {
        *opcode_p = CBC_PRE_DECR_IDENT;
        break;
      }
      case CBC_POST_INCR_IDENT:
      {
        *opcode_p = CBC_PRE_INCR_IDENT;
        break;
      }
      default:
      {
        break;
      }
    }

    while (flags & (CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2))
    {
      uint8_t *first_byte = page_p->bytes + offset;
      uint32_t literal_index = *first_byte;

      PARSER_NEXT_BYTE (page_p, offset);
      length++;

      literal_index |= ((uint32_t) page_p->bytes[offset]) << 8;

      if (literal_index >= PARSER_REGISTER_START)
      {
        literal_index -= PARSER_REGISTER_START;
      }
      else
      {
        literal_index = (PARSER_GET_LITERAL (literal_index))->prop.index;
      }

      if (literal_index <= literal_one_byte_limit)
      {
        *first_byte = (uint8_t) literal_index;
      }
      else
      {
        if (context_p->literal_count <= CBC_MAXIMUM_SMALL_VALUE)
        {
          JERRY_ASSERT (literal_index <= CBC_MAXIMUM_SMALL_VALUE);
          *first_byte = CBC_MAXIMUM_BYTE_VALUE;
          page_p->bytes[offset] = (uint8_t) (literal_index - CBC_MAXIMUM_BYTE_VALUE);
          length++;
        }
        else
        {
          JERRY_ASSERT (literal_index <= CBC_MAXIMUM_FULL_VALUE);
          *first_byte = (uint8_t) ((literal_index >> 8) | CBC_HIGHEST_BIT_MASK);
          page_p->bytes[offset] = (uint8_t) (literal_index & 0xff);
          length++;
        }
      }
      PARSER_NEXT_BYTE (page_p, offset);

      if (flags & CBC_HAS_LITERAL_ARG2)
      {
        if (flags & CBC_HAS_LITERAL_ARG)
        {
          flags = CBC_HAS_LITERAL_ARG;
        }
        else
        {
          flags = CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2;
        }
      }
      else
      {
        break;
      }
    }

    if (flags & CBC_HAS_BYTE_ARG)
    {
      /* This argument will be copied without modification. */
      PARSER_NEXT_BYTE (page_p, offset);
      length++;
    }

    if (flags & CBC_HAS_BRANCH_ARG)
    {
      bool prefix_zero = true;

      /* The leading zeroes are dropped from the stream.
       * Although dropping these zeroes for backward
       * branches are unnecessary, we use the same
       * code path for simplicity. */
      JERRY_ASSERT (branch_offset_length > 0 && branch_offset_length <= 3);

      while (--branch_offset_length > 0)
      {
        uint8_t byte = page_p->bytes[offset];
        if (byte > 0 || !prefix_zero)
        {
          prefix_zero = false;
          length++;
        }
        else
        {
          JERRY_ASSERT (CBC_BRANCH_IS_FORWARD (flags));
        }
        PARSER_NEXT_BYTE (page_p, offset);
      }

      if (last_opcode == (cbc_opcode_t) (CBC_JUMP_FORWARD + PARSER_MAX_BRANCH_LENGTH - 1) && prefix_zero
          && page_p->bytes[offset] == PARSER_MAX_BRANCH_LENGTH + 1)
      {
        /* Unconditional jumps which jump right after the instruction
         * are effectively NOPs. These jumps are removed from the
         * stream. The 1 byte long CBC_JUMP_FORWARD form marks these
         * instructions, since this form is constructed during post
         * processing and cannot be emitted directly. */
        *opcode_p = CBC_JUMP_FORWARD;
        length--;
      }
      else
      {
        /* Other last bytes are always copied. */
        length++;
      }

      PARSER_NEXT_BYTE (page_p, offset);
    }
  }

  if (!(context_p->status_flags & PARSER_NO_END_LABEL) || !(PARSER_OPCODE_IS_RETURN (last_opcode)))
  {
    context_p->status_flags &= (uint32_t) ~PARSER_NO_END_LABEL;

    if (PARSER_IS_NORMAL_ASYNC_FUNCTION (context_p->status_flags))
    {
      length++;
    }

    length++;
  }

  needs_uint16_arguments = false;
  total_size = sizeof (cbc_uint8_arguments_t);

  if (context_p->stack_limit > CBC_MAXIMUM_BYTE_VALUE || context_p->register_count > CBC_MAXIMUM_BYTE_VALUE
      || context_p->literal_count > CBC_MAXIMUM_BYTE_VALUE)
  {
    needs_uint16_arguments = true;
    total_size = sizeof (cbc_uint16_arguments_t);
  }

  literal_length = (size_t) (context_p->literal_count - context_p->register_count) * sizeof (ecma_value_t);

  total_size += literal_length + length;

  if (PARSER_NEEDS_MAPPED_ARGUMENTS (context_p->status_flags))
  {
    total_size += context_p->argument_count * sizeof (ecma_value_t);
  }

  /* function.name */
  if (!(context_p->status_flags & PARSER_CLASS_CONSTRUCTOR))
  {
    total_size += sizeof (ecma_value_t);
  }

  if (context_p->tagged_template_literal_cp != JMEM_CP_NULL)
  {
    total_size += sizeof (ecma_value_t);
  }

  uint8_t extended_info = 0;

  if (context_p->argument_length != UINT16_MAX)
  {
    extended_info |= CBC_EXTENDED_CODE_FLAGS_HAS_ARGUMENT_LENGTH;
    total_size += ecma_extended_info_get_encoded_length (context_p->argument_length);
  }
  if (extended_info != 0)
  {
    total_size += sizeof (uint8_t);
  }

  total_size = JERRY_ALIGNUP (total_size, JMEM_ALIGNMENT);
  compiled_code_p = (ecma_compiled_code_t *) parser_malloc (context_p, total_size);

#if JERRY_SNAPSHOT_SAVE || JERRY_PARSER_DUMP_BYTE_CODE
  // Avoid getting junk bytes
  memset (compiled_code_p, 0, total_size);
#endif /* JERRY_SNAPSHOT_SAVE || JERRY_PARSER_DUMP_BYTE_CODE */

  byte_code_p = (uint8_t *) compiled_code_p;
  compiled_code_p->size = (uint16_t) (total_size >> JMEM_ALIGNMENT_LOG);
  compiled_code_p->refs = 1;
  compiled_code_p->status_flags = 0;

  if (context_p->status_flags & PARSER_FUNCTION_HAS_REST_PARAM)
  {
    JERRY_ASSERT (context_p->argument_count > 0);
    context_p->argument_count--;
  }

  if (needs_uint16_arguments)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) compiled_code_p;

    args_p->stack_limit = context_p->stack_limit;
    args_p->script_value = context_p->script_value;
    args_p->argument_end = context_p->argument_count;
    args_p->register_end = context_p->register_count;
    args_p->ident_end = ident_end;
    args_p->const_literal_end = const_literal_end;
    args_p->literal_end = context_p->literal_count;

    compiled_code_p->status_flags |= CBC_CODE_FLAGS_UINT16_ARGUMENTS;
    byte_code_p += sizeof (cbc_uint16_arguments_t);
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) compiled_code_p;

    args_p->stack_limit = (uint8_t) context_p->stack_limit;
    args_p->argument_end = (uint8_t) context_p->argument_count;
    args_p->script_value = context_p->script_value;
    args_p->register_end = (uint8_t) context_p->register_count;
    args_p->ident_end = (uint8_t) ident_end;
    args_p->const_literal_end = (uint8_t) const_literal_end;
    args_p->literal_end = (uint8_t) context_p->literal_count;

    byte_code_p += sizeof (cbc_uint8_arguments_t);
  }

  uint16_t encoding_limit;
  uint16_t encoding_delta;

  if (context_p->literal_count > CBC_MAXIMUM_SMALL_VALUE)
  {
    compiled_code_p->status_flags |= CBC_CODE_FLAGS_FULL_LITERAL_ENCODING;
    encoding_limit = CBC_FULL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_FULL_LITERAL_ENCODING_DELTA;
  }
  else
  {
    encoding_limit = CBC_SMALL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_SMALL_LITERAL_ENCODING_DELTA;
  }

  if (context_p->status_flags & PARSER_IS_STRICT)
  {
    compiled_code_p->status_flags |= CBC_CODE_FLAGS_STRICT_MODE;
  }

  if ((context_p->status_flags & PARSER_ARGUMENTS_NEEDED) && PARSER_NEEDS_MAPPED_ARGUMENTS (context_p->status_flags))
  {
    compiled_code_p->status_flags |= CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED;
  }

  if (!(context_p->status_flags & PARSER_LEXICAL_ENV_NEEDED))
  {
    compiled_code_p->status_flags |= CBC_CODE_FLAGS_LEXICAL_ENV_NOT_NEEDED;
  }

  uint16_t function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_NORMAL);

  if (context_p->status_flags & (PARSER_IS_PROPERTY_GETTER | PARSER_IS_PROPERTY_SETTER))
  {
    function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_ACCESSOR);
  }
  else if (!(context_p->status_flags & PARSER_IS_FUNCTION))
  {
    function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_SCRIPT);
  }
  else if (context_p->status_flags & PARSER_IS_ARROW_FUNCTION)
  {
    if (context_p->status_flags & PARSER_IS_ASYNC_FUNCTION)
    {
      function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_ASYNC_ARROW);
    }
    else
    {
      function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_ARROW);
    }
  }
  else if (context_p->status_flags & PARSER_IS_GENERATOR_FUNCTION)
  {
    if (context_p->status_flags & PARSER_IS_ASYNC_FUNCTION)
    {
      function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_ASYNC_GENERATOR);
    }
    else
    {
      function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_GENERATOR);
    }
  }
  else if (context_p->status_flags & PARSER_IS_ASYNC_FUNCTION)
  {
    function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_ASYNC);
  }
  else if (context_p->status_flags & PARSER_CLASS_CONSTRUCTOR)
  {
    function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_CONSTRUCTOR);
  }
  else if (context_p->status_flags & PARSER_IS_METHOD)
  {
    function_type = CBC_FUNCTION_TO_TYPE_BITS (CBC_FUNCTION_METHOD);
  }

  if (context_p->status_flags & PARSER_LEXICAL_BLOCK_NEEDED)
  {
    JERRY_ASSERT (!(context_p->status_flags & PARSER_IS_FUNCTION));
    compiled_code_p->status_flags |= CBC_CODE_FLAGS_LEXICAL_BLOCK_NEEDED;
  }

  compiled_code_p->status_flags |= function_type;
  literal_pool_p = ((ecma_value_t *) byte_code_p) - context_p->register_count;
  byte_code_p += literal_length;
  dst_p = byte_code_p;

  parser_init_literal_pool (context_p, literal_pool_p);

  page_p = context_p->byte_code.first_p;
  offset = 0;
  real_offset = 0;
  uint8_t last_register_index =
    (uint8_t) JERRY_MIN (context_p->register_count, (PARSER_MAXIMUM_NUMBER_OF_REGISTERS - 1));

  while (page_p != last_page_p || offset < last_position)
  {
    uint8_t flags;
    uint8_t *opcode_p;
    uint8_t *branch_mark_p;
    cbc_opcode_t opcode;
    size_t branch_offset_length;

    opcode_p = dst_p;
    branch_mark_p = page_p->bytes + offset;
    opcode = (cbc_opcode_t) (*branch_mark_p);
    branch_offset_length = CBC_BRANCH_OFFSET_LENGTH (opcode);

    if (opcode == CBC_JUMP_FORWARD)
    {
      /* These opcodes are deleted from the stream. */
      size_t counter = PARSER_MAX_BRANCH_LENGTH + 1;

      do
      {
        PARSER_NEXT_BYTE_UPDATE (page_p, offset, real_offset);
      } while (--counter > 0);

      continue;
    }

    /* Storing the opcode */
    *dst_p++ = (uint8_t) opcode;
    real_offset++;
    PARSER_NEXT_BYTE_UPDATE (page_p, offset, real_offset);
    flags = cbc_flags[opcode];

    if (opcode == CBC_EXT_OPCODE)
    {
      cbc_ext_opcode_t ext_opcode;

      ext_opcode = (cbc_ext_opcode_t) page_p->bytes[offset];
      flags = cbc_ext_flags[ext_opcode];
      branch_offset_length = CBC_BRANCH_OFFSET_LENGTH (ext_opcode);

      /* Storing the extended opcode */
      *dst_p++ = (uint8_t) ext_opcode;
      opcode_p++;
      real_offset++;
      PARSER_NEXT_BYTE_UPDATE (page_p, offset, real_offset);
    }

    /* Only literal and call arguments can be combined. */
    JERRY_ASSERT (!(flags & CBC_HAS_BRANCH_ARG) || !(flags & (CBC_HAS_BYTE_ARG | CBC_HAS_LITERAL_ARG)));

    while (flags & (CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2))
    {
      uint16_t first_byte = page_p->bytes[offset];

      uint8_t *opcode_pos_p = dst_p - 1;
      *dst_p++ = (uint8_t) first_byte;
      real_offset++;
      PARSER_NEXT_BYTE_UPDATE (page_p, offset, real_offset);

      if (first_byte > literal_one_byte_limit)
      {
        *dst_p++ = page_p->bytes[offset];

        if (first_byte >= encoding_limit)
        {
          first_byte = (uint16_t) (((first_byte << 8) | dst_p[-1]) - encoding_delta);
        }
        real_offset++;
      }
      PARSER_NEXT_BYTE_UPDATE (page_p, offset, real_offset);

      if (flags & CBC_HAS_LITERAL_ARG2)
      {
        if (flags & CBC_HAS_LITERAL_ARG)
        {
          flags = CBC_HAS_LITERAL_ARG;
        }
        else
        {
          flags = CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2;
        }
      }
      else
      {
        if (opcode == CBC_ASSIGN_SET_IDENT && JERRY_LIKELY (first_byte < last_register_index))
        {
          *opcode_pos_p = CBC_MOV_IDENT;
        }

        break;
      }
    }

    if (flags & CBC_HAS_BYTE_ARG)
    {
      /* This argument will be copied without modification. */
      *dst_p++ = page_p->bytes[offset];
      real_offset++;
      PARSER_NEXT_BYTE_UPDATE (page_p, offset, real_offset);
      continue;
    }

    if (flags & CBC_HAS_BRANCH_ARG)
    {
      *branch_mark_p |= CBC_HIGHEST_BIT_MASK;
      bool prefix_zero = true;

      /* The leading zeroes are dropped from the stream. */
      JERRY_ASSERT (branch_offset_length > 0 && branch_offset_length <= 3);

      while (--branch_offset_length > 0)
      {
        uint8_t byte = page_p->bytes[offset];
        if (byte > 0 || !prefix_zero)
        {
          prefix_zero = false;
          *dst_p++ = page_p->bytes[offset];
          real_offset++;
        }
        else
        {
          /* When a leading zero is dropped, the branch
           * offset length must be decreased as well. */
          (*opcode_p)--;
        }
        PARSER_NEXT_BYTE_UPDATE (page_p, offset, real_offset);
      }

      *dst_p++ = page_p->bytes[offset];
      real_offset++;
      PARSER_NEXT_BYTE_UPDATE (page_p, offset, real_offset);
      continue;
    }
  }

  if (!(context_p->status_flags & PARSER_NO_END_LABEL))
  {
    *dst_p++ = CBC_RETURN_FUNCTION_END;

    if (PARSER_IS_NORMAL_ASYNC_FUNCTION (context_p->status_flags))
    {
      dst_p[-1] = CBC_EXT_OPCODE;
      dst_p[0] = CBC_EXT_ASYNC_EXIT;
      dst_p++;
    }
  }
  JERRY_ASSERT (dst_p == byte_code_p + length);

  parse_update_branches (context_p, byte_code_p);

  parser_cbc_stream_free (&context_p->byte_code);

  if (context_p->status_flags & PARSER_HAS_LATE_LIT_INIT)
  {
    parser_list_iterator_t literal_iterator;
    lexer_literal_t *literal_p;
    uint16_t register_count = context_p->register_count;

    parser_list_iterator_init (&context_p->literal_pool, &literal_iterator);
    while ((literal_p = (lexer_literal_t *) parser_list_iterator_next (&literal_iterator)))
    {
      if ((literal_p->status_flags & LEXER_FLAG_LATE_INIT) && literal_p->prop.index >= register_count)
      {
        uint32_t source_data = literal_p->u.source_data;
        const uint8_t *char_p = context_p->source_end_p - (source_data & 0xfffff);
        ecma_value_t lit_value = ecma_find_or_create_literal_string (char_p,
                                                                     source_data >> 20,
                                                                     (literal_p->status_flags & LEXER_FLAG_ASCII) != 0);
        literal_pool_p[literal_p->prop.index] = lit_value;
      }
    }
  }

  ecma_value_t *base_p = (ecma_value_t *) (((uint8_t *) compiled_code_p) + total_size);

  if (PARSER_NEEDS_MAPPED_ARGUMENTS (context_p->status_flags))
  {
    parser_list_iterator_t literal_iterator;
    uint16_t argument_count = 0;
    base_p -= context_p->argument_count;

    parser_list_iterator_init (&context_p->literal_pool, &literal_iterator);
    while (argument_count < context_p->argument_count)
    {
      lexer_literal_t *literal_p;
      literal_p = (lexer_literal_t *) parser_list_iterator_next (&literal_iterator);

      if (!(literal_p->status_flags & LEXER_FLAG_FUNCTION_ARGUMENT))
      {
        continue;
      }

      /* All arguments must be moved to initialized registers. */
      if (literal_p->type == LEXER_UNUSED_LITERAL)
      {
        base_p[argument_count] = ECMA_VALUE_EMPTY;
        argument_count++;
        continue;
      }
      base_p[argument_count] = literal_pool_p[literal_p->prop.index];
      argument_count++;
    }
  }

  if (!(context_p->status_flags & PARSER_CLASS_CONSTRUCTOR))
  {
    *(--base_p) = ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY);
  }

  if (context_p->tagged_template_literal_cp != JMEM_CP_NULL)
  {
    compiled_code_p->status_flags |= CBC_CODE_FLAGS_HAS_TAGGED_LITERALS;
    *(--base_p) = (ecma_value_t) context_p->tagged_template_literal_cp;
  }

  if (extended_info != 0)
  {
    uint8_t *extended_info_p = ((uint8_t *) base_p) - 1;

    compiled_code_p->status_flags |= CBC_CODE_FLAGS_HAS_EXTENDED_INFO;
    *extended_info_p = extended_info;

    if (context_p->argument_length != UINT16_MAX)
    {
      ecma_extended_info_encode_vlq (&extended_info_p, context_p->argument_length);
    }
  }

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    util_print_cbc (compiled_code_p);
    JERRY_DEBUG_MSG ("\nByte code size: %d bytes\n", (int) length);
    context_p->total_byte_code_size += (uint32_t) length;
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  return compiled_code_p;
} /* parser_post_processing */

#undef PARSER_NEXT_BYTE
#undef PARSER_NEXT_BYTE_UPDATE

/**
 * Resolve private identifier in direct eval context
 */
static bool
parser_resolve_private_identifier_eval (parser_context_t *context_p) /**< context */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_string_t *search_key_p;
  uint8_t *destination_p = (uint8_t *) parser_malloc (context_p, context_p->token.lit_location.length);

  lexer_convert_ident_to_cesu8 (destination_p,
                                context_p->token.lit_location.char_p,
                                context_p->token.lit_location.length);

  search_key_p = ecma_new_ecma_string_from_utf8 (destination_p, context_p->token.lit_location.length);

  parser_free (destination_p, context_p->token.lit_location.length);

  ecma_object_t *lex_env_p = JERRY_CONTEXT (vm_top_context_p)->lex_env_p;

  while (true)
  {
    JERRY_ASSERT (lex_env_p != NULL);

    if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_CLASS
        && (lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_LEXICAL_ENV_HAS_DATA) != 0
        && !ECMA_LEX_ENV_CLASS_IS_MODULE (lex_env_p))
    {
      ecma_object_t *class_object_p = ((ecma_lexical_environment_class_t *) lex_env_p)->object_p;

      ecma_string_t *internal_string_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS);
      ecma_property_t *prop_p = ecma_find_named_property (class_object_p, internal_string_p);

      if (prop_p != NULL)
      {
        ecma_value_t *collection_p =
          ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, ECMA_PROPERTY_VALUE_PTR (prop_p)->value);
        ecma_value_t *current_p = collection_p + 1;
        ecma_value_t *end_p = ecma_compact_collection_end (collection_p);

        while (current_p < end_p)
        {
          current_p++; /* skip kind */
          ecma_string_t *private_key_p = ecma_get_prop_name_from_value (*current_p++);
          current_p++; /* skip value */

          JERRY_ASSERT (ecma_prop_name_is_symbol (private_key_p));

          ecma_string_t *private_key_desc_p =
            ecma_get_string_from_value (((ecma_extended_string_t *) private_key_p)->u.symbol_descriptor);

          if (ecma_compare_ecma_strings (private_key_desc_p, search_key_p))
          {
            ecma_deref_ecma_string (search_key_p);
            lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_STRING_LITERAL);
            return true;
          }
        }
      }
    }

    if (lex_env_p->u2.outer_reference_cp == JMEM_CP_NULL)
    {
      break;
    }

    lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
  }

  ecma_deref_ecma_string (search_key_p);
  return false;
} /* parser_resolve_private_identifier_eval */

/**
 * Resolve private identifier
 */
void
parser_resolve_private_identifier (parser_context_t *context_p) /**< context */
{
  if ((context_p->global_status_flags & ECMA_PARSE_DIRECT_EVAL) && parser_resolve_private_identifier_eval (context_p))
  {
    return;
  }

  parser_private_context_t *context_iter_p = context_p->private_context_p;

  while (context_iter_p)
  {
    if (context_iter_p == NULL || !(context_iter_p->opts & SCANNER_PRIVATE_FIELD_ACTIVE))
    {
      parser_raise_error (context_p, PARSER_ERR_UNDECLARED_PRIVATE_FIELD);
    }

    if (!(context_iter_p->opts & SCANNER_SUCCESSFUL_CLASS_SCAN))
    {
      lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_STRING_LITERAL);
      return;
    }

    parser_private_context_t *private_context_p = context_iter_p;

    if (private_context_p == NULL)
    {
      parser_raise_error (context_p, PARSER_ERR_UNDECLARED_PRIVATE_FIELD);
    }

    scanner_class_private_member_t *ident_iter = private_context_p->members_p;

    while (ident_iter)
    {
      if (lexer_compare_identifiers (context_p, &context_p->token.lit_location, &ident_iter->loc))
      {
        lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_STRING_LITERAL);
        return;
      }

      ident_iter = ident_iter->prev_p;
    }

    context_iter_p = context_iter_p->prev_p;
  }

  parser_raise_error (context_p, PARSER_ERR_UNDECLARED_PRIVATE_FIELD);
} /* parser_resolve_private_identifier */

/**
 * Save private field context
 */
void
parser_save_private_context (parser_context_t *context_p, /**< context */
                             parser_private_context_t *private_ctx_p, /**< private context */
                             scanner_class_info_t *class_info_p) /**< class scanner info */
{
  private_ctx_p->prev_p = context_p->private_context_p;
  context_p->private_context_p = private_ctx_p;

  context_p->private_context_p->members_p = class_info_p->members;
  context_p->private_context_p->opts = class_info_p->info.u8_arg;
  class_info_p->members = NULL;
} /* parser_save_private_context */

/**
 * Release contexts private fields
 */
static void
parser_free_private_fields (parser_context_t *context_p) /**< context */
{
  parser_private_context_t *iter = context_p->private_context_p;

  while (iter != NULL)
  {
    parser_private_context_t *prev_p = iter->prev_p;
    scanner_release_private_fields (iter->members_p);
    iter = prev_p;
  }
} /* parser_free_private_fields */

/**
 * Restore contexts private fields
 */
void
parser_restore_private_context (parser_context_t *context_p, /**< context */
                                parser_private_context_t *private_ctx_p) /**< private context */
{
  scanner_release_private_fields (context_p->private_context_p->members_p);
  context_p->private_context_p = private_ctx_p->prev_p;
} /* parser_restore_private_context */

/**
 * Free identifiers and literals.
 */
static void
parser_free_literals (parser_list_t *literal_pool_p) /**< literals */
{
  parser_list_iterator_t literal_iterator;
  lexer_literal_t *literal_p;

  parser_list_iterator_init (literal_pool_p, &literal_iterator);
  while ((literal_p = (lexer_literal_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
  {
    util_free_literal (literal_p);
  }

  parser_list_free (literal_pool_p);
} /* parser_free_literals */

/**
 * Parse function arguments
 */
static void
parser_parse_function_arguments (parser_context_t *context_p, /**< context */
                                 lexer_token_type_t end_type) /**< expected end type */
{
  JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_FUNCTION);

  JERRY_ASSERT (context_p->status_flags & PARSER_IS_FUNCTION);
  JERRY_ASSERT (!(context_p->status_flags & PARSER_LEXICAL_BLOCK_NEEDED));

  bool has_duplicated_arg_names = false;

  if (PARSER_IS_NORMAL_ASYNC_FUNCTION (context_p->status_flags))
  {
    parser_branch_t branch;
    parser_emit_cbc_ext_forward_branch (context_p, CBC_EXT_TRY_CREATE_CONTEXT, &branch);
    parser_stack_push (context_p, &branch, sizeof (parser_branch_t));

#ifndef JERRY_NDEBUG
    context_p->context_stack_depth = PARSER_TRY_CONTEXT_STACK_ALLOCATION;
#endif /* !JERRY_NDEBUG */
  }

  if (context_p->token.type == end_type)
  {
    context_p->status_flags &= (uint32_t) ~PARSER_DISALLOW_AWAIT_YIELD;

    if (context_p->status_flags & PARSER_IS_GENERATOR_FUNCTION)
    {
      scanner_create_variables (context_p, SCANNER_CREATE_VARS_IS_FUNCTION_ARGS);
      parser_emit_cbc_ext (context_p, CBC_EXT_CREATE_GENERATOR);
      parser_emit_cbc (context_p, CBC_POP);
      scanner_create_variables (context_p, SCANNER_CREATE_VARS_IS_FUNCTION_BODY);
      return;
    }

    scanner_create_variables (context_p, SCANNER_CREATE_VARS_NO_OPTS);
    return;
  }

  bool has_complex_argument = (context_p->next_scanner_info_p->u8_arg & SCANNER_FUNCTION_HAS_COMPLEX_ARGUMENT) != 0;
  bool is_strict = (context_p->next_scanner_info_p->u8_arg & SCANNER_FUNCTION_IS_STRICT) != 0;

  scanner_create_variables (context_p, SCANNER_CREATE_VARS_IS_FUNCTION_ARGS);
  scanner_set_active (context_p);

  context_p->status_flags |= PARSER_FUNCTION_IS_PARSING_ARGS;

  while (true)
  {
    if (context_p->token.type == LEXER_THREE_DOTS)
    {
      if (context_p->status_flags & PARSER_IS_PROPERTY_SETTER)
      {
        parser_raise_error (context_p, PARSER_ERR_SETTER_REST_PARAMETER);
      }
      lexer_next_token (context_p);

      if (has_duplicated_arg_names)
      {
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_ARGUMENT_NAMES);
      }

      context_p->status_flags |= PARSER_FUNCTION_HAS_REST_PARAM | PARSER_FUNCTION_HAS_COMPLEX_ARGUMENT;
    }

    if (context_p->token.type == LEXER_LEFT_SQUARE || context_p->token.type == LEXER_LEFT_BRACE)
    {
      if (has_duplicated_arg_names)
      {
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_ARGUMENT_NAMES);
      }

      context_p->status_flags |= PARSER_FUNCTION_HAS_COMPLEX_ARGUMENT;

      if (!(context_p->status_flags & PARSER_FUNCTION_HAS_REST_PARAM))
      {
        parser_emit_cbc_literal (context_p,
                                 CBC_PUSH_LITERAL,
                                 (uint16_t) (PARSER_REGISTER_START + context_p->argument_count));
      }
      else
      {
        parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_REST_OBJECT);
      }

      uint32_t flags =
        (PARSER_PATTERN_BINDING | PARSER_PATTERN_TARGET_ON_STACK | PARSER_PATTERN_LOCAL | PARSER_PATTERN_ARGUMENTS);

      if (context_p->next_scanner_info_p->source_p == context_p->source_p)
      {
        if (context_p->next_scanner_info_p->type == SCANNER_TYPE_INITIALIZER)
        {
          if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_REST)
          {
            flags |= PARSER_PATTERN_HAS_REST_ELEMENT;
          }

          if (context_p->status_flags & PARSER_FUNCTION_HAS_REST_PARAM)
          {
            parser_raise_error (context_p, PARSER_ERR_REST_PARAMETER_DEFAULT_INITIALIZER);
          }

          if (context_p->argument_length == UINT16_MAX)
          {
            context_p->argument_length = context_p->argument_count;
          }

          flags |= PARSER_PATTERN_TARGET_DEFAULT;
        }
        else if (context_p->next_scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS)
        {
          if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_REST)
          {
            flags |= PARSER_PATTERN_HAS_REST_ELEMENT;
          }
          scanner_release_next (context_p, sizeof (scanner_info_t));
        }
        else
        {
          parser_raise_error (context_p, PARSER_ERR_INVALID_DESTRUCTURING_PATTERN);
        }
      }

      parser_parse_initializer (context_p, (parser_pattern_flags_t) flags);

      context_p->argument_count++;
      if (context_p->argument_count >= PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
      {
        parser_raise_error (context_p, PARSER_ERR_ARGUMENT_LIMIT_REACHED);
      }

      if (context_p->token.type != LEXER_COMMA)
      {
        if (context_p->token.type != end_type)
        {
          parser_error_msg_t error =
            ((end_type == LEXER_RIGHT_PAREN) ? PARSER_ERR_RIGHT_PAREN_EXPECTED : PARSER_ERR_IDENTIFIER_EXPECTED);

          parser_raise_error (context_p, error);
        }
        break;
      }

      lexer_next_token (context_p);

      if (context_p->token.type == end_type)
      {
        break;
      }
      continue;
    }

    if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
    {
      parser_raise_error (context_p, PARSER_ERR_IDENTIFIER_EXPECTED);
    }

    lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_IDENT_LITERAL);

    if (context_p->token.keyword_type >= LEXER_FIRST_NON_STRICT_ARGUMENTS)
    {
      context_p->status_flags |= PARSER_HAS_NON_STRICT_ARG;
    }

    if (JERRY_UNLIKELY (context_p->lit_object.literal_p->status_flags & LEXER_FLAG_FUNCTION_ARGUMENT))
    {
      if ((context_p->status_flags & PARSER_FUNCTION_HAS_COMPLEX_ARGUMENT)
          || (context_p->status_flags & PARSER_IS_ARROW_FUNCTION))
      {
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_ARGUMENT_NAMES);
      }
      has_duplicated_arg_names = true;

      context_p->status_flags |= PARSER_HAS_NON_STRICT_ARG;
    }
    else
    {
      context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_FUNCTION_ARGUMENT;
    }

    lexer_next_token (context_p);

    uint16_t literal_index = context_p->lit_object.index;

    if (context_p->token.type == LEXER_ASSIGN)
    {
      JERRY_ASSERT (has_complex_argument);

      if (context_p->status_flags & PARSER_FUNCTION_HAS_REST_PARAM)
      {
        parser_raise_error (context_p, PARSER_ERR_REST_PARAMETER_DEFAULT_INITIALIZER);
      }

      if (context_p->argument_length == UINT16_MAX)
      {
        context_p->argument_length = context_p->argument_count;
      }

      parser_branch_t skip_init;

      if (has_duplicated_arg_names)
      {
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_ARGUMENT_NAMES);
      }

      context_p->status_flags |= PARSER_FUNCTION_HAS_COMPLEX_ARGUMENT;

      /* LEXER_ASSIGN does not overwrite lit_object. */
      parser_emit_cbc_literal (context_p,
                               CBC_PUSH_LITERAL,
                               (uint16_t) (PARSER_REGISTER_START + context_p->argument_count));
      parser_emit_cbc_ext_forward_branch (context_p, CBC_EXT_DEFAULT_INITIALIZER, &skip_init);

      lexer_next_token (context_p);
      parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

      parser_set_branch_to_current_position (context_p, &skip_init);

      uint16_t opcode = CBC_ASSIGN_LET_CONST;

      if (literal_index >= PARSER_REGISTER_START)
      {
        opcode = CBC_MOV_IDENT;
      }
      else if (!scanner_literal_is_created (context_p, literal_index))
      {
        opcode = CBC_INIT_ARG_OR_CATCH;
      }

      parser_emit_cbc_literal (context_p, opcode, literal_index);
    }
    else if (context_p->status_flags & PARSER_FUNCTION_HAS_REST_PARAM)
    {
      parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_REST_OBJECT);

      uint16_t opcode = CBC_MOV_IDENT;

      if (literal_index < PARSER_REGISTER_START)
      {
        opcode = CBC_INIT_ARG_OR_CATCH;

        if (scanner_literal_is_created (context_p, literal_index))
        {
          opcode = CBC_ASSIGN_LET_CONST;
        }
      }

      parser_emit_cbc_literal (context_p, opcode, literal_index);
    }
    else if (has_complex_argument && literal_index < PARSER_REGISTER_START)
    {
      uint16_t opcode = CBC_INIT_ARG_OR_FUNC;

      if (scanner_literal_is_created (context_p, literal_index))
      {
        opcode = CBC_ASSIGN_LET_CONST_LITERAL;
      }

      parser_emit_cbc_literal_value (context_p,
                                     opcode,
                                     (uint16_t) (PARSER_REGISTER_START + context_p->argument_count),
                                     literal_index);
    }

    context_p->argument_count++;
    if (context_p->argument_count >= PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
    {
      parser_raise_error (context_p, PARSER_ERR_ARGUMENT_LIMIT_REACHED);
    }

    if (context_p->token.type != LEXER_COMMA)
    {
      if (context_p->token.type != end_type)
      {
        parser_error_msg_t error =
          ((end_type == LEXER_RIGHT_PAREN) ? PARSER_ERR_RIGHT_PAREN_EXPECTED : PARSER_ERR_IDENTIFIER_EXPECTED);

        parser_raise_error (context_p, error);
      }
      break;
    }

    if (context_p->status_flags & PARSER_FUNCTION_HAS_REST_PARAM)
    {
      parser_raise_error (context_p, PARSER_ERR_FORMAL_PARAM_AFTER_REST_PARAMETER);
    }

    lexer_next_token (context_p);

    if (context_p->token.type == end_type)
    {
      break;
    }
  }

  scanner_revert_active (context_p);

  JERRY_ASSERT (has_complex_argument || !(context_p->status_flags & PARSER_FUNCTION_HAS_COMPLEX_ARGUMENT));

  if (context_p->status_flags & PARSER_IS_GENERATOR_FUNCTION)
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_CREATE_GENERATOR);
    parser_emit_cbc (context_p, CBC_POP);
  }

  if (context_p->status_flags & PARSER_LEXICAL_BLOCK_NEEDED)
  {
    if ((context_p->next_scanner_info_p->u8_arg & SCANNER_FUNCTION_LEXICAL_ENV_NEEDED)
        || scanner_is_context_needed (context_p, PARSER_CHECK_FUNCTION_CONTEXT))
    {
      context_p->status_flags |= PARSER_LEXICAL_ENV_NEEDED;

      parser_branch_t branch;
      parser_emit_cbc_forward_branch (context_p, CBC_BLOCK_CREATE_CONTEXT, &branch);
      parser_stack_push (context_p, &branch, sizeof (parser_branch_t));

#ifndef JERRY_NDEBUG
      PARSER_PLUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_BLOCK_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */
    }
    else
    {
      context_p->status_flags &= (uint32_t) ~PARSER_LEXICAL_BLOCK_NEEDED;
    }
  }

  context_p->status_flags &= (uint32_t) ~(PARSER_DISALLOW_AWAIT_YIELD | PARSER_FUNCTION_IS_PARSING_ARGS);
  scanner_create_variables (context_p, SCANNER_CREATE_VARS_IS_FUNCTION_BODY);

  if (is_strict)
  {
    context_p->status_flags |= PARSER_IS_STRICT;
  }
} /* parser_parse_function_arguments */

#ifndef JERRY_NDEBUG
JERRY_STATIC_ASSERT ((PARSER_SCANNING_SUCCESSFUL == PARSER_HAS_LATE_LIT_INIT),
                     parser_scanning_successful_should_share_the_bit_position_with_parser_has_late_lit_init);
#endif /* !JERRY_NDEBUG */

/**
 * Parser script size
 */
static size_t
parser_script_size (parser_context_t *context_p) /**< context */
{
  size_t script_size = sizeof (cbc_script_t);

  if (context_p->user_value != ECMA_VALUE_EMPTY)
  {
    script_size += sizeof (ecma_value_t);
  }

#if JERRY_MODULE_SYSTEM
  if (context_p->global_status_flags & ECMA_PARSE_INTERNAL_HAS_IMPORT_META)
  {
    script_size += sizeof (ecma_value_t);
  }
#endif /* JERRY_MODULE_SYSTEM */
  return script_size;
} /* parser_script_size */

#if JERRY_SOURCE_NAME
/**
 * Parser resource name
 */
static ecma_value_t
parser_source_name (parser_context_t *context_p) /**< context */
{
  if (context_p->options_p != NULL && (context_p->options_p->options & JERRY_PARSE_HAS_SOURCE_NAME))
  {
    JERRY_ASSERT (ecma_is_value_string (context_p->options_p->source_name));

    ecma_ref_ecma_string (ecma_get_string_from_value (context_p->options_p->source_name));
    return context_p->options_p->source_name;
  }

  if (context_p->global_status_flags & ECMA_PARSE_EVAL)
  {
    return ecma_make_magic_string_value (LIT_MAGIC_STRING_SOURCE_NAME_EVAL);
  }

  return ecma_make_magic_string_value (LIT_MAGIC_STRING_SOURCE_NAME_ANON);
} /* parser_source_name */
#endif /* JERRY_SOURCE_NAME */

/**
 * Parse and compile EcmaScript source code
 *
 * Note: source must be a valid UTF-8 string
 *
 * @return compiled code
 */
static ecma_compiled_code_t *
parser_parse_source (void *source_p, /**< source code */
                     uint32_t parse_opts, /**< ecma_parse_opts_t option bits */
                     const jerry_parse_options_t *options_p) /**< additional configuration options */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  parser_context_t context;
  ecma_compiled_code_t *compiled_code_p;

  context.error = PARSER_ERR_NO_ERROR;
  context.status_flags = parse_opts & PARSER_STRICT_MODE_MASK;
  context.global_status_flags = parse_opts;

  context.status_flags |= PARSER_RESTORE_STATUS_FLAGS (parse_opts);
  context.tagged_template_literal_cp = JMEM_CP_NULL;

  context.stack_depth = 0;
  context.stack_limit = 0;
  context.options_p = options_p;
  context.script_p = NULL;
  context.arguments_start_p = NULL;
  context.arguments_size = 0;
#if JERRY_MODULE_SYSTEM
  if (context.global_status_flags & ECMA_PARSE_MODULE)
  {
    context.status_flags |= PARSER_IS_STRICT;
  }

  context.module_names_p = NULL;
#endif /* JERRY_MODULE_SYSTEM */

  context.argument_list = ECMA_VALUE_EMPTY;

  if (context.options_p != NULL && (context.options_p->options & JERRY_PARSE_HAS_ARGUMENT_LIST))
  {
    context.argument_list = context.options_p->argument_list;
  }
  else if (context.global_status_flags & ECMA_PARSE_HAS_ARGUMENT_LIST_VALUE)
  {
    JERRY_ASSERT (context.global_status_flags & ECMA_PARSE_HAS_SOURCE_VALUE);
    context.argument_list = ((ecma_value_t *) source_p)[1];
  }

  if (context.argument_list != ECMA_VALUE_EMPTY)
  {
    JERRY_ASSERT (ecma_is_value_string (context.argument_list));

    context.status_flags |= PARSER_IS_FUNCTION;

    if (parse_opts & ECMA_PARSE_GENERATOR_FUNCTION)
    {
      context.status_flags |= PARSER_IS_GENERATOR_FUNCTION;
    }
    if (parse_opts & ECMA_PARSE_ASYNC_FUNCTION)
    {
      context.status_flags |= PARSER_IS_ASYNC_FUNCTION;
    }

    ecma_string_t *string_p = ecma_get_string_from_value (context.argument_list);
    uint8_t flags = ECMA_STRING_FLAG_EMPTY;

    context.arguments_start_p = ecma_string_get_chars (string_p, &context.arguments_size, NULL, NULL, &flags);

    if (flags & ECMA_STRING_FLAG_MUST_BE_FREED)
    {
      context.global_status_flags |= ECMA_PARSE_INTERNAL_FREE_ARG_LIST;
    }
  }

  if (!(context.global_status_flags & ECMA_PARSE_HAS_SOURCE_VALUE))
  {
    context.source_start_p = ((parser_source_char_t *) source_p)->source_p;
    context.source_size = (lit_utf8_size_t) ((parser_source_char_t *) source_p)->source_size;
  }
  else
  {
    ecma_value_t source = ((ecma_value_t *) source_p)[0];

    JERRY_ASSERT (ecma_is_value_string (source));

    ecma_string_t *string_p = ecma_get_string_from_value (source);
    uint8_t flags = ECMA_STRING_FLAG_EMPTY;

    context.source_start_p = ecma_string_get_chars (string_p, &context.source_size, NULL, NULL, &flags);

    if (flags & ECMA_STRING_FLAG_MUST_BE_FREED)
    {
      context.global_status_flags |= ECMA_PARSE_INTERNAL_FREE_SOURCE;
    }
  }

  context.user_value = ECMA_VALUE_EMPTY;

  if ((context.global_status_flags & ECMA_PARSE_EVAL) && JERRY_CONTEXT (vm_top_context_p) != NULL)
  {
    const ecma_compiled_code_t *bytecode_header_p = JERRY_CONTEXT (vm_top_context_p)->shared_p->bytecode_header_p;

#if JERRY_SNAPSHOT_EXEC
    if (JERRY_LIKELY (!(bytecode_header_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION)))
    {
#endif /* JERRY_SNAPSHOT_EXEC */
      ecma_value_t parent_script_value = ((cbc_uint8_arguments_t *) bytecode_header_p)->script_value;
      ;
      cbc_script_t *parent_script_p = ECMA_GET_INTERNAL_VALUE_POINTER (cbc_script_t, parent_script_value);

      if (parent_script_p->refs_and_type & CBC_SCRIPT_HAS_USER_VALUE)
      {
        context.user_value = CBC_SCRIPT_GET_USER_VALUE (parent_script_p);
      }
#if JERRY_SNAPSHOT_EXEC
    }
#endif /* JERRY_SNAPSHOT_EXEC */
  }
  else if (context.options_p != NULL && (context.options_p->options & JERRY_PARSE_HAS_USER_VALUE))
  {
    context.user_value = context.options_p->user_value;
  }

  context.last_context_p = NULL;
  context.last_statement.current_p = NULL;
  context.token.flags = 0;
  lexer_init_line_info (&context);

  scanner_info_t scanner_info_end;
  scanner_info_end.next_p = NULL;
  scanner_info_end.source_p = NULL;
  scanner_info_end.type = SCANNER_TYPE_END;
  context.next_scanner_info_p = &scanner_info_end;
  context.active_scanner_info_p = NULL;
  context.skipped_scanner_info_p = NULL;
  context.skipped_scanner_info_end_p = NULL;

  context.last_cbc_opcode = PARSER_CBC_UNAVAILABLE;

  context.argument_count = 0;
  context.argument_length = UINT16_MAX;
  context.register_count = 0;
  context.literal_count = 0;

  parser_cbc_stream_init (&context.byte_code);
  context.byte_code_size = 0;
  parser_list_init (&context.literal_pool,
                    sizeof (lexer_literal_t),
                    (uint32_t) ((128 - sizeof (void *)) / sizeof (lexer_literal_t)));
  context.scope_stack_p = NULL;
  context.scope_stack_size = 0;
  context.scope_stack_top = 0;
  context.scope_stack_reg_top = 0;
  context.scope_stack_global_end = 0;
  context.tagged_template_literal_cp = JMEM_CP_NULL;
  context.private_context_p = NULL;

#ifndef JERRY_NDEBUG
  context.context_stack_depth = 0;
#endif /* !JERRY_NDEBUG */

#if JERRY_PARSER_DUMP_BYTE_CODE
  context.is_show_opcodes = (JERRY_CONTEXT (jerry_init_flags) & JERRY_INIT_SHOW_OPCODES);
  context.total_byte_code_size = 0;

  if (context.is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- %s parsing start ---\n\n", (context.arguments_start_p == NULL) ? "Script" : "Function");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  scanner_scan_all (&context);

  if (JERRY_UNLIKELY (context.error != PARSER_ERR_NO_ERROR))
  {
    JERRY_ASSERT (context.error == PARSER_ERR_OUT_OF_MEMORY);

    /* It is unlikely that memory can be allocated in an out-of-memory
     * situation. However, a simple value can still be thrown. */
    jcontext_raise_exception (ECMA_VALUE_NULL);
    return NULL;
  }

  if (context.arguments_start_p == NULL)
  {
    context.source_p = context.source_start_p;
    context.source_end_p = context.source_start_p + context.source_size;
  }
  else
  {
    context.source_p = context.arguments_start_p;
    context.source_end_p = context.arguments_start_p + context.arguments_size;
  }

  context.u.allocated_buffer_p = NULL;
  context.token.flags = 0;
  lexer_init_line_info (&context);

  parser_stack_init (&context);

  JERRY_ASSERT (context.next_scanner_info_p->source_p == context.source_p);
  JERRY_ASSERT (context.next_scanner_info_p->type == SCANNER_TYPE_FUNCTION);

  if (context.next_scanner_info_p->u8_arg & SCANNER_FUNCTION_IS_STRICT)
  {
    context.status_flags |= PARSER_IS_STRICT;
  }

  PARSER_TRY (context.try_buffer)
  {
    context.script_p = (cbc_script_t *) parser_malloc (&context, parser_script_size (&context));

    CBC_SCRIPT_SET_TYPE (context.script_p, context.user_value, CBC_SCRIPT_REF_ONE);

    if (context.global_status_flags & (ECMA_PARSE_EVAL | ECMA_PARSE_HAS_ARGUMENT_LIST_VALUE))
    {
      context.script_p->refs_and_type |= CBC_SCRIPT_IS_EVAL_CODE;
    }

#if JERRY_BUILTIN_REALMS
    context.script_p->realm_p = (ecma_object_t *) JERRY_CONTEXT (global_object_p);
#endif /* JERRY_BUILTIN_REALMS */

#if JERRY_SOURCE_NAME
    context.script_p->source_name = parser_source_name (&context);
#endif /* JERRY_SOURCE_NAME */

    ECMA_SET_INTERNAL_VALUE_POINTER (context.script_value, context.script_p);

    /* Pushing a dummy value ensures the stack is never empty.
     * This simplifies the stack management routines. */
    parser_stack_push_uint8 (&context, CBC_MAXIMUM_BYTE_VALUE);
    /* The next token must always be present to make decisions
     * in the parser. Therefore when a token is consumed, the
     * lexer_next_token() must be immediately called. */
    lexer_next_token (&context);

    if (context.arguments_start_p != NULL)
    {
      parser_parse_function_arguments (&context, LEXER_EOS);

      JERRY_ASSERT (context.next_scanner_info_p->type == SCANNER_TYPE_END_ARGUMENTS);
      scanner_release_next (&context, sizeof (scanner_info_t));

      context.source_p = context.source_start_p;
      context.source_end_p = context.source_start_p + context.source_size;
      lexer_init_line_info (&context);

      lexer_next_token (&context);
    }
#if JERRY_MODULE_SYSTEM
    else if (parse_opts & ECMA_PARSE_MODULE)
    {
      parser_branch_t branch;
      parser_emit_cbc_forward_branch (&context, CBC_JUMP_FORWARD, &branch);

      scanner_create_variables (&context, SCANNER_CREATE_VARS_IS_MODULE);
      parser_emit_cbc (&context, CBC_RETURN_FUNCTION_END);

      parser_set_branch_to_current_position (&context, &branch);
    }
#endif /* JERRY_MODULE_SYSTEM */
    else
    {
      JERRY_ASSERT (context.next_scanner_info_p->source_p == context.source_start_p
                    && context.next_scanner_info_p->type == SCANNER_TYPE_FUNCTION);

      if (scanner_is_context_needed (&context, PARSER_CHECK_GLOBAL_CONTEXT))
      {
        context.status_flags |= PARSER_LEXICAL_BLOCK_NEEDED;
      }

      if (!(parse_opts & ECMA_PARSE_EVAL))
      {
        scanner_check_variables (&context);
      }

      scanner_create_variables (&context, SCANNER_CREATE_VARS_IS_SCRIPT);
    }

    parser_parse_statements (&context);

    JERRY_ASSERT (context.last_statement.current_p == NULL);

    JERRY_ASSERT (context.last_cbc_opcode == PARSER_CBC_UNAVAILABLE);
    JERRY_ASSERT (context.u.allocated_buffer_p == NULL);

#ifndef JERRY_NDEBUG
    JERRY_ASSERT (context.status_flags & PARSER_SCANNING_SUCCESSFUL);
    JERRY_ASSERT (!(context.global_status_flags & ECMA_PARSE_INTERNAL_FOR_IN_OFF_CONTEXT_ERROR));
    context.status_flags &= (uint32_t) ~PARSER_SCANNING_SUCCESSFUL;
#endif /* !JERRY_NDEBUG */

    JERRY_ASSERT (!(context.status_flags & PARSER_HAS_LATE_LIT_INIT));

    compiled_code_p = parser_post_processing (&context);
    parser_list_free (&context.literal_pool);

    /* When parsing is successful, only the dummy value can be remained on the stack. */
    JERRY_ASSERT (context.stack_top_uint8 == CBC_MAXIMUM_BYTE_VALUE && context.stack.last_position == 1
                  && context.stack.first_p != NULL && context.stack.first_p->next_p == NULL
                  && context.stack.last_p == NULL);

    JERRY_ASSERT (context.arguments_start_p != NULL || !(context.status_flags & PARSER_ARGUMENTS_NEEDED));

    context.script_p->refs_and_type -= CBC_SCRIPT_REF_ONE;

    if (context.user_value != ECMA_VALUE_EMPTY)
    {
      CBC_SCRIPT_GET_USER_VALUE (context.script_p) = ecma_copy_value_if_not_object (context.user_value);
    }

#if JERRY_MODULE_SYSTEM
    if (context.global_status_flags & ECMA_PARSE_INTERNAL_HAS_IMPORT_META)
    {
      int idx = (context.user_value != ECMA_VALUE_EMPTY) ? 1 : 0;
      ecma_value_t module = ecma_make_object_value ((ecma_object_t *) JERRY_CONTEXT (module_current_p));

      CBC_SCRIPT_GET_OPTIONAL_VALUES (context.script_p)[idx] = module;
      context.script_p->refs_and_type |= CBC_SCRIPT_HAS_IMPORT_META;
    }
#endif /* JERRY_MODULE_SYSTEM */

#if JERRY_PARSER_DUMP_BYTE_CODE
    if (context.is_show_opcodes)
    {
      JERRY_DEBUG_MSG ("\n%s parsing successfully completed. Total byte code size: %d bytes\n",
                       (context.arguments_start_p == NULL) ? "Script" : "Function",
                       (int) context.total_byte_code_size);
    }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */
  }
  PARSER_CATCH
  {
    if (context.last_statement.current_p != NULL)
    {
      parser_free_jumps (context.last_statement);
    }

    parser_free_allocated_buffer (&context);

    scanner_cleanup (&context);

#if JERRY_MODULE_SYSTEM
    if (context.module_names_p != NULL)
    {
      ecma_module_release_module_names (context.module_names_p);
    }
#endif /* JERRY_MODULE_SYSTEM */

    compiled_code_p = NULL;
    parser_free_literals (&context.literal_pool);
    parser_cbc_stream_free (&context.byte_code);

#if JERRY_SOURCE_NAME
    ecma_deref_ecma_string (ecma_get_string_from_value (context.script_p->source_name));
#endif /* JERRY_SOURCE_NAME */

    if (context.script_p != NULL)
    {
      JERRY_ASSERT (context.script_p->refs_and_type >= CBC_SCRIPT_REF_ONE);
      jmem_heap_free_block (context.script_p, parser_script_size (&context));
    }
  }
  PARSER_TRY_END

  if (context.scope_stack_p != NULL)
  {
    parser_free (context.scope_stack_p, context.scope_stack_size * sizeof (parser_scope_stack_t));
  }

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context.is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- %s parsing end ---\n\n", (context.arguments_start_p == NULL) ? "Script" : "Function");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  parser_stack_free (&context);

  if (context.global_status_flags & ECMA_PARSE_INTERNAL_FREE_SOURCE)
  {
    jmem_heap_free_block ((void *) context.source_start_p, context.source_size);
  }

  if (context.global_status_flags & ECMA_PARSE_INTERNAL_FREE_ARG_LIST)
  {
    jmem_heap_free_block ((void *) context.arguments_start_p, context.arguments_size);
  }

  if (compiled_code_p != NULL)
  {
    return compiled_code_p;
  }

  if (context.error == PARSER_ERR_OUT_OF_MEMORY)
  {
    /* It is unlikely that memory can be allocated in an out-of-memory
     * situation. However, a simple value can still be thrown. */
    jcontext_raise_exception (ECMA_VALUE_NULL);
    return NULL;
  }
#if (JERRY_STACK_LIMIT != 0)
  if (context.error == PARSER_ERR_STACK_OVERFLOW)
  {
    ecma_raise_standard_error (JERRY_ERROR_RANGE, ECMA_ERR_MAXIMUM_CALL_STACK_SIZE_EXCEEDED);
    return NULL;
  }
#endif /* JERRY_STACK_LIMIT != 0 */

#if JERRY_ERROR_MESSAGES
  ecma_string_t *err_str_p;

  if (context.error == PARSER_ERR_INVALID_REGEXP)
  {
    ecma_value_t error = jcontext_take_exception ();
    ecma_property_t *prop_p =
      ecma_find_named_property (ecma_get_object_from_value (error), ecma_get_magic_string (LIT_MAGIC_STRING_MESSAGE));
    ecma_free_value (error);
    JERRY_ASSERT (prop_p);
    err_str_p = ecma_get_string_from_value (ECMA_PROPERTY_VALUE_PTR (prop_p)->value);
    ecma_ref_ecma_string (err_str_p);
  }
  else
  {
    err_str_p = ecma_new_ecma_external_string_from_cesu8 (parser_get_error_utf8 (context.error),
                                                          parser_get_error_size (context.error),
                                                          NULL);
  }
  ecma_value_t err_str_val = ecma_make_string_value (err_str_p);
  ecma_value_t line_str_val = ecma_make_uint32_value (context.token.line);
  ecma_value_t col_str_val = ecma_make_uint32_value (context.token.column);
  ecma_value_t source_name = parser_source_name (&context);

  ecma_raise_standard_error_with_format (JERRY_ERROR_SYNTAX,
                                         "% [%:%:%]",
                                         err_str_val,
                                         source_name,
                                         line_str_val,
                                         col_str_val);

  ecma_free_value (source_name);
  ecma_free_value (col_str_val);
  ecma_free_value (line_str_val);
  ecma_deref_ecma_string (err_str_p);
#else /* !JERRY_ERROR_MESSAGES */
  if (context.error == PARSER_ERR_INVALID_REGEXP)
  {
    jcontext_release_exception ();
  }

  ecma_raise_syntax_error (ECMA_ERR_EMPTY);
#endif /* JERRY_ERROR_MESSAGES */

  return NULL;
} /* parser_parse_source */

/**
 * Save parser context before function parsing.
 */
static void
parser_save_context (parser_context_t *context_p, /**< context */
                     parser_saved_context_t *saved_context_p) /**< target for saving the context */
{
  JERRY_ASSERT (context_p->last_cbc_opcode == PARSER_CBC_UNAVAILABLE);

  if (context_p->status_flags & PARSER_FUNCTION_IS_PARSING_ARGS)
  {
    context_p->status_flags |= PARSER_LEXICAL_BLOCK_NEEDED;
  }

  /* Save private part of the context. */

  saved_context_p->status_flags = context_p->status_flags;
  saved_context_p->stack_depth = context_p->stack_depth;
  saved_context_p->stack_limit = context_p->stack_limit;
  saved_context_p->prev_context_p = context_p->last_context_p;
  saved_context_p->last_statement = context_p->last_statement;

  saved_context_p->argument_count = context_p->argument_count;
  saved_context_p->argument_length = context_p->argument_length;
  saved_context_p->register_count = context_p->register_count;
  saved_context_p->literal_count = context_p->literal_count;

  saved_context_p->byte_code = context_p->byte_code;
  saved_context_p->byte_code_size = context_p->byte_code_size;
  saved_context_p->literal_pool_data = context_p->literal_pool.data;
  saved_context_p->scope_stack_p = context_p->scope_stack_p;
  saved_context_p->scope_stack_size = context_p->scope_stack_size;
  saved_context_p->scope_stack_top = context_p->scope_stack_top;
  saved_context_p->scope_stack_reg_top = context_p->scope_stack_reg_top;
  saved_context_p->scope_stack_global_end = context_p->scope_stack_global_end;
  saved_context_p->tagged_template_literal_cp = context_p->tagged_template_literal_cp;

#ifndef JERRY_NDEBUG
  saved_context_p->context_stack_depth = context_p->context_stack_depth;
#endif /* !JERRY_NDEBUG */

  /* Reset private part of the context. */

  context_p->status_flags &= PARSER_IS_STRICT;
  context_p->stack_depth = 0;
  context_p->stack_limit = 0;
  context_p->last_context_p = saved_context_p;
  context_p->last_statement.current_p = NULL;

  context_p->argument_count = 0;
  context_p->argument_length = UINT16_MAX;
  context_p->register_count = 0;
  context_p->literal_count = 0;

  parser_cbc_stream_init (&context_p->byte_code);
  context_p->byte_code_size = 0;
  parser_list_reset (&context_p->literal_pool);
  context_p->scope_stack_p = NULL;
  context_p->scope_stack_size = 0;
  context_p->scope_stack_top = 0;
  context_p->scope_stack_reg_top = 0;
  context_p->scope_stack_global_end = 0;
  context_p->tagged_template_literal_cp = JMEM_CP_NULL;

#ifndef JERRY_NDEBUG
  context_p->context_stack_depth = 0;
#endif /* !JERRY_NDEBUG */

} /* parser_save_context */

/**
 * Restore parser context after function parsing.
 */
static void
parser_restore_context (parser_context_t *context_p, /**< context */
                        parser_saved_context_t *saved_context_p) /**< target for saving the context */
{
  parser_list_free (&context_p->literal_pool);

  if (context_p->scope_stack_p != NULL)
  {
    parser_free (context_p->scope_stack_p, context_p->scope_stack_size * sizeof (parser_scope_stack_t));
  }

  /* Restore private part of the context. */

  JERRY_ASSERT (context_p->last_cbc_opcode == PARSER_CBC_UNAVAILABLE);

  context_p->status_flags = saved_context_p->status_flags;
  context_p->stack_depth = saved_context_p->stack_depth;
  context_p->stack_limit = saved_context_p->stack_limit;
  context_p->last_context_p = saved_context_p->prev_context_p;
  context_p->last_statement = saved_context_p->last_statement;

  context_p->argument_count = saved_context_p->argument_count;
  context_p->argument_length = saved_context_p->argument_length;
  context_p->register_count = saved_context_p->register_count;
  context_p->literal_count = saved_context_p->literal_count;

  context_p->byte_code = saved_context_p->byte_code;
  context_p->byte_code_size = saved_context_p->byte_code_size;
  context_p->literal_pool.data = saved_context_p->literal_pool_data;
  context_p->scope_stack_p = saved_context_p->scope_stack_p;
  context_p->scope_stack_size = saved_context_p->scope_stack_size;
  context_p->scope_stack_top = saved_context_p->scope_stack_top;
  context_p->scope_stack_reg_top = saved_context_p->scope_stack_reg_top;
  context_p->scope_stack_global_end = saved_context_p->scope_stack_global_end;
  context_p->tagged_template_literal_cp = saved_context_p->tagged_template_literal_cp;

#ifndef JERRY_NDEBUG
  context_p->context_stack_depth = saved_context_p->context_stack_depth;
#endif /* !JERRY_NDEBUG */

} /* parser_restore_context */

/**
 * Parse function code
 *
 * @return compiled code
 */
ecma_compiled_code_t *
parser_parse_function (parser_context_t *context_p, /**< context */
                       uint32_t status_flags) /**< extra status flags */
{
  parser_saved_context_t saved_context;
  ecma_compiled_code_t *compiled_code_p;

  JERRY_ASSERT (status_flags & PARSER_IS_FUNCTION);
  parser_save_context (context_p, &saved_context);
  context_p->status_flags |= status_flags;
  context_p->status_flags |= PARSER_ALLOW_NEW_TARGET;

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- %s parsing start ---\n\n",
                     (context_p->status_flags & PARSER_CLASS_CONSTRUCTOR) ? "Class constructor" : "Function");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  lexer_next_token (context_p);

  if (context_p->token.type != LEXER_LEFT_PAREN)
  {
    parser_raise_error (context_p, PARSER_ERR_ARGUMENT_LIST_EXPECTED);
  }

  lexer_next_token (context_p);

  parser_parse_function_arguments (context_p, LEXER_RIGHT_PAREN);
  lexer_next_token (context_p);

  if ((context_p->status_flags & PARSER_IS_PROPERTY_GETTER) && context_p->argument_count != 0)
  {
    parser_raise_error (context_p, PARSER_ERR_NO_ARGUMENTS_EXPECTED);
  }

  if ((context_p->status_flags & PARSER_IS_PROPERTY_SETTER) && context_p->argument_count != 1)
  {
    parser_raise_error (context_p, PARSER_ERR_ONE_ARGUMENT_EXPECTED);
  }

  if ((context_p->status_flags & (PARSER_CLASS_CONSTRUCTOR | PARSER_ALLOW_SUPER_CALL)) == PARSER_CLASS_CONSTRUCTOR)
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_RUN_FIELD_INIT);
    parser_flush_cbc (context_p);
  }

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes && (context_p->status_flags & PARSER_HAS_NON_STRICT_ARG))
  {
    JERRY_DEBUG_MSG ("  Note: legacy (non-strict) argument definition\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  if (context_p->token.type != LEXER_LEFT_BRACE)
  {
    parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_EXPECTED);
  }

  lexer_next_token (context_p);
  parser_parse_statements (context_p);
  compiled_code_p = parser_post_processing (context_p);

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- %s parsing end ---\n\n",
                     (context_p->status_flags & PARSER_CLASS_CONSTRUCTOR) ? "Class constructor" : "Function");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  parser_restore_context (context_p, &saved_context);

  return compiled_code_p;
} /* parser_parse_function */

/**
 * Parse static class block code
 *
 * @return compiled code
 */
ecma_compiled_code_t *
parser_parse_class_static_block (parser_context_t *context_p) /**< context */
{
  parser_saved_context_t saved_context;
  ecma_compiled_code_t *compiled_code_p;

  parser_save_context (context_p, &saved_context);
  context_p->status_flags |= (PARSER_IS_CLASS_STATIC_BLOCK | PARSER_FUNCTION_CLOSURE | PARSER_ALLOW_SUPER
                              | PARSER_INSIDE_CLASS_FIELD | PARSER_ALLOW_NEW_TARGET | PARSER_DISALLOW_AWAIT_YIELD);

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- Static class block parsing start ---\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  scanner_create_variables (context_p, SCANNER_CREATE_VARS_NO_OPTS);
  lexer_next_token (context_p);

  parser_parse_statements (context_p);
  compiled_code_p = parser_post_processing (context_p);

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- Static class block parsing end ---\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  parser_restore_context (context_p, &saved_context);

  return compiled_code_p;
} /* parser_parse_class_static_block */

/**
 * Parse arrow function code
 *
 * @return compiled code
 */
ecma_compiled_code_t *
parser_parse_arrow_function (parser_context_t *context_p, /**< context */
                             uint32_t status_flags) /**< extra status flags */
{
  parser_saved_context_t saved_context;
  ecma_compiled_code_t *compiled_code_p;

  JERRY_ASSERT (status_flags & PARSER_IS_FUNCTION);
  JERRY_ASSERT (status_flags & PARSER_IS_ARROW_FUNCTION);
  parser_save_context (context_p, &saved_context);
  context_p->status_flags |= status_flags;
  context_p->status_flags |=
    saved_context.status_flags & (PARSER_ALLOW_NEW_TARGET | PARSER_ALLOW_SUPER | PARSER_ALLOW_SUPER_CALL);

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- Arrow function parsing start ---\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  /* The `await` keyword is disallowed in the IdentifierReference position */
  if (status_flags & PARSER_IS_CLASS_STATIC_BLOCK)
  {
    context_p->status_flags |= PARSER_DISALLOW_AWAIT_YIELD;
  }

  if (context_p->token.type == LEXER_LEFT_PAREN)
  {
    lexer_next_token (context_p);
    parser_parse_function_arguments (context_p, LEXER_RIGHT_PAREN);
    lexer_next_token (context_p);
  }
  else
  {
    parser_parse_function_arguments (context_p, LEXER_ARROW);
  }

  /* The `await` keyword is interpreted as an identifier within the body of arrow functions */
  if (status_flags & PARSER_IS_CLASS_STATIC_BLOCK)
  {
    context_p->status_flags &= (uint32_t) ~(PARSER_DISALLOW_AWAIT_YIELD | PARSER_IS_CLASS_STATIC_BLOCK);
  }

  JERRY_ASSERT (context_p->token.type == LEXER_ARROW);

  lexer_next_token (context_p);
  bool next_token_needed = false;

  if (context_p->token.type == LEXER_LEFT_BRACE)
  {
    lexer_next_token (context_p);

    context_p->status_flags |= PARSER_IS_CLOSURE;
    parser_parse_statements (context_p);

    /* Unlike normal function, arrow functions consume their close brace. */
    JERRY_ASSERT (context_p->token.type == LEXER_RIGHT_BRACE);
    next_token_needed = true;
  }
  else
  {
    if (context_p->status_flags & PARSER_IS_STRICT && context_p->status_flags & PARSER_HAS_NON_STRICT_ARG)
    {
      parser_raise_error (context_p, PARSER_ERR_NON_STRICT_ARG_DEFINITION);
    }

    parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

    if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
    {
      context_p->last_cbc_opcode = CBC_RETURN_WITH_LITERAL;
    }
    else
    {
      parser_emit_cbc (context_p, CBC_RETURN);
    }
    parser_flush_cbc (context_p);

    lexer_update_await_yield (context_p, saved_context.status_flags);
  }

  compiled_code_p = parser_post_processing (context_p);

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- Arrow function parsing end ---\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  parser_restore_context (context_p, &saved_context);

  if (next_token_needed)
  {
    lexer_next_token (context_p);
  }

  return compiled_code_p;
} /* parser_parse_arrow_function */

/**
 * Parse class fields
 *
 * @return compiled code
 */
ecma_compiled_code_t *
parser_parse_class_fields (parser_context_t *context_p) /**< context */
{
  parser_saved_context_t saved_context;
  ecma_compiled_code_t *compiled_code_p;

  uint32_t extra_status_flags = context_p->status_flags & PARSER_INSIDE_WITH;

  parser_save_context (context_p, &saved_context);
  context_p->status_flags |= (PARSER_IS_FUNCTION | PARSER_ALLOW_SUPER | PARSER_INSIDE_CLASS_FIELD
                              | PARSER_ALLOW_NEW_TARGET | extra_status_flags);

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- Class fields parsing start ---\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  const uint8_t *source_end_p = context_p->source_end_p;
  bool first_computed_class_field = true;
  scanner_location_t end_location;
  scanner_get_location (&end_location, context_p);

  do
  {
    uint8_t class_field_type = context_p->stack_top_uint8;
    parser_stack_pop_uint8 (context_p);

    scanner_range_t range = { 0 };

    if (class_field_type & PARSER_CLASS_FIELD_INITIALIZED)
    {
      parser_stack_pop (context_p, &range, sizeof (scanner_range_t));
    }
    else if (class_field_type & PARSER_CLASS_FIELD_NORMAL)
    {
      parser_stack_pop (context_p, &range.start_location, sizeof (scanner_location_t));
    }

    uint16_t literal_index = 0;
    bool is_private = false;

    if (class_field_type & PARSER_CLASS_FIELD_NORMAL)
    {
      scanner_set_location (context_p, &range.start_location);

      if (class_field_type & PARSER_CLASS_FIELD_STATIC_BLOCK)
      {
        scanner_seek (context_p);
        JERRY_ASSERT (context_p->source_p[1] == LIT_CHAR_LEFT_BRACE);
        context_p->source_p += 2;
        context_p->source_end_p = source_end_p;

        uint16_t func_index = lexer_construct_class_static_block_function (context_p);

        parser_emit_cbc_ext_literal (context_p, CBC_EXT_CLASS_CALL_STATIC_BLOCK, func_index);
        continue;
      }

      uint32_t ident_opts = LEXER_OBJ_IDENT_ONLY_IDENTIFIERS;
      is_private = context_p->source_p[-1] == LIT_CHAR_HASHMARK;

      if (is_private)
      {
        ident_opts |= LEXER_OBJ_IDENT_CLASS_PRIVATE;
      }

      context_p->source_end_p = source_end_p;
      scanner_seek (context_p);

      lexer_expect_object_literal_id (context_p, ident_opts);

      literal_index = context_p->lit_object.index;

      if (class_field_type & PARSER_CLASS_FIELD_INITIALIZED)
      {
        lexer_next_token (context_p);
        JERRY_ASSERT (context_p->token.type == LEXER_ASSIGN);
      }
    }
    else if (first_computed_class_field)
    {
      parser_emit_cbc (context_p, CBC_PUSH_NUMBER_0);
      first_computed_class_field = false;
    }

    if (class_field_type & PARSER_CLASS_FIELD_INITIALIZED)
    {
      if (!(class_field_type & PARSER_CLASS_FIELD_NORMAL))
      {
        scanner_set_location (context_p, &range.start_location);
        scanner_seek (context_p);
      }

      context_p->source_end_p = range.source_end_p;
      lexer_next_token (context_p);

      parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

      if (context_p->token.type != LEXER_EOS)
      {
        parser_raise_error (context_p, PARSER_ERR_SEMICOLON_EXPECTED);
      }
    }
    else
    {
      parser_emit_cbc (context_p, CBC_PUSH_UNDEFINED);
    }

    if (class_field_type & PARSER_CLASS_FIELD_NORMAL)
    {
      uint16_t function_literal_index = parser_check_anonymous_function_declaration (context_p);

      if (function_literal_index == PARSER_ANONYMOUS_CLASS)
      {
        parser_emit_cbc_ext_literal (context_p, CBC_EXT_SET_CLASS_NAME, literal_index);
      }
      else if (function_literal_index < PARSER_NAMED_FUNCTION)
      {
        uint32_t function_name_status_flags = is_private ? PARSER_PRIVATE_FUNCTION_NAME : 0;
        parser_set_function_name (context_p, function_literal_index, literal_index, function_name_status_flags);
      }

      if (is_private)
      {
        parser_emit_cbc_ext_literal (context_p, CBC_EXT_PRIVATE_FIELD_ADD, literal_index);
      }
      else
      {
        parser_emit_cbc_ext_literal (context_p, CBC_EXT_DEFINE_FIELD, literal_index);
      }

      /* Prepare stack slot for assignment property reference base. Needed by vm.c */
      if (context_p->stack_limit == context_p->stack_depth)
      {
        context_p->stack_limit++;
        JERRY_ASSERT (context_p->stack_limit <= PARSER_MAXIMUM_STACK_LIMIT);
      }
    }
    else
    {
      uint16_t function_literal_index = parser_check_anonymous_function_declaration (context_p);
      uint16_t opcode = CBC_EXT_SET_NEXT_COMPUTED_FIELD;

      if (function_literal_index < PARSER_NAMED_FUNCTION || function_literal_index == PARSER_ANONYMOUS_CLASS)
      {
        opcode = CBC_EXT_SET_NEXT_COMPUTED_FIELD_ANONYMOUS_FUNC;
      }

      parser_flush_cbc (context_p);

      /* The next opcode pushes two more temporary values onto the stack */
      if (context_p->stack_depth + 1 > context_p->stack_limit)
      {
        context_p->stack_limit = (uint16_t) (context_p->stack_depth + 1);
        if (context_p->stack_limit > PARSER_MAXIMUM_STACK_LIMIT)
        {
          parser_raise_error (context_p, PARSER_ERR_STACK_LIMIT_REACHED);
        }
      }

      parser_emit_cbc_ext (context_p, opcode);
    }
  } while (!(context_p->stack_top_uint8 & PARSER_CLASS_FIELD_END));

  if (!first_computed_class_field)
  {
    parser_emit_cbc (context_p, CBC_POP);
  }

  parser_flush_cbc (context_p);
  context_p->source_end_p = source_end_p;
  scanner_set_location (context_p, &end_location);

  compiled_code_p = parser_post_processing (context_p);

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- Class fields parsing end ---\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  parser_restore_context (context_p, &saved_context);

  return compiled_code_p;
} /* parser_parse_class_fields */

/**
 * Check whether the last emitted cbc opcode was an anonymous function declaration
 *
 * @return PARSER_NOT_FUNCTION_LITERAL - if the last opcode is not a function literal
 *         PARSER_NAMED_FUNCTION - if the last opcode is not a named function declaration
 *         PARSER_ANONYMOUS_CLASS - if the last opcode is an anonymous class declaration
 *         literal index of the anonymous function literal - otherwise
 */
uint16_t
parser_check_anonymous_function_declaration (parser_context_t *context_p) /**< context */
{
  if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_FINALIZE_ANONYMOUS_CLASS))
  {
    return PARSER_ANONYMOUS_CLASS;
  }

  if (context_p->last_cbc.literal_type != LEXER_FUNCTION_LITERAL)
  {
    return PARSER_NOT_FUNCTION_LITERAL;
  }

  uint16_t literal_index = PARSER_NOT_FUNCTION_LITERAL;

  if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
  {
    literal_index = context_p->last_cbc.literal_index;
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_TWO_LITERALS)
  {
    literal_index = context_p->last_cbc.value;
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_THREE_LITERALS)
  {
    literal_index = context_p->last_cbc.third_literal_index;
  }
  else
  {
    return PARSER_NOT_FUNCTION_LITERAL;
  }

  const ecma_compiled_code_t *bytecode_p;
  bytecode_p = (const ecma_compiled_code_t *) (PARSER_GET_LITERAL (literal_index)->u.bytecode_p);
  bool is_anon =
    ecma_is_value_magic_string (*ecma_compiled_code_resolve_function_name (bytecode_p), LIT_MAGIC_STRING__EMPTY);

  return (is_anon ? literal_index : PARSER_NAMED_FUNCTION);
} /* parser_check_anonymous_function_declaration */

/**
 * Set the function name of the function literal corresponds to the given function literal index
 * to the given character buffer of literal corresponds to the given name index.
 */
void
parser_set_function_name (parser_context_t *context_p, /**< context */
                          uint16_t function_literal_index, /**< function literal index */
                          uint16_t name_index, /**< function name literal index */
                          uint32_t status_flags) /**< status flags */
{
  ecma_compiled_code_t *bytecode_p;
  bytecode_p = (ecma_compiled_code_t *) (PARSER_GET_LITERAL (function_literal_index)->u.bytecode_p);

  parser_compiled_code_set_function_name (context_p, bytecode_p, name_index, status_flags);
} /* parser_set_function_name */

/**
 * Prepend the given prefix into the current function name literal
 *
 * @return pointer to the newly allocated buffer
 */
static uint8_t *
parser_add_function_name_prefix (parser_context_t *context_p, /**< context */
                                 const char *prefix_p, /**< prefix */
                                 uint32_t prefix_size, /**< prefix's length */
                                 uint32_t *name_length_p, /**< [out] function name's size */
                                 lexer_literal_t *name_lit_p) /**< function name literal */
{
  *name_length_p += prefix_size;
  uint8_t *name_buffer_p = (uint8_t *) parser_malloc (context_p, *name_length_p * sizeof (uint8_t));
  memcpy (name_buffer_p, prefix_p, prefix_size);
  memcpy (name_buffer_p + prefix_size, name_lit_p->u.char_p, name_lit_p->prop.length);

  return name_buffer_p;
} /* parser_add_function_name_prefix */

/**
 * Set the function name of the given compiled code
 * to the given character buffer of literal corresponds to the given name index.
 */
void
parser_compiled_code_set_function_name (parser_context_t *context_p, /**< context */
                                        ecma_compiled_code_t *bytecode_p, /**< function literal index */
                                        uint16_t name_index, /**< function name literal index */
                                        uint32_t status_flags) /**< status flags */
{
  ecma_value_t *func_name_start_p;
  func_name_start_p = ecma_compiled_code_resolve_function_name ((const ecma_compiled_code_t *) bytecode_p);

  if (JERRY_UNLIKELY (!ecma_is_value_magic_string (*func_name_start_p, LIT_MAGIC_STRING__EMPTY)))
  {
    return;
  }

  parser_scope_stack_t *scope_stack_start_p = context_p->scope_stack_p;
  parser_scope_stack_t *scope_stack_p = scope_stack_start_p + context_p->scope_stack_top;

  while (scope_stack_p > scope_stack_start_p)
  {
    scope_stack_p--;

    if (scope_stack_p->map_from != PARSER_SCOPE_STACK_FUNC && scanner_decode_map_to (scope_stack_p) == name_index)
    {
      name_index = scope_stack_p->map_from;
      break;
    }
  }

  lexer_literal_t *name_lit_p = (lexer_literal_t *) PARSER_GET_LITERAL (name_index);

  if (name_lit_p->type != LEXER_IDENT_LITERAL && name_lit_p->type != LEXER_STRING_LITERAL)
  {
    return;
  }

  uint8_t *name_buffer_p = (uint8_t *) name_lit_p->u.char_p;
  uint32_t name_length = name_lit_p->prop.length;

  if (status_flags & PARSER_PRIVATE_FUNCTION_NAME)
  {
    name_buffer_p = parser_add_function_name_prefix (context_p, "#", 1, &name_length, name_lit_p);
  }
  else if (status_flags & (PARSER_IS_PROPERTY_GETTER | PARSER_IS_PROPERTY_SETTER))
  {
    name_buffer_p = parser_add_function_name_prefix (context_p,
                                                     (status_flags & PARSER_IS_PROPERTY_GETTER) ? "get " : "set ",
                                                     4,
                                                     &name_length,
                                                     name_lit_p);
  }

  *func_name_start_p =
    ecma_find_or_create_literal_string (name_buffer_p, name_length, (status_flags & LEXER_FLAG_ASCII) != 0);

  if (name_buffer_p != name_lit_p->u.char_p)
  {
    parser_free (name_buffer_p, name_length);
  }
} /* parser_compiled_code_set_function_name */

/**
 * Raise a parse error.
 */
void
parser_raise_error (parser_context_t *context_p, /**< context */
                    parser_error_msg_t error) /**< error code */
{
  /* Must be compatible with the scanner because
   * the lexer might throws errors during prescanning. */
  parser_saved_context_t *saved_context_p = context_p->last_context_p;

  while (saved_context_p != NULL)
  {
    parser_cbc_stream_free (&saved_context_p->byte_code);

    /* First the current literal pool is freed, and then it is replaced
     * by the literal pool coming from the saved context. Since literals
     * are not used anymore, this is a valid replacement. The last pool
     * is freed by parser_parse_source. */

    parser_free_literals (&context_p->literal_pool);
    context_p->literal_pool.data = saved_context_p->literal_pool_data;

    if (context_p->scope_stack_p != NULL)
    {
      parser_free (context_p->scope_stack_p, context_p->scope_stack_size * sizeof (parser_scope_stack_t));
    }
    context_p->scope_stack_p = saved_context_p->scope_stack_p;
    context_p->scope_stack_size = saved_context_p->scope_stack_size;

    if (saved_context_p->last_statement.current_p != NULL)
    {
      parser_free_jumps (saved_context_p->last_statement);
    }

    if (saved_context_p->tagged_template_literal_cp != JMEM_CP_NULL)
    {
      ecma_collection_t *collection =
        ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, saved_context_p->tagged_template_literal_cp);
      ecma_collection_free_template_literal (collection);
    }

    saved_context_p = saved_context_p->prev_context_p;
  }

  parser_free_private_fields (context_p);

  if (context_p->tagged_template_literal_cp != JMEM_CP_NULL)
  {
    ecma_collection_t *collection =
      ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, context_p->tagged_template_literal_cp);
    ecma_collection_free_template_literal (collection);
  }

  context_p->error = error;
  PARSER_THROW (context_p->try_buffer);
  /* Should never been reached. */
  JERRY_ASSERT (0);
} /* parser_raise_error */

#endif /* JERRY_PARSER */

/**
 * Parse EcmaScript source code
 *
 * Note:
 *      if arg_list_p is not NULL, a function body is parsed
 *      returned value must be freed with ecma_free_value
 *
 * @return pointer to compiled byte code - if success
 *         NULL - otherwise
 */
ecma_compiled_code_t *
parser_parse_script (void *source_p, /**< source code */
                     uint32_t parse_opts, /**< ecma_parse_opts_t option bits */
                     const jerry_parse_options_t *options_p) /**< additional configuration options */
{
#if JERRY_PARSER
  ecma_compiled_code_t *bytecode_p = parser_parse_source (source_p, parse_opts, options_p);

  if (JERRY_UNLIKELY (bytecode_p == NULL))
  {
    /* Exception has already thrown. */
    return NULL;
  }

  return bytecode_p;
#else /* !JERRY_PARSER */
  JERRY_UNUSED (arg_list_p);
  JERRY_UNUSED (arg_list_size);
  JERRY_UNUSED (source_p);
  JERRY_UNUSED (source_size);
  JERRY_UNUSED (parse_opts);
  JERRY_UNUSED (source_name);

  ecma_raise_syntax_error (ECMA_ERR_PARSER_NOT_SUPPORTED);
  return NULL;
#endif /* JERRY_PARSER */
} /* parser_parse_script */

/**
 * @}
 * @}
 * @}
 */
