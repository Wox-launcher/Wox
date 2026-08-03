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
#include "js-scanner-internal.h"
#include "lit-char-helpers.h"

#if JERRY_PARSER

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_scanner Scanner
 * @{
 */

/**
 * Add the "async" literal to the literal pool.
 */
void
scanner_add_async_literal (parser_context_t *context_p, /**< context */
                           scanner_context_t *scanner_context_p) /**< scanner context */
{
  lexer_lit_location_t async_literal;

  JERRY_ASSERT (context_p->stack_top_uint8 == SCAN_STACK_USE_ASYNC);

  parser_stack_pop_uint8 (context_p);
  parser_stack_pop (context_p, &async_literal, sizeof (lexer_lit_location_t));

  lexer_lit_location_t *lit_location_p =
    scanner_add_custom_literal (context_p, scanner_context_p->active_literal_pool_p, &async_literal);

  lit_location_p->type |= SCANNER_LITERAL_IS_USED;

  if (scanner_context_p->active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_WITH)
  {
    lit_location_p->type |= SCANNER_LITERAL_NO_REG;
  }
} /* scanner_add_async_literal */

/**
 * Init scanning the body of an arrow function.
 */
static void
scanner_check_arrow_body (parser_context_t *context_p, /**< context */
                          scanner_context_t *scanner_context_p) /**< scanner context */
{
  lexer_next_token (context_p);

  scanner_context_p->active_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_ARROW;

  if (context_p->token.type != LEXER_LEFT_BRACE)
  {
    scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
    parser_stack_push_uint8 (context_p, SCAN_STACK_ARROW_EXPRESSION);
    return;
  }

  lexer_next_token (context_p);
  parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_ARROW);
  scanner_check_directives (context_p, scanner_context_p);
} /* scanner_check_arrow_body */

/**
 * Process arrow function with argument list.
 */
void
scanner_check_arrow (parser_context_t *context_p, /**< context */
                     scanner_context_t *scanner_context_p) /**< scanner context */
{
  parser_stack_pop_uint8 (context_p);

  lexer_next_token (context_p);

  if (context_p->token.type != LEXER_ARROW || (context_p->token.flags & LEXER_WAS_NEWLINE))
  {
    if (context_p->stack_top_uint8 == SCAN_STACK_USE_ASYNC)
    {
      scanner_add_async_literal (context_p, scanner_context_p);
    }

    scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
    scanner_pop_literal_pool (context_p, scanner_context_p);
    return;
  }

  if (context_p->stack_top_uint8 == SCAN_STACK_USE_ASYNC)
  {
    parser_stack_pop (context_p, NULL, sizeof (lexer_lit_location_t) + 1);
  }

  scanner_literal_pool_t *literal_pool_p = scanner_context_p->active_literal_pool_p;
  uint16_t status_flags = literal_pool_p->status_flags;

  bool is_async_arrow = (status_flags & SCANNER_LITERAL_POOL_MAY_ASYNC_ARROW) != 0;

  status_flags |= SCANNER_LITERAL_POOL_ARROW_FLAGS;
  status_flags &=
    (uint16_t) ~(SCANNER_LITERAL_POOL_IN_WITH | SCANNER_LITERAL_POOL_GENERATOR | SCANNER_LITERAL_POOL_ASYNC);

  context_p->status_flags &= (uint32_t) ~(PARSER_IS_GENERATOR_FUNCTION | PARSER_IS_ASYNC_FUNCTION);

  if (is_async_arrow)
  {
    status_flags |= SCANNER_LITERAL_POOL_ASYNC;
    context_p->status_flags |= PARSER_IS_ASYNC_FUNCTION;
  }

  literal_pool_p->status_flags = status_flags;

  scanner_filter_arguments (context_p, scanner_context_p);
  scanner_check_arrow_body (context_p, scanner_context_p);
} /* scanner_check_arrow */

/**
 * Process arrow function with a single argument.
 */
void
scanner_scan_simple_arrow (parser_context_t *context_p, /**< context */
                           scanner_context_t *scanner_context_p, /**< scanner context */
                           const uint8_t *source_p) /**< identifier end position */
{
  uint16_t status_flags = SCANNER_LITERAL_POOL_ARROW_FLAGS;

  context_p->status_flags &= (uint32_t) ~(PARSER_IS_GENERATOR_FUNCTION | PARSER_IS_ASYNC_FUNCTION);

  if (scanner_context_p->async_source_p != NULL)
  {
    JERRY_ASSERT (scanner_context_p->async_source_p == source_p);

    status_flags |= SCANNER_LITERAL_POOL_ASYNC;
    context_p->status_flags |= PARSER_IS_ASYNC_FUNCTION;
  }

  scanner_literal_pool_t *literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, status_flags);
  literal_pool_p->source_p = source_p;

  lexer_lit_location_t *location_p = scanner_add_literal (context_p, scanner_context_p);
  location_p->type |= SCANNER_LITERAL_IS_ARG;

  /* Skip the => token, which size is two. */
  context_p->source_p += 2;
  PARSER_PLUS_EQUAL_LC (context_p->column, 2);
  context_p->token.flags = (uint8_t) (context_p->token.flags & ~LEXER_NO_SKIP_SPACES);

  scanner_check_arrow_body (context_p, scanner_context_p);
} /* scanner_scan_simple_arrow */

/**
 * Process the next argument of a might-be arrow function.
 */
void
scanner_check_arrow_arg (parser_context_t *context_p, /**< context */
                         scanner_context_t *scanner_context_p) /**< scanner context */
{
  JERRY_ASSERT (context_p->stack_top_uint8 == SCAN_STACK_ARROW_ARGUMENTS);

  const uint8_t *source_p = context_p->source_p;
  bool process_arrow = false;

  scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

  if (context_p->token.type == LEXER_THREE_DOTS)
  {
    lexer_next_token (context_p);
  }

  switch (context_p->token.type)
  {
    case LEXER_RIGHT_PAREN:
    {
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
      return;
    }
    case LEXER_LITERAL:
    {
      if (context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
      {
        break;
      }

      scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;

      if (lexer_check_arrow (context_p))
      {
        process_arrow = true;
        break;
      }

      lexer_lit_location_t *argument_literal_p = scanner_append_argument (context_p, scanner_context_p);

      scanner_detect_eval_call (context_p, scanner_context_p);

      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_COMMA || context_p->token.type == LEXER_RIGHT_PAREN)
      {
        return;
      }

      if (context_p->token.type != LEXER_ASSIGN)
      {
        break;
      }

      if (argument_literal_p->type & SCANNER_LITERAL_IS_USED)
      {
        JERRY_ASSERT (argument_literal_p->type & SCANNER_LITERAL_EARLY_CREATE);
        return;
      }

      scanner_binding_literal_t binding_literal;
      binding_literal.literal_p = argument_literal_p;

      parser_stack_push (context_p, &binding_literal, sizeof (scanner_binding_literal_t));
      parser_stack_push_uint8 (context_p, SCAN_STACK_BINDING_INIT);
      return;
    }
    case LEXER_LEFT_SQUARE:
    case LEXER_LEFT_BRACE:
    {
      scanner_append_hole (context_p, scanner_context_p);
      scanner_push_destructuring_pattern (context_p, scanner_context_p, SCANNER_BINDING_ARROW_ARG, false);

      if (context_p->token.type == LEXER_LEFT_BRACE)
      {
        parser_stack_push_uint8 (context_p, 0);
        parser_stack_push_uint8 (context_p, SCAN_STACK_OBJECT_LITERAL);
        scanner_context_p->mode = SCAN_MODE_PROPERTY_NAME;
        return;
      }

      parser_stack_push_uint8 (context_p, SCAN_STACK_ARRAY_LITERAL);
      scanner_context_p->mode = SCAN_MODE_BINDING;
      lexer_next_token (context_p);
      return;
    }
  }

  scanner_pop_literal_pool (context_p, scanner_context_p);
  parser_stack_pop_uint8 (context_p);

  if (context_p->stack_top_uint8 == SCAN_STACK_USE_ASYNC)
  {
    scanner_add_async_literal (context_p, scanner_context_p);
  }

  parser_stack_push_uint8 (context_p, SCAN_STACK_PAREN_EXPRESSION);

  if (process_arrow)
  {
    scanner_scan_simple_arrow (context_p, scanner_context_p, source_p);
  }
} /* scanner_check_arrow_arg */

/**
 * Detect async functions.
 *
 * @return true, if async is followed by a function keyword, false otherwise
 */
bool
scanner_check_async_function (parser_context_t *context_p, /**< context */
                              scanner_context_t *scanner_context_p) /**< scanner context */
{
  JERRY_ASSERT (lexer_token_is_async (context_p));
  JERRY_ASSERT (scanner_context_p->mode == SCAN_MODE_PRIMARY_EXPRESSION
                || scanner_context_p->mode == SCAN_MODE_PRIMARY_EXPRESSION_AFTER_NEW);
  JERRY_ASSERT (scanner_context_p->async_source_p != NULL);

  lexer_lit_location_t async_literal = context_p->token.lit_location;

  lexer_next_token (context_p);

  if (!(context_p->token.flags & LEXER_WAS_NEWLINE))
  {
    if (context_p->token.type == LEXER_KEYW_FUNCTION)
    {
      return true;
    }

    if (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
    {
      if (!lexer_check_arrow (context_p))
      {
        scanner_raise_error (context_p);
      }

      scanner_scan_simple_arrow (context_p, scanner_context_p, scanner_context_p->async_source_p);
      scanner_context_p->async_source_p = NULL;
      return false;
    }

    if (context_p->token.type == LEXER_LEFT_PAREN)
    {
      parser_stack_push (context_p, &async_literal, sizeof (lexer_lit_location_t));
      parser_stack_push_uint8 (context_p, SCAN_STACK_USE_ASYNC);
      return false;
    }
  }

  lexer_lit_location_t *lit_location_p =
    scanner_add_custom_literal (context_p, scanner_context_p->active_literal_pool_p, &async_literal);
  lit_location_p->type |= SCANNER_LITERAL_IS_USED;

  if (scanner_context_p->active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_WITH)
  {
    lit_location_p->type |= SCANNER_LITERAL_NO_REG;
  }

  scanner_context_p->async_source_p = NULL;
  scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
  return false;
} /* scanner_check_async_function */

/**
 * Check whether the statement of an if/else construct is a function statement.
 */
void
scanner_check_function_after_if (parser_context_t *context_p, /**< context */
                                 scanner_context_t *scanner_context_p) /**< scanner context */
{
  lexer_next_token (context_p);
  scanner_context_p->mode = SCAN_MODE_STATEMENT;

  if (JERRY_UNLIKELY (context_p->token.type == LEXER_KEYW_FUNCTION))
  {
    scanner_literal_pool_t *literal_pool_p;
    literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, 0);

    literal_pool_p->source_p = context_p->source_p;
    parser_stack_push_uint8 (context_p, SCAN_STACK_PRIVATE_BLOCK);
  }
} /* scanner_check_function_after_if */

#if JERRY_MODULE_SYSTEM

/**
 * Check whether the next token is meta.
 */
void
scanner_check_import_meta (parser_context_t *context_p) /**< context */
{
  lexer_next_token (context_p);

  if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL
      || context_p->token.keyword_type != LEXER_KEYW_META
      || (context_p->token.lit_location.status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE))
  {
    scanner_raise_error (context_p);
  }

  lexer_next_token (context_p);

  context_p->global_status_flags |= ECMA_PARSE_INTERNAL_HAS_IMPORT_META;
} /* scanner_check_import_meta */

#endif /* JERRY_MODULE_SYSTEM */

/**
 * Arrow types for scanner_scan_bracket() function.
 */
typedef enum
{
  SCANNER_SCAN_BRACKET_NO_ARROW, /**< not an arrow function */
  SCANNER_SCAN_BRACKET_SIMPLE_ARROW, /**< simple arrow function */
  SCANNER_SCAN_BRACKET_ARROW_WITH_ONE_ARG, /**< arrow function with one argument */
} scanner_scan_bracket_arrow_type_t;

/**
 * Scan bracketed expressions.
 */
void
scanner_scan_bracket (parser_context_t *context_p, /**< context */
                      scanner_context_t *scanner_context_p) /**< scanner context */
{
  size_t depth = 0;
  const uint8_t *arrow_source_p;
  const uint8_t *async_source_p = NULL;
  scanner_scan_bracket_arrow_type_t arrow_type = SCANNER_SCAN_BRACKET_NO_ARROW;

  JERRY_ASSERT (context_p->token.type == LEXER_LEFT_PAREN);

  do
  {
    arrow_source_p = context_p->source_p;
    depth++;
    lexer_next_token (context_p);
  } while (context_p->token.type == LEXER_LEFT_PAREN);

  scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

  switch (context_p->token.type)
  {
    case LEXER_LITERAL:
    {
      if (context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
      {
        arrow_source_p = NULL;
        break;
      }

      const uint8_t *source_p = context_p->source_p;

      if (lexer_check_arrow (context_p))
      {
        arrow_source_p = source_p;
        arrow_type = SCANNER_SCAN_BRACKET_SIMPLE_ARROW;
        break;
      }

      size_t total_depth = depth;

      while (depth > 0 && lexer_check_next_character (context_p, LIT_CHAR_RIGHT_PAREN))
      {
        lexer_consume_next_character (context_p);
        depth--;
      }

      if (context_p->token.keyword_type == LEXER_KEYW_EVAL
          && lexer_check_next_character (context_p, LIT_CHAR_LEFT_PAREN))
      {
        /* A function call cannot be an eval function. */
        arrow_source_p = NULL;
        const uint16_t flags = (uint16_t) (SCANNER_LITERAL_POOL_CAN_EVAL | SCANNER_LITERAL_POOL_HAS_SUPER_REFERENCE);

        scanner_context_p->active_literal_pool_p->status_flags |= flags;
        break;
      }

      if (total_depth == depth)
      {
        if (lexer_check_arrow_param (context_p))
        {
          JERRY_ASSERT (depth > 0);
          depth--;
          break;
        }

        if (JERRY_UNLIKELY (lexer_token_is_async (context_p)))
        {
          async_source_p = source_p;
        }
      }

      if (depth == total_depth - 1 && lexer_check_arrow (context_p))
      {
        arrow_type = SCANNER_SCAN_BRACKET_ARROW_WITH_ONE_ARG;
        break;
      }

      if (context_p->stack_top_uint8 == SCAN_STACK_USE_ASYNC)
      {
        scanner_add_async_literal (context_p, scanner_context_p);
      }

      arrow_source_p = NULL;
      break;
    }
    case LEXER_THREE_DOTS:
    case LEXER_LEFT_SQUARE:
    case LEXER_LEFT_BRACE:
    case LEXER_RIGHT_PAREN:
    {
      JERRY_ASSERT (depth > 0);
      depth--;
      break;
    }
    default:
    {
      arrow_source_p = NULL;
      break;
    }
  }

  if (JERRY_UNLIKELY (scanner_context_p->async_source_p != NULL) && (arrow_source_p == NULL || depth > 0))
  {
    scanner_context_p->async_source_p = NULL;
  }

  while (depth > 0)
  {
    parser_stack_push_uint8 (context_p, SCAN_STACK_PAREN_EXPRESSION);
    depth--;
  }

  if (arrow_source_p != NULL)
  {
    JERRY_ASSERT (async_source_p == NULL);

    if (arrow_type == SCANNER_SCAN_BRACKET_SIMPLE_ARROW)
    {
      scanner_scan_simple_arrow (context_p, scanner_context_p, arrow_source_p);
      return;
    }

    parser_stack_push_uint8 (context_p, SCAN_STACK_ARROW_ARGUMENTS);

    uint16_t status_flags = 0;

    if (JERRY_UNLIKELY (scanner_context_p->async_source_p != NULL))
    {
      status_flags |= SCANNER_LITERAL_POOL_MAY_ASYNC_ARROW;
      arrow_source_p = scanner_context_p->async_source_p;
      scanner_context_p->async_source_p = NULL;
    }

    scanner_literal_pool_t *literal_pool_p;
    literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, status_flags);
    literal_pool_p->source_p = arrow_source_p;

    if (arrow_type == SCANNER_SCAN_BRACKET_ARROW_WITH_ONE_ARG)
    {
      scanner_append_argument (context_p, scanner_context_p);
      scanner_detect_eval_call (context_p, scanner_context_p);

      context_p->token.type = LEXER_RIGHT_PAREN;
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
    }
    else if (context_p->token.type == LEXER_RIGHT_PAREN)
    {
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
    }
    else
    {
      scanner_check_arrow_arg (context_p, scanner_context_p);
    }
  }
  else if (JERRY_UNLIKELY (async_source_p != NULL))
  {
    scanner_context_p->async_source_p = async_source_p;
    scanner_check_async_function (context_p, scanner_context_p);
  }
} /* scanner_scan_bracket */

/**
 * Check directives before a source block.
 */
void
scanner_check_directives (parser_context_t *context_p, /**< context */
                          scanner_context_t *scanner_context_p) /**< scanner context */
{
  scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;

  while (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_STRING_LITERAL)
  {
    bool is_use_strict = false;

    if (lexer_string_is_use_strict (context_p) && !(context_p->status_flags & PARSER_IS_STRICT))
    {
      is_use_strict = true;
      context_p->status_flags |= PARSER_IS_STRICT;
    }

    lexer_next_token (context_p);

    if (!lexer_string_is_directive (context_p))
    {
      if (is_use_strict)
      {
        context_p->status_flags &= (uint32_t) ~PARSER_IS_STRICT;
      }

      /* The string is part of an expression statement. */
      scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
      break;
    }

    if (is_use_strict)
    {
      scanner_context_p->active_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_IS_STRICT;
    }

    if (context_p->token.type == LEXER_SEMICOLON)
    {
      lexer_next_token (context_p);
    }
  }
} /* scanner_check_directives */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_PARSER */
