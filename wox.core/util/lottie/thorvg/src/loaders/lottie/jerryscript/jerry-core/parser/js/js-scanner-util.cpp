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

#include "ecma-helpers.h"
#include "ecma-lex-env.h"

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

JERRY_STATIC_ASSERT (PARSER_MAXIMUM_NUMBER_OF_LITERALS + PARSER_MAXIMUM_NUMBER_OF_REGISTERS < PARSER_REGISTER_START,
                     maximum_number_of_literals_plus_registers_must_be_less_than_register_start);

JERRY_STATIC_ASSERT ((SCANNER_LITERAL_IS_ARROW_DESTRUCTURED_ARG & SCANNER_LITERAL_IS_LOCAL) == 0,
                     is_arrow_arg_binding_flag_must_not_use_local_flags);

JERRY_STATIC_ASSERT ((SCANNER_LITERAL_IS_LET & SCANNER_LITERAL_IS_LOCAL) != 0, is_let_flag_must_use_local_flags);

JERRY_STATIC_ASSERT ((SCANNER_LITERAL_IS_CONST & SCANNER_LITERAL_IS_LOCAL) != 0, is_const_flag_must_use_local_flags);

JERRY_STATIC_ASSERT ((SCANNER_LITERAL_IS_FUNC_DECLARATION & SCANNER_LITERAL_IS_LOCAL) != 0,
                     is_func_declaration_flag_must_use_local_flags);

JERRY_STATIC_ASSERT ((SCANNER_LITERAL_IS_DESTRUCTURED_ARG & SCANNER_LITERAL_IS_LOCAL) != 0,
                     is_arg_binding_flag_must_use_local_flags);

JERRY_STATIC_ASSERT (((int)SCANNER_LITERAL_IS_FUNC_DECLARATION != (int)SCANNER_LITERAL_IS_DESTRUCTURED_ARG),
                     is_func_declaration_must_be_different_from_is_arg_binding);

JERRY_STATIC_ASSERT (((int)PARSER_SCOPE_STACK_IS_CONST_REG == (int)PARSER_SCOPE_STACK_IS_LOCAL_CREATED),
                     scope_stack_is_const_reg_and_scope_stack_is_local_created_must_be_the_same);

/**
 * Raise a scanner error.
 */
void
scanner_raise_error (parser_context_t *context_p /**< context */)
{
  PARSER_THROW (context_p->try_buffer);
  /* Should never been reached. */
  JERRY_ASSERT (0);
} /* scanner_raise_error */

/**
 * Raise a variable redeclaration error.
 */
void
scanner_raise_redeclaration_error (parser_context_t *context_p) /**< context */
{
  scanner_info_t *info_p = scanner_insert_info (context_p, context_p->source_p, sizeof (scanner_info_t));
  info_p->type = SCANNER_TYPE_ERR_REDECLARED;

  scanner_raise_error (context_p);
} /* scanner_raise_redeclaration_error */

/**
 * Allocate memory for scanner.
 *
 * @return allocated memory
 */
void *
scanner_malloc (parser_context_t *context_p, /**< context */
                size_t size) /**< size of the memory block */
{
  void *result;

  JERRY_ASSERT (size > 0);
  result = jmem_heap_alloc_block_null_on_error (size);

  if (result == NULL)
  {
    scanner_cleanup (context_p);

    /* This is the only error which specify its reason. */
    context_p->error = PARSER_ERR_OUT_OF_MEMORY;
    PARSER_THROW (context_p->try_buffer);
  }
  return result;
} /* scanner_malloc */

/**
 * Free memory allocated by scanner_malloc.
 */
void
scanner_free (void *ptr, /**< pointer to free */
              size_t size) /**< size of the memory block */
{
  jmem_heap_free_block (ptr, size);
} /* scanner_free */

/**
 * Count the size of a stream after an info block.
 *
 * @return the size in bytes
 */
size_t
scanner_get_stream_size (scanner_info_t *info_p, /**< scanner info block */
                         size_t size) /**< size excluding the stream */
{
  const uint8_t *data_p = ((const uint8_t *) info_p) + size;
  const uint8_t *data_p_start = data_p;

  while (data_p[0] != SCANNER_STREAM_TYPE_END)
  {
    switch (data_p[0] & SCANNER_STREAM_TYPE_MASK)
    {
      case SCANNER_STREAM_TYPE_VAR:
      case SCANNER_STREAM_TYPE_LET:
      case SCANNER_STREAM_TYPE_CONST:
      case SCANNER_STREAM_TYPE_LOCAL:
#if JERRY_MODULE_SYSTEM
      case SCANNER_STREAM_TYPE_IMPORT:
#endif /* JERRY_MODULE_SYSTEM */
      case SCANNER_STREAM_TYPE_ARG:
      case SCANNER_STREAM_TYPE_ARG_VAR:
      case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG:
      case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR:
      case SCANNER_STREAM_TYPE_ARG_FUNC:
      case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC:
      case SCANNER_STREAM_TYPE_FUNC:
      {
        break;
      }
      default:
      {
        JERRY_ASSERT ((data_p[0] & SCANNER_STREAM_TYPE_MASK) == SCANNER_STREAM_TYPE_HOLE
                      || SCANNER_STREAM_TYPE_IS_ARGUMENTS (data_p[0] & SCANNER_STREAM_TYPE_MASK));
        data_p++;
        continue;
      }
    }

    data_p += 3;

    if (data_p[-3] & SCANNER_STREAM_UINT16_DIFF)
    {
      data_p++;
    }
    else if (data_p[-1] == 0)
    {
      data_p += sizeof (const uint8_t *);
    }
  }

  return size + 1 + (size_t) (data_p - data_p_start);
} /* scanner_get_stream_size */

/**
 * Insert a scanner info block into the scanner info chain.
 *
 * @return newly allocated scanner info
 */
scanner_info_t *
scanner_insert_info (parser_context_t *context_p, /**< context */
                     const uint8_t *source_p, /**< triggering position */
                     size_t size) /**< size of the memory block */
{
  scanner_info_t *new_scanner_info_p = (scanner_info_t *) scanner_malloc (context_p, size);
  scanner_info_t *scanner_info_p = context_p->next_scanner_info_p;
  scanner_info_t *prev_scanner_info_p = NULL;

  JERRY_ASSERT (scanner_info_p != NULL);
  JERRY_ASSERT (source_p != NULL);

  new_scanner_info_p->source_p = source_p;

  while (source_p < scanner_info_p->source_p)
  {
    prev_scanner_info_p = scanner_info_p;
    scanner_info_p = scanner_info_p->next_p;

    JERRY_ASSERT (scanner_info_p != NULL);
  }

  /* Multiple scanner info blocks cannot be assigned to the same position. */
  JERRY_ASSERT (source_p != scanner_info_p->source_p);

  new_scanner_info_p->next_p = scanner_info_p;

  if (JERRY_LIKELY (prev_scanner_info_p == NULL))
  {
    context_p->next_scanner_info_p = new_scanner_info_p;
  }
  else
  {
    prev_scanner_info_p->next_p = new_scanner_info_p;
  }

  return new_scanner_info_p;
} /* scanner_insert_info */

/**
 * Insert a scanner info block into the scanner info chain before a given info block.
 *
 * @return newly allocated scanner info
 */
scanner_info_t *
scanner_insert_info_before (parser_context_t *context_p, /**< context */
                            const uint8_t *source_p, /**< triggering position */
                            scanner_info_t *start_info_p, /**< first info position */
                            size_t size) /**< size of the memory block */
{
  JERRY_ASSERT (start_info_p != NULL);

  scanner_info_t *new_scanner_info_p = (scanner_info_t *) scanner_malloc (context_p, size);
  scanner_info_t *scanner_info_p = start_info_p->next_p;
  scanner_info_t *prev_scanner_info_p = start_info_p;

  new_scanner_info_p->source_p = source_p;

  while (source_p < scanner_info_p->source_p)
  {
    prev_scanner_info_p = scanner_info_p;
    scanner_info_p = scanner_info_p->next_p;

    JERRY_ASSERT (scanner_info_p != NULL);
  }

  /* Multiple scanner info blocks cannot be assigned to the same position. */
  JERRY_ASSERT (source_p != scanner_info_p->source_p);

  new_scanner_info_p->next_p = scanner_info_p;

  prev_scanner_info_p->next_p = new_scanner_info_p;
  return new_scanner_info_p;
} /* scanner_insert_info_before */

/**
 * Release the next scanner info.
 */
void
scanner_release_next (parser_context_t *context_p, /**< context */
                      size_t size) /**< size of the memory block */
{
  scanner_info_t *next_p = context_p->next_scanner_info_p->next_p;

  jmem_heap_free_block (context_p->next_scanner_info_p, size);
  context_p->next_scanner_info_p = next_p;
} /* scanner_release_next */

/**
 * Set the active scanner info to the next scanner info.
 */
void
scanner_set_active (parser_context_t *context_p) /**< context */
{
  scanner_info_t *scanner_info_p = context_p->next_scanner_info_p;

  context_p->next_scanner_info_p = scanner_info_p->next_p;
  scanner_info_p->next_p = context_p->active_scanner_info_p;
  context_p->active_scanner_info_p = scanner_info_p;
} /* scanner_set_active */

/**
 * Set the next scanner info to the active scanner info.
 */
void
scanner_revert_active (parser_context_t *context_p) /**< context */
{
  scanner_info_t *scanner_info_p = context_p->active_scanner_info_p;

  context_p->active_scanner_info_p = scanner_info_p->next_p;
  scanner_info_p->next_p = context_p->next_scanner_info_p;
  context_p->next_scanner_info_p = scanner_info_p;
} /* scanner_revert_active */

/**
 * Release the active scanner info.
 */
void
scanner_release_active (parser_context_t *context_p, /**< context */
                        size_t size) /**< size of the memory block */
{
  scanner_info_t *next_p = context_p->active_scanner_info_p->next_p;

  jmem_heap_free_block (context_p->active_scanner_info_p, size);
  context_p->active_scanner_info_p = next_p;
} /* scanner_release_active */

/**
 * Release switch cases.
 */
void
scanner_release_switch_cases (scanner_case_info_t *case_p) /**< case list */
{
  while (case_p != NULL)
  {
    scanner_case_info_t *next_p = case_p->next_p;

    jmem_heap_free_block (case_p, sizeof (scanner_case_info_t));
    case_p = next_p;
  }
} /* scanner_release_switch_cases */

/**
 * Release private fields.
 */
void
scanner_release_private_fields (scanner_class_private_member_t *member_p) /**< private member list */
{
  while (member_p != NULL)
  {
    scanner_class_private_member_t *prev_p = member_p->prev_p;

    jmem_heap_free_block (member_p, sizeof (scanner_class_private_member_t));
    member_p = prev_p;
  }
} /* scanner_release_private_fields */

/**
 * Seek to correct position in the scanner info list.
 */
void
scanner_seek (parser_context_t *context_p) /**< context */
{
  const uint8_t *source_p = context_p->source_p;
  scanner_info_t *prev_p;

  if (context_p->skipped_scanner_info_p != NULL)
  {
    JERRY_ASSERT (context_p->skipped_scanner_info_p->source_p != NULL);

    context_p->skipped_scanner_info_end_p->next_p = context_p->next_scanner_info_p;

    if (context_p->skipped_scanner_info_end_p->source_p <= source_p)
    {
      prev_p = context_p->skipped_scanner_info_end_p;
    }
    else
    {
      prev_p = context_p->skipped_scanner_info_p;

      if (prev_p->source_p > source_p)
      {
        context_p->next_scanner_info_p = prev_p;
        context_p->skipped_scanner_info_p = NULL;
        return;
      }

      context_p->skipped_scanner_info_p = prev_p;
    }
  }
  else
  {
    prev_p = context_p->next_scanner_info_p;

    if (prev_p->source_p == NULL || prev_p->source_p > source_p)
    {
      return;
    }

    context_p->skipped_scanner_info_p = prev_p;
  }

  while (prev_p->next_p->source_p != NULL && prev_p->next_p->source_p <= source_p)
  {
    prev_p = prev_p->next_p;
  }

  context_p->skipped_scanner_info_end_p = prev_p;
  context_p->next_scanner_info_p = prev_p->next_p;
} /* scanner_seek */

/**
 * Checks whether a literal is equal to "arguments".
 */
static inline bool
scanner_literal_is_arguments (lexer_lit_location_t *literal_p) /**< literal */
{
  return lexer_compare_identifier_to_string (literal_p, (const uint8_t *) "arguments", 9);
} /* scanner_literal_is_arguments */

/**
 * Find if there is a duplicated argument in the given context
 *
 * @return true - if there are duplicates, false - otherwise
 */
static bool
scanner_find_duplicated_arg (parser_context_t *context_p, lexer_lit_location_t *lit_loc_p)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (!(context_p->status_flags & PARSER_FUNCTION_IS_PARSING_ARGS))
  {
    return false;
  }

  if (scanner_literal_is_arguments (lit_loc_p))
  {
    return true;
  }

  uint16_t register_end, encoding_limit, encoding_delta;
  ecma_value_t *literal_p;
  ecma_value_t *literal_start_p;

  const ecma_compiled_code_t *bytecode_header_p = JERRY_CONTEXT (vm_top_context_p)->shared_p->bytecode_header_p;

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) bytecode_header_p;

    register_end = args_p->register_end;

    literal_p = (ecma_value_t *) (args_p + 1);
    literal_p -= register_end;
    literal_start_p = literal_p;
    literal_p += args_p->literal_end;
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) bytecode_header_p;

    register_end = args_p->register_end;

    literal_p = (ecma_value_t *) (args_p + 1);
    literal_p -= register_end;
    literal_start_p = literal_p;
    literal_p += args_p->literal_end;
  }

  if (!(bytecode_header_p->status_flags & CBC_CODE_FLAGS_FULL_LITERAL_ENCODING))
  {
    encoding_limit = CBC_SMALL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_SMALL_LITERAL_ENCODING_DELTA;
  }
  else
  {
    encoding_limit = CBC_FULL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_FULL_LITERAL_ENCODING_DELTA;
  }

  uint8_t *byte_code_p = (uint8_t *) literal_p;

  bool found_duplicate = false;

  while (*byte_code_p == CBC_CREATE_LOCAL)
  {
    byte_code_p++;
    uint16_t literal_index = *byte_code_p++;

    if (literal_index >= encoding_limit)
    {
      literal_index = (uint16_t) (((literal_index << 8) | *byte_code_p++) - encoding_delta);
    }

    ecma_string_t *arg_string = ecma_get_string_from_value (literal_start_p[literal_index]);
    uint8_t *destination_p = (uint8_t *) parser_malloc (context_p, lit_loc_p->length);
    lexer_convert_ident_to_cesu8 (destination_p, lit_loc_p->char_p, lit_loc_p->length);
    ecma_string_t *search_key_p = ecma_new_ecma_string_from_utf8 (destination_p, lit_loc_p->length);
    scanner_free (destination_p, lit_loc_p->length);

    found_duplicate = ecma_compare_ecma_strings (arg_string, search_key_p);
    ecma_deref_ecma_string (search_key_p);

    if (found_duplicate)
    {
      break;
    }
  }

  return found_duplicate;
} /* scanner_find_duplicated_arg */

/**
 * Find any let/const declaration of a given literal.
 *
 * @return true - if the literal is found, false - otherwise
 */
static bool
scanner_scope_find_lexical_declaration (parser_context_t *context_p, /**< context */
                                        lexer_lit_location_t *literal_p) /**< literal */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_string_t *name_p;
  uint32_t flags = context_p->global_status_flags;

  if (!(flags & ECMA_PARSE_EVAL) || (!(flags & ECMA_PARSE_DIRECT_EVAL) && (context_p->status_flags & PARSER_IS_STRICT)))
  {
    return false;
  }

  if (JERRY_LIKELY (!(literal_p->status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)))
  {
    name_p = ecma_new_ecma_string_from_utf8 (literal_p->char_p, literal_p->length);
  }
  else
  {
    uint8_t *destination_p = (uint8_t *) scanner_malloc (context_p, literal_p->length);

    lexer_convert_ident_to_cesu8 (destination_p, literal_p->char_p, literal_p->length);

    name_p = ecma_new_ecma_string_from_utf8 (destination_p, literal_p->length);

    scanner_free (destination_p, literal_p->length);
  }

  ecma_object_t *lex_env_p;

  if (flags & ECMA_PARSE_DIRECT_EVAL)
  {
    lex_env_p = JERRY_CONTEXT (vm_top_context_p)->lex_env_p;

    while (lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK)
    {
      if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
      {
        ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

        if (property_p != NULL && ecma_is_property_enumerable (*property_p))
        {
          ecma_deref_ecma_string (name_p);
          return true;
        }
      }

      JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
      lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
    }
  }
  else
  {
    lex_env_p = ecma_get_global_scope (ecma_builtin_get_global ());
  }

  if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
  {
    ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

    if (property_p != NULL
        && (ecma_is_property_enumerable (*property_p) || scanner_find_duplicated_arg (context_p, literal_p)))
    {
      ecma_deref_ecma_string (name_p);
      return true;
    }
  }

  ecma_deref_ecma_string (name_p);
  return false;
} /* scanner_scope_find_lexical_declaration */

/**
 * Push a new literal pool.
 *
 * @return the newly created literal pool
 */
scanner_literal_pool_t *
scanner_push_literal_pool (parser_context_t *context_p, /**< context */
                           scanner_context_t *scanner_context_p, /**< scanner context */
                           uint16_t status_flags) /**< combination of scanner_literal_pool_flags_t flags */
{
  scanner_literal_pool_t *prev_literal_pool_p = scanner_context_p->active_literal_pool_p;
  scanner_literal_pool_t *literal_pool_p;

  literal_pool_p = (scanner_literal_pool_t *) scanner_malloc (context_p, sizeof (scanner_literal_pool_t));

  if (!(status_flags & SCANNER_LITERAL_POOL_FUNCTION))
  {
    JERRY_ASSERT (prev_literal_pool_p != NULL);
    status_flags |= SCANNER_LITERAL_POOL_NO_ARGUMENTS;

    const uint16_t copied_flags =
      (SCANNER_LITERAL_POOL_IN_WITH | SCANNER_LITERAL_POOL_GENERATOR | SCANNER_LITERAL_POOL_ASYNC);

    status_flags |= (uint16_t) (prev_literal_pool_p->status_flags & copied_flags);
  }
  else
  {
    context_p->status_flags &= (uint32_t) ~(PARSER_IS_GENERATOR_FUNCTION | PARSER_IS_ASYNC_FUNCTION);

    if (status_flags & SCANNER_LITERAL_POOL_GENERATOR)
    {
      context_p->status_flags |= PARSER_IS_GENERATOR_FUNCTION;
    }

    if (status_flags & SCANNER_LITERAL_POOL_ASYNC)
    {
      context_p->status_flags |= PARSER_IS_ASYNC_FUNCTION;
    }
  }

  if (prev_literal_pool_p != NULL)
  {
    const uint16_t copied_flags = SCANNER_LITERAL_POOL_IS_STRICT;
    status_flags |= (uint16_t) (prev_literal_pool_p->status_flags & copied_flags);

    /* The logical value of these flags must be the same. */
    JERRY_ASSERT (!(status_flags & SCANNER_LITERAL_POOL_IS_STRICT) == !(context_p->status_flags & PARSER_IS_STRICT));
  }

  parser_list_init (&literal_pool_p->literal_pool,
                    sizeof (lexer_lit_location_t),
                    (uint32_t) ((128 - sizeof (void *)) / sizeof (lexer_lit_location_t)));
  literal_pool_p->source_p = NULL;
  literal_pool_p->status_flags = status_flags;
  literal_pool_p->no_declarations = 0;

  literal_pool_p->prev_p = prev_literal_pool_p;
  scanner_context_p->active_literal_pool_p = literal_pool_p;

  return literal_pool_p;
} /* scanner_push_literal_pool */

JERRY_STATIC_ASSERT (((uint8_t)PARSER_MAXIMUM_IDENT_LENGTH <= UINT8_MAX), maximum_ident_length_must_fit_in_a_byte);

/**
 * Current status of arguments.
 */
typedef enum
{
  SCANNER_ARGUMENTS_NOT_PRESENT, /**< arguments object must not be created */
  SCANNER_ARGUMENTS_MAY_PRESENT, /**< arguments object can be created */
  SCANNER_ARGUMENTS_MAY_PRESENT_IN_EVAL, /**< arguments object must be present unless otherwise declared */
  SCANNER_ARGUMENTS_PRESENT, /**< arguments object must be created */
  SCANNER_ARGUMENTS_PRESENT_NO_REG, /**< arguments object must be created and cannot be stored in registers */
} scanner_arguments_type_t;

/**
 * Pop the last literal pool from the end.
 */
void
scanner_pop_literal_pool (parser_context_t *context_p, /**< context */
                          scanner_context_t *scanner_context_p) /**< scanner context */
{
  scanner_literal_pool_t *literal_pool_p = scanner_context_p->active_literal_pool_p;
  scanner_literal_pool_t *prev_literal_pool_p = literal_pool_p->prev_p;

  const uint32_t arrow_super_flags = (SCANNER_LITERAL_POOL_ARROW | SCANNER_LITERAL_POOL_HAS_SUPER_REFERENCE);
  if ((literal_pool_p->status_flags & arrow_super_flags) == arrow_super_flags)
  {
    prev_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_HAS_SUPER_REFERENCE;
  }

  if (JERRY_UNLIKELY (literal_pool_p->source_p == NULL))
  {
    JERRY_ASSERT (literal_pool_p->status_flags & SCANNER_LITERAL_POOL_FUNCTION);
    JERRY_ASSERT (literal_pool_p->literal_pool.data.first_p == NULL
                  && literal_pool_p->literal_pool.data.last_p == NULL);

    scanner_context_p->active_literal_pool_p = literal_pool_p->prev_p;
    scanner_free (literal_pool_p, sizeof (scanner_literal_pool_t));
    return;
  }

  uint16_t status_flags = literal_pool_p->status_flags;
  scanner_arguments_type_t arguments_type = SCANNER_ARGUMENTS_MAY_PRESENT;

  if (status_flags & SCANNER_LITERAL_POOL_NO_ARGUMENTS)
  {
    arguments_type = SCANNER_ARGUMENTS_NOT_PRESENT;
  }
  else if (status_flags & SCANNER_LITERAL_POOL_CAN_EVAL)
  {
    arguments_type = SCANNER_ARGUMENTS_MAY_PRESENT_IN_EVAL;
  }

  if (status_flags & SCANNER_LITERAL_POOL_ARGUMENTS_IN_ARGS)
  {
    arguments_type = SCANNER_ARGUMENTS_PRESENT;

    if (status_flags & (SCANNER_LITERAL_POOL_NO_ARGUMENTS | SCANNER_LITERAL_POOL_CAN_EVAL))
    {
      arguments_type = SCANNER_ARGUMENTS_PRESENT_NO_REG;
      status_flags &= (uint16_t) ~SCANNER_LITERAL_POOL_NO_ARGUMENTS;
    }
  }

  uint8_t can_eval_types = 0;

  if (prev_literal_pool_p == NULL && !(context_p->global_status_flags & ECMA_PARSE_DIRECT_EVAL))
  {
    can_eval_types |= SCANNER_LITERAL_IS_FUNC;
  }

  if ((status_flags & SCANNER_LITERAL_POOL_CAN_EVAL) && prev_literal_pool_p != NULL)
  {
    prev_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_CAN_EVAL;
  }

  parser_list_iterator_t literal_iterator;
  lexer_lit_location_t *literal_p;
  int32_t no_declarations = literal_pool_p->no_declarations;

  parser_list_iterator_init (&literal_pool_p->literal_pool, &literal_iterator);

  uint8_t arguments_stream_type = SCANNER_STREAM_TYPE_ARGUMENTS;
  const uint8_t *prev_source_p = literal_pool_p->source_p - 1;
  lexer_lit_location_t *last_argument_p = NULL;
  size_t compressed_size = 1;

  while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
  {
    uint8_t type = literal_p->type;

    if (JERRY_UNLIKELY (no_declarations > PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK))
    {
      continue;
    }

    if (!(status_flags & SCANNER_LITERAL_POOL_NO_ARGUMENTS) && scanner_literal_is_arguments (literal_p))
    {
      JERRY_ASSERT (arguments_type != SCANNER_ARGUMENTS_NOT_PRESENT);
      status_flags |= SCANNER_LITERAL_POOL_NO_ARGUMENTS;

      if (type & SCANNER_LITERAL_IS_ARG)
      {
        JERRY_ASSERT (arguments_type != SCANNER_ARGUMENTS_PRESENT
                      && arguments_type != SCANNER_ARGUMENTS_PRESENT_NO_REG);
        arguments_type = SCANNER_ARGUMENTS_NOT_PRESENT;
        last_argument_p = literal_p;
      }
      else if (type & SCANNER_LITERAL_IS_LOCAL)
      {
        if (arguments_type == SCANNER_ARGUMENTS_MAY_PRESENT || arguments_type == SCANNER_ARGUMENTS_MAY_PRESENT_IN_EVAL)
        {
          arguments_type = SCANNER_ARGUMENTS_NOT_PRESENT;
        }
        else
        {
          if (arguments_type == SCANNER_ARGUMENTS_PRESENT_NO_REG)
          {
            type |= SCANNER_LITERAL_NO_REG;
          }
          else if (type & (SCANNER_LITERAL_NO_REG | SCANNER_LITERAL_EARLY_CREATE))
          {
            arguments_type = SCANNER_ARGUMENTS_PRESENT_NO_REG;
          }

          if ((type & SCANNER_LITERAL_IS_LOCAL_FUNC) == SCANNER_LITERAL_IS_LOCAL_FUNC)
          {
            type |= SCANNER_LITERAL_IS_ARG;
            literal_p->type = type;
            no_declarations--;
            arguments_stream_type = SCANNER_STREAM_TYPE_ARGUMENTS_FUNC;
          }
          else
          {
            arguments_stream_type |= SCANNER_STREAM_LOCAL_ARGUMENTS;
          }
        }
      }
      else
      {
        if ((type & SCANNER_LITERAL_IS_VAR)
            && (arguments_type == SCANNER_ARGUMENTS_PRESENT || arguments_type == SCANNER_ARGUMENTS_PRESENT_NO_REG))
        {
          if (arguments_type == SCANNER_ARGUMENTS_PRESENT_NO_REG)
          {
            type |= SCANNER_LITERAL_NO_REG;
          }
          else if (type & (SCANNER_LITERAL_NO_REG | SCANNER_LITERAL_EARLY_CREATE))
          {
            arguments_type = SCANNER_ARGUMENTS_PRESENT_NO_REG;
          }

          type |= SCANNER_LITERAL_IS_ARG;
          literal_p->type = type;
          no_declarations--;
        }

        if ((type & SCANNER_LITERAL_NO_REG) || arguments_type == SCANNER_ARGUMENTS_MAY_PRESENT_IN_EVAL)
        {
          arguments_type = SCANNER_ARGUMENTS_PRESENT_NO_REG;
        }
        else if (arguments_type == SCANNER_ARGUMENTS_MAY_PRESENT)
        {
          arguments_type = SCANNER_ARGUMENTS_PRESENT;
        }

        /* The SCANNER_LITERAL_IS_ARG may be set above. */
        if (!(type & SCANNER_LITERAL_IS_ARG))
        {
          literal_p->type = 0;
          continue;
        }
      }
    }
    else if (type & SCANNER_LITERAL_IS_ARG)
    {
      last_argument_p = literal_p;
    }

    if ((status_flags & SCANNER_LITERAL_POOL_FUNCTION)
        && (type & SCANNER_LITERAL_IS_LOCAL_FUNC) == SCANNER_LITERAL_IS_FUNC)
    {
      if (prev_literal_pool_p == NULL && scanner_scope_find_lexical_declaration (context_p, literal_p))
      {
        literal_p->type = 0;
        continue;
      }

      if (!(type & SCANNER_LITERAL_IS_ARG))
      {
        type |= SCANNER_LITERAL_IS_VAR;
      }

      type &= (uint8_t) ~SCANNER_LITERAL_IS_FUNC;
      literal_p->type = type;
    }

    if ((type & SCANNER_LITERAL_IS_LOCAL)
        || ((type & (SCANNER_LITERAL_IS_VAR | SCANNER_LITERAL_IS_ARG))
            && (status_flags & SCANNER_LITERAL_POOL_FUNCTION)))
    {
      JERRY_ASSERT ((status_flags & SCANNER_LITERAL_POOL_FUNCTION) || !(literal_p->type & SCANNER_LITERAL_IS_ARG));

      if (literal_p->length == 0)
      {
        compressed_size += 1;
        continue;
      }

      no_declarations++;

      if ((status_flags & SCANNER_LITERAL_POOL_CAN_EVAL) || (type & can_eval_types))
      {
        type |= SCANNER_LITERAL_NO_REG;
        literal_p->type = type;
      }

      if (type & SCANNER_LITERAL_IS_FUNC)
      {
        no_declarations++;

        if ((type & (SCANNER_LITERAL_IS_CONST | SCANNER_LITERAL_IS_ARG)) == SCANNER_LITERAL_IS_CONST)
        {
          JERRY_ASSERT (type & SCANNER_LITERAL_IS_LET);

          /* Catch parameters cannot be functions. */
          literal_p->type = (uint8_t) (type & ~SCANNER_LITERAL_IS_FUNC);
          no_declarations--;
        }
      }

      intptr_t diff = (intptr_t) (literal_p->char_p - prev_source_p);

      if (diff >= 1 && diff <= (intptr_t) UINT8_MAX)
      {
        compressed_size += 2 + 1;
      }
      else if (diff >= -(intptr_t) UINT8_MAX && diff <= (intptr_t) UINT16_MAX)
      {
        compressed_size += 2 + 2;
      }
      else
      {
        compressed_size += 2 + 1 + sizeof (const uint8_t *);
      }

      prev_source_p = literal_p->char_p + literal_p->length;

      if ((status_flags & SCANNER_LITERAL_POOL_FUNCTION)
          || ((type & SCANNER_LITERAL_IS_FUNC) && (status_flags & SCANNER_LITERAL_POOL_IS_STRICT))
          || !(type & (SCANNER_LITERAL_IS_VAR | SCANNER_LITERAL_IS_FUNC)))
      {
        continue;
      }
    }

    if (prev_literal_pool_p != NULL && literal_p->length > 0)
    {
      /* Propagate literal to upper level. */
      lexer_lit_location_t *literal_location_p = scanner_add_custom_literal (context_p, prev_literal_pool_p, literal_p);
      uint8_t extended_type = literal_location_p->type;

      if ((status_flags & (SCANNER_LITERAL_POOL_FUNCTION | SCANNER_LITERAL_POOL_CLASS_FIELD))
          || (type & SCANNER_LITERAL_NO_REG))
      {
        extended_type |= SCANNER_LITERAL_NO_REG;
      }

      extended_type |= SCANNER_LITERAL_IS_USED;

      if (status_flags & SCANNER_LITERAL_POOL_FUNCTION_STATEMENT)
      {
        extended_type |= SCANNER_LITERAL_EARLY_CREATE;
      }

      const uint8_t mask = (SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_LOCAL);

      if ((type & SCANNER_LITERAL_IS_ARG) || (literal_location_p->type & mask) == SCANNER_LITERAL_IS_LET
          || (literal_location_p->type & mask) == SCANNER_LITERAL_IS_CONST)
      {
        /* Clears the SCANNER_LITERAL_IS_VAR and SCANNER_LITERAL_IS_FUNC flags
         * for speculative arrow parameters and local (non-var) functions. */
        type = 0;
      }

      type = (uint8_t) (type & (SCANNER_LITERAL_IS_VAR | SCANNER_LITERAL_IS_FUNC));
      JERRY_ASSERT (type == 0 || !(status_flags & SCANNER_LITERAL_POOL_FUNCTION));

      literal_location_p->type = (uint8_t) (extended_type | type);
    }
  }

  if ((status_flags & SCANNER_LITERAL_POOL_FUNCTION) || (compressed_size > 1))
  {
    if (arguments_type == SCANNER_ARGUMENTS_MAY_PRESENT)
    {
      arguments_type = SCANNER_ARGUMENTS_NOT_PRESENT;
    }
    else if (arguments_type == SCANNER_ARGUMENTS_MAY_PRESENT_IN_EVAL)
    {
      arguments_type = SCANNER_ARGUMENTS_PRESENT_NO_REG;
    }

    if (arguments_type != SCANNER_ARGUMENTS_NOT_PRESENT)
    {
      compressed_size++;
    }

    compressed_size += sizeof (scanner_info_t);

    scanner_info_t *info_p;

    if (prev_literal_pool_p != NULL || scanner_context_p->end_arguments_p == NULL)
    {
      info_p = scanner_insert_info (context_p, literal_pool_p->source_p, compressed_size);
    }
    else
    {
      scanner_info_t *start_info_p = scanner_context_p->end_arguments_p;
      info_p = scanner_insert_info_before (context_p, literal_pool_p->source_p, start_info_p, compressed_size);
    }

    if (no_declarations > PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK)
    {
      no_declarations = PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK;
    }

    uint8_t *data_p = (uint8_t *) (info_p + 1);
    bool mapped_arguments = false;

    if (status_flags & SCANNER_LITERAL_POOL_FUNCTION)
    {
      info_p->type = SCANNER_TYPE_FUNCTION;

      uint8_t u8_arg = 0;

      if (arguments_type != SCANNER_ARGUMENTS_NOT_PRESENT)
      {
        u8_arg |= SCANNER_FUNCTION_ARGUMENTS_NEEDED;

        if (no_declarations < PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK)
        {
          no_declarations++;
        }

        if (!(status_flags & (SCANNER_LITERAL_POOL_IS_STRICT | SCANNER_LITERAL_POOL_HAS_COMPLEX_ARGUMENT)))
        {
          mapped_arguments = true;
        }

        if (arguments_type == SCANNER_ARGUMENTS_PRESENT_NO_REG)
        {
          arguments_stream_type |= SCANNER_STREAM_NO_REG;
        }

        if (last_argument_p == NULL)
        {
          *data_p++ = arguments_stream_type;
        }
      }
      else
      {
        last_argument_p = NULL;
      }

      if (status_flags & (SCANNER_LITERAL_POOL_HAS_COMPLEX_ARGUMENT | SCANNER_LITERAL_POOL_ARROW))
      {
        u8_arg |= SCANNER_FUNCTION_HAS_COMPLEX_ARGUMENT;
      }

      if (status_flags & SCANNER_LITERAL_POOL_ASYNC)
      {
        u8_arg |= SCANNER_FUNCTION_ASYNC;

        if (status_flags & SCANNER_LITERAL_POOL_FUNCTION_STATEMENT)
        {
          u8_arg |= SCANNER_FUNCTION_STATEMENT;
        }
      }

      if (status_flags & SCANNER_LITERAL_POOL_CAN_EVAL)
      {
        u8_arg |= SCANNER_FUNCTION_LEXICAL_ENV_NEEDED;
      }

      if (status_flags & SCANNER_LITERAL_POOL_IS_STRICT)
      {
        u8_arg |= SCANNER_FUNCTION_IS_STRICT;
      }

      info_p->u8_arg = u8_arg;
      info_p->u16_arg = (uint16_t) no_declarations;
    }
    else
    {
      info_p->type = SCANNER_TYPE_BLOCK;

      JERRY_ASSERT (prev_literal_pool_p != NULL);
    }

    parser_list_iterator_init (&literal_pool_p->literal_pool, &literal_iterator);
    prev_source_p = literal_pool_p->source_p - 1;
    no_declarations = literal_pool_p->no_declarations;

    while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
    {
      if (JERRY_UNLIKELY (no_declarations > PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK)
          || (!(literal_p->type & SCANNER_LITERAL_IS_LOCAL)
              && (!(literal_p->type & (SCANNER_LITERAL_IS_VAR | SCANNER_LITERAL_IS_ARG))
                  || !(status_flags & SCANNER_LITERAL_POOL_FUNCTION))))
      {
        continue;
      }

      if (literal_p->length == 0)
      {
        *data_p++ = SCANNER_STREAM_TYPE_HOLE;

        if (literal_p == last_argument_p)
        {
          *data_p++ = arguments_stream_type;
        }
        continue;
      }

      no_declarations++;

      uint8_t type = SCANNER_STREAM_TYPE_VAR;

      if (literal_p->type & SCANNER_LITERAL_IS_FUNC)
      {
        no_declarations++;
        type = SCANNER_STREAM_TYPE_FUNC;

        if (literal_p->type & SCANNER_LITERAL_IS_ARG)
        {
          type = SCANNER_STREAM_TYPE_ARG_FUNC;

          if (literal_p->type & SCANNER_LITERAL_IS_DESTRUCTURED_ARG)
          {
            type = SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC;
          }
        }
      }
      else if (literal_p->type & SCANNER_LITERAL_IS_ARG)
      {
        type = SCANNER_STREAM_TYPE_ARG;

        if (literal_p->type & SCANNER_LITERAL_IS_DESTRUCTURED_ARG)
        {
          type = SCANNER_STREAM_TYPE_DESTRUCTURED_ARG;
        }

        if (literal_p->type & SCANNER_LITERAL_IS_VAR)
        {
          type = (uint8_t) (type + 1);

          JERRY_ASSERT (type == SCANNER_STREAM_TYPE_ARG_VAR || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR);
        }
      }
      else if (literal_p->type & SCANNER_LITERAL_IS_LET)
      {
        if (!(literal_p->type & SCANNER_LITERAL_IS_CONST))
        {
          type = SCANNER_STREAM_TYPE_LET;

          if ((status_flags & SCANNER_LITERAL_POOL_CAN_EVAL) && (literal_p->type & SCANNER_LITERAL_NO_REG))
          {
            literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
          }
        }
#if JERRY_MODULE_SYSTEM
        else if (prev_literal_pool_p == NULL)
        {
          type = SCANNER_STREAM_TYPE_IMPORT;
        }
#endif /* JERRY_MODULE_SYSTEM */
        else
        {
          type = SCANNER_STREAM_TYPE_LOCAL;
        }
      }
      else if (literal_p->type & SCANNER_LITERAL_IS_CONST)
      {
        type = SCANNER_STREAM_TYPE_CONST;

        if ((status_flags & SCANNER_LITERAL_POOL_CAN_EVAL) && (literal_p->type & SCANNER_LITERAL_NO_REG))
        {
          literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
        }
      }

      if (literal_p->type & SCANNER_LITERAL_EARLY_CREATE)
      {
        type |= SCANNER_STREAM_NO_REG | SCANNER_STREAM_EARLY_CREATE;
      }

      if (literal_p->status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)
      {
        type |= SCANNER_STREAM_HAS_ESCAPE;
      }

      if ((literal_p->type & SCANNER_LITERAL_NO_REG)
          || (mapped_arguments && (literal_p->type & SCANNER_LITERAL_IS_ARG)))
      {
        type |= SCANNER_STREAM_NO_REG;
      }

      data_p[0] = type;
      data_p[1] = (uint8_t) literal_p->length;
      data_p += 3;

      intptr_t diff = (intptr_t) (literal_p->char_p - prev_source_p);

      if (diff >= 1 && diff <= (intptr_t) UINT8_MAX)
      {
        data_p[-1] = (uint8_t) diff;
      }
      else if (diff >= -(intptr_t) UINT8_MAX && diff <= (intptr_t) UINT16_MAX)
      {
        if (diff < 0)
        {
          diff = -diff;
        }

        data_p[-3] |= SCANNER_STREAM_UINT16_DIFF;
        data_p[-1] = (uint8_t) diff;
        data_p[0] = (uint8_t) (diff >> 8);
        data_p += 1;
      }
      else
      {
        data_p[-1] = 0;
        memcpy (data_p, &literal_p->char_p, sizeof (uintptr_t));
        data_p += sizeof (uintptr_t);
      }

      if (literal_p == last_argument_p)
      {
        *data_p++ = arguments_stream_type;
      }

      prev_source_p = literal_p->char_p + literal_p->length;
    }

    data_p[0] = SCANNER_STREAM_TYPE_END;

    JERRY_ASSERT (((uint8_t *) info_p) + compressed_size == data_p + 1);
  }

  if (!(status_flags & SCANNER_LITERAL_POOL_FUNCTION)
      && (int32_t) prev_literal_pool_p->no_declarations < no_declarations)
  {
    prev_literal_pool_p->no_declarations = (uint16_t) no_declarations;
  }

  if ((status_flags & SCANNER_LITERAL_POOL_FUNCTION) && prev_literal_pool_p != NULL)
  {
    if (prev_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IS_STRICT)
    {
      context_p->status_flags |= PARSER_IS_STRICT;
    }
    else
    {
      context_p->status_flags &= (uint32_t) ~PARSER_IS_STRICT;
    }

    if (prev_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_GENERATOR)
    {
      context_p->status_flags |= PARSER_IS_GENERATOR_FUNCTION;
    }
    else
    {
      context_p->status_flags &= (uint32_t) ~PARSER_IS_GENERATOR_FUNCTION;
    }

    if (prev_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_ASYNC)
    {
      context_p->status_flags |= PARSER_IS_ASYNC_FUNCTION;
    }
    else
    {
      context_p->status_flags &= (uint32_t) ~PARSER_IS_ASYNC_FUNCTION;
    }
  }

  scanner_context_p->active_literal_pool_p = literal_pool_p->prev_p;

  parser_list_free (&literal_pool_p->literal_pool);
  scanner_free (literal_pool_p, sizeof (scanner_literal_pool_t));
} /* scanner_pop_literal_pool */

/**
 * Filter out the arguments from a literal pool.
 */
void
scanner_filter_arguments (parser_context_t *context_p, /**< context */
                          scanner_context_t *scanner_context_p) /**< scanner context */
{
  /* Fast case: check whether all literals are arguments. */
  scanner_literal_pool_t *literal_pool_p = scanner_context_p->active_literal_pool_p;
  scanner_literal_pool_t *prev_literal_pool_p = literal_pool_p->prev_p;
  parser_list_iterator_t literal_iterator;
  lexer_lit_location_t *literal_p;
  bool can_eval = (literal_pool_p->status_flags & SCANNER_LITERAL_POOL_CAN_EVAL) != 0;
  bool has_arguments = (literal_pool_p->status_flags & SCANNER_LITERAL_POOL_NO_ARGUMENTS) == 0;

  JERRY_ASSERT (SCANNER_LITERAL_POOL_MAY_HAVE_ARGUMENTS (literal_pool_p->status_flags));

  if (JERRY_UNLIKELY (can_eval))
  {
    if (prev_literal_pool_p != NULL)
    {
      prev_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_CAN_EVAL;
    }

    literal_pool_p->status_flags &= (uint16_t) ~SCANNER_LITERAL_POOL_CAN_EVAL;
  }
  else
  {
    parser_list_iterator_init (&literal_pool_p->literal_pool, &literal_iterator);

    while (true)
    {
      literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator);

      if (literal_p == NULL)
      {
        return;
      }

      if (can_eval || (literal_p->type & SCANNER_LITERAL_EARLY_CREATE))
      {
        literal_p->type |= SCANNER_LITERAL_NO_REG | SCANNER_LITERAL_EARLY_CREATE;
      }

      uint8_t type = literal_p->type;
      const uint8_t mask =
        (SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_DESTRUCTURED_ARG | SCANNER_LITERAL_IS_ARROW_DESTRUCTURED_ARG);

      if ((type & mask) != SCANNER_LITERAL_IS_ARG)
      {
        break;
      }
    }
  }

  /* Destructured args are placed after the other arguments because of register assignments. */
  bool has_destructured_arg = false;
  scanner_literal_pool_t *new_literal_pool_p;

  new_literal_pool_p = (scanner_literal_pool_t *) scanner_malloc (context_p, sizeof (scanner_literal_pool_t));

  new_literal_pool_p->prev_p = literal_pool_p;
  scanner_context_p->active_literal_pool_p = new_literal_pool_p;

  *new_literal_pool_p = *literal_pool_p;
  parser_list_init (&new_literal_pool_p->literal_pool,
                    sizeof (lexer_lit_location_t),
                    (uint32_t) ((128 - sizeof (void *)) / sizeof (lexer_lit_location_t)));

  parser_list_iterator_init (&literal_pool_p->literal_pool, &literal_iterator);

  while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
  {
    uint8_t type = literal_p->type;

    if (type & SCANNER_LITERAL_IS_ARG)
    {
      if (can_eval || (literal_p->type & SCANNER_LITERAL_EARLY_CREATE))
      {
        type |= SCANNER_LITERAL_NO_REG | SCANNER_LITERAL_EARLY_CREATE;
        literal_p->type = type;
      }

      if (has_arguments && scanner_literal_is_arguments (literal_p))
      {
        has_arguments = false;
      }

      if (type & (SCANNER_LITERAL_IS_DESTRUCTURED_ARG | SCANNER_LITERAL_IS_ARROW_DESTRUCTURED_ARG))
      {
        has_destructured_arg = true;

        if (type & SCANNER_LITERAL_IS_DESTRUCTURED_ARG)
        {
          continue;
        }

        type &= (uint8_t) ~SCANNER_LITERAL_IS_ARROW_DESTRUCTURED_ARG;
        type |= SCANNER_LITERAL_IS_DESTRUCTURED_ARG;

        literal_p->type = type;
        continue;
      }

      lexer_lit_location_t *new_literal_p;
      new_literal_p = (lexer_lit_location_t *) parser_list_append (context_p, &new_literal_pool_p->literal_pool);
      *new_literal_p = *literal_p;
    }
    else if (has_arguments && scanner_literal_is_arguments (literal_p))
    {
      /* Arguments object is directly referenced from the function arguments */
      new_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_ARGUMENTS_IN_ARGS;

      if (type & SCANNER_LITERAL_NO_REG)
      {
        new_literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_NO_ARGUMENTS;
      }
    }
    else if (prev_literal_pool_p != NULL)
    {
      /* Propagate literal to upper level. */
      lexer_lit_location_t *literal_location_p = scanner_add_custom_literal (context_p, prev_literal_pool_p, literal_p);
      type |= SCANNER_LITERAL_NO_REG | SCANNER_LITERAL_IS_USED;
      literal_location_p->type |= type;
    }
  }

  if (has_destructured_arg)
  {
    parser_list_iterator_init (&literal_pool_p->literal_pool, &literal_iterator);

    while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
    {
      const uint8_t expected_flags = SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_DESTRUCTURED_ARG;

      if ((literal_p->type & expected_flags) == expected_flags)
      {
        lexer_lit_location_t *new_literal_p;
        new_literal_p = (lexer_lit_location_t *) parser_list_append (context_p, &new_literal_pool_p->literal_pool);
        *new_literal_p = *literal_p;
      }
    }
  }

  if (has_arguments)
  {
    /* Force the lexically stored arguments object creation */
    new_literal_pool_p->status_flags |= (SCANNER_LITERAL_POOL_ARGUMENTS_IN_ARGS | SCANNER_LITERAL_POOL_NO_ARGUMENTS);
  }

  new_literal_pool_p->prev_p = prev_literal_pool_p;

  parser_list_free (&literal_pool_p->literal_pool);
  scanner_free (literal_pool_p, sizeof (scanner_literal_pool_t));
} /* scanner_filter_arguments */

/**
 * Add any literal to the specified literal pool.
 *
 * @return pointer to the literal
 */
lexer_lit_location_t *
scanner_add_custom_literal (parser_context_t *context_p, /**< context */
                            scanner_literal_pool_t *literal_pool_p, /**< literal pool */
                            const lexer_lit_location_t *literal_location_p) /**< literal */
{
  while (true)
  {
    parser_list_iterator_t literal_iterator;
    parser_list_iterator_init (&literal_pool_p->literal_pool, &literal_iterator);
    lexer_lit_location_t *literal_p;

    const uint8_t *char_p = literal_location_p->char_p;
    prop_length_t length = literal_location_p->length;

    if (JERRY_LIKELY (!(literal_location_p->status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)))
    {
      while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
      {
        if (literal_p->length == length)
        {
          if (JERRY_LIKELY (!(literal_p->status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)))
          {
            if (memcmp (literal_p->char_p, char_p, length) == 0)
            {
              return literal_p;
            }
          }
          else if (lexer_compare_identifier_to_string (literal_p, char_p, length))
          {
            /* The non-escaped version is preferred. */
            literal_p->char_p = char_p;
            literal_p->status_flags = LEXER_LIT_LOCATION_NO_OPTS;
            return literal_p;
          }
        }
      }
    }
    else
    {
      while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
      {
        if (lexer_compare_identifiers (context_p, literal_p, literal_location_p))
        {
          return literal_p;
        }
      }
    }

    if (JERRY_UNLIKELY (literal_pool_p->status_flags & SCANNER_LITERAL_POOL_CLASS_NAME))
    {
      literal_pool_p = literal_pool_p->prev_p;
      continue;
    }

    literal_p = (lexer_lit_location_t *) parser_list_append (context_p, &literal_pool_p->literal_pool);
    *literal_p = *literal_location_p;

    literal_p->type = 0;

    return literal_p;
  }
} /* scanner_add_custom_literal */

/**
 * Add the current literal token to the current literal pool.
 *
 * @return pointer to the literal
 */
lexer_lit_location_t *
scanner_add_literal (parser_context_t *context_p, /**< context */
                     scanner_context_t *scanner_context_p) /**< scanner context */
{
  return scanner_add_custom_literal (context_p,
                                     scanner_context_p->active_literal_pool_p,
                                     &context_p->token.lit_location);
} /* scanner_add_literal */

/**
 * Add the current literal token to the current literal pool and
 * set SCANNER_LITERAL_NO_REG if it is inside a with statement.
 *
 * @return pointer to the literal
 */
void
scanner_add_reference (parser_context_t *context_p, /**< context */
                       scanner_context_t *scanner_context_p) /**< scanner context */
{
  lexer_lit_location_t *lit_location_p =
    scanner_add_custom_literal (context_p, scanner_context_p->active_literal_pool_p, &context_p->token.lit_location);
  lit_location_p->type |= SCANNER_LITERAL_IS_USED;

  if (scanner_context_p->active_literal_pool_p->status_flags & SCANNER_LITERAL_POOL_IN_WITH)
  {
    lit_location_p->type |= SCANNER_LITERAL_NO_REG;
  }

  scanner_detect_eval_call (context_p, scanner_context_p);
} /* scanner_add_reference */

/**
 * Append an argument to the literal pool. If the argument is already present, make it a "hole".
 *
 * @return newly created literal
 */
lexer_lit_location_t *
scanner_append_argument (parser_context_t *context_p, /**< context */
                         scanner_context_t *scanner_context_p) /**< scanner context */
{
  scanner_literal_pool_t *literal_pool_p = scanner_context_p->active_literal_pool_p;
  parser_list_iterator_t literal_iterator;
  parser_list_iterator_init (&literal_pool_p->literal_pool, &literal_iterator);
  lexer_lit_location_t *literal_location_p = &context_p->token.lit_location;
  lexer_lit_location_t *literal_p;

  const uint8_t *char_p = literal_location_p->char_p;
  prop_length_t length = literal_location_p->length;

  JERRY_ASSERT (SCANNER_LITERAL_POOL_MAY_HAVE_ARGUMENTS (literal_pool_p->status_flags));

  if (JERRY_LIKELY (!(context_p->token.lit_location.status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)))
  {
    while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
    {
      if (literal_p->length == length)
      {
        if (JERRY_LIKELY (!(literal_p->status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)))
        {
          if (memcmp (literal_p->char_p, char_p, length) == 0)
          {
            break;
          }
        }
        else if (lexer_compare_identifier_to_string (literal_p, char_p, length))
        {
          break;
        }
      }
    }
  }
  else
  {
    while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
    {
      if (lexer_compare_identifiers (context_p, literal_p, literal_location_p))
      {
        break;
      }
    }
  }

  uint8_t literal_type = SCANNER_LITERAL_IS_ARG;

  if (literal_p != NULL)
  {
    literal_p->length = 0;

    if (literal_p->type & SCANNER_LITERAL_IS_USED)
    {
      literal_type = SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_EARLY_CREATE;
    }
  }

  literal_p = (lexer_lit_location_t *) parser_list_append (context_p, &literal_pool_p->literal_pool);

  *literal_p = context_p->token.lit_location;
  literal_p->type = literal_type;

  return literal_p;
} /* scanner_append_argument */

/**
 * Add private identifiers to private ident pool
 */
void
scanner_add_private_identifier (parser_context_t *context_p, /**< context  */
                                scanner_private_field_flags_t opts) /**< options */
{
  scan_stack_modes_t stack_top = (scan_stack_modes_t) context_p->stack_top_uint8;
  parser_stack_pop_uint8 (context_p);
  scanner_class_info_t *class_info_p;
  parser_stack_pop (context_p, &class_info_p, sizeof (scanner_class_info_t *));

  scanner_class_private_member_t *iter = class_info_p->members;

  scanner_private_field_flags_t search_flag =
    (((unsigned int) opts & (unsigned int) SCANNER_PRIVATE_FIELD_PROPERTY) ? SCANNER_PRIVATE_FIELD_PROPERTY_GETTER_SETTER
                                             : (scanner_private_field_flags_t) ((unsigned int) opts & (unsigned int) SCANNER_PRIVATE_FIELD_GETTER_SETTER));

  while (iter != NULL)
  {
    if (lexer_compare_identifiers (context_p, &context_p->token.lit_location, &iter->loc)
        && (iter->u8_arg & search_flag))
    {
      scanner_raise_error (context_p);
    }

    iter = iter->prev_p;
  }

  scanner_class_private_member_t *p_member;
  p_member = (scanner_class_private_member_t *) scanner_malloc (context_p, sizeof (scanner_class_private_member_t));
  p_member->loc = context_p->token.lit_location;
  p_member->u8_arg = (uint8_t) opts;
  p_member->prev_p = class_info_p->members;
  class_info_p->members = p_member;

  parser_stack_push (context_p, &class_info_p, sizeof (scanner_class_info_t *));
  parser_stack_push_uint8 (context_p, (uint8_t) stack_top);
} /* scanner_add_private_identifier */

/**
 * Check whether an eval call is performed and update the status flags accordingly.
 */
void
scanner_detect_eval_call (parser_context_t *context_p, /**< context */
                          scanner_context_t *scanner_context_p) /**< scanner context */
{
  if (context_p->token.keyword_type == LEXER_KEYW_EVAL && lexer_check_next_character (context_p, LIT_CHAR_LEFT_PAREN))
  {
    scanner_context_p->active_literal_pool_p->status_flags |=
      (SCANNER_LITERAL_POOL_CAN_EVAL | SCANNER_LITERAL_POOL_HAS_SUPER_REFERENCE);
  }
} /* scanner_detect_eval_call */

/**
 * Throws an error for invalid var statements.
 */
void
scanner_detect_invalid_var (parser_context_t *context_p, /**< context */
                            scanner_context_t *scanner_context_p, /**< scanner context */
                            lexer_lit_location_t *var_literal_p) /**< var literal */
{
  if (var_literal_p->type & SCANNER_LITERAL_IS_LOCAL
      && !(var_literal_p->type & (SCANNER_LITERAL_IS_FUNC | SCANNER_LITERAL_IS_ARG))
      && (var_literal_p->type & SCANNER_LITERAL_IS_LOCAL) != SCANNER_LITERAL_IS_LOCAL)
  {
    scanner_raise_redeclaration_error (context_p);
  }

  scanner_literal_pool_t *literal_pool_p = scanner_context_p->active_literal_pool_p;

  if (!(literal_pool_p->status_flags & SCANNER_LITERAL_POOL_FUNCTION)
      && ((var_literal_p->type & SCANNER_LITERAL_IS_LOCAL_FUNC) == SCANNER_LITERAL_IS_LOCAL_FUNC))
  {
    scanner_raise_redeclaration_error (context_p);
  }

  const uint8_t *char_p = var_literal_p->char_p;
  prop_length_t length = var_literal_p->length;

  while (!(literal_pool_p->status_flags & SCANNER_LITERAL_POOL_FUNCTION))
  {
    literal_pool_p = literal_pool_p->prev_p;

    parser_list_iterator_t literal_iterator;
    parser_list_iterator_init (&literal_pool_p->literal_pool, &literal_iterator);
    lexer_lit_location_t *literal_p;

    if (JERRY_LIKELY (!(context_p->token.lit_location.status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)))
    {
      while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
      {
        if ((literal_p->type & SCANNER_LITERAL_IS_LOCAL) && !(literal_p->type & SCANNER_LITERAL_IS_ARG)
            && !((literal_p->type & SCANNER_LITERAL_IS_FUNC)
                 && (literal_pool_p->status_flags & SCANNER_LITERAL_POOL_FUNCTION))
            && (literal_p->type & SCANNER_LITERAL_IS_LOCAL) != SCANNER_LITERAL_IS_LOCAL && literal_p->length == length)
        {
          if (JERRY_LIKELY (!(literal_p->status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)))
          {
            if (memcmp (literal_p->char_p, char_p, length) == 0)
            {
              scanner_raise_redeclaration_error (context_p);
              return;
            }
          }
          else if (lexer_compare_identifier_to_string (literal_p, char_p, length))
          {
            scanner_raise_redeclaration_error (context_p);
            return;
          }
        }
      }
    }
    else
    {
      while ((literal_p = (lexer_lit_location_t *) parser_list_iterator_next (&literal_iterator)) != NULL)
      {
        if ((literal_p->type & SCANNER_LITERAL_IS_LOCAL) && !(literal_p->type & SCANNER_LITERAL_IS_ARG)
            && !((literal_p->type & SCANNER_LITERAL_IS_FUNC)
                 && (literal_pool_p->status_flags & SCANNER_LITERAL_POOL_FUNCTION))
            && (literal_p->type & SCANNER_LITERAL_IS_LOCAL) != SCANNER_LITERAL_IS_LOCAL
            && lexer_compare_identifiers (context_p, literal_p, var_literal_p))
        {
          scanner_raise_redeclaration_error (context_p);
          return;
        }
      }
    }
  }

  if (scanner_scope_find_lexical_declaration (context_p, var_literal_p))
  {
    scanner_raise_redeclaration_error (context_p);
  }
} /* scanner_detect_invalid_var */

/**
 * Throws an error for invalid let statements.
 */
void
scanner_detect_invalid_let (parser_context_t *context_p, /**< context */
                            lexer_lit_location_t *let_literal_p) /**< let literal */
{
  if (let_literal_p->type & (SCANNER_LITERAL_IS_ARG | SCANNER_LITERAL_IS_VAR | SCANNER_LITERAL_IS_LOCAL))
  {
    scanner_raise_redeclaration_error (context_p);
  }

  if (let_literal_p->type & SCANNER_LITERAL_IS_FUNC)
  {
    let_literal_p->type &= (uint8_t) ~SCANNER_LITERAL_IS_FUNC;
  }
} /* scanner_detect_invalid_let */

/**
 * Push the values required for class declaration parsing.
 *
 * @return literal reference created for class statements, NULL otherwise
 */
lexer_lit_location_t *
scanner_push_class_declaration (parser_context_t *context_p, /**< context */
                                scanner_context_t *scanner_context_p, /* scanner context */
                                uint8_t stack_mode) /**< stack mode */
{
  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_CLASS);

  const uint8_t *source_p = context_p->source_p;
  lexer_lit_location_t *literal_p = NULL;

#if JERRY_MODULE_SYSTEM
  bool is_export_default = context_p->stack_top_uint8 == SCAN_STACK_EXPORT_DEFAULT;
  JERRY_ASSERT (!is_export_default || stack_mode == SCAN_STACK_CLASS_EXPRESSION);
#endif /* JERRY_MODULE_SYSTEM */

  parser_stack_push_uint8 (context_p, stack_mode);
  lexer_next_token (context_p);

  bool class_has_name =
    (context_p->token.type == LEXER_LITERAL && context_p->token.lit_location.type == LEXER_IDENT_LITERAL);

  if (class_has_name)
  {
    literal_p = scanner_add_literal (context_p, scanner_context_p);
    scanner_context_p->active_literal_pool_p->no_declarations++;

#if JERRY_MODULE_SYSTEM
    if (is_export_default)
    {
      scanner_detect_invalid_let (context_p, literal_p);

      if (literal_p->type & SCANNER_LITERAL_IS_USED)
      {
        literal_p->type |= SCANNER_LITERAL_EARLY_CREATE;
      }

      literal_p->type |= SCANNER_LITERAL_IS_LET | SCANNER_LITERAL_NO_REG;
    }
#endif /* JERRY_MODULE_SYSTEM */
  }

  scanner_literal_pool_t *literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, 0);

  if (class_has_name)
  {
    scanner_add_literal (context_p, scanner_context_p);
    scanner_context_p->active_literal_pool_p->no_declarations++;
  }
#if JERRY_MODULE_SYSTEM
  else if (is_export_default)
  {
    lexer_lit_location_t *name_literal_p;
    name_literal_p =
      scanner_add_custom_literal (context_p, scanner_context_p->active_literal_pool_p->prev_p, &lexer_default_literal);

    name_literal_p->type |= SCANNER_LITERAL_IS_LET | SCANNER_LITERAL_NO_REG;
    scanner_context_p->active_literal_pool_p->no_declarations++;
  }
#endif /* JERRY_MODULE_SYSTEM */

  literal_pool_p->source_p = source_p;
  literal_pool_p->status_flags |= SCANNER_LITERAL_POOL_CLASS_NAME;

  const uint8_t *class_source_p = scanner_context_p->active_literal_pool_p->source_p;
  scanner_class_info_t *class_info_p =
    (scanner_class_info_t *) scanner_insert_info (context_p, class_source_p, sizeof (scanner_class_info_t));

  class_info_p->info.type = SCANNER_TYPE_CLASS_CONSTRUCTOR;
  class_info_p->members = NULL;
  class_info_p->info.u8_arg = SCANNER_CONSTRUCTOR_IMPLICIT;

  parser_stack_push (context_p, &class_info_p, sizeof (scanner_class_info_t *));
  parser_stack_push_uint8 (context_p, SCAN_STACK_IMPLICIT_CLASS_CONSTRUCTOR);
  scanner_context_p->mode = SCAN_MODE_CLASS_DECLARATION;

  return literal_p;
} /* scanner_push_class_declaration */

/**
 * Push the start of a class field initializer.
 */
void
scanner_push_class_field_initializer (parser_context_t *context_p, /**< context */
                                      scanner_context_t *scanner_context_p) /* scanner context */
{
  scanner_source_start_t source_start;
  source_start.source_p = context_p->source_p;

  parser_stack_push (context_p, &source_start, sizeof (scanner_source_start_t));
  parser_stack_push_uint8 (context_p, SCAN_STACK_CLASS_FIELD_INITIALIZER);

  scanner_literal_pool_t *literal_pool_p;
  literal_pool_p = scanner_push_literal_pool (context_p, scanner_context_p, SCANNER_LITERAL_POOL_CLASS_FIELD);
  literal_pool_p->source_p = context_p->source_p;

  scanner_context_p->mode = SCAN_MODE_PRIMARY_EXPRESSION;
} /* scanner_push_class_field_initializer */

/**
 * Push the values required for destructuring assignment or binding parsing.
 */
void
scanner_push_destructuring_pattern (parser_context_t *context_p, /**< context */
                                    scanner_context_t *scanner_context_p, /**< scanner context */
                                    uint8_t binding_type, /**< type of destructuring binding pattern */
                                    bool is_nested) /**< nested declaration */
{
  JERRY_ASSERT (binding_type != SCANNER_BINDING_NONE || !is_nested);

  scanner_source_start_t source_start;
  source_start.source_p = context_p->source_p;

  parser_stack_push (context_p, &source_start, sizeof (scanner_source_start_t));
  parser_stack_push_uint8 (context_p, scanner_context_p->binding_type);
  scanner_context_p->binding_type = binding_type;

  if (SCANNER_NEEDS_BINDING_LIST (binding_type))
  {
    scanner_binding_list_t *binding_list_p;
    binding_list_p = (scanner_binding_list_t *) scanner_malloc (context_p, sizeof (scanner_binding_list_t));

    binding_list_p->prev_p = scanner_context_p->active_binding_list_p;
    binding_list_p->items_p = NULL;
    binding_list_p->is_nested = is_nested;

    scanner_context_p->active_binding_list_p = binding_list_p;
  }
} /* scanner_push_destructuring_pattern */

/**
 * Pop binding list.
 */
void
scanner_pop_binding_list (scanner_context_t *scanner_context_p) /**< scanner context */
{
  scanner_binding_list_t *binding_list_p = scanner_context_p->active_binding_list_p;
  JERRY_ASSERT (binding_list_p != NULL);

  scanner_binding_item_t *item_p = binding_list_p->items_p;
  scanner_binding_list_t *prev_binding_list_p = binding_list_p->prev_p;
  bool is_nested = binding_list_p->is_nested;

  scanner_free (binding_list_p, sizeof (scanner_binding_list_t));
  scanner_context_p->active_binding_list_p = prev_binding_list_p;

  if (!is_nested)
  {
    while (item_p != NULL)
    {
      scanner_binding_item_t *next_p = item_p->next_p;

      JERRY_ASSERT (item_p->literal_p->type & (SCANNER_LITERAL_IS_LOCAL | SCANNER_LITERAL_IS_ARG));

      scanner_free (item_p, sizeof (scanner_binding_item_t));
      item_p = next_p;
    }
    return;
  }

  JERRY_ASSERT (prev_binding_list_p != NULL);

  while (item_p != NULL)
  {
    scanner_binding_item_t *next_p = item_p->next_p;

    item_p->next_p = prev_binding_list_p->items_p;
    prev_binding_list_p->items_p = item_p;

    item_p = next_p;
  }
} /* scanner_pop_binding_list */

/**
 * Append a hole into the literal pool.
 */
void
scanner_append_hole (parser_context_t *context_p, scanner_context_t *scanner_context_p)
{
  scanner_literal_pool_t *literal_pool_p = scanner_context_p->active_literal_pool_p;

  lexer_lit_location_t *literal_p;
  literal_p = (lexer_lit_location_t *) parser_list_append (context_p, &literal_pool_p->literal_pool);

  literal_p->char_p = NULL;
  literal_p->length = 0;
  literal_p->type = SCANNER_LITERAL_IS_ARG;
  literal_p->status_flags = LEXER_LIT_LOCATION_NO_OPTS;
} /* scanner_append_hole */

/**
 * Reverse the scanner info chain after the scanning is completed.
 */
void
scanner_reverse_info_list (parser_context_t *context_p) /**< context */
{
  scanner_info_t *scanner_info_p = context_p->next_scanner_info_p;
  scanner_info_t *last_scanner_info_p = NULL;

  if (scanner_info_p->type == SCANNER_TYPE_END)
  {
    return;
  }

  do
  {
    scanner_info_t *next_scanner_info_p = scanner_info_p->next_p;
    scanner_info_p->next_p = last_scanner_info_p;

    last_scanner_info_p = scanner_info_p;
    scanner_info_p = next_scanner_info_p;
  } while (scanner_info_p->type != SCANNER_TYPE_END);

  context_p->next_scanner_info_p->next_p = scanner_info_p;
  context_p->next_scanner_info_p = last_scanner_info_p;
} /* scanner_reverse_info_list */

/**
 * Release unused scanner info blocks.
 * This should happen only if an error is occurred.
 */
void
scanner_cleanup (parser_context_t *context_p) /**< context */
{
  if (context_p->skipped_scanner_info_p != NULL)
  {
    context_p->skipped_scanner_info_end_p->next_p = context_p->next_scanner_info_p;
    context_p->next_scanner_info_p = context_p->skipped_scanner_info_p;
    context_p->skipped_scanner_info_p = NULL;
  }

  scanner_info_t *scanner_info_p = context_p->next_scanner_info_p;

  while (scanner_info_p != NULL)
  {
    scanner_info_t *next_scanner_info_p = scanner_info_p->next_p;

    size_t size = sizeof (scanner_info_t);

    switch (scanner_info_p->type)
    {
      case SCANNER_TYPE_END:
      {
        scanner_info_p = context_p->active_scanner_info_p;
        continue;
      }
      case SCANNER_TYPE_FUNCTION:
      case SCANNER_TYPE_BLOCK:
      {
        size = scanner_get_stream_size (scanner_info_p, sizeof (scanner_info_t));
        break;
      }
      case SCANNER_TYPE_WHILE:
      case SCANNER_TYPE_FOR_IN:
      case SCANNER_TYPE_FOR_OF:
      case SCANNER_TYPE_CASE:
      case SCANNER_TYPE_INITIALIZER:
      case SCANNER_TYPE_CLASS_FIELD_INITIALIZER_END:
      case SCANNER_TYPE_CLASS_STATIC_BLOCK_END:
      {
        size = sizeof (scanner_location_info_t);
        break;
      }
      case SCANNER_TYPE_FOR:
      {
        size = sizeof (scanner_for_info_t);
        break;
      }
      case SCANNER_TYPE_SWITCH:
      {
        scanner_release_switch_cases (((scanner_switch_info_t *) scanner_info_p)->case_p);
        size = sizeof (scanner_switch_info_t);
        break;
      }
      case SCANNER_TYPE_CLASS_CONSTRUCTOR:
      {
        scanner_release_private_fields (((scanner_class_info_t *) scanner_info_p)->members);
        size = sizeof (scanner_class_info_t);
        break;
      }
      default:
      {
        JERRY_ASSERT (
          scanner_info_p->type == SCANNER_TYPE_END_ARGUMENTS || scanner_info_p->type == SCANNER_TYPE_LITERAL_FLAGS
          || scanner_info_p->type == SCANNER_TYPE_LET_EXPRESSION || scanner_info_p->type == SCANNER_TYPE_ERR_REDECLARED
          || scanner_info_p->type == SCANNER_TYPE_ERR_ASYNC_FUNCTION
          || scanner_info_p->type == SCANNER_TYPE_EXPORT_MODULE_SPECIFIER);
        break;
      }
    }

    scanner_free (scanner_info_p, size);
    scanner_info_p = next_scanner_info_p;
  }

  context_p->next_scanner_info_p = NULL;
  context_p->active_scanner_info_p = NULL;
} /* scanner_cleanup */

/**
 * Checks whether a context needs to be created for a block.
 *
 * @return true - if context is needed,
 *         false - otherwise
 */
bool
scanner_is_context_needed (parser_context_t *context_p, /**< context */
                           parser_check_context_type_t check_type) /**< context type */
{
  scanner_info_t *info_p = context_p->next_scanner_info_p;
  const uint8_t *data_p = (const uint8_t *) (info_p + 1);

  JERRY_UNUSED (check_type);

  JERRY_ASSERT ((check_type == PARSER_CHECK_BLOCK_CONTEXT ? info_p->type == SCANNER_TYPE_BLOCK
                                                          : info_p->type == SCANNER_TYPE_FUNCTION));

  uint32_t scope_stack_reg_top =
    (check_type != PARSER_CHECK_GLOBAL_CONTEXT ? context_p->scope_stack_reg_top : 1); /* block result */

  while (data_p[0] != SCANNER_STREAM_TYPE_END)
  {
    uint8_t data = data_p[0];
    uint32_t type = data & SCANNER_STREAM_TYPE_MASK;

    if (JERRY_UNLIKELY (check_type == PARSER_CHECK_FUNCTION_CONTEXT))
    {
      if (JERRY_UNLIKELY (type == SCANNER_STREAM_TYPE_HOLE))
      {
        data_p++;
        continue;
      }

      if (JERRY_UNLIKELY (SCANNER_STREAM_TYPE_IS_ARGUMENTS (type)))
      {
        if ((data & SCANNER_STREAM_NO_REG) || scope_stack_reg_top >= PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
        {
          return true;
        }

        scope_stack_reg_top++;
        data_p++;
        continue;
      }
    }

#ifndef JERRY_NDEBUG
    if (check_type == PARSER_CHECK_BLOCK_CONTEXT)
    {
      JERRY_ASSERT (type == SCANNER_STREAM_TYPE_VAR || type == SCANNER_STREAM_TYPE_LET
                    || type == SCANNER_STREAM_TYPE_CONST || type == SCANNER_STREAM_TYPE_LOCAL
                    || type == SCANNER_STREAM_TYPE_FUNC);
    }
    else if (check_type == PARSER_CHECK_GLOBAL_CONTEXT)
    {
#if JERRY_MODULE_SYSTEM
      const bool is_import = (type == SCANNER_STREAM_TYPE_IMPORT);
#else /* !JERRY_MODULE_SYSTEM */
      const bool is_import = true;
#endif /* JERRY_MODULE_SYSTEM */

      /* FIXME: a private declarative lexical environment should always be present
       * for modules. Remove SCANNER_STREAM_TYPE_IMPORT after it is implemented. */
      JERRY_ASSERT (type == SCANNER_STREAM_TYPE_VAR || type == SCANNER_STREAM_TYPE_LET
                    || type == SCANNER_STREAM_TYPE_CONST || type == SCANNER_STREAM_TYPE_FUNC || is_import);

      /* Only let/const can be stored in registers */
      JERRY_ASSERT ((data & SCANNER_STREAM_NO_REG)
                    || (type == SCANNER_STREAM_TYPE_FUNC && (context_p->global_status_flags & ECMA_PARSE_DIRECT_EVAL))
                    || type == SCANNER_STREAM_TYPE_LET || type == SCANNER_STREAM_TYPE_CONST);
    }
    else
    {
      JERRY_ASSERT (check_type == PARSER_CHECK_FUNCTION_CONTEXT);

      JERRY_ASSERT (type == SCANNER_STREAM_TYPE_VAR || type == SCANNER_STREAM_TYPE_LET
                    || type == SCANNER_STREAM_TYPE_CONST || type == SCANNER_STREAM_TYPE_LOCAL
                    || type == SCANNER_STREAM_TYPE_ARG || type == SCANNER_STREAM_TYPE_ARG_VAR
                    || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR
                    || type == SCANNER_STREAM_TYPE_ARG_FUNC || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC
                    || type == SCANNER_STREAM_TYPE_FUNC);
    }
#endif /* !JERRY_NDEBUG */
    if (!(data & SCANNER_STREAM_UINT16_DIFF))
    {
      if (data_p[2] != 0)
      {
        data_p += 2 + 1;
      }
      else
      {
        data_p += 2 + 1 + sizeof (const uint8_t *);
      }
    }
    else
    {
      data_p += 2 + 2;
    }

#if JERRY_MODULE_SYSTEM
    const bool is_import = (type == SCANNER_STREAM_TYPE_IMPORT);
#else /* !JERRY_MODULE_SYSTEM */
    const bool is_import = false;
#endif /* JERRY_MODULE_SYSTEM */

    if (JERRY_UNLIKELY (check_type == PARSER_CHECK_GLOBAL_CONTEXT)
        && (type == SCANNER_STREAM_TYPE_VAR
            || (type == SCANNER_STREAM_TYPE_FUNC && !(context_p->global_status_flags & ECMA_PARSE_EVAL)) || is_import))
    {
      continue;
    }

    if (JERRY_UNLIKELY (check_type == PARSER_CHECK_FUNCTION_CONTEXT))
    {
      if (SCANNER_STREAM_TYPE_IS_ARG_FUNC (type) || type == SCANNER_STREAM_TYPE_ARG_VAR
          || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR)
      {
        /* The return value is true, if the variable is stored in the lexical environment
         * or all registers have already been used for function arguments. This can be
         * imprecise in the latter case, but this is a very rare corner case. A more
         * sophisticated check would require to decode the literal. */
        if ((data & SCANNER_STREAM_NO_REG) || scope_stack_reg_top >= PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
        {
          return true;
        }
        continue;
      }

      if (SCANNER_STREAM_TYPE_IS_ARG (type))
      {
        continue;
      }
    }

    if ((data & SCANNER_STREAM_NO_REG) || scope_stack_reg_top >= PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
    {
      return true;
    }

    scope_stack_reg_top++;
  }

  return false;
} /* scanner_is_context_needed */

/**
 * Try to scan/parse the ".target" part in the "new.target" expression.
 *
 * Upon exiting with "true" the current token will point to the "target"
 * literal.
 *
 * If the "target" literal is not after the "new." then a scanner/parser
 * error will be raised.
 *
 * @returns true if the ".target" part was found
 *          false if there is no "." after the new.
 */
bool
scanner_try_scan_new_target (parser_context_t *context_p) /**< parser/scanner context */
{
  JERRY_ASSERT (context_p->token.type == LEXER_KEYW_NEW);

  if (lexer_check_next_character (context_p, LIT_CHAR_DOT))
  {
    lexer_next_token (context_p);
    if (context_p->token.type != LEXER_DOT)
    {
      parser_raise_error (context_p, PARSER_ERR_INVALID_CHARACTER);
    }

    lexer_next_token (context_p);
    if (!lexer_token_is_identifier (context_p, "target", 6))
    {
      parser_raise_error (context_p, PARSER_ERR_NEW_TARGET_EXPECTED);
    }

    return true;
  }
  return false;
} /* scanner_try_scan_new_target */

/**
 * Description of "arguments" literal string.
 */
const lexer_lit_location_t lexer_arguments_literal = { (const uint8_t *) "arguments",
                                                       9,
                                                       LEXER_IDENT_LITERAL,
                                                       LEXER_LIT_LOCATION_IS_ASCII };

/**
 * Create an unused literal.
 */
static void
scanner_create_unused_literal (parser_context_t *context_p, /**< context */
                               uint8_t status_flags) /**< initial status flags */
{
  if (JERRY_UNLIKELY (context_p->literal_count >= PARSER_MAXIMUM_NUMBER_OF_LITERALS))
  {
    parser_raise_error (context_p, PARSER_ERR_LITERAL_LIMIT_REACHED);
  }

  lexer_literal_t *literal_p = (lexer_literal_t *) parser_list_append (context_p, &context_p->literal_pool);

  literal_p->type = LEXER_UNUSED_LITERAL;
  literal_p->status_flags = status_flags;

  context_p->literal_count++;
} /* scanner_create_unused_literal */

/**
 * Emit checks for redeclared bindings in the global lexical scope.
 */
void
scanner_check_variables (parser_context_t *context_p) /**< context */
{
  scanner_info_t *info_p = context_p->next_scanner_info_p;
  const uint8_t *next_data_p = (const uint8_t *) (info_p + 1);
  lexer_lit_location_t literal;

  JERRY_ASSERT (info_p->type == SCANNER_TYPE_FUNCTION);

  literal.char_p = info_p->source_p - 1;

  while (next_data_p[0] != SCANNER_STREAM_TYPE_END)
  {
    uint32_t type = next_data_p[0] & SCANNER_STREAM_TYPE_MASK;
    const uint8_t *data_p = next_data_p;

    JERRY_ASSERT (type != SCANNER_STREAM_TYPE_HOLE && !SCANNER_STREAM_TYPE_IS_ARG (type)
                  && !SCANNER_STREAM_TYPE_IS_ARG_FUNC (type));
    JERRY_ASSERT (data_p[0] & SCANNER_STREAM_NO_REG);

    if (!(data_p[0] & SCANNER_STREAM_UINT16_DIFF))
    {
      if (data_p[2] != 0)
      {
        literal.char_p += data_p[2];
        next_data_p += 2 + 1;
      }
      else
      {
        memcpy (&literal.char_p, data_p + 2 + 1, sizeof (uintptr_t));
        next_data_p += 2 + 1 + sizeof (uintptr_t);
      }
    }
    else
    {
      int32_t diff = ((int32_t) data_p[2]) | ((int32_t) data_p[3]) << 8;

      if (diff <= (intptr_t) UINT8_MAX)
      {
        diff = -diff;
      }

      literal.char_p += diff;
      next_data_p += 2 + 2;
    }

    literal.length = data_p[1];
    literal.type = LEXER_IDENT_LITERAL;
    literal.status_flags =
      ((data_p[0] & SCANNER_STREAM_HAS_ESCAPE) ? LEXER_LIT_LOCATION_HAS_ESCAPE : LEXER_LIT_LOCATION_NO_OPTS);

    lexer_construct_literal_object (context_p, &literal, LEXER_NEW_IDENT_LITERAL);
    literal.char_p += data_p[1];

#if JERRY_MODULE_SYSTEM
    if (type == SCANNER_STREAM_TYPE_IMPORT)
    {
      continue;
    }
#endif /* JERRY_MODULE_SYSTEM */

    context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_USED;

    uint16_t opcode;
    if (type == SCANNER_STREAM_TYPE_VAR || type == SCANNER_STREAM_TYPE_FUNC)
    {
      opcode = CBC_CHECK_VAR;
    }
    else
    {
      opcode = CBC_CHECK_LET;
    }

    parser_emit_cbc_literal (context_p, opcode, context_p->lit_object.index);
  }

  parser_flush_cbc (context_p);
} /* scanner_check_variables */

/**
 * Create and/or initialize var/let/const/function/etc. variables.
 */
void
scanner_create_variables (parser_context_t *context_p, /**< context */
                          uint32_t option_flags) /**< combination of scanner_create_variables_flags_t bits */
{
  scanner_info_t *info_p = context_p->next_scanner_info_p;
  const uint8_t *next_data_p = (const uint8_t *) (info_p + 1);
  uint8_t info_type = info_p->type;
  uint8_t info_u8_arg = info_p->u8_arg;
  lexer_lit_location_t literal;
  parser_scope_stack_t *scope_stack_p;
  parser_scope_stack_t *scope_stack_end_p;

  JERRY_ASSERT (info_type == SCANNER_TYPE_FUNCTION || info_type == SCANNER_TYPE_BLOCK);
  JERRY_ASSERT (!(option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_ARGS)
                || !(option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_BODY));
  JERRY_ASSERT (info_type == SCANNER_TYPE_FUNCTION
                || !(option_flags & (SCANNER_CREATE_VARS_IS_FUNCTION_ARGS | SCANNER_CREATE_VARS_IS_FUNCTION_BODY)));

  uint32_t scope_stack_reg_top = context_p->scope_stack_reg_top;

  if (info_type == SCANNER_TYPE_FUNCTION && !(option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_BODY))
  {
    JERRY_ASSERT (context_p->scope_stack_p == NULL);

    size_t stack_size = info_p->u16_arg * sizeof (parser_scope_stack_t);
    context_p->scope_stack_size = info_p->u16_arg;

    scope_stack_p = NULL;

    if (stack_size > 0)
    {
      scope_stack_p = (parser_scope_stack_t *) parser_malloc (context_p, stack_size);
    }

    context_p->scope_stack_p = scope_stack_p;
    scope_stack_end_p = scope_stack_p + context_p->scope_stack_size;

    if (option_flags & (SCANNER_CREATE_VARS_IS_SCRIPT | SCANNER_CREATE_VARS_IS_MODULE))
    {
      scope_stack_reg_top++; /* block result */
    }
  }
  else
  {
    JERRY_ASSERT (context_p->scope_stack_p != NULL || context_p->scope_stack_size == 0);

    scope_stack_p = context_p->scope_stack_p;
    scope_stack_end_p = scope_stack_p + context_p->scope_stack_size;
    scope_stack_p += context_p->scope_stack_top;
  }

  literal.char_p = info_p->source_p - 1;

  while (next_data_p[0] != SCANNER_STREAM_TYPE_END)
  {
    uint32_t type = next_data_p[0] & SCANNER_STREAM_TYPE_MASK;
    const uint8_t *data_p = next_data_p;

    JERRY_ASSERT ((option_flags & (SCANNER_CREATE_VARS_IS_FUNCTION_BODY | SCANNER_CREATE_VARS_IS_FUNCTION_ARGS))
                  || (type != SCANNER_STREAM_TYPE_HOLE && !SCANNER_STREAM_TYPE_IS_ARG (type)
                      && !SCANNER_STREAM_TYPE_IS_ARG_FUNC (type)));

#if JERRY_MODULE_SYSTEM
    JERRY_ASSERT (type != SCANNER_STREAM_TYPE_IMPORT || (data_p[0] & SCANNER_STREAM_NO_REG));
#endif /* JERRY_MODULE_SYSTEM */

    if (JERRY_UNLIKELY (type == SCANNER_STREAM_TYPE_HOLE))
    {
      JERRY_ASSERT (info_type == SCANNER_TYPE_FUNCTION);
      next_data_p++;

      if (option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_BODY)
      {
        continue;
      }

      uint8_t mask = SCANNER_FUNCTION_ARGUMENTS_NEEDED | SCANNER_FUNCTION_HAS_COMPLEX_ARGUMENT;

      if (!(context_p->status_flags & PARSER_IS_STRICT) && (info_u8_arg & mask) == SCANNER_FUNCTION_ARGUMENTS_NEEDED)
      {
        scanner_create_unused_literal (context_p, LEXER_FLAG_FUNCTION_ARGUMENT);
      }

      if (scope_stack_reg_top < PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
      {
        scope_stack_reg_top++;
      }
      continue;
    }

    if (JERRY_UNLIKELY (SCANNER_STREAM_TYPE_IS_ARGUMENTS (type)))
    {
      JERRY_ASSERT (info_type == SCANNER_TYPE_FUNCTION);
      next_data_p++;

      if (option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_BODY)
      {
        continue;
      }

      context_p->status_flags |= PARSER_ARGUMENTS_NEEDED;

      if (JERRY_UNLIKELY (scope_stack_p >= scope_stack_end_p))
      {
        JERRY_ASSERT (context_p->scope_stack_size == PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK);
        parser_raise_error (context_p, PARSER_ERR_SCOPE_STACK_LIMIT_REACHED);
      }

      lexer_construct_literal_object (context_p, &lexer_arguments_literal, LEXER_NEW_IDENT_LITERAL);
      scope_stack_p->map_from = context_p->lit_object.index;

      uint16_t map_to;

      if (!(data_p[0] & SCANNER_STREAM_NO_REG) && scope_stack_reg_top < PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
      {
        map_to = (uint16_t) (PARSER_REGISTER_START + scope_stack_reg_top);

        scope_stack_p->map_to = (uint16_t) (scope_stack_reg_top + 1);
        scope_stack_reg_top++;
      }
      else
      {
        context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_USED;
        map_to = context_p->lit_object.index;

        context_p->status_flags |= PARSER_LEXICAL_ENV_NEEDED;

        if (data_p[0] & SCANNER_STREAM_LOCAL_ARGUMENTS)
        {
          context_p->status_flags |= PARSER_LEXICAL_BLOCK_NEEDED;
        }

        scope_stack_p->map_to = 0;
      }

      scope_stack_p++;

#if JERRY_PARSER_DUMP_BYTE_CODE
      context_p->scope_stack_top = (uint16_t) (scope_stack_p - context_p->scope_stack_p);
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

      parser_emit_cbc_ext_literal (context_p, CBC_EXT_CREATE_ARGUMENTS, map_to);

      if (type == SCANNER_STREAM_TYPE_ARGUMENTS_FUNC)
      {
        if (JERRY_UNLIKELY (scope_stack_p >= scope_stack_end_p))
        {
          JERRY_ASSERT (context_p->scope_stack_size == PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK);
          parser_raise_error (context_p, PARSER_ERR_SCOPE_STACK_LIMIT_REACHED);
        }

        scope_stack_p->map_from = PARSER_SCOPE_STACK_FUNC;
        scope_stack_p->map_to = context_p->literal_count;
        scope_stack_p++;

        scanner_create_unused_literal (context_p, 0);
      }

      if (option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_ARGS)
      {
        break;
      }
      continue;
    }

    JERRY_ASSERT (context_p->scope_stack_size != 0);

    if (!(data_p[0] & SCANNER_STREAM_UINT16_DIFF))
    {
      if (data_p[2] != 0)
      {
        literal.char_p += data_p[2];
        next_data_p += 2 + 1;
      }
      else
      {
        memcpy (&literal.char_p, data_p + 2 + 1, sizeof (uintptr_t));
        next_data_p += 2 + 1 + sizeof (uintptr_t);
      }
    }
    else
    {
      int32_t diff = ((int32_t) data_p[2]) | ((int32_t) data_p[3]) << 8;

      if (diff <= (intptr_t) UINT8_MAX)
      {
        diff = -diff;
      }

      literal.char_p += diff;
      next_data_p += 2 + 2;
    }

    if (SCANNER_STREAM_TYPE_IS_ARG (type))
    {
      if (option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_BODY)
      {
        if ((context_p->status_flags & PARSER_LEXICAL_BLOCK_NEEDED)
            && (type == SCANNER_STREAM_TYPE_ARG_VAR || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR))
        {
          literal.length = data_p[1];
          literal.type = LEXER_IDENT_LITERAL;
          literal.status_flags =
            ((data_p[0] & SCANNER_STREAM_HAS_ESCAPE) ? LEXER_LIT_LOCATION_HAS_ESCAPE : LEXER_LIT_LOCATION_NO_OPTS);

          /* Literal must be exists. */
          lexer_construct_literal_object (context_p, &literal, LEXER_IDENT_LITERAL);

          if (context_p->lit_object.index < PARSER_REGISTER_START)
          {
            parser_emit_cbc_ext_literal_from_token (context_p, CBC_EXT_COPY_FROM_ARG);
          }
        }

        literal.char_p += data_p[1];
        continue;
      }
    }
    else if ((option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_ARGS) && !SCANNER_STREAM_TYPE_IS_ARG_FUNC (type))
    {
      /* Function arguments must come first. */
      break;
    }

    literal.length = data_p[1];
    literal.type = LEXER_IDENT_LITERAL;
    literal.status_flags =
      ((data_p[0] & SCANNER_STREAM_HAS_ESCAPE) ? LEXER_LIT_LOCATION_HAS_ESCAPE : LEXER_LIT_LOCATION_NO_OPTS);

    lexer_construct_literal_object (context_p, &literal, LEXER_NEW_IDENT_LITERAL);
    literal.char_p += data_p[1];

    if (SCANNER_STREAM_TYPE_IS_ARG_FUNC (type) && (option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_BODY))
    {
      JERRY_ASSERT (scope_stack_p >= context_p->scope_stack_p + 2);
      JERRY_ASSERT (context_p->status_flags & PARSER_IS_FUNCTION);
      JERRY_ASSERT (!(context_p->status_flags & PARSER_FUNCTION_IS_PARSING_ARGS));

      parser_scope_stack_t *function_map_p = scope_stack_p - 2;
      uint16_t literal_index = context_p->lit_object.index;

      while (literal_index != function_map_p->map_from)
      {
        function_map_p--;

        JERRY_ASSERT (function_map_p >= context_p->scope_stack_p);
      }

      JERRY_ASSERT (function_map_p[1].map_from == PARSER_SCOPE_STACK_FUNC);

      cbc_opcode_t opcode = CBC_SET_VAR_FUNC;

      if (JERRY_UNLIKELY (context_p->status_flags & PARSER_LEXICAL_BLOCK_NEEDED)
          && (function_map_p[0].map_to & PARSER_SCOPE_STACK_REGISTER_MASK) == 0)
      {
        opcode = CBC_INIT_ARG_OR_FUNC;
      }

      parser_emit_cbc_literal_value (context_p,
                                     (uint16_t) opcode,
                                     function_map_p[1].map_to,
                                     scanner_decode_map_to (function_map_p));
      continue;
    }

    if (JERRY_UNLIKELY (scope_stack_p >= scope_stack_end_p))
    {
      JERRY_ASSERT (context_p->scope_stack_size == PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK);
      parser_raise_error (context_p, PARSER_ERR_SCOPE_STACK_LIMIT_REACHED);
    }

    scope_stack_p->map_from = context_p->lit_object.index;

    if (info_type == SCANNER_TYPE_FUNCTION)
    {
      if (type != SCANNER_STREAM_TYPE_LET
#if JERRY_MODULE_SYSTEM
          && type != SCANNER_STREAM_TYPE_IMPORT
#endif /* JERRY_MODULE_SYSTEM */
          && type != SCANNER_STREAM_TYPE_CONST)
      {
        context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_GLOBAL;
      }
    }

    uint16_t map_to;
    uint16_t func_init_opcode = CBC_INIT_ARG_OR_FUNC;

    if (!(data_p[0] & SCANNER_STREAM_NO_REG) && scope_stack_reg_top < PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
    {
      map_to = (uint16_t) (PARSER_REGISTER_START + scope_stack_reg_top);
      scope_stack_p->map_to = (uint16_t) (scope_stack_reg_top + 1);
      scope_stack_reg_top++;

      switch (type)
      {
        case SCANNER_STREAM_TYPE_CONST:
        {
          scope_stack_p->map_to |= PARSER_SCOPE_STACK_IS_CONST_REG;
          /* FALLTHRU */
        }
        case SCANNER_STREAM_TYPE_LET:
        case SCANNER_STREAM_TYPE_ARG:
        case SCANNER_STREAM_TYPE_ARG_VAR:
        case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG:
        case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR:
        case SCANNER_STREAM_TYPE_ARG_FUNC:
        case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC:
        {
          scope_stack_p->map_to |= PARSER_SCOPE_STACK_NO_FUNCTION_COPY;
          break;
        }
      }

      func_init_opcode = CBC_SET_VAR_FUNC;
    }
    else
    {
      context_p->lit_object.literal_p->status_flags |= LEXER_FLAG_USED;
      map_to = context_p->lit_object.index;

      uint16_t scope_stack_map_to = 0;

      if (info_type == SCANNER_TYPE_FUNCTION)
      {
        context_p->status_flags |= PARSER_LEXICAL_ENV_NEEDED;
      }

      switch (type)
      {
        case SCANNER_STREAM_TYPE_LET:
        case SCANNER_STREAM_TYPE_CONST:
        case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG:
        case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR:
        case SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC:
        {
          scope_stack_map_to |= PARSER_SCOPE_STACK_NO_FUNCTION_COPY;

          if (!(data_p[0] & SCANNER_STREAM_EARLY_CREATE))
          {
            break;
          }
          scope_stack_map_to |= PARSER_SCOPE_STACK_IS_LOCAL_CREATED;
          /* FALLTHRU */
        }
        case SCANNER_STREAM_TYPE_LOCAL:
        case SCANNER_STREAM_TYPE_VAR:
        {
#if JERRY_PARSER_DUMP_BYTE_CODE
          context_p->scope_stack_top = (uint16_t) (scope_stack_p - context_p->scope_stack_p);
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */
          uint16_t opcode;

          switch (type)
          {
            case SCANNER_STREAM_TYPE_LET:
            {
              opcode = CBC_CREATE_LET;
              break;
            }
            case SCANNER_STREAM_TYPE_CONST:
            {
              opcode = CBC_CREATE_CONST;
              break;
            }
            case SCANNER_STREAM_TYPE_VAR:
            {
              opcode = CBC_CREATE_VAR;

              if (option_flags & SCANNER_CREATE_VARS_IS_SCRIPT)
              {
                opcode = CBC_CREATE_VAR_EVAL;

                if ((context_p->global_status_flags & ECMA_PARSE_FUNCTION_CONTEXT)
                    && !(context_p->status_flags & PARSER_IS_STRICT))
                {
                  opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_CREATE_VAR_EVAL);
                }
              }
              break;
            }
            default:
            {
              JERRY_ASSERT (type == SCANNER_STREAM_TYPE_LOCAL || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG
                            || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_VAR
                            || type == SCANNER_STREAM_TYPE_DESTRUCTURED_ARG_FUNC);

              opcode = CBC_CREATE_LOCAL;
              break;
            }
          }

          parser_emit_cbc_literal (context_p, opcode, map_to);
          break;
        }
        case SCANNER_STREAM_TYPE_ARG:
        case SCANNER_STREAM_TYPE_ARG_VAR:
        case SCANNER_STREAM_TYPE_ARG_FUNC:
        {
#if JERRY_PARSER_DUMP_BYTE_CODE
          context_p->scope_stack_top = (uint16_t) (scope_stack_p - context_p->scope_stack_p);
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

          scope_stack_map_to |= PARSER_SCOPE_STACK_NO_FUNCTION_COPY;

          /* Argument initializers of functions with simple arguments (e.g. function f(a,b,a) {}) are
           * generated here. The other initializers are handled by parser_parse_function_arguments(). */
          if (!(info_u8_arg & SCANNER_FUNCTION_HAS_COMPLEX_ARGUMENT))
          {
            parser_emit_cbc_literal_value (context_p,
                                           CBC_INIT_ARG_OR_FUNC,
                                           (uint16_t) (PARSER_REGISTER_START + scope_stack_reg_top),
                                           map_to);
          }
          else if (data_p[0] & SCANNER_STREAM_EARLY_CREATE)
          {
            parser_emit_cbc_literal (context_p, CBC_CREATE_LOCAL, map_to);
            scope_stack_map_to |= PARSER_SCOPE_STACK_IS_LOCAL_CREATED;
          }

          if (scope_stack_reg_top < PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
          {
            scope_stack_reg_top++;
          }
          break;
        }
      }

      scope_stack_p->map_to = scope_stack_map_to;
    }

    scope_stack_p++;

    if (!SCANNER_STREAM_TYPE_IS_FUNCTION (type))
    {
      continue;
    }

    if (JERRY_UNLIKELY (scope_stack_p >= scope_stack_end_p))
    {
      JERRY_ASSERT (context_p->scope_stack_size == PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK);
      parser_raise_error (context_p, PARSER_ERR_SCOPE_STACK_LIMIT_REACHED);
    }

#if JERRY_PARSER_DUMP_BYTE_CODE
    context_p->scope_stack_top = (uint16_t) (scope_stack_p - context_p->scope_stack_p);
#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

    if (!SCANNER_STREAM_TYPE_IS_ARG_FUNC (type))
    {
      if (func_init_opcode == CBC_INIT_ARG_OR_FUNC && (option_flags & SCANNER_CREATE_VARS_IS_SCRIPT))
      {
        literal.char_p -= data_p[1];

        if (!scanner_scope_find_lexical_declaration (context_p, &literal))
        {
          func_init_opcode = CBC_CREATE_VAR_FUNC_EVAL;

          if (context_p->global_status_flags & ECMA_PARSE_FUNCTION_CONTEXT)
          {
            func_init_opcode = PARSER_TO_EXT_OPCODE (CBC_EXT_CREATE_VAR_FUNC_EVAL);
          }
        }
        literal.char_p += data_p[1];
      }

      parser_emit_cbc_literal_value (context_p, func_init_opcode, context_p->literal_count, map_to);
    }

    scope_stack_p->map_from = PARSER_SCOPE_STACK_FUNC;
    scope_stack_p->map_to = context_p->literal_count;
    scope_stack_p++;

    scanner_create_unused_literal (context_p, 0);
  }

  context_p->scope_stack_top = (uint16_t) (scope_stack_p - context_p->scope_stack_p);
  context_p->scope_stack_reg_top = (uint16_t) scope_stack_reg_top;

  if (info_type == SCANNER_TYPE_FUNCTION)
  {
    context_p->scope_stack_global_end = context_p->scope_stack_top;
  }

  if (context_p->register_count < scope_stack_reg_top)
  {
    context_p->register_count = (uint16_t) scope_stack_reg_top;
  }

  if (!(option_flags & SCANNER_CREATE_VARS_IS_FUNCTION_ARGS))
  {
    scanner_release_next (context_p, (size_t) (next_data_p + 1 - ((const uint8_t *) info_p)));
  }
  parser_flush_cbc (context_p);
} /* scanner_create_variables */

/**
 * Get location from context.
 */
void
scanner_get_location (scanner_location_t *location_p, /**< location */
                      parser_context_t *context_p) /**< context */
{
  location_p->source_p = context_p->source_p;
  location_p->line = context_p->line;
  location_p->column = context_p->column;
} /* scanner_get_location */

/**
 * Set context location.
 */
void
scanner_set_location (parser_context_t *context_p, /**< context */
                      scanner_location_t *location_p) /**< location */
{
  context_p->source_p = location_p->source_p;
  context_p->line = location_p->line;
  context_p->column = location_p->column;
} /* scanner_set_location */

/**
 * Get the real map_to value.
 */
uint16_t
scanner_decode_map_to (parser_scope_stack_t *stack_item_p) /**< scope stack item */
{
  JERRY_ASSERT (stack_item_p->map_from != PARSER_SCOPE_STACK_FUNC);

  uint16_t value = (stack_item_p->map_to & PARSER_SCOPE_STACK_REGISTER_MASK);
  return (value == 0) ? stack_item_p->map_from : (uint16_t) (value + (PARSER_REGISTER_START - 1));
} /* scanner_decode_map_to */

/**
 * Find the given literal index in the scope stack
 * and save it the constant literal pool if the literal is register stored
 *
 * @return given literal index - if literal corresponds to this index is not register stored
 *         literal index on which literal index has been mapped - otherwise
 */
uint16_t
scanner_save_literal (parser_context_t *context_p, /**< context */
                      uint16_t literal_index) /**< literal index */
{
  if (literal_index >= PARSER_REGISTER_START)
  {
    literal_index = (uint16_t) (literal_index - (PARSER_REGISTER_START - 1));

    parser_scope_stack_t *scope_stack_p = context_p->scope_stack_p + context_p->scope_stack_top;

    do
    {
      /* Registers must be found in the scope stack. */
      JERRY_ASSERT (scope_stack_p > context_p->scope_stack_p);
      scope_stack_p--;
    } while (scope_stack_p->map_from == PARSER_SCOPE_STACK_FUNC
             || literal_index != (scope_stack_p->map_to & PARSER_SCOPE_STACK_REGISTER_MASK));

    literal_index = scope_stack_p->map_from;
    PARSER_GET_LITERAL (literal_index)->status_flags |= LEXER_FLAG_USED;
  }

  return literal_index;
} /* scanner_save_literal */

/**
 * Checks whether the literal is a const in the current scope.
 *
 * @return true if the literal is a const, false otherwise
 */
bool
scanner_literal_is_const_reg (parser_context_t *context_p, /**< context */
                              uint16_t literal_index) /**< literal index */
{
  if (literal_index < PARSER_REGISTER_START)
  {
    /* Re-assignment of non-register const bindings are detected elsewhere. */
    return false;
  }

  parser_scope_stack_t *scope_stack_p = context_p->scope_stack_p + context_p->scope_stack_top;

  literal_index = (uint16_t) (literal_index - (PARSER_REGISTER_START - 1));

  do
  {
    /* Registers must be found in the scope stack. */
    JERRY_ASSERT (scope_stack_p > context_p->scope_stack_p);
    scope_stack_p--;
  } while (scope_stack_p->map_from == PARSER_SCOPE_STACK_FUNC
           || literal_index != (scope_stack_p->map_to & PARSER_SCOPE_STACK_REGISTER_MASK));

  return (scope_stack_p->map_to & PARSER_SCOPE_STACK_IS_CONST_REG) != 0;
} /* scanner_literal_is_const_reg */

/**
 * Checks whether the literal is created before.
 *
 * @return true if the literal is created before, false otherwise
 */
bool
scanner_literal_is_created (parser_context_t *context_p, /**< context */
                            uint16_t literal_index) /**< literal index */
{
  JERRY_ASSERT (literal_index < PARSER_REGISTER_START);

  parser_scope_stack_t *scope_stack_p = context_p->scope_stack_p + context_p->scope_stack_top;

  do
  {
    /* These literals must be found in the scope stack. */
    JERRY_ASSERT (scope_stack_p > context_p->scope_stack_p);
    scope_stack_p--;
  } while (literal_index != scope_stack_p->map_from);

  JERRY_ASSERT ((scope_stack_p->map_to & PARSER_SCOPE_STACK_REGISTER_MASK) == 0);

  return (scope_stack_p->map_to & PARSER_SCOPE_STACK_IS_LOCAL_CREATED) != 0;
} /* scanner_literal_is_created */

/**
 * Checks whether the literal exists.
 *
 * @return true if the literal exists, false otherwise
 */
bool
scanner_literal_exists (parser_context_t *context_p, /**< context */
                        uint16_t literal_index) /**< literal index */
{
  JERRY_ASSERT (literal_index < PARSER_REGISTER_START);

  parser_scope_stack_t *scope_stack_p = context_p->scope_stack_p + context_p->scope_stack_top;

  while (scope_stack_p-- > context_p->scope_stack_p)
  {
    if (scope_stack_p->map_from != PARSER_SCOPE_STACK_FUNC && scanner_decode_map_to (scope_stack_p) == literal_index)
    {
      return true;
    }
  }

  return false;
} /* scanner_literal_exists */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_PARSER */
