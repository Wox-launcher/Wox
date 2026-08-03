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

#include "js-parser-internal.h"

#if JERRY_PARSER
#include "ecma-helpers.h"

#include "jcontext.h"
#include "js-parser-tagged-template-literal.h"
#include "lit-char-helpers.h"

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_expr Expression parser
 * @{
 */

/**
 * Maximum precedence for right-to-left binary operation evaluation.
 */
#define PARSER_RIGHT_TO_LEFT_ORDER_MAX_PRECEDENCE 7

/**
 * Precedence for ternary operation.
 */
#define PARSER_RIGHT_TO_LEFT_ORDER_TERNARY_PRECEDENCE 4

/**
 * Precedence for exponentiation operation.
 */
#define PARSER_RIGHT_TO_LEFT_ORDER_EXPONENTIATION 16

/**
 * Value of grouping level increase and decrease.
 */
#define PARSER_GROUPING_LEVEL_INCREASE 2

/**
 * Precedence of the binary tokens.
 *
 * See also:
 *    lexer_token_type_t
 */
static const uint8_t parser_binary_precedence_table[] = {
  3, /**< "=" */
  3, /**< "+=" */
  3, /**< "-=" */
  3, /**< "*=" */
  3, /**< "/=" */
  3, /**< "=" */
  3, /**< "<<=" */
  3, /**< ">>=" */
  3, /**< ">>>=" */
  3, /**< "&=" */
  3, /**< "|=" */
  3, /**< "^=" */
  3, /**< "**=" */
  3, /**< "??=" */
  3, /**< "||=" */
  3, /**< "&&=" */
  4, /**< "?"*/
  5, /**< "??" */
  6, /**< "||" */
  7, /**< "&&" */
  8, /**< "|" */
  9, /**< "^" */
  10, /**< "&" */
  11, /**< "==" */
  11, /**< "!=" */
  11, /**< "===" */
  11, /**< "!==" */
  12, /**< "<" */
  12, /**< ">" */
  12, /**< "<=" */
  12, /**< ">=" */
  12, /**< in */
  12, /**< instanceof */
  13, /**< "<<" */
  13, /**< ">>" */
  13, /**< ">>>" */
  14, /**< "+" */
  14, /**< "-" */
  15, /**< "*" */
  15, /**< "/" */
  15, /**< "%" */
  16, /**< "**" */
};

JERRY_STATIC_ASSERT ((sizeof (parser_binary_precedence_table) == 42),
                     parser_binary_precedence_table_should_have_39_values_in_es2015);

/**
 * Generate byte code for operators with lvalue.
 */
static inline void
parser_push_result (parser_context_t *context_p) /**< context */
{
  if (CBC_NO_RESULT_OPERATION (context_p->last_cbc_opcode))
  {
    JERRY_ASSERT (CBC_SAME_ARGS (context_p->last_cbc_opcode, context_p->last_cbc_opcode + 1));

    if ((context_p->last_cbc_opcode == CBC_POST_INCR || context_p->last_cbc_opcode == CBC_POST_DECR)
        && context_p->stack_depth >= context_p->stack_limit)
    {
      /* Stack limit is increased for CBC_POST_INCR_PUSH_RESULT
       * and CBC_POST_DECR_PUSH_RESULT opcodes. Needed by vm.c. */
      JERRY_ASSERT (context_p->stack_depth == context_p->stack_limit);

      context_p->stack_limit++;

      if (context_p->stack_limit > PARSER_MAXIMUM_STACK_LIMIT)
      {
        parser_raise_error (context_p, PARSER_ERR_STACK_LIMIT_REACHED);
      }
    }

    context_p->last_cbc_opcode++;
    parser_flush_cbc (context_p);
  }
} /* parser_push_result */

/**
 * Check for invalid assignment for "eval" and "arguments"
 */
static void
parser_check_invalid_assign (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL);

  if (JERRY_UNLIKELY (context_p->status_flags & PARSER_IS_STRICT))
  {
    if (context_p->last_cbc.literal_keyword_type == LEXER_KEYW_EVAL)
    {
      parser_raise_error (context_p, PARSER_ERR_EVAL_CANNOT_ASSIGNED);
    }
    else if (context_p->last_cbc.literal_keyword_type == LEXER_KEYW_ARGUMENTS)
    {
      parser_raise_error (context_p, PARSER_ERR_ARGUMENTS_CANNOT_ASSIGNED);
    }
  }
} /* parser_check_invalid_assign */

/**
 * Check and throw an error if the "new.target" is invalid as a left-hand side expression.
 */
static void
parser_check_invalid_new_target (parser_context_t *context_p, /**< parser context */
                                 cbc_opcode_t opcode) /**< current opcode under parsing */
{
  /* new.target is an invalid left-hand side target */
  if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_NEW_TARGET))
  {
    /* Make sure that the call side is a post/pre increment or an assignment expression.
     * There should be no other ways the "new.target" expression should be here. */
    JERRY_ASSERT (
      (opcode >= CBC_PRE_INCR && opcode <= CBC_POST_DECR)
      || (opcode == CBC_ASSIGN
          && (context_p->token.type == LEXER_ASSIGN || LEXER_IS_BINARY_LVALUE_OP_TOKEN (context_p->token.type))));

    parser_raise_error (context_p, PARSER_ERR_NEW_TARGET_NOT_ALLOWED);
  }
} /* parser_check_invalid_new_target */

/**
 * Emit identifier reference
 */
static void
parser_emit_ident_reference (parser_context_t *context_p, /**< context */
                             uint16_t opcode) /* opcode */
{
  if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
  {
    context_p->last_cbc_opcode = opcode;
    return;
  }

  uint16_t literal_index;

  if (context_p->last_cbc_opcode == CBC_PUSH_TWO_LITERALS)
  {
    context_p->last_cbc_opcode = CBC_PUSH_LITERAL;
    literal_index = context_p->last_cbc.value;
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_THIS_LITERAL)
  {
    context_p->last_cbc_opcode = CBC_PUSH_THIS;
    literal_index = context_p->last_cbc.literal_index;
  }
  else
  {
    JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_THREE_LITERALS);
    context_p->last_cbc_opcode = CBC_PUSH_TWO_LITERALS;
    literal_index = context_p->last_cbc.third_literal_index;
  }

  parser_emit_cbc_literal (context_p, opcode, literal_index);
} /* parser_emit_ident_reference */

/**
 * Generate byte code for operators with lvalue.
 */
static void
parser_emit_unary_lvalue_opcode (parser_context_t *context_p, /**< context */
                                 cbc_opcode_t opcode) /**< opcode */
{
  if (PARSER_IS_PUSH_LITERALS_WITH_THIS (context_p->last_cbc_opcode)
      && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL)
  {
    parser_check_invalid_assign (context_p);

    uint16_t unary_opcode;

    if (opcode == CBC_DELETE_PUSH_RESULT)
    {
      if (JERRY_UNLIKELY (context_p->status_flags & PARSER_IS_STRICT))
      {
        parser_raise_error (context_p, PARSER_ERR_DELETE_IDENT_NOT_ALLOWED);
      }

      unary_opcode = CBC_DELETE_IDENT_PUSH_RESULT;
    }
    else
    {
      JERRY_ASSERT (CBC_SAME_ARGS (CBC_PUSH_LITERAL, opcode + CBC_UNARY_LVALUE_WITH_IDENT));
      unary_opcode = (uint16_t) (opcode + CBC_UNARY_LVALUE_WITH_IDENT);
    }

    parser_emit_ident_reference (context_p, unary_opcode);

    if (unary_opcode != CBC_DELETE_IDENT_PUSH_RESULT
        && scanner_literal_is_const_reg (context_p, context_p->last_cbc.literal_index))
    {
      /* The current value must be read, but it cannot be changed. */
      context_p->last_cbc_opcode = CBC_PUSH_LITERAL;
      parser_emit_cbc_ext (context_p, CBC_EXT_THROW_ASSIGN_CONST_ERROR);
    }
    return;
  }

  if (context_p->last_cbc_opcode == CBC_PUSH_PROP)
  {
    JERRY_ASSERT (CBC_SAME_ARGS (CBC_PUSH_PROP, opcode));
    context_p->last_cbc_opcode = (uint16_t) opcode;
    return;
  }

  if (PARSER_IS_PUSH_PROP_LITERAL (context_p->last_cbc_opcode))
  {
    context_p->last_cbc_opcode = PARSER_PUSH_PROP_LITERAL_TO_PUSH_LITERAL (context_p->last_cbc_opcode);
  }
  else
  {
    /* Invalid LeftHandSide expression. */
    if (opcode == CBC_DELETE_PUSH_RESULT)
    {
      if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_PRIVATE_PROP_LITERAL))
      {
        parser_raise_error (context_p, PARSER_ERR_DELETE_PRIVATE_FIELD);
      }

      if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER_PROP_LITERAL)
          || context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER_PROP))
      {
        parser_emit_cbc_ext (context_p, CBC_EXT_THROW_REFERENCE_ERROR);
        parser_emit_cbc (context_p, CBC_POP);
        return;
      }

      parser_emit_cbc (context_p, CBC_POP);
      parser_emit_cbc (context_p, CBC_PUSH_TRUE);
      return;
    }

    parser_check_invalid_new_target (context_p, opcode);
    if (opcode == CBC_PRE_INCR || opcode == CBC_PRE_DECR)
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_LHS_PREFIX_OP);
    }
    else
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_LHS_POSTFIX_OP);
    }
  }

  parser_emit_cbc (context_p, (uint16_t) opcode);
} /* parser_emit_unary_lvalue_opcode */

/**
 * Parse array literal.
 */
static void
parser_parse_array_literal (parser_context_t *context_p) /**< context */
{
  uint32_t pushed_items = 0;
  uint16_t opcode = (uint16_t) CBC_ARRAY_APPEND;

  JERRY_ASSERT (context_p->token.type == LEXER_LEFT_SQUARE);

  parser_emit_cbc (context_p, CBC_CREATE_ARRAY);
  lexer_next_token (context_p);

  while (true)
  {
    if (context_p->token.type == LEXER_RIGHT_SQUARE)
    {
      if (pushed_items > 0)
      {
        parser_emit_cbc_call (context_p, opcode, pushed_items);
      }
      return;
    }

    pushed_items++;

    if (context_p->token.type == LEXER_COMMA)
    {
      parser_emit_cbc (context_p, CBC_PUSH_ELISION);
      lexer_next_token (context_p);
    }
    else
    {
      if (context_p->token.type == LEXER_THREE_DOTS)
      {
        opcode = (uint16_t) (PARSER_TO_EXT_OPCODE (CBC_EXT_SPREAD_ARRAY_APPEND));
        pushed_items++;
        lexer_next_token (context_p);
        parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_SPREAD_ELEMENT);
      }

      parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

      if (context_p->last_cbc_opcode == CBC_PUSH_THIS)
      {
        parser_flush_cbc (context_p);
      }

      if (context_p->token.type == LEXER_COMMA)
      {
        lexer_next_token (context_p);
      }
      else if (context_p->token.type != LEXER_RIGHT_SQUARE)
      {
        parser_raise_error (context_p, PARSER_ERR_ARRAY_ITEM_SEPARATOR_EXPECTED);
      }
    }

    if (pushed_items >= 64)
    {
      parser_emit_cbc_call (context_p, opcode, pushed_items);
      opcode = (uint16_t) CBC_ARRAY_APPEND;
      pushed_items = 0;
    }
  }
} /* parser_parse_array_literal */

/** Forward definition of parse array initializer. */
static void parser_parse_array_initializer (parser_context_t *context_p, parser_pattern_flags_t flags);

/** Forward definition of parse object initializer. */
static void parser_parse_object_initializer (parser_context_t *context_p, parser_pattern_flags_t flags);

/**
 * Class literal parsing options.
 */
typedef enum
{
  PARSER_CLASS_LITERAL_NO_OPTS = 0, /**< no options are provided */
  PARSER_CLASS_LITERAL_CTOR_PRESENT = (1 << 0), /**< class constructor is present */
  PARSER_CLASS_LITERAL_HERITAGE_PRESENT = (1 << 1), /**< class heritage is present */
} parser_class_literal_opts_t;

/**
 * Checks whether the current string or identifier literal is constructor
 *
 * @return true, if constructor and false otherwise
 */
static inline bool
parser_is_constructor_literal (parser_context_t *context_p) /**< context */
{
  return (LEXER_IS_IDENT_OR_STRING (context_p->token.lit_location.type)
          && lexer_compare_literal_to_string (context_p, "constructor", 11));
} /* parser_is_constructor_literal */

/**
 * Checks if current private field is already declared
 */
static void
parser_check_duplicated_private_field (parser_context_t *context_p, /**< context */
                                       uint8_t opts) /**< options */
{
  JERRY_ASSERT (context_p->token.type == LEXER_LITERAL);
  JERRY_ASSERT (context_p->private_context_p);
  scanner_class_private_member_t *iter = context_p->private_context_p->members_p;

  bool search_for_property = (opts & SCANNER_PRIVATE_FIELD_PROPERTY);

  while (iter != NULL)
  {
    if (lexer_compare_identifiers (context_p, &context_p->token.lit_location, &iter->loc) && (iter->u8_arg & opts))
    {
      if (iter->u8_arg & SCANNER_PRIVATE_FIELD_SEEN)
      {
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_PRIVATE_FIELD);
      }

      iter->u8_arg |= SCANNER_PRIVATE_FIELD_SEEN;

      if (!search_for_property)
      {
        break;
      }
    }

    iter = iter->prev_p;
  }
} /* parser_check_duplicated_private_field */

/**
 * Parse class literal.
 *
 * @return true - if the class has static fields, false - otherwise
 */
static bool
parser_parse_class_body (parser_context_t *context_p, /**< context */
                         parser_class_literal_opts_t opts, /**< class literal parsing options */
                         uint16_t class_name_index) /**< class literal index */
{
  JERRY_ASSERT (context_p->token.type == LEXER_LEFT_BRACE);

  lexer_literal_t *ctor_literal_p = NULL;
  lexer_literal_t *static_fields_literal_p = NULL;

  if (opts & PARSER_CLASS_LITERAL_CTOR_PRESENT)
  {
    ctor_literal_p = lexer_construct_unused_literal (context_p);
    parser_emit_cbc_literal (context_p, CBC_PUSH_LITERAL, (uint16_t) (context_p->literal_count++));
  }
  else if (opts & PARSER_CLASS_LITERAL_HERITAGE_PRESENT)
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_IMPLICIT_CONSTRUCTOR_HERITAGE);
  }
  else
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_IMPLICIT_CONSTRUCTOR);
  }

  if (class_name_index != PARSER_INVALID_LITERAL_INDEX)
  {
    parser_emit_cbc_ext_literal (context_p, CBC_EXT_SET_CLASS_NAME, class_name_index);
  }

  parser_emit_cbc_ext (context_p, CBC_EXT_INIT_CLASS);

  bool is_static = false;
  bool is_private = false;
  size_t fields_size = 0;
  uint32_t computed_field_count = 0;

  while (true)
  {
    if (!is_static)
    {
      lexer_skip_empty_statements (context_p);
    }

    uint32_t flags = (LEXER_OBJ_IDENT_CLASS_IDENTIFIER | LEXER_OBJ_IDENT_SET_FUNCTION_START);

    if (!is_static)
    {
      flags |= LEXER_OBJ_IDENT_CLASS_NO_STATIC;
    }

    if (is_private)
    {
      flags |= LEXER_OBJ_IDENT_CLASS_PRIVATE;
    }

    lexer_expect_object_literal_id (context_p, flags);

    if (context_p->token.type == LEXER_RIGHT_BRACE)
    {
      JERRY_ASSERT (!is_static);
      break;
    }

    if (context_p->token.type == LEXER_HASHMARK)
    {
      is_private = true;
      lexer_next_token (context_p);
      context_p->token.flags |= LEXER_NO_SKIP_SPACES;
      continue;
    }

    if (context_p->token.type == LEXER_KEYW_STATIC)
    {
      JERRY_ASSERT (!is_static);
      is_static = true;
      continue;
    }

    bool is_constructor_literal = false;

    if (context_p->token.type == LEXER_LITERAL)
    {
      is_constructor_literal = parser_is_constructor_literal (context_p);

      if (is_private)
      {
        if (is_constructor_literal && lexer_check_next_character (context_p, LIT_CHAR_LEFT_PAREN))
        {
          parser_raise_error (context_p, PARSER_ERR_CLASS_PRIVATE_CONSTRUCTOR);
        }

        parser_check_duplicated_private_field (context_p, SCANNER_PRIVATE_FIELD_PROPERTY_GETTER_SETTER);
      }
    }

    if (!is_static && is_constructor_literal)
    {
      JERRY_ASSERT (!is_static);
      JERRY_ASSERT (opts & PARSER_CLASS_LITERAL_CTOR_PRESENT);
      JERRY_ASSERT (ctor_literal_p != NULL);

      if (ctor_literal_p->type == LEXER_FUNCTION_LITERAL)
      {
        /* 14.5.1 */
        parser_raise_error (context_p, PARSER_ERR_MULTIPLE_CLASS_CONSTRUCTORS);
      }

      uint32_t constructor_status_flags =
        (PARSER_FUNCTION_CLOSURE | PARSER_ALLOW_SUPER | PARSER_CLASS_CONSTRUCTOR | PARSER_LEXICAL_ENV_NEEDED);

      if (opts & PARSER_CLASS_LITERAL_HERITAGE_PRESENT)
      {
        constructor_status_flags |= PARSER_ALLOW_SUPER_CALL;
      }

      if (context_p->status_flags & PARSER_INSIDE_WITH)
      {
        constructor_status_flags |= PARSER_INSIDE_WITH;
      }

      parser_flush_cbc (context_p);
      ecma_compiled_code_t *compiled_code_p = parser_parse_function (context_p, constructor_status_flags);
      ctor_literal_p->u.bytecode_p = compiled_code_p;
      ctor_literal_p->type = LEXER_FUNCTION_LITERAL;
      continue;
    }

    bool is_computed = false;

    if (context_p->token.type == LEXER_PROPERTY_GETTER || context_p->token.type == LEXER_PROPERTY_SETTER)
    {
      uint16_t literal_index, function_literal_index;
      bool is_getter = (context_p->token.type == LEXER_PROPERTY_GETTER);

      uint32_t accessor_status_flags = PARSER_FUNCTION_CLOSURE | PARSER_ALLOW_SUPER;
      accessor_status_flags |= (is_getter ? PARSER_IS_PROPERTY_GETTER : PARSER_IS_PROPERTY_SETTER);

      uint8_t ident_opts = LEXER_OBJ_IDENT_ONLY_IDENTIFIERS;

      if (lexer_check_next_character (context_p, LIT_CHAR_HASHMARK))
      {
        lexer_next_token (context_p);
        context_p->token.flags |= LEXER_NO_SKIP_SPACES;
        ident_opts |= LEXER_OBJ_IDENT_CLASS_PRIVATE;
        is_private = true;
      }

      lexer_expect_object_literal_id (context_p, ident_opts);

      if (is_private)
      {
        parser_check_duplicated_private_field (context_p,
                                               is_getter ? SCANNER_PRIVATE_FIELD_GETTER : SCANNER_PRIVATE_FIELD_SETTER);
      }

      literal_index = context_p->lit_object.index;

      if (context_p->token.type == LEXER_RIGHT_SQUARE)
      {
        is_computed = true;
      }
      else if (is_static && !is_private)
      {
        if (LEXER_IS_IDENT_OR_STRING (context_p->token.lit_location.type)
            && lexer_compare_identifier_to_string (&context_p->token.lit_location, (uint8_t *) "prototype", 9))
        {
          parser_raise_error (context_p, PARSER_ERR_CLASS_STATIC_PROTOTYPE);
        }
      }
      else if (parser_is_constructor_literal (context_p))
      {
        JERRY_ASSERT (!is_static || is_private);
        parser_raise_error (context_p, PARSER_ERR_CLASS_CONSTRUCTOR_AS_ACCESSOR);
      }

      function_literal_index = lexer_construct_function_object (context_p, accessor_status_flags);

      parser_emit_cbc_literal (context_p, CBC_PUSH_LITERAL, literal_index);

      JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_LITERAL);

      cbc_ext_opcode_t opcode;

      if (is_computed)
      {
        context_p->last_cbc.literal_index = function_literal_index;

        if (is_getter)
        {
          opcode = is_static ? CBC_EXT_SET_STATIC_COMPUTED_GETTER : CBC_EXT_SET_COMPUTED_GETTER;
        }
        else
        {
          opcode = is_static ? CBC_EXT_SET_STATIC_COMPUTED_SETTER : CBC_EXT_SET_COMPUTED_SETTER;
        }
      }
      else
      {
        context_p->last_cbc.value = function_literal_index;

        if (is_getter)
        {
          opcode = is_static ? (is_private ? CBC_EXT_COLLECT_PRIVATE_STATIC_GETTER : CBC_EXT_SET_STATIC_GETTER)
                             : (is_private ? CBC_EXT_COLLECT_PRIVATE_GETTER : CBC_EXT_SET_GETTER);
        }
        else
        {
          opcode = is_static ? (is_private ? CBC_EXT_COLLECT_PRIVATE_STATIC_SETTER : CBC_EXT_SET_STATIC_SETTER)
                             : (is_private ? CBC_EXT_COLLECT_PRIVATE_SETTER : CBC_EXT_SET_SETTER);
        }
      }

      if (is_computed)
      {
        parser_emit_cbc_ext (context_p,
                             is_getter ? CBC_EXT_SET_COMPUTED_GETTER_NAME : CBC_EXT_SET_COMPUTED_SETTER_NAME);
        parser_emit_cbc_ext (context_p, opcode);
      }
      else
      {
        if (is_private)
        {
          accessor_status_flags |= PARSER_PRIVATE_FUNCTION_NAME;
        }
        parser_set_function_name (context_p, function_literal_index, literal_index, accessor_status_flags);
        context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (opcode);
      }

      is_static = false;
      is_private = false;
      continue;
    }

    uint32_t status_flags = PARSER_FUNCTION_CLOSURE | PARSER_ALLOW_SUPER;

    if (context_p->token.type == LEXER_KEYW_ASYNC)
    {
      status_flags |= PARSER_IS_ASYNC_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;

      uint8_t ident_opts = LEXER_OBJ_IDENT_ONLY_IDENTIFIERS;

      if (lexer_check_next_character (context_p, LIT_CHAR_HASHMARK))
      {
        lexer_next_token (context_p);
        context_p->token.flags |= LEXER_NO_SKIP_SPACES;
        ident_opts |= LEXER_OBJ_IDENT_CLASS_PRIVATE;
        is_private = true;
      }

      if (!lexer_consume_generator (context_p))
      {
        lexer_expect_object_literal_id (context_p, ident_opts);
      }

      if (is_private && context_p->token.type == LEXER_LITERAL)
      {
        if (parser_is_constructor_literal (context_p))
        {
          parser_raise_error (context_p, PARSER_ERR_CLASS_PRIVATE_CONSTRUCTOR);
        }

        parser_check_duplicated_private_field (context_p, SCANNER_PRIVATE_FIELD_PROPERTY_GETTER_SETTER);
      }
    }

    if (context_p->token.type == LEXER_MULTIPLY)
    {
      uint8_t ident_opts = LEXER_OBJ_IDENT_ONLY_IDENTIFIERS;

      if (lexer_check_next_character (context_p, LIT_CHAR_HASHMARK))
      {
        lexer_next_token (context_p);
        context_p->token.flags |= LEXER_NO_SKIP_SPACES;
        ident_opts |= LEXER_OBJ_IDENT_CLASS_PRIVATE;
        is_private = true;
      }

      lexer_expect_object_literal_id (context_p, ident_opts);

      status_flags |= PARSER_IS_GENERATOR_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;

      if (is_private && context_p->token.type == LEXER_LITERAL)
      {
        if (parser_is_constructor_literal (context_p))
        {
          parser_raise_error (context_p, PARSER_ERR_CLASS_PRIVATE_CONSTRUCTOR);
        }

        parser_check_duplicated_private_field (context_p, SCANNER_PRIVATE_FIELD_PROPERTY_GETTER_SETTER);
      }
    }

    bool is_static_block = context_p->token.type == LEXER_LEFT_BRACE;

    if (context_p->token.type == LEXER_RIGHT_SQUARE)
    {
      is_computed = true;
    }
    else if (!is_static_block && LEXER_IS_IDENT_OR_STRING (context_p->token.lit_location.type))
    {
      if (is_static && !is_private)
      {
        if (lexer_compare_identifier_to_string (&context_p->token.lit_location, (uint8_t *) "prototype", 9))
        {
          parser_raise_error (context_p, PARSER_ERR_CLASS_STATIC_PROTOTYPE);
        }
      }
      else if ((status_flags & (PARSER_IS_ASYNC_FUNCTION | PARSER_IS_GENERATOR_FUNCTION))
               && lexer_compare_literal_to_string (context_p, "constructor", 11))
      {
        parser_raise_error (context_p, PARSER_ERR_INVALID_CLASS_CONSTRUCTOR);
      }
    }

    if (!(status_flags & (PARSER_IS_ASYNC_FUNCTION | PARSER_IS_GENERATOR_FUNCTION)))
    {
      if (is_static_block || !lexer_check_next_character (context_p, LIT_CHAR_LEFT_PAREN))
      {
        /* Class field. */
        if (fields_size == 0)
        {
          parser_stack_push_uint8 (context_p, PARSER_CLASS_FIELD_END);
        }

        scanner_range_t range;
        uint8_t class_field_type = is_static ? PARSER_CLASS_FIELD_STATIC : 0;

        if (!is_computed)
        {
          if (is_private)
          {
            lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_STRING_LITERAL);
            uint8_t field_opcode = is_static ? CBC_EXT_COLLECT_PRIVATE_STATIC_FIELD : CBC_EXT_COLLECT_PRIVATE_FIELD;
            parser_emit_cbc_ext_literal_from_token (context_p, field_opcode);
          }

          if (is_static && !is_static_block && parser_is_constructor_literal (context_p))
          {
            parser_raise_error (context_p, PARSER_ERR_ARGUMENT_LIST_EXPECTED);
          }

          range.start_location.source_p = context_p->token.lit_location.char_p;
          range.start_location.line = context_p->token.line;
          range.start_location.column = context_p->token.column;
          class_field_type |= PARSER_CLASS_FIELD_NORMAL;

          if (context_p->token.lit_location.type == LEXER_STRING_LITERAL)
          {
            range.start_location.source_p--;
          }
        }
        else
        {
          if (++computed_field_count > ECMA_INTEGER_NUMBER_MAX)
          {
            parser_raise_error (context_p, PARSER_ERR_TOO_MANY_CLASS_FIELDS);
          }

          if (is_static && static_fields_literal_p == NULL)
          {
            static_fields_literal_p = lexer_construct_unused_literal (context_p);
            parser_emit_cbc_ext_literal (context_p,
                                         CBC_EXT_PUSH_STATIC_COMPUTED_FIELD_FUNC,
                                         (uint16_t) (context_p->literal_count++));
          }
          else
          {
            parser_emit_cbc_ext (context_p,
                                 (is_static ? CBC_EXT_ADD_STATIC_COMPUTED_FIELD : CBC_EXT_ADD_COMPUTED_FIELD));
          }
        }

        if (is_static_block)
        {
          class_field_type |= PARSER_CLASS_FIELD_STATIC_BLOCK;

          if (context_p->next_scanner_info_p->type != SCANNER_TYPE_CLASS_STATIC_BLOCK_END)
          {
            parser_flush_cbc (context_p);
            parser_parse_class_static_block (context_p);
          }

          JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_CLASS_STATIC_BLOCK_END);

          scanner_set_location (context_p, &((scanner_location_info_t *) context_p->next_scanner_info_p)->location);
          scanner_release_next (context_p, sizeof (scanner_location_info_t));
          JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_FUNCTION);
          range.start_location.source_p = context_p->next_scanner_info_p->source_p - 1;

          scanner_seek (context_p);

          parser_stack_push (context_p, &range.start_location, sizeof (scanner_location_t));
          fields_size += sizeof (scanner_location_t);

          lexer_consume_next_character (context_p);
        }
        else if (lexer_consume_assign (context_p))
        {
          class_field_type |= PARSER_CLASS_FIELD_INITIALIZED;

          if (context_p->next_scanner_info_p->source_p != context_p->source_p)
          {
            lexer_next_token (context_p);
            parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);
            parser_raise_error (context_p, PARSER_ERR_SEMICOLON_EXPECTED);
          }

          if (is_computed)
          {
            scanner_get_location (&range.start_location, context_p);
          }

          JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_CLASS_FIELD_INITIALIZER_END);
          range.source_end_p = ((scanner_location_info_t *) context_p->next_scanner_info_p)->location.source_p;

          scanner_set_location (context_p, &((scanner_location_info_t *) context_p->next_scanner_info_p)->location);
          scanner_release_next (context_p, sizeof (scanner_location_info_t));
          scanner_seek (context_p);

          parser_stack_push (context_p, &range, sizeof (scanner_range_t));
          fields_size += sizeof (scanner_range_t);
        }
        else
        {
          if (!(context_p->token.flags & LEXER_WAS_NEWLINE)
              && !lexer_check_next_characters (context_p, LIT_CHAR_SEMICOLON, LIT_CHAR_RIGHT_BRACE))
          {
            lexer_next_token (context_p);
            parser_raise_error (context_p, PARSER_ERR_SEMICOLON_EXPECTED);
          }

          if (!is_computed)
          {
            parser_stack_push (context_p, &range.start_location, sizeof (scanner_location_t));
            fields_size += sizeof (scanner_location_t);
          }
        }

        parser_stack_push_uint8 (context_p, class_field_type);
        fields_size++;
        is_static = false;
        is_private = false;
        continue;
      }

      if (!is_computed)
      {
        if (context_p->token.lit_location.type != LEXER_NUMBER_LITERAL)
        {
          JERRY_ASSERT (context_p->token.lit_location.type == LEXER_IDENT_LITERAL
                        || context_p->token.lit_location.type == LEXER_STRING_LITERAL);
          if (is_private)
          {
            parser_resolve_private_identifier (context_p);
          }
          else
          {
            lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_STRING_LITERAL);
          }
        }
        else
        {
          lexer_construct_number_object (context_p, false, false);
        }
      }
    }

    uint16_t literal_index = context_p->lit_object.index;
    uint16_t function_literal_index = lexer_construct_function_object (context_p, status_flags | PARSER_IS_METHOD);

    parser_emit_cbc_literal (context_p, CBC_PUSH_LITERAL, function_literal_index);

    if (is_computed)
    {
      parser_emit_cbc_ext (context_p, CBC_EXT_SET_COMPUTED_FUNCTION_NAME);
      parser_emit_cbc_ext (context_p, is_static ? CBC_EXT_SET_STATIC_COMPUTED_PROPERTY : CBC_EXT_SET_COMPUTED_PROPERTY);
      is_static = false;
      continue;
    }

    uint32_t function_name_status_flags = 0;

    if (is_private)
    {
      function_name_status_flags = PARSER_PRIVATE_FUNCTION_NAME;
    }

    parser_set_function_name (context_p, function_literal_index, literal_index, function_name_status_flags);

    JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_LITERAL);

    context_p->last_cbc.value = literal_index;

    if (is_static)
    {
      context_p->last_cbc_opcode = (is_private ? PARSER_TO_EXT_OPCODE (CBC_EXT_COLLECT_PRIVATE_STATIC_METHOD)
                                               : PARSER_TO_EXT_OPCODE (CBC_EXT_SET_STATIC_PROPERTY_LITERAL));
      is_static = false;
    }
    else if (is_private)
    {
      context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_COLLECT_PRIVATE_METHOD);
    }
    else
    {
      context_p->last_cbc_opcode = CBC_SET_LITERAL_PROPERTY;
    }

    is_private = false;
  }

  if (fields_size == 0)
  {
    return false;
  }

  parser_reverse_class_fields (context_p, fields_size);

  /* Since PARSER_IS_ARROW_FUNCTION and PARSER_CLASS_CONSTRUCTOR bits cannot
   * be set at the same time, this bit combination triggers class field parsing. */

  if (!(context_p->stack_top_uint8 & PARSER_CLASS_FIELD_STATIC))
  {
    lexer_literal_t *literal_p = lexer_construct_unused_literal (context_p);

    uint16_t function_literal_index = (uint16_t) (context_p->literal_count++);
    parser_emit_cbc_ext_literal (context_p, CBC_EXT_SET_FIELD_INIT, function_literal_index);
    parser_flush_cbc (context_p);

    literal_p->u.bytecode_p = parser_parse_class_fields (context_p);
    literal_p->type = LEXER_FUNCTION_LITERAL;
  }

  bool has_static_field = false;

  if (context_p->stack_top_uint8 & PARSER_CLASS_FIELD_STATIC)
  {
    if (static_fields_literal_p == NULL)
    {
      static_fields_literal_p = lexer_construct_unused_literal (context_p);
      uint16_t function_literal_index = (uint16_t) (context_p->literal_count++);
      parser_emit_cbc_ext_literal (context_p, CBC_EXT_PUSH_STATIC_FIELD_FUNC, function_literal_index);
    }

    parser_flush_cbc (context_p);
    static_fields_literal_p->u.bytecode_p = parser_parse_class_fields (context_p);
    static_fields_literal_p->type = LEXER_FUNCTION_LITERAL;

    has_static_field = true;
  }

  parser_stack_pop_uint8 (context_p);
  return has_static_field;
} /* parser_parse_class_body */

/**
 * Parse class statement or expression.
 */
void
parser_parse_class (parser_context_t *context_p, /**< context */
                    bool is_statement) /**< true - if class is parsed as a statement
                                        *   false - otherwise (as an expression) */
{
  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_CLASS);

  uint16_t class_ident_index = PARSER_INVALID_LITERAL_INDEX;
  uint16_t class_name_index = PARSER_INVALID_LITERAL_INDEX;
  parser_class_literal_opts_t opts = PARSER_CLASS_LITERAL_NO_OPTS;
  scanner_info_t *scanner_info_p = context_p->next_scanner_info_p;

  scanner_class_info_t *class_info_p = (scanner_class_info_t *) scanner_info_p;
  parser_private_context_t private_ctx;

  if (scanner_info_p->source_p == context_p->source_p)
  {
    JERRY_ASSERT (scanner_info_p->type == SCANNER_TYPE_CLASS_CONSTRUCTOR);
    parser_save_private_context (context_p, &private_ctx, class_info_p);

    if (scanner_info_p->u8_arg & SCANNER_CONSTRUCTOR_EXPLICIT)
    {
      opts = (parser_class_literal_opts_t) ((int) opts | (int) PARSER_CLASS_LITERAL_CTOR_PRESENT);
    }

    scanner_release_next (context_p, sizeof (scanner_class_info_t));
  }

  if (is_statement)
  {
    /* Class statement must contain an identifier. */
    lexer_expect_identifier (context_p, LEXER_IDENT_LITERAL);
    JERRY_ASSERT (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

    if (context_p->next_scanner_info_p->source_p == context_p->source_p)
    {
      JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_ERR_REDECLARED);
      parser_raise_error (context_p, PARSER_ERR_VARIABLE_REDECLARED);
    }
    class_ident_index = context_p->lit_object.index;

    lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_NEW_IDENT_LITERAL);
    context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_USED;
    class_name_index = context_p->lit_object.index;

#if JERRY_MODULE_SYSTEM
    parser_module_append_export_name (context_p);
    context_p->status_flags &= (uint32_t) ~PARSER_MODULE_STORE_IDENT;
#endif /* JERRY_MODULE_SYSTEM */

    lexer_next_token (context_p);
  }
  else
  {
    lexer_next_token (context_p);

    /* Class expression may contain an identifier. */
    if (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
    {
      lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_NEW_IDENT_LITERAL);
      context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_USED;
      class_name_index = context_p->lit_object.index;
      lexer_next_token (context_p);
    }
  }

  if (class_name_index != PARSER_INVALID_LITERAL_INDEX)
  {
    if (JERRY_UNLIKELY (context_p->scope_stack_top >= context_p->scope_stack_size))
    {
      JERRY_ASSERT (context_p->scope_stack_size == PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK);
      parser_raise_error (context_p, PARSER_ERR_SCOPE_STACK_LIMIT_REACHED);
    }

    parser_scope_stack_t *scope_stack_p = context_p->scope_stack_p + context_p->scope_stack_top;

    PARSER_PLUS_EQUAL_U16 (context_p->scope_stack_top, 1);
    scope_stack_p->map_from = class_name_index;
    scope_stack_p->map_to = 0;

    parser_emit_cbc_ext_literal (context_p, CBC_EXT_PUSH_NAMED_CLASS_ENV, class_name_index);
  }
  else
  {
    parser_emit_cbc (context_p, CBC_PUSH_UNDEFINED);
  }

  bool is_strict = (context_p->status_flags & PARSER_IS_STRICT) != 0;

  /* 14.5. A ClassBody is always strict code. */
  context_p->status_flags |= PARSER_IS_STRICT;

  if (context_p->token.type == LEXER_KEYW_EXTENDS)
  {
    lexer_next_token (context_p);
    parser_parse_expression (context_p, PARSE_EXPR | PARSE_EXPR_LEFT_HAND_SIDE);
    opts = (parser_class_literal_opts_t) ((int) opts | (int) PARSER_CLASS_LITERAL_HERITAGE_PRESENT);
  }
  else
  {
    /* Elisions represents that the classHeritage is not present */
    parser_emit_cbc (context_p, CBC_PUSH_ELISION);
  }

  if (context_p->token.type != LEXER_LEFT_BRACE)
  {
    parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_EXPECTED);
  }

  context_p->private_context_p->opts |= SCANNER_PRIVATE_FIELD_ACTIVE;

  /* ClassDeclaration is parsed. Continue with class body. */
  bool has_static_field = parser_parse_class_body (context_p, opts, class_name_index);

  if (class_name_index != PARSER_INVALID_LITERAL_INDEX)
  {
    parser_emit_cbc_ext_literal (context_p, CBC_EXT_FINALIZE_NAMED_CLASS, class_name_index);
    PARSER_MINUS_EQUAL_U16 (context_p->scope_stack_top, 1);
  }
  else
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_FINALIZE_ANONYMOUS_CLASS);
  }

  if (has_static_field)
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_RUN_STATIC_FIELD_INIT);
  }

  if (is_statement)
  {
    cbc_opcode_t opcode = CBC_MOV_IDENT;

    if (class_ident_index < PARSER_REGISTER_START)
    {
      opcode = (scanner_literal_is_created (context_p, class_ident_index) ? CBC_ASSIGN_LET_CONST : CBC_INIT_LET);
    }

    parser_emit_cbc_literal (context_p, (uint16_t) opcode, class_ident_index);
    parser_flush_cbc (context_p);
  }

  if (!is_strict)
  {
    /* Restore flag */
    context_p->status_flags &= (uint32_t) ~PARSER_IS_STRICT;
  }
  context_p->status_flags &= (uint32_t) ~PARSER_ALLOW_SUPER;

  parser_restore_private_context (context_p, &private_ctx);

  lexer_next_token (context_p);
} /* parser_parse_class */

/**
 * Parse object initializer method definition.
 *
 * See also: ES2015 14.3
 */
static void
parser_parse_object_method (parser_context_t *context_p) /**< context */
{
  context_p->source_p--;
  context_p->column--;
  uint16_t function_literal_index =
    lexer_construct_function_object (context_p, (PARSER_FUNCTION_CLOSURE | PARSER_ALLOW_SUPER | PARSER_IS_METHOD));

  parser_emit_cbc_literal (context_p, CBC_PUSH_LITERAL, function_literal_index);

  context_p->last_cbc.literal_type = LEXER_FUNCTION_LITERAL;

  lexer_next_token (context_p);
} /* parser_parse_object_method */

/**
 * Reparse the current literal as a common identifier.
 */
static void
parser_reparse_as_common_identifier (parser_context_t *context_p, /**< context */
                                     parser_line_counter_t start_line, /**< start line */
                                     parser_line_counter_t start_column) /**< start column */
{
  /* context_p->token.lit_location.char_p is showing the character after the string start,
     so it is not suitable for reparsing as identifier.
     e.g.: { 'foo' } */
  if (context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
  {
    parser_raise_error (context_p, PARSER_ERR_IDENTIFIER_EXPECTED);
  }

  context_p->source_p = context_p->token.lit_location.char_p;
  context_p->line = start_line;
  context_p->column = start_column;

  lexer_next_token (context_p);

  if (context_p->token.type != LEXER_LITERAL)
  {
    parser_raise_error (context_p, PARSER_ERR_IDENTIFIER_EXPECTED);
  }

  JERRY_ASSERT (context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

  lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_IDENT_LITERAL);

} /* parser_reparse_as_common_identifier */

/**
 * Parse object literal.
 */
static void
parser_parse_object_literal (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (context_p->token.type == LEXER_LEFT_BRACE);

  parser_emit_cbc (context_p, CBC_CREATE_OBJECT);

  bool proto_seen = false;
  bool has_super_env = false;

  if (context_p->next_scanner_info_p->source_p == context_p->source_p)
  {
    JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS);

    if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_SUPER)
    {
      parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_OBJECT_SUPER_ENVIRONMENT);
      has_super_env = true;
    }

    scanner_release_next (context_p, sizeof (scanner_info_t));
  }

  while (true)
  {
    lexer_expect_object_literal_id (context_p, LEXER_OBJ_IDENT_SET_FUNCTION_START);

    switch (context_p->token.type)
    {
      case LEXER_RIGHT_BRACE:
      {
        break;
      }
      case LEXER_PROPERTY_GETTER:
      case LEXER_PROPERTY_SETTER:
      {
        uint32_t status_flags;
        cbc_ext_opcode_t opcode;
        bool is_getter = context_p->token.type == LEXER_PROPERTY_GETTER;

        if (is_getter)
        {
          status_flags = PARSER_FUNCTION_CLOSURE | PARSER_IS_PROPERTY_GETTER;
          opcode = CBC_EXT_SET_GETTER;
        }
        else
        {
          status_flags = PARSER_FUNCTION_CLOSURE | PARSER_IS_PROPERTY_SETTER;
          opcode = CBC_EXT_SET_SETTER;
        }

        status_flags |= PARSER_ALLOW_SUPER;

        lexer_expect_object_literal_id (context_p, LEXER_OBJ_IDENT_ONLY_IDENTIFIERS);

        /* This assignment is a nop for computed getters/setters. */
        uint16_t literal_index = context_p->lit_object.index;

        bool is_computed = context_p->token.type == LEXER_RIGHT_SQUARE;

        if (is_computed)
        {
          opcode = ((opcode == CBC_EXT_SET_GETTER) ? CBC_EXT_SET_COMPUTED_GETTER : CBC_EXT_SET_COMPUTED_SETTER);
        }

        uint16_t function_literal_index = lexer_construct_function_object (context_p, status_flags);

        if (opcode >= CBC_EXT_SET_COMPUTED_GETTER)
        {
          literal_index = function_literal_index;
        }

        parser_emit_cbc_literal (context_p, CBC_PUSH_LITERAL, literal_index);

        JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_LITERAL);

        if (is_computed)
        {
          parser_emit_cbc_ext (context_p,
                               is_getter ? CBC_EXT_SET_COMPUTED_GETTER_NAME : CBC_EXT_SET_COMPUTED_SETTER_NAME);

          if (has_super_env)
          {
            parser_emit_cbc_ext (context_p, CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT_COMPUTED);
          }
          parser_emit_cbc_ext (context_p, opcode);
          lexer_next_token (context_p);
          break;
        }

        parser_set_function_name (context_p, function_literal_index, literal_index, status_flags);

        if (has_super_env)
        {
          context_p->last_cbc_opcode = CBC_PUSH_TWO_LITERALS;
          context_p->last_cbc.value = function_literal_index;
          parser_emit_cbc_ext (context_p, CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT_COMPUTED);
          parser_emit_cbc_ext (context_p, is_getter ? CBC_EXT_SET_COMPUTED_GETTER : CBC_EXT_SET_COMPUTED_SETTER);
        }
        else
        {
          context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (opcode);
          context_p->last_cbc.value = function_literal_index;
        }

        lexer_next_token (context_p);
        break;
      }
      case LEXER_RIGHT_SQUARE:
      {
        lexer_next_token (context_p);

        if (context_p->token.type == LEXER_LEFT_PAREN)
        {
          parser_parse_object_method (context_p);
          JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_LITERAL);

          if (parser_check_anonymous_function_declaration (context_p) < PARSER_NAMED_FUNCTION)
          {
            parser_emit_cbc_ext (context_p, CBC_EXT_SET_COMPUTED_FUNCTION_NAME);
            if (has_super_env)
            {
              parser_emit_cbc_ext (context_p, CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT_COMPUTED);
            }
            parser_emit_cbc_ext (context_p, CBC_EXT_SET_COMPUTED_PROPERTY);
          }
          else
          {
            context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_SET_COMPUTED_PROPERTY_LITERAL);
          }

          break;
        }

        if (context_p->token.type != LEXER_COLON)
        {
          parser_raise_error (context_p, PARSER_ERR_COLON_EXPECTED);
        }

        lexer_next_token (context_p);
        parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

        if (parser_check_anonymous_function_declaration (context_p) < PARSER_NAMED_FUNCTION)
        {
          parser_emit_cbc_ext (context_p, CBC_EXT_SET_COMPUTED_FUNCTION_NAME);
        }

        if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
        {
          context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_SET_COMPUTED_PROPERTY_LITERAL);
        }
        else
        {
          parser_emit_cbc_ext (context_p, CBC_EXT_SET_COMPUTED_PROPERTY);
        }
        break;
      }
      case LEXER_THREE_DOTS:
      {
        lexer_next_token (context_p);
        parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);
        parser_emit_cbc_ext (context_p, CBC_EXT_COPY_DATA_PROPERTIES);
        break;
      }
      case LEXER_KEYW_ASYNC:
      case LEXER_MULTIPLY:
      {
        uint32_t status_flags = PARSER_FUNCTION_CLOSURE;

        if (context_p->token.type == LEXER_KEYW_ASYNC)
        {
          status_flags |= PARSER_IS_ASYNC_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;
          lexer_consume_generator (context_p);
        }

        if (context_p->token.type == LEXER_MULTIPLY)
        {
          status_flags |= PARSER_IS_GENERATOR_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;
        }

        if (has_super_env)
        {
          status_flags |= PARSER_ALLOW_SUPER;
        }

        lexer_expect_object_literal_id (context_p, LEXER_OBJ_IDENT_ONLY_IDENTIFIERS);

        uint16_t opcode = CBC_SET_LITERAL_PROPERTY;
        /* This assignment is a nop for CBC_EXT_SET_COMPUTED_PROPERTY_LITERAL. */
        uint16_t literal_index = context_p->lit_object.index;
        bool is_computed = context_p->token.type == LEXER_RIGHT_SQUARE;

        if (is_computed)
        {
          opcode = CBC_EXT_SET_COMPUTED_PROPERTY;
        }

        uint16_t function_literal_index = lexer_construct_function_object (context_p, status_flags);

        parser_emit_cbc_literal (context_p, CBC_PUSH_LITERAL, function_literal_index);

        JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_LITERAL);

        if (is_computed)
        {
          parser_emit_cbc_ext (context_p, CBC_EXT_SET_COMPUTED_FUNCTION_NAME);
          if (has_super_env)
          {
            parser_emit_cbc_ext (context_p, CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT_COMPUTED);
          }
          parser_emit_cbc_ext (context_p, opcode);
          lexer_next_token (context_p);
          break;
        }

        parser_set_function_name (context_p, function_literal_index, literal_index, status_flags);

        if (has_super_env)
        {
          parser_emit_cbc_ext (context_p, CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT);
          parser_emit_cbc_literal (context_p, CBC_SET_PROPERTY, literal_index);
        }
        else
        {
          context_p->last_cbc_opcode = opcode;
          context_p->last_cbc.value = literal_index;
        }

        lexer_next_token (context_p);
        break;
      }
      default:
      {
        const lexer_lit_location_t *literal_p = (const lexer_lit_location_t *) context_p->lit_object.literal_p;
        bool is_proto = ((context_p->token.lit_location.type == LEXER_IDENT_LITERAL
                          || context_p->token.lit_location.type == LEXER_STRING_LITERAL)
                         && lexer_compare_identifier_to_string (literal_p, (uint8_t *) "__proto__", 9)
                         && lexer_check_next_character (context_p, LIT_CHAR_COLON));
        if (is_proto)
        {
          if (proto_seen)
          {
            parser_raise_error (context_p, PARSER_ERR_DUPLICATED_PROTO);
          }

          proto_seen = true;
        }

        uint16_t literal_index = context_p->lit_object.index;
        parser_line_counter_t start_line = context_p->token.line;
        parser_line_counter_t start_column = context_p->token.column;

        lexer_next_token (context_p);

        if (context_p->token.type == LEXER_LEFT_PAREN && !is_proto)
        {
          parser_parse_object_method (context_p);

          JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_LITERAL);
          parser_set_function_name (context_p, context_p->last_cbc.literal_index, literal_index, 0);

          if (has_super_env)
          {
            parser_emit_cbc_ext (context_p, CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT);
            parser_emit_cbc_literal (context_p, CBC_SET_PROPERTY, literal_index);
            break;
          }

          context_p->last_cbc_opcode = CBC_SET_LITERAL_PROPERTY;
          context_p->last_cbc.value = literal_index;
          break;
        }

        if ((context_p->token.type == LEXER_RIGHT_BRACE || context_p->token.type == LEXER_COMMA) && !is_proto)
        {
          parser_reparse_as_common_identifier (context_p, start_line, start_column);
          parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_LITERAL);

          context_p->last_cbc_opcode = CBC_SET_LITERAL_PROPERTY;
          context_p->last_cbc.value = literal_index;

          lexer_next_token (context_p);
          break;
        }

        if (context_p->token.type != LEXER_COLON)
        {
          parser_raise_error (context_p, PARSER_ERR_COLON_EXPECTED);
        }

        lexer_next_token (context_p);
        parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

        if (is_proto)
        {
          parser_emit_cbc_ext (context_p, CBC_EXT_SET__PROTO__);
          break;
        }

        if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
        {
          if (context_p->last_cbc.literal_type == LEXER_FUNCTION_LITERAL)
          {
            parser_set_function_name (context_p, context_p->last_cbc.literal_index, literal_index, 0);
          }
          context_p->last_cbc_opcode = CBC_SET_LITERAL_PROPERTY;
          context_p->last_cbc.value = literal_index;
        }
        else
        {
          if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_FINALIZE_ANONYMOUS_CLASS))
          {
            uint16_t name_index = scanner_save_literal (context_p, literal_index);
            parser_emit_cbc_ext_literal (context_p, CBC_EXT_SET_CLASS_NAME, name_index);
          }

          parser_emit_cbc_literal (context_p, CBC_SET_PROPERTY, literal_index);
        }

        break;
      }
    }

    if (context_p->token.type == LEXER_RIGHT_BRACE)
    {
      break;
    }
    else if (context_p->token.type != LEXER_COMMA)
    {
      parser_raise_error (context_p, PARSER_ERR_OBJECT_ITEM_SEPARATOR_EXPECTED);
    }
  }

  if (has_super_env)
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_POP_OBJECT_SUPER_ENVIRONMENT);
  }
} /* parser_parse_object_literal */

/**
 * Parse function literal.
 */
static void
parser_parse_function_expression (parser_context_t *context_p, /**< context */
                                  uint32_t status_flags) /**< function status flags */
{
  int literals = 0;
  uint16_t literal1 = 0;
  uint16_t literal2 = 0;
  uint16_t function_literal_index;
  int32_t function_name_index = -1;

  if (status_flags & PARSER_IS_FUNC_EXPRESSION)
  {
    uint32_t parent_status_flags = context_p->status_flags;

    context_p->status_flags &=
      (uint32_t) ~(PARSER_IS_ASYNC_FUNCTION | PARSER_IS_GENERATOR_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD);

    if (status_flags & PARSER_IS_ASYNC_FUNCTION)
    {
      /* The name of the function cannot be await. */
      context_p->status_flags |= PARSER_IS_ASYNC_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;
    }

    if (lexer_consume_generator (context_p))
    {
      /* The name of the function cannot be yield. */
      context_p->status_flags |= PARSER_IS_GENERATOR_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;
      status_flags |= PARSER_IS_GENERATOR_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;
    }

    if (!lexer_check_next_character (context_p, LIT_CHAR_LEFT_PAREN))
    {
      /* The `await` keyword is interpreted as an IdentifierReference within function expressions */
      context_p->status_flags &= (uint32_t) ~PARSER_IS_CLASS_STATIC_BLOCK;

      lexer_next_token (context_p);

      context_p->status_flags |= parent_status_flags & PARSER_IS_CLASS_STATIC_BLOCK;

      if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
      {
        parser_raise_error (context_p, PARSER_ERR_IDENTIFIER_EXPECTED);
      }

      parser_flush_cbc (context_p);

      lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_STRING_LITERAL);

      if (context_p->token.keyword_type >= LEXER_FIRST_NON_STRICT_ARGUMENTS)
      {
        status_flags |= PARSER_HAS_NON_STRICT_ARG;
      }

      function_name_index = context_p->lit_object.index;
    }

    context_p->status_flags = parent_status_flags;
  }

  if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
  {
    literals = 1;
    literal1 = context_p->last_cbc.literal_index;
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_TWO_LITERALS)
  {
    literals = 2;
    literal1 = context_p->last_cbc.literal_index;
    literal2 = context_p->last_cbc.value;
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
  }

  function_literal_index = lexer_construct_function_object (context_p, status_flags);

  JERRY_ASSERT (context_p->last_cbc_opcode == PARSER_CBC_UNAVAILABLE);

  if (function_name_index != -1)
  {
    parser_set_function_name (context_p, function_literal_index, (uint16_t) function_name_index, 0);
  }

  if (literals == 1)
  {
    context_p->last_cbc_opcode = CBC_PUSH_TWO_LITERALS;
    context_p->last_cbc.literal_index = literal1;
    context_p->last_cbc.value = function_literal_index;
  }
  else if (literals == 2)
  {
    context_p->last_cbc_opcode = CBC_PUSH_THREE_LITERALS;
    context_p->last_cbc.literal_index = literal1;
    context_p->last_cbc.value = literal2;
    context_p->last_cbc.third_literal_index = function_literal_index;
  }
  else
  {
    parser_emit_cbc_literal (context_p, CBC_PUSH_LITERAL, function_literal_index);

    if (function_name_index != -1)
    {
      context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_NAMED_FUNC_EXPRESSION);
      context_p->last_cbc.value = (uint16_t) function_name_index;
    }
  }

  context_p->last_cbc.literal_type = LEXER_FUNCTION_LITERAL;
  context_p->last_cbc.literal_keyword_type = LEXER_EOS;
} /* parser_parse_function_expression */

/**
 * Parse template literal.
 */
static void
parser_parse_template_literal (parser_context_t *context_p) /**< context */
{
  bool is_empty_head = true;

  if (context_p->token.lit_location.length > 0)
  {
    is_empty_head = false;

    lexer_construct_literal_object (context_p, &context_p->token.lit_location, context_p->token.lit_location.type);

    parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_LITERAL);
  }

  lexer_next_token (context_p);
  parser_parse_expression (context_p, PARSE_EXPR);

  if (context_p->token.type != LEXER_RIGHT_BRACE)
  {
    parser_raise_error (context_p, PARSER_ERR_RIGHT_BRACE_EXPECTED);
  }

  if (!is_empty_head)
  {
    if (context_p->last_cbc_opcode == CBC_PUSH_TWO_LITERALS)
    {
      context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_STRING_CONCAT_TWO_LITERALS);
    }
    else if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
    {
      context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_STRING_CONCAT_RIGHT_LITERAL);
    }
    else
    {
      parser_emit_cbc_ext (context_p, CBC_EXT_STRING_CONCAT);
    }
  }

  context_p->source_p--;
  context_p->column--;
  lexer_parse_string (context_p, LEXER_STRING_NO_OPTS);

  if (is_empty_head || context_p->token.lit_location.length > 0)
  {
    lexer_construct_literal_object (context_p, &context_p->token.lit_location, context_p->token.lit_location.type);

    if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
    {
      context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_STRING_CONCAT_TWO_LITERALS);
      context_p->last_cbc.value = context_p->lit_object.index;
      context_p->last_cbc.literal_type = context_p->token.lit_location.type;
      context_p->last_cbc.literal_keyword_type = context_p->token.keyword_type;
    }
    else
    {
      parser_emit_cbc_ext_literal_from_token (context_p, CBC_EXT_STRING_CONCAT_RIGHT_LITERAL);
    }
  }

  while (context_p->source_p[-1] != LIT_CHAR_GRAVE_ACCENT)
  {
    lexer_next_token (context_p);

    parser_parse_expression (context_p, PARSE_EXPR);

    if (context_p->token.type != LEXER_RIGHT_BRACE)
    {
      parser_raise_error (context_p, PARSER_ERR_RIGHT_BRACE_EXPECTED);
    }

    if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
    {
      context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_STRING_CONCAT_RIGHT_LITERAL);
    }
    else
    {
      parser_emit_cbc_ext (context_p, CBC_EXT_STRING_CONCAT);
    }

    context_p->source_p--;
    context_p->column--;
    lexer_parse_string (context_p, LEXER_STRING_NO_OPTS);

    if (context_p->token.lit_location.length > 0)
    {
      lexer_construct_literal_object (context_p, &context_p->token.lit_location, context_p->token.lit_location.type);

      parser_emit_cbc_ext_literal_from_token (context_p, CBC_EXT_STRING_CONCAT_RIGHT_LITERAL);
    }
  }
} /* parser_parse_template_literal */

/**
 * Parse tagged template literal.
 */
static size_t
parser_parse_tagged_template_literal (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (context_p->token.type == LEXER_TEMPLATE_LITERAL);

  uint32_t call_arguments = 0;
  ecma_collection_t *collection_p;

  if (context_p->tagged_template_literal_cp == JMEM_CP_NULL)
  {
    collection_p = ecma_new_collection ();
    ECMA_SET_INTERNAL_VALUE_POINTER (context_p->tagged_template_literal_cp, collection_p);
  }
  else
  {
    collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, context_p->tagged_template_literal_cp);
    if (collection_p->item_count > CBC_MAXIMUM_BYTE_VALUE)
    {
      parser_raise_error (context_p, PARSER_ERR_ARGUMENT_LIMIT_REACHED);
    }
  }

  const uint32_t tagged_id = collection_p->item_count;
  uint32_t prop_idx = 0;
  ecma_object_t *raw_strings_p;
  ecma_object_t *template_obj_p = parser_new_tagged_template_literal (&raw_strings_p);
  ecma_collection_push_back (collection_p, ecma_make_object_value (template_obj_p));

  parser_tagged_template_literal_append_strings (context_p, template_obj_p, raw_strings_p, prop_idx++);

  call_arguments++;
  parser_emit_cbc_ext_call (context_p, CBC_EXT_GET_TAGGED_TEMPLATE_LITERAL, tagged_id);

  while (context_p->source_p[-1] != LIT_CHAR_GRAVE_ACCENT)
  {
    JERRY_ASSERT (context_p->source_p[-1] == LIT_CHAR_LEFT_BRACE);
    lexer_next_token (context_p);

    if (++call_arguments > CBC_MAXIMUM_BYTE_VALUE)
    {
      parser_raise_error (context_p, PARSER_ERR_ARGUMENT_LIMIT_REACHED);
    }

    parser_parse_expression (context_p, PARSE_EXPR);

    if (context_p->token.type != LEXER_RIGHT_BRACE)
    {
      parser_raise_error (context_p, PARSER_ERR_RIGHT_BRACE_EXPECTED);
    }

    context_p->source_p--;
    context_p->column--;
    lexer_parse_string (context_p, LEXER_STRING_NO_OPTS);

    parser_tagged_template_literal_append_strings (context_p, template_obj_p, raw_strings_p, prop_idx++);
  }

  parser_tagged_template_literal_finalize (template_obj_p, raw_strings_p);

  return call_arguments;
} /* parser_parse_tagged_template_literal */

/**
 * Checks whether the current expression can be an assignment expression.
 *
 * @return true if the current expression can be an assignment expression, false otherwise
 */
static inline bool
parser_is_assignment_expr (parser_context_t *context_p)
{
  return (context_p->stack_top_uint8 == LEXER_EXPRESSION_START || context_p->stack_top_uint8 == LEXER_LEFT_PAREN
          || context_p->stack_top_uint8 == LEXER_COMMA_SEP_LIST
          || LEXER_IS_BINARY_LVALUE_OP_TOKEN (context_p->stack_top_uint8));
} /* parser_is_assignment_expr */

/**
 * Throws an error if the current expression is not an assignment expression.
 */
static inline void
parser_check_assignment_expr (parser_context_t *context_p)
{
  if (!parser_is_assignment_expr (context_p))
  {
    parser_raise_error (context_p, PARSER_ERR_ASSIGNMENT_EXPECTED);
  }
} /* parser_check_assignment_expr */

/**
 * Checks whether the next token is a valid continuation token after an AssignmentExpression.
 */
static inline bool
parser_abort_parsing_after_assignment_expression (parser_context_t *context_p)
{
  return (context_p->token.type != LEXER_RIGHT_PAREN && context_p->token.type != LEXER_COMMA);
} /* parser_abort_parsing_after_assignment_expression */

/**
 * Parse and record unary operators, and parse the primary literal.
 *
 * @return true if parsing should be aborted, true otherwise
 */
static bool
parser_parse_unary_expression (parser_context_t *context_p, /**< context */
                               size_t *grouping_level_p) /**< grouping level */
{
  bool new_was_seen = false;

  /* Collect unary operators. */
  while (true)
  {
    /* Convert plus and minus binary operators to unary operators. */
    switch (context_p->token.type)
    {
      case LEXER_ADD:
      {
        context_p->token.type = LEXER_PLUS;
        break;
      }
      case LEXER_SUBTRACT:
      {
        context_p->token.type = LEXER_NEGATE;
        break;
      }
      case LEXER_KEYW_AWAIT:
      {
#if JERRY_MODULE_SYSTEM
        if ((context_p->global_status_flags & ECMA_PARSE_MODULE)
            && !(context_p->status_flags & PARSER_IS_ASYNC_FUNCTION))
        {
          parser_raise_error (context_p, PARSER_ERR_AWAIT_NOT_ALLOWED);
        }
#endif /* JERRY_MODULE_SYSTEM */

        if (JERRY_UNLIKELY (context_p->token.lit_location.status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE))
        {
          parser_raise_error (context_p, PARSER_ERR_INVALID_KEYWORD);
        }
        break;
      }
    }

    /* Bracketed expressions are primary expressions. At this
     * point their left paren is pushed onto the stack and
     * they are processed when their closing paren is reached. */
    if (context_p->token.type == LEXER_LEFT_PAREN)
    {
      if (context_p->next_scanner_info_p->source_p == context_p->source_p)
      {
        JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_FUNCTION);
        break;
      }
      (*grouping_level_p) += PARSER_GROUPING_LEVEL_INCREASE;
      new_was_seen = false;
    }
    else if (context_p->token.type == LEXER_KEYW_NEW)
    {
      /* After 'new' unary operators are not allowed. */
      new_was_seen = true;

      /* Check if "new.target" is written here. */
      if (scanner_try_scan_new_target (context_p))
      {
        if (!(context_p->status_flags & PARSER_ALLOW_NEW_TARGET))
        {
          parser_raise_error (context_p, PARSER_ERR_NEW_TARGET_NOT_ALLOWED);
        }

        parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_NEW_TARGET);
        lexer_next_token (context_p);
        /* Found "new.target" return here */
        return false;
      }
    }
    else if (new_was_seen || (*grouping_level_p == PARSE_EXPR_LEFT_HAND_SIDE)
             || !LEXER_IS_UNARY_OP_TOKEN (context_p->token.type))
    {
      break;
    }

    parser_stack_push_uint8 (context_p, context_p->token.type);
    lexer_next_token (context_p);
  }

  /* Parse primary expression. */
  switch (context_p->token.type)
  {
    case LEXER_HASHMARK:
    {
      if (!lexer_scan_private_identifier (context_p))
      {
        parser_raise_error (context_p, PARSER_ERR_INVALID_CHARACTER);
      }

      parser_resolve_private_identifier (context_p);

      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_KEYW_IN)
      {
        parser_raise_error (context_p, PARSER_ERR_INVALID_CHARACTER);
      }

      parser_stack_push_uint16 (context_p, context_p->lit_object.index);
      parser_stack_push_uint8 (context_p, LEXER_PRIVATE_PRIMARY_EXPR);
      return false;
    }
    case LEXER_TEMPLATE_LITERAL:
    {
      if (context_p->source_p[-1] != LIT_CHAR_GRAVE_ACCENT)
      {
        parser_parse_template_literal (context_p);
        break;
      }

      /* The string is a normal string literal. */
      /* FALLTHRU */
    }
    case LEXER_LITERAL:
    {
      if (JERRY_UNLIKELY (context_p->next_scanner_info_p->source_p == context_p->source_p))
      {
        JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_FUNCTION);

        uint32_t arrow_status_flags =
          (PARSER_IS_FUNCTION | PARSER_IS_ARROW_FUNCTION
           | (context_p->status_flags & (PARSER_INSIDE_CLASS_FIELD | PARSER_IS_CLASS_STATIC_BLOCK)));

        if (context_p->next_scanner_info_p->u8_arg & SCANNER_FUNCTION_ASYNC)
        {
          JERRY_ASSERT (lexer_token_is_async (context_p));
          JERRY_ASSERT (!(context_p->next_scanner_info_p->u8_arg & SCANNER_FUNCTION_STATEMENT));

          uint32_t saved_status_flags = context_p->status_flags;

          context_p->status_flags |= PARSER_IS_ASYNC_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;
          lexer_next_token (context_p);
          context_p->status_flags = saved_status_flags;

          if (context_p->token.type == LEXER_KEYW_FUNCTION)
          {
            uint32_t status_flags = (PARSER_FUNCTION_CLOSURE | PARSER_IS_FUNC_EXPRESSION | PARSER_IS_ASYNC_FUNCTION
                                     | PARSER_DISALLOW_AWAIT_YIELD);
            parser_parse_function_expression (context_p, status_flags);
            break;
          }

          arrow_status_flags =
            (PARSER_IS_FUNCTION | PARSER_IS_ARROW_FUNCTION | PARSER_IS_ASYNC_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD);
        }

        parser_check_assignment_expr (context_p);
        parser_parse_function_expression (context_p, arrow_status_flags);
        return parser_abort_parsing_after_assignment_expression (context_p);
      }

      uint8_t type = context_p->token.lit_location.type;

      if (type == LEXER_IDENT_LITERAL || type == LEXER_STRING_LITERAL)
      {
        lexer_construct_literal_object (context_p, &context_p->token.lit_location, context_p->token.lit_location.type);
      }
      else if (type == LEXER_NUMBER_LITERAL)
      {
        bool is_negative_number = false;

        if ((context_p->stack_top_uint8 == LEXER_PLUS || context_p->stack_top_uint8 == LEXER_NEGATE)
            && !lexer_check_post_primary_exp (context_p))
        {
          do
          {
            if (context_p->stack_top_uint8 == LEXER_NEGATE)
            {
              is_negative_number = !is_negative_number;
            }
#if JERRY_BUILTIN_BIGINT
            else if (JERRY_LIKELY (context_p->token.extra_value == LEXER_NUMBER_BIGINT))
            {
              break;
            }
#endif /* JERRY_BUILTIN_BIGINT */
            parser_stack_pop_uint8 (context_p);
          } while (context_p->stack_top_uint8 == LEXER_PLUS || context_p->stack_top_uint8 == LEXER_NEGATE);
        }

        if (lexer_construct_number_object (context_p, true, is_negative_number))
        {
          JERRY_ASSERT (context_p->lit_object.index <= CBC_PUSH_NUMBER_BYTE_RANGE_END);

          parser_emit_cbc_push_number (context_p, is_negative_number);
          break;
        }
      }

      cbc_opcode_t opcode = CBC_PUSH_LITERAL;

      if (context_p->token.keyword_type != LEXER_KEYW_EVAL)
      {
        if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
        {
          context_p->last_cbc_opcode = CBC_PUSH_TWO_LITERALS;
          context_p->last_cbc.value = context_p->lit_object.index;
          context_p->last_cbc.literal_type = context_p->token.lit_location.type;
          context_p->last_cbc.literal_keyword_type = context_p->token.keyword_type;
          break;
        }

        if (context_p->last_cbc_opcode == CBC_PUSH_TWO_LITERALS)
        {
          context_p->last_cbc_opcode = CBC_PUSH_THREE_LITERALS;
          context_p->last_cbc.third_literal_index = context_p->lit_object.index;
          context_p->last_cbc.literal_type = context_p->token.lit_location.type;
          context_p->last_cbc.literal_keyword_type = context_p->token.keyword_type;
          break;
        }

        if (context_p->last_cbc_opcode == CBC_PUSH_THIS)
        {
          context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
          opcode = CBC_PUSH_THIS_LITERAL;
        }
      }

      parser_emit_cbc_literal_from_token (context_p, (uint16_t) opcode);
      break;
    }
    case LEXER_KEYW_FUNCTION:
    {
      parser_parse_function_expression (context_p, PARSER_FUNCTION_CLOSURE | PARSER_IS_FUNC_EXPRESSION);
      break;
    }
    case LEXER_LEFT_BRACE:
    {
      if (context_p->next_scanner_info_p->source_p == context_p->source_p
          && context_p->next_scanner_info_p->type == SCANNER_TYPE_INITIALIZER)
      {
        if (parser_is_assignment_expr (context_p))
        {
          uint32_t flags = PARSER_PATTERN_NO_OPTS;

          if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_REST)
          {
            flags |= PARSER_PATTERN_HAS_REST_ELEMENT;
          }

          parser_parse_object_initializer (context_p, (parser_pattern_flags_t) flags);
          return parser_abort_parsing_after_assignment_expression (context_p);
        }

        scanner_release_next (context_p, sizeof (scanner_location_info_t));
      }

      parser_parse_object_literal (context_p);
      break;
    }
    case LEXER_LEFT_SQUARE:
    {
      if (context_p->next_scanner_info_p->source_p == context_p->source_p)
      {
        if (context_p->next_scanner_info_p->type == SCANNER_TYPE_INITIALIZER)
        {
          if (parser_is_assignment_expr (context_p))
          {
            parser_parse_array_initializer (context_p, PARSER_PATTERN_NO_OPTS);
            return parser_abort_parsing_after_assignment_expression (context_p);
          }

          scanner_release_next (context_p, sizeof (scanner_location_info_t));
        }
        else
        {
          JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS
                        && context_p->next_scanner_info_p->u8_arg == SCANNER_LITERAL_NO_DESTRUCTURING);

          scanner_release_next (context_p, sizeof (scanner_info_t));
        }
      }

      parser_parse_array_literal (context_p);
      break;
    }
    case LEXER_DIVIDE:
    case LEXER_ASSIGN_DIVIDE:
    {
      lexer_construct_regexp_object (context_p, false);
      uint16_t literal_index = (uint16_t) (context_p->literal_count - 1);

      if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
      {
        context_p->last_cbc_opcode = CBC_PUSH_TWO_LITERALS;
        context_p->last_cbc.value = literal_index;
      }
      else if (context_p->last_cbc_opcode == CBC_PUSH_TWO_LITERALS)
      {
        context_p->last_cbc_opcode = CBC_PUSH_THREE_LITERALS;
        context_p->last_cbc.third_literal_index = literal_index;
      }
      else
      {
        parser_emit_cbc_literal (context_p, CBC_PUSH_LITERAL, literal_index);
      }

      context_p->last_cbc.literal_type = LEXER_REGEXP_LITERAL;
      context_p->last_cbc.literal_keyword_type = LEXER_EOS;
      break;
    }
    case LEXER_KEYW_THIS:
    {
      if (context_p->status_flags & PARSER_ALLOW_SUPER_CALL)
      {
        parser_emit_cbc_ext (context_p, CBC_EXT_RESOLVE_LEXICAL_THIS);
      }
      else
      {
        parser_emit_cbc (context_p, CBC_PUSH_THIS);
      }

      break;
    }
    case LEXER_LIT_TRUE:
    {
      parser_emit_cbc (context_p, CBC_PUSH_TRUE);
      break;
    }
    case LEXER_LIT_FALSE:
    {
      parser_emit_cbc (context_p, CBC_PUSH_FALSE);
      break;
    }
    case LEXER_LIT_NULL:
    {
      parser_emit_cbc (context_p, CBC_PUSH_NULL);
      break;
    }
    case LEXER_KEYW_CLASS:
    {
      parser_parse_class (context_p, false);
      return false;
    }
    case LEXER_KEYW_SUPER:
    {
      if (context_p->status_flags & PARSER_ALLOW_SUPER)
      {
        if (lexer_check_next_characters (context_p, LIT_CHAR_DOT, LIT_CHAR_LEFT_SQUARE))
        {
          parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_SUPER);
          break;
        }

        if (lexer_check_next_character (context_p, LIT_CHAR_LEFT_PAREN)
            && (context_p->status_flags & PARSER_ALLOW_SUPER_CALL))
        {
          parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_SUPER_CONSTRUCTOR);
          break;
        }
      }

      parser_raise_error (context_p, PARSER_ERR_UNEXPECTED_SUPER_KEYWORD);
    }
    case LEXER_LEFT_PAREN:
    {
      JERRY_ASSERT (context_p->next_scanner_info_p->source_p == context_p->source_p
                    && context_p->next_scanner_info_p->type == SCANNER_TYPE_FUNCTION);

      parser_check_assignment_expr (context_p);

      uint32_t arrow_status_flags =
        (PARSER_IS_FUNCTION | PARSER_IS_ARROW_FUNCTION
         | (context_p->status_flags & (PARSER_INSIDE_CLASS_FIELD | PARSER_IS_CLASS_STATIC_BLOCK)));
      parser_parse_function_expression (context_p, arrow_status_flags);
      return parser_abort_parsing_after_assignment_expression (context_p);
    }
    case LEXER_KEYW_YIELD:
    {
      JERRY_ASSERT ((context_p->status_flags & PARSER_IS_GENERATOR_FUNCTION)
                    && !(context_p->status_flags & PARSER_DISALLOW_AWAIT_YIELD));

      if (context_p->token.lit_location.status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)
      {
        parser_raise_error (context_p, PARSER_ERR_INVALID_KEYWORD);
      }

      parser_check_assignment_expr (context_p);
      lexer_next_token (context_p);

      cbc_ext_opcode_t opcode =
        ((context_p->status_flags & PARSER_IS_ASYNC_FUNCTION) ? CBC_EXT_ASYNC_YIELD : CBC_EXT_YIELD);
      if (!lexer_check_yield_no_arg (context_p))
      {
        if (context_p->token.type == LEXER_MULTIPLY)
        {
          lexer_next_token (context_p);
          opcode = ((context_p->status_flags & PARSER_IS_ASYNC_FUNCTION) ? CBC_EXT_ASYNC_YIELD_ITERATOR
                                                                         : CBC_EXT_YIELD_ITERATOR);
        }

        parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);
      }
      else
      {
        parser_emit_cbc (context_p, CBC_PUSH_UNDEFINED);
      }

      parser_emit_cbc_ext (context_p, opcode);

      return (context_p->token.type != LEXER_RIGHT_PAREN && context_p->token.type != LEXER_COMMA);
    }
#if JERRY_MODULE_SYSTEM
    case LEXER_KEYW_IMPORT:
    {
      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_DOT)
      {
        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL
            || context_p->token.keyword_type != LEXER_KEYW_META
            || (context_p->token.lit_location.status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE))
        {
          parser_raise_error (context_p, PARSER_ERR_META_EXPECTED);
        }

        if (!(context_p->global_status_flags & ECMA_PARSE_MODULE))
        {
          parser_raise_error (context_p, PARSER_ERR_IMPORT_META_REQUIRE_MODULE);
        }

        JERRY_ASSERT (context_p->global_status_flags & ECMA_PARSE_INTERNAL_HAS_IMPORT_META);

        parser_emit_cbc_ext (context_p, CBC_EXT_MODULE_IMPORT_META);
        break;
      }

      if (context_p->token.type != LEXER_LEFT_PAREN)
      {
        parser_raise_error (context_p, PARSER_ERR_LEFT_PAREN_EXPECTED);
      }

      if (new_was_seen)
      {
        parser_raise_error (context_p, PARSER_ERR_IMPORT_AFTER_NEW);
      }

      lexer_next_token (context_p);
      parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

      if (context_p->token.type != LEXER_RIGHT_PAREN)
      {
        parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
      }

      parser_emit_cbc_ext (context_p, CBC_EXT_MODULE_IMPORT);
      break;
    }
#endif /* JERRY_MODULE_SYSTEM */
    default:
    {
      bool is_left_hand_side = (*grouping_level_p == PARSE_EXPR_LEFT_HAND_SIDE);
      parser_raise_error (context_p,
                          (is_left_hand_side ? PARSER_ERR_LEFT_HAND_SIDE_EXP_EXPECTED : PARSER_ERR_UNEXPECTED_END));
      break;
    }
  }
  lexer_next_token (context_p);
  return false;
} /* parser_parse_unary_expression */

/**
 * Parse the postfix part of unary operators, and
 * generate byte code for the whole expression.
 */
static void
parser_process_unary_expression (parser_context_t *context_p, /**< context */
                                 size_t grouping_level) /**< grouping level */
{
  /* Parse postfix part of a primary expression. */
  while (true)
  {
    /* Since break would only break the switch, we use
     * continue to continue this loop. Without continue,
     * the code abandons the loop. */
    switch (context_p->token.type)
    {
      case LEXER_DOT:
      {
        parser_push_result (context_p);

        if (lexer_check_next_character (context_p, LIT_CHAR_HASHMARK))
        {
          lexer_next_token (context_p);

          if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER))
          {
            parser_raise_error (context_p, PARSER_ERR_UNEXPECTED_PRIVATE_FIELD);
          }

          if (!lexer_scan_private_identifier (context_p))
          {
            parser_raise_error (context_p, PARSER_ERR_IDENTIFIER_EXPECTED);
          }

          parser_resolve_private_identifier (context_p);

          parser_emit_cbc_ext_literal (context_p, CBC_EXT_PUSH_PRIVATE_PROP_LITERAL, context_p->lit_object.index);
          lexer_next_token (context_p);
          continue;
        }

        lexer_expect_identifier (context_p, LEXER_STRING_LITERAL);

        JERRY_ASSERT (context_p->token.type == LEXER_LITERAL
                      && context_p->lit_object.literal_p->type == LEXER_STRING_LITERAL);
        context_p->token.lit_location.type = LEXER_STRING_LITERAL;

        if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
        {
          JERRY_ASSERT (CBC_ARGS_EQ (CBC_PUSH_PROP_LITERAL_LITERAL, CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2));
          context_p->last_cbc_opcode = CBC_PUSH_PROP_LITERAL_LITERAL;
          context_p->last_cbc.value = context_p->lit_object.index;
        }
        else if (context_p->last_cbc_opcode == CBC_PUSH_THIS)
        {
          context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
          parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_PROP_THIS_LITERAL);
        }
        else if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER))
        {
          context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER_PROP_LITERAL);
          context_p->last_cbc.literal_index = context_p->lit_object.index;
        }
        else
        {
          parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_PROP_LITERAL);
        }
        lexer_next_token (context_p);
        continue;
      }

      case LEXER_LEFT_SQUARE:
      {
        parser_push_result (context_p);

        uint16_t last_cbc_opcode = context_p->last_cbc_opcode;

        if (last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER))
        {
          context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
        }

        lexer_next_token (context_p);
        parser_parse_expression (context_p, PARSE_EXPR);
        if (context_p->token.type != LEXER_RIGHT_SQUARE)
        {
          parser_raise_error (context_p, PARSER_ERR_RIGHT_SQUARE_EXPECTED);
        }
        lexer_next_token (context_p);

        if (last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER))
        {
          parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_SUPER_PROP);
          continue;
        }

        if (PARSER_IS_MUTABLE_PUSH_LITERAL (context_p->last_cbc_opcode))
        {
          context_p->last_cbc_opcode = PARSER_PUSH_LITERAL_TO_PUSH_PROP_LITERAL (context_p->last_cbc_opcode);
        }
        else
        {
          parser_emit_cbc (context_p, CBC_PUSH_PROP);
        }
        continue;
      }
      case LEXER_TEMPLATE_LITERAL:
      case LEXER_LEFT_PAREN:
      {
        size_t call_arguments = 0;
        uint16_t opcode = CBC_CALL;
        bool is_eval = false;

        parser_push_result (context_p);

        if (context_p->stack_top_uint8 == LEXER_KEYW_NEW)
        {
          if (context_p->token.type == LEXER_LEFT_PAREN)
          {
            parser_stack_pop_uint8 (context_p);
            opcode = CBC_NEW;
          }
        }
        else
        {
          if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL
              && context_p->last_cbc.literal_keyword_type == LEXER_KEYW_EVAL
              && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL)
          {
            is_eval = true;
          }

          if (PARSER_IS_PUSH_PROP (context_p->last_cbc_opcode))
          {
            opcode = CBC_CALL_PROP;
            context_p->last_cbc_opcode = PARSER_PUSH_PROP_TO_PUSH_PROP_REFERENCE (context_p->last_cbc_opcode);
          }
          else if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER_CONSTRUCTOR))
          {
            opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_SUPER_CALL);
          }
          else if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER_PROP_LITERAL))
          {
            context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_SUPER_PROP_LITERAL_REFERENCE);
            opcode = CBC_CALL_PROP;
          }
          else if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER_PROP))
          {
            context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_SUPER_PROP_REFERENCE);
            opcode = CBC_CALL_PROP;
          }
          else if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_PRIVATE_PROP_LITERAL))
          {
            context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_PRIVATE_PROP_LITERAL_REFERENCE);
            opcode = CBC_CALL_PROP;
          }
          else if (JERRY_UNLIKELY (context_p->status_flags & PARSER_INSIDE_WITH)
                   && PARSER_IS_PUSH_LITERALS_WITH_THIS (context_p->last_cbc_opcode)
                   && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL)
          {
            opcode = CBC_CALL_PROP;
            parser_emit_ident_reference (context_p, CBC_PUSH_IDENT_REFERENCE);
            parser_emit_cbc_ext (context_p, CBC_EXT_RESOLVE_BASE);
          }
        }

        bool has_spread_element = false;

        if (context_p->token.type == LEXER_TEMPLATE_LITERAL)
        {
          call_arguments = parser_parse_tagged_template_literal (context_p);
        }
        else
        {
          lexer_next_token (context_p);

          while (context_p->token.type != LEXER_RIGHT_PAREN)
          {
            if (++call_arguments > CBC_MAXIMUM_BYTE_VALUE)
            {
              parser_raise_error (context_p, PARSER_ERR_ARGUMENT_LIMIT_REACHED);
            }

            if (context_p->token.type == LEXER_THREE_DOTS)
            {
              has_spread_element = true;
              call_arguments++;
              parser_emit_cbc_ext (context_p, CBC_EXT_PUSH_SPREAD_ELEMENT);
              lexer_next_token (context_p);
            }

            parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

            if (context_p->token.type == LEXER_COMMA)
            {
              lexer_next_token (context_p);
              continue;
            }

            if (context_p->token.type != LEXER_RIGHT_PAREN)
            {
              parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
            }

            break;
          }
        }

        lexer_next_token (context_p);

        if (is_eval)
        {
          context_p->status_flags |= PARSER_LEXICAL_ENV_NEEDED;

          uint16_t eval_flags = PARSER_SAVE_STATUS_FLAGS (context_p->status_flags);
          const uint32_t required_flags = PARSER_IS_FUNCTION | PARSER_LEXICAL_BLOCK_NEEDED;

          if (context_p->status_flags & PARSER_FUNCTION_IS_PARSING_ARGS)
          {
            context_p->status_flags |= PARSER_LEXICAL_BLOCK_NEEDED;
          }
          else if (((context_p->status_flags & (required_flags | PARSER_IS_STRICT)) == required_flags)
                   || ((context_p->global_status_flags & ECMA_PARSE_FUNCTION_CONTEXT)
                       && !(context_p->status_flags & PARSER_IS_FUNCTION)))
          {
            eval_flags |= PARSER_GET_EVAL_FLAG (ECMA_PARSE_FUNCTION_CONTEXT);
          }

          if (eval_flags != 0)
          {
            parser_emit_cbc_ext_call (context_p, CBC_EXT_LOCAL_EVAL, eval_flags);
          }
          else
          {
            parser_emit_cbc (context_p, CBC_EVAL);
          }
        }

        if (has_spread_element)
        {
          uint16_t spread_opcode;

          if (opcode == CBC_CALL)
          {
            spread_opcode = CBC_EXT_SPREAD_CALL;
          }
          else if (opcode == CBC_CALL_PROP)
          {
            spread_opcode = CBC_EXT_SPREAD_CALL_PROP;
          }
          else if (opcode == CBC_NEW)
          {
            spread_opcode = CBC_EXT_SPREAD_NEW;
          }
          else
          {
            /* opcode is unchanged */
            JERRY_ASSERT (opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_SUPER_CALL));
            spread_opcode = CBC_EXT_SPREAD_SUPER_CALL;
          }

          parser_emit_cbc_ext_call (context_p, spread_opcode, call_arguments);
          continue;
        }

        if (call_arguments <= 1)
        {
          if (opcode == CBC_CALL)
          {
            parser_emit_cbc (context_p, (uint16_t) (CBC_CALL0 + (call_arguments * 6)));
            continue;
          }
          if (opcode == CBC_CALL_PROP)
          {
            parser_emit_cbc (context_p, (uint16_t) (CBC_CALL0_PROP + (call_arguments * 6)));
            continue;
          }
          if (opcode == CBC_NEW)
          {
            parser_emit_cbc (context_p, (uint16_t) (CBC_NEW0 + call_arguments));
            continue;
          }
        }
        else if (call_arguments == 2)
        {
          if (opcode == CBC_CALL)
          {
            parser_emit_cbc (context_p, CBC_CALL2);
            continue;
          }
          if (opcode == CBC_CALL_PROP)
          {
            parser_flush_cbc (context_p);
            /* Manually adjusting stack usage. */
            JERRY_ASSERT (context_p->stack_depth > 0);
            context_p->stack_depth--;
            parser_emit_cbc (context_p, CBC_CALL2_PROP);
            continue;
          }
        }

        parser_emit_cbc_call (context_p, opcode, call_arguments);
        continue;
      }
      default:
      {
        if (context_p->stack_top_uint8 == LEXER_KEYW_NEW)
        {
          parser_push_result (context_p);
          parser_emit_cbc (context_p, CBC_NEW0);
          parser_stack_pop_uint8 (context_p);
          continue;
        }

        if (!(context_p->token.flags & LEXER_WAS_NEWLINE)
            && (context_p->token.type == LEXER_INCREASE || context_p->token.type == LEXER_DECREASE)
            && grouping_level != PARSE_EXPR_LEFT_HAND_SIDE)
        {
          cbc_opcode_t opcode = (context_p->token.type == LEXER_INCREASE) ? CBC_POST_INCR : CBC_POST_DECR;
          parser_push_result (context_p);
          parser_emit_unary_lvalue_opcode (context_p, opcode);
          lexer_next_token (context_p);
        }
        break;
      }
    }
    break;
  }

  uint8_t last_unary_token = LEXER_INCREASE;

  /* Generate byte code for the unary operators. */
  while (true)
  {
    uint8_t token = context_p->stack_top_uint8;
    if (!LEXER_IS_UNARY_OP_TOKEN (token))
    {
      if (context_p->token.type == LEXER_EXPONENTIATION && last_unary_token != LEXER_INCREASE
          && last_unary_token != LEXER_DECREASE)
      {
        parser_raise_error (context_p, PARSER_ERR_INVALID_EXPONENTIATION);
      }

      break;
    }

    last_unary_token = token;
    parser_push_result (context_p);
    parser_stack_pop_uint8 (context_p);

    if (LEXER_IS_UNARY_LVALUE_OP_TOKEN (token))
    {
      if (token == LEXER_KEYW_DELETE)
      {
        token = CBC_DELETE_PUSH_RESULT;
      }
      else
      {
        token = (uint8_t) (LEXER_UNARY_LVALUE_OP_TOKEN_TO_OPCODE (token));
      }
      parser_emit_unary_lvalue_opcode (context_p, (cbc_opcode_t) token);
    }
    else if (JERRY_UNLIKELY (token == LEXER_KEYW_AWAIT))
    {
      cbc_ext_opcode_t opcode =
        ((context_p->status_flags & PARSER_IS_GENERATOR_FUNCTION) ? CBC_EXT_GENERATOR_AWAIT : CBC_EXT_AWAIT);
      parser_emit_cbc_ext (context_p, opcode);
    }
    else
    {
      token = (uint8_t) (LEXER_UNARY_OP_TOKEN_TO_OPCODE (token));

      if (token == CBC_TYPEOF)
      {
        if (PARSER_IS_PUSH_LITERALS_WITH_THIS (context_p->last_cbc_opcode)
            && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL)
        {
          parser_emit_ident_reference (context_p, CBC_TYPEOF_IDENT);
        }
        else
        {
          parser_emit_cbc (context_p, token);
        }
      }
      else
      {
        if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
        {
          /* It is not worth to combine with push multiple literals
           * since the byte code size will not decrease. */
          JERRY_ASSERT (CBC_SAME_ARGS (context_p->last_cbc_opcode, token + 1));
          context_p->last_cbc_opcode = (uint16_t) (token + 1);
        }
        else
        {
          parser_emit_cbc (context_p, token);
        }
      }
    }
  }
} /* parser_process_unary_expression */

/**
 * Append a binary '=' token.
 *
 * @return - pushed assignment opcode onto the parser stack
 */
static void
parser_append_binary_single_assignment_token (parser_context_t *context_p, /**< context */
                                              uint32_t pattern_flags) /**< pattern flags */
{
  JERRY_UNUSED (pattern_flags);

  /* Unlike other tokens, the whole byte code is saved for binary
   * assignment, since it has multiple forms depending on the
   * previous instruction. */

  uint8_t assign_opcode = CBC_ASSIGN;

  if (PARSER_IS_PUSH_LITERALS_WITH_THIS (context_p->last_cbc_opcode)
      && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL)
  {
    parser_check_invalid_assign (context_p);

    uint16_t literal_index;

    switch (context_p->last_cbc_opcode)
    {
      case CBC_PUSH_LITERAL:
      {
        literal_index = context_p->last_cbc.literal_index;
        context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
        break;
      }
      case CBC_PUSH_TWO_LITERALS:
      {
        literal_index = context_p->last_cbc.value;
        context_p->last_cbc_opcode = CBC_PUSH_LITERAL;
        break;
      }
      case CBC_PUSH_THIS_LITERAL:
      {
        literal_index = context_p->last_cbc.literal_index;
        context_p->last_cbc_opcode = CBC_PUSH_THIS;
        parser_flush_cbc (context_p);
        break;
      }
      default:
      {
        JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_THREE_LITERALS);
        literal_index = context_p->last_cbc.third_literal_index;
        context_p->last_cbc_opcode = CBC_PUSH_TWO_LITERALS;
        break;
      }
    }

    assign_opcode = CBC_ASSIGN_SET_IDENT;

    if (!(pattern_flags & (PARSER_PATTERN_LET | PARSER_PATTERN_CONST | PARSER_PATTERN_LOCAL)))
    {
      if (scanner_literal_is_const_reg (context_p, literal_index))
      {
        parser_stack_push_uint8 (context_p, LEXER_ASSIGN_CONST);
      }
    }
    else if (literal_index < PARSER_REGISTER_START)
    {
      assign_opcode = CBC_INIT_LET;

      if (scanner_literal_is_created (context_p, literal_index))
      {
        assign_opcode = CBC_ASSIGN_LET_CONST;
      }
      else if (pattern_flags & PARSER_PATTERN_CONST)
      {
        assign_opcode = CBC_INIT_CONST;
      }
      else if (pattern_flags & PARSER_PATTERN_LOCAL)
      {
        assign_opcode = CBC_INIT_ARG_OR_CATCH;
      }
    }

    parser_stack_push_uint16 (context_p, literal_index);
    JERRY_ASSERT (CBC_SAME_ARGS (CBC_PUSH_LITERAL, assign_opcode));
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_PROP)
  {
    JERRY_ASSERT (CBC_SAME_ARGS (CBC_PUSH_PROP, CBC_ASSIGN));
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_PROP_LITERAL)
  {
    if (context_p->last_cbc.literal_type != LEXER_IDENT_LITERAL)
    {
      JERRY_ASSERT (CBC_SAME_ARGS (CBC_PUSH_PROP_LITERAL, CBC_ASSIGN_PROP_LITERAL));
      parser_stack_push_uint16 (context_p, context_p->last_cbc.literal_index);
      assign_opcode = CBC_ASSIGN_PROP_LITERAL;
      context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
    }
    else
    {
      context_p->last_cbc_opcode = CBC_PUSH_LITERAL;
    }
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_PROP_LITERAL_LITERAL)
  {
    JERRY_ASSERT (CBC_SAME_ARGS (CBC_PUSH_PROP_LITERAL_LITERAL, CBC_PUSH_TWO_LITERALS));
    context_p->last_cbc_opcode = CBC_PUSH_TWO_LITERALS;
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_PROP_THIS_LITERAL)
  {
    if (context_p->last_cbc.literal_type != LEXER_IDENT_LITERAL)
    {
      JERRY_ASSERT (CBC_SAME_ARGS (CBC_PUSH_PROP_THIS_LITERAL, CBC_ASSIGN_PROP_THIS_LITERAL));
      parser_stack_push_uint16 (context_p, context_p->last_cbc.literal_index);
      assign_opcode = CBC_ASSIGN_PROP_THIS_LITERAL;
      context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
    }
    else
    {
      context_p->last_cbc_opcode = CBC_PUSH_THIS_LITERAL;
    }
  }
  else if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER_PROP_LITERAL))
  {
    context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_SUPER_PROP_LITERAL_ASSIGNMENT_REFERENCE);
    parser_stack_push_uint8 (context_p, CBC_EXT_ASSIGN_SUPER);
    assign_opcode = CBC_EXT_OPCODE;
  }
  else if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_SUPER_PROP))
  {
    context_p->last_cbc_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_SUPER_PROP_ASSIGNMENT_REFERENCE);
    parser_stack_push_uint8 (context_p, CBC_EXT_ASSIGN_SUPER);
    assign_opcode = CBC_EXT_OPCODE;
  }
  else if (context_p->last_cbc_opcode == PARSER_TO_EXT_OPCODE (CBC_EXT_PUSH_PRIVATE_PROP_LITERAL))
  {
    context_p->last_cbc_opcode = CBC_PUSH_LITERAL;
    parser_stack_push_uint8 (context_p, CBC_EXT_ASSIGN_PRIVATE);
    assign_opcode = CBC_EXT_OPCODE;
  }
  else
  {
    /* Invalid LeftHandSide expression. */ // 3820, 3815
    parser_check_invalid_new_target (context_p, CBC_ASSIGN);
    parser_raise_error (context_p, PARSER_ERR_INVALID_LHS_ASSIGNMENT);
  }

  parser_stack_push_uint8 (context_p, assign_opcode);
  parser_stack_push_uint8 (context_p, LEXER_ASSIGN);
} /* parser_append_binary_single_assignment_token */

/**
 * Check for invalid chain of logical operators
 */
static void
parser_check_invalid_logical_op (parser_context_t *context_p, /**< context */
                                 uint8_t invalid_token1, /**< token id of first invalid token */
                                 uint8_t invalid_token2) /**< token id of second invalid token */
{
  parser_stack_iterator_t iterator;
  parser_stack_iterator_init (context_p, &iterator);

  while (true)
  {
    uint8_t token = parser_stack_iterator_read_uint8 (&iterator);

    if (!LEXER_IS_BINARY_NON_LVALUE_OP_TOKEN (token))
    {
      return;
    }

    if (token == invalid_token1 || token == invalid_token2)
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_NULLISH_COALESCING);
    }

    /* If a logical operator is found, and there is no SyntaxError, the scan can be terminated
     * since there was no SyntaxError when the logical operator was pushed onto the stack. */
    if (token == LEXER_LOGICAL_OR || token == LEXER_LOGICAL_AND || token == LEXER_NULLISH_COALESCING)
    {
      return;
    }

    parser_stack_iterator_skip (&iterator, sizeof (uint8_t));
  }
} /* parser_check_invalid_logical_op */

/**
 * Append a binary lvalue token.
 */
static void
parser_append_binary_lvalue_token (parser_context_t *context_p, /**< context */
                                   bool is_logical_assignment) /**< true - if form logical assignment reference
                                                                *   false - otherwise */
{
  if (PARSER_IS_PUSH_LITERALS_WITH_THIS (context_p->last_cbc_opcode)
      && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL)
  {
    parser_check_invalid_assign (context_p);

    parser_emit_ident_reference (context_p, CBC_PUSH_IDENT_REFERENCE);

    if (!is_logical_assignment && scanner_literal_is_const_reg (context_p, context_p->last_cbc.literal_index))
    {
      parser_stack_push_uint8 (context_p, LEXER_ASSIGN_CONST);
    }
  }
  else if (PARSER_IS_PUSH_PROP (context_p->last_cbc_opcode))
  {
    context_p->last_cbc_opcode = PARSER_PUSH_PROP_TO_PUSH_PROP_REFERENCE (context_p->last_cbc_opcode);
  }
  else
  {
    /* Invalid LeftHandSide expression. */
    parser_check_invalid_new_target (context_p, CBC_ASSIGN);
    parser_raise_error (context_p, PARSER_ERR_INVALID_LHS_ASSIGNMENT);
    parser_emit_cbc (context_p, CBC_PUSH_PROP_REFERENCE);
  }

  if (!is_logical_assignment)
  {
    parser_stack_push_uint8 (context_p, (uint8_t) context_p->token.type);
  }
} /* parser_append_binary_lvalue_token */

/**
 * Append a logical token.
 */
static void
parser_append_logical_token (parser_context_t *context_p, /**< context */
                             uint16_t opcode) /**< opcode */
{
  if (opcode != PARSER_TO_EXT_OPCODE (CBC_EXT_BRANCH_IF_NULLISH))
  {
    parser_check_invalid_logical_op (context_p, LEXER_NULLISH_COALESCING, LEXER_NULLISH_COALESCING);
  }

  parser_branch_t branch;
  parser_emit_cbc_forward_branch (context_p, opcode, &branch);
  parser_stack_push (context_p, &branch, sizeof (parser_branch_t));
  parser_stack_push_uint8 (context_p, (uint8_t) context_p->token.type);
} /* parser_append_logical_token */

/**
 * Append a logical token.
 */
static void
parser_append_logical_assignment_token (parser_context_t *context_p, /**< context */
                                        uint16_t opcode) /**< opcode */
{
  uint16_t last_cbc_opcode = context_p->last_cbc_opcode;
  parser_append_binary_single_assignment_token (context_p, 0);
  parser_stack_change_last_uint8 (context_p, LEXER_ASSIGN_REFERENCE);
  context_p->last_cbc_opcode = last_cbc_opcode;

  parser_append_binary_lvalue_token (context_p, true);
  parser_append_logical_token (context_p, opcode);
} /* parser_append_logical_assignment_token */

/**
 * Append a binary token.
 */
static void
parser_append_binary_token (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (LEXER_IS_BINARY_OP_TOKEN (context_p->token.type));
  parser_push_result (context_p);

  switch (context_p->token.type)
  {
    case LEXER_ASSIGN:
    {
      parser_append_binary_single_assignment_token (context_p, 0);
      break;
    }
    /* Binary lvalue-opcodes */
    case LEXER_ASSIGN_ADD:
    case LEXER_ASSIGN_SUBTRACT:
    case LEXER_ASSIGN_MULTIPLY:
    case LEXER_ASSIGN_DIVIDE:
    case LEXER_ASSIGN_MODULO:
    case LEXER_ASSIGN_EXPONENTIATION:
    case LEXER_ASSIGN_LEFT_SHIFT:
    case LEXER_ASSIGN_RIGHT_SHIFT:
    case LEXER_ASSIGN_UNS_RIGHT_SHIFT:
    case LEXER_ASSIGN_BIT_AND:
    case LEXER_ASSIGN_BIT_OR:
    case LEXER_ASSIGN_BIT_XOR:
    {
      parser_append_binary_lvalue_token (context_p, false);
      break;
    }
    case LEXER_ASSIGN_NULLISH_COALESCING:
    {
      parser_append_logical_assignment_token (context_p, PARSER_TO_EXT_OPCODE (CBC_EXT_BRANCH_IF_NULLISH));
      break;
    }
    case LEXER_ASSIGN_LOGICAL_OR:
    {
      parser_append_logical_assignment_token (context_p, CBC_BRANCH_IF_LOGICAL_TRUE);
      break;
    }
    case LEXER_ASSIGN_LOGICAL_AND:
    {
      parser_append_logical_assignment_token (context_p, CBC_BRANCH_IF_LOGICAL_FALSE);
      break;
    }
    case LEXER_NULLISH_COALESCING:
    {
      parser_append_logical_token (context_p, PARSER_TO_EXT_OPCODE (CBC_EXT_BRANCH_IF_NULLISH));
      break;
    }
    case LEXER_LOGICAL_OR:
    {
      parser_append_logical_token (context_p, CBC_BRANCH_IF_LOGICAL_TRUE);
      break;
    }
    case LEXER_LOGICAL_AND:
    {
      parser_append_logical_token (context_p, CBC_BRANCH_IF_LOGICAL_FALSE);
      break;
    }
    default:
    {
      parser_stack_push_uint8 (context_p, (uint8_t) context_p->token.type);
      break;
    }
  }
} /* parser_append_binary_token */

/**
 * Emit opcode for binary assignment token.
 */
static void
parser_process_binary_assignment_token (parser_context_t *context_p, /**< context */
                                        uint8_t token) /**< token */
{
  uint16_t index = PARSER_INVALID_LITERAL_INDEX;
  uint16_t opcode = context_p->stack_top_uint8;

  if (JERRY_UNLIKELY (opcode == CBC_EXT_OPCODE))
  {
    parser_stack_pop_uint8 (context_p);
    JERRY_ASSERT (context_p->stack_top_uint8 == CBC_EXT_ASSIGN_SUPER
                  || context_p->stack_top_uint8 == CBC_EXT_ASSIGN_PRIVATE);
    opcode = PARSER_TO_EXT_OPCODE (context_p->stack_top_uint8);
    parser_stack_pop_uint8 (context_p);
  }
  else
  {
    parser_stack_pop_uint8 (context_p);

    if (cbc_flags[opcode] & CBC_HAS_LITERAL_ARG)
    {
      JERRY_ASSERT (opcode == CBC_ASSIGN_SET_IDENT || opcode == CBC_ASSIGN_PROP_LITERAL
                    || opcode == CBC_ASSIGN_PROP_THIS_LITERAL || opcode == CBC_ASSIGN_LET_CONST
                    || opcode == CBC_INIT_ARG_OR_CATCH || opcode == CBC_INIT_LET || opcode == CBC_INIT_CONST);

      index = parser_stack_pop_uint16 (context_p);
    }
  }

  bool group_expr_assignment = false;

  if (JERRY_UNLIKELY (context_p->stack_top_uint8 == LEXER_ASSIGN_GROUP_EXPR))
  {
    group_expr_assignment = true;
    parser_stack_pop_uint8 (context_p);
  }

  if (JERRY_UNLIKELY (context_p->stack_top_uint8 == LEXER_ASSIGN_CONST))
  {
    parser_stack_pop_uint8 (context_p);
    parser_emit_cbc_ext (context_p, CBC_EXT_THROW_ASSIGN_CONST_ERROR);
  }

  if (index == PARSER_INVALID_LITERAL_INDEX)
  {
    if (JERRY_UNLIKELY (token == LEXER_ASSIGN_REFERENCE))
    {
      opcode = CBC_ASSIGN_PUSH_RESULT;
    }

    parser_emit_cbc (context_p, opcode);
    return;
  }

  if (!group_expr_assignment)
  {
    uint16_t function_literal_index = parser_check_anonymous_function_declaration (context_p);

    if (function_literal_index == PARSER_ANONYMOUS_CLASS)
    {
      uint16_t name_index = scanner_save_literal (context_p, index);
      parser_emit_cbc_ext_literal (context_p, CBC_EXT_SET_CLASS_NAME, name_index);
    }
    else if (function_literal_index < PARSER_NAMED_FUNCTION)
    {
      parser_set_function_name (context_p, function_literal_index, (uint16_t) index, 0);
    }
  }

  if (JERRY_UNLIKELY (token == LEXER_ASSIGN_REFERENCE))
  {
    parser_emit_cbc (context_p, CBC_ASSIGN_PUSH_RESULT);
    return;
  }

  if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL && opcode == CBC_ASSIGN_SET_IDENT)
  {
    JERRY_ASSERT (CBC_ARGS_EQ (CBC_ASSIGN_LITERAL_SET_IDENT, CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2));

    context_p->last_cbc.value = index;
    context_p->last_cbc_opcode = CBC_ASSIGN_LITERAL_SET_IDENT;
    return;
  }

  parser_emit_cbc_literal (context_p, (uint16_t) opcode, index);

  if (opcode == CBC_ASSIGN_PROP_THIS_LITERAL && (context_p->stack_depth >= context_p->stack_limit))
  {
    /* Stack limit is increased for VM_OC_ASSIGN_PROP_THIS. Needed by vm.c. */
    JERRY_ASSERT (context_p->stack_depth == context_p->stack_limit);

    context_p->stack_limit++;

    if (context_p->stack_limit > PARSER_MAXIMUM_STACK_LIMIT)
    {
      parser_raise_error (context_p, PARSER_ERR_STACK_LIMIT_REACHED);
    }
  }
} /* parser_process_binary_opcodes */

/**
 * Emit opcode for logical tokens.
 */
static void
parser_process_logical_token (parser_context_t *context_p) /**< context */
{
  parser_branch_t branch;
  parser_stack_pop (context_p, &branch, sizeof (parser_branch_t));
  parser_set_branch_to_current_position (context_p, &branch);
} /* parser_process_logical_token */

/**
 * Emit opcode for logical assignment tokens.
 */
static void
parser_process_logical_assignment_token (parser_context_t *context_p) /**< context */
{
  parser_branch_t condition_branch;
  parser_stack_pop (context_p, &condition_branch, sizeof (parser_branch_t));

  uint8_t token = context_p->stack_top_uint8;
  JERRY_ASSERT (token == LEXER_ASSIGN_REFERENCE);
  parser_stack_pop_uint8 (context_p);
  parser_process_binary_assignment_token (context_p, token);

  parser_branch_t prop_reference_branch;
  parser_emit_cbc_forward_branch (context_p, CBC_JUMP_FORWARD, &prop_reference_branch);

  parser_set_branch_to_current_position (context_p, &condition_branch);
  parser_emit_cbc_ext (context_p, CBC_EXT_POP_REFERENCE);

  JERRY_ASSERT (context_p->stack_limit - context_p->stack_depth >= 2);
  PARSER_PLUS_EQUAL_U16 (context_p->stack_depth, 2);

  parser_set_branch_to_current_position (context_p, &prop_reference_branch);
} /* parser_process_logical_assignment_token */

/**
 * Emit opcode for binary tokens.
 */
static void
parser_process_binary_token (parser_context_t *context_p, /**< context */
                             uint8_t token) /**< token */
{
  uint16_t opcode = LEXER_BINARY_OP_TOKEN_TO_OPCODE (token);

  if (PARSER_IS_PUSH_NUMBER (context_p->last_cbc_opcode))
  {
    lexer_convert_push_number_to_push_literal (context_p);
  }

  if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
  {
    JERRY_ASSERT (CBC_SAME_ARGS (context_p->last_cbc_opcode, opcode + CBC_BINARY_WITH_LITERAL));
    context_p->last_cbc_opcode = (uint16_t) (opcode + CBC_BINARY_WITH_LITERAL);
    return;
  }

  if (context_p->last_cbc_opcode == CBC_PUSH_TWO_LITERALS)
  {
    JERRY_ASSERT (CBC_ARGS_EQ (opcode + CBC_BINARY_WITH_TWO_LITERALS, CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2));
    context_p->last_cbc_opcode = (uint16_t) (opcode + CBC_BINARY_WITH_TWO_LITERALS);
    return;
  }

  parser_emit_cbc (context_p, opcode);
} /* parser_process_binary_token */

/**
 * Emit opcode for binary lvalue tokens.
 */
static void
parser_process_binary_lvalue_token (parser_context_t *context_p, /**< context */
                                    uint8_t token) /**< token */
{
  parser_stack_push_uint8 (context_p, CBC_ASSIGN);
  parser_stack_push_uint8 (context_p, LEXER_ASSIGN);
  parser_stack_push_uint8 (context_p, lexer_convert_binary_lvalue_token_to_binary (token));
} /* parser_process_binary_lvalue_token */

/**
 * Emit opcode for binary computations.
 */
static void
parser_process_binary_opcodes (parser_context_t *context_p, /**< context */
                               uint8_t min_prec_threshhold) /**< minimal precedence of tokens */
{
  while (true)
  {
    uint8_t token = context_p->stack_top_uint8;

    /* For left-to-right operators (all binary operators except assignment
     * and logical operators), the byte code is flushed if the precedence
     * of the next operator is less or equal than the current operator. For
     * assignment and logical operators, we add 1 to the min precedence to
     * force right-to-left evaluation order. */

    if (!LEXER_IS_BINARY_OP_TOKEN (token)
        || parser_binary_precedence_table[token - LEXER_FIRST_BINARY_OP] < min_prec_threshhold)
    {
      return;
    }

    parser_push_result (context_p);
    parser_stack_pop_uint8 (context_p);

    switch (token)
    {
      case LEXER_ASSIGN:
      {
        parser_process_binary_assignment_token (context_p, token);
        continue;
      }
        /* Binary lvalue-opcodes */
      case LEXER_ASSIGN_ADD:
      case LEXER_ASSIGN_SUBTRACT:
      case LEXER_ASSIGN_MULTIPLY:
      case LEXER_ASSIGN_DIVIDE:
      case LEXER_ASSIGN_MODULO:
      case LEXER_ASSIGN_EXPONENTIATION:
      case LEXER_ASSIGN_LEFT_SHIFT:
      case LEXER_ASSIGN_RIGHT_SHIFT:
      case LEXER_ASSIGN_UNS_RIGHT_SHIFT:
      case LEXER_ASSIGN_BIT_AND:
      case LEXER_ASSIGN_BIT_OR:
      case LEXER_ASSIGN_BIT_XOR:
      {
        parser_process_binary_lvalue_token (context_p, token);
        continue;
      }
      case LEXER_ASSIGN_NULLISH_COALESCING:
      case LEXER_ASSIGN_LOGICAL_OR:
      case LEXER_ASSIGN_LOGICAL_AND:
      {
        parser_process_logical_assignment_token (context_p);
        continue;
      }
      case LEXER_NULLISH_COALESCING:
      case LEXER_LOGICAL_OR:
      case LEXER_LOGICAL_AND:
      {
        parser_process_logical_token (context_p);
        continue;
      }
      case LEXER_KEYW_IN:
      {
        if (context_p->stack_top_uint8 == LEXER_PRIVATE_PRIMARY_EXPR)
        {
          parser_stack_pop_uint8 (context_p);
          uint16_t lit_id = parser_stack_pop_uint16 (context_p);
          parser_emit_cbc_ext_literal (context_p, CBC_EXT_PUSH_PRIVATE_PROP_LITERAL_IN, lit_id);
          continue;
        }

        /* FALLTHRU */
      }
      default:
      {
        parser_process_binary_token (context_p, token);
        continue;
      }
    }
  }
} /* parser_process_binary_opcodes */

/**
 * End position marker of a pattern.
 */
typedef struct
{
  scanner_location_t location; /**< end position of the pattern */
  lexer_token_t token; /**< token at the end position */
} parser_pattern_end_marker_t;

/**
 * Literal index should not be emitted while processing rhs target value
 */
#define PARSER_PATTERN_RHS_NO_LIT PARSER_INVALID_LITERAL_INDEX

/**
 * Process the target of an initializer pattern.
 */
static parser_pattern_end_marker_t
parser_pattern_get_target (parser_context_t *context_p, /**< context */
                           parser_pattern_flags_t flags) /**< flags */
{
  parser_pattern_end_marker_t end_marker;
  end_marker.token.type = LEXER_INVALID_PATTERN;
  parser_branch_t skip_init;

  if (flags & PARSER_PATTERN_TARGET_DEFAULT)
  {
    JERRY_ASSERT (flags & PARSER_PATTERN_TARGET_ON_STACK);

    parser_emit_cbc_ext_forward_branch (context_p, CBC_EXT_DEFAULT_INITIALIZER, &skip_init);
  }

  if ((flags & (PARSER_PATTERN_TARGET_ON_STACK | PARSER_PATTERN_TARGET_DEFAULT)) != PARSER_PATTERN_TARGET_ON_STACK)
  {
    scanner_location_t start_location;

    if (context_p->next_scanner_info_p->source_p != context_p->source_p
        || context_p->next_scanner_info_p->type == SCANNER_TYPE_ERR_REDECLARED || (flags & PARSER_PATTERN_REST_ELEMENT))
    {
      /* Found invalid pattern, push null value to fake the rhs target. */
      parser_emit_cbc (context_p, CBC_PUSH_NULL);
    }
    else
    {
      JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_INITIALIZER);
      scanner_get_location (&start_location, context_p);

      scanner_set_location (context_p, &((scanner_location_info_t *) context_p->next_scanner_info_p)->location);
      scanner_release_next (context_p, sizeof (scanner_location_info_t));
      scanner_seek (context_p);
      lexer_next_token (context_p);

      parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);
      scanner_get_location (&(end_marker.location), context_p);
      end_marker.token = context_p->token;

      scanner_set_location (context_p, &start_location);
      scanner_seek (context_p);
      parser_flush_cbc (context_p);
    }
  }

  if (flags & PARSER_PATTERN_TARGET_DEFAULT)
  {
    parser_set_branch_to_current_position (context_p, &skip_init);
  }

  return end_marker;
} /* parser_pattern_get_target */

/**
 * Finalize an assignment/binding pattern.
 */
static void
parser_pattern_finalize (parser_context_t *context_p, /**< context */
                         parser_pattern_flags_t flags, /**< flags */
                         parser_pattern_end_marker_t *end_marker_p) /**< pattern end position  */
{
  if ((flags & (PARSER_PATTERN_TARGET_ON_STACK | PARSER_PATTERN_TARGET_DEFAULT)) != PARSER_PATTERN_TARGET_ON_STACK)
  {
    if (end_marker_p->token.type == LEXER_INVALID_PATTERN)
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_DESTRUCTURING_PATTERN);
    }

    scanner_set_location (context_p, &(end_marker_p->location));
    context_p->token = end_marker_p->token;
  }
  else
  {
    JERRY_ASSERT (!(flags & PARSER_PATTERN_TARGET_DEFAULT));
    lexer_next_token (context_p);
  }

  if ((flags & (PARSER_PATTERN_BINDING | PARSER_PATTERN_NESTED_PATTERN)) == PARSER_PATTERN_BINDING)
  {
    /* Pop the result of the expression. */
    parser_emit_cbc (context_p, CBC_POP);
  }

  parser_flush_cbc (context_p);
} /* parser_pattern_finalize */

/**
 * Emit right-hand-side target value.
 */
static void
parser_pattern_emit_rhs (parser_context_t *context_p, /**< context */
                         uint16_t rhs_opcode, /**< opcode to process the rhs value */
                         uint16_t literal_index) /**< literal index for object pattern */
{
  if (literal_index != PARSER_PATTERN_RHS_NO_LIT)
  {
    parser_emit_cbc_ext_literal (context_p, rhs_opcode, literal_index);
  }
  else
  {
    parser_emit_cbc_ext (context_p, rhs_opcode);
  }
} /* parser_pattern_emit_rhs */

/**
 * Form an assignment from a pattern.
 */
static void
parser_pattern_form_assignment (parser_context_t *context_p, /**< context */
                                parser_pattern_flags_t flags, /**< flags */
                                uint16_t rhs_opcode, /**< opcode to process the rhs value */
                                uint16_t literal_index, /**< literal index for object pattern */
                                parser_line_counter_t ident_line_counter) /**< identifier line counter */
{
  JERRY_UNUSED (ident_line_counter);

  uint16_t name_index = PARSER_INVALID_LITERAL_INDEX;

  if ((flags & PARSER_PATTERN_BINDING)
      || (context_p->last_cbc_opcode == CBC_PUSH_LITERAL && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL))
  {
    name_index = context_p->lit_object.index;
  }

  parser_stack_push_uint8 (context_p, LEXER_EXPRESSION_START);
  parser_append_binary_single_assignment_token (context_p, flags);
  parser_pattern_emit_rhs (context_p, rhs_opcode, literal_index);

  if (context_p->token.type == LEXER_ASSIGN && !(flags & PARSER_PATTERN_REST_ELEMENT))
  {
    parser_branch_t skip_init;
    lexer_next_token (context_p);
    parser_emit_cbc_ext_forward_branch (context_p, CBC_EXT_DEFAULT_INITIALIZER, &skip_init);

    parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

    if (name_index != PARSER_INVALID_LITERAL_INDEX)
    {
      uint16_t function_literal_index = parser_check_anonymous_function_declaration (context_p);

      if (function_literal_index == PARSER_ANONYMOUS_CLASS)
      {
        name_index = scanner_save_literal (context_p, name_index);
        parser_emit_cbc_ext_literal (context_p, CBC_EXT_SET_CLASS_NAME, name_index);
      }
      else if (function_literal_index < PARSER_NAMED_FUNCTION)
      {
        parser_set_function_name (context_p, function_literal_index, name_index, 0);
      }
    }
    parser_set_branch_to_current_position (context_p, &skip_init);
  }

  parser_process_binary_opcodes (context_p, 0);

  JERRY_ASSERT (context_p->stack_top_uint8 == LEXER_EXPRESSION_START);
  parser_stack_pop_uint8 (context_p);

} /* parser_pattern_form_assignment */

/**
 * Parse pattern inside a pattern.
 */
static void
parser_pattern_process_nested_pattern (parser_context_t *context_p, /**< context */
                                       parser_pattern_flags_t flags, /**< flags */
                                       uint16_t rhs_opcode, /**< opcode to process the rhs value */
                                       uint16_t literal_index) /**< literal index for object pattern */
{
  JERRY_ASSERT (context_p->token.type == LEXER_LEFT_BRACE || context_p->token.type == LEXER_LEFT_SQUARE);

  parser_pattern_flags_t options = (parser_pattern_flags_t) ((int) PARSER_PATTERN_NESTED_PATTERN | (int) PARSER_PATTERN_TARGET_ON_STACK
                                    | ((int) flags & ((int) PARSER_PATTERN_BINDING | (int) PARSER_PATTERN_LET | (int) PARSER_PATTERN_CONST
                                    | (int) PARSER_PATTERN_LOCAL | (int) PARSER_PATTERN_ARGUMENTS)));

  JERRY_ASSERT (context_p->next_scanner_info_p->source_p != context_p->source_p
                || context_p->next_scanner_info_p->type == SCANNER_TYPE_INITIALIZER
                || context_p->next_scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS);

  if (context_p->next_scanner_info_p->source_p == context_p->source_p)
  {
    if (context_p->next_scanner_info_p->type == SCANNER_TYPE_INITIALIZER)
    {
      if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_REST)
      {
        options = (parser_pattern_flags_t) ((int) options | (int) PARSER_PATTERN_HAS_REST_ELEMENT);
      }

      if (!(flags & PARSER_PATTERN_REST_ELEMENT))
      {
        options = (parser_pattern_flags_t) ((int) options | (int) PARSER_PATTERN_TARGET_DEFAULT);
      }
      else
      {
        scanner_release_next (context_p, sizeof (scanner_location_info_t));
      }
    }
    else
    {
      if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_REST)
      {
        options = (parser_pattern_flags_t) ((int) options | (int) PARSER_PATTERN_HAS_REST_ELEMENT);
      }
      scanner_release_next (context_p, sizeof (scanner_info_t));
    }
  }

  parser_pattern_emit_rhs (context_p, rhs_opcode, literal_index);

  if (context_p->token.type == LEXER_LEFT_BRACE)
  {
    parser_parse_object_initializer (context_p, options);
  }
  else
  {
    parser_parse_array_initializer (context_p, options);
  }

  parser_emit_cbc (context_p, CBC_POP);
} /* parser_pattern_process_nested_pattern */

/**
 * Process the current {Binding, Assignment}Property
 *
 * @return true, if a nested pattern is processed, false otherwise
 */
static bool
parser_pattern_process_assignment (parser_context_t *context_p, /**< context */
                                   parser_pattern_flags_t flags, /**< flags */
                                   uint16_t rhs_opcode, /**< opcode to process the rhs value */
                                   uint16_t literal_index, /**< literal index for object pattern */
                                   lexer_token_type_t end_type) /**< end type token */
{
  if ((context_p->token.type == LEXER_LEFT_BRACE || context_p->token.type == LEXER_LEFT_SQUARE)
      && (context_p->next_scanner_info_p->source_p != context_p->source_p
          || !(context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_NO_DESTRUCTURING)))
  {
    parser_pattern_process_nested_pattern (context_p, flags, rhs_opcode, literal_index);
    return true;
  }

  parser_line_counter_t ident_line_counter = context_p->token.line;

  if (flags & PARSER_PATTERN_BINDING)
  {
    if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
    {
      parser_raise_error (context_p, PARSER_ERR_IDENTIFIER_EXPECTED);
    }

    lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_IDENT_LITERAL);

    if (flags & (PARSER_PATTERN_LET | PARSER_PATTERN_CONST) && context_p->token.keyword_type == LEXER_KEYW_LET)
    {
      parser_raise_error (context_p, PARSER_ERR_LEXICAL_LET_BINDING);
    }

    if (context_p->next_scanner_info_p->source_p == context_p->source_p)
    {
      JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_ERR_REDECLARED);
      parser_raise_error (context_p, PARSER_ERR_VARIABLE_REDECLARED);
    }

    if (flags & PARSER_PATTERN_ARGUMENTS)
    {
      if (context_p->lit_object.literal_p->status_flags & LEXER_FLAG_FUNCTION_ARGUMENT)
      {
        parser_raise_error (context_p, PARSER_ERR_VARIABLE_REDECLARED);
      }
      context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_FUNCTION_ARGUMENT;
    }
#if JERRY_MODULE_SYSTEM
    parser_module_append_export_name (context_p);
#endif /* JERRY_MODULE_SYSTEM */

    parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_LITERAL);
    lexer_next_token (context_p);

    if (context_p->token.type != end_type && context_p->token.type != LEXER_ASSIGN
        && context_p->token.type != LEXER_COMMA)
    {
      parser_raise_error (context_p, PARSER_ERR_ILLEGAL_PROPERTY_IN_DECLARATION);
    }
  }
  else
  {
    parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA | PARSE_EXPR_LEFT_HAND_SIDE);

    if (!PARSER_IS_PUSH_LITERAL (context_p->last_cbc_opcode) && !PARSER_IS_PUSH_PROP (context_p->last_cbc_opcode))
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_DESTRUCTURING_PATTERN);
    }
  }

  parser_pattern_form_assignment (context_p, flags, rhs_opcode, literal_index, ident_line_counter);
  return false;
} /* parser_pattern_process_assignment */

/**
 * Parse array initializer.
 */
static void
parser_parse_array_initializer (parser_context_t *context_p, /**< context */
                                parser_pattern_flags_t flags) /**< flags */
{
  parser_pattern_end_marker_t end_pos = parser_pattern_get_target (context_p, flags);

  lexer_next_token (context_p);
  parser_emit_cbc_ext (context_p, CBC_EXT_ITERATOR_CONTEXT_CREATE);

  while (context_p->token.type != LEXER_RIGHT_SQUARE)
  {
    uint16_t rhs_opcode = CBC_EXT_ITERATOR_STEP;

    if (context_p->token.type == LEXER_COMMA)
    {
      parser_emit_cbc_ext (context_p, rhs_opcode);
      parser_emit_cbc (context_p, CBC_POP);
      lexer_next_token (context_p);
      continue;
    }

    parser_pattern_flags_t options = flags;

    if (context_p->token.type == LEXER_THREE_DOTS)
    {
      lexer_next_token (context_p);
      rhs_opcode = CBC_EXT_REST_INITIALIZER;
      options = (parser_pattern_flags_t) ((int) options | (int) PARSER_PATTERN_REST_ELEMENT);
    }

    parser_pattern_process_assignment (context_p, options, rhs_opcode, PARSER_PATTERN_RHS_NO_LIT, LEXER_RIGHT_SQUARE);

    if (context_p->token.type == LEXER_COMMA && rhs_opcode != CBC_EXT_REST_INITIALIZER)
    {
      lexer_next_token (context_p);
    }
    else if (context_p->token.type != LEXER_RIGHT_SQUARE)
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_DESTRUCTURING_PATTERN);
    }
  }

  /* close the iterator */
  parser_emit_cbc_ext (context_p, CBC_EXT_ITERATOR_CONTEXT_END);

  parser_pattern_finalize (context_p, flags, &end_pos);
} /* parser_parse_array_initializer */

/**
 * Parse object initializer.
 */
static void
parser_parse_object_initializer (parser_context_t *context_p, /**< context */
                                 parser_pattern_flags_t flags) /**< flags */
{
  parser_pattern_end_marker_t end_pos = parser_pattern_get_target (context_p, flags);

  /* 12.14.5.2:  ObjectAssignmentPattern : { } */
  if (lexer_check_next_character (context_p, LIT_CHAR_RIGHT_BRACE))
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_REQUIRE_OBJECT_COERCIBLE);
    lexer_consume_next_character (context_p);
    parser_pattern_finalize (context_p, flags, &end_pos);
    return;
  }

#ifndef JERRY_NDEBUG
  bool rest_found = false;
#endif /* !defined(JERRY_NDEBUG) */

  cbc_ext_opcode_t context_opcode = CBC_EXT_OBJ_INIT_CONTEXT_CREATE;

  if (flags & PARSER_PATTERN_HAS_REST_ELEMENT)
  {
    context_opcode = CBC_EXT_OBJ_INIT_REST_CONTEXT_CREATE;
  }

  parser_emit_cbc_ext (context_p, context_opcode);

  while (true)
  {
    lexer_expect_object_literal_id (context_p, LEXER_OBJ_IDENT_OBJECT_PATTERN);

    uint16_t prop_index = context_p->lit_object.index;
    parser_line_counter_t start_line = context_p->token.line;
    parser_line_counter_t start_column = context_p->token.column;
    uint16_t push_prop_opcode = CBC_EXT_INITIALIZER_PUSH_PROP_LITERAL;

    if (context_p->token.type == LEXER_RIGHT_BRACE)
    {
      break;
    }

    if (context_p->token.type == LEXER_THREE_DOTS)
    {
      lexer_next_token (context_p);

      flags = (parser_pattern_flags_t) ((int) flags | (int) PARSER_PATTERN_REST_ELEMENT);

      if (parser_pattern_process_assignment (context_p,
                                             flags,
                                             CBC_EXT_OBJ_INIT_PUSH_REST,
                                             PARSER_PATTERN_RHS_NO_LIT,
                                             LEXER_RIGHT_BRACE))
      {
        parser_raise_error (context_p, PARSER_ERR_INVALID_LHS_ASSIGNMENT);
      }

      if (context_p->token.type != LEXER_RIGHT_BRACE)
      {
        parser_raise_error (context_p, PARSER_ERR_RIGHT_BRACE_EXPECTED);
      }

#ifndef JERRY_NDEBUG
      rest_found = true;
#endif /* !defined(JERRY_NDEBUG) */
      break;
    }

    if (context_p->token.type == LEXER_RIGHT_SQUARE)
    {
      prop_index = PARSER_PATTERN_RHS_NO_LIT;
      push_prop_opcode =
        ((flags & PARSER_PATTERN_HAS_REST_ELEMENT) ? CBC_EXT_INITIALIZER_PUSH_NAME : CBC_EXT_INITIALIZER_PUSH_PROP);
    }
    else if (flags & PARSER_PATTERN_HAS_REST_ELEMENT)
    {
      push_prop_opcode = CBC_EXT_INITIALIZER_PUSH_NAME_LITERAL;
    }

    if (context_p->next_scanner_info_p->source_p == context_p->source_p)
    {
      JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_ERR_REDECLARED);
      parser_raise_error (context_p, PARSER_ERR_VARIABLE_REDECLARED);
    }

    lexer_next_token (context_p);

    if (context_p->token.type == LEXER_COLON)
    {
      lexer_next_token (context_p);
      parser_pattern_process_assignment (context_p, flags, push_prop_opcode, prop_index, LEXER_RIGHT_BRACE);
    }
    else
    {
      if (push_prop_opcode == CBC_EXT_INITIALIZER_PUSH_NAME || push_prop_opcode == CBC_EXT_INITIALIZER_PUSH_PROP)
      {
        parser_raise_error (context_p, PARSER_ERR_COLON_EXPECTED);
      }

      if (context_p->token.type != LEXER_RIGHT_BRACE && context_p->token.type != LEXER_ASSIGN
          && context_p->token.type != LEXER_COMMA)
      {
        parser_raise_error (context_p, PARSER_ERR_OBJECT_ITEM_SEPARATOR_EXPECTED);
      }

      parser_reparse_as_common_identifier (context_p, start_line, start_column);

      if (flags & PARSER_PATTERN_ARGUMENTS)
      {
        if (context_p->lit_object.literal_p->status_flags & LEXER_FLAG_FUNCTION_ARGUMENT)
        {
          parser_raise_error (context_p, PARSER_ERR_VARIABLE_REDECLARED);
        }
        context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_FUNCTION_ARGUMENT;
      }

#if JERRY_MODULE_SYSTEM
      parser_module_append_export_name (context_p);
#endif /* JERRY_MODULE_SYSTEM */

      parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_LITERAL);

      lexer_next_token (context_p);
      JERRY_ASSERT (context_p->token.type == LEXER_RIGHT_BRACE || context_p->token.type == LEXER_ASSIGN
                    || context_p->token.type == LEXER_COMMA);

      parser_pattern_form_assignment (context_p, flags, push_prop_opcode, prop_index, start_line);
    }

    if (context_p->token.type == LEXER_RIGHT_BRACE)
    {
      break;
    }
    else if (context_p->token.type != LEXER_COMMA)
    {
      parser_raise_error (context_p, PARSER_ERR_OBJECT_ITEM_SEPARATOR_EXPECTED);
    }
  }

  if (flags & PARSER_PATTERN_HAS_REST_ELEMENT)
  {
    PARSER_MINUS_EQUAL_U16 (context_p->stack_depth,
                            (PARSER_OBJ_INIT_REST_CONTEXT_STACK_ALLOCATION - PARSER_OBJ_INIT_CONTEXT_STACK_ALLOCATION));
  }

  parser_emit_cbc_ext (context_p, CBC_EXT_OBJ_INIT_CONTEXT_END);

  parser_pattern_finalize (context_p, flags, &end_pos);

#ifndef JERRY_NDEBUG
  /* Checked at the end because there might be syntax errors before. */
  JERRY_ASSERT (!!(flags & PARSER_PATTERN_HAS_REST_ELEMENT) == rest_found);
#endif /* !defined(JERRY_NDEBUG) */
} /* parser_parse_object_initializer */

/**
 * Parse an initializer.
 */
void
parser_parse_initializer (parser_context_t *context_p, /**< context */
                          parser_pattern_flags_t flags) /**< flags */
{
  if (context_p->token.type == LEXER_LEFT_BRACE)
  {
    parser_parse_object_initializer (context_p, flags);
  }
  else
  {
    JERRY_ASSERT (context_p->token.type == LEXER_LEFT_SQUARE);
    parser_parse_array_initializer (context_p, flags);
  }
} /* parser_parse_initializer */

/**
 * Parse an initializer using the next character.
 */
void
parser_parse_initializer_by_next_char (parser_context_t *context_p, /**< context */
                                       parser_pattern_flags_t flags) /**< flags */
{
  JERRY_ASSERT (lexer_check_next_characters (context_p, LIT_CHAR_LEFT_SQUARE, LIT_CHAR_LEFT_BRACE));

  if (lexer_consume_next_character (context_p) == LIT_CHAR_LEFT_BRACE)
  {
    if (context_p->next_scanner_info_p->source_p == context_p->source_p)
    {
      JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_INITIALIZER
                    || context_p->next_scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS);

      if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_REST)
      {
        flags = (parser_pattern_flags_t) ((int) flags | (int) PARSER_PATTERN_HAS_REST_ELEMENT);
      }

      if (context_p->next_scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS)
      {
        scanner_release_next (context_p, sizeof (scanner_info_t));
      }
    }

    parser_parse_object_initializer (context_p, flags);
  }
  else
  {
    parser_parse_array_initializer (context_p, flags);
  }
} /* parser_parse_initializer_by_next_char */

/**
 * Process ternary expression.
 */
static void
parser_process_ternary_expression (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (context_p->token.type == LEXER_QUESTION_MARK);

  cbc_opcode_t opcode = CBC_BRANCH_IF_FALSE_FORWARD;
  parser_branch_t cond_branch;
  parser_branch_t uncond_branch;

  parser_push_result (context_p);

  if (context_p->last_cbc_opcode == CBC_LOGICAL_NOT)
  {
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
    opcode = CBC_BRANCH_IF_TRUE_FORWARD;
  }

  parser_emit_cbc_forward_branch (context_p, (uint16_t) opcode, &cond_branch);

  lexer_next_token (context_p);
  parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);
  parser_emit_cbc_forward_branch (context_p, CBC_JUMP_FORWARD, &uncond_branch);
  parser_set_branch_to_current_position (context_p, &cond_branch);

  /* Although byte code is constructed for two branches,
   * only one of them will be executed. To reflect this
   * the stack is manually adjusted. */
  JERRY_ASSERT (context_p->stack_depth > 0);
  context_p->stack_depth--;

  if (context_p->token.type != LEXER_COLON)
  {
    parser_raise_error (context_p, PARSER_ERR_COLON_FOR_CONDITIONAL_EXPECTED);
  }

  lexer_next_token (context_p);

  parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);
  parser_set_branch_to_current_position (context_p, &uncond_branch);

  /* Last opcode rewrite is not allowed because
   * the result may come from the first branch. */
  parser_flush_cbc (context_p);

  parser_process_binary_opcodes (context_p, 0);
} /* parser_process_ternary_expression */

/**
 * Process expression sequence.
 */
static void
parser_process_expression_sequence (parser_context_t *context_p) /**< context */
{
  if (!CBC_NO_RESULT_OPERATION (context_p->last_cbc_opcode))
  {
    parser_emit_cbc (context_p, CBC_POP);
  }

  if (context_p->stack_top_uint8 == LEXER_LEFT_PAREN)
  {
    parser_mem_page_t *page_p = context_p->stack.first_p;

    JERRY_ASSERT (page_p != NULL);

    page_p->bytes[context_p->stack.last_position - 1] = LEXER_COMMA_SEP_LIST;
    context_p->stack_top_uint8 = LEXER_COMMA_SEP_LIST;
  }

  lexer_next_token (context_p);
} /* parser_process_expression_sequence */

/**
 * Process group expression.
 */
static void
parser_process_group_expression (parser_context_t *context_p, /**< context */
                                 size_t *grouping_level_p) /**< grouping level */
{
  JERRY_ASSERT (*grouping_level_p >= PARSER_GROUPING_LEVEL_INCREASE);
  (*grouping_level_p) -= PARSER_GROUPING_LEVEL_INCREASE;

  uint8_t token = context_p->stack_top_uint8;

  if (token == LEXER_COMMA_SEP_LIST)
  {
    parser_push_result (context_p);
    parser_flush_cbc (context_p);
  }

  parser_stack_pop_uint8 (context_p);
  lexer_next_token (context_p);

  /* Lookahead for anonymous function declaration after '=' token when the assignment base is LHS expression
     with a single identifier in it. e.g.: (a) = function () {} */
  if (JERRY_UNLIKELY (context_p->token.type == LEXER_ASSIGN
                      && PARSER_IS_PUSH_LITERALS_WITH_THIS (context_p->last_cbc_opcode)
                      && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL
                      && parser_is_assignment_expr (context_p) && *grouping_level_p != PARSE_EXPR_LEFT_HAND_SIDE))
  {
    parser_stack_push_uint8 (context_p, LEXER_ASSIGN_GROUP_EXPR);
  }
} /* parser_process_group_expression */

/**
 * Parse block expression.
 */
void
parser_parse_block_expression (parser_context_t *context_p, /**< context */
                               int options) /**< option flags */
{
  parser_parse_expression (context_p, options | PARSE_EXPR_NO_PUSH_RESULT);

  if (CBC_NO_RESULT_OPERATION (context_p->last_cbc_opcode))
  {
    JERRY_ASSERT (CBC_SAME_ARGS (context_p->last_cbc_opcode, context_p->last_cbc_opcode + 2));
    PARSER_PLUS_EQUAL_U16 (context_p->last_cbc_opcode, 2);
    parser_flush_cbc (context_p);
  }
  else
  {
    parser_emit_cbc (context_p, CBC_POP_BLOCK);
  }
} /* parser_parse_block_expression */

/**
 * Parse expression statement.
 */
void
parser_parse_expression_statement (parser_context_t *context_p, /**< context */
                                   int options) /**< option flags */
{
  parser_parse_expression (context_p, options | PARSE_EXPR_NO_PUSH_RESULT);

  if (!CBC_NO_RESULT_OPERATION (context_p->last_cbc_opcode))
  {
    parser_emit_cbc (context_p, CBC_POP);
  }
} /* parser_parse_expression_statement */

JERRY_STATIC_ASSERT ((PARSE_EXPR_LEFT_HAND_SIDE == 0x1), value_of_parse_expr_left_hand_side_must_be_1);

/**
 * Parse expression.
 */
void
parser_parse_expression (parser_context_t *context_p, /**< context */
                         int options) /**< option flags */
{
  size_t grouping_level = (options & PARSE_EXPR_LEFT_HAND_SIDE);

  parser_stack_push_uint8 (context_p, LEXER_EXPRESSION_START);

  if (options & PARSE_EXPR_HAS_LITERAL)
  {
    JERRY_ASSERT (context_p->last_cbc_opcode == CBC_PUSH_LITERAL);
    goto process_unary_expression;
  }

  while (true)
  {
    if (parser_parse_unary_expression (context_p, &grouping_level))
    {
      parser_process_binary_opcodes (context_p, 0);
      break;
    }

    while (true)
    {
process_unary_expression:
      parser_process_unary_expression (context_p, grouping_level);

      if (JERRY_LIKELY (grouping_level != PARSE_EXPR_LEFT_HAND_SIDE))
      {
        uint8_t min_prec_threshhold = 0;

        if (LEXER_IS_BINARY_OP_TOKEN (context_p->token.type))
        {
          if (JERRY_UNLIKELY (context_p->token.type == LEXER_NULLISH_COALESCING))
          {
            parser_check_invalid_logical_op (context_p, LEXER_LOGICAL_OR, LEXER_LOGICAL_AND);
          }

          min_prec_threshhold = parser_binary_precedence_table[context_p->token.type - LEXER_FIRST_BINARY_OP];

          /* Check for BINARY_LVALUE tokens + LEXER_LOGICAL_OR + LEXER_LOGICAL_AND + LEXER_EXPONENTIATION */
          if ((min_prec_threshhold == PARSER_RIGHT_TO_LEFT_ORDER_EXPONENTIATION)
              || (min_prec_threshhold <= PARSER_RIGHT_TO_LEFT_ORDER_MAX_PRECEDENCE
                  && min_prec_threshhold != PARSER_RIGHT_TO_LEFT_ORDER_TERNARY_PRECEDENCE))
          {
            /* Right-to-left evaluation order. */
            min_prec_threshhold++;
          }
        }

        parser_process_binary_opcodes (context_p, min_prec_threshhold);
      }
      if (context_p->token.type == LEXER_RIGHT_PAREN
          && (context_p->stack_top_uint8 == LEXER_LEFT_PAREN || context_p->stack_top_uint8 == LEXER_COMMA_SEP_LIST))
      {
        parser_process_group_expression (context_p, &grouping_level);
        continue;
      }

      break;
    }

    if (grouping_level == PARSE_EXPR_LEFT_HAND_SIDE)
    {
      break;
    }

    if (JERRY_UNLIKELY (context_p->token.type == LEXER_QUESTION_MARK))
    {
      parser_process_ternary_expression (context_p);

      if (context_p->token.type == LEXER_RIGHT_PAREN)
      {
        goto process_unary_expression;
      }
    }
    else if (LEXER_IS_BINARY_OP_TOKEN (context_p->token.type))
    {
      parser_append_binary_token (context_p);
      lexer_next_token (context_p);
      continue;
    }

    if (JERRY_UNLIKELY (context_p->token.type == LEXER_COMMA)
        && (!(options & PARSE_EXPR_NO_COMMA) || grouping_level >= PARSER_GROUPING_LEVEL_INCREASE))
    {
      parser_process_expression_sequence (context_p);
      continue;
    }

    break;
  }

  if (grouping_level >= PARSER_GROUPING_LEVEL_INCREASE)
  {
    parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
  }

  JERRY_ASSERT (context_p->stack_top_uint8 == LEXER_EXPRESSION_START);
  parser_stack_pop_uint8 (context_p);

  if (!(options & PARSE_EXPR_NO_PUSH_RESULT))
  {
    parser_push_result (context_p);
  }
} /* parser_parse_expression */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_PARSER */
