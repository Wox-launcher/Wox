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

#include "re-parser.h"

#include "ecma-exceptions.h"
#include "ecma-globals.h"

#include "jcontext.h"
#include "jrt-libc-includes.h"
#include "lit-char-helpers.h"
#include "re-compiler.h"

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
 * Get the start opcode for the current group.
 *
 * @return RegExp opcode
 */
static re_opcode_t
re_get_group_start_opcode (bool is_capturing) /**< is capturing group */
{
  return (is_capturing) ? RE_OP_CAPTURING_GROUP_START : RE_OP_NON_CAPTURING_GROUP_START;
} /* re_get_group_start_opcode */

/**
 * Get the end opcode for the current group.
 *
 * @return RegExp opcode
 */
static re_opcode_t
re_get_group_end_opcode (re_compiler_ctx_t *re_ctx_p, /**< RegExp compiler context */
                         bool is_capturing) /**< is capturing group */
{
  if (is_capturing)
  {
    if (re_ctx_p->token.greedy)
    {
      return RE_OP_GREEDY_CAPTURING_GROUP_END;
    }

    return RE_OP_LAZY_CAPTURING_GROUP_END;
  }

  if (re_ctx_p->token.greedy)
  {
    return RE_OP_GREEDY_NON_CAPTURING_GROUP_END;
  }

  return RE_OP_LAZY_NON_CAPTURING_GROUP_END;
} /* re_get_group_end_opcode */

/**
 * Enclose the given bytecode to a group.
 */
static void
re_insert_into_group (re_compiler_ctx_t *re_ctx_p, /**< RegExp compiler context */
                      uint32_t group_start_offset, /**< offset of group start */
                      uint32_t idx, /**< index of group */
                      uint32_t capture_start, /**< index of first nested capture */
                      bool is_capturing) /**< is capturing group */
{
  uint32_t qmin = re_ctx_p->token.qmin;
  uint32_t qmax = re_ctx_p->token.qmax;

  if (JERRY_UNLIKELY (!is_capturing && re_bytecode_size (re_ctx_p) == group_start_offset))
  {
    return;
  }

  if (qmin == 0)
  {
    re_insert_value (re_ctx_p, group_start_offset, re_bytecode_size (re_ctx_p) - group_start_offset);
  }

  re_insert_value (re_ctx_p, group_start_offset, qmin);
  re_insert_value (re_ctx_p, group_start_offset, re_ctx_p->captures_count - capture_start);

  if (!is_capturing)
  {
    re_insert_value (re_ctx_p, group_start_offset, capture_start);
  }
  else
  {
    JERRY_ASSERT (idx == capture_start);
  }

  re_insert_value (re_ctx_p, group_start_offset, idx);
  re_insert_opcode (re_ctx_p, group_start_offset, re_get_group_start_opcode (is_capturing));

  re_append_opcode (re_ctx_p, re_get_group_end_opcode (re_ctx_p, is_capturing));
  re_append_value (re_ctx_p, idx);
  re_append_value (re_ctx_p, qmin);
  re_append_value (re_ctx_p, qmax + RE_QMAX_OFFSET);
} /* re_insert_into_group */

/**
 * Insert simple atom iterator.
 */
static void
re_insert_atom_iterator (re_compiler_ctx_t *re_ctx_p, /**< RegExp compiler context */
                         uint32_t start_offset) /**< atom start offset */
{
  const uint32_t qmin = re_ctx_p->token.qmin;
  const uint32_t qmax = re_ctx_p->token.qmax;

  if (qmin == 1 && qmax == 1)
  {
    return;
  }

  re_append_opcode (re_ctx_p, RE_OP_ITERATOR_END);
  re_insert_value (re_ctx_p, start_offset, re_bytecode_size (re_ctx_p) - start_offset);
  re_insert_value (re_ctx_p, start_offset, qmax + RE_QMAX_OFFSET);
  re_insert_value (re_ctx_p, start_offset, qmin);
  re_insert_opcode (re_ctx_p, start_offset, re_ctx_p->token.greedy ? RE_OP_GREEDY_ITERATOR : RE_OP_LAZY_ITERATOR);
} /* re_insert_atom_iterator */

/**
 * Insert a lookahead assertion.
 */
static void
re_insert_assertion_lookahead (re_compiler_ctx_t *re_ctx_p, /**< RegExp compiler context */
                               uint32_t start_offset, /**< atom start offset */
                               uint32_t capture_start, /**< index of first nested capture */
                               bool negative) /** lookahead type */
{
  const uint32_t qmin = re_ctx_p->token.qmin;

  re_append_opcode (re_ctx_p, RE_OP_ASSERT_END);
  re_insert_value (re_ctx_p, start_offset, re_bytecode_size (re_ctx_p) - start_offset);

  /* We need to clear nested capturing group results when a negative assertion or the tail after a positive assertion
   * does not match, so we store the begin and end index of nested capturing groups. */
  re_insert_value (re_ctx_p, start_offset, re_ctx_p->captures_count - capture_start);
  re_insert_value (re_ctx_p, start_offset, capture_start);

  /* Lookaheads always result in zero length matches, which means iterations will always stop on the first match.
   * This allows us to not have to deal with iterations beyond one. Either qmin == 0 which will implicitly match,
   * or qmin > 0, in which case the first iteration will decide whether the assertion matches depending on whether
   * the iteration matched or not. This also allows us to ignore qmax entirely. */
  re_insert_byte (re_ctx_p, start_offset, (uint8_t) JERRY_MIN (qmin, 1));

  const re_opcode_t opcode = (negative) ? RE_OP_ASSERT_LOOKAHEAD_NEG : RE_OP_ASSERT_LOOKAHEAD_POS;
  re_insert_opcode (re_ctx_p, start_offset, opcode);
} /* re_insert_assertion_lookahead */

/**
 * Consume non greedy (question mark) character if present.
 */
static void
re_parse_lazy_char (re_compiler_ctx_t *re_ctx_p) /**< RegExp parser context */
{
  if (re_ctx_p->input_curr_p < re_ctx_p->input_end_p && *re_ctx_p->input_curr_p == LIT_CHAR_QUESTION)
  {
    re_ctx_p->input_curr_p++;
    re_ctx_p->token.greedy = false;
    return;
  }

  re_ctx_p->token.greedy = true;
} /* re_parse_lazy_char */

/**
 * Parse a max 3 digit long octal number from the input string, with a decimal value less than 256.
 *
 * @return value of the octal number
 */
static uint32_t
re_parse_octal (re_compiler_ctx_t *re_ctx_p) /**< RegExp parser context */
{
  JERRY_ASSERT (re_ctx_p->input_curr_p < re_ctx_p->input_end_p);
  JERRY_ASSERT (lit_char_is_octal_digit (*re_ctx_p->input_curr_p));

  uint32_t value = (uint32_t) (*re_ctx_p->input_curr_p++) - LIT_CHAR_0;

  if (re_ctx_p->input_curr_p < re_ctx_p->input_end_p && lit_char_is_octal_digit (*re_ctx_p->input_curr_p))
  {
    value = value * 8 + (*re_ctx_p->input_curr_p++) - LIT_CHAR_0;
  }

  if (re_ctx_p->input_curr_p < re_ctx_p->input_end_p && lit_char_is_octal_digit (*re_ctx_p->input_curr_p))
  {
    const uint32_t new_value = value * 8 + (*re_ctx_p->input_curr_p) - LIT_CHAR_0;

    if (new_value <= RE_MAX_OCTAL_VALUE)
    {
      value = new_value;
      re_ctx_p->input_curr_p++;
    }
  }

  return value;
} /* re_parse_octal */

/**
 * Check that the currently parsed quantifier is valid.
 *
 * @return ECMA_VALUE_ERROR, if quantifier is invalid
 *         ECMA_VALUE_EMPTY, otherwise
 */
static ecma_value_t
re_check_quantifier (re_compiler_ctx_t *re_ctx_p)
{
  if (re_ctx_p->token.qmin > re_ctx_p->token.qmax)
  {
    /* ECMA-262 v5.1 15.10.2.5 */
    return ecma_raise_syntax_error (ECMA_ERR_MIN_GREATER_THAN_MAX);
  }

  return ECMA_VALUE_EMPTY;
} /* re_check_quantifier */

/**
 * Parse RegExp quantifier.
 *
 * @return ECMA_VALUE_TRUE - if parsed successfully
 *         ECMA_VALUE_FALSE - otherwise
 */
static ecma_value_t
re_parse_quantifier (re_compiler_ctx_t *re_ctx_p) /**< RegExp compiler context */
{
  if (re_ctx_p->input_curr_p < re_ctx_p->input_end_p)
  {
    switch (*re_ctx_p->input_curr_p)
    {
      case LIT_CHAR_QUESTION:
      {
        re_ctx_p->input_curr_p++;
        re_ctx_p->token.qmin = 0;
        re_ctx_p->token.qmax = 1;

        re_parse_lazy_char (re_ctx_p);
        return ECMA_VALUE_TRUE;
      }
      case LIT_CHAR_ASTERISK:
      {
        re_ctx_p->input_curr_p++;
        re_ctx_p->token.qmin = 0;
        re_ctx_p->token.qmax = RE_INFINITY;

        re_parse_lazy_char (re_ctx_p);
        return ECMA_VALUE_TRUE;
      }
      case LIT_CHAR_PLUS:
      {
        re_ctx_p->input_curr_p++;
        re_ctx_p->token.qmin = 1;
        re_ctx_p->token.qmax = RE_INFINITY;

        re_parse_lazy_char (re_ctx_p);
        return ECMA_VALUE_TRUE;
      }
      case LIT_CHAR_LEFT_BRACE:
      {
        const lit_utf8_byte_t *current_p = re_ctx_p->input_curr_p + 1;
        uint32_t qmin = 0;
        uint32_t qmax = RE_INFINITY;

        if (current_p >= re_ctx_p->input_end_p)
        {
          break;
        }

        if (!lit_char_is_decimal_digit (*current_p))
        {
          break;
        }

        qmin = lit_parse_decimal (&current_p, re_ctx_p->input_end_p);

        if (current_p >= re_ctx_p->input_end_p)
        {
          break;
        }

        lit_utf8_byte_t ch = *current_p++;
        if (ch == LIT_CHAR_RIGHT_BRACE)
        {
          qmax = qmin;
        }
        else if (ch == LIT_CHAR_COMMA)
        {
          if (current_p >= re_ctx_p->input_end_p)
          {
            break;
          }

          if (lit_char_is_decimal_digit (*current_p))
          {
            qmax = lit_parse_decimal (&current_p, re_ctx_p->input_end_p);
          }

          if (current_p >= re_ctx_p->input_end_p || *current_p++ != LIT_CHAR_RIGHT_BRACE)
          {
            break;
          }
        }
        else
        {
          break;
        }

        re_ctx_p->token.qmin = qmin;
        re_ctx_p->token.qmax = qmax;
        re_ctx_p->input_curr_p = current_p;
        re_parse_lazy_char (re_ctx_p);
        return ECMA_VALUE_TRUE;
      }
      default:
      {
        break;
      }
    }
  }

  re_ctx_p->token.qmin = 1;
  re_ctx_p->token.qmax = 1;
  re_ctx_p->token.greedy = true;

  return ECMA_VALUE_FALSE;
} /* re_parse_quantifier */

/**
 * Count the number of groups in the current pattern.
 */
static void
re_count_groups (re_compiler_ctx_t *re_ctx_p) /**< RegExp compiler context */
{
  bool is_char_class = 0;
  re_ctx_p->groups_count = 0;
  const lit_utf8_byte_t *curr_p = re_ctx_p->input_start_p;

  while (curr_p < re_ctx_p->input_end_p)
  {
    switch (*curr_p++)
    {
      case LIT_CHAR_BACKSLASH:
      {
        if (curr_p < re_ctx_p->input_end_p)
        {
          lit_utf8_incr (&curr_p);
        }
        break;
      }
      case LIT_CHAR_LEFT_SQUARE:
      {
        is_char_class = true;
        break;
      }
      case LIT_CHAR_RIGHT_SQUARE:
      {
        is_char_class = false;
        break;
      }
      case LIT_CHAR_LEFT_PAREN:
      {
        if (curr_p < re_ctx_p->input_end_p && *curr_p != LIT_CHAR_QUESTION && !is_char_class)
        {
          re_ctx_p->groups_count++;
        }
        break;
      }
    }
  }
} /* re_count_groups */

/**
 * Check if a code point is a Syntax character
 *
 * @return true, if syntax character
 *         false, otherwise
 */
static bool
re_is_syntax_char (lit_code_point_t cp) /**< code point */
{
  return (cp == LIT_CHAR_CIRCUMFLEX || cp == LIT_CHAR_DOLLAR_SIGN || cp == LIT_CHAR_BACKSLASH || cp == LIT_CHAR_DOT
          || cp == LIT_CHAR_ASTERISK || cp == LIT_CHAR_PLUS || cp == LIT_CHAR_QUESTION || cp == LIT_CHAR_LEFT_PAREN
          || cp == LIT_CHAR_RIGHT_PAREN || cp == LIT_CHAR_LEFT_SQUARE || cp == LIT_CHAR_RIGHT_SQUARE
          || cp == LIT_CHAR_LEFT_BRACE || cp == LIT_CHAR_RIGHT_BRACE || cp == LIT_CHAR_VLINE);
} /* re_is_syntax_char */

/**
 * Parse a Character Escape or a Character Class Escape.
 *
 * @return ECMA_VALUE_EMPTY, if parsed successfully
 *         ECMA_VALUE_ERROR, otherwise
 */
static ecma_value_t
re_parse_char_escape (re_compiler_ctx_t *re_ctx_p) /**< RegExp compiler context */
{
  JERRY_ASSERT (re_ctx_p->input_curr_p < re_ctx_p->input_end_p);
  re_ctx_p->token.type = RE_TOK_CHAR;

  if (lit_char_is_decimal_digit (*re_ctx_p->input_curr_p))
  {
    /* NULL code point escape, only valid if there are no following digits. */
    if (*re_ctx_p->input_curr_p == LIT_CHAR_0
        && (re_ctx_p->input_curr_p + 1 >= re_ctx_p->input_end_p
            || !lit_char_is_decimal_digit (re_ctx_p->input_curr_p[1])))
    {
      re_ctx_p->input_curr_p++;
      re_ctx_p->token.value = LIT_UNICODE_CODE_POINT_NULL;
      return ECMA_VALUE_EMPTY;
    }

    if (re_ctx_p->flags & RE_FLAG_UNICODE)
    {
      return ecma_raise_syntax_error (ECMA_ERR_INVALID_ESCAPE_SEQUENCE);
    }

    /* Legacy octal escape sequence */
    if (lit_char_is_octal_digit (*re_ctx_p->input_curr_p))
    {
      re_ctx_p->token.value = re_parse_octal (re_ctx_p);
      return ECMA_VALUE_EMPTY;
    }

    /* Identity escape */
    re_ctx_p->token.value = *re_ctx_p->input_curr_p++;
    return ECMA_VALUE_EMPTY;
  }

  lit_code_point_t ch = lit_cesu8_read_next (&re_ctx_p->input_curr_p);
  switch (ch)
  {
    /* Character Class escapes */
    case LIT_CHAR_LOWERCASE_D:
    {
      re_ctx_p->token.type = RE_TOK_CLASS_ESCAPE;
      re_ctx_p->token.value = RE_ESCAPE_DIGIT;
      break;
    }
    case LIT_CHAR_UPPERCASE_D:
    {
      re_ctx_p->token.type = RE_TOK_CLASS_ESCAPE;
      re_ctx_p->token.value = RE_ESCAPE_NOT_DIGIT;
      break;
    }
    case LIT_CHAR_LOWERCASE_S:
    {
      re_ctx_p->token.type = RE_TOK_CLASS_ESCAPE;
      re_ctx_p->token.value = RE_ESCAPE_WHITESPACE;
      break;
    }
    case LIT_CHAR_UPPERCASE_S:
    {
      re_ctx_p->token.type = RE_TOK_CLASS_ESCAPE;
      re_ctx_p->token.value = RE_ESCAPE_NOT_WHITESPACE;
      break;
    }
    case LIT_CHAR_LOWERCASE_W:
    {
      re_ctx_p->token.type = RE_TOK_CLASS_ESCAPE;
      re_ctx_p->token.value = RE_ESCAPE_WORD_CHAR;
      break;
    }
    case LIT_CHAR_UPPERCASE_W:
    {
      re_ctx_p->token.type = RE_TOK_CLASS_ESCAPE;
      re_ctx_p->token.value = RE_ESCAPE_NOT_WORD_CHAR;
      break;
    }
    /* Control escapes */
    case LIT_CHAR_LOWERCASE_F:
    {
      re_ctx_p->token.value = LIT_CHAR_FF;
      break;
    }
    case LIT_CHAR_LOWERCASE_N:
    {
      re_ctx_p->token.value = LIT_CHAR_LF;
      break;
    }
    case LIT_CHAR_LOWERCASE_R:
    {
      re_ctx_p->token.value = LIT_CHAR_CR;
      break;
    }
    case LIT_CHAR_LOWERCASE_T:
    {
      re_ctx_p->token.value = LIT_CHAR_TAB;
      break;
    }
    case LIT_CHAR_LOWERCASE_V:
    {
      re_ctx_p->token.value = LIT_CHAR_VTAB;
      break;
    }
    /* Control letter */
    case LIT_CHAR_LOWERCASE_C:
    {
      if (re_ctx_p->input_curr_p < re_ctx_p->input_end_p)
      {
        ch = *re_ctx_p->input_curr_p;

        if ((ch >= LIT_CHAR_ASCII_UPPERCASE_LETTERS_BEGIN && ch <= LIT_CHAR_ASCII_UPPERCASE_LETTERS_END)
            || (ch >= LIT_CHAR_ASCII_LOWERCASE_LETTERS_BEGIN && ch <= LIT_CHAR_ASCII_LOWERCASE_LETTERS_END))
        {
          re_ctx_p->token.value = (ch % 32);
          re_ctx_p->input_curr_p++;

          break;
        }
      }

      if (re_ctx_p->flags & RE_FLAG_UNICODE)
      {
        return ecma_raise_syntax_error (ECMA_ERR_INVALID_CONTROL_ESCAPE_SEQUENCE);
      }

      re_ctx_p->token.value = LIT_CHAR_BACKSLASH;
      re_ctx_p->input_curr_p--;

      break;
    }
    /* Hex escape */
    case LIT_CHAR_LOWERCASE_X:
    {
      uint32_t hex_value = lit_char_hex_lookup (re_ctx_p->input_curr_p, re_ctx_p->input_end_p, 2);
      if (hex_value != UINT32_MAX)
      {
        re_ctx_p->token.value = hex_value;
        re_ctx_p->input_curr_p += 2;
        break;
      }

      if (re_ctx_p->flags & RE_FLAG_UNICODE)
      {
        return ecma_raise_syntax_error (ECMA_ERR_INVALID_HEX_ESCAPE_SEQUENCE);
      }

      re_ctx_p->token.value = LIT_CHAR_LOWERCASE_X;
      break;
    }
    /* Unicode escape */
    case LIT_CHAR_LOWERCASE_U:
    {
      uint32_t hex_value = lit_char_hex_lookup (re_ctx_p->input_curr_p, re_ctx_p->input_end_p, 4);
      if (hex_value != UINT32_MAX)
      {
        re_ctx_p->token.value = hex_value;
        re_ctx_p->input_curr_p += 4;

        if (re_ctx_p->flags & RE_FLAG_UNICODE && lit_is_code_point_utf16_high_surrogate (re_ctx_p->token.value)
            && re_ctx_p->input_curr_p + 6 <= re_ctx_p->input_end_p && re_ctx_p->input_curr_p[0] == '\\'
            && re_ctx_p->input_curr_p[1] == 'u')
        {
          hex_value = lit_char_hex_lookup (re_ctx_p->input_curr_p + 2, re_ctx_p->input_end_p, 4);
          if (lit_is_code_point_utf16_low_surrogate (hex_value))
          {
            re_ctx_p->token.value =
              lit_convert_surrogate_pair_to_code_point ((ecma_char_t) re_ctx_p->token.value, (ecma_char_t) hex_value);
            re_ctx_p->input_curr_p += 6;
          }
        }

        break;
      }

      if (re_ctx_p->flags & RE_FLAG_UNICODE)
      {
        if (re_ctx_p->input_curr_p + 1 < re_ctx_p->input_end_p && re_ctx_p->input_curr_p[0] == LIT_CHAR_LEFT_BRACE
            && lit_char_is_hex_digit (re_ctx_p->input_curr_p[1]))
        {
          lit_code_point_t cp = lit_char_hex_to_int (re_ctx_p->input_curr_p[1]);
          re_ctx_p->input_curr_p += 2;

          while (re_ctx_p->input_curr_p < re_ctx_p->input_end_p && lit_char_is_hex_digit (*re_ctx_p->input_curr_p))
          {
            cp = cp * 16 + lit_char_hex_to_int (*re_ctx_p->input_curr_p++);

            if (JERRY_UNLIKELY (cp > LIT_UNICODE_CODE_POINT_MAX))
            {
              return ecma_raise_syntax_error (ECMA_ERR_INVALID_UNICODE_ESCAPE_SEQUENCE);
            }
          }

          if (re_ctx_p->input_curr_p < re_ctx_p->input_end_p && *re_ctx_p->input_curr_p == LIT_CHAR_RIGHT_BRACE)
          {
            re_ctx_p->input_curr_p++;
            re_ctx_p->token.value = cp;
            break;
          }
        }

        return ecma_raise_syntax_error (ECMA_ERR_INVALID_UNICODE_ESCAPE_SEQUENCE);
      }

      re_ctx_p->token.value = LIT_CHAR_LOWERCASE_U;
      break;
    }
    /* Identity escape */
    default:
    {
      /* Must be '/', or one of SyntaxCharacter */
      if (re_ctx_p->flags & RE_FLAG_UNICODE && ch != LIT_CHAR_SLASH && !re_is_syntax_char (ch))
      {
        return ecma_raise_syntax_error (ECMA_ERR_INVALID_ESCAPE);
      }
      re_ctx_p->token.value = ch;
    }
  }

  return ECMA_VALUE_EMPTY;
} /* re_parse_char_escape */

/**
 * Read the input pattern and parse the next token for the RegExp compiler
 *
 * @return empty ecma value - if parsed successfully
 *         error ecma value - otherwise
 *
 *         Returned value must be freed with ecma_free_value
 */
static ecma_value_t
re_parse_next_token (re_compiler_ctx_t *re_ctx_p) /**< RegExp compiler context */
{
  if (re_ctx_p->input_curr_p >= re_ctx_p->input_end_p)
  {
    re_ctx_p->token.type = RE_TOK_EOF;
    return ECMA_VALUE_EMPTY;
  }

  ecma_char_t ch = lit_cesu8_read_next (&re_ctx_p->input_curr_p);

  switch (ch)
  {
    case LIT_CHAR_CIRCUMFLEX:
    {
      re_ctx_p->token.type = RE_TOK_ASSERT_START;
      return ECMA_VALUE_EMPTY;
    }
    case LIT_CHAR_DOLLAR_SIGN:
    {
      re_ctx_p->token.type = RE_TOK_ASSERT_END;
      return ECMA_VALUE_EMPTY;
    }
    case LIT_CHAR_VLINE:
    {
      re_ctx_p->token.type = RE_TOK_ALTERNATIVE;
      return ECMA_VALUE_EMPTY;
    }
    case LIT_CHAR_DOT:
    {
      re_ctx_p->token.type = RE_TOK_PERIOD;
      /* Check quantifier */
      break;
    }
    case LIT_CHAR_BACKSLASH:
    {
      if (re_ctx_p->input_curr_p >= re_ctx_p->input_end_p)
      {
        return ecma_raise_syntax_error (ECMA_ERR_INVALID_ESCAPE);
      }

      /* DecimalEscape, Backreferences cannot start with a zero digit. */
      if (*re_ctx_p->input_curr_p > LIT_CHAR_0 && *re_ctx_p->input_curr_p <= LIT_CHAR_9)
      {
        const lit_utf8_byte_t *digits_p = re_ctx_p->input_curr_p;
        const uint32_t value = lit_parse_decimal (&digits_p, re_ctx_p->input_end_p);

        if (re_ctx_p->groups_count < 0)
        {
          re_count_groups (re_ctx_p);
        }

        if (value <= (uint32_t) re_ctx_p->groups_count)
        {
          /* Valid backreference */
          re_ctx_p->input_curr_p = digits_p;
          re_ctx_p->token.type = RE_TOK_BACKREFERENCE;
          re_ctx_p->token.value = value;

          /* Check quantifier */
          break;
        }
      }

      if (*re_ctx_p->input_curr_p == LIT_CHAR_LOWERCASE_B)
      {
        re_ctx_p->input_curr_p++;
        re_ctx_p->token.type = RE_TOK_ASSERT_WORD_BOUNDARY;
        return ECMA_VALUE_EMPTY;
      }
      else if (*re_ctx_p->input_curr_p == LIT_CHAR_UPPERCASE_B)
      {
        re_ctx_p->input_curr_p++;
        re_ctx_p->token.type = RE_TOK_ASSERT_NOT_WORD_BOUNDARY;
        return ECMA_VALUE_EMPTY;
      }

      const ecma_value_t parse_result = re_parse_char_escape (re_ctx_p);

      if (ECMA_IS_VALUE_ERROR (parse_result))
      {
        return parse_result;
      }

      /* Check quantifier */
      break;
    }
    case LIT_CHAR_LEFT_PAREN:
    {
      if (re_ctx_p->input_curr_p >= re_ctx_p->input_end_p)
      {
        return ecma_raise_syntax_error (ECMA_ERR_UNTERMINATED_GROUP);
      }

      if (*re_ctx_p->input_curr_p == LIT_CHAR_QUESTION)
      {
        re_ctx_p->input_curr_p++;
        if (re_ctx_p->input_curr_p >= re_ctx_p->input_end_p)
        {
          return ecma_raise_syntax_error (ECMA_ERR_INVALID_GROUP);
        }

        ch = *re_ctx_p->input_curr_p++;

        if (ch == LIT_CHAR_EQUALS)
        {
          re_ctx_p->token.type = RE_TOK_ASSERT_LOOKAHEAD;
          re_ctx_p->token.value = false;
        }
        else if (ch == LIT_CHAR_EXCLAMATION)
        {
          re_ctx_p->token.type = RE_TOK_ASSERT_LOOKAHEAD;
          re_ctx_p->token.value = true;
        }
        else if (ch == LIT_CHAR_COLON)
        {
          re_ctx_p->token.type = RE_TOK_START_NON_CAPTURE_GROUP;
        }
        else
        {
          return ecma_raise_syntax_error (ECMA_ERR_INVALID_GROUP);
        }
      }
      else
      {
        re_ctx_p->token.type = RE_TOK_START_CAPTURE_GROUP;
      }

      return ECMA_VALUE_EMPTY;
    }
    case LIT_CHAR_RIGHT_PAREN:
    {
      re_ctx_p->token.type = RE_TOK_END_GROUP;

      return ECMA_VALUE_EMPTY;
    }
    case LIT_CHAR_LEFT_SQUARE:
    {
      re_ctx_p->token.type = RE_TOK_CHAR_CLASS;

      if (re_ctx_p->input_curr_p >= re_ctx_p->input_end_p)
      {
        return ecma_raise_syntax_error (ECMA_ERR_UNTERMINATED_CHARACTER_CLASS);
      }

      return ECMA_VALUE_EMPTY;
    }
    case LIT_CHAR_QUESTION:
    case LIT_CHAR_ASTERISK:
    case LIT_CHAR_PLUS:
    {
      return ecma_raise_syntax_error (ECMA_ERR_INVALID_QUANTIFIER);
    }
    case LIT_CHAR_LEFT_BRACE:
    {
      re_ctx_p->input_curr_p--;
      if (ecma_is_value_true (re_parse_quantifier (re_ctx_p)))
      {
        return ecma_raise_syntax_error (ECMA_ERR_NOTHING_TO_REPEAT);
      }

      if (re_ctx_p->flags & RE_FLAG_UNICODE)
      {
        return ecma_raise_syntax_error (ECMA_ERR_LONE_QUANTIFIER_BRACKET);
      }

      re_ctx_p->input_curr_p++;
      re_ctx_p->token.type = RE_TOK_CHAR;
      re_ctx_p->token.value = ch;

      /* Check quantifier */
      break;
    }
    case LIT_CHAR_RIGHT_SQUARE:
    case LIT_CHAR_RIGHT_BRACE:
    {
      if (re_ctx_p->flags & RE_FLAG_UNICODE)
      {
        return ecma_raise_syntax_error (ECMA_ERR_LONE_QUANTIFIER_BRACKET);
      }

      /* FALLTHRU */
    }
    default:
    {
      re_ctx_p->token.type = RE_TOK_CHAR;
      re_ctx_p->token.value = ch;

      if (re_ctx_p->flags & RE_FLAG_UNICODE && lit_is_code_point_utf16_high_surrogate (ch)
          && re_ctx_p->input_curr_p < re_ctx_p->input_end_p)
      {
        const ecma_char_t next = lit_cesu8_peek_next (re_ctx_p->input_curr_p);
        if (lit_is_code_point_utf16_low_surrogate (next))
        {
          re_ctx_p->token.value = lit_convert_surrogate_pair_to_code_point (ch, next);
          re_ctx_p->input_curr_p += LIT_UTF8_MAX_BYTES_IN_CODE_UNIT;
        }
      }

      /* Check quantifier */
      break;
    }
  }

  re_parse_quantifier (re_ctx_p);
  return re_check_quantifier (re_ctx_p);
} /* re_parse_next_token */

/**
 * Append a character class range to the bytecode.
 */
static void
re_class_add_range (re_compiler_ctx_t *re_ctx_p, /**< RegExp compiler context */
                    lit_code_point_t start, /**< range begin */
                    lit_code_point_t end) /**< range end */
{
  if (re_ctx_p->flags & RE_FLAG_IGNORE_CASE)
  {
    start = ecma_regexp_canonicalize_char (start, re_ctx_p->flags & RE_FLAG_UNICODE);
    end = ecma_regexp_canonicalize_char (end, re_ctx_p->flags & RE_FLAG_UNICODE);
  }

  re_append_char (re_ctx_p, start);
  re_append_char (re_ctx_p, end);
} /* re_class_add_range */

/**
 * Add a single character to the character class
 */
static void
re_class_add_char (re_compiler_ctx_t *re_ctx_p, /**< RegExp compiler context */
                   uint32_t class_offset, /**< character class bytecode offset*/
                   lit_code_point_t cp) /**< code point */
{
  if (re_ctx_p->flags & RE_FLAG_IGNORE_CASE)
  {
    cp = ecma_regexp_canonicalize_char (cp, re_ctx_p->flags & RE_FLAG_UNICODE);
  }

  re_insert_char (re_ctx_p, class_offset, cp);
} /* re_class_add_char */

/**
 * Invalid character code point
 */
#define RE_INVALID_CP 0xFFFFFFFF

/**
 * Read the input pattern and parse the range of character class
 *
 * @return empty ecma value - if parsed successfully
 *         error ecma value - otherwise
 *
 *         Returned value must be freed with ecma_free_value
 */
static ecma_value_t
re_parse_char_class (re_compiler_ctx_t *re_ctx_p) /**< RegExp compiler context */
{
  static const uint8_t escape_flags[] = { 0x01, 0x02, 0x04, 0x08, 0x10, 0x20 };
  const uint32_t class_offset = re_bytecode_size (re_ctx_p);

  uint8_t found_escape_flags = 0;
  uint8_t out_class_flags = 0;

  uint32_t range_count = 0;
  uint32_t char_count = 0;
  bool is_range = false;

  JERRY_ASSERT (re_ctx_p->input_curr_p < re_ctx_p->input_end_p);
  if (*re_ctx_p->input_curr_p == LIT_CHAR_CIRCUMFLEX)
  {
    re_ctx_p->input_curr_p++;
    out_class_flags |= RE_CLASS_INVERT;
  }

  lit_code_point_t start = RE_INVALID_CP;

  while (true)
  {
    if (re_ctx_p->input_curr_p >= re_ctx_p->input_end_p)
    {
      return ecma_raise_syntax_error (ECMA_ERR_UNTERMINATED_CHARACTER_CLASS);
    }

    if (*re_ctx_p->input_curr_p == LIT_CHAR_RIGHT_SQUARE)
    {
      if (is_range)
      {
        if (start != RE_INVALID_CP)
        {
          re_class_add_char (re_ctx_p, class_offset, start);
          char_count++;
        }

        re_class_add_char (re_ctx_p, class_offset, LIT_CHAR_MINUS);
        char_count++;
      }

      re_ctx_p->input_curr_p++;
      break;
    }

    JERRY_ASSERT (re_ctx_p->input_curr_p < re_ctx_p->input_end_p);
    lit_code_point_t current;

    if (*re_ctx_p->input_curr_p == LIT_CHAR_BACKSLASH)
    {
      re_ctx_p->input_curr_p++;
      if (re_ctx_p->input_curr_p >= re_ctx_p->input_end_p)
      {
        return ecma_raise_syntax_error (ECMA_ERR_INVALID_ESCAPE);
      }

      if (*re_ctx_p->input_curr_p == LIT_CHAR_LOWERCASE_B)
      {
        re_ctx_p->input_curr_p++;
        current = LIT_CHAR_BS;
      }
      else if (*re_ctx_p->input_curr_p == LIT_CHAR_MINUS)
      {
        re_ctx_p->input_curr_p++;
        current = LIT_CHAR_MINUS;
      }
      else if ((re_ctx_p->flags & RE_FLAG_UNICODE) == 0 && *re_ctx_p->input_curr_p == LIT_CHAR_LOWERCASE_C
               && re_ctx_p->input_curr_p + 1 < re_ctx_p->input_end_p
               && (lit_char_is_decimal_digit (*(re_ctx_p->input_curr_p + 1))
                   || *(re_ctx_p->input_curr_p + 1) == LIT_CHAR_UNDERSCORE))
      {
        current = ((uint8_t) * (re_ctx_p->input_curr_p + 1) % 32);
        re_ctx_p->input_curr_p += 2;
      }
      else
      {
        if (ECMA_IS_VALUE_ERROR (re_parse_char_escape (re_ctx_p)))
        {
          return ECMA_VALUE_ERROR;
        }

        if (re_ctx_p->token.type == RE_TOK_CLASS_ESCAPE)
        {
          const uint8_t escape = (uint8_t) re_ctx_p->token.value;
          found_escape_flags |= escape_flags[escape];
          current = RE_INVALID_CP;
        }
        else
        {
          JERRY_ASSERT (re_ctx_p->token.type == RE_TOK_CHAR);
          current = re_ctx_p->token.value;
        }
      }
    }
    else if (re_ctx_p->flags & RE_FLAG_UNICODE)
    {
      current = ecma_regexp_unicode_advance (&re_ctx_p->input_curr_p, re_ctx_p->input_end_p);
    }
    else
    {
      current = lit_cesu8_read_next (&re_ctx_p->input_curr_p);
    }

    if (is_range)
    {
      is_range = false;

      if (start != RE_INVALID_CP && current != RE_INVALID_CP)
      {
        if (start > current)
        {
          return ecma_raise_syntax_error (ECMA_ERR_RANGE_OUT_OF_ORDER_IN_CHARACTER_CLASS);
        }

        re_class_add_range (re_ctx_p, start, current);
        range_count++;
        continue;
      }

      if (re_ctx_p->flags & RE_FLAG_UNICODE)
      {
        return ecma_raise_syntax_error (ECMA_ERR_INVALID_CHARACTER_CLASS);
      }

      if (start != RE_INVALID_CP)
      {
        re_class_add_char (re_ctx_p, class_offset, start);
        char_count++;
      }
      else if (current != RE_INVALID_CP)
      {
        re_class_add_char (re_ctx_p, class_offset, current);
        char_count++;
      }

      re_class_add_char (re_ctx_p, class_offset, LIT_CHAR_MINUS);
      char_count++;
      continue;
    }

    if (re_ctx_p->input_curr_p < re_ctx_p->input_end_p && *re_ctx_p->input_curr_p == LIT_CHAR_MINUS)
    {
      re_ctx_p->input_curr_p++;
      start = current;
      is_range = true;
      continue;
    }

    if (current != RE_INVALID_CP)
    {
      re_class_add_char (re_ctx_p, class_offset, current);
      char_count++;
    }
  }

  uint8_t escape_count = 0;
  for (ecma_class_escape_t escape = RE_ESCAPE__START; escape < RE_ESCAPE__COUNT; escape = (ecma_class_escape_t)((int) escape + 1))
  {
    if (found_escape_flags & escape_flags[escape])
    {
      re_insert_byte (re_ctx_p, class_offset, (uint8_t) escape);
      escape_count++;
    }
  }

  if (range_count > 0)
  {
    re_insert_value (re_ctx_p, class_offset, range_count);
    out_class_flags |= RE_CLASS_HAS_RANGES;
  }

  if (char_count > 0)
  {
    re_insert_value (re_ctx_p, class_offset, char_count);
    out_class_flags |= RE_CLASS_HAS_CHARS;
  }

  JERRY_ASSERT (escape_count <= RE_CLASS_ESCAPE_COUNT_MASK);
  out_class_flags |= escape_count;

  re_insert_byte (re_ctx_p, class_offset, out_class_flags);
  re_insert_opcode (re_ctx_p, class_offset, RE_OP_CHAR_CLASS);

  re_parse_quantifier (re_ctx_p);
  return re_check_quantifier (re_ctx_p);
} /* re_parse_char_class */

/**
 * Parse alternatives
 *
 * @return empty ecma value - if alternative was successfully parsed
 *         error ecma value - otherwise
 *
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
re_parse_alternative (re_compiler_ctx_t *re_ctx_p, /**< RegExp compiler context */
                      bool expect_eof) /**< expect end of file */
{
  ECMA_CHECK_STACK_USAGE ();
  uint32_t alternative_offset = re_bytecode_size (re_ctx_p);
  bool first_alternative = true;

  while (true)
  {
    ecma_value_t next_token_result = re_parse_next_token (re_ctx_p);
    if (ECMA_IS_VALUE_ERROR (next_token_result))
    {
      return next_token_result;
    }

    JERRY_ASSERT (ecma_is_value_empty (next_token_result));

    uint32_t atom_offset = re_bytecode_size (re_ctx_p);

    switch (re_ctx_p->token.type)
    {
      case RE_TOK_START_CAPTURE_GROUP:
      {
        const uint32_t idx = re_ctx_p->captures_count++;
        const uint32_t capture_start = idx;

        ecma_value_t result = re_parse_alternative (re_ctx_p, false);
        if (ECMA_IS_VALUE_ERROR (result))
        {
          return result;
        }

        re_parse_quantifier (re_ctx_p);

        if (ECMA_IS_VALUE_ERROR (re_check_quantifier (re_ctx_p)))
        {
          return ECMA_VALUE_ERROR;
        }

        re_insert_into_group (re_ctx_p, atom_offset, idx, capture_start, true);
        break;
      }
      case RE_TOK_START_NON_CAPTURE_GROUP:
      {
        const uint32_t idx = re_ctx_p->non_captures_count++;
        const uint32_t capture_start = re_ctx_p->captures_count;

        ecma_value_t result = re_parse_alternative (re_ctx_p, false);
        if (ECMA_IS_VALUE_ERROR (result))
        {
          return result;
        }

        re_parse_quantifier (re_ctx_p);

        if (ECMA_IS_VALUE_ERROR (re_check_quantifier (re_ctx_p)))
        {
          return ECMA_VALUE_ERROR;
        }

        re_insert_into_group (re_ctx_p, atom_offset, idx, capture_start, false);
        break;
      }
      case RE_TOK_PERIOD:
      {
        re_append_opcode (re_ctx_p, (re_ctx_p->flags & RE_FLAG_UNICODE) ? RE_OP_UNICODE_PERIOD : RE_OP_PERIOD);
        re_insert_atom_iterator (re_ctx_p, atom_offset);
        break;
      }
      case RE_TOK_ALTERNATIVE:
      {
        re_insert_value (re_ctx_p, alternative_offset, re_bytecode_size (re_ctx_p) - alternative_offset);
        re_insert_opcode (re_ctx_p,
                          alternative_offset,
                          first_alternative ? RE_OP_ALTERNATIVE_START : RE_OP_ALTERNATIVE_NEXT);

        alternative_offset = re_bytecode_size (re_ctx_p);
        first_alternative = false;
        break;
      }
      case RE_TOK_ASSERT_START:
      {
        re_append_opcode (re_ctx_p, RE_OP_ASSERT_LINE_START);
        break;
      }
      case RE_TOK_ASSERT_END:
      {
        re_append_opcode (re_ctx_p, RE_OP_ASSERT_LINE_END);
        break;
      }
      case RE_TOK_ASSERT_WORD_BOUNDARY:
      {
        re_append_opcode (re_ctx_p, RE_OP_ASSERT_WORD_BOUNDARY);
        break;
      }
      case RE_TOK_ASSERT_NOT_WORD_BOUNDARY:
      {
        re_append_opcode (re_ctx_p, RE_OP_ASSERT_NOT_WORD_BOUNDARY);
        break;
      }
      case RE_TOK_ASSERT_LOOKAHEAD:
      {
        const uint32_t start_capture_count = re_ctx_p->captures_count;
        const bool is_negative = !!re_ctx_p->token.value;

        ecma_value_t result = re_parse_alternative (re_ctx_p, false);

        if (ECMA_IS_VALUE_ERROR (result))
        {
          return result;
        }

        if (re_ctx_p->flags & RE_FLAG_UNICODE)
        {
          re_ctx_p->token.qmin = 1;
          re_ctx_p->token.qmax = 1;
          re_ctx_p->token.greedy = true;
        }
        else
        {
          re_parse_quantifier (re_ctx_p);

          if (ECMA_IS_VALUE_ERROR (re_check_quantifier (re_ctx_p)))
          {
            return ECMA_VALUE_ERROR;
          }
        }

        re_insert_assertion_lookahead (re_ctx_p, atom_offset, start_capture_count, is_negative);
        break;
      }
      case RE_TOK_BACKREFERENCE:
      {
        const uint32_t backref_idx = re_ctx_p->token.value;
        re_append_opcode (re_ctx_p, RE_OP_BACKREFERENCE);
        re_append_value (re_ctx_p, backref_idx);

        if (re_ctx_p->token.qmin != 1 || re_ctx_p->token.qmax != 1)
        {
          const uint32_t group_idx = re_ctx_p->non_captures_count++;
          re_insert_into_group (re_ctx_p, atom_offset, group_idx, re_ctx_p->captures_count, false);
        }

        break;
      }
      case RE_TOK_CLASS_ESCAPE:
      {
        const ecma_class_escape_t escape = (ecma_class_escape_t) re_ctx_p->token.value;
        re_append_opcode (re_ctx_p, RE_OP_CLASS_ESCAPE);
        re_append_byte (re_ctx_p, (uint8_t) escape);

        re_insert_atom_iterator (re_ctx_p, atom_offset);
        break;
      }
      case RE_TOK_CHAR_CLASS:
      {
        ecma_value_t result = re_parse_char_class (re_ctx_p);

        if (ECMA_IS_VALUE_ERROR (result))
        {
          return result;
        }

        re_insert_atom_iterator (re_ctx_p, atom_offset);
        break;
      }
      case RE_TOK_END_GROUP:
      {
        if (expect_eof)
        {
          return ecma_raise_syntax_error (ECMA_ERR_UNMATCHED_CLOSE_BRACKET);
        }

        if (!first_alternative)
        {
          re_insert_value (re_ctx_p, alternative_offset, re_bytecode_size (re_ctx_p) - alternative_offset);
          re_insert_opcode (re_ctx_p, alternative_offset, RE_OP_ALTERNATIVE_NEXT);
        }

        return ECMA_VALUE_EMPTY;
      }
      case RE_TOK_EOF:
      {
        if (!expect_eof)
        {
          return ecma_raise_syntax_error (ECMA_ERR_UNEXPECTED_END_OF_PATTERN);
        }

        if (!first_alternative)
        {
          re_insert_value (re_ctx_p, alternative_offset, re_bytecode_size (re_ctx_p) - alternative_offset);
          re_insert_opcode (re_ctx_p, alternative_offset, RE_OP_ALTERNATIVE_NEXT);
        }

        re_append_opcode (re_ctx_p, RE_OP_EOF);
        return ECMA_VALUE_EMPTY;
      }
      default:
      {
        JERRY_ASSERT (re_ctx_p->token.type == RE_TOK_CHAR);

        lit_code_point_t ch = re_ctx_p->token.value;

        if (ch <= LIT_UTF8_1_BYTE_CODE_POINT_MAX && (re_ctx_p->flags & RE_FLAG_IGNORE_CASE) == 0)
        {
          re_append_opcode (re_ctx_p, RE_OP_BYTE);
          re_append_byte (re_ctx_p, (uint8_t) ch);

          re_insert_atom_iterator (re_ctx_p, atom_offset);
          break;
        }

        if (re_ctx_p->flags & RE_FLAG_IGNORE_CASE)
        {
          ch = ecma_regexp_canonicalize_char (ch, re_ctx_p->flags & RE_FLAG_UNICODE);
        }

        re_append_opcode (re_ctx_p, RE_OP_CHAR);
        re_append_char (re_ctx_p, ch);

        re_insert_atom_iterator (re_ctx_p, atom_offset);
        break;
      }
    }
  }

  return ECMA_VALUE_EMPTY;
} /* re_parse_alternative */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
