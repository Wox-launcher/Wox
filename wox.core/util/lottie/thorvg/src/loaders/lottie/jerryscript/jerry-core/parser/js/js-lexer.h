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

#ifndef JS_LEXER_H
#define JS_LEXER_H

#include "ecma-globals.h"

#include "common.h"

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_lexer Lexer
 * @{
 */

/**
 * Flags for lexer_parse_identifier.
 */
typedef enum
{
  LEXER_PARSE_NO_OPTS = 0, /**< no options */
  LEXER_PARSE_CHECK_KEYWORDS = (1 << 0), /**< check keywords */
  LEXER_PARSE_NO_STRICT_IDENT_ERROR = (1 << 1), /**< do not throw exception for strict mode keywords */
  LEXER_PARSE_CHECK_START_AND_RETURN = (1 << 2), /**< check identifier start and return */
  LEXER_PARSE_CHECK_PART_AND_RETURN = (1 << 3), /**< check identifier part and return */
} lexer_parse_options_t;

/**
 * Lexer token types.
 */
typedef enum
{
  LEXER_EOS, /**< end of source */

  /* Primary expressions */
  LEXER_LITERAL, /**< literal token */
  LEXER_KEYW_THIS, /**< this */
  LEXER_LIT_TRUE, /**< true (not a keyword!) */
  LEXER_LIT_FALSE, /**< false (not a keyword!) */
  LEXER_LIT_NULL, /**< null (not a keyword!) */
  LEXER_TEMPLATE_LITERAL, /**< multi segment template literal */
  LEXER_THREE_DOTS, /**< ... (rest or spread operator) */

/* Unary operators
 * IMPORTANT: update CBC_UNARY_OP_TOKEN_TO_OPCODE and
 *            CBC_UNARY_LVALUE_OP_TOKEN_TO_OPCODE after changes. */
#define LEXER_IS_UNARY_OP_TOKEN(token_type)        ((token_type) >= LEXER_PLUS && (token_type) <= LEXER_DECREASE)
#define LEXER_IS_UNARY_LVALUE_OP_TOKEN(token_type) ((token_type) >= LEXER_KEYW_DELETE && (token_type) <= LEXER_DECREASE)

  LEXER_PLUS, /**< "+" */
  LEXER_NEGATE, /**< "-" */
  LEXER_LOGICAL_NOT, /**< "!" */
  LEXER_BIT_NOT, /**< "~" */
  LEXER_KEYW_VOID, /**< void */
  LEXER_KEYW_TYPEOF, /**< typeof */
  LEXER_KEYW_AWAIT, /**< await */
  LEXER_KEYW_DELETE, /**< delete */
  LEXER_INCREASE, /**< "++" */
  LEXER_DECREASE, /**< "--" */

/* Binary operators
 * IMPORTANT: update CBC_BINARY_OP_TOKEN_TO_OPCODE,
 *            CBC_BINARY_LVALUE_OP_TOKEN_TO_OPCODE and
 *            parser_binary_precedence_table after changes. */
/**
 * Index of first binary operation opcode.
 */
#define LEXER_FIRST_BINARY_OP LEXER_ASSIGN

/**
 * Index of last binary operation opcode.
 */
#define LEXER_LAST_BINARY_OP LEXER_EXPONENTIATION

/**
 * Checks whether the token is a binary operation token.
 */
#define LEXER_IS_BINARY_OP_TOKEN(token_type) \
  ((token_type) >= LEXER_FIRST_BINARY_OP && (token_type) <= LEXER_LAST_BINARY_OP)
/**
 * Checks whether the token is an lvalue (assignment) operation token.
 */
#define LEXER_IS_BINARY_LVALUE_OP_TOKEN(token_type) \
  ((token_type) >= LEXER_ASSIGN && (token_type) <= LEXER_ASSIGN_BIT_XOR)
/**
 * Checks whether the token is a non-lvalue (assignment) operation token.
 */
#define LEXER_IS_BINARY_NON_LVALUE_OP_TOKEN(token_type) \
  ((token_type) >= LEXER_QUESTION_MARK && (token_type) <= LEXER_LAST_BINARY_OP)

  LEXER_ASSIGN, /**< "=" (prec: 3) */
  LEXER_ASSIGN_ADD, /**< "+=" (prec: 3) */
  LEXER_ASSIGN_SUBTRACT, /**< "-=" (prec: 3) */
  LEXER_ASSIGN_MULTIPLY, /**< "*=" (prec: 3) */
  LEXER_ASSIGN_DIVIDE, /**< "/=" (prec: 3) */
  LEXER_ASSIGN_MODULO, /**< "%=" (prec: 3) */
  LEXER_ASSIGN_EXPONENTIATION, /**< "**=" (prec: 3) */
  LEXER_ASSIGN_NULLISH_COALESCING, /**< "??=" (prec: 3) */
  LEXER_ASSIGN_LOGICAL_OR, /**< "||=" (prec: 3) */
  LEXER_ASSIGN_LOGICAL_AND, /**< "&&=" (prec: 3) */
  LEXER_ASSIGN_LEFT_SHIFT, /**< "<<=" (prec: 3) */
  LEXER_ASSIGN_RIGHT_SHIFT, /**< ">>=" (prec: 3) */
  LEXER_ASSIGN_UNS_RIGHT_SHIFT, /**< ">>>=" (prec: 3) */
  LEXER_ASSIGN_BIT_AND, /**< "&=" (prec: 3) */
  LEXER_ASSIGN_BIT_OR, /**< "|=" (prec: 3) */
  LEXER_ASSIGN_BIT_XOR, /**< "^=" (prec: 3) */
  LEXER_QUESTION_MARK, /**< "?" (prec: 4) */
  LEXER_NULLISH_COALESCING, /**< "??" (prec: 5) */
  LEXER_LOGICAL_OR, /**< "||" (prec: 6) */
  LEXER_LOGICAL_AND, /**< "&&" (prec: 7) */
  LEXER_BIT_OR, /**< "|" (prec: 8) */
  LEXER_BIT_XOR, /**< "^" (prec: 9) */
  LEXER_BIT_AND, /**< "&" (prec: 10) */
  LEXER_EQUAL, /**< "==" (prec: 11) */
  LEXER_NOT_EQUAL, /**< "!=" (prec: 11) */
  LEXER_STRICT_EQUAL, /**< "===" (prec: 11) */
  LEXER_STRICT_NOT_EQUAL, /**< "!==" (prec: 11) */
  LEXER_LESS, /**< "<" (prec: 12) */
  LEXER_GREATER, /**< ">" (prec: 12) */
  LEXER_LESS_EQUAL, /**< "<=" (prec: 12) */
  LEXER_GREATER_EQUAL, /**< ">=" (prec: 12) */
  LEXER_KEYW_IN, /**< in (prec: 12) */
  LEXER_KEYW_INSTANCEOF, /**< instanceof (prec: 12) */
  LEXER_LEFT_SHIFT, /**< "<<" (prec: 13) */
  LEXER_RIGHT_SHIFT, /**< ">>" (prec: 13) */
  LEXER_UNS_RIGHT_SHIFT, /**< ">>>" (prec: 13) */
  LEXER_ADD, /**< "+" (prec: 14) */
  LEXER_SUBTRACT, /**< "-" (prec: 14) */
  LEXER_MULTIPLY, /**< "*" (prec: 15) */
  LEXER_DIVIDE, /**< "/" (prec: 15) */
  LEXER_MODULO, /**< "%" (prec: 15) */
  LEXER_EXPONENTIATION, /**< "**" (prec: 16) */
  LEXER_LEFT_BRACE, /**< "{" */
  LEXER_LEFT_PAREN, /**< "(" */
  LEXER_LEFT_SQUARE, /**< "[" */
  LEXER_RIGHT_BRACE, /**< "}" */
  LEXER_RIGHT_PAREN, /**< ")" */
  LEXER_RIGHT_SQUARE, /**< "]" */
  LEXER_DOT, /**< "." */
  LEXER_SEMICOLON, /**< ";" */
  LEXER_COLON, /**< ":" */
  LEXER_COMMA, /**< "," */
  LEXER_ARROW, /**< "=>" */
  LEXER_HASHMARK, /**< "#" */

  LEXER_KEYW_BREAK, /**< break */
  LEXER_KEYW_DO, /**< do */
  LEXER_KEYW_CASE, /**< case  */
  LEXER_KEYW_ELSE, /**< else */
  LEXER_KEYW_NEW, /**< new */
  LEXER_KEYW_VAR, /**< var */
  LEXER_KEYW_CATCH, /**< catch */
  LEXER_KEYW_FINALLY, /**< finally */
  LEXER_KEYW_RETURN, /**< return */
  LEXER_KEYW_CONTINUE, /**< continue */
  LEXER_KEYW_FOR, /**< for */
  LEXER_KEYW_SWITCH, /**< switch */
  LEXER_KEYW_WHILE, /**< while */
  LEXER_KEYW_DEBUGGER, /**< debugger */
  LEXER_KEYW_FUNCTION, /**< function */
  LEXER_KEYW_WITH, /**< with */
  LEXER_KEYW_DEFAULT, /**< default */
  LEXER_KEYW_IF, /**< if */
  LEXER_KEYW_THROW, /**< throw */
  LEXER_KEYW_TRY, /**< try */

  LEXER_KEYW_CLASS, /**< class */
  LEXER_KEYW_EXTENDS, /**< extends */
  LEXER_KEYW_SUPER, /**< super */
  LEXER_KEYW_CONST, /**< const */
  LEXER_KEYW_EXPORT, /**< export */
  LEXER_KEYW_IMPORT, /**< import */
  LEXER_KEYW_ENUM, /**< enum */

#define LEXER_FIRST_NON_RESERVED_KEYWORD LEXER_EXPRESSION_START

  /* These are virtual tokens. */
  LEXER_EXPRESSION_START, /**< expression start */
  LEXER_PROPERTY_GETTER, /**< property getter function */
  LEXER_PROPERTY_SETTER, /**< property setter function */
  LEXER_COMMA_SEP_LIST, /**< comma separated bracketed expression list */
  LEXER_ASSIGN_GROUP_EXPR, /**< identifier for the assignment is located in a group expression */
  LEXER_ASSIGN_CONST, /**< a const binding is reassigned */
  LEXER_INVALID_PATTERN, /**< special value for invalid destructuring pattern */
  LEXER_PRIVATE_PRIMARY_EXPR, /**< private field in primary expression position */
  LEXER_ASSIGN_REFERENCE, /**< special value for reference assignment */

  /* Keywords which are not keyword tokens. */
  LEXER_KEYW_ASYNC, /**< async */
#if JERRY_MODULE_SYSTEM
  LEXER_KEYW_META, /**< meta */
#endif /* JERRY_MODULE_SYSTEM */

/* Keywords which cannot be assigned in strict mode. */
#define LEXER_FIRST_NON_STRICT_ARGUMENTS LEXER_KEYW_EVAL
  LEXER_KEYW_EVAL, /**< eval */
  LEXER_KEYW_ARGUMENTS, /**< arguments */

/* Future strict reserved words: these keywords
 * must form a group after non-reserved keywords. */
#define LEXER_FIRST_FUTURE_STRICT_RESERVED_WORD LEXER_KEYW_IMPLEMENTS
  LEXER_KEYW_IMPLEMENTS, /**< implements */
  LEXER_KEYW_PRIVATE, /**< private */
  LEXER_KEYW_PUBLIC, /**< public */
  LEXER_KEYW_INTERFACE, /**< interface */
  LEXER_KEYW_PACKAGE, /**< package */
  LEXER_KEYW_PROTECTED, /**< protected */

  /* Context dependent future strict reserved words:
   * See also: ECMA-262 v6, 11.6.2.1 */
  LEXER_KEYW_LET, /**< let */
  LEXER_KEYW_YIELD, /**< yield */
  LEXER_KEYW_STATIC, /**< static */
} lexer_token_type_t;

#define LEXER_NEWLINE_LS_PS_BYTE_1 0xe2
#define LEXER_NEWLINE_LS_PS_BYTE_23(source) \
  ((source)[1] == LIT_UTF8_2_BYTE_CODE_POINT_MIN && ((source)[2] | 0x1) == 0xa9)

#define LEXER_IS_LEFT_BRACKET(type) \
  ((type) == LEXER_LEFT_BRACE || (type) == LEXER_LEFT_PAREN || (type) == LEXER_LEFT_SQUARE)

#define LEXER_IS_RIGHT_BRACKET(type) \
  ((type) == LEXER_RIGHT_BRACE || (type) == LEXER_RIGHT_PAREN || (type) == LEXER_RIGHT_SQUARE)

#define LEXER_UNARY_OP_TOKEN_TO_OPCODE(token_type) ((((token_type) -LEXER_PLUS) * 2) + CBC_PLUS)

#define LEXER_UNARY_LVALUE_OP_TOKEN_TO_OPCODE(token_type) ((((token_type) -LEXER_INCREASE) * 6) + CBC_PRE_INCR)

#define LEXER_BINARY_OP_TOKEN_TO_OPCODE(token_type) ((uint16_t) ((((token_type) -LEXER_BIT_OR) * 3) + CBC_BIT_OR))

#define LEXER_BINARY_LVALUE_OP_TOKEN_TO_OPCODE(token_type) \
  ((cbc_opcode_t) ((((token_type) -LEXER_ASSIGN_ADD) * 2) + CBC_ASSIGN_ADD))

/**
 * Maximum local buffer size for identifiers which contains escape sequences.
 */
#define LEXER_MAX_LITERAL_LOCAL_BUFFER_SIZE 48

/**
 * Lexer newline flags.
 */
typedef enum
{
  LEXER_WAS_NEWLINE = (1u << 0), /**< newline was seen */
  LEXER_NO_SKIP_SPACES = (1u << 1) /**< ignore skip spaces */
} lexer_newline_flags_t;

/**
 * Lexer object identifier parse options.
 */
typedef enum
{
  LEXER_OBJ_IDENT_NO_OPTS = 0, /**< no options */
  LEXER_OBJ_IDENT_ONLY_IDENTIFIERS = (1u << 0), /**< only identifiers are accepted */
  LEXER_OBJ_IDENT_CLASS_IDENTIFIER = (1u << 1), /**< expect identifier inside a class body */
  LEXER_OBJ_IDENT_CLASS_NO_STATIC = (1u << 2), /**< static keyword was not present before the identifier */
  LEXER_OBJ_IDENT_OBJECT_PATTERN = (1u << 3), /**< parse "get"/"set" as string literal in object pattern */
  LEXER_OBJ_IDENT_CLASS_PRIVATE = (1u << 4), /**< static keyword was not present before the identifier */
  LEXER_OBJ_IDENT_SET_FUNCTION_START = 0, /**< set function start (disabled) */
} lexer_obj_ident_opts_t;

/**
 * Lexer string options.
 */
typedef enum
{
  LEXER_STRING_NO_OPTS = (1u << 0), /**< no options */
  LEXER_STRING_RAW = (1u << 1), /**< raw string ECMAScript v6, 11.8.6.1: TVR */
} lexer_string_options_t;

/**
 * Lexer number types.
 */
typedef enum
{
  LEXER_NUMBER_DECIMAL, /**< decimal number */
  LEXER_NUMBER_HEXADECIMAL, /**< hexadecimal number */
  LEXER_NUMBER_OCTAL, /**< octal number */
  LEXER_NUMBER_BINARY, /**< binary number */
#if JERRY_BUILTIN_BIGINT
  LEXER_NUMBER_BIGINT, /**< bigint number */
#endif /* JERRY_BUILTIN_BIGINT */
} lexer_number_type_t;

/**
 * Lexer literal flags.
 **/
typedef enum
{
  LEXER_LIT_LOCATION_NO_OPTS = 0, /**< no options */
  LEXER_LIT_LOCATION_HAS_ESCAPE = (1 << 0), /**< binding has escape */
  LEXER_LIT_LOCATION_IS_ASCII = (1 << 1), /**< all characters are ascii characters */
} lexer_lit_location_flags_t;

/**
 * Lexer character (string / identifier) literal data.
 */
typedef struct
{
  const uint8_t *char_p; /**< start of identifier or string token */
  prop_length_t length; /**< length or index of a literal */
  uint8_t type; /**< type of the current literal */
  uint8_t status_flags; /**< any combination of lexer_lit_location_flags_t status bits */
} lexer_lit_location_t;

/**
 * Lexer token.
 */
typedef struct
{
  uint8_t type; /**< token type */
  uint8_t keyword_type; /**< keyword type for identifiers */
  uint8_t extra_value; /**< helper value for different purposes */
  uint8_t flags; /**< flag bits for the current token */
  parser_line_counter_t line; /**< token start line */
  parser_line_counter_t column; /**< token start column */
  lexer_lit_location_t lit_location; /**< extra data for character literals */
} lexer_token_t;

/**
 * Literal data set by lexer_construct_literal_object.
 */
typedef struct
{
  lexer_literal_t *literal_p; /**< pointer to the literal object */
  uint16_t index; /**< literal index */
} lexer_lit_object_t;

/**
 * @}
 * @}
 * @}
 */

#endif /* !JS_LEXER_H */
