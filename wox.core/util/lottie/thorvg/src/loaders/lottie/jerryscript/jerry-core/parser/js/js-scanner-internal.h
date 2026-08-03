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

#ifndef JS_SCANNER_INTERNAL_H
#define JS_SCANNER_INTERNAL_H

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
 * Scan mode types.
 */
typedef enum
{
  SCAN_MODE_PRIMARY_EXPRESSION, /**< scanning primary expression */
  SCAN_MODE_PRIMARY_EXPRESSION_AFTER_NEW, /**< scanning primary expression after new */
  SCAN_MODE_POST_PRIMARY_EXPRESSION, /**< scanning post primary expression */
  SCAN_MODE_PRIMARY_EXPRESSION_END, /**< scanning primary expression end */
  SCAN_MODE_STATEMENT, /**< scanning statement */
  SCAN_MODE_STATEMENT_OR_TERMINATOR, /**< scanning statement or statement end */
  SCAN_MODE_STATEMENT_END, /**< scanning statement end */
  SCAN_MODE_VAR_STATEMENT, /**< scanning var statement */
  SCAN_MODE_PROPERTY_NAME, /**< scanning property name */
  SCAN_MODE_FUNCTION_ARGUMENTS, /**< scanning function arguments */
  SCAN_MODE_CONTINUE_FUNCTION_ARGUMENTS, /**< continue scanning function arguments */
  SCAN_MODE_BINDING, /**< array or object binding */
  SCAN_MODE_CLASS_DECLARATION, /**< scanning class declaration */
  SCAN_MODE_CLASS_BODY, /**< scanning class body */
  SCAN_MODE_CLASS_BODY_NO_SCAN, /**< scanning class body without calling lexer_scan_identifier */
} scan_modes_t;

/**
 * Scan stack mode types.
 */
typedef enum
{
  SCAN_STACK_SCRIPT, /**< script */
  SCAN_STACK_SCRIPT_FUNCTION, /**< script is a function body */
  SCAN_STACK_BLOCK_STATEMENT, /**< block statement group */
  SCAN_STACK_FUNCTION_STATEMENT, /**< function statement */
  SCAN_STACK_FUNCTION_EXPRESSION, /**< function expression */
  SCAN_STACK_FUNCTION_PROPERTY, /**< function expression in an object literal */
  SCAN_STACK_FUNCTION_ARROW, /**< arrow function expression */
  SCAN_STACK_SWITCH_BLOCK, /**< block part of "switch" statement */
  SCAN_STACK_IF_STATEMENT, /**< statement part of "if" statements */
  SCAN_STACK_WITH_STATEMENT, /**< statement part of "with" statements */
  SCAN_STACK_WITH_EXPRESSION, /**< expression part of "with" statements */
  SCAN_STACK_DO_STATEMENT, /**< statement part of "do" statements */
  SCAN_STACK_DO_EXPRESSION, /**< expression part of "do" statements */
  SCAN_STACK_WHILE_EXPRESSION, /**< expression part of "while" iterator */
  SCAN_STACK_PAREN_EXPRESSION, /**< expression in brackets */
  SCAN_STACK_STATEMENT_WITH_EXPR, /**< statement which starts with expression enclosed in brackets */
  SCAN_STACK_BINDING_INIT, /**< post processing after a single initializer */
  SCAN_STACK_BINDING_LIST_INIT, /**< post processing after an initializer list */
  SCAN_STACK_LET, /**< let statement */
  SCAN_STACK_CONST, /**< const statement */
  /* The SCANNER_IS_FOR_START macro needs to be updated when the following constants are reordered. */
  SCAN_STACK_VAR, /**< var statement */
  SCAN_STACK_FOR_VAR_START, /**< start of "for" iterator with var statement */
  SCAN_STACK_FOR_LET_START, /**< start of "for" iterator with let statement */
  SCAN_STACK_FOR_CONST_START, /**< start of "for" iterator with const statement */
  SCAN_STACK_FOR_START, /**< start of "for" iterator */
  SCAN_STACK_FOR_CONDITION, /**< condition part of "for" iterator */
  SCAN_STACK_FOR_EXPRESSION, /**< expression part of "for" iterator */
  SCAN_STACK_SWITCH_EXPRESSION, /**< expression part of "switch" statement */
  SCAN_STACK_CASE_STATEMENT, /**< case statement inside a switch statement */
  SCAN_STACK_COLON_EXPRESSION, /**< expression between a question mark and colon */
  SCAN_STACK_TRY_STATEMENT, /**< try statement */
  SCAN_STACK_CATCH_STATEMENT, /**< catch statement */
  SCAN_STACK_ARRAY_LITERAL, /**< array literal or destructuring assignment or binding */
  SCAN_STACK_OBJECT_LITERAL, /**< object literal group */
  SCAN_STACK_PROPERTY_ACCESSOR, /**< property accessor in square brackets */
  /* These four must be in this order. */
  SCAN_STACK_COMPUTED_PROPERTY, /**< computed property name */
  SCAN_STACK_COMPUTED_GENERATOR, /**< computed generator function */
  SCAN_STACK_COMPUTED_ASYNC, /**< computed async function */
  SCAN_STACK_COMPUTED_ASYNC_GENERATOR, /**< computed async function */
  SCAN_STACK_TEMPLATE_STRING, /**< template string */
  SCAN_STACK_TAGGED_TEMPLATE_LITERAL, /**< tagged template literal */
  SCAN_STACK_PRIVATE_BLOCK_EARLY, /**< private block for single statements (force early declarations) */
  SCAN_STACK_PRIVATE_BLOCK, /**< private block for single statements */
  SCAN_STACK_ARROW_ARGUMENTS, /**< might be arguments of an arrow function */
  SCAN_STACK_ARROW_EXPRESSION, /**< expression body of an arrow function */
  SCAN_STACK_EXPLICIT_CLASS_CONSTRUCTOR, /**< explicit class constructor */
  SCAN_STACK_IMPLICIT_CLASS_CONSTRUCTOR, /**< implicit class constructor */
  SCAN_STACK_CLASS_STATEMENT, /**< class statement */
  SCAN_STACK_CLASS_EXPRESSION, /**< class expression */
  SCAN_STACK_CLASS_EXTENDS, /**< class extends expression */
  SCAN_STACK_CLASS_FIELD_INITIALIZER, /**< class field initializer */
  SCAN_STACK_FUNCTION_PARAMETERS, /**< function parameter initializer */
  SCAN_STACK_FOR_START_PATTERN, /**< possible assignment pattern for "for" iterator */
  SCAN_STACK_USE_ASYNC, /**< an "async" identifier is used */
  SCAN_STACK_CLASS_STATIC_BLOCK, /**< class static block */
#if JERRY_MODULE_SYSTEM
  SCAN_STACK_EXPORT_DEFAULT, /**< scan primary expression after export default */
#endif /* JERRY_MODULE_SYSTEM */
} scan_stack_modes_t;

/**
 * Scanner context flag types.
 */
typedef enum
{
  SCANNER_CONTEXT_NO_FLAGS = 0, /**< no flags are set */
  SCANNER_CONTEXT_THROW_ERR_ASYNC_FUNCTION = (1 << 0), /**< throw async function error */
} scanner_context_flags_t;

/**
 * Checks whether the stack top is a for statement start.
 */
#define SCANNER_IS_FOR_START(stack_top) ((stack_top) >= SCAN_STACK_FOR_VAR_START && (stack_top) <= SCAN_STACK_FOR_START)

/**
 * Generic descriptor which stores only the start position.
 */
typedef struct
{
  const uint8_t *source_p; /**< start source byte */
} scanner_source_start_t;

/**
 * Descriptor for storing a binding literal on stack.
 */
typedef struct
{
  lexer_lit_location_t *literal_p; /**< binding literal */
} scanner_binding_literal_t;

/**
 * Flags for type member of lexer_lit_location_t structure in the literal pool.
 */
typedef enum
{
  SCANNER_LITERAL_IS_ARG = (1 << 0), /**< literal is argument */
  SCANNER_LITERAL_IS_VAR = (1 << 1), /**< literal is var */
  /** literal is a destructured argument binding of a possible arrow function */
  SCANNER_LITERAL_IS_ARROW_DESTRUCTURED_ARG = SCANNER_LITERAL_IS_VAR,
  SCANNER_LITERAL_IS_FUNC = (1 << 2), /**< literal is function */
  SCANNER_LITERAL_NO_REG = (1 << 3), /**< literal cannot be stored in a register */
  SCANNER_LITERAL_IS_LET = (1 << 4), /**< literal is let */
  /** literal is a function declared in this block (prevents declaring let/const with the same name) */
  SCANNER_LITERAL_IS_FUNC_DECLARATION = SCANNER_LITERAL_IS_LET,
  SCANNER_LITERAL_IS_CONST = (1 << 5), /**< literal is const */
  /** literal is a destructured argument binding */
  SCANNER_LITERAL_IS_DESTRUCTURED_ARG = SCANNER_LITERAL_IS_CONST,
  SCANNER_LITERAL_IS_USED = (1 << 6), /**< literal is used */
  SCANNER_LITERAL_EARLY_CREATE = (1 << 7), /**< binding should be created early with ECMA_VALUE_UNINITIALIZED */
} scanner_literal_type_flags_t;

/*
 * Known combinations:
 *
 *  SCANNER_LITERAL_IS_FUNC | SCANNER_LITERAL_IS_FUNC_DECLARATION :
 *         function declared in this block
 *  SCANNER_LITERAL_IS_LOCAL :
 *         module import on global scope, catch block variable otherwise
 *  SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_FUNC :
 *         a function argument which is reassigned to a function later
 *  SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_DESTRUCTURED_ARG :
 *         destructured binding argument
 *  SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_DESTRUCTURED_ARG | SCANNER_LITERAL_IS_FUNC :
 *         destructured binding argument which is reassigned to a function later
 */

/**
 * Literal is a local declaration (let, const, catch variable, etc.)
 */
#define SCANNER_LITERAL_IS_LOCAL (SCANNER_LITERAL_IS_LET | SCANNER_LITERAL_IS_CONST)

/**
 * Literal is a local function declaration
 */
#define SCANNER_LITERAL_IS_LOCAL_FUNC (SCANNER_LITERAL_IS_FUNC | SCANNER_LITERAL_IS_FUNC_DECLARATION)

/**
 * For statement descriptor.
 */
typedef struct
{
  /** shared fields of for statements */
  union
  {
    const uint8_t *source_p; /**< start source byte */
    scanner_for_info_t *for_info_p; /**< for info */
  } u;
} scanner_for_statement_t;

/**
 * Switch statement descriptor.
 */
typedef struct
{
  scanner_case_info_t **last_case_p; /**< last case info */
} scanner_switch_statement_t;

/**
 * Types of scanner destructuring bindings.
 */
typedef enum
{
  /* Update SCANNER_NEEDS_BINDING_LIST after changing these values. */
  SCANNER_BINDING_NONE, /**< not a destructuring binding expression */
  SCANNER_BINDING_VAR, /**< destructuring var binding */
  SCANNER_BINDING_LET, /**< destructuring let binding */
  SCANNER_BINDING_CATCH, /**< destructuring catch binding */
  SCANNER_BINDING_CONST, /**< destructuring const binding */
  SCANNER_BINDING_ARG, /**< destructuring arg binding */
  SCANNER_BINDING_ARROW_ARG, /**< possible destructuring arg binding of an arrow function */
} scanner_binding_type_t;

/**
 * Check whether a binding list is needed for the binding pattern.
 */
#define SCANNER_NEEDS_BINDING_LIST(type) ((type) >= SCANNER_BINDING_LET)

/**
 * Scanner binding items for destructuring binding patterns.
 */
typedef struct scanner_binding_item_t
{
  struct scanner_binding_item_t *next_p; /**< next binding in the list */
  lexer_lit_location_t *literal_p; /**< binding literal */
} scanner_binding_item_t;

/**
 * Scanner binding lists for destructuring binding patterns.
 */
typedef struct scanner_binding_list_t
{
  struct scanner_binding_list_t *prev_p; /**< prev list */
  scanner_binding_item_t *items_p; /**< list of bindings */
  bool is_nested; /**< is nested binding declaration */
} scanner_binding_list_t;

/**
 * Flags for scanner_literal_pool_t structure.
 */
typedef enum
{
  SCANNER_LITERAL_POOL_FUNCTION = (1 << 0), /**< literal pool represents a function */
  SCANNER_LITERAL_POOL_CLASS_NAME = (1 << 1), /**< literal pool which contains a class name */
  SCANNER_LITERAL_POOL_CLASS_FIELD = (1 << 2), /**< literal pool is created for a class field initializer */
  SCANNER_LITERAL_POOL_IS_STRICT = (1 << 3), /**< literal pool represents a strict mode code block */
  SCANNER_LITERAL_POOL_CAN_EVAL = (1 << 4), /**< prepare for executing eval in this block */
  SCANNER_LITERAL_POOL_NO_ARGUMENTS = (1 << 5), /**< arguments object must not be constructed,
                                                 *   or arguments cannot be stored in registers if
                                                 *   SCANNER_LITERAL_POOL_ARGUMENTS_IN_ARGS is set */
  SCANNER_LITERAL_POOL_ARGUMENTS_IN_ARGS = (1 << 6), /**< arguments is referenced in function args */
  SCANNER_LITERAL_POOL_HAS_COMPLEX_ARGUMENT = (1 << 7), /**< function has complex (ES2015+) argument definition */
  SCANNER_LITERAL_POOL_IN_WITH = (1 << 8), /**< literal pool is in a with statement */
  SCANNER_LITERAL_POOL_ARROW = (1 << 9), /**< arrow function */
  SCANNER_LITERAL_POOL_GENERATOR = (1 << 10), /**< generator function */
  SCANNER_LITERAL_POOL_ASYNC = (1 << 11), /**< async function */
  SCANNER_LITERAL_POOL_FUNCTION_STATEMENT = (1 << 12), /**< function statement */
  SCANNER_LITERAL_POOL_HAS_SUPER_REFERENCE = (1 << 13), /**< function body contains super reference */
#if JERRY_MODULE_SYSTEM
  SCANNER_LITERAL_POOL_IN_EXPORT = (1 << 14), /**< the declared variables are exported by the module system */
#endif /* JERRY_MODULE_SYSTEM */
} scanner_literal_pool_flags_t;

/**
 * Define a function where no arguments are allowed.
 */
#define SCANNER_LITERAL_POOL_ARROW_FLAGS \
  (SCANNER_LITERAL_POOL_FUNCTION | SCANNER_LITERAL_POOL_NO_ARGUMENTS | SCANNER_LITERAL_POOL_ARROW)

/**
 * This flag represents that the bracketed expression might be an async arrow function.
 * The SCANNER_LITERAL_POOL_ARROW flag is reused for this purpose.
 */
#define SCANNER_LITERAL_POOL_MAY_ASYNC_ARROW SCANNER_LITERAL_POOL_ARROW

/**
 * Getting the generator and async properties of literal pool status flags.
 */
#define SCANNER_FROM_LITERAL_POOL_TO_COMPUTED(status_flags) \
  ((uint8_t) ((((status_flags) >> 10) & 0x3) + SCAN_STACK_COMPUTED_PROPERTY))

/**
 * Setting the generator and async properties of literal pool status flags.
 */
#define SCANNER_FROM_COMPUTED_TO_LITERAL_POOL(mode) (((mode) -SCAN_STACK_COMPUTED_PROPERTY) << 10)

/**
 * Literal pool which may contains function argument identifiers
 */
#define SCANNER_LITERAL_POOL_MAY_HAVE_ARGUMENTS(status_flags) \
  (!((status_flags) & (SCANNER_LITERAL_POOL_CLASS_NAME | SCANNER_LITERAL_POOL_CLASS_FIELD)))

/**
 * Local literal pool.
 */
typedef struct scanner_literal_pool_t
{
  struct scanner_literal_pool_t *prev_p; /**< previous literal pool */
  const uint8_t *source_p; /**< source position where the final data needs to be inserted */
  parser_list_t literal_pool; /**< list of literal */
  uint16_t status_flags; /**< combination of scanner_literal_pool_flags_t flags */
  uint16_t no_declarations; /**< size of scope stack required during parsing */
} scanner_literal_pool_t;

/**
 * Scanner context.
 */
struct scanner_context_t
{
  uint32_t context_status_flags; /**< original status flags of the context */
  uint8_t mode; /**< scanner mode */
  uint8_t binding_type; /**< current destructuring binding type */
  uint16_t status_flags; /**< scanner status flags */
  scanner_binding_list_t *active_binding_list_p; /**< currently active binding list */
  scanner_literal_pool_t *active_literal_pool_p; /**< currently active literal pool */
  scanner_switch_statement_t active_switch_statement; /**< currently active switch statement */
  scanner_info_t *end_arguments_p; /**< position of end arguments */
  const uint8_t *async_source_p; /**< source position for async functions */
};

/* Scanner utils. */

void scanner_raise_error (parser_context_t *context_p);
void scanner_raise_redeclaration_error (parser_context_t *context_p);

void *scanner_malloc (parser_context_t *context_p, size_t size);
void scanner_free (void *ptr, size_t size);

size_t scanner_get_stream_size (scanner_info_t *info_p, size_t size);
scanner_info_t *scanner_insert_info (parser_context_t *context_p, const uint8_t *source_p, size_t size);
scanner_info_t *scanner_insert_info_before (parser_context_t *context_p,
                                            const uint8_t *source_p,
                                            scanner_info_t *start_info_p,
                                            size_t size);
scanner_literal_pool_t *
scanner_push_literal_pool (parser_context_t *context_p, scanner_context_t *scanner_context_p, uint16_t status_flags);
void scanner_pop_literal_pool (parser_context_t *context_p, scanner_context_t *scanner_context_p);
void scanner_filter_arguments (parser_context_t *context_p, scanner_context_t *scanner_context_p);
lexer_lit_location_t *scanner_add_custom_literal (parser_context_t *context_p,
                                                  scanner_literal_pool_t *literal_pool_p,
                                                  const lexer_lit_location_t *literal_location_p);
lexer_lit_location_t *scanner_add_literal (parser_context_t *context_p, scanner_context_t *scanner_context_p);
void scanner_add_reference (parser_context_t *context_p, scanner_context_t *scanner_context_p);
lexer_lit_location_t *scanner_append_argument (parser_context_t *context_p, scanner_context_t *scanner_context_p);
void scanner_add_private_identifier (parser_context_t *context_p, scanner_private_field_flags_t opts);
void scanner_detect_invalid_var (parser_context_t *context_p,
                                 scanner_context_t *scanner_context_p,
                                 lexer_lit_location_t *var_literal_p);
void scanner_detect_invalid_let (parser_context_t *context_p, lexer_lit_location_t *let_literal_p);
void scanner_detect_eval_call (parser_context_t *context_p, scanner_context_t *scanner_context_p);

lexer_lit_location_t *
scanner_push_class_declaration (parser_context_t *context_p, scanner_context_t *scanner_context_p, uint8_t stack_mode);
void scanner_push_class_field_initializer (parser_context_t *context_p, scanner_context_t *scanner_context_p);
void scanner_push_destructuring_pattern (parser_context_t *context_p,
                                         scanner_context_t *scanner_context_p,
                                         uint8_t binding_type,
                                         bool is_nested);
void scanner_pop_binding_list (scanner_context_t *scanner_context_p);
void scanner_append_hole (parser_context_t *context_p, scanner_context_t *scanner_context_p);

/* Scanner operations. */

void scanner_add_async_literal (parser_context_t *context_p, scanner_context_t *scanner_context_p);
void scanner_check_arrow (parser_context_t *context_p, scanner_context_t *scanner_context_p);
void
scanner_scan_simple_arrow (parser_context_t *context_p, scanner_context_t *scanner_context_p, const uint8_t *source_p);
void scanner_check_arrow_arg (parser_context_t *context_p, scanner_context_t *scanner_context_p);
bool scanner_check_async_function (parser_context_t *context_p, scanner_context_t *scanner_context_p);
void scanner_check_function_after_if (parser_context_t *context_p, scanner_context_t *scanner_context_p);
#if JERRY_MODULE_SYSTEM
void scanner_check_import_meta (parser_context_t *context_p);
#endif /* JERRY_MODULE_SYSTEM */
void scanner_scan_bracket (parser_context_t *context_p, scanner_context_t *scanner_context_p);
void scanner_check_directives (parser_context_t *context_p, scanner_context_t *scanner_context_p);

/**
 * @}
 * @}
 * @}
 */

#endif /* !JS_SCANNER_INTERNAL_H */
