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

#include "ecma-extended-info.h"

#include "ecma-helpers.h"

#include "byte-code.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaextendedinfo Extended info
 * @{
 */

/**
 * Decodes an uint32_t number, and updates the buffer position.
 *
 * @return the decoded value
 */
uint32_t
ecma_extended_info_decode_vlq (uint8_t **buffer_p) /**< [in/out] target buffer */
{
  uint8_t *source_p = *buffer_p;
  uint32_t value = 0;

  do
  {
    source_p--;
    value = (value << ECMA_EXTENDED_INFO_VLQ_SHIFT) | (*source_p & ECMA_EXTENDED_INFO_VLQ_MASK);
  } while (*source_p & ECMA_EXTENDED_INFO_VLQ_CONTINUE);

  *buffer_p = source_p;
  return value;
} /* ecma_extended_info_decode_vlq */

/**
 * Encodes an uint32_t number into a buffer.
 */
void
ecma_extended_info_encode_vlq (uint8_t **buffer_p, /**< target buffer */
                               uint32_t value) /**< encoded value */
{
  uint8_t *destination_p = *buffer_p - 1;

  if (value <= ECMA_EXTENDED_INFO_VLQ_MASK)
  {
    *destination_p = (uint8_t) value;
    *buffer_p = destination_p;
    return;
  }

  uint32_t length = 0;
  uint32_t current_value = value >> ECMA_EXTENDED_INFO_VLQ_SHIFT;

  do
  {
    current_value >>= ECMA_EXTENDED_INFO_VLQ_SHIFT;
    length++;
  } while (current_value > 0);

  destination_p -= length;
  *buffer_p = destination_p;

  do
  {
    *destination_p++ = (uint8_t) (value | ECMA_EXTENDED_INFO_VLQ_CONTINUE);
    value >>= ECMA_EXTENDED_INFO_VLQ_SHIFT;
  } while (value > 0);

  **buffer_p &= ECMA_EXTENDED_INFO_VLQ_MASK;
} /* ecma_extended_info_encode_vlq */

/**
 * Gets the encoded length of a number.
 *
 * @return encoded length
 */
uint32_t
ecma_extended_info_get_encoded_length (uint32_t value) /**< encoded value */
{
  uint32_t length = 0;

  do
  {
    value >>= ECMA_EXTENDED_INFO_VLQ_SHIFT;
    length++;
  } while (value > 0);

  return length;
} /* ecma_extended_info_get_encoded_length */

/**
 * Get the extended info from a byte code
 *
 * @return pointer to the extended info
 */
uint8_t *
ecma_compiled_code_resolve_extended_info (const ecma_compiled_code_t *bytecode_header_p) /**< compiled code */
{
  JERRY_ASSERT (bytecode_header_p != NULL);
  JERRY_ASSERT (bytecode_header_p->status_flags & CBC_CODE_FLAGS_HAS_EXTENDED_INFO);

  ecma_value_t *base_p = ecma_compiled_code_resolve_arguments_start (bytecode_header_p);

  if (CBC_FUNCTION_GET_TYPE (bytecode_header_p->status_flags) != CBC_FUNCTION_CONSTRUCTOR)
  {
    base_p--;
  }

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_HAS_TAGGED_LITERALS)
  {
    base_p--;
  }

  JERRY_ASSERT (((uint8_t *) base_p)[-1] != 0);

  return ((uint8_t *) base_p) - 1;
} /* ecma_compiled_code_resolve_extended_info */

/**
 * @}
 * @}
 */
