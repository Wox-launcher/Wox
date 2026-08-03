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

#ifndef ECMA_REGEXP_OBJECT_H
#define ECMA_REGEXP_OBJECT_H

#include "ecma-globals.h"

#include "re-compiler.h"

#if JERRY_BUILTIN_REGEXP

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaregexpobject ECMA RegExp object related routines
 * @{
 */

/**
 * RegExp flags
 * Note:
 *      This enum has to be kept in sync with jerry_regexp_flags_t.
 */
typedef enum
{
  RE_FLAG_EMPTY = 0u, /* Empty RegExp flags */
  RE_FLAG_GLOBAL = (1u << 1), /**< ECMA-262 v5, 15.10.7.2 */
  RE_FLAG_IGNORE_CASE = (1u << 2), /**< ECMA-262 v5, 15.10.7.3 */
  RE_FLAG_MULTILINE = (1u << 3), /**< ECMA-262 v5, 15.10.7.4 */
  RE_FLAG_STICKY = (1u << 4), /**< ECMA-262 v6, 21.2.5.12 */
  RE_FLAG_UNICODE = (1u << 5), /**< ECMA-262 v6, 21.2.5.15 */
  RE_FLAG_DOTALL = (1u << 6) /**< ECMA-262 v9, 21.2.5.3 */

  /* Bits from bit 13 is reserved for function types (see CBC_FUNCTION_TYPE_SHIFT). */
} ecma_regexp_flags_t;

/**
 * Class escapes
 */
typedef enum
{
  RE_ESCAPE__START, /**< escapes start */
  RE_ESCAPE_DIGIT = RE_ESCAPE__START, /**< digit */
  RE_ESCAPE_NOT_DIGIT, /**< not digit */
  RE_ESCAPE_WORD_CHAR, /**< word char */
  RE_ESCAPE_NOT_WORD_CHAR, /**< not word char */
  RE_ESCAPE_WHITESPACE, /**< whitespace */
  RE_ESCAPE_NOT_WHITESPACE, /**< not whitespace */
  RE_ESCAPE__COUNT, /**< escape count */
} ecma_class_escape_t;

/**
 * Character class flags escape count mask size.
 */
#define RE_CLASS_ESCAPE_COUNT_MASK_SIZE (3u)

/**
 * Character class flags escape count mask.
 */
#define RE_CLASS_ESCAPE_COUNT_MASK ((1 << RE_CLASS_ESCAPE_COUNT_MASK_SIZE) - 1u)

/**
 * Character class flags that are present in the upper bits of the class flags byte, while the 3 least significant bits
 * hold a value that contains the number of class escapes present in the character class.
 */
typedef enum
{
  RE_CLASS_HAS_CHARS = (1 << 5), /**< contains individual characters */
  RE_CLASS_HAS_RANGES = (1 << 6), /**< contains character ranges */
  RE_CLASS_INVERT = (1 << 7), /**< inverted */
} ecma_char_class_flags_t;

/**
 * Structure for matching capturing groups and storing their result
 */
typedef struct
{
  const lit_utf8_byte_t *begin_p; /**< capture start pointer */
  const lit_utf8_byte_t *end_p; /**< capture end pointer */
  const uint8_t *bc_p; /**< group bytecode pointer */
  uint32_t iterator; /**< iteration counter */
  uint32_t subcapture_count; /**< number of nested capturing groups */
} ecma_regexp_capture_t;

/**
 * Structure for matching non-capturing groups
 */
typedef struct
{
  const lit_utf8_byte_t *begin_p; /**< substring start pointer */
  const uint8_t *bc_p; /**< group bytecode pointer */
  uint32_t iterator; /**< iteration counter */
  uint32_t subcapture_start; /**< first nested capturing group index */
  uint32_t subcapture_count; /**< number of nested capturing groups */
} ecma_regexp_non_capture_t;

/**
 * Check if an ecma_regexp_capture_t contains a defined capture
 */
#define ECMA_RE_IS_CAPTURE_DEFINED(c) ((c)->begin_p != NULL)

ecma_value_t ecma_regexp_get_capture_value (const ecma_regexp_capture_t *const capture_p);

#if (JERRY_STACK_LIMIT != 0)
/**
 * Value used ase result when stack limit is reached
 */
#define ECMA_RE_OUT_OF_STACK ((const lit_utf8_byte_t *) UINTPTR_MAX)

/**
 * Checks if the stack limit has been reached during regexp matching
 */
#define ECMA_RE_STACK_LIMIT_REACHED(p) (JERRY_UNLIKELY (p == ECMA_RE_OUT_OF_STACK))
#else /* JERRY_STACK_LIMIT == 0 */
#define ECMA_RE_STACK_LIMIT_REACHED(p) (false)
#endif /* JERRY_STACK_LIMIT != 0 */

/**
 * Offset applied to qmax when encoded into the bytecode.
 *
 * It's common for qmax to be Infinity, which is represented a UINT32_MAX. By applying the offset we are able to store
 * it in a single byte az zero.
 */
#define RE_QMAX_OFFSET 1

/**
 * RegExp executor context
 */
typedef struct
{
  const lit_utf8_byte_t *input_start_p; /**< start of input string */
  const lit_utf8_byte_t *input_end_p; /**< end of input string */
  uint32_t captures_count; /**< number of capture groups */
  uint32_t non_captures_count; /**< number of non-capture groups */
  ecma_regexp_capture_t *captures_p; /**< capturing groups */
  ecma_regexp_non_capture_t *non_captures_p; /**< non-capturing groups */
  uint16_t flags; /**< RegExp flags */
  uint8_t char_size; /**< size of encoded characters */
} ecma_regexp_ctx_t;

/**
 * RegExpStringIterator object internal slots
 *
 * See also:
 *          ECMA-262 v11, 21.2.7.2
 */
typedef struct
{
  ecma_extended_object_t header; /**< extended object part */
  ecma_value_t iterating_regexp; /**< [[IteratingRegExp]] internal slot */
  ecma_value_t iterated_string; /**< [[IteratedString]] internal slot */
} ecma_regexp_string_iterator_t;

lit_code_point_t ecma_regexp_unicode_advance (const lit_utf8_byte_t **str_p, const lit_utf8_byte_t *end_p);
ecma_object_t *ecma_op_regexp_alloc (ecma_object_t *new_target_obj_p);
ecma_value_t ecma_regexp_exec_helper (ecma_object_t *regexp_object_p, ecma_string_t *input_string_p);
ecma_string_t *ecma_regexp_read_pattern_str_helper (ecma_value_t pattern_arg);
lit_code_point_t ecma_regexp_canonicalize_char (lit_code_point_t ch, bool unicode);
ecma_value_t ecma_regexp_parse_flags (ecma_string_t *flags_str_p, uint16_t *flags_p);
void ecma_regexp_create_and_initialize_props (ecma_object_t *re_object_p, ecma_string_t *source_p, uint16_t flags);
ecma_value_t ecma_regexp_replace_helper (ecma_value_t this_arg, ecma_value_t string_arg, ecma_value_t replace_arg);
ecma_value_t ecma_regexp_search_helper (ecma_value_t regexp_arg, ecma_value_t string_arg);
ecma_value_t ecma_regexp_split_helper (ecma_value_t this_arg, ecma_value_t string_arg, ecma_value_t limit_arg);
ecma_value_t ecma_regexp_match_helper (ecma_value_t this_arg, ecma_value_t string_arg);

ecma_value_t ecma_op_regexp_exec (ecma_value_t this_arg, ecma_string_t *str_p);

ecma_value_t ecma_op_create_regexp_from_bytecode (ecma_object_t *regexp_obj_p, re_compiled_code_t *bc_p);
ecma_value_t
ecma_op_create_regexp_from_pattern (ecma_object_t *regexp_obj_p, ecma_value_t pattern_value, ecma_value_t flags_value);
ecma_value_t ecma_op_create_regexp_with_flags (ecma_object_t *regexp_obj_p, ecma_value_t pattern_value, uint16_t flags);
/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
#endif /* !ECMA_REGEXP_OBJECT_H */
