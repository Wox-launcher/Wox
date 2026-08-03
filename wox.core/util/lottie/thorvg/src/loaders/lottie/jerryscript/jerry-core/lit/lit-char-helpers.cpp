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

#include "lit-char-helpers.h"

#include "ecma-helpers.h"

#include "jerry-config.h"
#include "lit-strings.h"
#include "lit-unicode-ranges-sup.inc.h"
#include "lit-unicode-ranges.inc.h"

#if JERRY_UNICODE_CASE_CONVERSION
#include "lit-unicode-conversions-sup.inc.h"
#include "lit-unicode-conversions.inc.h"
#include "lit-unicode-folding.inc.h"
#endif /* JERRY_UNICODE_CASE_CONVERSION */

#define NUM_OF_ELEMENTS(array) (sizeof (array) / sizeof ((array)[0]))

/**
 * Binary search algorithm that searches the a
 * character in the given char array.
 *
 * @return true - if the character is in the given array
 *         false - otherwise
 */
#define LIT_SEARCH_CHAR_IN_ARRAY_FN(function_name, char_type, array_type)   \
  static bool function_name (char_type c, /**< code unit */                 \
                             const array_type *array, /**< array */         \
                             int size_of_array) /**< length of the array */ \
  {                                                                         \
    int bottom = 0;                                                         \
    int top = size_of_array - 1;                                            \
                                                                            \
    while (bottom <= top)                                                   \
    {                                                                       \
      int middle = (bottom + top) / 2;                                      \
      char_type current = array[middle];                                    \
                                                                            \
      if (current == c)                                                     \
      {                                                                     \
        return true;                                                        \
      }                                                                     \
                                                                            \
      if (c < current)                                                      \
      {                                                                     \
        top = middle - 1;                                                   \
      }                                                                     \
      else                                                                  \
      {                                                                     \
        bottom = middle + 1;                                                \
      }                                                                     \
    }                                                                       \
                                                                            \
    return false;                                                           \
  } /* __function_name */

LIT_SEARCH_CHAR_IN_ARRAY_FN (lit_search_char_in_array, ecma_char_t, uint16_t)

LIT_SEARCH_CHAR_IN_ARRAY_FN (lit_search_codepoint_in_array, lit_code_point_t, uint32_t)

/**
 * Binary search algorithm that searches a character in the given intervals.
 * Intervals specified by two arrays. The first one contains the starting points
 * of the intervals, the second one contains the length of them.
 *
 * @return true - if the character is included (inclusively) in one of the intervals in the given array
 *         false - otherwise
 */
#define LIT_SEARCH_CHAR_IN_INTERVAL_ARRAY_FN(function_name, char_type, array_type, interval_type)  \
  static bool function_name (char_type c, /**< code unit */                                        \
                             const array_type *array_sp, /**< array of interval starting points */ \
                             const interval_type *lengths, /**< array of interval lengths */       \
                             int size_of_array) /**< length of the array */                        \
  {                                                                                                \
    int bottom = 0;                                                                                \
    int top = size_of_array - 1;                                                                   \
                                                                                                   \
    while (bottom <= top)                                                                          \
    {                                                                                              \
      int middle = (bottom + top) / 2;                                                             \
      char_type current_sp = array_sp[middle];                                                     \
                                                                                                   \
      if (current_sp <= c && c <= current_sp + lengths[middle])                                    \
      {                                                                                            \
        return true;                                                                               \
      }                                                                                            \
                                                                                                   \
      if (c > current_sp)                                                                          \
      {                                                                                            \
        bottom = middle + 1;                                                                       \
      }                                                                                            \
      else                                                                                         \
      {                                                                                            \
        top = middle - 1;                                                                          \
      }                                                                                            \
    }                                                                                              \
                                                                                                   \
    return false;                                                                                  \
  } /* function_name */

LIT_SEARCH_CHAR_IN_INTERVAL_ARRAY_FN (lit_search_char_in_interval_array, ecma_char_t, uint16_t, uint8_t)

LIT_SEARCH_CHAR_IN_INTERVAL_ARRAY_FN (lit_search_codepoint_in_interval_array, lit_code_point_t, uint32_t, uint16_t)

/**
 * Check if specified character is one of the Whitespace characters including those that fall into
 * "Space, Separator" ("Zs") Unicode character category or one of the Line Terminator characters.
 *
 * @return true - if the character is one of characters, listed in ECMA-262 v5, Table 2,
 *         false - otherwise
 */
bool
lit_char_is_white_space (lit_code_point_t c) /**< code point */
{
  if (c <= LIT_UTF8_1_BYTE_CODE_POINT_MAX)
  {
    return (c == LIT_CHAR_SP || (c >= LIT_CHAR_TAB && c <= LIT_CHAR_CR));
  }

  if (c == LIT_CHAR_BOM || c == LIT_CHAR_LS || c == LIT_CHAR_PS)
  {
    return true;
  }

  return (c <= LIT_UTF16_CODE_UNIT_MAX
          && ((c >= lit_unicode_white_space_interval_starts[0]
               && c <= lit_unicode_white_space_interval_starts[0] + lit_unicode_white_space_interval_lengths[0])
              || lit_search_char_in_array ((ecma_char_t) c,
                                           lit_unicode_white_space_chars,
                                           NUM_OF_ELEMENTS (lit_unicode_white_space_chars))));
} /* lit_char_is_white_space */

/**
 * Check if specified character is one of LineTerminator characters
 *
 * @return true - if the character is one of characters, listed in ECMA-262 v5, Table 3,
 *         false - otherwise
 */
bool
lit_char_is_line_terminator (ecma_char_t c) /**< code unit */
{
  return (c == LIT_CHAR_LF || c == LIT_CHAR_CR || c == LIT_CHAR_LS || c == LIT_CHAR_PS);
} /* lit_char_is_line_terminator */

/**
 * Check if specified character is a Unicode ID_Start
 *
 * See also:
 *          ECMA-262 v1, 11.6: UnicodeIDStart
 *
 * @return true - if the codepoint has Unicode property "ID_Start"
 *         false - otherwise
 */
static bool
lit_char_is_unicode_id_start (lit_code_point_t code_point) /**< code unit */
{
  if (JERRY_UNLIKELY (code_point >= LIT_UTF8_4_BYTE_CODE_POINT_MIN))
  {
    return (lit_search_codepoint_in_interval_array (code_point,
                                                    lit_unicode_id_start_interval_starts_sup,
                                                    lit_unicode_id_start_interval_lengths_sup,
                                                    NUM_OF_ELEMENTS (lit_unicode_id_start_interval_starts_sup))
            || lit_search_codepoint_in_array (code_point,
                                              lit_unicode_id_start_chars_sup,
                                              NUM_OF_ELEMENTS (lit_unicode_id_start_chars_sup)));
  }
  ecma_char_t c = (ecma_char_t) code_point;

  return (lit_search_char_in_interval_array (c,
                                             lit_unicode_id_start_interval_starts,
                                             lit_unicode_id_start_interval_lengths,
                                             NUM_OF_ELEMENTS (lit_unicode_id_start_interval_starts))
          || lit_search_char_in_array (c, lit_unicode_id_start_chars, NUM_OF_ELEMENTS (lit_unicode_id_start_chars)));
} /* lit_char_is_unicode_id_start */

/**
 * Check if specified character is a Unicode ID_Continue
 *
 * See also:
 *          ECMA-262 v1, 11.6: UnicodeIDContinue
 *
 * @return true - if the codepoint has Unicode property "ID_Continue"
 *         false - otherwise
 */
static bool
lit_char_is_unicode_id_continue (lit_code_point_t code_point) /**< code unit */
{
  /* Each ID_Start codepoint is ID_Continue as well. */
  if (lit_char_is_unicode_id_start (code_point))
  {
    return true;
  }

  if (JERRY_UNLIKELY (code_point >= LIT_UTF8_4_BYTE_CODE_POINT_MIN))
  {
    return (lit_search_codepoint_in_interval_array (code_point,
                                                    lit_unicode_id_continue_interval_starts_sup,
                                                    lit_unicode_id_continue_interval_lengths_sup,
                                                    NUM_OF_ELEMENTS (lit_unicode_id_continue_interval_starts_sup))
            || lit_search_codepoint_in_array (code_point,
                                              lit_unicode_id_continue_chars_sup,
                                              NUM_OF_ELEMENTS (lit_unicode_id_continue_chars_sup)));
  }
  ecma_char_t c = (ecma_char_t) code_point;

  return (
    lit_search_char_in_interval_array (c,
                                       lit_unicode_id_continue_interval_starts,
                                       lit_unicode_id_continue_interval_lengths,
                                       NUM_OF_ELEMENTS (lit_unicode_id_continue_interval_starts))
    || lit_search_char_in_array (c, lit_unicode_id_continue_chars, NUM_OF_ELEMENTS (lit_unicode_id_continue_chars)));
} /* lit_char_is_unicode_id_continue */

/**
 * Checks whether the character is a valid identifier start.
 *
 * @return true if it is.
 */
bool
lit_code_point_is_identifier_start (lit_code_point_t code_point) /**< code point */
{
  /* Fast path for ASCII-defined letters. */
  if (code_point <= LIT_UTF8_1_BYTE_CODE_POINT_MAX)
  {
    return ((LEXER_TO_ASCII_LOWERCASE (code_point) >= LIT_CHAR_LOWERCASE_A
             && LEXER_TO_ASCII_LOWERCASE (code_point) <= LIT_CHAR_LOWERCASE_Z)
            || code_point == LIT_CHAR_DOLLAR_SIGN || code_point == LIT_CHAR_UNDERSCORE);
  }

  return lit_char_is_unicode_id_start (code_point);
} /* lit_code_point_is_identifier_start */

/**
 * Checks whether the character is a valid identifier part.
 *
 * @return true if it is.
 */
bool
lit_code_point_is_identifier_part (lit_code_point_t code_point) /**< code point */
{
  /* Fast path for ASCII-defined letters. */
  if (code_point <= LIT_UTF8_1_BYTE_CODE_POINT_MAX)
  {
    return ((LEXER_TO_ASCII_LOWERCASE (code_point) >= LIT_CHAR_LOWERCASE_A
             && LEXER_TO_ASCII_LOWERCASE (code_point) <= LIT_CHAR_LOWERCASE_Z)
            || (code_point >= LIT_CHAR_0 && code_point <= LIT_CHAR_9) || code_point == LIT_CHAR_DOLLAR_SIGN
            || code_point == LIT_CHAR_UNDERSCORE);
  }

  return lit_char_is_unicode_id_continue (code_point);
} /* lit_code_point_is_identifier_part */

/**
 * Check if specified character is one of OctalDigit characters (ECMA-262 v5, B.1.2)
 *
 * @return true / false
 */
bool
lit_char_is_octal_digit (ecma_char_t c) /**< code unit */
{
  return (c >= LIT_CHAR_ASCII_OCTAL_DIGITS_BEGIN && c <= LIT_CHAR_ASCII_OCTAL_DIGITS_END);
} /* lit_char_is_octal_digit */

/**
 * Check if specified character is one of DecimalDigit characters (ECMA-262 v5, 7.8.3)
 *
 * @return true / false
 */
bool
lit_char_is_decimal_digit (ecma_char_t c) /**< code unit */
{
  return (c >= LIT_CHAR_ASCII_DIGITS_BEGIN && c <= LIT_CHAR_ASCII_DIGITS_END);
} /* lit_char_is_decimal_digit */

/**
 * Check if specified character is one of HexDigit characters (ECMA-262 v5, 7.8.3)
 *
 * @return true / false
 */
bool
lit_char_is_hex_digit (ecma_char_t c) /**< code unit */
{
  return ((c >= LIT_CHAR_ASCII_DIGITS_BEGIN && c <= LIT_CHAR_ASCII_DIGITS_END)
          || (LEXER_TO_ASCII_LOWERCASE (c) >= LIT_CHAR_ASCII_LOWERCASE_LETTERS_HEX_BEGIN
              && LEXER_TO_ASCII_LOWERCASE (c) <= LIT_CHAR_ASCII_LOWERCASE_LETTERS_HEX_END));
} /* lit_char_is_hex_digit */

/**
 * Check if specified character is one of BinaryDigits characters (ECMA-262 v6, 11.8.3)
 *
 * @return true / false
 */
bool
lit_char_is_binary_digit (ecma_char_t c) /** code unit */
{
  return (c == LIT_CHAR_0 || c == LIT_CHAR_1);
} /* lit_char_is_binary_digit */

/**
 * @return radix value
 */
uint8_t
lit_char_to_radix (lit_utf8_byte_t c) /** code unit */
{
  switch (LEXER_TO_ASCII_LOWERCASE (c))
  {
    case LIT_CHAR_LOWERCASE_X:
    {
      return 16;
    }
    case LIT_CHAR_LOWERCASE_O:
    {
      return 8;
    }
    case LIT_CHAR_LOWERCASE_B:
    {
      return 2;
    }
    default:
    {
      return 10;
    }
  }
} /* lit_char_to_radix */

/**
 * UnicodeEscape abstract method
 *
 * See also: ECMA-262 v10, 24.5.2.3
 */
void
lit_char_unicode_escape (ecma_stringbuilder_t *builder_p, /**< stringbuilder to append */
                         ecma_char_t c) /**< code unit to convert */
{
  ecma_stringbuilder_append_raw (builder_p, (lit_utf8_byte_t *) "\\u", 2);

  for (int8_t i = 3; i >= 0; i--)
  {
    int32_t result_char = (c >> (i * 4)) & 0xF;
    ecma_stringbuilder_append_byte (
      builder_p,
      (lit_utf8_byte_t) (result_char + (result_char <= 9 ? LIT_CHAR_0 : (LIT_CHAR_LOWERCASE_A - 10))));
  }
} /* lit_char_unicode_escape */

/**
 * Convert a HexDigit character to its numeric value, as defined in ECMA-262 v5, 7.8.3
 *
 * @return digit value, corresponding to the hex char
 */
uint32_t
lit_char_hex_to_int (ecma_char_t c) /**< code unit, corresponding to
                                     *    one of HexDigit characters */
{
  JERRY_ASSERT (lit_char_is_hex_digit (c));

  if (c >= LIT_CHAR_ASCII_DIGITS_BEGIN && c <= LIT_CHAR_ASCII_DIGITS_END)
  {
    return (uint32_t) (c - LIT_CHAR_ASCII_DIGITS_BEGIN);
  }

  const uint32_t hex_offset = 10 - (LIT_CHAR_LOWERCASE_A % 32);
  return (c % 32) + hex_offset;
} /* lit_char_hex_to_int */

/**
 * Converts a character to UTF8 bytes.
 *
 * @return length of the UTF8 representation.
 */
size_t
lit_code_point_to_cesu8_bytes (uint8_t *dst_p, /**< destination buffer */
                               lit_code_point_t code_point) /**< code point */
{
  if (code_point < LIT_UTF8_2_BYTE_CODE_POINT_MIN)
  {
    /* 00000000 0xxxxxxx -> 0xxxxxxx */
    dst_p[0] = (uint8_t) code_point;
    return 1;
  }

  if (code_point < LIT_UTF8_3_BYTE_CODE_POINT_MIN)
  {
    /* 00000yyy yyxxxxxx -> 110yyyyy 10xxxxxx */
    dst_p[0] = (uint8_t) (LIT_UTF8_2_BYTE_MARKER | ((code_point >> 6) & LIT_UTF8_LAST_5_BITS_MASK));
    dst_p[1] = (uint8_t) (LIT_UTF8_EXTRA_BYTE_MARKER | (code_point & LIT_UTF8_LAST_6_BITS_MASK));
    return 2;
  }

  if (code_point < LIT_UTF8_4_BYTE_CODE_POINT_MIN)
  {
    /* zzzzyyyy yyxxxxxx -> 1110zzzz 10yyyyyy 10xxxxxx */
    dst_p[0] = (uint8_t) (LIT_UTF8_3_BYTE_MARKER | ((code_point >> 12) & LIT_UTF8_LAST_4_BITS_MASK));
    dst_p[1] = (uint8_t) (LIT_UTF8_EXTRA_BYTE_MARKER | ((code_point >> 6) & LIT_UTF8_LAST_6_BITS_MASK));
    dst_p[2] = (uint8_t) (LIT_UTF8_EXTRA_BYTE_MARKER | (code_point & LIT_UTF8_LAST_6_BITS_MASK));
    return 3;
  }

  JERRY_ASSERT (code_point <= LIT_UNICODE_CODE_POINT_MAX);

  code_point -= LIT_UTF8_4_BYTE_CODE_POINT_MIN;

  dst_p[0] = (uint8_t) (LIT_UTF8_3_BYTE_MARKER | 0xd);
  dst_p[1] = (uint8_t) (LIT_UTF8_EXTRA_BYTE_MARKER | 0x20 | ((code_point >> 16) & LIT_UTF8_LAST_4_BITS_MASK));
  dst_p[2] = (uint8_t) (LIT_UTF8_EXTRA_BYTE_MARKER | ((code_point >> 10) & LIT_UTF8_LAST_6_BITS_MASK));

  dst_p[3] = (uint8_t) (LIT_UTF8_3_BYTE_MARKER | 0xd);
  dst_p[4] = (uint8_t) (LIT_UTF8_EXTRA_BYTE_MARKER | 0x30 | ((code_point >> 6) & LIT_UTF8_LAST_4_BITS_MASK));
  dst_p[5] = (uint8_t) (LIT_UTF8_EXTRA_BYTE_MARKER | (code_point & LIT_UTF8_LAST_6_BITS_MASK));

  return 3 * 2;
} /* lit_code_point_to_cesu8_bytes */

/**
 * Returns the length of the UTF8 representation of a character.
 *
 * @return length of the UTF8 representation.
 */
size_t
lit_code_point_get_cesu8_length (lit_code_point_t code_point) /**< code point */
{
  if (code_point < LIT_UTF8_2_BYTE_CODE_POINT_MIN)
  {
    /* 00000000 0xxxxxxx */
    return 1;
  }

  if (code_point < LIT_UTF8_3_BYTE_CODE_POINT_MIN)
  {
    /* 00000yyy yyxxxxxx */
    return 2;
  }

  if (code_point < LIT_UTF8_4_BYTE_CODE_POINT_MIN)
  {
    /* zzzzyyyy yyxxxxxx */
    return 3;
  }

  /* high + low surrogate */
  return 2 * 3;
} /* lit_code_point_get_cesu8_length */

/**
 * Convert a four byte long utf8 character to two three byte long cesu8 characters
 */
void
lit_four_byte_utf8_char_to_cesu8 (uint8_t *dst_p, /**< destination buffer */
                                  const uint8_t *source_p) /**< source buffer */
{
  lit_code_point_t code_point = ((((uint32_t) source_p[0]) & LIT_UTF8_LAST_3_BITS_MASK) << 18);
  code_point |= ((((uint32_t) source_p[1]) & LIT_UTF8_LAST_6_BITS_MASK) << 12);
  code_point |= ((((uint32_t) source_p[2]) & LIT_UTF8_LAST_6_BITS_MASK) << 6);
  code_point |= (((uint32_t) source_p[3]) & LIT_UTF8_LAST_6_BITS_MASK);

  lit_code_point_to_cesu8_bytes (dst_p, code_point);
} /* lit_four_byte_utf8_char_to_cesu8 */

/**
 * Lookup hex digits in a buffer
 *
 * @return UINT32_MAX - if next 'lookup' number of characters do not form a valid hex number
 *         value of hex number, otherwise
 */
uint32_t
lit_char_hex_lookup (const lit_utf8_byte_t *buf_p, /**< buffer */
                     const lit_utf8_byte_t *const buf_end_p, /**< buffer end */
                     uint32_t lookup) /**< size of lookup */
{
  JERRY_ASSERT (lookup <= 4);

  if (JERRY_UNLIKELY (buf_p + lookup > buf_end_p))
  {
    return UINT32_MAX;
  }

  uint32_t value = 0;

  while (lookup--)
  {
    lit_utf8_byte_t ch = *buf_p++;
    if (!lit_char_is_hex_digit (ch))
    {
      return UINT32_MAX;
    }

    value <<= 4;
    value += lit_char_hex_to_int (ch);
  }

  JERRY_ASSERT (value <= LIT_UTF16_CODE_UNIT_MAX);
  return value;
} /* lit_char_hex_lookup */

/**
 * Parse a decimal number with the value clamped to UINT32_MAX.
 *
 * @returns uint32_t number
 */
uint32_t
lit_parse_decimal (const lit_utf8_byte_t **buffer_p, /**< [in/out] character buffer */
                   const lit_utf8_byte_t *buffer_end_p) /**< buffer end */
{
  const lit_utf8_byte_t *current_p = *buffer_p;
  JERRY_ASSERT (lit_char_is_decimal_digit (*current_p));

  uint32_t value = (uint32_t) (*current_p++ - LIT_CHAR_0);

  while (current_p < buffer_end_p && lit_char_is_decimal_digit (*current_p))
  {
    const uint32_t digit = (uint32_t) (*current_p++ - LIT_CHAR_0);
    uint32_t new_value = value * 10 + digit;

    if (JERRY_UNLIKELY (value > UINT32_MAX / 10) || JERRY_UNLIKELY (new_value < value))
    {
      value = UINT32_MAX;
      continue;
    }

    value = new_value;
  }

  *buffer_p = current_p;
  return value;
} /* lit_parse_decimal */

/**
 * Check if specified character is a word character (part of IsWordChar abstract operation)
 *
 * See also: ECMA-262 v5, 15.10.2.6 (IsWordChar)
 *
 * @return true - if the character is a word character
 *         false - otherwise
 */
bool
lit_char_is_word_char (lit_code_point_t c) /**< code point */
{
  return ((c >= LIT_CHAR_ASCII_LOWERCASE_LETTERS_BEGIN && c <= LIT_CHAR_ASCII_LOWERCASE_LETTERS_END)
          || (c >= LIT_CHAR_ASCII_UPPERCASE_LETTERS_BEGIN && c <= LIT_CHAR_ASCII_UPPERCASE_LETTERS_END)
          || (c >= LIT_CHAR_ASCII_DIGITS_BEGIN && c <= LIT_CHAR_ASCII_DIGITS_END) || c == LIT_CHAR_UNDERSCORE);
} /* lit_char_is_word_char */

#if JERRY_UNICODE_CASE_CONVERSION

/**
 * Check if the specified character is in one of those tables which contain bidirectional conversions.
 *
 * @return codepoint of the converted character if it is found the tables
 *         LIT_INVALID_CP - otherwise.
 */
static lit_code_point_t
lit_search_in_bidirectional_conversion_tables (lit_code_point_t cp, /**< code point */
                                               bool is_lowercase) /**< is lowercase conversion */
{
  /* 1, Check if the specified character is part of the lit_unicode_character_case_ranges_{sup} table. */
  int number_of_case_ranges;
  bool is_supplementary = cp > LIT_UTF16_CODE_UNIT_MAX;

  if (is_supplementary)
  {
    number_of_case_ranges = NUM_OF_ELEMENTS (lit_unicode_character_case_ranges_sup);
  }
  else
  {
    number_of_case_ranges = NUM_OF_ELEMENTS (lit_unicode_character_case_ranges);
  }

  int conv_counter = 0;

  for (int i = 0; i < number_of_case_ranges; i++)
  {
    if (i % 2 == 0 && i > 0)
    {
      conv_counter++;
    }

    size_t range_length;
    lit_code_point_t start_point;

    if (is_supplementary)
    {
      range_length = lit_unicode_character_case_range_lengths_sup[conv_counter];
      start_point = lit_unicode_character_case_ranges_sup[i];
    }
    else
    {
      range_length = lit_unicode_character_case_range_lengths[conv_counter];
      start_point = lit_unicode_character_case_ranges[i];
    }

    if (start_point > cp || cp >= start_point + range_length)
    {
      continue;
    }

    uint32_t char_dist = (uint32_t) cp - start_point;
    int offset;
    if (i % 2 == 0)
    {
      if (!is_lowercase)
      {
        return cp;
      }

      offset = i + 1;
    }
    else
    {
      if (is_lowercase)
      {
        return cp;
      }

      offset = i - 1;
    }

    if (is_supplementary)
    {
      start_point = lit_unicode_character_case_ranges_sup[offset];
    }
    else
    {
      start_point = lit_unicode_character_case_ranges[offset];
    }

    return (lit_code_point_t) (start_point + char_dist);
  }

  /* Note: After this point based on the latest unicode standard(13.0.0.6) no conversion characters are
     defined for supplementary planes */
  if (is_supplementary)
  {
    return cp;
  }

  /* 2, Check if the specified character is part of the character_pair_ranges table. */
  int bottom = 0;
  int top = NUM_OF_ELEMENTS (lit_unicode_character_pair_ranges) - 1;

  while (bottom <= top)
  {
    int middle = (bottom + top) / 2;
    lit_code_point_t current_sp = lit_unicode_character_pair_ranges[middle];

    if (current_sp <= cp && cp < current_sp + lit_unicode_character_pair_range_lengths[middle])
    {
      uint32_t char_dist = (uint32_t) (cp - current_sp);

      if ((cp - current_sp) % 2 == 0)
      {
        return is_lowercase ? (lit_code_point_t) (current_sp + char_dist + 1) : cp;
      }

      return is_lowercase ? cp : (lit_code_point_t) (current_sp + char_dist - 1);
    }

    if (cp > current_sp)
    {
      bottom = middle + 1;
    }
    else
    {
      top = middle - 1;
    }
  }

  /* 3, Check if the specified character is part of the character_pairs table. */
  int number_of_character_pairs = NUM_OF_ELEMENTS (lit_unicode_character_pairs);

  for (int i = 0; i < number_of_character_pairs; i++)
  {
    if (cp != lit_unicode_character_pairs[i])
    {
      continue;
    }

    if (i % 2 == 0)
    {
      return is_lowercase ? lit_unicode_character_pairs[i + 1] : cp;
    }

    return is_lowercase ? cp : lit_unicode_character_pairs[i - 1];
  }

  return LIT_INVALID_CP;
} /* lit_search_in_bidirectional_conversion_tables */

/**
 * Check if the specified character is in the given conversion table.
 *
 * @return LIT_MULTIPLE_CU if the converted character consist more than a single code unit
 *         converted code point - otherwise
 */
static lit_code_point_t
lit_search_in_conversion_table (ecma_char_t character, /**< code unit */
                                ecma_stringbuilder_t *builder_p, /**< string builder */
                                const ecma_char_t *array, /**< array */
                                const uint8_t *counters) /**< case_values counter */
{
  int end_point = 0;

  for (int i = 0; i < 3; i++)
  {
    int start_point = end_point;
    int size_of_case_value = i + 1;
    end_point += counters[i] * (size_of_case_value + 1);

    int bottom = start_point;
    int top = end_point - size_of_case_value;

    while (bottom <= top)
    {
      int middle = (bottom + top) / 2;

      middle -= ((middle - bottom) % (size_of_case_value + 1));

      ecma_char_t current = array[middle];

      if (current == character)
      {
        if (builder_p != NULL)
        {
          ecma_stringbuilder_append_char (builder_p, array[middle + 1]);

          if (size_of_case_value > 1)
          {
            ecma_stringbuilder_append_char (builder_p, array[middle + 2]);
          }
          if (size_of_case_value > 2)
          {
            ecma_stringbuilder_append_char (builder_p, array[middle + 3]);
          }
        }

        return size_of_case_value == 1 ? array[middle + 1] : LIT_MULTIPLE_CU;
      }

      if (character < current)
      {
        top = middle - (size_of_case_value + 1);
      }
      else
      {
        bottom = middle + (size_of_case_value + 1);
      }
    }
  }

  if (builder_p != NULL)
  {
    ecma_stringbuilder_append_char (builder_p, character);
  }

  return (lit_code_point_t) character;
} /* lit_search_in_conversion_table */
#endif /* JERRY_UNICODE_CASE_CONVERSION */

/**
 * Append the converted lowercase codeunit sequence of an a given codepoint into the stringbuilder if it is present.
 *
 * @return LIT_MULTIPLE_CU if the converted codepoint consist more than a single code unit
 *         converted code point - otherwise
 */
lit_code_point_t
lit_char_to_lower_case (lit_code_point_t cp, /**< code point */
                        ecma_stringbuilder_t *builder_p) /**< string builder */
{
  if (cp <= LIT_UTF8_1_BYTE_CODE_POINT_MAX)
  {
    if (cp >= LIT_CHAR_UPPERCASE_A && cp <= LIT_CHAR_UPPERCASE_Z)
    {
      cp = (lit_utf8_byte_t) (cp + (LIT_CHAR_LOWERCASE_A - LIT_CHAR_UPPERCASE_A));
    }

    if (builder_p != NULL)
    {
      ecma_stringbuilder_append_byte (builder_p, (lit_utf8_byte_t) cp);
    }

    return cp;
  }

#if JERRY_UNICODE_CASE_CONVERSION
  lit_code_point_t lowercase_cp = lit_search_in_bidirectional_conversion_tables (cp, true);

  if (lowercase_cp != LIT_INVALID_CP)
  {
    if (builder_p != NULL)
    {
      ecma_stringbuilder_append_codepoint (builder_p, lowercase_cp);
    }

    return lowercase_cp;
  }

  JERRY_ASSERT (cp < LIT_UTF8_4_BYTE_CODE_POINT_MIN);

  int num_of_lowercase_ranges = NUM_OF_ELEMENTS (lit_unicode_lower_case_ranges);

  for (int i = 0, j = 0; i < num_of_lowercase_ranges; i += 2, j++)
  {
    JERRY_ASSERT (lit_unicode_lower_case_range_lengths[j] > 0);
    uint32_t range_length = (uint32_t) (lit_unicode_lower_case_range_lengths[j] - 1);
    lit_code_point_t start_point = lit_unicode_lower_case_ranges[i];

    if (start_point <= cp && cp <= start_point + range_length)
    {
      lowercase_cp = lit_unicode_lower_case_ranges[i + 1] + (cp - start_point);
      if (builder_p != NULL)
      {
        ecma_stringbuilder_append_codepoint (builder_p, lowercase_cp);
      }

      return lowercase_cp;
    }
  }

  return lit_search_in_conversion_table ((ecma_char_t) cp,
                                         builder_p,
                                         lit_unicode_lower_case_conversions,
                                         lit_unicode_lower_case_conversion_counters);
#else /* !JERRY_UNICODE_CASE_CONVERSION */
  if (builder_p != NULL)
  {
    ecma_stringbuilder_append_codepoint (builder_p, cp);
  }

  return cp;
#endif /* JERRY_UNICODE_CASE_CONVERSION */
} /* lit_char_to_lower_case */

/**
 * Append the converted uppercase codeunit sequence of an a given codepoint into the stringbuilder if it is present.
 *
 * @return LIT_MULTIPLE_CU if the converted codepoint consist more than a single code unit
 *         converted code point - otherwise
 */
lit_code_point_t
lit_char_to_upper_case (lit_code_point_t cp, /**< code point */
                        ecma_stringbuilder_t *builder_p) /**< string builder */
{
  if (cp <= LIT_UTF8_1_BYTE_CODE_POINT_MAX)
  {
    if (cp >= LIT_CHAR_LOWERCASE_A && cp <= LIT_CHAR_LOWERCASE_Z)
    {
      cp = (lit_utf8_byte_t) (cp - (LIT_CHAR_LOWERCASE_A - LIT_CHAR_UPPERCASE_A));
    }

    if (builder_p != NULL)
    {
      ecma_stringbuilder_append_byte (builder_p, (lit_utf8_byte_t) cp);
    }

    return cp;
  }

#if JERRY_UNICODE_CASE_CONVERSION
  lit_code_point_t uppercase_cp = lit_search_in_bidirectional_conversion_tables (cp, false);

  if (uppercase_cp != LIT_INVALID_CP)
  {
    if (builder_p != NULL)
    {
      ecma_stringbuilder_append_codepoint (builder_p, uppercase_cp);
    }

    return uppercase_cp;
  }

  int num_of_upper_case_special_ranges = NUM_OF_ELEMENTS (lit_unicode_upper_case_special_ranges);

  for (int i = 0, j = 0; i < num_of_upper_case_special_ranges; i += 3, j++)
  {
    uint32_t range_length = lit_unicode_upper_case_special_range_lengths[j];
    ecma_char_t start_point = lit_unicode_upper_case_special_ranges[i];

    if (start_point <= cp && cp <= start_point + range_length)
    {
      if (builder_p != NULL)
      {
        uppercase_cp = lit_unicode_upper_case_special_ranges[i + 1] + (cp - start_point);
        ecma_stringbuilder_append_codepoint (builder_p, uppercase_cp);
        ecma_stringbuilder_append_codepoint (builder_p, lit_unicode_upper_case_special_ranges[i + 2]);
      }

      return LIT_MULTIPLE_CU;
    }
  }

  return lit_search_in_conversion_table ((ecma_char_t) cp,
                                         builder_p,
                                         lit_unicode_upper_case_conversions,
                                         lit_unicode_upper_case_conversion_counters);
#else /* !JERRY_UNICODE_CASE_CONVERSION */
  if (builder_p != NULL)
  {
    ecma_stringbuilder_append_codepoint (builder_p, cp);
  }

  return cp;
#endif /* JERRY_UNICODE_CASE_CONVERSION */
} /* lit_char_to_upper_case */

/*
 * Look up whether the character should be folded to the lowercase variant.
 *
 * @return true, if character should be lowercased
 *         false, otherwise
 */
bool
lit_char_fold_to_lower (lit_code_point_t cp) /**< code point */
{
#if JERRY_UNICODE_CASE_CONVERSION
  return (cp <= LIT_UTF8_1_BYTE_CODE_POINT_MAX || cp > LIT_UTF16_CODE_UNIT_MAX
          || (!lit_search_char_in_interval_array ((ecma_char_t) cp,
                                                  lit_unicode_folding_skip_to_lower_interval_starts,
                                                  lit_unicode_folding_skip_to_lower_interval_lengths,
                                                  NUM_OF_ELEMENTS (lit_unicode_folding_skip_to_lower_interval_starts))
              && !lit_search_char_in_array ((ecma_char_t) cp,
                                            lit_unicode_folding_skip_to_lower_chars,
                                            NUM_OF_ELEMENTS (lit_unicode_folding_skip_to_lower_chars))));
#else /* !JERRY_UNICODE_CASE_CONVERSION */
  return true;
#endif /* JERRY_UNICODE_CASE_CONVERSION */
} /* lit_char_fold_to_lower */

/*
 * Look up whether the character should be folded to the uppercase variant.
 *
 * @return true, if character should be uppercased
 *         false, otherwise
 */
bool
lit_char_fold_to_upper (lit_code_point_t cp) /**< code point */
{
#if JERRY_UNICODE_CASE_CONVERSION
  return (cp > LIT_UTF8_1_BYTE_CODE_POINT_MAX && cp <= LIT_UTF16_CODE_UNIT_MAX
          && (lit_search_char_in_interval_array ((ecma_char_t) cp,
                                                 lit_unicode_folding_to_upper_interval_starts,
                                                 lit_unicode_folding_to_upper_interval_lengths,
                                                 NUM_OF_ELEMENTS (lit_unicode_folding_to_upper_interval_starts))
              || lit_search_char_in_array ((ecma_char_t) cp,
                                           lit_unicode_folding_to_upper_chars,
                                           NUM_OF_ELEMENTS (lit_unicode_folding_to_upper_chars))));
#else /* !JERRY_UNICODE_CASE_CONVERSION */
  return false;
#endif /* JERRY_UNICODE_CASE_CONVERSION */
} /* lit_char_fold_to_upper */

/**
 * Helper method to find a specific character in a string
 *
 * Used by:
 *         ecma_builtin_string_prototype_object_replace_helper
 *
 * @return true - if the given character is in the string
 *         false - otherwise
 */
bool
lit_find_char_in_string (ecma_string_t *str_p, /**< source string */
                         lit_utf8_byte_t c) /**< character to find*/
{
  ECMA_STRING_TO_UTF8_STRING (str_p, start_p, start_size);

  const lit_utf8_byte_t *str_curr_p = start_p;
  const lit_utf8_byte_t *str_end_p = start_p + start_size;
  bool have_char = false;

  while (str_curr_p < str_end_p)
  {
    if (*str_curr_p++ == c)
    {
      have_char = true;
      break;
    }
  }

  ECMA_FINALIZE_UTF8_STRING (start_p, start_size);

  return have_char;
} /* lit_find_char_in_string */
