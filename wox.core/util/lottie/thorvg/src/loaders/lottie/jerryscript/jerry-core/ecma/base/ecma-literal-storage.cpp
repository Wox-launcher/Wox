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

#include "ecma-literal-storage.h"

#include "ecma-alloc.h"
#include "ecma-big-uint.h"
#include "ecma-bigint.h"
#include "ecma-helpers.h"

#include "jcontext.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmalitstorage Literal storage
 * @{
 */

/**
 * Free symbol list
 */
static void
ecma_free_symbol_list (jmem_cpointer_t symbol_list_cp) /**< symbol list */
{
  while (symbol_list_cp != JMEM_CP_NULL)
  {
    ecma_lit_storage_item_t *symbol_list_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_lit_storage_item_t, symbol_list_cp);

    for (int i = 0; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
    {
      if (symbol_list_p->values[i] != JMEM_CP_NULL)
      {
        ecma_string_t *string_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_string_t, symbol_list_p->values[i]);

        JERRY_ASSERT (ECMA_STRING_IS_REF_EQUALS_TO_ONE (string_p));
        ecma_deref_ecma_string (string_p);
      }
    }

    jmem_cpointer_t next_item_cp = symbol_list_p->next_cp;
    jmem_pools_free (symbol_list_p, sizeof (ecma_lit_storage_item_t));
    symbol_list_cp = next_item_cp;
  }
} /* ecma_free_symbol_list */

/**
 * Free string list
 */
static void
ecma_free_string_list (jmem_cpointer_t string_list_cp) /**< string list */
{
  while (string_list_cp != JMEM_CP_NULL)
  {
    ecma_lit_storage_item_t *string_list_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_lit_storage_item_t, string_list_cp);

    for (int i = 0; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
    {
      if (string_list_p->values[i] != JMEM_CP_NULL)
      {
        ecma_string_t *string_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_string_t, string_list_p->values[i]);

        JERRY_ASSERT (ECMA_STRING_IS_REF_EQUALS_TO_ONE (string_p));
        ecma_destroy_ecma_string (string_p);
      }
    }

    jmem_cpointer_t next_item_cp = string_list_p->next_cp;
    jmem_pools_free (string_list_p, sizeof (ecma_lit_storage_item_t));
    string_list_cp = next_item_cp;
  }
} /* ecma_free_string_list */

/**
 * Free number list
 */
static void
ecma_free_number_list (jmem_cpointer_t number_list_cp) /**< number list */
{
  while (number_list_cp != JMEM_CP_NULL)
  {
    ecma_lit_storage_item_t *number_list_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_lit_storage_item_t, number_list_cp);

    for (int i = 0; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
    {
      if (number_list_p->values[i] != JMEM_CP_NULL)
      {
        ecma_dealloc_number (JMEM_CP_GET_NON_NULL_POINTER (ecma_number_t, number_list_p->values[i]));
      }
    }

    jmem_cpointer_t next_item_cp = number_list_p->next_cp;
    jmem_pools_free (number_list_p, sizeof (ecma_lit_storage_item_t));
    number_list_cp = next_item_cp;
  }
} /* ecma_free_number_list */

#if JERRY_BUILTIN_BIGINT

/**
 * Free bigint list
 */
static void
ecma_free_bigint_list (jmem_cpointer_t bigint_list_cp) /**< bigint list */
{
  while (bigint_list_cp != JMEM_CP_NULL)
  {
    ecma_lit_storage_item_t *bigint_list_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_lit_storage_item_t, bigint_list_cp);

    for (int i = 0; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
    {
      if (bigint_list_p->values[i] != JMEM_CP_NULL)
      {
        ecma_extended_primitive_t *bigint_p =
          JMEM_CP_GET_NON_NULL_POINTER (ecma_extended_primitive_t, bigint_list_p->values[i]);
        JERRY_ASSERT (ECMA_EXTENDED_PRIMITIVE_IS_REF_EQUALS_TO_ONE (bigint_p));
        ecma_deref_bigint (bigint_p);
      }
    }

    jmem_cpointer_t next_item_cp = bigint_list_p->next_cp;
    jmem_pools_free (bigint_list_p, sizeof (ecma_lit_storage_item_t));
    bigint_list_cp = next_item_cp;
  }
} /* ecma_free_bigint_list */

#endif /* JERRY_BUILTIN_BIGINT */

/**
 * Finalize literal storage
 */
void
ecma_finalize_lit_storage (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_free_symbol_list (JERRY_CONTEXT (symbol_list_first_cp));
  ecma_free_string_list (JERRY_CONTEXT (string_list_first_cp));
  ecma_free_number_list (JERRY_CONTEXT (number_list_first_cp));
#if JERRY_BUILTIN_BIGINT
  ecma_free_bigint_list (JERRY_CONTEXT (bigint_list_first_cp));
#endif /* JERRY_BUILTIN_BIGINT */
} /* ecma_finalize_lit_storage */

/**
 * Find or create a literal string.
 *
 * @return ecma_string_t compressed pointer
 */
ecma_value_t
ecma_find_or_create_literal_string (const lit_utf8_byte_t *chars_p, /**< string to be searched */
                                    lit_utf8_size_t size, /**< size of the string */
                                    bool is_ascii) /**< encode of the string */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_string_t *string_p =
    (is_ascii ? ecma_new_ecma_string_from_ascii (chars_p, size) : ecma_new_ecma_string_from_utf8 (chars_p, size));

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    return ecma_make_string_value (string_p);
  }

  jmem_cpointer_t string_list_cp = JERRY_CONTEXT (string_list_first_cp);
  jmem_cpointer_t *empty_cpointer_p = NULL;

  while (string_list_cp != JMEM_CP_NULL)
  {
    ecma_lit_storage_item_t *string_list_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_lit_storage_item_t, string_list_cp);

    for (int i = 0; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
    {
      if (string_list_p->values[i] == JMEM_CP_NULL)
      {
        if (empty_cpointer_p == NULL)
        {
          empty_cpointer_p = string_list_p->values + i;
        }
      }
      else
      {
        ecma_string_t *value_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_string_t, string_list_p->values[i]);

        if (ecma_compare_ecma_strings (string_p, value_p))
        {
          /* Return with string if found in the list. */
          ecma_deref_ecma_string (string_p);
          return ecma_make_string_value (value_p);
        }
      }
    }

    string_list_cp = string_list_p->next_cp;
  }

  ECMA_SET_STRING_AS_STATIC (string_p);
  jmem_cpointer_t result;
  JMEM_CP_SET_NON_NULL_POINTER (result, string_p);

  if (empty_cpointer_p != NULL)
  {
    *empty_cpointer_p = result;
    return ecma_make_string_value (string_p);
  }

  ecma_lit_storage_item_t *new_item_p;
  new_item_p = (ecma_lit_storage_item_t *) jmem_pools_alloc (sizeof (ecma_lit_storage_item_t));

  new_item_p->values[0] = result;
  for (int i = 1; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
  {
    new_item_p->values[i] = JMEM_CP_NULL;
  }

  new_item_p->next_cp = JERRY_CONTEXT (string_list_first_cp);
  JMEM_CP_SET_NON_NULL_POINTER (JERRY_CONTEXT (string_list_first_cp), new_item_p);

  return ecma_make_string_value (string_p);
} /* ecma_find_or_create_literal_string */

/**
 * Find or create a literal number.
 *
 * @return ecma value
 */
ecma_value_t
ecma_find_or_create_literal_number (ecma_number_t number_arg) /**< number to be searched */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_value_t num = ecma_make_number_value (number_arg);

  if (ecma_is_value_integer_number (num))
  {
    return num;
  }

  JERRY_ASSERT (ecma_is_value_float_number (num));

  jmem_cpointer_t number_list_cp = JERRY_CONTEXT (number_list_first_cp);
  jmem_cpointer_t *empty_cpointer_p = NULL;

  while (number_list_cp != JMEM_CP_NULL)
  {
    ecma_lit_storage_item_t *number_list_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_lit_storage_item_t, number_list_cp);

    for (int i = 0; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
    {
      if (number_list_p->values[i] == JMEM_CP_NULL)
      {
        if (empty_cpointer_p == NULL)
        {
          empty_cpointer_p = number_list_p->values + i;
        }
      }
      else
      {
        ecma_number_t *number_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_number_t, number_list_p->values[i]);

        if (*number_p == number_arg)
        {
          ecma_free_value (num);
          return ecma_make_float_value (number_p);
        }
      }
    }

    number_list_cp = number_list_p->next_cp;
  }

  jmem_cpointer_t result;
  JMEM_CP_SET_NON_NULL_POINTER (result, ecma_get_pointer_from_float_value (num));

  if (empty_cpointer_p != NULL)
  {
    *empty_cpointer_p = result;
    return num;
  }

  ecma_lit_storage_item_t *new_item_p;
  new_item_p = (ecma_lit_storage_item_t *) jmem_pools_alloc (sizeof (ecma_lit_storage_item_t));

  new_item_p->values[0] = result;
  for (int i = 1; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
  {
    new_item_p->values[i] = JMEM_CP_NULL;
  }

  new_item_p->next_cp = JERRY_CONTEXT (number_list_first_cp);
  JMEM_CP_SET_NON_NULL_POINTER (JERRY_CONTEXT (number_list_first_cp), new_item_p);

  return num;
} /* ecma_find_or_create_literal_number */

#if JERRY_BUILTIN_BIGINT

/**
 * Find or create a literal BigInt.
 *
 * @return BigInt value
 */
ecma_value_t
ecma_find_or_create_literal_bigint (ecma_value_t bigint) /**< bigint to be searched */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (ecma_is_value_bigint (bigint));

  if (bigint == ECMA_BIGINT_ZERO)
  {
    return bigint;
  }

  jmem_cpointer_t bigint_list_cp = JERRY_CONTEXT (bigint_list_first_cp);
  jmem_cpointer_t *empty_cpointer_p = NULL;

  while (bigint_list_cp != JMEM_CP_NULL)
  {
    ecma_lit_storage_item_t *bigint_list_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_lit_storage_item_t, bigint_list_cp);

    for (int i = 0; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
    {
      if (bigint_list_p->values[i] == JMEM_CP_NULL)
      {
        if (empty_cpointer_p == NULL)
        {
          empty_cpointer_p = bigint_list_p->values + i;
        }
      }
      else
      {
        ecma_extended_primitive_t *other_bigint_p =
          JMEM_CP_GET_NON_NULL_POINTER (ecma_extended_primitive_t, bigint_list_p->values[i]);
        ecma_value_t other_bigint = ecma_make_extended_primitive_value (other_bigint_p, ECMA_TYPE_BIGINT);

        if (ecma_bigint_is_equal_to_bigint (bigint, other_bigint))
        {
          ecma_free_value (bigint);
          return other_bigint;
        }
      }
    }

    bigint_list_cp = bigint_list_p->next_cp;
  }

  jmem_cpointer_t result;
  JMEM_CP_SET_NON_NULL_POINTER (result, ecma_get_extended_primitive_from_value (bigint));

  if (empty_cpointer_p != NULL)
  {
    *empty_cpointer_p = result;
    return bigint;
  }

  ecma_lit_storage_item_t *new_item_p;
  new_item_p = (ecma_lit_storage_item_t *) jmem_pools_alloc (sizeof (ecma_lit_storage_item_t));

  new_item_p->values[0] = result;
  for (int i = 1; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
  {
    new_item_p->values[i] = JMEM_CP_NULL;
  }

  new_item_p->next_cp = JERRY_CONTEXT (bigint_list_first_cp);
  JMEM_CP_SET_NON_NULL_POINTER (JERRY_CONTEXT (bigint_list_first_cp), new_item_p);

  return bigint;
} /* ecma_find_or_create_literal_bigint */

#endif /* JERRY_BUILTIN_BIGINT */

/**
 * Log2 of snapshot literal alignment.
 */
#define JERRY_SNAPSHOT_LITERAL_ALIGNMENT_LOG 1

/**
 * Snapshot literal alignment.
 */
#define JERRY_SNAPSHOT_LITERAL_ALIGNMENT (1u << JERRY_SNAPSHOT_LITERAL_ALIGNMENT_LOG)

/**
 * Literal offset shift.
 */
#define JERRY_SNAPSHOT_LITERAL_SHIFT (ECMA_VALUE_SHIFT + 2)

/**
 * Literal value is number.
 */
#define JERRY_SNAPSHOT_LITERAL_IS_NUMBER (1u << ECMA_VALUE_SHIFT)

#if JERRY_BUILTIN_BIGINT
/**
 * Literal value is BigInt.
 */
#define JERRY_SNAPSHOT_LITERAL_IS_BIGINT (2u << ECMA_VALUE_SHIFT)
#endif /* JERRY_BUILTIN_BIGINT */

#if JERRY_SNAPSHOT_SAVE

/**
 * Append the value at the end of the appropriate list if it is not present there.
 */
void
ecma_save_literals_append_value (ecma_value_t value, /**< value to be appended */
                                 ecma_collection_t *lit_pool_p) /**< list of known values */
{
  /* Unlike direct numbers, direct strings are converted to character literals. */
  if (!ecma_is_value_string (value)
#if JERRY_BUILTIN_BIGINT
      && (!ecma_is_value_bigint (value) || value == ECMA_BIGINT_ZERO)
#endif /* JERRY_BUILTIN_BIGINT */
      && !ecma_is_value_float_number (value))
  {
    return;
  }

  ecma_value_t *buffer_p = lit_pool_p->buffer_p;

  for (uint32_t i = 0; i < lit_pool_p->item_count; i++)
  {
    /* Strings / numbers are direct strings or stored in the literal storage.
     * Therefore direct comparison is enough to find the same strings / numbers. */
    if (buffer_p[i] == value)
    {
      return;
    }
  }

  ecma_collection_push_back (lit_pool_p, value);
} /* ecma_save_literals_append_value */

/**
 * Add names from a byte-code data to a list.
 */
void
ecma_save_literals_add_compiled_code (const ecma_compiled_code_t *compiled_code_p, /**< byte-code data */
                                      ecma_collection_t *lit_pool_p) /**< list of known values */
{
  ecma_value_t *literal_p;
  uint32_t argument_end;
  uint32_t register_end;
  uint32_t const_literal_end;
  uint32_t literal_end;

  JERRY_ASSERT (CBC_IS_FUNCTION (compiled_code_p->status_flags));

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) compiled_code_p;
    uint8_t *byte_p = (uint8_t *) compiled_code_p;

    literal_p = (ecma_value_t *) (byte_p + sizeof (cbc_uint16_arguments_t));
    register_end = args_p->register_end;
    const_literal_end = args_p->const_literal_end - register_end;
    literal_end = args_p->literal_end - register_end;
    argument_end = args_p->argument_end;
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) compiled_code_p;
    uint8_t *byte_p = (uint8_t *) compiled_code_p;

    literal_p = (ecma_value_t *) (byte_p + sizeof (cbc_uint8_arguments_t));
    register_end = args_p->register_end;
    const_literal_end = args_p->const_literal_end - register_end;
    literal_end = args_p->literal_end - register_end;
    argument_end = args_p->argument_end;
  }

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED)
  {
    for (uint32_t i = 0; i < argument_end; i++)
    {
      ecma_save_literals_append_value (literal_p[i], lit_pool_p);
    }
  }

  for (uint32_t i = 0; i < const_literal_end; i++)
  {
    ecma_save_literals_append_value (literal_p[i], lit_pool_p);
  }

  for (uint32_t i = const_literal_end; i < literal_end; i++)
  {
    ecma_compiled_code_t *bytecode_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, literal_p[i]);

    if (CBC_IS_FUNCTION (bytecode_p->status_flags) && bytecode_p != compiled_code_p)
    {
      ecma_save_literals_add_compiled_code (bytecode_p, lit_pool_p);
    }
  }

  uint8_t *byte_p = ((uint8_t *) compiled_code_p) + (((size_t) compiled_code_p->size) << JMEM_ALIGNMENT_LOG);
  literal_p = ecma_snapshot_resolve_serializable_values ((ecma_compiled_code_t *) compiled_code_p, byte_p);

  while (literal_p < (ecma_value_t *) byte_p)
  {
    ecma_save_literals_append_value (*literal_p, lit_pool_p);
    literal_p++;
  }
} /* ecma_save_literals_add_compiled_code */

/**
 * Save literals to specified snapshot buffer.
 *
 * Note:
 *      Frees 'lit_pool_p' regardless of success.
 *
 * @return true - if save was performed successfully (i.e. buffer size is sufficient),
 *         false - otherwise
 */
bool
ecma_save_literals_for_snapshot (ecma_collection_t *lit_pool_p, /**< list of known values */
                                 uint32_t *buffer_p, /**< [out] output snapshot buffer */
                                 size_t buffer_size, /**< size of the buffer */
                                 size_t *in_out_buffer_offset_p, /**< [in,out] write position in the buffer */
                                 lit_mem_to_snapshot_id_map_entry_t **out_map_p, /**< [out] map from literal identifiers
                                                                                  *   to the literal offsets
                                                                                  *   in snapshot */
                                 uint32_t *out_map_len_p) /**< [out] number of literals */
{
  if (lit_pool_p->item_count == 0)
  {
    *out_map_p = NULL;
    *out_map_len_p = 0;
  }

  uint32_t lit_table_size = 0;
  size_t max_lit_table_size = buffer_size - *in_out_buffer_offset_p;

  if (max_lit_table_size > (UINT32_MAX >> JERRY_SNAPSHOT_LITERAL_SHIFT))
  {
    max_lit_table_size = (UINT32_MAX >> JERRY_SNAPSHOT_LITERAL_SHIFT);
  }

  ecma_value_t *lit_buffer_p = lit_pool_p->buffer_p;

  /* Compute the size of the literal pool. */
  for (uint32_t i = 0; i < lit_pool_p->item_count; i++)
  {
    if (ecma_is_value_float_number (lit_buffer_p[i]))
    {
      lit_table_size += (uint32_t) sizeof (ecma_number_t);
    }
#if JERRY_BUILTIN_BIGINT
    else if (ecma_is_value_bigint (lit_buffer_p[i]))
    {
      ecma_extended_primitive_t *bigint_p = ecma_get_extended_primitive_from_value (lit_buffer_p[i]);

      lit_table_size += (uint32_t) JERRY_ALIGNUP (sizeof (uint32_t) + ECMA_BIGINT_GET_SIZE (bigint_p),
                                                  JERRY_SNAPSHOT_LITERAL_ALIGNMENT);
    }
#endif /* JERRY_BUILTIN_BIGINT */
    else
    {
      ecma_string_t *string_p = ecma_get_string_from_value (lit_buffer_p[i]);

      lit_table_size += (uint32_t) JERRY_ALIGNUP (sizeof (uint16_t) + ecma_string_get_size (string_p),
                                                  JERRY_SNAPSHOT_LITERAL_ALIGNMENT);
    }

    /* Check whether enough space is available and the maximum size is not reached. */
    if (lit_table_size > max_lit_table_size)
    {
      ecma_collection_destroy (lit_pool_p);
      return false;
    }
  }

  lit_mem_to_snapshot_id_map_entry_t *map_p;
  uint32_t total_count = lit_pool_p->item_count;

  map_p = jmem_heap_alloc_block (total_count * sizeof (lit_mem_to_snapshot_id_map_entry_t));

  /* Set return values (no error is possible from here). */
  JERRY_ASSERT ((*in_out_buffer_offset_p % sizeof (uint32_t)) == 0);

  uint8_t *destination_p = (uint8_t *) (buffer_p + (*in_out_buffer_offset_p / sizeof (uint32_t)));
  uint32_t literal_offset = 0;

  *in_out_buffer_offset_p += lit_table_size;
  *out_map_p = map_p;
  *out_map_len_p = total_count;

  lit_buffer_p = lit_pool_p->buffer_p;

  /* Generate literal pool data. */
  for (uint32_t i = 0; i < lit_pool_p->item_count; i++)
  {
    map_p->literal_id = lit_buffer_p[i];
    map_p->literal_offset = (literal_offset << JERRY_SNAPSHOT_LITERAL_SHIFT) | ECMA_TYPE_SNAPSHOT_OFFSET;

    lit_utf8_size_t length;

    if (ecma_is_value_float_number (lit_buffer_p[i]))
    {
      map_p->literal_offset |= JERRY_SNAPSHOT_LITERAL_IS_NUMBER;

      ecma_number_t num = ecma_get_float_from_value (lit_buffer_p[i]);
      memcpy (destination_p, &num, sizeof (ecma_number_t));

      length = JERRY_ALIGNUP (sizeof (ecma_number_t), JERRY_SNAPSHOT_LITERAL_ALIGNMENT);
    }
#if JERRY_BUILTIN_BIGINT
    else if (ecma_is_value_bigint (lit_buffer_p[i]))
    {
      map_p->literal_offset |= JERRY_SNAPSHOT_LITERAL_IS_BIGINT;

      ecma_extended_primitive_t *bigint_p = ecma_get_extended_primitive_from_value (lit_buffer_p[i]);
      uint32_t size = ECMA_BIGINT_GET_SIZE (bigint_p);

      memcpy (destination_p, &bigint_p->u.bigint_sign_and_size, sizeof (uint32_t));
      memcpy (destination_p + sizeof (uint32_t), ECMA_BIGINT_GET_DIGITS (bigint_p, 0), size);

      length = JERRY_ALIGNUP (sizeof (uint32_t) + size, JERRY_SNAPSHOT_LITERAL_ALIGNMENT);
    }
#endif /* JERRY_BUILTIN_BIGINT */
    else
    {
      ecma_string_t *string_p = ecma_get_string_from_value (lit_buffer_p[i]);
      length = ecma_string_get_size (string_p);

      *(uint16_t *) destination_p = (uint16_t) length;

      ecma_string_to_cesu8_bytes (string_p, destination_p + sizeof (uint16_t), length);

      length = JERRY_ALIGNUP (sizeof (uint16_t) + length, JERRY_SNAPSHOT_LITERAL_ALIGNMENT);
    }

    JERRY_ASSERT ((length % sizeof (uint16_t)) == 0);
    destination_p += length;
    literal_offset += length;

    map_p++;
  }

  ecma_collection_destroy (lit_pool_p);
  return true;
} /* ecma_save_literals_for_snapshot */

#endif /* JERRY_SNAPSHOT_SAVE */

#if JERRY_SNAPSHOT_EXEC || JERRY_SNAPSHOT_SAVE

/**
 * Get the compressed pointer of a given literal.
 *
 * @return literal compressed pointer
 */
ecma_value_t
ecma_snapshot_get_literal (const uint8_t *literal_base_p, /**< literal start */
                           ecma_value_t literal_value) /**< string / number offset */
{
  JERRY_ASSERT ((literal_value & ECMA_VALUE_TYPE_MASK) == ECMA_TYPE_SNAPSHOT_OFFSET);

  const uint8_t *literal_p = literal_base_p + (literal_value >> JERRY_SNAPSHOT_LITERAL_SHIFT);

  if (literal_value & JERRY_SNAPSHOT_LITERAL_IS_NUMBER)
  {
    ecma_number_t num;
    memcpy (&num, literal_p, sizeof (ecma_number_t));
    return ecma_find_or_create_literal_number (num);
  }

#if JERRY_BUILTIN_BIGINT
  if (literal_value & JERRY_SNAPSHOT_LITERAL_IS_BIGINT)
  {
    uint32_t bigint_sign_and_size = *(uint32_t *) literal_p;
    uint32_t size = bigint_sign_and_size & ~(uint32_t) (sizeof (ecma_bigint_digit_t) - 1);

    ecma_extended_primitive_t *bigint_p = ecma_bigint_create (size);

    if (bigint_p == NULL)
    {
      jerry_fatal (JERRY_FATAL_OUT_OF_MEMORY);
    }

    /* Only the sign bit can differ. */
    JERRY_ASSERT (bigint_p->u.bigint_sign_and_size == (bigint_sign_and_size & ~(uint32_t) ECMA_BIGINT_SIGN));

    bigint_p->u.bigint_sign_and_size = bigint_sign_and_size;
    memcpy (ECMA_BIGINT_GET_DIGITS (bigint_p, 0), literal_p + sizeof (uint32_t), size);
    return ecma_find_or_create_literal_bigint (ecma_make_extended_primitive_value (bigint_p, ECMA_TYPE_BIGINT));
  }
#endif /* JERRY_BUILTIN_BIGINT */

  uint16_t length = *(const uint16_t *) literal_p;

  return ecma_find_or_create_literal_string (literal_p + sizeof (uint16_t), length, false);
} /* ecma_snapshot_get_literal */

/**
 * Compute the start of the serializable ecma-values of the bytecode
 * Related values:
 *  - function argument names, if CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED is present
 *  - function name, if CBC_CODE_FLAGS_CLASS_CONSTRUCTOR is not present and es.next profile is enabled
 *
 * @return pointer to the beginning of the serializable ecma-values
 */
ecma_value_t *
ecma_snapshot_resolve_serializable_values (const ecma_compiled_code_t *compiled_code_p, /**< compiled code */
                                           uint8_t *bytecode_end_p) /**< end of the bytecode */
{
  ecma_value_t *base_p = (ecma_value_t *) bytecode_end_p;

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED)
  {
    uint32_t argument_end;
    if (compiled_code_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
    {
      argument_end = ((cbc_uint16_arguments_t *) compiled_code_p)->argument_end;
    }
    else
    {
      argument_end = ((cbc_uint8_arguments_t *) compiled_code_p)->argument_end;
    }

    base_p -= argument_end;
  }

  /* function name */
  if (CBC_FUNCTION_GET_TYPE (compiled_code_p->status_flags) != CBC_FUNCTION_CONSTRUCTOR)
  {
    base_p--;
  }

  return base_p;
} /* ecma_snapshot_resolve_serializable_values */
#endif /* JERRY_SNAPSHOT_EXEC || JERRY_SNAPSHOT_SAVE */

/**
 * @}
 * @}
 */
