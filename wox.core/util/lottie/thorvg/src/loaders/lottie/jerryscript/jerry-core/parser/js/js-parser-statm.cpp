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
#include "lit-char-helpers.h"

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_stmt Statement parser
 * @{
 */

/**
 * Parser statement types.
 *
 * When a new statement is added, the following
 * arrays must be updated as well:
 *  - statement_lengths[]
 *  - parser_statement_flags[]
 */
typedef enum
{
  PARSER_STATEMENT_START,
  PARSER_STATEMENT_BLOCK,
  PARSER_STATEMENT_BLOCK_SCOPE,
  PARSER_STATEMENT_PRIVATE_SCOPE,
  PARSER_STATEMENT_BLOCK_CONTEXT,
  PARSER_STATEMENT_PRIVATE_CONTEXT,
  PARSER_STATEMENT_LABEL,
  PARSER_STATEMENT_IF,
  PARSER_STATEMENT_ELSE,
  PARSER_STATEMENT_SWITCH,
  PARSER_STATEMENT_SWITCH_NO_DEFAULT,
  PARSER_STATEMENT_DO_WHILE,
  PARSER_STATEMENT_WHILE,
  PARSER_STATEMENT_FOR,
  PARSER_STATEMENT_FOR_IN,
  PARSER_STATEMENT_FOR_OF,
  PARSER_STATEMENT_FOR_AWAIT_OF,
  PARSER_STATEMENT_WITH,
  PARSER_STATEMENT_TRY,
} parser_statement_type_t;

/**
 * Parser statement type flags.
 */
typedef enum
{
  PARSER_STATM_NO_OPTS = 0, /**< no options */
  PARSER_STATM_SINGLE_STATM = (1 << 0), /**< statement can form single statement context */
  PARSER_STATM_HAS_BLOCK = (1 << 1), /**< statement always has a code block */
  PARSER_STATM_BREAK_TARGET = (1 << 2), /**< break target statement */
  PARSER_STATM_CONTINUE_TARGET = (1 << 3), /**< continue target statement */
  PARSER_STATM_CONTEXT_BREAK = (1 << 4), /**< uses another instruction form when crosses their borders */
} parser_statement_flags_t;

/**
 * Parser statement attributes.
 * Note: the order of the attributes must be keep in sync with parser_statement_type_t
 */
static const uint8_t parser_statement_flags[] = {
  /* PARSER_STATEMENT_START */
  PARSER_STATM_HAS_BLOCK,
  /* PARSER_STATEMENT_BLOCK, */
  PARSER_STATM_HAS_BLOCK,
  /* PARSER_STATEMENT_BLOCK_SCOPE, */
  PARSER_STATM_HAS_BLOCK,
  /* PARSER_STATEMENT_PRIVATE_SCOPE, */
  PARSER_STATM_NO_OPTS,
  /* PARSER_STATEMENT_BLOCK_CONTEXT, */
  PARSER_STATM_HAS_BLOCK | PARSER_STATM_CONTEXT_BREAK,
  /* PARSER_STATEMENT_PRIVATE_CONTEXT, */
  PARSER_STATM_CONTEXT_BREAK,
  /* PARSER_STATEMENT_LABEL */
  PARSER_STATM_SINGLE_STATM,
  /* PARSER_STATEMENT_IF */
  PARSER_STATM_SINGLE_STATM,
  /* PARSER_STATEMENT_ELSE */
  PARSER_STATM_SINGLE_STATM,
  /* PARSER_STATEMENT_SWITCH */
  PARSER_STATM_HAS_BLOCK | PARSER_STATM_BREAK_TARGET,
  /* PARSER_STATEMENT_SWITCH_NO_DEFAULT */
  PARSER_STATM_HAS_BLOCK | PARSER_STATM_BREAK_TARGET,
  /* PARSER_STATEMENT_DO_WHILE */
  PARSER_STATM_BREAK_TARGET | PARSER_STATM_CONTINUE_TARGET | PARSER_STATM_SINGLE_STATM,
  /* PARSER_STATEMENT_WHILE */
  PARSER_STATM_BREAK_TARGET | PARSER_STATM_CONTINUE_TARGET | PARSER_STATM_SINGLE_STATM,
  /* PARSER_STATEMENT_FOR */
  PARSER_STATM_BREAK_TARGET | PARSER_STATM_CONTINUE_TARGET | PARSER_STATM_SINGLE_STATM,
  /* PARSER_STATEMENT_FOR_IN */
  PARSER_STATM_BREAK_TARGET | PARSER_STATM_CONTINUE_TARGET | PARSER_STATM_SINGLE_STATM | PARSER_STATM_CONTEXT_BREAK,
  /* PARSER_STATEMENT_FOR_OF */
  PARSER_STATM_BREAK_TARGET | PARSER_STATM_CONTINUE_TARGET | PARSER_STATM_SINGLE_STATM | PARSER_STATM_CONTEXT_BREAK,
  /* PARSER_STATEMENT_FOR_AWAIT_OF */
  PARSER_STATM_BREAK_TARGET | PARSER_STATM_CONTINUE_TARGET | PARSER_STATM_SINGLE_STATM | PARSER_STATM_CONTEXT_BREAK,
  /* PARSER_STATEMENT_WITH */
  PARSER_STATM_CONTEXT_BREAK | PARSER_STATM_SINGLE_STATM,
  /* PARSER_STATEMENT_TRY */
  PARSER_STATM_HAS_BLOCK | PARSER_STATM_CONTEXT_BREAK
};

/**
 * Block statement.
 */
typedef struct
{
  uint16_t scope_stack_top; /**< preserved top of scope stack */
  uint16_t scope_stack_reg_top; /**< preserved top register of scope stack */
} parser_block_statement_t;

/**
 * Context of block statement.
 */
typedef struct
{
  parser_branch_t branch; /**< branch to the end */
} parser_block_context_t;

/**
 * Loop statement.
 */
typedef struct
{
  parser_branch_node_t *branch_list_p; /**< list of breaks and continues targeting this statement */
} parser_loop_statement_t;

/**
 * Label statement.
 */
typedef struct
{
  lexer_lit_location_t label_ident; /**< name of the label */
  parser_branch_node_t *break_list_p; /**< list of breaks targeting this label */
} parser_label_statement_t;

/**
 * If/else statement.
 */
typedef struct
{
  parser_branch_t branch; /**< branch to the end */
} parser_if_else_statement_t;

/**
 * Switch statement.
 */
typedef struct
{
  parser_branch_t default_branch; /**< branch to the default case */
  parser_branch_node_t *branch_list_p; /**< branches of case statements */
} parser_switch_statement_t;

/**
 * Do-while statement.
 */
typedef struct
{
  uint32_t start_offset; /**< start byte code offset */
} parser_do_while_statement_t;

/**
 * While statement.
 */
typedef struct
{
  parser_branch_t branch; /**< branch to the end */
  scanner_location_t condition_location; /**< condition part */
  uint32_t start_offset; /**< start byte code offset */
} parser_while_statement_t;

/**
 * For statement.
 */
typedef struct
{
  parser_branch_t branch; /**< branch to the end */
  scanner_location_t condition_location; /**< condition part */
  scanner_location_t expression_location; /**< expression part */
  uint32_t start_offset; /**< start byte code offset */
} parser_for_statement_t;

/**
 * For-in statement.
 */
typedef struct
{
  parser_branch_t branch; /**< branch to the end */
  uint32_t start_offset; /**< start byte code offset */
} parser_for_in_of_statement_t;

/**
 * With statement.
 */
typedef struct
{
  parser_branch_t branch; /**< branch to the end */
} parser_with_statement_t;

/**
 * Lexer token types.
 */
typedef enum
{
  parser_try_block, /**< try block */
  parser_catch_block, /**< catch block */
  parser_finally_block, /**< finally block */
} parser_try_block_type_t;

/**
 * Try statement.
 */
typedef struct
{
  parser_try_block_type_t type; /**< current block type */
  uint16_t scope_stack_top; /**< current top of scope stack */
  uint16_t scope_stack_reg_top; /**< current top register of scope stack */
  parser_branch_t branch; /**< branch to the end of the current block */
} parser_try_statement_t;

/**
 * Returns the data consumed by a statement. It can be used
 * to skip undesired frames on the stack during frame search.
 *
 * @return size consumed by a statement.
 */
static inline size_t
parser_statement_length (uint8_t type) /**< type of statement */
{
  static const uint8_t statement_lengths[] = {
    /* PARSER_STATEMENT_BLOCK */
    1,
    /* PARSER_STATEMENT_BLOCK_SCOPE */
    (uint8_t) (sizeof (parser_block_statement_t) + 1),
    /* PARSER_STATEMENT_PRIVATE_SCOPE */
    (uint8_t) (sizeof (parser_block_statement_t) + 1),
    /* PARSER_STATEMENT_BLOCK_CONTEXT */
    (uint8_t) (sizeof (parser_block_statement_t) + sizeof (parser_block_context_t) + 1),
    /* PARSER_STATEMENT_PRIVATE_CONTEXT */
    (uint8_t) (sizeof (parser_block_statement_t) + sizeof (parser_block_context_t) + 1),
    /* PARSER_STATEMENT_LABEL */
    (uint8_t) (sizeof (parser_label_statement_t) + 1),
    /* PARSER_STATEMENT_IF */
    (uint8_t) (sizeof (parser_if_else_statement_t) + 1),
    /* PARSER_STATEMENT_ELSE */
    (uint8_t) (sizeof (parser_if_else_statement_t) + 1),
    /* PARSER_STATEMENT_SWITCH */
    (uint8_t) (sizeof (parser_switch_statement_t) + sizeof (parser_loop_statement_t) + 1),
    /* PARSER_STATEMENT_SWITCH_NO_DEFAULT */
    (uint8_t) (sizeof (parser_switch_statement_t) + sizeof (parser_loop_statement_t) + 1),
    /* PARSER_STATEMENT_DO_WHILE */
    (uint8_t) (sizeof (parser_do_while_statement_t) + sizeof (parser_loop_statement_t) + 1),
    /* PARSER_STATEMENT_WHILE */
    (uint8_t) (sizeof (parser_while_statement_t) + sizeof (parser_loop_statement_t) + 1),
    /* PARSER_STATEMENT_FOR */
    (uint8_t) (sizeof (parser_for_statement_t) + sizeof (parser_loop_statement_t) + 1),
    /* PARSER_STATEMENT_FOR_IN */
    (uint8_t) (sizeof (parser_for_in_of_statement_t) + sizeof (parser_loop_statement_t) + 1),
    /* PARSER_STATEMENT_FOR_OF */
    (uint8_t) (sizeof (parser_for_in_of_statement_t) + sizeof (parser_loop_statement_t) + 1),
    /* PARSER_STATEMENT_FOR_AWAIT_OF */
    (uint8_t) (sizeof (parser_for_in_of_statement_t) + sizeof (parser_loop_statement_t) + 1),
    /* PARSER_STATEMENT_WITH */
    (uint8_t) (sizeof (parser_with_statement_t) + 1 + 1),
    /* PARSER_STATEMENT_TRY */
    (uint8_t) (sizeof (parser_try_statement_t) + 1),
  };

  JERRY_ASSERT (type >= PARSER_STATEMENT_BLOCK && type <= PARSER_STATEMENT_TRY);

  return statement_lengths[type - PARSER_STATEMENT_BLOCK];
} /* parser_statement_length */
/**
 * Parse expression enclosed in parens.
 */
static inline void
parser_parse_enclosed_expr (parser_context_t *context_p) /**< context */
{
  lexer_next_token (context_p);

  if (context_p->token.type != LEXER_LEFT_PAREN)
  {
    parser_raise_error (context_p, PARSER_ERR_LEFT_PAREN_EXPECTED);
  }

  lexer_next_token (context_p);
  parser_parse_expression (context_p, PARSE_EXPR);

  if (context_p->token.type != LEXER_RIGHT_PAREN)
  {
    parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
  }
  lexer_next_token (context_p);
} /* parser_parse_enclosed_expr */

/**
 * Create a block context.
 *
 * @return true - when a context is created, false - otherwise
 */
static bool
parser_push_block_context (parser_context_t *context_p, /**< context */
                           bool is_private) /**< is private (bound to a statement) context */
{
  JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_BLOCK);

  parser_block_statement_t block_statement;
  block_statement.scope_stack_top = context_p->scope_stack_top;
  block_statement.scope_stack_reg_top = context_p->scope_stack_reg_top;

  bool is_context_needed = false;

  if (scanner_is_context_needed (context_p, PARSER_CHECK_BLOCK_CONTEXT))
  {
    parser_block_context_t block_context;

#ifndef JERRY_NDEBUG
    PARSER_PLUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_BLOCK_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

    parser_emit_cbc_forward_branch (context_p, CBC_BLOCK_CREATE_CONTEXT, &block_context.branch);
    parser_stack_push (context_p, &block_context, sizeof (parser_block_context_t));
    is_context_needed = true;
  }

  scanner_create_variables (context_p, SCANNER_CREATE_VARS_NO_OPTS);
  parser_stack_push (context_p, &block_statement, sizeof (parser_block_statement_t));

  uint8_t statement_type;

  if (is_private)
  {
    statement_type = (is_context_needed ? PARSER_STATEMENT_PRIVATE_CONTEXT : PARSER_STATEMENT_PRIVATE_SCOPE);
  }
  else
  {
    statement_type = (is_context_needed ? PARSER_STATEMENT_BLOCK_CONTEXT : PARSER_STATEMENT_BLOCK_SCOPE);
  }

  parser_stack_push_uint8 (context_p, statement_type);

  return is_context_needed;
} /* parser_push_block_context */

/**
 * Pop block context.
 */
static void
parser_pop_block_context (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (context_p->stack_top_uint8 == PARSER_STATEMENT_BLOCK_SCOPE
                || context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_SCOPE
                || context_p->stack_top_uint8 == PARSER_STATEMENT_BLOCK_CONTEXT
                || context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_CONTEXT);

  uint8_t type = context_p->stack_top_uint8;

  parser_block_statement_t block_statement;

  parser_stack_pop_uint8 (context_p);
  parser_stack_pop (context_p, &block_statement, sizeof (parser_block_statement_t));

  context_p->scope_stack_top = block_statement.scope_stack_top;
  context_p->scope_stack_reg_top = block_statement.scope_stack_reg_top;

  if (type == PARSER_STATEMENT_BLOCK_CONTEXT || type == PARSER_STATEMENT_PRIVATE_CONTEXT)
  {
    PARSER_MINUS_EQUAL_U16 (context_p->stack_depth, PARSER_BLOCK_CONTEXT_STACK_ALLOCATION);
#ifndef JERRY_NDEBUG
    PARSER_MINUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_BLOCK_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

    parser_block_context_t block_context;
    parser_stack_pop (context_p, &block_context, sizeof (parser_block_context_t));

    parser_emit_cbc (context_p, CBC_CONTEXT_END);
    parser_set_branch_to_current_position (context_p, &block_context.branch);
  }

  parser_stack_iterator_init (context_p, &context_p->last_statement);
} /* parser_pop_block_context */

/**
 * Validate lexical context for a declaration.
 */
static void
parser_validate_lexical_context (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_LET || context_p->token.type == LEXER_KEYW_CONST
                || context_p->token.type == LEXER_KEYW_CLASS);

  if (parser_statement_flags[context_p->stack_top_uint8] & PARSER_STATM_SINGLE_STATM)
  {
    parser_raise_error (context_p, PARSER_ERR_LEXICAL_SINGLE_STATEMENT);
  }
} /* parser_validate_lexical_context */

/**
 * Parse var statement.
 */
static void
parser_parse_var_statement (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_VAR || context_p->token.type == LEXER_KEYW_LET
                || context_p->token.type == LEXER_KEYW_CONST);

  uint8_t declaration_type = context_p->token.type;

  if (declaration_type != LEXER_KEYW_VAR)
  {
    parser_validate_lexical_context (context_p);
  }

  while (true)
  {
    if (lexer_check_next_characters (context_p, LIT_CHAR_LEFT_SQUARE, LIT_CHAR_LEFT_BRACE))
    {
      parser_pattern_flags_t flags = PARSER_PATTERN_BINDING;

      if (declaration_type == LEXER_KEYW_LET)
      {
        flags = (parser_pattern_flags_t) ((int) flags | (int) PARSER_PATTERN_LET);
      }
      else if (declaration_type == LEXER_KEYW_CONST)
      {
        flags = (parser_pattern_flags_t) ((int) flags | (int) PARSER_PATTERN_CONST);
      }

      parser_parse_initializer_by_next_char (context_p, flags);
    }
    else
    {
      lexer_expect_identifier (context_p, LEXER_IDENT_LITERAL);
      JERRY_ASSERT (context_p->token.type == LEXER_LITERAL
                    && context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

                    #if JERRY_MODULE_SYSTEM
      parser_module_append_export_name (context_p);
#endif /* JERRY_MODULE_SYSTEM */

      if (declaration_type != LEXER_KEYW_VAR && context_p->token.keyword_type == LEXER_KEYW_LET)
      {
        parser_raise_error (context_p, PARSER_ERR_LEXICAL_LET_BINDING);
      }

      if (context_p->next_scanner_info_p->source_p == context_p->source_p)
      {
        JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_ERR_REDECLARED);
        parser_raise_error (context_p, PARSER_ERR_VARIABLE_REDECLARED);
      }

      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_ASSIGN)
      {

        uint16_t index = context_p->lit_object.index;

        lexer_next_token (context_p);
        parser_parse_expression (context_p, PARSE_EXPR_NO_COMMA);

        cbc_opcode_t opcode = CBC_ASSIGN_SET_IDENT;

        uint16_t function_literal_index = parser_check_anonymous_function_declaration (context_p);

        if (function_literal_index == PARSER_ANONYMOUS_CLASS)
        {
          uint16_t name_index = scanner_save_literal (context_p, index);
          parser_emit_cbc_ext_literal (context_p, CBC_EXT_SET_CLASS_NAME, name_index);
        }
        else if (function_literal_index < PARSER_NAMED_FUNCTION)
        {
          parser_set_function_name (context_p, function_literal_index, index, 0);
        }

        if (declaration_type != LEXER_KEYW_VAR && (index < PARSER_REGISTER_START))
        {
          opcode = CBC_INIT_LET;

          if (scanner_literal_is_created (context_p, index))
          {
            opcode = CBC_ASSIGN_LET_CONST;
          }
          else if (declaration_type == LEXER_KEYW_CONST)
          {
            opcode = CBC_INIT_CONST;
          }
        }

        parser_emit_cbc_literal (context_p, (uint16_t) opcode, index);
      }
      else if (declaration_type == LEXER_KEYW_LET)
      {
        parser_emit_cbc (context_p, CBC_PUSH_UNDEFINED);

        uint16_t index = context_p->lit_object.index;
        cbc_opcode_t opcode = CBC_MOV_IDENT;

        if (index < PARSER_REGISTER_START)
        {
          opcode = (scanner_literal_is_created (context_p, index) ? CBC_ASSIGN_LET_CONST : CBC_INIT_LET);
        }

        parser_emit_cbc_literal (context_p, (uint16_t) opcode, index);
      }
      else if (declaration_type == LEXER_KEYW_CONST)
      {
        parser_raise_error (context_p, PARSER_ERR_MISSING_ASSIGN_AFTER_CONST);
      }
    }

    if (context_p->token.type != LEXER_COMMA)
    {
      break;
    }
  }

#if JERRY_MODULE_SYSTEM
  context_p->status_flags &= (uint32_t) ~(PARSER_MODULE_STORE_IDENT);
#endif /* JERRY_MODULE_SYSTEM */
} /* parser_parse_var_statement */

/**
 * Parse function statement.
 */
static void
parser_parse_function_statement (parser_context_t *context_p) /**< context */
{
  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_FUNCTION);

  bool is_single_statement = (parser_statement_flags[context_p->stack_top_uint8] & PARSER_STATM_SINGLE_STATM) != 0;

  if (JERRY_UNLIKELY (is_single_statement))
  {
    if (context_p->status_flags & PARSER_IS_STRICT)
    {
      parser_raise_error (context_p, PARSER_ERR_LEXICAL_SINGLE_STATEMENT);
    }

    if (context_p->stack_top_uint8 == PARSER_STATEMENT_IF || context_p->stack_top_uint8 == PARSER_STATEMENT_ELSE)
    {
      /* There must be a parser error later if this check fails. */
      if (context_p->next_scanner_info_p->source_p == context_p->source_p)
      {
        parser_push_block_context (context_p, true);
      }
    }
    else if (context_p->stack_top_uint8 == PARSER_STATEMENT_LABEL)
    {
      parser_stack_iterator_t iterator;
      parser_stack_iterator_init (context_p, &iterator);
      parser_stack_iterator_skip (&iterator, sizeof (parser_label_statement_t) + 1);

      while (true)
      {
        uint8_t type = parser_stack_iterator_read_uint8 (&iterator);

        if (type == PARSER_STATEMENT_LABEL)
        {
          parser_stack_iterator_skip (&iterator, sizeof (parser_label_statement_t) + 1);
          continue;
        }

        if (parser_statement_flags[type] & PARSER_STATM_HAS_BLOCK)
        {
          break;
        }

        parser_raise_error (context_p, PARSER_ERR_LABELLED_FUNC_NOT_IN_BLOCK);
      }
    }
    else
    {
      parser_raise_error (context_p, PARSER_ERR_LEXICAL_SINGLE_STATEMENT);
    }
  }
  bool is_generator_function = false;

  if (lexer_consume_generator (context_p))
  {
    if (is_single_statement)
    {
      parser_raise_error (context_p, PARSER_ERR_GENERATOR_IN_SINGLE_STATEMENT_POS);
    }
    is_generator_function = true;
  }

  lexer_expect_identifier (context_p, LEXER_NEW_IDENT_LITERAL);
  JERRY_ASSERT (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

  if (context_p->next_scanner_info_p->source_p == context_p->source_p
      && context_p->next_scanner_info_p->type == SCANNER_TYPE_ERR_REDECLARED)
  {
    parser_raise_error (context_p, PARSER_ERR_VARIABLE_REDECLARED);
  }

  uint16_t function_name_index = context_p->lit_object.index;

#if JERRY_MODULE_SYSTEM
  parser_module_append_export_name (context_p);
  context_p->status_flags &= (uint32_t) ~(PARSER_MODULE_STORE_IDENT);
#endif /* JERRY_MODULE_SYSTEM */

  uint32_t status_flags = PARSER_FUNCTION_CLOSURE;

  if (context_p->token.keyword_type >= LEXER_FIRST_NON_STRICT_ARGUMENTS)
  {
    status_flags |= PARSER_HAS_NON_STRICT_ARG;
  }

  if (is_generator_function)
  {
    status_flags |= PARSER_IS_GENERATOR_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;
  }

  if (context_p->next_scanner_info_p->u8_arg & SCANNER_FUNCTION_ASYNC)
  {
    status_flags |= PARSER_IS_ASYNC_FUNCTION | PARSER_DISALLOW_AWAIT_YIELD;
  }

  JERRY_ASSERT (context_p->scope_stack_top >= 2);
  parser_scope_stack_t *scope_stack_p = context_p->scope_stack_p + context_p->scope_stack_top - 2;

  uint16_t literal_index = context_p->lit_object.index;

  while (literal_index != scope_stack_p->map_from)
  {
    scope_stack_p--;

    JERRY_ASSERT (scope_stack_p >= context_p->scope_stack_p);
  }

  JERRY_ASSERT (scope_stack_p[1].map_from == PARSER_SCOPE_STACK_FUNC);

  if (!(context_p->status_flags & PARSER_IS_STRICT)
      && (scope_stack_p >= context_p->scope_stack_p + context_p->scope_stack_global_end))
  {
    bool copy_value = true;

    parser_scope_stack_t *stack_p = context_p->scope_stack_p;

    while (stack_p < scope_stack_p)
    {
      if (literal_index == stack_p->map_from && (stack_p->map_to & PARSER_SCOPE_STACK_NO_FUNCTION_COPY))
      {
        copy_value = false;
        break;
      }
      stack_p++;
    }

    if (copy_value)
    {
      stack_p = context_p->scope_stack_p;

      while (stack_p < scope_stack_p)
      {
        if (literal_index == stack_p->map_from)
        {
          JERRY_ASSERT (!(stack_p->map_to & PARSER_SCOPE_STACK_NO_FUNCTION_COPY));

          uint16_t map_to = scanner_decode_map_to (stack_p);
          uint16_t opcode = ((map_to >= PARSER_REGISTER_START) ? CBC_ASSIGN_LITERAL_SET_IDENT : CBC_COPY_TO_GLOBAL);

          parser_emit_cbc_literal_value (context_p, opcode, scanner_decode_map_to (scope_stack_p), map_to);
          break;
        }
        stack_p++;
      }

      parser_flush_cbc (context_p);
    }

    if (JERRY_UNLIKELY (context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_SCOPE
                        || context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_CONTEXT))
    {
      parser_pop_block_context (context_p);
    }
  }

  lexer_literal_t *literal_p = PARSER_GET_LITERAL ((size_t) scope_stack_p[1].map_to);

  JERRY_ASSERT ((literal_p->type == LEXER_UNUSED_LITERAL || literal_p->type == LEXER_FUNCTION_LITERAL)
                && literal_p->status_flags == 0);

  ecma_compiled_code_t *compiled_code_p = parser_parse_function (context_p, status_flags);

  if (literal_p->type == LEXER_FUNCTION_LITERAL)
  {
    ecma_bytecode_deref (literal_p->u.bytecode_p);
  }

  literal_p->u.bytecode_p = compiled_code_p;
  literal_p->type = LEXER_FUNCTION_LITERAL;

  parser_compiled_code_set_function_name (context_p, compiled_code_p, function_name_index, 0);
  lexer_next_token (context_p);
} /* parser_parse_function_statement */

/**
 * Parse if statement (starting part).
 */
static void
parser_parse_if_statement_start (parser_context_t *context_p) /**< context */
{
  parser_if_else_statement_t if_statement;

  parser_parse_enclosed_expr (context_p);

  parser_emit_cbc_forward_branch (context_p, CBC_BRANCH_IF_FALSE_FORWARD, &if_statement.branch);

  parser_stack_push (context_p, &if_statement, sizeof (parser_if_else_statement_t));
  parser_stack_push_uint8 (context_p, PARSER_STATEMENT_IF);
  parser_stack_iterator_init (context_p, &context_p->last_statement);
} /* parser_parse_if_statement_start */

/**
 * Parse if statement (ending part).
 *
 * @return true  - if parsing an 'else' statement
 *         false - otherwise
 */
static bool
parser_parse_if_statement_end (parser_context_t *context_p) /**< context */
{
  parser_if_else_statement_t if_statement;
  parser_if_else_statement_t else_statement;
  parser_stack_iterator_t iterator;

  JERRY_ASSERT (context_p->stack_top_uint8 == PARSER_STATEMENT_IF);

  if (context_p->token.type != LEXER_KEYW_ELSE)
  {
    parser_stack_pop_uint8 (context_p);
    parser_stack_pop (context_p, &if_statement, sizeof (parser_if_else_statement_t));
    parser_stack_iterator_init (context_p, &context_p->last_statement);

    parser_set_branch_to_current_position (context_p, &if_statement.branch);

    return false;
  }

  parser_stack_change_last_uint8 (context_p, PARSER_STATEMENT_ELSE);
  parser_stack_iterator_init (context_p, &iterator);
  parser_stack_iterator_skip (&iterator, 1);
  parser_stack_iterator_read (&iterator, &if_statement, sizeof (parser_if_else_statement_t));

  parser_emit_cbc_forward_branch (context_p, CBC_JUMP_FORWARD, &else_statement.branch);

  parser_set_branch_to_current_position (context_p, &if_statement.branch);

  parser_stack_iterator_write (&iterator, &else_statement, sizeof (parser_if_else_statement_t));

  lexer_next_token (context_p);
  return true;
} /* parser_parse_if_statement_end */

/**
 * Parse with statement (starting part).
 */
static void
parser_parse_with_statement_start (parser_context_t *context_p) /**< context */
{
  parser_with_statement_t with_statement;

  if (context_p->status_flags & PARSER_IS_STRICT)
  {
    parser_raise_error (context_p, PARSER_ERR_WITH_NOT_ALLOWED);
  }

  parser_parse_enclosed_expr (context_p);

#ifndef JERRY_NDEBUG
  PARSER_PLUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_WITH_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

  uint8_t inside_with = (context_p->status_flags & PARSER_INSIDE_WITH) != 0;

  context_p->status_flags |= PARSER_INSIDE_WITH;
  parser_emit_cbc_ext_forward_branch (context_p, CBC_EXT_WITH_CREATE_CONTEXT, &with_statement.branch);

  parser_stack_push (context_p, &with_statement, sizeof (parser_with_statement_t));
  parser_stack_push_uint8 (context_p, inside_with);
  parser_stack_push_uint8 (context_p, PARSER_STATEMENT_WITH);
  parser_stack_iterator_init (context_p, &context_p->last_statement);
} /* parser_parse_with_statement_start */

/**
 * Parse with statement (ending part).
 */
static void
parser_parse_with_statement_end (parser_context_t *context_p) /**< context */
{
  parser_with_statement_t with_statement;

  JERRY_ASSERT (context_p->status_flags & PARSER_INSIDE_WITH);

  parser_stack_pop_uint8 (context_p);

  if (!context_p->stack_top_uint8)
  {
    context_p->status_flags &= (uint32_t) ~PARSER_INSIDE_WITH;
  }

  parser_stack_pop_uint8 (context_p);
  parser_stack_pop (context_p, &with_statement, sizeof (parser_with_statement_t));
  parser_stack_iterator_init (context_p, &context_p->last_statement);

  parser_flush_cbc (context_p);
  PARSER_MINUS_EQUAL_U16 (context_p->stack_depth, PARSER_WITH_CONTEXT_STACK_ALLOCATION);
#ifndef JERRY_NDEBUG
  PARSER_MINUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_WITH_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

  parser_emit_cbc (context_p, CBC_CONTEXT_END);
  parser_set_branch_to_current_position (context_p, &with_statement.branch);
} /* parser_parse_with_statement_end */

/**
 * Parse do-while statement (ending part).
 */
static void
parser_parse_do_while_statement_end (parser_context_t *context_p) /**< context */
{
  parser_loop_statement_t loop;

  JERRY_ASSERT (context_p->stack_top_uint8 == PARSER_STATEMENT_DO_WHILE);

  if (context_p->token.type != LEXER_KEYW_WHILE)
  {
    parser_raise_error (context_p, PARSER_ERR_WHILE_EXPECTED);
  }

  parser_stack_iterator_t iterator;
  parser_stack_iterator_init (context_p, &iterator);

  parser_stack_iterator_skip (&iterator, 1);
  parser_stack_iterator_read (&iterator, &loop, sizeof (parser_loop_statement_t));

  parser_set_continues_to_current_position (context_p, loop.branch_list_p);

  JERRY_ASSERT (context_p->next_scanner_info_p->source_p != context_p->source_p);

  parser_parse_enclosed_expr (context_p);

  if (context_p->last_cbc_opcode != CBC_PUSH_FALSE)
  {
    cbc_opcode_t opcode = CBC_BRANCH_IF_TRUE_BACKWARD;
    if (context_p->last_cbc_opcode == CBC_LOGICAL_NOT)
    {
      context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
      opcode = CBC_BRANCH_IF_FALSE_BACKWARD;
    }
    else if (context_p->last_cbc_opcode == CBC_PUSH_TRUE)
    {
      context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
      opcode = CBC_JUMP_BACKWARD;
    }

    parser_do_while_statement_t do_while_statement;
    parser_stack_iterator_skip (&iterator, sizeof (parser_loop_statement_t));
    parser_stack_iterator_read (&iterator, &do_while_statement, sizeof (parser_do_while_statement_t));

    parser_emit_cbc_backward_branch (context_p, (uint16_t) opcode, do_while_statement.start_offset);
  }
  else
  {
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
  }

  parser_stack_pop (context_p, NULL, 1 + sizeof (parser_loop_statement_t) + sizeof (parser_do_while_statement_t));
  parser_stack_iterator_init (context_p, &context_p->last_statement);

  parser_set_breaks_to_current_position (context_p, loop.branch_list_p);
} /* parser_parse_do_while_statement_end */

/**
 * Parse while statement (starting part).
 */
static void
parser_parse_while_statement_start (parser_context_t *context_p) /**< context */
{
  parser_while_statement_t while_statement;
  parser_loop_statement_t loop;

  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_WHILE);
  lexer_next_token (context_p);

  if (context_p->token.type != LEXER_LEFT_PAREN)
  {
    parser_raise_error (context_p, PARSER_ERR_LEFT_PAREN_EXPECTED);
  }

  JERRY_ASSERT (context_p->next_scanner_info_p->source_p != context_p->source_p
                || context_p->next_scanner_info_p->type == SCANNER_TYPE_WHILE);

  if (context_p->next_scanner_info_p->source_p != context_p->source_p)
  {
    /* The prescanner couldn't find the end of the while condition. */
    lexer_next_token (context_p);
    parser_parse_expression (context_p, PARSE_EXPR);

    JERRY_ASSERT (context_p->token.type != LEXER_RIGHT_PAREN);
    parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
  }

  parser_emit_cbc_forward_branch (context_p, CBC_JUMP_FORWARD, &while_statement.branch);

  JERRY_ASSERT (context_p->last_cbc_opcode == PARSER_CBC_UNAVAILABLE);

  while_statement.start_offset = context_p->byte_code_size;
  scanner_get_location (&while_statement.condition_location, context_p);

  scanner_set_location (context_p, &((scanner_location_info_t *) context_p->next_scanner_info_p)->location);
  scanner_release_next (context_p, sizeof (scanner_location_info_t));
  scanner_seek (context_p);
  lexer_next_token (context_p);

  loop.branch_list_p = NULL;

  parser_stack_push (context_p, &while_statement, sizeof (parser_while_statement_t));
  parser_stack_push (context_p, &loop, sizeof (parser_loop_statement_t));
  parser_stack_push_uint8 (context_p, PARSER_STATEMENT_WHILE);
  parser_stack_iterator_init (context_p, &context_p->last_statement);
} /* parser_parse_while_statement_start */

/**
 * Parse while statement (ending part).
 */
static void JERRY_ATTR_NOINLINE
parser_parse_while_statement_end (parser_context_t *context_p) /**< context */
{
  parser_while_statement_t while_statement;
  parser_loop_statement_t loop;
  lexer_token_t current_token;
  scanner_location_t location;
  cbc_opcode_t opcode;

  JERRY_ASSERT (context_p->stack_top_uint8 == PARSER_STATEMENT_WHILE);

  parser_stack_iterator_t iterator;
  parser_stack_iterator_init (context_p, &iterator);

  parser_stack_iterator_skip (&iterator, 1);
  parser_stack_iterator_read (&iterator, &loop, sizeof (parser_loop_statement_t));
  parser_stack_iterator_skip (&iterator, sizeof (parser_loop_statement_t));
  parser_stack_iterator_read (&iterator, &while_statement, sizeof (parser_while_statement_t));

  scanner_get_location (&location, context_p);
  current_token = context_p->token;

  parser_set_branch_to_current_position (context_p, &while_statement.branch);
  parser_set_continues_to_current_position (context_p, loop.branch_list_p);

  scanner_set_location (context_p, &while_statement.condition_location);
  scanner_seek (context_p);
  lexer_next_token (context_p);

  parser_parse_expression (context_p, PARSE_EXPR);
  if (context_p->token.type != LEXER_RIGHT_PAREN)
  {
    parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
  }

  opcode = CBC_BRANCH_IF_TRUE_BACKWARD;
  if (context_p->last_cbc_opcode == CBC_LOGICAL_NOT)
  {
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
    opcode = CBC_BRANCH_IF_FALSE_BACKWARD;
  }
  else if (context_p->last_cbc_opcode == CBC_PUSH_TRUE)
  {
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
    opcode = CBC_JUMP_BACKWARD;
  }

  parser_stack_pop (context_p, NULL, 1 + sizeof (parser_loop_statement_t) + sizeof (parser_while_statement_t));
  parser_stack_iterator_init (context_p, &context_p->last_statement);

  parser_emit_cbc_backward_branch (context_p, (uint16_t) opcode, while_statement.start_offset);
  parser_set_breaks_to_current_position (context_p, loop.branch_list_p);

  /* Calling scanner_seek is unnecessary because all
   * info blocks inside the while statement should be processed. */
  scanner_set_location (context_p, &location);
  context_p->token = current_token;
} /* parser_parse_while_statement_end */

/**
 * Check whether the opcode is a valid LeftHandSide expression
 * and convert it back to an assignment.
 *
 * @return the compatible assignment opcode
 */
static uint16_t
parser_check_left_hand_side_expression (parser_context_t *context_p, /**< context */
                                        uint16_t opcode) /**< opcode to check */
{
  if (opcode == CBC_PUSH_LITERAL && context_p->last_cbc.literal_type == LEXER_IDENT_LITERAL)
  {
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
    return CBC_ASSIGN_SET_IDENT;
  }
  else if (opcode == CBC_PUSH_PROP)
  {
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
    return CBC_ASSIGN;
  }
  else if (opcode == CBC_PUSH_PROP_LITERAL)
  {
    context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
    return CBC_ASSIGN_PROP_LITERAL;
  }
  else if (opcode == CBC_PUSH_PROP_LITERAL_LITERAL)
  {
    context_p->last_cbc_opcode = CBC_PUSH_TWO_LITERALS;
    return CBC_ASSIGN;
  }
  else if (opcode == CBC_PUSH_PROP_THIS_LITERAL)
  {
    context_p->last_cbc_opcode = CBC_PUSH_THIS_LITERAL;
    return CBC_ASSIGN;
  }
  else
  {
    /* Invalid LeftHandSide expression. */
    parser_raise_error (context_p, PARSER_ERR_INVALID_LHS_FOR_LOOP);
    return CBC_ASSIGN;
  }

  return opcode;
} /* parser_check_left_hand_side_expression */

/**
 * Parse for statement (starting part).
 */
static void
parser_parse_for_statement_start (parser_context_t *context_p) /**< context */
{
  parser_loop_statement_t loop;

  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_FOR);
  lexer_next_token (context_p);
  bool is_for_await = false;

  if (context_p->token.type == LEXER_KEYW_AWAIT)
  {
    if (JERRY_UNLIKELY (context_p->token.lit_location.status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE))
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_KEYWORD);
    }
    lexer_next_token (context_p);
    is_for_await = true;
  }

  if (context_p->token.type != LEXER_LEFT_PAREN)
  {
    if (context_p->token.type == LEXER_LITERAL && context_p->token.keyword_type == LEXER_KEYW_AWAIT
        && !(context_p->token.lit_location.status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE))
    {
      parser_raise_error (context_p, PARSER_ERR_FOR_AWAIT_NO_ASYNC);
    }

    parser_raise_error (context_p, PARSER_ERR_LEFT_PAREN_EXPECTED);
  }

  if (context_p->next_scanner_info_p->source_p == context_p->source_p)
  {
    parser_for_in_of_statement_t for_in_of_statement;
    scanner_location_t start_location, end_location;

    JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_FOR_IN
                  || context_p->next_scanner_info_p->type == SCANNER_TYPE_FOR_OF);

    bool is_for_in = (context_p->next_scanner_info_p->type == SCANNER_TYPE_FOR_IN);
    end_location = ((scanner_location_info_t *) context_p->next_scanner_info_p)->location;

    scanner_release_next (context_p, sizeof (scanner_location_info_t));

    scanner_get_location (&start_location, context_p);
    lexer_next_token (context_p);

    uint8_t token_type = LEXER_EOS;
    bool has_context = false;

    if (context_p->token.type == LEXER_KEYW_VAR || context_p->token.type == LEXER_KEYW_LET
        || context_p->token.type == LEXER_KEYW_CONST)
    {
      token_type = context_p->token.type;
      has_context = context_p->next_scanner_info_p->source_p == context_p->source_p;
      JERRY_ASSERT (!has_context || context_p->next_scanner_info_p->type == SCANNER_TYPE_BLOCK);
      scanner_get_location (&start_location, context_p);

      /* TODO: remove this after the pre-scanner supports strict mode detection. */
      if (context_p->next_scanner_info_p->source_p == context_p->source_p
          && context_p->next_scanner_info_p->type == SCANNER_TYPE_LET_EXPRESSION)
      {
        scanner_release_next (context_p, sizeof (scanner_info_t));
      }
    }
    else if (context_p->token.type == LEXER_LITERAL && lexer_token_is_let (context_p))
    {
      if (context_p->next_scanner_info_p->source_p == context_p->source_p
          && context_p->next_scanner_info_p->type == SCANNER_TYPE_LET_EXPRESSION)
      {
        scanner_release_next (context_p, sizeof (scanner_info_t));
      }
      else
      {
        token_type = LEXER_KEYW_LET;
        has_context = (context_p->next_scanner_info_p->source_p == context_p->source_p);
        scanner_get_location (&start_location, context_p);
      }
    }

    if (has_context)
    {
      has_context = parser_push_block_context (context_p, true);
    }

    scanner_set_location (context_p, &end_location);

    /* The length of both 'in' and 'of' is two. */
    const uint8_t *source_end_p = context_p->source_p - 2;

    scanner_seek (context_p);

    if (is_for_in && is_for_await)
    {
      context_p->token.line = context_p->line;
      context_p->token.column = context_p->column - 2;
      parser_raise_error (context_p, PARSER_ERR_FOR_AWAIT_NO_OF);
    }

    lexer_next_token (context_p);
    int options = is_for_in ? PARSE_EXPR : PARSE_EXPR_NO_COMMA;
    parser_parse_expression (context_p, options);

    if (context_p->token.type != LEXER_RIGHT_PAREN)
    {
      parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
    }

#ifndef JERRY_NDEBUG
    PARSER_PLUS_EQUAL_U16 (context_p->context_stack_depth,
                           is_for_in ? PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION : PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

    cbc_ext_opcode_t init_opcode = CBC_EXT_FOR_IN_INIT;

    if (!is_for_in)
    {
      init_opcode = is_for_await ? CBC_EXT_FOR_AWAIT_OF_INIT : CBC_EXT_FOR_OF_INIT;
    }

    parser_emit_cbc_ext_forward_branch (context_p, init_opcode, &for_in_of_statement.branch);

    JERRY_ASSERT (context_p->last_cbc_opcode == PARSER_CBC_UNAVAILABLE);
    for_in_of_statement.start_offset = context_p->byte_code_size;

    if (has_context)
    {
      parser_emit_cbc_ext (context_p, CBC_EXT_CLONE_CONTEXT);
    }

    /* The expression parser must not read the 'in' or 'of' tokens. */
    scanner_get_location (&end_location, context_p);
    scanner_set_location (context_p, &start_location);

    const uint8_t *original_source_end_p = context_p->source_end_p;
    context_p->source_end_p = source_end_p;
    scanner_seek (context_p);

    if (token_type == LEXER_EOS)
    {
      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_LEFT_SQUARE || context_p->token.type == LEXER_LEFT_BRACE)
      {
        token_type = context_p->token.type;
      }
    }

    switch (token_type)
    {
      case LEXER_KEYW_LET:
      case LEXER_KEYW_CONST:
      case LEXER_KEYW_VAR:
      {
        if (lexer_check_next_characters (context_p, LIT_CHAR_LEFT_SQUARE, LIT_CHAR_LEFT_BRACE))
        {
          parser_emit_cbc_ext (context_p, is_for_in ? CBC_EXT_FOR_IN_GET_NEXT : CBC_EXT_FOR_OF_GET_NEXT);

          parser_pattern_flags_t flags = (parser_pattern_flags_t ) ((unsigned int) PARSER_PATTERN_BINDING | (unsigned int) PARSER_PATTERN_TARGET_ON_STACK);

          if (context_p->next_scanner_info_p->source_p == (context_p->source_p + 1))
          {
            if (context_p->next_scanner_info_p->type == SCANNER_TYPE_INITIALIZER)
            {
              scanner_release_next (context_p, sizeof (scanner_location_info_t));
            }
            else
            {
              JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS);
              if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_REST)
              {
                flags = (parser_pattern_flags_t) ((int) flags | (int) PARSER_PATTERN_HAS_REST_ELEMENT);
              }

              scanner_release_next (context_p, sizeof (scanner_info_t));
            }
          }

          if (token_type == LEXER_KEYW_LET)
          {
            flags = (parser_pattern_flags_t) ((int) flags | (int) PARSER_PATTERN_LET);
          }
          else if (token_type == LEXER_KEYW_CONST)
          {
            flags = (parser_pattern_flags_t) ((int) flags | (int) PARSER_PATTERN_CONST);
          }

          parser_parse_initializer_by_next_char (context_p, flags);
          break;
        }

        lexer_expect_identifier (context_p, LEXER_IDENT_LITERAL);

        if (context_p->token.keyword_type == LEXER_KEYW_LET && token_type != LEXER_KEYW_VAR)
        {
          parser_raise_error (context_p, PARSER_ERR_LEXICAL_LET_BINDING);
        }

        JERRY_ASSERT (context_p->token.type == LEXER_LITERAL
                      && context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

        uint16_t literal_index = context_p->lit_object.index;
        lexer_next_token (context_p);

        if (context_p->token.type == LEXER_ASSIGN)
        {
          if ((context_p->status_flags & PARSER_IS_STRICT) || !is_for_in)
          {
            parser_raise_error (context_p, PARSER_ERR_FOR_IN_OF_DECLARATION);
          }
          parser_branch_t branch;

          /* Initialiser is never executed. */
          parser_emit_cbc_forward_branch (context_p, CBC_JUMP_FORWARD, &branch);
          lexer_next_token (context_p);
          parser_parse_expression_statement (context_p, PARSE_EXPR_NO_COMMA);
          parser_set_branch_to_current_position (context_p, &branch);
        }

        parser_emit_cbc_ext (context_p, is_for_in ? CBC_EXT_FOR_IN_GET_NEXT : CBC_EXT_FOR_OF_GET_NEXT);
#ifndef JERRY_NDEBUG
        if (literal_index < PARSER_REGISTER_START && has_context
            && !scanner_literal_is_created (context_p, literal_index))
        {
          context_p->global_status_flags |= ECMA_PARSE_INTERNAL_FOR_IN_OFF_CONTEXT_ERROR;
        }
#endif /* !JERRY_NDEBUG */

        uint16_t opcode = (has_context ? CBC_ASSIGN_LET_CONST : CBC_ASSIGN_SET_IDENT);
        parser_emit_cbc_literal (context_p, opcode, literal_index);
        break;
      }
      case LEXER_LEFT_BRACE:
      case LEXER_LEFT_SQUARE:
      {
        if (context_p->next_scanner_info_p->source_p == context_p->source_p
            && context_p->next_scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS
            && (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_DESTRUCTURING_FOR))
        {
          parser_emit_cbc_ext (context_p, is_for_in ? CBC_EXT_FOR_IN_GET_NEXT : CBC_EXT_FOR_OF_GET_NEXT);

          uint32_t flags = PARSER_PATTERN_TARGET_ON_STACK;

          if (context_p->next_scanner_info_p->u8_arg & SCANNER_LITERAL_OBJECT_HAS_REST)
          {
            flags |= PARSER_PATTERN_HAS_REST_ELEMENT;
          }

          scanner_release_next (context_p, sizeof (scanner_info_t));
          parser_parse_initializer (context_p, (parser_pattern_flags_t) flags);
          /* Pop the value returned by GET_NEXT. */
          parser_emit_cbc (context_p, CBC_POP);
          break;
        }
        /* FALLTHRU */
      }
      default:
      {
        uint16_t opcode;

        parser_parse_expression (context_p, PARSE_EXPR_LEFT_HAND_SIDE);

        opcode = context_p->last_cbc_opcode;

        /* The CBC_EXT_FOR_IN_CREATE_CONTEXT flushed the opcode combiner. */
        JERRY_ASSERT (opcode != CBC_PUSH_TWO_LITERALS && opcode != CBC_PUSH_THREE_LITERALS);

        opcode = parser_check_left_hand_side_expression (context_p, opcode);

        parser_emit_cbc_ext (context_p, is_for_in ? CBC_EXT_FOR_IN_GET_NEXT : CBC_EXT_FOR_OF_GET_NEXT);
        parser_flush_cbc (context_p);

        context_p->last_cbc_opcode = opcode;
        break;
      }
    }

    if (context_p->token.type != LEXER_EOS)
    {
      parser_raise_error (context_p, is_for_in ? PARSER_ERR_IN_EXPECTED : PARSER_ERR_OF_EXPECTED);
    }

    parser_flush_cbc (context_p);
    scanner_set_location (context_p, &end_location);
    context_p->source_end_p = original_source_end_p;
    lexer_next_token (context_p);

    loop.branch_list_p = NULL;

    parser_stack_push (context_p, &for_in_of_statement, sizeof (parser_for_in_of_statement_t));
    parser_stack_push (context_p, &loop, sizeof (parser_loop_statement_t));

    uint8_t for_type = PARSER_STATEMENT_FOR_IN;

    if (!is_for_in)
    {
      for_type = is_for_await ? PARSER_STATEMENT_FOR_AWAIT_OF : PARSER_STATEMENT_FOR_OF;
    }

    parser_stack_push_uint8 (context_p, for_type);
    parser_stack_iterator_init (context_p, &context_p->last_statement);
    return;
  }

  lexer_next_token (context_p);

  if (context_p->token.type != LEXER_SEMICOLON)
  {
    const uint8_t *source_p = context_p->source_p;

    switch (context_p->token.type)
    {
      case LEXER_LITERAL:
      {
        if (!lexer_token_is_let (context_p))
        {
          parser_parse_expression_statement (context_p, PARSE_EXPR);
          break;
        }

        /* FALLTHRU */
      }
      case LEXER_KEYW_LET:
      {
        if (context_p->next_scanner_info_p->source_p == context_p->source_p
            && context_p->next_scanner_info_p->type != SCANNER_TYPE_BLOCK)
        {
          if (context_p->next_scanner_info_p->type == SCANNER_TYPE_LET_EXPRESSION)
          {
            scanner_release_next (context_p, sizeof (scanner_info_t));
          }

          parser_parse_expression_statement (context_p, PARSE_EXPR);
          break;
        }

        context_p->token.type = LEXER_KEYW_LET;

        /* FALLTHRU */
      }
      case LEXER_KEYW_CONST:
      {
        if (context_p->next_scanner_info_p->source_p == source_p)
        {
          parser_push_block_context (context_p, true);
        }
        /* FALLTHRU */
      }
      case LEXER_KEYW_VAR:
      {
        parser_parse_var_statement (context_p);
        break;
      }
      default:
      {
        parser_parse_expression_statement (context_p, PARSE_EXPR);
        break;
      }
    }

    if (context_p->token.type != LEXER_SEMICOLON)
    {
      parser_raise_error (context_p, PARSER_ERR_SEMICOLON_EXPECTED);
    }
  }

  if (is_for_await)
  {
    parser_raise_error (context_p, PARSER_ERR_FOR_AWAIT_NO_OF);
  }

  JERRY_ASSERT (context_p->next_scanner_info_p->source_p != context_p->source_p
                || context_p->next_scanner_info_p->type == SCANNER_TYPE_FOR);

  if (context_p->next_scanner_info_p->source_p != context_p->source_p
      || ((scanner_for_info_t *) context_p->next_scanner_info_p)->end_location.source_p == NULL)
  {
    if (context_p->next_scanner_info_p->source_p == context_p->source_p)
    {
      /* Even though the scanning is failed, there might be valid statements
       * inside the for statement which depend on scanner info blocks. */
      scanner_release_next (context_p, sizeof (scanner_for_info_t));
    }

    /* The prescanner couldn't find the second semicolon or the closing parentheses. */
    lexer_next_token (context_p);
    parser_parse_expression (context_p, PARSE_EXPR);

    if (context_p->token.type != LEXER_SEMICOLON)
    {
      parser_raise_error (context_p, PARSER_ERR_SEMICOLON_EXPECTED);
    }

    lexer_next_token (context_p);
    parser_parse_expression_statement (context_p, PARSE_EXPR);

    JERRY_ASSERT (context_p->token.type != LEXER_RIGHT_PAREN);
    parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
  }

  parser_for_statement_t for_statement;
  scanner_for_info_t *for_info_p = (scanner_for_info_t *) context_p->next_scanner_info_p;

  parser_emit_cbc_forward_branch (context_p, CBC_JUMP_FORWARD, &for_statement.branch);

  JERRY_ASSERT (context_p->last_cbc_opcode == PARSER_CBC_UNAVAILABLE);

  for_statement.start_offset = context_p->byte_code_size;
  scanner_get_location (&for_statement.condition_location, context_p);
  for_statement.expression_location = for_info_p->expression_location;

  scanner_set_location (context_p, &for_info_p->end_location);
  scanner_release_next (context_p, sizeof (scanner_for_info_t));
  scanner_seek (context_p);
  lexer_next_token (context_p);

  loop.branch_list_p = NULL;

  parser_stack_push (context_p, &for_statement, sizeof (parser_for_statement_t));
  parser_stack_push (context_p, &loop, sizeof (parser_loop_statement_t));
  parser_stack_push_uint8 (context_p, PARSER_STATEMENT_FOR);
  parser_stack_iterator_init (context_p, &context_p->last_statement);
} /* parser_parse_for_statement_start */

/**
 * Parse for statement (ending part).
 */
static void JERRY_ATTR_NOINLINE
parser_parse_for_statement_end (parser_context_t *context_p) /**< context */
{
  parser_for_statement_t for_statement;
  parser_loop_statement_t loop;
  lexer_token_t current_token;
  scanner_location_t location;
  cbc_opcode_t opcode;

  JERRY_ASSERT (context_p->stack_top_uint8 == PARSER_STATEMENT_FOR);

  parser_stack_iterator_t iterator;
  parser_stack_iterator_init (context_p, &iterator);

  parser_stack_iterator_skip (&iterator, 1);
  parser_stack_iterator_read (&iterator, &loop, sizeof (parser_loop_statement_t));
  parser_stack_iterator_skip (&iterator, sizeof (parser_loop_statement_t));
  parser_stack_iterator_read (&iterator, &for_statement, sizeof (parser_for_statement_t));

  bool has_block_context = false;
  uint8_t next_statement_type;

  parser_stack_iterator_skip (&iterator, sizeof (parser_for_statement_t));
  parser_stack_iterator_read (&iterator, &next_statement_type, 1);

  if (next_statement_type == PARSER_STATEMENT_PRIVATE_CONTEXT)
  {
    has_block_context = true;
  }

  scanner_get_location (&location, context_p);
  current_token = context_p->token;

  scanner_set_location (context_p, &for_statement.expression_location);
  scanner_seek (context_p);
  lexer_next_token (context_p);

  parser_set_continues_to_current_position (context_p, loop.branch_list_p);

  if (has_block_context)
  {
    parser_emit_cbc_ext (context_p, CBC_EXT_CLONE_FULL_CONTEXT);
  }

  if (context_p->token.type != LEXER_RIGHT_PAREN)
  {
    parser_parse_expression_statement (context_p, PARSE_EXPR);

    if (context_p->token.type != LEXER_RIGHT_PAREN)
    {
      parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
    }
  }

  parser_set_branch_to_current_position (context_p, &for_statement.branch);

  scanner_set_location (context_p, &for_statement.condition_location);
  scanner_seek (context_p);
  lexer_next_token (context_p);

  if (context_p->token.type != LEXER_SEMICOLON)
  {
    parser_parse_expression (context_p, PARSE_EXPR);

    if (context_p->token.type != LEXER_SEMICOLON)
    {
      parser_raise_error (context_p, PARSER_ERR_SEMICOLON_EXPECTED);
    }

    opcode = CBC_BRANCH_IF_TRUE_BACKWARD;
    if (context_p->last_cbc_opcode == CBC_LOGICAL_NOT)
    {
      context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
      opcode = CBC_BRANCH_IF_FALSE_BACKWARD;
    }
    else if (context_p->last_cbc_opcode == CBC_PUSH_TRUE)
    {
      context_p->last_cbc_opcode = PARSER_CBC_UNAVAILABLE;
      opcode = CBC_JUMP_BACKWARD;
    }
  }
  else
  {
    opcode = CBC_JUMP_BACKWARD;
  }

  parser_stack_pop (context_p, NULL, 1 + sizeof (parser_loop_statement_t) + sizeof (parser_for_statement_t));
  parser_stack_iterator_init (context_p, &context_p->last_statement);

  parser_emit_cbc_backward_branch (context_p, (uint16_t) opcode, for_statement.start_offset);
  parser_set_breaks_to_current_position (context_p, loop.branch_list_p);

  if (context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_SCOPE
      || context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_CONTEXT)
  {
    parser_pop_block_context (context_p);
  }

  /* Calling scanner_seek is unnecessary because all
   * info blocks inside the for statement should be processed. */
  scanner_set_location (context_p, &location);
  context_p->token = current_token;
} /* parser_parse_for_statement_end */

/**
 * Parse switch statement (starting part).
 */
static void JERRY_ATTR_NOINLINE
parser_parse_switch_statement_start (parser_context_t *context_p) /**< context */
{
  parser_switch_statement_t switch_statement;
  parser_loop_statement_t loop;
  parser_stack_iterator_t iterator;
  scanner_location_t start_location;
  bool switch_case_was_found;
  bool default_case_was_found;
  parser_branch_node_t *case_branches_p = NULL;

  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_SWITCH);

  parser_parse_enclosed_expr (context_p);

  if (context_p->token.type != LEXER_LEFT_BRACE)
  {
    parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_EXPECTED);
  }

  if (context_p->next_scanner_info_p->source_p == context_p->source_p - 1)
  {
    parser_push_block_context (context_p, true);
  }

  JERRY_ASSERT (context_p->next_scanner_info_p->source_p == context_p->source_p
                && context_p->next_scanner_info_p->type == SCANNER_TYPE_SWITCH);

  scanner_case_info_t *case_info_p = ((scanner_switch_info_t *) context_p->next_scanner_info_p)->case_p;
  scanner_set_active (context_p);

  if (case_info_p == NULL)
  {
    lexer_next_token (context_p);

    if (context_p->token.type == LEXER_RIGHT_BRACE)
    {
      scanner_release_active (context_p, sizeof (scanner_switch_info_t));

      parser_emit_cbc (context_p, CBC_POP);
      parser_flush_cbc (context_p);

      parser_stack_push_uint8 (context_p, PARSER_STATEMENT_BLOCK);
      parser_stack_iterator_init (context_p, &context_p->last_statement);
      return;
    }

    parser_raise_error (context_p, PARSER_ERR_INVALID_SWITCH);
  }

  scanner_get_location (&start_location, context_p);

  /* The reason of using an iterator is error management. If an error
   * occur, parser_free_jumps() free all data. However, the branches
   * created by parser_emit_cbc_forward_branch_item() would not be freed.
   * To free these branches, the current switch data is always stored
   * on the stack. If any change happens, this data is updated. Updates
   * are done using the iterator. */

  switch_statement.branch_list_p = NULL;
  loop.branch_list_p = NULL;

  parser_stack_push (context_p, &switch_statement, sizeof (parser_switch_statement_t));
  parser_stack_iterator_init (context_p, &iterator);
  parser_stack_push (context_p, &loop, sizeof (parser_loop_statement_t));
  parser_stack_push_uint8 (context_p, PARSER_STATEMENT_SWITCH);
  parser_stack_iterator_init (context_p, &context_p->last_statement);

  switch_case_was_found = false;
  default_case_was_found = false;

  do
  {
    scanner_set_location (context_p, &case_info_p->location);
    scanner_seek (context_p);
    case_info_p = case_info_p->next_p;

    /* The last letter of case and default is 'e' and 't' respectively.  */
    JERRY_ASSERT (context_p->source_p[-1] == LIT_CHAR_LOWERCASE_E || context_p->source_p[-1] == LIT_CHAR_LOWERCASE_T);

    bool is_default = context_p->source_p[-1] == LIT_CHAR_LOWERCASE_T;
    lexer_next_token (context_p);

    if (is_default)
    {
      if (default_case_was_found)
      {
        parser_raise_error (context_p, PARSER_ERR_MULTIPLE_DEFAULTS_NOT_ALLOWED);
      }

      if (context_p->token.type != LEXER_COLON)
      {
        parser_raise_error (context_p, PARSER_ERR_COLON_EXPECTED);
      }

      default_case_was_found = true;
      continue;
    }

    switch_case_was_found = true;

    parser_parse_expression (context_p, PARSE_EXPR);

    if (context_p->token.type != LEXER_COLON)
    {
      parser_raise_error (context_p, PARSER_ERR_COLON_EXPECTED);
    }

    uint16_t opcode = CBC_BRANCH_IF_STRICT_EQUAL;

    if (case_info_p == NULL
        || (case_info_p->next_p == NULL && case_info_p->location.source_p[-1] == LIT_CHAR_LOWERCASE_T))
    {
      /* There are no more 'case' statements in the switch. */
      parser_emit_cbc (context_p, CBC_STRICT_EQUAL);
      opcode = CBC_BRANCH_IF_TRUE_FORWARD;
    }

    parser_branch_node_t *new_case_p = parser_emit_cbc_forward_branch_item (context_p, opcode, NULL);

    if (case_branches_p == NULL)
    {
      switch_statement.branch_list_p = new_case_p;
      parser_stack_iterator_write (&iterator, &switch_statement, sizeof (parser_switch_statement_t));
    }
    else
    {
      case_branches_p->next_p = new_case_p;
    }

    case_branches_p = new_case_p;
  } while (case_info_p != NULL);

  JERRY_ASSERT (switch_case_was_found || default_case_was_found);

  if (!switch_case_was_found)
  {
    /* There was no case statement, so the expression result
     * of the switch must be popped from the stack */
    parser_emit_cbc (context_p, CBC_POP);
  }

  parser_emit_cbc_forward_branch (context_p, CBC_JUMP_FORWARD, &switch_statement.default_branch);
  parser_stack_iterator_write (&iterator, &switch_statement, sizeof (parser_switch_statement_t));

  if (!default_case_was_found)
  {
    parser_stack_change_last_uint8 (context_p, PARSER_STATEMENT_SWITCH_NO_DEFAULT);
  }

  scanner_release_switch_cases (((scanner_switch_info_t *) context_p->active_scanner_info_p)->case_p);
  scanner_release_active (context_p, sizeof (scanner_switch_info_t));

  scanner_set_location (context_p, &start_location);
  scanner_seek (context_p);
  lexer_next_token (context_p);
} /* parser_parse_switch_statement_start */

/**
 * Parse try statement (ending part).
 */
static void
parser_parse_try_statement_end (parser_context_t *context_p) /**< context */
{
  parser_try_statement_t try_statement;
  parser_stack_iterator_t iterator;

  JERRY_ASSERT (context_p->stack_top_uint8 == PARSER_STATEMENT_TRY);

  parser_stack_iterator_init (context_p, &iterator);
  parser_stack_iterator_skip (&iterator, 1);
  parser_stack_iterator_read (&iterator, &try_statement, sizeof (parser_try_statement_t));

  context_p->scope_stack_top = try_statement.scope_stack_top;
  context_p->scope_stack_reg_top = try_statement.scope_stack_reg_top;

  lexer_next_token (context_p);

  if (try_statement.type == parser_finally_block)
  {
    parser_flush_cbc (context_p);
    PARSER_MINUS_EQUAL_U16 (context_p->stack_depth, PARSER_FINALLY_CONTEXT_STACK_ALLOCATION);
#ifndef JERRY_NDEBUG
    PARSER_MINUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_FINALLY_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

    parser_emit_cbc (context_p, CBC_CONTEXT_END);
    parser_set_branch_to_current_position (context_p, &try_statement.branch);
  }
  else
  {
    parser_set_branch_to_current_position (context_p, &try_statement.branch);

    if (try_statement.type == parser_catch_block)
    {
      if (context_p->token.type != LEXER_KEYW_FINALLY)
      {
        parser_flush_cbc (context_p);
        PARSER_MINUS_EQUAL_U16 (context_p->stack_depth, PARSER_TRY_CONTEXT_STACK_ALLOCATION);
#ifndef JERRY_NDEBUG
        PARSER_MINUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_TRY_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

        parser_emit_cbc (context_p, CBC_CONTEXT_END);
        parser_flush_cbc (context_p);

        try_statement.type = parser_finally_block;
      }
    }
    else
    {
      JERRY_ASSERT (try_statement.type == parser_try_block);

      if (context_p->token.type != LEXER_KEYW_CATCH && context_p->token.type != LEXER_KEYW_FINALLY)
      {
        parser_raise_error (context_p, PARSER_ERR_CATCH_FINALLY_EXPECTED);
      }
    }
  }

  if (try_statement.type == parser_finally_block)
  {
    parser_stack_pop (context_p, NULL, (uint32_t) (sizeof (parser_try_statement_t) + 1));
    parser_stack_iterator_init (context_p, &context_p->last_statement);
    return;
  }

  if (context_p->token.type == LEXER_KEYW_CATCH)
  {
    lexer_next_token (context_p);

    try_statement.type = parser_catch_block;
    parser_emit_cbc_ext_forward_branch (context_p, CBC_EXT_CATCH, &try_statement.branch);

    try_statement.scope_stack_top = context_p->scope_stack_top;
    try_statement.scope_stack_reg_top = context_p->scope_stack_reg_top;

#ifndef JERRY_NDEBUG
    bool block_found = false;
#endif /* !JERRY_NDEBUG */

    if (context_p->next_scanner_info_p->source_p == context_p->source_p)
    {
      JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_BLOCK);
#ifndef JERRY_NDEBUG
      block_found = true;
#endif /* !JERRY_NDEBUG */

      if (scanner_is_context_needed (context_p, PARSER_CHECK_BLOCK_CONTEXT))
      {
        parser_emit_cbc_ext (context_p, CBC_EXT_TRY_CREATE_ENV);
      }

      scanner_create_variables (context_p, SCANNER_CREATE_VARS_NO_OPTS);
    }

    if (context_p->token.type == LEXER_LEFT_PAREN)
    {
      if (lexer_check_next_characters (context_p, LIT_CHAR_LEFT_SQUARE, LIT_CHAR_LEFT_BRACE))
      {
        parser_pattern_flags_t flags = (parser_pattern_flags_t) ((unsigned int) PARSER_PATTERN_BINDING | (unsigned int) PARSER_PATTERN_TARGET_ON_STACK | (unsigned int) PARSER_PATTERN_LET);

        parser_parse_initializer_by_next_char (context_p, flags);
      }
      else
      {
        lexer_expect_identifier (context_p, LEXER_IDENT_LITERAL);
        JERRY_ASSERT (context_p->token.type == LEXER_LITERAL
                      && context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

        uint16_t literal_index = context_p->lit_object.index;
        parser_emit_cbc_literal (context_p,
                                 (literal_index >= PARSER_REGISTER_START) ? CBC_ASSIGN_SET_IDENT : CBC_ASSIGN_LET_CONST,
                                 literal_index);

        lexer_next_token (context_p);

#ifndef JERRY_NDEBUG
        JERRY_ASSERT (block_found);
#endif /* !JERRY_NDEBUG */
      }

      if (context_p->token.type != LEXER_RIGHT_PAREN)
      {
        parser_raise_error (context_p, PARSER_ERR_RIGHT_PAREN_EXPECTED);
      }

      lexer_next_token (context_p);

      if (context_p->token.type != LEXER_LEFT_BRACE)
      {
        parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_EXPECTED);
      }
    }
    else if (context_p->token.type == LEXER_LEFT_BRACE)
    {
      parser_emit_cbc (context_p, CBC_POP);
    }
    else
    {
      parser_raise_error (context_p, PARSER_ERR_LEFT_PAREN_EXPECTED);
    }

    parser_flush_cbc (context_p);
  }
  else
  {
    JERRY_ASSERT (context_p->token.type == LEXER_KEYW_FINALLY);

    lexer_next_token (context_p);

    if (context_p->token.type != LEXER_LEFT_BRACE)
    {
      parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_EXPECTED);
    }

#ifndef JERRY_NDEBUG
    PARSER_PLUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_FINALLY_CONTEXT_EXTRA_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

    try_statement.type = parser_finally_block;
    parser_emit_cbc_ext_forward_branch (context_p, CBC_EXT_FINALLY, &try_statement.branch);

    if (context_p->next_scanner_info_p->source_p == context_p->source_p)
    {
      JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_BLOCK);

      if (scanner_is_context_needed (context_p, PARSER_CHECK_BLOCK_CONTEXT))
      {
        parser_emit_cbc_ext (context_p, CBC_EXT_TRY_CREATE_ENV);
      }

      scanner_create_variables (context_p, SCANNER_CREATE_VARS_NO_OPTS);
    }
  }

  lexer_next_token (context_p);
  parser_stack_iterator_write (&iterator, &try_statement, sizeof (parser_try_statement_t));
} /* parser_parse_try_statement_end */

/**
 * Parse default statement.
 */
static void
parser_parse_default_statement (parser_context_t *context_p) /**< context */
{
  parser_stack_iterator_t iterator;
  parser_switch_statement_t switch_statement;

  if (context_p->stack_top_uint8 != PARSER_STATEMENT_SWITCH
      && context_p->stack_top_uint8 != PARSER_STATEMENT_SWITCH_NO_DEFAULT)
  {
    parser_raise_error (context_p, PARSER_ERR_DEFAULT_NOT_IN_SWITCH);
  }

  lexer_next_token (context_p);
  /* Already checked in parser_parse_switch_statement_start. */
  JERRY_ASSERT (context_p->token.type == LEXER_COLON);
  lexer_next_token (context_p);

  parser_stack_iterator_init (context_p, &iterator);
  parser_stack_iterator_skip (&iterator, 1 + sizeof (parser_loop_statement_t));
  parser_stack_iterator_read (&iterator, &switch_statement, sizeof (parser_switch_statement_t));

  parser_set_branch_to_current_position (context_p, &switch_statement.default_branch);
} /* parser_parse_default_statement */

/**
 * Parse case statement.
 */
static void
parser_parse_case_statement (parser_context_t *context_p) /**< context */
{
  parser_stack_iterator_t iterator;
  parser_switch_statement_t switch_statement;
  parser_branch_node_t *branch_p;

  if (context_p->stack_top_uint8 != PARSER_STATEMENT_SWITCH
      && context_p->stack_top_uint8 != PARSER_STATEMENT_SWITCH_NO_DEFAULT)
  {
    parser_raise_error (context_p, PARSER_ERR_CASE_NOT_IN_SWITCH);
  }

  if (context_p->next_scanner_info_p->source_p != context_p->source_p)
  {
    lexer_next_token (context_p);

    parser_parse_expression (context_p, PARSE_EXPR);

    JERRY_ASSERT (context_p->token.type != LEXER_COLON);
    parser_raise_error (context_p, PARSER_ERR_COLON_EXPECTED);
  }

  JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_CASE);

  scanner_set_location (context_p, &((scanner_location_info_t *) context_p->next_scanner_info_p)->location);
  scanner_release_next (context_p, sizeof (scanner_location_info_t));
  scanner_seek (context_p);
  lexer_next_token (context_p);

  parser_stack_iterator_init (context_p, &iterator);
  parser_stack_iterator_skip (&iterator, 1 + sizeof (parser_loop_statement_t));
  parser_stack_iterator_read (&iterator, &switch_statement, sizeof (parser_switch_statement_t));

  /* Free memory after the case statement is found. */

  branch_p = switch_statement.branch_list_p;
  JERRY_ASSERT (branch_p != NULL);
  switch_statement.branch_list_p = branch_p->next_p;
  parser_stack_iterator_write (&iterator, &switch_statement, sizeof (parser_switch_statement_t));

  parser_set_branch_to_current_position (context_p, &branch_p->branch);
  parser_free (branch_p, sizeof (parser_branch_node_t));
} /* parser_parse_case_statement */

/**
 * Parse break statement.
 */
static void
parser_parse_break_statement (parser_context_t *context_p) /**< context */
{
  parser_stack_iterator_t iterator;
  cbc_opcode_t opcode = CBC_JUMP_FORWARD;

  lexer_next_token (context_p);
  parser_stack_iterator_init (context_p, &iterator);

  if (!(context_p->token.flags & LEXER_WAS_NEWLINE) && context_p->token.type == LEXER_LITERAL
      && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
  {
    /* The label with the same name is searched on the stack. */
    while (true)
    {
      uint8_t type = parser_stack_iterator_read_uint8 (&iterator);
      if (type == PARSER_STATEMENT_START)
      {
        parser_raise_error (context_p, PARSER_ERR_INVALID_BREAK_LABEL);
      }

      if (parser_statement_flags[type] & PARSER_STATM_CONTEXT_BREAK)
      {
        opcode = CBC_JUMP_FORWARD_EXIT_CONTEXT;
      }

      if (type == PARSER_STATEMENT_LABEL)
      {
        parser_label_statement_t label_statement;

        parser_stack_iterator_skip (&iterator, 1);
        parser_stack_iterator_read (&iterator, &label_statement, sizeof (parser_label_statement_t));

        if (lexer_current_is_literal (context_p, &label_statement.label_ident))
        {
          label_statement.break_list_p =
            parser_emit_cbc_forward_branch_item (context_p, (uint16_t) opcode, label_statement.break_list_p);
          parser_stack_iterator_write (&iterator, &label_statement, sizeof (parser_label_statement_t));
          lexer_next_token (context_p);
          return;
        }
        parser_stack_iterator_skip (&iterator, sizeof (parser_label_statement_t));
      }
      else
      {
        parser_stack_iterator_skip (&iterator, parser_statement_length (type));
      }
    }
  }

  /* The first switch or loop statement is searched. */
  while (true)
  {
    uint8_t type = parser_stack_iterator_read_uint8 (&iterator);
    if (type == PARSER_STATEMENT_START)
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_BREAK);
    }

    if (parser_statement_flags[type] & PARSER_STATM_CONTEXT_BREAK)
    {
      opcode = CBC_JUMP_FORWARD_EXIT_CONTEXT;
    }

    if (parser_statement_flags[type] & PARSER_STATM_BREAK_TARGET)
    {
      parser_loop_statement_t loop;

      parser_stack_iterator_skip (&iterator, 1);
      parser_stack_iterator_read (&iterator, &loop, sizeof (parser_loop_statement_t));
      loop.branch_list_p = parser_emit_cbc_forward_branch_item (context_p, (uint16_t) opcode, loop.branch_list_p);
      parser_stack_iterator_write (&iterator, &loop, sizeof (parser_loop_statement_t));
      return;
    }

    parser_stack_iterator_skip (&iterator, parser_statement_length (type));
  }
} /* parser_parse_break_statement */

/**
 * Parse continue statement.
 */
static void
parser_parse_continue_statement (parser_context_t *context_p) /**< context */
{
  parser_stack_iterator_t iterator;
  cbc_opcode_t opcode = CBC_JUMP_FORWARD;

  lexer_next_token (context_p);
  parser_stack_iterator_init (context_p, &iterator);

  if (!(context_p->token.flags & LEXER_WAS_NEWLINE) && context_p->token.type == LEXER_LITERAL
      && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
  {
    parser_stack_iterator_t loop_iterator;

    loop_iterator.current_p = NULL;

    /* The label with the same name is searched on the stack. */
    while (true)
    {
      uint8_t type = parser_stack_iterator_read_uint8 (&iterator);

      if (type == PARSER_STATEMENT_START)
      {
        parser_raise_error (context_p, PARSER_ERR_INVALID_CONTINUE_LABEL);
      }

      /* Only those labels are checked, whose are label of a loop. */
      if (loop_iterator.current_p != NULL && type == PARSER_STATEMENT_LABEL)
      {
        parser_label_statement_t label_statement;

        parser_stack_iterator_skip (&iterator, 1);
        parser_stack_iterator_read (&iterator, &label_statement, sizeof (parser_label_statement_t));

        if (lexer_current_is_literal (context_p, &label_statement.label_ident))
        {
          parser_loop_statement_t loop;

          parser_stack_iterator_skip (&loop_iterator, 1);
          parser_stack_iterator_read (&loop_iterator, &loop, sizeof (parser_loop_statement_t));
          loop.branch_list_p = parser_emit_cbc_forward_branch_item (context_p, (uint16_t) opcode, loop.branch_list_p);
          loop.branch_list_p->branch.offset |= CBC_HIGHEST_BIT_MASK;
          parser_stack_iterator_write (&loop_iterator, &loop, sizeof (parser_loop_statement_t));
          lexer_next_token (context_p);
          return;
        }
        parser_stack_iterator_skip (&iterator, sizeof (parser_label_statement_t));
        continue;
      }

      if (parser_statement_flags[type] & PARSER_STATM_CONTEXT_BREAK)
      {
        opcode = CBC_JUMP_FORWARD_EXIT_CONTEXT;
      }

      const bool is_private_scope =
        (type == PARSER_STATEMENT_PRIVATE_SCOPE || type == PARSER_STATEMENT_PRIVATE_CONTEXT);

      if (parser_statement_flags[type] & PARSER_STATM_CONTINUE_TARGET)
      {
        loop_iterator = iterator;
      }
      else if (!is_private_scope)
      {
        loop_iterator.current_p = NULL;
      }

      parser_stack_iterator_skip (&iterator, parser_statement_length (type));
    }
  }

  /* The first loop statement is searched. */
  while (true)
  {
    uint8_t type = parser_stack_iterator_read_uint8 (&iterator);
    if (type == PARSER_STATEMENT_START)
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_CONTINUE);
    }

    if (parser_statement_flags[type] & PARSER_STATM_CONTINUE_TARGET)
    {
      parser_loop_statement_t loop;

      parser_stack_iterator_skip (&iterator, 1);
      parser_stack_iterator_read (&iterator, &loop, sizeof (parser_loop_statement_t));
      loop.branch_list_p = parser_emit_cbc_forward_branch_item (context_p, (uint16_t) opcode, loop.branch_list_p);
      loop.branch_list_p->branch.offset |= CBC_HIGHEST_BIT_MASK;
      parser_stack_iterator_write (&iterator, &loop, sizeof (parser_loop_statement_t));
      return;
    }

    if (parser_statement_flags[type] & PARSER_STATM_CONTEXT_BREAK)
    {
      opcode = CBC_JUMP_FORWARD_EXIT_CONTEXT;
    }

    parser_stack_iterator_skip (&iterator, parser_statement_length (type));
  }
} /* parser_parse_continue_statement */

#if JERRY_MODULE_SYSTEM
/**
 * Parse import statement.
 * Note: See 15.2.2
 */
static void
parser_parse_import_statement (parser_context_t *context_p) /**< parser context */
{
  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_IMPORT);
  JERRY_ASSERT (context_p->module_names_p == NULL);

  if (lexer_check_next_characters (context_p, LIT_CHAR_LEFT_PAREN, LIT_CHAR_DOT))
  {
    if (context_p->status_flags & PARSER_IS_FUNCTION)
    {
      parser_parse_expression_statement (context_p, PARSE_EXPR);
      return;
    }

    parser_parse_block_expression (context_p, PARSE_EXPR);
    return;
  }

  parser_module_check_request_place (context_p);
  lexer_next_token (context_p);

  /* Check for a ModuleSpecifier*/
  if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_STRING_LITERAL)
  {
    if (!(context_p->token.type == LEXER_LEFT_BRACE || context_p->token.type == LEXER_MULTIPLY
          || (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL)))
    {
      parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_MULTIPLY_LITERAL_EXPECTED);
    }

    if (context_p->token.type == LEXER_LITERAL)
    {
      /* Handle ImportedDefaultBinding */
      lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_IDENT_LITERAL);

      ecma_string_t *local_name_p = parser_new_ecma_string_from_literal (context_p->lit_object.literal_p);

      if (parser_module_check_duplicate_import (context_p, local_name_p))
      {
        ecma_deref_ecma_string (local_name_p);
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_IMPORT_BINDING);
      }

      ecma_string_t *import_name_p = ecma_get_magic_string (LIT_MAGIC_STRING_DEFAULT);
      parser_module_add_names_to_node (context_p, import_name_p, local_name_p);

      ecma_deref_ecma_string (local_name_p);
      ecma_deref_ecma_string (import_name_p);

      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_COMMA)
      {
        lexer_next_token (context_p);
        if (context_p->token.type != LEXER_MULTIPLY && context_p->token.type != LEXER_LEFT_BRACE)
        {
          parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_MULTIPLY_EXPECTED);
        }
      }
      else if (!lexer_token_is_identifier (context_p, "from", 4))
      {
        parser_raise_error (context_p, PARSER_ERR_FROM_COMMA_EXPECTED);
      }
    }

    if (context_p->token.type == LEXER_MULTIPLY)
    {
      /* NameSpaceImport */
      lexer_next_token (context_p);
      if (!lexer_token_is_identifier (context_p, "as", 2))
      {
        parser_raise_error (context_p, PARSER_ERR_AS_EXPECTED);
      }

      lexer_next_token (context_p);
      if (context_p->token.type != LEXER_LITERAL)
      {
        parser_raise_error (context_p, PARSER_ERR_IDENTIFIER_EXPECTED);
      }

      lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_IDENT_LITERAL);

      ecma_string_t *local_name_p = parser_new_ecma_string_from_literal (context_p->lit_object.literal_p);

      if (parser_module_check_duplicate_import (context_p, local_name_p))
      {
        ecma_deref_ecma_string (local_name_p);
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_IMPORT_BINDING);
      }

      ecma_string_t *import_name_p = ecma_get_magic_string (LIT_MAGIC_STRING_ASTERISK_CHAR);

      parser_module_add_names_to_node (context_p, import_name_p, local_name_p);
      ecma_deref_ecma_string (local_name_p);
      ecma_deref_ecma_string (import_name_p);

      lexer_next_token (context_p);
    }
    else if (context_p->token.type == LEXER_LEFT_BRACE)
    {
      /* Handle NamedImports */
      parser_module_parse_import_clause (context_p);
    }

    if (!lexer_token_is_identifier (context_p, "from", 4))
    {
      parser_raise_error (context_p, PARSER_ERR_FROM_EXPECTED);
    }
    lexer_next_token (context_p);
  }

  parser_module_handle_module_specifier (context_p, NULL);
} /* parser_parse_import_statement */

/**
 * Parse export statement.
 *
 * @return true - if function of class statement was found
 *         false - otherwise
 */
static bool
parser_parse_export_statement (parser_context_t *context_p) /**< context */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_EXPORT);
  JERRY_ASSERT (context_p->module_names_p == NULL);

  parser_module_check_request_place (context_p);

  bool consume_last_statement = false;

  lexer_next_token (context_p);
  switch (context_p->token.type)
  {
    case LEXER_KEYW_DEFAULT:
    {
      scanner_location_t location;
      scanner_get_location (&location, context_p);

      context_p->status_flags |= PARSER_MODULE_STORE_IDENT;

      lexer_next_token (context_p);

      if (context_p->token.type == LEXER_LITERAL && lexer_token_is_async (context_p)
          && context_p->next_scanner_info_p->source_p == context_p->source_p
          && context_p->next_scanner_info_p->type == SCANNER_TYPE_FUNCTION)
      {
        lexer_next_token (context_p);
      }

      if (context_p->token.type == LEXER_KEYW_CLASS)
      {
        context_p->status_flags |= PARSER_MODULE_DEFAULT_CLASS_OR_FUNC;
        parser_parse_class (context_p, true);
        consume_last_statement = true;
      }
      else if (context_p->token.type == LEXER_KEYW_FUNCTION)
      {
        context_p->status_flags |= PARSER_MODULE_DEFAULT_CLASS_OR_FUNC;
        parser_parse_function_statement (context_p);
        consume_last_statement = true;
      }
      else
      {
        /* Assignment expression */
        scanner_set_location (context_p, &location);

        /* 15.2.3.5 Use the synthetic name '*default*' as the identifier. */
        lexer_construct_literal_object (context_p, &lexer_default_literal, lexer_default_literal.type);

        context_p->token.lit_location.type = LEXER_IDENT_LITERAL;
        parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_LITERAL);

        /* Do not overwrite this identifier. */
        context_p->status_flags &= (uint32_t) ~PARSER_MODULE_STORE_IDENT;
        context_p->module_identifier_lit_p = context_p->lit_object.literal_p;

        /* Fake an assignment to the default identifier */
        context_p->token.type = LEXER_ASSIGN;

        parser_parse_expression_statement (context_p, PARSE_EXPR_NO_COMMA | PARSE_EXPR_HAS_LITERAL);
      }

      ecma_string_t *name_p = parser_new_ecma_string_from_literal (context_p->module_identifier_lit_p);

      ecma_string_t *export_name_p = ecma_get_magic_string (LIT_MAGIC_STRING_DEFAULT);

      if (parser_module_check_duplicate_export (context_p, export_name_p))
      {
        ecma_deref_ecma_string (name_p);
        ecma_deref_ecma_string (export_name_p);
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_EXPORT_IDENTIFIER);
      }

      parser_module_add_names_to_node (context_p, export_name_p, name_p);
      ecma_deref_ecma_string (name_p);
      ecma_deref_ecma_string (export_name_p);
      break;
    }
    case LEXER_MULTIPLY:
    {
      lexer_next_token (context_p);

      ecma_module_node_t **target_node_list_p = &(JERRY_CONTEXT (module_current_p)->star_exports_p);

      if (lexer_token_is_identifier (context_p, "as", 2))
      {
        target_node_list_p = &(JERRY_CONTEXT (module_current_p)->indirect_exports_p);

        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_IDENT_LITERAL)
        {
          parser_raise_error (context_p, PARSER_ERR_IDENTIFIER_EXPECTED);
        }

        lexer_construct_literal_object (context_p, &context_p->token.lit_location, LEXER_NEW_IDENT_LITERAL);

        lexer_literal_t *literal_p = PARSER_GET_LITERAL (context_p->lit_object.index);
        ecma_string_t *export_name_p = parser_new_ecma_string_from_literal (literal_p);

        if (parser_module_check_duplicate_export (context_p, export_name_p))
        {
          ecma_deref_ecma_string (export_name_p);
          parser_raise_error (context_p, PARSER_ERR_DUPLICATED_EXPORT_IDENTIFIER);
        }

        ecma_string_t *local_name_p = ecma_get_magic_string (LIT_MAGIC_STRING_ASTERISK_CHAR);
        parser_module_add_names_to_node (context_p, export_name_p, local_name_p);
        ecma_deref_ecma_string (export_name_p);

        lexer_next_token (context_p);
      }

      if (!lexer_token_is_identifier (context_p, "from", 4))
      {
        parser_raise_error (context_p, PARSER_ERR_FROM_EXPECTED);
      }

      lexer_next_token (context_p);
      parser_module_handle_module_specifier (context_p, target_node_list_p);
      return false;
    }
    case LEXER_KEYW_VAR:
    case LEXER_KEYW_LET:
    case LEXER_KEYW_CONST:
    {
      context_p->status_flags |= PARSER_MODULE_STORE_IDENT;
      parser_parse_var_statement (context_p);
      break;
    }
    case LEXER_KEYW_CLASS:
    {
      context_p->status_flags |= PARSER_MODULE_STORE_IDENT;
      parser_parse_class (context_p, true);
      consume_last_statement = true;
      break;
    }
    case LEXER_KEYW_FUNCTION:
    {
      context_p->status_flags |= PARSER_MODULE_STORE_IDENT;
      parser_parse_function_statement (context_p);
      consume_last_statement = true;
      break;
    }
    case LEXER_LEFT_BRACE:
    {
      parser_module_parse_export_clause (context_p);

      if (lexer_token_is_identifier (context_p, "from", 4))
      {
        lexer_next_token (context_p);
        parser_module_handle_module_specifier (context_p, &(JERRY_CONTEXT (module_current_p)->indirect_exports_p));
        return false;
      }
      break;
    }
    default:
    {
      parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_MULTIPLY_LITERAL_EXPECTED);
      break;
    }
  }

  context_p->status_flags &= (uint32_t) ~(PARSER_MODULE_DEFAULT_CLASS_OR_FUNC | PARSER_MODULE_STORE_IDENT);
  parser_module_append_names (context_p, &(JERRY_CONTEXT (module_current_p)->local_exports_p));

  return consume_last_statement;
} /* parser_parse_export_statement */
#endif /* JERRY_MODULE_SYSTEM */

/**
 * Parse label statement.
 */
static void
parser_parse_label (parser_context_t *context_p) /**< context */
{
  parser_stack_iterator_t iterator;
  parser_label_statement_t label_statement;

  parser_stack_iterator_init (context_p, &iterator);

  while (true)
  {
    uint8_t type = parser_stack_iterator_read_uint8 (&iterator);
    if (type == PARSER_STATEMENT_START)
    {
      break;
    }

    if (type == PARSER_STATEMENT_LABEL)
    {
      parser_stack_iterator_skip (&iterator, 1);
      parser_stack_iterator_read (&iterator, &label_statement, sizeof (parser_label_statement_t));
      parser_stack_iterator_skip (&iterator, sizeof (parser_label_statement_t));

      if (lexer_current_is_literal (context_p, &label_statement.label_ident))
      {
        parser_raise_error (context_p, PARSER_ERR_DUPLICATED_LABEL);
      }
    }
    else
    {
      parser_stack_iterator_skip (&iterator, parser_statement_length (type));
    }
  }

  label_statement.label_ident = context_p->token.lit_location;
  label_statement.break_list_p = NULL;
  parser_stack_push (context_p, &label_statement, sizeof (parser_label_statement_t));
  parser_stack_push_uint8 (context_p, PARSER_STATEMENT_LABEL);
  parser_stack_iterator_init (context_p, &context_p->last_statement);
} /* parser_parse_label */

/**
 * Strict mode types for statement parsing.
 */
typedef enum
{
  PARSER_USE_STRICT_NOT_FOUND = 0, /**< 'use strict' directive is not found */
  PARSER_USE_STRICT_FOUND = 1, /**< 'use strict' directive is found but strict mode has already been enabled */
  PARSER_USE_STRICT_SET = 2, /**< strict mode is enabled after 'use strict' directive is found */
} parser_strict_mode_type_t;

/**
 * Parse statements.
 */
void
parser_parse_statements (parser_context_t *context_p) /**< context */
{
  /* Statement parsing cannot be nested. */
  JERRY_ASSERT (context_p->last_statement.current_p == NULL);
  parser_stack_push_uint8 (context_p, PARSER_STATEMENT_START);
  parser_stack_iterator_init (context_p, &context_p->last_statement);

  while (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_STRING_LITERAL)
  {
    lexer_lit_location_t lit_location;
    parser_strict_mode_type_t strict_mode = PARSER_USE_STRICT_NOT_FOUND;

    JERRY_ASSERT (context_p->stack_depth <= 1);
#ifndef JERRY_NDEBUG
    JERRY_ASSERT (context_p->context_stack_depth == context_p->stack_depth);
#endif /* !JERRY_NDEBUG */

    if (lexer_string_is_use_strict (context_p))
    {
      strict_mode = PARSER_USE_STRICT_FOUND;

      if (!(context_p->status_flags & PARSER_IS_STRICT))
      {
        /* The next token should be parsed in strict mode. */
        context_p->status_flags |= PARSER_IS_STRICT;
        strict_mode = PARSER_USE_STRICT_SET;
      }
    }

    lit_location = context_p->token.lit_location;
    lexer_next_token (context_p);

    if (!lexer_string_is_directive (context_p))
    {
      /* The string is part of an expression statement. */
      if (strict_mode == PARSER_USE_STRICT_SET)
      {
        context_p->status_flags &= (uint32_t) ~PARSER_IS_STRICT;
      }

      lexer_construct_literal_object (context_p, &lit_location, LEXER_STRING_LITERAL);
      parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_LITERAL);
      /* The extra_value is used for saving the token. */
      context_p->token.extra_value = context_p->token.type;
      context_p->token.type = LEXER_EXPRESSION_START;
      break;
    }

#if JERRY_PARSER_DUMP_BYTE_CODE
    if (strict_mode == PARSER_USE_STRICT_SET && context_p->is_show_opcodes)
    {
      JERRY_DEBUG_MSG ("  Note: switch to strict mode\n\n");
    }
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

    if (strict_mode != PARSER_USE_STRICT_NOT_FOUND && (context_p->status_flags & PARSER_FUNCTION_HAS_COMPLEX_ARGUMENT))
    {
      parser_raise_error (context_p, PARSER_ERR_USE_STRICT_NOT_ALLOWED);
    }

    if (context_p->token.type == LEXER_SEMICOLON)
    {
      lexer_next_token (context_p);
    }

    /* The last directive prologue can be the result of the script. */
    if (!(context_p->status_flags & PARSER_IS_FUNCTION)
        && (context_p->token.type != LEXER_LITERAL || context_p->token.lit_location.type != LEXER_STRING_LITERAL))
    {
      lexer_construct_literal_object (context_p, &lit_location, LEXER_STRING_LITERAL);
      parser_emit_cbc_literal_from_token (context_p, CBC_PUSH_LITERAL);
      parser_emit_cbc (context_p, CBC_POP_BLOCK);
      parser_flush_cbc (context_p);
      break;
    }
  }

  if (context_p->status_flags & PARSER_IS_STRICT && context_p->status_flags & PARSER_HAS_NON_STRICT_ARG)
  {
    parser_raise_error (context_p, PARSER_ERR_NON_STRICT_ARG_DEFINITION);
  }

  while (context_p->token.type != LEXER_EOS || context_p->stack_top_uint8 != PARSER_STATEMENT_START)
  {
#ifndef JERRY_NDEBUG
    JERRY_ASSERT (context_p->stack_depth == context_p->context_stack_depth);
#endif /* !JERRY_NDEBUG */

    JERRY_ASSERT (context_p->stack_top_uint8 != PARSER_STATEMENT_PRIVATE_SCOPE
                  && context_p->stack_top_uint8 != PARSER_STATEMENT_PRIVATE_CONTEXT);

    switch (context_p->token.type)
    {
      case LEXER_SEMICOLON:
      {
        break;
      }

      case LEXER_RIGHT_BRACE:
      {
        if (parser_statement_flags[context_p->stack_top_uint8] & PARSER_STATM_SINGLE_STATM)
        {
          parser_raise_error (context_p, PARSER_ERR_STATEMENT_EXPECTED);
        }
        break;
      }

      case LEXER_LEFT_BRACE:
      {
        if (context_p->next_scanner_info_p->source_p == context_p->source_p)
        {
          parser_push_block_context (context_p, false);
        }
        else
        {
          parser_stack_push_uint8 (context_p, PARSER_STATEMENT_BLOCK);
        }

        parser_stack_iterator_init (context_p, &context_p->last_statement);
        lexer_next_token (context_p);
        continue;
      }

      case LEXER_KEYW_VAR:
      case LEXER_KEYW_LET:
      case LEXER_KEYW_CONST:
      {
        parser_parse_var_statement (context_p);
        break;
      }
      case LEXER_KEYW_CLASS:
      {
        parser_validate_lexical_context (context_p);
        parser_parse_class (context_p, true);
        goto consume_last_statement;
      }
#if JERRY_MODULE_SYSTEM
      case LEXER_KEYW_IMPORT:
      {
        parser_parse_import_statement (context_p);
        break;
      }

      case LEXER_KEYW_EXPORT:
      {
        if (parser_parse_export_statement (context_p))
        {
          goto consume_last_statement;
        }
        break;
      }
#endif /* JERRY_MODULE_SYSTEM */

      case LEXER_KEYW_FUNCTION:
      {
        parser_parse_function_statement (context_p);
        goto consume_last_statement;
      }

      case LEXER_KEYW_IF:
      {
        parser_parse_if_statement_start (context_p);
        continue;
      }

      case LEXER_KEYW_SWITCH:
      {
        parser_parse_switch_statement_start (context_p);
        continue;
      }

      case LEXER_KEYW_DO:
      {
        parser_do_while_statement_t do_while_statement;
        parser_loop_statement_t loop;

        JERRY_ASSERT (context_p->last_cbc_opcode == PARSER_CBC_UNAVAILABLE);

        do_while_statement.start_offset = context_p->byte_code_size;
        loop.branch_list_p = NULL;

        parser_stack_push (context_p, &do_while_statement, sizeof (parser_do_while_statement_t));
        parser_stack_push (context_p, &loop, sizeof (parser_loop_statement_t));
        parser_stack_push_uint8 (context_p, PARSER_STATEMENT_DO_WHILE);
        parser_stack_iterator_init (context_p, &context_p->last_statement);
        lexer_next_token (context_p);
        continue;
      }

      case LEXER_KEYW_WHILE:
      {
        parser_parse_while_statement_start (context_p);
        continue;
      }

      case LEXER_KEYW_FOR:
      {
        parser_parse_for_statement_start (context_p);
        continue;
      }

      case LEXER_KEYW_WITH:
      {
        parser_parse_with_statement_start (context_p);
        continue;
      }

      case LEXER_KEYW_TRY:
      {
        parser_try_statement_t try_statement;

        lexer_next_token (context_p);

        if (context_p->token.type != LEXER_LEFT_BRACE)
        {
          parser_raise_error (context_p, PARSER_ERR_LEFT_BRACE_EXPECTED);
        }

#ifndef JERRY_NDEBUG
        PARSER_PLUS_EQUAL_U16 (context_p->context_stack_depth, PARSER_TRY_CONTEXT_STACK_ALLOCATION);
#endif /* !JERRY_NDEBUG */

        try_statement.type = parser_try_block;
        parser_emit_cbc_ext_forward_branch (context_p, CBC_EXT_TRY_CREATE_CONTEXT, &try_statement.branch);

        try_statement.scope_stack_top = context_p->scope_stack_top;
        try_statement.scope_stack_reg_top = context_p->scope_stack_reg_top;

        if (context_p->next_scanner_info_p->source_p == context_p->source_p)
        {
          JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_BLOCK);

          if (scanner_is_context_needed (context_p, PARSER_CHECK_BLOCK_CONTEXT))
          {
            parser_emit_cbc_ext (context_p, CBC_EXT_TRY_CREATE_ENV);
          }

          scanner_create_variables (context_p, SCANNER_CREATE_VARS_NO_OPTS);
        }

        parser_stack_push (context_p, &try_statement, sizeof (parser_try_statement_t));
        parser_stack_push_uint8 (context_p, PARSER_STATEMENT_TRY);
        parser_stack_iterator_init (context_p, &context_p->last_statement);
        lexer_next_token (context_p);
        continue;
      }

      case LEXER_KEYW_DEFAULT:
      {
        parser_parse_default_statement (context_p);
        continue;
      }

      case LEXER_KEYW_CASE:
      {
        parser_parse_case_statement (context_p);
        continue;
      }

      case LEXER_KEYW_BREAK:
      {
        parser_parse_break_statement (context_p);
        break;
      }

      case LEXER_KEYW_CONTINUE:
      {
        parser_parse_continue_statement (context_p);
        break;
      }

      case LEXER_KEYW_THROW:
      {
        lexer_next_token (context_p);
        if (context_p->token.flags & LEXER_WAS_NEWLINE)
        {
          parser_raise_error (context_p, PARSER_ERR_EXPRESSION_EXPECTED);
        }
        parser_parse_expression (context_p, PARSE_EXPR);
        parser_emit_cbc (context_p, CBC_THROW);
        break;
      }

      case LEXER_KEYW_RETURN:
      {
        if (!(context_p->status_flags & PARSER_IS_FUNCTION))
        {
          parser_raise_error (context_p, PARSER_ERR_INVALID_RETURN);
        }
        if (context_p->status_flags & PARSER_IS_CLASS_STATIC_BLOCK)
        {
          parser_raise_error (context_p, PARSER_ERR_INVALID_RETURN);
        }

        lexer_next_token (context_p);

        if ((context_p->token.flags & LEXER_WAS_NEWLINE) || context_p->token.type == LEXER_SEMICOLON
            || context_p->token.type == LEXER_EOS || context_p->token.type == LEXER_RIGHT_BRACE)
        {
          if (context_p->status_flags & PARSER_IS_ASYNC_FUNCTION)
          {
            parser_emit_cbc_ext (context_p, CBC_EXT_RETURN_UNDEFINED);
            break;
          }

          parser_emit_cbc (context_p, CBC_RETURN_FUNCTION_END);
          break;
        }

        parser_parse_expression (context_p, PARSE_EXPR);

        if (context_p->last_cbc_opcode == CBC_PUSH_LITERAL)
        {
          context_p->last_cbc_opcode = CBC_RETURN_WITH_LITERAL;
          break;
        }

        parser_emit_cbc (context_p, CBC_RETURN);
        break;
      }

      case LEXER_KEYW_DEBUGGER:
      {
        lexer_next_token (context_p);
        break;
      }

      case LEXER_LITERAL:
      {
        if (context_p->token.lit_location.type == LEXER_IDENT_LITERAL)
        {
          if (JERRY_UNLIKELY (lexer_check_next_character (context_p, LIT_CHAR_COLON)))
          {
            parser_parse_label (context_p);
            lexer_consume_next_character (context_p);
            lexer_next_token (context_p);
            continue;
          }

          if (JERRY_UNLIKELY (lexer_token_is_let (context_p)))
          {
            if (context_p->next_scanner_info_p->source_p == context_p->source_p)
            {
              if (context_p->next_scanner_info_p->type == SCANNER_TYPE_LET_EXPRESSION)
              {
                scanner_release_next (context_p, sizeof (scanner_info_t));
              }

              if (context_p->status_flags & PARSER_IS_FUNCTION)
              {
                parser_parse_expression_statement (context_p, PARSE_EXPR);
                break;
              }

              parser_parse_block_expression (context_p, PARSE_EXPR);
              break;
            }

            context_p->token.type = LEXER_KEYW_LET;
            parser_parse_var_statement (context_p);
            break;
          }

          if (JERRY_UNLIKELY (lexer_token_is_async (context_p))
              && context_p->next_scanner_info_p->source_p == context_p->source_p)
          {
            bool is_statement = true;

            if (context_p->next_scanner_info_p->type == SCANNER_TYPE_FUNCTION)
            {
              is_statement = (context_p->next_scanner_info_p->u8_arg & SCANNER_FUNCTION_STATEMENT) != 0;

              JERRY_ASSERT (!is_statement || (context_p->next_scanner_info_p->u8_arg & SCANNER_FUNCTION_ASYNC));
            }
            else
            {
              JERRY_ASSERT (context_p->next_scanner_info_p->type == SCANNER_TYPE_ERR_ASYNC_FUNCTION);

              scanner_release_next (context_p, sizeof (scanner_info_t));
            }

            if (is_statement)
            {
              if (parser_statement_flags[context_p->stack_top_uint8] & PARSER_STATM_SINGLE_STATM)
              {
                parser_raise_error (context_p, PARSER_ERR_LEXICAL_SINGLE_STATEMENT);
              }

              lexer_next_token (context_p);
              JERRY_ASSERT (context_p->token.type == LEXER_KEYW_FUNCTION);
              continue;
            }
          }
        }
        /* FALLTHRU */
      }

      default:
      {
        int options = PARSE_EXPR;

        if (context_p->token.type == LEXER_EXPRESSION_START)
        {
          /* Restore the token type form the extra_value. */
          context_p->token.type = context_p->token.extra_value;
          options |= PARSE_EXPR_HAS_LITERAL;
        }

        if (context_p->status_flags & PARSER_IS_FUNCTION)
        {
          parser_parse_expression_statement (context_p, options);
        }
        else
        {
          parser_parse_block_expression (context_p, options);
        }

        break;
      }
    }

    parser_flush_cbc (context_p);

    if (context_p->token.type == LEXER_RIGHT_BRACE)
    {
      if (context_p->stack_top_uint8 == PARSER_STATEMENT_BLOCK)
      {
        parser_stack_pop_uint8 (context_p);
        parser_stack_iterator_init (context_p, &context_p->last_statement);
        lexer_next_token (context_p);
      }
      else if (context_p->stack_top_uint8 == PARSER_STATEMENT_BLOCK_SCOPE
               || context_p->stack_top_uint8 == PARSER_STATEMENT_BLOCK_CONTEXT)
      {
        parser_pop_block_context (context_p);
        lexer_next_token (context_p);
      }
      else if (context_p->stack_top_uint8 == PARSER_STATEMENT_SWITCH
               || context_p->stack_top_uint8 == PARSER_STATEMENT_SWITCH_NO_DEFAULT)
      {
        int has_default = (context_p->stack_top_uint8 == PARSER_STATEMENT_SWITCH);
        parser_loop_statement_t loop;
        parser_switch_statement_t switch_statement;

        parser_stack_pop_uint8 (context_p);
        parser_stack_pop (context_p, &loop, sizeof (parser_loop_statement_t));
        parser_stack_pop (context_p, &switch_statement, sizeof (parser_switch_statement_t));
        parser_stack_iterator_init (context_p, &context_p->last_statement);

        JERRY_ASSERT (switch_statement.branch_list_p == NULL);

        if (!has_default)
        {
          parser_set_branch_to_current_position (context_p, &switch_statement.default_branch);
        }

        parser_set_breaks_to_current_position (context_p, loop.branch_list_p);
        lexer_next_token (context_p);

        if (context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_SCOPE
            || context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_CONTEXT)
        {
          parser_pop_block_context (context_p);
        }
      }
      else if (context_p->stack_top_uint8 == PARSER_STATEMENT_TRY)
      {
        parser_parse_try_statement_end (context_p);
      }
      else if (context_p->stack_top_uint8 == PARSER_STATEMENT_START)
      {
        if (context_p->status_flags & PARSER_IS_CLOSURE)
        {
          parser_stack_pop_uint8 (context_p);
          context_p->last_statement.current_p = NULL;
          /* There is no lexer_next_token here, since the
           * next token belongs to the parent context. */
          return;
        }
        parser_raise_error (context_p, PARSER_ERR_INVALID_RIGHT_SQUARE);
      }
    }
    else if (context_p->token.type == LEXER_SEMICOLON)
    {
      lexer_next_token (context_p);
    }
    else if (context_p->token.type != LEXER_EOS && !(context_p->token.flags & LEXER_WAS_NEWLINE))
    {
      parser_raise_error (context_p, PARSER_ERR_SEMICOLON_EXPECTED);
    }

consume_last_statement:
    while (true)
    {
      switch (context_p->stack_top_uint8)
      {
        case PARSER_STATEMENT_LABEL:
        {
          parser_label_statement_t label;

          parser_stack_pop_uint8 (context_p);
          parser_stack_pop (context_p, &label, sizeof (parser_label_statement_t));
          parser_stack_iterator_init (context_p, &context_p->last_statement);

          parser_set_breaks_to_current_position (context_p, label.break_list_p);
          continue;
        }

        case PARSER_STATEMENT_IF:
        {
          if (parser_parse_if_statement_end (context_p))
          {
            break;
          }
          continue;
        }

        case PARSER_STATEMENT_ELSE:
        {
          parser_if_else_statement_t else_statement;

          parser_stack_pop_uint8 (context_p);
          parser_stack_pop (context_p, &else_statement, sizeof (parser_if_else_statement_t));
          parser_stack_iterator_init (context_p, &context_p->last_statement);

          parser_set_branch_to_current_position (context_p, &else_statement.branch);
          continue;
        }

        case PARSER_STATEMENT_DO_WHILE:
        {
          parser_parse_do_while_statement_end (context_p);
          if (context_p->token.type == LEXER_SEMICOLON)
          {
            lexer_next_token (context_p);
          }
          continue;
        }

        case PARSER_STATEMENT_WHILE:
        {
          parser_parse_while_statement_end (context_p);
          continue;
        }

        case PARSER_STATEMENT_FOR:
        {
          parser_parse_for_statement_end (context_p);
          continue;
        }

        case PARSER_STATEMENT_FOR_IN:
        case PARSER_STATEMENT_FOR_OF:
        case PARSER_STATEMENT_FOR_AWAIT_OF:
        {
          parser_for_in_of_statement_t for_in_of_statement;
          parser_loop_statement_t loop;

          uint8_t for_type = context_p->stack_top_uint8;

          parser_stack_pop_uint8 (context_p);
          parser_stack_pop (context_p, &loop, sizeof (parser_loop_statement_t));
          parser_stack_pop (context_p, &for_in_of_statement, sizeof (parser_for_in_of_statement_t));
          parser_stack_iterator_init (context_p, &context_p->last_statement);

          parser_set_continues_to_current_position (context_p, loop.branch_list_p);

          parser_flush_cbc (context_p);

          uint16_t stack_allocation = PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION;
          if (for_type != PARSER_STATEMENT_FOR_IN)
          {
            stack_allocation = (for_type == PARSER_STATEMENT_FOR_OF ? PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION
                                                                    : PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION);
          }

          PARSER_MINUS_EQUAL_U16 (context_p->stack_depth, stack_allocation);
#ifndef JERRY_NDEBUG
          PARSER_MINUS_EQUAL_U16 (context_p->context_stack_depth, stack_allocation);
#endif /* !JERRY_NDEBUG */

          cbc_ext_opcode_t opcode = CBC_EXT_BRANCH_IF_FOR_IN_HAS_NEXT;

          if (for_type != PARSER_STATEMENT_FOR_IN)
          {
            opcode = (for_type == PARSER_STATEMENT_FOR_OF ? CBC_EXT_BRANCH_IF_FOR_OF_HAS_NEXT
                                                          : CBC_EXT_BRANCH_IF_FOR_AWAIT_OF_HAS_NEXT);
          }

          parser_emit_cbc_ext_backward_branch (context_p, opcode, for_in_of_statement.start_offset);

          parser_set_breaks_to_current_position (context_p, loop.branch_list_p);
          parser_set_branch_to_current_position (context_p, &for_in_of_statement.branch);

          if (context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_SCOPE
              || context_p->stack_top_uint8 == PARSER_STATEMENT_PRIVATE_CONTEXT)
          {
            parser_pop_block_context (context_p);
          }

          continue;
        }

        case PARSER_STATEMENT_WITH:
        {
          parser_parse_with_statement_end (context_p);
          continue;
        }

        default:
        {
          break;
        }
      }
      break;
    }
  }

  parser_stack_pop_uint8 (context_p);
  context_p->last_statement.current_p = NULL;

  if (context_p->status_flags & PARSER_IS_CLOSURE)
  {
    parser_raise_error (context_p, PARSER_ERR_STATEMENT_EXPECTED);
  }
} /* parser_parse_statements */

/**
 * Free jumps stored on the stack if a parse error is occurred.
 */
void JERRY_ATTR_NOINLINE
parser_free_jumps (parser_stack_iterator_t iterator) /**< iterator position */
{
  while (true)
  {
    uint8_t type = parser_stack_iterator_read_uint8 (&iterator);
    parser_branch_node_t *branch_list_p = NULL;

    switch (type)
    {
      case PARSER_STATEMENT_START:
      {
        return;
      }

      case PARSER_STATEMENT_LABEL:
      {
        parser_label_statement_t label;

        parser_stack_iterator_skip (&iterator, 1);
        parser_stack_iterator_read (&iterator, &label, sizeof (parser_label_statement_t));
        parser_stack_iterator_skip (&iterator, sizeof (parser_label_statement_t));
        branch_list_p = label.break_list_p;
        break;
      }

      case PARSER_STATEMENT_SWITCH:
      case PARSER_STATEMENT_SWITCH_NO_DEFAULT:
      {
        parser_switch_statement_t switch_statement;
        parser_loop_statement_t loop;

        parser_stack_iterator_skip (&iterator, 1);
        parser_stack_iterator_read (&iterator, &loop, sizeof (parser_loop_statement_t));
        parser_stack_iterator_skip (&iterator, sizeof (parser_loop_statement_t));
        parser_stack_iterator_read (&iterator, &switch_statement, sizeof (parser_switch_statement_t));
        parser_stack_iterator_skip (&iterator, sizeof (parser_switch_statement_t));

        branch_list_p = switch_statement.branch_list_p;
        while (branch_list_p != NULL)
        {
          parser_branch_node_t *next_p = branch_list_p->next_p;
          parser_free (branch_list_p, sizeof (parser_branch_node_t));
          branch_list_p = next_p;
        }
        branch_list_p = loop.branch_list_p;
        break;
      }

      case PARSER_STATEMENT_DO_WHILE:
      case PARSER_STATEMENT_WHILE:
      case PARSER_STATEMENT_FOR:
      case PARSER_STATEMENT_FOR_IN:
      case PARSER_STATEMENT_FOR_OF:
      {
        parser_loop_statement_t loop;

        parser_stack_iterator_skip (&iterator, 1);
        parser_stack_iterator_read (&iterator, &loop, sizeof (parser_loop_statement_t));
        parser_stack_iterator_skip (&iterator, parser_statement_length (type) - 1);
        branch_list_p = loop.branch_list_p;
        break;
      }

      default:
      {
        parser_stack_iterator_skip (&iterator, parser_statement_length (type));
        continue;
      }
    }

    while (branch_list_p != NULL)
    {
      parser_branch_node_t *next_p = branch_list_p->next_p;
      parser_free (branch_list_p, sizeof (parser_branch_node_t));
      branch_list_p = next_p;
    }
  }
} /* parser_free_jumps */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_PARSER */
