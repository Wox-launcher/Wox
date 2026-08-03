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

#include "ecma-alloc.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers-number.h"
#include "ecma-helpers.h"

#include "jcontext.h"
#include "jrt-libc-includes.h"
#include "jrt.h"
#include "lit-char-helpers.h"
#include "lit-magic-strings.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmahelpers Helpers for operations with ECMA data types
 * @{
 */

JERRY_STATIC_ASSERT ((((int)ECMA_STRING_CONTAINER_MASK) >= ((int)ECMA_STRING_CONTAINER__MAX)),
                     ecma_string_container_types_must_be_lower_than_the_container_mask);

JERRY_STATIC_ASSERT (((ECMA_STRING_MAX_REF | ECMA_STRING_CONTAINER_MASK | ECMA_STATIC_STRING_FLAG) == UINT32_MAX),
                     ecma_string_ref_and_container_fields_should_fill_the_32_bit_field);

JERRY_STATIC_ASSERT (((uint32_t)ECMA_STRING_NOT_ARRAY_INDEX == UINT32_MAX),
                     ecma_string_not_array_index_must_be_equal_to_uint32_max);

JERRY_STATIC_ASSERT ((ECMA_TYPE_DIRECT_STRING & 0x1) != 0, ecma_type_direct_string_must_be_odd_number);

JERRY_STATIC_ASSERT ((((int)LIT_MAGIC_STRING__COUNT) <= ((int)ECMA_DIRECT_STRING_MAX_IMM)),
                     all_magic_strings_must_be_encoded_as_direct_string);

JERRY_STATIC_ASSERT (((int) ECMA_DIRECT_STRING_UINT == (int) ECMA_STRING_CONTAINER_UINT32_IN_DESC),
                     ecma_direct_and_container_types_must_match);

JERRY_STATIC_ASSERT ((((int)ECMA_PROPERTY_NAME_TYPE_SHIFT) > ((int)ECMA_VALUE_SHIFT)),
                     ecma_property_name_type_shift_must_be_greater_than_ecma_value_shift);

JERRY_STATIC_ASSERT ((sizeof (ecma_stringbuilder_header_t) <= ECMA_ASCII_STRING_HEADER_SIZE),
                     ecma_stringbuilder_header_must_not_be_larger_than_ecma_ascii_string);

/**
 * Convert a string to an unsigned 32 bit value if possible
 *
 * @return true if the conversion is successful
 *         false otherwise
 */
static bool
ecma_string_to_array_index (const lit_utf8_byte_t *string_p, /**< utf-8 string */
                            lit_utf8_size_t string_size, /**< string size */
                            uint32_t *result_p) /**< [out] converted value */
{
  JERRY_ASSERT (string_size > 0 && *string_p >= LIT_CHAR_0 && *string_p <= LIT_CHAR_9);

  if (*string_p == LIT_CHAR_0)
  {
    *result_p = 0;
    return (string_size == 1);
  }

  if (string_size > ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32)
  {
    return false;
  }

  uint32_t index = 0;
  const lit_utf8_byte_t *string_end_p = string_p + string_size;

  if (string_size == ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32)
  {
    string_end_p--;
  }

  do
  {
    if (*string_p > LIT_CHAR_9 || *string_p < LIT_CHAR_0)
    {
      return false;
    }

    index = (index * 10) + (uint32_t) (*string_p++ - LIT_CHAR_0);
  } while (string_p < string_end_p);

  if (string_size < ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32)
  {
    *result_p = index;
    return true;
  }

  /* Overflow must be checked as well when size is
   * equal to ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32. */
  if (*string_p > LIT_CHAR_9 || *string_p < LIT_CHAR_0 || index > (UINT32_MAX / 10)
      || (index == (UINT32_MAX / 10) && *string_p > LIT_CHAR_5))
  {
    return false;
  }

  *result_p = (index * 10) + (uint32_t) (*string_p - LIT_CHAR_0);
  return true;
} /* ecma_string_to_array_index */

/**
 * Returns the characters and size of a string.
 *
 * Note:
 *   UINT type is not supported
 *
 * @return byte array start - if the byte array of a string is available
 *         NULL - otherwise
 */
static const lit_utf8_byte_t *
ecma_string_get_chars_fast (const ecma_string_t *string_p, /**< ecma-string */
                            lit_utf8_size_t *size_p) /**< [out] size of the ecma string */
{
  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    if (ECMA_GET_DIRECT_STRING_TYPE (string_p) == ECMA_DIRECT_STRING_MAGIC)
    {
      uint32_t id = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);

      if (id >= LIT_MAGIC_STRING__COUNT)
      {
        id -= LIT_MAGIC_STRING__COUNT;

        *size_p = lit_get_magic_string_ex_size (id);
        return lit_get_magic_string_ex_utf8 (id);
      }

      *size_p = lit_get_magic_string_size (id);
      return lit_get_magic_string_utf8 (id);
    }
  }

  JERRY_ASSERT (string_p->refs_and_container >= ECMA_STRING_REF_ONE);

  switch (ECMA_STRING_GET_CONTAINER (string_p))
  {
    case ECMA_STRING_CONTAINER_HEAP_UTF8_STRING:
    {
      *size_p = ((ecma_short_string_t *) string_p)->size;
      return ECMA_SHORT_STRING_GET_BUFFER (string_p);
    }
    case ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING:
    {
      ecma_long_string_t *long_string_p = (ecma_long_string_t *) string_p;
      *size_p = long_string_p->size;
      return long_string_p->string_p;
    }
    case ECMA_STRING_CONTAINER_HEAP_ASCII_STRING:
    {
      *size_p = ECMA_ASCII_STRING_GET_SIZE (string_p);
      return ECMA_ASCII_STRING_GET_BUFFER (string_p);
    }
    default:
    {
      JERRY_ASSERT (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_MAGIC_STRING_EX);

      lit_magic_string_ex_id_t id = LIT_MAGIC_STRING__COUNT - string_p->u.magic_string_ex_id;
      *size_p = lit_get_magic_string_ex_size (id);
      return lit_get_magic_string_ex_utf8 (id);
    }
  }
} /* ecma_string_get_chars_fast */

/**
 * Allocate new ecma-string and fill it with reference to ECMA magic string
 *
 * @return pointer to ecma-string descriptor
 */
static ecma_string_t *
ecma_new_ecma_string_from_magic_string_ex_id (lit_magic_string_ex_id_t id) /**< identifier of external magic string */
{
  JERRY_ASSERT (id < lit_get_magic_string_ex_count ());

  uintptr_t string_id = (uintptr_t) (id + LIT_MAGIC_STRING__COUNT);

  if (JERRY_LIKELY (string_id <= ECMA_DIRECT_STRING_MAX_IMM))
  {
    return (ecma_string_t *) ECMA_CREATE_DIRECT_STRING (ECMA_DIRECT_STRING_MAGIC, string_id);
  }

  ecma_string_t *string_desc_p = ecma_alloc_string ();

  string_desc_p->refs_and_container = ECMA_STRING_CONTAINER_MAGIC_STRING_EX | ECMA_STRING_REF_ONE;
  string_desc_p->u.magic_string_ex_id = id + LIT_MAGIC_STRING__COUNT;

  return string_desc_p;
} /* ecma_new_ecma_string_from_magic_string_ex_id */

/**
 * Allocate new ecma-string and fill it with reference to the symbol descriptor
 *
 * Note:
 *     Takes the reference to the string_desc
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_symbol_from_descriptor_string (ecma_value_t string_desc) /**< ecma-string */
{
  JERRY_ASSERT (!ecma_is_value_symbol (string_desc));

  ecma_extended_string_t *symbol_p = ecma_alloc_extended_string ();
  symbol_p->header.refs_and_container = ECMA_STRING_REF_ONE | ECMA_STRING_CONTAINER_SYMBOL;
  symbol_p->u.symbol_descriptor = string_desc;
  symbol_p->header.u.hash = (lit_string_hash_t) (((uintptr_t) symbol_p) & (uintptr_t) ~ECMA_SYMBOL_FLAGS_MASK);

  return (ecma_string_t *) symbol_p;
} /* ecma_new_symbol_from_descriptor_string */

/**
 * Check whether an ecma-string contains an ecma-symbol
 *
 * @return true - if the ecma-string contains an ecma-symbol
 *         false - otherwise
 */
bool
ecma_prop_name_is_symbol (ecma_string_t *string_p) /**< ecma-string */
{
  JERRY_ASSERT (string_p != NULL);

  return (!ECMA_IS_DIRECT_STRING (string_p) && ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_SYMBOL);
} /* ecma_prop_name_is_symbol */

/**
 * Allocate new UTF8 ecma-string and fill it with characters from the given utf8 buffer
 *
 * @return pointer to ecma-string descriptor
 */
static inline ecma_string_t *
ecma_new_ecma_string_from_utf8_buffer (lit_utf8_size_t length, /**< length of the buffer */
                                       lit_utf8_size_t size, /**< size of the buffer */
                                       lit_utf8_byte_t **data_p) /**< [out] pointer to the start of the string buffer */
{
  if (JERRY_LIKELY (size <= UINT16_MAX))
  {
    if (JERRY_LIKELY (length == size) && size <= (UINT8_MAX + 1))
    {
      ecma_string_t *string_desc_p;
      string_desc_p = (ecma_string_t *) ecma_alloc_string_buffer (size + ECMA_ASCII_STRING_HEADER_SIZE);
      string_desc_p->refs_and_container = ECMA_STRING_CONTAINER_HEAP_ASCII_STRING | ECMA_STRING_REF_ONE;
      ECMA_ASCII_STRING_SET_SIZE (string_desc_p, size);

      *data_p = ECMA_ASCII_STRING_GET_BUFFER (string_desc_p);
      return (ecma_string_t *) string_desc_p;
    }

    ecma_short_string_t *string_desc_p;
    string_desc_p = (ecma_short_string_t *) ecma_alloc_string_buffer (size + sizeof (ecma_short_string_t));
    string_desc_p->header.refs_and_container = ECMA_STRING_CONTAINER_HEAP_UTF8_STRING | ECMA_STRING_REF_ONE;
    string_desc_p->size = (uint16_t) size;
    string_desc_p->length = (uint16_t) length;

    *data_p = ECMA_SHORT_STRING_GET_BUFFER (string_desc_p);
    return (ecma_string_t *) string_desc_p;
  }

  ecma_long_string_t *long_string_p;
  long_string_p = (ecma_long_string_t *) ecma_alloc_string_buffer (size + sizeof (ecma_long_string_t));
  long_string_p->header.refs_and_container = ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING | ECMA_STRING_REF_ONE;
  long_string_p->string_p = ECMA_LONG_STRING_BUFFER_START (long_string_p);
  long_string_p->size = size;
  long_string_p->length = length;

  *data_p = ECMA_LONG_STRING_BUFFER_START (long_string_p);
  return (ecma_string_t *) long_string_p;
} /* ecma_new_ecma_string_from_utf8_buffer */

/**
 * Checks whether a string has a special representation, that is, the string is either a magic string,
 * an external magic string, or an uint32 number, and creates an ecma string using the special representation,
 * if available.
 *
 * @return pointer to ecma string with the special representation
 *         NULL, if there is no special representation for the string
 */
static ecma_string_t *
ecma_find_special_string (const lit_utf8_byte_t *string_p, /**< utf8 string */
                          lit_utf8_size_t string_size) /**< string size */
{
  JERRY_ASSERT (string_p != NULL || string_size == 0);
  lit_magic_string_id_t magic_string_id = lit_is_utf8_string_magic (string_p, string_size);

  if (magic_string_id != LIT_MAGIC_STRING__COUNT)
  {
    return ecma_get_magic_string (magic_string_id);
  }

  JERRY_ASSERT (string_size > 0);

  if (*string_p >= LIT_CHAR_0 && *string_p <= LIT_CHAR_9)
  {
    uint32_t array_index;

    if (ecma_string_to_array_index (string_p, string_size, &array_index))
    {
      return ecma_new_ecma_string_from_uint32 (array_index);
    }
  }

  if (lit_get_magic_string_ex_count () > 0)
  {
    lit_magic_string_ex_id_t magic_string_ex_id = lit_is_ex_utf8_string_magic (string_p, string_size);

    if (magic_string_ex_id < lit_get_magic_string_ex_count ())
    {
      return ecma_new_ecma_string_from_magic_string_ex_id (magic_string_ex_id);
    }
  }

  return NULL;
} /* ecma_find_special_string */

/**
 * Allocate new ecma-string and fill it with characters from ascii characters
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_string_from_ascii (const lit_utf8_byte_t *string_p, /**< ascii string */
                                 lit_utf8_size_t string_size) /**< string size */
{
  JERRY_ASSERT (string_p != NULL || string_size == 0);

  ecma_string_t *string_desc_p = ecma_find_special_string (string_p, string_size);

  if (string_desc_p != NULL)
  {
    return string_desc_p;
  }

  lit_utf8_byte_t *data_p;
  string_desc_p = ecma_new_ecma_string_from_utf8_buffer (string_size, string_size, &data_p);

  string_desc_p->u.hash = lit_utf8_string_calc_hash (string_p, string_size);
  memcpy (data_p, string_p, string_size);

  return string_desc_p;
} /* ecma_new_ecma_string_from_ascii */

/**
 * Allocate new ecma-string and fill it with characters from the utf8 string
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_string_from_utf8 (const lit_utf8_byte_t *string_p, /**< utf-8 string */
                                lit_utf8_size_t string_size) /**< string size */
{
  JERRY_ASSERT (string_p != NULL || string_size == 0);
  JERRY_ASSERT (lit_is_valid_cesu8_string (string_p, string_size));

  ecma_string_t *string_desc_p = ecma_find_special_string (string_p, string_size);

  if (string_desc_p != NULL)
  {
    return string_desc_p;
  }

  lit_utf8_byte_t *data_p;
  string_desc_p =
    ecma_new_ecma_string_from_utf8_buffer (lit_utf8_string_length (string_p, string_size), string_size, &data_p);

  string_desc_p->u.hash = lit_utf8_string_calc_hash (string_p, string_size);
  memcpy (data_p, string_p, string_size);

  return string_desc_p;
} /* ecma_new_ecma_string_from_utf8 */

/**
 * Allocate a new ecma-string and initialize it from the utf8 string argument.
 * All 4-bytes long unicode sequences are converted into two 3-bytes long sequences.
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_string_from_utf8_converted_to_cesu8 (const lit_utf8_byte_t *string_p, /**< utf-8 string */
                                                   lit_utf8_size_t string_size) /**< utf-8 string size */
{
  JERRY_ASSERT (string_p != NULL || string_size == 0);

  lit_utf8_size_t converted_string_length = 0;
  lit_utf8_size_t converted_string_size = 0;
  lit_utf8_size_t pos = 0;

  /* Calculate the required length and size information of the converted cesu-8 encoded string */
  while (pos < string_size)
  {
    if ((string_p[pos] & LIT_UTF8_1_BYTE_MASK) == LIT_UTF8_1_BYTE_MARKER)
    {
      pos++;
    }
    else if ((string_p[pos] & LIT_UTF8_2_BYTE_MASK) == LIT_UTF8_2_BYTE_MARKER)
    {
      pos += 2;
    }
    else if ((string_p[pos] & LIT_UTF8_3_BYTE_MASK) == LIT_UTF8_3_BYTE_MARKER)
    {
      pos += 3;
    }
    else
    {
      JERRY_ASSERT ((string_p[pos] & LIT_UTF8_4_BYTE_MASK) == LIT_UTF8_4_BYTE_MARKER);
      pos += 4;
      converted_string_size += 2;
      converted_string_length++;
    }

    converted_string_length++;
  }

  JERRY_ASSERT (pos == string_size);

  if (converted_string_size == 0)
  {
    return ecma_new_ecma_string_from_utf8 (string_p, string_size);
  }

  converted_string_size += string_size;

  JERRY_ASSERT (lit_is_valid_utf8_string (string_p, string_size, false));

  lit_utf8_byte_t *data_p;
  ecma_string_t *string_desc_p =
    ecma_new_ecma_string_from_utf8_buffer (converted_string_length, converted_string_size, &data_p);

  const lit_utf8_byte_t *const begin_data_p = data_p;
  pos = 0;

  while (pos < string_size)
  {
    if ((string_p[pos] & LIT_UTF8_4_BYTE_MASK) == LIT_UTF8_4_BYTE_MARKER)
    {
      /* Processing 4 byte unicode sequence. Always converted to two 3 byte long sequence. */
      lit_four_byte_utf8_char_to_cesu8 (data_p, string_p + pos);
      data_p += 3 * 2;
      pos += 4;
    }
    else
    {
      *data_p++ = string_p[pos++];
    }
  }

  JERRY_ASSERT (pos == string_size);

  string_desc_p->u.hash = lit_utf8_string_calc_hash (begin_data_p, converted_string_size);

  return (ecma_string_t *) string_desc_p;
} /* ecma_new_ecma_string_from_utf8_converted_to_cesu8 */

/**
 * Allocate new ecma-external-string and fill it with characters from the cesu8 string
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_external_string_from_cesu8 (const lit_utf8_byte_t *string_p, /**< cesu-8 string */
                                          lit_utf8_size_t string_size, /**< string size */
                                          void *user_p) /**< user pointer passed to the callback
                                                         *   when the string is freed */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (string_p != NULL || string_size == 0);
  JERRY_ASSERT (lit_is_valid_cesu8_string (string_p, string_size));

  if (string_size < (sizeof (ecma_external_string_t) - sizeof (ecma_short_string_t)))
  {
    /* Normal strings are created for short strings. */
    ecma_string_t *string_desc_p = ecma_new_ecma_string_from_utf8 (string_p, string_size);

    jerry_external_string_free_cb_t free_cb = JERRY_CONTEXT (external_string_free_callback_p);

    if (free_cb != NULL)
    {
      free_cb ((lit_utf8_byte_t *) string_p, string_size, user_p);
    }
    return string_desc_p;
  }

  ecma_string_t *string_desc_p = ecma_find_special_string (string_p, string_size);

  if (string_desc_p != NULL)
  {
    jerry_external_string_free_cb_t free_cb = JERRY_CONTEXT (external_string_free_callback_p);

    if (free_cb != NULL)
    {
      free_cb ((lit_utf8_byte_t *) string_p, string_size, user_p);
    }
    return string_desc_p;
  }

  ecma_external_string_t *external_string_p = ecma_alloc_external_string ();
  ecma_long_string_t *long_string_p = (ecma_long_string_t *) external_string_p;

  long_string_p->header.refs_and_container = ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING | ECMA_STRING_REF_ONE;
  long_string_p->header.u.hash = lit_utf8_string_calc_hash (string_p, string_size);
  long_string_p->string_p = string_p;
  long_string_p->size = string_size;
  long_string_p->length = lit_utf8_string_length (string_p, string_size);
  external_string_p->user_p = user_p;

  return (ecma_string_t *) external_string_p;
} /* ecma_new_ecma_external_string_from_cesu8 */

/**
 * Allocate new ecma-string and fill it with cesu-8 character which represents specified code unit
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_string_from_code_unit (ecma_char_t code_unit) /**< code unit */
{
  lit_utf8_byte_t lit_utf8_bytes[LIT_UTF8_MAX_BYTES_IN_CODE_UNIT];
  lit_utf8_size_t bytes_size = lit_code_unit_to_utf8 (code_unit, lit_utf8_bytes);

  return ecma_new_ecma_string_from_utf8 (lit_utf8_bytes, bytes_size);
} /* ecma_new_ecma_string_from_code_unit */

/**
 * Allocate new ecma-string and fill it with cesu-8 character which represents specified code units
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_string_from_code_units (ecma_char_t first_code_unit, /**< code unit */
                                      ecma_char_t second_code_unit) /**< code unit */
{
  lit_utf8_byte_t lit_utf8_bytes[2 * LIT_UTF8_MAX_BYTES_IN_CODE_UNIT];
  lit_utf8_size_t bytes_size = lit_code_unit_to_utf8 (first_code_unit, lit_utf8_bytes);
  bytes_size += lit_code_unit_to_utf8 (second_code_unit, lit_utf8_bytes + bytes_size);

  return ecma_new_ecma_string_from_utf8 (lit_utf8_bytes, bytes_size);
} /* ecma_new_ecma_string_from_code_units */

/**
 * Allocate new ecma-string and fill it with ecma-number
 *
 * Note: the number cannot be represented as direct string
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_non_direct_string_from_uint32 (uint32_t uint32_number) /**< uint32 value of the string */
{
  JERRY_ASSERT (uint32_number > ECMA_DIRECT_STRING_MAX_IMM);

  ecma_string_t *string_p = ecma_alloc_string ();

  string_p->refs_and_container = ECMA_STRING_CONTAINER_UINT32_IN_DESC | ECMA_STRING_REF_ONE;
  string_p->u.uint32_number = uint32_number;

  return string_p;
} /* ecma_new_non_direct_string_from_uint32 */

/**
 * Allocate new ecma-string and fill it with property length number
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_string_from_length (ecma_length_t number) /**< property length */
{
  if (JERRY_LIKELY (number <= ECMA_DIRECT_STRING_MAX_IMM))
  {
    return (ecma_string_t *) ECMA_CREATE_DIRECT_STRING (ECMA_DIRECT_STRING_UINT, (uintptr_t) number);
  }

  JERRY_ASSERT ((ecma_number_t) number <= ECMA_NUMBER_MAX_SAFE_INTEGER);

  if (JERRY_UNLIKELY (number > UINT32_MAX))
  {
    return ecma_new_ecma_string_from_number ((ecma_number_t) number);
  }

  return ecma_new_non_direct_string_from_uint32 ((uint32_t) number);
} /* ecma_new_ecma_string_from_length */

/**
 * Allocate new ecma-string and fill it with uint32 number
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_string_from_uint32 (uint32_t uint32_number) /**< uint32 value of the string */
{
  if (JERRY_LIKELY (uint32_number <= ECMA_DIRECT_STRING_MAX_IMM))
  {
    return (ecma_string_t *) ECMA_CREATE_DIRECT_STRING (ECMA_DIRECT_STRING_UINT, (uintptr_t) uint32_number);
  }

  return ecma_new_non_direct_string_from_uint32 (uint32_number);
} /* ecma_new_ecma_string_from_uint32 */

/**
 * Returns the constant assigned to the uint32 number.
 *
 * Note:
 *   Calling ecma_deref_ecma_string on the returned pointer is optional.
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_get_ecma_string_from_uint32 (uint32_t uint32_number) /**< input number */
{
  JERRY_ASSERT (uint32_number <= ECMA_DIRECT_STRING_MAX_IMM);

  return (ecma_string_t *) ECMA_CREATE_DIRECT_STRING (ECMA_DIRECT_STRING_UINT, (uintptr_t) uint32_number);
} /* ecma_get_ecma_string_from_uint32 */

/**
 * Allocate new ecma-string and fill it with ecma-number
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_new_ecma_string_from_number (ecma_number_t num) /**< ecma-number */
{
  uint32_t uint32_num = ecma_number_to_uint32 (num);
  if (num == ((ecma_number_t) uint32_num))
  {
    return ecma_new_ecma_string_from_uint32 (uint32_num);
  }

  if (ecma_number_is_nan (num))
  {
    return ecma_get_magic_string (LIT_MAGIC_STRING_NAN);
  }

  if (ecma_number_is_infinity (num))
  {
    lit_magic_string_id_t id =
      (ecma_number_is_negative (num) ? LIT_MAGIC_STRING_NEGATIVE_INFINITY_UL : LIT_MAGIC_STRING_INFINITY_UL);
    return ecma_get_magic_string (id);
  }

  lit_utf8_byte_t str_buf[ECMA_MAX_CHARS_IN_STRINGIFIED_NUMBER];
  lit_utf8_size_t str_size = ecma_number_to_utf8_string (num, str_buf, sizeof (str_buf));

  JERRY_ASSERT (str_size > 0);
#ifndef JERRY_NDEBUG
  JERRY_ASSERT (lit_is_utf8_string_magic (str_buf, str_size) == LIT_MAGIC_STRING__COUNT
                && lit_is_ex_utf8_string_magic (str_buf, str_size) == lit_get_magic_string_ex_count ());
#endif /* !JERRY_NDEBUG */

  lit_utf8_byte_t *data_p;
  ecma_string_t *string_desc_p =
    ecma_new_ecma_string_from_utf8_buffer (lit_utf8_string_length (str_buf, str_size), str_size, &data_p);

  string_desc_p->u.hash = lit_utf8_string_calc_hash (str_buf, str_size);
  memcpy (data_p, str_buf, str_size);

  return string_desc_p;
} /* ecma_new_ecma_string_from_number */

/**
 * Returns the constant assigned to the magic string id.
 *
 * Note:
 *   Calling ecma_deref_ecma_string on the returned pointer is optional.
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_get_magic_string (lit_magic_string_id_t id) /**< identifier of magic string */
{
  JERRY_ASSERT (id < LIT_MAGIC_STRING__COUNT);
  return (ecma_string_t *) ECMA_CREATE_DIRECT_STRING (ECMA_DIRECT_STRING_MAGIC, (uintptr_t) id);
} /* ecma_get_magic_string */

/**
 * Returns the constant assigned to the internal magic string id.
 *
 * Note:
 *   Calling ecma_deref_ecma_string on the returned pointer is optional.
 *
 * @return pointer to ecma-string descriptor
 */
ecma_string_t *
ecma_get_internal_string (lit_magic_string_id_t id) /**< identifier of magic string */
{
  JERRY_ASSERT (id >= LIT_NON_INTERNAL_MAGIC_STRING__COUNT && id < LIT_MAGIC_STRING__COUNT);
  return (ecma_string_t *) ECMA_CREATE_DIRECT_STRING (ECMA_DIRECT_STRING_SPECIAL, (uintptr_t) id);
} /* ecma_get_internal_string */

/**
 * Append a cesu8 string after an ecma-string
 *
 * Note:
 *   The string1_p argument is freed. If it needs to be preserved,
 *   call ecma_ref_ecma_string with string1_p before the call.
 *
 * @return concatenation of an ecma-string and a cesu8 string
 */
ecma_string_t *
ecma_append_chars_to_string (ecma_string_t *string1_p, /**< base ecma-string */
                             const lit_utf8_byte_t *cesu8_string2_p, /**< characters to be appended */
                             lit_utf8_size_t cesu8_string2_size, /**< byte size of cesu8_string2_p */
                             lit_utf8_size_t cesu8_string2_length) /**< character length of cesu8_string2_p */
{
  JERRY_ASSERT (string1_p != NULL && cesu8_string2_size > 0 && cesu8_string2_length > 0);

  if (JERRY_UNLIKELY (ecma_string_is_empty (string1_p)))
  {
    return ecma_new_ecma_string_from_utf8 (cesu8_string2_p, cesu8_string2_size);
  }

  lit_utf8_size_t cesu8_string1_size;
  lit_utf8_size_t cesu8_string1_length;
  uint8_t flags = ECMA_STRING_FLAG_IS_ASCII;
  lit_utf8_byte_t uint32_to_string_buffer[ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32];

  const lit_utf8_byte_t *cesu8_string1_p =
    ecma_string_get_chars (string1_p, &cesu8_string1_size, &cesu8_string1_length, uint32_to_string_buffer, &flags);

  JERRY_ASSERT (!(flags & ECMA_STRING_FLAG_MUST_BE_FREED));
  JERRY_ASSERT (cesu8_string1_length > 0);
  JERRY_ASSERT (cesu8_string1_length <= cesu8_string1_size);

  lit_utf8_size_t new_size = cesu8_string1_size + cesu8_string2_size;

  /* Poor man's carry flag check: it is impossible to allocate this large string. */
  if (new_size < (cesu8_string1_size | cesu8_string2_size))
  {
    jerry_fatal (JERRY_FATAL_OUT_OF_MEMORY);
  }

  lit_magic_string_id_t magic_string_id;
  magic_string_id =
    lit_is_utf8_string_pair_magic (cesu8_string1_p, cesu8_string1_size, cesu8_string2_p, cesu8_string2_size);

  if (magic_string_id != LIT_MAGIC_STRING__COUNT)
  {
    ecma_deref_ecma_string (string1_p);
    return ecma_get_magic_string (magic_string_id);
  }

  if ((flags & ECMA_STRING_FLAG_IS_UINT32) && new_size <= ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32)
  {
    memcpy (uint32_to_string_buffer + cesu8_string1_size, cesu8_string2_p, cesu8_string2_size);

    uint32_t array_index;

    if (ecma_string_to_array_index (uint32_to_string_buffer, new_size, &array_index))
    {
      ecma_deref_ecma_string (string1_p);
      return ecma_new_ecma_string_from_uint32 (array_index);
    }
  }

  if (lit_get_magic_string_ex_count () > 0)
  {
    lit_magic_string_ex_id_t magic_string_ex_id;
    magic_string_ex_id =
      lit_is_ex_utf8_string_pair_magic (cesu8_string1_p, cesu8_string1_size, cesu8_string2_p, cesu8_string2_size);

    if (magic_string_ex_id < lit_get_magic_string_ex_count ())
    {
      ecma_deref_ecma_string (string1_p);
      return ecma_new_ecma_string_from_magic_string_ex_id (magic_string_ex_id);
    }
  }

  lit_utf8_byte_t *data_p;
  ecma_string_t *string_desc_p =
    ecma_new_ecma_string_from_utf8_buffer (cesu8_string1_length + cesu8_string2_length, new_size, &data_p);

  lit_string_hash_t hash_start;

  if (JERRY_UNLIKELY (flags & ECMA_STRING_FLAG_REHASH_NEEDED))
  {
    hash_start = lit_utf8_string_calc_hash (cesu8_string1_p, cesu8_string1_size);
  }
  else
  {
    JERRY_ASSERT (!ECMA_IS_DIRECT_STRING (string1_p));
    hash_start = string1_p->u.hash;
  }

  string_desc_p->u.hash = lit_utf8_string_hash_combine (hash_start, cesu8_string2_p, cesu8_string2_size);

  memcpy (data_p, cesu8_string1_p, cesu8_string1_size);
  memcpy (data_p + cesu8_string1_size, cesu8_string2_p, cesu8_string2_size);

  ecma_deref_ecma_string (string1_p);
  return (ecma_string_t *) string_desc_p;
} /* ecma_append_chars_to_string */

/**
 * Concatenate ecma-strings
 *
 * Note:
 *   The string1_p argument is freed. If it needs to be preserved,
 *   call ecma_ref_ecma_string with string1_p before the call.
 *
 * @return concatenation of two ecma-strings
 */
ecma_string_t *
ecma_concat_ecma_strings (ecma_string_t *string1_p, /**< first ecma-string */
                          ecma_string_t *string2_p) /**< second ecma-string */
{
  JERRY_ASSERT (string1_p != NULL && string2_p != NULL);

  if (JERRY_UNLIKELY (ecma_string_is_empty (string1_p)))
  {
    ecma_ref_ecma_string (string2_p);
    return string2_p;
  }
  else if (JERRY_UNLIKELY (ecma_string_is_empty (string2_p)))
  {
    return string1_p;
  }

  lit_utf8_size_t cesu8_string2_size;
  lit_utf8_size_t cesu8_string2_length;
  lit_utf8_byte_t uint32_to_string_buffer[ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32];
  uint8_t flags = ECMA_STRING_FLAG_IS_ASCII;

  const lit_utf8_byte_t *cesu8_string2_p =
    ecma_string_get_chars (string2_p, &cesu8_string2_size, &cesu8_string2_length, uint32_to_string_buffer, &flags);

  JERRY_ASSERT (cesu8_string2_p != NULL);

  ecma_string_t *result_p =
    ecma_append_chars_to_string (string1_p, cesu8_string2_p, cesu8_string2_size, cesu8_string2_length);

  JERRY_ASSERT (!(flags & ECMA_STRING_FLAG_MUST_BE_FREED));

  return result_p;
} /* ecma_concat_ecma_strings */

/**
 * Increase reference counter of non-direct ecma-string.
 *
 * @return void
 */
void
ecma_ref_ecma_string_non_direct (ecma_string_t *string_p) /**< string descriptor */
{
  JERRY_ASSERT (string_p != NULL);
  JERRY_ASSERT (!ECMA_IS_DIRECT_STRING (string_p));

#ifdef JERRY_NDEBUG
  if (ECMA_STRING_IS_STATIC (string_p))
  {
    return;
  }
#endif /* JERRY_NDEBUG */

  JERRY_ASSERT (string_p->refs_and_container >= ECMA_STRING_REF_ONE);

  if (JERRY_LIKELY (string_p->refs_and_container < ECMA_STRING_MAX_REF))
  {
    /* Increase reference counter. */
    string_p->refs_and_container += ECMA_STRING_REF_ONE;
  }
  else
  {
    jerry_fatal (JERRY_FATAL_REF_COUNT_LIMIT);
  }
} /* ecma_ref_ecma_string_non_direct */

/**
 * Increase reference counter of ecma-string.
 */
void
ecma_ref_ecma_string (ecma_string_t *string_p) /**< string descriptor */
{
  JERRY_ASSERT (string_p != NULL);

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    return;
  }

  ecma_ref_ecma_string_non_direct (string_p);
} /* ecma_ref_ecma_string */

/**
 * Decrease reference counter and deallocate a non-direct ecma-string
 * if the counter becomes zero.
 *
 * @return void
 */
void
ecma_deref_ecma_string_non_direct (ecma_string_t *string_p) /**< ecma-string */
{
  JERRY_ASSERT (!ECMA_IS_DIRECT_STRING (string_p));

#ifdef JERRY_NDEBUG
  if (ECMA_STRING_IS_STATIC (string_p))
  {
    return;
  }
#endif /* JERRY_NDEBUG */

  JERRY_ASSERT (string_p->refs_and_container >= ECMA_STRING_REF_ONE);

  /* Decrease reference counter. */
  string_p->refs_and_container -= ECMA_STRING_REF_ONE;

  if (string_p->refs_and_container >= ECMA_STRING_REF_ONE)
  {
    return;
  }

  ecma_destroy_ecma_string (string_p);
} /* ecma_deref_ecma_string_non_direct */

/**
 * Decrease reference counter and deallocate ecma-string
 * if the counter becomes zero.
 */
void
ecma_deref_ecma_string (ecma_string_t *string_p) /**< ecma-string */
{
  JERRY_ASSERT (string_p != NULL);

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    return;
  }

  ecma_deref_ecma_string_non_direct (string_p);
} /* ecma_deref_ecma_string */

/**
 * Deallocate an ecma-string
 */
void
ecma_destroy_ecma_string (ecma_string_t *string_p) /**< ecma-string */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (string_p != NULL);
  JERRY_ASSERT (!ECMA_IS_DIRECT_STRING (string_p));
  JERRY_ASSERT ((string_p->refs_and_container < ECMA_STRING_REF_ONE) || ECMA_STRING_IS_STATIC (string_p));

  switch (ECMA_STRING_GET_CONTAINER (string_p))
  {
    case ECMA_STRING_CONTAINER_HEAP_UTF8_STRING:
    {
      ecma_dealloc_string_buffer (string_p, ((ecma_short_string_t *) string_p)->size + sizeof (ecma_short_string_t));
      return;
    }
    case ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING:
    {
      ecma_long_string_t *long_string_p = (ecma_long_string_t *) string_p;

      if (long_string_p->string_p == ECMA_LONG_STRING_BUFFER_START (long_string_p))
      {
        ecma_dealloc_string_buffer (string_p, long_string_p->size + sizeof (ecma_long_string_t));
        return;
      }

      ecma_external_string_t *external_string_p = (ecma_external_string_t *) string_p;
      jerry_external_string_free_cb_t free_cb = JERRY_CONTEXT (external_string_free_callback_p);

      if (free_cb != NULL)
      {
        free_cb ((lit_utf8_byte_t *) external_string_p->header.string_p,
                 external_string_p->header.size,
                 external_string_p->user_p);
      }

      ecma_dealloc_external_string (external_string_p);
      return;
    }
    case ECMA_STRING_CONTAINER_HEAP_ASCII_STRING:
    {
      ecma_dealloc_string_buffer (string_p, ECMA_ASCII_STRING_GET_SIZE (string_p) + ECMA_ASCII_STRING_HEADER_SIZE);
      return;
    }
    case ECMA_STRING_CONTAINER_SYMBOL:
    {
      ecma_extended_string_t *symbol_p = (ecma_extended_string_t *) string_p;
      ecma_free_value (symbol_p->u.symbol_descriptor);
      ecma_dealloc_extended_string (symbol_p);
      return;
    }
    default:
    {
      JERRY_ASSERT (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC
                    || ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_MAGIC_STRING_EX);

      /* only the string descriptor itself should be freed */
      ecma_dealloc_string (string_p);
    }
  }
} /* ecma_destroy_ecma_string */

/**
 * Convert ecma-string to number
 *
 * @return converted ecma-number
 */
ecma_number_t
ecma_string_to_number (const ecma_string_t *string_p) /**< ecma-string */
{
  JERRY_ASSERT (string_p != NULL);

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    if (ECMA_IS_DIRECT_STRING_WITH_TYPE (string_p, ECMA_DIRECT_STRING_UINT))
    {
      return (ecma_number_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);
    }
  }
  else if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC)
  {
    return ((ecma_number_t) string_p->u.uint32_number);
  }

  lit_utf8_size_t size;
  const lit_utf8_byte_t *chars_p = ecma_string_get_chars_fast (string_p, &size);

  JERRY_ASSERT (chars_p != NULL);

  if (size == 0)
  {
    return ECMA_NUMBER_ZERO;
  }

  return ecma_utf8_string_to_number (chars_p, size, 0);
} /* ecma_string_to_number */

/**
 * Check if string is array index.
 *
 * @return ECMA_STRING_NOT_ARRAY_INDEX if string is not array index
 *         the array index otherwise
 */
uint32_t
ecma_string_get_array_index (const ecma_string_t *str_p) /**< ecma-string */
{
  if (ECMA_IS_DIRECT_STRING (str_p))
  {
    if (ECMA_IS_DIRECT_STRING_WITH_TYPE (str_p, ECMA_DIRECT_STRING_UINT))
    {
      /* Value cannot be equal to the maximum value of a 32 bit unsigned number. */
      return (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (str_p);
    }

    return ECMA_STRING_NOT_ARRAY_INDEX;
  }

  if (ECMA_STRING_GET_CONTAINER (str_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC)
  {
    /* When the uint32_number is equal to the maximum value of 32 bit unsigned integer number,
     * it is also an invalid array index. The comparison to ECMA_STRING_NOT_ARRAY_INDEX will
     * be true in this case. */
    return str_p->u.uint32_number;
  }

  return ECMA_STRING_NOT_ARRAY_INDEX;
} /* ecma_string_get_array_index */

/**
 * Copy digits of uint32 number, truncating if buffer is not large enough.
 *
 * @return number of digits copied
 */
static lit_utf8_size_t
ecma_uint32_to_buffer (uint32_t num, /**< number */
                       lit_utf8_byte_t *buffer_p /**< destination buffer */,
                       lit_utf8_size_t buffer_size /**< buffer size */)
{
  lit_utf8_byte_t digits[ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32];
  lit_utf8_size_t digit_count = ecma_uint32_to_utf8_string (num, digits, ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32);

  digit_count = JERRY_MIN (buffer_size, digit_count);
  memcpy (buffer_p, digits, digit_count);
  return digit_count;
} /* ecma_uint32_to_buffer */

/**
 * Convert ecma-string's contents to the specified encoding and copy it to the buffer.
 * String data will be truncated to fit the buffer.
 *
 * @return number of bytes copied to the buffer.
 */
lit_utf8_size_t
ecma_string_copy_to_buffer (const ecma_string_t *string_p, /**< ecma-string descriptor */
                            lit_utf8_byte_t *buffer_p, /**< destination buffer pointer
                                                        * (can be NULL if buffer_size == 0) */
                            lit_utf8_size_t buffer_size, /**< size of buffer */
                            jerry_encoding_t encoding) /**< encoding */
{
  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    if (ECMA_IS_DIRECT_STRING_WITH_TYPE (string_p, ECMA_DIRECT_STRING_UINT))
    {
      return ecma_uint32_to_buffer ((uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p), buffer_p, buffer_size);
    }
  }
  else
  {
    JERRY_ASSERT (string_p->refs_and_container >= ECMA_STRING_REF_ONE);

    if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC)
    {
      return ecma_uint32_to_buffer (string_p->u.uint32_number, buffer_p, buffer_size);
    }
  }

  lit_utf8_size_t string_size;
  const lit_utf8_byte_t *chars_p = ecma_string_get_chars_fast (string_p, &string_size);
  lit_utf8_size_t copy_size = 0;

  JERRY_ASSERT (chars_p != NULL);

  switch (encoding)
  {
    case JERRY_ENCODING_CESU8:
    {
      copy_size = JERRY_MIN (string_size, buffer_size);

      if (copy_size < string_size)
      {
        /* Do not copy partial characters */
        while ((chars_p[copy_size] & LIT_UTF8_EXTRA_BYTE_MASK) == LIT_UTF8_EXTRA_BYTE_MARKER)
        {
          copy_size--;
        }
      }

      memcpy (buffer_p, chars_p, copy_size);
      break;
    }
    case JERRY_ENCODING_UTF8:
    {
      if (string_size == ecma_string_get_length (string_p))
      {
        copy_size = JERRY_MIN (string_size, buffer_size);

        memcpy (buffer_p, chars_p, copy_size);
        break;
      }

      copy_size = lit_convert_cesu8_string_to_utf8_string (chars_p, string_size, buffer_p, buffer_size);
      break;
    }
    default:
    {
      break;
    }
  }

  return copy_size;
} /* ecma_string_copy_to_buffer */

/**
 * Convert ecma-string's contents to a cesu-8 string and put it to the buffer.
 * It is the caller's responsibility to make sure that the string fits in the buffer.
 * Check if the size of the string is equal with the size of the buffer.
 *
 * @return void
 */
void
ecma_string_to_cesu8_bytes (const ecma_string_t *string_desc_p, /**< ecma-string descriptor */
                            lit_utf8_byte_t *buffer_p, /**< destination buffer pointer (can be NULL if buffer_size == 0) */
                            lit_utf8_size_t buffer_size) /**< size of buffer */
{
  ecma_string_copy_to_buffer (string_desc_p, buffer_p, buffer_size, JERRY_ENCODING_CESU8);
}

/**
 * Get size of the uint32 number stored locally in the string's descriptor
 *
 * Note: the represented number size and length are equal
 *
 * @return size in bytes
 */
static inline lit_utf8_size_t
ecma_string_get_uint32_size (const uint32_t uint32_number) /**< number in the string-descriptor */
{
  uint32_t prev_number = 1;
  uint32_t next_number = 100;
  lit_utf8_size_t size = 1;

  const uint32_t max_size = 9;

  while (size < max_size && uint32_number >= next_number)
  {
    prev_number = next_number;
    next_number *= 100;
    size += 2;
  }

  if (uint32_number >= prev_number * 10)
  {
    size++;
  }

  return size;
} /* ecma_string_get_uint32_size */

/**
 * Checks whether the given string is a sequence of ascii characters.
 */
#define ECMA_STRING_IS_ASCII(char_p, size) ((size) == lit_utf8_string_length ((char_p), (size)))

/**
 * Returns with the cesu8 character array of a string.
 *
 * Note:
 *   - This function returns with a newly allocated buffer for uint32 strings,
 *     which must be freed if the optional uint32_buff_p parameter is NULL.
 *   - The ASCII check only happens if the flags parameter gets
 *     'ECMA_STRING_FLAG_IS_ASCII' as an input.
 *
 * @return start of cesu8 characters
 */
const lit_utf8_byte_t *
ecma_string_get_chars (const ecma_string_t *string_p, /**< ecma-string */
                       lit_utf8_size_t *size_p, /**< [out] size of the ecma string */
                       lit_utf8_size_t *length_p, /**< [out] optional argument. If the pointer is not NULL the pointed
                                                   *    memory area is filled with the length of the ecma string */
                       lit_utf8_byte_t *uint32_buff_p, /**< [out] optional argument. If the pointer is not NULL the
                                                        *    pointed memory area is filled with the string converted
                                                        *    uint32 string descriptor */
                       uint8_t *flags_p) /**< [in,out] any combination of ecma_string_flag_t bits */
{
  lit_utf8_size_t length;
  lit_utf8_size_t size;
  const lit_utf8_byte_t *result_p;

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    *flags_p |= ECMA_STRING_FLAG_REHASH_NEEDED;

    switch (ECMA_GET_DIRECT_STRING_TYPE (string_p))
    {
      case ECMA_DIRECT_STRING_MAGIC:
      {
        uint32_t id = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);

        if (id >= LIT_MAGIC_STRING__COUNT)
        {
          id -= LIT_MAGIC_STRING__COUNT;
          size = lit_get_magic_string_ex_size (id);
          result_p = lit_get_magic_string_ex_utf8 (id);
          length = 0;

          if (JERRY_UNLIKELY (*flags_p & ECMA_STRING_FLAG_IS_ASCII))
          {
            length = lit_utf8_string_length (result_p, size);
          }
        }
        else
        {
          size = lit_get_magic_string_size (id);
          length = size;

          result_p = lit_get_magic_string_utf8 (id);

          /* All magic strings must be ascii strings. */
          JERRY_ASSERT (ECMA_STRING_IS_ASCII (result_p, size));
        }
        break;
      }
      default:
      {
        JERRY_ASSERT (ECMA_GET_DIRECT_STRING_TYPE (string_p) == ECMA_DIRECT_STRING_UINT);
        uint32_t uint32_number = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);
        size = (lit_utf8_size_t) ecma_string_get_uint32_size (uint32_number);

        if (uint32_buff_p != NULL)
        {
          result_p = uint32_buff_p;
        }
        else
        {
          result_p = (const lit_utf8_byte_t *) jmem_heap_alloc_block (size);
          *flags_p |= ECMA_STRING_FLAG_MUST_BE_FREED;
        }

        length = ecma_uint32_to_utf8_string (uint32_number, (lit_utf8_byte_t *) result_p, size);

        JERRY_ASSERT (length == size);
        *flags_p |= ECMA_STRING_FLAG_IS_UINT32;
        break;
      }
    }
  }
  else
  {
    JERRY_ASSERT (string_p->refs_and_container >= ECMA_STRING_REF_ONE);

    switch (ECMA_STRING_GET_CONTAINER (string_p))
    {
      case ECMA_STRING_CONTAINER_HEAP_UTF8_STRING:
      {
        ecma_short_string_t *short_string_p = (ecma_short_string_t *) string_p;
        size = short_string_p->size;
        length = short_string_p->length;
        result_p = ECMA_SHORT_STRING_GET_BUFFER (short_string_p);
        break;
      }
      case ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING:
      {
        ecma_long_string_t *long_string_desc_p = (ecma_long_string_t *) string_p;
        size = long_string_desc_p->size;
        length = long_string_desc_p->length;
        result_p = long_string_desc_p->string_p;
        break;
      }
      case ECMA_STRING_CONTAINER_HEAP_ASCII_STRING:
      {
        size = ECMA_ASCII_STRING_GET_SIZE (string_p);
        length = size;
        result_p = ECMA_ASCII_STRING_GET_BUFFER (string_p);
        break;
      }
      case ECMA_STRING_CONTAINER_UINT32_IN_DESC:
      {
        size = (lit_utf8_size_t) ecma_string_get_uint32_size (string_p->u.uint32_number);

        if (uint32_buff_p != NULL)
        {
          result_p = uint32_buff_p;
        }
        else
        {
          result_p = (const lit_utf8_byte_t *) jmem_heap_alloc_block (size);
          *flags_p |= ECMA_STRING_FLAG_MUST_BE_FREED;
        }

        length = ecma_uint32_to_utf8_string (string_p->u.uint32_number, (lit_utf8_byte_t *) result_p, size);

        JERRY_ASSERT (length == size);
        *flags_p |= ECMA_STRING_FLAG_IS_UINT32 | ECMA_STRING_FLAG_REHASH_NEEDED;
        break;
      }
      default:
      {
        JERRY_ASSERT (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_MAGIC_STRING_EX);

        lit_magic_string_ex_id_t id = LIT_MAGIC_STRING__COUNT - string_p->u.magic_string_ex_id;
        size = lit_get_magic_string_ex_size (id);
        length = 0;

        if (JERRY_UNLIKELY (*flags_p & ECMA_STRING_FLAG_IS_ASCII))
        {
          length = lit_utf8_string_length (lit_get_magic_string_ex_utf8 (id), size);
        }

        result_p = lit_get_magic_string_ex_utf8 (id);
        *flags_p |= ECMA_STRING_FLAG_REHASH_NEEDED;
        break;
      }
    }
  }

  *size_p = size;
  if (length_p != NULL)
  {
    *length_p = length;
  }

  if ((*flags_p & ECMA_STRING_FLAG_IS_ASCII) && length != size)
  {
    *flags_p = (uint8_t) (*flags_p & (uint8_t) ~ECMA_STRING_FLAG_IS_ASCII);
  }

  return result_p;
} /* ecma_string_get_chars */

/**
 * Checks whether the string equals to the magic string id.
 *
 * @return true - if the string equals to the magic string id
 *         false - otherwise
 */
bool
ecma_compare_ecma_string_to_magic_id (const ecma_string_t *string_p, /**< property name */
                                      lit_magic_string_id_t id) /**< magic string id */
{
  return (string_p == ecma_get_magic_string (id));
} /* ecma_compare_ecma_string_to_magic_id */

/**
 * Checks whether ecma string is empty or not
 *
 * @return true - if the string is an empty string
 *         false - otherwise
 */
bool
ecma_string_is_empty (const ecma_string_t *string_p) /**< ecma-string */
{
  return ecma_compare_ecma_string_to_magic_id (string_p, LIT_MAGIC_STRING__EMPTY);
} /* ecma_string_is_empty */

/**
 * Checks whether the string equals to "length".
 *
 * @return true - if the string equals to "length"
 *         false - otherwise
 */
bool
ecma_string_is_length (const ecma_string_t *string_p) /**< property name */
{
  return ecma_compare_ecma_string_to_magic_id (string_p, LIT_MAGIC_STRING_LENGTH);
} /* ecma_string_is_length */

/**
 * Converts a property name into a string
 *
 * @return pointer to the converted ecma string
 */
static inline ecma_string_t *
ecma_property_to_string (ecma_property_t property, /**< property name type */
                         jmem_cpointer_t prop_name_cp) /**< property name compressed pointer */
{
  uintptr_t property_string = ((uintptr_t) (property)) & (0x3 << ECMA_PROPERTY_NAME_TYPE_SHIFT);
  property_string = (property_string >> ECMA_STRING_TYPE_CONVERSION_SHIFT) | ECMA_TYPE_DIRECT_STRING;
  return (ecma_string_t *) (property_string | (((uintptr_t) prop_name_cp) << ECMA_DIRECT_STRING_SHIFT));
} /* ecma_property_to_string */

/**
 * Converts a string into a property name
 *
 * @return the compressed pointer part of the name
 */
jmem_cpointer_t
ecma_string_to_property_name (ecma_string_t *prop_name_p, /**< property name */
                              ecma_property_t *name_type_p) /**< [out] property name type */
{
  if (ECMA_IS_DIRECT_STRING (prop_name_p))
  {
    *name_type_p = (ecma_property_t) ECMA_DIRECT_STRING_TYPE_TO_PROP_NAME_TYPE (prop_name_p);
    return (jmem_cpointer_t) ECMA_GET_DIRECT_STRING_VALUE (prop_name_p);
  }

  *name_type_p = ECMA_DIRECT_STRING_PTR << ECMA_PROPERTY_NAME_TYPE_SHIFT;

  ecma_ref_ecma_string (prop_name_p);

  jmem_cpointer_t prop_name_cp;
  ECMA_SET_NON_NULL_POINTER (prop_name_cp, prop_name_p);
  return prop_name_cp;
} /* ecma_string_to_property_name */

/**
 * Converts a property name into a string
 *
 * @return the string pointer
 *         string must be released with ecma_deref_ecma_string
 */
ecma_string_t *
ecma_string_from_property_name (ecma_property_t property, /**< property name type */
                                jmem_cpointer_t prop_name_cp) /**< property name compressed pointer */
{
  if (ECMA_PROPERTY_GET_NAME_TYPE (property) != ECMA_DIRECT_STRING_PTR)
  {
    return ecma_property_to_string (property, prop_name_cp);
  }

  ecma_string_t *prop_name_p = ECMA_GET_NON_NULL_POINTER (ecma_string_t, prop_name_cp);
  ecma_ref_ecma_string (prop_name_p);
  return prop_name_p;
} /* ecma_string_from_property_name */

/**
 * Get hash code of property name
 *
 * @return hash code of property name
 */
lit_string_hash_t
ecma_string_get_property_name_hash (ecma_property_t property, /**< property name type */
                                    jmem_cpointer_t prop_name_cp) /**< property name compressed pointer */
{
  if (ECMA_PROPERTY_GET_NAME_TYPE (property) == ECMA_DIRECT_STRING_PTR)
  {
    ecma_string_t *prop_name_p = ECMA_GET_NON_NULL_POINTER (ecma_string_t, prop_name_cp);
    return prop_name_p->u.hash;
  }

  return (lit_string_hash_t) prop_name_cp;
} /* ecma_string_get_property_name_hash */

/**
 * Check if property name is array index.
 *
 * @return ECMA_STRING_NOT_ARRAY_INDEX if string is not array index
 *         the array index otherwise
 */
uint32_t
ecma_string_get_property_index (ecma_property_t property, /**< property name type */
                                jmem_cpointer_t prop_name_cp) /**< property name compressed pointer */
{
  switch (ECMA_PROPERTY_GET_NAME_TYPE (property))
  {
    case ECMA_DIRECT_STRING_UINT:
    {
      return (uint32_t) prop_name_cp;
    }
    case ECMA_DIRECT_STRING_PTR:
    {
      ecma_string_t *prop_name_p = ECMA_GET_NON_NULL_POINTER (ecma_string_t, prop_name_cp);
      return ecma_string_get_array_index (prop_name_p);
    }
    default:
    {
      return ECMA_STRING_NOT_ARRAY_INDEX;
    }
  }
} /* ecma_string_get_property_index */

/**
 * Compare a property name to a string
 *
 * @return true if they are equals
 *         false otherwise
 */
bool
ecma_string_compare_to_property_name (ecma_property_t property, /**< property name type */
                                      jmem_cpointer_t prop_name_cp, /**< property name compressed pointer */
                                      const ecma_string_t *string_p) /**< other string */
{
  if (ECMA_PROPERTY_GET_NAME_TYPE (property) != ECMA_DIRECT_STRING_PTR)
  {
    return ecma_property_to_string (property, prop_name_cp) == string_p;
  }

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    return false;
  }

  ecma_string_t *prop_name_p = ECMA_GET_NON_NULL_POINTER (ecma_string_t, prop_name_cp);
  return ecma_compare_ecma_non_direct_strings (prop_name_p, string_p);
} /* ecma_string_compare_to_property_name */

/**
 * Helper for ecma_compare_ecma_strings_longpath to get string data
 *
 * @return string characters
 */
static const lit_utf8_byte_t *
ecma_compare_get_string_chars (const ecma_string_t *string_p, /**< ecma-string */
                               lit_utf8_size_t *size_and_length_p) /**< [out] size and length */
{
  switch (ECMA_STRING_GET_CONTAINER (string_p))
  {
    case ECMA_STRING_CONTAINER_HEAP_UTF8_STRING:
    {
      ecma_short_string_t *short_string_p = (ecma_short_string_t *) string_p;
      size_and_length_p[0] = short_string_p->size;
      size_and_length_p[1] = short_string_p->length;
      return ECMA_SHORT_STRING_GET_BUFFER (string_p);
    }
    case ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING:
    {
      ecma_long_string_t *long_string_p = (ecma_long_string_t *) string_p;
      size_and_length_p[0] = long_string_p->size;
      size_and_length_p[1] = long_string_p->length;
      return long_string_p->string_p;
    }
    case ECMA_STRING_CONTAINER_HEAP_ASCII_STRING:
    {
      size_and_length_p[0] = ECMA_ASCII_STRING_GET_SIZE (string_p);
      size_and_length_p[1] = size_and_length_p[0];
      return ECMA_ASCII_STRING_GET_BUFFER (string_p);
    }
    default:
    {
      return NULL;
    }
  }
} /* ecma_compare_get_string_chars */

/**
 * Long path part of ecma-string to ecma-string comparison routine
 *
 * See also:
 *          ecma_compare_ecma_strings
 *
 * @return true - if strings are equal;
 *         false - otherwise
 */
static bool JERRY_ATTR_NOINLINE
ecma_compare_ecma_strings_longpath (const ecma_string_t *string1_p, /**< ecma-string */
                                    const ecma_string_t *string2_p) /**< ecma-string */
{
  const lit_utf8_byte_t *utf8_string1_p, *utf8_string2_p;
  lit_utf8_size_t string1_size_and_length[2] = { 0, 0 }, string2_size_and_length[2] = { 0, 0 };

  utf8_string1_p = ecma_compare_get_string_chars (string1_p, string1_size_and_length);
  utf8_string2_p = ecma_compare_get_string_chars (string2_p, string2_size_and_length);

  if (utf8_string1_p == NULL || utf8_string2_p == NULL)
  {
    return false;
  }

  if (string1_size_and_length[0] != string2_size_and_length[0]
      || string1_size_and_length[1] != string2_size_and_length[1])
  {
    return false;
  }

  return !memcmp ((char *) utf8_string1_p, (char *) utf8_string2_p, string1_size_and_length[0]);
} /* ecma_compare_ecma_strings_longpath */

/**
 * Compare two ecma-strings
 *
 * @return true - if strings are equal;
 *         false - otherwise
 */
bool
ecma_compare_ecma_strings (const ecma_string_t *string1_p, /**< ecma-string */
                           const ecma_string_t *string2_p) /**< ecma-string */
{
  JERRY_ASSERT (string1_p != NULL && string2_p != NULL);

  /* Fast paths first. */
  if (string1_p == string2_p)
  {
    return true;
  }

  /* Either string is direct, return with false. */
  if (ECMA_IS_DIRECT_STRING (((uintptr_t) string1_p) | ((uintptr_t) string2_p)))
  {
    return false;
  }

  /* Also compares uint32 values in descriptor. */
  if (string1_p->u.hash != string2_p->u.hash)
  {
    return false;
  }

  if (ECMA_STRING_GET_CONTAINER (string1_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC)
  {
    return ECMA_STRING_GET_CONTAINER (string2_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC;
  }

  return ecma_compare_ecma_strings_longpath (string1_p, string2_p);
} /* ecma_compare_ecma_strings */

/**
 * Compare two non-direct ecma-strings
 *
 * @return true - if strings are equal;
 *         false - otherwise
 */
bool
ecma_compare_ecma_non_direct_strings (const ecma_string_t *string1_p, /**< ecma-string */
                                      const ecma_string_t *string2_p) /**< ecma-string */
{
  JERRY_ASSERT (string1_p != NULL && string2_p != NULL);
  JERRY_ASSERT (!ECMA_IS_DIRECT_STRING (string1_p) && !ECMA_IS_DIRECT_STRING (string2_p));

  /* Fast paths first. */
  if (string1_p == string2_p)
  {
    return true;
  }

  if (string1_p->u.hash != string2_p->u.hash)
  {
    return false;
  }

  if (ECMA_STRING_GET_CONTAINER (string1_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC)
  {
    return ECMA_STRING_GET_CONTAINER (string2_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC;
  }

  return ecma_compare_ecma_strings_longpath (string1_p, string2_p);
} /* ecma_compare_ecma_non_direct_strings */

/**
 * Relational compare of ecma-strings.
 *
 * First string is less than second string if:
 *  - strings are not equal;
 *  - first string is prefix of second or is lexicographically less than second.
 *
 * @return true - if first string is less than second string,
 *         false - otherwise
 */
bool
ecma_compare_ecma_strings_relational (const ecma_string_t *string1_p, /**< ecma-string */
                                      const ecma_string_t *string2_p) /**< ecma-string */
{
  if (ecma_compare_ecma_strings (string1_p, string2_p))
  {
    return false;
  }

  const lit_utf8_byte_t *utf8_string1_p, *utf8_string2_p;
  lit_utf8_size_t utf8_string1_size, utf8_string2_size;

  lit_utf8_byte_t uint32_to_string_buffer1[ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32];
  lit_utf8_byte_t uint32_to_string_buffer2[ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32];

  if (ECMA_IS_DIRECT_STRING (string1_p))
  {
    if (ECMA_GET_DIRECT_STRING_TYPE (string1_p) != ECMA_DIRECT_STRING_UINT)
    {
      utf8_string1_p = ecma_string_get_chars_fast (string1_p, &utf8_string1_size);
    }
    else
    {
      utf8_string1_size = ecma_uint32_to_utf8_string ((uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string1_p),
                                                      uint32_to_string_buffer1,
                                                      ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32);
      utf8_string1_p = uint32_to_string_buffer1;
    }
  }
  else
  {
    JERRY_ASSERT (string1_p->refs_and_container >= ECMA_STRING_REF_ONE);

    if (ECMA_STRING_GET_CONTAINER (string1_p) != ECMA_STRING_CONTAINER_UINT32_IN_DESC)
    {
      utf8_string1_p = ecma_string_get_chars_fast (string1_p, &utf8_string1_size);
    }
    else
    {
      utf8_string1_size = ecma_uint32_to_utf8_string (string1_p->u.uint32_number,
                                                      uint32_to_string_buffer1,
                                                      ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32);
      utf8_string1_p = uint32_to_string_buffer1;
    }
  }

  if (ECMA_IS_DIRECT_STRING (string2_p))
  {
    if (ECMA_GET_DIRECT_STRING_TYPE (string2_p) != ECMA_DIRECT_STRING_UINT)
    {
      utf8_string2_p = ecma_string_get_chars_fast (string2_p, &utf8_string2_size);
    }
    else
    {
      utf8_string2_size = ecma_uint32_to_utf8_string ((uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string2_p),
                                                      uint32_to_string_buffer2,
                                                      ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32);
      utf8_string2_p = uint32_to_string_buffer2;
    }
  }
  else
  {
    JERRY_ASSERT (string2_p->refs_and_container >= ECMA_STRING_REF_ONE);

    if (ECMA_STRING_GET_CONTAINER (string2_p) != ECMA_STRING_CONTAINER_UINT32_IN_DESC)
    {
      utf8_string2_p = ecma_string_get_chars_fast (string2_p, &utf8_string2_size);
    }
    else
    {
      utf8_string2_size = ecma_uint32_to_utf8_string (string2_p->u.uint32_number,
                                                      uint32_to_string_buffer2,
                                                      ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32);
      utf8_string2_p = uint32_to_string_buffer2;
    }
  }

  return lit_compare_utf8_strings_relational (utf8_string1_p, utf8_string1_size, utf8_string2_p, utf8_string2_size);
} /* ecma_compare_ecma_strings_relational */

/**
 * Special value to represent that no size is available.
 */
#define ECMA_STRING_NO_ASCII_SIZE 0xffffffff

/**
 * Return the size of uint32 and magic strings.
 * The length of these strings are equal to their size.
 *
 * @return number of characters in the string
 */
static lit_utf8_size_t
ecma_string_get_ascii_size (const ecma_string_t *string_p) /**< ecma-string */
{
  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    switch (ECMA_GET_DIRECT_STRING_TYPE (string_p))
    {
      case ECMA_DIRECT_STRING_MAGIC:
      {
        uint32_t id = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);

        if (id >= LIT_MAGIC_STRING__COUNT)
        {
          return ECMA_STRING_NO_ASCII_SIZE;
        }

        JERRY_ASSERT (ECMA_STRING_IS_ASCII (lit_get_magic_string_utf8 (id), lit_get_magic_string_size (id)));

        return lit_get_magic_string_size (id);
      }
      default:
      {
        JERRY_ASSERT (ECMA_GET_DIRECT_STRING_TYPE (string_p) == ECMA_DIRECT_STRING_UINT);
        uint32_t uint32_number = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);
        return ecma_string_get_uint32_size (uint32_number);
      }
    }
  }

  JERRY_ASSERT (string_p->refs_and_container >= ECMA_STRING_REF_ONE);

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_UINT32_IN_DESC)
  {
    return ecma_string_get_uint32_size (string_p->u.uint32_number);
  }
  else if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_HEAP_ASCII_STRING)
  {
    return ECMA_ASCII_STRING_GET_SIZE (string_p);
  }

  return ECMA_STRING_NO_ASCII_SIZE;
} /* ecma_string_get_ascii_size */

/**
 * Get length of ecma-string
 *
 * @return number of characters in the string
 */
lit_utf8_size_t
ecma_string_get_length (const ecma_string_t *string_p) /**< ecma-string */
{
  lit_utf8_size_t length = ecma_string_get_ascii_size (string_p);

  if (length != ECMA_STRING_NO_ASCII_SIZE)
  {
    return length;
  }

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    JERRY_ASSERT (ECMA_GET_DIRECT_STRING_TYPE (string_p) == ECMA_DIRECT_STRING_MAGIC);
    JERRY_ASSERT ((uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p) >= LIT_MAGIC_STRING__COUNT);

    uint32_t id = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p) - LIT_MAGIC_STRING__COUNT;
    return lit_utf8_string_length (lit_get_magic_string_ex_utf8 (id), lit_get_magic_string_ex_size (id));
  }

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_HEAP_UTF8_STRING)
  {
    return ((ecma_short_string_t *) string_p)->length;
  }

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING)
  {
    return ((ecma_long_string_t *) string_p)->length;
  }

  JERRY_ASSERT (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_MAGIC_STRING_EX);

  lit_magic_string_ex_id_t id = LIT_MAGIC_STRING__COUNT - string_p->u.magic_string_ex_id;
  return lit_utf8_string_length (lit_get_magic_string_ex_utf8 (id), lit_get_magic_string_ex_size (id));
} /* ecma_string_get_length */

/**
 * Get length of UTF-8 encoded string length from ecma-string
 *
 * @return number of characters in the UTF-8 encoded string
 */
lit_utf8_size_t
ecma_string_get_utf8_length (const ecma_string_t *string_p) /**< ecma-string */
{
  lit_utf8_size_t length = ecma_string_get_ascii_size (string_p);

  if (length != ECMA_STRING_NO_ASCII_SIZE)
  {
    return length;
  }

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    JERRY_ASSERT (ECMA_GET_DIRECT_STRING_TYPE (string_p) == ECMA_DIRECT_STRING_MAGIC);
    JERRY_ASSERT ((uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p) >= LIT_MAGIC_STRING__COUNT);

    uint32_t id = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p) - LIT_MAGIC_STRING__COUNT;
    return lit_get_utf8_length_of_cesu8_string (lit_get_magic_string_ex_utf8 (id), lit_get_magic_string_ex_size (id));
  }

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_HEAP_UTF8_STRING)
  {
    ecma_short_string_t *short_string_p = (ecma_short_string_t *) string_p;
    lit_utf8_size_t size = short_string_p->size;

    if (size == short_string_p->length)
    {
      return size;
    }

    return lit_get_utf8_length_of_cesu8_string (ECMA_SHORT_STRING_GET_BUFFER (string_p), size);
  }

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING)
  {
    ecma_long_string_t *long_string_p = (ecma_long_string_t *) string_p;
    lit_utf8_size_t size = long_string_p->size;

    if (size == long_string_p->length)
    {
      return size;
    }

    return lit_get_utf8_length_of_cesu8_string (long_string_p->string_p, size);
  }

  JERRY_ASSERT (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_MAGIC_STRING_EX);

  lit_magic_string_ex_id_t id = LIT_MAGIC_STRING__COUNT - string_p->u.magic_string_ex_id;

  return lit_get_utf8_length_of_cesu8_string (lit_get_magic_string_ex_utf8 (id), lit_get_magic_string_ex_size (id));
} /* ecma_string_get_utf8_length */

/**
 * Get size of ecma-string
 *
 * @return number of bytes in the buffer needed to represent the string
 */
lit_utf8_size_t
ecma_string_get_size (const ecma_string_t *string_p) /**< ecma-string */
{
  lit_utf8_size_t length = ecma_string_get_ascii_size (string_p);

  if (length != ECMA_STRING_NO_ASCII_SIZE)
  {
    return length;
  }

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    JERRY_ASSERT (ECMA_GET_DIRECT_STRING_TYPE (string_p) == ECMA_DIRECT_STRING_MAGIC);
    JERRY_ASSERT ((uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p) >= LIT_MAGIC_STRING__COUNT);

    return lit_get_magic_string_ex_size ((uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p) - LIT_MAGIC_STRING__COUNT);
  }

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_HEAP_UTF8_STRING)
  {
    return ((ecma_short_string_t *) string_p)->size;
  }

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING)
  {
    return ((ecma_long_string_t *) string_p)->size;
  }

  JERRY_ASSERT (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_MAGIC_STRING_EX);

  return lit_get_magic_string_ex_size (LIT_MAGIC_STRING__COUNT - string_p->u.magic_string_ex_id);
} /* ecma_string_get_size */

/**
 * Get the UTF-8 encoded string size from ecma-string
 *
 * @return number of bytes in the buffer needed to represent an UTF-8 encoded string
 */
lit_utf8_size_t
ecma_string_get_utf8_size (const ecma_string_t *string_p) /**< ecma-string */
{
  lit_utf8_size_t length = ecma_string_get_ascii_size (string_p);

  if (length != ECMA_STRING_NO_ASCII_SIZE)
  {
    return length;
  }

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    JERRY_ASSERT (ECMA_GET_DIRECT_STRING_TYPE (string_p) == ECMA_DIRECT_STRING_MAGIC);
    JERRY_ASSERT ((uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p) >= LIT_MAGIC_STRING__COUNT);

    uint32_t id = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p) - LIT_MAGIC_STRING__COUNT;
    return lit_get_utf8_size_of_cesu8_string (lit_get_magic_string_ex_utf8 (id), lit_get_magic_string_ex_size (id));
  }

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_HEAP_UTF8_STRING)
  {
    ecma_short_string_t *short_string_p = (ecma_short_string_t *) string_p;
    lit_utf8_size_t size = short_string_p->size;

    if (size == short_string_p->length)
    {
      return size;
    }

    return lit_get_utf8_size_of_cesu8_string (ECMA_SHORT_STRING_GET_BUFFER (string_p), size);
  }

  if (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING)
  {
    ecma_long_string_t *long_string_p = (ecma_long_string_t *) string_p;

    if (long_string_p->size == long_string_p->length)
    {
      return long_string_p->size;
    }

    return lit_get_utf8_size_of_cesu8_string (long_string_p->string_p, long_string_p->size);
  }

  JERRY_ASSERT (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_MAGIC_STRING_EX);

  lit_magic_string_ex_id_t id = LIT_MAGIC_STRING__COUNT - string_p->u.magic_string_ex_id;
  return lit_get_utf8_size_of_cesu8_string (lit_get_magic_string_ex_utf8 (id), lit_get_magic_string_ex_size (id));
} /* ecma_string_get_utf8_size */

/**
 * Get character from specified position in an external ecma-string.
 *
 * @return character value
 */
static ecma_char_t JERRY_ATTR_NOINLINE
ecma_external_string_get_char_at_pos (lit_utf8_size_t id, /**< id of the external magic string */
                                      lit_utf8_size_t index) /**< index of character */
{
  id -= LIT_MAGIC_STRING__COUNT;
  const lit_utf8_byte_t *data_p = lit_get_magic_string_ex_utf8 (id);
  lit_utf8_size_t size = lit_get_magic_string_ex_size (id);
  lit_utf8_size_t length = lit_utf8_string_length (data_p, size);

  if (JERRY_LIKELY (size == length))
  {
    return (ecma_char_t) data_p[index];
  }

  return lit_utf8_string_code_unit_at (data_p, size, index);
} /* ecma_external_string_get_char_at_pos */

/**
 * Get character from specified position in the ecma-string.
 *
 * @return character value
 */
ecma_char_t
ecma_string_get_char_at_pos (const ecma_string_t *string_p, /**< ecma-string */
                             lit_utf8_size_t index) /**< index of character */
{
  JERRY_ASSERT (index < ecma_string_get_length (string_p));

  lit_utf8_byte_t uint32_to_string_buffer[ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32];

  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    switch (ECMA_GET_DIRECT_STRING_TYPE (string_p))
    {
      case ECMA_DIRECT_STRING_MAGIC:
      {
        uint32_t id = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);

        if (JERRY_LIKELY (id < LIT_MAGIC_STRING__COUNT))
        {
          /* All magic strings must be ascii strings. */
          const lit_utf8_byte_t *data_p = lit_get_magic_string_utf8 (id);

          return (ecma_char_t) data_p[index];
        }

        return ecma_external_string_get_char_at_pos (id, index);
      }
      default:
      {
        JERRY_ASSERT (ECMA_GET_DIRECT_STRING_TYPE (string_p) == ECMA_DIRECT_STRING_UINT);
        uint32_t uint32_number = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);

        ecma_uint32_to_utf8_string (uint32_number, uint32_to_string_buffer, ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32);

        return (ecma_char_t) uint32_to_string_buffer[index];
      }
    }
  }

  JERRY_ASSERT (string_p->refs_and_container >= ECMA_STRING_REF_ONE);

  switch (ECMA_STRING_GET_CONTAINER (string_p))
  {
    case ECMA_STRING_CONTAINER_HEAP_UTF8_STRING:
    {
      ecma_short_string_t *short_string_p = (ecma_short_string_t *) string_p;
      lit_utf8_size_t size = short_string_p->size;
      const lit_utf8_byte_t *data_p = ECMA_SHORT_STRING_GET_BUFFER (string_p);

      if (JERRY_LIKELY (size == short_string_p->length))
      {
        return (ecma_char_t) data_p[index];
      }

      return lit_utf8_string_code_unit_at (data_p, size, index);
    }
    case ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING:
    {
      ecma_long_string_t *long_string_p = (ecma_long_string_t *) string_p;
      lit_utf8_size_t size = long_string_p->size;
      const lit_utf8_byte_t *data_p = long_string_p->string_p;

      if (JERRY_LIKELY (size == long_string_p->length))
      {
        return (ecma_char_t) data_p[index];
      }

      return lit_utf8_string_code_unit_at (data_p, size, index);
    }
    case ECMA_STRING_CONTAINER_HEAP_ASCII_STRING:
    {
      const lit_utf8_byte_t *data_p = ECMA_ASCII_STRING_GET_BUFFER (string_p);
      return (ecma_char_t) data_p[index];
    }
    case ECMA_STRING_CONTAINER_UINT32_IN_DESC:
    {
      ecma_uint32_to_utf8_string (string_p->u.uint32_number,
                                  uint32_to_string_buffer,
                                  ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32);

      return (ecma_char_t) uint32_to_string_buffer[index];
    }
    default:
    {
      JERRY_ASSERT (ECMA_STRING_GET_CONTAINER (string_p) == ECMA_STRING_CONTAINER_MAGIC_STRING_EX);
      return ecma_external_string_get_char_at_pos (string_p->u.magic_string_ex_id, index);
    }
  }
} /* ecma_string_get_char_at_pos */

/**
 * Check if passed string equals to one of magic strings
 * and if equal magic string was found, return it's id in 'out_id_p' argument.
 *
 * @return id - if magic string equal to passed string was found,
 *         LIT_MAGIC_STRING__COUNT - otherwise.
 */
lit_magic_string_id_t
ecma_get_string_magic (const ecma_string_t *string_p) /**< ecma-string */
{
  if (ECMA_IS_DIRECT_STRING_WITH_TYPE (string_p, ECMA_DIRECT_STRING_MAGIC))
  {
    uint32_t id = (uint32_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);

    if (id < LIT_MAGIC_STRING__COUNT)
    {
      return (lit_magic_string_id_t) id;
    }
  }

  return LIT_MAGIC_STRING__COUNT;
} /* ecma_get_string_magic */

/**
 * Try to calculate hash of the ecma-string
 *
 * @return calculated hash
 */
lit_string_hash_t
ecma_string_hash (const ecma_string_t *string_p) /**< ecma-string to calculate hash for */
{
  if (ECMA_IS_DIRECT_STRING (string_p))
  {
    return (lit_string_hash_t) ECMA_GET_DIRECT_STRING_VALUE (string_p);
  }

  return (lit_string_hash_t) string_p->u.hash;
} /* ecma_string_hash */

/**
 * Create a substring from an ecma string
 *
 * @return a newly constructed ecma string with its value initialized to a copy of a substring of the first argument
 */
ecma_string_t *
ecma_string_substr (const ecma_string_t *string_p, /**< pointer to an ecma string */
                    lit_utf8_size_t start_pos, /**< start position, should be less or equal than string length */
                    lit_utf8_size_t end_pos) /**< end position, should be less or equal than string length */
{
  const lit_utf8_size_t string_length = ecma_string_get_length (string_p);
  JERRY_ASSERT (start_pos <= string_length);
  JERRY_ASSERT (end_pos <= string_length);

  if (start_pos >= end_pos)
  {
    return ecma_get_magic_string (LIT_MAGIC_STRING__EMPTY);
  }

  ecma_string_t *ecma_string_p = NULL;
  end_pos -= start_pos;

  ECMA_STRING_TO_UTF8_STRING (string_p, start_p, buffer_size);

  if (string_length == buffer_size)
  {
    ecma_string_p = ecma_new_ecma_string_from_utf8 (start_p + start_pos, (lit_utf8_size_t) end_pos);
  }
  else
  {
    while (start_pos--)
    {
      start_p += lit_get_unicode_char_size_by_utf8_first_byte (*start_p);
    }

    const lit_utf8_byte_t *end_p = start_p;
    while (end_pos--)
    {
      end_p += lit_get_unicode_char_size_by_utf8_first_byte (*end_p);
    }

    ecma_string_p = ecma_new_ecma_string_from_utf8 (start_p, (lit_utf8_size_t) (end_p - start_p));
  }

  ECMA_FINALIZE_UTF8_STRING (start_p, buffer_size);

  return ecma_string_p;
} /* ecma_string_substr */

/**
 * Helper function for trimming.
 *
 * Used by:
 *        - ecma_string_trim_helper
 *        - ecma_builtin_global_object_parse_int
 *        - ecma_builtin_global_object_parse_float
 *
 * @return position of the first non whitespace character.
 */
const lit_utf8_byte_t *
ecma_string_trim_front (const lit_utf8_byte_t *start_p, /**< current string's start position */
                        const lit_utf8_byte_t *end_p) /**< current string's end position */
{
  ecma_char_t ch;

  while (start_p < end_p)
  {
    lit_utf8_size_t read_size = lit_read_code_unit_from_cesu8 (start_p, &ch);

    if (!lit_char_is_white_space (ch))
    {
      break;
    }

    start_p += read_size;
  }

  return start_p;
} /* ecma_string_trim_front */

/**
 * Helper function for trimming.
 *
 * Used by:
 *        - ecma_string_trim_helper
 *
 * @return position of the last non whitespace character.
 */
const lit_utf8_byte_t *
ecma_string_trim_back (const lit_utf8_byte_t *start_p, /**< current string's start position */
                       const lit_utf8_byte_t *end_p) /**< current string's end position */
{
  ecma_char_t ch;

  while (end_p > start_p)
  {
    lit_utf8_size_t read_size = lit_read_prev_code_unit_from_utf8 (end_p, &ch);

    if (!lit_char_is_white_space (ch))
    {
      break;
    }

    end_p -= read_size;
  }

  return end_p;
} /* ecma_string_trim_back */

/**
 * Helper function for trimming.
 *
 * Used by:
 *        - ecma_string_trim
 *        - ecma_utf8_string_to_number
 *
 * @return void
 */
void
ecma_string_trim_helper (const lit_utf8_byte_t **utf8_str_p, /**< [in, out] current string position */
                         lit_utf8_size_t *utf8_str_size) /**< [in, out] size of the given string */
{
  const lit_utf8_byte_t *end_p = *utf8_str_p + *utf8_str_size;
  const lit_utf8_byte_t *start_p = *utf8_str_p;

  const lit_utf8_byte_t *new_start_p = ecma_string_trim_front (start_p, end_p);
  const lit_utf8_byte_t *new_end_p = ecma_string_trim_back (new_start_p, end_p);

  *utf8_str_size = (lit_utf8_size_t) (new_end_p - new_start_p);
  *utf8_str_p = new_start_p;
} /* ecma_string_trim_helper */

/**
 * Trim leading and trailing whitespace characters from string.
 *
 * @return trimmed ecma string
 */
ecma_string_t *
ecma_string_trim (const ecma_string_t *string_p) /**< pointer to an ecma string */
{
  ecma_string_t *ret_string_p;

  lit_utf8_size_t utf8_str_size;
  uint8_t flags = ECMA_STRING_FLAG_IS_ASCII;
  const lit_utf8_byte_t *utf8_str_p = ecma_string_get_chars (string_p, &utf8_str_size, NULL, NULL, &flags);

  if (utf8_str_size > 0)
  {
    ecma_string_trim_helper (&utf8_str_p, &utf8_str_size);
    ret_string_p = ecma_new_ecma_string_from_utf8 (utf8_str_p, utf8_str_size);
  }
  else
  {
    ret_string_p = ecma_get_magic_string (LIT_MAGIC_STRING__EMPTY);
  }

  if (flags & ECMA_STRING_FLAG_MUST_BE_FREED)
  {
    jmem_heap_free_block ((void *) utf8_str_p, utf8_str_size);
  }

  return ret_string_p;
} /* ecma_string_trim */

/**
 * Pad the beginning or the end of string with parameter given in fill_string to the length of max_length.
 *
 * @return new string from original, padded with given parameters
 */
ecma_value_t
ecma_string_pad (ecma_value_t original_string_p, /**< Input ecma string */
                 ecma_value_t max_length, /**< Length to pad to, including original length */
                 ecma_value_t fill_string, /**< The string to pad with */
                 bool pad_on_start) /**< true - if we are padding to the start, calling with padStart
                                         false - if we are padding to the end, calling with padEnd */
{
  /* 3 */
  ecma_length_t int_max_length;
  if (ECMA_IS_VALUE_ERROR (ecma_op_to_length (max_length, &int_max_length)))
  {
    return ECMA_VALUE_ERROR;
  }
  /* 4 */
  ecma_string_t *original_str_val_p = ecma_get_string_from_value (original_string_p);
  const uint32_t string_length = ecma_string_get_length (original_str_val_p);
  /* 5 */
  if (int_max_length <= string_length)
  {
    ecma_ref_ecma_string (original_str_val_p);
    return original_string_p;
  }

  ecma_string_t *filler_p = ecma_get_magic_string (LIT_MAGIC_STRING_SPACE_CHAR);
  /* 6 - 7 */
  if (!ecma_is_value_undefined (fill_string))
  {
    filler_p = ecma_op_to_string (fill_string);
    if (filler_p == NULL)
    {
      return ECMA_VALUE_ERROR;
    }
    if (ecma_string_is_empty (filler_p))
    {
      ecma_ref_ecma_string (original_str_val_p);
      return original_string_p;
    }
  }

  if (int_max_length >= UINT32_MAX)
  {
    ecma_deref_ecma_string (filler_p);
    return ecma_raise_range_error (ECMA_ERR_MAXIMUM_STRING_LENGTH_IS_REACHED);
  }

  /* 9 */
  uint32_t fill_len = (uint32_t) int_max_length - string_length;

  /* 10 */
  uint32_t filler_length = ecma_string_get_length (filler_p);
  uint32_t prepend_count = fill_len / filler_length;
  ecma_stringbuilder_t builder = ecma_stringbuilder_create ();

  if (!pad_on_start)
  {
    ecma_stringbuilder_append (&builder, original_str_val_p);
  }

  for (uint32_t i = 0; i < prepend_count; i++)
  {
    ecma_stringbuilder_append (&builder, filler_p);
  }

  uint32_t remaining = fill_len - (prepend_count * filler_length);

  ECMA_STRING_TO_UTF8_STRING (filler_p, start_p, utf8_str_size);
  const lit_utf8_byte_t *temp_start_p = start_p;
  while (remaining > 0)
  {
    ecma_char_t ch;
    lit_utf8_size_t read_size = lit_read_code_unit_from_cesu8 (temp_start_p, &ch);
    ecma_stringbuilder_append_char (&builder, ch);
    temp_start_p += read_size;
    remaining--;
  }
  ECMA_FINALIZE_UTF8_STRING (start_p, utf8_str_size);
  ecma_deref_ecma_string (filler_p);

  /* 11 - 12 */
  if (pad_on_start)
  {
    ecma_stringbuilder_append (&builder, original_str_val_p);
  }

  return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
} /* ecma_string_pad */

/**
 * Create an empty string builder
 *
 * @return new string builder
 */
ecma_stringbuilder_t
ecma_stringbuilder_create (void)
{
  const lit_utf8_size_t initial_size = ECMA_ASCII_STRING_HEADER_SIZE;
  ecma_stringbuilder_header_t *header_p = (ecma_stringbuilder_header_t *) jmem_heap_alloc_block (initial_size);
  header_p->current_size = initial_size;

  ecma_stringbuilder_t ret = { header_p };
  return ret;
} /* ecma_stringbuilder_create */

/**
 * Create a string builder from an ecma string
 *
 * @return new string builder
 */
ecma_stringbuilder_t
ecma_stringbuilder_create_from (ecma_string_t *string_p) /**< ecma string */
{
  const lit_utf8_size_t string_size = ecma_string_get_size (string_p);
  const lit_utf8_size_t initial_size = string_size + ECMA_ASCII_STRING_HEADER_SIZE;

  ecma_stringbuilder_header_t *header_p = (ecma_stringbuilder_header_t *) jmem_heap_alloc_block (initial_size);
  header_p->current_size = initial_size;

  ecma_string_to_cesu8_bytes (string_p, ECMA_STRINGBUILDER_STRING_PTR (header_p), string_size);

  ecma_stringbuilder_t ret = { header_p };
  return ret;
} /* ecma_stringbuilder_create_from */

/**
 * Create a string builder from a raw string
 *
 * @return new string builder
 */
ecma_stringbuilder_t
ecma_stringbuilder_create_raw (const lit_utf8_byte_t *data_p, /**< pointer to data */
                               const lit_utf8_size_t data_size) /**< size of the data */
{
  const lit_utf8_size_t initial_size = data_size + ECMA_ASCII_STRING_HEADER_SIZE;

  ecma_stringbuilder_header_t *header_p = (ecma_stringbuilder_header_t *) jmem_heap_alloc_block (initial_size);
  header_p->current_size = initial_size;

  memcpy (ECMA_STRINGBUILDER_STRING_PTR (header_p), data_p, data_size);

  ecma_stringbuilder_t ret = { header_p };
  return ret;
} /* ecma_stringbuilder_create_raw */

/**
 * Grow the underlying buffer of a string builder
 *
 * @return pointer to the end of the data in the underlying buffer
 */
static lit_utf8_byte_t *
ecma_stringbuilder_grow (ecma_stringbuilder_t *builder_p, /**< string builder */
                         lit_utf8_size_t required_size) /**< required size */
{
  ecma_stringbuilder_header_t *header_p = builder_p->header_p;
  JERRY_ASSERT (header_p != NULL);

  const lit_utf8_size_t new_size = header_p->current_size + required_size;
  header_p = (ecma_stringbuilder_header_t *) jmem_heap_realloc_block (header_p, header_p->current_size, new_size);
  header_p->current_size = new_size;
  builder_p->header_p = header_p;

  return ((lit_utf8_byte_t *) header_p) + header_p->current_size - required_size;
} /* ecma_stringbuilder_grow */

/**
 * Get the current size of the string in a string builder
 *
 * @return the size of the string data
 */
lit_utf8_size_t
ecma_stringbuilder_get_size (ecma_stringbuilder_t *builder_p) /**< string builder */
{
  ecma_stringbuilder_header_t *header_p = builder_p->header_p;
  JERRY_ASSERT (header_p != NULL);

  return ECMA_STRINGBUILDER_STRING_SIZE (header_p);
} /* ecma_stringbuilder_get_size */

/**
 * Get pointer to the raw string data in a string builder
 *
 * @return pointer to the string data
 */
lit_utf8_byte_t *
ecma_stringbuilder_get_data (ecma_stringbuilder_t *builder_p) /**< string builder */
{
  ecma_stringbuilder_header_t *header_p = builder_p->header_p;
  JERRY_ASSERT (header_p != NULL);

  return ECMA_STRINGBUILDER_STRING_PTR (header_p);
} /* ecma_stringbuilder_get_data */

/**
 * Revert the string builder to a smaller size
 */
void
ecma_stringbuilder_revert (ecma_stringbuilder_t *builder_p, /**< string builder */
                           const lit_utf8_size_t size) /**< new size */
{
  ecma_stringbuilder_header_t *header_p = builder_p->header_p;
  JERRY_ASSERT (header_p != NULL);

  const lit_utf8_size_t new_size = size + ECMA_ASCII_STRING_HEADER_SIZE;
  JERRY_ASSERT (new_size <= header_p->current_size);

  header_p = (ecma_stringbuilder_header_t *) jmem_heap_realloc_block (header_p, header_p->current_size, new_size);
  header_p->current_size = new_size;
  builder_p->header_p = header_p;
} /* ecma_stringbuilder_revert */

/**
 * Append an ecma_string_t to a string builder
 */
void
ecma_stringbuilder_append (ecma_stringbuilder_t *builder_p, /**< string builder */
                           const ecma_string_t *string_p) /**< ecma string */
{
  const lit_utf8_size_t string_size = ecma_string_get_size (string_p);
  lit_utf8_byte_t *dest_p = ecma_stringbuilder_grow (builder_p, string_size);

  ecma_string_to_cesu8_bytes (string_p, dest_p, string_size);
} /* ecma_stringbuilder_append */

/**
 * Append a magic string to a string builder
 */
void
ecma_stringbuilder_append_magic (ecma_stringbuilder_t *builder_p, /**< string builder */
                                 const lit_magic_string_id_t id) /**< magic string id */
{
  const lit_utf8_size_t string_size = lit_get_magic_string_size (id);
  lit_utf8_byte_t *dest_p = ecma_stringbuilder_grow (builder_p, string_size);

  const lit_utf8_byte_t *string_data_p = lit_get_magic_string_utf8 (id);
  memcpy (dest_p, string_data_p, string_size);
} /* ecma_stringbuilder_append_magic */

/**
 * Append raw string data to a string builder
 */
void
ecma_stringbuilder_append_raw (ecma_stringbuilder_t *builder_p, /**< string builder */
                               const lit_utf8_byte_t *data_p, /**< pointer to data */
                               const lit_utf8_size_t data_size) /**< size of the data */
{
  lit_utf8_byte_t *dest_p = ecma_stringbuilder_grow (builder_p, data_size);
  memcpy (dest_p, data_p, data_size);
} /* ecma_stringbuilder_append_raw */

/**
 * Append a codepoint to a string builder
 */
void
ecma_stringbuilder_append_codepoint (ecma_stringbuilder_t *builder_p, /**< string builder */
                                     lit_code_point_t cp) /**< code point */
{
  const lit_utf8_size_t size = (lit_utf8_size_t) lit_code_point_get_cesu8_length (cp);
  lit_utf8_byte_t *dest_p = ecma_stringbuilder_grow (builder_p, size);

  lit_code_point_to_cesu8_bytes (dest_p, cp);
} /* ecma_stringbuilder_append_codepoint */

/**
 * Append an ecma_char_t to a string builder
 */
void
ecma_stringbuilder_append_char (ecma_stringbuilder_t *builder_p, /**< string builder */
                                const ecma_char_t c) /**< ecma char */
{
  ecma_stringbuilder_append_codepoint (builder_p, c);
} /* ecma_stringbuilder_append_char */

/**
 * Append a single byte to a string builder
 */
void
ecma_stringbuilder_append_byte (ecma_stringbuilder_t *builder_p, /**< string builder */
                                const lit_utf8_byte_t byte) /**< byte */
{
  lit_utf8_byte_t *dest_p = ecma_stringbuilder_grow (builder_p, 1);
  *dest_p = byte;
} /* ecma_stringbuilder_append_byte */

/**
 * Finalize a string builder, returning the created string, and releasing the underlying buffer.
 *
 * Note:
 *      The builder should no longer be used.
 *
 * @return the created string
 */
ecma_string_t *
ecma_stringbuilder_finalize (ecma_stringbuilder_t *builder_p) /**< string builder */
{
  ecma_stringbuilder_header_t *header_p = builder_p->header_p;
  JERRY_ASSERT (header_p != NULL);

  const lit_utf8_size_t string_size = ECMA_STRINGBUILDER_STRING_SIZE (header_p);
  lit_utf8_byte_t *string_begin_p = ECMA_STRINGBUILDER_STRING_PTR (header_p);

  ecma_string_t *string_p = ecma_find_special_string (string_begin_p, string_size);

  if (JERRY_UNLIKELY (string_p != NULL))
  {
    ecma_stringbuilder_destroy (builder_p);
    return string_p;
  }

#ifndef JERRY_NDEBUG
  builder_p->header_p = NULL;
#endif /* !defined (JERRY_NDEBUG) */

  size_t container_size = sizeof (ecma_short_string_t);
  const lit_string_hash_t hash = lit_utf8_string_calc_hash (string_begin_p, string_size);
  const lit_utf8_size_t length = lit_utf8_string_length (string_begin_p, string_size);

  if (JERRY_LIKELY (string_size <= UINT16_MAX))
  {
    if (JERRY_LIKELY (length == string_size) && string_size <= (UINT8_MAX + 1))
    {
      string_p = (ecma_string_t *) header_p;
      string_p->refs_and_container = ECMA_STRING_CONTAINER_HEAP_ASCII_STRING | ECMA_STRING_REF_ONE;
      string_p->u.hash = hash;
      ECMA_ASCII_STRING_SET_SIZE (string_p, string_size);

      return (ecma_string_t *) string_p;
    }
  }
  else
  {
    container_size = sizeof (ecma_long_string_t);
  }

  const size_t utf8_string_size = string_size + container_size;
  header_p = (ecma_stringbuilder_header_t *) jmem_heap_realloc_block (header_p, header_p->current_size, utf8_string_size);
  memmove (((lit_utf8_byte_t *) header_p + container_size), ECMA_STRINGBUILDER_STRING_PTR (header_p), string_size);

  if (JERRY_LIKELY (string_size <= UINT16_MAX))
  {
    ecma_short_string_t *short_string_p = (ecma_short_string_t *) header_p;

    short_string_p->header.refs_and_container = ECMA_STRING_CONTAINER_HEAP_UTF8_STRING | ECMA_STRING_REF_ONE;
    short_string_p->header.u.hash = hash;
    short_string_p->size = (uint16_t) string_size;
    short_string_p->length = (uint16_t) length;

    return (ecma_string_t *) short_string_p;
  }

  ecma_long_string_t *long_string_p = (ecma_long_string_t *) header_p;

  long_string_p->header.refs_and_container = ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING | ECMA_STRING_REF_ONE;
  long_string_p->header.u.hash = hash;
  long_string_p->string_p = ECMA_LONG_STRING_BUFFER_START (long_string_p);
  long_string_p->size = string_size;
  long_string_p->length = length;

  return (ecma_string_t *) long_string_p;
} /* ecma_stringbuilder_finalize */

/**
 * Destroy a string builder that is no longer needed without creating a string from the contents.
 */
void
ecma_stringbuilder_destroy (ecma_stringbuilder_t *builder_p) /**< string builder */
{
  JERRY_ASSERT (builder_p->header_p != NULL);
  const lit_utf8_size_t size = builder_p->header_p->current_size;
  jmem_heap_free_block (builder_p->header_p, size);

#ifndef JERRY_NDEBUG
  builder_p->header_p = NULL;
#endif /* !defined (JERRY_NDEBUG) */

} /* ecma_stringbuilder_destroy */

/**
 * AdvanceStringIndex operation
 *
 * See also:
 *          ECMA-262 v6.0, 21.2.5.2.3
 *
 * @return uint32_t - the proper character index based on the operation
 */
ecma_length_t
ecma_op_advance_string_index (ecma_string_t *str_p, /**< input string */
                              ecma_length_t index, /**< given character index */
                              bool is_unicode) /**< true - if regexp object's "unicode" flag is set
                                                    false - otherwise */
{
  JERRY_ASSERT ((ecma_number_t) index <= ECMA_NUMBER_MAX_SAFE_INTEGER);
  ecma_length_t next_index = index + 1;

  if (!is_unicode)
  {
    return next_index;
  }

  lit_utf8_size_t str_len = ecma_string_get_length (str_p);

  if (next_index >= str_len)
  {
    return next_index;
  }

  JERRY_ASSERT (index < UINT32_MAX);
  ecma_char_t first = ecma_string_get_char_at_pos (str_p, (lit_utf8_size_t) index);

  if (!lit_is_code_point_utf16_high_surrogate (first))
  {
    return next_index;
  }

  ecma_char_t second = ecma_string_get_char_at_pos (str_p, (lit_utf8_size_t) next_index);

  if (!lit_is_code_point_utf16_low_surrogate (second))
  {
    return next_index;
  }

  return next_index + 1;
} /* ecma_op_advance_string_index */

/**
 * @}
 * @}
 */
