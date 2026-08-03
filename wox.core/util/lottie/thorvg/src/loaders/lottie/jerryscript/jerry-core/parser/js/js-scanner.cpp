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

#include "jcontext.h"
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
 * Scan return types.
 */
typedef enum
{
  SCAN_NEXT_TOKEN, /**< get next token after return */
  SCAN_KEEP_TOKEN, /**< keep the current token after return */
} scan_return_types_t;

/**
 * Checks whether token type is "of".
 */
#define SCANNER_IDENTIFIER_IS_OF() (lexer_token_is_identifier (context_p, "of", 2))

JERRY_STATIC_ASSERT ((SCANNER_FROM_LITERAL_POOL_TO_COMPUTED (SCANNER_LITERAL_POOL_GENERATOR) == SCAN_STACK_COMPUTED_GENERATOR),
                     scanner_invalid_conversion_from_literal_pool_generator_to_computed_generator);
JERRY_STATIC_ASSERT ((SCANNER_FROM_LITERAL_POOL_TO_COMPUTED (SCANNER_LITERAL_POOL_ASYNC) == SCAN_STACK_COMPUTED_ASYNC),
                     scanner_invalid_conversion_from_literal_pool_async_to_computed_async);

JERRY_STATIC_ASSERT ((SCANNER_FROM_COMPUTED_TO_LITERAL_POOL (SCAN_STACK_COMPUTED_GENERATOR) == SCANNER_LITERAL_POOL_GENERATOR),
                     scanner_invalid_conversion_from_computed_generator_to_literal_pool_generator);
JERRY_STATIC_ASSERT ((SCANNER_FROM_COMPUTED_TO_LITERAL_POOL (SCAN_STACK_COMPUTED_ASYNC) == SCANNER_LITERAL_POOL_ASYNC),
                     scanner_invalid_conversion_from_computed_async_to_literal_pool_async);

/**
 * Change scanner mode from primary expression to post primary expression.
 *
 * @return SCAN_NEXT_TOKEN to read the next token, or SCAN_KEEP_TOKEN to do nothing
 */
static scan_return_types_t
scanner_primary_to_post_primary_expression (parser_context_t *context_p, /**< context */
                                            scanner_context_t *scanner_context_p) /* scanner context */
{
  scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;

  if (JERRY_UNLIKELY (context_p->stack_top_uint8 == SCAN_STACK_CLASS_FIELD_INITIALIZER
                      && (context_p->status_flags & PARSER_IS_STRICT)))
  {
    lexer_scan_identifier (context_p, (lexer_parse_options_t) ((unsigned int) LEXER_PARSE_CHECK_KEYWORDS | (unsigned int) LEXER_PARSE_NO_STRICT_IDENT_ERROR));

    if (context_p->token.type == LEXER_LITERAL && lexer_compare_literal_to_string (context_p, "static", 6))
    {
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
    }

    return SCAN_KEEP_TOKEN;
  }

  return SCAN_NEXT_TOKEN;
} /* scanner_primary_to_post_primary_expression */

/**
 * Scan primary expression.
 *
 * @return SCAN_NEXT_TOKEN to read the next token, or SCAN_KEEP_TOKEN to do nothing
 */
static scan_return_types_t
scanner_scan_primary_expression (parser_context_t *context_p, /**< context */
                                 scanner_context_t *scanner_context_p, /* scanner context */
                                 lexer_token_type_t type, /**< current token type */
                                 scan_stack_modes_t stack_top) /**< current stack top */
{
  switch (type)
  {
    case LEXER_KEYW_NEW:
    {
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_AFTER_NEW;

      if (scanner_try_scan_new_target (context_p))
      {
        return scanner_primary_to_post_primary_expression (context_p, scanner_context_p);
      }

      break;
    }
    case LEXER_DIVIDE:
    case LEXER_ASSIGN_DIVIDE:
    {
      lexer_construct_regexp_object (context_p, true);
      return scanner_primary_to_post_primary_expression (context_p, scanner_context_p);
    }
    case LEXER_KEYW_FUNCTION:
    {
      uint16_t status_flags = SCANNER_LITERAL_POOL_FUNCTION;
#if JERRY_MODULE_SYSTEM
      bool is_export_default = stack_top == SCAN_STACK_EXPORT_DEFAULT;
#endif /* JERRY_MODULE_SYSTEM */

      if (scanner_context_p->async_source_p != NULL)
      {
        status_flags |= SCANNER_LITERAL_POOL_ASYNC;
      }

      if (lexer_consume_generator (context_p))
      {
        status_flags |= SCANNER_LITERAL_POOL_GENERATOR;
      }

      scanner_push_literal_pool (context_p, scanner_context_p, status_flags);

      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
      {
#if JERRY_MODULE_SYSTEM
        if (is_export_default)
        {
          lexer_lit_location_t *location_p;
          location_p = scanner_add_custom_literal (context_p,
                                                   scanner_context_p->active_literal_pool_p->prev_p,
                                                   &context_p->token.lit_location);

          scanner_detect_invalid_let (context_p, location_p);
          location_p->type |= SCANNER_LITERAL_IS_FUNC | SCANNER_LITERAL_IS_LET;
        }
#endif /* JERRY_MODULE_SYSTEM */
        lexer_next_token (context_p);
      }
#if JERRY_MODULE_SYSTEM
      else if (is_export_default)
      {
        lexer_lit_location_t *location_p;
        location_p = scanner_add_custom_literal (context_p,
                                                 scanner_context_p->active_literal_pool_p->prev_p,
                                                 &lexer_default_literal);
        location_p->type |= SCANNER_LITERAL_IS_FUNC | SCANNER_LITERAL_IS_LET;
      }
#endif /* JERRY_MODULE_SYSTEM */

      parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_EXPRESSION);
      scanner_context_p->mode = SCAN_MODE_FUNCTION_ARGUMENTS;
      return SCAN_KEEP_TOKEN;
    }
    case LEXER_LEFT_PAREN:
    {
      scanner_scan_bracket (context_p, scanner_context_p);
      return SCAN_KEEP_TOKEN;
    }
    case LEXER_LEFT_SQUARE:
    {
      scanner_push_destructuring_pattern (context_p, scanner_context_p, SCANNER_BINDING_NONE, false);

      parser_stack_push_uint8 (context_p, SCAN_STACK_ARRAY_LITERAL);
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      break;
    }
    case LEXER_LEFT_BRACE:
    {
      scanner_push_destructuring_pattern (context_p, scanner_context_p, SCANNER_BINDING_NONE, false);
      parser_stack_push_uint8 (context_p, 0);

      parser_stack_push_uint8 (context_p, SCAN_STACK_OBJECT_LITERAL);
      scanner_context_p->mode = SCAN_MODE_PROPERTY_NAME;
      return SCAN_KEEP_TOKEN;
    }
    case LEXER_HASHMARK:
    {
      if (!lexer_scan_private_identifier (context_p))
      {
        scanner_raise_error (context_p);
      }

      return SCAN_KEEP_TOKEN;
    }
    case LEXER_TEMPLATE_LITERAL:
    {
      if (context_p->source_p[-1] != LIT_CHAR_GRAVE_ACCENT)
      {
        parser_stack_push_uint8 (context_p, SCAN_STACK_TEMPLATE_STRING);
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        break;
      }

      /* The string is a normal string literal. */
      /* FALLTHRU */
    }
    case LEXER_LITERAL:
    {
      const uint8_t *source_p = context_p->source_p;

      if (context_p->token.lit_location.type == LEXER_IDENT_LITERAL && lexer_check_arrow (context_p))
      {
        scanner_scan_simple_arrow (context_p, scanner_context_p, source_p);
        return SCAN_KEEP_TOKEN;
      }

      if (JERRY_UNLIKELY (lexer_token_is_async (context_p)))
      {
        scanner_context_p->async_source_p = source_p;
        scanner_check_async_function (context_p, scanner_context_p);
        return SCAN_KEEP_TOKEN;
      }

      if (context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
      {
#if JERRY_MODULE_SYSTEM
        if (stack_top == SCAN_STACK_EXPORT_DEFAULT)
        {
          lexer_lit_location_t *location_p = scanner_add_literal (context_p, scanner_context_p);
          location_p->type |= (SCANNER_LITERAL_IS_USED | SCANNER_LITERAL_IS_VAR);
          scanner_detect_eval_call (context_p, scanner_context_p);
          return scanner_primary_to_post_primary_expression (context_p, scanner_context_p);
        }
#endif /* JERRY_MODULE_SYSTEM */

        scanner_add_reference (context_p, scanner_context_p);
      }
      /* FALLTHRU */
    }
    case LEXER_KEYW_THIS:
    case LEXER_LIT_TRUE:
    case LEXER_LIT_FALSE:
    case LEXER_LIT_NULL:
    {
      return scanner_primary_to_post_primary_expression (context_p, scanner_context_p);
    }
    case LEXER_KEYW_SUPER:
    {
      scanner_context_p->active_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_HAS_SUPER_REFERENCE;
      return scanner_primary_to_post_primary_expression (context_p, scanner_context_p);
    }
    case LEXER_KEYW_CLASS:
    {
      scanner_push_class_declaration (context_p, scanner_context_p, SCAN_STACK_CLASS_EXPRESSION);

      if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
      {
        return SCAN_KEEP_TOKEN;
      }
      break;
    }
    case LEXER_RIGHT_SQUARE:
    {
      if (stack_top != SCAN_STACK_ARRAY_LITERAL)
      {
        scanner_raise_error (context_p);
      }

      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
      return SCAN_KEEP_TOKEN;
    }
    case LEXER_THREE_DOTS:
    {
      /* Elision or spread arguments */
      if (stack_top != SCAN_STACK_PAREN_EXPRESSION && stack_top != SCAN_STACK_ARRAY_LITERAL)
      {
        scanner_raise_error (context_p);
      }
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      break;
    }
    case LEXER_COMMA:
    {
      if (stack_top != SCAN_STACK_ARRAY_LITERAL)
      {
        scanner_raise_error (context_p);
      }
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

      if (scanner_context_p->binding_type != SCANNER_BINDING_NONE)
      {
        scanner_context_p->mode = SCAN_MODE_BINDING;
      }

      break;
    }
    case LEXER_KEYW_YIELD:
    {
      lexer_next_token (context_p);

      if (lexer_check_yield_no_arg (context_p))
      {
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
      }

      if (context_p->token.type == LEXER_MULTIPLY)
      {
        return SCAN_NEXT_TOKEN;
      }
      return SCAN_KEEP_TOKEN;
    }
#if JERRY_MODULE_SYSTEM
    case LEXER_KEYW_IMPORT:
    {
      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_DOT)
      {
        scanner_check_import_meta (context_p);
      }
      else if (context_p->token.type != LEXER_LEFT_PAREN)
      {
        scanner_raise_error (context_p);
      }

      scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
      return SCAN_KEEP_TOKEN;
    }
#endif /* JERRY_MODULE_SYSTEM */
    case LEXER_RIGHT_PAREN:
    {
      if (stack_top == SCAN_STACK_PAREN_EXPRESSION)
      {
        parser_stack_pop_uint8 (context_p);

        if (context_p->stack_top_uint8 == SCAN_STACK_USE_ASYNC)
        {
          scanner_add_async_literal (context_p, scanner_context_p);
        }

        return scanner_primary_to_post_primary_expression (context_p, scanner_context_p);
      }
      /* FALLTHRU */
    }
    default:
    {
      scanner_raise_error (context_p);
    }
  }
  return SCAN_NEXT_TOKEN;
} /* scanner_scan_primary_expression */

/**
 * Scan the tokens after the primary expression.
 *
 * @return true for break, false for fall through
 */
static bool
scanner_scan_post_primary_expression (parser_context_t *context_p, /**< context */
                                      scanner_context_t *scanner_context_p, /**< scanner context */
                                      lexer_token_type_t type, /**< current token type */
                                      scan_stack_modes_t stack_top) /**< current stack top */
{
  switch (type)
  {
    case LEXER_DOT:
    {
      lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);

      if (context_p->token.type == LEXER_HASHMARK)
      {
        context_p->token.flags |= LEXER_NO_SKIP_SPACES;
        lexer_next_token (context_p);
      }

      if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
      {
        scanner_raise_error (context_p);
      }

      return true;
    }
    case LEXER_LEFT_PAREN:
    {
      parser_stack_push_uint8 (context_p, SCAN_STACK_PAREN_EXPRESSION);
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      return true;
    }
    case LEXER_TEMPLATE_LITERAL:
    {
      if (JERRY_UNLIKELY (context_p->source_p[-1] != LIT_CHAR_GRAVE_ACCENT))
      {
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        parser_stack_push_uint8 (context_p, SCAN_STACK_TAGGED_TEMPLATE_LITERAL);
      }
      return true;
    }
    case LEXER_LEFT_SQUARE:
    {
      parser_stack_push_uint8 (context_p, SCAN_STACK_PROPERTY_ACCESSOR);
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      return true;
    }
    case LEXER_INCREASE:
    case LEXER_DECREASE:
    {
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;

      if (context_p->token.flags & LEXER_WAS_NEWLINE)
      {
        return false;
      }

      lexer_next_token (context_p);
      type = (lexer_token_type_t) context_p->token.type;

      if (type != LEXER_QUESTION_MARK)
      {
        break;
      }
      /* FALLTHRU */
    }
    case LEXER_QUESTION_MARK:
    {
      parser_stack_push_uint8 (context_p, SCAN_STACK_COLON_EXPRESSION);
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      return true;
    }
    default:
    {
      break;
    }
  }

  if (LEXER_IS_BINARY_OP_TOKEN (type) && (type != LEXER_KEYW_IN || !SCANNER_IS_FOR_START (stack_top)))
  {
    scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
    return true;
  }

  return false;
} /* scanner_scan_post_primary_expression */

/**
 * Scan the tokens after the primary expression.
 *
 * @return SCAN_NEXT_TOKEN to read the next token, or SCAN_KEEP_TOKEN to do nothing
 */
static scan_return_types_t
scanner_scan_primary_expression_end (parser_context_t *context_p, /**< context */
                                     scanner_context_t *scanner_context_p, /**< scanner context */
                                     lexer_token_type_t type, /**< current token type */
                                     scan_stack_modes_t stack_top) /**< current stack top */
{
  if (type == LEXER_COMMA)
  {
    switch (stack_top)
    {
      case SCAN_STACK_VAR:
      case SCAN_STACK_LET:
      case SCAN_STACK_CONST:
      case SCAN_STACK_FOR_VAR_START:
      case SCAN_STACK_FOR_LET_START:
      case SCAN_STACK_FOR_CONST_START:
      {
        scanner_context_p->mode = SCAN_MODE_VAR_STATEMENT;
        return SCAN_NEXT_TOKEN;
      }
      case SCAN_STACK_COLON_EXPRESSION:
      {
        scanner_raise_error (context_p);
        break;
      }
      case SCAN_STACK_BINDING_INIT:
      case SCAN_STACK_BINDING_LIST_INIT:
      {
        break;
      }
      case SCAN_STACK_ARROW_ARGUMENTS:
      {
        lexer_next_token (context_p);
        scanner_check_arrow_arg (context_p, scanner_context_p);
        return SCAN_KEEP_TOKEN;
      }
      case SCAN_STACK_ARROW_EXPRESSION:
      {
        break;
      }
      case SCAN_STACK_CLASS_FIELD_INITIALIZER:
      {
        scanner_raise_error (context_p);
        break;
      }
      case SCAN_STACK_FUNCTION_PARAMETERS:
      {
        scanner_context_p->mode = SCAN_MODE_CONTINUE_FUNCTION_ARGUMENTS;
        parser_stack_pop_uint8 (context_p);
        return SCAN_NEXT_TOKEN;
      }
      case SCAN_STACK_ARRAY_LITERAL:
      {
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

        if (scanner_context_p->binding_type != SCANNER_BINDING_NONE)
        {
          scanner_context_p->mode = SCAN_MODE_BINDING;
        }

        return SCAN_NEXT_TOKEN;
      }
      case SCAN_STACK_OBJECT_LITERAL:
      {
        scanner_context_p->mode = SCAN_MODE_PROPERTY_NAME;
        return SCAN_KEEP_TOKEN;
      }
      default:
      {
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        return SCAN_NEXT_TOKEN;
      }
    }
  }

  switch (stack_top)
  {
    case SCAN_STACK_WITH_EXPRESSION:
    {
      if (type != LEXER_RIGHT_PAREN)
      {
        break;
      }

      parser_stack_pop_uint8 (context_p);

      uint16_t status_flags = scanner_context_p->active_literal_pool_p->status_flags;
      parser_stack_push_uint8 (context_p, (status_flags & SCANNER_LITERAL_POOL_IN_WITH) ? 1 : 0);
      parser_stack_push_uint8 (context_p, SCAN_STACK_WITH_STATEMENT);
      status_flags |= SCANNER_LITERAL_POOL_IN_WITH;
      scanner_context_p->active_literal_pool_p->status_flags = status_flags;

      scanner_context_p->mode = SCAN_MODE_STATEMENT;
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_DO_EXPRESSION:
    {
      if (type != LEXER_RIGHT_PAREN)
      {
        break;
      }

      scanner_context_p->mode = SCAN_MODE_STATEMENT_END;
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_WHILE_EXPRESSION:
    {
      if (type != LEXER_RIGHT_PAREN)
      {
        break;
      }

      scanner_source_start_t source_start;

      parser_stack_pop_uint8 (context_p);
      parser_stack_pop (context_p, &source_start, sizeof (scanner_source_start_t));

      scanner_location_info_t *location_info_p;
      location_info_p = (scanner_location_info_t *) scanner_insert_info (context_p,
                                                                         source_start.source_p,
                                                                         sizeof (scanner_location_info_t));
      location_info_p->info.type = SCANNER_TYPE_WHILE;

      scanner_get_location (&location_info_p->location, context_p);

      scanner_context_p->mode = SCAN_MODE_STATEMENT;
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_PAREN_EXPRESSION:
    {
      if (type != LEXER_RIGHT_PAREN)
      {
        break;
      }

      parser_stack_pop_uint8 (context_p);

      if (context_p->stack_top_uint8 == SCAN_STACK_USE_ASYNC)
      {
        scanner_add_async_literal (context_p, scanner_context_p);
      }

      scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_STATEMENT_WITH_EXPR:
    {
      if (type != LEXER_RIGHT_PAREN)
      {
        break;
      }

      parser_stack_pop_uint8 (context_p);

      if (context_p->stack_top_uint8 == SCAN_STACK_IF_STATEMENT)
      {
        scanner_check_function_after_if (context_p, scanner_context_p);
        return SCAN_KEEP_TOKEN;
      }

      scanner_context_p->mode = SCAN_MODE_STATEMENT;
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_BINDING_LIST_INIT:
    {
      parser_stack_pop_uint8 (context_p);

      JERRY_ASSERT (context_p->stack_top_uint8 == SCAN_STACK_ARRAY_LITERAL
                    || context_p->stack_top_uint8 == SCAN_STACK_OBJECT_LITERAL
                    || context_p->stack_top_uint8 == SCAN_STACK_LET || context_p->stack_top_uint8 == SCAN_STACK_CONST
                    || context_p->stack_top_uint8 == SCAN_STACK_FOR_LET_START
                    || context_p->stack_top_uint8 == SCAN_STACK_FOR_CONST_START
                    || context_p->stack_top_uint8 == SCAN_STACK_FUNCTION_PARAMETERS
                    || context_p->stack_top_uint8 == SCAN_STACK_ARROW_ARGUMENTS);

      scanner_binding_item_t *item_p = scanner_context_p->active_binding_list_p->items_p;

      while (item_p != NULL)
      {
        if (item_p->literal_p->type & SCANNER_LITERAL_IS_USED)
        {
          item_p->literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
        }
        item_p = item_p->next_p;
      }

      scanner_pop_binding_list (scanner_context_p);
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_BINDING_INIT:
    {
      scanner_binding_literal_t binding_literal;

      parser_stack_pop_uint8 (context_p);
      parser_stack_pop (context_p, &binding_literal, sizeof (scanner_binding_literal_t));

      JERRY_ASSERT (context_p->stack_top_uint8 == SCAN_STACK_ARRAY_LITERAL
                    || context_p->stack_top_uint8 == SCAN_STACK_OBJECT_LITERAL
                    || context_p->stack_top_uint8 == SCAN_STACK_LET || context_p->stack_top_uint8 == SCAN_STACK_CONST
                    || context_p->stack_top_uint8 == SCAN_STACK_FOR_LET_START
                    || context_p->stack_top_uint8 == SCAN_STACK_FOR_CONST_START
                    || context_p->stack_top_uint8 == SCAN_STACK_FUNCTION_PARAMETERS
                    || context_p->stack_top_uint8 == SCAN_STACK_ARROW_ARGUMENTS);

      JERRY_ASSERT (SCANNER_NEEDS_BINDING_LIST (scanner_context_p->binding_type)
                    || (stack_top != SCAN_STACK_ARRAY_LITERAL && stack_top != SCAN_STACK_OBJECT_LITERAL));

      if (binding_literal.literal_p->type & SCANNER_LITERAL_IS_USED)
      {
        binding_literal.literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
      }

      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_VAR:
    case SCAN_STACK_LET:
    case SCAN_STACK_CONST:
    {
#if JERRY_MODULE_SYSTEM
      scanner_context_p->active_literal_pool_p->status_flags &= (uint16_t) ~SCANNER_LITERAL_POOL_IN_EXPORT;
#endif /* JERRY_MODULE_SYSTEM */

      parser_stack_pop_uint8 (context_p);
      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_FOR_VAR_START:
    case SCAN_STACK_FOR_LET_START:
    case SCAN_STACK_FOR_CONST_START:
    case SCAN_STACK_FOR_START:
    {
      if (type == LEXER_KEYW_IN || SCANNER_IDENTIFIER_IS_OF ())
      {
        scanner_for_statement_t for_statement;

        parser_stack_pop_uint8 (context_p);
        parser_stack_pop (context_p, &for_statement, sizeof (scanner_for_statement_t));

        scanner_location_info_t *location_info;
        location_info = (scanner_location_info_t *) scanner_insert_info (context_p,
                                                                         for_statement.u.source_p,
                                                                         sizeof (scanner_location_info_t));
        location_info->info.type = (type == LEXER_KEYW_IN) ? SCANNER_TYPE_FOR_IN : SCANNER_TYPE_FOR_OF;

        if (stack_top == SCAN_STACK_FOR_LET_START || stack_top == SCAN_STACK_FOR_CONST_START)
        {
          parser_stack_push_uint8 (context_p, SCAN_STACK_PRIVATE_BLOCK_EARLY);
        }
        scanner_get_location (&location_info->location, context_p);

        parser_stack_push_uint8 (context_p, SCAN_STACK_STATEMENT_WITH_EXPR);
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        return SCAN_NEXT_TOKEN;
      }

      if (type != LEXER_SEMICOLON)
      {
        break;
      }

      scanner_for_statement_t for_statement;

      parser_stack_pop_uint8 (context_p);
      parser_stack_pop (context_p, NULL, sizeof (scanner_for_statement_t));

      if (stack_top == SCAN_STACK_FOR_LET_START || stack_top == SCAN_STACK_FOR_CONST_START)
      {
        parser_stack_push_uint8 (context_p, SCAN_STACK_PRIVATE_BLOCK);
      }

      for_statement.u.source_p = context_p->source_p;
      parser_stack_push (context_p, &for_statement, sizeof (scanner_for_statement_t));
      parser_stack_push_uint8 (context_p, SCAN_STACK_FOR_CONDITION);

      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_SEMICOLON)
      {
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        return SCAN_KEEP_TOKEN;
      }

      type = LEXER_SEMICOLON;
      /* FALLTHRU */
    }
    case SCAN_STACK_FOR_CONDITION:
    {
      if (type != LEXER_SEMICOLON)
      {
        break;
      }

      scanner_for_statement_t for_statement;

      parser_stack_pop_uint8 (context_p);
      parser_stack_pop (context_p, &for_statement, sizeof (scanner_for_statement_t));

      scanner_for_info_t *for_info_p;
      for_info_p =
        (scanner_for_info_t *) scanner_insert_info (context_p, for_statement.u.source_p, sizeof (scanner_for_info_t));
      for_info_p->info.type = SCANNER_TYPE_FOR;

      scanner_get_location (&for_info_p->expression_location, context_p);
      for_info_p->end_location.source_p = NULL;

      for_statement.u.for_info_p = for_info_p;

      parser_stack_push (context_p, &for_statement, sizeof (scanner_for_statement_t));
      parser_stack_push_uint8 (context_p, SCAN_STACK_FOR_EXPRESSION);

      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_RIGHT_PAREN)
      {
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        return SCAN_KEEP_TOKEN;
      }

      type = LEXER_RIGHT_PAREN;
      /* FALLTHRU */
    }
    case SCAN_STACK_FOR_EXPRESSION:
    {
      if (type != LEXER_RIGHT_PAREN)
      {
        break;
      }

      scanner_for_statement_t for_statement;

      parser_stack_pop_uint8 (context_p);
      parser_stack_pop (context_p, &for_statement, sizeof (scanner_for_statement_t));

      scanner_get_location (&for_statement.u.for_info_p->end_location, context_p);

      scanner_context_p->mode = SCAN_MODE_STATEMENT;
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_SWITCH_EXPRESSION:
    {
      if (type != LEXER_RIGHT_PAREN)
      {
        break;
      }

      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_LEFT_BRACE)
      {
        break;
      }

      scanner_literal_pool_t *literal_pool_p;
      literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, 0);
      literal_pool_p->source_p = context_p->source_p - 1;

      parser_stack_pop_uint8 (context_p);

      scanner_switch_statement_t switch_statement = scanner_context_p->active_switch_statement;
      parser_stack_push (context_p, &switch_statement, sizeof (scanner_switch_statement_t));
      parser_stack_push_uint8 (context_p, SCAN_STACK_SWITCH_BLOCK);

      scanner_switch_info_t *switch_info_p;
      switch_info_p =
        (scanner_switch_info_t *) scanner_insert_info (context_p, context_p->source_p, sizeof (scanner_switch_info_t));
      switch_info_p->info.type = SCANNER_TYPE_SWITCH;
      switch_info_p->case_p = NULL;
      scanner_context_p->active_switch_statement.last_case_p = &switch_info_p->case_p;

      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_RIGHT_BRACE && context_p->token.type != LEXER_KEYW_CASE
          && context_p->token.type != LEXER_KEYW_DEFAULT)
      {
        break;
      }

      scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_CASE_STATEMENT:
    {
      if (type != LEXER_COLON)
      {
        break;
      }

      scanner_source_start_t source_start;

      parser_stack_pop_uint8 (context_p);
      parser_stack_pop (context_p, &source_start, sizeof (scanner_source_start_t));

      scanner_location_info_t *location_info_p;
      location_info_p = (scanner_location_info_t *) scanner_insert_info (context_p,
                                                                         source_start.source_p,
                                                                         sizeof (scanner_location_info_t));
      location_info_p->info.type = SCANNER_TYPE_CASE;

      scanner_get_location (&location_info_p->location, context_p);

      scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_COLON_EXPRESSION:
    {
      if (type != LEXER_COLON)
      {
        break;
      }

      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      parser_stack_pop_uint8 (context_p);
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_ARRAY_LITERAL:
    case SCAN_STACK_OBJECT_LITERAL:
    {
      if ((stack_top == SCAN_STACK_ARRAY_LITERAL && type != LEXER_RIGHT_SQUARE)
          || (stack_top == SCAN_STACK_OBJECT_LITERAL && type != LEXER_RIGHT_BRACE))
      {
        break;
      }

      scanner_source_start_t source_start;
      uint8_t binding_type = scanner_context_p->binding_type;
      uint8_t object_literal_flags = 0;

      parser_stack_pop_uint8 (context_p);

      if (stack_top == SCAN_STACK_OBJECT_LITERAL)
      {
        object_literal_flags = context_p->stack_top_uint8;
        parser_stack_pop_uint8 (context_p);
      }

      scanner_context_p->binding_type = context_p->stack_top_uint8;
      parser_stack_pop_uint8 (context_p);
      parser_stack_pop (context_p, &source_start, sizeof (scanner_source_start_t));

      lexer_next_token (context_p);

      stack_top = (scan_stack_modes_t) context_p->stack_top_uint8;

      if (binding_type == SCANNER_BINDING_CATCH && stack_top == SCAN_STACK_CATCH_STATEMENT)
      {
        scanner_pop_binding_list (scanner_context_p);

        if (object_literal_flags != 0)
        {
          scanner_info_t *info_p = scanner_insert_info (context_p, source_start.source_p, sizeof (scanner_info_t));
          info_p->type = SCANNER_TYPE_LITERAL_FLAGS;
          info_p->u8_arg = object_literal_flags;
        }

        if (context_p->token.type != LEXER_RIGHT_PAREN)
        {
          scanner_raise_error (context_p);
        }

        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_LEFT_BRACE)
        {
          scanner_raise_error (context_p);
        }

        scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
        return SCAN_NEXT_TOKEN;
      }

      if (stack_top == SCAN_STACK_FOR_START_PATTERN)
      {
        JERRY_ASSERT (binding_type == SCANNER_BINDING_NONE);

        parser_stack_change_last_uint8 (context_p, SCAN_STACK_FOR_START);

        if (context_p->token.type == LEXER_KEYW_IN || SCANNER_IDENTIFIER_IS_OF ())
        {
          scanner_info_t *info_p = scanner_insert_info (context_p, source_start.source_p, sizeof (scanner_info_t));
          info_p->type = SCANNER_TYPE_LITERAL_FLAGS;
          info_p->u8_arg = object_literal_flags | SCANNER_LITERAL_DESTRUCTURING_FOR;
          return SCAN_KEEP_TOKEN;
        }
      }

      if (context_p->token.type != LEXER_ASSIGN)
      {
        if (SCANNER_NEEDS_BINDING_LIST (binding_type))
        {
          scanner_pop_binding_list (scanner_context_p);
        }

        if ((stack_top == SCAN_STACK_ARRAY_LITERAL || stack_top == SCAN_STACK_OBJECT_LITERAL)
            && (binding_type == SCANNER_BINDING_NONE || binding_type == SCANNER_BINDING_ARROW_ARG)
            && context_p->token.type != LEXER_EOS && context_p->token.type != LEXER_COMMA
            && context_p->token.type != LEXER_RIGHT_BRACE && context_p->token.type != LEXER_RIGHT_SQUARE)
        {
          object_literal_flags |= SCANNER_LITERAL_NO_DESTRUCTURING;
        }

        if (object_literal_flags != 0)
        {
          scanner_info_t *info_p = scanner_insert_info (context_p, source_start.source_p, sizeof (scanner_info_t));
          info_p->type = SCANNER_TYPE_LITERAL_FLAGS;
          info_p->u8_arg = object_literal_flags;
        }

        scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
        return SCAN_KEEP_TOKEN;
      }

      scanner_location_info_t *location_info_p;
      location_info_p = (scanner_location_info_t *) scanner_insert_info (context_p,
                                                                         source_start.source_p,
                                                                         sizeof (scanner_location_info_t));
      location_info_p->info.type = SCANNER_TYPE_INITIALIZER;
      location_info_p->info.u8_arg = object_literal_flags;
      scanner_get_location (&location_info_p->location, context_p);
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

      if (SCANNER_NEEDS_BINDING_LIST (binding_type))
      {
        scanner_binding_item_t *item_p = scanner_context_p->active_binding_list_p->items_p;

        while (item_p != NULL)
        {
          item_p->literal_p->type &= (uint8_t) ~SCANNER_LITERAL_IS_USED;
          item_p = item_p->next_p;
        }

        parser_stack_push_uint8 (context_p, SCAN_STACK_BINDING_LIST_INIT);
      }
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_PROPERTY_ACCESSOR:
    {
      if (type != LEXER_RIGHT_SQUARE)
      {
        break;
      }

      scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
      parser_stack_pop_uint8 (context_p);
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_COMPUTED_PROPERTY:
    {
      if (type != LEXER_RIGHT_SQUARE)
      {
        break;
      }

      lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);

      parser_stack_pop_uint8 (context_p);
      stack_top = (scan_stack_modes_t) context_p->stack_top_uint8;

      if (stack_top == SCAN_STACK_FUNCTION_PROPERTY)
      {
        scanner_push_literal_pool (context_p, scanner_context_p, SCANNER_LITERAL_POOL_FUNCTION);
        scanner_context_p->mode = SCAN_MODE_FUNCTION_ARGUMENTS;
        return SCAN_KEEP_TOKEN;
      }

      if (stack_top == SCAN_STACK_EXPLICIT_CLASS_CONSTRUCTOR || stack_top == SCAN_STACK_IMPLICIT_CLASS_CONSTRUCTOR)
      {
        JERRY_ASSERT (scanner_context_p->active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_CLASS_NAME);

        if (context_p->token.type == LEXER_LEFT_PAREN)
        {
          scanner_push_literal_pool (context_p, scanner_context_p, SCANNER_LITERAL_POOL_FUNCTION);

          parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PROPERTY);
          scanner_context_p->mode = SCAN_MODE_FUNCTION_ARGUMENTS;
          return SCAN_KEEP_TOKEN;
        }

        if (context_p->token.type == LEXER_ASSIGN)
        {
          scanner_push_class_field_initializer (context_p, scanner_context_p);
          return SCAN_NEXT_TOKEN;
        }

        scanner_context_p->mode =
          (context_p->token.type != LEXER_SEMICOLON ? SCAN_MODE_CLASS_BODY_NO_SCAN : SCAN_MODE_CLASS_BODY);
        return SCAN_KEEP_TOKEN;
      }

      JERRY_ASSERT (stack_top == SCAN_STACK_OBJECT_LITERAL);

      if (context_p->token.type == LEXER_LEFT_PAREN)
      {
        scanner_push_literal_pool (context_p, scanner_context_p, SCANNER_LITERAL_POOL_FUNCTION);

        parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PROPERTY);
        scanner_context_p->mode = SCAN_MODE_FUNCTION_ARGUMENTS;
        return SCAN_KEEP_TOKEN;
      }

      if (context_p->token.type != LEXER_COLON)
      {
        scanner_raise_error (context_p);
      }

      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

      if (scanner_context_p->binding_type != SCANNER_BINDING_NONE)
      {
        scanner_context_p->mode = SCAN_MODE_BINDING;
      }
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_COMPUTED_GENERATOR:
    case SCAN_STACK_COMPUTED_ASYNC:
    case SCAN_STACK_COMPUTED_ASYNC_GENERATOR:
    {
      if (type != LEXER_RIGHT_SQUARE)
      {
        break;
      }

      lexer_next_token (context_p);
      parser_stack_pop_uint8 (context_p);

      JERRY_ASSERT (context_p->stack_top_uint8 == SCAN_STACK_OBJECT_LITERAL
                    || context_p->stack_top_uint8 == SCAN_STACK_FUNCTION_PROPERTY);

      uint16_t status_flags = (uint16_t) (SCANNER_LITERAL_POOL_FUNCTION | SCANNER_LITERAL_POOL_GENERATOR
                                          | SCANNER_FROM_COMPUTED_TO_LITERAL_POOL (stack_top));

      scanner_push_literal_pool (context_p, scanner_context_p, status_flags);

      scanner_context_p->mode = SCAN_MODE_FUNCTION_ARGUMENTS;
      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_TEMPLATE_STRING:
    case SCAN_STACK_TAGGED_TEMPLATE_LITERAL:
    {
      if (type != LEXER_RIGHT_BRACE)
      {
        break;
      }

      context_p->source_p--;
      context_p->column--;
      lexer_parse_string (context_p, LEXER_STRING_NO_OPTS);

      if (context_p->source_p[-1] != LIT_CHAR_GRAVE_ACCENT)
      {
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      }
      else
      {
        parser_stack_pop_uint8 (context_p);
        scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
      }
      return SCAN_NEXT_TOKEN;
    }
    case SCAN_STACK_ARROW_ARGUMENTS:
    {
      if (type != LEXER_RIGHT_PAREN)
      {
        break;
      }

      scanner_check_arrow (context_p, scanner_context_p);
      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_ARROW_EXPRESSION:
    {
      scanner_pop_literal_pool (context_p, scanner_context_p);
      parser_stack_pop_uint8 (context_p);
      lexer_update_await_yield (context_p, context_p->status_flags);
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_CLASS_EXTENDS:
    {
      if (type != LEXER_LEFT_BRACE)
      {
        break;
      }

      scanner_context_p->mode = SCAN_MODE_CLASS_BODY;
      parser_stack_pop_uint8 (context_p);

      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_CLASS_FIELD_INITIALIZER:
    {
      scanner_source_start_t source_start;
      const uint8_t *source_p = NULL;

      parser_stack_pop_uint8 (context_p);
      parser_stack_pop (context_p, &source_start, sizeof (scanner_source_start_t));
      scanner_pop_literal_pool (context_p, scanner_context_p);
      scanner_context_p->mode = SCAN_MODE_CLASS_BODY_NO_SCAN;

      switch (type)
      {
        case LEXER_SEMICOLON:
        {
          source_p = context_p->source_p - 1;
          scanner_context_p->mode = SCAN_MODE_CLASS_BODY;
          break;
        }
        case LEXER_RIGHT_BRACE:
        {
          source_p = context_p->source_p - 1;
          break;
        }
        default:
        {
          if (!(context_p->token.flags & LEXER_WAS_NEWLINE))
          {
            break;
          }

          if (type == LEXER_LEFT_SQUARE)
          {
            source_p = context_p->source_p - 1;
            break;
          }

          if (type == LEXER_LITERAL)
          {
            if (context_p->token.lit_location.type == LEXER_IDENT_LITERAL
                || context_p->token.lit_location.type == LEXER_NUMBER_LITERAL)
            {
              source_p = context_p->token.lit_location.char_p;
            }
            else if (context_p->token.lit_location.type == LEXER_STRING_LITERAL)
            {
              source_p = context_p->token.lit_location.char_p - 1;
            }
            break;
          }

          if (type == context_p->token.keyword_type && type != LEXER_EOS)
          {
            /* Convert keyword to literal. */
            source_p = context_p->token.lit_location.char_p;
            context_p->token.type = LEXER_LITERAL;
          }
          break;
        }
      }

      if (JERRY_UNLIKELY (source_p == NULL))
      {
        scanner_raise_error (context_p);
      }

      scanner_location_info_t *location_info_p;
      location_info_p = (scanner_location_info_t *) scanner_insert_info (context_p,
                                                                         source_start.source_p,
                                                                         sizeof (scanner_location_info_t));
      location_info_p->info.type = SCANNER_TYPE_CLASS_FIELD_INITIALIZER_END;
      location_info_p->location.source_p = source_p;
      location_info_p->location.line = context_p->token.line;
      location_info_p->location.column = context_p->token.column;
      return SCAN_KEEP_TOKEN;
    }
    case SCAN_STACK_FUNCTION_PARAMETERS:
    {
      parser_stack_pop_uint8 (context_p);

      if (type != LEXER_RIGHT_PAREN && (type != LEXER_EOS || context_p->stack_top_uint8 != SCAN_STACK_SCRIPT_FUNCTION))
      {
        break;
      }

      scanner_context_p->mode = SCAN_MODE_CONTINUE_FUNCTION_ARGUMENTS;
      return SCAN_KEEP_TOKEN;
    }
    default:
    {
      scanner_context_p->mode = SCAN_MODE_STATEMENT_END;
      return SCAN_KEEP_TOKEN;
    }
  }

  scanner_raise_error (context_p);
  return SCAN_NEXT_TOKEN;
} /* scanner_scan_primary_expression_end */

/**
 * Scan statements.
 *
 * @return SCAN_NEXT_TOKEN to read the next token, or SCAN_KEEP_TOKEN to do nothing
 */
static scan_return_types_t
scanner_scan_statement (parser_context_t *context_p, /**< context */
                        scanner_context_t *scanner_context_p, /**< scanner context */
                        lexer_token_type_t type, /**< current token type */
                        scan_stack_modes_t stack_top) /**< current stack top */
{
  switch (type)
  {
    case LEXER_SEMICOLON:
    {
      scanner_context_p->mode = SCAN_MODE_STATEMENT_END;
      return SCAN_KEEP_TOKEN;
    }
    case LEXER_LEFT_BRACE:
    {
      scanner_literal_pool_t *literal_pool_p;
      literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, 0);
      literal_pool_p->source_p = context_p->source_p;

      scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
      parser_stack_push_uint8 (context_p, SCAN_STACK_BLOCK_STATEMENT);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_DO:
    {
      scanner_context_p->mode = SCAN_MODE_STATEMENT;
      parser_stack_push_uint8 (context_p, SCAN_STACK_DO_STATEMENT);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_TRY:
    {
      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_LEFT_BRACE)
      {
        scanner_raise_error (context_p);
      }

      scanner_literal_pool_t *literal_pool_p;
      literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, 0);
      literal_pool_p->source_p = context_p->source_p;

      scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
      parser_stack_push_uint8 (context_p, SCAN_STACK_TRY_STATEMENT);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_DEBUGGER:
    {
      scanner_context_p->mode = SCAN_MODE_STATEMENT_END;
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_IF:
    case LEXER_KEYW_WITH:
    case LEXER_KEYW_SWITCH:
    {
      lexer_next_token (context_p);
      if (context_p->token.type != LEXER_LEFT_PAREN)
      {
        scanner_raise_error (context_p);
      }

      uint8_t mode = SCAN_STACK_STATEMENT_WITH_EXPR;

      if (type == LEXER_KEYW_IF)
      {
        parser_stack_push_uint8 (context_p, SCAN_STACK_IF_STATEMENT);
      }
      else if (type == LEXER_KEYW_WITH)
      {
        mode = SCAN_STACK_WITH_EXPRESSION;
      }
      else if (type == LEXER_KEYW_SWITCH)
      {
        mode = SCAN_STACK_SWITCH_EXPRESSION;
      }

      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      parser_stack_push_uint8 (context_p, mode);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_WHILE:
    {
      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_LEFT_PAREN)
      {
        scanner_raise_error (context_p);
      }

      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

      scanner_source_start_t source_start;
      source_start.source_p = context_p->source_p;

      parser_stack_push (context_p, &source_start, sizeof (scanner_source_start_t));
      parser_stack_push_uint8 (context_p, SCAN_STACK_WHILE_EXPRESSION);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_FOR:
    {
      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_KEYW_AWAIT)
      {
        lexer_next_token (context_p);
      }

      if (context_p->token.type != LEXER_LEFT_PAREN)
      {
        scanner_raise_error (context_p);
      }

      scanner_for_statement_t for_statement;
      for_statement.u.source_p = context_p->source_p;
      uint8_t stack_mode = SCAN_STACK_FOR_START;
      scan_return_types_t return_type = SCAN_KEEP_TOKEN;

      lexer_next_token (context_p);
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

      const uint8_t *source_p = context_p->source_p;

      switch (context_p->token.type)
      {
        case LEXER_SEMICOLON:
        {
          scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
          break;
        }
        case LEXER_KEYW_VAR:
        {
          scanner_context_p->mode = SCAN_MODE_VAR_STATEMENT;
          stack_mode = SCAN_STACK_FOR_VAR_START;
          return_type = SCAN_NEXT_TOKEN;
          break;
        }
        case LEXER_LEFT_BRACE:
        case LEXER_LEFT_SQUARE:
        {
          stack_mode = SCAN_STACK_FOR_START_PATTERN;
          break;
        }
        case LEXER_LITERAL:
        {
          if (!lexer_token_is_let (context_p))
          {
            break;
          }

          parser_line_counter_t line = context_p->line;
          parser_line_counter_t column = context_p->column;

          if (lexer_check_arrow (context_p))
          {
            context_p->source_p = source_p;
            context_p->line = line;
            context_p->column = column;
            context_p->token.flags &= (uint8_t) ~LEXER_NO_SKIP_SPACES;
            break;
          }

          lexer_next_token (context_p);

          type = (lexer_token_type_t) context_p->token.type;

          if (type != LEXER_LEFT_SQUARE && type != LEXER_LEFT_BRACE
              && (type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL))
          {
            scanner_info_t *info_p = scanner_insert_info (context_p, source_p, sizeof (scanner_info_t));
            info_p->type = SCANNER_TYPE_LET_EXPRESSION;

            scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
            break;
          }

          scanner_context_p->mode = SCAN_MODE_VAR_STATEMENT;
          /* FALLTHRU */
        }
        case LEXER_KEYW_LET:
        case LEXER_KEYW_CONST:
        {
          scanner_literal_pool_t *literal_pool_p;
          literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, 0);
          literal_pool_p->source_p = source_p;

          if (scanner_context_p->mode == SCAN_MODE_PRIMARY_EXPRESSION)
          {
            scanner_context_p->mode = SCAN_MODE_VAR_STATEMENT;
            return_type = SCAN_NEXT_TOKEN;
          }

          stack_mode =
            ((context_p->token.type == LEXER_KEYW_CONST) ? SCAN_STACK_FOR_CONST_START : SCAN_STACK_FOR_LET_START);
          break;
        }
      }

      parser_stack_push (context_p, &for_statement, sizeof (scanner_for_statement_t));
      parser_stack_push_uint8 (context_p, stack_mode);
      return return_type;
    }
    case LEXER_KEYW_VAR:
    {
      scanner_context_p->mode = SCAN_MODE_VAR_STATEMENT;
      parser_stack_push_uint8 (context_p, SCAN_STACK_VAR);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_LET:
    {
      scanner_context_p->mode = SCAN_MODE_VAR_STATEMENT;
      parser_stack_push_uint8 (context_p, SCAN_STACK_LET);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_CONST:
    {
      scanner_context_p->mode = SCAN_MODE_VAR_STATEMENT;
      parser_stack_push_uint8 (context_p, SCAN_STACK_CONST);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_THROW:
    {
      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_RETURN:
    {
      lexer_next_token (context_p);

      if (!(context_p->token.flags & LEXER_WAS_NEWLINE) && context_p->token.type != LEXER_SEMICOLON
          && context_p->token.type != LEXER_EOS && context_p->token.type != LEXER_RIGHT_BRACE)
      {
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        return SCAN_KEEP_TOKEN;
      }

      scanner_context_p->mode = SCAN_MODE_STATEMENT_END;
      return SCAN_KEEP_TOKEN;
    }
    case LEXER_KEYW_BREAK:
    case LEXER_KEYW_CONTINUE:
    {
      lexer_next_token (context_p);
      scanner_context_p->mode = SCAN_MODE_STATEMENT_END;

      if (!(context_p->token.flags & LEXER_WAS_NEWLINE) && context_p->token.type == LEXER_LITERAL
          && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
      {
        return SCAN_NEXT_TOKEN;
      }
      return SCAN_KEEP_TOKEN;
    }
    case LEXER_KEYW_CASE:
    case LEXER_KEYW_DEFAULT:
    {
      if (stack_top != SCAN_STACK_SWITCH_BLOCK)
      {
        scanner_raise_error (context_p);
      }

      scanner_case_info_t *case_info_p;
      case_info_p = (scanner_case_info_t *) scanner_malloc (context_p, sizeof (scanner_case_info_t));

      *(scanner_context_p->active_switch_statement.last_case_p) = case_info_p;
      scanner_context_p->active_switch_statement.last_case_p = &case_info_p->next_p;

      case_info_p->next_p = NULL;
      scanner_get_location (&case_info_p->location, context_p);

      if (type == LEXER_KEYW_DEFAULT)
      {
        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_COLON)
        {
          scanner_raise_error (context_p);
        }

        scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
        return SCAN_NEXT_TOKEN;
      }

      scanner_source_start_t source_start;
      source_start.source_p = context_p->source_p;

      parser_stack_push (context_p, &source_start, sizeof (scanner_source_start_t));
      parser_stack_push_uint8 (context_p, SCAN_STACK_CASE_STATEMENT);

      scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_FUNCTION:
    {
      uint16_t status_flags = SCANNER_LITERAL_POOL_FUNCTION | SCANNER_LITERAL_POOL_FUNCTION_STATEMENT;

      if (scanner_context_p->async_source_p != NULL)
      {
        scanner_context_p->status_flags |= SCANNER_CONTEXT_THROW_ERR_ASYNC_FUNCTION;
        status_flags |= SCANNER_LITERAL_POOL_ASYNC;
      }

      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_MULTIPLY)
      {
        status_flags |= SCANNER_LITERAL_POOL_GENERATOR;
        lexer_next_token (context_p);
      }

      if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
      {
        scanner_raise_error (context_p);
      }

      lexer_lit_location_t *literal_p = scanner_add_literal (context_p, scanner_context_p);

      const uint8_t mask = (SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_FUNC | SCANNER_LITERAL_IS_LOCAL);

      if ((literal_p->type & SCANNER_LITERAL_IS_LOCAL)
          && (literal_p->type & mask) != (SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_DESTRUCTURED_ARG)
          && (literal_p->type & mask) != SCANNER_LITERAL_IS_LOCAL_FUNC)
      {
        scanner_raise_redeclaration_error (context_p);
      }

      scanner_literal_pool_t *literal_pool_p = scanner_context_p->active_literal_pool_p;

      if (!(literal_pool_p->status_flags & SCANNER_LITERAL_POOL_FUNCTION)
          && (literal_p->type & (SCANNER_LITERAL_IS_VAR)))
      {
        scanner_raise_redeclaration_error (context_p);
      }

      literal_p->type |= SCANNER_LITERAL_IS_LOCAL_FUNC;

      scanner_context_p->status_flags &= (uint16_t) ~SCANNER_CONTEXT_THROW_ERR_ASYNC_FUNCTION;

      scanner_push_literal_pool (context_p, scanner_context_p, status_flags);

      scanner_context_p->mode = SCAN_MODE_FUNCTION_ARGUMENTS;
      parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_STATEMENT);
      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_CLASS:
    {
      lexer_lit_location_t *literal_p;
      literal_p = scanner_push_class_declaration (context_p, scanner_context_p, SCAN_STACK_CLASS_STATEMENT);

      if (literal_p == NULL)
      {
        scanner_raise_error (context_p);
      }

      scanner_detect_invalid_let (context_p, literal_p);
      literal_p->type |= SCANNER_LITERAL_IS_LET;

      if (literal_p->type & SCANNER_LITERAL_IS_USED)
      {
        literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
      }

#if JERRY_MODULE_SYSTEM
      if (scanner_context_p->active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_EXPORT)
      {
        literal_p->type |= SCANNER_LITERAL_NO_REG;
        scanner_context_p->active_literal_pool_p->status_flags &= (uint16_t) ~SCANNER_LITERAL_POOL_IN_EXPORT;
      }
#endif /* JERRY_MODULE_SYSTEM */

      return SCAN_NEXT_TOKEN;
    }
#if JERRY_MODULE_SYSTEM
    case LEXER_KEYW_IMPORT:
    {
      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_DOT)
      {
        scanner_check_import_meta (context_p);
        scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
        return SCAN_KEEP_TOKEN;
      }

      if (context_p->token.type == LEXER_LEFT_PAREN)
      {
        scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
        return SCAN_KEEP_TOKEN;
      }

      if (stack_top != SCAN_STACK_SCRIPT)
      {
        scanner_raise_error (context_p);
      }

      scanner_context_p->mode = SCAN_MODE_STATEMENT_END;

      if (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_STRING_LITERAL)
      {
        return SCAN_NEXT_TOKEN;
      }

      bool parse_imports = true;

      if (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
      {
        lexer_lit_location_t *literal_p = scanner_add_literal (context_p, scanner_context_p);

        scanner_detect_invalid_let (context_p, literal_p);
        literal_p->type |= SCANNER_LITERAL_IS_LOCAL | SCANNER_LITERAL_NO_REG;

        lexer_next_token (context_p);

        if (context_p->token.type == LEXER_COMMA)
        {
          lexer_next_token (context_p);
        }
        else
        {
          parse_imports = false;
        }
      }

      if (parse_imports)
      {
        if (context_p->token.type == LEXER_MULTIPLY)
        {
          lexer_next_token (context_p);
          if (!lexer_token_is_identifier (context_p, "as", 2))
          {
            scanner_raise_error (context_p);
          }

          lexer_next_token (context_p);

          if (context_p->token.type != LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
          {
            scanner_raise_error (context_p);
          }

          lexer_lit_location_t *literal_p = scanner_add_literal (context_p, scanner_context_p);

          scanner_detect_invalid_let (context_p, literal_p);
          literal_p->type |= SCANNER_LITERAL_IS_LOCAL | SCANNER_LITERAL_NO_REG;

          lexer_next_token (context_p);
        }
        else if (context_p->token.type == LEXER_LEFT_BRACE)
        {
          lexer_next_token (context_p);

          while (context_p->token.type != LEXER_RIGHT_BRACE)
          {
            if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
            {
              scanner_raise_error (context_p);
            }

            const uint8_t *source_p = context_p->source_p;

            if (lexer_check_next_character (context_p, LIT_CHAR_LOWERCASE_A))
            {
              lexer_next_token (context_p);

              if (!lexer_token_is_identifier (context_p, "as", 2))
              {
                scanner_raise_error (context_p);
              }

              lexer_next_token (context_p);

              if (context_p->token.type != LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
              {
                scanner_raise_error (context_p);
              }

              source_p = context_p->source_p;
            }

            lexer_lit_location_t *literal_p = scanner_add_literal (context_p, scanner_context_p);

            if (literal_p->type & (SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_VAR | SCANNER_LITERAL_IS_LOCAL))
            {
              context_p->source_p = source_p;
              scanner_raise_redeclaration_error (context_p);
            }

            if (literal_p->type & SCANNER_LITERAL_IS_FUNC)
            {
              literal_p->type &= (uint8_t) ~SCANNER_LITERAL_IS_FUNC;
            }

            literal_p->type |= SCANNER_LITERAL_IS_LOCAL | SCANNER_LITERAL_NO_REG;

            lexer_next_token (context_p);

            if (context_p->token.type != LEXER_RIGHT_BRACE)
            {
              if (context_p->token.type != LEXER_COMMA)
              {
                scanner_raise_error (context_p);
              }

              lexer_next_token (context_p);
            }
          }

          lexer_next_token (context_p);
        }
        else
        {
          scanner_raise_error (context_p);
        }
      }

      if (!lexer_token_is_identifier (context_p, "from", 4))
      {
        scanner_raise_error (context_p);
      }

      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_LITERAL && context_p->token.lit_location.type != LEXER_STRING_LITERAL)
      {
        scanner_raise_error (context_p);
      }

      return SCAN_NEXT_TOKEN;
    }
    case LEXER_KEYW_EXPORT:
    {
      if (stack_top != SCAN_STACK_SCRIPT)
      {
        scanner_raise_error (context_p);
      }

      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_KEYW_DEFAULT)
      {
        lexer_next_token (context_p);
        parser_stack_push_uint8 (context_p, SCAN_STACK_EXPORT_DEFAULT);
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        return SCAN_KEEP_TOKEN;
      }

      scanner_context_p->mode = SCAN_MODE_STATEMENT_END;

      if (context_p->token.type == LEXER_MULTIPLY)
      {
        lexer_next_token (context_p);

        if (lexer_token_is_identifier (context_p, "as", 2))
        {
          lexer_next_token (context_p);

          if (context_p->token.type != LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
          {
            scanner_raise_error (context_p);
          }

          lexer_next_token (context_p);
        }

        if (!lexer_token_is_identifier (context_p, "from", 4))
        {
          scanner_raise_error (context_p);
        }

        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_LITERAL && context_p->token.lit_location.type == LEXER_STRING_LITERAL)
        {
          scanner_raise_error (context_p);
        }

        return SCAN_NEXT_TOKEN;
      }

      scanner_source_start_t source_start;
      source_start.source_p = context_p->source_p;

      if (context_p->token.type == LEXER_LEFT_BRACE)
      {
        lexer_next_token (context_p);

        while (context_p->token.type != LEXER_RIGHT_BRACE)
        {
          if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
          {
            scanner_raise_error (context_p);
          }

          lexer_next_token (context_p);

          if (lexer_token_is_identifier (context_p, "as", 2))
          {
            lexer_next_token (context_p);

            if (context_p->token.type != LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
            {
              scanner_raise_error (context_p);
            }

            lexer_next_token (context_p);
          }

          if (context_p->token.type != LEXER_RIGHT_BRACE)
          {
            if (context_p->token.type != LEXER_COMMA)
            {
              scanner_raise_error (context_p);
            }

            lexer_next_token (context_p);
          }
        }

        lexer_next_token (context_p);

        if (!lexer_token_is_identifier (context_p, "from", 4))
        {
          return SCAN_KEEP_TOKEN;
        }

        scanner_info_t *info_p = scanner_insert_info (context_p, source_start.source_p, sizeof (scanner_info_t));
        info_p->type = SCANNER_TYPE_EXPORT_MODULE_SPECIFIER;

        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_LITERAL && context_p->token.lit_location.type == LEXER_STRING_LITERAL)
        {
          scanner_raise_error (context_p);
        }

        return SCAN_NEXT_TOKEN;
      }

      switch (context_p->token.type)
      {
        case LEXER_KEYW_CLASS:
        case LEXER_KEYW_LET:
        case LEXER_KEYW_CONST:
        case LEXER_KEYW_VAR:
        {
          scanner_context_p->active_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_IN_EXPORT;
          break;
        }
      }

      scanner_context_p->mode = SCAN_MODE_STATEMENT;
      return SCAN_KEEP_TOKEN;
    }
#endif /* JERRY_MODULE_SYSTEM */
    default:
    {
      break;
    }
  }

  scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;

  if (type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
  {
    if (JERRY_UNLIKELY (lexer_check_next_character (context_p, LIT_CHAR_COLON)))
    {
      lexer_consume_next_character (context_p);
      scanner_context_p->mode = SCAN_MODE_STATEMENT;
      return SCAN_NEXT_TOKEN;
    }

    JERRY_ASSERT (context_p->token.flags & LEXER_NO_SKIP_SPACES);

    /* The colon needs to be checked first because the parser also checks
     * it first, and this check skips the spaces which affects source_p. */
    if (JERRY_UNLIKELY (lexer_check_arrow (context_p)))
    {
      scanner_scan_simple_arrow (context_p, scanner_context_p, context_p->source_p);
      return SCAN_KEEP_TOKEN;
    }

    if (JERRY_UNLIKELY (lexer_token_is_let (context_p)))
    {
      lexer_lit_location_t let_literal = context_p->token.lit_location;
      const uint8_t *source_p = context_p->source_p;

      lexer_next_token (context_p);

      type = (lexer_token_type_t) context_p->token.type;

      if (type == LEXER_LEFT_SQUARE || type == LEXER_LEFT_BRACE
          || (type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL))
      {
        scanner_context_p->mode = SCAN_MODE_VAR_STATEMENT;
        parser_stack_push_uint8 (context_p, SCAN_STACK_LET);
        return SCAN_KEEP_TOKEN;
      }

      scanner_info_t *info_p = scanner_insert_info (context_p, source_p, sizeof (scanner_info_t));
      info_p->type = SCANNER_TYPE_LET_EXPRESSION;

      lexer_lit_location_t *lit_location_p =
        scanner_add_custom_literal (context_p, scanner_context_p->active_literal_pool_p, &let_literal);
      lit_location_p->type |= SCANNER_LITERAL_IS_USED;

      if (scanner_context_p->active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_WITH)
      {
        lit_location_p->type |= SCANNER_LITERAL_NO_REG;
      }

      scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
      return SCAN_KEEP_TOKEN;
    }

    if (JERRY_UNLIKELY (lexer_token_is_async (context_p)))
    {
      scanner_context_p->async_source_p = context_p->source_p;

      if (scanner_check_async_function (context_p, scanner_context_p))
      {
        scanner_context_p->mode = SCAN_MODE_STATEMENT;
      }
      return SCAN_KEEP_TOKEN;
    }

    scanner_add_reference (context_p, scanner_context_p);

    scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
    return SCAN_NEXT_TOKEN;
  }

  return SCAN_KEEP_TOKEN;
} /* scanner_scan_statement */

/**
 * Scan statement terminator.
 *
 * @return SCAN_NEXT_TOKEN to read the next token, or SCAN_KEEP_TOKEN to do nothing
 */
static scan_return_types_t
scanner_scan_statement_end (parser_context_t *context_p, /**< context */
                            scanner_context_t *scanner_context_p, /**< scanner context */
                            lexer_token_type_t type) /**< current token type */
{
  bool terminator_found = false;

  if (type == LEXER_SEMICOLON)
  {
    lexer_next_token (context_p);
    terminator_found = true;
  }

  while (true)
  {
    type = (lexer_token_type_t) context_p->token.type;

    switch (context_p->stack_top_uint8)
    {
      case SCAN_STACK_SCRIPT:
      case SCAN_STACK_SCRIPT_FUNCTION:
      {
        if (type == LEXER_EOS)
        {
          return SCAN_NEXT_TOKEN;
        }
        break;
      }
      case SCAN_STACK_BLOCK_STATEMENT:
      case SCAN_STACK_CLASS_STATEMENT:
      case SCAN_STACK_FUNCTION_STATEMENT:
      {
        if (type != LEXER_RIGHT_BRACE)
        {
          break;
        }

        if (context_p->stack_top_uint8 != SCAN_STACK_CLASS_STATEMENT)
        {
          scanner_pop_literal_pool (context_p, scanner_context_p);
        }

        terminator_found = true;
        parser_stack_pop_uint8 (context_p);
#if JERRY_MODULE_SYSTEM
        scanner_context_p->active_literal_pool_p->status_flags &= (uint16_t) ~SCANNER_LITERAL_POOL_IN_EXPORT;
#endif /* JERRY_MODULE_SYSTEM */
        lexer_next_token (context_p);
        continue;
      }
      case SCAN_STACK_FUNCTION_EXPRESSION:
      case SCAN_STACK_FUNCTION_ARROW:
      {
        if (type != LEXER_RIGHT_BRACE)
        {
          break;
        }

        scanner_context_p->mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;

        if (context_p->stack_top_uint8 == SCAN_STACK_FUNCTION_ARROW)
        {
          scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
        }

        scanner_pop_literal_pool (context_p, scanner_context_p);
        parser_stack_pop_uint8 (context_p);

#if JERRY_MODULE_SYSTEM
        if (context_p->stack_top_uint8 == SCAN_STACK_EXPORT_DEFAULT)
        {
          terminator_found = true;
          parser_stack_pop_uint8 (context_p);
          lexer_next_token (context_p);
          continue;
        }
#endif /* JERRY_MODULE_SYSTEM */
        return SCAN_NEXT_TOKEN;
      }
      case SCAN_STACK_FUNCTION_PROPERTY:
      {
        if (type != LEXER_RIGHT_BRACE)
        {
          break;
        }

        bool has_super_reference =
          (scanner_context_p->active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_HAS_SUPER_REFERENCE) != 0;

        scanner_pop_literal_pool (context_p, scanner_context_p);
        parser_stack_pop_uint8 (context_p);

        if (context_p->stack_top_uint8 == SCAN_STACK_EXPLICIT_CLASS_CONSTRUCTOR
            || context_p->stack_top_uint8 == SCAN_STACK_IMPLICIT_CLASS_CONSTRUCTOR)
        {
          scanner_context_p->mode = SCAN_MODE_CLASS_BODY;
          return SCAN_KEEP_TOKEN;
        }

        if (has_super_reference && context_p->stack_top_uint8 == SCAN_STACK_OBJECT_LITERAL)
        {
          *parser_stack_get_prev_uint8 (context_p) |= SCANNER_LITERAL_OBJECT_HAS_SUPER;
        }

        lexer_next_token (context_p);

        if (context_p->token.type == LEXER_RIGHT_BRACE)
        {
          scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
          return SCAN_KEEP_TOKEN;
        }

        if (context_p->token.type != LEXER_COMMA)
        {
          scanner_raise_error (context_p);
        }

        scanner_context_p->mode = SCAN_MODE_PROPERTY_NAME;
        return SCAN_KEEP_TOKEN;
      }
      case SCAN_STACK_SWITCH_BLOCK:
      {
        if (type != LEXER_RIGHT_BRACE)
        {
          break;
        }

        scanner_switch_statement_t switch_statement;

        parser_stack_pop_uint8 (context_p);
        parser_stack_pop (context_p, &switch_statement, sizeof (scanner_switch_statement_t));

        scanner_context_p->active_switch_statement = switch_statement;

        scanner_pop_literal_pool (context_p, scanner_context_p);

        terminator_found = true;
        lexer_next_token (context_p);
        continue;
      }
      case SCAN_STACK_IF_STATEMENT:
      {
        parser_stack_pop_uint8 (context_p);

        if (type == LEXER_KEYW_ELSE && (terminator_found || (context_p->token.flags & LEXER_WAS_NEWLINE)))
        {
          scanner_check_function_after_if (context_p, scanner_context_p);
          return SCAN_KEEP_TOKEN;
        }

        continue;
      }
      case SCAN_STACK_WITH_STATEMENT:
      {
        scanner_literal_pool_t *literal_pool_p = scanner_context_p->active_literal_pool_p;

        JERRY_ASSERT (literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_WITH);

        parser_stack_pop_uint8 (context_p);

        if (context_p->stack_top_uint8 == 0)
        {
          literal_pool_p->status_flags &= (uint16_t) ~SCANNER_LITERAL_POOL_IN_WITH;
        }

        parser_stack_pop_uint8 (context_p);
        continue;
      }
      case SCAN_STACK_DO_STATEMENT:
      {
        parser_stack_pop_uint8 (context_p);

        if (type != LEXER_KEYW_WHILE || (!terminator_found && !(context_p->token.flags & LEXER_WAS_NEWLINE)))
        {
          scanner_raise_error (context_p);
        }

        lexer_next_token (context_p);
        if (context_p->token.type != LEXER_LEFT_PAREN)
        {
          scanner_raise_error (context_p);
        }

        parser_stack_push_uint8 (context_p, SCAN_STACK_DO_EXPRESSION);
        scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
        return SCAN_NEXT_TOKEN;
      }
      case SCAN_STACK_DO_EXPRESSION:
      {
        parser_stack_pop_uint8 (context_p);
        terminator_found = true;
        continue;
      }
      case SCAN_STACK_CLASS_STATIC_BLOCK:
      {
        if (type != LEXER_RIGHT_BRACE)
        {
          break;
        }

        scanner_pop_literal_pool (context_p, scanner_context_p);
        scanner_source_start_t start_range;
        parser_stack_pop_uint8 (context_p);
        parser_stack_pop (context_p, &start_range, sizeof (scanner_source_start_t));

        scanner_context_p->mode = SCAN_MODE_CLASS_BODY_NO_SCAN;

        scanner_location_info_t *location_info_p;
        location_info_p = (scanner_location_info_t *) scanner_insert_info (context_p,
                                                                           start_range.source_p,
                                                                           sizeof (scanner_location_info_t));

        location_info_p->info.type = SCANNER_TYPE_CLASS_STATIC_BLOCK_END;
        location_info_p->location.source_p = context_p->source_p;
        location_info_p->location.line = context_p->token.line;
        location_info_p->location.column = context_p->token.column;

        lexer_scan_identifier (context_p, (lexer_parse_options_t) ((unsigned int) LEXER_PARSE_CHECK_KEYWORDS | (unsigned int) LEXER_PARSE_NO_STRICT_IDENT_ERROR));
        return SCAN_KEEP_TOKEN;
      }
      case SCAN_STACK_PRIVATE_BLOCK_EARLY:
      {
        parser_list_iterator_t literal_iterator;
        lexer_lit_location_t *literal_p;

        parser_list_iterator_init (&scanner_context_p->active_literal_pool_p->literal_pool, &literal_iterator);

        while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
        {
          if ((literal_p->type & (SCANNER_LITERAL_IS_LET | SCANNER_LITERAL_IS_CONST))
              && (literal_p->type & SCANNER_LITERAL_IS_USED))
          {
            literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
          }
        }
        /* FALLTHRU */
      }
      case SCAN_STACK_PRIVATE_BLOCK:
      {
        parser_stack_pop_uint8 (context_p);
        scanner_pop_literal_pool (context_p, scanner_context_p);
        continue;
      }
#if JERRY_MODULE_SYSTEM
      case SCAN_STACK_EXPORT_DEFAULT:
      {
        parser_stack_pop_uint8 (context_p);
        lexer_lit_location_t *location_p =
          scanner_add_custom_literal (context_p, scanner_context_p->active_literal_pool_p, &lexer_default_literal);
        location_p->type |= SCANNER_LITERAL_IS_VAR;
        continue;
      }
#endif /* JERRY_MODULE_SYSTEM */
      default:
      {
        JERRY_ASSERT (context_p->stack_top_uint8 == SCAN_STACK_TRY_STATEMENT
                      || context_p->stack_top_uint8 == SCAN_STACK_CATCH_STATEMENT);

        if (type != LEXER_RIGHT_BRACE)
        {
          break;
        }

        uint8_t stack_top = context_p->stack_top_uint8;
        parser_stack_pop_uint8 (context_p);
        lexer_next_token (context_p);

        scanner_pop_literal_pool (context_p, scanner_context_p);

        /* A finally statement is optional after a try or catch statement. */
        if (context_p->token.type == LEXER_KEYW_FINALLY)
        {
          lexer_next_token (context_p);

          if (context_p->token.type != LEXER_LEFT_BRACE)
          {
            scanner_raise_error (context_p);
          }

          scanner_literal_pool_t *literal_pool_p;
          literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, 0);
          literal_pool_p->source_p = context_p->source_p;

          parser_stack_push_uint8 (context_p, SCAN_STACK_BLOCK_STATEMENT);
          scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
          return SCAN_NEXT_TOKEN;
        }

        if (stack_top == SCAN_STACK_CATCH_STATEMENT)
        {
          terminator_found = true;
          continue;
        }

        /* A catch statement must be present after a try statement unless a finally is provided. */
        if (context_p->token.type != LEXER_KEYW_CATCH)
        {
          scanner_raise_error (context_p);
        }

        lexer_next_token (context_p);

        scanner_literal_pool_t *literal_pool_p;
        literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, 0);
        literal_pool_p->source_p = context_p->source_p;
        parser_stack_push_uint8 (context_p, SCAN_STACK_CATCH_STATEMENT);

        if (context_p->token.type == LEXER_LEFT_BRACE)
        {
          scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
          return SCAN_NEXT_TOKEN;
        }

        if (context_p->token.type != LEXER_LEFT_PAREN)
        {
          scanner_raise_error (context_p);
        }

        lexer_next_token (context_p);

        if (context_p->token.type == LEXER_LEFT_SQUARE || context_p->token.type == LEXER_LEFT_BRACE)
        {
          scanner_push_destructuring_pattern (context_p, scanner_context_p, SCANNER_BINDING_CATCH, false);

          if (context_p->token.type == LEXER_LEFT_SQUARE)
          {
            parser_stack_push_uint8 (context_p, SCAN_STACK_ARRAY_LITERAL);
            scanner_context_p->mode = SCAN_MODE_BINDING;
            return SCAN_NEXT_TOKEN;
          }

          parser_stack_push_uint8 (context_p, 0);
          parser_stack_push_uint8 (context_p, SCAN_STACK_OBJECT_LITERAL);
          scanner_context_p->mode = SCAN_MODE_PROPERTY_NAME;
          return SCAN_KEEP_TOKEN;
        }

        if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
        {
          scanner_raise_error (context_p);
        }

        lexer_lit_location_t *lit_location_p = scanner_add_literal (context_p, scanner_context_p);
        lit_location_p->type |= SCANNER_LITERAL_IS_LOCAL;

        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_RIGHT_PAREN)
        {
          scanner_raise_error (context_p);
        }

        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_LEFT_BRACE)
        {
          scanner_raise_error (context_p);
        }

        scanner_context_p->mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;
        return SCAN_NEXT_TOKEN;
      }
    }

    if (!terminator_found && !(context_p->token.flags & LEXER_WAS_NEWLINE))
    {
      scanner_raise_error (context_p);
    }

    scanner_context_p->mode = SCAN_MODE_STATEMENT;
    return SCAN_KEEP_TOKEN;
  }
} /* scanner_scan_statement_end */

/**
 * Scan the whole source code.
 */
void JERRY_ATTR_NOINLINE
scanner_scan_all (parser_context_t *context_p) /**< context */
{
  scanner_context_t scanner_context;

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    JERRY_DEBUG_MSG ("\n--- Scanning start ---\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  scanner_context.context_status_flags = context_p->status_flags;
  scanner_context.status_flags = SCANNER_CONTEXT_NO_FLAGS;
  scanner_context.binding_type = SCANNER_BINDING_NONE;
  scanner_context.active_binding_list_p = NULL;
  scanner_context.active_literal_pool_p = NULL;
  scanner_context.active_switch_statement.last_case_p = NULL;
  scanner_context.end_arguments_p = NULL;
  scanner_context.async_source_p = NULL;

  /* This assignment must be here because of Apple compilers. */
  context_p->u.scanner_context_p = &scanner_context;
  context_p->global_status_flags |= ECMA_PARSE_INTERNAL_PRE_SCANNING;

  parser_stack_init (context_p);

  PARSER_TRY (context_p->try_buffer)
  {
    if (context_p->arguments_start_p == NULL)
    {
      context_p->source_p = context_p->source_start_p;
      context_p->source_end_p = context_p->source_start_p + context_p->source_size;

      uint16_t status_flags =
        (SCANNER_LITERAL_POOL_FUNCTION | SCANNER_LITERAL_POOL_NO_ARGUMENTS | SCANNER_LITERAL_POOL_CAN_EVAL);

      if (context_p->status_flags & PARSER_IS_STRICT)
      {
        status_flags |= SCANNER_LITERAL_POOL_IS_STRICT;
      }

      scanner_literal_pool_t *literal_pool_p = scanner_push_literal_pool (context_p, &scanner_context, status_flags);
      literal_pool_p->source_p = context_p->source_start_p;

      parser_stack_push_uint8 (context_p, SCAN_STACK_SCRIPT);

      lexer_next_token (context_p);
      scanner_check_directives (context_p, &scanner_context);
    }
    else
    {
      context_p->source_p = context_p->arguments_start_p;
      context_p->source_end_p = context_p->arguments_start_p + context_p->arguments_size;

      uint16_t status_flags = SCANNER_LITERAL_POOL_FUNCTION;

      if (context_p->status_flags & PARSER_IS_STRICT)
      {
        status_flags |= SCANNER_LITERAL_POOL_IS_STRICT;
      }

      if (context_p->status_flags & PARSER_IS_GENERATOR_FUNCTION)
      {
        status_flags |= SCANNER_LITERAL_POOL_GENERATOR;
      }
      if (context_p->status_flags & PARSER_IS_ASYNC_FUNCTION)
      {
        status_flags |= SCANNER_LITERAL_POOL_ASYNC;
      }

      scanner_push_literal_pool (context_p, &scanner_context, status_flags);
      scanner_context.mode = SCAN_MODE_FUNCTION_ARGUMENTS;
      parser_stack_push_uint8 (context_p, SCAN_STACK_SCRIPT_FUNCTION);

      /* Faking the first token. */
      context_p->token.type = LEXER_LEFT_PAREN;
    }

    while (true)
    {
      lexer_token_type_t type = (lexer_token_type_t) context_p->token.type;
      scan_stack_modes_t stack_top = (scan_stack_modes_t) context_p->stack_top_uint8;

      switch (scanner_context.mode)
      {
        case SCAN_MODE_PRIMARY_EXPRESSION:
        {
          if (type == LEXER_ADD || type == LEXER_SUBTRACT || LEXER_IS_UNARY_OP_TOKEN (type))
          {
            break;
          }
          /* FALLTHRU */
        }
        case SCAN_MODE_PRIMARY_EXPRESSION_AFTER_NEW:
        {
          if (scanner_scan_primary_expression (context_p, &scanner_context, type, stack_top) != SCAN_NEXT_TOKEN)
          {
            continue;
          }
          break;
        }
        case SCAN_MODE_CLASS_DECLARATION:
        {
          if (context_p->token.type == LEXER_KEYW_EXTENDS)
          {
            parser_stack_push_uint8 (context_p, SCAN_STACK_CLASS_EXTENDS);
            scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;
            break;
          }
          else if (context_p->token.type != LEXER_LEFT_BRACE)
          {
            scanner_raise_error (context_p);
          }

          scanner_context.mode = SCAN_MODE_CLASS_BODY;
          /* FALLTHRU */
        }
        case SCAN_MODE_CLASS_BODY:
        {
          lexer_skip_empty_statements (context_p);
          lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);
          /* FALLTHRU */
        }
        case SCAN_MODE_CLASS_BODY_NO_SCAN:
        {
          JERRY_ASSERT (stack_top == SCAN_STACK_IMPLICIT_CLASS_CONSTRUCTOR
                        || stack_top == SCAN_STACK_EXPLICIT_CLASS_CONSTRUCTOR);
          JERRY_ASSERT (scanner_context.active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_CLASS_NAME);

          if (context_p->token.type == LEXER_RIGHT_BRACE)
          {
            parser_stack_pop_uint8 (context_p);

            scanner_class_info_t *private_members_p;
            parser_stack_pop (context_p, &private_members_p, sizeof (scanner_class_info_t *));

            private_members_p->info.u8_arg |= SCANNER_SUCCESSFUL_CLASS_SCAN;

            scanner_pop_literal_pool (context_p, &scanner_context);

            JERRY_ASSERT (context_p->stack_top_uint8 == SCAN_STACK_CLASS_STATEMENT
                          || context_p->stack_top_uint8 == SCAN_STACK_CLASS_EXPRESSION);

            if (context_p->stack_top_uint8 == SCAN_STACK_CLASS_STATEMENT)
            {
              /* The token is kept to disallow consuming a semicolon after it. */
              scanner_context.mode = SCAN_MODE_STATEMENT_END;
              continue;
            }

            scanner_context.mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;
            parser_stack_pop_uint8 (context_p);

#if JERRY_MODULE_SYSTEM
            if (context_p->stack_top_uint8 == SCAN_STACK_EXPORT_DEFAULT)
            {
              /* The token is kept to disallow consuming a semicolon after it. */
              parser_stack_change_last_uint8 (context_p, SCAN_STACK_CLASS_STATEMENT);
              scanner_context.mode = SCAN_MODE_STATEMENT_END;
              continue;
            }
#endif /* JERRY_MODULE_SYSTEM */
            break;
          }

          bool is_private = false;
          scanner_private_field_flags_t private_field_flags = SCANNER_PRIVATE_FIELD_PROPERTY;

          if (context_p->token.type == LEXER_HASHMARK)
          {
            is_private = true;
            context_p->token.flags |= LEXER_NO_SKIP_SPACES;
            lexer_next_token (context_p);
          }

          bool identifier_found = false;

          if (context_p->token.type == LEXER_LITERAL && LEXER_IS_IDENT_OR_STRING (context_p->token.lit_location.type)
              && lexer_compare_literal_to_string (context_p, "constructor", 11)
              && stack_top == SCAN_STACK_IMPLICIT_CLASS_CONSTRUCTOR)
          {
            parser_stack_pop_uint8 (context_p);
            scanner_class_info_t *private_members_p;
            parser_stack_pop (context_p, &private_members_p, sizeof (scanner_class_info_t *));
            private_members_p->info.u8_arg = SCANNER_CONSTRUCTOR_EXPLICIT;
            parser_stack_push (context_p, &private_members_p, sizeof (scanner_class_info_t *));
            parser_stack_push_uint8 (context_p, SCAN_STACK_EXPLICIT_CLASS_CONSTRUCTOR);
          }
          else if (lexer_token_is_identifier (context_p, "static", 6))
          {
            scanner_source_start_t static_start;
            static_start.source_p = context_p->source_p - 1;

            lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);
            identifier_found = true;
            private_field_flags = (scanner_private_field_flags_t) ((unsigned int) private_field_flags | (unsigned int) SCANNER_PRIVATE_FIELD_STATIC);

            if (!is_private && context_p->token.type == LEXER_LEFT_BRACE)
            {
              parser_stack_push (context_p, &static_start, sizeof (scanner_source_start_t));
              parser_stack_push_uint8 (context_p, SCAN_STACK_CLASS_STATIC_BLOCK);

              scanner_literal_pool_t *literal_pool_p =
                scanner_push_literal_pool (context_p, &scanner_context, SCANNER_LITERAL_POOL_FUNCTION);
              literal_pool_p->source_p = context_p->source_p - 1;

              lexer_next_token (context_p);

              scanner_context.mode = SCAN_MODE_STATEMENT_OR_TERMINATOR;

              continue;
            }
          }

          scanner_context.mode = SCAN_MODE_FUNCTION_ARGUMENTS;

          uint16_t literal_pool_flags = SCANNER_LITERAL_POOL_FUNCTION;

          private_field_flags = ((bool)private_field_flags || lexer_token_is_identifier (context_p, "get", 3) ? SCANNER_PRIVATE_FIELD_GETTER : (scanner_private_field_flags_t) 0);
          private_field_flags = ((bool)private_field_flags || lexer_token_is_identifier (context_p, "set", 3) ? SCANNER_PRIVATE_FIELD_SETTER : (scanner_private_field_flags_t) 0);

          if (private_field_flags & SCANNER_PRIVATE_FIELD_GETTER_SETTER)
          {
            private_field_flags = (scanner_private_field_flags_t) ((uint32_t) private_field_flags & ~(uint32_t) SCANNER_PRIVATE_FIELD_PROPERTY);

            lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);
            identifier_found = true;

            if (context_p->token.type == LEXER_LEFT_PAREN)
            {
              if (is_private)
              {
                private_field_flags = (scanner_private_field_flags_t) ((int) private_field_flags | (int) SCANNER_PRIVATE_FIELD_METHOD);
                scanner_add_private_identifier (context_p, private_field_flags);
              }

              parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PROPERTY);
              scanner_push_literal_pool (context_p, &scanner_context, SCANNER_LITERAL_POOL_FUNCTION);
              continue;
            }
          }
          else if (lexer_token_is_identifier (context_p, "async", 5))
          {
            lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);
            identifier_found = true;

            if (!(context_p->token.flags & LEXER_WAS_NEWLINE))
            {
              if (context_p->token.type == LEXER_LEFT_PAREN)
              {
                if (is_private)
                {
                  private_field_flags = (scanner_private_field_flags_t) ((int) private_field_flags | (int) SCANNER_PRIVATE_FIELD_METHOD);
                  scanner_add_private_identifier (context_p, private_field_flags);
                }

                parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PROPERTY);
                scanner_push_literal_pool (context_p, &scanner_context, SCANNER_LITERAL_POOL_FUNCTION);
                continue;
              }

              literal_pool_flags |= SCANNER_LITERAL_POOL_ASYNC;

              if (context_p->token.type == LEXER_MULTIPLY)
              {
                lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);
                literal_pool_flags |= SCANNER_LITERAL_POOL_GENERATOR;
              }
            }
          }
          else if (context_p->token.type == LEXER_MULTIPLY)
          {
            if (is_private)
            {
              scanner_raise_error (context_p);
            }

            lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);
            literal_pool_flags |= SCANNER_LITERAL_POOL_GENERATOR;
          }

          if (context_p->token.type == LEXER_LEFT_SQUARE)
          {
            if (is_private)
            {
              scanner_raise_error (context_p);
            }

            if (literal_pool_flags != SCANNER_LITERAL_POOL_FUNCTION)
            {
              parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PROPERTY);
            }

            parser_stack_push_uint8 (context_p, SCANNER_FROM_LITERAL_POOL_TO_COMPUTED (literal_pool_flags));
            scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;
            break;
          }

          if (context_p->token.type == LEXER_HASHMARK)
          {
            if (is_private)
            {
              scanner_raise_error (context_p);
            }

            is_private = true;
            context_p->token.flags |= LEXER_NO_SKIP_SPACES;
            lexer_next_token (context_p);
          }

          if (is_private)
          {
            if (lexer_check_next_character (context_p, LIT_CHAR_LEFT_PAREN))
            {
              private_field_flags = (scanner_private_field_flags_t) ((int) private_field_flags | (int) SCANNER_PRIVATE_FIELD_METHOD);
            }

            scanner_add_private_identifier (context_p, private_field_flags);
          }

          if (context_p->token.type == LEXER_LITERAL)
          {
            lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);
            identifier_found = true;
          }

          if (!identifier_found)
          {
            scanner_raise_error (context_p);
          }

          if (context_p->token.type == LEXER_LEFT_PAREN)
          {
            if (literal_pool_flags & SCANNER_LITERAL_POOL_GENERATOR)
            {
              context_p->status_flags |= PARSER_IS_GENERATOR_FUNCTION;
            }

            parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PROPERTY);
            scanner_push_literal_pool (context_p, &scanner_context, literal_pool_flags);
            continue;
          }

          if (literal_pool_flags & SCANNER_LITERAL_POOL_GENERATOR)
          {
            scanner_raise_error (context_p);
          }

          if (context_p->token.type == LEXER_ASSIGN)
          {
            scanner_push_class_field_initializer (context_p, &scanner_context);
            break;
          }

          if (context_p->token.type == LEXER_SEMICOLON)
          {
            scanner_context.mode = SCAN_MODE_CLASS_BODY;
            continue;
          }

          if (context_p->token.type != LEXER_RIGHT_BRACE && !(context_p->token.flags & LEXER_WAS_NEWLINE))
          {
            scanner_raise_error (context_p);
          }

          scanner_context.mode = SCAN_MODE_CLASS_BODY_NO_SCAN;
          continue;
        }
        case SCAN_MODE_POST_PRIMARY_EXPRESSION:
        {
          if (scanner_scan_post_primary_expression (context_p, &scanner_context, type, stack_top))
          {
            break;
          }
          type = (lexer_token_type_t) context_p->token.type;
          /* FALLTHRU */
        }
        case SCAN_MODE_PRIMARY_EXPRESSION_END:
        {
          if (scanner_scan_primary_expression_end (context_p, &scanner_context, type, stack_top) != SCAN_NEXT_TOKEN)
          {
            continue;
          }
          break;
        }
        case SCAN_MODE_STATEMENT_OR_TERMINATOR:
        {
          if (type == LEXER_RIGHT_BRACE || type == LEXER_EOS)
          {
            scanner_context.mode = SCAN_MODE_STATEMENT_END;
            continue;
          }
          /* FALLTHRU */
        }
        case SCAN_MODE_STATEMENT:
        {
          if (scanner_scan_statement (context_p, &scanner_context, type, stack_top) != SCAN_NEXT_TOKEN)
          {
            continue;
          }
          break;
        }
        case SCAN_MODE_STATEMENT_END:
        {
          if (scanner_scan_statement_end (context_p, &scanner_context, type) != SCAN_NEXT_TOKEN)
          {
            continue;
          }

          if (context_p->token.type == LEXER_EOS)
          {
            goto scan_completed;
          }

          break;
        }
        case SCAN_MODE_VAR_STATEMENT:
        {
          if (type == LEXER_LEFT_SQUARE || type == LEXER_LEFT_BRACE)
          {
            uint8_t binding_type = SCANNER_BINDING_VAR;

            if (stack_top == SCAN_STACK_LET || stack_top == SCAN_STACK_FOR_LET_START)
            {
              binding_type = SCANNER_BINDING_LET;
            }
            else if (stack_top == SCAN_STACK_CONST || stack_top == SCAN_STACK_FOR_CONST_START)
            {
              binding_type = SCANNER_BINDING_CONST;
            }

            scanner_push_destructuring_pattern (context_p, &scanner_context, binding_type, false);

            if (type == LEXER_LEFT_SQUARE)
            {
              parser_stack_push_uint8 (context_p, SCAN_STACK_ARRAY_LITERAL);
              scanner_context.mode = SCAN_MODE_BINDING;
              break;
            }

            parser_stack_push_uint8 (context_p, 0);
            parser_stack_push_uint8 (context_p, SCAN_STACK_OBJECT_LITERAL);
            scanner_context.mode = SCAN_MODE_PROPERTY_NAME;
            continue;
          }

          if (type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
          {
            scanner_raise_error (context_p);
          }

          lexer_lit_location_t *literal_p = scanner_add_literal (context_p, &scanner_context);

          if (stack_top != SCAN_STACK_VAR && stack_top != SCAN_STACK_FOR_VAR_START)
          {
            scanner_detect_invalid_let (context_p, literal_p);

            if (stack_top == SCAN_STACK_LET || stack_top == SCAN_STACK_FOR_LET_START)
            {
              literal_p->type |= SCANNER_LITERAL_IS_LET;
            }
            else
            {
              JERRY_ASSERT (stack_top == SCAN_STACK_CONST || stack_top == SCAN_STACK_FOR_CONST_START);
              literal_p->type |= SCANNER_LITERAL_IS_CONST;
            }

            lexer_next_token (context_p);

            if (literal_p->type & SCANNER_LITERAL_IS_USED)
            {
              literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
            }
            else if (context_p->token.type == LEXER_ASSIGN)
            {
              scanner_binding_literal_t binding_literal;
              binding_literal.literal_p = literal_p;

              parser_stack_push (context_p, &binding_literal, sizeof (scanner_binding_literal_t));
              parser_stack_push_uint8 (context_p, SCAN_STACK_BINDING_INIT);
            }
          }
          else
          {
            if (!(literal_p->type & SCANNER_LITERAL_IS_VAR))
            {
              scanner_detect_invalid_var (context_p, &scanner_context, literal_p);
              literal_p->type |= SCANNER_LITERAL_IS_VAR;

              if (scanner_context.active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_WITH)
              {
                literal_p->type |= SCANNER_LITERAL_NO_REG;
              }
            }

            lexer_next_token (context_p);
          }

#if JERRY_MODULE_SYSTEM
          if (scanner_context.active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_EXPORT)
          {
            literal_p->type |= SCANNER_LITERAL_NO_REG;
          }
#endif /* JERRY_MODULE_SYSTEM */

          switch (context_p->token.type)
          {
            case LEXER_ASSIGN:
            {
              scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;
              /* FALLTHRU */
            }
            case LEXER_COMMA:
            {
              lexer_next_token (context_p);
              continue;
            }
          }

          if (SCANNER_IS_FOR_START (stack_top))
          {
#if JERRY_MODULE_SYSTEM
            JERRY_ASSERT (!(scanner_context.active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_EXPORT));
#endif /* JERRY_MODULE_SYSTEM */

            if (context_p->token.type != LEXER_SEMICOLON && context_p->token.type != LEXER_KEYW_IN
                && !SCANNER_IDENTIFIER_IS_OF ())
            {
              scanner_raise_error (context_p);
            }

            scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
            continue;
          }

          JERRY_ASSERT (stack_top == SCAN_STACK_VAR || stack_top == SCAN_STACK_LET || stack_top == SCAN_STACK_CONST);

#if JERRY_MODULE_SYSTEM
          scanner_context.active_literal_pool_p->status_flags &= (uint16_t) ~SCANNER_LITERAL_POOL_IN_EXPORT;
#endif /* JERRY_MODULE_SYSTEM */

          scanner_context.mode = SCAN_MODE_STATEMENT_END;
          parser_stack_pop_uint8 (context_p);
          continue;
        }
        case SCAN_MODE_FUNCTION_ARGUMENTS:
        {
          JERRY_ASSERT (stack_top == SCAN_STACK_SCRIPT_FUNCTION || stack_top == SCAN_STACK_FUNCTION_STATEMENT
                        || stack_top == SCAN_STACK_FUNCTION_EXPRESSION || stack_top == SCAN_STACK_FUNCTION_PROPERTY);

          scanner_literal_pool_t *literal_pool_p = scanner_context.active_literal_pool_p;

          JERRY_ASSERT (literal_pool_p != NULL && (literal_pool_p->status_flags & SCANNER_LITERAL_POOL_FUNCTION));

          literal_pool_p->source_p = context_p->source_p;

          if (JERRY_UNLIKELY (scanner_context.async_source_p != NULL))
          {
            literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_ASYNC;
            literal_pool_p->source_p = scanner_context.async_source_p;
            scanner_context.async_source_p = NULL;
          }

          if (type != LEXER_LEFT_PAREN)
          {
            scanner_raise_error (context_p);
          }
          lexer_next_token (context_p);

          /* FALLTHRU */
        }
        case SCAN_MODE_CONTINUE_FUNCTION_ARGUMENTS:
        {
          if (context_p->token.type != LEXER_RIGHT_PAREN && context_p->token.type != LEXER_EOS)
          {
            lexer_lit_location_t *argument_literal_p;

            do
            {
              if (context_p->token.type == LEXER_THREE_DOTS)
              {
                scanner_context.active_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_HAS_COMPLEX_ARGUMENT;
                lexer_next_token (context_p);
              }

              if (context_p->token.type == LEXER_LEFT_SQUARE || context_p->token.type == LEXER_LEFT_BRACE)
              {
                argument_literal_p = NULL;
                break;
              }

              if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
              {
                scanner_raise_error (context_p);
              }

              argument_literal_p = scanner_append_argument (context_p, &scanner_context);
              lexer_next_token (context_p);

              if (context_p->token.type != LEXER_COMMA)
              {
                break;
              }

              lexer_next_token (context_p);
            } while (context_p->token.type != LEXER_RIGHT_PAREN && context_p->token.type != LEXER_EOS);

            if (argument_literal_p == NULL)
            {
              scanner_context.active_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_HAS_COMPLEX_ARGUMENT;

              parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PARAMETERS);
              scanner_append_hole (context_p, &scanner_context);
              scanner_push_destructuring_pattern (context_p, &scanner_context, SCANNER_BINDING_ARG, false);

              if (context_p->token.type == LEXER_LEFT_SQUARE)
              {
                parser_stack_push_uint8 (context_p, SCAN_STACK_ARRAY_LITERAL);
                scanner_context.mode = SCAN_MODE_BINDING;
                break;
              }

              parser_stack_push_uint8 (context_p, 0);
              parser_stack_push_uint8 (context_p, SCAN_STACK_OBJECT_LITERAL);
              scanner_context.mode = SCAN_MODE_PROPERTY_NAME;
              continue;
            }

            if (context_p->token.type == LEXER_ASSIGN)
            {
              scanner_context.active_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_HAS_COMPLEX_ARGUMENT;

              parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PARAMETERS);
              scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;

              if (argument_literal_p->type & SCANNER_LITERAL_IS_USED)
              {
                JERRY_ASSERT (argument_literal_p->type & SCANNER_LITERAL_EARLY_CREATE);
                break;
              }

              scanner_binding_literal_t binding_literal;
              binding_literal.literal_p = argument_literal_p;

              parser_stack_push (context_p, &binding_literal, sizeof (scanner_binding_literal_t));
              parser_stack_push_uint8 (context_p, SCAN_STACK_BINDING_INIT);
              break;
            }
          }

          if (context_p->token.type == LEXER_EOS && stack_top == SCAN_STACK_SCRIPT_FUNCTION)
          {
            /* End of argument parsing. */
            scanner_info_t *scanner_info_p = (scanner_info_t *) scanner_malloc (context_p, sizeof (scanner_info_t));
            scanner_info_p->next_p = context_p->next_scanner_info_p;
            scanner_info_p->source_p = NULL;
            scanner_info_p->type = SCANNER_TYPE_END_ARGUMENTS;
            scanner_context.end_arguments_p = scanner_info_p;

            context_p->next_scanner_info_p = scanner_info_p;
            context_p->source_p = context_p->source_start_p;
            context_p->source_end_p = context_p->source_start_p + context_p->source_size;
            lexer_init_line_info (context_p);

            scanner_filter_arguments (context_p, &scanner_context);
            lexer_next_token (context_p);
            scanner_check_directives (context_p, &scanner_context);
            continue;
          }

          if (context_p->token.type != LEXER_RIGHT_PAREN)
          {
            scanner_raise_error (context_p);
          }

          lexer_next_token (context_p);

          if (context_p->token.type != LEXER_LEFT_BRACE)
          {
            scanner_raise_error (context_p);
          }

          scanner_filter_arguments (context_p, &scanner_context);
          lexer_next_token (context_p);
          scanner_check_directives (context_p, &scanner_context);
          continue;
        }
        case SCAN_MODE_PROPERTY_NAME:
        {
          JERRY_ASSERT (stack_top == SCAN_STACK_OBJECT_LITERAL);

          if (lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS))
          {
            lexer_check_property_modifier (context_p);
          }

          if (context_p->token.type == LEXER_LEFT_SQUARE)
          {
            parser_stack_push_uint8 (context_p, SCAN_STACK_COMPUTED_PROPERTY);
            scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;
            break;
          }

          if (context_p->token.type == LEXER_THREE_DOTS)
          {
            *parser_stack_get_prev_uint8 (context_p) |= SCANNER_LITERAL_OBJECT_HAS_REST;
            scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;

            if (scanner_context.binding_type != SCANNER_BINDING_NONE)
            {
              scanner_context.mode = SCAN_MODE_BINDING;
            }
            break;
          }

          if (context_p->token.type == LEXER_RIGHT_BRACE)
          {
            scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
            continue;
          }

          if (context_p->token.type == LEXER_PROPERTY_GETTER || context_p->token.type == LEXER_KEYW_ASYNC
              || context_p->token.type == LEXER_MULTIPLY || context_p->token.type == LEXER_PROPERTY_SETTER)
          {
            uint16_t literal_pool_flags = SCANNER_LITERAL_POOL_FUNCTION;

            if (context_p->token.type == LEXER_MULTIPLY)
            {
              literal_pool_flags |= SCANNER_LITERAL_POOL_GENERATOR;
            }
            else if (context_p->token.type == LEXER_KEYW_ASYNC)
            {
              literal_pool_flags |= SCANNER_LITERAL_POOL_ASYNC;

              if (lexer_consume_generator (context_p))
              {
                literal_pool_flags |= SCANNER_LITERAL_POOL_GENERATOR;
              }
            }

            parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PROPERTY);
            lexer_scan_identifier (context_p, LEXER_PARSE_NO_OPTS);

            if (context_p->token.type == LEXER_LEFT_SQUARE)
            {
              parser_stack_push_uint8 (context_p, SCANNER_FROM_LITERAL_POOL_TO_COMPUTED (literal_pool_flags));
              scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;
              break;
            }

            if (context_p->token.type != LEXER_LITERAL)
            {
              scanner_raise_error (context_p);
            }

            scanner_push_literal_pool (context_p, &scanner_context, literal_pool_flags);
            scanner_context.mode = SCAN_MODE_FUNCTION_ARGUMENTS;
            break;
          }

          if (context_p->token.type != LEXER_LITERAL)
          {
            scanner_raise_error (context_p);
          }

          parser_line_counter_t start_line = context_p->token.line;
          parser_line_counter_t start_column = context_p->token.column;
          bool is_ident = (context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

          lexer_next_token (context_p);

          if (context_p->token.type == LEXER_LEFT_PAREN)
          {
            scanner_push_literal_pool (context_p, &scanner_context, SCANNER_LITERAL_POOL_FUNCTION);

            parser_stack_push_uint8 (context_p, SCAN_STACK_FUNCTION_PROPERTY);
            scanner_context.mode = SCAN_MODE_FUNCTION_ARGUMENTS;
            continue;
          }

          if (is_ident
              && (context_p->token.type == LEXER_COMMA || context_p->token.type == LEXER_RIGHT_BRACE
                  || context_p->token.type == LEXER_ASSIGN))
          {
            context_p->source_p = context_p->token.lit_location.char_p;
            context_p->line = start_line;
            context_p->column = start_column;

            lexer_next_token (context_p);

            JERRY_ASSERT (context_p->token.type != LEXER_LITERAL
                          || context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

            if (context_p->token.type != LEXER_LITERAL)
            {
              scanner_raise_error (context_p);
            }

            if (scanner_context.binding_type != SCANNER_BINDING_NONE)
            {
              scanner_context.mode = SCAN_MODE_BINDING;
              continue;
            }

            scanner_add_reference (context_p, &scanner_context);

            lexer_next_token (context_p);

            if (context_p->token.type == LEXER_ASSIGN)
            {
              scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;
              break;
            }

            scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION_END;
            continue;
          }

          if (context_p->token.type != LEXER_COLON)
          {
            scanner_raise_error (context_p);
          }

          scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;

          if (scanner_context.binding_type != SCANNER_BINDING_NONE)
          {
            scanner_context.mode = SCAN_MODE_BINDING;
          }
          break;
        }
        case SCAN_MODE_BINDING:
        {
          JERRY_ASSERT (scanner_context.binding_type == SCANNER_BINDING_VAR
                        || scanner_context.binding_type == SCANNER_BINDING_LET
                        || scanner_context.binding_type == SCANNER_BINDING_CATCH
                        || scanner_context.binding_type == SCANNER_BINDING_CONST
                        || scanner_context.binding_type == SCANNER_BINDING_ARG
                        || scanner_context.binding_type == SCANNER_BINDING_ARROW_ARG);

          if (type == LEXER_THREE_DOTS)
          {
            lexer_next_token (context_p);
            type = (lexer_token_type_t) context_p->token.type;
          }

          if (type == LEXER_LEFT_SQUARE || type == LEXER_LEFT_BRACE)
          {
            scanner_push_destructuring_pattern (context_p, &scanner_context, scanner_context.binding_type, true);

            if (type == LEXER_LEFT_SQUARE)
            {
              parser_stack_push_uint8 (context_p, SCAN_STACK_ARRAY_LITERAL);
              break;
            }

            parser_stack_push_uint8 (context_p, 0);
            parser_stack_push_uint8 (context_p, SCAN_STACK_OBJECT_LITERAL);
            scanner_context.mode = SCAN_MODE_PROPERTY_NAME;
            continue;
          }

          if (type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
          {
            scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;
            continue;
          }

          lexer_lit_location_t *literal_p = scanner_add_literal (context_p, &scanner_context);

          scanner_context.mode = SCAN_MODE_POST_PRIMARY_EXPRESSION;

          if (scanner_context.binding_type == SCANNER_BINDING_VAR)
          {
            if (!(literal_p->type & SCANNER_LITERAL_IS_VAR))
            {
              scanner_detect_invalid_var (context_p, &scanner_context, literal_p);
              literal_p->type |= SCANNER_LITERAL_IS_VAR;

              if (scanner_context.active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_WITH)
              {
                literal_p->type |= SCANNER_LITERAL_NO_REG;
              }
            }
            break;
          }

          if (scanner_context.binding_type == SCANNER_BINDING_ARROW_ARG)
          {
            literal_p->type |= SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_ARROW_DESTRUCTURED_ARG;

            if (literal_p->type & SCANNER_LITERAL_IS_USED)
            {
              literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
              break;
            }
          }
          else
          {
            scanner_detect_invalid_let (context_p, literal_p);

            if (scanner_context.binding_type <= SCANNER_BINDING_CATCH)
            {
              JERRY_ASSERT ((scanner_context.binding_type == SCANNER_BINDING_LET)
                            || (scanner_context.binding_type == SCANNER_BINDING_CATCH));

              literal_p->type |= SCANNER_LITERAL_IS_LET;
            }
            else
            {
              literal_p->type |= SCANNER_LITERAL_IS_CONST;

              if (scanner_context.binding_type == SCANNER_BINDING_ARG)
              {
                literal_p->type |= SCANNER_LITERAL_IS_ARG;

                if (literal_p->type & SCANNER_LITERAL_IS_USED)
                {
                  literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
                  break;
                }
              }
            }

            if (literal_p->type & SCANNER_LITERAL_IS_USED)
            {
              literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
              break;
            }
          }

          scanner_binding_item_t *binding_item_p;
          binding_item_p = (scanner_binding_item_t *) scanner_malloc (context_p, sizeof (scanner_binding_item_t));

          binding_item_p->next_p = scanner_context.active_binding_list_p->items_p;
          binding_item_p->literal_p = literal_p;

          scanner_context.active_binding_list_p->items_p = binding_item_p;

          lexer_next_token (context_p);
          if (context_p->token.type != LEXER_ASSIGN)
          {
            continue;
          }

          scanner_binding_literal_t binding_literal;
          binding_literal.literal_p = literal_p;

          parser_stack_push (context_p, &binding_literal, sizeof (scanner_binding_literal_t));
          parser_stack_push_uint8 (context_p, SCAN_STACK_BINDING_INIT);

          scanner_context.mode = SCAN_MODE_PRIMARY_EXPRESSION;
          break;
        }
      }

      lexer_next_token (context_p);
    }

scan_completed:
    if (context_p->stack_top_uint8 != SCAN_STACK_SCRIPT && context_p->stack_top_uint8 != SCAN_STACK_SCRIPT_FUNCTION)
    {
      scanner_raise_error (context_p);
    }

    scanner_pop_literal_pool (context_p, &scanner_context);

    JERRY_ASSERT (scanner_context.active_binding_list_p == NULL);
    JERRY_ASSERT (scanner_context.active_literal_pool_p == NULL);

#ifndef JERRY_NDEBUG
    scanner_context.context_status_flags |= PARSER_SCANNING_SUCCESSFUL;
#endif /* !JERRY_NDEBUG */
  }
  PARSER_CATCH
  {
    while (scanner_context.active_binding_list_p != NULL)
    {
      scanner_pop_binding_list (&scanner_context);
    }

    if (JERRY_UNLIKELY (context_p->error != PARSER_ERR_OUT_OF_MEMORY))
    {
      /* Ignore the errors thrown by the lexer. */
      context_p->error = PARSER_ERR_NO_ERROR;

      /* The following code may allocate memory, so it is enclosed in a try/catch. */
      PARSER_TRY (context_p->try_buffer)
      {
        if (scanner_context.status_flags & SCANNER_CONTEXT_THROW_ERR_ASYNC_FUNCTION)
        {
          JERRY_ASSERT (scanner_context.async_source_p != NULL);

          scanner_info_t *info_p;
          info_p = scanner_insert_info (context_p, scanner_context.async_source_p, sizeof (scanner_info_t));
          info_p->type = SCANNER_TYPE_ERR_ASYNC_FUNCTION;
        }

        while (scanner_context.active_literal_pool_p != NULL)
        {
          scanner_pop_literal_pool (context_p, &scanner_context);
        }
      }
      PARSER_CATCH
      {
        JERRY_ASSERT (context_p->error == PARSER_ERR_OUT_OF_MEMORY);
      }
      PARSER_TRY_END
    }

    JERRY_ASSERT (context_p->error == PARSER_ERR_NO_ERROR || context_p->error == PARSER_ERR_OUT_OF_MEMORY);

    if (context_p->error == PARSER_ERR_OUT_OF_MEMORY)
    {
      while (scanner_context.active_literal_pool_p != NULL)
      {
        scanner_literal_pool_t *literal_pool_p = scanner_context.active_literal_pool_p;

        scanner_context.active_literal_pool_p = literal_pool_p->prev_p;

        parser_list_free (&literal_pool_p->literal_pool);
        scanner_free (literal_pool_p, sizeof (scanner_literal_pool_t));
      }

      parser_stack_free (context_p);
      return;
    }
  }
  PARSER_TRY_END

  context_p->status_flags = scanner_context.context_status_flags;
  context_p->global_status_flags &= (uint32_t) ~ECMA_PARSE_INTERNAL_PRE_SCANNING;
  scanner_reverse_info_list (context_p);

#if JERRY_PARSER_DUMP_BYTE_CODE
  if (context_p->is_show_opcodes)
  {
    scanner_info_t *info_p = context_p->next_scanner_info_p;
    const uint8_t *source_start_p =
      (context_p->arguments_start_p == NULL ? context_p->source_start_p : context_p->arguments_start_p);

    while (info_p->type != SCANNER_TYPE_END)
    {
      const char *name_p = NULL;
      bool print_location = false;

      switch (info_p->type)
      {
        case SCANNER_TYPE_END_ARGUMENTS:
        {
          JERRY_DEBUG_MSG ("  END_ARGUMENTS\n");
          source_start_p = context_p->source_start_p;
          break;
        }
        case SCANNER_TYPE_FUNCTION:
        case SCANNER_TYPE_BLOCK:
        {
          const uint8_t *prev_source_p = info_p->source_p - 1;
          const uint8_t *data_p;

          if (info_p->type == SCANNER_TYPE_FUNCTION)
          {
            data_p = (const uint8_t *) (info_p + 1);

            JERRY_DEBUG_MSG ("  FUNCTION: flags: 0x%x declarations: %d", (int) info_p->u8_arg, (int) info_p->u16_arg);
          }
          else
          {
            data_p = (const uint8_t *) (info_p + 1);

            JERRY_DEBUG_MSG ("  BLOCK:");
          }

          JERRY_DEBUG_MSG (" source:%d\n", (int) (info_p->source_p - source_start_p));

          while (data_p[0] != SCANNER_STREAM_TYPE_END)
          {
            switch (data_p[0] & SCANNER_STREAM_TYPE_MASK)
            {
              case SCANNER_STREAM_TYPE_HOLE:
              {
                JERRY_DEBUG_MSG ("    HOLE\n");
                data_p++;
                continue;
              }
              case SCANNER_STREAM_TYPE_ARGUMENTS:
              {
                JERRY_DEBUG_MSG ("    ARGUMENTS%s%s\n",
                                 (data_p[0] & SCANNER_STREAM_NO_REG) ? " *" : "",
                                 (data_p[0] & SCANNER_STREAM_LOCAL_ARGUMENTS) ? " L" : "");
                data_p++;
                continue;
              }
              case SCANNER_STREAM_TYPE_ARGUMENTS_FUNC:
              {
                JERRY_DEBUG_MSG ("    ARGUMENTS_FUNC%s%s\n",
                                 (data_p[0] & SCANNER_STREAM_NO_REG) ? " *" : "",
                                 (data_p[0] & SCANNER_STREAM_LOCAL_ARGUMENTS) ? " L" : "");
                data_p++;
                continue;
              }
              case SCANNER_STREAM_TYPE_VAR:
              {
                JERRY_DEBUG_MSG ("    VAR ");
                break;
              }
              case SCANNER_STREAM_TYPE_LET:
              {
                JERRY_DEBUG_MSG ("    LET ");
                break;
              }
              case SCANNER_STREAM_TYPE_CONST:
              {
                JERRY_DEBUG_MSG ("    CONST ");
                break;
              }
              case SCANNER_STREAM_TYPE_LOCAL:
              {
                JERRY_DEBUG_MSG ("    LOCAL ");
                break;
              }
#if JERRY_MODULE_SYSTEM
              case SCANNER_STREAM_TYPE_IMPORT:
              {
                JERRY_DEBUG_MSG ("    IMPORT ");
                break;
              }
#endif /* JERRY_MODULE_SYSTEM */
              case SCANNER_STREAM_TYPE_ARG:
              {
                JERRY_DEBUG_MSG ("    ARG ");
                break;
              }
              case SCANNER_STREAM_TYPE_ARG_VAR:
              {
                JERRY_DEBUG_MSG ("    ARG_VAR ");
                break;
              }
              case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG:
              {
                JERRY_DEBUG_MSG ("    DESTRUCTURED_ARG ");
                break;
              }
              case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR:
              {
                JERRY_DEBUG_MSG ("    DESTRUCTURED_ARG_VAR ");
                break;
              }
              case SCANNER_STREAM_TYPE_ARG_FUNC:
              {
                JERRY_DEBUG_MSG ("    ARG_FUNC ");
                break;
              }
              case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC:
              {
                JERRY_DEBUG_MSG ("    DESTRUCTURED_ARG_FUNC ");
                break;
              }
              case SCANNER_STREAM_TYPE_FUNC:
              {
                JERRY_DEBUG_MSG ("    FUNC ");
                break;
              }
              default:
              {
                JERRY_UNREACHABLE ();
                data_p++;
                continue;
              }
            }

            size_t length;

            if (!(data_p[0] & SCANNER_STREAM_UINT16_DIFF))
            {
              if (data_p[2] != 0)
              {
                prev_source_p += data_p[2];
                length = 2 + 1;
              }
              else
              {
                memcpy (&prev_source_p, data_p + 2 + 1, sizeof (uintptr_t));
                length = 2 + 1 + sizeof (uintptr_t);
              }
            }
            else
            {
              int32_t diff = ((int32_t) data_p[2]) | ((int32_t) data_p[3]) << 8;

              if (diff <= UINT8_MAX)
              {
                diff = -diff;
              }

              prev_source_p += diff;
              length = 2 + 2;
            }

            if (data_p[0] & SCANNER_STREAM_EARLY_CREATE)
            {
              JERRY_ASSERT (data_p[0] & SCANNER_STREAM_NO_REG);
              JERRY_DEBUG_MSG ("*");
            }

            if (data_p[0] & SCANNER_STREAM_NO_REG)
            {
              JERRY_DEBUG_MSG ("* ");
            }

            JERRY_DEBUG_MSG ("'%.*s'\n", data_p[1], (char *) prev_source_p);
            prev_source_p += data_p[1];
            data_p += length;
          }
          break;
        }
        case SCANNER_TYPE_WHILE:
        {
          name_p = "WHILE";
          print_location = true;
          break;
        }
        case SCANNER_TYPE_FOR:
        {
          scanner_for_info_t *for_info_p = (scanner_for_info_t *) info_p;
          JERRY_DEBUG_MSG ("  FOR: source:%d expression:%d[%d:%d] end:%d[%d:%d]\n",
                           (int) (for_info_p->info.source_p - source_start_p),
                           (int) (for_info_p->expression_location.source_p - source_start_p),
                           (int) for_info_p->expression_location.line,
                           (int) for_info_p->expression_location.column,
                           (int) (for_info_p->end_location.source_p - source_start_p),
                           (int) for_info_p->end_location.line,
                           (int) for_info_p->end_location.column);
          break;
        }
        case SCANNER_TYPE_FOR_IN:
        {
          name_p = "FOR-IN";
          print_location = true;
          break;
        }
        case SCANNER_TYPE_FOR_OF:
        {
          name_p = "FOR-OF";
          print_location = true;
          break;
        }
        case SCANNER_TYPE_SWITCH:
        {
          JERRY_DEBUG_MSG ("  SWITCH: source:%d\n", (int) (info_p->source_p - source_start_p));

          scanner_case_info_t *current_case_p = ((scanner_switch_info_t *) info_p)->case_p;

          while (current_case_p != NULL)
          {
            JERRY_DEBUG_MSG ("    CASE: location:%d[%d:%d]\n",
                             (int) (current_case_p->location.source_p - source_start_p),
                             (int) current_case_p->location.line,
                             (int) current_case_p->location.column);

            current_case_p = current_case_p->next_p;
          }
          break;
        }
        case SCANNER_TYPE_CASE:
        {
          name_p = "CASE";
          print_location = true;
          break;
        }
        case SCANNER_TYPE_INITIALIZER:
        {
          scanner_location_info_t *location_info_p = (scanner_location_info_t *) info_p;
          JERRY_DEBUG_MSG ("  INITIALIZER: flags: 0x%x source:%d location:%d[%d:%d]\n",
                           (int) info_p->u8_arg,
                           (int) (location_info_p->info.source_p - source_start_p),
                           (int) (location_info_p->location.source_p - source_start_p),
                           (int) location_info_p->location.line,
                           (int) location_info_p->location.column);
          break;
        }
        case SCANNER_TYPE_CLASS_CONSTRUCTOR:
        {
          JERRY_DEBUG_MSG ("  CLASS: source:%d\n", (int) (info_p->source_p - source_start_p));
          print_location = false;
          break;
        }
        case SCANNER_TYPE_CLASS_STATIC_BLOCK_END:
        {
          name_p = "SCANNER_TYPE_CLASS_STATIC_BLOCK_END";
          print_location = true;
          break;
        }
        case SCANNER_TYPE_CLASS_FIELD_INITIALIZER_END:
        {
          name_p = "SCANNER_TYPE_CLASS_FIELD_INITIALIZER_END";
          print_location = true;
          break;
        }
        case SCANNER_TYPE_LET_EXPRESSION:
        {
          JERRY_DEBUG_MSG ("  LET_EXPRESSION: source:%d\n", (int) (info_p->source_p - source_start_p));
          break;
        }
        case SCANNER_TYPE_ERR_REDECLARED:
        {
          JERRY_DEBUG_MSG ("  JERRY_FATAL_REDECLARED: source:%d\n", (int) (info_p->source_p - source_start_p));
          break;
        }
        case SCANNER_TYPE_ERR_ASYNC_FUNCTION:
        {
          JERRY_DEBUG_MSG ("  JERRY_FATAL_ASYNC_FUNCTION: source:%d\n", (int) (info_p->source_p - source_start_p));
          break;
        }
        case SCANNER_TYPE_LITERAL_FLAGS:
        {
          JERRY_DEBUG_MSG ("  SCANNER_TYPE_LITERAL_FLAGS: flags: 0x%x source:%d\n",
                           (int) info_p->u8_arg,
                           (int) (info_p->source_p - source_start_p));
          print_location = false;
          break;
        }
        case SCANNER_TYPE_EXPORT_MODULE_SPECIFIER:
        {
          JERRY_DEBUG_MSG ("  EXPORT_WITH_MODULE_SPECIFIER: source:%d\n", (int) (info_p->source_p - source_start_p));
          print_location = false;
          break;
        }
      }

      if (print_location)
      {
        scanner_location_info_t *location_info_p = (scanner_location_info_t *) info_p;
        JERRY_DEBUG_MSG ("  %s: source:%d location:%d[%d:%d]\n",
                         name_p,
                         (int) (location_info_p->info.source_p - source_start_p),
                         (int) (location_info_p->location.source_p - source_start_p),
                         (int) location_info_p->location.line,
                         (int) location_info_p->location.column);
      }

      info_p = info_p->next_p;
    }

    JERRY_DEBUG_MSG ("\n--- Scanning end ---\n\n");
  }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

  parser_stack_free (context_p);
} /* scanner_scan_all */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_PARSER */
