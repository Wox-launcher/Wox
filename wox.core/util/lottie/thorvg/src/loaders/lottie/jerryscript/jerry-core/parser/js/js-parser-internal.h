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

#ifndef JS_PARSER_INTERNAL_H
#define JS_PARSER_INTERNAL_H

#include "ecma-module.h"

#include "byte-code.h"
#include "common.h"
#include "js-lexer.h"
#include "js-parser-limits.h"
#include "js-parser.h"
#include "js-scanner.h"
#include "parser-errors.h"

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_internals Internals
 * @{
 */

/**
 * General parser flags.
 */
typedef enum
{
  PARSER_IS_STRICT = (1u << 0), /**< strict mode code */
  PARSER_IS_FUNCTION = (1u << 1), /**< function body is parsed */
  PARSER_IS_CLOSURE = (1u << 2), /**< function body is encapsulated in {} block */
  PARSER_IS_FUNC_EXPRESSION = (1u << 3), /**< a function expression is parsed */
  PARSER_IS_PROPERTY_GETTER = (1u << 4), /**< a property getter function is parsed */
  PARSER_IS_PROPERTY_SETTER = (1u << 5), /**< a property setter function is parsed */
  PARSER_HAS_NON_STRICT_ARG = (1u << 6), /**< the function has arguments which
                                          *   are not supported in strict mode */
  PARSER_ARGUMENTS_NEEDED = (1u << 7), /**< arguments object must be created */
  PARSER_LEXICAL_ENV_NEEDED = (1u << 8), /**< lexical environment object must be created */
  PARSER_INSIDE_WITH = (1u << 9), /**< code block is inside a with statement */
  PARSER_NO_END_LABEL = (1u << 10), /**< return instruction must be inserted
                                     *   after the last byte code */
  PARSER_DEBUGGER_BREAKPOINT_APPENDED = (1u << 11), /**< pending (unsent) breakpoint
                                                     *   info is available */
  PARSER_LEXICAL_BLOCK_NEEDED = (1u << 12), /**< global script: needs a lexical environment for let and const
                                             *   function: needs a lexical environment for arguments */
  PARSER_IS_ARROW_FUNCTION = (1u << 13), /**< an arrow function is parsed */
  PARSER_IS_GENERATOR_FUNCTION = (1u << 14), /**< a generator function is parsed */
  PARSER_IS_ASYNC_FUNCTION = (1u << 15), /**< an async function is parsed */
  PARSER_DISALLOW_AWAIT_YIELD = (1u << 16), /**< throw SyntaxError for await / yield keywords */
  PARSER_FUNCTION_HAS_COMPLEX_ARGUMENT = (1u << 17), /**< function has complex (ES2015+) argument definition */
  PARSER_FUNCTION_HAS_REST_PARAM = (1u << 18), /**< function has rest parameter */
  PARSER_CLASS_CONSTRUCTOR = (1u << 19), /**< a class constructor is parsed
                                          *   Note: PARSER_ALLOW_SUPER must be present */
  /* These five status flags must be in this order. See PARSER_SAVED_FLAGS_OFFSET. */
  PARSER_ALLOW_SUPER = (1u << 20), /**< allow super property access */
  PARSER_ALLOW_SUPER_CALL = (1u << 21), /**< allow super constructor call
                                         *   Note: PARSER_CLASS_CONSTRUCTOR must be present */
  PARSER_FUNCTION_IS_PARSING_ARGS = (1u << 22), /**< set when parsing function arguments */
  PARSER_INSIDE_CLASS_FIELD = (1u << 23), /**< a class field is being parsed */
  PARSER_ALLOW_NEW_TARGET = (1u << 24), /**< allow new.target parsing in the current context */
  PARSER_IS_METHOD = (1u << 25), /**< method is parsed */
  PARSER_IS_CLASS_STATIC_BLOCK = (1u << 26), /**< a class static block is parsed */
  PARSER_PRIVATE_FUNCTION_NAME = PARSER_IS_FUNC_EXPRESSION, /**< represents private method for
                                                             *   parser_set_function_name*/
#if JERRY_MODULE_SYSTEM
  PARSER_MODULE_DEFAULT_CLASS_OR_FUNC = (1u << 27), /**< parsing a function or class default export */
  PARSER_MODULE_STORE_IDENT = (1u << 28), /**< store identifier of the current export statement */
#endif /* JERRY_MODULE_SYSTEM */
  PARSER_HAS_LATE_LIT_INIT = (1u << 30), /**< there are identifier or string literals which construction
                                          *   is postponed after the local parser data is freed */
#ifndef JERRY_NDEBUG
  PARSER_SCANNING_SUCCESSFUL = PARSER_HAS_LATE_LIT_INIT, /**< scanning process was successful */
#endif /* !JERRY_NDEBUG */
} parser_general_flags_t;

/**
 * Expression parsing flags.
 */
typedef enum
{
  PARSE_EXPR = 0, /**< parse an expression without any special flags */
  PARSE_EXPR_LEFT_HAND_SIDE = (1u << 0), /**< parse a left-hand-side expression */
  PARSE_EXPR_NO_PUSH_RESULT = (1u << 1), /**< do not push the result of the expression onto the stack */
  PARSE_EXPR_NO_COMMA = (1u << 2), /**< do not parse comma operator */
  PARSE_EXPR_HAS_LITERAL = (1u << 3), /**< a primary literal is provided by a
                                       *   CBC_PUSH_LITERAL instruction  */
} parser_expression_flags_t;

/**
 * Pattern parsing flags.
 */
typedef enum
{
  PARSER_PATTERN_NO_OPTS = 0, /**< parse the expression after '=' */
  PARSER_PATTERN_BINDING = (1u << 0), /**< parse BindingPattern */
  PARSER_PATTERN_TARGET_ON_STACK = (1u << 1), /**< assignment target is the topmost element on the stack */
  PARSER_PATTERN_TARGET_DEFAULT = (1u << 2), /**< perform default value comparison for assignment target */
  PARSER_PATTERN_NESTED_PATTERN = (1u << 3), /**< parse pattern inside a pattern */
  PARSER_PATTERN_LET = (1u << 4), /**< pattern is a let declaration */
  PARSER_PATTERN_CONST = (1u << 5), /**< pattern is a const declaration */
  PARSER_PATTERN_LOCAL = (1u << 6), /**< pattern is a local (catch parameter) declaration */
  PARSER_PATTERN_REST_ELEMENT = (1u << 7), /**< parse rest array / object element */
  PARSER_PATTERN_HAS_REST_ELEMENT = (1u << 8), /**< object literal rest element will be present */
  PARSER_PATTERN_ARGUMENTS = (1u << 9), /**< parse arguments binding */
} parser_pattern_flags_t;

/**
 * Check type for scanner_is_context_needed function.
 */
typedef enum
{
  PARSER_CHECK_BLOCK_CONTEXT, /**< check block context */
  PARSER_CHECK_GLOBAL_CONTEXT, /**< check global context */
  PARSER_CHECK_FUNCTION_CONTEXT, /**< check function context */
} parser_check_context_type_t;

/**
 * Class field bits.
 */
typedef enum
{
  PARSER_CLASS_FIELD_END = (1u << 0), /**< last class field */
  PARSER_CLASS_FIELD_NORMAL = (1u << 1), /**< normal (non-computed) class field */
  PARSER_CLASS_FIELD_INITIALIZED = (1u << 2), /**< class field is initialized */
  PARSER_CLASS_FIELD_STATIC = (1u << 3), /**< static class field */
  PARSER_CLASS_FIELD_STATIC_BLOCK = (1u << 4), /**< static class field */
} parser_class_field_type_t;

/**
 * Mask for strict mode code
 */
#define PARSER_STRICT_MODE_MASK 0x1

/**
 * Shorthand for function closure definition
 */
#define PARSER_FUNCTION_CLOSURE (PARSER_IS_FUNCTION | PARSER_IS_CLOSURE)

#if PARSER_MAXIMUM_CODE_SIZE <= UINT16_MAX
/**
 * Maximum number of bytes for branch target.
 */
#define PARSER_MAX_BRANCH_LENGTH 2
#else /* PARSER_MAXIMUM_CODE_SIZE > UINT16_MAX */
/**
 * Maximum number of bytes for branch target.
 */
#define PARSER_MAX_BRANCH_LENGTH 3
#endif /* PARSER_MAXIMUM_CODE_SIZE <= UINT16_MAX */

/**
 * Offset of PARSER_ALLOW_SUPER
 */
#define PARSER_SAVED_FLAGS_OFFSET JERRY_LOG2 (PARSER_ALLOW_SUPER)

/**
 * Mask of saved flags
 */
#define PARSER_SAVED_FLAGS_MASK \
  ((1 << (JERRY_LOG2 (PARSER_ALLOW_NEW_TARGET) - JERRY_LOG2 (PARSER_ALLOW_SUPER) + 1)) - 1)

/**
 * Get class option bits from parser_general_flags_t
 */
#define PARSER_SAVE_STATUS_FLAGS(opts) ((uint16_t) (((opts) >> PARSER_SAVED_FLAGS_OFFSET) & PARSER_SAVED_FLAGS_MASK))

/**
 * Mask for get class option bits from ecma_parse_opts_t
 */
#define PARSER_RESTORE_STATUS_FLAGS_MASK (((ECMA_PARSE_ALLOW_NEW_TARGET << 1) - 1) - (ECMA_PARSE_ALLOW_SUPER - 1))

/**
 * Shift for get class option bits from ecma_parse_opts_t
 */
#define PARSER_RESTORE_STATUS_FLAGS_SHIFT (JERRY_LOG2 (PARSER_ALLOW_SUPER) - JERRY_LOG2 (ECMA_PARSE_ALLOW_SUPER))

/**
 * Get class option bits from ecma_parse_opts_t
 */
#define PARSER_RESTORE_STATUS_FLAGS(opts) \
  (((opts) &PARSER_RESTORE_STATUS_FLAGS_MASK) << PARSER_RESTORE_STATUS_FLAGS_SHIFT)

/**
 * All flags that affect exotic arguments object creation.
 */
#define PARSER_ARGUMENTS_RELATED_FLAGS \
  (PARSER_ARGUMENTS_NEEDED | PARSER_FUNCTION_HAS_COMPLEX_ARGUMENT | PARSER_IS_STRICT)

/**
 * Get the corresponding eval flag for a ecma_parse_opts_t flag
 */
#define PARSER_GET_EVAL_FLAG(type) ((type) >> JERRY_LOG2 (ECMA_PARSE_ALLOW_SUPER))

/**
 * Check non-generator async functions
 */
#define PARSER_IS_NORMAL_ASYNC_FUNCTION(status_flags) \
  (((status_flags) & (PARSER_IS_GENERATOR_FUNCTION | PARSER_IS_ASYNC_FUNCTION)) == PARSER_IS_ASYNC_FUNCTION)

/* Checks whether unmapped arguments are needed. */
#define PARSER_NEEDS_MAPPED_ARGUMENTS(status_flags) \
  (((status_flags) &PARSER_ARGUMENTS_RELATED_FLAGS) == PARSER_ARGUMENTS_NEEDED)

/* The maximum of PARSER_CBC_STREAM_PAGE_SIZE is 127. */
#define PARSER_CBC_STREAM_PAGE_SIZE ((uint32_t) (64 - sizeof (void *)))

/* Defines the size of the max page. */
#define PARSER_STACK_PAGE_SIZE ((uint32_t) (((sizeof (void *) > 4) ? 128 : 64) - sizeof (void *)))

/* Avoid compiler warnings for += operations. */
#define PARSER_PLUS_EQUAL_U16(base, value)  (base) = (uint16_t) ((base) + (value))
#define PARSER_MINUS_EQUAL_U16(base, value) (base) = (uint16_t) ((base) - (value))
#define PARSER_PLUS_EQUAL_LC(base, value)   (base) = (parser_line_counter_t) ((base) + (value))

/**
 * Argument for a compact-byte code.
 */
typedef struct
{
  uint16_t literal_index; /**< literal index argument */
  uint16_t value; /**< other argument (second literal or byte). */
  uint16_t third_literal_index; /**< literal index argument */
  uint8_t literal_type; /**< last literal type */
  uint8_t literal_keyword_type; /**< last literal keyword type */
} cbc_argument_t;

/* Useful parser macros. */

#define PARSER_CBC_UNAVAILABLE CBC_EXT_OPCODE

#define PARSER_TO_EXT_OPCODE(opcode)   ((uint16_t) ((opcode) + 256))
#define PARSER_GET_EXT_OPCODE(opcode)  ((opcode) -256)
#define PARSER_IS_BASIC_OPCODE(opcode) ((opcode) < 256)
#define PARSER_IS_PUSH_LITERAL(opcode) \
  ((opcode) == CBC_PUSH_LITERAL || (opcode) == CBC_PUSH_TWO_LITERALS || (opcode) == CBC_PUSH_THREE_LITERALS)
#define PARSER_IS_PUSH_NUMBER(opcode) \
  ((opcode) >= CBC_PUSH_NUMBER_0 && (opcode) <= CBC_PUSH_LITERAL_PUSH_NUMBER_NEG_BYTE)

#define PARSER_IS_MUTABLE_PUSH_LITERAL(opcode) ((opcode) >= CBC_PUSH_LITERAL && (opcode) <= CBC_PUSH_THIS_LITERAL)

#define PARSER_IS_PUSH_LITERALS_WITH_THIS(opcode) ((opcode) >= CBC_PUSH_LITERAL && (opcode) <= CBC_PUSH_THREE_LITERALS)

#define PARSER_IS_PUSH_PROP(opcode) ((opcode) >= CBC_PUSH_PROP && (opcode) <= CBC_PUSH_PROP_THIS_LITERAL)

#define PARSER_IS_PUSH_PROP_LITERAL(opcode) \
  ((opcode) >= CBC_PUSH_PROP_LITERAL && (opcode) <= CBC_PUSH_PROP_THIS_LITERAL)

#define PARSER_PUSH_LITERAL_TO_PUSH_PROP_LITERAL(opcode) \
  (uint16_t) ((opcode) + (CBC_PUSH_PROP_LITERAL - CBC_PUSH_LITERAL))

#define PARSER_PUSH_PROP_LITERAL_TO_PUSH_LITERAL(opcode) \
  (uint16_t) ((opcode) - (CBC_PUSH_PROP_LITERAL - CBC_PUSH_LITERAL))

#define PARSER_PUSH_PROP_TO_PUSH_PROP_REFERENCE(opcode) \
  (uint16_t) ((opcode) + (CBC_PUSH_PROP_REFERENCE - CBC_PUSH_PROP))

#define PARSER_PUSH_PROP_REFERENCE_TO_PUSH_PROP(opcode) \
  (uint16_t) ((opcode) - (CBC_PUSH_PROP_REFERENCE - CBC_PUSH_PROP))

#define PARSER_GET_LITERAL(literal_index) \
  ((lexer_literal_t *) parser_list_get (&context_p->literal_pool, (literal_index)))

#define PARSER_TO_BINARY_OPERATION_WITH_RESULT(opcode) \
  (PARSER_TO_EXT_OPCODE (opcode) - CBC_ASSIGN_ADD + CBC_EXT_ASSIGN_ADD_PUSH_RESULT)

#define PARSER_TO_BINARY_OPERATION_WITH_BLOCK(opcode) \
  ((uint16_t) (PARSER_TO_EXT_OPCODE (opcode) - CBC_ASSIGN_ADD + CBC_EXT_ASSIGN_ADD_BLOCK))

#define PARSER_GET_FLAGS(op) (PARSER_IS_BASIC_OPCODE (op) ? cbc_flags[(op)] : cbc_ext_flags[PARSER_GET_EXT_OPCODE (op)])

#define PARSER_OPCODE_IS_RETURN(op) \
  ((op) == CBC_RETURN || (op) == CBC_RETURN_FUNCTION_END || (op) == CBC_RETURN_WITH_LITERAL)

#define PARSER_ARGS_EQ(op, types) ((PARSER_GET_FLAGS (op) & CBC_ARG_TYPES) == (types))

/**
 * All data allocated by the parser is
 * stored in parser_data_pages in the memory.
 */
#if defined(_MSC_VER )
#pragma warning(push)
#pragma warning(disable:4200)
#endif
typedef struct parser_mem_page_t
{
  struct parser_mem_page_t *next_p; /**< next page */
  uint8_t bytes[]; /**< memory bytes, C99 flexible array member */
} parser_mem_page_t;
#if defined(_MSC_VER)
#pragma warning(pop)
#endif

/**
 * Structure for managing parser memory.
 */
typedef struct
{
  parser_mem_page_t *first_p; /**< first allocated page */
  parser_mem_page_t *last_p; /**< last allocated page */
  uint32_t last_position; /**< position of the last allocated byte */
} parser_mem_data_t;

/**
 * Parser memory list.
 */
typedef struct
{
  parser_mem_data_t data; /**< storage space */
  uint32_t page_size; /**< size of each page */
  uint32_t item_size; /**< size of each item */
  uint32_t item_count; /**< number of items on each page */
} parser_list_t;

/**
 * Iterator for parser memory list.
 */
typedef struct
{
  parser_list_t *list_p; /**< parser list */
  parser_mem_page_t *current_p; /**< currently processed page */
  size_t current_position; /**< current position on the page */
} parser_list_iterator_t;

/**
 * Parser memory stack.
 */
typedef struct
{
  parser_mem_data_t data; /**< storage space */
  parser_mem_page_t *free_page_p; /**< space for fast allocation */
} parser_stack_t;

/**
 * Iterator for parser memory stack.
 */
typedef struct
{
  parser_mem_page_t *current_p; /**< currently processed page */
  size_t current_position; /**< current position on the page */
} parser_stack_iterator_t;

/**
 * Branch type.
 */
typedef struct
{
  parser_mem_page_t *page_p; /**< branch location page */
  uint32_t offset; /**< branch location offset */
} parser_branch_t;

/**
 * Branch chain type.
 */
typedef struct parser_branch_node_t
{
  struct parser_branch_node_t *next_p; /**< next linked list node */
  parser_branch_t branch; /**< branch */
} parser_branch_node_t;

/**
 * Items of scope stack.
 */
typedef struct
{
  uint16_t map_from; /**< original literal index */
  uint16_t map_to; /**< encoded register or literal index and flags */
} parser_scope_stack_t;

/**
 * This item represents a function literal in the scope stack.
 *
 * When map_from == PARSER_SCOPE_STACK_FUNC:
 *   map_to represents the literal reserved for a function literal
 *   Note: the name of the function is the previous value in the scope stack
 *   Note: map_to is not encoded in this case
 */
#define PARSER_SCOPE_STACK_FUNC 0xffff

/**
 * Mask for decoding the register index of map_to
 */
#define PARSER_SCOPE_STACK_REGISTER_MASK 0x3fff

/**
 * Function statements with the name specified
 * in map_from should not be copied to global scope.
 */
#define PARSER_SCOPE_STACK_NO_FUNCTION_COPY 0x8000

/**
 * The scope stack item represents a const binding stored in register
 */
#define PARSER_SCOPE_STACK_IS_CONST_REG 0x4000

/**
 * The scope stack item represents a binding which has already created with ECMA_VALUE_UNINITIALIZED
 */
#define PARSER_SCOPE_STACK_IS_LOCAL_CREATED (PARSER_SCOPE_STACK_IS_CONST_REG)

/**
 * Starting literal index for registers.
 */
#define PARSER_REGISTER_START 0x8000

/**
 * Invalid literal index
 */
#define PARSER_INVALID_LITERAL_INDEX UINT16_MAX

/**
 * Lastly emitted opcode is not a function literal
 */
#define PARSER_NOT_FUNCTION_LITERAL PARSER_INVALID_LITERAL_INDEX

/**
 * Lastly emitted opcode is not a named function literal
 */
#define PARSER_NAMED_FUNCTION (uint16_t) (PARSER_NOT_FUNCTION_LITERAL - 1)

/**
 * Lastly emitted opcode is not an anonymous class literal
 */
#define PARSER_ANONYMOUS_CLASS (uint16_t) (PARSER_NAMED_FUNCTION - 1)

/* Forward definitions for js-scanner-internal.h. */
struct scanner_context_t;
typedef struct scanner_context_t scanner_context_t;

/**
 * List of private field contexts
 */
typedef struct parser_private_context_t
{
  scanner_class_private_member_t *members_p; /**< current private field context members */
  struct parser_private_context_t *prev_p; /**< previous private field context */
  uint8_t opts; /**< options */
} parser_private_context_t;

/**
 * Those members of a context which needs
 * to be saved when a sub-function is parsed.
 */
typedef struct parser_saved_context_t
{
  /* Parser members. */
  uint32_t status_flags; /**< parsing options */
  uint16_t stack_depth; /**< current stack depth */
  uint16_t stack_limit; /**< maximum stack depth */
  struct parser_saved_context_t *prev_context_p; /**< last saved context */
  parser_stack_iterator_t last_statement; /**< last statement position */

  /* Literal types */
  uint16_t argument_count; /**< number of function arguments */
  uint16_t argument_length; /**< length property of arguments */
  uint16_t register_count; /**< number of registers */
  uint16_t literal_count; /**< number of literals */

  /* Memory storage members. */
  parser_mem_data_t byte_code; /**< byte code buffer */
  uint32_t byte_code_size; /**< byte code size for branches */
  parser_mem_data_t literal_pool_data; /**< literal list */
  parser_scope_stack_t *scope_stack_p; /**< scope stack */
  uint16_t scope_stack_size; /**< size of scope stack */
  uint16_t scope_stack_top; /**< preserved top of scope stack */
  uint16_t scope_stack_reg_top; /**< preserved top register of scope stack */
  uint16_t scope_stack_global_end; /**< end of global declarations of a function */
  ecma_value_t tagged_template_literal_cp; /**< compressed pointer to the tagged template literal collection */
#ifndef JERRY_NDEBUG
  uint16_t context_stack_depth; /**< current context stack depth */
#endif /* !JERRY_NDEBUG */
} parser_saved_context_t;

/**
 * Shared parser context.
 */
typedef struct
{
  PARSER_TRY_CONTEXT (try_buffer); /**< try_buffer */
  parser_error_msg_t error; /**< error code */
  /** Union for rarely used members. */
  union
  {
    void *allocated_buffer_p; /**< dynamically allocated buffer
                               *   which needs to be freed on error */
    scanner_context_t *scanner_context_p; /**< scanner context for the pre-scanner */
  } u;
  uint32_t allocated_buffer_size; /**< size of the dynamically allocated buffer */

  /* Parser members. */
  uint32_t status_flags; /**< status flags */
  uint32_t global_status_flags; /**< global status flags */
  uint16_t stack_depth; /**< current stack depth */
  uint16_t stack_limit; /**< maximum stack depth */
  const jerry_parse_options_t *options_p; /**< parse options */
  parser_saved_context_t *last_context_p; /**< last saved context */
  parser_stack_iterator_t last_statement; /**< last statement position */
  cbc_script_t *script_p; /**< current script */
  const uint8_t *source_start_p; /**< source start */
  lit_utf8_size_t source_size; /**< source size */
  const uint8_t *arguments_start_p; /**< function argument list start */
  lit_utf8_size_t arguments_size; /**< function argument list size */
  ecma_value_t script_value; /**< current script as value */
  ecma_value_t argument_list; /**< current argument list as value */
  ecma_value_t user_value; /**< current user value */

#if JERRY_MODULE_SYSTEM
  ecma_module_names_t *module_names_p; /**< import / export names that is being processed */
  lexer_literal_t *module_identifier_lit_p; /**< the literal for the identifier of the current element */
#endif /* JERRY_MODULE_SYSTEM */

  /* Lexer members. */
  lexer_token_t token; /**< current token */
  lexer_lit_object_t lit_object; /**< current literal object */
  const uint8_t *source_p; /**< next source byte */
  const uint8_t *source_end_p; /**< last source byte */
  parser_line_counter_t line; /**< current line */
  parser_line_counter_t column; /**< current column */

  /* Scanner members. */
  scanner_info_t *next_scanner_info_p; /**< next scanner info block */
  scanner_info_t *active_scanner_info_p; /**< currently active scanner info block */
  scanner_info_t *skipped_scanner_info_p; /**< next scanner info block */
  scanner_info_t *skipped_scanner_info_end_p; /**< currently active scanner info block */

  /* Compact byte code members. */
  cbc_argument_t last_cbc; /**< argument of the last cbc */
  uint16_t last_cbc_opcode; /**< opcode of the last cbc */

  /* Literal types */
  uint16_t argument_count; /**< number of function arguments */
  uint16_t argument_length; /**< length property of arguments */
  uint16_t register_count; /**< number of registers */
  uint16_t literal_count; /**< number of literals */

  /* Memory storage members. */
  parser_mem_data_t byte_code; /**< byte code buffer */
  uint32_t byte_code_size; /**< current byte code size for branches */
  parser_list_t literal_pool; /**< literal list */
  parser_mem_data_t stack; /**< storage space */
  parser_scope_stack_t *scope_stack_p; /**< scope stack */
  parser_mem_page_t *free_page_p; /**< space for fast allocation */
  uint16_t scope_stack_size; /**< size of scope stack */
  uint16_t scope_stack_top; /**< current top of scope stack */
  uint16_t scope_stack_reg_top; /**< current top register of scope stack */
  uint16_t scope_stack_global_end; /**< end of global declarations of a function */
  ecma_value_t tagged_template_literal_cp; /**< compressed pointer to the tagged template literal collection */
  parser_private_context_t *private_context_p; /**< private context */
  uint8_t stack_top_uint8; /**< top byte stored on the stack */

#ifndef JERRY_NDEBUG
  /* Variables for debugging / logging. */
  uint16_t context_stack_depth; /**< current context stack depth */
#endif /* !JERRY_NDEBUG */

#if JERRY_PARSER_DUMP_BYTE_CODE
  int is_show_opcodes; /**< show opcodes */
  uint32_t total_byte_code_size; /**< total byte code size */
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

} parser_context_t;

/**
 * @}
 * @}
 * @}
 *
 * \addtogroup mem Memory allocation
 * @{
 *
 * \addtogroup mem_parser Parser memory manager
 * @{
 */

/* Memory management.
 * Note: throws an error if unsuccessful. */
void *parser_malloc (parser_context_t *context_p, size_t size);
void parser_free (void *ptr, size_t size);
void *parser_malloc_local (parser_context_t *context_p, size_t size);
void parser_free_local (void *ptr, size_t size);
void parser_free_allocated_buffer (parser_context_t *context_p);

/* Parser byte stream. */

void parser_cbc_stream_init (parser_mem_data_t *data_p);
void parser_cbc_stream_free (parser_mem_data_t *data_p);
void parser_cbc_stream_alloc_page (parser_context_t *context_p, parser_mem_data_t *data_p);

/* Parser list. Ensures pointer alignment. */

void parser_list_init (parser_list_t *list_p, uint32_t item_size, uint32_t item_count);
void parser_list_free (parser_list_t *list_p);
void parser_list_reset (parser_list_t *list_p);
void *parser_list_append (parser_context_t *context_p, parser_list_t *list_p);
void *parser_list_get (parser_list_t *list_p, size_t index);
void parser_list_iterator_init (parser_list_t *list_p, parser_list_iterator_t *iterator_p);
void *parser_list_iterator_next (parser_list_iterator_t *iterator_p);

/* Parser stack. Optimized for pushing bytes.
 * Pop functions never throws error. */

void parser_stack_init (parser_context_t *context_p);
void parser_stack_free (parser_context_t *context_p);
void parser_stack_push_uint8 (parser_context_t *context_p, uint8_t uint8_value);
void parser_stack_pop_uint8 (parser_context_t *context_p);
void parser_stack_change_last_uint8 (parser_context_t *context_p, uint8_t new_value);
uint8_t *parser_stack_get_prev_uint8 (parser_context_t *context_p);
void parser_stack_push_uint16 (parser_context_t *context_p, uint16_t uint16_value);
uint16_t parser_stack_pop_uint16 (parser_context_t *context_p);
void parser_stack_push (parser_context_t *context_p, const void *data_p, uint32_t length);
void parser_stack_pop (parser_context_t *context_p, void *data_p, uint32_t length);
void parser_stack_iterator_init (parser_context_t *context_p, parser_stack_iterator_t *iterator);
uint8_t parser_stack_iterator_read_uint8 (parser_stack_iterator_t *iterator);
void parser_stack_iterator_skip (parser_stack_iterator_t *iterator, size_t length);
void parser_stack_iterator_read (parser_stack_iterator_t *iterator, void *data_p, size_t length);
void parser_stack_iterator_write (parser_stack_iterator_t *iterator, const void *data_p, size_t length);

/**
 * @}
 * @}
 *
 * \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_utils Utility
 * @{
 */

/* Compact byte code emitting functions. */

void parser_flush_cbc (parser_context_t *context_p);
void parser_emit_cbc (parser_context_t *context_p, uint16_t opcode);
void parser_emit_cbc_literal (parser_context_t *context_p, uint16_t opcode, uint16_t literal_index);
void
parser_emit_cbc_literal_value (parser_context_t *context_p, uint16_t opcode, uint16_t literal_index, uint16_t value);
void parser_emit_cbc_literal_from_token (parser_context_t *context_p, uint16_t opcode);
void parser_emit_cbc_call (parser_context_t *context_p, uint16_t opcode, size_t call_arguments);
void parser_emit_cbc_push_number (parser_context_t *context_p, bool is_negative_number);
void parser_emit_cbc_forward_branch (parser_context_t *context_p, uint16_t opcode, parser_branch_t *branch_p);
parser_branch_node_t *
parser_emit_cbc_forward_branch_item (parser_context_t *context_p, uint16_t opcode, parser_branch_node_t *next_p);
void parser_emit_cbc_backward_branch (parser_context_t *context_p, uint16_t opcode, uint32_t offset);
ecma_string_t *parser_new_ecma_string_from_literal (lexer_literal_t *literal_p);
void parser_set_branch_to_current_position (parser_context_t *context_p, parser_branch_t *branch_p);
void parser_set_breaks_to_current_position (parser_context_t *context_p, parser_branch_node_t *current_p);
void parser_set_continues_to_current_position (parser_context_t *context_p, parser_branch_node_t *current_p);
void parser_reverse_class_fields (parser_context_t *context_p, size_t fields_size);

/* Convenience macros. */
#define parser_emit_cbc_ext(context_p, opcode) parser_emit_cbc ((context_p), PARSER_TO_EXT_OPCODE (opcode))
#define parser_emit_cbc_ext_literal(context_p, opcode, literal_index) \
  parser_emit_cbc_literal ((context_p), PARSER_TO_EXT_OPCODE (opcode), (literal_index))
#define parser_emit_cbc_ext_literal_from_token(context_p, opcode) \
  parser_emit_cbc_literal_from_token ((context_p), PARSER_TO_EXT_OPCODE (opcode))
#define parser_emit_cbc_ext_call(context_p, opcode, call_arguments) \
  parser_emit_cbc_call ((context_p), PARSER_TO_EXT_OPCODE (opcode), (call_arguments))
#define parser_emit_cbc_ext_call(context_p, opcode, call_arguments) \
  parser_emit_cbc_call ((context_p), PARSER_TO_EXT_OPCODE (opcode), (call_arguments))
#define parser_emit_cbc_ext_forward_branch(context_p, opcode, branch_p) \
  parser_emit_cbc_forward_branch ((context_p), PARSER_TO_EXT_OPCODE (opcode), (branch_p))
#define parser_emit_cbc_ext_backward_branch(context_p, opcode, offset) \
  parser_emit_cbc_backward_branch ((context_p), PARSER_TO_EXT_OPCODE (opcode), (offset))

/**
 * @}
 *
 * \addtogroup jsparser_lexer Lexer
 * @{
 */

/* Lexer functions */

void lexer_next_token (parser_context_t *context_p);
bool lexer_check_next_character (parser_context_t *context_p, lit_utf8_byte_t character);
bool lexer_check_next_characters (parser_context_t *context_p, lit_utf8_byte_t character1, lit_utf8_byte_t character2);
uint8_t lexer_consume_next_character (parser_context_t *context_p);
bool lexer_check_post_primary_exp (parser_context_t *context_p);
void lexer_skip_empty_statements (parser_context_t *context_p);
bool lexer_check_arrow (parser_context_t *context_p);
bool lexer_check_arrow_param (parser_context_t *context_p);
bool lexer_check_yield_no_arg (parser_context_t *context_p);
bool lexer_consume_generator (parser_context_t *context_p);
bool lexer_consume_assign (parser_context_t *context_p);
void lexer_update_await_yield (parser_context_t *context_p, uint32_t status_flags);
bool lexer_scan_private_identifier (parser_context_t *context_p);
void lexer_parse_string (parser_context_t *context_p, lexer_string_options_t opts);
void lexer_expect_identifier (parser_context_t *context_p, uint8_t literal_type);
bool lexer_scan_identifier (parser_context_t *context_p, lexer_parse_options_t opts);
void lexer_check_property_modifier (parser_context_t *context_p);
void lexer_convert_ident_to_cesu8 (uint8_t *destination_p, const uint8_t *source_p, prop_length_t length);

const uint8_t *lexer_convert_literal_to_chars (parser_context_t *context_p,
                                               const lexer_lit_location_t *literal_p,
                                               uint8_t *local_byte_array_p,
                                               lexer_string_options_t opts);
void lexer_expect_object_literal_id (parser_context_t *context_p, uint32_t ident_opts);
lexer_literal_t *lexer_construct_unused_literal (parser_context_t *context_p);
void lexer_construct_literal_object (parser_context_t *context_p,
                                     const lexer_lit_location_t *lit_location_p,
                                     uint8_t literal_type);
bool lexer_construct_number_object (parser_context_t *context_p, bool is_expr, bool is_negative_number);
void lexer_convert_push_number_to_push_literal (parser_context_t *context_p);
uint16_t lexer_construct_function_object (parser_context_t *context_p, uint32_t extra_status_flags);
uint16_t lexer_construct_class_static_block_function (parser_context_t *context_p);
void lexer_construct_regexp_object (parser_context_t *context_p, bool parse_only);
bool lexer_compare_identifier_to_string (const lexer_lit_location_t *left_p, const uint8_t *right_p, size_t size);
bool lexer_compare_identifiers (parser_context_t *context_p,
                                const lexer_lit_location_t *left_p,
                                const lexer_lit_location_t *right_p);
bool lexer_current_is_literal (parser_context_t *context_p, const lexer_lit_location_t *right_ident_p);
bool lexer_string_is_use_strict (parser_context_t *context_p);
bool lexer_string_is_directive (parser_context_t *context_p);
bool lexer_token_is_identifier (parser_context_t *context_p, const char *identifier_p, size_t identifier_length);
bool lexer_token_is_let (parser_context_t *context_p);
bool lexer_token_is_async (parser_context_t *context_p);
bool lexer_compare_literal_to_string (parser_context_t *context_p, const char *string_p, size_t string_length);
void lexer_init_line_info (parser_context_t *context_p);
uint8_t lexer_convert_binary_lvalue_token_to_binary (uint8_t token);

/**
 * @}
 *
 * \addtogroup jsparser_expr Expression parser
 * @{
 */

/* Parser functions. */

void parser_parse_block_expression (parser_context_t *context_p, int options);
void parser_parse_expression_statement (parser_context_t *context_p, int options);
void parser_parse_expression (parser_context_t *context_p, int options);
void parser_resolve_private_identifier (parser_context_t *context_p);
void parser_save_private_context (parser_context_t *context_p,
                                  parser_private_context_t *private_ctx_p,
                                  scanner_class_info_t *class_info_p);
void parser_restore_private_context (parser_context_t *context_p, parser_private_context_t *private_ctx_p);
void parser_parse_class (parser_context_t *context_p, bool is_statement);
void parser_parse_initializer (parser_context_t *context_p, parser_pattern_flags_t flags);
void parser_parse_initializer_by_next_char (parser_context_t *context_p, parser_pattern_flags_t flags);
/**
 * @}
 *
 * \addtogroup jsparser_scanner Scanner
 * @{
 */

void scanner_release_next (parser_context_t *context_p, size_t size);
void scanner_set_active (parser_context_t *context_p);
void scanner_revert_active (parser_context_t *context_p);
void scanner_release_active (parser_context_t *context_p, size_t size);
void scanner_release_switch_cases (scanner_case_info_t *case_p);
void scanner_release_private_fields (scanner_class_private_member_t *member_p);
void scanner_seek (parser_context_t *context_p);
void scanner_reverse_info_list (parser_context_t *context_p);
void scanner_cleanup (parser_context_t *context_p);

bool scanner_is_context_needed (parser_context_t *context_p, parser_check_context_type_t check_type);
bool scanner_try_scan_new_target (parser_context_t *context_p);
void scanner_check_variables (parser_context_t *context_p);
void scanner_create_variables (parser_context_t *context_p, uint32_t option_flags);

void scanner_get_location (scanner_location_t *location_p, parser_context_t *context_p);
void scanner_set_location (parser_context_t *context_p, scanner_location_t *location_p);
uint16_t scanner_decode_map_to (parser_scope_stack_t *stack_item_p);
uint16_t scanner_save_literal (parser_context_t *context_p, uint16_t ident_index);
bool scanner_literal_is_const_reg (parser_context_t *context_p, uint16_t literal_index);
bool scanner_literal_is_created (parser_context_t *context_p, uint16_t literal_index);
bool scanner_literal_exists (parser_context_t *context_p, uint16_t literal_index);

void scanner_scan_all (parser_context_t *context_p);

/**
 * @}
 *
 * \addtogroup jsparser_stmt Statement parser
 * @{
 */

void parser_parse_statements (parser_context_t *context_p);
void parser_free_jumps (parser_stack_iterator_t iterator);

#if JERRY_MODULE_SYSTEM
/**
 * @}
 *
 * \addtogroup jsparser_stmt Module statement parser
 * @{
 */

extern const lexer_lit_location_t lexer_default_literal;
void parser_module_check_request_place (parser_context_t *context_p);
void parser_module_context_init (parser_context_t *context_p);
void parser_module_append_names (parser_context_t *context_p, ecma_module_names_t **module_names_p);
void parser_module_handle_module_specifier (parser_context_t *context_p, ecma_module_node_t **node_list_p);
void parser_module_handle_requests (parser_context_t *context_p);
void parser_module_parse_export_clause (parser_context_t *context_p);
void parser_module_parse_import_clause (parser_context_t *context_p);
void parser_module_set_default (parser_context_t *context_p);
bool parser_module_check_duplicate_import (parser_context_t *context_p, ecma_string_t *local_name_p);
bool parser_module_check_duplicate_export (parser_context_t *context_p, ecma_string_t *export_name_p);
void parser_module_append_export_name (parser_context_t *context_p);
void
parser_module_add_names_to_node (parser_context_t *context_p, ecma_string_t *imex_name_p, ecma_string_t *local_name_p);

#endif /* JERRY_MODULE_SYSTEM */

/**
 * @}
 *
 * \addtogroup jsparser_parser Parser
 * @{
 */

ecma_compiled_code_t *parser_parse_function (parser_context_t *context_p, uint32_t status_flags);
ecma_compiled_code_t *parser_parse_class_static_block (parser_context_t *context_p);
ecma_compiled_code_t *parser_parse_arrow_function (parser_context_t *context_p, uint32_t status_flags);
ecma_compiled_code_t *parser_parse_class_fields (parser_context_t *context_p);
void parser_set_function_name (parser_context_t *context_p,
                               uint16_t function_literal_index,
                               uint16_t name_index,
                               uint32_t status_flags);
void parser_compiled_code_set_function_name (parser_context_t *context_p,
                                             ecma_compiled_code_t *bytecode_p,
                                             uint16_t name_index,
                                             uint32_t status_flags);
uint16_t parser_check_anonymous_function_declaration (parser_context_t *context_p);

/* Error management. */

void parser_raise_error (parser_context_t *context_p, parser_error_msg_t error);

#if JERRY_PARSER_DUMP_BYTE_CODE
void util_print_cbc (ecma_compiled_code_t *compiled_code_p);
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

/**
 * @}
 * @}
 * @}
 */

#endif /* !JS_PARSER_INTERNAL_H */
