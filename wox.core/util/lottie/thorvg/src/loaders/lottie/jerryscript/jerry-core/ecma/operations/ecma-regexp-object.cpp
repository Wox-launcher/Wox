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

#include "ecma-regexp-object.h"

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-builtin-helpers.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-objects.h"

#include "jcontext.h"
#include "jrt-libc-includes.h"
#include "lit-char-helpers.h"
#include "re-compiler.h"

#if JERRY_BUILTIN_REGEXP

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaregexpobject ECMA RegExp object related routines
 * @{
 */

/**
 * Index of the global capturing group
 */
#define RE_GLOBAL_CAPTURE 0

/**
 * Parse RegExp flags (global, ignoreCase, multiline)
 *
 * See also: ECMA-262 v5, 15.10.4.1
 *
 * @return empty ecma value - if parsed successfully
 *         error ecma value - otherwise
 *
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_regexp_parse_flags (ecma_string_t *flags_str_p, /**< Input string with flags */
                         uint16_t *flags_p) /**< [out] parsed flag bits */
{
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;
  uint16_t result_flags = RE_FLAG_EMPTY;

  ECMA_STRING_TO_UTF8_STRING (flags_str_p, flags_start_p, flags_start_size);

  const lit_utf8_byte_t *flags_str_curr_p = flags_start_p;
  const lit_utf8_byte_t *flags_str_end_p = flags_start_p + flags_start_size;

  while (flags_str_curr_p < flags_str_end_p)
  {
    ecma_regexp_flags_t flag;
    switch (*flags_str_curr_p++)
    {
      case 'g':
      {
        flag = RE_FLAG_GLOBAL;
        break;
      }
      case 'i':
      {
        flag = RE_FLAG_IGNORE_CASE;
        break;
      }
      case 'm':
      {
        flag = RE_FLAG_MULTILINE;
        break;
      }
      case 'y':
      {
        flag = RE_FLAG_STICKY;
        break;
      }
      case 'u':
      {
        flag = RE_FLAG_UNICODE;
        break;
      }
      case 's':
      {
        flag = RE_FLAG_DOTALL;
        break;
      }
      default:
      {
        flag = RE_FLAG_EMPTY;
        break;
      }
    }

    if (flag == RE_FLAG_EMPTY || (result_flags & flag) != 0)
    {
      ret_value = ecma_raise_syntax_error (ECMA_ERR_INVALID_REGEXP_FLAGS);
      break;
    }

    result_flags = (uint16_t) (result_flags | flag);
  }

  ECMA_FINALIZE_UTF8_STRING (flags_start_p, flags_start_size);

  *flags_p = result_flags;
  return ret_value;
} /* ecma_regexp_parse_flags */

/**
 * RegExpAlloc method
 *
 * See also: ECMA-262 v5, 15.10.4.1
 *           ECMA-262 v6, 21.2.3.2.1
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return ecma_object_t
 */
ecma_object_t *
ecma_op_regexp_alloc (ecma_object_t *ctr_obj_p) /**< constructor object pointer */
{
  if (ctr_obj_p == NULL)
  {
    ctr_obj_p = ecma_builtin_get (ECMA_BUILTIN_ID_REGEXP);
  }

  ecma_object_t *proto_obj_p = ecma_op_get_prototype_from_constructor (ctr_obj_p, ECMA_BUILTIN_ID_REGEXP_PROTOTYPE);

  if (JERRY_UNLIKELY (proto_obj_p == NULL))
  {
    return proto_obj_p;
  }

  ecma_object_t *new_object_p =
    ecma_create_object (proto_obj_p, sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_deref_object (proto_obj_p);

  ecma_extended_object_t *regexp_obj_p = (ecma_extended_object_t *) new_object_p;

  /* Class id will be initialized after the bytecode is compiled. */
  regexp_obj_p->u.cls.type = ECMA_OBJECT_CLASS__MAX;

  ecma_value_t status = ecma_builtin_helper_def_prop (new_object_p,
                                                      ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL),
                                                      ecma_make_uint32_value (0),
                                                      ECMA_PROPERTY_FLAG_WRITABLE | JERRY_PROP_SHOULD_THROW);

  JERRY_ASSERT (ecma_is_value_true (status));

  return new_object_p;
} /* ecma_op_regexp_alloc */

/**
 * Helper method for initializing an already existing RegExp object.
 */
static inline void 
ecma_op_regexp_initialize (ecma_object_t *regexp_obj_p, /**< RegExp object */
                           const re_compiled_code_t *bc_p) /**< bytecode */
{
  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) regexp_obj_p;
  ext_obj_p->u.cls.type = ECMA_OBJECT_CLASS_REGEXP;
  ECMA_SET_INTERNAL_VALUE_POINTER (ext_obj_p->u.cls.u3.value, bc_p);
} /* ecma_op_regexp_initialize */

/**
 * Method for creating a RegExp object from pattern.
 *
 * Note:
 *      Allocation have to happen before invoking this function using ecma_op_regexp_alloc.
 *
 * @return ecma_value_t
 */
ecma_value_t
ecma_op_create_regexp_from_pattern (ecma_object_t *regexp_obj_p, /**< RegExp object */
                                    ecma_value_t pattern_value, /**< pattern */
                                    ecma_value_t flags_value) /**< flags */
{
  ecma_string_t *pattern_str_p = ecma_regexp_read_pattern_str_helper (pattern_value);
  uint16_t flags = 0;

  if (JERRY_UNLIKELY (pattern_str_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  if (!ecma_is_value_undefined (flags_value))
  {
    ecma_string_t *flags_str_p = ecma_op_to_string (flags_value);

    if (JERRY_UNLIKELY (flags_str_p == NULL))
    {
      ecma_deref_ecma_string (pattern_str_p);
      return ECMA_VALUE_ERROR;
    }

    ecma_value_t parse_flags_value = ecma_regexp_parse_flags (flags_str_p, &flags);
    ecma_deref_ecma_string (flags_str_p);

    if (ECMA_IS_VALUE_ERROR (parse_flags_value))
    {
      ecma_deref_ecma_string (pattern_str_p);
      return parse_flags_value;
    }

    JERRY_ASSERT (ecma_is_value_empty (parse_flags_value));
  }

  re_compiled_code_t *bc_p = re_compile_bytecode (pattern_str_p, flags);

  if (JERRY_UNLIKELY (bc_p == NULL))
  {
    ecma_deref_ecma_string (pattern_str_p);
    return ECMA_VALUE_ERROR;
  }

  ecma_op_regexp_initialize (regexp_obj_p, bc_p);
  ecma_deref_ecma_string (pattern_str_p);

  return ecma_make_object_value (regexp_obj_p);
} /* ecma_op_create_regexp_from_pattern */

/**
 * Method for creating a RegExp object from bytecode.
 *
 * Note:
 *      Allocation have to happen before invoking this function using ecma_op_regexp_alloc.
 *
 * @return ecma_value_t
 */
ecma_value_t
ecma_op_create_regexp_from_bytecode (ecma_object_t *regexp_obj_p, /**< RegExp object */
                                     re_compiled_code_t *bc_p) /**< bytecode */
{
  ecma_bytecode_ref ((ecma_compiled_code_t *) bc_p);

  ecma_op_regexp_initialize (regexp_obj_p, bc_p);

  return ecma_make_object_value (regexp_obj_p);
} /* ecma_op_create_regexp_from_bytecode */

/**
 * Method for creating a RegExp object from pattern with already parsed flags.
 *
 * Note:
 *      Allocation have to happen before invoking this function using ecma_op_regexp_alloc.
 *
 * @return ecma_value_t
 */
ecma_value_t
ecma_op_create_regexp_with_flags (ecma_object_t *regexp_obj_p, /**< RegExp object */
                                  ecma_value_t pattern_value, /**< pattern */
                                  uint16_t flags) /**< flags */
{
  ecma_string_t *pattern_str_p = ecma_regexp_read_pattern_str_helper (pattern_value);

  if (JERRY_UNLIKELY (pattern_str_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  re_compiled_code_t *bc_p = re_compile_bytecode (pattern_str_p, flags);
  ecma_deref_ecma_string (pattern_str_p);

  if (JERRY_UNLIKELY (bc_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_op_regexp_initialize (regexp_obj_p, bc_p);

  return ecma_make_object_value (regexp_obj_p);
} /* ecma_op_create_regexp_with_flags */

/**
 * Canonicalize a character
 *
 * @return ecma_char_t canonicalized character
 */
lit_code_point_t
ecma_regexp_canonicalize_char (lit_code_point_t ch, /**< character */
                               bool unicode) /**< unicode */
{
  if (unicode)
  {
    /* In unicode mode the mappings contained in the CaseFolding.txt file should be used to canonicalize the character.
     * These mappings generally correspond to the lowercase variant of the character, however there are some
     * differences. In some cases the uppercase variant is used, in others the lowercase of the uppercase character is
     * used, and there are also cases where the character has no case folding mapping even though it has upper/lower
     * variants. Since lowercasing is the most common this is used as the default behaviour, and characters with
     * differing behaviours are encoded in lookup tables. */

    if (lit_char_fold_to_upper (ch))
    {
      ch = lit_char_to_upper_case (ch, NULL);
      JERRY_ASSERT (ch != LIT_MULTIPLE_CU);
    }

    if (lit_char_fold_to_lower (ch))
    {
      ch = lit_char_to_lower_case (ch, NULL);
      JERRY_ASSERT (ch != LIT_MULTIPLE_CU);
    }

    return ch;
  }

  JERRY_UNUSED (unicode);
  lit_code_point_t cu = lit_char_to_upper_case (ch, NULL);

  if (ch <= LIT_UTF8_1_BYTE_CODE_POINT_MAX || (cu > LIT_UTF8_1_BYTE_CODE_POINT_MAX && cu != LIT_MULTIPLE_CU))
  {
    return cu;
  }

  return ch;
} /* ecma_regexp_canonicalize_char */

/**
 * RegExp Canonicalize abstract operation
 *
 * See also: ECMA-262 v5, 15.10.2.8
 *
 * @return ecma_char_t canonicalized character
 */
static inline lit_code_point_t 
ecma_regexp_canonicalize (lit_code_point_t ch, /**< character */
                          uint16_t flags) /**< flags */
{
  if (flags & RE_FLAG_IGNORE_CASE)
  {
    return ecma_regexp_canonicalize_char (ch, flags & RE_FLAG_UNICODE);
  }

  return ch;
} /* ecma_regexp_canonicalize */

/**
 * Check if a code point is matched by a class escape.
 *
 * @return true, if code point matches escape
 *         false, otherwise
 */
static bool
ecma_regexp_check_class_escape (lit_code_point_t cp, /**< char */
                                ecma_class_escape_t escape) /**< escape */
{
  switch (escape)
  {
    case RE_ESCAPE_DIGIT:
    {
      return (cp >= LIT_CHAR_0 && cp <= LIT_CHAR_9);
    }
    case RE_ESCAPE_NOT_DIGIT:
    {
      return (cp < LIT_CHAR_0 || cp > LIT_CHAR_9);
    }
    case RE_ESCAPE_WORD_CHAR:
    {
      return lit_char_is_word_char (cp);
    }
    case RE_ESCAPE_NOT_WORD_CHAR:
    {
      return !lit_char_is_word_char (cp);
    }
    case RE_ESCAPE_WHITESPACE:
    {
      return lit_char_is_white_space ((ecma_char_t) cp);
    }
    case RE_ESCAPE_NOT_WHITESPACE:
    {
      return !lit_char_is_white_space ((ecma_char_t) cp);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_regexp_check_class_escape */

/**
 * Helper function to get current code point or code unit depending on execution mode,
 * and advance the string pointer.
 *
 * @return lit_code_point_t current code point
 */
static lit_code_point_t
ecma_regexp_advance (ecma_regexp_ctx_t *re_ctx_p, /**< regexp context */
                     const lit_utf8_byte_t **str_p) /**< reference to string pointer */
{
  JERRY_ASSERT (str_p != NULL);
  lit_code_point_t cp = lit_cesu8_read_next (str_p);

  if (JERRY_UNLIKELY (re_ctx_p->flags & RE_FLAG_UNICODE) && lit_is_code_point_utf16_high_surrogate ((ecma_char_t) cp)
      && *str_p < re_ctx_p->input_end_p)
  {
    const ecma_char_t next_ch = lit_cesu8_peek_next (*str_p);
    if (lit_is_code_point_utf16_low_surrogate (next_ch))
    {
      cp = lit_convert_surrogate_pair_to_code_point ((ecma_char_t) cp, next_ch);
      *str_p += LIT_UTF8_MAX_BYTES_IN_CODE_UNIT;
    }
  }

  return ecma_regexp_canonicalize (cp, re_ctx_p->flags);
} /* ecma_regexp_advance */

/**
 * Helper function to get current full unicode code point and advance the string pointer.
 *
 * @return lit_code_point_t current code point
 */
lit_code_point_t
ecma_regexp_unicode_advance (const lit_utf8_byte_t **str_p, /**< reference to string pointer */
                             const lit_utf8_byte_t *end_p) /**< string end pointer */
{
  JERRY_ASSERT (str_p != NULL);
  const lit_utf8_byte_t *current_p = *str_p;

  lit_code_point_t ch = lit_cesu8_read_next (&current_p);
  if (lit_is_code_point_utf16_high_surrogate ((ecma_char_t) ch) && current_p < end_p)
  {
    const ecma_char_t next_ch = lit_cesu8_peek_next (current_p);
    if (lit_is_code_point_utf16_low_surrogate (next_ch))
    {
      ch = lit_convert_surrogate_pair_to_code_point ((ecma_char_t) ch, next_ch);
      current_p += LIT_UTF8_MAX_BYTES_IN_CODE_UNIT;
    }
  }

  *str_p = current_p;
  return ch;
} /* ecma_regexp_unicode_advance */

/**
 * Helper function to revert the string pointer to the previous code point.
 *
 * @return pointer to previous code point
 */
static JERRY_ATTR_NOINLINE const lit_utf8_byte_t *
ecma_regexp_step_back (ecma_regexp_ctx_t *re_ctx_p, /**< regexp context */
                       const lit_utf8_byte_t *str_p) /**< reference to string pointer */
{
  JERRY_ASSERT (str_p != NULL);

  lit_code_point_t ch = lit_cesu8_read_prev (&str_p);
  if (JERRY_UNLIKELY (re_ctx_p->flags & RE_FLAG_UNICODE) && lit_is_code_point_utf16_low_surrogate (ch)
      && lit_is_code_point_utf16_high_surrogate (lit_cesu8_peek_prev (str_p)))
  {
    str_p -= LIT_UTF8_MAX_BYTES_IN_CODE_UNIT;
  }

  return str_p;
} /* ecma_regexp_step_back */

/**
 * Check if the current position is on a word boundary.
 *
 * @return true, if on a word boundary
 *         false - otherwise
 */
static bool
ecma_regexp_is_word_boundary (ecma_regexp_ctx_t *re_ctx_p, /**< regexp context */
                              const lit_utf8_byte_t *str_p) /**< string pointer */
{
  lit_code_point_t left_cp;
  lit_code_point_t right_cp;

  if (JERRY_UNLIKELY (str_p <= re_ctx_p->input_start_p))
  {
    left_cp = LIT_INVALID_CP;
  }
  else if (JERRY_UNLIKELY ((re_ctx_p->flags & (RE_FLAG_UNICODE | RE_FLAG_IGNORE_CASE))
                           == (RE_FLAG_UNICODE | RE_FLAG_IGNORE_CASE)))
  {
    const lit_utf8_byte_t *prev_p = ecma_regexp_step_back (re_ctx_p, str_p);
    left_cp = ecma_regexp_advance (re_ctx_p, &prev_p);
    JERRY_ASSERT (prev_p == str_p);
  }
  else
  {
    left_cp = str_p[-1];
  }

  if (JERRY_UNLIKELY (str_p >= re_ctx_p->input_end_p))
  {
    right_cp = LIT_INVALID_CP;
  }
  else if (JERRY_UNLIKELY ((re_ctx_p->flags & (RE_FLAG_UNICODE | RE_FLAG_IGNORE_CASE))
                           == (RE_FLAG_UNICODE | RE_FLAG_IGNORE_CASE)))
  {
    right_cp = ecma_regexp_advance (re_ctx_p, &str_p);
  }
  else
  {
    right_cp = str_p[0];
  }

  return lit_char_is_word_char (left_cp) != lit_char_is_word_char (right_cp);
} /* ecma_regexp_is_word_boundary */

/**
 * Recursive function for executing RegExp bytecode.
 *
 * See also:
 *          ECMA-262 v5, 15.10.2.1
 *
 * @return pointer to the end of the currently matched substring
 *         NULL, if pattern did not match
 */
static const lit_utf8_byte_t *
ecma_regexp_run (ecma_regexp_ctx_t *re_ctx_p, /**< RegExp matcher context */
                 const uint8_t *bc_p, /**< pointer to the current RegExp bytecode */
                 const lit_utf8_byte_t *str_curr_p) /**< input string pointer */
{
#if (JERRY_STACK_LIMIT != 0)
  if (JERRY_UNLIKELY (ecma_get_current_stack_usage () > CONFIG_MEM_STACK_LIMIT))
  {
    return ECMA_RE_OUT_OF_STACK;
  }
#endif /* JERRY_STACK_LIMIT != 0 */

  const lit_utf8_byte_t *str_start_p = str_curr_p;
  const uint8_t *next_alternative_p = NULL;

  while (true)
  {
    const re_opcode_t op = re_get_opcode (&bc_p);

    switch (op)
    {
      case RE_OP_EOF:
      {
        re_ctx_p->captures_p[RE_GLOBAL_CAPTURE].end_p = str_curr_p;
        /* FALLTHRU */
      }
      case RE_OP_ASSERT_END:
      case RE_OP_ITERATOR_END:
      {
        return str_curr_p;
      }
      case RE_OP_ALTERNATIVE_START:
      {
        const uint32_t offset = re_get_value (&bc_p);
        next_alternative_p = bc_p + offset;
        continue;
      }
      case RE_OP_ALTERNATIVE_NEXT:
      {
        while (true)
        {
          const uint32_t offset = re_get_value (&bc_p);
          bc_p += offset;

          if (*bc_p != RE_OP_ALTERNATIVE_NEXT)
          {
            break;
          }

          bc_p++;
        }

        continue;
      }
      case RE_OP_NO_ALTERNATIVE:
      {
        return NULL;
      }
      case RE_OP_CAPTURING_GROUP_START:
      {
        const uint32_t group_idx = re_get_value (&bc_p);
        ecma_regexp_capture_t *const group_p = re_ctx_p->captures_p + group_idx;
        group_p->subcapture_count = re_get_value (&bc_p);

        const lit_utf8_byte_t *const saved_begin_p = group_p->begin_p;
        const lit_utf8_byte_t *const saved_end_p = group_p->end_p;
        const uint32_t saved_iterator = group_p->iterator;

        const uint32_t qmin = re_get_value (&bc_p);
        group_p->end_p = NULL;

        /* If zero iterations are allowed, then execute the end opcode which will handle further iterations,
         * otherwise run the 1st iteration immediately by executing group bytecode. */
        if (qmin == 0)
        {
          group_p->iterator = 0;
          group_p->begin_p = NULL;
          const uint32_t end_offset = re_get_value (&bc_p);
          group_p->bc_p = bc_p;

          bc_p += end_offset;
        }
        else
        {
          group_p->iterator = 1;
          group_p->begin_p = str_curr_p;
          group_p->bc_p = bc_p;
        }

        const lit_utf8_byte_t *matched_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);
        group_p->iterator = saved_iterator;

        if (matched_p == NULL)
        {
          group_p->begin_p = saved_begin_p;
          group_p->end_p = saved_end_p;
          goto fail;
        }

        return matched_p;
      }
      case RE_OP_NON_CAPTURING_GROUP_START:
      {
        const uint32_t group_idx = re_get_value (&bc_p);
        ecma_regexp_non_capture_t *const group_p = re_ctx_p->non_captures_p + group_idx;

        group_p->subcapture_start = re_get_value (&bc_p);
        group_p->subcapture_count = re_get_value (&bc_p);

        const lit_utf8_byte_t *const saved_begin_p = group_p->begin_p;
        const uint32_t saved_iterator = group_p->iterator;
        const uint32_t qmin = re_get_value (&bc_p);

        /* If zero iterations are allowed, then execute the end opcode which will handle further iterations,
         * otherwise run the 1st iteration immediately by executing group bytecode. */
        if (qmin == 0)
        {
          group_p->iterator = 0;
          group_p->begin_p = NULL;
          const uint32_t end_offset = re_get_value (&bc_p);
          group_p->bc_p = bc_p;

          bc_p += end_offset;
        }
        else
        {
          group_p->iterator = 1;
          group_p->begin_p = str_curr_p;
          group_p->bc_p = bc_p;
        }

        const lit_utf8_byte_t *matched_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);
        group_p->iterator = saved_iterator;

        if (matched_p == NULL)
        {
          group_p->begin_p = saved_begin_p;
          goto fail;
        }

        return matched_p;
      }
      case RE_OP_GREEDY_CAPTURING_GROUP_END:
      {
        const uint32_t group_idx = re_get_value (&bc_p);
        ecma_regexp_capture_t *const group_p = re_ctx_p->captures_p + group_idx;
        const uint32_t qmin = re_get_value (&bc_p);

        if (group_p->iterator < qmin)
        {
          /* No need to save begin_p since we don't have to backtrack beyond the minimum iteration count, but we have
           * to clear nested capturing groups. */
          group_p->begin_p = str_curr_p;
          for (uint32_t i = 1; i < group_p->subcapture_count; ++i)
          {
            group_p[i].begin_p = NULL;
          }

          group_p->iterator++;
          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, group_p->bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          group_p->iterator--;
          goto fail;
        }

        /* Empty matches are not allowed after reaching the minimum number of iterations. */
        if (JERRY_UNLIKELY (group_p->begin_p >= str_curr_p) && (group_p->iterator > qmin))
        {
          goto fail;
        }

        const uint32_t qmax = re_get_value (&bc_p) - RE_QMAX_OFFSET;
        if (JERRY_UNLIKELY (group_p->iterator >= qmax))
        {
          /* Reached maximum number of iterations, try to match tail bytecode. */
          group_p->end_p = str_curr_p;
          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          goto fail;
        }

        {
          /* Save and clear all nested capturing groups, and try to iterate. */
          JERRY_VLA (const lit_utf8_byte_t *, saved_captures_p, group_p->subcapture_count);
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            saved_captures_p[i] = group_p[i].begin_p;
            group_p[i].begin_p = NULL;
          }

          group_p->iterator++;
          group_p->begin_p = str_curr_p;

          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, group_p->bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          /* Failed to iterate again, backtrack to current match, and try to run tail bytecode. */
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            group_p[i].begin_p = saved_captures_p[i];
          }

          group_p->iterator--;
          group_p->end_p = str_curr_p;
        }

        const lit_utf8_byte_t *const tail_match_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

        if (tail_match_p != NULL)
        {
          return tail_match_p;
        }

        goto fail;
      }
      case RE_OP_GREEDY_NON_CAPTURING_GROUP_END:
      {
        const uint32_t group_idx = re_get_value (&bc_p);
        ecma_regexp_non_capture_t *const group_p = re_ctx_p->non_captures_p + group_idx;
        const uint32_t qmin = re_get_value (&bc_p);

        if (group_p->iterator < qmin)
        {
          /* No need to save begin_p but we have to clear nested capturing groups. */
          group_p->begin_p = str_curr_p;

          ecma_regexp_capture_t *const capture_p = re_ctx_p->captures_p + group_p->subcapture_start;
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            capture_p[i].begin_p = NULL;
          }

          group_p->iterator++;
          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, group_p->bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          group_p->iterator--;
          goto fail;
        }

        /* Empty matches are not allowed after reaching the minimum number of iterations. */
        if (JERRY_UNLIKELY (group_p->begin_p >= str_curr_p) && (group_p->iterator > qmin))
        {
          goto fail;
        }

        const uint32_t qmax = re_get_value (&bc_p) - RE_QMAX_OFFSET;
        if (JERRY_UNLIKELY (group_p->iterator >= qmax))
        {
          /* Reached maximum number of iterations, try to match tail bytecode. */
          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          goto fail;
        }

        {
          /* Save and clear all nested capturing groups, and try to iterate. */
          JERRY_VLA (const lit_utf8_byte_t *, saved_captures_p, group_p->subcapture_count);
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            ecma_regexp_capture_t *const capture_p = re_ctx_p->captures_p + group_p->subcapture_start + i;
            saved_captures_p[i] = capture_p->begin_p;
            capture_p->begin_p = NULL;
          }

          group_p->iterator++;
          const lit_utf8_byte_t *const saved_begin_p = group_p->begin_p;
          group_p->begin_p = str_curr_p;

          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, group_p->bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          /* Failed to iterate again, backtrack to current match, and try to run tail bytecode. */
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            ecma_regexp_capture_t *const capture_p = re_ctx_p->captures_p + group_p->subcapture_start + i;
            capture_p->begin_p = saved_captures_p[i];
          }

          group_p->iterator--;
          group_p->begin_p = saved_begin_p;
        }

        const lit_utf8_byte_t *const tail_match_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

        if (tail_match_p != NULL)
        {
          return tail_match_p;
        }

        goto fail;
      }
      case RE_OP_LAZY_CAPTURING_GROUP_END:
      {
        const uint32_t group_idx = re_get_value (&bc_p);
        ecma_regexp_capture_t *const group_p = re_ctx_p->captures_p + group_idx;
        const uint32_t qmin = re_get_value (&bc_p);

        if (group_p->iterator < qmin)
        {
          /* No need to save begin_p but we have to clear nested capturing groups. */
          group_p->begin_p = str_curr_p;
          for (uint32_t i = 1; i < group_p->subcapture_count; ++i)
          {
            group_p[i].begin_p = NULL;
          }

          group_p->iterator++;
          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, group_p->bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          group_p->iterator--;
          goto fail;
        }

        /* Empty matches are not allowed after reaching the minimum number of iterations. */
        if (JERRY_UNLIKELY (group_p->begin_p >= str_curr_p) && (group_p->iterator > qmin))
        {
          goto fail;
        }

        const uint32_t qmax = re_get_value (&bc_p) - RE_QMAX_OFFSET;
        group_p->end_p = str_curr_p;

        /* Try to match tail bytecode. */
        const lit_utf8_byte_t *const tail_match_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

        if (tail_match_p != NULL)
        {
          return tail_match_p;
        }

        if (JERRY_UNLIKELY (group_p->iterator >= qmax))
        {
          /* Reached maximum number of iterations and tail bytecode did not match. */
          goto fail;
        }

        {
          /* Save and clear all nested capturing groups, and try to iterate. */
          JERRY_VLA (const lit_utf8_byte_t *, saved_captures_p, group_p->subcapture_count);
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            saved_captures_p[i] = group_p[i].begin_p;
            group_p[i].begin_p = NULL;
          }

          group_p->iterator++;
          group_p->begin_p = str_curr_p;

          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, group_p->bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          /* Backtrack to current match. */
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            group_p[i].begin_p = saved_captures_p[i];
          }

          group_p->iterator--;
        }

        goto fail;
      }
      case RE_OP_LAZY_NON_CAPTURING_GROUP_END:
      {
        const uint32_t group_idx = re_get_value (&bc_p);
        ecma_regexp_non_capture_t *const group_p = re_ctx_p->non_captures_p + group_idx;
        const uint32_t qmin = re_get_value (&bc_p);

        if (group_p->iterator < qmin)
        {
          /* Clear nested captures. */
          ecma_regexp_capture_t *const capture_p = re_ctx_p->captures_p + group_p->subcapture_start;
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            capture_p[i].begin_p = NULL;
          }

          group_p->iterator++;
          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, group_p->bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          group_p->iterator--;
          goto fail;
        }

        /* Empty matches are not allowed after reaching the minimum number of iterations. */
        if (JERRY_UNLIKELY (group_p->begin_p >= str_curr_p) && (group_p->iterator > qmin))
        {
          goto fail;
        }

        const uint32_t qmax = re_get_value (&bc_p) - RE_QMAX_OFFSET;

        /* Try to match tail bytecode. */
        const lit_utf8_byte_t *const tail_match_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

        if (tail_match_p != NULL)
        {
          return tail_match_p;
        }

        if (JERRY_UNLIKELY (group_p->iterator >= qmax))
        {
          /* Reached maximum number of iterations and tail bytecode did not match. */
          goto fail;
        }

        {
          /* Save and clear all nested capturing groups, and try to iterate. */
          JERRY_VLA (const lit_utf8_byte_t *, saved_captures_p, group_p->subcapture_count);
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            ecma_regexp_capture_t *const capture_p = re_ctx_p->captures_p + group_p->subcapture_start + i;
            saved_captures_p[i] = capture_p->begin_p;
            capture_p->begin_p = NULL;
          }

          group_p->iterator++;
          const lit_utf8_byte_t *const saved_begin_p = group_p->begin_p;
          group_p->begin_p = str_curr_p;

          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, group_p->bc_p, str_curr_p);

          if (matched_p != NULL)
          {
            return matched_p;
          }

          /* Backtrack to current match. */
          for (uint32_t i = 0; i < group_p->subcapture_count; ++i)
          {
            ecma_regexp_capture_t *const capture_p = re_ctx_p->captures_p + group_p->subcapture_start + i;
            capture_p->begin_p = saved_captures_p[i];
          }

          group_p->iterator--;
          group_p->begin_p = saved_begin_p;
        }

        goto fail;
      }
      case RE_OP_GREEDY_ITERATOR:
      {
        const uint32_t qmin = re_get_value (&bc_p);
        const uint32_t qmax = re_get_value (&bc_p) - RE_QMAX_OFFSET;
        const uint32_t end_offset = re_get_value (&bc_p);

        uint32_t iterator = 0;
        while (iterator < qmin)
        {
          str_curr_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

          if (str_curr_p == NULL)
          {
            goto fail;
          }

          if (ECMA_RE_STACK_LIMIT_REACHED (str_curr_p))
          {
            return str_curr_p;
          }

          iterator++;
        }

        while (iterator < qmax)
        {
          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

          if (matched_p == NULL)
          {
            break;
          }

          if (ECMA_RE_STACK_LIMIT_REACHED (str_curr_p))
          {
            return str_curr_p;
          }

          str_curr_p = matched_p;
          iterator++;
        }

        const uint8_t *const tail_bc_p = bc_p + end_offset;
        while (true)
        {
          const lit_utf8_byte_t *const tail_match_p = ecma_regexp_run (re_ctx_p, tail_bc_p, str_curr_p);

          if (tail_match_p != NULL)
          {
            return tail_match_p;
          }

          if (JERRY_UNLIKELY (iterator <= qmin))
          {
            goto fail;
          }

          iterator--;
          JERRY_ASSERT (str_curr_p > re_ctx_p->input_start_p);
          str_curr_p = ecma_regexp_step_back (re_ctx_p, str_curr_p);
        }

        JERRY_UNREACHABLE ();
      }
      case RE_OP_LAZY_ITERATOR:
      {
        const uint32_t qmin = re_get_value (&bc_p);
        const uint32_t qmax = re_get_value (&bc_p) - RE_QMAX_OFFSET;
        const uint32_t end_offset = re_get_value (&bc_p);

        uint32_t iterator = 0;
        while (iterator < qmin)
        {
          str_curr_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

          if (str_curr_p == NULL)
          {
            goto fail;
          }

          if (ECMA_RE_STACK_LIMIT_REACHED (str_curr_p))
          {
            return str_curr_p;
          }

          iterator++;
        }

        const uint8_t *const tail_bc_p = bc_p + end_offset;
        while (true)
        {
          const lit_utf8_byte_t *const tail_match_p = ecma_regexp_run (re_ctx_p, tail_bc_p, str_curr_p);

          if (tail_match_p != NULL)
          {
            return tail_match_p;
          }

          if (JERRY_UNLIKELY (iterator >= qmax))
          {
            goto fail;
          }

          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

          if (matched_p == NULL)
          {
            goto fail;
          }

          if (ECMA_RE_STACK_LIMIT_REACHED (matched_p))
          {
            return matched_p;
          }

          iterator++;
          str_curr_p = matched_p;
        }

        JERRY_UNREACHABLE ();
      }
      case RE_OP_BACKREFERENCE:
      {
        const uint32_t backref_idx = re_get_value (&bc_p);
        JERRY_ASSERT (backref_idx >= 1 && backref_idx < re_ctx_p->captures_count);
        const ecma_regexp_capture_t *capture_p = re_ctx_p->captures_p + backref_idx;

        if (!ECMA_RE_IS_CAPTURE_DEFINED (capture_p) || capture_p->end_p <= capture_p->begin_p)
        {
          /* Undefined or zero length captures always match. */
          continue;
        }

        const lit_utf8_size_t capture_size = (lit_utf8_size_t) (capture_p->end_p - capture_p->begin_p);

        if (str_curr_p + capture_size > re_ctx_p->input_end_p || memcmp (str_curr_p, capture_p->begin_p, capture_size))
        {
          goto fail;
        }

        str_curr_p += capture_size;
        continue;
      }
      case RE_OP_ASSERT_LINE_START:
      {
        if (str_curr_p <= re_ctx_p->input_start_p)
        {
          continue;
        }

        if (!(re_ctx_p->flags & RE_FLAG_MULTILINE) || !lit_char_is_line_terminator (lit_cesu8_peek_prev (str_curr_p)))
        {
          goto fail;
        }

        continue;
      }
      case RE_OP_ASSERT_LINE_END:
      {
        if (str_curr_p >= re_ctx_p->input_end_p)
        {
          continue;
        }

        if (!(re_ctx_p->flags & RE_FLAG_MULTILINE) || !lit_char_is_line_terminator (lit_cesu8_peek_next (str_curr_p)))
        {
          goto fail;
        }

        continue;
      }
      case RE_OP_ASSERT_WORD_BOUNDARY:
      {
        if (!ecma_regexp_is_word_boundary (re_ctx_p, str_curr_p))
        {
          goto fail;
        }

        continue;
      }
      case RE_OP_ASSERT_NOT_WORD_BOUNDARY:
      {
        if (ecma_regexp_is_word_boundary (re_ctx_p, str_curr_p))
        {
          goto fail;
        }

        continue;
      }
      case RE_OP_ASSERT_LOOKAHEAD_POS:
      {
        const uint8_t qmin = re_get_byte (&bc_p);
        const uint32_t capture_start = re_get_value (&bc_p);
        const uint32_t capture_count = re_get_value (&bc_p);
        const uint32_t end_offset = re_get_value (&bc_p);

        /* If qmin is zero, the assertion implicitly matches. */
        if (qmin == 0)
        {
          bc_p += end_offset;
          continue;
        }

        /* Capture end pointers might get clobbered and need to be restored after a tail match fail. */
        JERRY_VLA (const lit_utf8_byte_t *, saved_captures_p, capture_count);
        for (uint32_t i = 0; i < capture_count; ++i)
        {
          ecma_regexp_capture_t *const capture_p = re_ctx_p->captures_p + capture_start + i;
          saved_captures_p[i] = capture_p->end_p;
        }

        /* The first iteration will decide whether the assertion matches depending on whether
         * the iteration matched or not. */
        const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

        if (ECMA_RE_STACK_LIMIT_REACHED (matched_p))
        {
          return matched_p;
        }

        if (matched_p == NULL)
        {
          goto fail;
        }

        const lit_utf8_byte_t *tail_match_p = ecma_regexp_run (re_ctx_p, bc_p + end_offset, str_curr_p);

        if (tail_match_p == NULL)
        {
          for (uint32_t i = 0; i < capture_count; ++i)
          {
            ecma_regexp_capture_t *const capture_p = re_ctx_p->captures_p + capture_start + i;
            capture_p->begin_p = NULL;
            capture_p->end_p = saved_captures_p[i];
          }

          goto fail;
        }

        return tail_match_p;
      }
      case RE_OP_ASSERT_LOOKAHEAD_NEG:
      {
        const uint8_t qmin = re_get_byte (&bc_p);
        uint32_t capture_idx = re_get_value (&bc_p);
        const uint32_t capture_count = re_get_value (&bc_p);
        const uint32_t end_offset = re_get_value (&bc_p);

        /* If qmin is zero, the assertion implicitly matches. */
        if (qmin > 0)
        {
          /* The first iteration will decide whether the assertion matches depending on whether
           * the iteration matched or not. */
          const lit_utf8_byte_t *const matched_p = ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);

          if (ECMA_RE_STACK_LIMIT_REACHED (matched_p))
          {
            return matched_p;
          }

          if (matched_p != NULL)
          {
            /* Nested capturing groups inside a negative lookahead can never capture, so we clear their results. */
            const uint32_t capture_end = capture_idx + capture_count;
            while (capture_idx < capture_end)
            {
              re_ctx_p->captures_p[capture_idx++].begin_p = NULL;
            }

            goto fail;
          }
        }

        bc_p += end_offset;
        continue;
      }
      case RE_OP_CLASS_ESCAPE:
      {
        if (str_curr_p >= re_ctx_p->input_end_p)
        {
          goto fail;
        }

        const lit_code_point_t cp = ecma_regexp_advance (re_ctx_p, &str_curr_p);

        const ecma_class_escape_t escape = (ecma_class_escape_t) re_get_byte (&bc_p);
        if (!ecma_regexp_check_class_escape (cp, escape))
        {
          goto fail;
        }

        continue;
      }
      case RE_OP_CHAR_CLASS:
      {
        if (str_curr_p >= re_ctx_p->input_end_p)
        {
          goto fail;
        }

        uint8_t flags = re_get_byte (&bc_p);
        uint32_t char_count = (flags & RE_CLASS_HAS_CHARS) ? re_get_value (&bc_p) : 0;
        uint32_t range_count = (flags & RE_CLASS_HAS_RANGES) ? re_get_value (&bc_p) : 0;

        const lit_code_point_t cp = ecma_regexp_advance (re_ctx_p, &str_curr_p);

        uint8_t escape_count = flags & RE_CLASS_ESCAPE_COUNT_MASK;
        while (escape_count > 0)
        {
          escape_count--;
          const ecma_class_escape_t escape = (ecma_class_escape_t) re_get_byte (&bc_p);
          if (ecma_regexp_check_class_escape (cp, escape))
          {
            goto class_found;
          }
        }

        while (char_count > 0)
        {
          char_count--;
          const lit_code_point_t curr = re_get_char (&bc_p, re_ctx_p->flags & RE_FLAG_UNICODE);
          if (cp == curr)
          {
            goto class_found;
          }
        }

        while (range_count > 0)
        {
          range_count--;
          const lit_code_point_t begin = re_get_char (&bc_p, re_ctx_p->flags & RE_FLAG_UNICODE);

          if (cp < begin)
          {
            bc_p += re_ctx_p->char_size;
            continue;
          }

          const lit_code_point_t end = re_get_char (&bc_p, re_ctx_p->flags & RE_FLAG_UNICODE);
          if (cp <= end)
          {
            goto class_found;
          }
        }

        /* Not found */
        if (flags & RE_CLASS_INVERT)
        {
          continue;
        }

        goto fail;

class_found:
        if (flags & RE_CLASS_INVERT)
        {
          goto fail;
        }

        const uint32_t chars_size = char_count * re_ctx_p->char_size;
        const uint32_t ranges_size = range_count * re_ctx_p->char_size * 2;
        bc_p = bc_p + escape_count + chars_size + ranges_size;
        continue;
      }
      case RE_OP_UNICODE_PERIOD:
      {
        if (str_curr_p >= re_ctx_p->input_end_p)
        {
          goto fail;
        }

        const lit_code_point_t cp = ecma_regexp_unicode_advance (&str_curr_p, re_ctx_p->input_end_p);

        if (!(re_ctx_p->flags & RE_FLAG_DOTALL)
            && JERRY_UNLIKELY (cp <= LIT_UTF16_CODE_UNIT_MAX && lit_char_is_line_terminator ((ecma_char_t) cp)))
        {
          goto fail;
        }

        continue;
      }
      case RE_OP_PERIOD:
      {
        if (str_curr_p >= re_ctx_p->input_end_p)
        {
          goto fail;
        }

        const ecma_char_t ch = lit_cesu8_read_next (&str_curr_p);

        if (!(re_ctx_p->flags & RE_FLAG_DOTALL) && lit_char_is_line_terminator (ch))
        {
          goto fail;
        }

        continue;
      }
      case RE_OP_CHAR:
      {
        if (str_curr_p >= re_ctx_p->input_end_p)
        {
          goto fail;
        }

        const lit_code_point_t ch1 = re_get_char (&bc_p, re_ctx_p->flags & RE_FLAG_UNICODE);
        const lit_code_point_t ch2 = ecma_regexp_advance (re_ctx_p, &str_curr_p);

        if (ch1 != ch2)
        {
          goto fail;
        }

        continue;
      }
      default:
      {
        JERRY_ASSERT (op == RE_OP_BYTE);

        if (str_curr_p >= re_ctx_p->input_end_p || *bc_p++ != *str_curr_p++)
        {
          goto fail;
        }

        continue;
      }
    }

    JERRY_UNREACHABLE ();
fail:
    bc_p = next_alternative_p;

    if (bc_p == NULL || *bc_p++ != RE_OP_ALTERNATIVE_NEXT)
    {
      /* None of the alternatives matched. */
      return NULL;
    }

    /* Get the end of the new alternative and continue execution. */
    str_curr_p = str_start_p;
    const uint32_t offset = re_get_value (&bc_p);
    next_alternative_p = bc_p + offset;
  }
} /* ecma_regexp_run */

/**
 * Match a RegExp at a specific position in the input string.
 *
 * @return pointer to the end of the matched sub-string
 *         NULL, if pattern did not match
 */
static const lit_utf8_byte_t *
ecma_regexp_match (ecma_regexp_ctx_t *re_ctx_p, /**< RegExp matcher context */
                   const uint8_t *bc_p, /**< pointer to the current RegExp bytecode */
                   const lit_utf8_byte_t *str_curr_p) /**< input string pointer */
{
  re_ctx_p->captures_p[RE_GLOBAL_CAPTURE].begin_p = str_curr_p;

  for (uint32_t i = 1; i < re_ctx_p->captures_count; ++i)
  {
    re_ctx_p->captures_p[i].begin_p = NULL;
  }

  return ecma_regexp_run (re_ctx_p, bc_p, str_curr_p);
} /* ecma_regexp_match */

/*
 * Helper function to get the result of a capture
 *
 * @return string value, if capture is defined
 *         undefined, otherwise
 */
ecma_value_t
ecma_regexp_get_capture_value (const ecma_regexp_capture_t *const capture_p) /**< capture */
{
  if (ECMA_RE_IS_CAPTURE_DEFINED (capture_p))
  {
    JERRY_ASSERT (capture_p->end_p >= capture_p->begin_p);
    const lit_utf8_size_t capture_size = (lit_utf8_size_t) (capture_p->end_p - capture_p->begin_p);
    ecma_string_t *const capture_str_p = ecma_new_ecma_string_from_utf8 (capture_p->begin_p, capture_size);
    return ecma_make_string_value (capture_str_p);
  }

  return ECMA_VALUE_UNDEFINED;
} /* ecma_regexp_get_capture_value */

/**
 * Helper function to create a result array from the captures in a regexp context
 *
 * @return ecma value containing the created array object
 */
static ecma_value_t
ecma_regexp_create_result_object (ecma_regexp_ctx_t *re_ctx_p, /**< regexp context */
                                  ecma_string_t *input_string_p, /**< input ecma string */
                                  uint32_t index) /**< match index */
{
  ecma_object_t *result_p = ecma_op_new_array_object (0);

  for (uint32_t i = 0; i < re_ctx_p->captures_count; i++)
  {
    ecma_value_t capture_value = ecma_regexp_get_capture_value (re_ctx_p->captures_p + i);
    ecma_builtin_helper_def_prop_by_index (result_p, i, capture_value, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
    ecma_free_value (capture_value);
  }

  ecma_builtin_helper_def_prop (result_p,
                                ecma_get_magic_string (LIT_MAGIC_STRING_INDEX),
                                ecma_make_uint32_value (index),
                                ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);

  ecma_builtin_helper_def_prop (result_p,
                                ecma_get_magic_string (LIT_MAGIC_STRING_INPUT),
                                ecma_make_string_value (input_string_p),
                                ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);

  return ecma_make_object_value (result_p);
} /* ecma_regexp_create_result_object */

/**
 * Helper function to initialize a regexp match context
 */
static void
ecma_regexp_initialize_context (ecma_regexp_ctx_t *ctx_p, /**< regexp context */
                                const re_compiled_code_t *bc_p, /**< regexp bytecode */
                                const lit_utf8_byte_t *input_start_p, /**< pointer to input string */
                                const lit_utf8_byte_t *input_end_p) /**< pointer to end of input string */
{
  JERRY_ASSERT (ctx_p != NULL);
  JERRY_ASSERT (bc_p != NULL);
  JERRY_ASSERT (input_start_p != NULL);
  JERRY_ASSERT (input_end_p >= input_start_p);

  ctx_p->flags = bc_p->header.status_flags;
  ctx_p->char_size = (ctx_p->flags & RE_FLAG_UNICODE) ? sizeof (lit_code_point_t) : sizeof (ecma_char_t);

  ctx_p->input_start_p = input_start_p;
  ctx_p->input_end_p = input_end_p;

  ctx_p->captures_count = bc_p->captures_count;
  ctx_p->non_captures_count = bc_p->non_captures_count;

  ctx_p->captures_p = (ecma_regexp_capture_t *) jmem_heap_alloc_block (ctx_p->captures_count * sizeof (ecma_regexp_capture_t));

  if (ctx_p->non_captures_count > 0)
  {
    ctx_p->non_captures_p = (ecma_regexp_non_capture_t *) jmem_heap_alloc_block (ctx_p->non_captures_count * sizeof (ecma_regexp_non_capture_t));
  }
} /* ecma_regexp_initialize_context */

/**
 * Helper function to clean up a regexp context
 */
static void
ecma_regexp_cleanup_context (ecma_regexp_ctx_t *ctx_p) /**< regexp context */
{
  JERRY_ASSERT (ctx_p != NULL);
  jmem_heap_free_block (ctx_p->captures_p, ctx_p->captures_count * sizeof (ecma_regexp_capture_t));

  if (ctx_p->non_captures_count > 0)
  {
    jmem_heap_free_block (ctx_p->non_captures_p, ctx_p->non_captures_count * sizeof (ecma_regexp_non_capture_t));
  }
} /* ecma_regexp_cleanup_context */

/**
 * RegExp helper function to start the recursive matching algorithm
 * and create the result Array object
 *
 * See also:
 *          ECMA-262 v5, 15.10.6.2
 *          ECMA-262 v11, 21.2.5.2.2
 *
 * @return array object - if matched
 *         null         - otherwise
 *
 *         May raise error.
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_regexp_exec_helper (ecma_object_t *regexp_object_p, /**< RegExp object */
                         ecma_string_t *input_string_p) /**< input string */
{
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;
  lit_utf8_byte_t *matched_p;
  uint8_t *bc_start_p;

  /* 1. */
  JERRY_ASSERT (ecma_object_is_regexp_object (ecma_make_object_value (regexp_object_p)));

  /* 9. */
  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) regexp_object_p;
  re_compiled_code_t *bc_p = ECMA_GET_INTERNAL_VALUE_POINTER (re_compiled_code_t, ext_object_p->u.cls.u3.value);

  /* 3. */
  lit_utf8_size_t input_size;
  lit_utf8_size_t input_length;
  uint8_t input_flags = ECMA_STRING_FLAG_IS_ASCII;
  const lit_utf8_byte_t *input_buffer_p =
    ecma_string_get_chars (input_string_p, &input_size, &input_length, NULL, &input_flags);

  const lit_utf8_byte_t *input_curr_p = input_buffer_p;
  const lit_utf8_byte_t *input_end_p = input_buffer_p + input_size;

  ecma_regexp_ctx_t re_ctx;
  ecma_regexp_initialize_context (&re_ctx, bc_p, input_buffer_p, input_end_p);

  /* 4. */
  ecma_length_t index = 0;
  ecma_value_t lastindex_value = ecma_op_object_get_by_magic_id (regexp_object_p, LIT_MAGIC_STRING_LASTINDEX_UL);

  ret_value = ecma_op_to_length (lastindex_value, &index);
  ecma_free_value (lastindex_value);

  if (ECMA_IS_VALUE_ERROR (ret_value))
  {
    goto cleanup_context;
  }

  if (re_ctx.flags & (RE_FLAG_GLOBAL | RE_FLAG_STICKY))
  {
    /* 12.a */
    if (index > input_length)
    {
      goto fail_put_lastindex;
    }

    if (index > 0)
    {
      if (input_flags & ECMA_STRING_FLAG_IS_ASCII)
      {
        input_curr_p += index;
      }
      else
      {
        for (uint32_t i = 0; i < index; i++)
        {
          lit_utf8_incr (&input_curr_p);
        }
      }
    }
  }
  /* 8. */
  else
  {
    index = 0;
  }

  /* 9. */
  bc_start_p = (uint8_t *) (bc_p + 1);

  /* 11. */
  matched_p = NULL;

  /* 12. */
  JERRY_ASSERT (index <= input_length);
  while (true)
  {
    matched_p = (lit_utf8_byte_t *) ecma_regexp_match (&re_ctx, bc_start_p, input_curr_p);

    if (matched_p != NULL)
    {
      goto match_found;
    }

    /* 12.c.i */
    if (re_ctx.flags & RE_FLAG_STICKY)
    {
      goto fail_put_lastindex;
    }

    /* 12.a */
    if (input_curr_p >= input_end_p)
    {
      if (re_ctx.flags & RE_FLAG_GLOBAL)
      {
        goto fail_put_lastindex;
      }

      goto match_failed;
    }

    JERRY_ASSERT (input_curr_p < input_end_p);

    /* 12.c.ii */
    index++;

    if (re_ctx.flags & RE_FLAG_UNICODE)
    {
      const lit_code_point_t cp = ecma_regexp_unicode_advance (&input_curr_p, input_end_p);

      if (cp > LIT_UTF16_CODE_UNIT_MAX)
      {
        index++;
      }

      continue;
    }

    lit_utf8_incr (&input_curr_p);
  }

  JERRY_UNREACHABLE ();

fail_put_lastindex:
  /* We should only get here if the regexp is global or sticky */
  JERRY_ASSERT ((re_ctx.flags & (RE_FLAG_GLOBAL | RE_FLAG_STICKY)) != 0);

  ret_value = ecma_op_object_put (regexp_object_p,
                                  ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL),
                                  ecma_make_integer_value (0),
                                  true);

  if (ECMA_IS_VALUE_ERROR (ret_value))
  {
    goto cleanup_context;
  }

  JERRY_ASSERT (ecma_is_value_boolean (ret_value));

match_failed:
  /* 12.a.ii */
  ret_value = ECMA_VALUE_NULL;
  goto cleanup_context;

match_found:
  JERRY_ASSERT (matched_p != NULL);

  if (ECMA_RE_STACK_LIMIT_REACHED (matched_p))
  {
    ret_value = ecma_raise_range_error (ECMA_ERR_STACK_LIMIT_EXCEEDED);
    goto cleanup_context;
  }

  JERRY_ASSERT (index <= input_length);

  /* 15. */
  if (re_ctx.flags & (RE_FLAG_GLOBAL | RE_FLAG_STICKY))
  {
    /* 13-14. */
    lit_utf8_size_t match_length;
    const lit_utf8_byte_t *match_begin_p = re_ctx.captures_p[0].begin_p;
    const lit_utf8_byte_t *match_end_p = re_ctx.captures_p[0].end_p;

    if (input_flags & ECMA_STRING_FLAG_IS_ASCII)
    {
      match_length = (lit_utf8_size_t) (match_end_p - match_begin_p);
    }
    else
    {
      match_length = lit_utf8_string_length (match_begin_p, (lit_utf8_size_t) (match_end_p - match_begin_p));
    }

    ret_value = ecma_op_object_put (regexp_object_p,
                                    ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL),
                                    ecma_make_uint32_value ((uint32_t) index + match_length),
                                    true);

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      goto cleanup_context;
    }

    JERRY_ASSERT (ecma_is_value_boolean (ret_value));
  }

  /* 16-27. */
  ret_value = ecma_regexp_create_result_object (&re_ctx, input_string_p, (uint32_t) index);

cleanup_context:
  ecma_regexp_cleanup_context (&re_ctx);

  if (input_flags & ECMA_STRING_FLAG_MUST_BE_FREED)
  {
    jmem_heap_free_block ((void *) input_buffer_p, input_size);
  }

  return ret_value;
} /* ecma_regexp_exec_helper */

/**
 * Helper function for converting a RegExp pattern parameter to string.
 *
 * See also:
 *         RegExp.compile
 *         RegExp dispatch call
 *
 * @return empty value if success, error value otherwise
 *         Returned value must be freed with ecma_free_value.
 */
ecma_string_t *
ecma_regexp_read_pattern_str_helper (ecma_value_t pattern_arg) /**< the RegExp pattern */
{
  if (!ecma_is_value_undefined (pattern_arg))
  {
    ecma_string_t *pattern_string_p = ecma_op_to_string (pattern_arg);
    if (JERRY_UNLIKELY (pattern_string_p == NULL) || !ecma_string_is_empty (pattern_string_p))
    {
      return pattern_string_p;
    }
  }

  return ecma_get_magic_string (LIT_MAGIC_STRING_EMPTY_NON_CAPTURE_GROUP);
} /* ecma_regexp_read_pattern_str_helper */

/**
 * Helper function for RegExp based string searches
 *
 * See also:
 *          ECMA-262 v6, 21.2.5.9
 *
 * @return index of the match
 */
ecma_value_t
ecma_regexp_search_helper (ecma_value_t regexp_arg, /**< regexp argument */
                           ecma_value_t string_arg) /**< string argument */
{
  /* 2. */
  if (!ecma_is_value_object (regexp_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_value_t result = ECMA_VALUE_ERROR;
  ecma_value_t current_last_index;
  ecma_value_t match;
  bool same_value;

  /* 3. */
  ecma_string_t *const string_p = ecma_op_to_string (string_arg);
  if (string_p == NULL)
  {
    return result;
  }

  ecma_object_t *const regexp_object_p = ecma_get_object_from_value (regexp_arg);

  /* 4. */
  ecma_string_t *const last_index_str_p = ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL);
  const ecma_value_t prev_last_index = ecma_op_object_get (regexp_object_p, last_index_str_p);
  if (ECMA_IS_VALUE_ERROR (prev_last_index))
  {
    goto cleanup_string;
  }

  /* 5. */
  if (prev_last_index != ecma_make_uint32_value (0))
  {
    const ecma_value_t status =
      ecma_op_object_put (regexp_object_p, last_index_str_p, ecma_make_uint32_value (0), true);

    if (ECMA_IS_VALUE_ERROR (status))
    {
      goto cleanup_prev_last_index;
    }

    JERRY_ASSERT (ecma_is_value_boolean (status));
  }

  /* 6. */
  match = ecma_op_regexp_exec (regexp_arg, string_p);
  if (ECMA_IS_VALUE_ERROR (match))
  {
    goto cleanup_prev_last_index;
  }

  /* 7. */
  current_last_index = ecma_op_object_get (regexp_object_p, last_index_str_p);
  if (ECMA_IS_VALUE_ERROR (current_last_index))
  {
    ecma_free_value (match);
    goto cleanup_prev_last_index;
  }

  same_value = ecma_op_same_value (prev_last_index, current_last_index);

  ecma_free_value (current_last_index);

  /* 8. */
  if (!same_value)
  {
    result = ecma_op_object_put (regexp_object_p, last_index_str_p, prev_last_index, true);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      ecma_free_value (match);
      goto cleanup_prev_last_index;
    }

    JERRY_ASSERT (ecma_is_value_boolean (result));
  }

  /* 9-10. */
  if (ecma_is_value_null (match))
  {
    result = ecma_make_int32_value (-1);
  }
  else
  {
    ecma_object_t *const match_p = ecma_get_object_from_value (match);
    result = ecma_op_object_get_by_magic_id (match_p, LIT_MAGIC_STRING_INDEX);
    ecma_deref_object (match_p);
  }

cleanup_prev_last_index:
  ecma_free_value (prev_last_index);

cleanup_string:
  ecma_deref_ecma_string (string_p);
  return result;
} /* ecma_regexp_search_helper */

/**
 * Helper function for RegExp based string split operation
 *
 * See also:
 *          ECMA-262 v6, 21.2.5.11
 *
 * @return array of split and captured strings
 */
ecma_value_t
ecma_regexp_split_helper (ecma_value_t this_arg, /**< this value */
                          ecma_value_t string_arg, /**< string value */
                          ecma_value_t limit_arg) /**< limit value */
{
  /* 2. */
  if (!ecma_is_value_object (this_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_value_t result = ECMA_VALUE_ERROR;
  ecma_length_t current_index;
  ecma_length_t previous_index;
  ecma_object_t *constructor_obj_p;
  ecma_value_t flags;
  ecma_string_t *flags_str_p;
  lit_utf8_size_t flags_size;
  uint8_t flags_str_flags;
  lit_utf8_byte_t *flags_buffer_p;
  lit_utf8_byte_t *flags_end_p;
  ecma_value_t arguments[2];
  ecma_value_t splitter;
  ecma_object_t *splitter_obj_p;
  ecma_object_t *array_p;
  ecma_value_t array;
  ecma_string_t *end_str_p;
  ecma_string_t *lastindex_str_p;
  lit_utf8_size_t string_length;
  uint32_t array_length;
  uint32_t limit;
  bool unicode;
  bool sticky;

  /* 3-4. */
  ecma_string_t *const string_p = ecma_op_to_string (string_arg);
  if (string_p == NULL)
  {
    return result;
  }

  /* 5-6. */
  ecma_object_t *const regexp_obj_p = ecma_get_object_from_value (this_arg);
  ecma_value_t constructor = ecma_op_species_constructor (regexp_obj_p, ECMA_BUILTIN_ID_REGEXP);
  if (ECMA_IS_VALUE_ERROR (constructor))
  {
    goto cleanup_string;
  }

  constructor_obj_p = ecma_get_object_from_value (constructor);

  /* 7-8. */
  flags = ecma_op_object_get_by_magic_id (regexp_obj_p, LIT_MAGIC_STRING_FLAGS);
  if (ECMA_IS_VALUE_ERROR (flags))
  {
    ecma_deref_object (constructor_obj_p);
    goto cleanup_string;
  }

  flags_str_p = ecma_op_to_string (flags);
  ecma_free_value (flags);

  if (JERRY_UNLIKELY (flags_str_p == NULL))
  {
    ecma_deref_object (constructor_obj_p);
    goto cleanup_string;
  }

  flags_str_flags = ECMA_STRING_FLAG_IS_ASCII;
  flags_buffer_p = (lit_utf8_byte_t *) ecma_string_get_chars (flags_str_p, &flags_size, NULL, NULL, &flags_str_flags);

  unicode = false;
  sticky = false;

  /* 9-11. */
  flags_end_p = flags_buffer_p + flags_size;
  for (const lit_utf8_byte_t *current_p = flags_buffer_p; current_p < flags_end_p; ++current_p)
  {
    switch (*current_p)
    {
      case LIT_CHAR_LOWERCASE_U:
      {
        unicode = true;
        break;
      }
      case LIT_CHAR_LOWERCASE_Y:
      {
        sticky = true;
        break;
      }
    }
  }

  if (flags_str_flags & ECMA_STRING_FLAG_MUST_BE_FREED)
  {
    jmem_heap_free_block ((void *) flags_buffer_p, flags_size);
  }

  /* 12. */
  if (!sticky)
  {
    ecma_stringbuilder_t builder = ecma_stringbuilder_create_from (flags_str_p);
    ecma_stringbuilder_append_byte (&builder, LIT_CHAR_LOWERCASE_Y);

    ecma_deref_ecma_string (flags_str_p);
    flags_str_p = ecma_stringbuilder_finalize (&builder);
  }

  /* 13-14. */
  arguments[0] = this_arg;
  arguments[1] = ecma_make_string_value (flags_str_p);
  splitter = ecma_op_function_construct (constructor_obj_p, constructor_obj_p, arguments, 2);

  ecma_deref_ecma_string (flags_str_p);
  ecma_deref_object (constructor_obj_p);

  if (ECMA_IS_VALUE_ERROR (splitter))
  {
    goto cleanup_string;
  }

  splitter_obj_p = ecma_get_object_from_value (splitter);

  /* 17. */
  limit = UINT32_MAX - 1;
  if (!ecma_is_value_undefined (limit_arg))
  {
    /* ECMA-262 v11, 21.2.5.13 13 */
    ecma_number_t num;
    if (ECMA_IS_VALUE_ERROR (ecma_op_to_number (limit_arg, &num)))
    {
      goto cleanup_splitter;
    }
    limit = ecma_number_to_uint32 (num);
  }

  /* 15. */
  array_p = ecma_op_new_array_object (0);
  array = ecma_make_object_value (array_p);

  /* 21. */
  if (limit == 0)
  {
    result = array;
    goto cleanup_splitter;
  }

  string_length = ecma_string_get_length (string_p);
  array_length = 0;

  /* 22. */
  if (string_length == 0)
  {
    const ecma_value_t match = ecma_op_regexp_exec (splitter, string_p);

    if (ECMA_IS_VALUE_ERROR (match))
    {
      goto cleanup_array;
    }

    if (ecma_is_value_null (match))
    {
      result = ecma_builtin_helper_def_prop_by_index (array_p,
                                                      array_length,
                                                      ecma_make_string_value (string_p),
                                                      ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
      JERRY_ASSERT (ecma_is_value_true (result));
    }

    ecma_free_value (match);
    result = array;
    goto cleanup_splitter;
  }

  /* 23. */
  current_index = 0;
  previous_index = 0;

  lastindex_str_p = ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL);

  /* 24. */
  while (current_index < string_length)
  {
    /* 24.a-b. */
    ecma_value_t index_value = ecma_make_length_value (current_index);
    result = ecma_op_object_put (splitter_obj_p, lastindex_str_p, index_value, true);

    ecma_free_value (index_value);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_array;
    }

    JERRY_ASSERT (ecma_is_value_true (result));

    /* 24.c-d. */
    result = ecma_op_regexp_exec (splitter, string_p);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_array;
    }

    /* 24.e. */
    if (ecma_is_value_null (result))
    {
      current_index = ecma_op_advance_string_index (string_p, current_index, unicode);
      continue;
    }

    ecma_object_t *const match_array_p = ecma_get_object_from_value (result);

    /* 24.f.i. */
    result = ecma_op_object_get (splitter_obj_p, lastindex_str_p);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      ecma_deref_object (match_array_p);
      goto cleanup_array;
    }

    ecma_length_t end_index;
    const ecma_value_t length_value = ecma_op_to_length (result, &end_index);
    ecma_free_value (result);

    if (ECMA_IS_VALUE_ERROR (length_value))
    {
      result = ECMA_VALUE_ERROR;
      ecma_deref_object (match_array_p);
      goto cleanup_array;
    }

    /* ECMA-262 v11, 21.2.5.11 19.d.ii */
    if (end_index > string_length)
    {
      end_index = string_length;
    }

    /* 24.f.iii. */
    if (previous_index == end_index)
    {
      ecma_deref_object (match_array_p);
      current_index = ecma_op_advance_string_index (string_p, current_index, unicode);
      continue;
    }

    /* 24.f.iv.1-4. */
    JERRY_ASSERT (previous_index <= string_length && current_index <= string_length);
    ecma_string_t *const split_str_p =
      ecma_string_substr (string_p, (lit_utf8_size_t) previous_index, (lit_utf8_size_t) current_index);

    result = ecma_builtin_helper_def_prop_by_index (array_p,
                                                    array_length++,
                                                    ecma_make_string_value (split_str_p),
                                                    ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
    JERRY_ASSERT (ecma_is_value_true (result));
    ecma_deref_ecma_string (split_str_p);

    /* 24.f.iv.5. */
    if (array_length == limit)
    {
      ecma_deref_object (match_array_p);
      result = array;
      goto cleanup_splitter;
    }

    /* 24.f.iv.6. */
    previous_index = end_index;

    /* 24.f.iv.7-8. */
    ecma_length_t match_length;
    result = ecma_op_object_get_length (match_array_p, &match_length);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      ecma_deref_object (match_array_p);
      goto cleanup_array;
    }

    /* 24.f.iv.9. */
    match_length = (match_length > 0) ? match_length - 1 : match_length;

    ecma_length_t match_index = 1;
    while (match_index <= match_length)
    {
      /* 24.f.iv.11.a-b. */
      result = ecma_op_object_get_by_index (match_array_p, match_index++);
      if (ECMA_IS_VALUE_ERROR (result))
      {
        ecma_deref_object (match_array_p);
        goto cleanup_array;
      }

      const ecma_value_t capture = result;

      /* 24.f.iv.11.c. */
      result = ecma_builtin_helper_def_prop_by_index (array_p,
                                                      array_length++,
                                                      capture,
                                                      ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
      JERRY_ASSERT (ecma_is_value_true (result));

      ecma_free_value (capture);

      if (array_length == limit)
      {
        ecma_deref_object (match_array_p);
        result = array;
        goto cleanup_splitter;
      }
    }

    /* 24.f.iv.12. */
    JERRY_ASSERT (end_index <= UINT32_MAX);
    current_index = (uint32_t) end_index;

    ecma_deref_object (match_array_p);
  }

  JERRY_ASSERT (previous_index <= string_length);
  end_str_p = ecma_string_substr (string_p, (lit_utf8_size_t) previous_index, string_length);
  result = ecma_builtin_helper_def_prop_by_index (array_p,
                                                  array_length++,
                                                  ecma_make_string_value (end_str_p),
                                                  ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
  JERRY_ASSERT (ecma_is_value_true (result));
  ecma_deref_ecma_string (end_str_p);

  result = array;
  goto cleanup_splitter;

cleanup_array:
  ecma_deref_object (array_p);
cleanup_splitter:
  ecma_deref_object (splitter_obj_p);
cleanup_string:
  ecma_deref_ecma_string (string_p);

  return result;
} /* ecma_regexp_split_helper */

/**
 * Fast path for RegExp based replace operation
 *
 * This method assumes the following:
 *   - The RegExp object is a built-in RegExp
 *   - The 'exec' method of the RegExp object is the built-in 'exec' method
 *   - The 'lastIndex' property is writable
 *
 * The standard would normally require us to first execute the regexp and collect the results,
 * and after that iterate over the collected results and replace them.
 * The assumptions above guarantee that during the matching phase there will be no exceptions thrown,
 * which means we can do the match/replace in a single loop, without collecting the results.
 *
 * @return string value if successful
 *         thrown value otherwise
 */
static ecma_value_t
ecma_regexp_replace_helper_fast (ecma_replace_context_t *ctx_p, /**<replace context */
                                 ecma_extended_object_t *re_obj_p, /**< regexp object */
                                 ecma_string_t *string_p, /**< source string */
                                 ecma_value_t replace_arg) /**< replace argument */
{
  const re_compiled_code_t *bc_p = ECMA_GET_INTERNAL_VALUE_POINTER (re_compiled_code_t, re_obj_p->u.cls.u3.value);
  ecma_bytecode_ref ((ecma_compiled_code_t *) bc_p);

  JERRY_ASSERT (bc_p != NULL);

  uint8_t string_flags = ECMA_STRING_FLAG_IS_ASCII;
  lit_utf8_size_t string_length;
  ctx_p->string_p = ecma_string_get_chars (string_p, &(ctx_p->string_size), &string_length, NULL, &string_flags);

  const lit_utf8_byte_t *const string_end_p = ctx_p->string_p + ctx_p->string_size;
  const uint8_t *const bc_start_p = (const uint8_t *) (bc_p + 1);
  const lit_utf8_byte_t *matched_p = NULL;
  const lit_utf8_byte_t *current_p = ctx_p->string_p;
  const lit_utf8_byte_t *last_append_p = current_p;
  ecma_length_t index;
  lit_utf8_size_t trailing_size;

  ecma_regexp_ctx_t re_ctx;
  ecma_regexp_initialize_context (&re_ctx, bc_p, ctx_p->string_p, string_end_p);

  /* lastIndex must be accessed to remain consistent with the standard, even though we may not need the value. */
  ecma_value_t lastindex_value =
    ecma_op_object_get_by_magic_id ((ecma_object_t *) re_obj_p, LIT_MAGIC_STRING_LASTINDEX_UL);
  ecma_value_t result = ecma_op_to_length (lastindex_value, &index);
  ecma_free_value (lastindex_value);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    goto cleanup_context;
  }

  /* Only non-global sticky matches use the lastIndex value, otherwise the starting index is 0. */
  if (JERRY_UNLIKELY ((ctx_p->flags & RE_FLAG_GLOBAL) == 0 && (re_ctx.flags & RE_FLAG_STICKY) != 0))
  {
    if (index > string_length)
    {
      result = ecma_op_object_put ((ecma_object_t *) re_obj_p,
                                   ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL),
                                   ecma_make_uint32_value (0),
                                   true);

      if (!ECMA_IS_VALUE_ERROR (result))
      {
        JERRY_ASSERT (ecma_is_value_true (result));
        ecma_ref_ecma_string (string_p);
        result = ecma_make_string_value (string_p);
      }

      goto cleanup_context;
    }

    if (string_flags & ECMA_STRING_FLAG_IS_ASCII)
    {
      current_p += index;
    }
    else
    {
      ecma_length_t counter = index;
      while (counter--)
      {
        lit_utf8_incr (&current_p);
      }
    }
  }
  else
  {
    index = 0;
  }

  ctx_p->builder = ecma_stringbuilder_create ();
  ctx_p->capture_count = re_ctx.captures_count;
  ctx_p->u.captures_p = re_ctx.captures_p;

  while (true)
  {
    matched_p = ecma_regexp_match (&re_ctx, bc_start_p, current_p);

    if (matched_p != NULL)
    {
      if (ECMA_RE_STACK_LIMIT_REACHED (matched_p))
      {
        result = ecma_raise_range_error (ECMA_ERR_STACK_LIMIT_EXCEEDED);
        goto cleanup_builder;
      }

      const lit_utf8_size_t remaining_size = (lit_utf8_size_t) (current_p - last_append_p);
      ecma_stringbuilder_append_raw (&(ctx_p->builder), last_append_p, remaining_size);

      if (ctx_p->replace_str_p != NULL)
      {
        ctx_p->matched_p = current_p;
        const ecma_regexp_capture_t *const global_capture_p = re_ctx.captures_p;
        ctx_p->matched_size = (lit_utf8_size_t) (global_capture_p->end_p - global_capture_p->begin_p);
        ctx_p->match_byte_pos = (lit_utf8_size_t) (current_p - re_ctx.input_start_p);

        ecma_builtin_replace_substitute (ctx_p);
      }
      else
      {
        ecma_collection_t *arguments_p = ecma_new_collection ();

        for (uint32_t i = 0; i < re_ctx.captures_count; i++)
        {
          ecma_value_t capture = ecma_regexp_get_capture_value (re_ctx.captures_p + i);
          ecma_collection_push_back (arguments_p, capture);
        }

        ecma_collection_push_back (arguments_p, ecma_make_length_value (index));
        ecma_ref_ecma_string (string_p);
        ecma_collection_push_back (arguments_p, ecma_make_string_value (string_p));
        ecma_object_t *function_p = ecma_get_object_from_value (replace_arg);

        result =
          ecma_op_function_call (function_p, ECMA_VALUE_UNDEFINED, arguments_p->buffer_p, arguments_p->item_count);

        ecma_collection_free (arguments_p);

        if (ECMA_IS_VALUE_ERROR (result))
        {
          goto cleanup_builder;
        }

        /* 16.m.v */
        ecma_string_t *const replace_result_p = ecma_op_to_string (result);
        ecma_free_value (result);

        if (replace_result_p == NULL)
        {
          result = ECMA_VALUE_ERROR;
          goto cleanup_builder;
        }

        ecma_stringbuilder_append (&(ctx_p->builder), replace_result_p);
        ecma_deref_ecma_string (replace_result_p);
      }

      const ecma_regexp_capture_t *global_capture_p = re_ctx.captures_p;
      last_append_p = global_capture_p->end_p;

      const lit_utf8_size_t matched_size = (lit_utf8_size_t) (global_capture_p->end_p - global_capture_p->begin_p);

      const bool is_ascii = (string_flags & ECMA_STRING_FLAG_IS_ASCII) != 0;
      index += is_ascii ? matched_size : lit_utf8_string_length (current_p, matched_size);

      if (!(ctx_p->flags & RE_FLAG_GLOBAL))
      {
        if (JERRY_UNLIKELY ((re_ctx.flags & RE_FLAG_STICKY) != 0))
        {
          ecma_value_t index_value = ecma_make_length_value (index);
          result = ecma_op_object_put ((ecma_object_t *) re_obj_p,
                                       ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL),
                                       index_value,
                                       true);

          ecma_free_value (index_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto cleanup_builder;
          }
        }

        break;
      }

      if (matched_size > 0)
      {
        current_p = last_append_p;
        continue;
      }
    }
    else if (JERRY_UNLIKELY ((re_ctx.flags & RE_FLAG_STICKY) != 0))
    {
      result = ecma_op_object_put ((ecma_object_t *) re_obj_p,
                                   ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL),
                                   ecma_make_uint32_value (0),
                                   true);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        goto cleanup_builder;
      }

      break;
    }

    if (current_p >= string_end_p)
    {
      break;
    }

    if ((ctx_p->flags & RE_FLAG_UNICODE) != 0)
    {
      index++;
      const lit_code_point_t cp = ecma_regexp_unicode_advance (&current_p, string_end_p);

      if (cp > LIT_UTF16_CODE_UNIT_MAX)
      {
        index++;
      }

      continue;
    }

    index++;
    lit_utf8_incr (&current_p);
  }

  trailing_size = (lit_utf8_size_t) (string_end_p - last_append_p);
  ecma_stringbuilder_append_raw (&(ctx_p->builder), last_append_p, trailing_size);

  result = ecma_make_string_value (ecma_stringbuilder_finalize (&(ctx_p->builder)));
  goto cleanup_context;

cleanup_builder:
  ecma_stringbuilder_destroy (&(ctx_p->builder));

cleanup_context:
  ecma_regexp_cleanup_context (&re_ctx);
  ecma_bytecode_deref ((ecma_compiled_code_t *) bc_p);

  if (string_flags & ECMA_STRING_FLAG_MUST_BE_FREED)
  {
    jmem_heap_free_block ((void *) ctx_p->string_p, ctx_p->string_size);
  }

  return result;
} /* ecma_regexp_replace_helper_fast */

/**
 * Helper function for RegExp based replacing
 *
 * See also:
 *          String.prototype.replace
 *          RegExp.prototype[@@replace]
 *
 * @return result string of the replacement, if successful
 *         error value, otherwise
 */
ecma_value_t
ecma_regexp_replace_helper (ecma_value_t this_arg, /**< this argument */
                            ecma_value_t string_arg, /**< source string */
                            ecma_value_t replace_arg) /**< replace string */
{
  /* 2. */
  if (!ecma_is_value_object (this_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  lit_utf8_size_t index;
  uint8_t string_flags;
  ecma_object_t *this_obj_p = ecma_get_object_from_value (this_arg);
  ecma_replace_context_t replace_ctx;
  ecma_collection_t *results_p;
  lit_utf8_size_t string_length;
  lit_utf8_byte_t *source_position_p;
  lit_utf8_byte_t *string_end_p;

  replace_ctx.flags = RE_FLAG_EMPTY;

  /* 3. */
  ecma_string_t *string_p = ecma_op_to_string (string_arg);
  if (string_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t result = ECMA_VALUE_ERROR;

  /* 6. */
  replace_ctx.replace_str_p = NULL;
  if (!ecma_op_is_callable (replace_arg))
  {
    replace_ctx.replace_str_p = ecma_op_to_string (replace_arg);

    if (replace_ctx.replace_str_p == NULL)
    {
      goto cleanup_string;
    }
  }

  /* 8 */
  result = ecma_op_object_get_by_magic_id (this_obj_p, LIT_MAGIC_STRING_GLOBAL);
  if (ECMA_IS_VALUE_ERROR (result))
  {
    goto cleanup_replace;
  }

  if (ecma_op_to_boolean (result))
  {
    replace_ctx.flags |= RE_FLAG_GLOBAL;
  }

  ecma_free_value (result);

  string_length = ecma_string_get_length (string_p);

  /* 10. */
  if (replace_ctx.flags & RE_FLAG_GLOBAL)
  {
    result = ecma_op_object_get_by_magic_id (this_obj_p, LIT_MAGIC_STRING_UNICODE);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_replace;
    }

    if (ecma_op_to_boolean (result))
    {
      replace_ctx.flags |= RE_FLAG_UNICODE;
    }

    ecma_free_value (result);

    result = ecma_op_object_put (this_obj_p,
                                 ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL),
                                 ecma_make_uint32_value (0),
                                 true);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_replace;
    }

    JERRY_ASSERT (ecma_is_value_boolean (result));
  }

  result = ecma_op_object_get_by_magic_id (this_obj_p, LIT_MAGIC_STRING_EXEC);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    goto cleanup_replace;
  }

  /* Check for fast path. */
  if (ecma_op_is_callable (result))
  {
    ecma_extended_object_t *function_p = (ecma_extended_object_t *) ecma_get_object_from_value (result);
    if (ecma_object_class_is (this_obj_p, ECMA_OBJECT_CLASS_REGEXP) && ecma_builtin_is_regexp_exec (function_p))
    {
      ecma_deref_object ((ecma_object_t *) function_p);

      result =
        ecma_regexp_replace_helper_fast (&replace_ctx, (ecma_extended_object_t *) this_obj_p, string_p, replace_arg);

      goto cleanup_replace;
    }
  }

  results_p = ecma_new_collection ();

  while (true)
  {
    /* 13.a */
    if (ecma_op_is_callable (result))
    {
      ecma_object_t *const function_p = ecma_get_object_from_value (result);

      ecma_value_t arguments[] = { ecma_make_string_value (string_p) };
      result = ecma_op_function_call (function_p, this_arg, arguments, 1);

      ecma_deref_object (function_p);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        goto cleanup_results;
      }

      if (!ecma_is_value_object (result) && !ecma_is_value_null (result))
      {
        ecma_free_value (result);
        result = ecma_raise_type_error (ECMA_ERR_RETURN_VALUE_OF_EXEC_MUST_BE_AN_OBJECT_OR_NULL);
        goto cleanup_results;
      }
    }
    else
    {
      ecma_free_value (result);

      if (!ecma_object_class_is (this_obj_p, ECMA_OBJECT_CLASS_REGEXP))
      {
        result = ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_REG_EXP_OBJECT);
        goto cleanup_results;
      }

      result = ecma_regexp_exec_helper (this_obj_p, string_p);
    }

    /* 13.c */
    if (ecma_is_value_null (result))
    {
      break;
    }

    /* 13.d.i */
    ecma_collection_push_back (results_p, result);

    if ((replace_ctx.flags & RE_FLAG_GLOBAL) == 0)
    {
      break;
    }

    /* 13.d.iii.1 */
    result = ecma_op_object_get_by_index (ecma_get_object_from_value (result), 0);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_results;
    }

    ecma_string_t *match_str_p = ecma_op_to_string (result);
    ecma_free_value (result);

    if (match_str_p == NULL)
    {
      result = ECMA_VALUE_ERROR;
      goto cleanup_results;
    }

    const bool is_empty = ecma_string_is_empty (match_str_p);
    ecma_deref_ecma_string (match_str_p);

    /* 13.d.iii.3 */
    if (is_empty)
    {
      result = ecma_op_object_get_by_magic_id (this_obj_p, LIT_MAGIC_STRING_LASTINDEX_UL);
      if (ECMA_IS_VALUE_ERROR (result))
      {
        goto cleanup_results;
      }

      ecma_value_t last_index = result;

      ecma_length_t index;
      result = ecma_op_to_length (last_index, &index);
      ecma_free_value (last_index);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        goto cleanup_results;
      }

      index = ecma_op_advance_string_index (string_p, index, (replace_ctx.flags & RE_FLAG_UNICODE) != 0);
      last_index = ecma_make_length_value (index);

      /* 10.d.iii.3.c */
      result = ecma_op_object_put (this_obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL), last_index, true);

      ecma_free_value (last_index);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        goto cleanup_results;
      }

      JERRY_ASSERT (ecma_is_value_boolean (result));
    }

    result = ecma_op_object_get_by_magic_id (this_obj_p, LIT_MAGIC_STRING_EXEC);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_results;
    }
  }

  string_flags = ECMA_STRING_FLAG_IS_ASCII;
  replace_ctx.string_p = ecma_string_get_chars (string_p, &(replace_ctx.string_size), NULL, NULL, &string_flags);

  /* 14. */
  replace_ctx.builder = ecma_stringbuilder_create ();
  replace_ctx.matched_p = NULL;
  replace_ctx.capture_count = 0;
  index = 0;

  /* 15. */
  source_position_p = (lit_utf8_byte_t*) replace_ctx.string_p;
  string_end_p = (lit_utf8_byte_t*) replace_ctx.string_p + replace_ctx.string_size;

  /* 16. */
  for (ecma_value_t *current_p = results_p->buffer_p; current_p < results_p->buffer_p + results_p->item_count;
       current_p++)
  {
    /* 16.a */
    ecma_object_t *current_object_p = ecma_get_object_from_value (*current_p);

    ecma_length_t capture_count;
    result = ecma_op_object_get_length (current_object_p, &capture_count);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_builder;
    }

    /* 16.c */
    capture_count = (capture_count > 0) ? capture_count - 1 : capture_count;

    /* 16.d */
    result = ecma_op_object_get_by_index (current_object_p, 0);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_builder;
    }

    ecma_string_t *matched_str_p = ecma_op_to_string (result);
    ecma_free_value (result);

    /* 16.e */
    if (matched_str_p == NULL)
    {
      result = ECMA_VALUE_ERROR;
      goto cleanup_builder;
    }

    /* 16.g */
    result = ecma_op_object_get_by_magic_id (current_object_p, LIT_MAGIC_STRING_INDEX);
    if (ECMA_IS_VALUE_ERROR (result))
    {
      ecma_deref_ecma_string (matched_str_p);
      goto cleanup_builder;
    }

    const ecma_value_t index_value = result;

    ecma_number_t position_num;
    result = ecma_op_to_integer (index_value, &position_num);
    ecma_free_value (index_value);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      ecma_deref_ecma_string (matched_str_p);
      goto cleanup_builder;
    }

    /* 16.i */
    lit_utf8_size_t position = JERRY_MIN ((lit_utf8_size_t) JERRY_MAX (position_num, 0.0f), string_length);

    /* 16.k */
    ecma_collection_t *arguments_p = ecma_new_collection ();
    ecma_collection_push_back (arguments_p, ecma_make_string_value (matched_str_p));

    /* 16.j, l */
    ecma_length_t n = 1;
    while (n <= capture_count)
    {
      result = ecma_op_object_get_by_index (current_object_p, n);
      if (ECMA_IS_VALUE_ERROR (result))
      {
        ecma_collection_free (arguments_p);
        goto cleanup_builder;
      }

      /* 16.l.iii */
      if (!ecma_is_value_undefined (result))
      {
        ecma_string_t *capture_str_p = ecma_op_to_string (result);
        ecma_free_value (result);

        if (capture_str_p == NULL)
        {
          ecma_collection_free (arguments_p);
          result = ECMA_VALUE_ERROR;
          goto cleanup_builder;
        }

        result = ecma_make_string_value (capture_str_p);
      }

      /* 16.l.iv */
      ecma_collection_push_back (arguments_p, result);
      n++;
    }

    const bool should_replace = (position >= index);
    /* 16.p */
    if (should_replace)
    {
      const lit_utf8_byte_t *match_position_p;
      const lit_utf8_size_t matched_str_size = ecma_string_get_size (matched_str_p);
      const lit_utf8_size_t matched_str_length = ecma_string_get_length (matched_str_p);

      if (string_flags & ECMA_STRING_FLAG_IS_ASCII)
      {
        match_position_p = replace_ctx.string_p + position;
      }
      else
      {
        match_position_p = source_position_p;
        lit_utf8_size_t distance = position - index;
        while (distance--)
        {
          lit_utf8_incr (&match_position_p);
        }
      }

      ecma_stringbuilder_append_raw (&replace_ctx.builder,
                                     source_position_p,
                                     (lit_utf8_size_t) (match_position_p - source_position_p));
      replace_ctx.match_byte_pos = (lit_utf8_size_t) (match_position_p - replace_ctx.string_p);

      if ((string_flags & ECMA_STRING_FLAG_IS_ASCII) && matched_str_size == matched_str_length)
      {
        source_position_p = (lit_utf8_byte_t*) JERRY_MIN (match_position_p + matched_str_size, string_end_p);
      }
      else
      {
        lit_utf8_size_t code_unit_count = matched_str_length;

        while (code_unit_count-- > 0 && JERRY_LIKELY (match_position_p < string_end_p))
        {
          lit_utf8_incr (&match_position_p);
        }

        source_position_p = (lit_utf8_byte_t*) match_position_p;
      }

      index = JERRY_MIN (position + matched_str_length, string_length);
    }

    /* 16.m */
    if (replace_ctx.replace_str_p == NULL)
    {
      /* 16.m.i-ii.
       * arguments_p already contains <<Matched, cap1, cap2, ..., capN>> */

      /* 16.m.iii */
      ecma_collection_push_back (arguments_p, ecma_make_uint32_value (position));
      ecma_ref_ecma_string (string_p);
      ecma_collection_push_back (arguments_p, ecma_make_string_value (string_p));

      result = ecma_op_function_call (ecma_get_object_from_value (replace_arg),
                                      ECMA_VALUE_UNDEFINED,
                                      arguments_p->buffer_p,
                                      arguments_p->item_count);

      ecma_collection_free (arguments_p);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        goto cleanup_builder;
      }

      /* 16.m.v */
      ecma_string_t *const replace_result_p = ecma_op_to_string (result);
      ecma_free_value (result);

      if (replace_result_p == NULL)
      {
        result = ECMA_VALUE_ERROR;
        goto cleanup_builder;
      }

      /* 16.m/p */
      if (should_replace)
      {
        ecma_stringbuilder_append (&replace_ctx.builder, replace_result_p);
      }

      ecma_deref_ecma_string (replace_result_p);
    }
    else
    {
      /* 16.n/p */
      if (should_replace)
      {
        replace_ctx.u.collection_p = arguments_p;
        ecma_builtin_replace_substitute (&replace_ctx);
      }

      ecma_collection_free (arguments_p);
    }
  }

  /* 18. */
  JERRY_ASSERT (index <= string_length);
  ecma_stringbuilder_append_raw (&(replace_ctx.builder),
                                 source_position_p,
                                 (lit_utf8_size_t) (string_end_p - source_position_p));

  result = ecma_make_string_value (ecma_stringbuilder_finalize (&replace_ctx.builder));
  goto cleanup_chars;

cleanup_builder:
  ecma_stringbuilder_destroy (&replace_ctx.builder);

cleanup_chars:
  if (string_flags & ECMA_STRING_FLAG_MUST_BE_FREED)
  {
    jmem_heap_free_block ((void *) replace_ctx.string_p, replace_ctx.string_size);
  }

cleanup_results:
  ecma_collection_free (results_p);

cleanup_replace:
  if (replace_ctx.replace_str_p != NULL)
  {
    ecma_deref_ecma_string (replace_ctx.replace_str_p);
  }

cleanup_string:
  ecma_deref_ecma_string (string_p);

  return result;
} /* ecma_regexp_replace_helper */

/**
 * Helper function for RegExp based matching
 *
 * See also:
 *          String.prototype.match
 *          RegExp.prototype[@@match]
 *
 * @return ecma_value_t
 */
ecma_value_t
ecma_regexp_match_helper (ecma_value_t this_arg, /**< this argument */
                          ecma_value_t string_arg) /**< source string */
{
  if (!ecma_is_value_object (this_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_string_t *str_p = ecma_op_to_string (string_arg);

  if (JERRY_UNLIKELY (str_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (this_arg);

  ecma_value_t global_value = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_GLOBAL);

  if (ECMA_IS_VALUE_ERROR (global_value))
  {
    ecma_deref_ecma_string (str_p);
    return global_value;
  }

  bool global = ecma_op_to_boolean (global_value);

  ecma_free_value (global_value);

  if (!global)
  {
    ecma_value_t result = ecma_op_regexp_exec (this_arg, str_p);
    ecma_deref_ecma_string (str_p);
    return result;
  }

  ecma_value_t full_unicode_value = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_UNICODE);

  if (ECMA_IS_VALUE_ERROR (full_unicode_value))
  {
    ecma_deref_ecma_string (str_p);
    return full_unicode_value;
  }

  bool full_unicode = ecma_op_to_boolean (full_unicode_value);

  ecma_free_value (full_unicode_value);

  ecma_value_t set_status =
    ecma_op_object_put (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL), ecma_make_uint32_value (0), true);

  if (ECMA_IS_VALUE_ERROR (set_status))
  {
    ecma_deref_ecma_string (str_p);
    return set_status;
  }

  ecma_value_t ret_value = ECMA_VALUE_ERROR;
  ecma_object_t *result_array_p = ecma_op_new_array_object (0);
  uint32_t n = 0;

  while (true)
  {
    ecma_value_t result_value = ecma_op_regexp_exec (this_arg, str_p);

    if (ECMA_IS_VALUE_ERROR (result_value))
    {
      goto result_cleanup;
    }

    if (ecma_is_value_null (result_value))
    {
      if (n == 0)
      {
        ret_value = ECMA_VALUE_NULL;
        goto result_cleanup;
      }

      ecma_deref_ecma_string (str_p);
      return ecma_make_object_value (result_array_p);
    }

    ecma_object_t *result_value_p = ecma_get_object_from_value (result_value);
    ecma_value_t match_value = ecma_op_object_get_by_index (result_value_p, 0);

    ecma_deref_object (result_value_p);

    if (ECMA_IS_VALUE_ERROR (match_value))
    {
      goto result_cleanup;
    }

    ecma_string_t *match_str_p = ecma_op_to_string (match_value);
    ecma_free_value (match_value);

    if (JERRY_UNLIKELY (match_str_p == NULL))
    {
      goto result_cleanup;
    }

    ecma_value_t new_prop = ecma_builtin_helper_def_prop_by_index (result_array_p,
                                                                   n,
                                                                   ecma_make_string_value (match_str_p),
                                                                   ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);

    JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (new_prop));

    const bool is_match_empty = ecma_string_is_empty (match_str_p);
    ecma_deref_ecma_string (match_str_p);

    if (is_match_empty)
    {
      ecma_value_t last_index = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_LASTINDEX_UL);

      if (ECMA_IS_VALUE_ERROR (last_index))
      {
        goto result_cleanup;
      }

      ecma_length_t index;
      ecma_value_t length_value = ecma_op_to_length (last_index, &index);

      ecma_free_value (last_index);

      if (ECMA_IS_VALUE_ERROR (length_value))
      {
        goto result_cleanup;
      }

      index = ecma_op_advance_string_index (str_p, index, full_unicode);

      last_index = ecma_make_length_value (index);
      ecma_value_t next_set_status =
        ecma_op_object_put (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL), last_index, true);

      ecma_free_value (last_index);

      if (ECMA_IS_VALUE_ERROR (next_set_status))
      {
        goto result_cleanup;
      }
    }

    n++;
  }

result_cleanup:
  ecma_deref_ecma_string (str_p);
  ecma_deref_object (result_array_p);
  return ret_value;
} /* ecma_regexp_match_helper */

/**
 * RegExpExec operation
 *
 * See also:
 *          ECMA-262 v6.0, 21.2.5.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_op_regexp_exec (ecma_value_t this_arg, /**< this argument */
                     ecma_string_t *str_p) /**< input string */
{
  ecma_object_t *arg_obj_p = ecma_get_object_from_value (this_arg);

  ecma_value_t exec = ecma_op_object_get_by_magic_id (arg_obj_p, LIT_MAGIC_STRING_EXEC);

  if (ECMA_IS_VALUE_ERROR (exec))
  {
    return exec;
  }

  if (ecma_op_is_callable (exec))
  {
    ecma_object_t *function_p = ecma_get_object_from_value (exec);
    ecma_value_t arguments[] = { ecma_make_string_value (str_p) };

    ecma_value_t result = ecma_op_function_call (function_p, this_arg, arguments, 1);

    ecma_deref_object (function_p);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      return result;
    }

    if (!ecma_is_value_object (result) && !ecma_is_value_null (result))
    {
      ecma_free_value (result);
      return ecma_raise_type_error (ECMA_ERR_RETURN_VALUE_OF_EXEC_MUST_BE_AN_OBJECT_OR_NULL);
    }

    return result;
  }
  else
  {
    ecma_free_value (exec);
  }

  if (!ecma_object_is_regexp_object (this_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_REG_EXP);
  }

  return ecma_regexp_exec_helper (arg_obj_p, str_p);
} /* ecma_op_regexp_exec */

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
