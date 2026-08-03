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

#ifndef JS_SCANNER_H
#define JS_SCANNER_H

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
 * Allowed types for scanner_info_t structures.
 */
typedef enum
{
  SCANNER_TYPE_END, /**< mark the last info block */
  SCANNER_TYPE_END_ARGUMENTS, /**< mark the end of function arguments
                               *   (only present if a function script is parsed) */
  SCANNER_TYPE_FUNCTION, /**< declarations in a function */
  SCANNER_TYPE_BLOCK, /**< declarations in a code block (usually enclosed in {}) */
  SCANNER_TYPE_WHILE, /**< while statement */
  SCANNER_TYPE_FOR, /**< for statement */
  SCANNER_TYPE_FOR_IN, /**< for-in statement */
  SCANNER_TYPE_FOR_OF, /**< for-of statement */
  SCANNER_TYPE_SWITCH, /**< switch statement */
  SCANNER_TYPE_CASE, /**< case statement */
  SCANNER_TYPE_INITIALIZER, /**< destructuring binding or assignment pattern with initializer */
  SCANNER_TYPE_LITERAL_FLAGS, /**< object or array literal with non-zero flags (stored in u8_arg) */
  SCANNER_TYPE_CLASS_CONSTRUCTOR, /**< class constructor */
  SCANNER_TYPE_CLASS_FIELD_INITIALIZER_END, /**< class field initializer end */
  SCANNER_TYPE_CLASS_STATIC_BLOCK_END, /**< class static block end */
  SCANNER_TYPE_LET_EXPRESSION, /**< let expression */
  SCANNER_TYPE_ERR_REDECLARED, /**< syntax error: a variable is redeclared */
  SCANNER_TYPE_ERR_ASYNC_FUNCTION, /**< an invalid async function follows */
  SCANNER_TYPE_EXPORT_MODULE_SPECIFIER, /**< export with module specifier */
} scanner_info_type_t;

/**
 * Source code location which can be used to change the position of parsing.
 */
typedef struct
{
  const uint8_t *source_p; /**< next source byte */
  parser_line_counter_t line; /**< token start line */
  parser_line_counter_t column; /**< token start column */
} scanner_location_t;

/**
 * Source code range with its start and end position.
 */
typedef struct
{
  const uint8_t *source_end_p; /**< end position */
  scanner_location_t start_location; /**< start location */
} scanner_range_t;

/**
 * Scanner info blocks which provides information for the parser.
 */
typedef struct scanner_info_t
{
  struct scanner_info_t *next_p; /**< next info structure */
  const uint8_t *source_p; /**< triggering position of this scanner info */
  uint8_t type; /**< type of the scanner info */
  uint8_t u8_arg; /**< custom 8-bit value */
  uint16_t u16_arg; /**< custom 16-bit value */
} scanner_info_t;

/**
 * Scanner info for class private field
 */
typedef struct scanner_class_private_member_t
{
  lexer_lit_location_t loc; /**< loc */
  uint8_t u8_arg; /**< custom 8-bit value */
  struct scanner_class_private_member_t *prev_p; /**< prev private field */
} scanner_class_private_member_t;

/**
 * Scanner info extended with class private fields.
 */
typedef struct
{
  scanner_info_t info; /**< header */
  scanner_class_private_member_t *members; /**< first private field */
} scanner_class_info_t;

/**
 * Scanner info extended with a location.
 */
typedef struct
{
  scanner_info_t info; /**< header */
  scanner_location_t location; /**< location */
} scanner_location_info_t;

/**
 * Scanner info for "for" statements.
 */
typedef struct
{
  scanner_info_t info; /**< header */
  scanner_location_t expression_location; /**< location of expression start */
  scanner_location_t end_location; /**< location of expression end */
} scanner_for_info_t;

/**
 * Case statement list for scanner_switch_info_t structure.
 */
typedef struct scanner_case_info_t
{
  struct scanner_case_info_t *next_p; /**< next case statement info */
  scanner_location_t location; /**< location of case statement */
} scanner_case_info_t;

/**
 * Scanner info for "switch" statements.
 */
typedef struct
{
  scanner_info_t info; /**< header */
  scanner_case_info_t *case_p; /**< list of switch cases */
} scanner_switch_info_t;

/*
 * Description of compressed streams.
 *
 * The stream is a sequence of commands which encoded as bytes. The first byte
 * contains the type of the command (see scanner_function_compressed_stream_types_t).
 *
 * The variable declaration commands has two arguments:
 *   - The first represents the length of the declared identifier
 *   - The second contains the relative distance from the end of the previous declaration
 *     Usually the distance is between 1 and 255, and represented as a single byte
 *     Distances between -256 and 65535 are encoded as two bytes
 *     Larger distances are encoded as pointers
 */

/**
 * Constants for compressed streams.
 */
typedef enum
{
  SCANNER_STREAM_UINT16_DIFF = (1 << 7), /**< relative distance is between -256 and 65535 */
  SCANNER_STREAM_HAS_ESCAPE = (1 << 6), /**< binding has escape */
  SCANNER_STREAM_NO_REG = (1 << 5), /**< binding cannot be stored in register */
  SCANNER_STREAM_EARLY_CREATE = (1 << 4), /**< binding must be created with ECMA_VALUE_UNINITIALIZED */
  SCANNER_STREAM_LOCAL_ARGUMENTS = SCANNER_STREAM_EARLY_CREATE, /**< arguments is redeclared
                                                                 *   as let/const binding later */
  /* Update SCANNER_STREAM_TYPE_MASK macro if more bits are added. */
} scanner_compressed_stream_flags_t;

/**
 * Types for compressed streams.
 */
typedef enum
{
  SCANNER_STREAM_TYPE_END, /**< end of scanner data */
  SCANNER_STREAM_TYPE_HOLE, /**< no name is assigned to this argument */
  SCANNER_STREAM_TYPE_ARGUMENTS, /**< arguments object should be created */
  SCANNER_STREAM_TYPE_ARGUMENTS_FUNC, /**< arguments object should be created which
                                       *   is later initialized with a function */
  SCANNER_STREAM_TYPE_VAR, /**< var declaration */
  SCANNER_STREAM_TYPE_LET, /**< let declaration */
  SCANNER_STREAM_TYPE_CONST, /**< const declaration */
  SCANNER_STREAM_TYPE_LOCAL, /**< local declaration (e.g. catch block) */
#if JERRY_MODULE_SYSTEM
  SCANNER_STREAM_TYPE_IMPORT, /**< module import */
#endif /* JERRY_MODULE_SYSTEM */
  /* The next four types must be in this order (see SCANNER_STREAM_TYPE_IS_ARG). */
  SCANNER_STREAM_TYPE_ARG, /**< argument declaration */
  SCANNER_STREAM_TYPE_ARG_VAR, /**< argument declaration which is later copied
                                *   into a variable declared by var statement */
  SCANNER_STREAM_TYPE_DESTRUCTURED_ARG, /**< destructuring argument declaration */
  SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR, /**< destructuring argument declaration which is later
                                             *   copied into a variable declared by var statement */
  /* Function types should be at the end. See the SCANNER_STREAM_TYPE_IS_FUNCTION macro. */
  SCANNER_STREAM_TYPE_ARG_FUNC, /**< argument declaration which
                                 *   is later initialized with a function */
  SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC, /**< destructuring argument declaration which
                                              *   is later initialized with a function */
  SCANNER_STREAM_TYPE_FUNC, /**< function declaration */
} scanner_compressed_stream_types_t;

/**
 * Mask for decoding the type from the compressed stream.
 */
#define SCANNER_STREAM_TYPE_MASK 0xf

/**
 * Checks whether the decoded type represents a function declaration.
 */
#define SCANNER_STREAM_TYPE_IS_FUNCTION(type) ((type) >= SCANNER_STREAM_TYPE_ARG_FUNC)

/**
 * Checks whether the decoded type represents a function argument.
 */
#define SCANNER_STREAM_TYPE_IS_ARG(type) \
  ((type) >= SCANNER_STREAM_TYPE_ARG && (type) <= SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR)

/**
 * Checks whether the decoded type represents both a function argument and a function declaration.
 */
#define SCANNER_STREAM_TYPE_IS_ARG_FUNC(type) \
  ((type) == SCANNER_STREAM_TYPE_ARG_FUNC || (type) == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC)

/**
 * Checks whether the decoded type represents an arguments declaration
 */
#define SCANNER_STREAM_TYPE_IS_ARGUMENTS(type) \
  ((type) == SCANNER_STREAM_TYPE_ARGUMENTS || (type) == SCANNER_STREAM_TYPE_ARGUMENTS_FUNC)

/**
 * Constants for u8_arg flags in scanner_function_info_t.
 */
typedef enum
{
  SCANNER_FUNCTION_ARGUMENTS_NEEDED = (1 << 0), /**< arguments object needs to be created */
  SCANNER_FUNCTION_HAS_COMPLEX_ARGUMENT = (1 << 1), /**< function has complex (ES2015+) argument definition */
  SCANNER_FUNCTION_LEXICAL_ENV_NEEDED = (1 << 2), /**< lexical environment is needed for the function body */
  SCANNER_FUNCTION_STATEMENT = (1 << 3), /**< function is function statement (not arrow expression)
                                          *   this flag must be combined with the type of function (e.g. async) */
  SCANNER_FUNCTION_ASYNC = (1 << 4), /**< function is async function */
  SCANNER_FUNCTION_IS_STRICT = (1 << 5), /**< function is strict */
} scanner_function_flags_t;

/**
 * Constants for u8_arg flags in scanner_class_info_t.
 */
typedef enum
{
  SCANNER_CONSTRUCTOR_IMPLICIT = 0, /**< implicit constructor */
  SCANNER_CONSTRUCTOR_EXPLICIT = (1 << 0), /**< explicit constructor */
  SCANNER_SUCCESSFUL_CLASS_SCAN = (1 << 1), /**< class scan was successful */
  SCANNER_PRIVATE_FIELD_ACTIVE = (1 << 2), /**< private field is active */
} scanner_constuctor_flags_t;

/**
 * Constants for u8_arg flags in scanner_class_private_member_t.
 */
typedef enum
{
  SCANNER_PRIVATE_FIELD_PROPERTY = (1 << 0), /**< private field initializer */
  SCANNER_PRIVATE_FIELD_METHOD = (1 << 1), /**< private field method */
  SCANNER_PRIVATE_FIELD_STATIC = (1 << 2), /**< static private property */
  SCANNER_PRIVATE_FIELD_GETTER = (1 << 3), /**< private field getter */
  SCANNER_PRIVATE_FIELD_SETTER = (1 << 4), /**< private field setter */
  SCANNER_PRIVATE_FIELD_SEEN = (1 << 5), /**< private field has already been seen */
  SCANNER_PRIVATE_FIELD_IGNORED = SCANNER_PRIVATE_FIELD_METHOD | SCANNER_PRIVATE_FIELD_STATIC,
  SCANNER_PRIVATE_FIELD_GETTER_SETTER = (SCANNER_PRIVATE_FIELD_GETTER | SCANNER_PRIVATE_FIELD_SETTER),
  SCANNER_PRIVATE_FIELD_PROPERTY_GETTER_SETTER = (SCANNER_PRIVATE_FIELD_PROPERTY | SCANNER_PRIVATE_FIELD_GETTER_SETTER),
} scanner_private_field_flags_t;

/**
 * Object or array literal constants for u8_arg flags in scanner_info_t.
 */
typedef enum
{
  /* These flags affects both array and object literals */
  SCANNER_LITERAL_DESTRUCTURING_FOR = (1 << 0), /**< for loop with destructuring pattern */
  SCANNER_LITERAL_NO_DESTRUCTURING = (1 << 1), /**< this literal cannot be a destructuring pattern */
  /* These flags affects only object literals */
  SCANNER_LITERAL_OBJECT_HAS_SUPER = (1 << 2), /**< super keyword is used in the object literal */
  SCANNER_LITERAL_OBJECT_HAS_REST = (1 << 3), /**< the object literal has a member prefixed with three dots */
} scanner_literal_flags_t;

/**
 * Option bits for scanner_create_variables function.
 */
typedef enum
{
  SCANNER_CREATE_VARS_NO_OPTS = 0, /**< no options */
  SCANNER_CREATE_VARS_IS_SCRIPT = (1 << 0), /**< create variables for script or direct eval */
  SCANNER_CREATE_VARS_IS_MODULE = (1 << 1), /**< create variables for module */
  SCANNER_CREATE_VARS_IS_FUNCTION_ARGS = (1 << 2), /**< create variables for function arguments */
  SCANNER_CREATE_VARS_IS_FUNCTION_BODY = (1 << 3), /**< create variables for function body */
} scanner_create_variables_flags_t;

/**
 * @}
 * @}
 * @}
 */

#endif /* !JS_SCANNER_H */
